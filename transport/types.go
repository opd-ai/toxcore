package transport

import (
	"net"
)

// PacketHandler is a function that processes incoming packets.
type PacketHandler func(packet *Packet, addr net.Addr) error

// Transport defines the interface for network transports used by Tox.
// This abstraction allows for different transport implementations (UDP, TCP)
// to be used interchangeably throughout the codebase.
type Transport interface {
	// Send sends a packet to the specified address.
	Send(packet *Packet, addr net.Addr) error

	// Close shuts down the transport.
	Close() error

	// LocalAddr returns the local address the transport is listening on.
	LocalAddr() net.Addr

	// RegisterHandler registers a handler for a specific packet type.
	RegisterHandler(packetType PacketType, handler PacketHandler)
}
