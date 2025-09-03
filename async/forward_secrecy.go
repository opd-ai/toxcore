package async

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// ForwardSecureMessage represents an async message with forward secrecy
type ForwardSecureMessage struct {
	Type          string      `json:"type"`
	MessageID     [32]byte    `json:"message_id"`
	SenderPK      [32]byte    `json:"sender_pk"`
	RecipientPK   [32]byte    `json:"recipient_pk"`
	PreKeyID      uint32      `json:"pre_key_id"`     // ID of the one-time key used
	EncryptedData []byte      `json:"encrypted_data"` // Message encrypted with one-time key
	Nonce         [24]byte    `json:"nonce"`
	MessageType   MessageType `json:"message_type"`
	Timestamp     time.Time   `json:"timestamp"`
	ExpiresAt     time.Time   `json:"expires_at"`
}

// PreKeyExchangeMessage is sent when peers come online to exchange/refresh pre-keys
type PreKeyExchangeMessage struct {
	Type      string              `json:"type"`
	SenderPK  [32]byte            `json:"sender_pk"`
	PreKeys   []PreKeyForExchange `json:"pre_keys"`
	Timestamp time.Time           `json:"timestamp"`
}

// PreKeyForExchange represents a pre-key being shared (without private key)
type PreKeyForExchange struct {
	ID        uint32   `json:"id"`
	PublicKey [32]byte `json:"public_key"`
}

// ForwardSecurityManager handles forward-secure async messaging
type ForwardSecurityManager struct {
	preKeyStore *PreKeyStore
	keyPair     *crypto.KeyPair
	peerPreKeys map[[32]byte][]PreKeyForExchange // Pre-keys received from peers
}

// NewForwardSecurityManager creates a new forward security manager
func NewForwardSecurityManager(keyPair *crypto.KeyPair, dataDir string) (*ForwardSecurityManager, error) {
	preKeyStore, err := NewPreKeyStore(keyPair, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create pre-key store: %w", err)
	}

	return &ForwardSecurityManager{
		preKeyStore: preKeyStore,
		keyPair:     keyPair,
		peerPreKeys: make(map[[32]byte][]PreKeyForExchange),
	}, nil
}

// GeneratePreKeysForPeer generates pre-keys for a specific peer
func (fsm *ForwardSecurityManager) GeneratePreKeysForPeer(peerPK [32]byte) error {
	_, err := fsm.preKeyStore.GeneratePreKeys(peerPK)
	return err
}

// SendForwardSecureMessage sends an async message using forward secrecy
func (fsm *ForwardSecurityManager) SendForwardSecureMessage(recipientPK [32]byte, message []byte, messageType MessageType) (*ForwardSecureMessage, error) {
	if len(message) == 0 {
		return nil, errors.New("empty message")
	}

	if len(message) > MaxMessageSize {
		return nil, fmt.Errorf("message too long: %d bytes (max %d)", len(message), MaxMessageSize)
	}

	// Check if we have pre-keys for this recipient
	peerPreKeys, exists := fsm.peerPreKeys[recipientPK]
	if !exists || len(peerPreKeys) == 0 {
		return nil, fmt.Errorf("no pre-keys available for recipient %x - cannot send forward-secure message", recipientPK[:8])
	}

	// Use the first available pre-key (FIFO)
	preKey := peerPreKeys[0]

	// Remove used pre-key from available pool
	fsm.peerPreKeys[recipientPK] = peerPreKeys[1:]

	// Generate random nonce
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt message using the one-time pre-key
	encryptedData, err := crypto.Encrypt(message, nonce, preKey.PublicKey, fsm.keyPair.Private)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message with pre-key: %w", err)
	}

	// Generate message ID
	var messageID [32]byte
	if _, err := rand.Read(messageID[:]); err != nil {
		return nil, fmt.Errorf("failed to generate message ID: %w", err)
	}

	return &ForwardSecureMessage{
		Type:          "forward_secure_message",
		MessageID:     messageID,
		SenderPK:      fsm.keyPair.Public,
		RecipientPK:   recipientPK,
		PreKeyID:      preKey.ID,
		EncryptedData: encryptedData,
		Nonce:         nonce,
		MessageType:   messageType,
		Timestamp:     time.Now(),
		ExpiresAt:     time.Now().Add(MaxStorageTime),
	}, nil
}

