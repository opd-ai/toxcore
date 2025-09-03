package crypto

import (
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// Nonce is a 24-byte value used for encryption.
type Nonce [24]byte

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

// Encrypt encrypts a message using authenticated encryption.
//
//export ToxEncrypt
func Encrypt(message []byte, nonce Nonce, recipientPK [32]byte, senderSK [32]byte) ([]byte, error) {
	// Validate inputs
	if len(message) == 0 {
		return nil, errors.New("empty message")
	}

	if len(message) > MaxMessageSize {
		return nil, errors.New("message too large")
	}

	// Encrypt the message
	encrypted := box.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&recipientPK), (*[32]byte)(&senderSK))

	// Create a copy of the encrypted data before potentially wiping any sensitive data
	encryptedCopy := make([]byte, len(encrypted))
	copy(encryptedCopy, encrypted)

	// Note: We're not wiping the message here since it might be needed by the caller
	// We're also not wiping the private key since that would affect the caller

	return encryptedCopy, nil
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

	// Make a copy of the key to avoid modifying the original
	var keyCopy [32]byte
	copy(keyCopy[:], key[:])

	// Use NaCl's secretbox for authenticated symmetric encryption
	// This provides both confidentiality and integrity protection
	out := secretbox.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&keyCopy))

	// Create a copy of the encrypted data
	outCopy := make([]byte, len(out))
	copy(outCopy, out)

	// Securely wipe the key copy
	ZeroBytes(keyCopy[:])

	// Note: We're not wiping the message here since it might be needed by the caller

	return outCopy, nil
}
