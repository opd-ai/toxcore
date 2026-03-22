package dht

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// ErrGroupDHTNotImplemented is returned by QueryGroup when the group was not
// found in local DHT storage and a network query was dispatched. Response
// collection from remote DHT nodes is not yet implemented; callers should treat
// this error as "query initiated, response not yet available".
var ErrGroupDHTNotImplemented = errors.New("group DHT response collection not yet implemented")

// GroupAnnouncement represents a group chat announcement stored in the DHT.
type GroupAnnouncement struct {
	GroupID   uint32
	Name      string
	Type      uint8 // ChatType
	Privacy   uint8 // Privacy level
	Timestamp time.Time
	TTL       time.Duration
}

// GroupQueryResponseCallback is called when a group query response is received from the DHT network.
type GroupQueryResponseCallback func(announcement *GroupAnnouncement)

// GroupStorage manages group announcements in the DHT.
type GroupStorage struct {
	announcements map[uint32]*GroupAnnouncement
	mu            sync.RWMutex

	// Callback for notifying upper layers of query responses
	responseCallback GroupQueryResponseCallback
	callbackMu       sync.RWMutex

	// pendingQueries holds per-query response channels keyed by groupID.
	pendingQueries map[uint32][]chan *GroupAnnouncement
	pendingMu      sync.Mutex
}

// NewGroupStorage creates a new group storage instance.
func NewGroupStorage() *GroupStorage {
	return &GroupStorage{
		announcements:  make(map[uint32]*GroupAnnouncement),
		pendingQueries: make(map[uint32][]chan *GroupAnnouncement),
	}
}

// registerQuery registers a buffered response channel for the given groupID.
// The caller must call deregisterQuery when done to avoid leaking channels.
func (gs *GroupStorage) registerQuery(groupID uint32) chan *GroupAnnouncement {
	ch := make(chan *GroupAnnouncement, 1)
	gs.pendingMu.Lock()
	gs.pendingQueries[groupID] = append(gs.pendingQueries[groupID], ch)
	gs.pendingMu.Unlock()
	return ch
}

// deregisterQuery removes a previously registered response channel.
func (gs *GroupStorage) deregisterQuery(groupID uint32, ch chan *GroupAnnouncement) {
	gs.pendingMu.Lock()
	defer gs.pendingMu.Unlock()
	channels := gs.pendingQueries[groupID]
	for i, c := range channels {
		if c == ch {
			gs.pendingQueries[groupID] = append(channels[:i], channels[i+1:]...)
			break
		}
	}
	if len(gs.pendingQueries[groupID]) == 0 {
		delete(gs.pendingQueries, groupID)
	}
}

// StoreAnnouncement stores a group announcement with TTL.
func (gs *GroupStorage) StoreAnnouncement(announcement *GroupAnnouncement) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.announcements[announcement.GroupID] = announcement
}

// GetAnnouncement retrieves a group announcement if it exists and hasn't expired.
func (gs *GroupStorage) GetAnnouncement(groupID uint32) (*GroupAnnouncement, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	announcement, exists := gs.announcements[groupID]
	if !exists {
		return nil, false
	}

	// Check if announcement has expired
	if time.Since(announcement.Timestamp) > announcement.TTL {
		return nil, false
	}

	return announcement, true
}

// CleanExpired removes expired announcements.
func (gs *GroupStorage) CleanExpired() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	for groupID, announcement := range gs.announcements {
		if time.Since(announcement.Timestamp) > announcement.TTL {
			delete(gs.announcements, groupID)
		}
	}
}

// SetResponseCallback registers a callback to be notified when group query responses are received.
func (gs *GroupStorage) SetResponseCallback(callback GroupQueryResponseCallback) {
	gs.callbackMu.Lock()
	defer gs.callbackMu.Unlock()
	gs.responseCallback = callback
}

// notifyResponse calls the registered callback with the announcement, if one is set.
// It also sends the announcement to any pending query channels registered for this group.
func (gs *GroupStorage) notifyResponse(announcement *GroupAnnouncement) {
	gs.callbackMu.RLock()
	callback := gs.responseCallback
	gs.callbackMu.RUnlock()

	if callback != nil {
		callback(announcement)
	}

	gs.pendingMu.Lock()
	channels := gs.pendingQueries[announcement.GroupID]
	for _, ch := range channels {
		select {
		case ch <- announcement:
		default:
		}
	}
	gs.pendingMu.Unlock()
}

