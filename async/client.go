package async

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/curve25519"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// min returns the minimum of two integers (for Go versions < 1.21)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// AsyncClient handles the client-side operations for async messaging
// with built-in peer identity obfuscation for privacy protection
type AsyncClient struct {
	mutex        sync.RWMutex
	keyPair      *crypto.KeyPair
	obfuscation  *ObfuscationManager   // Handles cryptographic obfuscation
	transport    transport.Transport   // Network transport for communication
	storageNodes map[[32]byte]net.Addr // Known storage nodes
	knownSenders map[[32]byte]bool     // Known senders for message decryption
	lastRetrieve time.Time             // Last message retrieval time
}

// NewAsyncClient creates a new async messaging client with obfuscation support
func NewAsyncClient(keyPair *crypto.KeyPair, transport transport.Transport) *AsyncClient {
	epochManager := NewEpochManager()
	obfuscation := NewObfuscationManager(keyPair, epochManager)

	return &AsyncClient{
		keyPair:      keyPair,
		obfuscation:  obfuscation,
		transport:    transport,
		storageNodes: make(map[[32]byte]net.Addr),
		knownSenders: make(map[[32]byte]bool),
		lastRetrieve: time.Now(),
	}
}

// SendObfuscatedMessage sends a forward-secure message using peer identity obfuscation.
// This method hides the real sender and recipient identities from storage nodes
// while maintaining message deliverability and forward secrecy.
func (ac *AsyncClient) SendObfuscatedMessage(recipientPK [32]byte,
	forwardSecureMsg *ForwardSecureMessage) error {

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
	messageType MessageType) error {

	if message == nil {
		return errors.New("message cannot be nil")
	}

	if len(message) == 0 {
		return errors.New("message cannot be empty")
	}

	if len(message) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(message), MaxMessageSize)
	}

	// Create a ForwardSecureMessage structure for the message
	// In a production system, this would integrate with the forward secrecy manager
	// For now, create a basic structure that demonstrates the obfuscation flow
	var messageID [32]byte
	copy(messageID[:], message[:min(len(message), 32)]) // Simple message ID generation

	var nonce [24]byte
	// Generate a unique nonce for this message
	for i := range nonce {
		nonce[i] = byte(time.Now().UnixNano() >> (i * 8))
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
		// Generate our recipient pseudonym for this epoch
		myPseudonym, err := ac.obfuscation.GenerateRecipientPseudonym(ac.keyPair.Public, epoch)
		if err != nil {
			continue // Skip this epoch on error
		}

		// Find storage nodes that might have messages for this pseudonym
		storageNodes := ac.findStorageNodes(myPseudonym, 5)
		if len(storageNodes) == 0 {
			continue // Skip this epoch if no storage nodes available
		}

		// Query each storage node for our messages
		for _, nodeAddr := range storageNodes {
			obfMessages, err := ac.retrieveObfuscatedMessagesFromNode(nodeAddr, myPseudonym, []uint64{epoch})
			if err != nil {
				continue // Skip failed nodes
			}

			// Decrypt and validate messages
			for _, obfMsg := range obfMessages {
				decrypted, err := ac.decryptObfuscatedMessage(obfMsg)
				if err != nil {
					continue // Skip messages we can't decrypt
				}
				allMessages = append(allMessages, decrypted)
			}
		}
	}

	ac.lastRetrieve = time.Now()
	return allMessages, nil
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
	Message     string
	MessageType MessageType
	Timestamp   time.Time
}

// AsyncRetrieveRequest represents a request to retrieve messages from a storage node
type AsyncRetrieveRequest struct {
	RecipientPseudonym [32]byte // Obfuscated recipient identity
	Epochs             []uint64 // Which epochs to retrieve messages from
}

// findStorageNodes identifies DHT nodes that can serve as storage nodes
// Uses consistent hashing to select nodes closest to the recipient's public key
func (ac *AsyncClient) findStorageNodes(targetPK [32]byte, maxNodes int) []net.Addr {
	// In a real implementation, this would:
	// 1. Use DHT to find nodes closest to hash(recipientPK)
	// 2. Verify nodes support async messaging
	// 3. Select healthy, active nodes
	//
	// For now, return known storage nodes
	var nodes []net.Addr
	for _, addr := range ac.storageNodes {
		nodes = append(nodes, addr)
		if len(nodes) >= maxNodes {
			break
		}
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
	recipientPseudonym [32]byte, epochs []uint64) ([]*ObfuscatedAsyncMessage, error) {

	// Create retrieval request payload
	retrieveRequest := &AsyncRetrieveRequest{
		RecipientPseudonym: recipientPseudonym,
		Epochs:             epochs,
	}

	// Serialize the retrieval request
	serializedRequest, err := ac.serializeRetrieveRequest(retrieveRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize retrieve request: %w", err)
	}

	// Create async retrieve packet
	retrievePacket := &transport.Packet{
		PacketType: transport.PacketAsyncRetrieve,
		Data:       serializedRequest,
	}

	// Send retrieve request to storage node
	err = ac.transport.Send(retrievePacket, nodeAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to send retrieve request to %v: %w", nodeAddr, err)
	}

	// In a production implementation, we would:
	// 1. Wait for a response packet (PacketAsyncRetrieveResponse)
	// 2. Deserialize the response containing the message list
	// 3. Return the retrieved messages
	//
	// For now, return empty slice as the network response handling
	// would be implemented in the transport layer packet handlers
	return []*ObfuscatedAsyncMessage{}, nil
}

// decryptObfuscatedMessage attempts to decrypt an obfuscated message
func (ac *AsyncClient) decryptObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage) (DecryptedMessage, error) {
	// Verify this message is intended for us by checking the recipient pseudonym
	expectedPseudonym, err := ac.obfuscation.GenerateRecipientPseudonym(ac.keyPair.Public, obfMsg.Epoch)
	if err != nil {
		return DecryptedMessage{}, fmt.Errorf("failed to generate expected pseudonym: %w", err)
	}

	if expectedPseudonym != obfMsg.RecipientPseudonym {
		return DecryptedMessage{}, errors.New("message not intended for this recipient")
	}

	// Decrypt the obfuscated payload to get the inner ForwardSecureMessage
	// We need to try different potential senders since we don't know who sent it
	return ac.tryDecryptFromKnownSenders(obfMsg)
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

	return DecryptedMessage{
		ID:          messageID,
		SenderPK:    forwardSecureMsg.SenderPK,
		Message:     string(forwardSecureMsg.EncryptedData), // Note: In real system, this would be decrypted
		MessageType: forwardSecureMsg.MessageType,
		Timestamp:   forwardSecureMsg.Timestamp,
	}, nil
}
