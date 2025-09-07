package net

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore"
)

// ToxListener implements net.Listener for accepting Tox connections.
// It listens for friend requests and creates connections for accepted friends.
type ToxListener struct {
	tox       *toxcore.Tox
	localAddr *ToxAddr

	// Listener state
	closed bool
	mu     sync.RWMutex

	// Channel for incoming connections
	connCh chan net.Conn
	errCh  chan error

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Auto-accept friend requests
	autoAccept bool
}

// newToxListener creates a new ToxListener instance
func newToxListener(tox *toxcore.Tox, autoAccept bool) *ToxListener {
	ctx, cancel := context.WithCancel(context.Background())

	// Create local address from Tox instance
	localPublicKey := tox.SelfGetPublicKey()
	localNospam := tox.SelfGetNospam()
	localAddr := NewToxAddrFromPublicKey(localPublicKey, localNospam)

	listener := &ToxListener{
		tox:        tox,
		localAddr:  localAddr,
		connCh:     make(chan net.Conn, 10), // Buffer for incoming connections
		errCh:      make(chan error, 1),
		ctx:        ctx,
		cancel:     cancel,
		autoAccept: autoAccept,
	}

	// Set up callbacks
	listener.setupCallbacks()

	return listener
}

// setupCallbacks configures the Tox callbacks for the listener
func (l *ToxListener) setupCallbacks() {
	// Handle friend requests
	l.tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		l.mu.RLock()
		closed := l.closed
		autoAccept := l.autoAccept
		l.mu.RUnlock()

		if closed {
			return
		}

		if autoAccept {
			// Automatically accept the friend request
			friendID, err := l.tox.AddFriendByPublicKey(publicKey)
			if err != nil {
				select {
				case l.errCh <- &ToxNetError{Op: "accept", Err: err}:
				default:
				}
				return
			}

			// Wait a moment for the connection to establish
			go l.waitAndCreateConnection(friendID, publicKey)
		}
	})
}

// waitAndCreateConnection waits for a friend to come online and creates a connection
func (l *ToxListener) waitAndCreateConnection(friendID uint32, publicKey [32]byte) {
	// Create remote address (we don't know the nospam, so use empty)
	remoteAddr := NewToxAddrFromPublicKey(publicKey, [4]byte{})

	// Create connection
	conn := newToxConn(l.tox, friendID, l.localAddr, remoteAddr)

	// Wait for the friend to come online (with timeout)
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			conn.Close()
			return
		case <-ticker.C:
			if conn.IsConnected() {
				select {
				case l.connCh <- conn:
				case <-l.ctx.Done():
					conn.Close()
				}
				return
			}
		case <-l.ctx.Done():
			conn.Close()
			return
		}
	}
}

// Accept implements net.Listener.Accept().
// It waits for and returns the next connection to the listener.
func (l *ToxListener) Accept() (net.Conn, error) {
	l.mu.RLock()
	if l.closed {
		l.mu.RUnlock()
		return nil, ErrListenerClosed
	}
	l.mu.RUnlock()

	select {
	case conn := <-l.connCh:
		return conn, nil
	case err := <-l.errCh:
		return nil, err
	case <-l.ctx.Done():
		return nil, ErrListenerClosed
	}
}

// Close implements net.Listener.Close().
// It closes the listener and stops accepting new connections.
func (l *ToxListener) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()

	l.cancel()

	// Close any pending connections
	for {
		select {
		case conn := <-l.connCh:
			conn.Close()
		default:
			return nil
		}
	}
}

// Addr implements net.Listener.Addr().
// It returns the listener's Tox address.
func (l *ToxListener) Addr() net.Addr {
	return l.localAddr
}

// SetAutoAccept configures whether to automatically accept friend requests.
func (l *ToxListener) SetAutoAccept(autoAccept bool) {
	l.mu.Lock()
	l.autoAccept = autoAccept
	l.mu.Unlock()
}

// IsAutoAccept returns whether the listener automatically accepts friend requests.
func (l *ToxListener) IsAutoAccept() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.autoAccept
}
