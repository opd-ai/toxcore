// Package dht implements the Distributed Hash Table for the Tox protocol.
package dht

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// BootstrapNode represents a known node used for entering the Tox network.
//
//export ToxDHTBootstrapNode
type BootstrapNode struct {
	Address   string
	Port      uint16
	PublicKey [32]byte
	LastUsed  time.Time
	Success   bool
}

// BootstrapManager handles the process of connecting to the Tox network.
//
//export ToxDHTBootstrapManager
type BootstrapManager struct {
	nodes        []*BootstrapNode
	selfID       crypto.ToxID
	transport    transport.Transport
	routingTable *RoutingTable
	bootstrapped bool
	minNodes     int
	mu           sync.RWMutex
	attempts     int
	maxAttempts  int
	backoff      time.Duration
	maxBackoff   time.Duration
}

// NewBootstrapManager creates a new bootstrap manager.
//
//export ToxDHTBootstrapManagerNew
func NewBootstrapManager(selfID crypto.ToxID, transport transport.Transport, routingTable *RoutingTable) *BootstrapManager {
	return &BootstrapManager{
		nodes:        make([]*BootstrapNode, 0),
		selfID:       selfID,
		transport:    transport,
		routingTable: routingTable,
		bootstrapped: false,
		minNodes:     4,               // Minimum nodes needed to consider bootstrapping successful
		maxAttempts:  5,               // Maximum number of bootstrap attempts
		backoff:      time.Second,     // Initial backoff duration
		maxBackoff:   2 * time.Minute, // Maximum backoff duration
	}
}

// AddNode adds a bootstrap node to the manager.
//
//export ToxDHTBootstrapManagerAddNode
func (bm *BootstrapManager) AddNode(address string, port uint16, publicKeyHex string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Convert hex public key to byte array
	var publicKey [32]byte
	if len(publicKeyHex) != 64 {
		return errors.New("invalid public key length")
	}

	decoded, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("invalid hex public key: %w", err)
	}

	if len(decoded) != 32 {
		return errors.New("decoded public key has incorrect length")
	}

	copy(publicKey[:], decoded)

	// Check if node already exists
	for _, node := range bm.nodes {
		if node.Address == address && node.Port == port {
			// Update existing node
			node.PublicKey = publicKey
			return nil
		}
	}

	// Add new node
	bm.nodes = append(bm.nodes, &BootstrapNode{
		Address:   address,
		Port:      port,
		PublicKey: publicKey,
		LastUsed:  time.Time{},
		Success:   false,
	})

	return nil
}

// Bootstrap attempts to join the Tox network by connecting to bootstrap nodes.
//
//export ToxDHTBootstrap
func (bm *BootstrapManager) Bootstrap(ctx context.Context) error {
	if err := bm.validateBootstrapRequest(); err != nil {
		return err
	}

	nodes := bm.prepareBootstrapNodes()
	resultChan := make(chan *Node, len(nodes))

	bm.launchBootstrapWorkers(nodes, resultChan)

	return bm.processBootstrapResults(ctx, resultChan)
}

// validateBootstrapRequest checks if bootstrap conditions are met and updates attempt counter.
func (bm *BootstrapManager) validateBootstrapRequest() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if len(bm.nodes) == 0 {
		return errors.New("no bootstrap nodes available")
	}

	bm.attempts++
	if bm.attempts > bm.maxAttempts {
		return errors.New("maximum bootstrap attempts reached")
	}

	return nil
}

// prepareBootstrapNodes creates a safe copy of bootstrap nodes for concurrent processing.
func (bm *BootstrapManager) prepareBootstrapNodes() []*BootstrapNode {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	nodes := make([]*BootstrapNode, len(bm.nodes))
	copy(nodes, bm.nodes)
	return nodes
}

// launchBootstrapWorkers starts goroutines to connect to each bootstrap node.
func (bm *BootstrapManager) launchBootstrapWorkers(nodes []*BootstrapNode, resultChan chan<- *Node) {
	var wg sync.WaitGroup

	for _, node := range nodes {
		wg.Add(1)
		go bm.connectToBootstrapNode(&wg, node, resultChan)
	}

	// Close channel when all workers complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()
}

