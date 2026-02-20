// Package av provides adaptive bitrate management for ToxAV calls.
//
// This module implements automatic bitrate adaptation based on network
// quality metrics to optimize audio/video call quality and reliability.
//
// Design Philosophy:
// - Use simple, proven algorithms instead of complex ML approaches
// - Prioritize audio quality over video when bandwidth is limited
// - React quickly to network degradation but recover slowly
// - Provide clear callbacks for application-level quality monitoring
//
// The adaptation algorithm follows industry best practices:
// 1. Monitor packet loss, jitter, and RTT from RTP statistics
// 2. Adjust bitrates based on network quality using AIMD algorithm
// 3. Trigger callbacks when significant changes occur
// 4. Maintain minimum quality levels for usable communication
package av

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// NetworkQuality represents current network condition assessment.
type NetworkQuality int

const (
	// NetworkExcellent indicates optimal network conditions (< 1% loss, < 50ms jitter)
	NetworkExcellent NetworkQuality = iota
	// NetworkGood indicates good network conditions (< 3% loss, < 100ms jitter)
	NetworkGood
	// NetworkFair indicates fair network conditions (< 5% loss, < 150ms jitter)
	NetworkFair
	// NetworkPoor indicates poor network conditions (> 5% loss, > 150ms jitter)
	NetworkPoor
)

// String returns human-readable network quality description.
func (nq NetworkQuality) String() string {
	switch nq {
	case NetworkExcellent:
		return "excellent"
	case NetworkGood:
		return "good"
	case NetworkFair:
		return "fair"
	case NetworkPoor:
		return "poor"
	default:
		return "unknown"
	}
}

// AdaptationConfig defines adaptation algorithm parameters.
//
// These values are tuned for VoIP applications following RFC recommendations
// and industry best practices. Conservative defaults prioritize stability.
type AdaptationConfig struct {
	// Monitoring intervals
	StatsInterval    time.Duration // How often to check network stats (default: 2s)
	AdaptationWindow time.Duration // Window for adaptation decisions (default: 10s)

	// Network quality thresholds
	PoorLossThreshold   float64       // Packet loss % threshold for poor quality (default: 5.0)
	FairLossThreshold   float64       // Packet loss % threshold for fair quality (default: 3.0)
	GoodLossThreshold   float64       // Packet loss % threshold for good quality (default: 1.0)
	PoorJitterThreshold time.Duration // Jitter threshold for poor quality (default: 150ms)
	FairJitterThreshold time.Duration // Jitter threshold for fair quality (default: 100ms)
	GoodJitterThreshold time.Duration // Jitter threshold for good quality (default: 50ms)

	// Bitrate limits and adjustments
	MinAudioBitRate uint32 // Minimum audio bitrate (default: 16000 bps)
	MaxAudioBitRate uint32 // Maximum audio bitrate (default: 64000 bps)
	MinVideoBitRate uint32 // Minimum video bitrate (default: 100000 bps)
	MaxVideoBitRate uint32 // Maximum video bitrate (default: 2000000 bps)

	// AIMD algorithm parameters (Additive Increase, Multiplicative Decrease)
	IncreaseStep       float64 // Bitrate increase step when quality is good (default: 0.1)
	DecreaseMultiplier float64 // Bitrate decrease multiplier when quality is poor (default: 0.8)

	// Stability controls
	MinChangeBitRate uint32        // Minimum bitrate change to trigger callback (default: 5000 bps)
	BackoffDuration  time.Duration // How long to wait after decrease before increasing (default: 5s)
}

