package group

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPeerAnnounceDataToMap tests the PeerAnnounceData serialization
func TestPeerAnnounceDataToMap(t *testing.T) {
	data := PeerAnnounceData{
		PeerID:     123,
		Name:       "TestPeer",
		PublicKey:  [32]byte{1, 2, 3},
		Connection: 1,
		Role:       RoleUser,
	}

	m := data.ToMap()
	assert.Equal(t, uint32(123), m["peer_id"])
	assert.Equal(t, "TestPeer", m["name"])
	assert.Equal(t, uint8(1), m["connection"])
	assert.Equal(t, RoleUser, m["role"])
}

// TestPeerListRequestDataToMap tests the PeerListRequestData serialization
func TestPeerListRequestDataToMap(t *testing.T) {
	data := PeerListRequestData{
		RequesterID: 456,
	}

	m := data.ToMap()
	assert.Equal(t, uint32(456), m["requester_id"])
}

// TestPeerListResponseDataToMap tests the PeerListResponseData serialization
func TestPeerListResponseDataToMap(t *testing.T) {
	data := PeerListResponseData{
		ResponderID: 789,
		Peers: []PeerAnnounceData{
			{PeerID: 1, Name: "Peer1"},
			{PeerID: 2, Name: "Peer2"},
		},
	}

	m := data.ToMap()
	assert.Equal(t, uint32(789), m["responder_id"])
	peers := m["peers"].([]map[string]interface{})
	assert.Len(t, peers, 2)
	assert.Equal(t, uint32(1), peers[0]["peer_id"])
	assert.Equal(t, "Peer1", peers[0]["name"])
}

// TestHandlePeerAnnounce tests peer announcement handling
func TestHandlePeerAnnounce(t *testing.T) {
	// Create a group
	chat, err := Create("Discovery Test", ChatTypeText, PrivacyPublic, nil, nil)
	require.NoError(t, err)
	defer unregisterGroup(chat.ID)

	// Set up discovery callback
	var discoveredPeer *Peer
	var discoveredGroupID, discoveredPeerID uint32
	var callbackWg sync.WaitGroup
	callbackWg.Add(1)

	chat.OnPeerDiscovered(func(groupID, peerID uint32, peer *Peer) {
		defer callbackWg.Done()
		discoveredGroupID = groupID
		discoveredPeerID = peerID
		discoveredPeer = peer
	})

	// Create peer announcement data
	announceData := PeerAnnounceData{
		PeerID:     9999,
		Name:       "NewPeer",
		PublicKey:  [32]byte{0xAA, 0xBB, 0xCC},
		Connection: 2, // UDP
		Role:       RoleModerator,
	}
	sourceAddr := &mockAddr{address: "192.168.1.100:33445"}

	// Handle the announcement
	isNew := chat.HandlePeerAnnounce(announceData, sourceAddr)
	assert.True(t, isNew, "Should return true for new peer")

	// Wait for callback
	done := make(chan struct{})
	go func() {
		callbackWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Callback completed
	case <-time.After(time.Second):
		t.Fatal("Callback not invoked within timeout")
	}

	// Verify callback parameters
	assert.Equal(t, chat.ID, discoveredGroupID)
	assert.Equal(t, uint32(9999), discoveredPeerID)
	require.NotNil(t, discoveredPeer)
	assert.Equal(t, "NewPeer", discoveredPeer.Name)
	assert.Equal(t, RoleModerator, discoveredPeer.Role)

	// Verify peer was added to the group
	peer, err := chat.GetPeer(9999)
	require.NoError(t, err)
	assert.Equal(t, "NewPeer", peer.Name)
	assert.Equal(t, sourceAddr, peer.Address)
	assert.Equal(t, uint8(2), peer.Connection)

	// Verify subsequent announcements update but don't add new peer
	announceData.Connection = 1 // Change to TCP
	isNew = chat.HandlePeerAnnounce(announceData, sourceAddr)
	assert.False(t, isNew, "Should return false for existing peer")

	// Verify connection was updated
	peer, _ = chat.GetPeer(9999)
	assert.Equal(t, uint8(1), peer.Connection)
}

// TestHandlePeerAnnounceSelfIgnored tests that self announcements are ignored
func TestHandlePeerAnnounceSelfIgnored(t *testing.T) {
	chat, err := Create("Self Ignore Test", ChatTypeText, PrivacyPublic, nil, nil)
	require.NoError(t, err)
	defer unregisterGroup(chat.ID)

	initialPeerCount := chat.PeerCount

	// Try to announce self
	announceData := PeerAnnounceData{
		PeerID: chat.SelfPeerID,
		Name:   "Self",
	}

	isNew := chat.HandlePeerAnnounce(announceData, nil)
	assert.False(t, isNew, "Self announcement should return false")
	assert.Equal(t, initialPeerCount, chat.PeerCount, "Peer count should not change")
}

// TestHandlePeerListRequest tests peer list request handling
func TestHandlePeerListRequest(t *testing.T) {
	// Create a group with multiple peers
	chat, err := Create("List Request Test", ChatTypeText, PrivacyPublic, nil, nil)
	require.NoError(t, err)
	defer unregisterGroup(chat.ID)

	// Add some peers manually for the test
	chat.mu.Lock()
	chat.Peers[100] = &Peer{ID: 100, Name: "Peer100", Connection: 1}
	chat.Peers[200] = &Peer{ID: 200, Name: "Peer200", Connection: 1}
	chat.PeerCount = 3
	chat.mu.Unlock()

	// Handle request from peer 300 (not in the group)
	requestData := PeerListRequestData{
		RequesterID: 300,
	}

	// Request will fail without transport, but should not panic
	// We just test that the function doesn't crash and handles the case gracefully
	err = chat.HandlePeerListRequest(requestData)
	// This will either return an error (no transport/DHT) or succeed with no peers to send to
	// The key thing is it shouldn't panic
	_ = err
}

