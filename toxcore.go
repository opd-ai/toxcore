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
//	    tox.AddFriendByPublicKey(publicKey)
//	})
//
//	tox.OnFriendMessage(func(friendID uint32, message string) {
//	    fmt.Printf("Message from %d: %s\n", friendID, message)
//	})
//
//	// Connect to the Tox network through a bootstrap node
//	err = tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
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
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/async"
	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/factory"
	"github.com/opd-ai/toxcore/file"
	"github.com/opd-ai/toxcore/friend"
	"github.com/opd-ai/toxcore/group"
	"github.com/opd-ai/toxcore/interfaces"
	"github.com/opd-ai/toxcore/messaging"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// TimeProvider is an interface for getting the current time.
// This allows injecting a mock time provider for deterministic testing.
type TimeProvider interface {
	Now() time.Time
}

// RealTimeProvider implements TimeProvider using the actual system time.
type RealTimeProvider struct{}

// Now returns the current system time.
func (RealTimeProvider) Now() time.Time {
	return time.Now()
}

// pendingFriendRequest tracks a friend request awaiting network delivery
type pendingFriendRequest struct {
	targetPublicKey [32]byte
	message         string
	packetData      []byte
	timestamp       time.Time
	retryCount      int
	nextRetry       time.Time
}

// ConnectionStatus represents a connection status.
type ConnectionStatus uint8

// Connection status constants define the current transport connection state of a friend.
const (
	// ConnectionNone indicates no active connection to the friend.
	ConnectionNone ConnectionStatus = iota
	// ConnectionTCP indicates the friend is connected via TCP transport.
	ConnectionTCP
	// ConnectionUDP indicates the friend is connected via UDP transport.
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

	// Relay configuration for symmetric NAT fallback
	RelayServers []RelayServerConfig // List of TCP relay servers
	RelayEnabled bool                // Enable relay fallback for failed connections

	// Async messaging configuration
	AsyncStorageEnabled bool // Enable storage node participation (default: true). When disabled, async messaging still works for sending but this node won't store messages for others.

	// Delivery retry configuration
	DeliveryRetryConfig *DeliveryRetryConfig // Optional retry configuration for message delivery

	// Testing configuration
	MinBootstrapNodes int // Minimum nodes required for bootstrap (default: 4, testing: 1)
}

// DeliveryRetryConfig configures automatic retry behavior for message delivery.
// When a message fails to send, it will be retried according to these settings.
type DeliveryRetryConfig struct {
	// Enabled controls whether automatic retry is enabled (default: true)
	Enabled bool
	// MaxRetries is the maximum number of retry attempts (default: 3)
	MaxRetries uint8
	// InitialDelay is the delay before the first retry (default: 5s)
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries (default: 5m)
	MaxDelay time.Duration
	// BackoffFactor is the exponential backoff multiplier (default: 2.0)
	BackoffFactor float64
}

// DefaultDeliveryRetryConfig returns the default delivery retry configuration.
// Defaults: 3 retries, 5s initial delay, 5m max delay, 2.0 backoff factor.
func DefaultDeliveryRetryConfig() *DeliveryRetryConfig {
	return &DeliveryRetryConfig{
		Enabled:       true,
		MaxRetries:    3,
		InitialDelay:  5 * time.Second,
		MaxDelay:      5 * time.Minute,
		BackoffFactor: 2.0,
	}
}

// RelayServerConfig holds configuration for a TCP relay server.
type RelayServerConfig struct {
	Address   string   // Hostname or IP address of the relay server
	Port      uint16   // TCP port of the relay server
	PublicKey [32]byte // 32-byte public key of the relay server
	Priority  int      // Order in which relay servers are tried (lower = higher priority)
}

// ToRelayServerInfo converts a RelayServerConfig to transport.RelayServerInfo.
func (c *RelayServerConfig) ToRelayServerInfo() transport.RelayServerInfo {
	return transport.RelayServerInfo{
		Address:   c.Address,
		Port:      c.Port,
		PublicKey: c.PublicKey,
		Priority:  c.Priority,
	}
}

// ProxyOptions contains proxy configuration for TCP connections.
//
// For SOCKS5 proxies, UDP traffic can also be proxied using the SOCKS5 UDP
// ASSOCIATE command (RFC 1928) by setting UDPProxyEnabled to true.
//
// NOTE: When UDPProxyEnabled is false (the default), UDP traffic bypasses the
// proxy and is sent directly, potentially leaking your real IP address.
//
// For complete proxy coverage (e.g., Tor anonymity), either:
//   - Enable UDP proxying (set UDPProxyEnabled = true) for SOCKS5 proxies
//   - Disable UDP (set Options.UDPEnabled = false) to force TCP-only mode
//   - Use system-level proxy routing (iptables, proxychains, or network namespaces)
//
// When UDPEnabled is true, UDPProxyEnabled is false, and a proxy is configured,
// a warning will be logged to alert you that UDP traffic is not being proxied.
type ProxyOptions struct {
	Type            ProxyType
	Host            string
	Port            uint16
	Username        string
	Password        string
	UDPProxyEnabled bool // Enable SOCKS5 UDP ASSOCIATE for UDP traffic (SOCKS5 only)
}

// ProxyType specifies the type of proxy to use.
type ProxyType uint8

// Proxy type constants define the supported proxy protocols.
const (
	// ProxyTypeNone indicates no proxy should be used.
	ProxyTypeNone ProxyType = iota
	// ProxyTypeHTTP indicates an HTTP CONNECT proxy.
	ProxyTypeHTTP
	// ProxyTypeSOCKS5 indicates a SOCKS5 proxy.
	ProxyTypeSOCKS5
)

// SaveDataType specifies the type of saved data.
type SaveDataType uint8

// SaveData type constants define the format of persisted Tox instance state.
const (
	// SaveDataTypeNone indicates no saved data is provided.
	SaveDataTypeNone SaveDataType = iota
	// SaveDataTypeToxSave indicates the data is a full Tox save file.
	SaveDataTypeToxSave
	// SaveDataTypeSecretKey indicates the data is just the secret key.
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
	Nospam        [4]byte            `json:"nospam"`
}

// Snapshot format constants
const (
	// SnapshotMagic identifies binary snapshot format
	SnapshotMagic uint32 = 0x544F5853 // "TOXS"
	// SnapshotVersion is the current snapshot format version
	SnapshotVersion uint16 = 1
)

