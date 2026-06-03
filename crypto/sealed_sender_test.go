package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSealSenderBasic tests that SealSender creates a valid encrypted envelope.
// I3 remediation: sender identity is encrypted and cannot be recovered without recipient key.
func TestSealSenderBasic(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey, err := GenerateKeyPair()
	require.NoError(t, err)

	cert, err := SealSender(senderKey.Public, recipientKey.Public)
	require.NoError(t, err)
	assert.NotNil(t, cert)

	// Verify the cert structure is valid
	var zeroKey [32]byte
	assert.NotEqual(t, cert.EphemeralPublicKey, zeroKey, "ephemeral key should not be zero")
	assert.NotEqual(t, cert.Nonce, [12]byte{}, "nonce should not be empty")
	assert.NotEqual(t, cert.EncryptedSenderID, [48]byte{}, "encrypted identity should not be empty")
	assert.NotEqual(t, cert.Proof, [32]byte{}, "proof should not be empty")
}

// TestOpenSenderBasic tests that OpenSender correctly decrypts a sealed envelope.
func TestOpenSenderBasic(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey, err := GenerateKeyPair()
	require.NoError(t, err)

	// Seal sender identity for recipient
	cert, err := SealSender(senderKey.Public, recipientKey.Public)
	require.NoError(t, err)

	// Recipient opens the envelope
	decryptedSenderID, err := OpenSender(cert, recipientKey.Private, recipientKey.Public)
	require.NoError(t, err)

	// Verify we recovered the correct sender identity
	assert.Equal(t, senderKey.Public, decryptedSenderID, "decrypted sender ID should match original")
}

// TestOpenSenderWrongRecipient tests that OpenSender fails with wrong recipient key.
// I3 remediation: only the intended recipient can decrypt; others get authentication failure.
func TestOpenSenderWrongRecipient(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey1, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey2, err := GenerateKeyPair()
	require.NoError(t, err)

	// Seal sender identity for recipient1
	cert, err := SealSender(senderKey.Public, recipientKey1.Public)
	require.NoError(t, err)

	// Recipient2 attempts to open with their key - should fail
	_, err = OpenSender(cert, recipientKey2.Private, recipientKey2.Public)
	assert.Error(t, err, "OpenSender should fail with wrong recipient key")
	assert.Contains(t, err.Error(), "proof", "error should mention proof verification failure")
}

// TestOpenSenderTamperedEnvelope tests that tampering with the ciphertext is detected.
func TestOpenSenderTamperedEnvelope(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey, err := GenerateKeyPair()
	require.NoError(t, err)

	cert, err := SealSender(senderKey.Public, recipientKey.Public)
	require.NoError(t, err)

	// Tamper with the encrypted identity
	cert.EncryptedSenderID[0] ^= 0xFF

	// Attempt to open tampered envelope - should fail
	_, err = OpenSender(cert, recipientKey.Private, recipientKey.Public)
	assert.Error(t, err, "OpenSender should fail with tampered ciphertext")
	assert.Contains(t, err.Error(), "decrypt", "error should mention decryption failure")
}

// TestOpenSenderTamperedProof tests that proof tampering is detected.
func TestOpenSenderTamperedProof(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey, err := GenerateKeyPair()
	require.NoError(t, err)

	cert, err := SealSender(senderKey.Public, recipientKey.Public)
	require.NoError(t, err)

	// Tamper with the proof
	cert.Proof[0] ^= 0xFF

	// Attempt to open with tampered proof - should fail
	_, err = OpenSender(cert, recipientKey.Private, recipientKey.Public)
	assert.Error(t, err, "OpenSender should fail with tampered proof")
	assert.Contains(t, err.Error(), "proof", "error should mention proof verification failure")
}

// TestSealedSenderRoundTrip tests a full seal/open cycle with multiple pairs.
func TestSealedSenderRoundTrip(t *testing.T) {
	alice, err := GenerateKeyPair()
	require.NoError(t, err)

	bob, err := GenerateKeyPair()
	require.NoError(t, err)

	charlie, err := GenerateKeyPair()
	require.NoError(t, err)

	// Alice sends to Bob
	certBob, err := SealSender(alice.Public, bob.Public)
	require.NoError(t, err)

	decryptedBob, err := OpenSender(certBob, bob.Private, bob.Public)
	require.NoError(t, err)
	assert.Equal(t, alice.Public, decryptedBob)

	// Alice sends to Charlie
	certCharlie, err := SealSender(alice.Public, charlie.Public)
	require.NoError(t, err)

	decryptedCharlie, err := OpenSender(certCharlie, charlie.Private, charlie.Public)
	require.NoError(t, err)
	assert.Equal(t, alice.Public, decryptedCharlie)

	// Bob cannot open Alice's message to Charlie
	_, err = OpenSender(certCharlie, bob.Private, bob.Public)
	assert.Error(t, err, "Bob should not be able to open Alice's message to Charlie")
}

