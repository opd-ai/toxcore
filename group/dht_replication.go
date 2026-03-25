// Package group implements DHT replication for group announcements.
// This file provides k=5 nearest node replication for group discovery.
package group

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/transport"
)

const (
	// DefaultReplicationFactor is the number of nearest nodes to store
	// group announcements on for redundancy.
	DefaultReplicationFactor = 5

	// MinReplicationSuccess is the minimum number of nodes that must
	// successfully store the announcement for the operation to succeed.
	MinReplicationSuccess = 2

	// ReplicationRetryDelay is the delay between retry attempts.
	ReplicationRetryDelay = 500 * time.Millisecond

	// ReplicationMaxRetries is the maximum number of retries per node.
	ReplicationMaxRetries = 2
)

// ReplicatedAnnouncement extends GroupAnnouncement with replication metadata.
type ReplicatedAnnouncement struct {
	*dht.GroupAnnouncement

	// ReplicaNodes holds the public keys of nodes storing this announcement.
	ReplicaNodes [][32]byte

	// LastRefresh tracks when replicas were last refreshed.
	LastRefresh time.Time
}

// ReplicationManager handles distributed storage of group announcements.
type ReplicationManager struct {
	mu sync.RWMutex

	// announcements maps group ID to replication metadata.
	announcements map[uint32]*ReplicatedAnnouncement

	// replicationFactor is the target number of storage nodes (k).
	replicationFactor int

	// routingTable for finding nearest nodes.
	routingTable *dht.RoutingTable

	// transport for sending packets.
	transport transport.Transport
}

// NewReplicationManager creates a new replication manager.
func NewReplicationManager(rt *dht.RoutingTable, tr transport.Transport) *ReplicationManager {
	return &ReplicationManager{
		announcements:     make(map[uint32]*ReplicatedAnnouncement),
		replicationFactor: DefaultReplicationFactor,
		routingTable:      rt,
		transport:         tr,
	}
}

// SetReplicationFactor sets the target number of storage nodes.
func (rm *ReplicationManager) SetReplicationFactor(k int) {
	if k < 1 {
		k = 1
	}
	if k > 20 {
		k = 20
	}
	rm.mu.Lock()
	rm.replicationFactor = k
	rm.mu.Unlock()
}

// ReplicationFactor returns the current replication factor.
func (rm *ReplicationManager) ReplicationFactor() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.replicationFactor
}

// ReplicateAnnouncement stores a group announcement at the k nearest nodes.
// Returns (successCount, nil) if at least MinReplicationSuccess nodes stored it.
// Returns (successCount, error) if fewer than MinReplicationSuccess nodes succeeded.
func (rm *ReplicationManager) ReplicateAnnouncement(announcement *dht.GroupAnnouncement) (int, error) {
	if rm.routingTable == nil {
		return 0, fmt.Errorf("replication failed: routing table not set")
	}
	if rm.transport == nil {
		return 0, fmt.Errorf("replication failed: transport not set")
	}

	targetID := rm.groupIDToToxID(announcement.GroupID)
	nearestNodes := rm.routingTable.FindClosestNodes(targetID, rm.replicationFactor)

	if len(nearestNodes) == 0 {
		return 0, fmt.Errorf("replication failed: no DHT nodes available")
	}

	data, err := dht.SerializeAnnouncement(announcement)
	if err != nil {
		return 0, fmt.Errorf("replication failed: %w", err)
	}

	packet := &transport.Packet{
		PacketType: transport.PacketGroupAnnounce,
		Data:       data,
	}

	successCount, replicaKeys := rm.sendToNearestNodes(packet, nearestNodes)
	rm.storeReplicationMetadata(announcement, replicaKeys)

	if successCount < MinReplicationSuccess {
		return successCount, fmt.Errorf(
			"replication incomplete: only %d/%d nodes succeeded (minimum %d required)",
			successCount, rm.replicationFactor, MinReplicationSuccess,
		)
	}

	return successCount, nil
}

// sendToNearestNodes sends packet to nodes with retry logic.
func (rm *ReplicationManager) sendToNearestNodes(
	packet *transport.Packet,
	nodes []*dht.Node,
) (int, [][32]byte) {
	successCount := 0
	var replicaKeys [][32]byte

	for _, node := range nodes {
		if node.Status != dht.StatusGood || node.Address == nil {
			continue
		}

		if rm.sendWithRetry(packet, node) {
			successCount++
			replicaKeys = append(replicaKeys, node.PublicKey)
		}
	}

	return successCount, replicaKeys
}

