package friend

import (
	"errors"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
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
func NewRequestWithTimeProvider(recipientPublicKey [32]byte, message string, senderSecretKey [32]byte, tp TimeProvider) (*Request, error) {
	if len(message) == 0 {
		return nil, errors.New("message cannot be empty")
	}

	if tp == nil {
		tp = defaultTimeProvider
	}

	// Generate nonce
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		return nil, err
	}

	request := &Request{
		Message:      message,
		Nonce:        nonce,
		Timestamp:    tp.Now(),
		timeProvider: tp,
	}

	return request, nil
}

// Encrypt encrypts a friend request for sending.
func (r *Request) Encrypt(senderKeyPair *crypto.KeyPair, recipientPublicKey [32]byte) ([]byte, error) {
	// Prepare message data
	messageData := []byte(r.Message)

	// Encrypt using crypto box
	encrypted, err := crypto.Encrypt(messageData, r.Nonce, recipientPublicKey, senderKeyPair.Private)
	if err != nil {
		return nil, err
	}

	// Format the request packet:
	// [sender public key (32 bytes)][nonce (24 bytes)][encrypted message]
	packet := make([]byte, 32+24+len(encrypted))
	copy(packet[0:32], senderKeyPair.Public[:])
	copy(packet[32:56], r.Nonce[:])
	copy(packet[56:], encrypted)

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
		return nil, err
	}

	// Create request
	request := &Request{
		SenderPublicKey: senderPublicKey,
		Message:         string(decrypted),
		Nonce:           nonce,
		Timestamp:       tp.Now(),
		timeProvider:    tp,
	}

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
