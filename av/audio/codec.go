// Package audio/codec provides audio codec integration for ToxAV.
//
// This file implements codec-specific functionality including Opus packet
// formatting and integration with the core audio processor.
package audio

import (
	"fmt"

	"github.com/pion/opus"
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
	return &OpusCodec{
		processor: NewProcessor(),
	}
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
	return c.processor.ProcessOutgoing(pcm, sampleRate)
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
	return c.processor.ProcessIncoming(data)
}

// SetBitRate updates the codec bit rate.
//
// Configures both encoder and any future decoder settings to use
// the specified bit rate.
func (c *OpusCodec) SetBitRate(bitRate uint32) error {
	return c.processor.SetBitRate(bitRate)
}

// GetSupportedSampleRates returns the sample rates supported by this codec.
//
// Opus supports multiple sample rates, but 48kHz is recommended for VoIP.
func (c *OpusCodec) GetSupportedSampleRates() []uint32 {
	return []uint32{8000, 12000, 16000, 24000, 48000}
}

// GetSupportedBitRates returns the bit rates supported by this codec.
//
// Opus supports a wide range of bit rates suitable for different use cases.
func (c *OpusCodec) GetSupportedBitRates() []uint32 {
	return []uint32{8000, 16000, 32000, 64000, 96000, 128000, 256000, 512000}
}

// ValidateFrameSize checks if the frame size is valid for Opus encoding.
//
// Opus requires specific frame durations: 2.5, 5, 10, 20, 40, or 60 ms.
func (c *OpusCodec) ValidateFrameSize(frameSize int, sampleRate uint32, channels int) error {
	// Calculate frame duration in milliseconds
	frameDurationMs := float32(frameSize) / float32(channels) * 1000.0 / float32(sampleRate)

	// Check if frame duration is valid for Opus
	validDurations := []float32{2.5, 5.0, 10.0, 20.0, 40.0, 60.0}
	for _, duration := range validDurations {
		if frameDurationMs == duration {
			return nil
		}
	}

	return fmt.Errorf("invalid Opus frame size: %d samples (%.2f ms) - must be 2.5, 5, 10, 20, 40, or 60 ms",
		frameSize, frameDurationMs)
}

// Close releases codec resources.
func (c *OpusCodec) Close() error {
	if c.processor != nil {
		return c.processor.Close()
	}
	return nil
}

// GetBandwidthFromSampleRate returns the appropriate Opus bandwidth for a sample rate.
//
// Maps sample rates to Opus bandwidth definitions for optimal encoding.
func GetBandwidthFromSampleRate(sampleRate uint32) opus.Bandwidth {
	switch sampleRate {
	case 8000:
		return opus.BandwidthNarrowband
	case 12000:
		return opus.BandwidthMediumband
	case 16000:
		return opus.BandwidthWideband
	case 24000:
		return opus.BandwidthSuperwideband
	case 48000:
		return opus.BandwidthFullband
	default:
		// Default to fullband for unsupported rates
		return opus.BandwidthFullband
	}
}
