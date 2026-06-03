package transport

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransport implements Transport for testing purposes
type mockTransport struct {
	addr net.Addr
}

func (m *mockTransport) Send(packet *Packet, addr net.Addr) error {
	return nil
}

func (m *mockTransport) Close() error {
	return nil
}

func (m *mockTransport) LocalAddr() net.Addr {
	return m.addr
}

func (m *mockTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
}

func (m *mockTransport) IsConnectionOriented() bool {
	return false
}

// newMockTransport creates a new mock transport for testing
func newMockTransport() *mockTransport {
	return &mockTransport{
		addr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}
}

// TestSignatureVerificationFailure verifies that signature verification failures are caught.
func TestSignatureVerificationFailure(t *testing.T) {
	// Test invalid signature detection
	invalidSigData := make([]byte, 32+64+2) // pubkey + sig + min version data
	// Set valid pubkey, invalid signature, valid version data
	copy(invalidSigData[0:32], make([]byte, 32))  // zero pubkey
	copy(invalidSigData[32:96], make([]byte, 64)) // zero signature
	invalidSigData[96] = byte(ProtocolLegacy)
	invalidSigData[97] = 1
	invalidSigData = append(invalidSigData, byte(ProtocolLegacy))

	_, err := ParseSignedVersionNegotiation(invalidSigData)
	require.Error(t, err)

	// Verify it's a SecurityError
	secErr, ok := AsSecurityError(err)
	require.True(t, ok, "expected SecurityError, got %T", err)
	assert.True(t, secErr.IsFatal(), "signature verification failure should be fatal")
	assert.Equal(t, FatalSecurityError, secErr.Category)
	assert.Equal(t, "signature_verification_failed", secErr.Event)
	assert.Equal(t, "version_negotiation", secErr.Path)
}

// TestNegotiationFailureWithoutFallback verifies fatal error when fallback is disabled.
func TestNegotiationFailureWithoutFallback(t *testing.T) {
	// We test handleNegotiationFailure directly without full transport setup
	// since we don't need a valid Noise transport for this test

	// Create a simple mock transport
	mockTransport := newMockTransport()

	// Create capabilities with fallback disabled
	caps := &ProtocolCapabilities{
		SupportedVersions:        []ProtocolVersion{ProtocolNoiseIK},
		PreferredVersion:         ProtocolNoiseIK,
		EnableLegacyFallback:     false, // Fallback disabled
		RequireSignedNegotiation: false,
	}

	// Create negotiating transport WITHOUT noise setup (don't validate keys)
	nt := &NegotiatingTransport{
		underlying:       mockTransport,
		capabilities:     caps,
		negotiator:       createVersionNegotiator(caps, [32]byte{}),
		peerVersions:     make(map[string]peerVersionEntry),
		fallbackEnabled:  false,
		staticPrivateKey: [32]byte{},
	}

	// Simulate negotiation failure
	packet := &Packet{
		PacketType: PacketPingRequest,
		Data:       []byte("test"),
	}
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}

	// handleNegotiationFailure with fallback disabled should return fatal error
	negotiationErr := errors.New("peer does not support Noise-IK")
	err := nt.handleNegotiationFailure(packet, addr, negotiationErr)

	// Verify it's a fatal SecurityError
	require.Error(t, err)
	secErr, ok := AsSecurityError(err)
	require.True(t, ok, "expected SecurityError, got %T", err)
	assert.True(t, secErr.IsFatal(), "negotiation failure without fallback should be fatal")
	assert.Equal(t, FatalSecurityError, secErr.Category)
	assert.Equal(t, "version_negotiation_failed", secErr.Event)
}

// TestNegotiationFailureWithFallback verifies downgrade event when fallback is enabled.
func TestNegotiationFailureWithFallback(t *testing.T) {
	// Create mock transport
	mockTransport := newMockTransport()

	// Create capabilities with fallback enabled
	caps := &ProtocolCapabilities{
		SupportedVersions:        []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		PreferredVersion:         ProtocolNoiseIK,
		EnableLegacyFallback:     true, // Fallback enabled
		RequireSignedNegotiation: false,
	}

	// Create negotiating transport WITHOUT noise setup (don't validate keys)
	nt := &NegotiatingTransport{
		underlying:       mockTransport,
		capabilities:     caps,
		negotiator:       createVersionNegotiator(caps, [32]byte{}),
		peerVersions:     make(map[string]peerVersionEntry),
		fallbackEnabled:  true,
		staticPrivateKey: [32]byte{},
	}

	// Simulate negotiation failure
	packet := &Packet{
		PacketType: PacketPingRequest,
		Data:       []byte("test"),
	}
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}

	// handleNegotiationFailure with fallback enabled should complete successfully
	negotiationErr := errors.New("peer does not support Noise-IK")
	err := nt.handleNegotiationFailure(packet, addr, negotiationErr)

	// Should succeed (packet sent via underlying transport)
	require.NoError(t, err)

	// Verify peer version was set to legacy
	peerVersion := nt.getPeerVersion(addr)
	assert.Equal(t, ProtocolLegacy, peerVersion)
}

