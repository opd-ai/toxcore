package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/sirupsen/logrus"
)

// TestClient represents a Tox client instance for testing.
type TestClient struct {
	tox          *toxcore.Tox
	name         string
	connected    bool
	friends      map[uint32]*FriendConnection
	mu           sync.RWMutex
	logger       *logrus.Entry
	metrics      *ClientMetrics
	timeProvider TimeProvider // Injectable time source for deterministic testing

	// Callback channels for testing
	friendRequestCh chan FriendRequest
	messageCh       chan Message
	connectionCh    chan ConnectionEvent
}

// FriendConnection represents a friend relationship state.
// It tracks the complete state of a friend for test validation:
//   - FriendID: Local identifier assigned by Tox for this friend
//   - PublicKey: 32-byte Ed25519 public key identifying the friend
//   - Status: Current online status (offline, online, away, busy)
//   - LastSeen: Timestamp of last activity from this friend
//   - MessagesSent/MessagesRecv: Counters for test assertions
type FriendConnection struct {
	FriendID     uint32
	PublicKey    [32]byte
	Status       FriendStatus
	LastSeen     time.Time
	MessagesSent int
	MessagesRecv int
}

// FriendStatus represents the status of a friend connection.
type FriendStatus int

const (
	FriendStatusOffline FriendStatus = iota
	FriendStatusOnline
	FriendStatusAway
	FriendStatusBusy
)

// FriendRequest represents an incoming friend request.
type FriendRequest struct {
	PublicKey [32]byte
	Message   string
	Timestamp time.Time
}

// Message represents a received message.
type Message struct {
	FriendID  uint32
	Content   string
	Timestamp time.Time
}

// ConnectionEvent represents a connection status change.
type ConnectionEvent struct {
	Status    toxcore.ConnectionStatus
	Timestamp time.Time
}

// ClientMetrics tracks client performance and activity during tests.
// It provides counters for validating expected message/request flows:
//   - StartTime: When the client was initialized
//   - MessagesSent/MessagesReceived: Total messages for throughput validation
//   - FriendRequestsSent/FriendRequestsRecv: Friend request counters
//   - ConnectionEvents: Number of connection state changes observed
//
// Metrics are safe for concurrent access via internal mutex.
type ClientMetrics struct {
	StartTime          time.Time
	MessagesSent       int64
	MessagesReceived   int64
	FriendRequestsSent int64
	FriendRequestsRecv int64
	ConnectionEvents   int64
	mu                 sync.RWMutex
}

// ClientStatus represents comprehensive client status information.
// This provides type-safe access to client state for programmatic inspection:
//   - Name: Human-readable identifier for the client
//   - Connected: Whether the client has established network connectivity
//   - PublicKey: Hex-encoded 32-byte Ed25519 public key
//   - ConnectionStatus: Current network connection type (None/TCP/UDP)
//   - FriendCount: Number of friends in the client's friend list
//   - Uptime: Duration since client initialization
//   - MessagesSent/MessagesReceived: Message counters from ClientMetrics
//   - FriendRequestsSent/FriendRequestsRecv: Friend request counters
//   - ConnectionEvents: Number of connection state changes
type ClientStatus struct {
	Name               string
	Connected          bool
	PublicKey          string
	ConnectionStatus   toxcore.ConnectionStatus
	FriendCount        int
	Uptime             time.Duration
	MessagesSent       int64
	MessagesReceived   int64
	FriendRequestsSent int64
	FriendRequestsRecv int64
	ConnectionEvents   int64
}

// ClientConfig holds configuration for a test client.
type ClientConfig struct {
	Name           string
	UDPEnabled     bool
	IPv6Enabled    bool
	LocalDiscovery bool
	StartPort      uint16
	EndPort        uint16
	Logger         *logrus.Entry
}

// DefaultClientConfig returns a default configuration for a test client.
func DefaultClientConfig(name string) *ClientConfig {
	// Use different port ranges for different clients to avoid conflicts
	var startPort, endPort uint16
	if name == "Alice" {
		startPort, endPort = AlicePortRangeStart, AlicePortRangeEnd
	} else if name == "Bob" {
		startPort, endPort = BobPortRangeStart, BobPortRangeEnd
	} else {
		startPort, endPort = OtherPortRangeStart, OtherPortRangeEnd
	}

	return &ClientConfig{
		Name:           name,
		UDPEnabled:     true,
		IPv6Enabled:    false, // Simplify for localhost testing
		LocalDiscovery: false,
		StartPort:      startPort,
		EndPort:        endPort,
		Logger:         logrus.WithField("component", "client"),
	}
}

