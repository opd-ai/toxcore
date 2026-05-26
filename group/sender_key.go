// Package group implements group chat functionality for the Tox protocol.
//
// This file implements the Sender-Key protocol for efficient O(1) group message
// encryption, inspired by Signal's sender-key distribution mechanism.
package group

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// SenderKeyState holds the state of a sender's key for a specific group.
// Each member of a group maintains their own sender key that they use
// to encrypt messages. The sender key is distributed to all other members
// encrypted with their individual public keys.
type SenderKeyState struct {
	// KeyID uniquely identifies this key generation (increments on rotation)
	KeyID uint32

	// Key is the 32-byte symmetric key used for ChaCha20-Poly1305
	Key [32]byte

	// ChainKey is used for key derivation (Double Ratchet style)
	ChainKey [32]byte

	// MessageCounter tracks messages sent with this key (for nonce derivation)
	MessageCounter uint64

	// CreatedAt is when this key was generated
	CreatedAt time.Time
}

// SenderKeyManager manages sender keys for group chat encryption.
// It provides O(1) encryption by using a symmetric key that all group
// members know, instead of encrypting separately for each recipient.
type SenderKeyManager struct {
	mu sync.RWMutex

	// groupID is the identifier of the group this manager belongs to
	groupID [32]byte

	// selfPublicKey is this member's Curve25519 public key
	selfPublicKey [32]byte

	// selfPrivateKey is this member's Curve25519 private key
	selfPrivateKey [32]byte

	// mySenderKey is this member's current sender key state
	mySenderKey *SenderKeyState

	// peerSenderKeys maps peer public keys to their sender key states
	// Used for decrypting messages from other group members
	peerSenderKeys map[[32]byte]*SenderKeyState

	// maxMessageCounter is the maximum messages before key rotation
	maxMessageCounter uint64

	// onKeyRotation is called when this member rotates their sender key
	onKeyRotation func(newKey *SenderKeyDistribution)
}

// SenderKeyDistribution represents a sender key being distributed to group members.
// This is encrypted separately for each recipient using their public key.
// Uses ephemeral DH for forward secrecy: compromise of long-term keys does not
// compromise past distributions.
type SenderKeyDistribution struct {
	// GroupID identifies the group
	GroupID [32]byte

	// SenderPublicKey is the public key of the sender
	SenderPublicKey [32]byte

	// KeyID identifies this key generation
	KeyID uint32

	// EphemeralPublicKey is a per-distribution ephemeral public key for forward secrecy
	// Combined with static ECDH to provide defense against key compromise
	EphemeralPublicKey [32]byte

	// EncryptedKey is the sender key encrypted for a specific recipient
	// Using Curve25519 ECDH + ephemeral DH + ChaCha20-Poly1305
	EncryptedKey []byte

	// Nonce used for encryption
	Nonce [24]byte
}

// SenderKeyMessage represents a message encrypted with a sender key.
type SenderKeyMessage struct {
	// GroupID identifies the group
	GroupID [32]byte

	// SenderPublicKey identifies the sender
	SenderPublicKey [32]byte

	// KeyID identifies which sender key generation was used
	KeyID uint32

	// Counter is the message counter for nonce derivation
	Counter uint64

	// Ciphertext is the encrypted message
	Ciphertext []byte
}

// NewSenderKeyManager creates a new sender key manager for a group member.
func NewSenderKeyManager(groupID, publicKey, privateKey [32]byte) (*SenderKeyManager, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewSenderKeyManager",
		"group_id": fmt.Sprintf("%x", groupID[:8]),
	}).Debug("Creating new sender key manager")

	skm := &SenderKeyManager{
		groupID:           groupID,
		selfPublicKey:     publicKey,
		selfPrivateKey:    privateKey,
		peerSenderKeys:    make(map[[32]byte]*SenderKeyState),
		maxMessageCounter: 1000, // Rotate after 1000 messages
	}

	// Generate initial sender key
	if err := skm.generateNewSenderKey(); err != nil {
		return nil, fmt.Errorf("failed to generate initial sender key: %w", err)
	}

	return skm, nil
}

