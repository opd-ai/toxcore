package dht

import (
	"container/heap"
	"sync"

	"github.com/opd-ai/toxcore/crypto"
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

	// Group storage for DHT-based group discovery
	groupStorage *GroupStorage
}

// NewRoutingTable creates a new DHT routing table.
//
//export ToxDHTRoutingTableNew
func NewRoutingTable(selfID crypto.ToxID, maxBucketSize int) *RoutingTable {
	rt := &RoutingTable{
		selfID:       selfID,
		maxNodes:     maxBucketSize * 256,
		groupStorage: NewGroupStorage(), // Initialize group storage for DHT discovery
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

// nodeHeap implements heap.Interface for finding closest nodes efficiently.
// It's a max-heap based on distance, keeping the k closest nodes.
type nodeHeap struct {
	nodes      []*Node
	distances  [][32]byte
	targetNode *Node
}

func (h *nodeHeap) Len() int { return len(h.nodes) }

func (h *nodeHeap) Less(i, j int) bool {
	// Max-heap: return true if i is farther than j
	return !lessDistance(h.distances[i], h.distances[j])
}

func (h *nodeHeap) Swap(i, j int) {
	h.nodes[i], h.nodes[j] = h.nodes[j], h.nodes[i]
	h.distances[i], h.distances[j] = h.distances[j], h.distances[i]
}

func (h *nodeHeap) Push(x interface{}) {
	item := x.(*Node)
	h.nodes = append(h.nodes, item)
	h.distances = append(h.distances, item.Distance(h.targetNode))
}

func (h *nodeHeap) Pop() interface{} {
	old := h.nodes
	n := len(old)
	item := old[n-1]
	h.nodes = old[0 : n-1]
	h.distances = h.distances[0 : n-1]
	return item
}

// FindClosestNodes finds the k closest nodes to the given target ID.
//
//export ToxDHTRoutingTableFindClosest
func (rt *RoutingTable) FindClosestNodes(targetID crypto.ToxID, count int) []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if count <= 0 {
		return []*Node{}
	}

	targetNode := &Node{ID: targetID}
	copy(targetNode.PublicKey[:], targetID.PublicKey[:])

	// Use a max-heap to maintain only the k closest nodes
	// This avoids collecting and sorting all nodes
	h := &nodeHeap{
		nodes:      make([]*Node, 0, count),
		distances:  make([][32]byte, 0, count),
		targetNode: targetNode,
	}

	// Iterate through all buckets and maintain heap of closest nodes
	for _, bucket := range rt.kBuckets {
		nodes := bucket.GetNodes()
		for _, node := range nodes {
			if len(h.nodes) < count {
				// Heap not full yet, just add the node
				heap.Push(h, node)
			} else {
				// Heap is full, check if this node is closer than the farthest
				dist := node.Distance(targetNode)
				if lessDistance(dist, h.distances[0]) {
					// This node is closer, replace the farthest
					heap.Pop(h)
					heap.Push(h, node)
				}
			}
		}
	}

	// Extract nodes from heap in order
	// Since this is a max-heap (farthest at root), popping gives us nodes
	// from farthest to closest. We reverse to get closest first.
	heapSize := h.Len()
	result := make([]*Node, heapSize)

	// Pop all nodes (gives farthest to closest order)
	for i := heapSize - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(*Node)
	}

	return result
}

// GetAllNodes returns all nodes from all k-buckets in the routing table.
// This is useful for operations that need to search all known peers,
// such as reverse address resolution.
func (rt *RoutingTable) GetAllNodes() []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var allNodes []*Node
	for _, bucket := range rt.kBuckets {
		nodes := bucket.GetNodes()
		allNodes = append(allNodes, nodes...)
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

// SetGroupResponseCallback registers a callback to be notified when group query responses are received.
// This allows the group layer to handle DHT responses without circular dependencies.
func (rt *RoutingTable) SetGroupResponseCallback(callback GroupQueryResponseCallback) {
	if rt.groupStorage != nil {
		rt.groupStorage.SetResponseCallback(callback)
	}
}

// HandleGroupQueryResponse processes a group query response received from the DHT network.
// This method is called by the BootstrapManager when a response packet is received.
func (rt *RoutingTable) HandleGroupQueryResponse(announcement *GroupAnnouncement) {
	if rt.groupStorage != nil && announcement != nil {
		rt.groupStorage.StoreAnnouncement(announcement)
		rt.groupStorage.notifyResponse(announcement)
	}
}

// ...existing code...
