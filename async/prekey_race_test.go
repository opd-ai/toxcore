package async

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// TestPreKeyRefreshRaceCondition verifies that RefreshPreKeys is atomic
// and doesn't allow concurrent access to inconsistent state
func TestPreKeyRefreshRaceCondition(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "prekey_race_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test key pair
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create pre-key store
	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	peerPK := [32]byte{0x01, 0x02, 0x03, 0x04}

	// Generate initial pre-keys
	_, err = store.GeneratePreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Test concurrent access during refresh
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Start multiple goroutines that will try to access the bundle
	// while RefreshPreKeys is running
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				// Use GetRemainingKeyCount which is a safe atomic operation
				remaining := store.GetRemainingKeyCount(peerPK)
				// Valid counts are 0-100, anything else indicates corruption
				if remaining < 0 || remaining > PreKeysPerPeer {
					errors <- fmt.Errorf("invalid remaining count: %d", remaining)
					return
				}
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Start goroutines that refresh pre-keys
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, err := store.RefreshPreKeys(peerPK)
				if err != nil {
					errors <- err
					return
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Start goroutines that try to get available pre-keys
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := store.GetAvailablePreKey(peerPK)
				if err != nil {
					// It's okay if we run out of keys temporarily
					if err.Error() != "no pre-key bundle found for peer 01020304" {
						errors <- err
						return
					}
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Final verification - bundle should be valid
	bundle, err := store.GetBundle(peerPK)
	if err != nil {
		t.Fatalf("Failed to get final bundle: %v", err)
	}

	if len(bundle.Keys) == 0 {
		t.Error("Final bundle should have keys")
	}
}

// TestPreKeyRefreshAtomicity verifies that the bundle is never in an
// inconsistent state during refresh
func TestPreKeyRefreshAtomicity(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "prekey_atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test key pair
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create pre-key store
	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	peerPK := [32]byte{0x05, 0x06, 0x07, 0x08}

	// Generate initial pre-keys
	initialBundle, err := store.GeneratePreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Use most keys to trigger refresh threshold
	for i := 0; i < PreKeysPerPeer-PreKeyRefreshThreshold; i++ {
		_, err = store.GetAvailablePreKey(peerPK)
		if err != nil {
			t.Fatalf("Failed to get pre-key %d: %v", i, err)
		}
	}

	// Track bundle states seen during concurrent operations
	type bundleState struct {
		keyCount  int
		usedCount int
		createdAt time.Time
	}
	statesSeen := make(chan bundleState, 1000)

	var wg sync.WaitGroup

	// Monitor bundle state continuously
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			bundle, err := store.GetBundle(peerPK)
			if err == nil && bundle != nil {
				statesSeen <- bundleState{
					keyCount:  len(bundle.Keys),
					usedCount: bundle.UsedCount,
					createdAt: bundle.CreatedAt,
				}
			}
			time.Sleep(100 * time.Microsecond)
		}
	}()

	// Perform refresh in parallel
	time.Sleep(time.Millisecond) // Let monitoring start
	newBundle, err := store.RefreshPreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to refresh pre-keys: %v", err)
	}

	wg.Wait()
	close(statesSeen)

	// Verify all seen states are valid
	validStateCount := 0
	for state := range statesSeen {
		// State should be either the old bundle or the new bundle
		// Never a partially-initialized state
		isOldBundle := state.createdAt.Equal(initialBundle.CreatedAt)
		isNewBundle := state.createdAt.Equal(newBundle.CreatedAt)

		if !isOldBundle && !isNewBundle {
			t.Errorf("Invalid intermediate state seen: keyCount=%d, usedCount=%d",
				state.keyCount, state.usedCount)
		}

		// Old bundle should have used keys, new bundle should have full fresh keys
		if isOldBundle && state.keyCount == 0 {
			t.Error("Old bundle seen with no keys (inconsistent state)")
		}
		if isNewBundle && state.keyCount != PreKeysPerPeer {
			t.Errorf("New bundle should have %d keys, saw %d", PreKeysPerPeer, state.keyCount)
		}
		if isNewBundle && state.usedCount != 0 {
			t.Errorf("New bundle should have 0 used keys, saw %d", state.usedCount)
		}

		validStateCount++
	}

	if validStateCount == 0 {
		t.Error("No bundle states were monitored")
	}
}

// TestPreKeyRefreshConcurrentReads verifies that reads during refresh
// never see nil or partially initialized bundles
func TestPreKeyRefreshConcurrentReads(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "prekey_concurrent_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test key pair
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create pre-key store
	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	peerPK := [32]byte{0x0A, 0x0B, 0x0C, 0x0D}

	// Generate initial pre-keys
	_, err = store.GeneratePreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	var wg sync.WaitGroup
	successfulReads := make(chan int, 100)

	// Start multiple readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			reads := 0
			for j := 0; j < 50; j++ {
				bundle, err := store.GetBundle(peerPK)
				if err == nil {
					// Verify bundle is never in invalid state
					if bundle == nil {
						t.Errorf("Reader %d: Got nil bundle without error", id)
						return
					}
					if len(bundle.Keys) == 0 {
						t.Errorf("Reader %d: Got bundle with zero keys", id)
						return
					}
					reads++
				}
				time.Sleep(10 * time.Microsecond)
			}
			successfulReads <- reads
		}(i)
	}

	// Perform multiple refreshes while readers are active
	time.Sleep(100 * time.Microsecond)
	for i := 0; i < 10; i++ {
		_, err := store.RefreshPreKeys(peerPK)
		if err != nil {
			t.Errorf("Refresh %d failed: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	wg.Wait()
	close(successfulReads)

	// Verify readers were successful
	totalReads := 0
	for reads := range successfulReads {
		totalReads += reads
	}

	if totalReads == 0 {
		t.Error("No successful reads occurred during refresh operations")
	}

	t.Logf("Completed %d successful concurrent reads during 10 refresh operations", totalReads)
}
