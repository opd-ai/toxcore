// Package internal provides the core components for the Tox network integration test suite.
//
// This package implements a comprehensive test harness that validates core Tox protocol
// operations through complete peer-to-peer communication workflows, including bootstrap
// server initialization, client management, and protocol validation.
//
// # Architecture Overview
//
// The test infrastructure consists of four main components that work together:
//
//   - TestOrchestrator: Manages the complete test execution workflow including
//     configuration, logging, result collection, and error reporting
//   - BootstrapServer: Provides a localhost bootstrap node for test network initialization
//   - TestClient: Wraps Tox client instances with callback channels for test validation
//   - ProtocolTestSuite: Coordinates the actual protocol validation test scenarios
//
// # Test Orchestration
//
// The TestOrchestrator is the entry point for running integration tests. It handles:
//
//   - Configuration validation and defaults
//   - Log file management and rotation
//   - Test execution with configurable timeouts and retries
//   - Result aggregation and reporting
//   - Cleanup of resources on completion or failure
//
// Example orchestrator usage:
//
//	config := internal.DefaultTestConfig()
//	config.LogFile = "/tmp/toxtest.log"
//	config.VerboseOutput = true
//
//	orchestrator, err := internal.NewTestOrchestrator(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer orchestrator.Cleanup()
//
//	results, err := orchestrator.ExecuteTest(ctx)
//
// # Bootstrap Server
//
// BootstrapServer creates a local DHT bootstrap node that test clients connect to
// for initial network discovery. It tracks connection metrics and provides a
// controlled environment for integration testing without depending on public nodes.
//
// Example bootstrap server usage:
//
//	config := internal.DefaultBootstrapConfig()
//	config.Port = 33445
//
//	server, err := internal.NewBootstrapServer(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer server.Stop()
//
//	if err := server.Start(); err != nil {
//	    log.Fatal(err)
//	}
//
//	publicKey := server.PublicKey()
//
// # Test Clients
//
// TestClient wraps a Tox instance with channel-based callbacks for test validation.
// This enables synchronous waiting on asynchronous events like friend requests
// and messages in test code.
//
// Example client usage:
//
//	config := internal.DefaultClientConfig("Alice")
//
//	client, err := internal.NewTestClient(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Stop()
//
//	// Wait for friend request on channel
//	select {
//	case req := <-client.FriendRequestChan():
//	    client.AcceptFriendRequest(req.PublicKey)
//	case <-ctx.Done():
//	    log.Fatal("timeout waiting for friend request")
//	}
//
// # Protocol Test Suite
//
// ProtocolTestSuite orchestrates complete peer-to-peer communication workflows:
//
//  1. Network initialization with bootstrap server
//  2. Client creation and DHT connection
//  3. Friend request exchange between clients
//  4. Bidirectional message delivery validation
//  5. Connection state verification
//  6. Cleanup and resource release
//
// Example protocol suite usage:
//
//	config := internal.DefaultProtocolConfig()
//	config.ConnectionTimeout = 30 * time.Second
//
//	suite := internal.NewProtocolTestSuite(config)
//	defer suite.Cleanup()
//
//	if err := suite.ExecuteTest(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// # Configuration
//
// Each component has a default configuration that can be customized:
//
//   - DefaultTestConfig: Overall test orchestration settings
//   - DefaultBootstrapConfig: Bootstrap server port, address, timeouts
//   - DefaultClientConfig: Test client port ranges, callbacks, logging
//   - DefaultProtocolConfig: Protocol test timeouts, retries, logging
//
// # Port Ranges
//
// Test clients use distinct port ranges to avoid conflicts:
//
//   - Bootstrap server: 33445 (default Tox port)
//   - Alice clients: 33500-33599
//   - Bob clients: 33600-33699
//   - Additional clients: 33700-33799
//
// # Metrics Collection
//
// Both BootstrapServer and TestClient collect metrics during test execution:
//
//   - ServerMetrics: ConnectionsServed, PacketsProcessed, ActiveClients
//   - ClientMetrics: MessagesSent, MessagesReceived, ConnectionAttempts
//
// These metrics help diagnose test failures and validate protocol behavior.
//
// # Thread Safety
//
// All components are safe for concurrent use. Internal state is protected by
// sync.RWMutex and sync.Mutex as appropriate. Callback channels have buffers
// to prevent blocking during high-frequency events.
//
// # Integration with toxcore
//
// The testnet package is a separate Go module that imports toxcore via a
// replace directive in go.mod. This allows testing the parent module without
// circular dependencies. The package validates:
//
//   - DHT bootstrap and peer discovery
//   - Friend request protocol (send/accept/reject)
//   - Message delivery (unicast)
//   - Connection state management
//   - Offline/online transitions
//
// # Logging
//
// The package uses standard library logging with configurable verbosity.
// Log output includes timestamps, test step names, and status indicators
// (emoji) for easy visual parsing of test progress.
//
// # Error Handling
//
// Test failures are collected and reported at the end of execution rather
// than immediately aborting. This allows complete test coverage even when
// some steps fail. Critical failures (bootstrap server crash, client panic)
// do abort immediately with detailed error context.
package internal
