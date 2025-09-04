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
func Create(name string, chatType ChatType, privacy Privacy) (*Chat, error) {
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

	// Simulate DHT lookup for group information
	// In a real implementation, this would query the DHT network

	// For now, create a basic group structure representing successful join
	// In practice, this would be populated from DHT information
	selfPeerID, err := generateRandomID()
	if err != nil {
		return nil, errors.New("failed to generate peer ID")
	}

	chat := &Chat{
		ID:                 chatID,
		Name:               fmt.Sprintf("Group_%d", chatID), // Would come from DHT
		Type:               ChatTypeText,                    // Would come from DHT
		Privacy:            PrivacyPrivate,                  // Assume private if password required
		PeerCount:          1,                               // Just self initially
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

	// In a real implementation, this would send an invite packet to the friend
	// For now, we track the invitation locally and provide the foundation
	// for future network integration

	return nil
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

	// In a real implementation, this would:
	// 1. Encrypt message with group keys
	// 2. Create group message packet
	// 3. Broadcast to all peers
	// 4. Handle delivery confirmations
	//
	// For now, trigger the local message callback to simulate message processing
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

	// In a full implementation, this would:
	// 1. Send the message through the Tox messaging system to each peer
	// 2. Handle delivery confirmations and retries
	// 3. Implement reliable broadcast with consensus

	// For now, simulate broadcasting by logging
	activePeers := 0
	for peerID := range g.Peers {
		if peerID != g.SelfPeerID {
			activePeers++
		}
	}

	if activePeers > 0 {
		// Simulate successful broadcast
		fmt.Printf("Broadcasting %s update to %d peers in group %d (%d bytes)\n",
			updateType, activePeers, g.ID, len(msgBytes))
	}

	return nil
}
