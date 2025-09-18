package main

import (
	"testing"
	"time"
)

// TestNewEffectsDemo tests the creation of a new effects demo
func TestNewEffectsDemo(t *testing.T) {
	demo, err := NewEffectsDemo()
	if err != nil {
		t.Fatalf("Failed to create EffectsDemo: %v", err)
	}
	defer demo.cleanup()

	// Verify initial state
	if demo.audioGain != 1.0 {
		t.Errorf("Expected initial audio gain 1.0, got %f", demo.audioGain)
	}

	if demo.noiseSuppressionLevel != 0.5 {
		t.Errorf("Expected initial noise suppression 0.5, got %f", demo.noiseSuppressionLevel)
	}

	if demo.agcTargetLevel != 0.7 {
		t.Errorf("Expected initial AGC target 0.7, got %f", demo.agcTargetLevel)
	}

	if demo.colorTemperature != 6500 {
		t.Errorf("Expected initial color temperature 6500K, got %d", demo.colorTemperature)
	}

	// Verify ToxAV instance is created
	if demo.toxav == nil {
		t.Error("ToxAV instance should not be nil")
	}

	// Verify Tox instance is created
	if demo.tox == nil {
		t.Error("Tox instance should not be nil")
	}
}

// TestAudioCommands tests audio effect command handling
func TestAudioCommands(t *testing.T) {
	demo, err := NewEffectsDemo()
	if err != nil {
		t.Fatalf("Failed to create EffectsDemo: %v", err)
	}
	defer demo.cleanup()

	tests := []struct {
		name          string
		args          []string
		expectedGain  float64
		expectedNoise float64
		expectedAGC   float64
		expectError   bool
	}{
		{"Set valid gain", []string{"gain", "1.5"}, 1.5, 0.5, 0.7, false},
		{"Set valid noise", []string{"noise", "0.8"}, 1.5, 0.8, 0.7, false},
		{"Set valid AGC", []string{"agc", "0.9"}, 1.5, 0.8, 0.9, false},
		{"Reset audio", []string{"reset"}, 1.0, 0.5, 0.7, false},
		{"Invalid gain high", []string{"gain", "5.0"}, 1.0, 0.5, 0.7, true},
		{"Invalid gain low", []string{"gain", "-0.1"}, 1.0, 0.5, 0.7, true},
		{"Invalid noise high", []string{"noise", "1.5"}, 1.0, 0.5, 0.7, true},
		{"Invalid AGC format", []string{"agc", "invalid"}, 1.0, 0.5, 0.7, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			demo.handleAudioCommand(tt.args)

			// Check if values changed as expected (only for valid commands)
			if !tt.expectError {
				if demo.audioGain != tt.expectedGain {
					t.Errorf("Expected audio gain %f, got %f", tt.expectedGain, demo.audioGain)
				}
				if demo.noiseSuppressionLevel != tt.expectedNoise {
					t.Errorf("Expected noise suppression %f, got %f", tt.expectedNoise, demo.noiseSuppressionLevel)
				}
				if demo.agcTargetLevel != tt.expectedAGC {
					t.Errorf("Expected AGC target %f, got %f", tt.expectedAGC, demo.agcTargetLevel)
				}
			}
		})
	}
}

// TestVideoCommands tests video effect command handling
func TestVideoCommands(t *testing.T) {
	demo, err := NewEffectsDemo()
	if err != nil {
		t.Fatalf("Failed to create EffectsDemo: %v", err)
	}
	defer demo.cleanup()

	tests := []struct {
		name         string
		args         []string
		expectedTemp int
		expectError  bool
	}{
		{"Set warm temperature", []string{"temp", "3000"}, 3000, false},
		{"Set cool temperature", []string{"temp", "10000"}, 10000, false},
		{"Set daylight temperature", []string{"temp", "6500"}, 6500, false},
		{"Reset video", []string{"reset"}, 6500, false},
		{"Invalid temp low", []string{"temp", "1000"}, 6500, true},
		{"Invalid temp high", []string{"temp", "25000"}, 6500, true},
		{"Invalid temp format", []string{"temp", "invalid"}, 6500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			demo.handleVideoCommand(tt.args)

			// Check if temperature changed as expected (only for valid commands)
			if !tt.expectError {
				if demo.colorTemperature != tt.expectedTemp {
					t.Errorf("Expected color temperature %d, got %d", tt.expectedTemp, demo.colorTemperature)
				}
			}
		})
	}
}

// TestGetTemperatureDescription tests color temperature descriptions
func TestGetTemperatureDescription(t *testing.T) {
	demo := &EffectsDemo{}

	tests := []struct {
		temp     int
		expected string
	}{
		{2500, "very warm"},
		{3500, "warm"},
		{4500, "neutral warm"},
		{5500, "neutral"},
		{6500, "cool"},
		{8000, "very cool"},
		{15000, "extremely cool"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := demo.getTemperatureDescription(tt.temp)
			if result != tt.expected {
				t.Errorf("Expected description '%s' for %dK, got '%s'", tt.expected, tt.temp, result)
			}
		})
	}
}

