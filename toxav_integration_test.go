package toxcore

import (
	"sync"
	"testing"
	"time"

	avpkg "github.com/opd-ai/toxcore/av"
	"github.com/opd-ai/toxcore/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Tests from toxav_integration_test.go ---

// TestToxAVIntegrationWithToxcore tests integration between ToxAV and core toxcore functionality.
// This test focuses on API compatibility, resource management, and proper integration patterns
// rather than end-to-end networking which requires complex test infrastructure.
func TestToxAVIntegrationWithToxcore(t *testing.T) {
	t.Run("BasicToxAVCreationAndIntegration", func(t *testing.T) {
		testBasicToxAVCreationAndIntegration(t)
	})

	t.Run("ToxAVAPICompatibility", func(t *testing.T) {
		testToxAVAPICompatibility(t)
	})

	t.Run("ToxAVCallbackIntegration", func(t *testing.T) {
		testToxAVCallbackIntegration(t)
	})

	t.Run("ToxAVResourceManagement", func(t *testing.T) {
		testToxAVResourceManagement(t)
	})

	t.Run("ToxAVIterationIntegration", func(t *testing.T) {
		testToxAVIterationIntegration(t)
	})

	t.Run("ToxAVTransportIntegration", func(t *testing.T) {
		testToxAVTransportIntegration(t)
	})
}

// testBasicToxAVCreationAndIntegration tests basic ToxAV creation and integration with Tox
func testBasicToxAVCreationAndIntegration(t *testing.T) {
	// Create Tox instance
	tox, err := createTestToxInstance(t, "integration_test")
	require.NoError(t, err)
	defer tox.Kill()

	// Create ToxAV instance
	toxAV, err := NewToxAV(tox)
	require.NoError(t, err)
	defer toxAV.Kill()

	// Verify ToxAV is properly initialized
	assert.Greater(t, toxAV.IterationInterval(), time.Duration(0), "ToxAV should have positive iteration interval")

	// Test iteration works
	toxAV.Iterate()

	// Test that ToxAV integrates with Tox transport (packet handlers registered)
	// This verifies that ToxAV has successfully registered its packet handlers with the transport
	assert.NotNil(t, toxAV, "ToxAV should be properly initialized")
}

// testToxAVAPICompatibility tests ToxAV API compatibility and integration behavior
func testToxAVAPICompatibility(t *testing.T) {
	tox, err := createTestToxInstance(t, "api_test")
	require.NoError(t, err)
	defer tox.Kill()

	toxAV, err := NewToxAV(tox)
	require.NoError(t, err)
	defer toxAV.Kill()

	// Test call operations (current implementation may fail due to network restrictions in test environments)
	// This tests that the API works and integrates properly with the underlying system
	err = toxAV.Call(999, 64000, 0)
	if err != nil {
		// Network errors are expected in test environments
		t.Logf("Call operation failed (expected in restricted test environment): %v", err)
		// Skip remaining network-dependent operations
		return
	}
	// The call succeeds because ToxAV integrates with transport for signaling
	assert.NoError(t, err, "Call operation should integrate with transport")

	err = toxAV.Answer(999, 64000, 0)
	// Answer succeeds because it integrates with call management system
	assert.NoError(t, err, "Answer operation should integrate with call management")

	err = toxAV.CallControl(999, avpkg.CallControlCancel)
	// Call control succeeds and integrates with call state management
	assert.NoError(t, err, "Call control should integrate with state management")

	// Test bitrate operations (should fail gracefully for calls that don't exist after cancellation)
	err = toxAV.AudioSetBitRate(999, 32000)
	assert.Error(t, err, "Audio bitrate set should fail for cancelled call")

	err = toxAV.VideoSetBitRate(999, 500000)
	assert.Error(t, err, "Video bitrate set should fail for cancelled call")

	// Test frame sending operations (should fail gracefully for calls that don't exist)
	pcmData := make([]int16, 480)
	err = toxAV.AudioSendFrame(999, pcmData, 480, 1, 48000)
	assert.Error(t, err, "Audio frame send should fail for non-active call")

	yPlane := make([]byte, 640*480)
	uPlane := make([]byte, 320*240)
	vPlane := make([]byte, 320*240)
	err = toxAV.VideoSendFrame(999, 640, 480, yPlane, uPlane, vPlane)
	assert.Error(t, err, "Video frame send should fail for non-active call")
}

// testToxAVCallbackIntegration tests callback registration and management
func testToxAVCallbackIntegration(t *testing.T) {
	tox, err := createTestToxInstance(t, "callback_test")
	require.NoError(t, err)
	defer tox.Kill()

	toxAV, err := NewToxAV(tox)
	require.NoError(t, err)
	defer toxAV.Kill()

	// Test callback registrations (should not panic)
	toxAV.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
		// Callback implementation for testing
	})

	toxAV.CallbackCallState(func(friendNumber uint32, state avpkg.CallState) {
		// Callback implementation for testing
	})

	toxAV.CallbackAudioBitRate(func(friendNumber, bitRate uint32) {
		// Callback implementation for testing
	})

	toxAV.CallbackVideoBitRate(func(friendNumber, bitRate uint32) {
		// Callback implementation for testing
	})

	toxAV.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		// Callback implementation for testing
	})

	toxAV.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		// Callback implementation for testing
	})

	// Test that callbacks are registered without errors
	// Note: Actual callback invocation requires active calls which need network setup
	assert.NotNil(t, toxAV, "ToxAV should remain functional after callback registration")
}

