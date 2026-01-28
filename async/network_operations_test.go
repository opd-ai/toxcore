package async

import (
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestNetworkOperations tests the production-ready network operations
func TestNetworkOperations(t *testing.T) {
	// Setup
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Test storing an obfuscated message
	obfMsg := &ObfuscatedAsyncMessage{
		RecipientPseudonym: [32]byte{1, 2, 3},
		SenderPseudonym:    [32]byte{4, 5, 6},
		Epoch:              12345,
		EncryptedPayload:   []byte("test encrypted payload"),
		Timestamp:          time.Now(),
	}

	nodeAddr := &MockAddr{network: "tcp", address: "127.0.0.1:9000"}
	err = client.storeObfuscatedMessageOnNode(nodeAddr, obfMsg)
	if err != nil {
		t.Errorf("storeObfuscatedMessageOnNode failed: %v", err)
	}

	// Verify the packet was sent
	packets := mockTransport.GetPackets()
	if len(packets) != 1 {
		t.Errorf("Expected 1 packet to be sent, got %d", len(packets))
	} else {
		packet := packets[0]
		if packet.packet.PacketType != transport.PacketAsyncStore {
			t.Errorf("Expected PacketAsyncStore, got %v", packet.packet.PacketType)
		}
		if packet.addr != nodeAddr {
			t.Errorf("Expected packet to be sent to %v, got %v", nodeAddr, packet.addr)
		}
	}
}

// TestRetrieveRequest tests the async retrieve request functionality
func TestRetrieveRequest(t *testing.T) {
	// Setup
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Configure mock transport to simulate a response
	nodeAddr := &MockAddr{network: "tcp", address: "127.0.0.1:9001"}
	
	// Create an empty response (no messages stored)
	var emptyMessages []*ObfuscatedAsyncMessage
	responseData, err := client.serializeRetrieveResponse(emptyMessages)
	if err != nil {
		t.Fatalf("Failed to serialize response: %v", err)
	}
	
	// Set up mock to auto-respond when retrieve packet is sent
	mockTransport.SetSendFunc(func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncRetrieve {
			// Simulate async response from storage node
			go func() {
				time.Sleep(10 * time.Millisecond) // Small delay to simulate network
				responsePacket := &transport.Packet{
					PacketType: transport.PacketAsyncRetrieveResponse,
					Data:       responseData,
				}
				_ = client.handleRetrieveResponse(responsePacket, addr)
			}()
		}
		return nil
	})

	// Test retrieving messages
	recipientPseudonym := [32]byte{7, 8, 9}
	epochs := []uint64{100, 101, 102}

	messages, err := client.retrieveObfuscatedMessagesFromNode(nodeAddr, recipientPseudonym, epochs)
	if err != nil {
		t.Errorf("retrieveObfuscatedMessagesFromNode failed: %v", err)
	}

	// Should return empty slice (simulated empty storage node response)
	if messages == nil || len(messages) != 0 {
		t.Errorf("Expected empty message slice, got %v", messages)
	}

	// Verify the retrieve packet was sent
	packets := mockTransport.GetPackets()
	if len(packets) != 1 {
		t.Errorf("Expected 1 packet to be sent, got %d", len(packets))
	} else {
		packet := packets[0]
		if packet.packet.PacketType != transport.PacketAsyncRetrieve {
			t.Errorf("Expected PacketAsyncRetrieve, got %v", packet.packet.PacketType)
		}
		if packet.addr != nodeAddr {
			t.Errorf("Expected packet to be sent to %v, got %v", nodeAddr, packet.addr)
		}
	}
}

// TestObfuscatedMessageSerialization tests the obfuscated message serialization
func TestObfuscatedMessageSerialization(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Create test obfuscated message
	obfMsg := &ObfuscatedAsyncMessage{
		RecipientPseudonym: [32]byte{1, 2, 3, 4, 5},
		SenderPseudonym:    [32]byte{6, 7, 8, 9, 10},
		Epoch:              54321,
		EncryptedPayload:   []byte("test obfuscated payload data"),
		Timestamp:          time.Unix(1234567890, 0),
	}

	// Test serialization
	data, err := client.serializeObfuscatedMessage(obfMsg)
	if err != nil {
		t.Errorf("serializeObfuscatedMessage failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Serialized data should not be empty")
	}

	// Test deserialization
	deserializedMsg, err := client.deserializeObfuscatedMessage(data)
	if err != nil {
		t.Errorf("deserializeObfuscatedMessage failed: %v", err)
	}

	// Verify round-trip accuracy
	if deserializedMsg.RecipientPseudonym != obfMsg.RecipientPseudonym {
		t.Error("RecipientPseudonym mismatch after round-trip")
	}
	if deserializedMsg.SenderPseudonym != obfMsg.SenderPseudonym {
		t.Error("SenderPseudonym mismatch after round-trip")
	}
	if deserializedMsg.Epoch != obfMsg.Epoch {
		t.Error("Epoch mismatch after round-trip")
	}
	if string(deserializedMsg.EncryptedPayload) != string(obfMsg.EncryptedPayload) {
		t.Error("EncryptedPayload mismatch after round-trip")
	}
	if !deserializedMsg.Timestamp.Equal(obfMsg.Timestamp) {
		t.Error("Timestamp mismatch after round-trip")
	}
}

// TestRetrieveRequestSerialization tests the retrieve request serialization
func TestRetrieveRequestSerialization(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Create test retrieve request
	req := &AsyncRetrieveRequest{
		RecipientPseudonym: [32]byte{11, 12, 13, 14, 15},
		Epochs:             []uint64{200, 201, 202, 203},
	}

	// Test serialization
	data, err := client.serializeRetrieveRequest(req)
	if err != nil {
		t.Errorf("serializeRetrieveRequest failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Serialized request data should not be empty")
	}

	t.Logf("Retrieve request serialized to %d bytes", len(data))
}

// TestNetworkOperationsErrorHandling tests error cases for network operations
func TestNetworkOperationsErrorHandling(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	nodeAddr := &MockAddr{network: "tcp", address: "127.0.0.1:9000"}

	// Test storing nil message
	err = client.storeObfuscatedMessageOnNode(nodeAddr, nil)
	if err == nil {
		t.Error("Expected error when storing nil obfuscated message")
	}

	// Test serializing nil obfuscated message
	_, err = client.serializeObfuscatedMessage(nil)
	if err == nil {
		t.Error("Expected error when serializing nil obfuscated message")
	}

	// Test deserializing empty data
	_, err = client.deserializeObfuscatedMessage(nil)
	if err == nil {
		t.Error("Expected error when deserializing nil data")
	}

	_, err = client.deserializeObfuscatedMessage([]byte{})
	if err == nil {
		t.Error("Expected error when deserializing empty data")
	}

	// Test serializing nil retrieve request
	_, err = client.serializeRetrieveRequest(nil)
	if err == nil {
		t.Error("Expected error when serializing nil retrieve request")
	}
}
