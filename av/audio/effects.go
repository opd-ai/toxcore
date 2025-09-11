// Package audio provides audio processing capabilities for ToxAV.
//
// This file implements basic audio effects including gain control for
// real-time audio processing in voice calls. Effects are designed to be
// lightweight and suitable for VoIP communication.
package audio

import (
	"fmt"
	"math"

	"github.com/sirupsen/logrus"
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
	logrus.WithFields(logrus.Fields{
		"function": "NewGainEffect",
		"gain":     gain,
	}).Info("Creating new gain effect")

	if gain < 0.0 {
		logrus.WithFields(logrus.Fields{
			"function": "NewGainEffect",
			"gain":     gain,
			"error":    "gain cannot be negative",
		}).Error("Gain validation failed")
		return nil, fmt.Errorf("gain cannot be negative: %f", gain)
	}
	if gain > 4.0 {
		logrus.WithFields(logrus.Fields{
			"function": "NewGainEffect",
			"gain":     gain,
			"error":    "gain too high (max 4.0)",
		}).Error("Gain validation failed")
		return nil, fmt.Errorf("gain too high (max 4.0): %f", gain)
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewGainEffect",
		"gain":     gain,
	}).Info("Gain effect created successfully")

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
	logrus.WithFields(logrus.Fields{
		"function":     "GainEffect.Process",
		"sample_count": len(samples),
		"gain":         g.gain,
	}).Debug("Processing audio samples with gain control")

	if len(samples) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "GainEffect.Process",
		}).Debug("Empty sample buffer, no processing needed")
		return samples, nil
	}

	clippedCount := 0

	// Process each sample with gain and clipping protection
	for i, sample := range samples {
		// Convert to float64 for precision during calculation
		floatSample := float64(sample) * g.gain

		// Apply clipping to prevent overflow
		if floatSample > 32767.0 {
			samples[i] = 32767
			clippedCount++
		} else if floatSample < -32768.0 {
			samples[i] = -32768
			clippedCount++
		} else {
			samples[i] = int16(floatSample)
		}
	}

	logrus.WithFields(logrus.Fields{
		"function":      "GainEffect.Process",
		"sample_count":  len(samples),
		"gain":          g.gain,
		"clipped_count": clippedCount,
	}).Debug("Gain processing completed")

	if clippedCount > 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "GainEffect.Process",
			"clipped_count": clippedCount,
			"total_samples": len(samples),
			"gain":          g.gain,
		}).Warn("Audio clipping detected during gain processing")
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
	logrus.WithFields(logrus.Fields{
		"function": "GainEffect.SetGain",
		"old_gain": g.gain,
		"new_gain": gain,
	}).Info("Updating gain effect value")

	if gain < 0.0 {
		logrus.WithFields(logrus.Fields{
			"function": "GainEffect.SetGain",
			"gain":     gain,
			"error":    "gain cannot be negative",
		}).Error("Gain validation failed")
		return fmt.Errorf("gain cannot be negative: %f", gain)
	}
	if gain > 4.0 {
		logrus.WithFields(logrus.Fields{
			"function": "GainEffect.SetGain",
			"gain":     gain,
			"error":    "gain too high (max 4.0)",
		}).Error("Gain validation failed")
		return fmt.Errorf("gain too high (max 4.0): %f", gain)
	}

	g.gain = gain

	logrus.WithFields(logrus.Fields{
		"function": "GainEffect.SetGain",
		"gain":     gain,
	}).Info("Gain effect value updated successfully")

	return nil
}

// GetGain returns the current gain value.
func (g *GainEffect) GetGain() float64 {
	logrus.WithFields(logrus.Fields{
		"function": "GainEffect.GetGain",
		"gain":     g.gain,
	}).Debug("Retrieving current gain value")
	return g.gain
}

// Close releases effect resources (no-op for gain effect).
func (g *GainEffect) Close() error {
	logrus.WithFields(logrus.Fields{
		"function": "GainEffect.Close",
		"gain":     g.gain,
	}).Debug("Closing gain effect (no resources to release)")
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
	agc := &AutoGainEffect{
		targetLevel: 0.3,    // Target 30% of max level
		currentGain: 1.0,    // Start with unity gain
		peakLevel:   0.0,    // No initial peak
		attackRate:  0.001,  // Gain increase rate per sample
		releaseRate: 0.0001, // Gain decrease rate per sample (slower than attack)
		minGain:     0.1,    // Minimum 10% gain (-20dB)
		maxGain:     4.0,    // Maximum 400% gain (+12dB)
	}

	logrus.WithFields(logrus.Fields{
		"function":     "NewAutoGainEffect",
		"target_level": agc.targetLevel,
		"min_gain":     agc.minGain,
		"max_gain":     agc.maxGain,
		"attack_rate":  agc.attackRate,
		"release_rate": agc.releaseRate,
	}).Info("Auto gain control effect created with default settings")

	return agc
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
	logrus.WithFields(logrus.Fields{
		"function":     "AutoGainEffect.Process",
		"sample_count": len(samples),
		"current_gain": a.currentGain,
		"peak_level":   a.peakLevel,
		"target_level": a.targetLevel,
	}).Debug("Processing audio samples with automatic gain control")

	if len(samples) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "AutoGainEffect.Process",
		}).Debug("Empty sample buffer, no processing needed")
		return samples, nil
	}

	// Calculate and smooth peak level
	peak := a.calculatePeakLevel(samples)
	oldPeakLevel := a.peakLevel
	a.smoothPeakLevel(peak)

	// Calculate and limit desired gain
	desiredGain := a.calculateDesiredGain()
	desiredGain = a.limitGainToSafeRange(desiredGain)

	// Smooth gain changes and apply to samples
	oldGain := a.currentGain
	a.smoothGainChanges(desiredGain, len(samples))
	a.applyGainToSamples(samples)

	logrus.WithFields(logrus.Fields{
		"function":       "AutoGainEffect.Process",
		"sample_count":   len(samples),
		"peak_measured":  peak,
		"old_peak_level": oldPeakLevel,
		"new_peak_level": a.peakLevel,
		"old_gain":       oldGain,
		"new_gain":       a.currentGain,
		"desired_gain":   desiredGain,
	}).Debug("Auto gain control processing completed")

	return samples, nil
}

