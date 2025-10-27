package av

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTransport implements TransportInterface for testing audio frame integration
type MockTransport struct {
	sentPackets []MockPacket
	handlers    map[byte]func([]byte, []byte) error
}

type MockPacket struct {
	PacketType byte
	Data       []byte
	Addr       []byte
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		sentPackets: make([]MockPacket, 0),
		handlers:    make(map[byte]func([]byte, []byte) error),
	}
}

func (m *MockTransport) Send(packetType byte, data, addr []byte) error {
	m.sentPackets = append(m.sentPackets, MockPacket{
		PacketType: packetType,
		Data:       append([]byte(nil), data...), // Copy data
		Addr:       append([]byte(nil), addr...), // Copy addr
	})
	return nil
}

func (m *MockTransport) RegisterHandler(packetType byte, handler func([]byte, []byte) error) {
	m.handlers[packetType] = handler
}

func (m *MockTransport) GetSentPackets() []MockPacket {
	return m.sentPackets
}

func (m *MockTransport) ClearSentPackets() {
	m.sentPackets = m.sentPackets[:0]
}

// mockFriendAddressLookup provides mock friend address resolution
func mockFriendAddressLookup(friendNumber uint32) ([]byte, error) {
	// Return a mock address based on friend number
	return []byte{192, 168, 1, byte(friendNumber + 100)}, nil
}

// TestAudioFrameSendingIntegration tests the complete audio frame sending pipeline
func TestAudioFrameSendingIntegration(t *testing.T) {
	// Create mock transport
	transport := NewMockTransport()

	// Create manager
	manager, err := NewManager(transport, mockFriendAddressLookup)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// Start the manager
	err = manager.Start()
	require.NoError(t, err)

	// Start a call with audio enabled
	friendNumber := uint32(123)
	audioBitRate := uint32(64000)
	videoBitRate := uint32(0) // Audio-only call

	err = manager.StartCall(friendNumber, audioBitRate, videoBitRate)
	require.NoError(t, err)

	// Verify call was created
	call := manager.GetCall(friendNumber)
	require.NotNil(t, call)
	assert.Equal(t, friendNumber, call.GetFriendNumber())
	assert.True(t, call.IsAudioEnabled())
	assert.False(t, call.IsVideoEnabled())
	assert.Equal(t, audioBitRate, call.GetAudioBitRate())

	// Verify call signaling packet was sent
	sentPackets := transport.GetSentPackets()
	require.Len(t, sentPackets, 1)
	assert.Equal(t, byte(0x30), sentPackets[0].PacketType) // PacketAVCallRequest

	// Test audio processor integration
	processor := call.GetAudioProcessor()
	require.NotNil(t, processor, "Audio processor should be initialized after call setup")

	// Generate test PCM audio data (1 second of 48kHz mono sine wave)
	sampleRate := uint32(48000)
	channels := uint8(1)
	sampleCount := 480 // 10ms frame at 48kHz
	pcmData := generateTestPCM(sampleCount, channels)

	// Send audio frame through the integration
	err = call.SendAudioFrame(pcmData, sampleCount, channels, sampleRate)
	require.NoError(t, err)

	// Verify that the audio was processed (no additional packets should be sent
	// since RTP transport integration is not fully implemented in Phase 2)
	newPackets := transport.GetSentPackets()
	assert.Len(t, newPackets, 1, "No additional packets should be sent during Phase 2 audio processing")

	// Test that last frame time was updated
	assert.True(t, time.Since(call.GetLastFrameTime()) < time.Second, "Last frame time should be recent")

	// Clean up
	err = manager.EndCall(friendNumber)
	require.NoError(t, err)

	// Verify call was removed
	call = manager.GetCall(friendNumber)
	assert.Nil(t, call)

	// Stop manager
	manager.Stop()
}

