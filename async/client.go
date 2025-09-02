package async

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// AsyncClient handles the client-side operations for async messaging
type AsyncClient struct {
	mutex        sync.RWMutex
	keyPair      *crypto.KeyPair
	storageNodes map[[32]byte]net.Addr // Known storage nodes
	lastRetrieve time.Time             // Last message retrieval time
}

// NewAsyncClient creates a new async messaging client
func NewAsyncClient(keyPair *crypto.KeyPair) *AsyncClient {
	return &AsyncClient{
		keyPair:      keyPair,
		storageNodes: make(map[[32]byte]net.Addr),
		lastRetrieve: time.Now(),
	}
}

// SendAsyncMessage stores a message for offline delivery
// Encrypts the message and selects appropriate storage nodes from the DHT
func (ac *AsyncClient) SendAsyncMessage(recipientPK [32]byte, message []byte,
	messageType MessageType) error {

	if len(message) == 0 {
		return errors.New("empty message")
	}

	if len(message) > MaxMessageSize {
		return fmt.Errorf("message too long: %d bytes (max %d)",
			len(message), MaxMessageSize)
	}

	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	// Encrypt message for recipient using sender's private key
	encryptedData, nonce, err := EncryptForRecipient(message, recipientPK, ac.keyPair.Private)
	if err != nil {
		return fmt.Errorf("failed to encrypt message: %w", err)
	}

	// Find suitable storage nodes from DHT
	storageNodes := ac.findStorageNodes(recipientPK, 3) // Use 3 nodes for redundancy
	if len(storageNodes) == 0 {
		return errors.New("no storage nodes available")
	}

	// Store message on multiple nodes for redundancy
	storedCount := 0
	for _, nodeAddr := range storageNodes {
		if err := ac.storeMessageOnNode(nodeAddr, recipientPK,
			ac.keyPair.Public, encryptedData, nonce, messageType); err == nil {
			storedCount++
		}
	}

	if storedCount == 0 {
		return errors.New("failed to store message on any storage node")
	}

	return nil
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
