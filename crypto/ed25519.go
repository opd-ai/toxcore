package crypto

import (
	"crypto/ed25519"
	"errors"
)

// SignatureSize is the size of an Ed25519 signature in bytes.
const SignatureSize = ed25519.SignatureSize

// Signature represents an Ed25519 signature.
//
//export ToxSignature
type Signature [SignatureSize]byte

// Sign creates an Ed25519 signature for a message using the private key.
//
//export ToxSign
func Sign(message []byte, privateKey [32]byte) (Signature, error) {
	if len(message) == 0 {
		return Signature{}, errors.New("empty message")
	}

	// Make a copy of the private key to avoid modifying the original
	var privateKeyCopy [32]byte
	copy(privateKeyCopy[:], privateKey[:])

	// Convert the 32-byte private key to the format expected by ed25519
	// Ed25519 private keys are 64 bytes (32 bytes seed + 32 bytes public key)
	edPrivateKey := ed25519.NewKeyFromSeed(privateKeyCopy[:])

	// Sign the message
	signatureBytes := ed25519.Sign(edPrivateKey, message)

	var signature Signature
	copy(signature[:], signatureBytes)

	// Securely wipe sensitive data
	ZeroBytes(privateKeyCopy[:])
	ZeroBytes(edPrivateKey)

	return signature, nil
}

// GetSignaturePublicKey derives the Ed25519 public key from a private key seed.
// This public key should be used for verifying signatures created with the same private key.
//
//export ToxGetSignaturePublicKey
func GetSignaturePublicKey(privateKey [32]byte) [32]byte {
	// Make a copy of the private key to avoid modifying the original
	var privateKeyCopy [32]byte
	copy(privateKeyCopy[:], privateKey[:])

	// Generate the full Ed25519 private key (64 bytes) from the seed
	edPrivateKey := ed25519.NewKeyFromSeed(privateKeyCopy[:])

	// The public key is in the second half of the expanded private key
	var publicKey [32]byte
	copy(publicKey[:], edPrivateKey[32:])

	// Securely wipe sensitive data
	ZeroBytes(privateKeyCopy[:])
	ZeroBytes(edPrivateKey)

	return publicKey
}

// Verify checks if a signature is valid for a message and public key.
//
//export ToxVerify
func Verify(message []byte, signature Signature, publicKey [32]byte) (bool, error) {
	if len(message) == 0 {
		return false, errors.New("empty message")
	}

	// Use the provided Ed25519 public key for verification
	return ed25519.Verify(publicKey[:], message, signature[:]), nil
}
