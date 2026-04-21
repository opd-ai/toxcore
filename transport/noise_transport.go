package transport

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/flynn/noise"
	"github.com/opd-ai/toxcore/crypto"
	toxnoise "github.com/opd-ai/toxcore/noise"
	"github.com/sirupsen/logrus"
)

var (
	// ErrNoiseNotSupported indicates peer doesn't support Noise protocol
	ErrNoiseNotSupported = errors.New("peer does not support noise protocol")
	// ErrNoiseSessionNotFound indicates no active session with peer
	ErrNoiseSessionNotFound = errors.New("noise session not found for peer")
	// ErrHandshakeReplay indicates a replay attack was detected
	ErrHandshakeReplay = errors.New("handshake replay attack detected")
	// ErrHandshakeTooOld indicates handshake timestamp is too old
	ErrHandshakeTooOld = errors.New("handshake timestamp too old")
	// ErrHandshakeFromFuture indicates handshake timestamp is from the future
	ErrHandshakeFromFuture = errors.New("handshake timestamp from future")
	// ErrRekeyRequired indicates the session has exceeded the message threshold
	// and requires a new handshake to establish fresh cipher keys.
	ErrRekeyRequired = errors.New("session rekey required: message count exceeds threshold")
	// ErrNoiseHandshakeFailed indicates that the Noise handshake could not be
	// initiated or completed for a peer. This is a critical security error that
	// prevents secure communication — callers must NOT fall back to unencrypted
	// transmission when this error is returned.
	ErrNoiseHandshakeFailed = errors.New("noise handshake failed: secure channel unavailable")
	// ErrNoiseSessionIncomplete indicates a handshake is in progress but not yet
	// complete. The caller should retry after the handshake completes.
	ErrNoiseSessionIncomplete = errors.New("noise session incomplete: handshake in progress")
)

const (
	// HandshakeMaxAge is the maximum age of a handshake (5 minutes)
	HandshakeMaxAge = 5 * time.Minute
	// HandshakeMaxFutureDrift is the maximum future drift allowed (1 minute)
	HandshakeMaxFutureDrift = 1 * time.Minute
	// NonceCleanupInterval is how often we cleanup old nonces
	NonceCleanupInterval = 10 * time.Minute
	// HandshakeTimeout is the max time for incomplete handshakes (30 seconds)
	HandshakeTimeout = 30 * time.Second
	// SessionIdleTimeout is the max idle time for complete sessions (5 minutes)
	SessionIdleTimeout = 5 * time.Minute
	// SessionCleanupInterval is how often we check for stale sessions (10 seconds)
	SessionCleanupInterval = 10 * time.Second

	// MaxNonceMapSize is the upper bound on the in-memory nonce map.
	// When this limit is reached new handshakes are refused until the periodic
	// cleanup has run, preventing memory exhaustion from handshake floods.
	MaxNonceMapSize = 100_000

	// DefaultRekeyThreshold is the default message count at which to trigger re-keying.
	// Set to 2^32 (4 billion messages) which provides a large safety margin before
	// the 64-bit nonce space (2^64) used by ChaCha20-Poly1305 could be exhausted.
	// This is a conservative threshold to prevent theoretical nonce reuse attacks.
	DefaultRekeyThreshold uint64 = 1 << 32 // 4,294,967,296 messages

	// RekeyWarningThreshold is the message count at which to start warning about
	// upcoming rekey requirement. Set to 90% of the rekey threshold.
	RekeyWarningThreshold uint64 = (1 << 32) * 9 / 10 // ~3.9 billion messages
)

// NoiseSession tracks the handshake and cipher state for a peer connection.
type NoiseSession struct {
	mu         sync.RWMutex // Protects all fields for concurrent access
	handshake  *toxnoise.IKHandshake
	sendCipher *noise.CipherState
	recvCipher *noise.CipherState
	peerAddr   net.Addr
	role       toxnoise.HandshakeRole
	complete   bool
	createdAt  time.Time // Time when session was created
	lastActive time.Time // Time of last activity (send/receive)

	// Version commitment state
	commitmentExchange *VersionCommitmentExchange
	versionCommitted   bool            // True after version commitment exchange completes
	agreedVersion      ProtocolVersion // Mutually agreed and verified version

	// Message counters for nonce exhaustion protection (flynn/noise vulnerability mitigation).
	// ChaCha20-Poly1305 uses a 64-bit counter that must never repeat with the same key.
	// These counters track messages to trigger re-keying before nonce exhaustion.
	sendMessageCount uint64 // Number of messages encrypted with current send cipher
	recvMessageCount uint64 // Number of messages decrypted with current receive cipher
	rekeyThreshold   uint64 // Configurable threshold for triggering re-key (default: 2^32)
}

