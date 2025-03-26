package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	if keyPair == nil {
		t.Fatal("GenerateKeyPair() returned nil key pair")
	}

	// Check that keys are not zero
	if isZeroKey(keyPair.Public) {
		t.Error("GenerateKeyPair() returned zero public key")
	}

	if isZeroKey(keyPair.Private) {
		t.Error("GenerateKeyPair() returned zero private key")
	}

	// Test that multiple key generations produce different keys
	keyPair2, _ := GenerateKeyPair()
	if bytes.Equal(keyPair.Public[:], keyPair2.Public[:]) {
		t.Error("Multiple GenerateKeyPair() calls produced identical public keys")
	}
}

func TestFromSecretKey(t *testing.T) {
	cases := []struct {
		name      string
		secretKey [32]byte
		wantError bool
	}{
		{
			name:      "Valid key",
			secretKey: [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			wantError: false,
		},
		{
			name:      "Zero key",
			secretKey: [32]byte{},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keyPair, err := FromSecretKey(tc.secretKey)

			if tc.wantError && err == nil {
				t.Fatal("FromSecretKey() expected error but got nil")
			}

			if !tc.wantError {
				if err != nil {
					t.Fatalf("FromSecretKey() unexpected error: %v", err)
				}

				if keyPair == nil {
					t.Fatal("FromSecretKey() returned nil key pair")
				}

				if bytes.Equal(keyPair.Public[:], make([]byte, 32)) {
					t.Error("FromSecretKey() returned zero public key")
				}

				// Check that private key matches input
				if !bytes.Equal(keyPair.Private[:], tc.secretKey[:]) {
					t.Error("FromSecretKey() modified the private key")
				}
			}
		})
	}
}

func TestGenerateNonce(t *testing.T) {
	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce() error: %v", err)
	}

	// Check nonce is not zero
	zeroNonce := Nonce{}
	if bytes.Equal(nonce[:], zeroNonce[:]) {
		t.Error("GenerateNonce() returned zero nonce")
	}

	// Test multiple nonce generations produce different values
	nonce2, _ := GenerateNonce()
	if bytes.Equal(nonce[:], nonce2[:]) {
		t.Error("Multiple GenerateNonce() calls produced identical nonces")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	// Generate key pairs for sender and recipient
	senderKeys, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeys, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Generate a nonce
	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}

	testCases := []struct {
		name      string
		message   []byte
		expectErr bool
	}{
		{"Normal message", []byte("Hello, this is a test message!"), false},
		{"Empty message", []byte{}, true},
		{"Binary data", []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD}, false},
		{"Long message", bytes.Repeat([]byte("A"), 1024), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt message
			ciphertext, err := Encrypt(tc.message, nonce, recipientKeys.Public, senderKeys.Private)

			if tc.expectErr {
				if err == nil {
					t.Fatal("Expected encryption error, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Encrypt() error: %v", err)
			}

			// Decrypt message
			decrypted, err := Decrypt(ciphertext, nonce, senderKeys.Public, recipientKeys.Private)
			if err != nil {
				t.Fatalf("Decrypt() error: %v", err)
			}

			// Check if decryption result matches original
			if !bytes.Equal(tc.message, decrypted) {
				t.Errorf("Decrypted message doesn't match original. Got: %v, want: %v", decrypted, tc.message)
			}
		})
	}

	// Test decryption failures
	t.Run("Decryption failures", func(t *testing.T) {
		validMsg := []byte("Valid message")
		ciphertext, _ := Encrypt(validMsg, nonce, recipientKeys.Public, senderKeys.Private)

		// Tamper with ciphertext
		if len(ciphertext) > 0 {
			tamperedCiphertext := make([]byte, len(ciphertext))
			copy(tamperedCiphertext, ciphertext)
			tamperedCiphertext[0] ^= 0xFF

			_, err := Decrypt(tamperedCiphertext, nonce, senderKeys.Public, recipientKeys.Private)
			if err == nil {
				t.Error("Decrypt() with tampered ciphertext should fail")
			}
		}

		// Empty ciphertext
		_, err := Decrypt([]byte{}, nonce, senderKeys.Public, recipientKeys.Private)
		if err == nil {
			t.Error("Decrypt() with empty ciphertext should fail")
		}
	})
}

