// Package friend implements the friend management system for the Tox protocol.
//
// This package handles friend requests, friend list management, and messaging
// between friends.
//
// Example:
//
//	f := friend.New(publicKey)
//	f.SetName("Alice")
//	f.SetStatusMessage("Available for chat")
package friend

import (
	"time"

	"github.com/sirupsen/logrus"
)

// Status represents the online/offline status of a friend.
type Status uint8

const (
	StatusNone Status = iota
	StatusAway
	StatusBusy
	StatusOnline
)

// ConnectionStatus represents the connection status to a friend.
type ConnectionStatus uint8

const (
	ConnectionNone ConnectionStatus = iota
	ConnectionTCP
	ConnectionUDP
)

// Friend represents a friend in the Tox network.
//
//export ToxFriend
type Friend struct {
	PublicKey        [32]byte
	Name             string
	StatusMessage    string
	Status           Status
	ConnectionStatus ConnectionStatus
	LastSeen         time.Time
	UserData         interface{}
	timeProvider     TimeProvider
}

// New creates a new Friend with the given public key.
//
//export ToxFriendNew
func New(publicKey [32]byte) *Friend {
	return NewWithTimeProvider(publicKey, defaultTimeProvider)
}

// NewWithTimeProvider creates a new Friend with a custom time provider.
func NewWithTimeProvider(publicKey [32]byte, tp TimeProvider) *Friend {
	if tp == nil {
		tp = defaultTimeProvider
	}

	logrus.WithFields(logrus.Fields{
		"function":   "New",
		"public_key": publicKey[:8], // Log first 8 bytes for privacy
	}).Info("Creating new friend")

	friend := &Friend{
		PublicKey:        publicKey,
		Status:           StatusNone,
		ConnectionStatus: ConnectionNone,
		LastSeen:         tp.Now(),
		timeProvider:     tp,
	}

	logrus.WithFields(logrus.Fields{
		"function":          "New",
		"public_key":        publicKey[:8],
		"status":            friend.Status,
		"connection_status": friend.ConnectionStatus,
		"last_seen":         friend.LastSeen,
	}).Info("Friend created successfully")

	return friend
}

// SetName sets the friend's name.
//
//export ToxFriendSetName
func (f *Friend) SetName(name string) {
	logrus.WithFields(logrus.Fields{
		"function":   "SetName",
		"public_key": f.PublicKey[:8],
		"old_name":   f.Name,
		"new_name":   name,
	}).Debug("Setting friend name")

	f.Name = name

	logrus.WithFields(logrus.Fields{
		"function":   "SetName",
		"public_key": f.PublicKey[:8],
		"name":       f.Name,
	}).Info("Friend name updated successfully")
}

// GetName gets the friend's name.
//
//export ToxFriendGetName
func (f *Friend) GetName() string {
	return f.Name
}

// SetStatusMessage sets the friend's status message.
//
//export ToxFriendSetStatusMessage
func (f *Friend) SetStatusMessage(message string) {
	f.StatusMessage = message
}

// GetStatusMessage gets the friend's status message.
//
//export ToxFriendGetStatusMessage
func (f *Friend) GetStatusMessage() string {
	return f.StatusMessage
}

// SetStatus sets the friend's online status.
//
//export ToxFriendSetStatus
func (f *Friend) SetStatus(status Status) {
	f.Status = status
}

// GetStatus gets the friend's online status.
//
//export ToxFriendGetStatus
func (f *Friend) GetStatus() Status {
	return f.Status
}

// SetConnectionStatus sets the friend's connection status.
//
//export ToxFriendSetConnectionStatus
func (f *Friend) SetConnectionStatus(status ConnectionStatus) {
	logrus.WithFields(logrus.Fields{
		"function":              "SetConnectionStatus",
		"public_key":            f.PublicKey[:8],
		"old_connection_status": f.ConnectionStatus,
		"new_connection_status": status,
		"previous_last_seen":    f.LastSeen,
	}).Debug("Setting friend connection status")

	f.ConnectionStatus = status
	tp := f.timeProvider
	if tp == nil {
		tp = defaultTimeProvider
	}
	f.LastSeen = tp.Now()

	logrus.WithFields(logrus.Fields{
		"function":          "SetConnectionStatus",
		"public_key":        f.PublicKey[:8],
		"connection_status": f.ConnectionStatus,
		"last_seen":         f.LastSeen,
		"is_online":         f.IsOnline(),
	}).Info("Friend connection status updated successfully")
}

// GetConnectionStatus gets the friend's connection status.
//
//export ToxFriendGetConnectionStatus
func (f *Friend) GetConnectionStatus() ConnectionStatus {
	return f.ConnectionStatus
}

// IsOnline checks if the friend is currently online.
//
//export ToxFriendIsOnline
func (f *Friend) IsOnline() bool {
	return f.ConnectionStatus != ConnectionNone
}

// LastSeenDuration returns the duration since the friend was last seen.
//
//export ToxFriendLastSeenDuration
func (f *Friend) LastSeenDuration() time.Duration {
	tp := f.timeProvider
	if tp == nil {
		tp = defaultTimeProvider
	}
	return tp.Now().Sub(f.LastSeen)
}
