// Package toxcore implements the core functionality of the Tox protocol.
//
// This file contains messaging-related methods for the Tox struct,
// including message sending, receiving, validation, and processing.
package toxcore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/opd-ai/toxcore/async"
	"github.com/opd-ai/toxcore/messaging"
	"github.com/opd-ai/toxcore/transport"
)

// SendFriendMessage sends a message to a friend.
// Automatically determines delivery method (real-time or async) based on friend's connection status.
//
// The message is limited to 1372 bytes (Tox protocol limit).
// Returns an error if the friend is not found or message cannot be sent.
//
//export ToxSendFriendMessage
func (t *Tox) SendFriendMessage(friendID uint32, message string, messageType ...MessageType) error {
	// Validate message input atomically within the send operation to prevent race conditions
	if len(message) == 0 {
		return errors.New("message cannot be empty")
	}

	// Create immutable copy of message length to prevent TOCTOU race conditions
	messageBytes := []byte(message)
	if len(messageBytes) > 1372 {
		return errors.New("message too long: maximum 1372 bytes")
	}

	msgType := t.determineMessageType(messageType...)

	if err := t.validateFriendStatus(friendID); err != nil {
		return err
	}

	return t.sendMessageToManager(friendID, message, msgType)
}

// isValidMessage checks if the provided message meets all required criteria.
// Returns true if the message is valid, false otherwise.
func (t *Tox) isValidMessage(message string) bool {
	if len(message) == 0 {
		return false // Empty messages are not valid
	}
	if len([]byte(message)) > 1372 { // Tox protocol message length limit
		return false // Oversized messages are not valid
	}
	return true
}

// validateMessageInput checks if the provided message meets all required criteria.
func (t *Tox) validateMessageInput(message string) error {
	if !t.isValidMessage(message) {
		if len(message) == 0 {
			return errors.New("message cannot be empty")
		}
		return errors.New("message too long: maximum 1372 bytes")
	}
	return nil
}

// determineMessageType resolves the message type from variadic parameters with default fallback.
func (t *Tox) determineMessageType(messageType ...MessageType) MessageType {
	msgType := MessageTypeNormal
	if len(messageType) > 0 {
		msgType = messageType[0]
	}
	return msgType
}

// validateFriendStatus verifies the friend exists and determines delivery method.
func (t *Tox) validateFriendStatus(friendID uint32) error {
	if !t.friends.Exists(friendID) {
		return errors.New("friend not found")
	}

	// Friend exists - delivery method will be determined in sendMessageToManager
	return nil
}

// sendMessageToManager creates and sends the message through the appropriate system.
func (t *Tox) sendMessageToManager(friendID uint32, message string, msgType MessageType) error {
	friend, err := t.validateAndRetrieveFriend(friendID)
	if err != nil {
		return err
	}

	if friend.ConnectionStatus != ConnectionNone {
		return t.sendRealTimeMessage(friendID, message, msgType)
	} else {
		return t.sendAsyncMessage(friend.PublicKey, message, msgType)
	}
}

// validateAndRetrieveFriend validates the friend ID and retrieves the friend information.
func (t *Tox) validateAndRetrieveFriend(friendID uint32) (*Friend, error) {
	f := t.friends.Get(friendID)
	if f == nil {
		return nil, errors.New("friend not found")
	}

	return f, nil
}

// sendRealTimeMessage sends a message to an online friend using the message manager.
func (t *Tox) sendRealTimeMessage(friendID uint32, message string, msgType MessageType) error {
	// Friend is online - use real-time messaging
	t.messageManagerMu.RLock()
	mm := t.messageManager
	t.messageManagerMu.RUnlock()
	if mm != nil {
		// Convert toxcore.MessageType to messaging.MessageType
		messagingMsgType := messaging.MessageType(msgType)
		// SendMessage returns (Message, error) but we only need to verify success.
		// The Message object contains metadata (ID, timestamp, status) that is useful
		// for tracking delivery confirmations, but the caller of sendRealTimeMessage
		// only needs to know if the send succeeded. The message manager internally
		// handles delivery tracking and callbacks.
		_, err := mm.SendMessage(friendID, message, messagingMsgType)
		if err != nil {
			return err
		}
	}
	return nil
}

// sendAsyncMessage sends a message to an offline friend using the async manager.
func (t *Tox) sendAsyncMessage(publicKey [32]byte, message string, msgType MessageType) error {
	// Friend is offline - use async messaging
	if t.asyncManager == nil {
		return fmt.Errorf("friend is not connected and async messaging is unavailable")
	}

	// Convert toxcore.MessageType to async.MessageType
	asyncMsgType := async.MessageType(msgType)
	err := t.asyncManager.SendAsyncMessage(publicKey, message, asyncMsgType)
	if err != nil {
		// Provide clearer error context for common async messaging issues
		if strings.Contains(err.Error(), "no pre-keys available") {
			return fmt.Errorf("friend is not connected and secure messaging keys are not available. %v", err)
		}
		return err
	}
	return nil
}

// doMessageProcessing performs periodic message processing tasks.
// This handles message queue processing and async message delivery.
func (t *Tox) doMessageProcessing() {
	t.messageManagerMu.RLock()
	mm := t.messageManager
	t.messageManagerMu.RUnlock()
	if mm == nil {
		return
	}

	// Process pending messages with retry logic
	// The messageManager handles delivery tracking, retries, and confirmations
	mm.ProcessPendingMessages()

	// Flush any pending async messages for friends whose pre-key exchange has now completed.
	if t.asyncManager != nil {
		t.asyncManager.ProcessPendingDeliveries()
	}
}

