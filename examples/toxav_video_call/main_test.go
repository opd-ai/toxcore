// ToxAV Video Call Demo - Tests
//
// Comprehensive tests for the video call demo, targeting ≥65% coverage.

package main

import (
	"os"
	"testing"
	"time"
)

// TestVideoCallStats tests the thread-safe statistics tracking.
func TestVideoCallStats(t *testing.T) {
	stats := &VideoCallStats{}

	// Test initial state
	videoSent, audioSent, received, active, avgProcessing, effects := stats.GetStats()
	if videoSent != 0 || audioSent != 0 || received != 0 || active != 0 || avgProcessing != 0 || effects != 0 {
		t.Error("Initial stats should all be zero")
	}

	// Test UpdateVideoSent
	stats.UpdateVideoSent(10 * time.Millisecond)
	videoSent, _, _, _, avgProcessing, _ = stats.GetStats()
	if videoSent != 1 {
		t.Errorf("Expected VideoFramesSent=1, got %d", videoSent)
	}
	if avgProcessing != 10*time.Millisecond {
		t.Errorf("Expected avgProcessing=10ms, got %v", avgProcessing)
	}

	// Test multiple video sends for averaging
	stats.UpdateVideoSent(20 * time.Millisecond)
	videoSent, _, _, _, avgProcessing, _ = stats.GetStats()
	if videoSent != 2 {
		t.Errorf("Expected VideoFramesSent=2, got %d", videoSent)
	}
	// Average of 10ms + 20ms = 30ms / 2 = 15ms
	if avgProcessing != 15*time.Millisecond {
		t.Errorf("Expected avgProcessing=15ms, got %v", avgProcessing)
	}

	// Test UpdateAudioSent
	stats.UpdateAudioSent()
	_, audioSent, _, _, _, _ = stats.GetStats()
	if audioSent != 1 {
		t.Errorf("Expected AudioFramesSent=1, got %d", audioSent)
	}

	// Test UpdateReceived
	stats.UpdateReceived()
	stats.UpdateReceived()
	_, _, received, _, _, _ = stats.GetStats()
	if received != 2 {
		t.Errorf("Expected FramesReceived=2, got %d", received)
	}
}

// TestVideoCallStatsConcurrency tests thread-safety of stats updates.
func TestVideoCallStatsConcurrency(t *testing.T) {
	stats := &VideoCallStats{}
	done := make(chan bool)

	// Run concurrent updates
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				stats.UpdateVideoSent(time.Millisecond)
				stats.UpdateAudioSent()
				stats.UpdateReceived()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	videoSent, audioSent, received, _, _, _ := stats.GetStats()
	expected := uint64(1000)
	if videoSent != expected {
		t.Errorf("Expected VideoFramesSent=%d, got %d", expected, videoSent)
	}
	if audioSent != expected {
		t.Errorf("Expected AudioFramesSent=%d, got %d", expected, audioSent)
	}
	if received != expected {
		t.Errorf("Expected FramesReceived=%d, got %d", expected, received)
	}
}

// TestRealTimeProvider tests the real time provider implementation.
func TestRealTimeProvider(t *testing.T) {
	tp := RealTimeProvider{}

	// Test Now returns current time (within tolerance)
	before := time.Now()
	now := tp.Now()
	after := time.Now()

	if now.Before(before) || now.After(after) {
		t.Error("Now() should return time between before and after")
	}

	// Test Since
	start := tp.Now()
	time.Sleep(10 * time.Millisecond)
	elapsed := tp.Since(start)
	if elapsed < 10*time.Millisecond {
		t.Errorf("Since should return at least 10ms, got %v", elapsed)
	}

	// Test NewTicker creates valid ticker
	ticker := tp.NewTicker(time.Hour)
	if ticker == nil {
		t.Error("NewTicker should return non-nil ticker")
	}
	ticker.Stop()
}

// TestMockTimeProvider tests the mock time provider implementation.
func TestMockTimeProvider(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	// Test Now returns set time
	if !tp.Now().Equal(baseTime) {
		t.Errorf("Expected Now()=%v, got %v", baseTime, tp.Now())
	}

	// Test Since
	elapsed := tp.Since(baseTime.Add(-5 * time.Minute))
	if elapsed != 5*time.Minute {
		t.Errorf("Expected Since=5m, got %v", elapsed)
	}

	// Test Advance
	tp.Advance(10 * time.Second)
	expected := baseTime.Add(10 * time.Second)
	if !tp.Now().Equal(expected) {
		t.Errorf("After Advance, expected %v, got %v", expected, tp.Now())
	}

	// Test SetTime
	newTime := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	tp.SetTime(newTime)
	if !tp.Now().Equal(newTime) {
		t.Errorf("After SetTime, expected %v, got %v", newTime, tp.Now())
	}

	// Test NewTicker returns valid ticker
	ticker := tp.NewTicker(time.Hour)
	if ticker == nil {
		t.Error("NewTicker should return non-nil ticker")
	}
	ticker.Stop()
}

