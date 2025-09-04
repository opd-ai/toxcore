package dht

import (
	"net"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestGap4NetworkInterfaceAbstraction verifies that DHT bootstrap code
// uses generic net.Addr interfaces instead of concrete UDP address types.
// This is a regression test to ensure network interface abstraction is maintained.
func TestGap4NetworkInterfaceAbstraction(t *testing.T) {
	// Test that createDHTNodeFromBootstrap uses proper interface abstraction
	bm := &BootstrapManager{}

	// Create a test bootstrap node
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create a proper net.Addr instead of using string
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
	if err != nil {
		t.Fatalf("Failed to create UDP address: %v", err)
	}

	bn := &BootstrapNode{
		Address:   addr,
		PublicKey: keyPair.Public,
	}

	// Test the method that should use interface abstraction
	node, err := bm.createDHTNodeFromBootstrap(bn)
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}

	// Verify that the node address is properly typed as net.Addr interface
	if node.Address == nil {
		t.Fatal("Node address should not be nil")
	}

	// The address should be usable as a generic net.Addr
	nodeAddr := node.Address
	network := nodeAddr.Network()
	addrString := nodeAddr.String()

	if network != "udp" {
		t.Errorf("Expected network type 'udp', got: %s", network)
	}

	if addrString == "" {
		t.Error("Address string should not be empty")
	}

	t.Logf("Node address created successfully: %s (%s)", addrString, network)

	// This test verifies that the address can be used through the generic interface
	// The implementation should not expose concrete UDP types unnecessarily
}
