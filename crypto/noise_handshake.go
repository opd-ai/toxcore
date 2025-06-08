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
	SendCipher    *noise.CipherState
	RecvCipher    *noise.CipherState
	StaticKeys    *KeyPair
	EphemeralKeys *KeyPair
	PeerKey       [32]byte
	Established   time.Time
	LastUsed      time.Time
	RekeyNeeded   bool
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

	// Create handshake state
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   cs,
		Random:        rand.Reader,
		Pattern:       handshakePattern,
		Initiator:     isInitiator,
		StaticKeypair: noise.DHKey{Private: staticKey[:], Public: derivePublicKey(staticKey)},
		PeerStatic:    peerKey[:],
	})
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

// NeedsRekey checks if the session needs rekeying
//
//export ToxNoiseNeedsRekey
func (ns *NoiseSession) NeedsRekey() bool {
	// Rekey after 24 hours or if explicitly marked
	return ns.RekeyNeeded || time.Since(ns.LastUsed) > 24*time.Hour
}

// ProtocolVersion represents supported protocol versions
type ProtocolVersion struct {
	Major uint8 `json:"major"`
	Minor uint8 `json:"minor"`
	Patch uint8 `json:"patch"`
}

// String returns the string representation of the protocol version.
func (pv ProtocolVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", pv.Major, pv.Minor, pv.Patch)
}

// Compare compares two protocol versions.
// Returns -1 if pv < other, 0 if pv == other, 1 if pv > other.
func (pv ProtocolVersion) Compare(other ProtocolVersion) int {
	if pv.Major != other.Major {
		if pv.Major < other.Major {
			return -1
		}
		return 1
	}
	if pv.Minor != other.Minor {
		if pv.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if pv.Patch != other.Patch {
		if pv.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// IsCompatibleWith checks if this version is compatible with another version.
// Compatible means same major version and this version >= other version.
func (pv ProtocolVersion) IsCompatibleWith(other ProtocolVersion) bool {
	return pv.Major == other.Major && pv.Compare(other) >= 0
}

// ProtocolCapabilities advertises node capabilities
//
//export ToxProtocolCapabilities
type ProtocolCapabilities struct {
	// MinVersion is the minimum protocol version this client supports
	MinVersion ProtocolVersion `json:"min_version"`
	// MaxVersion is the maximum protocol version this client supports
	MaxVersion ProtocolVersion `json:"max_version"`
	// SupportedCiphers lists the encryption ciphers this client supports
	SupportedCiphers []string `json:"supported_ciphers"`
	// NoiseSupported indicates if this client supports Noise protocol
	NoiseSupported bool `json:"noise_supported"`
	// LegacySupported indicates if this client supports legacy encryption
	LegacySupported bool `json:"legacy_supported"`
	// Extensions lists additional protocol extensions supported
	Extensions []string `json:"extensions,omitempty"`
}

// NewProtocolCapabilities creates default capabilities
//
//export ToxNewProtocolCapabilities
func NewProtocolCapabilities() *ProtocolCapabilities {
	return &ProtocolCapabilities{
		MinVersion: ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
		MaxVersion: ProtocolVersion{Major: 2, Minor: 0, Patch: 0},
		SupportedCiphers: []string{
			"Noise_IK_25519_ChaChaPoly_SHA256",
			"Noise_IK_25519_AESGCM_SHA256",
			"legacy",
		},
		NoiseSupported:  true,
		LegacySupported: true,
		Extensions:      []string{},
	}
}

// SelectBestProtocol chooses the best mutual protocol version
//
//export ToxSelectBestProtocol
func SelectBestProtocol(local, remote *ProtocolCapabilities) (ProtocolVersion, string, error) {
	if local == nil || remote == nil {
		return ProtocolVersion{}, "", errors.New("capabilities cannot be nil")
	}

	// Find the highest mutually supported protocol version
	var selectedVersion ProtocolVersion
	var versionFound bool

	// Check if we can find a compatible version
	// Start from the local max version and work down
	for major := local.MaxVersion.Major; major >= local.MinVersion.Major; major-- {
		for minor := local.MaxVersion.Minor; minor >= 0; minor-- {
			for patch := local.MaxVersion.Patch; patch >= 0; patch-- {
				candidateVersion := ProtocolVersion{Major: major, Minor: minor, Patch: patch}

				// Check if this version is within local range
				if candidateVersion.Compare(local.MinVersion) < 0 {
					continue
				}

				// Check if this version is compatible with remote
				if candidateVersion.Compare(remote.MinVersion) >= 0 &&
					candidateVersion.Compare(remote.MaxVersion) <= 0 {
					selectedVersion = candidateVersion
					versionFound = true
					break
				}
			}
			if versionFound {
				break
			}
		}
		if versionFound {
			break
		}
	}

	if !versionFound {
		return ProtocolVersion{}, "", errors.New("no compatible protocol version found")
	}

	// Select the best mutual cipher based on version
	var selectedCipher string

	// For version 2.x, prefer Noise protocol ciphers
	if selectedVersion.Major >= 2 && local.NoiseSupported && remote.NoiseSupported {
		// Find best mutual cipher for Noise
		preferredOrder := []string{
			"Noise_IK_25519_ChaChaPoly_SHA256",
			"Noise_IK_25519_AESGCM_SHA256",
		}

		for _, preferred := range preferredOrder {
			if containsCipher(local.SupportedCiphers, preferred) &&
				containsCipher(remote.SupportedCiphers, preferred) {
				selectedCipher = preferred
				break
			}
		}
	}

	// Fallback to legacy cipher if no Noise cipher found
	if selectedCipher == "" {
		if local.LegacySupported && remote.LegacySupported &&
			containsCipher(local.SupportedCiphers, "legacy") &&
			containsCipher(remote.SupportedCiphers, "legacy") {
			selectedCipher = "legacy"
		}
	}

	if selectedCipher == "" {
		return ProtocolVersion{}, "", errors.New("no compatible cipher found")
	}

	return selectedVersion, selectedCipher, nil
}

// containsCipher checks if a cipher is in the list of supported ciphers.
func containsCipher(ciphers []string, target string) bool {
	for _, cipher := range ciphers {
		if cipher == target {
			return true
		}
	}
	return false
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
