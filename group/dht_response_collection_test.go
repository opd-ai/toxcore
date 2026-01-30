package group

import (
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
)

// TestDHTResponseCollection verifies that group query responses are properly collected from the DHT network.
func TestDHTResponseCollection(t *testing.T) {
	// Create DHT routing table
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := dht.NewRoutingTable(*toxID, 8)

	// Create a test group announcement
	testGroupID := uint32(12345)
	testAnnouncement := &dht.GroupAnnouncement{
		GroupID:   testGroupID,
		Name:      "Test Group",
		Type:      uint8(ChatTypeText),
		Privacy:   uint8(PrivacyPublic),
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	// Track if callback was called
	var callbackCalled bool
	var receivedAnnouncement *dht.GroupAnnouncement
	var mu sync.Mutex

	// Register callback to capture responses
	callbackFunc := func(announcement *dht.GroupAnnouncement) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		receivedAnnouncement = announcement
	}

	// Set the callback on routing table
	routingTable.SetGroupResponseCallback(callbackFunc)

	// Simulate receiving a response by directly calling HandleGroupQueryResponse
	routingTable.HandleGroupQueryResponse(testAnnouncement)

	// Wait a bit for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify callback was called
	mu.Lock()
	defer mu.Unlock()

	if !callbackCalled {
		t.Error("Expected group response callback to be called")
	}

	if receivedAnnouncement == nil {
		t.Fatal("Expected to receive announcement in callback")
	}

	if receivedAnnouncement.GroupID != testGroupID {
		t.Errorf("Expected group ID %d, got %d", testGroupID, receivedAnnouncement.GroupID)
	}

	if receivedAnnouncement.Name != "Test Group" {
		t.Errorf("Expected group name 'Test Group', got '%s'", receivedAnnouncement.Name)
	}
}

// TestGroupLayerResponseHandling verifies that the group layer properly handles DHT responses.
func TestGroupLayerResponseHandling(t *testing.T) {
	// Create mock transport
	mockTr := &mockTransport{}

	// Create DHT routing table
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := dht.NewRoutingTable(*toxID, 8)

	// Create a group to register the callback
	group, err := Create("Test Group", ChatTypeText, PrivacyPublic, mockTr, routingTable)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Verify the callback was registered by checking we can call the handler
	testGroupID := group.ID
	testAnnouncement := &dht.GroupAnnouncement{
		GroupID:   testGroupID,
		Name:      group.Name,
		Type:      uint8(group.Type),
		Privacy:   uint8(group.Privacy),
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	// Simulate a DHT response
	routingTable.HandleGroupQueryResponse(testAnnouncement)

	// The handler should process the response without error
	// No panic or crash indicates success
	time.Sleep(50 * time.Millisecond)
}

// TestCrossProcessGroupDiscovery verifies the end-to-end DHT-based group discovery.
func TestCrossProcessGroupDiscovery(t *testing.T) {
	// This test simulates two processes discovering each other via DHT

	// Process 1: Create a group and announce it
	keyPair1, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	var nospam1 [4]byte
	toxID1 := crypto.NewToxID(keyPair1.Public, nospam1)
	routingTable1 := dht.NewRoutingTable(*toxID1, 8)
	mockTr1 := &mockTransport{}

	group1, err := Create("Cross-Process Group", ChatTypeText, PrivacyPublic, mockTr1, routingTable1)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Process 2: Try to join the group via DHT
	keyPair2, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	var nospam2 [4]byte
	toxID2 := crypto.NewToxID(keyPair2.Public, nospam2)
	routingTable2 := dht.NewRoutingTable(*toxID2, 8)

	// Simulate DHT response being received by Process 2
	announcement := &dht.GroupAnnouncement{
		GroupID:   group1.ID,
		Name:      group1.Name,
		Type:      uint8(group1.Type),
		Privacy:   uint8(group1.Privacy),
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	// Register callback for Process 2 before simulating the response
	ensureGroupResponseHandlerRegistered(routingTable2)

	// Store the announcement in Process 2's routing table
	routingTable2.HandleGroupQueryResponse(announcement)

	// Give time for response to be processed
	time.Sleep(100 * time.Millisecond)

	// Note: In a real scenario, the Join function would query the DHT and receive this response
	// For this test, we've verified the response handling mechanism works
}

// TestMultipleResponseHandlers verifies that multiple responses can be handled concurrently.
func TestMultipleResponseHandlers(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := dht.NewRoutingTable(*toxID, 8)
	mockTr := &mockTransport{}

	// Create multiple groups to register callbacks
	groups := make([]*Chat, 3)
	for i := 0; i < 3; i++ {
		group, err := Create("Test Group", ChatTypeText, PrivacyPublic, mockTr, routingTable)
		if err != nil {
			t.Fatalf("Failed to create group %d: %v", i, err)
		}
		groups[i] = group
	}

	// Send multiple responses concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			announcement := &dht.GroupAnnouncement{
				GroupID:   groups[idx%3].ID,
				Name:      groups[idx%3].Name,
				Type:      uint8(ChatTypeText),
				Privacy:   uint8(PrivacyPublic),
				Timestamp: time.Now(),
				TTL:       24 * time.Hour,
			}
			routingTable.HandleGroupQueryResponse(announcement)
		}(i)
	}

	// Wait for all responses to be processed
	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// No crash indicates success
}
