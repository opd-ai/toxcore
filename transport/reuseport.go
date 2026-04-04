// Package transport provides network transport implementations for Tox.
package transport

import (
	"context"
	"errors"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// ReusePortConfig configures SO_REUSEPORT UDP transport behavior.
type ReusePortConfig struct {
	// NumSockets is the number of UDP sockets to create.
	// Default is runtime.NumCPU() for optimal core scaling.
	NumSockets int

	// WorkerPool configuration for each socket's packet processing.
	// If nil, packets are processed in unbounded goroutines (legacy behavior).
	WorkerPool *WorkerPoolConfig
}

// DefaultReusePortConfig returns a sensible default configuration.
func DefaultReusePortConfig() *ReusePortConfig {
	return &ReusePortConfig{
		NumSockets: runtime.NumCPU(),
		WorkerPool: DefaultWorkerPoolConfig(),
	}
}

// ReusePortTransport implements UDP transport with SO_REUSEPORT.
// Multiple sockets share the same address, allowing kernel-level load balancing
// across CPU cores for improved throughput.
type ReusePortTransport struct {
	// sockets holds multiple UDP connections sharing the same port
	sockets []net.PacketConn

	// listenAddr is the shared address all sockets bind to
	listenAddr net.Addr

	// handlers maps packet types to their handlers
	handlers map[PacketType]PacketHandler

	// workerPool processes packets with bounded concurrency (optional)
	workerPool *WorkerPool

	// mu protects handler registration
	mu sync.RWMutex

	// ctx and cancel for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// stats for monitoring
	packetsReceived uint64
	packetsSent     uint64
	bytesReceived   uint64
	bytesSent       uint64
	sendErrors      uint64
	receiveErrors   uint64
}

// ReusePortStats provides statistics about the transport.
type ReusePortStats struct {
	NumSockets      int
	PacketsReceived uint64
	PacketsSent     uint64
	BytesReceived   uint64
	BytesSent       uint64
	SendErrors      uint64
	ReceiveErrors   uint64
	WorkerPoolStats *WorkerPoolStats
}

// NewReusePortTransport creates a new UDP transport with SO_REUSEPORT.
// This allows multiple sockets to bind to the same address:port,
// enabling kernel-level load balancing for improved throughput.
//
// Note: SO_REUSEPORT is supported on Linux 3.9+, FreeBSD 12+, and macOS.
// On unsupported platforms, this falls back to a single socket.
func NewReusePortTransport(listenAddr string, config *ReusePortConfig) (*ReusePortTransport, error) {
	config = normalizeReusePortConfig(config)

	logReusePortCreation(listenAddr, config.NumSockets)

	transport := initReusePortTransport(config)

	if err := transport.setupSockets(listenAddr, config.NumSockets); err != nil {
		transport.cancel()
		return nil, err
	}

	transport.startPacketProcessors()
	logReusePortSuccess(transport)

	return transport, nil
}

// normalizeReusePortConfig returns a valid config with defaults applied.
func normalizeReusePortConfig(config *ReusePortConfig) *ReusePortConfig {
	if config == nil {
		config = DefaultReusePortConfig()
	}
	if config.NumSockets < 1 {
		config.NumSockets = 1
	}
	return config
}

// logReusePortCreation logs the start of transport creation.
func logReusePortCreation(listenAddr string, numSockets int) {
	logrus.WithFields(logrus.Fields{
		"function":    "NewReusePortTransport",
		"listen_addr": listenAddr,
		"num_sockets": numSockets,
	}).Info("Creating SO_REUSEPORT UDP transport")
}

// initReusePortTransport creates a new transport with base configuration.
func initReusePortTransport(config *ReusePortConfig) *ReusePortTransport {
	ctx, cancel := context.WithCancel(context.Background())
	transport := &ReusePortTransport{
		sockets:  make([]net.PacketConn, 0, config.NumSockets),
		handlers: make(map[PacketType]PacketHandler),
		ctx:      ctx,
		cancel:   cancel,
	}
	if config.WorkerPool != nil {
		transport.workerPool = NewWorkerPool(config.WorkerPool)
	}
	return transport
}

// setupSockets creates SO_REUSEPORT sockets or falls back to a single socket.
func (t *ReusePortTransport) setupSockets(listenAddr string, numSockets int) error {
	sockets, addr, err := createReusePortSockets(listenAddr, numSockets)
	if err != nil {
		return t.fallbackToSingleSocket(listenAddr)
	}
	t.sockets = sockets
	t.listenAddr = addr
	return nil
}

// fallbackToSingleSocket creates a single standard UDP socket as fallback.
func (t *ReusePortTransport) fallbackToSingleSocket(listenAddr string) error {
	logrus.WithFields(logrus.Fields{
		"function": "NewReusePortTransport",
	}).Warn("SO_REUSEPORT not available, falling back to single socket")

	conn, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		return err
	}
	t.sockets = []net.PacketConn{conn}
	t.listenAddr = conn.LocalAddr()
	return nil
}

// startPacketProcessors starts a processing goroutine for each socket.
func (t *ReusePortTransport) startPacketProcessors() {
	for i, conn := range t.sockets {
		go t.processPackets(i, conn)
	}
}

// logReusePortSuccess logs successful transport creation.
func logReusePortSuccess(transport *ReusePortTransport) {
	logrus.WithFields(logrus.Fields{
		"function":    "NewReusePortTransport",
		"listen_addr": transport.listenAddr.String(),
		"num_sockets": len(transport.sockets),
	}).Info("SO_REUSEPORT transport created")
}

