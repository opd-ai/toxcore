package messaging

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/sirupsen/logrus"
)

func TestMessageEncryption(t *testing.T) {
	tests := []struct {
		name        string
		messageText string
		friendID    uint32
		setupKeys   bool
		expectError bool
	}{
		{
			name:        "Encrypt normal message",
			messageText: "Hello, encrypted world!",
			friendID:    testDefaultFriendID,
			setupKeys:   true,
			expectError: false,
		},
		{
			name:        "Encrypt empty message",
			messageText: "",
			friendID:    testDefaultFriendID,
			setupKeys:   true,
			expectError: true, // Empty messages should fail at SendMessage level
		},
		{
			name:        "Encrypt long message",
			messageText: strings.Repeat("A", testLongMessageSize),
			friendID:    testDefaultFriendID,
			setupKeys:   true,
			expectError: false,
		},
		{
			name:        "Friend not found",
			messageText: "Test message",
			friendID:    testInvalidFriendID,
			setupKeys:   false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create message manager
			mm := NewMessageManager()

			// Create key provider
			keyProvider := newMockKeyProvider()

			// Setup friend key if needed
			if tt.setupKeys {
				friendKeyPair, _ := crypto.GenerateKeyPair()
				keyProvider.friendPublicKeys[tt.friendID] = friendKeyPair.Public
			}

			// Set key provider
			mm.SetKeyProvider(keyProvider)

			// Create mock transport
			transport := &mockTransport{}
			mm.SetTransport(transport)

			// Send message
			if tt.messageText == "" {
				// Empty message should fail at SendMessage
				_, err := mm.SendMessage(tt.friendID, tt.messageText, MessageTypeNormal)
				if err == nil {
					t.Error("Expected error for empty message")
				}
				return
			}

			message, err := mm.SendMessage(tt.friendID, tt.messageText, MessageTypeNormal)

			// Wait for async send to complete
			time.Sleep(testAsyncWait)

			if tt.expectError {
				if message != nil && message.State != MessageStateFailed && message.State != MessageStatePending {
					t.Errorf("Expected message to fail, got state: %v", message.State)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				// Verify message was sent
				if message.State != MessageStateSent {
					t.Errorf("Expected MessageStateSent, got: %v", message.State)
				}

				// Verify message was encrypted (text should be different from original)
				if len(transport.sentMessages) == 0 {
					t.Fatal("No messages sent through transport")
				}

				sentMsg := transport.sentMessages[0]
				if sentMsg.Text == tt.messageText {
					t.Error("Message was not encrypted")
				}

				// Encrypted message should be longer due to encryption overhead
				if len(sentMsg.Text) <= len(tt.messageText) {
					t.Error("Encrypted message should be longer than plaintext")
				}
			}
		})
	}
}

