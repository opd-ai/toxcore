package dht

import (
	"container/heap"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

const (
	// DefaultLookupCacheTTL is the default time-to-live for cached lookup results.
	// Cached results older than this will be refreshed on next lookup.
	DefaultLookupCacheTTL = 30 * time.Second

	// DefaultLookupCacheMaxSize is the maximum number of cached lookup results.
	DefaultLookupCacheMaxSize = 256
)

// lookupCacheEntry stores a cached FindClosestNodes result with timestamp.
type lookupCacheEntry struct {
	nodes     []*Node
	timestamp time.Time
}

// LookupCache provides TTL-based caching for DHT node lookups.
// This reduces repeated expensive lookups for the same target.
type LookupCache struct {
	mu      sync.RWMutex
	entries map[[32]byte]*lookupCacheEntry
	ttl     time.Duration
	maxSize int
	hits    uint64 // Statistics: cache hits
	misses  uint64 // Statistics: cache misses
}

// NewLookupCache creates a new lookup cache with the given TTL and max size.
func NewLookupCache(ttl time.Duration, maxSize int) *LookupCache {
	if ttl <= 0 {
		ttl = DefaultLookupCacheTTL
	}
	if maxSize <= 0 {
		maxSize = DefaultLookupCacheMaxSize
	}
	return &LookupCache{
		entries: make(map[[32]byte]*lookupCacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Get retrieves cached nodes for a target, returns nil if not found or expired.
func (lc *LookupCache) Get(targetKey [32]byte) []*Node {
	lc.mu.RLock()
	entry, exists := lc.entries[targetKey]
	lc.mu.RUnlock()

	if !exists {
		lc.mu.Lock()
		lc.misses++
		lc.mu.Unlock()
		return nil
	}

	// Check if entry has expired
	if time.Since(entry.timestamp) > lc.ttl {
		lc.mu.Lock()
		delete(lc.entries, targetKey)
		lc.misses++
		lc.mu.Unlock()
		return nil
	}

	lc.mu.Lock()
	lc.hits++
	lc.mu.Unlock()

	// Return a copy to prevent external modification
	result := make([]*Node, len(entry.nodes))
	copy(result, entry.nodes)
	return result
}

// Put stores nodes for a target in the cache.
func (lc *LookupCache) Put(targetKey [32]byte, nodes []*Node) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Evict oldest entries if at capacity (simple FIFO eviction)
	if len(lc.entries) >= lc.maxSize {
		lc.evictOldestLocked()
	}

	// Store a copy of the nodes
	nodesCopy := make([]*Node, len(nodes))
	copy(nodesCopy, nodes)

	lc.entries[targetKey] = &lookupCacheEntry{
		nodes:     nodesCopy,
		timestamp: time.Now(),
	}
}

// evictOldestLocked removes the oldest entry from the cache. Must be called with lock held.
func (lc *LookupCache) evictOldestLocked() {
	var oldestKey [32]byte
	var oldestTime time.Time
	first := true

	for key, entry := range lc.entries {
		if first || entry.timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.timestamp
			first = false
		}
	}

	if !first {
		delete(lc.entries, oldestKey)
	}
}

// Clear removes all entries from the cache.
func (lc *LookupCache) Clear() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.entries = make(map[[32]byte]*lookupCacheEntry)
}

// Stats returns cache hit/miss statistics.
func (lc *LookupCache) Stats() (hits, misses uint64) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.hits, lc.misses
}

