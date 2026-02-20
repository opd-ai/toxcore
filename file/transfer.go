// Package file implements file transfer functionality for the Tox protocol.
//
// This package handles sending and receiving files between Tox users,
// with support for pausing, resuming, and canceling transfers.
//
// Example:
//
//	transfer := file.NewTransfer(friendID, fileID, fileName, fileSize)
//	transfer.OnProgress(func(received uint64) {
//	    fmt.Printf("Progress: %.2f%%\n", float64(received) / float64(fileSize) * 100)
//	})
//	transfer.Start()
package file

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrDirectoryTraversal indicates an attempt to access files outside allowed directories.
var ErrDirectoryTraversal = errors.New("path contains directory traversal")

// ErrChunkTooLarge indicates that a chunk exceeds the maximum allowed size.
var ErrChunkTooLarge = errors.New("chunk size exceeds maximum allowed")

// ErrFileNameTooLong indicates that a file name exceeds the maximum allowed length.
var ErrFileNameTooLong = errors.New("file name too long")

// ErrTransferStalled indicates that a transfer has not received data within the timeout period.
var ErrTransferStalled = errors.New("transfer stalled: no data received within timeout period")

// TransferDirection indicates whether a transfer is incoming or outgoing.
type TransferDirection uint8

const (
	// TransferDirectionIncoming represents a file being received.
	TransferDirectionIncoming TransferDirection = iota
	// TransferDirectionOutgoing represents a file being sent.
	TransferDirectionOutgoing
)

// TransferState represents the current state of a file transfer.
type TransferState uint8

const (
	// TransferStatePending indicates the transfer is waiting to start.
	TransferStatePending TransferState = iota
	// TransferStateRunning indicates the transfer is in progress.
	TransferStateRunning
	// TransferStatePaused indicates the transfer is temporarily paused.
	TransferStatePaused
	// TransferStateCompleted indicates the transfer has finished successfully.
	TransferStateCompleted
	// TransferStateCancelled indicates the transfer was cancelled.
	TransferStateCancelled
	// TransferStateError indicates the transfer failed due to an error.
	TransferStateError
)

// ChunkSize is the size of each file chunk in bytes.
const ChunkSize = 1024

// MaxChunkSize is the maximum allowed chunk size to prevent resource exhaustion.
const MaxChunkSize = 65536

// MaxFileNameLength is the maximum allowed file name length in bytes.
// This prevents DoS via memory exhaustion from excessively long names.
// The value (255) matches typical filesystem limits and fits in a uint16.
const MaxFileNameLength = 255

// DefaultStallTimeout is the default timeout duration for detecting stalled transfers.
// Transfers that receive no data for this duration are considered stalled.
const DefaultStallTimeout = 30 * time.Second

// TimeProvider abstracts time operations for deterministic testing.
type TimeProvider interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// DefaultTimeProvider uses the standard library time functions.
type DefaultTimeProvider struct{}

// Now returns the current time.
func (DefaultTimeProvider) Now() time.Time { return time.Now() }

// Since returns the duration since t.
func (DefaultTimeProvider) Since(t time.Time) time.Duration { return time.Since(t) }

// defaultTimeProvider is the package-level default time provider.
var defaultTimeProvider TimeProvider = DefaultTimeProvider{}

// Transfer represents a file transfer operation.
//
//export ToxFileTransfer
type Transfer struct {
	FriendID    uint32
	FileID      uint32
	Direction   TransferDirection
	FileName    string
	FileSize    uint64
	State       TransferState
	StartTime   time.Time
	Transferred uint64
	FileHandle  *os.File
	Error       error

	progressCallback func(uint64)
	completeCallback func(error)

	mu            sync.Mutex
	lastChunkTime time.Time
	transferSpeed float64       // bytes per second
	stallTimeout  time.Duration // timeout for stalled transfer detection
	timeProvider  TimeProvider
	acknowledged  uint64 // bytes acknowledged by peer (for flow control)
	ackCallback   func(uint64)
}

// NewTransfer creates a new file transfer.
//
//export ToxFileTransferNew
func NewTransfer(friendID, fileID uint32, fileName string, fileSize uint64, direction TransferDirection) *Transfer {
	logrus.WithFields(logrus.Fields{
		"function":  "NewTransfer",
		"friend_id": friendID,
		"file_id":   fileID,
		"file_name": fileName,
		"file_size": fileSize,
		"direction": direction,
	}).Info("Creating new file transfer")

	tp := defaultTimeProvider
	transfer := &Transfer{
		FriendID:      friendID,
		FileID:        fileID,
		Direction:     direction,
		FileName:      fileName,
		FileSize:      fileSize,
		State:         TransferStatePending,
		lastChunkTime: tp.Now(),
		stallTimeout:  DefaultStallTimeout,
		timeProvider:  tp,
	}

	logrus.WithFields(logrus.Fields{
		"function":  "NewTransfer",
		"friend_id": friendID,
		"file_id":   fileID,
		"state":     transfer.State,
	}).Info("File transfer created successfully")

	return transfer
}

