# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-06-02

## Project Profile

**Project:** `github.com/opd-ai/toxcore` (toxcore-go) — a pure-Go implementation of the
Tox peer-to-peer encrypted messaging protocol.

- **Purpose:** DHT-based peer discovery, friend management, 1-to-1 and group messaging,
  file transfers, ToxAV audio/video calling, asynchronous offline messaging with forward
  secrecy, and multi-network transport (UDP/TCP/Tor/I2P/Lokinet/Nym).
- **Target users:** Go application developers embedding a Tox client, plus C/C++ consumers
  via the libtoxcore-compatible `capi/` bindings.
- **Deployment model:** Embedded library / daemon. Core library is intended to build without
  cgo (`CGO_ENABLED=0` confirmed working); cgo is opt-in for hardened memory (`crypto/`),
  libvpx encoding (`av/video/`), and the C API (`capi/`).
- **Go version:** 1.25.0 (toolchain go1.25.8).
- **Critical paths (primary stated goals):**
  - `crypto/` — Curve25519/ChaCha20-Poly1305/Ed25519, secure memory wiping.
  - `async/` — store-and-forward offline messaging, one-time pre-key forward secrecy,
    epoch-based identity obfuscation.
  - `dht/` — Kademlia DHT, parses untrusted `send_nodes` packets (trust boundary).
  - `transport/` — multi-network transport + Noise-IK handshakes, parses untrusted wire data.
  - `noise/` — Noise Protocol Framework handshakes (IK/XX).
  - `file/` — chunked file transfer (peer-supplied filenames = trust boundary).
  - `messaging/`, `group/`, `ratchet/`, `av/` — message delivery and media.

**Trust boundaries:** Untrusted input enters via (1) DHT/transport UDP packet parsers,
(2) Noise handshake packets, (3) async storage-node responses, (4) peer-supplied file
metadata, and (5) RTP/media packets. The audit traced each parser for length validation
before indexing.

## Audit Scope

All 27 non-vendored packages were audited (example programs under `examples/` were treated
as documentation/illustration, not core library, but `examples/noise_demo` is included
because its test reproducibly fails). Coverage was performed by direct inspection of the
high-risk functions plus four parallel package-cluster sweeps:

- Cluster A: `crypto/`, `async/`
- Cluster B: `dht/`, `transport/` (+ `transport/internal/addressing`)
- Cluster C: `av/`, `av/audio/`, `av/rtp/`, `av/video/`, `file/`, `group/`, `messaging/`, `friend/`, `ratchet/`
- Cluster D: root package (`toxcore*.go`), `toxnet/`, `capi/`, `bootstrap/`, `noise/`, `factory/`, `limits/`

**go-stats-generator metrics summary:** 251 files, 44,194 LOC, 1,319 functions, 3,052 methods,
421 structs, 40 interfaces. Average cyclomatic complexity 3.5; only 2 functions exceed
cyclomatic complexity 10 (both manually inspected). 23 functions exceed 50 lines (none exceed
100). Documentation coverage 93.5%. Duplication ratio 0.53%. `go vet ./...` produced 0 warnings.

The codebase shows evidence of prior structured audits (inline references such as
`M-08`, `M-17`, `M-ASYNC-2`, `L-06`, `L-4`), and most classic bug classes are already guarded.

## Coverage Log

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/audio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/rtp | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bootstrap | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| factory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| limits | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ratchet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| real | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport/internal/addressing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary

| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| DHT routing / peer discovery | ✅ | — (L-02 is bounded log/CPU only) |
| Friend management | ✅ | — |
| 1-to-1 messaging | ✅ | — |
| Group chat | ✅ | — |
| File transfers | ✅ | — (path traversal already mitigated, M-08) |
| ToxAV audio/video | ✅ | — |
| Async offline messaging + forward secrecy | ✅ | — (L-01 is an inner-ID code smell, not a privacy break) |
| Multi-network transport | ⚠️ | Nym/Lokinet are dial-only stubs (see GAPS.md) |
| Noise-IK handshakes | ⚠️ | M-01: bidirectional demo test reproducibly fails |
| NAT traversal | ✅ | — |
| Cryptography / secure memory | ✅ | — |
| C API bindings | ✅ | — (L-03 mirrors libtoxcore caller contract) |
| Go net.* interfaces | ⚠️ | M-02: conn/listener contexts not tied to Tox lifecycle |
| Build without cgo in core | ✅ | — (CGO_ENABLED=0 build confirmed) |

