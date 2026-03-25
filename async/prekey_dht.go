// Package async provides DHT-based pre-key storage for forward-secure messaging.
// This file implements pre-key bundle publication and retrieval via the DHT network.
package async

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/transport"
)

const (
	// DefaultPreKeyReplicationFactor is the number of DHT nodes to store pre-keys on.
	DefaultPreKeyReplicationFactor = 3

	// PreKeyDHTTTL is the time-to-live for pre-key bundles in the DHT.
	PreKeyDHTTTL = 7 * 24 * time.Hour

	// PreKeyDHTRefreshInterval is how often to re-publish pre-keys to DHT.
	PreKeyDHTRefreshInterval = 24 * time.Hour

	// PreKeyDHTQueryTimeout is the timeout for querying pre-keys from DHT.
	PreKeyDHTQueryTimeout = 10 * time.Second
)

// PreKeyDHTBundle represents a pre-key bundle stored in the DHT.
type PreKeyDHTBundle struct {
	OwnerPK   [32]byte            `json:"owner_pk"`
	PreKeys   []PreKeyForExchange `json:"pre_keys"`
	Signature [64]byte            `json:"signature"`
	Timestamp time.Time           `json:"timestamp"`
	ExpiresAt time.Time           `json:"expires_at"`
	Version   uint32              `json:"version"`
}

// PreKeyDHTManager handles DHT-based pre-key storage and retrieval.
type PreKeyDHTManager struct {
	mu sync.RWMutex

	// keyPair is our identity key for signing pre-key bundles.
	keyPair *crypto.KeyPair

	// fsManager is the forward security manager to source pre-keys from.
	fsManager *ForwardSecurityManager

	// routingTable for DHT operations.
	routingTable *dht.RoutingTable

	// transport for sending packets.
	transport transport.Transport

	// replicationFactor is the number of DHT nodes to store on.
	replicationFactor int

	// localCache holds retrieved pre-key bundles.
	localCache map[[32]byte]*PreKeyDHTBundle

	// publishedAt tracks when we last published our pre-keys.
	publishedAt time.Time

	// version is incremented on each publish.
	version uint32

	// stopRefresh signals the refresh goroutine to stop.
	stopRefresh chan struct{}

	// refreshWg tracks the refresh goroutine.
	refreshWg sync.WaitGroup
}

// NewPreKeyDHTManager creates a new DHT-based pre-key manager.
func NewPreKeyDHTManager(
	keyPair *crypto.KeyPair,
	fsManager *ForwardSecurityManager,
	rt *dht.RoutingTable,
	tr transport.Transport,
) *PreKeyDHTManager {
	return &PreKeyDHTManager{
		keyPair:           keyPair,
		fsManager:         fsManager,
		routingTable:      rt,
		transport:         tr,
		replicationFactor: DefaultPreKeyReplicationFactor,
		localCache:        make(map[[32]byte]*PreKeyDHTBundle),
		stopRefresh:       make(chan struct{}),
	}
}

// SetReplicationFactor sets the number of DHT nodes to replicate to.
func (pm *PreKeyDHTManager) SetReplicationFactor(k int) {
	if k < 1 {
		k = 1
	}
	if k > 10 {
		k = 10
	}
	pm.mu.Lock()
	pm.replicationFactor = k
	pm.mu.Unlock()
}

// PublishPreKeys publishes our pre-key bundle to the DHT.
// Pre-keys are stored at k=3 nearest nodes to our public key.
func (pm *PreKeyDHTManager) PublishPreKeys() error {
	if pm.routingTable == nil {
		return fmt.Errorf("publish failed: routing table not set")
	}
	if pm.transport == nil {
		return fmt.Errorf("publish failed: transport not set")
	}
	if pm.fsManager == nil {
		return fmt.Errorf("publish failed: forward security manager not set")
	}

	bundle, err := pm.createSignedBundle()
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	data, err := pm.serializeBundle(bundle)
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	packet := &transport.Packet{
		PacketType: transport.PacketAsyncPreKeyExchange,
		Data:       data,
	}

	targetID := pm.publicKeyToToxID(pm.keyPair.Public)
	nearestNodes := pm.routingTable.FindClosestNodes(targetID, pm.replicationFactor)

	if len(nearestNodes) == 0 {
		return fmt.Errorf("publish failed: no DHT nodes available")
	}

	successCount := pm.sendToNodes(packet, nearestNodes)
	if successCount == 0 {
		return fmt.Errorf("publish failed: could not reach any of %d DHT nodes", len(nearestNodes))
	}

	pm.mu.Lock()
	pm.publishedAt = time.Now()
	pm.version = bundle.Version
	pm.mu.Unlock()

	return nil
}

