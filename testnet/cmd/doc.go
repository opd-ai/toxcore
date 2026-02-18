// Package main provides the command-line interface for the Tox network integration test suite.
//
// # Overview
//
// The testnet/cmd package is the executable entry point for validating core Tox protocol
// operations through complete peer-to-peer communication workflows. It orchestrates
// bootstrap server initialization, client management, friend connections, and message
// exchange to ensure protocol correctness.
//
// # Usage
//
// Run with default settings:
//
//	go run ./testnet/cmd
//
// Run with custom configuration:
//
//	go run ./testnet/cmd -port 8080 -connection-timeout 60s -verbose
//
// Run with log file and reduced verbosity:
//
//	go run ./testnet/cmd -log-file test.log -verbose=false
//
// # Configuration Options
//
// Network configuration:
//   - -port: Bootstrap server port (default: 33445)
//   - -address: Bootstrap server address (default: 127.0.0.1)
//
// Timeout configuration:
//   - -overall-timeout: Overall test timeout (default: 5m)
//   - -bootstrap-timeout: Bootstrap server startup timeout (default: 10s)
//   - -connection-timeout: Client connection timeout (default: 30s)
//   - -friend-request-timeout: Friend request timeout (default: 15s)
//   - -message-timeout: Message delivery timeout (default: 10s)
//
// Retry configuration:
//   - -retry-attempts: Number of retry attempts (default: 3)
//   - -retry-backoff: Initial backoff duration (default: 1s)
//
// Logging configuration:
//   - -log-level: Log level (DEBUG, INFO, WARN, ERROR) (default: INFO)
//   - -log-file: Log file path (default: stdout)
//   - -verbose: Enable verbose output (default: true)
//
// Feature flags:
//   - -health-checks: Enable health checks (default: true)
//   - -metrics: Enable metrics collection (default: true)
//
// # Test Workflow
//
// The test suite executes the following workflow:
//
//  1. Bootstrap server initialization on localhost
//  2. Client creation and connection to bootstrap server
//  3. Friend request exchange between test clients
//  4. Bidirectional message delivery verification
//  5. Resource cleanup and results reporting
//
// # Exit Codes
//
//   - 0: All tests passed
//   - 1: Configuration error, test failure, or execution error
//
// # Signal Handling
//
// The test suite handles SIGINT (Ctrl+C) gracefully, initiating proper cleanup
// of all resources including test clients and bootstrap servers.
//
// # Structured Logging
//
// All error conditions are logged using logrus structured logging with contextual
// fields for debugging. Log output includes:
//   - error: Error message
//   - context: Operation context (e.g., configuration_validation, orchestrator_creation)
//   - Additional fields specific to the operation (e.g., bootstrap_port, bootstrap_addr)
//
// # Integration
//
// The package integrates with the following internal components:
//   - testnet/internal.TestOrchestrator: Manages complete test execution workflow
//   - testnet/internal.TestConfig: Maps CLI flags to internal test configuration
//   - testnet/internal.TestResults: Contains test execution results and metrics
//   - testnet/internal.TestStatus: Enum for test pass/fail status determination
package main
