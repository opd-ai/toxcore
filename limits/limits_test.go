package limits

import (
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/nacl/box"
)

// TestEncryptionOverheadMatchesNaCl verifies that our EncryptionOverhead constant
// matches the actual overhead from golang.org/x/crypto/nacl/box
func TestEncryptionOverheadMatchesNaCl(t *testing.T) {
	if EncryptionOverhead != box.Overhead {
		t.Errorf("EncryptionOverhead = %d, want %d (box.Overhead)", EncryptionOverhead, box.Overhead)
	}
}

// TestMaxEncryptedMessageCalculation verifies that MaxEncryptedMessage is correctly
// calculated as MaxPlaintextMessage + EncryptionOverhead
func TestMaxEncryptedMessageCalculation(t *testing.T) {
	expected := MaxPlaintextMessage + EncryptionOverhead
	if MaxEncryptedMessage != expected {
		t.Errorf("MaxEncryptedMessage = %d, want %d (MaxPlaintextMessage + EncryptionOverhead)",
			MaxEncryptedMessage, expected)
	}
}

// TestActualNaClBoxOverhead tests that actual NaCl box encryption adds exactly
// EncryptionOverhead bytes to the ciphertext
func TestActualNaClBoxOverhead(t *testing.T) {
	// Generate test keys
	_, privateKey1, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair 1: %v", err)
	}

	publicKey2, _, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair 2: %v", err)
	}

	// Generate nonce
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}

	// Test with various message sizes
	testSizes := []int{0, 1, 100, 1000, MaxPlaintextMessage}

	for _, size := range testSizes {
		message := make([]byte, size)
		if size > 0 {
			if _, err := rand.Read(message); err != nil {
				t.Fatalf("Failed to generate test message: %v", err)
			}
		}

		// Encrypt with NaCl box
		encrypted := box.Seal(nil, message, &nonce, publicKey2, privateKey1)

		actualOverhead := len(encrypted) - len(message)
		if actualOverhead != EncryptionOverhead {
			t.Errorf("For message size %d: actual NaCl overhead = %d bytes, want %d bytes",
				size, actualOverhead, EncryptionOverhead)
		}

		// Verify encrypted size matches our MaxEncryptedMessage for max-size messages
		if size == MaxPlaintextMessage {
			if len(encrypted) > MaxEncryptedMessage {
				t.Errorf("Encrypted max-size message is %d bytes, exceeds MaxEncryptedMessage (%d bytes)",
					len(encrypted), MaxEncryptedMessage)
			}
		}
	}
}

// TestValidatePlaintextMessage tests the plaintext validation function
func TestValidatePlaintextMessage(t *testing.T) {
	tests := []struct {
		name    string
		message []byte
		wantErr error
	}{
		{
			name:    "empty message",
			message: []byte{},
			wantErr: ErrMessageEmpty,
		},
		{
			name:    "nil message",
			message: nil,
			wantErr: ErrMessageEmpty,
		},
		{
			name:    "valid small message",
			message: []byte("Hello, world!"),
			wantErr: nil,
		},
		{
			name:    "valid max-size message",
			message: make([]byte, MaxPlaintextMessage),
			wantErr: nil,
		},
		{
			name:    "message too large",
			message: make([]byte, MaxPlaintextMessage+1),
			wantErr: ErrMessageTooLarge,
		},
		{
			name:    "message much too large",
			message: make([]byte, MaxPlaintextMessage*2),
			wantErr: ErrMessageTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePlaintextMessage(tt.message)
			if err != tt.wantErr {
				t.Errorf("ValidatePlaintextMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateEncryptedMessage tests the encrypted message validation function
func TestValidateEncryptedMessage(t *testing.T) {
	tests := []struct {
		name    string
		message []byte
		wantErr error
	}{
		{
			name:    "empty message",
			message: []byte{},
			wantErr: ErrMessageEmpty,
		},
		{
			name:    "nil message",
			message: nil,
			wantErr: ErrMessageEmpty,
		},
		{
			name:    "valid small encrypted message",
			message: make([]byte, 100+EncryptionOverhead),
			wantErr: nil,
		},
		{
			name:    "valid max-size encrypted message",
			message: make([]byte, MaxEncryptedMessage),
			wantErr: nil,
		},
		{
			name:    "encrypted message too large",
			message: make([]byte, MaxEncryptedMessage+1),
			wantErr: ErrMessageTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEncryptedMessage(tt.message)
			if err != tt.wantErr {
				t.Errorf("ValidateEncryptedMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConstantConsistency verifies internal consistency of all message size constants
func TestConstantConsistency(t *testing.T) {
	// MaxEncryptedMessage should be larger than MaxPlaintextMessage
	if MaxEncryptedMessage <= MaxPlaintextMessage {
		t.Errorf("MaxEncryptedMessage (%d) should be > MaxPlaintextMessage (%d)",
			MaxEncryptedMessage, MaxPlaintextMessage)
	}

	// MaxStorageMessage should be larger than MaxEncryptedMessage (allows padding)
	if MaxStorageMessage <= MaxEncryptedMessage {
		t.Errorf("MaxStorageMessage (%d) should be > MaxEncryptedMessage (%d)",
			MaxStorageMessage, MaxEncryptedMessage)
	}

	// MaxProcessingBuffer should be largest
	if MaxProcessingBuffer <= MaxStorageMessage {
		t.Errorf("MaxProcessingBuffer (%d) should be > MaxStorageMessage (%d)",
			MaxProcessingBuffer, MaxStorageMessage)
	}

	// EncryptionOverhead should be positive
	if EncryptionOverhead <= 0 {
		t.Errorf("EncryptionOverhead must be positive, got %d", EncryptionOverhead)
	}

	// Verify the relationship: MaxEncryptedMessage = MaxPlaintextMessage + EncryptionOverhead
	if MaxEncryptedMessage != MaxPlaintextMessage+EncryptionOverhead {
		t.Errorf("MaxEncryptedMessage (%d) != MaxPlaintextMessage (%d) + EncryptionOverhead (%d)",
			MaxEncryptedMessage, MaxPlaintextMessage, EncryptionOverhead)
	}
}

// TestValidateMessageSize tests the generic message size validation function
func TestValidateMessageSize(t *testing.T) {
	tests := []struct {
		name    string
		message []byte
		maxSize int
		wantErr error
	}{
		{
			name:    "empty message",
			message: []byte{},
			maxSize: 100,
			wantErr: ErrMessageEmpty,
		},
		{
			name:    "valid message within limit",
			message: make([]byte, 50),
			maxSize: 100,
			wantErr: nil,
		},
		{
			name:    "message at exact limit",
			message: make([]byte, 100),
			maxSize: 100,
			wantErr: nil,
		},
		{
			name:    "message exceeds limit",
			message: make([]byte, 101),
			maxSize: 100,
			wantErr: ErrMessageTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessageSize(tt.message, tt.maxSize)
			if err != tt.wantErr {
				t.Errorf("ValidateMessageSize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkValidatePlaintextMessage benchmarks plaintext validation performance
func BenchmarkValidatePlaintextMessage(b *testing.B) {
	message := make([]byte, MaxPlaintextMessage)
	rand.Read(message)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidatePlaintextMessage(message)
	}
}

// BenchmarkValidateEncryptedMessage benchmarks encrypted message validation performance
func BenchmarkValidateEncryptedMessage(b *testing.B) {
	message := make([]byte, MaxEncryptedMessage)
	rand.Read(message)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateEncryptedMessage(message)
	}
}
