package async

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	"golang.org/x/crypto/curve25519"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/limits"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// min returns the minimum of two integers (for Go versions < 1.21)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// encodeGob encodes a value using gob encoding for network transmission.
// Returns the encoded bytes or an error if encoding fails.
func encodeGob(v interface{}, typeName string) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	err := encoder.Encode(v)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", typeName, err)
	}

	return buf.Bytes(), nil
}

// retrieveResponse holds a response from a storage node
type retrieveResponse struct {
	messages []*ObfuscatedAsyncMessage
	err      error
}

// nodeResult holds the result of querying a single storage node for messages
type nodeResult struct {
	messages []DecryptedMessage
	err      error
	nodeAddr net.Addr
}

// AsyncClient handles the client-side operations for async messaging
// with built-in peer identity obfuscation for privacy protection
// AsyncClient handles the client-side operations for async messaging
// with built-in peer identity obfuscation for privacy protection.
// The client uses erasure-coded storage across k=5 storage nodes to achieve
// 99.9% message survival with up to 2 node failures.
type AsyncClient struct {
	mutex              sync.RWMutex
	keyPair            *crypto.KeyPair
	obfuscation        *ObfuscationManager              // Handles cryptographic obfuscation
	transport          transport.Transport              // Network transport for communication
	storageNodes       map[[32]byte]net.Addr            // Known storage nodes
	knownSenders       map[[32]byte]bool                // Known senders for message decryption
	lastRetrieve       time.Time                        // Last message retrieval time
	retrievalScheduler *RetrievalScheduler              // Schedules randomized retrieval with cover traffic
	keyRotation        *crypto.KeyRotationManager       // Handles identity key rotation
	onKeyRotated       func(newKey *crypto.KeyPair)     // Called after every successful key rotation (may be nil)
	retrieveChannels   map[string]chan retrieveResponse // Channels for retrieve responses keyed by node address
	channelMutex       sync.Mutex                       // Protects retrieveChannels map
	retrieveTimeout    time.Duration                    // Timeout for storage node retrieval operations
	collectionTimeout  time.Duration                    // Overall timeout for collecting from all nodes
	parallelizeQueries bool                             // Whether to query storage nodes in parallel
	erasureStorage     *ErasureStorage                  // Erasure-coded shard storage for message reconstruction
	erasureEnabled     bool                             // Whether to use erasure-coded storage (default: true)
	stopChan           chan struct{}                    // Channel to signal goroutine shutdown
	closeOnce          sync.Once                        // Ensures Close is idempotent (M-19)
	forwardSecurity    *ForwardSecurityManager          // Forward secrecy manager (optional; set by AsyncManager)
}

// NewAsyncClient creates a new async messaging client with obfuscation support
// and erasure-coded storage for message redundancy across k=5 storage nodes.
func NewAsyncClient(keyPair *crypto.KeyPair, trans transport.Transport) *AsyncClient {
	logrus.WithFields(logrus.Fields{
		"function":           "NewAsyncClient",
		"public_key_preview": fmt.Sprintf("%x", keyPair.Public[:8]),
	}).Info("Creating new async client")

	epochManager := NewEpochManager()
	obfuscation := NewObfuscationManager(keyPair, epochManager)
	erasureStorage := initErasureStorage()

	ac := &AsyncClient{
		keyPair:            keyPair,
		obfuscation:        obfuscation,
		transport:          trans,
		storageNodes:       make(map[[32]byte]net.Addr),
		knownSenders:       make(map[[32]byte]bool),
		lastRetrieve:       time.Now(),
		retrieveChannels:   make(map[string]chan retrieveResponse),
		retrieveTimeout:    2 * time.Second,
		collectionTimeout:  5 * time.Second,
		parallelizeQueries: true,
		erasureStorage:     erasureStorage,
		erasureEnabled:     erasureStorage != nil,
		stopChan:           make(chan struct{}),
	}

	registerAsyncTransportHandler(ac, trans)
	ac.retrievalScheduler = NewRetrievalScheduler(ac)

	logrus.WithFields(logrus.Fields{
		"function":           "NewAsyncClient",
		"public_key_preview": fmt.Sprintf("%x", keyPair.Public[:8]),
	}).Info("Async client created successfully")
	return ac
}

// initErasureStorage initialises the erasure-coded storage with the default 3+2
// configuration, logging a warning and returning nil on failure.
func initErasureStorage() *ErasureStorage {
	es, err := NewErasureStorage(DefaultErasureCodingConfig())
	if err != nil {
		logrus.WithField("error", err.Error()).
			Warn("Failed to initialize erasure storage, falling back to simple redundancy")
	}
	return es
}

// registerAsyncTransportHandler registers the retrieve-response handler on trans
// when trans is non-nil, or logs a warning if no transport is available.
func registerAsyncTransportHandler(ac *AsyncClient, trans transport.Transport) {
	if trans != nil {
		trans.RegisterHandler(transport.PacketAsyncRetrieveResponse, ac.handleRetrieveResponse)
		return
	}
	logrus.WithField("function", "NewAsyncClient").
		Warn("Transport is nil - async messaging features will be unavailable")
}

// Close stops the async client's background goroutines and releases resources.
// This should be called when the client is no longer needed to prevent goroutine leaks.
// Close is idempotent and safe to call multiple times concurrently (M-19).
func (ac *AsyncClient) Close() {
	ac.closeOnce.Do(func() {
		close(ac.stopChan)
	})
	logrus.WithFields(logrus.Fields{
		"function": "AsyncClient.Close",
	}).Info("Async client closed")
}

// SetRetrieveTimeout configures the timeout for storage node retrieval operations.
// Default is 2 seconds. Lower values fail faster but may miss slow responses.
func (ac *AsyncClient) SetRetrieveTimeout(timeout time.Duration) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	ac.retrieveTimeout = timeout
}

// GetRetrieveTimeout returns the current retrieval timeout setting
func (ac *AsyncClient) GetRetrieveTimeout() time.Duration {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.retrieveTimeout
}

// SetCollectionTimeout configures the overall timeout for collecting messages from all storage nodes.
// Default is 5 seconds. This prevents excessive blocking when multiple nodes are unreachable.
func (ac *AsyncClient) SetCollectionTimeout(timeout time.Duration) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	ac.collectionTimeout = timeout
}

// GetCollectionTimeout returns the current collection timeout setting
func (ac *AsyncClient) GetCollectionTimeout() time.Duration {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.collectionTimeout
}

// SetParallelizeQueries configures whether storage node queries should be parallelized.
// Default is true. Parallel queries improve performance but may increase network load.
func (ac *AsyncClient) SetParallelizeQueries(parallel bool) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	ac.parallelizeQueries = parallel
}

// GetParallelizeQueries returns the current parallelization setting
func (ac *AsyncClient) GetParallelizeQueries() bool {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.parallelizeQueries
}

// SetErasureCodingEnabled enables or disables erasure-coded storage.
// When enabled (default), messages are split into 5 shards (3 data + 2 parity)
// and distributed across 5 storage nodes for 99.9% message survival.
// When disabled, falls back to simple 3-way replication.
func (ac *AsyncClient) SetErasureCodingEnabled(enabled bool) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	if enabled && ac.erasureStorage == nil {
		// Try to initialize erasure storage if it wasn't created
		if storage, err := NewErasureStorage(DefaultErasureCodingConfig()); err == nil {
			ac.erasureStorage = storage
		}
	}
	ac.erasureEnabled = enabled && ac.erasureStorage != nil
}

// IsErasureCodingEnabled returns whether erasure-coded storage is currently enabled.
func (ac *AsyncClient) IsErasureCodingEnabled() bool {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.erasureEnabled
}

