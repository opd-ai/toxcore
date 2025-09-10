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
	"github.com/sirupsen/logrus"
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
	// MaxStorageCapacity is the maximum storage capacity (1GB / ~650 bytes per message)
	MaxStorageCapacity = 1536000
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

// MessageStorage handles distributed storage of async messages with privacy-preserving
// pseudonym-based indexing. It supports both legacy AsyncMessage format and new
// ObfuscatedAsyncMessage format for gradual migration.
type MessageStorage struct {
	mutex          sync.RWMutex
	messages       map[[16]byte]*AsyncMessage  // Message ID -> Legacy Message
	recipientIndex map[[32]byte][]AsyncMessage // Recipient PK -> Legacy Messages

	// Obfuscated message storage with pseudonym-based indexing
	obfuscatedMessages map[[32]byte]*ObfuscatedAsyncMessage              // Message ID -> Obfuscated Message
	pseudonymIndex     map[[32]byte]map[uint64][]*ObfuscatedAsyncMessage // Pseudonym -> Epoch -> Messages

	storageNodes map[[32]byte]net.Addr // Storage node public keys -> addresses
	keyPair      *crypto.KeyPair       // Our key pair for storage node operations
	epochManager *EpochManager         // Epoch management for pseudonym rotation
	dataDir      string                // Directory for storage calculations
	maxCapacity  int                   // Dynamic capacity based on available storage
}

// NewMessageStorage creates a new message storage instance with dynamic capacity
// and support for both legacy and obfuscated message formats.
func NewMessageStorage(keyPair *crypto.KeyPair, dataDir string) *MessageStorage {
	logrus.WithFields(logrus.Fields{
		"function":   "NewMessageStorage",
		"public_key": keyPair.Public[:8],
		"data_dir":   dataDir,
	}).Info("Creating new message storage")

	// Calculate dynamic capacity based on 1% of available storage
	bytesLimit, err := CalculateAsyncStorageLimit(dataDir)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewMessageStorage",
			"data_dir": dataDir,
			"error":    err.Error(),
		}).Warn("Failed to calculate storage limit, using default capacity")
		// Fallback to default capacity if calculation fails
		bytesLimit = uint64(StorageNodeCapacity * 650) // 650 bytes avg per message
	}

	maxCapacity := EstimateMessageCapacity(bytesLimit)

	logrus.WithFields(logrus.Fields{
		"function":     "NewMessageStorage",
		"public_key":   keyPair.Public[:8],
		"data_dir":     dataDir,
		"bytes_limit":  bytesLimit,
		"max_capacity": maxCapacity,
	}).Info("Calculated storage capacity")

	storage := &MessageStorage{
		messages:           make(map[[16]byte]*AsyncMessage),
		recipientIndex:     make(map[[32]byte][]AsyncMessage),
		obfuscatedMessages: make(map[[32]byte]*ObfuscatedAsyncMessage),
		pseudonymIndex:     make(map[[32]byte]map[uint64][]*ObfuscatedAsyncMessage),
		storageNodes:       make(map[[32]byte]net.Addr),
		keyPair:            keyPair,
		epochManager:       NewEpochManager(), // Initialize epoch manager
		dataDir:            dataDir,
		maxCapacity:        maxCapacity,
	}

	logrus.WithFields(logrus.Fields{
		"function":              "NewMessageStorage",
		"public_key":            keyPair.Public[:8],
		"max_capacity":          maxCapacity,
		"epoch_manager_created": storage.epochManager != nil,
		"data_structures_count": 4, // messages, recipientIndex, obfuscatedMessages, pseudonymIndex
	}).Info("Message storage created successfully")

	return storage
}

