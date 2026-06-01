# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-06-01

## Project Profile

**Purpose**: `toxcore-go` (module `github.com/opd-ai/toxcore`) is a pure-Go implementation
of the Tox peer-to-peer encrypted messaging protocol. It provides DHT-based peer discovery,
friend management, 1-to-1 and group messaging, file transfers, ToxAV audio/video calling,
asynchronous (store-and-forward) offline messaging with forward secrecy, multi-network
transport (IPv4/IPv6, Tor, I2P, Lokinet, Nym), Noise-IK handshakes, and libtoxcore-compatible
C API bindings.

**Target users**: Go application developers building Tox clients/bots, and C/C++ developers
linking the `capi/` shared library.

**Deployment model**: Embedded library inside a host process. Every Tox node both initiates
and accepts traffic from arbitrary remote peers over the public Internet (and overlay
networks). **Network packets, file metadata, remote filenames, RTP media, and friend-supplied
data are all untrusted input.** The C API runs in-process with the host via cgo.

**Critical paths** (primary stated goals — given deepest scrutiny):
- DHT discovery and packet parsing (`dht/`, `transport/`)
- Cryptography and forward secrecy (`crypto/`, `async/`, `ratchet/`, `noise/`)
- Messaging delivery and async fallback (`messaging/`, `async/`)
- File transfer negotiation and chunking (`file/`, `toxcore_file.go`)
- ToxAV media pipeline and C bindings (`av/...`, `toxav.go`, `capi/`)

**Trust boundaries**: Untrusted input enters at (a) UDP/TCP packet handlers in `transport/`
and `toxnet/`, parsed in `dht/`, `messaging/`, `file/`, `group/`, `async/`; (b) RTP packets
in `av/rtp`, `av/video`; (c) C-supplied pointers/lengths in `capi/`. The audit traced how far
each travels before validation.

## Audit Scope

Packages audited (non-test `.go` files; tests read for context):

| Role | Packages |
|------|----------|
| Core facade | `toxcore` (root: `toxcore.go`, `toxcore_*.go`, `iteration_pipelines.go`) |
| Crypto/security | `crypto`, `async`, `ratchet`, `noise` |
| Networking | `dht`, `transport`, `transport/internal/addressing`, `toxnet`, `bootstrap` |
| Messaging/contacts | `friend`, `messaging`, `group` |
| File transfer | `file` |
| Media + C ABI | `toxav.go`, `av`, `av/audio`, `av/video`, `av/rtp`, `capi` |
| Support | `factory`, `interfaces`, `limits`, `real`, `simulation` |

Method: structural risk scan via `go-stats-generator`, then a systematic per-package pass over
checklist categories 3b–3k, with the Phase 3l false-positive filter applied to every candidate.
The `crypto`/`async`/`ratchet`/`noise` cluster received an additional independent manual
spot-check (RNG sourcing, constant-time comparison usage, pre-key consumption rate-limiting,
bounded skipped-key retention, secure memory wiping) — see Remaining Scope.

## Audit Scope — Metrics Summary

- 251 files, 44,100 LOC, 1,318 functions, 3,050 methods, 27 packages.
- Only 2 functions exceed cyclomatic complexity 10 (`cloneReflectValue` 16, `ImportPreKeys` 15);
  both inspected manually and carry prior-audit annotations.
- 23 functions exceed 50 lines (0.5%); none exceed 100 lines.
- Doc coverage 93.5% overall (100% packages, 98.75% functions).
- Duplication ratio 0.52% (largest clone 17 lines, mostly in `examples/`).

## Coverage Log

✅ = checklist category completed for the package.

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bootstrap | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av / av/audio / av/video / av/rtp | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| factory / interfaces / limits / real / simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ratchet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary

| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| DHT-based peer discovery | ✅ | — (gossip multi-network limitation: GAPS G-01, LOW L-01) |
| Friend management | ✅ | — |
| 1-to-1 messaging with retry/padding | ⚠️ | M-06, M-07 (stuck-message edge cases) |
| Group chat | ✅ | L-03 (nil-transport panic, edge case) |
| **File transfers** | ❌ | **C-01 — `FileSend` wire format unreadable by receiver** |
| ToxAV audio/video | ⚠️ | M-08 scaler bounds; L-06 jitter aliasing; inter-frame decode is a documented limitation |
| Async offline messaging + forward secrecy | ✅ | — (state-cleanup leak M-ASYNC-1; no crypto CRITICAL/HIGH) |
| Multi-network transport | ⚠️ | M-01 (overlay address double-port), GAPS G-01 |
| Noise-IK forward secrecy | ⚠️ | M-04, L-02 (rollback/replay layer inactive — interop still works) |
| NAT traversal | ✅ | — |
| C API bindings | ⚠️ | H-01, M-09 (cgo pointer/OOM safety) |
| `net.*` interfaces (toxnet) | ⚠️ | M-03 (unbounded buffer), M-05 (encryption not enforceable) |

## Findings

### CRITICAL

- [x] **File-transfer request wire format is incompatible between sender and receiver**

### HIGH

- [x] **Unvalidated C `ToxAV` handle dereference can crash the host process**

### MEDIUM

- [x] **Incoming file chunk `position` is parsed but ignored; writes are sequential** — `file/manager.go:429,443`, `file/transfer.go:491` (`WriteChunk`) — boundary / state logic — The remote-supplied `position` from a file-data packet is decoded and forwarded to callbacks but the actual write appends sequentially via `WriteChunk`; there is no check that `position == transfer.GetTransferred()`. A peer that sends duplicate, reordered, or gapped chunks corrupts the saved file while ACKs/progress advance as if the data were valid. — **Remediation**: Require `position == transfer.GetTransferred()` (reject/seek otherwise) or implement bounded `WriteAt(data, position)` with duplicate tracking. Add tests for duplicate and out-of-order chunks; validate with `go test -tags nonet -race ./file`.

- [x] **Message queued before a transport is configured becomes permanently stuck** — `messaging/message.go:1166-1178` (`sendThroughTransport`), with state set at `updateMessageSendingState` (called from `attemptMessageSend`, `message.go:1214`) — state machine — `attemptMessageSend` transitions the message to `MessageStateSending`, then `sendThroughTransport` early-returns when `mm.transport == nil` **without restoring `Pending`**. The code comment claims the message "stays in its current state (Pending)", but the state is actually `Sending`. `shouldProcessMessage`/the reprocessing path only act on `MessageStatePending` (`message.go:951-960`), so the message is never retried even after a transport is later configured — silent message loss in the exact scenario the comment (M-MSG-2) intended to fix. — **Remediation**: Check `mm.transport == nil` *before* transitioning to `Sending`, or reset to `MessageStatePending` when the transport is nil. Add a test that queues a message with a nil transport, configures one, and asserts delivery; validate with `go test -tags nonet -race ./messaging`.

- [x] **Overlay (`.onion`/`.i2p`/`.nym`/`.loki`) addresses serialize with a duplicated port** — `transport/address_parser.go:378` (`parsePrivacyNetworkAddress` stores `Data: []byte(address)` = full `host:port`) and `transport/address.go:115-118` (`toCustomAddr` re-appends `Port` via `net.JoinHostPort`) — logic — The parser keeps the entire `host:port` string in `Data` while also setting `Port`. When converted back to a `net.Addr`, `toCustomAddr` joins `Data` with `Port` again, yielding `host:port:port` (e.g. `[abcd.onion:80]:80`), which breaks overlay dialing and any round-trip serialization. — **Remediation**: Store only the host in `Data` (strip the port at parse time) so `Port` is the single source of truth. Add parser→`ToNetAddr` round-trip tests for each overlay type; validate with `go test -tags nonet -race ./transport`.

- [x] **Stale per-friend async state (including pre-key readiness channels) leaks on friend removal (M-ASYNC-1)** — `async/manager.go:355-386` (`RemoveFriend` → `ClearPendingMessagesForFriend`) — resource lifecycle / state cleanup — `ClearPendingMessagesForFriend` deletes `pendingMessages`, `onlineStatus`, `friendAddresses`, and `friendSignKeys` but never deletes `am.preKeyReadyCh[friendPK]`, so the channel and its map entry survive deletion. Worse, the function early-returns when the friend has no queued messages (`manager.go:376-378`), so for such friends **none** of the per-friend state is cleaned up at all. Over many add/remove cycles this leaks channels and stale signing-key/address state. — **Remediation**: Move the per-friend `delete(...)` cleanup (and `delete(am.preKeyReadyCh, friendPK)`) above the early-return so it always runs, regardless of pending-message count. Validate with `go test -tags nonet -race ./async`.

