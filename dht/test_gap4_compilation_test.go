package dht

import (
	"encoding/hex"
	"net"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestGap4CompilationAfterFix verifies that the network interface abstraction
// bug has been fixed and the code compiles successfully.
func TestGap4CompilationAfterFix(t *testing.T) {
	// This test reproduces the fix for Gap #4: network interface abstraction

	// Create test setup
	selfID := createTestToxID(1)
	addr := newMockAddr("local:1234")
	transport := newMockTransport(addr)
	routingTable := NewRoutingTable(selfID, 8)

	// Create a bootstrap manager
	bm := NewBootstrapManager(selfID, transport, routingTable)

	// Create a proper net.Addr (this would have caused a compilation error before the fix)
	bootstrapAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
	if err != nil {
		t.Fatalf("Failed to create UDP address: %v", err)
	}

	// Generate a test key pair and convert to hex
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	publicKeyHex := hex.EncodeToString(keyPair.Public[:])

	// This call would have failed to compile before the fix due to type mismatch
	// The AddNode method now correctly accepts net.Addr instead of string+port
	err = bm.AddNode(bootstrapAddr, publicKeyHex)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Verify the node was added correctly
	nodes := bm.GetNodes()
	if len(nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(nodes))
	}

	// Verify the address is stored as net.Addr interface
	node := nodes[0]
	if node.Address == nil {
		t.Fatal("Node address should not be nil")
	}

	// Verify the address matches what we provided
	if node.Address.String() != bootstrapAddr.String() {
		t.Errorf("Expected address %s, got %s", bootstrapAddr.String(), node.Address.String())
	}

	// Verify the address implements net.Addr interface correctly
	network := node.Address.Network()
	if network != "udp" {
		t.Errorf("Expected network type 'udp', got: %s", network)
	}

	t.Log("Gap #4 fix verified: network interface abstraction works correctly")
}
