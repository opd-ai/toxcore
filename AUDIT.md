# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-06-01

## Project Profile

**toxcore-go** (`github.com/opd-ai/toxcore`) is a pure-Go implementation of the Tox
peer-to-peer encrypted messaging protocol. Stated purpose (README): DHT-based peer
discovery, friend management, 1-to-1 and group messaging, file transfers, audio/video
calling (ToxAV), asynchronous offline messaging with forward secrecy, and multi-network
transport (UDP/TCP/Tor/I2P/Lokinet/Nym) — "all without cgo dependencies in the core
library".

- **Target users**: Go developers embedding a Tox client/library; C consumers via the
  optional cgo `capi/` bindings (libtoxcore-compatible).
- **Deployment model**: Library linked into an application process. Runs as a long-lived
  P2P node that parses untrusted packets from the network (DHT, Tox protocol, Noise
  handshakes, RTP media). The primary trust boundary is **inbound network packet parsing**.
- **Critical paths** (implement primary stated goals): `crypto/` (key exchange, AEAD,
  signatures, replay protection), `noise/` + `ratchet/` (forward secrecy), `dht/`
  (peer discovery), `transport/` (packet parsing, NAT, multi-network addressing),
  `async/` (offline messaging + forward secrecy + obfuscation), `av/` + `av/rtp/`
  (media), `file/`, `friend/`, `group/`, `messaging/`, and the root `toxcore` package.

## Audit Scope

Full pass over every non-test Go file in every first-party package (examples and
`testnet/` orchestration treated as non-critical demo code and scanned but not deeply
audited). Auditing combined manual inspection of high-risk functions (cyclomatic
complexity, untrusted-parser sites) with category-by-category scanning across all
packages, followed by Phase 3 false-positive verification (data-flow tracing, upstream
guard checks, comment review, `-race` test corroboration) on every candidate finding.

go-stats-generator metrics summary (`--skip-tests`, 251 files):

- 1318 functions, 3050 methods, 421 structs, 40 interfaces, 27 packages, 44,070 LOC.
- Average function length 12.2 lines; longest 75 lines; 23 functions > 50 lines; none > 100.
- Average cyclomatic complexity 3.5; only 2 functions with complexity > 10
  (`cloneReflectValue` 16, `async.ImportPreKeys` 15) — both inspected manually and found sound.
- Documentation coverage 93.5% overall (packages 100%, functions 98.8%).
- Duplication ratio 0.55% (largest clone 17 lines); no circular dependencies.

## Audit Scope — Tooling Baseline

| Command | Result |
|---------|--------|
| `go vet ./...` | 0 warnings |
| `go test -race ./...` | 35/35 packages `ok`, 0 DATA RACE, 0 failures |
| `go-stats-generator analyze . --skip-tests` | metrics above |

The clean `-race` run is treated as evidence (not proof) against the speculative race
conditions evaluated below, and is cited where relevant.

## Coverage Log

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ratchet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport (+internal/addressing) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av, av/audio, av/rtp, av/video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxnet (+subdirs) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bootstrap (+nodes) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| factory, limits, interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary

| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| DHT-based peer discovery | ⚠️ | H-01 / G-01 (gossip peer-exchange parser broken & IP-only; main DHT path works) |
| Friend management | ✅ | — |
| 1-to-1 messaging w/ padding & retry | ✅ | — |
| Group chat | ✅ | — |
| File transfers (pause/resume/cancel) | ✅ | L-04, L-05 (defensive nil handling only) |
| ToxAV audio/video calling | ⚠️ | M-01 (VP8 RTP descriptor not RFC 7741-compliant — interop) |
| Async offline messaging w/ forward secrecy & obfuscation | ✅ | — |
| Multi-network transport (Tor/I2P/Lokinet/Nym) | ✅ | Main DHT path uses multi-network `PacketParser` (`dht/parser_integration.go`); gossip sub-path is IP-only — see G-01 |
| Noise-IK handshakes (forward secrecy, KCI resistance) | ✅ | — |
| Cryptography (Curve25519/ChaCha20-Poly1305/Ed25519, secure wiping) | ✅ | L-03 (defensive nil-check inconsistency only) |
| C API bindings | ✅ | — |
| Go `net.*` interfaces | ✅ | L-06 (Close does not join background goroutines) |
| No-cgo core library | ✅ | — |