// RegisterHandler registers a handler for a specific packet type.
func (t *ReusePortTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers[packetType] = handler

	logrus.WithFields(logrus.Fields{
		"function":    "ReusePortTransport.RegisterHandler",
		"packet_type": packetType,
	}).Debug("Handler registered")
}

// Send sends a packet to the specified address.
// The packet is sent via the first socket (all sockets share the same local address).
func (t *ReusePortTransport) Send(packet *Packet, addr net.Addr) error {
	if len(t.sockets) == 0 {
		return errors.New("transport closed")
	}

	data, err := packet.Serialize()
	if err != nil {
		return err
	}

	// Use first socket for sending (kernel handles receive load balancing)
	_, err = t.sockets[0].WriteTo(data, addr)
	if err != nil {
		atomic.AddUint64(&t.sendErrors, 1)
		logrus.WithFields(logrus.Fields{
			"function":    "ReusePortTransport.Send",
			"target_addr": addr.String(),
			"error":       err.Error(),
		}).Debug("Failed to send packet")
		return err
	}

	atomic.AddUint64(&t.packetsSent, 1)
	atomic.AddUint64(&t.bytesSent, uint64(len(data)))
	return nil
}

// processPackets reads and processes packets from a single socket.
func (t *ReusePortTransport) processPackets(socketID int, conn net.PacketConn) {
	// UDP max payload: 65535 - 20 (IP) - 8 (UDP) = 65507 bytes
	// Use 2048 to match existing UDP transport
	const maxPacketSize = 2048
	buffer := make([]byte, maxPacketSize)

	logrus.WithFields(logrus.Fields{
		"function":  "ReusePortTransport.processPackets",
		"socket_id": socketID,
	}).Debug("Starting packet processing loop")

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			t.processIncomingPacket(socketID, conn, buffer)
		}
	}
}

// processIncomingPacket reads and processes a single packet.
func (t *ReusePortTransport) processIncomingPacket(socketID int, conn net.PacketConn, buffer []byte) {
	// Set read deadline for non-blocking reads with timeout
	if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		// Log but continue
		logrus.WithFields(logrus.Fields{
			"function":  "ReusePortTransport.processIncomingPacket",
			"socket_id": socketID,
			"error":     err.Error(),
		}).Debug("Failed to set read deadline")
	}

	n, addr, err := conn.ReadFrom(buffer)
	if err != nil {
		if t.ctx.Err() != nil {
			return // Context cancelled, shutting down
		}
		// Check for timeout
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return // Just a timeout, continue
		}
		atomic.AddUint64(&t.receiveErrors, 1)
		return
	}

	atomic.AddUint64(&t.packetsReceived, 1)
	atomic.AddUint64(&t.bytesReceived, uint64(n))

	packet, err := ParsePacket(buffer[:n])
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "ReusePortTransport.processIncomingPacket",
			"socket_id": socketID,
			"error":     err.Error(),
		}).Debug("Failed to parse packet")
		return
	}

	t.dispatchPacket(packet, addr)
}

// dispatchPacket dispatches a packet to the appropriate handler.
func (t *ReusePortTransport) dispatchPacket(packet *Packet, addr net.Addr) {
	t.mu.RLock()
	handler, exists := t.handlers[packet.PacketType]
	t.mu.RUnlock()

	if !exists {
		return
	}

	if t.workerPool != nil {
		// Use worker pool for bounded concurrency
		t.workerPool.Submit(packet, addr, handler)
	} else {
		// Legacy unbounded goroutine spawning
		go func(p *Packet, a net.Addr, h PacketHandler) {
			if err := h(p, a); err != nil {
				logrus.WithFields(logrus.Fields{
					"function":    "ReusePortTransport.dispatchPacket",
					"packet_type": p.PacketType,
					"error":       err.Error(),
				}).Debug("Handler error")
			}
		}(packet, addr, handler)
	}
}

// LocalAddr returns the local address the transport is listening on.
func (t *ReusePortTransport) LocalAddr() net.Addr {
	return t.listenAddr
}

// Close stops the transport and closes all sockets.
func (t *ReusePortTransport) Close() error {
	t.cancel()

	if t.workerPool != nil {
		t.workerPool.Stop()
	}

	var firstErr error
	for _, conn := range t.sockets {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	logrus.WithFields(logrus.Fields{
		"function":    "ReusePortTransport.Close",
		"num_sockets": len(t.sockets),
	}).Info("Transport closed")

	return firstErr
}

// Stats returns transport statistics.
func (t *ReusePortTransport) Stats() ReusePortStats {
	stats := ReusePortStats{
		NumSockets:      len(t.sockets),
		PacketsReceived: atomic.LoadUint64(&t.packetsReceived),
		PacketsSent:     atomic.LoadUint64(&t.packetsSent),
		BytesReceived:   atomic.LoadUint64(&t.bytesReceived),
		BytesSent:       atomic.LoadUint64(&t.bytesSent),
		SendErrors:      atomic.LoadUint64(&t.sendErrors),
		ReceiveErrors:   atomic.LoadUint64(&t.receiveErrors),
	}

	if t.workerPool != nil {
		wpStats := t.workerPool.Stats()
		stats.WorkerPoolStats = &wpStats
	}

	return stats
}

// IsConnectionOriented returns false since UDP is connectionless.
func (t *ReusePortTransport) IsConnectionOriented() bool {
	return false
}

// SupportedNetworks returns the list of supported network types.
func (t *ReusePortTransport) SupportedNetworks() []string {
	return []string{"udp", "udp4", "udp6"}
}
