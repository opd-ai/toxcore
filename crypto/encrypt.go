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

// Encrypt encrypts a message using authenticated encryption.
//
//export ToxEncrypt
func Encrypt(message []byte, nonce Nonce, recipientPK [32]byte, senderSK [32]byte) ([]byte, error) {
	// Validate inputs
	if len(message) == 0 {
		return nil, errors.New("empty message")
	}

	// Encrypt the message
	encrypted := box.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&recipientPK), (*[32]byte)(&senderSK))
	return encrypted, nil
}

// EncryptSymmetric encrypts a message using a symmetric key.
//
//export ToxEncryptSymmetric
func EncryptSymmetric(message []byte, nonce Nonce, key [32]byte) ([]byte, error) {
    if len(message) == 0 {
        return nil, errors.New("empty message")
    }

    // Use NaCl's secretbox for authenticated symmetric encryption
    // This provides both confidentiality and integrity protection
    out := secretbox.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&key))
    
    return out, nil
}