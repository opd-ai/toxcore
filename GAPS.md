# Implementation Gaps — 2026-06-16

This document identifies discrepancies between toxcore-go's stated goals (from README, docs, and design documents) and the current implementation.

---

## Gap 1: Panic-Based Error Handling Contradicts Library Reliability Goal

- **Stated Goal**: toxcore-go is a library for building applications; libraries should never crash the host process.
- **Current State**: Three `init()` functions and one utility function (`ZeroBytes`) use `panic()`:
  - `dht/mdns_discovery.go:48,51` — panics on mDNS address resolution failure
  - `transport/nat.go:27` — panics on NAT fallback address resolution failure
  - `crypto/secure_memory.go:48` — panics in `ZeroBytes()` if `SecureWipe()` fails
- **Impact**: Applications importing the `dht` or `transport` packages on systems without IPv6 or unusual network configurations crash at startup. Applications using `defer ZeroBytes(...)` can crash during cleanup on constrained systems.
- **Remediation**: Replace `init()` panics with lazy initialization that returns errors. Change `ZeroBytes` to either return an error or log and continue.

---

## Gap 2: Routing Table Lock Safety vs. DHT Reliability Goal

- **Stated Goal**: Reliable DHT-based peer discovery that operates continuously without intervention (README: "DHT Routing — Modified Kademlia DHT for serverless peer discovery").
- **Current State**: `dht/routing.go:311-313` and `390-394` use manual mutex unlock patterns without `defer`. Any panic in the locked section permanently deadlocks the routing table.
- **Impact**: A single unexpected panic in node addition or lookup kills all DHT operations for the lifetime of the process. Given that the DHT is the foundation of peer discovery, this makes the entire Tox instance non-functional.
- **Remediation**: Use `defer` for all mutex unlock operations in the routing table.

---

## Gap 3: File Transfer Race Window vs. Reliability Goal

- **Stated Goal**: "Bidirectional chunked file transfers with pause, resume, cancellation, and progress tracking" (README).
- **Current State**: `file/transfer.go:572-576` releases the transfer mutex before invoking progress callbacks, creating a race window where concurrent Pause/Cancel operations can interleave. This is documented as intentional (M-FILE-3) to prevent deadlocks.
- **Impact**: Concurrent file transfer operations (e.g., cancellation during progress callback) may observe inconsistent state. The transfer may continue processing chunks briefly after cancellation is requested.
- **Remediation**: Use a state snapshot approach — capture all callback-visible state under the lock, release, invoke callback with snapshot. After relock, verify state hasn't been invalidated before continuing.

---

## Gap 4: Relay Disconnect Reliability vs. Clean Session Teardown

- **Stated Goal**: TCP relay support for NAT traversal with proper session management (README: "TCP relay fallback for symmetric NAT").
- **Current State**: `transport/relay.go:628` discards the error from `Write(disconnectPacket)`. The relay server may not receive the disconnect signal, leaving stale sessions.
- **Impact**: Relay servers accumulate stale sessions until their own timeout expires. On relay servers with many clients, this can exhaust connection slots.
- **Remediation**: Log the disconnect write error for observability. Consider a retry with short timeout.

---

## Gap 5: Long-Running AV Calls vs. RTP Timestamp Integrity

- **Stated Goal**: "Peer-to-peer calling with Opus audio encoding... RTP transport, adaptive bitrate, and jitter buffering" (README).
- **Current State**: `av/rtp/session.go:362-363` computes video timestamps as `uint32(elapsed.Milliseconds() * 90)`. After ~47.7 days of continuous call, the timestamp wraps around. There is no evidence that the receiving jitter buffer handles timestamp wrap-around correctly.
- **Impact**: Video calls lasting more than ~47 days may experience frame reordering or jitter buffer corruption when the timestamp wraps. This is an edge case for most usage but relevant for persistent surveillance/monitoring applications.
- **Remediation**: Verify jitter buffer uses modular arithmetic for timestamp comparison; add integration test for wrap-around scenario.

---

## Gap 6: Pre-Key Rate Limiter Memory Growth vs. Scalability Goal

- **Stated Goal**: "Asynchronous offline messaging with forward secrecy via one-time pre-keys" (README). Designed for long-running nodes with many peers.
- **Current State**: `async/forward_secrecy.go:365-372` accumulates memory in the `preKeyConsumed` map due to slice aliasing. The backing array of the time slice grows indefinitely even though only a small window of entries is logically active.
- **Impact**: Long-running nodes communicating with many peers experience gradual memory growth in the forward secrecy subsystem. Growth rate is proportional to message frequency × peer count.
- **Remediation**: Replace reslicing with fresh allocation when the window slides forward.

---

## Gap 7: Structured Logging Convention Inconsistency

- **Stated Goal**: Project uses `github.com/sirupsen/logrus` for structured logging throughout (evidenced by 200+ logrus references in production code).
- **Current State**: `async/prekeys.go:346,654,688` uses `fmt.Printf()` for warning messages, bypassing the structured logging infrastructure.
- **Impact**: These warnings are invisible to applications that configure logrus output (custom formatters, log aggregation, level filtering). Operational visibility is reduced.
- **Remediation**: Replace `fmt.Printf` calls with appropriate `logrus.WithFields(...).Warn(...)` calls.

---

## Summary

| Gap | Severity | Effort to Fix |
|-----|----------|---------------|
| Gap 1: Panic in library code | HIGH | Low (replace panic with lazy init) |
| Gap 2: Routing table lock safety | HIGH | Low (add defer) |
| Gap 3: File transfer race window | MEDIUM | Medium (redesign callback invocation) |
| Gap 4: Relay disconnect reliability | MEDIUM | Low (add error logging) |
| Gap 5: RTP timestamp overflow | LOW | Medium (verify jitter buffer) |
| Gap 6: Pre-key memory growth | MEDIUM | Low (fresh slice allocation) |
| Gap 7: Logging inconsistency | LOW | Low (replace fmt.Printf) |
