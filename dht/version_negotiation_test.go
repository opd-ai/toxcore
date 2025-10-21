package dht

import (
	"net"
	"testing"

	"github.com/opd-ai/toxforge/async"
	"github.com/opd-ai/toxforge/crypto"
	"github.com/opd-ai/toxforge/noise"
	"github.com/opd-ai/toxforge/transport"
)

// TestDHTVersionNegotiation tests the version negotiation functionality in DHT packet processing
func TestDHTVersionNegotiation(t *testing.T) {
	t.Run("PeerProtocolVersionTracking", func(t *testing.T) {
		// Create a bootstrap manager with version support
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		routingTable := NewRoutingTable(*selfID, 8)
		mockTransport := async.NewMockTransport("127.0.0.1:33445")

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		// Test setting and getting peer protocol versions
		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}

		// Initially should return legacy version
		version := bm.GetPeerProtocolVersion(addr)
		if version != transport.ProtocolLegacy {
			t.Errorf("Expected ProtocolLegacy for unknown peer, got %v", version)
		}

		// Set a version and verify it's stored
		bm.SetPeerProtocolVersion(addr, transport.ProtocolNoiseIK)
		version = bm.GetPeerProtocolVersion(addr)
		if version != transport.ProtocolNoiseIK {
			t.Errorf("Expected ProtocolNoiseIK after setting, got %v", version)
		}

		// Clear the version and verify it returns to legacy
		bm.ClearPeerProtocolVersion(addr)
		version = bm.GetPeerProtocolVersion(addr)
		if version != transport.ProtocolLegacy {
			t.Errorf("Expected ProtocolLegacy after clearing, got %v", version)
		}
	})

	t.Run("VersionedHandshakePacketHandling", func(t *testing.T) {
		// Create a bootstrap manager with version support
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		routingTable := NewRoutingTable(*selfID, 8)
		mockTransport := async.NewMockTransport("127.0.0.1:33445")

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		// Create a proper Noise handshake message using initiator
		initiatorKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate initiator key pair: %v", err)
		}

		// Create initiator (knows responder's public key)
		initiator, err := noise.NewIKHandshake(initiatorKeyPair.Private[:], keyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatalf("Failed to create noise initiator: %v", err)
		}

		// Generate the initial handshake message
		noiseMessage, _, err := initiator.WriteMessage([]byte("test payload"), nil)
		if err != nil {
			t.Fatalf("Failed to create noise handshake message: %v", err)
		}

		// Create a valid versioned handshake request
		request := &transport.VersionedHandshakeRequest{
			ProtocolVersion:   transport.ProtocolNoiseIK,
			SupportedVersions: []transport.ProtocolVersion{transport.ProtocolLegacy, transport.ProtocolNoiseIK},
			NoiseMessage:      noiseMessage,
			LegacyData:        []byte{},
		}

		requestData, err := transport.SerializeVersionedHandshakeRequest(request)
		if err != nil {
			t.Fatalf("Failed to serialize handshake request: %v", err)
		}

		packet := &transport.Packet{
			PacketType: transport.PacketNoiseHandshake,
			Data:       requestData,
		}

		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}

		// Handle the versioned handshake packet
		err = bm.handleVersionedHandshakePacket(packet, addr)
		if err != nil {
			t.Errorf("Failed to handle versioned handshake packet: %v", err)
		}

		// Verify that a response was sent
		sentPackets := mockTransport.GetPackets()
		if len(sentPackets) == 0 {
			t.Error("Expected handshake response to be sent")
		} else {
			// Note: We can't access packet details due to unexported fields in MockPacketSend
			// Just verify that a packet was sent
			t.Logf("Handshake response sent successfully (%d packets)", len(sentPackets))
		}

		// Verify that the protocol version was recorded
		version := bm.GetPeerProtocolVersion(addr)
		if version == transport.ProtocolLegacy {
			t.Error("Expected negotiated version to be recorded, but got ProtocolLegacy")
		}
	})

	t.Run("VersionAwareNodeParsing", func(t *testing.T) {
		// Create a bootstrap manager
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		routingTable := NewRoutingTable(*selfID, 8)
		mockTransport := async.NewMockTransport("127.0.0.1:33445")

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		// Create a mock send_nodes packet with legacy format
		senderPK := keyPair.Public
		nodePublicKey := [32]byte{1, 2, 3, 4} // Simple test key

		// Legacy format: sender_pk(32) + num_nodes(1) + node_entry(50)
		packetData := make([]byte, 32+1+50)
		copy(packetData[:32], senderPK[:])
		packetData[32] = 1 // One node

		// Node entry: public_key(32) + IP(16) + port(2)
		nodeOffset := 33
		copy(packetData[nodeOffset:nodeOffset+32], nodePublicKey[:])

		// IPv4-mapped IPv6 address for 127.0.0.1:8080
		ipBytes := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 127, 0, 0, 1}
		copy(packetData[nodeOffset+32:nodeOffset+48], ipBytes)
		packetData[nodeOffset+48] = 0x1f // Port 8080 high byte
		packetData[nodeOffset+49] = 0x90 // Port 8080 low byte

		packet := &transport.Packet{
			PacketType: transport.PacketSendNodes,
			Data:       packetData,
		}

		addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}

		// Handle the send_nodes packet
		err = bm.handleSendNodesPacket(packet, addr)
		if err != nil {
			t.Errorf("Failed to handle send_nodes packet: %v", err)
		}

		// Verify that processing completed without error
		// Note: We can't easily check routing table contents without exposing internal methods
		t.Log("Send nodes packet processed successfully")
	})

	t.Run("VersionAwareResponseBuilding", func(t *testing.T) {
		// Create a bootstrap manager with some nodes
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		routingTable := NewRoutingTable(*selfID, 8)
		mockTransport := async.NewMockTransport("127.0.0.1:33445")

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		// Add a test node to the routing table
		testNodeKey := [32]byte{1, 2, 3, 4}
		testNodeID := crypto.NewToxID(testNodeKey, [4]byte{})
		testNodeAddr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 33445}
		testNode := NewNode(*testNodeID, testNodeAddr)
		routingTable.AddNode(testNode)

		// Test version-aware response building
		nodes := []*Node{testNode}

		// Test legacy response
		legacyResponse := bm.buildVersionedResponseData(nodes, transport.ProtocolLegacy)
		if len(legacyResponse) < 33 { // At least header + one node
			t.Error("Legacy response too short")
		}

		// Test extended response
		extendedResponse := bm.buildVersionedResponseData(nodes, transport.ProtocolNoiseIK)
		if len(extendedResponse) < 33 { // At least header + one node
			t.Error("Extended response too short")
		}

		// The extended response might be different size due to different encoding
		t.Logf("Legacy response size: %d, Extended response size: %d",
			len(legacyResponse), len(extendedResponse))
	})

	t.Run("ProtocolVersionDetection", func(t *testing.T) {
		// Create a bootstrap manager
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		routingTable := NewRoutingTable(*selfID, 8)
		mockTransport := async.NewMockTransport("127.0.0.1:33445")

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}

		// Test detection with no negotiated version
		version := bm.determineResponseProtocolVersion(addr)
		if bm.enableVersioned && version != transport.ProtocolNoiseIK {
			t.Error("Expected ProtocolNoiseIK when versioned handshakes are enabled")
		}

		// Set a negotiated version and test detection
		bm.SetPeerProtocolVersion(addr, transport.ProtocolLegacy)
		version = bm.determineResponseProtocolVersion(addr)
		if version != transport.ProtocolLegacy {
			t.Error("Expected to use negotiated ProtocolLegacy version")
		}

		// Change to extended version
		bm.SetPeerProtocolVersion(addr, transport.ProtocolNoiseIK)
		version = bm.determineResponseProtocolVersion(addr)
		if version != transport.ProtocolNoiseIK {
			t.Error("Expected to use negotiated ProtocolNoiseIK version")
		}
	})

	t.Run("PacketFormatDetection", func(t *testing.T) {
		// Create a bootstrap manager
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
		routingTable := NewRoutingTable(*selfID, 8)
		mockTransport := async.NewMockTransport("127.0.0.1:33445")

		bm := NewBootstrapManagerWithKeyPair(*selfID, keyPair, mockTransport, routingTable)

		// Test legacy packet detection
		senderPK := keyPair.Public

		// Create legacy format packet: sender_pk(32) + num_nodes(1) + node_entry(50)
		legacyPacket := make([]byte, 32+1+50)
		copy(legacyPacket[:32], senderPK[:])
		legacyPacket[32] = 1 // One node

		packet := &transport.Packet{
			PacketType: transport.PacketSendNodes,
			Data:       legacyPacket,
		}

		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}

		// Test protocol version detection from packet
		detectedVersion := bm.detectProtocolVersionFromPacket(packet, addr)
		if detectedVersion != transport.ProtocolLegacy {
			t.Errorf("Expected to detect legacy protocol, got %v", detectedVersion)
		}

		// Test extended format detection (shorter packet that doesn't match legacy format)
		extendedPacket := make([]byte, 32+1+40) // Different size
		copy(extendedPacket[:32], senderPK[:])
		extendedPacket[32] = 1 // One node

		packet.Data = extendedPacket
		detectedVersion = bm.detectProtocolVersionFromPacket(packet, addr)
		if detectedVersion != transport.ProtocolNoiseIK {
			t.Errorf("Expected to detect extended protocol, got %v", detectedVersion)
		}
	})
}

