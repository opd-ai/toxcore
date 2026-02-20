// Package transport implements network transport for the Tox protocol.
//
// This file implements TCP relay support for NAT traversal when direct
// UDP connections fail, particularly for symmetric NAT scenarios.
package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// RelayState represents the current state of a relay connection.
type RelayState uint8

const (
	// RelayStateDisconnected means not connected to any relay.
	RelayStateDisconnected RelayState = iota
	// RelayStateConnecting means connection is in progress.
	RelayStateConnecting
	// RelayStateConnected means connected and ready for relay.
	RelayStateConnected
	// RelayStateFailed means connection attempt failed.
	RelayStateFailed
)

// RelayPacketType identifies relay protocol packet types.
type RelayPacketType uint8

const (
	// RelayPacketRouting is for routing requests between peers.
	RelayPacketRouting RelayPacketType = 0x00
	// RelayPacketData is for relayed data packets.
	RelayPacketData RelayPacketType = 0x01
	// RelayPacketPing is for keepalive ping.
	RelayPacketPing RelayPacketType = 0x02
	// RelayPacketPong is for keepalive pong response.
	RelayPacketPong RelayPacketType = 0x03
	// RelayPacketDisconnect notifies disconnection.
	RelayPacketDisconnect RelayPacketType = 0x04
)

// RelayServerInfo contains information about a TCP relay server.
type RelayServerInfo struct {
	Address   string
	PublicKey [32]byte
	Port      uint16
	Priority  int
}

// RelayClient provides TCP relay functionality for NAT traversal.
// It maintains connections to relay servers and routes packets through them.
//
//export ToxRelayClient
type RelayClient struct {
	localPublicKey  [32]byte
	servers         []RelayServerInfo
	activeConn      net.Conn
	activeServer    *RelayServerInfo
	state           RelayState
	handlers        map[RelayPacketType]func([]byte, net.Addr) error
	dataHandler     func(*Packet, net.Addr) error
	mu              sync.RWMutex
	timeout         time.Duration
	reconnectDelay  time.Duration
	maxReconnects   int
	ctx             context.Context
	cancel          context.CancelFunc
	keepaliveTicker *time.Ticker
	lastPong        time.Time
}

// NewRelayClient creates a new TCP relay client.
//
//export ToxNewRelayClient
func NewRelayClient(localPublicKey [32]byte) *RelayClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &RelayClient{
		localPublicKey: localPublicKey,
		servers:        make([]RelayServerInfo, 0),
		state:          RelayStateDisconnected,
		handlers:       make(map[RelayPacketType]func([]byte, net.Addr) error),
		timeout:        10 * time.Second,
		reconnectDelay: 5 * time.Second,
		maxReconnects:  3,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// AddRelayServer adds a relay server to the list of available servers.
//
//export ToxAddRelayServer
func (rc *RelayClient) AddRelayServer(server RelayServerInfo) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "AddRelayServer",
		"address":  server.Address,
		"port":     server.Port,
		"priority": server.Priority,
	}).Info("Adding relay server")

	rc.servers = append(rc.servers, server)
}

// RemoveRelayServer removes a relay server from the available servers.
//
//export ToxRemoveRelayServer
func (rc *RelayClient) RemoveRelayServer(address string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	for i, server := range rc.servers {
		if server.Address == address {
			rc.servers = append(rc.servers[:i], rc.servers[i+1:]...)
			logrus.WithFields(logrus.Fields{
				"function": "RemoveRelayServer",
				"address":  address,
			}).Info("Removed relay server")
			return
		}
	}
}