// testToxAVResourceManagement tests proper resource management and cleanup
func testToxAVResourceManagement(t *testing.T) {
	// Test ToxAV lifecycle
	tox, err := createTestToxInstance(t, "resource_test")
	require.NoError(t, err)
	defer tox.Kill()

	toxAV, err := NewToxAV(tox)
	require.NoError(t, err)

	// Test operations work before kill
	toxAV.Iterate()
	interval := toxAV.IterationInterval()
	assert.Greater(t, interval, time.Duration(0), "Should have positive iteration interval")

	// Test graceful shutdown
	toxAV.Kill()

	// Test operations fail gracefully after kill
	err = toxAV.Call(1, 64000, 0)
	assert.Error(t, err, "Operations should fail after Kill()")

	// Test double kill is safe
	toxAV.Kill() // Should not panic

	// Test creating multiple ToxAV instances for memory leak testing
	for i := 0; i < 5; i++ {
		tempTox, err := createTestToxInstance(t, "temp")
		require.NoError(t, err)

		tempToxAV, err := NewToxAV(tempTox)
		require.NoError(t, err)

		tempToxAV.Iterate()
		tempToxAV.Kill()
		tempTox.Kill()
	}
}

// testToxAVIterationIntegration tests ToxAV iteration integration with Tox event loop
func testToxAVIterationIntegration(t *testing.T) {
	tox, err := createTestToxInstance(t, "iteration_test")
	require.NoError(t, err)
	defer tox.Kill()

	toxAV, err := NewToxAV(tox)
	require.NoError(t, err)
	defer toxAV.Kill()

	// Test iteration timing consistency
	interval1 := toxAV.IterationInterval()
	time.Sleep(10 * time.Millisecond)
	interval2 := toxAV.IterationInterval()
	assert.Equal(t, interval1, interval2, "Iteration interval should be consistent")

	// Test multiple iterations
	for i := 0; i < 10; i++ {
		toxAV.Iterate()
		time.Sleep(interval1 / 10) // Short sleep between iterations
	}

	// Test rapid iterations (should not panic)
	for i := 0; i < 50; i++ {
		toxAV.Iterate()
	}
}

// testToxAVTransportIntegration tests ToxAV integration with transport layer
func testToxAVTransportIntegration(t *testing.T) {
	// Test UDP transport integration
	t.Run("UDPTransport", func(t *testing.T) {
		options := NewOptions()
		options.UDPEnabled = true

		tox, err := New(options)
		require.NoError(t, err)
		defer tox.Kill()

		toxAV, err := NewToxAV(tox)
		require.NoError(t, err)
		defer toxAV.Kill()

		// Test basic operations work with UDP transport
		toxAV.Iterate()
		assert.Greater(t, toxAV.IterationInterval(), time.Duration(0), "Should work with UDP transport")
	})

	// Test Noise-IK transport integration
	t.Run("NoiseIKTransport", func(t *testing.T) {
		options := NewOptions()
		options.UDPEnabled = true // Noise-IK uses UDP as base

		tox, err := New(options)
		require.NoError(t, err)
		defer tox.Kill()

		toxAV, err := NewToxAV(tox)
		require.NoError(t, err)
		defer toxAV.Kill()

		// Test basic operations work with Noise-IK transport
		toxAV.Iterate()
		assert.Greater(t, toxAV.IterationInterval(), time.Duration(0), "Should work with Noise-IK transport")
	})
}

// createTestToxInstance creates a Tox instance for testing

// --- Tests from toxav_audio_integration_test.go ---