// NoiseTransport wraps an existing transport with Noise Protocol encryption.
// It provides automatic handshake negotiation and transparent encryption
// for all packet types except handshake packets themselves.
type NoiseTransport struct {
	underlying Transport
	staticPriv []byte                   // Our long-term private key (32 bytes)
	staticPub  []byte                   // Our long-term public key (32 bytes)
	sessions   map[string]*NoiseSession // Key: addr.String()
	sessionsMu sync.RWMutex
	peerKeys   map[string][]byte // Known peer public keys
	peerKeysMu sync.RWMutex
	handlers   map[PacketType]PacketHandler // Handlers for decrypted packets
	handlersMu sync.RWMutex
	// Replay protection — in-memory fallback (bounded to MaxNonceMapSize entries)
	usedNonces         map[[32]byte]int64 // Map of nonce to timestamp
	noncesMu           sync.RWMutex
	nonceStore         *crypto.NonceStore // Persistent nonce store (optional; preferred over usedNonces)
	stopCleanup        chan struct{}       // Signal to stop nonce cleanup goroutine
	stopSessionCleanup chan struct{}       // Signal to stop session cleanup goroutine
	cleanupWg          sync.WaitGroup     // Tracks cleanup goroutines for clean shutdown
	closed             bool               // Track if Close() has been called
	closedMu           sync.Mutex         // Protect closed flag

	// Protocol version for commitment exchange
	protocolVersion ProtocolVersion
}

// NewNoiseTransport creates a transport wrapper that adds Noise-IK encryption.
// staticPrivKey is our long-term Curve25519 private key (32 bytes).
// underlying is the base transport (UDP/TCP) to wrap.
func NewNoiseTransport(underlying Transport, staticPrivKey []byte) (*NoiseTransport, error) {
	logrus.WithFields(logrus.Fields{
		"function":        "NewNoiseTransport",
		"static_key_len":  len(staticPrivKey),
		"underlying_type": fmt.Sprintf("%T", underlying),
	}).Info("Creating new Noise transport")

	if err := validateNoiseTransportInputs(underlying, staticPrivKey); err != nil {
		return nil, err
	}

	keypair, err := generateKeypair(staticPrivKey)
	if err != nil {
		return nil, err
	}

	nt := createNoiseTransportInstance(underlying, staticPrivKey, keypair)
	startNoiseTransportCleanup(nt)
	registerNoiseHandlers(underlying, nt, keypair)

	logrus.WithFields(logrus.Fields{
		"function":            "NewNoiseTransport",
		"public_key":          keypair.Public[:8],
		"handlers_registered": 2,
	}).Info("Noise transport created successfully")

	return nt, nil
}

// validateNoiseTransportInputs validates the inputs for NewNoiseTransport.
func validateNoiseTransportInputs(underlying Transport, staticPrivKey []byte) error {
	if len(staticPrivKey) != 32 {
		logrus.WithFields(logrus.Fields{
			"function":       "NewNoiseTransport",
			"static_key_len": len(staticPrivKey),
			"expected_len":   32,
		}).Error("Invalid static private key length")
		return fmt.Errorf("static private key must be 32 bytes, got %d", len(staticPrivKey))
	}
	if underlying == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewNoiseTransport",
		}).Error("Underlying transport is nil")
		return errors.New("underlying transport cannot be nil")
	}
	return nil
}

// generateKeypair generates a keypair from the private key.
func generateKeypair(staticPrivKey []byte) (*crypto.KeyPair, error) {
	var staticPrivArray [32]byte
	copy(staticPrivArray[:], staticPrivKey)
	keypair, err := crypto.FromSecretKey(staticPrivArray)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewNoiseTransport",
			"error":    err.Error(),
		}).Error("Failed to generate keypair from private key")
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	return keypair, nil
}

// createNoiseTransportInstance creates and initializes a NoiseTransport instance.
func createNoiseTransportInstance(underlying Transport, staticPrivKey []byte, keypair *crypto.KeyPair) *NoiseTransport {
	nt := &NoiseTransport{
		underlying:         underlying,
		staticPriv:         make([]byte, 32),
		staticPub:          make([]byte, 32),
		sessions:           make(map[string]*NoiseSession),
		peerKeys:           make(map[string][]byte),
		handlers:           make(map[PacketType]PacketHandler),
		usedNonces:         make(map[[32]byte]int64),
		stopCleanup:        make(chan struct{}),
		stopSessionCleanup: make(chan struct{}),
		protocolVersion:    ProtocolNoiseIK, // Default to Noise-IK when using NoiseTransport
	}

	copy(nt.staticPriv, staticPrivKey)
	copy(nt.staticPub, keypair.Public[:])

	logrus.WithFields(logrus.Fields{
		"function":      "NewNoiseTransport",
		"public_key":    keypair.Public[:8],
		"session_count": 0,
		"peer_count":    0,
		"handler_count": 0,
	}).Info("Noise transport keys initialized")

	return nt
}

