package dht

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// mockIterativeTransport mocks the transport for iterative lookup tests.
type mockIterativeTransport struct {
	mu              sync.Mutex
	sentPackets     []sentPacket
	responseHandler func(addr net.Addr, data []byte) []*Node
}

type sentPacket struct {
	packet *transport.Packet
	addr   net.Addr
}

func newMockIterativeTransport() *mockIterativeTransport {
	return &mockIterativeTransport{
		sentPackets: make([]sentPacket, 0),
	}
}

func (m *mockIterativeTransport) Send(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	m.sentPackets = append(m.sentPackets, sentPacket{packet: packet, addr: addr})
	m.mu.Unlock()
	return nil
}

func (m *mockIterativeTransport) Receive() (*transport.Packet, net.Addr, error) {
	return nil, nil, nil
}

func (m *mockIterativeTransport) Close() error {
	return nil
}

func (m *mockIterativeTransport) LocalAddr() net.Addr {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
	return addr
}

func (m *mockIterativeTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}

func (m *mockIterativeTransport) IsConnectionOriented() bool {
	return false
}

func (m *mockIterativeTransport) getSentPacketCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sentPackets)
}

func TestLookupConfig_Default(t *testing.T) {
	config := DefaultLookupConfig()

	if config.Alpha != Alpha {
		t.Errorf("Expected Alpha=%d, got %d", Alpha, config.Alpha)
	}
	if config.K != DefaultLookupK {
		t.Errorf("Expected K=%d, got %d", DefaultLookupK, config.K)
	}
	if config.MaxIterations != DefaultMaxLookupIterations {
		t.Errorf("Expected MaxIterations=%d, got %d", DefaultMaxLookupIterations, config.MaxIterations)
	}
}

func TestIterativeLookup_NoNodes(t *testing.T) {
	// Create empty routing table
	selfKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(selfKey.Public, nospam)

	rt := NewRoutingTable(*selfID, 8)
	tr := newMockIterativeTransport()

	il := NewIterativeLookup(rt, tr, *selfID, nil)

	// Lookup with empty routing table
	targetKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	ctx := context.Background()
	result := il.FindNode(ctx, targetKey.Public)

	if result.Error != ErrNoNodesAvailable {
		t.Errorf("Expected ErrNoNodesAvailable, got %v", result.Error)
	}
	if result.Success {
		t.Error("Expected lookup to fail with no nodes")
	}
}

