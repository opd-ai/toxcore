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
			// Set read deadline for non-blocking reads with timeout
			_ = t.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

			n, addr, err := t.conn.ReadFrom(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// This is just a timeout, continue
					continue
				}
				// Log real errors here
				continue
			}

			if n < 1 {
				continue
			}

			// Parse the packet
			packet, err := ParsePacket(buffer[:n])
			if err != nil {
				// Log packet parsing errors here
				continue
			}

			// Handle the packet
			t.mu.RLock()
			handler, exists := t.handlers[packet.PacketType]
			t.mu.RUnlock()

			if exists {
				// Execute handler in a separate goroutine to avoid blocking
				go func(p *Packet, a net.Addr) {
					if err := handler(p, a); err != nil {
						// Log handler errors here
					}
				}(packet, addr)
			}
		}
	}
}

// LocalAddr returns the local address the transport is listening on.
//
//export ToxUDPLocalAddr
func (t *UDPTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}
