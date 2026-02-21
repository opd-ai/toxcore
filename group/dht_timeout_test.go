package group

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/transport"
)

// mockSlowTransport simulates a transport that never responds (unresponsive network).
type mockSlowTransport struct {
	mu        sync.Mutex
	sendCount int
}

func (m *mockSlowTransport) Send(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	m.sendCount++
	m.mu.Unlock()
	// Simulate packet being sent but never receiving a response
	return nil
}

func (m *mockSlowTransport) Close() error {
	return nil
}

func (m *mockSlowTransport) LocalAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:33445"}
}

func (m *mockSlowTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}

func (m *mockSlowTransport) IsConnectionOriented() bool {
	return false
}

func (m *mockSlowTransport) getSendCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sendCount
}

// TestDHTQueryTimeout verifies that DHT queries timeout correctly when the network is unresponsive.
func TestDHTQueryTimeout(t *testing.T) {
	// Create DHT components
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	toxID := crypto.ToxID{PublicKey: keyPair.Public}
	routingTable := dht.NewRoutingTable(toxID, 8)

	// Add some DHT nodes for the query to target
	for i := 0; i < 3; i++ {
		nodeKeyPair, _ := crypto.GenerateKeyPair()
		nodeID := crypto.ToxID{PublicKey: nodeKeyPair.Public}
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33446")
		node := dht.NewNode(nodeID, addr)
		node.Update(dht.StatusGood)
		routingTable.AddNode(node)
	}

	// Use a slow transport that never responds
	slowTransport := &mockSlowTransport{}

	// Query for a non-existent group with a short timeout
	groupID := uint32(99999)
	shortTimeout := 100 * time.Millisecond

	startTime := time.Now()
	_, err = queryDHTNetwork(groupID, routingTable, slowTransport, shortTimeout)
	elapsed := time.Since(startTime)

	// Verify we got a timeout error
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Verify the error message indicates a timeout
	if err.Error() != "DHT query timeout for group 99999" {
		t.Errorf("Expected timeout error message, got: %v", err)
	}

	// Verify the timeout was respected (elapsed time should be close to shortTimeout)
	if elapsed < shortTimeout {
		t.Errorf("Query returned too quickly: %v < %v", elapsed, shortTimeout)
	}
	if elapsed > shortTimeout+50*time.Millisecond {
		t.Errorf("Query took too long: %v > %v", elapsed, shortTimeout+50*time.Millisecond)
	}

	t.Logf("DHT query timed out correctly after %v", elapsed)
}

// TestDHTQueryDefaultTimeout verifies that the default 2 second timeout is applied.
func TestDHTQueryDefaultTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running timeout test in short mode")
	}

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	toxID := crypto.ToxID{PublicKey: keyPair.Public}
	routingTable := dht.NewRoutingTable(toxID, 8)

	// Add DHT nodes
	for i := 0; i < 2; i++ {
		nodeKeyPair, _ := crypto.GenerateKeyPair()
		nodeID := crypto.ToxID{PublicKey: nodeKeyPair.Public}
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33446")
		node := dht.NewNode(nodeID, addr)
		node.Update(dht.StatusGood)
		routingTable.AddNode(node)
	}

	slowTransport := &mockSlowTransport{}
	groupID := uint32(88888)

	// Use 0 timeout to trigger the default 2 second timeout
	startTime := time.Now()
	_, err = queryDHTNetwork(groupID, routingTable, slowTransport, 0)
	elapsed := time.Since(startTime)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Default timeout is 2 seconds
	defaultTimeout := 2 * time.Second
	if elapsed < defaultTimeout-100*time.Millisecond {
		t.Errorf("Query returned too quickly for default timeout: %v", elapsed)
	}
	if elapsed > defaultTimeout+200*time.Millisecond {
		t.Errorf("Query took too long for default timeout: %v", elapsed)
	}

	t.Logf("DHT query used default timeout correctly, elapsed: %v", elapsed)
}

