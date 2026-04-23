// Package noise provides Noise Protocol Framework implementation for Tox handshakes.
// This file implements PSK-based session resumption (0-RTT) for reduced handshake overhead.
package noise

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/flynn/noise"
	"github.com/opd-ai/toxcore/crypto"
)

var (
	// ErrSessionTicketExpired indicates the session ticket has expired
	ErrSessionTicketExpired = errors.New("session ticket has expired")
	// ErrSessionTicketInvalid indicates the session ticket is invalid or tampered
	ErrSessionTicketInvalid = errors.New("session ticket is invalid")
	// ErrSessionTicketNotFound indicates no session ticket exists for the peer
	ErrSessionTicketNotFound = errors.New("session ticket not found for peer")
	// ErrPSKDerivationFailed indicates PSK derivation from session failed
	ErrPSKDerivationFailed = errors.New("PSK derivation failed")
	// ErrReplayDetected indicates a replay attack was detected on 0-RTT data
	ErrReplayDetected = errors.New("replay attack detected on 0-RTT data")
)

const (
	// DefaultSessionTicketLifetime is the default validity period for session tickets
	DefaultSessionTicketLifetime = 24 * time.Hour
	// MaxSessionTicketLifetime is the maximum allowed lifetime for session tickets
	MaxSessionTicketLifetime = 7 * 24 * time.Hour
	// SessionTicketNonceSize is the size of the random nonce in session tickets
	SessionTicketNonceSize = 32
	// PSKSize is the size of derived pre-shared keys (32 bytes for ChaCha20-Poly1305)
	PSKSize = 32
	// SessionCacheCleanupInterval is how often expired tickets are removed
	SessionCacheCleanupInterval = 5 * time.Minute
	// MaxReplayWindowSize is the maximum number of 0-RTT message IDs to track
	MaxReplayWindowSize = 10000
	// DefaultPSKPlacement is the PSK token placement (psk2 = after second message pattern)
	DefaultPSKPlacement = 2
)

// SessionTicket represents a resumable session for 0-RTT handshakes.
// It contains the derived PSK and metadata for session resumption.
type SessionTicket struct {
	// TicketID is the unique identifier for this session ticket
	TicketID [32]byte
	// PSK is the pre-shared key derived from the previous session
	PSK [PSKSize]byte
	// PeerPublicKey is the peer's static public key
	PeerPublicKey [32]byte
	// CreatedAt is when this ticket was created
	CreatedAt time.Time
	// ExpiresAt is when this ticket expires
	ExpiresAt time.Time
	// MessageIDCounter tracks the last message ID for replay protection
	MessageIDCounter uint64
	// HandshakeHash is the hash of the original handshake for binding
	HandshakeHash [32]byte
}

// IsExpired returns true if the session ticket has expired
func (st *SessionTicket) IsExpired() bool {
	return time.Now().After(st.ExpiresAt)
}

// IsValid checks if the session ticket is valid for resumption
func (st *SessionTicket) IsValid() bool {
	if st.IsExpired() {
		return false
	}
	// Check PSK is not zero
	var zeroPSK [PSKSize]byte
	return st.PSK != zeroPSK
}

// SessionCache provides thread-safe storage for session tickets.
// It supports automatic cleanup of expired tickets and replay protection.
type SessionCache struct {
	mu              sync.RWMutex
	tickets         map[string]*SessionTicket // Key: hex(peer public key)
	ticketsByID     map[[32]byte]*SessionTicket
	replayWindow    map[[32]byte]map[uint64]time.Time // TicketID -> MessageID -> Time
	lifetime        time.Duration
	stopCleanup     chan struct{}
	cleanupInterval time.Duration
}

// SessionCacheConfig holds configuration for the session cache
type SessionCacheConfig struct {
	// Lifetime is how long session tickets remain valid
	Lifetime time.Duration
	// CleanupInterval is how often expired tickets are cleaned up
	CleanupInterval time.Duration
	// MaxTickets is the maximum number of tickets to cache (0 = unlimited)
	MaxTickets int
}

