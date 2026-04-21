package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"
)

// TestEncryptDecryptRoundtripProperty verifies that Encrypt followed by
// Decrypt always produces the original plaintext for any valid input.
func TestEncryptDecryptRoundtripProperty(t *testing.T) {
	err := quick.Check(func(plaintext []byte) bool {
		// Skip empty or unrealistically large payloads.
		if len(plaintext) == 0 || len(plaintext) > 4096 {
			return true
		}

		sender, err := GenerateKeyPair()
		if err != nil {
			return false
		}
		receiver, err := GenerateKeyPair()
		if err != nil {
			return false
		}
		nonce, err := GenerateNonce()
		if err != nil {
			return false
		}

		ciphertext, err := Encrypt(plaintext, nonce, receiver.Public, sender.Private)
		if err != nil {
			return false
		}

		decrypted, err := Decrypt(ciphertext, nonce, sender.Public, receiver.Private)
		if err != nil {
			return false // decrypt must succeed when encrypt succeeded
		}

		return bytes.Equal(plaintext, decrypted)
	}, &quick.Config{MaxCount: 200})
	require.NoError(t, err, "Encrypt/Decrypt roundtrip property violated")
}

// TestSharedSecretSymmetryProperty verifies that DeriveSharedSecret(A.pub, B.priv)
// equals DeriveSharedSecret(B.pub, A.priv) for any valid key pair.
func TestSharedSecretSymmetryProperty(t *testing.T) {
	err := quick.Check(func(_ []byte) bool {
		a, err := GenerateKeyPair()
		if err != nil {
			return false
		}
		b, err := GenerateKeyPair()
		if err != nil {
			return false
		}

		ab, err := DeriveSharedSecret(b.Public, a.Private)
		if err != nil {
			return false
		}
		ba, err := DeriveSharedSecret(a.Public, b.Private)
		if err != nil {
			return false
		}

		return ab == ba
	}, &quick.Config{MaxCount: 200})
	require.NoError(t, err, "SharedSecret symmetry property violated")
}

// TestGenerateKeyPairUniquenessProperty verifies that successive calls to
// GenerateKeyPair never produce the same key pair.
func TestGenerateKeyPairUniquenessProperty(t *testing.T) {
	seen := make(map[[KeySize]byte]struct{}, 100)
	for i := 0; i < 100; i++ {
		kp, err := GenerateKeyPair()
		require.NoError(t, err)
		if _, ok := seen[kp.Public]; ok {
			t.Fatalf("GenerateKeyPair produced duplicate public key on iteration %d", i)
		}
		seen[kp.Public] = struct{}{}
	}
}

// TestEncryptAuthenticationProperty verifies that a ciphertext modified at any
// byte fails to decrypt (authenticated encryption).
func TestEncryptAuthenticationProperty(t *testing.T) {
	sender, err := GenerateKeyPair()
	require.NoError(t, err)
	receiver, err := GenerateKeyPair()
	require.NoError(t, err)
	nonce, err := GenerateNonce()
	require.NoError(t, err)

	plaintext := make([]byte, 64)
	_, err = rand.Read(plaintext)
	require.NoError(t, err)

	ciphertext, err := Encrypt(plaintext, nonce, receiver.Public, sender.Private)
	require.NoError(t, err)

	// Flip every bit in every byte of the ciphertext.
	// Every modification must cause decryption to fail.
	for i := range ciphertext {
		tampered := make([]byte, len(ciphertext))
		copy(tampered, ciphertext)
		tampered[i] ^= 0xff

		_, err := Decrypt(tampered, nonce, sender.Public, receiver.Private)
		if err == nil {
			t.Errorf("Decrypt succeeded on tampered ciphertext at byte %d", i)
		}
	}
}

// TestSymmetricEncryptDecryptRoundtripProperty verifies that EncryptSymmetric
// followed by DecryptSymmetric always recovers the original plaintext.
func TestSymmetricEncryptDecryptRoundtripProperty(t *testing.T) {
	err := quick.Check(func(plaintext []byte) bool {
		// Skip empty or unrealistically large payloads.
		if len(plaintext) == 0 || len(plaintext) > 4096 {
			return true
		}

		var key [KeySize]byte
		if _, err := rand.Read(key[:]); err != nil {
			return false
		}
		nonce, err := GenerateNonce()
		if err != nil {
			return false
		}

		ct, err := EncryptSymmetric(plaintext, nonce, key)
		if err != nil {
			return false
		}

		pt, err := DecryptSymmetric(ct, nonce, key)
		if err != nil {
			return false
		}

		return bytes.Equal(plaintext, pt)
	}, &quick.Config{MaxCount: 200})
	require.NoError(t, err, "EncryptSymmetric/DecryptSymmetric roundtrip property violated")
}

// TestSignVerifyRoundtripProperty verifies that a valid signature always
// passes verification for any message.
func TestSignVerifyRoundtripProperty(t *testing.T) {
	err := quick.Check(func(message []byte) bool {
		if len(message) == 0 || len(message) > 4096 {
			return true
		}

		privKey, _, err := GenerateEd25519KeyPair()
		if err != nil {
			return false
		}

		sig, err := SignWithPrivateKey(privKey, message)
		if err != nil {
			return false
		}

		pubKey := privKey[32:]
		var pubKeyArr [32]byte
		copy(pubKeyArr[:], pubKey)

		valid, err := Verify(message, sig, pubKeyArr)
		if err != nil {
			return false
		}

		return valid
	}, &quick.Config{MaxCount: 200})
	require.NoError(t, err, "Sign/Verify roundtrip property violated")
}

// TestNonceDistinctnessProperty verifies that GenerateNonce never returns
// the same nonce twice in a reasonable sample.
func TestNonceDistinctnessProperty(t *testing.T) {
	seen := make(map[Nonce]struct{}, 100)
	for i := 0; i < 100; i++ {
		n, err := GenerateNonce()
		require.NoError(t, err)
		if _, ok := seen[n]; ok {
			t.Fatalf("GenerateNonce produced duplicate nonce on iteration %d", i)
		}
		seen[n] = struct{}{}
	}
}

// TestFromSecretKeyRoundtripProperty verifies that FromSecretKey preserves the
// private key value and derives a consistent public key.
func TestFromSecretKeyRoundtripProperty(t *testing.T) {
	err := quick.Check(func(_ []byte) bool {
		original, err := GenerateKeyPair()
		if err != nil {
			return false
		}

		restored, err := FromSecretKey(original.Private)
		if err != nil {
			return false
		}

		return original.Public == restored.Public && original.Private == restored.Private
	}, &quick.Config{MaxCount: 200})
	require.NoError(t, err, "FromSecretKey roundtrip property violated")
}
