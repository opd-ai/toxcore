// Package main demonstrates audio effects usage in ToxAV including gain control,
// automatic gain control (AGC), effect chaining, and processor integration.
//
// This example shows how to use the av/audio package's effect system to:
//   - Apply basic gain (volume) control to audio samples
//   - Use automatic gain control (AGC) for consistent audio levels
//   - Chain multiple effects for sequential processing
//   - Integrate effects with the audio processor pipeline
//
// To run: go run main.go
package main

import (
	"fmt"
	"math"

	"github.com/opd-ai/toxcore/av/audio"
	"github.com/sirupsen/logrus"
)

// logger is the package-level structured logger instance.
var logger = logrus.New()

func main() {
	fmt.Println("ToxAV Audio Effects Demo")
	fmt.Println("========================")

	// Demonstrate basic gain control
	demonstrateGainControl()

	// Demonstrate automatic gain control
	demonstrateAutoGain()

	// Demonstrate effect chaining
	demonstrateEffectChain()

	// Demonstrate processor integration
	demonstrateProcessorIntegration()
}

// demonstrateGainControl shows basic gain (volume) control with a GainEffect.
func demonstrateGainControl() {
	fmt.Println("\n1. Basic Gain Control")
	fmt.Println("---------------------")

	// Create gain effect with 50% volume
	gainEffect, err := audio.NewGainEffect(0.5)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create gain effect")
	}
	defer gainEffect.Close()

	// Generate test audio (1kHz sine wave)
	testAudio := generateTestAudio(480, 10000) // 10ms at 48kHz, moderate level

	fmt.Printf("Original audio level: max = %d\n", maxAmplitude(testAudio))

	// Apply gain
	processedAudio, err := gainEffect.Process(testAudio)
	if err != nil {
		logger.WithError(err).Error("Gain processing failed")
		return
	}

	fmt.Printf("After 0.5x gain: max = %d\n", maxAmplitude(processedAudio))

	// Change gain dynamically
	err = gainEffect.SetGain(2.0)
	if err != nil {
		logger.WithError(err).Error("Failed to set gain")
		return
	}

	processedAudio, err = gainEffect.Process(testAudio)
	if err != nil {
		logger.WithError(err).Error("Gain processing failed")
		return
	}

	fmt.Printf("After 2.0x gain: max = %d\n", maxAmplitude(processedAudio))
	fmt.Printf("Effect name: %s\n", gainEffect.GetName())
}

// demonstrateAutoGain shows automatic gain control (AGC) for normalizing audio levels.
func demonstrateAutoGain() {
	fmt.Println("\n2. Automatic Gain Control")
	fmt.Println("--------------------------")

	// Create AGC effect
	agcEffect := audio.NewAutoGainEffect()
	defer agcEffect.Close()

	fmt.Printf("Initial AGC gain: %.3f\n", agcEffect.GetCurrentGain())

	// Test with quiet signal
	quietAudio := generateTestAudio(480, 1000) // Very quiet
	fmt.Printf("Quiet signal level: max = %d\n", maxAmplitude(quietAudio))

	processedAudio, err := agcEffect.Process(quietAudio)
	if err != nil {
		logger.WithError(err).Error("AGC processing failed")
		return
	}

	fmt.Printf("After AGC: max = %d, gain = %.3f\n",
		maxAmplitude(processedAudio), agcEffect.GetCurrentGain())

	// Test with loud signal
	loudAudio := generateTestAudio(480, 25000) // Very loud
	fmt.Printf("Loud signal level: max = %d\n", maxAmplitude(loudAudio))

	processedAudio, err = agcEffect.Process(loudAudio)
	if err != nil {
		logger.WithError(err).Error("AGC processing failed")
		return
	}

	fmt.Printf("After AGC: max = %d, gain = %.3f\n",
		maxAmplitude(processedAudio), agcEffect.GetCurrentGain())

	// Set custom target level
	err = agcEffect.SetTargetLevel(0.5)
	if err != nil {
		logger.WithError(err).Error("Failed to set target level")
		return
	}

	fmt.Printf("AGC target level updated to 50%%\n")
}

