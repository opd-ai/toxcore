package net

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ToxPacketListener implements net.Listener for packet-based connections.
// It listens for incoming packet connections over the Tox network and provides
// a stream-like interface for packet-based communication.
type ToxPacketListener struct {
	packetConn net.PacketConn
	localAddr  *ToxAddr

	// Listener state
	closed bool
	mu     sync.RWMutex

	// Connection management
	connections map[string]*ToxPacketConnection
	connMu      sync.RWMutex

	// Channel for incoming connections
	acceptCh chan net.Conn
	errCh    chan error

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// timeProvider provides time for deadline checks (injectable for testing)
	timeProvider TimeProvider
}

// ToxPacketConnection represents a packet-based connection to a specific peer.
// It implements net.Conn interface for packet-based communication.
type ToxPacketConnection struct {
	listener   *ToxPacketListener
	remoteAddr net.Addr
	localAddr  *ToxAddr

	// Connection state
	closed bool
	mu     sync.RWMutex

	// Packet buffers
	readBuffer  chan []byte
	writeBuffer chan packetToSend

	// Deadline management
	readDeadline  time.Time
	writeDeadline time.Time
	deadlineMu    sync.RWMutex

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// timeProvider provides time for deadline checks (injectable for testing)
	timeProvider TimeProvider
}

// packetToSend represents a packet ready to be sent
type packetToSend struct {
	data []byte
	addr net.Addr
}

