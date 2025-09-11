package crypto

import (
	"crypto/rand"
	"errors"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// Nonce is a 24-byte value used for encryption.
type Nonce [24]byte

// GenerateNonce creates a cryptographically secure random nonce.
//
//export ToxGenerateNonce
func GenerateNonce() (Nonce, error) {
	logger := logrus.WithFields(logrus.Fields{
		"function": "GenerateNonce",
		"package":  "crypto",
	})

	logger.Debug("Function entry: generating new nonce")

	defer func() {
		logger.Debug("Function exit: GenerateNonce")
	}()

	var nonce Nonce
	_, err := rand.Read(nonce[:])
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"error_type": "random_generation_failed",
			"operation":  "rand.Read",
		}).Error("Failed to generate cryptographically secure nonce")
		return Nonce{}, err
	}

	logger.WithFields(logrus.Fields{
		"nonce_size": len(nonce),
		"operation":  "nonce_generation_success",
	}).Debug("Cryptographically secure nonce generated successfully")

	return nonce, nil
}

// Maximum message size (1MB to prevent excessive memory usage)
const MaxMessageSize = 1024 * 1024

// Encrypt encrypts a message using authenticated encryption.
//
//export ToxEncrypt
func Encrypt(message []byte, nonce Nonce, recipientPK [32]byte, senderSK [32]byte) ([]byte, error) {
	logger := logrus.WithFields(logrus.Fields{
		"function":     "Encrypt",
		"package":      "crypto",
		"message_size": len(message),
		"recipient_pk": recipientPK[:8], // First 8 bytes for privacy
		"sender_sk":    senderSK[:8],    // First 8 bytes for privacy
	})

	logger.Debug("Function entry: starting authenticated message encryption")

	defer func() {
		logger.Debug("Function exit: Encrypt")
	}()

	// Validate inputs
	if len(message) == 0 {
		logger.WithFields(logrus.Fields{
			"error":      "empty message",
			"error_type": "validation_failed",
			"operation":  "input_validation",
		}).Error("Encryption failed: message cannot be empty")
		return nil, errors.New("empty message")
	}

	if len(message) > MaxMessageSize {
		logger.WithFields(logrus.Fields{
			"message_size": len(message),
			"max_size":     MaxMessageSize,
			"error":        "message too large",
			"error_type":   "validation_failed",
			"operation":    "size_validation",
		}).Error("Encryption failed: message exceeds maximum allowed size")
		return nil, errors.New("message too large")
	}

	logger.WithFields(logrus.Fields{
		"message_size": len(message),
		"operation":    "nacl_box_seal",
		"crypto_lib":   "golang.org/x/crypto/nacl/box",
	}).Debug("Performing authenticated encryption with NaCl box")

	// Encrypt the message
	encrypted := box.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&recipientPK), (*[32]byte)(&senderSK))

	// Create a copy of the encrypted data before potentially wiping any sensitive data
	encryptedCopy := make([]byte, len(encrypted))
	copy(encryptedCopy, encrypted)

	logger.WithFields(logrus.Fields{
		"message_size":   len(message),
		"encrypted_size": len(encryptedCopy),
		"overhead_bytes": len(encryptedCopy) - len(message),
		"operation":      "encryption_success",
	}).Debug("Message encrypted successfully with authentication tag")

	// Note: We're not wiping the message here since it might be needed by the caller
	// We're also not wiping the private key since that would affect the caller

	return encryptedCopy, nil
}

// EncryptSymmetric encrypts a message using a symmetric key.
//
//export ToxEncryptSymmetric
func EncryptSymmetric(message []byte, nonce Nonce, key [32]byte) ([]byte, error) {
	logger := logrus.WithFields(logrus.Fields{
		"function":     "EncryptSymmetric",
		"package":      "crypto",
		"message_size": len(message),
		"key_preview":  key[:4], // First 4 bytes for debugging (safe for symmetric keys)
	})

	logger.Debug("Function entry: starting symmetric authenticated encryption")

	defer func() {
		logger.Debug("Function exit: EncryptSymmetric")
	}()

	if len(message) == 0 {
		logger.WithFields(logrus.Fields{
			"error":      "empty message",
			"error_type": "validation_failed",
			"operation":  "input_validation",
		}).Error("Symmetric encryption failed: message cannot be empty")
		return nil, errors.New("empty message")
	}

	if len(message) > MaxMessageSize {
		logger.WithFields(logrus.Fields{
			"message_size": len(message),
			"max_size":     MaxMessageSize,
			"error":        "message too large",
			"error_type":   "validation_failed",
			"operation":    "size_validation",
		}).Error("Symmetric encryption failed: message exceeds maximum allowed size")
		return nil, errors.New("message too large")
	}

	// Make a copy of the key to avoid modifying the original
	var keyCopy [32]byte
	copy(keyCopy[:], key[:])

	logger.WithFields(logrus.Fields{
		"message_size": len(message),
		"operation":    "secretbox_seal",
		"crypto_lib":   "golang.org/x/crypto/nacl/secretbox",
	}).Debug("Performing symmetric authenticated encryption with NaCl secretbox")

	// Use NaCl's secretbox for authenticated symmetric encryption
	// This provides both confidentiality and integrity protection
	out := secretbox.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&keyCopy))

	// Create a copy of the encrypted data
	outCopy := make([]byte, len(out))
	copy(outCopy, out)

	// Securely wipe the key copy
	ZeroBytes(keyCopy[:])

	logger.WithFields(logrus.Fields{
		"message_size":   len(message),
		"encrypted_size": len(outCopy),
		"overhead_bytes": len(outCopy) - len(message),
		"operation":      "symmetric_encryption_success",
	}).Debug("Message encrypted successfully with symmetric authentication")

	// Note: We're not wiping the message here since it might be needed by the caller

	return outCopy, nil
}
