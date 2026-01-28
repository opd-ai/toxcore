// Package limits provides centralized message size limits for the Tox protocol.
// This ensures consistent validation across different components of the system.
package limits

import "errors"

const (
	// MaxPlaintextMessage is the Tox protocol limit for plaintext messages (1372 bytes)
	// This matches the original Tox specification for message size
	MaxPlaintextMessage = 1372

	// MaxEncryptedMessage is the maximum size after encryption overhead
	// This includes the plaintext + NaCl box overhead (Poly1305 MAC tag)
	MaxEncryptedMessage = 1388 // MaxPlaintextMessage + 16 bytes (box.Overhead)

	// MaxStorageMessage is the maximum for storage operations (with padding)
	// This allows for message padding to standard sizes for privacy
	MaxStorageMessage = 16384

	// MaxProcessingBuffer is the absolute maximum for any operation
	// This prevents memory exhaustion attacks (1MB limit)
	MaxProcessingBuffer = 1024 * 1024

	// EncryptionOverhead is the overhead added by NaCl box encryption
	// This is the Poly1305 MAC tag added by box.Seal()
	// The nonce (24 bytes) is sent separately in the protocol header
	EncryptionOverhead = 16 // golang.org/x/crypto/nacl/box.Overhead
)

var (
	// ErrMessageEmpty indicates an empty message was provided
	ErrMessageEmpty = errors.New("empty message")

	// ErrMessageTooLarge indicates message exceeds maximum size
	ErrMessageTooLarge = errors.New("message too large")
)

// ValidateMessageSize validates a message against the specified maximum size.
func ValidateMessageSize(message []byte, maxSize int) error {
	if len(message) == 0 {
		return ErrMessageEmpty
	}
	if len(message) > maxSize {
		return ErrMessageTooLarge
	}
	return nil
}

// ValidatePlaintextMessage validates a plaintext message size.
func ValidatePlaintextMessage(message []byte) error {
	if len(message) == 0 {
		return ErrMessageEmpty
	}
	if len(message) > MaxPlaintextMessage {
		return ErrMessageTooLarge
	}
	return nil
}

// ValidateEncryptedMessage validates an encrypted message size.
func ValidateEncryptedMessage(message []byte) error {
	if len(message) == 0 {
		return ErrMessageEmpty
	}
	if len(message) > MaxEncryptedMessage {
		return ErrMessageTooLarge
	}
	return nil
}

// ValidateStorageMessage validates a storage message size (allows padding).
func ValidateStorageMessage(message []byte) error {
	if len(message) == 0 {
		return ErrMessageEmpty
	}
	if len(message) > MaxStorageMessage {
		return ErrMessageTooLarge
	}
	return nil
}

// ValidateProcessingBuffer validates against absolute maximum.
func ValidateProcessingBuffer(data []byte) error {
	if len(data) == 0 {
		return ErrMessageEmpty
	}
	if len(data) > MaxProcessingBuffer {
		return ErrMessageTooLarge
	}
	return nil
}
