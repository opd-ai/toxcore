package async

import (
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestAsyncMessageRetrievalIntegration tests the complete message retrieval flow
// from request to response handling with network simulation
func TestAsyncMessageRetrievalIntegration(t *testing.T) {
	// Create sender and recipient
	senderKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key: %v", err)
	}

	recipientKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key: %v", err)
	}

	// Create mock transports for sender and recipient
	senderTransport := NewMockTransport("127.0.0.1:8000")
	recipientTransport := NewMockTransport("127.0.0.1:9000")

	// Create async clients
	senderClient := NewAsyncClient(senderKey, senderTransport)
	recipientClient := NewAsyncClient(recipientKey, recipientTransport)

	// Add recipient to known senders for decryption
	recipientClient.knownSenders[senderKey.Public] = true

	// Create a test message
	testMessage := []byte("Hello, this is a test async message!")
	fsMsg := &ForwardSecureMessage{
		SenderPK:      senderKey.Public,
		RecipientPK:   recipientKey.Public,
		EncryptedData: testMessage,
		MessageID:     [32]byte{1, 2, 3, 4, 5},
		Timestamp:     time.Now(),
	}

	// Sender creates and stores obfuscated message
	obfMsg, err := senderClient.createObfuscatedMessage(recipientKey.Public, fsMsg)
	if err != nil {
		t.Fatalf("Failed to create obfuscated message: %v", err)
	}

	// Simulate storage node having the message
	storedMessages := []*ObfuscatedAsyncMessage{obfMsg}
	responseData, err := recipientClient.serializeRetrieveResponse(storedMessages)
	if err != nil {
		t.Fatalf("Failed to serialize response: %v", err)
	}

	// Configure recipient transport to simulate storage node response
	storageNodeAddr := &MockAddr{network: "udp", address: "127.0.0.1:7000"}
	recipientTransport.SetSendFunc(func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncRetrieve {
			// Simulate async response from storage node
			go func() {
				time.Sleep(10 * time.Millisecond)
				responsePacket := &transport.Packet{
					PacketType: transport.PacketAsyncRetrieveResponse,
					Data:       responseData,
				}
				_ = recipientClient.handleRetrieveResponse(responsePacket, addr)
			}()
		}
		return nil
	})

	// Recipient retrieves messages from storage node
	retrievedMessages, err := recipientClient.retrieveObfuscatedMessagesFromNode(
		storageNodeAddr,
		obfMsg.RecipientPseudonym,
		[]uint64{obfMsg.Epoch},
	)

	if err != nil {
		t.Fatalf("Failed to retrieve messages: %v", err)
	}

	// Verify we got the message back
	if len(retrievedMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(retrievedMessages))
	}

	// Verify the message content matches
	if retrievedMessages[0].MessageID != obfMsg.MessageID {
		t.Errorf("Message ID mismatch")
	}

	if retrievedMessages[0].RecipientPseudonym != obfMsg.RecipientPseudonym {
		t.Errorf("Recipient pseudonym mismatch")
	}

	if retrievedMessages[0].Epoch != obfMsg.Epoch {
		t.Errorf("Epoch mismatch")
	}

	// Verify the retrieve request packet was sent
	packets := recipientTransport.GetPackets()
	if len(packets) != 1 {
		t.Errorf("Expected 1 packet sent, got %d", len(packets))
	}

	if packets[0].packet.PacketType != transport.PacketAsyncRetrieve {
		t.Errorf("Expected PacketAsyncRetrieve, got %v", packets[0].packet.PacketType)
	}
}

// TestAsyncMessageRetrievalTimeout tests timeout handling when storage node doesn't respond
func TestAsyncMessageRetrievalTimeout(t *testing.T) {
	recipientKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:9000")
	client := NewAsyncClient(recipientKey, mockTransport)

	// Don't configure any response - should timeout
	storageNodeAddr := &MockAddr{network: "udp", address: "127.0.0.1:7000"}
	recipientPseudonym := [32]byte{1, 2, 3}

	start := time.Now()
	_, err = client.retrieveObfuscatedMessagesFromNode(
		storageNodeAddr,
		recipientPseudonym,
		[]uint64{100},
	)

	elapsed := time.Since(start)

	// Should timeout after ~5 seconds
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if elapsed < 4*time.Second || elapsed > 6*time.Second {
		t.Errorf("Expected ~5 second timeout, got %v", elapsed)
	}
}

// TestAsyncMessageRetrievalEmptyResponse tests handling of empty storage node responses
func TestAsyncMessageRetrievalEmptyResponse(t *testing.T) {
	recipientKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:9000")
	client := NewAsyncClient(recipientKey, mockTransport)

	// Configure transport to return empty message list
	emptyMessages := make([]*ObfuscatedAsyncMessage, 0)
	responseData, err := client.serializeRetrieveResponse(emptyMessages)
	if err != nil {
		t.Fatalf("Failed to serialize response: %v", err)
	}

	storageNodeAddr := &MockAddr{network: "udp", address: "127.0.0.1:7000"}
	mockTransport.SetSendFunc(func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncRetrieve {
			go func() {
				time.Sleep(10 * time.Millisecond)
				responsePacket := &transport.Packet{
					PacketType: transport.PacketAsyncRetrieveResponse,
					Data:       responseData,
				}
				_ = client.handleRetrieveResponse(responsePacket, addr)
			}()
		}
		return nil
	})

	messages, err := client.retrieveObfuscatedMessagesFromNode(
		storageNodeAddr,
		[32]byte{1, 2, 3},
		[]uint64{100},
	)

	if err != nil {
		t.Errorf("Empty response should not error: %v", err)
	}

	if messages == nil {
		t.Error("Expected non-nil empty slice, got nil")
	}

	if len(messages) != 0 {
		t.Errorf("Expected empty slice, got %d messages", len(messages))
	}
}

// createObfuscatedMessage is a helper that creates an obfuscated message for testing
func (ac *AsyncClient) createObfuscatedMessage(recipientPK [32]byte, fsMsg *ForwardSecureMessage) (*ObfuscatedAsyncMessage, error) {
	serializedMsg, err := ac.serializeForwardSecureMessage(fsMsg)
	if err != nil {
		return nil, err
	}

	sharedSecret, err := ac.deriveSharedSecret(recipientPK)
	if err != nil {
		return nil, err
	}

	return ac.obfuscation.CreateObfuscatedMessage(
		ac.keyPair.Private,
		recipientPK,
		serializedMsg,
		sharedSecret,
	)
}
