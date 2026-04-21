package async

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWriteAheadLog(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal.Close()

	// Verify WAL file was created
	walPath := filepath.Join(dir, "async.wal")
	_, err = os.Stat(walPath)
	assert.NoError(t, err)
}

func TestWALLogStoreMessage(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal.Close()

	msgID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	recipient := [32]byte{1, 2, 3, 4}
	data := []byte("test message data")

	seq, err := wal.LogStoreMessage(msgID, recipient, data)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), seq)

	// Log another entry
	seq2, err := wal.LogStoreMessage(msgID, recipient, data)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), seq2)
}

func TestWALLogDeleteMessage(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal.Close()

	msgID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	recipient := [32]byte{1, 2, 3, 4}

	seq, err := wal.LogDeleteMessage(msgID, recipient)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), seq)
}

func TestWALLogUpdateMessageState(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal.Close()

	msgID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	stateData := []byte(`{"old_state":1,"new_state":2}`)

	seq, err := wal.LogUpdateMessageState(msgID, stateData)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), seq)
}

func TestWALCommit(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal.Close()

	msgID := [16]byte{1, 2, 3, 4}
	recipient := [32]byte{1, 2, 3, 4}

	seq, err := wal.LogStoreMessage(msgID, recipient, []byte("data"))
	require.NoError(t, err)

	err = wal.Commit(seq)
	require.NoError(t, err)
}

func TestWALRecover(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	// Create WAL and add entries
	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)

	msgID1 := [16]byte{1}
	msgID2 := [16]byte{2}
	msgID3 := [16]byte{3}
	recipient := [32]byte{1, 2, 3, 4}

	seq1, err := wal.LogStoreMessage(msgID1, recipient, []byte("msg1"))
	require.NoError(t, err)

	_, err = wal.LogStoreMessage(msgID2, recipient, []byte("msg2"))
	require.NoError(t, err)

	seq3, err := wal.LogStoreMessage(msgID3, recipient, []byte("msg3"))
	require.NoError(t, err)

	// Commit only the first entry
	err = wal.Commit(seq1)
	require.NoError(t, err)

	wal.Close()

	// Reopen and recover
	wal2, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal2.Close()

	uncommitted, err := wal2.Recover()
	require.NoError(t, err)

	// Should have 2 uncommitted entries (seq2 and seq3)
	assert.Len(t, uncommitted, 2)

	// Verify sequence continues from where it left off
	assert.Equal(t, seq3, wal2.Sequence())
}

func TestWALCheckpoint(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal.Close()

	// Add some entries
	msgID := [16]byte{1}
	recipient := [32]byte{1, 2, 3, 4}

	for i := 0; i < 5; i++ {
		_, err := wal.LogStoreMessage(msgID, recipient, []byte("data"))
		require.NoError(t, err)
	}

	err = wal.Checkpoint()
	require.NoError(t, err)
}

func TestWALTruncate(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal.Close()

	msgID := [16]byte{1}
	recipient := [32]byte{1, 2, 3, 4}

	// Add entries
	_, err = wal.LogStoreMessage(msgID, recipient, []byte("data"))
	require.NoError(t, err)

	size, err := wal.Size()
	require.NoError(t, err)
	assert.Greater(t, size, int64(0))

	// Truncate
	err = wal.Truncate()
	require.NoError(t, err)

	// Verify size is 0
	size, err = wal.Size()
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)

	// Verify sequence reset
	assert.Equal(t, uint64(0), wal.Sequence())
}

func TestWALConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal.Close()

	const numGoroutines = 10
	const entriesPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			msgID := [16]byte{byte(id)}
			recipient := [32]byte{byte(id)}
			for j := 0; j < entriesPerGoroutine; j++ {
				_, err := wal.LogStoreMessage(msgID, recipient, []byte("data"))
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all entries were written
	assert.Equal(t, uint64(numGoroutines*entriesPerGoroutine), wal.Sequence())
}

