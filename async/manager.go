package async

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// pendingMessage represents a message waiting for pre-key exchange
type pendingMessage struct {
	message     string
	messageType MessageType
	timestamp   time.Time
}

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
	friendAddresses map[[32]byte]net.Addr                                            // Track network addresses of friends
	pendingMessages map[[32]byte][]pendingMessage                                    // Messages queued for pre-key exchange
	messageHandler  func(senderPK [32]byte, message string, messageType MessageType) // Callback for received async messages
	notificationHub *NotificationHub                                                 // Push notification system
	messageOrdering *MessageOrdering                                                 // Lamport clock for causal message ordering
	discovery       *StorageNodeDiscovery                                            // DHT-based storage node discovery
	running         bool
	stopChan        chan struct{}
	wg              sync.WaitGroup // Tracks background goroutines for clean shutdown
}

// AsyncManagerConfig provides configuration options for the AsyncManager.
type AsyncManagerConfig struct {
	// MaxMessagesPerRecipient limits the number of messages stored per recipient.
	// When set to 0 or not specified, the default of 100 is used.
	// This value configures the base limit for dynamic per-recipient limits.
	MaxMessagesPerRecipient int
}

// DefaultAsyncManagerConfig returns the default configuration for AsyncManager.
func DefaultAsyncManagerConfig() *AsyncManagerConfig {
	return &AsyncManagerConfig{
		MaxMessagesPerRecipient: MaxMessagesPerRecipient,
	}
}

// initializeWAL performs crash recovery if WAL was auto-enabled by NewMessageStorage.
// WAL is now auto-enabled in NewMessageStorage when dataDir is non-empty.
func (am *AsyncManager) initializeWAL(dataDir string) {
	if dataDir == "" {
		return
	}
	// WAL is auto-enabled by NewMessageStorage; only perform recovery here
	if !am.storage.IsWALEnabled() {
		// WAL not enabled (e.g., failed to initialize in NewMessageStorage)
		return
	}
	recovered, recoverErr := am.storage.RecoverFromWAL()
	if recoverErr != nil {
		logrus.WithFields(logrus.Fields{
			"function": "initializeWAL",
			"error":    recoverErr.Error(),
		}).Warn("WAL recovery encountered errors")
	} else if recovered > 0 {
		logrus.WithFields(logrus.Fields{
			"function":  "initializeWAL",
			"recovered": recovered,
		}).Info("Recovered messages from WAL after restart")
	}
}

// registerPreKeyHandler registers the pre-key exchange packet handler with the transport.
func (am *AsyncManager) registerPreKeyHandler(trans transport.Transport) {
	if trans == nil {
		return
	}
	trans.RegisterHandler(transport.PacketAsyncPreKeyExchange, func(packet *transport.Packet, addr net.Addr) error {
		am.handlePreKeyExchangePacket(packet, addr)
		return nil
	})
}

// NewAsyncManager creates a new async message manager with built-in obfuscation
// All users automatically become storage nodes with capacity based on available disk space
func NewAsyncManager(keyPair *crypto.KeyPair, trans transport.Transport, dataDir string) (*AsyncManager, error) {
	return NewAsyncManagerWithConfig(keyPair, trans, dataDir, nil)
}

