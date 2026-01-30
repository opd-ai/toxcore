package crypto

import (
	"testing"

	"github.com/opd-ai/toxcore/limits"
)

// TestMaxEncryptionBufferDistinctFromMessageLimit verifies that the crypto
// layer's buffer size limit is distinct from the protocol message size limit.
func TestMaxEncryptionBufferDistinctFromMessageLimit(t *testing.T) {
	// The crypto layer MaxEncryptionBuffer should be 1MB for memory safety
	if MaxEncryptionBuffer != 1024*1024 {
		t.Errorf("MaxEncryptionBuffer = %d, want %d (1MB)", MaxEncryptionBuffer, 1024*1024)
	}

	// The protocol message size limit should be 1372 bytes (from limits package)
	if limits.MaxPlaintextMessage != 1372 {
		t.Errorf("limits.MaxPlaintextMessage = %d, want 1372", limits.MaxPlaintextMessage)
	}

	// Verify they are different values
	if MaxEncryptionBuffer == limits.MaxPlaintextMessage {
		t.Error("MaxEncryptionBuffer and limits.MaxPlaintextMessage should be different values")
	}

	// MaxEncryptionBuffer should be larger to allow for buffer operations
	if MaxEncryptionBuffer <= limits.MaxPlaintextMessage {
		t.Errorf("MaxEncryptionBuffer (%d) should be larger than MaxPlaintextMessage (%d)",
			MaxEncryptionBuffer, limits.MaxPlaintextMessage)
	}
}

// TestMaxEncryptionBufferMatchesProcessingBuffer verifies consistency with
// the limits package's MaxProcessingBuffer constant.
func TestMaxEncryptionBufferMatchesProcessingBuffer(t *testing.T) {
	// Both should represent the same 1MB limit for consistency
	if MaxEncryptionBuffer != limits.MaxProcessingBuffer {
		t.Errorf("MaxEncryptionBuffer (%d) != limits.MaxProcessingBuffer (%d), should be equal for consistency",
			MaxEncryptionBuffer, limits.MaxProcessingBuffer)
	}
}

// TestEncryptionBufferLimitEnforced verifies the buffer limit is enforced
// in encryption operations.
func TestEncryptionBufferLimitEnforced(t *testing.T) {
	senderKeys, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	recipientKeys, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Create a message exactly at the limit - should succeed
	atLimitMessage := make([]byte, MaxEncryptionBuffer)
	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatal(err)
	}

	_, err = Encrypt(atLimitMessage, nonce, recipientKeys.Public, senderKeys.Private)
	if err != nil {
		t.Errorf("Encryption at limit (%d bytes) should succeed, got error: %v",
			MaxEncryptionBuffer, err)
	}

	// Create a message over the limit - should fail
	overLimitMessage := make([]byte, MaxEncryptionBuffer+1)
	nonce, err = GenerateNonce()
	if err != nil {
		t.Fatal(err)
	}

	_, err = Encrypt(overLimitMessage, nonce, recipientKeys.Public, senderKeys.Private)
	if err == nil {
		t.Errorf("Encryption over limit (%d bytes) should fail", MaxEncryptionBuffer+1)
	}
	if err.Error() != "message too large" {
		t.Errorf("Expected error 'message too large', got: %v", err)
	}
}

// TestSymmetricEncryptionBufferLimitEnforced verifies the buffer limit is
// enforced in symmetric encryption operations.
func TestSymmetricEncryptionBufferLimitEnforced(t *testing.T) {
	key := [32]byte{1, 2, 3, 4, 5, 6, 7, 8}

	// Create a message exactly at the limit - should succeed
	atLimitMessage := make([]byte, MaxEncryptionBuffer)
	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatal(err)
	}

	_, err = EncryptSymmetric(atLimitMessage, nonce, key)
	if err != nil {
		t.Errorf("Symmetric encryption at limit (%d bytes) should succeed, got error: %v",
			MaxEncryptionBuffer, err)
	}

	// Create a message over the limit - should fail
	overLimitMessage := make([]byte, MaxEncryptionBuffer+1)
	nonce, err = GenerateNonce()
	if err != nil {
		t.Fatal(err)
	}

	_, err = EncryptSymmetric(overLimitMessage, nonce, key)
	if err == nil {
		t.Errorf("Symmetric encryption over limit (%d bytes) should fail", MaxEncryptionBuffer+1)
	}
	if err.Error() != "message too large" {
		t.Errorf("Expected error 'message too large', got: %v", err)
	}
}
