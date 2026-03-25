package dht

import (
	"context"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

const (
	// Alpha is the standard Kademlia concurrency parameter.
	// The number of parallel lookups to perform at each step.
	Alpha = 3

	// DefaultLookupK is the number of closest nodes to find.
	DefaultLookupK = 8

	// DefaultLookupTimeout is the maximum time to wait for a single lookup step.
	DefaultLookupTimeout = 5 * time.Second

	// DefaultMaxLookupIterations is the maximum number of lookup iterations.
	DefaultMaxLookupIterations = 10

	// DefaultResponseTimeout is how long to wait for a response from a node.
	DefaultResponseTimeout = 3 * time.Second
)

// IterativeLookupResult contains the result of an iterative node lookup.
type IterativeLookupResult struct {
	// ClosestNodes contains the k closest nodes found to the target.
	ClosestNodes []*Node

	// QueriedNodes is the set of all nodes that were queried.
	QueriedNodes map[[32]byte]struct{}

	// Iterations is the number of lookup iterations performed.
	Iterations int

	// Duration is the total time the lookup took.
	Duration time.Duration

	// Success indicates if the lookup completed successfully.
	Success bool

	// Error, if any, that occurred during the lookup.
	Error error
}

// LookupConfig configures the iterative lookup behavior.
type LookupConfig struct {
	// Alpha is the number of parallel queries per iteration.
	Alpha int

	// K is the number of closest nodes to find.
	K int

	// Timeout is the maximum time for the entire lookup.
	Timeout time.Duration

	// ResponseTimeout is how long to wait for each node response.
	ResponseTimeout time.Duration

	// MaxIterations limits the number of lookup iterations.
	MaxIterations int
}

// DefaultLookupConfig returns the standard Kademlia lookup configuration.
func DefaultLookupConfig() *LookupConfig {
	return &LookupConfig{
		Alpha:           Alpha,
		K:               DefaultLookupK,
		Timeout:         30 * time.Second,
		ResponseTimeout: DefaultResponseTimeout,
		MaxIterations:   DefaultMaxLookupIterations,
	}
}

// nodeQueryResult represents the result of querying a single node.
type nodeQueryResult struct {
	queried   *Node
	responses []*Node
	err       error
}

// IterativeLookup performs a Kademlia-style iterative lookup for a target.
// It queries alpha nodes in parallel at each step, progressively getting closer
// to the target until no closer nodes are found.
type IterativeLookup struct {
	routingTable *RoutingTable
	transport    transport.Transport
	selfID       crypto.ToxID
	config       *LookupConfig

	// Response channel for receiving query results
	responsesMu      sync.Mutex
	pendingResponses map[[32]byte]chan []*Node

	// Time provider for testing
	timeProvider TimeProvider
}

// NewIterativeLookup creates a new iterative lookup instance.
func NewIterativeLookup(rt *RoutingTable, tr transport.Transport, selfID crypto.ToxID, config *LookupConfig) *IterativeLookup {
	if config == nil {
		config = DefaultLookupConfig()
	}
	return &IterativeLookup{
		routingTable:     rt,
		transport:        tr,
		selfID:           selfID,
		config:           config,
		pendingResponses: make(map[[32]byte]chan []*Node),
	}
}

// SetTimeProvider sets a custom time provider for testing.
func (il *IterativeLookup) SetTimeProvider(tp TimeProvider) {
	il.timeProvider = tp
}

// getTime returns the current time.
func (il *IterativeLookup) getTime() time.Time {
	if il.timeProvider != nil {
		return il.timeProvider.Now()
	}
	return time.Now()
}

// FindNode performs an iterative lookup for the target public key.
// Returns the k closest nodes found and lookup statistics.
func (il *IterativeLookup) FindNode(ctx context.Context, targetKey [32]byte) *IterativeLookupResult {
	startTime := il.getTime()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, il.config.Timeout)
	defer cancel()

	result := &IterativeLookupResult{
		QueriedNodes: make(map[[32]byte]struct{}),
	}

	// Initialize with closest nodes from our routing table
	targetNode, candidates, err := il.initializeCandidates(targetKey)
	if err != nil {
		result.Error = err
		result.Duration = il.getTime().Sub(startTime)
		return result
	}

	// Perform iterative lookup
	for iteration := 0; iteration < il.config.MaxIterations; iteration++ {
		result.Iterations = iteration + 1

		// Select alpha unqueried nodes closest to target
		nodesToQuery := candidates.selectUnqueried(il.config.Alpha, result.QueriedNodes)
		if len(nodesToQuery) == 0 {
			break
		}

		// Query nodes in parallel and process responses
		foundCloser := il.queryAndProcessResponses(ctx, nodesToQuery, candidates, targetNode, result)

		if !foundCloser {
			break
		}

		// Check context cancellation
		if err := il.checkContextCancellation(ctx, result, candidates, startTime); err != nil {
			return result
		}
	}

	// Return the k closest nodes found
	result.ClosestNodes = candidates.getClosest(il.config.K)
	result.Success = len(result.ClosestNodes) > 0
	result.Duration = il.getTime().Sub(startTime)
	return result
}

