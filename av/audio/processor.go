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

// Processor handles audio processing for ToxAV audio calls.
//
// Integrates encoding, decoding, resampling, and effects to provide a complete
// audio processing pipeline. Uses SimplePCMEncoder for encoding and
// pion/opus for decoding, with linear interpolation resampling support.
// Includes audio effects chain for gain control and other processing.
type Processor struct {
	encoder     Encoder
	decoder     *opus.Decoder
	resampler   *Resampler   // For sample rate conversion
	effectChain *EffectChain // For audio effects processing
	sampleRate  uint32
	bitRate     uint32
}

// NewProcessor creates a new audio processor instance.
//
// Initializes the audio processing pipeline with:
// - SimplePCMEncoder for encoding (minimal viable implementation)
// - pion/opus for decoding (pure Go)
// - Empty effect chain for audio effects processing
// - Standard sample rate and bit rate settings for VoIP
func NewProcessor() *Processor {
	decoder := opus.NewDecoder()
	return &Processor{
		encoder:     NewSimplePCMEncoder(48000, 64000), // 48kHz, 64kbps
		decoder:     &decoder,
		resampler:   nil,              // Created on-demand based on input sample rate
		effectChain: NewEffectChain(), // Empty effect chain, effects added as needed
		sampleRate:  48000,
		bitRate:     64000,
	}
}

// ProcessOutgoing processes audio data for transmission.
//
// Takes raw PCM audio samples, performs resampling if necessary to convert
// to the target sample rate (48kHz for Opus), then encodes using the
// configured encoder.
//
// Parameters:
//   - pcm: Raw PCM audio samples (int16 format)
//   - sampleRate: Original sample rate of the input audio
//
// Returns:
//   - []byte: Encoded audio data ready for transmission
//   - error: Any error that occurred during processing
func (p *Processor) ProcessOutgoing(pcm []int16, sampleRate uint32) ([]byte, error) {
	if p.encoder == nil {
		return nil, fmt.Errorf("audio encoder not initialized")
	}

	if len(pcm) == 0 {
		return nil, fmt.Errorf("empty PCM data")
	}

	// Resample if the input sample rate doesn't match our target rate
	processedPCM := pcm
	if sampleRate != p.sampleRate {
		// Create or update resampler if needed
		if p.resampler == nil || p.resampler.GetInputRate() != sampleRate {
			// Determine channel count (assume mono for now, could be enhanced)
			channels := 1

			resampler, err := NewResampler(ResamplerConfig{
				InputRate:  sampleRate,
				OutputRate: p.sampleRate,
				Channels:   channels,
				Quality:    4, // Good balance of quality and performance
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create resampler: %w", err)
			}

			// Clean up old resampler if it exists
			if p.resampler != nil {
				p.resampler.Close()
			}
			p.resampler = resampler
		}

		// Perform resampling
		resampledPCM, err := p.resampler.Resample(pcm)
		if err != nil {
			return nil, fmt.Errorf("resampling failed: %w", err)
		}
		processedPCM = resampledPCM
	}

	// Apply audio effects (gain control, etc.)
	if p.effectChain != nil && p.effectChain.GetEffectCount() > 0 {
		effectsPCM, err := p.effectChain.Process(processedPCM)
		if err != nil {
			return nil, fmt.Errorf("effects processing failed: %w", err)
		}
		processedPCM = effectsPCM
	}

	// Encode the processed PCM data
	return p.encoder.Encode(processedPCM, p.sampleRate)
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
// Properly cleans up encoder, decoder, resampler, and effects resources to prevent memory leaks.
func (p *Processor) Close() error {
	var errors []error

	if p.encoder != nil {
		if err := p.encoder.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close encoder: %w", err))
		}
	}

	if p.resampler != nil {
		if err := p.resampler.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close resampler: %w", err))
		}
	}

	if p.effectChain != nil {
		if err := p.effectChain.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close effect chain: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple close errors: %v", errors)
	}

	return nil
}

// AddEffect adds an audio effect to the processing chain.
//
// Effects are applied in the order they are added, after resampling but
// before encoding. This allows effects to process audio at the target
// sample rate for consistent behavior.
//
// Parameters:
//   - effect: Audio effect to add to the processing chain
func (p *Processor) AddEffect(effect AudioEffect) {
	if p.effectChain != nil {
		p.effectChain.AddEffect(effect)
	}
}

// GetEffectChain returns the current effect chain for advanced manipulation.
//
// Returns:
//   - *EffectChain: Current effect chain (may be nil)
func (p *Processor) GetEffectChain() *EffectChain {
	return p.effectChain
}

// SetGain sets a basic gain effect on the audio.
//
// This is a convenience method that adds or updates a gain effect.
// If a gain effect already exists, it updates the gain value.
// If no gain effect exists, it adds one to the beginning of the chain.
//
// Parameters:
//   - gain: Linear gain multiplier (0.0 = silence, 1.0 = no change, 2.0 = +6dB)
//
// Returns:
//   - error: Validation error if gain is invalid
func (p *Processor) SetGain(gain float64) error {
	if p.effectChain == nil {
		return fmt.Errorf("effect chain not initialized")
	}

	// Create new gain effect
	gainEffect, err := NewGainEffect(gain)
	if err != nil {
		return fmt.Errorf("failed to create gain effect: %w", err)
	}

	// Clear existing effects and add the gain effect
	// This is simplified - a full implementation might maintain other effects
	if err := p.effectChain.Clear(); err != nil {
		return fmt.Errorf("failed to clear existing effects: %w", err)
	}

	p.effectChain.AddEffect(gainEffect)
	return nil
}

// EnableAutoGain enables automatic gain control.
//
// This replaces any existing effects with an automatic gain control effect.
// AGC automatically adjusts audio levels for consistent output.
//
// Returns:
//   - error: Any error that occurred during AGC setup
func (p *Processor) EnableAutoGain() error {
	if p.effectChain == nil {
		return fmt.Errorf("effect chain not initialized")
	}

	// Clear existing effects
	if err := p.effectChain.Clear(); err != nil {
		return fmt.Errorf("failed to clear existing effects: %w", err)
	}

	// Add AGC effect
	agcEffect := NewAutoGainEffect()
	p.effectChain.AddEffect(agcEffect)
	return nil
}

// DisableEffects removes all audio effects.
//
// Returns:
//   - error: Any error that occurred during effect cleanup
func (p *Processor) DisableEffects() error {
	if p.effectChain == nil {
		return nil // Already disabled
	}

	return p.effectChain.Clear()
}
