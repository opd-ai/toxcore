package net

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// packetReadTimeout is the fixed timeout duration used for the packet processing
// loop's internal read deadline. This is cached to avoid recalculating
// time.Now().Add() in the hot path on every packet iteration.
const packetReadTimeout = 100 * time.Millisecond

// ToxPacketConn implements net.PacketConn for Tox packet-based communication.
// It provides UDP-like semantics over the Tox transport layer with encryption
// and routing through the Tox DHT network.
type ToxPacketConn struct {
	// Underlying UDP connection for transport
	udpConn   net.PacketConn
	localAddr *ToxAddr

	// Connection state
	closed bool
	mu     sync.RWMutex

	// Packet handling
	readBuffer chan packetWithAddr

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

// packetWithAddr bundles a packet with its source address
type packetWithAddr struct {
	data []byte
	addr net.Addr
}

// NewToxPacketConn creates a new ToxPacketConn.
// The localAddr should be a valid ToxAddr representing the local endpoint.
// If udpAddr is provided, it will be used for the underlying transport.
func NewToxPacketConn(localAddr *ToxAddr, udpAddr string) (*ToxPacketConn, error) {
	// Create UDP connection for transport
	udpConn, err := net.ListenPacket("udp", udpAddr)
	if err != nil {
		return nil, &ToxNetError{
			Op:   "listen",
			Addr: udpAddr,
			Err:  err,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	conn := &ToxPacketConn{
		udpConn:      udpConn,
		localAddr:    localAddr,
		readBuffer:   make(chan packetWithAddr, 100), // Buffer for incoming packets
		ctx:          ctx,
		cancel:       cancel,
		timeProvider: defaultTimeProvider,
	}

	// Start packet processing
	go conn.processPackets()

	logrus.WithFields(logrus.Fields{
		"local_addr": localAddr.String(),
		"udp_addr":   udpConn.LocalAddr().String(),
		"component":  "ToxPacketConn",
	}).Info("Created new Tox packet connection")

	return conn, nil
}

// processPackets handles incoming UDP packets and routes them to the read buffer
func (c *ToxPacketConn) processPackets() {
	buffer := make([]byte, 65536) // Maximum UDP packet size

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if !c.processIncomingPacket(buffer) {
				return
			}
		}
	}
}

// processIncomingPacket reads and processes a single incoming packet.
// Returns false if the connection should be terminated, true to continue processing.
func (c *ToxPacketConn) processIncomingPacket(buffer []byte) bool {
	// Set read deadline using pre-computed constant to avoid recalculating
	// time.Now().Add() in the hot loop for every packet
	if err := c.udpConn.SetReadDeadline(getTimeProvider(c.timeProvider).Now().Add(packetReadTimeout)); err != nil {
		return c.handleReadError(err)
	}

	n, addr, err := c.udpConn.ReadFrom(buffer)
	if err != nil {
		return c.handleReadError(err)
	}

	packet := c.createPacketWithAddr(buffer[:n], addr)
	c.enqueuePacket(packet, n)
	return true
}

// handleReadError processes read errors and determines if processing should continue.
// Returns false if the connection should be terminated, true to continue processing.
func (c *ToxPacketConn) handleReadError(err error) bool {
	// Check if it's a timeout error, which is expected
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Check if connection is closed
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()
	if closed {
		return false
	}

	logrus.WithFields(logrus.Fields{
		"error":     err.Error(),
		"component": "ToxPacketConn",
	}).Debug("Error reading packet")
	return true
}

// createPacketWithAddr creates a new packet structure with a copy of the data.
func (c *ToxPacketConn) createPacketWithAddr(data []byte, addr net.Addr) packetWithAddr {
	packet := packetWithAddr{
		data: make([]byte, len(data)),
		addr: addr,
	}
	copy(packet.data, data)
	return packet
}

// enqueuePacket attempts to send a packet to the read buffer with logging.
func (c *ToxPacketConn) enqueuePacket(packet packetWithAddr, dataSize int) {
	select {
	case c.readBuffer <- packet:
		logrus.WithFields(logrus.Fields{
			"data_size":   dataSize,
			"remote_addr": packet.addr.String(),
			"component":   "ToxPacketConn",
		}).Debug("Received packet")
	default:
		// Buffer full, drop packet
		logrus.WithFields(logrus.Fields{
			"remote_addr": packet.addr.String(),
			"component":   "ToxPacketConn",
		}).Warn("Dropped packet due to full buffer")
	}
}

// validateConnectionState checks if the connection is closed and returns an error if so.
func (c *ToxPacketConn) validateConnectionState() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return &ToxNetError{
			Op:  "read",
			Err: ErrConnectionClosed,
		}
	}
	return nil
}

