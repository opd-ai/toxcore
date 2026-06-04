package async

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/sirupsen/logrus"
)

// ErrMustUseObfuscatedTransport is returned when code attempts to transmit a
// ForwardSecureMessage directly, without first wrapping it in an
// ObfuscatedAsyncMessage.  Use AsyncClient.SendAsyncMessage or
// AsyncClient.SendObfuscatedMessage to ensure sender-identity obfuscation is
// always applied before the message hits any storage node.
var ErrMustUseObfuscatedTransport = errors.New(
	"ForwardSecureMessage must be wrapped in ObfuscatedAsyncMessage before sending; " +
		"use AsyncClient.SendObfuscatedMessage or AsyncManager.SendAsyncMessage",
)

// sendFSMDeprecationWarningOnce ensures the deprecation warning for
// ForwardSecurityManager.SendForwardSecureMessage is emitted at most once per
// process lifetime, avoiding log spam in applications that have not yet
// migrated to the obfuscated API.
var sendFSMDeprecationWarningOnce sync.Once

// ForwardSecureMessage represents an async message with forward secrecy.
//
// Deprecated: ForwardSecureMessage carries SenderPK as a plaintext field,
// which leaks the real sender identity if the message is transmitted without an
// outer obfuscation envelope.  Create and consume ForwardSecureMessage only
// through ForwardSecurityManager and always wrap the result in
// ObfuscatedAsyncMessage via AsyncClient.SendObfuscatedMessage.  For new code,
// prefer AsyncClient.SendAsyncMessage or AsyncManager.SendAsyncMessage, both of
// which enforce obfuscation automatically.
type ForwardSecureMessage struct {
	Type          string      `json:"type"`
	MessageID     [32]byte    `json:"message_id"`
	SenderPK      [32]byte    `json:"sender_pk"`
	RecipientPK   [32]byte    `json:"recipient_pk"`
	PreKeyID      uint32      `json:"pre_key_id"`     // ID of the one-time key used
	EncryptedData []byte      `json:"encrypted_data"` // Message encrypted with one-time key
	Nonce         [24]byte    `json:"nonce"`
	MessageType   MessageType `json:"message_type"`
	Timestamp     time.Time   `json:"timestamp"`
	ExpiresAt     time.Time   `json:"expires_at"`
}