// startNoiseTransportCleanup starts background cleanup goroutines.
func startNoiseTransportCleanup(nt *NoiseTransport) {
	nt.cleanupWg.Add(2)
	go func() {
		defer nt.cleanupWg.Done()
		nt.cleanupOldNonces()
	}()
	go func() {
		defer nt.cleanupWg.Done()
		nt.cleanupStaleSessions()
	}()
}

// registerNoiseHandlers registers Noise protocol packet handlers.
func registerNoiseHandlers(underlying Transport, nt *NoiseTransport, keypair *crypto.KeyPair) {
	underlying.RegisterHandler(PacketNoiseHandshake, nt.handleHandshakePacket)
	underlying.RegisterHandler(PacketNoiseMessage, nt.handleEncryptedPacket)
	underlying.RegisterHandler(PacketVersionCommitment, nt.handleVersionCommitment)
}

// validatePublicKey checks if the provided public key is valid for cryptographic operations.
func (nt *NoiseTransport) validatePublicKey(publicKey []byte) error {
	if len(publicKey) != 32 {
		return fmt.Errorf("public key must be 32 bytes, got %d", len(publicKey))
	}

	// Validate public key is not all zeros (invalid Curve25519 key)
	allZero := true
	for _, b := range publicKey {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return fmt.Errorf("invalid public key: all zeros")
	}

	return nil
}

// validateAddressCompatibility checks if the address type is compatible with the underlying transport.
func (nt *NoiseTransport) validateAddressCompatibility(addr net.Addr) error {
	switch nt.underlying.(type) {
	case *UDPTransport:
		// For UDP transports, check if address can be used with UDP
		if !nt.isUDPCompatible(addr) {
			return fmt.Errorf("address type %T incompatible with UDP transport", addr)
		}
	case *TCPTransport:
		// For TCP transports, check if address can be used with TCP
		if !nt.isTCPCompatible(addr) {
			return fmt.Errorf("address type %T incompatible with TCP transport", addr)
		}
	}
	return nil
}

// isUDPCompatible checks if an address can be used with UDP transport
// by checking the network type from the address
func (nt *NoiseTransport) isUDPCompatible(addr net.Addr) bool {
	network := addr.Network()
	return network == "udp" || network == "udp4" || network == "udp6"
}

// isTCPCompatible checks if an address can be used with TCP transport
// by checking the network type from the address
func (nt *NoiseTransport) isTCPCompatible(addr net.Addr) bool {
	network := addr.Network()
	return network == "tcp" || network == "tcp4" || network == "tcp6"
}

// storePeerKey safely stores the peer's public key in the internal map.
func (nt *NoiseTransport) storePeerKey(addr net.Addr, publicKey []byte) {
	nt.peerKeysMu.Lock()
	key := make([]byte, 32)
	copy(key, publicKey)
	nt.peerKeys[addr.String()] = key
	nt.peerKeysMu.Unlock()
}

// AddPeer registers a peer's public key for future encrypted communication.
// This enables us to initiate Noise-IK handshakes with known peers.
func (nt *NoiseTransport) AddPeer(addr net.Addr, publicKey []byte) error {
	if err := nt.validatePublicKey(publicKey); err != nil {
		return err
	}

	if err := nt.validateAddressCompatibility(addr); err != nil {
		return err
	}

	nt.storePeerKey(addr, publicKey)
	return nil
}

// Send sends a packet with automatic encryption if Noise session exists.
// Handshake packets are sent unencrypted, all others use Noise encryption.
func (nt *NoiseTransport) Send(packet *Packet, addr net.Addr) error {
	if packet.PacketType == PacketNoiseHandshake {
		// Handshake packets are never encrypted
		return nt.underlying.Send(packet, addr)
	}

	addrKey := addr.String()
	nt.sessionsMu.RLock()
	session, exists := nt.sessions[addrKey]
	nt.sessionsMu.RUnlock()

	if !exists || !session.IsComplete() {
		// Try to initiate handshake for known peers
		if err := nt.initiateHandshake(addr); err != nil {
			// SECURITY: Return error instead of silent unencrypted fallback.
			// Callers must handle this error appropriately — either retry after
			// backoff, use a different transport, or notify the user that secure
			// communication is not possible. Silent downgrade to cleartext is a
			// critical security vulnerability (CVE-class: protocol downgrade).
			logrus.WithFields(logrus.Fields{
				"peer":  addr.String(),
				"error": err,
			}).Warn("Noise handshake failed - refusing to send unencrypted")
			return fmt.Errorf("%w: %v", ErrNoiseHandshakeFailed, err)
		}
		// Handshake initiated but not yet complete. Return error to prevent
		// unencrypted transmission while handshake is in progress.
		logrus.WithField("peer", addr.String()).Debug("Noise handshake in progress")
		return ErrNoiseSessionIncomplete
	}

	// Update session activity timestamp
	session.mu.Lock()
	session.lastActive = time.Now()
	session.mu.Unlock()

	// Encrypt packet using Noise cipher
	encryptedPacket, err := nt.encryptPacket(packet, session)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	return nt.underlying.Send(encryptedPacket, addr)
}

