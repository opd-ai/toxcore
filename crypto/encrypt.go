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
	var nonce Nonce
	_, err := rand.Read(nonce[:])
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "GenerateNonce",
			"error":      err.Error(),
			"error_type": "random_generation_failed",
		}).Error("Failed to generate cryptographically secure nonce")
		return Nonce{}, err
	}

	if IsHotPathLoggingEnabled() {
		logrus.WithFields(logrus.Fields{
			"function":   "GenerateNonce",
			"nonce_size": len(nonce),
		}).Debug("Nonce generated successfully")
	}

	return nonce, nil
}

// MaxEncryptionBuffer is the maximum buffer size for encryption operations
// (1MB to prevent excessive memory usage). This is distinct from the Tox
// protocol message size limit (1372 bytes) defined in limits.MaxPlaintextMessage.
// Use this constant for buffer allocation limits in the crypto layer.
const MaxEncryptionBuffer = 1024 * 1024

// Encrypt encrypts a message using authenticated encryption.
//
//export ToxEncrypt
func Encrypt(message []byte, nonce Nonce, recipientPK, senderSK [32]byte) ([]byte, error) {
	// Validate inputs
	if len(message) == 0 {
		logrus.WithFields(logrus.Fields{
			"function":   "Encrypt",
			"error":      "empty message",
			"error_type": "validation_failed",
		}).Error("Encryption failed: message cannot be empty")
		return nil, errors.New("empty message")
	}

	if len(message) > MaxEncryptionBuffer {
		logrus.WithFields(logrus.Fields{
			"function":     "Encrypt",
			"message_size": len(message),
			"max_size":     MaxEncryptionBuffer,
			"error":        "message too large",
		}).Error("Encryption failed: message exceeds maximum allowed size")
		return nil, errors.New("message too large")
	}

	// Encrypt the message
	encrypted := box.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&recipientPK), (*[32]byte)(&senderSK))

	// Create a copy of the encrypted data before potentially wiping any sensitive data
	encryptedCopy := make([]byte, len(encrypted))
	copy(encryptedCopy, encrypted)

	if IsHotPathLoggingEnabled() {
		logrus.WithFields(logrus.Fields{
			"function":       "Encrypt",
			"message_size":   len(message),
			"encrypted_size": len(encryptedCopy),
		}).Debug("Message encrypted successfully")
	}

	return encryptedCopy, nil
}

// EncryptSymmetric encrypts a message using a symmetric key.
//
//export ToxEncryptSymmetric
func EncryptSymmetric(message []byte, nonce Nonce, key [32]byte) ([]byte, error) {
	if len(message) == 0 {
		logrus.WithFields(logrus.Fields{
			"function":   "EncryptSymmetric",
			"error":      "empty message",
			"error_type": "validation_failed",
		}).Error("Symmetric encryption failed: message cannot be empty")
		return nil, errors.New("empty message")
	}

	if len(message) > MaxEncryptionBuffer {
		logrus.WithFields(logrus.Fields{
			"function":     "EncryptSymmetric",
			"message_size": len(message),
			"max_size":     MaxEncryptionBuffer,
			"error":        "message too large",
		}).Error("Symmetric encryption failed: message exceeds maximum allowed size")
		return nil, errors.New("message too large")
	}

	// Make a copy of the key to avoid modifying the original
	var keyCopy [32]byte
	copy(keyCopy[:], key[:])

	// Use NaCl's secretbox for authenticated symmetric encryption
	out := secretbox.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&keyCopy))

	// Create a copy of the encrypted data
	outCopy := make([]byte, len(out))
	copy(outCopy, out)

	// Securely wipe the key copy
	ZeroBytes(keyCopy[:])

	if IsHotPathLoggingEnabled() {
		logrus.WithFields(logrus.Fields{
			"function":       "EncryptSymmetric",
			"message_size":   len(message),
			"encrypted_size": len(outCopy),
		}).Debug("Message encrypted successfully with symmetric authentication")
	}

	return outCopy, nil
}
