package crypto

import (
	"testing"
)

func TestSecureMemoryHandling(t *testing.T) {
	// Generate a key pair
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create a copy of the private key to test zeroing
	var privateCopy [32]byte
	copy(privateCopy[:], kp.Private[:])

	// Verify that the private key has non-zero data initially
	allZeroInitially := true
	for _, b := range kp.Private {
		if b != 0 {
			allZeroInitially = false
			break
		}
	}

	if allZeroInitially {
		t.Fatalf("Private key is all zeros before wiping, test cannot proceed")
	}

	// Test SecureWipe function
	err = SecureWipe(kp.Private[:])
	if err != nil {
		t.Fatalf("SecureWipe failed: %v", err)
	}

	// Check if the private key was zeroed
	allZeroAfterWipe := true
	for _, b := range kp.Private {
		if b != 0 {
			allZeroAfterWipe = false
			break
		}
	}

	if !allZeroAfterWipe {
		t.Fatalf("Private key data was not securely wiped by SecureWipe")
	}

	// Test WipeKeyPair function
	kp2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate second keypair: %v", err)
	}

	err = WipeKeyPair(kp2)
	if err != nil {
		t.Fatalf("WipeKeyPair failed: %v", err)
	}

	// Check if the private key was zeroed
	allZero := true
	for _, b := range kp2.Private {
		if b != 0 {
			allZero = false
			break
		}
	}

	if !allZero {
		t.Fatalf("Private key data was not securely wiped by WipeKeyPair")
	}

	// Test ZeroBytes function
	testData := []byte{1, 2, 3, 4, 5}
	ZeroBytes(testData)

	for i, b := range testData {
		if b != 0 {
			t.Fatalf("ZeroBytes failed to zero byte at position %d", i)
		}
	}

	// Verify that original copy is different from zeroed version
	sameAsOriginal := true
	for i, b := range privateCopy {
		if b != kp.Private[i] {
			sameAsOriginal = false
			break
		}
	}

	if sameAsOriginal {
		t.Fatalf("Private key data was not changed after wiping")
	}
}