// Close shuts down the transport and cleans up all sessions.
func (nt *NoiseTransport) Close() error {
	// Make Close() idempotent - safe to call multiple times
	nt.closedMu.Lock()
	if nt.closed {
		nt.closedMu.Unlock()
		return nil
	}
	nt.closed = true
	nt.closedMu.Unlock()

	// Stop cleanup goroutines
	close(nt.stopCleanup)
	close(nt.stopSessionCleanup)

	// Wait for cleanup goroutines to finish
	nt.cleanupWg.Wait()

	nt.sessionsMu.Lock()
	nt.sessions = make(map[string]*NoiseSession)
	nt.sessionsMu.Unlock()

	nt.noncesMu.Lock()
	nt.usedNonces = make(map[[32]byte]int64)
	nt.noncesMu.Unlock()

	// Persist the nonce store so nonces survive across restarts.
	if nt.nonceStore != nil {
		if err := nt.nonceStore.Close(); err != nil {
			logrus.WithError(err).Warn("NoiseTransport: failed to save persistent nonce store on close")
		}
	}

	return nt.underlying.Close()
}

// SetNonceDataDir configures the transport to use a persistent nonce store
// backed by the given data directory.  When set, handshake nonces are checked
// and stored in the persistent store instead of the in-memory map, providing
// replay protection that survives process restarts.
//
// Call this before the transport starts receiving handshakes.
func (nt *NoiseTransport) SetNonceDataDir(dataDir string) error {
	ns, err := crypto.NewNonceStore(dataDir)
	if err != nil {
		return fmt.Errorf("failed to create persistent nonce store: %w", err)
	}
	nt.noncesMu.Lock()
	nt.nonceStore = ns
	nt.noncesMu.Unlock()
	return nil
}

// LocalAddr returns the local address from the underlying transport.
func (nt *NoiseTransport) LocalAddr() net.Addr {
	return nt.underlying.LocalAddr()
}

// RegisterHandler registers a handler for decrypted packets.
func (nt *NoiseTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	nt.handlersMu.Lock()
	nt.handlers[packetType] = handler
	nt.handlersMu.Unlock()
}

// initiateHandshake starts a Noise-IK handshake with a known peer.
func (nt *NoiseTransport) initiateHandshake(addr net.Addr) error {
	addrKey := addr.String()

	nt.peerKeysMu.RLock()
	peerPubKey, exists := nt.peerKeys[addrKey]
	nt.peerKeysMu.RUnlock()

	if !exists {
		return ErrNoiseNotSupported
	}

	// Create initiator handshake
	handshake, err := toxnoise.NewIKHandshake(nt.staticPriv, peerPubKey, toxnoise.Initiator)
	if err != nil {
		return fmt.Errorf("failed to create handshake: %w", err)
	}

	// Generate initial message
	message, _, err := handshake.WriteMessage(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to generate handshake message: %w", err)
	}

	// Store session
	now := time.Now()
	nt.sessionsMu.Lock()
	nt.sessions[addrKey] = &NoiseSession{
		handshake:  handshake,
		peerAddr:   addr,
		role:       toxnoise.Initiator,
		complete:   false,
		createdAt:  now,
		lastActive: now,
	}
	nt.sessionsMu.Unlock()

	// Send handshake packet
	packet := &Packet{
		PacketType: PacketNoiseHandshake,
		Data:       message,
	}

	return nt.underlying.Send(packet, addr)
}

// handleHandshakePacket processes incoming Noise handshake packets.
func (nt *NoiseTransport) handleHandshakePacket(packet *Packet, addr net.Addr) error {
	session, err := nt.getOrCreateSession(addr)
	if err != nil {
		return err
	}

	session.mu.RLock()
	isComplete := session.complete
	role := session.role
	session.mu.RUnlock()

	if isComplete {
		return fmt.Errorf("handshake already complete for peer %s", addr)
	}

	if role == toxnoise.Responder {
		return nt.processResponderHandshake(session, packet, addr)
	} else {
		return nt.processInitiatorHandshake(session, packet)
	}
}

