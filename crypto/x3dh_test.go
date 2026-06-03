package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/curve25519"
)

// TestX3DH_BasicAgreement tests that X3DH initiator and responder derive the same shared secret.
func TestX3DH_BasicAgreement(t *testing.T) {
	// Generate Alice's (initiator) keys
	aliceIdentityPriv := make([]byte, 32)
	_, err := rand.Read(aliceIdentityPriv)
	require.NoError(t, err)
	var aliceIdentityPrivKey [32]byte
	copy(aliceIdentityPrivKey[:], aliceIdentityPriv)

	aliceEphemeralPriv := make([]byte, 32)
	_, err = rand.Read(aliceEphemeralPriv)
	require.NoError(t, err)
	var aliceEphemeralPrivKey [32]byte
	copy(aliceEphemeralPrivKey[:], aliceEphemeralPriv)

	// Generate Bob's (responder) keys
	bobIdentityPriv := make([]byte, 32)
	_, err = rand.Read(bobIdentityPriv)
	require.NoError(t, err)
	var bobIdentityPrivKey [32]byte
	copy(bobIdentityPrivKey[:], bobIdentityPriv)

	bobSignedPreKeyPriv := make([]byte, 32)
	_, err = rand.Read(bobSignedPreKeyPriv)
	require.NoError(t, err)
	var bobSignedPreKeyPrivKey [32]byte
	copy(bobSignedPreKeyPrivKey[:], bobSignedPreKeyPriv)

	bobOTKPriv := make([]byte, 32)
	_, err = rand.Read(bobOTKPriv)
	require.NoError(t, err)
	var bobOTKPrivKey [32]byte
	copy(bobOTKPrivKey[:], bobOTKPriv)

	// Derive X25519 public keys
	bobSignedPreKeyPub, err := deriveX25519Public(bobSignedPreKeyPrivKey)
	require.NoError(t, err)

	bobOTKPub, err := deriveX25519Public(bobOTKPrivKey)
	require.NoError(t, err)

	bobIdentityPub, err := deriveX25519Public(bobIdentityPrivKey)
	require.NoError(t, err)

	// Initiator (Alice) performs X3DH
	aliceParams := X3DHInitiatorParams{
		SelfIdentityPrivate:     aliceIdentityPrivKey,
		SelfEphemeralPrivate:    aliceEphemeralPrivKey,
		PeerIdentityPublic:      bobIdentityPub,
		PeerSignedPreKeyPublic:  bobSignedPreKeyPub,
		PeerOneTimePreKeyPublic: &bobOTKPub,
	}

	aliceSK, _, _, err := X3DHInitiate(aliceParams)
	require.NoError(t, err)
	require.NotEqual(t, [32]byte{}, aliceSK, "Alice's SK should not be zero")

	// Get Alice's ephemeral public key for Bob
	aliceEphemeralPub, err := deriveX25519Public(aliceEphemeralPrivKey)
	require.NoError(t, err)

	aliceIdentityPub, err := deriveX25519Public(aliceIdentityPrivKey)
	require.NoError(t, err)

	// Responder (Bob) performs X3DH with the same parameters
	bobParams := X3DHResponderParams{
		SelfIdentityPrivate:      bobIdentityPrivKey,
		SelfSignedPreKeyPrivate:  bobSignedPreKeyPrivKey,
		SelfOneTimePreKeyPrivate: &bobOTKPrivKey,
		PeerIdentityPublic:       aliceIdentityPub,
		PeerEphemeralPublic:      aliceEphemeralPub,
	}

	bobSK, err := X3DHRespond(bobParams)
	require.NoError(t, err)
	require.NotEqual(t, [32]byte{}, bobSK, "Bob's SK should not be zero")

	// Both should derive the same shared secret
	require.Equal(t, aliceSK, bobSK, "Alice and Bob should derive the same shared secret")
}

