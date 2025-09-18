package crypto

import (
	"testing"
	"time"
)

// TestNewKeyRotationManager tests the creation of a new key rotation manager
func TestNewKeyRotationManager(t *testing.T) {
	t.Parallel()

	// Generate a test key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	// Create a new key rotation manager
	krm := NewKeyRotationManager(keyPair)

	// Verify the manager was initialized correctly
	if krm == nil {
		t.Fatal("NewKeyRotationManager() returned nil")
	}

	if krm.CurrentKeyPair != keyPair {
		t.Error("Current key pair not set correctly")
	}

	if len(krm.PreviousKeys) != 0 {
		t.Errorf("Expected empty previous keys list, got %d keys", len(krm.PreviousKeys))
	}

	if krm.RotationPeriod != 30*24*time.Hour {
		t.Errorf("Expected default rotation period of 30 days, got %v", krm.RotationPeriod)
	}

	if krm.MaxPreviousKeys != 3 {
		t.Errorf("Expected default max previous keys of 3, got %d", krm.MaxPreviousKeys)
	}

	// Verify key creation time is recent (within last minute)
	if time.Since(krm.KeyCreationTime) > time.Minute {
		t.Error("Key creation time is too old")
	}
}

// TestKeyRotationManager_RotateKey tests the key rotation functionality
func TestKeyRotationManager_RotateKey(t *testing.T) {
	t.Parallel()

	// Generate initial key pair
	initialKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(initialKey)
	originalCreationTime := krm.KeyCreationTime

	// Sleep briefly to ensure time difference
	time.Sleep(time.Millisecond)

	// Rotate the key
	newKey, err := krm.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey() failed: %v", err)
	}

	// Verify new key was generated
	if newKey == nil {
		t.Fatal("RotateKey() returned nil key")
	}

	// Verify current key was updated
	if krm.CurrentKeyPair != newKey {
		t.Error("Current key pair not updated after rotation")
	}

	// Verify key creation time was updated
	if !krm.KeyCreationTime.After(originalCreationTime) {
		t.Error("Key creation time not updated after rotation")
	}

	// Verify previous key was stored
	if len(krm.PreviousKeys) != 1 {
		t.Errorf("Expected 1 previous key, got %d", len(krm.PreviousKeys))
	}

	if krm.PreviousKeys[0] != initialKey {
		t.Error("Previous key not stored correctly")
	}

	// Verify keys are different
	if krm.CurrentKeyPair.Public == initialKey.Public {
		t.Error("New key has same public key as previous key")
	}
}

// TestKeyRotationManager_MaxPreviousKeys tests the previous keys limit functionality
func TestKeyRotationManager_MaxPreviousKeys(t *testing.T) {
	t.Parallel()

	// Generate initial key pair
	initialKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(initialKey)
	krm.MaxPreviousKeys = 2 // Set limit to 2 for testing

	var rotatedKeys []*KeyPair

	// Rotate keys multiple times to exceed the limit
	for i := 0; i < 4; i++ {
		oldKey := krm.CurrentKeyPair
		newKey, err := krm.RotateKey()
		if err != nil {
			t.Fatalf("RotateKey() failed on iteration %d: %v", i, err)
		}
		rotatedKeys = append(rotatedKeys, newKey)

		// Verify that we never exceed MaxPreviousKeys
		if len(krm.PreviousKeys) > krm.MaxPreviousKeys {
			t.Errorf("Previous keys list exceeded limit: got %d, max %d",
				len(krm.PreviousKeys), krm.MaxPreviousKeys)
		}

		// After the first rotation, we should have some previous keys
		if i == 0 && len(krm.PreviousKeys) != 1 {
			t.Errorf("Expected 1 previous key after first rotation, got %d", len(krm.PreviousKeys))
		}

		// After exceeding the limit, we should have exactly MaxPreviousKeys
		if i >= krm.MaxPreviousKeys && len(krm.PreviousKeys) != krm.MaxPreviousKeys {
			t.Errorf("Expected exactly %d previous keys, got %d",
				krm.MaxPreviousKeys, len(krm.PreviousKeys))
		}

		// Verify the oldest key is not the initial key (should have been wiped)
		if i >= krm.MaxPreviousKeys {
			for _, prevKey := range krm.PreviousKeys {
				if prevKey == oldKey {
					continue // This is expected
				}
				if prevKey == initialKey {
					t.Error("Initial key still in previous keys list after exceeding limit")
				}
			}
		}
	}
}

// TestKeyRotationManager_ShouldRotate tests the rotation timing logic
func TestKeyRotationManager_ShouldRotate(t *testing.T) {
	t.Parallel()

	// Generate initial key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(keyPair)

	// Set a short rotation period for testing
	krm.RotationPeriod = 100 * time.Millisecond

	// Initially should not need rotation
	if krm.ShouldRotate() {
		t.Error("New key should not need rotation immediately")
	}

	// Wait for rotation period
	time.Sleep(150 * time.Millisecond)

	// Now should need rotation
	if !krm.ShouldRotate() {
		t.Error("Key should need rotation after rotation period")
	}

	// After rotation, should not need rotation again
	_, err = krm.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey() failed: %v", err)
	}

	if krm.ShouldRotate() {
		t.Error("Key should not need rotation immediately after rotation")
	}
}

