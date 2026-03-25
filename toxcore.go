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
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
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

	// Relay configuration for symmetric NAT fallback
	RelayServers []RelayServerConfig // List of TCP relay servers
	RelayEnabled bool                // Enable relay fallback for failed connections

	// Async messaging configuration
	AsyncStorageEnabled bool // Enable storage node participation (default: true). When disabled, async messaging still works for sending but this node won't store messages for others.

	// Testing configuration
	MinBootstrapNodes int // Minimum nodes required for bootstrap (default: 4, testing: 1)
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

// marshalBinary serializes the toxSaveData to a binary format for faster recovery.
// Format: [4B magic][2B version][2B flags][8B timestamp][32B pubkey][32B secretkey]
//
//	[4B nospam][2B name_len][name][2B status_len][status][4B friends_count][friends...]
func (s *toxSaveData) marshalBinary() ([]byte, error) {
	// Calculate size (approximate, will grow buffer if needed)
	estimatedSize := 4 + 2 + 2 + 8 + 32 + 32 + 4 + 2 + len(s.SelfName) + 2 + len(s.SelfStatusMsg) + 4
	for _, f := range s.Friends {
		estimatedSize += 32 + 1 + 1 + 2 + len(f.Name) + 2 + len(f.StatusMessage) + 8 + 4
	}
	buf := make([]byte, 0, estimatedSize)

	// Header
	buf = binary.BigEndian.AppendUint32(buf, SnapshotMagic)
	buf = binary.BigEndian.AppendUint16(buf, SnapshotVersion)
	buf = binary.BigEndian.AppendUint16(buf, 0) // flags (reserved)
	buf = binary.BigEndian.AppendUint64(buf, uint64(time.Now().UnixNano()))

	// KeyPair
	if s.KeyPair != nil {
		buf = append(buf, s.KeyPair.Public[:]...)
		buf = append(buf, s.KeyPair.Private[:]...)
	} else {
		buf = append(buf, make([]byte, 64)...)
	}

	// Nospam
	buf = append(buf, s.Nospam[:]...)

	// Self info
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(s.SelfName)))
	buf = append(buf, []byte(s.SelfName)...)
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(s.SelfStatusMsg)))
	buf = append(buf, []byte(s.SelfStatusMsg)...)

	// Friends
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(s.Friends)))
	for friendID, f := range s.Friends {
		buf = binary.BigEndian.AppendUint32(buf, friendID)
		buf = append(buf, f.PublicKey[:]...)
		buf = append(buf, byte(f.Status))
		buf = append(buf, byte(f.ConnectionStatus))
		buf = binary.BigEndian.AppendUint16(buf, uint16(len(f.Name)))
		buf = append(buf, []byte(f.Name)...)
		buf = binary.BigEndian.AppendUint16(buf, uint16(len(f.StatusMessage)))
		buf = append(buf, []byte(f.StatusMessage)...)
		buf = binary.BigEndian.AppendUint64(buf, uint64(f.LastSeen.UnixNano()))
	}

	return buf, nil
}

// snapshotReader wraps binary data and an offset for sequential reading.
type snapshotReader struct {
	data   []byte
	offset int
}

// remaining returns the number of unread bytes.
func (r *snapshotReader) remaining() int {
	return len(r.data) - r.offset
}

// ensureBytes checks that at least n bytes remain in the data.
func (r *snapshotReader) ensureBytes(n int, context string) error {
	if len(r.data) < r.offset+n {
		return fmt.Errorf("snapshot truncated at %s", context)
	}
	return nil
}

// readUint16 reads a big-endian uint16 and advances the offset.
func (r *snapshotReader) readUint16(context string) (uint16, error) {
	if err := r.ensureBytes(2, context); err != nil {
		return 0, err
	}
	v := binary.BigEndian.Uint16(r.data[r.offset:])
	r.offset += 2
	return v, nil
}

// readUint32 reads a big-endian uint32 and advances the offset.
func (r *snapshotReader) readUint32(context string) (uint32, error) {
	if err := r.ensureBytes(4, context); err != nil {
		return 0, err
	}
	v := binary.BigEndian.Uint32(r.data[r.offset:])
	r.offset += 4
	return v, nil
}

// readBytes reads exactly n bytes and advances the offset.
func (r *snapshotReader) readBytes(n int, context string) ([]byte, error) {
	if err := r.ensureBytes(n, context); err != nil {
		return nil, err
	}
	b := r.data[r.offset : r.offset+n]
	r.offset += n
	return b, nil
}

// readLengthPrefixedString reads a uint16 length followed by that many bytes as a string.
func (r *snapshotReader) readLengthPrefixedString(context string) (string, error) {
	length, err := r.readUint16(context + " length")
	if err != nil {
		return "", err
	}
	b, err := r.readBytes(int(length), context)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// skip advances the offset by n bytes without returning data.
func (r *snapshotReader) skip(n int) {
	r.offset += n
}

// unmarshalBinary deserializes binary snapshot data into toxSaveData.
func (s *toxSaveData) unmarshalBinary(data []byte) error {
	if len(data) < 86 { // Minimum header size
		return errors.New("snapshot data too short")
	}

	r := &snapshotReader{data: data}

	if err := s.unmarshalHeader(r); err != nil {
		return err
	}
	if err := s.unmarshalKeyPair(r); err != nil {
		return err
	}
	if err := s.unmarshalSelfInfo(r); err != nil {
		return err
	}
	return s.unmarshalFriends(r)
}

// unmarshalHeader validates the snapshot magic, version, and skips flags/timestamp.
func (s *toxSaveData) unmarshalHeader(r *snapshotReader) error {
	magic, err := r.readUint32("magic")
	if err != nil {
		return err
	}
	if magic != SnapshotMagic {
		return errors.New("invalid snapshot magic")
	}

	version, err := r.readUint16("version")
	if err != nil {
		return err
	}
	if version > SnapshotVersion {
		return fmt.Errorf("unsupported snapshot version %d", version)
	}

	if _, err := r.readBytes(2, "flags"); err != nil {
		return err
	}
	if _, err := r.readBytes(8, "timestamp"); err != nil {
		return err
	}
	return nil
}

// unmarshalKeyPair reads the public and private keys from the snapshot.
func (s *toxSaveData) unmarshalKeyPair(r *snapshotReader) error {
	keyData, err := r.readBytes(64, "keypair")
	if err != nil {
		return err
	}
	var pubKey, secKey [32]byte
	copy(pubKey[:], keyData[:32])
	copy(secKey[:], keyData[32:64])

	var zeroKey [32]byte
	if pubKey != zeroKey {
		s.KeyPair = &crypto.KeyPair{
			Public:  pubKey,
			Private: secKey,
		}
	}
	return nil
}

// unmarshalSelfInfo reads nospam, self name, and status message.
func (s *toxSaveData) unmarshalSelfInfo(r *snapshotReader) error {
	nospamData, err := r.readBytes(4, "nospam")
	if err != nil {
		return err
	}
	copy(s.Nospam[:], nospamData)

	s.SelfName, err = r.readLengthPrefixedString("self name")
	if err != nil {
		return err
	}
	s.SelfStatusMsg, err = r.readLengthPrefixedString("status message")
	return err
}

// unmarshalFriends reads the friends list from the snapshot.
func (s *toxSaveData) unmarshalFriends(r *snapshotReader) error {
	friendsCount, err := r.readUint32("friends count")
	if err != nil {
		return err
	}

	// Minimum bytes per friend entry: 4 (ID) + 32 (PK) + 2 (status) + 2 (name len) + 2 (status len) + 8 (last seen) = 50
	const minFriendEntrySize = 50
	maxPossible := r.remaining() / minFriendEntrySize
	if int(friendsCount) > maxPossible {
		return fmt.Errorf("friends count %d exceeds maximum possible from remaining data (%d bytes)", friendsCount, r.remaining())
	}

	s.Friends = make(map[uint32]*Friend, friendsCount)
	for i := 0; i < int(friendsCount); i++ {
		friendID, f, err := unmarshalFriendEntry(r)
		if err != nil {
			return err
		}
		s.Friends[friendID] = f
	}
	return nil
}

// unmarshalFriendEntry reads a single friend entry from the snapshot.
func unmarshalFriendEntry(r *snapshotReader) (uint32, *Friend, error) {
	friendID, err := r.readUint32("friend entry")
	if err != nil {
		return 0, nil, err
	}

	pkData, err := r.readBytes(32, "friend public key")
	if err != nil {
		return 0, nil, err
	}
	var pk [32]byte
	copy(pk[:], pkData)

	statusData, err := r.readBytes(2, "friend status")
	if err != nil {
		return 0, nil, err
	}
	status := FriendStatus(statusData[0])
	connStatus := ConnectionStatus(statusData[1])

	fName, err := r.readLengthPrefixedString("friend name")
	if err != nil {
		return 0, nil, err
	}
	fStatus, err := r.readLengthPrefixedString("friend status message")
	if err != nil {
		return 0, nil, err
	}

	lastSeenData, err := r.readBytes(8, "friend last seen")
	if err != nil {
		return 0, nil, err
	}
	lastSeenNano := int64(binary.BigEndian.Uint64(lastSeenData))

	return friendID, &Friend{
		PublicKey:        pk,
		Status:           status,
		ConnectionStatus: connStatus,
		Name:             fName,
		StatusMessage:    fStatus,
		LastSeen:         time.Unix(0, lastSeenNano),
	}, nil
}

// isSnapshotFormat checks if data is in binary snapshot format.
func isSnapshotFormat(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return binary.BigEndian.Uint32(data[:4]) == SnapshotMagic
}

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
	options.MinBootstrapNodes = 1  // Allow bootstrap with just 1 node for testing
	options.IPv6Enabled = false    // Simplify networking for localhost testing
	options.LocalDiscovery = false // Disable local discovery for controlled testing

	logrus.WithFields(logrus.Fields{
		"min_bootstrap_nodes": options.MinBootstrapNodes,
		"ipv6_enabled":        options.IPv6Enabled,
		"local_discovery":     options.LocalDiscovery,
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

// setupUDPTransport configures UDP transport with secure-by-default Noise-IK encryption.
// Returns a NegotiatingTransport that automatically handles protocol version negotiation.
//
// WARNING: UDP traffic bypasses configured proxies. The ProxyTransport only wraps
// TCP-style connections. If proxy anonymity is required, disable UDP by setting
// Options.UDPEnabled = false to force TCP-only operation.
func setupUDPTransport(options *Options, keyPair *crypto.KeyPair) (transport.Transport, error) {
	if !options.UDPEnabled {
		return nil, nil
	}

	// Warn if proxy is configured but UDP is enabled - UDP bypasses proxy
	if options.Proxy != nil && options.Proxy.Type != ProxyTypeNone {
		logrus.WithFields(logrus.Fields{
			"function":   "setupUDPTransport",
			"proxy_type": options.Proxy.Type,
			"warning":    "UDP_BYPASSES_PROXY",
		}).Warn("Proxy configured but UDP is enabled. UDP traffic will NOT be proxied " +
			"and may leak your real IP address. For full proxy coverage, disable UDP " +
			"(set UDPEnabled=false) or use system-level proxy routing.")
	}

	// Try ports in the range [StartPort, EndPort]
	for port := options.StartPort; port <= options.EndPort; port++ {
		addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(int(port)))
		udpTransport, err := transport.NewUDPTransport(addr)
		if err == nil {
			// Enable secure-by-default behavior by wrapping with NegotiatingTransport
			capabilities := transport.DefaultProtocolCapabilities()
			negotiatingTransport, err := transport.NewNegotiatingTransport(udpTransport, capabilities, keyPair.Private[:])
			if err != nil {
				// If secure transport setup fails, log warning but continue with UDP
				// This ensures backward compatibility while preferring security
				logrus.WithFields(logrus.Fields{
					"function": "setupUDPTransport",
					"error":    err.Error(),
					"port":     port,
				}).Warn("Failed to enable Noise-IK transport, falling back to legacy UDP")
				return wrapWithProxyIfConfigured(udpTransport, options.Proxy)
			}

			logrus.WithFields(logrus.Fields{
				"function": "setupUDPTransport",
				"port":     port,
				"security": "noise-ik_enabled",
			}).Info("Secure transport initialized with Noise-IK capability")

			return wrapWithProxyIfConfigured(negotiatingTransport, options.Proxy)
		}
	}

	return nil, errors.New("failed to bind to any UDP port")
}

// setupTCPTransport configures TCP transport with secure-by-default Noise-IK encryption.
// Returns a NegotiatingTransport that automatically handles protocol version negotiation.
// If proxy options are configured, wraps the transport with ProxyTransport.
func setupTCPTransport(options *Options, keyPair *crypto.KeyPair) (transport.Transport, error) {
	if options.TCPPort == 0 {
		return nil, nil
	}

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(int(options.TCPPort)))
	tcpTransport, err := transport.NewTCPTransport(addr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "setupTCPTransport",
			"error":    err.Error(),
			"port":     options.TCPPort,
		}).Error("Failed to create TCP transport")
		return nil, err
	}

	// Enable secure-by-default behavior by wrapping with NegotiatingTransport
	capabilities := transport.DefaultProtocolCapabilities()
	negotiatingTransport, err := transport.NewNegotiatingTransport(tcpTransport, capabilities, keyPair.Private[:])
	if err != nil {
		// If secure transport setup fails, log warning but continue with TCP
		// This ensures backward compatibility while preferring security
		logrus.WithFields(logrus.Fields{
			"function": "setupTCPTransport",
			"error":    err.Error(),
			"port":     options.TCPPort,
		}).Warn("Failed to enable Noise-IK transport, falling back to legacy TCP")
		return wrapWithProxyIfConfigured(tcpTransport, options.Proxy)
	}

	logrus.WithFields(logrus.Fields{
		"function": "setupTCPTransport",
		"port":     options.TCPPort,
		"security": "noise-ik_enabled",
	}).Info("Secure TCP transport initialized with Noise-IK capability")

	return wrapWithProxyIfConfigured(negotiatingTransport, options.Proxy)
}