// GetErasureStorageStats returns statistics about the erasure-coded storage.
// Returns nil if erasure coding is not enabled.
func (ac *AsyncClient) GetErasureStorageStats() *ErasureStats {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	if ac.erasureStorage == nil {
		return nil
	}
	stats := ac.erasureStorage.GetStats()
	return &stats
}

// SendObfuscatedMessage sends a forward-secure message using peer identity obfuscation.
// This method hides the real sender and recipient identities from storage nodes
// while maintaining message deliverability and forward secrecy.
func (ac *AsyncClient) SendObfuscatedMessage(recipientPK [32]byte,
	forwardSecureMsg *ForwardSecureMessage,
) error {
	if forwardSecureMsg == nil {
		return errors.New("nil forward secure message")
	}

	// Serialize the message and create the obfuscated envelope under a read lock.
	// The lock is released before calling storeObfuscatedMessage so that
	// findStorageNodes → collectCandidateNodes can acquire it without nesting.
	ac.mutex.RLock()
	serializedMsg, err := ac.serializeForwardSecureMessage(forwardSecureMsg)
	if err != nil {
		ac.mutex.RUnlock()
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	sharedSecret, err := ac.deriveSharedSecret(recipientPK)
	if err != nil {
		ac.mutex.RUnlock()
		return fmt.Errorf("failed to derive shared secret: %w", err)
	}

	obfMsg, err := ac.obfuscation.CreateObfuscatedMessage(
		ac.keyPair.Private, recipientPK, serializedMsg, sharedSecret,
	)
	ac.mutex.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to create obfuscated message: %w", err)
	}

	// Store on multiple storage nodes for redundancy.
	// Lock is not held here; collectCandidateNodes acquires its own RLock.
	return ac.storeObfuscatedMessage(obfMsg)
}

// SendAsyncMessage sends a message asynchronously using obfuscation by default.
// This method automatically provides forward secrecy and peer identity obfuscation.
// It creates a ForwardSecureMessage and sends it using the obfuscated transport.
//
// When a ForwardSecurityManager has been configured (via SetForwardSecurityManager or
// through AsyncManager), the message payload is encrypted with a one-time pre-key
// for genuine forward secrecy. If configured but pre-keys are unavailable for the
// recipient, an error is returned (fail-closed) rather than silently degrading to plaintext.
// Without a configured manager the message is placed in the ForwardSecureMessage envelope
// unencrypted; prefer using AsyncManager which wires the ForwardSecurityManager automatically.
func (ac *AsyncClient) SendAsyncMessage(recipientPK [32]byte, message []byte,
	messageType MessageType,
) error {
	if err := validateAsyncMessagePayload(message); err != nil {
		return err
	}

	paddedMessage, err := PadMessageToStandardSize(message)
	if err != nil {
		return fmt.Errorf("failed to pad message: %w", err)
	}

	// Use the ForwardSecurityManager when available — it provides genuine one-time
	// pre-key encryption so that compromise of the long-term static key does not
	// expose past messages.
	fsm := ac.getForwardSecurityManager()
	if fsm != nil && !fsm.CanSendMessage(recipientPK) {
		return fmt.Errorf("forward secrecy configured but pre-keys unavailable for recipient; queue message for retry or disable forward secrecy")
	}
	sent, err := ac.conditionalForwardSecureSend(recipientPK, paddedMessage, messageType, fsm)
	if err != nil {
		return err
	}
	if sent {
		return nil
	}

	// Fallback: create a ForwardSecureMessage with the message in plaintext.
	// The outer obfuscation layer still protects against storage-node observers,
	// but there is no per-message forward secrecy.
	// This path is only reached when FSM is nil (not configured).
	forwardSecureMsg, err := ac.createFallbackForwardSecureMessage(recipientPK, paddedMessage, messageType)
	if err != nil {
		return err
	}

	// Use the obfuscated message sending method for privacy protection
	return ac.SendObfuscatedMessage(recipientPK, forwardSecureMsg)
}

func validateAsyncMessagePayload(message []byte) error {
	if message == nil {
		return errors.New("message cannot be nil")
	}
	if len(message) == 0 {
		return errors.New("message cannot be empty")
	}
	if len(message) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(message), MaxMessageSize)
	}
	return nil
}

func (ac *AsyncClient) getForwardSecurityManager() *ForwardSecurityManager {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.forwardSecurity
}

func (ac *AsyncClient) conditionalForwardSecureSend(
	recipientPK [32]byte,
	message []byte,
	messageType MessageType,
	fsm *ForwardSecurityManager,
) (bool, error) {
	if fsm != nil && fsm.CanSendMessage(recipientPK) {
		forwardSecureMsg, err := fsm.createForwardSecureMessage(recipientPK, message, messageType)
		if err != nil {
			// Forward secrecy was possible but the FSM failed (e.g. key store I/O error).
			// Return the error rather than silently degrading to a plaintext envelope —
			// the caller must decide whether to retry or queue the message.
			return false, fmt.Errorf("forward-secure send failed: %w", err)
		}
		if err := ac.SendObfuscatedMessage(recipientPK, forwardSecureMsg); err != nil {
			return false, err
		}
		return true, nil
	}

	if fsm == nil {
		logrus.Warn("AsyncClient.SendAsyncMessage: no ForwardSecurityManager configured — message will be sent without inner-layer encryption. Use AsyncManager or call SetForwardSecurityManager.")
	} else {
		return false, fmt.Errorf("forward secrecy configured but pre-keys unavailable for recipient; queue message for retry or disable forward secrecy")
	}
	return false, nil
}

func (ac *AsyncClient) createFallbackForwardSecureMessage(
	recipientPK [32]byte,
	message []byte,
	messageType MessageType,
) (*ForwardSecureMessage, error) {
	// Fallback: create a ForwardSecureMessage with the message in plaintext.
	// The outer obfuscation layer still protects against storage-node observers,
	// but there is no per-message forward secrecy.
	var nonce [24]byte
	// Generate a cryptographically secure random nonce
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	messageID, err := generateMessageID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate message ID: %w", err)
	}

	forwardSecureMsg := &ForwardSecureMessage{
		Type:          "forward_secure_message",
		MessageID:     messageID,
		SenderPK:      ac.keyPair.Public,
		RecipientPK:   recipientPK,
		PreKeyID:      0,       // No pre-key used in this fallback path
		EncryptedData: message, // Not encrypted with a pre-key; outer obfuscation only
		Nonce:         nonce,
		MessageType:   messageType,
		Timestamp:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}
	return forwardSecureMsg, nil
}

// RetrieveAsyncMessages retrieves pending messages for this client using obfuscation by default.
// This method automatically provides privacy protection by using pseudonym-based retrieval.
func (ac *AsyncClient) RetrieveAsyncMessages() ([]DecryptedMessage, error) {
	// Use the obfuscated message retrieval method for privacy protection
	return ac.RetrieveObfuscatedMessages()
}

// RetrieveObfuscatedMessages retrieves pending obfuscated messages for this client
// using pseudonym-based retrieval for privacy protection
func (ac *AsyncClient) RetrieveObfuscatedMessages() ([]DecryptedMessage, error) {
	// Snapshot configuration and state under the lock, then release before
	// doing network I/O to avoid holding the mutex across blocking calls.
	ac.mutex.RLock()
	recentEpochs := ac.obfuscation.epochManager.GetRecentEpochs()
	keyPairPublic := ac.keyPair.Public
	collectionTimeout := ac.collectionTimeout
	parallelizeQueries := ac.parallelizeQueries
	retrieveTimeout := ac.retrieveTimeout
	storageNodesSnapshot := make(map[[32]byte]net.Addr, len(ac.storageNodes))
	for k, v := range ac.storageNodes {
		storageNodesSnapshot[k] = v
	}
	ac.mutex.RUnlock()

	var allMessages []DecryptedMessage

	// For each epoch, generate our pseudonym and retrieve messages
	for _, epoch := range recentEpochs {
		epochMessages := ac.retrieveMessagesForEpochLockFree(epoch, keyPairPublic, storageNodesSnapshot, collectionTimeout, parallelizeQueries, retrieveTimeout)
		allMessages = append(allMessages, epochMessages...)
	}

	ac.mutex.Lock()
	ac.lastRetrieve = time.Now()
	ac.mutex.Unlock()

	return allMessages, nil
}

