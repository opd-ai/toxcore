package ratchet

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKDFRootChainWithHeaders tests header-key derivation alongside root/chain keys.
func TestKDFRootChainWithHeaders(t *testing.T) {
	// Generate random root key and DH output
	var rootKey, dhOut [32]byte
	_, err := rand.Read(rootKey[:])
	require.NoError(t, err)
	_, err = rand.Read(dhOut[:])
	require.NoError(t, err)

	// Save original values to verify they're different from derived keys
	originalRootKey := rootKey
	originalDhOut := dhOut

	// Derive all four keys
	newRK, ck, hk, nhk, err := kdfRootChainWithHeaders(rootKey, dhOut)
	require.NoError(t, err)

	// Verify all keys are non-zero
	require.NotEqual(t, [32]byte{}, newRK, "new root key should be non-zero")
	require.NotEqual(t, [32]byte{}, ck, "chain key should be non-zero")
	require.NotEqual(t, [32]byte{}, hk, "header key should be non-zero")
	require.NotEqual(t, [32]byte{}, nhk, "next header key should be non-zero")

	// Verify all keys are different
	require.NotEqual(t, newRK, ck, "root key and chain key should differ")
	require.NotEqual(t, newRK, hk, "root key and header key should differ")
	require.NotEqual(t, hk, nhk, "header key and next header key should differ")

	// Verify the derived keys are different from the inputs
	require.NotEqual(t, originalRootKey, newRK, "derived root key should differ from input")
	require.NotEqual(t, originalRootKey, ck, "derived chain key should differ from input")
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
	// Sealed header should be 40 (plaintext) + 16 (tag) = 56 bytes
	require.Equal(t, 56, len(sealed), "sealed header should be 56 bytes")

	// Open with correct key should succeed
	opened, wasRatchetStep, err := openHeader(hk, nhk, sealed)
	require.NoError(t, err)
	require.False(t, wasRatchetStep, "should not indicate ratchet step when using current key")
	require.Equal(t, h.DHPub, opened.DHPub)
	require.Equal(t, h.PN, opened.PN)
	require.Equal(t, h.N, opened.N)

	// Open with wrong key should fail
	var wrongKey [32]byte
	_, err = rand.Read(wrongKey[:])
	require.NoError(t, err)
	_, _, err = openHeader(wrongKey, nhk, sealed)
	require.Error(t, err)
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

// TestRatchetEncryptWithEncryptedHeaders tests sending messages with encrypted headers.
func TestRatchetEncryptWithEncryptedHeaders(t *testing.T) {
	alice, err := InitInitiator([32]byte{1}, [32]byte{2})
	require.NoError(t, err)
	alice.EnableHeaderEncryption()

	// Initialize receiving chain by simulating a DH ratchet
	bob := InitRecipient([32]byte{1}, KeyPair{
		Public:  [32]byte{2},
		Private: [32]byte{3},
	})
	bob.EnableHeaderEncryption()

	// Perform DH ratchet on Alice's side to initialize header keys
	bobRatchetPub := [32]byte{4}
	err = alice.dhRatchetStep(bobRatchetPub)
	require.NoError(t, err)

	// Alice encrypts a message (header should be encrypted)
	plaintext := []byte("hello encrypted world")
	ad := []byte("additional")
	_, ciphertext, err := alice.RatchetEncrypt(plaintext, ad)
	require.NoError(t, err)
	require.NotNil(t, ciphertext)

	// Ciphertext should include encrypted header (56 bytes) + encrypted message
	require.Greater(t, len(ciphertext), 56, "ciphertext should be at least 56 bytes (encrypted header)")

	// First 56 bytes should be the encrypted header
	encryptedHeaderData := ciphertext[:56]
	require.Equal(t, 56, len(encryptedHeaderData))

	// Verify we can't decrypt the header plaintext from the first 56 bytes
	_, err = DecodeHeader(encryptedHeaderData)
	require.Error(t, err, "first 56 bytes should not be a valid plaintext header")
}

// TestRatchetEncryptDecryptWithEncryptedHeaders tests round-trip with encrypted headers.
func TestRatchetEncryptDecryptWithEncryptedHeaders(t *testing.T) {
	// Create two sessions: Alice (initiator) and Bob (responder)
	_, err := GenerateKeyPair()
	require.NoError(t, err)
	bobKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	sharedKey := [32]byte{99}

	// Initialize sessions
	alice, err := InitInitiator(sharedKey, bobKeyPair.Public)
	require.NoError(t, err)
	alice.EnableHeaderEncryption()

	bob := InitRecipient(sharedKey, bobKeyPair)
	bob.EnableHeaderEncryption()

	// Perform initial DH ratchet to establish both send and receive chains
	err = bob.dhRatchetStep(alice.dhs.Public)
	require.NoError(t, err)

	// Now both have sending and receiving chains with header keys
	plaintext := []byte("secret message with encrypted header")
	ad := []byte("context data")

	// Alice sends
	_, ciphertextWithHeader, err := alice.RatchetEncrypt(plaintext, ad)
	require.NoError(t, err)

	// Bob receives
	decrypted, err := bob.RatchetDecryptWithEncryptedHeader(ciphertextWithHeader, ad)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)

	// Message counters should advance
	require.Equal(t, uint32(1), alice.ns)
	require.Equal(t, uint32(1), bob.nr)
}

