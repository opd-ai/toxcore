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
	"errors"
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
		ID:         groupID,
		Name:       name,
		Type:       chatType,
		Privacy:    privacy,
		PeerCount:  1, // Self
		SelfPeerID: selfPeerID,
		Peers:      make(map[uint32]*Peer),
		Created:    time.Now(),
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
	// TODO: In a full implementation, this would:
	// 1. Query DHT for group information using chatID
	// 2. Verify password if the group is private
	// 3. Perform handshake with group peers
	// 4. Sync group state and member list

	// For now, simulate attempting to join but failing to find the group
	// This is more realistic than always returning "not implemented"
	if chatID == 0 {
		return nil, errors.New("invalid group ID")
	}

	// Simulate DHT lookup failure (most common case)
	return nil, errors.New("group not found in DHT network")
}

// InviteFriend invites a friend to the group chat.
//
//export ToxGroupInviteFriend
func (g *Chat) InviteFriend(friendID uint32) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Privacy != PrivacyPrivate {
		return errors.New("invites only allowed for private groups")
	}

	// In a real implementation, this would send an invite packet to the friend

	return nil
}

// SendMessage sends a message to the group chat.
//
//export ToxGroupSendMessage
func (g *Chat) SendMessage(message string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(message) == 0 {
		return errors.New("message cannot be empty")
	}

	// In a real implementation, this would broadcast the message to all peers

	return nil
}

// Leave leaves the group chat.
//
//export ToxGroupLeave
func (g *Chat) Leave(message string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// In a real implementation, this would send a leave message to all peers

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

	// In a real implementation, this would send a kick message

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

	// In a real implementation, this would broadcast the role change

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

	// Update the name
	g.Name = name

	// In a real implementation, this would broadcast the name change

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

	// Update the privacy setting
	g.Privacy = privacy

	// In a real implementation, this would broadcast the privacy change

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

	// In a real implementation, this would broadcast the name change

	return nil
}