// wrapWithProxyIfConfigured wraps a transport with proxy if proxy options are configured.
// Returns the original transport if no proxy is configured or an error occurs.
func wrapWithProxyIfConfigured(t transport.Transport, proxyOpts *ProxyOptions) (transport.Transport, error) {
	if proxyOpts == nil || proxyOpts.Type == ProxyTypeNone {
		return t, nil
	}

	var proxyType string
	switch proxyOpts.Type {
	case ProxyTypeHTTP:
		proxyType = "http"
	case ProxyTypeSOCKS5:
		proxyType = "socks5"
	default:
		logrus.WithFields(logrus.Fields{
			"function":   "wrapWithProxyIfConfigured",
			"proxy_type": proxyOpts.Type,
		}).Warn("Unknown proxy type, skipping proxy configuration")
		return t, nil
	}

	proxyConfig := &transport.ProxyConfig{
		Type:            proxyType,
		Host:            proxyOpts.Host,
		Port:            proxyOpts.Port,
		Username:        proxyOpts.Username,
		Password:        proxyOpts.Password,
		UDPProxyEnabled: proxyOpts.UDPProxyEnabled,
	}

	proxyTransport, err := transport.NewProxyTransport(t, proxyConfig)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "wrapWithProxyIfConfigured",
			"proxy_type": proxyType,
			"error":      err.Error(),
		}).Warn("Failed to create proxy transport, continuing without proxy")
		return t, nil
	}

	logrus.WithFields(logrus.Fields{
		"function":   "wrapWithProxyIfConfigured",
		"proxy_type": proxyType,
		"proxy_addr": fmt.Sprintf("%s:%d", proxyOpts.Host, proxyOpts.Port),
	}).Info("Proxy transport configured successfully")

	return proxyTransport, nil
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

// initializeLANDiscovery sets up local network peer discovery if enabled in options.
func initializeLANDiscovery(tox *Tox, options *Options) {
	if !options.LocalDiscovery {
		return
	}

	port := determineLANDiscoveryPort(options)
	tox.lanDiscovery = dht.NewLANDiscovery(tox.keyPair.Public, port)

	configureLANDiscoveryCallback(tox)
	startLANDiscovery(tox)
}

// determineLANDiscoveryPort returns the port to use for LAN discovery.
func determineLANDiscoveryPort(options *Options) uint16 {
	if options.StartPort == 0 {
		return 33445
	}
	return options.StartPort
}

// configureLANDiscoveryCallback sets up the peer discovery callback handler.
func configureLANDiscoveryCallback(tox *Tox) {
	tox.lanDiscovery.OnPeer(func(publicKey [32]byte, addr net.Addr) {
		toxID := crypto.ToxID{PublicKey: publicKey}
		node := dht.NewNode(toxID, addr)
		logrus.WithFields(logrus.Fields{
			"peer_addr":  addr.String(),
			"public_key": fmt.Sprintf("%x", publicKey[:8]),
		}).Info("Adding LAN-discovered peer to DHT")
		tox.dht.AddNode(node)
	})
}

// startLANDiscovery attempts to start LAN discovery with graceful fallback on failure.
func startLANDiscovery(tox *Tox) {
	if err := tox.lanDiscovery.Start(); err != nil {
		logrus.WithError(err).Debug("Failed to start LAN discovery, continuing without it")
		tox.lanDiscovery = nil
	} else {
		logrus.Info("LAN discovery started successfully")
	}
}

// initializeNATTraversal sets up advanced NAT traversal with relay support.
func initializeNATTraversal(tox *Tox) {
	if tox.selfAddress == nil {
		return
	}

	ant, err := transport.NewAdvancedNATTraversalWithKey(tox.selfAddress, tox.keyPair.Public)
	if err != nil {
		logrus.WithError(err).Debug("Failed to initialize NAT traversal, continuing without it")
		return
	}

	tox.natTraversal = ant

	// Configure relay servers from options
	if tox.options != nil {
		for _, server := range tox.options.RelayServers {
			ant.AddRelayServer(server.ToRelayServerInfo())
		}

		// Enable relay fallback if configured
		if tox.options.RelayEnabled {
			ant.EnableMethod(transport.ConnectionRelay, true)
		}
	}

	logrus.WithFields(logrus.Fields{
		"relay_servers": len(tox.options.RelayServers),
		"relay_enabled": tox.options != nil && tox.options.RelayEnabled,
	}).Info("Advanced NAT traversal initialized with relay support")
}

// startAsyncMessaging starts the async messaging service if available.
func startAsyncMessaging(asyncManager *async.AsyncManager) {
	if asyncManager != nil {
		asyncManager.Start()
	}
}

// registerPacketHandlers registers packet handlers for network integration.
func registerPacketHandlers(udpTransport transport.Transport, tox *Tox) {
	if udpTransport != nil {
		udpTransport.RegisterHandler(transport.PacketFriendMessage, tox.handleFriendMessagePacket)
		udpTransport.RegisterHandler(transport.PacketFriendRequest, tox.handleFriendRequestPacket)
	}
}