- [x] **Unbounded per-stream read buffer enables memory exhaustion** — `toxnet/callback_router.go:93` — resource exhaustion / DoS — Incoming stream messages are appended to a `bytes.Buffer` with no cap or backpressure. A malicious or slow-draining friend can grow the buffer without limit while the application reads slowly, exhausting host memory. — **Remediation**: Enforce a maximum buffered byte count; drop the connection (or return an error) on overflow. Validate with `go test -tags nonet -race ./toxnet`.

- [x] **Noise version-commitment / rollback-detection layer is initialized but never active** — `transport/noise_transport.go:681` (`sendVersionCommitment` is unused) and `:695` (app data sent without checking `versionCommitted`) — security logic — The transport sets up a version-commitment exchange intended to detect protocol-version rollback, but `sendVersionCommitment` is never called and application data is sent regardless of whether a commitment was verified, so the rollback-detection guarantee is inert. Core Noise-IK interop still works; the *additional* downgrade-detection layer does not. — **Remediation**: Call `sendVersionCommitment` after the handshake and gate application-data sends until the peer's commitment is verified, or remove the dormant layer and the comments that imply it is enforced. Validate with `go test -tags nonet -race ./transport`.

- [x] **Encrypted `ToxPacketConn` falls back to returning plaintext on unknown peer / decrypt failure** — `toxnet/packet_conn.go:580-598` (`decryptPacket`) — security (authenticity) — When a peer key is unknown or decryption fails, `decryptPacket` returns the original bytes as if they were valid plaintext (an explicitly acknowledged "mixed encrypted/unencrypted" design, per the inline comment). Consequently a caller that believes it has enabled encryption cannot rely on confidentiality or authenticity: forged plaintext from an unknown source is delivered to the reader. Reported at MEDIUM because the behavior is documented in-code as intentional mixed-mode, but it remains a real foot-gun. — **Remediation**: Add an opt-in "encryption required" mode that drops (or errors on) packets from unknown peers and on decrypt failure, and document that the default is best-effort mixed-mode. Validate with `go test -tags nonet -race ./toxnet`.

- [x] **Unchecked `C.malloc` result in `toxav_new`**

- [x] **`Scaler.Scale` accepts a stride smaller than width and indexes out of range** — `av/video/scaling.go:188-199` (`validatePlaneBuffers` only checks `len >= height*stride`) and `:201-225` (`interpolatePixel` indexes `y1*srcStride + x1` with `x1` up to `width-1`) — bounds safety — The public scaler validates buffer length against `height*stride` but interpolation indexes up to `(height-1)*stride + (width-1)`. When `stride < width` the maximum index exceeds the validated bound, causing an out-of-range panic on a caller-supplied `VideoFrame`. — **Remediation**: In `validatePlaneBuffers`, require `stride >= width` and `(height-1)*stride + width <= len(src)`. Add a test with `stride < width`; validate with `go test -tags nonet -race ./av/video`.

- [x] **Version-negotiation packets can ping-pong between two peers** — `transport/negotiating_transport.go:~350` — logic / DoS (uncertainty: not reproduced empirically) — An incoming `PacketVersionNegotiation` appears to be treated as both a response and a fresh request, and is answered with another identical negotiation packet; two peers exchanging these could loop. Trace suggests this is reachable but the audit did not produce a runtime repro — treat as MEDIUM pending a test. — **Remediation**: Distinguish request vs. response (a flag or a "pending negotiation satisfied" check) so a response is not itself answered. Add a two-peer negotiation test asserting convergence to a single exchange; validate with `go test -tags nonet -race ./transport`.

### LOW

- [ ] **`sendNodes` header count can exceed the number of serialized entries** — `dht/handler.go:~710` — protocol logic — The node count is written into the header before each node is serialized; if a later per-node conversion/serialization fails and is skipped, the advertised count overstates the body, producing a slightly malformed (but self-consistent enough to not crash) response. — **Remediation**: Count only successfully serialized nodes, or patch the count byte after the loop. Validate with `go test -tags nonet -race ./dht`.