// generateNewSenderKey creates a new sender key for this member.
func (skm *SenderKeyManager) generateNewSenderKey() error {
	skm.mu.Lock()
	defer skm.mu.Unlock()

	var key, chainKey [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return fmt.Errorf("failed to generate sender key: %w", err)
	}
	if _, err := rand.Read(chainKey[:]); err != nil {
		return fmt.Errorf("failed to generate chain key: %w", err)
	}

	keyID := uint32(1)
	if skm.mySenderKey != nil {
		keyID = skm.mySenderKey.KeyID + 1
	}

	skm.mySenderKey = &SenderKeyState{
		KeyID:          keyID,
		Key:            key,
		ChainKey:       chainKey,
		MessageCounter: 0,
		CreatedAt:      time.Now(),
	}

	logrus.WithFields(logrus.Fields{
		"function": "generateNewSenderKey",
		"key_id":   keyID,
	}).Debug("Generated new sender key")

	return nil
}

// RotateSenderKey generates a new sender key and returns distributions
// for all specified peer public keys. This should be called when a
// member leaves the group to maintain forward secrecy.
func (skm *SenderKeyManager) RotateSenderKey(peerPublicKeys [][32]byte) ([]*SenderKeyDistribution, error) {
	if err := skm.generateNewSenderKey(); err != nil {
		return nil, fmt.Errorf("failed to rotate sender key: %w", err)
	}

	distributions, err := skm.CreateDistributions(peerPublicKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to create key distributions: %w", err)
	}

	if skm.onKeyRotation != nil && len(distributions) > 0 {
		skm.onKeyRotation(distributions[0])
	}

	return distributions, nil
}

// CreateDistributions creates sender key distribution messages for the
// specified peer public keys. Each distribution is encrypted specifically
// for that peer using Curve25519 ECDH.
func (skm *SenderKeyManager) CreateDistributions(peerPublicKeys [][32]byte) ([]*SenderKeyDistribution, error) {
	skm.mu.RLock()
	defer skm.mu.RUnlock()

	if skm.mySenderKey == nil {
		return nil, errors.New("no sender key available")
	}

	distributions := make([]*SenderKeyDistribution, 0, len(peerPublicKeys))

	for _, peerPK := range peerPublicKeys {
		dist, err := skm.createDistributionForPeer(peerPK)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "CreateDistributions",
				"peer_pk":  fmt.Sprintf("%x", peerPK[:8]),
				"error":    err.Error(),
			}).Warn("Failed to create distribution for peer")
			continue
		}
		distributions = append(distributions, dist)
	}

	return distributions, nil
}

// combineAndDeriveKey combines two DH outputs and derives an encryption key via SHA-256.
// Both inputs are wiped on return.
func combineAndDeriveKey(dhA, dhB []byte) [32]byte {
	combined := make([]byte, 64)
	copy(combined[:32], dhA)
	copy(combined[32:], dhB)
	result := sha256.Sum256(combined)
	crypto.ZeroBytes(dhA)
	crypto.ZeroBytes(dhB)
	crypto.ZeroBytes(combined)
	return result
}

// generateEphemeralKeyPair generates an ephemeral Curve25519 key pair.
func generateEphemeralKeyPair() (private, public [32]byte, err error) {
	if _, err = rand.Read(private[:]); err != nil {
		return private, public, fmt.Errorf("failed to generate ephemeral private key: %w", err)
	}
	curve25519.ScalarBaseMult(&public, &private)
	return private, public, nil
}

