package crypto

import (
	"fmt"
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

// TestSecureAllocate verifies that SecureAllocate returns a correctly-sized,
// zeroed buffer and that the buffer can be wiped without error.
func TestSecureAllocate(t *testing.T) {
	t.Parallel()

	// Zero size should return nil.
	if got := SecureAllocate(0); got != nil {
		t.Errorf("SecureAllocate(0): expected nil, got len=%d", len(got))
	}

	// Negative size should return nil.
	if got := SecureAllocate(-1); got != nil {
		t.Errorf("SecureAllocate(-1): expected nil, got len=%d", len(got))
	}

	// Normal allocation: correct length and zeroed.
	const size = 64
	buf := SecureAllocate(size)
	if len(buf) != size {
		t.Fatalf("SecureAllocate(%d): got len=%d", size, len(buf))
	}
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("SecureAllocate(%d): byte %d not zero (%02x)", size, i, b)
		}
	}

	// Write key material and wipe it.
	for i := range buf {
		buf[i] = byte(i + 1)
	}
	if err := SecureWipe(buf); err != nil {
		t.Fatalf("SecureWipe on SecureAllocate'd buffer: %v", err)
	}
	for i, b := range buf {
		if b != 0 {
			t.Errorf("SecureWipe: byte %d not zero after wipe (%02x)", i, b)
		}
	}

	// MlockAvailable is a compile-time constant; just ensure it is callable.
	_ = MlockAvailable()
}

func TestSecureWipeEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		expectErr bool
	}{
		{
			name:      "nil slice",
			input:     nil,
			expectErr: true,
		},
		{
			name:      "empty slice",
			input:     []byte{},
			expectErr: false,
		},
		{
			name:      "single byte",
			input:     []byte{0xFF},
			expectErr: false,
		},
		{
			name:      "large buffer",
			input:     make([]byte, 1024),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For non-nil test data, set non-zero values
			if tt.input != nil && len(tt.input) > 0 {
				for i := range tt.input {
					tt.input[i] = byte(i % 256)
				}
			}

			err := SecureWipe(tt.input)

			if tt.expectErr && err == nil {
				t.Errorf("Expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Verify data was zeroed (for non-nil, non-error cases)
			if !tt.expectErr && tt.input != nil {
				for i, b := range tt.input {
					if b != 0 {
						t.Errorf("Byte at position %d was not zeroed: got %d", i, b)
					}
				}
			}
		})
	}
}

// TestZeroBytesBufferLifetime verifies that a buffer deferred for wiping is
// actually cleared when the enclosing scope exits, simulating the pattern used
// in crypto functions to limit the lifetime of sensitive key material.
func TestZeroBytesBufferLifetime(t *testing.T) {
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = byte(i + 1) // non-zero fill
	}

	// Copy reference to check after wipe.
	ref := make([]byte, len(buf))
	copy(ref, buf)

	func() {
		defer ZeroBytes(buf)
		// Simulate using the sensitive buffer within a scope.
		_ = buf[0]
	}()

	// buf must be all zeros after the deferred wipe fires.
	for i, b := range buf {
		if b != 0 {
			t.Errorf("buf[%d] = %d after deferred ZeroBytes; want 0", i, b)
		}
	}
	// Confirm ref still has the original non-zero data (not aliased).
	if ref[0] == 0 {
		t.Error("ref unexpectedly zeroed — aliasing bug in test setup")
	}
}

// TestWipeKeyPairClearsOnlyPrivate verifies that WipeKeyPair clears the private
// key field without disturbing the public key.
func TestWipeKeyPairClearsOnlyPrivate(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	pubCopy := kp.Public

	if err := WipeKeyPair(kp); err != nil {
		t.Fatalf("WipeKeyPair: %v", err)
	}

	// Private key must be zeroed.
	for i, b := range kp.Private {
		if b != 0 {
			t.Errorf("Private[%d] = %d after WipeKeyPair; want 0", i, b)
		}
	}
	// Public key must be unchanged.
	if kp.Public != pubCopy {
		t.Error("WipeKeyPair altered the public key")
	}
}

// BenchmarkSecureWipe measures the throughput of SecureWipe for various buffer sizes.
// This ensures the compiler cannot optimize the wipe away and that performance is acceptable.
func BenchmarkSecureWipe(b *testing.B) {
	sizes := []int{32, 256, 1024, 4096}
	for _, size := range sizes {
		buf := make([]byte, size)
		for i := range buf {
			buf[i] = byte(i)
		}
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = SecureWipe(buf)
			}
		})
	}
}

// BenchmarkZeroBytes measures ZeroBytes for the sizes most common in key material.
func BenchmarkZeroBytes(b *testing.B) {
	buf := make([]byte, 32) // typical key size
	b.SetBytes(int64(len(buf)))
	for i := 0; i < b.N; i++ {
		ZeroBytes(buf)
	}
}
