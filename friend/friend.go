// Package friend implements the friend management system for the Tox protocol.
//
// This package handles friend requests, friend list management, and messaging
// between friends.
//
// Example:
//
//	f := friend.New(publicKey)
//	if err := f.SetName("Alice"); err != nil {
//	    log.Fatal(err) // Name exceeds MaxNameLength
//	}
//	if err := f.SetStatusMessage("Available for chat"); err != nil {
//	    log.Fatal(err) // Message exceeds MaxStatusMessageLength
//	}
//
// Note: FriendInfo is used instead of Friend to avoid conflicts with toxcore.Friend.
package friend

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// FriendStatus represents the online/offline status of a friend.
// Named FriendStatus (not Status) to avoid conflicts with similar status types
// in other packages and to match toxcore.go naming conventions.
type FriendStatus uint8

const (
	FriendStatusNone FriendStatus = iota
	FriendStatusAway
	FriendStatusBusy
	FriendStatusOnline
)

// ConnectionStatus represents the connection status to a friend.
type ConnectionStatus uint8

const (
	ConnectionNone ConnectionStatus = iota
	ConnectionTCP
	ConnectionUDP
)

// FriendInfo represents a friend in the Tox network.
// NOTE: Named FriendInfo (not Friend) to avoid conflicts with toxcore.Friend type.
//
//export ToxFriendInfo
type FriendInfo struct {
	PublicKey        [32]byte
	Name             string
	StatusMessage    string
	Status           FriendStatus
	ConnectionStatus ConnectionStatus
	LastSeen         time.Time
	UserData         interface{}
	timeProvider     TimeProvider
}

// New creates a new FriendInfo with the given public key.
//
//export ToxFriendInfoNew
func New(publicKey [32]byte) *FriendInfo {
	return NewWithTimeProvider(publicKey, defaultTimeProvider)
}

// NewWithTimeProvider creates a new FriendInfo with a custom time provider.
func NewWithTimeProvider(publicKey [32]byte, tp TimeProvider) *FriendInfo {
	if tp == nil {
		tp = defaultTimeProvider
	}

	logrus.WithFields(logrus.Fields{
		"function":   "New",
		"public_key": publicKey[:8], // Log first 8 bytes for privacy
	}).Info("Creating new friend")

	friend := &FriendInfo{
		PublicKey:        publicKey,
		Status:           FriendStatusNone,
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
	}).Info("FriendInfo created successfully")

	return friend
}

// SetName sets the friend's name.
// Returns an error if the name exceeds MaxNameLength (128 bytes).
//
//export ToxFriendInfoSetName
func (f *FriendInfo) SetName(name string) error {
	if len(name) > MaxNameLength {
		return fmt.Errorf("%w: got %d bytes", ErrNameTooLong, len(name))
	}

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

	return nil
}

// GetName gets the friend's name.
//
//export ToxFriendInfoGetName
func (f *FriendInfo) GetName() string {
	return f.Name
}

// SetStatusMessage sets the friend's status message.
// Returns an error if the message exceeds MaxStatusMessageLength (1007 bytes).
//
//export ToxFriendInfoSetStatusMessage
func (f *FriendInfo) SetStatusMessage(message string) error {
	if len(message) > MaxStatusMessageLength {
		return fmt.Errorf("%w: got %d bytes", ErrStatusMessageTooLong, len(message))
	}

	logrus.WithFields(logrus.Fields{
		"function":           "SetStatusMessage",
		"public_key":         f.PublicKey[:8],
		"old_status_message": f.StatusMessage,
		"new_status_message": message,
	}).Debug("Setting friend status message")

	f.StatusMessage = message

	logrus.WithFields(logrus.Fields{
		"function":       "SetStatusMessage",
		"public_key":     f.PublicKey[:8],
		"status_message": f.StatusMessage,
	}).Info("Friend status message updated successfully")

	return nil
}

// GetStatusMessage gets the friend's status message.
//
//export ToxFriendInfoGetStatusMessage
func (f *FriendInfo) GetStatusMessage() string {
	return f.StatusMessage
}

// SetStatus sets the friend's online status.
//
//export ToxFriendInfoSetStatus
func (f *FriendInfo) SetStatus(status FriendStatus) {
	f.Status = status
}

// GetStatus gets the friend's online status.
//
//export ToxFriendInfoGetStatus
func (f *FriendInfo) GetStatus() FriendStatus {
	return f.Status
}

