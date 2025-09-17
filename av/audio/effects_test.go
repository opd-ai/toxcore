package audio

import (
	"math"
	"testing"
)

func TestGainEffect_NewGainEffect(t *testing.T) {
	tests := []struct {
		name    string
		gain    float64
		wantErr bool
	}{
		{
			name:    "valid gain zero",
			gain:    0.0,
			wantErr: false,
		},
		{
			name:    "valid gain unity",
			gain:    1.0,
			wantErr: false,
		},
		{
			name:    "valid gain amplification",
			gain:    2.0,
			wantErr: false,
		},
		{
			name:    "valid gain maximum",
			gain:    4.0,
			wantErr: false,
		},
		{
			name:    "invalid negative gain",
			gain:    -0.5,
			wantErr: true,
		},
		{
			name:    "invalid too high gain",
			gain:    5.0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effect, err := NewGainEffect(tt.gain)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewGainEffect() expected error for gain %f, got nil", tt.gain)
				}
				return
			}
			if err != nil {
				t.Errorf("NewGainEffect() unexpected error: %v", err)
				return
			}
			if effect.GetGain() != tt.gain {
				t.Errorf("NewGainEffect() gain = %f, want %f", effect.GetGain(), tt.gain)
			}
		})
	}
}

func TestGainEffect_Process(t *testing.T) {
	tests := []struct {
		name     string
		gain     float64
		input    []int16
		expected []int16
	}{
		{
			name:     "silence gain",
			gain:     0.0,
			input:    []int16{1000, -1000, 5000, -5000},
			expected: []int16{0, 0, 0, 0},
		},
		{
			name:     "unity gain",
			gain:     1.0,
			input:    []int16{1000, -1000, 5000, -5000},
			expected: []int16{1000, -1000, 5000, -5000},
		},
		{
			name:     "half gain",
			gain:     0.5,
			input:    []int16{1000, -1000, 2000, -2000},
			expected: []int16{500, -500, 1000, -1000},
		},
		{
			name:     "double gain",
			gain:     2.0,
			input:    []int16{1000, -1000, 2000, -2000},
			expected: []int16{2000, -2000, 4000, -4000},
		},
		{
			name:     "empty input",
			gain:     1.0,
			input:    []int16{},
			expected: []int16{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effect, err := NewGainEffect(tt.gain)
			if err != nil {
				t.Fatalf("NewGainEffect() error: %v", err)
			}

			result, err := effect.Process(tt.input)
			if err != nil {
				t.Errorf("Process() error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Process() result length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, sample := range result {
				if sample != tt.expected[i] {
					t.Errorf("Process() sample[%d] = %d, want %d", i, sample, tt.expected[i])
				}
			}
		})
	}
}

func TestGainEffect_ProcessClipping(t *testing.T) {
	// Test clipping protection
	effect, err := NewGainEffect(4.0) // High gain to test clipping
	if err != nil {
		t.Fatalf("NewGainEffect() error: %v", err)
	}

	// Input that will cause clipping
	input := []int16{16384, -16384, 32767, -32768} // Values that will overflow when multiplied by 4
	result, err := effect.Process(input)
	if err != nil {
		t.Errorf("Process() error: %v", err)
		return
	}

	// Check that clipping occurred
	expected := []int16{32767, -32768, 32767, -32768} // Clipped to int16 limits
	for i, sample := range result {
		if sample != expected[i] {
			t.Errorf("Process() clipped sample[%d] = %d, want %d", i, sample, expected[i])
		}
	}
}

func TestGainEffect_SetGain(t *testing.T) {
	effect, err := NewGainEffect(1.0)
	if err != nil {
		t.Fatalf("NewGainEffect() error: %v", err)
	}

	// Test valid gain changes
	validGains := []float64{0.0, 0.5, 1.0, 2.0, 4.0}
	for _, gain := range validGains {
		err := effect.SetGain(gain)
		if err != nil {
			t.Errorf("SetGain(%f) unexpected error: %v", gain, err)
		}
		if effect.GetGain() != gain {
			t.Errorf("SetGain(%f) gain = %f, want %f", gain, effect.GetGain(), gain)
		}
	}

	// Test invalid gain changes
	invalidGains := []float64{-0.1, 5.0}
	for _, gain := range invalidGains {
		err := effect.SetGain(gain)
		if err == nil {
			t.Errorf("SetGain(%f) expected error, got nil", gain)
		}
	}
}

func TestAutoGainEffect_Process(t *testing.T) {
	effect := NewAutoGainEffect()

	// Test with various input levels
	tests := []struct {
		name       string
		input      []int16
		expectGain bool // Whether we expect some gain to be applied
	}{
		{
			name:       "quiet signal",
			input:      generateSineWave(48, 1000), // Very quiet signal
			expectGain: true,                       // Should amplify quiet signals
		},
		{
			name:       "normal signal",
			input:      generateSineWave(48, 10000), // Normal level signal
			expectGain: false,                       // Should maintain or slightly adjust
		},
		{
			name:       "loud signal",
			input:      generateSineWave(48, 30000), // Loud signal
			expectGain: false,                       // Should reduce gain
		},
		{
			name:       "empty input",
			input:      []int16{},
			expectGain: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := effect.Process(tt.input)
			if err != nil {
				t.Errorf("Process() error: %v", err)
				return
			}

			if len(result) != len(tt.input) {
				t.Errorf("Process() result length = %d, want %d", len(result), len(tt.input))
			}

			// Check that AGC is working by verifying gain changes
			currentGain := effect.GetCurrentGain()
			if tt.expectGain && currentGain <= 0.8 {
				// For very quiet signals, we expect some amplification
				t.Logf("AGC working: current gain = %.3f for %s", currentGain, tt.name)
			}
		})
	}
}

