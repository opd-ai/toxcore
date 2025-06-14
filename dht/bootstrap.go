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
	bm.mu.Lock()
	if len(bm.nodes) == 0 {
		bm.mu.Unlock()
		return errors.New("no bootstrap nodes available")
	}
	bm.attempts++
	attemptNumber := bm.attempts
	bm.mu.Unlock()

	if attemptNumber > bm.maxAttempts {
		return errors.New("maximum bootstrap attempts reached")
	}

	// Try each bootstrap node
	var wg sync.WaitGroup
	resultChan := make(chan *Node, len(bm.nodes))

	bm.mu.RLock()
	nodes := make([]*BootstrapNode, len(bm.nodes))
	copy(nodes, bm.nodes)
	bm.mu.RUnlock()

	for _, node := range nodes {
		wg.Add(1)
		go func(bn *BootstrapNode) {
			defer wg.Done()

			// Resolve address
			addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(bn.Address, fmt.Sprintf("%d", bn.Port)))
			if err != nil {
				return
			}

			// Create Tox ID for bootstrap node
			var nospam [4]byte // Zeros for bootstrap nodes
			nodeID := crypto.NewToxID(bn.PublicKey, nospam)

			// Create node object
			dhtNode := NewNode(*nodeID, addr)

			// Send get nodes request packet
			packet := &transport.Packet{
				PacketType: transport.PacketGetNodes,
				Data:       bm.createGetNodesPacket(bn.PublicKey),
			}

			// Send packet
			err = bm.transport.Send(packet, addr)
			if err != nil {
				return
			}

			// Update last used timestamp
			bm.mu.Lock()
			for _, n := range bm.nodes {
				if n.Address == bn.Address && n.Port == bn.Port {
					n.LastUsed = time.Now()
					break
				}
			}
			bm.mu.Unlock()

			// Add to result channel
			resultChan <- dhtNode
		}(node)
	}

	// Wait for all goroutines to finish or context to cancel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	successful := 0
	for {
		select {
		case node, ok := <-resultChan:
			if !ok {
				// Channel closed, all nodes processed
				if successful >= bm.minNodes {
					bm.mu.Lock()
					bm.bootstrapped = true
					bm.attempts = 0 // Reset attempts counter on success
					bm.mu.Unlock()
					return nil
				}

				// Not enough successful connections
				return bm.scheduleRetry(ctx)
			}

			if node != nil {
				// Add node to routing table
				added := bm.routingTable.AddNode(node)
				if added {
					successful++
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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

// Stop shuts down the bootstrap manager and cleans up resources.
//
//export ToxDHTBootstrapManagerStop
func (bm *BootstrapManager) Stop() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.bootstrapped = false
	bm.nodes = make([]*BootstrapNode, 0)
	bm.attempts = 0
	bm.backoff = time.Second
}
