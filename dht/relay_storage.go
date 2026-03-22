package dht

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// RelayAnnouncement represents a relay server announcement stored in the DHT.
type RelayAnnouncement struct {
	PublicKey [32]byte
	Address   string
	Port      uint16
	Priority  int
	Timestamp time.Time
	TTL       time.Duration
	Capacity  uint32 // Number of clients the relay can handle
	Load      uint8  // Current load percentage (0-100)
}

// RelayQueryResponseCallback is called when a relay query response is received from the DHT network.
type RelayQueryResponseCallback func(announcement *RelayAnnouncement)

// RelayStorage manages relay server announcements in the DHT.
type RelayStorage struct {
	announcements map[[32]byte]*RelayAnnouncement
	mu            sync.RWMutex

	// Callback for notifying upper layers of query responses
	responseCallback RelayQueryResponseCallback
	callbackMu       sync.RWMutex

	// pendingQueries holds per-query response channels.
	pendingQueries map[uint64][]chan []*RelayAnnouncement
	pendingMu      sync.Mutex

	// queryCounter generates unique query IDs
	queryCounter uint64
}

// NewRelayStorage creates a new relay storage instance.
func NewRelayStorage() *RelayStorage {
	return &RelayStorage{
		announcements:  make(map[[32]byte]*RelayAnnouncement),
		pendingQueries: make(map[uint64][]chan []*RelayAnnouncement),
	}
}

// registerQuery registers a buffered response channel for a relay query.
// Returns a unique query ID and the response channel.
func (rs *RelayStorage) registerQuery() (uint64, chan []*RelayAnnouncement) {
	ch := make(chan []*RelayAnnouncement, 1)
	rs.pendingMu.Lock()
	rs.queryCounter++
	queryID := rs.queryCounter
	rs.pendingQueries[queryID] = append(rs.pendingQueries[queryID], ch)
	rs.pendingMu.Unlock()
	return queryID, ch
}

// deregisterQuery removes a previously registered response channel.
func (rs *RelayStorage) deregisterQuery(queryID uint64, ch chan []*RelayAnnouncement) {
	rs.pendingMu.Lock()
	defer rs.pendingMu.Unlock()
	channels := rs.pendingQueries[queryID]
	for i, c := range channels {
		if c == ch {
			rs.pendingQueries[queryID] = append(channels[:i], channels[i+1:]...)
			break
		}
	}
	if len(rs.pendingQueries[queryID]) == 0 {
		delete(rs.pendingQueries, queryID)
	}
}

// StoreAnnouncement stores a relay announcement with TTL.
func (rs *RelayStorage) StoreAnnouncement(announcement *RelayAnnouncement) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.announcements[announcement.PublicKey] = announcement
}

// GetAnnouncement retrieves a relay announcement if it exists and hasn't expired.
func (rs *RelayStorage) GetAnnouncement(publicKey [32]byte) (*RelayAnnouncement, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	announcement, exists := rs.announcements[publicKey]
	if !exists {
		return nil, false
	}

	// Check if announcement has expired
	if time.Since(announcement.Timestamp) > announcement.TTL {
		return nil, false
	}

	return announcement, true
}

// GetAllAnnouncements returns all non-expired relay announcements.
func (rs *RelayStorage) GetAllAnnouncements() []*RelayAnnouncement {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var result []*RelayAnnouncement
	now := time.Now()
	for _, announcement := range rs.announcements {
		if now.Sub(announcement.Timestamp) <= announcement.TTL {
			result = append(result, announcement)
		}
	}
	return result
}

// CleanExpired removes expired announcements.
func (rs *RelayStorage) CleanExpired() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for pubKey, announcement := range rs.announcements {
		if time.Since(announcement.Timestamp) > announcement.TTL {
			delete(rs.announcements, pubKey)
		}
	}
}

// SetResponseCallback registers a callback to be notified when relay query responses are received.
func (rs *RelayStorage) SetResponseCallback(callback RelayQueryResponseCallback) {
	rs.callbackMu.Lock()
	defer rs.callbackMu.Unlock()
	rs.responseCallback = callback
}

// notifyResponse calls the registered callback with the announcement.
func (rs *RelayStorage) notifyResponse(announcement *RelayAnnouncement) {
	rs.callbackMu.RLock()
	callback := rs.responseCallback
	rs.callbackMu.RUnlock()

	if callback != nil {
		callback(announcement)
	}
}

// notifyQueryResponse sends relay announcements to pending query channels.
func (rs *RelayStorage) notifyQueryResponse(queryID uint64, relays []*RelayAnnouncement) {
	rs.pendingMu.Lock()
	channels := rs.pendingQueries[queryID]
	for _, ch := range channels {
		select {
		case ch <- relays:
		default:
		}
	}
	rs.pendingMu.Unlock()
}