// setupReadTimeout configures the timeout channel for read operations based on the deadline.
func (c *ToxPacketConn) setupReadTimeout() <-chan time.Time {
	c.deadlineMu.RLock()
	deadline := c.readDeadline
	c.deadlineMu.RUnlock()

	if deadline.IsZero() {
		return nil
	}

	timer := time.NewTimer(time.Until(deadline))
	// Note: caller is responsible for stopping the timer
	return timer.C
}

// processPacketData copies packet data to the provided buffer and handles truncation warnings.
func (c *ToxPacketConn) processPacketData(p []byte, packet packetWithAddr) (int, net.Addr) {
	n := copy(p, packet.data)
	if n < len(packet.data) {
		logrus.WithFields(logrus.Fields{
			"buffer_size": len(p),
			"packet_size": len(packet.data),
			"component":   "ToxPacketConn",
		}).Warn("Packet truncated due to small buffer")
	}
	return n, packet.addr
}

// ReadFrom reads a packet from the connection and returns the data and source address.
// This implements net.PacketConn.ReadFrom().
func (c *ToxPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if err := c.validateConnectionState(); err != nil {
		return 0, nil, err
	}

	timeout := c.setupReadTimeout()
	var timer *time.Timer
	if timeout != nil {
		timer = time.NewTimer(time.Until(c.readDeadline))
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case packet := <-c.readBuffer:
		n, addr = c.processPacketData(p, packet)
		return n, addr, nil

	case <-timeout:
		return 0, nil, &ToxNetError{
			Op:  "read",
			Err: ErrTimeout,
		}

	case <-c.ctx.Done():
		return 0, nil, &ToxNetError{
			Op:  "read",
			Err: ErrConnectionClosed,
		}
	}
}

// WriteTo writes a packet to the specified address.
// This implements net.PacketConn.WriteTo().
//
// WARNING: This is a placeholder implementation that writes directly to the
// underlying UDP socket without Tox protocol encryption or formatting.
// In a production implementation, packets should be encrypted using the Tox
// protocol's encryption layer before transmission. This API is suitable for
// testing and development but should not be used for secure communication
// without proper Tox protocol integration.
//
// TODO: Implement Tox packet formatting and encryption for protocol compliance.
func (c *ToxPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, &ToxNetError{
			Op:   "write",
			Addr: addr.String(),
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
			Addr: addr.String(),
			Err:  ErrTimeout,
		}
	}

	// WARNING: Direct UDP write without Tox protocol encryption (placeholder)
	n, err = c.udpConn.WriteTo(p, addr)
	if err != nil {
		return 0, &ToxNetError{
			Op:   "write",
			Addr: addr.String(),
			Err:  err,
		}
	}

	logrus.WithFields(logrus.Fields{
		"bytes_sent":  n,
		"remote_addr": addr.String(),
		"component":   "ToxPacketConn",
	}).Debug("Sent packet")

	return n, nil
}

// Close closes the packet connection.
// This implements net.PacketConn.Close().
func (c *ToxPacketConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// Cancel context to stop all operations
	c.cancel()

	// Close UDP connection
	err := c.udpConn.Close()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error":     err.Error(),
			"component": "ToxPacketConn",
		}).Error("Error closing UDP connection")

		return &ToxNetError{
			Op:   "close",
			Addr: c.localAddr.String(),
			Err:  err,
		}
	}

	logrus.WithFields(logrus.Fields{
		"local_addr": c.localAddr.String(),
		"component":  "ToxPacketConn",
	}).Info("Closed Tox packet connection")

	return nil
}

// LocalAddr returns the local network address.
// This implements net.PacketConn.LocalAddr().
func (c *ToxPacketConn) LocalAddr() net.Addr {
	return c.localAddr
}

// SetDeadline sets both read and write deadlines.
// This implements net.PacketConn.SetDeadline().
func (c *ToxPacketConn) SetDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	c.readDeadline = t
	c.writeDeadline = t
	c.deadlineMu.Unlock()
	return nil
}

// SetReadDeadline sets the read deadline.
// This implements net.PacketConn.SetReadDeadline().
func (c *ToxPacketConn) SetReadDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	c.readDeadline = t
	c.deadlineMu.Unlock()
	return nil
}

// SetWriteDeadline sets the write deadline.
// This implements net.PacketConn.SetWriteDeadline().
func (c *ToxPacketConn) SetWriteDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	c.writeDeadline = t
	c.deadlineMu.Unlock()
	return nil
}

// SetTimeProvider sets the time provider for deadline checks.
// This is primarily useful for testing to inject deterministic time.
func (c *ToxPacketConn) SetTimeProvider(tp TimeProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timeProvider = tp
}