func TestEncryptionWithoutKeyProvider(t *testing.T) {
	// Create message manager without key provider
	mm := NewMessageManager()

	// Create mock transport
	transport := &mockTransport{}
	mm.SetTransport(transport)

	// Send message
	message, err := mm.SendMessage(testDefaultFriendID, "Test message", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Wait for async send
	time.Sleep(testAsyncWait)

	// Message should be sent unencrypted (backward compatibility)
	if message.State != MessageStateSent {
		t.Errorf("Expected MessageStateSent, got: %v", message.State)
	}

	if len(transport.sentMessages) == 0 {
		t.Fatal("No messages sent through transport")
	}

	// Message should remain in plaintext
	sentMsg := transport.sentMessages[0]
	if sentMsg.Text != "Test message" {
		t.Errorf("Expected plaintext message, got: %s", sentMsg.Text)
	}
}

func TestEncryptionFailureHandling(t *testing.T) {
	// Create message manager
	mm := NewMessageManager()
	mm.maxRetries = testReducedRetries // Reduce retries for faster test

	// Create key provider with missing friend key
	keyProvider := newMockKeyProvider()
	mm.SetKeyProvider(keyProvider)

	// Create mock transport
	transport := &mockTransport{}
	mm.SetTransport(transport)

	// Send message to non-existent friend
	message, err := mm.SendMessage(testInvalidFriendID, "Test message", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Unexpected error during SendMessage: %v", err)
	}

	// Wait for retries to complete (including retry interval)
	// First attempt: immediate
	// Second attempt: after retryInterval (5s)
	// Third attempt: after another retryInterval (5s)
	time.Sleep(testAsyncWait)

	// After initial failure, message should be pending or failed
	// With retries < maxRetries, it stays pending
	if message.State != MessageStatePending && message.State != MessageStateFailed {
		t.Logf("Message state after encryption failure: %v (expected pending or failed)", message.State)
	}

	// Verify retries were attempted
	if message.Retries < 1 {
		t.Errorf("Expected at least 1 retry attempt, got: %d", message.Retries)
	}

	// No messages should have been sent (encryption failed)
	if len(transport.sentMessages) > 0 {
		t.Errorf("Expected no messages sent, got: %d", len(transport.sentMessages))
	}
}

func TestMultipleFriendsEncryption(t *testing.T) {
	// Create message manager
	mm := NewMessageManager()

	// Create key provider with multiple friends
	keyProvider := newMockKeyProvider()

	// Add 3 friends with different keys
	for friendID := uint32(testDefaultFriendID); friendID <= testMultiFriendCount; friendID++ {
		friendKeyPair, _ := crypto.GenerateKeyPair()
		keyProvider.friendPublicKeys[friendID] = friendKeyPair.Public
	}

	mm.SetKeyProvider(keyProvider)

	// Create mock transport
	transport := &mockTransport{}
	mm.SetTransport(transport)

	// Send messages to all friends
	for friendID := uint32(testDefaultFriendID); friendID <= testMultiFriendCount; friendID++ {
		_, err := mm.SendMessage(friendID, "Hello friend!", MessageTypeNormal)
		if err != nil {
			t.Fatalf("Failed to send message to friend %d: %v", friendID, err)
		}
	}

	// Wait for all messages to be sent
	time.Sleep(testAsyncWaitMedium)

	// Verify all messages were sent
	if len(transport.sentMessages) != testMultiFriendCount {
		t.Errorf("Expected 3 messages sent, got: %d", len(transport.sentMessages))
	}

	// Verify each message is encrypted differently (different nonces)
	encryptedTexts := make(map[string]bool)
	for _, msg := range transport.sentMessages {
		if msg.Text == "Hello friend!" {
			t.Error("Message was not encrypted")
		}
		encryptedTexts[msg.Text] = true
	}

	// Each encrypted message should be unique due to different nonces
	if len(encryptedTexts) != testMultiFriendCount {
		t.Errorf("Expected 3 unique encrypted messages, got: %d", len(encryptedTexts))
	}
}

func TestTransportFailureWithEncryption(t *testing.T) {
	// Create message manager
	mm := NewMessageManager()
	mm.maxRetries = testSingleRetry // Single retry for faster test

	// Create key provider
	keyProvider := newMockKeyProvider()
	friendKeyPair, _ := crypto.GenerateKeyPair()
	keyProvider.friendPublicKeys[testDefaultFriendID] = friendKeyPair.Public
	mm.SetKeyProvider(keyProvider)

	// Create failing transport
	transport := &mockTransport{shouldFail: true}
	mm.SetTransport(transport)

	// Send message
	message, err := mm.SendMessage(testDefaultFriendID, "Test message", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Unexpected error during SendMessage: %v", err)
	}

	// Wait for retries to complete
	time.Sleep(testAsyncWaitMedium)

	// Message should fail due to transport failure
	if message.State != MessageStateFailed {
		t.Errorf("Expected MessageStateFailed, got: %v", message.State)
	}
}

func TestEncryptionPreservesMessageType(t *testing.T) {
	// Create message manager
	mm := NewMessageManager()

	// Create key provider
	keyProvider := newMockKeyProvider()
	friendKeyPair, _ := crypto.GenerateKeyPair()
	keyProvider.friendPublicKeys[testDefaultFriendID] = friendKeyPair.Public
	mm.SetKeyProvider(keyProvider)

	// Create mock transport
	transport := &mockTransport{}
	mm.SetTransport(transport)

	// Test both message types
	messageTypes := []MessageType{MessageTypeNormal, MessageTypeAction}

	for _, msgType := range messageTypes {
		message, err := mm.SendMessage(testDefaultFriendID, "Test message", msgType)
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}

		// Wait for send
		time.Sleep(testAsyncWaitShort)

		// Verify message type is preserved
		if message.Type != msgType {
			t.Errorf("Expected message type %v, got: %v", msgType, message.Type)
		}
	}
}

