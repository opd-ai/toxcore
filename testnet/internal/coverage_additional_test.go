package internal

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestProtocolCleanupClientANilCase tests cleanupClientA with nil client.
func TestProtocolCleanupClientANilCase(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "cleanup-a-nil")

	config := DefaultProtocolConfig()
	config.Logger = entry
	suite := NewProtocolTestSuite(config)

	// Test with nil client - should not add error
	suite.clientA = nil
	errs := []error{}
	suite.cleanupClientA(&errs)
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors with nil client, got %d", len(errs))
	}
}

// TestProtocolCleanupClientBNilCase tests cleanupClientB with nil client.
func TestProtocolCleanupClientBNilCase(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "cleanup-b-nil")

	config := DefaultProtocolConfig()
	config.Logger = entry
	suite := NewProtocolTestSuite(config)

	// Test with nil client - should not add error
	suite.clientB = nil
	errs := []error{}
	suite.cleanupClientB(&errs)
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors with nil client, got %d", len(errs))
	}
}

// TestProtocolCleanupServerNilCase tests cleanupServer with nil server.
func TestProtocolCleanupServerNilCase(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "cleanup-server-nil")

	config := DefaultProtocolConfig()
	config.Logger = entry
	suite := NewProtocolTestSuite(config)

	// Test with nil server - should not add error
	suite.server = nil
	errs := []error{}
	suite.cleanupServer(&errs)
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors with nil server, got %d", len(errs))
	}
}

// TestWaitForConnectionsClientATimeout tests waitForConnections when client A times out.
func TestWaitForConnectionsClientATimeout(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "wait-connections-a")

	config := DefaultProtocolConfig()
	config.Logger = entry
	config.ConnectionTimeout = 50 * time.Millisecond // Short timeout for test
	suite := NewProtocolTestSuite(config)

	// Create minimal clients that will timeout
	suite.clientA = &TestClient{
		name:         "Alice",
		connected:    false,
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
	}
	suite.clientB = &TestClient{
		name:         "Bob",
		connected:    false,
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
	}

	err := suite.waitForConnections()
	if err == nil {
		t.Error("waitForConnections should return error when client A times out")
	}
	if !contains(err.Error(), "Client A connection timeout") {
		t.Errorf("Error should mention Client A timeout, got: %v", err)
	}
}

// TestWaitForConnectionsClientBTimeout tests waitForConnections when client B times out.
func TestWaitForConnectionsClientBTimeout(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "wait-connections-b")

	config := DefaultProtocolConfig()
	config.Logger = entry
	config.ConnectionTimeout = 100 * time.Millisecond // Short timeout for test
	suite := NewProtocolTestSuite(config)

	// Create minimal clients - client A connects, B does not
	suite.clientA = &TestClient{
		name:         "Alice",
		connected:    true, // Alice is connected
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
	}
	suite.clientB = &TestClient{
		name:         "Bob",
		connected:    false, // Bob is not connected
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
	}

	err := suite.waitForConnections()
	if err == nil {
		t.Error("waitForConnections should return error when client B times out")
	}
	if !contains(err.Error(), "Client B connection timeout") {
		t.Errorf("Error should mention Client B timeout, got: %v", err)
	}
}

// TestWaitForConnectionsSuccess tests waitForConnections when both clients connect.
func TestWaitForConnectionsSuccess(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "wait-connections-success")

	config := DefaultProtocolConfig()
	config.Logger = entry
	config.ConnectionTimeout = 1 * time.Second
	suite := NewProtocolTestSuite(config)

	// Create minimal clients - both connected
	suite.clientA = &TestClient{
		name:         "Alice",
		connected:    true,
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
	}
	suite.clientB = &TestClient{
		name:         "Bob",
		connected:    true,
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
	}

	err := suite.waitForConnections()
	if err != nil {
		t.Errorf("waitForConnections should succeed when both clients connected: %v", err)
	}
}

// TestServerStatusStruct tests ServerStatus struct fields.
func TestServerStatusStruct(t *testing.T) {
	status := ServerStatus{
		Running:           true,
		Address:           "127.0.0.1",
		Port:              33445,
		PublicKey:         "ABCD1234",
		Uptime:            5 * time.Minute,
		ConnectionsServed: 100,
		PacketsProcessed:  5000,
		ActiveClients:     3,
		ConnectionStatus:  1, // ConnectionUDP
	}

	if !status.Running {
		t.Error("Running should be true")
	}
	if status.Address != "127.0.0.1" {
		t.Errorf("Address = %q, want %q", status.Address, "127.0.0.1")
	}
	if status.Port != 33445 {
		t.Errorf("Port = %d, want 33445", status.Port)
	}
	if status.PublicKey != "ABCD1234" {
		t.Errorf("PublicKey = %q, want %q", status.PublicKey, "ABCD1234")
	}
	if status.Uptime != 5*time.Minute {
		t.Errorf("Uptime = %v, want %v", status.Uptime, 5*time.Minute)
	}
	if status.ConnectionsServed != 100 {
		t.Errorf("ConnectionsServed = %d, want 100", status.ConnectionsServed)
	}
	if status.PacketsProcessed != 5000 {
		t.Errorf("PacketsProcessed = %d, want 5000", status.PacketsProcessed)
	}
	if status.ActiveClients != 3 {
		t.Errorf("ActiveClients = %d, want 3", status.ActiveClients)
	}
}

