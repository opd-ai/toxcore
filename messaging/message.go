// Package messaging implements the messaging system for the Tox protocol.
//
// This package handles sending and receiving messages between Tox users,
// including message formatting, delivery confirmation, and offline messaging.
//
// Example:
//
//	msg := messaging.NewMessage(friendID, "Hello, world!")
//	if err := msg.Send(); err != nil {
//	    log.Fatal(err)
//	}
package messaging

import (
	"errors"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/limits"
	"github.com/sirupsen/logrus"
)

// ErrMessageTooLong indicates the message exceeds the maximum allowed size.
var ErrMessageTooLong = errors.New("message exceeds maximum length")

// MessageType represents the type of message.
type MessageType uint8

const (
	// MessageTypeNormal is a regular text message.
	MessageTypeNormal MessageType = iota
	// MessageTypeAction is an action message (like /me).
	MessageTypeAction
)

// MessageState represents the delivery state of a message.
type MessageState uint8

const (
	// MessageStatePending means the message is waiting to be sent.
	MessageStatePending MessageState = iota
	// MessageStateSending means the message is being sent.
	MessageStateSending
	// MessageStateSent means the message has been sent but not confirmed.
	MessageStateSent
	// MessageStateDelivered means the message has been delivered to the recipient.
	MessageStateDelivered
	// MessageStateRead means the message has been read by the recipient.
	MessageStateRead
	// MessageStateFailed means the message failed to send.
	MessageStateFailed
)

// DeliveryCallback is called when a message's delivery state changes.
type DeliveryCallback func(message *Message, state MessageState)

// MessageTransport defines the interface for sending messages via transport layer.
type MessageTransport interface {
	SendMessagePacket(friendID uint32, message *Message) error
}

// KeyProvider defines the interface for retrieving friend public keys.
type KeyProvider interface {
	GetFriendPublicKey(friendID uint32) ([32]byte, error)
	GetSelfPrivateKey() [32]byte
}

// TimeProvider abstracts time operations for deterministic testing and
// prevents timing side-channel attacks by allowing controlled time injection.
type TimeProvider interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// DefaultTimeProvider uses the standard library time functions.
type DefaultTimeProvider struct{}

// Now returns the current time.
func (DefaultTimeProvider) Now() time.Time { return time.Now() }

// Since returns the duration since t.
func (DefaultTimeProvider) Since(t time.Time) time.Duration { return time.Since(t) }

// Message represents a Tox message.
//
//export ToxMessage
type Message struct {
	ID          uint32
	FriendID    uint32
	Type        MessageType
	Text        string
	Timestamp   time.Time
	State       MessageState
	Retries     uint8
	LastAttempt time.Time

	deliveryCallback DeliveryCallback

	mu sync.Mutex
}

// MessageManager handles message sending, receiving, and tracking.
type MessageManager struct {
	messages      map[uint32]*Message
	nextID        uint32
	pendingQueue  []*Message
	maxRetries    uint8
	retryInterval time.Duration
	transport     MessageTransport
	keyProvider   KeyProvider
	timeProvider  TimeProvider

	mu sync.Mutex
}

// NewMessage creates a new message.
//
//export ToxMessageNew
func NewMessage(friendID uint32, text string, messageType MessageType) *Message {
	return newMessageWithTime(friendID, text, messageType, time.Now())
}

// newMessageWithTime creates a new message with an explicit timestamp.
// This is used internally to support deterministic time for testing.
func newMessageWithTime(friendID uint32, text string, messageType MessageType, timestamp time.Time) *Message {
	logrus.WithFields(logrus.Fields{
		"function":     "NewMessage",
		"friend_id":    friendID,
		"message_type": messageType,
		"text_length":  len(text),
	}).Info("Creating new message")

	message := &Message{
		FriendID:    friendID,
		Type:        messageType,
		Text:        text,
		Timestamp:   timestamp,
		State:       MessageStatePending,
		Retries:     0,
		LastAttempt: time.Time{}, // Zero time
	}

	logrus.WithFields(logrus.Fields{
		"function":     "NewMessage",
		"friend_id":    friendID,
		"message_type": messageType,
		"timestamp":    message.Timestamp,
	}).Debug("Message created successfully")

	return message
}

