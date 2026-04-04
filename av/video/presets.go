// Package video provides video processing capabilities for ToxAV.
//
// This file defines quality presets for video encoding, providing
// pre-configured settings for different use cases and network conditions.
package video

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// QualityPreset defines the quality level for video encoding.
//
// Presets provide pre-configured combinations of resolution, bitrate, and
// frame rate optimized for different network conditions and use cases.
type QualityPreset int

const (
	// QualityLow is optimized for bandwidth-constrained connections.
	// Suitable for mobile networks (3G/4G), dial-up, or satellite.
	// Resolution: 320x240 (QVGA), Bitrate: 128kbps, FPS: 15
	QualityLow QualityPreset = iota

	// QualityMedium provides balanced quality for typical broadband.
	// Suitable for DSL, cable, or WiFi connections.
	// Resolution: 640x480 (VGA), Bitrate: 500kbps, FPS: 24
	QualityMedium

	// QualityHigh is optimized for high-bandwidth connections.
	// Suitable for fiber or fast broadband.
	// Resolution: 1280x720 (HD 720p), Bitrate: 1Mbps, FPS: 30
	QualityHigh

	// QualityUltra provides maximum quality for LAN or datacenter use.
	// Resolution: 1920x1080 (Full HD), Bitrate: 4Mbps, FPS: 30
	QualityUltra
)

// String returns a human-readable name for the quality preset.
func (q QualityPreset) String() string {
	switch q {
	case QualityLow:
		return "Low"
	case QualityMedium:
		return "Medium"
	case QualityHigh:
		return "High"
	case QualityUltra:
		return "Ultra"
	default:
		return fmt.Sprintf("Unknown(%d)", int(q))
	}
}

// PresetConfig contains the configuration parameters for a quality preset.
//
// These parameters control the video encoding quality. The KeyframeInterval
// setting controls how often key frames (I-frames) are emitted; between
// key frames the encoder produces inter frames (P-frames) for efficient
// bandwidth usage.
type PresetConfig struct {
	// Width is the horizontal resolution in pixels.
	Width uint16

	// Height is the vertical resolution in pixels.
	Height uint16

	// Bitrate is the target encoding bitrate in bits per second.
	Bitrate uint32

	// FrameRate is the target frames per second.
	FrameRate uint8

	// KeyframeInterval specifies how often to emit a keyframe (I-frame).
	// A value of 0 means every frame is a keyframe.
	// Otherwise, the value controls the GOP (Group of Pictures) size,
	// with inter frames (P-frames) emitted in between.
	//
	// Example: 30 means one keyframe every 30 frames (1 second at 30fps).
	KeyframeInterval uint16
}

// presetConfigs maps quality presets to their configurations.
var presetConfigs = map[QualityPreset]PresetConfig{
	QualityLow: {
		Width:            320,
		Height:           240,
		Bitrate:          128000, // 128 kbps
		FrameRate:        15,
		KeyframeInterval: 15, // 1 keyframe per second at 15fps
	},
	QualityMedium: {
		Width:            640,
		Height:           480,
		Bitrate:          500000, // 500 kbps
		FrameRate:        24,
		KeyframeInterval: 24, // 1 keyframe per second at 24fps
	},
	QualityHigh: {
		Width:            1280,
		Height:           720,
		Bitrate:          1000000, // 1 Mbps
		FrameRate:        30,
		KeyframeInterval: 30, // 1 keyframe per second at 30fps
	},
	QualityUltra: {
		Width:            1920,
		Height:           1080,
		Bitrate:          4000000, // 4 Mbps
		FrameRate:        30,
		KeyframeInterval: 30, // 1 keyframe per second at 30fps
	},
}

// GetPresetConfig returns the configuration for a given quality preset.
//
// Returns an error if the preset is not recognized.
func GetPresetConfig(preset QualityPreset) (PresetConfig, error) {
	logrus.WithFields(logrus.Fields{
		"function": "GetPresetConfig",
		"preset":   preset.String(),
	}).Debug("Getting preset configuration")

	config, ok := presetConfigs[preset]
	if !ok {
		logrus.WithFields(logrus.Fields{
			"function": "GetPresetConfig",
			"preset":   int(preset),
		}).Error("Unknown quality preset")
		return PresetConfig{}, fmt.Errorf("unknown quality preset: %d", preset)
	}

	logrus.WithFields(logrus.Fields{
		"function":  "GetPresetConfig",
		"preset":    preset.String(),
		"width":     config.Width,
		"height":    config.Height,
		"bitrate":   config.Bitrate,
		"framerate": config.FrameRate,
	}).Debug("Preset configuration retrieved")

	return config, nil
}