// NewToxPacketListener creates a new packet-based listener.
// The localAddr should be a valid ToxAddr representing the local endpoint.
// If udpAddr is provided, it will be used for the underlying transport.
func NewToxPacketListener(localAddr *ToxAddr, udpAddr string) (*ToxPacketListener, error) {
	// Create packet connection for transport
	packetConn, err := net.ListenPacket("udp", udpAddr)
	if err != nil {
		return nil, &ToxNetError{
			Op:   "listen",
			Addr: udpAddr,
			Err:  err,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	listener := &ToxPacketListener{
		packetConn:   packetConn,
		localAddr:    localAddr,
		connections:  make(map[string]*ToxPacketConnection),
		acceptCh:     make(chan net.Conn, 10), // Buffer for incoming connections
		errCh:        make(chan error, 1),
		ctx:          ctx,
		cancel:       cancel,
		timeProvider: defaultTimeProvider,
	}

	// Start packet processing
	go listener.processPackets()

	logrus.WithFields(logrus.Fields{
		"local_addr": localAddr.String(),
		"udp_addr":   packetConn.LocalAddr().String(),
		"component":  "ToxPacketListener",
	}).Info("Created new Tox packet listener")

	return listener, nil
}

// processPackets handles incoming packets and routes them to connections
func (l *ToxPacketListener) processPackets() {
	buffer := make([]byte, 65536) // Maximum UDP packet size

	for {
		select {
		case <-l.ctx.Done():
			return
		default:
			if l.readAndProcessSinglePacket(buffer) {
				return
			}
		}
	}
}

// readAndProcessSinglePacket attempts to read and process a single packet.
// Returns true if the listener should stop processing (due to closure).
func (l *ToxPacketListener) readAndProcessSinglePacket(buffer []byte) bool {
	l.packetConn.SetReadDeadline(getTimeProvider(l.timeProvider).Now().Add(100 * time.Millisecond))

	n, addr, err := l.packetConn.ReadFrom(buffer)
	if err != nil {
		return l.handleReadError(err)
	}

	l.handlePacket(buffer[:n], addr)
	return false
}

// handleReadError processes errors from packet reading operations.
// Returns true if the listener should stop processing.
func (l *ToxPacketListener) handleReadError(err error) bool {
	if l.isTimeoutError(err) {
		return false
	}

	if l.isListenerClosed() {
		return true
	}

	l.logReadError(err)
	return false
}

// isTimeoutError checks if the error is an expected timeout error.
func (l *ToxPacketListener) isTimeoutError(err error) bool {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return false
}

// isListenerClosed checks if the listener has been closed.
func (l *ToxPacketListener) isListenerClosed() bool {
	l.mu.RLock()
	closed := l.closed
	l.mu.RUnlock()
	return closed
}

// logReadError logs packet reading errors.
func (l *ToxPacketListener) logReadError(err error) {
	logrus.WithFields(logrus.Fields{
		"error":     err.Error(),
		"component": "ToxPacketListener",
	}).Debug("Error reading packet")
}

// handlePacket processes an incoming packet and routes it to appropriate connection
func (l *ToxPacketListener) handlePacket(data []byte, addr net.Addr) {
	addrKey := addr.String()

	l.connMu.Lock()
	conn, exists := l.connections[addrKey]
	if !exists {
		// Create new connection for this address
		ctx, cancel := context.WithCancel(l.ctx)
		conn = &ToxPacketConnection{
			listener:     l,
			remoteAddr:   addr,
			localAddr:    l.localAddr,
			readBuffer:   make(chan []byte, 100),
			writeBuffer:  make(chan packetToSend, 100),
			ctx:          ctx,
			cancel:       cancel,
			timeProvider: l.timeProvider, // Inherit time provider from listener
		}
		l.connections[addrKey] = conn

		// Start write processing for this connection
		go conn.processWrites()

		// Notify about new connection
		select {
		case l.acceptCh <- conn:
			logrus.WithFields(logrus.Fields{
				"remote_addr": addr.String(),
				"component":   "ToxPacketListener",
			}).Info("New packet connection established")
		default:
			// Accept queue full, reject connection
			conn.Close()
			delete(l.connections, addrKey)
			logrus.WithFields(logrus.Fields{
				"remote_addr": addr.String(),
				"component":   "ToxPacketListener",
			}).Warn("Accept queue full, rejecting connection")
		}
	}
	l.connMu.Unlock()

	// Send data to connection's read buffer
	select {
	case conn.readBuffer <- data:
		logrus.WithFields(logrus.Fields{
			"data_size":   len(data),
			"remote_addr": addr.String(),
			"component":   "ToxPacketListener",
		}).Debug("Routed packet to connection")
	default:
		// Buffer full, drop packet
		logrus.WithFields(logrus.Fields{
			"remote_addr": addr.String(),
			"component":   "ToxPacketListener",
		}).Warn("Dropped packet due to full connection buffer")
	}
}

// Accept waits for and returns the next connection to the listener.
// This implements net.Listener.Accept().
func (l *ToxPacketListener) Accept() (net.Conn, error) {
	l.mu.RLock()
	if l.closed {
		l.mu.RUnlock()
		return nil, &ToxNetError{
			Op:  "accept",
			Err: ErrListenerClosed,
		}
	}
	l.mu.RUnlock()

	select {
	case conn := <-l.acceptCh:
		return conn, nil
	case err := <-l.errCh:
		return nil, err
	case <-l.ctx.Done():
		return nil, &ToxNetError{
			Op:  "accept",
			Err: ErrListenerClosed,
		}
	}
}

// Close closes the listener.
// This implements net.Listener.Close().
func (l *ToxPacketListener) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()

	// Cancel context to stop all operations
	l.cancel()

	// Close all connections
	l.connMu.Lock()
	for _, conn := range l.connections {
		conn.Close()
	}
	l.connMu.Unlock()

	// Close packet connection
	err := l.packetConn.Close()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error":     err.Error(),
			"component": "ToxPacketListener",
		}).Error("Error closing packet connection")
	}

	logrus.WithFields(logrus.Fields{
		"local_addr": l.localAddr.String(),
		"component":  "ToxPacketListener",
	}).Info("Closed Tox packet listener")

	return err
}

// Addr returns the listener's network address.
// This implements net.Listener.Addr().
func (l *ToxPacketListener) Addr() net.Addr {
	return l.localAddr
}

// SetTimeProvider sets the time provider for deadline checks.
// This is primarily useful for testing to inject deterministic time.
func (l *ToxPacketListener) SetTimeProvider(tp TimeProvider) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.timeProvider = tp
}

// ToxPacketConnection implementation (net.Conn interface)

