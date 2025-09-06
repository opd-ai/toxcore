package transport

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TCPTransport implements TCP-based communication for the Tox protocol.
// It satisfies the Transport interface.
type TCPTransport struct {
	listener   net.Listener
	listenAddr net.Addr
	handlers   map[PacketType]PacketHandler
	clients    map[string]net.Conn
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewTCPTransport creates a new TCP transport listener.
//
//export ToxNewTCPTransport
func NewTCPTransport(listenAddr string) (Transport, error) {
	logrus.WithFields(logrus.Fields{
		"function":    "NewTCPTransport",
		"listen_addr": listenAddr,
	}).Info("Creating new TCP transport")

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "NewTCPTransport",
			"listen_addr": listenAddr,
			"error":       err.Error(),
		}).Error("Failed to create TCP listener")
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	transport := &TCPTransport{
		listener:   listener,
		listenAddr: listener.Addr(),
		handlers:   make(map[PacketType]PacketHandler),
		clients:    make(map[string]net.Conn),
		ctx:        ctx,
		cancel:     cancel,
	}

	logrus.WithFields(logrus.Fields{
		"function":     "NewTCPTransport",
		"listen_addr":  listenAddr,
		"actual_addr":  listener.Addr().String(),
		"client_count": 0,
	}).Info("TCP transport created successfully")

	// Start accepting connections
	go transport.acceptConnections()

	logrus.WithFields(logrus.Fields{
		"function":   "NewTCPTransport",
		"local_addr": listener.Addr().String(),
	}).Info("TCP transport initialization completed")

	return transport, nil
}

// RegisterHandler registers a handler for a specific packet type.
func (t *TCPTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	logrus.WithFields(logrus.Fields{
		"function":    "RegisterHandler",
		"packet_type": packetType,
		"local_addr":  t.listenAddr.String(),
	}).Debug("Registering TCP packet handler")

	t.mu.Lock()
	defer t.mu.Unlock()

	t.handlers[packetType] = handler

	logrus.WithFields(logrus.Fields{
		"function":      "RegisterHandler",
		"packet_type":   packetType,
		"handler_count": len(t.handlers),
		"local_addr":    t.listenAddr.String(),
	}).Info("TCP packet handler registered successfully")
}

// Send sends a packet to the specified address.
func (t *TCPTransport) Send(packet *Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":    "Send",
		"packet_type": packet.PacketType,
		"dest_addr":   addr.String(),
		"local_addr":  t.listenAddr.String(),
	}).Debug("Sending TCP packet")

	conn, err := t.getOrCreateConnection(addr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "Send",
			"packet_type": packet.PacketType,
			"dest_addr":   addr.String(),
			"error":       err.Error(),
		}).Error("Failed to get or create TCP connection")
		return err
	}

	data, err := t.serializePacket(packet)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "Send",
			"packet_type": packet.PacketType,
			"dest_addr":   addr.String(),
			"error":       err.Error(),
		}).Error("Failed to serialize packet for TCP")
		return err
	}

	err = t.writePacketToConnection(conn, addr, data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "Send",
			"packet_type": packet.PacketType,
			"dest_addr":   addr.String(),
			"data_size":   len(data),
			"error":       err.Error(),
		}).Error("Failed to write packet to TCP connection")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":    "Send",
			"packet_type": packet.PacketType,
			"dest_addr":   addr.String(),
			"data_size":   len(data),
		}).Debug("TCP packet sent successfully")
	}

	return err
}

// getOrCreateConnection retrieves an existing connection or creates a new one for the given address.
func (t *TCPTransport) getOrCreateConnection(addr net.Addr) (net.Conn, error) {
	addrKey := addr.String()

	logrus.WithFields(logrus.Fields{
		"function":  "getOrCreateConnection",
		"dest_addr": addrKey,
	}).Debug("Getting or creating TCP connection")

	t.mu.RLock()
	conn, exists := t.clients[addrKey]
	clientCount := len(t.clients)
	t.mu.RUnlock()

	if exists {
		logrus.WithFields(logrus.Fields{
			"function":     "getOrCreateConnection",
			"dest_addr":    addrKey,
			"client_count": clientCount,
		}).Debug("Using existing TCP connection")
		return conn, nil
	}

	logrus.WithFields(logrus.Fields{
		"function":     "getOrCreateConnection",
		"dest_addr":    addrKey,
		"client_count": clientCount,
	}).Info("Creating new TCP connection")

	// Try to establish a connection if none exists
	newConn, err := net.Dial("tcp", addrKey)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "getOrCreateConnection",
			"dest_addr": addrKey,
			"error":     err.Error(),
		}).Error("Failed to establish TCP connection")
		return nil, err
	}

	// Store the new connection
	t.mu.Lock()
	t.clients[addrKey] = newConn
	newClientCount := len(t.clients)
	t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":     "getOrCreateConnection",
		"dest_addr":    addrKey,
		"client_count": newClientCount,
		"remote_addr":  newConn.RemoteAddr().String(),
		"local_addr":   newConn.LocalAddr().String(),
	}).Info("TCP connection established successfully")

	// Handle incoming data from this connection
	go t.handleConnection(newConn)

	return newConn, nil
}

