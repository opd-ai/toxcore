package async

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// TestMessageQueueingWithoutPreKeys tests that messages are queued when pre-keys aren't available
func TestMessageQueueingWithoutPreKeys(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "async_queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create sender and recipient key pairs
	senderKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key: %v", err)
	}

	recipientKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key: %v", err)
	}

	// Create mock transport for sender
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Create async manager for sender
	senderManager, err := NewAsyncManager(senderKey, mockTransport, filepath.Join(tempDir, "sender"))
	if err != nil {
		t.Fatalf("Failed to create sender manager: %v", err)
	}

	// Start the async manager
	senderManager.Start()
	defer senderManager.Stop()

	// Set recipient as offline (no pre-keys exchanged yet)
	senderManager.SetFriendOnlineStatus(recipientKey.Public, false)

	// Attempt to send a message without pre-keys - should queue it
	err = senderManager.SendAsyncMessage(recipientKey.Public, "Hello without keys!", MessageTypeNormal)
	if err != nil {
		t.Fatalf("SendAsyncMessage should not return error when queuing: %v", err)
	}

	// Verify message was queued
	senderManager.mutex.RLock()
	queued := senderManager.pendingMessages[recipientKey.Public]
	senderManager.mutex.RUnlock()

	if len(queued) != 1 {
		t.Fatalf("Expected 1 queued message, got %d", len(queued))
	}

	if queued[0].message != "Hello without keys!" {
		t.Errorf("Expected queued message 'Hello without keys!', got '%s'", queued[0].message)
	}

	if queued[0].messageType != MessageTypeNormal {
		t.Errorf("Expected message type %v, got %v", MessageTypeNormal, queued[0].messageType)
	}
}

// TestMultipleMessagesQueueing tests queuing multiple messages for the same recipient
func TestMultipleMessagesQueueing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "async_multi_queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	senderKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key: %v", err)
	}

	recipientKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8081")
	senderManager, err := NewAsyncManager(senderKey, mockTransport, filepath.Join(tempDir, "sender"))
	if err != nil {
		t.Fatalf("Failed to create sender manager: %v", err)
	}

	senderManager.Start()
	defer senderManager.Stop()

	// Send multiple messages without pre-keys
	messages := []string{"Message 1", "Message 2", "Message 3"}
	for _, msg := range messages {
		err = senderManager.SendAsyncMessage(recipientKey.Public, msg, MessageTypeNormal)
		if err != nil {
			t.Fatalf("Failed to queue message '%s': %v", msg, err)
		}
	}

	// Verify all messages were queued
	senderManager.mutex.RLock()
	queued := senderManager.pendingMessages[recipientKey.Public]
	senderManager.mutex.RUnlock()

	if len(queued) != len(messages) {
		t.Fatalf("Expected %d queued messages, got %d", len(messages), len(queued))
	}

	for i, msg := range messages {
		if queued[i].message != msg {
			t.Errorf("Message %d: expected '%s', got '%s'", i, msg, queued[i].message)
		}
	}
}

// TestQueuedMessagesSentAfterPreKeyExchange tests that queued messages are sent when friend comes online
func TestQueuedMessagesSentAfterPreKeyExchange(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "async_queue_send_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	senderKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key: %v", err)
	}

	recipientKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key: %v", err)
	}

	// Create mock transport
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Create managers for both sender and recipient
	senderManager, err := NewAsyncManager(senderKey, mockTransport, filepath.Join(tempDir, "sender"))
	if err != nil {
		t.Fatalf("Failed to create sender manager: %v", err)
	}

	recipientManager, err := NewAsyncManager(recipientKey, mockTransport, filepath.Join(tempDir, "recipient"))
	if err != nil {
		t.Fatalf("Failed to create recipient manager: %v", err)
	}

	senderManager.Start()
	defer senderManager.Stop()

	recipientManager.Start()
	defer recipientManager.Stop()

	// Set mock addresses for network communication
	senderAddr := &MockAddr{network: "mock", address: "sender:1234"}
	recipientAddr := &MockAddr{network: "mock", address: "recipient:5678"}

	senderManager.SetFriendAddress(recipientKey.Public, recipientAddr)
	recipientManager.SetFriendAddress(senderKey.Public, senderAddr)

	// Queue messages before pre-keys are available
	testMessages := []string{"Queued 1", "Queued 2"}
	for _, msg := range testMessages {
		err = senderManager.SendAsyncMessage(recipientKey.Public, msg, MessageTypeNormal)
		if err != nil {
			t.Fatalf("Failed to queue message: %v", err)
		}
	}

	// Verify messages are queued
	senderManager.mutex.RLock()
	queuedBefore := len(senderManager.pendingMessages[recipientKey.Public])
	senderManager.mutex.RUnlock()

	if queuedBefore != len(testMessages) {
		t.Fatalf("Expected %d queued messages before going online, got %d", len(testMessages), queuedBefore)
	}

	// Exchange pre-keys by simulating both parties coming online
	// First, generate pre-keys for each peer
	if err := senderManager.forwardSecurity.GeneratePreKeysForPeer(recipientKey.Public); err != nil {
		t.Fatalf("Failed to generate sender pre-keys: %v", err)
	}

	if err := recipientManager.forwardSecurity.GeneratePreKeysForPeer(senderKey.Public); err != nil {
		t.Fatalf("Failed to generate recipient pre-keys: %v", err)
	}

	// Exchange pre-keys manually (simulating the network exchange)
	senderExchange, err := senderManager.forwardSecurity.ExchangePreKeys(recipientKey.Public)
	if err != nil {
		t.Fatalf("Failed to create sender pre-key exchange: %v", err)
	}

	recipientExchange, err := recipientManager.forwardSecurity.ExchangePreKeys(senderKey.Public)
	if err != nil {
		t.Fatalf("Failed to create recipient pre-key exchange: %v", err)
	}

	// Process pre-key exchanges
	if err := recipientManager.ProcessPreKeyExchange(senderExchange); err != nil {
		t.Fatalf("Failed to process sender's pre-keys: %v", err)
	}

	if err := senderManager.ProcessPreKeyExchange(recipientExchange); err != nil {
		t.Fatalf("Failed to process recipient's pre-keys: %v", err)
	}

	// Simulate friend coming online (which triggers sending queued messages)
	senderManager.SetFriendOnlineStatus(recipientKey.Public, true)

	// Wait for queued messages to be sent
	time.Sleep(1 * time.Second)

	// Verify queue was cleared
	senderManager.mutex.RLock()
	queuedAfter := len(senderManager.pendingMessages[recipientKey.Public])
	senderManager.mutex.RUnlock()

	if queuedAfter != 0 {
		t.Errorf("Expected queue to be cleared after friend comes online, but %d messages remain", queuedAfter)
	}
}

