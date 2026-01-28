package group

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// TestJoinValidGroupID tests that joining a registered group succeeds
func TestJoinValidGroupID(t *testing.T) {
	// First create a group to register it
	groupName := "Test Group"
	chat, err := Create(groupName, ChatTypeText, PrivacyPublic, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	defer unregisterGroup(chat.ID) // Cleanup

	chatID := chat.ID
	password := "test-password"

	// Now join should succeed
	joinedChat, err := Join(chatID, password)
	if err != nil {
		t.Fatalf("Expected successful join, got error: %v", err)
	}

	if joinedChat == nil {
		t.Fatal("Expected non-nil chat when Join succeeds")
	}

	// Verify the joined chat has the correct information
	if joinedChat.Name != groupName {
		t.Errorf("Expected group name '%s', got '%s'", groupName, joinedChat.Name)
	}

	if joinedChat.ID != chatID {
		t.Errorf("Expected group ID %d, got %d", chatID, joinedChat.ID)
	}
}

// TestJoinUnregisteredGroup tests that joining an unregistered group fails
func TestJoinUnregisteredGroup(t *testing.T) {
	chatID := uint32(99999)
	password := "test-password"

	chat, err := Join(chatID, password)

	// Join should fail because group is not registered
	if err == nil {
		t.Fatal("Expected error when joining unregistered group")
	}

	if chat != nil {
		t.Error("Expected nil chat when Join fails")
	}

	// Verify error message indicates DHT lookup failure
	expectedError := "not found in DHT"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

// TestJoinInvalidGroupID tests that joining with group ID 0 fails
func TestJoinInvalidGroupID(t *testing.T) {
	chatID := uint32(0)
	password := "test-password"

	chat, err := Join(chatID, password)

	if err == nil {
		t.Fatal("Expected error when joining with group ID 0")
	}

	if chat != nil {
		t.Error("Expected nil chat when error occurs")
	}

	expectedError := "invalid group ID"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

// TestJoinPrivateGroupWithoutPassword tests that joining private group without password fails
func TestJoinPrivateGroupWithoutPassword(t *testing.T) {
	// Create a private group
	groupName := "Private Group"
	chat, err := Create(groupName, ChatTypeText, PrivacyPrivate, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	defer unregisterGroup(chat.ID)

	chatID := chat.ID
	password := "" // Empty password

	joinedChat, err := Join(chatID, password)

	// Join should fail due to missing password for private group
	if err == nil {
		t.Fatal("Expected error when joining private group without password")
	}

	if joinedChat != nil {
		t.Error("Expected nil chat when error occurs")
	}

	// Error should be about password requirement
	expectedError := "password required"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

// TestJoinDHTLookupFailure tests that Join returns error when group is not registered
func TestJoinDHTLookupFailure(t *testing.T) {
	chatID := uint32(99999)
	password := "test-password"

	chat, err := Join(chatID, password)

	// Join should fail because group is not registered
	if err == nil {
		t.Fatal("Expected error when DHT lookup fails")
	}

	if chat != nil {
		t.Error("Expected nil chat when DHT lookup fails")
	}

	// Verify error indicates DHT lookup failure
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected error about group not found, got: %v", err)
	}
}

// TestJoinConcurrency tests that Join works correctly when called concurrently
func TestJoinConcurrency(t *testing.T) {
	const goroutines = 10
	results := make(chan error, goroutines)

	// Create groups for concurrent joining
	groupIDs := make([]uint32, goroutines)
	for i := 0; i < goroutines; i++ {
		chat, err := Create(fmt.Sprintf("Group %d", i), ChatTypeText, PrivacyPublic, nil, nil)
		if err != nil {
			t.Fatalf("Failed to create group %d: %v", i, err)
		}
		groupIDs[i] = chat.ID
		defer unregisterGroup(chat.ID)
	}

	// Join groups concurrently
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			chatID := groupIDs[id]
			password := "test-password"

			chat, err := Join(chatID, password)
			if err != nil {
				results <- fmt.Errorf("join failed: %w", err)
				return
			}

			if chat == nil {
				results <- fmt.Errorf("expected non-nil chat")
				return
			}

			results <- nil
		}(i)
	}

	// Collect results - all should succeed
	for i := 0; i < goroutines; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent Join test failed: %v", err)
		}
	}
}

