# toxcore-go Project Copilot Instructions

## Project Overview

I am a Go programming assistant specializing in the toxcore-go project - a pure Go implementation of the Tox Messenger core protocol designed for secure peer-to-peer communication without relying on centralized infrastructure. This project delivers advanced privacy features and modern cryptographic security while maintaining compatibility and performance.

**Key Features:**
- A clean, idiomatic Go implementation of the Tox protocol with Go 1.23.2
- No CGo dependencies, making it a pure Go solution for cross-platform deployment
- Comprehensive coverage of the Tox protocol features with security enhancements
- Advanced privacy protection through peer identity obfuscation and forward secrecy
- Noise Protocol Framework (IK pattern) integration for enhanced security
- Asynchronous messaging with obfuscated storage nodes to protect metadata
- C binding annotations for cross-language compatibility

## Project Structure

The codebase follows a modular structure with clear separation of concerns, similar to other Go networking projects:

```
toxcore/
├── async/          # Forward-secure asynchronous messaging with obfuscation (15 files)
│   ├── client.go          # AsyncClient for message handling
│   ├── manager.go         # AsyncManager integration layer
│   ├── storage.go         # MessageStorage with capacity management
│   ├── forward_secrecy.go # ForwardSecurityManager for pre-key handling
│   ├── obfs.go           # Identity obfuscation for privacy
│   └── epoch.go          # Epoch-based key rotation
├── transport/      # Network transport with UDP/TCP and Noise protocol support
│   ├── udp.go            # UDP transport implementation
│   ├── tcp.go            # TCP transport with NAT traversal
│   ├── noise_transport.go # Noise-IK protocol integration
│   ├── packet.go         # Packet type definitions
│   └── types.go          # Transport interface definitions
├── crypto/         # Cryptographic operations with secure memory handling
│   ├── encrypt.go        # Encryption operations
│   ├── decrypt.go        # Decryption operations
│   ├── ed25519.go        # Ed25519 signatures
│   ├── keypair.go        # Key pair management
│   ├── secure_memory.go  # Secure memory wiping
│   └── shared_secret.go  # ECDH key derivation
├── dht/            # Distributed Hash Table for peer discovery and routing
│   ├── routing.go        # DHT routing table management
│   ├── bootstrap.go      # Network bootstrap functionality
│   ├── node.go          # DHT node implementation
│   └── maintenance.go   # Periodic DHT maintenance
├── friend/         # Friend management with request handling
│   ├── friend.go         # Friend relationship management
│   └── request.go        # Friend request processing
├── group/          # Group chat functionality with role management
│   └── chat.go          # Group chat implementation
├── messaging/      # Core message handling
│   └── message.go       # Message types and processing
├── file/           # File transfer operations
├── noise/          # Noise Protocol Framework implementation
│   └── handshake.go     # Noise-IK handshake implementation
├── examples/       # Comprehensive demo applications (7 examples)
│   ├── async_demo/          # Async messaging demonstration
│   ├── noise_demo/          # Noise protocol example
│   └── async_obfuscation_demo/ # Privacy features demo
└── docs/           # Extensive technical documentation
    ├── ASYNC.md            # Asynchronous messaging specification
    ├── OBFS.md            # Identity obfuscation design
    └── SECURITY_AUDIT_REPORT.md # Comprehensive security analysis
```

## Core Implementation Focus

When working with this codebase, prioritize these core components:

1. **Asynchronous Messaging System** - Forward-secure messaging with identity obfuscation via the async package
2. **Noise Protocol Integration** - Mutual authentication and KCI resistance through Noise-IK handshakes
3. **DHT Implementation** - Peer discovery and routing via the distributed hash table
4. **Transport Layer** - UDP/TCP communications with NAT traversal and Noise protocol wrapping
5. **Cryptographic Operations** - Ed25519 signatures, Curve25519 key exchange, secure memory handling
6. **Privacy Protection** - Pseudonym-based routing and traffic analysis resistance
7. **Friend System** - Managing friend requests and relationships with forward secrecy
8. **Messaging** - Core functionality for secure message exchange with automatic padding

## Code Style Guidelines

All implementations should adhere to these principles:

1. **Idiomatic Go** - Follow Go conventions and best practices, especially for error handling and interface design
2. **Security-First Development** - Prioritize cryptographic correctness, use secure memory handling patterns from `crypto/secure_memory.go`
3. **Concurrency Safety** - Use goroutines and channels appropriately, ensure thread-safe operations in async messaging
4. **Comprehensive Error Handling** - Implement proper Go-style error handling with context (`fmt.Errorf("operation failed: %w", err)`)
5. **Interface-Based Design** - Use interface types for network operations and transport layers to enhance testability
6. **Resource Management** - Implement proper cleanup with defer statements, secure memory wiping, and connection management

## Networking Best Practices

When declaring network variables, always use interface types:
 - never use net.UDPAddr, net.IPAddr, or net.TCPAddr. Use net.Addr only instead.
 - never use net.UDPConn, use net.PacketConn instead
 - never use net.TCPConn, use net.Conn instead
 - never use net.UDPListener net.TCPLisenter, use net.Listener instead
 - never use a type switch or type assertion to convert from an interface type to a concrete type. Use the interface methods instead.

This approach enhances testability and flexibility when working with different network implementations or mocks.

