package friend

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// ProtocolType represents the encryption protocol used for friend requests
type ProtocolType uint8

const (
	ProtocolLegacy ProtocolType = 0
	ProtocolNoise  ProtocolType = 1
)

// Request represents a friend request with protocol support.
//
//export ToxFriendRequest
type Request struct {
	SenderPublicKey [32]byte
	Message         string
	Nonce           [24]byte // Used for legacy encryption
	Timestamp       time.Time
	Handled         bool
	Protocol        ProtocolType                 // Which protocol was used
	Capabilities    *crypto.ProtocolCapabilities // Sender's capabilities
	NoiseSession    *crypto.NoiseSession         // If using Noise protocol
}

// EnhancedRequest represents a friend request with capability negotiation
type EnhancedRequest struct {
	*Request
	SelectedProtocol crypto.ProtocolVersion
	SelectedCipher   string
}

// NewRequest creates a new outgoing friend request with protocol capabilities.
//
//export ToxFriendRequestNew
func NewRequest(recipientPublicKey [32]byte, message string, senderKeyPair *crypto.KeyPair, capabilities *crypto.ProtocolCapabilities) (*Request, error) {
	if len(message) == 0 {
		return nil, errors.New("message cannot be empty")
	}
	if capabilities == nil {
		capabilities = crypto.NewProtocolCapabilities()
	}

	// Generate nonce for legacy compatibility
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		return nil, err
	}

	request := &Request{
		Message:      message,
		Nonce:        nonce,
		Timestamp:    time.Now(),
		Protocol:     ProtocolLegacy, // Default to legacy for compatibility
		Capabilities: capabilities,
	}

	// Set sender public key from keypair
	copy(request.SenderPublicKey[:], senderKeyPair.Public[:])

	return request, nil
}

// EncryptWithNoise encrypts a friend request using Noise protocol
func (r *Request) EncryptWithNoise(senderKeyPair *crypto.KeyPair, recipientPublicKey [32]byte) ([]byte, error) {
	// Create Noise handshake as initiator
	handshake, err := crypto.NewNoiseHandshake(true, senderKeyPair.Private, recipientPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Noise handshake: %w", err)
	}

	// Prepare the friend request payload
	requestData := struct {
		Message      string                       `json:"message"`
		Timestamp    time.Time                    `json:"timestamp"`
		Capabilities *crypto.ProtocolCapabilities `json:"capabilities"`
	}{
		Message:      r.Message,
		Timestamp:    r.Timestamp,
		Capabilities: r.Capabilities,
	}

	payloadBytes, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	// Perform Noise handshake with the friend request as payload
	message, session, err := handshake.WriteMessage(payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write Noise handshake message: %w", err)
	}

	// Store the session for potential future use
	r.NoiseSession = session
	r.Protocol = ProtocolNoise

	// Format the packet: [protocol_type(1)][sender_public_key(32)][message_length(4)][noise_message]
	packet := make([]byte, 1+32+4+len(message))
	packet[0] = byte(ProtocolNoise)
	copy(packet[1:33], senderKeyPair.Public[:])

	// Write message length as big-endian uint32
	packet[33] = byte(len(message) >> 24)
	packet[34] = byte(len(message) >> 16)
	packet[35] = byte(len(message) >> 8)
	packet[36] = byte(len(message))

	copy(packet[37:], message)

	return packet, nil
}

// Encrypt encrypts a friend request for sending with automatic protocol selection.
func (r *Request) Encrypt(senderKeyPair *crypto.KeyPair, recipientPublicKey [32]byte) ([]byte, error) {
	// Try Noise protocol first if supported
	if r.Capabilities != nil && r.Capabilities.NoiseSupported {
		noisePacket, err := r.EncryptWithNoise(senderKeyPair, recipientPublicKey)
		if err == nil {
			return noisePacket, nil
		}
		// Fall back to legacy if Noise fails
	}

	// Use legacy encryption
	return r.EncryptLegacy(senderKeyPair, recipientPublicKey)
}