// SetTimeProvider sets a custom time provider for deterministic testing.
// Also resets lastChunkTime to the new provider's current time to ensure
// consistent timeout behavior after changing providers.
func (t *Transfer) SetTimeProvider(tp TimeProvider) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timeProvider = tp
	t.lastChunkTime = tp.Now()
}

// ValidatePath checks if a file path is safe from directory traversal attacks.
// It returns the cleaned path or an error if the path contains traversal attempts.
func ValidatePath(path string) (string, error) {
	// Clean the path to resolve any . or .. components
	cleanedPath := filepath.Clean(path)

	// Check for path traversal indicators
	if strings.Contains(cleanedPath, "..") {
		return "", ErrDirectoryTraversal
	}

	// On Unix systems, check for absolute paths that could escape
	if filepath.IsAbs(cleanedPath) {
		// Allow absolute paths, but verify they don't contain traversal after cleaning
		parts := strings.Split(cleanedPath, string(filepath.Separator))
		for _, part := range parts {
			if part == ".." {
				return "", ErrDirectoryTraversal
			}
		}
	}

	return cleanedPath, nil
}

// Start begins the file transfer.
//
//export ToxFileTransferStart
func (t *Transfer) Start() error {
	logrus.WithFields(logrus.Fields{
		"function":  "Start",
		"friend_id": t.FriendID,
		"file_id":   t.FileID,
		"file_name": t.FileName,
		"direction": t.Direction,
		"state":     t.State,
	}).Info("Starting file transfer")

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransferStatePending && t.State != TransferStatePaused {
		logrus.WithFields(logrus.Fields{
			"function":       "Start",
			"friend_id":      t.FriendID,
			"file_id":        t.FileID,
			"current_state":  t.State,
			"expected_state": "TransferStatePending or TransferStatePaused",
		}).Error("Transfer cannot be started in current state")
		return errors.New("transfer cannot be started in current state")
	}

	// Validate file path to prevent directory traversal attacks
	safePath, err := ValidatePath(t.FileName)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "Start",
			"friend_id": t.FriendID,
			"file_id":   t.FileID,
			"file_name": t.FileName,
			"error":     err.Error(),
		}).Error("File path validation failed")
		t.Error = err
		t.State = TransferStateError
		return err
	}
	t.FileName = safePath

	// Open the file
	if t.Direction == TransferDirectionOutgoing {
		logrus.WithFields(logrus.Fields{
			"function":  "Start",
			"friend_id": t.FriendID,
			"file_id":   t.FileID,
			"file_name": t.FileName,
			"operation": "opening file for reading",
		}).Debug("Opening file for outgoing transfer")
		t.FileHandle, err = os.Open(t.FileName)
	} else {
		logrus.WithFields(logrus.Fields{
			"function":  "Start",
			"friend_id": t.FriendID,
			"file_id":   t.FileID,
			"file_name": t.FileName,
			"operation": "creating file for writing",
		}).Debug("Creating file for incoming transfer")
		t.FileHandle, err = os.Create(t.FileName)
	}

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "Start",
			"friend_id": t.FriendID,
			"file_id":   t.FileID,
			"file_name": t.FileName,
			"direction": t.Direction,
			"error":     err.Error(),
		}).Error("Failed to open/create file for transfer")
		t.Error = err
		t.State = TransferStateError
		return err
	}

	t.State = TransferStateRunning
	t.StartTime = t.timeProvider.Now()

	logrus.WithFields(logrus.Fields{
		"function":   "Start",
		"friend_id":  t.FriendID,
		"file_id":    t.FileID,
		"file_name":  t.FileName,
		"direction":  t.Direction,
		"start_time": t.StartTime,
		"state":      t.State,
	}).Info("File transfer started successfully")

	return nil
}

