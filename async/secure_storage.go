package async

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	toxcrypto "github.com/opd-ai/toxcore/crypto"
	"golang.org/x/crypto/hkdf"
)

// secureStorageInfoLabel provides domain separation for HKDF key derivation.
// Using a stable context label prevents cross-protocol key reuse.
const secureStorageInfoLabel = "toxcore-async-secure-storage-v1"

// deriveAESGCM derives an AES-256-GCM cipher from key material and salt using HKDF.
func deriveAESGCM(keyMaterial, salt []byte) (cipher.AEAD, error) {
	kdf := hkdf.New(sha256.New, keyMaterial, salt, []byte(secureStorageInfoLabel))
	key := make([]byte, 32)
	defer toxcrypto.ZeroBytes(key)
	if _, err := io.ReadFull(kdf, key); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

// encryptData encrypts data using AES-GCM with a key derived from the provided key material
func encryptData(data, keyMaterial []byte) ([]byte, error) {
	// Generate a random salt for HKDF
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	aesGCM, err := deriveAESGCM(keyMaterial, salt)
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
	// New format: [salt (32 bytes)] [nonce (12 bytes)] [ciphertext]
	// Legacy format fallback: [nonce (12 bytes)] [ciphertext]
	const (
		saltSize  = 32
		nonceSize = 12
	)

	if len(encryptedData) >= saltSize+nonceSize {
		plaintext, err := decryptDataWithHKDF(encryptedData, keyMaterial, saltSize, nonceSize)
		if err == nil {
			return plaintext, nil
		}
	}

	return decryptDataLegacy(encryptedData, keyMaterial, nonceSize)
}

func decryptDataWithHKDF(encryptedData, keyMaterial []byte, saltSize, nonceSize int) ([]byte, error) {
	// Extract salt and derive AES-GCM cipher using HKDF
	salt := encryptedData[:saltSize]
	aesGCM, err := deriveAESGCM(keyMaterial, salt)
	if err != nil {
		return nil, err
	}

	// Extract nonce and ciphertext
	nonce := encryptedData[saltSize : saltSize+nonceSize]
	ciphertext := encryptedData[saltSize+nonceSize:]

	return aesGCM.Open(nil, nonce, ciphertext, nil)
}

func decryptDataLegacy(encryptedData, keyMaterial []byte, nonceSize int) ([]byte, error) {
	if len(encryptedData) < nonceSize {
		return nil, errors.New("encrypted data too short")
	}

	legacyKey := sha256.Sum256(keyMaterial)
	defer toxcrypto.ZeroBytes(legacyKey[:])

	block, err := aes.NewCipher(legacyKey[:])
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := encryptedData[:nonceSize]
	ciphertext := encryptedData[nonceSize:]

	return aesGCM.Open(nil, nonce, ciphertext, nil)
}