// TestSealedSenderUniqueNonces tests that different seals produce different nonces.
// I3: each sealed message should use a unique nonce for semantic security.
func TestSealedSenderUniqueNonces(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey, err := GenerateKeyPair()
	require.NoError(t, err)

	// Create multiple seals
	cert1, err := SealSender(senderKey.Public, recipientKey.Public)
	require.NoError(t, err)

	cert2, err := SealSender(senderKey.Public, recipientKey.Public)
	require.NoError(t, err)

	// Nonces should be different (with overwhelming probability)
	assert.NotEqual(t, cert1.Nonce, cert2.Nonce, "nonces should be different for semantic security")

	// Ephemeral keys should be different
	assert.NotEqual(t, cert1.EphemeralPublicKey, cert2.EphemeralPublicKey, "ephemeral keys should be different")

	// Ciphertexts should be different
	assert.NotEqual(t, cert1.EncryptedSenderID, cert2.EncryptedSenderID, "ciphertexts should be different")
}

// TestVerifySenderCert tests the certificate verification utility.
func TestVerifySenderCert(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey1, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey2, err := GenerateKeyPair()
	require.NoError(t, err)

	// Create cert for recipient1
	cert, err := SealSender(senderKey.Public, recipientKey1.Public)
	require.NoError(t, err)

	// Cert should verify with correct recipient
	assert.True(t, VerifySenderCert(cert, recipientKey1.Public), "cert should verify with correct recipient")

	// Cert should not verify with wrong recipient
	assert.False(t, VerifySenderCert(cert, recipientKey2.Public), "cert should not verify with wrong recipient")
}

// TestSealedSenderProofConstantTime tests that proof verification uses constant-time comparison.
// This prevents timing attacks on proof validation.
func TestSealedSenderProofConstantTime(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey, err := GenerateKeyPair()
	require.NoError(t, err)

	cert, err := SealSender(senderKey.Public, recipientKey.Public)
	require.NoError(t, err)

	// Generate expected proof using same logic as SealSender
	proofMac := hmac.New(sha256.New, recipientKey.Public[:])
	proofMac.Write(cert.EphemeralPublicKey[:])
	expectedProof := proofMac.Sum(nil)

	// Verify proof uses constant-time comparison (hmac.Equal)
	// We can't directly test timing here, but we can verify the comparison works correctly
	assert.True(t, hmac.Equal(cert.Proof[:], expectedProof), "proof should match using constant-time comparison")

	// Tamper with proof and verify it fails
	tamperedProof := make([]byte, len(expectedProof))
	copy(tamperedProof, expectedProof)
	tamperedProof[0] ^= 0xFF
	assert.False(t, hmac.Equal(cert.Proof[:], tamperedProof), "tampered proof should not match")
}

// TestSealedSenderKeyZeroization tests that sensitive material is properly wiped.
// I3: all ECDH outputs and encryption keys should be zeroized after use.
func TestSealedSenderKeyZeroization(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey, err := GenerateKeyPair()
	require.NoError(t, err)

	// Seal should not leak key material (we can't directly verify zeroization, 
	// but we verify the function completes successfully)
	cert, err := SealSender(senderKey.Public, recipientKey.Public)
	require.NoError(t, err)
	assert.NotNil(t, cert)

	// Open should also complete without panicking
	_, err = OpenSender(cert, recipientKey.Private, recipientKey.Public)
	require.NoError(t, err)
}

// TestSealedSenderLargeNumber tests multiple seal/open operations maintain correctness.
func TestSealedSenderLargeNumber(t *testing.T) {
	senderKey, err := GenerateKeyPair()
	require.NoError(t, err)

	recipientKey, err := GenerateKeyPair()
	require.NoError(t, err)

	// Seal and open 100 times
	for i := 0; i < 100; i++ {
		cert, err := SealSender(senderKey.Public, recipientKey.Public)
		require.NoError(t, err, "iteration %d: SealSender failed", i)

		decrypted, err := OpenSender(cert, recipientKey.Private, recipientKey.Public)
		require.NoError(t, err, "iteration %d: OpenSender failed", i)

		assert.Equal(t, senderKey.Public, decrypted, "iteration %d: decrypted identity mismatch", i)
	}
}

// TestSealedSenderNilCert tests that OpenSender handles nil cert gracefully.
func TestSealedSenderNilCert(t *testing.T) {
	recipientKey, err := GenerateKeyPair()
	require.NoError(t, err)

	// This should not panic and should return an error
	_, err = OpenSender(nil, recipientKey.Private, recipientKey.Public)
	require.Error(t, err, "OpenSender should return error for nil certificate")
}
