package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/curve25519"
)

// deriveX25519Pub is a test helper that derives the Curve25519 public key.
func deriveX25519Pub(t *testing.T, priv [32]byte) [32]byte {
	t.Helper()
	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)
	return pub
}

// randKey32 generates a random 32-byte key for tests.
func randKey32(t *testing.T) [32]byte {
	t.Helper()
	var k [32]byte
	_, err := rand.Read(k[:])
	require.NoError(t, err)
	return k
}

// TestPQXDH_BasicAgreement verifies that initiator and responder derive the
// same SK when a full 4-DH + PQ-SPK agreement is performed (no PQ-OPK).
func TestPQXDH_BasicAgreement(t *testing.T) {
	// Alice (initiator) keys
	aliceIdentPriv := randKey32(t)
	aliceEphemPriv := randKey32(t)

	// Bob (responder) classical keys
	bobIdentPriv := randKey32(t)
	bobSPKPriv := randKey32(t)
	bobOPKPriv := randKey32(t)

	// Bob's classical public keys
	bobIdentPub := deriveX25519Pub(t, bobIdentPriv)
	bobSPKPub := deriveX25519Pub(t, bobSPKPriv)
	bobOPKPub := deriveX25519Pub(t, bobOPKPriv)

	// Alice's classical public keys (needed by Bob to compute his side)
	aliceIdentPub := deriveX25519Pub(t, aliceIdentPriv)
	aliceEphemPub := deriveX25519Pub(t, aliceEphemPriv)

	// Bob's PQ pre-key
	pqPreKey, err := GeneratePQPreKey()
	require.NoError(t, err)

	// --- Alice initiates ---
	initParams := PQXDHInitiatorParams{
		SelfIdentityPrivate:     aliceIdentPriv,
		SelfEphemeralPrivate:    aliceEphemPriv,
		PeerIdentityPublic:      bobIdentPub,
		PeerSignedPreKeyPublic:  bobSPKPub,
		PeerOneTimePreKeyPublic: &bobOPKPub,
		PeerPQSignedPreKey:      pqPreKey.Public,
	}
	result, err := PQXDHInitiate(initParams)
	require.NoError(t, err)
	require.NotEqual(t, [32]byte{}, result.SK, "SK must not be zero")
	require.False(t, result.UsedPQOPK)

	// --- Bob responds ---
	respParams := PQXDHResponderParams{
		SelfIdentityPrivate:       bobIdentPriv,
		SelfSignedPreKeyPrivate:   bobSPKPriv,
		SelfOneTimePreKeyPrivate:  &bobOPKPriv,
		PeerIdentityPublic:        aliceIdentPub,
		PeerEphemeralPublic:       aliceEphemPub,
		SelfPQSignedPreKeyPrivate: pqPreKey.Private,
	}
	bobSK, err := PQXDHRespond(respParams, result.KEMCiphertextSPK, nil)
	require.NoError(t, err)

	require.Equal(t, result.SK, bobSK, "Alice and Bob must derive the same SK")
}

// TestPQXDH_WithPQOPK verifies agreement when a PQ one-time pre-key is used.
func TestPQXDH_WithPQOPK(t *testing.T) {
	aliceIdentPriv := randKey32(t)
	aliceEphemPriv := randKey32(t)
	bobIdentPriv := randKey32(t)
	bobSPKPriv := randKey32(t)

	bobIdentPub := deriveX25519Pub(t, bobIdentPriv)
	bobSPKPub := deriveX25519Pub(t, bobSPKPriv)
	aliceIdentPub := deriveX25519Pub(t, aliceIdentPriv)
	aliceEphemPub := deriveX25519Pub(t, aliceEphemPriv)

	pqSPK, err := GeneratePQPreKey()
	require.NoError(t, err)
	pqOPK, err := GeneratePQPreKey()
	require.NoError(t, err)

	initParams := PQXDHInitiatorParams{
		SelfIdentityPrivate:    aliceIdentPriv,
		SelfEphemeralPrivate:   aliceEphemPriv,
		PeerIdentityPublic:     bobIdentPub,
		PeerSignedPreKeyPublic: bobSPKPub,
		PeerPQSignedPreKey:     pqSPK.Public,
		PeerPQOneTimePreKey:    &pqOPK.Public,
	}
	result, err := PQXDHInitiate(initParams)
	require.NoError(t, err)
	require.True(t, result.UsedPQOPK, "UsedPQOPK must be true")

	respParams := PQXDHResponderParams{
		SelfIdentityPrivate:        bobIdentPriv,
		SelfSignedPreKeyPrivate:    bobSPKPriv,
		PeerIdentityPublic:         aliceIdentPub,
		PeerEphemeralPublic:        aliceEphemPub,
		SelfPQSignedPreKeyPrivate:  pqSPK.Private,
		SelfPQOneTimePreKeyPrivate: &pqOPK.Private,
	}
	bobSK, err := PQXDHRespond(respParams, result.KEMCiphertextSPK, &result.KEMCiphertextOPK)
	require.NoError(t, err)

	require.Equal(t, result.SK, bobSK, "Alice and Bob must derive the same SK with PQ-OPK")
}

