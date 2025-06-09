// ADDED: Package-level documentation for Noise protocol packet handling.
// This file implements packet structures and functions for the Noise protocol
// integration in Tox, providing enhanced security and forward secrecy.
//
// The Noise protocol implementation includes:
//   - Secure packet framing with session management
//   - Protocol version negotiation for compatibility
//   - Handshake packet handling for key exchange
//   - Legacy packet compatibility bridge
//   - Session-aware packet routing and management
//
// Key features:
//   - IK handshake pattern for known remote keys
//   - Forward secrecy through session key rotation
//   - Protocol capability negotiation
//   - Backward compatibility with legacy Tox packets
//
// Example usage:
//
//	// Create and serialize a Noise packet
//	packet := &NoisePacket{
//	    PacketType: PacketNoiseMessage,
//	    ProtocolVersion: 1,
//	    SessionID: sessionID,
//	    Payload: encryptedData,
//	}
//	data, _ := SerializeNoisePacket(packet)
//
//	// Parse incoming Noise packet
//	parsed, _ := ParseNoisePacket(receivedData)

package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ADDED: NoisePacket represents a Noise protocol packet with session management.
// This is the primary packet format for Noise protocol communications,
// providing encrypted messaging with forward secrecy and session tracking.
// Each packet is associated with a specific session and contains encrypted
// payload data that can only be decrypted by the intended recipient.
//
//export ToxNoisePacket
type NoisePacket struct {
	PacketType      PacketType // ADDED: Type of packet (message, handshake, etc.)
	ProtocolVersion uint8      // ADDED: Noise protocol version for compatibility
	SessionID       uint32     // ADDED: Unique session identifier for routing
	Payload         []byte     // ADDED: Encrypted packet payload data
}

// ADDED: ProtocolNegotiationPacket handles protocol version negotiation between peers.
// This packet type is used during the initial connection phase to establish
// which protocol versions and capabilities are supported by both parties.
// It enables graceful fallback to compatible protocol versions and features.
//
//export ToxProtocolNegotiationPacket
type ProtocolNegotiationPacket struct {
	Version      uint8  // ADDED: Protocol version being negotiated
	MessageType  uint8  // ADDED: 0=capabilities announcement, 1=selection response
	Capabilities []byte // ADDED: Serialized capabilities data (JSON or binary)
}

// ADDED: HandshakePacket contains Noise handshake data for secure key exchange.
// This packet type is used during the Noise-IK handshake process to establish
// secure communication channels between peers. It can optionally carry payload
// data for 0-RTT (zero round-trip time) message transmission during handshake.
//
//export ToxHandshakePacket
type HandshakePacket struct {
	HandshakeType uint8  // ADDED: 0=init (first message), 1=response (second message)
	SessionID     uint32 // ADDED: Session identifier for this handshake
	HandshakeData []byte // ADDED: Noise handshake state data (ephemeral keys, etc.)
	Payload       []byte // ADDED: Optional payload for 0-RTT messaging
}

// ADDED: SerializeNoisePacket converts a NoisePacket to bytes for network transmission.
// The packet is serialized using a structured binary format that includes
// packet type, protocol version, session ID, and payload length fields.
// This format ensures reliable parsing on the receiving end.
//
// Packet format: [type(1)][version(1)][session_id(4)][payload_len(4)][payload(variable)]
//
// Parameters:
//   - packet: The NoisePacket to serialize
//
// Returns the serialized packet bytes and any error encountered.
//
//export ToxSerializeNoisePacket
func SerializeNoisePacket(packet *NoisePacket) ([]byte, error) {
	if packet.Payload == nil {
		return nil, errors.New("packet payload is nil")
	}

	// ADDED: Format: [type(1)][version(1)][session_id(4)][payload_len(4)][payload]
	headerSize := 10
	result := make([]byte, headerSize+len(packet.Payload))

	result[0] = byte(packet.PacketType)
	result[1] = packet.ProtocolVersion
	binary.BigEndian.PutUint32(result[2:6], packet.SessionID)
	binary.BigEndian.PutUint32(result[6:10], uint32(len(packet.Payload)))
	copy(result[headerSize:], packet.Payload)

	return result, nil
}

