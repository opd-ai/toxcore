# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-28

## Project Profile
- **Project**: `github.com/opd-ai/toxcore` (toxcore-go) — a pure-Go implementation of the Tox
  peer-to-peer encrypted messaging protocol.
- **Purpose**: DHT-based peer discovery, friend management, 1-to-1 and group messaging, file
  transfers, ToxAV audio/video calling, asynchronous offline messaging with forward secrecy,
  and multi-network transport (UDP/TCP/Tor/I2P/Lokinet/Nym). No cgo in the core library
  (cgo only for the `capi/` C bindings).
- **Target users**: Go application developers embedding Tox; C/C++ programs via the
  libtoxcore-compatible `capi/` shared library.
- **Deployment model**: Library linked into a host process that runs an event loop
  (`tox.Iterate()`), participating in an open P2P network. **Untrusted input arrives from the
  network on every transport/DHT/AV/messaging path**, so packet-parsing code is the primary
  trust boundary.
- **Critical paths**: `transport/` (packet parsing, Noise handshakes), `dht/` (node parsing,
  routing table), `crypto/` (key management, AEAD), `async/` (offline message crypto),
  `messaging/`/`file/`/`group/` (peer-controlled payloads), `av/` (RTP from network).

## Audit Scope
- **Go version**: 1.25.0 (toolchain go1.25.8). Dependencies: `flynn/noise`, `klauspost/reedsolomon`,
  `opd-ai/magnum` (Opus), `opd-ai/vp8`, `pion/rtp`, `sirupsen/logrus`, `golang.org/x/crypto`, etc.
- **Method**: full read-only pass of every non-test `.go` file in the core packages, combining
  `go-stats-generator` structural metrics, `go vet`, `go test -race -tags nonet ./...`, six
  parallel package-scoped review agents, and **manual verification of every candidate finding**
  (data-flow tracing, upstream-guard checks, reachability confirmation).
- **go-stats-generator metrics**: 42,492 LOC, 1,166 functions, 2,899 methods, 408 structs,
  38 interfaces, 26 packages, 238 files. Only **2 functions exceed cyclomatic complexity 15**;
  average cyclomatic complexity **2.42**. Documentation coverage **93.3%** overall. Duplication
  ratio **0.45%** (largest clone 14 lines). The codebase is well-factored and low-complexity.

### High-risk functions inspected (complexity > 15 or notable)
- `crypto/keystore.go:389 reencryptWithNewKey` (cx 22, 95 ln) — inspected; three-phase commit
  with rollback is sound. One LOW hygiene note below.
- `toxnet/conn.go:146 waitForDataSignal` (cx 16) — inspected; `Read` contract is correct
  (rejected as false positive below).