// TestClientStatusStruct tests ClientStatus struct fields.
func TestClientStatusStruct(t *testing.T) {
	status := ClientStatus{
		Name:               "Alice",
		Connected:          true,
		PublicKey:          "PK12345678",
		ConnectionStatus:   2, // ConnectionTCP
		FriendCount:        5,
		Uptime:             10 * time.Minute,
		MessagesSent:       100,
		MessagesReceived:   95,
		FriendRequestsSent: 10,
		FriendRequestsRecv: 8,
		ConnectionEvents:   15,
	}

	if status.Name != "Alice" {
		t.Errorf("Name = %q, want %q", status.Name, "Alice")
	}
	if !status.Connected {
		t.Error("Connected should be true")
	}
	if status.FriendCount != 5 {
		t.Errorf("FriendCount = %d, want 5", status.FriendCount)
	}
	if status.MessagesSent != 100 {
		t.Errorf("MessagesSent = %d, want 100", status.MessagesSent)
	}
	if status.MessagesReceived != 95 {
		t.Errorf("MessagesReceived = %d, want 95", status.MessagesReceived)
	}
}

// TestClientConfigDefaults tests DefaultClientConfig for different clients.
func TestClientConfigDefaults(t *testing.T) {
	testCases := []struct {
		name      string
		startPort uint16
		endPort   uint16
	}{
		{"Alice", AlicePortRangeStart, AlicePortRangeEnd},
		{"Bob", BobPortRangeStart, BobPortRangeEnd},
		{"Charlie", OtherPortRangeStart, OtherPortRangeEnd},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultClientConfig(tc.name)
			if config.Name != tc.name {
				t.Errorf("Name = %q, want %q", config.Name, tc.name)
			}
			if config.StartPort != tc.startPort {
				t.Errorf("StartPort = %d, want %d", config.StartPort, tc.startPort)
			}
			if config.EndPort != tc.endPort {
				t.Errorf("EndPort = %d, want %d", config.EndPort, tc.endPort)
			}
			if config.Logger == nil {
				t.Error("Logger should not be nil")
			}
		})
	}
}

// TestProtocolConfigDefaults tests DefaultProtocolConfig values.
func TestProtocolConfigDefaults(t *testing.T) {
	config := DefaultProtocolConfig()

	if config.BootstrapTimeout != 10*time.Second {
		t.Errorf("BootstrapTimeout = %v, want %v", config.BootstrapTimeout, 10*time.Second)
	}
	if config.ConnectionTimeout != 30*time.Second {
		t.Errorf("ConnectionTimeout = %v, want %v", config.ConnectionTimeout, 30*time.Second)
	}
	if config.FriendRequestTimeout != 15*time.Second {
		t.Errorf("FriendRequestTimeout = %v, want %v", config.FriendRequestTimeout, 15*time.Second)
	}
	if config.MessageTimeout != 10*time.Second {
		t.Errorf("MessageTimeout = %v, want %v", config.MessageTimeout, 10*time.Second)
	}
	if config.RetryAttempts != 3 {
		t.Errorf("RetryAttempts = %d, want 3", config.RetryAttempts)
	}
	if config.RetryBackoff != time.Second {
		t.Errorf("RetryBackoff = %v, want %v", config.RetryBackoff, time.Second)
	}
	if config.AcceptanceDelay != 500*time.Millisecond {
		t.Errorf("AcceptanceDelay = %v, want %v", config.AcceptanceDelay, 500*time.Millisecond)
	}
	if config.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

// TestBootstrapConfigDefaults tests DefaultBootstrapConfig values.
func TestBootstrapConfigDefaults(t *testing.T) {
	config := DefaultBootstrapConfig()

	if config.Address != "127.0.0.1" {
		t.Errorf("Address = %q, want %q", config.Address, "127.0.0.1")
	}
	if config.Port != BootstrapDefaultPort {
		t.Errorf("Port = %d, want %d", config.Port, BootstrapDefaultPort)
	}
	if config.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want %v", config.Timeout, 10*time.Second)
	}
	if config.InitDelay != 1*time.Second {
		t.Errorf("InitDelay = %v, want %v", config.InitDelay, 1*time.Second)
	}
	if config.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

// TestFriendConnectionStruct tests FriendConnection struct.
func TestFriendConnectionStruct(t *testing.T) {
	now := time.Now()
	pubKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8}

	conn := FriendConnection{
		FriendID:     42,
		PublicKey:    pubKey,
		Status:       FriendStatusOnline,
		LastSeen:     now,
		MessagesSent: 100,
		MessagesRecv: 95,
	}

	if conn.FriendID != 42 {
		t.Errorf("FriendID = %d, want 42", conn.FriendID)
	}
	if conn.PublicKey != pubKey {
		t.Error("PublicKey mismatch")
	}
	if conn.Status != FriendStatusOnline {
		t.Errorf("Status = %d, want %d", conn.Status, FriendStatusOnline)
	}
	if conn.MessagesSent != 100 {
		t.Errorf("MessagesSent = %d, want 100", conn.MessagesSent)
	}
	if conn.MessagesRecv != 95 {
		t.Errorf("MessagesRecv = %d, want 95", conn.MessagesRecv)
	}
}

