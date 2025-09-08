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
	parser       *transport.ParserSelector // Multi-network packet parser

	// Versioned handshake support for protocol negotiation
	handshakeManager *transport.VersionedHandshakeManager
	enableVersioned  bool // Flag to enable/disable versioned handshakes

	// Protocol version tracking for peers
	peerVersions map[string]transport.ProtocolVersion // Maps address string to negotiated version
	versionMu    sync.RWMutex                         // Protects peerVersions map

	// Address type detection for multi-network support
	addressDetector *AddressTypeDetector // Detects and validates address types across different networks
	addressStats    *AddressTypeStats    // Statistics for address type distribution
} // NewBootstrapManager creates a new bootstrap manager.
//
//export ToxDHTBootstrapManagerNew
func NewBootstrapManager(selfID crypto.ToxID, transportArg transport.Transport, routingTable *RoutingTable) *BootstrapManager {
	bm := &BootstrapManager{
		nodes:           make([]*BootstrapNode, 0),
		selfID:          selfID,
		transport:       transportArg,
		routingTable:    routingTable,
		bootstrapped:    false,
		minNodes:        4,                                          // Minimum nodes needed to consider bootstrapping successful
		maxAttempts:     5,                                          // Maximum number of bootstrap attempts
		backoff:         time.Second,                                // Initial backoff duration
		maxBackoff:      2 * time.Minute,                            // Maximum backoff duration
		enableVersioned: true,                                       // Enable versioned handshakes by default
		peerVersions:    make(map[string]transport.ProtocolVersion), // Initialize version tracking
		addressDetector: NewAddressTypeDetector(),                   // Initialize address type detection
		addressStats:    &AddressTypeStats{},                        // Initialize address statistics
	}
	// Initialize parser after struct creation to avoid naming conflict
	bm.parser = transport.NewParserSelector()

	// For now, disable versioned handshakes until we have access to the private key
	// This will be updated when the constructor is enhanced to accept a keyPair
	bm.enableVersioned = false
	bm.handshakeManager = nil

	return bm
}

// NewBootstrapManagerWithKeyPair creates a new bootstrap manager with versioned handshake support.
// This extended constructor accepts a keyPair to enable cryptographic handshakes.
//
//export ToxDHTBootstrapManagerNewWithKeyPair
func NewBootstrapManagerWithKeyPair(selfID crypto.ToxID, keyPair *crypto.KeyPair, transportArg transport.Transport, routingTable *RoutingTable) *BootstrapManager {
	bm := &BootstrapManager{
		nodes:           make([]*BootstrapNode, 0),
		selfID:          selfID,
		transport:       transportArg,
		routingTable:    routingTable,
		bootstrapped:    false,
		minNodes:        4,                                          // Minimum nodes needed to consider bootstrapping successful
		maxAttempts:     5,                                          // Maximum number of bootstrap attempts
		backoff:         time.Second,                                // Initial backoff duration
		maxBackoff:      2 * time.Minute,                            // Maximum backoff duration
		enableVersioned: true,                                       // Enable versioned handshakes by default
		peerVersions:    make(map[string]transport.ProtocolVersion), // Initialize version tracking
		addressDetector: NewAddressTypeDetector(),                   // Initialize address type detection
		addressStats:    &AddressTypeStats{},                        // Initialize address statistics
	}
	// Initialize parser after struct creation to avoid naming conflict
	bm.parser = transport.NewParserSelector()

	// Initialize versioned handshake manager with keyPair for enhanced security
	if keyPair != nil {
		supportedVersions := []transport.ProtocolVersion{
			transport.ProtocolLegacy,  // Always support legacy for backward compatibility
			transport.ProtocolNoiseIK, // Support Noise-IK for enhanced security
		}
		bm.handshakeManager = transport.NewVersionedHandshakeManager(
			keyPair.Private,
			supportedVersions,
			transport.ProtocolNoiseIK, // Prefer Noise-IK when available
		)
	} else {
		bm.enableVersioned = false
		bm.handshakeManager = nil
	}

	return bm
}