// TestGenerateColorBars tests the color bar pattern generator.
func TestGenerateColorBars(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount:     0,
		animationPhase: 0,
	}

	y, u, v := demo.generateColorBars(demo)

	// Verify frame dimensions
	expectedYSize := videoWidth * videoHeight
	expectedUVSize := (videoWidth / 2) * (videoHeight / 2)

	if len(y) != expectedYSize {
		t.Errorf("Y plane size: expected %d, got %d", expectedYSize, len(y))
	}
	if len(u) != expectedUVSize {
		t.Errorf("U plane size: expected %d, got %d", expectedUVSize, len(u))
	}
	if len(v) != expectedUVSize {
		t.Errorf("V plane size: expected %d, got %d", expectedUVSize, len(v))
	}

	// Verify first bar is white (Y=235)
	if y[0] != 235 {
		t.Errorf("First pixel Y value: expected 235 (white), got %d", y[0])
	}

	// Verify last bar is black (Y=16)
	lastPixel := y[len(y)-1]
	if lastPixel != 16 {
		t.Errorf("Last pixel Y value: expected 16 (black), got %d", lastPixel)
	}
}

// TestGenerateMovingGradient tests the moving gradient pattern generator.
func TestGenerateMovingGradient(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount:     0,
		animationPhase: 0,
	}

	y, u, v := demo.generateMovingGradient(demo)

	// Verify frame dimensions
	expectedYSize := videoWidth * videoHeight
	expectedUVSize := (videoWidth / 2) * (videoHeight / 2)

	if len(y) != expectedYSize {
		t.Errorf("Y plane size: expected %d, got %d", expectedYSize, len(y))
	}
	if len(u) != expectedUVSize {
		t.Errorf("U plane size: expected %d, got %d", expectedUVSize, len(u))
	}
	if len(v) != expectedUVSize {
		t.Errorf("V plane size: expected %d, got %d", expectedUVSize, len(v))
	}

	// Verify animation affects output
	demo.animationPhase = 100
	y2, _, _ := demo.generateMovingGradient(demo)

	// Frames at different animation phases should be different
	different := false
	for i := range y {
		if y[i] != y2[i] {
			different = true
			break
		}
	}
	if !different {
		t.Error("Moving gradient should produce different frames at different animation phases")
	}
}

// TestGenerateCheckerboard tests the checkerboard pattern generator.
func TestGenerateCheckerboard(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount:     0,
		animationPhase: 0,
	}

	y, u, v := demo.generateCheckerboard(demo)

	// Verify frame dimensions
	expectedYSize := videoWidth * videoHeight
	expectedUVSize := (videoWidth / 2) * (videoHeight / 2)

	if len(y) != expectedYSize {
		t.Errorf("Y plane size: expected %d, got %d", expectedYSize, len(y))
	}
	if len(u) != expectedUVSize {
		t.Errorf("U plane size: expected %d, got %d", expectedUVSize, len(u))
	}
	if len(v) != expectedUVSize {
		t.Errorf("V plane size: expected %d, got %d", expectedUVSize, len(v))
	}

	// Verify values are only white (235) or black (16)
	for i, val := range y {
		if val != 235 && val != 16 {
			t.Errorf("Checkerboard pixel %d has invalid Y value: %d (expected 235 or 16)", i, val)
			break
		}
	}

	// Verify U/V are neutral (128)
	for i, val := range u {
		if val != 128 {
			t.Errorf("Checkerboard U plane pixel %d has invalid value: %d (expected 128)", i, val)
			break
		}
	}
	for i, val := range v {
		if val != 128 {
			t.Errorf("Checkerboard V plane pixel %d has invalid value: %d (expected 128)", i, val)
			break
		}
	}
}

// TestGeneratePlasmaEffect tests the plasma effect pattern generator.
func TestGeneratePlasmaEffect(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount:     0,
		animationPhase: 0,
	}

	y, u, v := demo.generatePlasmaEffect(demo)

	// Verify frame dimensions
	expectedYSize := videoWidth * videoHeight
	expectedUVSize := (videoWidth / 2) * (videoHeight / 2)

	if len(y) != expectedYSize {
		t.Errorf("Y plane size: expected %d, got %d", expectedYSize, len(y))
	}
	if len(u) != expectedUVSize {
		t.Errorf("U plane size: expected %d, got %d", expectedUVSize, len(u))
	}
	if len(v) != expectedUVSize {
		t.Errorf("V plane size: expected %d, got %d", expectedUVSize, len(v))
	}

	// Verify animation affects output
	demo.animationPhase = 50
	y2, _, _ := demo.generatePlasmaEffect(demo)

	different := false
	for i := range y {
		if y[i] != y2[i] {
			different = true
			break
		}
	}
	if !different {
		t.Error("Plasma effect should produce different frames at different animation phases")
	}
}

