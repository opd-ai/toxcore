# Changelog

## [1.2.0] - 2025-09-05

### Security Enhancements

#### Privacy Improvements

- **Message Size Protection**: Messages are now automatically padded to standard sizes (256B, 1024B, 4096B, 16384B), preventing traffic analysis based on message sizes.
- **Randomized Retrieval Patterns**: The client now uses variable timing with random jitter when retrieving messages, making it difficult for storage nodes to track user activity.
- **Cover Traffic Generation**: Added automatic generation of cover traffic during periods of inactivity to mask real usage patterns.
- **Enhanced Pseudonym System**: Improved identity protection through better pseudonym generation and management, preventing correlation of user activities.

#### Cryptography Improvements

- **Secure Memory Handling**: Implemented secure wiping of sensitive cryptographic material from memory after use, reducing the risk of key extraction.
- **Identity Key Rotation**: Added support for rotating long-term identity keys while maintaining backward compatibility with existing contacts.
- **Enhanced Forward Secrecy**: Improved the pre-key system to ensure that compromise of current keys does not affect past communications.

### User Experience Improvements

- **Configurable Privacy Levels**: Added user settings for privacy protection intensity, allowing trade-offs between maximum privacy and performance.
- **Smoother Key Rotation**: Key rotation now happens seamlessly in the background without interrupting communications.
- **Adaptive Scheduling**: Message retrieval frequency now intelligently adapts based on activity levels, improving battery life while maintaining privacy.

### Developer Improvements

- **Comprehensive Security Documentation**: Added detailed threat model and security guarantee documentation for developers.
- **Enhanced Testing Framework**: Expanded test suite with specific tests for privacy features and cryptographic operations.
- **Improved API Consistency**: Standardized API patterns for cryptographic operations and privacy features.

## [1.1.0] - 2025-04-15

*Note: This changelog starts with version 1.2.0. Earlier versions are not documented here.*

---

All improvements in version 1.2.0 were implemented based on recommendations from a comprehensive security audit. The security posture of the application has been significantly strengthened, particularly against storage node adversaries acting as honest-but-curious entities.
