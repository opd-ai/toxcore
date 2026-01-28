package async

import (
	"os"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestGetRemainingKeyCountAccuracy verifies that GetRemainingKeyCount
// accurately reflects the actual number of remaining keys after extraction.
// This test addresses the bug where GetRemainingKeyCount calculated from
// UsedCount instead of actual slice length, causing incorrect counts when
// keys are removed from storage.
func TestGetRemainingKeyCountAccuracy(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prekey-count-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	peerKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	// Generate initial bundle
	_, err = store.GeneratePreKeys(peerKey)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Initial count should be PreKeysPerPeer (100)
	initialCount := store.GetRemainingKeyCount(peerKey)
	if initialCount != PreKeysPerPeer {
		t.Fatalf("Expected initial count=%d, got %d", PreKeysPerPeer, initialCount)
	}

	// Extract multiple keys rapidly
	numKeysToExtract := 10
	for i := 0; i < numKeysToExtract; i++ {
		_, err := store.GetAvailablePreKey(peerKey)
		if err != nil {
			t.Fatalf("Failed to get pre-key %d: %v", i, err)
		}
	}

	// Verify count matches actual remaining keys
	countAfterExtraction := store.GetRemainingKeyCount(peerKey)
	expectedCount := PreKeysPerPeer - numKeysToExtract

	if countAfterExtraction != expectedCount {
		t.Fatalf("Expected count=%d after extracting %d keys, got %d",
			expectedCount, numKeysToExtract, countAfterExtraction)
	}

	// Verify bundle.Keys slice length matches the count
	bundle, err := store.GetBundle(peerKey)
	if err != nil {
		t.Fatalf("Failed to get bundle: %v", err)
	}

	actualKeysInBundle := len(bundle.Keys)
	if actualKeysInBundle != countAfterExtraction {
		t.Fatalf("Mismatch: GetRemainingKeyCount=%d but len(bundle.Keys)=%d",
			countAfterExtraction, actualKeysInBundle)
	}

	// Verify UsedCount is tracked correctly
	if bundle.UsedCount != numKeysToExtract {
		t.Fatalf("Expected UsedCount=%d, got %d", numKeysToExtract, bundle.UsedCount)
	}

	// Critical assertion: remaining count should equal slice length, not calculation from UsedCount
	if countAfterExtraction != actualKeysInBundle {
		t.Fatalf("GetRemainingKeyCount must return len(bundle.Keys), not PreKeysPerPeer-UsedCount")
	}
}

// TestNeedsRefreshAccuracy verifies that NeedsRefresh correctly triggers
// based on actual remaining keys, not a stale count calculation.
func TestNeedsRefreshAccuracy(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prekey-refresh-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	peerKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	// Generate initial bundle
	_, err = store.GeneratePreKeys(peerKey)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Should not need refresh initially
	if store.NeedsRefresh(peerKey) {
		t.Fatal("Should not need refresh with full bundle")
	}

	// Extract keys until we're just above the threshold
	// PreKeyRefreshThreshold = 20, with condition `availableKeys <= threshold`
	// So we need 21 keys to NOT trigger refresh, extract until we have 21 left
	keysToExtract := PreKeysPerPeer - (PreKeyRefreshThreshold + 1)
	for i := 0; i < keysToExtract; i++ {
		_, err := store.GetAvailablePreKey(peerKey)
		if err != nil {
			t.Fatalf("Failed to get pre-key %d: %v", i, err)
		}
	}

	// At exactly threshold + 1 (21 keys), should not need refresh yet
	if store.NeedsRefresh(peerKey) {
		remaining := store.GetRemainingKeyCount(peerKey)
		t.Fatalf("Should not need refresh with %d keys (threshold=%d)",
			remaining, PreKeyRefreshThreshold)
	}

	// Extract one more key to reach exactly the threshold (20 keys)
	_, err = store.GetAvailablePreKey(peerKey)
	if err != nil {
		t.Fatalf("Failed to extract key to threshold: %v", err)
	}

	// At exactly the threshold, should need refresh (condition is <=)
	if !store.NeedsRefresh(peerKey) {
		remaining := store.GetRemainingKeyCount(peerKey)
		t.Fatalf("Should need refresh with %d keys remaining (threshold=%d)",
			remaining, PreKeyRefreshThreshold)
	}

	// Verify the remaining count is what we expect (exactly at threshold)
	remaining := store.GetRemainingKeyCount(peerKey)
	expected := PreKeyRefreshThreshold
	if remaining != expected {
		t.Fatalf("Expected %d remaining keys, got %d", expected, remaining)
	}
}