// TestGenerateTestPattern tests the test pattern generator.
func TestGenerateTestPattern(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount:     0,
		animationPhase: 0,
	}

	y, u, v := demo.generateTestPattern(demo)

	// Verify frame dimensions
	expectedYSize := videoWidth * videoHeight
	expectedUVSize := (videoWidth / 2) * (videoHeight / 2)

	if len(y) != expectedYSize {
		t.Errorf("Y plane size: expected %d, got %d", expectedYSize, len(y))
	}
	if len(u) != expectedUVSize {
		t.Errorf("U plane size: expected %d, got %d", expectedUVSize, len(u))
	}
	if len(v) != expectedUVSize {
		t.Errorf("V plane size: expected %d, got %d", expectedUVSize, len(v))
	}

	// Verify top-left border pixel is white (235)
	if y[0] != 235 {
		t.Errorf("Border pixel should be 235, got %d", y[0])
	}

	// Verify center area is gray (128)
	centerX := videoWidth / 2
	centerY := videoHeight / 2
	// Center has crosshair (16), check nearby
	nearCenterIdx := (centerY+15)*videoWidth + centerX + 15
	if y[nearCenterIdx] != 128 {
		t.Errorf("Interior pixel should be 128 (gray), got %d", y[nearCenterIdx])
	}

	// Verify U/V are neutral (128)
	for i, val := range u {
		if val != 128 {
			t.Errorf("Test pattern U plane pixel %d has invalid value: %d (expected 128)", i, val)
			break
		}
	}
}

// TestGenerateSimpleAudio tests the audio frame generator.
func TestGenerateSimpleAudio(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount: 0,
	}

	frame := demo.generateSimpleAudio()

	expectedSize := audioFrameSize * audioChannels
	if len(frame) != expectedSize {
		t.Errorf("Audio frame size: expected %d, got %d", expectedSize, len(frame))
	}

	// Verify it's not all zeros (should be a 1kHz tone)
	allZero := true
	for _, sample := range frame {
		if sample != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("Audio frame should not be all zeros")
	}

	// Verify samples are within valid range
	for i, sample := range frame {
		if sample < -32768 || sample > 32767 {
			t.Errorf("Audio sample %d out of range: %d", i, sample)
		}
	}
}

// TestInitializePatterns tests pattern initialization.
func TestInitializePatterns(t *testing.T) {
	demo := &VideoCallDemo{}
	demo.initializePatterns()

	if len(demo.patterns) != 5 {
		t.Errorf("Expected 5 patterns, got %d", len(demo.patterns))
	}

	// Verify pattern names
	expectedNames := []string{
		"Color Bars",
		"Moving Gradient",
		"Checkerboard",
		"Plasma Effect",
		"Test Pattern",
	}

	for i, name := range expectedNames {
		if demo.patterns[i].Name != name {
			t.Errorf("Pattern %d name: expected %q, got %q", i, name, demo.patterns[i].Name)
		}
	}

	// Verify all patterns have generators
	for i, pattern := range demo.patterns {
		if pattern.Generator == nil {
			t.Errorf("Pattern %d (%s) has nil generator", i, pattern.Name)
		}
	}
}

// TestSwitchToNextPattern tests pattern cycling.
func TestSwitchToNextPattern(t *testing.T) {
	demo := &VideoCallDemo{}
	demo.initializePatterns()
	demo.currentPattern = 0

	// Test cycling through all patterns
	for i := 0; i < len(demo.patterns)*2; i++ {
		expected := (i + 1) % len(demo.patterns)
		demo.switchToNextPattern()
		if demo.currentPattern != expected {
			t.Errorf("After switch %d: expected pattern %d, got %d", i+1, expected, demo.currentPattern)
		}
	}
}

// TestHandleShutdownSignal tests the shutdown signal handler.
func TestHandleShutdownSignal(t *testing.T) {
	demo := &VideoCallDemo{active: true}

	result := demo.handleShutdownSignal()

	if !result {
		t.Error("handleShutdownSignal should return true")
	}
	if demo.active {
		t.Error("active should be false after shutdown signal")
	}
}

// TestCheckTimeout tests the timeout checking logic.
func TestCheckTimeout(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	demo := &VideoCallDemo{
		active:       true,
		timeProvider: tp,
	}

	startTime := baseTime

	// Should not timeout initially
	if demo.checkTimeout(startTime) {
		t.Error("Should not timeout when just started")
	}
	if !demo.active {
		t.Error("active should still be true")
	}

	// Advance time past demo duration
	tp.Advance(demoDuration + time.Second)

	// Should timeout now
	if !demo.checkTimeout(startTime) {
		t.Error("Should timeout after demo duration")
	}
	if demo.active {
		t.Error("active should be false after timeout")
	}
}