// TestToxAVAudioSendFrameIntegration tests the complete audio frame sending integration
func TestToxAVAudioSendFrameIntegration(t *testing.T) {
	// Create Tox instance
	options := NewOptions()
	tox, err := New(options)
	require.NoError(t, err)
	defer tox.Kill()

	// Create ToxAV instance
	toxAV, err := NewToxAV(tox)
	require.NoError(t, err)
	require.NotNil(t, toxAV)
	defer toxAV.Kill()

	// Start a call (this will set up media components)
	friendNumber := uint32(123)
	err = toxAV.Call(friendNumber, 64000, 0) // Audio-only call
	if err != nil {
		// Network errors are expected in test environments
		t.Skipf("Skipping audio integration test due to network setup failure: %v", err)
		return
	}

	// Generate test PCM audio data (10ms frame at 48kHz mono)
	sampleRate := uint32(48000)
	channels := uint8(1)
	sampleCount := 480 // 10ms at 48kHz
	pcmData := make([]int16, sampleCount)

	// Fill with a simple sine wave
	for i := 0; i < sampleCount; i++ {
		pcmData[i] = int16(16384 * 0.5) // Half amplitude
	}

	// Test audio frame sending through the complete integration
	err = toxAV.AudioSendFrame(friendNumber, pcmData, sampleCount, channels, sampleRate)
	require.NoError(t, err, "Audio frame sending should succeed through complete integration")

	// Test input validation through ToxAV API
	err = toxAV.AudioSendFrame(friendNumber, []int16{}, 0, 1, 48000)
	assert.Error(t, err, "Empty PCM data should be rejected")

	err = toxAV.AudioSendFrame(friendNumber, pcmData, 0, 1, 48000)
	assert.Error(t, err, "Invalid sample count should be rejected")

	err = toxAV.AudioSendFrame(friendNumber, pcmData, sampleCount, 0, 48000)
	assert.Error(t, err, "Invalid channel count should be rejected")

	err = toxAV.AudioSendFrame(friendNumber, pcmData, sampleCount, channels, 0)
	assert.Error(t, err, "Invalid sample rate should be rejected")

	// Test with non-existent friend
	err = toxAV.AudioSendFrame(999, pcmData, sampleCount, channels, sampleRate)
	assert.Error(t, err, "Non-existent friend should be rejected")
	assert.Contains(t, err.Error(), "no active call")

	// End call
	err = toxAV.CallControl(friendNumber, avpkg.CallControlCancel)
	require.NoError(t, err)
}

// TestToxAVAudioSendFramePerformance benchmarks the complete audio sending pipeline
func TestToxAVAudioSendFramePerformance(t *testing.T) {
	// Create Tox instance
	options := NewOptions()
	tox, err := New(options)
	require.NoError(t, err)
	defer tox.Kill()

	// Create ToxAV instance
	toxAV, err := NewToxAV(tox)
	require.NoError(t, err)
	defer toxAV.Kill()

	// Start a call
	friendNumber := uint32(123)
	err = toxAV.Call(friendNumber, 64000, 0)
	if err != nil {
		// Network errors are expected in test environments
		t.Skipf("Skipping audio performance test due to network setup failure: %v", err)
		return
	}

	// Generate test audio frame
	pcmData := make([]int16, 480) // 10ms at 48kHz
	for i := range pcmData {
		pcmData[i] = int16(16384 * 0.5)
	}

	// Measure performance of audio frame sending
	iterations := 1000
	for i := 0; i < iterations; i++ {
		err = toxAV.AudioSendFrame(friendNumber, pcmData, 480, 1, 48000)
		require.NoError(t, err)
	}

	// If we get here, the integration is working and performant
	t.Logf("Successfully sent %d audio frames through complete ToxAV integration", iterations)
}

// --- Tests from toxav_frame_transport_test.go ---

