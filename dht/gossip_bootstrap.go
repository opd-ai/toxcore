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

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// GossipConfig holds configuration for gossip-based peer discovery.
type GossipConfig struct {
	// MaxPeersPerExchange limits nodes exchanged per gossip message
	MaxPeersPerExchange int
	// ExchangeInterval controls how often gossip exchanges occur
	ExchangeInterval time.Duration
	// MaxCachedPeers limits the gossip peer cache size
	MaxCachedPeers int
	// PeerTTL defines how long a cached peer remains valid
	PeerTTL time.Duration
}

// DefaultGossipConfig returns sensible defaults for gossip discovery.
func DefaultGossipConfig() *GossipConfig {
	return &GossipConfig{
		MaxPeersPerExchange: 8,
		ExchangeInterval:    30 * time.Second,
		MaxCachedPeers:      64,
		PeerTTL:             10 * time.Minute,
	}
}

// GossipPeer represents a peer discovered via gossip protocol.
type GossipPeer struct {
	PublicKey [32]byte
	Address   net.Addr
	FirstSeen time.Time
	LastSeen  time.Time
	Source    net.Addr // Which peer told us about this node
}

// GossipBootstrap implements peer-exchange-based bootstrap as a fallback
// when hardcoded bootstrap nodes are unreachable.
//
//export ToxDHTGossipBootstrap
type GossipBootstrap struct {
	config       *GossipConfig
	selfID       crypto.ToxID
	transport    transport.Transport
	routingTable *RoutingTable
	peers        map[[32]byte]*GossipPeer
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	running      bool
	timeProvider TimeProvider
}

// NewGossipBootstrap creates a new gossip-based bootstrap manager.
//
//export ToxDHTGossipBootstrapNew
func NewGossipBootstrap(
	selfID crypto.ToxID,
	transportArg transport.Transport,
	routingTable *RoutingTable,
	config *GossipConfig,
) *GossipBootstrap {
	if config == nil {
		config = DefaultGossipConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	gb := &GossipBootstrap{
		config:       config,
		selfID:       selfID,
		transport:    transportArg,
		routingTable: routingTable,
		peers:        make(map[[32]byte]*GossipPeer),
		ctx:          ctx,
		cancel:       cancel,
		timeProvider: nil,
	}

	gb.registerGossipHandlers()
	return gb
}

// SetTimeProvider sets the time provider for deterministic testing.
func (gb *GossipBootstrap) SetTimeProvider(tp TimeProvider) {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	gb.timeProvider = tp
}

// getTimeProvider returns the time provider, using default if nil.
func (gb *GossipBootstrap) getTimeProvider() TimeProvider {
	if gb.timeProvider != nil {
		return gb.timeProvider
	}
	return getDefaultTimeProvider()
}

// Start begins the gossip exchange routine.
func (gb *GossipBootstrap) Start() error {
	gb.mu.Lock()
	if gb.running {
		gb.mu.Unlock()
		return nil
	}
	gb.running = true
	gb.mu.Unlock()

	go gb.exchangeRoutine()
	return nil
}

// Stop halts the gossip exchange routine.
func (gb *GossipBootstrap) Stop() {
	gb.mu.Lock()
	if !gb.running {
		gb.mu.Unlock()
		return
	}
	gb.running = false
	gb.cancel()
	gb.mu.Unlock()
}

// IsRunning returns whether gossip exchange is active.
func (gb *GossipBootstrap) IsRunning() bool {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return gb.running
}

// BootstrapFromGossip attempts to bootstrap using cached gossip peers.
// This is the fallback entry point when primary bootstrap nodes fail.
func (gb *GossipBootstrap) BootstrapFromGossip(ctx context.Context) error {
	peers := gb.getActivePeers()
	if len(peers) == 0 {
		return errors.New("no gossip peers available")
	}

	logrus.WithFields(logrus.Fields{
		"function":   "BootstrapFromGossip",
		"peer_count": len(peers),
	}).Info("Attempting gossip-based bootstrap")

	successCount := gb.sendGetNodesRequests(ctx, peers)

	if successCount == 0 {
		return errors.New("all gossip peers unreachable")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "BootstrapFromGossip",
		"success_count": successCount,
	}).Info("Gossip bootstrap requests sent")

	return nil
}

// sendGetNodesRequests sends get-nodes requests to peers, respecting context cancellation.
func (gb *GossipBootstrap) sendGetNodesRequests(ctx context.Context, peers []*GossipPeer) int {
	successCount := 0
	for _, peer := range peers {
		select {
		case <-ctx.Done():
			return successCount
		default:
		}

		if err := gb.sendGetNodesRequest(peer); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "BootstrapFromGossip",
				"peer":     peer.Address.String(),
				"error":    err.Error(),
			}).Debug("Gossip peer request failed")
			continue
		}
		successCount++
	}
	return successCount
}

