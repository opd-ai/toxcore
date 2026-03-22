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

// SignWithPrivateKey creates an Ed25519 signature using a full 64-byte private key.
// The privateKey parameter should be the concatenation of the 32-byte seed and 32-byte public key,
// as returned by ed25519.NewKeyFromSeed or ed25519.GenerateKey.
//
//export ToxSignWithPrivateKey
func SignWithPrivateKey(privateKey [64]byte, message []byte) (Signature, error) {
	if len(message) == 0 {
		return Signature{}, errors.New("empty message")
	}

	// Make a copy to avoid modifying the original
	privateKeyCopy := make([]byte, 64)
	copy(privateKeyCopy, privateKey[:])

	// Sign the message using the full Ed25519 private key
	signatureBytes := ed25519.Sign(privateKeyCopy, message)

	var signature Signature
	copy(signature[:], signatureBytes)

	// Securely wipe sensitive data
	ZeroBytes(privateKeyCopy)

	return signature, nil
}

// VerifySignature verifies an Ed25519 signature using a raw [64]byte signature.
// This is a convenience function that converts the signature to the Signature type.
func VerifySignature(publicKey [32]byte, message []byte, signature [64]byte) (bool, error) {
	if len(message) == 0 {
		return false, errors.New("empty message")
	}

	// Use the provided Ed25519 public key for verification
	return ed25519.Verify(publicKey[:], message, signature[:]), nil
}

// GenerateEd25519KeyPair generates a new Ed25519 key pair for signing operations.
// Returns the full 64-byte private key (seed + public) and the 32-byte public key.
// This is separate from Curve25519 keys used for encryption.
//
//export ToxGenerateEd25519KeyPair
func GenerateEd25519KeyPair() (privateKey [64]byte, publicKey [32]byte, err error) {
	pub, priv, err := ed25519.GenerateKey(nil) // Uses crypto/rand by default
	if err != nil {
		return [64]byte{}, [32]byte{}, err
	}
	copy(privateKey[:], priv)
	copy(publicKey[:], pub)
	return privateKey, publicKey, nil
}

// Ed25519PublicKeyFromSeed derives the Ed25519 public key from a 32-byte seed.
func Ed25519PublicKeyFromSeed(seed [32]byte) [32]byte {
	edPrivateKey := ed25519.NewKeyFromSeed(seed[:])
	var publicKey [32]byte
	copy(publicKey[:], edPrivateKey[32:])
	ZeroBytes(edPrivateKey)
	return publicKey
}

// Ed25519PrivateKeyFromSeed creates the full 64-byte Ed25519 private key from a 32-byte seed.
func Ed25519PrivateKeyFromSeed(seed [32]byte) [64]byte {
	edPrivateKey := ed25519.NewKeyFromSeed(seed[:])
	var privateKey [64]byte
	copy(privateKey[:], edPrivateKey)
	return privateKey
}
