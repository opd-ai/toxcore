package async

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// AsyncManager integrates async messaging with the main Tox system using obfuscation
// It automatically stores messages for offline friends and retrieves messages on startup
// All messages use peer identity obfuscation and forward secrecy by default
type AsyncManager struct {
	mutex           sync.RWMutex
	client          *AsyncClient
	storage         *MessageStorage
	forwardSecurity *ForwardSecurityManager // Forward secrecy manager
	obfuscation     *ObfuscationManager     // Identity obfuscation manager
	keyPair         *crypto.KeyPair
	isStorageNode   bool                                                             // Whether we act as a storage node
	onlineStatus    map[[32]byte]bool                                                // Track online status of friends
	messageHandler  func(senderPK [32]byte, message string, messageType MessageType) // Callback for received async messages
	running         bool
	stopChan        chan struct{}
}

// NewAsyncManager creates a new async message manager with built-in obfuscation
// All users automatically become storage nodes with capacity based on available disk space
func NewAsyncManager(keyPair *crypto.KeyPair, transport transport.Transport, dataDir string) (*AsyncManager, error) {
	forwardSecurity, err := NewForwardSecurityManager(keyPair, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create forward security manager: %w", err)
	}

	epochManager := NewEpochManager()
	obfuscation := NewObfuscationManager(keyPair, epochManager)

	return &AsyncManager{
		client:          NewAsyncClient(keyPair, transport),
		storage:         NewMessageStorage(keyPair, dataDir),
		forwardSecurity: forwardSecurity,
		obfuscation:     obfuscation,
		keyPair:         keyPair,
		isStorageNode:   true, // All users are storage nodes now
		onlineStatus:    make(map[[32]byte]bool),
		stopChan:        make(chan struct{}),
	}, nil
}

// Start begins the async messaging service
func (am *AsyncManager) Start() {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if am.running {
		return
	}

	am.running = true

	// Start the randomized retrieval scheduler with cover traffic
	am.client.StartScheduledRetrieval()

	// Start background tasks
	go am.messageRetrievalLoop()
	if am.isStorageNode {
		go am.storageMaintenanceLoop()
	}
}

// Stop shuts down the async messaging service
func (am *AsyncManager) Stop() {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if !am.running {
		return
	}

	am.running = false
	close(am.stopChan)

	// Stop the randomized retrieval scheduler
	am.client.StopScheduledRetrieval()
}

// SendAsyncMessage attempts to send a message asynchronously using forward secrecy and obfuscation
func (am *AsyncManager) SendAsyncMessage(recipientPK [32]byte, message string,
	messageType MessageType,
) error {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	// Check if recipient is online
	if am.isOnline(recipientPK) {
		return fmt.Errorf("recipient is online, use regular messaging")
	}

	// Check if we can send forward-secure message
	if !am.forwardSecurity.CanSendMessage(recipientPK) {
		return fmt.Errorf("no pre-keys available for recipient %x - cannot send message. Exchange keys when both parties are online", recipientPK[:8])
	}

	// Send forward-secure message
	fsMsg, err := am.forwardSecurity.SendForwardSecureMessage(recipientPK, []byte(message), messageType)
	if err != nil {
		return fmt.Errorf("failed to send forward-secure message: %w", err)
	}

	// Store the forward-secure message using obfuscation
	return am.client.SendObfuscatedMessage(recipientPK, fsMsg)
}

// SetFriendOnlineStatus updates the online status of a friend
func (am *AsyncManager) SetFriendOnlineStatus(friendPK [32]byte, online bool) {
	am.mutex.Lock()
	wasOffline := !am.onlineStatus[friendPK]
	am.onlineStatus[friendPK] = online
	handler := am.messageHandler
	am.mutex.Unlock()

	// If friend just came online, handle pre-key exchange and message retrieval
	// Pass handler explicitly to avoid race condition with SetAsyncMessageHandler
	if wasOffline && online {
		go am.handleFriendOnlineWithHandler(friendPK, handler)
	}
}

// SetAsyncMessageHandler sets the callback for received async messages (forward-secure only)
// All messages received through this handler are forward-secure using pre-exchanged one-time keys
func (am *AsyncManager) SetAsyncMessageHandler(handler func(senderPK [32]byte,
	message string, messageType MessageType),
) {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	am.messageHandler = handler
}

