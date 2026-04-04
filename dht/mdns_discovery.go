// Package dht implements the Distributed Hash Table for the Tox protocol.
package dht

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// mDNS multicast addresses
	mdnsIPv4Addr = "224.0.0.251:5353"
	mdnsIPv6Addr = "[ff02::fb]:5353"

	// Tox mDNS service name
	toxMDNSService = "_tox._udp.local."

	// mDNS constants
	mdnsQueryInterval   = 30 * time.Second
	mdnsResponseTimeout = 5 * time.Second
	mdnsMaxPacketSize   = 512
)

// MDNSDiscovery implements mDNS-based local peer discovery for Tox.
// It provides an alternative to UDP broadcast that works better in
// containerized environments like Docker and Kubernetes.
type MDNSDiscovery struct {
	enabled    bool
	publicKey  [32]byte
	port       uint16
	conn4      net.PacketConn
	conn6      net.PacketConn
	stopChan   chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
	onPeerFunc func(publicKey [32]byte, addr net.Addr)
	knownPeers map[string]time.Time // publicKey hex -> last seen
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewMDNSDiscovery creates a new mDNS discovery instance.
// publicKey is this node's Tox public key.
// port is the port this node listens on for Tox connections.
func NewMDNSDiscovery(publicKey [32]byte, port uint16) *MDNSDiscovery {
	ctx, cancel := context.WithCancel(context.Background())
	return &MDNSDiscovery{
		enabled:    false,
		publicKey:  publicKey,
		port:       port,
		stopChan:   make(chan struct{}),
		knownPeers: make(map[string]time.Time),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// joinMulticastGroups attempts to join both IPv4 and IPv6 multicast groups.
// Returns true if at least one group was successfully joined.
func (md *MDNSDiscovery) joinMulticastGroups() error {
	// Join IPv4 multicast group
	conn4, err := md.joinMulticastGroup("udp4", mdnsIPv4Addr)
	if err != nil {
		logrus.WithError(err).Warn("Failed to join IPv4 mDNS multicast group")
	} else {
		md.conn4 = conn4
	}

	// Join IPv6 multicast group
	conn6, err := md.joinMulticastGroup("udp6", mdnsIPv6Addr)
	if err != nil {
		logrus.WithError(err).Warn("Failed to join IPv6 mDNS multicast group")
	} else {
		md.conn6 = conn6
	}

	// Need at least one working connection
	if md.conn4 == nil && md.conn6 == nil {
		return fmt.Errorf("failed to join any mDNS multicast group")
	}
	return nil
}

// startReceiverGoroutines starts goroutines for receiving mDNS packets.
func (md *MDNSDiscovery) startReceiverGoroutines() {
	if md.conn4 != nil {
		md.wg.Add(1)
		go md.receiveLoop(md.conn4, "ipv4")
	}
	if md.conn6 != nil {
		md.wg.Add(1)
		go md.receiveLoop(md.conn6, "ipv6")
	}
}

// startBackgroundLoops starts the query and announce goroutines.
func (md *MDNSDiscovery) startBackgroundLoops() {
	md.wg.Add(1)
	go md.queryLoop()

	md.wg.Add(1)
	go md.announceLoop()
}

// Start begins mDNS discovery operations.
func (md *MDNSDiscovery) Start() error {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.enabled {
		return nil
	}

	if err := md.joinMulticastGroups(); err != nil {
		return err
	}

	md.enabled = true

	md.startReceiverGoroutines()
	md.startBackgroundLoops()

	logrus.WithFields(logrus.Fields{
		"port":    md.port,
		"hasIPv4": md.conn4 != nil,
		"hasIPv6": md.conn6 != nil,
	}).Info("mDNS discovery started")

	return nil
}

// joinMulticastGroup joins the specified multicast group.
func (md *MDNSDiscovery) joinMulticastGroup(network, addr string) (net.PacketConn, error) {
	multicastAddr, err := net.ResolveUDPAddr(network, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	// Listen on mDNS port
	conn, err := net.ListenPacket(network, fmt.Sprintf(":%d", multicastAddr.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on mDNS port: %w", err)
	}

	return conn, nil
}

// closeStopChannel safely closes the stop channel if not already closed.
func (md *MDNSDiscovery) closeStopChannel() {
	select {
	case <-md.stopChan:
		// Already closed
	default:
		close(md.stopChan)
	}
}

// closeConnections closes all multicast connections.
func (md *MDNSDiscovery) closeConnections() {
	if md.conn4 != nil {
		md.conn4.Close()
		md.conn4 = nil
	}
	if md.conn6 != nil {
		md.conn6.Close()
		md.conn6 = nil
	}
}

// Stop halts mDNS discovery operations.
func (md *MDNSDiscovery) Stop() {
	md.mu.Lock()

	if !md.enabled {
		md.mu.Unlock()
		return
	}

	md.enabled = false
	md.cancel()
	md.closeStopChannel()
	md.closeConnections()

	md.mu.Unlock()

	// Wait for goroutines to finish
	md.wg.Wait()

	logrus.Info("mDNS discovery stopped")
}

// OnPeer registers a callback for when a peer is discovered via mDNS.
func (md *MDNSDiscovery) OnPeer(callback func(publicKey [32]byte, addr net.Addr)) {
	md.mu.Lock()
	defer md.mu.Unlock()
	md.onPeerFunc = callback
}

// queryLoop periodically sends mDNS queries for Tox peers.
func (md *MDNSDiscovery) queryLoop() {
	defer md.wg.Done()

	ticker := time.NewTicker(mdnsQueryInterval)
	defer ticker.Stop()

	// Send initial query immediately
	md.sendQuery()

	for {
		select {
		case <-ticker.C:
			md.sendQuery()
		case <-md.stopChan:
			return
		}
	}
}

// announceLoop periodically announces this node via mDNS.
func (md *MDNSDiscovery) announceLoop() {
	defer md.wg.Done()

	ticker := time.NewTicker(mdnsQueryInterval)
	defer ticker.Stop()

	// Send initial announcement immediately
	md.sendAnnouncement()

	for {
		select {
		case <-ticker.C:
			md.sendAnnouncement()
		case <-md.stopChan:
			return
		}
	}
}

// sendQuery sends an mDNS query for Tox peers.
func (md *MDNSDiscovery) sendQuery() {
	md.mu.RLock()
	conn4 := md.conn4
	conn6 := md.conn6
	md.mu.RUnlock()

	// Build simplified mDNS query packet for Tox service
	query := md.buildMDNSQuery()

	// Send via IPv4
	if conn4 != nil {
		mdnsAddr4, _ := net.ResolveUDPAddr("udp4", mdnsIPv4Addr)
		if _, err := conn4.WriteTo(query, mdnsAddr4); err != nil {
			logrus.WithError(err).Debug("Failed to send mDNS IPv4 query")
		}
	}

	// Send via IPv6
	if conn6 != nil {
		mdnsAddr6, _ := net.ResolveUDPAddr("udp6", mdnsIPv6Addr)
		if _, err := conn6.WriteTo(query, mdnsAddr6); err != nil {
			logrus.WithError(err).Debug("Failed to send mDNS IPv6 query")
		}
	}
}

// sendAnnouncement sends an mDNS announcement for this node.
func (md *MDNSDiscovery) sendAnnouncement() {
	md.mu.RLock()
	conn4 := md.conn4
	conn6 := md.conn6
	publicKey := md.publicKey
	port := md.port
	md.mu.RUnlock()

	// Build mDNS response/announcement packet
	response := md.buildMDNSResponse(publicKey, port)

	// Send via IPv4
	if conn4 != nil {
		mdnsAddr4, _ := net.ResolveUDPAddr("udp4", mdnsIPv4Addr)
		if _, err := conn4.WriteTo(response, mdnsAddr4); err != nil {
			logrus.WithError(err).Debug("Failed to send mDNS IPv4 announcement")
		}
	}

	// Send via IPv6
	if conn6 != nil {
		mdnsAddr6, _ := net.ResolveUDPAddr("udp6", mdnsIPv6Addr)
		if _, err := conn6.WriteTo(response, mdnsAddr6); err != nil {
			logrus.WithError(err).Debug("Failed to send mDNS IPv6 announcement")
		}
	}
}

// buildMDNSQuery builds a simplified mDNS query packet for Tox service.
// Uses a custom Tox-specific format embedded in mDNS structure.
func (md *MDNSDiscovery) buildMDNSQuery() []byte {
	// Simplified mDNS-like packet structure:
	// - 2 bytes: magic number (0xF0F0 for Tox mDNS)
	// - 1 byte: type (0x01 = query, 0x02 = response)
	// - 1 byte: reserved
	// - 32 bytes: our public key (for identification)
	// - 2 bytes: our port
	packet := make([]byte, 38)
	binary.BigEndian.PutUint16(packet[0:2], 0xF0F0) // Magic
	packet[2] = 0x01                                // Query type
	packet[3] = 0x00                                // Reserved

	md.mu.RLock()
	copy(packet[4:36], md.publicKey[:])
	binary.BigEndian.PutUint16(packet[36:38], md.port)
	md.mu.RUnlock()

	return packet
}

// buildMDNSResponse builds an mDNS response/announcement packet.
func (md *MDNSDiscovery) buildMDNSResponse(publicKey [32]byte, port uint16) []byte {
	// Same format as query but with response type
	packet := make([]byte, 38)
	binary.BigEndian.PutUint16(packet[0:2], 0xF0F0) // Magic
	packet[2] = 0x02                                // Response type
	packet[3] = 0x00                                // Reserved
	copy(packet[4:36], publicKey[:])
	binary.BigEndian.PutUint16(packet[36:38], port)

	return packet
}

// shouldStopReceiveLoop checks if the receive loop should terminate.
func (md *MDNSDiscovery) shouldStopReceiveLoop() bool {
	select {
	case <-md.stopChan:
		return true
	default:
	}

	md.mu.RLock()
	enabled := md.enabled
	md.mu.RUnlock()
	return !enabled
}

// readPacketWithTimeout reads a packet with a 1-second timeout.
// Returns the number of bytes read, the sender address, and any error.
// Returns (0, nil, nil) on timeout to signal the caller should continue.
func (md *MDNSDiscovery) readPacketWithTimeout(conn net.PacketConn, buffer []byte) (int, net.Addr, error) {
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, addr, err := conn.ReadFrom(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return 0, nil, nil // Timeout is not an error, just continue
		}
		return 0, nil, err
	}
	return n, addr, nil
}

// receiveLoop listens for incoming mDNS packets.
func (md *MDNSDiscovery) receiveLoop(conn net.PacketConn, label string) {
	defer md.wg.Done()

	buffer := make([]byte, mdnsMaxPacketSize)

	for {
		if md.shouldStopReceiveLoop() {
			return
		}

		n, addr, err := md.readPacketWithTimeout(conn, buffer)
		if err != nil {
			if md.shouldStopReceiveLoop() {
				return
			}
			continue
		}
		if n == 0 {
			continue // Timeout, try again
		}

		md.handlePacket(buffer[:n], addr, label)
	}
}

// handlePacket processes an incoming mDNS packet.
func (md *MDNSDiscovery) handlePacket(data []byte, addr net.Addr, label string) {
	// Check minimum packet size
	if len(data) < 38 {
		return
	}

	// Verify magic number
	magic := binary.BigEndian.Uint16(data[0:2])
	if magic != 0xF0F0 {
		return // Not a Tox mDNS packet
	}

	packetType := data[2]

	// Extract public key and port
	var publicKey [32]byte
	copy(publicKey[:], data[4:36])
	port := binary.BigEndian.Uint16(data[36:38])

	// Don't process our own packets
	md.mu.RLock()
	selfKey := md.publicKey
	md.mu.RUnlock()
	if publicKey == selfKey {
		return
	}

	// If this is a query, respond with our info
	if packetType == 0x01 {
		md.sendAnnouncement()
		return
	}

	// For responses, notify about the discovered peer
	if packetType == 0x02 {
		md.notifyPeer(publicKey, port, addr, label)
	}
}

// notifyPeer notifies the callback about a discovered peer.
func (md *MDNSDiscovery) notifyPeer(publicKey [32]byte, port uint16, addr net.Addr, label string) {
	// Extract IP from address
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return
	}

	// Create peer address with the port from the packet
	peerAddr := &net.UDPAddr{
		IP:   ip,
		Port: int(port),
	}

	// Update known peers
	keyHex := fmt.Sprintf("%x", publicKey)
	md.mu.Lock()
	lastSeen, exists := md.knownPeers[keyHex]
	now := time.Now()
	md.knownPeers[keyHex] = now
	md.mu.Unlock()

	// Only log if this is a new peer or hasn't been seen recently
	if !exists || now.Sub(lastSeen) > time.Minute {
		logrus.WithFields(logrus.Fields{
			"peer_addr":  peerAddr.String(),
			"public_key": keyHex[:16],
			"via":        label,
		}).Info("Discovered peer via mDNS")
	}

	// Notify callback
	md.mu.RLock()
	callback := md.onPeerFunc
	md.mu.RUnlock()

	if callback != nil {
		callback(publicKey, peerAddr)
	}
}

// IsEnabled returns whether mDNS discovery is currently enabled.
func (md *MDNSDiscovery) IsEnabled() bool {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.enabled
}

// KnownPeerCount returns the number of peers discovered via mDNS.
func (md *MDNSDiscovery) KnownPeerCount() int {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return len(md.knownPeers)
}

// CleanupStale removes peers that haven't been seen recently.
func (md *MDNSDiscovery) CleanupStale(maxAge time.Duration) int {
	md.mu.Lock()
	defer md.mu.Unlock()

	now := time.Now()
	removed := 0
	for key, lastSeen := range md.knownPeers {
		if now.Sub(lastSeen) > maxAge {
			delete(md.knownPeers, key)
			removed++
		}
	}
	return removed
}
