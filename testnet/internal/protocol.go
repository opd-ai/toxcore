package internal

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ProtocolTestSuite manages the core Tox protocol test workflow.
type ProtocolTestSuite struct {
	server  *BootstrapServer
	clientA *TestClient
	clientB *TestClient
	logger  *log.Logger
	config  *ProtocolConfig
}

// ProtocolConfig holds configuration for protocol testing.
type ProtocolConfig struct {
	BootstrapTimeout     time.Duration
	ConnectionTimeout    time.Duration
	FriendRequestTimeout time.Duration
	MessageTimeout       time.Duration
	RetryAttempts        int
	RetryBackoff         time.Duration
	Logger               *log.Logger
}

// DefaultProtocolConfig returns a default configuration for protocol testing.
func DefaultProtocolConfig() *ProtocolConfig {
	return &ProtocolConfig{
		BootstrapTimeout:     10 * time.Second,
		ConnectionTimeout:    30 * time.Second,
		FriendRequestTimeout: 15 * time.Second,
		MessageTimeout:       10 * time.Second,
		RetryAttempts:        3,
		RetryBackoff:         time.Second,
		Logger:               log.Default(),
	}
}

// NewProtocolTestSuite creates a new protocol test suite.
func NewProtocolTestSuite(config *ProtocolConfig) *ProtocolTestSuite {
	if config == nil {
		config = DefaultProtocolConfig()
	}

	return &ProtocolTestSuite{
		config: config,
		logger: config.Logger,
	}
}

// ExecuteTest runs the complete protocol test workflow.
func (pts *ProtocolTestSuite) ExecuteTest(ctx context.Context) error {
	pts.logger.Println("🚀 Starting Tox Network Integration Test Suite")
	pts.logger.Println("=" + fmt.Sprintf("%50s", "="))

	// Step 1: Network Initialization
	if err := pts.initializeNetwork(ctx); err != nil {
		return fmt.Errorf("network initialization failed: %w", err)
	}

	// Step 2: Client Setup
	if err := pts.setupClients(ctx); err != nil {
		return fmt.Errorf("client setup failed: %w", err)
	}

	// Step 3: Friend Connection
	if err := pts.establishFriendConnection(ctx); err != nil {
		return fmt.Errorf("friend connection failed: %w", err)
	}

	// Step 4: Message Exchange
	if err := pts.testMessageExchange(ctx); err != nil {
		return fmt.Errorf("message exchange failed: %w", err)
	}

	pts.logger.Println("🎉 All tests completed successfully!")
	return nil
}

// initializeNetwork sets up and validates the bootstrap server.
func (pts *ProtocolTestSuite) initializeNetwork(ctx context.Context) error {
	pts.logger.Println("📡 Step 1: Network Initialization")

	// Create bootstrap server
	bootstrapConfig := DefaultBootstrapConfig()
	bootstrapConfig.Logger = pts.logger

	server, err := NewBootstrapServer(bootstrapConfig)
	if err != nil {
		return fmt.Errorf("failed to create bootstrap server: %w", err)
	}
	pts.server = server

	// Start the server
	if err := pts.server.Start(ctx); err != nil {
		return fmt.Errorf("failed to start bootstrap server: %w", err)
	}

	// Log server configuration
	status := pts.server.GetStatus()
	pts.logger.Printf("✅ Bootstrap server running on %s:%d", status["address"], status["port"])
	pts.logger.Printf("   Public key: %s", status["public_key"])
	pts.logger.Printf("   Connection status: %v", status["connection_status"])

	return nil
}

// setupClients creates and connects both test clients.
func (pts *ProtocolTestSuite) setupClients(ctx context.Context) error {
	pts.logger.Println("👥 Step 2: Client Setup")

	// Create Client A
	configA := DefaultClientConfig("Alice")
	configA.Logger = pts.logger
	clientA, err := NewTestClient(configA)
	if err != nil {
		return fmt.Errorf("failed to create Client A: %w", err)
	}
	pts.clientA = clientA

	// Create Client B
	configB := DefaultClientConfig("Bob")
	configB.Logger = pts.logger
	clientB, err := NewTestClient(configB)
	if err != nil {
		return fmt.Errorf("failed to create Client B: %w", err)
	}
	pts.clientB = clientB

	// Start both clients
	if err := pts.clientA.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Client A: %w", err)
	}

	if err := pts.clientB.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Client B: %w", err)
	}

	// Connect clients to bootstrap server
	err = pts.connectClientToBootstrap(pts.clientA)
	if err != nil {
		return fmt.Errorf("failed to connect Client A to bootstrap: %w", err)
	}

	err = pts.connectClientToBootstrap(pts.clientB)
	if err != nil {
		return fmt.Errorf("failed to connect Client B to bootstrap: %w", err)
	}

	// Wait for network connections
	if err := pts.waitForConnections(); err != nil {
		return fmt.Errorf("failed to establish network connections: %w", err)
	}

	pts.logger.Println("✅ Both clients connected to network")
	return nil
}