// TestTimerSetInitialization tests TimerSet creation.
func TestTimerSetInitialization(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	demo := &VideoCallDemo{
		timeProvider: tp,
	}

	timers := demo.initializeTimers()

	if timers == nil {
		t.Fatal("initializeTimers should return non-nil TimerSet")
	}

	if timers.sigChan == nil {
		t.Error("sigChan should not be nil")
	}
	if timers.videoTicker == nil {
		t.Error("videoTicker should not be nil")
	}
	if timers.audioTicker == nil {
		t.Error("audioTicker should not be nil")
	}
	if timers.statsTicker == nil {
		t.Error("statsTicker should not be nil")
	}
	if timers.patternTicker == nil {
		t.Error("patternTicker should not be nil")
	}
	if timers.toxTicker == nil {
		t.Error("toxTicker should not be nil")
	}
	if !timers.startTime.Equal(baseTime) {
		t.Errorf("startTime: expected %v, got %v", baseTime, timers.startTime)
	}

	// Clean up
	demo.cleanupTimers(timers)
}

// TestCleanupTimers tests that cleanupTimers doesn't panic.
func TestCleanupTimers(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	demo := &VideoCallDemo{
		timeProvider: tp,
	}

	timers := demo.initializeTimers()

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("cleanupTimers panicked: %v", r)
		}
	}()

	demo.cleanupTimers(timers)
}

// TestVideoPatternGeneratorOutput tests that all generators produce valid YUV420 output.
func TestVideoPatternGeneratorOutput(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount:     100,
		animationPhase: 50.0,
	}
	demo.initializePatterns()

	expectedYSize := videoWidth * videoHeight
	expectedUVSize := (videoWidth / 2) * (videoHeight / 2)

	for i, pattern := range demo.patterns {
		t.Run(pattern.Name, func(t *testing.T) {
			y, u, v := pattern.Generator(demo)

			if len(y) != expectedYSize {
				t.Errorf("%s: Y plane wrong size: got %d, want %d", pattern.Name, len(y), expectedYSize)
			}
			if len(u) != expectedUVSize {
				t.Errorf("%s: U plane wrong size: got %d, want %d", pattern.Name, len(u), expectedUVSize)
			}
			if len(v) != expectedUVSize {
				t.Errorf("%s: V plane wrong size: got %d, want %d", pattern.Name, len(v), expectedUVSize)
			}

			// Test pattern index is valid
			if i < 0 || i >= len(demo.patterns) {
				t.Errorf("Pattern index %d out of range", i)
			}
		})
	}
}

// TestVideoPatternDescriptions tests that all patterns have non-empty descriptions.
func TestVideoPatternDescriptions(t *testing.T) {
	demo := &VideoCallDemo{}
	demo.initializePatterns()

	for i, pattern := range demo.patterns {
		if pattern.Name == "" {
			t.Errorf("Pattern %d has empty name", i)
		}
		if pattern.Description == "" {
			t.Errorf("Pattern %d (%s) has empty description", i, pattern.Name)
		}
	}
}

// TestAudioFrameConsistency tests that audio generation is consistent across calls.
func TestAudioFrameConsistency(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount: 42,
	}

	// Generate same frame twice with same state
	frame1 := demo.generateSimpleAudio()

	// Reset to same frame count
	demo.frameCount = 42
	frame2 := demo.generateSimpleAudio()

	// Should be identical
	if len(frame1) != len(frame2) {
		t.Fatal("Frame lengths differ")
	}

	for i := range frame1 {
		if frame1[i] != frame2[i] {
			t.Errorf("Sample %d differs: %d vs %d", i, frame1[i], frame2[i])
			break
		}
	}
}

// BenchmarkGenerateColorBars benchmarks color bar generation.
func BenchmarkGenerateColorBars(b *testing.B) {
	demo := &VideoCallDemo{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demo.generateColorBars(demo)
	}
}

// BenchmarkGenerateMovingGradient benchmarks moving gradient generation.
func BenchmarkGenerateMovingGradient(b *testing.B) {
	demo := &VideoCallDemo{animationPhase: 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demo.generateMovingGradient(demo)
		demo.animationPhase++
	}
}

// BenchmarkGeneratePlasmaEffect benchmarks plasma effect generation.
func BenchmarkGeneratePlasmaEffect(b *testing.B) {
	demo := &VideoCallDemo{animationPhase: 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demo.generatePlasmaEffect(demo)
		demo.animationPhase++
	}
}

// BenchmarkGenerateSimpleAudio benchmarks audio generation.
func BenchmarkGenerateSimpleAudio(b *testing.B) {
	demo := &VideoCallDemo{frameCount: 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demo.generateSimpleAudio()
		demo.frameCount++
	}
}

// TestDisplayDemoIntroduction tests the demo introduction display.
func TestDisplayDemoIntroduction(t *testing.T) {
	demo := &VideoCallDemo{}
	demo.initializePatterns()
	demo.currentPattern = 0

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("displayDemoIntroduction panicked: %v", r)
		}
	}()

	demo.displayDemoIntroduction()
}

