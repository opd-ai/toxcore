package group

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"strings"
	"testing"
	"time"
)

// TestJoinValidGroupID tests joining a group with a valid group ID
func TestJoinValidGroupID(t *testing.T) {
	// Capture log output to verify warning is logged
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(log.Writer())

	chatID := uint32(12345)
	password := "test-password"

	chat, err := Join(chatID, password)

	if err != nil {
		t.Fatalf("Join failed with valid group ID: %v", err)
	}

	if chat == nil {
		t.Fatal("Join returned nil chat")
	}

	// Verify chat structure
	if chat.ID != chatID {
		t.Errorf("Expected chat ID %d, got %d", chatID, chat.ID)
	}

	if chat.PeerCount != 1 {
		t.Errorf("Expected peer count 1, got %d", chat.PeerCount)
	}

	if chat.SelfPeerID == 0 {
		t.Error("Expected non-zero self peer ID")
	}

	if len(chat.Peers) != 1 {
		t.Errorf("Expected 1 peer, got %d", len(chat.Peers))
	}

	// Verify self peer exists
	selfPeer, exists := chat.Peers[chat.SelfPeerID]
	if !exists {
		t.Fatal("Self peer not found in peers map")
	}

	if selfPeer.Name != "Self" {
		t.Errorf("Expected self peer name 'Self', got '%s'", selfPeer.Name)
	}

	if selfPeer.Role != RoleUser {
		t.Errorf("Expected self peer role RoleUser, got %v", selfPeer.Role)
	}

	// Verify warning was logged since DHT lookup is not implemented
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "WARNING: Group DHT lookup failed") {
		t.Error("Expected warning about DHT lookup failure to be logged")
	}

	if !strings.Contains(logOutput, "Creating local-only group") {
		t.Error("Expected warning about local-only group to be logged")
	}

	if !strings.Contains(logOutput, "NOT connected to an existing group") {
		t.Error("Expected warning about not being connected to existing group")
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

// TestJoinPrivateGroupWithoutPassword tests that private groups require passwords
func TestJoinPrivateGroupWithoutPassword(t *testing.T) {
	// Capture and suppress log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(log.Writer())

	chatID := uint32(54321)
	password := "" // Empty password

	chat, err := Join(chatID, password)

	if err == nil {
		t.Fatal("Expected error when joining private group without password")
	}

	if chat != nil {
		t.Error("Expected nil chat when error occurs")
	}

	expectedError := "password required for private group"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

// TestJoinDefaultValues tests that default values are set when DHT lookup fails
func TestJoinDefaultValues(t *testing.T) {
	// Capture and suppress log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(log.Writer())

	chatID := uint32(99999)
	password := "test-password"

	chat, err := Join(chatID, password)

	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}

	// Verify default group info values are applied
	expectedName := "Group_99999"
	if chat.Name != expectedName {
		t.Errorf("Expected default name '%s', got '%s'", expectedName, chat.Name)
	}

	if chat.Type != ChatTypeText {
		t.Errorf("Expected default type ChatTypeText, got %v", chat.Type)
	}

	if chat.Privacy != PrivacyPrivate {
		t.Errorf("Expected default privacy PrivacyPrivate, got %v", chat.Privacy)
	}
}

// TestJoinConcurrency tests that Join can be called concurrently safely
func TestJoinConcurrency(t *testing.T) {
	// Capture and suppress log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(log.Writer())

	const goroutines = 10
	results := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			chatID := uint32(1000 + id)
			password := "test-password"

			chat, err := Join(chatID, password)
			if err != nil {
				results <- err
				return
			}

			if chat == nil {
				results <- err
				return
			}

			if chat.ID != chatID {
				results <- err
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < goroutines; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent Join failed: %v", err)
		}
	}
}

// TestJoinDifferentGroupIDs tests joining multiple different groups
func TestJoinDifferentGroupIDs(t *testing.T) {
	// Capture and suppress log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(log.Writer())

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
		if err != nil {
			t.Errorf("Join failed for group ID %d: %v", tc.chatID, err)
			continue
		}

		if chat.ID != tc.chatID {
			t.Errorf("Expected chat ID %d, got %d", tc.chatID, chat.ID)
		}

		// Each join should create a unique self peer ID
		if chat.SelfPeerID == 0 {
			t.Errorf("Group %d has zero self peer ID", tc.chatID)
		}
	}
}

// TestJoinPeerIDUniqueness tests that each join generates a unique peer ID
func TestJoinPeerIDUniqueness(t *testing.T) {
	// Capture and suppress log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(log.Writer())

	const iterations = 100
	peerIDs := make(map[uint32]bool)

	for i := 0; i < iterations; i++ {
		chat, err := Join(uint32(1000+i), "password")
		if err != nil {
			t.Fatalf("Join failed at iteration %d: %v", i, err)
		}

		if peerIDs[chat.SelfPeerID] {
			t.Errorf("Duplicate peer ID %d generated at iteration %d", chat.SelfPeerID, i)
		}

		peerIDs[chat.SelfPeerID] = true
	}

	if len(peerIDs) != iterations {
		t.Errorf("Expected %d unique peer IDs, got %d", iterations, len(peerIDs))
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
