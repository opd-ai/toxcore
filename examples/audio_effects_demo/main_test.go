package main

import (
	"testing"
)

func TestGenerateTestAudio(t *testing.T) {
	tests := []struct {
		name      string
		samples   int
		amplitude int16
		wantLen   int
	}{
		{
			name:      "480 samples at moderate level",
			samples:   480,
			amplitude: 10000,
			wantLen:   480,
		},
		{
			name:      "960 samples at max level",
			samples:   960,
			amplitude: 32767,
			wantLen:   960,
		},
		{
			name:      "zero samples",
			samples:   0,
			amplitude: 1000,
			wantLen:   0,
		},
		{
			name:      "single sample",
			samples:   1,
			amplitude: 5000,
			wantLen:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateTestAudio(tt.samples, tt.amplitude)
			if len(result) != tt.wantLen {
				t.Errorf("generateTestAudio() returned %d samples, want %d", len(result), tt.wantLen)
			}

			// Verify samples are within expected amplitude range
			if tt.samples > 0 {
				maxAmp := maxAmplitude(result)
				if maxAmp > tt.amplitude {
					t.Errorf("maxAmplitude() = %d, exceeds input amplitude %d", maxAmp, tt.amplitude)
				}
			}
		})
	}
}

func TestMaxAmplitude(t *testing.T) {
	tests := []struct {
		name    string
		samples []int16
		want    int16
	}{
		{
			name:    "positive values only",
			samples: []int16{100, 200, 150, 50},
			want:    200,
		},
		{
			name:    "negative values only",
			samples: []int16{-100, -200, -150, -50},
			want:    200,
		},
		{
			name:    "mixed values",
			samples: []int16{-300, 100, -50, 250},
			want:    300,
		},
		{
			name:    "all zeros",
			samples: []int16{0, 0, 0, 0},
			want:    0,
		},
		{
			name:    "empty slice",
			samples: []int16{},
			want:    0,
		},
		{
			name:    "single positive value",
			samples: []int16{500},
			want:    500,
		},
		{
			name:    "single negative value",
			samples: []int16{-500},
			want:    500,
		},
		{
			name:    "max int16",
			samples: []int16{32767, -1000, 5000},
			want:    32767,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxAmplitude(tt.samples)
			if got != tt.want {
				t.Errorf("maxAmplitude() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGenerateTestAudioWaveform(t *testing.T) {
	// Test that generated audio has expected sine wave characteristics
	samples := generateTestAudio(480, 10000)

	// Check that we have both positive and negative samples (sine wave property)
	hasPositive := false
	hasNegative := false
	for _, s := range samples {
		if s > 0 {
			hasPositive = true
		}
		if s < 0 {
			hasNegative = true
		}
	}

	if !hasPositive {
		t.Error("sine wave should have positive samples")
	}
	if !hasNegative {
		t.Error("sine wave should have negative samples")
	}
}

func TestMaxAmplitudeWithGeneratedAudio(t *testing.T) {
	// Integration test: generate audio and verify max amplitude
	testCases := []struct {
		amplitude int16
	}{
		{1000},
		{5000},
		{10000},
		{20000},
	}

	for _, tc := range testCases {
		audio := generateTestAudio(480, tc.amplitude)
		maxAmp := maxAmplitude(audio)

		// Max amplitude should be close to but not exceed the input amplitude
		// Allow small tolerance for floating point rounding
		if maxAmp > tc.amplitude {
			t.Errorf("maxAmplitude %d exceeded input amplitude %d", maxAmp, tc.amplitude)
		}

		// For non-trivial amplitudes, max should be reasonably close to input
		if tc.amplitude > 100 {
			minExpected := tc.amplitude * 9 / 10 // At least 90% of input
			if maxAmp < minExpected {
				t.Errorf("maxAmplitude %d too low for input amplitude %d (expected at least %d)",
					maxAmp, tc.amplitude, minExpected)
			}
		}
	}
}

func BenchmarkGenerateTestAudio(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateTestAudio(480, 10000)
	}
}

func BenchmarkMaxAmplitude(b *testing.B) {
	samples := generateTestAudio(480, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		maxAmplitude(samples)
	}
}