// Size returns the current number of cached entries.
func (lc *LookupCache) Size() int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return len(lc.entries)
}

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

	// Check if the node already exists (direct byte comparison avoids hex allocation)
	for i, existingNode := range kb.nodes {
		if existingNode.ID.PublicKey == node.ID.PublicKey {
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

	// Relay storage for DHT-based relay server discovery
	relayStorage *RelayStorage

	// Lookup cache for reducing repeated FindClosestNodes queries
	lookupCache *LookupCache
}

// NewRoutingTable creates a new DHT routing table.
//
//export ToxDHTRoutingTableNew
func NewRoutingTable(selfID crypto.ToxID, maxBucketSize int) *RoutingTable {
	rt := &RoutingTable{
		selfID:       selfID,
		maxNodes:     maxBucketSize * 256,
		groupStorage: NewGroupStorage(), // Initialize group storage for DHT discovery
		relayStorage: NewRelayStorage(), // Initialize relay storage for relay server discovery
		lookupCache:  NewLookupCache(DefaultLookupCacheTTL, DefaultLookupCacheMaxSize),
	}

	// Initialize k-buckets
	for i := 0; i < 256; i++ {
		rt.kBuckets[i] = NewKBucket(maxBucketSize)
	}

	return rt
}

// AddNode adds a node to the appropriate k-bucket in the routing table.
// If successful, this invalidates the lookup cache since the routing table changed.
//
//export ToxDHTRoutingTableAddNode
func (rt *RoutingTable) AddNode(node *Node) bool {
	if node.ID.PublicKey == rt.selfID.PublicKey {
		return false // Don't add ourselves
	}

	bucketIndex := computeBucketIndex(rt.selfID, node)

	rt.mu.Lock()
	added := rt.kBuckets[bucketIndex].AddNode(node)
	rt.mu.Unlock()

	// Invalidate cache if node was added (routing table changed)
	if added && rt.lookupCache != nil {
		rt.lookupCache.Clear()
	}

	return added
}

// nodeHeap implements heap.Interface for finding closest nodes efficiently.
// It's a max-heap based on distance, keeping the k closest nodes.
type nodeHeap struct {
	nodes      []*Node
	distances  [][32]byte
	targetNode *Node
}

// Len returns the number of elements in the heap.
func (h *nodeHeap) Len() int { return len(h.nodes) }

// Less reports whether element i should sort before element j.
func (h *nodeHeap) Less(i, j int) bool {
	// Max-heap: return true if i is farther than j
	return !lessDistance(h.distances[i], h.distances[j])
}

// Swap exchanges the elements at indices i and j.
func (h *nodeHeap) Swap(i, j int) {
	h.nodes[i], h.nodes[j] = h.nodes[j], h.nodes[i]
	h.distances[i], h.distances[j] = h.distances[j], h.distances[i]
}

// Push adds an element to the heap.
func (h *nodeHeap) Push(x interface{}) {
	item, ok := x.(*Node)
	if !ok {
		// This should never occur because all callers pass *Node; silently drop
		// the invalid value rather than crashing the application.
		return
	}
	h.nodes = append(h.nodes, item)
	h.distances = append(h.distances, item.Distance(h.targetNode))
}

// Pop removes and returns the maximum element from the heap.
func (h *nodeHeap) Pop() interface{} {
	old := h.nodes
	n := len(old)
	item := old[n-1]
	h.nodes = old[0 : n-1]
	h.distances = h.distances[0 : n-1]
	return item
}

// FindClosestNodes finds the k closest nodes to the given target ID.
// Results are cached with a TTL to reduce repeated expensive lookups.
//
//export ToxDHTRoutingTableFindClosest
func (rt *RoutingTable) FindClosestNodes(targetID crypto.ToxID, count int) []*Node {
	if count <= 0 {
		return []*Node{}
	}

	// Check cache first (cache handles its own locking)
	if rt.lookupCache != nil {
		if cached := rt.lookupCache.Get(targetID.PublicKey); cached != nil {
			// Return up to count nodes from cache
			if len(cached) > count {
				return cached[:count]
			}
			return cached
		}
	}

	rt.mu.RLock()
	targetNode := rt.createTargetNode(targetID)
	h := rt.buildNodeHeap(targetNode, count)
	result := rt.extractSortedNodes(h)
	rt.mu.RUnlock()

	// Cache the result
	if rt.lookupCache != nil && len(result) > 0 {
		rt.lookupCache.Put(targetID.PublicKey, result)
	}

	return result
}

// FindClosestNodesNoCache finds closest nodes without using the cache.
// Use this for lookups that need fresh results regardless of cache state.
func (rt *RoutingTable) FindClosestNodesNoCache(targetID crypto.ToxID, count int) []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if count <= 0 {
		return []*Node{}
	}

	targetNode := rt.createTargetNode(targetID)
	h := rt.buildNodeHeap(targetNode, count)
	return rt.extractSortedNodes(h)
}

