package crypto

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNonceStoreCreation(t *testing.T) {
	tempDir := t.TempDir()

	ns, err := NewNonceStore(tempDir)
	require.NoError(t, err)
	require.NotNil(t, ns)
	defer ns.Close()

	// Verify data directory was created
	assert.DirExists(t, tempDir)

	// Verify initial state
	assert.Equal(t, 0, ns.Size())
}

func TestNonceStoreCheckAndStore(t *testing.T) {
	tempDir := t.TempDir()
	ns, err := NewNonceStore(tempDir)
	require.NoError(t, err)
	defer ns.Close()

	nonce := [32]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	timestamp := time.Now().Unix()

	// First use should succeed
	result := ns.CheckAndStore(nonce, timestamp)
	assert.True(t, result, "First nonce use should succeed")
	assert.Equal(t, 1, ns.Size())

	// Replay should be detected
	result = ns.CheckAndStore(nonce, timestamp)
	assert.False(t, result, "Replay should be detected")
	assert.Equal(t, 1, ns.Size(), "Size should not increase on replay")
}

func TestNonceStorePersistence(t *testing.T) {
	tempDir := t.TempDir()

	nonce1 := [32]byte{0x01}
	nonce2 := [32]byte{0x02}
	timestamp := time.Now().Unix()

	// Create first store and add nonces
	{
		ns, err := NewNonceStore(tempDir)
		require.NoError(t, err)

		assert.True(t, ns.CheckAndStore(nonce1, timestamp))
		assert.True(t, ns.CheckAndStore(nonce2, timestamp))

		// Close will save the state synchronously
		err = ns.Close()
		require.NoError(t, err)
	}

	// Verify file was created
	saveFile := filepath.Join(tempDir, "handshake_nonces.dat")
	assert.FileExists(t, saveFile)

	// Create second store and verify persistence
	{
		ns, err := NewNonceStore(tempDir)
		require.NoError(t, err)
		defer ns.Close()

		// Both nonces should be detected as replays
		assert.False(t, ns.CheckAndStore(nonce1, timestamp), "nonce1 should be loaded from disk")
		assert.False(t, ns.CheckAndStore(nonce2, timestamp), "nonce2 should be loaded from disk")

		// New nonce should work
		nonce3 := [32]byte{0x03}
		assert.True(t, ns.CheckAndStore(nonce3, timestamp))
	}
}

func TestNonceStoreExpiration(t *testing.T) {
	tempDir := t.TempDir()
	ns, err := NewNonceStore(tempDir)
	require.NoError(t, err)
	defer ns.Close()

	// Add nonce with old timestamp (expired)
	oldNonce := [32]byte{0x01}
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	ns.CheckAndStore(oldNonce, oldTimestamp)

	// Add nonce with current timestamp (not expired)
	currentNonce := [32]byte{0x02}
	currentTimestamp := time.Now().Unix()
	ns.CheckAndStore(currentNonce, currentTimestamp)

	assert.Equal(t, 2, ns.Size(), "Both nonces should be stored initially")

	// Run cleanup
	ns.cleanup()

	// Old nonce should be removed, current nonce should remain
	assert.Equal(t, 1, ns.Size(), "Expired nonce should be removed")

	// Old nonce should now be accepted (was removed)
	assert.True(t, ns.CheckAndStore(oldNonce, time.Now().Unix()))

	// Current nonce should still be detected
	assert.False(t, ns.CheckAndStore(currentNonce, currentTimestamp))
}

func TestNonceStoreLoadCorruptedFile(t *testing.T) {
	tempDir := t.TempDir()
	saveFile := filepath.Join(tempDir, "handshake_nonces.dat")

	// Create corrupted file (too small)
	err := os.WriteFile(saveFile, []byte{0x01, 0x02}, 0o600)
	require.NoError(t, err)

	// Should handle corrupted file gracefully
	ns, err := NewNonceStore(tempDir)
	assert.NoError(t, err, "Should handle corrupted file gracefully")
	require.NotNil(t, ns)
	defer ns.Close()

	// Should start with empty state
	assert.Equal(t, 0, ns.Size())
}

func TestNonceStoreLoadExpiredNonces(t *testing.T) {
	tempDir := t.TempDir()

	// Create store with expired nonce
	{
		ns, err := NewNonceStore(tempDir)
		require.NoError(t, err)

		expiredNonce := [32]byte{0xFF}
		expiredTimestamp := time.Now().Add(-10 * time.Minute).Unix()
		ns.CheckAndStore(expiredNonce, expiredTimestamp)

		// Close will save the state synchronously
		ns.Close()
	}

	// Load store - expired nonce should not be loaded
	{
		ns, err := NewNonceStore(tempDir)
		require.NoError(t, err)
		defer ns.Close()

		assert.Equal(t, 0, ns.Size(), "Expired nonces should not be loaded")

		// Expired nonce should be accepted as new
		expiredNonce := [32]byte{0xFF}
		assert.True(t, ns.CheckAndStore(expiredNonce, time.Now().Unix()))
	}
}

func TestNonceStoreConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	ns, err := NewNonceStore(tempDir)
	require.NoError(t, err)
	defer ns.Close()

	// Test concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(index int) {
			nonce := [32]byte{byte(index)}
			timestamp := time.Now().Unix()
			ns.CheckAndStore(nonce, timestamp)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// All 10 nonces should be stored
	assert.Equal(t, 10, ns.Size())
}

