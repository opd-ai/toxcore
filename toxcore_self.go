// Package toxcore implements the core functionality of the Tox protocol.
// This file contains self-management methods for the Tox instance.
package toxcore

import (
	"encoding/binary"
	"errors"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// SelfGetAddress returns the Tox ID of this instance.
//
//export ToxSelfGetAddress
func (t *Tox) SelfGetAddress() string {
	t.selfMutex.RLock()
	nospam := t.nospam
	t.selfMutex.RUnlock()

	toxID := crypto.NewToxID(t.keyPair.Public, nospam)
	return toxID.String()
}

// SelfGetNospam returns the nospam value of this instance.
//
//export ToxSelfGetNospam
func (t *Tox) SelfGetNospam() [4]byte {
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()
	return t.nospam
}

// SelfSetNospam sets the nospam value of this instance.
// This changes the Tox ID while keeping the same key pair.
//
//export ToxSelfSetNospam
func (t *Tox) SelfSetNospam(nospam [4]byte) {
	t.selfMutex.Lock()
	t.nospam = nospam
	t.selfMutex.Unlock()
}

// SelfGetPublicKey returns the public key of this instance.
//
//export ToxSelfGetPublicKey
func (t *Tox) SelfGetPublicKey() [32]byte {
	return t.keyPair.Public
}

// SelfGetSecretKey returns the secret key of this instance.
//
//export ToxSelfGetSecretKey
func (t *Tox) SelfGetSecretKey() [32]byte {
	return t.keyPair.Private
}

// SelfGetConnectionStatus returns the current connection status.
//
//export ToxSelfGetConnectionStatus
func (t *Tox) SelfGetConnectionStatus() ConnectionStatus {
	return t.connectionStatus
}

// setSelfField validates a string field's length and sets it with broadcast.
func (t *Tox) setSelfField(value string, maxLen int, errMsg string, setter, broadcast func(string)) error {
	if len([]byte(value)) > maxLen {
		return errors.New(errMsg)
	}

	t.selfMutex.Lock()
	setter(value)
	t.selfMutex.Unlock()

	broadcast(value)
	return nil
}

// SelfSetName sets the name of this Tox instance.
// The name will be broadcast to all connected friends and persisted in savedata.
// Maximum name length is 128 bytes in UTF-8 encoding.
//
//export ToxSelfSetName
func (t *Tox) SelfSetName(name string) error {
	return t.setSelfField(name, 128, "name too long: maximum 128 bytes",
		func(v string) { t.selfName = v }, t.broadcastNameUpdate)
}

// SelfGetName gets the name of this Tox instance.
// Returns the currently set name, or empty string if no name is set.
//
//export ToxSelfGetName
func (t *Tox) SelfGetName() string {
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()
	return t.selfName
}

// SelfSetStatusMessage sets the status message of this Tox instance.
// The status message will be broadcast to all connected friends and persisted in savedata.
// Maximum status message length is 1007 bytes in UTF-8 encoding.
//
//export ToxSelfSetStatusMessage
func (t *Tox) SelfSetStatusMessage(message string) error {
	return t.setSelfField(message, 1007, "status message too long: maximum 1007 bytes",
		func(v string) { t.selfStatusMsg = v }, t.broadcastStatusMessageUpdate)
}

// SelfGetStatusMessage gets the status message of this Tox instance.
// Returns the currently set status message, or empty string if no status message is set.
//
//export ToxSelfGetStatusMessage
func (t *Tox) SelfGetStatusMessage() string {
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()
	return t.selfStatusMsg
}

// GetSelfPublicKey returns the public key of this Tox instance.
func (t *Tox) GetSelfPublicKey() [32]byte {
	return t.keyPair.Public
}

// GetSelfPrivateKey returns the private key of this Tox instance.
// This is used by the message manager for message encryption.
func (t *Tox) GetSelfPrivateKey() [32]byte {
	return t.keyPair.Private
}

// getConnectedFriends returns a snapshot of currently connected friends.
// This helper avoids holding locks during packet sending operations.
func (t *Tox) getConnectedFriends() map[uint32]*Friend {
	connectedFriends := make(map[uint32]*Friend)
	t.friends.Range(func(friendID uint32, f *Friend) bool {
		if f.ConnectionStatus != ConnectionNone {
			connectedFriends[friendID] = f
		}
		return true
	})
	return connectedFriends
}

// broadcastNameUpdate sends name update packets to all connected friends.
func (t *Tox) broadcastNameUpdate(name string) {
	// Create name update packet: [TYPE(1)][FRIEND_ID(4)][NAME...]
	packet := make([]byte, 5+len(name))
	packet[0] = 0x02 // Name update packet type

	// Get list of connected friends (avoid holding lock during packet sending)
	connectedFriends := t.getConnectedFriends()

	// Send to all connected friends via transport layer
	for friendID, friend := range connectedFriends {
		// Set friend ID in packet
		binary.BigEndian.PutUint32(packet[1:5], 0) // Use 0 as placeholder for self
		copy(packet[5:], name)

		// Resolve friend's network address and send via transport
		if err := t.sendPacketToFriend(friendID, friend, packet, transport.PacketFriendNameUpdate); err != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "broadcastNameUpdate",
				"friend_id": friendID,
				"error":     err.Error(),
			}).Warn("Failed to send name update to friend")
		}
	}
}

// broadcastStatusMessageUpdate sends status message update packets to all connected friends.
func (t *Tox) broadcastStatusMessageUpdate(statusMessage string) {
	// Create status message update packet: [TYPE(1)][FRIEND_ID(4)][STATUS_MESSAGE...]
	packet := make([]byte, 5+len(statusMessage))
	packet[0] = 0x03 // Status message update packet type

	// Get list of connected friends (avoid holding lock during packet sending)
	connectedFriends := t.getConnectedFriends()

	// Send to all connected friends via transport layer
	for friendID, friend := range connectedFriends {
		// Set friend ID in packet
		binary.BigEndian.PutUint32(packet[1:5], 0) // Use 0 as placeholder for self
		copy(packet[5:], statusMessage)

		// Resolve friend's network address and send via transport
		if err := t.sendPacketToFriend(friendID, friend, packet, transport.PacketFriendStatusMessageUpdate); err != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "broadcastStatusMessageUpdate",
				"friend_id": friendID,
				"error":     err.Error(),
			}).Warn("Failed to send status message update to friend")
		}
	}
}
