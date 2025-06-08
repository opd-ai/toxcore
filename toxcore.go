// Package toxcore implements the core functionality of the Tox protocol.
//
// Tox is a peer-to-peer, encrypted messaging protocol designed for secure
// communications without relying on centralized infrastructure.
//
// Example:
//
//	options := toxcore.NewOptions()
//	options.UDPEnabled = true
//
//	tox, err := toxcore.New(options)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
//	    tox.AddFriend(publicKey, "Thanks for the request!")
//	})
//
//	tox.OnFriendMessage(func(friendID uint32, message string) {
//	    fmt.Printf("Message from %d: %s\n", friendID, message)
//	})
//
//	// Connect to the Tox network through a bootstrap node
//	err = tox.Bootstrap("node.tox.example.com", 33445, "FCBDA8AF731C1D70DCF950BA05BD40E2")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Start the Tox event loop
//	for tox.IsRunning() {
//	    tox.Iterate()
//	    time.Sleep(tox.IterationInterval())
//	}
package toxcore

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/friend"
	"github.com/opd-ai/toxcore/transport"
)

// ConnectionStatus represents a connection status.
type ConnectionStatus uint8

const (
	ConnectionNone ConnectionStatus = iota
	ConnectionTCP
	ConnectionUDP
)

// Options contains configuration options for creating a Tox instance.
//
//export ToxOptions
type Options struct {
	UDPEnabled       bool
	IPv6Enabled      bool
	LocalDiscovery   bool
	Proxy            *ProxyOptions
	StartPort        uint16
	EndPort          uint16
	TCPPort          uint16
	SavedataType     SaveDataType
	SavedataData     []byte
	SavedataLength   uint32
	ThreadsEnabled   bool
	BootstrapTimeout time.Duration
}

// ProxyOptions contains proxy configuration.
type ProxyOptions struct {
	Type     ProxyType
	Host     string
	Port     uint16
	Username string
	Password string
}

// ProxyType specifies the type of proxy to use.
type ProxyType uint8

const (
	ProxyTypeNone ProxyType = iota
	ProxyTypeHTTP
	ProxyTypeSOCKS5
)

// SaveDataType specifies the type of saved data.
type SaveDataType uint8

const (
	SaveDataTypeNone SaveDataType = iota
	SaveDataTypeToxSave
	SaveDataTypeSecretKey
)

// SaveData represents the serializable state of a Tox instance.
type SaveData struct {
	SecretKey        [32]byte
	PublicKey        [32]byte
	Nospam           [4]byte
	Friends          []SavedFriend
	ConnectionStatus ConnectionStatus
	Timestamp        int64
}

// SavedFriend represents a friend's saved state.
type SavedFriend struct {
	FriendID      uint32
	PublicKey     [32]byte
	Status        FriendStatus
	Name          string
	StatusMessage string
	LastSeen      int64
}

// Serialize converts SaveData to a byte slice using JSON for simplicity.
func (s *SaveData) Serialize() []byte {
	data, _ := json.Marshal(s)
	return data
}

// LoadSaveData deserializes a byte slice into SaveData using JSON.
func LoadSaveData(data []byte) (*SaveData, error) {
	var saveData SaveData
	err := json.Unmarshal(data, &saveData)
	if err != nil {
		return nil, err
	}
	return &saveData, nil
}

// NewOptions creates a new default Options.
//
//export ToxOptionsNew
func NewOptions() *Options {
	return &Options{
		UDPEnabled:       true,
		IPv6Enabled:      true,
		LocalDiscovery:   true,
		StartPort:        33445,
		EndPort:          33545,
		TCPPort:          0, // Disabled by default
		SavedataType:     SaveDataTypeNone,
		ThreadsEnabled:   true,
		BootstrapTimeout: 5 * time.Second,
	}
}

