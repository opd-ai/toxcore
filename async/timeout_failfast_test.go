package async

import (
	"bytes"
	"encoding/gob"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestSetRetrieveTimeout verifies timeout configuration
func TestSetRetrieveTimeout(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:5000")
	client := NewAsyncClient(keyPair, mockTransport)

	// Verify default timeout is 2 seconds
	defaultTimeout := client.GetRetrieveTimeout()
	if defaultTimeout != 2*time.Second {
		t.Errorf("Expected default timeout of 2s, got %v", defaultTimeout)
	}

	// Set custom timeout
	customTimeout := 500 * time.Millisecond
	client.SetRetrieveTimeout(customTimeout)

	// Verify timeout was updated
	newTimeout := client.GetRetrieveTimeout()
	if newTimeout != customTimeout {
		t.Errorf("Expected timeout %v, got %v", customTimeout, newTimeout)
	}
}

// TestAdaptiveTimeoutOnFailure verifies timeout reduces after first failure
func TestAdaptiveTimeoutOnFailure(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:5001")
	client := NewAsyncClient(keyPair, mockTransport)

	// Adaptive timeout is only meaningful in sequential mode
	client.SetParallelizeQueries(false)

	// Configure 3 unreachable storage nodes (no response configured)
	storageNodes := []net.Addr{
		&MockAddr{network: "udp", address: "127.0.0.1:8001"},
		&MockAddr{network: "udp", address: "127.0.0.1:8002"},
		&MockAddr{network: "udp", address: "127.0.0.1:8003"},
	}

	// Generate pseudonym for retrieval
	epochManager := NewEpochManager()
	currentEpoch := epochManager.GetCurrentEpoch()
	pseudonym, err := client.obfuscation.GenerateRecipientPseudonym(keyPair.Public, currentEpoch)
	if err != nil {
		t.Fatalf("Failed to generate pseudonym: %v", err)
	}

	// Measure time for retrieval from all unreachable nodes
	start := time.Now()
	messages := client.collectMessagesFromNodes(
		storageNodes,
		pseudonym,
		currentEpoch,
	)
	elapsed := time.Since(start)

	// Should return no messages (all nodes unreachable)
	if len(messages) != 0 {
		t.Errorf("Expected no messages, got %d", len(messages))
	}

	// With adaptive timeout:
	// - Node 1: 2 seconds (default timeout)
	// - Node 2: 1 second (50% of default after first failure)
	// - Node 3: 1 second (50% of default)
	// Total: ~4 seconds
	//
	// Without adaptive timeout, would be 3 * 2 = 6 seconds
	//
	// Allow some tolerance for timing variations
	if elapsed > 5*time.Second {
		t.Errorf("Adaptive timeout should complete in ~4s, took %v", elapsed)
	}

	if elapsed < 3500*time.Millisecond {
		t.Errorf("Expected at least 3.5s total wait time, got %v", elapsed)
	}
}

// TestEarlyExitAfterConsecutiveFailures verifies early exit after 3 consecutive failures
func TestEarlyExitAfterConsecutiveFailures(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:5002")
	client := NewAsyncClient(keyPair, mockTransport)

	// Configure 5 unreachable storage nodes
	storageNodes := []net.Addr{
		&MockAddr{network: "udp", address: "127.0.0.1:9001"},
		&MockAddr{network: "udp", address: "127.0.0.1:9002"},
		&MockAddr{network: "udp", address: "127.0.0.1:9003"},
		&MockAddr{network: "udp", address: "127.0.0.1:9004"},
		&MockAddr{network: "udp", address: "127.0.0.1:9005"},
	}

	// Generate pseudonym
	epochManager := NewEpochManager()
	currentEpoch := epochManager.GetCurrentEpoch()
	pseudonym, err := client.obfuscation.GenerateRecipientPseudonym(keyPair.Public, currentEpoch)
	if err != nil {
		t.Fatalf("Failed to generate pseudonym: %v", err)
	}

	// Track how many Send calls were made
	sendCount := 0
	mockTransport.RegisterHandler(transport.PacketAsyncRetrieve, func(packet *transport.Packet, addr net.Addr) error {
		sendCount++
		return nil // Send succeeds but no response comes
	})

	// Measure time for retrieval
	start := time.Now()
	messages := client.collectMessagesFromNodes(
		storageNodes,
		pseudonym,
		currentEpoch,
	)
	elapsed := time.Since(start)

	// Should return no messages
	if len(messages) != 0 {
		t.Errorf("Expected no messages, got %d", len(messages))
	}

	// Should only attempt first 3 nodes before early exit
	if sendCount > 3 {
		t.Errorf("Expected early exit after 3 failures, but attempted %d nodes", sendCount)
	}

	// Should complete in ~4 seconds (2s + 1s + 1s) instead of continuing
	if elapsed > 5*time.Second {
		t.Errorf("Early exit should complete in ~4s, took %v", elapsed)
	}
}

// TestMixedSuccessAndFailureResetCounter verifies failure counter resets on success
func TestMixedSuccessAndFailureResetCounter(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:5003")
	client := NewAsyncClient(keyPair, mockTransport)

	// Use sequential mode for this test to verify counter reset behavior
	client.SetParallelizeQueries(false)
	// Extend collection timeout to allow all 5 nodes to be attempted sequentially
	// (2s + 1s + ~0.1s + 2s + 1s = ~6.1s total, well within 15s)
	client.SetCollectionTimeout(15 * time.Second)

	// Configure 5 nodes: fail, fail, success, fail, fail
	storageNodes := []net.Addr{
		&MockAddr{network: "udp", address: "127.0.0.1:10001"}, // fail
		&MockAddr{network: "udp", address: "127.0.0.1:10002"}, // fail
		&MockAddr{network: "udp", address: "127.0.0.1:10003"}, // success
		&MockAddr{network: "udp", address: "127.0.0.1:10004"}, // fail
		&MockAddr{network: "udp", address: "127.0.0.1:10005"}, // fail
	}

	// Generate pseudonym
	epochManager := NewEpochManager()
	currentEpoch := epochManager.GetCurrentEpoch()
	pseudonym, err := client.obfuscation.GenerateRecipientPseudonym(keyPair.Public, currentEpoch)
	if err != nil {
		t.Fatalf("Failed to generate pseudonym: %v", err)
	}

	// Configure node 3 to succeed and track attempts using SetSendFunc.
	// Direct sendFunc assignment with a method value (originalSend := mockTransport.Send)
	// causes a deadlock because Send holds m.mu while calling sendFunc.
	successAddr := storageNodes[2]

	// Serialize an empty but valid retrieve response (non-empty gob-encoded slice).
	var emptyMessages []*ObfuscatedAsyncMessage
	var responseBuf bytes.Buffer
	if err := gob.NewEncoder(&responseBuf).Encode(emptyMessages); err != nil {
		t.Fatalf("Failed to encode empty response: %v", err)
	}
	validResponseData := responseBuf.Bytes()

	attemptCount := 0
	mockTransport.SetSendFunc(func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncRetrieve {
			attemptCount++
			if addr.String() == successAddr.String() {
				responsePacket := &transport.Packet{
					PacketType: transport.PacketAsyncRetrieveResponse,
					Data:       validResponseData,
				}
				go func() {
					time.Sleep(100 * time.Millisecond)
					_ = client.handleRetrieveResponse(responsePacket, successAddr)
				}()
			}
		}
		return nil
	})

	// Retrieve messages
	_ = client.collectMessagesFromNodes(
		storageNodes,
		pseudonym,
		currentEpoch,
	)

	// Should successfully retrieve from node 3
	// Note: messages will be empty because we sent empty response data,
	// but the success should reset the failure counter

	// Should attempt all 5 nodes because success on node 3 resets counter
	if attemptCount != 5 {
		t.Errorf("Expected 5 node attempts (counter resets on success), got %d", attemptCount)
	}
}

// TestCustomShortTimeout verifies very short timeouts work correctly
func TestCustomShortTimeout(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:5004")
	client := NewAsyncClient(keyPair, mockTransport)

	// Set very short timeout
	client.SetRetrieveTimeout(300 * time.Millisecond)

	storageNode := &MockAddr{network: "udp", address: "127.0.0.1:11000"}
	pseudonym := [32]byte{1, 2, 3}

	// No response configured - should timeout quickly
	start := time.Now()
	_, err = client.retrieveObfuscatedMessagesFromNode(
		storageNode,
		pseudonym,
		[]uint64{100},
		300*time.Millisecond,
	)
	elapsed := time.Since(start)

	// Should timeout
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	// Should complete in ~300ms
	if elapsed > 500*time.Millisecond {
		t.Errorf("Expected ~300ms timeout, took %v", elapsed)
	}

	if elapsed < 250*time.Millisecond {
		t.Errorf("Timeout too fast: %v", elapsed)
	}
}
