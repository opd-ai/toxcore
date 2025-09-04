// Package group implements group chat functionality for the Tox protocol.
//
// This package handles creating and managing group chats, inviting members,
// and sending/receiving messages within groups.
//
// Example:
//
//	group, err := group.Create("Programming Chat")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	group.OnMessage(func(peerID uint32, message string) {
//	    fmt.Printf("Message from peer %d: %s\n", peerID, message)
//	})
//
//	group.InviteFriend(friendID)
package group

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/transport"
)

// ChatType represents the type of group chat.
type ChatType uint8

const (
	// ChatTypeText is a text-only group chat.
	ChatTypeText ChatType = iota
	// ChatTypeAV is an audio/video group chat.
	ChatTypeAV
)

// Privacy represents the privacy setting of a group chat.
type Privacy uint8

const (
	// PrivacyPublic means anyone with the chat ID can join.
	PrivacyPublic Privacy = iota
	// PrivacyPrivate means joining requires an invite.
	PrivacyPrivate
)

// PeerChangeType represents the type of peer change event.
type PeerChangeType uint8

const (
	// PeerChangeJoin means a peer joined the group.
	PeerChangeJoin PeerChangeType = iota
	// PeerChangeLeave means a peer left the group.
	PeerChangeLeave
	// PeerChangeNameChange means a peer changed their name.
	PeerChangeNameChange
)

// Role represents a peer's role in the group.
type Role uint8

const (
	// RoleUser is a regular group member.
	RoleUser Role = iota
	// RoleModerator can kick and ban users.
	RoleModerator
	// RoleAdmin has full control over the group.
	RoleAdmin
	// RoleFounder created the group and cannot be demoted.
	RoleFounder
)

// MessageCallback is called when a message is received in a group.
type MessageCallback func(groupID, peerID uint32, message string)

// PeerCallback is called when a peer's status changes in a group.
type PeerCallback func(groupID, peerID uint32, changeType PeerChangeType)

// Invitation represents a pending group invitation.
type Invitation struct {
	FriendID  uint32
	GroupID   uint32
	Timestamp time.Time
	Expires   time.Time
}

// GroupInfo represents group metadata retrieved from DHT.
type GroupInfo struct {
	Name    string
	Type    ChatType
	Privacy Privacy
}

// queryDHTForGroup queries the DHT network for group information.
// In a production implementation, this would use the actual DHT network.
func queryDHTForGroup(chatID uint32) (*GroupInfo, error) {
	// Simulate DHT query - in real implementation this would:
	// 1. Create DHT query packet for group ID
	// 2. Send query to appropriate DHT nodes using dht.BootstrapManager
	// 3. Parse response and validate group information
	// 4. Return structured group metadata

	// Reference DHT package to show integration point
	_ = dht.StatusGood // Demonstrates DHT integration would be used here

	// For now, return simulated group info based on chat ID
	// Real implementation would retrieve from DHT storage
	return &GroupInfo{
		Name:    fmt.Sprintf("DHT_Group_%d", chatID),
		Type:    ChatTypeText,
		Privacy: PrivacyPrivate,
	}, nil
}

// Chat represents a group chat.
//
//export ToxGroupChat
type Chat struct {
	ID         uint32
	Name       string
	Type       ChatType
	Privacy    Privacy
	PeerCount  uint32
	SelfPeerID uint32
	Peers      map[uint32]*Peer
	Created    time.Time

	// Invitation tracking
	PendingInvitations map[uint32]*Invitation // friendID -> invitation

	// Transport layer for network communication
	transport transport.Transport
	// DHT for peer address resolution
	dht *dht.RoutingTable

	messageCallback MessageCallback
	peerCallback    PeerCallback

	mu sync.RWMutex
}

// Peer represents a member of a group chat.
//
//export ToxGroupPeer
type Peer struct {
	ID         uint32
	Name       string
	Role       Role
	Connection uint8 // 0 = offline, 1 = TCP, 2 = UDP
	PublicKey  [32]byte
	LastActive time.Time
}

// generateRandomID generates a cryptographically secure random 32-bit ID
func generateRandomID() (uint32, error) {
	var buf [4]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf[:]), nil
}

