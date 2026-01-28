package group

import (
	"testing"

	"github.com/opd-ai/toxcore/dht"
)

// TestBroadcastWithAllPeersOffline verifies that broadcasting when all peers are offline
// returns an appropriate error rather than silently succeeding
func TestBroadcastWithAllPeersOffline(t *testing.T) {
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

	// Add self as a peer (should be skipped in broadcast)
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Self",
		Connection: 1, // Online
		PublicKey:  [32]byte{1, 2, 3},
	}

	// Add multiple offline peers
	chat.Peers[2] = &Peer{
		ID:         2,
		Name:       "Peer2",
		Connection: 0, // Offline
		PublicKey:  [32]byte{2, 3, 4},
	}

	chat.Peers[3] = &Peer{
		ID:         3,
		Name:       "Peer3",
		Connection: 0, // Offline
		PublicKey:  [32]byte{3, 4, 5},
	}

	chat.Peers[4] = &Peer{
		ID:         4,
		Name:       "Peer4",
		Connection: 0, // Offline
		PublicKey:  [32]byte{4, 5, 6},
	}

	// Attempt to broadcast a message - all peers are offline except self
	err := chat.SendMessage("test message")

	// The issue: this currently returns nil (success) even though no peers received the message
	// Expected behavior: should return an error indicating no peers are available
	if err != nil {
		t.Logf("Broadcast correctly returned error: %v", err)
	} else {
		t.Error("Expected error when all peers are offline, got nil (success)")
	}

	// Verify no send calls were made (all peers were offline)
	sendCalls := mockTrans.getSendCalls()
	if len(sendCalls) > 0 {
		t.Errorf("Expected no send calls for offline peers, got %d calls", len(sendCalls))
	}
}

// TestBroadcastWithNoPeers verifies broadcasting to a group with only self returns error
func TestBroadcastWithNoPeers(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		Name:       "EmptyGroup",
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Only self in the group
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Self",
		Connection: 1,
		PublicKey:  [32]byte{1, 2, 3},
	}

	err := chat.SendMessage("test message")

	// Should return error - no other peers to send to
	if err != nil {
		t.Logf("Broadcast correctly returned error for empty group: %v", err)
	} else {
		t.Error("Expected error when group has no other peers, got nil")
	}
}

// TestBroadcastWithMixedOnlineOfflinePeers verifies partial success handling
func TestBroadcastWithMixedOnlineOfflinePeers(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	peerAddr := &mockAddr{address: "127.0.0.1:5000"}

	chat := &Chat{
		ID:         1,
		Name:       "MixedGroup",
		Peers:      make(map[uint32]*Peer),
		SelfPeerID: 1,
		transport:  mockTrans,
		dht:        testDHT,
	}

	// Self
	chat.Peers[1] = &Peer{
		ID:         1,
		Name:       "Self",
		Connection: 1,
		PublicKey:  [32]byte{1, 2, 3},
	}

	// Online peer with address
	chat.Peers[2] = &Peer{
		ID:         2,
		Name:       "OnlinePeer",
		Connection: 1, // Online
		PublicKey:  [32]byte{2, 3, 4},
		Address:    peerAddr,
	}

	// Offline peers
	chat.Peers[3] = &Peer{
		ID:         3,
		Name:       "OfflinePeer1",
		Connection: 0, // Offline
		PublicKey:  [32]byte{3, 4, 5},
	}

	chat.Peers[4] = &Peer{
		ID:         4,
		Name:       "OfflinePeer2",
		Connection: 0, // Offline
		PublicKey:  [32]byte{4, 5, 6},
	}

	err := chat.SendMessage("test message")

	// Should succeed because at least one peer is online
	if err != nil {
		t.Errorf("Expected success with at least one online peer, got error: %v", err)
	}

	// Verify send was called only once (for the online peer)
	sendCalls := mockTrans.getSendCalls()
	if len(sendCalls) != 1 {
		t.Errorf("Expected 1 send call (online peer only), got %d calls", len(sendCalls))
	}
}

// mockAddr implements net.Addr for testing
type mockAddr struct {
	address string
}

func (m *mockAddr) Network() string {
	return "udp"
}

func (m *mockAddr) String() string {
	return m.address
}
