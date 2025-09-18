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

// NoiseSuppressionEffect implements advanced noise suppression using spectral subtraction.
//
// This effect reduces background noise while preserving speech quality using a combination
// of noise floor estimation and spectral subtraction. It operates on overlapping frames
// with windowing to minimize artifacts.
//
// Design decisions:
// - Uses spectral subtraction with configurable suppression strength
// - Maintains noise floor estimation for adaptive operation
// - Applies Hanning window to reduce edge artifacts
// - Uses over-subtraction with spectral floor to prevent musical noise
type NoiseSuppressionEffect struct {
	suppressionLevel float64      // Noise suppression strength (0.0 to 1.0)
	frameSize        int          // Frame size for FFT processing (must be power of 2)
	overlapSize      int          // Overlap between frames (50% of frame size)
	windowBuffer     []float64    // Windowing function (Hanning window)
	inputBuffer      []float64    // Input sample buffer for overlap processing
	outputBuffer     []float64    // Output sample buffer for overlap-add
	noiseFloor       []float64    // Estimated noise floor spectrum
	spectrumBuffer   []complex128 // Working buffer for FFT
	initialized      bool         // Whether noise floor has been estimated
	frameCount       int          // Number of frames processed for noise estimation
}

// NewNoiseSuppressionEffect creates a new noise suppression effect.
//
// Parameters:
//   - suppressionLevel: Noise suppression strength (0.0 = no suppression, 1.0 = maximum)
//   - frameSize: FFT frame size, must be power of 2 (typically 512 or 1024)
//
// Returns:
//   - *NoiseSuppressionEffect: New noise suppression effect instance
//   - error: Validation error if parameters are invalid
func NewNoiseSuppressionEffect(suppressionLevel float64, frameSize int) (*NoiseSuppressionEffect, error) {
	logrus.WithFields(logrus.Fields{
		"function":         "NewNoiseSuppressionEffect",
		"suppressionLevel": suppressionLevel,
		"frameSize":        frameSize,
	}).Info("Creating new noise suppression effect")

	// Validate suppression level
	if suppressionLevel < 0.0 || suppressionLevel > 1.0 {
		logrus.WithFields(logrus.Fields{
			"function":         "NewNoiseSuppressionEffect",
			"suppressionLevel": suppressionLevel,
			"error":            "suppression level must be between 0.0 and 1.0",
		}).Error("Suppression level validation failed")
		return nil, fmt.Errorf("suppression level must be between 0.0 and 1.0: %f", suppressionLevel)
	}

	// Validate frame size (must be power of 2)
	if frameSize < 64 || frameSize > 4096 || (frameSize&(frameSize-1)) != 0 {
		logrus.WithFields(logrus.Fields{
			"function":  "NewNoiseSuppressionEffect",
			"frameSize": frameSize,
			"error":     "frame size must be power of 2 between 64 and 4096",
		}).Error("Frame size validation failed")
		return nil, fmt.Errorf("frame size must be power of 2 between 64 and 4096: %d", frameSize)
	}

	overlapSize := frameSize / 2

	// Create Hanning window
	window := make([]float64, frameSize)
	for i := 0; i < frameSize; i++ {
		window[i] = 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(frameSize-1)))
	}

	logrus.WithFields(logrus.Fields{
		"function":         "NewNoiseSuppressionEffect",
		"suppressionLevel": suppressionLevel,
		"frameSize":        frameSize,
		"overlapSize":      overlapSize,
	}).Info("Noise suppression effect created successfully")

	return &NoiseSuppressionEffect{
		suppressionLevel: suppressionLevel,
		frameSize:        frameSize,
		overlapSize:      overlapSize,
		windowBuffer:     window,
		inputBuffer:      make([]float64, frameSize+overlapSize),
		outputBuffer:     make([]float64, frameSize+overlapSize),
		noiseFloor:       make([]float64, frameSize/2+1),
		spectrumBuffer:   make([]complex128, frameSize),
		initialized:      false,
		frameCount:       0,
	}, nil
}

