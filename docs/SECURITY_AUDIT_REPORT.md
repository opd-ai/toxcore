# Security & Protocol Compliance Audit Report

**Project:** opd-ai/toxcore (Go Tox Implementation)  
**Date:** 2026-02-21  
**Scope:** Noise-IK integration, async messaging, backward compatibility, downgrade prevention, dual-mode operation

---

## 1. COMPATIBILITY ANALYSIS

This implementation maintains baseline **c-toxcore compatibility** through matching packet type constants (`transport/packet.go:29-90`), standard message size limits of 1372 bytes plaintext (`limits/limits.go`), NaCl box/secretbox cryptographic primitives (`crypto/encrypt.go`, `crypto/decrypt.go`), and libtoxcore-compatible ToxAV API bindings (`capi/toxav_c.go`). The DHT bootstrap protocol follows the standard Tox node discovery mechanism (`dht/bootstrap.go:125-139`).

**[INFO]** No direct **rstox** references exist in the codebase. This is a clean-room Go implementation rather than a port, so compatibility is achieved through protocol specification adherence rather than code-level alignment. Both implementations target the same Tox protocol specification.

**Breaking changes identified:**
- **[WARNING]** Packet types 249-251 (`PacketVersionNegotiation`, `PacketNoiseHandshake`, `PacketNoiseMessage`) are extensions not present in c-toxcore (`transport/packet.go`). These occupy the reserved range and will be ignored by legacy clients.
- **[INFO]** Async messaging (`async/`) and identity obfuscation (`async/obfs.go`) are novel extensions with no c-toxcore equivalent. These operate as optional layers atop the base protocol.

---

## 2. SECURITY ASSESSMENT

### Noise-IK Implementation Correctness
The IK pattern is correctly implemented via the `flynn/noise` library using Curve25519 + ChaCha20-Poly1305 + SHA256 (`noise/handshake.go:44-54`). The `IKHandshake` struct maintains proper state separation with `sendCipher`/`recvCipher` after completion. Handshake nonces are 32 bytes from `crypto/rand`, and handshake timestamps are recorded and exposed for observability but are not currently enforced for freshness validation. **[INFO]** Implementation delegates pattern correctness to the well-audited `flynn/noise` library.

### KCI Resistance
The IK pattern provides KCI resistance by design: the initiator encrypts their static key under the responder's key (`e, es, s, ss`), ensuring a compromised responder key cannot impersonate the initiator back to the responder (see the IK initiator handshake flow in `noise/handshake.go`). **[INFO]** KCI resistance is inherent to the IK pattern specification.

### Downgrade Attack Mitigation
Version negotiation (`transport/version_negotiation.go:14-18`) defines `ProtocolLegacy=0` and `ProtocolNoiseIK=1`. `SelectBestVersion` (`transport/version_negotiation.go:175-191`) selects the highest mutually supported version. Downgrades are logged as security warnings (`transport/negotiating_transport.go:117`). **[WARNING]** The downgrade log is advisory only — an active MITM stripping version negotiation packets could force legacy mode when `fallbackEnabled=true` (`transport/negotiating_transport.go:110-121`). The version negotiation packet itself is not cryptographically authenticated.

### Cryptographic Primitive Usage
- **[INFO]** Curve25519 key clamping correctly applied (`crypto/keypair.go:80-82`)
- **[INFO]** Constant-time comparisons via `subtle.ConstantTimeCompare` and `hmac.Equal` (`async/obfs.go:9,135`)
- **[INFO]** Secure memory wiping via `subtle.XORBytes` + `runtime.KeepAlive` (`crypto/secure_memory.go`)
- **[INFO]** PBKDF2 with 100K iterations for key derivation at rest (`crypto/keystore.go`)

---

## 3. PROTOCOL COMPLIANCE

### Standard Tox Protocol Adherence
The implementation follows the Tox protocol specification for DHT routing, friend requests, and messaging. Packet types and message sizes match the standard (`limits/doc.go:11-12`). The NaCl-based encryption layer is compatible with c-toxcore peers. When using `NewBootstrapManagerWithKeyPair`, versioned handshakes are enabled by default (`dht/bootstrap.go:133`), while `NewBootstrapManager` enables them only once a keypair is available; legacy fallback remains supported in both cases.

### Extension Implementations
**Async Messaging** (`async/`): Implements offline message delivery with one-time pre-keys (`ForwardSecurityManager`, `async/forward_secrecy.go:42-47`), 100 pre-keys per peer with low-watermark refresh at 10 keys. Messages expire after 24 hours. **[INFO]** This is an additive extension — non-supporting peers simply won't receive offline messages.

**Identity Obfuscation** (`async/obfs.go:25-28`): Epoch-based recipient pseudonyms rotating every 6 hours (`async/epoch.go:10`) with per-message sender pseudonyms via HKDF. Recipient proofs use HMAC-SHA256 for spam prevention without revealing identity.

### Protocol Version Negotiation
Handled by `NegotiatingTransport` wrapping the underlying transport. Per-peer version tracking enables mixed-network operation. **[INFO]** Version negotiation adds a round-trip before first communication with each peer.

---

## 4. CRITICAL FINDINGS

### High Severity
- [ ] **[CRITICAL]** Version negotiation packets are unauthenticated — an active attacker can strip `PacketVersionNegotiation` (type 249) to force downgrade to legacy protocol when `fallbackEnabled=true` (`transport/negotiating_transport.go:110-121`)

### Medium Severity
- [ ] **[WARNING]** Packet types 249-251 use reserved range; collision risk with future c-toxcore extensions (`transport/packet.go`)
- [ ] **[WARNING]** No mutual version commitment — peers independently select versions without cryptographic binding to prevent version rollback

### Low Severity
- [ ] **[INFO]** PBKDF2 used for key derivation at rest; Argon2id would provide stronger memory-hard protection (`crypto/keystore.go`)
- [ ] **[INFO]** Epoch genesis time hardcoded to 2025-01-01 UTC (`async/epoch.go:26`); no mechanism for epoch parameter negotiation

---

## 5. RECOMMENDATIONS

1. **Authenticate version negotiation**: Sign `VersionNegotiationPacket` with the sender's static key to prevent MITM downgrade attacks on the negotiation itself
2. **Add version commitment**: After Noise-IK handshake completes, exchange authenticated version confirmations to detect rollback attempts
3. **Register extension packet types**: Coordinate packet types 249-251 with the Tox protocol specification maintainers to prevent future collisions
4. **Upgrade to Argon2id**: Replace PBKDF2 in `crypto/keystore.go` with Argon2id for stronger resistance against GPU/ASIC attacks on stored keys
5. **Add `fallbackEnabled=false` as default**: Require explicit opt-in for legacy fallback to enforce secure-by-default operation
