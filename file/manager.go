// Package file provides file transfer coordination with network transport.
package file

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// AddressResolver resolves network addresses to friend IDs.
// This interface allows the file transfer manager to properly map incoming
// connections to the correct friend for transfer tracking.
type AddressResolver interface {
	// ResolveFriendID returns the friend ID associated with the given address,
	// or an error if the address cannot be resolved to a known friend.
	ResolveFriendID(addr net.Addr) (uint32, error)
}

// AddressResolverFunc is a function type that implements AddressResolver.
type AddressResolverFunc func(addr net.Addr) (uint32, error)

// ResolveFriendID implements AddressResolver for AddressResolverFunc.
func (f AddressResolverFunc) ResolveFriendID(addr net.Addr) (uint32, error) {
	return f(addr)
}

// Manager coordinates file transfers with the network transport layer.
type Manager struct {
	transport       transport.Transport
	transfers       map[transferKey]*Transfer
	addressResolver AddressResolver
	mu              sync.RWMutex
}

// transferKey uniquely identifies a file transfer.
type transferKey struct {
	friendID uint32
	fileID   uint32
}

// NewManager creates a new file transfer manager with transport integration.
func NewManager(t transport.Transport) *Manager {
	logrus.WithFields(logrus.Fields{
		"function": "NewManager",
	}).Info("Creating new file transfer manager")

	m := &Manager{
		transport:       t,
		transfers:       make(map[transferKey]*Transfer),
		addressResolver: nil, // Must be set via SetAddressResolver for proper friend ID resolution
	}

	// Register packet handlers for file transfer
	if t != nil {
		t.RegisterHandler(transport.PacketFileRequest, m.handleFileRequest)
		t.RegisterHandler(transport.PacketFileControl, m.handleFileControl)
		t.RegisterHandler(transport.PacketFileData, m.handleFileData)
		t.RegisterHandler(transport.PacketFileDataAck, m.handleFileDataAck)
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewManager",
	}).Info("File transfer manager created with handlers registered")

	return m
}

// SetAddressResolver sets the resolver used to map network addresses to friend IDs.
// This must be called before handling incoming file transfers to properly track
// which friend each transfer belongs to.
func (m *Manager) SetAddressResolver(resolver AddressResolver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addressResolver = resolver
	logrus.WithFields(logrus.Fields{
		"function":     "SetAddressResolver",
		"resolver_set": resolver != nil,
	}).Info("Address resolver configured")
}

// resolveFriendIDFromAddr resolves a friend ID from a network address using the
// configured resolver. If no resolver is configured or resolution fails, it
// returns the fallback value.
func (m *Manager) resolveFriendIDFromAddr(addr net.Addr, fallback uint32, functionName string) uint32 {
	m.mu.RLock()
	resolver := m.addressResolver
	m.mu.RUnlock()

	if resolver != nil {
		friendID, err := resolver.ResolveFriendID(addr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": functionName,
				"address":  addr.String(),
				"fallback": fallback,
				"error":    err.Error(),
			}).Warn("Failed to resolve friend ID from address, using fallback")
			return fallback
		}
		return friendID
	}

	// No resolver configured - use fallback
	logrus.WithFields(logrus.Fields{
		"function": functionName,
		"address":  addr.String(),
		"fallback": fallback,
	}).Debug("No address resolver configured, using fallback friendID")
	return fallback
}

// SendFile initiates an outgoing file transfer to a friend.
func (m *Manager) SendFile(friendID, fileID uint32, fileName string, fileSize uint64, addr net.Addr) (*Transfer, error) {
	logrus.WithFields(logrus.Fields{
		"function":  "SendFile",
		"friend_id": friendID,
		"file_id":   fileID,
		"file_name": fileName,
		"file_size": fileSize,
	}).Info("Initiating outgoing file transfer")

	m.mu.Lock()
	defer m.mu.Unlock()

	key := transferKey{friendID: friendID, fileID: fileID}
	if _, exists := m.transfers[key]; exists {
		return nil, fmt.Errorf("transfer already exists for friend %d file %d", friendID, fileID)
	}

	transfer := NewTransfer(friendID, fileID, fileName, fileSize, TransferDirectionOutgoing)
	m.transfers[key] = transfer

	// Send file request packet
	if m.transport != nil {
		packet := &transport.Packet{
			PacketType: transport.PacketFileRequest,
			Data:       serializeFileRequest(fileID, fileName, fileSize),
		}
		if err := m.transport.Send(packet, addr); err != nil {
			delete(m.transfers, key)
			return nil, fmt.Errorf("failed to send file request: %w", err)
		}
	}

	logrus.WithFields(logrus.Fields{
		"function":  "SendFile",
		"friend_id": friendID,
		"file_id":   fileID,
	}).Info("File transfer request sent successfully")

	return transfer, nil
}

