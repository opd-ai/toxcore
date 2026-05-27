# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-27

## Project Profile
- **Module**: `github.com/opd-ai/toxcore` — a pure-Go reimplementation of the Tox
  peer-to-peer encrypted messaging protocol (Go 1.25.0, toolchain 1.25.8).
- **Purpose** (from `README.md`): DHT peer discovery, friend management, 1-to-1
  and group messaging, file transfers, ToxAV audio/video, asynchronous offline
  messaging with forward secrecy, multi-network transport (UDP/TCP/Tor/I2P/Lokinet/Nym),
  Noise-IK handshakes, NAT traversal, libtoxcore-compatible C bindings.
- **Deployment model**: Library embedded by client applications; long-lived
  process; untrusted input enters from the network on every UDP/TCP packet.
- **Critical paths**: `crypto/` (trust anchor), `transport/` (packet I/O,
  Noise sessions), `dht/` (peer routing), `async/` (offline messaging, prekeys),
  `noise/`, `friend/`, `messaging/`, `file/`, `av/` (RTP frames), `capi/`
  (C ABI surface that mirrors libtoxcore semantics).

## Audit Scope
- 238 Go source files (excluding tests counted: 1161 functions, 2890 methods,
  407 structs, 37 interfaces, 26 packages, 42,294 LOC).
- All non-test, non-`examples/` packages received a category-by-category pass
  across §3b–§3j of the checklist; `examples/` packages were spot-checked for
  obvious crashes only because they are demos and do not ship in the library.
- Tooling: `go-stats-generator analyze . --skip-tests`, `go vet ./...`
  (clean), `go test -tags nonet -race -count=1 -timeout 600s ./...` (all
  packages pass).

## Coverage Log
Categories: 3b Logic · 3c Nil · 3d Errors · 3e Resources · 3f Concurrency ·
3g Security · 3h Aliasing · 3i Init · 3j API.

| Package | 3b | 3c | 3d | 3e | 3f | 3g | 3h | 3i | 3j |
|---------|----|----|----|----|----|----|----|----|----|
| `toxcore` (root)            | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `async`                     | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `av`, `av/audio`, `av/video`, `av/rtp` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `bootstrap`, `bootstrap/nodes` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `capi`                      | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `cmd/gen-bootstrap-nodes`   | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `crypto`                    | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `dht`                       | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `factory`                   | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `file`                      | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `friend`                    | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `group`                     | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `interfaces`                | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `limits`                    | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `messaging`                 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `noise`                     | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `privacy_networks`          | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `real`, `simulation`, `testnet` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `toxnet`                    | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `transport`                 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `addressing`, `common`      | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `examples/*`                | spot-check only (demo code; not shipped) |

## Goal-Achievement Summary
| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| DHT-based peer discovery (k-buckets, mDNS) | ✅ | — |
| Friend requests / contact list / connection tracking | ✅ | — |
| 1-to-1 encrypted messaging with delivery tracking | ✅ | — |
| Group chat with role-based permissions | ✅ | — |
| Bidirectional chunked file transfers | ✅ | — |
| ToxAV audio/video over RTP | ✅ | — |
| Asynchronous offline messaging with forward secrecy | ⚠️ | F-ASYNC-001 (queued-message retention semantics undocumented), F-ASYNC-002 (shallow-copied retrieved messages) |
| Multi-network transport (UDP/TCP/Tor/I2P/Lokinet/Nym) | ✅ | — |
| Noise-IK handshakes (forward secrecy, KCI resistance) | ✅ | — |
| NAT traversal (STUN/UPnP/hole-punching/TCP relay) | ⚠️ | F-TRANSPORT-001 (lock-release-reacquire in `GetPublicAddress`) |
| Cryptography (Curve25519, ChaCha20-Poly1305, Ed25519, replay protection, secure wipe) | ⚠️ | F-CRYPTO-001 (`NonceStore.Close` panics on double-call) |
| libtoxcore-compatible C API | ⚠️ | F-CAPI-001 (cgo callback pointer-retention contract undocumented), F-CAPI-002 (`cmd/gen-bootstrap-nodes` path heuristic) |
| Concurrent iteration pipelines | ✅ | — |

## Findings

### CRITICAL

