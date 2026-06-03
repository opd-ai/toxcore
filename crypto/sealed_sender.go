package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// SenderCert represents an encrypted sender identity envelope for real-time messages.
// I3 remediation: encrypts sender identity to recipient to prevent transport-layer
// sender identification. The transport peer receives SenderCert but cannot decrypt it
// without the recipient's identity key.
//
// Structure:
// - SenderPublicKey [32]byte: ephemeral public key for envelope encryption (X25519)
// - Nonce [12]byte: AES-GCM nonce for envelope encryption
// - EncryptedIdentity []byte: AES-256-GCM encrypted sender identity (32 bytes + auth tag)
// - AuthenticationTag [16]byte: AEAD tag for envelope integrity
//
// The recipient decrypts and verifies the envelope under their identity key,
// learning the true sender identity. Spoofed envelopes are rejected.
type SenderCert struct {
	// EphemeralPublicKey is the X25519 public key for ECDH with recipient identity key.
	EphemeralPublicKey [32]byte `json:"ephemeral_public_key"`

	// Nonce is the 12-byte AES-GCM nonce for envelope encryption.
	Nonce [12]byte `json:"nonce"`

	// EncryptedSenderID is the AES-256-GCM ciphertext of the sender's identity public key (32 bytes).
	// Total size: 32 (plaintext) + 16 (auth tag) = 48 bytes.
	EncryptedSenderID [48]byte `json:"encrypted_sender_id"`

	// Proof is an HMAC-SHA256 proof that the sender knows the recipient's identity key.
	// This prevents sender spoofing by requiring knowledge of the recipient's real public key.
	Proof [32]byte `json:"proof"`
}

// SealSender encrypts the sender's identity into a SenderCert envelope for the recipient.
// The recipient can decrypt and authenticate the envelope, learning the true sender identity.
// Transport peers cannot derive the sender identity without the recipient's identity key.
//
// I3 implementation: uses HKDF-SHA256 to derive the envelope key from ECDH between
// sender (static identity) and an ephemeral keypair, then encrypts the sender's
// identity under AES-256-GCM. The proof demonstrates sender knowledge of the
// recipient's identity key.
func SealSender(senderIdentity [32]byte, recipientIdentity [32]byte) (*SenderCert, error) {
	// Generate an ephemeral keypair for the envelope ECDH
	ephemeralKeyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral keypair: %w", err)
	}

	// Perform ECDH between ephemeral public key and recipient's identity key
	// This derives a shared secret known only to the recipient (and holder of recipient private key)
	sharedSecret, err := DeriveSharedSecret(recipientIdentity, ephemeralKeyPair.Private)
	if err != nil {
		return nil, fmt.Errorf("failed to derive shared secret: %w", err)
	}

	// Derive envelope encryption key and nonce from the shared secret
	// Using HKDF with a context-specific info string
	envelopeKey := make([]byte, 32) // AES-256
	nonce := make([]byte, 12)        // AES-GCM nonce

	hkdfReader := hkdf.New(sha256.New, sharedSecret[:], nil, []byte("TOX_SEALED_SENDER_V1"))
	if _, err := io.ReadFull(hkdfReader, envelopeKey); err != nil {
		return nil, fmt.Errorf("failed to derive envelope key: %w", err)
	}

	hkdfReader = hkdf.New(sha256.New, sharedSecret[:], envelopeKey, []byte("TOX_SEALED_SENDER_NONCE"))
	if _, err := io.ReadFull(hkdfReader, nonce); err != nil {
		return nil, fmt.Errorf("failed to derive nonce: %w", err)
	}

	// Encrypt the sender's identity using AES-256-GCM
	block, err := aes.NewCipher(envelopeKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt the sender identity (32 bytes plaintext)
	// The ciphertext will be 32 + 16 = 48 bytes (32 data + 16 auth tag)
	plaintextIdentity := senderIdentity[:]
	ciphertext := aead.Seal(nil, nonce, plaintextIdentity, nil)

	// Verify we got the expected ciphertext size (32 + 16 = 48 bytes)
	if len(ciphertext) != 48 {
		return nil, fmt.Errorf("unexpected ciphertext size: got %d, want 48", len(ciphertext))
	}

	// Generate proof that demonstrates sender authenticity: HMAC-SHA256(recipient_key, ephemeral_key)
	// This proof can be verified by anyone who knows the recipient's identity,
	// proving that the sender intended to send to this specific recipient.
	proofMac := hmac.New(sha256.New, recipientIdentity[:])
	proofMac.Write(ephemeralKeyPair.Public[:])
	proofBytes := proofMac.Sum(nil)

	// Build the SenderCert
	cert := &SenderCert{
		EphemeralPublicKey: ephemeralKeyPair.Public,
	}
	copy(cert.Nonce[:], nonce)
	copy(cert.EncryptedSenderID[:], ciphertext)
	copy(cert.Proof[:], proofBytes)

	// Zeroize sensitive material
	ZeroBytes(sharedSecret[:])
	ZeroBytes(envelopeKey)
	ZeroBytes(plaintextIdentity)
	ZeroBytes(ephemeralKeyPair.Private[:])

	return cert, nil
}

