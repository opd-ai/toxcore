package crypto

import (
	"errors"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// Decrypt decrypts a message using authenticated encryption (legacy mode).
//
//export ToxDecrypt
func Decrypt(ciphertext []byte, nonce Nonce, senderPK [32]byte, recipientSK [32]byte) ([]byte, error) {
	return DecryptWithMode(ciphertext, nonce, senderPK, recipientSK, EncryptionLegacy)
}

// DecryptWithMode decrypts a message with the specified mode
//
//export ToxDecryptWithMode
func DecryptWithMode(ciphertext []byte, nonce Nonce, senderPK [32]byte, recipientSK [32]byte, mode EncryptionMode) ([]byte, error) {
	// Validate inputs
	if len(ciphertext) == 0 {
		return nil, errors.New("empty ciphertext")
	}

	switch mode {
	case EncryptionLegacy:
		// Use legacy NaCl box decryption
		decrypted, ok := box.Open(nil, ciphertext, (*[24]byte)(&nonce), (*[32]byte)(&senderPK), (*[32]byte)(&recipientSK))
		if !ok {
			return nil, errors.New("decryption failed")
		}
		return decrypted, nil

	case EncryptionNoise:
		// For Noise mode, decryption is handled by the NoiseSession
		// This function primarily exists for backward compatibility
		return nil, errors.New("noise decryption must be handled by NoiseSession")

	default:
		return nil, errors.New("unsupported decryption mode")
	}
}

// DecryptSymmetric decrypts a message using a symmetric key.
//
//export ToxDecryptSymmetric
func DecryptSymmetric(ciphertext []byte, nonce Nonce, key [32]byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, errors.New("empty ciphertext")
	}

	// Decrypt and authenticate using NaCl's secretbox
	var out []byte
	var ok bool
	out, ok = secretbox.Open(nil, ciphertext, (*[24]byte)(&nonce), (*[32]byte)(&key))
	if !ok {
		return nil, errors.New("decryption failed: message authentication failed")
	}

	return out, nil
}