// DefaultAdaptationConfig returns configuration with conservative defaults.
//
// These settings prioritize call stability over maximum quality and work
// well across various network conditions and device capabilities.
func DefaultAdaptationConfig() *AdaptationConfig {
	return &AdaptationConfig{
		// Monitoring intervals - balance responsiveness with CPU usage
		StatsInterval:    2 * time.Second,
		AdaptationWindow: 10 * time.Second,

		// Network quality thresholds based on ITU-T G.114 recommendations
		PoorLossThreshold:   5.0,                    // > 5% loss severely impacts quality
		FairLossThreshold:   3.0,                    // 3-5% loss noticeable but acceptable
		GoodLossThreshold:   1.0,                    // < 1% loss is excellent for VoIP
		PoorJitterThreshold: 150 * time.Millisecond, // ITU-T G.114 max for good quality
		FairJitterThreshold: 100 * time.Millisecond, // Conservative fair threshold
		GoodJitterThreshold: 50 * time.Millisecond,  // Excellent quality threshold

		// Bitrate limits for Opus audio and VP8 video
		MinAudioBitRate: 16000,   // 16 kbps minimum for understandable speech
		MaxAudioBitRate: 64000,   // 64 kbps sufficient for high-quality voice
		MinVideoBitRate: 100000,  // 100 kbps minimum for usable video
		MaxVideoBitRate: 2000000, // 2 Mbps maximum for HD video calls

		// AIMD parameters tuned for VoIP stability
		IncreaseStep:       0.1, // 10% increase when network is good
		DecreaseMultiplier: 0.8, // 20% decrease when network degrades

		// Stability controls to prevent oscillation
		MinChangeBitRate: 5000,            // 5 kbps minimum change threshold
		BackoffDuration:  5 * time.Second, // 5s backoff after decrease
	}
}

// BitrateAdapter manages automatic bitrate adaptation for calls.
//
// This component monitors network quality and automatically adjusts
// audio and video bitrates to optimize call quality. It implements
// the AIMD (Additive Increase, Multiplicative Decrease) algorithm
// commonly used in TCP congestion control, adapted for real-time media.
type BitrateAdapter struct {
	mu     sync.RWMutex
	config *AdaptationConfig

	// Current state
	currentQuality NetworkQuality
	lastAdaptation time.Time
	lastDecrease   time.Time

	// Current bitrates
	audioBitRate uint32
	videoBitRate uint32

	// Statistics tracking
	adaptationCount uint64
	qualityHistory  []NetworkQuality // Last few quality measurements

	// Callbacks
	audioBitRateCb func(uint32)
	videoBitRateCb func(uint32)
	qualityCb      func(NetworkQuality)

	// Time provider for deterministic testing
	timeProvider TimeProvider
}

// NewBitrateAdapter creates a new adaptive bitrate manager.
//
// The adapter starts with provided initial bitrates and begins monitoring
// immediately. Callbacks are triggered when significant changes occur.
//
// Parameters:
//   - config: Adaptation algorithm configuration (use DefaultAdaptationConfig())
//   - initialAudioBitRate: Starting audio bitrate in bps
//   - initialVideoBitRate: Starting video bitrate in bps
//
// Returns:
//   - *BitrateAdapter: The new adapter instance
func NewBitrateAdapter(config *AdaptationConfig, initialAudioBitRate, initialVideoBitRate uint32) *BitrateAdapter {
	if config == nil {
		config = DefaultAdaptationConfig()
	}

	logrus.WithFields(logrus.Fields{
		"function":          "NewBitrateAdapter",
		"initial_audio_bps": initialAudioBitRate,
		"initial_video_bps": initialVideoBitRate,
		"stats_interval":    config.StatsInterval,
		"adaptation_window": config.AdaptationWindow,
	}).Info("Creating new bitrate adapter")

	adapter := &BitrateAdapter{
		config:         config,
		currentQuality: NetworkGood, // Start optimistic
		lastAdaptation: time.Time{}, // Will be set on first actual adaptation
		lastDecrease:   time.Time{}, // No previous decrease
		audioBitRate:   initialAudioBitRate,
		videoBitRate:   initialVideoBitRate,
		qualityHistory: make([]NetworkQuality, 0, 5), // Keep last 5 measurements
	}

	logrus.WithFields(logrus.Fields{
		"function":        "NewBitrateAdapter",
		"initial_quality": adapter.currentQuality.String(),
	}).Info("Bitrate adapter created successfully")

	return adapter
}

