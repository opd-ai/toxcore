package transport

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// UDPTransport implements UDP-based communication for the Tox protocol.
// It satisfies the Transport interface.
type UDPTransport struct {
	conn       net.PacketConn // Already using interface type
	listenAddr net.Addr       // Changed from *net.UDPAddr to net.Addr
	handlers   map[PacketType]PacketHandler
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// PacketHandler is a function that processes incoming packets.
type PacketHandler func(packet *Packet, addr net.Addr) error

// NewUDPTransport creates a new UDP transport listener.
//
//export ToxNewUDPTransport
func NewUDPTransport(listenAddr string) (Transport, error) {
	logrus.WithFields(logrus.Fields{
		"function":    "NewUDPTransport",
		"listen_addr": listenAddr,
	}).Info("Creating new UDP transport")

	// Use net.ListenPacket instead of net.ListenUDP for more abstraction
	conn, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "NewUDPTransport",
			"listen_addr": listenAddr,
			"error":       err.Error(),
		}).Error("Failed to create UDP listener")
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	transport := &UDPTransport{
		conn:       conn,
		listenAddr: conn.LocalAddr(), // Store the actual local address
		handlers:   make(map[PacketType]PacketHandler),
		ctx:        ctx,
		cancel:     cancel,
	}

	logrus.WithFields(logrus.Fields{
		"function":      "NewUDPTransport",
		"listen_addr":   listenAddr,
		"actual_addr":   conn.LocalAddr().String(),
		"handler_count": 0,
	}).Info("UDP transport created successfully")

	// Start packet processing loop
	go transport.processPackets()

	logrus.WithFields(logrus.Fields{
		"function":   "NewUDPTransport",
		"local_addr": conn.LocalAddr().String(),
	}).Info("UDP transport initialization completed")

	return transport, nil
}

// RegisterHandler registers a handler for a specific packet type.
func (t *UDPTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	logrus.WithFields(logrus.Fields{
		"function":    "RegisterHandler",
		"packet_type": packetType,
		"local_addr":  t.listenAddr.String(),
	}).Debug("Registering packet handler")

	t.mu.Lock()
	defer t.mu.Unlock()

	t.handlers[packetType] = handler

	logrus.WithFields(logrus.Fields{
		"function":      "RegisterHandler",
		"packet_type":   packetType,
		"handler_count": len(t.handlers),
		"local_addr":    t.listenAddr.String(),
	}).Info("Packet handler registered successfully")
}

// Send sends a packet to the specified address.
//
//export ToxUDPSend
func (t *UDPTransport) Send(packet *Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":    "Send",
		"packet_type": packet.PacketType,
		"dest_addr":   addr.String(),
		"local_addr":  t.listenAddr.String(),
	}).Debug("Sending UDP packet")

	data, err := packet.Serialize()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "Send",
			"packet_type": packet.PacketType,
			"dest_addr":   addr.String(),
			"error":       err.Error(),
		}).Error("Failed to serialize packet")
		return err
	}

	n, err := t.conn.WriteTo(data, addr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "Send",
			"packet_type": packet.PacketType,
			"dest_addr":   addr.String(),
			"data_size":   len(data),
			"error":       err.Error(),
		}).Error("Failed to send UDP packet")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function":    "Send",
		"packet_type": packet.PacketType,
		"dest_addr":   addr.String(),
		"bytes_sent":  n,
		"data_size":   len(data),
	}).Debug("UDP packet sent successfully")

	return nil
}

// Close shuts down the transport.
//
//export ToxUDPClose
func (t *UDPTransport) Close() error {
	logrus.WithFields(logrus.Fields{
		"function":   "Close",
		"local_addr": t.listenAddr.String(),
	}).Info("Closing UDP transport")

	t.cancel()
	err := t.conn.Close()

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "Close",
			"local_addr": t.listenAddr.String(),
			"error":      err.Error(),
		}).Error("Error closing UDP connection")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":   "Close",
			"local_addr": t.listenAddr.String(),
		}).Info("UDP transport closed successfully")
	}

	return err
}

// IsConnectionOriented returns false for UDP transport (connectionless protocol).
func (t *UDPTransport) IsConnectionOriented() bool {
	return false
}

// processPackets handles incoming packets.
func (t *UDPTransport) processPackets() {
	logrus.WithFields(logrus.Fields{
		"function":   "processPackets",
		"local_addr": t.listenAddr.String(),
	}).Info("Starting UDP packet processing loop")

	buffer := make([]byte, 2048)

	for {
		select {
		case <-t.ctx.Done():
			logrus.WithFields(logrus.Fields{
				"function":   "processPackets",
				"local_addr": t.listenAddr.String(),
			}).Info("UDP packet processing loop terminated")
			return
		default:
			t.processIncomingPacket(buffer)
		}
	}
}