## Findings

No CRITICAL or HIGH findings were confirmed. The candidate flagged as "CRITICAL" during the
sweep (deterministic async message ID) was downgraded to LOW after tracing the data flow —
see the False Positives table. All findings below were verified by reading the exact code path.

### CRITICAL
- [ ] _None confirmed._

### HIGH
- [ ] _None confirmed._

### MEDIUM
- [ ] **M-01 — Bidirectional Noise message exchange demo reproducibly fails** — `examples/noise_demo/main_test.go:159-164` (exercising `transport/noise_transport.go:377-415`) — Logic / API behavioral contract — `TestNoiseMessageExchange` fails on every run (verified 3×, not flaky): after node1→node2 establishes a session, the reverse `noise2.Send(addr1, "Reply from Node 2")` returns no error but the payload is never delivered within the 2s timeout. Root cause is the directional Noise-IK lifecycle: the responder side's first application send races handshake completion on the peer, and `NoiseTransport.Send` deliberately drops (does not queue) the payload while the session is incomplete (`return ErrNoiseSessionIncomplete`, line 405). The README advertises "Bidirectional communication" as a demonstrated feature, so a user copying this example will observe dropped first messages in the reverse direction. The library itself behaves safely (it returns an error rather than sending cleartext); the defect is the missing retry/queue at the example layer and the unstated "first reverse message is dropped" contract. **Remediation:** In `sendAndVerifyMessageWithTimeout` (`examples/noise_demo/main.go:142`), retry the send on `transport.ErrNoiseSessionIncomplete` with a short backoff until the session completes, OR add an explicit handshake-trigger for the node2→node1 direction mirroring the node1→node2 trigger at `main_test.go:151`. Validate with `go test -tags nonet -count=5 ./examples/noise_demo/`. Separately, document in `NoiseTransport.Send` GoDoc that the first packet to a not-yet-established peer is dropped and the caller must retry on `ErrNoiseSessionIncomplete`.
- [ ] **M-02 — `toxnet` connection/listener contexts are not derived from the Tox lifecycle** — `toxnet/conn.go:61`, `toxnet/listener.go:59`, `toxnet/packet_conn.go:79` — Resource lifecycle — Each wrapper creates its cancellation context with `context.WithCancel(context.Background())` instead of deriving it from the owning `Tox` instance's context. The cancel funcs are only invoked from the respective `Close()` methods (`conn.go:465`, `listener.go:261`, `packet_conn.go:378`). Consequence: if an application calls `Tox.Kill()` (which cancels `tox.ctx` at `toxcore_lifecycle.go`) but does not also call `Close()` on every outstanding `ToxConn`/`ToxListener`/`PacketConn`, the goroutine spawned in `ToxListener` (`listener.go:108`) and any goroutine blocked in `Read`/`Accept` on `ctx.Done()` are not released, retaining memory/goroutines for the process lifetime. This is bounded by the standard `net.Conn`/`net.Listener` "caller must Close" contract, so it is MEDIUM rather than HIGH. **Remediation:** Thread the parent `Tox` context (or a derived `context.Context`) into `newToxConn`/`newToxListener`/`newPacketConn` and create the child via `context.WithCancel(parentCtx)` so that `Tox.Kill()` propagates cancellation. Keep the existing `Close()`-based cancel for the explicit path. Validate with `go test -race ./toxnet/...` and a leak check (e.g. `goleak`) around a Kill-without-Close sequence.

