package av

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTimeoutTransport implements TransportInterface for testing timeout functionality
type MockTimeoutTransport struct {
	packets []mockTimeoutPacket
}

type mockTimeoutPacket struct {
	packetType byte
	data       []byte
	addr       []byte
}

func (m *MockTimeoutTransport) Send(packetType byte, data, addr []byte) error {
	m.packets = append(m.packets, mockTimeoutPacket{
		packetType: packetType,
		data:       data,
		addr:       addr,
	})
	return nil
}

func (m *MockTimeoutTransport) RegisterHandler(packetType byte, handler func(data, addr []byte) error) {
	// Mock implementation - handlers not needed for timeout tests
}

func mockTimeoutFriendAddressLookup(friendNumber uint32) ([]byte, error) {
	// Return a simple address based on friend number
	return []byte{byte(friendNumber), 0, 0, 0}, nil
}

// TestManagerCallTimeoutConfiguration tests timeout configuration methods
func TestManagerCallTimeoutConfiguration(t *testing.T) {
	transport := &MockTimeoutTransport{}
	manager, err := NewManager(transport, mockTimeoutFriendAddressLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)
	
	// Test default timeout
	defaultTimeout := manager.GetCallTimeout()
	assert.Equal(t, 30*time.Second, defaultTimeout, "Default timeout should be 30 seconds")
	
	// Test setting valid timeout
	err = manager.SetCallTimeout(60 * time.Second)
	assert.NoError(t, err)
	assert.Equal(t, 60*time.Second, manager.GetCallTimeout())
	
	// Test setting another valid timeout
	err = manager.SetCallTimeout(5 * time.Second)
	assert.NoError(t, err)
	assert.Equal(t, 5*time.Second, manager.GetCallTimeout())
	
	// Test setting invalid timeout (zero)
	err = manager.SetCallTimeout(0)
	assert.Error(t, err)
	assert.Equal(t, 5*time.Second, manager.GetCallTimeout(), "Timeout should not change on error")
	
	// Test setting invalid timeout (negative)
	err = manager.SetCallTimeout(-1 * time.Second)
	assert.Error(t, err)
	assert.Equal(t, 5*time.Second, manager.GetCallTimeout(), "Timeout should not change on error")
}

// TestManagerCallTimeoutCallback tests timeout callback configuration
func TestManagerCallTimeoutCallback(t *testing.T) {
	transport := &MockTimeoutTransport{}
	manager, err := NewManager(transport, mockTimeoutFriendAddressLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)
	
	// Test setting callback
	callbackCalled := false
	var callbackFriendNumber uint32
	
	manager.SetCallTimeoutCallback(func(friendNumber uint32) {
		callbackCalled = true
		callbackFriendNumber = friendNumber
	})
	
	// Start manager
	err = manager.Start()
	require.NoError(t, err)
	
	// Create a call with short timeout
	err = manager.SetCallTimeout(100 * time.Millisecond)
	require.NoError(t, err)
	
	// Start a call
	err = manager.StartCall(1, 48000, 0)
	require.NoError(t, err)
	
	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	
	// Process the call to trigger timeout detection
	manager.Iterate()
	
	// Give callback time to execute (it runs in goroutine)
	time.Sleep(50 * time.Millisecond)
	
	// Verify callback was called
	assert.True(t, callbackCalled, "Timeout callback should have been called")
	assert.Equal(t, uint32(1), callbackFriendNumber, "Callback should receive correct friend number")
	
	// Verify call was removed
	assert.Equal(t, 0, manager.GetCallCount(), "Timed-out call should be removed")
	
	// Stop manager
	err = manager.Stop()
	assert.NoError(t, err)
}

// TestCheckCallTimeout tests the timeout detection logic
func TestCheckCallTimeout(t *testing.T) {
	transport := &MockTimeoutTransport{}
	manager, err := NewManager(transport, mockTimeoutFriendAddressLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)
	
	// Set short timeout for testing
	err = manager.SetCallTimeout(100 * time.Millisecond)
	require.NoError(t, err)
	
	// Create a call
	call := NewCall(1)
	call.SetState(CallStateSendingAudio)
	call.markStarted()
	
	// Test 1: Fresh call should not timeout
	timedOut := manager.checkCallTimeout(call)
	assert.False(t, timedOut, "Fresh call should not timeout")
	
	// Test 2: Call after short delay should not timeout
	time.Sleep(50 * time.Millisecond)
	timedOut = manager.checkCallTimeout(call)
	assert.False(t, timedOut, "Call within timeout period should not timeout")
	
	// Test 3: Call after timeout should timeout
	time.Sleep(60 * time.Millisecond) // Total 110ms > 100ms timeout
	timedOut = manager.checkCallTimeout(call)
	assert.True(t, timedOut, "Call after timeout period should timeout")
	
	// Test 4: Nil call should not timeout
	timedOut = manager.checkCallTimeout(nil)
	assert.False(t, timedOut, "Nil call should not timeout")
	
	// Test 5: Finished call should not timeout
	finishedCall := NewCall(2)
	finishedCall.SetState(CallStateFinished)
	finishedCall.markStarted()
	time.Sleep(150 * time.Millisecond)
	timedOut = manager.checkCallTimeout(finishedCall)
	assert.False(t, timedOut, "Finished call should not timeout")
	
	// Test 6: Error call should not timeout
	errorCall := NewCall(3)
	errorCall.SetState(CallStateError)
	errorCall.markStarted()
	time.Sleep(150 * time.Millisecond)
	timedOut = manager.checkCallTimeout(errorCall)
	assert.False(t, timedOut, "Error call should not timeout")
}