// GetTransfer retrieves an active file transfer.
func (m *Manager) GetTransfer(friendID, fileID uint32) (*Transfer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := transferKey{friendID: friendID, fileID: fileID}
	transfer, exists := m.transfers[key]
	if !exists {
		return nil, fmt.Errorf("transfer not found for friend %d file %d", friendID, fileID)
	}

	return transfer, nil
}

// SendChunk sends the next chunk of data for an outgoing transfer.
func (m *Manager) SendChunk(friendID, fileID uint32, addr net.Addr) error {
	transfer, err := m.GetTransfer(friendID, fileID)
	if err != nil {
		return err
	}

	chunk, err := transfer.ReadChunk(ChunkSize)
	if err != nil {
		return fmt.Errorf("failed to read chunk: %w", err)
	}

	if m.transport != nil {
		packet := &transport.Packet{
			PacketType: transport.PacketFileData,
			Data:       serializeFileData(fileID, chunk),
		}
		if err := m.transport.Send(packet, addr); err != nil {
			return fmt.Errorf("failed to send file data: %w", err)
		}
	}

	return nil
}

// handleFileRequest processes incoming file transfer requests.
func (m *Manager) handleFileRequest(packet *transport.Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function": "handleFileRequest",
		"from":     addr.String(),
	}).Debug("Handling file request packet")

	fileID, fileName, fileSize, err := deserializeFileRequest(packet.Data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "handleFileRequest",
			"error":    err.Error(),
		}).Error("Failed to deserialize file request")
		return err
	}

	// Resolve friend ID from address using the configured resolver
	friendID := m.resolveFriendIDFromAddr(addr, fileID, "handleFileRequest")

	m.mu.Lock()
	key := transferKey{friendID: friendID, fileID: fileID}
	transfer := NewTransfer(friendID, fileID, fileName, fileSize, TransferDirectionIncoming)
	m.transfers[key] = transfer
	m.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":  "handleFileRequest",
		"friend_id": friendID,
		"file_id":   fileID,
		"file_name": fileName,
		"file_size": fileSize,
	}).Info("Incoming file transfer created")

	return nil
}

// handleFileControl processes file transfer control messages (pause, resume, cancel).
func (m *Manager) handleFileControl(packet *transport.Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function": "handleFileControl",
		"from":     addr.String(),
	}).Debug("Handling file control packet")

	// Control packet format: [file_id (4 bytes)][control_type (1 byte)]
	if len(packet.Data) < 5 {
		logrus.Error("File control packet too short")
		return errors.New("file control packet too short")
	}

	fileID := binary.BigEndian.Uint32(packet.Data[0:4])
	controlType := packet.Data[4]

	// Resolve friend ID from address using the configured resolver
	friendID := m.resolveFriendIDFromAddr(addr, fileID, "handleFileControl")

	transfer, err := m.GetTransfer(friendID, fileID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "handleFileControl",
			"friend_id": friendID,
			"file_id":   fileID,
			"error":     err.Error(),
		}).Warn("Transfer not found for control message")
		return err
	}

	switch controlType {
	case 1: // Pause
		return transfer.Pause()
	case 2: // Resume
		return transfer.Resume()
	case 3: // Cancel
		return transfer.Cancel()
	default:
		return fmt.Errorf("unknown control type: %d", controlType)
	}
}