func TestConcurrentEncryption(t *testing.T) {
	// Create message manager
	mm := NewMessageManager()

	// Create key provider
	keyProvider := newMockKeyProvider()
	for friendID := uint32(testDefaultFriendID); friendID <= testConcurrentFriendMax; friendID++ {
		friendKeyPair, _ := crypto.GenerateKeyPair()
		keyProvider.friendPublicKeys[friendID] = friendKeyPair.Public
	}
	mm.SetKeyProvider(keyProvider)

	// Create mock transport
	transport := &mockTransport{}
	mm.SetTransport(transport)

	// Send multiple messages concurrently
	done := make(chan bool, testConcurrentFriendMax)
	for friendID := uint32(testDefaultFriendID); friendID <= testConcurrentFriendMax; friendID++ {
		go func(fid uint32) {
			_, err := mm.SendMessage(fid, "Concurrent message", MessageTypeNormal)
			if err != nil {
				t.Errorf("Failed to send message: %v", err)
			}
			done <- true
		}(friendID)
	}

	// Wait for all sends
	for i := 0; i < testConcurrentFriendMax; i++ {
		<-done
	}

	// Wait for async processing
	time.Sleep(testAsyncWaitLong)

	// Verify all messages were sent
	if len(transport.sentMessages) != testConcurrentFriendMax {
		t.Errorf("Expected %d messages sent, got: %d", testConcurrentFriendMax, len(transport.sentMessages))
	}
}

// TestUnencryptedMessageWarning verifies that sending messages without encryption
// logs a warning to alert developers/operators of potential security issues.
func TestUnencryptedMessageWarning(t *testing.T) {
	// Capture log output
	var logBuffer bytes.Buffer

	// Save original settings
	originalOutput := logrus.StandardLogger().Out
	originalFormatter := logrus.StandardLogger().Formatter
	originalLevel := logrus.StandardLogger().Level

	// Configure logrus to capture output
	logrus.SetOutput(&logBuffer)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableColors:    true,
	})
	logrus.SetLevel(logrus.WarnLevel)

	// Restore original settings after test
	defer func() {
		logrus.SetOutput(originalOutput)
		logrus.SetFormatter(originalFormatter)
		logrus.SetLevel(originalLevel)
	}()

	// Create message manager without key provider (unencrypted mode)
	mm := NewMessageManager()

	// Create mock transport
	transport := &mockTransport{}
	mm.SetTransport(transport)

	// Send unencrypted message
	message, err := mm.SendMessage(testDefaultFriendID, "Unencrypted test", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for async processing
	time.Sleep(testAsyncWait)

	// Verify message was sent
	if message.State != MessageStateSent {
		t.Errorf("Expected MessageStateSent, got: %v", message.State)
	}

	// Verify warning was logged
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "Sending message without encryption") {
		t.Errorf("Expected warning log about unencrypted message, but got: %q", logOutput)
	}

	// Verify log includes friend_id
	if !strings.Contains(logOutput, "friend_id") {
		t.Error("Expected friend_id in warning log")
	}

	// Verify log includes message_type
	if !strings.Contains(logOutput, "message_type") {
		t.Error("Expected message_type in warning log")
	}
}

// TestEncryptedMessageNoWarning verifies that sending encrypted messages
// does not generate the unencrypted warning.
func TestEncryptedMessageNoWarning(t *testing.T) {
	// Capture log output
	var logBuffer bytes.Buffer

	// Save original settings
	originalOutput := logrus.StandardLogger().Out
	originalFormatter := logrus.StandardLogger().Formatter
	originalLevel := logrus.StandardLogger().Level

	// Configure logrus to capture output
	logrus.SetOutput(&logBuffer)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableColors:    true,
	})
	logrus.SetLevel(logrus.WarnLevel)

	// Restore original settings after test
	defer func() {
		logrus.SetOutput(originalOutput)
		logrus.SetFormatter(originalFormatter)
		logrus.SetLevel(originalLevel)
	}()

	// Create message manager with key provider (encrypted mode)
	mm := NewMessageManager()

	// Create key provider
	keyProvider := newMockKeyProvider()
	friendKeyPair, _ := crypto.GenerateKeyPair()
	keyProvider.friendPublicKeys[testDefaultFriendID] = friendKeyPair.Public
	mm.SetKeyProvider(keyProvider)

	// Create mock transport
	transport := &mockTransport{}
	mm.SetTransport(transport)

	// Send encrypted message
	message, err := mm.SendMessage(testDefaultFriendID, "Encrypted test", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for async processing
	time.Sleep(testAsyncWait)

	// Verify message was sent
	if message.State != MessageStateSent {
		t.Errorf("Expected MessageStateSent, got: %v", message.State)
	}

	// Verify NO warning about unencrypted message
	logOutput := logBuffer.String()
	if strings.Contains(logOutput, "Sending message without encryption") {
		t.Error("Unexpected warning log about unencrypted message for encrypted message")
	}
}

