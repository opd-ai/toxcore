package av

import (
	"testing"
	"time"
)

// TestNewCall verifies that NewCall creates a call with correct initial state.
func TestNewCall(t *testing.T) {
	friendNumber := uint32(42)
	call := NewCall(friendNumber)

	if call.GetFriendNumber() != friendNumber {
		t.Errorf("Expected friend number %d, got %d", friendNumber, call.GetFriendNumber())
	}

	if call.GetState() != CallStateNone {
		t.Errorf("Expected initial state CallStateNone, got %d", call.GetState())
	}

	if call.IsAudioEnabled() {
		t.Error("Expected audio to be disabled initially")
	}

	if call.IsVideoEnabled() {
		t.Error("Expected video to be disabled initially")
	}

	if call.GetAudioBitRate() != 0 {
		t.Error("Expected initial audio bit rate to be 0")
	}

	if call.GetVideoBitRate() != 0 {
		t.Error("Expected initial video bit rate to be 0")
	}
}

// TestCallStateTransitions verifies that call state changes work correctly.
func TestCallStateTransitions(t *testing.T) {
	call := NewCall(123)

	// Test state transitions
	states := []CallState{
		CallStateSendingAudio,
		CallStateSendingVideo,
		CallStateFinished,
		CallStateError,
	}

	for _, state := range states {
		call.SetState(state)
		if call.GetState() != state {
			t.Errorf("Expected state %d, got %d", state, call.GetState())
		}
	}
}

// TestCallBitRateManagement verifies bit rate management functionality.
func TestCallBitRateManagement(t *testing.T) {
	call := NewCall(456)

	// Test individual bit rate setting
	call.SetAudioBitRate(64000)
	if call.GetAudioBitRate() != 64000 {
		t.Errorf("Expected audio bit rate 64000, got %d", call.GetAudioBitRate())
	}

	call.SetVideoBitRate(1000000)
	if call.GetVideoBitRate() != 1000000 {
		t.Errorf("Expected video bit rate 1000000, got %d", call.GetVideoBitRate())
	}

	// Test atomic bit rate setting
	call.setBitRates(32000, 500000)
	if call.GetAudioBitRate() != 32000 {
		t.Errorf("Expected audio bit rate 32000, got %d", call.GetAudioBitRate())
	}
	if call.GetVideoBitRate() != 500000 {
		t.Errorf("Expected video bit rate 500000, got %d", call.GetVideoBitRate())
	}
}

// TestCallEnabledStatus verifies audio/video enabled status management.
func TestCallEnabledStatus(t *testing.T) {
	call := NewCall(789)

	// Test setting enabled status
	call.setEnabled(true, false)
	if !call.IsAudioEnabled() {
		t.Error("Expected audio to be enabled")
	}
	if call.IsVideoEnabled() {
		t.Error("Expected video to be disabled")
	}

	call.setEnabled(false, true)
	if call.IsAudioEnabled() {
		t.Error("Expected audio to be disabled")
	}
	if !call.IsVideoEnabled() {
		t.Error("Expected video to be enabled")
	}

	call.setEnabled(true, true)
	if !call.IsAudioEnabled() {
		t.Error("Expected audio to be enabled")
	}
	if !call.IsVideoEnabled() {
		t.Error("Expected video to be enabled")
	}
}

// TestCallTiming verifies timing functionality.
func TestCallTiming(t *testing.T) {
	call := NewCall(101)

	// Initially, start time should be zero
	if !call.GetStartTime().IsZero() {
		t.Error("Expected start time to be zero initially")
	}

	// Mark as started
	before := time.Now()
	call.markStarted()
	after := time.Now()

	startTime := call.GetStartTime()
	if startTime.Before(before) || startTime.After(after) {
		t.Error("Start time should be between before and after timestamps")
	}

	// Update last frame
	before = time.Now()
	call.updateLastFrame()
	after = time.Now()

	// We can't directly access lastFrame, but we can verify the call was successful
	// by ensuring no panic occurred and the method completed
}

// TestCallControlConstants verifies that call control constants have expected values.
func TestCallControlConstants(t *testing.T) {
	// Test that constants are defined and have unique values
	controls := map[CallControl]string{
		CallControlResume:      "Resume",
		CallControlPause:       "Pause",
		CallControlCancel:      "Cancel",
		CallControlMuteAudio:   "MuteAudio",
		CallControlUnmuteAudio: "UnmuteAudio",
		CallControlHideVideo:   "HideVideo",
		CallControlShowVideo:   "ShowVideo",
	}

	seen := make(map[CallControl]bool)
	for control, name := range controls {
		if seen[control] {
			t.Errorf("Duplicate value for CallControl: %s", name)
		}
		seen[control] = true
	}

	// Verify specific values that match libtoxcore
	if CallControlResume != 0 {
		t.Errorf("CallControlResume should be 0, got %d", CallControlResume)
	}
	if CallControlPause != 1 {
		t.Errorf("CallControlPause should be 1, got %d", CallControlPause)
	}
	if CallControlCancel != 2 {
		t.Errorf("CallControlCancel should be 2, got %d", CallControlCancel)
	}
}

// TestCallStateConstants verifies that call state constants have expected values.
func TestCallStateConstants(t *testing.T) {
	// Test that constants are defined and have unique values
	states := map[CallState]string{
		CallStateNone:           "None",
		CallStateError:          "Error",
		CallStateFinished:       "Finished",
		CallStateSendingAudio:   "SendingAudio",
		CallStateSendingVideo:   "SendingVideo",
		CallStateAcceptingAudio: "AcceptingAudio",
		CallStateAcceptingVideo: "AcceptingVideo",
	}

	seen := make(map[CallState]bool)
	for state, name := range states {
		if seen[state] {
			t.Errorf("Duplicate value for CallState: %s", name)
		}
		seen[state] = true
	}

	// Verify specific values that match libtoxcore
	if CallStateNone != 0 {
		t.Errorf("CallStateNone should be 0, got %d", CallStateNone)
	}
	if CallStateError != 1 {
		t.Errorf("CallStateError should be 1, got %d", CallStateError)
	}
}

// TestCallThreadSafety verifies thread safety of call operations.
func TestCallThreadSafety(t *testing.T) {
	call := NewCall(999)

	// Run concurrent operations to test for race conditions
	done := make(chan bool, 4)

	// Goroutine 1: State changes
	go func() {
		for i := 0; i < 100; i++ {
			call.SetState(CallStateSendingAudio)
			call.SetState(CallStateNone)
		}
		done <- true
	}()

	// Goroutine 2: Bit rate changes
	go func() {
		for i := 0; i < 100; i++ {
			call.SetAudioBitRate(64000)
			call.SetVideoBitRate(1000000)
		}
		done <- true
	}()

	// Goroutine 3: Reading state
	go func() {
		for i := 0; i < 100; i++ {
			_ = call.GetState()
			_ = call.GetAudioBitRate()
			_ = call.GetVideoBitRate()
		}
		done <- true
	}()

	// Goroutine 4: Timing updates
	go func() {
		for i := 0; i < 100; i++ {
			call.updateLastFrame()
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 4; i++ {
		<-done
	}

	// If we reach here without race conditions, the test passes
}