// TestMessageTypePreservationInQueue tests that message types are preserved when queued
func TestMessageTypePreservationInQueue(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "async_type_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	senderKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	recipientKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	manager, err := NewAsyncManager(senderKey, mockTransport, filepath.Join(tempDir, "test"))
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	manager.Start()
	defer manager.Stop()

	// Queue messages with different types
	testCases := []struct {
		message string
		msgType MessageType
	}{
		{"Normal message", MessageTypeNormal},
		{"Action message", MessageTypeAction},
	}

	for _, tc := range testCases {
		err = manager.SendAsyncMessage(recipientKey.Public, tc.message, tc.msgType)
		if err != nil {
			t.Fatalf("Failed to queue message: %v", err)
		}
	}

	// Verify message types are preserved
	manager.mutex.RLock()
	queued := manager.pendingMessages[recipientKey.Public]
	manager.mutex.RUnlock()

	if len(queued) != len(testCases) {
		t.Fatalf("Expected %d queued messages, got %d", len(testCases), len(queued))
	}

	for i, tc := range testCases {
		if queued[i].message != tc.message {
			t.Errorf("Message %d: expected '%s', got '%s'", i, tc.message, queued[i].message)
		}
		if queued[i].messageType != tc.msgType {
			t.Errorf("Message %d: expected type %v, got %v", i, tc.msgType, queued[i].messageType)
		}
	}
}

// TestNoQueueingWhenPreKeysAvailable tests that messages are sent immediately when pre-keys exist
func TestNoQueueingWhenPreKeysAvailable(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "async_no_queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	senderKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key: %v", err)
	}

	recipientKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key: %v", err)
	}

	senderTransport := NewMockTransport("127.0.0.1:8080")
	senderManager, err := NewAsyncManager(senderKey, senderTransport, filepath.Join(tempDir, "sender"))
	if err != nil {
		t.Fatalf("Failed to create sender manager: %v", err)
	}

	recipientTransport := NewMockTransport("127.0.0.1:8081")
	recipientManager, err := NewAsyncManager(recipientKey, recipientTransport, filepath.Join(tempDir, "recipient"))
	if err != nil {
		t.Fatalf("Failed to create recipient manager: %v", err)
	}

	senderManager.Start()
	defer senderManager.Stop()

	recipientManager.Start()
	defer recipientManager.Stop()

	// Exchange pre-keys first
	if err := senderManager.forwardSecurity.GeneratePreKeysForPeer(recipientKey.Public); err != nil {
		t.Fatalf("Failed to generate sender pre-keys: %v", err)
	}

	if err := recipientManager.forwardSecurity.GeneratePreKeysForPeer(senderKey.Public); err != nil {
		t.Fatalf("Failed to generate recipient pre-keys: %v", err)
	}

	senderExchange, err := senderManager.forwardSecurity.ExchangePreKeys(recipientKey.Public)
	if err != nil {
		t.Fatalf("Failed to create sender exchange: %v", err)
	}

	recipientExchange, err := recipientManager.forwardSecurity.ExchangePreKeys(senderKey.Public)
	if err != nil {
		t.Fatalf("Failed to create recipient exchange: %v", err)
	}

	if err := recipientManager.ProcessPreKeyExchange(senderExchange); err != nil {
		t.Fatalf("Failed to process sender's pre-keys: %v", err)
	}

	if err := senderManager.ProcessPreKeyExchange(recipientExchange); err != nil {
		t.Fatalf("Failed to process recipient's pre-keys: %v", err)
	}

	// Add storage nodes for both managers
	storageAddr := &MockAddr{network: "mock", address: "storage:9999"}
	senderManager.AddStorageNode(recipientKey.Public, storageAddr)
	recipientManager.AddStorageNode(senderKey.Public, storageAddr)

	// Now send a message - should NOT be queued since pre-keys are available
	err = senderManager.SendAsyncMessage(recipientKey.Public, "Direct message", MessageTypeNormal)
	if err != nil {
		t.Fatalf("SendAsyncMessage failed: %v", err)
	}

	// Verify message was NOT queued
	senderManager.mutex.RLock()
	queued := len(senderManager.pendingMessages[recipientKey.Public])
	senderManager.mutex.RUnlock()

	if queued != 0 {
		t.Errorf("Expected no queued messages when pre-keys available, got %d", queued)
	}
}