// SerializeRelayAnnouncement converts a relay announcement to bytes for network transmission.
func SerializeRelayAnnouncement(announcement *RelayAnnouncement) ([]byte, error) {
	addrBytes := []byte(announcement.Address)
	// Format: pubKey(32) + port(2) + priority(4) + timestamp(8) + capacity(4) + load(1) + addrLen(2) + addr(var)
	data := make([]byte, 32+2+4+8+4+1+2+len(addrBytes))

	copy(data[0:32], announcement.PublicKey[:])
	binary.BigEndian.PutUint16(data[32:34], announcement.Port)
	binary.BigEndian.PutUint32(data[34:38], uint32(announcement.Priority))
	binary.BigEndian.PutUint64(data[38:46], uint64(announcement.Timestamp.Unix()))
	binary.BigEndian.PutUint32(data[46:50], announcement.Capacity)
	data[50] = announcement.Load
	binary.BigEndian.PutUint16(data[51:53], uint16(len(addrBytes)))
	copy(data[53:], addrBytes)

	return data, nil
}

// DeserializeRelayAnnouncement converts bytes back to a relay announcement.
func DeserializeRelayAnnouncement(data []byte) (*RelayAnnouncement, error) {
	if len(data) < 53 {
		return nil, fmt.Errorf("relay announcement data too short: %d bytes", len(data))
	}

	var publicKey [32]byte
	copy(publicKey[:], data[0:32])
	port := binary.BigEndian.Uint16(data[32:34])
	priority := int(binary.BigEndian.Uint32(data[34:38]))
	timestamp := int64(binary.BigEndian.Uint64(data[38:46]))
	capacity := binary.BigEndian.Uint32(data[46:50])
	load := data[50]
	addrLen := binary.BigEndian.Uint16(data[51:53])

	if len(data) < int(53+addrLen) {
		return nil, fmt.Errorf("relay announcement data truncated, expected %d bytes", 53+addrLen)
	}

	address := string(data[53 : 53+addrLen])

	return &RelayAnnouncement{
		PublicKey: publicKey,
		Address:   address,
		Port:      port,
		Priority:  priority,
		Timestamp: time.Unix(timestamp, 0),
		TTL:       24 * time.Hour, // Default TTL
		Capacity:  capacity,
		Load:      load,
	}, nil
}

// AnnounceRelay broadcasts a relay server announcement to DHT nodes.
func (rt *RoutingTable) AnnounceRelay(announcement *RelayAnnouncement, tr transport.Transport) error {
	data, err := SerializeRelayAnnouncement(announcement)
	if err != nil {
		return fmt.Errorf("failed to serialize relay announcement: %w", err)
	}

	packet := &transport.Packet{
		PacketType: transport.PacketRelayAnnounce,
		Data:       data,
	}

	return rt.broadcastAnnouncement(packet, tr, "relay")
}

// QueryRelays queries the DHT for available relay servers.
// Returns a list of relay announcements found in local storage or from the network.
func (rt *RoutingTable) QueryRelays(tr transport.Transport) ([]*RelayAnnouncement, error) {
	return rt.QueryRelaysWithTimeout(tr, 5*time.Second)
}

// QueryRelaysWithTimeout queries the DHT for relay servers with a configurable timeout.
func (rt *RoutingTable) QueryRelaysWithTimeout(tr transport.Transport, timeout time.Duration) ([]*RelayAnnouncement, error) {
	if tr == nil {
		return nil, fmt.Errorf("transport is nil")
	}

	// First check local storage
	relays := rt.getLocalRelays()
	if len(relays) > 0 {
		return relays, nil
	}

	// Query the network
	return rt.queryRelayNetwork(tr, timeout)
}

// getLocalRelays retrieves all non-expired relays from local storage.
func (rt *RoutingTable) getLocalRelays() []*RelayAnnouncement {
	if rt.relayStorage == nil {
		return nil
	}
	return rt.relayStorage.GetAllAnnouncements()
}

// queryRelayNetwork sends relay queries to DHT nodes and waits for responses.
func (rt *RoutingTable) queryRelayNetwork(tr transport.Transport, timeout time.Duration) ([]*RelayAnnouncement, error) {
	nodes := rt.selectNodesToQuery()
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no DHT nodes available for relay query")
	}

	queryID, ch, err := rt.registerRelayQuery()
	if err != nil {
		return nil, err
	}
	defer rt.relayStorage.deregisterQuery(queryID, ch)

	rt.sendRelayQueriesToNodes(tr, nodes)

	return rt.waitForRelayResponses(ch, timeout)
}

// registerRelayQuery sets up a pending query channel for relay responses.
func (rt *RoutingTable) registerRelayQuery() (uint64, chan []*RelayAnnouncement, error) {
	if rt.relayStorage == nil {
		return 0, nil, fmt.Errorf("relay storage not initialized")
	}
	queryID, ch := rt.relayStorage.registerQuery()
	return queryID, ch, nil
}

// sendRelayQueriesToNodes sends relay query packets to good DHT nodes.
func (rt *RoutingTable) sendRelayQueriesToNodes(tr transport.Transport, nodes []*Node) {
	packet := &transport.Packet{
		PacketType: transport.PacketRelayQuery,
		Data:       []byte{},
	}
	for _, node := range nodes {
		if node.Status == StatusGood && node.Address != nil {
			_ = tr.Send(packet, node.Address) // Best effort
		}
	}
}