// EncryptLegacy encrypts a friend request using legacy crypto box encryption
func (r *Request) EncryptLegacy(senderKeyPair *crypto.KeyPair, recipientPublicKey [32]byte) ([]byte, error) {
	// Prepare message data with capabilities
	requestData := struct {
		Message      string                       `json:"message"`
		Timestamp    time.Time                    `json:"timestamp"`
		Capabilities *crypto.ProtocolCapabilities `json:"capabilities,omitempty"`
	}{
		Message:      r.Message,
		Timestamp:    r.Timestamp,
		Capabilities: r.Capabilities,
	}

	messageData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	// Encrypt using crypto box
	encrypted, err := crypto.Encrypt(messageData, r.Nonce, recipientPublicKey, senderKeyPair.Private)
	if err != nil {
		return nil, err
	}

	// Format the packet: [protocol_type(1)][sender_public_key(32)][nonce(24)][encrypted_message]
	packet := make([]byte, 1+32+24+len(encrypted))
	packet[0] = byte(ProtocolLegacy)
	copy(packet[1:33], senderKeyPair.Public[:])
	copy(packet[33:57], r.Nonce[:])
	copy(packet[57:], encrypted)

	r.Protocol = ProtocolLegacy
	return packet, nil
}

// DecryptRequest decrypts a received friend request packet with protocol detection.
//
//export ToxFriendRequestDecrypt
func DecryptRequest(packet []byte, recipientKeyPair *crypto.KeyPair) (*Request, error) {
	if len(packet) < 1 {
		return nil, errors.New("invalid friend request packet: too short")
	}

	protocolType := ProtocolType(packet[0])

	switch protocolType {
	case ProtocolNoise:
		return DecryptNoiseRequest(packet[1:], recipientKeyPair)
	case ProtocolLegacy:
		return DecryptLegacyRequest(packet[1:], recipientKeyPair)
	default:
		// Try legacy format for backward compatibility (no protocol prefix)
		return DecryptLegacyRequest(packet, recipientKeyPair)
	}
}

// DecryptNoiseRequest decrypts a Noise protocol friend request
func DecryptNoiseRequest(packet []byte, recipientKeyPair *crypto.KeyPair) (*Request, error) {
	if len(packet) < 36 { // 32 (public key) + 4 (length)
		return nil, errors.New("invalid Noise friend request packet")
	}

	var senderPublicKey [32]byte
	copy(senderPublicKey[:], packet[0:32])

	// Read message length
	messageLen := (uint32(packet[32]) << 24) | (uint32(packet[33]) << 16) |
		(uint32(packet[34]) << 8) | uint32(packet[35])

	if len(packet) < 36+int(messageLen) {
		return nil, errors.New("invalid Noise friend request packet: message truncated")
	}

	noiseMessage := packet[36 : 36+messageLen]

	// Create Noise handshake as responder
	handshake, err := crypto.NewNoiseHandshake(false, recipientKeyPair.Private, senderPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Noise handshake: %w", err)
	}

	// Process the handshake message
	payload, session, err := handshake.ReadMessage(noiseMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to read Noise handshake message: %w", err)
	}

	// Decode the friend request payload
	var requestData struct {
		Message      string                       `json:"message"`
		Timestamp    time.Time                    `json:"timestamp"`
		Capabilities *crypto.ProtocolCapabilities `json:"capabilities"`
	}

	err = json.Unmarshal(payload, &requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal request data: %w", err)
	}

	// Create request
	request := &Request{
		SenderPublicKey: senderPublicKey,
		Message:         requestData.Message,
		Timestamp:       requestData.Timestamp,
		Protocol:        ProtocolNoise,
		Capabilities:    requestData.Capabilities,
		NoiseSession:    session,
	}

	return request, nil
}