// DecryptForwardSecureMessage decrypts a received forward-secure message
func (fsm *ForwardSecurityManager) DecryptForwardSecureMessage(msg *ForwardSecureMessage) ([]byte, error) {
	// Find the pre-key used for this message
	preKey, err := fsm.preKeyStore.GetPreKeyByID(msg.SenderPK, msg.PreKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to find pre-key %d for sender %x: %w", msg.PreKeyID, msg.SenderPK[:8], err)
	}

	if preKey.Used {
		return nil, fmt.Errorf("pre-key %d already used - possible replay attack", msg.PreKeyID)
	}

	// Decrypt message using the one-time pre-key
	decryptedData, err := crypto.Decrypt(msg.EncryptedData, msg.Nonce, msg.SenderPK, preKey.KeyPair.Private)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}

	// Mark pre-key as used to prevent replay attacks
	if err := fsm.preKeyStore.MarkPreKeyUsed(msg.SenderPK, msg.PreKeyID); err != nil {
		return nil, fmt.Errorf("failed to mark pre-key as used: %w", err)
	}

	return decryptedData, nil
}

// ExchangePreKeys creates a pre-key exchange message for a peer
func (fsm *ForwardSecurityManager) ExchangePreKeys(peerPK [32]byte) (*PreKeyExchangeMessage, error) {
	// Check if we need to generate pre-keys for this peer
	if fsm.preKeyStore.NeedsRefresh(peerPK) {
		if _, err := fsm.preKeyStore.RefreshPreKeys(peerPK); err != nil {
			return nil, fmt.Errorf("failed to refresh pre-keys: %w", err)
		}
	}

	// Get our pre-key bundle for this peer
	bundle, err := fsm.preKeyStore.GetBundle(peerPK)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-key bundle: %w", err)
	}

	// Create exchange message with public keys only
	preKeysForExchange := make([]PreKeyForExchange, 0, len(bundle.Keys))
	for _, key := range bundle.Keys {
		if !key.Used {
			preKeysForExchange = append(preKeysForExchange, PreKeyForExchange{
				ID:        key.ID,
				PublicKey: key.KeyPair.Public,
			})
		}
	}

	return &PreKeyExchangeMessage{
		Type:      "pre_key_exchange",
		SenderPK:  fsm.keyPair.Public,
		PreKeys:   preKeysForExchange,
		Timestamp: time.Now(),
	}, nil
}

// ProcessPreKeyExchange processes received pre-keys from a peer
func (fsm *ForwardSecurityManager) ProcessPreKeyExchange(exchange *PreKeyExchangeMessage) error {
	if len(exchange.PreKeys) == 0 {
		return errors.New("empty pre-key exchange")
	}

	// Store pre-keys for this peer (replacing any existing ones)
	fsm.peerPreKeys[exchange.SenderPK] = exchange.PreKeys

	return nil
}

// GetAvailableKeyCount returns the number of available pre-keys for a peer
func (fsm *ForwardSecurityManager) GetAvailableKeyCount(peerPK [32]byte) int {
	if preKeys, exists := fsm.peerPreKeys[peerPK]; exists {
		return len(preKeys)
	}
	return 0
}

// NeedsKeyExchange checks if we need to exchange pre-keys with a peer
func (fsm *ForwardSecurityManager) NeedsKeyExchange(peerPK [32]byte) bool {
	// Need exchange if we have no keys or very few keys remaining
	availableKeys := fsm.GetAvailableKeyCount(peerPK)
	return availableKeys <= PreKeyRefreshThreshold
}

// CanSendMessage checks if we can send a forward-secure message to a peer
func (fsm *ForwardSecurityManager) CanSendMessage(peerPK [32]byte) bool {
	return fsm.GetAvailableKeyCount(peerPK) > 0
}

// CleanupExpiredData removes old pre-keys and expired data
func (fsm *ForwardSecurityManager) CleanupExpiredData() {
	// Cleanup local pre-key bundles
	fsm.preKeyStore.CleanupExpiredBundles()

	// Remove expired peer pre-keys (optional - could keep them longer)
	// For now, we'll keep peer pre-keys until they're used or refreshed
}
