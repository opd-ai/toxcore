# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-06-03

## Project Profile

**Project:** `toxcore-go` (`github.com/opd-ai/toxcore`) — a pure-Go implementation of the
Tox peer-to-peer encrypted messaging protocol.

- **Purpose:** DHT-based peer discovery, friend management, 1-to-1 and group messaging,
  file transfers, ToxAV audio/video calling, asynchronous offline messaging with forward
  secrecy, and multi-network transport (UDP/TCP, Tor, I2P, Nym/Lokinet dial-only) — without
  cgo in the core library.
- **Target users:** Go developers embedding Tox messaging; C developers via the
  `capi/` libtoxcore-compatible bindings (cgo).
- **Deployment model:** Library/SDK linked into an application process. Trust boundaries are
  the network surfaces: DHT/transport packet parsing, async storage-node responses, RTP
  media, and friend/group message payloads — all attacker-influenceable.
- **Critical paths:** `crypto/` (key exchange, AEAD, replay protection), `async/`
  (forward secrecy, pre-keys, obfuscation), `ratchet/` (Double Ratchet), `transport/` +
  `noise/` (handshakes, packet parsing, NAT), `dht/` (routing/lookups), `av/*` (media).

## Audit Scope

Full coverage pass over every non-test package. ~45,000 non-test LOC across 257 files.

go-stats-generator baseline (`--skip-tests`):

| Metric | Value |
|--------|-------|
| Total functions | 1340 |
| Total methods | 3107 |
| Total structs | 429 |
| Total interfaces | 40 |
| Total packages | 27 |
| Total files | 257 |
| Non-test LOC | 45030 |
| Avg cyclomatic complexity | 3.5 |
| Functions > complexity 10 | 4 |
| Functions > 50 lines | 27 (0.6%) |
| Functions > 100 lines | 0 |
| Doc coverage | 93.6% |
| Duplication ratio | 0.56% (largest clone 17 lines) |
| Circular dependencies | 0 |

Tooling results:

- `go vet ./...` — **0 warnings**.
- `go test -tags nonet -race ./...` — **34/34 test packages PASS**, no failures, no data
  races reported. (25 packages are example/demo `main`s with no test files.)

**Structural-risk manual inspection (3a):** All 4 functions above cyclomatic complexity 10
(`cloneReflectValue`, `ImportPreKeys`, `checkForRiskyConfigurations`,
`getConfigurationWarnings`) and the top length/complexity functions
(`runPeer`, `handlePreKeyExchangePacket`, `GetSecurityPosture`, `deserializeVideoRTPPacket`)
were read in full. No new defects were confirmed in them beyond what is listed below; the
two highest-complexity functions carry explicit acknowledgement comments (see False
Positives).

**Important context:** This repository has been through one or more prior audits. Many code
sites carry inline finding labels (`C-01`, `H-01`…`H-07`, `M-03`…`M-21`, `L-01`…`L-13`).
This pass independently **re-verified the previously documented HIGH/MEDIUM gaps and found
the substantive ones already remediated** (see "Previously Reported — Re-verified as Fixed").
The findings below are what remains confirmed in the **current** tree.

## Coverage Log

Every package below was inspected against all 3b–3j checklist categories (3k performance and
3l false-positive filtering applied throughout). ✅ = category inspected; no finding above LOW
unless noted in Findings.

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ratchet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport/internal/addressing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bootstrap | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ | ✅ | ✅ |
| av/audio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/rtp | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxnet | ✅ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ |
| capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| factory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| limits | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| real | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

⚠️ marks a package with at least one recorded finding in that category (see Findings).

## Goal-Achievement Summary

| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| DHT routing & peer discovery | ✅ | none |
| Friend management | ✅ | none |
| 1-to-1 encrypted messaging | ✅ | none |
| Group chat | ✅ | none |
| File transfers (path-traversal-safe) | ✅ | none |
| ToxAV audio/video (bidirectional) | ✅ | prior H-02 fixed; see L-1, L-2 (cleanup/aliasing smells) |
| Async offline messaging w/ forward secrecy | ✅ | prior H-04/M-03 fixed (fail-closed) |
| Double Ratchet (decrypt-then-commit) | ✅ | prior H-01 fixed (rollback implemented) |
| Multi-network transport (Tor/I2P/Nym/Loki) | ⚠️ | Nym/Loki dial-only + external dep (GAPS) |
| Noise-IK handshakes | ✅ | none |
| NAT traversal (UPnP SSRF-safe) | ✅ | prior M-06 fixed (LOCATION URL validated) |
| Cryptography (Curve25519/ChaCha20/Ed25519) | ✅ | none |
| C API bindings (libtoxcore-compatible) | ✅ | C-ABI buffer contract is by design (see FP table) |
| Go `net.*` interfaces (`toxnet`) | ⚠️ | **H-1**: `RequireEncryption()` strict mode is non-functional |
| VP8 I-frame + P-frame video | ⚠️ | P-frame decode unimplemented (GAPS) |

