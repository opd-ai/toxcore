package toxcore

import (
	"sync"
	"testing"
	"time"
)

// TestSimpleFriendMessageCallback tests the simplified callback API that matches the README documentation
func TestSimpleFriendMessageCallback(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test data
	testFriendID := uint32(1)
	testMessage := "Hello, world!"
	testMessageType := MessageTypeNormal

	// Add a mock friend
	tox.friends[testFriendID] = &Friend{
		PublicKey:        [32]byte{1, 2, 3},
		Status:           FriendStatusOnline,
		ConnectionStatus: ConnectionUDP,
		LastSeen:         time.Now(),
	}

	// Track callback invocations
	var callbackInvoked bool
	var receivedFriendID uint32
	var receivedMessage string
	var mu sync.Mutex

	// Register the simple callback (matches README.md documentation)
	tox.OnFriendMessage(func(friendID uint32, message string) {
		mu.Lock()
		defer mu.Unlock()
		callbackInvoked = true
		receivedFriendID = friendID
		receivedMessage = message
	})

	// Simulate receiving a message
	tox.receiveFriendMessage(testFriendID, testMessage, testMessageType)

	// Verify callback was called with correct parameters
	mu.Lock()
	if !callbackInvoked {
		t.Error("Simple friend message callback was not invoked")
	}
	if receivedFriendID != testFriendID {
		t.Errorf("Expected friend ID %d, got %d", testFriendID, receivedFriendID)
	}
	if receivedMessage != testMessage {
		t.Errorf("Expected message '%s', got '%s'", testMessage, receivedMessage)
	}
	mu.Unlock()

	t.Log("Simple friend message callback test passed")
}

// TestDetailedFriendMessageCallback tests the detailed callback API for advanced users
func TestDetailedFriendMessageCallback(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test data
	testFriendID := uint32(2)
	testMessage := "This is an action message"
	testMessageType := MessageTypeAction

	// Add a mock friend
	tox.friends[testFriendID] = &Friend{
		PublicKey:        [32]byte{4, 5, 6},
		Status:           FriendStatusOnline,
		ConnectionStatus: ConnectionUDP,
		LastSeen:         time.Now(),
	}

	// Track callback invocations
	var callbackInvoked bool
	var receivedFriendID uint32
	var receivedMessage string
	var receivedMessageType MessageType
	var mu sync.Mutex

	// Register the detailed callback
	tox.OnFriendMessageDetailed(func(friendID uint32, message string, messageType MessageType) {
		mu.Lock()
		defer mu.Unlock()
		callbackInvoked = true
		receivedFriendID = friendID
		receivedMessage = message
		receivedMessageType = messageType
	})

	// Simulate receiving a message
	tox.receiveFriendMessage(testFriendID, testMessage, testMessageType)

	// Verify callback was called with correct parameters
	mu.Lock()
	if !callbackInvoked {
		t.Error("Detailed friend message callback was not invoked")
	}
	if receivedFriendID != testFriendID {
		t.Errorf("Expected friend ID %d, got %d", testFriendID, receivedFriendID)
	}
	if receivedMessage != testMessage {
		t.Errorf("Expected message '%s', got '%s'", testMessage, receivedMessage)
	}
	if receivedMessageType != testMessageType {
		t.Errorf("Expected message type %v, got %v", testMessageType, receivedMessageType)
	}
	mu.Unlock()

	t.Log("Detailed friend message callback test passed")
}

