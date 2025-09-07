package crypto

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/curve25519"
)

// DeriveSharedSecret computes a shared secret between two parties
// using Elliptic Curve Diffie-Hellman (ECDH) on Curve25519.
//
//export ToxDeriveSharedSecret
func DeriveSharedSecret(peerPublicKey, privateKey [32]byte) ([32]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":        "DeriveSharedSecret",
		"peer_key_prefix": fmt.Sprintf("%x", peerPublicKey[:8]),
	}).Info("Computing shared secret using ECDH")

	// Create copies of the keys to prevent modification
	var publicKeyCopy [32]byte
	var privateKeyCopy [32]byte
	copy(publicKeyCopy[:], peerPublicKey[:])
	copy(privateKeyCopy[:], privateKey[:])

	logrus.WithFields(logrus.Fields{
		"function": "DeriveSharedSecret",
	}).Debug("Key copies created for ECDH computation")

	// Use X25519 for ECDH computation
	sharedSecret, err := curve25519.X25519(privateKeyCopy[:], publicKeyCopy[:])
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "DeriveSharedSecret",
			"error":    err.Error(),
		}).Error("X25519 computation failed")

		// Securely wipe the key copy before returning
		ZeroBytes(privateKeyCopy[:])
		return [32]byte{}, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function": "DeriveSharedSecret",
	}).Debug("X25519 computation completed successfully")

	// Copy the result to a fixed-size array
	var result [32]byte
	copy(result[:], sharedSecret)

	// Securely wipe the key copy and intermediate shared secret
	ZeroBytes(privateKeyCopy[:])
	ZeroBytes(sharedSecret)

	logrus.WithFields(logrus.Fields{
		"function": "DeriveSharedSecret",
	}).Info("Shared secret computed successfully, sensitive data wiped")

	return result, nil
}