// SetCallbacks configures adaptation event callbacks.
//
// These callbacks are triggered when the adapter makes significant
// bitrate changes or detects quality changes.
//
// Parameters:
//   - audioBitRateCb: Called when audio bitrate changes significantly
//   - videoBitRateCb: Called when video bitrate changes significantly
//   - qualityCb: Called when network quality assessment changes
func (ba *BitrateAdapter) SetCallbacks(
	audioBitRateCb func(uint32),
	videoBitRateCb func(uint32),
	qualityCb func(NetworkQuality),
) {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	ba.audioBitRateCb = audioBitRateCb
	ba.videoBitRateCb = videoBitRateCb
	ba.qualityCb = qualityCb

	logrus.WithFields(logrus.Fields{
		"function":         "SetCallbacks",
		"audio_callback":   audioBitRateCb != nil,
		"video_callback":   videoBitRateCb != nil,
		"quality_callback": qualityCb != nil,
	}).Debug("Bitrate adapter callbacks configured")
}

// UpdateNetworkStats processes new network statistics and triggers adaptation.
//
// This method should be called regularly (every 1-2 seconds) with current
// RTP statistics. It assesses network quality and adapts bitrates as needed.
//
// Parameters:
//   - packetsSent: Total packets sent since call start
//   - packetsReceived: Total packets received since call start
//   - packetsLost: Total packets lost (estimated)
//   - jitter: Current network jitter
//   - timestamp: When these statistics were measured
//
// Returns:
//   - bool: Whether adaptation occurred
//   - error: Any error during processing
func (ba *BitrateAdapter) UpdateNetworkStats(packetsSent, packetsReceived, packetsLost uint64, jitter time.Duration, timestamp time.Time) (bool, error) {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	ba.logNetworkStatsUpdate(packetsSent, packetsReceived, packetsLost, jitter)

	lossPercent := ba.calculatePacketLoss(packetsSent, packetsLost)
	newQuality := ba.assessNetworkQuality(lossPercent, jitter)

	ba.updateQualityHistory(newQuality)

	if ba.handleQualityChange(newQuality, lossPercent, jitter) {
		ba.currentQuality = newQuality
	}

	return ba.performAdaptation(newQuality, timestamp)
}

// logNetworkStatsUpdate logs the incoming network statistics.
func (ba *BitrateAdapter) logNetworkStatsUpdate(packetsSent, packetsReceived, packetsLost uint64, jitter time.Duration) {
	logrus.WithFields(logrus.Fields{
		"function":         "UpdateNetworkStats",
		"packets_sent":     packetsSent,
		"packets_received": packetsReceived,
		"packets_lost":     packetsLost,
		"jitter_ms":        jitter.Milliseconds(),
	}).Debug("Processing network statistics update")
}

// calculatePacketLoss computes the packet loss percentage.
func (ba *BitrateAdapter) calculatePacketLoss(packetsSent, packetsLost uint64) float64 {
	if packetsSent > 0 {
		return float64(packetsLost) / float64(packetsSent) * 100.0
	}
	return 0
}

// updateQualityHistory adds the new quality measurement to history.
func (ba *BitrateAdapter) updateQualityHistory(newQuality NetworkQuality) {
	ba.qualityHistory = append(ba.qualityHistory, newQuality)
	if len(ba.qualityHistory) > 5 {
		ba.qualityHistory = ba.qualityHistory[1:]
	}
}

// handleQualityChange processes network quality changes and triggers callbacks.
func (ba *BitrateAdapter) handleQualityChange(newQuality NetworkQuality, lossPercent float64, jitter time.Duration) bool {
	qualityChanged := ba.currentQuality != newQuality
	if !qualityChanged {
		return false
	}

	logrus.WithFields(logrus.Fields{
		"function":     "UpdateNetworkStats",
		"old_quality":  ba.currentQuality.String(),
		"new_quality":  newQuality.String(),
		"loss_percent": lossPercent,
		"jitter_ms":    jitter.Milliseconds(),
	}).Info("Network quality changed")

	if ba.qualityCb != nil {
		go ba.qualityCb(newQuality)
	}

	return true
}

