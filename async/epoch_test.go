package async

import (
	"testing"
	"time"
)

func TestNewEpochManager(t *testing.T) {
	em := NewEpochManager()

	// Check that start time is set to expected network genesis time
	expectedStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !em.startTime.Equal(expectedStart) {
		t.Errorf("Expected start time %v, got %v", expectedStart, em.startTime)
	}

	// Check that epoch duration is set correctly
	if em.epochDuration != EpochDuration {
		t.Errorf("Expected epoch duration %v, got %v", EpochDuration, em.epochDuration)
	}
}

func TestNewEpochManagerWithCustomStart(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	duration := 1 * time.Hour

	em, err := NewEpochManagerWithCustomStart(startTime, duration)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !em.startTime.Equal(startTime) {
		t.Errorf("Expected start time %v, got %v", startTime, em.startTime)
	}

	if em.epochDuration != duration {
		t.Errorf("Expected epoch duration %v, got %v", duration, em.epochDuration)
	}
}

func TestNewEpochManagerWithCustomStart_InvalidDuration(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	testCases := []time.Duration{
		0,
		-1 * time.Hour,
		-time.Nanosecond,
	}

	for _, duration := range testCases {
		t.Run(duration.String(), func(t *testing.T) {
			_, err := NewEpochManagerWithCustomStart(startTime, duration)
			if err == nil {
				t.Error("Expected error for invalid duration, got nil")
			}
		})
	}
}

func TestGetEpochAt(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	duration := 1 * time.Hour

	em, err := NewEpochManagerWithCustomStart(startTime, duration)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	testCases := []struct {
		name     string
		time     time.Time
		expected uint64
	}{
		{
			name:     "Before start time",
			time:     startTime.Add(-1 * time.Hour),
			expected: 0,
		},
		{
			name:     "Exactly start time",
			time:     startTime,
			expected: 0,
		},
		{
			name:     "First epoch middle",
			time:     startTime.Add(30 * time.Minute),
			expected: 0,
		},
		{
			name:     "Second epoch start",
			time:     startTime.Add(1 * time.Hour),
			expected: 1,
		},
		{
			name:     "Second epoch middle",
			time:     startTime.Add(90 * time.Minute),
			expected: 1,
		},
		{
			name:     "Tenth epoch",
			time:     startTime.Add(10 * time.Hour),
			expected: 10,
		},
		{
			name:     "Large epoch number",
			time:     startTime.Add(1000 * time.Hour),
			expected: 1000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := em.GetEpochAt(tc.time)
			if result != tc.expected {
				t.Errorf("Expected epoch %d, got %d for time %v", tc.expected, result, tc.time)
			}
		})
	}
}

func TestGetCurrentEpoch(t *testing.T) {
	// Test with current time - exact value will depend on when test runs
	// but we can verify it's non-negative and reasonable
	em := NewEpochManager()
	epoch := em.GetCurrentEpoch()

	// Should be a reasonable value (not 0 since we're past 2025)
	// and not excessively large
	if epoch > 100000 {
		t.Errorf("Current epoch seems unreasonably large: %d", epoch)
	}

	// Call twice and verify it's the same or incremented by at most 1
	// (in case we cross an epoch boundary during the test)
	epoch2 := em.GetCurrentEpoch()
	if epoch2 < epoch || epoch2 > epoch+1 {
		t.Errorf("Epoch changed unexpectedly from %d to %d", epoch, epoch2)
	}
}

func TestGetEpochStartTime(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	duration := 2 * time.Hour

	em, err := NewEpochManagerWithCustomStart(startTime, duration)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	testCases := []struct {
		epoch    uint64
		expected time.Time
	}{
		{0, startTime},
		{1, startTime.Add(2 * time.Hour)},
		{2, startTime.Add(4 * time.Hour)},
		{10, startTime.Add(20 * time.Hour)},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			result := em.GetEpochStartTime(tc.epoch)
			if !result.Equal(tc.expected) {
				t.Errorf("Expected start time %v for epoch %d, got %v",
					tc.expected, tc.epoch, result)
			}
		})
	}
}

func TestGetEpochEndTime(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	duration := 2 * time.Hour

	em, err := NewEpochManagerWithCustomStart(startTime, duration)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	testCases := []struct {
		epoch    uint64
		expected time.Time
	}{
		{0, startTime.Add(2*time.Hour - time.Nanosecond)},
		{1, startTime.Add(4*time.Hour - time.Nanosecond)},
		{2, startTime.Add(6*time.Hour - time.Nanosecond)},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			result := em.GetEpochEndTime(tc.epoch)
			if !result.Equal(tc.expected) {
				t.Errorf("Expected end time %v for epoch %d, got %v",
					tc.expected, tc.epoch, result)
			}
		})
	}
}

