// Package dht implements the Distributed Hash Table for the Tox protocol.
package dht

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
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

// Package-level pre-resolved multicast addresses. These are derived from compile-time
// constants (mdnsIPv4Addr / mdnsIPv6Addr), so resolution cannot fail; the init()
// function panics to make that invariant explicit rather than silently discarding an error.
var (
	mdnsUDPv4Addr net.Addr
	mdnsUDPv6Addr net.Addr
)

func init() {
	var err error
	if mdnsUDPv4Addr, err = net.ResolveUDPAddr("udp4", mdnsIPv4Addr); err != nil {
		panic("dht: failed to resolve mDNS IPv4 address: " + err.Error())
	}
	if mdnsUDPv6Addr, err = net.ResolveUDPAddr("udp6", mdnsIPv6Addr); err != nil {
		panic("dht: failed to resolve mDNS IPv6 address: " + err.Error())
	}
}

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

// startBackgroundLoops starts the query, announce, and cleanup goroutines.
func (md *MDNSDiscovery) startBackgroundLoops() {
	md.wg.Add(1)
	go md.queryLoop()

	md.wg.Add(1)
	go md.announceLoop()

	md.wg.Add(1)
	go md.cleanupLoop()
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

	// Recreate context and stopChan for this start session
	md.ctx, md.cancel = context.WithCancel(context.Background())
	md.stopChan = make(chan struct{})

	md.startReceiverGoroutines()
	md.startBackgroundLoops()

	logrus.WithFields(logrus.Fields{
		"port":    md.port,
		"hasIPv4": md.conn4 != nil,
		"hasIPv6": md.conn6 != nil,
	}).Info("mDNS discovery started")

	return nil
}

// joinMulticastGroup binds to the mDNS port and joins the multicast group so
// that incoming multicast packets are delivered to the socket.
func (md *MDNSDiscovery) joinMulticastGroup(network, addr string) (net.PacketConn, error) {
	multicastAddr, err := net.ResolveUDPAddr(network, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	// Listen on mDNS port (bind to the multicast port on INADDR_ANY)
	conn, err := net.ListenPacket(network, fmt.Sprintf(":%d", multicastAddr.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on mDNS port: %w", err)
	}

	// Join the multicast group so the OS delivers multicast packets to us.
	ifaces, err := net.Interfaces()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enumerate interfaces: %w", err)
	}

	switch network {
	case "udp4":
		p := ipv4.NewPacketConn(conn)
		md.joinInterfaces(ifaces, multicastAddr.IP.String(), func(iface *net.Interface) error {
			return p.JoinGroup(iface, transport.NewUDPAddr(multicastAddr.IP, 0))
		}, "ipv4")
	case "udp6":
		p := ipv6.NewPacketConn(conn)
		md.joinInterfaces(ifaces, multicastAddr.IP.String(), func(iface *net.Interface) error {
			return p.JoinGroup(iface, transport.NewUDPAddr(multicastAddr.IP, 0))
		}, "ipv6")
	}

	return conn, nil
}

// joinInterfaces attempts to join the multicast group on each interface.
func (md *MDNSDiscovery) joinInterfaces(ifaces []net.Interface, group string, join func(*net.Interface) error, network string) {
	for i := range ifaces {
		if err := join(&ifaces[i]); err != nil {
			md.logJoinGroupFailure(ifaces[i].Name, group, network, err)
		}
	}
}

// logJoinGroupFailure records an interface-specific multicast join failure.
func (md *MDNSDiscovery) logJoinGroupFailure(name, group, network string, err error) {
	logrus.WithFields(logrus.Fields{
		"interface": name,
		"group":     group,
		"error":     err.Error(),
	}).Debug(fmt.Sprintf("mDNS JoinGroup (%s) failed on interface", network))
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

// cleanupLoop periodically removes stale peer entries from knownPeers map.
// Peers are considered stale if not seen for 10 minutes (F-DHT-L2).
func (md *MDNSDiscovery) cleanupLoop() {
	defer md.wg.Done()

	// Run cleanup every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			md.cleanupStalePeers()
		case <-md.stopChan:
			return
		}
	}
}

// cleanupStalePeers removes stale peers and logs when entries were removed.
func (md *MDNSDiscovery) cleanupStalePeers() {
	removed := md.CleanupStale(10 * time.Minute)
	if removed == 0 {
		return
	}

	logrus.WithFields(logrus.Fields{
		"removed":   removed,
		"component": "MDNSDiscovery",
	}).Debug("Cleaned up stale mDNS peers")
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
		if _, err := conn4.WriteTo(query, mdnsUDPv4Addr); err != nil {
			logrus.WithError(err).Debug("Failed to send mDNS IPv4 query")
		}
	}

	// Send via IPv6
	if conn6 != nil {
		if _, err := conn6.WriteTo(query, mdnsUDPv6Addr); err != nil {
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
		if _, err := conn4.WriteTo(response, mdnsUDPv4Addr); err != nil {
			logrus.WithError(err).Debug("Failed to send mDNS IPv4 announcement")
		}
	}

	// Send via IPv6
	if conn6 != nil {
		if _, err := conn6.WriteTo(response, mdnsUDPv6Addr); err != nil {
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
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
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
		if md.shouldStopReceiveLoop() || !md.processReceiveIteration(conn, buffer, label) {
			return
		}
	}
}

// processReceiveIteration handles one packet read for receiveLoop.
func (md *MDNSDiscovery) processReceiveIteration(conn net.PacketConn, buffer []byte, label string) bool {
	n, addr, err := md.readPacketWithTimeout(conn, buffer)
	if err != nil {
		return !md.shouldStopReceiveLoop()
	}
	if n == 0 {
		return true
	}

	md.handlePacket(buffer[:n], addr, label)
	return true
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
	peerAddr := transport.NewUDPAddr(ip, int(port))

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