// demonstrateEffectChain shows how to chain multiple effects for sequential processing.
func demonstrateEffectChain() {
	fmt.Println("\n3. Effect Chain Processing")
	fmt.Println("---------------------------")

	// Create effect chain
	chain := audio.NewEffectChain()
	defer chain.Close()

	// Add pre-gain
	preGain, err := audio.NewGainEffect(1.5)
	if err != nil {
		logger.WithError(err).Error("Failed to create pre-gain")
		return
	}

	// Add AGC
	agc := audio.NewAutoGainEffect()

	// Add post-gain
	postGain, err := audio.NewGainEffect(0.8)
	if err != nil {
		logger.WithError(err).Error("Failed to create post-gain")
		return
	}

	// Build chain
	chain.AddEffect(preGain)
	chain.AddEffect(agc)
	chain.AddEffect(postGain)

	fmt.Printf("Effect chain created with %d effects\n", chain.GetEffectCount())
	fmt.Printf("Effects: %v\n", chain.GetEffectNames())

	// Process audio through chain
	testAudio := generateTestAudio(480, 8000)
	fmt.Printf("Original audio: max = %d\n", maxAmplitude(testAudio))

	processedAudio, err := chain.Process(testAudio)
	if err != nil {
		logger.WithError(err).Error("Chain processing failed")
		return
	}

	fmt.Printf("After effect chain: max = %d\n", maxAmplitude(processedAudio))
}

// demonstrateProcessorIntegration shows how to integrate effects with the audio processor.
func demonstrateProcessorIntegration() {
	fmt.Println("\n4. Processor Integration")
	fmt.Println("------------------------")

	// Create audio processor with effects support
	processor := audio.NewProcessor()
	defer processor.Close()

	testAudio := generateTestAudio(480, 12000)

	// Test without effects
	result1, err := processor.ProcessOutgoing(testAudio, 48000)
	if err != nil {
		logger.WithError(err).Error("Processing failed")
		return
	}
	fmt.Printf("Without effects: encoded %d bytes\n", len(result1))

	// Add gain effect
	err = processor.SetGain(0.7)
	if err != nil {
		logger.WithError(err).Error("Failed to set gain")
		return
	}

	result2, err := processor.ProcessOutgoing(testAudio, 48000)
	if err != nil {
		logger.WithError(err).Error("Processing with gain failed")
		return
	}
	fmt.Printf("With gain effect: encoded %d bytes\n", len(result2))

	// Enable AGC
	err = processor.EnableAutoGain()
	if err != nil {
		logger.WithError(err).Error("Failed to enable AGC")
		return
	}

	result3, err := processor.ProcessOutgoing(testAudio, 48000)
	if err != nil {
		logger.WithError(err).Error("Processing with AGC failed")
		return
	}
	fmt.Printf("With AGC: encoded %d bytes\n", len(result3))

	// Check effect chain
	chain := processor.GetEffectChain()
	if chain != nil {
		fmt.Printf("Active effects: %v\n", chain.GetEffectNames())
	}

	// Disable effects
	err = processor.DisableEffects()
	if err != nil {
		logger.WithError(err).Error("Failed to disable effects")
		return
	}

	fmt.Printf("Effects disabled, chain count: %d\n", chain.GetEffectCount())
}

// Helper functions

// generateTestAudio creates a sine wave test signal with the specified number of samples and amplitude.
func generateTestAudio(samples int, amplitude int16) []int16 {
	result := make([]int16, samples)
	for i := 0; i < samples; i++ {
		// Generate 1kHz sine wave at 48kHz sample rate
		angle := 2.0 * math.Pi * float64(i) / 48.0
		sample := float64(amplitude) * math.Sin(angle)
		result[i] = int16(sample)
	}
	return result
}

// maxAmplitude returns the maximum absolute amplitude value in an audio sample slice.
func maxAmplitude(samples []int16) int16 {
	var max int16
	for _, sample := range samples {
		if sample < 0 {
			sample = -sample
		}
		if sample > max {
			max = sample
		}
	}
	return max
}

func init() {
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}