// TestJoinDifferentGroupIDs tests that joining fails for unregistered groups
func TestJoinDifferentGroupIDs(t *testing.T) {
	testCases := []struct {
		chatID   uint32
		password string
	}{
		{chatID: 1, password: "pwd1"},
		{chatID: 100, password: "pwd2"},
		{chatID: 999999, password: "pwd3"},
		{chatID: 4294967295, password: "pwd4"}, // Max uint32
	}

	for _, tc := range testCases {
		chat, err := Join(tc.chatID, tc.password)
		// All joins should fail because groups are not registered
		if err == nil {
			t.Errorf("Expected error for group ID %d, but Join succeeded", tc.chatID)
			continue
		}

		if chat != nil {
			t.Errorf("Expected nil chat for group ID %d, got non-nil", tc.chatID)
		}

		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error for group ID %d, got: %v", tc.chatID, err)
		}
	}
}

// TestJoinConsistentFailure tests that Join consistently fails for unregistered groups
func TestJoinConsistentFailure(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		chat, err := Join(uint32(1000+i), "password")
		if err == nil {
			t.Errorf("Expected error at iteration %d, but Join succeeded", i)
		}

		if chat != nil {
			t.Errorf("Expected nil chat at iteration %d, got non-nil", i)
		}

		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error at iteration %d, got: %v", i, err)
		}
	}
}

// TestUpdatePeerAddress tests updating peer addresses
func TestUpdatePeerAddress(t *testing.T) {
	chat := &Chat{
		ID:    1,
		Peers: make(map[uint32]*Peer),
	}

	// Add a peer
	peerID := uint32(100)
	chat.Peers[peerID] = &Peer{
		ID:        peerID,
		Name:      "TestPeer",
		PublicKey: [32]byte{1, 2, 3},
	}

	// Create a test address
	testAddr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 33445,
	}

	// Update peer address
	err := chat.UpdatePeerAddress(peerID, testAddr)
	if err != nil {
		t.Fatalf("UpdatePeerAddress failed: %v", err)
	}

	// Verify address was updated
	peer := chat.Peers[peerID]
	if peer.Address == nil {
		t.Fatal("Peer address was not set")
	}

	// Verify the address matches
	udpAddr, ok := peer.Address.(*net.UDPAddr)
	if !ok {
		t.Fatal("Address is not a UDPAddr")
	}

	if udpAddr.IP.String() != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", udpAddr.IP.String())
	}

	if udpAddr.Port != 33445 {
		t.Errorf("Expected port 33445, got %d", udpAddr.Port)
	}
}

// TestUpdatePeerAddressNonExistentPeer tests error when peer doesn't exist
func TestUpdatePeerAddressNonExistentPeer(t *testing.T) {
	chat := &Chat{
		ID:    1,
		Peers: make(map[uint32]*Peer),
	}

	testAddr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 33445,
	}

	err := chat.UpdatePeerAddress(999, testAddr)
	if err == nil {
		t.Fatal("Expected error when updating non-existent peer")
	}

	if !strings.Contains(err.Error(), "peer 999 not found") {
		t.Errorf("Expected 'peer not found' error, got: %v", err)
	}
}

// TestUpdatePeerAddressUpdatesLastActive tests that LastActive is updated
func TestUpdatePeerAddressUpdatesLastActive(t *testing.T) {
	chat := &Chat{
		ID:    1,
		Peers: make(map[uint32]*Peer),
	}

	peerID := uint32(100)
	oldTime := time.Now().Add(-1 * time.Hour)
	chat.Peers[peerID] = &Peer{
		ID:         peerID,
		Name:       "TestPeer",
		PublicKey:  [32]byte{1, 2, 3},
		LastActive: oldTime,
	}

	testAddr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 33445,
	}

	err := chat.UpdatePeerAddress(peerID, testAddr)
	if err != nil {
		t.Fatalf("UpdatePeerAddress failed: %v", err)
	}

	peer := chat.Peers[peerID]
	if peer.LastActive.Before(oldTime) || peer.LastActive.Equal(oldTime) {
		t.Error("LastActive was not updated")
	}
}

// TestUpdatePeerAddressConcurrency tests concurrent address updates
func TestUpdatePeerAddressConcurrency(t *testing.T) {
	chat := &Chat{
		ID:    1,
		Peers: make(map[uint32]*Peer),
	}

	// Add multiple peers
	for i := uint32(1); i <= 10; i++ {
		chat.Peers[i] = &Peer{
			ID:        i,
			Name:      fmt.Sprintf("Peer%d", i),
			PublicKey: [32]byte{byte(i)},
		}
	}

	const goroutines = 100
	results := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			peerID := uint32((id % 10) + 1)
			testAddr := &net.UDPAddr{
				IP:   net.ParseIP("192.168.1.1"),
				Port: 30000 + id,
			}
			results <- chat.UpdatePeerAddress(peerID, testAddr)
		}(i)
	}

	// Collect results
	for i := 0; i < goroutines; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent UpdatePeerAddress failed: %v", err)
		}
	}

	// Verify all peers have addresses set
	for i := uint32(1); i <= 10; i++ {
		if chat.Peers[i].Address == nil {
			t.Errorf("Peer %d address not set after concurrent updates", i)
		}
	}
}