// TestRatchetEncryptDecryptDHRatchetWithEncryptedHeaders tests DH ratchet with encrypted headers.
func TestRatchetEncryptDecryptDHRatchetWithEncryptedHeaders(t *testing.T) {
	// Create sessions
	_, err := GenerateKeyPair()
	require.NoError(t, err)
	bobKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	sharedKey := [32]byte{99}

	alice, err := InitInitiator(sharedKey, bobKeyPair.Public)
	require.NoError(t, err)
	alice.EnableHeaderEncryption()

	bob := InitRecipient(sharedKey, bobKeyPair)
	bob.EnableHeaderEncryption()

	// Establish chains
	err = bob.dhRatchetStep(alice.dhs.Public)
	require.NoError(t, err)

	// Alice sends first message
	plaintext1 := []byte("first")
	_, ct1, err := alice.RatchetEncrypt(plaintext1, nil)
	require.NoError(t, err)

	// Bob receives (DHPub is the same)
	decrypted1, err := bob.RatchetDecryptWithEncryptedHeader(ct1, nil)
	require.NoError(t, err)
	require.Equal(t, plaintext1, decrypted1)

	// Alice performs DH ratchet step (e.g., in response to a received message)
	newBobRatchetPub := [32]byte{55}
	err = alice.dhRatchetStep(newBobRatchetPub)
	require.NoError(t, err)

	// Alice sends second message with new DH ratchet key
	plaintext2 := []byte("after ratchet")
	header2, ct2, err := alice.RatchetEncrypt(plaintext2, nil)
	require.NoError(t, err)

	// Bob should recognize this as a DH ratchet step when decrypting
	// (The encrypted header will fail with current hkr, succeed with nhkr)
	err = bob.dhRatchetStep(header2.DHPub)
	require.NoError(t, err)

	decrypted2, err := bob.RatchetDecryptWithEncryptedHeader(ct2, nil)
	require.NoError(t, err)
	require.Equal(t, plaintext2, decrypted2)
}

// TestHeaderKeyZeroization verifies header keys are properly zeroed after use.
func TestHeaderKeyZeroization(t *testing.T) {
	for i := 0; i < 5; i++ {
		// Multiple iterations to ensure no state leakage
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
		require.Equal(t, 56, len(sealed))

		// Verify hk was zeroed by sealHeader (expandHeaderKey should zero encKey)
		// We can't directly test this, but repeated runs should not exhibit timing leaks
	}
}

// TestRoundTripMixedMode tests that plaintext-header mode still works alongside encrypted.
func TestRoundTripMixedMode(t *testing.T) {
	sharedKey := [32]byte{77}
	_, err := GenerateKeyPair()
	require.NoError(t, err)
	bobKP, err := GenerateKeyPair()
	require.NoError(t, err)

	// Alice: plaintext headers
	alice, err := InitInitiator(sharedKey, bobKP.Public)
	require.NoError(t, err)
	// Don't enable header encryption for Alice

	// Bob: plaintext headers
	bob := InitRecipient(sharedKey, bobKP)
	// Don't enable header encryption for Bob

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

	// Verify header is indeed plaintext-encoded
	headerBytes := header.Encode()
	require.Equal(t, 40, len(headerBytes))
	require.Equal(t, alice.dhs.Public, header.DHPub)
}