// calculatePeakLevel computes the peak audio level in the current buffer.
// Normalizes samples to 0.0-1.0 range and finds maximum absolute value.
func (a *AutoGainEffect) calculatePeakLevel(samples []int16) float64 {
	var peak float64
	for _, sample := range samples {
		absSample := math.Abs(float64(sample) / 32768.0) // Normalize to 0.0-1.0
		if absSample > peak {
			peak = absSample
		}
	}
	return peak
}

// smoothPeakLevel applies low-pass filtering to the peak level measurement.
// Uses fast attack for volume increases and slow release for decreases.
func (a *AutoGainEffect) smoothPeakLevel(peak float64) {
	if peak > a.peakLevel {
		// Fast attack for volume increases
		a.peakLevel += (peak - a.peakLevel) * 0.1
	} else {
		// Slow release for volume decreases
		a.peakLevel += (peak - a.peakLevel) * 0.01
	}
}

// calculateDesiredGain determines the target gain based on current peak level.
// Returns maximum gain if signal is too quiet to avoid division by zero.
func (a *AutoGainEffect) calculateDesiredGain() float64 {
	if a.peakLevel > 0.001 { // Avoid division by very small numbers
		return a.targetLevel / a.peakLevel
	}
	return a.maxGain // No signal, use max gain
}

// limitGainToSafeRange constrains the desired gain within configured limits.
// Prevents excessive amplification or attenuation that could damage audio quality.
func (a *AutoGainEffect) limitGainToSafeRange(desiredGain float64) float64 {
	if desiredGain < a.minGain {
		return a.minGain
	} else if desiredGain > a.maxGain {
		return a.maxGain
	}
	return desiredGain
}

// smoothGainChanges gradually adjusts current gain toward desired gain.
// Uses different rates for attack (gain increase) and release (gain decrease)
// to avoid audio artifacts and pumping effects.
func (a *AutoGainEffect) smoothGainChanges(desiredGain float64, sampleCount int) {
	if desiredGain > a.currentGain {
		// Increase gain (attack)
		a.currentGain += a.attackRate * float64(sampleCount)
		if a.currentGain > desiredGain {
			a.currentGain = desiredGain
		}
	} else {
		// Decrease gain (release)
		a.currentGain -= a.releaseRate * float64(sampleCount)
		if a.currentGain < desiredGain {
			a.currentGain = desiredGain
		}
	}
}

// applyGainToSamples multiplies all samples by current gain with clipping protection.
// Prevents integer overflow by clamping values to valid int16 range.
func (a *AutoGainEffect) applyGainToSamples(samples []int16) {
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
}

// GetName returns the effect name for debugging and logging.
func (a *AutoGainEffect) GetName() string {
	return fmt.Sprintf("AutoGain(%.2f)", a.currentGain)
}

// GetCurrentGain returns the current gain being applied.
func (a *AutoGainEffect) GetCurrentGain() float64 {
	logrus.WithFields(logrus.Fields{
		"function":     "AutoGainEffect.GetCurrentGain",
		"current_gain": a.currentGain,
		"peak_level":   a.peakLevel,
	}).Debug("Retrieving current auto gain value")
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
	logrus.WithFields(logrus.Fields{
		"function":   "AutoGainEffect.SetTargetLevel",
		"old_target": a.targetLevel,
		"new_target": level,
	}).Info("Updating auto gain target level")

	if level < 0.0 || level > 1.0 {
		logrus.WithFields(logrus.Fields{
			"function": "AutoGainEffect.SetTargetLevel",
			"level":    level,
			"error":    "target level must be between 0.0 and 1.0",
		}).Error("Target level validation failed")
		return fmt.Errorf("target level must be between 0.0 and 1.0: %f", level)
	}

	a.targetLevel = level

	logrus.WithFields(logrus.Fields{
		"function":   "AutoGainEffect.SetTargetLevel",
		"new_target": level,
	}).Info("Auto gain target level updated successfully")

	return nil
}