// TestKeyRotationManager_GetAllActiveKeys tests retrieval of all active keys
func TestKeyRotationManager_GetAllActiveKeys(t *testing.T) {
	t.Parallel()

	// Generate initial key pair
	initialKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(initialKey)

	// Initially should only have one key
	keys := krm.GetAllActiveKeys()
	if len(keys) != 1 {
		t.Errorf("Expected 1 active key initially, got %d", len(keys))
	}
	if keys[0] != initialKey {
		t.Error("Initial key not returned correctly")
	}

	// Rotate a few keys
	var allKeys []*KeyPair
	allKeys = append(allKeys, initialKey)

	for i := 0; i < 3; i++ {
		newKey, err := krm.RotateKey()
		if err != nil {
			t.Fatalf("RotateKey() failed: %v", err)
		}
		allKeys = append(allKeys, newKey)

		keys = krm.GetAllActiveKeys()
		expectedCount := min(i+2, krm.MaxPreviousKeys+1) // Current + previous (up to limit)
		if len(keys) != expectedCount {
			t.Errorf("Expected %d active keys, got %d", expectedCount, len(keys))
		}

		// Current key should always be first
		if keys[0] != newKey {
			t.Error("Current key not first in active keys list")
		}
	}
}

// TestKeyRotationManager_FindKeyForPublicKey tests finding keys by public key
func TestKeyRotationManager_FindKeyForPublicKey(t *testing.T) {
	t.Parallel()

	// Generate initial key pair
	initialKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(initialKey)

	// Should find the initial key
	foundKey := krm.FindKeyForPublicKey(initialKey.Public)
	if foundKey != initialKey {
		t.Error("Failed to find initial key by public key")
	}

	// Rotate the key
	newKey, err := krm.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey() failed: %v", err)
	}

	// Should find the new current key
	foundKey = krm.FindKeyForPublicKey(newKey.Public)
	if foundKey != newKey {
		t.Error("Failed to find new current key by public key")
	}

	// Should still find the previous key
	foundKey = krm.FindKeyForPublicKey(initialKey.Public)
	if foundKey != initialKey {
		t.Error("Failed to find previous key by public key")
	}

	// Should not find a random key
	randomKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate random key pair: %v", err)
	}

	foundKey = krm.FindKeyForPublicKey(randomKey.Public)
	if foundKey != nil {
		t.Error("Found key that should not exist in manager")
	}
}

// TestKeyRotationManager_SetRotationPeriod tests setting rotation period
func TestKeyRotationManager_SetRotationPeriod(t *testing.T) {
	t.Parallel()

	// Generate initial key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(keyPair)

	// Test valid periods
	validPeriods := []time.Duration{
		24 * time.Hour,       // Minimum: 1 day
		7 * 24 * time.Hour,   // 1 week
		30 * 24 * time.Hour,  // 30 days
		365 * 24 * time.Hour, // 1 year
	}

	for _, period := range validPeriods {
		err := krm.SetRotationPeriod(period)
		if err != nil {
			t.Errorf("SetRotationPeriod(%v) failed: %v", period, err)
		}
		if krm.RotationPeriod != period {
			t.Errorf("Rotation period not set correctly: expected %v, got %v",
				period, krm.RotationPeriod)
		}
	}

	// Test invalid periods (less than 1 day)
	invalidPeriods := []time.Duration{
		0,
		time.Hour,
		23 * time.Hour,
		23*time.Hour + 59*time.Minute + 59*time.Second,
	}

	for _, period := range invalidPeriods {
		err := krm.SetRotationPeriod(period)
		if err == nil {
			t.Errorf("SetRotationPeriod(%v) should have failed but didn't", period)
		}
	}
}

// TestKeyRotationManager_EmergencyRotation tests emergency rotation
func TestKeyRotationManager_EmergencyRotation(t *testing.T) {
	t.Parallel()

	// Generate initial key pair
	initialKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(initialKey)

	// Set a very long rotation period
	krm.RotationPeriod = 365 * 24 * time.Hour

	// Should not normally need rotation
	if krm.ShouldRotate() {
		t.Error("Key should not need normal rotation with long period")
	}

	// Emergency rotation should work anyway
	newKey, err := krm.EmergencyRotation()
	if err != nil {
		t.Fatalf("EmergencyRotation() failed: %v", err)
	}

	// Verify rotation occurred
	if newKey == nil {
		t.Fatal("EmergencyRotation() returned nil key")
	}

	if krm.CurrentKeyPair != newKey {
		t.Error("Current key not updated after emergency rotation")
	}

	if len(krm.PreviousKeys) != 1 || krm.PreviousKeys[0] != initialKey {
		t.Error("Previous key not stored correctly after emergency rotation")
	}
}