// performAdaptation decides whether to perform bitrate adaptation.
func (ba *BitrateAdapter) performAdaptation(newQuality NetworkQuality, timestamp time.Time) (bool, error) {
	if ba.lastAdaptation.IsZero() {
		ba.lastAdaptation = timestamp
		logrus.WithFields(logrus.Fields{
			"function":  "UpdateNetworkStats",
			"timestamp": timestamp,
		}).Debug("Initialized adaptation baseline timestamp")
		return false, nil
	}

	timeSinceLastAdaptation := timestamp.Sub(ba.lastAdaptation)
	shouldAdapt := timeSinceLastAdaptation >= ba.config.AdaptationWindow

	if !shouldAdapt {
		logrus.WithFields(logrus.Fields{
			"function":          "UpdateNetworkStats",
			"time_since_last":   timeSinceLastAdaptation,
			"adaptation_window": ba.config.AdaptationWindow,
		}).Debug("Skipping adaptation - within adaptation window")
		return false, nil
	}

	adapted := ba.adaptBitrates(newQuality, timestamp)
	if adapted {
		ba.lastAdaptation = timestamp
		ba.adaptationCount++

		logrus.WithFields(logrus.Fields{
			"function":         "UpdateNetworkStats",
			"adaptation_count": ba.adaptationCount,
			"new_audio_bps":    ba.audioBitRate,
			"new_video_bps":    ba.videoBitRate,
		}).Info("Bitrate adaptation completed")
	}

	return adapted, nil
}

// assessNetworkQuality determines network quality from statistics.
//
// Uses both packet loss and jitter to assess quality, taking the
// worst condition of the two metrics for conservative adaptation.
func (ba *BitrateAdapter) assessNetworkQuality(lossPercent float64, jitter time.Duration) NetworkQuality {
	logrus.WithFields(logrus.Fields{
		"function":     "assessNetworkQuality",
		"loss_percent": lossPercent,
		"jitter_ms":    jitter.Milliseconds(),
	}).Debug("Assessing network quality")

	// Assess based on packet loss
	var qualityByLoss NetworkQuality
	switch {
	case lossPercent >= ba.config.PoorLossThreshold:
		qualityByLoss = NetworkPoor
	case lossPercent >= ba.config.FairLossThreshold:
		qualityByLoss = NetworkFair
	case lossPercent >= ba.config.GoodLossThreshold:
		qualityByLoss = NetworkGood
	default:
		qualityByLoss = NetworkExcellent
	}

	// Assess based on jitter
	var qualityByJitter NetworkQuality
	switch {
	case jitter >= ba.config.PoorJitterThreshold:
		qualityByJitter = NetworkPoor
	case jitter >= ba.config.FairJitterThreshold:
		qualityByJitter = NetworkFair
	case jitter >= ba.config.GoodJitterThreshold:
		qualityByJitter = NetworkGood
	default:
		qualityByJitter = NetworkExcellent
	}

	// Take the worse of the two assessments (conservative approach)
	finalQuality := qualityByLoss
	if qualityByJitter > finalQuality {
		finalQuality = qualityByJitter
	}

	logrus.WithFields(logrus.Fields{
		"function":          "assessNetworkQuality",
		"quality_by_loss":   qualityByLoss.String(),
		"quality_by_jitter": qualityByJitter.String(),
		"final_quality":     finalQuality.String(),
	}).Debug("Network quality assessment completed")

	return finalQuality
}