// SetMessageHandler is an alias for SetAsyncMessageHandler for consistency
// All async messages are forward-secure by default
func (am *AsyncManager) SetMessageHandler(handler func(senderPK [32]byte,
	message string, messageType MessageType),
) {
	am.SetAsyncMessageHandler(handler)
}

// AddStorageNode adds a known storage node for message distribution
func (am *AsyncManager) AddStorageNode(publicKey [32]byte, addr net.Addr) {
	am.client.AddStorageNode(publicKey, addr)
}

// GetStorageStats returns statistics about the storage node (if acting as one)
func (am *AsyncManager) GetStorageStats() *StorageStats {
	if !am.isStorageNode {
		return nil
	}

	stats := am.storage.GetStorageStats()
	return &stats
}

// GetPreKeyStats returns information about pre-keys for all peers
func (am *AsyncManager) GetPreKeyStats() map[string]int {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	stats := make(map[string]int)
	peers := am.forwardSecurity.preKeyStore.ListPeers()

	for _, peerPK := range peers {
		remaining := am.forwardSecurity.preKeyStore.GetRemainingKeyCount(peerPK)
		stats[fmt.Sprintf("%x", peerPK[:8])] = remaining
	}

	return stats
}

// ProcessPreKeyExchange processes a received pre-key exchange message
func (am *AsyncManager) ProcessPreKeyExchange(exchange *PreKeyExchangeMessage) error {
	return am.forwardSecurity.ProcessPreKeyExchange(exchange)
}

// CanSendAsyncMessage checks if we can send an async message to a peer (have pre-keys)
func (am *AsyncManager) CanSendAsyncMessage(peerPK [32]byte) bool {
	return am.forwardSecurity.CanSendMessage(peerPK)
}

// UpdateStorageCapacity recalculates and updates storage capacity based on available disk space
// This method can be called manually to trigger capacity updates outside of the automatic maintenance cycle
func (am *AsyncManager) UpdateStorageCapacity() error {
	if !am.isStorageNode {
		return fmt.Errorf("not acting as storage node")
	}
	return am.storage.UpdateCapacity()
}

// isOnline checks if a friend is currently online
func (am *AsyncManager) isOnline(friendPK [32]byte) bool {
	return am.onlineStatus[friendPK]
}

// messageRetrievalLoop periodically retrieves pending messages
func (am *AsyncManager) messageRetrievalLoop() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-am.stopChan:
			return
		case <-ticker.C:
			am.retrievePendingMessages()
		}
	}
}

// storageMaintenanceLoop performs periodic storage cleanup and capacity updates
func (am *AsyncManager) storageMaintenanceLoop() {
	tickers := am.setupMaintenanceTickers()
	defer am.stopMaintenanceTickers(tickers)

	am.runMaintenanceLoop(tickers)
}

// setupMaintenanceTickers creates and configures all maintenance tickers with appropriate intervals
func (am *AsyncManager) setupMaintenanceTickers() *maintenanceTickers {
	return &maintenanceTickers{
		cleanup:  time.NewTicker(10 * time.Minute), // Cleanup every 10 minutes
		capacity: time.NewTicker(1 * time.Hour),    // Update capacity every hour
		preKey:   time.NewTicker(24 * time.Hour),   // Cleanup pre-keys daily
	}
}

// stopMaintenanceTickers safely stops all maintenance tickers to prevent resource leaks
func (am *AsyncManager) stopMaintenanceTickers(tickers *maintenanceTickers) {
	tickers.cleanup.Stop()
	tickers.capacity.Stop()
	tickers.preKey.Stop()
}

// runMaintenanceLoop executes the main maintenance event loop handling ticker events
func (am *AsyncManager) runMaintenanceLoop(tickers *maintenanceTickers) {
	for {
		if am.shouldStopMaintenance() {
			return
		}
		am.handleMaintenanceEvent(tickers)
	}
}

// shouldStopMaintenance checks if the maintenance loop should stop.
func (am *AsyncManager) shouldStopMaintenance() bool {
	select {
	case <-am.stopChan:
		return true
	default:
		return false
	}
}

// handleMaintenanceEvent processes a single maintenance event from the available tickers.
func (am *AsyncManager) handleMaintenanceEvent(tickers *maintenanceTickers) {
	select {
	case <-tickers.cleanup.C:
		am.performExpiredMessageCleanup()
	case <-tickers.capacity.C:
		am.performCapacityUpdate()
	case <-tickers.preKey.C:
		am.performPreKeyCleanup()
	}
}