- [ ] **Noise handshake "replay" check validates a freshly-created local nonce, not the peer's** — `transport/noise_transport.go:~607` — security / DoS (limited) — The inbound-handshake replay guard checks a responder-side nonce created locally just before processing, rather than an authenticated nonce/timestamp carried in the peer's message, so captured valid handshakes are not actually replay-filtered by this code. Practical impact is limited because the Noise handshake itself prevents session compromise; the guard is misleading rather than a key-compromise vector. — **Remediation**: Bind replay detection to an authenticated peer nonce/timestamp inside the handshake payload, or remove the guard and its comment. Add a replay test; validate with `go test -tags nonet -race ./transport`.

- [ ] **Nil group transport can panic during broadcast after peer discovery** — `group/chat.go:~1865` (`SendMessage` → `g.transport.Send`) — nil safety / API contract — A `Chat` constructed without a transport can still register an online peer via `HandlePeerAnnounce`; a later `SendMessage` reaches `g.transport.Send` on a nil transport and panics. — **Remediation**: Return an error when `g.transport == nil` before attempting a broadcast. Validate with `go test -tags nonet -race ./group`.

- [ ] **`AddFriend(friendID, nil)` panics in logging before validation** — `real/packet_delivery.go:~329` — nil safety — The function logs `addr.String()` before validating that `addr` is non-nil, panicking on a nil address. — **Remediation**: Nil-check `addr` before logging/use. Validate with `go test -tags nonet -race ./real`.

- [ ] **Public constructors panic on nil config** — `real/packet_delivery.go:~40`, `simulation/packet_delivery_sim.go:~43` — initialization — Calling these constructors directly with a nil config panics (the factory path guards this, but the constructors are exported). — **Remediation**: Apply defaults or return an error on nil config. Validate with `go test -tags nonet -race ./real ./simulation`.

- [ ] **RTP jitter buffer stores the un-copied `pion/rtp` payload (buffer aliasing)** — `av/rtp/packet.go:~394,~634` — data aliasing — `pion/rtp.Unmarshal` returns a payload slice that aliases the input buffer; the jitter buffer retains it without copying. If the underlying receive buffer is reused/mutated before the buffered packet is consumed, the buffered audio is corrupted. — **Remediation**: Copy the payload before buffering. Validate with `go test -tags nonet -race ./av/rtp`.

- [ ] **Corrupted/nil pre-key material can panic the pre-key exchange path** — `async/forward_secrecy.go:~550` (`ExchangePreKeys`) — nil / boundary safety — If a persisted or imported bundle contains an unused `PreKey` whose `KeyPair` is nil, `ExchangePreKeys` dereferences `key.KeyPair.Public` and panics instead of skipping/erroring. Reachable only via corrupted on-disk/imported state (the in-memory import path filters nil entries, AUDIT M-ASYNC-2), so impact is a local crash on tampered data. — **Remediation**: Skip or reject entries with `KeyPair == nil` before use. Validate with `go test -tags nonet -race ./async`.

- [ ] **Corrupted pre-key material can panic the deprecated decrypt path** — `async/forward_secrecy.go:~493` (`DecryptForwardSecureMessage` → `CheckAndMarkPreKeyUsed`) — nil / boundary safety — `crypto.Decrypt(..., preKey.KeyPair.Private)` nil-derefs when stored pre-key material is corrupted. The path is deprecated and the impact is a local crash. — **Remediation**: Check `preKey.KeyPair != nil` before decrypting. Validate with `go test -tags nonet -race ./async`.

