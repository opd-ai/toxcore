// Package transport implements network monitoring and performance tracking for the Tox protocol.
//
// The network monitor provides real-time metrics about connection health, throughput,
// latency, and error rates to help diagnose network issues and optimize performance.
//
// Example usage:
//
//	monitor := transport.NewNetworkMonitor()
//	monitor.Start()
//	defer monitor.Stop()
//
//	// Record network activity
//	monitor.RecordPacketSent(packetSize)
//	monitor.RecordPacketReceived(packetSize, latency)
//
//	// Get current metrics
//	metrics := monitor.GetMetrics()
//	fmt.Printf("Average latency: %.2fms\n", metrics.AverageLatency)
package transport

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// NetworkMonitor tracks network performance and health metrics for Tox connections.
// It provides real-time monitoring of throughput, latency, packet loss, and connection health
// to help diagnose network issues and optimize performance.
//
//export ToxNetworkMonitor
type NetworkMonitor struct {
	metrics          *NetworkMetrics
	connectionHealth map[string]*ConnectionHealth
	alertThresholds  *AlertThresholds
	mu               sync.RWMutex
	startTime        time.Time
	lastUpdate       time.Time
}

// NetworkMetrics aggregates overall network performance data across all connections.
// These metrics are used to assess the overall health of the Tox network connectivity
// and identify performance bottlenecks or connectivity issues.
//
//export ToxNetworkMetrics
type NetworkMetrics struct {
	// Throughput metrics track data transfer rates
	BytesSent       uint64 `json:"bytes_sent"`
	BytesReceived   uint64 `json:"bytes_received"`
	PacketsSent     uint64 `json:"packets_sent"`
	PacketsReceived uint64 `json:"packets_received"`

	// Performance metrics measure network quality
	AverageLatency float64 `json:"average_latency_ms"`
	PacketLossRate float64 `json:"packet_loss_rate"`
	Throughput     float64 `json:"throughput_bps"`

	// Connection metrics track active and historical connections
	ActiveConnections int    `json:"active_connections"`
	FailedConnections uint64 `json:"failed_connections"`
	TotalConnections  uint64 `json:"total_connections"`

	// Error metrics help identify network problems
	NetworkErrors  uint64 `json:"network_errors"`
	ProtocolErrors uint64 `json:"protocol_errors"`
	TimeoutErrors  uint64 `json:"timeout_errors"`

	// Timing information for metric calculation
	Uptime      float64   `json:"uptime_seconds"`
	LastUpdated time.Time `json:"last_updated"`
}

// ConnectionHealth tracks health metrics for individual connections to specific peers.
// This enables per-connection monitoring and troubleshooting of specific network paths.
//
//export ToxConnectionHealth
type ConnectionHealth struct {
	ConnectionID  string    `json:"connection_id"`
	RemoteAddr    string    `json:"remote_addr"`
	State         string    `json:"state"`
	LastSeen      time.Time `json:"last_seen"`
	RTT           float64   `json:"rtt_ms"`
	PacketLoss    float64   `json:"packet_loss_rate"`
	BytesSent     uint64    `json:"bytes_sent"`
	BytesReceived uint64    `json:"bytes_received"`
	ErrorCount    uint64    `json:"error_count"`
	QualityScore  float64   `json:"quality_score"` // 0-100 scale where 100 is perfect
}

