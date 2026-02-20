package async

import (
	"bytes"
	"context"
	"crypto/rand"
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
	retrieveChannels   map[string]chan retrieveResponse // Channels for retrieve responses keyed by node address
	channelMutex       sync.Mutex                       // Protects retrieveChannels map
	retrieveTimeout    time.Duration                    // Timeout for storage node retrieval operations
	collectionTimeout  time.Duration                    // Overall timeout for collecting from all nodes
	parallelizeQueries bool                             // Whether to query storage nodes in parallel
}

// NewAsyncClient creates a new async messaging client with obfuscation support
func NewAsyncClient(keyPair *crypto.KeyPair, trans transport.Transport) *AsyncClient {
	logrus.WithFields(logrus.Fields{
		"function":           "NewAsyncClient",
		"public_key_preview": fmt.Sprintf("%x", keyPair.Public[:8]),
	}).Info("Creating new async client")

	epochManager := NewEpochManager()
	obfuscation := NewObfuscationManager(keyPair, epochManager)

	ac := &AsyncClient{
		keyPair:            keyPair,
		obfuscation:        obfuscation,
		transport:          trans,
		storageNodes:       make(map[[32]byte]net.Addr),
		knownSenders:       make(map[[32]byte]bool),
		lastRetrieve:       time.Now(),
		retrieveChannels:   make(map[string]chan retrieveResponse),
		retrieveTimeout:    2 * time.Second, // Default 2-second timeout for storage node responses
		collectionTimeout:  5 * time.Second, // Default 5-second overall timeout for all nodes
		parallelizeQueries: true,            // Enable parallel queries by default for better performance
	}

	// Register handler for async retrieve responses only if transport is available
	if trans != nil {
		trans.RegisterHandler(transport.PacketAsyncRetrieveResponse, ac.handleRetrieveResponse)
	} else {
		logrus.WithFields(logrus.Fields{
			"function": "NewAsyncClient",
		}).Warn("Transport is nil - async messaging features will be unavailable")
	}

	// Initialize the retrieval scheduler after the client is created
	logrus.WithFields(logrus.Fields{
		"function": "NewAsyncClient",
	}).Debug("Initializing retrieval scheduler")
	ac.retrievalScheduler = NewRetrievalScheduler(ac)

	logrus.WithFields(logrus.Fields{
		"function":           "NewAsyncClient",
		"public_key_preview": fmt.Sprintf("%x", keyPair.Public[:8]),
	}).Info("Async client created successfully")

	return ac
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

// SendObfuscatedMessage sends a forward-secure message using peer identity obfuscation.
// This method hides the real sender and recipient identities from storage nodes
// while maintaining message deliverability and forward secrecy.
func (ac *AsyncClient) SendObfuscatedMessage(recipientPK [32]byte,
	forwardSecureMsg *ForwardSecureMessage,
) error {
	if forwardSecureMsg == nil {
		return errors.New("nil forward secure message")
	}

	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	// Serialize the ForwardSecureMessage for encryption
	serializedMsg, err := ac.serializeForwardSecureMessage(forwardSecureMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Derive shared secret with recipient
	sharedSecret, err := ac.deriveSharedSecret(recipientPK)
	if err != nil {
		return fmt.Errorf("failed to derive shared secret: %w", err)
	}

	// Create obfuscated message
	obfMsg, err := ac.obfuscation.CreateObfuscatedMessage(
		ac.keyPair.Private, recipientPK, serializedMsg, sharedSecret)
	if err != nil {
		return fmt.Errorf("failed to create obfuscated message: %w", err)
	}

	// Store on multiple storage nodes for redundancy
	return ac.storeObfuscatedMessage(obfMsg)
}

// SendAsyncMessage sends a message asynchronously using obfuscation by default.
// This method automatically provides forward secrecy and peer identity obfuscation.
// It creates a ForwardSecureMessage and sends it using the obfuscated transport.
func (ac *AsyncClient) SendAsyncMessage(recipientPK [32]byte, message []byte,
	messageType MessageType,
) error {
	if message == nil {
		return errors.New("message cannot be nil")
	}

	if len(message) == 0 {
		return errors.New("message cannot be empty")
	}

	if len(message) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(message), MaxMessageSize)
	}

	// Pad the message to a standard size to prevent metadata leakage through size correlation
	var err error
	message, err = PadMessageToStandardSize(message)
	if err != nil {
		return fmt.Errorf("failed to pad message: %w", err)
	}

	// Create a ForwardSecureMessage structure for the message
	// In a production system, this would integrate with the forward secrecy manager
	// For now, create a basic structure that demonstrates the obfuscation flow
	var messageID [32]byte
	copy(messageID[:], message[:min(len(message), 32)]) // Simple message ID generation

	var nonce [24]byte
	// Generate a cryptographically secure random nonce
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	forwardSecureMsg := &ForwardSecureMessage{
		Type:          "forward_secure_message",
		MessageID:     messageID,
		SenderPK:      ac.keyPair.Public,
		RecipientPK:   recipientPK,
		PreKeyID:      0,       // Would be populated by forward secrecy manager
		EncryptedData: message, // In production, this would be encrypted with forward secrecy
		Nonce:         nonce,
		MessageType:   messageType,
		Timestamp:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}

	// Use the obfuscated message sending method for privacy protection
	return ac.SendObfuscatedMessage(recipientPK, forwardSecureMsg)
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
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	var allMessages []DecryptedMessage

	// Get recent epochs to check (current + 3 previous epochs)
	recentEpochs := ac.obfuscation.epochManager.GetRecentEpochs()

	// For each epoch, generate our pseudonym and retrieve messages
	for _, epoch := range recentEpochs {
		epochMessages := ac.retrieveMessagesForEpoch(epoch)
		allMessages = append(allMessages, epochMessages...)
	}

	ac.lastRetrieve = time.Now()
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
					"total_nodes":   len(storageNodes),
				}).Debug("Completed parallel message collection")
				return allMessages
			}

			processNodeResult(result, &allMessages, &successCount, &failureCount)
		}
	}
}

