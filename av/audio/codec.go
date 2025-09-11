// Package audio/codec provides audio codec integration for ToxAV.
//
// This file implements codec-specific functionality including Opus packet
// formatting and integration with the core audio processor.
package audio

import (
	"fmt"

	"github.com/pion/opus"
	"github.com/sirupsen/logrus"
)

// OpusCodec provides Opus-specific audio processing functionality.
//
// Wraps the generic audio processor with Opus-specific behavior including
// packet formatting, bandwidth detection, and proper integration with
// the pion/opus decoder.
type OpusCodec struct {
	processor *Processor
}

// NewOpusCodec creates a new Opus codec instance.
//
// Initializes the codec with a standard audio processor configured
// for Opus-compatible settings (48kHz sample rate, appropriate bit rates).
func NewOpusCodec() *OpusCodec {
	logrus.WithFields(logrus.Fields{
		"function": "NewOpusCodec",
	}).Info("Creating new Opus codec instance")

	processor := NewProcessor()
	codec := &OpusCodec{
		processor: processor,
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewOpusCodec",
	}).Info("Opus codec created successfully")

	return codec
}

// EncodeFrame encodes a PCM audio frame using Opus-compatible encoding.
//
// Currently uses the SimplePCMEncoder but maintains the Opus interface
// for future enhancement with proper Opus encoding.
//
// Parameters:
//   - pcm: Raw PCM audio samples (int16 format)
//   - sampleRate: Audio sample rate in Hz (typically 48000 for Opus)
//
// Returns:
//   - []byte: Encoded audio frame
//   - error: Any error that occurred during encoding
func (c *OpusCodec) EncodeFrame(pcm []int16, sampleRate uint32) ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":     "OpusCodec.EncodeFrame",
		"sample_count": len(pcm),
		"sample_rate":  sampleRate,
	}).Debug("Encoding PCM audio frame with Opus codec")

	if c.processor == nil {
		logrus.WithFields(logrus.Fields{
			"function": "OpusCodec.EncodeFrame",
			"error":    "processor not initialized",
		}).Error("Codec processor validation failed")
		return nil, fmt.Errorf("codec processor not initialized")
	}

	result, err := c.processor.ProcessOutgoing(pcm, sampleRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "OpusCodec.EncodeFrame",
			"error":    err.Error(),
		}).Error("Audio frame encoding failed")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"function":    "OpusCodec.EncodeFrame",
		"input_size":  len(pcm),
		"output_size": len(result),
		"sample_rate": sampleRate,
	}).Debug("Audio frame encoding completed successfully")

	return result, nil
}

// DecodeFrame decodes an Opus audio frame to PCM format.
//
// Uses the pion/opus decoder to handle actual Opus-encoded data.
// Provides bandwidth and stereo channel information.
//
// Parameters:
//   - data: Opus-encoded audio frame
//
// Returns:
//   - []int16: Decoded PCM audio samples
//   - uint32: Audio sample rate in Hz
//   - error: Any error that occurred during decoding
func (c *OpusCodec) DecodeFrame(data []byte) ([]int16, uint32, error) {
	logrus.WithFields(logrus.Fields{
		"function":  "OpusCodec.DecodeFrame",
		"data_size": len(data),
	}).Debug("Decoding Opus audio frame to PCM")

	if c.processor == nil {
		logrus.WithFields(logrus.Fields{
			"function": "OpusCodec.DecodeFrame",
			"error":    "processor not initialized",
		}).Error("Codec processor validation failed")
		return nil, 0, fmt.Errorf("codec processor not initialized")
	}

	pcm, sampleRate, err := c.processor.ProcessIncoming(data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "OpusCodec.DecodeFrame",
			"error":    err.Error(),
		}).Error("Audio frame decoding failed")
		return nil, 0, err
	}

	logrus.WithFields(logrus.Fields{
		"function":    "OpusCodec.DecodeFrame",
		"input_size":  len(data),
		"output_size": len(pcm),
		"sample_rate": sampleRate,
	}).Debug("Audio frame decoding completed successfully")

	return pcm, sampleRate, nil
}

// SetBitRate updates the codec bit rate.
//
// Configures both encoder and any future decoder settings to use
// the specified bit rate.
func (c *OpusCodec) SetBitRate(bitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function": "OpusCodec.SetBitRate",
		"bit_rate": bitRate,
	}).Info("Setting Opus codec bit rate")

	if c.processor == nil {
		logrus.WithFields(logrus.Fields{
			"function": "OpusCodec.SetBitRate",
			"error":    "processor not initialized",
		}).Error("Codec processor validation failed")
		return fmt.Errorf("codec processor not initialized")
	}

	err := c.processor.SetBitRate(bitRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "OpusCodec.SetBitRate",
			"bit_rate": bitRate,
			"error":    err.Error(),
		}).Error("Failed to set codec bit rate")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function": "OpusCodec.SetBitRate",
		"bit_rate": bitRate,
	}).Info("Opus codec bit rate updated successfully")

	return nil
}

