package crypto

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// PerformanceMonitor tracks cryptographic and protocol performance metrics
//
//export ToxPerformanceMonitor
type PerformanceMonitor struct {
	handshakeMetrics    *HandshakeMetrics
	encryptionMetrics   *EncryptionMetrics
	sessionMetrics      *SessionMetrics
	systemMetrics       *SystemMetrics
	mu                  sync.RWMutex
	startTime           time.Time
	samplingInterval    time.Duration
}

// HandshakeMetrics tracks Noise handshake performance
//
//export ToxHandshakeMetrics
type HandshakeMetrics struct {
	TotalHandshakes       uint64        `json:"total_handshakes"`
	SuccessfulHandshakes  uint64        `json:"successful_handshakes"`
	FailedHandshakes      uint64        `json:"failed_handshakes"`
	AverageLatency        float64       `json:"average_latency_ms"`
	MinLatency            float64       `json:"min_latency_ms"`
	MaxLatency            float64       `json:"max_latency_ms"`
	HandshakesPerSecond   float64       `json:"handshakes_per_second"`
	LastHandshake         time.Time     `json:"last_handshake"`
	LatencyDistribution   []float64     `json:"latency_distribution"`
}

// EncryptionMetrics tracks encryption/decryption performance
//
//export ToxEncryptionMetrics
type EncryptionMetrics struct {
	OperationsPerformed   uint64        `json:"operations_performed"`
	BytesProcessed        uint64        `json:"bytes_processed"`
	AverageLatency        float64       `json:"average_latency_us"`
	ThroughputBytesPerSec float64       `json:"throughput_bytes_per_sec"`
	EncryptionErrors      uint64        `json:"encryption_errors"`
	DecryptionErrors      uint64        `json:"decryption_errors"`
	LastOperation         time.Time     `json:"last_operation"`
}

// SystemMetrics tracks system resource usage
//
//export ToxSystemMetrics
type SystemMetrics struct {
	CPUUsage              float64       `json:"cpu_usage_percent"`
	MemoryUsage           uint64        `json:"memory_usage_bytes"`
	GoroutineCount        int           `json:"goroutine_count"`
	GCPauses              []float64     `json:"gc_pause_times_ms"`
	AllocRate             float64       `json:"alloc_rate_bytes_per_sec"`
	HeapSize              uint64        `json:"heap_size_bytes"`
	LastGCTime            time.Time     `json:"last_gc_time"`
}

// PerformanceDashboard provides a comprehensive view of system performance
//
//export ToxPerformanceDashboard
type PerformanceDashboard struct {
	monitor         *PerformanceMonitor
	updateInterval  time.Duration
	stopChannel     chan struct{}
	alertCallbacks  []AlertCallback
}

// AlertCallback is called when performance alerts are triggered
type AlertCallback func(alert PerformanceAlert)

// PerformanceAlert represents a performance-related alert
//
//export ToxPerformanceAlert
type PerformanceAlert struct {
	AlertType   PerformanceAlertType `json:"alert_type"`
	Severity    AlertSeverity        `json:"severity"`
	Message     string              `json:"message"`
	Timestamp   time.Time           `json:"timestamp"`
	Value       float64             `json:"value"`
	Threshold   float64             `json:"threshold"`
	Suggestions []string            `json:"suggestions"`
}

// PerformanceAlertType categorizes performance alerts
type PerformanceAlertType int

const (
	AlertHighLatency PerformanceAlertType = iota
	AlertHighMemoryUsage
	AlertHighCPUUsage
	AlertHighErrorRate
	AlertLowThroughput
	AlertFrequentGC
)

// NewPerformanceMonitor creates a new performance monitor
//
//export ToxNewPerformanceMonitor
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		handshakeMetrics: &HandshakeMetrics{
			MinLatency:          999999.0, // Initialize to high value
			LatencyDistribution: make([]float64, 0, 1000),
		},
		encryptionMetrics: &EncryptionMetrics{},
		sessionMetrics:    &SessionMetrics{},
		systemMetrics:     &SystemMetrics{},
		startTime:         time.Now(),
		samplingInterval:  time.Second,
	}
}

