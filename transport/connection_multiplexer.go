package transport

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ConnectionMultiplexer manages multiple connections over a single transport
//
//export ToxConnectionMultiplexer
type ConnectionMultiplexer struct {
	connections     map[string]*MultiplexedConnection
	transport       PacketConn
	listener        net.Listener
	mu              sync.RWMutex
	stopChannel     chan struct{}
	packetHandlers  map[PacketType]PacketHandler
	defaultHandler  PacketHandler
	stats           *MultiplexerStats
}

// MultiplexedConnection represents a logical connection within the multiplexer
//
//export ToxMultiplexedConnection
type MultiplexedConnection struct {
	ID              string
	RemoteAddr      net.Addr
	LocalAddr       net.Addr
	State           ConnectionState
	LastActivity    time.Time
	BytesSent       uint64
	BytesReceived   uint64
	PacketsSent     uint64
	PacketsReceived uint64
	ErrorCount      uint64
	SessionData     interface{} // For storing session-specific data
}

// ConnectionState represents the state of a multiplexed connection
type ConnectionState int

const (
	ConnectionStateIdle ConnectionState = iota
	ConnectionStateConnecting
	ConnectionStateConnected
	ConnectionStateDisconnecting
	ConnectionStateDisconnected
	ConnectionStateError
)

// MultiplexerStats tracks multiplexer performance metrics
//
//export ToxMultiplexerStats
type MultiplexerStats struct {
	TotalConnections    uint64
	ActiveConnections   uint64
	PacketsRouted       uint64
	RoutingErrors       uint64
	BytesTransferred    uint64
	ConnectionsCreated  uint64
	ConnectionsClosed   uint64
	LastActivity        time.Time
}

// PacketHandler processes packets for specific types
type PacketHandler func(*Packet, net.Addr) error

// NewConnectionMultiplexer creates a new connection multiplexer
//
//export ToxNewConnectionMultiplexer
func NewConnectionMultiplexer(transport PacketConn) *ConnectionMultiplexer {
	return &ConnectionMultiplexer{
		connections:    make(map[string]*MultiplexedConnection),
		transport:      transport,
		stopChannel:    make(chan struct{}),
		packetHandlers: make(map[PacketType]PacketHandler),
		stats: &MultiplexerStats{
			LastActivity: time.Now(),
		},
	}
}

// Start begins multiplexer operation
//
//export ToxMultiplexerStart
func (m *ConnectionMultiplexer) Start() error {
	go m.packetLoop()
	go m.maintenanceLoop()
	return nil
}

// Stop stops the multiplexer
//
//export ToxMultiplexerStop
func (m *ConnectionMultiplexer) Stop() error {
	close(m.stopChannel)
	return nil
}

// RegisterHandler registers a packet handler for a specific packet type
//
//export ToxRegisterHandler
func (m *ConnectionMultiplexer) RegisterHandler(packetType PacketType, handler PacketHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packetHandlers[packetType] = handler
}

// SetDefaultHandler sets the default packet handler
//
//export ToxSetDefaultHandler
func (m *ConnectionMultiplexer) SetDefaultHandler(handler PacketHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultHandler = handler
}

// CreateConnection creates a new logical connection
//
//export ToxCreateConnection
func (m *ConnectionMultiplexer) CreateConnection(remoteAddr net.Addr) (*MultiplexedConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	connID := generateConnectionID(remoteAddr)
	
	// Check if connection already exists
	if existing, exists := m.connections[connID]; exists {
		if existing.State == ConnectionStateConnected || existing.State == ConnectionStateConnecting {
			return existing, nil
		}
	}
	
	conn := &MultiplexedConnection{
		ID:           connID,
		RemoteAddr:   remoteAddr,
		LocalAddr:    m.transport.LocalAddr(),
		State:        ConnectionStateConnecting,
		LastActivity: time.Now(),
	}
	
	m.connections[connID] = conn
	m.stats.ConnectionsCreated++
	m.stats.TotalConnections++
	m.stats.ActiveConnections++
	
	return conn, nil
}

// GetConnection retrieves a connection by ID
//
//export ToxGetConnection
func (m *ConnectionMultiplexer) GetConnection(connID string) (*MultiplexedConnection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	conn, exists := m.connections[connID]
	return conn, exists
}

// CloseConnection closes a logical connection
//
//export ToxCloseConnection
func (m *ConnectionMultiplexer) CloseConnection(connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	conn, exists := m.connections[connID]
	if !exists {
		return fmt.Errorf("connection %s not found", connID)
	}
	
	conn.State = ConnectionStateDisconnected
	delete(m.connections, connID)
	
	m.stats.ConnectionsClosed++
	m.stats.ActiveConnections--
	
	return nil
}

