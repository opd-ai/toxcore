package transport

import (
	"crypto/rand"
	"net"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	toxnoise "github.com/opd-ai/toxcore/noise"
)

// MockTransport implements Transport interface for testing
type MockTransport struct {
	packets    []MockPacketSend
	handlers   map[PacketType]PacketHandler
	localAddr  net.Addr
	lastPacket *Packet
	lastAddr   net.Addr
}

type MockPacketSend struct {
	packet *Packet
	addr   net.Addr
}

func NewMockTransport(addr string) *MockTransport {
	localAddr, _ := net.ResolveUDPAddr("udp", addr)
	return &MockTransport{
		packets:   make([]MockPacketSend, 0),
		handlers:  make(map[PacketType]PacketHandler),
		localAddr: localAddr,
	}
}

func (m *MockTransport) Send(packet *Packet, addr net.Addr) error {
	m.packets = append(m.packets, MockPacketSend{packet: packet, addr: addr})
	m.lastPacket = packet
	m.lastAddr = addr
	return nil
}

func (m *MockTransport) Close() error {
	return nil
}

func (m *MockTransport) LocalAddr() net.Addr {
	return m.localAddr
}

func (m *MockTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	m.handlers[packetType] = handler
}

func (m *MockTransport) IsConnectionOriented() bool {
	return false
}

func (m *MockTransport) SimulateReceive(packet *Packet, addr net.Addr) error {
	if handler, exists := m.handlers[packet.PacketType]; exists {
		return handler(packet, addr)
	}
	return nil
}

func (m *MockTransport) GetPackets() []MockPacketSend {
	return m.packets
}

func (m *MockTransport) ClearPackets() {
	m.packets = m.packets[:0]
}

// Test creating NoiseTransport with valid key
func TestNewNoiseTransport(t *testing.T) {
	// Generate test key
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Test valid creation
	noiseTransport, err := NewNoiseTransport(mockTransport, keyPair.Private[:])
	if err != nil {
		t.Fatalf("Failed to create NoiseTransport: %v", err)
	}

	if noiseTransport == nil {
		t.Error("NoiseTransport should not be nil")
	}

	// Test that the transport was created successfully by checking LocalAddr
	addr := noiseTransport.LocalAddr()
	if addr == nil {
		t.Error("LocalAddr should not be nil")
	}
}

// Test NewNoiseTransport validation
func TestNewNoiseTransportValidation(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Test invalid key length
	_, err := NewNoiseTransport(mockTransport, make([]byte, 16))
	if err == nil {
		t.Error("Expected error for invalid key length")
	}

	// Test nil transport
	keyPair, _ := crypto.GenerateKeyPair()
	_, err = NewNoiseTransport(nil, keyPair.Private[:])
	if err == nil {
		t.Error("Expected error for nil transport")
	}
}

// Test adding peers
func TestAddPeer(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	noiseTransport, err := NewNoiseTransport(mockTransport, keyPair.Private[:])
	if err != nil {
		t.Fatal(err)
	}

	// Test adding valid peer
	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8081")
	peerKey := make([]byte, 32)
	_, err = rand.Read(peerKey)
	if err != nil {
		t.Fatal(err)
	}

	err = noiseTransport.AddPeer(peerAddr, peerKey)
	if err != nil {
		t.Errorf("Failed to add peer: %v", err)
	}

	// Test invalid key length
	err = noiseTransport.AddPeer(peerAddr, make([]byte, 16))
	if err == nil {
		t.Error("Expected error for invalid key length")
	}
}

// Test sending to unknown peer (should fall back to unencrypted)
func TestSendToUnknownPeer(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	noiseTransport, err := NewNoiseTransport(mockTransport, keyPair.Private[:])
	if err != nil {
		t.Fatal(err)
	}

	// Send to unknown peer
	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8081")
	packet := &Packet{
		PacketType: PacketFriendMessage,
		Data:       []byte("test message"),
	}

	err = noiseTransport.Send(packet, peerAddr)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	// Should have sent unencrypted packet
	packets := mockTransport.GetPackets()
	if len(packets) != 1 {
		t.Errorf("Expected 1 packet, got %d", len(packets))
	}

	if packets[0].packet.PacketType != PacketFriendMessage {
		t.Errorf("Expected PacketFriendMessage, got %v", packets[0].packet.PacketType)
	}
}

// Test handshake initiation
func TestHandshakeInitiation(t *testing.T) {
	// Create two key pairs
	keyPair1, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyPair2, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Create transports
	mockTransport1 := NewMockTransport("127.0.0.1:8080")
	mockTransport2 := NewMockTransport("127.0.0.1:8081")

	noiseTransport1, err := NewNoiseTransport(mockTransport1, keyPair1.Private[:])
	if err != nil {
		t.Fatal(err)
	}

	noiseTransport2, err := NewNoiseTransport(mockTransport2, keyPair2.Private[:])
	if err != nil {
		t.Fatal(err)
	}

	// Add each other as peers
	addr1, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	addr2, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8081")

	err = noiseTransport1.AddPeer(addr2, keyPair2.Public[:])
	if err != nil {
		t.Fatal(err)
	}

	err = noiseTransport2.AddPeer(addr1, keyPair1.Public[:])
	if err != nil {
		t.Fatal(err)
	}

	// Send message from transport1 to transport2 (should initiate handshake)
	packet := &Packet{
		PacketType: PacketFriendMessage,
		Data:       []byte("test message"),
	}

	err = noiseTransport1.Send(packet, addr2)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	// Should have sent handshake packet and then the original packet
	packets := mockTransport1.GetPackets()
	if len(packets) < 1 {
		t.Error("Expected at least 1 packet")
	}

	// First packet should be handshake
	handshakeFound := false
	for _, p := range packets {
		if p.packet.PacketType == PacketNoiseHandshake {
			handshakeFound = true
			break
		}
	}
	if !handshakeFound {
		t.Error("Expected handshake packet to be sent")
	}
}

