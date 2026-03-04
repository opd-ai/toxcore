package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// ProtocolTestSuite manages the core Tox protocol test workflow.
type ProtocolTestSuite struct {
	server  *BootstrapServer
	clientA *TestClient
	clientB *TestClient
	logger  *logrus.Entry
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
	AcceptanceDelay      time.Duration // Delay after friend request acceptance for processing
	Logger               *logrus.Entry
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
		AcceptanceDelay:      500 * time.Millisecond, // Default delay for friend acceptance
		Logger:               logrus.WithField("component", "protocol"),
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
	pts.logger.Info("🚀 Starting Tox Network Integration Test Suite")
	pts.logger.Info("=" + fmt.Sprintf("%50s", "="))

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

	pts.logger.Info("🎉 All tests completed successfully!")
	return nil
}

// initializeNetwork sets up and validates the bootstrap server.
func (pts *ProtocolTestSuite) initializeNetwork(ctx context.Context) error {
	pts.logger.Info("📡 Step 1: Network Initialization")

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
	pts.logger.WithFields(logrus.Fields{
		"address":           status["address"],
		"port":              status["port"],
		"public_key":        status["public_key"],
		"connection_status": status["connection_status"],
	}).Info("✅ Bootstrap server running")

	return nil
}

// setupClients creates and connects both test clients.
func (pts *ProtocolTestSuite) setupClients(ctx context.Context) error {
	pts.logger.Info("👥 Step 2: Client Setup")

	if err := pts.createClients(); err != nil {
		return err
	}

	if err := pts.startClients(ctx); err != nil {
		return err
	}

	if err := pts.connectClientsToBootstrap(); err != nil {
		return err
	}

	if err := pts.waitForConnections(); err != nil {
		return fmt.Errorf("failed to establish network connections: %w", err)
	}

	pts.logger.Info("✅ Both clients connected to network")
	return nil
}

// createClients initializes both test clients with default configurations.
func (pts *ProtocolTestSuite) createClients() error {
	configA := DefaultClientConfig("Alice")
	configA.Logger = pts.logger
	clientA, err := NewTestClient(configA)
	if err != nil {
		return fmt.Errorf("failed to create Client A: %w", err)
	}
	pts.clientA = clientA

	configB := DefaultClientConfig("Bob")
	configB.Logger = pts.logger
	clientB, err := NewTestClient(configB)
	if err != nil {
		return fmt.Errorf("failed to create Client B: %w", err)
	}
	pts.clientB = clientB

	return nil
}

// startClients starts both test clients.
func (pts *ProtocolTestSuite) startClients(ctx context.Context) error {
	if err := pts.clientA.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Client A: %w", err)
	}

	if err := pts.clientB.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Client B: %w", err)
	}

	return nil
}

// connectClientsToBootstrap connects both clients to the bootstrap server.
func (pts *ProtocolTestSuite) connectClientsToBootstrap() error {
	if err := pts.connectClientToBootstrap(pts.clientA); err != nil {
		return fmt.Errorf("failed to connect Client A to bootstrap: %w", err)
	}

	if err := pts.connectClientToBootstrap(pts.clientB); err != nil {
		return fmt.Errorf("failed to connect Client B to bootstrap: %w", err)
	}

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
	pts.logger.Info("🤝 Step 3: Friend Connection")

	friendRequest, err := pts.sendAndReceiveFriendRequest()
	if err != nil {
		return err
	}

	if err := pts.acceptFriendRequest(friendRequest); err != nil {
		return err
	}

	return pts.verifyBidirectionalConnection()
}

// sendAndReceiveFriendRequest sends a friend request from client A to B and waits for receipt.
func (pts *ProtocolTestSuite) sendAndReceiveFriendRequest() (*FriendRequest, error) {
	clientBPublicKey := pts.clientB.GetPublicKey()
	requestMessage := "Hello! This is a test friend request from Alice."

	pts.logger.Info("📤 Alice sending friend request to Bob...")
	if _, err := pts.clientA.SendFriendRequest(clientBPublicKey, requestMessage); err != nil {
		return nil, fmt.Errorf("failed to send friend request: %w", err)
	}

	pts.logger.Info("⏳ Waiting for Bob to receive friend request...")
	friendRequest, err := pts.clientB.WaitForFriendRequest(pts.config.FriendRequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("Client B did not receive friend request: %w", err)
	}

	pts.logger.WithField("message", friendRequest.Message).Info("📨 Bob received friend request")

	clientAPublicKey := pts.clientA.GetPublicKey()
	if friendRequest.PublicKey != clientAPublicKey {
		return nil, fmt.Errorf("friend request from unexpected client")
	}

	return friendRequest, nil
}

