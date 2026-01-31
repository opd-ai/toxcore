package dht

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestGroupPacketHandlerRegistration verifies that group packet handlers are registered during initialization.
func TestGroupPacketHandlerRegistration(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := NewRoutingTable(*toxID, 8)

	// Create a mock transport that tracks registered handlers
	mockTransport := NewMockTransportWithHandlerTracking()

	// Create bootstrap manager (should register handlers)
	_ = NewBootstrapManagerWithKeyPair(*toxID, keyPair, mockTransport, routingTable)

	// Verify that group packet handlers were registered
	expectedHandlers := []transport.PacketType{
		transport.PacketGroupAnnounce,
		transport.PacketGroupQuery,
		transport.PacketGroupQueryResponse,
	}

	for _, packetType := range expectedHandlers {
		if !mockTransport.HasHandler(packetType) {
			t.Errorf("Expected handler for packet type %v to be registered", packetType)
		}
	}
}

// TestGroupQueryResponseHandling verifies end-to-end group query response handling.
func TestGroupQueryResponseHandling(t *testing.T) {
	// Setup two instances: one announces a group, another queries for it
	keyPair1, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	keyPair2, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	var nospam [4]byte
	toxID1 := crypto.NewToxID(keyPair1.Public, nospam)
	toxID2 := crypto.NewToxID(keyPair2.Public, nospam)

	routingTable1 := NewRoutingTable(*toxID1, 8)
	routingTable2 := NewRoutingTable(*toxID2, 8)

	mockTransport1 := NewMockTransportWithHandlerTracking()
	mockTransport2 := NewMockTransportWithHandlerTracking()

	bm1 := NewBootstrapManagerWithKeyPair(*toxID1, keyPair1, mockTransport1, routingTable1)
	_ = NewBootstrapManagerWithKeyPair(*toxID2, keyPair2, mockTransport2, routingTable2)

	// Instance 1: Create and announce a group
	testGroupID := uint32(12345)
	announcement := &GroupAnnouncement{
		GroupID:   testGroupID,
		Name:      "Integration Test Group",
		Type:      0, // ChatTypeText
		Privacy:   0, // PrivacyPublic
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	// Store announcement in instance 1's DHT
	bm1.groupStorage.StoreAnnouncement(announcement)

	// Instance 2: Query for the group
	queryData := make([]byte, 4)
	queryData[0] = byte(testGroupID >> 24)
	queryData[1] = byte(testGroupID >> 16)
	queryData[2] = byte(testGroupID >> 8)
	queryData[3] = byte(testGroupID)

	queryPacket := &transport.Packet{
		PacketType: transport.PacketGroupQuery,
		Data:       queryData,
	}

	addr1 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}

	// Simulate instance 2 sending query to instance 1
	err = bm1.HandleGroupPacket(queryPacket, addr1)
	if err != nil {
		t.Fatalf("Failed to handle group query: %v", err)
	}

	// Verify that instance 1 sent a response
	sentPackets := mockTransport1.GetSentPackets()
	if len(sentPackets) == 0 {
		t.Fatal("Expected instance 1 to send a query response")
	}

	// Find the query response packet
	var responsePacket *transport.Packet
	for _, pkt := range sentPackets {
		if pkt.PacketType == transport.PacketGroupQueryResponse {
			responsePacket = pkt
			break
		}
	}

	if responsePacket == nil {
		t.Fatal("Expected to find a GroupQueryResponse packet")
	}

	// Verify response contains the announcement
	if len(responsePacket.Data) < 2 {
		t.Fatal("Response packet too short")
	}

	if responsePacket.Data[0] != 1 {
		t.Error("Expected response to indicate group was found")
	}
}

