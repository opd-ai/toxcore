package net

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore"
)

// ToxConn implements net.Conn for Tox friend connections.
// It provides a stream-like interface over Tox's message-based protocol.
type ToxConn struct {
	tox        *toxcore.Tox
	friendID   uint32
	localAddr  *ToxAddr
	remoteAddr *ToxAddr

	// Connection state
	connected bool
	closed    bool
	mu        sync.RWMutex

	// Read buffer for incoming messages
	readBuffer *bytes.Buffer
	readMu     sync.Mutex
	readCond   *sync.Cond

	// Write chunking for large messages
	writeMu sync.Mutex

	// Deadline management
	readDeadline  time.Time
	writeDeadline time.Time
	deadlineMu    sync.RWMutex

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Channel for signaling connection state changes
	connStateCh chan bool

	// router tracks the callback router for this connection's Tox instance
	router *callbackRouter

	// timeProvider provides time for deadline checks (injectable for testing)
	timeProvider TimeProvider
}

// newToxConn creates a new ToxConn instance
func newToxConn(tox *toxcore.Tox, friendID uint32, localAddr, remoteAddr *ToxAddr) *ToxConn {
	ctx, cancel := context.WithCancel(context.Background())

	conn := &ToxConn{
		tox:          tox,
		friendID:     friendID,
		localAddr:    localAddr,
		remoteAddr:   remoteAddr,
		connected:    false,
		readBuffer:   new(bytes.Buffer),
		ctx:          ctx,
		cancel:       cancel,
		connStateCh:  make(chan bool, 1),
		timeProvider: defaultTimeProvider,
	}

	conn.readCond = sync.NewCond(&conn.readMu)

	// Register with the callback router for this Tox instance
	conn.router = getOrCreateRouter(tox)
	conn.router.registerConnection(conn)

	// Check initial connection state
	friends := tox.GetFriends()
	for _, friend := range friends {
		if friend.PublicKey == remoteAddr.PublicKey() {
			conn.connected = (friend.ConnectionStatus != toxcore.ConnectionNone)
			break
		}
	}

	return conn
}

// validateReadInput checks if the provided buffer is valid for reading.
func (c *ToxConn) validateReadInput(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	return -1, nil // Continue processing
}

// checkConnectionClosed verifies the connection is not closed.
func (c *ToxConn) checkConnectionClosed() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return ErrConnectionClosed
	}
	return nil
}

// setupReadTimeout configures timeout channel for read operations.
// Returns the timeout channel and a cleanup function that must be called to prevent timer leaks.
// The cleanup function is safe to call multiple times and on nil receivers.
func (c *ToxConn) setupReadTimeout() (<-chan time.Time, func()) {
	c.deadlineMu.RLock()
	deadline := c.readDeadline
	c.deadlineMu.RUnlock()

	if !deadline.IsZero() {
		timer := time.NewTimer(time.Until(deadline))
		cleanup := func() {
			if !timer.Stop() {
				// Drain the channel if the timer already fired
				select {
				case <-timer.C:
				default:
				}
			}
		}
		return timer.C, cleanup
	}
	return nil, func() {} // No-op cleanup for nil timeout
}

// waitForDataSignal waits for data availability signal with timeout handling.
func (c *ToxConn) waitForDataSignal(timeout <-chan time.Time) error {
	done := make(chan struct{})
	go func() {
		c.readCond.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Data available, continue to read
		return nil
	case <-timeout:
		return &ToxNetError{Op: "read", Err: ErrTimeout}
	case <-c.ctx.Done():
		return ErrConnectionClosed
	}
}

// waitForReadData waits for data to be available in the read buffer with timeout handling.
func (c *ToxConn) waitForReadData(timeout <-chan time.Time) error {
	for c.readBuffer.Len() == 0 {
		if err := c.checkConnectionClosed(); err != nil {
			return err
		}

		if err := c.waitForDataSignal(timeout); err != nil {
			return err
		}
	}
	return nil
}

// Read implements net.Conn.Read().
// It reads data from the connection into the provided buffer.
func (c *ToxConn) Read(b []byte) (int, error) {
	if n, err := c.validateReadInput(b); n >= 0 {
		return n, err
	}

	if err := c.checkConnectionClosed(); err != nil {
		return 0, err
	}

	timeout, cleanup := c.setupReadTimeout()
	defer cleanup()

	c.readMu.Lock()
	defer c.readMu.Unlock()

	if err := c.waitForReadData(timeout); err != nil {
		return 0, err
	}

	return c.readBuffer.Read(b)
}

// Write implements net.Conn.Write().
// It writes data to the connection, chunking large messages as needed.
func (c *ToxConn) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	if err := c.validateWriteConditions(); err != nil {
		return 0, err
	}

	if err := c.ensureConnected(); err != nil {
		return 0, err
	}

	if err := c.checkWriteDeadline(); err != nil {
		return 0, err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.writeChunkedData(b)
}

// validateWriteConditions checks if the connection is in a valid state for writing.
func (c *ToxConn) validateWriteConditions() error {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return ErrConnectionClosed
	}
	return nil
}

// ensureConnected waits for the connection to be established if needed.
// Uses a double-check pattern: first checks under RLock, then waitForConnection
// will revalidate under lock to avoid TOCTOU races.
func (c *ToxConn) ensureConnected() error {
	// First check: quick path if already connected (common case)
	c.mu.RLock()
	connected := c.connected
	closed := c.closed
	c.mu.RUnlock()

	// Early exit if closed
	if closed {
		return ErrConnectionClosed
	}

	// Early exit if already connected
	if connected {
		return nil
	}

	// Slow path: wait for connection (re-checks state with proper locking)
	return c.waitForConnection()
}