- [ ] **F-CRYPTO-001** — `NonceStore.Close` panics on second invocation —
  `crypto/replay_protection.go:294-301` — *Resource Lifecycle / Concurrency* —
  `Close()` unconditionally `close(ns.stopChan)`. A second `Close()` (e.g. from
  `defer` in a wrapper plus an explicit shutdown path) panics with
  `close of closed channel`. Because `NonceStore` is owned by the long-lived
  transport layer and the only fail-safe shutdown is `defer`, this turns any
  duplicate teardown into a process-killing panic.
  **Data flow**: caller A calls `Close()` (intentional shutdown) → channel
  closed → caller B (deferred safety net) calls `Close()` → `close(ns.stopChan)`
  panics → entire process aborts, dropping all in-flight Tox traffic.
  **Remediation**: protect the close with `sync.Once`:
  ```go
  type NonceStore struct { /* ... */ closeOnce sync.Once }
  func (ns *NonceStore) Close() error {
      var saveErr error
      ns.closeOnce.Do(func() {
          close(ns.stopChan)
          ns.mu.RLock(); defer ns.mu.RUnlock()
          saveErr = ns.save()
      })
      return saveErr
  }
  ```
  **Validation**: extend `crypto/replay_protection_test.go` with
  `TestNonceStoreCloseIdempotent` that calls `Close()` twice and asserts
  no panic; run `go test -race ./crypto/...`.

### HIGH

*(none confirmed in this pass)*

### MEDIUM

- [ ] **F-ASYNC-002** — `RetrieveMessagesByPseudonym` returns shallow copies of
  stored messages — `async/storage.go:670-695` — *Data Aliasing* — The function
  copies `AsyncMessage` structs by value into the returned slice, but
  `EncryptedPayload []byte` is a slice header that still aliases the
  storage-internal buffer. A caller that mutates the returned `EncryptedPayload`
  (e.g. in-place decryption or scrubbing) corrupts the storage node's copy and
  poisons every subsequent retrieval for the same pseudonym. Existing tests do
  not mutate the returned data, so the race is latent.
  **Remediation**: deep-copy `EncryptedPayload` (and any other slice fields
  added later) before appending to the return slice — `payload := append([]byte(nil), msg.EncryptedPayload...)`. **Validation**: add a regression test that
  mutates a retrieved payload and asserts a subsequent retrieval still sees the
  original bytes; run `go test -race ./async/...`.

- [ ] **F-TRANSPORT-001** — `NAT.GetPublicAddress` releases its lock before
  fanning out to detection — `transport/nat.go:155-169` — *Concurrency* —
  The function reads `n.publicAddr` under a read lock, releases it, and then
  calls `n.DetectNATType(...)` which re-acquires the same lock. Between the
  release and the re-acquisition, a concurrent `SetPublicAddress` or a
  refresh from `StartPeriodicDetectionWithCallback` can install a different
  address; the returned tuple `(addr, natType)` can therefore describe two
  different network states. This is harmless on a stable network but causes
  detection callbacks to occasionally report contradictory `(publicAddr, type)`
  pairs to consumers.
  **Remediation**: read `publicAddr` and decide `natType` under a single lock
  acquisition, or have `DetectNATType` accept the already-read address as a
  parameter so the second lock is unnecessary. **Validation**: `go test -race ./transport/... -run TestNAT -count=10`.

- [ ] **F-CAPI-001** — cgo audio/video receive callbacks pass pointers into Go
  slice memory to C without documenting the "must consume before return"
  contract — `capi/toxav_c.go:1196-1209` (audio) and `capi/toxav_c.go:1268-1307`
  (video planes) — *API / Documentation* — `bridgeAudioReceiveFrame` casts
  `&pcm[0]` and `&y[0]`, `&u[0]`, `&v[0]` to `unsafe.Pointer` and passes them
  into the registered C callback. Per cgo rules the memory is pinned only for
  the duration of the C call; if a C client retains the pointer beyond the
  callback's return (which the upstream `libtoxav` header does not forbid in
  text), the Go GC may reclaim or relocate the slice, leading to silent
  use-after-free. The library does not currently document the lifetime
  expectation, and no `runtime.Pinner` is used.
  **Remediation**: add a GoDoc comment on the exported `toxav_callback_*`
  surface stating that callback receivers must copy data before returning, **or**
  copy into `C.malloc`-allocated buffers (and `C.free` after the call). **Validation**: add a cgo test that calls the registered callback synchronously
  and confirms reads succeed within the call boundary; `go test ./capi/...`.

- [ ] **F-ASYNC-001** — Pre-key wait re-queues messages indefinitely when the
  peer never returns — `async/manager.go:888-902` — *API / Behavioural
  Contract* — When `sendQueuedMessages` times out waiting for a peer's
  pre-key channel, it logs and re-queues every queued message. There is no
  bounded retry, no exponential back-off, no per-message TTL, and no eventual
  drop. The behaviour is intentional (storage nodes can preserve messages
  across long offline periods) but is invisible to API consumers, who have no
  way to observe how many of their `SendAsyncMessage` calls are stuck in
  `pendingMessages`. This is the canonical "silent retention" antipattern
  flagged by §3d/§3j of the checklist.
  **Remediation**: expose a metric/inspector (`AsyncManager.QueueDepth() int`)
  and/or a per-message `Expiry time.Time` that lets the caller bound retention.
  Document the current behaviour in `async/doc.go`. **Validation**: add unit
  tests asserting that an enqueued message is visible to a public introspection
  API and that exceeding `MaxQueueAge` evicts it.

