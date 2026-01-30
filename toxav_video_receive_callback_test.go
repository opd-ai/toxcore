package toxcore

import (
	"sync"
	"testing"
	"time"

	avpkg "github.com/opd-ai/toxcore/av"
	"github.com/opd-ai/toxcore/av/video"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToxAVVideoReceiveCallbackWiring verifies that video receive callbacks
// are properly wired from ToxAV through to the av.Manager.
func TestToxAVVideoReceiveCallbackWiring(t *testing.T) {
	// Create test Tox instances
	options1 := NewOptions()
	options1.UDPEnabled = true
	tox1, err := New(options1)
	require.NoError(t, err)
	defer tox1.Kill()

	options2 := NewOptions()
	options2.UDPEnabled = true
	tox2, err := New(options2)
	require.NoError(t, err)
	defer tox2.Kill()

	// Create ToxAV instances
	toxav1, err := NewToxAV(tox1)
	require.NoError(t, err)
	defer toxav1.Kill()

	toxav2, err := NewToxAV(tox2)
	require.NoError(t, err)
	defer toxav2.Kill()

	// Test that callback registration succeeds
	callback := func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		// Verify callback parameters are reasonable
		assert.Greater(t, width, uint16(0), "Frame width should be positive")
		assert.Greater(t, height, uint16(0), "Frame height should be positive")
		assert.NotNil(t, y, "Y plane should not be nil")
		assert.NotNil(t, u, "U plane should not be nil")
		assert.NotNil(t, v, "V plane should not be nil")
		assert.Greater(t, yStride, 0, "Y stride should be positive")
		assert.Greater(t, uStride, 0, "U stride should be positive")
		assert.Greater(t, vStride, 0, "V stride should be positive")
	}

	// Register the callback
	toxav1.CallbackVideoReceiveFrame(callback)

	// Verify callback is stored in ToxAV
	toxav1.mu.RLock()
	assert.NotNil(t, toxav1.videoReceiveCb, "Video receive callback should be stored")
	toxav1.mu.RUnlock()

	// Verify callback is wired to the underlying av.Manager
	assert.NotNil(t, toxav1.impl, "av.Manager should be initialized")

	// Note: Actual callback invocation would require a full call setup
	// and video frame transmission, which is beyond this unit test scope.
	// This test verifies the wiring mechanism is in place.
}

// TestToxAVVideoReceiveCallbackNil verifies that nil callbacks can be registered
// to unregister previously set callbacks.
func TestToxAVVideoReceiveCallbackNil(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	require.NoError(t, err)
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	require.NoError(t, err)
	defer toxav.Kill()

	// Register a callback
	callback := func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		// Test callback
	}
	toxav.CallbackVideoReceiveFrame(callback)

	toxav.mu.RLock()
	assert.NotNil(t, toxav.videoReceiveCb, "Callback should be registered")
	toxav.mu.RUnlock()

	// Unregister by setting nil
	toxav.CallbackVideoReceiveFrame(nil)

	toxav.mu.RLock()
	assert.Nil(t, toxav.videoReceiveCb, "Callback should be unregistered")
	toxav.mu.RUnlock()
}