// New creates a new Tox instance with the given options.
//
// setupTransports initializes and configures UDP and TCP transports based on options.
// It returns the configured transports or an error if setup fails.
//
//export ToxNew
func setupTransports(options *Options, keyPair *crypto.KeyPair) (transport.Transport, transport.Transport, error) {
	logrus.WithFields(logrus.Fields{
		"function":    "setupTransports",
		"udp_enabled": options.UDPEnabled,
	}).Debug("Setting up UDP transport")

	udpTransport, err := setupUDPTransport(options, keyPair)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "setupTransports",
			"error":    err.Error(),
		}).Error("Failed to setup UDP transport")
		return nil, nil, err
	}
	if udpTransport != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "setupTransports",
			"local_addr": udpTransport.LocalAddr().String(),
		}).Debug("UDP transport setup successfully")
	}

	logrus.WithFields(logrus.Fields{
		"function": "setupTransports",
		"tcp_port": options.TCPPort,
	}).Debug("Setting up TCP transport")

	tcpTransport, err := setupTCPTransport(options, keyPair)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "setupTransports",
			"error":    err.Error(),
		}).Error("Failed to setup TCP transport")
		return nil, nil, err
	}
	if tcpTransport != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "setupTransports",
			"local_addr": tcpTransport.LocalAddr().String(),
		}).Debug("TCP transport setup successfully")
	}

	return udpTransport, tcpTransport, nil
}

// registerTransportHandlers registers packet handlers for the configured transports.
func (t *Tox) registerTransportHandlers(udpTransport, tcpTransport transport.Transport) {
	if udpTransport != nil {
		logrus.WithFields(logrus.Fields{
			"function": "registerTransportHandlers",
		}).Debug("Registering UDP handlers")
		t.registerUDPHandlers()
	}

	if tcpTransport != nil {
		logrus.WithFields(logrus.Fields{
			"function": "registerTransportHandlers",
		}).Debug("Registering TCP handlers")
		t.registerTCPHandlers()
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

// registerUDPHandlers sets up packet handlers for the UDP transport.
func (t *Tox) registerUDPHandlers() {
	t.udpTransport.RegisterHandler(transport.PacketPingRequest, t.handlePingRequest)
	t.udpTransport.RegisterHandler(transport.PacketPingResponse, t.handlePingResponse)
	t.udpTransport.RegisterHandler(transport.PacketGetNodes, t.handleGetNodes)
	t.udpTransport.RegisterHandler(transport.PacketSendNodes, t.handleSendNodes)
	// Register more handlers here
}

// registerTCPHandlers registers packet handlers for TCP transport.
func (t *Tox) registerTCPHandlers() {
	t.tcpTransport.RegisterHandler(transport.PacketPingRequest, t.handlePingRequest)
	t.tcpTransport.RegisterHandler(transport.PacketPingResponse, t.handlePingResponse)
	t.tcpTransport.RegisterHandler(transport.PacketGetNodes, t.handleGetNodes)
	t.tcpTransport.RegisterHandler(transport.PacketSendNodes, t.handleSendNodes)
	// Register more handlers here
}

// handlePingRequest processes ping request packets.
func (t *Tox) handlePingRequest(packet *transport.Packet, addr net.Addr) error {
	// Delegate to the bootstrap manager which has the full implementation
	return t.bootstrapManager.HandlePacket(packet, addr)
}

// handlePingResponse processes ping response packets.
func (t *Tox) handlePingResponse(packet *transport.Packet, addr net.Addr) error {
	// Delegate to the bootstrap manager which has the full implementation
	return t.bootstrapManager.HandlePacket(packet, addr)
}

// handleGetNodes processes get nodes request packets.
func (t *Tox) handleGetNodes(packet *transport.Packet, addr net.Addr) error {
	// Delegate to the bootstrap manager which has the full implementation
	return t.bootstrapManager.HandlePacket(packet, addr)
}

// handleSendNodes processes send nodes response packets.
func (t *Tox) handleSendNodes(packet *transport.Packet, addr net.Addr) error {
	// Delegate to the bootstrap manager which has the full implementation
	return t.bootstrapManager.HandlePacket(packet, addr)
}

// Iterate performs a single iteration of the Tox event loop.
//
//export ToxIterate
func (t *Tox) Iterate() {
	t.iterationCount++

	// Process DHT maintenance
	t.doDHTMaintenance()

	// Process friend connections
	t.doFriendConnections()

	// Process message queue
	t.doMessageProcessing()

	// Retry pending friend requests (production retry queue)
	t.retryPendingFriendRequests()
}

// doDHTMaintenance performs periodic DHT maintenance tasks.
// Runs every ~6 seconds (120 iterations × 50 ms tick) to avoid flooding the network.
func (t *Tox) doDHTMaintenance() {
	if t.dht == nil || t.keyPair == nil || t.bootstrapManager == nil {
		return
	}

	// Rate-limit: run once every 120 iterations (~6 s at 50 ms/tick).
	if t.iterationCount%120 != 0 {
		return
	}

	selfToxID := crypto.NewToxID(t.keyPair.Public, t.nospam)
	allNodes := t.dht.FindClosestNodes(*selfToxID, 100)

	if len(allNodes) < 10 {
		// Routing table is sparse — re-bootstrap to replenish it.
		ctx, cancel := context.WithTimeout(t.ctx, t.options.BootstrapTimeout)
		defer cancel()

		if err := t.bootstrapManager.Bootstrap(ctx); err != nil {
			logrus.WithFields(logrus.Fields{
				"function":   "doDHTMaintenance",
				"node_count": len(allNodes),
				"error":      err.Error(),
			}).Debug("DHT re-bootstrap attempt failed")
		}
	} else {
		// Routing table has nodes — send FIND_NODE queries toward our own key to
		// keep buckets fresh. We reuse Bootstrap to ping the known bootstrap nodes;
		// a full FIND_NODE walk is handled by the DHT Maintainer when present.
		if t.bootstrapManager.IsBootstrapped() {
			return
		}
		ctx, cancel := context.WithTimeout(t.ctx, t.options.BootstrapTimeout)
		defer cancel()
		_ = t.bootstrapManager.Bootstrap(ctx) //nolint:errcheck // best-effort refresh
	}
}

// doFriendConnections manages friend connections.
// Rate-limited to every 240 iterations (~12 s at 50 ms/tick).
func (t *Tox) doFriendConnections() {
	if !t.shouldRunFriendConnections() {
		return
	}

	offlineKeys := t.collectOfflineFriendKeys()
	if len(offlineKeys) == 0 {
		return
	}

	t.scheduleFriendRequestRetries(offlineKeys)
}

// shouldRunFriendConnections checks if friend connection processing should run.
func (t *Tox) shouldRunFriendConnections() bool {
	if t.friends.Count() == 0 || t.dht == nil {
		return false
	}
	// Only run every 240 iterations to avoid DHT flooding.
	return t.iterationCount%240 == 0
}

// collectOfflineFriendKeys returns public keys of all offline friends.
func (t *Tox) collectOfflineFriendKeys() [][32]byte {
	offlineKeys := make([][32]byte, 0, t.friends.Count())
	t.friends.Range(func(_ uint32, f *Friend) bool {
		if f.ConnectionStatus == ConnectionNone {
			offlineKeys = append(offlineKeys, f.PublicKey)
		}
		return true
	})
	return offlineKeys
}

// scheduleFriendRequestRetries schedules immediate retries for pending friend requests
// when DHT routes are found for offline friends.
func (t *Tox) scheduleFriendRequestRetries(offlineKeys [][32]byte) {
	now := t.now()
	t.pendingFriendReqsMux.Lock()
	defer t.pendingFriendReqsMux.Unlock()

	for _, pk := range offlineKeys {
		t.maybeScheduleRetryForFriend(pk, now)
	}
}

// maybeScheduleRetryForFriend checks if a DHT route exists for the given friend
// and schedules immediate retry if so.
func (t *Tox) maybeScheduleRetryForFriend(pk [32]byte, now time.Time) {
	friendToxID := crypto.NewToxID(pk, [4]byte{})
	nodes := t.dht.FindClosestNodes(*friendToxID, 1)
	if len(nodes) == 0 {
		return
	}

	for i, req := range t.pendingFriendReqs {
		if req.targetPublicKey == pk && now.After(req.nextRetry) {
			t.pendingFriendReqs[i].nextRetry = now // schedule immediate retry
			logrus.WithFields(logrus.Fields{
				"function":  "doFriendConnections",
				"target_pk": fmt.Sprintf("%x", pk[:8]),
				"dht_nodes": len(nodes),
			}).Debug("DHT route found for offline friend, scheduling request retry")
		}
	}
}

// updateFriendField validates a string value, updates a friend field, and
// invokes a callback. This consolidates the common pattern used in
// receiveFriendNameUpdate and receiveFriendStatusMessageUpdate.
func (t *Tox) updateFriendField(
	friendID uint32,
	value string,
	maxLen int,
	updateFn func(*Friend, string),
	callbackFn func(uint32, string),
) {
	if len([]byte(value)) > maxLen {
		return // Ignore oversized values
	}

	// Use atomic Update to ensure thread-safe modification of friend fields
	updated := t.friends.Update(friendID, func(f *Friend) {
		updateFn(f, value)
	})

	if updated {
		callbackFn(friendID, value)
	}
}

// receiveFriendNameUpdate processes incoming friend name update packets
func (t *Tox) receiveFriendNameUpdate(friendID uint32, name string) {
	t.updateFriendField(
		friendID,
		name,
		128, // Max name length for Tox protocol
		func(f *Friend, v string) { f.Name = v },
		t.invokeFriendNameCallback,
	)
}

// receiveFriendTyping processes incoming typing notification packets
func (t *Tox) receiveFriendTyping(friendID uint32, isTyping bool) {
	// Use atomic Update to ensure thread-safe modification
	updated := t.friends.Update(friendID, func(f *Friend) {
		f.IsTyping = isTyping
	})

	if updated {
		// Dispatch to typing notification callback
		t.invokeFriendTypingCallback(friendID, isTyping)
	}
}

// receiveFriendRequest processes incoming friend request packets
func (t *Tox) receiveFriendRequest(senderPublicKey [32]byte, message string) {
	// Validate message length (1016 bytes max for Tox friend request message)
	if len([]byte(message)) > 1016 {
		return // Ignore oversized friend request messages
	}

	// Check if this public key is already a friend
	_, exists := t.getFriendIDByPublicKey(senderPublicKey)
	if exists {
		return // Ignore friend requests from existing friends
	}

	// Route through RequestManager if available for centralized request handling
	if t.requestManager != nil {
		// Create a friend.Request to track in RequestManager
		req := &friend.Request{
			SenderPublicKey: senderPublicKey,
			Message:         message,
		}
		t.requestManager.AddRequest(req)
	}

	// Trigger the friend request callback if set
	t.callbackMu.RLock()
	callback := t.friendRequestCallback
	t.callbackMu.RUnlock()
	if callback != nil {
		callback(senderPublicKey, message)
	}
}

// sendFriendRequest sends a friend request packet to the specified public key
func (t *Tox) sendFriendRequest(targetPublicKey [32]byte, message string) error {
	if len([]byte(message)) > 1016 {
		return errors.New("friend request message too long")
	}

	packetData := t.buildFriendRequestPacket(targetPublicKey, message)
	packet := &transport.Packet{
		PacketType: transport.PacketFriendRequest,
		Data:       packetData,
	}

	sentViaNetwork := t.attemptNetworkSend(targetPublicKey, message, packet)

	if !sentViaNetwork {
		t.handleFailedNetworkSend(targetPublicKey, message, packet, packetData)
	}

	return nil
}

// buildFriendRequestPacket constructs the friend request packet data.
func (t *Tox) buildFriendRequestPacket(targetPublicKey [32]byte, message string) []byte {
	packetData := make([]byte, 32+len(message))
	copy(packetData[0:32], t.keyPair.Public[:])
	copy(packetData[32:], message)
	return packetData
}

// attemptNetworkSend tries to send the friend request via DHT network.
func (t *Tox) attemptNetworkSend(targetPublicKey [32]byte, message string, packet *transport.Packet) bool {
	targetToxID := crypto.NewToxID(targetPublicKey, [4]byte{})
	closestNodes := t.dht.FindClosestNodes(*targetToxID, 1)

	if len(closestNodes) == 0 || t.udpTransport == nil || closestNodes[0].Address == nil {
		return false
	}

	logrus.WithFields(logrus.Fields{
		"function":       "sendFriendRequest",
		"target_pk":      fmt.Sprintf("%x", targetPublicKey[:8]),
		"closest_node":   closestNodes[0].Address.String(),
		"message_length": len(message),
	}).Info("Sending friend request via DHT network")

	if err := t.udpTransport.Send(packet, closestNodes[0].Address); err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "sendFriendRequest",
			"error":     err.Error(),
			"node_addr": closestNodes[0].Address.String(),
		}).Warn("Failed to send friend request via DHT, will queue for retry")
		return false
	}

	return true
}