func TestWALClosedOperations(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)

	err = wal.Close()
	require.NoError(t, err)

	// All operations should fail on closed WAL
	msgID := [16]byte{1}
	recipient := [32]byte{1}

	_, err = wal.LogStoreMessage(msgID, recipient, []byte("data"))
	assert.Error(t, err)

	_, err = wal.LogDeleteMessage(msgID, recipient)
	assert.Error(t, err)

	_, err = wal.LogUpdateMessageState(msgID, []byte("state"))
	assert.Error(t, err)

	err = wal.Commit(1)
	assert.Error(t, err)

	err = wal.Checkpoint()
	assert.Error(t, err)

	_, err = wal.Recover()
	assert.Error(t, err)

	err = wal.Truncate()
	assert.Error(t, err)

	_, err = wal.Size()
	assert.Error(t, err)
}

func TestWALAutoCheckpoint(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir
	config.MaxEntriesBeforeCheckpoint = 5 // Low threshold for testing

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal.Close()

	msgID := [16]byte{1}
	recipient := [32]byte{1, 2, 3, 4}

	// Write enough entries to trigger auto-checkpoint
	for i := 0; i < 10; i++ {
		_, err := wal.LogStoreMessage(msgID, recipient, []byte("data"))
		require.NoError(t, err)
	}

	// Give checkpoint goroutine time to run
	time.Sleep(100 * time.Millisecond)

	// Verify WAL is still functional
	seq, err := wal.LogStoreMessage(msgID, recipient, []byte("more data"))
	require.NoError(t, err)
	assert.Equal(t, uint64(11), seq)
}

func TestWALCloseWaitsForCheckpointGoroutines(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir
	config.MaxEntriesBeforeCheckpoint = 1

	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)

	msgID := [16]byte{1}
	recipient := [32]byte{1, 2, 3, 4}

	for i := 0; i < 20; i++ {
		_, err := wal.LogStoreMessage(msgID, recipient, []byte("checkpoint"))
		require.NoError(t, err)
	}

	err = wal.Close()
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		wal.checkpointWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("checkpoint goroutines did not finish before Close returned")
	}
}

func TestWALRecoveryAfterCheckpoint(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	// Create WAL, add entries, checkpoint, add more entries
	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)

	msgID1 := [16]byte{1}
	msgID2 := [16]byte{2}
	recipient := [32]byte{1, 2, 3, 4}

	// Pre-checkpoint entries
	_, err = wal.LogStoreMessage(msgID1, recipient, []byte("before checkpoint"))
	require.NoError(t, err)

	err = wal.Checkpoint()
	require.NoError(t, err)

	// Post-checkpoint entries (uncommitted)
	_, err = wal.LogStoreMessage(msgID2, recipient, []byte("after checkpoint"))
	require.NoError(t, err)

	wal.Close()

	// Reopen and recover
	wal2, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal2.Close()

	uncommitted, err := wal2.Recover()
	require.NoError(t, err)

	// Only post-checkpoint entry should be uncommitted
	assert.Len(t, uncommitted, 1)
	assert.Equal(t, msgID2, uncommitted[0].MessageID)
}

func TestWALSequencePreservedAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir

	// First session
	wal, err := NewWriteAheadLog(config)
	require.NoError(t, err)

	msgID := [16]byte{1}
	recipient := [32]byte{1, 2, 3, 4}

	for i := 0; i < 10; i++ {
		_, err := wal.LogStoreMessage(msgID, recipient, []byte("data"))
		require.NoError(t, err)
	}
	wal.Close()

	// Second session
	wal2, err := NewWriteAheadLog(config)
	require.NoError(t, err)
	defer wal2.Close()

	assert.Equal(t, uint64(10), wal2.Sequence())

	// New entries should continue from 11
	seq, err := wal2.LogStoreMessage(msgID, recipient, []byte("new data"))
	require.NoError(t, err)
	assert.Equal(t, uint64(11), seq)
}