// marshal serializes the toxSaveData to a JSON byte array.
// Using JSON for simplicity and readability during development.
// Future versions could use a binary format for efficiency.

// NewOptions creates a new default Options.
//
//export ToxOptionsNew
func NewOptions() *Options {
	logrus.WithFields(logrus.Fields{
		"function": "NewOptions",
	}).Info("Creating new default options")

	options := &Options{
		UDPEnabled:          true,
		IPv6Enabled:         true,
		LocalDiscovery:      true,
		StartPort:           33445,
		EndPort:             33545,
		TCPPort:             0, // Disabled by default
		SavedataType:        SaveDataTypeNone,
		ThreadsEnabled:      true,
		BootstrapTimeout:    30 * time.Second, // Increased from 5s for reliability on slow/congested networks
		MinBootstrapNodes:   4,                // Default: require 4 nodes for production use
		AsyncStorageEnabled: true,             // Default: participate as storage node for async messaging
		DeliveryRetryConfig: DefaultDeliveryRetryConfig(),
	}

	logrus.WithFields(logrus.Fields{
		"udp_enabled":           options.UDPEnabled,
		"ipv6_enabled":          options.IPv6Enabled,
		"local_discovery":       options.LocalDiscovery,
		"start_port":            options.StartPort,
		"end_port":              options.EndPort,
		"tcp_port":              options.TCPPort,
		"savedata_type":         options.SavedataType,
		"threads_enabled":       options.ThreadsEnabled,
		"bootstrap_timeout":     options.BootstrapTimeout,
		"async_storage_enabled": options.AsyncStorageEnabled,
	}).Info("Default options created successfully")

	return options
}

// NewOptionsForTesting creates Options optimized for testing environments.
// This includes relaxed bootstrap requirements and other testing-friendly settings.
//
//export ToxOptionsNewForTesting
func NewOptionsForTesting() *Options {
	logrus.WithFields(logrus.Fields{
		"function": "NewOptionsForTesting",
	}).Info("Creating new testing options")

	options := NewOptions()

	// Adjust settings for testing
	options.MinBootstrapNodes = 1       // Allow bootstrap with just 1 node for testing
	options.IPv6Enabled = false         // Simplify networking for localhost testing
	options.LocalDiscovery = false      // Disable local discovery for controlled testing
	options.AsyncStorageEnabled = false // Disable async storage to avoid using real data dir

	logrus.WithFields(logrus.Fields{
		"min_bootstrap_nodes":   options.MinBootstrapNodes,
		"ipv6_enabled":          options.IPv6Enabled,
		"local_discovery":       options.LocalDiscovery,
		"async_storage_enabled": options.AsyncStorageEnabled,
	}).Info("Testing options created successfully")

	return options
}

// Tox represents a Tox instance.
//
//export Tox
type Tox struct {
	// Core components
	options          *Options
	keyPair          *crypto.KeyPair
	dht              *dht.RoutingTable
	dhtMutex         sync.RWMutex // Protects dht pointer access
	selfAddress      net.Addr
	udpTransport     transport.Transport
	tcpTransport     transport.Transport
	bootstrapManager *dht.BootstrapManager

	// Packet delivery implementation (can be real or simulation)
	packetDelivery  interfaces.IPacketDelivery
	deliveryMu      sync.RWMutex // Protects packetDelivery pointer access
	deliveryFactory *factory.PacketDeliveryFactory

	// State
	connectionStatus ConnectionStatus
	running          bool
	iterationTime    time.Duration

	// Time provider for deterministic testing (defaults to RealTimeProvider)
	timeProvider TimeProvider

	// Self information
	selfName      string
	selfStatusMsg string
	nospam        [4]byte // Nospam value for ToxID generation
	selfMutex     sync.RWMutex

	// Friend-related fields - uses sharded storage for reduced mutex contention at scale
	friends              *friend.FriendStore[Friend]
	messageManager       *messaging.MessageManager
	messageManagerMu     sync.RWMutex // Protects messageManager pointer access
	pendingFriendReqs    []*pendingFriendRequest
	pendingFriendReqsMux sync.Mutex
	requestManager       *friend.RequestManager // Centralized friend request management

	// File transfers
	fileTransfers map[uint64]*file.Transfer // Key: (friendID << 32) | fileID
	transfersMu   sync.RWMutex
	fileManager   *file.Manager // Centralized file transfer management with transport integration

	// Conferences (simple group chats)
	conferences      map[uint32]*group.Chat
	conferencesMu    sync.RWMutex
	nextConferenceID uint32

	// Async messaging
	asyncManager *async.AsyncManager

	// LAN discovery
	lanDiscovery *dht.LANDiscovery

	// Advanced NAT traversal with relay support for symmetric NAT scenarios
	natTraversal *transport.AdvancedNATTraversal

	// Callbacks
	friendRequestCallback          FriendRequestCallback
	friendMessageCallback          FriendMessageCallback
	simpleFriendMessageCallback    SimpleFriendMessageCallback
	friendStatusCallback           FriendStatusCallback
	connectionStatusCallback       ConnectionStatusCallback
	friendConnectionStatusCallback FriendConnectionStatusCallback
	friendStatusChangeCallback     FriendStatusChangeCallback

	// File transfer callbacks
	fileRecvCallback            func(friendID, fileID, kind uint32, fileSize uint64, filename string)
	fileRecvChunkCallback       func(friendID, fileID uint32, position uint64, data []byte)
	fileChunkRequestCallback    func(friendID, fileID uint32, position uint64, length int)
	friendNameCallback          func(friendID uint32, name string)
	friendStatusMessageCallback func(friendID uint32, statusMessage string)
	friendTypingCallback        func(friendID uint32, isTyping bool)
	friendDeletedCallback       func(friendID uint32) // Called when a friend is deleted

	// Callback mutex for thread safety
	callbackMu sync.RWMutex

	// Context for clean shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Monotonic counter incremented on every Iterate() call.
	// Used to rate-limit periodic maintenance operations.
	iterationCount uint64

	// Message ID tracking for message delivery confirmation
	lastMessageID uint32
	messageIDMu   sync.Mutex
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
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()

	// Create a serializable representation of the Tox state
	saveData := toxSaveData{
		KeyPair:       t.keyPair,
		Friends:       make(map[uint32]*Friend),
		Options:       t.options,
		SelfName:      t.selfName,
		SelfStatusMsg: t.selfStatusMsg,
		Nospam:        t.nospam,
	}

	// Copy friends data using sharded store's GetAll
	// GetAll returns a consistent snapshot
	for id, f := range t.friends.GetAll() {
		saveData.Friends[id] = &Friend{
			PublicKey:        f.PublicKey,
			Status:           f.Status,
			ConnectionStatus: f.ConnectionStatus,
			Name:             f.Name,
			StatusMessage:    f.StatusMessage,
			LastSeen:         f.LastSeen,
			// Note: UserData is not serialized as it may contain non-serializable types
		}
	}

	return saveData.marshal()
}

