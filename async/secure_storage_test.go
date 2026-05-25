package async

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecryptDataLegacyFormatFallback(t *testing.T) {
	keyMaterial := []byte("legacy-storage-key-material")

	testCases := []struct {
		name      string
		plaintext []byte
	}{
		{
			name:      "short legacy payload skips HKDF path",
			plaintext: []byte("legacy"),
		},
		{
			name:      "longer legacy payload falls back after HKDF failure",
			plaintext: bytes.Repeat([]byte("a"), 64),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encryptedData, err := encryptLegacyData(tc.plaintext, keyMaterial)
			require.NoError(t, err)

			decryptedData, err := decryptData(encryptedData, keyMaterial)
			require.NoError(t, err)
			require.Equal(t, tc.plaintext, decryptedData)
		})
	}
}

func encryptLegacyData(data, keyMaterial []byte) ([]byte, error) {
	key := sha256.Sum256(keyMaterial)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return aesGCM.Seal(nonce, nonce, data, nil), nil
}
