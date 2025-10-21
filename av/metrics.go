// Package av provides enhanced metrics reporting for ToxAV call quality monitoring.
//
// This file extends the existing quality monitoring infrastructure by adding
// aggregated metrics reporting, historical tracking, and dashboard-friendly APIs.
// It builds on top of quality.go without duplicating functionality.
package av

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrAlreadyRunning is returned when trying to start an already running service.
var ErrAlreadyRunning = errors.New("service is already running")

// MetricsAggregator provides aggregated metrics reporting across multiple calls.
//
// This component collects and aggregates metrics from the existing QualityMonitor
// to provide system-wide statistics, historical trends, and periodic reporting
// suitable for monitoring dashboards and analytics.
//
// Example usage:
//
//	aggregator := NewMetricsAggregator(5 * time.Second)
//	aggregator.OnReport(func(report AggregatedReport) {
//	    fmt.Printf("System quality: %s, Active calls: %d\n",
//	        report.OverallQuality, report.ActiveCalls)
//	})
//	aggregator.Start()
//	defer aggregator.Stop()
type MetricsAggregator struct {
	// Configuration
	reportInterval time.Duration

	// State management
	mu      sync.RWMutex
	running bool

	// Metrics storage
	callMetrics     map[uint32]*CallMetricsHistory // Key: friend number
	systemMetrics   *SystemMetrics
	historyDuration time.Duration // How long to keep history

	// Callbacks
	reportCallback func(report AggregatedReport)

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// CallMetricsHistory maintains historical metrics for a single call.
type CallMetricsHistory struct {
	FriendNumber   uint32
	CurrentMetrics CallMetrics
	History        []CallMetrics // Rolling window of past metrics
	MaxHistory     int           // Maximum history entries
}

// SystemMetrics contains system-wide aggregated metrics.
type SystemMetrics struct {
	// Call statistics
	ActiveCalls   int
	TotalCalls    uint64
	FailedCalls   uint64
	AverageDuration time.Duration

	// Network statistics
	AveragePacketLoss float64
	AverageJitter     time.Duration
	AverageBitrate    uint32

	// Quality distribution
	ExcellentCalls int
	GoodCalls      int
	FairCalls      int
	PoorCalls      int

	// Timestamp
	LastUpdate time.Time
}

// AggregatedReport contains aggregated metrics for periodic reporting.
type AggregatedReport struct {
	// System-wide metrics
	SystemMetrics SystemMetrics

	// Per-call metrics
	CallReports map[uint32]CallMetrics

	// Overall quality assessment
	OverallQuality QualityLevel

	// Report metadata
	Timestamp      time.Time
	ReportDuration time.Duration
}

// NewMetricsAggregator creates a new metrics aggregation system.
//
// The aggregator collects metrics from individual calls and provides
// system-wide statistics and reporting at the specified interval.
//
// Parameters:
//   - reportInterval: How often to generate aggregated reports
//
// Returns:
//   - *MetricsAggregator: New aggregator instance
func NewMetricsAggregator(reportInterval time.Duration) *MetricsAggregator {
	logrus.WithFields(logrus.Fields{
		"function":        "NewMetricsAggregator",
		"report_interval": reportInterval,
	}).Info("Creating new metrics aggregator")

	ctx, cancel := context.WithCancel(context.Background())

	aggregator := &MetricsAggregator{
		reportInterval:  reportInterval,
		running:         false,
		callMetrics:     make(map[uint32]*CallMetricsHistory),
		systemMetrics:   &SystemMetrics{},
		historyDuration: 5 * time.Minute,
		ctx:             ctx,
		cancel:          cancel,
	}

	logrus.WithFields(logrus.Fields{
		"function":         "NewMetricsAggregator",
		"report_interval":  reportInterval,
		"history_duration": aggregator.historyDuration,
	}).Info("Metrics aggregator created successfully")

	return aggregator
}

// Start begins the metrics aggregation and reporting service.
func (ma *MetricsAggregator) Start() error {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	if ma.running {
		return ErrAlreadyRunning
	}

	logrus.WithFields(logrus.Fields{
		"function": "MetricsAggregator.Start",
	}).Info("Starting metrics aggregator")

	ma.running = true

	// Start reporting goroutine
	go ma.reportLoop()

	logrus.WithFields(logrus.Fields{
		"function": "MetricsAggregator.Start",
	}).Info("Metrics aggregator started successfully")

	return nil
}

// Stop halts the metrics aggregation service.
func (ma *MetricsAggregator) Stop() {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	if !ma.running {
		return
	}

	logrus.WithFields(logrus.Fields{
		"function": "MetricsAggregator.Stop",
	}).Info("Stopping metrics aggregator")

	ma.running = false
	ma.cancel()

	logrus.WithFields(logrus.Fields{
		"function": "MetricsAggregator.Stop",
	}).Info("Metrics aggregator stopped successfully")
}

// IsRunning returns whether the aggregator is currently active.
func (ma *MetricsAggregator) IsRunning() bool {
	ma.mu.RLock()
	defer ma.mu.RUnlock()
	return ma.running
}

// OnReport registers a callback for periodic aggregated reports.
//
// The callback is invoked at the configured report interval with
// complete system-wide metrics and per-call statistics.
//
// Example:
//
//	aggregator.OnReport(func(report AggregatedReport) {
//	    log.Printf("System: %d active calls, %s quality",
//	        report.SystemMetrics.ActiveCalls,
//	        report.OverallQuality)
//	})
func (ma *MetricsAggregator) OnReport(callback func(report AggregatedReport)) {
	ma.mu.Lock()
	defer ma.mu.Unlock()
	ma.reportCallback = callback

	logrus.WithFields(logrus.Fields{
		"function": "MetricsAggregator.OnReport",
	}).Debug("Report callback registered")
}

// RecordMetrics records metrics for a specific call.
//
// This should be called periodically with updated metrics from QualityMonitor.
// The aggregator maintains history and updates system-wide statistics.
func (ma *MetricsAggregator) RecordMetrics(friendNumber uint32, metrics CallMetrics) {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	// Get or create history for this call
	history, exists := ma.callMetrics[friendNumber]
	if !exists {
		history = &CallMetricsHistory{
			FriendNumber: friendNumber,
			History:      make([]CallMetrics, 0, 60), // 5 minutes at 5-second intervals
			MaxHistory:   60,
		}
		ma.callMetrics[friendNumber] = history
	}

	// Update current metrics
	history.CurrentMetrics = metrics

	// Add to history (maintaining rolling window)
	history.History = append(history.History, metrics)
	if len(history.History) > history.MaxHistory {
		history.History = history.History[1:] // Remove oldest
	}

	// Update system metrics
	ma.updateSystemMetrics()

	if logrus.IsLevelEnabled(logrus.TraceLevel) {
		logrus.WithFields(logrus.Fields{
			"function":        "MetricsAggregator.RecordMetrics",
			"friend_number":   friendNumber,
			"quality":         metrics.Quality.String(),
			"history_entries": len(history.History),
		}).Trace("Metrics recorded")
	}
}

// StartCallTracking begins tracking a new call.
//
// This initializes metrics history for the call and updates system statistics.
func (ma *MetricsAggregator) StartCallTracking(friendNumber uint32) {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":      "MetricsAggregator.StartCallTracking",
		"friend_number": friendNumber,
	}).Info("Starting call tracking")

	// Initialize history if not exists
	if _, exists := ma.callMetrics[friendNumber]; !exists {
		ma.callMetrics[friendNumber] = &CallMetricsHistory{
			FriendNumber: friendNumber,
			History:      make([]CallMetrics, 0, 60),
			MaxHistory:   60,
		}
	}

	// Update system metrics
	ma.systemMetrics.TotalCalls++
	ma.updateSystemMetrics()
}

