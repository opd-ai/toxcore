package interfaces

import "net"

// IPacketDelivery defines the interface for packet delivery operations.
// This abstraction allows switching between simulation and real network implementations.
type IPacketDelivery interface {
	// DeliverPacket delivers a packet to a specific friend
	DeliverPacket(friendID uint32, packet []byte) error

	// BroadcastPacket broadcasts a packet to all connected friends
	BroadcastPacket(packet []byte, excludeFriends []uint32) error

	// SetNetworkTransport sets the underlying network transport
	SetNetworkTransport(transport INetworkTransport) error

	// IsSimulation returns true if this is a simulation implementation
	IsSimulation() bool
}

// INetworkTransport extends the transport interface with network-specific operations
type INetworkTransport interface {
	// Send sends a packet to the specified network address
	Send(packet []byte, addr net.Addr) error

	// SendToFriend sends a packet specifically to a friend using their address
	SendToFriend(friendID uint32, packet []byte) error

	// GetFriendAddress returns the network address for a friend
	GetFriendAddress(friendID uint32) (net.Addr, error)

	// RegisterFriend registers a friend's network address
	RegisterFriend(friendID uint32, addr net.Addr) error

	// Close shuts down the transport
	Close() error

	// IsConnected returns true if the transport is connected to the network
	IsConnected() bool
}

// PacketDeliveryConfig holds configuration for packet delivery implementations
type PacketDeliveryConfig struct {
	// UseSimulation determines whether to use simulation or real network
	UseSimulation bool

	// NetworkTimeout sets the timeout for network operations
	NetworkTimeout int

	// RetryAttempts sets the number of retry attempts for failed deliveries
	RetryAttempts int

	// EnableBroadcast enables broadcast functionality
	EnableBroadcast bool
}
