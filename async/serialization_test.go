package async

import (
	"testing"
	"time"

	"github.com/opd-ai/toxforge/crypto"
)

// TestSerializeForwardSecureMessage tests the production-ready binary serialization
func TestSerializeForwardSecureMessage(t *testing.T) {
	// Setup
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Create test message
	testTime := time.Date(2025, 9, 3, 12, 0, 0, 0, time.UTC)
	fsMsg := &ForwardSecureMessage{
		Type:          "forward_secure_message",
		MessageID:     [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		SenderPK:      keyPair.Public,
		RecipientPK:   [32]byte{32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
		PreKeyID:      12345,
		EncryptedData: []byte("test encrypted message data"),
		Nonce:         [24]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24},
		MessageType:   MessageTypeNormal,
		Timestamp:     testTime,
		ExpiresAt:     testTime.Add(24 * time.Hour),
	}

	// Test serialization
	serialized, err := client.serializeForwardSecureMessage(fsMsg)
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}

	if len(serialized) == 0 {
		t.Error("Serialized data should not be empty")
	}

	// Test deserialization
	deserialized, err := client.deserializeForwardSecureMessage(serialized)
	if err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	// Verify round-trip accuracy
	if deserialized.Type != fsMsg.Type {
		t.Errorf("Type mismatch: got %s, want %s", deserialized.Type, fsMsg.Type)
	}
	if deserialized.MessageID != fsMsg.MessageID {
		t.Errorf("MessageID mismatch: got %x, want %x", deserialized.MessageID, fsMsg.MessageID)
	}
	if deserialized.SenderPK != fsMsg.SenderPK {
		t.Errorf("SenderPK mismatch: got %x, want %x", deserialized.SenderPK, fsMsg.SenderPK)
	}
	if deserialized.RecipientPK != fsMsg.RecipientPK {
		t.Errorf("RecipientPK mismatch: got %x, want %x", deserialized.RecipientPK, fsMsg.RecipientPK)
	}
	if deserialized.PreKeyID != fsMsg.PreKeyID {
		t.Errorf("PreKeyID mismatch: got %d, want %d", deserialized.PreKeyID, fsMsg.PreKeyID)
	}
	if string(deserialized.EncryptedData) != string(fsMsg.EncryptedData) {
		t.Errorf("EncryptedData mismatch: got %s, want %s", deserialized.EncryptedData, fsMsg.EncryptedData)
	}
	if deserialized.Nonce != fsMsg.Nonce {
		t.Errorf("Nonce mismatch: got %x, want %x", deserialized.Nonce, fsMsg.Nonce)
	}
	if deserialized.MessageType != fsMsg.MessageType {
		t.Errorf("MessageType mismatch: got %d, want %d", deserialized.MessageType, fsMsg.MessageType)
	}
	if !deserialized.Timestamp.Equal(fsMsg.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", deserialized.Timestamp, fsMsg.Timestamp)
	}
	if !deserialized.ExpiresAt.Equal(fsMsg.ExpiresAt) {
		t.Errorf("ExpiresAt mismatch: got %v, want %v", deserialized.ExpiresAt, fsMsg.ExpiresAt)
	}
}

// TestSerializeForwardSecureMessage_ErrorCases tests error handling in serialization
func TestSerializeForwardSecureMessage_ErrorCases(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Test nil message
	_, err = client.serializeForwardSecureMessage(nil)
	if err == nil {
		t.Error("Expected error for nil message")
	}
	expectedError := "cannot serialize nil ForwardSecureMessage"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got: %v", expectedError, err)
	}

	// Test empty data deserialization
	_, err = client.deserializeForwardSecureMessage([]byte{})
	if err == nil {
		t.Error("Expected error for empty data")
	}
	expectedError = "cannot deserialize empty data"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got: %v", expectedError, err)
	}

	// Test invalid data deserialization
	_, err = client.deserializeForwardSecureMessage([]byte("invalid data"))
	if err == nil {
		t.Error("Expected error for invalid data")
	}
	// Should contain "failed to decode" in the error message
	errorMsg := err.Error()
	expectedPrefix := "failed to decode"
	if len(errorMsg) < len(expectedPrefix) {
		t.Errorf("Error message too short: %v", err)
	} else if errorMsg[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("Expected '%s' prefix, got: %v", expectedPrefix, err)
	}
}

// TestSerializationPerformance benchmarks the serialization performance
/*func TestSerializationPerformance(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Create test message
	fsMsg := &ForwardSecureMessage{
		Type:          "forward_secure_message",
		MessageID:     [32]byte{1, 2, 3, 4, 5},
		SenderPK:      keyPair.Public,
		RecipientPK:   keyPair.Public,
		PreKeyID:      12345,
		EncryptedData: make([]byte, 1000), // 1KB encrypted data
		Nonce:         [24]byte{1, 2, 3, 4, 5},
		MessageType:   MessageTypeNormal,
		Timestamp:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}

	// Warm up
	for i := 0; i < 10; i++ {
		_, err := client.serializeForwardSecureMessage(fsMsg)
		if err != nil {
			t.Fatalf("Warmup serialization failed: %v", err)
		}
	}

	// Measure serialization time
	iterations := 1000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := client.serializeForwardSecureMessage(fsMsg)
		if err != nil {
			t.Fatalf("Serialization failed on iteration %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	avgTime := elapsed / time.Duration(iterations)
	t.Logf("Serialization performance: %v per operation (%d iterations)", avgTime, iterations)

	// Verify performance is reasonable (should be under 100μs per operation)
	if avgTime > 100*time.Microsecond {
		t.Errorf("Serialization too slow: %v per operation (expected < 100μs)", avgTime)
	}
}
*/
// TestSerializationSizeEfficiency tests that binary serialization is efficient
func TestSerializationSizeEfficiency(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Create minimal test message
	fsMsg := &ForwardSecureMessage{
		Type:          "forward_secure_message",
		MessageID:     [32]byte{1},
		SenderPK:      keyPair.Public,
		RecipientPK:   keyPair.Public,
		PreKeyID:      0,
		EncryptedData: []byte("Hello"),
		Nonce:         [24]byte{1},
		MessageType:   MessageTypeNormal,
		Timestamp:     time.Now(),
		ExpiresAt:     time.Now().Add(time.Hour),
	}

	// Test binary serialization
	binaryData, err := client.serializeForwardSecureMessage(fsMsg)
	if err != nil {
		t.Fatalf("Binary serialization failed: %v", err)
	}

	t.Logf("Binary serialization size: %d bytes", len(binaryData))

	// Binary should be reasonable size (under 500 bytes for this minimal message)
	if len(binaryData) > 500 {
		t.Errorf("Binary serialization too large: %d bytes (expected < 500)", len(binaryData))
	}

	// Verify it can round-trip
	deserialized, err := client.deserializeForwardSecureMessage(binaryData)
	if err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	if deserialized.Type != fsMsg.Type {
		t.Error("Round-trip failed: Type mismatch")
	}
	if string(deserialized.EncryptedData) != string(fsMsg.EncryptedData) {
		t.Error("Round-trip failed: EncryptedData mismatch")
	}
}
