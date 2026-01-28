package async

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestPreKeyExchangeOverNetwork verifies that pre-key exchange packets are sent over the network
func TestPreKeyExchangeOverNetwork(t *testing.T) {
	// Create two key pairs (Alice and Bob)
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Alice's key pair: %v", err)
	}

	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Bob's key pair: %v", err)
	}

	// Create mock transports
	aliceTransport := NewMockTransport("127.0.0.1:5000")
	bobTransport := NewMockTransport("127.0.0.1:5001")

	// Create async managers
	aliceDir := t.TempDir()
	bobDir := t.TempDir()

	aliceManager, err := NewAsyncManager(aliceKeyPair, aliceTransport, aliceDir)
	if err != nil {
		t.Fatalf("Failed to create Alice's async manager: %v", err)
	}

	bobManager, err := NewAsyncManager(bobKeyPair, bobTransport, bobDir)
	if err != nil {
		t.Fatalf("Failed to create Bob's async manager: %v", err)
	}

	// Set friend addresses
	aliceAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5000")
	bobAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5001")

	aliceManager.SetFriendAddress(bobKeyPair.Public, bobAddr)
	bobManager.SetFriendAddress(aliceKeyPair.Public, aliceAddr)

	// Track received packets  
	var bobReceivedPreKey bool

	// Set up mock transport to capture sent packets and deliver them
	aliceTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncPreKeyExchange {
			bobReceivedPreKey = true
			// Simulate delivery to Bob by calling his handler
			go bobManager.handlePreKeyExchangePacket(packet, aliceAddr)
		}
		return nil
	}

	bobTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncPreKeyExchange {
			// Simulate delivery to Alice by calling her handler
			go aliceManager.handlePreKeyExchangePacket(packet, bobAddr)
		}
		return nil
	}

	// Trigger pre-key exchange by setting Bob as online for Alice
	aliceManager.SetFriendOnlineStatus(bobKeyPair.Public, true)

	// Wait for pre-key exchange to complete
	time.Sleep(100 * time.Millisecond)

	// Verify Alice sent pre-key packet to Bob
	if !bobReceivedPreKey {
		t.Error("Alice should have sent pre-key exchange packet to Bob")
	}

	// Verify Bob can now send async messages to Alice
	if !bobManager.CanSendAsyncMessage(aliceKeyPair.Public) {
		t.Error("Bob should be able to send async messages to Alice after key exchange")
	}
}

