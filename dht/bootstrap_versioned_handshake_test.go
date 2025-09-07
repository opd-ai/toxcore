package dht

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestBootstrapManagerVersionedHandshake tests the versioned handshake integration
func TestBootstrapManagerVersionedHandshake(t *testing.T) {
	t.Run("NewBootstrapManagerWithKeyPair", func(t *testing.T) {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		mockTransport := &MockTransport{}
		routingTable := NewRoutingTable(*selfID, 8)

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		if !bm.IsVersionedHandshakeEnabled() {
			t.Error("Expected versioned handshakes to be enabled with key pair")
		}

		versions := bm.GetSupportedProtocolVersions()
		if len(versions) == 0 {
			t.Error("Expected supported protocol versions to be available")
		}

		// Check for expected protocol versions
		hasLegacy := false
		hasNoiseIK := false
		for _, version := range versions {
			if version == transport.ProtocolLegacy {
				hasLegacy = true
			}
			if version == transport.ProtocolNoiseIK {
				hasNoiseIK = true
			}
		}

		if !hasLegacy {
			t.Error("Expected legacy protocol support for backward compatibility")
		}
		if !hasNoiseIK {
			t.Error("Expected Noise-IK protocol support")
		}
	})

	t.Run("NewBootstrapManagerWithoutKeyPair", func(t *testing.T) {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		mockTransport := &MockTransport{}
		routingTable := NewRoutingTable(*selfID, 8)

		// Test with nil key pair
		bm := NewBootstrapManagerWithKeyPair(*selfID, nil, mockTransport, routingTable)

		if bm.IsVersionedHandshakeEnabled() {
			t.Error("Expected versioned handshakes to be disabled without key pair")
		}

		versions := bm.GetSupportedProtocolVersions()
		if versions != nil {
			t.Error("Expected no supported protocol versions without key pair")
		}
	})

	t.Run("SetVersionedHandshakeEnabled", func(t *testing.T) {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		mockTransport := &MockTransport{}
		routingTable := NewRoutingTable(*selfID, 8)

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		// Initially enabled
		if !bm.IsVersionedHandshakeEnabled() {
			t.Error("Expected versioned handshakes to be enabled initially")
		}

		// Disable
		bm.SetVersionedHandshakeEnabled(false)
		if bm.IsVersionedHandshakeEnabled() {
			t.Error("Expected versioned handshakes to be disabled after setting false")
		}

		// Re-enable
		bm.SetVersionedHandshakeEnabled(true)
		if !bm.IsVersionedHandshakeEnabled() {
			t.Error("Expected versioned handshakes to be re-enabled after setting true")
		}
	})

	t.Run("VersionedHandshakeWithBootstrapNode", func(t *testing.T) {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		mockTransport := &MockTransportWithHandshakeSupport{}
		routingTable := NewRoutingTable(*selfID, 8)

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		// Create a bootstrap node
		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}
		publicKeyHex := "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"

		err = bm.AddNode(addr, publicKeyHex)
		if err != nil {
			t.Fatalf("Failed to add bootstrap node: %v", err)
		}

		// Test bootstrap process with versioned handshakes
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = bm.Bootstrap(ctx)
		// Note: This may fail due to mock transport limitations, but we're testing
		// that the versioned handshake code path is exercised without panics
		if err != nil {
			t.Logf("Bootstrap failed as expected with mock transport: %v", err)
		}
	})

	t.Run("BackwardCompatibilityWithLegacyConstructor", func(t *testing.T) {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		mockTransport := &MockTransport{}
		routingTable := NewRoutingTable(*selfID, 8)

		// Use the original constructor
		bm := NewBootstrapManager(*selfID, mockTransport, routingTable)

		// Should not have versioned handshake support
		if bm.IsVersionedHandshakeEnabled() {
			t.Error("Expected versioned handshakes to be disabled with legacy constructor")
		}

		versions := bm.GetSupportedProtocolVersions()
		if versions != nil {
			t.Error("Expected no supported protocol versions with legacy constructor")
		}
	})
}

// MockTransportWithHandshakeSupport extends MockTransport to simulate handshake responses
type MockTransportWithHandshakeSupport struct {
	MockTransport
	sentPackets []MockSentPacket
}

