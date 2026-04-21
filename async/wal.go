// Package async provides asynchronous messaging capabilities with durability.
// This file implements a Write-Ahead Log (WAL) for crash recovery of critical state.
package async

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// WALOperationType defines the type of operation logged in the WAL.
type WALOperationType uint8

const (
	// WALOpStoreMessage logs a message storage operation.
	WALOpStoreMessage WALOperationType = iota + 1
	// WALOpDeleteMessage logs a message deletion operation.
	WALOpDeleteMessage
	// WALOpUpdateMessageState logs a message state change.
	WALOpUpdateMessageState
	// WALOpCheckpoint marks a checkpoint (state snapshot taken).
	WALOpCheckpoint
)

// WALEntryStatus indicates the state of a WAL entry.
type WALEntryStatus uint8

const (
	// WALStatusPending indicates the operation has not completed.
	WALStatusPending WALEntryStatus = iota + 1
	// WALStatusCommitted indicates the operation completed successfully.
	WALStatusCommitted
	// WALStatusAborted indicates the operation was rolled back.
	WALStatusAborted
)

// WALEntry represents a single entry in the write-ahead log.
type WALEntry struct {
	Sequence  uint64           `json:"seq"`
	Timestamp int64            `json:"ts"`
	Operation WALOperationType `json:"op"`
	Status    WALEntryStatus   `json:"status"`
	MessageID [16]byte         `json:"msg_id,omitempty"`
	Recipient [32]byte         `json:"recipient,omitempty"`
	Data      []byte           `json:"data,omitempty"`
	Checksum  uint32           `json:"checksum"`
}

// WALConfig contains configuration for the write-ahead log.
type WALConfig struct {
	// Directory where WAL files are stored.
	Directory string
	// MaxFileSize is the maximum size of a single WAL file before rotation.
	MaxFileSize int64
	// CheckpointInterval is how often to create checkpoints.
	CheckpointInterval time.Duration
	// MaxEntriesBeforeCheckpoint triggers checkpoint after this many entries.
	MaxEntriesBeforeCheckpoint int
	// SyncOnWrite forces fsync after each write (slower but safer).
	SyncOnWrite bool
}

// DefaultWALConfig returns the default WAL configuration.
func DefaultWALConfig() WALConfig {
	return WALConfig{
		Directory:                  "",
		MaxFileSize:                64 * 1024 * 1024, // 64 MB
		CheckpointInterval:         5 * time.Minute,
		MaxEntriesBeforeCheckpoint: 1000,
		SyncOnWrite:                true,
	}
}

// WriteAheadLog provides durable logging for crash recovery.
type WriteAheadLog struct {
	mu             sync.Mutex
	checkpointWg   sync.WaitGroup
	config         WALConfig
	file           *os.File
	writer         *bufio.Writer
	sequence       uint64
	entriesCount   int
	lastCheckpoint time.Time
	closed         bool
	logger         *logrus.Entry
}

// NewWriteAheadLog creates a new WAL with the given configuration.
func NewWriteAheadLog(config WALConfig) (*WriteAheadLog, error) {
	if config.Directory == "" {
		config.Directory = os.TempDir()
	}

	if err := os.MkdirAll(config.Directory, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	wal := &WriteAheadLog{
		config:         config,
		lastCheckpoint: time.Now(),
		logger: logrus.WithFields(logrus.Fields{
			"component": "wal",
			"directory": config.Directory,
		}),
	}

	if err := wal.openOrCreateFile(); err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return wal, nil
}

func (w *WriteAheadLog) walFilePath() string {
	return filepath.Join(w.config.Directory, "async.wal")
}

func (w *WriteAheadLog) openOrCreateFile() error {
	path := w.walFilePath()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open WAL file %s: %w", path, err)
	}

	w.file = file
	w.writer = bufio.NewWriterSize(file, 64*1024) // 64KB buffer

	// Read to end to get sequence number
	if err := w.recoverSequence(); err != nil {
		w.file.Close()
		return fmt.Errorf("failed to recover WAL sequence: %w", err)
	}

	w.logger.WithField("sequence", w.sequence).Info("WAL initialized")
	return nil
}

func (w *WriteAheadLog) recoverSequence() error {
	info, err := w.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat WAL file: %w", err)
	}

	if info.Size() == 0 {
		w.sequence = 0
		return nil
	}

	maxSeq, err := w.scanEntriesForMaxSequence()
	if err != nil {
		return err
	}
	w.sequence = maxSeq

	// Seek back to end for appending
	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("failed to seek to end: %w", err)
	}

	return nil
}

// scanEntriesForMaxSequence reads all WAL entries and returns the maximum sequence number.
func (w *WriteAheadLog) scanEntriesForMaxSequence() (uint64, error) {
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek to start: %w", err)
	}

	reader := bufio.NewReader(w.file)
	var maxSeq uint64

	for {
		entry, err := w.readEntry(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			w.logger.WithError(err).Warn("Incomplete WAL entry (possibly from crash)")
			break
		}
		if entry.Sequence > maxSeq {
			maxSeq = entry.Sequence
		}
		w.entriesCount++
	}

	return maxSeq, nil
}