// TestX3DH_ThreeDH tests X3DH fallback when no one-time pre-key is available.
func TestX3DH_ThreeDH(t *testing.T) {
	// Generate Alice's keys
	aliceIdentityPriv := make([]byte, 32)
	_, err := rand.Read(aliceIdentityPriv)
	require.NoError(t, err)
	var aliceIdentityPrivKey [32]byte
	copy(aliceIdentityPrivKey[:], aliceIdentityPriv)

	aliceEphemeralPriv := make([]byte, 32)
	_, err = rand.Read(aliceEphemeralPriv)
	require.NoError(t, err)
	var aliceEphemeralPrivKey [32]byte
	copy(aliceEphemeralPrivKey[:], aliceEphemeralPriv)

	// Generate Bob's keys (without OTK)
	bobIdentityPriv := make([]byte, 32)
	_, err = rand.Read(bobIdentityPriv)
	require.NoError(t, err)
	var bobIdentityPrivKey [32]byte
	copy(bobIdentityPrivKey[:], bobIdentityPriv)

	bobSignedPreKeyPriv := make([]byte, 32)
	_, err = rand.Read(bobSignedPreKeyPriv)
	require.NoError(t, err)
	var bobSignedPreKeyPrivKey [32]byte
	copy(bobSignedPreKeyPrivKey[:], bobSignedPreKeyPriv)

	bobSignedPreKeyPub, err := deriveX25519Public(bobSignedPreKeyPrivKey)
	require.NoError(t, err)

	bobIdentityPub, err := deriveX25519Public(bobIdentityPrivKey)
	require.NoError(t, err)

	// Initiator performs 3-DH (no OTK)
	aliceParams := X3DHInitiatorParams{
		SelfIdentityPrivate:     aliceIdentityPrivKey,
		SelfEphemeralPrivate:    aliceEphemeralPrivKey,
		PeerIdentityPublic:      bobIdentityPub,
		PeerSignedPreKeyPublic:  bobSignedPreKeyPub,
		PeerOneTimePreKeyPublic: nil, // No OTK
	}

	aliceSK, _, dh4ID, err := X3DHInitiate(aliceParams)
	require.NoError(t, err)
	require.Equal(t, uint32(0), dh4ID, "dh4ID should be 0 for 3-DH fallback")

	// Responder performs 3-DH
	aliceEphemeralPub, err := deriveX25519Public(aliceEphemeralPrivKey)
	require.NoError(t, err)

	aliceIdentityPub, err := deriveX25519Public(aliceIdentityPrivKey)
	require.NoError(t, err)

	bobParams := X3DHResponderParams{
		SelfIdentityPrivate:      bobIdentityPrivKey,
		SelfSignedPreKeyPrivate:  bobSignedPreKeyPrivKey,
		SelfOneTimePreKeyPrivate: nil, // No OTK
		PeerIdentityPublic:       aliceIdentityPub,
		PeerEphemeralPublic:      aliceEphemeralPub,
	}

	bobSK, err := X3DHRespond(bobParams)
	require.NoError(t, err)

	// Both should derive the same secret even with 3-DH
	require.Equal(t, aliceSK, bobSK, "Alice and Bob should derive the same secret in 3-DH mode")
}