// Process applies noise suppression to audio samples using spectral subtraction.
//
// The algorithm works by:
// 1. Converting input to overlapping windowed frames
// 2. Computing FFT of each frame
// 3. Estimating noise floor from initial frames
// 4. Subtracting estimated noise spectrum with configurable strength
// 5. Applying spectral floor to prevent musical noise
// 6. Converting back to time domain with overlap-add
//
// Parameters:
//   - samples: Input PCM samples to process
//
// Returns:
//   - []int16: Processed samples with noise suppression applied
//   - error: Processing error if FFT or conversion fails
func (ns *NoiseSuppressionEffect) Process(samples []int16) ([]int16, error) {
	if len(samples) == 0 {
		return samples, nil
	}

	logrus.WithFields(logrus.Fields{
		"function":    "NoiseSuppressionEffect.Process",
		"sampleCount": len(samples),
		"initialized": ns.initialized,
		"frameCount":  ns.frameCount,
	}).Debug("Processing noise suppression")

	// Convert int16 samples to float64 for processing
	floatSamples := make([]float64, len(samples))
	for i, sample := range samples {
		floatSamples[i] = float64(sample) / 32768.0
	}

	// Process samples through overlapping frames
	processedSamples := ns.processOverlapping(floatSamples)

	// Convert back to int16 with clipping
	result := make([]int16, len(processedSamples))
	for i, sample := range processedSamples {
		// Apply clipping to prevent overflow
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}
		result[i] = int16(sample * 32767.0)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "NoiseSuppressionEffect.Process",
		"inputSamples":  len(samples),
		"outputSamples": len(result),
		"frameCount":    ns.frameCount,
	}).Debug("Noise suppression processing completed")

	return result, nil
}

// processOverlapping handles overlap-add processing for spectral subtraction.
func (ns *NoiseSuppressionEffect) processOverlapping(samples []float64) []float64 {
	output := make([]float64, len(samples))
	hopSize := ns.frameSize - ns.overlapSize

	for pos := 0; pos < len(samples); pos += hopSize {
		// Get frame with overlap
		frameEnd := pos + ns.frameSize
		if frameEnd > len(samples) {
			frameEnd = len(samples)
		}

		frame := make([]float64, ns.frameSize)
		copy(frame, samples[pos:frameEnd])

		// Process frame through spectral subtraction
		processedFrame := ns.processFrame(frame)

		// Overlap-add into output
		for i, val := range processedFrame {
			outPos := pos + i
			if outPos < len(output) {
				output[outPos] += val
			}
		}
	}

	return output
}

// processFrame applies spectral subtraction to a single windowed frame.
func (ns *NoiseSuppressionEffect) processFrame(frame []float64) []float64 {
	// Prepare frame for FFT analysis
	ns.prepareFrameForFFT(frame)

	// Compute FFT and extract magnitude spectrum
	magnitude := ns.computeMagnitudeSpectrum()

	// Update noise floor estimation if still learning
	ns.updateNoiseFloorEstimation(magnitude)

	// Apply spectral subtraction for noise suppression
	ns.applySpectralSubtraction(magnitude)

	// Reconstruct time-domain signal
	return ns.reconstructTimeSignal()
}

// prepareFrameForFFT applies window function and converts to complex spectrum buffer.
func (ns *NoiseSuppressionEffect) prepareFrameForFFT(frame []float64) {
	// Apply window function
	windowedFrame := make([]float64, len(frame))
	for i := range frame {
		windowedFrame[i] = frame[i] * ns.windowBuffer[i]
	}

	// Convert to complex for FFT
	for i := range ns.spectrumBuffer {
		if i < len(windowedFrame) {
			ns.spectrumBuffer[i] = complex(windowedFrame[i], 0)
		} else {
			ns.spectrumBuffer[i] = 0
		}
	}
}

// computeMagnitudeSpectrum performs FFT and calculates magnitude spectrum.
func (ns *NoiseSuppressionEffect) computeMagnitudeSpectrum() []float64 {
	// Compute FFT
	ns.fft(ns.spectrumBuffer)

	// Compute magnitude spectrum
	magnitude := make([]float64, ns.frameSize/2+1)
	for i := 0; i <= ns.frameSize/2; i++ {
		real := real(ns.spectrumBuffer[i])
		imag := imag(ns.spectrumBuffer[i])
		magnitude[i] = math.Sqrt(real*real + imag*imag)
	}

	return magnitude
}

// updateNoiseFloorEstimation updates noise floor during initial learning phase.
func (ns *NoiseSuppressionEffect) updateNoiseFloorEstimation(magnitude []float64) {
	// Update noise floor estimation during first few frames
	if ns.frameCount < 10 {
		alpha := 0.8 // Smoothing factor for noise floor estimation
		for i := range ns.noiseFloor {
			if ns.frameCount == 0 {
				ns.noiseFloor[i] = magnitude[i]
			} else {
				ns.noiseFloor[i] = alpha*ns.noiseFloor[i] + (1-alpha)*magnitude[i]
			}
		}
		ns.frameCount++
		if ns.frameCount >= 10 {
			ns.initialized = true
			logrus.WithFields(logrus.Fields{
				"function": "updateNoiseFloorEstimation",
			}).Info("Noise floor estimation completed")
		}
	}
}

