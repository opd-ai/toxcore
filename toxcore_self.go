// Package toxcore implements the core functionality of the Tox protocol.
// This file contains self-management methods for the Tox instance.
package toxcore

import (
	"errors"
	"net"

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
	publicKey := t.keyPair.Public
	t.selfMutex.RUnlock()

	toxID := crypto.NewToxID(publicKey, nospam)
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
	t.selfMutex.RLock()
	pub := t.keyPair.Public
	t.selfMutex.RUnlock()
	return pub
}

// SelfGetSecretKey returns the secret key of this instance.
//
//export ToxSelfGetSecretKey
func (t *Tox) SelfGetSecretKey() [32]byte {
	t.selfMutex.RLock()
	priv := t.keyPair.Private
	t.selfMutex.RUnlock()
	return priv
}

// SelfGetConnectionStatus returns the current connection status.
//
//export ToxSelfGetConnectionStatus
func (t *Tox) SelfGetConnectionStatus() ConnectionStatus {
	t.selfMutex.RLock()
	status := t.connectionStatus
	t.selfMutex.RUnlock()
	return status
}

// SelfSetStatus sets the local user's status.
//
//export ToxSelfSetStatus
func (t *Tox) SelfSetStatus(status UserStatus) error {
	if status < UserStatusNone || status > UserStatusBusy {
		return errors.New("invalid user status")
	}
	t.selfMutex.Lock()
	t.selfStatus = status
	t.selfMutex.Unlock()
	return nil
}

// SelfGetStatus returns the local user's status.
//
//export ToxSelfGetStatus
func (t *Tox) SelfGetStatus() UserStatus {
	t.selfMutex.RLock()
	status := t.selfStatus
	t.selfMutex.RUnlock()
	return status
}

// updateConnectionStatus updates the connection status based on bootstrap state.
// This should be called regularly from the iteration loop.
func (t *Tox) updateConnectionStatus() {
	t.bootstrapManagerMu.RLock()
	bootstrapManager := t.bootstrapManager
	t.bootstrapManagerMu.RUnlock()

	if bootstrapManager == nil {
		return
	}

	// Update status based on bootstrap completion
	newStatus := ConnectionNone
	if bootstrapManager.IsBootstrapped() {
		newStatus = ConnectionUDP
	}

	t.selfMutex.Lock()
	oldStatus := t.connectionStatus
	if newStatus == oldStatus {
		t.selfMutex.Unlock()
		return
	}

	t.connectionStatus = newStatus
	callback := t.connectionStatusCallback
	t.selfMutex.Unlock()

	// Trigger callback if registered
	if callback != nil {
		callback(newStatus)
	}
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
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()
	return t.keyPair.Public
}

// GetSelfUDPAddr returns the local address the Tox UDP socket is bound to,
// including the OS-assigned port when StartPort/EndPort were both 0 (M-BOOT-1).
// Returns nil when UDP is disabled.
func (t *Tox) GetSelfUDPAddr() net.Addr {
	return t.selfAddress
}

// SafetyNumber derives a human-readable, versioned key fingerprint for the
// connection between this Tox instance and a peer identified by peerPK.
//
// The result is 12 groups of 5 decimal digits (60 digits total) and is
// suitable for out-of-band comparison to detect man-in-the-middle attacks.
//
// ⚠ SECURITY: Both parties MUST compare their safety numbers through an
// independent channel (e.g. a voice call or in-person) at least once per
// contact before trusting the connection. The fingerprint provides MITM
// detection only when verified through a channel the attacker cannot intercept.
//
//export ToxSafetyNumber
func (t *Tox) SafetyNumber(peerPK [32]byte) string {
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()
	return crypto.SafetyNumber(t.keyPair.Public, peerPK)
}

// FriendSignKeyState returns the trusted Ed25519 signing key recorded for a
// friend and whether any key has been seen yet.
//
// This key is captured from the first async pre-key exchange with the peer
// (Trust-On-First-Use). A subsequent call to OnFriendKeyChange will fire if
// the peer ever presents a different signing key.
//
// Returns (zero key, false) when no key has been observed for friendPK.
//
//export ToxFriendSignKeyState
func (t *Tox) FriendSignKeyState(friendPK [32]byte) (trustedKey [32]byte, known bool) {
	if t.asyncManager == nil {
		return [32]byte{}, false
	}
	return t.asyncManager.FriendSignKeyState(friendPK)
}

// MarkFriendSignKeyVerified acknowledges a TOFU key-change alarm for friendPK,
// accepting newKey as the trusted Ed25519 signing key for that friend.
//
// Call this after the user has confirmed the peer's identity via an out-of-band
// mechanism such as comparing safety numbers. Until this is called, async
// pre-key exchanges carrying the new key will be rejected.
//
//export ToxMarkFriendSignKeyVerified
func (t *Tox) MarkFriendSignKeyVerified(friendPK, newKey [32]byte) {
	if t.asyncManager == nil {
		return
	}
	t.asyncManager.AcceptNewSignKey(friendPK, newKey)
}

// GetSelfPrivateKey returns the private key of this Tox instance.
// This is used by the message manager for message encryption.
func (t *Tox) GetSelfPrivateKey() [32]byte {
	t.selfMutex.RLock()
	priv := t.keyPair.Private
	t.selfMutex.RUnlock()
	return priv
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
	// Create name update packet: [TYPE(1)][SENDER_PK(32)][NAME...]
	packet := make([]byte, 33+len(name))
	packet[0] = 0x02 // Name update packet type

	// Embed our own public key so the receiver can identify us
	selfPK := t.SelfGetPublicKey()
	copy(packet[1:33], selfPK[:])

	// Get list of connected friends (avoid holding lock during packet sending)
	connectedFriends := t.getConnectedFriends()

	// Send to all connected friends via transport layer
	for friendID, friend := range connectedFriends {
		copy(packet[33:], name)

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
	// Create status message update packet: [TYPE(1)][SENDER_PK(32)][STATUS_MESSAGE...]
	packet := make([]byte, 33+len(statusMessage))
	packet[0] = 0x03 // Status message update packet type

	// Embed our own public key so the receiver can identify us
	selfPK := t.SelfGetPublicKey()
	copy(packet[1:33], selfPK[:])

	// Get list of connected friends (avoid holding lock during packet sending)
	connectedFriends := t.getConnectedFriends()

	// Send to all connected friends via transport layer
	for friendID, friend := range connectedFriends {
		copy(packet[33:], statusMessage)

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