// getOrCreateSession retrieves an existing session or creates a new responder session.
func (nt *NoiseTransport) getOrCreateSession(addr net.Addr) (*NoiseSession, error) {
	addrKey := addr.String()

	nt.sessionsMu.RLock()
	session, exists := nt.sessions[addrKey]
	nt.sessionsMu.RUnlock()

	if exists {
		return session, nil
	}

	handshake, err := toxnoise.NewIKHandshake(nt.staticPriv, nil, toxnoise.Responder)
	if err != nil {
		return nil, fmt.Errorf("failed to create responder handshake: %w", err)
	}

	now := time.Now()
	session = &NoiseSession{
		handshake:  handshake,
		peerAddr:   addr,
		role:       toxnoise.Responder,
		complete:   false,
		createdAt:  now,
		lastActive: now,
	}

	nt.sessionsMu.Lock()
	nt.sessions[addrKey] = session
	nt.sessionsMu.Unlock()

	return session, nil
}

// processResponderHandshake handles handshake processing for responder role.
func (nt *NoiseTransport) processResponderHandshake(session *NoiseSession, packet *Packet, addr net.Addr) error {
	session.mu.Lock()
	handshake := session.handshake
	session.mu.Unlock()

	// Validate handshake replay protection
	nonce := handshake.GetNonce()
	timestamp := handshake.GetTimestamp()
	if err := nt.validateHandshakeNonce(nonce, timestamp); err != nil {
		return fmt.Errorf("handshake validation failed: %w", err)
	}

	response, complete, err := handshake.WriteMessage(nil, packet.Data)
	if err != nil {
		return fmt.Errorf("failed to generate handshake response: %w", err)
	}

	if complete {
		if err := nt.completeCipherSetup(session); err != nil {
			return err
		}
	}

	responsePacket := &Packet{
		PacketType: PacketNoiseHandshake,
		Data:       response,
	}
	return nt.underlying.Send(responsePacket, addr)
}

// processInitiatorHandshake handles handshake processing for initiator role.
func (nt *NoiseTransport) processInitiatorHandshake(session *NoiseSession, packet *Packet) error {
	session.mu.Lock()
	handshake := session.handshake
	session.mu.Unlock()

	_, complete, err := handshake.ReadMessage(packet.Data)
	if err != nil {
		return fmt.Errorf("failed to read handshake response: %w", err)
	}

	if complete {
		return nt.completeCipherSetup(session)
	}

	return nil
}

// completeCipherSetup extracts cipher states and marks the session as complete.
func (nt *NoiseTransport) completeCipherSetup(session *NoiseSession) error {
	session.mu.Lock()

	sendCipher, recvCipher, err := session.handshake.GetCipherStates()
	if err != nil {
		session.mu.Unlock()
		return fmt.Errorf("failed to get cipher states: %w", err)
	}

	session.sendCipher = sendCipher
	session.recvCipher = recvCipher
	session.complete = true

	// Get handshake hash for version commitment binding
	// Use the handshake nonce as a proxy for transcript hash
	nonce := session.handshake.GetNonce()
	handshakeHash := nonce[:]

	// Initialize version commitment exchange
	exchange, err := NewVersionCommitmentExchange(nt.protocolVersion, handshakeHash)
	if err != nil {
		session.mu.Unlock()
		logrus.WithError(err).Warn("Failed to create version commitment exchange")
		return nil // Don't fail handshake, commitment is defense-in-depth
	}
	session.commitmentExchange = exchange

	session.mu.Unlock()

	return nil
}

// sendVersionCommitment sends our version commitment to the peer.
func (nt *NoiseTransport) sendVersionCommitment(session *NoiseSession, addr net.Addr) error {
	session.mu.RLock()
	if session.commitmentExchange == nil {
		session.mu.RUnlock()
		return errors.New("commitment exchange not initialized")
	}

	commitmentData, err := session.commitmentExchange.GetLocalCommitment()
	session.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to serialize commitment: %w", err)
	}

	packet := &Packet{
		PacketType: PacketVersionCommitment,
		Data:       commitmentData,
	}

	// Encrypt the commitment packet using the Noise session
	encryptedPacket, err := nt.encryptPacket(packet, session)
	if err != nil {
		return fmt.Errorf("failed to encrypt commitment: %w", err)
	}

	return nt.underlying.Send(encryptedPacket, addr)
}

// handleVersionCommitment processes incoming version commitment packets.
func (nt *NoiseTransport) handleVersionCommitment(packet *Packet, addr net.Addr) error {
	session, err := nt.getCompleteSession(addr)
	if err != nil {
		return err
	}

	decryptedData, err := session.Decrypt(packet.Data)
	if err != nil {
		return fmt.Errorf("failed to decrypt version commitment: %w", err)
	}

	commitmentData, err := extractCommitmentData(decryptedData)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if err := nt.ensureCommitmentExchange(session); err != nil {
		return err
	}

	if err := nt.verifyPeerCommitment(session, commitmentData, addr); err != nil {
		return err
	}

	nt.finalizeVersionCommitment(session, addr)
	return nil
}

