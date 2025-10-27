// Package noise provides Noise Protocol Framework implementation for Tox handshakes.
// This package implements the IK (Initiator with Knowledge) pattern to replace
// the legacy custom handshake with a formally verified cryptographic protocol.
package noise

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/flynn/noise"
	"github.com/opd-ai/toxcore/crypto"
)

var (
	// ErrHandshakeNotComplete indicates handshake is still in progress
	ErrHandshakeNotComplete = errors.New("handshake not complete")
	// ErrInvalidMessage indicates received message is invalid for current state
	ErrInvalidMessage = errors.New("invalid message for current handshake state")
	// ErrHandshakeComplete indicates handshake is already complete
	ErrHandshakeComplete = errors.New("handshake already complete")
)

// HandshakeRole defines whether we're initiating or responding to handshake
type HandshakeRole uint8

const (
	// Initiator starts the handshake (knows peer's static key)
	Initiator HandshakeRole = iota
	// Responder responds to handshake initiation
	Responder
)

// IKHandshake implements the Noise IK pattern for Tox protocol.
// IK provides mutual authentication and forward secrecy, suitable for
// scenarios where the initiator knows the responder's static public key.
type IKHandshake struct {
	role       HandshakeRole
	state      *noise.HandshakeState
	sendCipher *noise.CipherState
	recvCipher *noise.CipherState
	complete   bool
	nonce      [32]byte // Unique handshake nonce for replay protection
	timestamp  int64    // Unix timestamp for handshake freshness validation
}

// NewIKHandshake creates a new IK pattern handshake.
// staticPrivKey is our long-term private key (32 bytes).
// peerPubKey is peer's long-term public key (32 bytes, nil for responder).
// role determines if we initiate or respond to the handshake.
func NewIKHandshake(staticPrivKey, peerPubKey []byte, role HandshakeRole) (*IKHandshake, error) {
	// Validate that we're using a supported handshake pattern
	// The Noise-IK pattern is currently the only one fully supported
	if err := validateHandshakePattern("IK"); err != nil {
		return nil, fmt.Errorf("handshake pattern validation failed: %w", err)
	}

	if len(staticPrivKey) != 32 {
		return nil, fmt.Errorf("static private key must be 32 bytes, got %d", len(staticPrivKey))
	}

	if role == Initiator && (peerPubKey == nil || len(peerPubKey) != 32) {
		return nil, fmt.Errorf("initiator requires peer public key (32 bytes), got %v", len(peerPubKey))
	}

	// Create a copy of the private key to avoid modifying the original
	var privateKeyArray [32]byte
	copy(privateKeyArray[:], staticPrivKey)

	keyPair, err := crypto.FromSecretKey(privateKeyArray)
	if err != nil {
		// Securely wipe the private key copy before returning
		crypto.ZeroBytes(privateKeyArray[:])
		return nil, fmt.Errorf("failed to derive keypair: %w", err)
	}

	staticKey := noise.DHKey{
		Private: make([]byte, 32),
		Public:  make([]byte, 32),
	}
	copy(staticKey.Private, keyPair.Private[:])
	copy(staticKey.Public, keyPair.Public[:])

	// Securely wipe the private key copy after copying it
	crypto.ZeroBytes(privateKeyArray[:])

	cipherSuite := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256)
	config := noise.Config{
		CipherSuite:   cipherSuite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeIK,
		Initiator:     role == Initiator,
		StaticKeypair: staticKey,
	}

	// Set peer's static key for initiator (required for IK pattern)
	if role == Initiator && peerPubKey != nil {
		config.PeerStatic = make([]byte, 32)
		copy(config.PeerStatic, peerPubKey)
	}

	ik := &IKHandshake{
		role:      role,
		timestamp: time.Now().Unix(),
	}

	// Generate unique nonce for replay protection
	if _, err := rand.Read(ik.nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate handshake nonce: %w", err)
	}

	// Initialize handshake state
	ik.state, err = noise.NewHandshakeState(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create handshake state: %w", err)
	}

	return ik, nil
}

// WriteMessage processes the next handshake message.
// For initiator: creates the initial handshake message.
// For responder: processes received message and creates response.
// Returns the message to send to peer, completion status, and any error.
func (ik *IKHandshake) WriteMessage(payload, receivedMessage []byte) ([]byte, bool, error) {
	if ik.complete {
		return nil, false, ErrHandshakeComplete
	}

	if ik.role == Initiator {
		return ik.processInitiatorMessage(payload)
	}
	return ik.processResponderMessage(payload, receivedMessage)
}

// processInitiatorMessage handles the initiator's message creation during handshake.
// Writes the first message containing ephemeral key exchange, static key signature.
func (ik *IKHandshake) processInitiatorMessage(payload []byte) ([]byte, bool, error) {
	// Initiator: write first message (-> e, es, s, ss)
	message, sendCipher, recvCipher, err := ik.state.WriteMessage(nil, payload)
	if err != nil {
		return nil, false, fmt.Errorf("initiator write failed: %w", err)
	}

	// In IK pattern, initiator gets cipher states but doesn't complete until responder replies
	ik.sendCipher = sendCipher
	ik.recvCipher = recvCipher
	// Note: ik.complete remains false - initiator must wait for responder's message

	return message, ik.complete, nil
}