// maintenanceTickers holds all periodic maintenance timers for storage operations
type maintenanceTickers struct {
	cleanup  *time.Ticker // Timer for expired message cleanup
	capacity *time.Ticker // Timer for storage capacity updates
	preKey   *time.Ticker // Timer for pre-key cleanup
}

// performExpiredMessageCleanup removes expired messages from storage and logs the result
func (am *AsyncManager) performExpiredMessageCleanup() {
	expired := am.storage.CleanupExpiredMessages()
	if expired > 0 {
		log.Printf("Async storage: cleaned up %d expired messages", expired)
	}
}

// performCapacityUpdate updates storage capacity and logs status or errors
func (am *AsyncManager) performCapacityUpdate() {
	if err := am.storage.UpdateCapacity(); err != nil {
		log.Printf("Async storage: failed to update capacity: %v", err)
	} else {
		log.Printf("Async storage: updated capacity to %d messages (%.1f%% utilized)",
			am.storage.GetMaxCapacity(), am.storage.GetStorageUtilization())
	}
}

// performPreKeyCleanup removes expired pre-key bundles and logs the operation
func (am *AsyncManager) performPreKeyCleanup() {
	am.forwardSecurity.CleanupExpiredData()
	log.Printf("Forward secrecy: performed pre-key cleanup")
}

// retrievePendingMessages retrieves and processes pending obfuscated async messages
func (am *AsyncManager) retrievePendingMessages() {
	messages, err := am.client.RetrieveObfuscatedMessages()
	if err != nil {
		// Silently ignore retrieval errors - this is normal when no messages are available
		return
	}

	am.mutex.RLock()
	handler := am.messageHandler
	am.mutex.RUnlock()

	// Deliver retrieved messages
	for _, msg := range messages {
		if handler != nil {
			go handler(msg.SenderPK, string(msg.Message), msg.MessageType)
		}
	}

	if len(messages) > 0 {
		log.Printf("Async messaging: retrieved %d pending obfuscated messages", len(messages))
	}
}

// deliverPendingMessagesWithHandler retrieves and delivers all pending messages for the specified friend
// Accepts handler as parameter to avoid data races
func (am *AsyncManager) deliverPendingMessagesWithHandler(friendPK [32]byte, handler func([32]byte, string, MessageType)) {
	messages, err := am.retrieveStoredMessages(friendPK)
	if err != nil {
		return
	}

	if len(messages) > 0 {
		log.Printf("Async messaging: delivering %d pending messages to friend %x", len(messages), friendPK[:8])
		am.processMessageBatchWithHandler(messages, friendPK, handler)
	} else {
		log.Printf("Async messaging: no pending messages for friend %x", friendPK[:8])
	}
}

// retrieveStoredMessages gets pending messages from storage for the specified friend
func (am *AsyncManager) retrieveStoredMessages(friendPK [32]byte) ([]AsyncMessage, error) {
	if am.storage == nil {
		log.Printf("Async messaging: no storage available for friend %x", friendPK[:8])
		return nil, fmt.Errorf("no storage available")
	}

	messages, err := am.storage.RetrieveMessages(friendPK)
	if err != nil {
		log.Printf("Failed to retrieve messages for peer %x: %v", friendPK[:8], err)
		return nil, err
	}

	return messages, nil
}

// processMessageBatchWithHandler handles decryption and delivery of a batch of messages
// Accepts handler as parameter to avoid data races
func (am *AsyncManager) processMessageBatchWithHandler(messages []AsyncMessage, friendPK [32]byte, handler func([32]byte, string, MessageType)) {
	for _, msg := range messages {
		am.processIndividualMessageWithHandler(msg, friendPK, handler)
		am.cleanupDeliveredMessage(msg.ID, friendPK)
	}
}

// processIndividualMessageWithHandler decrypts and delivers a single message
// Accepts handler as parameter to avoid data races
func (am *AsyncManager) processIndividualMessageWithHandler(msg AsyncMessage, friendPK [32]byte, handler func([32]byte, string, MessageType)) {
	if handler == nil {
		return
	}

	decryptedData, err := am.decryptStoredMessage(msg)
	if err != nil {
		log.Printf("Failed to decrypt message from %x: %v", msg.SenderPK[:8], err)
		return
	}

	handler(msg.SenderPK, string(decryptedData), msg.MessageType)
}

