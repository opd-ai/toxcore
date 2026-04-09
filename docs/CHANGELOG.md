# Changelog

All notable changes to toxcore-go will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.4.0-qtox-preview] - 2026-04-04

### Added
- qTox C API compatibility testing suite (`capi/compatibility_test.go`)
- Bootstrap node connectivity documentation with fallback patterns
- qTox integration example (`examples/qtox_integration/`)
- Configurable `MaxMessagesPerRecipient` in `AsyncManagerConfig`
- WAL persistence auto-enabled by default for storage nodes
- TCP relay NAT traversal enabled by default
- DHT routing table scalability documentation (`docs/DHT.md`)
- Group peer auto-discovery protocol with `PeerDiscoveredCallback`
- Async storage node DHT-based discovery
- Delivery receipt support for reliable message confirmation

### Changed
- Refactored `toxcore.go` from 2,570 lines to 1,432 lines by extracting:
  - File transfer functions to `toxcore_file.go`
  - Conference functions to `toxcore_conference.go`
  - Persistence functions to `toxcore_persistence.go`
  - Friend management expanded in `toxcore_friends.go`
  - Network helpers expanded in `toxcore_network.go`
- Improved goroutine lifecycle management with graceful shutdown
- Enhanced documentation coverage to 93%+

### Fixed
- File transfer callback wiring for bidirectional transfers
- Async message persistence via WAL crash recovery
- GAPS.md incorrectly stated flynn/noise v1.1.0 was vulnerable (CVE-2021-4239 was fixed in v1.0.0)
- Legacy fallback MITM risk documentation in README

### Security
- Confirmed flynn/noise v1.1.0 is patched against CVE-2021-4239
- Added security warning for `EnableLegacyFallback` option in README

## [1.3.0] - 2026-03-04

### Added
- Pure Go VP8 video encoder via opd-ai/vp8 (I-frames only)
- Pure Go Opus audio codec via opd-ai/magnum
- Multi-network transport support (Tor, I2P, Lokinet, Nym)
- Noise Protocol Framework (IK pattern) for secure handshakes
- Epoch-based forward secrecy with automatic key rotation
- Identity obfuscation via cryptographic pseudonyms
- Automatic message padding (256B, 1024B, 4096B buckets)

### Changed
- Upgraded Go requirement to 1.25.0 (toolchain go1.25.8)
- Replaced pion/opus with opd-ai/magnum for pure Go audio

### Known Limitations
- VP8 encoder produces key frames only (P-frames blocked on upstream)
- Lokinet and Nym support Dial only (Listen requires manual daemon config)

## [1.2.0] - 2026-02-20

### Added
- ToxAV audio/video calling infrastructure
- Group chat with DHT-based peer discovery
- File transfer with chunked streaming

### Changed
- Major refactoring for maintainability
- Improved test coverage

## [1.1.0] - 2026-01-28

### Added
- Full Tox protocol implementation
- DHT routing with k-buckets
- Friend management and messaging
- C API bindings in capi/

## [1.0.0] - 2026-01-01

### Added
- Initial release
- Pure Go implementation of Tox core protocol
- UDP and TCP transport support
- Bootstrap node connectivity
- Basic friend request/accept flow
