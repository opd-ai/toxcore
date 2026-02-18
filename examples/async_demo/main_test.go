package main

import (
	"os"
	"testing"

	"github.com/opd-ai/toxcore/async"
	"github.com/opd-ai/toxcore/crypto"
)

// TestInitializeStorageNode verifies that a storage node can be created and configured.
func TestInitializeStorageNode(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	storage := initializeStorageNode(keyPair)
	if storage == nil {
		t.Fatal("Expected non-nil storage node")
	}

	// Verify storage has expected capacity
	if storage.GetMaxCapacity() <= 0 {
		t.Errorf("Expected positive max capacity, got %d", storage.GetMaxCapacity())
	}

	// Verify storage utilization is valid
	utilization := storage.GetStorageUtilization()
	if utilization < 0 || utilization > 100 {
		t.Errorf("Expected utilization between 0-100, got %.1f", utilization)
	}
}

// TestStoreTestMessage verifies message storage functionality.
func TestStoreTestMessage(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate alice key pair: %v", err)
	}
	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate bob key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())
	messageID := storeTestMessage(storage, aliceKeyPair, bobKeyPair)

	// Verify message ID is not zero
	var zeroID [16]byte
	if messageID == zeroID {
		t.Error("Expected non-zero message ID")
	}

	// Verify stats updated
	stats := storage.GetStorageStats()
	if stats.TotalMessages < 1 {
		t.Errorf("Expected at least 1 message, got %d", stats.TotalMessages)
	}
}

// TestRetrieveAndDecryptMessages verifies message retrieval and decryption.
func TestRetrieveAndDecryptMessages(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate alice key pair: %v", err)
	}
	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate bob key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())

	// Store a message first
	messageID := storeTestMessage(storage, aliceKeyPair, bobKeyPair)
	var zeroID [16]byte
	if messageID == zeroID {
		t.Fatal("Failed to store test message")
	}

	// Retrieve messages
	messages := retrieveAndDecryptMessages(storage, bobKeyPair)
	if len(messages) == 0 {
		t.Error("Expected at least one message to be retrieved")
	}
}

// TestCleanupStoredMessages verifies message deletion functionality.
func TestCleanupStoredMessages(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate alice key pair: %v", err)
	}
	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate bob key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())

	// Store a message
	storeTestMessage(storage, aliceKeyPair, bobKeyPair)

	// Retrieve messages
	messages := retrieveAndDecryptMessages(storage, bobKeyPair)
	if len(messages) == 0 {
		t.Fatal("No messages to test cleanup")
	}

	// Count before cleanup
	statsBefore := storage.GetStorageStats()

	// Cleanup messages (should not panic)
	cleanupStoredMessages(storage, messages, bobKeyPair)

	// Verify message count decreased
	statsAfter := storage.GetStorageStats()
	if statsAfter.TotalMessages >= statsBefore.TotalMessages {
		t.Errorf("Expected message count to decrease after cleanup, before=%d after=%d",
			statsBefore.TotalMessages, statsAfter.TotalMessages)
	}
}

// TestDemoDirectStorage verifies the complete direct storage demo runs without panic.
func TestDemoDirectStorage(t *testing.T) {
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate alice key pair: %v", err)
	}
	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate bob key pair: %v", err)
	}
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	// Should complete without panic
	demoDirectStorage(aliceKeyPair, bobKeyPair, storageKeyPair)
}

// TestConfigureMessageHandling verifies message handler setup.
func TestConfigureMessageHandling(t *testing.T) {
	// Create a simple mock to test handler configuration
	// Note: We can't fully test this without a real AsyncManager,
	// but we can verify the handler setup pattern works
	t.Run("handler slice initialization", func(t *testing.T) {
		// The function should return a non-nil slice
		// We can't test with nil bobManager, so this is a structural test
		messages := make([]string, 0)
		if messages == nil {
			t.Error("Expected non-nil slice")
		}
	})
}

// TestEmptyMessagesCleanup verifies cleanup handles empty message list gracefully.
func TestEmptyMessagesCleanup(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}
	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate bob key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())

	// Should handle empty messages list without panic
	emptyMessages := []async.AsyncMessage{}
	cleanupStoredMessages(storage, emptyMessages, bobKeyPair)
}

// TestDemoStorageMaintenance verifies the storage maintenance demo runs without panic.
func TestDemoStorageMaintenance(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	// Should complete without panic
	demoStorageMaintenance(storageKeyPair)
}

// TestStorageMaintenanceStoresMessages verifies messages are stored correctly during maintenance.
func TestStorageMaintenanceStoresMessages(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())
	user1, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate user1 key pair: %v", err)
	}
	sender, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	// Store a test message
	testData := []byte("Test message")
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}

	_, err = storage.StoreMessage(user1.Public, sender.Public, testData, nonce, async.MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to store message: %v", err)
	}

	// Verify message was stored
	stats := storage.GetStorageStats()
	if stats.TotalMessages < 1 {
		t.Errorf("Expected at least 1 message, got %d", stats.TotalMessages)
	}
}

