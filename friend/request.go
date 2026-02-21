package friend

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/sirupsen/logrus"
)

// Tox protocol length limits per official specification.
const (
	// MaxNameLength is the maximum length for a friend's name (128 bytes per Tox spec).
	MaxNameLength = 128

	// MaxStatusMessageLength is the maximum length for a status message (1007 bytes per Tox spec).
	MaxStatusMessageLength = 1007

	// MaxFriendRequestMessageLength is the maximum length for a friend request message (1016 bytes per Tox spec).
	MaxFriendRequestMessageLength = 1016
)

// Input validation errors.
var (
	// ErrNameTooLong is returned when a name exceeds MaxNameLength bytes.
	ErrNameTooLong = errors.New("name exceeds maximum length of 128 bytes")

	// ErrStatusMessageTooLong is returned when a status message exceeds MaxStatusMessageLength bytes.
	ErrStatusMessageTooLong = errors.New("status message exceeds maximum length of 1007 bytes")

	// ErrFriendRequestMessageTooLong is returned when a friend request message exceeds MaxFriendRequestMessageLength bytes.
	ErrFriendRequestMessageTooLong = errors.New("friend request message exceeds maximum length of 1016 bytes")
)

// TimeProvider is an interface for getting the current time.
// This allows for deterministic testing and simulation environments.
type TimeProvider interface {
	Now() time.Time
}

// DefaultTimeProvider uses the system clock.
type DefaultTimeProvider struct{}

// Now returns the current system time.
func (DefaultTimeProvider) Now() time.Time {
	return time.Now()
}

// defaultTimeProvider is the package-level default time provider.
var defaultTimeProvider TimeProvider = DefaultTimeProvider{}

// Request represents a friend request.
//
//export ToxFriendRequest
type Request struct {
	SenderPublicKey [32]byte
	Message         string
	Nonce           [24]byte
	Timestamp       time.Time
	Handled         bool
	timeProvider    TimeProvider
}

// NewRequest creates a new outgoing friend request.
//
//export ToxFriendRequestNew
func NewRequest(recipientPublicKey [32]byte, message string, senderSecretKey [32]byte) (*Request, error) {
	return NewRequestWithTimeProvider(recipientPublicKey, message, senderSecretKey, defaultTimeProvider)
}

// NewRequestWithTimeProvider creates a new outgoing friend request with a custom time provider.
// The sender's public key is derived from the provided secret key using curve25519.
func NewRequestWithTimeProvider(recipientPublicKey [32]byte, message string, senderSecretKey [32]byte, tp TimeProvider) (*Request, error) {
	if len(message) == 0 {
		logrus.WithFields(logrus.Fields{
			"function":             "NewRequestWithTimeProvider",
			"recipient_public_key": fmt.Sprintf("%x", recipientPublicKey[:8]),
		}).Warn("Friend request rejected: empty message")
		return nil, errors.New("message cannot be empty")
	}

	if len(message) > MaxFriendRequestMessageLength {
		logrus.WithFields(logrus.Fields{
			"function":             "NewRequestWithTimeProvider",
			"recipient_public_key": fmt.Sprintf("%x", recipientPublicKey[:8]),
			"message_length":       len(message),
			"max_length":           MaxFriendRequestMessageLength,
		}).Warn("Friend request rejected: message too long")
		return nil, fmt.Errorf("%w: got %d bytes", ErrFriendRequestMessageTooLong, len(message))
	}

	if tp == nil {
		tp = defaultTimeProvider
	}

	// Derive sender's public key from secret key
	senderKeyPair, err := crypto.FromSecretKey(senderSecretKey)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":             "NewRequestWithTimeProvider",
			"recipient_public_key": fmt.Sprintf("%x", recipientPublicKey[:8]),
			"error":                err.Error(),
		}).Error("Failed to derive public key from secret key")
		return nil, fmt.Errorf("failed to derive sender public key: %w", err)
	}

	// Generate nonce
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":             "NewRequestWithTimeProvider",
			"recipient_public_key": fmt.Sprintf("%x", recipientPublicKey[:8]),
			"error":                err.Error(),
		}).Error("Failed to generate nonce for friend request")
		return nil, err
	}

	request := &Request{
		SenderPublicKey: senderKeyPair.Public,
		Message:         message,
		Nonce:           nonce,
		Timestamp:       tp.Now(),
		timeProvider:    tp,
	}

	logrus.WithFields(logrus.Fields{
		"function":             "NewRequestWithTimeProvider",
		"sender_public_key":    fmt.Sprintf("%x", request.SenderPublicKey[:8]),
		"recipient_public_key": fmt.Sprintf("%x", recipientPublicKey[:8]),
		"message_length":       len(message),
	}).Debug("Friend request created successfully")

	return request, nil
}

