package async

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

// encryptData encrypts data using AES-GCM with a key derived from the provided key material
func encryptData(data, keyMaterial []byte) ([]byte, error) {
	// Generate a random salt for HKDF
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	// Derive a key using HKDF with domain separation label and salt
	kdf := hkdf.New(sha256.New, keyMaterial, salt, []byte("toxcore-async-secure-storage-v1"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(kdf, key); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
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

	// Encrypt and include the salt and nonce at the beginning
	// Format: [salt (32 bytes)] [nonce (12 bytes)] [ciphertext]
	result := make([]byte, len(salt)+len(nonce))
	copy(result, salt)
	copy(result[len(salt):], nonce)
	ciphertext := aesGCM.Seal(result, nonce, data, nil)
	return ciphertext, nil
}

// decryptData decrypts data that was encrypted with encryptData
func decryptData(encryptedData, keyMaterial []byte) ([]byte, error) {
	// Expected format: [salt (32 bytes)] [nonce (12 bytes)] [ciphertext]
	const saltSize = 32
	const nonceSize = 12

	if len(encryptedData) < saltSize+nonceSize {
		return nil, errors.New("encrypted data too short")
	}

	// Extract salt and derive key using HKDF
	salt := encryptedData[:saltSize]
	kdf := hkdf.New(sha256.New, keyMaterial, salt, []byte("toxcore-async-secure-storage-v1"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(kdf, key); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Extract nonce and ciphertext
	nonce := encryptedData[saltSize : saltSize+nonceSize]
	ciphertext := encryptedData[saltSize+nonceSize:]

	return aesGCM.Open(nil, nonce, ciphertext, nil)
}