## Coverage Log
Legend: ✅ category fully reviewed for the package · n/a category not applicable.

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport/internal/addressing | ✅ | ✅ | ✅ | n/a | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bootstrap | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av, av/audio, av/video, av/rtp | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| factory, interfaces, limits, real, simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| examples/*, cmd/* | partial | partial | partial | partial | partial | ✅ | partial | partial | partial |

`examples/` and `cmd/` are demonstration/tooling code (not library APIs); they received a
security-focused pass only. See **Remaining Scope**.

## Goal-Achievement Summary
| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| DHT peer discovery & routing | ✅ | none (LOW: dead-code count guard) |
| Friend management | ✅ | none |
| 1-to-1 encrypted messaging + async fallback | ✅ | none |
| Group chat with role permissions | ✅ | none (role checks verified correct) |
| File transfers (path-traversal safe) | ✅ | none (defenses verified) |
| ToxAV audio/video calling | ⚠️ | Inter-frame decode not implemented (documented); see GAPS.md |
| Async offline messaging / forward secrecy | ✅ | none |
| Multi-network transport (UDP/TCP/Tor/I2P/Lokinet/Nym) | ✅ | none |
| Noise-IK handshakes | ✅ | none |
| Linux socket receive-buffer tuning | ❌ | F-1: `SetSocketReceiveBuffer` panics on every real conn |
| C API bindings | ✅ | none (defensive pointer handling verified) |
| No race conditions (`-race` clean) | ✅ | `go test -race -tags nonet ./...` passes (33 pkgs) |

## Findings

All findings were verified by tracing the data flow, checking upstream guards, reading
acknowledging comments, and confirming reachability. Where a defect exists only in code not
yet wired into a production call path, it is labelled **latent** and rated accordingly.

### CRITICAL
- [ ] None. No confirmed bug on a critical goal path, no data-corruption or exploitable
  security issue with a traceable network data flow was found. (`go vet` clean; `go test -race
  -tags nonet ./...` passes for all 33 testable packages.)

### HIGH
- [ ] None confirmed. Several candidates initially rated HIGH by automated review agents were
  downgraded or rejected after manual verification (see *False Positives* and *MEDIUM/LOW*).

### MEDIUM
- [x] **F-1: `SetSocketReceiveBuffer` panics on every valid connection** — `transport/batch_receive_linux.go:465` — Logic / Nil-and-boundary (bad type assertion) — Fixed: replaced incorrect interface declaration with proper `syscall.RawConn` type, added two-value assertion check to return error instead of panicking.

- [x] **F-2: `RelayMux.handleStreamOpen` overwrites the per-peer stream entry, breaking the one-stream-per-peer invariant** — `transport/relay_mux.go:410-434` — Logic / data-structure consistency (latent) — Fixed: added check for existing stream in `streamsByKey` and reuse existing stream on glare (simultaneous-open) to mirror the dedup logic in `OpenStream`.

### LOW
- [x] **F-3: Dead-code node-count guard** — `dht/handler.go:57-60` (and the equivalent in `detectProtocolVersionFromPacket`, ~`handler.go:455`) — Logic (unreachable branch) — Fixed: removed the impossible `< 0` check and added explanatory comment clarifying that numNodes is always 0-255 (byte range).

- [x] **F-4: `GetAdditionRate` releases its read lock mid-function (TOCTOU window)** — `dht/dynamic_bucket.go:127-152` — Concurrency (lock dance) — Fixed: hold a single `Lock()` for the whole prune-then-compute sequence instead of releasing and re-acquiring the lock mid-function, eliminating the TOCTOU window.

- [x] **F-5: `BootstrapManager` dereferences `routingTable` without a nil guard** — `dht/handler.go:291` (`processSender`), and similarly `:431`, `:527`, `:561`, `:778` — Nil safety (defensive) — Fixed: added nil checks with clear panic messages in all three BootstrapManager constructors (NewBootstrapManager, NewBootstrapManagerWithKeyPair, NewBootstrapManagerForTesting).

- [x] **F-6: `PriorityQueue.Enqueue` dereferences `msg.ID` before a nil check** — `messaging/priority_queue.go:177` — Nil safety / API contract (defensive) — Fixed: added nil check at the start of Enqueue to return false if msg is nil.

- [x] **F-7: `DeserializeSenderKeyDistribution` lacks the size sanity guard its sibling has** — `group/sender_key.go:649` — Logic / boundary (latent, 32-bit) — Fixed: added 16 MB cap check (consistent with `DeserializeSenderKeyMessage`) before attempting to allocate `EncryptedKey`.

- [x] **F-8: `Tox.GetSavedata` swallows the marshal error and returns `nil`** — `toxcore.go:445-450` — Error handling — Fixed: added comprehensive GoDoc documentation explaining that nil is returned on error (matching libtoxcore convention) and directing callers needing error details to use `Save()` instead.

- [x] **F-9: Untracked background goroutine in listener accept path** — `toxnet/listener.go:80` — Concurrency (lifecycle) — Fixed: added `sync.WaitGroup` to ToxListener struct and tracked goroutine spawning; `Close()` now calls `goroutineWg.Wait()` after context cancellation for deterministic shutdown.

- [x] **F-10: `NewRelayMux` panics on invalid configuration instead of returning an error** — `transport/relay_mux.go:157-163` (`validateMuxConfig`) — API contract (latent) — Fixed: changed `validateMuxConfig` to return error instead of panicking; updated `NewRelayMux` signature to return `(*RelayMux, error)` for proper error propagation; updated all test cases accordingly.

- [x] **F-11: Sensitive plaintext not wiped on the key-rotation error path** — `crypto/keystore.go:403-417` — Security hygiene — Fixed: added loop to wipe all remaining plaintexts from fileData in the Phase-1 error branch before returning.

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total functions | 1,166 |
| Total methods | 2,899 |
| Functions above cyclomatic complexity 15 | 2 (`crypto/keystore.go:389`=22, `toxnet/conn.go:146`=16) |
| Avg cyclomatic complexity | 2.42 |
| Doc coverage (overall) | 93.3% (packages 100%, functions 98.7%) |
| Duplication ratio | 0.45% (largest clone 14 lines, 26 clone pairs) |
| Test pass rate | 33/33 testable packages pass under `go test -race -tags nonet ./...` |
| `go vet ./...` warnings | 0 |
| TODO/FIXME/HACK code comments | 0 |

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|----------------|
| `av/rtp/packet.go:634` — jitter buffer stores RTP payload slice without copy (claimed HIGH aliasing) | `transport/udp.go:204` reuses one read buffer, but `ParsePacket` (`transport/packet.go`) does `Data: make(...)` + `copy`, so `Packet.Data` (hence `packet.Payload`) is a **fresh per-packet allocation**, not the shared buffer. No corruption. |
| `av/video/rtp.go:274-294` — `frameBuffer` map access without a mutex (claimed CRITICAL race) | All mutations occur inside `ProcessPacket`, invoked only from the single UDP receive goroutine (`transport/udp.go` `processPackets`), which dispatches handlers synchronously. The only concurrent reader (`GetRTPStats`→`GetBufferedFrameCount`) has no non-test callers. Not concurrently reachable today; `-race` suite is clean. (Latent: struct is unsynchronized — see GAPS.md.) |
| `crypto/key_rotation.go:139` — `key.Public` deref without nil check (claimed CRITICAL) | `FindKeyForPublicKey` holds `RLock`; `Cleanup` (which sets entries to nil) holds the write `Lock` — they are mutually exclusive. `previousKeys` is only ever appended with the non-nil `currentKeyPair` (line 75) and nilled wholesale under the write lock. A nil element is never observable under `RLock`. |
| `dht/relay_storage.go:423` — `count := int(packet.Data[0])` used unchecked (claimed signed/unsigned bug) | `parseRelayAnnouncements` bounds-checks every read (`offset < len(data)`, `offset+2 > len(data)`, `offset+announcementLen > len(data)`) and breaks when data is exhausted, so an inflated `count` is harmless. |
| `dht/dynamic_bucket.go:127` — mismatched lock/unlock causing panic (claimed CRITICAL) | RWMutex has no nesting levels; the RLock/RUnlock and Lock/Unlock counts are balanced. No panic or mutex corruption — only a benign TOCTOU window (recorded as LOW F-4). |
| `iteration_pipelines.go:287,291` — division-by-zero / `NewTicker(0)` panic | `NewIterationPipelines` (lines 98-105) resolves any zero `MessageInterval`/`DHTInterval`/`FriendInterval` to defaults before the pipeline runs, so the divisor and ticker interval are always positive. |
| `toxnet/conn.go:95-99` — `validateReadInput` returns `-1` ("continue") (claimed logic bug) | `Read` checks `if n >= 0 { return n, err }`; `(0,nil)` correctly returns a 0-length read and `-1` correctly signals "continue". Contract is consistent with `net.Conn`. |
| `toxnet/listener.go:114-124` — ticker not stopped on timeout (claimed leak) | `waitAndCreateConnection` uses `defer l.cleanupTimers(timeout, ticker)` (line 96), which stops both the timer and ticker on every return path. |
| `capi/toxcore_c.go:115-149` — `safeGetToxID` recovers a panic on a bad C pointer | Intentional defensive handling at the cgo boundary; converts an invalid/stale C pointer into `(nil, false)` instead of crashing. Acknowledged design. |
| `crypto/secure_memory.go:48` — `panic` when `SecureWipe` fails | Intentional security invariant: if key material cannot be wiped, failing fast is the correct behavior (acknowledged in comments). |
| `transport/nat.go:23-30`, `dht/mdns_discovery.go:41-49` — `panic` in `init()` | `net.ResolveUDPAddr` on hard-coded literal addresses (`203.0.113.1:0`, mDNS multicast) cannot fail; comments explicitly mark these as invariant assertions. |
| `async/retrieval_scheduler.go:149` — `_, _ = ...RetrieveObfuscatedMessages()` | Intentional cover traffic; swallowing the result is required so cover requests are indistinguishable from real ones (acknowledged). |
| File-transfer path traversal (`file/transfer.go`) | Defended: `deserializeFileRequest` strips peer names with `filepath.Base` (`file/manager.go:628`), and `Start` calls `validateAndSanitizePath`→`ValidatePath` (rejects absolute paths and `..`) **before** `openTransferFile`. |
| `group/chat.go` role/permission checks | Verified correct: `Role < requiredRole` rejects insufficient privilege and `Role <= targetPeer.Role` prevents acting on equal/higher-role peers, with founder protection enforced. |
| `math/rand` for security | Only used in `examples/av_quality_monitor`; all security-sensitive randomness uses `crypto/rand` (21 files). |

## Remaining Scope
The core library (all packages in the Coverage Log marked ✅) received a complete pass; a final
full re-pass produced no new confirmed findings above LOW. The following received only a
security-focused (not exhaustive) review and are the recommended resume points:

| Package | Status | Notes |
|---------|--------|-------|
| `examples/*` (≈30 demo programs) | Partially audited | Demonstration code, not a library API. Security pass done (no `math/rand`-for-security except the documented quality monitor, no hardcoded secrets, no path traversal). Full logic/concurrency pass not performed. |
| `cmd/gen-bootstrap-nodes` | Partially audited | Build-time tool; `exec.Command("gofmt", ...)` uses a fixed binary, no untrusted args. |
| `testnet/` (separate module) | Not audited | Separate Go module / tooling; outside the main module's scope. |