## Findings

### CRITICAL
- [ ] _None confirmed in the current tree._ (The previously labelled `C-01`/`C-2` sites were
  re-inspected and are guarded; see "Previously Reported — Re-verified as Fixed".)

### HIGH
- [x] **H-1 — `RequireEncryption()` strict mode does not drop undecryptable packets; plaintext is delivered to the application** — `toxnet/packet_conn.go:182-189` (caller `toxnet/packet_conn.go:146-147`) — security / swallowed error (3d, 3g, 3j).
  **Data flow:** `RequireEncryption()` documents (`packet_conn.go:532-535`) that "packets
  from unknown peers and packets that fail decryption are dropped instead of passed through
  as plaintext." `decryptPacket()` honours this — in strict mode it returns a `*ToxNetError`
  for an unknown peer (`:605-610`) or an AEAD failure (`:621-626`). **But the only caller,
  `createPacketWithAddr()`, ignores that error:** `decrypted, err := c.decryptPacket(...);
  if err == nil { finalData = decrypted }` and then *falls through* with
  `finalData = data` (the original bytes) on error (`:184-189`). `processIncomingPacket()`
  then unconditionally `enqueuePacket(packet, n)` (`:146-147`), so `ReadFrom()` hands the
  raw, unauthenticated packet to the application. The documented `M-05` hardening is
  therefore inoperative: an off-path attacker who injects a forged or plaintext datagram to a
  connection the application believes is in strict-encryption mode gets that data accepted as
  if authenticated. **Concrete consequence:** silent authentication bypass for any caller
  relying on `RequireEncryption()`. **Evidence of reachability:** `decryptPacket` is the live
  decode path; `RequireEncryption()` has **no test coverage** (`grep RequireEncryption
  toxnet/*_test.go` returns nothing), so the regression is undetected by the passing suite.
  **Remediation:** In `createPacketWithAddr` (`toxnet/packet_conn.go:177-197`), propagate the
  strict-mode failure: read `c.encryptionRequired` under the existing lock and, when it is
  set and `decryptPacket` returns an error, signal "drop" (e.g. return a sentinel / `ok bool`)
  so `processIncomingPacket` skips `enqueuePacket`. Keep the current pass-through only for the
  default mixed mode (`encryptionRequired == false`). Add a test that enables
  `EnableEncryption`+`RequireEncryption`, feeds a non-decryptable datagram, and asserts
  `ReadFrom` does **not** return it. Validate with `go test -race ./toxnet/...` and
  `go vet ./toxnet/...`.**✅ FIXED:** `createPacketWithAddr` now returns `(packetWithAddr, bool)` where the bool indicates whether to enqueue. When `encryptionRequired` is true and decryption fails, returns `(_, false)` to drop. `processIncomingPacket` skips `enqueuePacket` on false. Added `TestRequireEncryptionDropsUndecryptablePackets` and `TestRequireEncryptionMixedMode` tests. All toxnet tests pass under `-race`.

