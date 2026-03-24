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
// Implementation uses opd-ai/magnum for Opus encoding and decoding (pure Go).
package audio

import (
	"fmt"

	"github.com/opd-ai/magnum"
	"github.com/sirupsen/logrus"
)

// Encoder provides the audio encoder interface.
//
// Implementations encode PCM audio samples into compressed audio data
// suitable for network transmission.
type Encoder interface {
	// Encode converts PCM samples to encoded audio data
	Encode(pcm []int16, sampleRate uint32) ([]byte, error)
	// SetBitRate updates the target encoding bit rate
	SetBitRate(bitRate uint32) error
	// Close releases encoder resources
	Close() error
}

// MagnumOpusEncoder wraps the opd-ai/magnum Opus encoder to implement
// the Encoder interface with proper Opus compression.
type MagnumOpusEncoder struct {
	enc        *magnum.Encoder
	bitRate    uint32
	sampleRate uint32
	channels   int
}

// NewMagnumOpusEncoder creates a new Opus encoder using the magnum library.
//
// Parameters:
//   - sampleRate: Audio sample rate in Hz (8000, 16000, 24000, or 48000)
//   - channels: Number of audio channels (1 for mono, 2 for stereo)
//   - bitRate: Target encoding bit rate in bits per second
func NewMagnumOpusEncoder(sampleRate, bitRate uint32, channels int) (*MagnumOpusEncoder, error) {
	logrus.WithFields(logrus.Fields{
		"function":    "NewMagnumOpusEncoder",
		"sample_rate": sampleRate,
		"bit_rate":    bitRate,
		"channels":    channels,
	}).Info("Creating new Magnum Opus encoder")

	enc, err := magnum.NewEncoderWithApplication(int(sampleRate), channels, magnum.ApplicationVoIP)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "NewMagnumOpusEncoder",
			"sample_rate": sampleRate,
			"channels":    channels,
			"error":       err.Error(),
		}).Error("Failed to create magnum encoder")
		return nil, fmt.Errorf("failed to create magnum encoder: %w", err)
	}

	enc.SetBitrate(int(bitRate))

	// Enable the appropriate codec path based on sample rate
	switch sampleRate {
	case 8000, 16000:
		if err := enc.EnableSILK(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function":    "NewMagnumOpusEncoder",
				"sample_rate": sampleRate,
				"error":       err.Error(),
			}).Warn("Failed to enable SILK codec path, continuing with default encoder mode")
		}
	case 24000, 48000:
		if err := enc.EnableCELT(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function":    "NewMagnumOpusEncoder",
				"sample_rate": sampleRate,
				"error":       err.Error(),
			}).Warn("Failed to enable CELT codec path, continuing with default encoder mode")
		}
	}

	encoder := &MagnumOpusEncoder{
		enc:        enc,
		bitRate:    bitRate,
		sampleRate: sampleRate,
		channels:   channels,
	}

	logrus.WithFields(logrus.Fields{
		"function":    "NewMagnumOpusEncoder",
		"sample_rate": encoder.sampleRate,
		"bit_rate":    encoder.bitRate,
		"channels":    encoder.channels,
	}).Info("Magnum Opus encoder created successfully")

	return encoder, nil
}

