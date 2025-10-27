package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNewEncryptedKeyStore(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password-123")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatalf("Failed to create key store: %v", err)
	}
	defer ks.Close()

	// Verify salt file was created
	saltPath := filepath.Join(tempDir, ".salt")
	if _, err := os.Stat(saltPath); os.IsNotExist(err) {
		t.Error("Salt file was not created")
	}

	// Verify salt has correct size
	salt, err := os.ReadFile(saltPath)
	if err != nil {
		t.Fatalf("Failed to read salt: %v", err)
	}
	if len(salt) != SaltSize {
		t.Errorf("Salt size = %d, want %d", len(salt), SaltSize)
	}
}

func TestEncryptedKeyStore_WriteRead(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password-456")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatal(err)
	}
	defer ks.Close()

	testData := []byte("sensitive-pre-key-data-12345")

	// Write encrypted data
	err = ks.WriteEncrypted("test.dat", testData)
	if err != nil {
		t.Fatalf("Failed to write encrypted: %v", err)
	}

	// Read encrypted data
	decrypted, err := ks.ReadEncrypted("test.dat")
	if err != nil {
		t.Fatalf("Failed to read encrypted: %v", err)
	}

	// Verify data matches
	if !bytes.Equal(testData, decrypted) {
		t.Errorf("Decrypted data doesn't match original\nGot:  %s\nWant: %s", decrypted, testData)
	}

	// Verify data is actually encrypted on disk
	rawData, err := os.ReadFile(filepath.Join(tempDir, "test.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(rawData, testData) {
		t.Error("Data appears to be stored in plaintext")
	}
}

func TestEncryptedKeyStore_WrongPassword(t *testing.T) {
	tempDir := t.TempDir()
	password1 := []byte("correct-password")
	password2 := []byte("wrong-password")

	// Create key store with password1
	ks1, err := NewEncryptedKeyStore(tempDir, password1)
	if err != nil {
		t.Fatal(err)
	}

	testData := []byte("secret-data")
	err = ks1.WriteEncrypted("test.dat", testData)
	if err != nil {
		t.Fatal(err)
	}
	ks1.Close()

	// Try to read with password2
	ks2, err := NewEncryptedKeyStore(tempDir, password2)
	if err != nil {
		t.Fatal(err)
	}
	defer ks2.Close()

	_, err = ks2.ReadEncrypted("test.dat")
	if err == nil {
		t.Error("Expected error when reading with wrong password")
	}
}

func TestEncryptedKeyStore_KeyDerivationConsistency(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password")

	// Create first key store
	ks1, err := NewEncryptedKeyStore(tempDir, append([]byte(nil), password...))
	if err != nil {
		t.Fatal(err)
	}
	key1 := ks1.encryptionKey
	ks1.Close()

	// Create second key store with same password (make a copy since it gets wiped)
	ks2, err := NewEncryptedKeyStore(tempDir, append([]byte(nil), password...))
	if err != nil {
		t.Fatal(err)
	}
	defer ks2.Close()
	key2 := ks2.encryptionKey

	// Keys should be identical (same salt)
	if !bytes.Equal(key1[:], key2[:]) {
		t.Error("Derived keys should be identical with same password and salt")
	}
}

func TestEncryptedKeyStore_DifferentPasswords(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	password1 := []byte("password1")
	password2 := []byte("password2")

	ks1, _ := NewEncryptedKeyStore(tempDir1, password1)
	defer ks1.Close()

	ks2, _ := NewEncryptedKeyStore(tempDir2, password2)
	defer ks2.Close()

	// Keys should be different
	if bytes.Equal(ks1.encryptionKey[:], ks2.encryptionKey[:]) {
		t.Error("Different passwords should produce different keys")
	}
}

func TestEncryptedKeyStore_EmptyPassword(t *testing.T) {
	tempDir := t.TempDir()
	emptyPassword := []byte("")

	_, err := NewEncryptedKeyStore(tempDir, emptyPassword)
	if err == nil {
		t.Error("Should reject empty password")
	}
}

func TestEncryptedKeyStore_LargeData(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatal(err)
	}
	defer ks.Close()

	// Test with 1MB of data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err = ks.WriteEncrypted("large.dat", largeData)
	if err != nil {
		t.Fatalf("Failed to write large data: %v", err)
	}

	decrypted, err := ks.ReadEncrypted("large.dat")
	if err != nil {
		t.Fatalf("Failed to read large data: %v", err)
	}

	if !bytes.Equal(largeData, decrypted) {
		t.Error("Large data roundtrip failed")
	}
}

func TestEncryptedKeyStore_NonexistentFile(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatal(err)
	}
	defer ks.Close()

	_, err = ks.ReadEncrypted("nonexistent.dat")
	if err == nil {
		t.Error("Should return error for nonexistent file")
	}
}

func TestEncryptedKeyStore_CorruptedData(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatal(err)
	}
	defer ks.Close()

	// Write valid data
	testData := []byte("test-data")
	err = ks.WriteEncrypted("test.dat", testData)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the file
	filePath := filepath.Join(tempDir, "test.dat")
	data, _ := os.ReadFile(filePath)
	if len(data) > 20 {
		data[20] ^= 0xFF // Flip a byte in the ciphertext
		os.WriteFile(filePath, data, 0o600)
	}

	// Try to read corrupted data
	_, err = ks.ReadEncrypted("test.dat")
	if err == nil {
		t.Error("Should detect data corruption")
	}
}

