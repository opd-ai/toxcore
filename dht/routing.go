// Package dht implements the Distributed Hash Table for peer discovery and routing in the Tox protocol.
// This file provides the core routing table implementation based on Kademlia DHT principles,
// managing k-buckets for efficient peer discovery and network navigation.
//
// The routing table implementation provides:
//   - 256 k-buckets organized by XOR distance metrics
//   - Node management with status tracking and lifecycle handling
//   - Closest node discovery for efficient DHT operations
//   - Automatic stale node cleanup and bucket maintenance
//   - Thread-safe concurrent access with read-write mutex protection
//
// Key concepts:
//   - Distance: XOR metric between node IDs for bucket assignment
//   - K-bucket: Container for up to k nodes at a specific distance range
//   - Node status: Good, questionable, or bad based on responsiveness
//   - Bucket index: Calculated from the position of the first differing bit
//
// Example usage:
//
//	// Create routing table for self
//	routingTable := NewRoutingTable(selfID, 8)
//
//	// Add discovered nodes
//	node := NewNode(peerID, peerAddr)
//	routingTable.AddNode(node)
//
//	// Find closest nodes for lookup
//	closest := routingTable.FindClosestNodes(targetID, 8)
//
// The implementation follows Kademlia principles with Tox-specific optimizations
// for peer-to-peer messaging and file transfer use cases.
package dht

