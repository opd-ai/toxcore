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
	"github.com/sirupsen/logrus"
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
	logrus.WithFields(logrus.Fields{
		"function":    "NewSimplePCMEncoder",
		"sample_rate": sampleRate,
		"bit_rate":    bitRate,
	}).Info("Creating new simple PCM encoder")

	encoder := &SimplePCMEncoder{
		bitRate:    bitRate,
		sampleRate: sampleRate,
	}

	logrus.WithFields(logrus.Fields{
		"function":    "NewSimplePCMEncoder",
		"sample_rate": encoder.sampleRate,
		"bit_rate":    encoder.bitRate,
	}).Info("Simple PCM encoder created successfully")

	return encoder
}

// Encode passes through PCM data as-is for now.
// In future phases, this will be replaced with proper Opus encoding.
func (e *SimplePCMEncoder) Encode(pcm []int16, sampleRate uint32) ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":      "SimplePCMEncoder.Encode",
		"pcm_length":    len(pcm),
		"sample_rate":   sampleRate,
		"expected_rate": e.sampleRate,
		"bit_rate":      e.bitRate,
	}).Debug("Encoding PCM audio data")

	if sampleRate != e.sampleRate {
		logrus.WithFields(logrus.Fields{
			"function":      "SimplePCMEncoder.Encode",
			"expected_rate": e.sampleRate,
			"actual_rate":   sampleRate,
			"error":         "sample rate mismatch",
		}).Error("Sample rate validation failed")
		return nil, fmt.Errorf("sample rate mismatch: expected %d, got %d", e.sampleRate, sampleRate)
	}

	// Convert []int16 to []byte (little-endian)
	data := make([]byte, len(pcm)*2)
	for i, sample := range pcm {
		data[i*2] = byte(sample)
		data[i*2+1] = byte(sample >> 8)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "SimplePCMEncoder.Encode",
		"input_size":  len(pcm),
		"output_size": len(data),
		"sample_rate": sampleRate,
		"bit_rate":    e.bitRate,
	}).Debug("PCM encoding completed successfully")

	return data, nil
}

// SetBitRate updates the target bit rate.
func (e *SimplePCMEncoder) SetBitRate(bitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":     "SimplePCMEncoder.SetBitRate",
		"old_bit_rate": e.bitRate,
		"new_bit_rate": bitRate,
	}).Info("Updating encoder bit rate")

	e.bitRate = bitRate

	logrus.WithFields(logrus.Fields{
		"function": "SimplePCMEncoder.SetBitRate",
		"bit_rate": e.bitRate,
	}).Info("Encoder bit rate updated successfully")

	return nil
}