// connectToBootstrapNode handles the connection process for a single bootstrap node.
func (bm *BootstrapManager) connectToBootstrapNode(wg *sync.WaitGroup, bn *BootstrapNode, resultChan chan<- *Node) {
	defer wg.Done()

	dhtNode, err := bm.createDHTNodeFromBootstrap(bn)
	if err != nil {
		return
	}

	if err := bm.sendGetNodesRequest(bn, dhtNode.Address); err != nil {
		return
	}

	bm.updateNodeLastUsed(bn)
	resultChan <- dhtNode
}

// createDHTNodeFromBootstrap creates a DHT node from bootstrap node information.
func (bm *BootstrapManager) createDHTNodeFromBootstrap(bn *BootstrapNode) (*Node, error) {
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(bn.Address, fmt.Sprintf("%d", bn.Port)))
	if err != nil {
		return nil, err
	}

	var nospam [4]byte // Zeros for bootstrap nodes
	nodeID := crypto.NewToxID(bn.PublicKey, nospam)
	return NewNode(*nodeID, addr), nil
}

// sendGetNodesRequest sends a get nodes packet to the specified address.
func (bm *BootstrapManager) sendGetNodesRequest(bn *BootstrapNode, addr net.Addr) error {
	packet := &transport.Packet{
		PacketType: transport.PacketGetNodes,
		Data:       bm.createGetNodesPacket(bn.PublicKey),
	}

	return bm.transport.Send(packet, addr)
}

// updateNodeLastUsed updates the last used timestamp for the specified bootstrap node.
func (bm *BootstrapManager) updateNodeLastUsed(bn *BootstrapNode) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for _, n := range bm.nodes {
		if n.Address == bn.Address && n.Port == bn.Port {
			n.LastUsed = time.Now()
			break
		}
	}
}

// processBootstrapResults handles the results from bootstrap workers and determines success.
func (bm *BootstrapManager) processBootstrapResults(ctx context.Context, resultChan <-chan *Node) error {
	successful := 0

	for {
		select {
		case node, ok := <-resultChan:
			if !ok {
				return bm.handleBootstrapCompletion(successful)
			}

			if node != nil {
				if added := bm.routingTable.AddNode(node); added {
					successful++
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// handleBootstrapCompletion determines if bootstrap was successful and handles next steps.
func (bm *BootstrapManager) handleBootstrapCompletion(successful int) error {
	if successful >= bm.minNodes {
		bm.mu.Lock()
		bm.bootstrapped = true
		bm.attempts = 0 // Reset attempts counter on success
		bm.mu.Unlock()
		return nil
	}

	// Not enough successful connections, schedule retry
	return bm.scheduleRetry(context.Background())
}

// createGetNodesPacket creates a packet for requesting nodes from a bootstrap node.
// Format: [sender_pk(32 bytes)][target_pk(32 bytes)]
func (bm *BootstrapManager) createGetNodesPacket(targetPK [32]byte) []byte {
	// Create a 64-byte packet
	packet := make([]byte, 64)

	// First 32 bytes: our public key (so the recipient knows who is asking)
	copy(packet[:32], bm.selfID.PublicKey[:])

	// Next 32 bytes: the target public key we're searching for
	// For initial bootstrap, we can use the target node's key or our own key
	copy(packet[32:64], targetPK[:])

	return packet
}

// scheduleRetry schedules a retry with exponential backoff.
func (bm *BootstrapManager) scheduleRetry(ctx context.Context) error {
	bm.mu.Lock()
	backoff := bm.backoff
	// Exponential backoff with jitter
	jitter := time.Duration(float64(backoff) * (0.5 + rand.Float64())) // 50-150% of backoff
	bm.backoff = time.Duration(float64(bm.backoff) * 1.5)
	if bm.backoff > bm.maxBackoff {
		bm.backoff = bm.maxBackoff
	}
	bm.mu.Unlock()

	// Schedule retry
	select {
	case <-time.After(jitter):
		return errors.New("bootstrap failed, retry scheduled")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsBootstrapped returns true if the node is successfully bootstrapped.
//
//export ToxDHTIsBootstrapped
func (bm *BootstrapManager) IsBootstrapped() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.bootstrapped
}

// GetNodes returns the list of bootstrap nodes.
//
//export ToxDHTBootstrapManagerGetNodes
func (bm *BootstrapManager) GetNodes() []*BootstrapNode {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	nodes := make([]*BootstrapNode, len(bm.nodes))
	copy(nodes, bm.nodes)
	return nodes
}

// ClearNodes removes all bootstrap nodes.
//
//export ToxDHTBootstrapManagerClearNodes
func (bm *BootstrapManager) ClearNodes() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.nodes = make([]*BootstrapNode, 0)
}