// StoreMessage stores an encrypted message for later retrieval
// The message should be pre-encrypted by the sender for the recipient
func (ms *MessageStorage) StoreMessage(recipientPK, senderPK [32]byte,
	encryptedMessage []byte, nonce [24]byte, messageType MessageType,
) ([16]byte, error) {
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

// CleanupExpiredMessages removes messages older than MaxStorageTime for both
// legacy and obfuscated message formats
// CleanupExpiredMessages removes expired messages from both legacy and obfuscated storage.
// Returns the total number of messages that were cleaned up.
func (ms *MessageStorage) CleanupExpiredMessages() int {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	now := time.Now()
	expiredCount := 0

	// Clean up legacy messages
	expiredCount += ms.cleanupExpiredLegacyMessages(now)

	// Clean up obfuscated messages
	expiredCount += ms.cleanupExpiredObfuscatedMessages(now)

	return expiredCount
}

// cleanupExpiredLegacyMessages removes expired legacy messages from storage and updates indices.
// Returns the number of legacy messages that were cleaned up.
func (ms *MessageStorage) cleanupExpiredLegacyMessages(now time.Time) int {
	expiredIDs := ms.findExpiredLegacyMessageIDs(now)
	return ms.removeLegacyMessages(expiredIDs)
}

// findExpiredLegacyMessageIDs identifies legacy messages that have exceeded MaxStorageTime.
func (ms *MessageStorage) findExpiredLegacyMessageIDs(now time.Time) [][16]byte {
	expiredIDs := make([][16]byte, 0)
	for id, message := range ms.messages {
		if now.Sub(message.Timestamp) > MaxStorageTime {
			expiredIDs = append(expiredIDs, id)
		}
	}
	return expiredIDs
}

// removeLegacyMessages removes legacy messages by ID and updates recipient indices.
func (ms *MessageStorage) removeLegacyMessages(expiredIDs [][16]byte) int {
	expiredCount := 0
	for _, id := range expiredIDs {
		message := ms.messages[id]

		// Remove from main storage
		delete(ms.messages, id)

		// Remove from recipient index
		ms.removeFromRecipientIndex(message.RecipientPK, id)

		expiredCount++
	}
	return expiredCount
}

// removeFromRecipientIndex removes a message from the recipient index and cleans up empty entries.
func (ms *MessageStorage) removeFromRecipientIndex(recipientPK [32]byte, messageID [16]byte) {
	recipientMessages := ms.recipientIndex[recipientPK]
	for i, msg := range recipientMessages {
		if msg.ID == messageID {
			ms.recipientIndex[recipientPK] = append(recipientMessages[:i],
				recipientMessages[i+1:]...)
			break
		}
	}

	// Clean up empty recipient index
	if len(ms.recipientIndex[recipientPK]) == 0 {
		delete(ms.recipientIndex, recipientPK)
	}
}

// cleanupExpiredObfuscatedMessages removes expired obfuscated messages from storage and updates indices.
// Returns the number of obfuscated messages that were cleaned up.
func (ms *MessageStorage) cleanupExpiredObfuscatedMessages(now time.Time) int {
	expiredIDs := ms.findExpiredObfuscatedMessageIDs(now)
	return ms.removeObfuscatedMessages(expiredIDs)
}

// findExpiredObfuscatedMessageIDs identifies obfuscated messages that have passed their expiration time.
func (ms *MessageStorage) findExpiredObfuscatedMessageIDs(now time.Time) [][32]byte {
	expiredObfuscatedIDs := make([][32]byte, 0)
	for id, message := range ms.obfuscatedMessages {
		if now.After(message.ExpiresAt) {
			expiredObfuscatedIDs = append(expiredObfuscatedIDs, id)
		}
	}
	return expiredObfuscatedIDs
}

// removeObfuscatedMessages removes obfuscated messages by ID and updates pseudonym indices.
func (ms *MessageStorage) removeObfuscatedMessages(expiredIDs [][32]byte) int {
	expiredCount := 0
	for _, id := range expiredIDs {
		message := ms.obfuscatedMessages[id]

		// Remove from main storage
		delete(ms.obfuscatedMessages, id)

		// Remove from pseudonym index
		ms.removeFromPseudonymIndex(message.RecipientPseudonym, message.Epoch, id)

		expiredCount++
	}
	return expiredCount
}

// removeFromPseudonymIndex removes a message from the pseudonym index and cleans up empty entries.
func (ms *MessageStorage) removeFromPseudonymIndex(pseudonym [32]byte, epoch uint64, messageID [32]byte) {
	pseudonymMessages := ms.pseudonymIndex[pseudonym]
	if pseudonymMessages != nil {
		epochMessages := pseudonymMessages[epoch]
		for i, msg := range epochMessages {
			if msg.MessageID == messageID {
				ms.pseudonymIndex[pseudonym][epoch] = append(
					epochMessages[:i], epochMessages[i+1:]...)
				break
			}
		}

		// Clean up empty epoch
		if len(ms.pseudonymIndex[pseudonym][epoch]) == 0 {
			delete(ms.pseudonymIndex[pseudonym], epoch)
		}

		// Clean up empty pseudonym index
		if len(ms.pseudonymIndex[pseudonym]) == 0 {
			delete(ms.pseudonymIndex, pseudonym)
		}
	}
}

// StoreObfuscatedMessage stores an obfuscated message using pseudonym-based indexing.
// This provides privacy by hiding real sender and recipient identities from storage nodes.
func (ms *MessageStorage) StoreObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage) error {
	if err := ms.validateObfuscatedMessage(obfMsg); err != nil {
		return err
	}

	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	if err := ms.checkStorageCapacity(obfMsg); err != nil {
		return err
	}

	ms.storeAndIndexMessage(obfMsg)
	return nil
}