// OnDeliveryStateChange sets a callback for delivery state changes.
//
//export ToxMessageOnDeliveryStateChange
func (m *Message) OnDeliveryStateChange(callback DeliveryCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deliveryCallback = callback
}

// SetState updates the message's delivery state.
func (m *Message) SetState(state MessageState) {
	m.mu.Lock()
	m.State = state
	callback := m.deliveryCallback
	m.mu.Unlock()

	if callback != nil {
		callback(m, state)
	}
}

// NewMessageManager creates a new message manager.
func NewMessageManager() *MessageManager {
	return &MessageManager{
		messages:      make(map[uint32]*Message),
		pendingQueue:  make([]*Message, 0),
		maxRetries:    3,
		retryInterval: 5 * time.Second,
		nextID:        1,
		timeProvider:  DefaultTimeProvider{},
	}
}

// SetTimeProvider sets the time provider for deterministic testing.
func (mm *MessageManager) SetTimeProvider(tp TimeProvider) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.timeProvider = tp
}

// SetTransport sets the transport layer for sending messages.
func (mm *MessageManager) SetTransport(transport MessageTransport) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.transport = transport
}

// SetKeyProvider sets the key provider for encryption.
func (mm *MessageManager) SetKeyProvider(keyProvider KeyProvider) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.keyProvider = keyProvider
}

// SendMessage sends a message to a friend.
//
//export ToxSendMessage
func (mm *MessageManager) SendMessage(friendID uint32, text string, messageType MessageType) (*Message, error) {
	if len(text) == 0 {
		return nil, errors.New("message text cannot be empty")
	}
	if len(text) > limits.MaxPlaintextMessage {
		return nil, ErrMessageTooLong
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Create a new message using injected time provider
	message := newMessageWithTime(friendID, text, messageType, mm.timeProvider.Now())
	message.ID = mm.nextID
	mm.nextID++

	// Store the message
	mm.messages[message.ID] = message

	// Add to pending queue
	mm.pendingQueue = append(mm.pendingQueue, message)

	// Trigger immediate send attempt
	go mm.attemptMessageSend(message)

	return message, nil
}

// ProcessPendingMessages attempts to send pending messages.
func (mm *MessageManager) ProcessPendingMessages() {
	pendingMessages := mm.retrievePendingMessages()
	mm.processMessageBatch(pendingMessages)
	mm.cleanupProcessedMessages()
}

// retrievePendingMessages safely copies the pending message queue.
func (mm *MessageManager) retrievePendingMessages() []*Message {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	pending := make([]*Message, len(mm.pendingQueue))
	copy(pending, mm.pendingQueue)
	return pending
}

// processMessageBatch attempts to send each message in the batch.
func (mm *MessageManager) processMessageBatch(messages []*Message) {
	for _, message := range messages {
		if mm.shouldProcessMessage(message) {
			mm.attemptMessageSend(message)
		}
	}
}

// shouldProcessMessage checks if a message is ready to be processed.
func (mm *MessageManager) shouldProcessMessage(message *Message) bool {
	message.mu.Lock()
	defer message.mu.Unlock()

	// Skip messages that are not pending or already being sent
	if message.State != MessageStatePending {
		return false
	}

	// Check if we need to wait before retrying (uses injected time provider)
	if !message.LastAttempt.IsZero() && mm.timeProvider.Since(message.LastAttempt) < mm.retryInterval {
		return false
	}

	return true
}

// PaddingSizes defines the standard message padding tiers for traffic analysis resistance.
// Messages are padded to the smallest size that can contain them.
var PaddingSizes = []int{256, 1024, 4096}

// padMessage pads data to the nearest standard size boundary for traffic analysis resistance.
// Returns the original data unchanged if it exceeds all padding tiers.
func padMessage(data []byte) []byte {
	for _, size := range PaddingSizes {
		if len(data) <= size {
			padded := make([]byte, size)
			copy(padded, data)
			return padded
		}
	}
	return data
}

// encryptMessage encrypts a message for the recipient friend.
func (mm *MessageManager) encryptMessage(message *Message) error {
	// Check if encryption is available
	if mm.keyProvider == nil {
		// No key provider configured - send unencrypted (backward compatibility)
		logrus.WithFields(logrus.Fields{
			"friend_id":    message.FriendID,
			"message_type": message.Type,
		}).Warn("Sending message without encryption: no key provider configured")
		return nil
	}

	// Get friend's public key
	recipientPK, err := mm.keyProvider.GetFriendPublicKey(message.FriendID)
	if err != nil {
		return err
	}

	// Get our private key
	senderSK := mm.keyProvider.GetSelfPrivateKey()

	// Generate nonce
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		return err
	}

	// Pad message to standard size for traffic analysis resistance
	paddedData := padMessage([]byte(message.Text))

	// Encrypt the padded message text
	encryptedData, err := crypto.Encrypt(paddedData, nonce, recipientPK, senderSK)
	if err != nil {
		return err
	}

	// Replace message text with encrypted data (base64 or hex encoding would be done at transport layer)
	message.Text = string(encryptedData)

	return nil
}

