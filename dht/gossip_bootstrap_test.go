package dht

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransportForGossip implements transport.Transport for gossip tests.
type mockTransportForGossip struct {
	mu       sync.Mutex
	packets  []*transport.Packet
	handlers map[transport.PacketType]transport.PacketHandler
}

func newMockTransportForGossip() *mockTransportForGossip {
	return &mockTransportForGossip{
		packets:  make([]*transport.Packet, 0),
		handlers: make(map[transport.PacketType]transport.PacketHandler),
	}
}

func (m *mockTransportForGossip) Send(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packets = append(m.packets, packet)
	return nil
}

func (m *mockTransportForGossip) Receive() (*transport.Packet, net.Addr, error) {
	return nil, nil, nil
}

func (m *mockTransportForGossip) Close() error {
	return nil
}

func (m *mockTransportForGossip) RegisterHandler(pt transport.PacketType, h transport.PacketHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[pt] = h
}

func (m *mockTransportForGossip) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 33445}
}

func (m *mockTransportForGossip) IsConnectionOriented() bool {
	return false
}

func (m *mockTransportForGossip) GetPackets() []*transport.Packet {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*transport.Packet, len(m.packets))
	copy(result, m.packets)
	return result
}

func (m *mockTransportForGossip) InvokeHandler(pt transport.PacketType, packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	h, ok := m.handlers[pt]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return h(packet, addr)
}

// mockTimeProviderForGossip allows controlling time in tests.
type mockTimeProviderForGossip struct {
	now time.Time
	mu  sync.Mutex
}

func newMockTimeProviderForGossip() *mockTimeProviderForGossip {
	return &mockTimeProviderForGossip{now: time.Now()}
}

func (m *mockTimeProviderForGossip) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

func (m *mockTimeProviderForGossip) Since(t time.Time) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now.Sub(t)
}

func (m *mockTimeProviderForGossip) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}

func TestGossipBootstrap_NewGossipBootstrap(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(1)

	gb := NewGossipBootstrap(selfID, mockTransport, nil, nil)

	require.NotNil(t, gb)
	assert.NotNil(t, gb.config)
	assert.Equal(t, 8, gb.config.MaxPeersPerExchange)
	assert.Equal(t, 64, gb.config.MaxCachedPeers)
	assert.False(t, gb.IsRunning())
}

func TestGossipBootstrap_CustomConfig(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(2)

	config := &GossipConfig{
		MaxPeersPerExchange: 16,
		ExchangeInterval:    time.Minute,
		MaxCachedPeers:      128,
		PeerTTL:             20 * time.Minute,
	}

	gb := NewGossipBootstrap(selfID, mockTransport, nil, config)

	require.NotNil(t, gb)
	assert.Equal(t, 16, gb.config.MaxPeersPerExchange)
	assert.Equal(t, 128, gb.config.MaxCachedPeers)
	assert.Equal(t, 20*time.Minute, gb.config.PeerTTL)
}

func TestGossipBootstrap_AddPeer(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(3)
	gb := NewGossipBootstrap(selfID, mockTransport, nil, nil)

	// Add a peer
	var publicKey [32]byte
	copy(publicKey[:], "test-peer-public-key-123456789a")
	addr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 33445}
	sourceAddr := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 33445}

	gb.AddPeer(publicKey, addr, sourceAddr)

	assert.Equal(t, 1, gb.GetPeerCount())

	// Adding same peer should update, not add duplicate
	gb.AddPeer(publicKey, addr, sourceAddr)
	assert.Equal(t, 1, gb.GetPeerCount())

	// Add different peer
	var publicKey2 [32]byte
	copy(publicKey2[:], "test-peer-public-key-223456789b")
	addr2 := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 2), Port: 33445}

	gb.AddPeer(publicKey2, addr2, sourceAddr)
	assert.Equal(t, 2, gb.GetPeerCount())
}

func TestGossipBootstrap_PeerEviction(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(4)

	config := &GossipConfig{
		MaxPeersPerExchange: 8,
		ExchangeInterval:    30 * time.Second,
		MaxCachedPeers:      3, // Small limit for testing
		PeerTTL:             10 * time.Minute,
	}

	gb := NewGossipBootstrap(selfID, mockTransport, nil, config)

	// Add peers up to and beyond limit
	for i := 0; i < 5; i++ {
		var pk [32]byte
		pk[0] = byte(i)
		addr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, byte(i)), Port: 33445}
		gb.AddPeer(pk, addr, nil)
	}

	// Should have evicted oldest peers
	assert.Equal(t, 3, gb.GetPeerCount())
}