// adaptBitrates adjusts bitrates based on network quality.
//
// Implements AIMD algorithm: increase gradually when quality is good,
// decrease more aggressively when quality degrades.
// applyQualityBasedAdaptation adjusts bitrates based on network quality.
func (ba *BitrateAdapter) applyQualityBasedAdaptation(quality NetworkQuality, timestamp time.Time) {
	switch quality {
	case NetworkPoor:
		ba.decreaseBitrates(timestamp)
	case NetworkFair:
		ba.conservativeBitrates()
	case NetworkGood, NetworkExcellent:
		if ba.canIncreaseBitrates(timestamp) {
			ba.increaseBitrates()
		}
	}
}

// triggerBitrateCallbacks invokes callbacks if bitrates changed significantly.
func (ba *BitrateAdapter) triggerBitrateCallbacks(oldAudioBitRate, oldVideoBitRate uint32) (audioChanged, videoChanged bool) {
	audioChanged = ba.isSignificantChange(oldAudioBitRate, ba.audioBitRate)
	videoChanged = ba.isSignificantChange(oldVideoBitRate, ba.videoBitRate)

	if audioChanged && ba.audioBitRateCb != nil {
		go ba.audioBitRateCb(ba.audioBitRate)
	}
	if videoChanged && ba.videoBitRateCb != nil {
		go ba.videoBitRateCb(ba.videoBitRate)
	}

	return audioChanged, videoChanged
}

func (ba *BitrateAdapter) adaptBitrates(quality NetworkQuality, timestamp time.Time) bool {
	oldAudioBitRate := ba.audioBitRate
	oldVideoBitRate := ba.videoBitRate

	logrus.WithFields(logrus.Fields{
		"function":      "adaptBitrates",
		"quality":       quality.String(),
		"current_audio": oldAudioBitRate,
		"current_video": oldVideoBitRate,
	}).Debug("Starting bitrate adaptation")

	ba.applyQualityBasedAdaptation(quality, timestamp)

	audioChanged, videoChanged := ba.triggerBitrateCallbacks(oldAudioBitRate, oldVideoBitRate)

	adapted := audioChanged || videoChanged
	if adapted {
		logrus.WithFields(logrus.Fields{
			"function":      "adaptBitrates",
			"audio_changed": audioChanged,
			"video_changed": videoChanged,
			"new_audio":     ba.audioBitRate,
			"new_video":     ba.videoBitRate,
		}).Info("Bitrate adaptation triggered callbacks")
	}

	return adapted
}

// decreaseBitrates aggressively reduces bitrates for poor network conditions.
func (ba *BitrateAdapter) decreaseBitrates(timestamp time.Time) {
	ba.lastDecrease = timestamp

	// Decrease both audio and video using multiplicative decrease
	newAudioBitRate := uint32(float64(ba.audioBitRate) * ba.config.DecreaseMultiplier)
	newVideoBitRate := uint32(float64(ba.videoBitRate) * ba.config.DecreaseMultiplier)

	// Respect minimum limits
	if newAudioBitRate < ba.config.MinAudioBitRate {
		newAudioBitRate = ba.config.MinAudioBitRate
	}
	if newVideoBitRate < ba.config.MinVideoBitRate {
		newVideoBitRate = ba.config.MinVideoBitRate
	}

	ba.audioBitRate = newAudioBitRate
	ba.videoBitRate = newVideoBitRate

	logrus.WithFields(logrus.Fields{
		"function":      "decreaseBitrates",
		"new_audio":     ba.audioBitRate,
		"new_video":     ba.videoBitRate,
		"decrease_mult": ba.config.DecreaseMultiplier,
	}).Info("Decreased bitrates due to poor network quality")
}

