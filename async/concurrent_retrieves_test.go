package async

import (
	"bytes"
	"encoding/gob"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestConcurrentRetrievesToSameNode validates L-4 fix: concurrent requests to same node
// are correctly correlated using RequestID instead of being overwritten.
// Before the fix, overlapping requests to the same node would overwrite each other's
// response channels, causing one caller to timeout or lose its response.
// After the fix, each request has a unique RequestID that is echoed in the response,
// allowing the correct response to be routed to the correct caller.
func TestConcurrentRetrievesToSameNode(t *testing.T) {
	t.Parallel()

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Node to retrieve from
	nodeAddr := &MockAddr{network: "udp", address: "127.0.0.1:9000"}

	// Track requests and responses
	requestIDs := make(map[uint64]int)
	var mu sync.Mutex

	// Set up mock to track requests and send responses
	mockTransport.SetSendFunc(func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncRetrieve {
			// Extract RequestID from request
			var request AsyncRetrieveRequest
			reqBuf := bytes.NewBuffer(packet.Data)
			decoder := gob.NewDecoder(reqBuf)
			if err := decoder.Decode(&request); err != nil {
				t.Logf("Failed to decode request: %v", err)
				return nil
			}

			mu.Lock()
			requestIDs[request.RequestID]++
			mu.Unlock()

			// Simulate response after a delay
			go func(requestID uint64) {
				time.Sleep(10 * time.Millisecond)

				// Create response with echoed RequestID (L-4 fix)
				emptyMessages := []*ObfuscatedAsyncMessage{}
				responseData, err := client.serializeRetrieveResponse(requestID, emptyMessages)
				if err != nil {
					t.Logf("Failed to serialize response: %v", err)
					return
				}

				responsePacket := &transport.Packet{
					PacketType: transport.PacketAsyncRetrieveResponse,
					Data:       responseData,
				}
				_ = client.handleRetrieveResponse(responsePacket, addr)
			}(request.RequestID)
		}
		return nil
	})

	// Launch 5 concurrent retrieve operations
	const numConcurrent = 5
	var wg sync.WaitGroup
	resultsChan := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			recipientPseudonym := [32]byte{byte(index), 0, 0}
			epochs := []uint64{100}
			_, err := client.retrieveObfuscatedMessagesFromNode(
				nodeAddr,
				recipientPseudonym,
				epochs,
				2*time.Second,
			)
			resultsChan <- err
		}(i)
	}

	wg.Wait()
	close(resultsChan)

	// Verify all concurrent requests succeeded
	for err := range resultsChan {
		if err != nil {
			t.Errorf("Concurrent retrieve failed: %v", err)
		}
	}

	// Verify all requests got unique RequestIDs
	if len(requestIDs) != numConcurrent {
		t.Errorf("Expected %d unique RequestIDs, got %d", numConcurrent, len(requestIDs))
	}

	// Verify no RequestID was used more than once
	for requestID, count := range requestIDs {
		if count != 1 {
			t.Errorf("RequestID %d was used %d times, expected 1", requestID, count)
		}
	}
}