// createKeyPair creates a cryptographic key pair based on the provided options.
// It either generates a new key pair or creates one from saved data.
func createKeyPair(options *Options) (*crypto.KeyPair, error) {
	if options.SavedataType == SaveDataTypeSecretKey && len(options.SavedataData) == 32 {
		// Create from saved secret key
		var secretKey [32]byte
		copy(secretKey[:], options.SavedataData)
		return crypto.FromSecretKey(secretKey)
	}
	// Generate new key pair
	return crypto.GenerateKeyPair()
}

// getDefaultDataDir returns the default data directory for Tox storage
func getDefaultDataDir() string {
	// Try to use XDG_DATA_HOME first, then fallback to home directory
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "tox")
	}

	// Fallback to home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".local", "share", "tox")
	}

	// Last resort: current directory
	return "./tox_data"
}

// initializeToxInstance creates and initializes a Tox instance with the provided components.
func initializeToxInstance(options *Options, keyPair *crypto.KeyPair, udpTransport, tcpTransport transport.Transport, nospam [4]byte, toxID *crypto.ToxID) *Tox {
	ctx, cancel := context.WithCancel(context.Background())
	rdht := dht.NewRoutingTable(*toxID, 8)

	bootstrapManager := createBootstrapManager(options, toxID, keyPair, udpTransport, rdht)

	// Initialize async messaging only if storage is enabled
	var asyncManager *async.AsyncManager
	if options.AsyncStorageEnabled {
		asyncManager = initializeAsyncMessaging(keyPair, udpTransport)
	} else {
		logrus.WithFields(logrus.Fields{
			"function": "initializeToxInstance",
		}).Info("Async storage disabled by configuration")
	}

	packetDelivery := setupPacketDelivery(udpTransport)

	tox := createToxInstance(options, keyPair, rdht, udpTransport, tcpTransport, bootstrapManager, packetDelivery, nospam, asyncManager, ctx, cancel)

	// Set selfAddress for NAT traversal from UDP transport
	if udpTransport != nil {
		tox.selfAddress = udpTransport.LocalAddr()
	}

	startAsyncMessaging(asyncManager)
	registerPacketHandlers(udpTransport, tox)
	initializeNATTraversal(tox)

	return tox
}

// createBootstrapManager creates the appropriate bootstrap manager based on configuration.
func createBootstrapManager(options *Options, toxID *crypto.ToxID, keyPair *crypto.KeyPair, udpTransport transport.Transport, rdht *dht.RoutingTable) *dht.BootstrapManager {
	if options.MinBootstrapNodes != 4 {
		// Use testing constructor for non-standard minimum nodes
		return dht.NewBootstrapManagerForTesting(*toxID, udpTransport, rdht, options.MinBootstrapNodes)
	}
	// Use the enhanced bootstrap manager with versioned handshake support for production
	return dht.NewBootstrapManagerWithKeyPair(*toxID, keyPair, udpTransport, rdht)
}

// initializeAsyncMessaging sets up async messaging with error handling.
func initializeAsyncMessaging(keyPair *crypto.KeyPair, udpTransport transport.Transport) *async.AsyncManager {
	dataDir := getDefaultDataDir()
	asyncManager, err := async.NewAsyncManager(keyPair, udpTransport, dataDir)
	if err != nil {
		// Log error but continue - async messaging is optional
		fmt.Printf("Warning: failed to initialize async messaging: %v\n", err)
		return nil
	}
	return asyncManager
}

// startAsyncMessaging starts the async messaging system if configured.
func startAsyncMessaging(asyncManager *async.AsyncManager) {
	if asyncManager != nil {
		asyncManager.Start()
	}
}

// setupPacketDelivery initializes packet delivery system with fallback to simulation.
func setupPacketDelivery(udpTransport transport.Transport) interfaces.IPacketDelivery {
	deliveryFactory := factory.NewPacketDeliveryFactory()

	if udpTransport == nil {
		// No transport available, use simulation
		return deliveryFactory.CreateSimulationForTesting()
	}

	underlyingUDP := extractUDPTransport(udpTransport)
	if underlyingUDP == nil {
		logrus.WithFields(logrus.Fields{
			"function": "setupPacketDelivery",
		}).Warn("Unable to extract UDP transport for network adapter, using simulation")
		return deliveryFactory.CreateSimulationForTesting()
	}

	networkTransport := transport.NewNetworkTransportAdapter(underlyingUDP)
	packetDelivery, err := deliveryFactory.CreatePacketDelivery(networkTransport)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "setupPacketDelivery",
			"error":    err.Error(),
		}).Warn("Failed to create packet delivery, falling back to simulation")
		return deliveryFactory.CreateSimulationForTesting()
	}

	return packetDelivery
}

// extractUDPTransport attempts to extract the underlying UDP transport from various wrapper types.
func extractUDPTransport(udpTransport transport.Transport) *transport.UDPTransport {
	if negotiatingTransport, ok := udpTransport.(*transport.NegotiatingTransport); ok {
		if udp, ok := negotiatingTransport.GetUnderlying().(*transport.UDPTransport); ok {
			return udp
		}
	} else if udp, ok := udpTransport.(*transport.UDPTransport); ok {
		return udp
	}
	return nil
}

// createToxInstance creates and configures the main Tox instance.
func createToxInstance(options *Options, keyPair *crypto.KeyPair, rdht *dht.RoutingTable, udpTransport, tcpTransport transport.Transport, bootstrapManager *dht.BootstrapManager, packetDelivery interfaces.IPacketDelivery, nospam [4]byte, asyncManager *async.AsyncManager, ctx context.Context, cancel context.CancelFunc) *Tox {
	tox := &Tox{
		options:          options,
		keyPair:          keyPair,
		dht:              rdht,
		udpTransport:     udpTransport,
		tcpTransport:     tcpTransport,
		bootstrapManager: bootstrapManager,
		packetDelivery:   packetDelivery,
		deliveryFactory:  factory.NewPacketDeliveryFactory(),
		connectionStatus: ConnectionNone,
		running:          true,
		iterationTime:    50 * time.Millisecond,
		nospam:           nospam,
		friends:          friend.NewFriendStore[Friend](),
		fileTransfers:    make(map[uint64]*file.Transfer),
		conferences:      make(map[uint32]*group.Chat),
		nextConferenceID: 1,
		asyncManager:     asyncManager,
		ctx:              ctx,
		cancel:           cancel,
		timeProvider:     RealTimeProvider{},
	}

	initializeMessagingManagers(tox)
	initializeFileManager(tox, udpTransport)
	initializeLANDiscovery(tox, options)

	return tox
}