// NewAsyncManagerWithConfig creates a new async message manager with custom configuration.
// If config is nil, default configuration is used.
func NewAsyncManagerWithConfig(keyPair *crypto.KeyPair, trans transport.Transport, dataDir string, config *AsyncManagerConfig) (*AsyncManager, error) {
	if config == nil {
		config = DefaultAsyncManagerConfig()
	}

	forwardSecurity, err := NewForwardSecurityManager(keyPair, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create forward security manager: %w", err)
	}

	epochManager := NewEpochManager()
	obfuscation := NewObfuscationManager(keyPair, epochManager)

	discovery := NewStorageNodeDiscovery()

	storage := NewMessageStorage(keyPair, dataDir)
	// Apply configured message limit if non-default
	if config.MaxMessagesPerRecipient > 0 && config.MaxMessagesPerRecipient != MaxMessagesPerRecipient {
		storage.SetMaxMessagesPerRecipient(config.MaxMessagesPerRecipient)
	}

	am := &AsyncManager{
		client:          NewAsyncClient(keyPair, trans),
		storage:         storage,
		forwardSecurity: forwardSecurity,
		obfuscation:     obfuscation,
		keyPair:         keyPair,
		isStorageNode:   true,
		onlineStatus:    make(map[[32]byte]bool),
		friendAddresses: make(map[[32]byte]net.Addr),
		pendingMessages: make(map[[32]byte][]pendingMessage),
		messageOrdering: NewMessageOrdering(),
		discovery:       discovery,
		stopChan:        make(chan struct{}),
	}

	// Wire the ForwardSecurityManager into the client so that AsyncClient.SendAsyncMessage
	// uses one-time pre-key encryption when called directly (e.g., from examples/tests).
	am.client.SetForwardSecurityManager(forwardSecurity)

	// Set up auto-add callback for discovered storage nodes
	discovery.OnNodeDiscovered(func(ann *StorageNodeAnnouncement) {
		am.client.AddStorageNode(ann.PublicKey, ann.ToNetAddr())
		logrus.WithFields(logrus.Fields{
			"function": "StorageNodeDiscovery",
			"address":  ann.Address,
			"port":     ann.Port,
			"load":     ann.Load,
		}).Info("Auto-added discovered storage node")
	})

	am.initializeWAL(dataDir)
	am.registerPreKeyHandler(trans)

	return am, nil
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

	// Start background tasks with WaitGroup tracking
	am.wg.Add(1)
	go func() {
		defer am.wg.Done()
		am.messageRetrievalLoop()
	}()

	if am.isStorageNode {
		am.wg.Add(1)
		go func() {
			defer am.wg.Done()
			am.storageMaintenanceLoop()
		}()
	}

	// Start storage node discovery loop
	am.wg.Add(1)
	go func() {
		defer am.wg.Done()
		am.storageNodeDiscoveryLoop()
	}()
}

// Stop shuts down the async messaging service
func (am *AsyncManager) Stop() {
	am.mutex.Lock()
	if !am.running {
		am.mutex.Unlock()
		return
	}

	am.running = false
	close(am.stopChan)
	am.mutex.Unlock()

	// Stop the randomized retrieval scheduler first (it has its own goroutine)
	am.client.StopScheduledRetrieval()

	// Close the forward security manager to stop its cleanup routine
	if am.forwardSecurity != nil {
		am.forwardSecurity.Close()
	}

	// Wait for all background goroutines to finish
	am.wg.Wait()
}

// SendAsyncMessage attempts to send a message asynchronously using forward secrecy and obfuscation
// If pre-keys are not available, the message is queued and will be sent automatically when the friend comes online
func (am *AsyncManager) SendAsyncMessage(recipientPK [32]byte, message string,
	messageType MessageType,
) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	// Check if recipient is online
	if am.isOnline(recipientPK) {
		return ErrRecipientOnline
	}

	// Check if we can send forward-secure message
	if !am.forwardSecurity.CanSendMessage(recipientPK) {
		// Queue the message for automatic sending when pre-keys become available
		am.queuePendingMessage(recipientPK, message, messageType)
		log.Printf("Queued async message for recipient %x - will send after pre-key exchange", recipientPK[:8])
		return nil
	}

	// Send forward-secure message immediately
	return am.sendForwardSecureMessage(recipientPK, message, messageType)
}

// queuePendingMessage adds a message to the pending queue for a recipient
func (am *AsyncManager) queuePendingMessage(recipientPK [32]byte, message string, messageType MessageType) {
	pending := pendingMessage{
		message:     message,
		messageType: messageType,
		timestamp:   time.Now(),
	}
	am.pendingMessages[recipientPK] = append(am.pendingMessages[recipientPK], pending)
}