// serializePacket converts a packet to its byte representation.
func (t *TCPTransport) serializePacket(packet *Packet) ([]byte, error) {
	return packet.Serialize()
}

// writePacketToConnection writes packet data to the connection with length prefixing and error handling.
func (t *TCPTransport) writePacketToConnection(conn net.Conn, addr net.Addr, data []byte) error {
	// Set write deadline
	err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return err
	}

	// Create and write length prefix (4 bytes)
	prefix := t.createLengthPrefix(data)
	if err := t.writeWithCleanup(conn, addr, prefix); err != nil {
		return err
	}

	// Write packet data
	return t.writeWithCleanup(conn, addr, data)
}

// createLengthPrefix creates a 4-byte length prefix for the data.
func (t *TCPTransport) createLengthPrefix(data []byte) []byte {
	prefix := make([]byte, 4)
	dataLen := len(data)
	prefix[0] = byte(dataLen >> 24)
	prefix[1] = byte(dataLen >> 16)
	prefix[2] = byte(dataLen >> 8)
	prefix[3] = byte(dataLen)
	return prefix
}

// writeWithCleanup writes data to connection and cleans up on error.
func (t *TCPTransport) writeWithCleanup(conn net.Conn, addr net.Addr, data []byte) error {
	_, err := conn.Write(data)
	if err != nil {
		t.cleanupConnection(conn, addr)
		return err
	}
	return nil
}

// cleanupConnection removes connection from clients map and closes it.
func (t *TCPTransport) cleanupConnection(conn net.Conn, addr net.Addr) {
	t.mu.Lock()
	delete(t.clients, addr.String())
	t.mu.Unlock()
	conn.Close()
}

// Close shuts down the transport.
func (t *TCPTransport) Close() error {
	t.cancel()

	// Close all client connections
	t.mu.Lock()
	for _, conn := range t.clients {
		conn.Close()
	}
	t.mu.Unlock()

	return t.listener.Close()
}

// LocalAddr returns the local address the transport is listening on.
func (t *TCPTransport) LocalAddr() net.Addr {
	return t.listenAddr
}

// acceptConnections handles incoming connections.
func (t *TCPTransport) acceptConnections() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			conn, err := t.listener.Accept()
			if err != nil {
				// Log accept errors here
				continue
			}

			// Handle the connection in a new goroutine
			go t.handleConnection(conn)
		}
	}
}

// handleConnection processes data from a single TCP connection.
func (t *TCPTransport) handleConnection(conn net.Conn) {
	defer conn.Close()

	addr := conn.RemoteAddr()
	t.registerClient(addr, conn)
	defer t.unregisterClient(addr)

	// Process packets continuously
	t.processPacketLoop(conn, addr)
}

// registerClient adds a new client connection to the transport.
func (t *TCPTransport) registerClient(addr net.Addr, conn net.Conn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clients[addr.String()] = conn
}

// unregisterClient removes a client connection from the transport.
func (t *TCPTransport) unregisterClient(addr net.Addr) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.clients, addr.String())
}

// processPacketLoop continuously reads and processes packets from a connection.
func (t *TCPTransport) processPacketLoop(conn net.Conn, addr net.Addr) {
	header := make([]byte, 4)
	for {
		// Read and parse packet length
		length, err := t.readPacketLength(conn, header)
		if err != nil {
			return
		}

		// Read packet data
		data, err := t.readPacketData(conn, length)
		if err != nil {
			return
		}

		// Process the packet
		t.processPacket(data, addr)
	}
}

// readPacketLength reads the 4-byte packet length header and returns the parsed length.
func (t *TCPTransport) readPacketLength(conn net.Conn, header []byte) (uint32, error) {
	_, err := conn.Read(header)
	if err != nil {
		return 0, err
	}

	length := (uint32(header[0]) << 24) |
		(uint32(header[1]) << 16) |
		(uint32(header[2]) << 8) |
		uint32(header[3])

	return length, nil
}

// readPacketData reads packet data of the specified length from the connection.
func (t *TCPTransport) readPacketData(conn net.Conn, length uint32) ([]byte, error) {
	data := make([]byte, length)
	_, err := conn.Read(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// processPacket parses packet data and dispatches it to the appropriate handler.
func (t *TCPTransport) processPacket(data []byte, addr net.Addr) {
	packet, err := ParsePacket(data)
	if err != nil {
		return
	}

	t.mu.RLock()
	handler, exists := t.handlers[packet.PacketType]
	t.mu.RUnlock()

	if exists {
		go func(p *Packet, a net.Addr) {
			if err := handler(p, a); err != nil {
				// Log handler errors here
			}
		}(packet, addr)
	}
}