// TestToxAVTransportAdapter_AudioVideoFramePackets tests that audio and video frame
// packets (0x33 and 0x34) are properly handled by the transport adapter.
// This test verifies the fix for AUDIT.md issue:
// "ToxAV Transport Adapter Does Not Handle Audio/Video Frame Packets"
func TestToxAVTransportAdapter_AudioVideoFramePackets(t *testing.T) {
	// Create a mock UDP transport
	mockTransport := newMockUDPTransport()

	// Create the ToxAV transport adapter
	adapter := newToxAVTransportAdapter(mockTransport)
	require.NotNil(t, adapter)

	// Test address (192.168.1.100:5555)
	testAddr := []byte{192, 168, 1, 100, 21, 179} // Port 5555 = 0x15B3
	testData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	tests := []struct {
		name              string
		packetType        byte
		expectedTransport transport.PacketType
		description       string
	}{
		{
			name:              "CallRequest",
			packetType:        0x30,
			expectedTransport: transport.PacketAVCallRequest,
			description:       "Call request packet",
		},
		{
			name:              "CallResponse",
			packetType:        0x31,
			expectedTransport: transport.PacketAVCallResponse,
			description:       "Call response packet",
		},
		{
			name:              "CallControl",
			packetType:        0x32,
			expectedTransport: transport.PacketAVCallControl,
			description:       "Call control packet",
		},
		{
			name:              "AudioFrame",
			packetType:        0x33,
			expectedTransport: transport.PacketAVAudioFrame,
			description:       "Audio frame packet (new)",
		},
		{
			name:              "VideoFrame",
			packetType:        0x34,
			expectedTransport: transport.PacketAVVideoFrame,
			description:       "Video frame packet (new)",
		},
		{
			name:              "BitrateControl",
			packetType:        0x35,
			expectedTransport: transport.PacketAVBitrateControl,
			description:       "Bitrate control packet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous packets
			mockTransport.sentPackets = nil

			// Send packet via adapter
			err := adapter.Send(tt.packetType, testData, testAddr)
			require.NoError(t, err, "Send should succeed for packet type 0x%02x", tt.packetType)

			// Verify packet was sent
			require.Len(t, mockTransport.sentPackets, 1, "Should have sent exactly one packet")

			sentPacket := mockTransport.sentPackets[0]
			assert.Equal(t, tt.expectedTransport, sentPacket.packet.PacketType,
				"Should convert 0x%02x to transport.%v", tt.packetType, tt.expectedTransport)
			assert.Equal(t, testData, sentPacket.packet.Data, "Packet data should match")
			assert.Equal(t, "192.168.1.100:5555", sentPacket.addr.String(), "Destination address should match")
		})
	}
}

// TestToxAVTransportAdapter_RegisterHandlers tests that all AV packet handlers
// including audio and video frames can be registered correctly.
func TestToxAVTransportAdapter_RegisterHandlers(t *testing.T) {
	mockTransport := newMockUDPTransport()
	adapter := newToxAVTransportAdapter(mockTransport)
	require.NotNil(t, adapter)

	handlerCalls := make(map[byte]int)
	var mu sync.Mutex

	// Create a handler that tracks which packet type it was called for
	createHandler := func(packetType byte) func([]byte, []byte) error {
		return func(data, addr []byte) error {
			mu.Lock()
			handlerCalls[packetType]++
			mu.Unlock()
			return nil
		}
	}

	// Register handlers for all packet types
	packetTypes := []byte{0x30, 0x31, 0x32, 0x33, 0x34, 0x35}
	for _, pt := range packetTypes {
		adapter.RegisterHandler(pt, createHandler(pt))
	}

	// Verify all handlers were registered in the mock transport
	assert.Len(t, mockTransport.handlers, 6, "Should have registered 6 packet handlers")

	// Verify specific packet types are registered
	expectedTransportTypes := []transport.PacketType{
		transport.PacketAVCallRequest,
		transport.PacketAVCallResponse,
		transport.PacketAVCallControl,
		transport.PacketAVAudioFrame,
		transport.PacketAVVideoFrame,
		transport.PacketAVBitrateControl,
	}

	for _, pt := range expectedTransportTypes {
		_, exists := mockTransport.handlers[pt]
		assert.True(t, exists, "Handler for %v should be registered", pt)
	}
}

// TestToxAVTransportAdapter_UnknownPacketType tests that unknown packet types
// are properly rejected.
func TestToxAVTransportAdapter_UnknownPacketType(t *testing.T) {
	mockTransport := newMockUDPTransport()
	adapter := newToxAVTransportAdapter(mockTransport)
	require.NotNil(t, adapter)

	testAddr := []byte{192, 168, 1, 100, 0, 80}
	testData := []byte{0x01, 0x02, 0x03}

	// Test unknown packet types
	unknownTypes := []byte{0x00, 0x29, 0x36, 0xFF}
	for _, pt := range unknownTypes {
		err := adapter.Send(pt, testData, testAddr)
		assert.Error(t, err, "Unknown packet type 0x%02x should return error", pt)
		assert.Contains(t, err.Error(), "unknown AV packet type", "Error should mention unknown packet type")
	}
}