// TestVersionNegotiationWithoutKeyPair tests behavior when versioned handshakes are not available
func TestVersionNegotiationWithoutKeyPair(t *testing.T) {
	// Create a bootstrap manager without key pair (legacy constructor)
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	selfID := crypto.NewToxID(keyPair.Public, [4]byte{})
	routingTable := NewRoutingTable(*selfID, 8)
	mockTransport := async.NewMockTransport("127.0.0.1:33445")

	bm := NewBootstrapManager(*selfID, mockTransport, routingTable)

	// Verify versioned handshakes are disabled
	if bm.IsVersionedHandshakeEnabled() {
		t.Error("Expected versioned handshakes to be disabled for legacy constructor")
	}

	// Test handling versioned handshake packet (should be ignored)
	request := &transport.VersionedHandshakeRequest{
		ProtocolVersion:   transport.ProtocolNoiseIK,
		SupportedVersions: []transport.ProtocolVersion{transport.ProtocolLegacy, transport.ProtocolNoiseIK},
		NoiseMessage:      []byte("test_noise_message"),
		LegacyData:        []byte{},
	}

	requestData, err := transport.SerializeVersionedHandshakeRequest(request)
	if err != nil {
		t.Fatalf("Failed to serialize handshake request: %v", err)
	}

	packet := &transport.Packet{
		PacketType: transport.PacketNoiseHandshake,
		Data:       requestData,
	}

	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}

	// Handle the packet (should not error, but should be ignored)
	err = bm.handleVersionedHandshakePacket(packet, addr)
	if err != nil {
		t.Errorf("handleVersionedHandshakePacket should not error when disabled: %v", err)
	}

	// Verify no response was sent
	sentPackets := mockTransport.GetPackets()
	if len(sentPackets) > 0 {
		t.Error("Expected no response when versioned handshakes are disabled")
	}

	// Verify response protocol version determination defaults to legacy
	version := bm.determineResponseProtocolVersion(addr)
	if version != transport.ProtocolLegacy {
		t.Errorf("Expected ProtocolLegacy when versioned handshakes disabled, got %v", version)
	}
}