// initializeCandidates sets up the initial candidate set from the routing table.
func (il *IterativeLookup) initializeCandidates(targetKey [32]byte) (*Node, *nodeSet, error) {
	var targetNospam [4]byte
	targetID := crypto.NewToxID(targetKey, targetNospam)
	initialNodes := il.routingTable.FindClosestNodes(*targetID, il.config.K)

	if len(initialNodes) == 0 {
		return nil, nil, ErrNoNodesAvailable
	}

	// Create target node for distance calculations
	targetNode := &Node{ID: *targetID}
	copy(targetNode.PublicKey[:], targetKey[:])

	// Candidate set: nodes to potentially query, sorted by distance
	candidates := newNodeSet(targetNode, il.config.K*3)
	for _, n := range initialNodes {
		candidates.add(n)
	}

	return targetNode, candidates, nil
}

// queryAndProcessResponses queries nodes in parallel and processes their responses.
// Returns true if any discovered node is closer than the previously closest.
func (il *IterativeLookup) queryAndProcessResponses(
	ctx context.Context,
	nodesToQuery []*Node,
	candidates *nodeSet,
	targetNode *Node,
	result *IterativeLookupResult,
) bool {
	responsesChan := make(chan *nodeQueryResult, len(nodesToQuery))
	var wg sync.WaitGroup

	// Mark nodes as queried and launch parallel queries
	for _, node := range nodesToQuery {
		result.QueriedNodes[node.PublicKey] = struct{}{}
		wg.Add(1)
		go func(n *Node) {
			defer wg.Done()
			responses, err := il.queryNode(ctx, n, targetNode.PublicKey)
			responsesChan <- &nodeQueryResult{
				queried:   n,
				responses: responses,
				err:       err,
			}
		}(node)
	}

	// Close channel when all queries complete
	go func() {
		wg.Wait()
		close(responsesChan)
	}()

	// Collect responses and check for closer nodes
	return il.processDiscoveredNodes(responsesChan, candidates, targetNode, result)
}

// processDiscoveredNodes adds discovered nodes to candidates and checks if any are closer.
func (il *IterativeLookup) processDiscoveredNodes(
	responsesChan <-chan *nodeQueryResult,
	candidates *nodeSet,
	targetNode *Node,
	result *IterativeLookupResult,
) bool {
	closestBefore := candidates.closestDistance()
	foundCloser := false

	for resp := range responsesChan {
		if resp.err != nil {
			continue
		}
		if il.addDiscoveredNodes(resp.responses, candidates, targetNode, result.QueriedNodes, closestBefore) {
			foundCloser = true
		}
	}

	return foundCloser
}

// addDiscoveredNodes adds a batch of discovered nodes to candidates, returning true if any are closer.
func (il *IterativeLookup) addDiscoveredNodes(
	nodes []*Node, candidates *nodeSet, targetNode *Node,
	queriedNodes map[[32]byte]struct{}, closestBefore [32]byte,
) bool {
	foundCloser := false
	for _, discovered := range nodes {
		if il.shouldSkipNode(discovered, queriedNodes) {
			continue
		}
		if candidates.add(discovered) {
			if lessDistance(discovered.Distance(targetNode), closestBefore) {
				foundCloser = true
			}
		}
	}
	return foundCloser
}

// shouldSkipNode returns true if the node should not be added to candidates.
func (il *IterativeLookup) shouldSkipNode(node *Node, queriedNodes map[[32]byte]struct{}) bool {
	if _, queried := queriedNodes[node.PublicKey]; queried {
		return true
	}
	return node.PublicKey == il.selfID.PublicKey
}

// checkContextCancellation checks if the context is cancelled and updates result.
func (il *IterativeLookup) checkContextCancellation(
	ctx context.Context,
	result *IterativeLookupResult,
	candidates *nodeSet,
	startTime time.Time,
) error {
	select {
	case <-ctx.Done():
		result.Error = ctx.Err()
		result.Duration = il.getTime().Sub(startTime)
		result.ClosestNodes = candidates.getClosest(il.config.K)
		return ctx.Err()
	default:
		return nil
	}
}

