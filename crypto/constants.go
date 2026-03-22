// Package crypto constants defines cryptographic size constants used throughout toxcore.
//
// These constants ensure consistent key and nonce sizes across the codebase
// and eliminate magic numbers in cryptographic code.
package crypto

const (
	// KeySize is the size in bytes of Curve25519/NaCl public and private keys.
	// This is 32 bytes (256 bits), the standard size for Curve25519 operations.
	KeySize = 32

	// NonceSize is the size in bytes of NaCl box/secretbox nonces.
	// This is 24 bytes (192 bits), providing sufficient security margin against
	// nonce reuse in random nonce generation schemes.
	NonceSize = 24

	// Ed25519PublicKeySize is the size in bytes of Ed25519 public keys.
	Ed25519PublicKeySize = 32

	// Ed25519PrivateKeySize is the size in bytes of Ed25519 private keys.
	// Ed25519 private keys are 64 bytes: 32 bytes seed + 32 bytes public key.
	Ed25519PrivateKeySize = 64

	// Ed25519SeedSize is the size in bytes of Ed25519 seed (the secret part).
	Ed25519SeedSize = 32

	// SharedSecretSize is the size in bytes of NaCl shared secrets.
	// Computed via Curve25519 scalar multiplication.
	SharedSecretSize = 32

	// BoxOverhead is the authentication tag overhead for NaCl box encryption.
	// This is the Poly1305 MAC tag size (16 bytes).
	BoxOverhead = 16

	// SecretBoxOverhead is the authentication tag overhead for NaCl secretbox encryption.
	// Same as BoxOverhead since both use Poly1305.
	SecretBoxOverhead = 16

	// ToxIDSize is the total size of a ToxID: public key (32) + nospam (4) + checksum (2).
	ToxIDSize = 38

	// ToxIDNospamSize is the size of the nospam field in a ToxID.
	ToxIDNospamSize = 4

	// ToxIDChecksumSize is the size of the checksum field in a ToxID.
	ToxIDChecksumSize = 2

	// ToxIDHexLength is the length of a hex-encoded ToxID (38 bytes * 2).
	ToxIDHexLength = ToxIDSize * 2 // 76 characters
)