// Encode converts PCM samples to Opus-encoded audio data.
//
// The magnum encoder expects 20ms frames. This method passes the PCM data
// to the encoder which buffers and returns encoded packets when ready.
func (e *MagnumOpusEncoder) Encode(pcm []int16, sampleRate uint32) ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":      "MagnumOpusEncoder.Encode",
		"pcm_length":    len(pcm),
		"sample_rate":   sampleRate,
		"expected_rate": e.sampleRate,
		"bit_rate":      e.bitRate,
	}).Debug("Encoding PCM audio data with Opus")

	if sampleRate != e.sampleRate {
		logrus.WithFields(logrus.Fields{
			"function":      "MagnumOpusEncoder.Encode",
			"expected_rate": e.sampleRate,
			"actual_rate":   sampleRate,
			"error":         "sample rate mismatch",
		}).Error("Sample rate validation failed")
		return nil, fmt.Errorf("sample rate mismatch: expected %d, got %d", e.sampleRate, sampleRate)
	}

	packet, err := e.enc.Encode(pcm)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "MagnumOpusEncoder.Encode",
			"error":    err.Error(),
		}).Error("Opus encoding failed")
		return nil, fmt.Errorf("opus encode failed: %w", err)
	}

	// If packet is nil, the encoder is still buffering; flush to get the packet
	if packet == nil {
		packet, err = e.enc.Flush()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "MagnumOpusEncoder.Encode",
				"error":    err.Error(),
			}).Error("Opus flush failed")
			return nil, fmt.Errorf("opus flush failed: %w", err)
		}
	}

	if packet == nil {
		logrus.WithFields(logrus.Fields{
			"function":   "MagnumOpusEncoder.Encode",
			"pcm_length": len(pcm),
		}).Warn("Encoder returned nil packet after flush (insufficient data for frame)")
		return nil, fmt.Errorf("encoder returned no packet: input may be too short for a complete frame")
	}

	logrus.WithFields(logrus.Fields{
		"function":    "MagnumOpusEncoder.Encode",
		"input_size":  len(pcm),
		"output_size": len(packet),
		"sample_rate": sampleRate,
		"bit_rate":    e.bitRate,
	}).Debug("Opus encoding completed successfully")

	return packet, nil
}

// SetBitRate updates the target bit rate.
func (e *MagnumOpusEncoder) SetBitRate(bitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":     "MagnumOpusEncoder.SetBitRate",
		"old_bit_rate": e.bitRate,
		"new_bit_rate": bitRate,
	}).Info("Updating encoder bit rate")

	e.enc.SetBitrate(int(bitRate))
	e.bitRate = bitRate

	logrus.WithFields(logrus.Fields{
		"function": "MagnumOpusEncoder.SetBitRate",
		"bit_rate": e.bitRate,
	}).Info("Encoder bit rate updated successfully")

	return nil
}

// Close releases encoder resources.
func (e *MagnumOpusEncoder) Close() error {
	logrus.WithFields(logrus.Fields{
		"function":    "MagnumOpusEncoder.Close",
		"sample_rate": e.sampleRate,
		"bit_rate":    e.bitRate,
	}).Info("Closing Magnum Opus encoder")

	logrus.WithFields(logrus.Fields{
		"function": "MagnumOpusEncoder.Close",
	}).Info("Magnum Opus encoder closed successfully")

	return nil
}

// Processor handles audio processing for ToxAV audio calls.
//
// Integrates encoding, decoding, resampling, and effects to provide a complete
// audio processing pipeline. Uses MagnumOpusEncoder for Opus encoding and
// magnum.Decoder for Opus decoding, with linear interpolation resampling support.
// Includes audio effects chain for gain control and other processing.
type Processor struct {
	encoder     Encoder
	decoder     *magnum.Decoder
	resampler   *Resampler   // For sample rate conversion
	effectChain *EffectChain // For audio effects processing
	sampleRate  uint32
	bitRate     uint32
	channels    int
}