// connectClientToBootstrap connects a client to the bootstrap server.
func (pts *ProtocolTestSuite) connectClientToBootstrap(client *TestClient) error {
	return pts.retryOperation(func() error {
		return client.ConnectToBootstrap(
			pts.server.GetAddress(),
			pts.server.GetPort(),
			pts.server.GetPublicKeyHex(),
		)
	})
}

// waitForConnections waits for both clients to connect to the network.
func (pts *ProtocolTestSuite) waitForConnections() error {
	// Wait for Client A connection
	err := pts.clientA.WaitForConnection(pts.config.ConnectionTimeout)
	if err != nil {
		return fmt.Errorf("Client A connection timeout: %w", err)
	}

	// Wait for Client B connection
	err = pts.clientB.WaitForConnection(pts.config.ConnectionTimeout)
	if err != nil {
		return fmt.Errorf("Client B connection timeout: %w", err)
	}

	return nil
}

// establishFriendConnection creates bidirectional friend relationship.
func (pts *ProtocolTestSuite) establishFriendConnection(ctx context.Context) error {
	pts.logger.Println("🤝 Step 3: Friend Connection")

	// Client A sends friend request to Client B
	clientBPublicKey := pts.clientB.GetPublicKey()
	requestMessage := "Hello! This is a test friend request from Alice."

	pts.logger.Printf("📤 Alice sending friend request to Bob...")
	_, err := pts.clientA.SendFriendRequest(clientBPublicKey, requestMessage)
	if err != nil {
		return fmt.Errorf("failed to send friend request: %w", err)
	}

	// Client B waits for and accepts the friend request
	pts.logger.Printf("⏳ Waiting for Bob to receive friend request...")
	friendRequest, err := pts.clientB.WaitForFriendRequest(pts.config.FriendRequestTimeout)
	if err != nil {
		return fmt.Errorf("Client B did not receive friend request: %w", err)
	}

	pts.logger.Printf("📨 Bob received friend request: %s", friendRequest.Message)

	// Verify the request is from Client A
	clientAPublicKey := pts.clientA.GetPublicKey()
	if friendRequest.PublicKey != clientAPublicKey {
		return fmt.Errorf("friend request from unexpected client")
	}

	// Client B accepts the friend request
	pts.logger.Printf("✅ Bob accepting friend request...")
	_, err = pts.clientB.AcceptFriendRequest(friendRequest.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to accept friend request: %w", err)
	}

	// Small delay to ensure the acceptance is processed
	time.Sleep(500 * time.Millisecond)

	// Verify bidirectional friend status
	clientAFriends := pts.clientA.GetFriends()
	clientBFriends := pts.clientB.GetFriends()

	if len(clientAFriends) == 0 {
		return fmt.Errorf("Client A has no friends after connection")
	}

	if len(clientBFriends) == 0 {
		return fmt.Errorf("Client B has no friends after connection")
	}

	pts.logger.Printf("✅ Bidirectional friend relationship established")
	pts.logger.Printf("   Alice friends: %d, Bob friends: %d", len(clientAFriends), len(clientBFriends))

	return nil
}