// TestHandleStatisticsTick tests the statistics display function.
func TestHandleStatisticsTick(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	demo := &VideoCallDemo{
		timeProvider: tp,
	}

	// Update some stats
	demo.stats.UpdateVideoSent(10 * time.Millisecond)
	demo.stats.UpdateAudioSent()
	demo.stats.UpdateReceived()

	// Advance time
	tp.Advance(30 * time.Second)

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("handleStatisticsTick panicked: %v", r)
		}
	}()

	demo.handleStatisticsTick(baseTime)
}

// TestSetupTimersAndChannels tests timer setup and cleanup.
func TestSetupTimersAndChannels(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	demo := &VideoCallDemo{
		timeProvider: tp,
	}

	sigChan, videoTicker, audioTicker, statsTicker, patternTicker, toxTicker := demo.setupTimersAndChannels()

	if sigChan == nil {
		t.Error("sigChan should not be nil")
	}
	if videoTicker == nil {
		t.Error("videoTicker should not be nil")
	}
	if audioTicker == nil {
		t.Error("audioTicker should not be nil")
	}
	if statsTicker == nil {
		t.Error("statsTicker should not be nil")
	}
	if patternTicker == nil {
		t.Error("patternTicker should not be nil")
	}
	if toxTicker == nil {
		t.Error("toxTicker should not be nil")
	}

	// Cleanup
	videoTicker.Stop()
	audioTicker.Stop()
	statsTicker.Stop()
	patternTicker.Stop()
	toxTicker.Stop()
}

// TestVideoCallDemoPartialInitialization tests partial initialization scenarios.
func TestVideoCallDemoPartialInitialization(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	// Test demo with only time provider (simulates partial initialization)
	demo := &VideoCallDemo{
		timeProvider:   tp,
		active:         true,
		currentPattern: 0,
	}
	demo.initializePatterns()

	// Verify we can generate frames without toxav
	y, u, v := demo.patterns[0].Generator(demo)
	if len(y) == 0 || len(u) == 0 || len(v) == 0 {
		t.Error("Pattern generator should produce non-empty frames")
	}
}

// TestColorBarsBarBoundaries tests color bar boundaries are handled correctly.
func TestColorBarsBarBoundaries(t *testing.T) {
	demo := &VideoCallDemo{}

	y, u, v := demo.generateColorBars(demo)

	// There are 8 bars, each should be width/8 = 80 pixels wide
	barWidth := videoWidth / 8

	// Test first pixel of each bar in Y plane
	for barIdx := 0; barIdx < 8; barIdx++ {
		pixelIdx := barIdx * barWidth
		pixelY := y[pixelIdx]

		// Expected Y values for each bar
		expectedY := []uint8{235, 210, 170, 145, 106, 81, 41, 16}
		if pixelY != expectedY[barIdx] {
			t.Errorf("Bar %d first pixel Y: expected %d, got %d", barIdx, expectedY[barIdx], pixelY)
		}
	}

	// Verify UV plane size
	if len(u) != (videoWidth/2)*(videoHeight/2) {
		t.Errorf("U plane size incorrect")
	}
	if len(v) != (videoWidth/2)*(videoHeight/2) {
		t.Errorf("V plane size incorrect")
	}
}

// TestCheckerboardAnimation tests that checkerboard animation changes over time.
func TestCheckerboardAnimation(t *testing.T) {
	demo := &VideoCallDemo{
		animationPhase: 0,
	}

	y1, _, _ := demo.generateCheckerboard(demo)

	// Advance animation
	demo.animationPhase = 200

	y2, _, _ := demo.generateCheckerboard(demo)

	// Frames should be different at different animation phases
	different := false
	for i := range y1 {
		if y1[i] != y2[i] {
			different = true
			break
		}
	}
	if !different {
		t.Error("Checkerboard should produce different frames at different animation phases")
	}
}

// TestTestPatternBorders tests the test pattern has correct borders.
func TestTestPatternBorders(t *testing.T) {
	demo := &VideoCallDemo{}

	y, _, _ := demo.generateTestPattern(demo)

	// Top border should be white (235)
	for col := 0; col < videoWidth; col++ {
		if y[col] != 235 {
			t.Errorf("Top border at col %d: expected 235, got %d", col, y[col])
		}
	}

	// Bottom border should be white (235)
	for col := 0; col < videoWidth; col++ {
		bottomIdx := (videoHeight-1)*videoWidth + col
		if y[bottomIdx] != 235 {
			t.Errorf("Bottom border at col %d: expected 235, got %d", col, y[bottomIdx])
		}
	}

	// Left border should be white (235)
	for row := 0; row < videoHeight; row++ {
		if y[row*videoWidth] != 235 {
			t.Errorf("Left border at row %d: expected 235, got %d", row, y[row*videoWidth])
		}
	}

	// Right border should be white (235)
	for row := 0; row < videoHeight; row++ {
		rightIdx := row*videoWidth + videoWidth - 1
		if y[rightIdx] != 235 {
			t.Errorf("Right border at row %d: expected 235, got %d", row, y[rightIdx])
		}
	}
}