// Close releases effect resources (no-op for AGC effect).
func (a *AutoGainEffect) Close() error {
	logrus.WithFields(logrus.Fields{
		"function":     "AutoGainEffect.Close",
		"current_gain": a.currentGain,
		"peak_level":   a.peakLevel,
	}).Debug("Closing auto gain effect (no resources to release)")
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
	logrus.WithFields(logrus.Fields{
		"function": "NewEffectChain",
	}).Info("Creating new audio effect chain")

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
	logrus.WithFields(logrus.Fields{
		"function":     "EffectChain.AddEffect",
		"effect_name":  effect.GetName(),
		"effect_count": len(e.effects),
		"new_position": len(e.effects),
	}).Info("Adding effect to audio chain")

	e.effects = append(e.effects, effect)

	logrus.WithFields(logrus.Fields{
		"function":     "EffectChain.AddEffect",
		"effect_name":  effect.GetName(),
		"effect_count": len(e.effects),
	}).Debug("Effect added to chain successfully")
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
	logrus.WithFields(logrus.Fields{
		"function":     "EffectChain.Process",
		"sample_count": len(samples),
		"effect_count": len(e.effects),
	}).Debug("Processing audio through effect chain")

	if len(e.effects) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "EffectChain.Process",
		}).Debug("No effects in chain, returning samples unchanged")
		return samples, nil
	}

	currentSamples := samples

	for i, effect := range e.effects {
		logrus.WithFields(logrus.Fields{
			"function":     "EffectChain.Process",
			"effect_index": i,
			"effect_name":  effect.GetName(),
			"sample_count": len(currentSamples),
		}).Debug("Processing samples through effect")

		processedSamples, err := effect.Process(currentSamples)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":     "EffectChain.Process",
				"effect_index": i,
				"effect_name":  effect.GetName(),
				"error":        err.Error(),
			}).Error("Effect processing failed")
			return nil, fmt.Errorf("effect %d (%s) failed: %w", i, effect.GetName(), err)
		}
		currentSamples = processedSamples
	}

	logrus.WithFields(logrus.Fields{
		"function":        "EffectChain.Process",
		"input_samples":   len(samples),
		"output_samples":  len(currentSamples),
		"effects_applied": len(e.effects),
	}).Debug("Effect chain processing completed successfully")

	return currentSamples, nil
}

// GetEffectCount returns the number of effects in the chain.
func (e *EffectChain) GetEffectCount() int {
	count := len(e.effects)
	logrus.WithFields(logrus.Fields{
		"function":     "EffectChain.GetEffectCount",
		"effect_count": count,
	}).Debug("Retrieving effect chain count")
	return count
}

// GetEffectNames returns the names of all effects in the chain.
func (e *EffectChain) GetEffectNames() []string {
	names := make([]string, len(e.effects))
	for i, effect := range e.effects {
		names[i] = effect.GetName()
	}

	logrus.WithFields(logrus.Fields{
		"function":     "EffectChain.GetEffectNames",
		"effect_count": len(names),
		"effect_names": names,
	}).Debug("Retrieving effect chain names")

	return names
}

// Clear removes all effects from the chain.
//
// Calls Close() on each effect to release resources properly.
func (e *EffectChain) Clear() error {
	logrus.WithFields(logrus.Fields{
		"function":     "EffectChain.Clear",
		"effect_count": len(e.effects),
	}).Info("Clearing all effects from chain")

	var errors []error

	for i, effect := range e.effects {
		logrus.WithFields(logrus.Fields{
			"function":     "EffectChain.Clear",
			"effect_index": i,
			"effect_name":  effect.GetName(),
		}).Debug("Closing effect")

		if err := effect.Close(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function":     "EffectChain.Clear",
				"effect_index": i,
				"effect_name":  effect.GetName(),
				"error":        err.Error(),
			}).Error("Failed to close effect")
			errors = append(errors, fmt.Errorf("effect %d (%s) close failed: %w", i, effect.GetName(), err))
		}
	}

	e.effects = e.effects[:0] // Clear slice but keep capacity

	if len(errors) > 0 {
		logrus.WithFields(logrus.Fields{
			"function":    "EffectChain.Clear",
			"error_count": len(errors),
		}).Error("Multiple errors occurred during effect chain clear")
		return fmt.Errorf("multiple close errors: %v", errors)
	}

	logrus.WithFields(logrus.Fields{
		"function": "EffectChain.Clear",
	}).Info("Effect chain cleared successfully")

	return nil
}

// Close releases all effect resources.
func (e *EffectChain) Close() error {
	logrus.WithFields(logrus.Fields{
		"function":     "EffectChain.Close",
		"effect_count": len(e.effects),
	}).Info("Closing effect chain")

	err := e.Clear()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "EffectChain.Close",
			"error":    err.Error(),
		}).Error("Failed to close effect chain")
	} else {
		logrus.WithFields(logrus.Fields{
			"function": "EffectChain.Close",
		}).Info("Effect chain closed successfully")
	}

	return err
}
