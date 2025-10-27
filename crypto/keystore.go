package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptedKeyStore wraps file storage with AES-GCM encryption at rest.
// This provides defense-in-depth protection for sensitive cryptographic material
// even if the filesystem is compromised.
type EncryptedKeyStore struct {
	encryptionKey [32]byte
	dataDir       string
	saltFile      string
}

const (
	// PBKDF2Iterations is the number of iterations for key derivation (NIST recommendation)
	PBKDF2Iterations = 100000
	// EncryptionVersion is the current encryption format version
	EncryptionVersion = 1
	// SaltSize is the size of the salt for PBKDF2
	SaltSize = 32
)

// NewEncryptedKeyStore creates a key store with encryption at rest.
// masterPassword should be a user-provided passphrase or derived from system keyring.
// For production use, consider using a key derivation service or hardware security module.
//
// CWE-311: Missing Encryption of Sensitive Data (addressed)
func NewEncryptedKeyStore(dataDir string, masterPassword []byte) (*EncryptedKeyStore, error) {
	if len(masterPassword) == 0 {
		return nil, fmt.Errorf("master password cannot be empty")
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	ks := &EncryptedKeyStore{
		dataDir:  dataDir,
		saltFile: filepath.Join(dataDir, ".salt"),
	}

	// Load or generate salt
	salt, err := ks.loadOrGenerateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize salt: %w", err)
	}

	// Derive encryption key using PBKDF2
	// This makes brute-force attacks on the master password significantly more expensive
	derivedKey := pbkdf2.Key(masterPassword, salt, PBKDF2Iterations, 32, sha256.New)
	copy(ks.encryptionKey[:], derivedKey)

	// Securely wipe intermediate values
	SecureWipe(derivedKey)
	SecureWipe(masterPassword)

	return ks, nil
}

// loadOrGenerateSalt loads existing salt or generates a new one
func (ks *EncryptedKeyStore) loadOrGenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)

	// Try to load existing salt
	data, err := os.ReadFile(ks.saltFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read salt file: %w", err)
		}

		// Generate new salt
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}

		// Save salt with restricted permissions
		if err := os.WriteFile(ks.saltFile, salt, 0o600); err != nil {
			return nil, fmt.Errorf("failed to save salt: %w", err)
		}

		return salt, nil
	}

	if len(data) != SaltSize {
		return nil, fmt.Errorf("invalid salt file size: got %d, want %d", len(data), SaltSize)
	}

	copy(salt, data)
	return salt, nil
}

