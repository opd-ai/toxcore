package crypto

import (
	"errors"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// Decrypt decrypts a message using authenticated encryption.
//
//export ToxDecrypt
func Decrypt(ciphertext []byte, nonce Nonce, senderPK, recipientSK [32]byte) ([]byte, error) {
	// Validate inputs
	if len(ciphertext) == 0 {
		return nil, errors.New("empty ciphertext")
	}

	// Make a copy of the private key to avoid modifying the original
	var skCopy [32]byte
	copy(skCopy[:], recipientSK[:])

	// Decrypt the message
	decrypted, ok := box.Open(nil, ciphertext, (*[24]byte)(&nonce), (*[32]byte)(&senderPK), (*[32]byte)(&skCopy))
	if !ok {
		// Securely wipe the key copy before returning
		ZeroBytes(skCopy[:])
		return nil, errors.New("decryption failed")
	}

	// Create a copy of the decrypted data
	decryptedCopy := make([]byte, len(decrypted))
	copy(decryptedCopy, decrypted)

	// Securely wipe the intermediate buffers
	ZeroBytes(skCopy[:])
	ZeroBytes(decrypted)

	return decryptedCopy, nil
}

// DecryptSymmetric decrypts a message using a symmetric key.
//
//export ToxDecryptSymmetric
func DecryptSymmetric(ciphertext []byte, nonce Nonce, key [32]byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, errors.New("empty ciphertext")
	}

	// Make a copy of the key to avoid modifying the original
	var keyCopy [32]byte
	copy(keyCopy[:], key[:])

	// Decrypt and authenticate using NaCl's secretbox
	var out []byte
	var ok bool
	out, ok = secretbox.Open(nil, ciphertext, (*[24]byte)(&nonce), (*[32]byte)(&keyCopy))
	if !ok {
		// Securely wipe the key copy before returning
		ZeroBytes(keyCopy[:])
		return nil, errors.New("decryption failed: message authentication failed")
	}

	// Create a copy of the decrypted data
	outCopy := make([]byte, len(out))
	copy(outCopy, out)

	// Securely wipe the intermediate buffers
	ZeroBytes(keyCopy[:])
	ZeroBytes(out)

	return outCopy, nil
}