// RequestPeerExchange sends a peer exchange request to the given node.
func (gb *GossipBootstrap) RequestPeerExchange(addr net.Addr) error {
	packet := gb.buildPeerExchangeRequest()
	return gb.transport.Send(packet, addr)
}

// AddPeer adds a peer discovered from another source.
func (gb *GossipBootstrap) AddPeer(publicKey [32]byte, addr, source net.Addr) {
	gb.mu.Lock()
	defer gb.mu.Unlock()

	now := gb.getTimeProvider().Now()

	if existing, ok := gb.peers[publicKey]; ok {
		existing.LastSeen = now
		return
	}

	if len(gb.peers) >= gb.config.MaxCachedPeers {
		gb.evictOldestPeer()
	}

	gb.peers[publicKey] = &GossipPeer{
		PublicKey: publicKey,
		Address:   addr,
		FirstSeen: now,
		LastSeen:  now,
		Source:    source,
	}
}

// GetPeerCount returns the number of cached gossip peers.
func (gb *GossipBootstrap) GetPeerCount() int {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return len(gb.peers)
}

// registerGossipHandlers registers packet handlers for gossip protocol.
func (gb *GossipBootstrap) registerGossipHandlers() {
	if gb.transport == nil {
		return
	}

	// Use SendNodes packet type for peer exchange (compatible with Tox protocol)
	gb.transport.RegisterHandler(transport.PacketSendNodes, func(packet *transport.Packet, senderAddr net.Addr) error {
		return gb.handleSendNodes(packet, senderAddr)
	})

	logrus.WithFields(logrus.Fields{
		"function": "registerGossipHandlers",
	}).Debug("Registered gossip packet handlers")
}

// handleSendNodes processes incoming SendNodes packets and extracts peers.
func (gb *GossipBootstrap) handleSendNodes(packet *transport.Packet, senderAddr net.Addr) error {
	if len(packet.Data) < 1 {
		return errors.New("empty SendNodes packet")
	}

	nodeCount := int(packet.Data[0])
	if nodeCount == 0 || nodeCount > gb.config.MaxPeersPerExchange {
		return nil // Ignore empty or oversized responses
	}

	offset := 1
	for i := 0; i < nodeCount && offset+39 <= len(packet.Data); i++ {
		peer, newOffset, err := gb.parseNodeEntry(packet.Data, offset, senderAddr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "handleSendNodes",
				"error":    err.Error(),
			}).Debug("Failed to parse node entry")
			break
		}
		offset = newOffset

		gb.AddPeer(peer.PublicKey, peer.Address, senderAddr)
	}

	return nil
}

// parseNodeEntry extracts a single node from SendNodes packet data.
func (gb *GossipBootstrap) parseNodeEntry(data []byte, offset int, source net.Addr) (*GossipPeer, int, error) {
	// Node format: 1 byte IP type + 4/16 bytes IP + 2 bytes port + 32 bytes public key
	if offset >= len(data) {
		return nil, offset, errors.New("offset out of bounds")
	}

	ipType := data[offset]
	offset++

	ip, ipLen, err := parseIPFromType(data, offset, ipType)
	if err != nil {
		return nil, offset, err
	}
	offset += ipLen

	port, offset, err := parsePort(data, offset)
	if err != nil {
		return nil, offset, err
	}

	publicKey, offset, err := parsePublicKey(data, offset)
	if err != nil {
		return nil, offset, err
	}

	addr := &net.UDPAddr{IP: ip, Port: int(port)}
	now := gb.getTimeProvider().Now()

	return &GossipPeer{
		PublicKey: publicKey,
		Address:   addr,
		FirstSeen: now,
		LastSeen:  now,
		Source:    source,
	}, offset, nil
}

// parseIPFromType extracts IP address based on type byte.
func parseIPFromType(data []byte, offset int, ipType byte) (net.IP, int, error) {
	switch ipType {
	case 2: // IPv4 UDP
		if offset+4 > len(data) {
			return nil, 0, errors.New("insufficient data for IPv4")
		}
		return net.IP(data[offset : offset+4]), 4, nil
	case 10: // IPv6 UDP
		if offset+16 > len(data) {
			return nil, 0, errors.New("insufficient data for IPv6")
		}
		return net.IP(data[offset : offset+16]), 16, nil
	default:
		return nil, 0, fmt.Errorf("unsupported IP type: %d", ipType)
	}
}