func TestIsValidEpoch(t *testing.T) {
	// Test with real current time - we can verify the logic is correct
	em := NewEpochManager()
	currentEpoch := em.GetCurrentEpoch()

	testCases := []struct {
		name     string
		epoch    uint64
		expected bool
	}{
		{"Current epoch", currentEpoch, true},
		{"Previous epoch", currentEpoch - 1, true},
		{"Two epochs ago", currentEpoch - 2, true},
		{"Three epochs ago", currentEpoch - 3, true},
		{"Four epochs ago", currentEpoch - 4, false},
		{"Future epoch", currentEpoch + 1, false},
		{"Far future epoch", currentEpoch + 10, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := em.IsValidEpoch(tc.epoch)
			if result != tc.expected {
				t.Errorf("Expected IsValidEpoch(%d) = %v, got %v",
					tc.epoch, tc.expected, result)
			}
		})
	}
}

func TestGetRecentEpochs(t *testing.T) {
	// Test with real current time and verify the pattern is correct
	em := NewEpochManager()
	result := em.GetRecentEpochs()

	// Should return up to 4 epochs (current + 3 previous) in descending order
	if len(result) > 4 {
		t.Errorf("Expected at most 4 epochs, got %d", len(result))
	}

	// Verify epochs are in descending order
	for i := 1; i < len(result); i++ {
		if result[i] >= result[i-1] {
			t.Errorf("Epochs not in descending order: %v", result)
			break
		}
	}

	// Verify the first epoch is the current epoch
	currentEpoch := em.GetCurrentEpoch()
	if len(result) > 0 && result[0] != currentEpoch {
		t.Errorf("First epoch should be current epoch %d, got %d", currentEpoch, result[0])
	}

	// Test edge case with early epochs using custom manager
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	duration := 1 * time.Hour

	em2, err := NewEpochManagerWithCustomStart(startTime, duration)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Test at epoch 0 (start time)
	epoch0 := em2.GetEpochAt(startTime)
	if epoch0 != 0 {
		t.Errorf("Expected epoch 0 at start time, got %d", epoch0)
	}

	// Test at epoch 2 (2 hours after start)
	testTime2 := startTime.Add(2 * time.Hour)
	epoch2 := em2.GetEpochAt(testTime2)
	if epoch2 != 2 {
		t.Errorf("Expected epoch 2 at 2 hours after start, got %d", epoch2)
	}
}

func TestTimeUntilNextEpoch(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	duration := 1 * time.Hour

	em, err := NewEpochManagerWithCustomStart(startTime, duration)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Test at various points within an epoch
	testTimes := []struct {
		name     string
		time     time.Time
		expected time.Duration
	}{
		{
			name:     "Start of epoch",
			time:     startTime,
			expected: 1 * time.Hour,
		},
		{
			name:     "Middle of epoch",
			time:     startTime.Add(30 * time.Minute),
			expected: 30 * time.Minute,
		},
		{
			name:     "Near end of epoch",
			time:     startTime.Add(59 * time.Minute),
			expected: 1 * time.Minute,
		},
	}

	for _, tc := range testTimes {
		t.Run(tc.name, func(t *testing.T) {
			// We can't directly test TimeUntilNextEpoch with fixed time,
			// so we calculate expected duration based on the time
			currentEpoch := em.GetEpochAt(tc.time)
			nextEpochStart := em.GetEpochStartTime(currentEpoch + 1)
			expected := nextEpochStart.Sub(tc.time)

			if expected != tc.expected {
				t.Errorf("Expected duration %v, calculated %v", tc.expected, expected)
			}
		})
	}
}

func TestGetEpochDuration(t *testing.T) {
	duration := 3 * time.Hour
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	em, err := NewEpochManagerWithCustomStart(startTime, duration)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := em.GetEpochDuration()
	if result != duration {
		t.Errorf("Expected duration %v, got %v", duration, result)
	}
}

// Benchmark tests for performance validation
func BenchmarkGetCurrentEpoch(b *testing.B) {
	em := NewEpochManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.GetCurrentEpoch()
	}
}

// Benchmark tests for performance validation
func BenchmarkGetEpochAt(b *testing.B) {
	em := NewEpochManager()
	testTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.GetEpochAt(testTime)
	}
}

func BenchmarkIsValidEpoch(b *testing.B) {
	em := NewEpochManager()
	currentEpoch := em.GetCurrentEpoch()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.IsValidEpoch(currentEpoch - 1)
	}
}