// sendForwardSecureMessage sends a message using forward secrecy (internal helper)
func (am *AsyncManager) sendForwardSecureMessage(recipientPK [32]byte, message string, messageType MessageType) error {
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

// SetFriendAddress updates the network address of a friend
// This must be called before sending pre-key exchange packets to the friend
func (am *AsyncManager) SetFriendAddress(friendPK [32]byte, addr net.Addr) {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	am.friendAddresses[friendPK] = addr
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

// AddFriend registers a friend's public key so that incoming obfuscated async
// messages from that friend can be decrypted.  This must be called for every
// friend that the local user has accepted; otherwise RetrieveObfuscatedMessages
// cannot identify the sender and will silently drop their messages.
func (am *AsyncManager) AddFriend(friendPK [32]byte) {
	am.client.AddKnownSender(friendPK)
}

// RemoveFriend unregisters a friend's public key, preventing decryption of any
// further messages attributed to that key.  It also clears any pending async
// messages queued for that friend.
func (am *AsyncManager) RemoveFriend(friendPK [32]byte) {
	am.client.RemoveKnownSender(friendPK)
	am.ClearPendingMessagesForFriend(friendPK)
}

// ResetKnownSenders clears the entire known-sender allowlist.  Call this before
// re-populating the list from a freshly loaded save file so that stale keys from
// previously-removed friends do not persist across reloads.
func (am *AsyncManager) ResetKnownSenders() {
	am.client.mutex.Lock()
	am.client.knownSenders = make(map[[32]byte]bool)
	am.client.mutex.Unlock()
}

// ClearPendingMessagesForFriend removes all queued pending messages for a specific friend.
// This should be called when deleting a friend to clean up orphaned message state.
// Returns the number of messages that were cleared.
func (am *AsyncManager) ClearPendingMessagesForFriend(friendPK [32]byte) int {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	pendingMsgs, exists := am.pendingMessages[friendPK]
	if !exists || len(pendingMsgs) == 0 {
		return 0
	}

	count := len(pendingMsgs)
	delete(am.pendingMessages, friendPK)
	delete(am.onlineStatus, friendPK)
	delete(am.friendAddresses, friendPK)

	return count
}

// GetMessageTimestamp returns a new Lamport timestamp for an outgoing message.
// The timestamp is monotonically increasing and can be used for causal ordering.
func (am *AsyncManager) GetMessageTimestamp() uint64 {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	return am.messageOrdering.GetTimestamp()
}

// ProcessReceivedTimestamp updates the local Lamport clock based on a received message's timestamp.
// This should be called when processing an incoming message to maintain causal ordering.
// Returns the updated local timestamp.
func (am *AsyncManager) ProcessReceivedTimestamp(timestamp uint64) uint64 {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	return am.messageOrdering.ProcessIncoming(timestamp)
}

// GetCurrentClock returns the current Lamport clock value without incrementing.
func (am *AsyncManager) GetCurrentClock() uint64 {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	return am.messageOrdering.CurrentClock()
}

// GetStorageStats returns statistics about the storage node (if acting as one)
func (am *AsyncManager) GetStorageStats() *StorageStats {
	if !am.isStorageNode {
		return nil
	}

	stats := am.storage.GetStorageStats()
	return &stats
}

// GetStorage returns the underlying message storage for advanced configuration.
// Use with caution as direct storage manipulation may affect message integrity.
func (am *AsyncManager) GetStorage() *MessageStorage {
	return am.storage
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

// ProcessPendingDeliveries attempts to send any messages that are queued in the
// pending map but haven't been delivered yet. This should be called periodically
// from the main Tox iteration loop so that messages queued before pre-key
// exchange completes are flushed as soon as keys become available.
func (am *AsyncManager) ProcessPendingDeliveries() {
	am.mutex.Lock()
	// Collect recipients with queued messages that now have pre-keys available.
	var ready [][32]byte
	for pk := range am.pendingMessages {
		if am.forwardSecurity.CanSendMessage(pk) {
			ready = append(ready, pk)
		}
	}
	am.mutex.Unlock()

	for _, pk := range ready {
		am.sendQueuedMessages(pk)
	}
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

// storageNodeDiscoveryLoop periodically discovers and announces storage nodes via DHT.
func (am *AsyncManager) storageNodeDiscoveryLoop() {
	if !am.waitForInitialDelay() {
		return
	}
	am.runDiscoveryLoop()
}

// waitForInitialDelay waits for the initial delay before starting discovery.
// Returns false if stopped during the delay period.
func (am *AsyncManager) waitForInitialDelay() bool {
	initialDelay := time.NewTimer(5 * time.Second)
	select {
	case <-am.stopChan:
		initialDelay.Stop()
		return false
	case <-initialDelay.C:
		am.performStorageNodeDiscovery()
		return true
	}
}

// runDiscoveryLoop runs the periodic storage node discovery loop.
func (am *AsyncManager) runDiscoveryLoop() {
	ticker := time.NewTicker(am.discovery.discoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-am.stopChan:
			return
		case <-ticker.C:
			am.performStorageNodeDiscovery()
		}
	}
}

// performStorageNodeDiscovery executes one round of storage node discovery and announcement.
func (am *AsyncManager) performStorageNodeDiscovery() {
	am.mutex.RLock()
	isStorageNode := am.isStorageNode
	am.mutex.RUnlock()

	// Clean up expired announcements
	removed := am.discovery.CleanExpired()
	if removed > 0 {
		logrus.WithFields(logrus.Fields{
			"function": "performStorageNodeDiscovery",
			"removed":  removed,
		}).Debug("Cleaned up expired storage node announcements")
	}

	// Announce ourselves if we're a storage node
	if isStorageNode {
		am.announceAsStorageNode()
	}

	// Check if we need to discover more nodes
	if am.discovery.NeedsDiscovery() {
		logrus.WithFields(logrus.Fields{
			"function":    "performStorageNodeDiscovery",
			"cachedNodes": am.discovery.Count(),
		}).Debug("Storage node cache low, would initiate DHT discovery")
		// Note: Full DHT discovery requires integration with dht.RoutingTable
		// For now, nodes are discovered via announcements from other nodes
	}
}

// announceAsStorageNode announces this node as a storage node to the network.
func (am *AsyncManager) announceAsStorageNode() {
	am.mutex.RLock()
	storage := am.storage
	am.mutex.RUnlock()

	if storage == nil {
		return
	}

	// Calculate current load based on message count
	load := am.calculateStorageLoad()

	ann := am.discovery.CreateSelfAnnouncement(load)
	if ann == nil {
		return
	}

	logrus.WithFields(logrus.Fields{
		"function": "announceAsStorageNode",
		"load":     load,
		"capacity": ann.Capacity,
	}).Debug("Would announce self as storage node to DHT")
	// Note: Full announcement requires DHT broadcast integration
}

// calculateStorageLoad returns the current storage load as a percentage (0-100).
func (am *AsyncManager) calculateStorageLoad() uint8 {
	am.mutex.RLock()
	storage := am.storage
	am.mutex.RUnlock()

	if storage == nil {
		return 0
	}

	count := storage.TotalMessageCount()
	capacity := storage.GetMaxCapacity()

	if capacity == 0 {
		return 0
	}

	load := (count * 100) / capacity
	if load > 100 {
		load = 100
	}
	return uint8(load)
}

// GetDiscoveredStorageNodes returns the currently cached storage node announcements.
func (am *AsyncManager) GetDiscoveredStorageNodes() []*StorageNodeAnnouncement {
	return am.discovery.GetActiveNodes()
}

// AddDiscoveredStorageNode manually adds a storage node announcement.
// This can be used when nodes are discovered through other means (e.g., DHT responses).
func (am *AsyncManager) AddDiscoveredStorageNode(ann *StorageNodeAnnouncement) {
	if am.discovery.StoreAnnouncement(ann) {
		// New node - callback will auto-add to client
		logrus.WithFields(logrus.Fields{
			"function": "AddDiscoveredStorageNode",
			"address":  ann.Address,
			"port":     ann.Port,
		}).Info("Added discovered storage node")
	}
}

// ConfigureAsStorageNode configures this manager to act as a storage node with DHT announcements.
func (am *AsyncManager) ConfigureAsStorageNode(address string, port uint16, capacity uint32) {
	am.mutex.Lock()
	am.isStorageNode = true
	am.mutex.Unlock()

	am.discovery.SetSelfAsStorageNode(am.keyPair.Public, address, port, capacity)
}

// setupMaintenanceTickers creates and configures all maintenance tickers with appropriate intervals
func (am *AsyncManager) setupMaintenanceTickers() *maintenanceTickers {
	return &maintenanceTickers{
		cleanup:  time.NewTicker(10 * time.Minute), // Cleanup every 10 minutes
		capacity: time.NewTicker(5 * time.Minute),  // Update capacity every 5 minutes
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
		if !am.handleMaintenanceEvent(tickers) {
			return
		}
	}
}

// handleMaintenanceEvent processes a single maintenance event from the available tickers.
// Returns true if the loop should continue, false if it should stop.
func (am *AsyncManager) handleMaintenanceEvent(tickers *maintenanceTickers) bool {
	select {
	case <-am.stopChan:
		return false
	case <-tickers.cleanup.C:
		am.performExpiredMessageCleanup()
	case <-tickers.capacity.C:
		am.performCapacityUpdate()
	case <-tickers.preKey.C:
		am.performPreKeyCleanup()
	}
	return true
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
	if am.forwardSecurity == nil {
		log.Printf("AsyncManager: forwardSecurity not initialized, skipping pre-key exchange for peer %x", friendPK[:8])
		return
	}

	am.initiatePreKeyExchange(friendPK)
	am.sendQueuedMessages(friendPK)
	am.deliverPendingMessagesWithHandler(friendPK, handler)
}

// initiatePreKeyExchange generates and sends pre-keys for a peer if needed.
func (am *AsyncManager) initiatePreKeyExchange(friendPK [32]byte) {
	if !am.forwardSecurity.NeedsKeyExchange(friendPK) {
		return
	}
	if err := am.forwardSecurity.GeneratePreKeysForPeer(friendPK); err != nil {
		log.Printf("Failed to generate pre-keys for peer %x: %v", friendPK[:8], err)
	}
	exchange, err := am.forwardSecurity.ExchangePreKeys(friendPK)
	if err != nil {
		log.Printf("Failed to create pre-key exchange for peer %x: %v", friendPK[:8], err)
		return
	}
	if err := am.sendPreKeyExchange(friendPK, exchange); err != nil {
		log.Printf("Failed to send pre-key exchange for peer %x: %v", friendPK[:8], err)
	} else {
		log.Printf("Pre-key exchange sent for peer %x (%d pre-keys)", friendPK[:8], len(exchange.PreKeys))
	}
}

// sendQueuedMessages sends all messages that were queued for a friend waiting for pre-key exchange
func (am *AsyncManager) sendQueuedMessages(friendPK [32]byte) {
	am.mutex.Lock()
	queued := am.pendingMessages[friendPK]
	delete(am.pendingMessages, friendPK)
	am.mutex.Unlock()

	if len(queued) == 0 {
		return
	}

	// Wait briefly for pre-key exchange to complete
	time.Sleep(500 * time.Millisecond)

	successCount := 0
	for _, pending := range queued {
		am.mutex.Lock()
		err := am.sendForwardSecureMessage(friendPK, pending.message, pending.messageType)
		am.mutex.Unlock()

		if err != nil {
			log.Printf("Failed to send queued message to %x: %v", friendPK[:8], err)
		} else {
			successCount++
		}
	}

	if successCount > 0 {
		log.Printf("Sent %d queued messages to friend %x after pre-key exchange", successCount, friendPK[:8])
	}
}

// createPreKeyExchangePacket creates a serialized pre-key exchange packet with Ed25519 signature
func (am *AsyncManager) createPreKeyExchangePacket(exchange *PreKeyExchangeMessage) ([]byte, error) {
	// Packet format: [MAGIC(4)][VERSION(1)][SENDER_PK(32)][ED25519_PK(32)][KEY_COUNT(2)][KEYS...][SIGNATURE(64)]
	// Ed25519 signature provides cryptographic authentication of the sender
	// Both Curve25519 PK (for encryption) and Ed25519 PK (for verification) are included

	magic := []byte("PKEY") // Pre-key magic bytes
	version := byte(1)
	keyCount := uint16(len(exchange.PreKeys))

	// Derive Ed25519 public key from our private key for signature verification
	ed25519PK := crypto.GetSignaturePublicKey(am.keyPair.Private)

	// Calculate total packet size
	payloadSize := 4 + 1 + 32 + 32 + 2 + (len(exchange.PreKeys) * 32) // magic + version + curve25519_pk + ed25519_pk + count + keys
	packetSize := payloadSize + crypto.SignatureSize                  // Add signature size (64 bytes)
	packet := make([]byte, packetSize)

	offset := 0

	// Write magic
	copy(packet[offset:], magic)
	offset += 4

	// Write version
	packet[offset] = version
	offset += 1

	// Write sender Curve25519 public key
	copy(packet[offset:], am.keyPair.Public[:])
	offset += 32

	// Write sender Ed25519 public key
	copy(packet[offset:], ed25519PK[:])
	offset += 32

	// Write key count
	packet[offset] = byte(keyCount >> 8)
	packet[offset+1] = byte(keyCount & 0xFF)
	offset += 2

	// Write pre-keys
	for _, key := range exchange.PreKeys {
		copy(packet[offset:], key.PublicKey[:])
		offset += 32
	}

	// Sign the payload with Ed25519
	payload := packet[:payloadSize]
	signature, err := crypto.Sign(payload, am.keyPair.Private)
	if err != nil {
		return nil, fmt.Errorf("failed to sign pre-key packet: %w", err)
	}

	// Append signature
	copy(packet[payloadSize:], signature[:])

	return packet, nil
}

// sendPreKeyExchange sends a pre-key exchange packet to a friend over the network
func (am *AsyncManager) sendPreKeyExchange(friendPK [32]byte, exchange *PreKeyExchangeMessage) error {
	// Get friend address
	am.mutex.RLock()
	friendAddr, ok := am.friendAddresses[friendPK]
	am.mutex.RUnlock()

	if !ok {
		return fmt.Errorf("no address known for friend %x", friendPK[:8])
	}

	// Check if client transport is available
	if am.client.transport == nil {
		return fmt.Errorf("transport not available")
	}

	// Create pre-key exchange packet
	packet, err := am.createPreKeyExchangePacket(exchange)
	if err != nil {
		return fmt.Errorf("failed to create pre-key packet: %w", err)
	}

	// Send packet via transport
	transportPacket := &transport.Packet{
		PacketType: transport.PacketAsyncPreKeyExchange,
		Data:       packet,
	}

	if err := am.client.transport.Send(transportPacket, friendAddr); err != nil {
		return fmt.Errorf("failed to send pre-key packet: %w", err)
	}

	return nil
}

// handlePreKeyExchangePacket handles incoming pre-key exchange packets
func (am *AsyncManager) handlePreKeyExchangePacket(packet *transport.Packet, addr net.Addr) {
	// Verify packet has minimum size: magic(4) + version(1) + sender_pk(32) + ed25519_pk(32) + count(2) + 1 key(32) + signature(64)
	minSize := 4 + 1 + 32 + 32 + 2 + 32 + crypto.SignatureSize
	if len(packet.Data) < minSize {
		log.Printf("Received pre-key packet too small: %d bytes", len(packet.Data))
		return
	}

	// Parse and validate the packet (includes signature verification)
	exchange, senderPK, err := am.parsePreKeyExchangePacket(packet.Data)
	if err != nil {
		log.Printf("Failed to parse pre-key exchange packet: %v", err)
		return
	}

	// SECURITY: Only accept pre-key exchanges from known friends
	// This provides an additional layer of defense-in-depth beyond signature verification
	am.mutex.RLock()
	_, isKnownFriend := am.friendAddresses[senderPK]
	am.mutex.RUnlock()

	if !isKnownFriend {
		log.Printf("Rejected pre-key exchange from unknown sender %x (anti-spam protection)", senderPK[:8])
		return
	}

	// Process the pre-key exchange
	if err := am.forwardSecurity.ProcessPreKeyExchange(exchange); err != nil {
		log.Printf("Failed to process pre-key exchange from %x: %v", senderPK[:8], err)
		return
	}

	log.Printf("Successfully processed pre-key exchange from friend %x (%d keys received)", senderPK[:8], len(exchange.PreKeys))
}

// parsePreKeyExchangePacket parses and validates a pre-key exchange packet
func (am *AsyncManager) parsePreKeyExchangePacket(data []byte) (*PreKeyExchangeMessage, [32]byte, error) {
	var zeroPK [32]byte

	if err := validatePreKeyPacketSize(data); err != nil {
		return nil, zeroPK, err
	}

	senderPK, ed25519PK, keyCount, err := extractPreKeyPacketHeaders(data)
	if err != nil {
		return nil, zeroPK, err
	}

	if err := verifyPreKeyPacketSize(data, keyCount); err != nil {
		return nil, zeroPK, err
	}

	if err := verifyPreKeyPacketSignature(data, ed25519PK); err != nil {
		return nil, zeroPK, err
	}

	preKeys := extractPreKeysFromPacket(data, keyCount)
	exchange := &PreKeyExchangeMessage{
		SenderPK:  senderPK,
		PreKeys:   preKeys,
		Timestamp: time.Now(),
	}

	return exchange, senderPK, nil
}

// validatePreKeyPacketSize checks if the packet meets minimum size requirements.
func validatePreKeyPacketSize(data []byte) error {
	minSize := 4 + 1 + 32 + 32 + 2 + 32 + crypto.SignatureSize
	if len(data) < minSize {
		return fmt.Errorf("packet too small: %d bytes", len(data))
	}
	return nil
}

// extractPreKeyPacketHeaders extracts and validates the packet headers.
func extractPreKeyPacketHeaders(data []byte) ([32]byte, [32]byte, uint16, error) {
	var zeroPK [32]byte

	if string(data[0:4]) != "PKEY" {
		return zeroPK, zeroPK, 0, fmt.Errorf("invalid magic bytes")
	}

	if data[4] != 1 {
		return zeroPK, zeroPK, 0, fmt.Errorf("unsupported version: %d", data[4])
	}

	var senderPK, ed25519PK [32]byte
	copy(senderPK[:], data[5:37])
	copy(ed25519PK[:], data[37:69])

	keyCount := uint16(data[69])<<8 | uint16(data[70])
	if keyCount == 0 {
		return zeroPK, zeroPK, 0, fmt.Errorf("zero key count")
	}

	return senderPK, ed25519PK, keyCount, nil
}

// verifyPreKeyPacketSize ensures the packet size matches the expected size based on key count.
func verifyPreKeyPacketSize(data []byte, keyCount uint16) error {
	expectedSize := 4 + 1 + 32 + 32 + 2 + (int(keyCount) * 32) + crypto.SignatureSize
	if len(data) != expectedSize {
		return fmt.Errorf("invalid packet size: expected %d, got %d", expectedSize, len(data))
	}
	return nil
}

// verifyPreKeyPacketSignature verifies the Ed25519 signature for authentication.
func verifyPreKeyPacketSignature(data []byte, ed25519PK [32]byte) error {
	payloadSize := len(data) - crypto.SignatureSize
	payload := data[:payloadSize]

	var receivedSignature crypto.Signature
	copy(receivedSignature[:], data[payloadSize:])

	valid, err := crypto.Verify(payload, receivedSignature, ed25519PK)
	if err != nil {
		return fmt.Errorf("signature verification error: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid signature - authentication failed")
	}
	return nil
}

// extractPreKeysFromPacket extracts all pre-keys from the validated packet.
func extractPreKeysFromPacket(data []byte, keyCount uint16) []PreKeyForExchange {
	preKeys := make([]PreKeyForExchange, keyCount)
	offset := 71

	for i := uint16(0); i < keyCount; i++ {
		var pubKey [32]byte
		copy(pubKey[:], data[offset:offset+32])

		preKeys[i] = PreKeyForExchange{
			ID:        uint32(i),
			PublicKey: pubKey,
		}
		offset += 32
	}

	return preKeys
}
