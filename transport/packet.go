// Package transport implements network transport layers for the Tox protocol.
// This file defines packet structures, types, and serialization functions
// for the Tox protocol communication layer.
//
// The packet system provides:
//   - Strongly-typed packet identification using PacketType constants
//   - Binary serialization and parsing for network transmission
//   - Support for both legacy and Noise protocol packet types
//   - Specialized packets for DHT node communication
//   - Efficient zero-copy operations where possible
//
// Packet types are organized by functional areas:
//   - DHT operations (ping, get_nodes, send_nodes)
//   - Friend communication (requests, messages, acknowledgments)
//   - Onion routing for privacy and NAT traversal
//   - File transfer with chunking and flow control
//   - Noise protocol for enhanced security
//
// Example usage:
//
//	// Create and send a ping packet
//	packet := &Packet{
//	    PacketType: PacketPingRequest,
//	    Data:       []byte("ping_data"),
//	}
//	data, _ := packet.Serialize()
//	transport.Send(packet, remoteAddr)
//
//	// Parse received packet
//	received, _ := ParsePacket(networkData)
//	switch received.PacketType {
//	case PacketPingResponse:
//	    // Handle ping response
//	}

package transport

import (
	"errors"
)

// ADDED: PacketType identifies the type of a Tox protocol packet.
// This enumeration provides strongly-typed packet identification for
// routing and processing. Packet types are organized by functional
// areas and maintain backward compatibility with the Tox protocol.
type PacketType byte

const (
	// ADDED: DHT packet types for distributed hash table operations
	PacketPingRequest  PacketType = iota + 1 // DHT ping request
	PacketPingResponse                       // DHT ping response
	PacketGetNodes                           // Request for nearby nodes
	PacketSendNodes                          // Response with node list

	// ADDED: Friend-related packet types for peer communication
	PacketFriendRequest    // Friend request message
	PacketLANDiscovery     // Local area network discovery
	PacketFriendMessage    // Direct friend message
	PacketFriendMessageAck // Message acknowledgment

	// ADDED: Onion routing packet types for privacy and NAT traversal
	PacketOnionSend             // Send data through onion routing
	PacketOnionReceive          // Receive data from onion route
	PacketOnionReply            // Reply through onion route
	PacketOnionAnnounceRequest  // Announce presence request
	PacketOnionAnnounceResponse // Announce presence response
	PacketOnionDataRequest      // Request data through onion
	PacketOnionDataResponse     // Respond with data through onion

	// ADDED: File transfer packet types for reliable file transmission
	PacketFileRequest // File transfer request
	PacketFileControl // File transfer control (pause/resume/cancel)
	PacketFileData    // File transfer data chunk
	PacketFileDataAck // File transfer data acknowledgment

	// ADDED: Other legacy packet types for compatibility
	PacketOnet       // Legacy one-time packet
	PacketDHTRequest // Generic DHT request

	// ADDED: Noise protocol packet types (starting at 100 for compatibility)
	// These provide enhanced security and forward secrecy
	PacketNoiseHandshakeInit   PacketType = 100 // Noise handshake initiation
	PacketNoiseHandshakeResp                    // Noise handshake response
	PacketNoiseMessage                          // Encrypted Noise message
	PacketNoiseRekey                            // Session key rotation
	PacketProtocolCapabilities                  // Protocol capability negotiation
	PacketProtocolSelection                     // Protocol version selection
	PacketFriendMessageNoise                    // Friend message via Noise protocol
)

// ADDED: Packet represents a Tox protocol packet with type and data payload.
// This is the fundamental unit of communication in the Tox protocol,
// containing a packet type identifier and variable-length data payload.
// The structure supports both legacy and modern Noise protocol packets.
//
//export ToxPacket
type Packet struct {
	PacketType PacketType // ADDED: Identifies the packet type for routing
	Data       []byte     // ADDED: Variable-length packet payload data
}

// ADDED: Serialize converts a packet to a byte slice for network transmission.
// This method creates a binary representation of the packet suitable for
// sending over network transports. The format includes a single byte
// packet type followed by the variable-length data payload.
//
// Packet format: [packet_type(1)][data(variable)]
//
// Returns the serialized packet bytes and any error encountered.
//
//export ToxSerializePacket
func (p *Packet) Serialize() ([]byte, error) {
	if p.Data == nil {
		return nil, errors.New("packet data is nil") // ADDED: Validate payload
	}

	// ADDED: Create buffer with packet type and data
	result := make([]byte, 1+len(p.Data))
	result[0] = byte(p.PacketType)
	copy(result[1:], p.Data)

	return result, nil
}

// ADDED: ParsePacket converts a byte slice to a Packet structure.
// This function parses network data according to the Tox packet format,
// extracting the packet type and payload data. It validates the minimum
// packet size and creates a new Packet instance with copied data.
//
// Parameters:
//   - data: Raw packet bytes received from network
//
// Returns the parsed Packet and any error encountered during parsing.
//
//export ToxParsePacket
func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < 1 {
		return nil, errors.New("packet too short") // ADDED: Validate minimum size
	}

	packetType := PacketType(data[0])
	packet := &Packet{
		PacketType: packetType,
		Data:       make([]byte, len(data)-1), // ADDED: Allocate data buffer
	}

	copy(packet.Data, data[1:]) // ADDED: Copy payload data

	return packet, nil
}

// ADDED: NodePacket is a specialized packet for DHT node communication.
// This packet type is used for encrypted DHT operations where the payload
// needs to be associated with a specific node's public key and nonce.
// It provides cryptographic context for secure DHT communications.
//
//export ToxNodePacket
type NodePacket struct {
	PublicKey [32]byte // ADDED: Node's public key for identification
	Nonce     [24]byte // ADDED: Cryptographic nonce for encryption
	Payload   []byte   // ADDED: Encrypted packet payload
}

// ADDED: Serialize converts a NodePacket to a byte slice for transmission.
// This method creates a binary representation of the node packet with
// fixed-size fields for public key and nonce, followed by variable-length
// encrypted payload data.
//
// Packet format: [public_key(32)][nonce(24)][payload(variable)]
//
// Returns the serialized packet bytes and any error encountered.
//
//export ToxSerializeNodePacket
func (np *NodePacket) Serialize() ([]byte, error) {
	// ADDED: Create buffer for fixed fields plus payload
	result := make([]byte, 32+24+len(np.Payload))

	copy(result[0:32], np.PublicKey[:]) // ADDED: Copy public key
	copy(result[32:56], np.Nonce[:])    // ADDED: Copy nonce
	copy(result[56:], np.Payload)       // ADDED: Copy payload

	return result, nil
}

// ADDED: ParseNodePacket converts a byte slice to a NodePacket structure.
// This function parses network data according to the DHT node packet format,
// extracting the public key, nonce, and encrypted payload. It validates
// the minimum packet size before parsing the fixed-size fields.
//
// Parameters:
//   - data: Raw packet bytes received from network
//
// Returns the parsed NodePacket and any error encountered during parsing.
//
//export ToxParseNodePacket
func ParseNodePacket(data []byte) (*NodePacket, error) {
	if len(data) < 56 { // ADDED: Validate minimum size (32 + 24)
		return nil, errors.New("node packet too short")
	}

	packet := &NodePacket{
		Payload: make([]byte, len(data)-56), // ADDED: Allocate payload buffer
	}

	copy(packet.PublicKey[:], data[0:32]) // ADDED: Extract public key
	copy(packet.Nonce[:], data[32:56])    // ADDED: Extract nonce
	copy(packet.Payload, data[56:])       // ADDED: Extract payload

	return packet, nil
}