// TestDowngradeEventVersusCompatibilityWarning verifies distinct error categories.
func TestDowngradeEventVersusCompatibilityWarning(t *testing.T) {
	// Downgrade event is distinct from compatibility warning
	downgrade := NewDowngradeEvent("fallback", "negotiation", "falling back to legacy", nil)
	warning := NewCompatibilityWarning("ratchet_unavailable", "session", "ratchet not supported", nil)

	assert.False(t, downgrade.IsCompatibilityWarning())
	assert.False(t, downgrade.IsFatal())
	assert.Equal(t, DowngradeEvent, downgrade.Category)

	assert.True(t, warning.IsCompatibilityWarning())
	assert.False(t, warning.IsFatal())
	assert.Equal(t, CompatibilityWarning, warning.Category)
}

// TestVerificationFailurePath verifies verification failure error creation and classification.
func TestVerificationFailurePath(t *testing.T) {
	verifyErr := NewVerificationFailure(
		"peer_key_changed",
		"friend_connection",
		"peer public key changed since last connection",
		errors.New("key mismatch"),
	)

	assert.Equal(t, VerificationFailure, verifyErr.Category)
	assert.Equal(t, "peer_key_changed", verifyErr.Event)
	assert.False(t, verifyErr.IsFatal())
	assert.False(t, verifyErr.IsCompatibilityWarning())

	// Verification failures should be observable in logs
	errorMsg := verifyErr.Error()
	assert.Contains(t, errorMsg, "VerificationFailure")
	assert.Contains(t, errorMsg, "peer_key_changed")
	assert.Contains(t, errorMsg, "friend_connection")
}

// TestExplicitDowngradePath verifies that downgrades are explicit in error messages.
func TestExplicitDowngradePath(t *testing.T) {
	downgrade := NewDowngradeEvent(
		"protocol_negotiation_fallback",
		"transport_selection",
		"negotiation failed, falling back to Legacy from Noise-IK",
		errors.New("peer does not support Noise-IK"),
	)

	errorMsg := downgrade.Error()
	assert.Contains(t, errorMsg, "DowngradeEvent")
	assert.Contains(t, errorMsg, "protocol_negotiation_fallback")
	assert.Contains(t, errorMsg, "transport_selection")
	assert.Contains(t, errorMsg, "Noise-IK")
	assert.Contains(t, errorMsg, "Legacy")
}

// TestSecurityErrorObservability verifies errors are observable in logs with structured fields.
func TestSecurityErrorObservability(t *testing.T) {
	tests := []struct {
		name        string
		se          *SecurityError
		checkFields func(t *testing.T, se *SecurityError)
	}{
		{
			name: "Fatal security error has clear path and reason",
			se: NewFatalSecurityError(
				"signature_verification_failed",
				"version_negotiation",
				"invalid signature on version negotiation packet",
				errors.New("crypto/ed25519 verify failed"),
			),
			checkFields: func(t *testing.T, se *SecurityError) {
				assert.Equal(t, FatalSecurityError, se.Category)
				assert.NotEmpty(t, se.Event)
				assert.NotEmpty(t, se.Path)
				assert.NotEmpty(t, se.Reason)
				assert.NotNil(t, se.Err)
			},
		},
		{
			name: "Downgrade event has clear escalation path",
			se: NewDowngradeEvent(
				"negotiate_fallback",
				"negotiating_transport",
				"peer does not support Noise-IK, falling back to Legacy encryption",
				errors.New("ProtocolNoiseIK not in peer's supported versions"),
			),
			checkFields: func(t *testing.T, se *SecurityError) {
				assert.Equal(t, DowngradeEvent, se.Category)
				assert.Contains(t, se.Event, "fallback")
				assert.Contains(t, se.Path, "transport")
				assert.Contains(t, se.Reason, "Legacy")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFields(t, tt.se)
			// Verify error is observable
			errorMsg := tt.se.Error()
			assert.NotEmpty(t, errorMsg)
			assert.Contains(t, errorMsg, tt.se.Category.String())
		})
	}
}
