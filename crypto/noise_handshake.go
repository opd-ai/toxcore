package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/flynn/noise"
)

// NoiseConfig represents configuration for Noise protocol
type NoiseConfig struct {
	Pattern     string
	CipherSuite string
	Initiator   bool
	StaticKey   [32]byte
	PeerKey     [32]byte
}

// NoiseHandshake manages the Noise-IK handshake state
//
//export ToxNoiseHandshake
type NoiseHandshake struct {
	config      *NoiseConfig
	handshake   *noise.HandshakeState
	cipherState *noise.CipherState
	completed   bool
	timestamp   time.Time
}

// NoiseSession represents an established Noise session
//
//export ToxNoiseSession
type NoiseSession struct {
	SendCipher       *noise.CipherState
	RecvCipher       *noise.CipherState
	StaticKeys       *KeyPair
	EphemeralKeys    *KeyPair
	PeerKey          [32]byte
	Established      time.Time
	LastUsed         time.Time
	RekeyNeeded      bool
	RekeysPerformed  uint64
	LastRekey        time.Time
	MessageCounter   uint64
}

// NewNoiseHandshake creates a new Noise-IK handshake
//
//export ToxNewNoiseHandshake
func NewNoiseHandshake(isInitiator bool, staticKey [32]byte, peerKey [32]byte) (*NoiseHandshake, error) {
	config := &NoiseConfig{
		Pattern:     "IK",
		CipherSuite: "Noise_IK_25519_ChaChaPoly_SHA256",
		Initiator:   isInitiator,
		StaticKey:   staticKey,
		PeerKey:     peerKey,
	}

	// Create cipher suite
	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256)

	var handshakePattern noise.HandshakePattern
	if isInitiator {
		handshakePattern = noise.HandshakeIK
	} else {
		handshakePattern = noise.HandshakeIK
	}

	// Create handshake state configuration
	hsConfig := noise.Config{
		CipherSuite:   cs,
		Random:        rand.Reader,
		Pattern:       handshakePattern,
		Initiator:     isInitiator,
		StaticKeypair: noise.DHKey{Private: staticKey[:], Public: derivePublicKey(staticKey)},
	}

	// In IK pattern, only the initiator knows the responder's static key beforehand
	if isInitiator {
		hsConfig.PeerStatic = peerKey[:]
	}
	// Responder learns initiator's static key during handshake (rs starts as nil)

	hs, err := noise.NewHandshakeState(hsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create handshake state: %w", err)
	}

	return &NoiseHandshake{
		config:    config,
		handshake: hs,
		completed: false,
		timestamp: time.Now(),
	}, nil
}

// WriteMessage processes the next handshake message for sending
//
//export ToxNoiseWriteMessage
func (nh *NoiseHandshake) WriteMessage(payload []byte) ([]byte, *NoiseSession, error) {
	if nh.completed {
		return nil, nil, errors.New("handshake already completed")
	}

	message, cs1, cs2, err := nh.handshake.WriteMessage(nil, payload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write handshake message: %w", err)
	}

	// Check if handshake is complete
	if cs1 != nil && cs2 != nil {
		nh.completed = true
		nh.cipherState = cs1

		session := &NoiseSession{
			SendCipher:  cs1,
			RecvCipher:  cs2,
			PeerKey:     nh.config.PeerKey,
			Established: time.Now(),
			LastUsed:    time.Now(),
			RekeyNeeded: false,
		}

		return message, session, nil
	}

	return message, nil, nil
}

// ReadMessage processes a received handshake message
//
//export ToxNoiseReadMessage
func (nh *NoiseHandshake) ReadMessage(message []byte) ([]byte, *NoiseSession, error) {
	if nh.completed {
		return nil, nil, errors.New("handshake already completed")
	}

	payload, cs1, cs2, err := nh.handshake.ReadMessage(nil, message)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read handshake message: %w", err)
	}

	// Check if handshake is complete
	if cs1 != nil && cs2 != nil {
		nh.completed = true
		nh.cipherState = cs1

		session := &NoiseSession{
			SendCipher:  cs1,
			RecvCipher:  cs2,
			PeerKey:     nh.config.PeerKey,
			Established: time.Now(),
			LastUsed:    time.Now(),
			RekeyNeeded: false,
		}

		return payload, session, nil
	}

	return payload, nil, nil
}

// IsCompleted returns whether the handshake is complete
//
//export ToxNoiseIsCompleted
func (nh *NoiseHandshake) IsCompleted() bool {
	return nh.completed
}

// EncryptMessage encrypts a message using the established session
//
//export ToxNoiseEncryptMessage
func (ns *NoiseSession) EncryptMessage(plaintext []byte) ([]byte, error) {
	if ns.SendCipher == nil {
		return nil, errors.New("session not established")
	}

	ciphertext, err := ns.SendCipher.Encrypt(nil, nil, plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %w", err)
	}

	ns.LastUsed = time.Now()
	return ciphertext, nil
}

// DecryptMessage decrypts a message using the established session
//
//export ToxNoiseDecryptMessage
func (ns *NoiseSession) DecryptMessage(ciphertext []byte) ([]byte, error) {
	if ns.RecvCipher == nil {
		return nil, errors.New("session not established")
	}

	plaintext, err := ns.RecvCipher.Decrypt(nil, nil, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}

	ns.LastUsed = time.Now()
	return plaintext, nil
}

// derivePublicKey derives a public key from a private key
func derivePublicKey(privateKey [32]byte) []byte {
	// Use the existing FromSecretKey function which properly derives the public key
	keyPair, err := FromSecretKey(privateKey)
	if err != nil {
		// Return zero key on error - this should be handled at a higher level
		return make([]byte, 32)
	}
	return keyPair.Public[:]
}

// SessionManager manages multiple Noise sessions
//
//export ToxSessionManager
type SessionManager struct {
	sessions map[string]*NoiseSession
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
//
//export ToxNewSessionManager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*NoiseSession),
	}
}

// AddSession adds a session for a peer
//
//export ToxAddSession
func (sm *SessionManager) AddSession(peerKey [32]byte, session *NoiseSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	peerID := fmt.Sprintf("%x", peerKey)
	sm.sessions[peerID] = session
}

// GetSession retrieves a session for a peer
//
//export ToxGetSession
func (sm *SessionManager) GetSession(peerKey [32]byte) (*NoiseSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	peerID := fmt.Sprintf("%x", peerKey)
	session, exists := sm.sessions[peerID]
	return session, exists
}

// RemoveSession removes a session for a peer
//
//export ToxRemoveSession
func (sm *SessionManager) RemoveSession(peerKey [32]byte) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	peerID := fmt.Sprintf("%x", peerKey)
	delete(sm.sessions, peerID)
}

// CleanupExpiredSessions removes old sessions
//
//export ToxCleanupExpiredSessions
func (sm *SessionManager) CleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for peerID, session := range sm.sessions {
		// Remove sessions older than 48 hours
		if now.Sub(session.LastUsed) > 48*time.Hour {
			delete(sm.sessions, peerID)
		}
	}
}