func TestGossipBootstrap_PeerExpiration(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(5)
	tp := newMockTimeProviderForGossip()

	config := &GossipConfig{
		MaxPeersPerExchange: 8,
		ExchangeInterval:    30 * time.Second,
		MaxCachedPeers:      64,
		PeerTTL:             5 * time.Minute,
	}

	gb := NewGossipBootstrap(selfID, mockTransport, nil, config)
	gb.SetTimeProvider(tp)

	// Add a peer
	var publicKey [32]byte
	publicKey[0] = 1
	addr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 33445}
	gb.AddPeer(publicKey, addr, nil)

	assert.Equal(t, 1, gb.GetPeerCount())

	// Advance time past TTL
	tp.Advance(6 * time.Minute)

	// Trigger pruning
	gb.pruneExpiredPeers()

	assert.Equal(t, 0, gb.GetPeerCount())
}

func TestGossipBootstrap_StartStop(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(6)
	gb := NewGossipBootstrap(selfID, mockTransport, nil, nil)

	assert.False(t, gb.IsRunning())

	err := gb.Start()
	require.NoError(t, err)
	assert.True(t, gb.IsRunning())

	// Starting again should be no-op
	err = gb.Start()
	require.NoError(t, err)

	gb.Stop()
	// Give goroutine time to exit
	time.Sleep(10 * time.Millisecond)
	assert.False(t, gb.IsRunning())
}

func TestGossipBootstrap_RequestPeerExchange(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(7)
	gb := NewGossipBootstrap(selfID, mockTransport, nil, nil)

	addr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 33445}
	err := gb.RequestPeerExchange(addr)

	require.NoError(t, err)

	packets := mockTransport.GetPackets()
	require.Len(t, packets, 1)
	assert.Equal(t, transport.PacketGetNodes, packets[0].PacketType)
}

func TestGossipBootstrap_HandleSendNodes(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(8)
	gb := NewGossipBootstrap(selfID, mockTransport, nil, nil)

	// Build a SendNodes packet with one IPv4 node
	data := make([]byte, 0, 100)
	data = append(data, 1)                // 1 node
	data = append(data, 2)                // IPv4 UDP type
	data = append(data, 192, 168, 1, 100) // IP
	data = append(data, 0x82, 0xB5)       // Port 33461 big-endian

	var pk [32]byte
	for i := range pk {
		pk[i] = byte(i)
	}
	data = append(data, pk[:]...)

	packet := &transport.Packet{
		PacketType: transport.PacketSendNodes,
		Data:       data,
	}
	senderAddr := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 33445}

	err := gb.handleSendNodes(packet, senderAddr)
	require.NoError(t, err)

	assert.Equal(t, 1, gb.GetPeerCount())
}

func TestGossipBootstrap_BootstrapFromGossipNoPeers(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(9)
	gb := NewGossipBootstrap(selfID, mockTransport, nil, nil)

	ctx := context.Background()
	err := gb.BootstrapFromGossip(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no gossip peers")
}

func TestGossipBootstrap_BootstrapFromGossipWithPeers(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(10)
	gb := NewGossipBootstrap(selfID, mockTransport, nil, nil)

	// Add some peers
	for i := 0; i < 3; i++ {
		var pk [32]byte
		pk[0] = byte(i)
		addr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, byte(i+1)), Port: 33445}
		gb.AddPeer(pk, addr, nil)
	}

	ctx := context.Background()
	err := gb.BootstrapFromGossip(ctx)

	require.NoError(t, err)

	// Should have sent GetNodes to each peer
	packets := mockTransport.GetPackets()
	assert.Len(t, packets, 3)
}

func TestGossipBootstrap_SeedFromRoutingTable(t *testing.T) {
	mockTransport := newMockTransportForGossip()
	selfID := createTestToxID(11)

	// Create routing table with some nodes
	routingTable := NewRoutingTable(selfID, 8)

	// Add some good nodes
	for i := 0; i < 5; i++ {
		var nospam [4]byte
		var pk [32]byte
		pk[0] = byte(i + 10)
		nodeID := crypto.NewToxID(pk, nospam)
		addr := &net.UDPAddr{IP: net.IPv4(10, 0, 0, byte(i+1)), Port: 33445}
		node := NewNode(*nodeID, addr)
		node.Status = StatusGood
		node.PublicKey = pk
		routingTable.AddNode(node)
	}

	gb := NewGossipBootstrap(selfID, mockTransport, routingTable, nil)

	count := gb.SeedFromRoutingTable()

	assert.Equal(t, 5, count)
	assert.Equal(t, 5, gb.GetPeerCount())
}

func TestBootstrapManager_GossipIntegration(t *testing.T) {
	mockTransport := newMockTransportForGossip()

	var pk [32]byte
	var nospam [4]byte
	selfID := crypto.NewToxID(pk, nospam)
	routingTable := NewRoutingTable(*selfID, 8)

	bm := NewBootstrapManager(*selfID, mockTransport, routingTable)

	require.NotNil(t, bm.gossipBootstrap)
	assert.True(t, bm.IsGossipEnabled())

	bm.SetGossipEnabled(false)
	assert.False(t, bm.IsGossipEnabled())

	bm.SetGossipEnabled(true)
	assert.True(t, bm.IsGossipEnabled())
}
