package transport

import (
	"crypto/rand"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNegotiatingTransport_LocalAddr tests the LocalAddr method returns the underlying transport's address.
func TestNegotiatingTransport_LocalAddr(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	require.NoError(t, err)
	defer nt.Close()

	addr := nt.LocalAddr()
	assert.NotNil(t, addr)
	assert.Equal(t, mockTransport.LocalAddr().String(), addr.String())
}

// TestNegotiatingTransport_RegisterHandler tests that handlers are registered on the underlying transport.
func TestNegotiatingTransport_RegisterHandler(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	require.NoError(t, err)
	defer nt.Close()

	// Register a custom handler
	handlerCalled := false
	nt.RegisterHandler(PacketPingRequest, func(packet *Packet, addr net.Addr) error {
		handlerCalled = true
		return nil
	})

	// Simulate receiving a packet
	packet := &Packet{PacketType: PacketPingRequest, Data: []byte("test")}
	peerAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9090}

	// The mock transport should have the handler registered
	err = mockTransport.SimulateReceive(packet, peerAddr)
	assert.NoError(t, err)
	assert.True(t, handlerCalled, "Handler should have been called")
}

// TestNegotiatingTransport_IsConnectionOriented tests that connection orientation is delegated.
func TestNegotiatingTransport_IsConnectionOriented(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	require.NoError(t, err)
	defer nt.Close()

	// MockTransport returns false for IsConnectionOriented by default
	result := nt.IsConnectionOriented()
	assert.Equal(t, mockTransport.IsConnectionOriented(), result)
}

// TestGetSecurityLevel tests the security level helper function.
func TestGetSecurityLevel(t *testing.T) {
	tests := []struct {
		name          string
		version       ProtocolVersion
		expectedLevel string
	}{
		{
			name:          "legacy protocol",
			version:       ProtocolLegacy,
			expectedLevel: "basic_nacl_encryption",
		},
		{
			name:          "noise protocol",
			version:       ProtocolNoiseIK,
			expectedLevel: "high_forward_secrecy",
		},
		{
			name:          "unknown protocol",
			version:       ProtocolVersion(99),
			expectedLevel: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := getSecurityLevel(tt.version)
			assert.Equal(t, tt.expectedLevel, level)
		})
	}
}

// TestNegotiatingTransport_GetUnderlying tests the underlying transport getter.
func TestNegotiatingTransport_GetUnderlying(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	require.NoError(t, err)
	defer nt.Close()

	underlying := nt.GetUnderlying()
	assert.Equal(t, mockTransport, underlying)
}

// TestNegotiatingTransport_AddNoiseKeyForPeer tests adding a noise key for a peer.
func TestNegotiatingTransport_AddNoiseKeyForPeer(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	require.NoError(t, err)
	defer nt.Close()

	peerAddr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 8080}
	peerPubKey := make([]byte, 32)
	rand.Read(peerPubKey)

	// Should not return error when adding peer key
	err = nt.AddNoiseKeyForPeer(peerAddr, peerPubKey)
	assert.NoError(t, err)

	// Set the peer version manually (as would happen after negotiation)
	nt.SetPeerVersion(peerAddr, ProtocolNoiseIK)

	// Verify the peer is now marked as using NoiseIK
	version := nt.getPeerVersion(peerAddr)
	assert.Equal(t, ProtocolNoiseIK, version)
}

// TestDefaultProtocolCapabilities_Negotiating tests that default capabilities are reasonable.
func TestDefaultProtocolCapabilities_Negotiating(t *testing.T) {
	caps := DefaultProtocolCapabilities()

	assert.NotNil(t, caps)
	assert.Contains(t, caps.SupportedVersions, ProtocolLegacy)
	assert.Contains(t, caps.SupportedVersions, ProtocolNoiseIK)
	assert.Equal(t, ProtocolNoiseIK, caps.PreferredVersion)
	assert.False(t, caps.EnableLegacyFallback) // Secure-by-default: legacy fallback disabled
	assert.Greater(t, caps.NegotiationTimeout.Nanoseconds(), int64(0))
}

// TestNegotiatingTransport_NewWithInvalidKey tests error handling for invalid static key.
func TestNegotiatingTransport_NewWithInvalidKey(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	tests := []struct {
		name      string
		keyLength int
		expectErr bool
	}{
		{
			name:      "empty key",
			keyLength: 0,
			expectErr: true,
		},
		{
			name:      "short key",
			keyLength: 16,
			expectErr: true,
		},
		{
			name:      "valid 32 byte key",
			keyLength: 32,
			expectErr: false,
		},
		{
			name:      "too long key",
			keyLength: 64,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			staticPrivKey := make([]byte, tt.keyLength)
			if tt.keyLength > 0 {
				rand.Read(staticPrivKey)
			}

			nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, nt)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, nt)
				if nt != nil {
					nt.Close()
				}
			}
		})
	}
}

// TestNegotiatingTransport_NewWithEmptyVersions tests error handling for empty supported versions.
func TestNegotiatingTransport_NewWithEmptyVersions(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	caps := &ProtocolCapabilities{
		SupportedVersions:    []ProtocolVersion{},
		PreferredVersion:     ProtocolLegacy,
		EnableLegacyFallback: true,
	}

	nt, err := NewNegotiatingTransport(mockTransport, caps, staticPrivKey)
	assert.Error(t, err)
	assert.Nil(t, nt)
}

// TestNegotiatingTransport_SetPeerVersion tests the SetPeerVersion method.
func TestNegotiatingTransport_SetPeerVersion(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	require.NoError(t, err)
	defer nt.Close()

	peerAddr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 8080}

	// Initially unknown (returns 255)
	version := nt.getPeerVersion(peerAddr)
	assert.Equal(t, ProtocolVersion(255), version)

	// Set to Legacy
	nt.SetPeerVersion(peerAddr, ProtocolLegacy)
	version = nt.getPeerVersion(peerAddr)
	assert.Equal(t, ProtocolLegacy, version)

	// Update to NoiseIK
	nt.SetPeerVersion(peerAddr, ProtocolNoiseIK)
	version = nt.getPeerVersion(peerAddr)
	assert.Equal(t, ProtocolNoiseIK, version)
}

// TestNegotiatingTransport_Close tests the Close method.
func TestNegotiatingTransport_Close(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	require.NoError(t, err)

	// Should not panic
	err = nt.Close()
	assert.NoError(t, err)

	// Closing again should also be safe
	err = nt.Close()
	assert.NoError(t, err)
}
