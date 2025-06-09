// Package transport implements network transport layers for the Tox protocol.
// This file provides TCP-based transport implementation with connection management,
// packet handling, and support for persistent client connections.
//
// The TCP transport offers:
//   - Reliable stream-based communication
//   - Persistent client connection management
//   - Concurrent connection handling
//   - Graceful connection cleanup and shutdown
//   - Stream framing for packet boundaries
//
// TCP transport is used for Tox relay connections, file transfers,
// and scenarios where reliability is prioritized over latency.
//
// Example usage:
//
//	// Create TCP transport
//	transport, err := NewTCPTransport(":33445")
//	if err != nil {
//	    panic(err)
//	}
//	defer transport.Close()
//
//	// Register packet handler
//	transport.RegisterHandler(PacketTypeFile, func(packet *Packet, addr net.Addr) error {
//	    // Process file transfer packet
//	    return nil
//	})

package transport

import (
	"context"
	"net"
	"sync"
	"time"
)

// ADDED: TCPTransport implements TCP-based communication for the Tox protocol.
// This structure provides a complete TCP transport layer that satisfies the
// Transport interface. It manages persistent client connections, handles
// connection acceptance, and processes packets over reliable streams.
//
// The transport maintains active client connections and automatically
// handles connection lifecycle including cleanup of disconnected clients.
// It provides stream framing to maintain packet boundaries over TCP.
//
//export ToxTCPTransport
type TCPTransport struct {
	listener   net.Listener                 // ADDED: TCP listener for incoming connections
	listenAddr net.Addr                     // ADDED: Local listening address
	handlers   map[PacketType]PacketHandler // ADDED: Packet type to handler mappings
	clients    map[string]net.Conn          // ADDED: Active client connections by address
	mu         sync.RWMutex                 // ADDED: Protects clients map and handlers
	ctx        context.Context              // ADDED: Context for graceful shutdown
	cancel     context.CancelFunc           // ADDED: Cancel function for shutdown
}

// ADDED: NewTCPTransport creates and initializes a new TCP transport listener.
// This function sets up a TCP listener on the specified address and starts
// the connection acceptance loop in a separate goroutine. The transport
// manages persistent connections and handles stream framing automatically.
//
// Parameters:
//   - listenAddr: The address to bind the TCP listener to (e.g., ":33445", "0.0.0.0:33445")
//
// Returns a Transport interface implementation and any error encountered.
//
//export ToxNewTCPTransport
func NewTCPTransport(listenAddr string) (Transport, error) {
	// ADDED: Create TCP listener on specified address
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}

	// ADDED: Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	transport := &TCPTransport{
		listener:   listener,
		listenAddr: listener.Addr(), // ADDED: Store actual listening address
		handlers:   make(map[PacketType]PacketHandler),
		clients:    make(map[string]net.Conn), // ADDED: Initialize client connection map
		ctx:        ctx,
		cancel:     cancel,
	}

	// ADDED: Start connection acceptance loop in background goroutine
	go transport.acceptConnections()

	return transport, nil
}

// ADDED: RegisterHandler registers a packet handler for a specific packet type.
// This method associates a PacketHandler function with a particular PacketType,
// enabling automatic routing of incoming packets from TCP streams.
// Handlers are called concurrently in separate goroutines.
//
// Thread safety: This method uses write locking to safely modify the handlers map.
//
// Parameters:
//   - packetType: The PacketType to handle
//   - handler: The PacketHandler function to process packets of this type
func (t *TCPTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.handlers[packetType] = handler // ADDED: Store handler with thread safety
}

// ADDED: Send transmits a packet to the specified address over a TCP connection.
// This method manages TCP connections automatically, establishing new connections
// as needed and reusing existing ones. It uses stream framing with length prefixes
// to maintain packet boundaries over the TCP stream.
//
// The method handles connection lifecycle including cleanup on errors and
// concurrent access to the client connection map.
//
// Parameters:
//   - packet: The Packet to send
//   - addr: The destination network address
//
// Returns an error if connection establishment, serialization, or transmission fails.
func (t *TCPTransport) Send(packet *Packet, addr net.Addr) error {
	// ADDED: Check for existing connection with read lock
	t.mu.RLock()
	conn, exists := t.clients[addr.String()]
	t.mu.RUnlock()

	if !exists {
		// ADDED: Establish new connection if none exists
		var err error
		conn, err = net.Dial("tcp", addr.String())
		if err != nil {
			return err
		}

		// ADDED: Store new connection in client map with write lock
		t.mu.Lock()
		t.clients[addr.String()] = conn
		t.mu.Unlock()

		// ADDED: Start handling incoming data from this connection
		go t.handleConnection(conn)
	}

	// ADDED: Serialize packet to binary format
	data, err := packet.Serialize()
	if err != nil {
		return err
	}

	// ADDED: Create length prefix for stream framing (4 bytes big-endian)
	prefix := make([]byte, 4)
	prefix[0] = byte(len(data) >> 24)
	prefix[1] = byte(len(data) >> 16)
	prefix[2] = byte(len(data) >> 8)
	prefix[3] = byte(len(data))

	// ADDED: Set write deadline to prevent hanging
	err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return err
	}

	// ADDED: Write length prefix first
	_, err = conn.Write(prefix)
	if err != nil {
		// ADDED: Clean up connection on write error
		t.mu.Lock()
		delete(t.clients, addr.String())
		t.mu.Unlock()
		conn.Close()
		return err
	}

	// ADDED: Write packet data
	_, err = conn.Write(data)
	if err != nil {
		// ADDED: Clean up connection on write error
		t.mu.Lock()
		delete(t.clients, addr.String())
		t.mu.Unlock()
		conn.Close()
		return err
	}

	return nil
}

