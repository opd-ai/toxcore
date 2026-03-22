// Package dht provides distributed hash table functionality for the Tox protocol.
// This file contains common storage utilities shared across different announcement types.
package dht

import (
	"fmt"
	"net"

	"github.com/opd-ai/toxcore/transport"
)

// broadcastAnnouncement sends an announcement packet to all good nodes in the routing table.
// It broadcasts to all known good nodes and retries once on transient failures.
// Returns an error if no DHT nodes could be reached.
func (rt *RoutingTable) broadcastAnnouncement(packet *transport.Packet, tr transport.Transport, announcementType string) error {
	if tr == nil {
		return fmt.Errorf("transport is nil")
	}

	nodes := rt.collectBroadcastNodes()
	successCount := rt.sendToNodes(packet, tr, nodes)

	if len(nodes) > 0 && successCount == 0 {
		return fmt.Errorf("%s announcement failed: could not reach any of %d DHT nodes", announcementType, len(nodes))
	}

	return nil
}

// collectBroadcastNodes gathers all nodes from k-buckets for broadcasting.
func (rt *RoutingTable) collectBroadcastNodes() []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var nodes []*Node
	for _, bucket := range rt.kBuckets {
		nodes = append(nodes, bucket.GetNodes()...)
	}
	return nodes
}

// sendToNodes sends a packet to all good nodes, retrying once on failure.
func (rt *RoutingTable) sendToNodes(packet *transport.Packet, tr transport.Transport, nodes []*Node) int {
	successCount := 0
	for _, node := range nodes {
		if node.Status != StatusGood || node.Address == nil {
			continue
		}
		if rt.sendWithRetry(packet, tr, node.Address) {
			successCount++
		}
	}
	return successCount
}

// sendWithRetry sends a packet and retries once on failure.
func (rt *RoutingTable) sendWithRetry(packet *transport.Packet, tr transport.Transport, addr net.Addr) bool {
	if err := tr.Send(packet, addr); err != nil {
		return tr.Send(packet, addr) == nil
	}
	return true
}
