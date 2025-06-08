package toxcore

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/friend"
)

func TestFriendRequestBasicFunctionality(t *testing.T) {
	// Create key pairs for testing
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to create sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to create recipient key pair: %v", err)
	}

	testMessage := "Test friend request message"

	// Test 1: Create a new friend request
	request, err := friend.NewRequest(recipientKeyPair.Public, testMessage, senderKeyPair, nil)
	if err != nil {
		t.Fatalf("Failed to create friend request: %v", err)
	}

	if request.Message != testMessage {
		t.Errorf("Expected message '%s', got '%s'", testMessage, request.Message)
	}

	// Test 2: Encrypt the request
	encryptedPacket, err := request.Encrypt(senderKeyPair, recipientKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to encrypt friend request: %v", err)
	}

	if len(encryptedPacket) < 56 {
		t.Errorf("Encrypted packet too short: %d bytes", len(encryptedPacket))
	}

	// Test 3: Decrypt the request
	decryptedRequest, err := friend.DecryptRequest(encryptedPacket, recipientKeyPair)
	if err != nil {
		t.Fatalf("Failed to decrypt friend request: %v", err)
	}

	if decryptedRequest.Message != testMessage {
		t.Errorf("Expected decrypted message '%s', got '%s'", testMessage, decryptedRequest.Message)
	}

	// Test 4: Test RequestManager functionality
	manager := friend.NewRequestManager(nil)
	if manager == nil {
		t.Fatal("Failed to create request manager")
	}

	// Create enhanced request for testing
	enhancedRequest, err := decryptedRequest.NegotiateProtocol(manager.GetCapabilities())
	if err != nil {
		t.Fatalf("Failed to negotiate protocol: %v", err)
	}

	// Add the enhanced request
	manager.AddRequest(enhancedRequest)

	// Get pending requests
	pending := manager.GetPendingRequests()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending request, got %d", len(pending))
	}

	// Accept the request
	_, accepted := manager.AcceptRequest(decryptedRequest.SenderPublicKey)
	if !accepted {
		t.Error("Failed to accept friend request")
	}

	// Verify no more pending requests
	pending = manager.GetPendingRequests()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending requests after acceptance, got %d", len(pending))
	}

	t.Log("Friend request basic functionality test completed successfully")
}

func TestFriendRequestEdgeCases(t *testing.T) {
	// Test empty message
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()

	_, err := friend.NewRequest(recipientKeyPair.Public, "", senderKeyPair, nil)
	if err == nil {
		t.Error("Expected error for empty message, but got none")
	}

	// Test invalid packet length for decryption
	shortPacket := make([]byte, 50) // Less than 56 bytes required
	_, err = friend.DecryptRequest(shortPacket, recipientKeyPair)
	if err == nil {
		t.Error("Expected error for short packet, but got none")
	}

	t.Log("Friend request edge cases test completed successfully")
}
