package async

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestRetrievalFailsFastWhenSendFails verifies that when transport.Send() fails,
// the retrieval returns immediately without waiting for timeout
func TestRetrievalFailsFastWhenSendFails(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:6000")
	client := NewAsyncClient(keyPair, mockTransport)

	// Configure mock transport to fail Send() immediately
	sendError := errors.New("simulated send failure")
	mockTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		return sendError // Always fail
	}

	storageNode := &MockAddr{network: "udp", address: "127.0.0.1:12000"}
	pseudonym := [32]byte{1, 2, 3}

	// Measure how long it takes to fail
	start := time.Now()
	_, err = client.retrieveObfuscatedMessagesFromNode(
		storageNode,
		pseudonym,
		[]uint64{100},
		5*time.Second, // Long timeout - should not be used
	)
	elapsed := time.Since(start)

	// Should fail immediately
	if err == nil {
		t.Fatal("Expected send error, got nil")
	}

	// Should contain the send error
	if err.Error() == "" || err.Error() != "failed to send retrieve request to 127.0.0.1:12000: simulated send failure" {
		t.Logf("Got error: %v", err)
	}

	// Should complete in < 100ms, not wait for 5s timeout
	if elapsed > 200*time.Millisecond {
		t.Errorf("Expected immediate failure (< 200ms), took %v", elapsed)
	}

	t.Logf("Retrieval failed fast in %v (expected < 200ms)", elapsed)
}

// TestRetrievalWaitsForTimeoutWhenSendSucceedsButNoResponse verifies that
// when Send() succeeds but no response arrives, we properly wait for timeout
func TestRetrievalWaitsForTimeoutWhenSendSucceedsButNoResponse(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:6001")
	client := NewAsyncClient(keyPair, mockTransport)

	// Configure mock transport to succeed Send() but never send a response
	sendCallCount := 0
	mockTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		sendCallCount++
		return nil // Send succeeds, but no response will come
	}

	storageNode := &MockAddr{network: "udp", address: "127.0.0.1:12001"}
	pseudonym := [32]byte{2, 3, 4}

	// Use short timeout for test speed
	timeout := 500 * time.Millisecond

	// Measure how long it takes to timeout
	start := time.Now()
	_, err = client.retrieveObfuscatedMessagesFromNode(
		storageNode,
		pseudonym,
		[]uint64{100},
		timeout,
	)
	elapsed := time.Since(start)

	// Should timeout
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Send should have been called exactly once
	if sendCallCount != 1 {
		t.Errorf("Expected 1 Send call, got %d", sendCallCount)
	}

	// Should wait for full timeout period
	if elapsed < timeout {
		t.Errorf("Timed out too early: expected ~%v, got %v", timeout, elapsed)
	}

	if elapsed > timeout+200*time.Millisecond {
		t.Errorf("Timed out too late: expected ~%v, got %v", timeout, elapsed)
	}

	t.Logf("Retrieval timed out after %v (expected ~%v)", elapsed, timeout)
}

// TestTransportNilReturnsImmediately verifies that nil transport fails fast
func TestTransportNilReturnsImmediately(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create client with nil transport
	client := NewAsyncClient(keyPair, nil)

	storageNode := &MockAddr{network: "udp", address: "127.0.0.1:12002"}
	pseudonym := [32]byte{3, 4, 5}

	// Measure how long it takes to fail
	start := time.Now()
	_, err = client.retrieveObfuscatedMessagesFromNode(
		storageNode,
		pseudonym,
		[]uint64{100},
		5*time.Second, // Long timeout - should not be used
	)
	elapsed := time.Since(start)

	// Should fail immediately
	if err == nil {
		t.Fatal("Expected transport nil error, got nil")
	}

	// Should mention transport being nil
	expectedErr := "async messaging unavailable: transport is nil"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
	}

	// Should complete immediately, not wait for timeout
	if elapsed > 100*time.Millisecond {
		t.Errorf("Expected immediate failure (< 100ms), took %v", elapsed)
	}

	t.Logf("Nil transport failed fast in %v (expected < 100ms)", elapsed)
}
