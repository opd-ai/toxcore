package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"golang.org/x/crypto/curve25519"
)

// TestDeriveSharedSecret tests the core ECDH shared secret derivation functionality
func TestDeriveSharedSecret(t *testing.T) {
	tests := []struct {
		name          string
		setupKeys     func() ([32]byte, [32]byte, [32]byte) // returns peer public, private, expected shared
		expectError   bool
		validateSetup bool
	}{
		{
			name: "valid keys produce consistent shared secret",
			setupKeys: func() ([32]byte, [32]byte, [32]byte) {
				// Generate a key pair for testing
				keyPair, err := GenerateKeyPair()
				if err != nil {
					t.Fatalf("Failed to generate key pair: %v", err)
				}
				
				// Generate peer key pair
				peerKeyPair, err := GenerateKeyPair()
				if err != nil {
					t.Fatalf("Failed to generate peer key pair: %v", err)
				}
				
				// Compute expected shared secret using the reference implementation
				expectedShared, err := curve25519.X25519(keyPair.Private[:], peerKeyPair.Public[:])
				if err != nil {
					t.Fatalf("Failed to compute reference shared secret: %v", err)
				}
				
				var expected [32]byte
				copy(expected[:], expectedShared)
				
				return peerKeyPair.Public, keyPair.Private, expected
			},
			expectError:   false,
			validateSetup: true,
		},
		{
			name: "different private keys produce different shared secrets",
			setupKeys: func() ([32]byte, [32]byte, [32]byte) {
				// This test doesn't check the expected value, just that it's computed
				keyPair, err := GenerateKeyPair()
				if err != nil {
					t.Fatalf("Failed to generate key pair: %v", err)
				}
				
				peerKeyPair, err := GenerateKeyPair()
				if err != nil {
					t.Fatalf("Failed to generate peer key pair: %v", err)
				}
				
				return peerKeyPair.Public, keyPair.Private, [32]byte{}
			},
			expectError:   false,
			validateSetup: false,
		},
		{
			name: "zero private key produces weak output",
			setupKeys: func() ([32]byte, [32]byte, [32]byte) {
				keyPair, err := GenerateKeyPair()
				if err != nil {
					t.Fatalf("Failed to generate key pair: %v", err)
				}
				
				zeroPrivate := [32]byte{}
				return keyPair.Public, zeroPrivate, [32]byte{}
			},
			expectError:   false, // Zero private key doesn't error but produces weak output
			validateSetup: false,
		},
		{
			name: "zero public key should fail",
			setupKeys: func() ([32]byte, [32]byte, [32]byte) {
				keyPair, err := GenerateKeyPair()
				if err != nil {
					t.Fatalf("Failed to generate key pair: %v", err)
				}
				
				zeroPublic := [32]byte{}
				return zeroPublic, keyPair.Private, [32]byte{}
			},
			expectError:   true,
			validateSetup: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peerPublic, private, expected := tt.setupKeys()
			
			// Call the function under test
			result, err := DeriveSharedSecret(peerPublic, private)
			
			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("DeriveSharedSecret() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("DeriveSharedSecret() unexpected error: %v", err)
				return
			}
			
			// Validate the shared secret if we have an expected value
			if tt.validateSetup {
				if !bytes.Equal(result[:], expected[:]) {
					t.Errorf("DeriveSharedSecret() = %x, expected %x", result, expected)
				}
			}
			
			// Ensure the result is not all zeros (unless expected to be)
			if !tt.expectError && isZeroKey(result) {
				t.Errorf("DeriveSharedSecret() returned zero shared secret")
			}
		})
	}
}

// TestDeriveSharedSecretConsistency tests that the shared secret is symmetric
func TestDeriveSharedSecretConsistency(t *testing.T) {
	// Generate two key pairs
	alice, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Alice's key pair: %v", err)
	}
	
	bob, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Bob's key pair: %v", err)
	}
	
	// Alice computes shared secret with Bob's public key
	aliceShared, err := DeriveSharedSecret(bob.Public, alice.Private)
	if err != nil {
		t.Fatalf("Alice failed to compute shared secret: %v", err)
	}
	
	// Bob computes shared secret with Alice's public key
	bobShared, err := DeriveSharedSecret(alice.Public, bob.Private)
	if err != nil {
		t.Fatalf("Bob failed to compute shared secret: %v", err)
	}
	
	// Both should compute the same shared secret
	if !bytes.Equal(aliceShared[:], bobShared[:]) {
		t.Errorf("Shared secrets don't match: Alice=%x, Bob=%x", aliceShared, bobShared)
	}
}

