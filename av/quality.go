// Package av provides call quality monitoring capabilities for ToxAV.
//
// This module implements comprehensive quality monitoring by collecting
// metrics from RTP sessions, adaptive bitrate system, and call timing
// to provide real-time assessment of call quality.
//
// Design Philosophy:
// - Leverage existing components (RTP stats, adaptation, timing)
// - Provide actionable quality metrics for applications
// - Use simple thresholds instead of complex algorithms
// - Enable quality-based decision making for call management
//
// The monitoring system integrates with existing infrastructure:
// 1. Collects RTP statistics from session management
// 2. Uses network quality from adaptive bitrate system
// 3. Tracks call duration and frame timing
// 4. Provides quality callbacks for application responses
package av

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// QualityLevel represents overall call quality assessment.
type QualityLevel int

const (
	// QualityExcellent indicates optimal call quality
	QualityExcellent QualityLevel = iota
	// QualityGood indicates good call quality with minor issues
	QualityGood
	// QualityFair indicates acceptable call quality with noticeable issues
	QualityFair
	// QualityPoor indicates poor call quality with significant problems
	QualityPoor
	// QualityUnacceptable indicates unacceptable call quality
	QualityUnacceptable
)

// String returns the string representation of QualityLevel.
func (q QualityLevel) String() string {
	switch q {
	case QualityExcellent:
		return "Excellent"
	case QualityGood:
		return "Good"
	case QualityFair:
		return "Fair"
	case QualityPoor:
		return "Poor"
	case QualityUnacceptable:
		return "Unacceptable"
	default:
		return fmt.Sprintf("Unknown(%d)", int(q))
	}
}

// CallMetrics represents comprehensive call quality metrics.
//
// This structure provides real-time quality information collected
// from various system components for monitoring and optimization.
type CallMetrics struct {
	// Network metrics from RTP session
	PacketLoss      float64       // Packet loss percentage (0.0-100.0)
	Jitter          time.Duration // Network jitter measurement
	RoundTripTime   time.Duration // Estimated RTT from RTP timestamps
	PacketsSent     uint64        // Total packets sent
	PacketsReceived uint64        // Total packets received

	// Bandwidth metrics
	AudioBitRate   uint32         // Current audio bitrate (bps)
	VideoBitRate   uint32         // Current video bitrate (bps)
	NetworkQuality NetworkQuality // Network quality assessment

	// Call timing metrics
	CallDuration time.Duration // Total call duration
	LastFrameAge time.Duration // Time since last frame received

	// Overall quality assessment
	Quality   QualityLevel // Computed overall quality level
	Timestamp time.Time    // When metrics were collected
}

// QualityThresholds defines thresholds for quality level assessment.
//
// These thresholds are used to categorize call quality based on
// measured metrics. Applications can customize these values.
type QualityThresholds struct {
	// Packet loss thresholds (percentage)
	ExcellentPacketLoss float64 // < 1.0%
	GoodPacketLoss      float64 // < 3.0%
	FairPacketLoss      float64 // < 8.0%
	PoorPacketLoss      float64 // < 15.0%

	// Jitter thresholds
	ExcellentJitter time.Duration // < 20ms
	GoodJitter      time.Duration // < 50ms
	FairJitter      time.Duration // < 100ms
	PoorJitter      time.Duration // < 200ms

	// Frame timeout threshold
	FrameTimeout time.Duration // > 2 seconds = connection issue
}

// DefaultQualityThresholds returns sensible default quality thresholds.
//
// These values are based on VoIP industry standards and provide
// good quality assessment for typical voice/video calling scenarios.
func DefaultQualityThresholds() *QualityThresholds {
	return &QualityThresholds{
		ExcellentPacketLoss: 1.0,
		GoodPacketLoss:      3.0,
		FairPacketLoss:      8.0,
		PoorPacketLoss:      15.0,
		ExcellentJitter:     20 * time.Millisecond,
		GoodJitter:          50 * time.Millisecond,
		FairJitter:          100 * time.Millisecond,
		PoorJitter:          200 * time.Millisecond,
		FrameTimeout:        2 * time.Second,
	}
}

// QualityMonitor provides real-time call quality monitoring.
//
// The monitor collects metrics from existing system components
// and provides quality assessment with configurable callbacks.
type QualityMonitor struct {
	mu         sync.RWMutex
	thresholds *QualityThresholds

	// Quality change callback
	qualityCallback func(friendNumber uint32, metrics CallMetrics)

	// Monitoring configuration
	enabled         bool
	monitorInterval time.Duration
}

// NewQualityMonitor creates a new call quality monitoring system.
//
// The monitor uses provided thresholds for quality assessment and
// can trigger callbacks when quality changes significantly.
//
// Parameters:
//   - thresholds: Quality assessment thresholds (use DefaultQualityThresholds())
//
// Returns:
//   - *QualityMonitor: New monitor instance
func NewQualityMonitor(thresholds *QualityThresholds) *QualityMonitor {
	if thresholds == nil {
		thresholds = DefaultQualityThresholds()
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewQualityMonitor",
	}).Info("Creating new quality monitor")

	monitor := &QualityMonitor{
		thresholds:      thresholds,
		enabled:         true,
		monitorInterval: 5 * time.Second, // Monitor every 5 seconds
	}

	logrus.WithFields(logrus.Fields{
		"function":         "NewQualityMonitor",
		"monitor_interval": monitor.monitorInterval,
		"enabled":          monitor.enabled,
	}).Info("Quality monitor created successfully")

	return monitor
}

