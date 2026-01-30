// Package dht implements the Distributed Hash Table for the Tox protocol.
package dht

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// LAN discovery packet constants
	lanDiscoveryInterval = 10 * time.Second
	lanDiscoveryTimeout  = 60 * time.Second
)

// LANDiscovery handles local area network peer discovery via UDP broadcast.
type LANDiscovery struct {
	enabled       bool
	publicKey     [32]byte
	port          uint16
	discoveryPort uint16
	conn          net.PacketConn
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	onPeerFunc    func(publicKey [32]byte, addr net.Addr)
}

// NewLANDiscovery creates a new LAN discovery instance.
// port is the port this node listens on for Tox connections.
// The discovery port is automatically set to port+1 to avoid conflicts with the main UDP transport.
func NewLANDiscovery(publicKey [32]byte, port uint16) *LANDiscovery {
	discoveryPort := port + 1
	if discoveryPort == 0 {
		discoveryPort = 1
	}

	return &LANDiscovery{
		enabled:       false,
		publicKey:     publicKey,
		port:          port,
		discoveryPort: discoveryPort,
		stopChan:      make(chan struct{}),
	}
}

// Start begins LAN discovery operations.
func (ld *LANDiscovery) Start() error {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	if ld.enabled {
		return nil
	}

	// Create UDP connection for broadcasting
	conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", ld.discoveryPort))
	if err != nil {
		logrus.WithError(err).Error("Failed to create LAN discovery socket")
		return fmt.Errorf("failed to create LAN discovery socket: %w", err)
	}

	ld.conn = conn
	ld.enabled = true

	// Start broadcast goroutine
	ld.wg.Add(1)
	go ld.broadcastLoop()

	// Start receiver goroutine
	ld.wg.Add(1)
	go ld.receiveLoop()

	logrus.WithFields(logrus.Fields{
		"port": ld.discoveryPort,
	}).Info("LAN discovery started")

	return nil
}

// Stop halts LAN discovery operations.
func (ld *LANDiscovery) Stop() {
	ld.mu.Lock()

	if !ld.enabled {
		ld.mu.Unlock()
		return
	}

	ld.enabled = false

	// Close the stopChan to signal goroutines
	select {
	case <-ld.stopChan:
		// Already closed
	default:
		close(ld.stopChan)
	}

	// Close the connection to unblock any ReadFrom calls
	if ld.conn != nil {
		ld.conn.Close()
		ld.conn = nil
	}

	ld.mu.Unlock()

	// Wait for goroutines to finish
	ld.wg.Wait()

	logrus.Info("LAN discovery stopped")
}

// OnPeer registers a callback for when a peer is discovered on the LAN.
func (ld *LANDiscovery) OnPeer(callback func(publicKey [32]byte, addr net.Addr)) {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	ld.onPeerFunc = callback
}

// broadcastLoop periodically broadcasts LAN discovery packets.
func (ld *LANDiscovery) broadcastLoop() {
	defer ld.wg.Done()

	ticker := time.NewTicker(lanDiscoveryInterval)
	defer ticker.Stop()

	// Send initial broadcast immediately
	ld.broadcast()

	for {
		select {
		case <-ticker.C:
			ld.broadcast()
		case <-ld.stopChan:
			return
		}
	}
}