// NewBootstrapManagerForTesting creates a new bootstrap manager with configurable minimum nodes.
// This constructor is specifically designed for testing environments where fewer nodes are acceptable.
//
//export ToxDHTBootstrapManagerNewForTesting
func NewBootstrapManagerForTesting(selfID crypto.ToxID, transportArg transport.Transport, routingTable *RoutingTable, minNodes int) *BootstrapManager {
	if minNodes < 1 {
		minNodes = 1 // Ensure at least 1 node is required
	}

	bm := &BootstrapManager{
		nodes:           make([]*BootstrapNode, 0),
		selfID:          selfID,
		transport:       transportArg,
		routingTable:    routingTable,
		bootstrapped:    false,
		minNodes:        minNodes,                                   // Configurable minimum nodes for testing
		maxAttempts:     5,                                          // Maximum number of bootstrap attempts
		backoff:         time.Second,                                // Initial backoff duration
		maxBackoff:      2 * time.Minute,                            // Maximum backoff duration
		enableVersioned: false,                                      // Disable versioned handshakes for testing simplicity
		peerVersions:    make(map[string]transport.ProtocolVersion), // Initialize version tracking
		addressDetector: NewAddressTypeDetector(),                   // Initialize address type detection
		addressStats:    &AddressTypeStats{},                        // Initialize address statistics
	}
	// Initialize parser after struct creation to avoid naming conflict
	bm.parser = transport.NewParserSelector()

	// Disable versioned handshakes for testing simplicity
	bm.enableVersioned = false
	bm.handshakeManager = nil

	return bm
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

	// Attempt versioned handshake if supported and enabled
	if bm.enableVersioned && bm.handshakeManager != nil {
		if err := bm.attemptVersionedHandshake(bn, dhtNode); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "connectToBootstrapNode",
				"address":  bn.Address.String(),
				"error":    err.Error(),
			}).Debug("Versioned handshake failed, falling back to legacy bootstrap")
		} else {
			logrus.WithFields(logrus.Fields{
				"function": "connectToBootstrapNode",
				"address":  bn.Address.String(),
			}).Info("Versioned handshake successful")

			bm.updateNodeLastUsed(bn)
			resultChan <- &BootstrapResult{Node: dhtNode}
			return
		}
	}

	// Fall back to traditional bootstrap method
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

// attemptVersionedHandshake performs a versioned handshake with a bootstrap node.
// This enables protocol negotiation and secure communication setup.
func (bm *BootstrapManager) attemptVersionedHandshake(bn *BootstrapNode, dhtNode *Node) error {
	if bm.handshakeManager == nil {
		return errors.New("handshake manager not initialized")
	}

	logrus.WithFields(logrus.Fields{
		"function": "attemptVersionedHandshake",
		"address":  bn.Address.String(),
	}).Debug("Initiating versioned handshake")

	// Initiate the versioned handshake
	response, err := bm.handshakeManager.InitiateHandshake(bn.PublicKey, bm.transport, bn.Address)
	if err != nil {
		return fmt.Errorf("handshake initiation failed: %w", err)
	}

	// Log the negotiated protocol version
	logrus.WithFields(logrus.Fields{
		"function":       "attemptVersionedHandshake",
		"address":        bn.Address.String(),
		"agreed_version": response.AgreedVersion,
	}).Info("Protocol version negotiated")

	// Additional setup based on negotiated protocol version
	switch response.AgreedVersion {
	case transport.ProtocolNoiseIK:
		logrus.WithFields(logrus.Fields{
			"function": "attemptVersionedHandshake",
			"address":  bn.Address.String(),
		}).Debug("Setting up Noise-IK secure channel")
		// In a complete implementation, this would establish the secure channel
		// For now, we just log the successful negotiation

	case transport.ProtocolLegacy:
		logrus.WithFields(logrus.Fields{
			"function": "attemptVersionedHandshake",
			"address":  bn.Address.String(),
		}).Debug("Using legacy protocol")
		// Fall back to legacy handshake process

	default:
		return fmt.Errorf("unsupported agreed protocol version: %v", response.AgreedVersion)
	}

	return nil
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

// SetVersionedHandshakeEnabled enables or disables versioned handshake support.
// This allows runtime control over protocol negotiation behavior.
//
//export ToxDHTBootstrapManagerSetVersionedHandshakeEnabled
func (bm *BootstrapManager) SetVersionedHandshakeEnabled(enabled bool) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.enableVersioned = enabled && bm.handshakeManager != nil

	logrus.WithFields(logrus.Fields{
		"function": "SetVersionedHandshakeEnabled",
		"enabled":  bm.enableVersioned,
	}).Info("Versioned handshake support updated")
}

