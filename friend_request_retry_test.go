package toxcore

import (
	"testing"
	"time"
)

// TestFriendRequestRetryQueue verifies that friend requests are properly queued for retry
// when DHT nodes are not available
func TestFriendRequestRetryQueue(t *testing.T) {
	// Create Tox instance with minimal bootstrap to simulate sparse DHT
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create a target public key for the friend request
	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 1)
	}

	// Send friend request without any DHT nodes available
	message := "Hello, friend!"
	err = tox.sendFriendRequest(targetPK, message)
	if err != nil {
		t.Fatalf("sendFriendRequest failed: %v", err)
	}

	// Verify the request was queued
	tox.pendingFriendReqsMux.Lock()
	if len(tox.pendingFriendReqs) != 1 {
		t.Fatalf("Expected 1 pending request, got %d", len(tox.pendingFriendReqs))
	}

	req := tox.pendingFriendReqs[0]
	if req.targetPublicKey != targetPK {
		t.Error("Target public key mismatch in pending request")
	}
	if req.message != message {
		t.Errorf("Message mismatch: got %q, want %q", req.message, message)
	}
	if req.retryCount != 0 {
		t.Errorf("Initial retry count should be 0, got %d", req.retryCount)
	}
	tox.pendingFriendReqsMux.Unlock()

	t.Log("SUCCESS: Friend request properly queued for retry")
}

// TestFriendRequestRetryBackoff verifies exponential backoff for retries
func TestFriendRequestRetryBackoff(t *testing.T) {
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 100)
	}

	// Queue a friend request
	err = tox.sendFriendRequest(targetPK, "Test retry backoff")
	if err != nil {
		t.Fatalf("sendFriendRequest failed: %v", err)
	}

	// Wait for initial retry time to pass (5 seconds + 1 second buffer)
	time.Sleep(6 * time.Second)

	// Run iteration to trigger retry
	tox.Iterate()

	// Check that retry count increased and backoff applied
	tox.pendingFriendReqsMux.Lock()
	if len(tox.pendingFriendReqs) == 0 {
		t.Fatal("Request should still be pending after failed retry")
	}

	req := tox.pendingFriendReqs[0]
	if req.retryCount != 1 {
		t.Errorf("Retry count should be 1, got %d", req.retryCount)
	}

	// Verify exponential backoff (should be ~10 seconds for second retry)
	expectedBackoff := 10 * time.Second
	actualBackoff := req.nextRetry.Sub(time.Now())
	if actualBackoff < expectedBackoff-time.Second || actualBackoff > expectedBackoff+time.Second {
		t.Errorf("Backoff not exponential: expected ~%v, got ~%v", expectedBackoff, actualBackoff)
	}
	tox.pendingFriendReqsMux.Unlock()

	t.Log("SUCCESS: Exponential backoff working correctly")
}

// TestFriendRequestMaxRetries verifies that requests are dropped after max retries
func TestFriendRequestMaxRetries(t *testing.T) {
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 200)
	}

	// Queue a friend request
	err = tox.sendFriendRequest(targetPK, "Test max retries")
	if err != nil {
		t.Fatalf("sendFriendRequest failed: %v", err)
	}

	// Simulate 10 failed retries by manually incrementing retry count
	tox.pendingFriendReqsMux.Lock()
	tox.pendingFriendReqs[0].retryCount = 9
	tox.pendingFriendReqs[0].nextRetry = time.Now() // Make it ready for immediate retry
	tox.pendingFriendReqsMux.Unlock()

	// Run iteration to trigger final retry and removal
	tox.Iterate()

	// Verify request was removed after max retries
	tox.pendingFriendReqsMux.Lock()
	pendingCount := len(tox.pendingFriendReqs)
	tox.pendingFriendReqsMux.Unlock()

	if pendingCount != 0 {
		t.Errorf("Request should be removed after max retries, but %d pending", pendingCount)
	}

	t.Log("SUCCESS: Requests properly dropped after maximum retries")
}

// TestFriendRequestDuplicatePrevention verifies that duplicate requests update existing entry
func TestFriendRequestDuplicatePrevention(t *testing.T) {
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 50)
	}

	// Send first friend request
	message1 := "First message"
	err = tox.sendFriendRequest(targetPK, message1)
	if err != nil {
		t.Fatalf("First sendFriendRequest failed: %v", err)
	}

	// Send second friend request to same target
	message2 := "Updated message"
	err = tox.sendFriendRequest(targetPK, message2)
	if err != nil {
		t.Fatalf("Second sendFriendRequest failed: %v", err)
	}

	// Verify only one request exists with updated message
	tox.pendingFriendReqsMux.Lock()
	if len(tox.pendingFriendReqs) != 1 {
		t.Fatalf("Expected 1 pending request, got %d", len(tox.pendingFriendReqs))
	}

	req := tox.pendingFriendReqs[0]
	if req.message != message2 {
		t.Errorf("Message should be updated to %q, got %q", message2, req.message)
	}
	tox.pendingFriendReqsMux.Unlock()

	t.Log("SUCCESS: Duplicate requests properly update existing entry")
}

// TestFriendRequestProductionVsTestPath verifies separation of production and test code paths
func TestFriendRequestProductionVsTestPath(t *testing.T) {
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 150)
	}

	// Send friend request
	err = tox.sendFriendRequest(targetPK, "Test dual path")
	if err != nil {
		t.Fatalf("sendFriendRequest failed: %v", err)
	}

	// Verify request is in both production queue AND test registry
	tox.pendingFriendReqsMux.Lock()
	productionQueued := len(tox.pendingFriendReqs) > 0
	tox.pendingFriendReqsMux.Unlock()

	globalFriendRequestRegistry.RLock()
	testRegistered := globalFriendRequestRegistry.requests[targetPK] != nil
	globalFriendRequestRegistry.RUnlock()

	if !productionQueued {
		t.Error("Request should be in production retry queue")
	}

	if !testRegistered {
		t.Error("Request should be in test registry for backward compatibility")
	}

	t.Log("SUCCESS: Request properly exists in both production queue and test registry")
}