// Tox represents a Tox instance.
//
//export Tox
type Tox struct {
	// Core components
	options          *Options
	keyPair          *crypto.KeyPair
	dht              *dht.RoutingTable
	selfAddress      net.Addr
	udpTransport     *transport.UDPTransport
	bootstrapManager *dht.BootstrapManager

	// State
	connectionStatus ConnectionStatus
	running          bool
	iterationTime    time.Duration
	nospam           [4]byte

	// Friend-related fields
	friends      map[uint32]*Friend
	friendsMutex sync.RWMutex

	// Callbacks
	friendRequestCallback    FriendRequestCallback
	friendMessageCallback    FriendMessageCallback
	friendStatusCallback     FriendStatusCallback
	connectionStatusCallback ConnectionStatusCallback
	fileRecvCallback         FileRecvCallback
	fileRecvChunkCallback    FileRecvChunkCallback
	fileChunkRequestCallback FileChunkRequestCallback

	// Message processing
	messageQueue      []*PendingMessage
	messageQueueMutex sync.Mutex

	// Friend request processing
	requestManager *friend.RequestManager

	// Context for clean shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// GetSavedata returns the current Tox state as a byte slice for persistence.
//
//export ToxGetSavedata
func (t *Tox) GetSavedata() []byte {
	saveData := &SaveData{
		SecretKey:        t.keyPair.Private,
		PublicKey:        t.keyPair.Public,
		Nospam:           t.nospam,
		Friends:          make([]SavedFriend, 0),
		ConnectionStatus: t.connectionStatus,
		Timestamp:        time.Now().Unix(),
	}

	// Save friends
	t.friendsMutex.RLock()
	for id, friend := range t.friends {
		savedFriend := SavedFriend{
			FriendID:         id,
			PublicKey:        friend.PublicKey,
			Status:           friend.Status,
			Name:             friend.Name,
			StatusMessage:    friend.StatusMessage,
			LastSeen:         friend.LastSeen.Unix(),
		}
		saveData.Friends = append(saveData.Friends, savedFriend)
	}
	t.friendsMutex.RUnlock()

	// Serialize to bytes (simplified implementation)
	return saveData.Serialize()
}

// New creates a new Tox instance with the given options.
//
//export ToxNew
func New(options *Options) (*Tox, error) {
	if options == nil {
		options = NewOptions()
	}

	// Create key pair
	var keyPair *crypto.KeyPair
	var err error

	if options.SavedataType == SaveDataTypeSecretKey && len(options.SavedataData) == 32 {
		// Create from saved secret key
		var secretKey [32]byte
		copy(secretKey[:], options.SavedataData)
		keyPair, err = crypto.FromSecretKey(secretKey)
	} else {
		// Generate new key pair
		keyPair, err = crypto.GenerateKeyPair()
	}

	if err != nil {
		return nil, err
	}

	// Create Tox ID from public key
	toxID := crypto.NewToxID(keyPair.Public, generateNospam())

	// Set up UDP transport if enabled
	var udpTransport *transport.UDPTransport
	if options.UDPEnabled {
		// Try ports in the range [StartPort, EndPort]
		for port := options.StartPort; port <= options.EndPort; port++ {
			addr := net.JoinHostPort("0.0.0.0", string(port))
			transportImpl, err := transport.NewUDPTransport(addr)
			if err == nil {
				var ok bool
				udpTransport, ok = transportImpl.(*transport.UDPTransport)
				if !ok {
					err = errors.New("unexpected transport type")
					continue
				}
				break
			}
		}

		if udpTransport == nil {
			return nil, errors.New("failed to bind to any UDP port")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	rdht := dht.NewRoutingTable(*toxID, 8)
	bootstrapManager := dht.NewBootstrapManager(*toxID, udpTransport, rdht)

	tox := &Tox{
		options:          options,
		keyPair:          keyPair,
		dht:              rdht,
		udpTransport:     udpTransport,
		bootstrapManager: bootstrapManager,
		connectionStatus: ConnectionNone,
		running:          true,
		iterationTime:    50 * time.Millisecond,
		nospam:           generateNospam(),
		friends:          make(map[uint32]*Friend),
		messageQueue:     make([]*PendingMessage, 0),
		requestManager:   friend.NewRequestManager(),
		ctx:              ctx,
		cancel:           cancel,
	}

	// Register handlers for the UDP transport
	if udpTransport != nil {
		tox.registerUDPHandlers()
	}

	// Load friends from saved data if available
	if options.SavedataType == SaveDataTypeToxSave && len(options.SavedataData) > 0 {
		err := tox.loadFromSaveData(options.SavedataData)
		if err != nil {
			// Log error but don't fail completely
			// In a real implementation, you might want to handle this differently
		}
	}

	return tox, nil
}

// registerUDPHandlers sets up packet handlers for the UDP transport.
func (t *Tox) registerUDPHandlers() {
	t.udpTransport.RegisterHandler(transport.PacketPingRequest, t.handlePingRequest)
	t.udpTransport.RegisterHandler(transport.PacketPingResponse, t.handlePingResponse)
	t.udpTransport.RegisterHandler(transport.PacketGetNodes, t.handleGetNodes)
	t.udpTransport.RegisterHandler(transport.PacketSendNodes, t.handleSendNodes)

	// Register friend-related packet handlers
	t.udpTransport.RegisterHandler(transport.PacketFriendRequest, t.handleFriendRequestPacket)
	t.udpTransport.RegisterHandler(transport.PacketFriendMessage, t.handleFriendMessagePacket)
}

// handlePingRequest processes ping request packets.
func (t *Tox) handlePingRequest(packet *transport.Packet, addr net.Addr) error {
	// Implementation of ping request handling
	// This would decrypt the packet, verify it, and send a response
	return nil
}

// handlePingResponse processes ping response packets.
func (t *Tox) handlePingResponse(packet *transport.Packet, addr net.Addr) error {
	// Implementation of ping response handling
	return nil
}

// handleGetNodes processes get nodes request packets.
func (t *Tox) handleGetNodes(packet *transport.Packet, addr net.Addr) error {
	// Implementation of get nodes handling
	return nil
}

// handleSendNodes processes send nodes response packets.
func (t *Tox) handleSendNodes(packet *transport.Packet, addr net.Addr) error {
	// Implementation of send nodes handling
	return nil
}

// handleFriendRequestPacket processes friend request packets.
func (t *Tox) handleFriendRequestPacket(packet *transport.Packet, addr net.Addr) error {
	// Decrypt and parse friend request
	if t.requestManager == nil {
		return errors.New("request manager not initialized")
	}

	// In a real implementation, this would:
	// 1. Decrypt the packet using our secret key
	// 2. Parse the sender's public key and message
	// 3. Create a friend request object
	// 4. Add it to the request manager

	// For demonstration, create a mock request
	var senderPublicKey [32]byte
	copy(senderPublicKey[:], packet.Data[:32]) // Mock extraction
	message := "Friend request received"       // Mock message

	request := &friend.Request{
		SenderPublicKey: senderPublicKey,
		Message:         message,
		Timestamp:       time.Now(),
		Handled:         false,
	}

	t.requestManager.AddRequest(request)
	return nil
}

// handleFriendMessagePacket processes friend message packets.
func (t *Tox) handleFriendMessagePacket(packet *transport.Packet, addr net.Addr) error {
	// In a real implementation, this would:
	// 1. Decrypt the message using the shared secret with the friend
	// 2. Verify the sender's identity
	// 3. Extract the message content and type
	// 4. Call the message callback

	// For demonstration, extract basic info
	if len(packet.Data) < 36 { // 32 bytes public key + 4 bytes friend ID
		return errors.New("invalid message packet")
	}

	var senderPublicKey [32]byte
	copy(senderPublicKey[:], packet.Data[:32])

	// Find friend ID by public key
	friendID, exists := t.getFriendIDByPublicKey(senderPublicKey)
	if !exists {
		return errors.New("message from unknown friend")
	}

	// Mock message extraction (in real implementation, this would be decrypted)
	message := "Received message" // Mock message content
	messageType := MessageTypeNormal

	// Trigger callback if registered
	if t.friendMessageCallback != nil {
		t.friendMessageCallback(friendID, message, messageType)
	}

	return nil
}

// Iterate performs a single iteration of the Tox event loop.
//
//export ToxIterate
func (t *Tox) Iterate() {
	// Process incoming friend requests
	t.processFriendRequests()

	// Process DHT maintenance
	t.doDHTMaintenance()

	// Process friend connections
	t.doFriendConnections()

	// Process message queue
	t.doMessageProcessing()

	// Process incoming messages
	t.processIncomingMessages()
}

// doDHTMaintenance performs periodic DHT maintenance tasks.
func (t *Tox) doDHTMaintenance() {
	// Implementation of DHT maintenance
	// - Ping known nodes
	// - Remove stale nodes
	// - Look for new nodes if needed
}

// doFriendConnections manages friend connections.
func (t *Tox) doFriendConnections() {
	// Implementation of friend connection management
	// - Check status of friends
	// - Try to establish connections to offline friends
	// - Maintain existing connections
}

// doMessageProcessing handles the message queue.
func (t *Tox) doMessageProcessing() {
	t.messageQueueMutex.Lock()
	defer t.messageQueueMutex.Unlock()

	now := time.Now()

	// Process pending messages
	for i := len(t.messageQueue) - 1; i >= 0; i-- {
		msg := t.messageQueue[i]

		// Check if enough time has passed since last attempt
		if now.Sub(msg.LastTry) < time.Second {
			continue
		}

		// Try to send the message
		if t.attemptSendMessage(msg) {
			// Remove from queue if successful
			t.messageQueue = append(t.messageQueue[:i], t.messageQueue[i+1:]...)
		} else {
			// Update attempt info
			msg.Attempts++
			msg.LastTry = now

			// Remove if too many attempts
			if msg.Attempts > 5 {
				t.messageQueue = append(t.messageQueue[:i], t.messageQueue[i+1:]...)
			}
		}
	}
}

// processFriendRequests handles incoming friend requests.
func (t *Tox) processFriendRequests() {
	if t.requestManager == nil {
		return
	}

	// Set up request handler if callback is registered
	if t.friendRequestCallback != nil {
		t.requestManager.SetHandler(func(request *friend.Request) bool {
			// Call the registered callback
			t.friendRequestCallback(request.SenderPublicKey, request.Message)
			return true // Auto-accept for now, user can implement custom logic
		})
	}

	// Process any pending requests
	pendingRequests := t.requestManager.GetPendingRequests()
	for _, request := range pendingRequests {
		if !request.Handled && t.friendRequestCallback != nil {
			t.friendRequestCallback(request.SenderPublicKey, request.Message)
		}
	}
}

// processIncomingMessages handles incoming friend messages.
func (t *Tox) processIncomingMessages() {
	// This would be called when messages are received from the transport layer
	// For now, this is a placeholder for the actual message processing logic

	// In a real implementation, this would:
	// 1. Decrypt incoming message packets
	// 2. Verify sender identity
	// 3. Call the friendMessageCallback
	// 4. Handle delivery confirmations
}

// attemptSendMessage tries to send a pending message.
func (t *Tox) attemptSendMessage(msg *PendingMessage) bool {
	t.friendsMutex.RLock()
	friend, exists := t.friends[msg.FriendID]
	t.friendsMutex.RUnlock()

	if !exists {
		return false // Friend no longer exists
	}

	if friend.ConnectionStatus == ConnectionNone {
		return false // Friend not connected
	}

	// In a real implementation, this would:
	// 1. Encrypt the message
	// 2. Send it via the transport layer
	// 3. Return success/failure

	// For now, simulate successful sending
	return true
}

// handleIncomingFriendMessage processes an incoming friend message.
func (t *Tox) handleIncomingFriendMessage(friendID uint32, message string, messageType MessageType) {
	if t.friendMessageCallback != nil {
		t.friendMessageCallback(friendID, message, messageType)
	}
}

// handleIncomingFriendRequest processes an incoming friend request.
func (t *Tox) handleIncomingFriendRequest(publicKey [32]byte, message string) {
	if t.friendRequestCallback != nil {
		t.friendRequestCallback(publicKey, message)
	}
}

// notifyConnectionStatusChange notifies about connection status changes.
func (t *Tox) notifyConnectionStatusChange(status ConnectionStatus) {
	t.connectionStatus = status
	if t.connectionStatusCallback != nil {
		t.connectionStatusCallback(status)
	}
}

// notifyFriendStatusChange notifies about friend status changes.
func (t *Tox) notifyFriendStatusChange(friendID uint32, status FriendStatus) {
	if t.friendStatusCallback != nil {
		t.friendStatusCallback(friendID, status)
	}
}

// IterationInterval returns the recommended interval between iterations.
//
//export ToxIterationInterval
func (t *Tox) IterationInterval() time.Duration {
	return t.iterationTime
}

// IsRunning checks if the Tox instance is still running.
//
//export ToxIsRunning
func (t *Tox) IsRunning() bool {
	return t.running
}

// Kill stops the Tox instance and releases all resources.
//
//export ToxKill
func (t *Tox) Kill() {
	t.running = false
	t.cancel()

	if t.udpTransport != nil {
		t.udpTransport.Close()
	}

	// Clean up message queue
	t.messageQueueMutex.Lock()
	t.messageQueue = nil
	t.messageQueueMutex.Unlock()

	// Clean up friends list
	t.friendsMutex.Lock()
	t.friends = nil
	t.friendsMutex.Unlock()

	// Clean up request manager
	if t.requestManager != nil {
		t.requestManager.Clear()
		t.requestManager = nil
	}

	// Clean up DHT and bootstrap manager
	if t.bootstrapManager != nil {
		t.bootstrapManager.Stop()
		t.bootstrapManager = nil
	}
	t.dht = nil
}
}

// Bootstrap connects to a bootstrap node to join the Tox network.
//
//export ToxBootstrap
func (t *Tox) Bootstrap(address string, port uint16, publicKeyHex string) error {
	// Add the bootstrap node to the bootstrap manager
	err := t.bootstrapManager.AddNode(address, port, publicKeyHex)
	if err != nil {
		return err
	}

	// Attempt to bootstrap with a timeout
	ctx, cancel := context.WithTimeout(t.ctx, t.options.BootstrapTimeout)
	defer cancel()

	return t.bootstrapManager.Bootstrap(ctx)
}

// SelfGetAddress returns the Tox ID of this instance.
//
//export ToxSelfGetAddress
func (t *Tox) SelfGetAddress() string {
	toxID := crypto.NewToxID(t.keyPair.Public, t.nospam)
	return toxID.String()
}

// SelfGetPublicKey returns the public key of this instance.
//
//export ToxSelfGetPublicKey
func (t *Tox) SelfGetPublicKey() [32]byte {
	return t.keyPair.Public
}

// SelfGetSecretKey returns the secret key of this instance.
//
//export ToxSelfGetSecretKey
func (t *Tox) SelfGetSecretKey() [32]byte {
	return t.keyPair.Private
}

// SelfGetConnectionStatus returns the current connection status.
//
//export ToxSelfGetConnectionStatus
func (t *Tox) SelfGetConnectionStatus() ConnectionStatus {
	return t.connectionStatus
}

// Friend represents a Tox friend.
type Friend struct {
	PublicKey        [32]byte
	Status           FriendStatus
	ConnectionStatus ConnectionStatus
	Name             string
	StatusMessage    string
	LastSeen         time.Time
	UserData         interface{}
}

// FriendStatus represents the status of a friend.
type FriendStatus uint8

const (
	FriendStatusNone FriendStatus = iota
	FriendStatusAway
	FriendStatusBusy
	FriendStatusOnline
)

// FriendRequestCallback is called when a friend request is received.
type FriendRequestCallback func(publicKey [32]byte, message string)

// FriendStatusCallback is called when a friend's status changes.
type FriendStatusCallback func(friendID uint32, status FriendStatus)

// ConnectionStatusCallback is called when the connection status changes.
type ConnectionStatusCallback func(status ConnectionStatus)

// FileRecvCallback is called when a file transfer is offered.
type FileRecvCallback func(friendID uint32, fileID uint32, kind uint32, fileSize uint64, filename string)

// FileRecvChunkCallback is called when a file chunk is received.
type FileRecvChunkCallback func(friendID uint32, fileID uint32, position uint64, data []byte)

// FileChunkRequestCallback is called when a file chunk is requested.
type FileChunkRequestCallback func(friendID uint32, fileID uint32, position uint64, length int)

// PendingMessage represents a message waiting to be sent.
type PendingMessage struct {
	FriendID    uint32
	Message     string
	MessageType MessageType
	Attempts    int
	LastTry     time.Time
}

// OnFriendRequest sets the callback for friend requests.
//
//export ToxOnFriendRequest
func (t *Tox) OnFriendRequest(callback FriendRequestCallback) {
	t.friendRequestCallback = callback
}

// OnFriendMessage sets the callback for friend messages.
//
//export ToxOnFriendMessage
func (t *Tox) OnFriendMessage(callback FriendMessageCallback) {
	t.friendMessageCallback = callback
}

// OnFriendStatus sets the callback for friend status changes.
//
//export ToxOnFriendStatus
func (t *Tox) OnFriendStatus(callback FriendStatusCallback) {
	t.friendStatusCallback = callback
}

// OnConnectionStatus sets the callback for connection status changes.
//
//export ToxOnConnectionStatus
func (t *Tox) OnConnectionStatus(callback ConnectionStatusCallback) {
	t.connectionStatusCallback = callback
}

func (t *Tox) AddFriend(address string) (uint32, error) {
	return t.AddFriendMessage(address, "friend request")
}

// AddFriendByPublicKey adds a friend directly by public key (used by callbacks).
//
//export ToxAddFriendByPublicKey
func (t *Tox) AddFriendByPublicKey(publicKey [32]byte) (uint32, error) {
	// Check if already a friend
	friendID, exists := t.getFriendIDByPublicKey(publicKey)
	if exists {
		return friendID, errors.New("already a friend")
	}

	// Create a new friend
	friendID = t.generateFriendID()
	friend := &Friend{
		PublicKey:        publicKey,
		Status:           FriendStatusNone,
		ConnectionStatus: ConnectionNone,
		LastSeen:         time.Now(),
	}

	// Add to friends list
	t.friendsMutex.Lock()
	t.friends[friendID] = friend
	t.friendsMutex.Unlock()

	return friendID, nil
}

// AddFriend adds a friend by Tox ID.
//
//export ToxAddFriendMessage
func (t *Tox) AddFriendMessage(address string, message string) (uint32, error) {
	// Parse the Tox ID
	toxID, err := crypto.ToxIDFromString(address)
	if err != nil {
		return 0, err
	}

	// Check if already a friend
	friendID, exists := t.getFriendIDByPublicKey(toxID.PublicKey)
	if exists {
		return friendID, errors.New("already a friend")
	}

	// Create a new friend
	friendID = t.generateFriendID()
	friend := &Friend{
		PublicKey:        toxID.PublicKey,
		Status:           FriendStatusNone,
		ConnectionStatus: ConnectionNone,
		LastSeen:         time.Now(),
	}

	// Add to friends list
	t.friendsMutex.Lock()
	t.friends[friendID] = friend
	t.friendsMutex.Unlock()

	// Send friend request
	// This would be implemented in the actual code

	return friendID, nil
}

// getFriendIDByPublicKey finds a friend ID by public key.
func (t *Tox) getFriendIDByPublicKey(publicKey [32]byte) (uint32, bool) {
	t.friendsMutex.RLock()
	defer t.friendsMutex.RUnlock()

	for id, friend := range t.friends {
		if friend.PublicKey == publicKey {
			return id, true
		}
	}

	return 0, false
}

// generateFriendID creates a new unique friend ID.
func (t *Tox) generateFriendID() uint32 {
	t.friendsMutex.RLock()
	defer t.friendsMutex.RUnlock()

	// Find the first unused ID
	var id uint32 = 0
	for {
		if _, exists := t.friends[id]; !exists {
			return id
		}
		id++
	}
}

// generateNospam creates a random nospam value.
func generateNospam() [4]byte {
	var nospam [4]byte
	_, _ = crypto.GenerateNonce() // Use some bytes from a nonce
	// In real implementation, would use proper random generator
	return nospam
}

// SendFriendMessage sends a message to a friend.
//
//export ToxSendFriendMessage
func (t *Tox) SendFriendMessage(friendID uint32, message string) error {
	t.friendsMutex.RLock()
	_, exists := t.friends[friendID]
	t.friendsMutex.RUnlock()

	if !exists {
		return errors.New("friend not found")
	}

	// Create pending message
	pendingMsg := &PendingMessage{
		FriendID:    friendID,
		Message:     message,
		MessageType: MessageTypeNormal,
		Attempts:    0,
		LastTry:     time.Time{}, // Will be set on first attempt
	}

	// Add to message queue
	t.messageQueueMutex.Lock()
	t.messageQueue = append(t.messageQueue, pendingMsg)
	t.messageQueueMutex.Unlock()

	return nil
}

// FriendExists checks if a friend exists.
//
//export ToxFriendExists
func (t *Tox) FriendExists(friendID uint32) bool {
	t.friendsMutex.RLock()
	defer t.friendsMutex.RUnlock()

	_, exists := t.friends[friendID]
	return exists
}

// GetFriendByPublicKey gets a friend ID by public key.
//
//export ToxGetFriendByPublicKey
func (t *Tox) GetFriendByPublicKey(publicKey [32]byte) (uint32, error) {
	id, exists := t.getFriendIDByPublicKey(publicKey)
	if !exists {
		return 0, errors.New("friend not found")
	}
	return id, nil
}

// GetFriendPublicKey gets a friend's public key.
//
//export ToxGetFriendPublicKey
func (t *Tox) GetFriendPublicKey(friendID uint32) ([32]byte, error) {
	t.friendsMutex.RLock()
	defer t.friendsMutex.RUnlock()

	friend, exists := t.friends[friendID]
	if !exists {
		return [32]byte{}, errors.New("friend not found")
	}

	return friend.PublicKey, nil
}

// Save saves the Tox state to a byte slice.
//
//export ToxSave
func (t *Tox) Save() ([]byte, error) {
	// Implementation of state serialization
	// This would save keys, friends list, DHT state, etc.
	return nil, nil
}

// Load loads the Tox state from a byte slice.
//
//export ToxLoad
func (t *Tox) Load(data []byte) error {
	// Implementation of state deserialization
	return nil
}

// MessageType represents the type of a message.
type MessageType uint8

const (
	MessageTypeNormal MessageType = iota
	MessageTypeAction
)

// FriendMessageCallback is called when a message is received from a friend.
type FriendMessageCallback func(friendID uint32, message string, messageType MessageType)

// DeleteFriend removes a friend from the friends list.
//
//export ToxDeleteFriend
func (t *Tox) DeleteFriend(friendID uint32) error {
	t.friendsMutex.Lock()
	defer t.friendsMutex.Unlock()

	if _, exists := t.friends[friendID]; !exists {
		return errors.New("friend not found")
	}

	delete(t.friends, friendID)
	return nil
}

// SelfSetName sets the name of this Tox instance.
//
//export ToxSelfSetName
func (t *Tox) SelfSetName(name string) error {
	// Implementation of setting self name
	return nil
}

// SelfGetName gets the name of this Tox instance.
//
//export ToxSelfGetName
func (t *Tox) SelfGetName() string {
	// Implementation of getting self name
	return ""
}

// SelfSetStatusMessage sets the status message of this Tox instance.
//
//export ToxSelfSetStatusMessage
func (t *Tox) SelfSetStatusMessage(message string) error {
	// Implementation of setting status message
	return nil
}

// SelfGetStatusMessage gets the status message of this Tox instance.
//
//export ToxSelfGetStatusMessage
func (t *Tox) SelfGetStatusMessage() string {
	// Implementation of getting status message
	return ""
}

// FriendSendMessage sends a message to a friend with a specified type.
//
//export ToxFriendSendMessage
func (t *Tox) FriendSendMessage(friendID uint32, message string, messageType MessageType) (uint32, error) {
	t.friendsMutex.RLock()
	friend, exists := t.friends[friendID]
	t.friendsMutex.RUnlock()

	if !exists {
		return 0, errors.New("friend not found")
	}

	if friend.ConnectionStatus == ConnectionNone {
		return 0, errors.New("friend not connected")
	}

	// Send the message with the specified type
	// This would be implemented in the actual code
	// Return a message ID
	return 0, nil
}

// FileControl represents a file transfer control action.
type FileControl uint8

const (
	FileControlResume FileControl = iota
	FileControlPause
	FileControlCancel
)

// FileControl controls an ongoing file transfer.
//
//export ToxFileControl
func (t *Tox) FileControl(friendID uint32, fileID uint32, control FileControl) error {
	// Implementation of file control
	return nil
}

// FileSend starts a file transfer.
//
//export ToxFileSend
func (t *Tox) FileSend(friendID uint32, kind uint32, fileSize uint64, fileID [32]byte, filename string) (uint32, error) {
	// Implementation of file send
	return 0, nil
}

// FileSendChunk sends a chunk of file data.
//
//export ToxFileSendChunk
func (t *Tox) FileSendChunk(friendID uint32, fileID uint32, position uint64, data []byte) error {
	// Implementation of file send chunk
	return nil
}

// OnFileRecv sets the callback for file receive events.
//
//export ToxOnFileRecv
func (t *Tox) OnFileRecv(callback FileRecvCallback) {
	t.fileRecvCallback = callback
}

// OnFileRecvChunk sets the callback for file chunk receive events.
//
//export ToxOnFileRecvChunk
func (t *Tox) OnFileRecvChunk(callback FileRecvChunkCallback) {
	t.fileRecvChunkCallback = callback
}

// OnFileChunkRequest sets the callback for file chunk request events.
//
//export ToxOnFileChunkRequest
func (t *Tox) OnFileChunkRequest(callback FileChunkRequestCallback) {
	t.fileChunkRequestCallback = callback
}

// ConferenceNew creates a new conference (group chat).
//
//export ToxConferenceNew
func (t *Tox) ConferenceNew() (uint32, error) {
	// Implementation of conference creation
	return 0, nil
}

// ConferenceInvite invites a friend to a conference.
//
//export ToxConferenceInvite
func (t *Tox) ConferenceInvite(friendID uint32, conferenceID uint32) error {
	// Implementation of conference invitation
	return nil
}

// ConferenceSendMessage sends a message to a conference.
//
//export ToxConferenceSendMessage
func (t *Tox) ConferenceSendMessage(conferenceID uint32, message string, messageType MessageType) error {
	// Implementation of conference message sending
	return nil
}

// OnFriendName sets the callback for friend name changes.
//
//export ToxOnFriendName
func (t *Tox) OnFriendName(callback func(friendID uint32, name string)) {
	// Store the callback
}

// FriendByPublicKey finds a friend by their public key.
//
//export ToxFriendByPublicKey
func (t *Tox) FriendByPublicKey(publicKey [32]byte) (uint32, error) {
	id, found := t.getFriendIDByPublicKey(publicKey)
	if !found {
		return 0, errors.New("friend not found")
	}
	return id, nil
}

// loadFromSaveData loads the Tox state from saved data.
func (t *Tox) loadFromSaveData(data []byte) error {
	saveData, err := LoadSaveData(data)
	if err != nil {
		return err
	}

	// Update key pair if provided
	if saveData.SecretKey != [32]byte{} && saveData.PublicKey != [32]byte{} {
		t.keyPair = &crypto.KeyPair{
			Private: saveData.SecretKey,
			Public:  saveData.PublicKey,
		}
	}

	// Update nospam
	if saveData.Nospam != [4]byte{} {
		t.nospam = saveData.Nospam
	}

	// Load friends
	t.friendsMutex.Lock()
	for _, savedFriend := range saveData.Friends {
		friend := &Friend{
			PublicKey:        savedFriend.PublicKey,
			Status:           savedFriend.Status,
			ConnectionStatus: ConnectionNone, // Start as disconnected
			Name:             savedFriend.Name,
			StatusMessage:    savedFriend.StatusMessage,
			LastSeen:         time.Unix(savedFriend.LastSeen, 0),
		}
		t.friends[savedFriend.FriendID] = friend
	}
	t.friendsMutex.Unlock()

	return nil
}
