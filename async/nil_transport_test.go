package async

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestNewAsyncClientWithNilTransport verifies that creating an AsyncClient with nil transport
// does not panic and gracefully handles the absence of transport.
// This addresses the critical bug identified in AUDIT.md where nil transport caused SIGSEGV.
func TestNewAsyncClientWithNilTransport(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// This should NOT panic - previously would crash with SIGSEGV
	client := NewAsyncClient(keyPair, nil)
	if client == nil {
		t.Fatal("NewAsyncClient returned nil")
	}

	// Verify client was created with expected fields
	if client.keyPair != keyPair {
		t.Error("Client key pair not set correctly")
	}

	if client.transport != nil {
		t.Error("Expected transport to be nil")
	}

	if client.obfuscation == nil {
		t.Error("Obfuscation manager should be initialized even with nil transport")
	}

	if client.storageNodes == nil {
		t.Error("Storage nodes map should be initialized")
	}

	if client.retrievalScheduler == nil {
		t.Error("Retrieval scheduler should be initialized")
	}
}

// TestAsyncClientSendWithNilTransport verifies that attempting to send messages
// with nil transport returns appropriate errors instead of panicking.
func TestAsyncClientSendWithNilTransport(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	client := NewAsyncClient(keyPair, nil)

	// Create a test forward secure message
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	fsMsg := &ForwardSecureMessage{
		PreKeyID:      1,
		EncryptedData: []byte("test message"),
		Nonce:         [24]byte{},
		RecipientPK:   recipientKeyPair.Public,
		SenderPK:      keyPair.Public,
	}

	// Test SendObfuscatedMessage with nil transport
	// This should return an error, not panic
	err = client.SendObfuscatedMessage(recipientKeyPair.Public, fsMsg)
	if err == nil {
		t.Error("Expected error when sending with nil transport, got nil")
	}

	// With nil transport, there are no storage nodes, so we expect either:
	// - "no storage nodes available" error (graceful degradation)
	// - "transport is nil" error (if reached the transport check)
	// Both are acceptable as they indicate proper error handling without panic
	if err != nil {
		errMsg := err.Error()
		if !containsError(errMsg, "storage nodes") && !containsError(errMsg, "transport") {
			t.Errorf("Expected error related to storage nodes or transport, got: %v", err)
		}
	}
}

// TestAsyncClientRetrieveWithNilTransport verifies that retrieve operations
// handle nil transport gracefully without panicking.
func TestAsyncClientRetrieveWithNilTransport(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	client := NewAsyncClient(keyPair, nil)

	// Attempt to retrieve messages - should return error
	// This should return an error, not panic
	messages, err := client.RetrieveObfuscatedMessages()
	if err == nil {
		// If no error, verify messages are empty (graceful degradation)
		if messages != nil && len(messages) > 0 {
			t.Error("Expected no messages when transport is unavailable")
		}
	}
}

// TestNewAsyncManagerWithNilTransport verifies that AsyncManager can be created
// with nil transport without panicking, supporting graceful degradation.
func TestNewAsyncManagerWithNilTransport(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create temporary directory for storage
	dataDir := t.TempDir()

	// This should NOT panic - async manager should handle nil transport gracefully
	manager, err := NewAsyncManager(keyPair, nil, dataDir)
	if err != nil {
		t.Fatalf("NewAsyncManager failed with nil transport: %v", err)
	}

	if manager == nil {
		t.Fatal("NewAsyncManager returned nil")
	}

	if manager.client == nil {
		t.Error("AsyncManager client should be initialized")
	}

	if manager.client.transport != nil {
		t.Error("Expected client transport to be nil")
	}

	if manager.storage == nil {
		t.Error("AsyncManager storage should be initialized")
	}

	if manager.forwardSecurity == nil {
		t.Error("AsyncManager forward security should be initialized")
	}
}

// TestAsyncManagerSendWithNilTransport verifies that sending async messages
// through AsyncManager with nil transport queues the message (new automatic queueing behavior).
// The message will fail to send later when the friend comes online and sending is attempted.
func TestAsyncManagerSendWithNilTransport(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	dataDir := t.TempDir()
	manager, err := NewAsyncManager(keyPair, nil, dataDir)
	if err != nil {
		t.Fatalf("NewAsyncManager failed: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Attempt to send a message without pre-keys - should queue successfully
	// (new automatic queueing behavior - message will fail later when actually sent)
	err = manager.SendAsyncMessage(recipientKeyPair.Public, "test message", MessageTypeNormal)
	if err != nil {
		t.Errorf("Unexpected error when queueing message: %v", err)
	}

	// Verify message was queued
	manager.mutex.RLock()
	queued := manager.pendingMessages[recipientKeyPair.Public]
	manager.mutex.RUnlock()

	if len(queued) != 1 {
		t.Errorf("Expected 1 queued message, got %d", len(queued))
	}

	if len(queued) > 0 && queued[0].message != "test message" {
		t.Errorf("Expected queued message 'test message', got '%s'", queued[0].message)
	}
}

// Helper function to check if an error message contains a substring
func containsError(errMsg, substr string) bool {
	return len(errMsg) > 0 && len(substr) > 0 &&
		(errMsg == substr || len(errMsg) >= len(substr) &&
			findSubstring(errMsg, substr))
}

// Simple substring search helper
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