// queryNode sends a FIND_NODE request to a node and waits for response.
func (il *IterativeLookup) queryNode(ctx context.Context, node *Node, targetKey [32]byte) ([]*Node, error) {
	// Create response channel for this query
	responseChan := make(chan []*Node, 1)

	il.responsesMu.Lock()
	il.pendingResponses[node.PublicKey] = responseChan
	il.responsesMu.Unlock()

	defer func() {
		il.responsesMu.Lock()
		delete(il.pendingResponses, node.PublicKey)
		il.responsesMu.Unlock()
	}()

	// Create and send FIND_NODE packet
	data := make([]byte, 64)
	copy(data[:32], il.selfID.PublicKey[:]) // Our public key
	copy(data[32:], targetKey[:])           // Target key

	packet := &transport.Packet{
		PacketType: transport.PacketGetNodes,
		Data:       data,
	}

	if err := il.transport.Send(packet, node.Address); err != nil {
		return nil, err
	}

	// Wait for response with timeout
	select {
	case nodes := <-responseChan:
		return nodes, nil
	case <-time.After(il.config.ResponseTimeout):
		return nil, ErrQueryTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HandleNodesResponse handles incoming NODES response packets.
// This should be called from the transport layer when a NODES response is received.
func (il *IterativeLookup) HandleNodesResponse(fromKey [32]byte, nodes []*Node) {
	il.responsesMu.Lock()
	ch, exists := il.pendingResponses[fromKey]
	il.responsesMu.Unlock()

	if exists {
		select {
		case ch <- nodes:
		default:
			// Channel full or closed; ignore
		}
	}
}

// nodeSet maintains a distance-sorted set of nodes with capacity limit.
type nodeSet struct {
	target   *Node
	nodes    []*Node
	capacity int
	mu       sync.RWMutex
}

// newNodeSet creates a new node set for the given target.
func newNodeSet(target *Node, capacity int) *nodeSet {
	return &nodeSet{
		target:   target,
		nodes:    make([]*Node, 0, capacity),
		capacity: capacity,
	}
}

// add adds a node to the set if it's among the closest. Returns true if added.
func (ns *nodeSet) add(node *Node) bool {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Check for duplicates
	for _, existing := range ns.nodes {
		if existing.PublicKey == node.PublicKey {
			return false
		}
	}

	dist := node.Distance(ns.target)

	// Find insertion point (sorted by distance)
	insertIdx := len(ns.nodes)
	for i, existing := range ns.nodes {
		existingDist := existing.Distance(ns.target)
		if lessDistance(dist, existingDist) {
			insertIdx = i
			break
		}
	}

	// Check if we should add this node
	if insertIdx >= ns.capacity {
		return false
	}

	// Insert at position
	ns.nodes = append(ns.nodes, nil)
	copy(ns.nodes[insertIdx+1:], ns.nodes[insertIdx:])
	ns.nodes[insertIdx] = node

	// Trim to capacity
	if len(ns.nodes) > ns.capacity {
		ns.nodes = ns.nodes[:ns.capacity]
	}

	return true
}

// selectUnqueried returns up to n unqueried nodes closest to target.
func (ns *nodeSet) selectUnqueried(n int, queried map[[32]byte]struct{}) []*Node {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	result := make([]*Node, 0, n)
	for _, node := range ns.nodes {
		if _, isQueried := queried[node.PublicKey]; !isQueried {
			result = append(result, node)
			if len(result) >= n {
				break
			}
		}
	}
	return result
}

// closestDistance returns the distance of the closest node, or max if empty.
func (ns *nodeSet) closestDistance() [32]byte {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if len(ns.nodes) == 0 {
		var max [32]byte
		for i := range max {
			max[i] = 0xFF
		}
		return max
	}
	return ns.nodes[0].Distance(ns.target)
}

// getClosest returns up to k closest nodes.
func (ns *nodeSet) getClosest(k int) []*Node {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if len(ns.nodes) <= k {
		result := make([]*Node, len(ns.nodes))
		copy(result, ns.nodes)
		return result
	}

	result := make([]*Node, k)
	copy(result, ns.nodes[:k])
	return result
}

// Errors for iterative lookup.
var (
	ErrNoNodesAvailable = &lookupError{"no nodes available in routing table"}
	ErrQueryTimeout     = &lookupError{"query timeout"}
)

// lookupError represents an error that occurred during DHT lookup operations.
type lookupError struct {
	msg string
}

// Error implements the error interface for lookupError.
func (e *lookupError) Error() string {
	return e.msg
}