// TestInviteFriendToPublicGroup tests that inviting friends to public groups works
func TestInviteFriendToPublicGroup(t *testing.T) {
	chat := &Chat{
		ID:                 1,
		Name:               "Public Group",
		Privacy:            PrivacyPublic,
		Peers:              make(map[uint32]*Peer),
		PendingInvitations: make(map[uint32]*Invitation),
	}

	friendID := uint32(100)

	err := chat.InviteFriend(friendID)
	if err != nil {
		t.Fatalf("InviteFriend failed for public group: %v", err)
	}

	// Verify invitation was created
	if _, exists := chat.PendingInvitations[friendID]; !exists {
		t.Error("Invitation was not created for friend")
	}
}

// TestInviteFriendToPrivateGroup tests that inviting friends to private groups works
func TestInviteFriendToPrivateGroup(t *testing.T) {
	chat := &Chat{
		ID:                 1,
		Name:               "Private Group",
		Privacy:            PrivacyPrivate,
		Peers:              make(map[uint32]*Peer),
		PendingInvitations: make(map[uint32]*Invitation),
	}

	friendID := uint32(200)

	err := chat.InviteFriend(friendID)
	if err != nil {
		t.Fatalf("InviteFriend failed for private group: %v", err)
	}

	// Verify invitation was created
	if _, exists := chat.PendingInvitations[friendID]; !exists {
		t.Error("Invitation was not created for friend")
	}
}

// TestInviteFriendWithInvalidID tests that invalid friend IDs are rejected
func TestInviteFriendWithInvalidID(t *testing.T) {
	chat := &Chat{
		ID:                 1,
		Name:               "Test Group",
		Privacy:            PrivacyPublic,
		Peers:              make(map[uint32]*Peer),
		PendingInvitations: make(map[uint32]*Invitation),
	}

	err := chat.InviteFriend(0)
	if err == nil {
		t.Fatal("Expected error when inviting friend with ID 0")
	}

	if !strings.Contains(err.Error(), "invalid friend ID") {
		t.Errorf("Expected 'invalid friend ID' error, got: %v", err)
	}
}

// TestInviteFriendAlreadyInvited tests that duplicate invitations are rejected
func TestInviteFriendAlreadyInvited(t *testing.T) {
	chat := &Chat{
		ID:                 1,
		Name:               "Test Group",
		Privacy:            PrivacyPublic,
		Peers:              make(map[uint32]*Peer),
		PendingInvitations: make(map[uint32]*Invitation),
	}

	friendID := uint32(100)

	// First invitation should succeed
	err := chat.InviteFriend(friendID)
	if err != nil {
		t.Fatalf("First InviteFriend call failed: %v", err)
	}

	// Second invitation should fail
	err = chat.InviteFriend(friendID)
	if err == nil {
		t.Fatal("Expected error when inviting already invited friend")
	}

	if !strings.Contains(err.Error(), "already has a pending invitation") {
		t.Errorf("Expected 'already has a pending invitation' error, got: %v", err)
	}
}

// TestInviteFriendAlreadyInGroup tests that inviting existing group members is rejected
func TestInviteFriendAlreadyInGroup(t *testing.T) {
	friendID := uint32(100)

	chat := &Chat{
		ID:      1,
		Name:    "Test Group",
		Privacy: PrivacyPublic,
		Peers: map[uint32]*Peer{
			friendID: {
				ID:        friendID,
				Name:      "Existing Member",
				PublicKey: [32]byte{1, 2, 3},
			},
		},
		PendingInvitations: make(map[uint32]*Invitation),
	}

	err := chat.InviteFriend(friendID)
	if err == nil {
		t.Fatal("Expected error when inviting friend already in group")
	}

	if !strings.Contains(err.Error(), "already in the group") {
		t.Errorf("Expected 'already in the group' error, got: %v", err)
	}
}