// retrieveMessagesForEpoch retrieves all messages for a specific epoch using pseudonym-based lookup
func (ac *AsyncClient) retrieveMessagesForEpoch(epoch uint64) []DecryptedMessage {
	myPseudonym, err := ac.generateRecipientPseudonymForEpoch(epoch)
	if err != nil {
		log.Printf("AsyncClient: Failed to generate pseudonym for epoch %d: %v", epoch, err)
		return nil // Skip this epoch on error
	}

	storageNodes := ac.findAvailableStorageNodes(myPseudonym)
	if len(storageNodes) == 0 {
		log.Printf("AsyncClient: No storage nodes available for epoch %d", epoch)
		return nil // Skip this epoch if no storage nodes available
	}

	return ac.collectMessagesFromNodes(storageNodes, myPseudonym, epoch)
}

// retrieveMessagesForEpochLockFree retrieves messages for a specific epoch without
// holding the mutex, using pre-snapshotted state to avoid recursive lock acquisition.
func (ac *AsyncClient) retrieveMessagesForEpochLockFree(epoch uint64, publicKey [32]byte, storageNodesMap map[[32]byte]net.Addr, collectionTimeout time.Duration, parallelizeQueries bool, retrieveTimeout time.Duration) []DecryptedMessage {
	myPseudonym, err := ac.obfuscation.GenerateRecipientPseudonym(publicKey, epoch)
	if err != nil {
		log.Printf("AsyncClient: Failed to generate pseudonym for epoch %d: %v", epoch, err)
		return nil
	}

	nodes := ac.findStorageNodesFromSnapshot(myPseudonym, 5, storageNodesMap)
	if len(nodes) == 0 {
		log.Printf("AsyncClient: No storage nodes available for epoch %d", epoch)
		return nil
	}

	return ac.collectMessagesFromNodesLockFree(nodes, myPseudonym, epoch, collectionTimeout, parallelizeQueries, retrieveTimeout)
}

// generateRecipientPseudonymForEpoch creates a recipient pseudonym for the given epoch
func (ac *AsyncClient) generateRecipientPseudonymForEpoch(epoch uint64) ([32]byte, error) {
	return ac.obfuscation.GenerateRecipientPseudonym(ac.keyPair.Public, epoch)
}

// findAvailableStorageNodes locates storage nodes that might contain messages for the pseudonym
func (ac *AsyncClient) findAvailableStorageNodes(pseudonym [32]byte) []net.Addr {
	return ac.findStorageNodes(pseudonym, 5)
}

// collectMessagesFromNodes retrieves and decrypts messages from all available storage nodes
// with overall timeout and optional parallelization to prevent excessive blocking
func (ac *AsyncClient) collectMessagesFromNodes(storageNodes []net.Addr, pseudonym [32]byte, epoch uint64) []DecryptedMessage {
	// Get configuration settings
	ac.mutex.RLock()
	collectionTimeout := ac.collectionTimeout
	parallelizeQueries := ac.parallelizeQueries
	retrieveTimeout := ac.retrieveTimeout
	ac.mutex.RUnlock()

	// Create context with overall timeout for all node queries
	ctx, cancel := context.WithTimeout(context.Background(), collectionTimeout)
	defer cancel()

	if parallelizeQueries {
		return ac.collectMessagesParallel(ctx, storageNodes, pseudonym, epoch, retrieveTimeout)
	}
	return ac.collectMessagesSequential(ctx, storageNodes, pseudonym, epoch, retrieveTimeout)
}

// collectMessagesSequential queries storage nodes one at a time with adaptive timeout
func (ac *AsyncClient) collectMessagesSequential(ctx context.Context, storageNodes []net.Addr, pseudonym [32]byte, epoch uint64, baseTimeout time.Duration) []DecryptedMessage {
	var messages []DecryptedMessage
	consecutiveFailures := 0

	for _, nodeAddr := range storageNodes {
		if shouldStopSequentialCollection(ctx) {
			return messages
		}

		timeout := calculateAdaptiveTimeout(baseTimeout, consecutiveFailures)
		nodeMessages, err := ac.retrieveMessagesFromSingleNodeWithTimeout(nodeAddr, pseudonym, epoch, timeout)
		if err != nil {
			consecutiveFailures++
			if shouldAbortSequentialCollection(consecutiveFailures) {
				break
			}
			continue
		}

		consecutiveFailures = 0
		messages = append(messages, nodeMessages...)
	}

	return messages
}

// shouldStopSequentialCollection checks if the context has been cancelled.
func shouldStopSequentialCollection(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		logrus.WithFields(logrus.Fields{
			"function": "collectMessagesSequential",
			"reason":   "overall timeout exceeded",
		}).Warn("Stopping node queries due to timeout")
		return true
	default:
		return false
	}
}

// calculateAdaptiveTimeout adjusts timeout based on consecutive failure count.
func calculateAdaptiveTimeout(baseTimeout time.Duration, consecutiveFailures int) time.Duration {
	if consecutiveFailures > 0 {
		return baseTimeout / 2
	}
	return baseTimeout
}

// shouldAbortSequentialCollection determines if retrieval should abort due to failures.
func shouldAbortSequentialCollection(consecutiveFailures int) bool {
	if consecutiveFailures >= 3 {
		logrus.WithFields(logrus.Fields{
			"function":             "collectMessagesSequential",
			"consecutive_failures": consecutiveFailures,
		}).Warn("Multiple consecutive node failures - aborting further retrieval attempts")
		return true
	}
	return false
}

// collectMessagesParallel queries all storage nodes in parallel for better performance
// queryStorageNode queries a single storage node for messages and sends the result to the channel.
// It checks for context cancellation before starting the retrieval operation.
func (ac *AsyncClient) queryStorageNode(ctx context.Context, addr net.Addr, pseudonym [32]byte, epoch uint64, timeout time.Duration, resultChan chan<- nodeResult) {
	select {
	case <-ctx.Done():
		resultChan <- nodeResult{err: ctx.Err(), nodeAddr: addr}
		return
	default:
	}

	nodeMessages, err := ac.retrieveMessagesFromSingleNodeWithTimeout(addr, pseudonym, epoch, timeout)
	resultChan <- nodeResult{
		messages: nodeMessages,
		err:      err,
		nodeAddr: addr,
	}
}