// GetSupportedSampleRates returns the sample rates supported by this codec.
//
// Opus supports multiple sample rates, but 48kHz is recommended for VoIP.
func (c *OpusCodec) GetSupportedSampleRates() []uint32 {
	rates := []uint32{8000, 12000, 16000, 24000, 48000}
	logrus.WithFields(logrus.Fields{
		"function":        "OpusCodec.GetSupportedSampleRates",
		"supported_rates": rates,
	}).Debug("Retrieving supported sample rates")
	return rates
}

// GetSupportedBitRates returns the bit rates supported by this codec.
//
// Opus supports a wide range of bit rates suitable for different use cases.
func (c *OpusCodec) GetSupportedBitRates() []uint32 {
	rates := []uint32{8000, 16000, 32000, 64000, 96000, 128000, 256000, 512000}
	logrus.WithFields(logrus.Fields{
		"function":           "OpusCodec.GetSupportedBitRates",
		"supported_bitrates": rates,
	}).Debug("Retrieving supported bit rates")
	return rates
}

// ValidateFrameSize checks if the frame size is valid for Opus encoding.
//
// Opus requires specific frame durations: 2.5, 5, 10, 20, 40, or 60 ms.
func (c *OpusCodec) ValidateFrameSize(frameSize int, sampleRate uint32, channels int) error {
	logrus.WithFields(logrus.Fields{
		"function":    "OpusCodec.ValidateFrameSize",
		"frame_size":  frameSize,
		"sample_rate": sampleRate,
		"channels":    channels,
	}).Debug("Validating Opus frame size")

	// Calculate frame duration in milliseconds
	frameDurationMs := float32(frameSize) / float32(channels) * 1000.0 / float32(sampleRate)

	// Check if frame duration is valid for Opus
	validDurations := []float32{2.5, 5.0, 10.0, 20.0, 40.0, 60.0}
	for _, duration := range validDurations {
		if frameDurationMs == duration {
			logrus.WithFields(logrus.Fields{
				"function":       "OpusCodec.ValidateFrameSize",
				"frame_duration": frameDurationMs,
				"is_valid":       true,
			}).Debug("Frame size validation successful")
			return nil
		}
	}

	logrus.WithFields(logrus.Fields{
		"function":        "OpusCodec.ValidateFrameSize",
		"frame_size":      frameSize,
		"frame_duration":  frameDurationMs,
		"valid_durations": validDurations,
		"error":           "invalid frame duration",
	}).Error("Frame size validation failed")

	return fmt.Errorf("invalid Opus frame size: %d samples (%.2f ms) - must be 2.5, 5, 10, 20, 40, or 60 ms",
		frameSize, frameDurationMs)
}

// Close releases codec resources.
func (c *OpusCodec) Close() error {
	logrus.WithFields(logrus.Fields{
		"function": "OpusCodec.Close",
	}).Info("Closing Opus codec and releasing resources")

	if c.processor != nil {
		err := c.processor.Close()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "OpusCodec.Close",
				"error":    err.Error(),
			}).Error("Failed to close codec processor")
			return err
		}
		logrus.WithFields(logrus.Fields{
			"function": "OpusCodec.Close",
		}).Info("Opus codec closed successfully")
	} else {
		logrus.WithFields(logrus.Fields{
			"function": "OpusCodec.Close",
		}).Debug("No processor to close")
	}

	return nil
}

// GetBandwidthFromSampleRate returns the appropriate Opus bandwidth for a sample rate.
//
// Maps sample rates to Opus bandwidth definitions for optimal encoding.
func GetBandwidthFromSampleRate(sampleRate uint32) opus.Bandwidth {
	logrus.WithFields(logrus.Fields{
		"function":    "GetBandwidthFromSampleRate",
		"sample_rate": sampleRate,
	}).Debug("Mapping sample rate to Opus bandwidth")

	var bandwidth opus.Bandwidth
	switch sampleRate {
	case 8000:
		bandwidth = opus.BandwidthNarrowband
	case 12000:
		bandwidth = opus.BandwidthMediumband
	case 16000:
		bandwidth = opus.BandwidthWideband
	case 24000:
		bandwidth = opus.BandwidthSuperwideband
	case 48000:
		bandwidth = opus.BandwidthFullband
	default:
		// Default to fullband for unsupported rates
		bandwidth = opus.BandwidthFullband
		logrus.WithFields(logrus.Fields{
			"function":    "GetBandwidthFromSampleRate",
			"sample_rate": sampleRate,
			"warning":     "unsupported sample rate, defaulting to fullband",
		}).Warn("Unsupported sample rate detected")
	}

	logrus.WithFields(logrus.Fields{
		"function":    "GetBandwidthFromSampleRate",
		"sample_rate": sampleRate,
		"bandwidth":   bandwidth.String(),
	}).Debug("Sample rate mapped to Opus bandwidth")

	return bandwidth
}