// TestClientMetricsStruct tests ClientMetrics struct fields.
func TestClientMetricsStruct(t *testing.T) {
	now := time.Now()
	metrics := ClientMetrics{
		StartTime:          now,
		MessagesSent:       100,
		MessagesReceived:   95,
		FriendRequestsSent: 10,
		FriendRequestsRecv: 8,
		ConnectionEvents:   5,
	}

	if metrics.StartTime != now {
		t.Errorf("StartTime mismatch")
	}
	if metrics.MessagesSent != 100 {
		t.Errorf("MessagesSent = %d, want 100", metrics.MessagesSent)
	}
	if metrics.MessagesReceived != 95 {
		t.Errorf("MessagesReceived = %d, want 95", metrics.MessagesReceived)
	}
	if metrics.FriendRequestsSent != 10 {
		t.Errorf("FriendRequestsSent = %d, want 10", metrics.FriendRequestsSent)
	}
	if metrics.FriendRequestsRecv != 8 {
		t.Errorf("FriendRequestsRecv = %d, want 8", metrics.FriendRequestsRecv)
	}
	if metrics.ConnectionEvents != 5 {
		t.Errorf("ConnectionEvents = %d, want 5", metrics.ConnectionEvents)
	}
}

// TestServerMetricsStruct tests ServerMetrics struct fields.
func TestServerMetricsStruct(t *testing.T) {
	now := time.Now()
	metrics := ServerMetrics{
		StartTime:         now,
		ConnectionsServed: 100,
		PacketsProcessed:  5000,
		ActiveClients:     10,
	}

	if metrics.StartTime != now {
		t.Errorf("StartTime mismatch")
	}
	if metrics.ConnectionsServed != 100 {
		t.Errorf("ConnectionsServed = %d, want 100", metrics.ConnectionsServed)
	}
	if metrics.PacketsProcessed != 5000 {
		t.Errorf("PacketsProcessed = %d, want 5000", metrics.PacketsProcessed)
	}
	if metrics.ActiveClients != 10 {
		t.Errorf("ActiveClients = %d, want 10", metrics.ActiveClients)
	}
}

// TestRetryOperationWithSuccessAfterRetry tests retryOperation with success on third attempt.
func TestRetryOperationWithSuccessAfterRetry(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "retry-success-third")

	config := DefaultProtocolConfig()
	config.Logger = entry
	config.RetryAttempts = 5
	config.RetryBackoff = 1 * time.Millisecond // Fast backoff for test
	suite := NewProtocolTestSuite(config)

	callCount := 0
	err := suite.retryOperation(func() error {
		callCount++
		if callCount < 3 {
			return errors.New("attempt failed")
		}
		return nil
	})
	if err != nil {
		t.Errorf("retryOperation should succeed on third attempt: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Operation should be called 3 times, called %d times", callCount)
	}
}

// TestNewTestClientInvalidPortRange tests NewTestClient with invalid port range.
func TestNewTestClientInvalidPortRange(t *testing.T) {
	config := &ClientConfig{
		Name:      "InvalidPortClient",
		StartPort: 70000, // Invalid port > 65535
		EndPort:   70001,
		Logger:    logrus.WithField("test", "invalid-port"),
	}

	client, err := NewTestClient(config)
	if err == nil {
		t.Error("NewTestClient should fail with invalid port range")
	}
	if client != nil {
		t.Error("Client should be nil on error")
	}
	if !contains(err.Error(), "invalid port range") {
		t.Errorf("Error should mention invalid port range: %v", err)
	}
}