## Implementation Pattern

The typical usage pattern for toxcore-go follows this structure:

```go
// Create a new Tox instance
options := toxcore.NewOptions()
options.UDPEnabled = true

tox, err := toxcore.New(options)
if err != nil {
    log.Fatal(err)
}
defer tox.Kill()

// Set up callbacks
tox.OnFriendRequest(func(publicKey [32]byte, message string) {
    // Handle friend request logic
})

// Bootstrap to the Tox network
tox.Bootstrap("tox.abiliri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")

// Main iteration loop
for tox.IsRunning() {
    tox.Iterate()
    time.Sleep(tox.IterationInterval())
}
```

All implementations should support this pattern and integrate with the main Tox interface.

## Cryptography Implementation

The crypto package implements advanced cryptographic operations with security-first principles:

**Core Cryptographic Stack:**
- **Ed25519** for digital signatures with secure key handling
- **Curve25519** for elliptic curve Diffie-Hellman key exchange
- **ChaCha20-Poly1305** for authenticated encryption (via Noise protocol)
- **SHA256** for hashing operations and key derivation
- **Secure Memory Management** - Automatic wiping of sensitive data using `crypto/secure_memory.go`

**Advanced Security Features:**
- **Forward Secrecy** - Pre-key system with automatic rotation via `ForwardSecurityManager`
- **Noise-IK Protocol** - Mutual authentication with KCI resistance for enhanced security
- **Identity Obfuscation** - Cryptographic pseudonyms to protect user identities from storage nodes
- **Message Padding** - Automatic padding to standard sizes (256B, 1024B, 4096B) to prevent traffic analysis

Follow the established patterns with constant-time operations and proper key lifecycle management.

## Implementation Considerations

When implementing or reviewing code:

1. **Security** - Prioritize security in all cryptographic operations, implement secure memory handling patterns
2. **Compatibility** - Ensure implementations are compatible with the Tox protocol and network
3. **Performance** - Consider the efficiency of implementations, especially for high-throughput operations
4. **Cross-Platform** - Ensure the code works across different operating systems
5. **Testing** - Write comprehensive unit tests for all functionality (maintain 94% test-to-source ratio)

**Security-Specific Guidelines:**
- Always use secure random number generation for cryptographic operations
- Implement proper key rotation and forward secrecy guarantees
- Protect against timing attacks using constant-time algorithms
- Use the established mock transport pattern for deterministic network testing
- Follow the privacy protection patterns for async messaging with identity obfuscation

## Config Management

Follow established patterns for configuration management:
- Use struct-based configuration options
- Provide sensible defaults
- Allow overriding through explicit API calls
- Consider file-based configuration where appropriate

## C Interoperability

The project provides C bindings for integration with C codebases. When implementing core functionality, consider how it will be exposed through these bindings:

```c
// Example C binding usage
void friend_request_callback(uint8_t* public_key, const char* message, void* user_data) {
    printf("Friend request received: %s\n", message);
    // Handle the friend request
}
```

## Technical Stack & Dependencies

**Language & Version:**
- Go 1.23.2 (minimum required version)
- Pure Go implementation with no CGo dependencies

**Core Dependencies:**
- `golang.org/x/crypto v0.36.0` - Cryptographic primitives and secure implementations
- `github.com/flynn/noise v1.1.0` - Noise Protocol Framework for enhanced security
- `golang.org/x/sys v0.31.0` - System-level operations (indirect dependency)

**Testing Framework:**
- Go's built-in testing package with comprehensive coverage (48 test files for 51 source files)
- Table-driven tests for business logic functions
- Integration tests using mock transport pattern
- Security validation tests for cryptographic operations

## Quality Standards & Testing Requirements

**Testing Standards:**
- Maintain >90% test coverage using Go's built-in testing framework
- Write table-driven tests for all business logic functions using `testify` patterns
- Include integration tests for all network operations using `async/mock_transport.go`
- Security validation tests for all cryptographic operations (see `security_validation_test.go`)
- Performance benchmarks for critical paths (see `*_benchmark_test.go` files)

**Code Quality Requirements:**
- All public APIs must have comprehensive GoDoc comments starting with function names
- Implement comprehensive error context using Go's error wrapping patterns
- Follow established callback patterns for event handling (`OnFriendRequest`, `OnFriendMessage`)
- Use interface-based design for network operations to enhance testability
- Proper resource management with defer statements and secure memory cleanup

**Security Standards:**
- All cryptographic implementations must follow patterns from security audit (`SECURITY_AUDIT_REPORT.md`)
- Implement secure memory handling using `crypto/secure_memory.go` patterns
- Use constant-time algorithms for all cryptographic operations
- Forward secrecy must be maintained throughout message lifecycle
- Privacy protection through identity obfuscation and traffic analysis resistance

## Contributing Guidelines

When contributing to the project:

1. Ensure code follows the project's style and patterns
2. Provide comprehensive documentation for public APIs
3. Include unit tests for new functionality
4. Consider performance and security implications
5. Maintain compatibility with the existing API contract

**Security-Critical Contributions:**
- All changes to cryptographic code require security-focused review
- Network protocol changes must include compatibility testing with existing Tox implementations
- Performance-critical paths should include benchmark tests and profiling
- Public API changes require documentation updates and working example code
- Privacy-related changes must include threat model analysis and validation