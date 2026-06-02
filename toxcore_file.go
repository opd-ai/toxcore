package toxcore

// toxcore_file.go contains file transfer functionality for the Tox instance.
// This file is part of the toxcore package refactoring to improve maintainability.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/opd-ai/toxcore/file"
	"github.com/opd-ai/toxcore/transport"
)

// FileControl controls an active file transfer.
//
//export ToxFileControl
func (t *Tox) FileControl(friendID, fileID uint32, control FileControl) error {
	// Validate friend exists
	if !t.friends.Exists(friendID) {
		return errors.New("friend not found")
	}

	// Find the file transfer
	transferKey := (uint64(friendID) << 32) | uint64(fileID)
	t.transfersMu.RLock()
	transfer, exists := t.fileTransfers[transferKey]
	t.transfersMu.RUnlock()

	if !exists {
		return errors.New("file transfer not found")
	}

	// Apply the control action
	switch control {
	case FileControlResume:
		// An incoming transfer starts in Pending state; accepting it means
		// starting it (Pending → Running).  A paused transfer uses Resume.
		if transfer.GetState() == file.TransferStatePending {
			return transfer.Start()
		}
		return transfer.Resume()
	case FileControlPause:
		return transfer.Pause()
	case FileControlCancel:
		return transfer.Cancel()
	default:
		return errors.New("invalid file control action")
	}
}

// FileAccept accepts an incoming file transfer.
// This is a convenience method equivalent to FileControl(friendID, fileID, FileControlResume).
// Call this from the OnFileRecv callback to accept an incoming file transfer.
//
//export ToxFileAccept
func (t *Tox) FileAccept(friendID, fileID uint32) error {
	return t.FileControl(friendID, fileID, FileControlResume)
}

// FileReject rejects or cancels an incoming file transfer.
// This is a convenience method equivalent to FileControl(friendID, fileID, FileControlCancel).
// Call this from the OnFileRecv callback to reject an incoming file transfer.
//
//export ToxFileReject
func (t *Tox) FileReject(friendID, fileID uint32) error {
	return t.FileControl(friendID, fileID, FileControlCancel)
}

// FileSend starts a file transfer.
//
//export ToxFileSend
func (t *Tox) FileSend(friendID, kind uint32, fileSize uint64, fileID [32]byte, filename string) (uint32, error) {
	// Validate friend exists and is connected — snapshot inside lock to avoid data race (H-05).
	var connStatus ConnectionStatus
	if !t.friends.Read(friendID, func(f *Friend) { connStatus = f.ConnectionStatus }) {
		return 0, errors.New("friend not found")
	}

	if connStatus == ConnectionNone {
		return 0, errors.New("friend is not connected")
	}

	// Validate parameters
	if len(filename) == 0 {
		return 0, errors.New("filename cannot be empty")
	}

	// Generate a unique local file transfer ID using a monotonic counter to
	// avoid collisions from timestamp masking.
	localFileID := atomic.AddUint32(&t.fileIDCounter, 1)

	// Create new file transfer
	transfer := file.NewTransfer(friendID, localFileID, filename, fileSize, file.TransferDirectionOutgoing)

	// Store the transfer
	transferKey := (uint64(friendID) << 32) | uint64(localFileID)
	t.transfersMu.Lock()
	t.fileTransfers[transferKey] = transfer
	t.transfersMu.Unlock()

	// Create and send file transfer request packet
	err := t.sendFileTransferRequest(friendID, localFileID, fileSize, filename)
	if err != nil {
		// Clean up the transfer on send failure
		t.transfersMu.Lock()
		delete(t.fileTransfers, transferKey)
		t.transfersMu.Unlock()
		return 0, fmt.Errorf("failed to send file transfer request: %w", err)
	}

	return localFileID, nil
}

// sendFileTransferRequest creates and sends a file transfer request packet
func (t *Tox) sendFileTransferRequest(friendID, fileID uint32, fileSize uint64, filename string) error {
	packetData, err := t.createFileTransferPacketData(fileID, fileSize, filename)
	if err != nil {
		return err
	}

	packet := &transport.Packet{
		PacketType: transport.PacketFileRequest,
		Data:       packetData,
	}

	friend, err := t.lookupFriendForTransfer(friendID)
	if err != nil {
		return err
	}

	targetAddr, err := t.resolveFriendAddress(friend)
	if err != nil {
		return err
	}

	return t.sendPacketToTarget(packet, targetAddr)
}

// createFileTransferPacketData constructs the binary packet data for file transfer requests.
// Packet format: [fileID(4)][fileSize(8)][filename_length(2)][filename]
// This format matches file.deserializeFileRequest so sender and receiver are wire-compatible.
// The fileHash is stored in the local Transfer record for integrity checking but is not
// fileIDSize, fileSizeSize, and filenameLenSize are the fixed-width field sizes
// in the file-transfer request wire format.
const (
	fileIDSize      = 4
	fileSizeSize    = 8
	filenameLenSize = 2
)