// createSignedBundle creates a signed pre-key bundle for DHT publication.
func (pm *PreKeyDHTManager) createSignedBundle() (*PreKeyDHTBundle, error) {
	exchange, err := pm.fsManager.ExchangePreKeys(pm.keyPair.Public)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-keys: %w", err)
	}

	pm.mu.Lock()
	pm.version++
	version := pm.version
	pm.mu.Unlock()

	now := time.Now()
	bundle := &PreKeyDHTBundle{
		OwnerPK:   pm.keyPair.Public,
		PreKeys:   exchange.PreKeys,
		Timestamp: now,
		ExpiresAt: now.Add(PreKeyDHTTTL),
		Version:   version,
	}

	dataToSign := pm.bundleDataForSigning(bundle)
	signature, err := crypto.Sign(dataToSign, pm.keyPair.Private)
	if err != nil {
		return nil, fmt.Errorf("failed to sign bundle: %w", err)
	}
	copy(bundle.Signature[:], signature[:])

	return bundle, nil
}

// bundleDataForSigning returns the data to be signed for a bundle.
func (pm *PreKeyDHTManager) bundleDataForSigning(bundle *PreKeyDHTBundle) []byte {
	data := make([]byte, 32+8+8+4)
	copy(data[0:32], bundle.OwnerPK[:])
	binary.BigEndian.PutUint64(data[32:40], uint64(bundle.Timestamp.Unix()))
	binary.BigEndian.PutUint64(data[40:48], uint64(bundle.ExpiresAt.Unix()))
	binary.BigEndian.PutUint32(data[48:52], bundle.Version)
	return data
}

// serializeBundle serializes a pre-key bundle for network transmission.
func (pm *PreKeyDHTManager) serializeBundle(bundle *PreKeyDHTBundle) ([]byte, error) {
	return json.Marshal(bundle)
}

// deserializeBundle deserializes a pre-key bundle from network data.
func (pm *PreKeyDHTManager) deserializeBundle(data []byte) (*PreKeyDHTBundle, error) {
	var bundle PreKeyDHTBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bundle: %w", err)
	}
	return &bundle, nil
}

// sendToNodes sends packet to nodes with retry logic.
func (pm *PreKeyDHTManager) sendToNodes(packet *transport.Packet, nodes []*dht.Node) int {
	successCount := 0
	for _, node := range nodes {
		if node.Status != dht.StatusGood || node.Address == nil {
			continue
		}
		if pm.sendWithRetry(packet, node) {
			successCount++
		}
	}
	return successCount
}

// sendWithRetry attempts to send a packet with one retry.
func (pm *PreKeyDHTManager) sendWithRetry(packet *transport.Packet, node *dht.Node) bool {
	if err := pm.transport.Send(packet, node.Address); err != nil {
		return pm.transport.Send(packet, node.Address) == nil
	}
	return true
}

// publicKeyToToxID converts a public key to a ToxID for DHT lookups.
func (pm *PreKeyDHTManager) publicKeyToToxID(pk [32]byte) crypto.ToxID {
	return crypto.ToxID{PublicKey: pk}
}

// RetrievePreKeys retrieves pre-keys for a peer from the DHT.
// Returns the pre-key bundle if found, or an error if not available.
func (pm *PreKeyDHTManager) RetrievePreKeys(peerPK [32]byte) (*PreKeyDHTBundle, error) {
	pm.mu.RLock()
	if cached, exists := pm.localCache[peerPK]; exists {
		if time.Now().Before(cached.ExpiresAt) {
			pm.mu.RUnlock()
			return cached, nil
		}
	}
	pm.mu.RUnlock()

	if pm.routingTable == nil {
		return nil, fmt.Errorf("retrieve failed: routing table not set")
	}
	if pm.transport == nil {
		return nil, fmt.Errorf("retrieve failed: transport not set")
	}

	return pm.queryDHT(peerPK)
}