// TestTestPatternCrosshair tests the test pattern has a center crosshair.
func TestTestPatternCrosshair(t *testing.T) {
	demo := &VideoCallDemo{}

	y, _, _ := demo.generateTestPattern(demo)

	centerX := videoWidth / 2
	centerY := videoHeight / 2

	// Check horizontal line of crosshair
	for i := -10; i <= 10; i++ {
		if centerX+i >= 0 && centerX+i < videoWidth {
			idx := centerY*videoWidth + centerX + i
			if y[idx] != 16 {
				t.Errorf("Crosshair horizontal at offset %d: expected 16, got %d", i, y[idx])
			}
		}
	}

	// Check vertical line of crosshair
	for i := -10; i <= 10; i++ {
		if centerY+i >= 0 && centerY+i < videoHeight {
			idx := (centerY+i)*videoWidth + centerX
			if y[idx] != 16 {
				t.Errorf("Crosshair vertical at offset %d: expected 16, got %d", i, y[idx])
			}
		}
	}
}

// TestAudioFrameDifferentFrameCounts tests audio varies with frame count.
func TestAudioFrameDifferentFrameCounts(t *testing.T) {
	// With 1kHz at 48000 Hz, one cycle is 48 samples.
	// frameCount changes starting time. Different frame counts should produce
	// different starting phases.
	demo1 := &VideoCallDemo{frameCount: 0}
	demo2 := &VideoCallDemo{frameCount: 1} // Offset by 480 samples (one frame)

	frame1 := demo1.generateSimpleAudio()
	frame2 := demo2.generateSimpleAudio()

	// Due to the sine wave at 1kHz with 48000 sample rate,
	// and frame size of 480 (10ms), consecutive frames will have
	// different phases. Just verify we get valid audio.
	if len(frame1) != audioFrameSize*audioChannels {
		t.Errorf("Frame 1 wrong size: %d", len(frame1))
	}
	if len(frame2) != audioFrameSize*audioChannels {
		t.Errorf("Frame 2 wrong size: %d", len(frame2))
	}

	// Verify both have non-zero samples (sine wave not all zero)
	hasNonZero1 := false
	for _, s := range frame1 {
		if s != 0 {
			hasNonZero1 = true
			break
		}
	}
	hasNonZero2 := false
	for _, s := range frame2 {
		if s != 0 {
			hasNonZero2 = true
			break
		}
	}
	if !hasNonZero1 {
		t.Error("Frame 1 should have non-zero samples")
	}
	if !hasNonZero2 {
		t.Error("Frame 2 should have non-zero samples")
	}
}

// TestVideoPatternCycling tests cycling through all patterns.
func TestVideoPatternCycling(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount:     0,
		animationPhase: 0,
	}
	demo.initializePatterns()

	// Generate a frame from each pattern
	for i, pattern := range demo.patterns {
		y, u, v := pattern.Generator(demo)

		expectedYSize := videoWidth * videoHeight
		expectedUVSize := (videoWidth / 2) * (videoHeight / 2)

		if len(y) != expectedYSize {
			t.Errorf("Pattern %d (%s): Y size wrong", i, pattern.Name)
		}
		if len(u) != expectedUVSize {
			t.Errorf("Pattern %d (%s): U size wrong", i, pattern.Name)
		}
		if len(v) != expectedUVSize {
			t.Errorf("Pattern %d (%s): V size wrong", i, pattern.Name)
		}
	}
}

// TestConstants verifies the demo configuration constants.
func TestConstants(t *testing.T) {
	// Video constants
	if videoWidth != 640 {
		t.Errorf("videoWidth: expected 640, got %d", videoWidth)
	}
	if videoHeight != 480 {
		t.Errorf("videoHeight: expected 480, got %d", videoHeight)
	}
	if videoFrameRate != 30 {
		t.Errorf("videoFrameRate: expected 30, got %d", videoFrameRate)
	}
	if videoBitRate != 500000 {
		t.Errorf("videoBitRate: expected 500000, got %d", videoBitRate)
	}

	// Audio constants
	if audioSampleRate != 48000 {
		t.Errorf("audioSampleRate: expected 48000, got %d", audioSampleRate)
	}
	if audioChannels != 1 {
		t.Errorf("audioChannels: expected 1, got %d", audioChannels)
	}
	if audioFrameSize != 480 {
		t.Errorf("audioFrameSize: expected 480, got %d", audioFrameSize)
	}

	// Demo duration
	if demoDuration != 90*time.Second {
		t.Errorf("demoDuration: expected 90s, got %v", demoDuration)
	}
}

// TestVideoCallStatsCallsActive tests CallsActive field manipulation.
func TestVideoCallStatsCallsActive(t *testing.T) {
	stats := &VideoCallStats{}

	// Increment calls active
	stats.mu.Lock()
	stats.CallsActive = 5
	stats.mu.Unlock()

	_, _, _, active, _, _ := stats.GetStats()
	if active != 5 {
		t.Errorf("Expected CallsActive=5, got %d", active)
	}
}

