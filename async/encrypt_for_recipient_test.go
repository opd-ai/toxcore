package async

import (
	"bytes"
	"strings"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestEncryptForRecipient_BasicFunctionality verifies basic encryption/decryption
func TestEncryptForRecipient_BasicFunctionality(t *testing.T) {
	// Generate key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	message := []byte("Hello, this is a test message!")

	// Encrypt message
	encryptedData, nonce, err := EncryptForRecipient(message, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("EncryptForRecipient failed: %v", err)
	}

	// Verify encrypted data is not empty
	if len(encryptedData) == 0 {
		t.Fatal("Encrypted data should not be empty")
	}

	// Verify nonce is not zero
	zeroNonce := [24]byte{}
	if nonce == zeroNonce {
		t.Fatal("Nonce should not be zero")
	}

	// Decrypt message
	var cryptoNonce crypto.Nonce
	copy(cryptoNonce[:], nonce[:])
	decrypted, err := crypto.Decrypt(encryptedData, cryptoNonce, senderKeyPair.Public, recipientKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to decrypt message: %v", err)
	}

	// Verify decrypted message matches original
	if !bytes.Equal(decrypted, message) {
		t.Errorf("Expected decrypted message %q, got %q", message, decrypted)
	}
}

// TestEncryptForRecipient_EmptyMessage verifies empty message handling
func TestEncryptForRecipient_EmptyMessage(t *testing.T) {
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()

	_, _, err := EncryptForRecipient([]byte{}, recipientKeyPair.Public, senderKeyPair.Private)
	if err == nil {
		t.Fatal("Expected error for empty message")
	}
	if !strings.Contains(err.Error(), "empty message") {
		t.Errorf("Expected 'empty message' error, got: %v", err)
	}
}

// TestEncryptForRecipient_MaxMessageSize verifies max size handling
func TestEncryptForRecipient_MaxMessageSize(t *testing.T) {
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()

	// Test exactly at max size (should succeed)
	maxMessage := make([]byte, MaxMessageSize)
	for i := range maxMessage {
		maxMessage[i] = byte(i % 256)
	}

	encryptedData, nonce, err := EncryptForRecipient(maxMessage, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("Should accept message at max size: %v", err)
	}

	// Verify decryption works
	var cryptoNonce crypto.Nonce
	copy(cryptoNonce[:], nonce[:])
	decrypted, err := crypto.Decrypt(encryptedData, cryptoNonce, senderKeyPair.Public, recipientKeyPair.Private)
	if err != nil {
		t.Fatalf("Failed to decrypt max-size message: %v", err)
	}
	if !bytes.Equal(decrypted, maxMessage) {
		t.Error("Decrypted max-size message doesn't match original")
	}

	// Test over max size (should fail)
	tooLargeMessage := make([]byte, MaxMessageSize+1)
	_, _, err = EncryptForRecipient(tooLargeMessage, recipientKeyPair.Public, senderKeyPair.Private)
	if err == nil {
		t.Fatal("Expected error for message exceeding max size")
	}
	if !strings.Contains(err.Error(), "message too long") {
		t.Errorf("Expected 'message too long' error, got: %v", err)
	}
}

// TestEncryptForRecipient_DifferentMessageTypes verifies various message contents
func TestEncryptForRecipient_DifferentMessageTypes(t *testing.T) {
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()

	testCases := []struct {
		name    string
		message []byte
	}{
		{
			name:    "ASCII text",
			message: []byte("Hello, World!"),
		},
		{
			name:    "Unicode text",
			message: []byte("Hello 世界 🌍"),
		},
		{
			name:    "Binary data",
			message: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		},
		{
			name:    "Single byte",
			message: []byte{0x42},
		},
		{
			name:    "Repeated pattern",
			message: bytes.Repeat([]byte("ABC"), 100),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			encryptedData, nonce, err := EncryptForRecipient(tc.message, recipientKeyPair.Public, senderKeyPair.Private)
			if err != nil {
				t.Fatalf("Encryption failed: %v", err)
			}

			// Decrypt
			var cryptoNonce crypto.Nonce
			copy(cryptoNonce[:], nonce[:])
			decrypted, err := crypto.Decrypt(encryptedData, cryptoNonce, senderKeyPair.Public, recipientKeyPair.Private)
			if err != nil {
				t.Fatalf("Decryption failed: %v", err)
			}

			// Verify
			if !bytes.Equal(decrypted, tc.message) {
				t.Errorf("Expected %v, got %v", tc.message, decrypted)
			}
		})
	}
}

// TestEncryptForRecipient_UniqueNonces verifies each call generates unique nonces
func TestEncryptForRecipient_UniqueNonces(t *testing.T) {
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()

	message := []byte("Same message encrypted multiple times")
	nonces := make(map[[24]byte]bool)

	// Encrypt same message 100 times
	for i := 0; i < 100; i++ {
		_, nonce, err := EncryptForRecipient(message, recipientKeyPair.Public, senderKeyPair.Private)
		if err != nil {
			t.Fatalf("Encryption %d failed: %v", i, err)
		}

		// Check for duplicate nonces
		if nonces[nonce] {
			t.Fatalf("Duplicate nonce detected at iteration %d", i)
		}
		nonces[nonce] = true
	}
}