// NewTestClient creates a new test client instance.
func NewTestClient(config *ClientConfig) (*TestClient, error) {
	if config == nil {
		config = DefaultClientConfig("TestClient")
	}

	// Validate port range configuration
	if !ValidatePortRange(config.StartPort, config.EndPort) {
		return nil, fmt.Errorf("invalid port range [%d-%d] for client %s: must be between %d and %d with start <= end",
			config.StartPort, config.EndPort, config.Name, MinValidPort, MaxValidPort)
	}

	// Create Tox options optimized for testing
	options := toxcore.NewOptionsForTesting()
	options.UDPEnabled = config.UDPEnabled
	options.IPv6Enabled = config.IPv6Enabled
	options.LocalDiscovery = config.LocalDiscovery
	options.StartPort = config.StartPort
	options.EndPort = config.EndPort

	// Create Tox instance
	tox, err := toxcore.New(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tox instance for %s: %w", config.Name, err)
	}

	client := &TestClient{
		tox:             tox,
		name:            config.Name,
		connected:       false,
		friends:         make(map[uint32]*FriendConnection),
		logger:          config.Logger.WithField("client_name", config.Name),
		timeProvider:    NewDefaultTimeProvider(),
		metrics:         &ClientMetrics{}, // StartTime set below using timeProvider
		friendRequestCh: make(chan FriendRequest, 10),
		messageCh:       make(chan Message, 100),
		connectionCh:    make(chan ConnectionEvent, 10),
	}
	client.metrics.StartTime = client.getTimeProvider().Now()

	// Set up callbacks
	client.setupCallbacks()

	return client, nil
}

// setupCallbacks configures the Tox event callbacks for testing.
func (tc *TestClient) setupCallbacks() {
	// Friend request callback
	tc.tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		tc.logger.WithFields(logrus.Fields{
			"message": message,
		}).Info("Received friend request")

		tc.metrics.mu.Lock()
		tc.metrics.FriendRequestsRecv++
		tc.metrics.mu.Unlock()

		select {
		case tc.friendRequestCh <- FriendRequest{
			PublicKey: publicKey,
			Message:   message,
			Timestamp: tc.getTimeProvider().Now(),
		}:
		default:
			tc.logger.Warn("Friend request channel full, dropping request")
		}
	})

	// Friend message callback
	tc.tox.OnFriendMessage(func(friendID uint32, message string) {
		tc.logger.WithFields(logrus.Fields{
			"friend_id": friendID,
			"message":   message,
		}).Info("Received message from friend")

		tc.metrics.mu.Lock()
		tc.metrics.MessagesReceived++
		tc.metrics.mu.Unlock()

		tc.mu.Lock()
		if friend, exists := tc.friends[friendID]; exists {
			friend.MessagesRecv++
			friend.LastSeen = tc.getTimeProvider().Now()
		}
		tc.mu.Unlock()

		select {
		case tc.messageCh <- Message{
			FriendID:  friendID,
			Content:   message,
			Timestamp: tc.getTimeProvider().Now(),
		}:
		default:
			tc.logger.Warn("Message channel full, dropping message")
		}
	})
}

// Start initializes and starts the client.
func (tc *TestClient) Start(ctx context.Context) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.logger.Info("Starting client...")
	tc.logger.WithField("public_key", fmt.Sprintf("%X", tc.tox.GetSelfPublicKey())).Debug("Client public key")

	// Set client name
	tc.tox.SelfSetName(tc.name)

	// Start event loop
	go tc.eventLoop(ctx)

	tc.logger.Info("✅ Client started successfully")
	return nil
}

// Stop gracefully shuts down the client.
func (tc *TestClient) Stop() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.logger.Info("Stopping client...")
	tc.tox.Kill()
	tc.connected = false

	uptime := tc.getTimeProvider().Since(tc.metrics.StartTime)
	tc.logger.WithField("uptime", uptime).Info("✅ Client stopped")
	return nil
}

// eventLoop runs the main Tox iteration loop for the client.
func (tc *TestClient) eventLoop(ctx context.Context) {
	ticker := time.NewTicker(tc.tox.IterationInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tc.tox.Iterate()
			tc.updateConnectionStatus()
		}
	}
}

