// Package audio provides audio processing capabilities for ToxAV.
//
// This module implements audio resampling functionality to convert between
// different sample rates. This is essential for ToxAV audio processing as
// different audio sources may use different sample rates (8kHz, 16kHz, 44.1kHz, 48kHz)
// but Opus encoding typically expects 48kHz input.
package audio

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// Resampler provides audio sample rate conversion functionality.
//
// Uses linear interpolation for sample rate conversion, which provides
// good quality for voice communication without external dependencies.
// This follows the SIMPLICITY RULE: provide basic functionality that works well.
type Resampler struct {
	inputRate   uint32
	outputRate  uint32
	channels    int
	quality     int     // Quality setting (currently unused, for future enhancement)
	lastSamples []int16 // Previous samples for interpolation
	position    float64 // Current fractional position in input stream
}

// ResamplerConfig holds configuration for creating a resampler.
type ResamplerConfig struct {
	InputRate  uint32 // Input sample rate in Hz
	OutputRate uint32 // Output sample rate in Hz
	Channels   int    // Number of audio channels (1=mono, 2=stereo)
	Quality    int    // Resampling quality (0-10, default: 4)
}

// NewResampler creates a new audio resampler instance.
//
// Initializes a resampler to convert audio from inputRate to outputRate.
// Higher quality values provide better audio quality at the cost of more CPU usage.
//
// Parameters:
//   - config: Resampler configuration
//
// Returns:
//   - *Resampler: New resampler instance
//   - error: Any error that occurred during initialization
func NewResampler(config ResamplerConfig) (*Resampler, error) {
	logrus.WithFields(logrus.Fields{
		"function":    "NewResampler",
		"input_rate":  config.InputRate,
		"output_rate": config.OutputRate,
		"channels":    config.Channels,
		"quality":     config.Quality,
	}).Info("Creating new audio resampler")

	if config.InputRate == 0 || config.OutputRate == 0 {
		logrus.WithFields(logrus.Fields{
			"function":    "NewResampler",
			"input_rate":  config.InputRate,
			"output_rate": config.OutputRate,
			"error":       "invalid sample rates",
		}).Error("Sample rate validation failed")
		return nil, fmt.Errorf("invalid sample rates: input=%d, output=%d", config.InputRate, config.OutputRate)
	}

	if config.Channels < 1 || config.Channels > 2 {
		logrus.WithFields(logrus.Fields{
			"function": "NewResampler",
			"channels": config.Channels,
			"error":    "unsupported channel count",
		}).Error("Channel count validation failed")
		return nil, fmt.Errorf("unsupported channel count: %d (must be 1 or 2)", config.Channels)
	}

	// Set default quality if not specified
	quality := config.Quality
	if quality == 0 {
		quality = 4 // Good balance between quality and performance
		logrus.WithFields(logrus.Fields{
			"function":        "NewResampler",
			"default_quality": quality,
		}).Debug("Using default quality setting")
	}
	if quality < 0 || quality > 10 {
		logrus.WithFields(logrus.Fields{
			"function": "NewResampler",
			"quality":  quality,
			"error":    "invalid quality setting",
		}).Error("Quality validation failed")
		return nil, fmt.Errorf("invalid quality setting: %d (must be 0-10)", quality)
	}

	resampler := &Resampler{
		inputRate:   config.InputRate,
		outputRate:  config.OutputRate,
		channels:    config.Channels,
		quality:     quality,
		lastSamples: make([]int16, config.Channels),
		position:    0.0,
	}

	logrus.WithFields(logrus.Fields{
		"function":    "NewResampler",
		"input_rate":  resampler.inputRate,
		"output_rate": resampler.outputRate,
		"channels":    resampler.channels,
		"quality":     resampler.quality,
		"ratio":       float64(config.InputRate) / float64(config.OutputRate),
	}).Info("Audio resampler created successfully")

	return resampler, nil
}

