package av

import (
	"testing"
	"time"
)

// TestManagerQualityMonitoringIntegration verifies quality monitoring integration.
func TestManagerQualityMonitoringIntegration(t *testing.T) {
	// Create mock transport
	transport := NewMockTransport()
	
	// Create friend lookup function
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{1, 2, 3, 4}, nil
	}
	
	// Create manager
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Stop()
	
	// Verify quality monitor is available
	qualityMonitor := manager.GetQualityMonitor()
	if qualityMonitor == nil {
		t.Fatal("Quality monitor should be available")
	}
	
	// Verify it's enabled by default
	if !qualityMonitor.IsEnabled() {
		t.Error("Quality monitor should be enabled by default")
	}
	
	// Test quality callback configuration
	callbackCalled := false
	var callbackFriendNumber uint32
	var callbackMetrics CallMetrics
	
	manager.SetQualityCallback(func(friendNumber uint32, metrics CallMetrics) {
		callbackCalled = true
		callbackFriendNumber = friendNumber
		callbackMetrics = metrics
	})
	
	// Start manager
	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	
	// Start a call
	friendNumber := uint32(123)
	err = manager.StartCall(friendNumber, 64000, 0)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}
	
	// Get the call to set it to an active state
	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call should exist after starting")
	}
	
	// Set call to sending audio state to make it "active"
	call.SetState(CallStateSendingAudio)
	
	// Run iteration to trigger quality monitoring
	manager.Iterate()
	
	// Small delay to allow async processing
	time.Sleep(10 * time.Millisecond)
	
	// Check if callback was called
	if !callbackCalled {
		t.Error("Quality callback should have been called during iteration")
	}
	
	if callbackFriendNumber != friendNumber {
		t.Errorf("Expected callback for friend %d, got %d", friendNumber, callbackFriendNumber)
	}
	
	if callbackMetrics.Timestamp.IsZero() {
		t.Error("Callback metrics should have valid timestamp")
	}
	
	// Verify call information in metrics
	if callbackMetrics.AudioBitRate == 0 {
		t.Error("Audio bitrate should be set in metrics")
	}
	
	// End the call
	err = manager.EndCall(friendNumber)
	if err != nil {
		t.Fatalf("Failed to end call: %v", err)
	}
}

// TestManagerQualityMonitorConfiguration verifies quality monitor configuration.
func TestManagerQualityMonitorConfiguration(t *testing.T) {
	// Create manager
	transport := NewMockTransport()
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{1, 2, 3, 4}, nil
	}
	
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Stop()
	
	qualityMonitor := manager.GetQualityMonitor()
	
	// Test enabling/disabling
	qualityMonitor.SetEnabled(false)
	if qualityMonitor.IsEnabled() {
		t.Error("Quality monitor should be disabled")
	}
	
	qualityMonitor.SetEnabled(true)
	if !qualityMonitor.IsEnabled() {
		t.Error("Quality monitor should be enabled")
	}
	
	// Test monitor interval configuration
	newInterval := 10 * time.Second
	qualityMonitor.SetMonitorInterval(newInterval)
	if qualityMonitor.GetMonitorInterval() != newInterval {
		t.Errorf("Expected interval %v, got %v", newInterval, qualityMonitor.GetMonitorInterval())
	}
}

// TestManagerQualityMonitoringWithInactiveCalls verifies monitoring behavior with inactive calls.
func TestManagerQualityMonitoringWithInactiveCalls(t *testing.T) {
	transport := NewMockTransport()
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{1, 2, 3, 4}, nil
	}
	
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Stop()
	
	// Track quality callback calls
	callbackCallCount := 0
	manager.SetQualityCallback(func(friendNumber uint32, metrics CallMetrics) {
		callbackCallCount++
	})
	
	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	
	// Start a call but keep it in initial state (inactive)
	friendNumber := uint32(456)
	err = manager.StartCall(friendNumber, 64000, 0)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}
	
	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call should exist")
	}
	
	// Ensure call is in None state (inactive)
	call.SetState(CallStateNone)
	
	// Run iteration - should not trigger quality monitoring for inactive calls
	manager.Iterate()
	
	// Small delay
	time.Sleep(10 * time.Millisecond)
	
	// Verify no quality callback was triggered for inactive call
	if callbackCallCount > 0 {
		t.Error("Quality callback should not be called for inactive calls")
	}
	
	// Now make the call active
	call.SetState(CallStateSendingAudio)
	
	// Run iteration again
	manager.Iterate()
	
	// Small delay
	time.Sleep(10 * time.Millisecond)
	
	// Now callback should have been called
	if callbackCallCount == 0 {
		t.Error("Quality callback should be called for active calls")
	}
}

// TestManagerQualityMonitoringDisabled verifies behavior when monitoring is disabled.
func TestManagerQualityMonitoringDisabled(t *testing.T) {
	transport := NewMockTransport()
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{1, 2, 3, 4}, nil
	}
	
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Stop()
	
	// Disable quality monitoring
	qualityMonitor := manager.GetQualityMonitor()
	qualityMonitor.SetEnabled(false)
	
	// Track quality callback calls
	callbackCallCount := 0
	manager.SetQualityCallback(func(friendNumber uint32, metrics CallMetrics) {
		callbackCallCount++
	})
	
	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	
	// Start an active call
	friendNumber := uint32(789)
	err = manager.StartCall(friendNumber, 64000, 0)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}
	
	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call should exist")
	}
	
	// Set call to active state
	call.SetState(CallStateSendingAudio)
	
	// Run iteration - should not trigger quality monitoring when disabled
	manager.Iterate()
	
	// Small delay
	time.Sleep(10 * time.Millisecond)
	
	// Verify no quality callback was triggered when monitoring disabled
	if callbackCallCount > 0 {
		t.Error("Quality callback should not be called when monitoring is disabled")
	}
}

// BenchmarkManagerIterationWithQualityMonitoring benchmarks manager iteration with quality monitoring.
func BenchmarkManagerIterationWithQualityMonitoring(b *testing.B) {
	transport := NewMockTransport()
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{1, 2, 3, 4}, nil
	}
	
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Stop()
	
	err = manager.Start()
	if err != nil {
		b.Fatalf("Failed to start manager: %v", err)
	}
	
	// Start multiple calls
	for i := 0; i < 5; i++ {
		friendNumber := uint32(i + 1)
		err = manager.StartCall(friendNumber, 64000, 0)
		if err != nil {
			b.Fatalf("Failed to start call %d: %v", i, err)
		}
		
		call := manager.GetCall(friendNumber)
		if call != nil {
			call.SetState(CallStateSendingAudio) // Make it active
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Iterate()
	}
}