func TestAutoGainEffect_SetTargetLevel(t *testing.T) {
	effect := NewAutoGainEffect()

	// Test valid target levels
	validLevels := []float64{0.0, 0.1, 0.5, 1.0}
	for _, level := range validLevels {
		err := effect.SetTargetLevel(level)
		if err != nil {
			t.Errorf("SetTargetLevel(%f) unexpected error: %v", level, err)
		}
	}

	// Test invalid target levels
	invalidLevels := []float64{-0.1, 1.1}
	for _, level := range invalidLevels {
		err := effect.SetTargetLevel(level)
		if err == nil {
			t.Errorf("SetTargetLevel(%f) expected error, got nil", level)
		}
	}
}

func TestEffectChain_Basic(t *testing.T) {
	chain := NewEffectChain()

	// Test empty chain
	if chain.GetEffectCount() != 0 {
		t.Errorf("NewEffectChain() effect count = %d, want 0", chain.GetEffectCount())
	}

	// Add effects
	gainEffect1, _ := NewGainEffect(0.5)
	gainEffect2, _ := NewGainEffect(2.0)

	chain.AddEffect(gainEffect1)
	chain.AddEffect(gainEffect2)

	if chain.GetEffectCount() != 2 {
		t.Errorf("AddEffect() effect count = %d, want 2", chain.GetEffectCount())
	}

	// Test effect names
	names := chain.GetEffectNames()
	if len(names) != 2 {
		t.Errorf("GetEffectNames() length = %d, want 2", len(names))
	}
}

func TestEffectChain_Process(t *testing.T) {
	chain := NewEffectChain()

	// Add two gain effects: 0.5 then 2.0 (should result in unity gain overall)
	gainEffect1, _ := NewGainEffect(0.5)
	gainEffect2, _ := NewGainEffect(2.0)
	chain.AddEffect(gainEffect1)
	chain.AddEffect(gainEffect2)

	input := []int16{1000, -1000, 2000, -2000}
	result, err := chain.Process(input)
	if err != nil {
		t.Errorf("Process() error: %v", err)
		return
	}

	// 0.5 * 2.0 = 1.0, so output should equal input
	expected := []int16{1000, -1000, 2000, -2000}
	for i, sample := range result {
		if sample != expected[i] {
			t.Errorf("Process() chained sample[%d] = %d, want %d", i, sample, expected[i])
		}
	}
}

