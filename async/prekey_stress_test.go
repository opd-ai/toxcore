package async

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/require"
)

// isPreKeyExhaustionError returns true when err indicates the expected
// exhaustion or missing-bundle condition from GetAvailablePreKey.
func isPreKeyExhaustionError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no available pre-keys") ||
		strings.Contains(msg, "no pre-key bundle found")
}

// TestPreKeyStoreStressConcurrentConsumption exercises the PreKeyStore under
// high concurrency, where many goroutines simultaneously consume pre-keys for
// the same peer.  It verifies:
//   - No data races (run with -race).
//   - Each consumed pre-key is only returned once.
//   - Total unique keys consumed never exceeds PreKeysPerPeer.
func TestPreKeyStoreStressConcurrentConsumption(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	store, err := NewPreKeyStore(keyPair, tmpDir)
	require.NoError(t, err)

	peerPK := [32]byte{0xab, 0xcd}
	_, err = store.GeneratePreKeys(peerPK)
	require.NoError(t, err)

	const workers = 20
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		consumed = make(map[uint32]struct{}, PreKeysPerPeer)
		dupCount atomic.Int64
	)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for {
				pk, err := store.GetAvailablePreKey(peerPK)
				if err != nil {
					if !isPreKeyExhaustionError(err) {
						t.Errorf("GetAvailablePreKey unexpected error: %v", err)
					}
					return
				}
				mu.Lock()
				if _, dup := consumed[pk.ID]; dup {
					dupCount.Add(1)
				}
				consumed[pk.ID] = struct{}{}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	require.Zero(t, dupCount.Load(), "duplicate pre-key IDs returned under concurrent access")

	consumedCount := len(consumed)
	require.LessOrEqual(t, consumedCount, PreKeysPerPeer,
		"consumed more keys than were generated")
}

// TestPreKeyStoreStressConcurrentPeers exercises the PreKeyStore with many
// goroutines consuming pre-keys for distinct peers simultaneously.
func TestPreKeyStoreStressConcurrentPeers(t *testing.T) {
	t.Parallel()

	const peerCount = 10
	tmpDir := t.TempDir()
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	store, err := NewPreKeyStore(keyPair, tmpDir)
	require.NoError(t, err)

	// Pre-generate a bundle for each peer.
	peers := make([][32]byte, peerCount)
	for i := 0; i < peerCount; i++ {
		var pk [32]byte
		pk[0] = byte(i + 1)
		peers[i] = pk
		_, err := store.GeneratePreKeys(pk)
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	var totalConsumed atomic.Int64

	for _, peer := range peers {
		peer := peer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, err := store.GetAvailablePreKey(peer)
				if err != nil {
					if !isPreKeyExhaustionError(err) {
						t.Errorf("GetAvailablePreKey unexpected error: %v", err)
					}
					return
				}
				totalConsumed.Add(1)
			}
		}()
	}

	wg.Wait()

	require.LessOrEqual(t, int(totalConsumed.Load()), peerCount*PreKeysPerPeer,
		"consumed more keys than were generated across all peers")
}

// TestPreKeyStoreStressMixedOperations exercises concurrent reads and writes
// (GeneratePreKeys / GetAvailablePreKey / NeedsRefresh) on the same store.
func TestPreKeyStoreStressMixedOperations(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	store, err := NewPreKeyStore(keyPair, tmpDir)
	require.NoError(t, err)

	peerPK := [32]byte{0xde, 0xad}
	_, err = store.GeneratePreKeys(peerPK)
	require.NoError(t, err)

	const (
		consumers = 5
		checkers  = 5
		iters     = 50
	)

	var wg sync.WaitGroup

	// Goroutines that consume keys.
	for i := 0; i < consumers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				_, _ = store.GetAvailablePreKey(peerPK)
			}
		}()
	}

	// Goroutines that read status without modifying state.
	for i := 0; i < checkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				_ = store.NeedsRefresh(peerPK)
				_ = store.GetRemainingKeyCount(peerPK)
			}
		}()
	}

	wg.Wait()
}

// TestPreKeyStoreStressRefreshUnderLoad verifies that RefreshPreKeys can be
// called concurrently without corrupting the key store.
func TestPreKeyStoreStressRefreshUnderLoad(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Use os.MkdirTemp to get a unique path that is cleaned up automatically.
	subDir, err := os.MkdirTemp(tmpDir, "refresh_stress")
	require.NoError(t, err)

	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	store, err := NewPreKeyStore(keyPair, subDir)
	require.NoError(t, err)

	peerPK := [32]byte{0xbe, 0xef}
	_, err = store.GeneratePreKeys(peerPK)
	require.NoError(t, err)

	var wg sync.WaitGroup
	var refreshErrors atomic.Int64

	// Drain all keys so that refresh is needed.
	for {
		_, err := store.GetAvailablePreKey(peerPK)
		if err != nil {
			break
		}
	}

	// Race concurrent refresh calls.
	const refreshers = 5
	for i := 0; i < refreshers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.RefreshPreKeys(peerPK); err != nil {
				refreshErrors.Add(1)
			}
		}()
	}

	wg.Wait()

	require.Zero(t, refreshErrors.Load(), "RefreshPreKeys returned unexpected errors under concurrent access")

	// After concurrent refreshes the store should have a valid bundle.
	count := store.GetRemainingKeyCount(peerPK)
	require.Greater(t, count, 0, "expected at least some keys after concurrent refresh")
}
