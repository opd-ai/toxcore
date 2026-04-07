// Package toxcore - toxcore_file.go contains file transfer functionality for the Tox instance.
// This file is part of the toxcore package refactoring to improve maintainability.
package toxcore

import (
	"encoding/binary"
	"errors"
	"fmt"

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
	// Validate friend exists and is connected
	f := t.friends.Get(friendID)
	if f == nil {
		return 0, errors.New("friend not found")
	}

	if f.ConnectionStatus == ConnectionNone {
		return 0, errors.New("friend is not connected")
	}

	// Validate parameters
	if len(filename) == 0 {
		return 0, errors.New("filename cannot be empty")
	}

	// Generate a unique local file transfer ID (simplified)
	localFileID := uint32(t.now().UnixNano() & 0xFFFFFFFF)

	// Create new file transfer
	transfer := file.NewTransfer(friendID, localFileID, filename, fileSize, file.TransferDirectionOutgoing)

	// Store the transfer
	transferKey := (uint64(friendID) << 32) | uint64(localFileID)
	t.transfersMu.Lock()
	t.fileTransfers[transferKey] = transfer
	t.transfersMu.Unlock()

	// Create and send file transfer request packet
	err := t.sendFileTransferRequest(friendID, localFileID, fileSize, fileID, filename)
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
func (t *Tox) sendFileTransferRequest(friendID, fileID uint32, fileSize uint64, fileHash [32]byte, filename string) error {
	packetData, err := t.createFileTransferPacketData(fileID, fileSize, fileHash, filename)
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
// Packet format: [fileID(4)][fileSize(8)][fileHash(32)][filename_length(2)][filename]
func (t *Tox) createFileTransferPacketData(fileID uint32, fileSize uint64, fileHash [32]byte, filename string) ([]byte, error) {
	filenameBytes := []byte(filename)
	if len(filenameBytes) > 65535 {
		return nil, errors.New("filename too long")
	}

	packetData := make([]byte, 4+8+32+2+len(filenameBytes))
	offset := 0

	// File ID (4 bytes)
	binary.BigEndian.PutUint32(packetData[offset:], fileID)
	offset += 4

	// File size (8 bytes)
	binary.BigEndian.PutUint64(packetData[offset:], fileSize)
	offset += 8

	// File hash (32 bytes)
	copy(packetData[offset:], fileHash[:])
	offset += 32

	// Filename length (2 bytes)
	binary.BigEndian.PutUint16(packetData[offset:], uint16(len(filenameBytes)))
	offset += 2

	// Filename
	copy(packetData[offset:], filenameBytes)

	return packetData, nil
}

// lookupFriendForTransfer retrieves the friend information needed for file transfer operations.
func (t *Tox) lookupFriendForTransfer(friendID uint32) (*Friend, error) {
	f := t.friends.Get(friendID)
	if f == nil {
		return nil, errors.New("friend not found for file transfer")
	}

	return f, nil
}

// lookupFileTransfer retrieves and validates a file transfer for the given friend and file IDs.
// Returns the transfer object if found and valid, otherwise returns an error.
func (t *Tox) lookupFileTransfer(friendID, fileID uint32) (*file.Transfer, error) {
	transferKey := (uint64(friendID) << 32) | uint64(fileID)
	t.transfersMu.RLock()
	transfer, exists := t.fileTransfers[transferKey]
	t.transfersMu.RUnlock()

	if !exists {
		return nil, errors.New("file transfer not found")
	}

	if transfer.State != file.TransferStateRunning {
		return nil, errors.New("transfer is not in running state")
	}

	return transfer, nil
}

// validateChunkData validates the chunk position and size according to protocol constraints.
// Returns an error if validation fails, otherwise returns nil.
func (t *Tox) validateChunkData(position uint64, data []byte, fileSize uint64) error {
	if position > fileSize {
		return errors.New("position exceeds file size")
	}

	const maxChunkSize = 1024 // 1KB chunks
	if len(data) > maxChunkSize {
		return fmt.Errorf("chunk size %d exceeds maximum %d", len(data), maxChunkSize)
	}

	return nil
}

// updateTransferProgress updates the transfer progress after a successful chunk send.
// This function is thread-safe and updates the transferred bytes count.
func (t *Tox) updateTransferProgress(friendID, fileID uint32, position uint64, dataLen int) {
	transferKey := (uint64(friendID) << 32) | uint64(fileID)
	t.transfersMu.Lock()
	if transfer, exists := t.fileTransfers[transferKey]; exists {
		transfer.Transferred = position + uint64(dataLen)
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
// Packet format: [fileID(4)][position(8)][data_length(2)][data]
func (t *Tox) buildFileChunkPacket(fileID uint32, position uint64, data []byte) []byte {
	dataLength := len(data)
	packetData := make([]byte, 4+8+2+dataLength)
	offset := 0

	// File ID (4 bytes)
	binary.BigEndian.PutUint32(packetData[offset:], fileID)
	offset += 4

	// Position (8 bytes)
	binary.BigEndian.PutUint64(packetData[offset:], position)
	offset += 8

	// Data length (2 bytes)
	binary.BigEndian.PutUint16(packetData[offset:], uint16(dataLength))
	offset += 2

	// Data
	copy(packetData[offset:], data)

	return packetData
}

// FileManager returns the file transfer manager.
func (t *Tox) FileManager() *file.Manager {
	return t.fileManager
}