// queryDHT queries the DHT for pre-keys of a specific peer.
func (pm *PreKeyDHTManager) queryDHT(peerPK [32]byte) (*PreKeyDHTBundle, error) {
	targetID := pm.publicKeyToToxID(peerPK)
	nearestNodes := pm.routingTable.FindClosestNodes(targetID, pm.replicationFactor)

	if len(nearestNodes) == 0 {
		return nil, fmt.Errorf("query failed: no DHT nodes available")
	}

	queryPacket := pm.buildQueryPacket(peerPK)

	for _, node := range nearestNodes {
		if node.Status != dht.StatusGood || node.Address == nil {
			continue
		}
		_ = pm.transport.Send(queryPacket, node.Address)
	}

	return nil, fmt.Errorf("query initiated: response pending")
}

// buildQueryPacket creates a pre-key query packet.
func (pm *PreKeyDHTManager) buildQueryPacket(peerPK [32]byte) *transport.Packet {
	data := make([]byte, 32)
	copy(data, peerPK[:])
	return &transport.Packet{
		PacketType: transport.PacketAsyncPreKeyExchange,
		Data:       data,
	}
}

// HandlePreKeyPacket processes a received pre-key packet.
func (pm *PreKeyDHTManager) HandlePreKeyPacket(packet *transport.Packet) error {
	if len(packet.Data) <= 32 {
		return nil
	}

	bundle, err := pm.deserializeBundle(packet.Data)
	if err != nil {
		return fmt.Errorf("failed to deserialize pre-key bundle: %w", err)
	}

	if err := pm.validateBundle(bundle); err != nil {
		return fmt.Errorf("invalid pre-key bundle: %w", err)
	}

	pm.mu.Lock()
	pm.localCache[bundle.OwnerPK] = bundle
	pm.mu.Unlock()

	if pm.fsManager != nil {
		exchange := &PreKeyExchangeMessage{
			Type:      "pre_key_exchange",
			SenderPK:  bundle.OwnerPK,
			PreKeys:   bundle.PreKeys,
			Timestamp: bundle.Timestamp,
		}
		_ = pm.fsManager.ProcessPreKeyExchange(exchange)
	}

	return nil
}

// validateBundle verifies the signature and expiration of a bundle.
func (pm *PreKeyDHTManager) validateBundle(bundle *PreKeyDHTBundle) error {
	if time.Now().After(bundle.ExpiresAt) {
		return fmt.Errorf("bundle expired")
	}

	dataToVerify := pm.bundleDataForSigning(bundle)
	valid, err := crypto.Verify(dataToVerify, bundle.Signature, bundle.OwnerPK)
	if err != nil {
		return fmt.Errorf("signature verification error: %w", err)
	}
	if !valid {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// StartAutoRefresh starts a background goroutine to periodically refresh pre-keys.
func (pm *PreKeyDHTManager) StartAutoRefresh() {
	pm.refreshWg.Add(1)
	go func() {
		defer pm.refreshWg.Done()

		ticker := time.NewTicker(PreKeyDHTRefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = pm.PublishPreKeys()
			case <-pm.stopRefresh:
				return
			}
		}
	}()
}

// StopAutoRefresh stops the background refresh goroutine.
func (pm *PreKeyDHTManager) StopAutoRefresh() {
	select {
	case <-pm.stopRefresh:
	default:
		close(pm.stopRefresh)
	}
	pm.refreshWg.Wait()
}

// GetCachedBundle returns a cached pre-key bundle if available.
func (pm *PreKeyDHTManager) GetCachedBundle(peerPK [32]byte) (*PreKeyDHTBundle, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	bundle, exists := pm.localCache[peerPK]
	if !exists {
		return nil, false
	}

	if time.Now().After(bundle.ExpiresAt) {
		return nil, false
	}

	return bundle, true
}

// ClearCache removes all cached pre-key bundles.
func (pm *PreKeyDHTManager) ClearCache() {
	pm.mu.Lock()
	pm.localCache = make(map[[32]byte]*PreKeyDHTBundle)
	pm.mu.Unlock()
}

// GetPublishedVersion returns the version of the last published bundle.
func (pm *PreKeyDHTManager) GetPublishedVersion() uint32 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.version
}

// NeedsRefresh checks if pre-keys should be re-published.
func (pm *PreKeyDHTManager) NeedsRefresh() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.publishedAt.IsZero() {
		return true
	}

	return time.Since(pm.publishedAt) >= PreKeyDHTRefreshInterval
}
