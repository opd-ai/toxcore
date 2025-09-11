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
	logger := logrus.WithFields(logrus.Fields{
		"function": "GenerateKeyPair",
		"package":  "crypto",
	})

	logger.Info("Function entry: generating new cryptographic key pair")

	defer func() {
		logger.Debug("Function exit: GenerateKeyPair")
	}()

	logger.WithFields(logrus.Fields{
		"operation":  "nacl_box_generate_key",
		"crypto_lib": "golang.org/x/crypto/nacl/box",
		"entropy":    "crypto/rand.Reader",
	}).Debug("Generating NaCl box key pair with secure random entropy")

	publicKey, privateKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"error_type": "key_generation_failed",
			"operation":  "box.GenerateKey",
		}).Error("Failed to generate cryptographic key pair")
		return nil, err
	}

	keyPair := &KeyPair{
		Public:  *publicKey,
		Private: *privateKey,
	}

	logger.WithFields(logrus.Fields{
		"public_key_preview": fmt.Sprintf("%x", keyPair.Public[:8]),
		"key_size_bytes":     32,
		"operation":          "key_generation_success",
	}).Info("Cryptographic key pair generated successfully")

	return keyPair, nil
}

// FromSecretKey creates a key pair from an existing private key.
//
//export ToxKeyPairFromSecretKey
func FromSecretKey(secretKey [32]byte) (*KeyPair, error) {
	logger := logrus.WithFields(logrus.Fields{
		"function": "FromSecretKey",
		"package":  "crypto",
	})

	logger.Info("Function entry: creating key pair from existing secret key")

	defer func() {
		logger.Debug("Function exit: FromSecretKey")
	}()

	// Validate the secret key
	if isZeroKey(secretKey) {
		logger.WithFields(logrus.Fields{
			"error":      "invalid secret key: all zeros",
			"error_type": "validation_failed",
			"operation":  "secret_key_validation",
		}).Error("Secret key validation failed: key cannot be all zeros")
		return nil, errors.New("invalid secret key: all zeros")
	}

	logger.WithFields(logrus.Fields{
		"operation": "secret_key_validation",
	}).Debug("Secret key validation passed")

	// Create a copy of the secret key to avoid modifying the original
	var privateKey [32]byte
	copy(privateKey[:], secretKey[:])

	// In NaCl/libsodium, the private key needs to be "clamped" before use
	// This ensures it meets the requirements for curve25519
	privateKey[0] &= 248  // Clear the bottom 3 bits
	privateKey[31] &= 127 // Clear the top bit
	privateKey[31] |= 64  // Set the second-to-top bit

	logger.WithFields(logrus.Fields{
		"operation":  "curve25519_key_clamping",
		"crypto_lib": "golang.org/x/crypto/curve25519",
	}).Debug("Applied curve25519 key clamping to private key")

	// Derive public key from private key using curve25519
	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)

	logger.WithFields(logrus.Fields{
		"operation":  "scalar_base_mult",
		"crypto_lib": "golang.org/x/crypto/curve25519",
	}).Debug("Derived public key from private key using curve25519")

	keyPair := &KeyPair{
		Public:  publicKey,
		Private: secretKey, // Return the original unclamped key as per NaCl convention
	}

	// Securely wipe the temporary private key
	logger.WithFields(logrus.Fields{
		"operation": "secure_memory_wipe",
	}).Debug("Securely wiping temporary key material")
	ZeroBytes(privateKey[:])

	logger.WithFields(logrus.Fields{
		"public_key_preview": fmt.Sprintf("%x", keyPair.Public[:8]),
		"operation":          "key_derivation_success",
	}).Info("Key pair created successfully from secret key")

	return keyPair, nil
}

// isZeroKey checks if a key consists of all zeros.
func isZeroKey(key [32]byte) bool {
	logger := logrus.WithFields(logrus.Fields{
		"function": "isZeroKey",
		"package":  "crypto",
	})

	logger.Debug("Function entry: validating key is not all zeros")

	defer func() {
		logger.Debug("Function exit: isZeroKey")
	}()

	for i, b := range key {
		if b != 0 {
			logger.WithFields(logrus.Fields{
				"operation":     "zero_key_check",
				"result":        "valid_key",
				"first_nonzero": i,
			}).Debug("Key validation: found non-zero byte, key is valid")
			return false
		}
	}

	logger.WithFields(logrus.Fields{
		"operation": "zero_key_check",
		"result":    "invalid_key",
		"error":     "all_bytes_zero",
	}).Warn("Key validation: key consists of all zero bytes")
	return true
}