func TestEncryptDecryptSymmetric(t *testing.T) {
	// Generate a symmetric key (using 32 bytes as per NaCl's secretbox)
	var key [32]byte
	for i := range key {
		key[i] = byte(i)
	}

	// Generate a nonce
	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}

	testCases := []struct {
		name      string
		message   []byte
		expectErr bool
	}{
		{"Normal message", []byte("Hello, this is a test message!"), false},
		{"Empty message", []byte{}, true},
		{"Binary data", []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD}, false},
		{"Long message", bytes.Repeat([]byte("A"), 1024), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt message
			ciphertext, err := EncryptSymmetric(tc.message, nonce, key)

			if tc.expectErr {
				if err == nil {
					t.Fatal("Expected encryption error, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("EncryptSymmetric() error: %v", err)
			}

			// Decrypt message
			decrypted, err := DecryptSymmetric(ciphertext, nonce, key)
			if err != nil {
				t.Fatalf("DecryptSymmetric() error: %v", err)
			}

			// Check if decryption result matches original
			if !bytes.Equal(tc.message, decrypted) {
				t.Errorf("Decrypted message doesn't match original. Got: %v, want: %v", decrypted, tc.message)
			}
		})
	}

	// Test decryption failures
	t.Run("Symmetric decryption failures", func(t *testing.T) {
		validMsg := []byte("Valid message")
		ciphertext, _ := EncryptSymmetric(validMsg, nonce, key)

		// Tamper with ciphertext
		if len(ciphertext) > 0 {
			tamperedCiphertext := make([]byte, len(ciphertext))
			copy(tamperedCiphertext, ciphertext)
			tamperedCiphertext[0] ^= 0xFF

			_, err := DecryptSymmetric(tamperedCiphertext, nonce, key)
			if err == nil {
				t.Error("DecryptSymmetric() with tampered ciphertext should fail")
			}
		}

		// Empty ciphertext
		_, err := DecryptSymmetric([]byte{}, nonce, key)
		if err == nil {
			t.Error("DecryptSymmetric() with empty ciphertext should fail")
		}
	})
}

func TestToxID(t *testing.T) {
	// Create test public key and nospam
	var publicKey [32]byte
	var nospam [4]byte

	for i := range publicKey {
		publicKey[i] = byte(i)
	}

	for i := range nospam {
		nospam[i] = byte(i + 100)
	}

	t.Run("Create ToxID", func(t *testing.T) {
		toxID := NewToxID(publicKey, nospam)

		if toxID == nil {
			t.Fatal("NewToxID() returned nil")
		}

		if !bytes.Equal(toxID.PublicKey[:], publicKey[:]) {
			t.Error("ToxID public key doesn't match input")
		}

		if !bytes.Equal(toxID.Nospam[:], nospam[:]) {
			t.Error("ToxID nospam value doesn't match input")
		}
	})

	t.Run("String serialization", func(t *testing.T) {
		toxID := NewToxID(publicKey, nospam)
		idStr := toxID.String()

		// Check length: 32 (public key) + 4 (nospam) + 2 (checksum) = 38 bytes = 76 hex chars
		if len(idStr) != 76 {
			t.Errorf("ToxID string has wrong length: got %d, want 76", len(idStr))
		}

		// Parse back
		parsedID, err := ToxIDFromString(idStr)
		if err != nil {
			t.Fatalf("ToxIDFromString() error: %v", err)
		}

		// Check fields match
		if !bytes.Equal(parsedID.PublicKey[:], toxID.PublicKey[:]) {
			t.Error("Parsed ToxID public key doesn't match original")
		}

		if !bytes.Equal(parsedID.Nospam[:], toxID.Nospam[:]) {
			t.Error("Parsed ToxID nospam value doesn't match original")
		}

		if !bytes.Equal(parsedID.Checksum[:], toxID.Checksum[:]) {
			t.Error("Parsed ToxID checksum doesn't match original")
		}
	})

	t.Run("Invalid ToxID strings", func(t *testing.T) {
		testCases := []struct {
			name    string
			idStr   string
			wantErr bool
		}{
			{"Too short", "abcdef", true},
			{"Too long", hex.EncodeToString(bytes.Repeat([]byte{1}, 39)), true},
			{"Invalid hex", "Z0000000000000000000000000000000000000000000000000000000000000000000000000", true},
			{"Invalid checksum", hex.EncodeToString(bytes.Repeat([]byte{1}, 38)), true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := ToxIDFromString(tc.idStr)
				if !tc.wantErr && err != nil {
					t.Errorf("ToxIDFromString() unexpected error: %v", err)
				}
				if tc.wantErr && err == nil {
					t.Error("ToxIDFromString() expected error but got nil")
				}
			})
		}
	})
}

func TestSignAndVerify(t *testing.T) {
	// Generate a key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	testCases := []struct {
		name      string
		message   []byte
		expectErr bool
	}{
		{"Normal message", []byte("Test message to sign"), false},
		{"Empty message", []byte{}, true},
		{"Binary data", []byte{0x00, 0x01, 0x02, 0x03, 0xFF}, false},
		{"Long message", bytes.Repeat([]byte("A"), 1024), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Sign the message
			signature, err := Sign(tc.message, keyPair.Private)

			if tc.expectErr {
				if err == nil {
					t.Fatal("Expected signing error, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Sign() error: %v", err)
			}

			// Verify the signature
			valid, err := Verify(tc.message, signature, keyPair.Public)
			if err != nil {
				t.Fatalf("Verify() error: %v", err)
			}

			if !valid {
				t.Error("Signature verification failed")
			}

			// Test verification with tampered message
			if len(tc.message) > 0 {
				tamperedMsg := make([]byte, len(tc.message))
				copy(tamperedMsg, tc.message)
				tamperedMsg[0] ^= 0xFF

				valid, _ := Verify(tamperedMsg, signature, keyPair.Public)
				if valid {
					t.Error("Verification should fail with tampered message")
				}
			}
		})
	}
}
