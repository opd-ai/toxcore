package main

import (
	"fmt"
	"log"
	"math"

	"github.com/opd-ai/toxforge/av/audio"
)

// Example demonstrating audio effects usage in ToxAV
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

func demonstrateGainControl() {
	fmt.Println("\n1. Basic Gain Control")
	fmt.Println("---------------------")

	// Create gain effect with 50% volume
	gainEffect, err := audio.NewGainEffect(0.5)
	if err != nil {
		log.Fatalf("Failed to create gain effect: %v", err)
	}
	defer gainEffect.Close()

	// Generate test audio (1kHz sine wave)
	testAudio := generateTestAudio(480, 10000) // 10ms at 48kHz, moderate level

	fmt.Printf("Original audio level: max = %d\n", maxAmplitude(testAudio))

	// Apply gain
	processedAudio, err := gainEffect.Process(testAudio)
	if err != nil {
		log.Printf("Gain processing failed: %v", err)
		return
	}

	fmt.Printf("After 0.5x gain: max = %d\n", maxAmplitude(processedAudio))

	// Change gain dynamically
	err = gainEffect.SetGain(2.0)
	if err != nil {
		log.Printf("Failed to set gain: %v", err)
		return
	}

	processedAudio, err = gainEffect.Process(testAudio)
	if err != nil {
		log.Printf("Gain processing failed: %v", err)
		return
	}

	fmt.Printf("After 2.0x gain: max = %d\n", maxAmplitude(processedAudio))
	fmt.Printf("Effect name: %s\n", gainEffect.GetName())
}

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
		log.Printf("AGC processing failed: %v", err)
		return
	}

	fmt.Printf("After AGC: max = %d, gain = %.3f\n",
		maxAmplitude(processedAudio), agcEffect.GetCurrentGain())

	// Test with loud signal
	loudAudio := generateTestAudio(480, 25000) // Very loud
	fmt.Printf("Loud signal level: max = %d\n", maxAmplitude(loudAudio))

	processedAudio, err = agcEffect.Process(loudAudio)
	if err != nil {
		log.Printf("AGC processing failed: %v", err)
		return
	}

	fmt.Printf("After AGC: max = %d, gain = %.3f\n",
		maxAmplitude(processedAudio), agcEffect.GetCurrentGain())

	// Set custom target level
	err = agcEffect.SetTargetLevel(0.5)
	if err != nil {
		log.Printf("Failed to set target level: %v", err)
		return
	}

	fmt.Printf("AGC target level updated to 50%%\n")
}

func demonstrateEffectChain() {
	fmt.Println("\n3. Effect Chain Processing")
	fmt.Println("---------------------------")

	// Create effect chain
	chain := audio.NewEffectChain()
	defer chain.Close()

	// Add pre-gain
	preGain, err := audio.NewGainEffect(1.5)
	if err != nil {
		log.Printf("Failed to create pre-gain: %v", err)
		return
	}

	// Add AGC
	agc := audio.NewAutoGainEffect()

	// Add post-gain
	postGain, err := audio.NewGainEffect(0.8)
	if err != nil {
		log.Printf("Failed to create post-gain: %v", err)
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
		log.Printf("Chain processing failed: %v", err)
		return
	}

	fmt.Printf("After effect chain: max = %d\n", maxAmplitude(processedAudio))
}

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
		log.Printf("Processing failed: %v", err)
		return
	}
	fmt.Printf("Without effects: encoded %d bytes\n", len(result1))

	// Add gain effect
	err = processor.SetGain(0.7)
	if err != nil {
		log.Printf("Failed to set gain: %v", err)
		return
	}

	result2, err := processor.ProcessOutgoing(testAudio, 48000)
	if err != nil {
		log.Printf("Processing with gain failed: %v", err)
		return
	}
	fmt.Printf("With gain effect: encoded %d bytes\n", len(result2))

	// Enable AGC
	err = processor.EnableAutoGain()
	if err != nil {
		log.Printf("Failed to enable AGC: %v", err)
		return
	}

	result3, err := processor.ProcessOutgoing(testAudio, 48000)
	if err != nil {
		log.Printf("Processing with AGC failed: %v", err)
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
		log.Printf("Failed to disable effects: %v", err)
		return
	}

	fmt.Printf("Effects disabled, chain count: %d\n", chain.GetEffectCount())
}

// Helper functions

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
	// Set up logging
	log.SetFlags(log.Ltime | log.Lshortfile)
}