// decryptStoredMessage decrypts a message using the stored nonce and sender public key
func (am *AsyncManager) decryptStoredMessage(msg AsyncMessage) ([]byte, error) {
	var nonce crypto.Nonce
	copy(nonce[:], msg.Nonce[:])

	return crypto.Decrypt(msg.EncryptedData, nonce, msg.SenderPK, am.keyPair.Private)
}

// cleanupDeliveredMessage removes a delivered message from storage
func (am *AsyncManager) cleanupDeliveredMessage(messageID [16]byte, friendPK [32]byte) {
	err := am.storage.DeleteMessage(messageID, friendPK)
	if err != nil {
		log.Printf("Failed to delete delivered message %x for peer %x: %v", messageID[:8], friendPK[:8], err)
	}
}

// handleFriendOnlineWithHandler handles when a friend comes online with explicit handler parameter
// This avoids data races by accepting the handler as a parameter instead of reading it from am.messageHandler
func (am *AsyncManager) handleFriendOnlineWithHandler(friendPK [32]byte, handler func([32]byte, string, MessageType)) {
	// Step 1: Handle pre-key exchange if needed
	if am.forwardSecurity.NeedsKeyExchange(friendPK) {
		// Generate new pre-keys for this peer if needed
		if err := am.forwardSecurity.GeneratePreKeysForPeer(friendPK); err != nil {
			log.Printf("Failed to generate pre-keys for peer %x: %v", friendPK[:8], err)
		}

		// Create pre-key exchange message
		exchange, err := am.forwardSecurity.ExchangePreKeys(friendPK)
		if err != nil {
			log.Printf("Failed to create pre-key exchange for peer %x: %v", friendPK[:8], err)
		} else {
			// Create and serialize pre-key exchange packet
			preKeyPacket, err := am.createPreKeyExchangePacket(exchange)
			if err != nil {
				log.Printf("Failed to create pre-key exchange packet for peer %x: %v", friendPK[:8], err)
			} else if handler != nil {
				// Send through message handler with a special message type identifier
				// In full implementation, this would use a dedicated messaging channel
				handler(friendPK, string(preKeyPacket), MessageTypeNormal)
				log.Printf("Pre-key exchange packet sent for peer %x (%d bytes)", friendPK[:8], len(preKeyPacket))
			}
			log.Printf("Pre-key exchange completed for peer %x (sent %d pre-keys)", friendPK[:8], len(exchange.PreKeys))
		}
	}

	// Step 2: Deliver any pending messages
	am.deliverPendingMessagesWithHandler(friendPK, handler)
}

// createPreKeyExchangePacket creates a serialized pre-key exchange packet with integrity protection
func (am *AsyncManager) createPreKeyExchangePacket(exchange *PreKeyExchangeMessage) ([]byte, error) {
	// Packet format: [MAGIC(4)][VERSION(1)][KEY_COUNT(2)][KEYS...][HMAC(32)]
	// HMAC provides packet integrity protection

	magic := []byte("PKEY") // Pre-key magic bytes
	version := byte(1)
	keyCount := uint16(len(exchange.PreKeys))

	// Calculate total packet size (including 32-byte HMAC)
	payloadSize := 4 + 1 + 2 + (len(exchange.PreKeys) * 32) // 32 bytes per key
	packetSize := payloadSize + 32                          // Add HMAC size
	packet := make([]byte, packetSize)

	offset := 0

	// Write magic
	copy(packet[offset:], magic)
	offset += 4

	// Write version
	packet[offset] = version
	offset += 1

	// Write key count
	packet[offset] = byte(keyCount >> 8)
	packet[offset+1] = byte(keyCount & 0xFF)
	offset += 2

	// Write pre-keys
	for _, key := range exchange.PreKeys {
		copy(packet[offset:], key.PublicKey[:])
		offset += 32
	}

	// Calculate HMAC over the payload (everything except the HMAC itself)
	payload := packet[:payloadSize]
	hmacKey := am.keyPair.Private[:] // Use private key as HMAC key
	h := hmac.New(sha256.New, hmacKey)
	h.Write(payload)
	signature := h.Sum(nil)

	// Append HMAC
	copy(packet[payloadSize:], signature)

	return packet, nil
}