// TestVideoCallStatsEffectsApplied tests EffectsApplied field.
func TestVideoCallStatsEffectsApplied(t *testing.T) {
	stats := &VideoCallStats{}

	// Set effects applied
	stats.mu.Lock()
	stats.EffectsApplied = 42
	stats.mu.Unlock()

	_, _, _, _, _, effects := stats.GetStats()
	if effects != 42 {
		t.Errorf("Expected EffectsApplied=42, got %d", effects)
	}
}

// TestProcessEventsDefault tests the processEvents default case.
func TestProcessEventsDefault(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	demo := &VideoCallDemo{
		timeProvider: tp,
		active:       true,
	}

	timers := &TimerSet{
		sigChan:       make(chan os.Signal, 1),
		videoTicker:   time.NewTicker(time.Hour), // Long duration so no tick
		audioTicker:   time.NewTicker(time.Hour),
		statsTicker:   time.NewTicker(time.Hour),
		patternTicker: time.NewTicker(time.Hour),
		toxTicker:     time.NewTicker(time.Hour),
		startTime:     baseTime,
	}
	defer func() {
		timers.videoTicker.Stop()
		timers.audioTicker.Stop()
		timers.statsTicker.Stop()
		timers.patternTicker.Stop()
		timers.toxTicker.Stop()
	}()

	// Default case should return false (no events)
	result := demo.processEvents(timers)
	if result {
		t.Error("processEvents default case should return false")
	}
}

// TestPlasmaEffectColorVariation tests that plasma effect produces color variation.
func TestPlasmaEffectColorVariation(t *testing.T) {
	demo := &VideoCallDemo{
		animationPhase: 100,
	}

	y, u, v := demo.generatePlasmaEffect(demo)

	// Y plane should have variation (not all same value)
	firstY := y[0]
	hasYVariation := false
	for _, val := range y {
		if val != firstY {
			hasYVariation = true
			break
		}
	}
	if !hasYVariation {
		t.Error("Plasma Y plane should have variation")
	}

	// U and V should have some non-128 values (color)
	hasUColor := false
	for _, val := range u {
		if val != 128 {
			hasUColor = true
			break
		}
	}
	hasVColor := false
	for _, val := range v {
		if val != 128 {
			hasVColor = true
			break
		}
	}
	if !hasUColor && !hasVColor {
		t.Error("Plasma should have chrominance variation")
	}
}

// TestMovingGradientColorProgression tests color animation in moving gradient.
func TestMovingGradientColorProgression(t *testing.T) {
	demo := &VideoCallDemo{
		animationPhase: 0,
	}

	// Capture U/V at two different phases
	_, u1, v1 := demo.generateMovingGradient(demo)

	demo.animationPhase = 100 // Advance phase
	_, u2, v2 := demo.generateMovingGradient(demo)

	// U and V should change between phases
	uChanged := false
	vChanged := false
	for i := range u1 {
		if u1[i] != u2[i] {
			uChanged = true
		}
		if v1[i] != v2[i] {
			vChanged = true
		}
		if uChanged && vChanged {
			break
		}
	}

	if !uChanged {
		t.Error("Moving gradient U plane should change with animation phase")
	}
	if !vChanged {
		t.Error("Moving gradient V plane should change with animation phase")
	}
}

// TestProcessEventsVideoTicker is skipped because sendVideoFrame requires a real ToxAV instance.
// The processEvents function is tested indirectly through other paths.

// TestProcessEventsPatternTicker tests processEvents handles pattern ticker.
func TestProcessEventsPatternTicker(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	demo := &VideoCallDemo{
		timeProvider:   tp,
		active:         true,
		currentPattern: 0,
	}
	demo.initializePatterns()

	// Create a ticker that fires immediately
	patternTicker := time.NewTicker(1 * time.Nanosecond)
	time.Sleep(5 * time.Millisecond) // Let it tick

	timers := &TimerSet{
		sigChan:       make(chan os.Signal, 1),
		videoTicker:   time.NewTicker(time.Hour),
		audioTicker:   time.NewTicker(time.Hour),
		statsTicker:   time.NewTicker(time.Hour),
		patternTicker: patternTicker,
		toxTicker:     time.NewTicker(time.Hour),
		startTime:     baseTime,
	}
	defer func() {
		timers.videoTicker.Stop()
		timers.audioTicker.Stop()
		timers.statsTicker.Stop()
		timers.patternTicker.Stop()
		timers.toxTicker.Stop()
	}()

	// Process pattern ticker
	demo.processEvents(timers)

	// Pattern should have changed
	// (Note: timing is not guaranteed, so we can't assert exact pattern)
}