- [ ] **`MessageManager` concurrency: potential lock-ordering concern under concurrent `ProcessPendingMessages`** — `messaging/message.go:951` (`message.mu`) and `:1166,:1269` (`mm.mu`) — concurrency (uncertainty: not confirmed) — The public docs permit concurrent `ProcessPendingMessages` calls. Both `message.mu` and `mm.mu` are used, but the audit did **not** confirm a nested acquisition in opposing orders (the observed paths acquire and release `message.mu` before taking `mm.mu`). Recorded as LOW with explicit uncertainty; `go test -race` currently passes. — **Remediation**: Document/enforce a single lock-ordering invariant (or snapshot `mm` config before locking per-message state) and add a concurrent stress test with a timeout to rule out deadlock; validate with `go test -tags nonet -race ./messaging`.

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total functions | 1,318 (+3,050 methods) |
| Functions above complexity 15 | 2 (`cloneReflectValue` 16, `ImportPreKeys` 15) |
| Functions above complexity 10 | 2 |
| Functions > 50 lines | 23 (0.5%) |
| Avg cyclomatic complexity | 3.5 |
| Doc coverage (overall) | 93.5% |
| Duplication ratio | 0.52% (largest clone 17 lines) |
| Test pass rate | All packages `ok` (`go test -tags nonet`), 0 failures, 0 data races |
| go vet warnings | 0 |

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|-----------------|
| Path traversal via remote filename in file transfer | `deserializeFileRequest` strips to `filepath.Base`; `Transfer.openTransferFile` additionally rejects pre-existing symlinks (M-08) and `ValidatePath` enforces `ErrDirectoryTraversal` (`file/transfer.go:220-255`). |
| Oversized incoming file/chunk integer overflow | `deserializeFileRequest` enforces `MaxFileSize`/`MaxFileNameLength`; `WriteChunk` bounds chunk size and remaining bytes. |
| Gossip `handleSendNodes` header-offset bug (prior H-01) | **Fixed** — now reads count at `Data[32]`, starts at `offset=33`, guards `len<33` (`dht/gossip_bootstrap.go:246-269`). Remaining multi-network limitation tracked in GAPS G-01. |
| VP8 RTP descriptor "not RFC 7741" (prior G-02/M-01) | **Fixed** — `buildVP8Payload` now emits the X bit, extension octet, and 15-bit `M`-bit PictureID (`av/video/rtp.go:175-205`). |
| `math/rand` used for security | None in core packages; `crypto` uses `crypto/rand` exclusively (`crypto/encrypt.go:20`, `keystore.go:187`), GCM nonces are unique-per-encryption. |
| Non-constant-time secret comparisons | Secret/MAC/pseudonym comparisons use `subtle.ConstantTimeCompare`/`hmac.Equal` (`crypto/constant_time.go`, `async/obfs.go:131,393`, `async/client.go:1384`). |
| `cloneReflectValue` complexity 22.3 deep-copy gap | Acknowledged prior-audit comment (L-4): unexported pointer fields are shallow-shared, but no reachable public setter populates arbitrary `UserData` with such fields (`toxcore_friends.go:277-358`). |
| `ImportPreKeys` aliasing/nil entries | Deep-copies via `clonePreKeyBundle`, filters nil `KeyPair` (M-ASYNC-2), in-place `Keys[:0]` filter operates on the clone only (`async/prekeys.go`). |
| `parseVP8Payload` panic on malformed RTP | Length guards cover all indexed accesses (`av/video/rtp.go`). |
| STUN / SOCKS5 / TCP-frame parsing overflow | Header/attribute lengths checked; TCP frame length capped at 1 MiB; SOCKS5 address forms length-checked. |
| File-transfer callback deadlocks | Callbacks intentionally unlock before invocation (acknowledged M-FILE-3). |
| `ratchet` deterministic nonce derivation | Nonce derived from a unique per-message key and used exactly once — no reuse path. |
| `noise` private-key copy not wiped immediately after handshake | In-code comment documents `flynn/noise` library slice ownership; wiping early would break the handshake. |
| `async` random nonces reuse | All from `crypto/rand`; no confirmed reuse path. |

## Remaining Scope

A complete checklist pass (3b–3k) was performed for **every** package, including a dedicated
deep-dive over `crypto`, `async`, `ratchet`, and `noise`. No CRITICAL or HIGH findings exist in
the cryptographic core: RNG is sourced exclusively from `crypto/rand`, GCM nonces are unique per
encryption, secret/MAC comparisons are constant-time (`subtle.ConstantTimeCompare`/`hmac.Equal`),
ratchet nonces are derived from unique message keys and used once, skipped keys are bounded
(`MaxSkippedKeys=1000`), and pre-key consumption is rate-limited. The residual crypto-area
findings (async pre-key channel leak M-ASYNC-1; two nil-`KeyPair` panics on corrupted/imported
state) are recorded above at MEDIUM/LOW.

No empirical data race or test failure was observed (`go test -tags nonet -race ./...` clean),
which is supporting (not conclusive) evidence against undetected races on exercised paths.

A complete additional pass produced no **new** confirmed findings above LOW beyond those listed,
satisfying the end-to-end stop condition.