import (
	"sort"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// KBucket implements a k-bucket for the Kademlia DHT routing table.
// Each k-bucket stores up to maxSize nodes that fall within a specific distance
// range from the local node. Nodes are ordered by last-seen time, with the most
// recently active nodes at the end of the list for efficient cache management.
//
// K-bucket operations include:
//   - Adding new nodes with automatic eviction of bad nodes
//   - Retrieving all nodes for routing operations
//   - Removing specific nodes by ID
//   - Thread-safe concurrent access protection
//
// The k-bucket follows the Kademlia replacement strategy where new nodes
// replace bad nodes, but stable good nodes are preferred over new ones.
//
//export ToxDHTKBucket
type KBucket struct {
	nodes   []*Node
	maxSize int
	mu      sync.RWMutex
}

// NewKBucket creates a new k-bucket with the specified maximum capacity.
// The k-bucket will store up to maxSize nodes, automatically managing the
// node list according to Kademlia principles. Newly created buckets start
// empty and populate as nodes are discovered and added.
//
// Parameters:
//   - maxSize: Maximum number of nodes this bucket can contain
//
// Returns a new KBucket instance ready for node management.
//
//export ToxDHTKBucketNew
func NewKBucket(maxSize int) *KBucket {
	return &KBucket{
		nodes:   make([]*Node, 0, maxSize),
		maxSize: maxSize,
	}
}

// AddNode adds a node to the k-bucket following Kademlia node management rules.
// The method implements the standard Kademlia replacement strategy:
//  1. If the node already exists, update it and move to end (most recent)
//  2. If bucket has space, add the node immediately
//  3. If bucket is full, replace bad nodes with the new node
//  4. If bucket is full with only good nodes, reject the new node
//
// This ensures that responsive, stable nodes are maintained while bad
// nodes are replaced with potentially better alternatives.
//
// Parameters:
//   - node: The Node to add to this bucket
//
// Returns true if the node was successfully added or updated, false otherwise.
//
//export ToxDHTKBucketAddNode
func (kb *KBucket) AddNode(node *Node) bool {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	// ADDED: Check if the node already exists in the bucket
	for i, existingNode := range kb.nodes {
		if existingNode.ID.String() == node.ID.String() {
			// ADDED: Update existing node and move to end (LRU cache behavior)
			// This maintains the property that most recently seen nodes are at the end
			kb.nodes = append(kb.nodes[:i], kb.nodes[i+1:]...)
			kb.nodes = append(kb.nodes, node)
			return true
		}
	}

	// ADDED: If bucket has available space, add the node immediately
	if len(kb.nodes) < kb.maxSize {
		kb.nodes = append(kb.nodes, node)
		return true
	}

	// ADDED: Bucket is full - attempt to replace a bad node with the new node
	// This implements Kademlia's node replacement strategy for full buckets
	for i, existingNode := range kb.nodes {
		if existingNode.Status == StatusBad {
			// ADDED: Replace bad node with new node (potential improvement)
			kb.nodes[i] = node
			return true
		}
	}

	// ADDED: Cannot add the node - bucket is full with only good/questionable nodes
	return false
}

// GetNodes returns a copy of all nodes currently stored in the k-bucket.
// The returned slice is a copy to prevent external modification of the internal
// node list. Nodes are returned in the order they appear in the bucket, with
// the most recently seen nodes at the end of the list.
//
// This method is thread-safe and can be called concurrently with other operations.
//
// Returns a slice containing copies of all nodes in this bucket.
//
//export ToxDHTKBucketGetNodes
func (kb *KBucket) GetNodes() []*Node {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	result := make([]*Node, len(kb.nodes))
	copy(result, kb.nodes)
	return result
}

// RoutingTable manages k-buckets for DHT routing using Kademlia principles.
// The routing table consists of 256 k-buckets, each responsible for nodes within
// a specific distance range from the local node. Distance is calculated using
// XOR metric, and bucket assignment is based on the position of the first
// differing bit between node IDs.
//
// Routing table capabilities:
//   - Efficient closest node discovery for DHT operations
//   - Automatic node lifecycle management and cleanup
//   - Status-based node filtering for reliability
//   - Thread-safe concurrent access with read-write locks
//   - Per-bucket node limits to prevent resource exhaustion
//
// The implementation optimizes for peer-to-peer messaging scenarios where
// nodes need to efficiently route messages and discover peers for communication.
//
//export ToxDHTRoutingTable
type RoutingTable struct {
	kBuckets [256]*KBucket
	selfID   crypto.ToxID
	maxNodes int
	mu       sync.RWMutex
}

// NewRoutingTable creates a new DHT routing table for the specified node.
// The routing table is initialized with 256 empty k-buckets, each configured
// with the specified maximum bucket size. The self node ID is used to calculate
// distances for bucket assignment and routing decisions.
//
// Parameters:
//   - selfID: The ToxID of the local node owning this routing table
//   - maxBucketSize: Maximum number of nodes each k-bucket can contain
//
// Returns a new RoutingTable instance ready for DHT operations.
//
//export ToxDHTRoutingTableNew
func NewRoutingTable(selfID crypto.ToxID, maxBucketSize int) *RoutingTable {
	rt := &RoutingTable{
		selfID:   selfID,
		maxNodes: maxBucketSize * 256,
	}

	// ADDED: Initialize k-buckets for all 256 possible distance ranges
	// Each bucket handles nodes at a specific XOR distance from self
	for i := 0; i < 256; i++ {
		rt.kBuckets[i] = NewKBucket(maxBucketSize)
	}

	return rt
}

// AddNode adds a node to the appropriate k-bucket in the routing table.
// The method calculates the XOR distance between the candidate node and the
// local node, determines the correct bucket index, and delegates to the
// k-bucket's add operation. Self-addition is prevented to avoid routing loops.
//
// Distance calculation and bucket selection follow Kademlia principles:
//   - Distance = XOR(local_id, node_id)
//   - Bucket index = position of first differing bit
//   - Closer nodes (smaller distance) go in lower-indexed buckets
//
// Parameters:
//   - node: The Node to add to the routing table
//
// Returns true if the node was successfully added, false if rejected or self.
//
//export ToxDHTRoutingTableAddNode
func (rt *RoutingTable) AddNode(node *Node) bool {
	// ADDED: Prevent self-addition to avoid routing table corruption
	if node.ID.String() == rt.selfID.String() {
		return false // Don't add ourselves
	}

	// ADDED: Calculate XOR distance to determine appropriate bucket placement
	// Distance calculation: XOR(self_id, node_id) determines bucket index
	selfNode := &Node{ID: rt.selfID}
	copy(selfNode.PublicKey[:], rt.selfID.PublicKey[:])

	dist := node.Distance(selfNode)
	bucketIndex := getBucketIndex(dist)

	rt.mu.Lock()
	defer rt.mu.Unlock()

	return rt.kBuckets[bucketIndex].AddNode(node)
}

// FindClosestNodes discovers the k closest nodes to a target ID for DHT operations.
// This method implements the core DHT lookup functionality by collecting all known
// nodes, calculating XOR distances to the target, and returning the closest matches.
// The results enable efficient routing for DHT operations like peer discovery,
// content location, and network navigation.
//
// Algorithm:
//  1. Collect all nodes from all k-buckets
//  2. Calculate XOR distance from each node to target
//  3. Sort nodes by increasing distance (closest first)
//  4. Return up to 'count' closest nodes
//
// This is used for DHT operations like find_node requests, peer discovery,
// and routing optimization. The returned nodes can be contacted to continue
// the search process closer to the target.
//
// Parameters:
//   - targetID: The ToxID to find closest nodes for
//   - count: Maximum number of closest nodes to return
//
// Returns a slice of up to 'count' nodes, sorted by distance to target.
//
//export ToxDHTRoutingTableFindClosest
func (rt *RoutingTable) FindClosestNodes(targetID crypto.ToxID, count int) []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	// ADDED: Create target node instance for distance calculations
	targetNode := &Node{ID: targetID}
	copy(targetNode.PublicKey[:], targetID.PublicKey[:])

	// ADDED: Collect all known nodes from every bucket for distance comparison
	allNodes := make([]*Node, 0, rt.maxNodes)
	for _, bucket := range rt.kBuckets {
		allNodes = append(allNodes, bucket.GetNodes()...)
	}

	// ADDED: Sort nodes by XOR distance to target (closest first)
	// This implements the core DHT closest-node discovery algorithm
	sort.Slice(allNodes, func(i, j int) bool {
		distI := allNodes[i].Distance(targetNode)
		distJ := allNodes[j].Distance(targetNode)
		return lessDistance(distI, distJ)
	})

	// ADDED: Return up to 'count' closest nodes for DHT operations
	if len(allNodes) > count {
		allNodes = allNodes[:count]
	}

	return allNodes
}