// launchParallelQueries initiates concurrent queries to all storage nodes.
// It returns a channel that will be closed when all queries complete.
func (ac *AsyncClient) launchParallelQueries(ctx context.Context, storageNodes []net.Addr, pseudonym [32]byte, epoch uint64, timeout time.Duration) <-chan nodeResult {
	resultChan := make(chan nodeResult, len(storageNodes))
	var wg sync.WaitGroup

	for _, nodeAddr := range storageNodes {
		wg.Add(1)
		go func(addr net.Addr) {
			defer wg.Done()
			ac.queryStorageNode(ctx, addr, pseudonym, epoch, timeout, resultChan)
		}(nodeAddr)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	return resultChan
}

// processNodeResult processes a single node result and updates counters.
// It returns true if result was successful, false otherwise.
func processNodeResult(result nodeResult, allMessages *[]DecryptedMessage, successCount, failureCount *int) bool {
	if result.err != nil {
		*failureCount++
		return false
	}
	*successCount++
	*allMessages = append(*allMessages, result.messages...)
	return true
}

func (ac *AsyncClient) collectMessagesParallel(ctx context.Context, storageNodes []net.Addr, pseudonym [32]byte, epoch uint64, timeout time.Duration) []DecryptedMessage {
	resultChan := ac.launchParallelQueries(ctx, storageNodes, pseudonym, epoch, timeout)
	return drainQueryResults(ctx, resultChan, len(storageNodes))
}

// drainQueryResults consumes all results from resultChan, returning the accumulated
// decrypted messages. It returns early if ctx is cancelled.
func drainQueryResults(ctx context.Context, resultChan <-chan nodeResult, totalNodes int) []DecryptedMessage {
	var allMessages []DecryptedMessage
	successCount := 0
	failureCount := 0

	for {
		select {
		case <-ctx.Done():
			logrus.WithFields(logrus.Fields{
				"function":      "collectMessagesParallel",
				"success_count": successCount,
				"failure_count": failureCount,
			}).Warn("Overall timeout exceeded while collecting results")
			return allMessages

		case result, ok := <-resultChan:
			if !ok {
				logrus.WithFields(logrus.Fields{
					"function":      "collectMessagesParallel",
					"success_count": successCount,
					"failure_count": failureCount,
					"total_nodes":   totalNodes,
				}).Debug("Completed parallel message collection")
				return allMessages
			}

			processNodeResult(result, &allMessages, &successCount, &failureCount)
		}
	}
}

// retrieveMessagesFromSingleNodeWithTimeout retrieves and decrypts messages from one storage node with custom timeout
func (ac *AsyncClient) retrieveMessagesFromSingleNodeWithTimeout(nodeAddr net.Addr, pseudonym [32]byte, epoch uint64, timeout time.Duration) ([]DecryptedMessage, error) {
	obfMessages, err := ac.retrieveObfuscatedMessagesFromNode(nodeAddr, pseudonym, []uint64{epoch}, timeout)
	if err != nil {
		log.Printf("AsyncClient: Failed to retrieve messages from node %v for epoch %d: %v", nodeAddr, epoch, err)
		return nil, err
	}

	return ac.decryptRetrievedMessages(obfMessages), nil
}

// decryptRetrievedMessages decrypts and validates a collection of obfuscated messages
func (ac *AsyncClient) decryptRetrievedMessages(obfMessages []*ObfuscatedAsyncMessage) []DecryptedMessage {
	var decryptedMessages []DecryptedMessage

	// Acquire a single RLock that covers both the forwardSecurity read and the
	// decryptObfuscatedMessage calls (which read ac.knownSenders, ac.keyPair, and
	// ac.keyRotation).  This function is called only after network I/O has completed,
	// so the lock is not held across any blocking operations.
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	fsm := ac.forwardSecurity

	for _, obfMsg := range obfMessages {
		if obfMsg == nil {
			continue
		}
		forwardSecureMsg, err := ac.decryptObfuscatedMessage(obfMsg)
		if err != nil {
			continue // Skip messages we can't decrypt
		}

		plaintext, err := ac.decryptInnerRetrievedPayload(forwardSecureMsg, fsm)
		if err != nil {
			continue
		}

		decryptedMessages = append(decryptedMessages, buildDecryptedMessage(forwardSecureMsg, plaintext))
	}

	return decryptedMessages
}

func (ac *AsyncClient) decryptInnerRetrievedPayload(
	forwardSecureMsg *ForwardSecureMessage,
	fsm *ForwardSecurityManager,
) ([]byte, error) {
	if fsm != nil && forwardSecureMsg.PreKeyID != 0 {
		plaintext, err := fsm.DecryptForwardSecureMessage(forwardSecureMsg)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":   "decryptInnerRetrievedPayload",
				"pre_key_id": forwardSecureMsg.PreKeyID,
				"sender":     fmt.Sprintf("%x", forwardSecureMsg.SenderPK[:8]),
				"error":      err.Error(),
			}).Warn("FSM inner-layer decryption failed; skipping message")
			return nil, err
		}
		plaintext, err = UnpadMessage(plaintext)
		if err != nil {
			return nil, fmt.Errorf("failed to unpad FSM inner-layer payload: %w", err)
		}
		return plaintext, nil
	}

	plaintext, err := UnpadMessage(forwardSecureMsg.EncryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to unpad message: %w", err)
	}
	return plaintext, nil
}

func buildDecryptedMessage(forwardSecureMsg *ForwardSecureMessage, plaintext []byte) DecryptedMessage {
	var messageID [16]byte
	copy(messageID[:], forwardSecureMsg.MessageID[:16])

	return DecryptedMessage{
		ID:          messageID,
		SenderPK:    forwardSecureMsg.SenderPK,
		Message:     plaintext,
		MessageType: forwardSecureMsg.MessageType,
		Timestamp:   forwardSecureMsg.Timestamp,
	}
}

// serializeForwardSecureMessage converts a ForwardSecureMessage to bytes using efficient binary encoding.
// This production implementation uses Go's gob encoder for type-safe, versioned serialization
// that's more efficient and reliable than string-based formats.
func (ac *AsyncClient) serializeForwardSecureMessage(fsMsg *ForwardSecureMessage) ([]byte, error) {
	if fsMsg == nil {
		return nil, errors.New("cannot serialize nil ForwardSecureMessage")
	}

	// Use bytes.Buffer for efficient memory allocation
	var buf bytes.Buffer

	// Create gob encoder for binary serialization
	encoder := gob.NewEncoder(&buf)

	// Serialize the message structure
	err := encoder.Encode(fsMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode ForwardSecureMessage: %w", err)
	}

	return buf.Bytes(), nil
}

// deserializeForwardSecureMessage converts bytes back to ForwardSecureMessage.
// This companion function enables round-trip serialization for testing and message processing.
func (ac *AsyncClient) deserializeForwardSecureMessage(data []byte) (*ForwardSecureMessage, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot deserialize empty data")
	}

	// Create buffer from input data
	buf := bytes.NewBuffer(data)

	// Create gob decoder for binary deserialization
	decoder := gob.NewDecoder(buf)

	// Deserialize into message structure
	var fsMsg ForwardSecureMessage
	err := decoder.Decode(&fsMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ForwardSecureMessage: %w", err)
	}

	return &fsMsg, nil
}

// serializeObfuscatedMessage converts an ObfuscatedAsyncMessage to bytes for network transmission
func (ac *AsyncClient) serializeObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage) ([]byte, error) {
	if obfMsg == nil {
		return nil, errors.New("cannot serialize nil ObfuscatedAsyncMessage")
	}
	return encodeGob(obfMsg, "obfuscated message")
}

// deserializeObfuscatedMessage converts bytes back to an ObfuscatedAsyncMessage
func (ac *AsyncClient) deserializeObfuscatedMessage(data []byte) (*ObfuscatedAsyncMessage, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot deserialize empty data")
	}

	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)

	var obfMsg ObfuscatedAsyncMessage
	err := decoder.Decode(&obfMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode obfuscated message: %w", err)
	}

	return &obfMsg, nil
}

// serializeRetrieveRequest converts an AsyncRetrieveRequest to bytes for network transmission
func (ac *AsyncClient) serializeRetrieveRequest(req *AsyncRetrieveRequest) ([]byte, error) {
	if req == nil {
		return nil, errors.New("cannot serialize nil AsyncRetrieveRequest")
	}
	return encodeGob(req, "retrieve request")
}

// serializeRetrieveResponse converts a list of obfuscated messages to bytes for network transmission.
// L-4 fix: Include RequestID in the serialized response for request correlation.
// Note: In production, the storage node should echo the RequestID from the request in the response.
func (ac *AsyncClient) serializeRetrieveResponse(requestID uint64, messages []*ObfuscatedAsyncMessage) ([]byte, error) {
	if len(messages) > MaxMessagesPerRecipient {
		return nil, fmt.Errorf("message count %d exceeds maximum %d per recipient", len(messages), MaxMessagesPerRecipient)
	}
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	// L-4 fix: Encode RequestID first so deserializer can extract it for correlation
	if err := encoder.Encode(requestID); err != nil {
		return nil, fmt.Errorf("failed to encode request ID: %w", err)
	}

	count := int32(len(messages))
	if err := encoder.Encode(count); err != nil {
		return nil, fmt.Errorf("failed to encode message count: %w", err)
	}
	for i, msg := range messages {
		if err := encoder.Encode(msg); err != nil {
			return nil, fmt.Errorf("failed to encode message %d: %w", i, err)
		}
	}
	return buf.Bytes(), nil
}