// validateResamplerInput checks if the input samples are valid for resampling.
//
// Validates that input is not empty and that samples are properly aligned
// to the configured channel count.
//
// Parameters:
//   - input: Input PCM audio samples
//   - channels: Number of audio channels
//
// Returns:
//   - error: Validation error, or nil if input is valid
func validateResamplerInput(input []int16, channels int) error {
	logrus.WithFields(logrus.Fields{
		"function":     "validateResamplerInput",
		"input_length": len(input),
		"channels":     channels,
	}).Debug("Validating resampler input")

	if len(input) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "validateResamplerInput",
			"error":    "empty input samples",
		}).Error("Input validation failed")
		return fmt.Errorf("empty input samples")
	}

	// Check if samples are properly aligned for channels
	if len(input)%channels != 0 {
		logrus.WithFields(logrus.Fields{
			"function":     "validateResamplerInput",
			"input_length": len(input),
			"channels":     channels,
			"remainder":    len(input) % channels,
			"error":        "samples not aligned to channel count",
		}).Error("Input alignment validation failed")
		return fmt.Errorf("input samples (%d) not aligned to channel count (%d)", len(input), channels)
	}

	logrus.WithFields(logrus.Fields{
		"function":     "validateResamplerInput",
		"input_length": len(input),
		"channels":     channels,
		"frames":       len(input) / channels,
	}).Debug("Input validation successful")

	return nil
}

// handleSameRateResampling optimizes the case where input and output rates are identical.
//
// When no rate conversion is needed, this function returns a copy of the input
// samples without performing any interpolation.
//
// Parameters:
//   - input: Input PCM audio samples
//
// Returns:
//   - []int16: Copy of input samples
//   - bool: true if same-rate optimization was applied
func handleSameRateResampling(input []int16, inputRate, outputRate uint32) ([]int16, bool) {
	logrus.WithFields(logrus.Fields{
		"function":    "handleSameRateResampling",
		"input_rate":  inputRate,
		"output_rate": outputRate,
		"input_size":  len(input),
	}).Debug("Checking for same-rate optimization")

	if inputRate == outputRate {
		result := make([]int16, len(input))
		copy(result, input)

		logrus.WithFields(logrus.Fields{
			"function":    "handleSameRateResampling",
			"input_rate":  inputRate,
			"output_rate": outputRate,
			"input_size":  len(input),
			"output_size": len(result),
		}).Debug("Applied same-rate optimization")

		return result, true
	}

	logrus.WithFields(logrus.Fields{
		"function":    "handleSameRateResampling",
		"input_rate":  inputRate,
		"output_rate": outputRate,
		"ratio":       float64(inputRate) / float64(outputRate),
	}).Debug("Different rates detected, interpolation required")

	return nil, false
}

// interpolateSample performs linear interpolation for a single channel sample.
//
// Calculates the interpolated sample value based on the current position
// and fractional component using linear interpolation between adjacent samples.
//
// Parameters:
//   - input: Input PCM audio samples
//   - inputIndex: Integer part of input position
//   - frac: Fractional part of input position
//   - ch: Channel index
//   - channels: Number of audio channels
//   - inputFrames: Total number of input frames
//   - lastSamples: Previous samples for boundary conditions
//
// Returns:
//   - int16: Interpolated sample value
func interpolateSample(input []int16, inputIndex int, frac float64, ch, channels, inputFrames int, lastSamples []int16) int16 {
	if inputIndex < 0 {
		return getSampleFromPrevious(lastSamples, ch, inputIndex)
	}
	if inputIndex >= inputFrames-1 {
		return getSampleAtUpperBoundary(input, inputIndex, ch, channels, inputFrames)
	}
	return performLinearInterpolation(input, inputIndex, frac, ch, channels)
}

// getSampleFromPrevious retrieves a sample from the previous batch when index is negative.
func getSampleFromPrevious(lastSamples []int16, ch, inputIndex int) int16 {
	if len(lastSamples) > ch {
		sample := lastSamples[ch]
		logrus.WithFields(logrus.Fields{
			"function":    "getSampleFromPrevious",
			"input_index": inputIndex,
			"channel":     ch,
			"source":      "last_samples",
			"sample":      sample,
		}).Debug("Using last sample for boundary condition")
		return sample
	}
	return 0
}

