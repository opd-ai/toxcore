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

	// Proof is an HMAC-SHA256 over envelope fields keyed by sender-secret-derived material.
	// This binds the encrypted sender identity to the sender's private key material.
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
func SealSender(senderIdentityPublic [32]byte, senderIdentityPrivate [32]byte, recipientIdentity [32]byte) (*SenderCert, error) {
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
	plaintextIdentity := senderIdentityPublic[:]
	ciphertext := aead.Seal(nil, nonce, plaintextIdentity, nil)

	// Verify we got the expected ciphertext size (32 + 16 = 48 bytes)
	if len(ciphertext) != 48 {
		return nil, fmt.Errorf("unexpected ciphertext size: got %d, want 48", len(ciphertext))
	}

	// Generate proof tied to sender secret material:
	// HMAC-SHA256(ECDH(sender_private, recipient_public), ephemeral_key || nonce || ciphertext)
	proofSecret, err := DeriveSharedSecret(recipientIdentity, senderIdentityPrivate)
	if err != nil {
		return nil, fmt.Errorf("failed to derive sender proof secret: %w", err)
	}
	proofMac := hmac.New(sha256.New, proofSecret[:])
	proofMac.Write(ephemeralKeyPair.Public[:])
	proofMac.Write(nonce)
	proofMac.Write(ciphertext)
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
	ZeroBytes(proofSecret[:])
	ZeroBytes(envelopeKey)
	ZeroBytes(plaintextIdentity)
	ZeroBytes(ephemeralKeyPair.Private[:])
	ZeroBytes(senderIdentityPrivate[:])

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
	_ = recipientPublicKey

	if cert == nil {
		return [32]byte{}, fmt.Errorf("certificate is nil")
	}
	// Perform ECDH between ephemeral public key and recipient's private key
	// This recovers the same shared secret that was used during SealSender
	sharedSecret, err := DeriveSharedSecret(cert.EphemeralPublicKey, recipientPrivateKey)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to derive shared secret: %w", err)
	}
	defer ZeroBytes(sharedSecret[:])

	// Re-derive the envelope key using the same HKDF
	envelopeKey := make([]byte, 32) // AES-256
	defer ZeroBytes(envelopeKey)

	hkdfReader := hkdf.New(sha256.New, sharedSecret[:], nil, []byte("TOX_SEALED_SENDER_V1"))
	if _, err := io.ReadFull(hkdfReader, envelopeKey); err != nil {
		return [32]byte{}, fmt.Errorf("failed to derive envelope key: %w", err)
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
	defer ZeroBytes(plaintextIdentity)
	_, err = aead.Open(plaintextIdentity[:0], cert.Nonce[:], cert.EncryptedSenderID[:], nil)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to decrypt sender identity: %w", err)
	}

	// Verify proof using sender secret material derived from decrypted sender identity.
	var senderIdentity [32]byte
	copy(senderIdentity[:], plaintextIdentity)
	proofSecret, err := DeriveSharedSecret(senderIdentity, recipientPrivateKey)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to derive sender proof secret: %w", err)
	}
	defer ZeroBytes(proofSecret[:])

	proofMac := hmac.New(sha256.New, proofSecret[:])
	proofMac.Write(cert.EphemeralPublicKey[:])
	proofMac.Write(cert.Nonce[:])
	proofMac.Write(cert.EncryptedSenderID[:])
	expectedProof := proofMac.Sum(nil)
	if !hmac.Equal(cert.Proof[:], expectedProof) {
		return [32]byte{}, fmt.Errorf("sender proof verification failed")
	}

	// Convert to [32]byte
	var senderID [32]byte
	copy(senderID[:], plaintextIdentity)

	return senderID, nil
}

// VerifySenderCert performs basic structural validation for a SenderCert envelope.
// Full sender authentication requires OpenSender, which validates proof material
// derived from the recipient private key and decrypted sender identity.
func VerifySenderCert(cert *SenderCert, recipientPublicKey [32]byte) bool {
	_ = recipientPublicKey

	if cert == nil {
		return false
	}

	return cert.EphemeralPublicKey != [32]byte{} &&
		cert.Nonce != [12]byte{} &&
		cert.EncryptedSenderID != [48]byte{} &&
		cert.Proof != [32]byte{}
}