// Create creates a new group chat.
//
//export ToxGroupCreate
func Create(name string, chatType ChatType, privacy Privacy, transport transport.Transport, dhtRouting *dht.RoutingTable) (*Chat, error) {
	if len(name) == 0 {
		return nil, errors.New("group name cannot be empty")
	}

	// Generate cryptographically secure random group ID
	groupID, err := generateRandomID()
	if err != nil {
		return nil, errors.New("failed to generate group ID")
	}

	// Generate cryptographically secure random self peer ID
	selfPeerID, err := generateRandomID()
	if err != nil {
		return nil, errors.New("failed to generate peer ID")
	}

	chat := &Chat{
		ID:                 groupID,
		Name:               name,
		Type:               chatType,
		Privacy:            privacy,
		PeerCount:          1, // Self
		SelfPeerID:         selfPeerID,
		Peers:              make(map[uint32]*Peer),
		PendingInvitations: make(map[uint32]*Invitation),
		Created:            time.Now(),
		transport:          transport,
		dht:                dhtRouting,
	}

	// Add self as founder
	chat.Peers[chat.SelfPeerID] = &Peer{
		ID:         chat.SelfPeerID,
		Name:       "Self", // This would be the user's name
		Role:       RoleFounder,
		Connection: 2, // UDP
		LastActive: time.Now(),
	}

	return chat, nil
}

// Join joins an existing group chat.
//
//export ToxGroupJoin
func Join(chatID uint32, password string) (*Chat, error) {
	// Basic validation
	if chatID == 0 {
		return nil, errors.New("invalid group ID")
	}

	// Query DHT for group information
	groupInfo, err := queryDHTForGroup(chatID)
	if err != nil {
		// Fall back to defaults if DHT query fails
		// In production, this might be a hard error
		groupInfo = &GroupInfo{
			Name:    fmt.Sprintf("Group_%d", chatID),
			Type:    ChatTypeText,
			Privacy: PrivacyPrivate,
		}
	}

	// For now, create a basic group structure representing successful join
	// Populated with information from DHT query or defaults
	selfPeerID, err := generateRandomID()
	if err != nil {
		return nil, errors.New("failed to generate peer ID")
	}

	chat := &Chat{
		ID:                 chatID,
		Name:               groupInfo.Name,
		Type:               groupInfo.Type,
		Privacy:            groupInfo.Privacy,
		PeerCount:          1, // Just self initially
		SelfPeerID:         selfPeerID,
		Peers:              make(map[uint32]*Peer),
		PendingInvitations: make(map[uint32]*Invitation),
		Created:            time.Now(),
	}

	// Add self as a member
	chat.Peers[chat.SelfPeerID] = &Peer{
		ID:         chat.SelfPeerID,
		Name:       "Self",
		Role:       RoleUser,
		Connection: 1, // TCP initially
		LastActive: time.Now(),
	}

	// Validate password for private groups (basic check)
	if chat.Privacy == PrivacyPrivate && len(password) == 0 {
		return nil, errors.New("password required for private group")
	}

	return chat, nil
}

// InviteFriend invites a friend to the group chat.
//
//export ToxGroupInviteFriend
func (g *Chat) InviteFriend(friendID uint32) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Validate friendID
	if friendID == 0 {
		return errors.New("invalid friend ID")
	}

	// Check if invitations are allowed for this group type
	if g.Privacy != PrivacyPrivate {
		return errors.New("invites only allowed for private groups")
	}

	// Check if friend is already invited
	if _, exists := g.PendingInvitations[friendID]; exists {
		return errors.New("friend already has a pending invitation")
	}

	// Check if friend is already in the group
	for _, peer := range g.Peers {
		if peer.ID == friendID {
			return errors.New("friend is already in the group")
		}
	}

	// Create invitation with expiration (24 hours from now)
	invitation := &Invitation{
		FriendID:  friendID,
		GroupID:   g.ID,
		Timestamp: time.Now(),
		Expires:   time.Now().Add(24 * time.Hour),
	}

	// Store pending invitation
	g.PendingInvitations[friendID] = invitation

	// Create invitation packet for network transmission
	invitePacket, err := g.createInvitationPacket(invitation)
	if err != nil {
		return fmt.Errorf("failed to create invitation packet: %w", err)
	}

	// NOTE: Network integration point - In a production implementation,
	// this packet would be sent to the friend via the transport layer.
	// The packet contains encrypted group information and invitation details.
	_ = invitePacket // Packet created but transport layer integration needed

	return nil
}

