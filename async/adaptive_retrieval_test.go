package async

import (
	"testing"
	"time"
)

// TestAdaptiveRetrievalInterval tests that the interval adapts based on activity
func TestAdaptiveRetrievalInterval(t *testing.T) {
	// Create a standalone scheduler for testing
	scheduler := &RetrievalScheduler{
		baseInterval:     1 * time.Minute,
		jitterPercent:    20,
		consecutiveEmpty: 0,
	}

	// Get baseline interval
	baselineInterval := scheduler.calculateNextInterval()

	// Set consecutive empty count to simulate inactivity
	scheduler.consecutiveEmpty = 5

	// Get adapted interval
	adaptedInterval := scheduler.calculateNextInterval()

	// Verify adapted interval is longer than baseline
	if adaptedInterval <= baselineInterval {
		t.Errorf("Adaptive interval should increase after inactivity: got %v, expected > %v",
			adaptedInterval, baselineInterval)
	} else {
		t.Logf("Interval correctly adapted: %v → %v", baselineInterval, adaptedInterval)
	}

	// Test extreme inactivity
	scheduler.consecutiveEmpty = 10
	longInterval := scheduler.calculateNextInterval()

	// The multiplier caps at 4x (for consecutiveEmpty=10) vs 3x (for consecutiveEmpty=5).
	// Both intervals include ±20% random jitter, so their ranges can overlap.
	// Verify the long interval is within the expected 4x range instead of
	// requiring strict ordering, which is non-deterministic due to jitter.
	maxExpected := time.Duration(float64(scheduler.baseInterval) * 4 * 1.25) // 4x + 25% margin
	if longInterval > maxExpected {
		t.Errorf("Long interval exceeded expected range: got %v, expected <= %v",
			longInterval, maxExpected)
	}
	minExpected := time.Duration(float64(scheduler.baseInterval) * 4 * 0.75) // 4x - 25% margin
	if longInterval < minExpected {
		t.Errorf("Long interval below expected range: got %v, expected >= %v",
			longInterval, minExpected)
	}

	// Reset when activity resumes
	scheduler.consecutiveEmpty = 0
	resetInterval := scheduler.calculateNextInterval()

	// Should be back to baseline range
	if resetInterval > 2*baselineInterval {
		t.Errorf("Reset interval should be back to baseline range: got %v, expected ~%v",
			resetInterval, baselineInterval)
	}
}
