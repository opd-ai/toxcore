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

	encryptedData, nonce, err := encryptForRecipientInternal([]byte(message), recipientPK, senderSK)
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

	storage := NewMessageStorage(keyPair, "/tmp")
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

	storage := NewMessageStorage(storageKeyPair, "/tmp")

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
	encryptedData, nonce, err := encryptForRecipientInternal([]byte(message), recipientKeyPair.Public, senderKeyPair.Private)
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

	storage := NewMessageStorage(keyPair, "/tmp")

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
			encryptedData, nonce, err := encryptForRecipientInternal(test.message, recipientPK, [32]byte{})
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

	storage := NewMessageStorage(storageKeyPair, "/tmp")

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

	storage := NewMessageStorage(storageKeyPair, "/tmp")

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

	storage := NewMessageStorage(storageKeyPair, "/tmp")

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

	manager, err := NewAsyncManager(keyPair, "/tmp")
	if err != nil {
		t.Fatalf("Failed to create AsyncManager: %v", err)
	}
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

	storage := NewMessageStorage(storageKeyPair, "/tmp")

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

// TestStoreObfuscatedMessage tests obfuscated message storage functionality
func TestStoreObfuscatedMessage(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := NewMessageStorage(keyPair, "/tmp")

	// Create a test obfuscated message
	obfManager := NewObfuscationManager(keyPair, storage.epochManager)

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Simulate shared secret for testing
	sharedSecret := [32]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20}

	forwardSecureMsg := []byte("Test obfuscated message")

	obfMsg, err := obfManager.CreateObfuscatedMessage(
		senderKeyPair.Private,
		recipientKeyPair.Public,
		forwardSecureMsg,
		sharedSecret,
	)
	if err != nil {
		t.Fatalf("Failed to create obfuscated message: %v", err)
	}

	// Test storing the obfuscated message
	err = storage.StoreObfuscatedMessage(obfMsg)
	if err != nil {
		t.Fatalf("Failed to store obfuscated message: %v", err)
	}

	// Verify storage statistics
	stats := storage.GetStorageStats()
	if stats.ObfuscatedMessages != 1 {
		t.Errorf("Expected 1 obfuscated message, got %d", stats.ObfuscatedMessages)
	}

	if stats.TotalMessages != 1 {
		t.Errorf("Expected 1 total message, got %d", stats.TotalMessages)
	}

	if stats.UniquePseudonyms != 1 {
		t.Errorf("Expected 1 unique pseudonym, got %d", stats.UniquePseudonyms)
	}
}

// TestStoreObfuscatedMessageValidation tests validation in obfuscated message storage
func TestStoreObfuscatedMessageValidation(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := NewMessageStorage(keyPair, "/tmp")
	obfManager := NewObfuscationManager(keyPair, storage.epochManager)

	// Test nil message
	err = storage.StoreObfuscatedMessage(nil)
	if err == nil {
		t.Error("Should have failed with nil message")
	}

	// Create base message for other tests
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()
	sharedSecret := [32]byte{0x01, 0x02, 0x03}
	forwardSecureMsg := []byte("Test message")

	obfMsg, err := obfManager.CreateObfuscatedMessage(
		senderKeyPair.Private,
		recipientKeyPair.Public,
		forwardSecureMsg,
		sharedSecret,
	)
	if err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}

	// Test expired message
	expiredMsg := *obfMsg
	expiredMsg.ExpiresAt = time.Now().Add(-1 * time.Hour)
	err = storage.StoreObfuscatedMessage(&expiredMsg)
	if err == nil {
		t.Error("Should have failed with expired message")
	}

	// Test empty payload
	emptyMsg := *obfMsg
	emptyMsg.EncryptedPayload = []byte{}
	err = storage.StoreObfuscatedMessage(&emptyMsg)
	if err == nil {
		t.Error("Should have failed with empty payload")
	}

	// Test oversized payload
	oversizedMsg := *obfMsg
	oversizedMsg.EncryptedPayload = make([]byte, MaxMessageSize+EncryptionOverhead+1)
	err = storage.StoreObfuscatedMessage(&oversizedMsg)
	if err == nil {
		t.Error("Should have failed with oversized payload")
	}
}