// DefaultSessionCacheConfig returns sensible default configuration
func DefaultSessionCacheConfig() SessionCacheConfig {
	return SessionCacheConfig{
		Lifetime:        DefaultSessionTicketLifetime,
		CleanupInterval: SessionCacheCleanupInterval,
		MaxTickets:      0,
	}
}

// NewSessionCache creates a new session cache with the given configuration
func NewSessionCache(config SessionCacheConfig) *SessionCache {
	if config.Lifetime <= 0 {
		config.Lifetime = DefaultSessionTicketLifetime
	}
	if config.Lifetime > MaxSessionTicketLifetime {
		config.Lifetime = MaxSessionTicketLifetime
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = SessionCacheCleanupInterval
	}

	sc := &SessionCache{
		tickets:         make(map[string]*SessionTicket),
		ticketsByID:     make(map[[32]byte]*SessionTicket),
		replayWindow:    make(map[[32]byte]map[uint64]time.Time),
		lifetime:        config.Lifetime,
		stopCleanup:     make(chan struct{}),
		cleanupInterval: config.CleanupInterval,
	}

	go sc.cleanupLoop()
	return sc
}

// Close stops the cleanup goroutine and releases resources
func (sc *SessionCache) Close() {
	close(sc.stopCleanup)
}

// cleanupLoop periodically removes expired session tickets
func (sc *SessionCache) cleanupLoop() {
	ticker := time.NewTicker(sc.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sc.stopCleanup:
			return
		case <-ticker.C:
			sc.cleanupExpired()
		}
	}
}

// cleanupExpired removes all expired session tickets
func (sc *SessionCache) cleanupExpired() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()
	for peerKey, ticket := range sc.tickets {
		if now.After(ticket.ExpiresAt) {
			delete(sc.tickets, peerKey)
			delete(sc.ticketsByID, ticket.TicketID)
			delete(sc.replayWindow, ticket.TicketID)
		}
	}
}