// Close releases encoder resources.
func (e *SimplePCMEncoder) Close() error {
	logrus.WithFields(logrus.Fields{
		"function":    "SimplePCMEncoder.Close",
		"sample_rate": e.sampleRate,
		"bit_rate":    e.bitRate,
	}).Info("Closing simple PCM encoder")

	logrus.WithFields(logrus.Fields{
		"function": "SimplePCMEncoder.Close",
	}).Info("Simple PCM encoder closed successfully")

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
	logrus.WithFields(logrus.Fields{
		"function": "NewProcessor",
	}).Info("Creating new audio processor")

	decoder := opus.NewDecoder()

	processor := &Processor{
		encoder:     NewSimplePCMEncoder(48000, 64000), // 48kHz, 64kbps
		decoder:     &decoder,
		resampler:   nil,              // Created on-demand based on input sample rate
		effectChain: NewEffectChain(), // Empty effect chain, effects added as needed
		sampleRate:  48000,
		bitRate:     64000,
	}

	logrus.WithFields(logrus.Fields{
		"function":    "NewProcessor",
		"sample_rate": processor.sampleRate,
		"bit_rate":    processor.bitRate,
		"encoder":     "SimplePCMEncoder",
		"decoder":     "opus.Decoder",
	}).Info("Audio processor created successfully")

	return processor
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
	logrus.WithFields(logrus.Fields{
		"function":    "ProcessOutgoing",
		"pcm_length":  len(pcm),
		"sample_rate": sampleRate,
		"target_rate": p.sampleRate,
		"bit_rate":    p.bitRate,
	}).Info("Processing outgoing audio data")

	if p.encoder == nil {
		logrus.WithFields(logrus.Fields{
			"function": "ProcessOutgoing",
			"error":    "encoder not initialized",
		}).Error("Audio encoder validation failed")
		return nil, fmt.Errorf("audio encoder not initialized")
	}

	if len(pcm) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "ProcessOutgoing",
			"error":    "empty PCM data",
		}).Error("PCM data validation failed")
		return nil, fmt.Errorf("empty PCM data")
	}

	// Resample if the input sample rate doesn't match our target rate
	processedPCM := pcm
	if sampleRate != p.sampleRate {
		logrus.WithFields(logrus.Fields{
			"function":    "ProcessOutgoing",
			"input_rate":  sampleRate,
			"target_rate": p.sampleRate,
		}).Debug("Sample rate mismatch detected, resampling required")

		// Create or update resampler if needed
		if p.resampler == nil || p.resampler.GetInputRate() != sampleRate {
			// Determine channel count (assume mono for now, could be enhanced)
			channels := 1

			logrus.WithFields(logrus.Fields{
				"function":    "ProcessOutgoing",
				"input_rate":  sampleRate,
				"output_rate": p.sampleRate,
				"channels":    channels,
			}).Debug("Creating new resampler")

			resampler, err := NewResampler(ResamplerConfig{
				InputRate:  sampleRate,
				OutputRate: p.sampleRate,
				Channels:   channels,
				Quality:    4, // Good balance of quality and performance
			})
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"function": "ProcessOutgoing",
					"error":    err.Error(),
				}).Error("Failed to create resampler")
				return nil, fmt.Errorf("failed to create resampler: %w", err)
			}

			// Clean up old resampler if it exists
			if p.resampler != nil {
				logrus.WithFields(logrus.Fields{
					"function": "ProcessOutgoing",
				}).Debug("Closing old resampler")
				p.resampler.Close()
			}
			p.resampler = resampler
		}

		// Perform resampling
		logrus.WithFields(logrus.Fields{
			"function":   "ProcessOutgoing",
			"input_size": len(pcm),
		}).Debug("Performing audio resampling")

		resampledPCM, err := p.resampler.Resample(pcm)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "ProcessOutgoing",
				"error":    err.Error(),
			}).Error("Resampling failed")
			return nil, fmt.Errorf("resampling failed: %w", err)
		}
		processedPCM = resampledPCM

		logrus.WithFields(logrus.Fields{
			"function":    "ProcessOutgoing",
			"input_size":  len(pcm),
			"output_size": len(processedPCM),
		}).Debug("Audio resampling completed")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":    "ProcessOutgoing",
			"sample_rate": sampleRate,
		}).Debug("Sample rates match, no resampling needed")
	}

	// Apply audio effects (gain control, etc.)
	if p.effectChain != nil && p.effectChain.GetEffectCount() > 0 {
		logrus.WithFields(logrus.Fields{
			"function":     "ProcessOutgoing",
			"effect_count": p.effectChain.GetEffectCount(),
			"input_size":   len(processedPCM),
		}).Debug("Applying audio effects")

		effectsPCM, err := p.effectChain.Process(processedPCM)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "ProcessOutgoing",
				"error":    err.Error(),
			}).Error("Effects processing failed")
			return nil, fmt.Errorf("effects processing failed: %w", err)
		}
		processedPCM = effectsPCM

		logrus.WithFields(logrus.Fields{
			"function":    "ProcessOutgoing",
			"input_size":  len(effectsPCM),
			"output_size": len(processedPCM),
		}).Debug("Audio effects applied successfully")
	} else {
		logrus.WithFields(logrus.Fields{
			"function": "ProcessOutgoing",
		}).Debug("No audio effects to apply")
	}

	// Encode the processed PCM data
	logrus.WithFields(logrus.Fields{
		"function":    "ProcessOutgoing",
		"pcm_size":    len(processedPCM),
		"sample_rate": p.sampleRate,
	}).Debug("Encoding processed PCM data")

	result, err := p.encoder.Encode(processedPCM, p.sampleRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "ProcessOutgoing",
			"error":    err.Error(),
		}).Error("Audio encoding failed")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"function":        "ProcessOutgoing",
		"input_size":      len(pcm),
		"output_size":     len(result),
		"sample_rate":     sampleRate,
		"target_rate":     p.sampleRate,
		"resampled":       sampleRate != p.sampleRate,
		"effects_applied": p.effectChain != nil && p.effectChain.GetEffectCount() > 0,
	}).Info("Outgoing audio processing completed successfully")

	return result, nil
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
	logrus.WithFields(logrus.Fields{
		"function":  "ProcessIncoming",
		"data_size": len(data),
	}).Info("Processing incoming audio data")

	if len(data) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "ProcessIncoming",
			"error":    "empty audio data",
		}).Error("Audio data validation failed")
		return nil, 0, fmt.Errorf("empty audio data")
	}

	if p.decoder == nil {
		logrus.WithFields(logrus.Fields{
			"function": "ProcessIncoming",
			"error":    "decoder not initialized",
		}).Error("Audio decoder validation failed")
		return nil, 0, fmt.Errorf("audio decoder not initialized")
	}

	// Use a buffer for decoded output
	// Opus frames are typically small, so 1920 samples (40ms at 48kHz) should suffice
	outputSize := 1920 * 2 // *2 for int16 size
	output := make([]byte, outputSize)

	logrus.WithFields(logrus.Fields{
		"function":    "ProcessIncoming",
		"input_size":  len(data),
		"buffer_size": outputSize,
	}).Debug("Decoding opus audio data")

	bandwidth, isStereo, err := p.decoder.Decode(data, output)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "ProcessIncoming",
			"error":    err.Error(),
		}).Error("Opus decode failed")
		return nil, 0, fmt.Errorf("opus decode failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "ProcessIncoming",
		"bandwidth":   bandwidth.String(),
		"is_stereo":   isStereo,
		"output_size": len(output),
	}).Debug("Opus decode completed successfully")

	// Convert []byte to []int16 (little-endian)
	sampleCount := len(output) / 2
	if isStereo {
		sampleCount = sampleCount / 2 // Account for stereo channels
		logrus.WithFields(logrus.Fields{
			"function":     "ProcessIncoming",
			"stereo_mode":  true,
			"sample_count": sampleCount,
		}).Debug("Processing stereo audio data")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":     "ProcessIncoming",
			"stereo_mode":  false,
			"sample_count": sampleCount,
		}).Debug("Processing mono audio data")
	}

	pcm := make([]int16, sampleCount)
	for i := 0; i < sampleCount; i++ {
		pcm[i] = int16(output[i*2]) | int16(output[i*2+1])<<8
	}

	// Get sample rate from bandwidth
	sampleRate := uint32(bandwidth.SampleRate())

	logrus.WithFields(logrus.Fields{
		"function":    "ProcessIncoming",
		"input_size":  len(data),
		"pcm_samples": len(pcm),
		"sample_rate": sampleRate,
		"bandwidth":   bandwidth.String(),
		"is_stereo":   isStereo,
	}).Info("Incoming audio processing completed successfully")

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
	logrus.WithFields(logrus.Fields{
		"function":     "SetBitRate",
		"new_bit_rate": bitRate,
		"old_bit_rate": p.bitRate,
	}).Info("Updating audio encoder bit rate")

	if p.encoder == nil {
		logrus.WithFields(logrus.Fields{
			"function": "SetBitRate",
			"error":    "encoder not initialized",
		}).Error("Audio encoder validation failed")
		return fmt.Errorf("audio encoder not initialized")
	}

	logrus.WithFields(logrus.Fields{
		"function": "SetBitRate",
		"bit_rate": bitRate,
	}).Debug("Setting encoder bit rate")

	if err := p.encoder.SetBitRate(bitRate); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "SetBitRate",
			"bit_rate": bitRate,
			"error":    err.Error(),
		}).Error("Failed to set encoder bit rate")
		return fmt.Errorf("failed to set encoder bit rate: %w", err)
	}

	p.bitRate = bitRate

	logrus.WithFields(logrus.Fields{
		"function": "SetBitRate",
		"bit_rate": bitRate,
	}).Info("Audio encoder bit rate updated successfully")

	return nil
}

