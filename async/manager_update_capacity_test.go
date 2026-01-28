package async

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestUpdateStorageCapacity verifies that the UpdateStorageCapacity method
// correctly delegates to the underlying storage's UpdateCapacity method
func TestUpdateStorageCapacity(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	tmpDir := t.TempDir()

	manager, err := NewAsyncManager(keyPair, NewMockTransport("127.0.0.1:33445"), tmpDir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	// Get initial capacity directly from storage
	initialCapacity := manager.storage.GetMaxCapacity()

	// Call UpdateStorageCapacity
	err = manager.UpdateStorageCapacity()
	if err != nil {
		t.Fatalf("UpdateStorageCapacity failed: %v", err)
	}

	// Verify capacity was updated (should recalculate based on disk space)
	updatedCapacity := manager.storage.GetMaxCapacity()

	// The capacity should be set (even if it's the same value)
	if updatedCapacity == 0 {
		t.Error("Max capacity should not be zero after update")
	}

	// Log capacity values for debugging
	t.Logf("Initial capacity: %d, Updated capacity: %d", initialCapacity, updatedCapacity)
}

// TestUpdateStorageCapacityNonStorageNode verifies that calling UpdateStorageCapacity
// on a non-storage node returns an appropriate error
func TestUpdateStorageCapacityNonStorageNode(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	tmpDir := t.TempDir()

	manager, err := NewAsyncManager(keyPair, NewMockTransport("127.0.0.1:33445"), tmpDir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	// Manually set isStorageNode to false to simulate non-storage node
	manager.isStorageNode = false

	// Call UpdateStorageCapacity should return error
	err = manager.UpdateStorageCapacity()
	if err == nil {
		t.Fatal("UpdateStorageCapacity should fail for non-storage node")
	}

	expectedErrMsg := "not acting as storage node"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
	}
}

// TestUpdateStorageCapacityAfterMessages verifies that UpdateStorageCapacity
// works correctly after messages have been stored
func TestUpdateStorageCapacityAfterMessages(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	tmpDir := t.TempDir()

	manager, err := NewAsyncManager(keyPair, NewMockTransport("127.0.0.1:33445"), tmpDir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	// Store a message
	message := "Test message for capacity update"
	encryptedData, nonce, err := encryptForRecipientInternal([]byte(message), recipientKeyPair.Public, keyPair.Private)
	if err != nil {
		t.Fatalf("Failed to encrypt message: %v", err)
	}

	_, err = manager.storage.StoreMessage(recipientKeyPair.Public, keyPair.Public, encryptedData, nonce, MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to store message: %v", err)
	}

	// Verify message is stored
	stats := manager.GetStorageStats()
	if stats.TotalMessages != 1 {
		t.Errorf("Expected 1 message, got %d", stats.TotalMessages)
	}

	// Update capacity
	err = manager.UpdateStorageCapacity()
	if err != nil {
		t.Fatalf("UpdateStorageCapacity failed: %v", err)
	}

	// Verify stats are still correct after capacity update
	statsAfter := manager.GetStorageStats()
	if statsAfter.TotalMessages != 1 {
		t.Errorf("Expected 1 message after capacity update, got %d", statsAfter.TotalMessages)
	}

	updatedCapacity := manager.storage.GetMaxCapacity()
	if updatedCapacity == 0 {
		t.Error("Max capacity should not be zero after update")
	}
}

// TestUpdateStorageCapacityREADMEExample verifies that the README example pattern works
func TestUpdateStorageCapacityREADMEExample(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	tmpDir := t.TempDir()

	asyncManager, err := NewAsyncManager(keyPair, NewMockTransport("127.0.0.1:33445"), tmpDir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	// This is the pattern from README.md line 932
	// It should compile and execute without error
	err = asyncManager.UpdateStorageCapacity()
	if err != nil {
		t.Fatalf("README example pattern failed: %v", err)
	}
}