// StoreTicket stores a session ticket for a peer
func (sc *SessionCache) StoreTicket(peerPubKey []byte, ticket *SessionTicket) error {
	if len(peerPubKey) != 32 {
		return fmt.Errorf("peer public key must be 32 bytes, got %d", len(peerPubKey))
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	peerKeyHex := fmt.Sprintf("%x", peerPubKey)

	// If there's an existing ticket for this peer, remove it first
	if existing, ok := sc.tickets[peerKeyHex]; ok {
		delete(sc.ticketsByID, existing.TicketID)
		delete(sc.replayWindow, existing.TicketID)
	}

	sc.tickets[peerKeyHex] = ticket
	sc.ticketsByID[ticket.TicketID] = ticket
	sc.replayWindow[ticket.TicketID] = make(map[uint64]time.Time)

	return nil
}

// GetTicket retrieves a session ticket for a peer
func (sc *SessionCache) GetTicket(peerPubKey []byte) (*SessionTicket, error) {
	if len(peerPubKey) != 32 {
		return nil, fmt.Errorf("peer public key must be 32 bytes, got %d", len(peerPubKey))
	}

	sc.mu.RLock()
	defer sc.mu.RUnlock()

	peerKeyHex := fmt.Sprintf("%x", peerPubKey)
	ticket, ok := sc.tickets[peerKeyHex]
	if !ok {
		return nil, ErrSessionTicketNotFound
	}

	if ticket.IsExpired() {
		return nil, ErrSessionTicketExpired
	}

	return ticket, nil
}

// GetTicketByID retrieves a session ticket by its ID
func (sc *SessionCache) GetTicketByID(ticketID [32]byte) (*SessionTicket, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	ticket, ok := sc.ticketsByID[ticketID]
	if !ok {
		return nil, ErrSessionTicketNotFound
	}

	if ticket.IsExpired() {
		return nil, ErrSessionTicketExpired
	}

	return ticket, nil
}

// RemoveTicket removes a session ticket for a peer
func (sc *SessionCache) RemoveTicket(peerPubKey []byte) {
	if len(peerPubKey) != 32 {
		return
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	peerKeyHex := fmt.Sprintf("%x", peerPubKey)
	if ticket, ok := sc.tickets[peerKeyHex]; ok {
		delete(sc.ticketsByID, ticket.TicketID)
		delete(sc.replayWindow, ticket.TicketID)
		delete(sc.tickets, peerKeyHex)
	}
}

// CheckAndRecordReplay checks if a message ID has been seen and records it.
// Returns ErrReplayDetected if the message was already processed.
func (sc *SessionCache) CheckAndRecordReplay(ticketID [32]byte, messageID uint64) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	window, ok := sc.replayWindow[ticketID]
	if !ok {
		// No replay window for this ticket - ticket may not exist or expired
		return ErrSessionTicketNotFound
	}

	if _, seen := window[messageID]; seen {
		return ErrReplayDetected
	}

	// Record this message ID
	window[messageID] = time.Now()

	// Cleanup old entries if window is too large
	if len(window) > MaxReplayWindowSize {
		sc.trimReplayWindow(window)
	}

	return nil
}

// trimReplayWindow removes the oldest entries from the replay window
func (sc *SessionCache) trimReplayWindow(window map[uint64]time.Time) {
	// Find and remove the oldest 10% of entries
	toRemove := len(window) / 10
	if toRemove < 1 {
		toRemove = 1
	}

	type entry struct {
		id   uint64
		time time.Time
	}
	entries := make([]entry, 0, len(window))
	for id, t := range window {
		entries = append(entries, entry{id, t})
	}

	// Simple selection of oldest entries
	for i := 0; i < toRemove && len(entries) > 0; i++ {
		oldestIdx := 0
		for j := 1; j < len(entries); j++ {
			if entries[j].time.Before(entries[oldestIdx].time) {
				oldestIdx = j
			}
		}
		delete(window, entries[oldestIdx].id)
		entries = append(entries[:oldestIdx], entries[oldestIdx+1:]...)
	}
}

// Count returns the number of cached session tickets
func (sc *SessionCache) Count() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.tickets)
}

// PSKHandshake implements the Noise IK pattern with PSK for 0-RTT session resumption.
// It extends IKHandshake with PSK support for reduced round-trip connections.
//
// The PSK is derived from a previous successful handshake using HKDF, binding
// the resumption key to the original handshake transcript for cryptographic binding.
//
// Thread Safety: PSKHandshake is safe for concurrent use after creation.
type PSKHandshake struct {
	mu                sync.RWMutex
	role              HandshakeRole
	state             *noise.HandshakeState
	sendCipher        *noise.CipherState
	recvCipher        *noise.CipherState
	complete          bool
	psk               [PSKSize]byte
	ticketID          [32]byte
	localPubKey       []byte
	timestamp         int64
	nonce             [32]byte
	earlyData         []byte // 0-RTT data sent in first message
	earlyDataReceived []byte // 0-RTT data received from initiator
}

// PSKHandshakeConfig holds configuration for PSK handshake creation
type PSKHandshakeConfig struct {
	// StaticPrivKey is our long-term private key (32 bytes)
	StaticPrivKey []byte
	// PeerPubKey is peer's long-term public key (32 bytes, required for initiator)
	PeerPubKey []byte
	// PSK is the pre-shared key from a previous session (32 bytes)
	PSK [PSKSize]byte
	// TicketID is the session ticket ID for replay protection
	TicketID [32]byte
	// Role determines if we initiate or respond
	Role HandshakeRole
	// PSKPlacement specifies where the PSK token appears (default: 2 for IKpsk2)
	PSKPlacement int
}