// getSampleAtUpperBoundary retrieves a sample when at or beyond the upper boundary.
func getSampleAtUpperBoundary(input []int16, inputIndex, ch, channels, inputFrames int) int16 {
	if inputIndex < inputFrames {
		sample := input[inputIndex*channels+ch]
		logrus.WithFields(logrus.Fields{
			"function":    "getSampleAtUpperBoundary",
			"input_index": inputIndex,
			"channel":     ch,
			"source":      "current_frame",
			"sample":      sample,
		}).Debug("Using current frame sample at boundary")
		return sample
	}
	if len(input) > ch {
		sample := input[len(input)-channels+ch]
		logrus.WithFields(logrus.Fields{
			"function":    "getSampleAtUpperBoundary",
			"input_index": inputIndex,
			"channel":     ch,
			"source":      "last_available",
			"sample":      sample,
		}).Debug("Using last available sample")
		return sample
	}
	return 0
}

// performLinearInterpolation calculates the interpolated value between two adjacent samples.
func performLinearInterpolation(input []int16, inputIndex int, frac float64, ch, channels int) int16 {
	sample1 := input[inputIndex*channels+ch]
	sample2 := input[(inputIndex+1)*channels+ch]
	interpolated := float64(sample1)*(1.0-frac) + float64(sample2)*frac
	sample := int16(interpolated)

	logrus.WithFields(logrus.Fields{
		"function":     "performLinearInterpolation",
		"input_index":  inputIndex,
		"channel":      ch,
		"frac":         frac,
		"sample1":      sample1,
		"sample2":      sample2,
		"interpolated": sample,
	}).Debug("Performed linear interpolation")

	return sample
}

// updateResamplerState updates the resampler's internal state after processing.
//
// Updates the position counter and stores the last samples for use in
// the next resampling call to maintain continuity.
//
// Parameters:
//   - r: Resampler instance
//   - input: Input PCM audio samples
//   - inputFrames: Number of input frames processed
func updateResamplerState(r *Resampler, input []int16, inputFrames int) {
	logrus.WithFields(logrus.Fields{
		"function":     "updateResamplerState",
		"old_position": r.position,
		"input_frames": inputFrames,
		"input_length": len(input),
		"channels":     r.channels,
	}).Debug("Updating resampler state")

	// Update position for next call (subtract processed frames)
	oldPosition := r.position
	r.position -= float64(inputFrames)

	// Store last samples for next interpolation
	if len(input) >= r.channels {
		copy(r.lastSamples, input[len(input)-r.channels:])
		logrus.WithFields(logrus.Fields{
			"function":     "updateResamplerState",
			"old_position": oldPosition,
			"new_position": r.position,
			"last_samples": r.lastSamples,
		}).Debug("Updated resampler state and stored last samples")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":     "updateResamplerState",
			"input_length": len(input),
			"channels":     r.channels,
			"error":        "insufficient input for last samples",
		}).Warn("Could not store last samples due to insufficient input")
	}
}