// getCompleteSession retrieves and validates a complete session for the given address.
func (nt *NoiseTransport) getCompleteSession(addr net.Addr) (*NoiseSession, error) {
	addrKey := addr.String()
	nt.sessionsMu.RLock()
	session, exists := nt.sessions[addrKey]
	nt.sessionsMu.RUnlock()

	if !exists || !session.IsComplete() {
		return nil, errors.New("no complete session for version commitment")
	}
	return session, nil
}

// extractCommitmentData extracts commitment data from decrypted packet.
func extractCommitmentData(decryptedData []byte) ([]byte, error) {
	if len(decryptedData) < 2 {
		return nil, errors.New("decrypted commitment packet too short")
	}
	return decryptedData[1:], nil
}

// ensureCommitmentExchange creates a commitment exchange if one doesn't exist.
func (nt *NoiseTransport) ensureCommitmentExchange(session *NoiseSession) error {
	if session.commitmentExchange != nil {
		return nil
	}

	nonce := session.handshake.GetNonce()
	exchange, err := NewVersionCommitmentExchange(nt.protocolVersion, nonce[:])
	if err != nil {
		return fmt.Errorf("failed to create commitment exchange: %w", err)
	}
	session.commitmentExchange = exchange
	return nil
}

// verifyPeerCommitment validates the peer's version commitment.
func (nt *NoiseTransport) verifyPeerCommitment(session *NoiseSession, commitmentData []byte, addr net.Addr) error {
	if err := session.commitmentExchange.ProcessPeerCommitment(commitmentData); err != nil {
		logrus.WithFields(logrus.Fields{
			"peer":  addr.String(),
			"error": err.Error(),
		}).Warn("Version commitment verification failed - potential downgrade attack")
		return fmt.Errorf("version commitment failed: %w", err)
	}
	return nil
}

// finalizeVersionCommitment completes the version commitment exchange.
func (nt *NoiseTransport) finalizeVersionCommitment(session *NoiseSession, addr net.Addr) {
	session.versionCommitted = true
	session.agreedVersion = nt.protocolVersion

	logrus.WithFields(logrus.Fields{
		"peer":    addr.String(),
		"version": session.agreedVersion.String(),
	}).Info("Version commitment exchange complete")
}

// handleEncryptedPacket processes incoming encrypted Noise messages.
func (nt *NoiseTransport) handleEncryptedPacket(packet *Packet, addr net.Addr) error {
	addrKey := addr.String()

	nt.sessionsMu.RLock()
	session, exists := nt.sessions[addrKey]
	nt.sessionsMu.RUnlock()

	if !exists || !session.IsComplete() {
		return ErrNoiseSessionNotFound
	}

	// Update session activity timestamp
	session.mu.Lock()
	session.lastActive = time.Now()
	session.mu.Unlock()

	// Decrypt the packet using thread-safe method
	decryptedData, err := session.Decrypt(packet.Data)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	// Parse the decrypted packet
	if len(decryptedData) < 1 {
		return errors.New("decrypted packet too short")
	}

	decryptedPacket := &Packet{
		PacketType: PacketType(decryptedData[0]),
		Data:       decryptedData[1:],
	}

	// Forward decrypted packet to appropriate handler
	nt.handlersMu.RLock()
	handler, exists := nt.handlers[decryptedPacket.PacketType]
	nt.handlersMu.RUnlock()

	if exists {
		go handler(decryptedPacket, session.peerAddr)
	}

	return nil
}

// encryptPacket encrypts a packet using the session's send cipher.
func (nt *NoiseTransport) encryptPacket(packet *Packet, session *NoiseSession) (*Packet, error) {
	session.mu.RLock()
	if !session.complete {
		session.mu.RUnlock()
		return nil, errors.New("session not complete")
	}

	if session.sendCipher == nil {
		session.mu.RUnlock()
		return nil, errors.New("send cipher not initialized")
	}
	session.mu.RUnlock()

	// Serialize the original packet
	serialized, err := packet.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize packet: %w", err)
	}

	// Encrypt the serialized packet
	session.mu.Lock()
	encrypted, err := session.sendCipher.Encrypt(nil, nil, serialized)
	session.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	return &Packet{
		PacketType: PacketNoiseMessage,
		Data:       encrypted,
	}, nil
}

