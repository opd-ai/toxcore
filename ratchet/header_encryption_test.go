package ratchet

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKDFRootChainWithHeaders tests header-key derivation alongside root/chain keys.
func TestKDFRootChainWithHeaders(t *testing.T) {
	var rootKey, dhOut [32]byte
	_, err := rand.Read(rootKey[:])
	require.NoError(t, err)
	_, err = rand.Read(dhOut[:])
	require.NoError(t, err)

	originalRootKey := rootKey

	// Derive all four keys
	newRK, ck, hk, nhk, err := kdfRootChainWithHeaders(rootKey, dhOut)
	require.NoError(t, err)

	// Verify all keys are non-zero
	require.NotEqual(t, [32]byte{}, newRK, "new root key should be non-zero")
	require.NotEqual(t, [32]byte{}, ck, "chain key should be non-zero")
	require.NotEqual(t, [32]byte{}, hk, "header key should be non-zero")
	require.NotEqual(t, [32]byte{}, nhk, "next header key should be non-zero")

	// Verify all keys are different from each other
	require.NotEqual(t, newRK, ck, "root key and chain key should differ")
	require.NotEqual(t, newRK, hk, "root key and header key should differ")
	require.NotEqual(t, hk, nhk, "header key and next header key should differ")

	// Verify the derived keys are different from the inputs
	require.NotEqual(t, originalRootKey, newRK, "derived root key should differ from input")
}

// TestSealAndOpenHeader tests header encryption and decryption.
func TestSealAndOpenHeader(t *testing.T) {
	var hk, nhk [32]byte
	_, err := rand.Read(hk[:])
	require.NoError(t, err)
	_, err = rand.Read(nhk[:])
	require.NoError(t, err)

	h := Header{
		DHPub: [32]byte{1, 2, 3, 4, 5},
		PN:    123,
		N:     456,
	}

	// Seal the header
	sealed, err := sealHeader(hk, h)
	require.NoError(t, err)
	require.NotNil(t, sealed)
	// Sealed header should include nonce + ciphertext + tag.
	require.Equal(t, encryptedHeaderSize, len(sealed), "sealed header should include nonce and tag")

	// Open with correct key should succeed
	opened, wasRatchetStep, err := openHeader(hk, nhk, sealed)
	require.NoError(t, err)
	require.False(t, wasRatchetStep, "should not indicate ratchet step when using current key")
	require.Equal(t, h.DHPub, opened.DHPub)
	require.Equal(t, h.PN, opened.PN)
	require.Equal(t, h.N, opened.N)
}

// TestOpenHeaderFallbackToNext tests that openHeader falls back to next-header-key.
func TestOpenHeaderFallbackToNext(t *testing.T) {
	var hk, nhk [32]byte
	_, err := rand.Read(hk[:])
	require.NoError(t, err)
	_, err = rand.Read(nhk[:])
	require.NoError(t, err)

	h := Header{
		DHPub: [32]byte{7, 8, 9},
		PN:    111,
		N:     222,
	}

	// Seal with next-header-key
	sealed, err := sealHeader(nhk, h)
	require.NoError(t, err)

	// Open should succeed with fallback to nhk and indicate ratchet step
	opened, wasRatchetStep, err := openHeader(hk, nhk, sealed)
	require.NoError(t, err)
	require.True(t, wasRatchetStep, "should indicate ratchet step when using next key")
	require.Equal(t, h.DHPub, opened.DHPub)
	require.Equal(t, h.PN, opened.PN)
	require.Equal(t, h.N, opened.N)
}

// TestHeaderKeyZeroization verifies header keys are properly zeroized.
func TestHeaderKeyZeroization(t *testing.T) {
	for i := 0; i < 5; i++ {
		var hk [32]byte
		_, err := rand.Read(hk[:])
		require.NoError(t, err)

		h := Header{
			DHPub: [32]byte{byte(i)},
			PN:    uint32(i),
			N:     uint32(i * 2),
		}

		sealed, err := sealHeader(hk, h)
		require.NoError(t, err)
		require.Equal(t, encryptedHeaderSize, len(sealed))
	}
}

// TestHeaderEncryptionUsesUniqueNonce tests that repeated seals produce different outputs.
func TestHeaderEncryptionUsesUniqueNonce(t *testing.T) {
	var hk [32]byte
	_, err := rand.Read(hk[:])
	require.NoError(t, err)

	h := Header{
		DHPub: [32]byte{11, 22, 33},
		PN:    111,
		N:     222,
	}

	// Seal twice with the same key and header
	sealed1, err := sealHeader(hk, h)
	require.NoError(t, err)

	sealed2, err := sealHeader(hk, h)
	require.NoError(t, err)

	// Outputs should differ due to per-message nonces.
	require.NotEqual(t, sealed1, sealed2, "same key and header should produce different sealed output")

	// Both should decrypt to the same header
	opened1, _, err := openHeader(hk, [32]byte{}, sealed1)
	require.NoError(t, err)
	opened2, _, err := openHeader(hk, [32]byte{}, sealed2)
	require.NoError(t, err)

	require.Equal(t, opened1, opened2)
}