// acceptFriendRequest accepts the pending friend request and waits for processing.
func (pts *ProtocolTestSuite) acceptFriendRequest(friendRequest *FriendRequest) error {
	pts.logger.Info("✅ Bob accepting friend request...")
	if _, err := pts.clientB.AcceptFriendRequest(friendRequest.PublicKey); err != nil {
		return fmt.Errorf("failed to accept friend request: %w", err)
	}

	if pts.config.AcceptanceDelay > 0 {
		time.Sleep(pts.config.AcceptanceDelay)
	}

	return nil
}

// verifyBidirectionalConnection confirms both clients see each other as friends.
func (pts *ProtocolTestSuite) verifyBidirectionalConnection() error {
	clientAFriends := pts.clientA.GetFriends()
	clientBFriends := pts.clientB.GetFriends()

	if len(clientAFriends) == 0 {
		return fmt.Errorf("Client A has no friends after connection")
	}

	if len(clientBFriends) == 0 {
		return fmt.Errorf("Client B has no friends after connection")
	}

	pts.logger.WithFields(logrus.Fields{
		"alice_friends": len(clientAFriends),
		"bob_friends":   len(clientBFriends),
	}).Info("✅ Bidirectional friend relationship established")

	return nil
}

// testMessageExchange tests bidirectional message communication.
func (pts *ProtocolTestSuite) testMessageExchange(ctx context.Context) error {
	pts.logger.Info("💬 Step 4: Message Exchange")

	friendIDA, friendIDB, err := pts.getFriendIDsForMessaging()
	if err != nil {
		return fmt.Errorf("failed to get friend IDs: %w", err)
	}

	if err := pts.testBobSendsToAlice(friendIDB); err != nil {
		return err
	}

	if err := pts.testAliceSendsToBob(friendIDA); err != nil {
		return err
	}

	// Verify delivery metrics
	pts.logFinalMetrics()

	pts.logger.Info("✅ Message exchange completed successfully")
	return nil
}

// getFriendIDsForMessaging retrieves the friend IDs for both clients.
func (pts *ProtocolTestSuite) getFriendIDsForMessaging() (friendIDA, friendIDB uint32, err error) {
	clientAFriends := pts.clientA.GetFriends()
	clientBFriends := pts.clientB.GetFriends()

	for id := range clientAFriends {
		friendIDA = id
		break
	}

	for id := range clientBFriends {
		friendIDB = id
		break
	}

	return friendIDA, friendIDB, nil
}

// sendAndVerifyMessage sends a message from one client to another and validates delivery.
// This helper consolidates the common message exchange pattern used by both testBobSendsToAlice
// and testAliceSendsToBob.
func (pts *ProtocolTestSuite) sendAndVerifyMessage(
	sender *TestClient,
	receiver *TestClient,
	friendID uint32,
	message string,
	senderName string,
	receiverName string,
) error {
	pts.logger.WithField("message", message).Info("📤 " + senderName + " sending message to " + receiverName)

	err := sender.SendMessage(friendID, message)
	if err != nil {
		return fmt.Errorf("%s failed to send message: %w", senderName, err)
	}

	pts.logger.Info("⏳ Waiting for " + receiverName + " to receive message...")
	receivedMsg, err := receiver.WaitForMessage(pts.config.MessageTimeout)
	if err != nil {
		return fmt.Errorf("%s did not receive message: %w", receiverName, err)
	}

	if receivedMsg.Content != message {
		return fmt.Errorf("message content mismatch: expected %q, got %q", message, receivedMsg.Content)
	}

	pts.logger.WithField("message", receivedMsg.Content).Info("✅ " + receiverName + " received message")
	return nil
}

// testBobSendsToAlice tests Bob sending an initial message to Alice and validates delivery.
func (pts *ProtocolTestSuite) testBobSendsToAlice(friendIDB uint32) error {
	return pts.sendAndVerifyMessage(
		pts.clientB,
		pts.clientA,
		friendIDB,
		"Hello Alice! This is Bob's first message.",
		"Bob",
		"Alice",
	)
}