// DecryptLegacyRequest decrypts a legacy crypto box friend request
func DecryptLegacyRequest(packet []byte, recipientKeyPair *crypto.KeyPair) (*Request, error) {
	if len(packet) < 56 { // 32 (public key) + 24 (nonce)
		return nil, errors.New("invalid legacy friend request packet")
	}

	var senderPublicKey [32]byte
	var nonce [24]byte
	copy(senderPublicKey[:], packet[0:32])
	copy(nonce[:], packet[32:56])

	encrypted := packet[56:]

	// Decrypt message
	decrypted, err := crypto.Decrypt(encrypted, nonce, senderPublicKey, recipientKeyPair.Private)
	if err != nil {
		return nil, err
	}

	// Try to decode as JSON with capabilities
	var requestData struct {
		Message      string                       `json:"message"`
		Timestamp    time.Time                    `json:"timestamp"`
		Capabilities *crypto.ProtocolCapabilities `json:"capabilities,omitempty"`
	}

	err = json.Unmarshal(decrypted, &requestData)
	if err != nil {
		// Fall back to plain string message for old clients
		request := &Request{
			SenderPublicKey: senderPublicKey,
			Message:         string(decrypted),
			Nonce:           nonce,
			Timestamp:       time.Now(),
			Protocol:        ProtocolLegacy,
		}
		return request, nil
	}

	// Create request with capabilities
	request := &Request{
		SenderPublicKey: senderPublicKey,
		Message:         requestData.Message,
		Nonce:           nonce,
		Timestamp:       requestData.Timestamp,
		Protocol:        ProtocolLegacy,
		Capabilities:    requestData.Capabilities,
	}

	return request, nil
}

// NegotiateProtocol determines the best protocol to use for communication
func (r *Request) NegotiateProtocol(localCapabilities *crypto.ProtocolCapabilities) (*EnhancedRequest, error) {
	if r.Capabilities == nil {
		// No capabilities provided, use legacy
		return &EnhancedRequest{
			Request:          r,
			SelectedProtocol: crypto.ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			SelectedCipher:   "legacy",
		}, nil
	}

	// Perform protocol negotiation
	version, cipher, err := crypto.SelectBestProtocol(localCapabilities, r.Capabilities)
	if err != nil {
		return nil, fmt.Errorf("protocol negotiation failed: %w", err)
	}

	return &EnhancedRequest{
		Request:          r,
		SelectedProtocol: version,
		SelectedCipher:   cipher,
	}, nil
}

// SupportsNoise returns whether this request supports Noise protocol
func (r *Request) SupportsNoise() bool {
	return r.Capabilities != nil && r.Capabilities.NoiseSupported
}

// GetProtocolString returns a human-readable protocol description
func (r *Request) GetProtocolString() string {
	switch r.Protocol {
	case ProtocolNoise:
		return "Noise-IK"
	case ProtocolLegacy:
		return "Legacy"
	default:
		return "Unknown"
	}
}

// RequestHandler is a callback function for handling friend requests.
type RequestHandler func(request *EnhancedRequest) bool

// RequestManager manages friend requests with protocol capability support.
type RequestManager struct {
	pendingRequests   []*EnhancedRequest
	handler           RequestHandler
	localCapabilities *crypto.ProtocolCapabilities
	sessionManager    *crypto.SessionManager
	mu                sync.RWMutex
}