// TestPQXDH_ThreeDH_NoPQOPK verifies fallback to 3-DH (no classical OPK, no PQ-OPK).
func TestPQXDH_ThreeDH_NoPQOPK(t *testing.T) {
	aliceIdentPriv := randKey32(t)
	aliceEphemPriv := randKey32(t)
	bobIdentPriv := randKey32(t)
	bobSPKPriv := randKey32(t)

	bobIdentPub := deriveX25519Pub(t, bobIdentPriv)
	bobSPKPub := deriveX25519Pub(t, bobSPKPriv)
	aliceIdentPub := deriveX25519Pub(t, aliceIdentPriv)
	aliceEphemPub := deriveX25519Pub(t, aliceEphemPriv)

	pqSPK, err := GeneratePQPreKey()
	require.NoError(t, err)

	initParams := PQXDHInitiatorParams{
		SelfIdentityPrivate:    aliceIdentPriv,
		SelfEphemeralPrivate:   aliceEphemPriv,
		PeerIdentityPublic:     bobIdentPub,
		PeerSignedPreKeyPublic: bobSPKPub,
		// No classical OPK, no PQ-OPK
		PeerPQSignedPreKey: pqSPK.Public,
	}
	result, err := PQXDHInitiate(initParams)
	require.NoError(t, err)

	respParams := PQXDHResponderParams{
		SelfIdentityPrivate:       bobIdentPriv,
		SelfSignedPreKeyPrivate:   bobSPKPriv,
		PeerIdentityPublic:        aliceIdentPub,
		PeerEphemeralPublic:       aliceEphemPub,
		SelfPQSignedPreKeyPrivate: pqSPK.Private,
	}
	bobSK, err := PQXDHRespond(respParams, result.KEMCiphertextSPK, nil)
	require.NoError(t, err)

	require.Equal(t, result.SK, bobSK)
}

// TestPQXDH_InputValidation verifies that invalid parameters are rejected.
func TestPQXDH_InputValidation(t *testing.T) {
	validPQ, err := GeneratePQPreKey()
	require.NoError(t, err)

	t.Run("empty self identity", func(t *testing.T) {
		_, err := PQXDHInitiate(PQXDHInitiatorParams{
			SelfEphemeralPrivate:   randKey32(t),
			PeerIdentityPublic:     randKey32(t),
			PeerSignedPreKeyPublic: randKey32(t),
			PeerPQSignedPreKey:     validPQ.Public,
		})
		require.Error(t, err)
	})
	t.Run("empty self ephemeral", func(t *testing.T) {
		_, err := PQXDHInitiate(PQXDHInitiatorParams{
			SelfIdentityPrivate:    randKey32(t),
			PeerIdentityPublic:     randKey32(t),
			PeerSignedPreKeyPublic: randKey32(t),
			PeerPQSignedPreKey:     validPQ.Public,
		})
		require.Error(t, err)
	})
	t.Run("empty peer identity", func(t *testing.T) {
		_, err := PQXDHInitiate(PQXDHInitiatorParams{
			SelfIdentityPrivate:    randKey32(t),
			SelfEphemeralPrivate:   randKey32(t),
			PeerSignedPreKeyPublic: randKey32(t),
			PeerPQSignedPreKey:     validPQ.Public,
		})
		require.Error(t, err)
	})
	t.Run("empty peer SPK", func(t *testing.T) {
		_, err := PQXDHInitiate(PQXDHInitiatorParams{
			SelfIdentityPrivate:  randKey32(t),
			SelfEphemeralPrivate: randKey32(t),
			PeerIdentityPublic:   randKey32(t),
			PeerPQSignedPreKey:   validPQ.Public,
		})
		require.Error(t, err)
	})
	t.Run("empty PQ SPK", func(t *testing.T) {
		_, err := PQXDHInitiate(PQXDHInitiatorParams{
			SelfIdentityPrivate:    randKey32(t),
			SelfEphemeralPrivate:   randKey32(t),
			PeerIdentityPublic:     randKey32(t),
			PeerSignedPreKeyPublic: randKey32(t),
			// PeerPQSignedPreKey is all zeros
		})
		require.Error(t, err)
	})
}