func TestNonceStoreMultipleNonces(t *testing.T) {
	tempDir := t.TempDir()
	ns, err := NewNonceStore(tempDir)
	require.NoError(t, err)
	defer ns.Close()

	timestamp := time.Now().Unix()
	count := 100

	// Add multiple nonces
	for i := 0; i < count; i++ {
		nonce := [32]byte{byte(i)}
		assert.True(t, ns.CheckAndStore(nonce, timestamp))
	}

	assert.Equal(t, count, ns.Size())

	// Verify all are detected as replays
	for i := 0; i < count; i++ {
		nonce := [32]byte{byte(i)}
		assert.False(t, ns.CheckAndStore(nonce, timestamp))
	}
}

func TestNonceStoreReplayProtection(t *testing.T) {
	tempDir := t.TempDir()
	ns, err := NewNonceStore(tempDir)
	require.NoError(t, err)
	defer ns.Close()

	nonce := [32]byte{0xAA, 0xBB, 0xCC, 0xDD}
	timestamp := time.Now().Unix()

	// Scenario 1: First use
	assert.True(t, ns.CheckAndStore(nonce, timestamp), "First use should succeed")

	// Scenario 2: Immediate replay
	assert.False(t, ns.CheckAndStore(nonce, timestamp), "Immediate replay should be detected")

	// Scenario 3: Replay with different timestamp
	newTimestamp := timestamp + 60
	assert.False(t, ns.CheckAndStore(nonce, newTimestamp), "Replay with different timestamp should be detected")
}

func TestNonceStoreCleanupLoop(t *testing.T) {
	tempDir := t.TempDir()
	ns, err := NewNonceStore(tempDir)
	require.NoError(t, err)
	defer ns.Close()

	// Add expired and current nonces
	expiredNonce := [32]byte{0x01}
	expiredTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	ns.CheckAndStore(expiredNonce, expiredTimestamp)

	currentNonce := [32]byte{0x02}
	currentTimestamp := time.Now().Unix()
	ns.CheckAndStore(currentNonce, currentTimestamp)

	assert.Equal(t, 2, ns.Size())

	// Trigger cleanup manually (cleanup loop runs in background)
	ns.cleanup()

	// Only current nonce should remain
	assert.Equal(t, 1, ns.Size())
}

func TestNonceStoreAtomicSave(t *testing.T) {
	tempDir := t.TempDir()
	ns, err := NewNonceStore(tempDir)
	require.NoError(t, err)
	defer ns.Close()

	// Add nonces
	for i := 0; i < 10; i++ {
		nonce := [32]byte{byte(i)}
		ns.CheckAndStore(nonce, time.Now().Unix())
	}

	// Force save
	err = ns.save()
	require.NoError(t, err)

	// Verify no temporary files remain
	tmpFile := ns.saveFile + ".tmp"
	_, err = os.Stat(tmpFile)
	assert.True(t, os.IsNotExist(err), "Temporary file should not exist after save")

	// Verify actual file exists
	_, err = os.Stat(ns.saveFile)
	assert.NoError(t, err, "Save file should exist")
}

func TestNonceStoreWithTimeProvider(t *testing.T) {
	tempDir := t.TempDir()

	// Create a fixed time for deterministic testing
	fixedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	mock := &MockTimeProvider{currentTime: fixedTime}

	ns, err := NewNonceStoreWithTimeProvider(tempDir, mock)
	require.NoError(t, err)
	defer ns.Close()

	// Add nonce with current timestamp (expiry = timestamp + 6 minutes = fixedTime + 6 minutes)
	nonce1 := [32]byte{0x01}
	ns.CheckAndStore(nonce1, fixedTime.Unix())

	// Add nonce with old timestamp (expiry = oldTimestamp + 6 minutes = fixedTime - 4 minutes)
	nonce2 := [32]byte{0x02}
	oldTimestamp := fixedTime.Add(-10 * time.Minute).Unix()
	ns.CheckAndStore(nonce2, oldTimestamp)

	assert.Equal(t, 2, ns.Size(), "Both nonces should be stored")

	// Run cleanup - should remove the nonce with expired time (fixedTime - 4 minutes < fixedTime)
	ns.cleanup()

	// Only nonce1 should remain (expiry = fixedTime + 6 minutes > fixedTime)
	assert.Equal(t, 1, ns.Size(), "Only non-expired nonce should remain after cleanup")

	// Advance mock time past the first nonce's expiry (fixedTime + 6 minutes)
	mock.Advance(7 * time.Minute)
	ns.cleanup()

	// Now both should be cleaned up
	assert.Equal(t, 0, ns.Size(), "All nonces should be cleaned up")
}

func TestNonceStoreSetTimeProvider(t *testing.T) {
	tempDir := t.TempDir()
	ns, err := NewNonceStore(tempDir)
	require.NoError(t, err)
	defer ns.Close()

	// Set a mock time provider
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	mock := &MockTimeProvider{currentTime: fixedTime}
	ns.SetTimeProvider(mock)

	// Add nonce with old timestamp (expiry = oldTimestamp + 6 min = fixedTime - 4 min < fixedTime)
	nonce := [32]byte{0xAA}
	oldTimestamp := fixedTime.Add(-10 * time.Minute).Unix()
	ns.CheckAndStore(nonce, oldTimestamp)

	// Run cleanup - should remove expired nonce
	ns.cleanup()
	assert.Equal(t, 0, ns.Size(), "Expired nonce should be removed")

	// Test setting nil (should use default)
	ns.SetTimeProvider(nil)
	// Should still work without panic
	ns.cleanup()
}