// SerializeAnnouncement converts a group announcement to bytes for network transmission.
func SerializeAnnouncement(announcement *GroupAnnouncement) ([]byte, error) {
	data := make([]byte, 4+4+1+1+8) // groupID(4) + nameLen(4) + type(1) + privacy(1) + timestamp(8)

	binary.BigEndian.PutUint32(data[0:4], announcement.GroupID)
	binary.BigEndian.PutUint32(data[4:8], uint32(len(announcement.Name)))
	data[8] = announcement.Type
	data[9] = announcement.Privacy
	binary.BigEndian.PutUint64(data[10:18], uint64(announcement.Timestamp.Unix()))

	// Append name
	data = append(data, []byte(announcement.Name)...)

	return data, nil
}

// DeserializeAnnouncement converts bytes back to a group announcement.
func DeserializeAnnouncement(data []byte) (*GroupAnnouncement, error) {
	if len(data) < 18 {
		return nil, fmt.Errorf("announcement data too short: %d bytes", len(data))
	}

	groupID := binary.BigEndian.Uint32(data[0:4])
	nameLen := binary.BigEndian.Uint32(data[4:8])
	chatType := data[8]
	privacy := data[9]
	timestamp := int64(binary.BigEndian.Uint64(data[10:18]))

	if len(data) < int(18+nameLen) {
		return nil, fmt.Errorf("announcement data truncated, expected %d bytes", 18+nameLen)
	}

	name := string(data[18 : 18+nameLen])

	return &GroupAnnouncement{
		GroupID:   groupID,
		Name:      name,
		Type:      chatType,
		Privacy:   privacy,
		Timestamp: time.Unix(timestamp, 0),
		TTL:       24 * time.Hour, // Default TTL
	}, nil
}

// AnnounceGroup broadcasts a group announcement to DHT nodes.
// Announcements are sent to all good nodes; the function returns an error only
// if no node accepted the packet. Individual send failures are retried once
// before being counted as failures.
func (rt *RoutingTable) AnnounceGroup(announcement *GroupAnnouncement, tr transport.Transport) error {
	data, err := SerializeAnnouncement(announcement)
	if err != nil {
		return fmt.Errorf("failed to serialize announcement: %w", err)
	}

	packet := &transport.Packet{
		PacketType: transport.PacketGroupAnnounce,
		Data:       data,
	}

	return rt.broadcastAnnouncement(packet, tr, "group")
}

// QueryGroup queries the DHT for group information.
// First checks local storage, then queries the network if not found locally.
// Returns (announcement, nil) if found in local storage or a network response arrives within the timeout.
// Returns (nil, error) if no DHT nodes are available or the query times out.
func (rt *RoutingTable) QueryGroup(groupID uint32, tr transport.Transport) (*GroupAnnouncement, error) {
	return rt.QueryGroupWithTimeout(groupID, tr, 5*time.Second)
}

// QueryGroupWithTimeout queries the DHT for group information with a configurable timeout.
// First checks local storage, then queries the network if not found locally.
// Returns (announcement, nil) if found in local storage or a network response arrives within the timeout.
// Returns (nil, error) if no DHT nodes are available or the query times out.
func (rt *RoutingTable) QueryGroupWithTimeout(groupID uint32, tr transport.Transport, timeout time.Duration) (*GroupAnnouncement, error) {
	if tr == nil {
		return nil, fmt.Errorf("transport is nil")
	}

	announcement, found := rt.checkLocalStorage(groupID)
	if found {
		return announcement, nil
	}

	return rt.queryNetwork(groupID, tr, timeout)
}

// checkLocalStorage checks if the group announcement exists in local storage.
func (rt *RoutingTable) checkLocalStorage(groupID uint32) (*GroupAnnouncement, bool) {
	if rt.groupStorage != nil {
		if announcement, exists := rt.groupStorage.GetAnnouncement(groupID); exists {
			return announcement, true
		}
	}
	return nil, false
}

// queryNetwork sends a group query to DHT nodes and waits for a response within the specified timeout.
// Returns the first GroupAnnouncement received, or an error if no response arrives.
func (rt *RoutingTable) queryNetwork(groupID uint32, tr transport.Transport, timeout time.Duration) (*GroupAnnouncement, error) {
	packet := rt.buildQueryPacket(groupID)
	nodes := rt.selectNodesToQuery()

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no DHT nodes available for query")
	}

	// Register a pending query channel before dispatching to avoid a response race.
	var ch chan *GroupAnnouncement
	if rt.groupStorage != nil {
		ch = rt.groupStorage.registerQuery(groupID)
		defer rt.groupStorage.deregisterQuery(groupID, ch)
	}

	rt.sendQueryToNodes(packet, nodes, tr)

	if ch == nil {
		return nil, ErrGroupDHTNotImplemented
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case announcement := <-ch:
		return announcement, nil
	case <-timer.C:
		return nil, fmt.Errorf("group query timed out: no response received for group %d", groupID)
	}
}

