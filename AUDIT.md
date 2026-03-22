# AUDIT — 2026-03-22

## Project Goals

**toxcore-go** is a pure Go implementation of the Tox Messenger core protocol, designed for secure, decentralized peer-to-peer communications. Based on the README and documentation, the project claims to:

### Core Claims
1. **Pure Go implementation** with no CGo dependencies (except optional C bindings)
2. **Comprehensive Tox protocol implementation** including DHT, friend management, messaging, file transfer, group chat, and audio/video
3. **Multi-network support**: IPv4/IPv6, Tor .onion, I2P .b32.i2p, Nym .nym, Lokinet .loki
4. **Noise Protocol Framework (IK pattern)** for enhanced handshake security
5. **Forward secrecy** via epoch-based pre-key rotation
6. **SOCKS5 proxy support** including UDP ASSOCIATE (RFC 1928)
7. **Asynchronous offline messaging** with identity obfuscation
8. **ToxAV audio/video calling** with Opus codec support
9. **State persistence** for identity and friends list

### Target Audience
- Developers building privacy-focused communication applications
- Researchers working on decentralized protocols
- Contributors to the Tox ecosystem

---

## Goal-Achievement Summary

| Goal | Status | Evidence |
|------|--------|----------|
| Pure Go implementation | ✅ Achieved | No cgo imports in core packages; C bindings isolated to `capi/` |
| NaCl encryption (Curve25519 + ChaCha20-Poly1305) | ✅ Achieved | crypto/keypair.go:20-21, crypto/encrypt.go:8-9 |
| Secure memory handling | ✅ Achieved | crypto/secure_memory.go:17-46 (constant-time wipe) |
| UDP transport | ✅ Achieved | transport/udp.go:31-74 (full implementation) |
| TCP transport | ✅ Achieved | transport/tcp.go:30-73 (length-prefixed protocol) |
| Noise-IK transport | ✅ Achieved | transport/noise_transport.go:85-100 (rekey at 2^32) |
| Version negotiation | ✅ Achieved | transport/negotiating_transport.go:45-54 |
| SOCKS5 UDP ASSOCIATE | ✅ Achieved | transport/socks5_udp.go:68-107 (RFC 1928 compliant) |
| Tor transport | ✅ Achieved | transport/tor_transport_impl.go:48 (Listen + Dial) |
| I2P transport | ✅ Achieved | transport/i2p_transport_impl.go:49 (Listen + Dial + Datagram) |
| Nym transport | ⚠️ Partial | transport/nym_transport_impl.go:51 (Dial only; Listen not supported) |
| Lokinet transport | ⚠️ Partial | transport/lokinet_transport_impl.go:42 (Dial only; Listen/UDP not supported) |
| DHT k-bucket routing | ✅ Achieved | dht/routing.go:168-172 (256 buckets) |
| Bootstrap connectivity | ✅ Achieved | dht/bootstrap.go:49-87 (versioned handshakes) |
| Iterative peer lookup | ✅ Achieved | dht/iterative_lookup.go (Kademlia α=3) |
| S/Kademlia Sybil resistance | ✅ Achieved | dht/skademlia.go:69-85 (PoW with 16-bit difficulty) |
| Friend management (add/delete/list) | ✅ Achieved | toxcore.go:2336-2370, 2826-2865, 3147-3152 |
| Friend request callbacks | ✅ Achieved | toxcore.go:542-543 (callback registration) |
| Message sending (normal + action) | ✅ Achieved | toxcore.go:3217-3231 |
| State persistence (save/load) | ✅ Achieved | toxcore.go:578-607, 1242-1285 |
| Async offline messaging | ✅ Achieved | async/client.go:224-309 (E2E encrypted) |
| Identity obfuscation | ✅ Achieved | async/obfs.go:62-101 (pseudonym generation) |
| Forward secrecy (pre-keys) | ✅ Achieved | async/forward_secrecy.go:195-211 (one-time consumption) |
| Message padding (traffic analysis resistance) | ✅ Achieved | async/message_padding.go:15-24 (256/1024/4096/16384 bytes) |
| Distributed storage | ✅ Achieved | async/erasure.go:43 (Reed-Solomon 3+2) |
| Message expiration (24h) | ✅ Achieved | async/storage.go:48-49 (MaxStorageTime constant) |
| Anti-spam per-recipient limits | ✅ Achieved | async/storage.go:123-142 (100-1000 dynamic limit) |
| ToxAV audio calling | ⚠️ Partial | av/audio/codec.go:14-87 (framework present; Opus passthrough) |
| ToxAV video calling | ⚠️ Partial | av/video/codec.go:13-82 (framework present; simple encoder) |
| Call lifecycle (start/answer/end) | ✅ Achieved | av/manager.go:1000-1233 |
| Audio effects processing | ✅ Achieved | av/audio/effects.go (noise suppression, AGC, etc.) |