// Connect establishes a connection to an available relay server.
//
//export ToxRelayConnect
func (rc *RelayClient) Connect(ctx context.Context) error {
	rc.mu.Lock()
	if rc.state == RelayStateConnecting || rc.state == RelayStateConnected {
		rc.mu.Unlock()
		return nil
	}
	rc.state = RelayStateConnecting
	rc.mu.Unlock()

	logrus.WithField("function", "Connect").Info("Attempting relay connection")

	servers := rc.getServersByPriority()
	if len(servers) == 0 {
		rc.setState(RelayStateFailed)
		return errors.New("no relay servers available")
	}

	var lastErr error
	for _, server := range servers {
		if err := rc.connectToServer(ctx, server); err != nil {
			lastErr = err
			logrus.WithFields(logrus.Fields{
				"function": "Connect",
				"address":  server.Address,
				"error":    err.Error(),
			}).Warn("Failed to connect to relay server")
			continue
		}

		rc.mu.Lock()
		rc.activeServer = &server
		rc.state = RelayStateConnected
		rc.lastPong = time.Now()
		rc.mu.Unlock()

		rc.startKeepalive()
		go rc.readLoop()

		logrus.WithFields(logrus.Fields{
			"function": "Connect",
			"address":  server.Address,
		}).Info("Connected to relay server")

		return nil
	}

	rc.setState(RelayStateFailed)
	return fmt.Errorf("failed to connect to any relay server: %w", lastErr)
}

// connectToServer attempts TCP connection to a specific relay server.
func (rc *RelayClient) connectToServer(ctx context.Context, server RelayServerInfo) error {
	dialer := &net.Dialer{Timeout: rc.timeout}
	address := fmt.Sprintf("%s:%d", server.Address, server.Port)

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	rc.mu.Lock()
	rc.activeConn = conn
	rc.mu.Unlock()

	if err := rc.performHandshake(conn, server); err != nil {
		conn.Close()
		return fmt.Errorf("handshake failed: %w", err)
	}

	return nil
}

// performHandshake performs the relay protocol handshake.
func (rc *RelayClient) performHandshake(conn net.Conn, server RelayServerInfo) error {
	// Send registration packet with our public key
	regPacket := make([]byte, 33)
	regPacket[0] = byte(RelayPacketRouting)
	copy(regPacket[1:], rc.localPublicKey[:])

	if err := conn.SetWriteDeadline(time.Now().Add(rc.timeout)); err != nil {
		return err
	}

	if _, err := conn.Write(regPacket); err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}

	// Wait for acknowledgment
	if err := conn.SetReadDeadline(time.Now().Add(rc.timeout)); err != nil {
		return err
	}

	ackBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, ackBuf); err != nil {
		return fmt.Errorf("failed to read acknowledgment: %w", err)
	}

	if ackBuf[0] != byte(RelayPacketRouting) || ackBuf[1] != 0x01 {
		return errors.New("invalid handshake response")
	}

	// Reset deadlines
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	return nil
}

// RelayTo sends a packet through the relay to a target peer.
//
//export ToxRelayTo
func (rc *RelayClient) RelayTo(packet *Packet, targetPublicKey [32]byte) error {
	rc.mu.RLock()
	conn := rc.activeConn
	state := rc.state
	rc.mu.RUnlock()

	if state != RelayStateConnected || conn == nil {
		return errors.New("not connected to relay server")
	}

	packetData, err := packet.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize packet: %w", err)
	}

	// Build relay data packet: [type][target_key][length][data]
	relayPacket := make([]byte, 1+32+4+len(packetData))
	relayPacket[0] = byte(RelayPacketData)
	copy(relayPacket[1:33], targetPublicKey[:])
	relayPacket[33] = byte(len(packetData) >> 24)
	relayPacket[34] = byte(len(packetData) >> 16)
	relayPacket[35] = byte(len(packetData) >> 8)
	relayPacket[36] = byte(len(packetData))
	copy(relayPacket[37:], packetData)

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	defer conn.SetWriteDeadline(time.Time{})

	if _, err := conn.Write(relayPacket); err != nil {
		return fmt.Errorf("failed to send relayed packet: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "RelayTo",
		"packet_type": packet.PacketType,
		"data_size":   len(packetData),
	}).Debug("Packet relayed successfully")

	return nil
}

