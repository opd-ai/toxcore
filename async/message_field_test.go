package async

import (
	"bytes"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestAsyncMessageDecrypt tests the Decrypt method for AsyncMessage
func TestAsyncMessageDecrypt(t *testing.T) {
	// Generate sender and recipient key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create and encrypt a test message
	plaintext := []byte("Hello, this is a test message for async delivery")
	encryptedData, nonce, err := encryptForRecipientInternal(plaintext, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to encrypt test message: %v", err)
	}

	// Create AsyncMessage with encrypted data
	msg := AsyncMessage{
		ID:            [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		RecipientPK:   recipientKeyPair.Public,
		SenderPK:      senderKeyPair.Public,
		EncryptedData: encryptedData,
		Nonce:         nonce,
		MessageType:   MessageTypeNormal,
	}

	// Verify message is not decrypted initially
	if msg.IsDecrypted() {
		t.Error("Message should not be marked as decrypted before calling Decrypt()")
	}

	// Decrypt the message
	decrypted, err := msg.Decrypt(recipientKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to decrypt message: %v", err)
	}

	// Verify decrypted content matches original plaintext
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted message doesn't match original.\nExpected: %s\nGot: %s", plaintext, decrypted)
	}

	// Verify Message field is populated
	if !msg.IsDecrypted() {
		t.Error("Message should be marked as decrypted after calling Decrypt()")
	}

	if !bytes.Equal(msg.Message, plaintext) {
		t.Errorf("Message field doesn't match decrypted content.\nExpected: %s\nGot: %s", plaintext, msg.Message)
	}

	// Verify EncryptedData field is unchanged
	if !bytes.Equal(msg.EncryptedData, encryptedData) {
		t.Error("EncryptedData field should remain unchanged after decryption")
	}
}

// TestAsyncMessageDecryptWrongKey tests decryption with incorrect key
func TestAsyncMessageDecryptWrongKey(t *testing.T) {
	// Generate sender, recipient, and wrong key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	wrongKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate wrong key pair: %v", err)
	}

	// Create and encrypt a test message
	plaintext := []byte("Secret message")
	encryptedData, nonce, err := encryptForRecipientInternal(plaintext, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to encrypt test message: %v", err)
	}

	// Create AsyncMessage
	msg := AsyncMessage{
		ID:            [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		RecipientPK:   recipientKeyPair.Public,
		SenderPK:      senderKeyPair.Public,
		EncryptedData: encryptedData,
		Nonce:         nonce,
		MessageType:   MessageTypeNormal,
	}

	// Attempt to decrypt with wrong private key
	_, err = msg.Decrypt(wrongKeyPair.Private)
	if err == nil {
		t.Error("Expected decryption to fail with wrong private key")
	}

	// Verify Message field is not populated after failed decryption
	if msg.IsDecrypted() {
		t.Error("Message should not be marked as decrypted after failed decryption")
	}
}

// TestAsyncMessageDecryptEmptyData tests decryption with empty encrypted data
func TestAsyncMessageDecryptEmptyData(t *testing.T) {
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	// Create AsyncMessage with empty EncryptedData
	msg := AsyncMessage{
		ID:            [16]byte{1, 2, 3},
		RecipientPK:   recipientKeyPair.Public,
		SenderPK:      senderKeyPair.Public,
		EncryptedData: []byte{},
		Nonce:         [24]byte{},
		MessageType:   MessageTypeNormal,
	}

	// Attempt to decrypt empty data
	_, err = msg.Decrypt(recipientKeyPair.Private)
	if err == nil {
		t.Error("Expected decryption to fail with empty encrypted data")
	}

	// Verify Message field is not populated
	if msg.IsDecrypted() {
		t.Error("Message should not be marked as decrypted after failed decryption")
	}
}

// TestAsyncMessageIsDecrypted tests the IsDecrypted method
func TestAsyncMessageIsDecrypted(t *testing.T) {
	tests := []struct {
		name           string
		message        []byte
		expectedResult bool
	}{
		{
			name:           "Empty message field",
			message:        nil,
			expectedResult: false,
		},
		{
			name:           "Empty byte slice",
			message:        []byte{},
			expectedResult: false,
		},
		{
			name:           "Non-empty message",
			message:        []byte("decrypted content"),
			expectedResult: true,
		},
		{
			name:           "Single byte message",
			message:        []byte{0x00},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := AsyncMessage{
				Message: tt.message,
			}

			result := msg.IsDecrypted()
			if result != tt.expectedResult {
				t.Errorf("IsDecrypted() = %v, expected %v", result, tt.expectedResult)
			}
		})
	}
}