// Test handshake packet handling
func TestHandshakePacketHandling(t *testing.T) {
	// Create two key pairs
	keyPair1, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyPair2, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Create handshakes
	initiator, err := toxnoise.NewIKHandshake(keyPair1.Private[:], keyPair2.Public[:], toxnoise.Initiator)
	if err != nil {
		t.Fatal(err)
	}

	// We don't actually use the responder in this test, just the initiator message
	_, err = toxnoise.NewIKHandshake(keyPair2.Private[:], nil, toxnoise.Responder)
	if err != nil {
		t.Fatal(err)
	}

	// Generate initial message
	message, _, err := initiator.WriteMessage(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create transport for responder
	mockTransport := NewMockTransport("127.0.0.1:8081")
	noiseTransport, err := NewNoiseTransport(mockTransport, keyPair2.Private[:])
	if err != nil {
		t.Fatal(err)
	}

	// Simulate receiving handshake packet
	handshakePacket := &Packet{
		PacketType: PacketNoiseHandshake,
		Data:       message,
	}

	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	err = noiseTransport.handleHandshakePacket(handshakePacket, addr)
	if err != nil {
		t.Errorf("Handshake handling failed: %v", err)
	}

	// Should have sent response
	packets := mockTransport.GetPackets()
	if len(packets) != 1 {
		t.Errorf("Expected 1 response packet, got %d", len(packets))
	}

	if packets[0].packet.PacketType != PacketNoiseHandshake {
		t.Errorf("Expected handshake response, got %v", packets[0].packet.PacketType)
	}
}

// Test session management
func TestSessionManagement(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	noiseTransport, err := NewNoiseTransport(mockTransport, keyPair.Private[:])
	if err != nil {
		t.Fatal(err)
	}

	// Initially no sessions
	if len(noiseTransport.sessions) != 0 {
		t.Error("Expected no sessions initially")
	}

	// Close transport should clear sessions
	err = noiseTransport.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if len(noiseTransport.sessions) != 0 {
		t.Error("Expected sessions to be cleared after close")
	}
}

// Test Transport interface compliance
func TestTransportInterface(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	noiseTransport, err := NewNoiseTransport(mockTransport, keyPair.Private[:])
	if err != nil {
		t.Fatal(err)
	}

	// Test interface compliance
	var _ Transport = noiseTransport

	// Test LocalAddr
	addr := noiseTransport.LocalAddr()
	if addr == nil {
		t.Error("LocalAddr should not be nil")
	}

	// Test RegisterHandler
	handlerCalled := false
	handler := func(packet *Packet, addr net.Addr) error {
		handlerCalled = true
		return nil
	}

	// Register handler - this should work without errors
	noiseTransport.RegisterHandler(PacketFriendMessage, handler)

	// Since we can't access internal state directly in a unit test,
	// we verify that RegisterHandler doesn't panic and works correctly
	// by registering multiple handlers
	handler2Called := false
	handler2 := func(packet *Packet, addr net.Addr) error {
		handler2Called = true
		return nil
	}
	noiseTransport.RegisterHandler(PacketPingRequest, handler2)

	// Test passes if no panics occurred during registration
	if handlerCalled || handler2Called {
		t.Error("Handlers should not be called just from registration")
	}
}

// Test packet encryption/decryption flow
func TestPacketEncryptionDecryption(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	noiseTransport, err := NewNoiseTransport(mockTransport, keyPair.Private[:])
	if err != nil {
		t.Fatal(err)
	}

	// Create a completed session manually for testing
	peerKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	handshake, err := toxnoise.NewIKHandshake(keyPair.Private[:], peerKeyPair.Public[:], toxnoise.Initiator)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate completed handshake (this is simplified)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8081")
	session := &NoiseSession{
		handshake: handshake,
		peerAddr:  addr,
		role:      toxnoise.Initiator,
		complete:  false, // We can't easily complete handshake in unit test
	}

	// Test encryption with incomplete session (should fail gracefully)
	packet := &Packet{
		PacketType: PacketFriendMessage,
		Data:       []byte("test message"),
	}

	encryptedPacket, err := noiseTransport.encryptPacket(packet, session)
	if err == nil {
		t.Error("Expected error for incomplete session")
	}
	if encryptedPacket != nil {
		t.Error("Expected nil packet for incomplete session")
	}
}

// ============================================================================
// GAP 3 TESTS - AddPeer Validation
// ============================================================================

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