// Close releases audio processor resources.
//
// Properly cleans up encoder, decoder, resampler, and effects resources to prevent memory leaks.
func (p *Processor) Close() error {
	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Info("Closing audio processor and releasing resources")

	var errors []error

	if p.encoder != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Close",
		}).Debug("Closing audio encoder")

		if err := p.encoder.Close(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "Close",
				"error":    err.Error(),
			}).Error("Failed to close encoder")
			errors = append(errors, fmt.Errorf("failed to close encoder: %w", err))
		} else {
			logrus.WithFields(logrus.Fields{
				"function": "Close",
			}).Debug("Audio encoder closed successfully")
		}
	}

	if p.resampler != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Close",
		}).Debug("Closing audio resampler")

		if err := p.resampler.Close(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "Close",
				"error":    err.Error(),
			}).Error("Failed to close resampler")
			errors = append(errors, fmt.Errorf("failed to close resampler: %w", err))
		} else {
			logrus.WithFields(logrus.Fields{
				"function": "Close",
			}).Debug("Audio resampler closed successfully")
		}
	}

	if p.effectChain != nil {
		logrus.WithFields(logrus.Fields{
			"function":     "Close",
			"effect_count": p.effectChain.GetEffectCount(),
		}).Debug("Closing audio effect chain")

		if err := p.effectChain.Close(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "Close",
				"error":    err.Error(),
			}).Error("Failed to close effect chain")
			errors = append(errors, fmt.Errorf("failed to close effect chain: %w", err))
		} else {
			logrus.WithFields(logrus.Fields{
				"function": "Close",
			}).Debug("Audio effect chain closed successfully")
		}
	}

	if len(errors) > 0 {
		logrus.WithFields(logrus.Fields{
			"function":    "Close",
			"error_count": len(errors),
			"errors":      errors,
		}).Error("Multiple errors occurred during audio processor close")
		return fmt.Errorf("multiple close errors: %v", errors)
	}

	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Info("Audio processor closed successfully")

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
	logrus.WithFields(logrus.Fields{
		"function":    "AddEffect",
		"effect_type": fmt.Sprintf("%T", effect),
	}).Info("Adding audio effect to processing chain")

	if p.effectChain != nil {
		p.effectChain.AddEffect(effect)
		logrus.WithFields(logrus.Fields{
			"function":     "AddEffect",
			"effect_count": p.effectChain.GetEffectCount(),
		}).Debug("Audio effect added successfully")
	} else {
		logrus.WithFields(logrus.Fields{
			"function": "AddEffect",
			"error":    "effect chain not initialized",
		}).Error("Failed to add effect - chain not initialized")
	}
}