// Encrypt encrypts a friend request for sending.
func (r *Request) Encrypt(senderKeyPair *crypto.KeyPair, recipientPublicKey [32]byte) ([]byte, error) {
	// Prepare message data
	messageData := []byte(r.Message)

	// Encrypt using crypto box
	encrypted, err := crypto.Encrypt(messageData, r.Nonce, recipientPublicKey, senderKeyPair.Private)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":             "Request.Encrypt",
			"sender_public_key":    fmt.Sprintf("%x", senderKeyPair.Public[:8]),
			"recipient_public_key": fmt.Sprintf("%x", recipientPublicKey[:8]),
			"message_length":       len(messageData),
			"error":                err.Error(),
		}).Error("Failed to encrypt friend request")
		return nil, fmt.Errorf("failed to encrypt friend request: %w", err)
	}

	// Format the request packet:
	// [sender public key (32 bytes)][nonce (24 bytes)][encrypted message]
	packet := make([]byte, 32+24+len(encrypted))
	copy(packet[0:32], senderKeyPair.Public[:])
	copy(packet[32:56], r.Nonce[:])
	copy(packet[56:], encrypted)

	logrus.WithFields(logrus.Fields{
		"function":             "Request.Encrypt",
		"sender_public_key":    fmt.Sprintf("%x", senderKeyPair.Public[:8]),
		"recipient_public_key": fmt.Sprintf("%x", recipientPublicKey[:8]),
		"packet_length":        len(packet),
	}).Debug("Friend request encrypted successfully")

	return packet, nil
}

// Decrypt decrypts a received friend request packet.
//
//export ToxFriendRequestDecrypt
func DecryptRequest(packet []byte, recipientSecretKey [32]byte) (*Request, error) {
	return DecryptRequestWithTimeProvider(packet, recipientSecretKey, defaultTimeProvider)
}

// DecryptRequestWithTimeProvider decrypts a received friend request packet with a custom time provider.
func DecryptRequestWithTimeProvider(packet []byte, recipientSecretKey [32]byte, tp TimeProvider) (*Request, error) {
	if len(packet) < 56 { // 32 (public key) + 24 (nonce)
		logrus.WithFields(logrus.Fields{
			"function":      "DecryptRequestWithTimeProvider",
			"packet_length": len(packet),
			"min_length":    56,
		}).Warn("Friend request decryption failed: invalid packet length")
		return nil, errors.New("invalid friend request packet")
	}

	if tp == nil {
		tp = defaultTimeProvider
	}

	var senderPublicKey [32]byte
	var nonce [24]byte
	copy(senderPublicKey[:], packet[0:32])
	copy(nonce[:], packet[32:56])

	encrypted := packet[56:]

	// Decrypt message
	decrypted, err := crypto.Decrypt(encrypted, nonce, senderPublicKey, recipientSecretKey)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":          "DecryptRequestWithTimeProvider",
			"sender_public_key": fmt.Sprintf("%x", senderPublicKey[:8]),
			"encrypted_length":  len(encrypted),
			"error":             err.Error(),
		}).Error("Failed to decrypt friend request")
		return nil, fmt.Errorf("failed to decrypt friend request: %w", err)
	}

	// Create request
	request := &Request{
		SenderPublicKey: senderPublicKey,
		Message:         string(decrypted),
		Nonce:           nonce,
		Timestamp:       tp.Now(),
		timeProvider:    tp,
	}

	logrus.WithFields(logrus.Fields{
		"function":          "DecryptRequestWithTimeProvider",
		"sender_public_key": fmt.Sprintf("%x", senderPublicKey[:8]),
		"message_length":    len(decrypted),
	}).Debug("Friend request decrypted successfully")

	return request, nil
}

// RequestHandler is a callback function for handling friend requests.
type RequestHandler func(request *Request) bool

// RequestManager manages friend requests with thread-safe access.
type RequestManager struct {
	mu              sync.RWMutex
	pendingRequests []*Request
	handler         RequestHandler
}

// NewRequestManager creates a new friend request manager.
//
//export ToxFriendRequestManagerNew
func NewRequestManager() *RequestManager {
	return &RequestManager{
		pendingRequests: make([]*Request, 0),
	}
}

// SetHandler sets the handler for incoming friend requests.
//
//export ToxFriendRequestManagerSetHandler
func (m *RequestManager) SetHandler(handler RequestHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
}

