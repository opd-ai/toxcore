package messaging

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/limits"
	"github.com/sirupsen/logrus"
)

// ErrMessageTooLong indicates the message exceeds the maximum allowed size.
var ErrMessageTooLong = errors.New("message exceeds maximum length")

// ErrNoEncryption indicates encryption is not available due to missing key provider.
// This is a sentinel error that allows callers to explicitly handle unencrypted mode.
var ErrNoEncryption = errors.New("encryption not available: no key provider configured")

// ErrMessageEmpty indicates the message text is empty.
var ErrMessageEmpty = errors.New("message text cannot be empty")

// ErrMessageNotFound indicates the requested message does not exist.
var ErrMessageNotFound = errors.New("message not found")

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

// MessageTransport defines the interface for sending messages via the transport layer.
//
// Implementations must be safe for concurrent use from multiple goroutines.
// The transport layer is responsible for:
//   - Packet serialization and network transmission
//   - Connection management and routing to the correct friend
//   - Handling network errors and connection state
//
// The Tox instance implements this interface via SendMessagePacket in toxcore.go.
// Test implementations should return nil for successful sends or an error for failures.
//
// Error handling: Implementations should return errors for network failures.
// The MessageManager will retry failed sends up to maxRetries times.
type MessageTransport interface {
	// SendMessagePacket sends a message to the specified friend.
	// Returns nil on success, or an error if the send fails.
	// The message.Text field contains the (possibly encrypted) message content.
	SendMessagePacket(friendID uint32, message *Message) error
}

// KeyProvider defines the interface for retrieving cryptographic keys.
//
// Implementations provide access to friend public keys for encryption and
// the local private key for signing. This interface enables the messaging
// system to perform end-to-end encryption without direct coupling to key storage.
//
// The Tox instance implements this interface by wrapping the friend management
// and self identity systems. Test implementations can provide static or mock keys.
//
// Thread safety: Implementations must be safe for concurrent access.
// Key rotation: If keys are rotated, implementations should return the current
// valid key for the specified friend.
type KeyProvider interface {
	// GetFriendPublicKey retrieves the Curve25519 public key for a friend.
	// Returns an error if the friend is not found or the key is unavailable.
	GetFriendPublicKey(friendID uint32) ([32]byte, error)

	// GetSelfPrivateKey retrieves the local Curve25519 private key.
	// This key is used for ECDH key derivation during message encryption.
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
//
// MessageManager is the central coordinator for the messaging system. It manages:
//   - Message creation and ID assignment
//   - Pending message queue with retry logic
//   - Encryption via the KeyProvider interface
//   - Transport via the MessageTransport interface
//   - Delivery state callbacks
//
// # Initialization
//
// Create a MessageManager with NewMessageManager, then configure transport and
// key provider before sending messages:
//
//	mm := NewMessageManager()
//	mm.SetTransport(transportImpl)
//	mm.SetKeyProvider(keyProviderImpl)
//
// # Lifecycle
//
// The MessageManager spawns goroutines for asynchronous message delivery.
// Call Close() to gracefully shut down these goroutines before discarding
// the manager. Failure to call Close() may result in goroutine leaks.
//
// # Thread Safety
//
// MessageManager is safe for concurrent use. All public methods use internal
// locking. Multiple goroutines may call SendMessage, ProcessPendingMessages,
// and other methods concurrently.
type MessageManager struct {
	messages      map[uint32]*Message
	nextID        uint32
	pendingQueue  []*Message
	maxRetries    uint8
	retryInterval time.Duration
	transport     MessageTransport
	keyProvider   KeyProvider
	timeProvider  TimeProvider

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

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

// GetState returns the message's current delivery state.
// This method is safe for concurrent use.
func (m *Message) GetState() MessageState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.State
}

// GetRetries returns the number of retry attempts for this message.
// This method is safe for concurrent use.
func (m *Message) GetRetries() uint8 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Retries
}

// NewMessageManager creates a new message manager.
// Call Close() to gracefully shut down the manager and wait for pending goroutines.
func NewMessageManager() *MessageManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &MessageManager{
		messages:      make(map[uint32]*Message),
		pendingQueue:  make([]*Message, 0),
		maxRetries:    3,
		retryInterval: 5 * time.Second,
		nextID:        1,
		timeProvider:  DefaultTimeProvider{},
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetTimeProvider sets the time provider for deterministic testing.
func (mm *MessageManager) SetTimeProvider(tp TimeProvider) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.timeProvider = tp
}

// SetTransport sets the transport layer for sending messages.
//
// The transport must implement the MessageTransport interface. This method
// should be called during initialization before sending any messages.
// It is safe to call multiple times to change the transport.
//
// If transport is nil, messages will be encrypted and queued but not sent.
// They will remain in the pending queue until a transport is configured.
//
// Thread safety: Safe for concurrent use.
//
// Example:
//
//	mm.SetTransport(toxInstance) // Tox implements MessageTransport
func (mm *MessageManager) SetTransport(transport MessageTransport) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.transport = transport
}

// SetKeyProvider sets the key provider for message encryption.
//
// The key provider must implement the KeyProvider interface. This method
// should be called during initialization to enable end-to-end encryption.
// It is safe to call multiple times to change the key provider.
//
// If keyProvider is nil, messages will be sent unencrypted with a warning
// logged. This mode is provided for backward compatibility and testing,
// but should not be used in production deployments.
//
// Thread safety: Safe for concurrent use.
//
// Example:
//
//	mm.SetKeyProvider(toxInstance) // Tox implements KeyProvider
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
		return nil, ErrMessageEmpty
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

	// Trigger immediate send attempt with lifecycle tracking
	mm.wg.Add(1)
	go func() {
		defer mm.wg.Done()
		mm.attemptMessageSend(message)
	}()

	return message, nil
}