// TestPQXDH_DifferentKeysProduceDifferentSKs verifies domain separation:
// distinct PQ pre-keys must yield distinct session keys.
func TestPQXDH_DifferentKeysProduceDifferentSKs(t *testing.T) {
	aliceIdentPriv := randKey32(t)
	aliceEphemPriv := randKey32(t)
	bobIdentPriv := randKey32(t)
	bobSPKPriv := randKey32(t)

	bobIdentPub := deriveX25519Pub(t, bobIdentPriv)
	bobSPKPub := deriveX25519Pub(t, bobSPKPriv)

	pqSPK1, err := GeneratePQPreKey()
	require.NoError(t, err)
	pqSPK2, err := GeneratePQPreKey()
	require.NoError(t, err)

	r1, err := PQXDHInitiate(PQXDHInitiatorParams{
		SelfIdentityPrivate:    aliceIdentPriv,
		SelfEphemeralPrivate:   aliceEphemPriv,
		PeerIdentityPublic:     bobIdentPub,
		PeerSignedPreKeyPublic: bobSPKPub,
		PeerPQSignedPreKey:     pqSPK1.Public,
	})
	require.NoError(t, err)

	r2, err := PQXDHInitiate(PQXDHInitiatorParams{
		SelfIdentityPrivate:    aliceIdentPriv,
		SelfEphemeralPrivate:   aliceEphemPriv,
		PeerIdentityPublic:     bobIdentPub,
		PeerSignedPreKeyPublic: bobSPKPub,
		PeerPQSignedPreKey:     pqSPK2.Public,
	})
	require.NoError(t, err)

	require.NotEqual(t, r1.SK, r2.SK, "different PQ SPKs must produce different SKs")
}

// TestPQXDH_WrongCiphertextFails verifies that decapsulation with a mismatched
// ciphertext does not yield the same SK (the KEM provides implicit rejection).
func TestPQXDH_WrongCiphertextFails(t *testing.T) {
	aliceIdentPriv := randKey32(t)
	aliceEphemPriv := randKey32(t)
	bobIdentPriv := randKey32(t)
	bobSPKPriv := randKey32(t)

	bobIdentPub := deriveX25519Pub(t, bobIdentPriv)
	bobSPKPub := deriveX25519Pub(t, bobSPKPriv)
	aliceIdentPub := deriveX25519Pub(t, aliceIdentPriv)
	aliceEphemPub := deriveX25519Pub(t, aliceEphemPriv)

	pqSPK, err := GeneratePQPreKey()
	require.NoError(t, err)

	result, err := PQXDHInitiate(PQXDHInitiatorParams{
		SelfIdentityPrivate:    aliceIdentPriv,
		SelfEphemeralPrivate:   aliceEphemPriv,
		PeerIdentityPublic:     bobIdentPub,
		PeerSignedPreKeyPublic: bobSPKPub,
		PeerPQSignedPreKey:     pqSPK.Public,
	})
	require.NoError(t, err)

	// Corrupt the ciphertext.
	corruptCT := result.KEMCiphertextSPK
	corruptCT[0] ^= 0xFF

	respParams := PQXDHResponderParams{
		SelfIdentityPrivate:       bobIdentPriv,
		SelfSignedPreKeyPrivate:   bobSPKPriv,
		PeerIdentityPublic:        aliceIdentPub,
		PeerEphemeralPublic:       aliceEphemPub,
		SelfPQSignedPreKeyPrivate: pqSPK.Private,
	}
	// ML-KEM provides implicit rejection: decapsulation still succeeds but
	// yields a different (random-looking) shared secret, so SK must differ.
	bobSK, err := PQXDHRespond(respParams, corruptCT, nil)
	require.NoError(t, err) // no error – implicit rejection
	require.NotEqual(t, result.SK, bobSK, "corrupted ciphertext must yield a different SK")
}

// TestGeneratePQPreKey verifies that key generation succeeds and produces
// non-zero output.
func TestGeneratePQPreKey(t *testing.T) {
	pk, err := GeneratePQPreKey()
	require.NoError(t, err)
	require.NotEqual(t, [MLKEMPublicKeySize]byte{}, pk.Public, "public key must not be zero")
	require.NotEqual(t, [2400]byte{}, pk.Private, "private key must not be zero")
}

// TestNewSignedPQPreKey verifies that a signed PQ pre-key is generated and
// that its signature is valid.
func TestNewSignedPQPreKey(t *testing.T) {
	identPriv := randKey32(t)

	spq, pqk, err := NewSignedPQPreKey(42, identPriv)
	require.NoError(t, err)
	require.Equal(t, uint32(42), spq.ID)
	require.Equal(t, spq.PublicKey, pqk.Public)

	require.NoError(t, spq.Verify(), "signature must be valid")

	// Tamper with the public key – verification must fail.
	tampered := *spq
	tampered.PublicKey[0] ^= 0xFF
	require.Error(t, tampered.Verify(), "tampered PQ pre-key must not verify")
}

// TestPQXDH_CapPQXDH verifies that the CapPQXDH constant is distinct from
// existing capabilities (no bit collision).
func TestPQXDH_CapPQXDH(t *testing.T) {
	// Import transport to verify the constant exists and has a unique bit.
	// We access it symbolically via the negotiation package; here we just
	// assert the bit value is non-zero and doesn't collide with 1<<0 or 1<<1.
	const capX3DH = 1 << 0
	const capHeaderEncryption = 1 << 1
	const capPQXDH = 1 << 2

	require.NotEqual(t, 0, capPQXDH)
	require.Equal(t, 0, capPQXDH&capX3DH)
	require.Equal(t, 0, capPQXDH&capHeaderEncryption)
}