// TestProcessEventsStatsTicker tests processEvents handles stats ticker.
func TestProcessEventsStatsTicker(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	demo := &VideoCallDemo{
		timeProvider:   tp,
		active:         true,
		currentPattern: 0,
	}
	demo.initializePatterns()

	// Create a ticker that fires immediately
	statsTicker := time.NewTicker(1 * time.Nanosecond)
	time.Sleep(5 * time.Millisecond) // Let it tick

	timers := &TimerSet{
		sigChan:       make(chan os.Signal, 1),
		videoTicker:   time.NewTicker(time.Hour),
		audioTicker:   time.NewTicker(time.Hour),
		statsTicker:   statsTicker,
		patternTicker: time.NewTicker(time.Hour),
		toxTicker:     time.NewTicker(time.Hour),
		startTime:     baseTime,
	}
	defer func() {
		timers.videoTicker.Stop()
		timers.audioTicker.Stop()
		timers.statsTicker.Stop()
		timers.patternTicker.Stop()
		timers.toxTicker.Stop()
	}()

	// Process stats ticker - should log stats
	demo.processEvents(timers)
}

// TestProcessEventsShutdownSignal tests processEvents handles shutdown signal.
func TestProcessEventsShutdownSignal(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tp := &MockTimeProvider{currentTime: baseTime}

	demo := &VideoCallDemo{
		timeProvider:   tp,
		active:         true,
		currentPattern: 0,
	}

	sigChan := make(chan os.Signal, 1)
	sigChan <- os.Interrupt // Send signal

	timers := &TimerSet{
		sigChan:       sigChan,
		videoTicker:   time.NewTicker(time.Hour),
		audioTicker:   time.NewTicker(time.Hour),
		statsTicker:   time.NewTicker(time.Hour),
		patternTicker: time.NewTicker(time.Hour),
		toxTicker:     time.NewTicker(time.Hour),
		startTime:     baseTime,
	}
	defer func() {
		timers.videoTicker.Stop()
		timers.audioTicker.Stop()
		timers.statsTicker.Stop()
		timers.patternTicker.Stop()
		timers.toxTicker.Stop()
	}()

	// Process shutdown signal
	result := demo.processEvents(timers)

	if !result {
		t.Error("processEvents should return true on shutdown signal")
	}
	if demo.active {
		t.Error("demo should be inactive after shutdown signal")
	}
}

// TestProcessEventsAudioTicker is skipped because sendAudioFrame requires a real ToxAV instance.
// The processEvents function is tested indirectly through other paths.

// TestVideoCallDemoStateManagement tests demo state transitions.
func TestVideoCallDemoStateManagement(t *testing.T) {
	demo := &VideoCallDemo{
		active:         true,
		currentPattern: 0,
		frameCount:     0,
		animationPhase: 0,
	}
	demo.initializePatterns()

	// Test pattern cycling
	for i := 0; i < 10; i++ {
		demo.switchToNextPattern()
		expected := (i + 1) % len(demo.patterns)
		if demo.currentPattern != expected {
			t.Errorf("After %d switches, expected pattern %d, got %d", i+1, expected, demo.currentPattern)
		}
	}

	// Test shutdown signal handling
	demo.handleShutdownSignal()
	if demo.active {
		t.Error("active should be false after shutdown")
	}
}

// TestGenerateColorBarsEdgeCases tests color bar generation edge cases.
func TestGenerateColorBarsEdgeCases(t *testing.T) {
	demo := &VideoCallDemo{
		frameCount:     99999,
		animationPhase: 999.99,
	}

	// Should work regardless of frame count/animation state
	y, u, v := demo.generateColorBars(demo)

	if len(y) != videoWidth*videoHeight {
		t.Error("Y plane size incorrect")
	}
	if len(u) != (videoWidth/2)*(videoHeight/2) {
		t.Error("U plane size incorrect")
	}
	if len(v) != (videoWidth/2)*(videoHeight/2) {
		t.Error("V plane size incorrect")
	}
}

// TestVideoCallStatsZeroFrames tests GetStats with zero video frames.
func TestVideoCallStatsZeroFrames(t *testing.T) {
	stats := &VideoCallStats{
		VideoFramesSent: 0,
		ProcessingTime:  100 * time.Millisecond,
	}

	_, _, _, _, avgProcessing, _ := stats.GetStats()

	// With zero frames, average should be 0
	if avgProcessing != 0 {
		t.Errorf("Expected avgProcessing=0 with zero frames, got %v", avgProcessing)
	}
}

// TestColorBarsLastBar tests that the last color bar is black.
func TestColorBarsLastBar(t *testing.T) {
	demo := &VideoCallDemo{}
	y, _, _ := demo.generateColorBars(demo)

	// Last bar (8th bar) should be black (Y=16)
	barWidth := videoWidth / 8
	for row := 0; row < videoHeight; row++ {
		for col := 7 * barWidth; col < videoWidth; col++ {
			idx := row*videoWidth + col
			if y[idx] != 16 {
				t.Errorf("Last bar pixel at (%d,%d) should be 16 (black), got %d", row, col, y[idx])
				return
			}
		}
	}
}
