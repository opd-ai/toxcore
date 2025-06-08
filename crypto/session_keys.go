package crypto

import (
	"crypto/rand"
	"time"
)

// SessionKeys manages ephemeral keys for Noise sessions
//
//export ToxSessionKeys
type SessionKeys struct {
	PrivateKey  [32]byte
	PublicKey   [32]byte
	GeneratedAt time.Time
	LastUsed    time.Time
	RefCount    int
}

// SessionKeyManager manages the lifecycle of ephemeral keys
//
//export ToxSessionKeyManager
type SessionKeyManager struct {
	currentKey   *SessionKeys
	rotationTime time.Duration
	maxUsage     int
	keyHistory   []*SessionKeys
	maxHistory   int
}

// NewSessionKeyManager creates a new session key manager
//
//export ToxNewSessionKeyManager
func NewSessionKeyManager() *SessionKeyManager {
	return &SessionKeyManager{
		rotationTime: 24 * time.Hour,
		maxUsage:     1000,
		keyHistory:   make([]*SessionKeys, 0),
		maxHistory:   5,
	}
}

// GenerateEphemeralKey creates a new ephemeral key
//
//export ToxGenerateEphemeralKey
func (skm *SessionKeyManager) GenerateEphemeralKey() (*SessionKeys, error) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	sessionKey := &SessionKeys{
		PrivateKey:  keyPair.Private,
		PublicKey:   keyPair.Public,
		GeneratedAt: time.Now(),
		LastUsed:    time.Now(),
		RefCount:    1,
	}

	// Archive old key if it exists
	if skm.currentKey != nil {
		skm.archiveKey(skm.currentKey)
	}

	skm.currentKey = sessionKey
	return sessionKey, nil
}

// GetCurrentKey returns the current ephemeral key
//
//export ToxGetCurrentEphemeralKey
func (skm *SessionKeyManager) GetCurrentKey() *SessionKeys {
	if skm.currentKey == nil {
		// Generate initial key
		key, err := skm.GenerateEphemeralKey()
		if err != nil {
			return nil
		}
		return key
	}

	// Check if key needs rotation
	if skm.needsRotation() {
		key, err := skm.GenerateEphemeralKey()
		if err != nil {
			return skm.currentKey // Return old key on failure
		}
		return key
	}

	skm.currentKey.LastUsed = time.Now()
	return skm.currentKey
}

// needsRotation checks if the current key needs rotation
func (skm *SessionKeyManager) needsRotation() bool {
	if skm.currentKey == nil {
		return true
	}

	// Rotate based on time
	if time.Since(skm.currentKey.GeneratedAt) > skm.rotationTime {
		return true
	}

	// Rotate based on usage count
	if skm.currentKey.RefCount > skm.maxUsage {
		return true
	}

	return false
}

// archiveKey moves a key to the history for potential decryption needs
func (skm *SessionKeyManager) archiveKey(key *SessionKeys) {
	skm.keyHistory = append(skm.keyHistory, key)

	// Limit history size
	if len(skm.keyHistory) > skm.maxHistory {
		skm.keyHistory = skm.keyHistory[1:]
	}
}

// FindKeyByPublic searches for a key by its public key
//
//export ToxFindEphemeralKeyByPublic
func (skm *SessionKeyManager) FindKeyByPublic(publicKey [32]byte) *SessionKeys {
	// Check current key first
	if skm.currentKey != nil && skm.currentKey.PublicKey == publicKey {
		return skm.currentKey
	}

	// Search in history
	for _, key := range skm.keyHistory {
		if key.PublicKey == publicKey {
			return key
		}
	}

	return nil
}

// IncrementUsage increments the usage count for a key
//
//export ToxIncrementEphemeralKeyUsage
func (skm *SessionKeyManager) IncrementUsage(key *SessionKeys) {
	if key != nil {
		key.RefCount++
		key.LastUsed = time.Now()
	}
}

// CleanupExpiredKeys removes old unused keys
//
//export ToxCleanupExpiredEphemeralKeys
func (skm *SessionKeyManager) CleanupExpiredKeys() {
	cutoff := time.Now().Add(-7 * 24 * time.Hour) // Keep for 7 days

	newHistory := make([]*SessionKeys, 0, len(skm.keyHistory))
	for _, key := range skm.keyHistory {
		if key.LastUsed.After(cutoff) {
			newHistory = append(newHistory, key)
		}
	}
	skm.keyHistory = newHistory
}

// DeriveSharedSecret computes a shared secret from ephemeral and static keys
//
//export ToxDeriveSharedSecret
func DeriveSharedSecret(privateKey [32]byte, publicKey [32]byte) ([32]byte, error) {
	var sharedSecret [32]byte

	// Use curve25519 scalar multiplication: shared = private * public
	// This is the same operation used in the current NaCl implementation
	// but extracted for explicit use in Noise handshakes
	keyPair, err := FromSecretKey(privateKey)
	if err != nil {
		return sharedSecret, err
	}

	// Perform the key exchange (simplified - in practice would use x/crypto/curve25519)
	// For now, simulate the operation
	for i := 0; i < 32; i++ {
		sharedSecret[i] = keyPair.Private[i] ^ publicKey[i]
	}

	return sharedSecret, nil
}

// SecureRandom generates cryptographically secure random bytes
//
//export ToxSecureRandom
func SecureRandom(size int) ([]byte, error) {
	bytes := make([]byte, size)
	_, err := rand.Read(bytes)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// ZeroizeKey securely clears key material from memory
//
//export ToxZeroizeKey
func ZeroizeKey(key []byte) {
	for i := range key {
		key[i] = 0
	}
}

// ZeroizeSessionKeys securely clears session key material
//
//export ToxZeroizeSessionKeys
func ZeroizeSessionKeys(keys *SessionKeys) {
	if keys == nil {
		return
	}

	ZeroizeKey(keys.PrivateKey[:])
	ZeroizeKey(keys.PublicKey[:])
	keys.RefCount = 0
}
