# Tox Network Integration Test Suite

A comprehensive test harness that validates core Tox protocol operations through complete peer-to-peer communication workflows.

## Overview

This test suite implements a complete integration test environment for the Tox protocol, including:

- **Bootstrap Server Module**: Localhost server initialization, network coordination, client registration
- **Client Module**: Client instance management, connection handling, state tracking  
- **Protocol Module**: Friend request operations, message exchange, delivery confirmation
- **Test Orchestration**: Workflow coordination, validation checks, result reporting

## Architecture

The test suite is organized with clear separation of concerns:

```
testnet/
├── cmd/              # Command-line executable
│   └── main.go       # CLI interface and main entry point
├── internal/         # Internal test modules
│   ├── bootstrap.go  # Bootstrap server implementation
│   ├── client.go     # Test client implementation
│   ├── protocol.go   # Protocol test workflow
│   └── orchestrator.go # Test orchestration and reporting
├── go.mod           # Go module configuration
└── README.md        # This file
```

## Test Workflow

The integration test executes the following sequence with validation at each step:

### 1. Network Initialization
- Start a bootstrap server on localhost (configurable port)
- Verify server is accepting connections
- Log server configuration and status

### 2. Client Setup  
- Create two client instances (Client A "Alice" and Client B "Bob")
- Connect both clients to the bootstrap node
- Confirm network formation and peer discovery

### 3. Friend Connection
- Client A sends friend request to Client B (include request message)
- Client B receives and accepts the request
- Verify bidirectional friend status

### 4. Message Exchange
- Client B sends initial message to Client A
- Verify Client A receives the message
- Client A sends reply to Client B
- Confirm successful delivery of reply

## Usage

### Basic Usage

Run the test suite with default settings:

```bash
cd testnet/cmd
go run main.go
```

### Advanced Usage

Run with custom configuration:

```bash
# Custom port and timeouts
go run main.go -port 8080 -connection-timeout 60s -verbose

# Log to file with reduced verbosity
go run main.go -log-file test.log -verbose=false

# Adjust retry behavior
go run main.go -retry-attempts 5 -retry-backoff 2s
```

### Command-Line Options

```
Network Configuration:
  -port uint            Bootstrap server port (default 33445)
  -address string       Bootstrap server address (default "127.0.0.1")

Timeout Configuration:
  -overall-timeout duration         Overall test timeout (default 5m0s)
  -bootstrap-timeout duration       Bootstrap server startup timeout (default 10s)
  -connection-timeout duration      Client connection timeout (default 30s)
  -friend-request-timeout duration  Friend request timeout (default 15s)
  -message-timeout duration         Message delivery timeout (default 10s)

Retry Configuration:
  -retry-attempts int       Number of retry attempts for operations (default 3)
  -retry-backoff duration   Initial backoff duration for retries (default 1s)

Logging Configuration:
  -log-level string    Log level (DEBUG, INFO, WARN, ERROR) (default "INFO")
  -log-file string     Log file path (default: stdout)
  -verbose             Enable verbose output (default true)

Feature Flags:
  -health-checks       Enable health checks (default true)
  -metrics             Enable metrics collection (default true)
  -help                Show help message
```

## Building

Build the executable:

```bash
cd testnet/cmd
go build -o toxtest main.go
./toxtest -help
```

## Example Output