func TestIterativeLookup_WithNodes(t *testing.T) {
	// Create routing table with some nodes
	selfKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(selfKey.Public, nospam)

	rt := NewRoutingTable(*selfID, 8)
	tr := newMockIterativeTransport()

	// Add some nodes to the routing table
	for i := 0; i < 10; i++ {
		nodeKey, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		nodeID := crypto.NewToxID(nodeKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		node := NewNode(*nodeID, addr)
		rt.AddNode(node)
	}

	config := DefaultLookupConfig()
	config.ResponseTimeout = 100 * time.Millisecond // Short timeout for testing
	config.Timeout = 500 * time.Millisecond

	il := NewIterativeLookup(rt, tr, *selfID, config)

	// Lookup a target
	targetKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	ctx := context.Background()
	result := il.FindNode(ctx, targetKey.Public)

	// Should have sent some packets
	if tr.getSentPacketCount() == 0 {
		t.Error("Expected some packets to be sent")
	}

	// Should have closest nodes from initial set (even if no responses)
	if len(result.ClosestNodes) == 0 {
		t.Error("Expected some closest nodes")
	}

	// Should have recorded iterations
	if result.Iterations == 0 {
		t.Error("Expected at least one iteration")
	}
}

func TestIterativeLookup_ParallelQueries(t *testing.T) {
	selfKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(selfKey.Public, nospam)

	rt := NewRoutingTable(*selfID, 8)
	tr := newMockIterativeTransport()

	// Add more nodes than Alpha to test parallel querying
	for i := 0; i < 20; i++ {
		nodeKey, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		nodeID := crypto.NewToxID(nodeKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		node := NewNode(*nodeID, addr)
		rt.AddNode(node)
	}

	config := DefaultLookupConfig()
	config.Alpha = 3
	config.ResponseTimeout = 50 * time.Millisecond
	config.Timeout = 200 * time.Millisecond

	il := NewIterativeLookup(rt, tr, *selfID, config)

	targetKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	ctx := context.Background()
	_ = il.FindNode(ctx, targetKey.Public)

	// Should query at most Alpha nodes in first round
	sentCount := tr.getSentPacketCount()
	if sentCount == 0 {
		t.Error("Expected packets to be sent")
	}
}

func TestIterativeLookup_HandleResponse(t *testing.T) {
	selfKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(selfKey.Public, nospam)

	rt := NewRoutingTable(*selfID, 8)
	tr := newMockIterativeTransport()

	il := NewIterativeLookup(rt, tr, *selfID, nil)

	// Simulate pending response
	nodeKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	responseChan := make(chan []*Node, 1)
	il.responsesMu.Lock()
	il.pendingResponses[nodeKey.Public] = responseChan
	il.responsesMu.Unlock()

	// Create some response nodes
	responseNodes := make([]*Node, 3)
	for i := 0; i < 3; i++ {
		rKey, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		rID := crypto.NewToxID(rKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		responseNodes[i] = NewNode(*rID, addr)
	}

	// Handle the response
	il.HandleNodesResponse(nodeKey.Public, responseNodes)

	// Check that response was received
	select {
	case nodes := <-responseChan:
		if len(nodes) != 3 {
			t.Errorf("Expected 3 response nodes, got %d", len(nodes))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Response was not delivered")
	}
}

func TestNodeSet_Add(t *testing.T) {
	targetKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	targetID := crypto.NewToxID(targetKey.Public, nospam)
	targetNode := &Node{ID: *targetID}
	copy(targetNode.PublicKey[:], targetKey.Public[:])

	ns := newNodeSet(targetNode, 5)

	// Add nodes
	for i := 0; i < 10; i++ {
		nodeKey, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		nodeID := crypto.NewToxID(nodeKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		node := NewNode(*nodeID, addr)
		ns.add(node)
	}

	// Should be limited to capacity
	closest := ns.getClosest(10)
	if len(closest) != 5 {
		t.Errorf("Expected 5 nodes (capacity), got %d", len(closest))
	}
}

func TestNodeSet_NoDuplicates(t *testing.T) {
	targetKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	targetID := crypto.NewToxID(targetKey.Public, nospam)
	targetNode := &Node{ID: *targetID}
	copy(targetNode.PublicKey[:], targetKey.Public[:])

	ns := newNodeSet(targetNode, 10)

	// Add same node twice
	nodeKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	nodeID := crypto.NewToxID(nodeKey.Public, nospam)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
	node := NewNode(*nodeID, addr)

	added1 := ns.add(node)
	added2 := ns.add(node)

	if !added1 {
		t.Error("First add should succeed")
	}
	if added2 {
		t.Error("Second add (duplicate) should fail")
	}

	closest := ns.getClosest(10)
	if len(closest) != 1 {
		t.Errorf("Expected 1 node, got %d", len(closest))
	}
}

func TestNodeSet_SelectUnqueried(t *testing.T) {
	targetKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	targetID := crypto.NewToxID(targetKey.Public, nospam)
	targetNode := &Node{ID: *targetID}
	copy(targetNode.PublicKey[:], targetKey.Public[:])

	ns := newNodeSet(targetNode, 10)

	// Add nodes
	nodes := make([]*Node, 5)
	for i := 0; i < 5; i++ {
		nodeKey, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		nodeID := crypto.NewToxID(nodeKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		nodes[i] = NewNode(*nodeID, addr)
		ns.add(nodes[i])
	}

	// Mark some as queried
	queried := make(map[[32]byte]struct{})
	queried[nodes[0].PublicKey] = struct{}{}
	queried[nodes[1].PublicKey] = struct{}{}

	unqueried := ns.selectUnqueried(5, queried)
	if len(unqueried) != 3 {
		t.Errorf("Expected 3 unqueried nodes, got %d", len(unqueried))
	}

	// Verify none of the returned nodes are in the queried set
	for _, n := range unqueried {
		if _, isQueried := queried[n.PublicKey]; isQueried {
			t.Error("Returned node was already queried")
		}
	}
}

func TestIterativeLookup_ContextCancellation(t *testing.T) {
	selfKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(selfKey.Public, nospam)

	rt := NewRoutingTable(*selfID, 8)
	tr := newMockIterativeTransport()

	// Add some nodes
	for i := 0; i < 10; i++ {
		nodeKey, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		nodeID := crypto.NewToxID(nodeKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		node := NewNode(*nodeID, addr)
		rt.AddNode(node)
	}

	config := DefaultLookupConfig()
	config.ResponseTimeout = 5 * time.Second // Long timeout
	config.Timeout = 100 * time.Millisecond  // Short overall timeout

	il := NewIterativeLookup(rt, tr, *selfID, config)

	targetKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Use a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result := il.FindNode(ctx, targetKey.Public)

	// Should have terminated due to context
	if result.Error == nil && result.Duration > 200*time.Millisecond {
		t.Error("Expected lookup to be cancelled by context")
	}
}

func BenchmarkIterativeLookup(b *testing.B) {
	selfKey, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(selfKey.Public, nospam)

	rt := NewRoutingTable(*selfID, 8)
	tr := newMockIterativeTransport()

	// Add nodes
	for i := 0; i < 100; i++ {
		nodeKey, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatalf("Failed to generate keypair: %v", err)
		}
		nodeID := crypto.NewToxID(nodeKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		node := NewNode(*nodeID, addr)
		rt.AddNode(node)
	}

	config := DefaultLookupConfig()
	config.ResponseTimeout = 1 * time.Millisecond
	config.Timeout = 10 * time.Millisecond
	config.MaxIterations = 2

	il := NewIterativeLookup(rt, tr, *selfID, config)

	// Pre-generate target keys
	targets := make([][32]byte, b.N)
	for i := 0; i < b.N; i++ {
		targetKey, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatalf("Failed to generate keypair: %v", err)
		}
		targets[i] = targetKey.Public
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		il.FindNode(context.Background(), targets[i])
	}
}