// SetQualityCallback registers a callback for quality changes.
//
// The callback will be triggered when significant quality changes
// are detected during monitoring. This allows applications to respond
// to quality issues (e.g., adjust UI, log warnings, change settings).
//
// Parameters:
//   - callback: Function to call on quality changes (can be nil to disable)
func (qm *QualityMonitor) SetQualityCallback(callback func(friendNumber uint32, metrics CallMetrics)) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	qm.qualityCallback = callback

	logrus.WithFields(logrus.Fields{
		"function":     "SetQualityCallback",
		"has_callback": callback != nil,
	}).Debug("Quality callback updated")
}

// SetEnabled enables or disables quality monitoring.
//
// When disabled, GetCallMetrics still works but callbacks are not triggered
// and internal monitoring stops. This can be useful for reducing overhead.
func (qm *QualityMonitor) SetEnabled(enabled bool) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	qm.enabled = enabled

	logrus.WithFields(logrus.Fields{
		"function": "SetEnabled",
		"enabled":  enabled,
	}).Info("Quality monitoring enabled status changed")
}

// IsEnabled returns whether quality monitoring is currently enabled.
func (qm *QualityMonitor) IsEnabled() bool {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	return qm.enabled
}

// GetCallMetrics collects and returns current call quality metrics.
//
// This method gathers metrics from the call's RTP session, bitrate adapter,
// and timing information to provide comprehensive quality assessment.
//
// Parameters:
//   - call: The call to collect metrics for
//   - adapter: Bitrate adapter for network quality (can be nil)
//
// Returns:
//   - CallMetrics: Current call quality metrics
//   - error: Any error during metrics collection
func (qm *QualityMonitor) GetCallMetrics(call *Call, adapter *BitrateAdapter) (CallMetrics, error) {
	if call == nil {
		return CallMetrics{}, fmt.Errorf("call cannot be nil")
	}

	metrics := qm.buildBasicMetrics(call, adapter)
	qm.enrichWithRTPStatistics(call, &metrics)
	metrics.Quality = qm.assessQuality(metrics)

	logrus.WithFields(logrus.Fields{
		"function":      "GetCallMetrics",
		"friend_number": call.GetFriendNumber(),
		"quality":       metrics.Quality.String(),
		"packet_loss":   metrics.PacketLoss,
		"jitter":        metrics.Jitter,
		"call_duration": metrics.CallDuration,
	}).Debug("Call metrics collected successfully")

	return metrics, nil
}

// buildBasicMetrics initializes the metrics structure with basic call information.
func (qm *QualityMonitor) buildBasicMetrics(call *Call, adapter *BitrateAdapter) CallMetrics {
	friendNumber := call.GetFriendNumber()
	callStart := call.GetStartTime()
	lastFrame := call.GetLastFrameTime()

	logrus.WithFields(logrus.Fields{
		"function":      "GetCallMetrics",
		"friend_number": friendNumber,
	}).Trace("Collecting call metrics")

	metrics := CallMetrics{
		AudioBitRate:   call.GetAudioBitRate(),
		VideoBitRate:   call.GetVideoBitRate(),
		CallDuration:   time.Since(callStart),
		LastFrameAge:   time.Since(lastFrame),
		Timestamp:      time.Now(),
		NetworkQuality: NetworkPoor,
	}

	if adapter != nil {
		metrics.NetworkQuality = adapter.GetNetworkQuality()
		logrus.WithFields(logrus.Fields{
			"function":        "GetCallMetrics",
			"friend_number":   friendNumber,
			"network_quality": metrics.NetworkQuality,
		}).Trace("Got network quality from adapter")
	}

	return metrics
}

// enrichWithRTPStatistics adds RTP session statistics to the metrics.
func (qm *QualityMonitor) enrichWithRTPStatistics(call *Call, metrics *CallMetrics) {
	rtpSession := call.GetRTPSession()
	if rtpSession == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "GetCallMetrics",
			"friend_number": call.GetFriendNumber(),
		}).Trace("No RTP session available for metrics")
		return
	}

	rtpStats := rtpSession.GetStatistics()
	totalPackets := rtpStats.PacketsSent + rtpStats.PacketsReceived
	if totalPackets > 0 {
		metrics.PacketLoss = float64(rtpStats.PacketsLost) / float64(totalPackets) * 100.0
	}

	metrics.PacketsSent = rtpStats.PacketsSent
	metrics.PacketsReceived = rtpStats.PacketsReceived
	metrics.Jitter = rtpStats.Jitter

	logrus.WithFields(logrus.Fields{
		"function":         "GetCallMetrics",
		"friend_number":    call.GetFriendNumber(),
		"packet_loss":      metrics.PacketLoss,
		"jitter":           metrics.Jitter,
		"packets_sent":     metrics.PacketsSent,
		"packets_received": metrics.PacketsReceived,
	}).Trace("Collected RTP statistics")
}