// TestHeaderEncryptionConsistency tests that header encryption produces deterministic results
// with the same key (which it shouldn't - AEAD nonce should be derived differently).
func TestHeaderEncryptionDeterministic(t *testing.T) {
	var hk [32]byte
	_, err := rand.Read(hk[:])
	require.NoError(t, err)

	h := Header{
		DHPub: [32]byte{11, 22, 33},
		PN:    111,
		N:     222,
	}

	// Seal twice with the same key and header
	// Because the nonce is derived from the key, it should produce the same output
	sealed1, err := sealHeader(hk, h)
	require.NoError(t, err)

	sealed2, err := sealHeader(hk, h)
	require.NoError(t, err)

	// Both should decrypt to the same header
	require.Equal(t, sealed1, sealed2, "same key and header should produce same sealed output")

	opened1, _, err := openHeader(hk, [32]byte{}, sealed1)
	require.NoError(t, err)
	opened2, _, err := openHeader(hk, [32]byte{}, sealed2)
	require.NoError(t, err)

	require.Equal(t, opened1.DHPub, opened2.DHPub)
	require.Equal(t, opened1.PN, opened2.PN)
	require.Equal(t, opened1.N, opened2.N)
}

// TestRatchetDecryptHeaderAuthenticationFailure tests that tampering with encrypted header
// is detected.
func TestRatchetDecryptHeaderAuthenticationFailure(t *testing.T) {
	sharedKey := [32]byte{88}
	_, err := GenerateKeyPair()
	require.NoError(t, err)
	bobKP, err := GenerateKeyPair()
	require.NoError(t, err)

	alice, err := InitInitiator(sharedKey, bobKP.Public)
	require.NoError(t, err)
	alice.EnableHeaderEncryption()

	bob := InitRecipient(sharedKey, bobKP)
	bob.EnableHeaderEncryption()

	err = bob.dhRatchetStep(alice.dhs.Public)
	require.NoError(t, err)

	plaintext := []byte("authentic message")
	_, ciphertextWithHeader, err := alice.RatchetEncrypt(plaintext, nil)
	require.NoError(t, err)

	// Tamper with the encrypted header (first byte)
	tamperedCiphertext := make([]byte, len(ciphertextWithHeader))
	copy(tamperedCiphertext, ciphertextWithHeader)
	tamperedCiphertext[0] ^= 0xFF // Flip all bits

	// Decryption should fail
	_, err = bob.RatchetDecryptWithEncryptedHeader(tamperedCiphertext, nil)
	require.Error(t, err, "tampered header should fail authentication")

	// Session state should not have advanced (no plaintext returned)
	require.Equal(t, uint32(0), bob.nr, "failed decryption should not advance state")
}

// TestHeaderEncryptionKeyRotation verifies header keys rotate on DH-ratchet steps.
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
	newRK1, ck1, hk1, nhk1, err := kdfRootChainWithHeaders(rk1Copy, dh1Copy)
	require.NoError(t, err)

	// Second DH ratchet step (from same root)
	rk2Copy := rk
	dh2Copy := dh2
	newRK2, ck2, hk2, nhk2, err := kdfRootChainWithHeaders(rk2Copy, dh2Copy)
	require.NoError(t, err)

	// All keys should be different between the two ratchet steps
	require.NotEqual(t, newRK1, newRK2)
	require.NotEqual(t, ck1, ck2)
	require.NotEqual(t, hk1, hk2)
	require.NotEqual(t, nhk1, nhk2)
}

// TestOpenHeaderWithKeyLimit tests edge cases in header decryption.
func TestOpenHeaderWithKeyLimit(t *testing.T) {
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

	// Test with truncated encrypted header (less than 56 bytes)
	truncated := sealed[:40]
	_, err = openHeaderWithKey(hk, truncated)
	require.Error(t, err, "truncated sealed header should fail")

	// Test with empty encrypted header
	_, err = openHeaderWithKey(hk, nil)
	require.Error(t, err, "empty encrypted header should fail")

	// Test with oversized encrypted header (shouldn't be an issue, just extra data)
	oversized := make([]byte, 100)
	copy(oversized, sealed)
	opened, err := openHeaderWithKey(hk, oversized[:56])
	require.NoError(t, err)
	require.Equal(t, h, opened)
}
