# toxcore-go Project Copilot Instructions

## Project Overview

toxcore-go is a pure Go implementation of the Tox Messenger core protocol - a peer-to-peer, encrypted messaging system designed for secure communications without relying on centralized infrastructure. This implementation emphasizes security, privacy, and performance while providing comprehensive Tox protocol functionality with modern cryptographic enhancements.

**Primary Purpose**: Deliver a clean, idiomatic Go implementation of the Tox protocol with advanced privacy features including Noise-IK protocol integration and asynchronous messaging with identity obfuscation.

**Target Audience**: Developers building secure messaging applications, privacy-focused communication tools, and peer-to-peer systems requiring robust cryptographic guarantees and decentralized architecture.

**Key Differentiators**: 
- No CGo dependencies for pure Go deployment
- Advanced privacy features through peer identity obfuscation 
- Noise Protocol Framework (IK pattern) for enhanced security
- Asynchronous messaging with forward secrecy
- C binding annotations for cross-language compatibility

## Technical Stack

- **Primary Language**: Go 1.23.2
- **Core Dependencies**: 
  - `golang.org/x/crypto v0.36.0` (cryptographic primitives)
  - `github.com/flynn/noise v1.1.0` (Noise Protocol Framework implementation)
- **Testing**: Go's built-in testing package with comprehensive coverage (48 test files for 51 source files)
- **Cryptography**: Ed25519 signatures, Curve25519 key exchange, ChaCha20-Poly1305 AEAD, SHA256 hashing
- **Network Protocols**: UDP/TCP transport with NAT traversal, Noise-IK handshakes for mutual authentication
- **Build/Deploy**: Standard Go tooling, no external build dependencies

## Code Assistance Guidelines

1. **Cryptographic Security First**: Always prioritize cryptographic correctness over performance. Use secure memory handling (`crypto/secure_memory.go` patterns), implement proper key rotation, and ensure forward secrecy guarantees. All cryptographic operations must use constant-time algorithms and secure random number generation.

2. **Network Interface Abstraction**: Always use interface types for network operations - `net.Addr` instead of `*net.UDPAddr`, `net.PacketConn` instead of `*net.UDPConn`, `net.Conn` instead of `*net.TCPConn`. This enhances testability and supports the project's mock transport pattern used throughout the test suite.

3. **Async Messaging Pattern**: Follow the established asynchronous messaging architecture with identity obfuscation. Use the `AsyncManager` for coordination, implement proper pre-key exchange, and ensure all messages use pseudonym-based routing to protect sender/recipient identities from storage nodes.

4. **Error Handling Standards**: Implement comprehensive error context using Go's error wrapping (`fmt.Errorf("operation failed: %w", err)`). Provide actionable error messages for users and detailed diagnostic information for developers. Never silently ignore errors, especially in cryptographic operations.

5. **Testing Requirements**: Write table-driven tests for all business logic, include integration tests for network operations using mock transports, and maintain the current 94% test-to-source file ratio. Every public API must have corresponding test coverage with security validation.

6. **Callback-Based Event Handling**: Use the established callback pattern for user interactions (`OnFriendRequest`, `OnFriendMessage`, etc.). Ensure callbacks are goroutine-safe and provide proper error handling. Follow the signature patterns in `toxcore.go` for consistency.

7. **Resource Management**: Implement proper cleanup patterns with `defer` statements, ensure all network connections and file handles are properly closed, and follow the `Kill()` method pattern for graceful shutdown. Memory-sensitive operations should use secure wiping of cryptographic material.

## Project Context

- **Domain**: Secure peer-to-peer messaging with advanced privacy features including identity obfuscation, forward secrecy, and resistance to storage node surveillance. The system implements a distributed hash table for peer discovery and supports both online and offline message delivery.

- **Architecture**: Modular design with clear separation between transport layer (`transport/`), cryptographic operations (`crypto/`), distributed hash table (`dht/`), friend management (`friend/`), group communications (`group/`), and asynchronous messaging (`async/`). The noise package provides Noise-IK protocol implementation for enhanced security.

- **Key Directories**:
  - `async/`: Forward-secure asynchronous messaging with obfuscation (15 files)
  - `transport/`: Network transport layer with UDP/TCP and Noise protocol support
  - `crypto/`: Cryptographic primitives and secure memory handling
  - `dht/`: Distributed hash table for peer discovery and routing
  - `examples/`: Comprehensive demo applications showing proper usage patterns
  - `docs/`: Extensive technical documentation including security specifications

- **Configuration**: Use struct-based configuration with the `Options` pattern. Provide sensible defaults for security settings while allowing advanced users to customize behavior. Critical settings include UDP/TCP transport selection, DHT bootstrap nodes, and privacy protection levels.

## Quality Standards

- **Testing Requirements**: Maintain >90% test coverage using Go's built-in testing framework. Write comprehensive unit tests for all cryptographic operations, integration tests for network protocols, and security validation tests for privacy features. Use mock transports (`async/mock_transport.go`) for deterministic network testing.

- **Security Standards**: All cryptographic implementations must follow the patterns established in the security audit (see `SECURITY_AUDIT_REPORT.md`). Implement secure memory handling, proper key rotation, and protection against timing attacks. Forward secrecy must be maintained throughout the message lifecycle.

- **Code Review Requirements**: All changes to cryptographic code require security-focused review. Network protocol changes must include compatibility testing. Performance-critical paths should include benchmark tests. Public API changes require documentation updates and example code.

- **Documentation Standards**: Maintain comprehensive GoDoc comments starting with function names. Include working examples in package documentation. Update technical specifications in `docs/` for protocol changes. Security-related changes must include threat model updates.

- **Performance Standards**: Network operations should be non-blocking with proper timeout handling. Cryptographic operations should use constant-time algorithms. Memory usage should be bounded with proper cleanup. Background maintenance tasks should not impact user-facing operations.

## Security Considerations

- **Threat Model**: Assume storage nodes are honest-but-curious adversaries. Protect against traffic analysis, timing attacks, and metadata correlation. Implement defense against key compromise impersonation (KCI) attacks through Noise-IK protocol.

- **Privacy Protection**: All message routing must use cryptographic pseudonyms to hide sender/recipient identities. Implement message padding to prevent size-based traffic analysis. Use cover traffic and randomized timing to resist activity pattern analysis.

- **Forward Secrecy**: Implement proper pre-key rotation and secure deletion of used keys. Ensure past communications remain secure even if current keys are compromised. Use the established `ForwardSecurityManager` pattern for key lifecycle management.

- **Compliance**: Follow the established C binding patterns for cross-language compatibility. Maintain API stability for existing integrations. Security updates must preserve backward compatibility for legitimate use cases.