// TestInviteFriendBothPrivacyTypes tests invitations work for both public and private groups
func TestInviteFriendBothPrivacyTypes(t *testing.T) {
	testCases := []struct {
		name    string
		privacy Privacy
	}{
		{"Public Group", PrivacyPublic},
		{"Private Group", PrivacyPrivate},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chat := &Chat{
				ID:                 1,
				Name:               tc.name,
				Privacy:            tc.privacy,
				Peers:              make(map[uint32]*Peer),
				PendingInvitations: make(map[uint32]*Invitation),
			}

			friendID := uint32(100)

			err := chat.InviteFriend(friendID)
			if err != nil {
				t.Fatalf("InviteFriend failed for %s: %v", tc.name, err)
			}

			// Verify invitation was created
			if _, exists := chat.PendingInvitations[friendID]; !exists {
				t.Errorf("Invitation was not created for %s", tc.name)
			}
		})
	}
}

// TestInviteFriendConcurrency tests concurrent invitation requests
func TestInviteFriendConcurrency(t *testing.T) {
	chat := &Chat{
		ID:                 1,
		Name:               "Test Group",
		Privacy:            PrivacyPublic,
		Peers:              make(map[uint32]*Peer),
		PendingInvitations: make(map[uint32]*Invitation),
	}

	const goroutines = 50
	results := make(chan error, goroutines)

	// Invite different friends concurrently
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			friendID := uint32(100 + id)
			results <- chat.InviteFriend(friendID)
		}(i)
	}

	// Collect results
	for i := 0; i < goroutines; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent InviteFriend failed: %v", err)
		}
	}

	// Verify all invitations were created
	if len(chat.PendingInvitations) != goroutines {
		t.Errorf("Expected %d invitations, got %d", goroutines, len(chat.PendingInvitations))
	}
}

// TestGroupRegistration tests that groups are properly registered and unregistered
func TestGroupRegistration(t *testing.T) {
	groupName := "Registration Test Group"
	chat, err := Create(groupName, ChatTypeText, PrivacyPublic, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Verify group is registered
	info, err := queryDHTForGroup(chat.ID)
	if err != nil {
		t.Fatalf("Group should be registered after creation: %v", err)
	}

	if info.Name != groupName {
		t.Errorf("Expected group name '%s', got '%s'", groupName, info.Name)
	}

	// Unregister and verify
	unregisterGroup(chat.ID)
	_, err = queryDHTForGroup(chat.ID)
	if err == nil {
		t.Error("Group should not be found after unregistration")
	}
}

// TestGroupRegistrationConcurrency tests concurrent group registration
func TestGroupRegistrationConcurrency(t *testing.T) {
	const goroutines = 20
	results := make(chan error, goroutines)
	groupIDs := make(chan uint32, goroutines)

	// Create groups concurrently
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			chat, err := Create(fmt.Sprintf("Concurrent Group %d", id), ChatTypeText, PrivacyPublic, nil, nil)
			if err != nil {
				results <- fmt.Errorf("create failed: %w", err)
				groupIDs <- 0
				return
			}
			groupIDs <- chat.ID
			results <- nil
		}(i)
	}

	// Collect group IDs and check for errors
	var createdIDs []uint32
	for i := 0; i < goroutines; i++ {
		err := <-results
		groupID := <-groupIDs
		if err != nil {
			t.Errorf("Concurrent creation failed: %v", err)
		} else {
			createdIDs = append(createdIDs, groupID)
		}
	}

	// Verify all groups are registered
	for _, id := range createdIDs {
		if id == 0 {
			continue
		}
		_, err := queryDHTForGroup(id)
		if err != nil {
			t.Errorf("Group %d should be registered: %v", id, err)
		}
	}

	// Cleanup
	for _, id := range createdIDs {
		if id != 0 {
			unregisterGroup(id)
		}
	}
}

