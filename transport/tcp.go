package transport

import (
	"context"
	"net"
	"sync"
	"time"
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
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
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

	// Start accepting connections
	go transport.acceptConnections()

	return transport, nil
}

// RegisterHandler registers a handler for a specific packet type.
func (t *TCPTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.handlers[packetType] = handler
}

// Send sends a packet to the specified address.
func (t *TCPTransport) Send(packet *Packet, addr net.Addr) error {
	t.mu.RLock()
	conn, exists := t.clients[addr.String()]
	t.mu.RUnlock()

	if !exists {
		// Try to establish a connection if none exists
		var err error
		conn, err = net.Dial("tcp", addr.String())
		if err != nil {
			return err
		}

		// Store the new connection
		t.mu.Lock()
		t.clients[addr.String()] = conn
		t.mu.Unlock()

		// Handle incoming data from this connection
		go t.handleConnection(conn)
	}

	// Serialize packet
	data, err := packet.Serialize()
	if err != nil {
		return err
	}

	// Send data length prefix (4 bytes) followed by data
	prefix := make([]byte, 4)
	prefix[0] = byte(len(data) >> 24)
	prefix[1] = byte(len(data) >> 16)
	prefix[2] = byte(len(data) >> 8)
	prefix[3] = byte(len(data))

	// Set write deadline
	err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return err
	}

	// Write length prefix
	_, err = conn.Write(prefix)
	if err != nil {
		// Remove connection on error
		t.mu.Lock()
		delete(t.clients, addr.String())
		t.mu.Unlock()
		conn.Close()
		return err
	}

	// Write data
	_, err = conn.Write(data)
	if err != nil {
		// Remove connection on error
		t.mu.Lock()
		delete(t.clients, addr.String())
		t.mu.Unlock()
		conn.Close()
		return err
	}

	return nil
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

	t.mu.Lock()
	t.clients[addr.String()] = conn
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.clients, addr.String())
		t.mu.Unlock()
	}()

	// Read packets in a loop
	header := make([]byte, 4)
	for {
		// Read packet length
		_, err := conn.Read(header)
		if err != nil {
			// Connection closed or error
			return
		}

		length := (uint32(header[0]) << 24) |
			(uint32(header[1]) << 16) |
			(uint32(header[2]) << 8) |
			uint32(header[3])

		// Read packet data
		data := make([]byte, length)
		_, err = conn.Read(data)
		if err != nil {
			return
		}

		// Parse and handle packet
		packet, err := ParsePacket(data)
		if err != nil {
			continue
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
}