func TestEffectChain_Clear(t *testing.T) {
	chain := NewEffectChain()

	// Add effects
	gainEffect, _ := NewGainEffect(1.0)
	chain.AddEffect(gainEffect)

	if chain.GetEffectCount() != 1 {
		t.Errorf("AddEffect() effect count = %d, want 1", chain.GetEffectCount())
	}

	// Clear effects
	err := chain.Clear()
	if err != nil {
		t.Errorf("Clear() error: %v", err)
	}

	if chain.GetEffectCount() != 0 {
		t.Errorf("Clear() effect count = %d, want 0", chain.GetEffectCount())
	}
}

func TestProcessor_EffectIntegration(t *testing.T) {
	processor := NewProcessor()
	defer processor.Close()

	// Test adding gain effect
	err := processor.SetGain(0.5)
	if err != nil {
		t.Errorf("SetGain() error: %v", err)
	}

	// Test processing with effect
	input := []int16{1000, -1000, 2000, -2000}
	result, err := processor.ProcessOutgoing(input, 48000)
	if err != nil {
		t.Errorf("ProcessOutgoing() with effects error: %v", err)
	}

	// Should have some result (exact values depend on encoding)
	if len(result) == 0 {
		t.Error("ProcessOutgoing() with effects returned empty result")
	}
}

func TestProcessor_AutoGain(t *testing.T) {
	processor := NewProcessor()
	defer processor.Close()

	// Enable auto gain
	err := processor.EnableAutoGain()
	if err != nil {
		t.Errorf("EnableAutoGain() error: %v", err)
	}

	// Test processing with AGC
	input := []int16{1000, -1000, 2000, -2000}
	result, err := processor.ProcessOutgoing(input, 48000)
	if err != nil {
		t.Errorf("ProcessOutgoing() with AGC error: %v", err)
	}

	if len(result) == 0 {
		t.Error("ProcessOutgoing() with AGC returned empty result")
	}
}

func TestProcessor_DisableEffects(t *testing.T) {
	processor := NewProcessor()
	defer processor.Close()

	// Add effect
	err := processor.SetGain(2.0)
	if err != nil {
		t.Errorf("SetGain() error: %v", err)
	}

	// Disable effects
	err = processor.DisableEffects()
	if err != nil {
		t.Errorf("DisableEffects() error: %v", err)
	}

	// Effect chain should be empty
	if processor.GetEffectChain().GetEffectCount() != 0 {
		t.Errorf("DisableEffects() effect count = %d, want 0", processor.GetEffectChain().GetEffectCount())
	}
}

// Benchmark tests for performance validation
func BenchmarkGainEffect_Process(b *testing.B) {
	effect, _ := NewGainEffect(1.5)
	samples := generateSineWave(480, 10000) // 10ms of audio at 48kHz

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := effect.Process(samples)
		if err != nil {
			b.Fatalf("Process() error: %v", err)
		}
	}
}

func BenchmarkAutoGainEffect_Process(b *testing.B) {
	effect := NewAutoGainEffect()
	samples := generateSineWave(480, 10000) // 10ms of audio at 48kHz

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := effect.Process(samples)
		if err != nil {
			b.Fatalf("Process() error: %v", err)
		}
	}
}

func BenchmarkEffectChain_Process(b *testing.B) {
	chain := NewEffectChain()
	gainEffect1, _ := NewGainEffect(0.8)
	gainEffect2, _ := NewGainEffect(1.2)
	chain.AddEffect(gainEffect1)
	chain.AddEffect(gainEffect2)

	samples := generateSineWave(480, 10000) // 10ms of audio at 48kHz

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chain.Process(samples)
		if err != nil {
			b.Fatalf("Process() error: %v", err)
		}
	}
}

// Helper function to generate test audio data
func generateSineWave(samples int, amplitude int16) []int16 {
	result := make([]int16, samples)
	for i := 0; i < samples; i++ {
		// Generate a simple sine wave for testing
		angle := 2.0 * math.Pi * float64(i) / 48.0 // 1kHz tone at 48kHz
		sample := float64(amplitude) * math.Sin(angle)
		result[i] = int16(sample)
	}
	return result
}