// TestBothMessageCallbacks tests that both simple and detailed callbacks work simultaneously
func TestBothMessageCallbacks(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test data
	testFriendID := uint32(3)
	testMessage := "Test message for both callbacks"
	testMessageType := MessageTypeNormal

	// Add a mock friend
	tox.friends[testFriendID] = &Friend{
		PublicKey:        [32]byte{7, 8, 9},
		Status:           FriendStatusOnline,
		ConnectionStatus: ConnectionUDP,
		LastSeen:         time.Now(),
	}

	// Track callback invocations
	var simpleCallbackInvoked bool
	var detailedCallbackInvoked bool
	var mu sync.Mutex

	// Register both callbacks
	tox.OnFriendMessage(func(friendID uint32, message string) {
		mu.Lock()
		defer mu.Unlock()
		simpleCallbackInvoked = true
	})

	tox.OnFriendMessageDetailed(func(friendID uint32, message string, messageType MessageType) {
		mu.Lock()
		defer mu.Unlock()
		detailedCallbackInvoked = true
	})

	// Simulate receiving a message
	tox.receiveFriendMessage(testFriendID, testMessage, testMessageType)

	// Verify both callbacks were called
	mu.Lock()
	if !simpleCallbackInvoked {
		t.Error("Simple callback was not invoked")
	}
	if !detailedCallbackInvoked {
		t.Error("Detailed callback was not invoked")
	}
	mu.Unlock()

	t.Log("Both message callbacks test passed")
}

// TestAddFriendByPublicKey tests the new method that matches the documented API
func TestAddFriendByPublicKey(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test public key
	testPublicKey := [32]byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41}

	// Add friend by public key (should match documented API)
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend by public key: %v", err)
	}

	// Verify friend was added
	tox.friendsMutex.RLock()
	friend, exists := tox.friends[friendID]
	tox.friendsMutex.RUnlock()

	if !exists {
		t.Error("Friend was not added to friends list")
	}
	if friend.PublicKey != testPublicKey {
		t.Error("Friend public key does not match")
	}

	// Test adding the same friend again (should fail)
	_, err = tox.AddFriendByPublicKey(testPublicKey)
	if err == nil {
		t.Error("Expected error when adding duplicate friend")
	}

	t.Log("AddFriendByPublicKey test passed")
}

// TestDocumentedAPICompatibility tests the exact API usage shown in README.md
func TestDocumentedAPICompatibility(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test the exact callback signature from README.md
	var requestHandled bool
	var mu sync.Mutex

	// This should compile and work (matches README.md example)
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		mu.Lock()
		defer mu.Unlock()

		// This should also work (matches README.md example)
		friendID, err := tox.AddFriendByPublicKey(publicKey)
		if err != nil {
			t.Errorf("Error accepting friend request: %v", err)
		} else {
			t.Logf("Accepted friend request. Friend ID: %d", friendID)
			requestHandled = true
		}
	})

	// This should also compile and work (matches README.md example)
	tox.OnFriendMessage(func(friendID uint32, message string) {
		t.Logf("Message from friend %d: %s", friendID, message)

		// This should work (matches README.md example)
		err := tox.SendFriendMessage(friendID, "You said: "+message)
		if err != nil {
			t.Logf("Error sending response: %v", err)
		}
	})

	// Simulate a friend request to test the flow
	testPublicKey := [32]byte{42}
	if tox.friendRequestCallback != nil {
		tox.friendRequestCallback(testPublicKey, testFriendRequestMessage)
	}

	// Verify the flow worked
	mu.Lock()
	if !requestHandled {
		t.Error("Friend request was not handled correctly")
	}
	mu.Unlock()

	t.Log("Documented API compatibility test passed")
}

// TestMessageFromUnknownFriend tests that messages from unknown friends are ignored
func TestMessageFromUnknownFriend(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Track callback invocations
	var callbackInvoked bool
	var mu sync.Mutex

	// Register callback
	tox.OnFriendMessage(func(friendID uint32, message string) {
		mu.Lock()
		defer mu.Unlock()
		callbackInvoked = true
	})

	// Try to receive message from unknown friend (ID 999)
	tox.receiveFriendMessage(999, "Should be ignored", MessageTypeNormal)

	// Verify callback was NOT called
	mu.Lock()
	if callbackInvoked {
		t.Error("Callback should not be invoked for unknown friend")
	}
	mu.Unlock()

	t.Log("Unknown friend message filtering test passed")
}