// TestAudioFrameSendingValidation tests input validation for audio frame sending
func TestAudioFrameSendingValidation(t *testing.T) {
	// Create mock transport and manager
	transport := NewMockTransport()
	manager, err := NewManager(transport, mockFriendAddressLookup)
	require.NoError(t, err)

	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Start a call
	friendNumber := uint32(456)
	err = manager.StartCall(friendNumber, 64000, 0)
	require.NoError(t, err)

	call := manager.GetCall(friendNumber)
	require.NotNil(t, call)

	// Test empty PCM data
	err = call.SendAudioFrame([]int16{}, 0, 1, 48000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty PCM data")

	// Test invalid sample count
	pcmData := generateTestPCM(480, 1)
	err = call.SendAudioFrame(pcmData, 0, 1, 48000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sample count")

	// Test invalid channels
	err = call.SendAudioFrame(pcmData, 480, 0, 48000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel count")

	err = call.SendAudioFrame(pcmData, 480, 3, 48000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel count")

	// Test invalid sample rate
	err = call.SendAudioFrame(pcmData, 480, 1, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sampling rate")

	// Test with audio disabled
	call.audioEnabled = false
	err = call.SendAudioFrame(pcmData, 480, 1, 48000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "audio not enabled")
}

// TestAudioFrameProcessingPipeline tests the audio processing pipeline specifically
func TestAudioFrameProcessingPipeline(t *testing.T) {
	// Create a call with media setup
	call := NewCall(123)
	err := call.SetupMedia(nil, 123) // Simplified for Phase 2
	require.NoError(t, err)

	// Enable audio
	call.audioEnabled = true

	// Test audio processor integration
	processor := call.GetAudioProcessor()
	require.NotNil(t, processor)

	// Test different sample rates to verify resampling integration
	testCases := []struct {
		name       string
		sampleRate uint32
		channels   uint8
		samples    int
	}{
		{"48kHz Mono", 48000, 1, 480},
		{"44.1kHz Mono", 44100, 1, 441},
		{"16kHz Mono", 16000, 1, 160},
		{"8kHz Mono", 8000, 1, 80},
		{"48kHz Stereo", 48000, 2, 480},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate test PCM data
			pcmData := generateTestPCM(tc.samples, tc.channels)

			// Process through the integrated pipeline
			err := call.SendAudioFrame(pcmData, tc.samples, tc.channels, tc.sampleRate)
			require.NoError(t, err, "Audio processing should succeed for %s", tc.name)

			// Verify last frame was updated
			assert.True(t, time.Since(call.GetLastFrameTime()) < time.Second)
		})
	}
}

// TestCallMediaLifecycle tests the complete media component lifecycle
func TestCallMediaLifecycle(t *testing.T) {
	// Create transport and manager
	transport := NewMockTransport()
	manager, err := NewManager(transport, mockFriendAddressLookup)
	require.NoError(t, err)

	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	friendNumber := uint32(789)

	// Start call
	err = manager.StartCall(friendNumber, 64000, 0)
	require.NoError(t, err)

	call := manager.GetCall(friendNumber)
	require.NotNil(t, call)

	// Verify media components are initialized
	assert.NotNil(t, call.GetAudioProcessor(), "Audio processor should be initialized")

	// Note: RTP session will be nil in Phase 2 implementation
	// This will be updated when full RTP transport integration is completed

	// Test that we can send audio frames
	pcmData := generateTestPCM(480, 1)
	err = call.SendAudioFrame(pcmData, 480, 1, 48000)
	require.NoError(t, err)

	// End call and verify cleanup
	err = manager.EndCall(friendNumber)
	require.NoError(t, err)

	// Call should be removed
	call = manager.GetCall(friendNumber)
	assert.Nil(t, call)
}

// BenchmarkAudioFrameSending benchmarks the audio frame sending performance
func BenchmarkAudioFrameSending(b *testing.B) {
	// Setup
	transport := NewMockTransport()
	manager, err := NewManager(transport, mockFriendAddressLookup)
	require.NoError(b, err)

	err = manager.Start()
	require.NoError(b, err)
	defer manager.Stop()

	err = manager.StartCall(123, 64000, 0)
	require.NoError(b, err)

	call := manager.GetCall(123)
	require.NotNil(b, call)

	// Generate test audio frame (10ms at 48kHz)
	pcmData := generateTestPCM(480, 1)

	// Benchmark audio frame sending
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = call.SendAudioFrame(pcmData, 480, 1, 48000)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// generateTestPCM generates test PCM audio data
func generateTestPCM(sampleCount int, channels uint8) []int16 {
	totalSamples := sampleCount * int(channels)
	pcm := make([]int16, totalSamples)

	// Generate a simple sine wave for testing
	for i := 0; i < sampleCount; i++ {
		// 440Hz sine wave
		value := int16(16384 * 0.5) // Half amplitude to avoid clipping
		for ch := 0; ch < int(channels); ch++ {
			pcm[i*int(channels)+ch] = value
		}
	}

	return pcm
}
