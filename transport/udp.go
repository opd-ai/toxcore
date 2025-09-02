package transport

import (
	"context"
	"net"
	"sync"
	"time"
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
	// Use net.ListenPacket instead of net.ListenUDP for more abstraction
	conn, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
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

	// Start packet processing loop
	go transport.processPackets()

	return transport, nil
}

// RegisterHandler registers a handler for a specific packet type.
func (t *UDPTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.handlers[packetType] = handler
}

// Send sends a packet to the specified address.
//
//export ToxUDPSend
func (t *UDPTransport) Send(packet *Packet, addr net.Addr) error {
	data, err := packet.Serialize()
	if err != nil {
		return err
	}

	_, err = t.conn.WriteTo(data, addr)
	return err
}

// Close shuts down the transport.
//
//export ToxUDPClose
func (t *UDPTransport) Close() error {
	t.cancel()
	return t.conn.Close()
}

// processPackets handles incoming packets.
func (t *UDPTransport) processPackets() {
	buffer := make([]byte, 2048)

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			t.processIncomingPacket(buffer)
		}
	}
}

// processIncomingPacket reads and processes a single incoming packet.
func (t *UDPTransport) processIncomingPacket(buffer []byte) {
	data, addr, err := t.readPacketData(buffer)
	if err != nil {
		return // Error already handled in readPacketData
	}

	packet, err := t.parsePacketData(data)
	if err != nil {
		return // Error already handled in parsePacketData
	}

	t.dispatchPacketToHandler(packet, addr)
}

// readPacketData reads data from the connection with timeout handling.
func (t *UDPTransport) readPacketData(buffer []byte) ([]byte, net.Addr, error) {
	// Set read deadline for non-blocking reads with timeout
	_ = t.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

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
		return err
	}
	if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "message too long" {
		// Packet larger than buffer, log and discard
		return err
	}
	// Log other errors here
	return err
}

// parsePacketData parses raw packet data into a structured packet.
func (t *UDPTransport) parsePacketData(data []byte) (*Packet, error) {
	packet, err := ParsePacket(data)
	if err != nil {
		// Log error but continue processing other packets
		return nil, err
	}
	return packet, nil
}

// dispatchPacketToHandler finds and executes the appropriate packet handler.
func (t *UDPTransport) dispatchPacketToHandler(packet *Packet, addr net.Addr) {
	t.mu.RLock()
	handler, exists := t.handlers[packet.PacketType]
	t.mu.RUnlock()

	if exists {
		go handler(packet, addr) // Handle packet in separate goroutine
	}
}

// LocalAddr returns the local address the transport is listening on.
//
//export ToxUDPLocalAddr
func (t *UDPTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}