## Findings

### CRITICAL

*(none confirmed)*

### HIGH

- [ ] **H-01 — Gossip peer-exchange parser reads the SendNodes packet at the wrong
  offsets, silently disabling gossip peer discovery** — `dht/gossip_bootstrap.go:245-254`
  — Logic / untrusted-packet parsing —
  The on-wire SendNodes format is `[senderPK(32)][numNodes(1)][nodeEntries...]`, as
  produced/validated by the DHT path: `dht/handler.go:265-269` requires `len(Data) >= 33`,
  `handler.go:273` reads the sender public key from `Data[:32]`, and `handler.go:63` reads
  the node count from `Data[32]`. `BootstrapManager.handleSendNodesPacket` then *also*
  forwards the same packet to the gossip handler (`dht/handler.go:71`). But
  `GossipBootstrap.handleSendNodes` validates only `len(Data) < 1` (line 245), reads the
  node count from `Data[0]` (line 249) — which is actually the first byte of the sender's
  public key — and begins parsing node entries at `offset = 1` (line 254), i.e. inside the
  public-key bytes. **Concrete consequence:** the "count" is a uniformly random key byte;
  when it exceeds `MaxPeersPerExchange` the function returns with zero peers, and otherwise
  `parseNodeEntry` reads `Data[1]` as an IP-type byte (`gossip_bootstrap.go:278`), which is
  almost never a valid type (2 or 10), so parsing breaks on the first entry. The result is
  that the gossip peer-exchange discovery path adds essentially no peers — a documented
  feature that is non-functional. The error is swallowed at `handler.go:72-78` ("gossip is
  supplemental"), so the defect is silent. No panic occurs (loop guard `offset+39 <= len`
  and per-field bounds checks in `parseNodeEntry`/`parseIPFromType`/`parsePort`/`parsePublicKey`
  prevent out-of-bounds). **Remediation:** In `handleSendNodes`, require `len(packet.Data) >= 33`,
  read `nodeCount := int(packet.Data[32])`, and set `offset := 33` so the parser matches the
  format already implemented in `dht/handler.go:265-273` and the builder. Add a unit test that
  feeds a real builder-produced SendNodes packet (see `dht/handler.go` `sendNodesResponse`)
  through `handleSendNodes` and asserts the expected peers are added. Validate with
  `go test -race ./dht/...`.

### MEDIUM

- [ ] **M-01 — VP8 RTP payload descriptor is not RFC 7741-compliant (media interop with
  third-party VP8/WebRTC stacks)** — `av/video/rtp.go:176-202` (`buildVP8Payload`), with the
  matching non-standard reader at the depacketizer and `av/rtp/session.go`
  `deserializeVideoRTPPacket` — Protocol/API contract —
  `buildVP8Payload` allocates a fixed 3-byte descriptor and writes the 15-bit PictureID
  directly into `payload[1..2]` with the `I` bit, *omitting the RFC 7741 extension byte*
  (`X|I|L|T|K|RSV`). RFC 7741 §4.2 requires, when the X bit is set in the first octet, an
  extension octet to precede the PictureID, and the PictureID's `M` bit selects 7-bit vs
  15-bit form. This implementation therefore emits a descriptor layout that standards-
  compliant decoders cannot parse. **Concrete consequence:** toxcore-to-toxcore video works
  (the serializer and deserializer agree), but interop with any RFC 7741 VP8 receiver
  (e.g. a WebRTC gateway or libtoxcore peer) fails to decode the picture-ID/keyframe
  signaling. This is an interoperability gap, not a crash. *(Severity capped at MEDIUM:
  the project ships its own packetizer/depacketizer pair and does not explicitly claim
  RFC-7741 wire interop; impact is limited to cross-stack interop — see GAPS.md G-02. Some
  uncertainty remains about whether external interop is an intended guarantee.)*
  **Remediation:** Rework `buildVP8Payload` (and its reader) to emit the RFC 7741 descriptor:
  first octet with `X` bit, an extension octet with the `I` bit, then the 1- or 2-byte
  PictureID with the `M` bit; size the buffer accordingly instead of the hard-coded `3`.
  Add a round-trip test plus a fixed-vector test against an RFC 7741 sample. Validate with
  `go test -race ./av/...`.

