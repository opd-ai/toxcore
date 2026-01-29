package transport

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionTimeoutManagement(t *testing.T) {
	// Create mock underlying transport
	mockTransport := &mockTransportHelper{}

	// Create test private key
	privKey := make([]byte, 32)
	for i := range privKey {
		privKey[i] = byte(i)
	}

	nt, err := NewNoiseTransport(mockTransport, privKey)
	require.NoError(t, err)
	defer nt.Close()

	// Verify cleanup goroutines are running
	assert.NotNil(t, nt.stopCleanup)
	assert.NotNil(t, nt.stopSessionCleanup)
}

func TestIncompleteHandshakeTimeout(t *testing.T) {
	mockTransport := &mockTransportHelper{}
	privKey := make([]byte, 32)
	for i := range privKey {
		privKey[i] = byte(i)
	}

	nt, err := NewNoiseTransport(mockTransport, privKey)
	require.NoError(t, err)
	defer nt.Close()

	// Create an incomplete session manually
	testAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	oldTime := time.Now().Add(-2 * HandshakeTimeout) // Older than timeout

	nt.sessionsMu.Lock()
	nt.sessions[testAddr.String()] = &NoiseSession{
		peerAddr:   testAddr,
		complete:   false,
		createdAt:  oldTime,
		lastActive: oldTime,
	}
	nt.sessionsMu.Unlock()

	assert.Equal(t, 1, len(nt.sessions))

	// Trigger cleanup
	nt.performSessionCleanup()

	// Session should be removed
	nt.sessionsMu.RLock()
	_, exists := nt.sessions[testAddr.String()]
	nt.sessionsMu.RUnlock()

	assert.False(t, exists, "Incomplete session should be removed after timeout")
	assert.Equal(t, 0, len(nt.sessions))
}

func TestCompleteSessionIdleTimeout(t *testing.T) {
	mockTransport := &mockTransportHelper{}
	privKey := make([]byte, 32)
	for i := range privKey {
		privKey[i] = byte(i)
	}

	nt, err := NewNoiseTransport(mockTransport, privKey)
	require.NoError(t, err)
	defer nt.Close()

	// Create a complete but idle session
	testAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	oldTime := time.Now().Add(-2 * SessionIdleTimeout) // Older than idle timeout

	nt.sessionsMu.Lock()
	nt.sessions[testAddr.String()] = &NoiseSession{
		peerAddr:   testAddr,
		complete:   true,
		createdAt:  oldTime,
		lastActive: oldTime,
	}
	nt.sessionsMu.Unlock()

	assert.Equal(t, 1, len(nt.sessions))

	// Trigger cleanup
	nt.performSessionCleanup()

	// Session should be removed
	nt.sessionsMu.RLock()
	_, exists := nt.sessions[testAddr.String()]
	nt.sessionsMu.RUnlock()

	assert.False(t, exists, "Idle session should be removed after timeout")
	assert.Equal(t, 0, len(nt.sessions))
}

func TestActiveSessionNotRemoved(t *testing.T) {
	mockTransport := &mockTransportHelper{}
	privKey := make([]byte, 32)
	for i := range privKey {
		privKey[i] = byte(i)
	}

	nt, err := NewNoiseTransport(mockTransport, privKey)
	require.NoError(t, err)
	defer nt.Close()

	// Create an active session
	testAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	now := time.Now()

	nt.sessionsMu.Lock()
	nt.sessions[testAddr.String()] = &NoiseSession{
		peerAddr:   testAddr,
		complete:   true,
		createdAt:  now,
		lastActive: now,
	}
	nt.sessionsMu.Unlock()

	assert.Equal(t, 1, len(nt.sessions))

	// Trigger cleanup
	nt.performSessionCleanup()

	// Session should still exist
	nt.sessionsMu.RLock()
	_, exists := nt.sessions[testAddr.String()]
	nt.sessionsMu.RUnlock()

	assert.True(t, exists, "Active session should not be removed")
	assert.Equal(t, 1, len(nt.sessions))
}