// deriveSharedSecret computes the shared secret with a recipient using ECDH
func (ac *AsyncClient) deriveSharedSecret(recipientPK [32]byte) ([32]byte, error) {
	// Use curve25519.X25519 for ECDH computation (replaces deprecated ScalarMult)
	// This is the same computation that NaCl box uses internally
	sharedSecret, err := curve25519.X25519(ac.keyPair.Private[:], recipientPK[:])
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	var result [32]byte
	copy(result[:], sharedSecret)
	return result, nil
}

// storeObfuscatedMessage stores an obfuscated message on multiple storage nodes.
// When erasure coding is enabled, the message is split into k=5 shards (3 data + 2 parity)
// and distributed across 5 storage nodes. This allows message reconstruction from any 3 nodes,
// providing 99.9% message survival even with 2 node failures.
func (ac *AsyncClient) storeObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage) error {
	// Snapshot the erasure-coding fields under a brief read lock so that
	// collectCandidateNodes can acquire its own RLock without nesting.
	ac.mutex.RLock()
	erasureEnabled := ac.erasureEnabled
	erasureStorage := ac.erasureStorage
	ac.mutex.RUnlock()

	if erasureEnabled && erasureStorage != nil {
		return ac.storeWithErasureCoding(obfMsg)
	}
	return ac.storeWithSimpleRedundancy(obfMsg)
}

// storeWithErasureCoding stores a message using Reed-Solomon erasure coding across k=5 nodes.
// This provides 2-of-5 failure tolerance, meaning the message can be reconstructed
// even if 2 out of 5 storage nodes fail or become unreachable.
func (ac *AsyncClient) storeWithErasureCoding(obfMsg *ObfuscatedAsyncMessage) error {
	serializedMsg, err := ac.serializeObfuscatedMessage(obfMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize message for erasure coding: %w", err)
	}

	shards, err := ac.erasureStorage.StoreMessage(obfMsg.MessageID, serializedMsg)
	if err != nil {
		return fmt.Errorf("failed to create erasure-coded shards: %w", err)
	}

	storageNodes := ac.findStorageNodes(obfMsg.RecipientPseudonym, 5)
	if len(storageNodes) < 3 {
		return errors.New("insufficient storage nodes available (need at least 3 for erasure coding)")
	}

	storedCount := ac.distributeShardsToNodes(shards, storageNodes, obfMsg, len(serializedMsg))

	if storedCount < 3 {
		return fmt.Errorf("failed to store sufficient shards: stored %d, need at least 3", storedCount)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "storeWithErasureCoding",
		"message_id":    fmt.Sprintf("%x", obfMsg.MessageID[:8]),
		"shards_total":  len(shards),
		"shards_stored": storedCount,
	}).Debug("Message stored with erasure coding")

	return nil
}

// distributeShardsToNodes distributes erasure-coded shards across storage nodes.
// Returns the number of successfully stored shards.
func (ac *AsyncClient) distributeShardsToNodes(shards []*EncodedShard, storageNodes []net.Addr, obfMsg *ObfuscatedAsyncMessage, originalSize int) int {
	storedCount := 0

	for i, shard := range shards {
		if i >= len(storageNodes) {
			break
		}

		if ac.storeShardOnNodeWithLogging(i, shard, storageNodes[i], obfMsg, originalSize) {
			storedCount++
		}
	}

	return storedCount
}

// storeShardOnNodeWithLogging stores a single shard on a node and logs failures.
func (ac *AsyncClient) storeShardOnNodeWithLogging(shardIndex int, shard *EncodedShard, node net.Addr, obfMsg *ObfuscatedAsyncMessage, originalSize int) bool {
	envelope, err := NewErasureShardEnvelope(shard, obfMsg.RecipientPseudonym, originalSize)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "storeWithErasureCoding",
			"shard":      shardIndex,
			"message_id": fmt.Sprintf("%x", obfMsg.MessageID[:8]),
			"error":      err.Error(),
		}).Warn("Failed to create shard envelope")
		return false
	}

	return ac.storeShardOnNode(node, envelope) == nil
}

// storeWithSimpleRedundancy stores a message using simple replication (legacy behavior).
func (ac *AsyncClient) storeWithSimpleRedundancy(obfMsg *ObfuscatedAsyncMessage) error {
	// Find suitable storage nodes from DHT
	storageNodes := ac.findStorageNodes(obfMsg.RecipientPseudonym, 3) // Use 3 nodes for redundancy
	if len(storageNodes) == 0 {
		return errors.New("no storage nodes available")
	}

	// Store obfuscated message on multiple nodes for redundancy
	storedCount := 0
	for _, nodeAddr := range storageNodes {
		if err := ac.storeObfuscatedMessageOnNode(nodeAddr, obfMsg); err == nil {
			storedCount++
		}
	}

	if storedCount == 0 {
		return errors.New("failed to store obfuscated message on any storage node")
	}

	return nil
}

// storeShardOnNode sends an erasure-coded shard to a specific storage node.
func (ac *AsyncClient) storeShardOnNode(nodeAddr net.Addr, envelope *ErasureShardEnvelope) error {
	if envelope == nil || envelope.Shard == nil {
		return errors.New("nil shard envelope")
	}

	// Serialize the shard envelope for network transmission
	serializedEnvelope, err := encodeGob(envelope, "ErasureShardEnvelope")
	if err != nil {
		return fmt.Errorf("failed to serialize shard envelope: %w", err)
	}

	// Check if transport is available
	if ac.transport == nil {
		return errors.New("async messaging unavailable: transport is nil")
	}

	// Create async store packet for erasure shard
	storePacket := &transport.Packet{
		PacketType: transport.PacketAsyncStore,
		Data:       serializedEnvelope,
	}

	// Send store request to storage node
	if err := ac.transport.Send(storePacket, nodeAddr); err != nil {
		return fmt.Errorf("failed to send shard to %v: %w", nodeAddr, err)
	}

	return nil
}

// storeObfuscatedMessageOnNode sends an obfuscated message to a specific storage node
func (ac *AsyncClient) storeObfuscatedMessageOnNode(nodeAddr net.Addr, obfMsg *ObfuscatedAsyncMessage) error {
	if obfMsg == nil {
		return errors.New("obfuscated message is nil")
	}

	// Serialize the obfuscated message for network transmission
	serializedMsg, err := ac.serializeObfuscatedMessage(obfMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize obfuscated message: %w", err)
	}

	// Check if transport is available
	if ac.transport == nil {
		return errors.New("async messaging unavailable: transport is nil")
	}

	// Create async store packet
	storePacket := &transport.Packet{
		PacketType: transport.PacketAsyncStore,
		Data:       serializedMsg,
	}

	// Send store request to storage node
	err = ac.transport.Send(storePacket, nodeAddr)
	if err != nil {
		return fmt.Errorf("failed to send store request to %v: %w", nodeAddr, err)
	}

	// In a production implementation, we would wait for and verify the response
	// For now, we assume success if the send operation succeeded
	return nil
}

// DecryptedMessage represents a decrypted async message
type DecryptedMessage struct {
	ID          [16]byte
	SenderPK    [32]byte
	Message     []byte
	MessageType MessageType
	Timestamp   time.Time
}