// Pause temporarily halts the file transfer.
//
//export ToxFileTransferPause
func (t *Transfer) Pause() error {
	logrus.WithFields(logrus.Fields{
		"function":  "Pause",
		"friend_id": t.FriendID,
		"file_id":   t.FileID,
		"state":     t.State,
	}).Info("Pausing file transfer")

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransferStateRunning {
		logrus.WithFields(logrus.Fields{
			"function":       "Pause",
			"friend_id":      t.FriendID,
			"file_id":        t.FileID,
			"current_state":  t.State,
			"expected_state": "TransferStateRunning",
		}).Error("Transfer is not running and cannot be paused")
		return errors.New("transfer is not running")
	}

	t.State = TransferStatePaused

	logrus.WithFields(logrus.Fields{
		"function":  "Pause",
		"friend_id": t.FriendID,
		"file_id":   t.FileID,
		"state":     t.State,
	}).Info("File transfer paused successfully")

	return nil
}

// Resume continues a paused file transfer.
//
//export ToxFileTransferResume
func (t *Transfer) Resume() error {
	logrus.WithFields(logrus.Fields{
		"function":  "Resume",
		"friend_id": t.FriendID,
		"file_id":   t.FileID,
		"state":     t.State,
	}).Info("Resuming file transfer")

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransferStatePaused {
		logrus.WithFields(logrus.Fields{
			"function":       "Resume",
			"friend_id":      t.FriendID,
			"file_id":        t.FileID,
			"current_state":  t.State,
			"expected_state": "TransferStatePaused",
		}).Error("Transfer is not paused and cannot be resumed")
		return errors.New("transfer is not paused")
	}

	t.State = TransferStateRunning

	logrus.WithFields(logrus.Fields{
		"function":  "Resume",
		"friend_id": t.FriendID,
		"file_id":   t.FileID,
		"state":     t.State,
	}).Info("File transfer resumed successfully")

	return nil
}

// Cancel aborts the file transfer.
//
//export ToxFileTransferCancel
func (t *Transfer) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State == TransferStateCompleted || t.State == TransferStateCancelled {
		return errors.New("transfer already finished")
	}

	if t.FileHandle != nil {
		if closeErr := t.FileHandle.Close(); closeErr != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "Cancel",
				"friend_id": t.FriendID,
				"file_id":   t.FileID,
				"file_name": t.FileName,
				"error":     closeErr.Error(),
			}).Warn("Failed to close file handle during cancel")
		}
	}

	t.State = TransferStateCancelled

	if t.completeCallback != nil {
		t.completeCallback(errors.New("transfer cancelled"))
	}

	return nil
}

// WriteChunk adds data to an incoming file transfer.
func (t *Transfer) WriteChunk(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate chunk size to prevent resource exhaustion
	if len(data) > MaxChunkSize {
		logrus.WithFields(logrus.Fields{
			"function":       "WriteChunk",
			"friend_id":      t.FriendID,
			"file_id":        t.FileID,
			"chunk_size":     len(data),
			"max_chunk_size": MaxChunkSize,
		}).Error("Chunk size exceeds maximum allowed")
		return ErrChunkTooLarge
	}

	if err := t.validateWriteRequest(); err != nil {
		return err
	}

	if err := t.writeDataToFile(data); err != nil {
		return err
	}

	t.updateWriteProgress(data)
	t.checkTransferCompletion()

	return nil
}

// validateWriteRequest checks if the transfer is in a valid state for writing.
func (t *Transfer) validateWriteRequest() error {
	if t.Direction != TransferDirectionIncoming {
		return errors.New("cannot write to outgoing transfer")
	}

	if t.State != TransferStateRunning {
		return errors.New("transfer is not running")
	}

	return nil
}

// writeDataToFile writes the chunk data to the file and handles write errors.
func (t *Transfer) writeDataToFile(data []byte) error {
	_, err := t.FileHandle.Write(data)
	if err != nil {
		t.Error = err
		t.State = TransferStateError

		if t.completeCallback != nil {
			t.completeCallback(err)
		}

		return err
	}

	return nil
}

// updateWriteProgress updates transfer progress and speed metrics after writing data.
func (t *Transfer) updateWriteProgress(data []byte) {
	t.Transferred += uint64(len(data))
	t.updateTransferSpeed(uint64(len(data)))

	if t.progressCallback != nil {
		t.progressCallback(t.Transferred)
	}
}

// checkTransferCompletion checks if the transfer is complete and triggers completion if needed.
func (t *Transfer) checkTransferCompletion() {
	if t.Transferred >= t.FileSize {
		t.complete(nil)
	}
}