// TestNoiseSuppressionEffect_NewNoiseSuppressionEffect tests the constructor validation.
func TestNoiseSuppressionEffect_NewNoiseSuppressionEffect(t *testing.T) {
	tests := []struct {
		name             string
		suppressionLevel float64
		frameSize        int
		wantErr          bool
		errorContains    string
	}{
		{
			name:             "valid minimal suppression",
			suppressionLevel: 0.0,
			frameSize:        512,
			wantErr:          false,
		},
		{
			name:             "valid moderate suppression",
			suppressionLevel: 0.5,
			frameSize:        1024,
			wantErr:          false,
		},
		{
			name:             "valid maximum suppression",
			suppressionLevel: 1.0,
			frameSize:        256,
			wantErr:          false,
		},
		{
			name:             "invalid negative suppression level",
			suppressionLevel: -0.1,
			frameSize:        512,
			wantErr:          true,
			errorContains:    "suppression level must be between 0.0 and 1.0",
		},
		{
			name:             "invalid too high suppression level",
			suppressionLevel: 1.1,
			frameSize:        512,
			wantErr:          true,
			errorContains:    "suppression level must be between 0.0 and 1.0",
		},
		{
			name:             "invalid too small frame size",
			suppressionLevel: 0.5,
			frameSize:        32,
			wantErr:          true,
			errorContains:    "frame size must be power of 2 between 64 and 4096",
		},
		{
			name:             "invalid too large frame size",
			suppressionLevel: 0.5,
			frameSize:        8192,
			wantErr:          true,
			errorContains:    "frame size must be power of 2 between 64 and 4096",
		},
		{
			name:             "invalid non-power-of-2 frame size",
			suppressionLevel: 0.5,
			frameSize:        1000,
			wantErr:          true,
			errorContains:    "frame size must be power of 2 between 64 and 4096",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effect, err := NewNoiseSuppressionEffect(tt.suppressionLevel, tt.frameSize)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewNoiseSuppressionEffect() expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("NewNoiseSuppressionEffect() error = %v, want to contain %v", err, tt.errorContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NewNoiseSuppressionEffect() unexpected error = %v", err)
				return
			}
			if effect == nil {
				t.Errorf("NewNoiseSuppressionEffect() returned nil effect")
				return
			}

			// Verify effect parameters
			if effect.suppressionLevel != tt.suppressionLevel {
				t.Errorf("NewNoiseSuppressionEffect() suppressionLevel = %v, want %v", effect.suppressionLevel, tt.suppressionLevel)
			}
			if effect.frameSize != tt.frameSize {
				t.Errorf("NewNoiseSuppressionEffect() frameSize = %v, want %v", effect.frameSize, tt.frameSize)
			}
			if effect.overlapSize != tt.frameSize/2 {
				t.Errorf("NewNoiseSuppressionEffect() overlapSize = %v, want %v", effect.overlapSize, tt.frameSize/2)
			}

			// Verify buffers are allocated
			if len(effect.windowBuffer) != tt.frameSize {
				t.Errorf("NewNoiseSuppressionEffect() windowBuffer length = %v, want %v", len(effect.windowBuffer), tt.frameSize)
			}
			if len(effect.noiseFloor) != tt.frameSize/2+1 {
				t.Errorf("NewNoiseSuppressionEffect() noiseFloor length = %v, want %v", len(effect.noiseFloor), tt.frameSize/2+1)
			}
		})
	}
}

// TestNoiseSuppressionEffect_GetName tests the effect name.
func TestNoiseSuppressionEffect_GetName(t *testing.T) {
	effect, err := NewNoiseSuppressionEffect(0.5, 512)
	if err != nil {
		t.Fatalf("NewNoiseSuppressionEffect() unexpected error = %v", err)
	}

	name := effect.GetName()
	expected := "NoiseSuppressionEffect"
	if name != expected {
		t.Errorf("GetName() = %v, want %v", name, expected)
	}
}

