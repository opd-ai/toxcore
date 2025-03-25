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
}

// New creates a new Friend with the given public key.
//
//export ToxFriendNew
func New(publicKey [32]byte) *Friend {
	return &Friend{
		PublicKey:        publicKey,
		Status:           StatusNone,
		ConnectionStatus: ConnectionNone,
		LastSeen:         time.Now(),
	}
}

// SetName sets the friend's name.
//
//export ToxFriendSetName
func (f *Friend) SetName(name string) {
	f.Name = name
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
	f.ConnectionStatus = status
	f.LastSeen = time.Now()
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
	return time.Since(f.LastSeen)
}