// Resample converts audio samples from input rate to output rate.
//
// Converts the provided PCM audio samples from the configured input sample rate
// to the configured output sample rate using linear interpolation.
//
// Parameters:
//   - input: Input PCM audio samples (int16 format)
//
// Returns:
//   - []int16: Resampled PCM audio samples
//   - error: Any error that occurred during resampling
func (r *Resampler) Resample(input []int16) ([]int16, error) {
	logrus.WithFields(logrus.Fields{
		"function":     "Resample",
		"input_length": len(input),
		"input_rate":   r.inputRate,
		"output_rate":  r.outputRate,
		"channels":     r.channels,
		"position":     r.position,
	}).Info("Starting audio resampling")

	// Validate input samples
	if err := validateResamplerInput(input, r.channels); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Resample",
			"error":    err.Error(),
		}).Error("Input validation failed")
		return nil, err
	}

	// Handle same-rate optimization
	if result, handled := handleSameRateResampling(input, r.inputRate, r.outputRate); handled {
		logrus.WithFields(logrus.Fields{
			"function":     "Resample",
			"input_size":   len(input),
			"output_size":  len(result),
			"optimization": "same_rate",
		}).Info("Resampling completed using same-rate optimization")
		return result, nil
	}

	// Calculate resampling ratio
	ratio := float64(r.inputRate) / float64(r.outputRate)

	// Calculate number of input frames
	inputFrames := len(input) / r.channels

	// Estimate output size
	outputFrames := int(float64(inputFrames)/ratio + 0.5)
	output := make([]int16, 0, outputFrames*r.channels)

	logrus.WithFields(logrus.Fields{
		"function":      "Resample",
		"ratio":         ratio,
		"input_frames":  inputFrames,
		"output_frames": outputFrames,
		"channels":      r.channels,
	}).Debug("Starting linear interpolation resampling")

	// Perform linear interpolation resampling
	for outputFrame := 0; outputFrame < outputFrames; outputFrame++ {
		// Calculate input position
		inputPos := r.position
		inputIndex := int(inputPos)
		frac := inputPos - float64(inputIndex)

		// Interpolate each channel
		for ch := 0; ch < r.channels; ch++ {
			sample := interpolateSample(input, inputIndex, frac, ch, r.channels, inputFrames, r.lastSamples)
			output = append(output, sample)
		}

		// Advance position
		r.position += ratio

		// Log progress for long operations
		if outputFrames > 1000 && outputFrame%500 == 0 {
			logrus.WithFields(logrus.Fields{
				"function":     "Resample",
				"progress":     float64(outputFrame) / float64(outputFrames) * 100,
				"output_frame": outputFrame,
				"total_frames": outputFrames,
			}).Debug("Resampling progress")
		}
	}

	// Update internal state
	updateResamplerState(r, input, inputFrames)

	logrus.WithFields(logrus.Fields{
		"function":       "Resample",
		"input_length":   len(input),
		"output_length":  len(output),
		"input_frames":   inputFrames,
		"output_frames":  len(output) / r.channels,
		"ratio":          ratio,
		"final_position": r.position,
	}).Info("Audio resampling completed successfully")

	return output, nil
}

// GetInputRate returns the configured input sample rate.
func (r *Resampler) GetInputRate() uint32 {
	logrus.WithFields(logrus.Fields{
		"function":   "GetInputRate",
		"input_rate": r.inputRate,
	}).Debug("Retrieved input sample rate")
	return r.inputRate
}

// GetOutputRate returns the configured output sample rate.
func (r *Resampler) GetOutputRate() uint32 {
	logrus.WithFields(logrus.Fields{
		"function":    "GetOutputRate",
		"output_rate": r.outputRate,
	}).Debug("Retrieved output sample rate")
	return r.outputRate
}

// GetChannels returns the configured number of channels.
func (r *Resampler) GetChannels() int {
	logrus.WithFields(logrus.Fields{
		"function": "GetChannels",
		"channels": r.channels,
	}).Debug("Retrieved channel count")
	return r.channels
}

// GetQuality returns the configured resampling quality.
func (r *Resampler) GetQuality() int {
	logrus.WithFields(logrus.Fields{
		"function": "GetQuality",
		"quality":  r.quality,
	}).Debug("Retrieved resampling quality")
	return r.quality
}