// TestAsyncMessageDecryptMultipleCalls tests calling Decrypt multiple times
func TestAsyncMessageDecryptMultipleCalls(t *testing.T) {
	// Generate key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create and encrypt a test message
	plaintext := []byte("Test message for multiple decryption calls")
	encryptedData, nonce, err := encryptForRecipientInternal(plaintext, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to encrypt test message: %v", err)
	}

	// Create AsyncMessage
	msg := AsyncMessage{
		ID:            [16]byte{1, 2, 3},
		RecipientPK:   recipientKeyPair.Public,
		SenderPK:      senderKeyPair.Public,
		EncryptedData: encryptedData,
		Nonce:         nonce,
		MessageType:   MessageTypeNormal,
	}

	// First decryption
	decrypted1, err := msg.Decrypt(recipientKeyPair.Private)
	if err != nil {
		t.Fatalf("First decryption failed: %v", err)
	}

	// Second decryption (should succeed and return same result)
	decrypted2, err := msg.Decrypt(recipientKeyPair.Private)
	if err != nil {
		t.Fatalf("Second decryption failed: %v", err)
	}

	// Verify both decryptions return the same content
	if !bytes.Equal(decrypted1, decrypted2) {
		t.Error("Multiple decryptions should return the same result")
	}

	// Verify Message field is populated correctly
	if !bytes.Equal(msg.Message, plaintext) {
		t.Error("Message field should contain the decrypted content after multiple calls")
	}
}

// TestAsyncMessageDecryptActionMessage tests decryption of action message type
func TestAsyncMessageDecryptActionMessage(t *testing.T) {
	// Generate key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create and encrypt an action message
	plaintext := []byte("waves hello")
	encryptedData, nonce, err := encryptForRecipientInternal(plaintext, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to encrypt test message: %v", err)
	}

	// Create AsyncMessage with Action type
	msg := AsyncMessage{
		ID:            [16]byte{1, 2, 3},
		RecipientPK:   recipientKeyPair.Public,
		SenderPK:      senderKeyPair.Public,
		EncryptedData: encryptedData,
		Nonce:         nonce,
		MessageType:   MessageTypeAction,
	}

	// Decrypt the message
	decrypted, err := msg.Decrypt(recipientKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to decrypt action message: %v", err)
	}

	// Verify decrypted content
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted action message doesn't match original.\nExpected: %s\nGot: %s", plaintext, decrypted)
	}

	// Verify MessageType is preserved
	if msg.MessageType != MessageTypeAction {
		t.Errorf("MessageType should be preserved. Expected: %v, Got: %v", MessageTypeAction, msg.MessageType)
	}
}

// TestAsyncMessageFieldsPreserved tests that all fields are preserved during decryption
func TestAsyncMessageFieldsPreserved(t *testing.T) {
	// Generate key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create and encrypt a test message
	plaintext := []byte("Test for field preservation")
	encryptedData, nonce, err := encryptForRecipientInternal(plaintext, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to encrypt test message: %v", err)
	}

	// Create AsyncMessage with all fields populated
	originalID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	msg := AsyncMessage{
		ID:            originalID,
		RecipientPK:   recipientKeyPair.Public,
		SenderPK:      senderKeyPair.Public,
		EncryptedData: encryptedData,
		Nonce:         nonce,
		MessageType:   MessageTypeNormal,
	}

	// Store copies of original values
	originalRecipientPK := msg.RecipientPK
	originalSenderPK := msg.SenderPK
	originalEncryptedData := make([]byte, len(msg.EncryptedData))
	copy(originalEncryptedData, msg.EncryptedData)
	originalNonce := msg.Nonce

	// Decrypt the message
	_, err = msg.Decrypt(recipientKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to decrypt message: %v", err)
	}

	// Verify all original fields are preserved
	if msg.ID != originalID {
		t.Error("ID field was modified during decryption")
	}

	if msg.RecipientPK != originalRecipientPK {
		t.Error("RecipientPK field was modified during decryption")
	}

	if msg.SenderPK != originalSenderPK {
		t.Error("SenderPK field was modified during decryption")
	}

	if !bytes.Equal(msg.EncryptedData, originalEncryptedData) {
		t.Error("EncryptedData field was modified during decryption")
	}

	if msg.Nonce != originalNonce {
		t.Error("Nonce field was modified during decryption")
	}

	if msg.MessageType != MessageTypeNormal {
		t.Error("MessageType field was modified during decryption")
	}
}