// AsyncRetrieveRequest represents a request to retrieve messages from a storage node
type AsyncRetrieveRequest struct {
	RequestID          uint64   // Unique request ID for correlating responses (L-4 fix)
	RecipientPseudonym [32]byte // Obfuscated recipient identity
	Epochs             []uint64 // Which epochs to retrieve messages from
}

// AsyncRetrieveResponse represents the response from a storage node retrieval request
type AsyncRetrieveResponse struct {
	RequestID uint64                    // Echo of the request ID for correlation (L-4 fix)
	Messages  []*ObfuscatedAsyncMessage // Retrieved messages
}

// nodeDistance represents a storage node candidate with its distance from target
type nodeDistance struct {
	addr     net.Addr
	distance uint64
}

// findStorageNodes identifies DHT nodes that can serve as storage nodes
// Uses consistent hashing to select nodes closest to the recipient's public key
func (ac *AsyncClient) findStorageNodes(targetPK [32]byte, maxNodes int) []net.Addr {
	targetHash := ac.calculateNodeHash(targetPK)
	candidates := ac.collectCandidateNodes(targetHash)
	ac.sortCandidatesByDistance(candidates)
	return ac.selectClosestNodes(candidates, maxNodes)
}

// collectCandidateNodes calculates distance from target for each known storage node.
func (ac *AsyncClient) collectCandidateNodes(targetHash uint64) []nodeDistance {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	var candidates []nodeDistance
	for pk, addr := range ac.storageNodes {
		nodeHash := ac.calculateNodeHash(pk)
		distance := ac.calculateHashDistance(targetHash, nodeHash)
		candidates = append(candidates, nodeDistance{addr: addr, distance: distance})
	}
	return candidates
}

// sortCandidatesByDistance sorts candidates by distance using standard library sort
func (ac *AsyncClient) sortCandidatesByDistance(candidates []nodeDistance) {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})
}

// selectClosestNodes returns the closest nodes up to maxNodes limit
func (ac *AsyncClient) selectClosestNodes(candidates []nodeDistance, maxNodes int) []net.Addr {
	var nodes []net.Addr
	for i, candidate := range candidates {
		if i >= maxNodes {
			break
		}
		nodes = append(nodes, candidate.addr)
	}
	return nodes
}

// findStorageNodesFromSnapshot identifies storage nodes closest to targetPK using
// a pre-snapshotted map, avoiding any lock acquisition.
func (ac *AsyncClient) findStorageNodesFromSnapshot(targetPK [32]byte, maxNodes int, storageNodesMap map[[32]byte]net.Addr) []net.Addr {
	targetHash := ac.calculateNodeHash(targetPK)
	var candidates []nodeDistance
	for pk, addr := range storageNodesMap {
		nodeHash := ac.calculateNodeHash(pk)
		distance := ac.calculateHashDistance(targetHash, nodeHash)
		candidates = append(candidates, nodeDistance{addr: addr, distance: distance})
	}
	ac.sortCandidatesByDistance(candidates)
	return ac.selectClosestNodes(candidates, maxNodes)
}

// collectMessagesFromNodesLockFree retrieves and decrypts messages without
// re-acquiring the mutex; config values are passed in directly.
func (ac *AsyncClient) collectMessagesFromNodesLockFree(storageNodes []net.Addr, pseudonym [32]byte, epoch uint64, collectionTimeout time.Duration, parallelizeQueries bool, retrieveTimeout time.Duration) []DecryptedMessage {
	ctx, cancel := context.WithTimeout(context.Background(), collectionTimeout)
	defer cancel()

	if parallelizeQueries {
		return ac.collectMessagesParallel(ctx, storageNodes, pseudonym, epoch, retrieveTimeout)
	}
	return ac.collectMessagesSequential(ctx, storageNodes, pseudonym, epoch, retrieveTimeout)
}

// AddStorageNode adds a known storage node to the client
func (ac *AsyncClient) AddStorageNode(publicKey [32]byte, addr net.Addr) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	ac.storageNodes[publicKey] = addr
}

// AddKnownSender adds a sender's public key to the known senders list for message decryption.
// This is required for the client to attempt decryption of obfuscated messages from this sender.
func (ac *AsyncClient) AddKnownSender(senderPK [32]byte) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	ac.knownSenders[senderPK] = true
}

// RemoveKnownSender removes a sender's public key from the known senders list
func (ac *AsyncClient) RemoveKnownSender(senderPK [32]byte) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	delete(ac.knownSenders, senderPK)
}

// GetKnownSenders returns a copy of the known senders list
func (ac *AsyncClient) GetKnownSenders() map[[32]byte]bool {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	result := make(map[[32]byte]bool)
	for k, v := range ac.knownSenders {
		result[k] = v
	}
	return result
}

// SetForwardSecurityManager wires the ForwardSecurityManager into the client so
// that SendAsyncMessage uses one-time pre-key encryption instead of plaintext
// data in the ForwardSecureMessage envelope.  AsyncManager sets this automatically
// during construction; call it explicitly only when using AsyncClient standalone.
func (ac *AsyncClient) SetForwardSecurityManager(fsm *ForwardSecurityManager) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	ac.forwardSecurity = fsm
}

// GetLastRetrieveTime returns when messages were last retrieved
func (ac *AsyncClient) GetLastRetrieveTime() time.Time {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.lastRetrieve
}

// SendForwardSecureAsyncMessage stores a forward-secure message for offline delivery using obfuscation.
// This method automatically provides peer identity obfuscation for privacy protection.
func (ac *AsyncClient) SendForwardSecureAsyncMessage(fsMsg *ForwardSecureMessage) error {
	if fsMsg == nil {
		return errors.New("nil forward secure message")
	}

	// Use the obfuscated message sending method for privacy protection
	return ac.SendObfuscatedMessage(fsMsg.RecipientPK, fsMsg)
}

// retrieveObfuscatedMessagesFromNode retrieves obfuscated messages from a specific storage node
func (ac *AsyncClient) retrieveObfuscatedMessagesFromNode(nodeAddr net.Addr,
	recipientPseudonym [32]byte, epochs []uint64, timeout time.Duration,
) ([]*ObfuscatedAsyncMessage, error) {
	serializedRequest, requestID, err := ac.prepareRetrieveRequest(recipientPseudonym, epochs)
	if err != nil {
		return nil, err
	}

	// Register the response channel BEFORE sending so a fast response is never missed.
	responseChan := ac.setupResponseChannel(nodeAddr, requestID)
	defer ac.cleanupResponseChannel(nodeAddr, requestID)

	if err := ac.sendRetrieveRequest(serializedRequest, nodeAddr); err != nil {
		return nil, err
	}

	return ac.waitForRetrieveResponse(responseChan, nodeAddr, timeout)
}

// prepareRetrieveRequest creates and serializes a retrieve request.
// Returns the serialized request bytes, the unique request ID, and any error.
// L-4 fix: Generate unique RequestID for each request to enable concurrent request correlation.
func (ac *AsyncClient) prepareRetrieveRequest(recipientPseudonym [32]byte, epochs []uint64) ([]byte, uint64, error) {
	// Generate a unique request ID for correlation (L-4 fix)
	// Use binary.Read directly to avoid intermediate allocation
	var requestID uint64
	if err := binary.Read(rand.Reader, binary.LittleEndian, &requestID); err != nil {
		return nil, 0, fmt.Errorf("failed to generate request ID: %w", err)
	}

	retrieveRequest := &AsyncRetrieveRequest{
		RequestID:          requestID,
		RecipientPseudonym: recipientPseudonym,
		Epochs:             epochs,
	}

	serializedRequest, err := ac.serializeRetrieveRequest(retrieveRequest)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to serialize retrieve request: %w", err)
	}

	return serializedRequest, requestID, nil
}

