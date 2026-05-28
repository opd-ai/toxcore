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
- [ ] **F-1: `SetSocketReceiveBuffer` panics on every valid connection** — `transport/batch_receive_linux.go:465` — Logic / Nil-and-boundary (bad type assertion) — The function does `conn.(interface{ SyscallConn() (interface{}, error) }).SyscallConn()`. The interface literal declares the return type as `interface{}`, but real connections (`*net.UDPConn`) implement `SyscallConn() (syscall.RawConn, error)`. Go interface satisfaction requires an exact method-signature match, so **no real `net.PacketConn` satisfies this interface**; the *single-value* assertion therefore panics (`interface conversion: ... is not interface { SyscallConn(...) }`). Verified empirically: `c.(interface{ SyscallConn() (interface{}, error) })` returns `ok=false` for a live UDP socket. **Consequence**: the exported, documented Linux tuning helper is non-functional and crashes the caller for all valid input. Reachability is limited (no internal callers), which caps severity at MEDIUM. **Remediation**: use the correct type `conn.(interface{ SyscallConn() (syscall.RawConn, error) })` (or the standard `syscall.Conn` interface) with the two-value form `rc, ok := conn.(syscall.Conn)`; return an error when `!ok`. Validate with a unit test that calls `SetSocketReceiveBuffer` on a real `net.ListenPacket("udp", ...)` conn and asserts no panic: `go test -tags nonet ./transport/...`.

- [ ] **F-2: `RelayMux.handleStreamOpen` overwrites the per-peer stream entry, breaking the one-stream-per-peer invariant** — `transport/relay_mux.go:410-434` — Logic / data-structure consistency (latent) — `OpenStream` (line 177) enforces one stream per peer by returning the existing entry in `streamsByKey[peerKey]`. `handleStreamOpen` (inbound stream-open from a peer) unconditionally calls `createStream` and assigns `m.streamsByKey[peerKey] = stream` without checking for an existing entry. On a simultaneous-open ("glare") between two peers, the locally-opened stream is orphaned: it remains in `m.streams[oldStreamID]` (still routable by ID) but is no longer reachable via `streamsByKey`, and it is never removed until `Close`/`removeStream` is called for its ID. **Consequence**: the dedup guarantee `OpenStream` relies on is violated, and the orphaned `MuxStream` (with its channels/buffers) is retained until shutdown. It is bounded by `MaxStreams` (line 425 rejects new opens once the cap is hit), so it is not an unbounded leak. **Latent**: `NewRelayMux` has no non-test callers, so the multiplexer is not currently wired into production. **Remediation**: in `handleStreamOpen`, before creating a stream, check `if existing, ok := m.streamsByKey[peerKey]; ok { ... }` and either reuse/replace-and-close the existing stream or reject the duplicate; mirror the dedup logic in `OpenStream`. Validate: `go test -tags nonet ./transport/...`.

### LOW
- [ ] **F-3: Dead-code node-count guard** — `dht/handler.go:57-60` (and the equivalent in `detectProtocolVersionFromPacket`, ~`handler.go:455`) — Logic (unreachable branch) — `numNodes := int(packet.Data[32]); if numNodes < 0 { return ... }`. `packet.Data[32]` is a `byte`, so `int(byte)` is always `0..255` and the `< 0` branch is unreachable. The real bounds are enforced later in `processReceivedNodesWithVersionDetection`, so there is no over-read, but the guard is misleading dead code. **Remediation**: remove the impossible `< 0` check or replace it with a meaningful upper-bound sanity check against the packet length. Validate: `go vet ./dht/... && go test -tags nonet ./dht/...`.

- [ ] **F-4: `GetAdditionRate` releases its read lock mid-function (TOCTOU window)** — `dht/dynamic_bucket.go:127-152` — Concurrency (lock dance) — The method takes `RLock` (deferred `RUnlock`), then explicitly `RUnlock()`s, takes `Lock()` to prune, `Unlock()`s, and re-acquires `RLock()`. The RLock/RUnlock/Lock/Unlock counts are balanced (no panic, no deadlock), but the lock is fully released between the prune and the final read, creating a small TOCTOU window. The value read (`count`) is captured under the write lock, so the computed rate is internally consistent; impact is limited to a momentarily stale rate. **Remediation**: hold a single `Lock()` for the whole prune-then-compute sequence (the prune already needs the write lock), eliminating the release/re-acquire dance. Validate: `go test -race -tags nonet ./dht/...`.

- [ ] **F-5: `BootstrapManager` dereferences `routingTable` without a nil guard** — `dht/handler.go:291` (`processSender`), and similarly `:431`, `:527`, `:561`, `:778` — Nil safety (defensive) — `bm.routingTable.AddNode(...)` is called without a nil check, and `NewBootstrapManager` (`dht/bootstrap.go:124`) stores the `routingTable` argument without validating it. All in-tree callers pass a non-nil table, so this is reachable only by a programming error (passing `nil`), **not** from untrusted input. Note `relay_storage.go:430` already guards `if bm.routingTable != nil`, so the convention is inconsistent. **Remediation**: validate `routingTable != nil` in the constructors (return error or panic with a clear message), or add a nil guard consistent with `relay_storage.go`. Validate: `go test -tags nonet ./dht/...`.

