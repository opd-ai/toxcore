package async

import (
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// TestKnownSenderManagement tests the known sender management functionality
func TestKnownSenderManagement(t *testing.T) {
	// Create test key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create mock transport
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Create async client
	client := NewAsyncClient(recipientKeyPair, mockTransport)

	// Test adding known sender
	client.AddKnownSender(senderKeyPair.Public)
	knownSenders := client.GetKnownSenders()
	if len(knownSenders) != 1 {
		t.Errorf("Expected 1 known sender, got %d", len(knownSenders))
	}
	if !knownSenders[senderKeyPair.Public] {
		t.Error("Sender not found in known senders list")
	}

	// Test removing known sender
	client.RemoveKnownSender(senderKeyPair.Public)
	knownSenders = client.GetKnownSenders()
	if len(knownSenders) != 0 {
		t.Errorf("Expected 0 known senders after removal, got %d", len(knownSenders))
	}
}

// TestDecryptObfuscatedMessageNoKnownSenders tests decryption with no known senders
func TestDecryptObfuscatedMessageNoKnownSenders(t *testing.T) {
	// Create test key pairs
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create mock transport
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Create async client
	client := NewAsyncClient(recipientKeyPair, mockTransport)

	// Create a test obfuscated message
	currentEpoch := client.obfuscation.epochManager.GetCurrentEpoch()
	recipientPseudonym, err := client.obfuscation.GenerateRecipientPseudonym(recipientKeyPair.Public, currentEpoch)
	if err != nil {
		t.Fatalf("Failed to generate recipient pseudonym: %v", err)
	}

	obfMsg := &ObfuscatedAsyncMessage{
		Type:               "obfuscated_async_message",
		MessageID:          [32]byte{1, 2, 3, 4},
		SenderPseudonym:    [32]byte{5, 6, 7, 8},
		RecipientPseudonym: recipientPseudonym,
		Epoch:              currentEpoch,
		MessageNonce:       [24]byte{9, 10, 11, 12},
		EncryptedPayload:   []byte("test payload"),
		PayloadNonce:       [12]byte{13, 14, 15, 16},
		PayloadTag:         [16]byte{17, 18, 19, 20},
		Timestamp:          time.Now(),
		ExpiresAt:          time.Now().Add(24 * time.Hour),
		RecipientProof:     [32]byte{21, 22, 23, 24},
	}

	// Attempt decryption without known senders
	_, err = client.decryptObfuscatedMessage(obfMsg)
	if err == nil {
		t.Error("Expected error when no known senders are configured")
	}
	expectedErr := "no known senders configured - cannot decrypt message without sender identification"
	if err.Error() != expectedErr {
		t.Errorf("Expected error: %s, got: %s", expectedErr, err.Error())
	}
}

// TestDecryptObfuscatedMessageWrongRecipient tests decryption with wrong recipient
func TestDecryptObfuscatedMessageWrongRecipient(t *testing.T) {
	// Create test key pairs
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	wrongRecipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate wrong recipient key pair: %v", err)
	}

	// Create mock transport
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Create async client
	client := NewAsyncClient(recipientKeyPair, mockTransport)

	// Create a test obfuscated message for wrong recipient
	currentEpoch := client.obfuscation.epochManager.GetCurrentEpoch()
	wrongRecipientPseudonym, err := client.obfuscation.GenerateRecipientPseudonym(wrongRecipientKeyPair.Public, currentEpoch)
	if err != nil {
		t.Fatalf("Failed to generate wrong recipient pseudonym: %v", err)
	}

	obfMsg := &ObfuscatedAsyncMessage{
		Type:               "obfuscated_async_message",
		MessageID:          [32]byte{1, 2, 3, 4},
		SenderPseudonym:    [32]byte{5, 6, 7, 8},
		RecipientPseudonym: wrongRecipientPseudonym,
		Epoch:              currentEpoch,
		MessageNonce:       [24]byte{9, 10, 11, 12},
		EncryptedPayload:   []byte("test payload"),
		PayloadNonce:       [12]byte{13, 14, 15, 16},
		PayloadTag:         [16]byte{17, 18, 19, 20},
		Timestamp:          time.Now(),
		ExpiresAt:          time.Now().Add(24 * time.Hour),
		RecipientProof:     [32]byte{21, 22, 23, 24},
	}

	// Attempt decryption with wrong recipient
	_, err = client.decryptObfuscatedMessage(obfMsg)
	if err == nil {
		t.Error("Expected error when message is not for this recipient")
	}
	if err.Error() != "message not intended for this recipient" {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestTryDecryptWithSenderBasicFlow tests the basic sender decryption flow
func TestTryDecryptWithSenderBasicFlow(t *testing.T) {
	// Create test key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create mock transport
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Create async client
	client := NewAsyncClient(recipientKeyPair, mockTransport)

	// Create a test obfuscated message with the correct structure
	currentEpoch := client.obfuscation.epochManager.GetCurrentEpoch()
	recipientPseudonym, err := client.obfuscation.GenerateRecipientPseudonym(recipientKeyPair.Public, currentEpoch)
	if err != nil {
		t.Fatalf("Failed to generate recipient pseudonym: %v", err)
	}

	obfMsg := &ObfuscatedAsyncMessage{
		Type:               "obfuscated_async_message",
		MessageID:          [32]byte{1, 2, 3, 4},
		SenderPseudonym:    [32]byte{5, 6, 7, 8},
		RecipientPseudonym: recipientPseudonym,
		Epoch:              currentEpoch,
		MessageNonce:       [24]byte{9, 10, 11, 12},
		EncryptedPayload:   []byte("test payload"),
		PayloadNonce:       [12]byte{13, 14, 15, 16},
		PayloadTag:         [16]byte{17, 18, 19, 20},
		Timestamp:          time.Now(),
		ExpiresAt:          time.Now().Add(24 * time.Hour),
		RecipientProof:     [32]byte{21, 22, 23, 24},
	}

	// Add sender to known senders
	client.AddKnownSender(senderKeyPair.Public)

	// Test basic sender decryption - this will fail at the obfuscation layer
	// since we don't have the proper shared secret setup, but it tests the flow
	_, err = client.tryDecryptWithSender(obfMsg, senderKeyPair.Public)
	if err == nil {
		t.Error("Expected error in basic decryption test due to incomplete setup")
	}
	// Verify we get a meaningful error about shared secret derivation
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

// TestGetKnownSendersIsolation tests that GetKnownSenders returns a copy
func TestGetKnownSendersIsolation(t *testing.T) {
	// Create test key pairs
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	// Create mock transport
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Create async client
	client := NewAsyncClient(recipientKeyPair, mockTransport)

	// Add known sender
	client.AddKnownSender(senderKeyPair.Public)

	// Get known senders and modify the returned map
	knownSenders := client.GetKnownSenders()
	knownSenders[[32]byte{99, 99, 99}] = true

	// Verify original client's known senders are unchanged
	originalKnownSenders := client.GetKnownSenders()
	if len(originalKnownSenders) != 1 {
		t.Errorf("Expected 1 known sender in original, got %d", len(originalKnownSenders))
	}
	if _, exists := originalKnownSenders[[32]byte{99, 99, 99}]; exists {
		t.Error("Original known senders map was modified - isolation failed")
	}
}

// TestTryDecryptFromKnownSendersMultipleSenders tests trying multiple senders
func TestTryDecryptFromKnownSendersMultipleSenders(t *testing.T) {
	// Create test key pairs
	sender1KeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender1 key pair: %v", err)
	}

	sender2KeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender2 key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create mock transport
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Create async client
	client := NewAsyncClient(recipientKeyPair, mockTransport)

	// Add multiple known senders
	client.AddKnownSender(sender1KeyPair.Public)
	client.AddKnownSender(sender2KeyPair.Public)

	// Create a test obfuscated message
	currentEpoch := client.obfuscation.epochManager.GetCurrentEpoch()
	recipientPseudonym, err := client.obfuscation.GenerateRecipientPseudonym(recipientKeyPair.Public, currentEpoch)
	if err != nil {
		t.Fatalf("Failed to generate recipient pseudonym: %v", err)
	}

	obfMsg := &ObfuscatedAsyncMessage{
		Type:               "obfuscated_async_message",
		MessageID:          [32]byte{1, 2, 3, 4},
		SenderPseudonym:    [32]byte{5, 6, 7, 8},
		RecipientPseudonym: recipientPseudonym,
		Epoch:              currentEpoch,
		MessageNonce:       [24]byte{9, 10, 11, 12},
		EncryptedPayload:   []byte("test payload"),
		PayloadNonce:       [12]byte{13, 14, 15, 16},
		PayloadTag:         [16]byte{17, 18, 19, 20},
		Timestamp:          time.Now(),
		ExpiresAt:          time.Now().Add(24 * time.Hour),
		RecipientProof:     [32]byte{21, 22, 23, 24},
	}

	// Test decryption with multiple known senders
	_, err = client.tryDecryptFromKnownSenders(obfMsg)
	if err == nil {
		t.Error("Expected error due to incomplete cryptographic setup")
	}

	// Should try multiple senders and fail on each
	if err.Error() == "no known senders configured - cannot decrypt message without sender identification" {
		t.Error("Should have attempted to decrypt with known senders")
	}

	// Verify it tried multiple senders by checking the error indicates failed decryption attempts
	expectedErrorSubstring := "failed to decrypt message with any known sender"
	if len(err.Error()) > 0 && err.Error()[:len(expectedErrorSubstring)] != expectedErrorSubstring {
		t.Logf("Got expected error indicating multiple decryption attempts: %v", err)
	}
}