// checkWriteDeadline verifies if the write deadline has been exceeded.
func (c *ToxConn) checkWriteDeadline() error {
	c.deadlineMu.RLock()
	deadline := c.writeDeadline
	c.deadlineMu.RUnlock()

	if !deadline.IsZero() && getTimeProvider(c.timeProvider).Now().After(deadline) {
		return &ToxNetError{Op: "write", Err: ErrTimeout}
	}
	return nil
}

// writeChunkedData writes data in chunks, respecting Tox message size limits.
// Returns the number of bytes written and an error. If a partial write occurs due to
// an underlying error, both the written count and a wrapped ErrPartialWrite are returned.
func (c *ToxConn) writeChunkedData(b []byte) (int, error) {
	// Tox message size limit is typically around 1372 bytes
	const maxChunkSize = 1300
	data := b
	totalWritten := 0

	c.deadlineMu.RLock()
	deadline := c.writeDeadline
	c.deadlineMu.RUnlock()

	for len(data) > 0 {
		chunkSize := len(data)
		if chunkSize > maxChunkSize {
			chunkSize = maxChunkSize
		}

		chunk := data[:chunkSize]
		_, err := c.tox.FriendSendMessage(c.friendID, string(chunk), toxcore.MessageTypeNormal)
		if err != nil {
			if totalWritten > 0 {
				// Return partial write with wrapped error indicating partial success
				return totalWritten, &ToxNetError{Op: "write", Err: fmt.Errorf("%w: %v", ErrPartialWrite, err)}
			}
			return 0, &ToxNetError{Op: "write", Err: err}
		}

		totalWritten += chunkSize
		data = data[chunkSize:]

		// Check deadline between chunks
		if !deadline.IsZero() && getTimeProvider(c.timeProvider).Now().After(deadline) {
			if totalWritten > 0 {
				// Return partial write with wrapped error indicating partial success
				return totalWritten, &ToxNetError{Op: "write", Err: fmt.Errorf("%w: %v", ErrPartialWrite, ErrTimeout)}
			}
			return 0, &ToxNetError{Op: "write", Err: ErrTimeout}
		}
	}

	return totalWritten, nil
}

// waitForConnection waits for the friend to come online
func (c *ToxConn) waitForConnection() error {
	timeout, cleanup := c.setupConnectionTimeout()
	defer cleanup()

	for {
		connected, err := c.checkConnectionStatus()
		if err != nil {
			return err
		}
		if connected {
			return nil
		}

		if err := c.waitForConnectionEvent(timeout); err != nil {
			return err
		}
	}
}

// setupConnectionTimeout prepares timeout channel based on write deadline.
// Returns the timeout channel and a cleanup function that must be called to prevent timer leaks.
// The cleanup function is safe to call multiple times and on nil receivers.
func (c *ToxConn) setupConnectionTimeout() (<-chan time.Time, func()) {
	c.deadlineMu.RLock()
	deadline := c.writeDeadline
	c.deadlineMu.RUnlock()

	if deadline.IsZero() {
		return nil, func() {} // No-op cleanup for nil timeout
	}

	timer := time.NewTimer(time.Until(deadline))
	cleanup := func() {
		if !timer.Stop() {
			// Drain the channel if the timer already fired
			select {
			case <-timer.C:
			default:
			}
		}
	}
	return timer.C, cleanup
}

// checkConnectionStatus verifies current connection state and returns connected status.
func (c *ToxConn) checkConnectionStatus() (bool, error) {
	c.mu.RLock()
	connected := c.connected
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return false, ErrConnectionClosed
	}

	return connected, nil
}

// waitForConnectionEvent waits for connection state changes or timeout.
func (c *ToxConn) waitForConnectionEvent(timeout <-chan time.Time) error {
	select {
	case <-c.connStateCh:
		// Connection state changed, will check again in main loop
		return nil
	case <-timeout:
		return &ToxNetError{Op: "write", Err: ErrTimeout}
	case <-c.ctx.Done():
		return ErrConnectionClosed
	}
}

// Close implements net.Conn.Close().
// It closes the connection and cleans up resources.
func (c *ToxConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// Unregister from callback router
	if c.router != nil {
		c.router.unregisterConnection(c.friendID)
		cleanupRouter(c.tox)
	}

	c.cancel()
	c.readCond.Broadcast()

	return nil
}

// LocalAddr implements net.Conn.LocalAddr().
// It returns the local Tox address.
func (c *ToxConn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr implements net.Conn.RemoteAddr().
// It returns the remote Tox address.
func (c *ToxConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// SetDeadline implements net.Conn.SetDeadline().
// It sets both read and write deadlines.
func (c *ToxConn) SetDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	c.readDeadline = t
	c.writeDeadline = t
	c.deadlineMu.Unlock()
	return nil
}

// SetReadDeadline implements net.Conn.SetReadDeadline().
// It sets the deadline for read operations.
func (c *ToxConn) SetReadDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	c.readDeadline = t
	c.deadlineMu.Unlock()
	return nil
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline().
// It sets the deadline for write operations.
func (c *ToxConn) SetWriteDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	c.writeDeadline = t
	c.deadlineMu.Unlock()
	return nil
}

// FriendID returns the Tox friend ID for this connection.
func (c *ToxConn) FriendID() uint32 {
	return c.friendID
}

// IsConnected returns true if the friend is currently online.
func (c *ToxConn) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && !c.closed
}

// SetTimeProvider sets the time provider for deadline checks.
// This is primarily useful for testing to inject deterministic time.
func (c *ToxConn) SetTimeProvider(tp TimeProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timeProvider = tp
}