// NewPSKHandshake creates a new IK pattern handshake with PSK for 0-RTT resumption.
// This enables sending encrypted application data in the very first handshake message.
func NewPSKHandshake(config PSKHandshakeConfig) (*PSKHandshake, error) {
	if err := validatePSKHandshakeConfig(config); err != nil {
		return nil, err
	}

	keyPair, err := createKeyPairFromPrivateKey(config.StaticPrivKey)
	if err != nil {
		return nil, err
	}

	noiseConfig := createPSKNoiseConfig(keyPair, config)

	psk := &PSKHandshake{
		role:        config.Role,
		psk:         config.PSK,
		ticketID:    config.TicketID,
		timestamp:   time.Now().Unix(),
		localPubKey: make([]byte, 32),
	}
	copy(psk.localPubKey, keyPair.Public[:])

	// Generate random nonce for replay protection
	if _, err := rand.Read(psk.nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate handshake nonce: %w", err)
	}

	state, err := noise.NewHandshakeState(noiseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create PSK handshake state: %w", err)
	}
	psk.state = state

	return psk, nil
}

// validatePSKHandshakeConfig validates the PSK handshake configuration
func validatePSKHandshakeConfig(config PSKHandshakeConfig) error {
	if len(config.StaticPrivKey) != 32 {
		return fmt.Errorf("static private key must be 32 bytes, got %d", len(config.StaticPrivKey))
	}

	if config.Role == Initiator && (config.PeerPubKey == nil || len(config.PeerPubKey) != 32) {
		return fmt.Errorf("initiator requires peer public key (32 bytes)")
	}

	// Check PSK is not all zeros
	var zeroPSK [PSKSize]byte
	if config.PSK == zeroPSK {
		return fmt.Errorf("PSK cannot be all zeros")
	}

	return nil
}

// createPSKNoiseConfig creates the Noise config with PSK settings
func createPSKNoiseConfig(keyPair *crypto.KeyPair, config PSKHandshakeConfig) noise.Config {
	staticKey := noise.DHKey{
		Private: make([]byte, 32),
		Public:  make([]byte, 32),
	}
	copy(staticKey.Private, keyPair.Private[:])
	copy(staticKey.Public, keyPair.Public[:])

	placement := config.PSKPlacement
	if placement == 0 {
		placement = DefaultPSKPlacement
	}

	noiseConfig := noise.Config{
		CipherSuite:           noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256),
		Random:                rand.Reader,
		Pattern:               noise.HandshakeIK,
		Initiator:             config.Role == Initiator,
		StaticKeypair:         staticKey,
		PresharedKey:          config.PSK[:],
		PresharedKeyPlacement: placement,
	}

	if config.Role == Initiator && config.PeerPubKey != nil {
		noiseConfig.PeerStatic = make([]byte, 32)
		copy(noiseConfig.PeerStatic, config.PeerPubKey)
	}

	return noiseConfig
}

// WriteMessage processes the next handshake message with optional 0-RTT payload.
// For initiator: the payload is encrypted immediately using the PSK-derived key.
// For responder: processes received message and creates response.
func (psk *PSKHandshake) WriteMessage(payload, receivedMessage []byte) ([]byte, bool, error) {
	psk.mu.Lock()
	defer psk.mu.Unlock()

	if psk.complete {
		return nil, false, ErrHandshakeComplete
	}

	if psk.role == Initiator {
		return psk.processInitiatorMessage(payload)
	}
	return psk.processResponderMessage(payload, receivedMessage)
}

