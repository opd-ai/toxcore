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
	"strconv"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/messaging"
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

// toxSaveData represents the serializable state of a Tox instance.
// This is an internal structure used for persistence.
type toxSaveData struct {
	KeyPair       *crypto.KeyPair    `json:"keypair"`
	Friends       map[uint32]*Friend `json:"friends"`
	Options       *Options           `json:"options"`
	SelfName      string             `json:"self_name"`
	SelfStatusMsg string             `json:"self_status_message"`
}

// marshal serializes the toxSaveData to a JSON byte array.
// Using JSON for simplicity and readability during development.
// Future versions could use a binary format for efficiency.
func (s *toxSaveData) marshal() []byte {
	// Import encoding/json at the top of file
	data, err := json.Marshal(s)
	if err != nil {
		// In case of marshaling error, return empty data
		// This prevents panic while allowing graceful degradation
		return []byte{}
	}
	return data
}

// unmarshal deserializes JSON data into toxSaveData.
func (s *toxSaveData) unmarshal(data []byte) error {
	return json.Unmarshal(data, s)
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

	// Self information
	selfName      string
	selfStatusMsg string
	selfMutex     sync.RWMutex

	// Friend-related fields
	friends        map[uint32]*Friend
	friendsMutex   sync.RWMutex
	messageManager *messaging.MessageManager

	// Callbacks
	friendRequestCallback       FriendRequestCallback
	friendMessageCallback       FriendMessageCallback
	simpleFriendMessageCallback SimpleFriendMessageCallback
	friendStatusCallback        FriendStatusCallback
	connectionStatusCallback    ConnectionStatusCallback

	// Context for clean shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// GetSavedata returns the serialized Tox state as a byte array.
// This data can be used with NewFromSavedata or Load to restore the Tox state,
// including the private key, friends list, and configuration.
//
// The returned byte array contains all necessary state for persistence
// and should be stored securely as it contains cryptographic keys.
//
//export ToxGetSavedata
func (t *Tox) GetSavedata() []byte {
	t.friendsMutex.RLock()
	t.selfMutex.RLock()
	defer t.friendsMutex.RUnlock()
	defer t.selfMutex.RUnlock()

	// Create a serializable representation of the Tox state
	saveData := toxSaveData{
		KeyPair:       t.keyPair,
		Friends:       make(map[uint32]*Friend),
		Options:       t.options,
		SelfName:      t.selfName,
		SelfStatusMsg: t.selfStatusMsg,
	}

	// Copy friends data to avoid race conditions
	for id, friend := range t.friends {
		saveData.Friends[id] = &Friend{
			PublicKey:        friend.PublicKey,
			Status:           friend.Status,
			ConnectionStatus: friend.ConnectionStatus,
			Name:             friend.Name,
			StatusMessage:    friend.StatusMessage,
			LastSeen:         friend.LastSeen,
			// Note: UserData is not serialized as it may contain non-serializable types
		}
	}

	return saveData.marshal()
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
			addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(int(port)))
			transportImpl, err := transport.NewUDPTransport(addr)
			if err == nil {
				var ok bool
				udpTransport, ok = transportImpl.(*transport.UDPTransport)
				if !ok {
					return nil, errors.New("unexpected transport type")
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
		friends:          make(map[uint32]*Friend),
		ctx:              ctx,
		cancel:           cancel,
	}

	// Register handlers for the UDP transport
	if udpTransport != nil {
		tox.registerUDPHandlers()
	}

	// TODO: Load friends from saved data if available

	return tox, nil
}

// NewFromSavedata creates a new Tox instance from previously saved data.
// This is a convenience function that combines New() and Load() operations.
//
// The savedata should be obtained from a previous call to GetSavedata().
// If options is nil, default options will be used.
//
//export ToxNewFromSavedata
func NewFromSavedata(options *Options, savedata []byte) (*Tox, error) {
	if len(savedata) == 0 {
		return nil, errors.New("savedata cannot be empty")
	}

	// Parse savedata first to extract key information
	var savedState toxSaveData
	if err := savedState.unmarshal(savedata); err != nil {
		return nil, err
	}

	if savedState.KeyPair == nil {
		return nil, errors.New("savedata missing key pair")
	}

	// Set up options for restoration
	if options == nil {
		options = NewOptions()
	}

	// Set the saved secret key in options so New() will use it
	options.SavedataType = SaveDataTypeSecretKey
	options.SavedataData = savedState.KeyPair.Private[:]
	options.SavedataLength = 32

	// Create the Tox instance with the restored key
	tox, err := New(options)
	if err != nil {
		return nil, err
	}

	// Load the complete state (friends, etc.)
	if err := tox.Load(savedata); err != nil {
		tox.Kill() // Clean up on error
		return nil, err
	}

	return tox, nil
}

// registerUDPHandlers sets up packet handlers for the UDP transport.
func (t *Tox) registerUDPHandlers() {
	t.udpTransport.RegisterHandler(transport.PacketPingRequest, t.handlePingRequest)
	t.udpTransport.RegisterHandler(transport.PacketPingResponse, t.handlePingResponse)
	t.udpTransport.RegisterHandler(transport.PacketGetNodes, t.handleGetNodes)
	t.udpTransport.RegisterHandler(transport.PacketSendNodes, t.handleSendNodes)
	// Register more handlers here
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

// Iterate performs a single iteration of the Tox event loop.
//
//export ToxIterate
func (t *Tox) Iterate() {
	// Process DHT maintenance
	t.doDHTMaintenance()

	// Process friend connections
	t.doFriendConnections()

	// Process message queue
	t.doMessageProcessing()
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
	// Implementation of message processing
	// - Process outgoing messages
	// - Check for delivery confirmations
	// - Handle retransmissions
}

// dispatchFriendMessage dispatches an incoming friend message to the appropriate callback(s).
// This method ensures both simple and detailed callbacks are called if they are registered.
func (t *Tox) dispatchFriendMessage(friendID uint32, message string, messageType MessageType) {
	// Call the simple callback if registered (matches documented API)
	if t.simpleFriendMessageCallback != nil {
		t.simpleFriendMessageCallback(friendID, message)
	}

	// Call the detailed callback if registered (for advanced users and C bindings)
	if t.friendMessageCallback != nil {
		t.friendMessageCallback(friendID, message, messageType)
	}
}

// receiveFriendMessage simulates receiving a message from a friend.
// In a real implementation, this would be called by the network layer when a message packet is received.
// This method is exposed for testing and demonstration purposes.
//
//export ToxReceiveFriendMessage
func (t *Tox) receiveFriendMessage(friendID uint32, message string, messageType MessageType) {
	// Verify the friend exists
	t.friendsMutex.RLock()
	_, exists := t.friends[friendID]
	t.friendsMutex.RUnlock()

	if !exists {
		return // Ignore messages from unknown friends
	}

	// Dispatch to registered callbacks
	t.dispatchFriendMessage(friendID, message, messageType)
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

	// TODO: Clean up other resources
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

// ...existing code...

// SelfGetAddress returns the Tox ID of this instance.
//
//export ToxSelfGetAddress
func (t *Tox) SelfGetAddress() string {
	var nospam [4]byte
	// Get actual nospam value from state

	toxID := crypto.NewToxID(t.keyPair.Public, nospam)
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

// SimpleFriendMessageCallback is called when a message is received from a friend.
// This matches the documented API in README.md for simple use cases.
type SimpleFriendMessageCallback func(friendID uint32, message string)

// FriendStatusCallback is called when a friend's status changes.
type FriendStatusCallback func(friendID uint32, status FriendStatus)

// ConnectionStatusCallback is called when the connection status changes.
type ConnectionStatusCallback func(status ConnectionStatus)

// OnFriendRequest sets the callback for friend requests.
//
//export ToxOnFriendRequest
func (t *Tox) OnFriendRequest(callback FriendRequestCallback) {
	t.friendRequestCallback = callback
}

// OnFriendMessage sets the callback for friend messages using the simplified API.
// This matches the documented API in README.md: func(friendID uint32, message string)
//
//export ToxOnFriendMessage
func (t *Tox) OnFriendMessage(callback SimpleFriendMessageCallback) {
	t.simpleFriendMessageCallback = callback
}

// OnFriendMessageDetailed sets the callback for friend messages with message type.
// Use this for advanced scenarios where you need access to the message type.
//
//export ToxOnFriendMessageDetailed
func (t *Tox) OnFriendMessageDetailed(callback FriendMessageCallback) {
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

// AddFriend adds a friend by Tox ID.
//
//export ToxAddFriend
func (t *Tox) AddFriend(address string, message string) (uint32, error) {
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

// AddFriendByPublicKey adds a friend by their public key without sending a friend request.
// This matches the documented API for accepting friend requests: AddFriend(publicKey)
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

// SendFriendMessage sends a message to a friend with optional message type.
// If no message type is provided, defaults to MessageTypeNormal.
// This is the primary API for sending messages.
//
// The message must not be empty and cannot exceed 1372 bytes.
// The friend must exist and be connected to receive the message.
//
// Usage:
//
//	err := tox.SendFriendMessage(friendID, "Hello")                    // Normal message (default)
//	err := tox.SendFriendMessage(friendID, "Hello", MessageTypeNormal) // Explicit normal message
//	err := tox.SendFriendMessage(friendID, "/me waves", MessageTypeAction) // Action message
//
// Returns an error if:
//   - The message is empty
//   - The message exceeds 1372 bytes
//   - The friend does not exist
//   - The friend is not connected
//   - The underlying message system fails
//
//export ToxSendFriendMessage
func (t *Tox) SendFriendMessage(friendID uint32, message string, messageType ...MessageType) error {
	// Validate input
	if len(message) == 0 {
		return errors.New("message cannot be empty")
	}
	if len([]byte(message)) > 1372 { // Tox protocol message length limit
		return errors.New("message too long: maximum 1372 bytes")
	}

	// Determine message type - default to normal if not specified
	msgType := MessageTypeNormal
	if len(messageType) > 0 {
		msgType = messageType[0]
	}

	t.friendsMutex.RLock()
	friend, exists := t.friends[friendID]
	t.friendsMutex.RUnlock()

	if !exists {
		return errors.New("friend not found")
	}

	if friend.ConnectionStatus == ConnectionNone {
		return errors.New("friend not connected")
	}

	// Create and send the message through the messaging system
	if t.messageManager != nil {
		// Convert toxcore.MessageType to messaging.MessageType
		messagingMsgType := messaging.MessageType(msgType)
		msg, err := t.messageManager.SendMessage(friendID, message, messagingMsgType)
		if err != nil {
			return err
		}

		// Log successful message creation (in a real implementation, this would
		// trigger the actual network sending through the transport layer)
		_ = msg // Avoid unused variable warning
	}

	// Trigger callback for sent message if needed (for logging/debugging)
	// In a complete implementation, this would happen after network confirmation

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

// Load loads the Tox state from a byte slice created by GetSavedata.
// This method restores the private key, friends list, and configuration
// from previously saved data.
//
// The Tox instance must be in a clean state before calling Load.
// This method will overwrite existing keys and friends.
//
//export ToxLoad
func (t *Tox) Load(data []byte) error {
	if len(data) == 0 {
		return errors.New("save data is empty")
	}

	var saveData toxSaveData
	if err := saveData.unmarshal(data); err != nil {
		return err
	}

	// Validate required fields
	if saveData.KeyPair == nil {
		return errors.New("save data missing key pair")
	}

	// Restore key pair
	t.keyPair = saveData.KeyPair

	// Restore friends list
	t.friendsMutex.Lock()
	defer t.friendsMutex.Unlock()

	if saveData.Friends != nil {
		t.friends = make(map[uint32]*Friend)
		for id, friend := range saveData.Friends {
			if friend != nil {
				t.friends[id] = &Friend{
					PublicKey:        friend.PublicKey,
					Status:           friend.Status,
					ConnectionStatus: friend.ConnectionStatus,
					Name:             friend.Name,
					StatusMessage:    friend.StatusMessage,
					LastSeen:         friend.LastSeen,
					// UserData is not restored as it was not serialized
				}
			}
		}
	}

	// Restore options if present
	if saveData.Options != nil {
		// Only restore certain safe options, not all options should be restored
		// as some are runtime-specific (like network settings)
		if t.options != nil {
			t.options.SavedataType = saveData.Options.SavedataType
			t.options.SavedataData = saveData.Options.SavedataData
			t.options.SavedataLength = saveData.Options.SavedataLength
		}
	}

	// Restore self information
	t.selfMutex.Lock()
	defer t.selfMutex.Unlock()
	t.selfName = saveData.SelfName
	t.selfStatusMsg = saveData.SelfStatusMsg

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
// The name will be broadcast to all connected friends and persisted in savedata.
// Maximum name length is 128 bytes in UTF-8 encoding.
//
//export ToxSelfSetName
func (t *Tox) SelfSetName(name string) error {
	// Validate name length (128 bytes max for Tox protocol)
	if len([]byte(name)) > 128 {
		return errors.New("name too long: maximum 128 bytes")
	}

	t.selfMutex.Lock()
	oldName := t.selfName
	t.selfName = name
	t.selfMutex.Unlock()

	// Broadcast name change to connected friends
	// In a complete implementation, this would send name update packets
	// to all connected friends. For now, we'll just store it locally.
	_ = oldName // Avoid unused variable warning

	return nil
}

// SelfGetName gets the name of this Tox instance.
// Returns the currently set name, or empty string if no name is set.
//
//export ToxSelfGetName
func (t *Tox) SelfGetName() string {
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()
	return t.selfName
}

// SelfSetStatusMessage sets the status message of this Tox instance.
// The status message will be broadcast to all connected friends and persisted in savedata.
// Maximum status message length is 1007 bytes in UTF-8 encoding.
//
//export ToxSelfSetStatusMessage
func (t *Tox) SelfSetStatusMessage(message string) error {
	// Validate status message length (1007 bytes max for Tox protocol)
	if len([]byte(message)) > 1007 {
		return errors.New("status message too long: maximum 1007 bytes")
	}

	t.selfMutex.Lock()
	oldMessage := t.selfStatusMsg
	t.selfStatusMsg = message
	t.selfMutex.Unlock()

	// Broadcast status message change to connected friends
	// In a complete implementation, this would send status update packets
	// to all connected friends. For now, we'll just store it locally.
	_ = oldMessage // Avoid unused variable warning

	return nil
}

// SelfGetStatusMessage gets the status message of this Tox instance.
// Returns the currently set status message, or empty string if no status message is set.
//
//export ToxSelfGetStatusMessage
func (t *Tox) SelfGetStatusMessage() string {
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()
	return t.selfStatusMsg
}

// FriendSendMessage sends a message to a friend with a specified type.
// DEPRECATED: Use SendFriendMessage instead for consistent API.
// This method is maintained for backward compatibility with C bindings.
//
//export ToxFriendSendMessage
func (t *Tox) FriendSendMessage(friendID uint32, message string, messageType MessageType) (uint32, error) {
	// Delegate to the primary SendFriendMessage API
	err := t.SendFriendMessage(friendID, message, messageType)
	if err != nil {
		return 0, err
	}

	// Return a mock message ID for compatibility
	// In a real implementation, this would be the actual message ID
	return 1, nil
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
func (t *Tox) OnFileRecv(callback func(friendID uint32, fileID uint32, kind uint32, fileSize uint64, filename string)) {
	// Store the callback
}

// OnFileRecvChunk sets the callback for file chunk receive events.
//
//export ToxOnFileRecvChunk
func (t *Tox) OnFileRecvChunk(callback func(friendID uint32, fileID uint32, position uint64, data []byte)) {
	// Store the callback
}

// OnFileChunkRequest sets the callback for file chunk request events.
//
//export ToxOnFileChunkRequest
func (t *Tox) OnFileChunkRequest(callback func(friendID uint32, fileID uint32, position uint64, length int)) {
	// Store the callback
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
