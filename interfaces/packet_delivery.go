package interfaces

import (
	"errors"
	"net"
)

// ErrInvalidTimeout is returned when NetworkTimeout is not positive.
var ErrInvalidTimeout = errors.New("network timeout must be positive")

// ErrInvalidRetryAttempts is returned when RetryAttempts is negative.
var ErrInvalidRetryAttempts = errors.New("retry attempts cannot be negative")

// IPacketDelivery defines the interface for packet delivery operations.
// This abstraction allows switching between simulation and real network implementations.
//
// All methods are safe for concurrent use from multiple goroutines.
// Implementations must handle their own synchronization.
type IPacketDelivery interface {
	// DeliverPacket delivers a packet to a specific friend.
	//
	// Returns an error if the friend is not registered, the transport is not
	// available, or the delivery times out after configured retry attempts.
	// The packet data is not modified by this method.
	DeliverPacket(friendID uint32, packet []byte) error

	// BroadcastPacket broadcasts a packet to all connected friends except
	// those listed in excludeFriends.
	//
	// Returns an error if any individual delivery fails. Partial delivery
	// may occur - some friends may receive the packet even if others fail.
	// The excludeFriends slice may be nil to broadcast to all friends.
	BroadcastPacket(packet []byte, excludeFriends []uint32) error

	// SetNetworkTransport sets the underlying network transport.
	//
	// This may be called to switch transports at runtime. If a transport
	// was previously set, it should be closed before setting a new one.
	// Returns an error if the transport is nil or invalid.
	SetNetworkTransport(transport INetworkTransport) error

	// IsSimulation returns true if this is a simulation implementation.
	//
	// Simulation implementations are used for testing and do not perform
	// actual network operations.
	IsSimulation() bool
}

// INetworkTransport extends the transport interface with network-specific operations.
//
// Implementations must be safe for concurrent use from multiple goroutines.
// All address parameters use net.Addr interface, never concrete types.
type INetworkTransport interface {
	// Send sends a packet to the specified network address.
	//
	// Returns an error if the address is invalid, the transport is closed,
	// or a network error occurs during transmission.
	Send(packet []byte, addr net.Addr) error

	// SendToFriend sends a packet specifically to a friend using their
	// registered address.
	//
	// Returns an error if the friend is not registered or network send fails.
	SendToFriend(friendID uint32, packet []byte) error

	// GetFriendAddress returns the network address for a friend.
	//
	// Returns an error if the friend is not registered with this transport.
	GetFriendAddress(friendID uint32) (net.Addr, error)

	// RegisterFriend registers a friend's network address.
	//
	// If the friend was previously registered, their address is updated.
	// Returns an error if the address is nil or invalid.
	RegisterFriend(friendID uint32, addr net.Addr) error

	// Close shuts down the transport and releases resources.
	//
	// After Close is called, all other methods will return errors.
	// Close is idempotent and safe to call multiple times.
	Close() error

	// IsConnected returns true if the transport is connected to the network.
	//
	// This indicates whether the transport is ready to send and receive packets.
	IsConnected() bool
}

// PacketDeliveryConfig holds configuration for packet delivery implementations.
type PacketDeliveryConfig struct {
	// UseSimulation determines whether to use simulation or real network.
	// When true, packets are not sent over actual network connections.
	UseSimulation bool

	// NetworkTimeout sets the timeout for network operations in milliseconds.
	// Must be positive. Default is typically 5000ms.
	NetworkTimeout int

	// RetryAttempts sets the number of retry attempts for failed deliveries.
	// Must be non-negative. Zero means no retries. Default is typically 3.
	RetryAttempts int

	// EnableBroadcast enables broadcast functionality.
	// When false, BroadcastPacket will return an error.
	EnableBroadcast bool
}

// Validate checks that the configuration values are within acceptable bounds.
//
// Returns ErrInvalidTimeout if NetworkTimeout is not positive.
// Returns ErrInvalidRetryAttempts if RetryAttempts is negative.
func (c *PacketDeliveryConfig) Validate() error {
	if c.NetworkTimeout <= 0 {
		return ErrInvalidTimeout
	}
	if c.RetryAttempts < 0 {
		return ErrInvalidRetryAttempts
	}
	return nil
}
