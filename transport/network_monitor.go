package transport

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// NetworkMonitor tracks network performance and health metrics
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

// NetworkMetrics aggregates overall network performance data
//
//export ToxNetworkMetrics
type NetworkMetrics struct {
	// Throughput metrics
	BytesSent           uint64    `json:"bytes_sent"`
	BytesReceived       uint64    `json:"bytes_received"`
	PacketsSent         uint64    `json:"packets_sent"`
	PacketsReceived     uint64    `json:"packets_received"`
	
	// Performance metrics  
	AverageLatency      float64   `json:"average_latency_ms"`
	PacketLossRate      float64   `json:"packet_loss_rate"`
	Throughput          float64   `json:"throughput_bps"`
	
	// Connection metrics
	ActiveConnections   int       `json:"active_connections"`
	FailedConnections   uint64    `json:"failed_connections"`
	TotalConnections    uint64    `json:"total_connections"`
	
	// Error metrics
	NetworkErrors       uint64    `json:"network_errors"`
	ProtocolErrors      uint64    `json:"protocol_errors"`
	TimeoutErrors       uint64    `json:"timeout_errors"`
	
	// Timing
	Uptime              float64   `json:"uptime_seconds"`
	LastUpdated         time.Time `json:"last_updated"`
}

// ConnectionHealth tracks health of individual connections
//
//export ToxConnectionHealth
type ConnectionHealth struct {
	ConnectionID     string    `json:"connection_id"`
	RemoteAddr       string    `json:"remote_addr"`
	State           string    `json:"state"`
	LastSeen        time.Time `json:"last_seen"`
	RTT             float64   `json:"rtt_ms"`
	PacketLoss      float64   `json:"packet_loss_rate"`
	BytesSent       uint64    `json:"bytes_sent"`
	BytesReceived   uint64    `json:"bytes_received"`
	ErrorCount      uint64    `json:"error_count"`
	QualityScore    float64   `json:"quality_score"` // 0-100 scale
}

// AlertThresholds defines when to trigger network alerts
//
//export ToxAlertThresholds
type AlertThresholds struct {
	MaxLatency       float64 `json:"max_latency_ms"`
	MaxPacketLoss    float64 `json:"max_packet_loss_rate"`
	MinThroughput    float64 `json:"min_throughput_bps"`
	MaxErrorRate     float64 `json:"max_error_rate"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
}

// NetworkAlert represents a network health alert
//
//export ToxNetworkAlert
type NetworkAlert struct {
	AlertType   AlertType `json:"alert_type"`
	Severity    AlertSeverity `json:"severity"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	MetricValue float64   `json:"metric_value"`
	Threshold   float64   `json:"threshold"`
	ConnectionID string   `json:"connection_id,omitempty"`
}

// AlertType categorizes different types of network alerts
type AlertType int

const (
	AlertLatencyHigh AlertType = iota
	AlertPacketLossHigh
	AlertThroughputLow
	AlertConnectionFailed
	AlertErrorRateHigh
	AlertConnectionTimeout
)

// AlertSeverity indicates the severity of an alert
type AlertSeverity int

const (
	SeverityInfo AlertSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

// NewNetworkMonitor creates a new network monitor
//
//export ToxNewNetworkMonitor
func NewNetworkMonitor() *NetworkMonitor {
	return &NetworkMonitor{
		metrics: &NetworkMetrics{
			LastUpdated: time.Now(),
		},
		connectionHealth: make(map[string]*ConnectionHealth),
		alertThresholds: &AlertThresholds{
			MaxLatency:       1000.0, // 1 second
			MaxPacketLoss:    0.05,   // 5%
			MinThroughput:    1024,   // 1 KB/s
			MaxErrorRate:     0.01,   // 1%
			ConnectionTimeout: 30 * time.Second,
		},
		startTime:  time.Now(),
		lastUpdate: time.Now(),
	}
}

// RecordPacketSent records a sent packet
//
//export ToxRecordPacketSent
func (nm *NetworkMonitor) RecordPacketSent(connectionID string, size int) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	nm.metrics.PacketsSent++
	nm.metrics.BytesSent += uint64(size)
	
	if health, exists := nm.connectionHealth[connectionID]; exists {
		health.BytesSent += uint64(size)
	}
	
	nm.updateThroughput()
}

// RecordPacketReceived records a received packet
//
//export ToxRecordPacketReceived
func (nm *NetworkMonitor) RecordPacketReceived(connectionID string, size int, latency time.Duration) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	nm.metrics.PacketsReceived++
	nm.metrics.BytesReceived += uint64(size)
	
	// Update latency (exponential moving average)
	latencyMs := float64(latency.Nanoseconds()) / 1e6
	if nm.metrics.AverageLatency == 0 {
		nm.metrics.AverageLatency = latencyMs
	} else {
		nm.metrics.AverageLatency = 0.9*nm.metrics.AverageLatency + 0.1*latencyMs
	}
	
	if health, exists := nm.connectionHealth[connectionID]; exists {
		health.BytesReceived += uint64(size)
		health.RTT = latencyMs
		health.LastSeen = time.Now()
	}
	
	nm.updateThroughput()
}

