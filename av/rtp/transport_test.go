package rtp

import (
	"net"
	"testing"

	"github.com/opd-ai/toxcore/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTransportIntegration(t *testing.T) {
	tests := []struct {
		name        string
		transport   transport.Transport
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid transport",
			transport:   NewMockTransport(),
			expectError: false,
		},
		{
			name:        "Nil transport",
			transport:   nil,
			expectError: true,
			errorMsg:    "transport cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration, err := NewTransportIntegration(tt.transport)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, integration)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, integration)
				assert.Equal(t, tt.transport, integration.transport)
				assert.NotNil(t, integration.sessions)
				assert.Equal(t, 0, len(integration.sessions))
			}
		})
	}
}

func TestTransportIntegration_CreateSession(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	tests := []struct {
		name         string
		friendNumber uint32
		remoteAddr   net.Addr
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "Valid session creation",
			friendNumber: 42,
			remoteAddr:   remoteAddr,
			expectError:  false,
		},
		{
			name:         "Duplicate session creation",
			friendNumber: 42, // Same friend number as above
			remoteAddr:   remoteAddr,
			expectError:  true,
			errorMsg:     "session already exists",
		},
		{
			name:         "Different friend session",
			friendNumber: 43,
			remoteAddr:   remoteAddr,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := integration.CreateSession(tt.friendNumber, tt.remoteAddr)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, session)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, session)
				assert.Equal(t, tt.friendNumber, session.friendNumber)

				// Verify session is stored
				storedSession, exists := integration.GetSession(tt.friendNumber)
				assert.True(t, exists)
				assert.Equal(t, session, storedSession)
			}
		})
	}
}

func TestTransportIntegration_GetSession(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	// Test getting non-existent session
	session, exists := integration.GetSession(42)
	assert.False(t, exists)
	assert.Nil(t, session)

	// Create a session
	createdSession, err := integration.CreateSession(42, remoteAddr)
	require.NoError(t, err)

	// Test getting existing session
	session, exists = integration.GetSession(42)
	assert.True(t, exists)
	assert.Equal(t, createdSession, session)

	// Test getting different friend's session
	session, exists = integration.GetSession(43)
	assert.False(t, exists)
	assert.Nil(t, session)
}

func TestTransportIntegration_CloseSession(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	// Test closing non-existent session
	err = integration.CloseSession(42)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session exists")

	// Create a session
	_, err = integration.CreateSession(42, remoteAddr)
	require.NoError(t, err)

	// Verify session exists
	_, exists := integration.GetSession(42)
	assert.True(t, exists)

	// Close the session
	err = integration.CloseSession(42)
	assert.NoError(t, err)

	// Verify session is removed
	_, exists = integration.GetSession(42)
	assert.False(t, exists)

	// Test closing already closed session
	err = integration.CloseSession(42)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session exists")
}

func TestTransportIntegration_GetAllSessions(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	// Initially no sessions
	sessions := integration.GetAllSessions()
	assert.Equal(t, 0, len(sessions))

	// Create multiple sessions
	session1, err := integration.CreateSession(42, remoteAddr)
	require.NoError(t, err)

	session2, err := integration.CreateSession(43, remoteAddr)
	require.NoError(t, err)

	// Get all sessions
	sessions = integration.GetAllSessions()
	assert.Equal(t, 2, len(sessions))
	assert.Equal(t, session1, sessions[42])
	assert.Equal(t, session2, sessions[43])

	// Verify it's a copy (modifications don't affect original)
	sessions[99] = session1
	originalSessions := integration.GetAllSessions()
	assert.Equal(t, 2, len(originalSessions))
	_, exists := originalSessions[99]
	assert.False(t, exists)
}

func TestTransportIntegration_Close(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	// Create multiple sessions
	_, err = integration.CreateSession(42, remoteAddr)
	require.NoError(t, err)

	_, err = integration.CreateSession(43, remoteAddr)
	require.NoError(t, err)

	// Verify sessions exist
	sessions := integration.GetAllSessions()
	assert.Equal(t, 2, len(sessions))

	// Close integration
	err = integration.Close()
	assert.NoError(t, err)

	// Verify all sessions are closed
	sessions = integration.GetAllSessions()
	assert.Equal(t, 0, len(sessions))

	// Verify individual sessions no longer exist
	_, exists := integration.GetSession(42)
	assert.False(t, exists)

	_, exists = integration.GetSession(43)
	assert.False(t, exists)
}