// AlertThresholds defines when to trigger network alerts for various metrics.
// These thresholds help automatically detect network performance degradation.
//
//export ToxAlertThresholds
type AlertThresholds struct {
	MaxLatency        float64       `json:"max_latency_ms"`
	MaxPacketLoss     float64       `json:"max_packet_loss_rate"`
	MinThroughput     float64       `json:"min_throughput_bps"`
	MaxErrorRate      float64       `json:"max_error_rate"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
}

// NetworkAlert represents a network health alert triggered when metrics exceed thresholds.
// Alerts help administrators and applications respond to network issues automatically.
//
//export ToxNetworkAlert
type NetworkAlert struct {
	AlertType    AlertType     `json:"alert_type"`
	Severity     AlertSeverity `json:"severity"`
	Message      string        `json:"message"`
	Timestamp    time.Time     `json:"timestamp"`
	MetricValue  float64       `json:"metric_value"`
	Threshold    float64       `json:"threshold"`
	ConnectionID string        `json:"connection_id,omitempty"`
}

// AlertType categorizes different types of network alerts for proper handling and filtering.
type AlertType int

const (
	// High latency detected
	AlertLatencyHigh AlertType = iota
	// Packet loss rate exceeds threshold
	AlertPacketLossHigh
	// Throughput below minimum threshold
	AlertThroughputLow
	// Connection establishment failed
	AlertConnectionFailed
	// Error rate exceeds acceptable level
	AlertErrorRateHigh
	// Connection timed out
	AlertConnectionTimeout
)

// AlertSeverity indicates the severity level of a network alert for prioritization.
type AlertSeverity int

const (
	// Informational message, no action required
	SeverityInfo AlertSeverity = iota
	// Warning condition, monitoring recommended
	SeverityWarning
	// Error condition, action may be required
	SeverityError
	// Critical condition, immediate action required
	SeverityCritical
)

// NewNetworkMonitor creates a new network monitor with default settings.
// The monitor starts with empty metrics and default alert thresholds.
//
//export ToxNewNetworkMonitor
func NewNetworkMonitor() *NetworkMonitor {
	return &NetworkMonitor{
		metrics: &NetworkMetrics{
			LastUpdated: time.Now(),
		},
		connectionHealth: make(map[string]*ConnectionHealth),
		alertThresholds: &AlertThresholds{
			MaxLatency:        1000.0, // ADDED: 1 second maximum latency threshold
			MaxPacketLoss:     0.05,   // ADDED: 5% maximum packet loss threshold
			MinThroughput:     1024,   // ADDED: 1 KB/s minimum throughput threshold
			MaxErrorRate:      0.01,   // ADDED: 1% maximum error rate threshold
			ConnectionTimeout: 30 * time.Second,
		},
		startTime:  time.Now(),
		lastUpdate: time.Now(),
	}
}

// ADDED: RecordPacketSent records metrics for a sent packet including size and connection tracking.
// This method updates both global metrics and per-connection statistics for monitoring.
//
//export ToxRecordPacketSent
func (nm *NetworkMonitor) RecordPacketSent(connectionID string, size int) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// ADDED: Update global packet and byte counters
	nm.metrics.PacketsSent++
	nm.metrics.BytesSent += uint64(size)

	// ADDED: Update per-connection statistics if connection exists
	if health, exists := nm.connectionHealth[connectionID]; exists {
		health.BytesSent += uint64(size)
	}

	// ADDED: Recalculate throughput based on recent activity
	nm.updateThroughput()
}

// ADDED: RecordPacketReceived records metrics for a received packet including latency measurement.
// Latency is tracked using exponential moving average for smooth trend analysis.
//
//export ToxRecordPacketReceived
func (nm *NetworkMonitor) RecordPacketReceived(connectionID string, size int, latency time.Duration) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// ADDED: Update global receive counters
	nm.metrics.PacketsReceived++
	nm.metrics.BytesReceived += uint64(size)

	// ADDED: Calculate and update latency using exponential moving average for smoothing
	latencyMs := float64(latency.Nanoseconds()) / 1e6
	if nm.metrics.AverageLatency == 0 {
		nm.metrics.AverageLatency = latencyMs
	} else {
		// ADDED: Use exponential moving average (90% old, 10% new) for smooth latency tracking
		nm.metrics.AverageLatency = 0.9*nm.metrics.AverageLatency + 0.1*latencyMs
	}

	// ADDED: Update per-connection health metrics if connection exists
	if health, exists := nm.connectionHealth[connectionID]; exists {
		health.BytesReceived += uint64(size)
		health.RTT = latencyMs
		health.LastSeen = time.Now()
	}

	// ADDED: Recalculate overall throughput metrics
	nm.updateThroughput()
}

// ADDED: RecordConnectionEstablished records a new connection establishment with initial health metrics.
// This creates tracking for the new connection and updates global connection counters.
//
//export ToxRecordConnectionEstablished
func (nm *NetworkMonitor) RecordConnectionEstablished(connectionID, remoteAddr string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// ADDED: Increment global connection counters
	nm.metrics.TotalConnections++
	nm.metrics.ActiveConnections++

	// ADDED: Initialize health tracking for the new connection with perfect initial score
	nm.connectionHealth[connectionID] = &ConnectionHealth{
		ConnectionID: connectionID,
		RemoteAddr:   remoteAddr,
		State:        "connected",
		LastSeen:     time.Now(),
		QualityScore: 100.0, // ADDED: Start with perfect quality score
	}
}

// ADDED: RecordConnectionClosed records a connection closure and updates tracking accordingly.
// The connection health is marked as closed but retained for historical analysis.
//
//export ToxRecordConnectionClosed
func (nm *NetworkMonitor) RecordConnectionClosed(connectionID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// ADDED: Update connection state and decrement active counter
	if health, exists := nm.connectionHealth[connectionID]; exists {
		health.State = "closed"
		nm.metrics.ActiveConnections--
	}
}

// ADDED: RecordError records a network error by type and updates relevant counters.
// Errors are categorized for better debugging and the connection quality score is updated.
//
//export ToxRecordError
func (nm *NetworkMonitor) RecordError(connectionID string, errorType string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// ADDED: Categorize and count different types of errors for analysis
	switch errorType {
	case "network":
		nm.metrics.NetworkErrors++
	case "protocol":
		nm.metrics.ProtocolErrors++
	case "timeout":
		nm.metrics.TimeoutErrors++
	}

	// ADDED: Update per-connection error tracking and recalculate quality score
	if health, exists := nm.connectionHealth[connectionID]; exists {
		health.ErrorCount++
		nm.updateConnectionQuality(health)
	}
}

// ADDED: GetMetrics returns a snapshot of current network metrics including calculated uptime.
// This method returns a copy to prevent external modification of internal metrics.
//
//export ToxGetNetworkMetrics
func (nm *NetworkMonitor) GetMetrics() *NetworkMetrics {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// ADDED: Calculate current uptime since monitor started
	nm.metrics.Uptime = time.Since(nm.startTime).Seconds()
	nm.metrics.LastUpdated = time.Now()

	// ADDED: Return a copy to prevent external modification
	metricsCopy := *nm.metrics
	return &metricsCopy
}

// ADDED: GetConnectionHealth returns health status for all tracked connections.
// This provides per-connection visibility into network performance and issues.
//
//export ToxGetConnectionHealth
func (nm *NetworkMonitor) GetConnectionHealth() map[string]*ConnectionHealth {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// Return copies to avoid race conditions
	healthCopy := make(map[string]*ConnectionHealth)
	for id, health := range nm.connectionHealth {
		healthCopyItem := *health
		healthCopy[id] = &healthCopyItem
	}

	return healthCopy
}

// ADDED: CheckAlerts analyzes current network metrics and returns alerts for conditions exceeding thresholds.
// This method checks both global metrics and per-connection health to identify network issues.
//
//export ToxCheckAlerts
func (nm *NetworkMonitor) CheckAlerts() []NetworkAlert {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var alerts []NetworkAlert
	now := time.Now()

	// ADDED: Check global latency threshold
	if nm.metrics.AverageLatency > nm.alertThresholds.MaxLatency {
		alerts = append(alerts, NetworkAlert{
			AlertType:   AlertLatencyHigh,
			Severity:    SeverityWarning,
			Message:     fmt.Sprintf("High average latency: %.2fms", nm.metrics.AverageLatency),
			Timestamp:   now,
			MetricValue: nm.metrics.AverageLatency,
			Threshold:   nm.alertThresholds.MaxLatency,
		})
	}

	// ADDED: Check global packet loss threshold
	if nm.metrics.PacketLossRate > nm.alertThresholds.MaxPacketLoss {
		alerts = append(alerts, NetworkAlert{
			AlertType:   AlertPacketLossHigh,
			Severity:    SeverityError,
			Message:     fmt.Sprintf("High packet loss: %.2f%%", nm.metrics.PacketLossRate*100),
			Timestamp:   now,
			MetricValue: nm.metrics.PacketLossRate,
			Threshold:   nm.alertThresholds.MaxPacketLoss,
		})
	}

	// ADDED: Check global throughput threshold
	if nm.metrics.Throughput < nm.alertThresholds.MinThroughput {
		alerts = append(alerts, NetworkAlert{
			AlertType:   AlertThroughputLow,
			Severity:    SeverityWarning,
			Message:     fmt.Sprintf("Low throughput: %.2f bps", nm.metrics.Throughput),
			Timestamp:   now,
			MetricValue: nm.metrics.Throughput,
			Threshold:   nm.alertThresholds.MinThroughput,
		})
	}

	// ADDED: Check individual connection health metrics
	for id, health := range nm.connectionHealth {
		// ADDED: Check for connection timeout
		if time.Since(health.LastSeen) > nm.alertThresholds.ConnectionTimeout {
			alerts = append(alerts, NetworkAlert{
				AlertType:    AlertConnectionTimeout,
				Severity:     SeverityError,
				Message:      fmt.Sprintf("Connection timeout: %s", id),
				Timestamp:    now,
				ConnectionID: id,
			})
		}

		// ADDED: Check for poor connection quality (below 50%)
		if health.QualityScore < 50.0 {
			alerts = append(alerts, NetworkAlert{
				AlertType:    AlertConnectionFailed,
				Severity:     SeverityWarning,
				Message:      fmt.Sprintf("Poor connection quality: %.1f/100", health.QualityScore),
				Timestamp:    now,
				MetricValue:  health.QualityScore,
				Threshold:    50.0,
				ConnectionID: id,
			})
		}
	}

	return alerts
}

// ADDED: updateThroughput calculates current throughput based on total bytes transferred over time.
// This internal method updates the throughput metric approximately once per second.
func (nm *NetworkMonitor) updateThroughput() {
	timeDelta := time.Since(nm.lastUpdate).Seconds()
	if timeDelta > 1.0 { // ADDED: Update throughput calculation every second
		totalBytes := nm.metrics.BytesSent + nm.metrics.BytesReceived
		nm.metrics.Throughput = float64(totalBytes) / time.Since(nm.startTime).Seconds()
		nm.lastUpdate = time.Now()
	}
}

// ADDED: updateConnectionQuality calculates a quality score (0-100) for a connection based on performance metrics.
// The score considers RTT, packet loss, and error rates to provide an overall health indicator.
func (nm *NetworkMonitor) updateConnectionQuality(health *ConnectionHealth) {
	// ADDED: Start with perfect quality score
	quality := 100.0

	// ADDED: Penalize high RTT (above 100ms baseline)
	if health.RTT > 100 {
		quality -= (health.RTT - 100) / 10
	}

	// ADDED: Penalize packet loss (1% loss = 1 point deduction)
	quality -= health.PacketLoss * 100

	// ADDED: Penalize error rate if there has been traffic
	if health.BytesSent+health.BytesReceived > 0 {
		errorRate := float64(health.ErrorCount) / float64(health.BytesSent+health.BytesReceived)
		quality -= errorRate * 1000 // ADDED: Errors heavily penalized
	}

	// ADDED: Ensure quality score stays within valid bounds
	if quality < 0 {
		quality = 0
	}
	if quality > 100 {
		quality = 100
	}

	health.QualityScore = quality
}

// ADDED: ExportMetricsJSON exports current network metrics in JSON format for external monitoring tools.
// This provides a standardized way to integrate with monitoring and alerting systems.
//
//export ToxExportMetricsJSON
func (nm *NetworkMonitor) ExportMetricsJSON() ([]byte, error) {
	metrics := nm.GetMetrics()
	return json.MarshalIndent(metrics, "", "  ")
}

// ADDED: SetAlertThresholds updates the thresholds used for triggering network alerts.
// This allows dynamic adjustment of monitoring sensitivity based on network conditions.
//
//export ToxSetAlertThresholds
func (nm *NetworkMonitor) SetAlertThresholds(thresholds *AlertThresholds) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.alertThresholds = thresholds
}