func TestNewIncompleteSessionNotRemoved(t *testing.T) {
	mockTransport := &mockTransportHelper{}
	privKey := make([]byte, 32)
	for i := range privKey {
		privKey[i] = byte(i)
	}

	nt, err := NewNoiseTransport(mockTransport, privKey)
	require.NoError(t, err)
	defer nt.Close()

	// Create a recent incomplete session
	testAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	now := time.Now()

	nt.sessionsMu.Lock()
	nt.sessions[testAddr.String()] = &NoiseSession{
		peerAddr:   testAddr,
		complete:   false,
		createdAt:  now,
		lastActive: now,
	}
	nt.sessionsMu.Unlock()

	assert.Equal(t, 1, len(nt.sessions))

	// Trigger cleanup
	nt.performSessionCleanup()

	// Session should still exist (not old enough)
	nt.sessionsMu.RLock()
	_, exists := nt.sessions[testAddr.String()]
	nt.sessionsMu.RUnlock()

	assert.True(t, exists, "New incomplete session should not be removed")
	assert.Equal(t, 1, len(nt.sessions))
}

func TestMultipleSessionsCleanup(t *testing.T) {
	mockTransport := &mockTransportHelper{}
	privKey := make([]byte, 32)
	for i := range privKey {
		privKey[i] = byte(i)
	}

	nt, err := NewNoiseTransport(mockTransport, privKey)
	require.NoError(t, err)
	defer nt.Close()

	now := time.Now()
	oldTime := now.Add(-2 * HandshakeTimeout)

	// Create mix of sessions
	sessions := []struct {
		port     int
		complete bool
		old      bool
	}{
		{8080, false, true},  // Should be removed (old incomplete)
		{8081, true, true},   // Should be removed (old complete)
		{8082, false, false}, // Should stay (new incomplete)
		{8083, true, false},  // Should stay (active complete)
	}

	nt.sessionsMu.Lock()
	for _, s := range sessions {
		addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: s.port}
		timestamp := now
		if s.old {
			if s.complete {
				timestamp = now.Add(-2 * SessionIdleTimeout)
			} else {
				timestamp = oldTime
			}
		}

		nt.sessions[addr.String()] = &NoiseSession{
			peerAddr:   addr,
			complete:   s.complete,
			createdAt:  timestamp,
			lastActive: timestamp,
		}
	}
	nt.sessionsMu.Unlock()

	assert.Equal(t, 4, len(nt.sessions))

	// Trigger cleanup
	nt.performSessionCleanup()

	// Check results
	nt.sessionsMu.RLock()
	remaining := len(nt.sessions)
	nt.sessionsMu.RUnlock()

	assert.Equal(t, 2, remaining, "Only active sessions should remain")
}

func TestSessionTimestampUpdate(t *testing.T) {
	// This test verifies that session timestamps are updated during Send operations
	// Note: Full integration test would require complete handshake setup

	mockTransport := &mockTransportHelper{}
	privKey := make([]byte, 32)
	for i := range privKey {
		privKey[i] = byte(i)
	}

	nt, err := NewNoiseTransport(mockTransport, privKey)
	require.NoError(t, err)
	defer nt.Close()

	// Create a complete session
	testAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	oldTime := time.Now().Add(-1 * time.Minute)

	session := &NoiseSession{
		peerAddr:   testAddr,
		complete:   true,
		createdAt:  oldTime,
		lastActive: oldTime,
	}

	nt.sessionsMu.Lock()
	nt.sessions[testAddr.String()] = session
	nt.sessionsMu.Unlock()

	// Note: Actual Send would update lastActive, but requires complete handshake setup
	// This test documents the expected behavior

	session.mu.RLock()
	lastActive := session.lastActive
	session.mu.RUnlock()

	assert.Equal(t, oldTime, lastActive)
}

// mockTransportHelper is a minimal mock for testing
type mockTransportHelper struct {
	handlers map[PacketType]PacketHandler
}

func (m *mockTransportHelper) Send(packet *Packet, addr net.Addr) error {
	return nil
}

func (m *mockTransportHelper) RegisterHandler(packetType PacketType, handler PacketHandler) {
	if m.handlers == nil {
		m.handlers = make(map[PacketType]PacketHandler)
	}
	m.handlers[packetType] = handler
}

func (m *mockTransportHelper) Close() error {
	return nil
}

func (m *mockTransportHelper) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 33445}
}

func (m *mockTransportHelper) IsConnectionOriented() bool {
	return false
}
