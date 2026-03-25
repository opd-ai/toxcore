// Package toxcore implements the core functionality of the Tox protocol.
// This file contains callback type definitions and registration methods.
package toxcore

import (
	"github.com/opd-ai/toxcore/async"
)

// FriendRequestCallback is called when a friend request is received.
type FriendRequestCallback func(publicKey [32]byte, message string)

// SimpleFriendMessageCallback is called when a message is received from a friend.
// This matches the documented API in README.md for simple use cases.
type SimpleFriendMessageCallback func(friendID uint32, message string)

// FriendStatusCallback is called when a friend's status changes.
type FriendStatusCallback func(friendID uint32, status FriendStatus)

// ConnectionStatusCallback is called when the connection status changes.
type ConnectionStatusCallback func(status ConnectionStatus)

// FriendConnectionStatusCallback is called when a friend's connection status changes.
type FriendConnectionStatusCallback func(friendID uint32, connectionStatus ConnectionStatus)

// FriendStatusChangeCallback is called when a friend comes online or goes offline.
type FriendStatusChangeCallback func(friendPK [32]byte, online bool)

// FriendMessageCallback is called when a message is received from a friend.
type FriendMessageCallback func(friendID uint32, message string, messageType MessageType)

// OnFriendRequest sets the callback for friend requests.
//
//export ToxOnFriendRequest
func (t *Tox) OnFriendRequest(callback FriendRequestCallback) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.friendRequestCallback = callback
}

// OnFriendMessage sets the callback for friend messages using the simplified API.
// This matches the documented API in README.md: func(friendID uint32, message string)
//
//export ToxOnFriendMessage
func (t *Tox) OnFriendMessage(callback SimpleFriendMessageCallback) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.simpleFriendMessageCallback = callback
}

// OnFriendMessageDetailed sets the callback for friend messages with message type.
// Use this for advanced scenarios where you need access to the message type.
//
//export ToxOnFriendMessageDetailed
func (t *Tox) OnFriendMessageDetailed(callback FriendMessageCallback) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.friendMessageCallback = callback
}

// OnFriendStatus sets the callback for friend status changes.
//
//export ToxOnFriendStatus
func (t *Tox) OnFriendStatus(callback FriendStatusCallback) {
	t.callbackMu.Lock()
	t.friendStatusCallback = callback
	t.callbackMu.Unlock()
	// Set up async message handler to receive offline messages
	if t.asyncManager != nil {
		t.asyncManager.SetAsyncMessageHandler(func(senderPK [32]byte, message string, messageType async.MessageType) {
			// Find friend ID from public key
			friendID := t.findFriendByPublicKey(senderPK)
			if friendID != 0 {
				// Convert async.MessageType to toxcore.MessageType and trigger callback
				toxMsgType := MessageType(messageType)
				t.callbackMu.RLock()
				cb := t.friendMessageCallback
				t.callbackMu.RUnlock()
				if cb != nil {
					cb(friendID, message, toxMsgType)
				}
			}
		})
	}
}

// OnConnectionStatus sets the callback for connection status changes.
//
//export ToxOnConnectionStatus
func (t *Tox) OnConnectionStatus(callback ConnectionStatusCallback) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.connectionStatusCallback = callback
}

// OnFriendConnectionStatus sets the callback for friend connection status changes.
// This is called whenever a friend's connection status changes between None, UDP, or TCP.
//
//export ToxOnFriendConnectionStatus
func (t *Tox) OnFriendConnectionStatus(callback FriendConnectionStatusCallback) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.friendConnectionStatusCallback = callback
}

// OnFriendStatusChange sets the callback for friend online/offline status changes.
// This is called when a friend transitions between online (connected) and offline (not connected).
// The callback receives the friend's public key and a boolean indicating if they are online.
//
//export ToxOnFriendStatusChange
func (t *Tox) OnFriendStatusChange(callback FriendStatusChangeCallback) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.friendStatusChangeCallback = callback
}

// OnAsyncMessage sets the callback for async messages (offline messages).
// This provides access to the async messaging system through the main Tox interface.
//
//export ToxOnAsyncMessage
func (t *Tox) OnAsyncMessage(callback func(senderPK [32]byte, message string, messageType async.MessageType)) {
	if t.asyncManager != nil {
		t.asyncManager.SetAsyncMessageHandler(callback)
	}
}

// OnFileRecv sets the callback for file receive events.
//
//export ToxOnFileRecv
func (t *Tox) OnFileRecv(callback func(friendID, fileID, kind uint32, fileSize uint64, filename string)) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.fileRecvCallback = callback
}

// OnFileRecvChunk sets the callback for file chunk receive events.
//
//export ToxOnFileRecvChunk
func (t *Tox) OnFileRecvChunk(callback func(friendID, fileID uint32, position uint64, data []byte)) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.fileRecvChunkCallback = callback
}

// OnFileChunkRequest sets the callback for file chunk request events.
//
//export ToxOnFileChunkRequest
func (t *Tox) OnFileChunkRequest(callback func(friendID, fileID uint32, position uint64, length int)) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.fileChunkRequestCallback = callback
}

// OnFriendName sets the callback for friend name changes.
//
//export ToxOnFriendName
func (t *Tox) OnFriendName(callback func(friendID uint32, name string)) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.friendNameCallback = callback
}

// OnFriendStatusMessage sets the callback for friend status message changes.
//
//export ToxOnFriendStatusMessage
func (t *Tox) OnFriendStatusMessage(callback func(friendID uint32, statusMessage string)) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.friendStatusMessageCallback = callback
}

// OnFriendTyping sets the callback for friend typing notifications.
//
//export ToxOnFriendTyping
func (t *Tox) OnFriendTyping(callback func(friendID uint32, isTyping bool)) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.friendTypingCallback = callback
}

// FriendDeletedCallback is called when a friend is deleted from the friend list.
// This allows subsystems like ToxAV to clean up resources (e.g., active calls).
type FriendDeletedCallback func(friendID uint32)

// OnFriendDeleted sets the callback for friend deletion events.
// This is useful for cleaning up related resources (e.g., active calls, pending transfers).
//
//export ToxOnFriendDeleted
func (t *Tox) OnFriendDeleted(callback FriendDeletedCallback) {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()
	t.friendDeletedCallback = callback
}
