# UNIVERSAL BUG AUDIT (END-TO-END) — toxcore — 2026-06-16

## Project Profile

**toxcore-go**: Pure Go implementation of the Tox peer-to-peer encrypted messaging protocol. Provides DHT-based peer discovery, friend management, 1-to-1 and group messaging, file transfers, audio/video calling (ToxAV), asynchronous offline messaging with forward secrecy, and multi-network transport (IPv4/IPv6, Tor, I2P, Lokinet, Nym) — all without cgo dependencies in the core library.

**Target**: Secure, backward-compatible P2P messaging with cryptographic integrity, forward secrecy, and identity obfuscation.

## Audit Scope

- **Module**: `github.com/opd-ai/toxcore`
- **Go Version**: 1.25.0 (toolchain go1.25.11)
- **Total Packages**: 60
- **Source Files**: 266 (non-test), 283 (test)
- **Critical Dependencies**: `golang.org/x/crypto v0.52.0`, `github.com/flynn/noise v1.1.0`, `github.com/cloudflare/circl v1.6.3`

### Packages Audited

| Package | Role |
|---------|------|
| `toxcore` (root) | API facade, lifecycle, iteration pipelines |
| `crypto/` | Encryption, signatures, key management, secure memory |
| `transport/` | UDP/TCP/Noise/Tor/I2P/Nym/Lokinet transports, NAT traversal |
| `async/` | Offline messaging, forward secrecy, storage nodes, obfuscation |
| `dht/` | DHT routing, k-bucket, peer discovery, mDNS |
| `friend/` | Friend management, request handling |
| `messaging/` | Message types, processing, delivery tracking |
| `file/` | File transfers, chunked I/O |
| `group/` | Group chat, role-based permissions |
| `noise/` | Noise Protocol Framework handshakes |
| `ratchet/` | Double ratchet, header encryption |
| `av/` | Audio/video calling management |
| `av/audio/` | Opus codec, audio effects |
| `av/rtp/` | RTP packet handling |
| `av/video/` | VP8 codec |
| `toxnet/` | net.Conn/Listener/PacketConn implementations |
| `capi/` | C API bindings |
| `bootstrap/` | Bootstrap server, node management |

## Coverage Log

| Package | 2b Logic | 2c Nil | 2d Errors | 2e Resources | 2f Concurrency | 2g Security | 2h Aliasing | 2i Init | 2j API | Status |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|--------|
| crypto/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| transport/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| async/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| dht/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| friend/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| messaging/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| file/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| group/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| noise/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| ratchet/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| av/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| av/rtp/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| av/audio/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| av/video/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| toxnet/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| capi/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| bootstrap/ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Done |

## Goal-Achievement Summary

| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| Backward compatible with legacy Tox protocol | ✅ | No blocking findings |
| Cryptographic integrity (keys, signatures, MACs) | ✅ | No confirmed crypto weaknesses |
| P2P message delivery and state management | ⚠️ | H-01 (routing table lock safety) |
| Forward secrecy via epoch-based pre-keys | ⚠️ | M-05 (memory growth in rate limiter) |
| File transfer reliability | ⚠️ | H-03 (unlock-invoke-relock race window) |
| Audio/video calling | ⚠️ | M-06 (RTP timestamp overflow after 24.8 days) |

## Findings

### CRITICAL

No confirmed CRITICAL-severity findings. The codebase demonstrates strong defensive programming with prior audit remediations in place (labels C-01, H-04, M-06, L-12 etc. found in comments).

### HIGH

- [ ] **H-01: Missing defer on mutex unlock in RoutingTable.addNodeWithFn** — `dht/routing.go:311-313` — Concurrency/Resources — If `addFn(bucketIndex)` panics (e.g., index out of range in `rt.kBuckets[bucketIndex].AddNode(node)`), `rt.mu` is never released, causing permanent deadlock of the routing table. All subsequent DHT operations (`AddNode`, `FindClosestNodes`, `RemoveNode`) will hang indefinitely. **Remediation:** Replace manual unlock with `defer rt.mu.Unlock()` immediately after `rt.mu.Lock()`.