// TestRetrieveMessagesByPseudonym tests pseudonym-based message retrieval
func TestRetrieveMessagesByPseudonym(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := NewMessageStorage(keyPair, "/tmp")
	obfManager := NewObfuscationManager(keyPair, storage.epochManager)

	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()
	sharedSecret := [32]byte{0x01, 0x02, 0x03}

	// Store test messages
	var testMessages []*ObfuscatedAsyncMessage
	for i := 0; i < 3; i++ {
		forwardSecureMsg := []byte("Test message " + string(rune('0'+i)))
		obfMsg, err := obfManager.CreateObfuscatedMessage(
			senderKeyPair.Private,
			recipientKeyPair.Public,
			forwardSecureMsg,
			sharedSecret,
		)
		if err != nil {
			t.Fatalf("Failed to create test message %d: %v", i, err)
		}

		err = storage.StoreObfuscatedMessage(obfMsg)
		if err != nil {
			t.Fatalf("Failed to store test message %d: %v", i, err)
		}

		testMessages = append(testMessages, obfMsg)
	}

	// Test retrieval by pseudonym
	recipientPseudonym := testMessages[0].RecipientPseudonym
	epochs := []uint64{testMessages[0].Epoch}

	retrievedMessages, err := storage.RetrieveMessagesByPseudonym(recipientPseudonym, epochs)
	if err != nil {
		t.Fatalf("Failed to retrieve messages: %v", err)
	}

	// All messages should have the same recipient pseudonym since they're for the same recipient in the same epoch
	if len(retrievedMessages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(retrievedMessages))
	}

	// Test retrieval with non-existent pseudonym
	nonExistentPseudonym := [32]byte{0xFF, 0xFF, 0xFF}
	_, err = storage.RetrieveMessagesByPseudonym(nonExistentPseudonym, epochs)
	if err != ErrMessageNotFound {
		t.Errorf("Expected ErrMessageNotFound, got %v", err)
	}
}

// TestRetrieveRecentObfuscatedMessages tests recent message retrieval
func TestRetrieveRecentObfuscatedMessages(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := NewMessageStorage(keyPair, "/tmp")
	obfManager := NewObfuscationManager(keyPair, storage.epochManager)

	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()
	sharedSecret := [32]byte{0x01, 0x02, 0x03}

	// Store a test message
	forwardSecureMsg := []byte("Test recent message")
	obfMsg, err := obfManager.CreateObfuscatedMessage(
		senderKeyPair.Private,
		recipientKeyPair.Public,
		forwardSecureMsg,
		sharedSecret,
	)
	if err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}

	err = storage.StoreObfuscatedMessage(obfMsg)
	if err != nil {
		t.Fatalf("Failed to store test message: %v", err)
	}

	// Test retrieval of recent messages
	retrievedMessages, err := storage.RetrieveRecentObfuscatedMessages(obfMsg.RecipientPseudonym)
	if err != nil {
		t.Fatalf("Failed to retrieve recent messages: %v", err)
	}

	if len(retrievedMessages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(retrievedMessages))
	}

	if retrievedMessages[0].MessageID != obfMsg.MessageID {
		t.Error("Retrieved message ID doesn't match stored message")
	}
}

// TestDeleteObfuscatedMessage tests obfuscated message deletion
func TestDeleteObfuscatedMessage(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := NewMessageStorage(keyPair, "/tmp")
	obfManager := NewObfuscationManager(keyPair, storage.epochManager)

	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()
	sharedSecret := [32]byte{0x01, 0x02, 0x03}

	// Store a test message
	forwardSecureMsg := []byte("Test delete message")
	obfMsg, err := obfManager.CreateObfuscatedMessage(
		senderKeyPair.Private,
		recipientKeyPair.Public,
		forwardSecureMsg,
		sharedSecret,
	)
	if err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}

	err = storage.StoreObfuscatedMessage(obfMsg)
	if err != nil {
		t.Fatalf("Failed to store test message: %v", err)
	}

	// Verify message exists
	stats := storage.GetStorageStats()
	if stats.ObfuscatedMessages != 1 {
		t.Errorf("Expected 1 obfuscated message before deletion, got %d", stats.ObfuscatedMessages)
	}

	// Test successful deletion
	err = storage.DeleteObfuscatedMessage(obfMsg.MessageID, obfMsg.RecipientPseudonym)
	if err != nil {
		t.Fatalf("Failed to delete obfuscated message: %v", err)
	}

	// Verify message was deleted
	stats = storage.GetStorageStats()
	if stats.ObfuscatedMessages != 0 {
		t.Errorf("Expected 0 obfuscated messages after deletion, got %d", stats.ObfuscatedMessages)
	}

	// Test deletion of non-existent message
	err = storage.DeleteObfuscatedMessage(obfMsg.MessageID, obfMsg.RecipientPseudonym)
	if err != ErrMessageNotFound {
		t.Errorf("Expected ErrMessageNotFound for non-existent message, got %v", err)
	}

	// Test unauthorized deletion
	wrongPseudonym := [32]byte{0xFF, 0xFF, 0xFF}
	err = storage.DeleteObfuscatedMessage(obfMsg.MessageID, wrongPseudonym)
	if err != ErrMessageNotFound {
		t.Errorf("Expected ErrMessageNotFound for wrong pseudonym, got %v", err)
	}
}

