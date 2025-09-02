package crypto

import (
	"testing"
)

// BenchmarkGenerateKeyPair measures key pair generation performance
func BenchmarkGenerateKeyPair(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateKeyPair()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGenerateNonce measures nonce generation performance
func BenchmarkGenerateNonce(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateNonce()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGenerateNospam measures nospam generation performance
func BenchmarkGenerateNospam(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateNospam()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncrypt measures encryption performance
func BenchmarkEncrypt(b *testing.B) {
	// Setup test keys and data
	senderKeyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	receiverKeyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	message := []byte("This is a benchmark test message for encryption performance")
	nonce, err := GenerateNonce()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encrypt(message, nonce, receiverKeyPair.Public, senderKeyPair.Private)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecrypt measures decryption performance
func BenchmarkDecrypt(b *testing.B) {
	// Setup test keys and data
	senderKeyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	receiverKeyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	message := []byte("This is a benchmark test message for decryption performance")
	nonce, err := GenerateNonce()
	if err != nil {
		b.Fatal(err)
	}

	// Pre-encrypt the message
	ciphertext, err := Encrypt(message, nonce, receiverKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Decrypt(ciphertext, nonce, senderKeyPair.Public, receiverKeyPair.Private)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncryptSymmetric measures symmetric encryption performance
func BenchmarkEncryptSymmetric(b *testing.B) {
	// Generate a symmetric key
	keyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	key := keyPair.Private // Use private key as symmetric key

	message := []byte("This is a benchmark test message for symmetric encryption performance")
	nonce, err := GenerateNonce()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncryptSymmetric(message, nonce, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecryptSymmetric measures symmetric decryption performance
func BenchmarkDecryptSymmetric(b *testing.B) {
	// Generate a symmetric key
	keyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	key := keyPair.Private // Use private key as symmetric key

	message := []byte("This is a benchmark test message for symmetric decryption performance")
	nonce, err := GenerateNonce()
	if err != nil {
		b.Fatal(err)
	}

	// Pre-encrypt the message
	ciphertext, err := EncryptSymmetric(message, nonce, key)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DecryptSymmetric(ciphertext, nonce, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSign measures digital signature performance
func BenchmarkSign(b *testing.B) {
	// Generate signing key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	message := []byte("This is a benchmark test message for signing performance")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Sign(message, keyPair.Private)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVerify measures signature verification performance
func BenchmarkVerify(b *testing.B) {
	// Generate signing key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	message := []byte("This is a benchmark test message for verification performance")

	// Pre-sign the message
	signature, err := Sign(message, keyPair.Private)
	if err != nil {
		b.Fatal(err)
	}

	// Get the verification public key
	verifyPublicKey := GetSignaturePublicKey(keyPair.Private)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Verify(message, signature, verifyPublicKey)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkToxIDFromString measures ToxID parsing performance
func BenchmarkToxIDFromString(b *testing.B) {
	// Create a valid ToxID string for benchmarking
	keyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	nospam, err := GenerateNospam()
	if err != nil {
		b.Fatal(err)
	}

	toxID := NewToxID(keyPair.Public, nospam)
	toxIDString := toxID.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ToxIDFromString(toxIDString)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkToxIDString measures ToxID string serialization performance
func BenchmarkToxIDString(b *testing.B) {
	// Create a ToxID for benchmarking
	keyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	nospam, err := GenerateNospam()
	if err != nil {
		b.Fatal(err)
	}

	toxID := NewToxID(keyPair.Public, nospam)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = toxID.String()
	}
}