// TestDHTQueryConcurrentTimeout verifies multiple concurrent queries timeout independently.
func TestDHTQueryConcurrentTimeout(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	toxID := crypto.ToxID{PublicKey: keyPair.Public}
	routingTable := dht.NewRoutingTable(toxID, 8)

	// Add DHT nodes
	for i := 0; i < 2; i++ {
		nodeKeyPair, _ := crypto.GenerateKeyPair()
		nodeID := crypto.ToxID{PublicKey: nodeKeyPair.Public}
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33446")
		node := dht.NewNode(nodeID, addr)
		node.Update(dht.StatusGood)
		routingTable.AddNode(node)
	}

	slowTransport := &mockSlowTransport{}

	// Launch multiple concurrent queries with different timeouts
	var wg sync.WaitGroup
	results := make(chan struct {
		groupID uint32
		elapsed time.Duration
		err     error
	}, 3)

	timeouts := []struct {
		groupID uint32
		timeout time.Duration
	}{
		{10001, 50 * time.Millisecond},
		{10002, 100 * time.Millisecond},
		{10003, 150 * time.Millisecond},
	}

	for _, tc := range timeouts {
		wg.Add(1)
		go func(gid uint32, timeout time.Duration) {
			defer wg.Done()
			start := time.Now()
			_, err := queryDHTNetwork(gid, routingTable, slowTransport, timeout)
			results <- struct {
				groupID uint32
				elapsed time.Duration
				err     error
			}{gid, time.Since(start), err}
		}(tc.groupID, tc.timeout)
	}

	wg.Wait()
	close(results)

	// Verify each query timed out at approximately the right time
	for r := range results {
		if r.err == nil {
			t.Errorf("Query for group %d: expected timeout error, got nil", r.groupID)
			continue
		}

		expectedTimeout := time.Duration(0)
		for _, tc := range timeouts {
			if tc.groupID == r.groupID {
				expectedTimeout = tc.timeout
				break
			}
		}

		if r.elapsed < expectedTimeout-20*time.Millisecond {
			t.Errorf("Query %d returned too quickly: %v < %v", r.groupID, r.elapsed, expectedTimeout)
		}
		if r.elapsed > expectedTimeout+50*time.Millisecond {
			t.Errorf("Query %d took too long: %v > %v", r.groupID, r.elapsed, expectedTimeout+50*time.Millisecond)
		}

		t.Logf("Query %d timed out correctly after %v (expected ~%v)", r.groupID, r.elapsed, expectedTimeout)
	}
}

// TestDHTQueryHandlerCleanup verifies response handlers are cleaned up after timeout.
func TestDHTQueryHandlerCleanup(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	toxID := crypto.ToxID{PublicKey: keyPair.Public}
	routingTable := dht.NewRoutingTable(toxID, 8)

	// Add a DHT node
	nodeKeyPair, _ := crypto.GenerateKeyPair()
	nodeID := crypto.ToxID{PublicKey: nodeKeyPair.Public}
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33446")
	node := dht.NewNode(nodeID, addr)
	node.Update(dht.StatusGood)
	routingTable.AddNode(node)

	slowTransport := &mockSlowTransport{}

	// Count handlers before query
	groupResponseHandlers.RLock()
	handlersBefore := len(groupResponseHandlers.handlers)
	groupResponseHandlers.RUnlock()

	// Run query that will timeout
	groupID := uint32(77777)
	_, _ = queryDHTNetwork(groupID, routingTable, slowTransport, 50*time.Millisecond)

	// Give a moment for cleanup
	time.Sleep(10 * time.Millisecond)

	// Verify handler was cleaned up
	groupResponseHandlers.RLock()
	handlersAfter := len(groupResponseHandlers.handlers)
	groupResponseHandlers.RUnlock()

	if handlersAfter != handlersBefore {
		t.Errorf("Handler not cleaned up after timeout: before=%d, after=%d", handlersBefore, handlersAfter)
	}

	t.Log("Response handler cleaned up correctly after timeout")
}

// TestSafeInvokeCallbackPanicRecovery verifies callback panics don't crash the caller.
func TestSafeInvokeCallbackPanicRecovery(t *testing.T) {
	recovered := make(chan bool, 1)

	// Create a callback that panics
	panicCallback := func() {
		panic("intentional test panic")
	}

	// This should not panic the test
	safeInvokeCallback(panicCallback)

	// Wait a bit for the goroutine to execute
	time.Sleep(50 * time.Millisecond)

	// If we get here without crashing, the panic was recovered
	select {
	case <-recovered:
		t.Log("Panic was recovered as expected")
	default:
		// This is the expected path - panic was silently recovered
		t.Log("safeInvokeCallback correctly recovered from panic")
	}
}

// TestSafeInvokeCallbackNormalExecution verifies normal callbacks execute correctly.
func TestSafeInvokeCallbackNormalExecution(t *testing.T) {
	executed := make(chan bool, 1)

	normalCallback := func() {
		executed <- true
	}

	safeInvokeCallback(normalCallback)

	select {
	case <-executed:
		t.Log("Normal callback executed successfully")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Callback was not executed within timeout")
	}
}
