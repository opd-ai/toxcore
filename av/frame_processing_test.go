package av

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAudioFrameProcessing verifies audio frame handling through the manager.
func TestAudioFrameProcessing(t *testing.T) {
	// Create mock transport
	mockTransport := NewMockTransport()

	// Create manager
	manager, err := NewManager(mockTransport, func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 192, 168, 1, 100, 0}, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	friendNumber := uint32(1)

	// Create a call
	call := NewCall(friendNumber)
	call.markStarted()
	call.setEnabled(true, false)

	// Create a mock transport wrapper for RTP session
	// The RTP session needs a different transport interface (transport.Transport)
	// For this test, we skip RTP session initialization and just test the frame routing
	call.rtpSession = nil // Don't initialize for this test

	manager.mu.Lock()
	manager.calls[friendNumber] = call
	manager.mu.Unlock()

	// Create a valid RTP audio packet
	audioPacket := createValidAudioRTPPacket(t)

	// Simulate receiving an audio frame
	addr := []byte{byte(friendNumber), 192, 168, 1, 100, 0}
	err = manager.handleAudioFrame(audioPacket, addr)
	assert.NoError(t, err)

	// Verify last frame time was updated
	lastFrame := call.GetLastFrameTime()
	assert.False(t, lastFrame.IsZero(), "Last frame time should be updated")
	assert.WithinDuration(t, time.Now(), lastFrame, time.Second)
}

// TestVideoFrameProcessing verifies video frame handling through the manager.
func TestVideoFrameProcessing(t *testing.T) {
	// Create mock transport
	mockTransport := NewMockTransport()

	// Create manager
	manager, err := NewManager(mockTransport, func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 192, 168, 1, 100, 0}, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	friendNumber := uint32(2)

	// Create a call
	call := NewCall(friendNumber)
	call.markStarted()
	call.setEnabled(true, true)

	// Skip RTP session initialization for this test
	call.rtpSession = nil

	manager.mu.Lock()
	manager.calls[friendNumber] = call
	manager.mu.Unlock()

	// Create a valid RTP video packet
	videoPacket := createValidVideoRTPPacket(t)

	// Simulate receiving a video frame
	addr := []byte{byte(friendNumber), 192, 168, 1, 100, 0}
	err = manager.handleVideoFrame(videoPacket, addr)
	assert.NoError(t, err)

	// Verify last frame time was updated
	lastFrame := call.GetLastFrameTime()
	assert.False(t, lastFrame.IsZero(), "Last frame time should be updated")
	assert.WithinDuration(t, time.Now(), lastFrame, time.Second)
}

// TestFrameProcessingWithoutRTPSession verifies graceful handling when RTP session is not initialized.
func TestFrameProcessingWithoutRTPSession(t *testing.T) {
	// Create mock transport
	mockTransport := NewMockTransport()

	// Create manager
	manager, err := NewManager(mockTransport, func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 192, 168, 1, 100, 0}, nil
	})
	assert.NoError(t, err)

	friendNumber := uint32(3)

	// Create a call without RTP session
	call := NewCall(friendNumber)
	call.markStarted()
	call.setEnabled(true, true)
	// Don't set rtpSession

	manager.mu.Lock()
	manager.calls[friendNumber] = call
	manager.mu.Unlock()

	// Create packets
	audioPacket := createValidAudioRTPPacket(t)
	videoPacket := createValidVideoRTPPacket(t)

	addr := []byte{byte(friendNumber), 192, 168, 1, 100, 0}

	// Should not error even without RTP session
	err = manager.handleAudioFrame(audioPacket, addr)
	assert.NoError(t, err)

	err = manager.handleVideoFrame(videoPacket, addr)
	assert.NoError(t, err)

	// Last frame should still be updated
	lastFrame := call.GetLastFrameTime()
	assert.False(t, lastFrame.IsZero())
}

// TestFrameProcessingUnknownFriend verifies error handling for frames from unknown friends.
func TestFrameProcessingUnknownFriend(t *testing.T) {
	// Create mock transport
	mockTransport := NewMockTransport()

	// Create manager
	manager, err := NewManager(mockTransport, func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 192, 168, 1, 100, 0}, nil
	})
	assert.NoError(t, err)

	// Create packets
	audioPacket := createValidAudioRTPPacket(t)
	videoPacket := createValidVideoRTPPacket(t)

	// Use address with zero byte (maps to friend 0, which is invalid)
	addr := []byte{0, 0, 0, 0}

	// Should error for unknown friend
	err = manager.handleAudioFrame(audioPacket, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown friend")

	err = manager.handleVideoFrame(videoPacket, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown friend")
}

// TestFrameProcessingNoActiveCall verifies error handling when no call exists.
func TestFrameProcessingNoActiveCall(t *testing.T) {
	// Create mock transport
	mockTransport := NewMockTransport()

	// Create manager
	manager, err := NewManager(mockTransport, func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 192, 168, 1, 100, 0}, nil
	})
	assert.NoError(t, err)

	// Create packets
	audioPacket := createValidAudioRTPPacket(t)
	videoPacket := createValidVideoRTPPacket(t)

	// Use valid address but no call exists
	addr := []byte{5, 192, 168, 1, 100, 0}

	// Should error for non-existent call
	err = manager.handleAudioFrame(audioPacket, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active call")

	err = manager.handleVideoFrame(videoPacket, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active call")
}

// TestCallTimeout verifies call timeout detection and handling.
func TestCallTimeout(t *testing.T) {
	// Create mock transport
	mockTransport := NewMockTransport()

	// Create manager
	manager, err := NewManager(mockTransport, func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 192, 168, 1, 100, 0}, nil
	})
	assert.NoError(t, err)

	friendNumber := uint32(10)

	// Create a call and mark it as started
	call := NewCall(friendNumber)
	call.markStarted()
	call.SetState(CallStateSendingAudio)

	// Manually set last frame time to past the timeout threshold
	call.mu.Lock()
	call.lastFrame = time.Now().Add(-CallTimeout - time.Second)
	call.mu.Unlock()

	manager.mu.Lock()
	manager.calls[friendNumber] = call
	manager.mu.Unlock()

	// Process the call - should detect timeout
	manager.processCall(call)

	// Verify call state changed to finished
	assert.Equal(t, CallStateFinished, call.GetState())

	// Verify call was removed from active calls
	manager.mu.RLock()
	_, exists := manager.calls[friendNumber]
	manager.mu.RUnlock()
	assert.False(t, exists, "Timed out call should be removed")
}