// RecordConnectionEstablished records a new connection
//
//export ToxRecordConnectionEstablished
func (nm *NetworkMonitor) RecordConnectionEstablished(connectionID, remoteAddr string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	nm.metrics.TotalConnections++
	nm.metrics.ActiveConnections++
	
	nm.connectionHealth[connectionID] = &ConnectionHealth{
		ConnectionID:  connectionID,
		RemoteAddr:    remoteAddr,
		State:        "connected",
		LastSeen:     time.Now(),
		QualityScore: 100.0,
	}
}

// RecordConnectionClosed records a closed connection
//
//export ToxRecordConnectionClosed
func (nm *NetworkMonitor) RecordConnectionClosed(connectionID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	if health, exists := nm.connectionHealth[connectionID]; exists {
		health.State = "closed"
		nm.metrics.ActiveConnections--
	}
}

// RecordError records a network error
//
//export ToxRecordError
func (nm *NetworkMonitor) RecordError(connectionID string, errorType string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	switch errorType {
	case "network":
		nm.metrics.NetworkErrors++
	case "protocol":
		nm.metrics.ProtocolErrors++
	case "timeout":
		nm.metrics.TimeoutErrors++
	}
	
	if health, exists := nm.connectionHealth[connectionID]; exists {
		health.ErrorCount++
		nm.updateConnectionQuality(health)
	}
}

// GetMetrics returns current network metrics
//
//export ToxGetNetworkMetrics
func (nm *NetworkMonitor) GetMetrics() *NetworkMetrics {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	// Update uptime
	nm.metrics.Uptime = time.Since(nm.startTime).Seconds()
	nm.metrics.LastUpdated = time.Now()
	
	// Return a copy
	metricsCopy := *nm.metrics
	return &metricsCopy
}

// GetConnectionHealth returns health status for all connections
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

// CheckAlerts checks for network health issues and returns alerts
//
//export ToxCheckAlerts
func (nm *NetworkMonitor) CheckAlerts() []NetworkAlert {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	var alerts []NetworkAlert
	now := time.Now()
	
	// Check overall metrics
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
	
	// Check individual connections
	for id, health := range nm.connectionHealth {
		if time.Since(health.LastSeen) > nm.alertThresholds.ConnectionTimeout {
			alerts = append(alerts, NetworkAlert{
				AlertType:    AlertConnectionTimeout,
				Severity:     SeverityError,
				Message:      fmt.Sprintf("Connection timeout: %s", id),
				Timestamp:    now,
				ConnectionID: id,
			})
		}
		
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

// updateThroughput calculates current throughput
func (nm *NetworkMonitor) updateThroughput() {
	timeDelta := time.Since(nm.lastUpdate).Seconds()
	if timeDelta > 1.0 { // Update every second
		totalBytes := nm.metrics.BytesSent + nm.metrics.BytesReceived
		nm.metrics.Throughput = float64(totalBytes) / time.Since(nm.startTime).Seconds()
		nm.lastUpdate = time.Now()
	}
}

// updateConnectionQuality calculates connection quality score
func (nm *NetworkMonitor) updateConnectionQuality(health *ConnectionHealth) {
	// Base quality score calculation
	quality := 100.0
	
	// Penalize high RTT
	if health.RTT > 100 {
		quality -= (health.RTT - 100) / 10
	}
	
	// Penalize packet loss
	quality -= health.PacketLoss * 100
	
	// Penalize errors
	if health.BytesSent+health.BytesReceived > 0 {
		errorRate := float64(health.ErrorCount) / float64(health.BytesSent+health.BytesReceived)
		quality -= errorRate * 1000
	}
	
	// Ensure quality is within bounds
	if quality < 0 {
		quality = 0
	}
	if quality > 100 {
		quality = 100
	}
	
	health.QualityScore = quality
}

// ExportMetricsJSON exports metrics in JSON format
//
//export ToxExportMetricsJSON
func (nm *NetworkMonitor) ExportMetricsJSON() ([]byte, error) {
	metrics := nm.GetMetrics()
	return json.MarshalIndent(metrics, "", "  ")
}

// SetAlertThresholds updates alert thresholds
//
//export ToxSetAlertThresholds
func (nm *NetworkMonitor) SetAlertThresholds(thresholds *AlertThresholds) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.alertThresholds = thresholds
}