// handleFailedNetworkSend handles friend request when network send fails.
func (t *Tox) handleFailedNetworkSend(targetPublicKey [32]byte, message string, _ *transport.Packet, packetData []byte) {
	t.queuePendingFriendRequest(targetPublicKey, message, packetData)
}

// queuePendingFriendRequest queues a friend request for retry in production scenarios
func (t *Tox) queuePendingFriendRequest(targetPublicKey [32]byte, message string, packetData []byte) {
	t.pendingFriendReqsMux.Lock()
	defer t.pendingFriendReqsMux.Unlock()

	// Check if we already have a pending request for this public key
	for i, req := range t.pendingFriendReqs {
		if req.targetPublicKey == targetPublicKey {
			// Update existing request
			t.pendingFriendReqs[i].message = message
			t.pendingFriendReqs[i].packetData = packetData
			t.pendingFriendReqs[i].timestamp = t.now()
			logrus.WithFields(logrus.Fields{
				"function":  "queuePendingFriendRequest",
				"target_pk": fmt.Sprintf("%x", targetPublicKey[:8]),
			}).Debug("Updated existing pending friend request")
			return
		}
	}

	// Add new pending request
	now := t.now()
	req := &pendingFriendRequest{
		targetPublicKey: targetPublicKey,
		message:         message,
		packetData:      packetData,
		timestamp:       now,
		retryCount:      0,
		nextRetry:       now.Add(5 * time.Second), // Initial retry after 5 seconds
	}
	t.pendingFriendReqs = append(t.pendingFriendReqs, req)

	logrus.WithFields(logrus.Fields{
		"function":   "queuePendingFriendRequest",
		"target_pk":  fmt.Sprintf("%x", targetPublicKey[:8]),
		"next_retry": req.nextRetry,
	}).Info("Queued friend request for retry")
}

// retryPendingFriendRequests attempts to resend friend requests that failed initial delivery
func (t *Tox) retryPendingFriendRequests() {
	t.pendingFriendReqsMux.Lock()
	defer t.pendingFriendReqsMux.Unlock()

	now := t.now()
	var stillPending []*pendingFriendRequest

	for _, req := range t.pendingFriendReqs {
		if now.Before(req.nextRetry) {
			stillPending = append(stillPending, req)
			continue
		}

		if t.attemptSendRequest(req, now) {
			continue
		}

		if t.shouldKeepRetrying(req, now) {
			t.scheduleNextRetry(req, now)
			stillPending = append(stillPending, req)
		}
	}

	t.pendingFriendReqs = stillPending
}

// attemptSendRequest tries to send a friend request via DHT and returns true if successful.
func (t *Tox) attemptSendRequest(req *pendingFriendRequest, now time.Time) bool {
	targetToxID := crypto.NewToxID(req.targetPublicKey, [4]byte{})
	closestNodes := t.dht.FindClosestNodes(*targetToxID, 1)

	if len(closestNodes) == 0 || t.udpTransport == nil || closestNodes[0].Address == nil {
		return false
	}

	packet := &transport.Packet{
		PacketType: transport.PacketFriendRequest,
		Data:       req.packetData,
	}

	if err := t.udpTransport.Send(packet, closestNodes[0].Address); err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "retryPendingFriendRequests",
			"target_pk":   fmt.Sprintf("%x", req.targetPublicKey[:8]),
			"retry_count": req.retryCount,
			"error":       err.Error(),
		}).Warn("Failed to retry friend request")
		return false
	}

	logrus.WithFields(logrus.Fields{
		"function":    "retryPendingFriendRequests",
		"target_pk":   fmt.Sprintf("%x", req.targetPublicKey[:8]),
		"retry_count": req.retryCount,
		"node_addr":   closestNodes[0].Address.String(),
	}).Info("Successfully retried friend request via DHT")
	return true
}

// shouldKeepRetrying determines if we should continue retrying a failed request.
func (t *Tox) shouldKeepRetrying(req *pendingFriendRequest, now time.Time) bool {
	req.retryCount++

	if req.retryCount >= 10 {
		logrus.WithFields(logrus.Fields{
			"function":    "retryPendingFriendRequests",
			"target_pk":   fmt.Sprintf("%x", req.targetPublicKey[:8]),
			"retry_count": req.retryCount,
			"age":         now.Sub(req.timestamp),
		}).Warn("Giving up on friend request after maximum retries")
		return false
	}
	return true
}

// scheduleNextRetry calculates and schedules the next retry with exponential backoff.
func (t *Tox) scheduleNextRetry(req *pendingFriendRequest, now time.Time) {
	backoff := time.Duration(5*(1<<uint(req.retryCount))) * time.Second
	req.nextRetry = now.Add(backoff)

	logrus.WithFields(logrus.Fields{
		"function":    "retryPendingFriendRequests",
		"target_pk":   fmt.Sprintf("%x", req.targetPublicKey[:8]),
		"retry_count": req.retryCount,
		"next_retry":  req.nextRetry,
		"backoff":     backoff,
	}).Debug("Scheduled friend request retry with exponential backoff")
}

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
	if err := validatePacketSize(packet); err != nil {
		return err
	}

	packetType := packet[0]
	return t.routePacketByType(packetType, packet)
}