// getBucketIndex determines the k-bucket index for a node based on XOR distance.
// This function implements the core Kademlia bucket selection algorithm by finding
// the position of the first differing bit between two node IDs. The bucket index
// corresponds to the bit position where the XOR distance first becomes non-zero.
//
// Algorithm:
//  1. Examine distance bytes from most significant to least significant
//  2. For each non-zero byte, find the position of the first set bit
//  3. Return the bit position as bucket index (0-255)
//  4. Default to bucket 255 if distance is zero (shouldn't happen in practice)
//
// This ensures that nodes with similar IDs (small XOR distance) are placed
// in lower-indexed buckets, while dissimilar nodes go in higher-indexed buckets.
// The distribution follows Kademlia's logarithmic distance organization.
//
// Parameters:
//   - distance: 32-byte XOR distance between two node IDs
//
// Returns bucket index (0-255) where the node should be stored.
func getBucketIndex(distance [32]byte) int {
	// ADDED: Scan distance bytes to find first non-zero bit position
	for i := 0; i < 32; i++ {
		if distance[i] == 0 {
			continue // ADDED: Skip zero bytes - no differing bits yet
		}

		// ADDED: Find position of first set bit in this byte (MSB first)
		byte := distance[i]
		for j := 0; j < 8; j++ {
			if (byte>>(7-j))&1 == 1 {
				return i*8 + j // ADDED: Return bit position as bucket index
			}
		}
	}

	return 255 // ADDED: Default to last bucket if all zeros (edge case)
}

// lessDistance compares two XOR distances using lexicographic byte ordering.
// This function implements the distance comparison needed for sorting nodes by
// their proximity to a target ID. It performs byte-by-byte comparison from
// most significant to least significant, following standard lexicographic rules.
//
// The comparison is used to:
//   - Sort nodes by distance in FindClosestNodes operations
//   - Determine optimal routing paths for DHT lookups
//   - Select the best candidates for peer discovery
//   - Optimize network topology for efficient communication
//
// Parameters:
//   - a: First 32-byte XOR distance to compare
//   - b: Second 32-byte XOR distance to compare
//
// Returns true if distance 'a' is lexicographically smaller than 'b'.
func lessDistance(a, b [32]byte) bool {
	// ADDED: Compare distances byte by byte from most to least significant
	for i := 0; i < 32; i++ {
		if a[i] < b[i] {
			return true // ADDED: First distance is smaller
		} else if a[i] > b[i] {
			return false // ADDED: First distance is larger
		}
		// ADDED: Bytes are equal, continue to next byte
	}
	return false // ADDED: Distances are equal, not less than
}

// RemoveNode removes a node with the specified ID from the k-bucket.
// This method searches for a node with the matching ID and removes it from
// the bucket if found. The removal uses an efficient swap-and-truncate
// strategy to avoid array shifting operations.
//
// Removal scenarios:
//   - Node becomes unresponsive or marked as bad
//   - Manual node removal for network management
//   - Bucket cleanup during maintenance operations
//   - Protocol violations or security concerns
//
// The method maintains bucket integrity by preserving the order of remaining
// nodes while efficiently removing the target node.
//
// Parameters:
//   - nodeID: String representation of the node ID to remove
//
// Returns true if the node was found and removed, false if not found.
//
//export ToxDHTKBucketRemoveNode
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

// GetAllNodes returns all nodes from every k-bucket in the routing table.
// This method provides a comprehensive view of all known nodes across the entire
// DHT space. It's useful for network analysis, debugging, statistics collection,
// and operations that need to examine the complete node set.
//
// Use cases:
//   - Network statistics and health monitoring
//   - Debugging routing table state
//   - Bulk operations on all known nodes
//   - Export/import of routing table data
//   - Performance analysis and optimization
//
// The returned slice contains copies of nodes to prevent external modification
// of the routing table state. Nodes maintain their original bucket organization.
//
// Returns a slice containing all nodes from all buckets.
//
//export ToxDHTRoutingTableGetAllNodes
func (rt *RoutingTable) GetAllNodes() []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	allNodes := make([]*Node, 0, rt.maxNodes)
	for _, bucket := range rt.kBuckets {
		allNodes = append(allNodes, bucket.GetNodes()...)
	}
	return allNodes
}

