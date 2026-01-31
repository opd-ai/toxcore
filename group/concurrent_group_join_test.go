package group

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
)

// TestConcurrentGroupJoinFiltering verifies that concurrent Join() calls receive correct group-specific responses.
// This test addresses Gap #4 from AUDIT.md: ensuring response filtering by group ID.
func TestConcurrentGroupJoinFiltering(t *testing.T) {
	// Create DHT routing table
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := dht.NewRoutingTable(*toxID, 8)

	// Register the response handler
	ensureGroupResponseHandlerRegistered(routingTable)

	// Create two different group announcements
	group1ID := uint32(1001)
	group2ID := uint32(2002)

	announcement1 := &dht.GroupAnnouncement{
		GroupID:   group1ID,
		Name:      "Group One",
		Type:      uint8(ChatTypeText),
		Privacy:   uint8(PrivacyPublic),
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	announcement2 := &dht.GroupAnnouncement{
		GroupID:   group2ID,
		Name:      "Group Two",
		Type:      uint8(ChatTypeText),
		Privacy:   uint8(PrivacyPublic),
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	// Test channels to verify correct responses
	var wg sync.WaitGroup
	results := make(map[uint32]string)
	var resultsMu sync.Mutex

	// Register handlers for both groups
	respChan1 := make(chan *GroupInfo, 1)
	handlerID1 := registerGroupResponseHandler(group1ID, respChan1)
	defer unregisterGroupResponseHandler(handlerID1)

	respChan2 := make(chan *GroupInfo, 1)
	handlerID2 := registerGroupResponseHandler(group2ID, respChan2)
	defer unregisterGroupResponseHandler(handlerID2)

	// Start goroutines to wait for responses
	wg.Add(2)

	// Group 1 waiter
	go func() {
		defer wg.Done()
		select {
		case info := <-respChan1:
			resultsMu.Lock()
			results[group1ID] = info.Name
			resultsMu.Unlock()
		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for group 1 response")
		}
	}()

	// Group 2 waiter
	go func() {
		defer wg.Done()
		select {
		case info := <-respChan2:
			resultsMu.Lock()
			results[group2ID] = info.Name
			resultsMu.Unlock()
		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for group 2 response")
		}
	}()

	// Allow goroutines to start waiting
	time.Sleep(100 * time.Millisecond)

	// Send responses in order (simulating DHT responses arriving)
	routingTable.HandleGroupQueryResponse(announcement1)
	time.Sleep(50 * time.Millisecond)
	routingTable.HandleGroupQueryResponse(announcement2)

	// Wait for all handlers to complete
	wg.Wait()

	// Verify each handler received the CORRECT group's response
	resultsMu.Lock()
	defer resultsMu.Unlock()

	if name, ok := results[group1ID]; !ok {
		t.Error("Group 1 handler did not receive a response")
	} else if name != "Group One" {
		t.Errorf("Group 1 handler received wrong response: got %q, want %q", name, "Group One")
	}

	if name, ok := results[group2ID]; !ok {
		t.Error("Group 2 handler did not receive a response")
	} else if name != "Group Two" {
		t.Errorf("Group 2 handler received wrong response: got %q, want %q", name, "Group Two")
	}
}

// TestResponseFilteringPreventsCrossTalk verifies that handlers don't receive responses for other groups.
func TestResponseFilteringPreventsCrossTalk(t *testing.T) {
	// Create DHT routing table
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	var nospam [4]byte
	toxID := crypto.NewToxID(keyPair.Public, nospam)
	routingTable := dht.NewRoutingTable(*toxID, 8)

	ensureGroupResponseHandlerRegistered(routingTable)

	// Create handler for group A
	groupA := uint32(9001)
	groupB := uint32(9002)

	respChanA := make(chan *GroupInfo, 10) // Buffered to catch any incorrect sends
	handlerIDA := registerGroupResponseHandler(groupA, respChanA)
	defer unregisterGroupResponseHandler(handlerIDA)

	// Send responses for group B only
	announcementB := &dht.GroupAnnouncement{
		GroupID:   groupB,
		Name:      "Group B",
		Type:      uint8(ChatTypeText),
		Privacy:   uint8(PrivacyPublic),
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	// Send multiple B responses
	for i := 0; i < 5; i++ {
		routingTable.HandleGroupQueryResponse(announcementB)
		time.Sleep(20 * time.Millisecond)
	}

	// Verify handler A received NOTHING (since we only sent B responses)
	select {
	case info := <-respChanA:
		t.Errorf("Handler A (waiting for group %d) incorrectly received response for group %d: %q",
			groupA, groupB, info.Name)
	case <-time.After(300 * time.Millisecond):
		// Expected: timeout means no cross-talk occurred
	}
}

// TestMultipleHandlersSameGroup verifies multiple handlers for the same group all receive the response.
func TestMultipleHandlersSameGroup(t *testing.T) {
	groupID := uint32(5000)
	numHandlers := 5

	// Register multiple handlers for the same group
	channels := make([]chan *GroupInfo, numHandlers)
	handlerIDs := make([]string, numHandlers)

	for i := 0; i < numHandlers; i++ {
		channels[i] = make(chan *GroupInfo, 1)
		handlerIDs[i] = registerGroupResponseHandler(groupID, channels[i])
		defer unregisterGroupResponseHandler(handlerIDs[i])
	}

	// Send one response directly to HandleGroupQueryResponse
	announcement := &dht.GroupAnnouncement{
		GroupID:   groupID,
		Name:      "Shared Group",
		Type:      uint8(ChatTypeText),
		Privacy:   uint8(PrivacyPublic),
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	HandleGroupQueryResponse(announcement)

	// Verify ALL handlers received the response
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < numHandlers; i++ {
		select {
		case info := <-channels[i]:
			if info.Name != "Shared Group" {
				t.Errorf("Handler %d received wrong group name: got %q, want %q",
					i, info.Name, "Shared Group")
			}
		default:
			t.Errorf("Handler %d did not receive the response", i)
		}
	}
}

// TestRapidConcurrentJoins stress tests the filtering logic with many concurrent joins.
func TestRapidConcurrentJoins(t *testing.T) {
	numGroups := 20
	var wg sync.WaitGroup
	errors := make(chan error, numGroups)

	// Launch concurrent handlers for different groups
	for i := 0; i < numGroups; i++ {
		wg.Add(1)
		go func(groupNum int) {
			defer wg.Done()

			groupID := uint32(10000 + groupNum)
			expectedName := fmt.Sprintf("Group %d", groupNum)

			respChan := make(chan *GroupInfo, 1)
			handlerID := registerGroupResponseHandler(groupID, respChan)
			defer unregisterGroupResponseHandler(handlerID)

			// Send the response for THIS group directly
			announcement := &dht.GroupAnnouncement{
				GroupID:   groupID,
				Name:      expectedName,
				Type:      uint8(ChatTypeText),
				Privacy:   uint8(PrivacyPublic),
				Timestamp: time.Now(),
				TTL:       24 * time.Hour,
			}
			HandleGroupQueryResponse(announcement)

			// Verify we get the correct response
			select {
			case info := <-respChan:
				if info.Name != expectedName {
					errors <- fmt.Errorf("group %d: got name %q, want %q",
						groupID, info.Name, expectedName)
				}
			case <-time.After(2 * time.Second):
				errors <- fmt.Errorf("group %d: timeout waiting for response", groupID)
			}
		}(i)

		// Stagger the goroutine starts slightly
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	var foundErrors []error
	for err := range errors {
		foundErrors = append(foundErrors, err)
	}

	if len(foundErrors) > 0 {
		t.Errorf("Found %d errors during concurrent joins:", len(foundErrors))
		for _, err := range foundErrors {
			t.Errorf("  - %v", err)
		}
	}
}