// NewRequestManager creates a new friend request manager with protocol capabilities.
//
//export ToxFriendRequestManagerNew
func NewRequestManager(capabilities *crypto.ProtocolCapabilities) *RequestManager {
	if capabilities == nil {
		capabilities = crypto.NewProtocolCapabilities()
	}

	return &RequestManager{
		pendingRequests:   make([]*EnhancedRequest, 0),
		localCapabilities: capabilities,
		sessionManager:    crypto.NewSessionManager(),
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

// ProcessIncomingRequest processes a received friend request packet and creates an enhanced request
//
//export ToxFriendRequestManagerProcessIncoming
func (m *RequestManager) ProcessIncomingRequest(packet []byte, recipientKeyPair *crypto.KeyPair) error {
	// Decrypt the request
	request, err := DecryptRequest(packet, recipientKeyPair)
	if err != nil {
		return fmt.Errorf("failed to decrypt friend request: %w", err)
	}

	// Perform protocol negotiation
	enhancedRequest, err := request.NegotiateProtocol(m.localCapabilities)
	if err != nil {
		return fmt.Errorf("protocol negotiation failed: %w", err)
	}

	// Store Noise session if available
	if request.NoiseSession != nil && m.sessionManager != nil {
		m.sessionManager.AddSession(request.SenderPublicKey, request.NoiseSession)
	}

	// Add the request
	m.AddRequest(enhancedRequest)
	return nil
}

// AddRequest adds a new incoming friend request.
//
//export ToxFriendRequestManagerAddRequest
func (m *RequestManager) AddRequest(request *EnhancedRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this is a duplicate
	for _, existing := range m.pendingRequests {
		if existing.SenderPublicKey == request.SenderPublicKey {
			// Update the existing request
			existing.Message = request.Message
			existing.Timestamp = request.Timestamp
			existing.Handled = false
			existing.SelectedProtocol = request.SelectedProtocol
			existing.SelectedCipher = request.SelectedCipher
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
func (m *RequestManager) GetPendingRequests() []*EnhancedRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return only unhandled requests
	var pending []*EnhancedRequest
	for _, req := range m.pendingRequests {
		if !req.Handled {
			pending = append(pending, req)
		}
	}
	return pending
}

// AcceptRequest accepts a friend request and returns the established session (if Noise).
//
//export ToxFriendRequestManagerAcceptRequest
func (m *RequestManager) AcceptRequest(publicKey [32]byte) (*crypto.NoiseSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, req := range m.pendingRequests {
		if req.SenderPublicKey == publicKey && !req.Handled {
			req.Handled = true

			// Return Noise session if available
			if req.NoiseSession != nil {
				return req.NoiseSession, true
			}
			return nil, true
		}
	}
	return nil, false
}

// RejectRequest rejects a friend request.
//
//export ToxFriendRequestManagerRejectRequest
func (m *RequestManager) RejectRequest(publicKey [32]byte) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, req := range m.pendingRequests {
		if req.SenderPublicKey == publicKey && !req.Handled {
			// Remove Noise session if exists
			if req.NoiseSession != nil && m.sessionManager != nil {
				m.sessionManager.RemoveSession(req.SenderPublicKey)
			}

			// Remove the request
			m.pendingRequests = append(m.pendingRequests[:i], m.pendingRequests[i+1:]...)
			return true
		}
	}
	return false
}

// GetCapabilities returns the local protocol capabilities
//
//export ToxFriendRequestManagerGetCapabilities
func (m *RequestManager) GetCapabilities() *crypto.ProtocolCapabilities {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.localCapabilities
}

// UpdateCapabilities updates the local protocol capabilities
//
//export ToxFriendRequestManagerUpdateCapabilities
func (m *RequestManager) UpdateCapabilities(capabilities *crypto.ProtocolCapabilities) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.localCapabilities = capabilities
}

// GetSessionManager returns the session manager
//
//export ToxFriendRequestManagerGetSessionManager
func (m *RequestManager) GetSessionManager() *crypto.SessionManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessionManager
}

// Clear removes all pending friend requests.
//
//export ToxFriendRequestManagerClear
func (m *RequestManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear Noise sessions
	if m.sessionManager != nil {
		for _, req := range m.pendingRequests {
			if req.NoiseSession != nil {
				m.sessionManager.RemoveSession(req.SenderPublicKey)
			}
		}
	}

	m.pendingRequests = make([]*EnhancedRequest, 0)
	m.handler = nil
}

// CreateOutgoingRequest creates an outgoing friend request with automatic protocol selection
//
//export ToxFriendRequestManagerCreateOutgoing
func (m *RequestManager) CreateOutgoingRequest(recipientPublicKey [32]byte, message string, senderKeyPair *crypto.KeyPair) ([]byte, error) {
	m.mu.RLock()
	capabilities := m.localCapabilities
	m.mu.RUnlock()

	// Create the request
	request, err := NewRequest(recipientPublicKey, message, senderKeyPair, capabilities)
	if err != nil {
		return nil, err
	}

	// Encrypt with automatic protocol selection
	return request.Encrypt(senderKeyPair, recipientPublicKey)
}
