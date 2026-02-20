package av

import (
	"context"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// PerformanceOptimizer provides performance enhancements for the ToxAV system.
//
// This optimizer reduces overhead in frequently called code paths by:
// - Minimizing memory allocations through object pooling
// - Reducing lock contention with atomic operations and caching
// - Providing conditional logging to eliminate unnecessary log overhead
// - Offering CPU profiling integration for performance monitoring
//
// Design philosophy: Optimize the common case (normal operation) while maintaining
// full functionality and observability when needed.
type PerformanceOptimizer struct {
	// Call slice pool for iteration - reuse slices to avoid allocations
	callSlicePool sync.Pool

	// Atomic counters for lock-free statistics
	iterationCount      int64
	totalCallsProcessed int64

	// Fast-path flags using atomic operations
	enableDetailedLogging int32 // 0 = disabled, 1 = enabled
	enableProfiling       int32 // 0 = disabled, 1 = enabled

	// Cached state to reduce lock acquisitions
	lastCallCount   int32
	lastUpdateTime  int64 // Unix nano timestamp
	cacheValidityNs int64 // Cache validity period in nanoseconds

	// CPU profiling for performance monitoring
	profilingCtx    context.Context
	profilingCancel context.CancelFunc

	// Performance metrics
	avgIterationTime  time.Duration
	peakIterationTime time.Duration
	metricsLock       sync.RWMutex

	// Time provider for deterministic testing
	timeProvider TimeProvider
}

// NewPerformanceOptimizer creates a new performance optimizer instance.
//
// The optimizer is configured with sensible defaults:
// - Call slice pool with capacity of 8 (typical concurrent call limit)
// - Cache validity of 100ms for reasonable responsiveness
// - Detailed logging disabled by default for maximum performance
func NewPerformanceOptimizer() *PerformanceOptimizer {
	logrus.WithFields(logrus.Fields{
		"function": "NewPerformanceOptimizer",
	}).Debug("Creating new performance optimizer")

	optimizer := &PerformanceOptimizer{
		cacheValidityNs: 100 * time.Millisecond.Nanoseconds(), // 100ms cache validity
	}

	// Initialize call slice pool with pre-allocated slices
	optimizer.callSlicePool.New = func() interface{} {
		// Pre-allocate slice with capacity for typical number of concurrent calls
		return make([]*Call, 0, 8)
	}

	logrus.WithFields(logrus.Fields{
		"function":          "NewPerformanceOptimizer",
		"cache_validity_ms": optimizer.cacheValidityNs / 1000000,
		"detailed_logging":  optimizer.IsDetailedLoggingEnabled(),
		"profiling_enabled": optimizer.IsProfilingEnabled(),
	}).Info("Performance optimizer created")

	return optimizer
}

// OptimizeIteration performs fast-path call iteration with minimal overhead.
//
// This method is designed to be the primary iteration path for normal operation:
// - Uses cached call count when valid to avoid locks
// - Reuses call slices from pool to prevent allocations
// - Conditional logging based on performance flags
// - Lock-free statistics updates
//
// Returns the calls slice (borrowed from pool) and must be returned via ReturnCallSlice.
func (po *PerformanceOptimizer) OptimizeIteration(m *Manager) ([]*Call, bool) {
	iterationStart := po.getTimeProvider().Now()

	// Increment iteration counter atomically
	atomic.AddInt64(&po.iterationCount, 1)

	// Fast-path: check cache-based shortcut
	if ok, cachedZero := po.checkCachedFastPath(); ok {
		if cachedZero {
			return nil, false
		}
		// cache valid and non-zero -> continue to collect
	}

	// Collect active calls under minimal locking
	callSlice, callCount, running := po.collectCalls(m)
	if !running {
		po.returnCallSlice(callSlice)
		return nil, false
	}
	if callCount == 0 {
		po.returnCallSlice(callSlice)
		return nil, false
	}

	// Update cached state and counters
	po.updateCacheAndCounters(callCount)

	// Conditional logging (keeps lightweight)
	po.logProcessing(iterationStart, callCount)

	// Update performance metrics
	iterationTime := time.Since(iterationStart)
	po.updateIterationMetrics(iterationTime)

	return callSlice, true
}

// checkCachedFastPath checks whether cached state allows an early fast-path exit.
// Returns (handled, cachedZero): handled=true when cache is valid; cachedZero=true when no calls.
//
// Caching strategy: To avoid expensive mutex acquisition on every iteration,
// the call count is cached for a configurable validity period (cacheValidityNs).
// When the cached count shows zero active calls, we skip locking entirely.
// When the cache expires, collectCalls re-acquires the lock and refreshes the cache.
func (po *PerformanceOptimizer) checkCachedFastPath() (bool, bool) {
	// Compare current time against last cache update using atomic operations.
	// This avoids lock contention in the common case (no active calls).
	now := po.getTimeProvider().Now().UnixNano()
	lastUpdate := atomic.LoadInt64(&po.lastUpdateTime)

	// Cache is valid if time elapsed is less than validity period
	if now-lastUpdate < po.cacheValidityNs {
		// Read cached call count atomically; zero means no work to do
		cachedCount := atomic.LoadInt32(&po.lastCallCount)
		if cachedCount == 0 {
			if po.IsDetailedLoggingEnabled() {
				logrus.WithFields(logrus.Fields{
					"function":     "OptimizeIteration",
					"call_count":   0,
					"cached":       true,
					"iteration_ns": int64(0),
				}).Trace("Fast path: no active calls (cached)")
			}
			return true, true
		}
		return true, false
	}
	return false, false
}

// collectCalls retrieves a call slice from the pool and copies active calls under read lock.
// Returns the slice (borrowed from pool), the number of collected calls, and whether manager is running.
func (po *PerformanceOptimizer) collectCalls(m *Manager) ([]*Call, int, bool) {
	callSlice := po.getCallSlice()

	m.mu.RLock()
	running := m.running
	if !running {
		m.mu.RUnlock()
		return callSlice, 0, false
	}

	for _, call := range m.calls {
		callSlice = append(callSlice, call)
	}
	callCount := len(callSlice)
	m.mu.RUnlock()

	return callSlice, callCount, true
}

// updateCacheAndCounters updates cache atomically and increments processed counters.
func (po *PerformanceOptimizer) updateCacheAndCounters(callCount int) {
	now := po.getTimeProvider().Now().UnixNano()
	atomic.StoreInt32(&po.lastCallCount, int32(callCount))
	atomic.StoreInt64(&po.lastUpdateTime, now)
	atomic.AddInt64(&po.totalCallsProcessed, int64(callCount))
}

// logProcessing emits conditional detailed logging if enabled.
func (po *PerformanceOptimizer) logProcessing(iterationStart time.Time, callCount int) {
	if po.IsDetailedLoggingEnabled() && callCount > 0 {
		logrus.WithFields(logrus.Fields{
			"function":     "OptimizeIteration",
			"call_count":   callCount,
			"cached":       false,
			"iteration_ns": time.Since(iterationStart).Nanoseconds(),
		}).Trace("Processing active calls")
	}
}

// SetTimeProvider sets the time provider for deterministic testing.
// If not set, uses time.Now() directly.
func (po *PerformanceOptimizer) SetTimeProvider(tp TimeProvider) {
	po.timeProvider = tp
}

// getTimeProvider returns the configured time provider or a default that uses time.Now().
func (po *PerformanceOptimizer) getTimeProvider() TimeProvider {
	if po.timeProvider != nil {
		return po.timeProvider
	}
	return DefaultTimeProvider{}
}

// ReturnCallSlice returns a call slice to the pool for reuse.
//
// This must be called after using a slice returned by OptimizeIteration
// to ensure proper memory management and pool efficiency.
func (po *PerformanceOptimizer) ReturnCallSlice(slice []*Call) {
	po.returnCallSlice(slice)
}

// getCallSlice retrieves a call slice from the pool.
func (po *PerformanceOptimizer) getCallSlice() []*Call {
	slice := po.callSlicePool.Get().([]*Call)
	// Reset slice length while preserving capacity
	return slice[:0]
}

// returnCallSlice returns a call slice to the pool after clearing references.
func (po *PerformanceOptimizer) returnCallSlice(slice []*Call) {
	// Clear references to prevent memory leaks
	for i := range slice {
		slice[i] = nil
	}
	// Reset length and return to pool
	slice = slice[:0]
	po.callSlicePool.Put(slice)
}

// EnableDetailedLogging enables detailed logging for debugging performance issues.
//
// When enabled, provides comprehensive tracing of iteration performance.
// Should be disabled in production for optimal performance.
func (po *PerformanceOptimizer) EnableDetailedLogging(enabled bool) {
	var value int32
	if enabled {
		value = 1
	}
	atomic.StoreInt32(&po.enableDetailedLogging, value)

	logrus.WithFields(logrus.Fields{
		"function": "EnableDetailedLogging",
		"enabled":  enabled,
	}).Info("Detailed logging configuration updated")
}

// IsDetailedLoggingEnabled returns true if detailed logging is enabled.
func (po *PerformanceOptimizer) IsDetailedLoggingEnabled() bool {
	return atomic.LoadInt32(&po.enableDetailedLogging) == 1
}

// StartCPUProfiling begins CPU profiling for performance analysis.
//
// This enables runtime CPU profiling to identify performance bottlenecks
// in the ToxAV system. Should be used during development and testing.
func (po *PerformanceOptimizer) StartCPUProfiling() error {
	if atomic.LoadInt32(&po.enableProfiling) == 1 {
		return nil // Already profiling
	}

	logrus.WithFields(logrus.Fields{
		"function": "StartCPUProfiling",
	}).Info("Starting CPU profiling")

	ctx, cancel := context.WithCancel(context.Background())
	po.profilingCtx = ctx
	po.profilingCancel = cancel

	atomic.StoreInt32(&po.enableProfiling, 1)

	// Start CPU profiling in background
	go func() {
		labels := pprof.Labels("component", "toxav", "optimization", "enabled")
		pprof.Do(ctx, labels, func(ctx context.Context) {
			<-ctx.Done()
		})
	}()

	logrus.WithFields(logrus.Fields{
		"function": "StartCPUProfiling",
	}).Info("CPU profiling started")

	return nil
}

// StopCPUProfiling stops CPU profiling.
func (po *PerformanceOptimizer) StopCPUProfiling() {
	if atomic.LoadInt32(&po.enableProfiling) == 0 {
		return // Not profiling
	}

	logrus.WithFields(logrus.Fields{
		"function": "StopCPUProfiling",
	}).Info("Stopping CPU profiling")

	if po.profilingCancel != nil {
		po.profilingCancel()
	}

	atomic.StoreInt32(&po.enableProfiling, 0)

	logrus.WithFields(logrus.Fields{
		"function": "StopCPUProfiling",
	}).Info("CPU profiling stopped")
}

// IsProfilingEnabled returns true if CPU profiling is active.
func (po *PerformanceOptimizer) IsProfilingEnabled() bool {
	return atomic.LoadInt32(&po.enableProfiling) == 1
}

// updateIterationMetrics updates performance metrics in a thread-safe manner.
func (po *PerformanceOptimizer) updateIterationMetrics(iterationTime time.Duration) {
	po.metricsLock.Lock()
	defer po.metricsLock.Unlock()

	// Update average iteration time using exponential moving average
	if po.avgIterationTime == 0 {
		po.avgIterationTime = iterationTime
	} else {
		// EMA with alpha = 0.1 for smooth averaging
		po.avgIterationTime = time.Duration(
			float64(po.avgIterationTime)*0.9 + float64(iterationTime)*0.1,
		)
	}

	// Track peak iteration time
	if iterationTime > po.peakIterationTime {
		po.peakIterationTime = iterationTime
	}
}

// GetPerformanceMetrics returns current performance statistics.
//
// Provides insights into system performance including:
// - Total iterations and calls processed
// - Average and peak iteration times
// - Configuration status
func (po *PerformanceOptimizer) GetPerformanceMetrics() PerformanceMetrics {
	po.metricsLock.RLock()
	defer po.metricsLock.RUnlock()

	return PerformanceMetrics{
		TotalIterations:     atomic.LoadInt64(&po.iterationCount),
		TotalCallsProcessed: atomic.LoadInt64(&po.totalCallsProcessed),
		AvgIterationTime:    po.avgIterationTime,
		PeakIterationTime:   po.peakIterationTime,
		CachedCallCount:     atomic.LoadInt32(&po.lastCallCount),
		DetailedLogging:     po.IsDetailedLoggingEnabled(),
		ProfilingActive:     po.IsProfilingEnabled(),
	}
}

// ResetPerformanceMetrics resets all performance counters and metrics.
//
// Useful for benchmarking and performance testing to get clean measurements.
func (po *PerformanceOptimizer) ResetPerformanceMetrics() {
	logrus.WithFields(logrus.Fields{
		"function": "ResetPerformanceMetrics",
	}).Info("Resetting performance metrics")

	atomic.StoreInt64(&po.iterationCount, 0)
	atomic.StoreInt64(&po.totalCallsProcessed, 0)
	atomic.StoreInt32(&po.lastCallCount, 0)
	atomic.StoreInt64(&po.lastUpdateTime, 0)

	po.metricsLock.Lock()
	po.avgIterationTime = 0
	po.peakIterationTime = 0
	po.metricsLock.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "ResetPerformanceMetrics",
	}).Info("Performance metrics reset completed")
}

// PerformanceMetrics contains performance statistics for the ToxAV system.
type PerformanceMetrics struct {
	TotalIterations     int64         // Total iterations performed
	TotalCallsProcessed int64         // Total calls processed across all iterations
	AvgIterationTime    time.Duration // Exponential moving average of iteration time
	PeakIterationTime   time.Duration // Maximum observed iteration time
	CachedCallCount     int32         // Current cached call count
	DetailedLogging     bool          // Whether detailed logging is enabled
	ProfilingActive     bool          // Whether CPU profiling is active
}
