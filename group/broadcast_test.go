package group

import (
"errors"
"net"
"sync"
"testing"

"github.com/opd-ai/toxcore/crypto"
"github.com/opd-ai/toxcore/dht"
"github.com/opd-ai/toxcore/transport"
)

// mockTransport is a test transport that tracks send calls
type mockTransport struct {
mu            sync.Mutex
sendCalls     []sendCall
shouldFail    bool
failOnAddress net.Addr
}

type sendCall struct {
packet *transport.Packet
addr   net.Addr
}

func (m *mockTransport) Send(packet *transport.Packet, addr net.Addr) error {
m.mu.Lock()
defer m.mu.Unlock()

m.sendCalls = append(m.sendCalls, sendCall{packet: packet, addr: addr})

if m.shouldFail {
return errors.New("mock transport error")
}

if m.failOnAddress != nil && addr.String() == m.failOnAddress.String() {
return errors.New("address-specific failure")
}

return nil
}

func (m *mockTransport) Close() error {
return nil
}

func (m *mockTransport) LocalAddr() net.Addr {
return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}
}

func (m *mockTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}

func (m *mockTransport) getSendCalls() []sendCall {
m.mu.Lock()
defer m.mu.Unlock()
return append([]sendCall{}, m.sendCalls...)
}

// createTestRoutingTable creates a routing table with predefined test nodes
func createTestRoutingTable(nodes []*dht.Node) *dht.RoutingTable {
selfID := crypto.ToxID{PublicKey: [32]byte{0xff}}
rt := dht.NewRoutingTable(selfID, 8)

for _, node := range nodes {
rt.AddNode(node)
}

return rt
}

// TestBroadcastPeerUpdateWithDirectAddress tests direct peer communication
func TestBroadcastPeerUpdateWithDirectAddress(t *testing.T) {
mockTrans := &mockTransport{}
testDHT := createTestRoutingTable([]*dht.Node{})

chat := &Chat{
ID:        1,
Peers:     make(map[uint32]*Peer),
transport: mockTrans,
dht:       testDHT,
}

peerID := uint32(100)
directAddr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 33445}

chat.Peers[peerID] = &Peer{
ID:         peerID,
Name:       "TestPeer",
Connection: 2, // UDP
PublicKey:  [32]byte{1, 2, 3},
Address:    directAddr,
}

packet := &transport.Packet{
PacketType: transport.PacketGroupBroadcast,
Data:       []byte("test message"),
}

err := chat.broadcastPeerUpdate(peerID, packet)
if err != nil {
t.Fatalf("broadcastPeerUpdate failed: %v", err)
}

calls := mockTrans.getSendCalls()
if len(calls) != 1 {
t.Fatalf("Expected 1 send call, got %d", len(calls))
}

if calls[0].addr.String() != directAddr.String() {
t.Errorf("Expected send to %s, got %s", directAddr, calls[0].addr)
}
}

// TestBroadcastPeerUpdateFallbackToDHT tests DHT fallback when direct send fails
func TestBroadcastPeerUpdateFallbackToDHT(t *testing.T) {
directAddr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 33445}
dhtAddr := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 33445}

mockTrans := &mockTransport{
failOnAddress: directAddr,
}

testNodes := []*dht.Node{
{
ID:        crypto.ToxID{PublicKey: [32]byte{1, 2, 3}},
Address:   dhtAddr,
PublicKey: [32]byte{1, 2, 3},
},
}
testDHT := createTestRoutingTable(testNodes)

chat := &Chat{
ID:        1,
Peers:     make(map[uint32]*Peer),
transport: mockTrans,
dht:       testDHT,
}

peerID := uint32(100)
chat.Peers[peerID] = &Peer{
ID:         peerID,
Name:       "TestPeer",
Connection: 2,
PublicKey:  [32]byte{1, 2, 3},
Address:    directAddr,
}

packet := &transport.Packet{
PacketType: transport.PacketGroupBroadcast,
Data:       []byte("test message"),
}

err := chat.broadcastPeerUpdate(peerID, packet)
if err != nil {
t.Fatalf("broadcastPeerUpdate failed: %v", err)
}

calls := mockTrans.getSendCalls()
if len(calls) != 2 {
t.Fatalf("Expected 2 send calls (direct + DHT), got %d", len(calls))
}

// First call should be to direct address
if calls[0].addr.String() != directAddr.String() {
t.Errorf("First call expected to %s, got %s", directAddr, calls[0].addr)
}

// Second call should be to DHT address
if calls[1].addr.String() != dhtAddr.String() {
t.Errorf("Second call expected to %s, got %s", dhtAddr, calls[1].addr)
}
}