// initializeMessagingManagers configures the message and friend request managers.
func initializeMessagingManagers(tox *Tox) {
	tox.messageManager = messaging.NewMessageManager()
	tox.messageManager.SetTransport(tox)
	tox.messageManager.SetKeyProvider(tox)

	// Apply delivery retry configuration from options
	if tox.options.DeliveryRetryConfig != nil {
		cfg := tox.options.DeliveryRetryConfig
		tox.messageManager.SetRetryConfig(
			cfg.Enabled,
			cfg.MaxRetries,
			cfg.InitialDelay,
			cfg.MaxDelay,
			cfg.BackoffFactor,
		)
	}

	tox.requestManager = friend.NewRequestManager()
}

// initializeFileManager sets up the file transfer manager with transport integration.
func initializeFileManager(tox *Tox, udpTransport transport.Transport) {
	tox.fileManager = file.NewManager(udpTransport)
	if tox.fileManager != nil {
		tox.fileManager.SetAddressResolver(file.AddressResolverFunc(func(addr net.Addr) (uint32, error) {
			return tox.resolveFriendIDFromAddress(addr)
		}))
		// Wire file transfer callbacks from Manager to Tox
		tox.fileManager.SetFileRecvCallback(func(friendID, fileID, kind uint32, fileSize uint64, filename string) {
			tox.callbackMu.RLock()
			cb := tox.fileRecvCallback
			tox.callbackMu.RUnlock()
			if cb != nil {
				cb(friendID, fileID, kind, fileSize, filename)
			}
		})
		tox.fileManager.SetFileRecvChunkCallback(func(friendID, fileID uint32, position uint64, data []byte) {
			tox.callbackMu.RLock()
			cb := tox.fileRecvChunkCallback
			tox.callbackMu.RUnlock()
			if cb != nil {
				cb(friendID, fileID, position, data)
			}
		})
		tox.fileManager.SetFileChunkRequestCallback(func(friendID, fileID uint32, position uint64, length int) {
			tox.callbackMu.RLock()
			cb := tox.fileChunkRequestCallback
			tox.callbackMu.RUnlock()
			if cb != nil {
				cb(friendID, fileID, position, length)
			}
		})
	}
}

// New creates a new Tox instance with the specified options.
// If options is nil, default options are used.
// Returns the Tox instance or an error if initialization fails.
func New(options *Options) (*Tox, error) {
	logNewInstanceStarting()
	options = validateAndInitializeOptions(options)
	logOptionsConfiguration(options)

	keyPair, nospam, toxID, err := createToxIdentity(options)
	if err != nil {
		return nil, err
	}

	udpTransport, tcpTransport, err := setupTransports(options, keyPair)
	if err != nil {
		return nil, err
	}

	tox := assembleAndConfigureToxInstance(options, keyPair, udpTransport, tcpTransport, nospam, toxID)

	if err := tox.loadSavedState(options); err != nil {
		logrus.WithFields(logrus.Fields{"function": "New", "error": err.Error()}).Error("Failed to load saved state, cleaning up")
		tox.Kill()
		return nil, err
	}

	logToxInstanceCreated(keyPair.Public)
	return tox, nil
}

// logNewInstanceStarting logs the start of Tox instance creation.
func logNewInstanceStarting() {
	logrus.WithFields(logrus.Fields{"function": "New"}).Info("Creating new Tox instance")
}

// validateAndInitializeOptions ensures options are not nil and returns valid options.
func validateAndInitializeOptions(options *Options) *Options {
	if options == nil {
		logrus.WithFields(logrus.Fields{"function": "New"}).Info("No options provided, using defaults")
		return NewOptions()
	}
	return options
}

// logOptionsConfiguration logs the configuration options being used.
func logOptionsConfiguration(options *Options) {
	logrus.WithFields(logrus.Fields{
		"function":        "New",
		"udp_enabled":     options.UDPEnabled,
		"ipv6_enabled":    options.IPv6Enabled,
		"local_discovery": options.LocalDiscovery,
		"start_port":      options.StartPort,
		"end_port":        options.EndPort,
	}).Debug("Using options for Tox creation")
}

// createToxIdentity generates the cryptographic identity components for a Tox instance.
func createToxIdentity(options *Options) (*crypto.KeyPair, [4]byte, *crypto.ToxID, error) {
	logrus.WithFields(logrus.Fields{"function": "New"}).Debug("Creating key pair")
	keyPair, err := createKeyPair(options)
	if err != nil {
		logrus.WithFields(logrus.Fields{"function": "New", "error": err.Error()}).Error("Failed to create key pair")
		return nil, [4]byte{}, nil, err
	}
	logrus.WithFields(logrus.Fields{"function": "New", "public_key_preview": fmt.Sprintf("%x", keyPair.Public[:8])}).Debug("Key pair created successfully")

	logrus.WithFields(logrus.Fields{"function": "New"}).Debug("Generating nospam value")
	nospam, err := generateNospam()
	if err != nil {
		logrus.WithFields(logrus.Fields{"function": "New", "error": err.Error()}).Error("Failed to generate nospam value")
		return nil, [4]byte{}, nil, fmt.Errorf("nospam generation failed: %w", err)
	}

	toxID := crypto.NewToxID(keyPair.Public, nospam)
	return keyPair, nospam, toxID, nil
}

// assembleAndConfigureToxInstance creates and configures a Tox instance with provided components.
func assembleAndConfigureToxInstance(options *Options, keyPair *crypto.KeyPair, udpTransport, tcpTransport transport.Transport, nospam [4]byte, toxID *crypto.ToxID) *Tox {
	logrus.WithFields(logrus.Fields{"function": "New"}).Debug("Initializing Tox instance")
	tox := initializeToxInstance(options, keyPair, udpTransport, tcpTransport, nospam, toxID)
	tox.registerTransportHandlers(udpTransport, tcpTransport)
	return tox
}

