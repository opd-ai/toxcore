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
	"github.com/sirupsen/logrus"
)

// BootstrapError represents specific bootstrap failure types
type BootstrapError struct {
	Type  string
	Node  string
	Cause error
}

func (e *BootstrapError) Error() string {
	return fmt.Sprintf("bootstrap %s failed for %s: %v", e.Type, e.Node, e.Cause)
}

// BootstrapResult represents the result of a bootstrap attempt
type BootstrapResult struct {
	Node  *Node
	Error *BootstrapError
}

//
//export ToxDHTBootstrapNode
type BootstrapNode struct {
	Address   net.Addr
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
func (bm *BootstrapManager) AddNode(address net.Addr, publicKeyHex string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Convert hex public key to byte array
	var publicKey [32]byte
	if len(publicKeyHex) != 64 {
		logrus.WithFields(logrus.Fields{
			"function":          "AddNode",
			"address":           address.String(),
			"public_key_length": len(publicKeyHex),
			"error":             "invalid public key length",
		}).Error("Public key validation failed")
		return errors.New("invalid public key length")
	}

	// Safe to preview public key after validation
	logrus.WithFields(logrus.Fields{
		"function":   "AddNode",
		"address":    address.String(),
		"public_key": publicKeyHex[:16] + "...",
	}).Info("Adding bootstrap node")

	logrus.WithFields(logrus.Fields{
		"function": "AddNode",
		"address":  address.String(),
	}).Debug("Decoding public key")
	decoded, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "AddNode",
			"address":  address.String(),
			"error":    err.Error(),
		}).Error("Failed to decode public key")
		return fmt.Errorf("invalid hex public key: %w", err)
	}

	if len(decoded) != 32 {
		logrus.WithFields(logrus.Fields{
			"function":       "AddNode",
			"address":        address.String(),
			"decoded_length": len(decoded),
			"error":          "decoded public key has incorrect length",
		}).Error("Decoded public key validation failed")
		return errors.New("decoded public key has incorrect length")
	}

	copy(publicKey[:], decoded)

	// Check if node already exists
	for _, node := range bm.nodes {
		if node.Address.String() == address.String() {
			logrus.WithFields(logrus.Fields{
				"function": "AddNode",
				"address":  address.String(),
			}).Info("Updating existing bootstrap node")
			node.PublicKey = publicKey
			return nil
		}
	}

	// Add new node
	bm.nodes = append(bm.nodes, &BootstrapNode{
		Address:   address,
		PublicKey: publicKey,
		LastUsed:  time.Time{},
		Success:   false,
	})

	logrus.WithFields(logrus.Fields{
		"function":    "AddNode",
		"address":     address.String(),
		"total_nodes": len(bm.nodes),
	}).Info("Bootstrap node added successfully")

	return nil
}

// Bootstrap attempts to join the Tox network by connecting to bootstrap nodes.
//
//export ToxDHTBootstrap
func (bm *BootstrapManager) Bootstrap(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"function":    "Bootstrap",
		"nodes_count": len(bm.nodes),
	}).Info("Starting bootstrap process")

	if err := bm.validateBootstrapRequest(); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"error":    err.Error(),
		}).Error("Bootstrap validation failed")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function": "Bootstrap",
	}).Debug("Preparing bootstrap nodes")
	nodes := bm.prepareBootstrapNodes()
	resultChan := make(chan *BootstrapResult, len(nodes))

	logrus.WithFields(logrus.Fields{
		"function":       "Bootstrap",
		"prepared_nodes": len(nodes),
	}).Debug("Launching bootstrap workers")
	bm.launchBootstrapWorkers(nodes, resultChan)

	logrus.WithFields(logrus.Fields{
		"function": "Bootstrap",
	}).Debug("Processing bootstrap results")
	err := bm.processBootstrapResults(ctx, resultChan)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"error":    err.Error(),
		}).Error("Bootstrap process failed")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function": "Bootstrap",
	}).Info("Bootstrap process completed successfully")

	return nil
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
func (bm *BootstrapManager) launchBootstrapWorkers(nodes []*BootstrapNode, resultChan chan<- *BootstrapResult) {
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
func (bm *BootstrapManager) connectToBootstrapNode(wg *sync.WaitGroup, bn *BootstrapNode, resultChan chan<- *BootstrapResult) {
	defer wg.Done()

	dhtNode, err := bm.createDHTNodeFromBootstrap(bn)
	if err != nil {
		resultChan <- &BootstrapResult{
			Error: &BootstrapError{
				Type:  "node creation",
				Node:  bn.Address.String(),
				Cause: err,
			},
		}
		return
	}

	if err := bm.sendGetNodesRequest(bn, dhtNode.Address); err != nil {
		resultChan <- &BootstrapResult{
			Error: &BootstrapError{
				Type:  "connection",
				Node:  bn.Address.String(),
				Cause: err,
			},
		}
		return
	}

	bm.updateNodeLastUsed(bn)
	resultChan <- &BootstrapResult{Node: dhtNode}
}

// createDHTNodeFromBootstrap creates a DHT node from bootstrap node information.
func (bm *BootstrapManager) createDHTNodeFromBootstrap(bn *BootstrapNode) (*Node, error) {
	// Use the net.Addr directly - no need to resolve since it's already provided
	addr := bn.Address

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
		if n.Address.String() == bn.Address.String() {
			n.LastUsed = time.Now()
			break
		}
	}
}

// processBootstrapResults handles the results from bootstrap workers and determines success.
func (bm *BootstrapManager) processBootstrapResults(ctx context.Context, resultChan <-chan *BootstrapResult) error {
	successful := 0
	var lastError *BootstrapError

	for {
		select {
		case result, ok := <-resultChan:
			if !ok {
				return bm.handleBootstrapCompletion(successful, lastError)
			}
			if result.Error != nil {
				lastError = result.Error
			} else {
				successful = bm.processReceivedNode(result.Node, successful)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// processReceivedNode handles a single received node and updates the success counter.
func (bm *BootstrapManager) processReceivedNode(node *Node, successful int) int {
	if node != nil {
		if added := bm.routingTable.AddNode(node); added {
			successful++
		}
	}
	return successful
}

// handleBootstrapCompletion determines if bootstrap was successful and handles next steps.
func (bm *BootstrapManager) handleBootstrapCompletion(successful int, lastError *BootstrapError) error {
	if successful >= bm.minNodes {
		bm.mu.Lock()
		bm.bootstrapped = true
		bm.attempts = 0 // Reset attempts counter on success
		bm.mu.Unlock()
		return nil
	}

	// Not enough successful connections, provide specific error context
	if lastError != nil {
		return fmt.Errorf("bootstrap failed: %v (attempted %d nodes, need %d)", lastError, successful, bm.minNodes)
	}
	return fmt.Errorf("bootstrap failed: insufficient connections (%d/%d nodes connected)", successful, bm.minNodes)
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
