// Package async implements an asynchronous message delivery system for Tox.
// This is an unofficial extension of the Tox protocol providing offline messaging
// capabilities while maintaining Tox's decentralized nature and security properties.
package async

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

var (
	// ErrMessageNotFound indicates a message was not found in storage
	ErrMessageNotFound = errors.New("message not found")
	// ErrStorageFull indicates the storage node is at capacity
	ErrStorageFull = errors.New("storage full")
	// ErrInvalidRecipient indicates an invalid recipient public key
	ErrInvalidRecipient = errors.New("invalid recipient")
)

// MessageType represents the type of message (normal or action)
type MessageType uint8

const (
	// MessageTypeNormal represents a normal text message
	MessageTypeNormal MessageType = iota
	// MessageTypeAction represents an action message (like "/me" in IRC)
	MessageTypeAction
)

// Constants for async message system
const (
	// MaxMessageSize is the maximum size for stored messages (same as regular Tox limit)
	MaxMessageSize = 1372
	// MaxStorageTime is how long messages are stored before expiration (24 hours)
	MaxStorageTime = 24 * time.Hour
	// MaxMessagesPerRecipient limits messages per recipient to prevent abuse
	MaxMessagesPerRecipient = 100
	// StorageNodeCapacity is the maximum number of messages a storage node can hold
	StorageNodeCapacity = 10000
	// EncryptionOverhead is the extra bytes added by nacl/box encryption (16 bytes)
	EncryptionOverhead = 16
)

// AsyncMessage represents a stored message with metadata
type AsyncMessage struct {
	ID            [16]byte    // Unique message identifier
	RecipientPK   [32]byte    // Recipient's public key
	SenderPK      [32]byte    // Sender's public key
	EncryptedData []byte      // Encrypted message content
	Timestamp     time.Time   // When message was stored
	Nonce         [24]byte    // Encryption nonce
	MessageType   MessageType // Normal or Action message
}

// MessageStorage handles distributed storage of async messages
type MessageStorage struct {
	mutex          sync.RWMutex
	messages       map[[16]byte]*AsyncMessage  // Message ID -> Message
	recipientIndex map[[32]byte][]AsyncMessage // Recipient PK -> Messages
	storageNodes   map[[32]byte]net.Addr       // Storage node public keys -> addresses
	keyPair        *crypto.KeyPair             // Our key pair for storage node operations
	dataDir        string                      // Directory for storage calculations
	maxCapacity    int                         // Dynamic capacity based on available storage
}

// NewMessageStorage creates a new message storage instance with dynamic capacity
func NewMessageStorage(keyPair *crypto.KeyPair, dataDir string) *MessageStorage {
	// Calculate dynamic capacity based on 1% of available storage
	bytesLimit, err := CalculateAsyncStorageLimit(dataDir)
	if err != nil {
		// Fallback to default capacity if calculation fails
		bytesLimit = uint64(StorageNodeCapacity * 650) // 650 bytes avg per message
	}

	maxCapacity := EstimateMessageCapacity(bytesLimit)

	return &MessageStorage{
		messages:       make(map[[16]byte]*AsyncMessage),
		recipientIndex: make(map[[32]byte][]AsyncMessage),
		storageNodes:   make(map[[32]byte]net.Addr),
		keyPair:        keyPair,
		dataDir:        dataDir,
		maxCapacity:    maxCapacity,
	}
}

// StoreMessage stores an encrypted message for later retrieval
// The message should be pre-encrypted by the sender for the recipient
func (ms *MessageStorage) StoreMessage(recipientPK, senderPK [32]byte,
	encryptedMessage []byte, nonce [24]byte, messageType MessageType) ([16]byte, error) {

	if len(encryptedMessage) == 0 {
		return [16]byte{}, errors.New("empty encrypted message")
	}

	if len(encryptedMessage) > MaxMessageSize+EncryptionOverhead {
		return [16]byte{}, fmt.Errorf("encrypted message too long: %d bytes (max %d)",
			len(encryptedMessage), MaxMessageSize+EncryptionOverhead)
	}

	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	// Check storage capacity (use dynamic capacity)
	if len(ms.messages) >= ms.maxCapacity {
		return [16]byte{}, ErrStorageFull
	}

	// Check per-recipient limit
	if len(ms.recipientIndex[recipientPK]) >= MaxMessagesPerRecipient {
		return [16]byte{}, fmt.Errorf("too many messages for recipient (max %d)",
			MaxMessagesPerRecipient)
	}

	// Generate unique message ID
	var messageID [16]byte
	_, err := rand.Read(messageID[:])
	if err != nil {
		return [16]byte{}, fmt.Errorf("failed to generate message ID: %w", err)
	}

	// Create and store message (data is already encrypted by sender)
	message := &AsyncMessage{
		ID:            messageID,
		RecipientPK:   recipientPK,
		SenderPK:      senderPK,
		EncryptedData: encryptedMessage,
		Timestamp:     time.Now(),
		Nonce:         nonce,
		MessageType:   messageType,
	}

	ms.messages[messageID] = message
	ms.recipientIndex[recipientPK] = append(ms.recipientIndex[recipientPK], *message)

	return messageID, nil
}