// attemptMessageSend attempts to send a message through the transport layer.
func (mm *MessageManager) attemptMessageSend(message *Message) {
	message.mu.Lock()
	message.State = MessageStateSending
	message.LastAttempt = mm.timeProvider.Now()
	message.Retries++
	message.mu.Unlock()

	// Encrypt the message (or log warning if encryption not available)
	err := mm.encryptMessage(message)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "attemptMessageSend",
			"friend_id": message.FriendID,
			"error":     err.Error(),
		}).Error("Failed to encrypt message")

		// Mark as failed if encryption fails
		if message.Retries >= mm.maxRetries {
			message.SetState(MessageStateFailed)
		} else {
			message.SetState(MessageStatePending)
		}
		return
	}

	// Try to send through transport layer if available
	if mm.transport != nil {
		err := mm.transport.SendMessagePacket(message.FriendID, message)
		if err != nil {
			// Failed to send - mark as failed if max retries exceeded
			if message.Retries >= mm.maxRetries {
				message.SetState(MessageStateFailed)
			} else {
				// Reset to pending for retry
				message.SetState(MessageStatePending)
			}
			return
		}
	}

	// Successfully sent (or no transport configured)
	message.SetState(MessageStateSent)
}

// cleanupProcessedMessages removes completed messages from the pending queue.
func (mm *MessageManager) cleanupProcessedMessages() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	newPending := make([]*Message, 0, len(mm.pendingQueue))
	for _, message := range mm.pendingQueue {
		if mm.shouldKeepInQueue(message) {
			newPending = append(newPending, message)
		}
	}
	mm.pendingQueue = newPending
}

// shouldKeepInQueue determines if a message should remain in the pending queue.
func (mm *MessageManager) shouldKeepInQueue(message *Message) bool {
	message.mu.Lock()
	defer message.mu.Unlock()

	state := message.State
	retries := message.Retries

	if state == MessageStatePending || state == MessageStateSending {
		return true // Keep in pending queue
	}

	if state == MessageStateSent {
		return true // Sent but not confirmed yet, keep tracking
	}

	if state == MessageStateFailed && retries < mm.maxRetries {
		// Failed but can retry
		message.State = MessageStatePending
		return true
	}

	return false
}

// MarkMessageDelivered updates a message as delivered.
func (mm *MessageManager) MarkMessageDelivered(messageID uint32) {
	mm.mu.Lock()
	message, exists := mm.messages[messageID]
	mm.mu.Unlock()

	if exists {
		message.SetState(MessageStateDelivered)
	}
}

// MarkMessageRead updates a message as read.
func (mm *MessageManager) MarkMessageRead(messageID uint32) {
	mm.mu.Lock()
	message, exists := mm.messages[messageID]
	mm.mu.Unlock()

	if exists {
		message.SetState(MessageStateRead)
	}
}

// GetMessage retrieves a message by ID.
//
//export ToxGetMessage
func (mm *MessageManager) GetMessage(messageID uint32) (*Message, error) {
	mm.mu.Lock()
	message, exists := mm.messages[messageID]
	mm.mu.Unlock()

	if !exists {
		return nil, errors.New("message not found")
	}

	return message, nil
}

// GetMessagesByFriend retrieves all messages for a friend.
//
//export ToxGetMessagesByFriend
func (mm *MessageManager) GetMessagesByFriend(friendID uint32) []*Message {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	messages := make([]*Message, 0)
	for _, message := range mm.messages {
		if message.FriendID == friendID {
			messages = append(messages, message)
		}
	}

	return messages
}
