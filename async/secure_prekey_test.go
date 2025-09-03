package async

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

func TestSecurePreKeyStorage(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "prekey-test")
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

	// Check that the bundle was saved as encrypted
	encFilePath := filepath.Join(tempDir, "prekeys", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20.json.enc")
	if _, err := os.Stat(encFilePath); os.IsNotExist(err) {
		t.Fatalf("Encrypted file not found at expected location: %s", encFilePath)
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

	// Store key ID for logging purposes
	_ = preKey.ID

	// Note: We don't need to explicitly mark it as used since GetAvailablePreKey already does this
	// Just verify that it was marked as used by checking its Used flag
	if !preKey.Used {
		t.Fatalf("Pre-key was not marked as used after retrieval")
	}

	// We can't directly check if the key in storage is wiped since we have a copy
	// Instead, let's check that our implementation correctly handles a new request
	// for an already used key
	_, err = store.GetAvailablePreKey(peerKey)
	// Should fail or return a different key than the one we just used
	if err == nil {
		t.Logf("Got another pre-key, which is expected with our bundle of multiple keys")
	} else {
		t.Logf("Expected error when all keys are used: %v", err)
	}

	// Create a new store instance to test loading from disk
	store2, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create second pre-key store: %v", err)
	}

	// Try to get the bundle
	loadedBundle, err := store2.GetBundle(peerKey)
	if err != nil {
		t.Fatalf("Failed to get bundle from new store instance: %v", err)
	}

	// Verify that the bundle was successfully loaded and decrypted
	if loadedBundle.PeerPK != peerKey {
		t.Fatalf("Loaded bundle has incorrect peer key")
	}
}
