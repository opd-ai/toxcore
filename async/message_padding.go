package async

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
)

// ErrInvalidPaddedMessage is returned when attempting to unpad an invalid message
var ErrInvalidPaddedMessage = errors.New("invalid padded message")

// ErrMessageTooLarge is returned when a message exceeds the maximum size and would be truncated
var ErrMessageTooLarge = errors.New("message exceeds maximum size")

const (
	// Define standard message size buckets in bytes
	MessageSizeSmall  = 256
	MessageSizeMedium = 1024
	MessageSizeLarge  = 4096
	MessageSizeMax    = 16384

	// Size of the length prefix
	LengthPrefixSize = 4
)

// PadMessageToStandardSize pads a message to a standard size bucket to prevent size correlation.
// Returns an error if the message exceeds the maximum allowed size and would require truncation.
func PadMessageToStandardSize(message []byte) ([]byte, error) {
	originalLen := len(message)

	// Check if message would be truncated
	if originalLen > MessageSizeMax-LengthPrefixSize {
		return nil, ErrMessageTooLarge
	}

	var targetSize int

	switch {
	case originalLen <= MessageSizeSmall:
		targetSize = MessageSizeSmall
	case originalLen <= MessageSizeMedium:
		targetSize = MessageSizeMedium
	case originalLen <= MessageSizeLarge:
		targetSize = MessageSizeLarge
	default:
		targetSize = MessageSizeMax
	}

	// Allocate the padded buffer with space for length prefix
	paddedMessage := make([]byte, targetSize)

	// First 4 bytes store the actual message length as uint32
	binary.BigEndian.PutUint32(paddedMessage[:LengthPrefixSize], uint32(originalLen))

	// Copy the original message
	copy(paddedMessage[LengthPrefixSize:], message)

	// Fill the rest with random bytes
	if targetSize > originalLen+LengthPrefixSize {
		rand.Read(paddedMessage[originalLen+LengthPrefixSize:])
	}

	return paddedMessage, nil
}

// UnpadMessage extracts the original message from a padded message
func UnpadMessage(paddedMessage []byte) ([]byte, error) {
	if len(paddedMessage) < LengthPrefixSize {
		return nil, ErrInvalidPaddedMessage
	}

	// Extract the original message length
	originalLen := binary.BigEndian.Uint32(paddedMessage[:LengthPrefixSize])

	// Validate the length
	if originalLen > uint32(len(paddedMessage)-LengthPrefixSize) {
		return nil, ErrInvalidPaddedMessage
	}

	// Extract the original message
	return paddedMessage[LengthPrefixSize : LengthPrefixSize+originalLen], nil
}
