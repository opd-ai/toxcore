package crypto

import (
	"testing"
)

// FuzzEncryptDecrypt fuzzes the encryption/decryption functions
func FuzzEncryptDecrypt(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("Hello, World!"))
	f.Add([]byte(""))
	f.Add(make([]byte, 100))

	f.Fuzz(func(t *testing.T, plaintext []byte) {
		// Generate random keypair
		sender, err := GenerateKeyPair()
		if err != nil {
			return
		}
		receiver, err := GenerateKeyPair()
		if err != nil {
			return
		}

		// Skip very large inputs to prevent OOM
		if len(plaintext) > 10000 {
			return
		}

		// Generate a random nonce
		var nonce Nonce
		// Attempt encryption - should not panic
		ciphertext, err := Encrypt(plaintext, nonce, receiver.Public, sender.Private)
		if err != nil {
			// Encryption can fail for valid reasons, just don't panic
			return
		}

		// Attempt decryption - should not panic
		decrypted, err := Decrypt(ciphertext, nonce, sender.Public, receiver.Private)
		if err != nil {
			// Decryption can fail, just verify no panic
			return
		}

		// If both succeeded, verify correctness
		if string(plaintext) != string(decrypted) {
			t.Errorf("Decryption mismatch: got %q, want %q", decrypted, plaintext)
		}
	})
}

// FuzzSharedSecret fuzzes the shared secret computation
func FuzzSharedSecret(f *testing.F) {
	// Add seed corpus with various key patterns
	validKey := make([]byte, 32)
	for i := range validKey {
		validKey[i] = byte(i)
	}
	f.Add(validKey)
	f.Add(make([]byte, 32)) // All zeros
	f.Add(make([]byte, 31)) // Too short
	f.Add(make([]byte, 33)) // Too long

	f.Fuzz(func(t *testing.T, keyData []byte) {
		// Attempt to create shared secret - should not panic
		if len(keyData) != 32 {
			return
		}

		var privKey, pubKey [32]byte
		copy(privKey[:], keyData)
		copy(pubKey[:], keyData)

		// This should not panic even with invalid keys
		_, _ = DeriveSharedSecret(pubKey, privKey)
	})
}

// FuzzSecureWipe fuzzes the secure memory wiping function
func FuzzSecureWipe(f *testing.F) {
	// Add seed corpus
	f.Add(make([]byte, 0))
	f.Add(make([]byte, 1))
	f.Add(make([]byte, 32))
	f.Add(make([]byte, 1024))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Make a copy since SecureWipe modifies in place
		testData := make([]byte, len(data))
		copy(testData, data)

		// Should not panic on any input
		_ = SecureWipe(testData)

		// Verify data was zeroed if non-nil
		if testData != nil {
			for i, b := range testData {
				if b != 0 {
					t.Errorf("Byte at index %d not zeroed: got %d", i, b)
				}
			}
		}
	})
}

// FuzzKeypairFromSecret fuzzes keypair derivation from secret
func FuzzKeypairFromSecret(f *testing.F) {
	// Add seed corpus
	validSecret := make([]byte, 32)
	for i := range validSecret {
		validSecret[i] = byte(i * 7 % 256)
	}

	f.Add(validSecret)
	f.Add(make([]byte, 32))

	f.Fuzz(func(t *testing.T, secretData []byte) {
		if len(secretData) != 32 {
			return
		}

		var secret [32]byte
		copy(secret[:], secretData)

		// Should not panic
		kp, err := FromSecretKey(secret)
		if err != nil {
			return
		}

		// Verify keypair properties
		if kp == nil {
			t.Error("FromSecretKey returned nil keypair without error")
		}
	})
}

// FuzzNonceHandling fuzzes nonce generation and handling
func FuzzNonceHandling(f *testing.F) {
	// Add seed corpus
	validNonce := make([]byte, 24)
	f.Add(validNonce)
	f.Add(make([]byte, 24))

	f.Fuzz(func(t *testing.T, nonceData []byte) {
		if len(nonceData) != 24 {
			return
		}

		var nonce Nonce
		copy(nonce[:], nonceData)

		// Generate keypair
		kp, err := GenerateKeyPair()
		if err != nil {
			return
		}

		// Test encryption with fuzzed nonce - should not panic
		message := []byte("test")
		_, _ = Encrypt(message, nonce, kp.Public, kp.Private)
	})
}
