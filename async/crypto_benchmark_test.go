package async

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// BenchmarkRecipientPseudonymGeneration benchmarks recipient pseudonym generation
func BenchmarkRecipientPseudonymGeneration(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	currentEpoch := epochManager.GetCurrentEpoch()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := obfuscationManager.GenerateRecipientPseudonym(recipientKeyPair.Public, currentEpoch)
		if err != nil {
			b.Fatal("Failed to generate recipient pseudonym:", err)
		}
	}
}

// BenchmarkSenderPseudonymGeneration benchmarks sender pseudonym generation
func BenchmarkSenderPseudonymGeneration(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	// Generate test nonce
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		b.Fatal("Failed to generate nonce:", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := obfuscationManager.GenerateSenderPseudonym(keyPair.Private, recipientKeyPair.Public, nonce)
		if err != nil {
			b.Fatal("Failed to generate sender pseudonym:", err)
		}
	}
}

// BenchmarkSharedSecretDerivation benchmarks ECDH shared secret computation
func BenchmarkSharedSecretDerivation(b *testing.B) {
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	mockTransport := &MockTransport{}
	client := NewAsyncClient(senderKeyPair, mockTransport)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.deriveSharedSecret(recipientKeyPair.Public)
		if err != nil {
			b.Fatal("Failed to derive shared secret:", err)
		}
	}
}

// BenchmarkPayloadEncryptionAES benchmarks AES-GCM payload encryption
func BenchmarkPayloadEncryptionAES(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	// Create test payload
	payload := []byte("test payload for encryption benchmarking - this is a realistic message size for async messaging performance testing")

	// Generate test key
	var encKey [32]byte
	copy(encKey[:], "test-encryption-key-for-bench123")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, _, err := obfuscationManager.EncryptPayload(payload, encKey)
		if err != nil {
			b.Fatal("Failed to encrypt payload:", err)
		}
	}
}

// BenchmarkPayloadDecryptionAES benchmarks AES-GCM payload decryption
func BenchmarkPayloadDecryptionAES(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	// Create test payload
	payload := []byte("test payload for decryption benchmarking - this is a realistic message size for async messaging performance testing")

	// Generate test key
	var encKey [32]byte
	copy(encKey[:], "test-encryption-key-for-bench123")

	// Pre-encrypt the payload
	encryptedPayload, nonce, tag, err := obfuscationManager.EncryptPayload(payload, encKey)
	if err != nil {
		b.Fatal("Failed to encrypt payload for test:", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := obfuscationManager.DecryptPayload(encryptedPayload, nonce, tag, encKey)
		if err != nil {
			b.Fatal("Failed to decrypt payload:", err)
		}
	}
}

// BenchmarkRecipientProofGeneration benchmarks HMAC-based recipient proof generation
func BenchmarkRecipientProofGeneration(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	// Test data
	var messageID [32]byte
	copy(messageID[:], "test-message-id-for-benchmarking")
	currentEpoch := epochManager.GetCurrentEpoch()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := obfuscationManager.GenerateRecipientProof(recipientKeyPair.Public, messageID, currentEpoch)
		if err != nil {
			b.Fatal("Failed to generate recipient proof:", err)
		}
	}
}

func BenchmarkCryptoGenerateSenderPseudonym(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	// Generate test nonce
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		b.Fatal("Failed to generate nonce:", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := obfuscationManager.GenerateSenderPseudonym(keyPair.Private, recipientKeyPair.Public, nonce)
		if err != nil {
			b.Fatal("Failed to generate sender pseudonym:", err)
		}
	}
}

// BenchmarkSharedSecretDerivation benchmarks ECDH shared secret computation
func BenchmarkDeriveSharedSecret(b *testing.B) {
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	mockTransport := &MockTransport{}
	client := NewAsyncClient(senderKeyPair, mockTransport)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.deriveSharedSecret(recipientKeyPair.Public)
		if err != nil {
			b.Fatal("Failed to derive shared secret:", err)
		}
	}
}

// BenchmarkCryptoPayloadEncryption benchmarks AES-GCM payload encryption
func BenchmarkCryptoEncryptPayload(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	// Create test payload
	payload := []byte("test payload for encryption benchmarking - this is a realistic message size for async messaging performance testing")

	// Generate test key
	var encKey [32]byte
	copy(encKey[:], "test-encryption-key-for-bench123")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, _, err := obfuscationManager.EncryptPayload(payload, encKey)
		if err != nil {
			b.Fatal("Failed to encrypt payload:", err)
		}
	}
}

// BenchmarkPayloadDecryption benchmarks AES-GCM payload decryption
func BenchmarkDecryptPayload(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	// Create test payload
	payload := []byte("test payload for decryption benchmarking - this is a realistic message size for async messaging performance testing")

	// Generate test key
	var encKey [32]byte
	copy(encKey[:], "test-encryption-key-for-bench123")

	// Pre-encrypt the payload
	encryptedPayload, nonce, tag, err := obfuscationManager.EncryptPayload(payload, encKey)
	if err != nil {
		b.Fatal("Failed to encrypt payload for test:", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := obfuscationManager.DecryptPayload(encryptedPayload, nonce, tag, encKey)
		if err != nil {
			b.Fatal("Failed to decrypt payload:", err)
		}
	}
}

// BenchmarkRecipientProofGeneration benchmarks HMAC-based recipient proof generation
func BenchmarkGenerateRecipientProof(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	// Test data
	var messageID [32]byte
	copy(messageID[:], "test-message-id-for-benchmarking")
	currentEpoch := epochManager.GetCurrentEpoch()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := obfuscationManager.GenerateRecipientProof(recipientKeyPair.Public, messageID, currentEpoch)
		if err != nil {
			b.Fatal("Failed to generate recipient proof:", err)
		}
	}
}

// BenchmarkRecipientProofValidation benchmarks HMAC-based recipient proof validation
func BenchmarkValidateRecipientProof(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(keyPair, epochManager)

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	// Test data
	var messageID [32]byte
	copy(messageID[:], "test-message-id-for-benchmarking")
	currentEpoch := epochManager.GetCurrentEpoch()

	// Pre-generate proof to avoid errors
	_, err = obfuscationManager.GenerateRecipientProof(recipientKeyPair.Public, messageID, currentEpoch)
	if err != nil {
		b.Fatal("Failed to generate recipient proof:", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Benchmark epoch checking
		_ = epochManager.GetCurrentEpoch()
	}
}