// ReadChunk reads the next chunk from an outgoing file transfer.
func (t *Transfer) ReadChunk(size uint16) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate chunk size to prevent resource exhaustion
	if int(size) > MaxChunkSize {
		logrus.WithFields(logrus.Fields{
			"function":       "ReadChunk",
			"friend_id":      t.FriendID,
			"file_id":        t.FileID,
			"chunk_size":     size,
			"max_chunk_size": MaxChunkSize,
		}).Error("Chunk size exceeds maximum allowed")
		return nil, ErrChunkTooLarge
	}

	if err := t.validateReadRequest(); err != nil {
		return nil, err
	}

	chunk, n, err := t.readFileChunk(size)
	if err != nil {
		return t.handleReadError(err, chunk, n)
	}

	t.updateReadProgress(uint64(n))
	return chunk[:n], nil
}

// validateReadRequest checks if a read operation is valid for the current transfer state.
func (t *Transfer) validateReadRequest() error {
	if t.Direction != TransferDirectionOutgoing {
		return errors.New("cannot read from incoming transfer")
	}

	if t.State != TransferStateRunning {
		return errors.New("transfer is not running")
	}

	return nil
}

// readFileChunk reads data from the file and returns the chunk, bytes read, and any error.
func (t *Transfer) readFileChunk(size uint16) ([]byte, int, error) {
	chunk := make([]byte, size)
	n, err := t.FileHandle.Read(chunk)
	return chunk, n, err
}

// handleReadError processes read errors including EOF conditions.
func (t *Transfer) handleReadError(err error, chunk []byte, n int) ([]byte, error) {
	if err == io.EOF {
		return t.handleEOF(chunk, n)
	}

	t.Error = err
	t.State = TransferStateError

	if t.completeCallback != nil {
		t.completeCallback(err)
	}

	return nil, err
}

// handleEOF processes end-of-file conditions and determines if transfer is complete.
func (t *Transfer) handleEOF(chunk []byte, n int) ([]byte, error) {
	if t.Transferred+uint64(n) >= t.FileSize {
		t.complete(nil)
	}

	if n == 0 {
		return nil, io.EOF
	}

	// Return the final partial chunk
	return chunk[:n], nil
}

// updateReadProgress updates transfer progress and invokes progress callbacks.
func (t *Transfer) updateReadProgress(bytesRead uint64) {
	t.Transferred += bytesRead
	t.updateTransferSpeed(bytesRead)

	if t.progressCallback != nil {
		t.progressCallback(t.Transferred)
	}
}

// complete marks the transfer as completed.
func (t *Transfer) complete(err error) {
	if t.FileHandle != nil {
		if closeErr := t.FileHandle.Close(); closeErr != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "complete",
				"friend_id": t.FriendID,
				"file_id":   t.FileID,
				"file_name": t.FileName,
				"error":     closeErr.Error(),
			}).Warn("Failed to close file handle during completion")
		}
	}

	if err != nil {
		t.State = TransferStateError
		t.Error = err
	} else {
		t.State = TransferStateCompleted
	}

	if t.completeCallback != nil {
		t.completeCallback(err)
	}
}

// updateTransferSpeed calculates the current transfer speed.
func (t *Transfer) updateTransferSpeed(chunkSize uint64) {
	now := t.timeProvider.Now()
	duration := t.timeProvider.Since(t.lastChunkTime).Seconds()

	if duration > 0 {
		instantSpeed := float64(chunkSize) / duration

		// Exponential moving average with alpha = 0.3
		if t.transferSpeed == 0 {
			t.transferSpeed = instantSpeed
		} else {
			t.transferSpeed = 0.7*t.transferSpeed + 0.3*instantSpeed
		}
	}

	t.lastChunkTime = now
}

// OnProgress sets a callback function to be called when progress updates.
// This method is safe for concurrent use.
//
//export ToxFileTransferOnProgress
func (t *Transfer) OnProgress(callback func(uint64)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.progressCallback = callback
}

// OnComplete sets a callback function to be called when the transfer completes.
// This method is safe for concurrent use.
//
//export ToxFileTransferOnComplete
func (t *Transfer) OnComplete(callback func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.completeCallback = callback
}

// GetProgress returns the current progress of the transfer as a percentage.
//
//export ToxFileTransferGetProgress
func (t *Transfer) GetProgress() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.FileSize == 0 {
		return 0.0
	}

	return float64(t.Transferred) / float64(t.FileSize) * 100.0
}

// GetSpeed returns the current transfer speed in bytes per second.
//
//export ToxFileTransferGetSpeed
func (t *Transfer) GetSpeed() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.transferSpeed
}

// GetEstimatedTimeRemaining returns the estimated time remaining for the transfer.
//
//export ToxFileTransferGetEstimatedTimeRemaining
func (t *Transfer) GetEstimatedTimeRemaining() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransferStateRunning || t.transferSpeed <= 0 {
		return 0
	}

	bytesRemaining := t.FileSize - t.Transferred
	secondsRemaining := float64(bytesRemaining) / t.transferSpeed

	return time.Duration(secondsRemaining * float64(time.Second))
}