- [ ] **H-02: Missing defer on read lock in FindClosestNodes** — `dht/routing.go:390-394` — Concurrency/Resources — The `rt.mu.RLock()` is released via manual `rt.mu.RUnlock()` 4 lines later. If `createTargetNode()`, `buildNodeHeap()`, or `extractSortedNodes()` panic, the read lock leaks, blocking all future write operations. **Remediation:** Use `defer rt.mu.RUnlock()` immediately after `rt.mu.RLock()`.

- [ ] **H-03: Race window in file transfer callback invocation** — `file/transfer.go:572-576` — Concurrency — The transfer mutex is released before invoking the progress callback, then re-acquired. Between unlock and relock, concurrent operations (Cancel, Pause, additional data chunks) can modify transfer state. While this pattern is documented as intentional to prevent re-entrant deadlocks (M-FILE-3), it creates a real race window where `t.State` may change during callback execution. **Remediation:** Capture all needed state into locals before unlock; validate state after relock; consider read-only snapshot for callbacks.

- [ ] **H-04: Unchecked Write during relay disconnect** — `transport/relay.go:628` — Error Handling — `rc.activeConn.Write(disconnectPacket)` return value is discarded. If the write fails (broken pipe, RST), the relay server never receives the disconnect signal, leaving a stale session until timeout. Combined with `SetWriteDeadline` error also being unchecked (line 627). **Remediation:** Log the Write error; this is a best-effort disconnect but should be observable.

### MEDIUM

- [ ] **M-01: Panic in crypto.ZeroBytes on SecureWipe failure** — `crypto/secure_memory.go:48` — Error Handling — `ZeroBytes()` calls `panic()` if `SecureWipe()` fails. This function is widely used in `defer ZeroBytes(...)` patterns throughout the codebase. A SecureWipe failure (unlikely but possible on constrained systems) crashes the entire application instead of degrading gracefully. **Remediation:** Log the error and continue, or return an error.

- [ ] **M-02: Panics in init() for mDNS address resolution** — `dht/mdns_discovery.go:48,51` — Error Handling — `init()` panics if `net.ResolveUDPAddr` fails for mDNS multicast addresses ("224.0.0.251:5353" and "[ff02::fb]:5353"). On systems without IPv6 support, this can crash the application at import time. **Remediation:** Defer resolution to first use, or gracefully disable mDNS for unavailable address families.

- [ ] **M-03: Panic in init() for NAT fallback address** — `transport/nat.go:27` — Error Handling — Similar to M-02; `init()` panics if NAT fallback address resolution fails. **Remediation:** Lazy initialization with error return.

- [ ] **M-04: Goroutine leak in relay readLoop** — `transport/relay.go:173` — Concurrency/Resources — `go rc.readLoop()` is launched without WaitGroup tracking. `Close()` doesn't synchronize with readLoop termination. If context cancellation is delayed or readLoop is blocked on I/O, resources may not be cleaned up promptly. **Remediation:** Add WaitGroup; wait in Close().

- [ ] **M-05: Unbounded memory growth in pre-key rate limiter** — `async/forward_secrecy.go:365-372` — Data Aliasing — `times = times[start:]` creates a slice header sharing the original backing array. `append(times, now)` at line 372 may write into unused capacity of the original array. Over many iterations, the backing array grows unboundedly while the logical slice remains small. **Remediation:** Allocate a new slice: `newTimes := make([]time.Time, len(times), len(times)+1)`.

- [ ] **M-06: Video RTP timestamp overflow after ~24.8 days** — `av/rtp/session.go:362-363` — Logic/Arithmetic — `elapsed.Milliseconds() * 90` overflows int64 after ~1.18e15 nanoseconds (theoretical), but the `uint32()` cast wraps after `elapsed > 47.7 days`. While RTP timestamp wrap is expected per RFC 3550, there's no wrap-around handling in the jitter buffer that consumes these values. **Remediation:** Document that timestamp wraps are expected; verify jitter buffer handles wraps.

