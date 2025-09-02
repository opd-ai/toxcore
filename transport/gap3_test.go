package transport

import (
	"net"
	"testing"
)

// TestGap3AddPeerValidation tests that AddPeer properly validates inputs
// Regression test for Gap #3: Transport AddPeer Method Missing Validation
func TestGap3AddPeerValidation(t *testing.T) {
	// Setup: Create a UDP transport and noise transport
	udpTransport, err := NewUDPTransport(":0")
	if err != nil {
		t.Fatalf("Failed to create UDP transport: %v", err)
	}
	defer udpTransport.Close()

	staticKey := make([]byte, 32)
	staticKey[0] = 1 // Ensure non-zero key
	noiseTransport, err := NewNoiseTransport(udpTransport, staticKey)
	if err != nil {
		t.Fatalf("Failed to create noise transport: %v", err)
	}
	defer noiseTransport.Close()

	t.Run("should reject all-zero public key", func(t *testing.T) {
		validAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
		allZeroKey := make([]byte, 32) // All zeros

		err := noiseTransport.AddPeer(validAddr, allZeroKey)
		if err == nil {
			t.Error("Expected error for all-zero public key, but got none")
		}
	})

	t.Run("should reject incompatible address types", func(t *testing.T) {
		// TCP address should be rejected for UDP-based transport
		tcpAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
		validKey := make([]byte, 32)
		validKey[0] = 1 // Non-zero key

		err := noiseTransport.AddPeer(tcpAddr, validKey)
		if err == nil {
			t.Error("Expected error for TCP address on UDP transport, but got none")
		}
	})

	t.Run("should accept valid UDP address and non-zero key", func(t *testing.T) {
		validAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
		validKey := make([]byte, 32)
		validKey[0] = 1 // Non-zero key

		err := noiseTransport.AddPeer(validAddr, validKey)
		if err != nil {
			t.Errorf("Expected no error for valid inputs, but got: %v", err)
		}
	})
}