// CalculateOutputSize estimates the output size for a given input size.
//
// This is useful for pre-allocating buffers and understanding the
// size relationship between input and output.
//
// Parameters:
//   - inputSize: Number of input samples
//
// Returns:
//   - int: Estimated number of output samples
func (r *Resampler) CalculateOutputSize(inputSize int) int {
	logrus.WithFields(logrus.Fields{
		"function":    "CalculateOutputSize",
		"input_size":  inputSize,
		"input_rate":  r.inputRate,
		"output_rate": r.outputRate,
	}).Debug("Calculating output size")

	if r.inputRate == r.outputRate {
		logrus.WithFields(logrus.Fields{
			"function":     "CalculateOutputSize",
			"input_size":   inputSize,
			"output_size":  inputSize,
			"optimization": "same_rate",
		}).Debug("Same rate detected, output size equals input size")
		return inputSize
	}

	// Calculate the ratio and apply it
	ratio := float64(r.outputRate) / float64(r.inputRate)
	outputSize := int(float64(inputSize)*ratio + 0.5) // Round to nearest integer

	logrus.WithFields(logrus.Fields{
		"function":    "CalculateOutputSize",
		"input_size":  inputSize,
		"output_size": outputSize,
		"ratio":       ratio,
	}).Debug("Calculated output size using ratio")

	return outputSize
}

// Reset resets the resampler's internal state.
//
// This is useful when starting a new audio stream or when there's
// a discontinuity in the audio data.
func (r *Resampler) Reset() error {
	logrus.WithFields(logrus.Fields{
		"function":     "Reset",
		"old_position": r.position,
		"channels":     r.channels,
	}).Info("Resetting resampler state")

	r.position = 0.0
	// Clear last samples
	for i := range r.lastSamples {
		r.lastSamples[i] = 0
	}

	logrus.WithFields(logrus.Fields{
		"function":        "Reset",
		"new_position":    r.position,
		"cleared_samples": len(r.lastSamples),
	}).Info("Resampler state reset successfully")

	return nil
}

// Close releases resampler resources.
//
// After calling Close, the resampler should not be used.
func (r *Resampler) Close() error {
	logrus.WithFields(logrus.Fields{
		"function":    "Close",
		"input_rate":  r.inputRate,
		"output_rate": r.outputRate,
		"channels":    r.channels,
	}).Info("Closing resampler")

	// No resources to clean up for our simple implementation
	logrus.WithFields(logrus.Fields{
		"function": "Close",
	}).Info("Resampler closed successfully")

	return nil
}

// Common resampling configurations for ToxAV

// NewTelephoneToOpusResampler creates a resampler for telephone quality (8kHz) to Opus (48kHz).
func NewTelephoneToOpusResampler(channels int) (*Resampler, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewTelephoneToOpusResampler",
		"channels": channels,
		"type":     "telephone_to_opus",
	}).Info("Creating telephone to Opus resampler")

	return NewResampler(ResamplerConfig{
		InputRate:  8000,
		OutputRate: 48000,
		Channels:   channels,
		Quality:    4,
	})
}

// NewCDToOpusResampler creates a resampler for CD quality (44.1kHz) to Opus (48kHz).
func NewCDToOpusResampler(channels int) (*Resampler, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewCDToOpusResampler",
		"channels": channels,
		"type":     "cd_to_opus",
	}).Info("Creating CD to Opus resampler")

	return NewResampler(ResamplerConfig{
		InputRate:  44100,
		OutputRate: 48000,
		Channels:   channels,
		Quality:    6, // Higher quality for CD audio
	})
}

// NewWidebandToOpusResampler creates a resampler for wideband audio (16kHz) to Opus (48kHz).
func NewWidebandToOpusResampler(channels int) (*Resampler, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewWidebandToOpusResampler",
		"channels": channels,
		"type":     "wideband_to_opus",
	}).Info("Creating wideband to Opus resampler")

	return NewResampler(ResamplerConfig{
		InputRate:  16000,
		OutputRate: 48000,
		Channels:   channels,
		Quality:    4,
	})
}

// NewOpusToPlaybackResampler creates a resampler for Opus (48kHz) to common playback rates.
func NewOpusToPlaybackResampler(outputRate uint32, channels int) (*Resampler, error) {
	logrus.WithFields(logrus.Fields{
		"function":    "NewOpusToPlaybackResampler",
		"output_rate": outputRate,
		"channels":    channels,
		"type":        "opus_to_playback",
	}).Info("Creating Opus to playback resampler")

	return NewResampler(ResamplerConfig{
		InputRate:  48000,
		OutputRate: outputRate,
		Channels:   channels,
		Quality:    4,
	})
}