// TestPreKeyPacketFormat verifies the pre-key packet format is correct
func TestPreKeyPacketFormat(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	dir := t.TempDir()
	transport := NewMockTransport("127.0.0.1:6000")

	manager, err := NewAsyncManager(keyPair, transport, dir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	// Generate pre-keys
	peerKeyPair, _ := crypto.GenerateKeyPair()
	if err := manager.forwardSecurity.GeneratePreKeysForPeer(peerKeyPair.Public); err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Create pre-key exchange
	exchange, err := manager.forwardSecurity.ExchangePreKeys(peerKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to create pre-key exchange: %v", err)
	}

	// Create packet
	packet, err := manager.createPreKeyExchangePacket(exchange)
	if err != nil {
		t.Fatalf("Failed to create pre-key packet: %v", err)
	}

	// Verify packet structure
	if len(packet) < 4+1+32+2+32+32 {
		t.Fatalf("Packet too small: %d bytes", len(packet))
	}

	// Verify magic bytes
	if string(packet[0:4]) != "PKEY" {
		t.Error("Invalid magic bytes in packet")
	}

	// Verify version
	if packet[4] != 1 {
		t.Errorf("Invalid version: expected 1, got %d", packet[4])
	}

	// Verify sender public key
	var senderPK [32]byte
	copy(senderPK[:], packet[5:37])
	if senderPK != keyPair.Public {
		t.Error("Sender public key in packet doesn't match manager's public key")
	}

	// Verify key count
	keyCount := uint16(packet[37])<<8 | uint16(packet[38])
	if keyCount != uint16(len(exchange.PreKeys)) {
		t.Errorf("Key count mismatch: expected %d, got %d", len(exchange.PreKeys), keyCount)
	}
}

// TestPreKeyPacketParsing verifies that pre-key packets can be parsed correctly
func TestPreKeyPacketParsing(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	dir := t.TempDir()
	transport := NewMockTransport("127.0.0.1:6000")

	manager, err := NewAsyncManager(keyPair, transport, dir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	// Generate pre-keys
	peerKeyPair, _ := crypto.GenerateKeyPair()
	if err := manager.forwardSecurity.GeneratePreKeysForPeer(peerKeyPair.Public); err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Create pre-key exchange
	exchange, err := manager.forwardSecurity.ExchangePreKeys(peerKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to create pre-key exchange: %v", err)
	}

	// Create packet
	packet, err := manager.createPreKeyExchangePacket(exchange)
	if err != nil {
		t.Fatalf("Failed to create pre-key packet: %v", err)
	}

	// Parse packet
	parsedExchange, senderPK, err := manager.parsePreKeyExchangePacket(packet)
	if err != nil {
		t.Fatalf("Failed to parse pre-key packet: %v", err)
	}

	// Verify sender PK
	if senderPK != keyPair.Public {
		t.Error("Parsed sender PK doesn't match original")
	}

	// Verify key count
	if len(parsedExchange.PreKeys) != len(exchange.PreKeys) {
		t.Errorf("Key count mismatch: expected %d, got %d", len(exchange.PreKeys), len(parsedExchange.PreKeys))
	}

	// Verify keys match
	for i := range exchange.PreKeys {
		if parsedExchange.PreKeys[i].PublicKey != exchange.PreKeys[i].PublicKey {
			t.Errorf("Pre-key %d doesn't match", i)
		}
	}
}

// TestPreKeyExchangeWithoutFriendAddress verifies error handling when friend address is not set
func TestPreKeyExchangeWithoutFriendAddress(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	peerKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate peer key pair: %v", err)
	}

	dir := t.TempDir()
	transport := NewMockTransport("127.0.0.1:6000")

	manager, err := NewAsyncManager(keyPair, transport, dir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	// Generate pre-keys
	if err := manager.forwardSecurity.GeneratePreKeysForPeer(peerKeyPair.Public); err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Create pre-key exchange
	exchange, err := manager.forwardSecurity.ExchangePreKeys(peerKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to create pre-key exchange: %v", err)
	}

	// Try to send without setting friend address
	err = manager.sendPreKeyExchange(peerKeyPair.Public, exchange)
	if err == nil {
		t.Error("Should fail when friend address is not set")
	}
}

// TestPreKeyExchangeWithNilTransport verifies error handling when transport is nil
func TestPreKeyExchangeWithNilTransport(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	peerKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate peer key pair: %v", err)
	}

	dir := t.TempDir()

	// Create manager with nil transport
	manager, err := NewAsyncManager(keyPair, nil, dir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	// Set friend address
	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5000")
	manager.SetFriendAddress(peerKeyPair.Public, peerAddr)

	// Generate pre-keys
	if err := manager.forwardSecurity.GeneratePreKeysForPeer(peerKeyPair.Public); err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Create pre-key exchange
	exchange, err := manager.forwardSecurity.ExchangePreKeys(peerKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to create pre-key exchange: %v", err)
	}

	// Try to send with nil transport
	err = manager.sendPreKeyExchange(peerKeyPair.Public, exchange)
	if err == nil {
		t.Error("Should fail when transport is nil")
	}
}

// TestBidirectionalPreKeyExchange verifies both peers can exchange keys
func TestBidirectionalPreKeyExchange(t *testing.T) {
	// Create two key pairs
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Alice's key pair: %v", err)
	}

	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Bob's key pair: %v", err)
	}

	// Create mock transports
	aliceTransport := NewMockTransport("127.0.0.1:5000")
	bobTransport := NewMockTransport("127.0.0.1:5001")

	// Create async managers
	aliceDir := t.TempDir()
	bobDir := t.TempDir()

	aliceManager, err := NewAsyncManager(aliceKeyPair, aliceTransport, aliceDir)
	if err != nil {
		t.Fatalf("Failed to create Alice's async manager: %v", err)
	}

	bobManager, err := NewAsyncManager(bobKeyPair, bobTransport, bobDir)
	if err != nil {
		t.Fatalf("Failed to create Bob's async manager: %v", err)
	}

	// Set friend addresses
	aliceAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5000")
	bobAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5001")

	aliceManager.SetFriendAddress(bobKeyPair.Public, bobAddr)
	bobManager.SetFriendAddress(aliceKeyPair.Public, aliceAddr)

	// Set up bidirectional packet delivery
	bobTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncPreKeyExchange {
			go aliceManager.handlePreKeyExchangePacket(packet, bobAddr)
		}
		return nil
	}

	aliceTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncPreKeyExchange {
			go bobManager.handlePreKeyExchangePacket(packet, aliceAddr)
		}
		return nil
	}

	// Both peers come online
	aliceManager.SetFriendOnlineStatus(bobKeyPair.Public, true)
	bobManager.SetFriendOnlineStatus(aliceKeyPair.Public, true)

	// Wait for exchanges to complete
	time.Sleep(200 * time.Millisecond)

	// Verify both can send async messages
	if !bobManager.CanSendAsyncMessage(aliceKeyPair.Public) {
		t.Error("Bob should be able to send async messages to Alice")
	}

	if !aliceManager.CanSendAsyncMessage(bobKeyPair.Public) {
		t.Error("Alice should be able to send async messages to Bob")
	}
}

// TestInvalidPreKeyPackets verifies rejection of malformed packets
func TestInvalidPreKeyPackets(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	dir := t.TempDir()
	transport := NewMockTransport("127.0.0.1:6000")

	manager, err := NewAsyncManager(keyPair, transport, dir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	tests := []struct {
		name   string
		packet []byte
	}{
		{
			name:   "Too small",
			packet: []byte{1, 2, 3},
		},
		{
			name:   "Invalid magic",
			packet: make([]byte, 150),
		},
		{
			name: "Wrong version",
			packet: func() []byte {
				p := make([]byte, 150)
				copy(p[0:4], []byte("PKEY"))
				p[4] = 99 // Invalid version
				return p
			}(),
		},
		{
			name: "Zero key count",
			packet: func() []byte {
				p := make([]byte, 150)
				copy(p[0:4], []byte("PKEY"))
				p[4] = 1 // Version
				// key count at p[37:39] is already zero
				return p
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := manager.parsePreKeyExchangePacket(tt.packet)
			if err == nil {
				t.Error("Should reject invalid packet")
			}
		})
	}
}

// TestSetFriendAddress verifies friend address management
func TestSetFriendAddress(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	friendKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate friend key pair: %v", err)
	}

	dir := t.TempDir()
	transport := NewMockTransport("127.0.0.1:6000")

	manager, err := NewAsyncManager(keyPair, transport, dir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	// Set friend address
	friendAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5000")
	manager.SetFriendAddress(friendKeyPair.Public, friendAddr)

	// Verify address is stored
	manager.mutex.RLock()
	storedAddr, exists := manager.friendAddresses[friendKeyPair.Public]
	manager.mutex.RUnlock()

	if !exists {
		t.Error("Friend address should be stored")
	}

	if storedAddr.String() != friendAddr.String() {
		t.Errorf("Stored address %s doesn't match set address %s", storedAddr, friendAddr)
	}
}

// Cleanup test directories
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
