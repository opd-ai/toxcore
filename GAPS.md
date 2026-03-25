# Implementation Gaps — 2026-03-25

This document identifies gaps between stated goals in the README/documentation and the current implementation state.

---

## Relay-Based NAT Traversal for Symmetric NAT

- **Stated Goal**: README line ~1340: "NAT traversal techniques (UDP hole punching, port prediction)" listed as fully implemented; however, it also states "Relay-based NAT traversal for symmetric NAT is planned but not yet implemented."
- **Current State**: 
  - UDP hole punching: ✅ Implemented (transport/hole_puncher.go:17-57)
  - STUN client: ✅ Implemented (transport/stun_client.go)
  - UPnP client: ✅ Implemented (transport/upnp_client.go)
  - Relay client: Code exists (transport/relay.go:61-501, 642 lines) but is **disabled by default** (`ConnectionRelay: false` at transport/advanced_nat.go:100)
- **Impact**: Users behind symmetric NAT (common in carrier-grade NAT environments, estimated 62% of mobile users) may be unable to establish direct connections. They must fall back to TCP relay nodes as a manual workaround.
- **Closing the Gap**: 
  1. Enable `ConnectionRelay: true` by default in `AdvancedNATTraversal`
  2. Or complete the relay implementation with proper relay server discovery
  3. Document TCP relay node configuration as interim workaround
  4. Validate: `grep -n "ConnectionRelay" transport/advanced_nat.go`

---

## Argon2id Key Derivation

- **Stated Goal**: Modern cryptographic best practices for password-based key derivation.
- **Current State**: Uses PBKDF2 in crypto/keystore.go for deriving encryption keys. PBKDF2 is vulnerable to GPU/ASIC acceleration attacks and is considered outdated for new implementations.
- **Impact**: Lower resistance to brute-force attacks on encrypted key storage compared to memory-hard algorithms. Affects users who encrypt their Tox savedata with passwords.
- **Closing the Gap**:
  1. Add `golang.org/x/crypto/argon2` dependency
  2. Implement `deriveKeyArgon2id()` function with recommended parameters (time=1, memory=64MB, threads=4)
  3. Add version field to encrypted format to support migration
  4. Keep PBKDF2 path for backward compatibility with v2 format
  5. Validate: `go test ./crypto/... -v`

---

## Authenticated Version Negotiation

- **Stated Goal**: transport/version_negotiation.go:41-48 defines `SignedVersionNegotiationPacket` with Ed25519 signatures for MITM protection.
- **Current State**: Signed negotiation is implemented, but when `EnableLegacyFallback: true`, the system accepts unsigned legacy packets, enabling MITM downgrade attacks.
- **Impact**: Attackers can force peers to use the weaker legacy protocol even when both support Noise-IK.
- **Closing the Gap**:
  1. Default `EnableLegacyFallback: false` (already the case in `DefaultProtocolCapabilities()`)
  2. Document security implications when enabling legacy fallback
  3. Add logging/metrics when legacy fallback is triggered
  4. Consider deprecation timeline for legacy protocol support
  5. Validate: `grep -rn "EnableLegacyFallback" transport/*.go`

---

## Dynamic Async Message Limits

- **Stated Goal**: ASYNC.md describes "Spam Resistant: Rate limiting and capacity controls prevent abuse" and "Fair Resource Usage: Storage limited to 1% of available disk space."
- **Current State**: 
  - Per-recipient cap is hardcoded at 100 messages (async/storage.go:50)
  - Storage capacity uses 1% of disk with 1MB-1GB bounds (async/storage_limits.go:175)
  - Popular users could exceed 100 pending messages within minutes during high-traffic periods
- **Impact**: Message loss for popular users when offline. Senders receive errors when recipient's queue is full.
- **Closing the Gap**:
  1. Make per-recipient limit configurable via `AsyncManagerConfig`
  2. Implement dynamic limits: `maxPerRecipient = maxCapacity / activeRecipients`
  3. Add overflow handling (oldest-message eviction or sender notification)
  4. Document limit behavior in user-facing documentation
  5. Validate: `go test ./async/... -v -run TestMessageCapacity`

---

## Scalability Beyond Single-Node

- **Stated Goal**: REPORT.md acknowledges the goal of replacing phone/text messaging at global scale (5-8 billion users).
- **Current State**: 
  - DHT routing table capped at 2,048 nodes (dht/routing.go:72-79)
  - Single-threaded `Iterate()` loop with 50ms tick (toxcore.go:1084-1102)
  - In-memory state with no sharding or replication (toxcore.go:315-328)
  - Async message storage is per-node with no coordination
