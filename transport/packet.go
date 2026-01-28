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

const (
	// DHT packet types
	PacketPingRequest PacketType = iota + 1
	PacketPingResponse
	PacketGetNodes
	PacketSendNodes

	// Friend related packet types
	PacketFriendRequest
	PacketLANDiscovery
	PacketFriendMessage
	PacketFriendMessageAck
	PacketFriendNameUpdate
	PacketFriendStatusMessageUpdate

	// Onion routing packet types
	PacketOnionSend
	PacketOnionReceive
	PacketOnionReply
	PacketOnionAnnounceRequest
	PacketOnionAnnounceResponse
	PacketOnionDataRequest
	PacketOnionDataResponse

	// File transfer packet types
	PacketFileRequest
	PacketFileControl
	PacketFileData
	PacketFileDataAck

	// Group chat packet types
	PacketGroupInvite
	PacketGroupInviteResponse
	PacketGroupBroadcast

	// Other packet types
	PacketOnet
	PacketDHTRequest

	// Async messaging packet types
	PacketAsyncStore
	PacketAsyncStoreResponse
	PacketAsyncRetrieve
	PacketAsyncRetrieveResponse
	PacketAsyncPreKeyExchange

	// ToxAV (audio/video calling) packet types
	PacketAVCallRequest
	PacketAVCallResponse
	PacketAVCallControl
	PacketAVAudioFrame
	PacketAVVideoFrame
	PacketAVBitrateControl

	// Noise Protocol Framework packet types
	PacketVersionNegotiation PacketType = 249
	PacketNoiseHandshake     PacketType = 250
	PacketNoiseMessage       PacketType = 251
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