// TestApplyColorTemperatureEffect tests color temperature effect application
func TestApplyColorTemperatureEffect(t *testing.T) {
	demo := &EffectsDemo{colorTemperature: 3000} // Warm temperature

	const width, height = 8, 8
	u := make([]byte, width*height)
	v := make([]byte, width*height)

	// Fill with middle values
	for i := range u {
		u[i] = 128
		v[i] = 128
	}

	originalU := make([]byte, len(u))
	originalV := make([]byte, len(v))
	copy(originalU, u)
	copy(originalV, v)

	demo.applyColorTemperatureEffect(u, v, width, height)

	// For warm temperature (3000K < 6500K), we expect:
	// - U values to be reduced (less blue)
	// - V values to be increased (more red)

	// Check that at least some values changed
	uChanged := false
	vChanged := false

	for i := range u {
		if u[i] != originalU[i] {
			uChanged = true
		}
		if v[i] != originalV[i] {
			vChanged = true
		}
	}

	if !uChanged {
		t.Error("Expected U values to change with color temperature effect")
	}
	if !vChanged {
		t.Error("Expected V values to change with color temperature effect")
	}
}

// TestFrameGeneration tests audio frame generation with effects
func TestFrameGeneration(t *testing.T) {
	demo, err := NewEffectsDemo()
	if err != nil {
		t.Fatalf("Failed to create EffectsDemo: %v", err)
	}
	defer demo.cleanup()

	// Set active friend to enable frame generation
	demo.hasActiveFriend = true
	demo.friendNumber = 0

	// Test audio frame generation
	initialAudioCount := demo.audioFrameCount
	demo.generateAudioWithEffects()

	if demo.audioFrameCount != initialAudioCount+1 {
		t.Errorf("Expected audio frame count to increase by 1, got %d to %d",
			initialAudioCount, demo.audioFrameCount)
	}

	// Test video frame generation
	initialVideoCount := demo.videoFrameCount
	demo.generateVideoWithEffects()

	if demo.videoFrameCount != initialVideoCount+1 {
		t.Errorf("Expected video frame count to increase by 1, got %d to %d",
			initialVideoCount, demo.videoFrameCount)
	}
}

// TestPerformanceTracking tests performance metrics tracking
func TestPerformanceTracking(t *testing.T) {
	demo, err := NewEffectsDemo()
	if err != nil {
		t.Fatalf("Failed to create EffectsDemo: %v", err)
	}
	defer demo.cleanup()

	// Set active friend to enable frame generation
	demo.hasActiveFriend = true
	demo.friendNumber = 0

	// Generate some frames to accumulate timing data
	for i := 0; i < 5; i++ {
		demo.generateAudioWithEffects()
		demo.generateVideoWithEffects()
	}

	// Check that performance data was collected
	if demo.audioFrameCount == 0 {
		t.Error("Expected audio frame count to be > 0")
	}

	if demo.videoFrameCount == 0 {
		t.Error("Expected video frame count to be > 0")
	}

	if demo.totalAudioTime == 0 {
		t.Error("Expected total audio time to be > 0")
	}

	if demo.totalVideoTime == 0 {
		t.Error("Expected total video time to be > 0")
	}

	// Test average calculation (should not panic)
	avgAudio := demo.totalAudioTime / time.Duration(demo.audioFrameCount)
	avgVideo := demo.totalVideoTime / time.Duration(demo.videoFrameCount)

	if avgAudio <= 0 {
		t.Error("Expected positive average audio processing time")
	}

	if avgVideo <= 0 {
		t.Error("Expected positive average video processing time")
	}
}

// TestCommandHandling tests general command handling
func TestCommandHandling(t *testing.T) {
	demo, err := NewEffectsDemo()
	if err != nil {
		t.Fatalf("Failed to create EffectsDemo: %v", err)
	}
	defer demo.cleanup()

	// Test quit command
	demo.running = true
	demo.handleCommand("quit")
	if demo.running {
		t.Error("Expected demo to stop running after quit command")
	}

	// Reset for next test
	demo.running = true
	demo.handleCommand("exit")
	if demo.running {
		t.Error("Expected demo to stop running after exit command")
	}

	// Test empty command (should not panic)
	demo.running = true
	demo.handleCommand("")
	if !demo.running {
		t.Error("Expected demo to keep running after empty command")
	}
}

// BenchmarkAudioFrameGeneration benchmarks audio frame generation with effects
func BenchmarkAudioFrameGeneration(b *testing.B) {
	demo, err := NewEffectsDemo()
	if err != nil {
		b.Fatalf("Failed to create EffectsDemo: %v", err)
	}
	defer demo.cleanup()

	demo.hasActiveFriend = true
	demo.friendNumber = 0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demo.generateAudioWithEffects()
	}
}

// BenchmarkVideoFrameGeneration benchmarks video frame generation with effects
func BenchmarkVideoFrameGeneration(b *testing.B) {
	demo, err := NewEffectsDemo()
	if err != nil {
		b.Fatalf("Failed to create EffectsDemo: %v", err)
	}
	defer demo.cleanup()

	demo.hasActiveFriend = true
	demo.friendNumber = 0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demo.generateVideoWithEffects()
	}
}

// BenchmarkColorTemperatureEffect benchmarks color temperature effect application
func BenchmarkColorTemperatureEffect(b *testing.B) {
	demo := &EffectsDemo{colorTemperature: 3000}

	const width, height = 320, 240
	u := make([]byte, width*height)
	v := make([]byte, width*height)

	// Fill with test data
	for i := range u {
		u[i] = uint8(i % 256)
		v[i] = uint8((i * 2) % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demo.applyColorTemperatureEffect(u, v, width, height)
	}
}