// SetDataHandler sets the handler for incoming relayed data packets.
//
//export ToxSetRelayDataHandler
func (rc *RelayClient) SetDataHandler(handler func(*Packet, net.Addr) error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.dataHandler = handler
}

// readLoop continuously reads from the relay connection.
func (rc *RelayClient) readLoop() {
	for {
		if rc.shouldStopReading() {
			return
		}

		conn := rc.getActiveConnection()
		if conn == nil {
			return
		}

		if err := rc.readAndProcessPacket(conn); err != nil {
			rc.handleReadError(err)
			return
		}
	}
}

// shouldStopReading checks if the read loop should terminate.
func (rc *RelayClient) shouldStopReading() bool {
	select {
	case <-rc.ctx.Done():
		return true
	default:
		return false
	}
}

// getActiveConnection retrieves the current active connection.
func (rc *RelayClient) getActiveConnection() net.Conn {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.activeConn
}

// handleReadError processes errors from the read operation.
func (rc *RelayClient) handleReadError(err error) {
	if !errors.Is(err, io.EOF) {
		logrus.WithFields(logrus.Fields{
			"function": "readLoop",
			"error":    err.Error(),
		}).Warn("Relay read error")
	}
	rc.handleDisconnect()
}

// readAndProcessPacket reads and processes a single packet from the relay.
func (rc *RelayClient) readAndProcessPacket(conn net.Conn) error {
	header := make([]byte, 1)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}

	packetType := RelayPacketType(header[0])

	switch packetType {
	case RelayPacketData:
		return rc.handleDataPacket(conn)
	case RelayPacketPong:
		rc.mu.Lock()
		rc.lastPong = time.Now()
		rc.mu.Unlock()
		return nil
	case RelayPacketDisconnect:
		return io.EOF
	default:
		return fmt.Errorf("unknown relay packet type: %d", packetType)
	}
}

// handleDataPacket processes an incoming relayed data packet.
func (rc *RelayClient) handleDataPacket(conn net.Conn) error {
	// Read source public key
	sourceKey := make([]byte, 32)
	if _, err := io.ReadFull(conn, sourceKey); err != nil {
		return err
	}

	// Read length
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lengthBuf); err != nil {
		return err
	}
	length := (uint32(lengthBuf[0]) << 24) |
		(uint32(lengthBuf[1]) << 16) |
		(uint32(lengthBuf[2]) << 8) |
		uint32(lengthBuf[3])

	// Read data
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return err
	}

	// Parse and dispatch packet
	packet, err := ParsePacket(data)
	if err != nil {
		return fmt.Errorf("failed to parse relayed packet: %w", err)
	}

	rc.mu.RLock()
	handler := rc.dataHandler
	server := rc.activeServer
	rc.mu.RUnlock()

	if handler != nil && server != nil {
		// Create a virtual address representing the source peer through relay
		relayedAddr := &RelayedAddress{
			RelayServer: server.Address,
			SourceKey:   sourceKey,
		}
		go func(ctx context.Context) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := handler(packet, relayedAddr); err != nil {
				logrus.WithFields(logrus.Fields{
					"function": "handleDataPacket",
					"error":    err.Error(),
				}).Warn("Data handler error")
			}
		}(rc.ctx)
	}

	return nil
}

// handleDisconnect handles relay disconnection and attempts reconnection.
func (rc *RelayClient) handleDisconnect() {
	rc.mu.Lock()
	rc.state = RelayStateDisconnected
	if rc.activeConn != nil {
		rc.activeConn.Close()
		rc.activeConn = nil
	}
	rc.activeServer = nil
	if rc.keepaliveTicker != nil {
		rc.keepaliveTicker.Stop()
		rc.keepaliveTicker = nil
	}
	rc.mu.Unlock()

	logrus.WithField("function", "handleDisconnect").Info("Relay disconnected")
}

