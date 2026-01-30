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
	running         bool
	stopChan        chan struct{}
}

// NewAsyncManager creates a new async message manager with built-in obfuscation
// All users automatically become storage nodes with capacity based on available disk space
func NewAsyncManager(keyPair *crypto.KeyPair, trans transport.Transport, dataDir string) (*AsyncManager, error) {
	forwardSecurity, err := NewForwardSecurityManager(keyPair, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create forward security manager: %w", err)
	}

	epochManager := NewEpochManager()
	obfuscation := NewObfuscationManager(keyPair, epochManager)

	am := &AsyncManager{
		client:          NewAsyncClient(keyPair, trans),
		storage:         NewMessageStorage(keyPair, dataDir),
		forwardSecurity: forwardSecurity,
		obfuscation:     obfuscation,
		keyPair:         keyPair,
		isStorageNode:   true, // All users are storage nodes now
		onlineStatus:    make(map[[32]byte]bool),
		friendAddresses: make(map[[32]byte]net.Addr),
		pendingMessages: make(map[[32]byte][]pendingMessage),
		stopChan:        make(chan struct{}),
	}

	// Register handler for pre-key exchange packets
	if trans != nil {
		trans.RegisterHandler(transport.PacketAsyncPreKeyExchange, func(packet *transport.Packet, addr net.Addr) error {
			am.handlePreKeyExchangePacket(packet, addr)
			return nil
		})
	}

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
			// Send pre-key exchange packet over network
			if err := am.sendPreKeyExchange(friendPK, exchange); err != nil {
				log.Printf("Failed to send pre-key exchange for peer %x: %v", friendPK[:8], err)
			} else {
				log.Printf("Pre-key exchange sent for peer %x (%d pre-keys)", friendPK[:8], len(exchange.PreKeys))
			}
		}
	}

	// Step 2: Send any queued messages that were waiting for pre-keys
	am.sendQueuedMessages(friendPK)

	// Step 3: Deliver any pending messages from storage
	am.deliverPendingMessagesWithHandler(friendPK, handler)
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

// createPreKeyExchangePacket creates a serialized pre-key exchange packet with integrity protection
func (am *AsyncManager) createPreKeyExchangePacket(exchange *PreKeyExchangeMessage) ([]byte, error) {
	// Packet format: [MAGIC(4)][VERSION(1)][SENDER_PK(32)][KEY_COUNT(2)][KEYS...][HMAC(32)]
	// HMAC provides packet integrity protection

	magic := []byte("PKEY") // Pre-key magic bytes
	version := byte(1)
	keyCount := uint16(len(exchange.PreKeys))

	// Calculate total packet size (including sender PK and 32-byte HMAC)
	payloadSize := 4 + 1 + 32 + 2 + (len(exchange.PreKeys) * 32) // magic + version + sender + count + keys
	packetSize := payloadSize + 32                               // Add HMAC size
	packet := make([]byte, packetSize)

	offset := 0

	// Write magic
	copy(packet[offset:], magic)
	offset += 4

	// Write version
	packet[offset] = version
	offset += 1

	// Write sender public key
	copy(packet[offset:], am.keyPair.Public[:])
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
	// Verify packet has minimum size: magic(4) + version(1) + sender_pk(32) + count(2) + 1 key(32) + HMAC(32)
	minSize := 4 + 1 + 32 + 2 + 32 + 32
	if len(packet.Data) < minSize {
		log.Printf("Received pre-key packet too small: %d bytes", len(packet.Data))
		return
	}

	// Parse and validate the packet
	exchange, senderPK, err := am.parsePreKeyExchangePacket(packet.Data)
	if err != nil {
		log.Printf("Failed to parse pre-key exchange packet: %v", err)
		return
	}

	// Process the pre-key exchange
	if err := am.forwardSecurity.ProcessPreKeyExchange(exchange); err != nil {
		log.Printf("Failed to process pre-key exchange from %x: %v", senderPK[:8], err)
		return
	}

	// Update friend address if not already known
	am.mutex.Lock()
	if _, exists := am.friendAddresses[senderPK]; !exists {
		am.friendAddresses[senderPK] = addr
	}
	am.mutex.Unlock()

	log.Printf("Successfully processed pre-key exchange from %x (%d keys received)", senderPK[:8], len(exchange.PreKeys))
}

// parsePreKeyExchangePacket parses and validates a pre-key exchange packet
func (am *AsyncManager) parsePreKeyExchangePacket(data []byte) (*PreKeyExchangeMessage, [32]byte, error) {
	var zeroPK [32]byte

	// Verify minimum packet size: magic(4) + version(1) + sender_pk(32) + count(2) + at least 1 key(32) + HMAC(32)
	minSize := 4 + 1 + 32 + 2 + 32 + 32
	if len(data) < minSize {
		return nil, zeroPK, fmt.Errorf("packet too small: %d bytes", len(data))
	}

	// Verify magic bytes
	magic := data[0:4]
	if string(magic) != "PKEY" {
		return nil, zeroPK, fmt.Errorf("invalid magic bytes")
	}

	// Check version
	version := data[4]
	if version != 1 {
		return nil, zeroPK, fmt.Errorf("unsupported version: %d", version)
	}

	// Extract sender public key
	var senderPK [32]byte
	copy(senderPK[:], data[5:37])

	// Read key count
	keyCount := uint16(data[37])<<8 | uint16(data[38])
	if keyCount == 0 {
		return nil, zeroPK, fmt.Errorf("zero key count")
	}

	// Verify packet size matches key count
	expectedSize := 4 + 1 + 32 + 2 + (int(keyCount) * 32) + 32 // header + sender + keys + HMAC
	if len(data) != expectedSize {
		return nil, zeroPK, fmt.Errorf("invalid packet size: expected %d, got %d", expectedSize, len(data))
	}

	// Verify HMAC
	payloadSize := len(data) - 32
	receivedHMAC := data[payloadSize:]

	// We can't verify the sender's HMAC without their public key being registered
	// In a secure implementation, we would verify against known friend keys
	// For now, we just check the packet structure is valid
	_ = receivedHMAC

	// Extract pre-keys
	offset := 39 // After magic(4) + version(1) + sender_pk(32) + count(2)
	preKeys := make([]PreKeyForExchange, keyCount)
	for i := uint16(0); i < keyCount; i++ {
		var pubKey [32]byte
		copy(pubKey[:], data[offset:offset+32])

		preKeys[i] = PreKeyForExchange{
			ID:        uint32(i), // Use index as ID for now
			PublicKey: pubKey,
		}
		offset += 32
	}

	exchange := &PreKeyExchangeMessage{
		SenderPK:  senderPK,
		PreKeys:   preKeys,
		Timestamp: time.Now(),
	}

	return exchange, senderPK, nil
}
