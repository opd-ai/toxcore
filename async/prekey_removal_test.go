package async

import (
	"os"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

func TestPreKeyRemovalAfterUse(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "prekey-removal-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a key pair for testing
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	// Create a peer key for testing
	peerKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	
	// Create a new pre-key store
	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}
	
	// Generate pre-keys
	_, err = store.GeneratePreKeys(peerKey)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}
	
	// Get initial key count
	initialCount := store.GetRemainingKeyCount(peerKey)
	if initialCount <= 0 {
		t.Fatalf("Expected positive key count, got %d", initialCount)
	}
	
	// Get an available pre-key
	preKey, err := store.GetAvailablePreKey(peerKey)
	if err != nil {
		t.Fatalf("Failed to get available pre-key: %v", err)
	}
	
	// Verify that the pre-key is valid
	if preKey == nil || preKey.KeyPair == nil {
		t.Fatalf("Returned pre-key is nil or missing key pair")
	}

	// Verify the pre-key was marked as used
	if !preKey.Used {
		t.Fatalf("Pre-key was not marked as used")
	}
	
	// Get key count after using one key
	countAfterUse := store.GetRemainingKeyCount(peerKey)
	if countAfterUse != initialCount-1 {
		t.Fatalf("Expected key count to decrease by 1, got initial=%d, after=%d", initialCount, countAfterUse)
	}
	
	// Create a new store instance to test loading from disk
	store2, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create second pre-key store: %v", err)
	}
	
	// Check that the used key is actually removed from storage, not just marked as used
	bundle, err := store2.GetBundle(peerKey)
	if err != nil {
		t.Fatalf("Failed to get bundle from new store instance: %v", err)
	}
	
	// Check that the count of used keys matches what we expect
	if bundle.UsedCount != 1 {
		t.Fatalf("Expected UsedCount=1, got %d", bundle.UsedCount)
	}
	
	// Verify that there are no entries in the bundle that are marked as used
	// This confirms that used keys are completely removed, not just marked
	usedKeysFound := 0
	for _, key := range bundle.Keys {
		if key.Used {
			usedKeysFound++
		}
	}
	
	if usedKeysFound > 0 {
		t.Fatalf("Found %d keys marked as used, expected 0 (keys should be removed not marked)", usedKeysFound)
	}
	
	// Verify the total key count matches what we expect (original count minus used keys)
	if len(bundle.Keys) != PreKeysPerPeer-1 {
		t.Fatalf("Expected %d keys in bundle, got %d", PreKeysPerPeer-1, len(bundle.Keys))
	}
}
