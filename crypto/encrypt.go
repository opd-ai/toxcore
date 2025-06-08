package crypto

import (
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// Nonce is a 24-byte value used for encryption.
type Nonce [24]byte

// EncryptionMode represents the encryption protocol being used
type EncryptionMode int

const (
	EncryptionLegacy EncryptionMode = iota
	EncryptionNoise
)

// GenerateNonce creates a cryptographically secure random nonce.
//
//export ToxGenerateNonce
func GenerateNonce() (Nonce, error) {
	var nonce Nonce
	_, err := rand.Read(nonce[:])
	if err != nil {
		return Nonce{}, err
	}
	return nonce, nil
}

// Maximum message size (1MB to prevent excessive memory usage)
const MaxMessageSize = 1024 * 1024

// Encrypt encrypts a message using authenticated encryption (legacy mode).
//
//export ToxEncrypt
func Encrypt(message []byte, nonce Nonce, recipientPK [32]byte, senderSK [32]byte) ([]byte, error) {
	return EncryptWithMode(message, nonce, recipientPK, senderSK, EncryptionLegacy)
}

// EncryptWithMode encrypts a message with the specified mode
//
//export ToxEncryptWithMode
func EncryptWithMode(message []byte, nonce Nonce, recipientPK [32]byte, senderSK [32]byte, mode EncryptionMode) ([]byte, error) {
	// Validate inputs
	if len(message) == 0 {
		return nil, errors.New("empty message")
	}

	if len(message) > MaxMessageSize {
		return nil, errors.New("message too large")
	}

	switch mode {
	case EncryptionLegacy:
		// Use legacy NaCl box encryption
		encrypted := box.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&recipientPK), (*[32]byte)(&senderSK))
		return encrypted, nil

	case EncryptionNoise:
		// For Noise mode, encryption is handled by the NoiseSession
		// This function primarily exists for backward compatibility
		return nil, errors.New("noise encryption must be handled by NoiseSession")

	default:
		return nil, errors.New("unsupported encryption mode")
	}
}

// EncryptSymmetric encrypts a message using a symmetric key.
//
//export ToxEncryptSymmetric
func EncryptSymmetric(message []byte, nonce Nonce, key [32]byte) ([]byte, error) {
	if len(message) == 0 {
		return nil, errors.New("empty message")
	}

	if len(message) > MaxMessageSize {
		return nil, errors.New("message too large")
	}

	// Use NaCl's secretbox for authenticated symmetric encryption
	// This provides both confidentiality and integrity protection
	out := secretbox.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&key))

	return out, nil
}
