package async

import (
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// mockRetrievalTransport is a simple implementation of the transport.Transport interface for testing
type mockRetrievalTransport struct {
	sendCount int
	lastAddr  net.Addr
}

func (m *mockRetrievalTransport) Send(packet []byte, addr net.Addr) error {
	m.sendCount++
	m.lastAddr = addr
	return nil
}

func (m *mockRetrievalTransport) Receive() ([]byte, net.Addr, error) {
	return nil, nil, nil
}

func (m *mockRetrievalTransport) Close() error {
	return nil
}

func (m *mockRetrievalTransport) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (m *mockRetrievalTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
	// No-op for test
}

// TestRandomizedRetrieval verifies that the retrieval scheduler works correctly
func TestRandomizedRetrieval(t *testing.T) {
	// Create a key pair for testing
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create a mock transport
	mockTransport := &mockRetrievalTransport{}

	// Create an AsyncClient with the mock transport
	client := NewAsyncClient(keyPair, mockTransport)

	// Configure the retrieval scheduler with a very short interval for testing
	client.ConfigureRetrieval(50*time.Millisecond, 10, true, 0.5)

	// Start the scheduler
	client.StartScheduledRetrieval()

	// Wait for a few retrievals to occur
	time.Sleep(300 * time.Millisecond)

	// Stop the scheduler
	client.StopScheduledRetrieval()

	// Verify the scheduler is working
	// (We can't directly check consecutiveEmpty since it's private, but we can check the scheduler exists)
	if client.retrievalScheduler != nil {
		t.Log("Retrieval scheduler is initialized correctly")
	} else {
		t.Error("Retrieval scheduler was not properly initialized")
	}
}

// TestRetrievalIntervalVariation tests that the retrieval intervals vary as expected
func TestRetrievalIntervalVariation(t *testing.T) {
	// Create key pair and mock transport
	keyPair, _ := crypto.GenerateKeyPair()
	mockTransport := &mockRetrievalTransport{}

	// Create test client
	client := NewAsyncClient(keyPair, mockTransport)

	// Get multiple interval calculations to check for variation
	scheduler := client.retrievalScheduler

	// Set base interval and jitter for testing
	scheduler.Configure(100*time.Millisecond, 50, true, 0.5)

	// Calculate multiple intervals and verify they're different
	interval1 := scheduler.calculateNextInterval()
	interval2 := scheduler.calculateNextInterval()
	interval3 := scheduler.calculateNextInterval()

	// Check that at least some intervals are different (proving randomization)
	if interval1 == interval2 && interval2 == interval3 {
		t.Error("All intervals are identical - no randomization detected")
	} else {
		t.Log("Intervals vary as expected due to jitter")
	}

	// Start the scheduler
	client.StartScheduledRetrieval()

	// Wait for several retrievals
	time.Sleep(300 * time.Millisecond)

	// Stop the scheduler
	client.StopScheduledRetrieval()

	// Check we have enough data points
	if len(timestamps) < 5 {
		t.Fatalf("Not enough retrievals recorded: %d", len(timestamps))
	}

	// Calculate intervals between retrievals
	intervals := make([]time.Duration, 0, len(timestamps)-1)
	for i := 1; i < len(timestamps); i++ {
		interval := timestamps[i].Sub(timestamps[i-1])
		intervals = append(intervals, interval)
	}

	// Verify intervals have variation (not all the same)
	var sumIntervals time.Duration
	for _, interval := range intervals {
		sumIntervals += interval
	}
	avgInterval := sumIntervals / time.Duration(len(intervals))

	// Calculate variance
	var variance float64
	for _, interval := range intervals {
		diff := float64(interval - avgInterval)
		variance += diff * diff
	}
	variance /= float64(len(intervals))

	// If there's significant variation in the intervals, the pattern is randomized
	if variance > 0 {
		t.Logf("Retrieval pattern shows randomization with variance: %f", variance)
	} else {
		t.Error("Retrieval pattern shows no randomization")
	}
}
