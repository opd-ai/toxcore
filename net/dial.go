package net

import (
	"context"
	"net"
	"time"

	"github.com/opd-ai/toxcore"
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

// checkContextDone checks if context is done and returns appropriate error.
func checkContextDone(ctx context.Context, toxID string) error {
	select {
	case <-ctx.Done():
		return &ToxNetError{
			Op:   "dial",
			Addr: toxID,
			Err:  ctx.Err(),
		}
	default:
		return nil
	}
}

// findExistingFriend searches for an existing friend by public key.
func findExistingFriend(tox *toxcore.Tox, remoteAddr *ToxAddr) (uint32, bool) {
	friends := tox.GetFriends()
	for id, friend := range friends {
		if friend.PublicKey == remoteAddr.PublicKey() {
			return id, true
		}
	}
	return 0, false
}

// addFriendWithContext adds a friend with context timeout support.
func addFriendWithContext(ctx context.Context, tox *toxcore.Tox, toxID string) (uint32, error) {
	type addResult struct {
		friendID uint32
		err      error
	}
	resultCh := make(chan addResult, 1)

	go func() {
		fid, ferr := tox.AddFriend(toxID, "Connection request from Tox networking layer")
		resultCh <- addResult{fid, ferr}
	}()

	select {
	case <-ctx.Done():
		return 0, &ToxNetError{
			Op:   "dial",
			Addr: toxID,
			Err:  ctx.Err(),
		}
	case result := <-resultCh:
		if result.err != nil {
			return 0, &ToxNetError{
				Op:   "dial",
				Addr: toxID,
				Err:  result.err,
			}
		}
		return result.friendID, nil
	}
}

// DialContext connects to a Tox address with a context and returns a net.Conn.
func DialContext(ctx context.Context, toxID string, tox *toxcore.Tox) (net.Conn, error) {
	if err := checkContextDone(ctx, toxID); err != nil {
		return nil, err
	}

	remoteAddr, err := NewToxAddr(toxID)
	if err != nil {
		return nil, err
	}

	localPublicKey := tox.SelfGetPublicKey()
	localNospam := tox.SelfGetNospam()
	localAddr := NewToxAddrFromPublicKey(localPublicKey, localNospam)

	friendID, found := findExistingFriend(tox, remoteAddr)
	if !found {
		if err := checkContextDone(ctx, toxID); err != nil {
			return nil, err
		}

		friendID, err = addFriendWithContext(ctx, tox, toxID)
		if err != nil {
			return nil, err
		}
	}

	conn := newToxConn(tox, friendID, localAddr, remoteAddr)

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

// waitForConnection waits for a ToxConn to establish connection.
// It respects the context deadline/timeout and polls at an adaptive interval.
func waitForConnection(ctx context.Context, conn *ToxConn) error {
	// Check context first before any waiting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if already connected
	if conn.IsConnected() {
		return nil
	}

	// Get time provider (use package default if not set on connection)
	tp := getTimeProvider(conn.timeProvider)

	// Calculate adaptive poll interval based on context deadline
	pollInterval := 100 * time.Millisecond
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		// Use at most 1/10 of remaining time as poll interval, minimum 1ms
		adaptive := remaining / 10
		if adaptive < time.Millisecond {
			adaptive = time.Millisecond
		}
		if adaptive < pollInterval {
			pollInterval = adaptive
		}
	}

	ticker := tp.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if conn.IsConnected() {
				return nil
			}
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
// The addr parameter is ignored since Tox listeners derive their address from the
// Tox instance's public key and nospam value.
//
// Deprecated: Use ListenConfig(tox, autoAccept) for explicit control over behavior,
// or Listen(tox) if the addr parameter is not needed.
func ListenAddr(addr string, tox *toxcore.Tox) (net.Listener, error) {
	_ = addr // Explicitly ignored - address derived from tox instance
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
// The tox parameter must be a valid Tox instance to derive the local address.
//
// Returns a net.Listener that wraps packet-based UDP transport in a stream-like
// interface. Each unique remote address becomes a separate net.Conn via Accept().
// The returned listener implements net.Listener to provide compatibility with
// standard Go networking patterns, despite the underlying packet semantics.
//
// Note: The packet-based API is a low-level interface for UDP-like communication
// over the Tox network. For most use cases, prefer the stream-based Listen() function.
func PacketListen(network, address string, tox *toxcore.Tox) (net.Listener, error) {
	if network != "tox" {
		return nil, &ToxNetError{
			Op:   "listen",
			Addr: address,
			Err:  net.UnknownNetworkError(network),
		}
	}

	if tox == nil {
		return nil, &ToxNetError{
			Op:   "listen",
			Addr: address,
			Err:  ErrInvalidToxID,
		}
	}

	// Create local address from the Tox instance
	localPublicKey := tox.SelfGetPublicKey()
	localNospam := tox.SelfGetNospam()
	localAddr := NewToxAddrFromPublicKey(localPublicKey, localNospam)

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