// TestCallTimeoutNotTriggeredForRecentFrames verifies timeout doesn't trigger for active calls.
func TestCallTimeoutNotTriggeredForRecentFrames(t *testing.T) {
	// Create mock transport
	mockTransport := NewMockTransport()

	// Create manager
	manager, err := NewManager(mockTransport, func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 192, 168, 1, 100, 0}, nil
	})
	assert.NoError(t, err)

	friendNumber := uint32(11)

	// Create a call with recent frame
	call := NewCall(friendNumber)
	call.markStarted()
	call.SetState(CallStateSendingAudio)
	call.updateLastFrame() // Set to now

	manager.mu.Lock()
	manager.calls[friendNumber] = call
	manager.mu.Unlock()

	// Process the call - should NOT timeout
	manager.processCall(call)

	// Verify call state is still active
	assert.Equal(t, CallStateSendingAudio, call.GetState())

	// Verify call still exists
	manager.mu.RLock()
	_, exists := manager.calls[friendNumber]
	manager.mu.RUnlock()
	assert.True(t, exists, "Active call should not be removed")
}

// TestCallTimeoutNotTriggeredForNonStartedCalls verifies timeout only applies to started calls.
func TestCallTimeoutNotTriggeredForNonStartedCalls(t *testing.T) {
	// Create mock transport
	mockTransport := NewMockTransport()

	// Create manager
	manager, err := NewManager(mockTransport, func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 192, 168, 1, 100, 0}, nil
	})
	assert.NoError(t, err)

	friendNumber := uint32(12)

	// Create a call but don't mark it as started (startTime is zero)
	call := NewCall(friendNumber)
	call.SetState(CallStateSendingAudio)
	// Don't call markStarted()

	manager.mu.Lock()
	manager.calls[friendNumber] = call
	manager.mu.Unlock()

	// Process the call - should NOT timeout since it wasn't started
	manager.processCall(call)

	// Verify call state unchanged
	assert.Equal(t, CallStateSendingAudio, call.GetState())

	// Verify call still exists
	manager.mu.RLock()
	_, exists := manager.calls[friendNumber]
	manager.mu.RUnlock()
	assert.True(t, exists)
}

// TestFrameHandlersRegistered verifies audio and video frame handlers are registered.
func TestFrameHandlersRegistered(t *testing.T) {
	// Create mock transport
	mockTransport := NewMockTransport()

	// Create manager - this calls registerPacketHandlers
	manager, err := NewManager(mockTransport, func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 192, 168, 1, 100, 0}, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	// Verify audio frame handler is registered (0x33 = PacketAVAudioFrame)
	audioHandler, exists := mockTransport.handlers[0x33]
	assert.True(t, exists, "Audio frame handler should be registered")
	assert.NotNil(t, audioHandler)

	// Verify video frame handler is registered (0x34 = PacketAVVideoFrame)
	videoHandler, exists := mockTransport.handlers[0x34]
	assert.True(t, exists, "Video frame handler should be registered")
	assert.NotNil(t, videoHandler)
}

// Helper functions to create valid RTP packets for testing

func createValidAudioRTPPacket(t *testing.T) []byte {
	// RTP header (12 bytes)
	packet := make([]byte, 32)
	packet[0] = 0x80 // V=2, P=0, X=0, CC=0
	packet[1] = 0x60 // M=0, PT=96 (Opus)
	// Sequence number
	packet[2] = 0x00
	packet[3] = 0x01
	// Timestamp
	packet[4] = 0x00
	packet[5] = 0x00
	packet[6] = 0x00
	packet[7] = 0x64
	// SSRC
	packet[8] = 0x12
	packet[9] = 0x34
	packet[10] = 0x56
	packet[11] = 0x78
	// Opus payload (dummy data)
	for i := 12; i < 32; i++ {
		packet[i] = byte(i)
	}
	return packet
}

func createValidVideoRTPPacket(t *testing.T) []byte {
	// RTP header (12 bytes) + VP8 payload descriptor (3 bytes) + payload
	packet := make([]byte, 50)
	packet[0] = 0x80 // V=2, P=0, X=0, CC=0
	packet[1] = 0xE0 // M=1 (marker), PT=96 (VP8)
	// Sequence number
	packet[2] = 0x00
	packet[3] = 0x01
	// Timestamp
	packet[4] = 0x00
	packet[5] = 0x00
	packet[6] = 0x00
	packet[7] = 0x64
	// SSRC
	packet[8] = 0x12
	packet[9] = 0x34
	packet[10] = 0x56
	packet[11] = 0x78
	// VP8 Payload Descriptor (3 bytes)
	packet[12] = 0x90 // X=1, S=1 (start of partition)
	packet[13] = 0x80 // I=1 (picture ID)
	packet[14] = 0x01 // Picture ID
	// VP8 payload (dummy data)
	for i := 15; i < 50; i++ {
		packet[i] = byte(i)
	}
	return packet
}
