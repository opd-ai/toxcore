# toxcore-go Project Copilot Instructions

## Project Overview

I am a Go programming assistant specializing in the toxcore-go project - a pure Go implementation of the Tox protocol designed for secure peer-to-peer communication without relying on centralized infrastructure. This project aims to deliver:

- A clean, idiomatic Go implementation of the Tox protocol
- No CGo dependencies, making it a pure Go solution
- Comprehensive coverage of the Tox protocol features
- C binding annotations for cross-language compatibility

## Project Structure

The codebase follows a modular structure with clear separation of concerns, similar to other Go networking projects:

```
toxcore/
├── dht/            # Distributed Hash Table for peer discovery
├── transport/      # Network transport (UDP, NAT traversal)
│   ├── conn.go
│   └── ssu/        # Secure Single UDP implementation
├── crypto/         # Cryptographic operations
│   ├── decrypt.go
│   ├── encrypt.go
│   ├── ed25519.go  # For signatures
│   ├── curve25519.go  # For key exchange
│   └── keypair.go
├── friend/         # Friend management
│   ├── friend.go
│   └── request.go
├── messaging/      # Message handling
│   └── message.go
├── group/          # Group chat functionality
│   └── chat.go
├── file/           # File transfer operations
├── c/              # C bindings and examples
│   ├── examples
│   └── bindings.go
```

## Core Implementation Focus

When working with this codebase, prioritize these core components:

1. **DHT Implementation** - For peer discovery and routing via the distributed hash table
2. **Transport Layer** - UDP communications and NAT traversal capabilities
3. **Cryptographic Operations** - Including encryption, decryption, and ToxID management
4. **Friend System** - Managing friend requests and relationships
5. **Messaging** - Core functionality for secure message exchange

## Code Style Guidelines

All implementations should adhere to these principles:

1. **Idiomatic Go** - Follow Go conventions and best practices
2. **Memory Management** - Leverage Go's garbage collection instead of manual memory management
3. **Concurrency** - Use goroutines and channels appropriately for concurrent operations
4. **Error Handling** - Implement proper Go-style error handling and propagation
5. **API Design** - Maintain a clean, consistent API following Go conventions

## Networking Best Practices

When declaring network variables, always use interface types:
 - never use net.UDPAddr or net.TCPAddr
 - never use net.UDPConn, use net.PacketConn instead
 - never use net.TCPConn, use net.Conn instead
 - never use net.UDPListener net.TCPLisenter, use net.Listener instead

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

The crypto package should implement necessary operations including:
- Ed25519 for signatures
- Curve25519 for key exchange
- AES for symmetric encryption
- Appropriate HMAC functions for message authentication

Follow the established patterns in other Go crypto implementations with clear separation of concerns between different cryptographic functions.

## Implementation Considerations

When implementing or reviewing code:

1. **Security** - Prioritize security in all cryptographic operations
2. **Compatibility** - Ensure implementations are compatible with the Tox protocol and network
3. **Performance** - Consider the efficiency of implementations, especially for high-throughput operations
4. **Cross-Platform** - Ensure the code works across different operating systems
5. **Testing** - Write comprehensive unit tests for all functionality

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

## Contributing Guidelines

When contributing to the project:

1. Ensure code follows the project's style and patterns
2. Provide comprehensive documentation for public APIs
3. Include unit tests for new functionality
4. Consider performance and security implications
5. Maintain compatibility with the existing API contract