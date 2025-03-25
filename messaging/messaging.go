// Package messaging implements the messaging system for the Tox protocol.
//
// This package handles sending and receiving messages between Tox users,
// including message formatting, delivery confirmation, and offline messaging.
//
// Example:
//  msg := messaging.NewMessage(friendID, "Hello, world!")
//  if err := msg.Send(); err != nil {
//      log.Fatal(err)
//  }
package messaging

import (
	"errors"
	"sync"
	"time"
)

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

// Message represents a Tox message.
//
//export ToxMessage
type Message struct {
	ID           uint32
	FriendID     uint32
	Type         MessageType
	Text         string
	Timestamp    time.Time
	State        MessageState
	Retries      uint8
	LastAttempt  time.Time
	
	deliveryCallback DeliveryCallback
	
	mu             sync.Mutex
}

// MessageManager handles message sending, receiving, and tracking.
type MessageManager struct {
	messages       map[uint32]*Message
	nextID         uint32
	pendingQueue   []*Message
	maxRetries     uint8
	retryInterval  time.Duration
	
	mu             sync.Mutex
}

// NewMessage creates a new message.
//
//export ToxMessageNew
func NewMessage(friendID uint32, text string, messageType MessageType) *Message {
	return &Message{
		FriendID:    friendID,
		Type:        messageType,
		Text:        text,
		Timestamp:   time.Now(),
		State:       MessageStatePending,
		Retries:     0,
		LastAttempt: time.Time{}, // Zero time
	}
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
		nextID:        1,
		pendingQueue:  make([]*Message, 0),
		maxRetries:    5,
		retryInterval: 30 * time.Second,
	}
}

// SendMessage sends a message to a friend.
//
//export ToxSendMessage
func (mm *MessageManager) SendMessage(friendID uint32, text string, messageType MessageType) (*Message, error) {
	if len(text) == 0 {
		return nil, errors.New("message text cannot be empty")
	}
	
	mm.mu.Lock()
	defer mm.mu.Unlock()
	
	// Create a new message
	message := NewMessage(friendID, text, messageType)
	message.ID = mm.nextID
	mm.nextID++
	
	// Store the message
	mm.messages[message.ID] = message
	
	// Add to pending queue
	mm.pendingQueue = append(mm.pendingQueue, message)
	
	// In a real implementation, this would trigger the actual send
	// through the transport layer
	
	return message, nil
}

// ProcessPendingMessages attempts to send pending messages.
func (mm *MessageManager) ProcessPendingMessages() {
	mm.mu.Lock()
	pending := make([]*Message, len(mm.pendingQueue))
	copy