// TestEncryptedMessageBase64Encoding verifies that encrypted messages are base64 encoded
// to prevent data corruption from null bytes or invalid UTF-8 sequences.
func TestEncryptedMessageBase64Encoding(t *testing.T) {
	// Create message manager with key provider
	mm := NewMessageManager()

	// Create key provider
	keyProvider := newMockKeyProvider()
	friendKeyPair, _ := crypto.GenerateKeyPair()
	keyProvider.friendPublicKeys[testDefaultFriendID] = friendKeyPair.Public
	mm.SetKeyProvider(keyProvider)

	// Create mock transport
	transport := &mockTransport{}
	mm.SetTransport(transport)

	// Send encrypted message
	_, err := mm.SendMessage(testDefaultFriendID, "Test message for base64 encoding", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for async processing
	time.Sleep(testAsyncWait)

	// Verify message was sent
	if len(transport.sentMessages) == 0 {
		t.Fatal("No messages sent through transport")
	}

	sentMsg := transport.sentMessages[0]

	// Verify encrypted text is valid base64
	_, err = base64.StdEncoding.DecodeString(sentMsg.Text)
	if err != nil {
		t.Errorf("Encrypted message is not valid base64: %v", err)
	}

	// Verify encoded text differs from plaintext
	if sentMsg.Text == "Test message for base64 encoding" {
		t.Error("Message was not encrypted")
	}

	// Verify message contains only printable ASCII (base64 property)
	for _, c := range sentMsg.Text {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=') {
			t.Errorf("Encrypted message contains non-base64 character: %q", c)
		}
	}
}

// TestErrNoEncryptionSentinelError verifies that ErrNoEncryption is returned
// by encryptMessage when no key provider is configured, and can be identified
// using errors.Is for explicit handling by callers.
func TestErrNoEncryptionSentinelError(t *testing.T) {
	// Create message manager without key provider
	mm := NewMessageManager()

	// Create a test message
	message := NewMessage(1, "Test message", MessageTypeNormal)

	// Call encryptMessage directly to verify it returns ErrNoEncryption
	err := mm.encryptMessage(message)

	// Verify error is ErrNoEncryption
	if err == nil {
		t.Fatal("Expected ErrNoEncryption, got nil")
	}

	if !errors.Is(err, ErrNoEncryption) {
		t.Errorf("Expected errors.Is(err, ErrNoEncryption) to be true, got error: %v", err)
	}

	// Verify the error message is descriptive
	expectedMsg := "encryption not available: no key provider configured"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestErrNoEncryptionAllowsUnencryptedTransmission verifies that when encryptMessage
// returns ErrNoEncryption, the message is still sent unencrypted for backward compatibility.
func TestErrNoEncryptionAllowsUnencryptedTransmission(t *testing.T) {
	// Create message manager without key provider
	mm := NewMessageManager()

	// Create mock transport
	transport := &mockTransport{}
	mm.SetTransport(transport)

	// Send message - should succeed despite no encryption
	message, err := mm.SendMessage(testDefaultFriendID, "Test unencrypted message", MessageTypeNormal)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Wait for async send
	time.Sleep(testAsyncWait)

	// Message should be sent successfully
	if message.State != MessageStateSent {
		t.Errorf("Expected MessageStateSent, got: %v", message.State)
	}

	// Verify message was sent through transport
	if len(transport.sentMessages) == 0 {
		t.Fatal("No messages sent through transport")
	}

	// Message text should be unencrypted (plaintext preserved)
	sentMsg := transport.sentMessages[0]
	if sentMsg.Text != "Test unencrypted message" {
		t.Errorf("Expected plaintext message, got: %s", sentMsg.Text)
	}
}
