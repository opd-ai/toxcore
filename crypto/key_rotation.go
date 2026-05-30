package crypto

import (
	"errors"
	"sync"
	"time"
)

// KeyRotationConfig holds the configuration settings for key rotation operations.
// It controls the rotation frequency, key retention policy, and automation settings.
//
// Example configuration:
//
//	config := KeyRotationConfig{
//	    RotationPeriod:  30 * 24 * time.Hour, // Rotate every 30 days
//	    MaxPreviousKeys: 3,                    // Keep last 3 keys for backward compatibility
//	    Enabled:         true,                 // Enable rotation
//	    AutoRotate:      true,                 // Rotate automatically when period expires
//	}
type KeyRotationConfig struct {
	RotationPeriod  time.Duration `json:"rotation_period"`   // How often keys should be rotated
	MaxPreviousKeys int           `json:"max_previous_keys"` // Maximum number of previous keys to keep
	Enabled         bool          `json:"enabled"`           // Whether key rotation is enabled
	AutoRotate      bool          `json:"auto_rotate"`       // Whether to automatically rotate keys
}

// KeyRotationManager handles the rotation of long-term identity keys
// to provide improved forward secrecy and mitigate the impact of key compromise
type KeyRotationManager struct {
	mu              sync.RWMutex  // Protects concurrent access to all fields
	currentKeyPair  *KeyPair      // Current active identity keypair (unexported to enforce mutex access)
	previousKeys    []*KeyPair    // Previous identity keys, kept for message backward compatibility (unexported to enforce mutex access)
	KeyCreationTime time.Time     // When the current key was created
	RotationPeriod  time.Duration // How often keys should be rotated
	MaxPreviousKeys int           // Maximum number of previous keys to keep
	timeProvider    TimeProvider  // Time provider for deterministic testing
}

// NewKeyRotationManager creates a new key rotation manager with the provided keypair
// The rotation period defaults to 30 days, and the manager keeps up to 3 previous keys
func NewKeyRotationManager(initialKeyPair *KeyPair) *KeyRotationManager {
	return NewKeyRotationManagerWithTimeProvider(initialKeyPair, nil)
}

// NewKeyRotationManagerWithTimeProvider creates a new key rotation manager with a custom TimeProvider.
// Pass nil for timeProvider to use the default time provider.
func NewKeyRotationManagerWithTimeProvider(initialKeyPair *KeyPair, timeProvider TimeProvider) *KeyRotationManager {
	if timeProvider == nil {
		timeProvider = DefaultTimeProvider{}
	}
	return &KeyRotationManager{
		currentKeyPair:  initialKeyPair,
		previousKeys:    make([]*KeyPair, 0),
		KeyCreationTime: timeProvider.Now(),
		RotationPeriod:  7 * 24 * time.Hour, // Default: rotate every 7 days (matches Signal's signed pre-key cadence)
		MaxPreviousKeys: 3,                  // Keep last 3 keys by default
		timeProvider:    timeProvider,
	}
}

// RotateKey generates a new identity key and moves the current key to the previous keys list
// This should be called periodically or when there's suspicion of key compromise
func (krm *KeyRotationManager) RotateKey() (*KeyPair, error) {
	krm.mu.Lock()
	defer krm.mu.Unlock()

	// Generate a new key pair
	newKeyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	// Move current key to previous keys list
	if krm.currentKeyPair != nil {
		krm.previousKeys = append([]*KeyPair{krm.currentKeyPair}, krm.previousKeys...)

		// Trim the list if we have too many keys
		if len(krm.previousKeys) > krm.MaxPreviousKeys {
			// Securely wipe the oldest key before removing it
			oldKey := krm.previousKeys[len(krm.previousKeys)-1]
			err := WipeKeyPair(oldKey)
			if err != nil {
				return nil, err
			}
			krm.previousKeys = krm.previousKeys[:len(krm.previousKeys)-1]
		}
	}

	// Set the new key as current
	krm.currentKeyPair = newKeyPair
	krm.KeyCreationTime = krm.getTimeProvider().Now()

	return newKeyPair, nil
}

// ShouldRotate checks if the key is due for rotation based on the rotation period
func (krm *KeyRotationManager) ShouldRotate() bool {
	krm.mu.RLock()
	defer krm.mu.RUnlock()
	return krm.getTimeProvider().Since(krm.KeyCreationTime) >= krm.RotationPeriod
}

// GetAllActiveKeys returns deep copies of all currently active keys (current + previous).
// Deep copies prevent data races when concurrent RotateKey() zeroes private key bytes.
func (krm *KeyRotationManager) GetAllActiveKeys() []*KeyPair {
	krm.mu.RLock()
	defer krm.mu.RUnlock()

	keys := make([]*KeyPair, 0, len(krm.previousKeys)+1)
	if krm.currentKeyPair != nil {
		cp := *krm.currentKeyPair
		keys = append(keys, &cp)
	}
	for _, kp := range krm.previousKeys {
		if kp != nil {
			cp := *kp
			keys = append(keys, &cp)
		}
	}
	return keys
}