// processWrites handles outgoing packets for this connection
func (c *ToxPacketConnection) processWrites() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case packet := <-c.writeBuffer:
			// Send packet through listener's packet connection
			_, err := c.listener.packetConn.WriteTo(packet.data, packet.addr)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"error":       err.Error(),
					"remote_addr": packet.addr.String(),
					"component":   "ToxPacketConnection",
				}).Error("Error sending packet")
			}
		}
	}
}

// Read reads data from the connection.
// This implements net.Conn.Read().
func (c *ToxPacketConnection) Read(b []byte) (n int, err error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, &ToxNetError{
			Op:  "read",
			Err: ErrConnectionClosed,
		}
	}
	c.mu.RUnlock()

	// Check for read deadline
	c.deadlineMu.RLock()
	deadline := c.readDeadline
	c.deadlineMu.RUnlock()

	var timeout <-chan time.Time
	if !deadline.IsZero() {
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case data := <-c.readBuffer:
		n = copy(b, data)
		if n < len(data) {
			logrus.WithFields(logrus.Fields{
				"buffer_size": len(b),
				"data_size":   len(data),
				"component":   "ToxPacketConnection",
			}).Warn("Data truncated due to small buffer")
		}
		return n, nil

	case <-timeout:
		return 0, &ToxNetError{
			Op:  "read",
			Err: ErrTimeout,
		}

	case <-c.ctx.Done():
		return 0, &ToxNetError{
			Op:  "read",
			Err: ErrConnectionClosed,
		}
	}
}

// Write writes data to the connection.
// This implements net.Conn.Write().
func (c *ToxPacketConnection) Write(b []byte) (n int, err error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, &ToxNetError{
			Op:   "write",
			Addr: c.remoteAddr.String(),
			Err:  ErrConnectionClosed,
		}
	}
	c.mu.RUnlock()

	// Check for write deadline
	c.deadlineMu.RLock()
	deadline := c.writeDeadline
	c.deadlineMu.RUnlock()

	if !deadline.IsZero() && getTimeProvider(c.timeProvider).Now().After(deadline) {
		return 0, &ToxNetError{
			Op:   "write",
			Addr: c.remoteAddr.String(),
			Err:  ErrTimeout,
		}
	}

	// Create packet to send
	packet := packetToSend{
		data: make([]byte, len(b)),
		addr: c.remoteAddr,
	}
	copy(packet.data, b)

	select {
	case c.writeBuffer <- packet:
		return len(b), nil
	default:
		return 0, &ToxNetError{
			Op:   "write",
			Addr: c.remoteAddr.String(),
			Err:  ErrBufferFull,
		}
	}
}

// Close closes the connection.
// This implements net.Conn.Close().
func (c *ToxPacketConnection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// Cancel context
	c.cancel()

	// Remove from listener's connection map
	c.listener.connMu.Lock()
	delete(c.listener.connections, c.remoteAddr.String())
	c.listener.connMu.Unlock()

	logrus.WithFields(logrus.Fields{
		"remote_addr": c.remoteAddr.String(),
		"component":   "ToxPacketConnection",
	}).Info("Closed packet connection")

	return nil
}

// LocalAddr returns the local network address.
// This implements net.Conn.LocalAddr().
func (c *ToxPacketConnection) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns the remote network address.
// This implements net.Conn.RemoteAddr().
func (c *ToxPacketConnection) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// SetDeadline sets both read and write deadlines.
// This implements net.Conn.SetDeadline().
func (c *ToxPacketConnection) SetDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	c.readDeadline = t
	c.writeDeadline = t
	c.deadlineMu.Unlock()
	return nil
}

// SetReadDeadline sets the read deadline.
// This implements net.Conn.SetReadDeadline().
func (c *ToxPacketConnection) SetReadDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	c.readDeadline = t
	c.deadlineMu.Unlock()
	return nil
}

// SetWriteDeadline sets the write deadline.
// This implements net.Conn.SetWriteDeadline().
func (c *ToxPacketConnection) SetWriteDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	c.writeDeadline = t
	c.deadlineMu.Unlock()
	return nil
}

// SetTimeProvider sets the time provider for deadline checks.
// This is primarily useful for testing to inject deterministic time.
func (c *ToxPacketConnection) SetTimeProvider(tp TimeProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timeProvider = tp
}
