package av

import (
	"testing"
	"time"
)

// TestNewManager verifies that NewManager creates a manager with correct initial state.
func TestNewManager(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	if manager.IsRunning() {
		t.Error("Manager should not be running initially")
	}

	if len(manager.GetActiveCalls()) != 0 {
		t.Error("Manager should have no active calls initially")
	}

	// Test iteration interval
	interval := manager.IterationInterval()
	expectedInterval := 20 * time.Millisecond
	if interval != expectedInterval {
		t.Errorf("Expected iteration interval %v, got %v", expectedInterval, interval)
	}
}

// TestManagerLifecycle verifies manager start/stop lifecycle.
func TestManagerLifecycle(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test starting
	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	if !manager.IsRunning() {
		t.Error("Manager should be running after Start()")
	}

	// Test starting when already running
	err = manager.Start()
	if err == nil {
		t.Error("Expected error when starting already running manager")
	}

	// Test stopping
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	if manager.IsRunning() {
		t.Error("Manager should not be running after Stop()")
	}

	// Test stopping when already stopped
	err = manager.Stop()
	if err != nil {
		t.Errorf("Unexpected error when stopping already stopped manager: %v", err)
	}
}

// TestManagerCallManagement verifies call management functionality.
func TestManagerCallManagement(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(123)
	audioBitRate := uint32(64000)
	videoBitRate := uint32(1000000)

	// Test starting a call
	err = manager.StartCall(friendNumber, audioBitRate, videoBitRate)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Verify call exists
	activeCalls := manager.GetActiveCalls()
	if len(activeCalls) != 1 {
		t.Errorf("Expected 1 active call, got %d", len(activeCalls))
	}
	if activeCalls[0] != friendNumber {
		t.Errorf("Expected active call with friend %d, got %d", friendNumber, activeCalls[0])
	}

	// Get call details
	call, exists := manager.GetCall(friendNumber)
	if !exists {
		t.Error("Call should exist")
	}
	if call.GetFriendNumber() != friendNumber {
		t.Errorf("Expected friend number %d, got %d", friendNumber, call.GetFriendNumber())
	}
	if call.GetAudioBitRate() != audioBitRate {
		t.Errorf("Expected audio bit rate %d, got %d", audioBitRate, call.GetAudioBitRate())
	}
	if call.GetVideoBitRate() != videoBitRate {
		t.Errorf("Expected video bit rate %d, got %d", videoBitRate, call.GetVideoBitRate())
	}

	// Test ending the call
	err = manager.EndCall(friendNumber)
	if err != nil {
		t.Fatalf("Failed to end call: %v", err)
	}

	// Verify call no longer exists
	activeCalls = manager.GetActiveCalls()
	if len(activeCalls) != 0 {
		t.Errorf("Expected 0 active calls after ending, got %d", len(activeCalls))
	}

	_, exists = manager.GetCall(friendNumber)
	if exists {
		t.Error("Call should not exist after ending")
	}
}

// TestManagerCallValidation verifies call validation logic.
func TestManagerCallValidation(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(456)

	// Test starting call with no audio or video
	err = manager.StartCall(friendNumber, 0, 0)
	if err == nil {
		t.Error("Expected error when starting call with no audio or video")
	}

	// Test starting valid call
	err = manager.StartCall(friendNumber, 64000, 0)
	if err != nil {
		t.Fatalf("Failed to start valid call: %v", err)
	}

	// Test starting duplicate call
	err = manager.StartCall(friendNumber, 32000, 0)
	if err == nil {
		t.Error("Expected error when starting duplicate call")
	}

	// Clean up
	manager.EndCall(friendNumber)
}

// TestManagerAnswerCall verifies call answering functionality.
func TestManagerAnswerCall(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(789)

	// Test answering non-existent call
	err = manager.AnswerCall(friendNumber, 64000, 0)
	if err == nil {
		t.Error("Expected error when answering non-existent call")
	}

	// Create a call in the correct state for answering
	// In a real implementation, this would be created by receiving an incoming call
	call := NewCall(friendNumber)
	call.SetState(CallStateNone) // This represents an incoming call state
	manager.calls[friendNumber] = call

	// Test answering with invalid bit rates
	err = manager.AnswerCall(friendNumber, 0, 0)
	if err == nil {
		t.Error("Expected error when answering call with no audio or video")
	}

	// Test valid answer
	err = manager.AnswerCall(friendNumber, 64000, 1000000)
	if err != nil {
		t.Fatalf("Failed to answer call: %v", err)
	}

	// Verify call configuration
	answeredCall, exists := manager.GetCall(friendNumber)
	if !exists {
		t.Error("Call should exist after answering")
	}
	if !answeredCall.IsAudioEnabled() {
		t.Error("Audio should be enabled after answering")
	}
	if !answeredCall.IsVideoEnabled() {
		t.Error("Video should be enabled after answering")
	}
}