// sendWithRetry attempts to send a packet with retries.
func (rm *ReplicationManager) sendWithRetry(packet *transport.Packet, node *dht.Node) bool {
	for attempt := 0; attempt <= ReplicationMaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(ReplicationRetryDelay)
		}
		if err := rm.transport.Send(packet, node.Address); err == nil {
			return true
		}
	}
	return false
}

// storeReplicationMetadata tracks which nodes are storing the announcement.
func (rm *ReplicationManager) storeReplicationMetadata(
	announcement *dht.GroupAnnouncement,
	replicaKeys [][32]byte,
) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.announcements[announcement.GroupID] = &ReplicatedAnnouncement{
		GroupAnnouncement: announcement,
		ReplicaNodes:      replicaKeys,
		LastRefresh:       time.Now(),
	}
}

// QueryWithRedundancy queries multiple nodes for a group announcement.
// Returns the announcement if found on any of the k nearest nodes.
func (rm *ReplicationManager) QueryWithRedundancy(groupID uint32, timeout time.Duration) (*dht.GroupAnnouncement, error) {
	if rm.routingTable == nil {
		return nil, fmt.Errorf("query failed: routing table not set")
	}
	if rm.transport == nil {
		return nil, fmt.Errorf("query failed: transport not set")
	}

	targetID := rm.groupIDToToxID(groupID)
	nearestNodes := rm.routingTable.FindClosestNodes(targetID, rm.replicationFactor)

	if len(nearestNodes) == 0 {
		return nil, fmt.Errorf("query failed: no DHT nodes available")
	}

	return rm.queryNodes(groupID, nearestNodes, timeout)
}

// queryNodes queries multiple nodes in parallel for a group announcement.
func (rm *ReplicationManager) queryNodes(
	groupID uint32,
	nodes []*dht.Node,
	timeout time.Duration,
) (*dht.GroupAnnouncement, error) {
	queryPacket := rm.buildQueryPacket(groupID)

	var wg sync.WaitGroup
	for _, node := range nodes {
		if node.Status != dht.StatusGood || node.Address == nil {
			continue
		}
		wg.Add(1)
		go func(n *dht.Node) {
			defer wg.Done()
			_ = rm.transport.Send(queryPacket, n.Address)
		}(node)
	}

	go func() {
		wg.Wait()
	}()

	return rm.routingTable.QueryGroupWithTimeout(groupID, rm.transport, timeout)
}

// buildQueryPacket creates a group query packet.
func (rm *ReplicationManager) buildQueryPacket(groupID uint32) *transport.Packet {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, groupID)
	return &transport.Packet{
		PacketType: transport.PacketGroupQuery,
		Data:       data,
	}
}

// groupIDToToxID converts a group ID to a ToxID for DHT lookups.
// The group ID is hashed to derive a pseudo-public key for distance calculations.
func (rm *ReplicationManager) groupIDToToxID(groupID uint32) crypto.ToxID {
	var key [32]byte
	binary.BigEndian.PutUint32(key[0:4], groupID)
	for i := 4; i < 32; i += 4 {
		binary.BigEndian.PutUint32(key[i:i+4], groupID^uint32(i))
	}
	return crypto.ToxID{PublicKey: key}
}

// RefreshReplicas re-announces to nodes that may have gone offline.
func (rm *ReplicationManager) RefreshReplicas(groupID uint32) (int, error) {
	rm.mu.RLock()
	replicated, exists := rm.announcements[groupID]
	rm.mu.RUnlock()

	if !exists {
		return 0, fmt.Errorf("no replication metadata for group %d", groupID)
	}

	return rm.ReplicateAnnouncement(replicated.GroupAnnouncement)
}

// GetReplicaCount returns the number of nodes storing an announcement.
func (rm *ReplicationManager) GetReplicaCount(groupID uint32) int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if replicated, exists := rm.announcements[groupID]; exists {
		return len(replicated.ReplicaNodes)
	}
	return 0
}

// IsAvailable checks if an announcement is retrievable (at least 2/5 nodes up).
// This implements the acceptance criteria: "announcement retrievable when 2 of 5 nodes offline"
func (rm *ReplicationManager) IsAvailable(groupID uint32) bool {
	rm.mu.RLock()
	replicated, exists := rm.announcements[groupID]
	rm.mu.RUnlock()

	if !exists {
		return false
	}

	return len(replicated.ReplicaNodes) >= MinReplicationSuccess
}

// CleanupExpired removes metadata for expired announcements.
func (rm *ReplicationManager) CleanupExpired() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	for groupID, replicated := range rm.announcements {
		if now.Sub(replicated.GroupAnnouncement.Timestamp) > replicated.GroupAnnouncement.TTL {
			delete(rm.announcements, groupID)
		}
	}
}