// transmitted in the request packet (C-01 fix).
func (t *Tox) createFileTransferPacketData(fileID uint32, fileSize uint64, filename string) ([]byte, error) {
	filenameBytes := []byte(filename)
	if len(filenameBytes) > 65535 {
		return nil, errors.New("filename too long")
	}

	packetData := make([]byte, fileIDSize+fileSizeSize+filenameLenSize+len(filenameBytes))
	offset := 0

	// File ID (4 bytes)
	binary.BigEndian.PutUint32(packetData[offset:], fileID)
	offset += fileIDSize

	// File size (8 bytes)
	binary.BigEndian.PutUint64(packetData[offset:], fileSize)
	offset += fileSizeSize

	// Filename length (2 bytes)
	binary.BigEndian.PutUint16(packetData[offset:], uint16(len(filenameBytes)))
	offset += filenameLenSize

	// Filename
	copy(packetData[offset:], filenameBytes)

	return packetData, nil
}

// lookupFriendForTransfer retrieves a snapshot copy of the friend for file transfer operations.
// Returns a copy to avoid data races with concurrent mutations (H-05).
func (t *Tox) lookupFriendForTransfer(friendID uint32) (*Friend, error) {
	var snapshot Friend
	if !t.friends.Read(friendID, func(f *Friend) { snapshot = *f }) {
		return nil, errors.New("friend not found for file transfer")
	}
	return &snapshot, nil
}

// lookupFileTransfer retrieves and validates a file transfer for the given friend and file IDs.
// Returns the transfer object if found and valid, otherwise returns an error.
func (t *Tox) lookupFileTransfer(friendID, fileID uint32) (*file.Transfer, error) {
	transferKey := (uint64(friendID) << 32) | uint64(fileID)
	t.transfersMu.RLock()
	transfer, exists := t.fileTransfers[transferKey]
	t.transfersMu.RUnlock()

	if !exists || transfer == nil {
		return nil, errors.New("file transfer not found")
	}

	if transfer.GetState() != file.TransferStateRunning {
		return nil, errors.New("transfer is not in running state")
	}

	return transfer, nil
}

// validateChunkData validates the chunk position and size according to protocol constraints.
// Returns an error if validation fails, otherwise returns nil.
func (t *Tox) validateChunkData(position uint64, data []byte, fileSize uint64) error {
	if position >= fileSize {
		return errors.New("position exceeds file size")
	}

	const maxChunkSize = 1024 // 1KB chunks
	if len(data) > maxChunkSize {
		return fmt.Errorf("chunk size %d exceeds maximum %d", len(data), maxChunkSize)
	}
	if uint64(len(data)) > fileSize-position {
		return fmt.Errorf("chunk length %d exceeds remaining bytes %d", len(data), fileSize-position)
	}

	return nil
}

// updateTransferProgress updates the transfer progress after a successful chunk send.
// This function is thread-safe and updates the transferred bytes count.
func (t *Tox) updateTransferProgress(friendID, fileID uint32, position uint64, dataLen int) {
	transferKey := (uint64(friendID) << 32) | uint64(fileID)
	t.transfersMu.Lock()
	if transfer, exists := t.fileTransfers[transferKey]; exists {
		transfer.SetTransferred(position + uint64(dataLen))
	}
	t.transfersMu.Unlock()
}

// FileSendChunk sends a chunk of file data.
//
//export ToxFileSendChunk
func (t *Tox) FileSendChunk(friendID, fileID uint32, position uint64, data []byte) error {
	// Validate friend exists and is connected
	_, err := t.validateFriendConnection(friendID)
	if err != nil {
		return err
	}

	// Find and validate file transfer
	transfer, err := t.lookupFileTransfer(friendID, fileID)
	if err != nil {
		return err
	}

	// Validate chunk data
	err = t.validateChunkData(position, data, transfer.FileSize)
	if err != nil {
		return err
	}

	// Create and send file chunk packet
	err = t.sendFileChunk(friendID, fileID, position, data)
	if err != nil {
		return fmt.Errorf("failed to send file chunk: %w", err)
	}

	// Update transfer progress on successful send
	t.updateTransferProgress(friendID, fileID, position, len(data))

	return nil
}

// sendFileChunk creates and sends a file data chunk packet
func (t *Tox) sendFileChunk(friendID, fileID uint32, position uint64, data []byte) error {
	friend, err := t.validateFriendConnection(friendID)
	if err != nil {
		return fmt.Errorf("friend not found for file chunk transfer: %w", err)
	}

	packetData := t.buildFileChunkPacket(fileID, position, data)

	packet := &transport.Packet{
		PacketType: transport.PacketFileData,
		Data:       packetData,
	}

	targetAddr, err := t.resolveFriendAddress(friend)
	if err != nil {
		return err
	}

	return t.sendPacketToTarget(packet, targetAddr)
}

// buildFileChunkPacket creates the binary packet data for a file chunk.
// Packet format: [fileID(4)][position(8)][data]
func (t *Tox) buildFileChunkPacket(fileID uint32, position uint64, data []byte) []byte {
	packetData := make([]byte, 4+8+len(data))

	// File ID (4 bytes)
	binary.BigEndian.PutUint32(packetData[0:4], fileID)

	// Position (8 bytes)
	binary.BigEndian.PutUint64(packetData[4:12], position)

	// Data
	copy(packetData[12:], data)

	return packetData
}

// FileManager returns the file transfer manager.
func (t *Tox) FileManager() *file.Manager {
	return t.fileManager
}
