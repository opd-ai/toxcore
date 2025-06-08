package toxcore

import (
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/friend"
)

func TestCallbackSystem(t *testing.T) {
	// Create two Tox instances
	options1 := NewOptions()
	options1.UDPEnabled = true
	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	options2 := NewOptions()
	options2.UDPEnabled = true
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Test callback registration
	var friendRequestReceived bool
	var friendMessageReceived bool
	var connectionStatusChanged bool
	var mu sync.Mutex

	// Register friend request callback
	tox2.OnFriendRequest(func(publicKey [32]byte, message string) {
		mu.Lock()
		defer mu.Unlock()
		friendRequestReceived = true
		t.Logf("Friend request received: %s", message)
	})

	// Register friend message callback
	tox1.OnFriendMessage(func(friendID uint32, message string, messageType MessageType) {
		mu.Lock()
		defer mu.Unlock()
		friendMessageReceived = true
		t.Logf("Message received from friend %d: %s", friendID, message)
	})

	// Register connection status callback
	tox1.OnConnectionStatus(func(status ConnectionStatus) {
		mu.Lock()
		defer mu.Unlock()
		connectionStatusChanged = true
		t.Logf("Connection status changed to: %v", status)
	})

	// Test manual friend request processing
	mockRequest := &friend.Request{
		SenderPublicKey: tox1.SelfGetPublicKey(),
		Message:         "Test friend request",
		Timestamp:       time.Now(),
		Handled:         false,
	}

	// Add request to tox2's request manager
	tox2.requestManager.AddRequest(mockRequest)

	// Process iterations to trigger callbacks
	for i := 0; i < 10; i++ {
		tox1.Iterate()
		tox2.Iterate()
		time.Sleep(10 * time.Millisecond)
	}

	// Check if callback was triggered
	mu.Lock()
	if !friendRequestReceived {
		t.Error("Friend request callback was not triggered")
	}
	mu.Unlock()

	// Test message sending and callback
	// First add friend to simulate existing friendship
	tox1.friends[0] = &Friend{
		PublicKey:        tox2.SelfGetPublicKey(),
		Status:           FriendStatusOnline,
		ConnectionStatus: ConnectionUDP,
		LastSeen:         time.Now(),
	}

	// Send a message
	err = tox1.SendFriendMessage(0, "Test message")
	if err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	// Process iterations
	for i := 0; i < 10; i++ {
		tox1.Iterate()
		tox2.Iterate()
		time.Sleep(10 * time.Millisecond)
	}

	// Test connection status notification
	tox1.notifyConnectionStatusChange(ConnectionUDP)

	// Check if callback was triggered
	mu.Lock()
	if !connectionStatusChanged {
		t.Error("Connection status callback was not triggered")
	}
	mu.Unlock()

	t.Log("Callback system test completed successfully")
}

func TestMessageQueue(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend
	tox.friends[1] = &Friend{
		PublicKey:        [32]byte{1, 2, 3}, // Mock public key
		Status:           FriendStatusOnline,
		ConnectionStatus: ConnectionUDP,
		LastSeen:         time.Now(),
	}

	// Send multiple messages
	messages := []string{"Hello", "How are you?", "This is a test"}
	for _, msg := range messages {
		err := tox.SendFriendMessage(1, msg)
		if err != nil {
			t.Errorf("Failed to send message '%s': %v", msg, err)
		}
	}

	// Verify messages are queued
	tox.messageQueueMutex.Lock()
	queueLength := len(tox.messageQueue)
	tox.messageQueueMutex.Unlock()

	if queueLength != len(messages) {
		t.Errorf("Expected %d messages in queue, got %d", len(messages), queueLength)
	}

	// Process the queue
	for i := 0; i < 5; i++ {
		tox.Iterate()
		time.Sleep(10 * time.Millisecond)
	}

	// Messages should have been processed (mock implementation succeeds)
	tox.messageQueueMutex.Lock()
	finalQueueLength := len(tox.messageQueue)
	tox.messageQueueMutex.Unlock()

	if finalQueueLength != 0 {
		t.Errorf("Expected empty message queue after processing, got %d messages", finalQueueLength)
	}

	t.Log("Message queue test completed successfully")
}

func TestFriendRequestHandling(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var requestCount int
	var lastMessage string
	var mu sync.Mutex

	// Register callback
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		mu.Lock()
		defer mu.Unlock()
		requestCount++
		lastMessage = message
		t.Logf("Friend request from %x: %s", publicKey[:8], message)
	})

	// Create and add multiple friend requests
	requests := []struct {
		publicKey [32]byte
		message   string
	}{
		{[32]byte{1}, "First request"},
		{[32]byte{2}, "Second request"},
		{[32]byte{3}, "Third request"},
	}

	for _, req := range requests {
		mockRequest := &friend.Request{
			SenderPublicKey: req.publicKey,
			Message:         req.message,
			Timestamp:       time.Now(),
			Handled:         false,
		}
		tox.requestManager.AddRequest(mockRequest)
	}

	// Process iterations to trigger callbacks
	for i := 0; i < 10; i++ {
		tox.Iterate()
		time.Sleep(10 * time.Millisecond)
	}

	// Verify callbacks were triggered
	mu.Lock()
	if requestCount != len(requests) {
		t.Errorf("Expected %d friend request callbacks, got %d", len(requests), requestCount)
	}
	if lastMessage != "Third request" {
		t.Errorf("Expected last message to be 'Third request', got '%s'", lastMessage)
	}
	mu.Unlock()

	t.Log("Friend request handling test completed successfully")
}