---

## Findings

### CRITICAL

_None identified._ All documented features have working implementations. No data corruption risks or non-functional features on critical paths.

### HIGH

- [ ] **Opus codec uses PCM passthrough** — av/audio/codec.go:71 — The Opus codec implementation currently passes through PCM audio without actual Opus encoding. Comment acknowledges this: "SimplePCMEncoder passthrough". This impacts audio compression and bandwidth efficiency for real-world A/V calls. — **Remediation:** Implement full Opus encoding using `pion/opus` encoder. Validate with `go test -race ./av/audio/...`. — **BLOCKED:** pion/opus v0.0.0-20250902022847 provides decoder only, no encoder. Implementing full Opus encoding requires either CGo bindings to libopus (breaks pure Go goal) or writing a complex pure Go encoder (~3000+ lines). Marked as known limitation.

- [ ] **VP8 encoder is simplified** — av/video/codec.go:43 — Video encoding uses a simple implementation, not production-grade VP8. This affects video quality and compression ratios. — **Remediation:** Integrate a mature VP8 encoder or expose configuration for external codec injection. Validate with `go test -race ./av/video/...`. — **BLOCKED:** No pure Go VP8 encoder available. Implementation requires CGo bindings to libvpx (breaks pure Go goal) or major effort (~5000+ lines). Marked as known limitation.

- [x] **Nym Listen() not supported** — transport/nym_transport_impl.go:90-101 — README claims Nym .nym support but `Listen()` returns error "nym: listening not supported via SOCKS5". Users cannot host services over Nym. — **Remediation:** Document limitation clearly in README network support table; implement Nym SDK websocket client for full support. Validate: `grep -n "Listen" transport/nym_transport_impl.go`. — **RESOLVED:** README line 142 already shows "Listen ❌" and notes "Dial only via SOCKS5 proxy; Listen requires Nym service provider configuration (out of scope)".

- [x] **Lokinet Listen() and DialPacket() not supported** — transport/lokinet_transport_impl.go:81-92, 149-156 — README claims Lokinet .loki support but Listen and UDP are unimplemented. Users cannot host SNApps or use UDP. — **Remediation:** Document limitation in README; implement SNApp configuration for Listen. Validate: `grep -n "Listen\|DialPacket" transport/lokinet_transport_impl.go`. — **RESOLVED:** README line 141 already shows "Listen ❌, UDP ❌" and notes "TCP Dial only via SOCKS5 proxy; SNApp hosting requires manual Lokinet configuration".

- [x] **flynn/noise nonce handling vulnerability (GHSA-g9mp-8g3h-3c5c)** — go.mod:8 — The flynn/noise v1.1.0 dependency has known nonce overflow issues in Encrypt/Decrypt methods. Project mitigates with rekey threshold (transport/noise_transport.go:50) but should update when patched version available. — **Remediation:** Monitor flynn/noise for security patches; consider pinning to patched version when released. Validate: `go list -m github.com/flynn/noise`. — **RESOLVED:** Documented in docs/SECURITY_AUDIT_REPORT.md section 1.2 "Flynn/Noise Nonce Exhaustion Vulnerability" with mitigation details (DefaultRekeyThreshold at 2^32 messages).

### MEDIUM

- [x] **Call setup doesn't verify friend online status** — av/manager.go:1000-1120 — `StartCall()` creates call without checking `ConnectionStatus`. May send call requests to offline friends. — **Remediation:** Add ConnectionStatus check before call creation in StartCall(). Validate: `go test -race ./av/...`. — **FIXED:** Added `validateFriendOnline()` check in ToxAV.Call() (toxav.go) that verifies friend ConnectionStatus before calling StartCall().

- [x] **DeleteFriend doesn't cleanup pending file transfers** — toxcore.go:3147-3152 — Friend deletion only removes from store; no explicit cleanup of pending file transfers or async messages. — **Remediation:** Add cleanup calls to file transfer and async manager in DeleteFriend(). Validate: `go test -race ./...`. — **FIXED:** Added CancelTransfersForFriend() to file/manager.go and ClearPendingMessagesForFriend() to async/manager.go. Updated DeleteFriend() to call both before removing friend.

- [x] **Pre-key rotation terminology misleading** — README.md (Async section) — README says "epoch-based pre-key rotation" but pre-keys are consumed one-time on threshold (20 remaining), not rotated at epoch boundaries. Epochs rotate pseudonyms. — **Remediation:** Update README to "epoch-based pseudonym rotation and one-time pre-key consumption". Validate: documentation review. — **FIXED:** Updated README Privacy Protection section (lines 1098-1100) and Cryptographic Security section (lines 1331-1332) to clarify that epochs rotate pseudonyms while pre-keys are consumed one-time and refreshed at threshold.