// waitForRelayResponses waits for relay responses or returns local relays on timeout.
func (rt *RoutingTable) waitForRelayResponses(ch chan []*RelayAnnouncement, timeout time.Duration) ([]*RelayAnnouncement, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case relays := <-ch:
		return relays, nil
	case <-timer.C:
		// Return any relays collected so far from local storage
		return rt.getLocalRelays(), nil
	}
}

// HandleRelayQueryResponse processes a relay query response received from the DHT network.
func (rt *RoutingTable) HandleRelayQueryResponse(announcements []*RelayAnnouncement) {
	if rt.relayStorage == nil {
		return
	}
	for _, announcement := range announcements {
		if announcement != nil {
			rt.relayStorage.StoreAnnouncement(announcement)
			rt.relayStorage.notifyResponse(announcement)
		}
	}
}

// GetRelayStorage returns the relay storage for external access.
func (rt *RoutingTable) GetRelayStorage() *RelayStorage {
	return rt.relayStorage
}

// ToTransportServerInfo converts a RelayAnnouncement to transport.RelayServerInfo.
func (ra *RelayAnnouncement) ToTransportServerInfo() transport.RelayServerInfo {
	return transport.RelayServerInfo{
		Address:   ra.Address,
		PublicKey: ra.PublicKey,
		Port:      ra.Port,
		Priority:  ra.Priority,
	}
}

// HandleRelayPacket processes relay-related DHT packets.
func (bm *BootstrapManager) HandleRelayPacket(packet *transport.Packet, senderAddr net.Addr) error {
	switch packet.PacketType {
	case transport.PacketRelayAnnounce:
		return bm.handleRelayAnnounce(packet, senderAddr)
	case transport.PacketRelayQuery:
		return bm.handleRelayQuery(packet, senderAddr)
	case transport.PacketRelayQueryResponse:
		return bm.handleRelayQueryResponse(packet, senderAddr)
	default:
		return fmt.Errorf("unsupported relay packet type: %d", packet.PacketType)
	}
}

// handleRelayAnnounce processes a relay announcement from another node.
func (bm *BootstrapManager) handleRelayAnnounce(packet *transport.Packet, senderAddr net.Addr) error {
	if bm.routingTable == nil || bm.routingTable.relayStorage == nil {
		return fmt.Errorf("relay storage not initialized")
	}

	announcement, err := DeserializeRelayAnnouncement(packet.Data)
	if err != nil {
		return fmt.Errorf("failed to deserialize relay announcement: %w", err)
	}

	bm.routingTable.relayStorage.StoreAnnouncement(announcement)
	return nil
}

// handleRelayQuery processes a relay query request.
func (bm *BootstrapManager) handleRelayQuery(packet *transport.Packet, senderAddr net.Addr) error {
	if bm.routingTable == nil || bm.routingTable.relayStorage == nil || bm.transport == nil {
		return fmt.Errorf("relay storage or transport not initialized")
	}

	// Get all known relay announcements
	relays := bm.routingTable.relayStorage.GetAllAnnouncements()

	// Build response
	var responseData []byte
	responseData = append(responseData, byte(len(relays)))

	for _, relay := range relays {
		data, err := SerializeRelayAnnouncement(relay)
		if err != nil {
			continue
		}
		// Add length prefix for each announcement
		lenBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(lenBuf, uint16(len(data)))
		responseData = append(responseData, lenBuf...)
		responseData = append(responseData, data...)
	}

	response := &transport.Packet{
		PacketType: transport.PacketRelayQueryResponse,
		Data:       responseData,
	}

	return bm.transport.Send(response, senderAddr)
}

// handleRelayQueryResponse processes a relay query response.
func (bm *BootstrapManager) handleRelayQueryResponse(packet *transport.Packet, senderAddr net.Addr) error {
	if len(packet.Data) < 1 {
		return fmt.Errorf("relay query response too short")
	}

	count := int(packet.Data[0])
	if count == 0 {
		return nil
	}

	announcements := parseRelayAnnouncements(packet.Data[1:], count)

	if bm.routingTable != nil {
		bm.routingTable.HandleRelayQueryResponse(announcements)
	}

	return nil
}

// parseRelayAnnouncements deserializes relay announcements from packet data.
func parseRelayAnnouncements(data []byte, count int) []*RelayAnnouncement {
	var announcements []*RelayAnnouncement
	offset := 0

	for i := 0; i < count && offset < len(data); i++ {
		if offset+2 > len(data) {
			break
		}
		announcementLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2

		if offset+announcementLen > len(data) {
			break
		}

		announcement, err := DeserializeRelayAnnouncement(data[offset : offset+announcementLen])
		if err == nil {
			announcements = append(announcements, announcement)
		}
		offset += announcementLen
	}
	return announcements
}