// AddRequest adds a new incoming friend request.
//
//export ToxFriendRequestManagerAddRequest
func (m *RequestManager) AddRequest(request *Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this is a duplicate
	for _, existing := range m.pendingRequests {
		if existing.SenderPublicKey == request.SenderPublicKey {
			// Update the existing request
			existing.Message = request.Message
			existing.Timestamp = request.Timestamp
			existing.Handled = false
			return
		}
	}

	// Add the new request
	m.pendingRequests = append(m.pendingRequests, request)

	// Call the handler if set (handler is read under the lock)
	handler := m.handler
	if handler != nil {
		// Release lock before calling handler to prevent deadlocks
		m.mu.Unlock()
		accepted := handler(request)
		m.mu.Lock()
		request.Handled = accepted
	}
}

// GetPendingRequests returns all pending friend requests.
//
//export ToxFriendRequestManagerGetPendingRequests
func (m *RequestManager) GetPendingRequests() []*Request {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return only unhandled requests
	var pending []*Request
	for _, req := range m.pendingRequests {
		if !req.Handled {
			pending = append(pending, req)
		}
	}
	return pending
}

// AcceptRequest accepts a friend request.
//
//export ToxFriendRequestManagerAcceptRequest
func (m *RequestManager) AcceptRequest(publicKey [32]byte) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, req := range m.pendingRequests {
		if req.SenderPublicKey == publicKey && !req.Handled {
			req.Handled = true
			return true
		}
	}
	return false
}

// RejectRequest rejects a friend request.
//
//export ToxFriendRequestManagerRejectRequest
func (m *RequestManager) RejectRequest(publicKey [32]byte) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, req := range m.pendingRequests {
		if req.SenderPublicKey == publicKey && !req.Handled {
			// Remove the request
			m.pendingRequests = append(m.pendingRequests[:i], m.pendingRequests[i+1:]...)
			return true
		}
	}
	return false
}

// requestSerialized is the internal representation for JSON serialization.
// This excludes non-serializable fields like timeProvider.
type requestSerialized struct {
	SenderPublicKey [32]byte  `json:"sender_public_key"`
	Message         string    `json:"message"`
	Nonce           [24]byte  `json:"nonce"`
	Timestamp       time.Time `json:"timestamp"`
	Handled         bool      `json:"handled"`
}

// Marshal serializes the Request to a JSON byte slice.
// This enables persistence of pending friend requests for savedata integration.
//
//export ToxFriendRequestMarshal
func (r *Request) Marshal() ([]byte, error) {
	serialized := requestSerialized{
		SenderPublicKey: r.SenderPublicKey,
		Message:         r.Message,
		Nonce:           r.Nonce,
		Timestamp:       r.Timestamp,
		Handled:         r.Handled,
	}

	data, err := json.Marshal(serialized)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":          "Request.Marshal",
			"sender_public_key": fmt.Sprintf("%x", r.SenderPublicKey[:8]),
			"error":             err.Error(),
		}).Error("Failed to marshal Request")
		return nil, fmt.Errorf("failed to marshal Request: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":          "Request.Marshal",
		"sender_public_key": fmt.Sprintf("%x", r.SenderPublicKey[:8]),
		"data_size":         len(data),
	}).Debug("Request marshaled successfully")

	return data, nil
}

// Unmarshal deserializes JSON data into this Request.
// The timeProvider is preserved if already set, otherwise defaults to system clock.
//
//export ToxFriendRequestUnmarshal
func (r *Request) Unmarshal(data []byte) error {
	var serialized requestSerialized
	if err := json.Unmarshal(data, &serialized); err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "Request.Unmarshal",
			"data_size": len(data),
			"error":     err.Error(),
		}).Error("Failed to unmarshal Request")
		return fmt.Errorf("failed to unmarshal Request: %w", err)
	}

	r.SenderPublicKey = serialized.SenderPublicKey
	r.Message = serialized.Message
	r.Nonce = serialized.Nonce
	r.Timestamp = serialized.Timestamp
	r.Handled = serialized.Handled

	// Preserve existing timeProvider or use default
	if r.timeProvider == nil {
		r.timeProvider = defaultTimeProvider
	}

	logrus.WithFields(logrus.Fields{
		"function":          "Request.Unmarshal",
		"sender_public_key": fmt.Sprintf("%x", r.SenderPublicKey[:8]),
		"handled":           r.Handled,
	}).Debug("Request unmarshaled successfully")

	return nil
}

// UnmarshalRequest creates a new Request from JSON data.
// This is a convenience function for creating a Request from serialized data.
//
//export ToxFriendRequestUnmarshalNew
func UnmarshalRequest(data []byte) (*Request, error) {
	r := &Request{
		timeProvider: defaultTimeProvider,
	}
	if err := r.Unmarshal(data); err != nil {
		return nil, err
	}
	return r, nil
}