// testAliceSendsToBob tests Alice sending a reply message to Bob and validates delivery.
func (pts *ProtocolTestSuite) testAliceSendsToBob(friendIDA uint32) error {
	return pts.sendAndVerifyMessage(
		pts.clientA,
		pts.clientB,
		friendIDA,
		"Hi Bob! This is Alice's reply message.",
		"Alice",
		"Bob",
	)
}

// logFinalMetrics outputs final test metrics and status.
func (pts *ProtocolTestSuite) logFinalMetrics() {
	pts.logger.Info("📊 Final Test Metrics:")

	// Server metrics
	serverStatus := pts.server.GetStatus()
	pts.logger.WithFields(logrus.Fields{
		"uptime":            serverStatus["uptime"],
		"packets_processed": serverStatus["packets_processed"],
		"active_clients":    serverStatus["active_clients"],
	}).Info("Bootstrap Server metrics")

	// Client A metrics
	clientAStatus := pts.clientA.GetStatus()
	pts.logger.WithFields(logrus.Fields{
		"messages_sent":        clientAStatus["messages_sent"],
		"messages_received":    clientAStatus["messages_received"],
		"friend_requests_sent": clientAStatus["friend_requests_sent"],
		"friend_count":         clientAStatus["friend_count"],
	}).Info("Client A (Alice) metrics")

	// Client B metrics
	clientBStatus := pts.clientB.GetStatus()
	pts.logger.WithFields(logrus.Fields{
		"messages_sent":        clientBStatus["messages_sent"],
		"messages_received":    clientBStatus["messages_received"],
		"friend_requests_recv": clientBStatus["friend_requests_recv"],
		"friend_count":         clientBStatus["friend_count"],
	}).Info("Client B (Bob) metrics")
}

// retryOperation performs an operation with exponential backoff retry logic.
func (pts *ProtocolTestSuite) retryOperation(operation func() error) error {
	var lastErr error
	backoff := pts.config.RetryBackoff

	for attempt := 0; attempt < pts.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			pts.logger.WithFields(logrus.Fields{
				"attempt":      attempt + 1,
				"max_attempts": pts.config.RetryAttempts,
				"backoff":      backoff,
			}).Info("⏳ Retrying operation")
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
		pts.logger.WithFields(logrus.Fields{
			"attempt":      attempt + 1,
			"max_attempts": pts.config.RetryAttempts,
			"error":        err,
		}).Warn("⚠️  Operation failed")
	}

	return fmt.Errorf("operation failed after %d attempts: %w", pts.config.RetryAttempts, lastErr)
}

// Cleanup gracefully shuts down all test components.
func (pts *ProtocolTestSuite) Cleanup() error {
	pts.logger.Info("🧹 Cleaning up test resources...")

	var errors []error

	// Stop all components and collect any errors
	pts.cleanupClientA(&errors)
	pts.cleanupClientB(&errors)
	pts.cleanupServer(&errors)

	return pts.reportCleanupResults(errors)
}

// cleanupClientA stops client A and records any errors.
func (pts *ProtocolTestSuite) cleanupClientA(errors *[]error) {
	if pts.clientA != nil {
		if err := pts.clientA.Stop(); err != nil {
			*errors = append(*errors, fmt.Errorf("failed to stop Client A: %w", err))
		}
	}
}

// cleanupClientB stops client B and records any errors.
func (pts *ProtocolTestSuite) cleanupClientB(errors *[]error) {
	if pts.clientB != nil {
		if err := pts.clientB.Stop(); err != nil {
			*errors = append(*errors, fmt.Errorf("failed to stop Client B: %w", err))
		}
	}
}

// cleanupServer stops the bootstrap server and records any errors.
func (pts *ProtocolTestSuite) cleanupServer(errors *[]error) {
	if pts.server != nil {
		if err := pts.server.Stop(); err != nil {
			*errors = append(*errors, fmt.Errorf("failed to stop bootstrap server: %w", err))
		}
	}
}

// reportCleanupResults logs cleanup results and returns appropriate error.
func (pts *ProtocolTestSuite) reportCleanupResults(errors []error) error {
	if len(errors) > 0 {
		pts.logger.WithField("error_count", len(errors)).Warn("⚠️  Cleanup completed with errors")
		for _, err := range errors {
			pts.logger.WithError(err).Warn("Cleanup error")
		}
		return fmt.Errorf("cleanup completed with errors")
	}

	pts.logger.Info("✅ Cleanup completed successfully")
	return nil
}