// TestCallTimeoutWithFrameActivity tests that frame activity prevents timeout
func TestCallTimeoutWithFrameActivity(t *testing.T) {
	transport := &MockTimeoutTransport{}
	manager, err := NewManager(transport, mockTimeoutFriendAddressLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)
	
	// Set short timeout for testing
	err = manager.SetCallTimeout(150 * time.Millisecond)
	require.NoError(t, err)
	
	// Create a call
	call := NewCall(1)
	call.SetState(CallStateSendingAudio)
	call.markStarted()
	
	// Wait almost to timeout
	time.Sleep(100 * time.Millisecond)
	
	// Update frame activity
	call.updateLastFrame()
	
	// Wait longer (would have timed out without update)
	time.Sleep(100 * time.Millisecond)
	
	// Should not timeout due to recent frame activity
	timedOut := manager.checkCallTimeout(call)
	assert.False(t, timedOut, "Call with recent frame activity should not timeout")
	
	// Wait for actual timeout from last frame
	time.Sleep(60 * time.Millisecond)
	
	// Now it should timeout
	timedOut = manager.checkCallTimeout(call)
	assert.True(t, timedOut, "Call should timeout after inactivity")
}

// TestTimeoutIntegration tests complete timeout flow with manager iteration
func TestTimeoutIntegration(t *testing.T) {
	transport := &MockTimeoutTransport{}
	manager, err := NewManager(transport, mockTimeoutFriendAddressLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)
	
	// Start manager
	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	// Set very short timeout for testing
	err = manager.SetCallTimeout(100 * time.Millisecond)
	require.NoError(t, err)
	
	// Track timeout events
	timeoutEvents := make([]uint32, 0)
	manager.SetCallTimeoutCallback(func(friendNumber uint32) {
		timeoutEvents = append(timeoutEvents, friendNumber)
	})
	
	// Start multiple calls
	err = manager.StartCall(1, 48000, 0) // Audio only
	require.NoError(t, err)
	
	err = manager.StartCall(2, 48000, 500000) // Audio + video
	require.NoError(t, err)
	
	// Verify calls exist
	assert.Equal(t, 2, manager.GetCallCount())
	
	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	
	// Iterate to process timeouts
	manager.Iterate()
	
	// Give callbacks time to execute
	time.Sleep(50 * time.Millisecond)
	
	// Verify both calls timed out
	assert.Equal(t, 0, manager.GetCallCount(), "All calls should be removed after timeout")
	assert.Equal(t, 2, len(timeoutEvents), "Should have 2 timeout events")
	
	// Verify the friend numbers in timeout events (order may vary)
	assert.Contains(t, timeoutEvents, uint32(1))
	assert.Contains(t, timeoutEvents, uint32(2))
}

// TestTimeoutWithNilCallback tests that timeout works without callback configured
func TestTimeoutWithNilCallback(t *testing.T) {
	transport := &MockTimeoutTransport{}
	manager, err := NewManager(transport, mockTimeoutFriendAddressLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)
	
	// Start manager
	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	// Set short timeout
	err = manager.SetCallTimeout(100 * time.Millisecond)
	require.NoError(t, err)
	
	// Explicitly set nil callback
	manager.SetCallTimeoutCallback(nil)
	
	// Start a call
	err = manager.StartCall(1, 48000, 0)
	require.NoError(t, err)
	
	assert.Equal(t, 1, manager.GetCallCount())
	
	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	
	// Iterate to process timeout - should not panic without callback
	assert.NotPanics(t, func() {
		manager.Iterate()
	})
	
	// Verify call was removed
	assert.Equal(t, 0, manager.GetCallCount())
}

// TestTimeoutDoesNotAffectActiveState tests that non-active states don't timeout
func TestTimeoutDoesNotAffectActiveState(t *testing.T) {
	transport := &MockTimeoutTransport{}
	manager, err := NewManager(transport, mockTimeoutFriendAddressLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)
	
	// Set short timeout
	err = manager.SetCallTimeout(50 * time.Millisecond)
	require.NoError(t, err)
	
	// Test each non-active state
	testCases := []struct {
		name  string
		state CallState
	}{
		{"None state", CallStateNone},
		{"Error state", CallStateError},
		{"Finished state", CallStateFinished},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			call := NewCall(1)
			call.SetState(tc.state)
			call.markStarted()
			
			// Wait beyond timeout
			time.Sleep(100 * time.Millisecond)
			
			// Should not timeout
			timedOut := manager.checkCallTimeout(call)
			assert.False(t, timedOut, "%s should not timeout", tc.name)
		})
	}
}

// TestTimeoutWithDifferentActiveStates tests timeout with various active call states
func TestTimeoutWithDifferentActiveStates(t *testing.T) {
	transport := &MockTimeoutTransport{}
	manager, err := NewManager(transport, mockTimeoutFriendAddressLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)
	
	// Set short timeout
	err = manager.SetCallTimeout(50 * time.Millisecond)
	require.NoError(t, err)
	
	// Test each active state
	testCases := []struct {
		name  string
		state CallState
	}{
		{"SendingAudio", CallStateSendingAudio},
		{"SendingVideo", CallStateSendingVideo},
		{"AcceptingAudio", CallStateAcceptingAudio},
		{"AcceptingVideo", CallStateAcceptingVideo},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			call := NewCall(1)
			call.SetState(tc.state)
			call.markStarted()
			
			// Wait beyond timeout
			time.Sleep(100 * time.Millisecond)
			
			// Should timeout
			timedOut := manager.checkCallTimeout(call)
			assert.True(t, timedOut, "%s should timeout after inactivity", tc.name)
		})
	}
}