func TestMessageStorageWALIntegration(t *testing.T) {
	dir := t.TempDir()

	storageKeyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	storage := NewMessageStorage(storageKeyPair, dir)

	// Enable WAL
	err = storage.EnableWAL()
	require.NoError(t, err)
	assert.True(t, storage.IsWALEnabled())

	// Disable WAL
	err = storage.DisableWAL()
	require.NoError(t, err)
	assert.False(t, storage.IsWALEnabled())

	// Enable again
	err = storage.EnableWAL()
	require.NoError(t, err)
	assert.True(t, storage.IsWALEnabled())

	// Close
	err = storage.Close()
	require.NoError(t, err)
	assert.False(t, storage.IsWALEnabled())
}

func TestMessageStorageWALWithConfig(t *testing.T) {
	dir := t.TempDir()

	storageKeyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	storage := NewMessageStorage(storageKeyPair, dir)

	config := DefaultWALConfig()
	config.Directory = dir
	config.SyncOnWrite = false

	err = storage.EnableWALWithConfig(config)
	require.NoError(t, err)
	assert.True(t, storage.IsWALEnabled())

	err = storage.Close()
	require.NoError(t, err)
}

func TestMessageStorageWALCheckpoint(t *testing.T) {
	dir := t.TempDir()

	storageKeyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	storage := NewMessageStorage(storageKeyPair, dir)

	err = storage.EnableWAL()
	require.NoError(t, err)
	defer storage.Close()

	// Checkpoint should succeed
	err = storage.WALCheckpoint()
	require.NoError(t, err)
}

func TestMessageStorageRecoverFromWAL(t *testing.T) {
	dir := t.TempDir()

	storageKeyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	// WAL is now auto-enabled when dataDir is provided
	storage := NewMessageStorage(storageKeyPair, dir)
	defer storage.Close()

	// Recovery with empty WAL should succeed
	recovered, err := storage.RecoverFromWAL()
	require.NoError(t, err)
	assert.Equal(t, 0, recovered)

	// Test recovery without WAL using empty dataDir
	storageNoWAL := NewMessageStorage(storageKeyPair, "")
	_, err = storageNoWAL.RecoverFromWAL()
	assert.Error(t, err)
}

func TestDefaultWALConfig(t *testing.T) {
	config := DefaultWALConfig()

	assert.Equal(t, int64(64*1024*1024), config.MaxFileSize)
	assert.Equal(t, 5*time.Minute, config.CheckpointInterval)
	assert.Equal(t, 1000, config.MaxEntriesBeforeCheckpoint)
	assert.True(t, config.SyncOnWrite)
}

