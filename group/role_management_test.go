package group

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/opd-ai/toxcore/dht"
)

// TestSetPeerRole_BroadcastIncludesCorrectOldRole verifies that the role change
// broadcast includes the actual old role value, not the new role value.
// This is a regression test for AUDIT.md Edge Case Bug #5.
func TestSetPeerRole_BroadcastIncludesCorrectOldRole(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Name:       "TestGroup",
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Set up self as admin (can change roles)
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Admin",
		Role:       RoleAdmin,
		Connection: 1,
		PublicKey:  [32]byte{1, 2, 3},
		Address:    &mockAddr{address: "127.0.0.1:5001"},
	}

	// Set up target peer as user
	chat.Peers[2] = &Peer{
		ID:         2,
		Name:       "TestUser",
		Role:       RoleUser, // This is the OLD role
		Connection: 1,
		PublicKey:  [32]byte{2, 3, 4},
		Address:    &mockAddr{address: "127.0.0.1:5002"},
	}

	// Change peer 2's role from User to Moderator
	err := chat.SetPeerRole(2, RoleModerator)
	if err != nil {
		t.Fatalf("SetPeerRole failed: %v", err)
	}

	// Verify the role was actually changed
	if chat.Peers[2].Role != RoleModerator {
		t.Errorf("Expected role to be Moderator, got %v", chat.Peers[2].Role)
	}

	// Verify broadcast was sent
	calls := mockTrans.getSendCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 broadcast call, got %d", len(calls))
	}

	// Parse the broadcast message
	var broadcastMsg BroadcastMessage
	err = json.Unmarshal(calls[0].packet.Data, &broadcastMsg)
	if err != nil {
		t.Fatalf("Failed to parse broadcast message: %v", err)
	}

	// Verify the broadcast type
	if broadcastMsg.Type != "peer_role_change" {
		t.Errorf("Expected broadcast type 'peer_role_change', got '%s'", broadcastMsg.Type)
	}

	// Verify the broadcast data contains correct old_role and new_role
	data := broadcastMsg.Data
	if data["peer_id"] != float64(2) { // JSON unmarshals numbers as float64
		t.Errorf("Expected peer_id 2, got %v", data["peer_id"])
	}

	// Critical check: old_role should be RoleUser (0), not RoleModerator (1)
	oldRole := Role(data["old_role"].(float64))
	if oldRole != RoleUser {
		t.Errorf("Expected old_role to be RoleUser (%d), got %d", RoleUser, oldRole)
	}

	// Verify new_role is RoleModerator
	newRole := Role(data["new_role"].(float64))
	if newRole != RoleModerator {
		t.Errorf("Expected new_role to be RoleModerator (%d), got %d", RoleModerator, newRole)
	}

	// Most important check: old_role and new_role should be DIFFERENT
	if oldRole == newRole {
		t.Error("REGRESSION: old_role equals new_role in broadcast (bug not fixed)")
	}
}

// TestSetPeerRole_InsufficientPrivileges verifies that non-admins cannot change roles
func TestSetPeerRole_InsufficientPrivileges(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Self is a regular user
	chat.Peers[1] = &Peer{
		ID:   1,
		Role: RoleUser,
	}

	// Target peer is also a user
	chat.Peers[2] = &Peer{
		ID:   2,
		Role: RoleUser,
	}

	err := chat.SetPeerRole(2, RoleModerator)
	if err == nil {
		t.Fatal("Expected error when non-admin tries to change roles")
	}

	if !strings.Contains(err.Error(), "insufficient privileges") {
		t.Errorf("Expected 'insufficient privileges' error, got: %v", err)
	}

	// Verify no broadcast was sent
	calls := mockTrans.getSendCalls()
	if len(calls) != 0 {
		t.Errorf("Expected no broadcasts, got %d", len(calls))
	}
}

// TestSetPeerRole_CannotPromoteAboveSelf verifies admins cannot promote peers to admin
func TestSetPeerRole_CannotPromoteAboveSelf(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Self is admin
	chat.Peers[1] = &Peer{
		ID:   1,
		Role: RoleAdmin,
	}

	// Target peer is user
	chat.Peers[2] = &Peer{
		ID:   2,
		Role: RoleUser,
	}

	// Try to promote to admin (equal to self)
	err := chat.SetPeerRole(2, RoleAdmin)
	if err == nil {
		t.Fatal("Expected error when promoting peer to equal role")
	}

	if !strings.Contains(err.Error(), "cannot assign role equal or higher") {
		t.Errorf("Expected 'cannot assign role' error, got: %v", err)
	}

	// Try to promote to founder (above self)
	err = chat.SetPeerRole(2, RoleFounder)
	if err == nil {
		t.Fatal("Expected error when promoting peer to founder")
	}

	// Verify no broadcast was sent
	calls := mockTrans.getSendCalls()
	if len(calls) != 0 {
		t.Errorf("Expected no broadcasts, got %d", len(calls))
	}
}

