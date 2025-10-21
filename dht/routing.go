package dht

import (
	"sort"
	"sync"

	"github.com/opd-ai/toxforge/crypto"
)

// KBucket implements a k-bucket for the Kademlia DHT.
type KBucket struct {
	nodes   []*Node
	maxSize int
	mu      sync.RWMutex
}

// NewKBucket creates a new k-bucket with the specified maximum size.
func NewKBucket(maxSize int) *KBucket {
	return &KBucket{
		nodes:   make([]*Node, 0, maxSize),
		maxSize: maxSize,
	}
}

// AddNode adds a node to the k-bucket if there is space or if it's better than an existing node.
func (kb *KBucket) AddNode(node *Node) bool {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	// Check if the node already exists
	for i, existingNode := range kb.nodes {
		if existingNode.ID.String() == node.ID.String() {
			// Update the existing node and move it to the end (most recently seen)
			kb.nodes = append(kb.nodes[:i], kb.nodes[i+1:]...)
			kb.nodes = append(kb.nodes, node)
			return true
		}
	}

	// If the bucket isn't full, add the node
	if len(kb.nodes) < kb.maxSize {
		kb.nodes = append(kb.nodes, node)
		return true
	}

	// The bucket is full, check if we can replace a bad node
	for i, existingNode := range kb.nodes {
		if existingNode.Status == StatusBad {
			kb.nodes[i] = node
			return true
		}
	}

	// Cannot add the node
	return false
}

// GetNodes returns a copy of all nodes in the k-bucket.
func (kb *KBucket) GetNodes() []*Node {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	result := make([]*Node, len(kb.nodes))
	copy(result, kb.nodes)
	return result
}

// RoutingTable manages k-buckets for the DHT routing.
//
//export ToxDHTRoutingTable
type RoutingTable struct {
	kBuckets [256]*KBucket
	selfID   crypto.ToxID
	maxNodes int
	mu       sync.RWMutex
}

// NewRoutingTable creates a new DHT routing table.
//
//export ToxDHTRoutingTableNew
func NewRoutingTable(selfID crypto.ToxID, maxBucketSize int) *RoutingTable {
	rt := &RoutingTable{
		selfID:   selfID,
		maxNodes: maxBucketSize * 256,
	}

	// Initialize k-buckets
	for i := 0; i < 256; i++ {
		rt.kBuckets[i] = NewKBucket(maxBucketSize)
	}

	return rt
}

// AddNode adds a node to the appropriate k-bucket in the routing table.
//
//export ToxDHTRoutingTableAddNode
func (rt *RoutingTable) AddNode(node *Node) bool {
	if node.ID.String() == rt.selfID.String() {
		return false // Don't add ourselves
	}

	// Calculate distance to determine bucket index
	selfNode := &Node{ID: rt.selfID}
	copy(selfNode.PublicKey[:], rt.selfID.PublicKey[:])

	dist := node.Distance(selfNode)
	bucketIndex := getBucketIndex(dist)

	rt.mu.Lock()
	defer rt.mu.Unlock()

	return rt.kBuckets[bucketIndex].AddNode(node)
}

// FindClosestNodes finds the k closest nodes to the given target ID.
//
//export ToxDHTRoutingTableFindClosest
func (rt *RoutingTable) FindClosestNodes(targetID crypto.ToxID, count int) []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	targetNode := &Node{ID: targetID}
	copy(targetNode.PublicKey[:], targetID.PublicKey[:])

	// Collect all nodes
	allNodes := make([]*Node, 0, rt.maxNodes)
	for _, bucket := range rt.kBuckets {
		allNodes = append(allNodes, bucket.GetNodes()...)
	}

	// Sort by distance to target
	sort.Slice(allNodes, func(i, j int) bool {
		distI := allNodes[i].Distance(targetNode)
		distJ := allNodes[j].Distance(targetNode)
		return lessDistance(distI, distJ)
	})

	// Return the closest nodes, up to count
	if len(allNodes) > count {
		allNodes = allNodes[:count]
	}

	return allNodes
}

// getBucketIndex determines which k-bucket a node belongs in based on distance.
func getBucketIndex(distance [32]byte) int {
	// Find the index of the first bit that is 1
	for i := 0; i < 32; i++ {
		if distance[i] == 0 {
			continue
		}

		// Find the position of the first 1 bit
		byte := distance[i]
		for j := 0; j < 8; j++ {
			if (byte>>(7-j))&1 == 1 {
				return i*8 + j
			}
		}
	}

	return 255 // Default to last bucket if all zeros (shouldn't happen)
}

// lessDistance compares two distances and returns true if a is less than b.
func lessDistance(a, b [32]byte) bool {
	for i := 0; i < 32; i++ {
		if a[i] < b[i] {
			return true
		} else if a[i] > b[i] {
			return false
		}
	}
	return false
}

// RemoveNode removes a node with the given ID from the k-bucket if it exists.
// Returns true if the node was found and removed, false otherwise.
//
//export toxDHTRoutingTableRemoveNode
func (kb *KBucket) RemoveNode(nodeID string) bool {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	for i, node := range kb.nodes {
		if node.ID.String() == nodeID {
			// Remove the node by replacing it with the last node in the slice
			// and then truncating the slice (more efficient than creating a new slice)
			lastIndex := len(kb.nodes) - 1
			if i != lastIndex {
				kb.nodes[i] = kb.nodes[lastIndex]
			}
			kb.nodes = kb.nodes[:lastIndex]
			return true
		}
	}

	return false
}

// ...existing code...