- [ ] **F-BOOTSTRAP-001** — Overlay listener goroutine can leak a listener when
  the start context times out — `bootstrap/server.go:519-526, 528-537` —
  *Resource Lifecycle* — `startOverlayListener` spawns a goroutine that calls
  `cfg.transport.Listen` and sends the result on a buffered channel. On the
  `startCtx.Done()` branch it calls `cfg.transport.Close()` (which usually
  unblocks `Listen` with an error) and returns. If, however, the underlying
  transport returns a successful listener concurrently with `Close()` (typical
  for Tor/I2P SAM bridges where `Listen` is already past the cancellation
  check), the listener is delivered to the buffered channel and never received
  by anyone; the FD escapes scope and is leaked until process exit.
  **Remediation**: drain `listenerCh` after the timeout (close the listener if
  one arrives), or replace the channel-based fan-in with a context-aware
  `Listen` that cooperates with `startCtx`. **Validation**:
  `go test ./bootstrap/... -run TestStartOverlayListenerTimeout`.

- [ ] **F-CAPI-002** — Output path heuristic in `cmd/gen-bootstrap-nodes`
  silently writes to the wrong directory when invoked outside the repo root —
  `cmd/gen-bootstrap-nodes/main.go:68-76` — *Logic* — The CLI defaults the
  output path to `bootstrap/nodes/default_nodes.go`, then inspects the current
  directory for a sibling `node_info.go` and rewrites `outPath` to
  `default_nodes.go`. Any directory that happens to contain a file named
  `node_info.go` (for example a user's own package) will cause the tool to
  silently overwrite that directory's `default_nodes.go` instead of the project
  bootstrap file.
  **Remediation**: derive the path from the executable's `runtime.GOFILE` /
  `go:generate` environment (`os.Getenv("GOFILE")`) or require an explicit
  `-out` flag. **Validation**: `go test ./cmd/gen-bootstrap-nodes/... -run TestOutPathResolution`.

### LOW

- [ ] **F-KEYROT-001** — `KeyRotationManager.previousKeys` prepend creates a
  shared-tail slice — `crypto/key_rotation.go:75` — *Data Aliasing* —
  `append([]*KeyPair{krm.currentKeyPair}, krm.previousKeys...)` produces a new
  header but shares the tail with the existing slice's backing array. While
  every mutation here is performed under `krm.mu`, a future caller that
  retains a reference to the old `previousKeys` slice and then appends to it
  (rare) would observe interleaved data. **Remediation**: when the lifetime
  rules ever change, switch to an explicit copy. No fix needed today.

- [ ] **F-VC-001** — Manual big-endian uint64 decoding in
  `ParseVersionCommitment` — `transport/version_commitment.go:113-120` —
  *Logic / Maintainability* — Eight-byte timestamp parsed by hand-rolled bit
  shifts; identical to and slightly more error-prone than
  `binary.BigEndian.Uint64(data[1:9])`. Currently correct.
  **Remediation**: replace the shifts with `binary.BigEndian.Uint64`.
  **Validation**: existing `transport/version_commitment_test.go` covers parity.

- [ ] **F-PREKEY-001** — `&now` returned from a stack-local in
  `async/prekeys.go:182` — *Code Smell* — Escape analysis correctly moves the
  value to the heap, so behaviour is correct; the pattern is fragile in the
  face of refactors. **Remediation**: return `time.Time` by value and let the
  caller take an address if needed.

- [ ] **F-PREKEY-002** — Shallow `PreKey` copy followed by selective overwrite
  in `async/prekeys.go:510-512` — *Code Smell* — The `*time.Time` `UsedAt`
  field aliases the original; safe because the early-return for "already used"
  prevents the copy when it would matter, but the pattern is easy to misread.
  **Remediation**: explicit field-by-field construction.

- [ ] **F-TOXNET-001** — `setupReadTimeout` captures the deadline, releases the
  lock, then constructs the timer — `toxnet/conn.go:115-134` — *Concurrency
  (TOCTOU)* — A concurrent `SetReadDeadline` between capture and timer
  construction will be ignored for the in-flight read. This matches the
  `net.Conn` documented snapshot semantics — the operation uses the deadline
  in effect when it was sampled — so it is intentional, but if the library
  wants stricter semantics the deadline read and timer creation should be
  fused. No fix required today.

- [ ] **F-STORAGE-001** — `CalculateAsyncStorageLimit` divides by 100 for the
  1 % calculation — `async/storage_limits.go:209` — *Logic* — Integer division
  rounds down; on extremely small volumes the result can be zero. The
  `MinStorageCapacity` clamp at line 318 saves us. Document the floor and
  add a unit test for `availableBytes < 100`.

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total files                        | 238 |
| Total LOC (excluding tests)        | 42,294 |
| Total functions                    | 1,161 |
| Total methods                      | 2,890 |
| Total structs / interfaces         | 407 / 37 |
| Total packages                     | 26 |
| Functions with cyclomatic > 10     | 5 |
| Functions > 50 lines               | 33 (0.8 %) |
| Functions > 100 lines              | 0 |
| Highest cyclomatic (overall)       | `crypto.reencryptWithNewKey` — 22 cyclomatic / 30.6 overall |
| Average cyclomatic complexity      | 3.6 |
| Average function length            | 12.7 lines |
| Duplication ratio                  | 0.43 % (25 clone pairs, largest 14 lines, all in `examples/`) |
| Naming-convention score            | 0.99 |
| Circular dependencies              | 0 |
| `go vet ./...`                     | clean (0 warnings) |
| `go test -tags nonet -race`        | all packages pass |

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|-----------------|
| `NonceStore.CheckAndStore` silently drops messages from clock-skewed peers (alleged in prior `GAPS.md`) | The current implementation (`crypto/replay_protection.go:92-133`) accepts any timestamp and delegates skew enforcement to the transport layer, exactly as documented in the comment at lines 94–96. All three referenced tests pass under `go test -race ./crypto/...`. |
| `file/transfer.go` `writeDataToFile` leaks file descriptors on write error (alleged in prior `GAPS.md`) | Lines 535-543 explicitly call `t.FileHandle.Close()` and nil the handle on the error path, matching the cleanup in `Cancel` and `completeLocked`. Verified by inspection. |
| `async.sendQueuedMessages` loses queued messages because `TestQueuedMessagesSentAfterPreKeyExchange` fails (alleged in prior `GAPS.md`) | The async package passes `go test -race -count=1 ./async/...` (36 s). The current code at `async/manager.go:861-933` correctly re-queues failed messages and is unblocked by `signalPreKeyReady`. |
| `TestRetrieveMessagesByPseudonym` proves pseudonym uniqueness regression (alleged in prior `GAPS.md`) | The pseudonym is **intentionally** keyed by `(recipient, epoch)` so all messages from a sender to a recipient within the same epoch share the obfuscation key; this is the documented design (`async/doc.go`, `async/obfs.go`). Returning multiple messages for one pseudonym is the expected behaviour, not a leak. |
| Six remaining `BUG:` annotations in production crypto / `toxav.go` / `toxcore_defaults.go` (alleged in prior `GAPS.md`) | `rg "BUG:" --type=go` against the current tree returns zero matches outside `toxcore_integration_test.go`. The annotations have been removed. |
| `capi/toxav_c.go:837` `convertPCMToSlice` cast to `[1<<20]int16` without checking C buffer | Callers validate `totalSamples` against the same `1<<20` bound (`capi/toxav_c.go:826-832`) before invoking; the C ABI contract owns the upstream buffer-size guarantee that any cgo-style implementation has. Marked LOW elsewhere as documentation. |
| `bridgeAudio/Video` callbacks "leak Go pointers to C" | cgo rules pin the slice memory for the duration of the C call; the only risk is post-return retention, which is a C-side API contract (see F-CAPI-001 above for the documentation gap). |
| `dht/group_storage.go:155-161` SerializeAnnouncement off-by-one | Inspection of the field layout (`groupID:4 + nameLen:4 + type:1 + privacy:1 = 10`) confirms the timestamp offset of 10 is correct. |
| `dht/mdns_discovery.go` `receiveLoop` blocks on `readChan` | `shouldStopReceiveLoop()` is consulted on every iteration and the channel is buffered; verified by tests passing under `-race`. |

## Remaining Scope
A complete checklist pass was executed for every non-`examples/` package; no
package was deferred. The `examples/` directory was spot-checked only,
because it ships as demonstration code (not part of the library API surface)
and its bugs would not affect library consumers. A future end-to-end run that
wants exhaustive coverage of `examples/` should focus on
`examples/toxav_effects_processing/` (largest demo, contains the project's
most complex example code).
