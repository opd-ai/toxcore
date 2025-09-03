package async

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/curve25519"

	"github.com/opd-ai/toxcore/crypto"
)

// AsyncClient handles the client-side operations for async messaging
// with built-in peer identity obfuscation for privacy protection
type AsyncClient struct {
	mutex        sync.RWMutex
	keyPair      *crypto.KeyPair
	obfuscation  *ObfuscationManager   // Handles cryptographic obfuscation
	storageNodes map[[32]byte]net.Addr // Known storage nodes
	lastRetrieve time.Time             // Last message retrieval time
}

// NewAsyncClient creates a new async messaging client with obfuscation support
func NewAsyncClient(keyPair *crypto.KeyPair) *AsyncClient {
	epochManager := NewEpochManager()
	obfuscation := NewObfuscationManager(keyPair, epochManager)

	return &AsyncClient{
		keyPair:      keyPair,
		obfuscation:  obfuscation,
		storageNodes: make(map[[32]byte]net.Addr),
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

// SendAsyncMessage is deprecated - use SendObfuscatedMessage for secure messaging
// This method is kept for backward compatibility but will not provide privacy protection
func (ac *AsyncClient) SendAsyncMessage(recipientPK [32]byte, message []byte,
	messageType MessageType) error {

	// This API is insecure and should not be used for new applications
	// Privacy protection is not provided by this method
	return errors.New("insecure API deprecated: use SendObfuscatedMessage for privacy-protected messaging")
}

// RetrieveAsyncMessages retrieves pending messages for this client
func (ac *AsyncClient) RetrieveAsyncMessages() ([]DecryptedMessage, error) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	// Find storage nodes that might have our messages
	storageNodes := ac.findStorageNodes(ac.keyPair.Public, 5)
	if len(storageNodes) == 0 {
		return nil, errors.New("no storage nodes available")
	}

	var allMessages []DecryptedMessage

	// Query each storage node for our messages
	for _, nodeAddr := range storageNodes {
		messages, err := ac.retrieveMessagesFromNode(nodeAddr, ac.keyPair.Public)
		if err != nil {
			continue // Skip failed nodes
		}

		// Decrypt and validate messages
		for _, encMsg := range messages {
			decrypted, err := ac.decryptMessage(encMsg)
			if err != nil {
				continue // Skip messages we can't decrypt
			}
			allMessages = append(allMessages, decrypted)
		}
	}

	ac.lastRetrieve = time.Now()
	return allMessages, nil
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

// serializeForwardSecureMessage converts a ForwardSecureMessage to bytes
func (ac *AsyncClient) serializeForwardSecureMessage(fsMsg *ForwardSecureMessage) ([]byte, error) {
	// In a real implementation, this would use a proper serialization format
	// like Protocol Buffers, MessagePack, or JSON
	// For now, we'll use a simple format that can be reconstructed

	// This is a placeholder - in production, use proper serialization
	result := fmt.Sprintf("FSM|%s|%x|%x|%x|%d|%x|%x|%d|%s|%s",
		fsMsg.Type, fsMsg.MessageID, fsMsg.SenderPK, fsMsg.RecipientPK,
		fsMsg.PreKeyID, fsMsg.EncryptedData, fsMsg.Nonce,
		int(fsMsg.MessageType), fsMsg.Timestamp.Format(time.RFC3339),
		fsMsg.ExpiresAt.Format(time.RFC3339))

	return []byte(result), nil
}

// deriveSharedSecret computes the shared secret with a recipient using ECDH
func (ac *AsyncClient) deriveSharedSecret(recipientPK [32]byte) ([32]byte, error) {
	// Use curve25519 for ECDH computation
	// This is the same computation that NaCl box uses internally
	var sharedSecret [32]byte
	curve25519.ScalarMult(&sharedSecret, &ac.keyPair.Private, &recipientPK)
	return sharedSecret, nil
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
	// In a real implementation, this would:
	// 1. Establish connection to storage node
	// 2. Send store request with obfuscated message
	// 3. Handle response and confirm storage

	// For demo purposes, simulate successful storage
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

// storeMessageOnNode sends a message to a specific storage node
func (ac *AsyncClient) storeMessageOnNode(nodeAddr net.Addr, recipientPK, senderPK [32]byte,
	encryptedMessage []byte, nonce [24]byte, messageType MessageType) error {

	// In a real implementation, this would:
	// 1. Establish connection to storage node
	// 2. Authenticate with node
	// 3. Send encrypted store request with the encrypted message and nonce
	// 4. Handle response and confirm storage

	// For demo purposes, simulate successful storage
	return nil
}

// storeForwardSecureMessageOnNode sends a forward-secure message to a specific storage node
func (ac *AsyncClient) storeForwardSecureMessageOnNode(nodeAddr net.Addr, fsMsg *ForwardSecureMessage) error {
	// In a real implementation, this would:
	// 1. Establish connection to storage node
	// 2. Authenticate with node
	// 3. Send encrypted store request with the forward-secure message
	// 4. Handle response and confirm storage

	// For demo purposes, simulate successful storage
	return nil
}

// retrieveMessagesFromNode retrieves messages from a specific storage node
func (ac *AsyncClient) retrieveMessagesFromNode(nodeAddr net.Addr,
	recipientPK [32]byte) ([]AsyncMessage, error) {

	// In a real implementation, this would:
	// 1. Connect to storage node
	// 2. Authenticate as the recipient
	// 3. Request pending messages
	// 4. Process and return encrypted messages

	// For demo purposes, return empty slice
	return []AsyncMessage{}, nil
}

// decryptMessage decrypts an async message for the recipient
func (ac *AsyncClient) decryptMessage(encMsg AsyncMessage) (DecryptedMessage, error) {
	// Verify this message is for us
	if encMsg.RecipientPK != ac.keyPair.Public {
		return DecryptedMessage{}, errors.New("message not for us")
	}

	// Decrypt the message using our private key
	decrypted, err := crypto.Decrypt(encMsg.EncryptedData, encMsg.Nonce,
		encMsg.SenderPK, ac.keyPair.Private)
	if err != nil {
		return DecryptedMessage{}, fmt.Errorf("decryption failed: %w", err)
	}

	return DecryptedMessage{
		ID:          encMsg.ID,
		SenderPK:    encMsg.SenderPK,
		Message:     string(decrypted),
		MessageType: encMsg.MessageType,
		Timestamp:   encMsg.Timestamp,
	}, nil
}

// AddStorageNode adds a known storage node to the client
func (ac *AsyncClient) AddStorageNode(publicKey [32]byte, addr net.Addr) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	ac.storageNodes[publicKey] = addr
}

// GetLastRetrieveTime returns when messages were last retrieved
func (ac *AsyncClient) GetLastRetrieveTime() time.Time {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.lastRetrieve
}

// SendForwardSecureAsyncMessage stores a forward-secure message for offline delivery
func (ac *AsyncClient) SendForwardSecureAsyncMessage(fsMsg *ForwardSecureMessage) error {
	if fsMsg == nil {
		return errors.New("nil forward secure message")
	}

	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	// Find suitable storage nodes from DHT
	storageNodes := ac.findStorageNodes(fsMsg.RecipientPK, 3) // Use 3 nodes for redundancy
	if len(storageNodes) == 0 {
		return errors.New("no storage nodes available")
	}

	// Store forward-secure message on multiple nodes for redundancy
	storedCount := 0
	for _, nodeAddr := range storageNodes {
		if err := ac.storeForwardSecureMessageOnNode(nodeAddr, fsMsg); err == nil {
			storedCount++
		}
	}

	if storedCount == 0 {
		return errors.New("failed to store forward-secure message on any storage node")
	}

	return nil
}

// retrieveObfuscatedMessagesFromNode retrieves obfuscated messages from a specific storage node
func (ac *AsyncClient) retrieveObfuscatedMessagesFromNode(nodeAddr net.Addr,
	recipientPseudonym [32]byte, epochs []uint64) ([]*ObfuscatedAsyncMessage, error) {

	// In a real implementation, this would:
	// 1. Connect to storage node
	// 2. Send retrieval request with pseudonym and epochs
	// 3. Process and return obfuscated messages

	// For demo purposes, return empty slice
	return []*ObfuscatedAsyncMessage{}, nil
}

// decryptObfuscatedMessage attempts to decrypt an obfuscated message
func (ac *AsyncClient) decryptObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage) (DecryptedMessage, error) {
	// Try to decrypt with known potential senders
	// In practice, this would iterate through known friends or use key exchange information

	// For now, we'll implement a simple approach that assumes we know the sender
	// This is a limitation of the current demo implementation

	// Generate the expected recipient pseudonym to verify this message is for us
	expectedPseudonym, err := ac.obfuscation.GenerateRecipientPseudonym(ac.keyPair.Public, obfMsg.Epoch)
	if err != nil {
		return DecryptedMessage{}, fmt.Errorf("failed to generate expected pseudonym: %w", err)
	}

	if expectedPseudonym != obfMsg.RecipientPseudonym {
		return DecryptedMessage{}, errors.New("message not intended for this recipient")
	}

	// For a complete implementation, we would need to:
	// 1. Try all known potential senders
	// 2. Derive shared secrets with each
	// 3. Attempt decryption until one succeeds

	// This requires integration with the contact/friend system
	// For now, return an error indicating incomplete implementation
	return DecryptedMessage{}, errors.New("obfuscated message decryption requires sender identification - integration with contact system needed")
}
