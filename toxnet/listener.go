package toxnet

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

	// timeProvider provides time for connection timeout management (injectable for testing)
	timeProvider TimeProvider

	// goroutineWg tracks background goroutines for synchronous shutdown
	goroutineWg sync.WaitGroup
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

// tryStartAcceptRequest reserves a background worker for accepting a friend request.
func (l *ToxListener) tryStartAcceptRequest() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || !l.autoAccept {
		return false
	}
	l.goroutineWg.Add(1)
	return true
}

// acceptFriendRequest adds a friend and initiates connection setup.
func (l *ToxListener) acceptFriendRequest(publicKey [32]byte) {
	if !l.tryStartAcceptRequest() {
		return
	}

	friendID, err := l.tox.AddFriendByPublicKey(publicKey)
	if err != nil {
		l.goroutineWg.Done()
		select {
		case l.errCh <- &ToxNetError{Op: "accept", Err: err}:
		default:
		}
		return
	}
	l.goroutineWg.Add(1)
	go func() {
		defer l.goroutineWg.Done()
		l.waitAndCreateConnection(friendID, publicKey)
	}()
}

func (l *ToxListener) setupCallbacks() {
	l.tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		l.acceptFriendRequest(publicKey)
	})
}

// waitAndCreateConnection waits for a friend to come online and creates a connection
func (l *ToxListener) waitAndCreateConnection(friendID uint32, publicKey [32]byte) {
	conn := l.createNewConnection(friendID, publicKey)

	timeout, ticker := l.setupConnectionTimers()
	defer l.cleanupTimers(timeout, ticker)

	delivered := l.monitorConnectionStatus(conn, timeout, ticker)
	if !delivered {
		// Only clean up if the connection was never delivered to Accept().
		l.cleanupConnection(conn)
	}
}

// createNewConnection initializes a new connection for the given friend.
func (l *ToxListener) createNewConnection(friendID uint32, publicKey [32]byte) *ToxConn {
	// Create remote address (we don't know the nospam, so use empty)
	remoteAddr := NewToxAddrFromPublicKey(publicKey, [4]byte{})
	return newToxConn(l.tox, friendID, l.localAddr, remoteAddr)
}

// setupConnectionTimers creates and configures the timeout and ticker for connection monitoring.
func (l *ToxListener) setupConnectionTimers() (*time.Timer, *time.Ticker) {
	tp := getTimeProvider(l.timeProvider)
	timeout := tp.NewTimer(30 * time.Second)
	ticker := tp.NewTicker(100 * time.Millisecond)
	return timeout, ticker
}

// cleanupTimers stops and cleans up the timeout timer and ticker.
func (l *ToxListener) cleanupTimers(timeout *time.Timer, ticker *time.Ticker) {
	timeout.Stop()
	ticker.Stop()
}

// checkAndDeliverConnection checks if connection is ready and delivers it.
// Returns true if the connection was successfully delivered to Accept().
func (l *ToxListener) checkAndDeliverConnection(conn *ToxConn) bool {
	if conn.IsConnected() {
		l.deliverConnection(conn)
		return true
	}
	return false
}

// monitorConnectionStatus monitors the connection status until it is delivered or times out.
// Returns true if the connection was successfully delivered.
func (l *ToxListener) monitorConnectionStatus(conn *ToxConn, timeout *time.Timer, ticker *time.Ticker) bool {
	for {
		delivered, stop := l.shouldStopMonitoring(conn, timeout, ticker)
		if stop {
			return delivered
		}
	}
}

// shouldStopMonitoring checks if connection monitoring should stop.
// Returns (delivered, stop) — delivered is true when the conn was sent to Accept().
func (l *ToxListener) shouldStopMonitoring(conn *ToxConn, timeout *time.Timer, ticker *time.Ticker) (bool, bool) {
	select {
	case <-timeout.C:
		return false, true
	case <-ticker.C:
		if l.checkAndDeliverConnection(conn) {
			return true, true
		}
		return false, false
	case <-l.ctx.Done():
		return false, true
	}
}

// deliverConnection attempts to deliver the connection to the connection channel.
func (l *ToxListener) deliverConnection(conn *ToxConn) {
	select {
	case l.connCh <- conn:
	case <-l.ctx.Done():
	}
}

// cleanupConnection closes the connection if it's not nil.
func (l *ToxListener) cleanupConnection(conn *ToxConn) {
	if conn != nil {
		conn.Close()
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

	// Wait for all background goroutines to drain
	l.goroutineWg.Wait()

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

// SetTimeProvider sets the time provider for deterministic testing.
// If tp is nil, uses the package-level default time provider.
func (l *ToxListener) SetTimeProvider(tp TimeProvider) {
	l.mu.Lock()
	l.timeProvider = tp
	l.mu.Unlock()
}