// validatePacketSize checks if the packet meets minimum size requirements.
func validatePacketSize(packet []byte) error {
	if len(packet) < 4 {
		return errors.New("packet too small")
	}
	return nil
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

// SetTimeProvider sets a custom time provider for deterministic testing.
// This should only be used in tests. In production, the default RealTimeProvider is used.
func (t *Tox) SetTimeProvider(tp TimeProvider) {
	t.timeProvider = tp
}

// now returns the current time using the configured time provider.
func (t *Tox) now() time.Time {
	return t.timeProvider.Now()
}

// Kill stops the Tox instance and releases all resources.
//
//export ToxKill
func (t *Tox) Kill() {
	t.running = false
	t.cancel()

	t.closeTransports()
	t.stopBackgroundServices()
	t.cleanupManagers()
	t.clearCallbacks()
}

// closeTransports closes UDP and TCP transport connections.
func (t *Tox) closeTransports() {
	if t.udpTransport != nil {
		if err := t.udpTransport.Close(); err != nil {
			logrus.WithError(err).Warn("Failed to close UDP transport")
		}
	}

	if t.tcpTransport != nil {
		if err := t.tcpTransport.Close(); err != nil {
			logrus.WithError(err).Warn("Failed to close TCP transport")
		}
	}
}

// stopBackgroundServices stops async manager and LAN discovery services.
func (t *Tox) stopBackgroundServices() {
	if t.asyncManager != nil {
		t.asyncManager.Stop()
	}

	if t.lanDiscovery != nil {
		t.lanDiscovery.Stop()
	}

	if t.natTraversal != nil {
		if err := t.natTraversal.Close(); err != nil {
			logrus.WithError(err).Warn("Failed to close NAT traversal")
		}
		t.natTraversal = nil
	}

	t.dhtMutex.Lock()
	if t.dht != nil {
		t.dht = nil
	}
	t.dhtMutex.Unlock()

	if t.bootstrapManager != nil {
		t.bootstrapManager = nil
	}
}

// cleanupManagers cleans up all manager instances and the friends list.
func (t *Tox) cleanupManagers() {
	t.messageManagerMu.Lock()
	if t.messageManager != nil {
		t.messageManager = nil
	}
	t.messageManagerMu.Unlock()

	if t.fileManager != nil {
		t.fileManager = nil
	}

	if t.requestManager != nil {
		t.requestManager = nil
	}

	// Clear the friends store
	if t.friends != nil {
		t.friends.Clear()
	}
}

// clearCallbacks clears all callback functions to prevent memory leaks.
func (t *Tox) clearCallbacks() {
	t.friendRequestCallback = nil
	t.friendMessageCallback = nil
	t.simpleFriendMessageCallback = nil
	t.friendStatusCallback = nil
	t.connectionStatusCallback = nil
	t.friendConnectionStatusCallback = nil
	t.friendStatusChangeCallback = nil
}

// Bootstrap connects to a bootstrap node to join the Tox network.
//
// validateBootstrapPublicKey validates the public key format and hex encoding.
//
//export ToxBootstrap
func validateBootstrapPublicKey(publicKeyHex, address string, port uint16) error {
	if len(publicKeyHex) != 64 {
		err := fmt.Errorf("invalid public key length: expected 64, got %d", len(publicKeyHex))
		logrus.WithFields(logrus.Fields{
			"function":          "Bootstrap",
			"address":           address,
			"port":              port,
			"public_key_length": len(publicKeyHex),
			"error":             err.Error(),
		}).Error("Public key validation failed")
		return err
	}

	_, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		err := fmt.Errorf("invalid hex public key: %w", err)
		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"address":  address,
			"port":     port,
			"error":    err.Error(),
		}).Error("Public key hex decoding failed")
		return err
	}
	return nil
}

// resolveBootstrapAddress resolves the bootstrap node address.
func resolveBootstrapAddress(address string, port uint16) (net.Addr, error) {
	logrus.WithFields(logrus.Fields{
		"function": "Bootstrap",
		"address":  address,
		"port":     port,
	}).Debug("Resolving bootstrap address")

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(address, fmt.Sprintf("%d", port)))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"address":  address,
			"port":     port,
			"error":    err.Error(),
		}).Error("Bootstrap address resolution failed")
		return nil, fmt.Errorf("failed to resolve bootstrap address %s:%d: %w", address, port, err)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "Bootstrap",
		"resolved_addr": addr.String(),
	}).Debug("Bootstrap address resolved successfully")
	return addr, nil
}

// Bootstrap connects to a bootstrap node to join the Tox network.
// The address is the hostname or IP, port is the UDP port, and publicKeyHex
// is the node's public key in hexadecimal format.
func (t *Tox) Bootstrap(address string, port uint16, publicKeyHex string) error {
	logrus.WithFields(logrus.Fields{
		"function":   "Bootstrap",
		"address":    address,
		"port":       port,
		"public_key": publicKeyHex[:16] + "...",
	}).Info("Attempting to bootstrap")

	if err := validateBootstrapPublicKey(publicKeyHex, address, port); err != nil {
		return err
	}

	addr, err := resolveBootstrapAddress(address, port)
	if err != nil {
		return err
	}

	if err := t.addBootstrapNode(addr, publicKeyHex); err != nil {
		return err
	}

	return t.executeBootstrapProcess(address, port)
}

// addBootstrapNode adds a bootstrap node to the manager.
func (t *Tox) addBootstrapNode(addr net.Addr, publicKeyHex string) error {
	logrus.WithFields(logrus.Fields{
		"function": "Bootstrap",
	}).Debug("Adding bootstrap node to manager")

	if err := t.bootstrapManager.AddNode(addr, publicKeyHex); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"error":    err.Error(),
		}).Error("Failed to add bootstrap node to manager")
		return err
	}
	return nil
}

// executeBootstrapProcess starts and monitors the bootstrap process with timeout and retry.
// Uses exponential backoff with up to 3 retries to improve reliability on congested networks.
func (t *Tox) executeBootstrapProcess(address string, port uint16) error {
	const maxRetries = 3

	logrus.WithFields(logrus.Fields{
		"function":    "Bootstrap",
		"timeout":     t.options.BootstrapTimeout,
		"max_retries": maxRetries,
	}).Debug("Starting bootstrap process with timeout and retry")

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			logrus.WithFields(logrus.Fields{
				"function": "Bootstrap",
				"attempt":  attempt + 1,
				"backoff":  backoff,
			}).Debug("Retrying bootstrap after backoff")
			time.Sleep(backoff)
		}

		ctx, cancel := context.WithTimeout(t.ctx, t.options.BootstrapTimeout)
		lastErr = t.bootstrapManager.Bootstrap(ctx)
		cancel()

		if lastErr == nil {
			logrus.WithFields(logrus.Fields{
				"function": "Bootstrap",
				"address":  address,
				"port":     port,
				"attempts": attempt + 1,
			}).Info("Bootstrap completed successfully")
			return nil
		}

		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"address":  address,
			"port":     port,
			"attempt":  attempt + 1,
			"error":    lastErr.Error(),
		}).Debug("Bootstrap attempt failed, will retry")
	}

	logrus.WithFields(logrus.Fields{
		"function":    "Bootstrap",
		"address":     address,
		"port":        port,
		"max_retries": maxRetries,
		"error":       lastErr.Error(),
	}).Error("Bootstrap process failed after all retries")
	return lastErr
}

// AddRelayServer adds a TCP relay server for symmetric NAT fallback.
// Relay servers are used when direct UDP connections fail, particularly
// for users behind symmetric NAT where UDP hole punching doesn't work.
//
// Example:
//
//	tox.AddRelayServer("relay.example.com", 33445, publicKey, 1)
//
//export ToxAddRelayServer
func (t *Tox) AddRelayServer(address string, port uint16, publicKey [32]byte, priority int) {
	if t.natTraversal == nil {
		logrus.Warn("NAT traversal not initialized, cannot add relay server")
		return
	}

	server := transport.RelayServerInfo{
		Address:   address,
		PublicKey: publicKey,
		Port:      port,
		Priority:  priority,
	}

	t.natTraversal.AddRelayServer(server)

	logrus.WithFields(logrus.Fields{
		"function": "AddRelayServer",
		"address":  address,
		"port":     port,
		"priority": priority,
	}).Info("Added relay server")
}

// RemoveRelayServer removes a TCP relay server by address.
//
//export ToxRemoveRelayServer
func (t *Tox) RemoveRelayServer(address string) {
	if t.natTraversal == nil {
		return
	}

	t.natTraversal.RemoveRelayServer(address)

	logrus.WithFields(logrus.Fields{
		"function": "RemoveRelayServer",
		"address":  address,
	}).Info("Removed relay server")
}

// EnableRelayFallback enables or disables relay connection fallback.
// When enabled, connections that fail via direct UDP will attempt to connect
// through configured relay servers.
//
//export ToxEnableRelayFallback
func (t *Tox) EnableRelayFallback(enabled bool) {
	if t.natTraversal == nil {
		logrus.Warn("NAT traversal not initialized, cannot configure relay fallback")
		return
	}

	t.natTraversal.EnableMethod(transport.ConnectionRelay, enabled)

	logrus.WithFields(logrus.Fields{
		"function": "EnableRelayFallback",
		"enabled":  enabled,
	}).Info("Relay fallback configuration updated")
}

// IsRelayConnected returns true if connected to a TCP relay server.
//
//export ToxIsRelayConnected
func (t *Tox) IsRelayConnected() bool {
	if t.natTraversal == nil {
		return false
	}
	return t.natTraversal.IsRelayConnected()
}