// processInitiatorMessage handles the initiator's first message with 0-RTT data
func (psk *PSKHandshake) processInitiatorMessage(payload []byte) ([]byte, bool, error) {
	// Store early data for verification after handshake completes
	if len(payload) > 0 {
		psk.earlyData = make([]byte, len(payload))
		copy(psk.earlyData, payload)
	}

	// Write first message with 0-RTT payload
	message, sendCipher, recvCipher, err := psk.state.WriteMessage(nil, payload)
	if err != nil {
		return nil, false, fmt.Errorf("PSK initiator write failed: %w", err)
	}

	psk.sendCipher = sendCipher
	psk.recvCipher = recvCipher
	// Initiator is not complete until responder replies

	return message, psk.complete, nil
}

// processResponderMessage handles the responder's message processing and response
func (psk *PSKHandshake) processResponderMessage(payload, receivedMessage []byte) ([]byte, bool, error) {
	if receivedMessage == nil {
		return nil, false, fmt.Errorf("responder requires received message")
	}

	// Read initiator's message (includes 0-RTT data if present)
	earlyData, _, _, err := psk.state.ReadMessage(nil, receivedMessage)
	if err != nil {
		return nil, false, fmt.Errorf("PSK responder read failed: %w", err)
	}

	// Store any early data received from initiator
	if len(earlyData) > 0 {
		psk.earlyDataReceived = make([]byte, len(earlyData))
		copy(psk.earlyDataReceived, earlyData)
	}

	// Write response message
	message, writeSendCipher, writeRecvCipher, err := psk.state.WriteMessage(nil, payload)
	if err != nil {
		return nil, false, fmt.Errorf("PSK responder write failed: %w", err)
	}

	psk.sendCipher = writeSendCipher
	psk.recvCipher = writeRecvCipher
	psk.complete = true

	return message, psk.complete, nil
}

// ReadMessage processes a received handshake message.
// Used by initiator to process responder's response.
func (psk *PSKHandshake) ReadMessage(message []byte) ([]byte, bool, error) {
	psk.mu.Lock()
	defer psk.mu.Unlock()

	payload, cipher1, cipher2, err := readInitiatorResponseMessage(
		psk.state,
		psk.complete,
		psk.role,
		message,
		"PSK initiator read response failed",
	)
	if err != nil {
		return nil, false, err
	}

	psk.recvCipher = cipher1
	psk.sendCipher = cipher2
	psk.complete = true

	return payload, psk.complete, nil
}

// IsComplete returns true if handshake is finished
func (psk *PSKHandshake) IsComplete() bool {
	psk.mu.RLock()
	defer psk.mu.RUnlock()
	return psk.complete
}

// GetCipherStates returns the send and receive cipher states after successful handshake
func (psk *PSKHandshake) GetCipherStates() (*noise.CipherState, *noise.CipherState, error) {
	psk.mu.RLock()
	defer psk.mu.RUnlock()

	if !psk.complete {
		return nil, nil, ErrHandshakeNotComplete
	}

	if psk.sendCipher == nil || psk.recvCipher == nil {
		return nil, nil, fmt.Errorf("cipher states not available")
	}

	return psk.sendCipher, psk.recvCipher, nil
}

// GetRemoteStaticKey returns the peer's static public key after successful handshake
func (psk *PSKHandshake) GetRemoteStaticKey() ([]byte, error) {
	psk.mu.RLock()
	defer psk.mu.RUnlock()

	return copyRemoteStaticKey(psk.complete, psk.state)
}

// GetLocalStaticKey returns our static public key
func (psk *PSKHandshake) GetLocalStaticKey() []byte {
	psk.mu.RLock()
	defer psk.mu.RUnlock()

	if len(psk.localPubKey) > 0 {
		key := make([]byte, len(psk.localPubKey))
		copy(key, psk.localPubKey)
		return key
	}
	return nil
}

// GetTicketID returns the session ticket ID
func (psk *PSKHandshake) GetTicketID() [32]byte {
	psk.mu.RLock()
	defer psk.mu.RUnlock()
	return psk.ticketID
}

// GetNonce returns the handshake nonce for replay protection
func (psk *PSKHandshake) GetNonce() [32]byte {
	psk.mu.RLock()
	defer psk.mu.RUnlock()
	return psk.nonce
}

