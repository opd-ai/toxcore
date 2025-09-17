package audio

import (
	"testing"
)

// TestNoiseSuppressionIntegration tests noise suppression in the full audio processing pipeline.
func TestNoiseSuppressionIntegration(t *testing.T) {
	// Create audio processor with noise suppression effect
	processor := NewProcessor()
	defer processor.Close()

	// Create noise suppression effect
	noiseSuppressionEffect, err := NewNoiseSuppressionEffect(0.7, 512)
	if err != nil {
		t.Fatalf("NewNoiseSuppressionEffect() unexpected error = %v", err)
	}
	defer noiseSuppressionEffect.Close()

	// Add noise suppression to the processor's effect chain
	processor.AddEffect(noiseSuppressionEffect)

	// Generate test audio with some "noise"
	testAudio := make([]int16, 480) // 10ms at 48kHz
	for i := range testAudio {
		// Add a mix of signal and noise
		signal := int16(8000)            // Strong signal
		noise := int16(100 * (i%10 - 5)) // Weak repeating pattern as "noise"
		testAudio[i] = signal + noise
	}

	// Process audio multiple times to allow noise floor estimation
	var processedAudio []byte
	for frame := 0; frame < 15; frame++ {
		processed, err := processor.ProcessOutgoing(testAudio, 48000)
		if err != nil {
			t.Fatalf("ProcessOutgoing() frame %d unexpected error = %v", frame, err)
		}
		if frame == 14 { // Save final result
			processedAudio = processed
		}
	}

	// Verify we got processed audio output
	if len(processedAudio) == 0 {
		t.Errorf("ProcessOutgoing() should produce audio output")
	}

	// Verify noise suppression effect was applied (it should be initialized by now)
	if !noiseSuppressionEffect.initialized {
		t.Errorf("Noise suppression effect should be initialized after processing frames")
	}

	// Verify effect chain includes noise suppression
	effectCount := processor.effectChain.GetEffectCount()
	if effectCount != 1 {
		t.Errorf("Effect chain should have 1 effect, got %d", effectCount)
	}
}

// TestMultipleEffectsWithNoiseSuppression tests noise suppression combined with other effects.
func TestMultipleEffectsWithNoiseSuppression(t *testing.T) {
	// Create audio processor
	processor := NewProcessor()
	defer processor.Close()

	// Create gain effect
	gainEffect, err := NewGainEffect(1.5)
	if err != nil {
		t.Fatalf("NewGainEffect() unexpected error = %v", err)
	}
	defer gainEffect.Close()

	// Create noise suppression effect
	noiseSuppressionEffect, err := NewNoiseSuppressionEffect(0.5, 256)
	if err != nil {
		t.Fatalf("NewNoiseSuppressionEffect() unexpected error = %v", err)
	}
	defer noiseSuppressionEffect.Close()

	// Add effects to the processor's effect chain (gain first, then noise suppression)
	processor.AddEffect(gainEffect)
	processor.AddEffect(noiseSuppressionEffect)

	// Generate test audio
	testAudio := generateSineWave(480, 10000) // 10ms sine wave

	// Process audio to apply both effects
	for frame := 0; frame < 12; frame++ {
		_, err := processor.ProcessOutgoing(testAudio, 48000)
		if err != nil {
			t.Fatalf("ProcessOutgoing() frame %d unexpected error = %v", frame, err)
		}
	}

	// Verify both effects are in the chain
	effectCount := processor.effectChain.GetEffectCount()
	if effectCount != 2 {
		t.Errorf("Effect chain should have 2 effects, got %d", effectCount)
	}

	// Verify noise suppression is initialized
	if !noiseSuppressionEffect.initialized {
		t.Errorf("Noise suppression effect should be initialized after processing frames")
	}
}

// TestNoiseSuppressionPerformanceInPipeline benchmarks noise suppression in the full pipeline.
func BenchmarkNoiseSuppressionInPipeline(b *testing.B) {
	// Create audio processor with noise suppression
	processor := NewProcessor()
	defer processor.Close()

	noiseSuppressionEffect, err := NewNoiseSuppressionEffect(0.6, 512)
	if err != nil {
		b.Fatalf("NewNoiseSuppressionEffect() unexpected error = %v", err)
	}
	defer noiseSuppressionEffect.Close()

	processor.AddEffect(noiseSuppressionEffect)

	// Initialize noise floor with a few frames
	testAudio := generateSineWave(480, 12000)
	for i := 0; i < 12; i++ {
		_, _ = processor.ProcessOutgoing(testAudio, 48000)
	}

	// Benchmark the full pipeline with noise suppression
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := processor.ProcessOutgoing(testAudio, 48000)
		if err != nil {
			b.Fatalf("ProcessOutgoing() unexpected error = %v", err)
		}
	}
}