func BenchmarkWALLogStoreMessage(b *testing.B) {
	dir := b.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir
	config.SyncOnWrite = false // Disable sync for benchmark

	wal, err := NewWriteAheadLog(config)
	require.NoError(b, err)
	defer wal.Close()

	msgID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	recipient := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	data := make([]byte, 256)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := wal.LogStoreMessage(msgID, recipient, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWALLogStoreMessageWithSync(b *testing.B) {
	dir := b.TempDir()
	config := DefaultWALConfig()
	config.Directory = dir
	config.SyncOnWrite = true

	wal, err := NewWriteAheadLog(config)
	require.NoError(b, err)
	defer wal.Close()

	msgID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	recipient := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	data := make([]byte, 256)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := wal.LogStoreMessage(msgID, recipient, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestMessageStoragePersistence validates that messages persist across storage restarts.
// This tests the WAL integration with MessageStorage to ensure crash recovery works.
func TestMessageStoragePersistence(t *testing.T) {
	dir := t.TempDir()

	// Generate test key pair for storage
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "Failed to generate key pair")

	// Create storage and enable WAL
	storage := NewMessageStorage(keyPair, dir)
	require.NotNil(t, storage)

	err = storage.EnableWAL()
	require.NoError(t, err, "Failed to enable WAL")
	assert.True(t, storage.IsWALEnabled(), "WAL should be enabled")

	// Store test messages
	recipientPK := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}
	senderPK := keyPair.Public
	nonce := [24]byte{}
	testMessage := []byte("test message content for persistence verification")

	// Store first message
	msgID1, err := storage.StoreMessage(recipientPK, senderPK, testMessage, nonce, MessageTypeNormal)
	require.NoError(t, err, "Failed to store message 1")

	// Store second message
	testMessage2 := []byte("second test message for recovery")
	msgID2, err := storage.StoreMessage(recipientPK, senderPK, testMessage2, nonce, MessageTypeAction)
	require.NoError(t, err, "Failed to store message 2")

	// Verify messages are in storage
	messages, err := storage.RetrieveMessages(recipientPK)
	require.NoError(t, err, "Failed to retrieve messages")
	assert.Len(t, messages, 2, "Should have 2 messages before restart")

	// Close storage (simulates crash/restart)
	storage.Close()

	// Create new storage instance with same directory (simulates restart)
	storage2 := NewMessageStorage(keyPair, dir)
	require.NotNil(t, storage2)

	// Enable WAL and recover
	err = storage2.EnableWAL()
	require.NoError(t, err, "Failed to enable WAL on restarted storage")

	recovered, err := storage2.RecoverFromWAL()
	require.NoError(t, err, "Failed to recover from WAL")
	t.Logf("Recovered %d messages from WAL", recovered)

	// Verify messages were recovered
	recoveredMessages, err := storage2.RetrieveMessages(recipientPK)
	require.NoError(t, err, "Failed to retrieve recovered messages")
	assert.Len(t, recoveredMessages, 2, "Should have recovered 2 messages")

	// Verify message content
	foundMsg1 := false
	foundMsg2 := false
	for _, msg := range recoveredMessages {
		if msg.ID == msgID1 {
			foundMsg1 = true
			assert.Equal(t, testMessage, msg.EncryptedData, "Message 1 content mismatch")
			assert.Equal(t, MessageTypeNormal, msg.MessageType, "Message 1 type mismatch")
		}
		if msg.ID == msgID2 {
			foundMsg2 = true
			assert.Equal(t, testMessage2, msg.EncryptedData, "Message 2 content mismatch")
			assert.Equal(t, MessageTypeAction, msg.MessageType, "Message 2 type mismatch")
		}
	}
	assert.True(t, foundMsg1, "Message 1 should be recovered")
	assert.True(t, foundMsg2, "Message 2 should be recovered")

	// Cleanup
	storage2.Close()
}

// TestMessageDeletionPersistence validates that deleted messages stay deleted after restart.
func TestMessageDeletionPersistence(t *testing.T) {
	dir := t.TempDir()

	// Generate test key pair for storage
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "Failed to generate key pair")

	// Create storage and enable WAL
	storage := NewMessageStorage(keyPair, dir)
	require.NotNil(t, storage)

	err = storage.EnableWAL()
	require.NoError(t, err)

	// Store and then delete a message
	recipientPK := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}
	senderPK := keyPair.Public
	nonce := [24]byte{}
	testMessage := []byte("message to be deleted")

	msgID, err := storage.StoreMessage(recipientPK, senderPK, testMessage, nonce, MessageTypeNormal)
	require.NoError(t, err)

	// Delete the message
	err = storage.DeleteMessage(msgID, recipientPK)
	require.NoError(t, err)

	// Verify message is gone (RetrieveMessages returns ErrMessageNotFound when empty)
	messages, err := storage.RetrieveMessages(recipientPK)
	if err != nil {
		assert.ErrorIs(t, err, ErrMessageNotFound, "Expected no messages error")
	} else {
		assert.Len(t, messages, 0, "Should have 0 messages after deletion")
	}

	// Close storage (simulates restart)
	storage.Close()

	// Create new storage and recover
	storage2 := NewMessageStorage(keyPair, dir)
	err = storage2.EnableWAL()
	require.NoError(t, err)

	_, err = storage2.RecoverFromWAL()
	require.NoError(t, err)

	// Verify message is still gone after recovery (RetrieveMessages returns ErrMessageNotFound when empty)
	recoveredMessages, err := storage2.RetrieveMessages(recipientPK)
	if err != nil {
		assert.ErrorIs(t, err, ErrMessageNotFound, "Expected no messages error after recovery")
	} else {
		assert.Len(t, recoveredMessages, 0, "Deleted message should not be recovered")
	}

	// Cleanup
	storage2.Close()
}
