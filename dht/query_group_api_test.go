package dht

import (
	"testing"
	"time"

	"github.com/opd-ai/toxcore/async"
	"github.com/opd-ai/toxcore/crypto"
)

// TestQueryGroup_LocalStorageFound verifies QueryGroup returns announcement from local storage.
func TestQueryGroup_LocalStorageFound(t *testing.T) {
	// Create routing table
	keyPair, _ := crypto.GenerateKeyPair()
	selfID := *crypto.NewToxID(keyPair.Public, [4]byte{})
	rt := NewRoutingTable(selfID, 8)

	// Store an announcement in local storage
	announcement := &GroupAnnouncement{
		GroupID:   12345,
		Name:      "Test Group",
		Type:      0,
		Privacy:   0,
		Timestamp: time.Now(),
		TTL:       1 * time.Hour,
	}
	rt.groupStorage.StoreAnnouncement(announcement)

	// Create mock transport
	mockTr := async.NewMockTransport("127.0.0.1:33445")

	// Query should return the local announcement without error
	result, err := rt.QueryGroup(12345, mockTr)
	if err != nil {
		t.Errorf("Expected no error when announcement exists locally, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected announcement to be returned from local storage")
	}
	if result.GroupID != 12345 {
		t.Errorf("Expected GroupID 12345, got %d", result.GroupID)
	}
	if result.Name != "Test Group" {
		t.Errorf("Expected Name 'Test Group', got %s", result.Name)
	}

	// Verify no network packets were sent (local cache hit)
	if len(mockTr.GetPackets()) > 0 {
		t.Errorf("Expected no network packets when using local cache, got %d packets", len(mockTr.GetPackets()))
	}
}

// TestQueryGroup_NetworkQueryInitiated verifies QueryGroup sends network queries when not in local storage.
func TestQueryGroup_NetworkQueryInitiated(t *testing.T) {
	// Create routing table with some nodes
	keyPair, _ := crypto.GenerateKeyPair()
	selfID := *crypto.NewToxID(keyPair.Public, [4]byte{})
	rt := NewRoutingTable(selfID, 8)

	// Add a few nodes to the routing table so network query can be sent
	mockTr := async.NewMockTransport("127.0.0.1:33445")
	for i := 0; i < 3; i++ {
		nodeKeyPair, _ := crypto.GenerateKeyPair()
		node := &Node{
			ID:      *crypto.NewToxID(nodeKeyPair.Public, [4]byte{}),
			Address: mockTr.LocalAddr(),
			Status:  StatusGood,
		}
		rt.AddNode(node)
	}

	// Query for a group that doesn't exist locally
	result, err := rt.QueryGroup(99999, mockTr)

	// Should return error indicating async operation
	if err == nil {
		t.Error("Expected error when group not in local storage")
	}
	if result != nil {
		t.Error("Expected nil result when initiating network query")
	}

	// Verify network packets were sent
	if len(mockTr.GetPackets()) == 0 {
		t.Error("Expected network packets to be sent for DHT query")
	}
}

// TestQueryGroup_NilTransport verifies QueryGroup returns error with nil transport.
func TestQueryGroup_NilTransport(t *testing.T) {
	keyPair, _ := crypto.GenerateKeyPair()
	selfID := *crypto.NewToxID(keyPair.Public, [4]byte{})
	rt := NewRoutingTable(selfID, 8)

	// Query with nil transport should fail immediately
	result, err := rt.QueryGroup(12345, nil)
	if err == nil {
		t.Error("Expected error with nil transport")
	}
	if result != nil {
		t.Error("Expected nil result with nil transport")
	}
}

// TestQueryGroup_ExpiredAnnouncementNotReturned verifies expired announcements are not returned.
func TestQueryGroup_ExpiredAnnouncementNotReturned(t *testing.T) {
	keyPair, _ := crypto.GenerateKeyPair()
	selfID := *crypto.NewToxID(keyPair.Public, [4]byte{})
	rt := NewRoutingTable(selfID, 8)

	// Add nodes for network query
	mockTr := async.NewMockTransport("127.0.0.1:33445")
	nodeKeyPair, _ := crypto.GenerateKeyPair()
	node := &Node{
		ID:      *crypto.NewToxID(nodeKeyPair.Public, [4]byte{}),
		Address: mockTr.LocalAddr(),
		Status:  StatusGood,
	}
	rt.AddNode(node)

	// Store an expired announcement
	expiredAnnouncement := &GroupAnnouncement{
		GroupID:   12345,
		Name:      "Expired Group",
		Type:      0,
		Privacy:   0,
		Timestamp: time.Now().Add(-2 * time.Hour), // 2 hours ago
		TTL:       1 * time.Hour,                  // 1 hour TTL
	}
	rt.groupStorage.StoreAnnouncement(expiredAnnouncement)

	// Query should not return the expired announcement
	result, err := rt.QueryGroup(12345, mockTr)
	if err == nil {
		t.Error("Expected error (network query) when announcement is expired")
	}
	if result != nil {
		t.Error("Expected nil result when announcement is expired")
	}

	// Should have initiated network query instead
	if len(mockTr.GetPackets()) == 0 {
		t.Error("Expected network query to be sent when local announcement is expired")
	}
}

// TestQueryGroup_NoNodesAvailable verifies QueryGroup returns error when no DHT nodes available.
func TestQueryGroup_NoNodesAvailable(t *testing.T) {
	keyPair, _ := crypto.GenerateKeyPair()
	selfID := *crypto.NewToxID(keyPair.Public, [4]byte{})
	rt := NewRoutingTable(selfID, 8)
	// No nodes added to routing table

	mockTr := async.NewMockTransport("127.0.0.1:33445")

	// Query when no nodes available should return specific error
	result, err := rt.QueryGroup(12345, mockTr)
	if err == nil {
		t.Error("Expected error when no DHT nodes available")
	}
	if result != nil {
		t.Error("Expected nil result when no DHT nodes available")
	}

	// Should not have sent any packets
	if len(mockTr.GetPackets()) > 0 {
		t.Errorf("Expected no packets sent when no nodes available, got %d", len(mockTr.GetPackets()))
	}
}
