package async

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// encryptData encrypts data using AES-GCM with a key derived from the provided key material
func encryptData(data []byte, keyMaterial []byte) ([]byte, error) {
	// Derive a key using SHA-256 from the provided key material
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
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt and include the nonce at the beginning
	ciphertext := aesGCM.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// decryptData decrypts data that was encrypted with encryptData
func decryptData(encryptedData []byte, keyMaterial []byte) ([]byte, error) {
	// Derive the key using SHA-256
	key := sha256.Sum256(keyMaterial)

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(encryptedData) < aesGCM.NonceSize() {
		return nil, errors.New("encrypted data too short")
	}

	nonce := encryptedData[:aesGCM.NonceSize()]
	ciphertext := encryptedData[aesGCM.NonceSize():]

	return aesGCM.Open(nil, nonce, ciphertext, nil)
}