// SetStallTimeout configures the timeout duration for detecting stalled transfers.
// A transfer is considered stalled if no data is received within this duration.
// Set to 0 to disable stall detection.
//
//export ToxFileTransferSetStallTimeout
func (t *Transfer) SetStallTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stallTimeout = timeout

	logrus.WithFields(logrus.Fields{
		"function":      "SetStallTimeout",
		"friend_id":     t.FriendID,
		"file_id":       t.FileID,
		"stall_timeout": timeout,
	}).Debug("Stall timeout configured")
}

// GetStallTimeout returns the current stall timeout duration.
//
//export ToxFileTransferGetStallTimeout
func (t *Transfer) GetStallTimeout() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stallTimeout
}

// IsStalled returns true if the transfer has not received data within the stall timeout.
// Returns false if stall detection is disabled (timeout is 0) or if the transfer
// is not in the Running state.
//
//export ToxFileTransferIsStalled
func (t *Transfer) IsStalled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Stall detection disabled
	if t.stallTimeout == 0 {
		return false
	}

	// Only running transfers can be stalled
	if t.State != TransferStateRunning {
		return false
	}

	timeSinceLastChunk := t.timeProvider.Since(t.lastChunkTime)
	return timeSinceLastChunk >= t.stallTimeout
}

// CheckTimeout checks if the transfer has stalled and marks it as errored if so.
// Returns ErrTransferStalled if the transfer has stalled, nil otherwise.
// This method should be called periodically (e.g., in an iteration loop) to
// detect and handle stalled transfers.
//
//export ToxFileTransferCheckTimeout
func (t *Transfer) CheckTimeout() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Stall detection disabled
	if t.stallTimeout == 0 {
		return nil
	}

	// Only running transfers can be stalled
	if t.State != TransferStateRunning {
		return nil
	}

	timeSinceLastChunk := t.timeProvider.Since(t.lastChunkTime)
	if timeSinceLastChunk >= t.stallTimeout {
		logrus.WithFields(logrus.Fields{
			"function":             "CheckTimeout",
			"friend_id":            t.FriendID,
			"file_id":              t.FileID,
			"file_name":            t.FileName,
			"stall_timeout":        t.stallTimeout,
			"time_since_last_data": timeSinceLastChunk,
			"transferred":          t.Transferred,
			"file_size":            t.FileSize,
		}).Warn("Transfer stalled: no data received within timeout period")

		t.Error = ErrTransferStalled
		t.State = TransferStateError

		if t.completeCallback != nil {
			t.completeCallback(ErrTransferStalled)
		}

		return ErrTransferStalled
	}

	return nil
}

// GetTimeSinceLastChunk returns the duration since the last chunk was received.
// This can be used to monitor transfer activity without triggering timeout handling.
//
//export ToxFileTransferGetTimeSinceLastChunk
func (t *Transfer) GetTimeSinceLastChunk() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.timeProvider.Since(t.lastChunkTime)
}

// SetAcknowledgedBytes updates the number of bytes acknowledged by the peer.
// This is used for flow control in outgoing transfers to track confirmation
// of successfully received data.
//
//export ToxFileTransferSetAcknowledgedBytes
func (t *Transfer) SetAcknowledgedBytes(bytes uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.acknowledged = bytes

	logrus.WithFields(logrus.Fields{
		"function":     "SetAcknowledgedBytes",
		"friend_id":    t.FriendID,
		"file_id":      t.FileID,
		"acknowledged": bytes,
		"transferred":  t.Transferred,
	}).Debug("Updated acknowledged bytes")

	if t.ackCallback != nil {
		t.ackCallback(bytes)
	}
}

// GetAcknowledgedBytes returns the number of bytes acknowledged by the peer.
//
//export ToxFileTransferGetAcknowledgedBytes
func (t *Transfer) GetAcknowledgedBytes() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.acknowledged
}

// OnAcknowledge sets a callback function to be called when acknowledgment updates.
// This method is safe for concurrent use.
//
//export ToxFileTransferOnAcknowledge
func (t *Transfer) OnAcknowledge(callback func(uint64)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ackCallback = callback
}

// GetPendingBytes returns the number of bytes sent but not yet acknowledged.
// This can be used for flow control decisions (e.g., backpressure).
//
//export ToxFileTransferGetPendingBytes
func (t *Transfer) GetPendingBytes() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Transferred > t.acknowledged {
		return t.Transferred - t.acknowledged
	}
	return 0
}
