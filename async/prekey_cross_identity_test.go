package async

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestPreKeyStore_CrossIdentityBundleHandling tests that bundles encrypted
// with a different identity key are silently skipped without warnings
func TestPreKeyStore_CrossIdentityBundleHandling(t *testing.T) {
	// Create two temporary directories for two different identities
	dir1, err := os.MkdirTemp("", "prekey_identity1_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(dir1)

	dir2, err := os.MkdirTemp("", "prekey_identity2_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(dir2)

	// Create two different key pairs (two identities)
	keyPair1, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair 1: %v", err)
	}

	keyPair2, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair 2: %v", err)
	}

	// Create a peer public key for testing
	peerPK, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate peer key: %v", err)
	}

	// Identity 1: Create pre-key store and generate bundles
	store1, err := NewPreKeyStore(keyPair1, dir1)
	if err != nil {
		t.Fatalf("Failed to create store 1: %v", err)
	}

	_, err = store1.GeneratePreKeys(peerPK.Public)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys for identity 1: %v", err)
	}

	// Verify bundle was created for identity 1
	bundleFilename := fmt.Sprintf("%x.json.enc", peerPK.Public)
	bundlePath1 := filepath.Join(dir1, "prekeys", bundleFilename)
	if _, err := os.Stat(bundlePath1); os.IsNotExist(err) {
		t.Fatalf("Bundle file was not created for identity 1")
	}

	// Copy identity 1's bundle file to identity 2's directory
	// This simulates leftover bundles from previous test runs with different keys
	preKeyDir2 := filepath.Join(dir2, "prekeys")
	if err := os.MkdirAll(preKeyDir2, 0o755); err != nil {
		t.Fatalf("Failed to create prekeys directory for identity 2: %v", err)
	}

	bundleData, err := os.ReadFile(bundlePath1)
	if err != nil {
		t.Fatalf("Failed to read bundle from identity 1: %v", err)
	}

	crossIdentityBundlePath := filepath.Join(preKeyDir2, bundleFilename)
	if err := os.WriteFile(crossIdentityBundlePath, bundleData, 0o600); err != nil {
		t.Fatalf("Failed to write cross-identity bundle: %v", err)
	}

	// Identity 2: Create pre-key store, which should silently skip the cross-identity bundle
	store2, err := NewPreKeyStore(keyPair2, dir2)
	if err != nil {
		t.Fatalf("Failed to create store 2: %v", err)
	}

	// Verify that the cross-identity bundle was NOT loaded
	if len(store2.bundles) != 0 {
		t.Errorf("Expected 0 bundles loaded for identity 2, got %d", len(store2.bundles))
	}

	// Verify the bundle file still exists (not deleted, just skipped)
	if _, err := os.Stat(crossIdentityBundlePath); os.IsNotExist(err) {
		t.Errorf("Cross-identity bundle file should not be deleted, just skipped")
	}

	// Now generate a legitimate bundle for identity 2 with a different peer
	otherPeerPK, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate other peer key: %v", err)
	}

	_, err = store2.GeneratePreKeys(otherPeerPK.Public)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys for identity 2: %v", err)
	}

	// Verify identity 2 can load its own bundle
	if len(store2.bundles) != 1 {
		t.Errorf("Expected 1 bundle loaded for identity 2, got %d", len(store2.bundles))
	}

	// Recreate store2 to test loading from disk
	store2Reloaded, err := NewPreKeyStore(keyPair2, dir2)
	if err != nil {
		t.Fatalf("Failed to recreate store 2: %v", err)
	}

	// Should load only its own bundle, not the cross-identity one
	if len(store2Reloaded.bundles) != 1 {
		t.Errorf("Expected 1 bundle after reload for identity 2, got %d", len(store2Reloaded.bundles))
	}

	// Verify it's the correct bundle
	if _, exists := store2Reloaded.bundles[otherPeerPK.Public]; !exists {
		t.Errorf("Expected to find bundle for other peer, but it's missing")
	}
}

// TestPreKeyStore_CorruptedBundleHandling tests that truly corrupted bundles
// (not just wrong identity) are logged as warnings
func TestPreKeyStore_CorruptedBundleHandling(t *testing.T) {
	dir, err := os.MkdirTemp("", "prekey_corrupted_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create prekeys directory
	preKeyDir := filepath.Join(dir, "prekeys")
	if err := os.MkdirAll(preKeyDir, 0o755); err != nil {
		t.Fatalf("Failed to create prekeys directory: %v", err)
	}

	// Write a corrupted bundle file (invalid data)
	corruptedPath := filepath.Join(preKeyDir, "corrupted_bundle.json.enc")
	if err := os.WriteFile(corruptedPath, []byte("invalid encrypted data"), 0o600); err != nil {
		t.Fatalf("Failed to write corrupted bundle: %v", err)
	}

	// Create store - should handle corrupted bundle gracefully
	store, err := NewPreKeyStore(keyPair, dir)
	if err != nil {
		t.Fatalf("Failed to create store with corrupted bundle: %v", err)
	}

	// Should have 0 bundles loaded (corrupted one was skipped)
	if len(store.bundles) != 0 {
		t.Errorf("Expected 0 bundles with corrupted file, got %d", len(store.bundles))
	}
}

// TestPreKeyStore_MixedIdentityBundles tests a realistic scenario with
// multiple bundles from different test runs
func TestPreKeyStore_MixedIdentityBundles(t *testing.T) {
	dir, err := os.MkdirTemp("", "prekey_mixed_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Simulate 3 different test runs with different identities
	identities := make([]*crypto.KeyPair, 3)
	for i := 0; i < 3; i++ {
		kp, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate identity %d: %v", i, err)
		}
		identities[i] = kp

		// Each identity generates bundles
		store, err := NewPreKeyStore(kp, dir)
		if err != nil {
			t.Fatalf("Failed to create store for identity %d: %v", i, err)
		}

		peerPK, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate peer key for identity %d: %v", i, err)
		}

		_, err = store.GeneratePreKeys(peerPK.Public)
		if err != nil {
			t.Fatalf("Failed to generate pre-keys for identity %d: %v", i, err)
		}
	}

	// Now load with the last identity - should only load its own bundle
	finalStore, err := NewPreKeyStore(identities[2], dir)
	if err != nil {
		t.Fatalf("Failed to create final store: %v", err)
	}

	// Should only load 1 bundle (the one for identity 2)
	if len(finalStore.bundles) != 1 {
		t.Errorf("Expected 1 bundle for final identity, got %d", len(finalStore.bundles))
	}
}