// TestCleanupOldEpochs tests cleanup of messages from old epochs
func TestCleanupOldEpochs(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := NewMessageStorage(keyPair, "/tmp")

	// Create a custom epoch manager for testing with very short epochs
	customEpochManager, err := NewEpochManagerWithCustomStart(time.Now().Add(-10*time.Hour), 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create custom epoch manager: %v", err)
	}
	storage.epochManager = customEpochManager

	obfManager := NewObfuscationManager(keyPair, storage.epochManager)

	// Create an old message (5 epochs ago, which should be cleaned up)
	oldEpoch := storage.epochManager.GetCurrentEpoch() - 5
	recipientPK := [32]byte{0x01, 0x02, 0x03}
	recipientPseudonym, err := obfManager.GenerateRecipientPseudonym(recipientPK, oldEpoch)
	if err != nil {
		t.Fatalf("Failed to generate recipient pseudonym: %v", err)
	}

	oldMsg := &ObfuscatedAsyncMessage{
		MessageID:          [32]byte{0x01, 0x02, 0x03},
		RecipientPseudonym: recipientPseudonym,
		Epoch:              oldEpoch,
		EncryptedPayload:   []byte("old message"),
		ExpiresAt:          time.Now().Add(1 * time.Hour),
	}

	// Manually add the old message to simulate it being stored in the past
	storage.mutex.Lock()
	storage.obfuscatedMessages[oldMsg.MessageID] = oldMsg
	if storage.pseudonymIndex[oldMsg.RecipientPseudonym] == nil {
		storage.pseudonymIndex[oldMsg.RecipientPseudonym] = make(map[uint64][]*ObfuscatedAsyncMessage)
	}
	storage.pseudonymIndex[oldMsg.RecipientPseudonym][oldMsg.Epoch] = []*ObfuscatedAsyncMessage{oldMsg}
	storage.mutex.Unlock()

	// Verify message exists
	stats := storage.GetStorageStats()
	if stats.ObfuscatedMessages != 1 {
		t.Errorf("Expected 1 obfuscated message before cleanup, got %d", stats.ObfuscatedMessages)
	}

	// Run cleanup
	cleanedCount := storage.CleanupOldEpochs()
	if cleanedCount != 1 {
		t.Errorf("Expected 1 message cleaned up, got %d", cleanedCount)
	}

	// Verify message was cleaned up
	stats = storage.GetStorageStats()
	if stats.ObfuscatedMessages != 0 {
		t.Errorf("Expected 0 obfuscated messages after cleanup, got %d", stats.ObfuscatedMessages)
	}
}

// TestMixedStorageCleanup tests cleanup with both legacy and obfuscated messages
func TestMixedStorageCleanup(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := NewMessageStorage(keyPair, "/tmp")

	// Store a legacy message
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()

	_, err = storeTestMessage(storage, recipientKeyPair.Public, senderKeyPair.Public, senderKeyPair.Private, "Legacy message", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to store legacy message: %v", err)
	}

	// Store an obfuscated message
	obfManager := NewObfuscationManager(keyPair, storage.epochManager)
	sharedSecret := [32]byte{0x01, 0x02, 0x03}
	forwardSecureMsg := []byte("Obfuscated message")

	obfMsg, err := obfManager.CreateObfuscatedMessage(
		senderKeyPair.Private,
		recipientKeyPair.Public,
		forwardSecureMsg,
		sharedSecret,
	)
	if err != nil {
		t.Fatalf("Failed to create obfuscated message: %v", err)
	}

	err = storage.StoreObfuscatedMessage(obfMsg)
	if err != nil {
		t.Fatalf("Failed to store obfuscated message: %v", err)
	}

	// Verify both messages exist
	stats := storage.GetStorageStats()
	if stats.TotalMessages != 2 {
		t.Errorf("Expected 2 total messages, got %d", stats.TotalMessages)
	}
	if stats.LegacyMessages != 1 {
		t.Errorf("Expected 1 legacy message, got %d", stats.LegacyMessages)
	}
	if stats.ObfuscatedMessages != 1 {
		t.Errorf("Expected 1 obfuscated message, got %d", stats.ObfuscatedMessages)
	}

	// Test storage utilization
	utilization := storage.GetStorageUtilization()
	if utilization <= 0 {
		t.Error("Storage utilization should be greater than 0")
	}
}
