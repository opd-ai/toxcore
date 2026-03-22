// Package limits provides centralized message size limits for the Tox protocol.
// This ensures consistent validation across different components of the system.
package limits

import (
	"errors"
	"fmt"
)

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
// Returns an error with context including the actual and maximum sizes.
func ValidateMessageSize(message []byte, maxSize int) error {
	if len(message) == 0 {
		return ErrMessageEmpty
	}
	if len(message) > maxSize {
		return fmt.Errorf("%w: size %d exceeds limit %d", ErrMessageTooLarge, len(message), maxSize)
	}
	return nil
}

// ValidatePlaintextMessage validates a plaintext message size against MaxPlaintextMessage.
// Returns an error with context if the message is empty or exceeds the limit.
func ValidatePlaintextMessage(message []byte) error {
	if len(message) == 0 {
		return ErrMessageEmpty
	}
	if len(message) > MaxPlaintextMessage {
		return fmt.Errorf("%w: plaintext size %d exceeds limit %d", ErrMessageTooLarge, len(message), MaxPlaintextMessage)
	}
	return nil
}

// ValidateEncryptedMessage validates an encrypted message size against MaxEncryptedMessage.
// Returns an error with context if the message is empty or exceeds the limit.
func ValidateEncryptedMessage(message []byte) error {
	if len(message) == 0 {
		return ErrMessageEmpty
	}
	if len(message) > MaxEncryptedMessage {
		return fmt.Errorf("%w: encrypted size %d exceeds limit %d", ErrMessageTooLarge, len(message), MaxEncryptedMessage)
	}
	return nil
}

// ValidateStorageMessage validates a storage message size against MaxStorageMessage.
// Storage messages may include padding for traffic analysis resistance.
// Returns an error with context if the message is empty or exceeds the limit.
func ValidateStorageMessage(message []byte) error {
	if len(message) == 0 {
		return ErrMessageEmpty
	}
	if len(message) > MaxStorageMessage {
		return fmt.Errorf("%w: storage size %d exceeds limit %d", ErrMessageTooLarge, len(message), MaxStorageMessage)
	}
	return nil
}

// ValidateProcessingBuffer validates data against the absolute maximum (MaxProcessingBuffer).
// This limit prevents memory exhaustion attacks and should be used for all untrusted input.
// Returns an error with context if the data is empty or exceeds the limit.
func ValidateProcessingBuffer(data []byte) error {
	if len(data) == 0 {
		return ErrMessageEmpty
	}
	if len(data) > MaxProcessingBuffer {
		return fmt.Errorf("%w: buffer size %d exceeds limit %d", ErrMessageTooLarge, len(data), MaxProcessingBuffer)
	}
	return nil
}