// sendRetrieveRequest sends the retrieve request packet to a storage node.
func (ac *AsyncClient) sendRetrieveRequest(serializedRequest []byte, nodeAddr net.Addr) error {
	if ac.transport == nil {
		return errors.New("async messaging unavailable: transport is nil")
	}

	retrievePacket := &transport.Packet{
		PacketType: transport.PacketAsyncRetrieve,
		Data:       serializedRequest,
	}

	err := ac.transport.Send(retrievePacket, nodeAddr)
	if err != nil {
		return fmt.Errorf("failed to send retrieve request to %v: %w", nodeAddr, err)
	}

	return nil
}

// setupResponseChannel creates and registers a response channel for the node and request ID.
// L-4 fix: Use composite key (nodeAddr:requestID) to support concurrent requests to same node.
func (ac *AsyncClient) setupResponseChannel(nodeAddr net.Addr, requestID uint64) chan retrieveResponse {
	nodeKey := fmt.Sprintf("%s:%d", nodeAddr.String(), requestID)
	responseChan := make(chan retrieveResponse, 1)

	ac.channelMutex.Lock()
	ac.retrieveChannels[nodeKey] = responseChan
	ac.channelMutex.Unlock()

	return responseChan
}

// cleanupResponseChannel removes the response channel for the node and request ID.
// The channel is intentionally left open: sendResponseToChannel uses a
// non-blocking select, so any late delivery after cleanup is silently
// discarded rather than panicking on a closed channel.
// L-4 fix: Use composite key (nodeAddr:requestID) to support concurrent requests to same node.
func (ac *AsyncClient) cleanupResponseChannel(nodeAddr net.Addr, requestID uint64) {
	nodeKey := fmt.Sprintf("%s:%d", nodeAddr.String(), requestID)
	ac.channelMutex.Lock()
	delete(ac.retrieveChannels, nodeKey)
	ac.channelMutex.Unlock()
}

// waitForRetrieveResponse waits for a response from the storage node or times out.
func (ac *AsyncClient) waitForRetrieveResponse(responseChan chan retrieveResponse, nodeAddr net.Addr, timeout time.Duration) ([]*ObfuscatedAsyncMessage, error) {
	select {
	case response := <-responseChan:
		if response.err != nil {
			return nil, fmt.Errorf("retrieve response error: %w", response.err)
		}
		return response.messages, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for retrieve response from %v after %v", nodeAddr, timeout)
	}
}

// handleRetrieveResponse processes incoming PacketAsyncRetrieveResponse packets
// L-4 fix: Extract RequestID from response to route to correct request handler
func (ac *AsyncClient) handleRetrieveResponse(packet *transport.Packet, addr net.Addr) error {
	requestID, err := ac.decodeRetrieveResponseRequestID(packet.Data)
	if err != nil {
		log.Printf("AsyncClient: Failed to decode retrieve response request ID from %v: %v", addr, err)
		return nil
	}

	// L-4 fix: Use RequestID to find the correct response channel for this request
	responseChan := ac.findResponseChannel(addr, requestID)
	if responseChan == nil {
		log.Printf("AsyncClient: Received unexpected retrieve response from %v with request ID %d", addr, requestID)
		return nil
	}

	// Deserialize the full response after locating the waiting goroutine.
	response, err := ac.deserializeRetrieveResponse(packet.Data)
	if err != nil {
		log.Printf("AsyncClient: Failed to deserialize retrieve response from %v (request ID %d): %v", addr, requestID, err)
		ac.sendResponseToChannel(responseChan, ac.buildRetrieveResponse(nil, err))
		return nil
	}

	retrieveResponse := ac.buildRetrieveResponse(response.Messages, nil)
	ac.sendResponseToChannel(responseChan, retrieveResponse)

	return err
}

// findResponseChannel locates the response channel for the given address and request ID.
// L-4 fix: Use composite key (nodeAddr:requestID) to support concurrent requests to same node.
func (ac *AsyncClient) findResponseChannel(addr net.Addr, requestID uint64) chan retrieveResponse {
	nodeKey := fmt.Sprintf("%s:%d", addr.String(), requestID)
	ac.channelMutex.Lock()
	defer ac.channelMutex.Unlock()

	responseChan, exists := ac.retrieveChannels[nodeKey]
	if !exists {
		return nil
	}
	return responseChan
}

// buildRetrieveResponse creates a response struct from messages or error.
func (ac *AsyncClient) buildRetrieveResponse(messages []*ObfuscatedAsyncMessage, err error) retrieveResponse {
	if err != nil {
		return retrieveResponse{err: fmt.Errorf("failed to deserialize response: %w", err)}
	}
	return retrieveResponse{messages: messages}
}

// sendResponseToChannel sends the response to the channel without blocking.
// The channel has capacity 1; if the slot is already taken or the receiver
// timed out and cleanupResponseChannel has removed the entry, the default
// branch discards the response safely — no close, no panic.
func (ac *AsyncClient) sendResponseToChannel(responseChan chan retrieveResponse, response retrieveResponse) {
	select {
	case responseChan <- response:
	default:
	}
}

// decodeRetrieveResponseRequestID extracts only the request ID so callers can
// route decode failures to the waiting request goroutine.
func (ac *AsyncClient) decodeRetrieveResponseRequestID(data []byte) (uint64, error) {
	if len(data) == 0 {
		return 0, errors.New("cannot decode request ID from empty response data")
	}

	if err := limits.ValidateProcessingBuffer(data); err != nil {
		return 0, fmt.Errorf("retrieve response buffer exceeds maximum size: %w", err)
	}

	decoder := gob.NewDecoder(bytes.NewBuffer(data))
	var requestID uint64
	if err := decoder.Decode(&requestID); err != nil {
		return 0, fmt.Errorf("failed to decode request ID: %w", err)
	}

	return requestID, nil
}

// deserializeRetrieveResponse converts response bytes to an AsyncRetrieveResponse with RequestID and messages
// L-4 fix: Extract RequestID from response for request correlation with concurrent requests
// M-1 remediation: reads the element count first and validates it against MaxMessagesPerRecipient
// before allocating or decoding any message data, preventing gob allocation-DoS via a crafted count.
// Also validates buffer size to prevent memory exhaustion attacks.
func (ac *AsyncClient) deserializeRetrieveResponse(data []byte) (*AsyncRetrieveResponse, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot deserialize empty response data")
	}

	// Validate buffer size to prevent gob decoder allocation attacks
	// M-1 remediation: reject oversized inputs before decoding
	if err := limits.ValidateProcessingBuffer(data); err != nil {
		return nil, fmt.Errorf("retrieve response buffer exceeds maximum size: %w", err)
	}

	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)

	// L-4 fix: Decode RequestID first to correlate response with correct request
	var requestID uint64
	if err := decoder.Decode(&requestID); err != nil {
		return nil, fmt.Errorf("failed to decode request ID: %w", err)
	}

	// M-1 remediation: decode the element count first so the allocation is bounded before
	// reading any message data.  A malicious peer can no longer trigger a large slice
	// allocation by declaring a huge count in the gob wire format.
	var count int32
	if err := decoder.Decode(&count); err != nil {
		return nil, fmt.Errorf("failed to decode message count: %w", err)
	}
	if count < 0 || int(count) > MaxMessagesPerRecipient {
		return nil, fmt.Errorf("decoded message count %d exceeds maximum %d per recipient", count, MaxMessagesPerRecipient)
	}

	messages := make([]*ObfuscatedAsyncMessage, 0, count)
	for i := int32(0); i < count; i++ {
		var msg ObfuscatedAsyncMessage
		if err := decoder.Decode(&msg); err != nil {
			return nil, fmt.Errorf("failed to decode message %d: %w", i, err)
		}
		messages = append(messages, &msg)
	}

	return &AsyncRetrieveResponse{
		RequestID: requestID,
		Messages:  messages,
	}, nil
}

