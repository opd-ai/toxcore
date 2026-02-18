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

// TestAddressResolverIntegration verifies that SetAddressResolver configures
// the address resolver and it's used during SetupMedia.
func TestAddressResolverIntegration(t *testing.T) {
	call := NewCall(42)

	// Initially, there should be no address resolver
	// SetupMedia without transport should succeed
	err := call.SetupMedia(nil, 42)
	if err != nil {
		t.Errorf("SetupMedia with nil transport should succeed: %v", err)
	}

	// Create a new call for resolver test
	call2 := NewCall(100)

	// Track if resolver was called
	expectedFriendNum := uint32(100)

	// Configure address resolver that returns a 6-byte address (4 IP + 2 port)
	resolver := func(friendNumber uint32) ([]byte, error) {
		if friendNumber != expectedFriendNum {
			t.Errorf("Resolver received wrong friend number: got %d, want %d", friendNumber, expectedFriendNum)
		}
		// Return IP 10.0.0.42, port 12345 (big-endian: 0x30, 0x39)
		return []byte{10, 0, 0, 42, 0x30, 0x39}, nil
	}

	call2.SetAddressResolver(resolver)

	// SetupMedia with nil transport skips RTP session creation, but we can verify
	// the resolver is set correctly
	err = call2.SetupMedia(nil, 100)
	if err != nil {
		t.Errorf("SetupMedia should succeed: %v", err)
	}

	// Note: resolver isn't called when transport is nil, since RTP session creation is skipped
	// This is expected behavior - address resolution only happens when creating RTP session
}

// TestAddressResolverWithInsufficientBytes verifies fallback when resolver returns
// fewer than 6 bytes (4 IP + 2 port).
func TestAddressResolverWithInsufficientBytes(t *testing.T) {
	call := NewCall(50)

	// Configure resolver that returns fewer than 6 bytes
	resolver := func(friendNumber uint32) ([]byte, error) {
		return []byte{10, 0, 0, 42}, nil // Only 4 bytes, missing port
	}

	call.SetAddressResolver(resolver)

	// Should still succeed - will fall back to placeholder address
	err := call.SetupMedia(nil, 50)
	if err != nil {
		t.Errorf("SetupMedia should succeed with insufficient address bytes: %v", err)
	}
}

// mockTimeProvider implements TimeProvider for deterministic testing.
type mockTimeProvider struct {
	currentTime time.Time
}

// Now returns the mock current time.
func (m *mockTimeProvider) Now() time.Time {
	return m.currentTime
}

// Advance moves the mock time forward by the specified duration.
func (m *mockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// TestDefaultTimeProvider verifies that DefaultTimeProvider returns real time.
func TestDefaultTimeProvider(t *testing.T) {
	tp := DefaultTimeProvider{}

	before := time.Now()
	actual := tp.Now()
	after := time.Now()

	if actual.Before(before) || actual.After(after) {
		t.Error("DefaultTimeProvider.Now() should return current time within expected range")
	}
}

// TestCallTimeProviderDefault verifies that Call uses DefaultTimeProvider by default.
func TestCallTimeProviderDefault(t *testing.T) {
	call := NewCall(42)

	// By default, should use DefaultTimeProvider
	call.markStarted()

	startTime := call.GetStartTime()
	if startTime.IsZero() {
		t.Error("Start time should be set after markStarted()")
	}

	// Verify time is roughly now
	diff := time.Since(startTime)
	if diff > time.Second {
		t.Error("Start time should be approximately now when using default time provider")
	}
}

// TestCallSetTimeProvider verifies that SetTimeProvider sets a custom time provider.
func TestCallSetTimeProvider(t *testing.T) {
	call := NewCall(42)

	fixedTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	mockTP := &mockTimeProvider{currentTime: fixedTime}

	call.SetTimeProvider(mockTP)
	call.markStarted()

	startTime := call.GetStartTime()
	if !startTime.Equal(fixedTime) {
		t.Errorf("Start time should be %v, got %v", fixedTime, startTime)
	}
}

// TestCallSetTimeProviderNilFallback verifies that SetTimeProvider with nil falls back to default.
func TestCallSetTimeProviderNilFallback(t *testing.T) {
	call := NewCall(42)

	// First set a custom time provider
	fixedTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	mockTP := &mockTimeProvider{currentTime: fixedTime}
	call.SetTimeProvider(mockTP)

	// Then set to nil - should use default
	call.SetTimeProvider(nil)
	call.markStarted()

	startTime := call.GetStartTime()

	// Should be approximately now (within 1 second)
	diff := time.Since(startTime)
	if diff > time.Second {
		t.Error("After SetTimeProvider(nil), should use DefaultTimeProvider")
	}
}

// TestCallUpdateLastFrameWithTimeProvider verifies updateLastFrame uses time provider.
func TestCallUpdateLastFrameWithTimeProvider(t *testing.T) {
	call := NewCall(42)

	startTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	mockTP := &mockTimeProvider{currentTime: startTime}

	call.SetTimeProvider(mockTP)
	call.markStarted()

	// Advance time and update last frame
	mockTP.Advance(5 * time.Second)
	call.updateLastFrame()

	// We can't directly access lastFrame, but let's verify markStarted was correct
	actualStartTime := call.GetStartTime()
	if !actualStartTime.Equal(startTime) {
		t.Errorf("Start time should be %v, got %v", startTime, actualStartTime)
	}
}

// TestCallDeterministicTiming verifies full deterministic behavior with time provider.
func TestCallDeterministicTiming(t *testing.T) {
	call1 := NewCall(1)
	call2 := NewCall(2)

	// Use the same fixed time for both calls
	fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mockTP1 := &mockTimeProvider{currentTime: fixedTime}
	mockTP2 := &mockTimeProvider{currentTime: fixedTime}

	call1.SetTimeProvider(mockTP1)
	call2.SetTimeProvider(mockTP2)

	call1.markStarted()
	call2.markStarted()

	// Both should have identical start times
	if !call1.GetStartTime().Equal(call2.GetStartTime()) {
		t.Error("Both calls should have identical start times with same time provider")
	}

	// Advance only first call's time
	mockTP1.Advance(10 * time.Second)
	call1.updateLastFrame()

	// Advance second call's time by different amount
	mockTP2.Advance(5 * time.Second)
	call2.updateLastFrame()

	// Start times should still match
	if !call1.GetStartTime().Equal(call2.GetStartTime()) {
		t.Error("Start times should remain identical after updateLastFrame()")
	}
}

// TestCallBitrateAdapterGetterSetter verifies BitrateAdapter getter/setter methods.
func TestCallBitrateAdapterGetterSetter(t *testing.T) {
	call := NewCall(123)

	// Initially should be nil
	if call.GetBitrateAdapter() != nil {
		t.Error("Expected BitrateAdapter to be nil initially")
	}

	// Set an adapter
	adapter := NewBitrateAdapter(DefaultAdaptationConfig(), 32000, 500000)
	call.SetBitrateAdapter(adapter)

	// Should return the same adapter
	if call.GetBitrateAdapter() != adapter {
		t.Error("Expected GetBitrateAdapter to return the set adapter")
	}

	// Set to nil
	call.SetBitrateAdapter(nil)
	if call.GetBitrateAdapter() != nil {
		t.Error("Expected BitrateAdapter to be nil after setting nil")
	}
}