// TestBroadcastPeerUpdateNoDHTNodes tests error when no DHT nodes available
func TestBroadcastPeerUpdateNoDHTNodes(t *testing.T) {
mockTrans := &mockTransport{}
testDHT := createTestRoutingTable([]*dht.Node{})

chat := &Chat{
ID:        1,
Peers:     make(map[uint32]*Peer),
transport: mockTrans,
dht:       testDHT,
}

peerID := uint32(100)
chat.Peers[peerID] = &Peer{
ID:         peerID,
Name:       "TestPeer",
Connection: 2,
PublicKey:  [32]byte{1, 2, 3},
Address:    nil, // No direct address
}

packet := &transport.Packet{
PacketType: transport.PacketGroupBroadcast,
Data:       []byte("test message"),
}

err := chat.broadcastPeerUpdate(peerID, packet)
if err == nil {
t.Fatal("Expected error when no DHT nodes available and no direct address")
}

if err.Error() != "no reachable address found for peer 100" {
t.Errorf("Unexpected error message: %v", err)
}
}

// TestBroadcastPeerUpdateOfflinePeer tests error for offline peers
func TestBroadcastPeerUpdateOfflinePeer(t *testing.T) {
mockTrans := &mockTransport{}
testDHT := createTestRoutingTable([]*dht.Node{})

chat := &Chat{
ID:        1,
Peers:     make(map[uint32]*Peer),
transport: mockTrans,
dht:       testDHT,
}

peerID := uint32(100)
chat.Peers[peerID] = &Peer{
ID:         peerID,
Name:       "TestPeer",
Connection: 0, // Offline
PublicKey:  [32]byte{1, 2, 3},
}

packet := &transport.Packet{
PacketType: transport.PacketGroupBroadcast,
Data:       []byte("test message"),
}

err := chat.broadcastPeerUpdate(peerID, packet)
if err == nil {
t.Fatal("Expected error when broadcasting to offline peer")
}

if err.Error() != "peer 100 is offline" {
t.Errorf("Expected 'peer is offline' error, got: %v", err)
}
}

// TestBroadcastPeerUpdateNonExistentPeer tests error for non-existent peers
func TestBroadcastPeerUpdateNonExistentPeer(t *testing.T) {
mockTrans := &mockTransport{}
testDHT := createTestRoutingTable([]*dht.Node{})

chat := &Chat{
ID:        1,
Peers:     make(map[uint32]*Peer),
transport: mockTrans,
dht:       testDHT,
}

packet := &transport.Packet{
PacketType: transport.PacketGroupBroadcast,
Data:       []byte("test message"),
}

err := chat.broadcastPeerUpdate(999, packet)
if err == nil {
t.Fatal("Expected error when broadcasting to non-existent peer")
}

if err.Error() != "peer 999 not found" {
t.Errorf("Expected 'peer not found' error, got: %v", err)
}
}

// TestBroadcastPeerUpdateDHTAddressReachability tests DHT with multiple nodes
func TestBroadcastPeerUpdateDHTAddressReachability(t *testing.T) {
addr1 := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 33445}
addr2 := &net.UDPAddr{IP: net.ParseIP("10.0.0.2"), Port: 33445}
addr3 := &net.UDPAddr{IP: net.ParseIP("10.0.0.3"), Port: 33445}

mockTrans := &mockTransport{
failOnAddress: addr1,
}

testNodes := []*dht.Node{
{ID: crypto.ToxID{PublicKey: [32]byte{1}}, Address: addr1, PublicKey: [32]byte{1}},
{ID: crypto.ToxID{PublicKey: [32]byte{2}}, Address: addr2, PublicKey: [32]byte{2}},
{ID: crypto.ToxID{PublicKey: [32]byte{3}}, Address: addr3, PublicKey: [32]byte{3}},
}
testDHT := createTestRoutingTable(testNodes)

chat := &Chat{
ID:        1,
Peers:     make(map[uint32]*Peer),
transport: mockTrans,
dht:       testDHT,
}

peerID := uint32(100)
chat.Peers[peerID] = &Peer{
ID:         peerID,
Name:       "TestPeer",
Connection: 2,
PublicKey:  [32]byte{1, 2, 3},
Address:    nil,
}

packet := &transport.Packet{
PacketType: transport.PacketGroupBroadcast,
Data:       []byte("test message"),
}

err := chat.broadcastPeerUpdate(peerID, packet)
if err != nil {
t.Fatalf("broadcastPeerUpdate failed: %v", err)
}


calls := mockTrans.getSendCalls()
// Should make multiple attempts until one succeeds
if len(calls) < 2 {
t.Fatalf("Expected at least 2 send calls, got %d", len(calls))
}

// Should eventually succeed (no error returned from broadcastPeerUpdate)
// The exact order of DHT node attempts depends on FindClosestNodes implementation
}