// WriteEncrypted encrypts and writes data to a file.
// Format: [version:2][nonce:12][ciphertext+tag:N]
//
// The encryption provides:
// - Confidentiality: AES-256-GCM encryption
// - Integrity: GCM authentication tag
// - Freshness: Unique nonce per encryption
func (ks *EncryptedKeyStore) WriteEncrypted(filename string, plaintext []byte) error {
	// Create AES cipher with our encryption key
	block, err := aes.NewCipher(ks.encryptionKey[:])
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode for authenticated encryption
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate unique nonce (critical for GCM security)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt with authentication
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Construct output: version || nonce || ciphertext
	output := make([]byte, 2+len(nonce)+len(ciphertext))
	binary.BigEndian.PutUint16(output[0:2], EncryptionVersion)
	copy(output[2:2+len(nonce)], nonce)
	copy(output[2+len(nonce):], ciphertext)

	// Atomic write using temporary file + rename
	tmpFile := filepath.Join(ks.dataDir, filename+".tmp")
	finalFile := filepath.Join(ks.dataDir, filename)

	if err := os.WriteFile(tmpFile, output, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	if err := os.Rename(tmpFile, finalFile); err != nil {
		// Clean up temporary file on error
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// ReadEncrypted reads and decrypts data from a file.
// Returns error if the file doesn't exist, is corrupted, or authentication fails.
func (ks *EncryptedKeyStore) ReadEncrypted(filename string) ([]byte, error) {
	// Read encrypted file
	filePath := filepath.Join(ks.dataDir, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Verify minimum size (version + nonce + tag)
	if len(data) < 2+12+16 {
		return nil, fmt.Errorf("file too short: %d bytes (minimum 30 bytes)", len(data))
	}

	// Check version
	version := binary.BigEndian.Uint16(data[0:2])
	if version != EncryptionVersion {
		return nil, fmt.Errorf("unsupported encryption version: %d (expected %d)", version, EncryptionVersion)
	}

	// Create AES cipher
	block, err := aes.NewCipher(ks.encryptionKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < 2+nonceSize {
		return nil, fmt.Errorf("file too short for nonce: %d bytes", len(data))
	}

	// Extract nonce and ciphertext
	nonce := data[2 : 2+nonceSize]
	ciphertext := data[2+nonceSize:]

	// Decrypt and verify authentication tag
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong password or corrupted data): %w", err)
	}

	return plaintext, nil
}

// DeleteEncrypted securely deletes an encrypted file.
// On most filesystems, this overwrites the file with zeros before deletion.
func (ks *EncryptedKeyStore) DeleteEncrypted(filename string) error {
	filePath := filepath.Join(ks.dataDir, filename)

	// Get file size
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Overwrite with zeros (best-effort secure deletion)
	zeros := make([]byte, info.Size())
	if err := os.WriteFile(filePath, zeros, 0o600); err != nil {
		// Continue with deletion even if overwrite fails
		return os.Remove(filePath)
	}

	// Delete the file
	return os.Remove(filePath)
}

// Close securely wipes the encryption key from memory.
// After calling Close, the EncryptedKeyStore should not be used.
func (ks *EncryptedKeyStore) Close() error {
	// Securely wipe encryption key
	ZeroBytes(ks.encryptionKey[:])
	return nil
}

// RotateKey derives a new encryption key from a new master password.
// This requires decrypting and re-encrypting all stored data.
// Returns error if any file operations fail.
func (ks *EncryptedKeyStore) RotateKey(newMasterPassword []byte) error {
	if len(newMasterPassword) == 0 {
		return fmt.Errorf("new master password cannot be empty")
	}

	// Find all encrypted files in the directory
	files, err := filepath.Glob(filepath.Join(ks.dataDir, "*"))
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	// Decrypt all files with current key
	fileData := make(map[string][]byte)
	for _, file := range files {
		if file == ks.saltFile || filepath.Ext(file) == ".tmp" {
			continue // Skip salt and temporary files
		}

		filename := filepath.Base(file)
		plaintext, err := ks.ReadEncrypted(filename)
		if err != nil {
			return fmt.Errorf("failed to decrypt %s: %w", filename, err)
		}
		fileData[filename] = plaintext
	}

	// Generate new salt
	newSalt := make([]byte, SaltSize)
	if _, err := rand.Read(newSalt); err != nil {
		return fmt.Errorf("failed to generate new salt: %w", err)
	}

	// Derive new encryption key
	newKey := pbkdf2.Key(newMasterPassword, newSalt, PBKDF2Iterations, 32, sha256.New)
	oldKey := ks.encryptionKey
	copy(ks.encryptionKey[:], newKey)
	SecureWipe(newKey)

	// Re-encrypt all files with new key
	for filename, plaintext := range fileData {
		if err := ks.WriteEncrypted(filename, plaintext); err != nil {
			// Restore old key on failure
			ks.encryptionKey = oldKey
			return fmt.Errorf("failed to re-encrypt %s: %w", filename, err)
		}
		SecureWipe(plaintext)
	}

	// Save new salt
	if err := os.WriteFile(ks.saltFile, newSalt, 0o600); err != nil {
		// Restore old key on failure
		ks.encryptionKey = oldKey
		return fmt.Errorf("failed to save new salt: %w", err)
	}

	// Wipe old key
	ZeroBytes(oldKey[:])
	SecureWipe(newMasterPassword)

	return nil
}