### MEDIUM
- [x] **M-1 — Metrics report callback launched in an untracked goroutine on a periodic timer; unbounded under a slow/blocking callback and not awaited on `Stop()`** — `av/metrics.go:395` — concurrency / goroutine retention (3f).
  **Data flow:** `MetricsAggregator.Start()` → `reportLoop()` (`av/metrics.go:352-363`) fires
  every `reportInterval` → `generateReport()` → `go callback(report)` (`:395`). The goroutine
  is not tracked by any `sync.WaitGroup`, and `Stop()` (`:178-195`) only calls `ma.cancel()`
  — it does not wait for in-flight callback goroutines. If a user-supplied `reportCallback`
  blocks (e.g. writes to a full channel or does slow I/O), a new goroutine is spawned every
  interval without bound; on shutdown, an in-flight report may run after `Stop()` returns.
  This is inconsistent with the sibling `av/adaptation.go`, which tracks its callback with a
  `callbackWg`. **Consequence:** goroutine accumulation / late callback execution on the
  media metrics path when the embedding application's callback misbehaves. Bounded in the
  common case (well-behaved, fast callbacks), hence MEDIUM. **Remediation:** Track the
  callback goroutine with a `sync.WaitGroup` (mirroring `adaptation.go`) and `Wait()` for it
  in `Stop()`, or invoke the callback synchronously within `reportLoop`. Validate with
  `go test -race ./av/...`.**✅ FIXED:** Added `callbackWg sync.WaitGroup` field to `MetricsAggregator` struct. `generateReport()` now adds to waitgroup before spawning callback goroutine. `Stop()` now unlocks before waiting and waits for all in-flight callbacks to complete. All av tests pass under `-race`.

### LOW
- [x] **L-1 — `CleanupMedia()` discards the RTP session without calling its documented `Close()`** — `av/types.go:1210-1217` — resource lifecycle / API contract (3e, 3j).
  `CleanupMedia` sets `c.rtpSession = nil` under a "RTP session cleanup (if needed)" comment
  but never calls `c.rtpSession.Close()`, whereas `av/rtp/doc.go:63` documents the
  `defer session.Close()` pattern and the video/bitrate paths just above
  (`:1200`, `:1225`) do call `Close()`. **Impact is low:** `Session.Close()`
  (`av/rtp/session.go:559-572`) only nils internal packetizer/depacketizer pointers — the
  session owns no goroutines, channels, or sockets — so the values are garbage-collected
  regardless. This is a consistency/contract smell, not a live leak.
  **Remediation:** Call `if err := c.rtpSession.Close(); err != nil { log }` before
  `c.rtpSession = nil`, matching the surrounding cleanup blocks. Validate with
  `go test -race ./av/...`.**✅ FIXED:** `CleanupMedia()` now calls `c.rtpSession.Close()` before setting to nil, with proper error logging matching the audioProcessor/videoProcessor cleanup pattern. All av tests pass under `-race`.
- [x] **L-2 — `Session.ReceivePacket` returns audio payload that aliases the input packet buffer** — `av/rtp/transport.go:402` / `av/rtp/session.go:411-432` — data aliasing (3h).
  `AudioDepacketizer.ProcessPacket` returns `packet.Payload` (`av/rtp/transport.go:402`),
  which `pion/rtp`'s `Unmarshal` makes alias the input slice (the file itself documents this
  at `av/rtp/packet.go:634-639`, where the **jitter-buffer** copy path is correctly fixed).
  The directly-returned slice is not copied, so if the caller's upstream `transport.Packet.Data`
  buffer is later reused, the returned audio could be corrupted. In the current call chain the
  decoded frame is consumed immediately and `transport.Packet.Data` does not appear to be a
  reused scratch buffer, so this is theoretical — labelled LOW with that uncertainty noted.
  **Remediation:** Return a copy (`out := append([]byte(nil), packet.Payload...)`) from
  `ProcessPacket`, mirroring the jitter-buffer fix at `packet.go:637-638`. Validate with
  `go test -race ./av/rtp/...`.**✅ FIXED:** `AudioDepacketizer.ProcessPacket` now returns a copy of `packet.Payload` using `append([]byte(nil), packet.Payload...)` pattern, matching the jitter buffer copy approach. All av/rtp tests pass under `-race`.
- [x] **L-3 — Off-by-one in the `ExtendedParser` minimum-length error message** — `transport/parser.go:193-195` — logic / documentation (3b).
  The guard reads `if len(data) < offset+35` with the comment "minimum: 32 + 1 + 1 + 0 + 2",
  but that minimum is **36**, and the error text says "need at least 35 bytes". **Not
  exploitable:** the actual port read at `:234` (`data[currentOffset]`,
  `data[currentOffset+1]`, `currentOffset == offset+34`) is independently guarded by the
  second check `if len(data) < currentOffset+addrLen+2` at `:223`, which for `addrLen == 0`
  requires `len(data) >= offset+36`. So no out-of-range access occurs; only the early-exit
  error message is inaccurate. **Remediation:** Change the constant to `offset+36` and the
  message to "36 bytes" for clarity. Validate with `go test ./transport/...`.**✅ FIXED:** Changed check from `offset+35` to `offset+36` and updated error message to "need at least 36 bytes". Improved comment clarity. All transport tests pass.