// RecordHandshake records a handshake completion
//
//export ToxRecordHandshake
func (pm *PerformanceMonitor) RecordHandshake(duration time.Duration, success bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	latencyMs := float64(duration.Nanoseconds()) / 1e6
	
	pm.handshakeMetrics.TotalHandshakes++
	if success {
		pm.handshakeMetrics.SuccessfulHandshakes++
	} else {
		pm.handshakeMetrics.FailedHandshakes++
	}
	
	// Update latency statistics
	if latencyMs < pm.handshakeMetrics.MinLatency {
		pm.handshakeMetrics.MinLatency = latencyMs
	}
	if latencyMs > pm.handshakeMetrics.MaxLatency {
		pm.handshakeMetrics.MaxLatency = latencyMs
	}
	
	// Update average latency (exponential moving average)
	if pm.handshakeMetrics.AverageLatency == 0 {
		pm.handshakeMetrics.AverageLatency = latencyMs
	} else {
		pm.handshakeMetrics.AverageLatency = 0.9*pm.handshakeMetrics.AverageLatency + 0.1*latencyMs
	}
	
	pm.handshakeMetrics.LastHandshake = time.Now()
	
	// Add to latency distribution (keep last 1000 samples)
	pm.handshakeMetrics.LatencyDistribution = append(pm.handshakeMetrics.LatencyDistribution, latencyMs)
	if len(pm.handshakeMetrics.LatencyDistribution) > 1000 {
		pm.handshakeMetrics.LatencyDistribution = pm.handshakeMetrics.LatencyDistribution[1:]
	}
	
	// Calculate handshakes per second
	elapsed := time.Since(pm.startTime).Seconds()
	if elapsed > 0 {
		pm.handshakeMetrics.HandshakesPerSecond = float64(pm.handshakeMetrics.TotalHandshakes) / elapsed
	}
}

// RecordEncryption records an encryption/decryption operation
//
//export ToxRecordEncryption
func (pm *PerformanceMonitor) RecordEncryption(duration time.Duration, bytes int, success bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	latencyUs := float64(duration.Nanoseconds()) / 1e3
	
	pm.encryptionMetrics.OperationsPerformed++
	pm.encryptionMetrics.BytesProcessed += uint64(bytes)
	
	if !success {
		pm.encryptionMetrics.EncryptionErrors++
	}
	
	// Update average latency
	if pm.encryptionMetrics.AverageLatency == 0 {
		pm.encryptionMetrics.AverageLatency = latencyUs
	} else {
		pm.encryptionMetrics.AverageLatency = 0.9*pm.encryptionMetrics.AverageLatency + 0.1*latencyUs
	}
	
	pm.encryptionMetrics.LastOperation = time.Now()
	
	// Calculate throughput
	elapsed := time.Since(pm.startTime).Seconds()
	if elapsed > 0 {
		pm.encryptionMetrics.ThroughputBytesPerSec = float64(pm.encryptionMetrics.BytesProcessed) / elapsed
	}
}

// UpdateSystemMetrics updates system performance metrics
//
//export ToxUpdateSystemMetrics
func (pm *PerformanceMonitor) UpdateSystemMetrics() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	pm.systemMetrics.MemoryUsage = m.Alloc
	pm.systemMetrics.HeapSize = m.HeapAlloc
	pm.systemMetrics.GoroutineCount = runtime.NumGoroutine()
	
	// Calculate allocation rate
	elapsed := time.Since(pm.startTime).Seconds()
	if elapsed > 0 {
		pm.systemMetrics.AllocRate = float64(m.TotalAlloc) / elapsed
	}
	
	// Track GC pause times (keep last 100 pauses)
	if len(m.PauseNs) > 0 {
		lastPause := float64(m.PauseNs[(m.NumGC+255)%256]) / 1e6
		pm.systemMetrics.GCPauses = append(pm.systemMetrics.GCPauses, lastPause)
		if len(pm.systemMetrics.GCPauses) > 100 {
			pm.systemMetrics.GCPauses = pm.systemMetrics.GCPauses[1:]
		}
	}
}

// GetMetrics returns all performance metrics
//
//export ToxGetPerformanceMetrics
func (pm *PerformanceMonitor) GetMetrics() (handshake *HandshakeMetrics, encryption *EncryptionMetrics, system *SystemMetrics) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	// Return copies to avoid race conditions
	handshakeCopy := *pm.handshakeMetrics
	encryptionCopy := *pm.encryptionMetrics
	systemCopy := *pm.systemMetrics
	
	return &handshakeCopy, &encryptionCopy, &systemCopy
}

