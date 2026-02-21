// Package transport implements the network transport layer for the Tox protocol.
//
// This package handles packet formatting, UDP and TCP communication, and NAT traversal.
//
// Example:
//
//	transport, err := transport.New(options)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	packet := &transport.Packet{
//	    PacketType: transport.PacketPingRequest,
//	    Data:       []byte{...},
//	}
//
//	err = transport.Send(packet, remoteAddr)
package transport

import (
	"errors"

	"github.com/sirupsen/logrus"
)

// PacketType identifies the type of a Tox packet.
type PacketType byte

// PacketType constants define all recognized Tox protocol packet types.
// Each category serves a specific protocol function.
const (
	// PacketPingRequest is a DHT ping request for node liveness checking.
	PacketPingRequest PacketType = iota + 1
	// PacketPingResponse is a DHT ping response confirming node availability.
	PacketPingResponse
	// PacketGetNodes requests DHT nodes from a peer.
	PacketGetNodes
	// PacketSendNodes responds with DHT nodes to a GetNodes request.
	PacketSendNodes

	// PacketFriendRequest initiates a friend request to another user.
	PacketFriendRequest
	// PacketLANDiscovery broadcasts for local network peer discovery.
	PacketLANDiscovery
	// PacketFriendMessage carries an encrypted message to a friend.
	PacketFriendMessage
	// PacketFriendMessageAck acknowledges receipt of a friend message.
	PacketFriendMessageAck
	// PacketFriendNameUpdate notifies a friend of a name change.
	PacketFriendNameUpdate
	// PacketFriendStatusMessageUpdate notifies a friend of a status message change.
	PacketFriendStatusMessageUpdate

	// PacketOnionSend sends data through the onion routing layer.
	PacketOnionSend
	// PacketOnionReceive receives data from the onion routing layer.
	PacketOnionReceive
	// PacketOnionReply is a reply through the onion routing layer.
	PacketOnionReply
	// PacketOnionAnnounceRequest requests announcement on the onion network.
	PacketOnionAnnounceRequest
	// PacketOnionAnnounceResponse responds to an onion announcement request.
	PacketOnionAnnounceResponse
	// PacketOnionDataRequest requests data via onion routing.
	PacketOnionDataRequest
	// PacketOnionDataResponse responds with data via onion routing.
	PacketOnionDataResponse

	// PacketFileRequest initiates a file transfer request.
	PacketFileRequest
	// PacketFileControl controls an ongoing file transfer.
	PacketFileControl
	// PacketFileData carries file data during transfer.
	PacketFileData
	// PacketFileDataAck acknowledges receipt of file data.
	PacketFileDataAck

	// PacketGroupInvite invites a user to join a group chat.
	PacketGroupInvite
	// PacketGroupInviteResponse responds to a group invitation.
	PacketGroupInviteResponse
	// PacketGroupBroadcast broadcasts a message to all group members.
	PacketGroupBroadcast
	// PacketGroupAnnounce announces group presence on the DHT.
	PacketGroupAnnounce
	// PacketGroupQuery queries for group information from the DHT.
	PacketGroupQuery
	// PacketGroupQueryResponse responds to a group query.
	PacketGroupQueryResponse

	// PacketOnet is reserved for overlay network extensions.
	PacketOnet
	// PacketDHTRequest is a generic DHT request packet.
	PacketDHTRequest

	// PacketAsyncStore stores a message for asynchronous delivery.
	PacketAsyncStore
	// PacketAsyncStoreResponse confirms async message storage.
	PacketAsyncStoreResponse
	// PacketAsyncRetrieve retrieves stored async messages.
	PacketAsyncRetrieve
	// PacketAsyncRetrieveResponse returns retrieved async messages.
	PacketAsyncRetrieveResponse
	// PacketAsyncPreKeyExchange exchanges pre-keys for forward secrecy.
	PacketAsyncPreKeyExchange

	// PacketAVCallRequest initiates an audio/video call.
	PacketAVCallRequest
	// PacketAVCallResponse responds to a call request.
	PacketAVCallResponse
	// PacketAVCallControl sends call control commands (mute, hold, end).
	PacketAVCallControl
	// PacketAVAudioFrame carries encoded audio data during a call.
	PacketAVAudioFrame
	// PacketAVVideoFrame carries encoded video data during a call.
	PacketAVVideoFrame
	// PacketAVBitrateControl adjusts media bitrate during a call.
	PacketAVBitrateControl

	// --- opd-ai Extension Packet Types ---
	// The following packet types (249-254) are opd-ai extensions not present in
	// c-toxcore. They use the reserved range 0xF9-0xFE per the Tox protocol spec.
	// Legacy c-toxcore clients will ignore these packet types.
	// See packet_extensions.go for the extension registry and compatibility notes.

	// PacketVersionNegotiation negotiates protocol version compatibility.
	// Extension type: opd-ai v0.1
	PacketVersionNegotiation PacketType = 249

	// PacketNoiseHandshake initiates or responds to a Noise protocol handshake.
	// Extension type: opd-ai v0.1
	PacketNoiseHandshake PacketType = 250

	// PacketNoiseMessage carries Noise-encrypted payload data.
	// Extension type: opd-ai v0.1
	PacketNoiseMessage PacketType = 251

	// PacketVersionCommitment confirms the mutually agreed protocol version
	// after Noise handshake completion to prevent version rollback attacks.
	// Extension type: opd-ai v0.1
	PacketVersionCommitment PacketType = 252
)

