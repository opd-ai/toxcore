package internal

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/opd-ai/toxforge"
)

// TestClient represents a Tox client instance for testing.
type TestClient struct {
	tox       *toxcore.Tox
	name      string
	connected bool
	friends   map[uint32]*FriendConnection
	mu        sync.RWMutex
	logger    *log.Logger
	metrics   *ClientMetrics

	// Callback channels for testing
	friendRequestCh chan FriendRequest
	messageCh       chan Message
	connectionCh    chan ConnectionEvent
}

// FriendConnection represents a friend relationship state.
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

// ClientMetrics tracks client performance and activity.
type ClientMetrics struct {
	StartTime          time.Time
	MessagesSent       int64
	MessagesReceived   int64
	FriendRequestsSent int64
	FriendRequestsRecv int64
	ConnectionEvents   int64
	mu                 sync.RWMutex
}

// ClientConfig holds configuration for a test client.
type ClientConfig struct {
	Name           string
	UDPEnabled     bool
	IPv6Enabled    bool
	LocalDiscovery bool
	StartPort      uint16
	EndPort        uint16
	Logger         *log.Logger
}

// DefaultClientConfig returns a default configuration for a test client.
func DefaultClientConfig(name string) *ClientConfig {
	// Use different port ranges for different clients to avoid conflicts
	var startPort, endPort uint16
	if name == "Alice" {
		startPort, endPort = 33500, 33599
	} else if name == "Bob" {
		startPort, endPort = 33600, 33699
	} else {
		startPort, endPort = 33700, 33799
	}

	return &ClientConfig{
		Name:           name,
		UDPEnabled:     true,
		IPv6Enabled:    false, // Simplify for localhost testing
		LocalDiscovery: false,
		StartPort:      startPort,
		EndPort:        endPort,
		Logger:         log.Default(),
	}
}

// NewTestClient creates a new test client instance.
func NewTestClient(config *ClientConfig) (*TestClient, error) {
	if config == nil {
		config = DefaultClientConfig("TestClient")
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
		logger:          config.Logger,
		metrics:         &ClientMetrics{StartTime: time.Now()},
		friendRequestCh: make(chan FriendRequest, 10),
		messageCh:       make(chan Message, 100),
		connectionCh:    make(chan ConnectionEvent, 10),
	}

	// Set up callbacks
	client.setupCallbacks()

	return client, nil
}

// setupCallbacks configures the Tox event callbacks for testing.
func (tc *TestClient) setupCallbacks() {
	// Friend request callback
	tc.tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		tc.logger.Printf("[%s] Received friend request: %s", tc.name, message)

		tc.metrics.mu.Lock()
		tc.metrics.FriendRequestsRecv++
		tc.metrics.mu.Unlock()

		select {
		case tc.friendRequestCh <- FriendRequest{
			PublicKey: publicKey,
			Message:   message,
			Timestamp: time.Now(),
		}:
		default:
			tc.logger.Printf("[%s] Friend request channel full, dropping request", tc.name)
		}
	})

	// Friend message callback
	tc.tox.OnFriendMessage(func(friendID uint32, message string) {
		tc.logger.Printf("[%s] Received message from friend %d: %s", tc.name, friendID, message)

		tc.metrics.mu.Lock()
		tc.metrics.MessagesReceived++
		tc.metrics.mu.Unlock()

		tc.mu.Lock()
		if friend, exists := tc.friends[friendID]; exists {
			friend.MessagesRecv++
			friend.LastSeen = time.Now()
		}
		tc.mu.Unlock()

		select {
		case tc.messageCh <- Message{
			FriendID:  friendID,
			Content:   message,
			Timestamp: time.Now(),
		}:
		default:
			tc.logger.Printf("[%s] Message channel full, dropping message", tc.name)
		}
	})
}

// Start initializes and starts the client.
func (tc *TestClient) Start(ctx context.Context) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.logger.Printf("[%s] Starting client...", tc.name)
	tc.logger.Printf("[%s] Public key: %X", tc.name, tc.tox.GetSelfPublicKey())

	// Set client name
	tc.tox.SelfSetName(tc.name)

	// Start event loop
	go tc.eventLoop(ctx)

	tc.logger.Printf("[%s] ✅ Client started successfully", tc.name)
	return nil
}

// Stop gracefully shuts down the client.
func (tc *TestClient) Stop() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.logger.Printf("[%s] Stopping client...", tc.name)
	tc.tox.Kill()
	tc.connected = false

	uptime := time.Since(tc.metrics.StartTime)
	tc.logger.Printf("[%s] ✅ Client stopped after %v uptime", tc.name, uptime)
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
			Timestamp: time.Now(),
		}:
		default:
		}

		tc.logger.Printf("[%s] Connection status changed: %v", tc.name, currentStatus)
	}
}

// ConnectToBootstrap connects the client to a bootstrap server.
func (tc *TestClient) ConnectToBootstrap(address string, port uint16, publicKeyHex string) error {
	tc.logger.Printf("[%s] Connecting to bootstrap %s:%d", tc.name, address, port)

	err := tc.tox.Bootstrap(address, port, publicKeyHex)
	if err != nil {
		return fmt.Errorf("failed to connect to bootstrap: %w", err)
	}

	return nil
}

// SendFriendRequest sends a friend request to the specified public key.
func (tc *TestClient) SendFriendRequest(publicKey [32]byte, message string) (uint32, error) {
	tc.logger.Printf("[%s] Sending friend request: %s", tc.name, message)

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
		LastSeen:  time.Now(),
	}
	tc.mu.Unlock()

	tc.metrics.mu.Lock()
	tc.metrics.FriendRequestsSent++
	tc.metrics.mu.Unlock()

	tc.logger.Printf("[%s] ✅ Friend request sent (ID: %d)", tc.name, friendID)
	return friendID, nil
}

// AcceptFriendRequest accepts a pending friend request.
func (tc *TestClient) AcceptFriendRequest(publicKey [32]byte) (uint32, error) {
	tc.logger.Printf("[%s] Accepting friend request from %X", tc.name, publicKey)

	friendID, err := tc.tox.AddFriendByPublicKey(publicKey)
	if err != nil {
		return 0, fmt.Errorf("failed to accept friend request: %w", err)
	}

	tc.mu.Lock()
	tc.friends[friendID] = &FriendConnection{
		FriendID:  friendID,
		PublicKey: publicKey,
		Status:    FriendStatusOffline,
		LastSeen:  time.Now(),
	}
	tc.mu.Unlock()

	tc.logger.Printf("[%s] ✅ Friend request accepted (ID: %d)", tc.name, friendID)
	return friendID, nil
}

// SendMessage sends a message to a friend.
func (tc *TestClient) SendMessage(friendID uint32, message string) error {
	tc.logger.Printf("[%s] Sending message to friend %d: %s", tc.name, friendID, message)

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

	tc.logger.Printf("[%s] ✅ Message sent to friend %d", tc.name, friendID)
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
			return fmt.Errorf("timeout waiting for connection")
		case <-ticker.C:
			if tc.IsConnected() {
				tc.logger.Printf("[%s] ✅ Connected to network", tc.name)
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
		return nil, fmt.Errorf("timeout waiting for friend request")
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
		return nil, fmt.Errorf("timeout waiting for message")
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
		"uptime":               time.Since(metrics.StartTime).String(),
		"messages_sent":        metrics.MessagesSent,
		"messages_received":    metrics.MessagesReceived,
		"friend_requests_sent": metrics.FriendRequestsSent,
		"friend_requests_recv": metrics.FriendRequestsRecv,
		"connection_events":    metrics.ConnectionEvents,
	}
}