### LOW

- [ ] **L-01 — `GainEffect.SetGain` logs a stale `old_gain` value (TOCTOU on a log field)**
  — `av/audio/effects.go:172-205` — Concurrency (logging correctness only) —
  `oldGain` is captured under `RLock` at lines 172-174 and released; the new value is only
  written under `Lock` at lines 199-201. A concurrent `SetGain` between the two critical
  sections makes the logged `old_gain` not reflect the actual prior value. The gain field
  itself is correctly mutex-protected, so there is no data race on the value (consistent
  with the clean `-race` run); only the audit-log line can be misleading. **Remediation:**
  Capture `oldGain` inside the same write-locked section that performs the update, or accept
  the imprecision and document it. Validate with `go test -race ./av/audio/...`.

- [ ] **L-02 — Single-value type assertion on `sync.Pool.Get()`** —
  `av/performance.go:229` (`getCallSlice`) — Nil/boundary (defensive smell) —
  `slice := po.callSlicePool.Get().([]*Call)` uses the panicking single-value form. It is
  currently safe because `callSlicePool.New` is always set (`av/performance.go:69`) and only
  `[]*Call` is ever `Put` back (`returnCallSlice`), so `Get` cannot return another type.
  Flagged LOW because a future refactor that shares the pool or stores a different type would
  turn this into a panic with no graceful path. **Remediation:** Use the comma-ok form and
  fall back to `make([]*Call, 0, 8)` on `!ok`. Validate with `go test -race ./av/...`.

- [ ] **L-03 — Missing nil guard inconsistent with sibling methods in
  `FindKeyForPublicKey`** — `crypto/key_rotation.go:138-143` — Nil/boundary (defensive) —
  The loop dereferences `key.Public` without a `key != nil` check, whereas the sibling
  iterators `GetAllActiveKeys` (`key_rotation.go:114-115`) and `GetPreviousKeys`
  (`key_rotation.go:260-261`) defensively skip nil entries. In practice `previousKeys` only
  ever receives the non-nil `currentKeyPair` (`key_rotation.go:75`), and `Cleanup` nils
  entries only under the exclusive write lock before discarding the slice
  (`key_rotation.go:192-198`) — mutually exclusive with this `RLock`ed reader — so a nil is
  not reachable today. Flagged LOW for consistency/defense-in-depth. **Remediation:** Add
  `if key == nil { continue }` to match the sibling methods. Validate with
  `go test -race ./crypto/...`.

- [ ] **L-04 — Misplaced nil check after dereference in `writeDataToFile`** —
  `file/transfer.go:542-544` — Nil/boundary (dead defensive code) —
  Line 542 calls `t.FileHandle.Write(data)`; the `if t.FileHandle != nil` guard appears
  *after*, at line 544, inside the error branch. If `FileHandle` were nil the program would
  already have panicked at 542, so the guard is effectively dead code. The actual panic is
  not reachable on the normal path: callers reach `writeDataToFile` only via `WriteChunk`,
  which first calls `validateWriteRequest` requiring `State == TransferStateRunning`
  (`file/transfer.go:507`, `533-535`), and the handle is opened before that state is set.
  **Remediation:** Move the nil check before the `Write` (returning a clear error), or remove
  the now-redundant check. Validate with `go test -race ./file/...`.

- [ ] **L-05 — `readFileChunk` dereferences `FileHandle` without a nil guard** —
  `file/transfer.go:627-629` — Nil/boundary (defensive) —
  Same shape as L-04 on the read path. Reachable only via `ReadChunk`, which enforces
  `State == TransferStateRunning` through `validateReadRequest`
  (`file/transfer.go:604`, `615-621`), so `FileHandle` is non-nil in normal flow.
  **Remediation:** Add an explicit nil check returning a descriptive error to harden against
  misuse. Validate with `go test -race ./file/...`.