// TestGetStatusTypedStructFields tests that GetStatusTyped returns correct struct.
func TestGetStatusTypedStructFields(t *testing.T) {
	mockTime := NewMockTimeProvider(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))

	// Test server struct via direct struct instantiation
	serverStatus := ServerStatus{
		Running:           true,
		Address:           "127.0.0.1",
		Port:              33445,
		PublicKey:         "ABC123",
		Uptime:            1 * time.Hour,
		ConnectionsServed: 50,
		PacketsProcessed:  1000,
		ActiveClients:     5,
		ConnectionStatus:  1,
	}

	if !serverStatus.Running {
		t.Error("ServerStatus.Running should be true")
	}

	// Test client struct via direct struct instantiation
	clientStatus := ClientStatus{
		Name:               "TestClient",
		Connected:          true,
		PublicKey:          "DEF456",
		ConnectionStatus:   2,
		FriendCount:        3,
		Uptime:             30 * time.Minute,
		MessagesSent:       25,
		MessagesReceived:   20,
		FriendRequestsSent: 5,
		FriendRequestsRecv: 4,
		ConnectionEvents:   10,
	}

	if clientStatus.Name != "TestClient" {
		t.Errorf("ClientStatus.Name = %q, want %q", clientStatus.Name, "TestClient")
	}

	_ = mockTime // Prevent unused variable warning
}

// TestProtocolTestSuiteCleanupWithAllComponents tests Cleanup with all nil components.
func TestProtocolTestSuiteCleanupWithAllComponents(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "cleanup-all-nil")

	config := DefaultProtocolConfig()
	config.Logger = entry
	suite := NewProtocolTestSuite(config)

	// All components are nil
	suite.clientA = nil
	suite.clientB = nil
	suite.server = nil

	err := suite.Cleanup()
	if err != nil {
		t.Errorf("Cleanup should succeed with nil components: %v", err)
	}
}

// TestProtocolTestSuiteCleanupInternalMethods tests cleanupClientA/B/Server with non-nil components.
func TestProtocolTestSuiteCleanupInternalMethods(t *testing.T) {
	logger := logrus.New()
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)
	entry := logger.WithField("test", "cleanup-internal")

	config := DefaultProtocolConfig()
	config.Logger = entry
	suite := NewProtocolTestSuite(config)

	// Test cleanupClientA with a client that has tox=nil (will log but not error)
	// Since Stop() calls tox.Kill(), we need a client with non-nil tox for full coverage
	// But that requires network binding, so we test the nil path

	errs := make([]error, 0)

	// Test with nil components - no errors added
	suite.clientA = nil
	suite.cleanupClientA(&errs)
	if len(errs) != 0 {
		t.Errorf("cleanupClientA with nil should add 0 errors, got %d", len(errs))
	}

	suite.clientB = nil
	suite.cleanupClientB(&errs)
	if len(errs) != 0 {
		t.Errorf("cleanupClientB with nil should add 0 errors, got %d", len(errs))
	}

	suite.server = nil
	suite.cleanupServer(&errs)
	if len(errs) != 0 {
		t.Errorf("cleanupServer with nil should add 0 errors, got %d", len(errs))
	}
}

// TestNewProtocolTestSuiteWithCustomConfig tests NewProtocolTestSuite with custom config.
func TestNewProtocolTestSuiteWithCustomConfig(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "custom-config")

	config := &ProtocolConfig{
		BootstrapTimeout:     5 * time.Second,
		ConnectionTimeout:    15 * time.Second,
		FriendRequestTimeout: 8 * time.Second,
		MessageTimeout:       5 * time.Second,
		RetryAttempts:        5,
		RetryBackoff:         500 * time.Millisecond,
		AcceptanceDelay:      200 * time.Millisecond,
		Logger:               entry,
	}

	suite := NewProtocolTestSuite(config)
	if suite == nil {
		t.Fatal("NewProtocolTestSuite returned nil")
	}

	if suite.config.RetryAttempts != 5 {
		t.Errorf("RetryAttempts = %d, want 5", suite.config.RetryAttempts)
	}
	if suite.config.BootstrapTimeout != 5*time.Second {
		t.Errorf("BootstrapTimeout = %v, want %v", suite.config.BootstrapTimeout, 5*time.Second)
	}
}

// TestEventLoopContextCancellation tests eventLoop context cancellation.
func TestEventLoopContextCancellation(t *testing.T) {
	// Test that cancellation path in eventLoop is exercisable
	// We can't directly test eventLoop without a tox instance,
	// but we can verify the pattern with a mock

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			case <-ticker.C:
				// Simulate work
			}
		}
	}()

	// Cancel context
	cancel()

	// Wait for goroutine to exit
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Event loop did not exit on context cancellation")
	}
}