// logToxInstanceCreated logs successful creation of a Tox instance.
func logToxInstanceCreated(publicKey [32]byte) {
	logrus.WithFields(logrus.Fields{
		"function":           "New",
		"public_key_preview": fmt.Sprintf("%x", publicKey[:8]),
	}).Info("Tox instance created successfully")
}

// NewFromSavedata creates a new Tox instance from previously saved data.
// This is a convenience function that combines New() and Load() operations.
//
// The savedata should be obtained from a previous call to GetSavedata().
// If options is nil, default options will be used.
//
// parseSavedState unmarshals and validates the savedata.
//
//export ToxNewFromSavedata
func parseSavedState(savedata []byte) (*toxSaveData, error) {
	if len(savedata) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "NewFromSavedata",
			"error":    "savedata cannot be empty",
		}).Error("Savedata validation failed")
		return nil, errors.New("savedata cannot be empty")
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewFromSavedata",
	}).Debug("Parsing savedata")

	var savedState toxSaveData
	if err := savedState.unmarshal(savedata); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewFromSavedata",
			"error":    err.Error(),
		}).Error("Failed to unmarshal savedata")
		return nil, err
	}

	if savedState.KeyPair == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewFromSavedata",
			"error":    "savedata missing key pair",
		}).Error("Savedata validation failed - missing key pair")
		return nil, errors.New("savedata missing key pair")
	}

	logrus.WithFields(logrus.Fields{
		"function":           "NewFromSavedata",
		"friends_count":      len(savedState.Friends),
		"self_name":          savedState.SelfName,
		"public_key_preview": fmt.Sprintf("%x", savedState.KeyPair.Public[:8]),
	}).Debug("Savedata parsed successfully")

	return &savedState, nil
}

// prepareOptionsWithSavedKey sets up options with the saved secret key.
func prepareOptionsWithSavedKey(options *Options, savedState *toxSaveData) *Options {
	if options == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewFromSavedata",
		}).Debug("No options provided, using defaults")
		options = NewOptions()
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewFromSavedata",
	}).Debug("Setting saved secret key in options")
	options.SavedataType = SaveDataTypeSecretKey
	options.SavedataData = savedState.KeyPair.Private[:]
	options.SavedataLength = 32

	return options
}

// NewFromSavedata creates a new Tox instance from previously saved state data.
// The savedata parameter contains the serialized state from a prior Tox.Save() call.
// Returns the restored Tox instance or an error if the savedata is invalid.
func NewFromSavedata(options *Options, savedata []byte) (*Tox, error) {
	logrus.WithFields(logrus.Fields{
		"function":        "NewFromSavedata",
		"savedata_length": len(savedata),
	}).Info("Creating Tox instance from savedata")

	savedState, err := parseSavedState(savedata)
	if err != nil {
		return nil, err
	}

	options = prepareOptionsWithSavedKey(options, savedState)

	logrus.WithFields(logrus.Fields{
		"function": "NewFromSavedata",
	}).Debug("Creating Tox instance with restored key")
	tox, err := New(options)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewFromSavedata",
			"error":    err.Error(),
		}).Error("Failed to create Tox instance with restored key")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewFromSavedata",
	}).Debug("Loading complete state")
	if err := tox.Load(savedata); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewFromSavedata",
			"error":    err.Error(),
		}).Error("Failed to load complete state, cleaning up")
		tox.Kill()
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"function":       "NewFromSavedata",
		"friends_loaded": len(savedState.Friends),
	}).Info("Tox instance created successfully from savedata")

	return tox, nil
}

// updateFriendField validates a string value, updates a friend field, and
// invokes a callback. This consolidates the common pattern used in
// receiveFriendNameUpdate and receiveFriendStatusMessageUpdate.

// handleFriendRequestPacket processes incoming friend request packets from the transport layer
func (t *Tox) handleFriendRequestPacket(packet *transport.Packet, senderAddr net.Addr) error {
	// Packet format: [SENDER_PUBLIC_KEY(32)][MESSAGE...]
	if len(packet.Data) < 32 {
		return errors.New("friend request packet too small")
	}

	var senderPublicKey [32]byte
	copy(senderPublicKey[:], packet.Data[0:32])
	message := string(packet.Data[32:])

	// Process the friend request
	t.receiveFriendRequest(senderPublicKey, message)
	return nil
}

// processIncomingPacket handles raw network packets and routes them appropriately
// This integrates with the transport layer for automatic packet processing
func (t *Tox) processIncomingPacket(packet []byte, senderAddr net.Addr) error {
	if len(packet) < 4 {
		return errors.New("packet too small")
	}

	packetType := packet[0]
	return t.routePacketByType(packetType, packet)
}

// routePacketByType routes the packet to the appropriate handler based on type.
func (t *Tox) routePacketByType(packetType byte, packet []byte) error {
	switch packetType {
	case 0x01:
		return t.processFriendMessagePacket(packet)
	case 0x02:
		return t.processFriendNameUpdatePacket(packet)
	case 0x03:
		return t.processFriendStatusMessageUpdatePacket(packet)
	case 0x04:
		return t.processFriendRequestPacket(packet)
	case 0x05:
		return t.processTypingNotificationPacket(packet)
	default:
		return fmt.Errorf("unknown packet type: 0x%02x", packetType)
	}
}

// processFriendNameUpdatePacket handles incoming friend name update packets.
func (t *Tox) processFriendNameUpdatePacket(packet []byte) error {
	if len(packet) < 5 {
		return errors.New("friend name update packet too small")
	}

	friendID := binary.BigEndian.Uint32(packet[1:5])
	name := string(packet[5:])

	t.receiveFriendNameUpdate(friendID, name)
	return nil
}

// processFriendRequestPacket handles incoming friend request packets.
func (t *Tox) processFriendRequestPacket(packet []byte) error {
	if len(packet) < 33 {
		return errors.New("friend request packet too small")
	}

	var senderPublicKey [32]byte
	copy(senderPublicKey[:], packet[1:33])
	message := string(packet[33:])

	t.receiveFriendRequest(senderPublicKey, message)
	return nil
}

// processTypingNotificationPacket handles incoming typing notification packets.
func (t *Tox) processTypingNotificationPacket(packet []byte) error {
	if len(packet) < 6 {
		return errors.New("typing notification packet too small")
	}

	friendID := binary.BigEndian.Uint32(packet[1:5])
	isTyping := packet[5] != 0

	t.receiveFriendTyping(friendID, isTyping)
	return nil
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
	IsTyping         bool
}