// validateObfuscatedMessage performs comprehensive validation of an obfuscated message.
func (ms *MessageStorage) validateObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage) error {
	if obfMsg == nil {
		return errors.New("nil obfuscated message")
	}

	// Validate epoch is current or recent
	if !ms.epochManager.IsValidEpoch(obfMsg.Epoch) {
		return fmt.Errorf("invalid epoch %d (too old or future)", obfMsg.Epoch)
	}

	// Validate message has not expired
	if time.Now().After(obfMsg.ExpiresAt) {
		return errors.New("message has expired")
	}

	// Validate payload size
	if len(obfMsg.EncryptedPayload) == 0 {
		return errors.New("empty encrypted payload")
	}

	if len(obfMsg.EncryptedPayload) > MaxMessageSize+EncryptionOverhead {
		return fmt.Errorf("encrypted payload too long: %d bytes (max %d)",
			len(obfMsg.EncryptedPayload), MaxMessageSize+EncryptionOverhead)
	}

	return nil
}

// checkStorageCapacity verifies total storage and per-pseudonym limits.
func (ms *MessageStorage) checkStorageCapacity(obfMsg *ObfuscatedAsyncMessage) error {
	// Check total storage capacity (include both legacy and obfuscated messages)
	totalMessages := len(ms.messages) + len(ms.obfuscatedMessages)
	if totalMessages >= ms.maxCapacity {
		return ErrStorageFull
	}

	// Check per-pseudonym limit to prevent spam
	pseudonymMessages := ms.pseudonymIndex[obfMsg.RecipientPseudonym]
	totalForPseudonym := 0
	for _, epochMessages := range pseudonymMessages {
		totalForPseudonym += len(epochMessages)
	}
	if totalForPseudonym >= MaxMessagesPerRecipient {
		return fmt.Errorf("too many messages for recipient pseudonym (max %d)", MaxMessagesPerRecipient)
	}

	return nil
}

// storeAndIndexMessage stores the message and updates the pseudonym index.
func (ms *MessageStorage) storeAndIndexMessage(obfMsg *ObfuscatedAsyncMessage) {
	// Store the obfuscated message
	ms.obfuscatedMessages[obfMsg.MessageID] = obfMsg

	// Update pseudonym index
	if ms.pseudonymIndex[obfMsg.RecipientPseudonym] == nil {
		ms.pseudonymIndex[obfMsg.RecipientPseudonym] = make(map[uint64][]*ObfuscatedAsyncMessage)
	}
	if ms.pseudonymIndex[obfMsg.RecipientPseudonym][obfMsg.Epoch] == nil {
		ms.pseudonymIndex[obfMsg.RecipientPseudonym][obfMsg.Epoch] = make([]*ObfuscatedAsyncMessage, 0)
	}

	ms.pseudonymIndex[obfMsg.RecipientPseudonym][obfMsg.Epoch] = append(
		ms.pseudonymIndex[obfMsg.RecipientPseudonym][obfMsg.Epoch], obfMsg)
}