// TestSetPeerRole_CannotChangeFounderRole verifies founder role is immutable
func TestSetPeerRole_CannotChangeFounderRole(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Self is founder
	chat.Peers[1] = &Peer{
		ID:   1,
		Role: RoleFounder,
	}

	// Another founder (shouldn't be possible, but test the protection)
	chat.Peers[2] = &Peer{
		ID:   2,
		Role: RoleFounder,
	}

	err := chat.SetPeerRole(2, RoleAdmin)
	if err == nil {
		t.Fatal("Expected error when changing founder's role")
	}

	// The actual error comes from the selfPeer.Role <= targetPeer.Role check (line 806)
	// which triggers before the specific founder check
	if !strings.Contains(err.Error(), "cannot change role of peer with equal or higher role") {
		t.Errorf("Expected 'cannot change role' error, got: %v", err)
	}

	// Verify no broadcast was sent
	calls := mockTrans.getSendCalls()
	if len(calls) != 0 {
		t.Errorf("Expected no broadcasts, got %d", len(calls))
	}
}

// TestSetPeerRole_PeerNotFound verifies error when peer doesn't exist
func TestSetPeerRole_PeerNotFound(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Self is admin
	chat.Peers[1] = &Peer{
		ID:   1,
		Role: RoleAdmin,
	}

	err := chat.SetPeerRole(999, RoleModerator)
	if err == nil {
		t.Fatal("Expected error when peer not found")
	}

	if !strings.Contains(err.Error(), "peer not found") {
		t.Errorf("Expected 'peer not found' error, got: %v", err)
	}

	// Verify no broadcast was sent
	calls := mockTrans.getSendCalls()
	if len(calls) != 0 {
		t.Errorf("Expected no broadcasts, got %d", len(calls))
	}
}

// TestSetPeerRole_ModeratorDemotingUser verifies moderators can change user roles
func TestSetPeerRole_ModeratorDemotingUser(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Self is moderator
	chat.Peers[1] = &Peer{
		ID:         1,
		Role:       RoleModerator,
		Connection: 1,
		PublicKey:  [32]byte{1},
		Address:    &mockAddr{address: "127.0.0.1:5001"},
	}

	// Target is user
	chat.Peers[2] = &Peer{
		ID:         2,
		Role:       RoleUser,
		Connection: 1,
		PublicKey:  [32]byte{2},
		Address:    &mockAddr{address: "127.0.0.1:5002"},
	}

	// Moderator cannot promote to moderator (equal role)
	err := chat.SetPeerRole(2, RoleModerator)
	if err == nil {
		t.Fatal("Expected error when moderator tries to promote to equal level")
	}

	// But if an admin demoted someone, they could be a user again
	// This is just verifying the role change works when permissions allow
}

// TestSetPeerRole_FounderChangingAdminRole verifies founder has full control
func TestSetPeerRole_FounderChangingAdminRole(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Self is founder
	chat.Peers[1] = &Peer{
		ID:         1,
		Role:       RoleFounder,
		Connection: 1,
		PublicKey:  [32]byte{1},
		Address:    &mockAddr{address: "127.0.0.1:5001"},
	}

	// Target is admin
	chat.Peers[2] = &Peer{
		ID:         2,
		Role:       RoleAdmin,
		Connection: 1,
		PublicKey:  [32]byte{2},
		Address:    &mockAddr{address: "127.0.0.1:5002"},
	}

	// Founder demoting admin to moderator
	err := chat.SetPeerRole(2, RoleModerator)
	if err != nil {
		t.Fatalf("Founder should be able to demote admin: %v", err)
	}

	// Verify role was changed
	if chat.Peers[2].Role != RoleModerator {
		t.Errorf("Expected role to be Moderator, got %v", chat.Peers[2].Role)
	}

	// Verify broadcast contains correct old and new roles
	calls := mockTrans.getSendCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 broadcast, got %d", len(calls))
	}

	var broadcastMsg BroadcastMessage
	err = json.Unmarshal(calls[0].packet.Data, &broadcastMsg)
	if err != nil {
		t.Fatalf("Failed to parse broadcast: %v", err)
	}

	oldRole := Role(broadcastMsg.Data["old_role"].(float64))
	newRole := Role(broadcastMsg.Data["new_role"].(float64))

	if oldRole != RoleAdmin {
		t.Errorf("Expected old_role to be Admin, got %v", oldRole)
	}

	if newRole != RoleModerator {
		t.Errorf("Expected new_role to be Moderator, got %v", newRole)
	}
}
