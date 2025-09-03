package crypto

import (
	"fmt"
	
	"golang.org/x/crypto/curve25519"
)

// DeriveSharedSecret computes a shared secret between two parties
// using Elliptic Curve Diffie-Hellman (ECDH) on Curve25519.
//
//export ToxDeriveSharedSecret
func DeriveSharedSecret(peerPublicKey, privateKey [32]byte) ([32]byte, error) {
	// Create copies of the keys to prevent modification
	var publicKeyCopy [32]byte
	var privateKeyCopy [32]byte
	copy(publicKeyCopy[:], peerPublicKey[:])
	copy(privateKeyCopy[:], privateKey[:])
	
	// Use X25519 for ECDH computation
	sharedSecret, err := curve25519.X25519(privateKeyCopy[:], publicKeyCopy[:])
	if err != nil {
		// Securely wipe the key copy before returning
		ZeroBytes(privateKeyCopy[:])
		return [32]byte{}, fmt.Errorf("failed to compute shared secret: %w", err)
	}
	
	// Copy the result to a fixed-size array
	var result [32]byte
	copy(result[:], sharedSecret)
	
	// Securely wipe the key copy and intermediate shared secret
	ZeroBytes(privateKeyCopy[:])
	ZeroBytes(sharedSecret)
	
	return result, nil
}