// processIncomingPacket reads and processes a single incoming packet.
func (t *UDPTransport) processIncomingPacket(buffer []byte) {
	logrus.WithFields(logrus.Fields{
		"function":    "processIncomingPacket",
		"buffer_size": len(buffer),
	}).Debug("Processing incoming UDP packet")

	data, addr, err := t.readPacketData(buffer)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "processIncomingPacket",
			"error":    err.Error(),
		}).Debug("Failed to read packet data")
		return // Error already handled in readPacketData
	}

	packet, err := t.parsePacketData(data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "processIncomingPacket",
			"source_addr": addr.String(),
			"data_size":   len(data),
			"error":       err.Error(),
		}).Debug("Failed to parse packet data")
		return // Error already handled in parsePacketData
	}

	logrus.WithFields(logrus.Fields{
		"function":    "processIncomingPacket",
		"source_addr": addr.String(),
		"packet_type": packet.PacketType,
		"data_size":   len(data),
	}).Debug("Successfully processed incoming packet")

	t.dispatchPacketToHandler(packet, addr)
}

// readPacketData reads data from the connection with timeout handling.
func (t *UDPTransport) readPacketData(buffer []byte) ([]byte, net.Addr, error) {
	// Set read deadline for non-blocking reads with timeout
	if err := t.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		logrus.WithError(err).Warn("Failed to set read deadline on UDP connection")
	}

	n, addr, err := t.conn.ReadFrom(buffer)
	if err != nil {
		return nil, nil, t.handleReadError(err)
	}

	return buffer[:n], addr, nil
}

// handleReadError processes different types of connection read errors.
func (t *UDPTransport) handleReadError(err error) error {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		// This is just a timeout, continue
		logrus.WithFields(logrus.Fields{
			"function": "handleReadError",
			"error":    "timeout",
		}).Debug("UDP read timeout (normal operation)")
		return err
	}
	// Check for message too long error by inspecting error string
	if strings.Contains(err.Error(), "message too long") {
		// Packet larger than buffer, log and discard
		logrus.WithFields(logrus.Fields{
			"function": "handleReadError",
			"error":    "message too long",
		}).Warn("UDP packet too large for buffer, discarding")
		return err
	}
	// Log other errors here
	logrus.WithFields(logrus.Fields{
		"function": "handleReadError",
		"error":    err.Error(),
	}).Error("UDP read error")
	return err
}

// parsePacketData parses raw packet data into a structured packet.
func (t *UDPTransport) parsePacketData(data []byte) (*Packet, error) {
	logrus.WithFields(logrus.Fields{
		"function":  "parsePacketData",
		"data_size": len(data),
	}).Debug("Parsing packet data")

	packet, err := ParsePacket(data)
	if err != nil {
		// Log error but continue processing other packets
		logrus.WithFields(logrus.Fields{
			"function":  "parsePacketData",
			"data_size": len(data),
			"error":     err.Error(),
		}).Debug("Failed to parse packet")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"function":    "parsePacketData",
		"packet_type": packet.PacketType,
		"data_size":   len(data),
	}).Debug("Successfully parsed packet")

	return packet, nil
}

// dispatchPacketToHandler finds and executes the appropriate packet handler.
func (t *UDPTransport) dispatchPacketToHandler(packet *Packet, addr net.Addr) {
	logrus.WithFields(logrus.Fields{
		"function":    "dispatchPacketToHandler",
		"packet_type": packet.PacketType,
		"source_addr": addr.String(),
	}).Debug("Dispatching packet to handler")

	t.mu.RLock()
	handler, exists := t.handlers[packet.PacketType]
	handlerCount := len(t.handlers)
	t.mu.RUnlock()

	if exists {
		logrus.WithFields(logrus.Fields{
			"function":      "dispatchPacketToHandler",
			"packet_type":   packet.PacketType,
			"source_addr":   addr.String(),
			"handler_count": handlerCount,
		}).Debug("Handler found, processing packet in goroutine")
		go handler(packet, addr) // Handle packet in separate goroutine
	} else {
		logrus.WithFields(logrus.Fields{
			"function":      "dispatchPacketToHandler",
			"packet_type":   packet.PacketType,
			"source_addr":   addr.String(),
			"handler_count": handlerCount,
		}).Debug("No handler registered for packet type")
	}
}

// LocalAddr returns the local address the transport is listening on.
//
//export ToxUDPLocalAddr
func (t *UDPTransport) LocalAddr() net.Addr {
	logrus.WithFields(logrus.Fields{
		"function":   "LocalAddr",
		"local_addr": t.listenAddr.String(),
	}).Debug("Returning local address")
	return t.conn.LocalAddr()
}