// PreKeyExchangeMessage is sent when peers come online to exchange/refresh pre-keys
type PreKeyExchangeMessage struct {
	Type     string              `json:"type"`
	SenderPK [32]byte            `json:"sender_pk"`
	PreKeys  []PreKeyForExchange `json:"pre_keys"`
	// SignedPreKey is a medium-term Curve25519 key that is Ed25519-signed by the
	// sender's identity key, providing X3DH-style binding between the pre-key
	// bundle and the sender's identity.  Receivers MUST verify this signature
	// before accepting the bundle.
	SignedPreKey *SignedPreKey `json:"signed_pre_key,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
}

// PreKeyForExchange represents a pre-key being shared (without private key)
type PreKeyForExchange struct {
	ID        uint32   `json:"id"`
	PublicKey [32]byte `json:"public_key"`
}

// ForwardSecurityManager handles forward-secure async messaging
type ForwardSecurityManager struct {
	preKeyStore       *PreKeyStore
	keyPair           *crypto.KeyPair
	peerPreKeys       map[[32]byte][]PreKeyForExchange // Pre-keys received from peers
	peerPreKeysMutex  sync.RWMutex                     // Protects peerPreKeys map
	preKeyRefreshFunc func([32]byte) error             // Callback to trigger pre-key exchange
	cleanupInterval   time.Duration                    // Interval for automatic cleanup
	stopCleanup       chan struct{}                    // Channel to stop cleanup goroutine
	cleanupWg         sync.WaitGroup                   // WaitGroup for all goroutines (cleanup + refresh)
	closed            bool                             // Flag indicating manager is closed
	closedMu          sync.Mutex                       // Protects closed flag

	// signedPreKey is our current medium-term signed pre-key (X3DH SPK).
	// Rotated every SignedPreKeyRotationInterval; protected by peerPreKeysMutex.
	signedPreKey *SignedPreKey
	spkNextID    uint32 // monotonic ID counter for signed pre-keys

	// preKeyConsumed tracks per-peer pre-key consumption for rate limiting.
	// Each entry is a slice of times at which a pre-key was consumed; entries
	// older than PreKeyRateWindowDuration are pruned on each access.
	// Protected by peerPreKeysMutex.
	preKeyConsumed map[[32]byte][]time.Time

	// onPreKeyLowWatermark, if non-nil, is called whenever a peer's remaining
	// pre-key count drops to or below PreKeyLowWatermark.  The callback receives
	// the peer public key and the current remaining count.  It is invoked outside
	// of any mutex and must not call back into the ForwardSecurityManager.
	onPreKeyLowWatermark func(peerPK [32]byte, remaining int)
}

const (
	// PreKeyLowWatermark triggers automatic pre-key refresh.
	// When the remaining key count drops to or below this threshold AFTER consuming a key,
	// an asynchronous refresh is triggered to replenish the pre-key pool.
	PreKeyLowWatermark = 30

	// PreKeyMinimum is the minimum number of pre-keys required to send a message.
	// Messages can be sent when available keys >= PreKeyMinimum.
	// After consuming a key for sending, if remaining keys < PreKeyMinimum,
	// further sends are blocked until refresh completes.
	//
	// The gap between PreKeyLowWatermark and PreKeyMinimum (30 - 20 = 10 keys)
	// provides a safety window for async refresh to complete before exhaustion.
	PreKeyMinimum = 20

	// PreKeyRateWindowDuration is the length of the sliding window used to
	// rate-limit how quickly a single peer can consume one-time pre-keys.
	// A peer may consume at most PreKeyRateLimit keys within this window.
	PreKeyRateWindowDuration = 5 * time.Minute

	// PreKeyRateLimit is the maximum number of pre-keys a single remote peer
	// may consume within PreKeyRateWindowDuration before further consumption
	// is refused.  This prevents a targeted DoS that silences a user by
	// draining their pre-key pool.
	PreKeyRateLimit = 10

	// PreKeyProactiveRefreshInterval is how often the manager proactively
	// checks each peer's pre-key pool and triggers a refresh even when the
	// pool has not yet reached PreKeyLowWatermark.  Weekly proactive refresh
	// mirrors Signal Protocol's signed pre-key rotation cadence.
	PreKeyProactiveRefreshInterval = 7 * 24 * time.Hour

	// DefaultCleanupInterval is the default interval for automatic pre-key cleanup.
	// Expired pre-keys are removed every 24 hours to prevent unbounded disk growth.
	DefaultCleanupInterval = 24 * time.Hour

	// preKeyRefreshTimeout is the maximum time a single preKeyRefreshFunc invocation
	// may run before the goroutine unblocks and allows Close() to proceed.
	// The callback goroutine itself is not cancelled — it will run to completion in
	// the background — but Close() will no longer wait for it beyond this deadline.
	preKeyRefreshTimeout = 30 * time.Second
)

// NewForwardSecurityManager creates a new forward security manager.
// Starts an automatic cleanup goroutine that runs every 24 hours by default.
// Call Close() when done to stop the cleanup goroutine and release resources.
func NewForwardSecurityManager(keyPair *crypto.KeyPair, dataDir string) (*ForwardSecurityManager, error) {
	return NewForwardSecurityManagerWithInterval(keyPair, dataDir, DefaultCleanupInterval)
}

// NewForwardSecurityManagerWithInterval creates a new forward security manager
// with a custom cleanup interval. Set interval to 0 to disable automatic cleanup.
func NewForwardSecurityManagerWithInterval(keyPair *crypto.KeyPair, dataDir string, cleanupInterval time.Duration) (*ForwardSecurityManager, error) {
	preKeyStore, err := NewPreKeyStore(keyPair, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create pre-key store: %w", err)
	}

	fsm := &ForwardSecurityManager{
		preKeyStore:     preKeyStore,
		keyPair:         keyPair,
		peerPreKeys:     make(map[[32]byte][]PreKeyForExchange),
		preKeyConsumed:  make(map[[32]byte][]time.Time),
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start automatic cleanup goroutine if interval is positive
	if cleanupInterval > 0 {
		fsm.startCleanupRoutine()
	}

	// Start proactive pre-key refresh goroutine (weekly by default).
	fsm.startProactiveRefreshRoutine()

	return fsm, nil
}

// startCleanupRoutine starts the background cleanup goroutine.
func (fsm *ForwardSecurityManager) startCleanupRoutine() {
	fsm.cleanupWg.Add(1)
	go func() {
		defer fsm.cleanupWg.Done()

		ticker := time.NewTicker(fsm.cleanupInterval)
		defer ticker.Stop()

		logrus.WithField("interval", fsm.cleanupInterval).Info("Started pre-key cleanup routine")

		for {
			select {
			case <-ticker.C:
				logrus.Debug("Running scheduled pre-key cleanup")
				fsm.CleanupExpiredData()
				logrus.Debug("Scheduled pre-key cleanup completed")
			case <-fsm.stopCleanup:
				logrus.Info("Stopping pre-key cleanup routine")
				return
			}
		}
	}()
}

// startProactiveRefreshRoutine starts a background goroutine that proactively
// refreshes every peer's pre-key pool on a weekly schedule
// (PreKeyProactiveRefreshInterval), independently of the low-watermark trigger.
// This mirrors Signal Protocol's signed pre-key rotation cadence and ensures
// the pool is replenished even for peers who have not exchanged messages lately.
// The goroutine exits when the manager is closed via Close().
func (fsm *ForwardSecurityManager) startProactiveRefreshRoutine() {
	fsm.cleanupWg.Add(1)
	go func() {
		defer fsm.cleanupWg.Done()

		ticker := time.NewTicker(PreKeyProactiveRefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fsm.proactiveRefreshAll()
			case <-fsm.stopCleanup:
				return
			}
		}
	}()
}

// proactiveRefreshAll iterates over every peer with known pre-keys and triggers
// a refresh if the preKeyRefreshFunc callback is registered.
func (fsm *ForwardSecurityManager) proactiveRefreshAll() {
	if fsm.preKeyRefreshFunc == nil {
		return
	}
	fsm.peerPreKeysMutex.RLock()
	peers := make([][32]byte, 0, len(fsm.peerPreKeys))
	for pk := range fsm.peerPreKeys {
		peers = append(peers, pk)
	}
	fsm.peerPreKeysMutex.RUnlock()

	for _, pk := range peers {
		fsm.closedMu.Lock()
		if fsm.closed {
			fsm.closedMu.Unlock()
			return
		}
		fsm.cleanupWg.Add(1)
		fsm.closedMu.Unlock()

		go func(peerPK [32]byte) {
			defer fsm.cleanupWg.Done()
			// Skip if already shutting down.
			select {
			case <-fsm.stopCleanup:
				return
			default:
			}
			done := make(chan error, 1)
			go func() { done <- fsm.preKeyRefreshFunc(peerPK) }()
			select {
			case err := <-done:
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"peer":  fmt.Sprintf("%x", peerPK[:8]),
						"error": err.Error(),
					}).Warn("Proactive pre-key refresh failed")
				}
			case <-time.After(preKeyRefreshTimeout):
				logrus.WithFields(logrus.Fields{
					"peer": fmt.Sprintf("%x", peerPK[:8]),
				}).Warn("Proactive pre-key refresh timed out; callback will continue in background")
			case <-fsm.stopCleanup:
				// Shutting down; callback goroutine will drain on its own.
			}
		}(pk)
	}
}

// Close is safe to call multiple times.
func (fsm *ForwardSecurityManager) Close() error {
	// Mark as closed and signal cleanup routine to stop
	fsm.closedMu.Lock()
	fsm.closed = true
	fsm.closedMu.Unlock()

	select {
	case <-fsm.stopCleanup:
		// Already closed
	default:
		close(fsm.stopCleanup)
	}

	// Wait for all goroutines (cleanup + async refresh operations) to finish
	fsm.cleanupWg.Wait()

	logrus.Info("ForwardSecurityManager closed")
	return nil
}

// GeneratePreKeysForPeer generates pre-keys for a specific peer
func (fsm *ForwardSecurityManager) GeneratePreKeysForPeer(peerPK [32]byte) error {
	_, err := fsm.preKeyStore.GeneratePreKeys(peerPK)
	return err
}

// SendForwardSecureMessage sends an async message using forward secrecy
// validateMessage checks if the message meets basic size requirements.
// It returns an error if the message is empty or exceeds the maximum size.
func validateMessage(message []byte) error {
	if len(message) == 0 {
		return errors.New("empty message")
	}
	if len(message) > MaxMessageSize {
		return fmt.Errorf("message too long: %d bytes (max %d)", len(message), MaxMessageSize)
	}
	return nil
}

// validatePreKeys checks if sufficient pre-keys are available for the recipient.
// It returns the available pre-keys or an error if insufficient.
// Must be called while holding peerPreKeysMutex.Lock().
func (fsm *ForwardSecurityManager) validatePreKeys(recipientPK [32]byte) ([]PreKeyForExchange, error) {
	peerPreKeys, exists := fsm.peerPreKeys[recipientPK]

	if !exists || len(peerPreKeys) == 0 {
		return nil, fmt.Errorf("no pre-keys available for recipient %x - cannot send forward-secure message", recipientPK[:8])
	}

	if len(peerPreKeys) < PreKeyMinimum {
		return nil, fmt.Errorf("insufficient pre-keys (%d) for recipient %x - waiting for refresh", len(peerPreKeys), recipientPK[:8])
	}

	if len(peerPreKeys) <= PreKeyMinimum+1 {
		logrus.WithFields(logrus.Fields{
			"recipient":      fmt.Sprintf("%x", recipientPK[:8]),
			"available_keys": len(peerPreKeys),
			"minimum":        PreKeyMinimum,
			"low_watermark":  PreKeyLowWatermark,
		}).Warn("Sending message with low pre-key count - may fail after this send if refresh hasn't completed")
	}

	return peerPreKeys, nil
}

// consumePreKey removes and returns the first available pre-key for the recipient.
// It updates the internal state and triggers refresh if needed.
// Must be called while holding peerPreKeysMutex.Lock().
// Returns an error if the peer has exceeded PreKeyRateLimit consumptions within
// PreKeyRateWindowDuration (DoS protection).
func (fsm *ForwardSecurityManager) consumePreKey(recipientPK [32]byte, peerPreKeys []PreKeyForExchange) (PreKeyForExchange, error) {
	// Rate-limit: prune old entries then check the window.
	now := time.Now()
	cutoff := now.Add(-PreKeyRateWindowDuration)
	times := fsm.preKeyConsumed[recipientPK]
	start := 0
	for start < len(times) && times[start].Before(cutoff) {
		start++
	}
	times = times[start:]
	if len(times) >= PreKeyRateLimit {
		return PreKeyForExchange{}, fmt.Errorf(
			"pre-key rate limit exceeded for peer %x: %d consumptions in the last %s",
			recipientPK[:8], len(times), PreKeyRateWindowDuration,
		)
	}
	fsm.preKeyConsumed[recipientPK] = append(times, now)

	preKey := peerPreKeys[0]
	fsm.peerPreKeys[recipientPK] = peerPreKeys[1:]
	remainingKeys := len(fsm.peerPreKeys[recipientPK])

	if remainingKeys <= PreKeyLowWatermark {
		if fsm.preKeyRefreshFunc != nil {
			fsm.triggerAsyncRefresh(recipientPK, remainingKeys)
		}
		// Fire the application-level hook outside the mutex via goroutine.
		if hook := fsm.onPreKeyLowWatermark; hook != nil {
			go hook(recipientPK, remainingKeys)
		}
	}

	return preKey, nil
}

// triggerAsyncRefresh initiates an asynchronous pre-key refresh operation.
// Spawned goroutines are tracked in the manager's WaitGroup to prevent leaks.
func (fsm *ForwardSecurityManager) triggerAsyncRefresh(recipientPK [32]byte, remainingKeys int) {
	// Check if manager is already closed to avoid spawning new goroutines
	fsm.closedMu.Lock()
	if fsm.closed {
		fsm.closedMu.Unlock()
		return
	}
	fsm.cleanupWg.Add(1)
	fsm.closedMu.Unlock()

	logrus.WithFields(logrus.Fields{
		"recipient":      fmt.Sprintf("%x", recipientPK[:8]),
		"remaining_keys": remainingKeys,
		"low_watermark":  PreKeyLowWatermark,
		"minimum":        PreKeyMinimum,
		"safety_window":  remainingKeys - PreKeyMinimum,
	}).Info("Pre-key count at or below low watermark - triggering async refresh")

	go func() {
		defer fsm.cleanupWg.Done()
		// Skip if already shutting down.
		select {
		case <-fsm.stopCleanup:
			return
		default:
		}
		done := make(chan error, 1)
		go func() { done <- fsm.preKeyRefreshFunc(recipientPK) }()
		select {
		case err := <-done:
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"recipient": fmt.Sprintf("%x", recipientPK[:8]),
					"error":     err.Error(),
				}).Error("Pre-key refresh failed")
			} else {
				logrus.WithFields(logrus.Fields{
					"recipient": fmt.Sprintf("%x", recipientPK[:8]),
				}).Info("Pre-key refresh completed successfully")
			}
		case <-time.After(preKeyRefreshTimeout):
			logrus.WithFields(logrus.Fields{
				"recipient": fmt.Sprintf("%x", recipientPK[:8]),
			}).Warn("Pre-key refresh timed out; callback will continue in background")
		case <-fsm.stopCleanup:
			// Shutting down; callback goroutine will drain on its own.
		}
	}()
}

// encryptWithPreKey encrypts a message using the specified pre-key.
// It generates a random nonce and returns the encrypted data.
func (fsm *ForwardSecurityManager) encryptWithPreKey(message []byte, preKey PreKeyForExchange) ([]byte, [24]byte, error) {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, nonce, fmt.Errorf("failed to generate nonce: %w", err)
	}

	encryptedData, err := crypto.Encrypt(message, nonce, preKey.PublicKey, fsm.keyPair.Private)
	if err != nil {
		return nil, nonce, fmt.Errorf("failed to encrypt message with pre-key: %w", err)
	}

	return encryptedData, nonce, nil
}

// generateMessageID creates a random 32-byte message identifier.
func generateMessageID() ([32]byte, error) {
	var messageID [32]byte
	if _, err := rand.Read(messageID[:]); err != nil {
		return messageID, fmt.Errorf("failed to generate message ID: %w", err)
	}
	return messageID, nil
}

// SendForwardSecureMessage encrypts a message with forward secrecy guarantees
// and returns the resulting envelope.  The caller MUST pass the returned
// *ForwardSecureMessage to AsyncClient.SendObfuscatedMessage; transmitting it
// directly exposes SenderPK in plaintext.
//
// Deprecated: call AsyncClient.SendAsyncMessage or AsyncManager.SendAsyncMessage
// instead — they combine forward secrecy with mandatory obfuscation in one
// step, preventing accidental identity leakage.
func (fsm *ForwardSecurityManager) SendForwardSecureMessage(recipientPK [32]byte, message []byte, messageType MessageType) (*ForwardSecureMessage, error) {
	sendFSMDeprecationWarningOnce.Do(func() {
		logrus.Warn("ForwardSecurityManager.SendForwardSecureMessage is deprecated: " +
			"wrap the result in ObfuscatedAsyncMessage via AsyncClient.SendObfuscatedMessage, " +
			"or switch to AsyncClient.SendAsyncMessage / AsyncManager.SendAsyncMessage")
	})
	return fsm.createForwardSecureMessage(recipientPK, message, messageType)
}

func (fsm *ForwardSecurityManager) createForwardSecureMessage(recipientPK [32]byte, message []byte, messageType MessageType) (*ForwardSecureMessage, error) {
	if err := validateMessage(message); err != nil {
		return nil, err
	}

	// Hold peerPreKeysMutex for the entire check-and-consume sequence to prevent
	// concurrent goroutines from consuming the same pre-key (TOCTOU).
	fsm.peerPreKeysMutex.Lock()
	peerPreKeys, err := fsm.validatePreKeys(recipientPK)
	if err != nil {
		fsm.peerPreKeysMutex.Unlock()
		return nil, err
	}
	preKey, err := fsm.consumePreKey(recipientPK, peerPreKeys)
	if err != nil {
		fsm.peerPreKeysMutex.Unlock()
		return nil, err
	}
	fsm.peerPreKeysMutex.Unlock()

	encryptedData, nonce, err := fsm.encryptWithPreKey(message, preKey)
	if err != nil {
		return nil, err
	}

	messageID, err := generateMessageID()
	if err != nil {
		return nil, err
	}

	return &ForwardSecureMessage{
		Type:          "forward_secure_message",
		MessageID:     messageID,
		SenderPK:      fsm.keyPair.Public,
		RecipientPK:   recipientPK,
		PreKeyID:      preKey.ID,
		EncryptedData: encryptedData,
		Nonce:         nonce,
		MessageType:   messageType,
		Timestamp:     time.Now(),
		ExpiresAt:     time.Now().Add(MaxStorageTime),
	}, nil
}

// DecryptForwardSecureMessage decrypts a received forward-secure message.
//
// Deprecated: the recommended receive path is AsyncClient.RetrieveAsyncMessages,
// which decrypts via the ObfuscatedAsyncMessage layer automatically.
//
// NOTE: This path marks the pre-key as used *before* successful authentication.
// A forged ciphertext can still consume a pre-key. Fixing that requires
// refactoring this deprecated path to reserve (but not persist) the key before
// decrypt, then commit only on success. For now, the existing behavior remains
// here; the recommended obfuscated path is not affected.
func (fsm *ForwardSecurityManager) DecryptForwardSecureMessage(msg *ForwardSecureMessage) ([]byte, error) {
	// Atomically check Used flag and mark the pre-key as used under a single
	// Lock to prevent two concurrent goroutines from both seeing Used==false
	// and consuming the same one-time pre-key (TOCTOU).
	preKey, err := fsm.preKeyStore.CheckAndMarkPreKeyUsed(msg.SenderPK, msg.PreKeyID)
	if err != nil {
		return nil, err
	}

	// Decrypt message using the one-time pre-key
	if preKey.KeyPair == nil {
		return nil, fmt.Errorf("pre-key %d has nil key pair (corrupted state)", msg.PreKeyID)
	}
	decryptedData, err := crypto.Decrypt(msg.EncryptedData, msg.Nonce, msg.SenderPK, preKey.KeyPair.Private)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}

	return decryptedData, nil
}

// SendForwardSecureMessageDirect is a compile/runtime guard that always returns
// ErrMustUseObfuscatedTransport.  It exists so that any code that tries to
// transmit a ForwardSecureMessage without an outer ObfuscatedAsyncMessage
// envelope fails explicitly rather than silently leaking sender identity.
//
// Use AsyncClient.SendObfuscatedMessage or AsyncManager.SendAsyncMessage instead.
func SendForwardSecureMessageDirect(_ *ForwardSecureMessage) error {
	return ErrMustUseObfuscatedTransport
}

// SetPreKeyRefreshCallback sets the callback function for pre-key refresh.
func (fsm *ForwardSecurityManager) SetPreKeyRefreshCallback(callback func([32]byte) error) {
	fsm.preKeyRefreshFunc = callback
}

// SetPreKeyLowWatermarkHook registers a callback that is fired (in a separate
// goroutine) whenever a peer's remaining pre-key count drops to or below
// [PreKeyLowWatermark].  The callback receives the peer's public key and the
// current remaining count so the application can present a warning to the user.
// Pass nil to clear an existing hook.
func (fsm *ForwardSecurityManager) SetPreKeyLowWatermarkHook(hook func(peerPK [32]byte, remaining int)) {
	fsm.peerPreKeysMutex.Lock()
	defer fsm.peerPreKeysMutex.Unlock()
	fsm.onPreKeyLowWatermark = hook
}

// ExchangePreKeys creates a pre-key exchange message for a peer
func (fsm *ForwardSecurityManager) ExchangePreKeys(peerPK [32]byte) (*PreKeyExchangeMessage, error) {
	// Check if we need to generate pre-keys for this peer
	if fsm.preKeyStore.NeedsRefresh(peerPK) {
		if _, err := fsm.preKeyStore.RefreshPreKeys(peerPK); err != nil {
			return nil, fmt.Errorf("failed to refresh pre-keys: %w", err)
		}
	}

	// Get our pre-key bundle for this peer
	bundle, err := fsm.preKeyStore.GetBundle(peerPK)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-key bundle: %w", err)
	}

	// Create exchange message with public keys only
	preKeysForExchange := make([]PreKeyForExchange, 0, len(bundle.Keys))
	for _, key := range bundle.Keys {
		if !key.Used && key.KeyPair != nil {
			preKeysForExchange = append(preKeysForExchange, PreKeyForExchange{
				ID:        key.ID,
				PublicKey: key.KeyPair.Public,
			})
		}
	}

	return &PreKeyExchangeMessage{
		Type:         "pre_key_exchange",
		SenderPK:     fsm.keyPair.Public,
		PreKeys:      preKeysForExchange,
		SignedPreKey: fsm.currentSignedPreKey(),
		Timestamp:    time.Now(),
	}, nil
}

// currentSignedPreKey returns the current signed pre-key, rotating it if expired.
// The caller must not be holding peerPreKeysMutex.
func (fsm *ForwardSecurityManager) currentSignedPreKey() *SignedPreKey {
	fsm.peerPreKeysMutex.Lock()
	defer fsm.peerPreKeysMutex.Unlock()

	if fsm.signedPreKey == nil || fsm.signedPreKey.ShouldRotate() {
		fsm.spkNextID++
		spk, err := NewSignedPreKey(fsm.spkNextID, fsm.keyPair.Private)
		if err != nil {
			return nil
		}
		fsm.signedPreKey = spk
	}
	return fsm.signedPreKey
}

// ProcessPreKeyExchange processes received pre-keys from a peer
func (fsm *ForwardSecurityManager) ProcessPreKeyExchange(exchange *PreKeyExchangeMessage) error {
	if exchange == nil {
		return errors.New("nil pre-key exchange")
	}
	if len(exchange.PreKeys) == 0 {
		return errors.New("empty pre-key exchange")
	}

	// Verify the signed pre-key if present.
	// A bundle with a signed pre-key whose signature fails MUST be rejected to
	// prevent a relay from substituting a bogus bundle.
	if exchange.SignedPreKey != nil {
		if err := exchange.SignedPreKey.Verify(); err != nil {
			return fmt.Errorf("ProcessPreKeyExchange: %w", err)
		}
	}

	fsm.peerPreKeysMutex.Lock()
	defer fsm.peerPreKeysMutex.Unlock()

	fsm.peerPreKeys[exchange.SenderPK] = mergeUniquePreKeys(
		fsm.peerPreKeys[exchange.SenderPK],
		exchange.PreKeys,
	)

	return nil
}

func mergeUniquePreKeys(existing, incoming []PreKeyForExchange) []PreKeyForExchange {
	merged := make([]PreKeyForExchange, 0, len(existing)+len(incoming))
	seenIDs := make(map[uint32]struct{}, len(existing)+len(incoming))

	appendUnique := func(keys []PreKeyForExchange) {
		for _, key := range keys {
			if _, exists := seenIDs[key.ID]; exists {
				continue
			}
			seenIDs[key.ID] = struct{}{}
			merged = append(merged, key)
		}
	}

	appendUnique(existing)
	appendUnique(incoming)

	return merged
}

// GetAvailableKeyCount returns the number of available pre-keys for a peer
func (fsm *ForwardSecurityManager) GetAvailableKeyCount(peerPK [32]byte) int {
	fsm.peerPreKeysMutex.RLock()
	preKeys, exists := fsm.peerPreKeys[peerPK]
	fsm.peerPreKeysMutex.RUnlock()

	if exists {
		return len(preKeys)
	}
	return 0
}

// NeedsKeyExchange checks if we need to exchange pre-keys with a peer
func (fsm *ForwardSecurityManager) NeedsKeyExchange(peerPK [32]byte) bool {
	// Need exchange if we have no keys or very few keys remaining
	availableKeys := fsm.GetAvailableKeyCount(peerPK)
	return availableKeys <= PreKeyRefreshThreshold
}

// CanSendMessage checks if we can send a forward-secure message to a peer
// Returns true if we have at least the minimum required pre-keys
func (fsm *ForwardSecurityManager) CanSendMessage(peerPK [32]byte) bool {
	return fsm.GetAvailableKeyCount(peerPK) >= PreKeyMinimum
}

// CleanupExpiredData removes old pre-keys and expired data
func (fsm *ForwardSecurityManager) CleanupExpiredData() {
	// Cleanup local pre-key bundles
	fsm.preKeyStore.CleanupExpiredBundles()

	// Remove expired peer pre-keys (optional - could keep them longer)
	// For now, we'll keep peer pre-keys until they're used or refreshed
}