// CheckPerformanceAlerts checks for performance issues and returns alerts
//
//export ToxCheckPerformanceAlerts
func (pm *PerformanceMonitor) CheckPerformanceAlerts() []PerformanceAlert {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	var alerts []PerformanceAlert
	now := time.Now()
	
	// Check handshake latency
	if pm.handshakeMetrics.AverageLatency > 1000.0 { // 1 second threshold
		alerts = append(alerts, PerformanceAlert{
			AlertType: AlertHighLatency,
			Severity:  SeverityWarning,
			Message:   fmt.Sprintf("High handshake latency: %.2fms", pm.handshakeMetrics.AverageLatency),
			Timestamp: now,
			Value:     pm.handshakeMetrics.AverageLatency,
			Threshold: 1000.0,
			Suggestions: []string{
				"Check network connectivity",
				"Consider handshake caching",
				"Monitor CPU usage",
			},
		})
	}
	
	// Check memory usage
	if pm.systemMetrics.MemoryUsage > 100*1024*1024 { // 100MB threshold
		alerts = append(alerts, PerformanceAlert{
			AlertType: AlertHighMemoryUsage,
			Severity:  SeverityWarning,
			Message:   fmt.Sprintf("High memory usage: %.2fMB", float64(pm.systemMetrics.MemoryUsage)/1024/1024),
			Timestamp: now,
			Value:     float64(pm.systemMetrics.MemoryUsage),
			Threshold: 100*1024*1024,
			Suggestions: []string{
				"Review session cleanup policies",
				"Check for memory leaks",
				"Consider reducing cache sizes",
			},
		})
	}
	
	// Check error rates
	if pm.handshakeMetrics.TotalHandshakes > 0 {
		errorRate := float64(pm.handshakeMetrics.FailedHandshakes) / float64(pm.handshakeMetrics.TotalHandshakes)
		if errorRate > 0.05 { // 5% error rate threshold
			alerts = append(alerts, PerformanceAlert{
				AlertType: AlertHighErrorRate,
				Severity:  SeverityError,
				Message:   fmt.Sprintf("High handshake error rate: %.2f%%", errorRate*100),
				Timestamp: now,
				Value:     errorRate,
				Threshold: 0.05,
				Suggestions: []string{
					"Review network stability",
					"Check peer compatibility",
					"Validate configuration",
				},
			})
		}
	}
	
	return alerts
}

// NewPerformanceDashboard creates a new performance dashboard
//
//export ToxNewPerformanceDashboard
func NewPerformanceDashboard(monitor *PerformanceMonitor) *PerformanceDashboard {
	return &PerformanceDashboard{
		monitor:        monitor,
		updateInterval: 5 * time.Second,
		stopChannel:    make(chan struct{}),
		alertCallbacks: make([]AlertCallback, 0),
	}
}

// Start starts the performance dashboard monitoring
//
//export ToxStartDashboard
func (pd *PerformanceDashboard) Start() {
	go pd.monitoringLoop()
}

// Stop stops the performance dashboard
//
//export ToxStopDashboard
func (pd *PerformanceDashboard) Stop() {
	close(pd.stopChannel)
}

// AddAlertCallback adds a callback for performance alerts
//
//export ToxAddAlertCallback
func (pd *PerformanceDashboard) AddAlertCallback(callback AlertCallback) {
	pd.alertCallbacks = append(pd.alertCallbacks, callback)
}

// monitoringLoop runs the main monitoring loop
func (pd *PerformanceDashboard) monitoringLoop() {
	ticker := time.NewTicker(pd.updateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-pd.stopChannel:
			return
		case <-ticker.C:
			pd.monitor.UpdateSystemMetrics()
			alerts := pd.monitor.CheckPerformanceAlerts()
			
			// Trigger alert callbacks
			for _, alert := range alerts {
				for _, callback := range pd.alertCallbacks {
					go callback(alert)
				}
			}
		}
	}
}

// GenerateReport generates a comprehensive performance report
//
//export ToxGeneratePerformanceReport
func (pd *PerformanceDashboard) GenerateReport() (*PerformanceReport, error) {
	handshake, encryption, system := pd.monitor.GetMetrics()
	
	report := &PerformanceReport{
		GeneratedAt:       time.Now(),
		Uptime:           time.Since(pd.monitor.startTime),
		HandshakeMetrics: handshake,
		EncryptionMetrics: encryption,
		SystemMetrics:    system,
		Alerts:          pd.monitor.CheckPerformanceAlerts(),
	}
	
	return report, nil
}

// PerformanceReport provides a comprehensive performance overview
//
//export ToxPerformanceReport
type PerformanceReport struct {
	GeneratedAt       time.Time             `json:"generated_at"`
	Uptime           time.Duration         `json:"uptime"`
	HandshakeMetrics *HandshakeMetrics     `json:"handshake_metrics"`
	EncryptionMetrics *EncryptionMetrics   `json:"encryption_metrics"`
	SystemMetrics    *SystemMetrics        `json:"system_metrics"`
	Alerts           []PerformanceAlert    `json:"current_alerts"`
}

// ExportJSON exports the performance report as JSON
//
//export ToxExportPerformanceJSON
func (pr *PerformanceReport) ExportJSON() ([]byte, error) {
	return json.MarshalIndent(pr, "", "  ")
}