```
🚀 Starting Tox Network Integration Test Suite...

🧪 Tox Network Integration Test Suite
=====================================
⏰ Test execution started at 2025-09-06T10:30:00Z

📋 Test Configuration:
   Bootstrap: 127.0.0.1:33445
   Overall timeout: 5m0s
   Bootstrap timeout: 10s
   Connection timeout: 30s
   Friend request timeout: 15s
   Message timeout: 10s
   Retry attempts: 3
   Retry backoff: 1s
   Health checks: true
   Metrics collection: true

📡 Step 1: Network Initialization
Starting bootstrap server on 127.0.0.1:33445
Public key: A1B2C3D4E5F6789012345678901234567890ABCDEF1234567890ABCDEF123456
✅ Bootstrap server started successfully
✅ Bootstrap server running on 127.0.0.1:33445

👥 Step 2: Client Setup
[Alice] Starting client...
[Alice] Public key: 1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF
[Alice] ✅ Client started successfully
[Bob] Starting client...
[Bob] Public key: ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890
[Bob] ✅ Client started successfully
[Alice] Connecting to bootstrap 127.0.0.1:33445
[Bob] Connecting to bootstrap 127.0.0.1:33445
[Alice] ✅ Connected to network
[Bob] ✅ Connected to network
✅ Both clients connected to network

🤝 Step 3: Friend Connection
📤 Alice sending friend request to Bob...
[Alice] Sending friend request: Hello! This is a test friend request from Alice.
[Alice] ✅ Friend request sent (ID: 0)
⏳ Waiting for Bob to receive friend request...
[Bob] Received friend request: Hello! This is a test friend request from Alice.
📨 Bob received friend request: Hello! This is a test friend request from Alice.
✅ Bob accepting friend request...
[Bob] Accepting friend request from 1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF
[Bob] ✅ Friend request accepted (ID: 0)
✅ Bidirectional friend relationship established
   Alice friends: 1, Bob friends: 1

💬 Step 4: Message Exchange
📤 Bob sending message to Alice: Hello Alice! This is Bob's first message.
[Bob] Sending message to friend 0: Hello Alice! This is Bob's first message.
[Bob] ✅ Message sent to friend 0
⏳ Waiting for Alice to receive message...
[Alice] Received message from friend 0: Hello Alice! This is Bob's first message.
✅ Alice received message: Hello Alice! This is Bob's first message.
📤 Alice sending reply to Bob: Hi Bob! This is Alice's reply message.
[Alice] Sending message to friend 0: Hi Bob! This is Alice's reply message.
[Alice] ✅ Message sent to friend 0
⏳ Waiting for Bob to receive reply...
[Bob] Received message from friend 0: Hi Bob! This is Alice's reply message.
✅ Bob received reply: Hi Bob! This is Alice's reply message.

📊 Final Test Metrics:
   Bootstrap Server:
     Uptime: 45.123s
     Packets processed: 156
     Active clients: 2
   Client A (Alice):
     Messages sent: 1
     Messages received: 1
     Friend requests sent: 1
     Friend count: 1
   Client B (Bob):
     Messages sent: 1
     Messages received: 1
     Friend requests received: 1
     Friend count: 1
✅ Message exchange completed successfully

🧹 Cleaning up test resources...
[Alice] Stopping client...
[Alice] ✅ Client stopped after 45.234s uptime
[Bob] Stopping client...
[Bob] ✅ Client stopped after 45.245s uptime
Stopping bootstrap server...
✅ Bootstrap server stopped after 45.256s uptime
✅ Cleanup completed successfully

🎉 All tests completed successfully!

📊 Test Execution Summary
========================
🎯 Overall Status: PASSED
⏱️  Total Execution Time: 45.3s
📈 Tests: 1 total, 1 passed, 0 failed, 0 skipped

📋 Step Details:
   ✅ Complete Protocol Test (45.2s)

🎉 All tests completed successfully!
✅ Tox protocol validation: PASSED
✅ Network connectivity: VERIFIED
✅ Friend requests: WORKING
✅ Message delivery: CONFIRMED

🏁 Test run completed at 2025-09-06T10:30:45Z
==================================================

🎉 Test suite completed successfully!

📊 Summary: 1 tests, 1 passed, 0 failed (execution time: 45.3s)
```

## Technical Features

- **Interface-based Design**: Clean separation between modules using Go interfaces
- **Timeout Handling**: Comprehensive timeout management for all network operations
- **Retry Logic**: Exponential backoff retry mechanism for resilient operation
- **Health Checks**: Built-in validation for server and client states
- **Metrics Collection**: Performance and timing metrics for all operations
- **Graceful Shutdown**: Signal handling for clean test termination
- **Structured Logging**: Detailed logs with timestamps and participant tracking
- **Configuration Flexibility**: Extensive command-line configuration options

## Error Handling

The test suite includes comprehensive error handling with:

- Detailed error messages with context
- Retry mechanisms with exponential backoff
- Graceful degradation on partial failures
- Clear diagnostic information in reports
- Proper resource cleanup on errors

## Development

To extend the test suite:

1. Add new test modules in the `internal/` directory
2. Implement the appropriate interfaces for integration
3. Update the test orchestrator to include new test steps
4. Add corresponding CLI options as needed

The codebase follows Go best practices with proper error handling, interface design, and comprehensive documentation.