- [ ] **F-6: `PriorityQueue.Enqueue` dereferences `msg.ID` before a nil check** — `messaging/priority_queue.go:177` — Nil safety / API contract (defensive) — When the queue is full, the debug log reads `msg.ID`; if a caller passes `msg == nil`, this panics. The non-full path also stores `msg` without a nil check, so a nil message would later panic in heap comparisons/consumers. `Enqueue` is exported (and `//export ToxPriorityQueueEnqueue`), but it is only reached via internal `EnqueueWithDefault` and is never fed nil or untrusted data in-tree. **Remediation**: add `if msg == nil { return false }` at the top of `Enqueue`. Validate: `go test -tags nonet ./messaging/...`.

- [ ] **F-7: `DeserializeSenderKeyDistribution` lacks the size sanity guard its sibling has** — `group/sender_key.go:649` — Logic / boundary (latent, 32-bit) — `dist.EncryptedKey = make([]byte, encKeyLen)` where `encKeyLen` is a `uint32` from the packet. Line 645 checks `len(data) < offset+int(encKeyLen)` first, which bounds `encKeyLen` by the packet length on 64-bit platforms. However, on 32-bit platforms `int(encKeyLen)` can sign-flip to a negative value for `encKeyLen > MaxInt32`, defeating the length check and reaching `make` with a negative/huge size (panic). The sibling `DeserializeSenderKeyMessage` (line 585) guards with a 16 MB cap; this function does not, making the two inconsistent. **Latent**: currently only invoked from `group/sender_key_test.go` — not wired into a packet handler. **Remediation**: add a `const maxEncKeyLen` (e.g. 16 MB) cap and reject larger values before `make`, mirroring `DeserializeSenderKeyMessage`. Validate: `go test -tags nonet ./group/...`.

- [ ] **F-8: `Tox.GetSavedata` swallows the marshal error and returns `nil`** — `toxcore.go:445-450` — Error handling — On `saveData.marshal()` failure the method logs and `return nil`, so a caller cannot distinguish "serialization failed" from "no data". The error-returning alternative `Tox.Save() ([]byte, error)` (`toxcore_lifecycle.go:312`) exists, and the `nil`-return shape matches the libtoxcore `tox_get_savedata` convention, which is why this is LOW rather than higher. **Remediation**: document the `nil`-on-error behavior in the `GetSavedata` GoDoc and steer callers needing error detail to `Save()`; optionally have `GetSavedata` delegate to `Save()` and drop the error for compatibility. Validate: `go test -tags nonet .`.

- [ ] **F-9: Untracked background goroutine in listener accept path** — `toxnet/listener.go:80` — Concurrency (lifecycle) — `acceptFriendRequest` launches `go l.waitAndCreateConnection(...)` without registering it in a `sync.WaitGroup`. The goroutine *does* observe cancellation (its monitor loop selects on `<-l.ctx.Done()` in `shouldStopMonitoring`, `listener.go`), so it is **not** a true leak, but `Close()` does not wait for in-flight accept goroutines to drain, so shutdown is not fully synchronous. **Remediation**: track the goroutine with a `sync.WaitGroup` and `Wait()` in `Close()` for deterministic shutdown. Validate: `go test -race -tags nonet ./toxnet/...`.

- [ ] **F-10: `NewRelayMux` panics on invalid configuration instead of returning an error** — `transport/relay_mux.go:157-163` (`validateMuxConfig`) — API contract (latent) — An exported constructor `panic`s when given an out-of-range `MuxConfig`, rather than returning `(*RelayMux, error)`. This is an acknowledged invariant-style panic, but it is on an exported public constructor whose input could come from a host application's configuration. **Latent**: `NewRelayMux` has no non-test callers. **Remediation**: return a validation error from `NewRelayMux` rather than panicking. Validate: `go test -tags nonet ./transport/...`.

- [ ] **F-11: Sensitive plaintext not wiped on the key-rotation error path** — `crypto/keystore.go:403-417` — Security hygiene — In `reencryptWithNewKey` Phase 1, `SecureWipe(plaintext)` runs only after a successful `WriteEncrypted`; if a write fails mid-loop, the function returns immediately and the remaining decrypted key plaintexts in `fileData` are left un-wiped in memory. The repository convention is to wipe sensitive buffers with `crypto.SecureWipe`/`crypto.ZeroBytes`. **Remediation**: wipe all remaining `fileData` plaintexts in the Phase-1 failure branch before returning. Validate: `go test -tags nonet ./crypto/...`.

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