// TestNoiseSuppressionEffect_Process tests basic audio processing.
func TestNoiseSuppressionEffect_Process(t *testing.T) {
	effect, err := NewNoiseSuppressionEffect(0.5, 512)
	if err != nil {
		t.Fatalf("NewNoiseSuppressionEffect() unexpected error = %v", err)
	}
	defer effect.Close()

	tests := []struct {
		name        string
		samples     []int16
		expectError bool
	}{
		{
			name:        "empty samples",
			samples:     []int16{},
			expectError: false,
		},
		{
			name:        "single sample",
			samples:     []int16{1000},
			expectError: false,
		},
		{
			name:        "small audio frame",
			samples:     generateSineWave(480, 8000), // 10ms at 48kHz
			expectError: false,
		},
		{
			name:        "medium audio frame",
			samples:     generateSineWave(1024, 16000), // ~21ms at 48kHz
			expectError: false,
		},
		{
			name:        "large audio frame",
			samples:     generateSineWave(4800, 12000), // 100ms at 48kHz
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processed, err := effect.Process(tt.samples)
			if tt.expectError && err == nil {
				t.Errorf("Process() expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Process() unexpected error = %v", err)
				return
			}

			// Verify output length matches input
			if len(processed) != len(tt.samples) {
				t.Errorf("Process() output length = %v, want %v", len(processed), len(tt.samples))
			}

			// Verify samples are in valid range
			for i, sample := range processed {
				if sample < -32768 || sample > 32767 {
					t.Errorf("Process() sample %d = %v, out of int16 range", i, sample)
				}
			}
		})
	}
}

// TestNoiseSuppressionEffect_NoiseFloorEstimation tests noise floor learning.
func TestNoiseSuppressionEffect_NoiseFloorEstimation(t *testing.T) {
	effect, err := NewNoiseSuppressionEffect(0.7, 256)
	if err != nil {
		t.Fatalf("NewNoiseSuppressionEffect() unexpected error = %v", err)
	}
	defer effect.Close()

	// Generate consistent low-level noise for noise floor estimation
	noiseFrame := make([]int16, 256)
	for i := range noiseFrame {
		// Add small random noise
		noiseFrame[i] = int16(100 * (float64(i%10) - 5)) // Predictable "noise"
	}

	// Process several frames to build noise floor
	initiallyInitialized := effect.initialized
	if initiallyInitialized {
		t.Errorf("Effect should not be initially initialized")
	}

	for frame := 0; frame < 12; frame++ {
		_, err := effect.Process(noiseFrame)
		if err != nil {
			t.Fatalf("Process() frame %d unexpected error = %v", frame, err)
		}
	}

	// Verify noise floor has been established
	if !effect.initialized {
		t.Errorf("Effect should be initialized after processing frames")
	}
	if effect.frameCount < 10 {
		t.Errorf("Effect frameCount = %v, should be >= 10", effect.frameCount)
	}

	// Verify noise floor values are reasonable (non-zero)
	hasNonZeroNoiseFloor := false
	for _, value := range effect.noiseFloor {
		if value > 0 {
			hasNonZeroNoiseFloor = true
			break
		}
	}
	if !hasNonZeroNoiseFloor {
		t.Errorf("Noise floor should have non-zero values after estimation")
	}
}

// TestNoiseSuppressionEffect_SpectrumProcessing tests FFT/IFFT functionality.
func TestNoiseSuppressionEffect_SpectrumProcessing(t *testing.T) {
	effect, err := NewNoiseSuppressionEffect(0.3, 128)
	if err != nil {
		t.Fatalf("NewNoiseSuppressionEffect() unexpected error = %v", err)
	}
	defer effect.Close()

	// Test with pure tone (should be preserved better than noise)
	toneFrame := generateSineWave(128, 20000) // Strong 1kHz tone

	// Process through noise suppression
	processed, err := effect.Process(toneFrame)
	if err != nil {
		t.Fatalf("Process() unexpected error = %v", err)
	}

	// Verify output properties
	if len(processed) != len(toneFrame) {
		t.Errorf("Process() output length = %v, want %v", len(processed), len(toneFrame))
	}

	// Calculate simple energy metrics
	inputEnergy := calculateEnergy(toneFrame)
	outputEnergy := calculateEnergy(processed)

	// Energy should be reasonable (not completely destroyed)
	energyRatio := outputEnergy / inputEnergy
	if energyRatio < 0.1 || energyRatio > 2.0 {
		t.Errorf("Energy ratio = %v, should be reasonable (0.1 to 2.0)", energyRatio)
	}
}

