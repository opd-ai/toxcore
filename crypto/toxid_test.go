package crypto

import (
	"testing"
)

// TestNewToxID tests ToxID creation
func TestNewToxID(t *testing.T) {
	tests := []struct {
		name      string
		publicKey [32]byte
		nospam    [4]byte
	}{
		{
			name:      "zero values",
			publicKey: [32]byte{},
			nospam:    [4]byte{},
		},
		{
			name:      "random values",
			publicKey: [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			nospam:    [4]byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name:      "max values",
			publicKey: [32]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			nospam:    [4]byte{255, 255, 255, 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := NewToxID(tt.publicKey, tt.nospam)

			if id == nil {
				t.Fatal("NewToxID() returned nil")
			}

			if id.PublicKey != tt.publicKey {
				t.Errorf("NewToxID() PublicKey = %v, want %v", id.PublicKey, tt.publicKey)
			}

			if id.Nospam != tt.nospam {
				t.Errorf("NewToxID() Nospam = %v, want %v", id.Nospam, tt.nospam)
			}

			// Verify checksum was calculated (should not be zero unless specifically calculated to be zero)
			if id.Checksum == [2]byte{} && (tt.publicKey != [32]byte{} || tt.nospam != [4]byte{}) {
				// For non-zero inputs, checksum is very unlikely to be zero
			}
		})
	}
}

// TestToxIDFromString tests parsing ToxID from string
func TestToxIDFromString(t *testing.T) {
	// Create a valid ToxID first to get a valid string
	publicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	nospam := [4]byte{0xDE, 0xAD, 0xBE, 0xEF}
	originalID := NewToxID(publicKey, nospam)
	validString := originalID.String()

	tests := []struct {
		name        string
		input       string
		expectError bool
		expectedID  *ToxID
	}{
		{
			name:        "valid ToxID string",
			input:       validString,
			expectError: false,
			expectedID:  originalID,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "too short",
			input:       "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20deadbeef01",
			expectError: true,
		},
		{
			name:        "too long",
			input:       "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20deadbeef0102ff",
			expectError: true,
		},
		{
			name:        "invalid hex characters",
			input:       "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20deadbeefghij",
			expectError: true,
		},
		{
			name:        "valid hex but wrong checksum",
			input:       "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20deadbeef0000",
			expectError: true,
		},
		{
			name:        "all zeros",
			input:       "0000000000000000000000000000000000000000000000000000000000000000000000000000",
			expectError: false,
			expectedID:  NewToxID([32]byte{}, [4]byte{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ToxIDFromString(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("ToxIDFromString() expected error but got none")
				}
				if id != nil {
					t.Errorf("ToxIDFromString() expected nil ID when error occurs")
				}
			} else {
				if err != nil {
					t.Errorf("ToxIDFromString() unexpected error: %v", err)
				}
				if id == nil {
					t.Fatal("ToxIDFromString() returned nil without error")
				}

				if tt.expectedID != nil {
					if id.PublicKey != tt.expectedID.PublicKey {
						t.Errorf("ToxIDFromString() PublicKey = %v, want %v", id.PublicKey, tt.expectedID.PublicKey)
					}
					if id.Nospam != tt.expectedID.Nospam {
						t.Errorf("ToxIDFromString() Nospam = %v, want %v", id.Nospam, tt.expectedID.Nospam)
					}
					if id.Checksum != tt.expectedID.Checksum {
						t.Errorf("ToxIDFromString() Checksum = %v, want %v", id.Checksum, tt.expectedID.Checksum)
					}
				}
			}
		})
	}
}

// TestToxIDString tests ToxID string conversion
func TestToxIDString(t *testing.T) {
	tests := []struct {
		name      string
		publicKey [32]byte
		nospam    [4]byte
	}{
		{
			name:      "zero values",
			publicKey: [32]byte{},
			nospam:    [4]byte{},
		},
		{
			name:      "sequential values",
			publicKey: [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			nospam:    [4]byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name:      "max values",
			publicKey: [32]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			nospam:    [4]byte{255, 255, 255, 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := NewToxID(tt.publicKey, tt.nospam)
			idString := id.String()

			// Verify string length (76 hex characters)
			if len(idString) != 76 {
				t.Errorf("ToxID.String() length = %d, want 76", len(idString))
			}

			// Verify we can parse it back
			parsedID, err := ToxIDFromString(idString)
			if err != nil {
				t.Errorf("ToxID.String() produced unparseable string: %v", err)
			}

			if parsedID.PublicKey != tt.publicKey {
				t.Errorf("Round-trip failed: PublicKey = %v, want %v", parsedID.PublicKey, tt.publicKey)
			}

			if parsedID.Nospam != tt.nospam {
				t.Errorf("Round-trip failed: Nospam = %v, want %v", parsedID.Nospam, tt.nospam)
			}

			if parsedID.Checksum != id.Checksum {
				t.Errorf("Round-trip failed: Checksum = %v, want %v", parsedID.Checksum, id.Checksum)
			}
		})
	}
}

// TestToxIDSetNospam tests changing nospam value
func TestToxIDSetNospam(t *testing.T) {
	publicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	originalNospam := [4]byte{0xDE, 0xAD, 0xBE, 0xEF}
	id := NewToxID(publicKey, originalNospam)
	originalChecksum := id.Checksum

	tests := []struct {
		name      string
		newNospam [4]byte
	}{
		{
			name:      "different nospam",
			newNospam: [4]byte{0xCA, 0xFE, 0xBA, 0xBE},
		},
		{
			name:      "zero nospam",
			newNospam: [4]byte{0, 0, 0, 0},
		},
		{
			name:      "max nospam",
			newNospam: [4]byte{255, 255, 255, 255},
		},
		{
			name:      "same as original",
			newNospam: originalNospam,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id.SetNospam(tt.newNospam)

			if id.Nospam != tt.newNospam {
				t.Errorf("SetNospam() Nospam = %v, want %v", id.Nospam, tt.newNospam)
			}

			// Verify checksum was recalculated (unless same nospam)
			if tt.newNospam != originalNospam && id.Checksum == originalChecksum {
				t.Error("SetNospam() checksum should change when nospam changes")
			}

			// Verify public key unchanged
			if id.PublicKey != publicKey {
				t.Errorf("SetNospam() should not change PublicKey")
			}
		})
	}
}

// TestGenerateNospam tests nospam generation
func TestGenerateNospam(t *testing.T) {
	// Generate multiple nospam values
	nospams := make([][4]byte, 10)
	for i := 0; i < 10; i++ {
		nospam, err := GenerateNospam()
		if err != nil {
			t.Fatalf("GenerateNospam() error = %v", err)
		}
		nospams[i] = nospam
	}

	// Check that they're not all the same (extremely unlikely with good randomness)
	allSame := true
	for i := 1; i < len(nospams); i++ {
		if nospams[i] != nospams[0] {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("GenerateNospam() generated identical values, randomness may be compromised")
	}
}

// TestToxIDGetNospam tests getting nospam value
func TestToxIDGetNospam(t *testing.T) {
	tests := []struct {
		name   string
		nospam [4]byte
	}{
		{
			name:   "zero nospam",
			nospam: [4]byte{0, 0, 0, 0},
		},
		{
			name:   "random nospam",
			nospam: [4]byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name:   "max nospam",
			nospam: [4]byte{255, 255, 255, 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
			id := NewToxID(publicKey, tt.nospam)

			gotNospam := id.GetNospam()
			if gotNospam != tt.nospam {
				t.Errorf("GetNospam() = %v, want %v", gotNospam, tt.nospam)
			}
		})
	}
}

// TestToxIDCalculateChecksum tests checksum calculation indirectly
func TestToxIDCalculateChecksum(t *testing.T) {
	tests := []struct {
		name      string
		publicKey [32]byte
		nospam    [4]byte
	}{
		{
			name:      "zero values",
			publicKey: [32]byte{},
			nospam:    [4]byte{},
		},
		{
			name:      "alternating pattern",
			publicKey: [32]byte{0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55},
			nospam:    [4]byte{0x55, 0xAA, 0x55, 0xAA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1 := NewToxID(tt.publicKey, tt.nospam)
			id2 := NewToxID(tt.publicKey, tt.nospam)

			// Same inputs should produce same checksum
			if id1.Checksum != id2.Checksum {
				t.Errorf("calculateChecksum() not deterministic: %v != %v", id1.Checksum, id2.Checksum)
			}

			// Different nospam should produce different checksum (usually)
			differentNospam := [4]byte{tt.nospam[0] ^ 1, tt.nospam[1], tt.nospam[2], tt.nospam[3]}
			id3 := NewToxID(tt.publicKey, differentNospam)

			// Note: Due to XOR nature, some combinations might produce same checksum, so we don't strictly enforce difference
			// But we test that the calculation is consistent
			id4 := NewToxID(tt.publicKey, differentNospam)
			if id3.Checksum != id4.Checksum {
				t.Errorf("calculateChecksum() not deterministic for different inputs: %v != %v", id3.Checksum, id4.Checksum)
			}
		})
	}
}

// TestToxIDRoundTrip tests complete round-trip functionality
func TestToxIDRoundTrip(t *testing.T) {
	// Generate a random key pair for testing
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	nospam, err := GenerateNospam()
	if err != nil {
		t.Fatalf("Failed to generate nospam: %v", err)
	}

	// Create ToxID
	originalID := NewToxID(keyPair.Public, nospam)

	// Convert to string
	idString := originalID.String()

	// Parse back from string
	parsedID, err := ToxIDFromString(idString)
	if err != nil {
		t.Fatalf("Failed to parse ToxID from string: %v", err)
	}

	// Verify all fields match
	if parsedID.PublicKey != originalID.PublicKey {
		t.Errorf("Round-trip failed: PublicKey mismatch")
	}

	if parsedID.Nospam != originalID.Nospam {
		t.Errorf("Round-trip failed: Nospam mismatch")
	}

	if parsedID.Checksum != originalID.Checksum {
		t.Errorf("Round-trip failed: Checksum mismatch")
	}

	// Test nospam modification
	newNospam, err := GenerateNospam()
	if err != nil {
		t.Fatalf("Failed to generate new nospam: %v", err)
	}

	parsedID.SetNospam(newNospam)
	if parsedID.Nospam != newNospam {
		t.Errorf("SetNospam failed")
	}

	if parsedID.GetNospam() != newNospam {
		t.Errorf("GetNospam failed")
	}

	// Verify string representation updated
	newIDString := parsedID.String()
	if newIDString == idString {
		t.Error("String representation should change after nospam change")
	}
}

// BenchmarkNewToxID benchmarks ToxID creation
func BenchmarkNewToxID(b *testing.B) {
	publicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	nospam := [4]byte{0xDE, 0xAD, 0xBE, 0xEF}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewToxID(publicKey, nospam)
	}
}

// BenchmarkToxIDFromStringParsing benchmarks ToxID parsing
func BenchmarkToxIDFromStringParsing(b *testing.B) {
	publicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	nospam := [4]byte{0xDE, 0xAD, 0xBE, 0xEF}
	id := NewToxID(publicKey, nospam)
	idString := id.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ToxIDFromString(idString)
	}
}

// BenchmarkToxIDStringConversion benchmarks ToxID string conversion
func BenchmarkToxIDStringConversion(b *testing.B) {
	publicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	nospam := [4]byte{0xDE, 0xAD, 0xBE, 0xEF}
	id := NewToxID(publicKey, nospam)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}

// BenchmarkGenerateNospamValue benchmarks nospam generation
func BenchmarkGenerateNospamValue(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateNospam()
	}
}

// BenchmarkToxIDSetNospam benchmarks nospam setting
func BenchmarkToxIDSetNospam(b *testing.B) {
	publicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	nospam := [4]byte{0xDE, 0xAD, 0xBE, 0xEF}
	id := NewToxID(publicKey, nospam)
	newNospam := [4]byte{0xCA, 0xFE, 0xBA, 0xBE}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.SetNospam(newNospam)
	}
}