// parsePort extracts port from data buffer.
func parsePort(data []byte, offset int) (uint16, int, error) {
	if offset+2 > len(data) {
		return 0, offset, errors.New("insufficient data for port")
	}
	port := binary.BigEndian.Uint16(data[offset : offset+2])
	return port, offset + 2, nil
}

// parsePublicKey extracts 32-byte public key from data buffer.
func parsePublicKey(data []byte, offset int) ([32]byte, int, error) {
	var publicKey [32]byte
	if offset+32 > len(data) {
		return publicKey, offset, errors.New("insufficient data for public key")
	}
	copy(publicKey[:], data[offset:offset+32])
	return publicKey, offset + 32, nil
}

// exchangeRoutine periodically exchanges peers with known nodes.
func (gb *GossipBootstrap) exchangeRoutine() {
	ticker := time.NewTicker(gb.config.ExchangeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-gb.ctx.Done():
			return
		case <-ticker.C:
			gb.performExchange()
		}
	}
}

// performExchange sends peer exchange requests to routing table nodes.
func (gb *GossipBootstrap) performExchange() {
	gb.pruneExpiredPeers()

	if gb.routingTable == nil {
		return
	}

	// Get a few random nodes from routing table
	closestNodes := gb.routingTable.FindClosestNodes(gb.selfID, 4)
	for _, node := range closestNodes {
		if err := gb.RequestPeerExchange(node.Address); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "performExchange",
				"node":     node.Address.String(),
				"error":    err.Error(),
			}).Debug("Peer exchange request failed")
		}
	}
}

// buildPeerExchangeRequest creates a GetNodes packet requesting peers.
func (gb *GossipBootstrap) buildPeerExchangeRequest() *transport.Packet {
	// Request nodes close to our own ID (standard Kademlia behavior)
	data := make([]byte, 64)
	copy(data[:32], gb.selfID.PublicKey[:])
	copy(data[32:], gb.selfID.PublicKey[:]) // Target = self for self-discovery

	return &transport.Packet{
		PacketType: transport.PacketGetNodes,
		Data:       data,
	}
}

// sendGetNodesRequest sends a GetNodes request to a gossip peer.
func (gb *GossipBootstrap) sendGetNodesRequest(peer *GossipPeer) error {
	packet := gb.buildPeerExchangeRequest()
	return gb.transport.Send(packet, peer.Address)
}

// getActivePeers returns all non-expired cached peers.
func (gb *GossipBootstrap) getActivePeers() []*GossipPeer {
	gb.mu.RLock()
	defer gb.mu.RUnlock()

	now := gb.getTimeProvider().Now()
	active := make([]*GossipPeer, 0, len(gb.peers))

	for _, peer := range gb.peers {
		if now.Sub(peer.LastSeen) < gb.config.PeerTTL {
			active = append(active, peer)
		}
	}

	return active
}

// pruneExpiredPeers removes peers that have exceeded their TTL.
func (gb *GossipBootstrap) pruneExpiredPeers() {
	gb.mu.Lock()
	defer gb.mu.Unlock()

	now := gb.getTimeProvider().Now()
	for key, peer := range gb.peers {
		if now.Sub(peer.LastSeen) >= gb.config.PeerTTL {
			delete(gb.peers, key)
		}
	}
}

// evictOldestPeer removes the oldest peer from the cache.
func (gb *GossipBootstrap) evictOldestPeer() {
	var oldestKey [32]byte
	var oldestTime time.Time
	first := true

	for key, peer := range gb.peers {
		if first || peer.FirstSeen.Before(oldestTime) {
			oldestKey = key
			oldestTime = peer.FirstSeen
			first = false
		}
	}

	if !first {
		delete(gb.peers, oldestKey)
	}
}

// SeedFromRoutingTable populates gossip cache from the routing table.
// This is useful when restarting to preserve peer knowledge.
func (gb *GossipBootstrap) SeedFromRoutingTable() int {
	if gb.routingTable == nil {
		return 0
	}

	count := 0
	for i := 0; i < 256 && count < gb.config.MaxCachedPeers; i++ {
		bucket := gb.routingTable.kBuckets[i]
		nodes := bucket.GetNodes()

		for _, node := range nodes {
			if node.Status != StatusGood {
				continue
			}
			gb.AddPeer(node.PublicKey, node.Address, nil)
			count++
			if count >= gb.config.MaxCachedPeers {
				break
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"function":   "SeedFromRoutingTable",
		"peer_count": count,
	}).Debug("Seeded gossip cache from routing table")

	return count
}