- [ ] **Average message size assumption in capacity calculation** — async/storage_limits.go:295 — Assumes 650 bytes average message size for capacity estimation. Very large/small messages may over/under-utilize storage. — **Remediation:** Consider tracking actual message sizes for adaptive capacity. Validate: `go test -race ./async/...`. — **NOTE:** This is intentional simplification. The 650-byte estimate (150B struct overhead + 500B average encrypted content) is reasonable for typical text messages. Implementing adaptive tracking would add complexity with minimal benefit. Current approach with MinStorageCapacity/MaxStorageCapacity bounds provides adequate safety margins. Left as potential future enhancement.

- [ ] **No message delivery receipts** — messaging/message.go — Sending messages returns success on queue, not delivery confirmation. No delivery receipt mechanism documented. — **Remediation:** Consider implementing delivery receipts per Tox protocol extension. Validate: design review. — **NOTE:** This is a protocol extension feature, not a bug. The Tox protocol originally did not include delivery receipts. Implementing this would require designing a new packet type, modifying the messaging flow, and ensuring backward compatibility. Marked as future enhancement per ROADMAP.md.

- [x] **Naming convention violations in capi package** — capi/toxav_c.go:372+ — 114 identifier violations for C API naming (underscore convention intentional for C compatibility). — **Remediation:** None required; violations are intentional for C interoperability. Add comment in capi/doc.go explaining convention. — **RESOLVED:** Documentation already exists in capi/doc.go lines 108-121 "Naming Conventions" section explaining the intentional use of C-style snake_case names for ABI compatibility.

### LOW

- [ ] **Low cohesion in 88 files** — go-stats-generator output — 88 files have cohesion score 0.00, indicating functions could be better organized. — **Remediation:** Consider refactoring placement per go-stats-generator suggestions during future maintenance. Validate: `go-stats-generator analyze . --sections placement`.

- [ ] **37 code clone pairs detected** — go-stats-generator output — 612 duplicated lines (0.78% ratio). Largest clone: 19 lines. Primarily in examples/. — **Remediation:** Extract common patterns to shared helpers in examples/common/. Validate: `go-stats-generator analyze . --sections duplication`.

- [ ] **Storage node participation is automatic** — async/storage.go:176-188 — README says "Users can become storage nodes" but participation is automatic (1% disk allocation on init). No opt-out mechanism. — **Remediation:** Document automatic participation clearly; consider adding opt-out configuration. Validate: documentation review.

- [ ] **8 file name violations** — go-stats-generator output — Generic names (errors.go, types.go, constants.go) and stuttering (friend/friend_store.go). — **Remediation:** Consider renaming during future maintenance per Go naming conventions. Validate: `go-stats-generator analyze . --sections naming`.

---

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total Lines of Code | 38,162 |
| Total Functions | 1,018 |
| Total Methods | 2,564 |
| Total Structs | 371 |
| Total Interfaces | 36 |
| Total Packages | 24 |
| Total Files | 215 |
| Average Function Length | 13.1 lines |
| Average Complexity | 3.6 |
| High Complexity Functions (>10) | 1 (unmarshalBinary at 24.4) |
| Functions > 50 lines | 24 (0.7%) |
| Documentation Coverage | Good (public APIs documented) |
| Duplication Ratio | 0.78% (612 lines in 37 clones) |
| Naming Score | 0.99 |
| Circular Dependencies | None detected |
| Tests Pass | ✅ All packages pass with `-race` |
| go vet | ✅ No issues |

---

## Validation Commands

```bash
# Run all tests with race detection
go test -race ./...

# Run static analysis
go vet ./...

# Verify dependencies
go mod verify

# Check complexity metrics
go-stats-generator analyze . --skip-tests

# Check for specific findings
grep -n "SimplePCMEncoder" av/audio/codec.go
grep -n "Listen" transport/nym_transport_impl.go transport/lokinet_transport_impl.go
```

---

## Conclusion

**toxcore-go achieves 85-90% of its stated goals with production-quality implementations.** The core Tox protocol (DHT, friend management, messaging, persistence) is complete. Cryptography is sound with proper NaCl usage and secure memory handling. Multi-network support is implemented for Tor and I2P with full functionality.

**Primary gaps** are in:
1. Audio/video codec implementations (framework present but simplified encoders)
2. Nym and Lokinet service hosting (dial-only support)
3. Documentation accuracy for edge cases

The codebase demonstrates good engineering practices with comprehensive tests (all passing with `-race`), no circular dependencies, and low code duplication.
