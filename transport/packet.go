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

	// Other packet types
	PacketOnet
	PacketDHTRequest

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
	if p.Data == nil {
		return nil, errors.New("packet data is nil")
	}

	// Format: [packet type (1 byte)][data (variable length)]
	result := make([]byte, 1+len(p.Data))
	result[0] = byte(p.PacketType)
	copy(result[1:], p.Data)

	return result, nil
}

// ParsePacket converts a byte slice to a Packet structure.
//
//export ToxParsePacket
func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < 1 {
		return nil, errors.New("packet too short")
	}

	packetType := PacketType(data[0])
	packet := &Packet{
		PacketType: packetType,
		Data:       make([]byte, len(data)-1),
	}

	copy(packet.Data, data[1:])

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
	// Format: [public key (32 bytes)][nonce (24 bytes)][payload (variable)]
	result := make([]byte, 32+24+len(np.Payload))

	copy(result[0:32], np.PublicKey[:])
	copy(result[32:56], np.Nonce[:])
	copy(result[56:], np.Payload)

	return result, nil
}

// ParseNodePacket converts a byte slice to a NodePacket structure.
func ParseNodePacket(data []byte) (*NodePacket, error) {
	if len(data) < 56 { // 32 (pubkey) + 24 (nonce)
		return nil, errors.New("node packet too short")
	}

	packet := &NodePacket{
		Payload: make([]byte, len(data)-56),
	}

	copy(packet.PublicKey[:], data[0:32])
	copy(packet.Nonce[:], data[32:56])
	copy(packet.Payload, data[56:])

	return packet, nil
}