// TestJoinPublicGroupSuccess tests successful joining of a public group
func TestJoinPublicGroupSuccess(t *testing.T) {
	// Create a public group
	groupName := "Public Test Group"
	creator, err := Create(groupName, ChatTypeText, PrivacyPublic, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	defer unregisterGroup(creator.ID)

	// Join the group
	joiner, err := Join(creator.ID, "")
	if err != nil {
		t.Fatalf("Failed to join public group: %v", err)
	}

	// Verify joined group properties
	if joiner.ID != creator.ID {
		t.Errorf("Expected group ID %d, got %d", creator.ID, joiner.ID)
	}

	if joiner.Name != groupName {
		t.Errorf("Expected group name '%s', got '%s'", groupName, joiner.Name)
	}

	if joiner.Type != ChatTypeText {
		t.Errorf("Expected chat type %v, got %v", ChatTypeText, joiner.Type)
	}

	if joiner.Privacy != PrivacyPublic {
		t.Errorf("Expected privacy %v, got %v", PrivacyPublic, joiner.Privacy)
	}

	// Verify joiner has a peer ID
	if joiner.SelfPeerID == 0 {
		t.Error("Joiner should have a non-zero peer ID")
	}

	// Verify joiner has self in peers map
	if _, exists := joiner.Peers[joiner.SelfPeerID]; !exists {
		t.Error("Joiner should have self in peers map")
	}
}

// TestJoinPrivateGroupSuccess tests successful joining of a private group with password
func TestJoinPrivateGroupSuccess(t *testing.T) {
	// Create a private group
	groupName := "Private Test Group"
	creator, err := Create(groupName, ChatTypeText, PrivacyPrivate, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	defer unregisterGroup(creator.ID)

	// Join the group with password
	password := "secret123"
	joiner, err := Join(creator.ID, password)
	if err != nil {
		t.Fatalf("Failed to join private group: %v", err)
	}

	// Verify joined group properties
	if joiner.ID != creator.ID {
		t.Errorf("Expected group ID %d, got %d", creator.ID, joiner.ID)
	}

	if joiner.Privacy != PrivacyPrivate {
		t.Errorf("Expected privacy %v, got %v", PrivacyPrivate, joiner.Privacy)
	}
}

// TestLeaveGroupUnregistration tests that founder leaving unregisters the group
func TestLeaveGroupUnregistration(t *testing.T) {
	// Create a group
	creator, err := Create("Leave Test Group", ChatTypeText, PrivacyPublic, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	groupID := creator.ID

	// Verify group is registered
	_, err = queryDHTForGroup(groupID)
	if err != nil {
		t.Fatalf("Group should be registered: %v", err)
	}

	// Founder leaves
	err = creator.Leave("Goodbye")
	if err != nil {
		t.Fatalf("Failed to leave group: %v", err)
	}

	// Verify group is unregistered
	_, err = queryDHTForGroup(groupID)
	if err == nil {
		t.Error("Group should be unregistered after founder leaves")
	}
}

// TestQueryDHTForGroupCopiesData tests that queryDHTForGroup returns a copy
func TestQueryDHTForGroupCopiesData(t *testing.T) {
	// Create a group
	groupName := "Copy Test Group"
	creator, err := Create(groupName, ChatTypeText, PrivacyPublic, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	defer unregisterGroup(creator.ID)

	// Get group info
	info1, err := queryDHTForGroup(creator.ID)
	if err != nil {
		t.Fatalf("Failed to query group: %v", err)
	}

	// Modify the returned info
	info1.Name = "Modified Name"

	// Get group info again
	info2, err := queryDHTForGroup(creator.ID)
	if err != nil {
		t.Fatalf("Failed to query group again: %v", err)
	}

	// Verify original name is unchanged
	if info2.Name != groupName {
		t.Errorf("Expected original name '%s', got '%s'", groupName, info2.Name)
	}
}

// TestJoinMultipleGroups tests joining multiple different groups
func TestJoinMultipleGroups(t *testing.T) {
	const numGroups = 5
	var groupIDs []uint32

	// Create multiple groups
	for i := 0; i < numGroups; i++ {
		chat, err := Create(fmt.Sprintf("Multi Group %d", i), ChatTypeText, PrivacyPublic, nil, nil)
		if err != nil {
			t.Fatalf("Failed to create group %d: %v", i, err)
		}
		groupIDs = append(groupIDs, chat.ID)
		defer unregisterGroup(chat.ID)
	}

	// Join all groups
	for i, groupID := range groupIDs {
		joiner, err := Join(groupID, "")
		if err != nil {
			t.Errorf("Failed to join group %d: %v", i, err)
			continue
		}

		if joiner.ID != groupID {
			t.Errorf("Group %d: expected ID %d, got %d", i, groupID, joiner.ID)
		}

		expectedName := fmt.Sprintf("Multi Group %d", i)
		if joiner.Name != expectedName {
			t.Errorf("Group %d: expected name '%s', got '%s'", i, expectedName, joiner.Name)
		}
	}
}

// TestJoinAVGroup tests joining an audio/video group
func TestJoinAVGroup(t *testing.T) {
	// Create an AV group
	creator, err := Create("AV Group", ChatTypeAV, PrivacyPublic, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create AV group: %v", err)
	}
	defer unregisterGroup(creator.ID)

	// Join the AV group
	joiner, err := Join(creator.ID, "")
	if err != nil {
		t.Fatalf("Failed to join AV group: %v", err)
	}

	if joiner.Type != ChatTypeAV {
		t.Errorf("Expected chat type %v, got %v", ChatTypeAV, joiner.Type)
	}
}
