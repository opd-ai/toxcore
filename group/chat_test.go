package group

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// TestJoinValidGroupID tests that joining returns error when DHT lookup is not implemented
func TestJoinValidGroupID(t *testing.T) {
	chatID := uint32(12345)
	password := "test-password"

	chat, err := Join(chatID, password)

	// Join should fail because DHT lookup is not yet implemented
	if err == nil {
		t.Fatal("Expected error when joining group (DHT lookup not implemented)")
	}

	if chat != nil {
		t.Error("Expected nil chat when Join fails")
	}

	// Verify error message indicates DHT lookup failure
	expectedError := "cannot join group"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}

	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected error mentioning 'not yet implemented', got: %v", err)
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

// TestJoinPrivateGroupWithoutPassword tests that joining fails due to unimplemented DHT lookup
func TestJoinPrivateGroupWithoutPassword(t *testing.T) {
	chatID := uint32(54321)
	password := "" // Empty password

	chat, err := Join(chatID, password)

	// Join fails at DHT lookup stage, before password validation
	if err == nil {
		t.Fatal("Expected error when joining group (DHT lookup not implemented)")
	}

	if chat != nil {
		t.Error("Expected nil chat when error occurs")
	}

	// Error should be about DHT lookup, not password
	expectedError := "cannot join group"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

// TestJoinDHTLookupFailure tests that Join returns error when DHT lookup fails
func TestJoinDHTLookupFailure(t *testing.T) {
	chatID := uint32(99999)
	password := "test-password"

	chat, err := Join(chatID, password)

	// Join should fail because DHT lookup is not implemented
	if err == nil {
		t.Fatal("Expected error when DHT lookup fails")
	}

	if chat != nil {
		t.Error("Expected nil chat when DHT lookup fails")
	}

	// Verify error indicates DHT lookup failure
	if !strings.Contains(err.Error(), "cannot join group") {
		t.Errorf("Expected error about joining group, got: %v", err)
	}
}

// TestJoinConcurrency tests that Join fails consistently when called concurrently
func TestJoinConcurrency(t *testing.T) {
	const goroutines = 10
	results := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			chatID := uint32(1000 + id)
			password := "test-password"

			chat, err := Join(chatID, password)
			if err == nil {
				results <- fmt.Errorf("expected error but got nil")
				return
			}

			if chat != nil {
				results <- fmt.Errorf("expected nil chat but got non-nil")
				return
			}

			results <- nil
		}(i)
	}

	// Collect results - all should consistently fail with DHT lookup error
	for i := 0; i < goroutines; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent Join test failed: %v", err)
		}
	}
}

// TestJoinDifferentGroupIDs tests that joining fails for different group IDs
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
		// All joins should fail because DHT lookup is not implemented
		if err == nil {
			t.Errorf("Expected error for group ID %d, but Join succeeded", tc.chatID)
			continue
		}

		if chat != nil {
			t.Errorf("Expected nil chat for group ID %d, got non-nil", tc.chatID)
		}

		if !strings.Contains(err.Error(), "cannot join group") {
			t.Errorf("Expected 'cannot join group' error for group ID %d, got: %v", tc.chatID, err)
		}
	}
}

// TestJoinConsistentFailure tests that Join consistently fails
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

		if !strings.Contains(err.Error(), "cannot join group") {
			t.Errorf("Expected 'cannot join group' error at iteration %d, got: %v", i, err)
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