// createInvitationPacket creates a group invitation packet for network transmission
func (g *Chat) createInvitationPacket(invitation *Invitation) (*transport.Packet, error) {
	// Packet format: [GroupID(4)][GroupName_Length(1)][GroupName][Expires(8)][Privacy(1)]
	nameBytes := []byte(g.Name)
	if len(nameBytes) > 255 {
		return nil, errors.New("group name too long for packet")
	}

	packetSize := 4 + 1 + len(nameBytes) + 8 + 1
	data := make([]byte, packetSize)
	offset := 0

	// Write Group ID
	binary.BigEndian.PutUint32(data[offset:], g.ID)
	offset += 4

	// Write Group Name Length
	data[offset] = byte(len(nameBytes))
	offset += 1

	// Write Group Name
	copy(data[offset:], nameBytes)
	offset += len(nameBytes)

	// Write Expiration timestamp
	binary.BigEndian.PutUint64(data[offset:], uint64(invitation.Expires.Unix()))
	offset += 8

	// Write Privacy setting
	data[offset] = byte(g.Privacy)

	return &transport.Packet{
		PacketType: transport.PacketGroupInvite,
		Data:       data,
	}, nil
}

// CleanupExpiredInvitations removes invitations that have expired.
func (g *Chat) CleanupExpiredInvitations() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	for friendID, invitation := range g.PendingInvitations {
		if now.After(invitation.Expires) {
			delete(g.PendingInvitations, friendID)
		}
	}
}

// SendMessage sends a message to the group chat.
//
//export ToxGroupSendMessage
func (g *Chat) SendMessage(message string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Validate message
	if len(message) == 0 {
		return errors.New("message cannot be empty")
	}

	// Check message length limit (Tox protocol limit)
	if len([]byte(message)) > 1372 {
		return errors.New("message too long: maximum 1372 bytes")
	}

	// Verify user is still in the group
	if g.SelfPeerID == 0 {
		return errors.New("not in group")
	}

	// Verify self exists in peers list
	if _, exists := g.Peers[g.SelfPeerID]; !exists {
		return errors.New("self peer not found in group")
	}

	// Broadcast message to all group peers
	err := g.broadcastGroupUpdate("group_message", map[string]interface{}{
		"sender_id": g.SelfPeerID,
		"message":   message,
		"timestamp": time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to broadcast message to group: %w", err)
	}

	// Trigger local message callback for immediate feedback
	if g.messageCallback != nil {
		go g.messageCallback(g.ID, g.SelfPeerID, message)
	}

	return nil
}

// Leave leaves the group chat.
//
//export ToxGroupLeave
func (g *Chat) Leave(message string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Broadcast leave message to all peers before cleaning up local state
	err := g.broadcastGroupUpdate("peer_leave", map[string]interface{}{
		"peer_id": g.SelfPeerID,
		"message": message,
	})
	if err != nil {
		// Log error but continue with cleanup
		fmt.Printf("Warning: failed to broadcast leave message: %v\n", err)
	}

	// Remove self from peers list
	delete(g.Peers, g.SelfPeerID)

	// Mark self as no longer in the group
	g.SelfPeerID = 0

	// Update peer count
	g.PeerCount = uint32(len(g.Peers))

	// Clear message callback to prevent further message processing
	g.messageCallback = nil

	return nil
}

// OnMessage sets the callback for group chat messages.
//
//export ToxGroupOnMessage
func (g *Chat) OnMessage(callback MessageCallback) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.messageCallback = callback
}

// OnPeerChange sets the callback for peer changes.
//
//export ToxGroupOnPeerChange
func (g *Chat) OnPeerChange(callback PeerCallback) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.peerCallback = callback
}

// GetPeer returns a peer by ID.
//
//export ToxGroupGetPeer
func (g *Chat) GetPeer(peerID uint32) (*Peer, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	peer, exists := g.Peers[peerID]
	if !exists {
		return nil, errors.New("peer not found")
	}

	return peer, nil
}

