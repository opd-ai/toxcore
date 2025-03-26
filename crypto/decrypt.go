package crypto

import (
	"errors"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// Decrypt decrypts a message using authenticated encryption.
//
//export ToxDecrypt
func Decrypt(ciphertext []byte, nonce Nonce, senderPK [32]byte, recipientSK [32]byte) ([]byte, error) {
	// Validate inputs
	if len(ciphertext) == 0 {
		return nil, errors.New("empty ciphertext")
	}

	// Decrypt the message
	decrypted, ok := box.Open(nil, ciphertext, (*[24]byte)(&nonce), (*[32]byte)(&senderPK), (*[32]byte)(&recipientSK))
	if !ok {
		return nil, errors.New("decryption failed")
	}

	return decrypted, nil
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
