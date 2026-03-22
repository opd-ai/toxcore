package async

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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