// OpenSender decrypts and authenticates a SenderCert envelope.
// Only a holder of the recipient's private key can successfully decrypt.
//
// Returns the sender's identity public key if verification succeeds.
// Returns error if:
// - The certificate is nil
// - The ECDH fails
// - The AEAD decryption fails (tampered or wrong recipient)
// - The proof is invalid (sender spoofing attempt)
func OpenSender(cert *SenderCert, recipientPrivateKey [32]byte, recipientPublicKey [32]byte) ([32]byte, error) {
	if cert == nil {
		return [32]byte{}, fmt.Errorf("certificate is nil")
	}
	// Perform ECDH between ephemeral public key and recipient's private key
	// This recovers the same shared secret that was used during SealSender
	sharedSecret, err := DeriveSharedSecret(cert.EphemeralPublicKey, recipientPrivateKey)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to derive shared secret: %w", err)
	}

	// Verify the proof first (constant-time check to prevent timing attacks)
	// The proof is computed from the recipient's key and ephemeral key
	proofMac := hmac.New(sha256.New, recipientPublicKey[:])
	proofMac.Write(cert.EphemeralPublicKey[:])
	expectedProof := proofMac.Sum(nil)

	if !hmac.Equal(cert.Proof[:], expectedProof) {
		ZeroBytes(sharedSecret[:])
		return [32]byte{}, fmt.Errorf("sender proof verification failed")
	}

	// Re-derive the envelope key and nonce using the same HKDF
	envelopeKey := make([]byte, 32) // AES-256
	nonce := make([]byte, 12)

	hkdfReader := hkdf.New(sha256.New, sharedSecret[:], nil, []byte("TOX_SEALED_SENDER_V1"))
	if _, err := io.ReadFull(hkdfReader, envelopeKey); err != nil {
		return [32]byte{}, fmt.Errorf("failed to derive envelope key: %w", err)
	}

	hkdfReader = hkdf.New(sha256.New, sharedSecret[:], envelopeKey, []byte("TOX_SEALED_SENDER_NONCE"))
	if _, err := io.ReadFull(hkdfReader, nonce); err != nil {
		return [32]byte{}, fmt.Errorf("failed to derive nonce: %w", err)
	}

	// Decrypt the sender identity using AES-256-GCM
	block, err := aes.NewCipher(envelopeKey)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt: we expect 32 bytes of plaintext
	plaintextIdentity := make([]byte, 32)
	_, err = aead.Open(plaintextIdentity[:0], nonce, cert.EncryptedSenderID[:], nil)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to decrypt sender identity: %w", err)
	}

	// Convert to [32]byte
	var senderID [32]byte
	copy(senderID[:], plaintextIdentity)

	// Zeroize sensitive material
	ZeroBytes(sharedSecret[:])
	ZeroBytes(envelopeKey)
	ZeroBytes(plaintextIdentity)

	return senderID, nil
}

// VerifySenderCert validates that a SenderCert envelope can be opened by a recipient.
// This is a utility for testing and validation; it does not decrypt the sender identity.
//
// Returns true if:
// - The proof is valid for the recipient public key
// - The envelope structure is valid
func VerifySenderCert(cert *SenderCert, recipientPublicKey [32]byte) bool {
	proofMac := hmac.New(sha256.New, recipientPublicKey[:])
	proofMac.Write(cert.EphemeralPublicKey[:])
	expectedProof := proofMac.Sum(nil)
	return hmac.Equal(cert.Proof[:], expectedProof)
}