// FriendStatus represents the status of a friend.
type FriendStatus uint8

// Friend status constants define the user-set availability status of a friend.
const (
	// FriendStatusNone indicates no status has been set (default).
	FriendStatusNone FriendStatus = iota
	// FriendStatusAway indicates the user is away from keyboard.
	FriendStatusAway
	// FriendStatusBusy indicates the user is busy and may not respond.
	FriendStatusBusy
	// FriendStatusOnline indicates the user is online and available.
	FriendStatusOnline
)

// generateNospam creates a random nospam value.
// Returns an error if cryptographic random generation fails, indicating a serious system issue.
// Callers MUST check this error as a failed CSPRNG compromises security.
func generateNospam() ([4]byte, error) {
	nospam, err := crypto.GenerateNospam()
	if err != nil {
		return [4]byte{}, fmt.Errorf("failed to generate nospam: %w", err)
	}
	return nospam, nil
}

// SetTyping sends a typing notification to a friend.
//
//export ToxSetTyping
func (t *Tox) SetTyping(friendID uint32, isTyping bool) error {
	friend, err := t.validateFriendForTyping(friendID)
	if err != nil {
		return err
	}

	packet := buildTypingPacket(friendID, isTyping)

	friendAddr, err := t.resolveFriendAddress(friend)
	if err != nil {
		return fmt.Errorf("failed to resolve friend address: %w", err)
	}

	return t.sendTypingPacket(packet, friendAddr)
}

// validateFriendForTyping checks if friend exists and is online for typing notifications.
func (t *Tox) validateFriendForTyping(friendID uint32) (*Friend, error) {
	return t.validateFriendOnline(friendID, "friend is not online")
}

// validateFriendOnline checks if a friend exists and is connected with a custom error message.
// This helper consolidates the common friend lookup and connection check pattern.
func (t *Tox) validateFriendOnline(friendID uint32, offlineMsg string) (*Friend, error) {
	f := t.friends.Get(friendID)
	if f == nil {
		return nil, errors.New("friend not found")
	}

	if f.ConnectionStatus == ConnectionNone {
		return nil, errors.New(offlineMsg)
	}

	return f, nil
}

// buildTypingPacket constructs a typing notification packet.
func buildTypingPacket(friendID uint32, isTyping bool) []byte {
	packet := make([]byte, 6)
	packet[0] = 0x05 // Typing notification packet type
	binary.BigEndian.PutUint32(packet[1:5], friendID)
	if isTyping {
		packet[5] = 1
	} else {
		packet[5] = 0
	}
	return packet
}

// sendTypingPacket sends the typing notification through UDP transport.
func (t *Tox) sendTypingPacket(packet []byte, friendAddr net.Addr) error {
	if t.udpTransport != nil {
		transportPacket := &transport.Packet{
			PacketType: transport.PacketFriendMessage,
			Data:       packet,
		}

		if err := t.udpTransport.Send(transportPacket, friendAddr); err != nil {
			return fmt.Errorf("failed to send typing notification: %w", err)
		}
	}
	return nil
}

// findFriendByPublicKey finds a friend ID by their public key
func (t *Tox) findFriendByPublicKey(publicKey [32]byte) uint32 {
	id, _ := t.friends.FindByPublicKey(publicKey, func(f *Friend) [32]byte {
		return f.PublicKey
	})
	return id // Returns 0 if not found (which is our sentinel value)
}

// updateFriendOnlineStatus notifies the async manager and callbacks about friend status changes
func (t *Tox) updateFriendOnlineStatus(friendID uint32, online bool) {
	f := t.friends.Get(friendID)
	if f == nil {
		return
	}

	// Notify async manager
	if t.asyncManager != nil {
		t.asyncManager.SetFriendOnlineStatus(f.PublicKey, online)
	}

	// Trigger OnFriendStatusChange callback
	t.callbackMu.RLock()
	statusChangeCallback := t.friendStatusChangeCallback
	t.callbackMu.RUnlock()

	if statusChangeCallback != nil {
		statusChangeCallback(f.PublicKey, online)
	}
}

// SetFriendConnectionStatus updates a friend's connection status and notifies
// the async manager for pre-key exchange triggering.
//
// This method ensures that when a friend's connection status changes (e.g., from
// offline to online), the async manager is properly notified so it can initiate
// pre-key exchanges for forward-secure messaging.
//
// Parameters:
//   - friendID: The friend number
//   - status: The new connection status (ConnectionNone, ConnectionUDP, ConnectionTCP)
//
// Returns an error if the friend does not exist.
//
//export ToxSetFriendConnectionStatus
func (t *Tox) SetFriendConnectionStatus(friendID uint32, status ConnectionStatus) error {
	var oldStatus ConnectionStatus
	var wasOnline, willBeOnline, shouldNotify bool

	// Use atomic Update to ensure thread-safe modification
	updated := t.friends.Update(friendID, func(f *Friend) {
		oldStatus = f.ConnectionStatus
		wasOnline = f.ConnectionStatus != ConnectionNone
		willBeOnline = status != ConnectionNone
		shouldNotify = wasOnline != willBeOnline

		f.ConnectionStatus = status
		f.LastSeen = t.now()
	})

	if !updated {
		return fmt.Errorf("friend %d does not exist", friendID)
	}

	// Trigger OnFriendConnectionStatus callback if status changed
	if oldStatus != status {
		t.callbackMu.RLock()
		connStatusCallback := t.friendConnectionStatusCallback
		t.callbackMu.RUnlock()

		if connStatusCallback != nil {
			connStatusCallback(friendID, status)
		}
	}

	if shouldNotify {
		t.updateFriendOnlineStatus(friendID, willBeOnline)
	}

	return nil
}

// MessageType represents the type of a message.
type MessageType uint8

// Message type constants define how a message should be displayed.
const (
	// MessageTypeNormal indicates a regular text message.
	MessageTypeNormal MessageType = iota
	// MessageTypeAction indicates an action message (like IRC /me).
	MessageTypeAction
)

