# Changelog

## [1.3.0] - 2026-03-22

### C API Expansion

- **Extended Function Coverage**: C API now includes 63 exported functions (~79% of libtoxcore coverage), up from ~25 functions previously.
- **New Self Functions**: Added `tox_self_get_connection_status`, `tox_self_get_status`, `tox_self_set_status`, `tox_self_get_nospam`, `tox_self_set_nospam`, `tox_self_get_friend_list`, `tox_self_get_friend_list_size`.
- **New Friend Functions**: Added `tox_friend_get_name`, `tox_friend_get_name_size`, `tox_friend_get_status`, `tox_friend_get_status_message`, `tox_friend_get_status_message_size`, `tox_friend_get_connection_status`, `tox_friend_get_public_key`, `tox_friend_get_last_online`, `tox_friend_exists`.
- **New Conference Functions**: Added `tox_conference_get_title`, `tox_conference_peer_get_name`, `tox_conference_peer_get_name_size`, `tox_conference_peer_get_public_key`, `tox_conference_connected`, `tox_conference_offline_peer_count`, `tox_conference_offline_peer_get_name`, `tox_conference_offline_peer_get_name_size`.
- **New Utility Functions**: Added `tox_file_get_file_id`, `tox_hash`.

### Code Quality Improvements

- **Transport Layer Refactoring**: Split monolithic `network_transport_impl.go` (970 lines, 5 transport types) into focused files: `ip_transport.go`, `tor_transport_impl.go`, `i2p_transport_impl.go`, `nym_transport_impl.go`, `lokinet_transport_impl.go`. Improves cohesion and maintainability.
- **Reduced Complexity**: Refactored `tox_conference_send_message` from complexity 15.3 to 5.7 by extracting validation logic into helper functions.

### API Additions

- **ValidateConferenceAccess**: Exported new method for C API access to conference validation.

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