// SendPacket sends a packet over a specific connection
//
//export ToxSendPacket
func (m *ConnectionMultiplexer) SendPacket(connID string, packet *Packet) error {
	m.mu.RLock()
	conn, exists := m.connections[connID]
	m.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("connection %s not found", connID)
	}
	
	err := m.transport.WriteTo(packet.Data, conn.RemoteAddr)
	if err != nil {
		conn.ErrorCount++
		return fmt.Errorf("failed to send packet: %w", err)
	}
	
	// Update connection stats
	m.mu.Lock()
	conn.BytesSent += uint64(len(packet.Data))
	conn.PacketsSent++
	conn.LastActivity = time.Now()
	m.stats.BytesTransferred += uint64(len(packet.Data))
	m.stats.LastActivity = time.Now()
	m.mu.Unlock()
	
	return nil
}

// packetLoop handles incoming packets
func (m *ConnectionMultiplexer) packetLoop() {
	buffer := make([]byte, 65535)
	
	for {
		select {
		case <-m.stopChannel:
			return
		default:
			// Set read timeout to avoid blocking indefinitely
			if udpConn, ok := m.transport.(*net.UDPConn); ok {
				udpConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			}
			
			n, addr, err := m.transport.ReadFrom(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout is expected
				}
				continue // Other errors are logged but don't stop the loop
			}
			
			// Parse packet
			packet, err := ParsePacket(buffer[:n])
			if err != nil {
				m.stats.RoutingErrors++
				continue
			}
			
			// Route packet to appropriate handler
			m.routePacket(packet, addr)
		}
	}
}

// routePacket routes packets to appropriate handlers
func (m *ConnectionMultiplexer) routePacket(packet *Packet, addr net.Addr) {
	m.mu.RLock()
	handler, exists := m.packetHandlers[packet.PacketType]
	defaultHandler := m.defaultHandler
	m.mu.RUnlock()
	
	// Update connection stats
	connID := generateConnectionID(addr)
	m.mu.Lock()
	if conn, exists := m.connections[connID]; exists {
		conn.BytesReceived += uint64(len(packet.Data))
		conn.PacketsReceived++
		conn.LastActivity = time.Now()
	}
	m.stats.PacketsRouted++
	m.stats.BytesTransferred += uint64(len(packet.Data))
	m.stats.LastActivity = time.Now()
	m.mu.Unlock()
	
	// Call appropriate handler
	if exists && handler != nil {
		if err := handler(packet, addr); err != nil {
			m.stats.RoutingErrors++
		}
	} else if defaultHandler != nil {
		if err := defaultHandler(packet, addr); err != nil {
			m.stats.RoutingErrors++
		}
	} else {
		m.stats.RoutingErrors++
	}
}

// maintenanceLoop performs periodic maintenance tasks
func (m *ConnectionMultiplexer) maintenanceLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.stopChannel:
			return
		case <-ticker.C:
			m.cleanupStaleConnections()
		}
	}
}

// cleanupStaleConnections removes inactive connections
func (m *ConnectionMultiplexer) cleanupStaleConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	staleThreshold := time.Now().Add(-5 * time.Minute)
	
	for connID, conn := range m.connections {
		if conn.LastActivity.Before(staleThreshold) && 
		   (conn.State == ConnectionStateDisconnected || conn.State == ConnectionStateError) {
			delete(m.connections, connID)
			m.stats.ConnectionsClosed++
			m.stats.ActiveConnections--
		}
	}
}

// GetStats returns current multiplexer statistics
//
//export ToxGetMultiplexerStats
func (m *ConnectionMultiplexer) GetStats() *MultiplexerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	statsCopy := *m.stats
	statsCopy.ActiveConnections = uint64(len(m.connections))
	
	return &statsCopy
}

// generateConnectionID creates a unique connection identifier
func generateConnectionID(addr net.Addr) string {
	return fmt.Sprintf("%s_%d", addr.Network(), time.Now().UnixNano())
}

// ListConnections returns all active connections
//
//export ToxListConnections
func (m *ConnectionMultiplexer) ListConnections() []*MultiplexedConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	connections := make([]*MultiplexedConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		// Create a copy to avoid race conditions
		connCopy := *conn
		connections = append(connections, &connCopy)
	}
	
	return connections
}