// ProcessPendingMessages attempts to send messages in the pending queue.
//
// This method should be called periodically in the Tox iteration loop:
//
//	for running {
//	    tox.Iterate()
//	    mm.ProcessPendingMessages()
//	    time.Sleep(tox.IterationInterval())
//	}
//
// The method performs three phases:
//  1. Retrieves a snapshot of pending messages
//  2. Attempts to send messages that are ready (respecting retry intervals)
//  3. Cleans up completed messages from the queue
//
// Thread safety: Safe for concurrent use. Multiple calls from different
// goroutines are serialized internally.
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
//
// Encryption scheme:
//  1. Retrieve recipient's Curve25519 public key via KeyProvider
//  2. Retrieve sender's Curve25519 private key via KeyProvider
//  3. Generate cryptographically secure random 24-byte nonce via crypto.GenerateNonce()
//  4. Pad plaintext to nearest standard size (256B, 1024B, 4096B) for traffic analysis resistance
//  5. Encrypt using NaCl box (XSalsa20-Poly1305) with ECDH key derivation
//  6. Encode ciphertext as base64 for safe string storage
//
// Returns ErrNoEncryption if no key provider is configured, allowing callers
// to explicitly handle the unencrypted case via errors.Is().
func (mm *MessageManager) encryptMessage(message *Message) error {
	// Check if encryption is available
	if mm.keyProvider == nil {
		// No key provider configured - return typed error for explicit handling
		logrus.WithFields(logrus.Fields{
			"friend_id":    message.FriendID,
			"message_type": message.Type,
		}).Warn("Sending message without encryption: no key provider configured")
		return ErrNoEncryption
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

	// Encode encrypted binary data as base64 to ensure safe storage in string field.
	// This prevents data corruption from null bytes or invalid UTF-8 sequences.
	message.Text = base64.StdEncoding.EncodeToString(encryptedData)

	return nil
}

// attemptMessageSend attempts to send a message through the transport layer.
// It respects context cancellation for graceful shutdown.
func (mm *MessageManager) attemptMessageSend(message *Message) {
	// Check for shutdown before starting
	select {
	case <-mm.ctx.Done():
		return
	default:
	}

	message.mu.Lock()
	message.State = MessageStateSending
	message.LastAttempt = mm.timeProvider.Now()
	message.Retries++
	message.mu.Unlock()

	// Encrypt the message (or continue unencrypted if ErrNoEncryption)
	err := mm.encryptMessage(message)
	if err != nil {
		// ErrNoEncryption is expected when no key provider is configured;
		// allow unencrypted transmission for backward compatibility
		if errors.Is(err, ErrNoEncryption) {
			// Continue with unencrypted message (warning already logged)
		} else {
			// Other encryption errors are fatal for this message
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
	}

	// Check for shutdown before sending
	select {
	case <-mm.ctx.Done():
		// Mark as pending for retry on next startup
		message.SetState(MessageStatePending)
		return
	default:
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
// For failed messages that can be retried, it explicitly transitions them
// back to pending state via retryMessage() before deciding whether to keep them.
func (mm *MessageManager) cleanupProcessedMessages() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	newPending := make([]*Message, 0, len(mm.pendingQueue))
	for _, message := range mm.pendingQueue {
		// Explicitly retry failed messages before checking if they should be kept.
		// This separates the state transition from the keep/remove decision.
		if mm.canRetryMessage(message) {
			mm.retryMessage(message)
		}
		if mm.shouldKeepInQueue(message) {
			newPending = append(newPending, message)
		}
	}
	mm.pendingQueue = newPending
}

// shouldKeepInQueue determines if a message should remain in the pending queue.
// This is a pure function with no side effects - it only inspects message state.
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
		// Failed but can retry - state transition handled by retryMessage()
		return true
	}

	return false
}

// canRetryMessage checks if a failed message is eligible for retry.
func (mm *MessageManager) canRetryMessage(message *Message) bool {
	message.mu.Lock()
	defer message.mu.Unlock()
	return message.State == MessageStateFailed && message.Retries < mm.maxRetries
}

// retryMessage explicitly transitions a failed message back to pending state.
// This method encapsulates the state machine transition for retry logic,
// maintaining clear boundaries for state changes.
func (mm *MessageManager) retryMessage(message *Message) {
	message.mu.Lock()
	defer message.mu.Unlock()
	if message.State == MessageStateFailed && message.Retries < mm.maxRetries {
		message.State = MessageStatePending
	}
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
		return nil, ErrMessageNotFound
	}

	return message, nil
}

// GetMessagesByFriend retrieves all messages for a friend.
//
//export ToxGetMessagesByFriend
func (mm *MessageManager) GetMessagesByFriend(friendID uint32) []*Message {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Count matching messages first for size hint
	count := 0
	for _, message := range mm.messages {
		if message.FriendID == friendID {
			count++
		}
	}

	// Allocate with exact capacity
	messages := make([]*Message, 0, count)
	for _, message := range mm.messages {
		if message.FriendID == friendID {
			messages = append(messages, message)
		}
	}

	return messages
}

// Close gracefully shuts down the MessageManager, canceling pending goroutines
// and waiting for them to complete. Messages that were being sent will be
// marked as pending for retry on next startup.
func (mm *MessageManager) Close() {
	mm.cancel()
	mm.wg.Wait()
}
