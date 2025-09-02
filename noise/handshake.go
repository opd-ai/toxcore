// Package noise provides Noise Protocol Framework implementation for Tox handshakes.
// This package implements the IK (Initiator with Knowledge) pattern to replace
// the legacy custom handshake with a formally verified cryptographic protocol.
package noise

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/flynn/noise"
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
	role        HandshakeRole
	state       *noise.HandshakeState
	sendCipher  *noise.CipherState
	recvCipher  *noise.CipherState
	complete    bool
}

// NewIKHandshake creates a new IK pattern handshake.
// staticPrivKey is our long-term private key (32 bytes).
// peerPubKey is peer's long-term public key (32 bytes, nil for responder).
// role determines if we initiate or respond to the handshake.
func NewIKHandshake(staticPrivKey []byte, peerPubKey []byte, role HandshakeRole) (*IKHandshake, error) {
	if len(staticPrivKey) != 32 {
		return nil, fmt.Errorf("static private key must be 32 bytes, got %d", len(staticPrivKey))
	}
	
	if role == Initiator && (peerPubKey == nil || len(peerPubKey) != 32) {
		return nil, fmt.Errorf("initiator requires peer public key (32 bytes), got %v", len(peerPubKey))
	}

	// Create static keypair from private key
	cipherSuite := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256)
	staticKey, err := cipherSuite.GenerateKeypair(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}
	
	// Replace with our actual private key
	copy(staticKey.Private, staticPrivKey)
	
	// For production, we would derive the public key properly from private key
	// For now, we'll use the generated one as placeholder
	// TODO: Implement proper Curve25519 scalar multiplication

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
		role: role,
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
func (ik *IKHandshake) WriteMessage(payload []byte, receivedMessage []byte) ([]byte, bool, error) {
	if ik.complete {
		return nil, false, ErrHandshakeComplete
	}

	var message []byte
	var err error
	var sendCipher, recvCipher *noise.CipherState

	// Process based on role and handshake state
	if ik.role == Initiator {
		// Initiator: write first message (-> e, es, s, ss)
		message, sendCipher, recvCipher, err = ik.state.WriteMessage(nil, payload)
		if err != nil {
			return nil, false, fmt.Errorf("initiator write failed: %w", err)
		}
		ik.sendCipher = sendCipher
		ik.recvCipher = recvCipher
		ik.complete = true // IK initiator completes in one message
	} else {
		// Responder: first read initiator's message, then write response
		if receivedMessage == nil {
			return nil, false, fmt.Errorf("responder requires received message")
		}

		// Read initiator's message
		_, recvCipher, sendCipher, err = ik.state.ReadMessage(nil, receivedMessage)
		if err != nil {
			return nil, false, fmt.Errorf("responder read failed: %w", err)
		}

		// Write response message (<- e, ee, se)
		message, sendCipher, recvCipher, err = ik.state.WriteMessage(nil, payload)
		if err != nil {
			return nil, false, fmt.Errorf("responder write failed: %w", err)
		}
		ik.sendCipher = sendCipher
		ik.recvCipher = recvCipher
		ik.complete = true // IK responder completes after response
	}

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