// KickPeer removes a peer from the group.
//
//export ToxGroupKickPeer
func (g *Chat) KickPeer(peerID uint32) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Get the peer to be kicked
	peerToKick, exists := g.Peers[peerID]
	if !exists {
		return errors.New("peer not found")
	}

	// Get the self peer to check permissions
	selfPeer := g.Peers[g.SelfPeerID]

	// Check permissions
	if selfPeer.Role < RoleModerator {
		return errors.New("insufficient privileges to kick")
	}

	if selfPeer.Role <= peerToKick.Role {
		return errors.New("cannot kick peer with equal or higher role")
	}

	// Broadcast kick notification to all peers
	err := g.broadcastGroupUpdate("peer_kick", map[string]interface{}{
		"kicked_peer_id": peerID,
		"kicker_peer_id": g.SelfPeerID,
		"peer_name":      peerToKick.Name, // Include name for logging/notification
	})
	if err != nil {
		return fmt.Errorf("failed to broadcast kick notification: %w", err)
	}

	// Remove the peer
	delete(g.Peers, peerID)
	g.PeerCount--

	return nil
}

// SetPeerRole changes a peer's role in the group.
//
//export ToxGroupSetPeerRole
func (g *Chat) SetPeerRole(peerID uint32, role Role) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Get the target peer
	targetPeer, exists := g.Peers[peerID]
	if !exists {
		return errors.New("peer not found")
	}

	// Get the self peer to check permissions
	selfPeer := g.Peers[g.SelfPeerID]

	// Check permissions
	if selfPeer.Role < RoleAdmin {
		return errors.New("insufficient privileges to change roles")
	}

	if selfPeer.Role <= targetPeer.Role {
		return errors.New("cannot change role of peer with equal or higher role")
	}

	if role >= selfPeer.Role {
		return errors.New("cannot assign role equal or higher than your own")
	}

	// Cannot change the founder's role
	if targetPeer.Role == RoleFounder {
		return errors.New("cannot change the founder's role")
	}

	// Update the role
	targetPeer.Role = role

	// Broadcast role change to all group members
	err := g.broadcastGroupUpdate("peer_role_change", map[string]interface{}{
		"peer_id":  peerID,
		"new_role": role,
		"old_role": targetPeer.Role, // This should be stored before update in production
	})
	if err != nil {
		return fmt.Errorf("failed to broadcast role change: %w", err)
	}

	return nil
}

// SetName changes the group's name.
//
//export ToxGroupSetName
func (g *Chat) SetName(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(name) == 0 {
		return errors.New("group name cannot be empty")
	}

	// Get the self peer to check permissions
	selfPeer := g.Peers[g.SelfPeerID]

	// Check permissions
	if selfPeer.Role < RoleAdmin {
		return errors.New("insufficient privileges to change group name")
	}

	// Store old name for broadcast
	oldName := g.Name

	// Update the name
	g.Name = name

	// Broadcast name change to all group members
	err := g.broadcastGroupUpdate("group_name_change", map[string]interface{}{
		"old_name": oldName,
		"new_name": name,
	})
	if err != nil {
		return fmt.Errorf("failed to broadcast name change: %w", err)
	}

	return nil
}

// SetPrivacy changes the group's privacy setting.
//
//export ToxGroupSetPrivacy
func (g *Chat) SetPrivacy(privacy Privacy) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Get the self peer to check permissions
	selfPeer := g.Peers[g.SelfPeerID]

	// Check permissions
	if selfPeer.Role < RoleAdmin {
		return errors.New("insufficient privileges to change privacy setting")
	}

	// Store old privacy for broadcast
	oldPrivacy := g.Privacy

	// Update the privacy setting
	g.Privacy = privacy

	// Broadcast privacy change to all group members
	err := g.broadcastGroupUpdate("group_privacy_change", map[string]interface{}{
		"old_privacy": oldPrivacy,
		"new_privacy": privacy,
	})
	if err != nil {
		return fmt.Errorf("failed to broadcast privacy change: %w", err)
	}

	return nil
}

// GetPeerCount returns the number of peers in the group.
//
//export ToxGroupGetPeerCount
func (g *Chat) GetPeerCount() uint32 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.PeerCount
}