// validateHandshakeNonce checks if a handshake nonce has been used before (replay attack).
func (nt *NoiseTransport) validateHandshakeNonce(nonce [32]byte, timestamp int64) error {
	now := time.Now().Unix()

	// Check timestamp freshness (within HandshakeMaxAge)
	age := time.Duration(now-timestamp) * time.Second
	if age > HandshakeMaxAge {
		return ErrHandshakeTooOld
	}

	// Check timestamp isn't too far in the future (within HandshakeMaxFutureDrift)
	futureTime := time.Duration(timestamp-now) * time.Second
	if futureTime > HandshakeMaxFutureDrift {
		return ErrHandshakeFromFuture
	}

	// Use the persistent nonce store when available — this survives process restarts
	// and prevents replay attacks on fresh start within the HandshakeMaxAge window.
	nt.noncesMu.RLock()
	store := nt.nonceStore
	nt.noncesMu.RUnlock()

	if store != nil {
		if !store.CheckAndStore(nonce, timestamp) {
			return ErrHandshakeReplay
		}
		return nil
	}

	// Fallback: in-memory map with a size cap to prevent memory exhaustion.
	nt.noncesMu.Lock()
	defer nt.noncesMu.Unlock()

	if _, used := nt.usedNonces[nonce]; used {
		return ErrHandshakeReplay
	}

	if len(nt.usedNonces) >= MaxNonceMapSize {
		logrus.Warn("NoiseTransport: nonce map at capacity — dropping handshake to prevent memory exhaustion; consider calling SetNonceDataDir")
		return ErrHandshakeReplay
	}

	nt.usedNonces[nonce] = now
	return nil
}

// cleanupOldNonces periodically removes old nonces to prevent memory growth.
func (nt *NoiseTransport) cleanupOldNonces() {
	ticker := time.NewTicker(NonceCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			nt.performNonceCleanup()
		case <-nt.stopCleanup:
			return
		}
	}
}

// cleanupStaleSessions periodically removes stale sessions (incomplete or idle).
func (nt *NoiseTransport) cleanupStaleSessions() {
	ticker := time.NewTicker(SessionCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			nt.performSessionCleanup()
		case <-nt.stopSessionCleanup:
			return
		}
	}
}

// performSessionCleanup removes stale sessions based on timeouts.
func (nt *NoiseTransport) performSessionCleanup() {
	nt.sessionsMu.Lock()
	defer nt.sessionsMu.Unlock()

	now := time.Now()
	removed := 0

	for addrKey, session := range nt.sessions {
		if nt.shouldRemoveSession(session, now) {
			delete(nt.sessions, addrKey)
			removed++
		}
	}

	if removed > 0 {
		logrus.WithField("removed_count", removed).Debug("Session cleanup completed")
	}
}

// shouldRemoveSession determines if a session should be removed based on timeouts.
func (nt *NoiseTransport) shouldRemoveSession(session *NoiseSession, now time.Time) bool {
	session.mu.RLock()
	defer session.mu.RUnlock()

	if !session.complete {
		return nt.isHandshakeTimedOut(session, now)
	}
	return nt.isSessionIdle(session, now)
}

// isHandshakeTimedOut checks if an incomplete handshake has exceeded the timeout.
func (nt *NoiseTransport) isHandshakeTimedOut(session *NoiseSession, now time.Time) bool {
	if now.Sub(session.createdAt) > HandshakeTimeout {
		logrus.WithFields(logrus.Fields{
			"age":     now.Sub(session.createdAt),
			"timeout": HandshakeTimeout,
		}).Info("Removing incomplete handshake session (timeout)")
		return true
	}
	return false
}

// isSessionIdle checks if a complete session has been idle too long.
func (nt *NoiseTransport) isSessionIdle(session *NoiseSession, now time.Time) bool {
	if now.Sub(session.lastActive) > SessionIdleTimeout {
		logrus.WithFields(logrus.Fields{
			"idle":    now.Sub(session.lastActive),
			"timeout": SessionIdleTimeout,
		}).Info("Removing idle session (timeout)")
		return true
	}
	return false
}

// performNonceCleanup removes nonces older than HandshakeMaxAge.
func (nt *NoiseTransport) performNonceCleanup() {
	now := time.Now().Unix()
	cutoff := now - int64(HandshakeMaxAge.Seconds())

	nt.noncesMu.Lock()
	defer nt.noncesMu.Unlock()

	for nonce, timestamp := range nt.usedNonces {
		if timestamp < cutoff {
			delete(nt.usedNonces, nonce)
		}
	}
}

// IsComplete returns whether the session handshake is complete.
func (ns *NoiseSession) IsComplete() bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.complete
}