- **Impact**: Current architecture cannot scale beyond single-node deployment. Global-scale peer discovery would require ~30 DHT hops with high failure rates due to churn.
- **Closing the Gap**: This is a fundamental architectural limitation requiring multi-year engineering effort:
  1. Implement hierarchical/recursive Kademlia with parallel α-lookups
  2. Decouple `Iterate()` into parallel goroutines with priority queues
  3. Implement distributed state management with sharding
  4. Add erasure-coded redundant async message storage
  5. These changes are tracked in ROADMAP.md as future considerations

---

## Write-Ahead Log Default Behavior

- **Stated Goal**: ASYNC.md mentions "crash recovery" and persistent storage.
- **Current State**: WAL support exists (async/storage.go:936) but is optional and disabled by default. Must be explicitly enabled via `storage.EnableWAL()`.
- **Impact**: Node crashes lose all pending offline messages unless application code explicitly enables WAL.
- **Closing the Gap**:
  1. Make WAL enabled by default when `dataDir` is provided to `NewMessageStorage()`
  2. Or document WAL activation as a required step for production deployments
  3. Add `DisableWAL()` method for testing scenarios
  4. Validate: `grep -rn "EnableWAL\|RecoverFromWAL" async/*.go`

---

## Lokinet/Nym Listen Support

- **Stated Goal**: README multi-network table shows Lokinet and Nym as supported transports.
- **Current State**: 
  - Lokinet: TCP Dial only via SOCKS5 (transport/lokinet_transport_impl.go:127); Listen returns error (lines 77-92)
  - Nym: TCP Dial only via SOCKS5 (transport/nym_transport_impl.go:106); Listen not supported (lines 90-101)
  - README correctly documents these as "Dial only"
- **Impact**: Users cannot host services on Lokinet SNApps or Nym without external configuration. This is correctly documented but may surprise users expecting full bidirectional support.
- **Closing the Gap**:
  1. **Lokinet**: Document that SNApp hosting requires manual Lokinet configuration (create .ini service file)
  2. **Nym**: Document that hosting requires Nym service provider configuration (out of scope for client library)
  3. Consider future support if upstream libraries provide hosting APIs
  4. No code changes needed; documentation accurately reflects current state

---

## Group Message History Synchronization

- **Stated Goal**: README "Future Considerations" lists "Group chat message history synchronization."
- **Current State**: Group chat is fully implemented (group/chat.go) but new members joining a group do not receive message history from before they joined.
- **Impact**: Users joining active group chats have no context for ongoing conversations.
- **Closing the Gap**:
  1. Design: Store recent group messages (configurable window, e.g., 100 messages or 24 hours)
  2. Implement history sync protocol during group join handshake
  3. Add encryption for stored history (group key rotation considerations)
  4. This is explicitly marked as a "Future Consideration" in the roadmap

---

## Multi-Device Synchronization

- **Stated Goal**: README "Future Considerations" lists "Multi-device synchronization."
- **Current State**: Each device operates as an independent Tox identity. No mechanism exists to link devices or sync messages/contacts across them.
- **Impact**: Users with multiple devices (phone + desktop) must manage separate Tox identities and manually share contacts.
- **Closing the Gap**:
  1. Design linked-device protocol (primary device authorizes secondaries)
  2. Implement secure message mirroring between linked devices
  3. Add contact list synchronization
  4. This is explicitly marked as a "Future Consideration" in the roadmap

---

## File Transfer Resumption

- **Stated Goal**: README "Future Considerations" lists "File transfer resumption."
- **Current State**: File transfers (file/manager.go) do not persist state. If a transfer is interrupted (network failure, application restart), it must be restarted from the beginning.
- **Impact**: Large file transfers over unreliable connections may fail repeatedly.
- **Closing the Gap**:
  1. Persist transfer state (file ID, position, hash checkpoints) to disk
  2. Implement resume protocol with position negotiation
  3. Add integrity verification for resumed transfers
  4. This is explicitly marked as a "Future Consideration" in the roadmap

---

## Summary

| Gap | Severity | Status |
|-----|----------|--------|
| Relay NAT traversal disabled | HIGH | Code exists, needs enablement |
| PBKDF2 → Argon2id | HIGH | Security improvement |
| Unauthenticated legacy fallback | HIGH | Secure default, needs documentation |
| Dynamic async message limits | MEDIUM | Enhancement needed |
| Scalability architecture | MEDIUM | Documented limitation, future work |
| WAL default behavior | MEDIUM | Documentation/config change |
| Lokinet/Nym listen | LOW | Correctly documented |
| Group history sync | LOW | Future consideration |
| Multi-device sync | LOW | Future consideration |
| File transfer resumption | LOW | Future consideration |

**Note**: Items marked "Future Consideration" are explicitly listed in the README roadmap and do not represent undocumented gaps. The HIGH severity items represent the most actionable improvements for security hardening.