// dispatchFriendMessage dispatches an incoming friend message to the appropriate callback(s).
// This method ensures both simple and detailed callbacks are called if they are registered.
func (t *Tox) dispatchFriendMessage(friendID uint32, message string, messageType MessageType) {
	t.callbackMu.RLock()
	simpleCb := t.simpleFriendMessageCallback
	detailedCb := t.friendMessageCallback
	t.callbackMu.RUnlock()

	if simpleCb != nil {
		simpleCb(friendID, message)
	}
	if detailedCb != nil {
		detailedCb(friendID, message, messageType)
	}
}

// receiveFriendMessage processes incoming messages from friends.
// This method is automatically called by the network layer when message packets are received
// and is integrated with the transport system for real-time message handling.
//
//export ToxReceiveFriendMessage
func (t *Tox) receiveFriendMessage(friendID uint32, message string, messageType MessageType) {
	// Basic packet validation using shared validation logic
	if !t.isValidMessage(message) {
		return // Ignore invalid messages (empty or oversized)
	}

	// Verify the friend exists
	if !t.friends.Exists(friendID) {
		return // Ignore messages from unknown friends
	}

	// Dispatch to registered callbacks
	t.dispatchFriendMessage(friendID, message, messageType)
}

// receiveFriendStatusMessageUpdate processes incoming friend status message update packets
func (t *Tox) receiveFriendStatusMessageUpdate(friendID uint32, statusMessage string) {
	t.updateFriendField(
		friendID,
		statusMessage,
		1007, // Max status message length for Tox protocol
		func(f *Friend, v string) { f.StatusMessage = v },
		t.invokeFriendStatusMessageCallback,
	)
}

// handleFriendMessagePacket processes incoming friend message packets from the transport layer
func (t *Tox) handleFriendMessagePacket(packet *transport.Packet, senderAddr net.Addr) error {
	// Delegate to the existing packet processing infrastructure
	return t.processIncomingPacket(packet.Data, senderAddr)
}

// processFriendMessagePacket handles incoming friend message packets.
func (t *Tox) processFriendMessagePacket(packet []byte) error {
	if len(packet) < 6 {
		return errors.New("friend message packet too small")
	}

	friendID := binary.BigEndian.Uint32(packet[1:5])
	messageType := MessageType(packet[5])
	message := string(packet[6:])

	t.receiveFriendMessage(friendID, message, messageType)
	return nil
}

// processFriendStatusMessageUpdatePacket handles incoming friend status message update packets.
func (t *Tox) processFriendStatusMessageUpdatePacket(packet []byte) error {
	if len(packet) < 5 {
		return errors.New("friend status message update packet too small")
	}

	friendID := binary.BigEndian.Uint32(packet[1:5])
	statusMessage := string(packet[5:])

	t.receiveFriendStatusMessageUpdate(friendID, statusMessage)
	return nil
}

// SendMessagePacket sends a message packet to a friend using the transport layer.
// This is a low-level method used by the message manager for actual packet delivery.
func (t *Tox) SendMessagePacket(friendID uint32, message *messaging.Message) error {
	f := t.friends.Get(friendID)
	if f == nil {
		return errors.New("friend not found")
	}

	// Build packet: [TYPE(1)][FRIEND_ID(4)][MESSAGE_TYPE(1)][MESSAGE...]
	msgText := message.GetText()
	packet := make([]byte, 6+len(msgText))
	packet[0] = 0x01 // Friend message packet type
	binary.BigEndian.PutUint32(packet[1:5], friendID)
	packet[5] = byte(message.Type)
	copy(packet[6:], msgText)

	// Get friend's network address from DHT
	friendAddr, err := t.resolveFriendAddress(f)
	if err != nil {
		return fmt.Errorf("failed to resolve friend address: %w", err)
	}

	// Send through UDP transport if available
	if t.udpTransport != nil {
		transportPacket := &transport.Packet{
			PacketType: transport.PacketFriendMessage,
			Data:       packet,
		}
		return t.udpTransport.Send(transportPacket, friendAddr)
	}

	return errors.New("transport not available")
}

// FriendSendMessage sends a message to a friend and returns a message ID.
// This is the API that matches the c-toxcore interface for message tracking.
//
//export ToxFriendSendMessage
func (t *Tox) FriendSendMessage(friendID uint32, message string, messageType MessageType) (uint32, error) {
	// Validate message
	if err := t.validateMessageInput(message); err != nil {
		return 0, err
	}

	// Use the main SendFriendMessage method which handles async vs real-time delivery
	if err := t.SendFriendMessage(friendID, message, messageType); err != nil {
		return 0, err
	}

	// Return a message ID for tracking (simple incrementing counter for now)
	// In a full implementation, this would be managed by the message manager
	return t.nextMessageID(), nil
}

// nextMessageID generates a unique message ID for tracking sent messages.
func (t *Tox) nextMessageID() uint32 {
	t.messageIDMu.Lock()
	defer t.messageIDMu.Unlock()
	t.lastMessageID++
	return t.lastMessageID
}