// handleFileData processes incoming file data chunks.
func (m *Manager) handleFileData(packet *transport.Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function": "handleFileData",
		"from":     addr.String(),
	}).Debug("Handling file data packet")

	fileID, chunk, err := deserializeFileData(packet.Data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "handleFileData",
			"error":    err.Error(),
		}).Error("Failed to deserialize file data")
		return err
	}

	// Resolve friend ID from address using the configured resolver
	friendID := m.resolveFriendIDFromAddr(addr, fileID, "handleFileData")

	transfer, err := m.GetTransfer(friendID, fileID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "handleFileData",
			"friend_id": friendID,
			"file_id":   fileID,
			"error":     err.Error(),
		}).Warn("Transfer not found for data packet")
		return err
	}

	if err := transfer.WriteChunk(chunk); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "handleFileData",
			"file_id":  fileID,
			"error":    err.Error(),
		}).Error("Failed to write chunk to transfer")
		return err
	}

	// Send acknowledgment
	if m.transport != nil {
		ackPacket := &transport.Packet{
			PacketType: transport.PacketFileDataAck,
			Data:       serializeFileDataAck(fileID, transfer.Transferred),
		}
		if err := m.transport.Send(ackPacket, addr); err != nil {
			return fmt.Errorf("failed to send acknowledgment: %w", err)
		}
	}

	return nil
}

// handleFileDataAck processes file data acknowledgments.
func (m *Manager) handleFileDataAck(packet *transport.Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function": "handleFileDataAck",
		"from":     addr.String(),
	}).Debug("Handling file data acknowledgment")

	// Ack packet format: [file_id (4 bytes)][bytes_received (8 bytes)]
	if len(packet.Data) < 12 {
		logrus.Error("File data ack packet too short")
		return errors.New("file data ack packet too short")
	}

	fileID := binary.BigEndian.Uint32(packet.Data[0:4])
	bytesReceived := binary.BigEndian.Uint64(packet.Data[4:12])

	logrus.WithFields(logrus.Fields{
		"function":       "handleFileDataAck",
		"file_id":        fileID,
		"bytes_received": bytesReceived,
	}).Debug("File data acknowledged by peer")

	return nil
}

// serializeFileRequest creates a file request packet payload.
func serializeFileRequest(fileID uint32, fileName string, fileSize uint64) []byte {
	// Format: [file_id (4 bytes)][file_size (8 bytes)][name_len (2 bytes)][file_name]
	nameBytes := []byte(fileName)
	data := make([]byte, 4+8+2+len(nameBytes))

	binary.BigEndian.PutUint32(data[0:4], fileID)
	binary.BigEndian.PutUint64(data[4:12], fileSize)
	binary.BigEndian.PutUint16(data[12:14], uint16(len(nameBytes)))
	copy(data[14:], nameBytes)

	return data
}

// deserializeFileRequest parses a file request packet payload.
func deserializeFileRequest(data []byte) (uint32, string, uint64, error) {
	if len(data) < 14 {
		return 0, "", 0, errors.New("file request packet too short")
	}

	fileID := binary.BigEndian.Uint32(data[0:4])
	fileSize := binary.BigEndian.Uint64(data[4:12])
	nameLen := binary.BigEndian.Uint16(data[12:14])

	if len(data) < 14+int(nameLen) {
		return 0, "", 0, errors.New("file request packet truncated")
	}

	fileName := string(data[14 : 14+nameLen])

	return fileID, fileName, fileSize, nil
}

// serializeFileData creates a file data packet payload.
func serializeFileData(fileID uint32, chunk []byte) []byte {
	// Format: [file_id (4 bytes)][chunk_data]
	data := make([]byte, 4+len(chunk))
	binary.BigEndian.PutUint32(data[0:4], fileID)
	copy(data[4:], chunk)
	return data
}

// deserializeFileData parses a file data packet payload.
func deserializeFileData(data []byte) (uint32, []byte, error) {
	if len(data) < 4 {
		return 0, nil, errors.New("file data packet too short")
	}

	fileID := binary.BigEndian.Uint32(data[0:4])
	chunk := make([]byte, len(data)-4)
	copy(chunk, data[4:])

	return fileID, chunk, nil
}

// serializeFileDataAck creates a file data acknowledgment packet payload.
func serializeFileDataAck(fileID uint32, bytesReceived uint64) []byte {
	// Format: [file_id (4 bytes)][bytes_received (8 bytes)]
	data := make([]byte, 12)
	binary.BigEndian.PutUint32(data[0:4], fileID)
	binary.BigEndian.PutUint64(data[4:12], bytesReceived)
	return data
}