// updateConnectionStatus monitors and updates connection status.
func (tc *TestClient) updateConnectionStatus() {
	currentStatus := tc.tox.SelfGetConnectionStatus()

	tc.mu.Lock()
	wasConnected := tc.connected
	tc.connected = currentStatus != toxcore.ConnectionNone
	tc.mu.Unlock()

	// Notify on connection changes
	if wasConnected != tc.connected {
		tc.metrics.mu.Lock()
		tc.metrics.ConnectionEvents++
		tc.metrics.mu.Unlock()

		select {
		case tc.connectionCh <- ConnectionEvent{
			Status:    currentStatus,
			Timestamp: tc.getTimeProvider().Now(),
		}:
		default:
		}

		tc.logger.WithField("connection_status", currentStatus).Info("Connection status changed")
	}
}

// ConnectToBootstrap connects the client to a bootstrap server.
func (tc *TestClient) ConnectToBootstrap(address string, port uint16, publicKeyHex string) error {
	tc.logger.WithFields(logrus.Fields{
		"address": address,
		"port":    port,
	}).Info("Connecting to bootstrap")

	err := tc.tox.Bootstrap(address, port, publicKeyHex)
	if err != nil {
		return fmt.Errorf("failed to connect to bootstrap: %w", err)
	}

	return nil
}

// SendFriendRequest sends a friend request to the specified public key.
func (tc *TestClient) SendFriendRequest(publicKey [32]byte, message string) (uint32, error) {
	tc.logger.WithField("message", message).Info("Sending friend request")

	// Convert public key to hex string for AddFriend API
	publicKeyHex := fmt.Sprintf("%X", publicKey)
	// Create a simple Tox ID with zero nospam and checksum for testing
	// In production, you'd need proper nospam and checksum calculation
	toxIDStr := publicKeyHex + "00000000" + "0000" // 32 bytes pubkey + 4 bytes nospam + 2 bytes checksum

	friendID, err := tc.tox.AddFriend(toxIDStr, message)
	if err != nil {
		return 0, fmt.Errorf("failed to send friend request: %w", err)
	}

	tc.mu.Lock()
	tc.friends[friendID] = &FriendConnection{
		FriendID:  friendID,
		PublicKey: publicKey,
		Status:    FriendStatusOffline,
		LastSeen:  tc.getTimeProvider().Now(),
	}
	tc.mu.Unlock()

	tc.metrics.mu.Lock()
	tc.metrics.FriendRequestsSent++
	tc.metrics.mu.Unlock()

	tc.logger.WithField("friend_id", friendID).Info("✅ Friend request sent")
	return friendID, nil
}

// AcceptFriendRequest accepts a pending friend request.
func (tc *TestClient) AcceptFriendRequest(publicKey [32]byte) (uint32, error) {
	tc.logger.WithField("public_key", fmt.Sprintf("%X", publicKey)).Info("Accepting friend request")

	friendID, err := tc.tox.AddFriendByPublicKey(publicKey)
	if err != nil {
		return 0, fmt.Errorf("failed to accept friend request: %w", err)
	}

	tc.mu.Lock()
	tc.friends[friendID] = &FriendConnection{
		FriendID:  friendID,
		PublicKey: publicKey,
		Status:    FriendStatusOffline,
		LastSeen:  tc.getTimeProvider().Now(),
	}
	tc.mu.Unlock()

	tc.logger.WithField("friend_id", friendID).Info("✅ Friend request accepted")
	return friendID, nil
}

// SendMessage sends a message to a friend.
func (tc *TestClient) SendMessage(friendID uint32, message string) error {
	tc.logger.WithFields(logrus.Fields{
		"friend_id": friendID,
		"message":   message,
	}).Info("Sending message to friend")

	err := tc.tox.SendFriendMessage(friendID, message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	tc.mu.Lock()
	if friend, exists := tc.friends[friendID]; exists {
		friend.MessagesSent++
	}
	tc.mu.Unlock()

	tc.metrics.mu.Lock()
	tc.metrics.MessagesSent++
	tc.metrics.mu.Unlock()

	tc.logger.WithField("friend_id", friendID).Info("✅ Message sent to friend")
	return nil
}

// WaitForConnection waits for the client to connect to the network.
func (tc *TestClient) WaitForConnection(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("client %s: timeout waiting for connection after %v", tc.name, timeout)
		case <-ticker.C:
			if tc.IsConnected() {
				tc.logger.Info("✅ Connected to network")
				return nil
			}
		}
	}
}