// RetrieveMessagesByPseudonym retrieves obfuscated messages for a specific recipient
// pseudonym across multiple epochs. This allows recipients to find their messages
// without revealing their real identity to storage nodes.
func (ms *MessageStorage) RetrieveMessagesByPseudonym(recipientPseudonym [32]byte, epochs []uint64) ([]*ObfuscatedAsyncMessage, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	var allMessages []*ObfuscatedAsyncMessage

	pseudonymMessages, exists := ms.pseudonymIndex[recipientPseudonym]
	if !exists {
		return nil, ErrMessageNotFound
	}

	// Collect messages from requested epochs
	for _, epoch := range epochs {
		epochMessages, exists := pseudonymMessages[epoch]
		if !exists {
			continue // No messages for this epoch
		}

		// Create copies to prevent external modification
		for _, msg := range epochMessages {
			msgCopy := *msg
			allMessages = append(allMessages, &msgCopy)
		}
	}

	if len(allMessages) == 0 {
		return nil, ErrMessageNotFound
	}

	return allMessages, nil
}

// RetrieveRecentObfuscatedMessages retrieves all obfuscated messages for a recipient
// pseudonym from recent epochs (current + 3 previous epochs).
func (ms *MessageStorage) RetrieveRecentObfuscatedMessages(recipientPseudonym [32]byte) ([]*ObfuscatedAsyncMessage, error) {
	recentEpochs := ms.epochManager.GetRecentEpochs()
	return ms.RetrieveMessagesByPseudonym(recipientPseudonym, recentEpochs)
}

// DeleteObfuscatedMessage removes an obfuscated message from storage after successful retrieval.
func (ms *MessageStorage) DeleteObfuscatedMessage(messageID [32]byte, recipientPseudonym [32]byte) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	message, err := ms.validateObfuscatedMessageDeletion(messageID, recipientPseudonym)
	if err != nil {
		return err
	}

	ms.removeObfuscatedMessageFromStorage(messageID)
	ms.cleanupPseudonymIndex(recipientPseudonym, messageID, message.Epoch)

	return nil
}

// validateObfuscatedMessageDeletion checks if the message exists and the deletion is authorized.
func (ms *MessageStorage) validateObfuscatedMessageDeletion(messageID [32]byte, recipientPseudonym [32]byte) (*ObfuscatedAsyncMessage, error) {
	message, exists := ms.obfuscatedMessages[messageID]
	if !exists {
		return nil, ErrMessageNotFound
	}

	// Verify the pseudonym matches (authorization check)
	if message.RecipientPseudonym != recipientPseudonym {
		return nil, errors.New("unauthorized deletion attempt")
	}

	return message, nil
}

// removeObfuscatedMessageFromStorage removes the message from the main storage map.
func (ms *MessageStorage) removeObfuscatedMessageFromStorage(messageID [32]byte) {
	delete(ms.obfuscatedMessages, messageID)
}

// cleanupPseudonymIndex removes the message from pseudonym index and cleans up empty entries.
func (ms *MessageStorage) cleanupPseudonymIndex(recipientPseudonym [32]byte, messageID [32]byte, epoch uint64) {
	pseudonymMessages := ms.pseudonymIndex[recipientPseudonym]
	if pseudonymMessages == nil {
		return
	}

	ms.removeMessageFromEpoch(recipientPseudonym, pseudonymMessages, messageID, epoch)
	ms.cleanupEmptyEpoch(recipientPseudonym, epoch)
	ms.cleanupEmptyPseudonym(recipientPseudonym)
}

