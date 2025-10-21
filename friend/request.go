package friend

import (
	"errors"
	"time"

	"github.com/opd-ai/toxforge/crypto"
)

// Request represents a friend request.
//
//export ToxFriendRequest
type Request struct {
	SenderPublicKey [32]byte
	Message         string
	Nonce           [24]byte
	Timestamp       time.Time
	Handled         bool
}

// NewRequest creates a new outgoing friend request.
//
//export ToxFriendRequestNew
func NewRequest(recipientPublicKey [32]byte, message string, senderSecretKey [32]byte) (*Request, error) {
	if len(message) == 0 {
		return nil, errors.New("message cannot be empty")
	}

	// Generate nonce
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		return nil, err
	}

	request := &Request{
		Message:   message,
		Nonce:     nonce,
		Timestamp: time.Now(),
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
	if len(packet) < 56 { // 32 (public key) + 24 (nonce)
		return nil, errors.New("invalid friend request packet")
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
		Timestamp:       time.Now(),
	}

	return request, nil
}

// RequestHandler is a callback function for handling friend requests.
type RequestHandler func(request *Request) bool

// RequestManager manages friend requests.
type RequestManager struct {
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
	m.handler = handler
}

// AddRequest adds a new incoming friend request.
//
//export ToxFriendRequestManagerAddRequest
func (m *RequestManager) AddRequest(request *Request) {
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

	// Call the handler if set
	if m.handler != nil {
		accepted := m.handler(request)
		request.Handled = accepted
	}
}

// GetPendingRequests returns all pending friend requests.
//
//export ToxFriendRequestManagerGetPendingRequests
func (m *RequestManager) GetPendingRequests() []*Request {
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
	for i, req := range m.pendingRequests {
		if req.SenderPublicKey == publicKey && !req.Handled {
			// Remove the request
			m.pendingRequests = append(m.pendingRequests[:i], m.pendingRequests[i+1:]...)
			return true
		}
	}
	return false
}