// decryptObfuscatedMessage attempts to decrypt an obfuscated message
func (ac *AsyncClient) decryptObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage) (*ForwardSecureMessage, error) {
	if len(ac.knownSenders) == 0 {
		logrus.Warn("AsyncClient: decryptObfuscatedMessage called with empty knownSenders — " +
			"all messages will fail decryption. Call AddKnownSender for each friend public key, " +
			"or use AsyncManager.AddFriend to populate the list automatically.")
		return nil, errors.New("no known senders configured - cannot decrypt message without sender identification")
	}
	for senderPK := range ac.knownSenders {
		if msg, err := ac.tryDecryptWithAllKeys(obfMsg, senderPK); err == nil {
			return msg, nil
		}
	}
	return nil, errors.New("could not decrypt message with any available key")
}

// tryDecryptWithAllKeys tries the current key and any previous rotated keys for a given sender.
func (ac *AsyncClient) tryDecryptWithAllKeys(obfMsg *ObfuscatedAsyncMessage, senderPK [32]byte) (*ForwardSecureMessage, error) {
	if msg, err := ac.tryDecryptWithKeys(obfMsg, senderPK, ac.keyPair); err == nil {
		return msg, nil
	}
	if ac.keyRotation != nil {
		previousKeys := ac.keyRotation.GetPreviousKeys()
		for _, prevKey := range previousKeys {
			if msg, err := ac.tryDecryptWithKeys(obfMsg, senderPK, prevKey); err == nil {
				return msg, nil
			}
		}
	}
	return nil, errors.New("decryption failed for sender")
}

// tryDecryptWithKeys attempts to decrypt a message using a specific recipient key pair
func (ac *AsyncClient) tryDecryptWithKeys(obfMsg *ObfuscatedAsyncMessage, senderPK [32]byte, recipientKey *crypto.KeyPair) (*ForwardSecureMessage, error) {
	// Derive shared secret for this sender
	sharedSecret, err := crypto.DeriveSharedSecret(senderPK, recipientKey.Private)
	if err != nil {
		return nil, err
	}

	// Use the obfuscation manager to decrypt the payload
	decryptedPayload, err := ac.obfuscation.DecryptObfuscatedMessage(obfMsg, recipientKey.Private, senderPK, sharedSecret)
	if err != nil {
		return nil, err
	}

	// Deserialize the inner ForwardSecureMessage
	forwardSecureMsg, err := ac.deserializeForwardSecureMessage(decryptedPayload)
	if err != nil {
		return nil, err
	}

	// Verify the message is intended for us (constant-time comparison to avoid timing side-channel)
	if subtle.ConstantTimeCompare(forwardSecureMsg.RecipientPK[:], recipientKey.Public[:]) != 1 {
		return nil, errors.New("message recipient public key doesn't match ours")
	}

	return forwardSecureMsg, nil
}

// tryDecryptFromKnownSenders attempts to decrypt an obfuscated message by trying
// all known senders until one succeeds. This implements sender identification
// through cryptographic trial and error.
func (ac *AsyncClient) tryDecryptFromKnownSenders(obfMsg *ObfuscatedAsyncMessage) (DecryptedMessage, error) {
	// Derive the shared secret for payload decryption
	// Since we don't know the sender yet, we'll need to try all known contacts
	// For now, we'll implement a simplified version that assumes a single known sender

	// In a production system, this would iterate through a contact list
	// For this implementation, we'll create a basic framework that can be extended
	var lastErr error

	// Try to decrypt using the obfuscation manager's decrypt function
	// This requires us to know the sender and derive the shared secret
	// For the basic implementation, we'll assume the sender public key can be
	// derived from the sender pseudonym (which it cannot in the real system)

	// Since we can't reverse the sender pseudonym to get the real sender PK,
	// we need a different approach. In practice, this would:
	// 1. Maintain a list of known friends/contacts
	// 2. For each contact, derive the expected sender pseudonym
	// 3. Try decryption with that contact's keys
	// 4. Return success on first successful decryption

	// For now, implement a simplified version that demonstrates the flow
	// but requires the sender to be added to a known senders list
	if len(ac.knownSenders) == 0 {
		logrus.Warn("AsyncClient: RetrieveObfuscatedMessages called with empty knownSenders — " +
			"all messages will fail decryption. Call AddKnownSender for each friend public key, " +
			"or use AsyncManager.AddFriend to populate the list automatically.")
		return DecryptedMessage{}, errors.New("no known senders configured - cannot decrypt message without sender identification")
	}

	for senderPK := range ac.knownSenders {
		decrypted, err := ac.tryDecryptWithSender(obfMsg, senderPK)
		if err == nil {
			return decrypted, nil
		}
		lastErr = err
	}

	return DecryptedMessage{}, fmt.Errorf("failed to decrypt message with any known sender: %w", lastErr)
}

// tryDecryptWithSender attempts to decrypt an obfuscated message using a specific sender's keys
func (ac *AsyncClient) tryDecryptWithSender(obfMsg *ObfuscatedAsyncMessage, senderPK [32]byte) (DecryptedMessage, error) {
	forwardSecureMsg, err := ac.decryptForwardSecureMessageFromSender(obfMsg, senderPK)
	if err != nil {
		return DecryptedMessage{}, err
	}

	plaintext, err := ac.decryptMessagePayload(forwardSecureMsg)
	if err != nil {
		return DecryptedMessage{}, err
	}

	return buildDecryptedMessage(forwardSecureMsg, plaintext), nil
}

func (ac *AsyncClient) decryptForwardSecureMessageFromSender(
	obfMsg *ObfuscatedAsyncMessage,
	senderPK [32]byte,
) (*ForwardSecureMessage, error) {
	sharedSecret, err := ac.deriveSharedSecret(senderPK)
	if err != nil {
		return nil, fmt.Errorf("failed to derive shared secret with sender %x: %w", senderPK[:8], err)
	}

	decryptedPayload, err := ac.obfuscation.DecryptObfuscatedMessage(obfMsg, ac.keyPair.Private, senderPK, sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt obfuscated payload: %w", err)
	}

	forwardSecureMsg, err := ac.deserializeForwardSecureMessage(decryptedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize ForwardSecureMessage: %w", err)
	}
	if forwardSecureMsg.SenderPK != senderPK {
		return nil, errors.New("sender public key mismatch in ForwardSecureMessage")
	}
	return forwardSecureMsg, nil
}

func (ac *AsyncClient) decryptMessagePayload(forwardSecureMsg *ForwardSecureMessage) ([]byte, error) {
	fsm := ac.getForwardSecurityManager()
	if fsm != nil && forwardSecureMsg.PreKeyID != 0 {
		plaintext, err := fsm.DecryptForwardSecureMessage(forwardSecureMsg)
		if err != nil {
			return nil, fmt.Errorf("FSM inner-layer decryption failed: %w", err)
		}
		plaintext, err = UnpadMessage(plaintext)
		if err != nil {
			return nil, fmt.Errorf("failed to unpad FSM-decrypted message: %w", err)
		}
		return plaintext, nil
	}

	plaintext, err := UnpadMessage(forwardSecureMsg.EncryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to unpad message: %w", err)
	}
	return plaintext, nil
}

// calculateNodeHash creates a hash from a public key for consistent hashing
func (ac *AsyncClient) calculateNodeHash(pk [32]byte) uint64 {
	// Simple hash function - in production would use a better hash like SHA256
	var hash uint64
	for i := 0; i < len(pk); i += 8 {
		var chunk uint64
		for j := 0; j < 8 && i+j < len(pk); j++ {
			chunk |= uint64(pk[i+j]) << (j * 8)
		}
		hash ^= chunk
	}
	return hash
}

// calculateHashDistance calculates XOR distance between two hashes (Kademlia-style)
func (ac *AsyncClient) calculateHashDistance(hash1, hash2 uint64) uint64 {
	return hash1 ^ hash2
}