// processResponderMessage handles the responder's message processing and response creation.
// First reads the initiator's message, then creates and returns the response.
func (ik *IKHandshake) processResponderMessage(payload, receivedMessage []byte) ([]byte, bool, error) {
	// Responder: first read initiator's message, then write response
	if receivedMessage == nil {
		return nil, false, fmt.Errorf("responder requires received message")
	}

	// Read initiator's message
	_, _, _, err := ik.state.ReadMessage(nil, receivedMessage)
	if err != nil {
		return nil, false, fmt.Errorf("responder read failed: %w", err)
	}

	// Write response message (<- e, ee, se)
	message, writeSendCipher, writeRecvCipher, err := ik.state.WriteMessage(nil, payload)
	if err != nil {
		return nil, false, fmt.Errorf("responder write failed: %w", err)
	}

	ik.sendCipher = writeSendCipher
	ik.recvCipher = writeRecvCipher
	ik.complete = true // IK responder completes after response

	return message, ik.complete, nil
}

// ReadMessage processes a received handshake message.
// Only used by initiator to process responder's response.
// Returns decrypted payload and completion status.
func (ik *IKHandshake) ReadMessage(message []byte) ([]byte, bool, error) {
	if ik.complete {
		return nil, false, ErrHandshakeComplete
	}

	if ik.role != Initiator {
		return nil, false, fmt.Errorf("only initiator can read response messages")
	}

	// Read responder's response
	payload, recvCipher, sendCipher, err := ik.state.ReadMessage(nil, message)
	if err != nil {
		return nil, false, fmt.Errorf("initiator read response failed: %w", err)
	}

	ik.recvCipher = recvCipher
	ik.sendCipher = sendCipher
	ik.complete = true
	return payload, ik.complete, nil
}

// IsComplete returns true if handshake is finished and cipher states are available.
func (ik *IKHandshake) IsComplete() bool {
	return ik.complete
}

// GetCipherStates returns the send and receive cipher states after successful handshake.
// Send cipher encrypts outgoing messages, receive cipher decrypts incoming messages.
func (ik *IKHandshake) GetCipherStates() (*noise.CipherState, *noise.CipherState, error) {
	if !ik.complete {
		return nil, nil, ErrHandshakeNotComplete
	}

	if ik.sendCipher == nil || ik.recvCipher == nil {
		return nil, nil, fmt.Errorf("cipher states not available")
	}

	return ik.sendCipher, ik.recvCipher, nil
}

// GetRemoteStaticKey returns the peer's static public key after successful handshake.
// This key can be used to verify the peer's identity.
func (ik *IKHandshake) GetRemoteStaticKey() ([]byte, error) {
	if !ik.complete {
		return nil, ErrHandshakeNotComplete
	}

	remoteKey := ik.state.PeerStatic()
	if len(remoteKey) == 0 {
		return nil, fmt.Errorf("remote static key not available")
	}

	// Return a copy to prevent modification
	key := make([]byte, len(remoteKey))
	copy(key, remoteKey)
	return key, nil
}

// GetLocalStaticKey returns our static public key.
// This is the key other peers will use to identify us.
func (ik *IKHandshake) GetLocalStaticKey() []byte {
	// Get our static key from the handshake state
	localEphemeral := ik.state.LocalEphemeral()
	if len(localEphemeral.Public) > 0 {
		// Return a copy to prevent modification
		key := make([]byte, len(localEphemeral.Public))
		copy(key, localEphemeral.Public)
		return key
	}
	return nil
}

// GetNonce returns the handshake nonce for replay protection.
func (ik *IKHandshake) GetNonce() [32]byte {
	return ik.nonce
}

// GetTimestamp returns the handshake creation timestamp.
func (ik *IKHandshake) GetTimestamp() int64 {
	return ik.timestamp
}

// XXHandshake implements the Noise XX pattern for mutual authentication
// without prior key knowledge. Useful for initial contact scenarios.
type XXHandshake struct {
	role        HandshakeRole
	state       *noise.HandshakeState
	sendCipher  *noise.CipherState
	recvCipher  *noise.CipherState
	complete    bool
	localPubKey []byte // Store our static public key
}