func TestEncryptedKeyStore_DeleteEncrypted(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatal(err)
	}
	defer ks.Close()

	// Write data
	testData := []byte("test-data")
	err = ks.WriteEncrypted("test.dat", testData)
	if err != nil {
		t.Fatal(err)
	}

	// Delete data
	err = ks.DeleteEncrypted("test.dat")
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	// Verify file is gone
	filePath := filepath.Join(tempDir, "test.dat")
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("File should be deleted")
	}

	// Deleting non-existent file should not error
	err = ks.DeleteEncrypted("nonexistent.dat")
	if err != nil {
		t.Errorf("Deleting nonexistent file should not error: %v", err)
	}
}

func TestEncryptedKeyStore_MultipleFiles(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatal(err)
	}
	defer ks.Close()

	// Write multiple files
	files := map[string][]byte{
		"file1.dat": []byte("data1"),
		"file2.dat": []byte("data2"),
		"file3.dat": []byte("data3"),
	}

	for filename, data := range files {
		err = ks.WriteEncrypted(filename, data)
		if err != nil {
			t.Fatalf("Failed to write %s: %v", filename, err)
		}
	}

	// Read and verify all files
	for filename, expectedData := range files {
		data, err := ks.ReadEncrypted(filename)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", filename, err)
		}
		if !bytes.Equal(data, expectedData) {
			t.Errorf("File %s data mismatch", filename)
		}
	}
}

func TestEncryptedKeyStore_RotateKey(t *testing.T) {
	tempDir := t.TempDir()
	oldPassword := []byte("old-password")
	newPassword := []byte("new-password")

	// Create key store with old password (make a copy since it gets wiped)
	ks, err := NewEncryptedKeyStore(tempDir, append([]byte(nil), oldPassword...))
	if err != nil {
		t.Fatal(err)
	}

	// Write some data
	testData := []byte("important-data")
	err = ks.WriteEncrypted("test.dat", testData)
	if err != nil {
		t.Fatal(err)
	}

	// Rotate to new password (make a copy since it gets wiped)
	err = ks.RotateKey(append([]byte(nil), newPassword...))
	if err != nil {
		t.Fatalf("Failed to rotate key: %v", err)
	}

	// Verify data can still be read with the rotated key
	decrypted, err := ks.ReadEncrypted("test.dat")
	if err != nil {
		t.Fatalf("Failed to read after rotation: %v", err)
	}
	if !bytes.Equal(testData, decrypted) {
		t.Error("Data mismatch after key rotation")
	}

	ks.Close()

	// Verify old password no longer works (make a copy)
	ksOld, err := NewEncryptedKeyStore(tempDir, append([]byte(nil), oldPassword...))
	if err != nil {
		t.Fatal(err)
	}
	defer ksOld.Close()
	_, err = ksOld.ReadEncrypted("test.dat")
	if err == nil {
		t.Error("Old password should not work after rotation")
	}

	// Verify new password works (make a copy)
	ksNew, err := NewEncryptedKeyStore(tempDir, append([]byte(nil), newPassword...))
	if err != nil {
		t.Fatal(err)
	}
	defer ksNew.Close()
	decrypted, err = ksNew.ReadEncrypted("test.dat")
	if err != nil {
		t.Fatalf("New password should work: %v", err)
	}
	if !bytes.Equal(testData, decrypted) {
		t.Error("Data mismatch with new password")
	}
}

func TestEncryptedKeyStore_RotateKeyEmptyPassword(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatal(err)
	}
	defer ks.Close()

	err = ks.RotateKey([]byte(""))
	if err == nil {
		t.Error("Should reject empty password for rotation")
	}
}

func TestEncryptedKeyStore_Close(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatal(err)
	}

	// Key should be non-zero before close
	keyBefore := ks.encryptionKey
	hasNonZero := false
	for _, b := range keyBefore {
		if b != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("Encryption key should be non-zero before close")
	}

	// Close should wipe the key
	ks.Close()

	// Key should be all zeros after close
	for i, b := range ks.encryptionKey {
		if b != 0 {
			t.Errorf("Encryption key byte %d not zeroed after close: %x", i, b)
		}
	}
}

func TestEncryptedKeyStore_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	password := []byte("test-password")

	ks, err := NewEncryptedKeyStore(tempDir, password)
	if err != nil {
		t.Fatal(err)
	}
	defer ks.Close()

	// Write data
	testData := []byte("test-data")
	err = ks.WriteEncrypted("test.dat", testData)
	if err != nil {
		t.Fatal(err)
	}

	// Verify no temporary file left behind
	tmpFile := filepath.Join(tempDir, "test.dat.tmp")
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("Temporary file should be cleaned up")
	}

	// Verify final file exists
	finalFile := filepath.Join(tempDir, "test.dat")
	if _, err := os.Stat(finalFile); os.IsNotExist(err) {
		t.Error("Final file should exist")
	}
}
