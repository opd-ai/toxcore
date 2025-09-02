package async

import (
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// Helper function to store a message with encryption
func storeTestMessage(storage *MessageStorage, recipientPK, senderPK [32]byte, senderSK [32]byte, message string, messageType MessageType) ([16]byte, error) {
	if len(message) == 0 {
		// For empty message test, pass empty encrypted data
		return storage.StoreMessage(recipientPK, senderPK, []byte{}, [24]byte{}, messageType)
	}
	
	encryptedData, nonce, err := EncryptForRecipient([]byte(message), recipientPK, senderSK)
	if err != nil {
		return [16]byte{}, err
	}
	
	return storage.StoreMessage(recipientPK, senderPK, encryptedData, nonce, messageType)
}

// TestNewMessageStorage tests creation of message storage
func TestNewMessageStorage(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := NewMessageStorage(keyPair)
	if storage == nil {
		t.Fatal("NewMessageStorage returned nil")
	}

	if storage.keyPair != keyPair {
		t.Error("Storage key pair not set correctly")
	}

	stats := storage.GetStorageStats()
	if stats.TotalMessages != 0 || stats.UniqueRecipients != 0 {
		t.Error("New storage should be empty")
	}
}

// TestStoreMessage tests message storage functionality
func TestStoreMessage(t *testing.T) {
	// Create storage with a key pair
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	storage := NewMessageStorage(storageKeyPair)

	// Create sender and recipient key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Test message storage
	message := "Hello, async world!"
	
	// Encrypt the message first
	encryptedData, nonce, err := EncryptForRecipient([]byte(message), recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to encrypt message: %v", err)
	}
	
	messageID, err := storage.StoreMessage(recipientKeyPair.Public, senderKeyPair.Public, 
		encryptedData, nonce, MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to store message: %v", err)
	}

	if messageID == [16]byte{} {
		t.Error("Message ID should not be zero")
	}

	// Verify storage stats
	stats := storage.GetStorageStats()
	if stats.TotalMessages != 1 {
		t.Errorf("Expected 1 message in storage, got %d", stats.TotalMessages)
	}
	if stats.UniqueRecipients != 1 {
		t.Errorf("Expected 1 unique recipient, got %d", stats.UniqueRecipients)
	}
}

// TestStoreMessageValidation tests input validation for message storage
func TestStoreMessageValidation(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := NewMessageStorage(keyPair)

	recipientPK, senderPK := [32]byte{}, [32]byte{}

	tests := []struct {
		name        string
		message     []byte
		expectError bool
	}{
		{"Empty message", []byte{}, true},
		{"Normal message", []byte("Hello"), false},
		{"Maximum length message", make([]byte, MaxMessageSize), false},
		{"Too long message", make([]byte, MaxMessageSize+1), true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(test.message) == 0 {
				// Test empty message directly 
				_, err := storage.StoreMessage(recipientPK, senderPK, []byte{}, [24]byte{}, MessageTypeNormal)
				if !test.expectError && err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			// Encrypt message first for non-empty tests
			encryptedData, nonce, err := EncryptForRecipient(test.message, recipientPK, [32]byte{})
			if test.expectError && len(test.message) > MaxMessageSize {
				// Should fail at encryption stage for too-long messages
				if err == nil {
					t.Error("Expected error for too-long message")
				}
				return
			}
			if err != nil && !test.expectError {
				t.Errorf("Unexpected encryption error: %v", err)
				return
			}
			
			_, err = storage.StoreMessage(recipientPK, senderPK, encryptedData, nonce, MessageTypeNormal)
			if test.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestRetrieveMessages tests message retrieval functionality
func TestRetrieveMessages(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	storage := NewMessageStorage(storageKeyPair)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	// Store multiple messages for the same recipient
	messages := []string{"Message 1", "Message 2", "Message 3"}
	for _, msg := range messages {
		_, err := storeTestMessage(storage, recipientKeyPair.Public, senderKeyPair.Public, senderKeyPair.Private, msg, MessageTypeNormal)
		if err != nil {
			t.Fatalf("Failed to store message: %v", err)
		}
	}

	// Retrieve messages
	retrieved, err := storage.RetrieveMessages(recipientKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to retrieve messages: %v", err)
	}

	if len(retrieved) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(retrieved))
	}

	// Test retrieval for non-existent recipient
	nonExistentPK := [32]byte{0xFF}
	_, err = storage.RetrieveMessages(nonExistentPK)
	if err != ErrMessageNotFound {
		t.Errorf("Expected ErrMessageNotFound, got %v", err)
	}
}

// TestDeleteMessage tests message deletion functionality
func TestDeleteMessage(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	storage := NewMessageStorage(storageKeyPair)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	// Store a message
	messageID, err := storeTestMessage(storage, recipientKeyPair.Public, senderKeyPair.Public, senderKeyPair.Private, "Test message", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to store message: %v", err)
	}

	// Delete the message
	err = storage.DeleteMessage(messageID, recipientKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to delete message: %v", err)
	}

	// Verify message is gone
	_, err = storage.RetrieveMessages(recipientKeyPair.Public)
	if err != ErrMessageNotFound {
		t.Error("Message should have been deleted")
	}

	// Test unauthorized deletion
	anotherKeyPair, _ := crypto.GenerateKeyPair()
	messageID2, _ := storeTestMessage(storage, recipientKeyPair.Public, senderKeyPair.Public, senderKeyPair.Private, "Another message", MessageTypeNormal)
	
	err = storage.DeleteMessage(messageID2, anotherKeyPair.Public)
	if err == nil {
		t.Error("Should not allow unauthorized deletion")
	}
}

// TestCleanupExpiredMessages tests message expiration functionality
func TestCleanupExpiredMessages(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	storage := NewMessageStorage(storageKeyPair)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	// Store a message
	messageID, err := storeTestMessage(storage, recipientKeyPair.Public, senderKeyPair.Public, senderKeyPair.Private, "Test message", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to store message: %v", err)
	}

	// Manually set the message to be expired
	storage.mutex.Lock()
	if msg, exists := storage.messages[messageID]; exists {
		msg.Timestamp = time.Now().Add(-(MaxStorageTime + time.Hour))
	}
	storage.mutex.Unlock()

	// Run cleanup
	expiredCount := storage.CleanupExpiredMessages()
	if expiredCount != 1 {
		t.Errorf("Expected 1 expired message, got %d", expiredCount)
	}

	// Verify message is gone
	_, err = storage.RetrieveMessages(recipientKeyPair.Public)
	if err != ErrMessageNotFound {
		t.Error("Expired message should have been cleaned up")
	}
}

// TestAsyncClient tests the client functionality
func TestAsyncClient(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	client := NewAsyncClient(keyPair)
	if client == nil {
		t.Fatal("NewAsyncClient returned nil")
	}

	if client.keyPair != keyPair {
		t.Error("Client key pair not set correctly")
	}

	// Test adding storage node
	storageNodePK := [32]byte{0x12, 0x34}
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	client.AddStorageNode(storageNodePK, addr)

	client.mutex.RLock()
	if client.storageNodes[storageNodePK] != addr {
		t.Error("Storage node not added correctly")
	}
	client.mutex.RUnlock()
}

// TestAsyncManager tests the manager functionality
func TestAsyncManager(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	manager := NewAsyncManager(keyPair, true)
	if manager == nil {
		t.Fatal("NewAsyncManager returned nil")
	}

	// Test start and stop
	manager.Start()
	if !manager.running {
		t.Error("Manager should be running after Start()")
	}

	manager.Stop()
	if manager.running {
		t.Error("Manager should not be running after Stop()")
	}

	// Test friend online status
	friendPK := [32]byte{0x11, 0x22}
	manager.SetFriendOnlineStatus(friendPK, true)
	
	manager.mutex.RLock()
	if !manager.onlineStatus[friendPK] {
		t.Error("Friend should be marked as online")
	}
	manager.mutex.RUnlock()

	// Test storage stats (when acting as storage node)
	stats := manager.GetStorageStats()
	if stats == nil {
		t.Error("Storage stats should be available when acting as storage node")
	}
}

// TestMessageTypeConstants tests that message type constants are defined correctly
func TestMessageTypeConstants(t *testing.T) {
	if MessageTypeNormal != 0 {
		t.Errorf("MessageTypeNormal should be 0, got %d", MessageTypeNormal)
	}
	if MessageTypeAction != 1 {
		t.Errorf("MessageTypeAction should be 1, got %d", MessageTypeAction)
	}
}

// TestStorageCapacityLimits tests storage capacity enforcement
func TestStorageCapacityLimits(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	storage := NewMessageStorage(storageKeyPair)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	// Test per-recipient limit
	for i := 0; i < MaxMessagesPerRecipient; i++ {
		_, err := storeTestMessage(storage, recipientKeyPair.Public, senderKeyPair.Public, senderKeyPair.Private, "Test message", MessageTypeNormal)
		if err != nil {
			t.Fatalf("Failed to store message %d: %v", i, err)
		}
	}

	// Next message should fail due to per-recipient limit
	_, err = storeTestMessage(storage, recipientKeyPair.Public, senderKeyPair.Public, senderKeyPair.Private, "Overflow message", MessageTypeNormal)
	if err == nil {
		t.Error("Should have failed due to per-recipient limit")
	}
}