// checkRekeyThreshold validates message count against rekey threshold.
// Logs a warning when approaching the threshold. Returns error if threshold exceeded.
// Caller must hold ns.mu.
func (ns *NoiseSession) checkRekeyThreshold(msgCount uint64, direction string) (uint64, error) {
	threshold := ns.rekeyThreshold
	if threshold == 0 {
		threshold = DefaultRekeyThreshold
	}
	if msgCount >= threshold {
		return 0, ErrRekeyRequired
	}
	if msgCount == RekeyWarningThreshold {
		logrus.WithFields(logrus.Fields{
			"function":      "NoiseSession." + direction,
			"peer_addr":     ns.peerAddr.String(),
			"message_count": msgCount,
			"threshold":     threshold,
		}).Warn("Approaching rekey threshold, session re-handshake recommended")
	}
	return threshold, nil
}

// doCipherOp performs a cipher operation (encrypt/decrypt) with common validation.
// Caller must NOT hold ns.mu. The function handles locking internally.
func (ns *NoiseSession) doCipherOp(
	data []byte,
	cipher **noise.CipherState,
	msgCount *uint64,
	direction string,
	cipherNilErr string,
	op func(cs *noise.CipherState, data []byte) ([]byte, error),
) ([]byte, error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if !ns.complete {
		return nil, errors.New("handshake not complete")
	}

	if _, err := ns.checkRekeyThreshold(*msgCount, direction); err != nil {
		return nil, err
	}

	if *cipher == nil {
		return nil, errors.New(cipherNilErr)
	}

	result, err := op(*cipher, data)
	if err != nil {
		return nil, err
	}

	(*msgCount)++
	return result, nil
}

// Encrypt encrypts data using the session's send cipher.
// Returns ErrRekeyRequired if the message count exceeds the rekey threshold,
// indicating that a new handshake should be performed before continuing.
func (ns *NoiseSession) Encrypt(plaintext []byte) ([]byte, error) {
	return ns.doCipherOp(
		plaintext,
		&ns.sendCipher,
		&ns.sendMessageCount,
		"Encrypt",
		"send cipher not initialized",
		func(cs *noise.CipherState, data []byte) ([]byte, error) {
			return cs.Encrypt(nil, nil, data)
		},
	)
}

// Decrypt decrypts data using the session's receive cipher.
// Returns ErrRekeyRequired if the message count exceeds the rekey threshold,
// indicating that a new handshake should be performed before continuing.
func (ns *NoiseSession) Decrypt(ciphertext []byte) ([]byte, error) {
	return ns.doCipherOp(
		ciphertext,
		&ns.recvCipher,
		&ns.recvMessageCount,
		"Decrypt",
		"receive cipher not initialized",
		func(cs *noise.CipherState, data []byte) ([]byte, error) {
			return cs.Decrypt(nil, nil, data)
		},
	)
}

// IsConnectionOriented delegates to the underlying transport.
func (nt *NoiseTransport) IsConnectionOriented() bool {
	return nt.underlying.IsConnectionOriented()
}

// IsVersionCommitted returns whether the version commitment exchange is complete for a peer.
func (ns *NoiseSession) IsVersionCommitted() bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.versionCommitted
}

// GetAgreedVersion returns the mutually verified protocol version after commitment exchange.
func (ns *NoiseSession) GetAgreedVersion() (ProtocolVersion, bool) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if !ns.versionCommitted {
		return ProtocolLegacy, false
	}
	return ns.agreedVersion, true
}

// SetProtocolVersion configures the protocol version for commitment exchange.
func (nt *NoiseTransport) SetProtocolVersion(version ProtocolVersion) {
	nt.protocolVersion = version
}

// NeedsRekey returns true if the session has reached or exceeded the rekey threshold
// for either send or receive message counts. This indicates a new handshake should
// be performed to establish fresh cipher keys.
func (ns *NoiseSession) NeedsRekey() bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	threshold := ns.rekeyThreshold
	if threshold == 0 {
		threshold = DefaultRekeyThreshold
	}

	return ns.sendMessageCount >= threshold || ns.recvMessageCount >= threshold
}

// NeedsRekeyWarning returns true if the session is approaching the rekey threshold.
// This can be used to proactively initiate a new handshake before hitting the limit.
func (ns *NoiseSession) NeedsRekeyWarning() bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return ns.sendMessageCount >= RekeyWarningThreshold || ns.recvMessageCount >= RekeyWarningThreshold
}

// GetMessageCounts returns the current send and receive message counts.
func (ns *NoiseSession) GetMessageCounts() (send, recv uint64) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.sendMessageCount, ns.recvMessageCount
}

// SetRekeyThreshold sets a custom rekey threshold for the session.
// A value of 0 uses the DefaultRekeyThreshold.
func (ns *NoiseSession) SetRekeyThreshold(threshold uint64) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.rekeyThreshold = threshold
}

// GetRekeyThreshold returns the configured rekey threshold (or default if not set).
func (ns *NoiseSession) GetRekeyThreshold() uint64 {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.rekeyThreshold == 0 {
		return DefaultRekeyThreshold
	}
	return ns.rekeyThreshold
}
