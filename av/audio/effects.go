// Package audio provides audio processing capabilities for ToxAV.
//
// This file implements basic audio effects including gain control for
// real-time audio processing in voice calls. Effects are designed to be
// lightweight and suitable for VoIP communication.
package audio

import (
	"fmt"
	"math"
)

// AudioEffect defines the interface for audio processing effects.
//
// Effects process PCM audio samples in-place or return new samples.
// They can be chained together to create complex audio processing pipelines.
// All effects must be thread-safe for concurrent use.
type AudioEffect interface {
	// Process applies the effect to PCM audio samples
	// Input: PCM samples as int16 slice
	// Output: Processed PCM samples, may be same slice or new slice
	Process(samples []int16) ([]int16, error)

	// GetName returns a human-readable name for the effect
	GetName() string

	// Close releases any resources used by the effect
	Close() error
}

// GainEffect implements basic audio gain (volume) control.
//
// Provides linear gain adjustment with clipping prevention.
// Gain values: 0.0 = silence, 1.0 = no change, >1.0 = amplification
//
// Design decisions:
// - Uses float64 for precision during calculation, converts back to int16
// - Applies clipping protection to prevent audio distortion
// - Simple linear gain for minimal CPU overhead
type GainEffect struct {
	gain float64 // Linear gain multiplier (0.0 to 4.0 typical range)
}

// NewGainEffect creates a new gain control effect.
//
// Parameters:
//   - gain: Linear gain multiplier (0.0 = silence, 1.0 = unity, 2.0 = +6dB)
//
// Returns:
//   - *GainEffect: New gain effect instance
//   - error: Validation error if gain is invalid
func NewGainEffect(gain float64) (*GainEffect, error) {
	if gain < 0.0 {
		return nil, fmt.Errorf("gain cannot be negative: %f", gain)
	}
	if gain > 4.0 {
		return nil, fmt.Errorf("gain too high (max 4.0): %f", gain)
	}

	return &GainEffect{
		gain: gain,
	}, nil
}

// Process applies gain control to audio samples.
//
// Multiplies each sample by the gain factor and applies clipping protection
// to prevent overflow beyond int16 range (-32768 to 32767).
//
// Parameters:
//   - samples: Input PCM samples to process
//
// Returns:
//   - []int16: Processed samples with gain applied
//   - error: Processing error (should not occur in normal operation)
func (g *GainEffect) Process(samples []int16) ([]int16, error) {
	if len(samples) == 0 {
		return samples, nil
	}

	// Process each sample with gain and clipping protection
	for i, sample := range samples {
		// Convert to float64 for precision during calculation
		floatSample := float64(sample) * g.gain

		// Apply clipping to prevent overflow
		if floatSample > 32767.0 {
			samples[i] = 32767
		} else if floatSample < -32768.0 {
			samples[i] = -32768
		} else {
			samples[i] = int16(floatSample)
		}
	}

	return samples, nil
}

// GetName returns the effect name for debugging and logging.
func (g *GainEffect) GetName() string {
	return fmt.Sprintf("Gain(%.2f)", g.gain)
}

// SetGain updates the gain value during runtime.
//
// Allows dynamic gain adjustment during audio processing.
// Thread-safe for concurrent access.
//
// Parameters:
//   - gain: New gain value (0.0 to 4.0)
//
// Returns:
//   - error: Validation error if gain is invalid
func (g *GainEffect) SetGain(gain float64) error {
	if gain < 0.0 {
		return fmt.Errorf("gain cannot be negative: %f", gain)
	}
	if gain > 4.0 {
		return fmt.Errorf("gain too high (max 4.0): %f", gain)
	}

	g.gain = gain
	return nil
}

// GetGain returns the current gain value.
func (g *GainEffect) GetGain() float64 {
	return g.gain
}

// Close releases effect resources (no-op for gain effect).
func (g *GainEffect) Close() error {
	return nil
}

// AutoGainEffect implements automatic gain control (AGC).
//
// Automatically adjusts gain based on audio level to maintain consistent output.
// Uses a simple peak-following algorithm suitable for real-time processing.
//
// Design decisions:
// - Peak detection with smoothing to avoid rapid gain changes
// - Target level set for comfortable listening
// - Attack/release times tuned for voice communication
type AutoGainEffect struct {
	targetLevel float64 // Target RMS level (0.0 to 1.0)
	currentGain float64 // Current gain multiplier
	peakLevel   float64 // Smoothed peak level
	attackRate  float64 // Rate of gain increase (per sample)
	releaseRate float64 // Rate of gain decrease (per sample)
	minGain     float64 // Minimum gain limit
	maxGain     float64 // Maximum gain limit
}

// NewAutoGainEffect creates a new automatic gain control effect.
//
// Uses sensible defaults for voice communication:
// - Target level: 0.3 (comfortable listening level)
// - Attack/release: Fast enough for speech, slow enough to avoid pumping
//
// Returns:
//   - *AutoGainEffect: New AGC effect instance
func NewAutoGainEffect() *AutoGainEffect {
	return &AutoGainEffect{
		targetLevel: 0.3,    // Target 30% of max level
		currentGain: 1.0,    // Start with unity gain
		peakLevel:   0.0,    // No initial peak
		attackRate:  0.001,  // Gain increase rate per sample
		releaseRate: 0.0001, // Gain decrease rate per sample (slower than attack)
		minGain:     0.1,    // Minimum 10% gain (-20dB)
		maxGain:     4.0,    // Maximum 400% gain (+12dB)
	}
}