- [ ] **M-07: Unvalidated odd-length audio buffer** — `av/rtp/transport.go:299-305` — Logic — `convertToPCMSamples()` converts byte buffer to int16 samples assuming even length. If `len(audioData)` is odd (from truncated network packet), the last byte is silently dropped. **Remediation:** Validate `len(audioData) % 2 == 0` or return error.

- [ ] **M-08: Lock/unlock without defer in Message.SetState** — `messaging/message.go:310-318` — Concurrency/Resources — Manual unlock pattern without defer. If future modifications between lock and unlock introduce panic-able code, the lock leaks. Current code is safe but fragile. **Remediation:** Use defer unlock pattern.

- [ ] **M-09: fmt.Printf used instead of structured logger** — `async/prekeys.go:346,654,688` — Error Handling/Logging — Three locations use `fmt.Printf()` for warnings instead of the project's structured logger (logrus), bypassing log level filtering and structured fields. **Remediation:** Replace with `logrus.Warn()` or `logrus.WithField(...).Warn()`.

### LOW

- [ ] **L-01: Redundant zero-before-delete in skipped keys map** — `ratchet/skipped.go:45,61` — Logic — Code zeros map values (`s.keys[k] = [32]byte{}`) before `delete(s.keys, k)`. The delete alone removes the reference; zeroing the map value is ineffective for security because Go's map implementation may retain the internal bucket memory. **Remediation:** If security wiping is intended, wipe the actual key bytes via a pointer before delete; otherwise remove the redundant assignment.

- [ ] **L-02: Duplicate validation functions** — `toxcore_messaging.go:49-67` — API Contracts — `isValidMessage()` and `validateMessageInput()` implement overlapping validation logic. The latter wraps the former but adds error messages. Both are internal, but the duplication increases maintenance burden. **Remediation:** Remove `isValidMessage` and use `validateMessageInput` throughout.

- [ ] **L-03: Potential goroutine leak in replay_protection cleanup** — `crypto/replay_protection.go:89` — Resources — `go ns.cleanupLoop()` is started during NonceStore creation. If `Close()` is never called (e.g., abandoned NonceStore), the goroutine runs indefinitely. **Remediation:** Document that Close() must be called; consider weak reference or finalizer.

- [ ] **L-04: Cache check outside lock in FindClosestNodes** — `dht/routing.go:386-388` — Concurrency — `getCachedClosestNodes()` is called before acquiring the read lock. Between cache check and lock acquisition, the routing table may change, leading to returning stale cached results. Impact is low because stale results are eventually refreshed. **Remediation:** Move cache check inside the read lock, or accept as documented eventual consistency.

## Metrics

| Metric | Value |
|--------|-------|
| Total source files | 266 |
| Total test files | 283 |
| Test/source ratio | 1.06 |
| go vet warnings | 0 |
| go test (nonet) | ✅ All pass |
| Packages with panics in non-init | 1 (crypto/secure_memory.go) |
| Packages with init() panics | 2 (dht/mdns_discovery.go, transport/nat.go) |
| Sync primitives usage | 222 instances |
| Goroutine launches (non-test) | 38 |
| math/rand in production | 0 (✅ crypto/rand only) |
| InsecureSkipVerify | 0 (✅ none) |

## False Positives Rejected

| Candidate | Reason |
|-----------|--------|
| sealed_sender.go ZeroBytes corrupts caller data | `senderIdentityPublic [32]byte` is passed by value; slice references local copy on stack, not caller's data |
| generateUniqueCallID race condition | Called only from StartCall which holds `m.mu.Lock()` via defer; no unsynchronized path |
| k-bucket nil node dereference | AddNode validates node is non-nil before insertion; no path inserts nil |
| Slice corruption in friend RejectRequest | Operation is within mutex-protected section; no concurrent access possible |
| relay.go:138-144 panic on nil conn | Close() only called when conn is established; nil check present |
| RTP session audioDepacketizer TOCTOU | Close() acquires full write lock before nil-setting; ReceivePacket acquires read lock; mutually exclusive |
| Unbuffered readNotify channel panic | `broadcastRead()` replaces channel atomically via mutex; closed-channel receive returns zero value, doesn't panic |

## Remaining Scope

All packages audited — no remaining scope.
