package group

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/transport"
)

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