// FindKeyForPublicKey finds a keypair that matches the given public key
// Returns a copy of the keypair to prevent data races with concurrent RotateKey() calls.
// Returns nil if no matching key is found.
func (krm *KeyRotationManager) FindKeyForPublicKey(publicKey [32]byte) *KeyPair {
	krm.mu.RLock()
	defer krm.mu.RUnlock()

	// Check current key first
	if krm.currentKeyPair != nil && ConstantTimeEqual32(krm.currentKeyPair.Public, publicKey) {
		// Return a deep copy to prevent data races when concurrent RotateKey() zeroes private key bytes
		cp := *krm.currentKeyPair
		return &cp
	}

	// Check previous keys
	for _, key := range krm.previousKeys {
		if ConstantTimeEqual32(key.Public, publicKey) {
			// Return a deep copy to prevent data races when concurrent RotateKey() zeroes private key bytes
			cp := *key
			return &cp
		}
	}

	return nil
}

// SetRotationPeriod updates the key rotation period
// The period must be at least 1 day to prevent excessive key rotation
func (krm *KeyRotationManager) SetRotationPeriod(period time.Duration) error {
	if period < 24*time.Hour {
		return errors.New("rotation period must be at least 1 day")
	}
	krm.mu.Lock()
	defer krm.mu.Unlock()
	krm.RotationPeriod = period
	return nil
}

// EmergencyRotation immediately rotates the identity key regardless of the
// scheduled rotation period.  Call this whenever key compromise is suspected
// (e.g., in response to a user tapping "Reset Identity" in the application UI,
// or when a security audit reveals that private key material may have been
// exposed).
//
// After calling EmergencyRotation, all active Noise sessions established with
// the previous key should be torn down and re-initiated with the new key so
// that peers authenticate the fresh identity.
//
//export ToxEmergencyRotation
func (krm *KeyRotationManager) EmergencyRotation() (*KeyPair, error) {
	return krm.RotateKey()
}

// Cleanup securely wipes all key material before destroying the manager
func (krm *KeyRotationManager) Cleanup() error {
	krm.mu.Lock()
	defer krm.mu.Unlock()

	var lastErr error

	// Wipe current key
	if krm.currentKeyPair != nil {
		if err := WipeKeyPair(krm.currentKeyPair); err != nil {
			lastErr = err
		}
		krm.currentKeyPair = nil
	}

	// Wipe all previous keys
	for i, key := range krm.previousKeys {
		if err := WipeKeyPair(key); err != nil {
			lastErr = err
		}
		krm.previousKeys[i] = nil
	}
	krm.previousKeys = nil

	return lastErr
}

// GetConfig returns the current key rotation configuration
func (krm *KeyRotationManager) GetConfig() *KeyRotationConfig {
	if krm == nil {
		return nil
	}

	krm.mu.RLock()
	defer krm.mu.RUnlock()

	return &KeyRotationConfig{
		RotationPeriod:  krm.RotationPeriod,
		MaxPreviousKeys: krm.MaxPreviousKeys,
		Enabled:         true, // If the manager exists, rotation is enabled
		AutoRotate:      true, // Assume auto-rotation is enabled by default
	}
}

// SetTimeProvider sets the time provider for deterministic testing.
// Pass nil to reset to the default time provider.
func (krm *KeyRotationManager) SetTimeProvider(tp TimeProvider) {
	krm.mu.Lock()
	defer krm.mu.Unlock()
	if tp == nil {
		tp = DefaultTimeProvider{}
	}
	krm.timeProvider = tp
}

// getTimeProvider returns the time provider, defaulting to DefaultTimeProvider if not set.
func (krm *KeyRotationManager) getTimeProvider() TimeProvider {
	if krm.timeProvider == nil {
		return DefaultTimeProvider{}
	}
	return krm.timeProvider
}

// GetCurrentKeyPair returns a copy of the current key pair.
// Returns nil if no key pair is set.
func (krm *KeyRotationManager) GetCurrentKeyPair() *KeyPair {
	krm.mu.RLock()
	defer krm.mu.RUnlock()
	if krm.currentKeyPair == nil {
		return nil
	}
	cp := *krm.currentKeyPair
	return &cp
}

// GetPreviousKeys returns copies of all previous keys.
// Returns nil if no previous keys exist.
func (krm *KeyRotationManager) GetPreviousKeys() []*KeyPair {
	krm.mu.RLock()
	defer krm.mu.RUnlock()
	if len(krm.previousKeys) == 0 {
		return nil
	}
	keys := make([]*KeyPair, 0, len(krm.previousKeys))
	for _, kp := range krm.previousKeys {
		if kp != nil {
			cp := *kp
			keys = append(keys, &cp)
		}
	}
	return keys
}
