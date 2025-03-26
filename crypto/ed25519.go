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

	// Convert the 32-byte private key to the format expected by ed25519
	// Ed25519 private keys are 64 bytes (32 bytes seed + 32 bytes public key)
	edPrivateKey := ed25519.NewKeyFromSeed(privateKey[:])

	// Sign the message
	signatureBytes := ed25519.Sign(edPrivateKey, message)

	var signature Signature
	copy(signature[:], signatureBytes)

	return signature, nil
}

// Verify checks if a signature is valid for a message and public key.
//
//export ToxVerify
func Verify(message []byte, signature Signature, publicKey [32]byte) (bool, error) {
	if len(message) == 0 {
		return false, errors.New("empty message")
	}

	// Convert the 32-byte public key to the format expected by ed25519
	var edPublicKey [ed25519.PublicKeySize]byte
	copy(edPublicKey[:], publicKey[:])

	// Verify the signature
	return ed25519.Verify(edPublicKey[:], message, signature[:]), nil
}