### LOW
- [ ] **L-01 — Fallback async message ID derived from message plaintext** — `async/client.go:382` — Security / data aliasing (code smell) — In `createFallbackForwardSecureMessage`, the inner `ForwardSecureMessage.MessageID` is set with `copy(messageID[:], message[:min(len(message),32)])`, i.e. the first 32 bytes of the (padded) plaintext, despite a proper `generateMessageID()` (crypto/rand) helper existing at `async/client.go:416`. Identical plaintexts therefore produce identical inner IDs. Impact is LOW because the inner `ForwardSecureMessage` is encrypted into `EncryptedPayload` and the *outer* `ObfuscatedAsyncMessage.MessageID` that storage nodes actually see is regenerated randomly in `ObfuscationManager.CreateObfuscatedMessage` (`async/obfs.go:336`, `generateRandomIdentifiers`). The deterministic inner ID is not used for delivery deduplication. The practical consequence is limited to a recipient-local `DecryptedMessage.ID` collision for byte-identical messages. **Remediation:** Replace line 382 with `messageID, err := generateMessageID()` and propagate the error, matching the FSM path. Validate with `go test -race ./async/...`.
- [ ] **L-02 — Unbounded offset advance / log amplification on malformed legacy `send_nodes` packets** — `dht/handler.go:378-379` — Logic / resource (minor DoS) — When a legacy-format node entry fails to parse, `handleNodeParsingError` advances `context.offset += 50` and returns `true`, so the loop in `processReceivedNodesWithVersionDetection` (`dht/handler.go:310`) continues up to `numNodes` (attacker-controlled, max 255) iterations. Each iteration calls `ParseNodeEntry`, which is itself bounds-checked (no out-of-bounds read occurs), but emits a `Warn` log line per failure. A single crafted packet (`numNodes=255` with truncated body) yields up to 255 failed parses and 255 warning logs. Impact is LOW: CPU cost is trivial and there is no memory-safety violation, but log-volume amplification is real. **Remediation:** Before advancing, bound-check `if context.offset+50 > len(context.packet.Data) { return false }` and/or demote the per-entry message to `Debug`. Validate with `go test -race ./dht/...`.
- [ ] **L-03 — `copyStringToByteBuffer` writes `len(str)` bytes with no caller-buffer size** — `capi/toxcore_c.go:264` (callers `tox_self_get_name` ~line 782, `tox_friend_get_status_message` ~line 813) — Boundary safety (C boundary) — `unsafe.Slice(dst, len(str))` followed by `copy` trusts the C caller to have allocated at least `len(str)` bytes (as reported by the corresponding `tox_*_size()` call). This faithfully mirrors the upstream libtoxcore contract, so a conforming caller is safe; a non-conforming caller that under-allocates can corrupt memory. Because this matches the documented libtoxcore ABI and there is no in-process untrusted caller, it is LOW. **Remediation (defensive, optional):** Thread the destination capacity (already known to size-returning callers) into `copyStringToByteBuffer` and validate `len(str) <= cap` before copying, returning `-1` on violation. Validate with `go test ./capi/...`.
- [ ] **L-04 — `cloneReflectValue` shallow-shares unexported pointer fields** — `toxcore_friends.go:284-358` (used by `cloneFriendUserData`, line 264) — Data aliasing — The reflection-based deep copy skips unexported struct fields (`field.CanSet()` is false, line 346), so an arbitrary `Friend.UserData` whose type has unexported pointer fields is only shallow-copied; the original and the "clone" share the pointed-to data. This is already documented inline (lines 277-283) as a known theoretical limitation with no reachable public setter that exposes it. Recorded for completeness. **Remediation:** None required; if stronger guarantees are later needed, document that `UserData` must be a value type or implement an explicit `Clone()` interface that `cloneFriendUserData` prefers. No validation command needed (documentation-only).
- [ ] **L-05 — README overstates cgo isolation to `capi/` only** — documentation vs. code — `crypto/secure_alloc_cgo.go` (`//go:build cgo && (linux||darwin)`) and `av/video/encoder_cgo.go` (`//go:build cgo && libvpx`) use cgo outside `capi/`, contradicting the README line "cgo required only for C API bindings (`capi/` package)". Both have pure-Go fallbacks (`crypto/secure_alloc_nocgo.go`, `av/video/encoder_purgo.go`) and the core builds with `CGO_ENABLED=0` (verified), so the primary "no cgo in core" claim holds; only the "only in capi" phrasing is inaccurate. See GAPS.md. **Remediation:** Reword the README requirement to note that cgo is *optionally* used for hardened memory (`crypto/`) and libvpx VP8 encoding (`av/video/`) in addition to `capi/`. Documentation-only.

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total functions | 1,319 (plus 3,052 methods) |
| Functions above complexity 15 (cyclomatic) | 1 (`cloneReflectValue`, cyclo 16) — both >10 functions inspected |
| Avg cyclomatic complexity | 3.5 |
| Functions > 50 lines | 23 (0.5%); none > 100 lines |
| Doc coverage | 93.5% |
| Duplication ratio | 0.53% (32 clone pairs, largest 17 lines) |
| Test pass rate (library packages, `-tags nonet -race`) | All library packages pass; 1 example fails (`examples/noise_demo`, M-01) |
| go vet warnings | 0 |

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|----------------|
| "CRITICAL: deterministic async message ID leaks plaintext to storage nodes" (`async/client.go:382`) | Storage nodes only observe the *outer* `ObfuscatedAsyncMessage.MessageID`, which is regenerated with `crypto/rand` in `async/obfs.go:336`; the plaintext-derived inner ID is encrypted inside `EncryptedPayload`. Real impact is a recipient-local code smell → recorded as L-01, not CRITICAL. |
| "CRITICAL: context leak in `toxnet` (goroutine leak on Kill)" | `context.WithCancel(context.Background())` creates no propagation goroutine; cancel is invoked by `Close()`. Leak only occurs if the caller violates the standard `net.Conn`/`Listener` Close contract → recorded as MEDIUM (M-02), not CRITICAL. |
| 7 `BUG:` annotations reported by go-stats-generator (`crypto/logging.go:18,24,118`, `crypto/shared_secret.go:15`, `av/types.go:633`, …) | False matches on the substring "de**BUG** logging"; no actual `// BUG` markers exist (`grep '// BUG'` returns none). |
| `panic(...)` in `crypto/secure_memory.go:48`, `transport/nat.go:27`, `dht/mdns_discovery.go:48/51` | All are package-init / constant-address resolution that cannot fail at runtime with the hardcoded literals; idiomatic must-resolve panics, not reachable on user input. |
| `recover()` swallowing panics (`group/chat.go:198`, `transport/worker_pool.go:135`, `capi/*`, `transport/tor_transport_impl.go`) | Each logs the recovered value (e.g. `group/chat.go:199` logs at Error) and is a deliberate callback/worker isolation boundary, documented as such. |
| File-transfer path traversal (`file/transfer.go`) | Multiple defensive layers already present: absolute-path rejection + `..` component rejection in `ValidatePath` (lines 205-225), `filepath.Base` enforcement (line 312), and existing-symlink rejection (line 252, "M-08"). |
| `math/rand` misuse for security | No `math/rand` import in non-test core code; only `dht/local_discovery_test.go` uses it (test jitter). |
| `InsecureSkipVerify` TLS misconfig | Not present anywhere in the tree. |
| RTP packet parsing OOB (`av/rtp/session.go`) | Minimum-length validation precedes indexing (≈line 664); confirmed bounds-checked. |
| Mutex copied by value | `go vet ./...` (copylocks) is clean (0 warnings). |
| `ImportPreKeys` (cyclo 15) corrupted-backup handling (`async/prekeys.go:767`) | Nil bundles/keys are explicitly skipped (M-17, M-ASYNC-2); deep-copied to avoid aliasing; verified safe. |

## Remaining Scope (if session ended before completion)

| Package | Status | Notes |
|---------|--------|-------|
| (all 27 packages) | Audited | A complete pass across all packages and all 3b–3j categories was performed. No unaudited packages remain. A subsequent confirmatory pass produced no new confirmed findings above LOW. |