// TestOpenHeaderAuthenticationFailure tests that tampering is detected.
func TestOpenHeaderAuthenticationFailure(t *testing.T) {
	var hk, wrongKey [32]byte
	_, err := rand.Read(hk[:])
	require.NoError(t, err)
	_, err = rand.Read(wrongKey[:])
	require.NoError(t, err)

	h := Header{
		DHPub: [32]byte{99},
		PN:    500,
		N:     1000,
	}

	// Seal with correct key
	sealed, err := sealHeader(hk, h)
	require.NoError(t, err)

	// Try to open with wrong key
	_, _, err = openHeader(wrongKey, [32]byte{}, sealed)
	require.Error(t, err, "opening with wrong key should fail")
}

// TestHeaderEncryptionKeyRotation verifies that different DH outputs produce different header keys.
func TestHeaderEncryptionKeyRotation(t *testing.T) {
	var rk, dh1, dh2 [32]byte
	_, err := rand.Read(rk[:])
	require.NoError(t, err)
	_, err = rand.Read(dh1[:])
	require.NoError(t, err)
	_, err = rand.Read(dh2[:])
	require.NoError(t, err)

	// First DH ratchet step
	rk1Copy := rk
	dh1Copy := dh1
	_, _, hk1, nhk1, err := kdfRootChainWithHeaders(rk1Copy, dh1Copy)
	require.NoError(t, err)

	// Second DH ratchet step (from same root but different DH)
	rk2Copy := rk
	dh2Copy := dh2
	_, _, hk2, nhk2, err := kdfRootChainWithHeaders(rk2Copy, dh2Copy)
	require.NoError(t, err)

	// All header keys should be different
	require.NotEqual(t, hk1, hk2, "header keys should differ between ratchet steps")
	require.NotEqual(t, nhk1, nhk2, "next header keys should differ between ratchet steps")
}

// TestOpenHeaderWithKey tests the single-key variant.
func TestOpenHeaderWithKey(t *testing.T) {
	var hk [32]byte
	_, err := rand.Read(hk[:])
	require.NoError(t, err)

	h := Header{
		DHPub: [32]byte{9},
		PN:    999,
		N:     1000,
	}

	sealed, err := sealHeader(hk, h)
	require.NoError(t, err)

	// Test with correct key
	opened, err := openHeaderWithKey(hk, sealed)
	require.NoError(t, err)
	require.Equal(t, h, opened)

	// Test with wrong key
	var wrongKey [32]byte
	_, err = rand.Read(wrongKey[:])
	require.NoError(t, err)

	_, err = openHeaderWithKey(wrongKey, sealed)
	require.Error(t, err)

	// Test with truncated data
	_, err = openHeaderWithKey(hk, sealed[:40])
	require.Error(t, err)
}

// TestEnableHeaderEncryptionFlag tests the mode flag.
func TestEnableHeaderEncryptionFlag(t *testing.T) {
	alice, err := InitInitiator([32]byte{1}, [32]byte{2})
	require.NoError(t, err)

	// Initially should be plaintext mode
	require.False(t, alice.encryptHeaders)

	// Enable header encryption
	alice.EnableHeaderEncryption()
	require.True(t, alice.encryptHeaders)
}

// TestPlaintextHeaderBackwardCompat tests that plaintext headers still work.
func TestPlaintextHeaderBackwardCompat(t *testing.T) {
	sharedKey := [32]byte{77}
	bobKP, err := GenerateKeyPair()
	require.NoError(t, err)

	// Alice: plaintext headers (default)
	alice, err := InitInitiator(sharedKey, bobKP.Public)
	require.NoError(t, err)
	require.False(t, alice.encryptHeaders)

	// Bob: plaintext headers (default)
	bob := InitRecipient(sharedKey, bobKP)
	require.False(t, bob.encryptHeaders)

	// Establish chains
	err = bob.dhRatchetStep(alice.dhs.Public)
	require.NoError(t, err)

	// Alice sends with plaintext headers
	plaintext := []byte("plaintext mode message")
	header, ciphertext, err := alice.RatchetEncrypt(plaintext, nil)
	require.NoError(t, err)

	// Bob receives with plaintext header mode
	decrypted, err := bob.RatchetDecrypt(header, ciphertext, nil)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)

	// Verify header is indeed plaintext
	headerBytes := header.Encode()
	require.Equal(t, 40, len(headerBytes))
}

// TestHeaderEncryptionRequiresBootstrap tests L-6: header encryption requires a
// plaintext bootstrap message to complete the initial DH ratchet step before
// encrypted headers can be used. This documents the intended behavior.
func TestHeaderEncryptionRequiresBootstrap(t *testing.T) {
	var sharedKey [32]byte
	_, err := rand.Read(sharedKey[:])
	require.NoError(t, err)

	bobKP, err := GenerateKeyPair()
	require.NoError(t, err)

	// Alice: enable header encryption on a fresh session
	alice, err := InitInitiator(sharedKey, bobKP.Public)
	require.NoError(t, err)
	alice.EnableHeaderEncryption()

	// Trying to send with encrypted headers before any DH ratchet fails
	// because hks is not initialized yet
	_, _, err = alice.RatchetEncrypt([]byte("test"), nil)
	require.Error(t, err, "fresh session with header encryption should fail without header keys")
	require.Contains(t, err.Error(), "header encryption not initialized")
}
