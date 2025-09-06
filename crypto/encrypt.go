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
	logrus.WithFields(logrus.Fields{
		"function": "GenerateNonce",
	}).Debug("Generating new nonce")

	var nonce Nonce
	_, err := rand.Read(nonce[:])
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "GenerateNonce",
			"error":    err.Error(),
		}).Error("Failed to generate nonce")
		return Nonce{}, err
	}

	logrus.WithFields(logrus.Fields{
		"function":   "GenerateNonce",
		"nonce_size": len(nonce),
	}).Debug("Nonce generated successfully")

	return nonce, nil
}

// Maximum message size (1MB to prevent excessive memory usage)
const MaxMessageSize = 1024 * 1024

// Encrypt encrypts a message using authenticated encryption.
//
//export ToxEncrypt
func Encrypt(message []byte, nonce Nonce, recipientPK [32]byte, senderSK [32]byte) ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":     "Encrypt",
		"message_size": len(message),
		"recipient_pk": recipientPK[:8], // First 8 bytes for privacy
		"sender_sk":    senderSK[:8],    // First 8 bytes for privacy
	}).Debug("Starting message encryption")

	// Validate inputs
	if len(message) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "Encrypt",
			"error":    "empty message",
		}).Error("Encryption failed: empty message")
		return nil, errors.New("empty message")
	}

	if len(message) > MaxMessageSize {
		logrus.WithFields(logrus.Fields{
			"function":     "Encrypt",
			"message_size": len(message),
			"max_size":     MaxMessageSize,
			"error":        "message too large",
		}).Error("Encryption failed: message too large")
		return nil, errors.New("message too large")
	}

	logrus.WithFields(logrus.Fields{
		"function":     "Encrypt",
		"message_size": len(message),
		"operation":    "box.Seal",
	}).Debug("Performing NaCl box encryption")

	// Encrypt the message
	encrypted := box.Seal(nil, message, (*[24]byte)(&nonce), (*[32]byte)(&recipientPK), (*[32]byte)(&senderSK))

	// Create a copy of the encrypted data before potentially wiping any sensitive data
	encryptedCopy := make([]byte, len(encrypted))
	copy(encryptedCopy, encrypted)

	logrus.WithFields(logrus.Fields{
		"function":       "Encrypt",
		"message_size":   len(message),
		"encrypted_size": len(encryptedCopy),
		"overhead":       len(encryptedCopy) - len(message),
	}).Debug("Message encrypted successfully")

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
