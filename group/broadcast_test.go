package group

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

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

// ============================================================================
// Broadcast Offline/Empty Group Tests
// ============================================================================

// TestBroadcastWithAllPeersOffline verifies that broadcasting when all peers are offline
// succeeds without error, as this is a valid operational state (broadcast attempted correctly,
// but had no recipients)
func TestBroadcastWithAllPeersOffline(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Name:       "TestGroup",
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Add self as a peer (should be skipped in broadcast)
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Self",
		Connection: 1, // Online
		PublicKey:  [32]byte{1, 2, 3},
	}

	// Add multiple offline peers
	chat.Peers[2] = &Peer{
		ID:         2,
		Name:       "Peer2",
		Connection: 0, // Offline
		PublicKey:  [32]byte{2, 3, 4},
	}

	chat.Peers[3] = &Peer{
		ID:         3,
		Name:       "Peer3",
		Connection: 0, // Offline
		PublicKey:  [32]byte{3, 4, 5},
	}

	chat.Peers[4] = &Peer{
		ID:         4,
		Name:       "Peer4",
		Connection: 0, // Offline
		PublicKey:  [32]byte{4, 5, 6},
	}

	// Attempt to broadcast a message - all peers are offline except self
	err := chat.SendMessage("test message")
	// Fixed behavior: broadcasting when all peers are offline is a valid state (not an error)
	// The broadcast was attempted correctly but had no recipients - this should succeed
	if err != nil {
		t.Errorf("Expected success (nil) when all peers are offline, got error: %v", err)
	}

	// Verify no send calls were made (all peers were offline)
	sendCalls := mockTrans.getSendCalls()
	if len(sendCalls) > 0 {
		t.Errorf("Expected no send calls for offline peers, got %d calls", len(sendCalls))
	}
}

// TestBroadcastWithNoPeers verifies broadcasting to a group with only self succeeds
// (this is a valid state, common when creating a group or when all members leave)
func TestBroadcastWithNoPeers(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Name:       "EmptyGroup",
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Only self in the group
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Self",
		Connection: 1,
		PublicKey:  [32]byte{1, 2, 3},
	}

	err := chat.SendMessage("test message")
	// Fixed behavior: broadcasting to a group with only self is a valid state (not an error)
	// This is common when a user first creates a group or all other members have left
	if err != nil {
		t.Errorf("Expected success (nil) when group has no other peers, got error: %v", err)
	}
}

// TestBroadcastWithMixedOnlineOfflinePeers verifies partial success handling
func TestBroadcastWithMixedOnlineOfflinePeers(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	peerAddr := &mockAddr{address: "127.0.0.1:5000"}

	chat := &Chat{
		ID:         1,
		Name:       "MixedGroup",
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Self
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Self",
		Connection: 1,
		PublicKey:  [32]byte{1, 2, 3},
	}

	// Online peer with address
	chat.Peers[2] = &Peer{
		ID:         2,
		Name:       "OnlinePeer",
		Connection: 1, // Online
		PublicKey:  [32]byte{2, 3, 4},
		Address:    peerAddr,
	}

	// Offline peers
	chat.Peers[3] = &Peer{
		ID:         3,
		Name:       "OfflinePeer1",
		Connection: 0, // Offline
		PublicKey:  [32]byte{3, 4, 5},
	}

	chat.Peers[4] = &Peer{
		ID:         4,
		Name:       "OfflinePeer2",
		Connection: 0, // Offline
		PublicKey:  [32]byte{4, 5, 6},
	}

	err := chat.SendMessage("test message")
	// Should succeed because at least one peer is online
	if err != nil {
		t.Errorf("Expected success with at least one online peer, got error: %v", err)
	}

	// Verify send was called only once (for the online peer)
	sendCalls := mockTrans.getSendCalls()
	if len(sendCalls) != 1 {
		t.Errorf("Expected 1 send call (online peer only), got %d calls", len(sendCalls))
	}
}

// mockAddr implements net.Addr for testing
type mockAddr struct {
	address string
}

func (m *mockAddr) Network() string {
	return "udp"
}

func (m *mockAddr) String() string {
	return m.address
}

// ============================================================================
// Broadcast Performance Tests
// ============================================================================

// mockDelayTransport simulates network latency for performance testing
type mockDelayTransport struct {
	mu        sync.Mutex
	sendCount int
	delay     time.Duration
}

func (m *mockDelayTransport) Send(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	m.sendCount++
	m.mu.Unlock()

	// Simulate network latency
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *mockDelayTransport) Close() error {
	return nil
}

func (m *mockDelayTransport) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}
}

func (m *mockDelayTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}

func (m *mockDelayTransport) getSendCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sendCount
}