// NewProcessor creates a new audio processor instance.
//
// Initializes the audio processing pipeline with:
// - MagnumOpusEncoder for Opus encoding (pure Go)
// - magnum.Decoder for Opus decoding (pure Go)
// - Empty effect chain for audio effects processing
// - Standard sample rate and bit rate settings for VoIP
func NewProcessor() *Processor {
	logrus.WithFields(logrus.Fields{
		"function": "NewProcessor",
	}).Info("Creating new audio processor")

	sampleRate := uint32(48000)
	bitRate := uint32(64000)
	channels := 1

	encoder, err := NewMagnumOpusEncoder(sampleRate, bitRate, channels)
	if err != nil {
		// This should not happen with standard parameters (48kHz, 64kbps, mono)
		// but we handle it gracefully. Callers will get "audio encoder not initialized"
		// from validateProcessingInput if they try to encode.
		logrus.WithFields(logrus.Fields{
			"function": "NewProcessor",
			"error":    err.Error(),
		}).Error("Failed to create magnum encoder, processor will have nil encoder")
	}

	decoder, err := magnum.NewDecoder(int(sampleRate), channels)
	if err != nil {
		// This should not happen with standard parameters
		// but we handle it gracefully. Callers will get "audio decoder not initialized"
		// from validateIncomingData if they try to decode.
		logrus.WithFields(logrus.Fields{
			"function": "NewProcessor",
			"error":    err.Error(),
		}).Error("Failed to create magnum decoder, processor will have nil decoder")
	} else {
		// Enable CELT decoding for 48kHz
		if celtErr := decoder.EnableCELT(); celtErr != nil {
			logrus.WithFields(logrus.Fields{
				"function": "NewProcessor",
				"error":    celtErr.Error(),
			}).Warn("Failed to enable CELT decoding")
		}
	}

	// Assign encoder through the Encoder interface
	var enc Encoder
	if encoder != nil {
		enc = encoder
	}

	processor := &Processor{
		encoder:     enc,
		decoder:     decoder,
		resampler:   nil,              // Created on-demand based on input sample rate
		effectChain: NewEffectChain(), // Empty effect chain, effects added as needed
		sampleRate:  sampleRate,
		bitRate:     bitRate,
		channels:    channels,
	}

	logrus.WithFields(logrus.Fields{
		"function":            "NewProcessor",
		"sample_rate":         processor.sampleRate,
		"bit_rate":            processor.bitRate,
		"encoder_initialized": processor.encoder != nil,
		"decoder_initialized": processor.decoder != nil,
	}).Info("Audio processor created")

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

	// Validate input parameters
	if err := p.validateProcessingInput(pcm); err != nil {
		return nil, err
	}

	// Resample if needed
	processedPCM, err := p.resampleAudioIfNeeded(pcm, sampleRate)
	if err != nil {
		return nil, err
	}

	// Apply audio effects
	processedPCM, err = p.applyAudioEffects(processedPCM)
	if err != nil {
		return nil, err
	}

	// Encode the processed audio
	result, err := p.encodeProcessedAudio(processedPCM)
	if err != nil {
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

// validateProcessingInput validates the input parameters for audio processing.
// Returns an error if the encoder is not initialized or PCM data is empty.
func (p *Processor) validateProcessingInput(pcm []int16) error {
	if p.encoder == nil {
		logrus.WithFields(logrus.Fields{
			"function": "validateProcessingInput",
			"error":    "encoder not initialized",
		}).Error("Audio encoder validation failed")
		return fmt.Errorf("audio encoder not initialized")
	}

	if len(pcm) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "validateProcessingInput",
			"error":    "empty PCM data",
		}).Error("PCM data validation failed")
		return fmt.Errorf("empty PCM data")
	}

	return nil
}

// resampleAudioIfNeeded resamples the input PCM data if the sample rate doesn't match the target rate.
// Returns the resampled PCM data or the original data if no resampling is needed.
func (p *Processor) resampleAudioIfNeeded(pcm []int16, sampleRate uint32) ([]int16, error) {
	if sampleRate == p.sampleRate {
		logrus.WithFields(logrus.Fields{
			"function":    "resampleAudioIfNeeded",
			"sample_rate": sampleRate,
		}).Debug("Sample rates match, no resampling needed")
		return pcm, nil
	}

	logrus.WithFields(logrus.Fields{
		"function":    "resampleAudioIfNeeded",
		"input_rate":  sampleRate,
		"target_rate": p.sampleRate,
	}).Debug("Sample rate mismatch detected, resampling required")

	// Create or update resampler if needed
	if err := p.ensureResamplerReady(sampleRate); err != nil {
		return nil, err
	}

	// Perform resampling
	logrus.WithFields(logrus.Fields{
		"function":   "resampleAudioIfNeeded",
		"input_size": len(pcm),
	}).Debug("Performing audio resampling")

	resampledPCM, err := p.resampler.Resample(pcm)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "resampleAudioIfNeeded",
			"error":    err.Error(),
		}).Error("Resampling failed")
		return nil, fmt.Errorf("resampling failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "resampleAudioIfNeeded",
		"input_size":  len(pcm),
		"output_size": len(resampledPCM),
	}).Debug("Audio resampling completed")

	return resampledPCM, nil
}