- [ ] **L-06 — `ToxPacketConnection.Close` / `ToxPacketConn.Close` cancel context but do not
  join background goroutines** — `toxnet/packet_listener.go:303-359` and
  `toxnet/packet_conn.go:88-104` — Resource lifecycle —
  `Close()` calls `cancel()` (`packet_listener.go:309`, `packet_conn.go` Close), and the
  background `processPackets`/`processWrites` goroutines exit on `<-ctx.Done()`
  (`packet_listener.go:123`, `367`). They self-terminate (within the per-iteration read
  timeout), so this is **not** a true goroutine leak — but `Close` returns before they have
  observably exited, which can briefly hold buffers/FDs after `Close` and complicates
  deterministic shutdown in tests. **Remediation:** Track the goroutines with a
  `sync.WaitGroup` and `wg.Wait()` in `Close` (after `cancel`) so shutdown is synchronous.
  Validate with `go test -race ./toxnet/...`.

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total functions (free functions) | 1318 |
| Total methods | 3050 |
| Functions above complexity 15 | 2 (`cloneReflectValue` 16, `async.ImportPreKeys` 15) |
| Functions above complexity 10 | 2 |
| Functions > 50 lines | 23 |
| Avg cyclomatic complexity | 3.5 |
| Doc coverage (overall) | 93.5% |
| Duplication ratio | 0.55% |
| Test pass rate | 35/35 packages `ok` (0 failures, 0 data races) |
| go vet warnings | 0 |

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|----------------|
| `async/prekeys.go` `clonePreKey` (≈579-586) "use-after-free: returns pointer to stack local `copyKeyPair`" | Go is not C. `&copyKeyPair` escapes to the heap via escape analysis; the returned pointer is valid. Idiomatic and safe. |
| `async/prekeys.go` `findUnusedPreKey` (≈597-598) "returns pointer to stack local `snapshot`" | Same: returning `&snapshot` is safe in Go; the value is heap-allocated because it escapes. |
| `friend/request.go:348-349` `GetPendingRequests` "returns dangling pointers to loop-local `cp`" | Same Go escape-analysis reasoning; each `&cp` is a distinct heap allocation. Safe. |
| `toxnet/packet_listener.go` Read "race: SetReadDeadline closes the channel a blocked Read selects on" | This is the **intended** wakeup mechanism. `notifyReadDeadlineChanged` deliberately closes `readDeadlineChanged` so blocked `performRead` returns `retry=true` (`packet_listener.go:463-465`) and the `Read` loop re-evaluates the new deadline (`Read` loop 405-419). All accesses to the channel are guarded by `deadlineMu`; closing a channel concurrently with a receiver is safe. The `-race` suite passes. |
| `toxnet/packet_listener.go:397-410` "init race on `readDeadlineChanged`" | The lazy init and every read/replace of the channel pointer are performed under `deadlineMu`; the captured local `changed` is a copy of the pointer and receiving on a soon-to-be-closed channel is safe. No data race (corroborated by `-race`). |
| `toxnet/conn.go:115-127` `setupReadTimeout` "stale timer when deadline changes mid-read" | Mid-read deadline changes are handled by the `deadlineChanged` wakeup path, which causes the read loop to rebuild the timer with the new deadline; the old timer is stopped/drained on the next iteration. By design. |
| `async/prekeys.go:788` `filtered := cp.Keys[:0]` in-place filter "aliases caller slice" | `cp` is a fresh deep copy from `clonePreKeyBundle` (`prekeys.go:787`); the in-place filter mutates only the copy, never the caller's backing array. Safe (and explicitly commented M-17/M-ASYNC-2). |
| `toxcore_friends.go:284` `cloneReflectValue` "shallow-shares unexported struct fields" | Already acknowledged in-code (L-4) with a documented argument that no reachable public setter produces UserData with unexported pointer fields; theoretical, not reachable. |
| `crypto/secure_memory.go:48`, `transport/nat.go:27`, `dht/mdns_discovery.go:48/51` `panic(...)` | These panic only on impossible failures of resolving hard-coded constant addresses / a wipe of a fixed-size buffer during initialization — truly unrecoverable init paths, which the checklist explicitly permits. |

## Remaining Scope (if session ended before completion)

A complete pass was performed across all first-party non-test packages; no package remains
unaudited. `examples/**` and `testnet/**` are demonstration/orchestration code outside the
library's trust boundary and were scanned (no findings above LOW) but not exhaustively
line-audited. A subsequent pass could deep-audit those if they become shipped surface.

| Package | Status | Notes |
|---------|--------|-------|
| examples/** | Scanned, not deep-audited | Demo code; not part of the library API/trust boundary |
| testnet/** | Scanned, not deep-audited | Local test orchestration tooling |