// TestGroupAnnounceHandling verifies that group announcements are stored when received.
func TestGroupAnnounceHandling(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := NewRoutingTable(*toxID, 8)
	mockTransport := NewMockTransportWithHandlerTracking()

	bm := NewBootstrapManagerWithKeyPair(*toxID, keyPair, mockTransport, routingTable)

	// Create an announcement packet
	announcement := &GroupAnnouncement{
		GroupID:   67890,
		Name:      "Test Announcement",
		Type:      0,
		Privacy:   0,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	announcementData, err := SerializeAnnouncement(announcement)
	if err != nil {
		t.Fatalf("Failed to serialize announcement: %v", err)
	}

	announcePacket := &transport.Packet{
		PacketType: transport.PacketGroupAnnounce,
		Data:       announcementData,
	}

	senderAddr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 33445}

	// Handle the announcement
	err = bm.HandleGroupPacket(announcePacket, senderAddr)
	if err != nil {
		t.Fatalf("Failed to handle group announcement: %v", err)
	}

	// Verify the announcement was stored
	storedAnnouncement, exists := bm.groupStorage.GetAnnouncement(announcement.GroupID)
	if !exists {
		t.Fatal("Expected announcement to be stored")
	}

	if storedAnnouncement.Name != "Test Announcement" {
		t.Errorf("Expected announcement name 'Test Announcement', got '%s'", storedAnnouncement.Name)
	}
}

// TestGroupQueryResponseCallback verifies that callbacks are triggered on query responses.
func TestGroupQueryResponseCallback(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := NewRoutingTable(*toxID, 8)
	mockTransport := NewMockTransportWithHandlerTracking()

	bm := NewBootstrapManagerWithKeyPair(*toxID, keyPair, mockTransport, routingTable)

	// Register a callback to track responses
	var callbackCalled bool
	var receivedAnnouncement *GroupAnnouncement
	var mu sync.Mutex

	callback := func(announcement *GroupAnnouncement) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		receivedAnnouncement = announcement
	}

	// Set callback on the routing table (where responses are actually processed)
	routingTable.SetGroupResponseCallback(callback)

	// Create a query response packet
	announcement := &GroupAnnouncement{
		GroupID:   99999,
		Name:      "Callback Test Group",
		Type:      0,
		Privacy:   0,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	announcementData, err := SerializeAnnouncement(announcement)
	if err != nil {
		t.Fatalf("Failed to serialize announcement: %v", err)
	}

	// Query response format: [found(1)][announcement_data]
	responseData := append([]byte{1}, announcementData...)

	responsePacket := &transport.Packet{
		PacketType: transport.PacketGroupQueryResponse,
		Data:       responseData,
	}

	senderAddr := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 33445}

	// Handle the query response
	err = bm.HandleGroupPacket(responsePacket, senderAddr)
	if err != nil {
		t.Fatalf("Failed to handle group query response: %v", err)
	}

	// Give callback time to execute
	time.Sleep(50 * time.Millisecond)

	// Verify callback was called
	mu.Lock()
	defer mu.Unlock()

	if !callbackCalled {
		t.Error("Expected callback to be called")
	}

	if receivedAnnouncement == nil {
		t.Fatal("Expected to receive announcement in callback")
	}

	if receivedAnnouncement.Name != "Callback Test Group" {
		t.Errorf("Expected announcement name 'Callback Test Group', got '%s'", receivedAnnouncement.Name)
	}
}

// TestGroupQueryNotFound verifies handling of "not found" query responses.
func TestGroupQueryNotFound(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := NewRoutingTable(*toxID, 8)
	mockTransport := NewMockTransportWithHandlerTracking()

	bm := NewBootstrapManagerWithKeyPair(*toxID, keyPair, mockTransport, routingTable)

	// Query for a group that doesn't exist
	testGroupID := uint32(54321)
	queryData := make([]byte, 4)
	queryData[0] = byte(testGroupID >> 24)
	queryData[1] = byte(testGroupID >> 16)
	queryData[2] = byte(testGroupID >> 8)
	queryData[3] = byte(testGroupID)

	queryPacket := &transport.Packet{
		PacketType: transport.PacketGroupQuery,
		Data:       queryData,
	}

	senderAddr := &net.UDPAddr{IP: net.ParseIP("172.16.0.1"), Port: 33445}

	// Handle the query (group doesn't exist in storage)
	err = bm.HandleGroupPacket(queryPacket, senderAddr)
	if err != nil {
		t.Fatalf("Failed to handle group query: %v", err)
	}

	// Verify that a "not found" response was sent
	sentPackets := mockTransport.GetSentPackets()
	if len(sentPackets) == 0 {
		t.Fatal("Expected a query response to be sent")
	}

	// Find the query response packet
	var responsePacket *transport.Packet
	for _, pkt := range sentPackets {
		if pkt.PacketType == transport.PacketGroupQueryResponse {
			responsePacket = pkt
			break
		}
	}

	if responsePacket == nil {
		t.Fatal("Expected to find a GroupQueryResponse packet")
	}

	// Verify response indicates "not found"
	if len(responsePacket.Data) < 1 {
		t.Fatal("Response packet too short")
	}

	if responsePacket.Data[0] != 0 {
		t.Error("Expected response to indicate group was not found")
	}
}