func (w *WriteAheadLog) readEntry(reader *bufio.Reader) (*WALEntry, error) {
	// Read length prefix (4 bytes)
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, lengthBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf)

	if length > 10*1024*1024 { // 10 MB sanity check
		return nil, errors.New("WAL entry too large")
	}

	// Read entry data
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}

	var entry WALEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal WAL entry: %w", err)
	}

	// Verify checksum
	expectedChecksum := entry.Checksum
	entry.Checksum = 0
	dataForChecksum, _ := json.Marshal(entry)
	actualChecksum := crc32.ChecksumIEEE(dataForChecksum)

	if actualChecksum != expectedChecksum {
		return nil, fmt.Errorf("WAL entry checksum mismatch: expected %d, got %d", expectedChecksum, actualChecksum)
	}
	entry.Checksum = expectedChecksum

	return &entry, nil
}

// LogStoreMessage logs a message storage operation.
func (w *WriteAheadLog) LogStoreMessage(msgID [16]byte, recipient [32]byte, data []byte) (uint64, error) {
	return w.logEntry(WALOpStoreMessage, msgID, recipient, data)
}

// LogDeleteMessage logs a message deletion operation.
func (w *WriteAheadLog) LogDeleteMessage(msgID [16]byte, recipient [32]byte) (uint64, error) {
	return w.logEntry(WALOpDeleteMessage, msgID, recipient, nil)
}

// LogUpdateMessageState logs a message state change.
func (w *WriteAheadLog) LogUpdateMessageState(msgID [16]byte, stateData []byte) (uint64, error) {
	return w.logEntry(WALOpUpdateMessageState, msgID, [32]byte{}, stateData)
}

func (w *WriteAheadLog) logEntry(op WALOperationType, msgID [16]byte, recipient [32]byte, data []byte) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, errors.New("WAL is closed")
	}

	w.sequence++
	entry := WALEntry{
		Sequence:  w.sequence,
		Timestamp: time.Now().UnixNano(),
		Operation: op,
		Status:    WALStatusPending,
		MessageID: msgID,
		Recipient: recipient,
		Data:      data,
	}

	// Calculate checksum (with Checksum field zeroed)
	dataForChecksum, err := json.Marshal(entry)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal entry for checksum: %w", err)
	}
	entry.Checksum = crc32.ChecksumIEEE(dataForChecksum)

	if err := w.writeEntry(&entry); err != nil {
		return 0, fmt.Errorf("failed to write WAL entry: %w", err)
	}

	w.entriesCount++

	// Check if checkpoint is needed
	if w.shouldCheckpoint() {
		w.checkpointWg.Add(1)
		go func() {
			defer w.checkpointWg.Done()
			if err := w.Checkpoint(); err != nil {
				w.logger.WithError(err).Warn("Failed to create checkpoint")
			}
		}()
	}

	return entry.Sequence, nil
}

func (w *WriteAheadLog) writeEntry(entry *WALEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal WAL entry: %w", err)
	}

	if err := w.writeLengthPrefixedData(data); err != nil {
		return err
	}

	return w.flushAndSync()
}

// writeLengthPrefixedData writes data with a 4-byte length prefix.
func (w *WriteAheadLog) writeLengthPrefixedData(data []byte) error {
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))

	if _, err := w.writer.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write length prefix: %w", err)
	}
	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write entry data: %w", err)
	}
	return nil
}

// flushAndSync flushes the buffer and optionally syncs to disk.
func (w *WriteAheadLog) flushAndSync() error {
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush WAL buffer: %w", err)
	}
	if w.config.SyncOnWrite {
		if err := w.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync WAL file: %w", err)
		}
	}
	return nil
}

// Commit marks an operation as successfully completed.
func (w *WriteAheadLog) Commit(sequence uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return errors.New("WAL is closed")
	}

	entry := WALEntry{
		Sequence:  sequence,
		Timestamp: time.Now().UnixNano(),
		Operation: WALOpCheckpoint, // Reuse for commit marker
		Status:    WALStatusCommitted,
	}

	dataForChecksum, _ := json.Marshal(entry)
	entry.Checksum = crc32.ChecksumIEEE(dataForChecksum)

	return w.writeEntry(&entry)
}

func (w *WriteAheadLog) shouldCheckpoint() bool {
	if w.entriesCount >= w.config.MaxEntriesBeforeCheckpoint {
		return true
	}
	if time.Since(w.lastCheckpoint) >= w.config.CheckpointInterval {
		return true
	}
	return false
}

// Checkpoint creates a checkpoint and truncates committed entries.
func (w *WriteAheadLog) Checkpoint() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return errors.New("WAL is closed")
	}

	entry := WALEntry{
		Sequence:  w.sequence,
		Timestamp: time.Now().UnixNano(),
		Operation: WALOpCheckpoint,
		Status:    WALStatusCommitted,
	}

	dataForChecksum, _ := json.Marshal(entry)
	entry.Checksum = crc32.ChecksumIEEE(dataForChecksum)

	if err := w.writeEntry(&entry); err != nil {
		return fmt.Errorf("failed to write checkpoint entry: %w", err)
	}

	w.lastCheckpoint = time.Now()
	w.entriesCount = 0

	w.logger.WithField("sequence", w.sequence).Info("WAL checkpoint created")
	return nil
}

