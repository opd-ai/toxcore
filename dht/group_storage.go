package dht

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// GroupAnnouncement represents a group chat announcement stored in the DHT.
type GroupAnnouncement struct {
	GroupID   uint32
	Name      string
	Type      uint8 // ChatType
	Privacy   uint8 // Privacy level
	Timestamp time.Time
	TTL       time.Duration
}

// GroupStorage manages group announcements in the DHT.
type GroupStorage struct {
	announcements map[uint32]*GroupAnnouncement
	mu            sync.RWMutex
}

// NewGroupStorage creates a new group storage instance.
func NewGroupStorage() *GroupStorage {
	return &GroupStorage{
		announcements: make(map[uint32]*GroupAnnouncement),
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
func (rt *RoutingTable) AnnounceGroup(announcement *GroupAnnouncement, tr transport.Transport) error {
	if tr == nil {
		return fmt.Errorf("transport is nil")
	}

	data, err := SerializeAnnouncement(announcement)
	if err != nil {
		return fmt.Errorf("failed to serialize announcement: %w", err)
	}

	packet := &transport.Packet{
		PacketType: transport.PacketGroupAnnounce,
		Data:       data,
	}

	// Get nodes from routing table to announce to
	rt.mu.RLock()
	var nodes []*Node
	for _, bucket := range rt.kBuckets {
		nodes = append(nodes, bucket.GetNodes()...)
	}
	rt.mu.RUnlock()

	// Send announcement to known nodes
	for _, node := range nodes {
		if node.Status == StatusGood && node.Address != nil {
			_ = tr.Send(packet, node.Address) // Best effort
		}
	}

	return nil
}

// QueryGroup queries the DHT for group information.
func (rt *RoutingTable) QueryGroup(groupID uint32, tr transport.Transport) (*GroupAnnouncement, error) {
	if tr == nil {
		return nil, fmt.Errorf("transport is nil")
	}

	// Serialize query
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data[0:4], groupID)

	packet := &transport.Packet{
		PacketType: transport.PacketGroupQuery,
		Data:       data,
	}

	// Get nodes from routing table to query
	rt.mu.RLock()
	var nodes []*Node
	for _, bucket := range rt.kBuckets {
		nodes = append(nodes, bucket.GetNodes()...)
		if len(nodes) >= 8 { // Query up to 8 nodes
			break
		}
	}
	rt.mu.RUnlock()

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no DHT nodes available for query")
	}

	// Send query to nodes
	for _, node := range nodes {
		if node.Status == StatusGood && node.Address != nil {
			_ = tr.Send(packet, node.Address) // Best effort
		}
	}

	// Note: This is a simplified implementation. In a complete version, we would:
	// 1. Wait for responses with a timeout
	// 2. Collect responses from multiple nodes
	// 3. Verify consistency across responses
	// For now, we return nil to indicate async operation
	return nil, fmt.Errorf("DHT query sent, response handling not yet implemented")
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

	// Store in our local cache for future queries
	if bm.groupStorage != nil {
		bm.groupStorage.StoreAnnouncement(announcement)
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