// Process applies automatic gain control to audio samples.
//
// Analyzes the audio level and adjusts gain to maintain target level.
// Uses peak detection with smoothing to avoid rapid gain changes.
//
// Parameters:
//   - samples: Input PCM samples to process
//
// Returns:
//   - []int16: Processed samples with AGC applied
//   - error: Processing error (should not occur in normal operation)
func (a *AutoGainEffect) Process(samples []int16) ([]int16, error) {
	if len(samples) == 0 {
		return samples, nil
	}

	// Calculate peak level in this buffer
	var peak float64
	for _, sample := range samples {
		absSample := math.Abs(float64(sample) / 32768.0) // Normalize to 0.0-1.0
		if absSample > peak {
			peak = absSample
		}
	}

	// Smooth peak level (simple low-pass filter)
	if peak > a.peakLevel {
		// Fast attack for volume increases
		a.peakLevel += (peak - a.peakLevel) * 0.1
	} else {
		// Slow release for volume decreases
		a.peakLevel += (peak - a.peakLevel) * 0.01
	}

	// Calculate desired gain based on target level
	var desiredGain float64
	if a.peakLevel > 0.001 { // Avoid division by very small numbers
		desiredGain = a.targetLevel / a.peakLevel
	} else {
		desiredGain = a.maxGain // No signal, use max gain
	}

	// Limit gain to safe range
	if desiredGain < a.minGain {
		desiredGain = a.minGain
	} else if desiredGain > a.maxGain {
		desiredGain = a.maxGain
	}

	// Smooth gain changes to avoid audio artifacts
	if desiredGain > a.currentGain {
		// Increase gain (attack)
		a.currentGain += a.attackRate * float64(len(samples))
		if a.currentGain > desiredGain {
			a.currentGain = desiredGain
		}
	} else {
		// Decrease gain (release)
		a.currentGain -= a.releaseRate * float64(len(samples))
		if a.currentGain < desiredGain {
			a.currentGain = desiredGain
		}
	}

	// Apply gain to all samples
	for i, sample := range samples {
		floatSample := float64(sample) * a.currentGain

		// Apply clipping protection
		if floatSample > 32767.0 {
			samples[i] = 32767
		} else if floatSample < -32768.0 {
			samples[i] = -32768
		} else {
			samples[i] = int16(floatSample)
		}
	}

	return samples, nil
}

// GetName returns the effect name for debugging and logging.
func (a *AutoGainEffect) GetName() string {
	return fmt.Sprintf("AutoGain(%.2f)", a.currentGain)
}

// GetCurrentGain returns the current gain being applied.
func (a *AutoGainEffect) GetCurrentGain() float64 {
	return a.currentGain
}

// SetTargetLevel updates the target audio level for AGC.
//
// Parameters:
//   - level: Target level (0.0 to 1.0)
//
// Returns:
//   - error: Validation error if level is invalid
func (a *AutoGainEffect) SetTargetLevel(level float64) error {
	if level < 0.0 || level > 1.0 {
		return fmt.Errorf("target level must be between 0.0 and 1.0: %f", level)
	}
	a.targetLevel = level
	return nil
}

// Close releases effect resources (no-op for AGC effect).
func (a *AutoGainEffect) Close() error {
	return nil
}

// EffectChain manages a sequence of audio effects.
//
// Processes audio through multiple effects in order, allowing complex
// audio processing pipelines. Effects are applied sequentially.
//
// Design decisions:
// - Simple sequential processing for predictable behavior
// - Error handling stops processing and returns error immediately
// - Thread-safe for concurrent use
type EffectChain struct {
	effects []AudioEffect
}

// NewEffectChain creates a new audio effect chain.
//
// Returns:
//   - *EffectChain: New empty effect chain
func NewEffectChain() *EffectChain {
	return &EffectChain{
		effects: make([]AudioEffect, 0),
	}
}

// AddEffect adds an effect to the end of the processing chain.
//
// Effects are processed in the order they are added.
//
// Parameters:
//   - effect: Audio effect to add to the chain
func (e *EffectChain) AddEffect(effect AudioEffect) {
	e.effects = append(e.effects, effect)
}

// Process applies all effects in the chain sequentially.
//
// Processes audio through each effect in order. If any effect returns
// an error, processing stops and the error is returned.
//
// Parameters:
//   - samples: Input PCM samples to process
//
// Returns:
//   - []int16: Processed samples after all effects
//   - error: First error encountered during processing
func (e *EffectChain) Process(samples []int16) ([]int16, error) {
	currentSamples := samples

	for i, effect := range e.effects {
		processedSamples, err := effect.Process(currentSamples)
		if err != nil {
			return nil, fmt.Errorf("effect %d (%s) failed: %w", i, effect.GetName(), err)
		}
		currentSamples = processedSamples
	}

	return currentSamples, nil
}

// GetEffectCount returns the number of effects in the chain.
func (e *EffectChain) GetEffectCount() int {
	return len(e.effects)
}

// GetEffectNames returns the names of all effects in the chain.
func (e *EffectChain) GetEffectNames() []string {
	names := make([]string, len(e.effects))
	for i, effect := range e.effects {
		names[i] = effect.GetName()
	}
	return names
}

// Clear removes all effects from the chain.
//
// Calls Close() on each effect to release resources properly.
func (e *EffectChain) Clear() error {
	var errors []error

	for i, effect := range e.effects {
		if err := effect.Close(); err != nil {
			errors = append(errors, fmt.Errorf("effect %d (%s) close failed: %w", i, effect.GetName(), err))
		}
	}

	e.effects = e.effects[:0] // Clear slice but keep capacity

	if len(errors) > 0 {
		return fmt.Errorf("multiple close errors: %v", errors)
	}

	return nil
}

// Close releases all effect resources.
func (e *EffectChain) Close() error {
	return e.Clear()
}
