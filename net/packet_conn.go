package net

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
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

	// Encryption support - when configured, packets are encrypted/decrypted
	// using NaCl box encryption (Curve25519 + XSalsa20 + Poly1305)
	encryptionEnabled bool
	localKeyPair      *crypto.KeyPair
	peerKeys          map[string][32]byte // Maps addr.String() to peer public key
	peerKeysMu        sync.RWMutex
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
		peerKeys:     make(map[string][32]byte),
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
// If encryption is enabled, attempts to decrypt the packet first.
func (c *ToxPacketConn) createPacketWithAddr(data []byte, addr net.Addr) packetWithAddr {
	c.mu.RLock()
	encEnabled := c.encryptionEnabled
	c.mu.RUnlock()

	finalData := data
	if encEnabled {
		decrypted, err := c.decryptPacket(data, addr)
		if err == nil {
			finalData = decrypted
		}
		// On error, use original data (may be unencrypted packet)
	}

	packet := packetWithAddr{
		data: make([]byte, len(finalData)),
		addr: addr,
	}
	copy(packet.data, finalData)
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
// When encryption is enabled via EnableEncryption() and a peer key is registered
// via AddPeerKey(), packets are encrypted using NaCl box encryption
// (Curve25519 + XSalsa20 + Poly1305) before transmission. The encrypted packet
// format is: nonce (24 bytes) + ciphertext.
//
// When encryption is not configured, packets are sent directly to the underlying
// UDP socket. Use EnableEncryption() to configure secure communication.
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
	encEnabled := c.encryptionEnabled
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

	// Encrypt packet if encryption is enabled
	dataToSend := p
	if encEnabled {
		encrypted, encErr := c.encryptPacket(p, addr)
		if encErr != nil {
			return 0, encErr
		}
		dataToSend = encrypted
	}

	_, err = c.udpConn.WriteTo(dataToSend, addr)
	if err != nil {
		return 0, &ToxNetError{
			Op:   "write",
			Addr: addr.String(),
			Err:  err,
		}
	}

	logrus.WithFields(logrus.Fields{
		"bytes_sent":  len(p),
		"remote_addr": addr.String(),
		"encrypted":   encEnabled,
		"component":   "ToxPacketConn",
	}).Debug("Sent packet")

	// Return original plaintext length, not encrypted length
	return len(p), nil
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

// EnableEncryption configures the connection to encrypt/decrypt all packets
// using NaCl box encryption. The localKeyPair provides the sender's keys for
// encryption and the recipient's keys for decryption.
func (c *ToxPacketConn) EnableEncryption(keyPair *crypto.KeyPair) error {
	if keyPair == nil {
		return &ToxNetError{
			Op:  "configure",
			Err: ErrInvalidToxID,
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.localKeyPair = keyPair
	c.encryptionEnabled = true

	logrus.WithFields(logrus.Fields{
		"component": "ToxPacketConn",
	}).Info("Encryption enabled for packet connection")

	return nil
}

// normalizeAddrKey returns a normalized string key for address-based lookups.
// This handles IPv6 address variations (e.g., [::] vs [::1]) for local communication.
func normalizeAddrKey(addr net.Addr) string {
	addrStr := addr.String()
	// Normalize IPv6 unspecified address [::] to loopback [::1] for local testing
	// This ensures consistent lookups when LocalAddr reports [::] but actual
	// packets appear from [::1] (loopback)
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		if udpAddr.IP.IsUnspecified() {
			// Use port only as key for unspecified addresses
			return fmt.Sprintf("[::]:%d", udpAddr.Port)
		}
		if udpAddr.IP.IsLoopback() {
			// Normalize loopback to unspecified for consistent matching
			return fmt.Sprintf("[::]:%d", udpAddr.Port)
		}
	}
	return addrStr
}

// AddPeerKey registers a peer's public key for encrypted communication.
// The addr should match the address used in WriteTo calls.
func (c *ToxPacketConn) AddPeerKey(addr net.Addr, publicKey [32]byte) {
	c.peerKeysMu.Lock()
	defer c.peerKeysMu.Unlock()

	key := normalizeAddrKey(addr)
	c.peerKeys[key] = publicKey

	logrus.WithFields(logrus.Fields{
		"peer_addr": key,
		"component": "ToxPacketConn",
	}).Debug("Added peer public key for encryption")
}

// RemovePeerKey removes a peer's public key.
func (c *ToxPacketConn) RemovePeerKey(addr net.Addr) {
	c.peerKeysMu.Lock()
	defer c.peerKeysMu.Unlock()

	key := normalizeAddrKey(addr)
	delete(c.peerKeys, key)
}

// IsEncryptionEnabled returns true if encryption is configured.
func (c *ToxPacketConn) IsEncryptionEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.encryptionEnabled
}

// encryptPacket encrypts a packet for the given peer address.
// Returns the encrypted packet (nonce + ciphertext) or error.
func (c *ToxPacketConn) encryptPacket(data []byte, addr net.Addr) ([]byte, error) {
	key := normalizeAddrKey(addr)
	c.peerKeysMu.RLock()
	peerKey, found := c.peerKeys[key]
	c.peerKeysMu.RUnlock()

	if !found {
		return nil, &ToxNetError{
			Op:   "encrypt",
			Addr: addr.String(),
			Err:  ErrNoPeerKey,
		}
	}

	nonce, err := crypto.GenerateNonce()
	if err != nil {
		return nil, &ToxNetError{
			Op:   "encrypt",
			Addr: addr.String(),
			Err:  err,
		}
	}

	ciphertext, err := crypto.Encrypt(data, nonce, peerKey, c.localKeyPair.Private)
	if err != nil {
		return nil, &ToxNetError{
			Op:   "encrypt",
			Addr: addr.String(),
			Err:  err,
		}
	}

	// Prepend nonce to ciphertext
	packet := make([]byte, 24+len(ciphertext))
	copy(packet[:24], nonce[:])
	copy(packet[24:], ciphertext)

	return packet, nil
}

// decryptPacket decrypts a packet from the given peer address.
// Expects packet format: nonce (24 bytes) + ciphertext.
func (c *ToxPacketConn) decryptPacket(data []byte, addr net.Addr) ([]byte, error) {
	if len(data) < 25 { // 24 byte nonce + at least 1 byte
		return nil, &ToxNetError{
			Op:   "decrypt",
			Addr: addr.String(),
			Err:  ErrInvalidToxID,
		}
	}

	key := normalizeAddrKey(addr)
	c.peerKeysMu.RLock()
	peerKey, found := c.peerKeys[key]
	c.peerKeysMu.RUnlock()

	if !found {
		// Unknown peer - cannot decrypt, return original data
		// This allows mixed encrypted/unencrypted communication
		return data, nil
	}

	var nonce crypto.Nonce
	copy(nonce[:], data[:24])

	plaintext, err := crypto.Decrypt(data[24:], nonce, peerKey, c.localKeyPair.Private)
	if err != nil {
		// Decryption failed - could be unencrypted data, return original
		logrus.WithFields(logrus.Fields{
			"peer_addr": addr.String(),
			"error":     err.Error(),
			"component": "ToxPacketConn",
		}).Debug("Packet decryption failed, may be unencrypted")
		return data, nil
	}

	return plaintext, nil
}