// ensureResamplerReady creates or updates the resampler if needed for the given sample rate.
// This method ensures the resampler is configured correctly for the input sample rate.
func (p *Processor) ensureResamplerReady(sampleRate uint32) error {
	if p.resampler != nil && p.resampler.GetInputRate() == sampleRate {
		return nil
	}

	// Determine channel count (assume mono for now, could be enhanced)
	channels := 1

	logrus.WithFields(logrus.Fields{
		"function":    "ensureResamplerReady",
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
			"function": "ensureResamplerReady",
			"error":    err.Error(),
		}).Error("Failed to create resampler")
		return fmt.Errorf("failed to create resampler: %w", err)
	}

	// Clean up old resampler if it exists
	if p.resampler != nil {
		logrus.WithFields(logrus.Fields{
			"function": "ensureResamplerReady",
		}).Debug("Closing old resampler")
		p.resampler.Close()
	}
	p.resampler = resampler

	return nil
}

// applyAudioEffects applies the configured audio effects to the PCM data.
// Returns the processed PCM data or the original data if no effects are configured.
func (p *Processor) applyAudioEffects(pcm []int16) ([]int16, error) {
	if p.effectChain == nil || p.effectChain.GetEffectCount() == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "applyAudioEffects",
		}).Debug("No audio effects to apply")
		return pcm, nil
	}

	logrus.WithFields(logrus.Fields{
		"function":     "applyAudioEffects",
		"effect_count": p.effectChain.GetEffectCount(),
		"input_size":   len(pcm),
	}).Debug("Applying audio effects")

	effectsPCM, err := p.effectChain.Process(pcm)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "applyAudioEffects",
			"error":    err.Error(),
		}).Error("Effects processing failed")
		return nil, fmt.Errorf("effects processing failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "applyAudioEffects",
		"input_size":  len(pcm),
		"output_size": len(effectsPCM),
	}).Debug("Audio effects applied successfully")

	return effectsPCM, nil
}

// encodeProcessedAudio encodes the processed PCM data using the configured encoder.
// Returns the encoded audio data ready for transmission.
func (p *Processor) encodeProcessedAudio(pcm []int16) ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":    "encodeProcessedAudio",
		"pcm_size":    len(pcm),
		"sample_rate": p.sampleRate,
	}).Debug("Encoding processed PCM data")

	result, err := p.encoder.Encode(pcm, p.sampleRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "encodeProcessedAudio",
			"error":    err.Error(),
		}).Error("Audio encoding failed")
		return nil, err
	}

	return result, nil
}

// ProcessIncoming processes incoming audio data from transmission.
//
// Decodes received audio data using the magnum Opus decoder and converts it to
// PCM format for playback. The decoder handles Opus frame formats including
// SILK, CELT, and hybrid modes.
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

	if err := p.validateIncomingData(data); err != nil {
		return nil, 0, err
	}

	pcm, err := p.decodeOpusData(data)
	if err != nil {
		return nil, 0, err
	}

	logrus.WithFields(logrus.Fields{
		"function":    "ProcessIncoming",
		"input_size":  len(data),
		"pcm_samples": len(pcm),
		"sample_rate": p.sampleRate,
	}).Info("Incoming audio processing completed successfully")

	return pcm, p.sampleRate, nil
}