func TestTransportIntegration_PacketHandlers(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	// Test without session - should return error
	packet := &transport.Packet{
		PacketType: transport.PacketAVAudioFrame,
		Data:       []byte{0x01, 0x02, 0x03, 0x04},
	}
	err = integration.handleIncomingAudioFrame(packet, remoteAddr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session found")

	// Create a session for testing packet handling
	_, err = integration.CreateSession(42, remoteAddr)
	require.NoError(t, err)

	// Test video frame handler with properly formatted RTP packet
	// RTP header (12 bytes) + VP8 payload descriptor (3 bytes) + payload
	validVideoPacket := make([]byte, 16)
	// RTP header
	validVideoPacket[0] = 0x80 // Version 2
	validVideoPacket[1] = 0xE0 // Marker bit + Payload type 96 (VP8)
	validVideoPacket[2] = 0x00 // Sequence number (upper byte)
	validVideoPacket[3] = 0x01 // Sequence number (lower byte)
	// Timestamp (bytes 4-7)
	validVideoPacket[4] = 0x00
	validVideoPacket[5] = 0x00
	validVideoPacket[6] = 0x00
	validVideoPacket[7] = 0x10
	// SSRC (bytes 8-11)
	validVideoPacket[8] = 0x12
	validVideoPacket[9] = 0x34
	validVideoPacket[10] = 0x56
	validVideoPacket[11] = 0x78
	// VP8 Payload Descriptor (3 bytes)
	validVideoPacket[12] = 0x90 // X=1, S=1 (extended bits, start of partition)
	validVideoPacket[13] = 0x80 // I=1, picture ID upper bits
	validVideoPacket[14] = 0x01 // Picture ID lower bits
	// Payload data
	validVideoPacket[15] = 0xFF

	packet.PacketType = transport.PacketAVVideoFrame
	packet.Data = validVideoPacket
	err = integration.handleIncomingVideoFrame(packet, remoteAddr)
	assert.NoError(t, err)
}

func TestTransportIntegration_AddressMapping(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr1, _ := net.ResolveUDPAddr("udp", "127.0.0.1:10001")
	remoteAddr2, _ := net.ResolveUDPAddr("udp", "127.0.0.1:10002")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	// Create sessions for different friends with different addresses
	session1, err := integration.CreateSession(1, remoteAddr1)
	require.NoError(t, err)
	assert.NotNil(t, session1)

	session2, err := integration.CreateSession(2, remoteAddr2)
	require.NoError(t, err)
	assert.NotNil(t, session2)

	// Verify address-to-friend mappings are registered
	assert.Equal(t, uint32(1), integration.addrToFriend[remoteAddr1.String()])
	assert.Equal(t, uint32(2), integration.addrToFriend[remoteAddr2.String()])

	// Verify friend-to-address mappings are registered
	assert.Equal(t, remoteAddr1, integration.friendToAddr[1])
	assert.Equal(t, remoteAddr2, integration.friendToAddr[2])

	// Close a session and verify mappings are removed
	err = integration.CloseSession(1)
	require.NoError(t, err)

	_, exists := integration.addrToFriend[remoteAddr1.String()]
	assert.False(t, exists)

	_, exists = integration.friendToAddr[1]
	assert.False(t, exists)

	// Verify other session's mappings are intact
	assert.Equal(t, uint32(2), integration.addrToFriend[remoteAddr2.String()])
	assert.Equal(t, remoteAddr2, integration.friendToAddr[2])
}

func TestTransportIntegration_IncomingPacketRouting(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	// Create a session
	_, err = integration.CreateSession(42, remoteAddr)
	require.NoError(t, err)

	// Create a valid RTP packet with header
	rtpPacket := []byte{
		0x80, 0x60, 0x00, 0x01, // Version, Payload Type, Sequence
		0x00, 0x00, 0x00, 0x10, // Timestamp
		0x12, 0x34, 0x56, 0x78, // SSRC
		0x01, 0x02, 0x03, 0x04, // Payload data
	}

	packet := &transport.Packet{
		PacketType: transport.PacketAVAudioFrame,
		Data:       rtpPacket,
	}

	// Test routing packet to correct session
	err = integration.handleIncomingAudioFrame(packet, remoteAddr)
	// The packet should be successfully routed and processed
	assert.NoError(t, err)

	// Test with unknown address - should fail with "no session found"
	unknownAddr, _ := net.ResolveUDPAddr("udp", "192.168.1.100:9999")
	err = integration.handleIncomingAudioFrame(packet, unknownAddr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session found")
}

// Benchmark tests for performance validation
func BenchmarkTransportIntegration_CreateSession(b *testing.B) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use different friend numbers to avoid "already exists" errors
		friendNumber := uint32(i)
		session, err := integration.CreateSession(friendNumber, remoteAddr)
		if err != nil {
			b.Fatal(err)
		}
		_ = session
	}
}

// TestTransportIntegration_Callbacks verifies callback registration and invocation.
func TestTransportIntegration_Callbacks(t *testing.T) {
	mockTransport := NewMockTransport()
	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	t.Run("AudioCallback", func(t *testing.T) {
		var receivedFriend uint32
		var receivedSamples []int16
		var receivedChannels uint8

		callback := func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
			receivedFriend = friendNumber
			receivedSamples = pcm
			receivedChannels = channels
		}

		// Set callback
		integration.SetAudioReceiveCallback(callback)
		assert.NotNil(t, integration.audioReceiveCallback)

		// Unset callback
		integration.SetAudioReceiveCallback(nil)
		assert.Nil(t, integration.audioReceiveCallback)

		// Verify callback was stored correctly
		integration.SetAudioReceiveCallback(callback)
		// We can't easily invoke the callback without setting up full session,
		// but we verify the setter works
		_ = receivedFriend
		_ = receivedSamples
		_ = receivedChannels
	})

	t.Run("VideoCallback", func(t *testing.T) {
		var receivedFriend uint32
		var receivedPictureID uint16
		var receivedData []byte

		callback := func(friendNumber uint32, pictureID uint16, frameData []byte) {
			receivedFriend = friendNumber
			receivedPictureID = pictureID
			receivedData = frameData
		}

		// Set callback
		integration.SetVideoReceiveCallback(callback)
		assert.NotNil(t, integration.videoReceiveCallback)

		// Unset callback
		integration.SetVideoReceiveCallback(nil)
		assert.Nil(t, integration.videoReceiveCallback)

		// Verify callback was stored correctly
		integration.SetVideoReceiveCallback(callback)
		_ = receivedFriend
		_ = receivedPictureID
		_ = receivedData
	})
}