// startKeepalive starts the keepalive ping/pong mechanism.
func (rc *RelayClient) startKeepalive() {
	rc.mu.Lock()
	if rc.keepaliveTicker != nil {
		rc.keepaliveTicker.Stop()
	}
	rc.keepaliveTicker = time.NewTicker(30 * time.Second)
	rc.mu.Unlock()

	go func() {
		for {
			select {
			case <-rc.ctx.Done():
				return
			case <-rc.keepaliveTicker.C:
				if err := rc.sendPing(); err != nil {
					logrus.WithFields(logrus.Fields{
						"function": "startKeepalive",
						"error":    err.Error(),
					}).Warn("Failed to send keepalive ping")
				}
			}
		}
	}()
}

// sendPing sends a keepalive ping to the relay server.
func (rc *RelayClient) sendPing() error {
	rc.mu.RLock()
	conn := rc.activeConn
	rc.mu.RUnlock()

	if conn == nil {
		return errors.New("no active connection")
	}

	pingPacket := []byte{byte(RelayPacketPing)}
	if _, err := conn.Write(pingPacket); err != nil {
		return err
	}
	return nil
}

// getServersByPriority returns servers sorted by priority.
func (rc *RelayClient) getServersByPriority() []RelayServerInfo {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	servers := make([]RelayServerInfo, len(rc.servers))
	copy(servers, rc.servers)

	// Simple insertion sort by priority (lower is better)
	for i := 1; i < len(servers); i++ {
		key := servers[i]
		j := i - 1
		for j >= 0 && servers[j].Priority > key.Priority {
			servers[j+1] = servers[j]
			j--
		}
		servers[j+1] = key
	}

	return servers
}

// setState safely updates the relay state.
func (rc *RelayClient) setState(state RelayState) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.state = state
}

// GetState returns the current relay connection state.
//
//export ToxGetRelayState
func (rc *RelayClient) GetState() RelayState {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.state
}

// IsConnected returns true if connected to a relay server.
//
//export ToxIsRelayConnected
func (rc *RelayClient) IsConnected() bool {
	return rc.GetState() == RelayStateConnected
}

// GetActiveServer returns information about the currently connected relay server.
//
//export ToxGetActiveRelayServer
func (rc *RelayClient) GetActiveServer() *RelayServerInfo {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.activeServer == nil {
		return nil
	}
	server := *rc.activeServer
	return &server
}

// Close closes the relay client and releases resources.
//
//export ToxCloseRelayClient
func (rc *RelayClient) Close() error {
	rc.cancel()

	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.keepaliveTicker != nil {
		rc.keepaliveTicker.Stop()
		rc.keepaliveTicker = nil
	}

	if rc.activeConn != nil {
		// Send disconnect packet
		disconnectPacket := []byte{byte(RelayPacketDisconnect)}
		rc.activeConn.SetWriteDeadline(time.Now().Add(time.Second))
		rc.activeConn.Write(disconnectPacket)
		rc.activeConn.Close()
		rc.activeConn = nil
	}

	rc.state = RelayStateDisconnected
	rc.activeServer = nil

	logrus.WithField("function", "Close").Info("Relay client closed")

	return nil
}

// SetTimeout sets the connection timeout for relay operations.
//
//export ToxSetRelayTimeout
func (rc *RelayClient) SetTimeout(timeout time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.timeout = timeout
}

// GetServerCount returns the number of configured relay servers.
//
//export ToxGetRelayServerCount
func (rc *RelayClient) GetServerCount() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.servers)
}

// RelayedAddress represents an address reached through a relay.
// It implements net.Addr for use in packet handling.
type RelayedAddress struct {
	RelayServer string
	SourceKey   []byte
}

// Network returns the network type for a relayed address.
func (ra *RelayedAddress) Network() string {
	return "relay"
}

// String returns a string representation of the relayed address.
func (ra *RelayedAddress) String() string {
	return fmt.Sprintf("relay://%s/%x", ra.RelayServer, ra.SourceKey[:8])
}