// validateIncomingData checks if the audio data and decoder are valid.
func (p *Processor) validateIncomingData(data []byte) error {
	if len(data) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "ProcessIncoming",
			"error":    "empty audio data",
		}).Error("Audio data validation failed")
		return fmt.Errorf("empty audio data")
	}

	if p.decoder == nil {
		logrus.WithFields(logrus.Fields{
			"function": "ProcessIncoming",
			"error":    "decoder not initialized",
		}).Error("Audio decoder validation failed")
		return fmt.Errorf("audio decoder not initialized")
	}

	return nil
}

// decodeOpusData decodes the Opus audio data into PCM samples using magnum.
func (p *Processor) decodeOpusData(data []byte) ([]int16, error) {
	// Allocate buffer for up to 120ms of audio (maximum Opus packet duration).
	// Opus packets can legally contain 2.5–60ms frames, and multiple frames
	// per packet, so we size for the worst case to avoid decode failures.
	maxFrameSamples := int(p.sampleRate) / 1000 * 120 * p.channels
	out := make([]int16, maxFrameSamples)

	logrus.WithFields(logrus.Fields{
		"function":    "decodeOpusData",
		"input_size":  len(data),
		"buffer_size": maxFrameSamples,
		"sample_rate": p.sampleRate,
		"channels":    p.channels,
	}).Debug("Decoding opus audio data")

	n, err := p.decoder.Decode(data, out)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "decodeOpusData",
			"error":    err.Error(),
		}).Error("Opus decode failed")
		return nil, fmt.Errorf("opus decode failed: %w", err)
	}

	// Trim to actual decoded samples
	if n < len(out) {
		out = out[:n]
	}

	logrus.WithFields(logrus.Fields{
		"function":        "decodeOpusData",
		"decoded_samples": n,
		"output_size":     len(out),
	}).Debug("Opus decode completed successfully")

	return out, nil
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

	errors := p.closeAllComponents()

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

// closeAllComponents closes encoder, decoder, resampler, and effect chain, collecting any errors.
func (p *Processor) closeAllComponents() []error {
	var errors []error

	if err := p.closeEncoder(); err != nil {
		errors = append(errors, err)
	}

	if err := p.closeDecoder(); err != nil {
		errors = append(errors, err)
	}

	if err := p.closeResampler(); err != nil {
		errors = append(errors, err)
	}

	if err := p.closeEffectChain(); err != nil {
		errors = append(errors, err)
	}

	return errors
}

// closeDecoder releases the audio decoder and nils it to prevent use-after-close.
func (p *Processor) closeDecoder() error {
	if p.decoder == nil {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Debug("Closing audio decoder")

	// magnum.Decoder has no explicit Close method, but we nil the reference
	// to release internal buffers for garbage collection.
	p.decoder = nil

	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Debug("Audio decoder closed successfully")

	return nil
}

// closeEncoder closes the audio encoder component.
func (p *Processor) closeEncoder() error {
	if p.encoder == nil {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Debug("Closing audio encoder")

	if err := p.encoder.Close(); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Close",
			"error":    err.Error(),
		}).Error("Failed to close encoder")
		return fmt.Errorf("failed to close encoder: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Debug("Audio encoder closed successfully")

	return nil
}

// closeResampler closes the audio resampler component.
func (p *Processor) closeResampler() error {
	if p.resampler == nil {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Debug("Closing audio resampler")

	if err := p.resampler.Close(); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Close",
			"error":    err.Error(),
		}).Error("Failed to close resampler")
		return fmt.Errorf("failed to close resampler: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Debug("Audio resampler closed successfully")

	return nil
}

// closeEffectChain closes the audio effect chain component.
func (p *Processor) closeEffectChain() error {
	if p.effectChain == nil {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"function":     "Close",
		"effect_count": p.effectChain.GetEffectCount(),
	}).Debug("Closing audio effect chain")

	if err := p.effectChain.Close(); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Close",
			"error":    err.Error(),
		}).Error("Failed to close effect chain")
		return fmt.Errorf("failed to close effect chain: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Debug("Audio effect chain closed successfully")

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
