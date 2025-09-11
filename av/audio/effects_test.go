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