// NewXXHandshake creates a new XX pattern handshake.
// staticPrivKey is our long-term private key (32 bytes).
// role determines if we initiate or respond to the handshake.
func NewXXHandshake(staticPrivKey []byte, role HandshakeRole) (*XXHandshake, error) {
	if len(staticPrivKey) != 32 {
		return nil, fmt.Errorf("static private key must be 32 bytes, got %d", len(staticPrivKey))
	}

	// Create keypair from private key using crypto package
	var privateKeyArray [32]byte
	copy(privateKeyArray[:], staticPrivKey)

	keyPair, err := crypto.FromSecretKey(privateKeyArray)
	if err != nil {
		return nil, fmt.Errorf("failed to create keypair: %w", err)
	}

	// Create static keypair for noise
	staticKey := noise.DHKey{
		Private: make([]byte, 32),
		Public:  make([]byte, 32),
	}
	copy(staticKey.Private, keyPair.Private[:])
	copy(staticKey.Public, keyPair.Public[:])

	// Securely wipe the private key copy after copying it
	crypto.ZeroBytes(privateKeyArray[:])

	cipherSuite := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256)
	config := noise.Config{
		CipherSuite:   cipherSuite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeXX,
		Initiator:     role == Initiator,
		StaticKeypair: staticKey,
	}

	// XX pattern doesn't require peer's static key beforehand (mutual auth)
	hs, err := noise.NewHandshakeState(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create XX handshake state: %w", err)
	}

	return &XXHandshake{
		role:        role,
		state:       hs,
		localPubKey: keyPair.Public[:],
	}, nil
}

// WriteMessage writes a handshake message for XX pattern.
func (xx *XXHandshake) WriteMessage(payload, receivedMessage []byte) ([]byte, bool, error) {
	if xx.complete {
		return nil, false, ErrHandshakeComplete
	}

	message, send, recv, err := xx.state.WriteMessage(nil, payload)
	if err != nil {
		return nil, false, fmt.Errorf("XX handshake write failed: %w", err)
	}

	// Check if handshake is complete
	if send != nil && recv != nil {
		xx.sendCipher = send
		xx.recvCipher = recv
		xx.complete = true
		return message, true, nil
	}

	return message, false, nil
}

// ReadMessage reads a handshake message for XX pattern.
func (xx *XXHandshake) ReadMessage(message []byte) ([]byte, bool, error) {
	if xx.complete {
		return nil, false, ErrHandshakeComplete
	}

	payload, send, recv, err := xx.state.ReadMessage(nil, message)
	if err != nil {
		return nil, false, fmt.Errorf("XX handshake read failed: %w", err)
	}

	// Check if handshake is complete
	if send != nil && recv != nil {
		xx.sendCipher = send
		xx.recvCipher = recv
		xx.complete = true
		return payload, true, nil
	}

	return payload, false, nil
}

// IsComplete returns whether the XX handshake is complete.
func (xx *XXHandshake) IsComplete() bool {
	return xx.complete
}

// GetCipherStates returns the established cipher states for XX pattern.
func (xx *XXHandshake) GetCipherStates() (*noise.CipherState, *noise.CipherState, error) {
	if !xx.complete {
		return nil, nil, ErrHandshakeNotComplete
	}
	return xx.sendCipher, xx.recvCipher, nil
}

// GetRemoteStaticKey returns the peer's static key after XX handshake completion.
func (xx *XXHandshake) GetRemoteStaticKey() ([]byte, error) {
	if !xx.complete {
		return nil, ErrHandshakeNotComplete
	}
	return xx.state.PeerStatic(), nil
}

// GetLocalStaticKey returns our static public key for XX pattern.
func (xx *XXHandshake) GetLocalStaticKey() []byte {
	// Return a copy of our stored static public key
	if len(xx.localPubKey) > 0 {
		key := make([]byte, len(xx.localPubKey))
		copy(key, xx.localPubKey)
		return key
	}
	return nil
}

// validateHandshakePattern validates that a handshake pattern is supported.
func validateHandshakePattern(pattern string) error {
	supportedPatterns := map[string]bool{
		"IK": true,  // Initiator with Knowledge - currently supported - used in tox because friend public keys are known in advance
		"XX": true,  // Interactive Exchange - basic support added - useful for initial contact without prior key knowledge
		"XK": false, // Responder has Knowledge - future support planned - may be useful for server scenarios where client keys are unknown
		"NK": false, // No static key authentication - future support planned - may be useful for anonymous connections or public services
		"KK": false, // Known Keys on both sides - future support planned - may be useful for high-trust scenarios with pre-shared keys
	}

	supported, exists := supportedPatterns[pattern]
	if !exists {
		return fmt.Errorf("unknown handshake pattern: %s", pattern)
	}

	if !supported {
		return fmt.Errorf("handshake pattern %s is not yet supported", pattern)
	}

	// Additional security policy validation for supported patterns
	if pattern == "IK" {
		// IK pattern requires that the initiator knows the responder's static key
		// This is appropriate for Tox where friend public keys are known in advance
		return nil
	}

	if pattern == "XX" {
		// XX pattern provides mutual authentication without prior key knowledge
		// Useful for initial contact or when static keys are not pre-shared
		return nil
	}

	return fmt.Errorf("handshake pattern %s failed security validation", pattern)
}