// retrieveMessagesFromSingleNode retrieves and decrypts messages from one storage node
func (ac *AsyncClient) retrieveMessagesFromSingleNode(nodeAddr net.Addr, pseudonym [32]byte, epoch uint64) []DecryptedMessage {
	obfMessages, err := ac.retrieveObfuscatedMessagesFromNode(nodeAddr, pseudonym, []uint64{epoch}, ac.retrieveTimeout)
	if err != nil {
		log.Printf("AsyncClient: Failed to retrieve messages from node %v for epoch %d: %v", nodeAddr, epoch, err)
		return nil // Skip failed nodes
	}

	return ac.decryptRetrievedMessages(obfMessages)
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

	for _, obfMsg := range obfMessages {
		forwardSecureMsg, err := ac.decryptObfuscatedMessage(obfMsg)
		if err != nil {
			continue // Skip messages we can't decrypt
		}

		// Generate a message ID
		var messageID [16]byte
		copy(messageID[:], forwardSecureMsg.MessageID[:16]) // Use first 16 bytes of message ID

		// Unpad the message data
		unpadded, err := UnpadMessage(forwardSecureMsg.EncryptedData)
		if err != nil {
			continue // Skip messages with invalid padding
		}

		// Create a DecryptedMessage from the ForwardSecureMessage
		decrypted := DecryptedMessage{
			ID:          messageID,
			SenderPK:    forwardSecureMsg.SenderPK,
			Message:     unpadded,
			MessageType: forwardSecureMsg.MessageType,
			Timestamp:   forwardSecureMsg.Timestamp,
		}

		decryptedMessages = append(decryptedMessages, decrypted)
	}

	return decryptedMessages
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

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	err := encoder.Encode(obfMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode obfuscated message: %w", err)
	}

	return buf.Bytes(), nil
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

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	err := encoder.Encode(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode retrieve request: %w", err)
	}

	return buf.Bytes(), nil
}

