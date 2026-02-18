package limits

import (
	"crypto/rand"
	"errors"
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
		name      string
		message   []byte
		wantErr   error
		checkWrap bool
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
			name:      "message too large",
			message:   make([]byte, MaxPlaintextMessage+1),
			wantErr:   ErrMessageTooLarge,
			checkWrap: true,
		},
		{
			name:      "message much too large",
			message:   make([]byte, MaxPlaintextMessage*2),
			wantErr:   ErrMessageTooLarge,
			checkWrap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePlaintextMessage(tt.message)
			if tt.checkWrap {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidatePlaintextMessage() error = %v, should wrap %v", err, tt.wantErr)
				}
			} else if err != tt.wantErr {
				t.Errorf("ValidatePlaintextMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateEncryptedMessage tests the encrypted message validation function
func TestValidateEncryptedMessage(t *testing.T) {
	tests := []struct {
		name      string
		message   []byte
		wantErr   error
		checkWrap bool
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
			name:      "encrypted message too large",
			message:   make([]byte, MaxEncryptedMessage+1),
			wantErr:   ErrMessageTooLarge,
			checkWrap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEncryptedMessage(tt.message)
			if tt.checkWrap {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateEncryptedMessage() error = %v, should wrap %v", err, tt.wantErr)
				}
			} else if err != tt.wantErr {
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
		name      string
		message   []byte
		maxSize   int
		wantErr   error
		checkWrap bool
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
			name:      "message exceeds limit",
			message:   make([]byte, 101),
			maxSize:   100,
			wantErr:   ErrMessageTooLarge,
			checkWrap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessageSize(tt.message, tt.maxSize)
			if tt.checkWrap {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateMessageSize() error = %v, should wrap %v", err, tt.wantErr)
				}
			} else if err != tt.wantErr {
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

// TestValidateStorageMessage tests the storage message validation function
func TestValidateStorageMessage(t *testing.T) {
	tests := []struct {
		name      string
		message   []byte
		wantErr   error
		checkWrap bool // check if error wraps the sentinel
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
			name:    "valid small storage message",
			message: make([]byte, 256),
			wantErr: nil,
		},
		{
			name:    "valid padded message 1024",
			message: make([]byte, 1024),
			wantErr: nil,
		},
		{
			name:    "valid padded message 4096",
			message: make([]byte, 4096),
			wantErr: nil,
		},
		{
			name:    "valid max-size storage message",
			message: make([]byte, MaxStorageMessage),
			wantErr: nil,
		},
		{
			name:      "storage message too large",
			message:   make([]byte, MaxStorageMessage+1),
			wantErr:   ErrMessageTooLarge,
			checkWrap: true,
		},
		{
			name:      "storage message much too large",
			message:   make([]byte, MaxStorageMessage*2),
			wantErr:   ErrMessageTooLarge,
			checkWrap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStorageMessage(tt.message)
			if tt.checkWrap {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateStorageMessage() error = %v, should wrap %v", err, tt.wantErr)
				}
			} else if err != tt.wantErr {
				t.Errorf("ValidateStorageMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateProcessingBuffer tests the processing buffer validation function
func TestValidateProcessingBuffer(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantErr   error
		checkWrap bool
	}{
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: ErrMessageEmpty,
		},
		{
			name:    "nil data",
			data:    nil,
			wantErr: ErrMessageEmpty,
		},
		{
			name:    "valid small buffer",
			data:    make([]byte, 100),
			wantErr: nil,
		},
		{
			name:    "valid medium buffer",
			data:    make([]byte, 65536),
			wantErr: nil,
		},
		{
			name:    "valid max-size buffer",
			data:    make([]byte, MaxProcessingBuffer),
			wantErr: nil,
		},
		{
			name:      "buffer exceeds limit",
			data:      make([]byte, MaxProcessingBuffer+1),
			wantErr:   ErrMessageTooLarge,
			checkWrap: true,
		},
		{
			name:      "buffer much too large",
			data:      make([]byte, MaxProcessingBuffer+1000),
			wantErr:   ErrMessageTooLarge,
			checkWrap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProcessingBuffer(tt.data)
			if tt.checkWrap {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateProcessingBuffer() error = %v, should wrap %v", err, tt.wantErr)
				}
			} else if err != tt.wantErr {
				t.Errorf("ValidateProcessingBuffer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestErrorContextFormat verifies that error messages include size context
func TestErrorContextFormat(t *testing.T) {
	// Test that error messages contain useful context
	tests := []struct {
		name        string
		validate    func() error
		wantContain string
	}{
		{
			name: "plaintext too large includes sizes",
			validate: func() error {
				return ValidatePlaintextMessage(make([]byte, MaxPlaintextMessage+100))
			},
			wantContain: "1472",
		},
		{
			name: "encrypted too large includes sizes",
			validate: func() error {
				return ValidateEncryptedMessage(make([]byte, MaxEncryptedMessage+50))
			},
			wantContain: "1438",
		},
		{
			name: "storage too large includes sizes",
			validate: func() error {
				return ValidateStorageMessage(make([]byte, MaxStorageMessage+10))
			},
			wantContain: "16394",
		},
		{
			name: "processing buffer too large includes sizes",
			validate: func() error {
				return ValidateProcessingBuffer(make([]byte, MaxProcessingBuffer+5))
			},
			wantContain: "1048581",
		},
		{
			name: "generic validate includes sizes",
			validate: func() error {
				return ValidateMessageSize(make([]byte, 200), 100)
			},
			wantContain: "200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			errStr := err.Error()
			if !contains(errStr, tt.wantContain) {
				t.Errorf("error message %q should contain %q", errStr, tt.wantContain)
			}
		})
	}
}

// contains checks if s contains substr (simple helper to avoid importing strings)
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// BenchmarkValidateStorageMessage benchmarks storage message validation performance
func BenchmarkValidateStorageMessage(b *testing.B) {
	message := make([]byte, MaxStorageMessage)
	rand.Read(message)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateStorageMessage(message)
	}
}

// BenchmarkValidateProcessingBuffer benchmarks processing buffer validation performance
func BenchmarkValidateProcessingBuffer(b *testing.B) {
	data := make([]byte, MaxProcessingBuffer)
	rand.Read(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateProcessingBuffer(data)
	}
}