// Packet represents a Tox protocol packet.
//
//export ToxPacket
type Packet struct {
	PacketType PacketType
	Data       []byte
}

// Serialize converts a packet to a byte slice for transmission.
func (p *Packet) Serialize() ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":    "Serialize",
		"packet_type": p.PacketType,
		"data_size":   len(p.Data),
	}).Debug("Serializing packet for transmission")

	if p.Data == nil {
		logrus.WithFields(logrus.Fields{
			"function": "Serialize",
			"error":    "packet data is nil",
		}).Error("Packet serialization failed")
		return nil, errors.New("packet data is nil")
	}

	// Format: [packet type (1 byte)][data (variable length)]
	result := make([]byte, 1+len(p.Data))
	result[0] = byte(p.PacketType)
	copy(result[1:], p.Data)

	logrus.WithFields(logrus.Fields{
		"function":        "Serialize",
		"packet_type":     p.PacketType,
		"serialized_size": len(result),
	}).Debug("Packet serialized successfully")

	return result, nil
}

// ParsePacket converts a byte slice to a Packet structure.
//
//export ToxParsePacket
func ParsePacket(data []byte) (*Packet, error) {
	logrus.WithFields(logrus.Fields{
		"function":  "ParsePacket",
		"data_size": len(data),
	}).Debug("Parsing packet from byte slice")

	if len(data) < 1 {
		logrus.WithFields(logrus.Fields{
			"function":  "ParsePacket",
			"data_size": len(data),
			"error":     "packet too short",
		}).Error("Packet parsing failed")
		return nil, errors.New("packet too short")
	}

	packetType := PacketType(data[0])
	packet := &Packet{
		PacketType: packetType,
		Data:       make([]byte, len(data)-1),
	}

	copy(packet.Data, data[1:])

	logrus.WithFields(logrus.Fields{
		"function":    "ParsePacket",
		"packet_type": packetType,
		"data_size":   len(packet.Data),
	}).Debug("Packet parsed successfully")

	return packet, nil
}

// NodePacket is a specialized packet for DHT node communication.
type NodePacket struct {
	PublicKey [32]byte
	Nonce     [24]byte
	Payload   []byte
}

// Serialize converts a NodePacket to a byte slice.
func (np *NodePacket) Serialize() ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":     "Serialize",
		"public_key":   string(np.PublicKey[:8]),
		"payload_size": len(np.Payload),
	}).Debug("Serializing node packet")

	// Format: [public key (32 bytes)][nonce (24 bytes)][payload (variable)]
	result := make([]byte, 32+24+len(np.Payload))

	copy(result[0:32], np.PublicKey[:])
	copy(result[32:56], np.Nonce[:])
	copy(result[56:], np.Payload)

	logrus.WithFields(logrus.Fields{
		"function":        "Serialize",
		"serialized_size": len(result),
	}).Debug("Node packet serialized successfully")

	return result, nil
}

// ParseNodePacket converts a byte slice to a NodePacket structure.
func ParseNodePacket(data []byte) (*NodePacket, error) {
	logrus.WithFields(logrus.Fields{
		"function":  "ParseNodePacket",
		"data_size": len(data),
	}).Debug("Parsing node packet from byte slice")

	if len(data) < 56 { // 32 (pubkey) + 24 (nonce)
		logrus.WithFields(logrus.Fields{
			"function":     "ParseNodePacket",
			"data_size":    len(data),
			"min_required": 56,
			"error":        "node packet too short",
		}).Error("Node packet parsing failed")
		return nil, errors.New("node packet too short")
	}

	packet := &NodePacket{
		Payload: make([]byte, len(data)-56),
	}

	copy(packet.PublicKey[:], data[0:32])
	copy(packet.Nonce[:], data[32:56])
	copy(packet.Payload, data[56:])

	logrus.WithFields(logrus.Fields{
		"function":     "ParseNodePacket",
		"public_key":   string(packet.PublicKey[:8]),
		"payload_size": len(packet.Payload),
	}).Debug("Node packet parsed successfully")

	return packet, nil
}