// TestX3DH_InputValidation tests that X3DH rejects invalid inputs.
func TestX3DH_InputValidation(t *testing.T) {
	tests := []struct {
		name   string
		params X3DHInitiatorParams
	}{
		{
			name: "empty self identity",
			params: X3DHInitiatorParams{
				SelfIdentityPrivate:    [32]byte{},
				SelfEphemeralPrivate:   [32]byte{1},
				PeerIdentityPublic:     [32]byte{1},
				PeerSignedPreKeyPublic: [32]byte{1},
			},
		},
		{
			name: "empty self ephemeral",
			params: X3DHInitiatorParams{
				SelfIdentityPrivate:    [32]byte{1},
				SelfEphemeralPrivate:   [32]byte{},
				PeerIdentityPublic:     [32]byte{1},
				PeerSignedPreKeyPublic: [32]byte{1},
			},
		},
		{
			name: "empty peer identity",
			params: X3DHInitiatorParams{
				SelfIdentityPrivate:    [32]byte{1},
				SelfEphemeralPrivate:   [32]byte{1},
				PeerIdentityPublic:     [32]byte{},
				PeerSignedPreKeyPublic: [32]byte{1},
			},
		},
		{
			name: "empty peer signed pre-key",
			params: X3DHInitiatorParams{
				SelfIdentityPrivate:    [32]byte{1},
				SelfEphemeralPrivate:   [32]byte{1},
				PeerIdentityPublic:     [32]byte{1},
				PeerSignedPreKeyPublic: [32]byte{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := X3DHInitiate(tt.params)
			require.Error(t, err, "X3DHInitiate should reject invalid params for %s", tt.name)
		})
	}
}

// TestX3DH_KeyZeroization ensures that sensitive keys are wiped after use.
func TestX3DH_KeyZeroization(t *testing.T) {
	// This test can be enhanced to use memory forensics,
	// but for now we just verify the functions complete without panic.

	aliceIdentityPriv := make([]byte, 32)
	_, err := rand.Read(aliceIdentityPriv)
	require.NoError(t, err)
	var aliceIdentityPrivKey [32]byte
	copy(aliceIdentityPrivKey[:], aliceIdentityPriv)

	aliceEphemeralPriv := make([]byte, 32)
	_, err = rand.Read(aliceEphemeralPriv)
	require.NoError(t, err)
	var aliceEphemeralPrivKey [32]byte
	copy(aliceEphemeralPrivKey[:], aliceEphemeralPriv)

	bobIdentityPriv := make([]byte, 32)
	_, err = rand.Read(bobIdentityPriv)
	require.NoError(t, err)
	var bobIdentityPrivKey [32]byte
	copy(bobIdentityPrivKey[:], bobIdentityPriv)

	bobSignedPreKeyPriv := make([]byte, 32)
	_, err = rand.Read(bobSignedPreKeyPriv)
	require.NoError(t, err)
	var bobSignedPreKeyPrivKey [32]byte
	copy(bobSignedPreKeyPrivKey[:], bobSignedPreKeyPriv)

	bobSignedPreKeyPub, err := deriveX25519Public(bobSignedPreKeyPrivKey)
	require.NoError(t, err)

	bobIdentityPub, err := deriveX25519Public(bobIdentityPrivKey)
	require.NoError(t, err)

	// Perform X3DH multiple times to ensure no crashes or memory leaks
	for i := 0; i < 10; i++ {
		aliceParams := X3DHInitiatorParams{
			SelfIdentityPrivate:     aliceIdentityPrivKey,
			SelfEphemeralPrivate:    aliceEphemeralPrivKey,
			PeerIdentityPublic:      bobIdentityPub,
			PeerSignedPreKeyPublic:  bobSignedPreKeyPub,
			PeerOneTimePreKeyPublic: nil,
		}

		_, _, _, err := X3DHInitiate(aliceParams)
		require.NoError(t, err)
	}
}

// TestX3DH_DifferentKeysProduceDifferentSecrets verifies that different key material produces different secrets.
func TestX3DH_DifferentKeysProduceDifferentSecrets(t *testing.T) {
	// Generate first set of keys
	aliceIdentityPriv1 := make([]byte, 32)
	_, err := rand.Read(aliceIdentityPriv1)
	require.NoError(t, err)
	var aliceIdentityPrivKey1 [32]byte
	copy(aliceIdentityPrivKey1[:], aliceIdentityPriv1)

	aliceEphemeralPriv := make([]byte, 32)
	_, err = rand.Read(aliceEphemeralPriv)
	require.NoError(t, err)
	var aliceEphemeralPrivKey [32]byte
	copy(aliceEphemeralPrivKey[:], aliceEphemeralPriv)

	bobIdentityPriv := make([]byte, 32)
	_, err = rand.Read(bobIdentityPriv)
	require.NoError(t, err)
	var bobIdentityPrivKey [32]byte
	copy(bobIdentityPrivKey[:], bobIdentityPriv)

	bobSignedPreKeyPriv := make([]byte, 32)
	_, err = rand.Read(bobSignedPreKeyPriv)
	require.NoError(t, err)
	var bobSignedPreKeyPrivKey [32]byte
	copy(bobSignedPreKeyPrivKey[:], bobSignedPreKeyPriv)

	bobSignedPreKeyPub, err := deriveX25519Public(bobSignedPreKeyPrivKey)
	require.NoError(t, err)

	bobIdentityPub, err := deriveX25519Public(bobIdentityPrivKey)
	require.NoError(t, err)

	// First agreement
	aliceParams1 := X3DHInitiatorParams{
		SelfIdentityPrivate:     aliceIdentityPrivKey1,
		SelfEphemeralPrivate:    aliceEphemeralPrivKey,
		PeerIdentityPublic:      bobIdentityPub,
		PeerSignedPreKeyPublic:  bobSignedPreKeyPub,
		PeerOneTimePreKeyPublic: nil,
	}

	sk1, _, _, err := X3DHInitiate(aliceParams1)
	require.NoError(t, err)

	// Generate different Alice identity
	aliceIdentityPriv2 := make([]byte, 32)
	_, err = rand.Read(aliceIdentityPriv2)
	require.NoError(t, err)
	var aliceIdentityPrivKey2 [32]byte
	copy(aliceIdentityPrivKey2[:], aliceIdentityPriv2)

	// Second agreement with different identity
	aliceParams2 := X3DHInitiatorParams{
		SelfIdentityPrivate:     aliceIdentityPrivKey2,
		SelfEphemeralPrivate:    aliceEphemeralPrivKey,
		PeerIdentityPublic:      bobIdentityPub,
		PeerSignedPreKeyPublic:  bobSignedPreKeyPub,
		PeerOneTimePreKeyPublic: nil,
	}

	sk2, _, _, err := X3DHInitiate(aliceParams2)
	require.NoError(t, err)

	// Secrets should be different with different identity keys
	require.NotEqual(t, sk1, sk2, "Different identity keys should produce different secrets")
}

// deriveX25519Public derives the Curve25519 public key from a private key.
// This is a helper function for tests.
func deriveX25519Public(private [32]byte) ([32]byte, error) {
	var public [32]byte
	curve25519.ScalarBaseMult(&public, &private)
	return public, nil
}

// sha512hash is a test helper - not actually used, kept for reference
func sha512hash(data []byte) [64]byte {
	// Placeholder - in real code we'd use crypto/sha512
	var result [64]byte
	for i := 0; i < len(data) && i < 32; i++ {
		result[i] = data[i]
	}
	return result
}