// assessQuality determines overall call quality based on collected metrics.
//
// This method uses configurable thresholds to categorize quality levels
// based on packet loss, jitter, and other factors.
func (qm *QualityMonitor) assessQuality(metrics CallMetrics) QualityLevel {
	qm.mu.RLock()
	thresholds := qm.thresholds
	qm.mu.RUnlock()

	// Check for connection issues first
	if qm.validateFrameTimeout(metrics, thresholds) {
		return QualityUnacceptable
	}

	// Assess based on packet loss (primary indicator)
	if packetLossQuality := qm.assessPacketLossQuality(metrics, thresholds); packetLossQuality != QualityExcellent {
		return packetLossQuality
	}

	// Excellent packet loss - check jitter for final assessment
	return qm.assessJitterQuality(metrics, thresholds)
}

// validateFrameTimeout checks if the frame timeout indicates connection issues.
//
// Returns true if the last frame age exceeds the timeout threshold,
// indicating unacceptable call quality due to connection problems.
func (qm *QualityMonitor) validateFrameTimeout(metrics CallMetrics, thresholds *QualityThresholds) bool {
	return metrics.LastFrameAge > thresholds.FrameTimeout
}

// assessPacketLossQuality determines quality level based on packet loss metrics.
//
// This is the primary quality indicator that categorizes quality from
// unacceptable to excellent based on packet loss percentage thresholds.
// Returns QualityExcellent if packet loss is below excellent threshold.
// checkJitterForGoodRange evaluates jitter when packet loss is in the good range.
func (qm *QualityMonitor) checkJitterForGoodRange(jitter, jitterThreshold time.Duration) QualityLevel {
	if jitter >= jitterThreshold {
		return QualityFair
	}
	return QualityGood
}

func (qm *QualityMonitor) assessPacketLossQuality(metrics CallMetrics, thresholds *QualityThresholds) QualityLevel {
	switch {
	case metrics.PacketLoss >= thresholds.PoorPacketLoss:
		return QualityUnacceptable
	case metrics.PacketLoss >= thresholds.FairPacketLoss:
		return QualityPoor
	case metrics.PacketLoss >= thresholds.GoodPacketLoss:
		return QualityFair
	case metrics.PacketLoss >= thresholds.ExcellentPacketLoss:
		return qm.checkJitterForGoodRange(metrics.Jitter, thresholds.GoodJitter)
	default:
		return QualityExcellent
	}
}

// assessJitterQuality determines final quality level based on jitter when packet loss is excellent.
//
// This method provides fine-grained quality assessment for calls with excellent
// packet loss by evaluating jitter thresholds to determine the final quality level.
func (qm *QualityMonitor) assessJitterQuality(metrics CallMetrics, thresholds *QualityThresholds) QualityLevel {
	if metrics.Jitter >= thresholds.PoorJitter {
		return QualityFair
	} else if metrics.Jitter >= thresholds.FairJitter {
		return QualityGood
	} else if metrics.Jitter >= thresholds.GoodJitter {
		return QualityGood
	} else if metrics.Jitter >= thresholds.ExcellentJitter {
		return QualityGood
	}

	return QualityExcellent
}

// MonitorCall performs quality monitoring for a specific call.
//
// This method should be called periodically (e.g., from Manager.Iterate)
// to track quality changes and trigger callbacks when needed.
//
// Parameters:
//   - call: The call to monitor
//   - adapter: Bitrate adapter for network quality (can be nil)
//
// Returns:
//   - CallMetrics: Current metrics (for convenience)
//   - error: Any error during monitoring
func (qm *QualityMonitor) MonitorCall(call *Call, adapter *BitrateAdapter) (CallMetrics, error) {
	if !qm.IsEnabled() {
		// Return empty metrics when disabled
		return CallMetrics{}, nil
	}

	metrics, err := qm.GetCallMetrics(call, adapter)
	if err != nil {
		return metrics, fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Trigger callback if registered
	qm.mu.RLock()
	callback := qm.qualityCallback
	qm.mu.RUnlock()

	if callback != nil {
		friendNumber := call.GetFriendNumber()

		logrus.WithFields(logrus.Fields{
			"function":      "MonitorCall",
			"friend_number": friendNumber,
			"quality":       metrics.Quality.String(),
		}).Trace("Triggering quality callback")

		// Call callback without holding lock
		callback(friendNumber, metrics)
	}

	return metrics, nil
}

// GetMonitorInterval returns the current monitoring interval.
func (qm *QualityMonitor) GetMonitorInterval() time.Duration {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	return qm.monitorInterval
}

// SetMonitorInterval updates the monitoring interval.
//
// This controls how frequently quality monitoring is performed
// when integrated with the Manager's iteration loop.
func (qm *QualityMonitor) SetMonitorInterval(interval time.Duration) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	qm.monitorInterval = interval

	logrus.WithFields(logrus.Fields{
		"function": "SetMonitorInterval",
		"interval": interval,
	}).Debug("Quality monitor interval updated")
}
