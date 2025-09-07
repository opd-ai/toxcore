package transport

import (
	"net"
)

// NetworkTransport defines the interface for network transport implementations.
// This interface supports multiple network types including IP, Tor, I2P, Nym, and Loki.
// Each transport implementation handles the specifics of its network type while
// providing a unified interface for the toxcore protocol layer.
type NetworkTransport interface {
	// Listen creates a listener on the given address for the transport's network type.
	// The address format depends on the network type:
	// - IP networks: "host:port" (e.g., "127.0.0.1:8080" or ":8080")
	// - Tor: ".onion:port" (e.g., "3g2upl4pq6kufc4m.onion:80")
	// - I2P: ".i2p:port" (e.g., "example.b32.i2p:80")
	// - Nym: ".nym:port" (e.g., "example.nym:80")
	// - Loki: ".loki:port" (e.g., "example.loki:80")
	Listen(address string) (net.Listener, error)

	// Dial establishes a connection to the given address for stream-oriented protocols.
	// This is primarily used for TCP-like connections.
	Dial(address string) (net.Conn, error)

	// DialPacket creates a packet connection for datagram-oriented protocols.
	// This is primarily used for UDP-like connections.
	DialPacket(address string) (net.PacketConn, error)

	// SupportedNetworks returns the list of network types this transport handles.
	// Examples: ["tcp", "udp"] for IP, ["tor"] for Tor, ["i2p"] for I2P, etc.
	SupportedNetworks() []string

	// Close shuts down the transport and releases all resources.
	Close() error
}