// RetrieveMessages retrieves all messages for a recipient
// Only the recipient can decrypt and read the messages
func (ms *MessageStorage) RetrieveMessages(recipientPK [32]byte) ([]AsyncMessage, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	messages := ms.recipientIndex[recipientPK]
	if len(messages) == 0 {
		return nil, ErrMessageNotFound
	}

	// Return copies to prevent external modification
	result := make([]AsyncMessage, len(messages))
	copy(result, messages)

	return result, nil
}

// DeleteMessage removes a message from storage after successful retrieval
func (ms *MessageStorage) DeleteMessage(messageID [16]byte, recipientPK [32]byte) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	message, exists := ms.messages[messageID]
	if !exists {
		return ErrMessageNotFound
	}

	// Verify the recipient is authorized to delete this message
	if message.RecipientPK != recipientPK {
		return errors.New("unauthorized deletion attempt")
	}

	// Remove from main storage
	delete(ms.messages, messageID)

	// Remove from recipient index
	recipientMessages := ms.recipientIndex[recipientPK]
	for i, msg := range recipientMessages {
		if msg.ID == messageID {
			// Remove this message from the slice
			ms.recipientIndex[recipientPK] = append(recipientMessages[:i],
				recipientMessages[i+1:]...)
			break
		}
	}

	// Clean up empty recipient index
	if len(ms.recipientIndex[recipientPK]) == 0 {
		delete(ms.recipientIndex, recipientPK)
	}

	return nil
}

// CleanupExpiredMessages removes messages older than MaxStorageTime
func (ms *MessageStorage) CleanupExpiredMessages() int {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	now := time.Now()
	expiredCount := 0

	// Find expired messages
	expiredIDs := make([][16]byte, 0)
	for id, message := range ms.messages {
		if now.Sub(message.Timestamp) > MaxStorageTime {
			expiredIDs = append(expiredIDs, id)
		}
	}

	// Remove expired messages
	for _, id := range expiredIDs {
		message := ms.messages[id]

		// Remove from main storage
		delete(ms.messages, id)

		// Remove from recipient index
		recipientMessages := ms.recipientIndex[message.RecipientPK]
		for i, msg := range recipientMessages {
			if msg.ID == id {
				ms.recipientIndex[message.RecipientPK] = append(recipientMessages[:i],
					recipientMessages[i+1:]...)
				break
			}
		}

		// Clean up empty recipient index
		if len(ms.recipientIndex[message.RecipientPK]) == 0 {
			delete(ms.recipientIndex, message.RecipientPK)
		}

		expiredCount++
	}

	return expiredCount
}

// GetStorageStats returns current storage statistics
func (ms *MessageStorage) GetStorageStats() StorageStats {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	return StorageStats{
		TotalMessages:    len(ms.messages),
		UniqueRecipients: len(ms.recipientIndex),
		StorageCapacity:  ms.maxCapacity,
		StorageNodes:     len(ms.storageNodes),
	}
}

// StorageStats provides information about storage utilization
type StorageStats struct {
	TotalMessages    int
	UniqueRecipients int
	StorageCapacity  int
	StorageNodes     int
}

// EncryptForRecipient is DEPRECATED - does not provide forward secrecy
// Use ForwardSecurityManager for forward-secure messaging instead
// This function is kept for backward compatibility only
func EncryptForRecipient(message []byte, recipientPK [32]byte, senderSK [32]byte) ([]byte, [24]byte, error) {
	// This function does not provide forward secrecy and should not be used for new applications
	return nil, [24]byte{}, errors.New("deprecated: EncryptForRecipient does not provide forward secrecy - use ForwardSecurityManager instead")
}

// encryptForRecipientInternal is an internal function for storage layer testing
// This should not be used in production code - use ForwardSecurityManager instead
func encryptForRecipientInternal(message []byte, recipientPK [32]byte, senderSK [32]byte) ([]byte, [24]byte, error) {
	if len(message) == 0 {
		return nil, [24]byte{}, errors.New("empty message")
	}

	if len(message) > MaxMessageSize {
		return nil, [24]byte{}, fmt.Errorf("message too long: %d bytes (max %d)",
			len(message), MaxMessageSize)
	}

	// Generate nonce for encryption
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		return nil, [24]byte{}, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt message for recipient
	encryptedData, err := crypto.Encrypt(message, nonce, recipientPK, senderSK)
	if err != nil {
		return nil, [24]byte{}, fmt.Errorf("failed to encrypt message: %w", err)
	}

	return encryptedData, nonce, nil
}

// GetMaxCapacity returns the current maximum storage capacity
func (ms *MessageStorage) GetMaxCapacity() int {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	return ms.maxCapacity
}

// UpdateCapacity recalculates and updates the storage capacity based on current disk space
func (ms *MessageStorage) UpdateCapacity() error {
	bytesLimit, err := CalculateAsyncStorageLimit(ms.dataDir)
	if err != nil {
		return err
	}

	newCapacity := EstimateMessageCapacity(bytesLimit)

	ms.mutex.Lock()
	ms.maxCapacity = newCapacity
	ms.mutex.Unlock()

	return nil
}

// GetStorageUtilization returns current storage utilization as a percentage
func (ms *MessageStorage) GetStorageUtilization() float64 {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	if ms.maxCapacity == 0 {
		return 0.0
	}

	return float64(len(ms.messages)) / float64(ms.maxCapacity) * 100.0
}
