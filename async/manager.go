package async

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// AsyncManager integrates async messaging with the main Tox system
// It automatically stores messages for offline friends and retrieves messages on startup
type AsyncManager struct {
	mutex          sync.RWMutex
	client         *AsyncClient
	storage        *MessageStorage
	keyPair        *crypto.KeyPair
	isStorageNode  bool                                                             // Whether we act as a storage node
	onlineStatus   map[[32]byte]bool                                                // Track online status of friends
	messageHandler func(senderPK [32]byte, message string, messageType MessageType) // Callback for received async messages
	running        bool
	stopChan       chan struct{}
}

// NewAsyncManager creates a new async message manager
// All users automatically become storage nodes with capacity based on available disk space
func NewAsyncManager(keyPair *crypto.KeyPair, dataDir string) *AsyncManager {
	return &AsyncManager{
		client:        NewAsyncClient(keyPair),
		storage:       NewMessageStorage(keyPair, dataDir),
		keyPair:       keyPair,
		isStorageNode: true, // All users are storage nodes now
		onlineStatus:  make(map[[32]byte]bool),
		stopChan:      make(chan struct{}),
	}
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

// SendAsyncMessage attempts to send a message asynchronously if the recipient is offline
func (am *AsyncManager) SendAsyncMessage(recipientPK [32]byte, message string,
	messageType MessageType) error {

	am.mutex.RLock()
	defer am.mutex.RUnlock()

	// Check if recipient is online
	if am.isOnline(recipientPK) {
		return fmt.Errorf("recipient is online, use regular messaging")
	}

	// Store message for offline delivery
	return am.client.SendAsyncMessage(recipientPK, []byte(message), messageType)
}

// SetFriendOnlineStatus updates the online status of a friend
func (am *AsyncManager) SetFriendOnlineStatus(friendPK [32]byte, online bool) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	wasOffline := !am.onlineStatus[friendPK]
	am.onlineStatus[friendPK] = online

	// If friend just came online, trigger message retrieval for them
	if wasOffline && online {
		go am.deliverPendingMessages(friendPK)
	}
}

// SetAsyncMessageHandler sets the callback for received async messages
func (am *AsyncManager) SetAsyncMessageHandler(handler func(senderPK [32]byte,
	message string, messageType MessageType)) {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	am.messageHandler = handler
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
	defer cleanupTicker.Stop()
	defer capacityTicker.Stop()

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
		}
	}
}

// retrievePendingMessages retrieves and processes pending async messages
func (am *AsyncManager) retrievePendingMessages() {
	messages, err := am.client.RetrieveAsyncMessages()
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
		log.Printf("Async messaging: retrieved %d pending messages", len(messages))
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