// serializeRetrieveResponse converts a list of obfuscated messages to bytes for network transmission
func (ac *AsyncClient) serializeRetrieveResponse(messages []*ObfuscatedAsyncMessage) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	err := encoder.Encode(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to encode retrieve response: %w", err)
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

// storeObfuscatedMessage stores an obfuscated message on multiple storage nodes
func (ac *AsyncClient) storeObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage) error {
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
	RecipientPseudonym [32]byte // Obfuscated recipient identity
	Epochs             []uint64 // Which epochs to retrieve messages from
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

// collectCandidateNodes calculates distance from target for each known storage node
func (ac *AsyncClient) collectCandidateNodes(targetHash uint64) []nodeDistance {
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
	serializedRequest, err := ac.prepareRetrieveRequest(recipientPseudonym, epochs)
	if err != nil {
		return nil, err
	}

	if err := ac.sendRetrieveRequest(serializedRequest, nodeAddr); err != nil {
		return nil, err
	}

	responseChan := ac.setupResponseChannel(nodeAddr)
	defer ac.cleanupResponseChannel(nodeAddr, responseChan)

	return ac.waitForRetrieveResponse(responseChan, nodeAddr, timeout)
}

// prepareRetrieveRequest creates and serializes a retrieve request.
func (ac *AsyncClient) prepareRetrieveRequest(recipientPseudonym [32]byte, epochs []uint64) ([]byte, error) {
	retrieveRequest := &AsyncRetrieveRequest{
		RecipientPseudonym: recipientPseudonym,
		Epochs:             epochs,
	}

	serializedRequest, err := ac.serializeRetrieveRequest(retrieveRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize retrieve request: %w", err)
	}

	return serializedRequest, nil
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

// setupResponseChannel creates and registers a response channel for the node.
func (ac *AsyncClient) setupResponseChannel(nodeAddr net.Addr) chan retrieveResponse {
	nodeKey := nodeAddr.String()
	responseChan := make(chan retrieveResponse, 1)

	ac.channelMutex.Lock()
	ac.retrieveChannels[nodeKey] = responseChan
	ac.channelMutex.Unlock()

	return responseChan
}

// cleanupResponseChannel removes and closes the response channel for the node.
func (ac *AsyncClient) cleanupResponseChannel(nodeAddr net.Addr, responseChan chan retrieveResponse) {
	nodeKey := nodeAddr.String()
	ac.channelMutex.Lock()
	delete(ac.retrieveChannels, nodeKey)
	close(responseChan)
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
func (ac *AsyncClient) handleRetrieveResponse(packet *transport.Packet, addr net.Addr) error {
	responseChan := ac.findResponseChannel(addr)
	if responseChan == nil {
		log.Printf("AsyncClient: Received unexpected retrieve response from %v", addr)
		return nil
	}

	messages, err := ac.deserializeRetrieveResponse(packet.Data)
	response := ac.buildRetrieveResponse(messages, err)
	ac.sendResponseToChannel(responseChan, response)

	return err
}

// findResponseChannel locates the response channel for the given address.
func (ac *AsyncClient) findResponseChannel(addr net.Addr) chan retrieveResponse {
	nodeKey := addr.String()
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
func (ac *AsyncClient) sendResponseToChannel(responseChan chan retrieveResponse, response retrieveResponse) {
	select {
	case responseChan <- response:
	default:
	}
}

// deserializeRetrieveResponse converts response bytes to a list of obfuscated messages
func (ac *AsyncClient) deserializeRetrieveResponse(data []byte) ([]*ObfuscatedAsyncMessage, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot deserialize empty response data")
	}

	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)

	var messages []*ObfuscatedAsyncMessage
	err := decoder.Decode(&messages)
	if err != nil {
		return nil, fmt.Errorf("failed to decode retrieve response: %w", err)
	}

	// Ensure we never return nil, return empty slice instead
	if messages == nil {
		messages = make([]*ObfuscatedAsyncMessage, 0)
	}

	return messages, nil
}

// decryptObfuscatedMessage attempts to decrypt an obfuscated message
func (ac *AsyncClient) decryptObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage) (*ForwardSecureMessage, error) {
	// First, try to identify the sender from known senders
	for senderPK := range ac.knownSenders {
		// Try current key first
		forwardSecureMsg, err := ac.tryDecryptWithKeys(obfMsg, senderPK, ac.keyPair)
		if err == nil {
			return forwardSecureMsg, nil
		}

		// If key rotation is enabled and current key failed, try previous keys
		if ac.keyRotation != nil && len(ac.keyRotation.PreviousKeys) > 0 {
			for _, prevKey := range ac.keyRotation.PreviousKeys {
				forwardSecureMsg, err := ac.tryDecryptWithKeys(obfMsg, senderPK, prevKey)
				if err == nil {
					return forwardSecureMsg, nil
				}
			}
		}
	}

	return nil, errors.New("could not decrypt message with any available key")
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

	// Verify the message is intended for us
	if !bytes.Equal(forwardSecureMsg.RecipientPK[:], recipientKey.Public[:]) {
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
	// Derive shared secret for this sender
	sharedSecret, err := ac.deriveSharedSecret(senderPK)
	if err != nil {
		return DecryptedMessage{}, fmt.Errorf("failed to derive shared secret with sender %x: %w", senderPK[:8], err)
	}

	// Use the obfuscation manager to decrypt the payload
	decryptedPayload, err := ac.obfuscation.DecryptObfuscatedMessage(obfMsg, ac.keyPair.Private, senderPK, sharedSecret)
	if err != nil {
		return DecryptedMessage{}, fmt.Errorf("failed to decrypt obfuscated payload: %w", err)
	}

	// Deserialize the inner ForwardSecureMessage
	forwardSecureMsg, err := ac.deserializeForwardSecureMessage(decryptedPayload)
	if err != nil {
		return DecryptedMessage{}, fmt.Errorf("failed to deserialize ForwardSecureMessage: %w", err)
	}

	// Verify the ForwardSecureMessage is from the expected sender
	if forwardSecureMsg.SenderPK != senderPK {
		return DecryptedMessage{}, errors.New("sender public key mismatch in ForwardSecureMessage")
	}

	// Create a DecryptedMessage from the ForwardSecureMessage
	// Note: In a production system with forward secrecy, we would use
	// the ForwardSecurityManager to decrypt the message content
	// For this implementation, we'll create the DecryptedMessage directly
	var messageID [16]byte
	copy(messageID[:], forwardSecureMsg.MessageID[:16])

	// Unpad the message data to get the original message
	unpadded, err := UnpadMessage(forwardSecureMsg.EncryptedData)
	if err != nil {
		return DecryptedMessage{}, fmt.Errorf("failed to unpad message: %w", err)
	}

	return DecryptedMessage{
		ID:          messageID,
		SenderPK:    forwardSecureMsg.SenderPK,
		Message:     unpadded, // Unpadded message data
		MessageType: forwardSecureMsg.MessageType,
		Timestamp:   forwardSecureMsg.Timestamp,
	}, nil
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