// IsVersionedHandshakeEnabled returns true if versioned handshakes are enabled.
//
//export ToxDHTBootstrapManagerIsVersionedHandshakeEnabled
func (bm *BootstrapManager) IsVersionedHandshakeEnabled() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return bm.enableVersioned
}

// GetSupportedProtocolVersions returns the list of supported protocol versions.
// Returns nil if versioned handshakes are not available.
//
//export ToxDHTBootstrapManagerGetSupportedProtocolVersions
func (bm *BootstrapManager) GetSupportedProtocolVersions() []transport.ProtocolVersion {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	if bm.handshakeManager == nil {
		return nil
	}

	// Create a copy to avoid exposing internal state
	versions := make([]transport.ProtocolVersion, len(bm.handshakeManager.GetSupportedVersions()))
	copy(versions, bm.handshakeManager.GetSupportedVersions())
	return versions
}

// SetPeerProtocolVersion records the negotiated protocol version for a peer.
// This is used to track which protocol version to use for future communications.
func (bm *BootstrapManager) SetPeerProtocolVersion(peerAddr net.Addr, version transport.ProtocolVersion) {
	bm.versionMu.Lock()
	defer bm.versionMu.Unlock()

	bm.peerVersions[peerAddr.String()] = version

	logrus.WithFields(logrus.Fields{
		"function": "SetPeerProtocolVersion",
		"peer":     peerAddr.String(),
		"version":  version,
	}).Debug("Set peer protocol version")
}

// GetPeerProtocolVersion retrieves the negotiated protocol version for a peer.
// Returns ProtocolLegacy if no version has been negotiated.
func (bm *BootstrapManager) GetPeerProtocolVersion(peerAddr net.Addr) transport.ProtocolVersion {
	bm.versionMu.RLock()
	defer bm.versionMu.RUnlock()

	if version, exists := bm.peerVersions[peerAddr.String()]; exists {
		return version
	}

	// Default to legacy for unknown peers
	return transport.ProtocolLegacy
}

// ClearPeerProtocolVersion removes the stored protocol version for a peer.
// This is useful when a peer disconnects or becomes unreachable.
func (bm *BootstrapManager) ClearPeerProtocolVersion(peerAddr net.Addr) {
	bm.versionMu.Lock()
	defer bm.versionMu.Unlock()

	delete(bm.peerVersions, peerAddr.String())

	logrus.WithFields(logrus.Fields{
		"function": "ClearPeerProtocolVersion",
		"peer":     peerAddr.String(),
	}).Debug("Cleared peer protocol version")
}

// GetAddressTypeStats returns the current address type statistics.
// This provides insight into the distribution of network types in the DHT.
func (bm *BootstrapManager) GetAddressTypeStats() *AddressTypeStats {
	// Return a copy to prevent external modification
	stats := *bm.addressStats
	return &stats
}

// GetDominantNetworkType returns the most frequently encountered network type.
// This can help optimize network selection and routing decisions.
func (bm *BootstrapManager) GetDominantNetworkType() transport.AddressType {
	return bm.addressStats.GetDominantAddressType()
}

// ResetAddressTypeStats clears the address type statistics.
// This can be useful for monitoring changes over time periods.
func (bm *BootstrapManager) ResetAddressTypeStats() {
	bm.addressStats = &AddressTypeStats{}

	logrus.WithFields(logrus.Fields{
		"function": "ResetAddressTypeStats",
	}).Debug("Reset address type statistics")
}

// GetSupportedAddressTypes returns the list of address types supported by this DHT instance.
// This can be used for capability advertisement and compatibility checking.
func (bm *BootstrapManager) GetSupportedAddressTypes() []transport.AddressType {
	return bm.addressDetector.GetSupportedAddressTypes()
}

// ValidateNodeAddress checks if a node address is valid and supported.
// This provides a public interface for address validation.
func (bm *BootstrapManager) ValidateNodeAddress(addr net.Addr) error {
	if addr == nil {
		return fmt.Errorf("address is nil")
	}

	addrType, err := bm.addressDetector.DetectAddressType(addr)
	if err != nil {
		return fmt.Errorf("address type detection failed: %w", err)
	}

	if !bm.addressDetector.ValidateAddressType(addrType) {
		return fmt.Errorf("unsupported address type: %s", addrType.String())
	}

	if !bm.addressDetector.IsRoutableAddress(addrType) {
		return fmt.Errorf("address type is not routable: %s", addrType.String())
	}

	return nil
}