// DiscoverRelayServers queries the DHT for available relay servers
// and automatically adds them to the relay server list.
// Returns the number of relay servers discovered.
//
//export ToxDiscoverRelayServers
func (t *Tox) DiscoverRelayServers() (int, error) {
	if t.dht == nil {
		return 0, fmt.Errorf("DHT not initialized")
	}
	if t.natTraversal == nil {
		return 0, fmt.Errorf("NAT traversal not initialized")
	}

	relays, err := t.dht.QueryRelays(t.udpTransport)
	if err != nil {
		return 0, fmt.Errorf("failed to query relays from DHT: %w", err)
	}

	count := 0
	for _, relay := range relays {
		t.natTraversal.AddRelayServer(relay.ToTransportServerInfo())
		count++
	}

	logrus.WithFields(logrus.Fields{
		"function":     "DiscoverRelayServers",
		"relays_found": count,
	}).Info("Discovered relay servers from DHT")

	return count, nil
}

// GetRelayServerCount returns the number of configured relay servers.
//
//export ToxGetRelayServerCount
func (t *Tox) GetRelayServerCount() int {
	if t.natTraversal == nil || t.natTraversal.GetRelayClient() == nil {
		return 0
	}
	return t.natTraversal.GetRelayClient().GetServerCount()
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

const (
	FriendStatusNone FriendStatus = iota
	FriendStatusAway
	FriendStatusBusy
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

// Save saves the Tox state to a byte slice.
//
//export ToxSave
func (t *Tox) Save() ([]byte, error) {
	// Use the existing GetSavedata implementation to serialize state
	savedata := t.GetSavedata()
	if savedata == nil {
		return nil, errors.New("failed to serialize Tox state")
	}

	return savedata, nil
}

// Load loads the Tox state from a byte slice created by GetSavedata.
// This method restores the private key, friends list, and configuration
// from previously saved data.
//
// The Tox instance must be in a clean state before calling Load.
// This method will overwrite existing keys and friends.
//
// Load restores the Tox instance state from saved data.
//
// The function loads a previously saved Tox state including keypair,
// friends list, self information, and nospam value. It validates the
// data integrity and maintains backward compatibility with older formats.
//
//export ToxLoad
func (t *Tox) Load(data []byte) error {
	if err := t.validateLoadData(data); err != nil {
		return err
	}

	saveData, err := t.unmarshalSaveData(data)
	if err != nil {
		return err
	}

	if err := t.restoreKeyPair(saveData); err != nil {
		return err
	}

	t.restoreFriendsList(saveData)
	t.restoreOptions(saveData)
	t.restoreSelfInformation(saveData)
	if err := t.restoreNospamValue(saveData); err != nil {
		return err
	}

	return nil
}

// SaveSnapshot saves the Tox state in binary snapshot format for faster recovery.
// The binary format is significantly faster to serialize/deserialize than JSON,
// making it suitable for frequent checkpoints and fast startup times.
//
// The snapshot format includes a magic header and version for compatibility checking.
// Use LoadSnapshot or Load (which auto-detects format) to restore the state.
//
//export ToxSaveSnapshot
func (t *Tox) SaveSnapshot() ([]byte, error) {
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()

	saveData := toxSaveData{
		KeyPair:       t.keyPair,
		Friends:       make(map[uint32]*Friend),
		Options:       t.options,
		SelfName:      t.selfName,
		SelfStatusMsg: t.selfStatusMsg,
		Nospam:        t.nospam,
	}

	// Copy friends data using sharded store's GetAll
	for id, f := range t.friends.GetAll() {
		saveData.Friends[id] = &Friend{
			PublicKey:        f.PublicKey,
			Status:           f.Status,
			ConnectionStatus: f.ConnectionStatus,
			Name:             f.Name,
			StatusMessage:    f.StatusMessage,
			LastSeen:         f.LastSeen,
		}
	}

	return saveData.marshalBinary()
}

// LoadSnapshot loads the Tox state from a binary snapshot.
// This is an explicit method for loading binary snapshots. The regular Load
// method will also auto-detect and load binary snapshots.
//
//export ToxLoadSnapshot
func (t *Tox) LoadSnapshot(data []byte) error {
	if err := t.validateLoadData(data); err != nil {
		return err
	}

	if !isSnapshotFormat(data) {
		return errors.New("not a binary snapshot format")
	}

	var saveData toxSaveData
	if err := saveData.unmarshalBinary(data); err != nil {
		return fmt.Errorf("snapshot unmarshal: %w", err)
	}

	if err := t.restoreKeyPair(&saveData); err != nil {
		return err
	}

	t.restoreFriendsList(&saveData)
	t.restoreOptions(&saveData)
	t.restoreSelfInformation(&saveData)
	if err := t.restoreNospamValue(&saveData); err != nil {
		return err
	}

	return nil
}

// validateLoadData checks if the provided save data is valid for loading.
func (t *Tox) validateLoadData(data []byte) error {
	if len(data) == 0 {
		return errors.New("save data is empty")
	}
	return nil
}

// unmarshalSaveData parses the binary save data into a structured format.
// Automatically detects binary snapshot format vs legacy JSON format.
func (t *Tox) unmarshalSaveData(data []byte) (*toxSaveData, error) {
	var saveData toxSaveData

	// Auto-detect format
	if isSnapshotFormat(data) {
		if err := saveData.unmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("binary snapshot unmarshal: %w", err)
		}
	} else {
		if err := saveData.unmarshal(data); err != nil {
			return nil, fmt.Errorf("json unmarshal: %w", err)
		}
	}
	return &saveData, nil
}

// restoreKeyPair validates and restores the cryptographic key pair.
func (t *Tox) restoreKeyPair(saveData *toxSaveData) error {
	if saveData.KeyPair == nil {
		return errors.New("save data missing key pair")
	}
	t.keyPair = saveData.KeyPair
	return nil
}

// restoreFriendsList reconstructs the friends list from saved data.
func (t *Tox) restoreFriendsList(saveData *toxSaveData) {
	if saveData.Friends != nil {
		// Clear and re-populate the friends store
		t.friends.Clear()
		for id, f := range saveData.Friends {
			if f != nil {
				t.friends.Set(id, &Friend{
					PublicKey:        f.PublicKey,
					Status:           f.Status,
					ConnectionStatus: f.ConnectionStatus,
					Name:             f.Name,
					StatusMessage:    f.StatusMessage,
					LastSeen:         f.LastSeen,
					// UserData is not restored as it was not serialized
				})
			}
		}
	}
}

// restoreOptions selectively restores safe configuration options.
func (t *Tox) restoreOptions(saveData *toxSaveData) {
	if saveData.Options != nil && t.options != nil {
		// Only restore certain safe options, not all options should be restored
		// as some are runtime-specific (like network settings)
		t.options.SavedataType = saveData.Options.SavedataType
		t.options.SavedataData = saveData.Options.SavedataData
		t.options.SavedataLength = saveData.Options.SavedataLength
	}
}

// restoreSelfInformation restores the user's profile information.
func (t *Tox) restoreSelfInformation(saveData *toxSaveData) {
	t.selfMutex.Lock()
	defer t.selfMutex.Unlock()
	t.selfName = saveData.SelfName
	t.selfStatusMsg = saveData.SelfStatusMsg
}

// restoreNospamValue restores or generates the nospam value for backward compatibility.
// Returns an error if generation fails for old savedata without nospam.
func (t *Tox) restoreNospamValue(saveData *toxSaveData) error {
	if saveData.Nospam == [4]byte{} {
		// Old savedata without nospam - generate a new one
		nospam, err := generateNospam()
		if err != nil {
			return fmt.Errorf("failed to generate nospam during restore: %w", err)
		}
		t.nospam = nospam
	} else {
		t.nospam = saveData.Nospam
	}
	return nil
}

// loadSavedState loads saved state from options during initialization.
// This method handles different savedata types and integrates with the existing Load functionality.
func (t *Tox) loadSavedState(options *Options) error {
	if options == nil {
		return nil
	}

	switch options.SavedataType {
	case SaveDataTypeNone:
		// No saved data to load
		return nil
	case SaveDataTypeSecretKey:
		// Secret key is already handled in createKeyPair
		return nil
	case SaveDataTypeToxSave:
		// Load complete Tox state including friends
		if len(options.SavedataData) == 0 {
			return errors.New("savedata type is ToxSave but no data provided")
		}

		// Validate savedata length matches
		if options.SavedataLength > 0 && len(options.SavedataData) != int(options.SavedataLength) {
			return errors.New("savedata length mismatch")
		}

		// Use the existing Load method to restore state
		return t.Load(options.SavedataData)
	default:
		return errors.New("unknown savedata type")
	}
}

// MessageType represents the type of a message.
type MessageType uint8

const (
	MessageTypeNormal MessageType = iota
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
	if t.packetDelivery != nil {
		err := t.packetDelivery.DeliverPacket(friendID, packet)
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

// generateMessageID generates a cryptographically secure random 32-bit message ID
func generateMessageID() (uint32, error) {
	var buf [4]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf[:]), nil
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
func (t *Tox) FileControl(friendID, fileID uint32, control FileControl) error {
	// Validate friend exists
	if !t.friends.Exists(friendID) {
		return errors.New("friend not found")
	}

	// Find the file transfer
	transferKey := (uint64(friendID) << 32) | uint64(fileID)
	t.transfersMu.RLock()
	transfer, exists := t.fileTransfers[transferKey]
	t.transfersMu.RUnlock()

	if !exists {
		return errors.New("file transfer not found")
	}

	// Apply the control action
	switch control {
	case FileControlResume:
		return transfer.Resume()
	case FileControlPause:
		return transfer.Pause()
	case FileControlCancel:
		return transfer.Cancel()
	default:
		return errors.New("invalid file control action")
	}
}

// FileAccept accepts an incoming file transfer.
// This is a convenience method equivalent to FileControl(friendID, fileID, FileControlResume).
// Call this from the OnFileRecv callback to accept an incoming file transfer.
//
//export ToxFileAccept
func (t *Tox) FileAccept(friendID, fileID uint32) error {
	return t.FileControl(friendID, fileID, FileControlResume)
}

// FileReject rejects or cancels an incoming file transfer.
// This is a convenience method equivalent to FileControl(friendID, fileID, FileControlCancel).
// Call this from the OnFileRecv callback to reject an incoming file transfer.
//
//export ToxFileReject
func (t *Tox) FileReject(friendID, fileID uint32) error {
	return t.FileControl(friendID, fileID, FileControlCancel)
}

// FileSend starts a file transfer.
//
//export ToxFileSend
func (t *Tox) FileSend(friendID, kind uint32, fileSize uint64, fileID [32]byte, filename string) (uint32, error) {
	// Validate friend exists and is connected
	f := t.friends.Get(friendID)
	if f == nil {
		return 0, errors.New("friend not found")
	}

	if f.ConnectionStatus == ConnectionNone {
		return 0, errors.New("friend is not connected")
	}

	// Validate parameters
	if len(filename) == 0 {
		return 0, errors.New("filename cannot be empty")
	}

	// Generate a unique local file transfer ID (simplified)
	localFileID := uint32(t.now().UnixNano() & 0xFFFFFFFF)

	// Create new file transfer
	transfer := file.NewTransfer(friendID, localFileID, filename, fileSize, file.TransferDirectionOutgoing)

	// Store the transfer
	transferKey := (uint64(friendID) << 32) | uint64(localFileID)
	t.transfersMu.Lock()
	t.fileTransfers[transferKey] = transfer
	t.transfersMu.Unlock()

	// Create and send file transfer request packet
	err := t.sendFileTransferRequest(friendID, localFileID, fileSize, fileID, filename)
	if err != nil {
		// Clean up the transfer on send failure
		t.transfersMu.Lock()
		delete(t.fileTransfers, transferKey)
		t.transfersMu.Unlock()
		return 0, fmt.Errorf("failed to send file transfer request: %w", err)
	}

	return localFileID, nil
}

// sendFileTransferRequest creates and sends a file transfer request packet
func (t *Tox) sendFileTransferRequest(friendID, fileID uint32, fileSize uint64, fileHash [32]byte, filename string) error {
	packetData, err := t.createFileTransferPacketData(fileID, fileSize, fileHash, filename)
	if err != nil {
		return err
	}

	packet := &transport.Packet{
		PacketType: transport.PacketFileRequest,
		Data:       packetData,
	}

	friend, err := t.lookupFriendForTransfer(friendID)
	if err != nil {
		return err
	}

	targetAddr, err := t.resolveFriendAddress(friend)
	if err != nil {
		return err
	}

	return t.sendPacketToTarget(packet, targetAddr)
}

// createFileTransferPacketData constructs the binary packet data for file transfer requests.
// Packet format: [fileID(4)][fileSize(8)][fileHash(32)][filename_length(2)][filename]
func (t *Tox) createFileTransferPacketData(fileID uint32, fileSize uint64, fileHash [32]byte, filename string) ([]byte, error) {
	filenameBytes := []byte(filename)
	if len(filenameBytes) > 65535 {
		return nil, errors.New("filename too long")
	}

	packetData := make([]byte, 4+8+32+2+len(filenameBytes))
	offset := 0

	// File ID (4 bytes)
	binary.BigEndian.PutUint32(packetData[offset:], fileID)
	offset += 4

	// File size (8 bytes)
	binary.BigEndian.PutUint64(packetData[offset:], fileSize)
	offset += 8

	// File hash (32 bytes)
	copy(packetData[offset:], fileHash[:])
	offset += 32

	// Filename length (2 bytes)
	binary.BigEndian.PutUint16(packetData[offset:], uint16(len(filenameBytes)))
	offset += 2

	// Filename
	copy(packetData[offset:], filenameBytes)

	return packetData, nil
}

// lookupFriendForTransfer retrieves the friend information needed for file transfer operations.
func (t *Tox) lookupFriendForTransfer(friendID uint32) (*Friend, error) {
	f := t.friends.Get(friendID)
	if f == nil {
		return nil, errors.New("friend not found for file transfer")
	}

	return f, nil
}

// resolveFriendAddress determines the network address for a friend using DHT lookup.
func (t *Tox) resolveFriendAddress(friend *Friend) (net.Addr, error) {
	t.dhtMutex.RLock()
	dht := t.dht
	t.dhtMutex.RUnlock()

	if dht == nil {
		return nil, fmt.Errorf("DHT not available for address resolution")
	}

	// Create ToxID from friend's public key for DHT lookup
	friendToxID := crypto.ToxID{
		PublicKey: friend.PublicKey,
		Nospam:    [4]byte{}, // Unknown nospam, but DHT uses public key for routing
		Checksum:  [2]byte{}, // Checksum not needed for DHT lookup
	}

	// Find closest nodes to the friend in our routing table
	closestNodes := dht.FindClosestNodes(friendToxID, 1)
	if len(closestNodes) > 0 && closestNodes[0].Address != nil {
		return closestNodes[0].Address, nil
	}

	return nil, fmt.Errorf("failed to resolve network address for friend via DHT lookup")
}

// resolveFriendIDFromAddress attempts to find a friend ID from a network address.
// This performs a reverse lookup through the DHT to find which friend is associated
// with the given address. Returns an error if no friend is found.
func (t *Tox) resolveFriendIDFromAddress(addr net.Addr) (uint32, error) {
	if t.dht == nil {
		return 0, fmt.Errorf("DHT not available for reverse address resolution")
	}

	// Search through DHT nodes to find one matching this address
	// and then check if that public key belongs to a friend
	nodes := t.dht.GetAllNodes()
	for _, node := range nodes {
		if node.Address != nil && node.Address.String() == addr.String() {
			// Found a matching node, check if this public key is a friend
			friendID, exists := t.getFriendIDByPublicKey(node.ID.PublicKey)
			if exists {
				return friendID, nil
			}
		}
	}

	return 0, fmt.Errorf("no friend found for address: %s", addr.String())
}

// sendPacketToTarget transmits a packet to the specified network address using the UDP transport.
func (t *Tox) sendPacketToTarget(packet *transport.Packet, targetAddr net.Addr) error {
	if t.udpTransport == nil {
		return fmt.Errorf("no transport available")
	}

	err := t.udpTransport.Send(packet, targetAddr)
	if err != nil {
		return fmt.Errorf("failed to send file transfer request: %w", err)
	}

	return nil
}

// sendPacketToFriend resolves a friend's address and sends a packet to them.
// This is a convenience method that combines address resolution with packet transmission.
func (t *Tox) sendPacketToFriend(friendID uint32, friend *Friend, data []byte, packetType transport.PacketType) error {
	// Resolve friend's network address
	friendAddr, err := t.resolveFriendAddress(friend)
	if err != nil {
		return fmt.Errorf("failed to resolve friend address: %w", err)
	}

	// Check if transport is available
	if t.udpTransport == nil {
		return fmt.Errorf("no transport available")
	}

	// Create transport packet
	transportPacket := &transport.Packet{
		PacketType: packetType,
		Data:       data,
	}

	// Send packet to friend
	if err := t.udpTransport.Send(transportPacket, friendAddr); err != nil {
		return fmt.Errorf("failed to send packet to friend: %w", err)
	}

	return nil
}

// validateFriendConnection validates that a friend exists and is connected.
// Returns the friend object if validation passes, otherwise returns an error.
func (t *Tox) validateFriendConnection(friendID uint32) (*Friend, error) {
	return t.validateFriendOnline(friendID, "friend is not connected")
}

// lookupFileTransfer retrieves and validates a file transfer for the given friend and file IDs.
// Returns the transfer object if found and valid, otherwise returns an error.
func (t *Tox) lookupFileTransfer(friendID, fileID uint32) (*file.Transfer, error) {
	transferKey := (uint64(friendID) << 32) | uint64(fileID)
	t.transfersMu.RLock()
	transfer, exists := t.fileTransfers[transferKey]
	t.transfersMu.RUnlock()

	if !exists {
		return nil, errors.New("file transfer not found")
	}

	if transfer.State != file.TransferStateRunning {
		return nil, errors.New("transfer is not in running state")
	}

	return transfer, nil
}

// validateChunkData validates the chunk position and size according to protocol constraints.
// Returns an error if validation fails, otherwise returns nil.
func (t *Tox) validateChunkData(position uint64, data []byte, fileSize uint64) error {
	if position > fileSize {
		return errors.New("position exceeds file size")
	}

	const maxChunkSize = 1024 // 1KB chunks
	if len(data) > maxChunkSize {
		return fmt.Errorf("chunk size %d exceeds maximum %d", len(data), maxChunkSize)
	}

	return nil
}

// updateTransferProgress updates the transfer progress after a successful chunk send.
// This function is thread-safe and updates the transferred bytes count.
func (t *Tox) updateTransferProgress(friendID, fileID uint32, position uint64, dataLen int) {
	transferKey := (uint64(friendID) << 32) | uint64(fileID)
	t.transfersMu.Lock()
	if transfer, exists := t.fileTransfers[transferKey]; exists {
		transfer.Transferred = position + uint64(dataLen)
	}
	t.transfersMu.Unlock()
}

// FileSendChunk sends a chunk of file data.
//
//export ToxFileSendChunk
func (t *Tox) FileSendChunk(friendID, fileID uint32, position uint64, data []byte) error {
	// Validate friend exists and is connected
	_, err := t.validateFriendConnection(friendID)
	if err != nil {
		return err
	}

	// Find and validate file transfer
	transfer, err := t.lookupFileTransfer(friendID, fileID)
	if err != nil {
		return err
	}

	// Validate chunk data
	err = t.validateChunkData(position, data, transfer.FileSize)
	if err != nil {
		return err
	}

	// Create and send file chunk packet
	err = t.sendFileChunk(friendID, fileID, position, data)
	if err != nil {
		return fmt.Errorf("failed to send file chunk: %w", err)
	}

	// Update transfer progress on successful send
	t.updateTransferProgress(friendID, fileID, position, len(data))

	return nil
}

// sendFileChunk creates and sends a file data chunk packet
func (t *Tox) sendFileChunk(friendID, fileID uint32, position uint64, data []byte) error {
	friend, err := t.validateFriendConnection(friendID)
	if err != nil {
		return fmt.Errorf("friend not found for file chunk transfer: %w", err)
	}

	packetData := t.buildFileChunkPacket(fileID, position, data)

	packet := &transport.Packet{
		PacketType: transport.PacketFileData,
		Data:       packetData,
	}

	targetAddr, err := t.resolveFriendAddress(friend)
	if err != nil {
		return err
	}

	return t.sendPacketToTarget(packet, targetAddr)
}

// buildFileChunkPacket creates the binary packet data for a file chunk.
// Packet format: [fileID(4)][position(8)][data_length(2)][data]
func (t *Tox) buildFileChunkPacket(fileID uint32, position uint64, data []byte) []byte {
	dataLength := len(data)
	packetData := make([]byte, 4+8+2+dataLength)
	offset := 0

	// File ID (4 bytes)
	binary.BigEndian.PutUint32(packetData[offset:], fileID)
	offset += 4

	// Position (8 bytes)
	binary.BigEndian.PutUint64(packetData[offset:], position)
	offset += 8

	// Data length (2 bytes)
	binary.BigEndian.PutUint16(packetData[offset:], uint16(dataLength))
	offset += 2

	// Data
	copy(packetData[offset:], data)

	return packetData
}

// ConferenceNew creates a new conference (group chat).
//
//export ToxConferenceNew
func (t *Tox) ConferenceNew() (uint32, error) {
	t.conferencesMu.Lock()
	defer t.conferencesMu.Unlock()

	// Generate unique conference ID
	conferenceID := t.nextConferenceID
	t.nextConferenceID++

	// Create a new group chat for the conference
	// Use CreateWithKeyPair to enable encryption for group messages
	chat, err := group.CreateWithKeyPair("Conference", group.ChatTypeText, group.PrivacyPublic, t.udpTransport, t.dht, t.keyPair)
	if err != nil {
		return 0, fmt.Errorf("failed to create conference: %w", err)
	}

	// Override the ID with our conference ID
	chat.ID = conferenceID

	// Store the conference
	t.conferences[conferenceID] = chat

	return conferenceID, nil
}

// ConferenceInvite invites a friend to a conference.
//
//export ToxConferenceInvite
func (t *Tox) ConferenceInvite(friendID, conferenceID uint32) error {
	// Validate friend exists
	if !t.friends.Exists(friendID) {
		return errors.New("friend not found")
	}

	// Validate conference exists
	t.conferencesMu.RLock()
	conference, exists := t.conferences[conferenceID]
	t.conferencesMu.RUnlock()

	if !exists {
		return errors.New("conference not found")
	}

	// Basic permission check - for now allow all invitations
	// In a full implementation, this would check if the user has invite permissions

	// Generate conference invitation data
	inviteData := fmt.Sprintf("CONF_INVITE:%d:%s", conferenceID, conference.Name)

	// Send invitation through friend messaging system
	_, err := t.FriendSendMessage(friendID, inviteData, MessageTypeNormal)
	if err != nil {
		return fmt.Errorf("failed to send conference invitation: %w", err)
	}

	return nil
}

// ConferenceSendMessage sends a message to a conference.
//
//export ToxConferenceSendMessage
func (t *Tox) ConferenceSendMessage(conferenceID uint32, message string, messageType MessageType) error {
	if err := t.validateConferenceMessage(message); err != nil {
		return err
	}

	conference, err := t.validateConferenceAccess(conferenceID)
	if err != nil {
		return err
	}

	messageData := t.createConferenceMessagePacket(conferenceID, message, messageType)

	return t.broadcastConferenceMessage(conference, messageData)
}

// validateConferenceMessage checks if the conference message input is valid.
func (t *Tox) validateConferenceMessage(message string) error {
	if len(message) == 0 {
		return errors.New("message cannot be empty")
	}

	// Validate message length (Tox message limit)
	if len(message) > 1372 {
		return errors.New("message too long")
	}

	return nil
}

// ValidateConferenceAccess verifies conference exists and user membership.
// Returns the conference Chat object if access is valid, or an error otherwise.
// This method is exported for use by the C API bindings.
func (t *Tox) ValidateConferenceAccess(conferenceID uint32) (*group.Chat, error) {
	return t.validateConferenceAccess(conferenceID)
}

// validateConferenceAccess verifies conference exists and user membership.
func (t *Tox) validateConferenceAccess(conferenceID uint32) (*group.Chat, error) {
	// Validate conference exists
	t.conferencesMu.RLock()
	conference, exists := t.conferences[conferenceID]
	t.conferencesMu.RUnlock()

	if !exists {
		return nil, errors.New("conference not found")
	}

	// Validate we are a member of the conference
	if conference.SelfPeerID == 0 && len(conference.Peers) == 0 {
		return nil, errors.New("not a member of this conference")
	}

	return conference, nil
}

// createConferenceMessagePacket formats the message for conference transmission.
func (t *Tox) createConferenceMessagePacket(conferenceID uint32, message string, messageType MessageType) string {
	// Create conference message packet
	// For now, using a simple packet format without encryption
	return fmt.Sprintf("CONF_MSG:%d:%d:%s", conferenceID, messageType, message)
}

// broadcastConferenceMessage sends the message to all conference peers.
func (t *Tox) broadcastConferenceMessage(conference *group.Chat, messageData string) error {
	broadcastCount := t.sendToConferencePeers(conference, messageData)

	if broadcastCount == 0 && len(conference.Peers) > 1 {
		return errors.New("failed to broadcast to any conference peers")
	}
	return nil
}

// sendToConferencePeers sends a message to all remote conference peers and returns the success count.
func (t *Tox) sendToConferencePeers(conference *group.Chat, messageData string) int {
	count := 0
	for peerID, peer := range conference.Peers {
		if peerID == conference.SelfPeerID {
			continue
		}
		friendID, exists := t.getFriendIDByPublicKey(peer.PublicKey)
		if !exists {
			continue
		}
		if err := t.SendFriendMessage(friendID, messageData, MessageTypeNormal); err == nil {
			count++
		}
	}
	return count
}

// ConferenceDelete leaves and deletes a conference (group chat).
// This removes the local copy of the conference after broadcasting a leave message.
//
//export ToxConferenceDelete
func (t *Tox) ConferenceDelete(conferenceID uint32) error {
	t.conferencesMu.Lock()
	conference, exists := t.conferences[conferenceID]
	if !exists {
		t.conferencesMu.Unlock()
		return errors.New("conference not found")
	}
	// Remove from map while holding lock
	delete(t.conferences, conferenceID)
	t.conferencesMu.Unlock()

	// Call Leave on the group.Chat to broadcast departure and clean up
	if err := conference.Leave(""); err != nil {
		// Log but don't fail - conference already removed locally
		logrus.WithFields(logrus.Fields{
			"function":      "ConferenceDelete",
			"conference_id": conferenceID,
			"error":         err.Error(),
		}).Warn("Failed to broadcast leave message")
	}

	return nil
}

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

// FileManager returns the centralized file transfer manager.
// The manager coordinates file transfers with transport integration,
// handling packet routing, address resolution, and transfer lifecycle.
// Returns nil if the manager was not initialized (e.g., no transport available).
func (t *Tox) FileManager() *file.Manager {
	return t.fileManager
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
		"current_mode":   t.packetDelivery.IsSimulation(),
	}).Info("Switching packet delivery mode")

	if err := t.validateDeliveryFactory(); err != nil {
		return err
	}

	t.switchDeliveryFactory(useSimulation)

	newDelivery := t.createPacketDelivery(useSimulation)
	t.packetDelivery = newDelivery

	logrus.WithFields(logrus.Fields{
		"function":   "SetPacketDeliveryMode",
		"new_mode":   t.packetDelivery.IsSimulation(),
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
	if t.packetDelivery == nil {
		return interfaces.PacketDeliveryStats{
			IsSimulation: true,
		}
	}

	return t.packetDelivery.GetTypedStats()
}

// IsPacketDeliverySimulation returns true if currently using simulation
func (t *Tox) IsPacketDeliverySimulation() bool {
	if t.packetDelivery == nil {
		return true // Default to simulation if not initialized
	}
	return t.packetDelivery.IsSimulation()
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

const (
	EncryptionNoiseIK       EncryptionStatus = "noise-ik"
	EncryptionLegacy        EncryptionStatus = "legacy"
	EncryptionForwardSecure EncryptionStatus = "forward-secure"
	EncryptionOffline       EncryptionStatus = "offline"
	EncryptionUnknown       EncryptionStatus = "unknown"
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
