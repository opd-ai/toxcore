package async

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
)

var (
	// ErrInvalidPaddedMessage is returned when attempting to unpad an invalid message
	ErrInvalidPaddedMessage = errors.New("invalid padded message")
)

const (
	// Define standard message size buckets in bytes
	MessageSizeSmall  = 256
	MessageSizeMedium = 1024
	MessageSizeLarge  = 4096
	MessageSizeMax    = 16384
	
	// Size of the length prefix
	LengthPrefixSize = 4
)

// PadMessageToStandardSize pads a message to a standard size bucket to prevent size correlation
func PadMessageToStandardSize(message []byte) []byte {
	originalLen := len(message)
	var targetSize int
	
	switch {
	case originalLen <= MessageSizeSmall:
		targetSize = MessageSizeSmall
	case originalLen <= MessageSizeMedium:
		targetSize = MessageSizeMedium
	case originalLen <= MessageSizeLarge:
		targetSize = MessageSizeLarge
	default:
		if originalLen > MessageSizeMax {
			targetSize = MessageSizeMax
			// For messages larger than max, they'll be truncated
			message = message[:MessageSizeMax-LengthPrefixSize]
		} else {
			targetSize = MessageSizeMax
		}
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
	
	return paddedMessage
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
	return paddedMessage[LengthPrefixSize:LengthPrefixSize+originalLen], nil
}