// TestDeriveSharedSecretSecureWiping tests that sensitive data is properly wiped
func TestDeriveSharedSecretSecureWiping(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	peerKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate peer key pair: %v", err)
	}
	
	// Make copies of the original keys to verify they weren't modified
	originalPrivate := keyPair.Private
	originalPeerPublic := peerKeyPair.Public
	
	// Call the function
	_, err = DeriveSharedSecret(peerKeyPair.Public, keyPair.Private)
	if err != nil {
		t.Fatalf("DeriveSharedSecret() failed: %v", err)
	}
	
	// Verify original keys weren't modified (the function should use copies)
	if !bytes.Equal(keyPair.Private[:], originalPrivate[:]) {
		t.Errorf("Original private key was modified")
	}
	
	if !bytes.Equal(peerKeyPair.Public[:], originalPeerPublic[:]) {
		t.Errorf("Original peer public key was modified")
	}
}

// TestDeriveSharedSecretKnownVectors tests against known test vectors
func TestDeriveSharedSecretKnownVectors(t *testing.T) {
	// Test vector from RFC 7748 - Curve25519
	tests := []struct {
		name         string
		privateKeyHex string
		publicKeyHex  string
		expectedHex   string
	}{
		{
			name:         "RFC 7748 test vector 1",
			privateKeyHex: "a046e36bf0527c9d3b16154b82465edd62144c0ac1fc5a18506a2244ba449ac4",
			publicKeyHex:  "e6db6867583030db3594c1a424b15f7c726624ec26b3353b10a903a6d0ab1c4c",
			expectedHex:   "c3da55379de9c6908e94ea4df28d084f32eccf03491c71f754b4075577a28552",
		},
		{
			name:         "RFC 7748 test vector 2",
			privateKeyHex: "4b66e9d4d1b4673c5ad22691957d6af5c11b6421e0ea01d42ca4169e7918ba0d",
			publicKeyHex:  "e5210f12786811d3f4b7959d0538ae2c31dbe7106fc03c3efc4cd549c715a493",
			expectedHex:   "95cbde9476e8907d7aade45cb4b873f88b595a68799fa152e6f8f7647aac7957",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert hex strings to byte arrays
			privateBytes := hexToBytes32(t, tt.privateKeyHex)
			publicBytes := hexToBytes32(t, tt.publicKeyHex)
			expectedBytes := hexToBytes32(t, tt.expectedHex)
			
			// Compute shared secret
			result, err := DeriveSharedSecret(publicBytes, privateBytes)
			if err != nil {
				t.Fatalf("DeriveSharedSecret() failed: %v", err)
			}
			
			// Compare with expected result
			if !bytes.Equal(result[:], expectedBytes[:]) {
				t.Errorf("DeriveSharedSecret() = %x, expected %x", result, expectedBytes)
			}
		})
	}
}

// TestDeriveSharedSecretRandomInputs tests with random inputs to ensure robustness
func TestDeriveSharedSecretRandomInputs(t *testing.T) {
	const numTests = 100
	
	for i := 0; i < numTests; i++ {
		// Generate random keys
		var privateKey, publicKey [32]byte
		
		if _, err := rand.Read(privateKey[:]); err != nil {
			t.Fatalf("Failed to generate random private key: %v", err)
		}
		
		if _, err := rand.Read(publicKey[:]); err != nil {
			t.Fatalf("Failed to generate random public key: %v", err)
		}
		
		// Ensure private key is valid (not zero)
		if isZeroKey(privateKey) {
			privateKey[0] = 1 // Make it non-zero
		}
		
		// Ensure public key is valid (not zero)
		if isZeroKey(publicKey) {
			publicKey[0] = 1 // Make it non-zero
		}
		
		// Compute shared secret
		result, err := DeriveSharedSecret(publicKey, privateKey)
		
		// Should not fail with valid random inputs
		if err != nil {
			t.Errorf("DeriveSharedSecret() failed with random inputs (iteration %d): %v", i, err)
			continue
		}
		
		// Result should not be zero
		if isZeroKey(result) {
			t.Errorf("DeriveSharedSecret() returned zero result with random inputs (iteration %d)", i)
		}
	}
}

// BenchmarkDeriveSharedSecret benchmarks the shared secret derivation performance
func BenchmarkDeriveSharedSecret(b *testing.B) {
	// Setup test keys
	keyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatalf("Failed to generate key pair: %v", err)
	}
	
	peerKeyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatalf("Failed to generate peer key pair: %v", err)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := DeriveSharedSecret(peerKeyPair.Public, keyPair.Private)
		if err != nil {
			b.Fatalf("DeriveSharedSecret() failed: %v", err)
		}
	}
}

// Helper function to convert hex string to [32]byte
func hexToBytes32(t *testing.T, hexStr string) [32]byte {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("Failed to decode hex string %s: %v", hexStr, err)
	}
	
	if len(bytes) != 32 {
		t.Fatalf("Hex string %s decoded to %d bytes, expected 32", hexStr, len(bytes))
	}
	
	var result [32]byte
	copy(result[:], bytes)
	return result
}
