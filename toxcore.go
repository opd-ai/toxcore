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
	"encoding/hex"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
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
	options      *Options
	keyPair      *crypto.KeyPair
	dht          *dht.RoutingTable
	selfAddress  net.Addr
	udpTransport *transport.UDPTransport

	// State
	connectionStatus ConnectionStatus
	running          bool
	iterationTime    time.Duration

	// Friend-related fields
	friends      map[uint32]*Friend
	friendsMutex sync.RWMutex

	// Callbacks
	friendRequestCallback    FriendRequestCallback
	friendMessageCallback    FriendMessageCallback
	friendStatusCallback     FriendStatusCallback
	connectionStatusCallback ConnectionStatusCallback

	// Context for clean shutdown
	ctx    context.Context
	cancel context.CancelFunc
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
			udpTransport, err = transport.NewUDPTransport(addr)
			if err == nil {
				break
			}
		}

		if udpTransport == nil {
			return nil, errors.New("failed to bind to any UDP port")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	tox := &Tox{
		options:          options,
		keyPair:          keyPair,
		dht:              dht.NewRoutingTable(toxID, 8),
		udpTransport:     udpTransport,
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
	publicKey, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return err
	}

	if len(publicKey) != 32 {
		return errors.New("invalid public key length")
	}

	// Resolve the address
	host := net.JoinHostPort(address, string(port))
	addr, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		return err
	}

	// Create a Tox ID for the bootstrap node
	var pubKeyArray [32]byte
	copy(pubKeyArray[:], publicKey)

	var nospam [4]byte // Zeros for bootstrap nodes
	nodeID := crypto.NewToxID(pubKeyArray, nospam)

	// Create a node object
	node := dht.NewNode(nodeID, addr)

	// Add to routing table
	t.dht.AddNode(node)

	// Send get nodes request to the bootstrap node
	// This would be implemented in the actual code

	return nil
}

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

// FriendMessageCallback is called when a message is received from a friend.
type FriendMessageCallback func(friendID uint32, message string)

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
	friend, exists := t.friends[friendID]
	t.friendsMutex.RUnlock()

	if !exists {
		return errors.New("friend not found")
	}

	if friend.ConnectionStatus == ConnectionNone {
		return errors.New("friend not connected")
	}

	// Send the message
	// This would be implemented in the actual code

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
