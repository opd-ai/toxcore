package toxcore

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestGap2NegotiatingTransportImplementation is a regression test confirming that
// NewNegotiatingTransport exists and works as documented in README.md
// This addresses Gap #2 from AUDIT.md - the implementation was found to already exist
func TestGap2NegotiatingTransportImplementation(t *testing.T) {
	// Create a UDP transport as shown in documentation
	udpTransport, err := transport.NewUDPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create UDP transport: %v", err)
	}
	defer udpTransport.Close()

	// Create protocol capabilities as shown in documentation
	capabilities := transport.DefaultProtocolCapabilities()

	// Generate a static key for testing
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// This is the exact call documented in README.md that AUDIT.md claims fails
	negotiatingTransport, err := transport.NewNegotiatingTransport(udpTransport, capabilities, keyPair.Private[:])

	if err != nil {
		t.Errorf("NewNegotiatingTransport failed: %v", err)
	}

	if negotiatingTransport == nil {
		t.Error("NewNegotiatingTransport returned nil transport")
	}

	// Verify we can also use default capabilities as documented
	negotiatingTransport2, err := transport.NewNegotiatingTransport(udpTransport, nil, keyPair.Private[:])
	if err != nil {
		t.Errorf("NewNegotiatingTransport with nil capabilities failed: %v", err)
	}

	if negotiatingTransport2 == nil {
		t.Error("NewNegotiatingTransport with nil capabilities returned nil transport")
	}

	t.Log("Gap #2 was already resolved - NewNegotiatingTransport works as documented")
}