// SetConnectionStatus sets the friend's connection status.
//
//export ToxFriendInfoSetConnectionStatus
func (f *FriendInfo) SetConnectionStatus(status ConnectionStatus) {
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
//export ToxFriendInfoGetConnectionStatus
func (f *FriendInfo) GetConnectionStatus() ConnectionStatus {
	return f.ConnectionStatus
}

// IsOnline checks if the friend is currently online.
//
//export ToxFriendInfoIsOnline
func (f *FriendInfo) IsOnline() bool {
	return f.ConnectionStatus != ConnectionNone
}

// LastSeenDuration returns the duration since the friend was last seen.
//
//export ToxFriendInfoLastSeenDuration
func (f *FriendInfo) LastSeenDuration() time.Duration {
	tp := f.timeProvider
	if tp == nil {
		tp = defaultTimeProvider
	}
	return tp.Now().Sub(f.LastSeen)
}

// friendInfoSerialized is the internal representation for JSON serialization.
// This excludes non-serializable fields like UserData and timeProvider.
type friendInfoSerialized struct {
	PublicKey        [32]byte         `json:"public_key"`
	Name             string           `json:"name"`
	StatusMessage    string           `json:"status_message"`
	Status           FriendStatus           `json:"status"`
	ConnectionStatus ConnectionStatus `json:"connection_status"`
	LastSeen         time.Time        `json:"last_seen"`
}

// Marshal serializes the FriendInfo to a JSON byte slice.
// This enables persistence of friend state for savedata integration.
// Note: UserData is not serialized as it may contain non-JSON-serializable types.
//
//export ToxFriendInfoMarshal
func (f *FriendInfo) Marshal() ([]byte, error) {
	serialized := friendInfoSerialized{
		PublicKey:        f.PublicKey,
		Name:             f.Name,
		StatusMessage:    f.StatusMessage,
		Status:           f.Status,
		ConnectionStatus: f.ConnectionStatus,
		LastSeen:         f.LastSeen,
	}

	data, err := json.Marshal(serialized)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "FriendInfo.Marshal",
			"public_key": fmt.Sprintf("%x", f.PublicKey[:8]),
			"error":      err.Error(),
		}).Error("Failed to marshal FriendInfo")
		return nil, fmt.Errorf("failed to marshal FriendInfo: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "FriendInfo.Marshal",
		"public_key": fmt.Sprintf("%x", f.PublicKey[:8]),
		"data_size":  len(data),
	}).Debug("FriendInfo marshaled successfully")

	return data, nil
}

// Unmarshal deserializes JSON data into this FriendInfo.
// The timeProvider is preserved if already set, otherwise defaults to system clock.
//
//export ToxFriendInfoUnmarshal
func (f *FriendInfo) Unmarshal(data []byte) error {
	var serialized friendInfoSerialized
	if err := json.Unmarshal(data, &serialized); err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "FriendInfo.Unmarshal",
			"data_size": len(data),
			"error":     err.Error(),
		}).Error("Failed to unmarshal FriendInfo")
		return fmt.Errorf("failed to unmarshal FriendInfo: %w", err)
	}

	f.PublicKey = serialized.PublicKey
	f.Name = serialized.Name
	f.StatusMessage = serialized.StatusMessage
	f.Status = serialized.Status
	f.ConnectionStatus = serialized.ConnectionStatus
	f.LastSeen = serialized.LastSeen

	// Preserve existing timeProvider or use default
	if f.timeProvider == nil {
		f.timeProvider = defaultTimeProvider
	}

	logrus.WithFields(logrus.Fields{
		"function":   "FriendInfo.Unmarshal",
		"public_key": fmt.Sprintf("%x", f.PublicKey[:8]),
		"name":       f.Name,
	}).Debug("FriendInfo unmarshaled successfully")

	return nil
}

// UnmarshalFriendInfo creates a new FriendInfo from JSON data.
// This is a convenience function for creating a FriendInfo from serialized data.
//
//export ToxFriendInfoUnmarshalNew
func UnmarshalFriendInfo(data []byte) (*FriendInfo, error) {
	f := &FriendInfo{
		timeProvider: defaultTimeProvider,
	}
	if err := f.Unmarshal(data); err != nil {
		return nil, err
	}
	return f, nil
}
