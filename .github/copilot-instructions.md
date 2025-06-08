# toxcore-go Project Copilot Instructions

## Project Overview

toxcore-go is a pure Go implementation of the Tox Messenger core protocol, designed for secure peer-to-peer communication without centralized infrastructure. This project targets developers building secure messaging applications who need a CGo-free, idiomatic Go implementation with comprehensive Tox protocol coverage. The implementation emphasizes clean API design, robust concurrency patterns, and cross-language compatibility through C binding annotations, making it suitable for both Go applications and integration with C codebases.

The project follows modern Go architectural patterns with modular design, leveraging Go's built-in concurrency primitives and garbage collection for memory management. It provides a complete alternative to the original C libtoxcore implementation while maintaining protocol compatibility and extending functionality with Go-native features.

## Technical Stack

- **Primary Language**: Go 1.23.2 with pure Go implementation (no CGo dependencies)
- **Cryptography**: golang.org/x/crypto v0.36.0 for Ed25519 signatures, Curve25519 key exchange, and AES encryption
- **Networking**: Standard library net package with custom UDP/TCP transport layers
- **Concurrency**: Native goroutines and channels for DHT maintenance, message processing, and file transfers
- **Testing**: Go's built-in testing package with table-driven tests and integration test patterns
- **Build/Deploy**: Go modules with standard go build/install process
- **Documentation**: Godoc-compatible comments with C binding export annotations

## Code Assistance Guidelines

1. **Network Interface Patterns**: Always use interface types for network variables - net.Addr instead of concrete types like net.UDPAddr, net.PacketConn instead of net.UDPConn, and net.Conn instead of net.TCPConn. This enhances testability and flexibility with different network implementations or mocks.

2. **Concurrency Safety**: Implement proper mutex protection for shared state like friends maps, message queues, and file transfers. Use sync.RWMutex for read-heavy operations and sync.Mutex for write operations. Follow the pattern of acquiring locks at function entry and deferring unlocks.

3. **Error Handling**: Use Go's idiomatic error handling with descriptive error messages. Return errors from all fallible operations and handle them appropriately at call sites. Avoid panics except for truly exceptional circumstances that indicate programming errors.

4. **Callback Pattern Implementation**: Register callbacks using function type fields in the main Tox struct. Call callbacks with nil checks before invocation. Examples: FriendRequestCallback, FriendMessageCallback, ConnectionStatusCallback following the established patterns.

5. **Export Annotations**: Add `//export FunctionName` comments above public functions intended for C binding compatibility. Follow the naming convention of prefixing exported functions with "Tox" (e.g., ToxNew, ToxIterate, ToxBootstrap).

6. **Packet Processing**: Implement packet handlers as methods that accept *transport.Packet and net.Addr parameters. Register handlers using transport.RegisterHandler() and process packets asynchronously. Follow encryption/decryption patterns for secure communication.

7. **Memory Management**: Rely on Go's garbage collector instead of manual memory management. Use byte slices for binary data, copy sensitive data when necessary, and avoid retaining large objects unnecessarily. Clean up resources in Kill() methods.

## Project Context

- **Domain**: Secure peer-to-peer messaging protocol implementation focusing on privacy, decentralization, and cryptographic security. Core concepts include ToxID (public key + nospam), DHT routing tables, friend relationships, and end-to-end encrypted messaging with file transfer capabilities.

- **Architecture**: Modular design with clear separation between DHT (peer discovery), transport (UDP/TCP networking), crypto (encryption/signatures), friend management, messaging, and file transfers. Each module maintains its own state and communicates through well-defined interfaces.

- **Key Directories**: 
  - `/toxcore.go` - Main API and Tox struct implementation
  - `/dht/` - Distributed Hash Table for peer discovery and routing
  - `/transport/` - Network transport layers (UDP, TCP, packet handling)
  - `/crypto/` - Cryptographic operations (Ed25519, Curve25519, ToxID management)
  - `/friend/` - Friend request management and relationship handling
  - `/file/` - File transfer implementation with chunking and progress tracking

- **Configuration**: Options struct with UDPEnabled, IPv6Enabled, LocalDiscovery flags. Bootstrap node configuration with address, port, and public key. Save/load functionality for persistent state across sessions.

## Quality Standards

- **Testing Requirements**: Maintain comprehensive test coverage using Go's built-in testing package. Write table-driven tests for cryptographic functions and protocol operations. Include integration tests for network communication using local test instances. Test concurrent operations with race condition detection using `go test -race`.

- **Code Review Criteria**: Ensure all public functions have godoc comments with usage examples. Verify proper error handling and resource cleanup. Check for race conditions in concurrent code. Validate network interface usage patterns and cryptographic security. Confirm C export annotations are correctly formatted.

- **Documentation Standards**: Document all public APIs with godoc-compatible comments including parameter descriptions and return values. Provide usage examples in package documentation. Maintain README.md with current installation instructions and basic usage patterns. Update C binding examples when API changes occur.

- **Security Considerations**: Validate all input data sizes and formats before processing. Use constant-time comparisons for cryptographic operations. Implement proper key generation and management. Ensure encrypted communication channels for all peer-to-peer messaging. Clear sensitive data from memory when no longer needed.

## DHT Implementation Patterns

- Use 8-bucket routing table structure following Kademlia principles
- Implement node pinging with exponential backoff for connection maintenance  
- Handle bootstrap node management with timeout and retry logic
- Process get_nodes/send_nodes packets for peer discovery
- Maintain node statistics for connection quality assessment

## File Transfer Protocol

- Implement chunked file transfers with configurable chunk sizes
- Use progress callbacks for transfer status updates
- Support pause/resume/cancel operations on active transfers
- Handle both incoming and outgoing transfer directions
- Implement proper cleanup for completed or cancelled transfers

## Friend Management Patterns

- Generate unique friend IDs using incremental allocation
- Maintain friend state including connection status, name, and status message
- Process friend requests with encryption validation
- Implement friend discovery through DHT lookups
- Support persistent friend relationships across sessions