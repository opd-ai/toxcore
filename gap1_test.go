package toxcore

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/opd-ai/toxforge/transport"
)

// TestGap1ReadmeVersionNegotiationExample tests that the README.md version negotiation
// example compiles and executes successfully
// Regression test for Gap #1: Non-existent Function Referenced in Version Negotiation Example
func TestGap1ReadmeVersionNegotiationExample(t *testing.T) {
	// Create UDP transport (this part works)
	udp, err := transport.NewUDPTransport(":0")
	if err != nil {
		t.Fatalf("Failed to create UDP transport: %v", err)
	}
	defer udp.Close()

	// Protocol capabilities (this part works)
	capabilities := &transport.ProtocolCapabilities{
		SupportedVersions: []transport.ProtocolVersion{
			transport.ProtocolLegacy,
			transport.ProtocolNoiseIK,
		},
		PreferredVersion:     transport.ProtocolNoiseIK,
		EnableLegacyFallback: true,
		NegotiationTimeout:   5 * time.Second,
	}

	// This is the FIXED line from README.md that should now work
	staticKey := make([]byte, 32)
	rand.Read(staticKey) // Generate 32-byte Curve25519 key

	// This should work with the fix
	_, err = transport.NewNegotiatingTransport(udp, capabilities, staticKey)
	if err != nil {
		t.Errorf("Failed to create negotiating transport: %v", err)
	}
}
