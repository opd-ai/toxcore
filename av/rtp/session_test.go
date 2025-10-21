package rtp

import (
	"net"
	"testing"

	"github.com/opd-ai/toxforge/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSession(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	tests := []struct {
		name         string
		friendNumber uint32
		transport    transport.Transport
		remoteAddr   net.Addr
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "Valid parameters",
			friendNumber: 42,
			transport:    mockTransport,
			remoteAddr:   remoteAddr,
			expectError:  false,
		},
		{
			name:         "Nil transport",
			friendNumber: 42,
			transport:    nil,
			remoteAddr:   remoteAddr,
			expectError:  true,
			errorMsg:     "transport cannot be nil",
		},
		{
			name:         "Nil remote address",
			friendNumber: 42,
			transport:    mockTransport,
			remoteAddr:   nil,
			expectError:  true,
			errorMsg:     "remote address cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := NewSession(tt.friendNumber, tt.transport, tt.remoteAddr)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, session)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, session)
				assert.Equal(t, tt.friendNumber, session.friendNumber)
				assert.NotNil(t, session.audioPacketizer)
				assert.NotNil(t, session.audioDepacketizer)
				assert.Equal(t, tt.transport, session.transport)
				assert.Equal(t, tt.remoteAddr, session.remoteAddr)
				assert.NotZero(t, session.created)
			}
		})
	}
}

func TestSession_SendAudioPacket(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	session, err := NewSession(42, mockTransport, remoteAddr)
	require.NoError(t, err)

	tests := []struct {
		name        string
		audioData   []byte
		sampleCount uint32
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid audio data",
			audioData:   []byte{0x01, 0x02, 0x03, 0x04},
			sampleCount: 960, // 20ms at 48kHz
			expectError: false,
		},
		{
			name:        "Empty audio data",
			audioData:   []byte{},
			sampleCount: 960,
			expectError: true,
			errorMsg:    "failed to send audio packet",
		},
		{
			name:        "Large audio frame",
			audioData:   make([]byte, 512),
			sampleCount: 2880, // 60ms at 48kHz
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialStats := session.GetStatistics()
			initialPacketCount := len(mockTransport.GetSentPackets())

			err := session.SendAudioPacket(tt.audioData, tt.sampleCount)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				// Statistics should not change on error
				finalStats := session.GetStatistics()
				assert.Equal(t, initialStats.PacketsSent, finalStats.PacketsSent)
			} else {
				assert.NoError(t, err)

				// Should send exactly one packet
				sentPackets := mockTransport.GetSentPackets()
				assert.Equal(t, initialPacketCount+1, len(sentPackets))

				// Statistics should be updated
				finalStats := session.GetStatistics()
				assert.Equal(t, initialStats.PacketsSent+1, finalStats.PacketsSent)
			}
		})
	}
}

func TestSession_SendVideoPacket(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	session, err := NewSession(42, mockTransport, remoteAddr)
	require.NoError(t, err)

	// Video packets should return error (not implemented yet)
	err = session.SendVideoPacket([]byte{0x01, 0x02, 0x03})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "video RTP packetization not yet implemented")

	// Should not send any packets
	sentPackets := mockTransport.GetSentPackets()
	assert.Equal(t, 0, len(sentPackets))
}

func TestSession_ReceivePacket(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	session, err := NewSession(42, mockTransport, remoteAddr)
	require.NoError(t, err)

	// Create a valid RTP audio packet for testing
	// First, we need to send a packet through the packetizer to get valid RTP data
	testAudioData := []byte{0x01, 0x02, 0x03, 0x04}
	err = session.SendAudioPacket(testAudioData, 960)
	require.NoError(t, err)

	// Get the sent packet data
	sentPackets := mockTransport.GetSentPackets()
	require.Equal(t, 1, len(sentPackets))
	rtpData := sentPackets[0].Packet.Data

	tests := []struct {
		name         string
		packet       []byte
		expectError  bool
		errorMsg     string
		expectedType string
	}{
		{
			name:         "Valid RTP packet",
			packet:       rtpData,
			expectError:  false,
			expectedType: "audio",
		},
		{
			name:        "Empty packet",
			packet:      []byte{},
			expectError: true,
			errorMsg:    "packet cannot be empty",
		},
		{
			name:        "Invalid RTP packet",
			packet:      []byte{0x01, 0x02}, // Too short
			expectError: true,
			errorMsg:    "failed to process audio packet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialStats := session.GetStatistics()

			data, mediaType, err := session.ReceivePacket(tt.packet)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, data)
				assert.Empty(t, mediaType)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
				assert.Equal(t, tt.expectedType, mediaType)

				// Statistics should be updated
				finalStats := session.GetStatistics()
				assert.Equal(t, initialStats.PacketsReceived+1, finalStats.PacketsReceived)
			}
		})
	}
}

func TestSession_GetStatistics(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	session, err := NewSession(42, mockTransport, remoteAddr)
	require.NoError(t, err)

	// Initial statistics should be zero
	stats := session.GetStatistics()
	assert.Equal(t, uint64(0), stats.PacketsSent)
	assert.Equal(t, uint64(0), stats.PacketsReceived)

	// Send some packets
	audioData := []byte{0x01, 0x02, 0x03, 0x04}
	for i := 0; i < 3; i++ {
		err = session.SendAudioPacket(audioData, 960)
		require.NoError(t, err)
	}

	// Statistics should reflect sent packets
	stats = session.GetStatistics()
	assert.Equal(t, uint64(3), stats.PacketsSent)
	assert.Equal(t, uint64(0), stats.PacketsReceived)
}

func TestSession_GetBufferedAudio(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	session, err := NewSession(42, mockTransport, remoteAddr)
	require.NoError(t, err)

	// Initially, no buffered audio should be available
	data, available := session.GetBufferedAudio()
	assert.False(t, available)
	assert.Nil(t, data)

	// The actual buffering behavior is tested in the jitter buffer tests
	// This test just ensures the method is accessible and doesn't panic
}

func TestSession_Close(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	session, err := NewSession(42, mockTransport, remoteAddr)
	require.NoError(t, err)

	// Verify session is properly initialized
	assert.NotNil(t, session.audioPacketizer)
	assert.NotNil(t, session.audioDepacketizer)

	// Close session
	err = session.Close()
	assert.NoError(t, err)

	// Resources should be cleaned up
	assert.Nil(t, session.audioPacketizer)
	assert.Nil(t, session.audioDepacketizer)

	// Operations after close should fail gracefully
	err = session.SendAudioPacket([]byte{0x01, 0x02}, 960)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "audio packetizer not initialized")

	data, available := session.GetBufferedAudio()
	assert.False(t, available)
	assert.Nil(t, data)
}

// Benchmark tests for performance validation
func BenchmarkSession_SendAudioPacket(b *testing.B) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	session, err := NewSession(42, mockTransport, remoteAddr)
	require.NoError(b, err)

	audioData := make([]byte, 160) // Typical Opus frame size
	sampleCount := uint32(960)     // 20ms at 48kHz

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := session.SendAudioPacket(audioData, sampleCount)
		if err != nil {
			b.Fatal(err)
		}
	}
}