// testMessageExchange tests bidirectional message communication.
func (pts *ProtocolTestSuite) testMessageExchange(ctx context.Context) error {
	pts.logger.Println("💬 Step 4: Message Exchange")

	// Get friend IDs for messaging
	clientAFriends := pts.clientA.GetFriends()
	clientBFriends := pts.clientB.GetFriends()

	var friendIDA, friendIDB uint32
	for id := range clientAFriends {
		friendIDA = id
		break
	}

	for id := range clientBFriends {
		friendIDB = id
		break
	}

	// Test 1: Bob sends initial message to Alice
	initialMessage := "Hello Alice! This is Bob's first message."
	pts.logger.Printf("📤 Bob sending message to Alice: %s", initialMessage)

	err := pts.clientB.SendMessage(friendIDB, initialMessage)
	if err != nil {
		return fmt.Errorf("Bob failed to send message: %w", err)
	}

	// Alice waits for the message
	pts.logger.Printf("⏳ Waiting for Alice to receive message...")
	receivedMsg, err := pts.clientA.WaitForMessage(pts.config.MessageTimeout)
	if err != nil {
		return fmt.Errorf("Alice did not receive message: %w", err)
	}

	if receivedMsg.Content != initialMessage {
		return fmt.Errorf("message content mismatch: expected %q, got %q", initialMessage, receivedMsg.Content)
	}

	pts.logger.Printf("✅ Alice received message: %s", receivedMsg.Content)

	// Test 2: Alice sends reply to Bob
	replyMessage := "Hi Bob! This is Alice's reply message."
	pts.logger.Printf("📤 Alice sending reply to Bob: %s", replyMessage)

	err = pts.clientA.SendMessage(friendIDA, replyMessage)
	if err != nil {
		return fmt.Errorf("Alice failed to send reply: %w", err)
	}

	// Bob waits for the reply
	pts.logger.Printf("⏳ Waiting for Bob to receive reply...")
	receivedReply, err := pts.clientB.WaitForMessage(pts.config.MessageTimeout)
	if err != nil {
		return fmt.Errorf("Bob did not receive reply: %w", err)
	}

	if receivedReply.Content != replyMessage {
		return fmt.Errorf("reply content mismatch: expected %q, got %q", replyMessage, receivedReply.Content)
	}

	pts.logger.Printf("✅ Bob received reply: %s", receivedReply.Content)

	// Verify delivery metrics
	pts.logFinalMetrics()

	pts.logger.Println("✅ Message exchange completed successfully")
	return nil
}

// logFinalMetrics outputs final test metrics and status.
func (pts *ProtocolTestSuite) logFinalMetrics() {
	pts.logger.Println("\n📊 Final Test Metrics:")

	// Server metrics
	serverStatus := pts.server.GetStatus()
	pts.logger.Printf("   Bootstrap Server:")
	pts.logger.Printf("     Uptime: %s", serverStatus["uptime"])
	pts.logger.Printf("     Packets processed: %v", serverStatus["packets_processed"])
	pts.logger.Printf("     Active clients: %v", serverStatus["active_clients"])

	// Client A metrics
	clientAStatus := pts.clientA.GetStatus()
	pts.logger.Printf("   Client A (Alice):")
	pts.logger.Printf("     Messages sent: %v", clientAStatus["messages_sent"])
	pts.logger.Printf("     Messages received: %v", clientAStatus["messages_received"])
	pts.logger.Printf("     Friend requests sent: %v", clientAStatus["friend_requests_sent"])
	pts.logger.Printf("     Friend count: %v", clientAStatus["friend_count"])

	// Client B metrics
	clientBStatus := pts.clientB.GetStatus()
	pts.logger.Printf("   Client B (Bob):")
	pts.logger.Printf("     Messages sent: %v", clientBStatus["messages_sent"])
	pts.logger.Printf("     Messages received: %v", clientBStatus["messages_received"])
	pts.logger.Printf("     Friend requests received: %v", clientBStatus["friend_requests_recv"])
	pts.logger.Printf("     Friend count: %v", clientBStatus["friend_count"])
}

// retryOperation performs an operation with exponential backoff retry logic.
func (pts *ProtocolTestSuite) retryOperation(operation func() error) error {
	var lastErr error
	backoff := pts.config.RetryBackoff

	for attempt := 0; attempt < pts.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			pts.logger.Printf("⏳ Retrying operation (attempt %d/%d) after %v delay...",
				attempt+1, pts.config.RetryAttempts, backoff)
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
		pts.logger.Printf("⚠️  Operation failed (attempt %d/%d): %v",
			attempt+1, pts.config.RetryAttempts, err)
	}

	return fmt.Errorf("operation failed after %d attempts: %w", pts.config.RetryAttempts, lastErr)
}

// Cleanup gracefully shuts down all test components.
func (pts *ProtocolTestSuite) Cleanup() error {
	pts.logger.Println("🧹 Cleaning up test resources...")

	var errors []error

	// Stop clients
	if pts.clientA != nil {
		if err := pts.clientA.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop Client A: %w", err))
		}
	}

	if pts.clientB != nil {
		if err := pts.clientB.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop Client B: %w", err))
		}
	}

	// Stop server
	if pts.server != nil {
		if err := pts.server.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop bootstrap server: %w", err))
		}
	}

	if len(errors) > 0 {
		pts.logger.Printf("⚠️  Cleanup completed with %d errors", len(errors))
		for _, err := range errors {
			pts.logger.Printf("   - %v", err)
		}
		return fmt.Errorf("cleanup completed with errors")
	}

	pts.logger.Println("✅ Cleanup completed successfully")
	return nil
}