// StopCallTracking stops tracking a call.
//
// This removes the call from active tracking and updates system statistics.
// Historical data is preserved for the configured duration.
func (ma *MetricsAggregator) StopCallTracking(friendNumber uint32) {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":      "MetricsAggregator.StopCallTracking",
		"friend_number": friendNumber,
	}).Info("Stopping call tracking")

	// Remove from active tracking (history is cleaned up by reportLoop)
	delete(ma.callMetrics, friendNumber)

	// Update system metrics
	ma.updateSystemMetrics()
}

// GetSystemMetrics returns current system-wide metrics.
//
// Returns a snapshot of aggregated metrics across all active calls.
func (ma *MetricsAggregator) GetSystemMetrics() SystemMetrics {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	// Return a copy
	return *ma.systemMetrics
}

// GetCallHistory returns historical metrics for a specific call.
//
// Returns nil if the call is not being tracked or has no history.
func (ma *MetricsAggregator) GetCallHistory(friendNumber uint32) []CallMetrics {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	history, exists := ma.callMetrics[friendNumber]
	if !exists {
		return nil
	}

	// Return a copy of history
	historyCopy := make([]CallMetrics, len(history.History))
	copy(historyCopy, history.History)
	return historyCopy
}

// reportLoop runs the periodic aggregated reporting.
func (ma *MetricsAggregator) reportLoop() {
	ticker := time.NewTicker(ma.reportInterval)
	defer ticker.Stop()

	logrus.WithFields(logrus.Fields{
		"function": "MetricsAggregator.reportLoop",
		"interval": ma.reportInterval,
	}).Debug("Starting aggregated report loop")

	for {
		select {
		case <-ma.ctx.Done():
			logrus.WithFields(logrus.Fields{
				"function": "MetricsAggregator.reportLoop",
			}).Debug("Report loop stopped")
			return

		case <-ticker.C:
			ma.generateReport()
		}
	}
}