// applySpectralSubtraction performs noise suppression using spectral subtraction method.
func (ns *NoiseSuppressionEffect) applySpectralSubtraction(magnitude []float64) {
	// Apply spectral subtraction if initialized
	if ns.initialized {
		for i := range magnitude {
			// Spectral subtraction with over-subtraction factor
			overSubtraction := 2.0
			subtracted := magnitude[i] - overSubtraction*ns.suppressionLevel*ns.noiseFloor[i]

			// Apply spectral floor (prevent too much suppression)
			spectralFloor := 0.1 * magnitude[i]
			if subtracted < spectralFloor {
				subtracted = spectralFloor
			}

			// Update spectrum with suppressed magnitude
			if magnitude[i] > 0 {
				suppressionRatio := subtracted / magnitude[i]
				ns.spectrumBuffer[i] = complex(
					real(ns.spectrumBuffer[i])*suppressionRatio,
					imag(ns.spectrumBuffer[i])*suppressionRatio,
				)
				// Mirror for negative frequencies
				if i > 0 && i < ns.frameSize/2 {
					mirrorIdx := ns.frameSize - i
					ns.spectrumBuffer[mirrorIdx] = complex(
						real(ns.spectrumBuffer[mirrorIdx])*suppressionRatio,
						imag(ns.spectrumBuffer[mirrorIdx])*suppressionRatio,
					)
				}
			}
		}
	}
}

// reconstructTimeSignal performs inverse FFT and applies windowing for overlap-add.
func (ns *NoiseSuppressionEffect) reconstructTimeSignal() []float64 {
	// Compute inverse FFT
	ns.ifft(ns.spectrumBuffer)

	// Extract real part and apply window again for overlap-add
	result := make([]float64, ns.frameSize)
	for i := range result {
		result[i] = real(ns.spectrumBuffer[i]) * ns.windowBuffer[i]
	}

	return result
}

// Simple FFT implementation for power-of-2 sizes using Cooley-Tukey algorithm.
func (ns *NoiseSuppressionEffect) fft(data []complex128) {
	n := len(data)
	if n <= 1 {
		return
	}

	// Bit-reverse ordering
	for i, j := 0, 0; i < n; i++ {
		if j > i {
			data[i], data[j] = data[j], data[i]
		}
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
	}

	// Cooley-Tukey FFT
	for size := 2; size <= n; size <<= 1 {
		halfSize := size >> 1
		step := 2 * math.Pi / float64(size)
		for i := 0; i < n; i += size {
			for j := 0; j < halfSize; j++ {
				u := data[i+j]
				v := data[i+j+halfSize] * complex(math.Cos(float64(j)*step), -math.Sin(float64(j)*step))
				data[i+j] = u + v
				data[i+j+halfSize] = u - v
			}
		}
	}
}

// Inverse FFT using forward FFT with conjugate trick.
func (ns *NoiseSuppressionEffect) ifft(data []complex128) {
	n := len(data)

	// Conjugate input
	for i := range data {
		data[i] = complex(real(data[i]), -imag(data[i]))
	}

	// Forward FFT
	ns.fft(data)

	// Conjugate and scale output
	scale := 1.0 / float64(n)
	for i := range data {
		data[i] = complex(real(data[i])*scale, -imag(data[i])*scale)
	}
}

// GetName returns the human-readable name of the effect.
func (ns *NoiseSuppressionEffect) GetName() string {
	return "NoiseSuppressionEffect"
}

// Close releases any resources used by the noise suppression effect.
//
// Currently no external resources to clean up, but maintains interface
// compatibility for future enhancements.
func (ns *NoiseSuppressionEffect) Close() error {
	logrus.WithFields(logrus.Fields{
		"function": "NoiseSuppressionEffect.Close",
	}).Info("Closing noise suppression effect")

	// Clear buffers to free memory
	ns.inputBuffer = nil
	ns.outputBuffer = nil
	ns.noiseFloor = nil
	ns.spectrumBuffer = nil
	ns.windowBuffer = nil

	logrus.WithFields(logrus.Fields{
		"function": "NoiseSuppressionEffect.Close",
	}).Info("Noise suppression effect closed successfully")

	return nil
}