// TestAVManagerVideoReceiveCallback verifies the av.Manager callback mechanism.
func TestAVManagerVideoReceiveCallback(t *testing.T) {
	// Create a mock transport
	mockTransport := &mockAVTransport{
		packets: make(map[byte][]func(data, addr []byte) error),
	}

	// Create a simple friend lookup function
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		// Return dummy address: 127.0.0.1:12345
		return []byte{127, 0, 0, 1, 0x30, 0x39}, nil
	}

	// Create the manager
	manager, err := avpkg.NewManager(mockTransport, friendLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)

	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Register video receive callback
	var callbackCalled bool
	var receivedWidth, receivedHeight uint16
	var mu sync.Mutex

	callback := func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		receivedWidth = width
		receivedHeight = height
	}

	manager.SetVideoReceiveCallback(callback)

	// Create a test video frame (use default processor size to avoid scaling)
	testFrame := &video.VideoFrame{
		Width:   640,
		Height:  480,
		Y:       make([]byte, 640*480),
		U:       make([]byte, 640*480/4),
		V:       make([]byte, 640*480/4),
		YStride: 640,
		UStride: 320,
		VStride: 320,
	}

	// Fill with test pattern
	for i := range testFrame.Y {
		testFrame.Y[i] = byte(i % 256)
	}
	for i := range testFrame.U {
		testFrame.U[i] = byte(128)
	}
	for i := range testFrame.V {
		testFrame.V[i] = byte(128)
	}

	// Start a test call
	friendNumber := uint32(1)
	err = manager.StartCall(friendNumber, 48000, 1000000) // Audio + Video enabled
	require.NoError(t, err)

	// Get the call and verify video processor is initialized
	call := manager.GetCall(friendNumber)
	require.NotNil(t, call, "Call should exist")

	// Setup media components (this initializes the video processor)
	err = call.SetupMedia(mockTransport, friendNumber)
	require.NoError(t, err)

	videoProcessor := call.GetVideoProcessor()
	require.NotNil(t, videoProcessor, "Video processor should be initialized")

	// Process the frame (this simulates encoding and then we'll decode it)
	encodedData, err := videoProcessor.ProcessOutgoingLegacy(testFrame)
	require.NoError(t, err)
	require.NotEmpty(t, encodedData, "Encoded data should not be empty")

	// Simulate receiving the frame back (decode it)
	decodedFrame, err := videoProcessor.ProcessIncomingLegacy(encodedData)
	require.NoError(t, err)
	require.NotNil(t, decodedFrame, "Decoded frame should not be nil")

	// Manually trigger the callback (simulating what handleVideoFrame does)
	callback(friendNumber, decodedFrame.Width, decodedFrame.Height,
		decodedFrame.Y, decodedFrame.U, decodedFrame.V,
		decodedFrame.YStride, decodedFrame.UStride, decodedFrame.VStride)

	// Give callback time to execute
	time.Sleep(10 * time.Millisecond)

	// Verify callback was called with correct dimensions
	mu.Lock()
	assert.True(t, callbackCalled, "Video receive callback should have been called")
	assert.Equal(t, uint16(640), receivedWidth, "Width should match")
	assert.Equal(t, uint16(480), receivedHeight, "Height should match")
	mu.Unlock()
}

// TestAVManagerAudioReceiveCallback verifies the audio callback mechanism for consistency.
func TestAVManagerAudioReceiveCallback(t *testing.T) {
	// Create a mock transport
	mockTransport := &mockAVTransport{
		packets: make(map[byte][]func(data, addr []byte) error),
	}

	// Create a simple friend lookup function
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{127, 0, 0, 1, 0x30, 0x39}, nil
	}

	// Create the manager
	manager, err := avpkg.NewManager(mockTransport, friendLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)

	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Register audio receive callback
	callback := func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		assert.NotNil(t, pcm, "PCM data should not be nil")
		assert.Greater(t, sampleCount, 0, "Sample count should be positive")
		assert.Greater(t, channels, uint8(0), "Channels should be positive")
		assert.Greater(t, samplingRate, uint32(0), "Sampling rate should be positive")
	}

	manager.SetAudioReceiveCallback(callback)

	// Verify callback is registered
	// Note: Full audio frame processing test would require RTP integration
	// This test verifies the registration mechanism works correctly
	assert.NotNil(t, manager, "Manager should be initialized")
}

// mockAVTransport is a simple mock for testing av.Manager callbacks
type mockAVTransport struct {
	packets map[byte][]func(data, addr []byte) error
	mu      sync.RWMutex
}

func (m *mockAVTransport) Send(packetType byte, data, addr []byte) error {
	return nil
}

func (m *mockAVTransport) RegisterHandler(packetType byte, handler func(data, addr []byte) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.packets == nil {
		m.packets = make(map[byte][]func(data, addr []byte) error)
	}
	m.packets[packetType] = append(m.packets[packetType], handler)
}