// NewProcessorWithPreset creates a video processor configured for a quality preset.
//
// This is a convenience function that creates a processor with the appropriate
// resolution and bitrate settings for the specified quality level.
func NewProcessorWithPreset(preset QualityPreset) (*Processor, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewProcessorWithPreset",
		"preset":   preset.String(),
	}).Info("Creating video processor with quality preset")

	config, err := GetPresetConfig(preset)
	if err != nil {
		return nil, fmt.Errorf("failed to get preset config: %w", err)
	}

	processor := NewProcessorWithSettings(config.Width, config.Height, config.Bitrate)
	if processor == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewProcessorWithPreset",
			"preset":   preset.String(),
		}).Error("Failed to create video processor")
		return nil, fmt.Errorf("failed to create video processor for preset %s", preset.String())
	}

	// Configure keyframe interval from preset.
	// Apply the configured value unconditionally so 0 retains its documented
	// meaning (all keyframes) instead of silently falling back to the encoder
	// default interval.
	processor.encoder.SetKeyFrameInterval(int(config.KeyframeInterval))

	logrus.WithFields(logrus.Fields{
		"function": "NewProcessorWithPreset",
		"preset":   preset.String(),
		"width":    config.Width,
		"height":   config.Height,
		"bitrate":  config.Bitrate,
	}).Info("Video processor created with quality preset")

	return processor, nil
}

// PresetForBandwidth returns the recommended quality preset based on available bandwidth.
//
// Parameters:
//   - bandwidthKbps: Available upload bandwidth in kilobits per second
//
// Returns the highest quality preset that fits within the available bandwidth,
// with a 20% headroom for network variance.
func PresetForBandwidth(bandwidthKbps uint32) QualityPreset {
	logrus.WithFields(logrus.Fields{
		"function":       "PresetForBandwidth",
		"bandwidth_kbps": bandwidthKbps,
	}).Debug("Selecting preset for bandwidth")

	// Apply 20% headroom for network variance
	effectiveBandwidth := bandwidthKbps * 800 // Convert to bps with 80% factor

	var selected QualityPreset
	switch {
	case effectiveBandwidth >= 4000000:
		selected = QualityUltra
	case effectiveBandwidth >= 1000000:
		selected = QualityHigh
	case effectiveBandwidth >= 500000:
		selected = QualityMedium
	default:
		selected = QualityLow
	}

	logrus.WithFields(logrus.Fields{
		"function":       "PresetForBandwidth",
		"bandwidth_kbps": bandwidthKbps,
		"effective_bps":  effectiveBandwidth,
		"selected":       selected.String(),
	}).Debug("Quality preset selected for bandwidth")

	return selected
}

// PresetForResolution returns the recommended quality preset for a target resolution.
//
// Returns the preset that best matches the target resolution. If the exact
// resolution is not available, returns the nearest preset that doesn't exceed
// the target.
func PresetForResolution(width, height uint16) QualityPreset {
	logrus.WithFields(logrus.Fields{
		"function": "PresetForResolution",
		"width":    width,
		"height":   height,
	}).Debug("Selecting preset for resolution")

	targetPixels := uint32(width) * uint32(height)

	var selected QualityPreset
	switch {
	case targetPixels >= 1920*1080:
		selected = QualityUltra
	case targetPixels >= 1280*720:
		selected = QualityHigh
	case targetPixels >= 640*480:
		selected = QualityMedium
	default:
		selected = QualityLow
	}

	logrus.WithFields(logrus.Fields{
		"function":      "PresetForResolution",
		"width":         width,
		"height":        height,
		"target_pixels": targetPixels,
		"selected":      selected.String(),
	}).Debug("Quality preset selected for resolution")

	return selected
}

// EstimateBandwidthUsage returns the expected bandwidth consumption for a preset.
//
// Returns the bitrate in kilobits per second that the preset will typically use.
// Actual usage may vary based on scene complexity.
func EstimateBandwidthUsage(preset QualityPreset) uint32 {
	config, err := GetPresetConfig(preset)
	if err != nil {
		return 0
	}
	return config.Bitrate / 1000 // Convert to kbps
}

// ValidatePresetForNetwork checks if a preset is suitable for current network conditions.
//
// Parameters:
//   - preset: The quality preset to validate
//   - availableBandwidthKbps: Current available bandwidth in kbps
//   - packetLossPercent: Current packet loss percentage (0-100)
//
// Returns true if the preset can be used reliably with current conditions.
func ValidatePresetForNetwork(preset QualityPreset, availableBandwidthKbps uint32, packetLossPercent float32) bool {
	logrus.WithFields(logrus.Fields{
		"function":    "ValidatePresetForNetwork",
		"preset":      preset.String(),
		"bandwidth":   availableBandwidthKbps,
		"packet_loss": packetLossPercent,
	}).Debug("Validating preset for network conditions")

	config, err := GetPresetConfig(preset)
	if err != nil {
		return false
	}

	// Require 20% bandwidth headroom
	requiredBandwidth := config.Bitrate / 1000 * 120 / 100

	// If packet loss is high, recommend lower quality
	if packetLossPercent > 5.0 && preset > QualityLow {
		logrus.WithFields(logrus.Fields{
			"function":    "ValidatePresetForNetwork",
			"preset":      preset.String(),
			"packet_loss": packetLossPercent,
		}).Debug("High packet loss, recommending lower quality")
		return false
	}

	valid := availableBandwidthKbps >= requiredBandwidth

	logrus.WithFields(logrus.Fields{
		"function":           "ValidatePresetForNetwork",
		"preset":             preset.String(),
		"required_bandwidth": requiredBandwidth,
		"available":          availableBandwidthKbps,
		"valid":              valid,
	}).Debug("Preset validation result")

	return valid
}

// AllPresets returns all available quality presets in order from lowest to highest.
func AllPresets() []QualityPreset {
	return []QualityPreset{QualityLow, QualityMedium, QualityHigh, QualityUltra}
}