// simulatePacketDelivery simulates packet delivery for testing purposes
// DEPRECATED: This method is deprecated in favor of the new packet delivery interface.
// Use packetDelivery.DeliverPacket() instead.
// In a real implementation, this would go through the transport layer
func (t *Tox) simulatePacketDelivery(friendID uint32, packet []byte) {
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function":    "simulatePacketDelivery",
		"friend_id":   friendID,
		"packet_size": len(packet),
		"deprecated":  true,
	}).Warn("Using deprecated simulatePacketDelivery - consider migrating to packet delivery interface")

	// Use the new packet delivery interface if available
	if d := t.loadDelivery(); d != nil {
		err := d.DeliverPacket(friendID, packet)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "simulatePacketDelivery",
				"friend_id": friendID,
				"error":     err.Error(),
			}).Error("Packet delivery failed through interface")
		}
		return
	}

	// Fallback to old simulation behavior
	logrus.WithFields(logrus.Fields{
		"function":    "simulatePacketDelivery",
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Info("Simulating packet delivery (fallback)")

	// For testing purposes, we'll just process the packet directly
	// In production, this would involve actual network transmission
	logrus.WithFields(logrus.Fields{
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Debug("Processing packet directly for simulation")

	t.processIncomingPacket(packet, nil)

	logrus.WithFields(logrus.Fields{
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Debug("Packet simulation completed")
}

// FileControl represents a file transfer control action.
type FileControl uint8

// File control constants define actions for managing file transfers.
const (
	// FileControlResume resumes a paused file transfer.
	FileControlResume FileControl = iota
	// FileControlPause temporarily pauses a file transfer.
	FileControlPause
	// FileControlCancel permanently cancels a file transfer.
	FileControlCancel
)

// GetAsyncStorageStats returns statistics about the async message storage
func (t *Tox) GetAsyncStorageStats() *async.StorageStats {
	if t.asyncManager == nil {
		return nil
	}
	stats := t.asyncManager.GetStorageStats()
	return stats
}

// IsAsyncMessagingAvailable returns true if async messaging features are available.
// Returns false if async manager initialization failed during Tox instance creation.
// Applications should check this before calling async-related methods.
func (t *Tox) IsAsyncMessagingAvailable() bool {
	return t.asyncManager != nil
}

// RequestManager returns the centralized friend request manager.
// The manager tracks incoming friend requests, handles duplicate detection,
// and provides pending request enumeration for application-level handling.
// Returns nil if the manager was not initialized.
//
//export ToxRequestManager
func (t *Tox) RequestManager() *friend.RequestManager {
	return t.requestManager
}

// Packet Delivery Interface Management

// SetPacketDeliveryMode switches between simulation and real packet delivery
// SetPacketDeliveryMode switches between simulation and real packet delivery modes.
func (t *Tox) SetPacketDeliveryMode(useSimulation bool) error {
	logrus.WithFields(logrus.Fields{
		"function":       "SetPacketDeliveryMode",
		"use_simulation": useSimulation,
		"current_mode":   t.IsPacketDeliverySimulation(),
	}).Info("Switching packet delivery mode")

	if err := t.validateDeliveryFactory(); err != nil {
		return err
	}

	t.switchDeliveryFactory(useSimulation)

	newDelivery := t.createPacketDelivery(useSimulation)
	t.storeDelivery(newDelivery)

	logrus.WithFields(logrus.Fields{
		"function":   "SetPacketDeliveryMode",
		"new_mode":   t.IsPacketDeliverySimulation(),
		"successful": true,
	}).Info("Packet delivery mode switched successfully")

	return nil
}

// validateDeliveryFactory checks if the delivery factory is properly initialized.
func (t *Tox) validateDeliveryFactory() error {
	if t.deliveryFactory == nil {
		return fmt.Errorf("delivery factory not initialized")
	}
	return nil
}

// switchDeliveryFactory switches the factory mode between simulation and real delivery.
func (t *Tox) switchDeliveryFactory(useSimulation bool) {
	if useSimulation {
		t.deliveryFactory.SwitchToSimulation()
	} else {
		t.deliveryFactory.SwitchToReal()
	}
}

// createPacketDelivery creates the appropriate packet delivery based on the mode.
func (t *Tox) createPacketDelivery(useSimulation bool) interfaces.IPacketDelivery {
	if t.udpTransport != nil && !useSimulation {
		return t.createRealPacketDelivery()
	}
	return t.deliveryFactory.CreateSimulationForTesting()
}

// createRealPacketDelivery attempts to create real packet delivery with fallback to simulation.
func (t *Tox) createRealPacketDelivery() interfaces.IPacketDelivery {
	underlyingUDP := t.extractUnderlyingUDPTransport()
	if underlyingUDP == nil {
		return t.deliveryFactory.CreateSimulationForTesting()
	}

	networkTransport := transport.NewNetworkTransportAdapter(underlyingUDP)
	newDelivery, err := t.deliveryFactory.CreatePacketDelivery(networkTransport)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "createRealPacketDelivery",
			"error":    err.Error(),
		}).Error("Failed to create real packet delivery, falling back to simulation")
		return t.deliveryFactory.CreateSimulationForTesting()
	}

	return newDelivery
}

// extractUnderlyingUDPTransport extracts the underlying UDP transport from wrapper types.
func (t *Tox) extractUnderlyingUDPTransport() *transport.UDPTransport {
	if negotiatingTransport, ok := t.udpTransport.(*transport.NegotiatingTransport); ok {
		if udp, ok := negotiatingTransport.GetUnderlying().(*transport.UDPTransport); ok {
			return udp
		}
	} else if udp, ok := t.udpTransport.(*transport.UDPTransport); ok {
		return udp
	}
	return nil
}

// GetPacketDeliveryStats returns statistics about packet delivery.
// Deprecated: Use GetPacketDeliveryTypedStats() for type-safe access.
func (t *Tox) GetPacketDeliveryStats() map[string]interface{} {
	logrus.Warn("Tox.GetPacketDeliveryStats() is deprecated and will be removed in v2.0.0; use GetPacketDeliveryTypedStats() instead")
	stats := t.GetPacketDeliveryTypedStats()
	return map[string]interface{}{
		"is_simulation":      stats.IsSimulation,
		"friend_count":       stats.FriendCount,
		"packets_sent":       stats.PacketsSent,
		"packets_delivered":  stats.PacketsDelivered,
		"packets_failed":     stats.PacketsFailed,
		"bytes_sent":         stats.BytesSent,
		"average_latency_ms": stats.AverageLatencyMs,
		// Backward compatible keys for legacy code
		"total_friends":         stats.FriendCount,
		"total_deliveries":      int(stats.PacketsDelivered),
		"successful_deliveries": int(stats.PacketsDelivered),
		"failed_deliveries":     int(stats.PacketsFailed),
	}
}

// GetPacketDeliveryTypedStats returns type-safe statistics about packet delivery.
func (t *Tox) GetPacketDeliveryTypedStats() interfaces.PacketDeliveryStats {
	d := t.loadDelivery()
	if d == nil {
		return interfaces.PacketDeliveryStats{
			IsSimulation: true,
		}
	}

	return d.GetTypedStats()
}

// IsPacketDeliverySimulation returns true if currently using simulation
func (t *Tox) IsPacketDeliverySimulation() bool {
	d := t.loadDelivery()
	if d == nil {
		return true // Default to simulation if not initialized
	}
	return d.IsSimulation()
}

// SetPacketDelivery replaces the active packet delivery implementation.
//
// This method allows external consumers to inject a custom [interfaces.IPacketDelivery]
// implementation, making the abstraction a true plug-in point. Use cases include:
//   - Custom transport backends (e.g. encrypted overlays, metrics decorators)
//   - Testing with purpose-built delivery stubs
//   - Integration harnesses that route packets through custom middleware
//
// The provided delivery must not be nil. The existing delivery is replaced
// atomically; any in-flight deliveries via the old implementation may complete
// independently.
//
// Example:
//
//	type loggingDelivery struct { inner interfaces.IPacketDelivery }
//	// … implement IPacketDelivery forwarding to inner …
//	if err := tox.SetPacketDelivery(&loggingDelivery{inner: tox.GetPacketDelivery()}); err != nil {
//	    log.Fatalf("inject delivery: %v", err)
//	}
func (t *Tox) SetPacketDelivery(delivery interfaces.IPacketDelivery) error {
	if delivery == nil {
		return fmt.Errorf("packet delivery cannot be nil")
	}
	logrus.WithFields(logrus.Fields{
		"function":   "SetPacketDelivery",
		"is_sim_old": t.IsPacketDeliverySimulation(),
		"is_sim_new": delivery.IsSimulation(),
	}).Info("Installing custom packet delivery implementation")
	t.storeDelivery(delivery)
	return nil
}

// GetPacketDelivery returns the active packet delivery implementation.
//
// The returned value is the live implementation; callers should not cache it
// across calls to [SetPacketDelivery] or [SetPacketDeliveryMode].
func (t *Tox) GetPacketDelivery() interfaces.IPacketDelivery {
	return t.loadDelivery()
}

// loadDelivery returns the current packet delivery implementation under a read lock.
func (t *Tox) loadDelivery() interfaces.IPacketDelivery {
	t.deliveryMu.RLock()
	defer t.deliveryMu.RUnlock()
	return t.packetDelivery
}

// storeDelivery replaces the packet delivery implementation under a write lock.
func (t *Tox) storeDelivery(d interfaces.IPacketDelivery) {
	t.deliveryMu.Lock()
	defer t.deliveryMu.Unlock()
	t.packetDelivery = d
}

// GetAsyncStorageCapacity returns the current storage capacity for async messages
func (t *Tox) GetAsyncStorageCapacity() int {
	if t.asyncManager == nil {
		return 0
	}
	return t.asyncManager.GetStorageStats().StorageCapacity
}

// GetAsyncStorageUtilization returns the current storage utilization as a percentage
func (t *Tox) GetAsyncStorageUtilization() float64 {
	stats := t.GetAsyncStorageStats()
	if stats == nil || stats.StorageCapacity == 0 {
		return 0.0
	}
	return float64(stats.TotalMessages) / float64(stats.StorageCapacity) * 100.0
}

// Security Status APIs

// EncryptionStatus represents the encryption status of a friend connection
type EncryptionStatus string

// Encryption status constants indicate the security level of a friend connection.
const (
	// EncryptionNoiseIK indicates the connection uses Noise-IK protocol handshake.
	EncryptionNoiseIK EncryptionStatus = "noise-ik"
	// EncryptionLegacy indicates the connection uses legacy Tox encryption.
	EncryptionLegacy EncryptionStatus = "legacy"
	// EncryptionForwardSecure indicates the connection has forward secrecy enabled.
	EncryptionForwardSecure EncryptionStatus = "forward-secure"
	// EncryptionOffline indicates the friend is offline (async messaging mode).
	EncryptionOffline EncryptionStatus = "offline"
	// EncryptionUnknown indicates the encryption status cannot be determined.
	EncryptionUnknown EncryptionStatus = "unknown"
)

// TransportSecurityInfo provides information about the transport layer security
type TransportSecurityInfo struct {
	TransportType         string   `json:"transport_type"`
	NoiseIKEnabled        bool     `json:"noise_ik_enabled"`
	LegacyFallbackEnabled bool     `json:"legacy_fallback_enabled"`
	ActiveSessions        int      `json:"active_sessions"`
	SupportedVersions     []string `json:"supported_versions"`
}

// GetTransportSecurityInfo returns detailed information about transport security
//
//export ToxGetTransportSecurityInfo
func (t *Tox) GetTransportSecurityInfo() *TransportSecurityInfo {
	info := &TransportSecurityInfo{
		TransportType:         "unknown",
		NoiseIKEnabled:        false,
		LegacyFallbackEnabled: false,
		ActiveSessions:        0,
		SupportedVersions:     []string{},
	}

	if t.udpTransport == nil {
		return info
	}

	// Check if we have negotiating transport (secure-by-default)
	if negotiatingTransport, ok := t.udpTransport.(*transport.NegotiatingTransport); ok {
		info.TransportType = "negotiating"
		info.NoiseIKEnabled = true
		info.LegacyFallbackEnabled = true // Default capability includes fallback
		info.SupportedVersions = []string{"legacy", "noise-ik"}

		// Get underlying transport info
		if underlying := negotiatingTransport.GetUnderlying(); underlying != nil {
			if _, ok := underlying.(*transport.UDPTransport); ok {
				info.TransportType = "negotiating-udp"
			}
		}
	} else if _, ok := t.udpTransport.(*transport.UDPTransport); ok {
		info.TransportType = "udp"
		info.SupportedVersions = []string{"legacy"}
	}

	return info
}

// GetSecuritySummary returns a human-readable summary of the security status
//
//export ToxGetSecuritySummary
func (t *Tox) GetSecuritySummary() string {
	info := t.GetTransportSecurityInfo()

	if info.NoiseIKEnabled {
		return "Secure: Noise-IK encryption enabled with legacy fallback"
	} else {
		return "Basic: Legacy encryption only (consider enabling secure transport)"
	}
}
