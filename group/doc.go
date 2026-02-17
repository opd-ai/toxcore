// Package group implements group chat functionality for the Tox protocol,
// supporting DHT-based discovery, role management, and peer-to-peer message
// broadcasting.
//
// # Overview
//
// The group package provides comprehensive group chat capabilities:
//
//   - Group creation with configurable privacy and chat types
//   - DHT-based group discovery across processes and networks
//   - Local registry for same-process group lookups
//   - Role-based permissions (Founder, Moderator, User, Observer)
//   - Peer-to-peer message broadcasting with worker pool optimization
//   - Friend invitation system for private groups
//
// # Creating Groups
//
// Create a group with DHT discovery enabled for cross-network visibility:
//
//	group, err := group.Create("Programming Chat", group.ChatTypeText,
//	    group.PrivacyPublic, transport, dhtRoutingTable)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer group.Leave()
//
//	fmt.Printf("Group ID: %x\n", group.GetID())
//	fmt.Printf("Group Name: %s\n", group.GetName())
//
// # Joining Groups
//
// Join existing groups via DHT discovery or invitation:
//
//	// Join via DHT network discovery
//	joinedGroup, err := group.Join(groupID, "", transport, dhtRoutingTable)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Join via invitation with password
//	joinedGroup, err := group.Join(groupID, "secret123", transport, dhtRoutingTable)
//
// # Messaging
//
// Send and receive messages within the group:
//
//	// Set up message callback
//	group.OnMessage(func(peerID uint32, message string) {
//	    fmt.Printf("Message from peer %d: %s\n", peerID, message)
//	})
//
//	// Send a message to all peers
//	err := group.SendMessage("Hello everyone!")
//
// # Group Discovery
//
// The package supports two discovery mechanisms:
//
// Local Discovery: Groups created in the same process are stored in a local
// registry for fast lookups without network overhead. This is automatic.
//
// DHT Discovery: When transport and DHT routing table are provided, groups
// are announced to the distributed Tox DHT network. Other peers can discover
// groups by querying DHT nodes.
//
//	// For optimal discovery, provide both transport and DHT
//	group, _ := group.Create(name, chatType, privacy, transport, dhtRoutingTable)
//
//	// Query DHT for group announcements (handled internally by Join)
//	// Share group IDs via friend messages for invitation-based joining
//
// # Role Management
//
// Groups support hierarchical role-based permissions:
//
//	const (
//	    RoleFounder    // Full control, can delete group
//	    RoleModerator  // Can kick/ban users, moderate messages
//	    RoleUser       // Can send messages, participate in chat
//	    RoleObserver   // Can only read messages
//	)
//
//	// Set peer role (requires Founder or Moderator privileges)
//	err := group.SetPeerRole(peerID, group.RoleModerator)
//
//	// Get peer information
//	role := group.GetPeerRole(peerID)
//	name := group.GetPeerName(peerID)
//
// # Chat Types
//
// Two chat types are supported:
//
//	const (
//	    ChatTypeText  // Text-only group chat
//	    ChatTypeAV    // Audio/video enabled group chat
//	)
//
// # Privacy Settings
//
// Control group visibility and access:
//
//	const (
//	    PrivacyPublic   // Anyone can join via DHT discovery
//	    PrivacyPrivate  // Invitation only, requires password
//	)
//
// # Friend Invitations
//
// Invite friends to join the group:
//
//	// Invite a friend (they receive group ID and optional password)
//	err := group.InviteFriend(friendID)
//
//	// Handle incoming invitations via callback
//	tox.OnGroupInvite(func(friendID uint32, groupID [32]byte) {
//	    // Auto-accept or prompt user
//	})
//
// # Peer Management
//
// Query and manage group peers:
//
//	// Get peer count and list
//	count := group.GetPeerCount()
//	peers := group.GetPeers()
//
//	for _, peer := range peers {
//	    fmt.Printf("Peer %d: %s (role: %d)\n", peer.ID, peer.Name, peer.Role)
//	}
//
//	// Kick a peer (requires Moderator privileges)
//	err := group.KickPeer(peerID)
//
// # Deterministic Testing
//
// For reproducible test scenarios, use the TimeProvider interface:
//
//	group.SetTimeProvider(&MockTimeProvider{currentTime: fixedTime})
//
// The TimeProvider allows injection of controlled time values for testing
// timestamp generation, peer timeouts, and message ordering.
//
// # Thread Safety
//
// All exported methods use sync.RWMutex for concurrent access safety.
// Message callbacks are invoked synchronously from the broadcast worker pool.
// The package passes Go's race detector validation.
//
// # Broadcast Optimization
//
// Message broadcasting uses a worker pool pattern for efficient delivery:
//
//   - Messages are dispatched to all peers in parallel
//   - Worker count scales with peer count (capped at runtime.NumCPU)
//   - Partial delivery failures are logged but don't block other peers
//   - Results are aggregated and logged via structured logging
//
// # Integration
//
// The group package integrates with core toxcore-go infrastructure:
//
//   - Transport layer for packet transmission via transport.Transport
//   - DHT routing table for group announcements via dht.RoutingTable
//   - Crypto package for group ID generation via crypto.ToxID
//   - Main Tox struct stores groups in conferences map
//
// # C Bindings
//
// The package provides 18 exported C functions for interoperability.
// See chat.go for //export annotations on all public functions.
package group
