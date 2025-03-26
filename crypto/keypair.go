// Package crypto implements cryptographic primitives for the Tox protocol.
//
// This package handles key generation, encryption, decryption, and signatures
// using the NaCl cryptography library through Go's x/crypto packages.
//
// Example:
//
//	keys, err := crypto.GenerateKeyPair()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Public key:", hex.EncodeToString(keys.Public[:]))
package crypto

import (
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/curve25519"
)

// KeyPair represents a NaCl crypto_box key pair used for Tox communications.
//
//export ToxKeyPair
type KeyPair struct {
	Public  [32]byte
	Private [32]byte
}

// GenerateKeyPair creates a new random NaCl key pair.
//
//export ToxGenerateKeyPair
func GenerateKeyPair() (*KeyPair, error) {
	publicKey, privateKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	keyPair := &KeyPair{
		Public:  *publicKey,
		Private: *privateKey,
	}

	return keyPair, nil
}

// FromSecretKey creates a key pair from an existing private key.
//
//export ToxKeyPairFromSecretKey
func FromSecretKey(secretKey [32]byte) (*KeyPair, error) {
    // Validate the secret key
    if isZeroKey(secretKey) {
        return nil, errors.New("invalid secret key: all zeros")
    }

    // Create a copy of the secret key to avoid modifying the original
    privateKey := secretKey
    
    // In NaCl/libsodium, the private key needs to be "clamped" before use
    // This ensures it meets the requirements for curve25519
    privateKey[0] &= 248  // Clear the bottom 3 bits
    privateKey[31] &= 127 // Clear the top bit
    privateKey[31] |= 64  // Set the second-to-top bit
    
    // Derive public key from private key using curve25519
	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)
    
    return &KeyPair{
        Public:  publicKey,
        Private: secretKey, // Return the original unclamped key as per NaCl convention
    }, nil
}

// isZeroKey checks if a key consists of all zeros.
func isZeroKey(key [32]byte) bool {
	for _, b := range key {
		if b != 0 {
			return false
		}
	}
	return true
}
