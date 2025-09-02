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
	"sync"
	"time"
)

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
	transferSpeed float64 // bytes per second
}

// NewTransfer creates a new file transfer.
//
//export ToxFileTransferNew
func NewTransfer(friendID, fileID uint32, fileName string, fileSize uint64, direction TransferDirection) *Transfer {
	return &Transfer{
		FriendID:      friendID,
		FileID:        fileID,
		Direction:     direction,
		FileName:      fileName,
		FileSize:      fileSize,
		State:         TransferStatePending,
		lastChunkTime: time.Now(),
	}
}

// Start begins the file transfer.
//
//export ToxFileTransferStart
func (t *Transfer) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransferStatePending && t.State != TransferStatePaused {
		return errors.New("transfer cannot be started in current state")
	}

	var err error

	// Open the file
	if t.Direction == TransferDirectionOutgoing {
		t.FileHandle, err = os.Open(t.FileName)
	} else {
		t.FileHandle, err = os.Create(t.FileName)
	}

	if err != nil {
		t.Error = err
		t.State = TransferStateError
		return err
	}

	t.State = TransferStateRunning
	t.StartTime = time.Now()

	return nil
}

// Pause temporarily halts the file transfer.
//
//export ToxFileTransferPause
func (t *Transfer) Pause() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransferStateRunning {
		return errors.New("transfer is not running")
	}

	t.State = TransferStatePaused
	return nil
}

// Resume continues a paused file transfer.
//
//export ToxFileTransferResume
func (t *Transfer) Resume() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransferStatePaused {
		return errors.New("transfer is not paused")
	}

	t.State = TransferStateRunning
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
		t.FileHandle.Close()
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

	if t.Direction != TransferDirectionIncoming {
		return errors.New("cannot write to outgoing transfer")
	}

	if t.State != TransferStateRunning {
		return errors.New("transfer is not running")
	}

	// Write the chunk to the file
	_, err := t.FileHandle.Write(data)
	if err != nil {
		t.Error = err
		t.State = TransferStateError

		if t.completeCallback != nil {
			t.completeCallback(err)
		}

		return err
	}

	// Update progress
	t.Transferred += uint64(len(data))
	t.updateTransferSpeed(uint64(len(data)))

	if t.progressCallback != nil {
		t.progressCallback(t.Transferred)
	}

	// Check if transfer is complete
	if t.Transferred >= t.FileSize {
		t.complete(nil)
	}

	return nil
}

// ReadChunk reads the next chunk from an outgoing file transfer.
func (t *Transfer) ReadChunk(size uint16) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

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
		t.FileHandle.Close()
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
	now := time.Now()
	duration := now.Sub(t.lastChunkTime).Seconds()

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
//
//export ToxFileTransferOnProgress
func (t *Transfer) OnProgress(callback func(uint64)) {
	t.progressCallback = callback
}

// OnComplete sets a callback function to be called when the transfer completes.
//
//export ToxFileTransferOnComplete
func (t *Transfer) OnComplete(callback func(error)) {
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