// encryptSenderKeyForPeer encrypts the sender key + chain key for a peer using
// an AEAD cipher derived from the provided key material.
func (skm *SenderKeyManager) encryptSenderKeyForPeer(encKey [32]byte, groupID [32]byte) (ciphertext []byte, nonce [24]byte, err error) {
	aead, err := chacha20poly1305.NewX(encKey[:])
	if err != nil {
		return nil, nonce, fmt.Errorf("failed to create cipher: %w", err)
	}
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, nonce, fmt.Errorf("failed to generate nonce: %w", err)
	}
	plaintext := make([]byte, 64)
	copy(plaintext[:32], skm.mySenderKey.Key[:])
	copy(plaintext[32:], skm.mySenderKey.ChainKey[:])
	return aead.Seal(nil, nonce[:], plaintext, groupID[:]), nonce, nil
}

// createDistributionForPeer creates an encrypted sender key distribution
// for a specific peer using ephemeral ECDH for forward secrecy.
// The encryption key is derived from both static and ephemeral DH components.
func (skm *SenderKeyManager) createDistributionForPeer(peerPublicKey [32]byte) (*SenderKeyDistribution, error) {
	ephemeralPrivate, ephemeralPublic, err := generateEphemeralKeyPair()
	if err != nil {
		return nil, err
	}

	// Compute static DH: DH(static_private, peer_public)
	staticDH, err := curve25519.X25519(skm.selfPrivateKey[:], peerPublicKey[:])
	if err != nil {
		crypto.ZeroBytes(ephemeralPrivate[:])
		return nil, fmt.Errorf("failed to compute static DH: %w", err)
	}

	// Compute ephemeral DH: DH(ephemeral_private, peer_public)
	ephemeralDH, err := curve25519.X25519(ephemeralPrivate[:], peerPublicKey[:])
	crypto.ZeroBytes(ephemeralPrivate[:])
	if err != nil {
		return nil, fmt.Errorf("failed to compute ephemeral DH: %w", err)
	}

	// Derive encryption key (wipes staticDH and ephemeralDH).
	encKey := combineAndDeriveKey(staticDH, ephemeralDH)

	ciphertext, nonce, err := skm.encryptSenderKeyForPeer(encKey, skm.groupID)
	if err != nil {
		return nil, err
	}

	return &SenderKeyDistribution{
		GroupID:            skm.groupID,
		SenderPublicKey:    skm.selfPublicKey,
		KeyID:              skm.mySenderKey.KeyID,
		EphemeralPublicKey: ephemeralPublic,
		EncryptedKey:       ciphertext,
		Nonce:              nonce,
	}, nil
}

// decryptSenderKeyFromDist derives the decryption key from a distribution and decrypts
// the embedded sender key material.
func (skm *SenderKeyManager) decryptSenderKeyFromDist(dist *SenderKeyDistribution) (key, chainKey [32]byte, err error) {
	staticDH, err := curve25519.X25519(skm.selfPrivateKey[:], dist.SenderPublicKey[:])
	if err != nil {
		return key, chainKey, fmt.Errorf("failed to compute static DH: %w", err)
	}

	ephemeralDH, err := curve25519.X25519(skm.selfPrivateKey[:], dist.EphemeralPublicKey[:])
	if err != nil {
		return key, chainKey, fmt.Errorf("failed to compute ephemeral DH: %w", err)
	}

	// Derive decryption key (wipes staticDH and ephemeralDH).
	decKey := combineAndDeriveKey(staticDH, ephemeralDH)

	aead, err := chacha20poly1305.NewX(decKey[:])
	if err != nil {
		return key, chainKey, fmt.Errorf("failed to create cipher: %w", err)
	}

	plaintext, err := aead.Open(nil, dist.Nonce[:], dist.EncryptedKey, skm.groupID[:])
	if err != nil {
		return key, chainKey, fmt.Errorf("failed to decrypt sender key: %w", err)
	}
	if len(plaintext) != 64 {
		return key, chainKey, errors.New("invalid sender key length")
	}

	copy(key[:], plaintext[:32])
	copy(chainKey[:], plaintext[32:])
	return key, chainKey, nil
}