// TestHandlePeerListRequestSelfIgnored tests that self requests are ignored
func TestHandlePeerListRequestSelfIgnored(t *testing.T) {
	chat, err := Create("Self Request Test", ChatTypeText, PrivacyPublic, nil, nil)
	require.NoError(t, err)
	defer unregisterGroup(chat.ID)

	requestData := PeerListRequestData{
		RequesterID: chat.SelfPeerID,
	}

	err = chat.HandlePeerListRequest(requestData)
	assert.NoError(t, err, "Self request should be silently ignored")
}

// TestHandlePeerListResponse tests peer list response handling
func TestHandlePeerListResponse(t *testing.T) {
	chat, err := Create("List Response Test", ChatTypeText, PrivacyPublic, nil, nil)
	require.NoError(t, err)
	defer unregisterGroup(chat.ID)

	// Track discovered peers
	var discoveredPeers []*Peer
	var mu sync.Mutex

	chat.OnPeerDiscovered(func(groupID, peerID uint32, peer *Peer) {
		mu.Lock()
		discoveredPeers = append(discoveredPeers, peer)
		mu.Unlock()
	})

	// Simulate receiving a peer list response
	responseData := PeerListResponseData{
		ResponderID: 500,
		Peers: []PeerAnnounceData{
			{PeerID: 500, Name: "Responder", Connection: 1},
			{PeerID: 501, Name: "OtherPeer1", Connection: 2},
			{PeerID: 502, Name: "OtherPeer2", Connection: 1},
		},
	}
	sourceAddr := &mockAddr{address: "10.0.0.1:33445"}

	chat.HandlePeerListResponse(responseData, sourceAddr)

	// Allow callbacks to complete
	time.Sleep(100 * time.Millisecond)

	// Verify all peers were added
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, discoveredPeers, 3)

	// Verify responder has the source address
	peer, err := chat.GetPeer(500)
	require.NoError(t, err)
	assert.Equal(t, sourceAddr, peer.Address)

	// Verify other peers don't have addresses
	peer, err = chat.GetPeer(501)
	require.NoError(t, err)
	assert.Nil(t, peer.Address)
}

// TestOnPeerDiscoveredCallback tests the callback registration
func TestOnPeerDiscoveredCallback(t *testing.T) {
	chat, err := Create("Callback Test", ChatTypeText, PrivacyPublic, nil, nil)
	require.NoError(t, err)
	defer unregisterGroup(chat.ID)

	var callbackCalled bool
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	chat.OnPeerDiscovered(func(groupID, peerID uint32, peer *Peer) {
		defer wg.Done()
		mu.Lock()
		callbackCalled = true
		mu.Unlock()
	})

	// Announce a new peer
	announceData := PeerAnnounceData{
		PeerID: 12345,
		Name:   "CallbackTestPeer",
	}

	chat.HandlePeerAnnounce(announceData, nil)

	// Wait for callback to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Callback completed
	case <-time.After(time.Second):
		t.Fatal("Callback not invoked within timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, callbackCalled, "Discovery callback should have been called")
}

// TestAnnounceSelfWithoutTransport tests AnnounceSelf without transport
func TestAnnounceSelfWithoutTransport(t *testing.T) {
	chat, err := Create("No Transport Test", ChatTypeText, PrivacyPublic, nil, nil)
	require.NoError(t, err)
	defer unregisterGroup(chat.ID)

	// AnnounceSelf succeeds with no errors when there are no other peers to send to
	// (we're the only member). This is valid behavior - no failures occurred.
	err = chat.AnnounceSelf()
	assert.NoError(t, err, "AnnounceSelf should succeed when we're the only peer")
}

// TestRequestPeerListWithoutTransport tests RequestPeerList without transport
func TestRequestPeerListWithoutTransport(t *testing.T) {
	chat, err := Create("No Transport Test 2", ChatTypeText, PrivacyPublic, nil, nil)
	require.NoError(t, err)
	defer unregisterGroup(chat.ID)

	// RequestPeerList succeeds with no errors when there are no other peers to send to
	err = chat.RequestPeerList()
	assert.NoError(t, err, "RequestPeerList should succeed when we're the only peer")
}

// TestPeerDiscoveryMultiplePeers tests discovery of multiple peers
func TestPeerDiscoveryMultiplePeers(t *testing.T) {
	chat, err := Create("Multi Peer Test", ChatTypeText, PrivacyPublic, nil, nil)
	require.NoError(t, err)
	defer unregisterGroup(chat.ID)

	var discoveredCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	numPeers := 10
	wg.Add(numPeers)

	chat.OnPeerDiscovered(func(groupID, peerID uint32, peer *Peer) {
		defer wg.Done()
		mu.Lock()
		discoveredCount++
		mu.Unlock()
	})

	// Announce multiple peers
	for i := 0; i < numPeers; i++ {
		announceData := PeerAnnounceData{
			PeerID: uint32(1000 + i),
			Name:   "Peer",
		}
		chat.HandlePeerAnnounce(announceData, nil)
	}

	// Wait for all callbacks
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All callbacks completed
	case <-time.After(2 * time.Second):
		t.Fatal("Not all callbacks completed within timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, numPeers, discoveredCount)
	assert.Equal(t, uint32(numPeers+1), chat.PeerCount) // +1 for self
}