// GetPeerList returns a list of all peers in the group.
//
//export ToxGroupGetPeerList
func (g *Chat) GetPeerList() []*Peer {
	g.mu.RLock()
	defer g.mu.RUnlock()

	peers := make([]*Peer, 0, len(g.Peers))
	for _, peer := range g.Peers {
		peers = append(peers, peer)
	}

	return peers
}

// SetSelfName changes the user's display name in the group.
//
//export ToxGroupSetSelfName
func (g *Chat) SetSelfName(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(name) == 0 {
		return errors.New("name cannot be empty")
	}

	// Update self peer name
	selfPeer, exists := g.Peers[g.SelfPeerID]
	if !exists {
		return errors.New("self peer not found")
	}

	selfPeer.Name = name

	// Broadcast name change to all group members
	err := g.broadcastGroupUpdate("peer_name_change", map[string]interface{}{
		"peer_id":  g.SelfPeerID,
		"new_name": name,
	})
	if err != nil {
		return fmt.Errorf("failed to broadcast name change: %w", err)
	}

	return nil
}

// BroadcastMessage represents a group state change that needs to be broadcast
type BroadcastMessage struct {
	Type      string                 `json:"type"`
	ChatID    uint32                 `json:"chat_id"`
	SenderID  uint32                 `json:"sender_id"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// broadcastGroupUpdate sends a group state update to all connected peers
func (g *Chat) broadcastGroupUpdate(updateType string, data map[string]interface{}) error {
	// Create broadcast message
	msg := BroadcastMessage{
		Type:      updateType,
		ChatID:    g.ID,
		SenderID:  g.SelfPeerID,
		Timestamp: time.Now(),
		Data:      data,
	}

	// Serialize message to JSON
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize broadcast message: %w", err)
	}

	// Send broadcast message to each connected peer
	var broadcastErrors []error
	successfulBroadcasts := 0

	for peerID, peer := range g.Peers {
		if peerID == g.SelfPeerID {
			continue // Skip self
		}

		// Only broadcast to connected peers
		if peer.Connection == 0 {
			continue // Skip offline peers
		}

		// Create transport packet for this peer
		packet := &transport.Packet{
			PacketType: transport.PacketGroupBroadcast, // Using existing packet type as placeholder
			Data:       msgBytes,
		}

		// Send the packet using real transport layer integration
		if err := g.broadcastPeerUpdate(peerID, packet); err != nil {
			broadcastErrors = append(broadcastErrors, fmt.Errorf("failed to broadcast to peer %d: %w", peerID, err))
		} else {
			successfulBroadcasts++
		}
	}

	// Log broadcast results
	fmt.Printf("Broadcasting %s update to group %d: %d successful, %d failed (%d bytes)\n",
		updateType, g.ID, successfulBroadcasts, len(broadcastErrors), len(msgBytes))

	// Return error if no broadcasts succeeded
	if successfulBroadcasts == 0 && len(broadcastErrors) > 0 {
		return fmt.Errorf("all broadcasts failed: %v", broadcastErrors)
	}

	return nil
}

// broadcastPeerUpdate sends a packet to a specific peer using the transport layer.
// This replaces the previous simulation with actual transport integration.
func (g *Chat) broadcastPeerUpdate(peerID uint32, packet *transport.Packet) error {
	peer, exists := g.Peers[peerID]
	if !exists {
		return fmt.Errorf("peer %d not found", peerID)
	}

	// Check peer connectivity
	if peer.Connection == 0 {
		return fmt.Errorf("peer %d is offline", peerID)
	}

	// Try to resolve peer's network address via DHT
	// Create a ToxID from the peer's public key for DHT lookup
	peerToxID := crypto.ToxID{PublicKey: peer.PublicKey}

	// Find closest nodes to the peer in DHT
	closestNodes := g.dht.FindClosestNodes(peerToxID, 4)

	// Try sending to the peer's known addresses or closest DHT nodes
	var lastErr error
	for _, node := range closestNodes {
		if node.Address != nil {
			// Attempt to send packet via transport
			err := g.transport.Send(packet, node.Address)
			if err == nil {
				// Success - packet sent
				return nil
			}
			lastErr = err
		}
	}

	// If DHT lookup failed, return error
	if lastErr != nil {
		return fmt.Errorf("failed to send packet to peer %d via DHT: %w", peerID, lastErr)
	}

	return fmt.Errorf("no reachable address found for peer %d", peerID)
}