// Recover reads the WAL and returns uncommitted entries for replay.
func (w *WriteAheadLog) Recover() ([]*WALEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, errors.New("WAL is closed")
	}

	entries, committed, lastCheckpointSeq, err := w.readAllWALEntries()
	if err != nil {
		return nil, err
	}

	uncommitted := w.filterUncommittedEntries(entries, committed, lastCheckpointSeq)

	if err := w.seekToEnd(); err != nil {
		return nil, err
	}

	w.logger.WithFields(logrus.Fields{
		"uncommitted_count":   len(uncommitted),
		"last_checkpoint_seq": lastCheckpointSeq,
	}).Info("WAL recovery complete")

	return uncommitted, nil
}

// readAllWALEntries reads all entries from the WAL file and categorizes them.
// Returns: pending entries map, committed sequences map, last checkpoint sequence, and any error.
func (w *WriteAheadLog) readAllWALEntries() (map[uint64]*WALEntry, map[uint64]bool, uint64, error) {
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to seek to start: %w", err)
	}

	reader := bufio.NewReader(w.file)
	entries := make(map[uint64]*WALEntry)
	committed := make(map[uint64]bool)
	var lastCheckpointSeq uint64

	for {
		entry, err := w.readEntry(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			w.logger.WithError(err).Warn("Stopping recovery at corrupted entry")
			break
		}

		w.categorizeWALEntry(entry, entries, committed, &lastCheckpointSeq)
	}

	return entries, committed, lastCheckpointSeq, nil
}

// categorizeWALEntry processes a single WAL entry and updates the tracking maps.
func (w *WriteAheadLog) categorizeWALEntry(entry *WALEntry, entries map[uint64]*WALEntry, committed map[uint64]bool, lastCheckpointSeq *uint64) {
	switch {
	case entry.Operation == WALOpCheckpoint && entry.Status == WALStatusCommitted:
		*lastCheckpointSeq = entry.Sequence
		committed[entry.Sequence] = true
	case entry.Status == WALStatusCommitted:
		committed[entry.Sequence] = true
	case entry.Status == WALStatusPending:
		entries[entry.Sequence] = entry
	}
}

// filterUncommittedEntries returns entries that are uncommitted and after the last checkpoint.
// Entries are sorted by sequence number to ensure correct replay order.
func (w *WriteAheadLog) filterUncommittedEntries(entries map[uint64]*WALEntry, committed map[uint64]bool, lastCheckpointSeq uint64) []*WALEntry {
	var uncommitted []*WALEntry
	for seq, entry := range entries {
		if !committed[seq] && seq > lastCheckpointSeq {
			uncommitted = append(uncommitted, entry)
		}
	}
	// Sort by sequence to ensure operations are replayed in the correct order
	sort.Slice(uncommitted, func(i, j int) bool {
		return uncommitted[i].Sequence < uncommitted[j].Sequence
	})
	return uncommitted
}

// seekToEnd positions the file pointer at the end for appending.
func (w *WriteAheadLog) seekToEnd() error {
	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("failed to seek to end: %w", err)
	}
	return nil
}

// Close closes the WAL file.
func (w *WriteAheadLog) Close() error {
	if w.markClosed() {
		return nil
	}

	w.checkpointWg.Wait()
	return w.closeResources()
}

// markClosed marks the WAL as closed and returns true if it was already closed.
func (w *WriteAheadLog) markClosed() bool {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return true
	}

	w.closed = true
	w.mu.Unlock()
	return false
}

// closeResources flushes buffered data and closes the underlying WAL file.
func (w *WriteAheadLog) closeResources() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			w.logger.WithError(err).Warn("Failed to flush WAL buffer on close")
		}
	}

	if w.file != nil {
		if err := w.file.Sync(); err != nil {
			w.logger.WithError(err).Warn("Failed to sync WAL file on close")
		}
		closeErr := w.file.Close()
		w.file = nil
		if closeErr != nil {
			return closeErr
		}
	}

	return nil
}

// Truncate removes all entries from the WAL file.
func (w *WriteAheadLog) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return errors.New("WAL is closed")
	}

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush before truncate: %w", err)
	}

	if err := w.file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate WAL file: %w", err)
	}

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek after truncate: %w", err)
	}

	w.sequence = 0
	w.entriesCount = 0
	w.lastCheckpoint = time.Now()

	w.logger.Info("WAL truncated")
	return nil
}

// Size returns the current size of the WAL file.
func (w *WriteAheadLog) Size() (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, errors.New("WAL is closed")
	}

	info, err := w.file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat WAL file: %w", err)
	}

	return info.Size(), nil
}

// Sequence returns the current sequence number.
func (w *WriteAheadLog) Sequence() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.sequence
}