// conservativeBitrates slightly reduces or maintains current bitrates.
func (ba *BitrateAdapter) conservativeBitrates() {
	// For fair quality, slightly reduce video but maintain audio
	// Audio is more important for communication than video quality
	newVideoBitRate := uint32(float64(ba.videoBitRate) * 0.95) // 5% decrease
	if newVideoBitRate < ba.config.MinVideoBitRate {
		newVideoBitRate = ba.config.MinVideoBitRate
	}

	ba.videoBitRate = newVideoBitRate
	// Audio bitrate remains unchanged for fair quality

	logrus.WithFields(logrus.Fields{
		"function":  "conservativeBitrates",
		"new_video": ba.videoBitRate,
		"audio":     ba.audioBitRate,
	}).Debug("Applied conservative bitrate adjustment")
}

// increaseBitrates gradually increases bitrates for good network conditions.
func (ba *BitrateAdapter) increaseBitrates() {
	// Additive increase for both audio and video
	newAudioBitRate := uint32(float64(ba.audioBitRate) * (1.0 + ba.config.IncreaseStep))
	newVideoBitRate := uint32(float64(ba.videoBitRate) * (1.0 + ba.config.IncreaseStep))

	// Respect maximum limits
	if newAudioBitRate > ba.config.MaxAudioBitRate {
		newAudioBitRate = ba.config.MaxAudioBitRate
	}
	if newVideoBitRate > ba.config.MaxVideoBitRate {
		newVideoBitRate = ba.config.MaxVideoBitRate
	}

	ba.audioBitRate = newAudioBitRate
	ba.videoBitRate = newVideoBitRate

	logrus.WithFields(logrus.Fields{
		"function":      "increaseBitrates",
		"new_audio":     ba.audioBitRate,
		"new_video":     ba.videoBitRate,
		"increase_step": ba.config.IncreaseStep,
	}).Info("Increased bitrates due to good network quality")
}

// canIncreaseBitrates checks if it's safe to increase bitrates.
//
// Implements backoff mechanism: don't increase too soon after a decrease
// to avoid oscillation and allow network to stabilize.
func (ba *BitrateAdapter) canIncreaseBitrates(timestamp time.Time) bool {
	if ba.lastDecrease.IsZero() {
		return true // No previous decrease
	}

	timeSinceDecrease := timestamp.Sub(ba.lastDecrease)
	canIncrease := timeSinceDecrease >= ba.config.BackoffDuration

	logrus.WithFields(logrus.Fields{
		"function":            "canIncreaseBitrates",
		"time_since_decrease": timeSinceDecrease,
		"backoff_duration":    ba.config.BackoffDuration,
		"can_increase":        canIncrease,
	}).Debug("Checking if bitrate increase is allowed")

	return canIncrease
}

// isSignificantChange checks if a bitrate change meets the minimum threshold.
func (ba *BitrateAdapter) isSignificantChange(oldBitRate, newBitRate uint32) bool {
	if oldBitRate == newBitRate {
		return false
	}

	var change uint32
	if newBitRate > oldBitRate {
		change = newBitRate - oldBitRate
	} else {
		change = oldBitRate - newBitRate
	}

	return change >= ba.config.MinChangeBitRate
}

// GetCurrentBitrates returns current audio and video bitrates.
func (ba *BitrateAdapter) GetCurrentBitrates() (audio, video uint32) {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	return ba.audioBitRate, ba.videoBitRate
}

// GetNetworkQuality returns current assessed network quality.
func (ba *BitrateAdapter) GetNetworkQuality() NetworkQuality {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	return ba.currentQuality
}

// SetTimeProvider sets the time provider for deterministic testing.
// If not set, uses time.Now() directly.
func (ba *BitrateAdapter) SetTimeProvider(tp TimeProvider) {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	ba.timeProvider = tp
}

// getTimeProvider returns the configured time provider or a default that uses time.Now().
func (ba *BitrateAdapter) getTimeProvider() TimeProvider {
	if ba.timeProvider != nil {
		return ba.timeProvider
	}
	return DefaultTimeProvider{}
}

// GetAdaptationStats returns statistics about adaptation behavior.
func (ba *BitrateAdapter) GetAdaptationStats() (adaptationCount uint64, lastAdaptation time.Time) {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	return ba.adaptationCount, ba.lastAdaptation
}
