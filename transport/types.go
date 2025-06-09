// Package transport implements network transport layers for the Tox protocol.
// This file defines core interfaces and types used throughout the transport layer,
// providing abstractions for different transport implementations and packet handling.
//
// Key interfaces and types:
//   - Transport: Core interface for network transport implementations
//   - PacketHandler: Function type for processing incoming packets
//   - Network abstraction using Go's standard net interfaces
//
// The transport layer supports multiple transport protocols (UDP, TCP) through
// a common interface, enabling flexible network communication patterns while
// maintaining protocol compatibility and performance.

package transport

import (
	"net"
)

// ADDED: PacketHandler is a function type that processes incoming packets.
// Handlers are responsible for processing specific packet types and are
// called concurrently for each received packet. They receive the packet
// data and the source address for context-aware processing.
//
// Parameters:
//   - packet: The parsed Packet received from the network
//   - addr: The network address of the packet sender
//
// Returns an error if packet processing fails.
type PacketHandler func(packet *Packet, addr net.Addr) error

// ADDED: Transport defines the interface for network transports used by Tox.
// This abstraction allows for different transport implementations (UDP, TCP)
// to be used interchangeably throughout the codebase. All transport implementations
// must provide packet sending, handler registration, address information, and
// proper resource cleanup capabilities.
//
// The interface supports both connectionless (UDP) and connection-oriented (TCP)
// transport protocols while maintaining a consistent API for upper layers.
//
//export ToxTransport
type Transport interface {
	// ADDED: Send transmits a packet to the specified network address.
	// Implementations should handle serialization and transmission details.
	Send(packet *Packet, addr net.Addr) error

	// ADDED: Close shuts down the transport and releases all resources.
	// After calling Close, the transport should not be used further.
	Close() error

	// ADDED: LocalAddr returns the local address the transport is listening on.
	// This may differ from the requested address in some cases.
	LocalAddr() net.Addr

	// ADDED: RegisterHandler associates a handler function with a packet type.
	// Enables automatic routing of incoming packets to appropriate processors.
	RegisterHandler(packetType PacketType, handler PacketHandler)
}