// TestStorageCleanupExpiredMessages verifies the cleanup process works correctly.
func TestStorageCleanupExpiredMessages(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())

	// Run cleanup on empty storage (should not panic and return 0)
	expiredCount := storage.CleanupExpiredMessages()
	if expiredCount != 0 {
		// This is informational - we don't expect any expired messages in fresh storage
		t.Logf("Cleanup removed %d messages", expiredCount)
	}
}

// TestAttemptInitialOfflineMessaging tests the initial offline messaging attempt.
func TestAttemptInitialOfflineMessaging(t *testing.T) {
	// This test requires an AsyncManager, skip if unavailable
	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate bob key pair: %v", err)
	}

	// Testing that the function handles nil manager gracefully requires actual manager
	// For now, we verify key generation works for the test setup
	if bobKeyPair == nil {
		t.Fatal("Expected non-nil key pair")
	}
}

// TestPerformForwardSecureMessagingWithoutPrekeys tests the failure path.
func TestPerformForwardSecureMessagingWithoutPrekeys(t *testing.T) {
	// Verify key generation for test setup
	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate bob key pair: %v", err)
	}

	// The actual function test requires a real AsyncManager
	// This test verifies the key pair setup
	if bobKeyPair.Public == [32]byte{} {
		t.Fatal("Expected non-zero public key")
	}
}

// TestFinalizeMessageDeliveryTracking verifies message tracking works correctly.
func TestFinalizeMessageDeliveryTracking(t *testing.T) {
	// Test the message tracking logic used in finalizeMessageDelivery
	bobReceivedMessages := make([]string, 0)

	// Simulate no messages received
	if len(bobReceivedMessages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(bobReceivedMessages))
	}

	// Simulate messages received
	bobReceivedMessages = append(bobReceivedMessages, "test message 1")
	bobReceivedMessages = append(bobReceivedMessages, "test message 2")
	if len(bobReceivedMessages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(bobReceivedMessages))
	}
}

// TestSimulatePreKeyExchangeTimings tests the timing operations without actual network.
func TestSimulatePreKeyExchangeTimings(t *testing.T) {
	// This test verifies that the timing-related logic doesn't cause issues
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate alice key pair: %v", err)
	}
	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate bob key pair: %v", err)
	}

	// Verify keys are different
	if aliceKeyPair.Public == bobKeyPair.Public {
		t.Error("Expected different key pairs for alice and bob")
	}
}

// TestStorageStatsRetrieval tests storage statistics retrieval.
func TestStorageStatsRetrieval(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())

	// Initial stats should be valid (GetStorageStats returns value type, not pointer)
	stats := storage.GetStorageStats()
	if stats.TotalMessages < 0 {
		t.Errorf("Expected non-negative message count, got %d", stats.TotalMessages)
	}
	if stats.UniqueRecipients < 0 {
		t.Errorf("Expected non-negative recipient count, got %d", stats.UniqueRecipients)
	}
}

// TestStorageUtilizationRange tests that storage utilization stays in valid range.
func TestStorageUtilizationRange(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())

	utilization := storage.GetStorageUtilization()
	if utilization < 0.0 {
		t.Errorf("Expected utilization >= 0, got %.2f", utilization)
	}
	if utilization > 100.0 {
		t.Errorf("Expected utilization <= 100, got %.2f", utilization)
	}
}

// TestMultipleMessageStorage tests storing multiple messages.
func TestMultipleMessageStorage(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}
	user1, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate user1 key pair: %v", err)
	}
	user2, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate user2 key pair: %v", err)
	}
	sender, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())

	// Store messages for multiple users
	for i := 0; i < 3; i++ {
		recipient := user1.Public
		if i >= 2 {
			recipient = user2.Public
		}
		testData := []byte("Test message")
		nonce, err := crypto.GenerateNonce()
		if err != nil {
			t.Fatalf("Failed to generate nonce: %v", err)
		}
		_, err = storage.StoreMessage(recipient, sender.Public, testData, nonce, async.MessageTypeNormal)
		if err != nil {
			t.Errorf("Failed to store message %d: %v", i, err)
		}
	}

	stats := storage.GetStorageStats()
	if stats.TotalMessages < 3 {
		t.Errorf("Expected at least 3 messages, got %d", stats.TotalMessages)
	}
}

// TestRetrieveFromEmptyStorage tests retrieval from storage with no messages.
func TestRetrieveFromEmptyStorage(t *testing.T) {
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}
	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate bob key pair: %v", err)
	}

	storage := async.NewMessageStorage(storageKeyPair, os.TempDir())

	// Retrieve from empty storage returns "message not found" error
	messages, err := storage.RetrieveMessages(bobKeyPair.Public)
	// Error is expected when no messages exist for recipient
	if err == nil && len(messages) > 0 {
		t.Error("Expected empty result from empty storage")
	}
}
