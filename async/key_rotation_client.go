package async

import (
	"errors"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// ErrKeyRotationNotConfigured is returned when key rotation operations are attempted
// on a client that was not configured with key rotation support
var ErrKeyRotationNotConfigured = errors.New("key rotation is not configured for this client")

// NewClientWithKeyRotation creates a new async client with key rotation support
func NewClientWithKeyRotation(keyPair *crypto.KeyPair, transport transport.Transport,
	rotationPeriod time.Duration,
) (*AsyncClient, error) {
	// Create the base client
	client := NewAsyncClient(keyPair, transport)

	// Create and configure the key rotation manager
	rotationManager := crypto.NewKeyRotationManager(keyPair)
	if rotationPeriod > 0 {
		if err := rotationManager.SetRotationPeriod(rotationPeriod); err != nil {
			return nil, err
		}
	}

	// Add the rotation manager to the client
	client.keyRotation = rotationManager

	// Start a background goroutine to check for key rotation
	go client.startKeyRotationChecker()

	return client, nil
}

// startKeyRotationChecker runs a background process to check if keys need rotation
func (ac *AsyncClient) startKeyRotationChecker() {
	if ac.keyRotation == nil {
		return
	}

	ticker := time.NewTicker(24 * time.Hour) // Check once per day
	defer ticker.Stop()

	for range ticker.C {
		ac.checkAndRotateKeys()
	}
}

// checkAndRotateKeys checks if key rotation is needed and performs it if necessary
func (ac *AsyncClient) checkAndRotateKeys() {
	if ac.keyRotation == nil {
		return
	}

	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	if ac.keyRotation.ShouldRotate() {
		// Rotate the key
		newKeyPair, err := ac.keyRotation.RotateKey()
		if err != nil {
			// Log error but continue - better to use old key than fail
			return
		}

		// Update the client's active key pair
		ac.keyPair = newKeyPair

		// You might want to notify the application about the key rotation
		// through a callback or channel
	}
}

// GetAllActiveIdentities returns all identity keys (current and previous)
func (ac *AsyncClient) GetAllActiveIdentities() []*crypto.KeyPair {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	if ac.keyRotation == nil {
		// Return just the current key if no rotation manager exists
		return []*crypto.KeyPair{ac.keyPair}
	}

	return ac.keyRotation.GetAllActiveKeys()
}

// EmergencyRotateIdentity immediately rotates the identity key
// This should be used when key compromise is suspected
func (ac *AsyncClient) EmergencyRotateIdentity() error {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	if ac.keyRotation == nil {
		return ErrKeyRotationNotConfigured
	}

	newKey, err := ac.keyRotation.EmergencyRotation()
	if err != nil {
		return err
	}

	ac.keyPair = newKey
	return nil
}

// GetKeyRotationConfig returns the current key rotation configuration
// Returns nil if key rotation is not configured for this client
func (ac *AsyncClient) GetKeyRotationConfig() *crypto.KeyRotationConfig {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	if ac.keyRotation == nil {
		return nil // No key rotation configured - this is expected behavior
	}

	return ac.keyRotation.GetConfig()
}

// IsKeyRotationEnabled returns true if key rotation is configured for this client
func (ac *AsyncClient) IsKeyRotationEnabled() bool {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.keyRotation != nil
}
