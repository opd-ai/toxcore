package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// NoisePacket represents a Noise protocol packet
//
//export ToxNoisePacket
type NoisePacket struct {
	PacketType      PacketType
	ProtocolVersion uint8
	SessionID       uint32
	Payload         []byte
}

// ProtocolNegotiationPacket handles protocol version negotiation
//
//export ToxProtocolNegotiationPacket
type ProtocolNegotiationPacket struct {
	Version      uint8
	MessageType  uint8  // 0=capabilities, 1=selection
	Capabilities []byte // Serialized capabilities
}

// HandshakePacket contains Noise handshake data
//
//export ToxHandshakePacket
type HandshakePacket struct {
	HandshakeType uint8 // 0=init, 1=response
	SessionID     uint32
	HandshakeData []byte
	Payload       []byte // Optional payload for 0-RTT
}

// SerializeNoisePacket converts a NoisePacket to bytes
//
//export ToxSerializeNoisePacket
func SerializeNoisePacket(packet *NoisePacket) ([]byte, error) {
	if packet.Payload == nil {
		return nil, errors.New("packet payload is nil")
	}

	// Format: [type(1)][version(1)][session_id(4)][payload_len(4)][payload]
	headerSize := 10
	result := make([]byte, headerSize+len(packet.Payload))

	result[0] = byte(packet.PacketType)
	result[1] = packet.ProtocolVersion
	binary.BigEndian.PutUint32(result[2:6], packet.SessionID)
	binary.BigEndian.PutUint32(result[6:10], uint32(len(packet.Payload)))
	copy(result[headerSize:], packet.Payload)

	return result, nil
}

// ParseNoisePacket converts bytes to a NoisePacket
//
//export ToxParseNoisePacket
func ParseNoisePacket(data []byte) (*NoisePacket, error) {
	if len(data) < 10 {
		return nil, errors.New("packet too short for noise packet")
	}

	packetType := PacketType(data[0])
	protocolVersion := data[1]
	sessionID := binary.BigEndian.Uint32(data[2:6])
	payloadLen := binary.BigEndian.Uint32(data[6:10])

	if len(data) < 10+int(payloadLen) {
		return nil, errors.New("packet payload truncated")
	}

	payload := make([]byte, payloadLen)
	copy(payload, data[10:10+payloadLen])

	return &NoisePacket{
		PacketType:      packetType,
		ProtocolVersion: protocolVersion,
		SessionID:       sessionID,
		Payload:         payload,
	}, nil
}

// SerializeHandshakePacket converts a HandshakePacket to bytes
//
//export ToxSerializeHandshakePacket
func SerializeHandshakePacket(packet *HandshakePacket) ([]byte, error) {
	// Format: [type(1)][session_id(4)][handshake_len(4)][handshake_data][payload_len(4)][payload]
	headerSize := 13
	totalSize := headerSize + len(packet.HandshakeData) + len(packet.Payload)
	result := make([]byte, totalSize)

	result[0] = packet.HandshakeType
	binary.BigEndian.PutUint32(result[1:5], packet.SessionID)
	binary.BigEndian.PutUint32(result[5:9], uint32(len(packet.HandshakeData)))

	offset := 9
	copy(result[offset:offset+len(packet.HandshakeData)], packet.HandshakeData)
	offset += len(packet.HandshakeData)

	binary.BigEndian.PutUint32(result[offset:offset+4], uint32(len(packet.Payload)))
	offset += 4
	copy(result[offset:], packet.Payload)

	return result, nil
}

// ParseHandshakePacket converts bytes to a HandshakePacket
//
//export ToxParseHandshakePacket
func ParseHandshakePacket(data []byte) (*HandshakePacket, error) {
	if len(data) < 13 {
		return nil, errors.New("packet too short for handshake packet")
	}

	handshakeType := data[0]
	sessionID := binary.BigEndian.Uint32(data[1:5])
	handshakeLen := binary.BigEndian.Uint32(data[5:9])

	if len(data) < 9+int(handshakeLen)+4 {
		return nil, errors.New("packet truncated")
	}

	handshakeData := make([]byte, handshakeLen)
	copy(handshakeData, data[9:9+handshakeLen])

	offset := 9 + int(handshakeLen)
	payloadLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	if len(data) < offset+int(payloadLen) {
		return nil, errors.New("packet payload truncated")
	}

	payload := make([]byte, payloadLen)
	copy(payload, data[offset:offset+int(payloadLen)])

	return &HandshakePacket{
		HandshakeType: handshakeType,
		SessionID:     sessionID,
		HandshakeData: handshakeData,
		Payload:       payload,
	}, nil
}

// IsNoisePacket checks if a packet is a Noise protocol packet
//
//export ToxIsNoisePacket
func IsNoisePacket(packetType PacketType) bool {
	return packetType >= 100 && packetType <= 110
}

// ConvertLegacyPacket wraps legacy packets for compatibility
//
//export ToxConvertLegacyPacket
func ConvertLegacyPacket(legacyPacket *Packet) *NoisePacket {
	return &NoisePacket{
		PacketType:      PacketType(legacyPacket.PacketType),
		ProtocolVersion: 1, // Legacy version
		SessionID:       0, // No session concept in legacy
		Payload:         legacyPacket.Data,
	}
}

// ConvertToLegacyPacket converts Noise packets back to legacy format
//
//export ToxConvertToLegacyPacket
func ConvertToLegacyPacket(noisePacket *NoisePacket) *Packet {
	return &Packet{
		PacketType: PacketType(noisePacket.PacketType),
		Data:       noisePacket.Payload,
	}
}

// SessionManager manages packet routing to sessions
//
//export ToxPacketSessionManager
type PacketSessionManager struct {
	activeSessions map[uint32]string // sessionID -> peerKey
	handlers       map[PacketType]NoisePacketHandler
}

// NoisePacketHandler processes Noise packets
type NoisePacketHandler func(packet *NoisePacket, sessionID uint32) error

// NewPacketSessionManager creates a new packet session manager
//
//export ToxNewPacketSessionManager
func NewPacketSessionManager() *PacketSessionManager {
	return &PacketSessionManager{
		activeSessions: make(map[uint32]string),
		handlers:       make(map[PacketType]NoisePacketHandler),
	}
}

// RegisterHandler registers a handler for a Noise packet type
//
//export ToxRegisterNoiseHandler
func (psm *PacketSessionManager) RegisterHandler(packetType PacketType, handler NoisePacketHandler) {
	psm.handlers[packetType] = handler
}

// RoutePacket routes a packet to the appropriate session handler
//
//export ToxRouteNoisePacket
func (psm *PacketSessionManager) RoutePacket(packet *NoisePacket) error {
	handler, exists := psm.handlers[packet.PacketType]
	if !exists {
		return fmt.Errorf("no handler registered for packet type %d", packet.PacketType)
	}

	return handler(packet, packet.SessionID)
}

// AddSession associates a session ID with a peer
//
//export ToxAddPacketSession
func (psm *PacketSessionManager) AddSession(sessionID uint32, peerKey string) {
	psm.activeSessions[sessionID] = peerKey
}

// RemoveSession removes a session association
//
//export ToxRemovePacketSession
func (psm *PacketSessionManager) RemoveSession(sessionID uint32) {
	delete(psm.activeSessions, sessionID)
}

// GetPeer returns the peer key for a session ID
//
//export ToxGetSessionPeer
func (psm *PacketSessionManager) GetPeer(sessionID uint32) (string, bool) {
	peerKey, exists := psm.activeSessions[sessionID]
	return peerKey, exists
}