// TestBroadcastPerformanceWithLargeGroup tests parallel broadcast efficiency
func TestBroadcastPerformanceWithLargeGroup(t *testing.T) {
	mockTrans := &mockDelayTransport{delay: 10 * time.Millisecond}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		SelfPeerID: 1, // Set non-zero SelfPeerID
		Peers:      make(map[uint32]*Peer),
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Add self peer (required for SendMessage validation)
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Self",
		Connection: 2,
		PublicKey:  [32]byte{1},
		Address:    &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445},
	}

	// Create 50 online peers to test performance at scale
	peerCount := 50
	for i := 2; i <= peerCount+1; i++ {
		peerID := uint32(i)
		chat.Peers[peerID] = &Peer{
			ID:         peerID,
			Name:       "Peer",
			Connection: 2, // UDP connection
			PublicKey:  [32]byte{byte(i)},
			Address:    &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 33445 + i},
		}
	}

	// Measure broadcast time
	start := time.Now()
	err := chat.SendMessage("Test message to large group")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	// Verify all peers received the message
	sendCount := mockTrans.getSendCount()
	if sendCount != peerCount {
		t.Errorf("Expected %d sends, got %d", peerCount, sendCount)
	}

	// With 10ms latency per send and 50 peers:
	// Sequential would take ~500ms (50 * 10ms)
	// Parallel (10 workers) should take ~50-100ms (5 batches * 10ms + overhead)
	// We use 200ms as threshold to allow for test environment variance
	maxExpectedDuration := 200 * time.Millisecond

	if duration > maxExpectedDuration {
		t.Errorf("Broadcast took too long: %v (expected < %v)", duration, maxExpectedDuration)
	}

	t.Logf("Successfully broadcast to %d peers in %v (parallel optimization)", peerCount, duration)
}

// TestBroadcastConcurrencyCorrectness verifies parallel sends maintain correctness
func TestBroadcastConcurrencyCorrectness(t *testing.T) {
	mockTrans := &mockDelayTransport{delay: 1 * time.Millisecond}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		SelfPeerID: 1, // Set non-zero SelfPeerID
		Peers:      make(map[uint32]*Peer),
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Add self
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Self",
		Connection: 2,
		PublicKey:  [32]byte{1},
		Address:    &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445},
	}

	// Create 100 peers to test concurrency
	peerCount := 100
	for i := 2; i <= peerCount+1; i++ {
		peerID := uint32(i)
		chat.Peers[peerID] = &Peer{
			ID:         peerID,
			Name:       "Peer",
			Connection: 2,
			PublicKey:  [32]byte{byte(i)},
			Address:    &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 33445 + i},
		}
	}

	// Send multiple broadcasts concurrently to stress-test
	iterations := 10
	var wg sync.WaitGroup
	errors := make(chan error, iterations)

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			if err := chat.SendMessage("Concurrent test message"); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent broadcast error: %v", err)
	}

	// Verify expected number of sends (100 peers * 10 iterations)
	expectedSends := peerCount * iterations
	actualSends := mockTrans.getSendCount()
	if actualSends != expectedSends {
		t.Errorf("Expected %d sends, got %d", expectedSends, actualSends)
	}

	t.Logf("Successfully completed %d concurrent broadcasts to %d peers (%d total sends)",
		iterations, peerCount, actualSends)
}

// TestBroadcastWorkerPoolBehavior verifies worker pool limits concurrent operations
func TestBroadcastWorkerPoolBehavior(t *testing.T) {
	// Track concurrent send operations
	activeSends := &sync.Map{}
	maxConcurrent := 0
	var mu sync.Mutex

	mockTrans := &mockTrackedTransport{
		onSend: func() {
			// Track concurrent operations
			ptr := new(int)
			activeSends.Store(ptr, true)
			defer activeSends.Delete(ptr)

			// Count current concurrent sends
			count := 0
			activeSends.Range(func(key, value interface{}) bool {
				count++
				return true
			})

			mu.Lock()
			if count > maxConcurrent {
				maxConcurrent = count
			}
			mu.Unlock()

			// Simulate work
			time.Sleep(5 * time.Millisecond)
		},
	}

	testDHT := createTestRoutingTable([]*dht.Node{})
	chat := &Chat{
		ID:         1,
		SelfPeerID: 1, // Set non-zero SelfPeerID
		Peers:      make(map[uint32]*Peer),
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Add self
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Self",
		Connection: 2,
		PublicKey:  [32]byte{1},
		Address:    &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445},
	}

	// Create 30 peers (should use worker pool limit of 10)
	for i := 2; i <= 31; i++ {
		peerID := uint32(i)
		chat.Peers[peerID] = &Peer{
			ID:         peerID,
			Name:       "Peer",
			Connection: 2,
			PublicKey:  [32]byte{byte(i)},
			Address:    &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 33445 + i},
		}
	}

	err := chat.SendMessage("Worker pool test")
	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Worker pool should limit to 10 concurrent operations
	expectedMax := 10
	if maxConcurrent > expectedMax {
		t.Errorf("Worker pool exceeded limit: got %d concurrent sends, expected <= %d",
			maxConcurrent, expectedMax)
	}

	if maxConcurrent < 1 {
		t.Errorf("Worker pool not used: got %d concurrent sends", maxConcurrent)
	}

	t.Logf("Worker pool correctly limited concurrency to %d (max allowed: %d)",
		maxConcurrent, expectedMax)
}

// mockTrackedTransport allows tracking concurrent operations
type mockTrackedTransport struct {
	onSend func()
}

func (m *mockTrackedTransport) Send(packet *transport.Packet, addr net.Addr) error {
	if m.onSend != nil {
		m.onSend()
	}
	return nil
}

func (m *mockTrackedTransport) Close() error {
	return nil
}

func (m *mockTrackedTransport) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}
}

func (m *mockTrackedTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}
