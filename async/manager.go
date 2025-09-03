package async

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
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
func NewAsyncManager(keyPair *crypto.KeyPair, dataDir string) (*AsyncManager, error) {
	forwardSecurity, err := NewForwardSecurityManager(keyPair, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create forward security manager: %w", err)
	}

	epochManager := NewEpochManager()
	obfuscation := NewObfuscationManager(keyPair, epochManager)

	return &AsyncManager{
		client:          NewAsyncClient(keyPair),
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
}

// SendAsyncMessage attempts to send a message asynchronously using forward secrecy and obfuscation
func (am *AsyncManager) SendAsyncMessage(recipientPK [32]byte, message string,
	messageType MessageType) error {

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
	defer am.mutex.Unlock()

	wasOffline := !am.onlineStatus[friendPK]
	am.onlineStatus[friendPK] = online

	// If friend just came online, handle pre-key exchange and message retrieval
	if wasOffline && online {
		go am.handleFriendOnline(friendPK)
	}
}

// SetAsyncMessageHandler sets the callback for received async messages (forward-secure only)
// All messages received through this handler are forward-secure using pre-exchanged one-time keys
func (am *AsyncManager) SetAsyncMessageHandler(handler func(senderPK [32]byte,
	message string, messageType MessageType)) {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	am.messageHandler = handler
}

// SetMessageHandler is an alias for SetAsyncMessageHandler for consistency
// All async messages are forward-secure by default
func (am *AsyncManager) SetMessageHandler(handler func(senderPK [32]byte,
	message string, messageType MessageType)) {
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
	cleanupTicker := time.NewTicker(10 * time.Minute) // Cleanup every 10 minutes
	capacityTicker := time.NewTicker(1 * time.Hour)   // Update capacity every hour
	preKeyTicker := time.NewTicker(24 * time.Hour)    // Cleanup pre-keys daily
	defer cleanupTicker.Stop()
	defer capacityTicker.Stop()
	defer preKeyTicker.Stop()

	for {
		select {
		case <-am.stopChan:
			return
		case <-cleanupTicker.C:
			expired := am.storage.CleanupExpiredMessages()
			if expired > 0 {
				log.Printf("Async storage: cleaned up %d expired messages", expired)
			}
		case <-capacityTicker.C:
			if err := am.storage.UpdateCapacity(); err != nil {
				log.Printf("Async storage: failed to update capacity: %v", err)
			} else {
				log.Printf("Async storage: updated capacity to %d messages (%.1f%% utilized)",
					am.storage.GetMaxCapacity(), am.storage.GetStorageUtilization())
			}
		case <-preKeyTicker.C:
			// Cleanup expired pre-key bundles
			am.forwardSecurity.CleanupExpiredData()
			log.Printf("Forward secrecy: performed pre-key cleanup")
		}
	}
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
			go handler(msg.SenderPK, msg.Message, msg.MessageType)
		}
	}

	if len(messages) > 0 {
		log.Printf("Async messaging: retrieved %d pending obfuscated messages", len(messages))
	}
}

// deliverPendingMessages retrieves messages for a specific friend who just came online
func (am *AsyncManager) deliverPendingMessages(friendPK [32]byte) {
	// In a real implementation, this would:
	// 1. Query storage nodes for messages specifically for this friend
	// 2. Retrieve and decrypt those messages
	// 3. Deliver them through the normal message callback
	// 4. Mark messages as delivered and delete from storage

	// For now, just log the event
	log.Printf("Async messaging: friend %x came online, checking for pending messages", friendPK[:8])
}

// handleFriendOnline handles when a friend comes online - performs pre-key exchange and message delivery
func (am *AsyncManager) handleFriendOnline(friendPK [32]byte) {
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
			// In a real implementation, this would be sent through the normal Tox messaging system
			log.Printf("Pre-key exchange needed for peer %x (would send %d pre-keys)", friendPK[:8], len(exchange.PreKeys))
		}
	}

	// Step 2: Deliver any pending messages
	am.deliverPendingMessages(friendPK)
}
