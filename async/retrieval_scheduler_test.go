package async

import (
	"testing"
	"time"
)

func TestRetrievalScheduler(t *testing.T) {
	// Create a mock AsyncClient
	mockClient := &AsyncClient{}
	
	// Create a retrieval scheduler
	scheduler := NewRetrievalScheduler(mockClient)
	
	// Test configuration
	scheduler.Configure(2*time.Minute, 30, true, 0.5)
	
	if scheduler.baseInterval != 2*time.Minute {
		t.Errorf("Expected base interval 2m, got %v", scheduler.baseInterval)
	}
	
	if scheduler.jitterPercent != 30 {
		t.Errorf("Expected jitter percent 30, got %d", scheduler.jitterPercent)
	}
	
	if !scheduler.coverTrafficEnabled {
		t.Error("Cover traffic should be enabled")
	}
	
	if scheduler.coverTrafficRatio != 0.5 {
		t.Errorf("Expected cover traffic ratio 0.5, got %f", scheduler.coverTrafficRatio)
	}
	
	// Test interval calculation (should be within bounds)
	// Run multiple times to account for randomization
	const iterations = 100
	const minExpected = time.Minute      // 2m - 50% jitter
	const maxExpected = 3 * time.Minute  // 2m + 50% jitter
	
	for i := 0; i < iterations; i++ {
		interval := scheduler.calculateNextInterval()
		
		if interval < minExpected || interval > maxExpected {
			t.Errorf("Interval %v outside expected range [%v, %v]", 
				interval, minExpected, maxExpected)
		}
	}
	
	// Test backoff behavior
	scheduler.consecutiveEmpty = 5 // Simulate 5 consecutive empty retrievals
	interval := scheduler.calculateNextInterval()
	
	// Should be larger than base due to backoff
	if interval <= scheduler.baseInterval {
		t.Errorf("Expected increased interval due to backoff, got %v", interval)
	}
}

func TestCoverTrafficRatio(t *testing.T) {
	// Create a mock AsyncClient
	mockClient := &AsyncClient{}
	
	// Create a retrieval scheduler
	scheduler := NewRetrievalScheduler(mockClient)
	
	// Configure for testing
	scheduler.Configure(time.Minute, 0, true, 0.5) // 50% cover traffic
	
	// Test the ratio by sampling many decisions
	const iterations = 1000
	coverCount := 0
	
	for i := 0; i < iterations; i++ {
		if scheduler.shouldSendCoverTraffic() {
			coverCount++
		}
	}
	
	// Check the ratio is roughly what we expected (with some tolerance)
	ratio := float64(coverCount) / float64(iterations)
	
	// Given 1000 samples, we can expect to be within ±5% of the target ratio
	if ratio < 0.45 || ratio > 0.55 {
		t.Errorf("Cover traffic ratio %f significantly differs from expected 0.5", ratio)
	}
	
	// Test disabling cover traffic
	scheduler.SetCoverTrafficEnabled(false)
	
	coverCount = 0
	for i := 0; i < 100; i++ {
		if scheduler.shouldSendCoverTraffic() {
			coverCount++
		}
	}
	
	if coverCount > 0 {
		t.Errorf("Expected 0 cover traffic when disabled, got %d", coverCount)
	}
}