// GetEffectChain returns the current effect chain for advanced manipulation.
//
// Returns:
//   - *EffectChain: Current effect chain (may be nil)
func (p *Processor) GetEffectChain() *EffectChain {
	logrus.WithFields(logrus.Fields{
		"function":  "GetEffectChain",
		"has_chain": p.effectChain != nil,
		"effect_count": func() int {
			if p.effectChain != nil {
				return p.effectChain.GetEffectCount()
			} else {
				return 0
			}
		}(),
	}).Debug("Retrieving audio effect chain")

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
	logrus.WithFields(logrus.Fields{
		"function": "SetGain",
		"gain":     gain,
	}).Info("Setting audio gain effect")

	if p.effectChain == nil {
		logrus.WithFields(logrus.Fields{
			"function": "SetGain",
			"error":    "effect chain not initialized",
		}).Error("Effect chain validation failed")
		return fmt.Errorf("effect chain not initialized")
	}

	// Create new gain effect
	logrus.WithFields(logrus.Fields{
		"function": "SetGain",
		"gain":     gain,
	}).Debug("Creating new gain effect")

	gainEffect, err := NewGainEffect(gain)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "SetGain",
			"gain":     gain,
			"error":    err.Error(),
		}).Error("Failed to create gain effect")
		return fmt.Errorf("failed to create gain effect: %w", err)
	}

	// Clear existing effects and add the gain effect
	// This is simplified - a full implementation might maintain other effects
	logrus.WithFields(logrus.Fields{
		"function": "SetGain",
	}).Debug("Clearing existing effects")

	if err := p.effectChain.Clear(); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "SetGain",
			"error":    err.Error(),
		}).Error("Failed to clear existing effects")
		return fmt.Errorf("failed to clear existing effects: %w", err)
	}

	p.effectChain.AddEffect(gainEffect)

	logrus.WithFields(logrus.Fields{
		"function": "SetGain",
		"gain":     gain,
	}).Info("Audio gain effect set successfully")

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
	logrus.WithFields(logrus.Fields{
		"function": "EnableAutoGain",
	}).Info("Enabling automatic gain control")

	if p.effectChain == nil {
		logrus.WithFields(logrus.Fields{
			"function": "EnableAutoGain",
			"error":    "effect chain not initialized",
		}).Error("Effect chain validation failed")
		return fmt.Errorf("effect chain not initialized")
	}

	// Clear existing effects
	logrus.WithFields(logrus.Fields{
		"function": "EnableAutoGain",
	}).Debug("Clearing existing effects for AGC")

	if err := p.effectChain.Clear(); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "EnableAutoGain",
			"error":    err.Error(),
		}).Error("Failed to clear existing effects")
		return fmt.Errorf("failed to clear existing effects: %w", err)
	}

	// Add AGC effect
	logrus.WithFields(logrus.Fields{
		"function": "EnableAutoGain",
	}).Debug("Creating automatic gain control effect")

	agcEffect := NewAutoGainEffect()
	p.effectChain.AddEffect(agcEffect)

	logrus.WithFields(logrus.Fields{
		"function": "EnableAutoGain",
	}).Info("Automatic gain control enabled successfully")

	return nil
}

// DisableEffects removes all audio effects.
//
// Returns:
//   - error: Any error that occurred during effect cleanup
func (p *Processor) DisableEffects() error {
	logrus.WithFields(logrus.Fields{
		"function": "DisableEffects",
	}).Info("Disabling all audio effects")

	if p.effectChain == nil {
		logrus.WithFields(logrus.Fields{
			"function": "DisableEffects",
		}).Debug("No effect chain to disable")
		return nil // Already disabled
	}

	logrus.WithFields(logrus.Fields{
		"function":     "DisableEffects",
		"effect_count": p.effectChain.GetEffectCount(),
	}).Debug("Clearing all effects from chain")

	err := p.effectChain.Clear()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "DisableEffects",
			"error":    err.Error(),
		}).Error("Failed to clear effects")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function": "DisableEffects",
	}).Info("All audio effects disabled successfully")

	return nil
}
