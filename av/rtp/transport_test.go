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

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	// Create test packet and address
	packet := &transport.Packet{
		PacketType: transport.PacketAVAudioFrame,
		Data:       []byte{0x01, 0x02, 0x03, 0x04},
	}
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	// Test audio frame handler (should not panic)
	err = integration.handleIncomingAudioFrame(packet, addr)
	assert.NoError(t, err)

	// Test video frame handler (should not panic)
	packet.PacketType = transport.PacketAVVideoFrame
	err = integration.handleIncomingVideoFrame(packet, addr)
	assert.NoError(t, err)
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