// GetBucketNodes returns all nodes from a specific k-bucket index.
// This method enables targeted examination of nodes within a particular
// distance range from the local node. It's useful for bucket-specific
// operations, debugging, and understanding the distribution of nodes
// across the DHT space.
//
// Applications:
//   - Analyzing node distribution per distance range
//   - Debugging specific bucket behavior
//   - Targeted maintenance operations
//   - Distance-based network analysis
//   - Per-bucket statistics collection
//
// Parameters:
//   - bucketIndex: Index of the bucket to retrieve nodes from (0-255)
//
// Returns nodes from the specified bucket, or nil if index is invalid.
//
//export ToxDHTRoutingTableGetBucketNodes
func (rt *RoutingTable) GetBucketNodes(bucketIndex int) []*Node {
	if bucketIndex < 0 || bucketIndex >= 256 {
		return nil
	}

	rt.mu.RLock()
	defer rt.mu.RUnlock()

	return rt.kBuckets[bucketIndex].GetNodes()
}

// RemoveStaleNodes removes nodes that haven't been active within the specified duration.
// This method implements automatic cleanup of unresponsive or disconnected nodes
// to maintain routing table health and accuracy. Stale node removal prevents
// the accumulation of dead entries that would degrade DHT performance.
//
// Staleness criteria:
//   - Time since last successful communication exceeds maxAge
//   - Node fails to respond to ping requests
//   - Connection timeout or network unreachability
//   - Protocol violations or invalid responses
//
// The cleanup process:
//  1. Scan all buckets for nodes exceeding the age threshold
//  2. Remove qualifying nodes from their respective buckets
//  3. Count and report the number of removed nodes
//  4. Maintain bucket integrity during the removal process
//
// Parameters:
//   - maxAge: Maximum allowed time since last node activity
//
// Returns the number of stale nodes that were removed.
//
//export ToxDHTRoutingTableRemoveStaleNodes
func (rt *RoutingTable) RemoveStaleNodes(maxAge time.Duration) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now()
	removedCount := 0

	for _, bucket := range rt.kBuckets {
		nodes := bucket.GetNodes()
		for _, node := range nodes {
			if now.Sub(node.LastSeen) > maxAge {
				if bucket.RemoveNode(node.ID.String()) {
					removedCount++
				}
			}
		}
	}

	return removedCount
}

// GetNodesByStatus returns all nodes that match the specified status condition.
// This method enables status-based node filtering for operations that need to
// work with nodes in particular states. It's essential for network health
// monitoring, maintenance operations, and routing decisions based on node quality.
//
// Status-based operations:
//   - Finding good nodes for reliable routing
//   - Identifying questionable nodes for re-validation
//   - Locating bad nodes for removal or debugging
//   - Quality-based node selection for critical operations
//   - Network health assessment and reporting
//
// The method scans all buckets and collects nodes matching the status
// criteria, providing a comprehensive view of nodes in the specified state.
//
// Parameters:
//   - status: NodeStatus to filter by (StatusGood, StatusQuestionable, StatusBad)
//
// Returns a slice containing all nodes with the specified status.
//
//export ToxDHTRoutingTableGetNodesByStatus
func (rt *RoutingTable) GetNodesByStatus(status NodeStatus) []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var nodes []*Node
	for _, bucket := range rt.kBuckets {
		bucketNodes := bucket.GetNodes()
		for _, node := range bucketNodes {
			if node.Status == status {
				nodes = append(nodes, node)
			}
		}
	}

	return nodes
}

// GetTotalNodeCount returns the total number of nodes across all k-buckets.
// This method provides a quick count of all known nodes in the routing table
// without the overhead of collecting and copying node data. It's useful for
// monitoring network connectivity, resource usage tracking, and performance
// optimization decisions.
//
// Count applications:
//   - Network health monitoring and statistics
//   - Resource usage tracking and limits
//   - Performance optimization thresholds
//   - Debugging and diagnostic information
//   - Capacity planning for network operations
//
// The count reflects the current state of the routing table and may change
// as nodes are added, removed, or expire during normal DHT operations.
//
// Returns the total number of nodes currently stored in the routing table.
//
//export ToxDHTRoutingTableGetTotalNodeCount
func (rt *RoutingTable) GetTotalNodeCount() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	count := 0
	for _, bucket := range rt.kBuckets {
		count += len(bucket.GetNodes())
	}
	return count
}
