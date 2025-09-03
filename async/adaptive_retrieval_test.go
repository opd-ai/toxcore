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
	
	// Should be longer but still have an upper bound
	if longInterval <= adaptedInterval {
		t.Errorf("Long interval should be longer than adapted: got %v, expected > %v",
			longInterval, adaptedInterval)
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