// ADDED: ParseNoisePacket converts bytes to a NoisePacket structure.
// This function parses network data according to the Noise packet format,
// extracting packet type, protocol version, session ID, and payload.
// It validates packet structure and length before creating the packet object.
//
// Parameters:
//   - data: Raw packet bytes received from network
//
// Returns the parsed NoisePacket and any error encountered during parsing.
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

// ADDED: SerializeHandshakePacket converts a HandshakePacket to bytes for network transmission.
// This function serializes Noise handshake packets using a structured binary format
// that includes handshake type, session ID, handshake data length, handshake data,
// payload length, and optional payload data. This format supports 0-RTT messaging
// during the handshake process.
//
// Packet format: [type(1)][session_id(4)][handshake_len(4)][handshake_data(variable)][payload_len(4)][payload(variable)]
//
// Parameters:
//   - packet: The HandshakePacket to serialize
//
// Returns the serialized packet bytes and any error encountered.
//
//export ToxSerializeHandshakePacket
func SerializeHandshakePacket(packet *HandshakePacket) ([]byte, error) {
	// ADDED: Format: [type(1)][session_id(4)][handshake_len(4)][handshake_data][payload_len(4)][payload]
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

// ADDED: ParseHandshakePacket converts bytes to a HandshakePacket structure.
// This function parses network data according to the Noise handshake packet format,
// extracting handshake type, session ID, handshake data, and optional payload.
// It validates packet structure and all length fields before creating the packet object.
//
// Parameters:
//   - data: Raw packet bytes received from network
//
// Returns the parsed HandshakePacket and any error encountered during parsing.
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

// ADDED: IsNoisePacket checks if a packet type represents a Noise protocol packet.
// This function determines whether a given PacketType belongs to the Noise protocol
// packet range (100-110), which is reserved for Noise protocol communications.
// Used for packet routing and protocol detection.
//
// Parameters:
//   - packetType: The PacketType to check
//
// Returns true if the packet type is within the Noise protocol range.
//
//export ToxIsNoisePacket
func IsNoisePacket(packetType PacketType) bool {
	return packetType >= 100 && packetType <= 110 // ADDED: Noise protocol packet range
}

// ADDED: ConvertLegacyPacket wraps legacy packets for Noise protocol compatibility.
// This function creates a NoisePacket wrapper around legacy Tox packets to enable
// seamless integration with Noise protocol infrastructure. Legacy packets are
// assigned protocol version 1 and session ID 0 since they don't support sessions.
//
// Parameters:
//   - legacyPacket: The legacy Packet to wrap
//
// Returns a NoisePacket wrapper with compatibility settings.
//
//export ToxConvertLegacyPacket
func ConvertLegacyPacket(legacyPacket *Packet) *NoisePacket {
	return &NoisePacket{
		PacketType:      PacketType(legacyPacket.PacketType),
		ProtocolVersion: 1, // ADDED: Legacy version for compatibility
		SessionID:       0, // ADDED: No session concept in legacy protocol
		Payload:         legacyPacket.Data,
	}
}

// ADDED: ConvertToLegacyPacket converts Noise packets back to legacy format.
// This function extracts the essential packet data from a NoisePacket and creates
// a legacy Packet structure for backward compatibility. Session and protocol
// version information is discarded as legacy packets don't support these features.
//
// Parameters:
//   - noisePacket: The NoisePacket to convert
//
// Returns a legacy Packet with the core packet data.
//
//export ToxConvertToLegacyPacket
func ConvertToLegacyPacket(noisePacket *NoisePacket) *Packet {
	return &Packet{
		PacketType: PacketType(noisePacket.PacketType),
		Data:       noisePacket.Payload, // ADDED: Extract payload as packet data
	}
}

// ADDED: PacketSessionManager manages packet routing to sessions and handlers.
// This structure provides centralized management of active Noise protocol sessions
// and packet routing. It maintains mappings between session IDs and peer keys,
// as well as packet type handlers for different kinds of Noise packets.
// Thread-safe operations should be implemented by callers using appropriate locking.
//
//export ToxPacketSessionManager
type PacketSessionManager struct {
	activeSessions map[uint32]string                 // ADDED: sessionID -> peerKey mapping
	handlers       map[PacketType]NoisePacketHandler // ADDED: packetType -> handler mapping
}

// ADDED: NoisePacketHandler is a function type that processes Noise packets.
// Handlers are responsible for processing specific packet types within the context
// of a session. They receive the packet and session ID for context-aware processing.
//
// Parameters:
//   - packet: The NoisePacket to process
//   - sessionID: The session ID for context
//
// Returns an error if packet processing fails.
type NoisePacketHandler func(packet *NoisePacket, sessionID uint32) error

// ADDED: NewPacketSessionManager creates a new packet session manager instance.
// This function initializes a PacketSessionManager with empty session and handler
// maps. The returned manager is ready to register handlers and manage sessions
// for Noise protocol packet routing.
//
// Returns a new PacketSessionManager with initialized internal maps.
//
//export ToxNewPacketSessionManager
func NewPacketSessionManager() *PacketSessionManager {
	return &PacketSessionManager{
		activeSessions: make(map[uint32]string),                 // ADDED: Initialize session map
		handlers:       make(map[PacketType]NoisePacketHandler), // ADDED: Initialize handler map
	}
}

// ADDED: RegisterHandler registers a handler function for a specific Noise packet type.
// This method associates a packet processing function with a particular PacketType,
// enabling automatic routing of incoming packets to appropriate handlers.
// Multiple handlers can be registered for different packet types.
//
// Parameters:
//   - packetType: The PacketType to handle
//   - handler: The NoisePacketHandler function to process packets of this type
//
//export ToxRegisterNoiseHandler
func (psm *PacketSessionManager) RegisterHandler(packetType PacketType, handler NoisePacketHandler) {
	psm.handlers[packetType] = handler // ADDED: Store handler for packet type
}

// ADDED: RoutePacket routes a Noise packet to the appropriate registered handler.
// This method looks up the handler for the packet's type and invokes it with
// the packet and session ID. Returns an error if no handler is registered
// for the packet type or if the handler fails to process the packet.
//
// Parameters:
//   - packet: The NoisePacket to route
//
// Returns an error if no handler exists or if packet processing fails.
//
//export ToxRouteNoisePacket
func (psm *PacketSessionManager) RoutePacket(packet *NoisePacket) error {
	handler, exists := psm.handlers[packet.PacketType]
	if !exists {
		// ADDED: Return error if no handler registered for this packet type
		return fmt.Errorf("no handler registered for packet type %d", packet.PacketType)
	}

	// ADDED: Invoke handler with packet and session context
	return handler(packet, packet.SessionID)
}

// ADDED: AddSession associates a session ID with a peer key for routing context.
// This method creates a mapping between a session ID and the peer's public key,
// enabling session-aware packet processing and peer identification.
// Used during session establishment to track active sessions.
//
// Parameters:
//   - sessionID: The unique session identifier
//   - peerKey: The peer's public key or identifier
//
//export ToxAddPacketSession
func (psm *PacketSessionManager) AddSession(sessionID uint32, peerKey string) {
	psm.activeSessions[sessionID] = peerKey // ADDED: Store session-to-peer mapping
}

// ADDED: RemoveSession removes a session association and cleans up resources.
// This method deletes the mapping between a session ID and peer key,
// effectively terminating the session tracking. Used during session cleanup
// when connections are closed or sessions expire.
//
// Parameters:
//   - sessionID: The session identifier to remove
//
//export ToxRemovePacketSession
func (psm *PacketSessionManager) RemoveSession(sessionID uint32) {
	delete(psm.activeSessions, sessionID) // ADDED: Remove session mapping
}

// ADDED: GetPeer returns the peer key associated with a session ID.
// This method looks up the peer key for a given session ID, enabling
// session context retrieval for packet processing. Used to identify
// which peer a session belongs to.
//
// Parameters:
//   - sessionID: The session identifier to look up
//
// Returns the peer key and a boolean indicating if the session exists.
//
//export ToxGetSessionPeer
func (psm *PacketSessionManager) GetPeer(sessionID uint32) (string, bool) {
	peerKey, exists := psm.activeSessions[sessionID] // ADDED: Look up peer for session
	return peerKey, exists
}