// TestManagerBitRateManagement verifies bit rate management.
func TestManagerBitRateManagement(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(321)

	// Test setting bit rates for non-existent call
	err = manager.SetAudioBitRate(friendNumber, 32000)
	if err == nil {
		t.Error("Expected error when setting audio bit rate for non-existent call")
	}

	err = manager.SetVideoBitRate(friendNumber, 500000)
	if err == nil {
		t.Error("Expected error when setting video bit rate for non-existent call")
	}

	// Start a call
	err = manager.StartCall(friendNumber, 64000, 1000000)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Test setting bit rates
	err = manager.SetAudioBitRate(friendNumber, 32000)
	if err != nil {
		t.Fatalf("Failed to set audio bit rate: %v", err)
	}

	err = manager.SetVideoBitRate(friendNumber, 500000)
	if err != nil {
		t.Fatalf("Failed to set video bit rate: %v", err)
	}

	// Verify bit rates were updated
	call, _ := manager.GetCall(friendNumber)
	if call.GetAudioBitRate() != 32000 {
		t.Errorf("Expected audio bit rate 32000, got %d", call.GetAudioBitRate())
	}
	if call.GetVideoBitRate() != 500000 {
		t.Errorf("Expected video bit rate 500000, got %d", call.GetVideoBitRate())
	}
}

// TestManagerIteration verifies iteration functionality.
func TestManagerIteration(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Test iteration with no calls
	manager.Iterate() // Should not panic

	// Start a call
	friendNumber := uint32(654)
	err = manager.StartCall(friendNumber, 64000, 0)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Test iteration with active call
	manager.Iterate() // Should not panic

	// Set call to error state to test cleanup
	call, _ := manager.GetCall(friendNumber)
	call.SetState(CallStateError)

	// Iterate should clean up the failed call
	manager.Iterate()

	// Verify call was cleaned up
	activeCalls := manager.GetActiveCalls()
	if len(activeCalls) != 0 {
		t.Errorf("Expected 0 active calls after error cleanup, got %d", len(activeCalls))
	}
}

// TestManagerStoppedOperations verifies behavior when manager is stopped.
func TestManagerStoppedOperations(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	friendNumber := uint32(987)

	// Test operations when manager is not running
	err = manager.StartCall(friendNumber, 64000, 0)
	if err == nil {
		t.Error("Expected error when starting call with stopped manager")
	}

	err = manager.AnswerCall(friendNumber, 64000, 0)
	if err == nil {
		t.Error("Expected error when answering call with stopped manager")
	}

	// Test iteration with stopped manager
	manager.Iterate() // Should not panic
}

// TestManagerStopCleansUpCalls verifies that stopping cleans up all calls.
func TestManagerStopCleansUpCalls(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Start multiple calls
	friends := []uint32{100, 200, 300}
	for _, friend := range friends {
		err = manager.StartCall(friend, 64000, 0)
		if err != nil {
			t.Fatalf("Failed to start call with friend %d: %v", friend, err)
		}
	}

	// Verify calls exist
	activeCalls := manager.GetActiveCalls()
	if len(activeCalls) != len(friends) {
		t.Errorf("Expected %d active calls, got %d", len(friends), len(activeCalls))
	}

	// Stop manager
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	// Verify all calls were cleaned up
	activeCalls = manager.GetActiveCalls()
	if len(activeCalls) != 0 {
		t.Errorf("Expected 0 active calls after stop, got %d", len(activeCalls))
	}

	// Verify individual calls have finished state
	for _, friend := range friends {
		call, exists := manager.GetCall(friend)
		if exists && call.GetState() != CallStateFinished {
			t.Errorf("Expected call with friend %d to be finished, got state %d", friend, call.GetState())
		}
	}
}