// WaitForFriendRequest waits for a friend request to arrive.
func (tc *TestClient) WaitForFriendRequest(timeout time.Duration) (*FriendRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("client %s: timeout waiting for friend request after %v", tc.name, timeout)
	case request := <-tc.friendRequestCh:
		return &request, nil
	}
}

// WaitForMessage waits for a message to arrive.
func (tc *TestClient) WaitForMessage(timeout time.Duration) (*Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("client %s: timeout waiting for message after %v", tc.name, timeout)
	case message := <-tc.messageCh:
		return &message, nil
	}
}

// GetName returns the client name.
func (tc *TestClient) GetName() string {
	return tc.name
}

// GetPublicKey returns the client's public key.
func (tc *TestClient) GetPublicKey() [32]byte {
	return tc.tox.GetSelfPublicKey()
}

// IsConnected returns whether the client is connected to the network.
func (tc *TestClient) IsConnected() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.connected
}

// GetFriends returns a list of current friends.
func (tc *TestClient) GetFriends() map[uint32]*FriendConnection {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	friends := make(map[uint32]*FriendConnection)
	for id, friend := range tc.friends {
		friends[id] = &FriendConnection{
			FriendID:     friend.FriendID,
			PublicKey:    friend.PublicKey,
			Status:       friend.Status,
			LastSeen:     friend.LastSeen,
			MessagesSent: friend.MessagesSent,
			MessagesRecv: friend.MessagesRecv,
		}
	}
	return friends
}

// GetMetrics returns a copy of the current client metrics.
func (tc *TestClient) GetMetrics() ClientMetrics {
	tc.metrics.mu.RLock()
	defer tc.metrics.mu.RUnlock()
	return ClientMetrics{
		StartTime:          tc.metrics.StartTime,
		MessagesSent:       tc.metrics.MessagesSent,
		MessagesReceived:   tc.metrics.MessagesReceived,
		FriendRequestsSent: tc.metrics.FriendRequestsSent,
		FriendRequestsRecv: tc.metrics.FriendRequestsRecv,
		ConnectionEvents:   tc.metrics.ConnectionEvents,
	}
}

// GetStatus returns comprehensive client status information.
// Deprecated: Use GetStatusTyped for type-safe access.
func (tc *TestClient) GetStatus() map[string]interface{} {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	metrics := tc.GetMetrics()
	friends := tc.GetFriends()

	return map[string]interface{}{
		"name":                 tc.name,
		"connected":            tc.connected,
		"public_key":           fmt.Sprintf("%X", tc.GetPublicKey()),
		"connection_status":    tc.tox.SelfGetConnectionStatus(),
		"friend_count":         len(friends),
		"uptime":               tc.getTimeProvider().Since(metrics.StartTime).String(),
		"messages_sent":        metrics.MessagesSent,
		"messages_received":    metrics.MessagesReceived,
		"friend_requests_sent": metrics.FriendRequestsSent,
		"friend_requests_recv": metrics.FriendRequestsRecv,
		"connection_events":    metrics.ConnectionEvents,
	}
}

// GetStatusTyped returns comprehensive client status as a typed struct.
// This provides type-safe access to client state for programmatic inspection.
func (tc *TestClient) GetStatusTyped() ClientStatus {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	metrics := tc.GetMetrics()
	friends := tc.GetFriends()

	return ClientStatus{
		Name:               tc.name,
		Connected:          tc.connected,
		PublicKey:          fmt.Sprintf("%X", tc.GetPublicKey()),
		ConnectionStatus:   tc.tox.SelfGetConnectionStatus(),
		FriendCount:        len(friends),
		Uptime:             tc.getTimeProvider().Since(metrics.StartTime),
		MessagesSent:       metrics.MessagesSent,
		MessagesReceived:   metrics.MessagesReceived,
		FriendRequestsSent: metrics.FriendRequestsSent,
		FriendRequestsRecv: metrics.FriendRequestsRecv,
		ConnectionEvents:   metrics.ConnectionEvents,
	}
}

// SetTimeProvider sets a custom TimeProvider for deterministic testing.
// If nil is passed, the default time provider (system clock) will be used.
func (tc *TestClient) SetTimeProvider(tp TimeProvider) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.timeProvider = tp
}

// getTimeProvider returns the configured TimeProvider or the default.
func (tc *TestClient) getTimeProvider() TimeProvider {
	return getTimeProvider(tc.timeProvider)
}
