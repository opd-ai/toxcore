package net

import (
	"context"
	"net"
	"time"

	"github.com/opd-ai/toxforge"
)

// Dial connects to a Tox address and returns a net.Conn.
// The toxID should be a 76-character hexadecimal Tox ID string.
func Dial(toxID string, tox *toxcore.Tox) (net.Conn, error) {
	return DialTimeout(toxID, tox, 0)
}

// DialTimeout connects to a Tox address with a timeout and returns a net.Conn.
// If timeout is 0, no timeout is applied.
func DialTimeout(toxID string, tox *toxcore.Tox, timeout time.Duration) (net.Conn, error) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return DialContext(ctx, toxID, tox)
}

// DialContext connects to a Tox address with a context and returns a net.Conn.
func DialContext(ctx context.Context, toxID string, tox *toxcore.Tox) (net.Conn, error) {
	// Parse the remote address
	remoteAddr, err := NewToxAddr(toxID)
	if err != nil {
		return nil, err
	}

	// Create local address
	localPublicKey := tox.SelfGetPublicKey()
	localNospam := tox.SelfGetNospam()
	localAddr := NewToxAddrFromPublicKey(localPublicKey, localNospam)

	// Check if we're already friends with this Tox ID
	friends := tox.GetFriends()
	var friendID uint32
	var found bool

	for id, friend := range friends {
		if friend.PublicKey == remoteAddr.PublicKey() {
			friendID = id
			found = true
			break
		}
	}

	if !found {
		// Send friend request
		friendID, err = tox.AddFriend(toxID, "Connection request from Tox networking layer")
		if err != nil {
			return nil, &ToxNetError{
				Op:   "dial",
				Addr: toxID,
				Err:  err,
			}
		}
	}

	// Create connection
	conn := newToxConn(tox, friendID, localAddr, remoteAddr)

	// Wait for connection to establish
	if err := waitForConnection(ctx, conn); err != nil {
		conn.Close()
		return nil, &ToxNetError{
			Op:   "dial",
			Addr: toxID,
			Err:  err,
		}
	}

	return conn, nil
}

// waitForConnection waits for a ToxConn to establish connection
func waitForConnection(ctx context.Context, conn *ToxConn) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if conn.IsConnected() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Continue checking
		}
	}
}

// Listen creates a Tox listener that accepts incoming connections.
// The listener automatically accepts friend requests and creates connections.
func Listen(tox *toxcore.Tox) (net.Listener, error) {
	return ListenConfig(tox, true)
}

// ListenConfig creates a Tox listener with configuration options.
// If autoAccept is true, friend requests are automatically accepted.
// If autoAccept is false, you must manually accept friend requests via the Tox instance.
func ListenConfig(tox *toxcore.Tox, autoAccept bool) (net.Listener, error) {
	listener := newToxListener(tox, autoAccept)
	return listener, nil
}

// ListenAddr is an alias for Listen that matches net package conventions.
// The addr parameter is ignored since Tox listeners use the Tox instance's address.
func ListenAddr(addr string, tox *toxcore.Tox) (net.Listener, error) {
	return Listen(tox)
}

// LookupToxAddr looks up a Tox address. Since Tox addresses are direct
// identifiers, this is equivalent to ResolveToxAddr.
func LookupToxAddr(address string) (*ToxAddr, error) {
	return ResolveToxAddr(address)
}

// DialTox is an alias for Dial that follows Go naming conventions.
func DialTox(toxID string, tox *toxcore.Tox) (net.Conn, error) {
	return Dial(toxID, tox)
}

// ListenTox is an alias for Listen that follows Go naming conventions.
func ListenTox(tox *toxcore.Tox) (net.Listener, error) {
	return Listen(tox)
}

// PacketDial creates a packet-based connection to a remote Tox address.
// The network parameter should be "tox" for Tox packet connections.
// The address parameter should be a Tox ID string or local address for binding.
// Returns a net.PacketConn that can be used for packet-based communication.
func PacketDial(network, address string) (net.PacketConn, error) {
	if network != "tox" {
		return nil, &ToxNetError{
			Op:   "dial",
			Addr: address,
			Err:  net.UnknownNetworkError(network),
		}
	}

	// Parse the address as a Tox ID
	toxAddr, err := NewToxAddr(address)
	if err != nil {
		return nil, &ToxNetError{
			Op:   "dial",
			Addr: address,
			Err:  err,
		}
	}

	// Create a packet connection - use any available UDP port for outgoing
	conn, err := NewToxPacketConn(toxAddr, ":0")
	if err != nil {
		return nil, &ToxNetError{
			Op:   "dial",
			Addr: address,
			Err:  err,
		}
	}

	return conn, nil
}

// PacketListen creates a packet-based listener for incoming Tox connections.
// The network parameter should be "tox" for Tox packet listeners.
// The address parameter should be a local UDP address (e.g., ":8080") for binding.
// Returns a net.Listener that accepts packet-based connections.
func PacketListen(network, address string) (net.Listener, error) {
	if network != "tox" {
		return nil, &ToxNetError{
			Op:   "listen",
			Addr: address,
			Err:  net.UnknownNetworkError(network),
		}
	}

	// Generate a new Tox address for this listener
	// In a real implementation, this would use the actual Tox instance
	localAddr := &ToxAddr{
		toxID: nil, // This would be set from a real Tox instance
	}

	// Create a packet listener
	listener, err := NewToxPacketListener(localAddr, address)
	if err != nil {
		return nil, &ToxNetError{
			Op:   "listen",
			Addr: address,
			Err:  err,
		}
	}

	return listener, nil
}
