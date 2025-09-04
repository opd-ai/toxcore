package crypto

import (
	"errors"
	"time"
)

// KeyRotationConfig represents the configuration for key rotation
type KeyRotationConfig struct {
	RotationPeriod  time.Duration `json:"rotation_period"`   // How often keys should be rotated
	MaxPreviousKeys int           `json:"max_previous_keys"` // Maximum number of previous keys to keep
	Enabled         bool          `json:"enabled"`           // Whether key rotation is enabled
	AutoRotate      bool          `json:"auto_rotate"`       // Whether to automatically rotate keys
}

// KeyRotationManager handles the rotation of long-term identity keys
// to provide improved forward secrecy and mitigate the impact of key compromise
type KeyRotationManager struct {
	CurrentKeyPair  *KeyPair      // Current active identity keypair
	PreviousKeys    []*KeyPair    // Previous identity keys, kept for message backward compatibility
	KeyCreationTime time.Time     // When the current key was created
	RotationPeriod  time.Duration // How often keys should be rotated
	MaxPreviousKeys int           // Maximum number of previous keys to keep
}

// NewKeyRotationManager creates a new key rotation manager with the provided keypair
// The rotation period defaults to 30 days, and the manager keeps up to 3 previous keys
func NewKeyRotationManager(initialKeyPair *KeyPair) *KeyRotationManager {
	return &KeyRotationManager{
		CurrentKeyPair:  initialKeyPair,
		PreviousKeys:    make([]*KeyPair, 0),
		KeyCreationTime: time.Now(),
		RotationPeriod:  30 * 24 * time.Hour, // Default: rotate every 30 days
		MaxPreviousKeys: 3,                   // Keep last 3 keys by default
	}
}

// RotateKey generates a new identity key and moves the current key to the previous keys list
// This should be called periodically or when there's suspicion of key compromise
func (krm *KeyRotationManager) RotateKey() (*KeyPair, error) {
	// Generate a new key pair
	newKeyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	// Move current key to previous keys list
	if krm.CurrentKeyPair != nil {
		krm.PreviousKeys = append([]*KeyPair{krm.CurrentKeyPair}, krm.PreviousKeys...)

		// Trim the list if we have too many keys
		if len(krm.PreviousKeys) > krm.MaxPreviousKeys {
			// Securely wipe the oldest key before removing it
			oldKey := krm.PreviousKeys[len(krm.PreviousKeys)-1]
			err := WipeKeyPair(oldKey)
			if err != nil {
				return nil, err
			}
			krm.PreviousKeys = krm.PreviousKeys[:len(krm.PreviousKeys)-1]
		}
	}

	// Set the new key as current
	krm.CurrentKeyPair = newKeyPair
	krm.KeyCreationTime = time.Now()

	return newKeyPair, nil
}

// ShouldRotate checks if the key is due for rotation based on the rotation period
func (krm *KeyRotationManager) ShouldRotate() bool {
	return time.Since(krm.KeyCreationTime) >= krm.RotationPeriod
}

// GetAllActiveKeys returns all currently active keys (current + previous)
// This can be used to check incoming messages against all possible identities
func (krm *KeyRotationManager) GetAllActiveKeys() []*KeyPair {
	keys := make([]*KeyPair, 0, len(krm.PreviousKeys)+1)
	keys = append(keys, krm.CurrentKeyPair)
	keys = append(keys, krm.PreviousKeys...)
	return keys
}

// FindKeyForPublicKey finds a keypair that matches the given public key
// Returns nil if no matching key is found
func (krm *KeyRotationManager) FindKeyForPublicKey(publicKey [32]byte) *KeyPair {
	// Check current key first
	if krm.CurrentKeyPair != nil && krm.CurrentKeyPair.Public == publicKey {
		return krm.CurrentKeyPair
	}

	// Check previous keys
	for _, key := range krm.PreviousKeys {
		if key.Public == publicKey {
			return key
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
	krm.RotationPeriod = period
	return nil
}

// EmergencyRotation immediately rotates the key regardless of rotation schedule
// This should be used when there's suspicion of key compromise
func (krm *KeyRotationManager) EmergencyRotation() (*KeyPair, error) {
	return krm.RotateKey()
}

// Cleanup securely wipes all key material before destroying the manager
func (krm *KeyRotationManager) Cleanup() error {
	var lastErr error

	// Wipe current key
	if krm.CurrentKeyPair != nil {
		if err := WipeKeyPair(krm.CurrentKeyPair); err != nil {
			lastErr = err
		}
		krm.CurrentKeyPair = nil
	}

	// Wipe all previous keys
	for i, key := range krm.PreviousKeys {
		if err := WipeKeyPair(key); err != nil {
			lastErr = err
		}
		krm.PreviousKeys[i] = nil
	}
	krm.PreviousKeys = nil

	return lastErr
}

// GetConfig returns the current key rotation configuration
func (krm *KeyRotationManager) GetConfig() *KeyRotationConfig {
	if krm == nil {
		return nil
	}

	return &KeyRotationConfig{
		RotationPeriod:  krm.RotationPeriod,
		MaxPreviousKeys: krm.MaxPreviousKeys,
		Enabled:         true, // If the manager exists, rotation is enabled
		AutoRotate:      true, // Assume auto-rotation is enabled by default
	}
}