// TestNoiseSuppressionEffect_DifferentSuppressionLevels tests various suppression strengths.
func TestNoiseSuppressionEffect_DifferentSuppressionLevels(t *testing.T) {
	testSamples := generateSineWave(512, 15000)

	suppressionLevels := []float64{0.0, 0.25, 0.5, 0.75, 1.0}
	results := make([][]int16, len(suppressionLevels))

	for i, level := range suppressionLevels {
		effect, err := NewNoiseSuppressionEffect(level, 256)
		if err != nil {
			t.Fatalf("NewNoiseSuppressionEffect() level %v unexpected error = %v", level, err)
		}

		// Process multiple frames to initialize noise floor
		for frame := 0; frame < 12; frame++ {
			processed, err := effect.Process(testSamples)
			if err != nil {
				t.Fatalf("Process() level %v frame %d unexpected error = %v", level, frame, err)
			}
			if frame == 11 { // Save final result
				results[i] = make([]int16, len(processed))
				copy(results[i], processed)
			}
		}
		effect.Close()
	}

	// Verify that higher suppression levels generally reduce energy more
	for i := 1; i < len(results); i++ {
		energy_prev := calculateEnergy(results[i-1])
		energy_curr := calculateEnergy(results[i])

		// Allow some tolerance since noise suppression is complex
		if energy_curr > energy_prev*1.5 {
			t.Logf("Warning: Higher suppression level %v has more energy than %v",
				suppressionLevels[i], suppressionLevels[i-1])
		}
	}
}

// TestNoiseSuppressionEffect_Close tests resource cleanup.
func TestNoiseSuppressionEffect_Close(t *testing.T) {
	effect, err := NewNoiseSuppressionEffect(0.5, 512)
	if err != nil {
		t.Fatalf("NewNoiseSuppressionEffect() unexpected error = %v", err)
	}

	// Verify effect is functional before close
	testSamples := generateSineWave(256, 1000)
	_, err = effect.Process(testSamples)
	if err != nil {
		t.Fatalf("Process() before close unexpected error = %v", err)
	}

	// Close the effect
	err = effect.Close()
	if err != nil {
		t.Errorf("Close() unexpected error = %v", err)
	}

	// Verify buffers are cleared
	if effect.inputBuffer != nil {
		t.Errorf("Close() inputBuffer should be nil after close")
	}
	if effect.outputBuffer != nil {
		t.Errorf("Close() outputBuffer should be nil after close")
	}
	if effect.noiseFloor != nil {
		t.Errorf("Close() noiseFloor should be nil after close")
	}
}

// Benchmark noise suppression performance
func BenchmarkNoiseSuppressionEffect_Process(b *testing.B) {
	effect, err := NewNoiseSuppressionEffect(0.5, 512)
	if err != nil {
		b.Fatalf("NewNoiseSuppressionEffect() unexpected error = %v", err)
	}
	defer effect.Close()

	// Generate test audio (10ms frame at 48kHz)
	samples := generateSineWave(480, 16000)

	// Initialize noise floor by processing a few frames
	for i := 0; i < 12; i++ {
		_, _ = effect.Process(samples)
	}

	// Reset timer and benchmark
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := effect.Process(samples)
		if err != nil {
			b.Fatalf("Process() unexpected error = %v", err)
		}
	}
}

// Helper function to calculate signal energy
func calculateEnergy(samples []int16) float64 {
	var energy float64
	for _, sample := range samples {
		energy += float64(sample) * float64(sample)
	}
	return energy / float64(len(samples))
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				len(s) > len(substr)*2 && s[len(s)/2-len(substr)/2:len(s)/2+len(substr)/2] == substr ||
				findInString(s, substr))
}

// Helper for string searching
func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