// generateReport creates and dispatches an aggregated report.
func (ma *MetricsAggregator) generateReport() {
	ma.mu.RLock()
	callback := ma.reportCallback
	if callback == nil {
		ma.mu.RUnlock()
		return
	}

	// Build report
	report := AggregatedReport{
		SystemMetrics:  *ma.systemMetrics,
		CallReports:    make(map[uint32]CallMetrics),
		Timestamp:      time.Now(),
		ReportDuration: ma.reportInterval,
	}

	// Collect current metrics for all calls
	for friendNumber, history := range ma.callMetrics {
		report.CallReports[friendNumber] = history.CurrentMetrics
	}

	// Calculate overall quality
	report.OverallQuality = ma.calculateOverallQuality()

	ma.mu.RUnlock()

	// Invoke callback asynchronously
	go callback(report)

	logrus.WithFields(logrus.Fields{
		"function":      "MetricsAggregator.generateReport",
		"active_calls":  report.SystemMetrics.ActiveCalls,
		"overall_quality": report.OverallQuality.String(),
	}).Debug("Generated aggregated report")
}

// updateSystemMetrics recalculates system-wide metrics.
func (ma *MetricsAggregator) updateSystemMetrics() {
	// Count active calls
	ma.systemMetrics.ActiveCalls = len(ma.callMetrics)

	if ma.systemMetrics.ActiveCalls == 0 {
		return
	}

	// Aggregate metrics across all calls
	var totalPacketLoss float64
	var totalJitter time.Duration
	var totalBitrate uint64
	var totalDuration time.Duration

	// Quality distribution
	ma.systemMetrics.ExcellentCalls = 0
	ma.systemMetrics.GoodCalls = 0
	ma.systemMetrics.FairCalls = 0
	ma.systemMetrics.PoorCalls = 0

	for _, history := range ma.callMetrics {
		metrics := history.CurrentMetrics

		totalPacketLoss += metrics.PacketLoss
		totalJitter += metrics.Jitter
		totalBitrate += uint64(metrics.AudioBitRate + metrics.VideoBitRate)
		totalDuration += metrics.CallDuration

		// Count quality levels
		switch metrics.Quality {
		case QualityExcellent:
			ma.systemMetrics.ExcellentCalls++
		case QualityGood:
			ma.systemMetrics.GoodCalls++
		case QualityFair:
			ma.systemMetrics.FairCalls++
		case QualityPoor, QualityUnacceptable:
			ma.systemMetrics.PoorCalls++
		}
	}

	// Calculate averages
	count := float64(ma.systemMetrics.ActiveCalls)
	ma.systemMetrics.AveragePacketLoss = totalPacketLoss / count
	ma.systemMetrics.AverageJitter = time.Duration(int64(totalJitter) / int64(ma.systemMetrics.ActiveCalls))
	ma.systemMetrics.AverageBitrate = uint32(totalBitrate / uint64(ma.systemMetrics.ActiveCalls))

	if ma.systemMetrics.ActiveCalls > 0 {
		ma.systemMetrics.AverageDuration = time.Duration(int64(totalDuration) / int64(ma.systemMetrics.ActiveCalls))
	}

	ma.systemMetrics.LastUpdate = time.Now()
}

// calculateOverallQuality determines overall system quality.
//
// This assesses the distribution of call qualities across all active calls
// to provide a single system-wide quality indicator.
func (ma *MetricsAggregator) calculateOverallQuality() QualityLevel {
	if ma.systemMetrics.ActiveCalls == 0 {
		return QualityExcellent // No calls = excellent state
	}

	// If majority are poor, overall is poor
	if ma.systemMetrics.PoorCalls > ma.systemMetrics.ActiveCalls/2 {
		return QualityPoor
	}

	// If majority are fair or worse, overall is fair
	if (ma.systemMetrics.FairCalls + ma.systemMetrics.PoorCalls) > ma.systemMetrics.ActiveCalls/2 {
		return QualityFair
	}

	// If majority are good or better, overall is good
	if (ma.systemMetrics.GoodCalls + ma.systemMetrics.ExcellentCalls) > ma.systemMetrics.ActiveCalls/2 {
		// If most good calls are excellent, overall is excellent
		if ma.systemMetrics.ExcellentCalls > ma.systemMetrics.GoodCalls {
			return QualityExcellent
		}
		return QualityGood
	}

	// Default to good
	return QualityGood
}

// GetActiveCallCount returns the number of currently tracked calls.
func (ma *MetricsAggregator) GetActiveCallCount() int {
	ma.mu.RLock()
	defer ma.mu.RUnlock()
	return ma.systemMetrics.ActiveCalls
}

// GetTotalCallCount returns the total number of calls tracked since start.
func (ma *MetricsAggregator) GetTotalCallCount() uint64 {
	ma.mu.RLock()
	defer ma.mu.RUnlock()
	return ma.systemMetrics.TotalCalls
}