// broadcast sends a LAN discovery packet to the broadcast address.
func (ld *LANDiscovery) broadcast() {
	ld.mu.RLock()
	conn := ld.conn
	publicKey := ld.publicKey
	port := ld.port
	discoveryPort := ld.discoveryPort
	ld.mu.RUnlock()

	if conn == nil {
		return
	}

	// Create LAN discovery packet: [public key (32 bytes)][port (2 bytes)]
	packet := make([]byte, 34)
	copy(packet[0:32], publicKey[:])
	binary.BigEndian.PutUint16(packet[32:34], port)

	// Broadcast to IPv4
	broadcastAddr := &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: int(discoveryPort),
	}

	_, err := conn.WriteTo(packet, broadcastAddr)
	if err != nil {
		logrus.WithError(err).Debug("Failed to send IPv4 LAN discovery broadcast")
	} else {
		logrus.WithFields(logrus.Fields{
			"addr": broadcastAddr.String(),
			"port": port,
		}).Debug("Sent LAN discovery broadcast")
	}

	// Also try common private network broadcast addresses
	privateBroadcasts := []string{
		"192.168.255.255", // Common /16 network
		"10.255.255.255",  // Common /8 network
		"172.31.255.255",  // Common /16 network
	}

	for _, bcAddr := range privateBroadcasts {
		addr := &net.UDPAddr{
			IP:   net.ParseIP(bcAddr),
			Port: int(discoveryPort),
		}
		conn.WriteTo(packet, addr)
	}
}

// receiveLoop listens for incoming LAN discovery packets.
func (ld *LANDiscovery) receiveLoop() {
	defer ld.wg.Done()

	buffer := make([]byte, 1024)

	for {
		// Check if we should stop before attempting read
		select {
		case <-ld.stopChan:
			return
		default:
		}

		ld.mu.RLock()
		conn := ld.conn
		enabled := ld.enabled
		ld.mu.RUnlock()

		if conn == nil || !enabled {
			return
		}

		// Set read deadline to allow checking stopChan
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			// Check if we're stopping
			select {
			case <-ld.stopChan:
				return
			default:
				// Network errors or timeout - continue
				continue
			}
		}

		ld.handlePacket(buffer[:n], addr)
	}
}

// handlePacket processes an incoming LAN discovery packet.
func (ld *LANDiscovery) handlePacket(data []byte, addr net.Addr) {
	// LAN discovery packet format: [public key (32 bytes)][port (2 bytes)]
	if len(data) < 34 {
		logrus.Debug("Received invalid LAN discovery packet (too short)")
		return
	}

	var publicKey [32]byte
	copy(publicKey[:], data[0:32])

	// Don't process our own broadcasts
	ld.mu.RLock()
	selfKey := ld.publicKey
	ld.mu.RUnlock()

	if publicKey == selfKey {
		return
	}

	port := binary.BigEndian.Uint16(data[32:34])

	// Extract IP from the UDP address
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		logrus.Debug("Received LAN discovery from non-UDP address")
		return
	}

	// Create peer address with the port from the packet
	peerAddr := &net.UDPAddr{
		IP:   udpAddr.IP,
		Port: int(port),
	}

	logrus.WithFields(logrus.Fields{
		"peer_addr":  peerAddr.String(),
		"public_key": fmt.Sprintf("%x", publicKey[:8]),
	}).Info("Discovered LAN peer")

	// Notify callback
	ld.mu.RLock()
	callback := ld.onPeerFunc
	ld.mu.RUnlock()

	if callback != nil {
		callback(publicKey, peerAddr)
	}
}

// IsEnabled returns whether LAN discovery is currently enabled.
func (ld *LANDiscovery) IsEnabled() bool {
	ld.mu.RLock()
	defer ld.mu.RUnlock()
	return ld.enabled
}

// lanDiscoveryPacketData creates a LAN discovery packet payload.
// This is used by the main Tox instance when receiving PacketLANDiscovery.
func LANDiscoveryPacketData(publicKey [32]byte, port uint16) []byte {
	packet := make([]byte, 34)
	copy(packet[0:32], publicKey[:])
	binary.BigEndian.PutUint16(packet[32:34], port)
	return packet
}

// ParseLANDiscoveryPacket extracts public key and port from a LAN discovery packet.
func ParseLANDiscoveryPacket(data []byte) (publicKey [32]byte, port uint16, err error) {
	if len(data) < 34 {
		return publicKey, 0, fmt.Errorf("invalid LAN discovery packet: too short")
	}

	copy(publicKey[:], data[0:32])
	port = binary.BigEndian.Uint16(data[32:34])

	return publicKey, port, nil
}