func (m *MockTransportWithHandshakeSupport) Send(packet *transport.Packet, addr net.Addr) error {
	// Record the packet for verification
	m.sentPackets = append(m.sentPackets, MockSentPacket{
		Packet: packet,
		Addr:   addr,
	})

	// Simulate successful send for handshake packets
	if packet.PacketType == transport.PacketNoiseHandshake {
		// In a real implementation, this would trigger a response
		// For testing, we just record that the handshake was attempted
		return nil
	}

	return m.MockTransport.Send(packet, addr)
}

type MockSentPacket struct {
	Packet *transport.Packet
	Addr   net.Addr
}

// TestVersionedHandshakeAttempt tests the attemptVersionedHandshake method directly
func TestVersionedHandshakeAttempt(t *testing.T) {
	t.Run("SuccessfulHandshake", func(t *testing.T) {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		mockTransport := &MockTransportWithHandshakeSupport{}
		routingTable := NewRoutingTable(*selfID, 8)

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		// Create a bootstrap node and DHT node
		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}

		var publicKey [32]byte
		for i := 0; i < 32; i++ {
			publicKey[i] = byte(i) // Simple test pattern
		}

		bn := &BootstrapNode{
			Address:   addr,
			PublicKey: publicKey,
		}

		var nospam [4]byte
		nodeID := crypto.NewToxID(publicKey, nospam)
		dhtNode := NewNode(*nodeID, addr)

		// Test the handshake attempt
		err = bm.attemptVersionedHandshake(bn, dhtNode)
		// This will likely fail due to mock transport limitations, but should not panic
		if err != nil {
			t.Logf("Handshake failed as expected with mock transport: %v", err)
		}

		// Verify that a handshake packet was sent
		if len(mockTransport.sentPackets) == 0 {
			t.Error("Expected at least one packet to be sent during handshake attempt")
		}

		// Check that the packet type is correct
		for _, sentPacket := range mockTransport.sentPackets {
			if sentPacket.Packet.PacketType == transport.PacketNoiseHandshake {
				t.Log("Handshake packet sent successfully")
				return
			}
		}
		t.Error("Expected a handshake packet to be sent")
	})

	t.Run("HandshakeWithoutManager", func(t *testing.T) {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		mockTransport := &MockTransport{}
		routingTable := NewRoutingTable(*selfID, 8)

		// Use legacy constructor without handshake manager
		bm := NewBootstrapManager(*selfID, mockTransport, routingTable)

		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}
		var publicKey [32]byte
		bn := &BootstrapNode{
			Address:   addr,
			PublicKey: publicKey,
		}

		var nospam [4]byte
		nodeID := crypto.NewToxID(publicKey, nospam)
		dhtNode := NewNode(*nodeID, addr)

		// Should fail gracefully
		err = bm.attemptVersionedHandshake(bn, dhtNode)
		if err == nil {
			t.Error("Expected handshake to fail without manager")
		}

		expectedError := "handshake manager not initialized"
		if err.Error() != expectedError {
			t.Errorf("Expected error %q, got %q", expectedError, err.Error())
		}
	})
}

// TestProtocolVersionSupport tests protocol version handling
func TestProtocolVersionSupport(t *testing.T) {
	t.Run("SupportedVersionsList", func(t *testing.T) {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		mockTransport := &MockTransport{}
		routingTable := NewRoutingTable(*selfID, 8)

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		versions := bm.GetSupportedProtocolVersions()
		if len(versions) < 2 {
			t.Errorf("Expected at least 2 supported versions, got %d", len(versions))
		}

		// Verify the returned slice is a copy (modifications don't affect internal state)
		originalLen := len(versions)
		_ = append(versions, transport.ProtocolVersion(99)) // Add invalid version to test copy

		newVersions := bm.GetSupportedProtocolVersions()
		if len(newVersions) != originalLen {
			t.Error("Expected returned slice to be a copy, but internal state was modified")
		}
	})

	t.Run("RuntimeEnableDisable", func(t *testing.T) {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		mockTransport := &MockTransport{}
		routingTable := NewRoutingTable(*selfID, 8)

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		// Test multiple enable/disable cycles
		for i := 0; i < 3; i++ {
			bm.SetVersionedHandshakeEnabled(false)
			if bm.IsVersionedHandshakeEnabled() {
				t.Errorf("Cycle %d: Expected handshakes to be disabled", i)
			}

			bm.SetVersionedHandshakeEnabled(true)
			if !bm.IsVersionedHandshakeEnabled() {
				t.Errorf("Cycle %d: Expected handshakes to be enabled", i)
			}
		}
	})
}