// ProcessDistribution processes a received sender key distribution from a peer.
// Decrypts and stores the peer's sender key for later message decryption.
// Uses ephemeral DH for forward secrecy.
func (skm *SenderKeyManager) ProcessDistribution(dist *SenderKeyDistribution) error {
	skm.mu.Lock()
	defer skm.mu.Unlock()

	if dist.GroupID != skm.groupID {
		return errors.New("group ID mismatch")
	}

	key, chainKey, err := skm.decryptSenderKeyFromDist(dist)
	if err != nil {
		return err
	}

	skm.peerSenderKeys[dist.SenderPublicKey] = &SenderKeyState{
		KeyID:          dist.KeyID,
		Key:            key,
		ChainKey:       chainKey,
		MessageCounter: ^uint64(0), // sentinel: no message seen yet
		CreatedAt:      time.Now(),
	}

	logrus.WithFields(logrus.Fields{
		"function":  "ProcessDistribution",
		"sender_pk": fmt.Sprintf("%x", dist.SenderPublicKey[:8]),
		"key_id":    dist.KeyID,
	}).Debug("Processed sender key distribution")

	return nil
}

// EncryptMessage encrypts a message using this member's sender key.
// Returns a SenderKeyMessage that can be broadcast to all group members.
// This achieves O(1) encryption regardless of group size.
func (skm *SenderKeyManager) EncryptMessage(plaintext []byte) (*SenderKeyMessage, error) {
	skm.mu.Lock()
	defer skm.mu.Unlock()

	if skm.mySenderKey == nil {
		return nil, errors.New("no sender key available")
	}

	// Check if key rotation is needed — refuse to encrypt rather than reuse a nonce
	// that would break forward-secrecy guarantees (F-GROUP-H1).
	if skm.mySenderKey.MessageCounter >= skm.maxMessageCounter {
		return nil, fmt.Errorf("sender key exhausted after %d messages: rotate key before sending", skm.mySenderKey.MessageCounter)
	}

	// Derive nonce from counter (deterministic, no random generation needed)
	nonce := deriveNonce(skm.mySenderKey.MessageCounter)

	// Create cipher
	aead, err := chacha20poly1305.New(skm.mySenderKey.Key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Encrypt with group ID as additional authenticated data
	ciphertext := aead.Seal(nil, nonce[:], plaintext, skm.groupID[:])

	msg := &SenderKeyMessage{
		GroupID:         skm.groupID,
		SenderPublicKey: skm.selfPublicKey,
		KeyID:           skm.mySenderKey.KeyID,
		Counter:         skm.mySenderKey.MessageCounter,
		Ciphertext:      ciphertext,
	}

	// Increment counter
	skm.mySenderKey.MessageCounter++

	return msg, nil
}

// DecryptMessage decrypts a message from another group member using their sender key.
func (skm *SenderKeyManager) DecryptMessage(msg *SenderKeyMessage) ([]byte, error) {
	skm.mu.Lock()
	defer skm.mu.Unlock()

	// Verify group ID
	if msg.GroupID != skm.groupID {
		return nil, errors.New("group ID mismatch")
	}

	// Look up the sender's key
	senderKey, exists := skm.peerSenderKeys[msg.SenderPublicKey]
	if !exists {
		return nil, fmt.Errorf("no sender key for peer %x", msg.SenderPublicKey[:8])
	}

	// Verify key ID matches (or handle key rotation)
	if msg.KeyID != senderKey.KeyID {
		return nil, fmt.Errorf("key ID mismatch: expected %d, got %d", senderKey.KeyID, msg.KeyID)
	}

	// Replay protection: reject messages with a counter ≤ last seen.
	// senderKey.MessageCounter is initialised to ^uint64(0) ("unseen") to
	// allow the very first message (Counter == 0) through.
	if senderKey.MessageCounter != ^uint64(0) && msg.Counter <= senderKey.MessageCounter {
		return nil, fmt.Errorf("replay detected: counter %d already seen (last=%d)", msg.Counter, senderKey.MessageCounter)
	}

	// Derive nonce from counter
	nonce := deriveNonce(msg.Counter)

	// Create cipher
	aead, err := chacha20poly1305.New(senderKey.Key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Decrypt
	plaintext, err := aead.Open(nil, nonce[:], msg.Ciphertext, skm.groupID[:])
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}

	// Advance the stored counter to prevent replay of this message.
	senderKey.MessageCounter = msg.Counter

	return plaintext, nil
}

// deriveNonce creates a 12-byte nonce from a message counter.
// Using counter-based nonces ensures uniqueness without random generation.
func deriveNonce(counter uint64) [12]byte {
	var nonce [12]byte
	binary.LittleEndian.PutUint64(nonce[:8], counter)
	return nonce
}

// RemovePeer removes a peer's sender key when they leave the group.
// After calling this, a key rotation should be triggered to maintain
// forward secrecy (the leaving member should not be able to decrypt
// future messages).
func (skm *SenderKeyManager) RemovePeer(peerPublicKey [32]byte) {
	skm.mu.Lock()
	defer skm.mu.Unlock()

	delete(skm.peerSenderKeys, peerPublicKey)

	logrus.WithFields(logrus.Fields{
		"function": "RemovePeer",
		"peer_pk":  fmt.Sprintf("%x", peerPublicKey[:8]),
	}).Debug("Removed peer sender key")
}

// GetPeerCount returns the number of peers with registered sender keys.
func (skm *SenderKeyManager) GetPeerCount() int {
	skm.mu.RLock()
	defer skm.mu.RUnlock()

	return len(skm.peerSenderKeys)
}

// HasPeerKey checks if a sender key exists for a given peer.
func (skm *SenderKeyManager) HasPeerKey(peerPublicKey [32]byte) bool {
	skm.mu.RLock()
	defer skm.mu.RUnlock()

	_, exists := skm.peerSenderKeys[peerPublicKey]
	return exists
}

// GetCurrentKeyID returns the current sender key ID for this member.
func (skm *SenderKeyManager) GetCurrentKeyID() uint32 {
	skm.mu.RLock()
	defer skm.mu.RUnlock()

	if skm.mySenderKey == nil {
		return 0
	}
	return skm.mySenderKey.KeyID
}

// SetOnKeyRotation sets the callback for key rotation events.
func (skm *SenderKeyManager) SetOnKeyRotation(callback func(newKey *SenderKeyDistribution)) {
	skm.mu.Lock()
	defer skm.mu.Unlock()

	skm.onKeyRotation = callback
}

// SetMaxMessageCounter sets the maximum message count before key rotation is recommended.
func (skm *SenderKeyManager) SetMaxMessageCounter(max uint64) {
	skm.mu.Lock()
	defer skm.mu.Unlock()

	skm.maxMessageCounter = max
}

// NeedsKeyRotation returns true if the sender key should be rotated
// (message counter has reached the maximum threshold).
func (skm *SenderKeyManager) NeedsKeyRotation() bool {
	skm.mu.RLock()
	defer skm.mu.RUnlock()

	if skm.mySenderKey == nil {
		return true
	}
	return skm.mySenderKey.MessageCounter >= skm.maxMessageCounter
}

// SerializeSenderKeyMessage serializes a SenderKeyMessage for network transmission.
func SerializeSenderKeyMessage(msg *SenderKeyMessage) ([]byte, error) {
	// Format: GroupID(32) + SenderPK(32) + KeyID(4) + Counter(8) + CiphertextLen(4) + Ciphertext
	size := 32 + 32 + 4 + 8 + 4 + len(msg.Ciphertext)
	data := make([]byte, size)

	offset := 0
	copy(data[offset:], msg.GroupID[:])
	offset += 32
	copy(data[offset:], msg.SenderPublicKey[:])
	offset += 32
	binary.LittleEndian.PutUint32(data[offset:], msg.KeyID)
	offset += 4
	binary.LittleEndian.PutUint64(data[offset:], msg.Counter)
	offset += 8
	binary.LittleEndian.PutUint32(data[offset:], uint32(len(msg.Ciphertext)))
	offset += 4
	copy(data[offset:], msg.Ciphertext)

	return data, nil
}

// DeserializeSenderKeyMessage deserializes a SenderKeyMessage from network data.
func DeserializeSenderKeyMessage(data []byte) (*SenderKeyMessage, error) {
	if len(data) < 80 { // Minimum size: 32+32+4+8+4 = 80
		return nil, errors.New("data too short for sender key message")
	}

	msg := &SenderKeyMessage{}

	offset := 0
	copy(msg.GroupID[:], data[offset:offset+32])
	offset += 32
	copy(msg.SenderPublicKey[:], data[offset:offset+32])
	offset += 32
	msg.KeyID = binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	msg.Counter = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	ciphertextLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	// Guard against integer overflow on 32-bit systems and unreasonably large ciphertexts
	// Maximum reasonable ciphertext size is 16MB (group message size limit)
	const maxCiphertextLen = 16 * 1024 * 1024
	if ciphertextLen > maxCiphertextLen {
		return nil, fmt.Errorf("ciphertext length %d exceeds maximum allowed %d", ciphertextLen, maxCiphertextLen)
	}

	if len(data) < offset+int(ciphertextLen) {
		return nil, errors.New("data too short for ciphertext")
	}

	msg.Ciphertext = make([]byte, ciphertextLen)
	copy(msg.Ciphertext, data[offset:offset+int(ciphertextLen)])

	return msg, nil
}

// SerializeSenderKeyDistribution serializes a SenderKeyDistribution for network transmission.
func SerializeSenderKeyDistribution(dist *SenderKeyDistribution) ([]byte, error) {
	// Format: GroupID(32) + SenderPK(32) + KeyID(4) + EphemeralPK(32) + Nonce(24) + EncKeyLen(4) + EncryptedKey
	size := 32 + 32 + 4 + 32 + 24 + 4 + len(dist.EncryptedKey)
	data := make([]byte, size)

	offset := 0
	copy(data[offset:], dist.GroupID[:])
	offset += 32
	copy(data[offset:], dist.SenderPublicKey[:])
	offset += 32
	binary.LittleEndian.PutUint32(data[offset:], dist.KeyID)
	offset += 4
	copy(data[offset:], dist.EphemeralPublicKey[:])
	offset += 32
	copy(data[offset:], dist.Nonce[:])
	offset += 24
	binary.LittleEndian.PutUint32(data[offset:], uint32(len(dist.EncryptedKey)))
	offset += 4
	copy(data[offset:], dist.EncryptedKey)

	return data, nil
}

// DeserializeSenderKeyDistribution deserializes a SenderKeyDistribution from network data.
func DeserializeSenderKeyDistribution(data []byte) (*SenderKeyDistribution, error) {
	if len(data) < 128 { // Minimum size: 32+32+4+32+24+4 = 128
		return nil, errors.New("data too short for sender key distribution")
	}

	dist := &SenderKeyDistribution{}

	offset := 0
	copy(dist.GroupID[:], data[offset:offset+32])
	offset += 32
	copy(dist.SenderPublicKey[:], data[offset:offset+32])
	offset += 32
	dist.KeyID = binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	copy(dist.EphemeralPublicKey[:], data[offset:offset+32])
	offset += 32
	copy(dist.Nonce[:], data[offset:offset+24])
	offset += 24
	encKeyLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	if len(data) < offset+int(encKeyLen) {
		return nil, errors.New("data too short for encrypted key")
	}

	dist.EncryptedKey = make([]byte, encKeyLen)
	copy(dist.EncryptedKey, data[offset:offset+int(encKeyLen)])

	return dist, nil
}