// ADDED: Close shuts down the TCP transport and releases all resources.
// This method cancels the context to stop accepting new connections,
// closes all active client connections, and shuts down the listener.
// After calling Close, the transport should not be used further.
//
// Returns an error if the listener close operation fails.
func (t *TCPTransport) Close() error {
	t.cancel() // ADDED: Cancel context to stop connection acceptance

	// ADDED: Close all active client connections
	t.mu.Lock()
	for _, conn := range t.clients {
		conn.Close()
	}
	t.mu.Unlock()

	return t.listener.Close()
}

// ADDED: LocalAddr returns the local network address the transport is listening on.
// This method provides access to the actual address bound by the TCP listener,
// which may differ from the requested address (e.g., when binding to ":0"
// results in an automatically assigned port).
//
// Returns the local network address of the TCP listener.
func (t *TCPTransport) LocalAddr() net.Addr {
	return t.listenAddr // ADDED: Return stored listening address
}

// ADDED: acceptConnections runs the main connection acceptance loop for the TCP transport.
// This method continuously accepts incoming connections and spawns goroutines
// to handle each connection. It runs until the context is cancelled during
// transport shutdown, providing graceful termination of the acceptance loop.
//
// Each accepted connection is handled in a separate goroutine to maintain
// high concurrency and prevent blocking on slow connections.
func (t *TCPTransport) acceptConnections() {
	for {
		select {
		case <-t.ctx.Done():
			return // ADDED: Exit loop when context is cancelled
		default:
			conn, err := t.listener.Accept()
			if err != nil {
				continue // ADDED: Log accept errors and continue accepting
			}

			// ADDED: Handle each connection in separate goroutine for concurrency
			go t.handleConnection(conn)
		}
	}
}

// ADDED: handleConnection processes data from a single TCP connection.
// This method manages the complete lifecycle of a TCP connection including
// client registration, stream reading with framing, packet parsing, and
// cleanup. It reads length-prefixed packets from the TCP stream and
// dispatches them to appropriate handlers.
//
// The method handles stream framing by reading a 4-byte length prefix
// followed by the packet data, ensuring proper packet boundaries over
// the TCP stream. Connection cleanup is performed automatically on
// errors or when the connection is closed.
//
// Parameters:
//   - conn: The TCP connection to handle
func (t *TCPTransport) handleConnection(conn net.Conn) {
	defer conn.Close() // ADDED: Ensure connection is closed on function exit

	addr := conn.RemoteAddr()

	// ADDED: Register connection in client map
	t.mu.Lock()
	t.clients[addr.String()] = conn
	t.mu.Unlock()

	// ADDED: Ensure connection cleanup on function exit
	defer func() {
		t.mu.Lock()
		delete(t.clients, addr.String())
		t.mu.Unlock()
	}()

	// ADDED: Read packets in a loop with stream framing
	header := make([]byte, 4) // Buffer for length prefix
	for {
		// ADDED: Read 4-byte length prefix for stream framing
		_, err := conn.Read(header)
		if err != nil {
			return // Connection closed or read error
		}

		// ADDED: Decode length from big-endian 4-byte prefix
		length := (uint32(header[0]) << 24) |
			(uint32(header[1]) << 16) |
			(uint32(header[2]) << 8) |
			uint32(header[3])

		// ADDED: Read packet data based on length prefix
		data := make([]byte, length)
		_, err = conn.Read(data)
		if err != nil {
			return // Read error or connection closed
		}

		// ADDED: Parse packet from received data
		packet, err := ParsePacket(data)
		if err != nil {
			continue // Skip malformed packets and continue processing
		}

		// ADDED: Find and invoke appropriate handler for packet type
		t.mu.RLock()
		handler, exists := t.handlers[packet.PacketType]
		t.mu.RUnlock()

		if exists {
			// ADDED: Handle packet in separate goroutine with error handling
			go func(p *Packet, a net.Addr) {
				if err := handler(p, a); err != nil {
					// Log handler errors for debugging
				}
			}(packet, addr)
		}
	}
}