// createTargetNode creates a node instance for distance calculations.
func (rt *RoutingTable) createTargetNode(targetID crypto.ToxID) *Node {
	targetNode := &Node{ID: targetID}
	copy(targetNode.PublicKey[:], targetID.PublicKey[:])
	return targetNode
}

// buildNodeHeap constructs a max-heap of closest nodes to the target.
// Starts scanning from the target's bucket index and expands outward,
// which populates the heap with likely-closest nodes first. This makes
// the replaceIfCloser check reject distant nodes faster, reducing
// unnecessary heap operations.
func (rt *RoutingTable) buildNodeHeap(targetNode *Node, count int) *nodeHeap {
	h := &nodeHeap{
		nodes:      make([]*Node, 0, count),
		distances:  make([][32]byte, 0, count),
		targetNode: targetNode,
	}

	// Calculate the target bucket index to start scanning from closest buckets first
	var dist [32]byte
	for i := range dist {
		dist[i] = targetNode.PublicKey[i] ^ rt.selfID.PublicKey[i]
	}
	targetBucket := getBucketIndex(dist)

	// Process the target bucket first
	rt.processNodesInBucket(rt.kBuckets[targetBucket], h, count)

	// Expand outward from the target bucket in both directions
	for offset := 1; offset < 256; offset++ {
		lo := targetBucket - offset
		hi := targetBucket + offset
		if lo < 0 && hi >= 256 {
			break // Both directions exhausted
		}
		if lo >= 0 {
			rt.processNodesInBucket(rt.kBuckets[lo], h, count)
		}
		if hi < 256 {
			rt.processNodesInBucket(rt.kBuckets[hi], h, count)
		}
	}

	return h
}

// processNodesInBucket adds nodes from a bucket to the heap, maintaining k-closest invariant.
func (rt *RoutingTable) processNodesInBucket(bucket *KBucket, h *nodeHeap, count int) {
	nodes := bucket.GetNodes()
	for _, node := range nodes {
		rt.addNodeToHeap(h, node, count)
	}
}

// addNodeToHeap adds a node to the heap if it's among the k-closest.
func (rt *RoutingTable) addNodeToHeap(h *nodeHeap, node *Node, count int) {
	if len(h.nodes) < count {
		heap.Push(h, node)
	} else {
		rt.replaceIfCloser(h, node)
	}
}

// replaceIfCloser replaces the farthest node in heap if new node is closer.
func (rt *RoutingTable) replaceIfCloser(h *nodeHeap, node *Node) {
	dist := node.Distance(h.targetNode)
	if lessDistance(dist, h.distances[0]) {
		heap.Pop(h)
		heap.Push(h, node)
	}
}

// extractSortedNodes extracts nodes from heap in closest-first order.
func (rt *RoutingTable) extractSortedNodes(h *nodeHeap) []*Node {
	heapSize := h.Len()
	result := make([]*Node, heapSize)

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

// computeBucketIndex calculates the bucket index for a node relative to a self ID.
// It creates a temporary node from selfID for distance calculation.
func computeBucketIndex(selfID crypto.ToxID, node *Node) int {
	selfNode := &Node{ID: selfID}
	copy(selfNode.PublicKey[:], selfID.PublicKey[:])
	dist := node.Distance(selfNode)
	return getBucketIndex(dist)
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

// RemoveNode removes a node with the given public key from the k-bucket if it exists.
// Returns true if the node was found and removed, false otherwise.
// Uses direct byte comparison instead of string conversion for efficiency.
//
//export toxDHTRoutingTableRemoveNode
func (kb *KBucket) RemoveNode(publicKey [32]byte) bool {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	for i, node := range kb.nodes {
		if node.ID.PublicKey == publicKey {
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

// GetLookupCacheStats returns cache hit/miss statistics for monitoring.
func (rt *RoutingTable) GetLookupCacheStats() (hits, misses uint64) {
	if rt.lookupCache == nil {
		return 0, 0
	}
	return rt.lookupCache.Stats()
}

// ClearLookupCache manually clears the lookup cache.
// This is automatically called when nodes are added, but can be called
// manually if needed (e.g., after bulk routing table updates).
func (rt *RoutingTable) ClearLookupCache() {
	if rt.lookupCache != nil {
		rt.lookupCache.Clear()
	}
}