// TestEncryptForRecipient_WrongDecryptionKey verifies decryption fails with wrong key
func TestEncryptForRecipient_WrongDecryptionKey(t *testing.T) {
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()
	wrongKeyPair, _ := crypto.GenerateKeyPair()

	message := []byte("Secret message")

	// Encrypt
	encryptedData, nonce, err := EncryptForRecipient(message, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with wrong recipient key
	var cryptoNonce crypto.Nonce
	copy(cryptoNonce[:], nonce[:])
	_, err = crypto.Decrypt(encryptedData, cryptoNonce, senderKeyPair.Public, wrongKeyPair.Private)
	if err == nil {
		t.Fatal("Decryption should fail with wrong recipient key")
	}

	// Try to decrypt with wrong sender key
	_, err = crypto.Decrypt(encryptedData, cryptoNonce, wrongKeyPair.Public, recipientKeyPair.Private)
	if err == nil {
		t.Fatal("Decryption should fail with wrong sender key")
	}
}

// TestEncryptForRecipient_EncryptedDataDiffers verifies encrypted data differs from plaintext
func TestEncryptForRecipient_EncryptedDataDiffers(t *testing.T) {
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()

	message := []byte("This message should be encrypted!")

	encryptedData, _, err := EncryptForRecipient(message, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Verify encrypted data differs from plaintext
	if bytes.Equal(encryptedData, message) {
		t.Error("Encrypted data should differ from plaintext")
	}

	// Verify encrypted data doesn't contain plaintext
	if bytes.Contains(encryptedData, message) {
		t.Error("Encrypted data should not contain plaintext")
	}
}

// TestEncryptForRecipient_MultipleSenderRecipientPairs verifies different key pairs work
func TestEncryptForRecipient_MultipleSenderRecipientPairs(t *testing.T) {
	// Generate multiple key pairs
	pairs := make([]struct {
		sender    *crypto.KeyPair
		recipient *crypto.KeyPair
	}, 5)

	for i := range pairs {
		senderKP, _ := crypto.GenerateKeyPair()
		recipientKP, _ := crypto.GenerateKeyPair()
		pairs[i].sender = senderKP
		pairs[i].recipient = recipientKP
	}

	message := []byte("Test message")

	// Test each pair
	for i, pair := range pairs {
		encryptedData, nonce, err := EncryptForRecipient(message, pair.recipient.Public, pair.sender.Private)
		if err != nil {
			t.Fatalf("Pair %d encryption failed: %v", i, err)
		}

		var cryptoNonce crypto.Nonce
		copy(cryptoNonce[:], nonce[:])
		decrypted, err := crypto.Decrypt(encryptedData, cryptoNonce, pair.sender.Public, pair.recipient.Private)
		if err != nil {
			t.Fatalf("Pair %d decryption failed: %v", i, err)
		}

		if !bytes.Equal(decrypted, message) {
			t.Errorf("Pair %d: decrypted message doesn't match", i)
		}
	}
}

// TestEncryptForRecipient_NoForwardSecrecy verifies lack of forward secrecy
// This test documents the security limitation of this function
func TestEncryptForRecipient_NoForwardSecrecy(t *testing.T) {
	senderKeyPair, _ := crypto.GenerateKeyPair()
	recipientKeyPair, _ := crypto.GenerateKeyPair()

	// Encrypt multiple messages with same keys
	messages := [][]byte{
		[]byte("Message 1"),
		[]byte("Message 2"),
		[]byte("Message 3"),
	}

	encrypted := make([]struct {
		data  []byte
		nonce [24]byte
	}, len(messages))

	for i, msg := range messages {
		data, nonce, err := EncryptForRecipient(msg, recipientKeyPair.Public, senderKeyPair.Private)
		if err != nil {
			t.Fatalf("Encryption %d failed: %v", i, err)
		}
		encrypted[i].data = data
		encrypted[i].nonce = nonce
	}

	// If sender key is compromised, all messages can be decrypted
	// This demonstrates the lack of forward secrecy
	for i, enc := range encrypted {
		var cryptoNonce crypto.Nonce
		copy(cryptoNonce[:], enc.nonce[:])
		decrypted, err := crypto.Decrypt(enc.data, cryptoNonce, senderKeyPair.Public, recipientKeyPair.Private)
		if err != nil {
			t.Fatalf("Message %d decryption failed: %v", i, err)
		}

		if !bytes.Equal(decrypted, messages[i]) {
			t.Errorf("Message %d doesn't match", i)
		}
	}

	// This test demonstrates that all messages remain decryptable with the same keys
	// In contrast, ForwardSecurityManager would use ephemeral pre-keys that can be deleted
	t.Log("WARNING: EncryptForRecipient does not provide forward secrecy")
	t.Log("All messages remain decryptable if keys are compromised")
	t.Log("Use ForwardSecurityManager for forward-secure messaging")
}