// buildQueryPacket creates a query packet for the specified group ID.
func (rt *RoutingTable) buildQueryPacket(groupID uint32) *transport.Packet {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data[0:4], groupID)

	return &transport.Packet{
		PacketType: transport.PacketGroupQuery,
		Data:       data,
	}
}

// selectNodesToQuery retrieves up to 8 good nodes from the routing table.
func (rt *RoutingTable) selectNodesToQuery() []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var nodes []*Node
	for _, bucket := range rt.kBuckets {
		nodes = append(nodes, bucket.GetNodes()...)
		if len(nodes) >= 8 {
			break
		}
	}
	return nodes
}

// sendQueryToNodes sends the query packet to all good nodes.
func (rt *RoutingTable) sendQueryToNodes(packet *transport.Packet, nodes []*Node, tr transport.Transport) {
	for _, node := range nodes {
		if node.Status == StatusGood && node.Address != nil {
			_ = tr.Send(packet, node.Address) // Best effort
		}
	}
}

// HandleGroupPacket processes group-related DHT packets.
func (bm *BootstrapManager) HandleGroupPacket(packet *transport.Packet, senderAddr net.Addr) error {
	switch packet.PacketType {
	case transport.PacketGroupAnnounce:
		return bm.handleGroupAnnounce(packet, senderAddr)
	case transport.PacketGroupQuery:
		return bm.handleGroupQuery(packet, senderAddr)
	case transport.PacketGroupQueryResponse:
		return bm.handleGroupQueryResponse(packet, senderAddr)
	default:
		return fmt.Errorf("unsupported group packet type: %d", packet.PacketType)
	}
}

// handleGroupAnnounce processes a group announcement from another node.
func (bm *BootstrapManager) handleGroupAnnounce(packet *transport.Packet, senderAddr net.Addr) error {
	if bm.groupStorage == nil {
		return fmt.Errorf("group storage not initialized")
	}

	announcement, err := DeserializeAnnouncement(packet.Data)
	if err != nil {
		return fmt.Errorf("failed to deserialize announcement: %w", err)
	}

	bm.groupStorage.StoreAnnouncement(announcement)
	return nil
}

// handleGroupQuery processes a group query request.
func (bm *BootstrapManager) handleGroupQuery(packet *transport.Packet, senderAddr net.Addr) error {
	if bm.groupStorage == nil || bm.transport == nil {
		return fmt.Errorf("group storage or transport not initialized")
	}

	if len(packet.Data) < 4 {
		return fmt.Errorf("group query packet too short")
	}

	groupID := binary.BigEndian.Uint32(packet.Data[0:4])

	// Look up group in storage
	announcement, exists := bm.groupStorage.GetAnnouncement(groupID)
	if !exists {
		// Send empty response to indicate group not found
		response := &transport.Packet{
			PacketType: transport.PacketGroupQueryResponse,
			Data:       []byte{0}, // 0 = not found
		}
		return bm.transport.Send(response, senderAddr)
	}

	// Serialize announcement and send response
	data, err := SerializeAnnouncement(announcement)
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	response := &transport.Packet{
		PacketType: transport.PacketGroupQueryResponse,
		Data:       append([]byte{1}, data...), // 1 = found
	}

	return bm.transport.Send(response, senderAddr)
}

// handleGroupQueryResponse processes a group query response.
func (bm *BootstrapManager) handleGroupQueryResponse(packet *transport.Packet, senderAddr net.Addr) error {
	if len(packet.Data) < 1 {
		return fmt.Errorf("group query response too short")
	}

	found := packet.Data[0]
	if found == 0 {
		// Group not found on this node
		return nil
	}

	// Deserialize announcement
	announcement, err := DeserializeAnnouncement(packet.Data[1:])
	if err != nil {
		return fmt.Errorf("failed to deserialize response: %w", err)
	}

	// Forward to routing table which will store and notify callbacks
	if bm.routingTable != nil {
		bm.routingTable.HandleGroupQueryResponse(announcement)
	}

	return nil
}

// GroupQueryResponse represents a response to a group query (for future use).
type GroupQueryResponse struct {
	Found        bool
	Announcement *GroupAnnouncement
}

// MarshalJSON implements json.Marshaler for GroupAnnouncement.
func (ga *GroupAnnouncement) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		GroupID   uint32 `json:"group_id"`
		Name      string `json:"name"`
		Type      uint8  `json:"type"`
		Privacy   uint8  `json:"privacy"`
		Timestamp string `json:"timestamp"`
	}{
		GroupID:   ga.GroupID,
		Name:      ga.Name,
		Type:      ga.Type,
		Privacy:   ga.Privacy,
		Timestamp: ga.Timestamp.Format(time.RFC3339),
	})
}