// removeMessageFromEpoch removes a specific message from its epoch slice.
func (ms *MessageStorage) removeMessageFromEpoch(recipientPseudonym [32]byte, pseudonymMessages map[uint64][]*ObfuscatedAsyncMessage, messageID [32]byte, epoch uint64) {
	epochMessages := pseudonymMessages[epoch]
	for i, msg := range epochMessages {
		if msg.MessageID == messageID {
			// Remove this message from the slice
			ms.pseudonymIndex[recipientPseudonym][epoch] = append(
				epochMessages[:i], epochMessages[i+1:]...)
			break
		}
	}
}

// cleanupEmptyEpoch removes empty epoch entries from the pseudonym index.
func (ms *MessageStorage) cleanupEmptyEpoch(recipientPseudonym [32]byte, epoch uint64) {
	if len(ms.pseudonymIndex[recipientPseudonym][epoch]) == 0 {
		delete(ms.pseudonymIndex[recipientPseudonym], epoch)
	}
}

// cleanupEmptyPseudonym removes empty pseudonym entries from the index.
func (ms *MessageStorage) cleanupEmptyPseudonym(recipientPseudonym [32]byte) {
	if len(ms.pseudonymIndex[recipientPseudonym]) == 0 {
		delete(ms.pseudonymIndex, recipientPseudonym)
	}
}

// GetStorageStats returns current storage statistics for both legacy and obfuscated messages
func (ms *MessageStorage) GetStorageStats() StorageStats {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	return StorageStats{
		TotalMessages:      len(ms.messages) + len(ms.obfuscatedMessages),
		LegacyMessages:     len(ms.messages),
		ObfuscatedMessages: len(ms.obfuscatedMessages),
		UniqueRecipients:   len(ms.recipientIndex),
		UniquePseudonyms:   len(ms.pseudonymIndex),
		StorageCapacity:    ms.maxCapacity,
		StorageNodes:       len(ms.storageNodes),
	}
}

// StorageStats provides information about storage utilization
type StorageStats struct {
	TotalMessages      int
	LegacyMessages     int // Count of traditional AsyncMessage
	ObfuscatedMessages int // Count of ObfuscatedAsyncMessage
	UniqueRecipients   int // Count of unique recipient public keys (legacy)
	UniquePseudonyms   int // Count of unique recipient pseudonyms (obfuscated)
	StorageCapacity    int
	StorageNodes       int
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
// including both legacy and obfuscated messages
func (ms *MessageStorage) GetStorageUtilization() float64 {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	if ms.maxCapacity == 0 {
		return 0.0
	}

	totalMessages := len(ms.messages) + len(ms.obfuscatedMessages)
	return float64(totalMessages) / float64(ms.maxCapacity) * 100.0
}

// GetEpochManager returns the epoch manager for external access.
// This allows other components to perform epoch-related operations.
func (ms *MessageStorage) GetEpochManager() *EpochManager {
	return ms.epochManager
}

// CleanupOldEpochs removes obfuscated messages from epochs older than the valid range.
// This helps maintain storage efficiency by removing messages that can no longer be retrieved.
func (ms *MessageStorage) CleanupOldEpochs() int {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	cleanedCount := 0
	currentEpoch := ms.epochManager.GetCurrentEpoch()

	for pseudonym, epochMap := range ms.pseudonymIndex {
		for epoch, messages := range epochMap {
			// Remove epochs that are too old (more than 3 epochs ago)
			if currentEpoch > epoch && currentEpoch-epoch > 3 {
				for _, msg := range messages {
					// Remove from main storage
					delete(ms.obfuscatedMessages, msg.MessageID)
					cleanedCount++
				}
				// Remove epoch from pseudonym index
				delete(ms.pseudonymIndex[pseudonym], epoch)
			}
		}

		// Clean up empty pseudonym entries
		if len(ms.pseudonymIndex[pseudonym]) == 0 {
			delete(ms.pseudonymIndex, pseudonym)
		}
	}

	return cleanedCount
}