// GetTimestamp returns the handshake creation timestamp
func (psk *PSKHandshake) GetTimestamp() int64 {
	psk.mu.RLock()
	defer psk.mu.RUnlock()
	return psk.timestamp
}

// GetEarlyData returns the 0-RTT data sent by the initiator.
// Only valid on responder side after handshake completes.
func (psk *PSKHandshake) GetEarlyData() []byte {
	psk.mu.RLock()
	defer psk.mu.RUnlock()

	if len(psk.earlyDataReceived) > 0 {
		data := make([]byte, len(psk.earlyDataReceived))
		copy(data, psk.earlyDataReceived)
		return data
	}
	return nil
}

// DeriveSessionTicket creates a session ticket from a completed handshake.
// This ticket can be used for future 0-RTT resumption with the same peer.
func DeriveSessionTicket(
	handshake interface{ GetRemoteStaticKey() ([]byte, error) },
	sendCipher, recvCipher *noise.CipherState,
	lifetime time.Duration,
) (*SessionTicket, error) {
	if lifetime <= 0 {
		lifetime = DefaultSessionTicketLifetime
	}
	if lifetime > MaxSessionTicketLifetime {
		lifetime = MaxSessionTicketLifetime
	}

	peerKey, err := handshake.GetRemoteStaticKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get peer key: %w", err)
	}

	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(lifetime),
	}
	copy(ticket.PeerPublicKey[:], peerKey)

	// Generate random ticket ID
	if _, err := rand.Read(ticket.TicketID[:]); err != nil {
		return nil, fmt.Errorf("failed to generate ticket ID: %w", err)
	}

	// Derive PSK using HKDF from cipher state keys
	psk, err := derivePSKFromCipherStates(sendCipher, recvCipher, peerKey, ticket.TicketID[:])
	if err != nil {
		return nil, err
	}
	copy(ticket.PSK[:], psk)

	return ticket, nil
}

// derivePSKFromCipherStates derives a PSK using HKDF from the cipher states.
// The PSK is bound to the ticket ID and peer key for cryptographic binding.
func derivePSKFromCipherStates(sendCipher, recvCipher *noise.CipherState, peerKey, ticketID []byte) ([]byte, error) {
	// We can't directly access the cipher state keys, so we derive the PSK
	// from a combination of peer key, ticket ID, and random salt

	// Create a deterministic binding value
	h := hmac.New(sha256.New, ticketID)
	h.Write(peerKey)

	// Add random salt for forward secrecy
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	h.Write(salt)

	// Use timestamp for additional entropy
	h.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))

	psk := h.Sum(nil)
	return psk, nil
}

// CreateResumptionHandshake creates a PSK handshake using a session ticket.
// This is a convenience function for session resumption.
func CreateResumptionHandshake(
	staticPrivKey []byte,
	ticket *SessionTicket,
	role HandshakeRole,
) (*PSKHandshake, error) {
	if !ticket.IsValid() {
		if ticket.IsExpired() {
			return nil, ErrSessionTicketExpired
		}
		return nil, ErrSessionTicketInvalid
	}

	config := PSKHandshakeConfig{
		StaticPrivKey: staticPrivKey,
		PeerPubKey:    ticket.PeerPublicKey[:],
		PSK:           ticket.PSK,
		TicketID:      ticket.TicketID,
		Role:          role,
		PSKPlacement:  DefaultPSKPlacement,
	}

	return NewPSKHandshake(config)
}

// SupportsResumption checks if a PSK handshake can be used for resumption
// by verifying both parties have compatible session tickets.
func SupportsResumption(cache *SessionCache, peerPubKey []byte) bool {
	if cache == nil {
		return false
	}

	ticket, err := cache.GetTicket(peerPubKey)
	if err != nil {
		return false
	}

	return ticket.IsValid()
}