// TestConcurrentGroupQueryHandling verifies thread safety of group query handling.
func TestConcurrentGroupQueryHandling(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := NewRoutingTable(*toxID, 8)
	mockTransport := NewMockTransportWithHandlerTracking()

	bm := NewBootstrapManagerWithKeyPair(*toxID, keyPair, mockTransport, routingTable)

	// Store multiple announcements
	for i := uint32(1); i <= 10; i++ {
		announcement := &GroupAnnouncement{
			GroupID:   i,
			Name:      "Concurrent Test Group",
			Type:      0,
			Privacy:   0,
			Timestamp: time.Now(),
			TTL:       24 * time.Hour,
		}
		bm.groupStorage.StoreAnnouncement(announcement)
	}

	// Send concurrent queries
	var wg sync.WaitGroup
	for i := uint32(1); i <= 20; i++ {
		wg.Add(1)
		go func(groupID uint32) {
			defer wg.Done()

			queryData := make([]byte, 4)
			queryData[0] = byte(groupID >> 24)
			queryData[1] = byte(groupID >> 16)
			queryData[2] = byte(groupID >> 8)
			queryData[3] = byte(groupID)

			queryPacket := &transport.Packet{
				PacketType: transport.PacketGroupQuery,
				Data:       queryData,
			}

			addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30000 + int(groupID)}

			_ = bm.HandleGroupPacket(queryPacket, addr)
		}(i)
	}

	// Wait for all queries to complete
	wg.Wait()

	// Verify responses were sent
	sentPackets := mockTransport.GetSentPackets()
	if len(sentPackets) != 20 {
		t.Errorf("Expected 20 query responses, got %d", len(sentPackets))
	}
}

// MockTransportWithHandlerTracking is a test transport that tracks registered handlers and sent packets.
type MockTransportWithHandlerTracking struct {
	handlers     map[transport.PacketType]transport.PacketHandler
	handlersMu   sync.RWMutex
	sentPackets  []*transport.Packet
	sentAddrs    []net.Addr
	packetsMu    sync.Mutex
	localAddr    net.Addr
}

// NewMockTransportWithHandlerTracking creates a new mock transport.
func NewMockTransportWithHandlerTracking() *MockTransportWithHandlerTracking {
	return &MockTransportWithHandlerTracking{
		handlers:    make(map[transport.PacketType]transport.PacketHandler),
		sentPackets: make([]*transport.Packet, 0),
		sentAddrs:   make([]net.Addr, 0),
		localAddr:   &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}
}

func (m *MockTransportWithHandlerTracking) Send(packet *transport.Packet, addr net.Addr) error {
	m.packetsMu.Lock()
	defer m.packetsMu.Unlock()
	m.sentPackets = append(m.sentPackets, packet)
	m.sentAddrs = append(m.sentAddrs, addr)
	return nil
}

func (m *MockTransportWithHandlerTracking) Close() error {
	return nil
}

func (m *MockTransportWithHandlerTracking) LocalAddr() net.Addr {
	return m.localAddr
}

func (m *MockTransportWithHandlerTracking) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
	m.handlersMu.Lock()
	defer m.handlersMu.Unlock()
	m.handlers[packetType] = handler
}

func (m *MockTransportWithHandlerTracking) IsConnectionOriented() bool {
	return false
}

func (m *MockTransportWithHandlerTracking) HasHandler(packetType transport.PacketType) bool {
	m.handlersMu.RLock()
	defer m.handlersMu.RUnlock()
	_, exists := m.handlers[packetType]
	return exists
}

func (m *MockTransportWithHandlerTracking) GetSentPackets() []*transport.Packet {
	m.packetsMu.Lock()
	defer m.packetsMu.Unlock()
	// Return a copy to avoid race conditions
	packets := make([]*transport.Packet, len(m.sentPackets))
	copy(packets, m.sentPackets)
	return packets
}

func (m *MockTransportWithHandlerTracking) ClearSentPackets() {
	m.packetsMu.Lock()
	defer m.packetsMu.Unlock()
	m.sentPackets = make([]*transport.Packet, 0)
	m.sentAddrs = make([]net.Addr, 0)
}
