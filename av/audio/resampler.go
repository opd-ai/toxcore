// Package audio provides audio processing capabilities for ToxAV.
//
// This module implements audio resampling functionality to convert between
// different sample rates. This is essential for ToxAV audio processing as
// different audio sources may use different sample rates (8kHz, 16kHz, 44.1kHz, 48kHz)
// but Opus encoding typically expects 48kHz input.
package audio

import (
	"fmt"
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
	if config.InputRate == 0 || config.OutputRate == 0 {
		return nil, fmt.Errorf("invalid sample rates: input=%d, output=%d", config.InputRate, config.OutputRate)
	}

	if config.Channels < 1 || config.Channels > 2 {
		return nil, fmt.Errorf("unsupported channel count: %d (must be 1 or 2)", config.Channels)
	}

	// Set default quality if not specified
	quality := config.Quality
	if quality == 0 {
		quality = 4 // Good balance between quality and performance
	}
	if quality < 0 || quality > 10 {
		return nil, fmt.Errorf("invalid quality setting: %d (must be 0-10)", quality)
	}

	return &Resampler{
		inputRate:   config.InputRate,
		outputRate:  config.OutputRate,
		channels:    config.Channels,
		quality:     quality,
		lastSamples: make([]int16, config.Channels),
		position:    0.0,
	}, nil
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
	if len(input) == 0 {
		return nil, fmt.Errorf("empty input samples")
	}

	// Check if samples are properly aligned for channels
	if len(input)%r.channels != 0 {
		return nil, fmt.Errorf("input samples (%d) not aligned to channel count (%d)", len(input), r.channels)
	}

	// If input and output rates are the same, return input as-is
	if r.inputRate == r.outputRate {
		result := make([]int16, len(input))
		copy(result, input)
		return result, nil
	}

	// Calculate resampling ratio
	ratio := float64(r.inputRate) / float64(r.outputRate)

	// Calculate number of input frames
	inputFrames := len(input) / r.channels

	// Estimate output size
	outputFrames := int(float64(inputFrames)/ratio + 0.5)
	output := make([]int16, 0, outputFrames*r.channels)

	// Perform linear interpolation resampling
	for outputFrame := 0; outputFrame < outputFrames; outputFrame++ {
		// Calculate input position
		inputPos := r.position
		inputIndex := int(inputPos)
		frac := inputPos - float64(inputIndex)

		// Interpolate each channel
		for ch := 0; ch < r.channels; ch++ {
			var sample int16

			if inputIndex < 0 {
				// Use last samples from previous call
				if len(r.lastSamples) > ch {
					sample = r.lastSamples[ch]
				}
			} else if inputIndex >= inputFrames-1 {
				// Use last available sample
				if inputIndex < inputFrames {
					sample = input[inputIndex*r.channels+ch]
				} else if len(input) > ch {
					sample = input[len(input)-r.channels+ch]
				}
			} else {
				// Linear interpolation between two samples
				sample1 := input[inputIndex*r.channels+ch]
				sample2 := input[(inputIndex+1)*r.channels+ch]

				// Interpolate
				interpolated := float64(sample1)*(1.0-frac) + float64(sample2)*frac
				sample = int16(interpolated)
			}

			output = append(output, sample)
		}

		// Advance position
		r.position += ratio
	}

	// Update position for next call (subtract processed frames)
	r.position -= float64(inputFrames)

	// Store last samples for next interpolation
	if len(input) >= r.channels {
		copy(r.lastSamples, input[len(input)-r.channels:])
	}

	return output, nil
}

// GetInputRate returns the configured input sample rate.
func (r *Resampler) GetInputRate() uint32 {
	return r.inputRate
}

// GetOutputRate returns the configured output sample rate.
func (r *Resampler) GetOutputRate() uint32 {
	return r.outputRate
}

// GetChannels returns the configured number of channels.
func (r *Resampler) GetChannels() int {
	return r.channels
}

// GetQuality returns the configured resampling quality.
func (r *Resampler) GetQuality() int {
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
	if r.inputRate == r.outputRate {
		return inputSize
	}

	// Calculate the ratio and apply it
	ratio := float64(r.outputRate) / float64(r.inputRate)
	return int(float64(inputSize)*ratio + 0.5) // Round to nearest integer
}

// Reset resets the resampler's internal state.
//
// This is useful when starting a new audio stream or when there's
// a discontinuity in the audio data.
func (r *Resampler) Reset() error {
	r.position = 0.0
	// Clear last samples
	for i := range r.lastSamples {
		r.lastSamples[i] = 0
	}
	return nil
}

// Close releases resampler resources.
//
// After calling Close, the resampler should not be used.
func (r *Resampler) Close() error {
	// No resources to clean up for our simple implementation
	return nil
}

// Common resampling configurations for ToxAV

// NewTelephoneToOpusResampler creates a resampler for telephone quality (8kHz) to Opus (48kHz).
func NewTelephoneToOpusResampler(channels int) (*Resampler, error) {
	return NewResampler(ResamplerConfig{
		InputRate:  8000,
		OutputRate: 48000,
		Channels:   channels,
		Quality:    4,
	})
}

// NewCDToOpusResampler creates a resampler for CD quality (44.1kHz) to Opus (48kHz).
func NewCDToOpusResampler(channels int) (*Resampler, error) {
	return NewResampler(ResamplerConfig{
		InputRate:  44100,
		OutputRate: 48000,
		Channels:   channels,
		Quality:    6, // Higher quality for CD audio
	})
}

// NewWidebandToOpusResampler creates a resampler for wideband audio (16kHz) to Opus (48kHz).
func NewWidebandToOpusResampler(channels int) (*Resampler, error) {
	return NewResampler(ResamplerConfig{
		InputRate:  16000,
		OutputRate: 48000,
		Channels:   channels,
		Quality:    4,
	})
}

// NewOpusToPlaybackResampler creates a resampler for Opus (48kHz) to common playback rates.
func NewOpusToPlaybackResampler(outputRate uint32, channels int) (*Resampler, error) {
	return NewResampler(ResamplerConfig{
		InputRate:  48000,
		OutputRate: outputRate,
		Channels:   channels,
		Quality:    4,
	})
}
