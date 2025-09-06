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
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
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
	logrus.WithFields(logrus.Fields{
		"function": "GenerateKeyPair",
	}).Info("Generating new key pair")

	publicKey, privateKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "GenerateKeyPair",
			"error":    err.Error(),
		}).Error("Failed to generate key pair")
		return nil, err
	}

	keyPair := &KeyPair{
		Public:  *publicKey,
		Private: *privateKey,
	}

	logrus.WithFields(logrus.Fields{
		"function":           "GenerateKeyPair",
		"public_key_preview": fmt.Sprintf("%x", keyPair.Public[:8]),
	}).Info("Key pair generated successfully")

	return keyPair, nil
}

// FromSecretKey creates a key pair from an existing private key.
//
//export ToxKeyPairFromSecretKey
func FromSecretKey(secretKey [32]byte) (*KeyPair, error) {
	logrus.WithFields(logrus.Fields{
		"function": "FromSecretKey",
	}).Info("Creating key pair from secret key")

	// Validate the secret key
	if isZeroKey(secretKey) {
		logrus.WithFields(logrus.Fields{
			"function": "FromSecretKey",
			"error":    "invalid secret key: all zeros",
		}).Error("Secret key validation failed")
		return nil, errors.New("invalid secret key: all zeros")
	}

	logrus.WithFields(logrus.Fields{
		"function": "FromSecretKey",
	}).Debug("Validating and preparing secret key")

	// Create a copy of the secret key to avoid modifying the original
	var privateKey [32]byte
	copy(privateKey[:], secretKey[:])

	// In NaCl/libsodium, the private key needs to be "clamped" before use
	// This ensures it meets the requirements for curve25519
	privateKey[0] &= 248  // Clear the bottom 3 bits
	privateKey[31] &= 127 // Clear the top bit
	privateKey[31] |= 64  // Set the second-to-top bit

	logrus.WithFields(logrus.Fields{
		"function": "FromSecretKey",
	}).Debug("Deriving public key from private key")

	// Derive public key from private key using curve25519
	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)

	keyPair := &KeyPair{
		Public:  publicKey,
		Private: secretKey, // Return the original unclamped key as per NaCl convention
	}

	// Securely wipe the temporary private key
	logrus.WithFields(logrus.Fields{
		"function": "FromSecretKey",
	}).Debug("Securely wiping temporary key material")
	ZeroBytes(privateKey[:])

	logrus.WithFields(logrus.Fields{
		"function":           "FromSecretKey",
		"public_key_preview": fmt.Sprintf("%x", keyPair.Public[:8]),
	}).Info("Key pair created successfully from secret key")

	return keyPair, nil
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
