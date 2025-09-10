// Package audio provides audio processing capabilities for ToxAV.
//
// This package handles audio encoding, decoding, resampling, and effects
// processing for audio/video calls. It integrates with pure Go audio
// libraries to provide Opus codec support and audio effects.
//
// The audio processing pipeline:
//
//	PCM Input → Resampling → Effects → Opus Encoding → RTP Packetization
//	PCM Output ← Resampling ← Effects ← Opus Decoding ← RTP Depacketization
//
// Implementation uses pion/opus for decoding and a simplified encoder for encoding.
package audio

import (
	"fmt"

	"github.com/pion/opus"
)

// Encoder provides a simplified audio encoder interface.
//
// For Phase 2 implementation, this starts as a PCM passthrough encoder
// that can be enhanced with proper Opus encoding in future phases.
// This follows the SIMPLICITY RULE: provide basic functionality first.
type Encoder interface {
	// Encode converts PCM samples to encoded audio data
	Encode(pcm []int16, sampleRate uint32) ([]byte, error)
	// SetBitRate updates the target encoding bit rate
	SetBitRate(bitRate uint32) error
	// Close releases encoder resources
	Close() error
}

// SimplePCMEncoder is a basic encoder that passes through PCM data.
// This provides immediate functionality while maintaining the interface
// for future Opus encoder integration.
type SimplePCMEncoder struct {
	bitRate    uint32
	sampleRate uint32
}

// NewSimplePCMEncoder creates a new PCM passthrough encoder.
func NewSimplePCMEncoder(sampleRate, bitRate uint32) *SimplePCMEncoder {
	return &SimplePCMEncoder{
		bitRate:    bitRate,
		sampleRate: sampleRate,
	}
}

// Encode passes through PCM data as-is for now.
// In future phases, this will be replaced with proper Opus encoding.
func (e *SimplePCMEncoder) Encode(pcm []int16, sampleRate uint32) ([]byte, error) {
	if sampleRate != e.sampleRate {
		return nil, fmt.Errorf("sample rate mismatch: expected %d, got %d", e.sampleRate, sampleRate)
	}

	// Convert []int16 to []byte (little-endian)
	data := make([]byte, len(pcm)*2)
	for i, sample := range pcm {
		data[i*2] = byte(sample)
		data[i*2+1] = byte(sample >> 8)
	}

	return data, nil
}

// SetBitRate updates the target bit rate.
func (e *SimplePCMEncoder) SetBitRate(bitRate uint32) error {
	e.bitRate = bitRate
	return nil
}

// Close releases encoder resources.
func (e *SimplePCMEncoder) Close() error {
	return nil
}

// Processor handles audio encoding/decoding and effects processing.
//
// Uses pion/opus for decoding and SimplePCMEncoder for encoding.
// This provides immediate functionality with a clean interface for
// future enhancements.
type Processor struct {
	encoder    Encoder
	decoder    opus.Decoder
	sampleRate uint32
	bitRate    uint32
}

// NewProcessor creates a new audio processor instance.
//
// Initializes the audio processing pipeline with:
// - pion/opus for decoding (pure Go)
// - SimplePCMEncoder for encoding (minimal viable implementation)
// - Standard sample rate and bit rate settings for VoIP
func NewProcessor() *Processor {
	return &Processor{
		encoder:    NewSimplePCMEncoder(48000, 64000), // 48kHz, 64kbps
		decoder:    opus.NewDecoder(),
		sampleRate: 48000,
		bitRate:    64000,
	}
}

// ProcessOutgoing processes outgoing audio data for transmission.
//
// Encodes PCM audio data using the configured encoder and prepares it
// for RTP transmission. Currently uses SimplePCMEncoder which can be
// enhanced with proper Opus encoding in future phases.
//
// Parameters:
//   - pcm: Raw PCM audio samples (int16 format)
//   - sampleRate: Audio sample rate in Hz
//
// Returns:
//   - []byte: Encoded audio data ready for transmission
//   - error: Any error that occurred during processing
func (p *Processor) ProcessOutgoing(pcm []int16, sampleRate uint32) ([]byte, error) {
	if p.encoder == nil {
		return nil, fmt.Errorf("audio encoder not initialized")
	}

	return p.encoder.Encode(pcm, sampleRate)
}

// ProcessIncoming processes incoming audio data from transmission.
//
// Decodes received audio data using pion/opus decoder and converts it to
// PCM format for playback. The decoder handles various Opus frame formats
// and provides bandwidth/stereo information.
//
// Parameters:
//   - data: Encoded audio data received from network
//
// Returns:
//   - []int16: Decoded PCM audio samples
//   - uint32: Audio sample rate in Hz
//   - error: Any error that occurred during processing
func (p *Processor) ProcessIncoming(data []byte) ([]int16, uint32, error) {
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("empty audio data")
	}

	// Use a buffer for decoded output
	// Opus frames are typically small, so 1920 samples (40ms at 48kHz) should suffice
	output := make([]byte, 1920*2) // *2 for int16 size

	bandwidth, isStereo, err := p.decoder.Decode(data, output)
	if err != nil {
		return nil, 0, fmt.Errorf("opus decode failed: %w", err)
	}

	// Convert []byte to []int16 (little-endian)
	sampleCount := len(output) / 2
	if isStereo {
		sampleCount = sampleCount / 2 // Account for stereo channels
	}

	pcm := make([]int16, sampleCount)
	for i := 0; i < sampleCount; i++ {
		pcm[i] = int16(output[i*2]) | int16(output[i*2+1])<<8
	}

	// Get sample rate from bandwidth
	sampleRate := uint32(bandwidth.SampleRate())

	return pcm, sampleRate, nil
}

// SetBitRate updates the audio encoding bit rate.
//
// Adjusts the encoder settings to use the specified bit rate for encoding.
// Also updates the processor's internal bit rate tracking.
//
// Parameters:
//   - bitRate: Target bit rate in bits per second
//
// Returns:
//   - error: Any error that occurred during bit rate update
func (p *Processor) SetBitRate(bitRate uint32) error {
	if p.encoder == nil {
		return fmt.Errorf("audio encoder not initialized")
	}

	if err := p.encoder.SetBitRate(bitRate); err != nil {
		return fmt.Errorf("failed to set encoder bit rate: %w", err)
	}

	p.bitRate = bitRate
	return nil
}

// Close releases audio processor resources.
//
// Properly cleans up encoder and decoder resources to prevent memory leaks.
func (p *Processor) Close() error {
	if p.encoder != nil {
		if err := p.encoder.Close(); err != nil {
			return fmt.Errorf("failed to close encoder: %w", err)
		}
	}
	return nil
}