// TestKeyRotationManager_Cleanup tests secure cleanup of all keys
func TestKeyRotationManager_Cleanup(t *testing.T) {
	t.Parallel()

	// Generate initial key pair
	initialKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(initialKey)

	// Rotate a few keys to have multiple keys to clean up
	for i := 0; i < 3; i++ {
		_, err := krm.RotateKey()
		if err != nil {
			t.Fatalf("RotateKey() failed: %v", err)
		}
	}

	// Verify we have keys before cleanup
	if krm.CurrentKeyPair == nil {
		t.Fatal("No current key before cleanup")
	}
	if len(krm.PreviousKeys) == 0 {
		t.Fatal("No previous keys before cleanup")
	}

	// Perform cleanup
	err = krm.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}

	// Verify all keys were cleared
	if krm.CurrentKeyPair != nil {
		t.Error("Current key not cleared after cleanup")
	}

	if krm.PreviousKeys != nil {
		t.Error("Previous keys list not cleared after cleanup")
	}
}

// TestKeyRotationManager_GetConfig tests configuration retrieval
func TestKeyRotationManager_GetConfig(t *testing.T) {
	t.Parallel()

	// Test with nil manager
	var krm *KeyRotationManager
	config := krm.GetConfig()
	if config != nil {
		t.Error("GetConfig() should return nil for nil manager")
	}

	// Generate initial key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm = NewKeyRotationManager(keyPair)

	// Test with valid manager
	config = krm.GetConfig()
	if config == nil {
		t.Fatal("GetConfig() returned nil for valid manager")
	}

	if config.RotationPeriod != krm.RotationPeriod {
		t.Errorf("Config rotation period mismatch: expected %v, got %v",
			krm.RotationPeriod, config.RotationPeriod)
	}

	if config.MaxPreviousKeys != krm.MaxPreviousKeys {
		t.Errorf("Config max previous keys mismatch: expected %d, got %d",
			krm.MaxPreviousKeys, config.MaxPreviousKeys)
	}

	if !config.Enabled {
		t.Error("Config should show rotation as enabled")
	}

	if !config.AutoRotate {
		t.Error("Config should show auto-rotation as enabled")
	}

	// Test with modified configuration
	err = krm.SetRotationPeriod(7 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("SetRotationPeriod() failed: %v", err)
	}
	krm.MaxPreviousKeys = 5

	config = krm.GetConfig()
	if config.RotationPeriod != 7*24*time.Hour {
		t.Error("Config not updated after rotation period change")
	}
	if config.MaxPreviousKeys != 5 {
		t.Error("Config not updated after max previous keys change")
	}
}

// TestKeyRotationConcurrency tests concurrent operations on key rotation manager
func TestKeyRotationConcurrency(t *testing.T) {
	t.Parallel()

	// Generate initial key pair
	initialKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(initialKey)

	// Run concurrent operations
	done := make(chan bool, 6)

	// Concurrent rotations
	go func() {
		for i := 0; i < 5; i++ {
			_, err := krm.RotateKey()
			if err != nil {
				t.Errorf("Concurrent RotateKey() failed: %v", err)
			}
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Concurrent config access
	go func() {
		for i := 0; i < 10; i++ {
			_ = krm.GetConfig()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Concurrent key lookups
	go func() {
		for i := 0; i < 10; i++ {
			_ = krm.GetAllActiveKeys()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Concurrent should rotate checks
	go func() {
		for i := 0; i < 10; i++ {
			_ = krm.ShouldRotate()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Concurrent key searches
	go func() {
		for i := 0; i < 10; i++ {
			_ = krm.FindKeyForPublicKey(initialKey.Public)
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Concurrent period updates
	go func() {
		periods := []time.Duration{
			24 * time.Hour,
			48 * time.Hour,
			72 * time.Hour,
		}
		for i := 0; i < 5; i++ {
			_ = krm.SetRotationPeriod(periods[i%len(periods)])
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 6; i++ {
		<-done
	}
}

// Helper function for Go versions that don't have min built-in
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BenchmarkKeyRotation benchmarks the key rotation operation
func BenchmarkKeyRotation(b *testing.B) {
	// Generate initial key pair
	initialKey, err := GenerateKeyPair()
	if err != nil {
		b.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(initialKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := krm.RotateKey()
		if err != nil {
			b.Fatalf("RotateKey() failed: %v", err)
		}
	}
}

// BenchmarkFindKeyForPublicKey benchmarks key lookup by public key
func BenchmarkFindKeyForPublicKey(b *testing.B) {
	// Generate initial key pair
	initialKey, err := GenerateKeyPair()
	if err != nil {
		b.Fatalf("Failed to generate initial key pair: %v", err)
	}

	krm := NewKeyRotationManager(initialKey)

	// Add several previous keys
	for i := 0; i < 10; i++ {
		_, err := krm.RotateKey()
		if err != nil {
			b.Fatalf("RotateKey() failed: %v", err)
		}
	}

	allKeys := krm.GetAllActiveKeys()
	if len(allKeys) == 0 {
		b.Fatal("No keys available for benchmark")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Alternate between looking for current and previous keys
		keyToFind := allKeys[i%len(allKeys)]
		found := krm.FindKeyForPublicKey(keyToFind.Public)
		if found == nil {
			b.Fatal("Key not found during benchmark")
		}
	}
}
