# toxcore-go Documentation

## Protocol Specifications
- **[PROTOCOL_SPEC.md](PROTOCOL_SPEC.md)** — Complete protocol specification
- **[ASYNC.md](ASYNC.md)** — Asynchronous messaging with forward secrecy
- **[FORWARD_SECRECY.md](FORWARD_SECRECY.md)** — Epoch-based forward secrecy and pseudonym rotation
- **[OBFS.md](OBFS.md)** — Identity obfuscation and privacy protection
- **[MULTINETWORK.md](MULTINETWORK.md)** — Multi-network transport architecture
- **[NETWORK_ADDRESS.md](NETWORK_ADDRESS.md)** — Network address handling
- **[SINGLE_PROXY.md](SINGLE_PROXY.md)** — TSP/2.0 proxy specification
- **[DHT.md](DHT.md)** — DHT routing table scalability
- **[BOOTSTRAP_SERVER.md](BOOTSTRAP_SERVER.md)** — Bootstrap server connectivity
- **[MESSAGE_RECEIPTS.md](MESSAGE_RECEIPTS.md)** — Delivery receipt support

## Transport Implementations
- **[TOR_TRANSPORT.md](TOR_TRANSPORT.md)** — Tor transport via onramp
- **[I2P_TRANSPORT.md](I2P_TRANSPORT.md)** — I2P transport via onramp/SAMv3
- **[NYM_TRANSPORT.md](NYM_TRANSPORT.md)** — Nym transport via SOCKS5
- **[LOKINET_MANUAL.md](LOKINET_MANUAL.md)** — Lokinet transport
- **[FRIEND_REQUEST_TRANSPORT.md](FRIEND_REQUEST_TRANSPORT.md)** — Friend request transport
- **[FILE_TRANSFER.md](FILE_TRANSFER.md)** — File transfer operations

## Audio/Video
- **[TOXAV_BENCHMARKING.md](TOXAV_BENCHMARKING.md)** — ToxAV performance benchmarks
- **[VP8_ENCODER_EVALUATION.md](VP8_ENCODER_EVALUATION.md)** — VP8 encoder evaluation

## Security & Cryptography
- **[SECURE_INTEGRATION_GUIDE.md](SECURE_INTEGRATION_GUIDE.md)** — Security levels and integration patterns
- **[SECURITY_PATCH_PLAYBOOK.md](SECURITY_PATCH_PLAYBOOK.md)** — Security patch procedures
- **[SECURITY_ADVISORY_BEHAVIOR_CHANGES.md](SECURITY_ADVISORY_BEHAVIOR_CHANGES.md)** — Behavior changes from security fixes
- **[SIDE_CHANNEL_REVIEW.md](SIDE_CHANNEL_REVIEW.md)** — Side-channel analysis (X3DH, PQXDH, sealed sender)
- **[COVER_TRAFFIC.md](COVER_TRAFFIC.md)** — Transport-layer cover traffic design
- **[SECURITY_AUDIT_REPORT.md](SECURITY_AUDIT_REPORT.md)** — Security assessment
- **[SECURITY_AUDIT_SUMMARY.md](SECURITY_AUDIT_SUMMARY.md)** — Security audit executive summary
- **[CI_COMPATIBILITY_MATRIX.md](CI_COMPATIBILITY_MATRIX.md)** — Protocol compatibility testing matrix

## Development
- **[CHANGELOG.md](CHANGELOG.md)** — Version history
- **[PROFILING.md](PROFILING.md)** — Performance profiling guide
- **[PERFORMANCE_BENCHMARKS.md](PERFORMANCE_BENCHMARKS.md)** — Benchmark results
- **[DEPENDENCY_MANAGEMENT.md](DEPENDENCY_MANAGEMENT.md)** — Dependency management
- **[PRIVACY_NETWORK_QUICKSTART.md](PRIVACY_NETWORK_QUICKSTART.md)** — Quick-start for Tor/I2P
- **[SECURE_MESSAGING_MIGRATION.md](SECURE_MESSAGING_MIGRATION.md)** — Fail-closed messaging migration
- **[RELEASE_CANDIDATE_FREEZE.md](RELEASE_CANDIDATE_FREEZE.md)** — Release process

## Key Security Features

toxcore-go includes modern cryptographic enhancements beyond the original Tox protocol:

- **PQXDH** — Post-quantum hybrid key agreement (ML-KEM-768 + X3DH) for quantum-resistant session establishment
- **X3DH** — Extended Triple Diffie-Hellman for perfect forward secrecy and deniable authentication
- **Sealed Sender** — Encrypts sender identity to prevent transport-layer identification
- **Double Ratchet** — Per-message forward secrecy with header encryption
- **Capability Negotiation** — Automatic per-peer feature negotiation with backward compatibility

See [SECURE_INTEGRATION_GUIDE.md](SECURE_INTEGRATION_GUIDE.md) for security levels and deployment patterns.