## Previously Reported — Re-verified as Fixed

These were independently re-checked against the current tree and confirmed **remediated**;
they are recorded here so the next session does not re-flag them.

- [x] **H-01 (ratchet decrypt-then-commit):** `RatchetDecrypt` now snapshots receive state
  (`ratchet/session.go:160-171`) and rolls back on AEAD failure (`:195-208`); a forged
  ciphertext no longer desynchronizes the session. Tests pass under `-race`.
- [x] **H-02 (ToxAV answerer transmits no media):** `AnswerCall` now unwraps the transport via
  the `underlyingTransportProvider` shim before `SetupMedia`
  (`av/manager.go:1248-1260`), mirroring `StartCall`.
- [x] **H-04 / M-03 (forward-secrecy refresh accounting & silent non-FS fallback):**
  `NeedsRefresh` and `GetRemainingKeyCount` now count only `!Used` keys
  (`async/prekeys.go:315-321`, `:394-402`); `SendAsyncMessage` is fail-closed when a
  `ForwardSecurityManager` is configured but pre-keys are unavailable
  (`async/client.go:305-307`, `:354-372`).
- [x] **M-06 (UPnP SSDP SSRF):** SSDP `LOCATION` URLs are validated for scheme and
  private/LAN host before fetch (`transport/upnp_client.go:126-175`,
  `validateUPnPLocationURL`).

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total functions | 1340 |
| Total methods | 3107 |
| Functions above complexity 15 | 2 (`cloneReflectValue` 22.3, `ImportPreKeys` 21.5) |
| Functions above complexity 10 | 4 |
| Avg cyclomatic complexity | 3.5 |
| Doc coverage | 93.6% |
| Duplication ratio | 0.56% |
| Test pass rate | 34/34 packages (`-race`, `-tags nonet`) |
| go vet warnings | 0 |

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|----------------|
| `transport/parser.go:193` panic via slice OOB on a 35-byte packet | The port read at `:234` is independently guarded by the second bounds check at `:223` (`len(data) < currentOffset+addrLen+2`). No OOB occurs; only the early-exit error message is off by one (recorded as L-3, not a panic). |
| `capi/toxcore_c.go:311` `copyStringToCBuffer` writes `len(src)` bytes to a caller buffer with no size param | This is the standard libtoxcore C-ABI contract: the C caller pre-allocates using the matching `*_size()` query (e.g. `tox_friend_get_name_size`). Matches upstream libtoxcore semantics; the Go side cannot know the C allocation size. By design, not a Go-side bug. The `dst == nil` guard at `:309` is present. |
| `capi/toxcore_c.go:294` `setStringFromByteBuffer` reads `dataLen` bytes via `unsafe.Slice` | Same C-ABI contract: `dataLen` is the caller's asserted length; validating it is impossible from Go and is the C caller's responsibility. A `data == nil && dataLen > 0` guard already exists at `:289-291`. By design. |
| `cloneReflectValue` shallow-shares unexported struct pointer fields (`toxcore_friends.go:345-357`) | Explicitly acknowledged in-code as L-4 with a documented rationale (`:277-287`): no reachable public setter populates `Friend.UserData` with unexported pointer fields, so impact is theoretical. Acknowledged pattern → not reported. |
| `mdns_discovery.go:48-51` / parser `init()` panics | Intentional compile-time-invariant panics on pre-resolved constants (not reachable from untrusted input). Acknowledged design. |
| Async/crypto "missing" bounds or nonce-reuse issues | `crypto/` uses `crypto/rand` throughout, wipes secrets via `ZeroBytes`/`SecureWipe`, and bounds-checks before indexing; `async` packet handlers validate lengths before slicing. No reachable defect found. |

## Remaining Scope (if session ended before completion)

| Package | Status | Notes |
|---------|--------|-------|
| (all) | Audited | A full coverage pass completed for every non-test package. The `examples/*`, `testnet/`, and `cmd/` demo `main` packages are non-shipping and were scanned only for obvious defects (none above LOW). A future pass could deep-dive `transport/` (largest package, 856 functions) and `av/video/` VP8 internals for performance hot paths. |
