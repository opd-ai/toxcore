# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-06-04

## Project Profile

**Project:** `toxcore-go` (`github.com/opd-ai/toxcore`) — a pure-Go implementation of
the Tox peer-to-peer encrypted messaging protocol.

- **Purpose:** DHT-based serverless peer discovery, friend management, 1:1 and group
  messaging, file transfers, audio/video calling (ToxAV), asynchronous offline
  messaging with forward secrecy, and multi-network transport — without cgo in the
  core library.
- **Target users:** Go developers embedding a Tox client/library; C/C++ consumers via
  the libtoxcore-compatible `capi/` bindings.
- **Deployment model:** A long-running peer process that exchanges UDP/TCP/Tor/I2P
  packets with untrusted remote peers. The primary **trust boundary** is every byte
  received from the network (DHT packets, handshakes, RTP media, friend/file/group
  payloads) and any loaded savedata.
- **Critical paths:** packet parsing (`transport/`, `dht/`, `av/rtp/`), cryptographic
  handshakes (`crypto/`, `noise/`, `ratchet/`), async store-and-forward
  (`async/`), and the public Go API plus `capi/` C bridge.
- **Self-declared status:** `SECURITY.md` declares the library **experimental** and
  pending a third-party audit.

## Audit Scope

Full-coverage pass over all 27 non-example packages (46,274 LOC, 1,365 functions +
3,123 methods across 261 files). Examples (`examples/…`, `cmd/…`, `testnet/…`) were
treated as non-critical demonstration code and scanned only opportunistically.

Tooling baseline:
- `go vet ./...` → **0 warnings**.
- `go test -race -tags nonet ./...` → **34 packages ok, 0 failures, 25 no-test-files**
  (the `nonet` tag avoids a known UDP port-conflict flake in `examples/noise_demo`).
- `go-stats-generator analyze . --skip-tests` → metrics in the snapshot below.

Method: seven parallel package-group auditors hunted every checklist category
(3b–3k); all candidate findings were then independently re-verified against source
(file+line+data flow) before inclusion, applying the Phase 3l false-positive filter.

## Coverage Log

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ratchet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport/internal/addressing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bootstrap (+nodes) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| factory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| limits | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| real | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/audio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/rtp | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

> `examples/*`, `cmd/gen-bootstrap-nodes`, and `testnet/*` are demonstration/utility
> code outside the security boundary; scanned but not exhaustively line-audited.

## Goal-Achievement Summary

| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| DHT routing & peer discovery | ⚠️ | M-01 (unsynchronized `Node.Status` reads on maintenance/broadcast paths) |
| Friend management | ✅ | — (L-02 minor unlocked self-key read) |
| 1:1 messaging (+async fallback) | ✅ | — |
| Group chat / conferences (Go) | ✅ | — |
| File transfers | ✅ | — |
| ToxAV audio/video | ⚠️ | M-03 (RTP sequence number skips 0 at wraparound) |
| Asynchronous offline messaging | ✅ | — |
| Multi-network transport | ⚠️ | Nym/Lokinet are dial-only (documented; see GAPS.md Gap 4) |
| Noise-IK handshakes | ✅ | — |
| Cryptography (X25519/ChaCha20/Ed25519) | ✅ | — |
| C API bindings (libtoxcore-compatible) | ❌ | Conference + file-recv callbacks never fire; conference title unimplemented (GAPS.md Gaps 1–2) |
| Go `net.*` interfaces | ✅ | — |
| Protocol version negotiation | ✅ | — |
| Concurrent iteration pipelines | ⚠️ | M-02 (unlocked `connectionStatus`/`nospam` reads from the maintenance goroutine) |

## Findings

> Tests pass under `go test -race`, which is evidence (not proof) against the
> concurrency findings below: the race detector did not trigger because the existing
> test suite does not exercise the specific concurrent producer/consumer pairs
> identified. Each finding is a confirmed *unsynchronized access* that is inconsistent
> with the package's own locked-accessor convention and has a real concurrent writer.

### CRITICAL

_None confirmed._

### HIGH

_None confirmed._ (The two async "use-after-free" and two `crypto/sealed_sender`
"unwiped secret" candidates were rejected — see False Positives.)

### MEDIUM

- [ ] **M-01 — DHT `Node.Status` read without lock on 4 send paths** — `dht/storage_common.go:46`, `dht/group_storage.go:354`, `dht/relay_storage.go:331`, `dht/gossip_bootstrap.go:486` — concurrency (data race) — These four loops read `node.Status` directly while the routing-table maintenance goroutine concurrently writes `Status` under `Node.mu` via `Update`/`RecordPingResponse`/`SetStatus` (`dht/node.go:157,222,227,250`). The codebase added `Node.mu` and the locked accessor `GetStatus()` (`dht/node.go:232`, fix F-DHT-L1) precisely for this field, and 5 other call sites correctly use `GetStatus()`; these 4 bypass it. **Consequence:** a torn/stale `NodeStatus` read can cause a good node to be skipped or a bad node to be queried, and is a genuine data race (undefined behavior under the Go memory model) on the live DHT maintenance/broadcast path. **Data path:** `broadcastAnnouncement → collectBroadcastNodes` (releases `RoutingTable.mu` at `storage_common.go:33`) `→ sendToNodes` (`storage_common.go:46`) runs concurrently with the ping/refresh goroutine calling `node.Update(...)`. **Remediation:** replace the four `node.Status <op> StatusGood` reads with `node.GetStatus() <op> StatusGood`; validate with `go test -race ./dht/...` plus a targeted test that pings nodes in one goroutine while broadcasting in another.

- [ ] **M-02 — `connectionStatus` read without `selfMutex`** — `toxcore_self.go:70` (`SelfGetConnectionStatus`) — concurrency (data race) — Reads `t.connectionStatus` with no lock, while `updateConnectionStatus()` writes it under `t.selfMutex.Lock()` (`toxcore_self.go:97`) from the iteration/maintenance loop. The sibling accessor `SelfGetSecretKey` (`toxcore_self.go:60`) holds `selfMutex.RLock()`, establishing the intended convention. **Consequence:** an exported public API method races with the background pipeline writer, returning a torn/stale `ConnectionStatus`. **Data path:** application goroutine calls `SelfGetConnectionStatus()` while the `Iterate()` pipeline calls `updateConnectionStatus()`. **Remediation:** wrap the read in `t.selfMutex.RLock()/RUnlock()`; validate with `go test -race ./...`.

- [ ] **M-03 — RTP sequence number skips 0 at wraparound** — `av/video/rtp.go:142-145` (`incrementSequenceNumber`) — logic (off-by-one / RFC deviation) — On overflow the code resets `sequenceNumber` to `1` instead of allowing the natural `uint16` wrap to `0` (initial value is also `1`, `av/video/rtp.go:65`). **Consequence:** sequence number `0` is never emitted and `1` is emitted twice per 65,536-packet cycle, violating RFC 3550 §5.1; a standards-compliant receiver computing loss/reordering from sequence gaps will mis-account exactly one packet at each wraparound. Secondary (media) path, edge-case impact. **Data path:** any `RTPPacketizer` that transmits ≥ 65,535 video packets in a call. **Remediation:** drop the `if rp.sequenceNumber == 0 { rp.sequenceNumber = 1 }` reset and allow natural wraparound; optionally randomize the initial value per RFC 3550. Validate with a unit test asserting `65535 → 0 → 1`.

- [ ] **M-04 — `nospam` read without `selfMutex` in DHT maintenance** — `toxcore_lifecycle.go:259` (`doDHTMaintenance`) — concurrency (data race) — `crypto.NewToxID(t.keyPair.Public, t.nospam)` reads `t.nospam` with no lock from the maintenance goroutine, while `SelfSetNospam` (`toxcore_self.go:40`) writes `t.nospam` under `t.selfMutex.Lock()` and is a public, user-callable mutator. **Consequence:** a data race on `nospam` whenever an application changes its nospam concurrently with iteration, potentially producing a torn Tox ID in the self-announcement. **Data path:** user calls `SelfSetNospam(...)` while the `Iterate()` pipeline runs `doDHTMaintenance()`. **Remediation:** snapshot `nospam` (and `keyPair.Public`) under `selfMutex.RLock()` at the top of `doDHTMaintenance` before constructing the Tox ID; validate with `go test -race ./...`.

### LOW

- [ ] **L-01 — `keyPair.Public` read without lock in friend-request build** — `toxcore_friends.go:684` (`buildFriendRequestPacket`) — concurrency (data race, theoretical) — Reads `t.keyPair.Public` with no lock; sibling `SelfGetPublicKey` (`toxcore_self.go:49`) uses `selfMutex.RLock()`. `keyPair` is only reassigned during `load()` (`toxcore_lifecycle.go:499`), which normally completes before goroutines start, so a live race is unlikely. **Consequence:** inconsistency with the locking convention; a torn read only if savedata is loaded after the instance is already iterating. **Remediation:** snapshot `keyPair.Public` under `selfMutex.RLock()`; validate with `go test -race ./...`.

- [ ] **L-02 — `keyPair.Private` read without lock in `GetSelfPrivateKey`** — `toxcore_self.go:227` — concurrency (data race, theoretical) — Reads `t.keyPair.Private` unlocked, whereas the sibling `SelfGetSecretKey` (`toxcore_self.go:60`) holds `selfMutex.RLock()`. Same reasoning as L-01 (`keyPair` effectively immutable post-construction). **Consequence:** convention inconsistency; theoretical torn read versus a concurrent `load()`. **Remediation:** acquire `selfMutex.RLock()` before reading; validate with `go test -race ./...`.

- [ ] **L-03 — One-time pre-key is *not* converted to Curve25519 in multi-device X3DH** — `crypto/multi_device.go:171-173` (`AddDevice`) — API/behavioral contract — A comment states that, in production, Ed25519 identity keys "would be converted to Curve25519 using `DeriveX25519FromEd25519Seed` before X3DH initiation" but the simplified path assumes the keys are already Curve25519. **Consequence:** if a caller supplies Ed25519-form keys in a `DeviceBundle`, the X3DH DH operations silently use the wrong key type; only a documentation/usage hazard today because no exported path feeds Ed25519 keys here. (Single-use OPK accounting — a prior gap — is now correctly enforced via `mds.UsedOPKs`, `crypto/multi_device.go:151-193`.) **Remediation:** either perform the documented conversion inside `AddDevice` or document the precondition on the exported method and reject non-Curve25519 input; add a unit test.

- [ ] **L-04 — `cloneReflectValue` cannot deep-copy unexported pointer fields** — `toxcore_friends.go:302-376` — data aliasing (acknowledged, labeled L-4) — The reflection-based best-effort deep copy shallow-shares unexported struct fields (`CanSet()==false`). This is explicitly documented at `toxcore_friends.go:291-301` and judged theoretical. **Consequence:** if a future caller stores `UserData` with unexported pointer fields, the "copy" aliases the original. **Remediation:** none required now; if needed, delegate to an optional `Clone()` method on `UserData`. Re-verified as the acknowledged L-4 limitation.

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total functions (free funcs) | 1,365 |
| Total methods | 3,123 |
| Functions > 50 lines | 39 (0.9%) |
| Functions > 100 lines | 2 |
| Functions cyclomatic > 10 | 13 |
| Highest cyclomatic complexity | `PQXDHInitiate`/`PQXDHRespond` (18) |
| Avg cyclomatic complexity | 3.6 |
| Doc coverage (overall) | 93.7% (packages 100%, funcs 98.8%) |
| Duplication ratio | 0.66% (40 clone pairs, largest 22 lines) |
| Circular dependencies | 0 |
| Test pass rate (`-race -tags nonet`) | 34/34 packages (0 fail) |
| `go vet` warnings | 0 |

All 13 functions with cyclomatic complexity > 10 and all 39 functions > 50 lines were
inspected manually (notably `crypto/pqxdh.go` PQXDHInitiate/Respond, `crypto/x3dh.go`,
`crypto/multi_device.go` UpdateDeviceList/AddDevice, `ratchet/session.go`
dhRatchetStep/RatchetDecryptWithEncryptedHeader, `toxcore_friends.go`
cloneReflectValue, `toxcore.go` checkForRiskyConfigurations); no additional defects
beyond those listed were confirmed.

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|-----------------|
| `async/storage.go:653` & `:1190` — "use-after-free: `&stored`/`&msg` (local var) stored in a map, dangling after return" | **Not a bug in Go.** Taking the address of a local that escapes the function causes the compiler's escape analysis to heap-allocate it; the pointer remains valid and is GC-managed. There is no use-after-free for escaping locals in Go. Both sites also intentionally deep-copy the payload (`storage.go:650`, comment M-18). |
| `crypto/sealed_sender.go:111` & `:193` — "secret HMAC proof buffer (`proofBytes`/`expectedProof`) not zeroized" | The proof is a **public** HMAC authentication tag that is stored in `cert.Proof` and transmitted on the wire (`sealed_sender.go:119`). It is not secret key material, so wiping it provides no confidentiality benefit. The secret HMAC key (`proofSecret`) is the sensitive value and is derived locally. |
| `crypto/multi_device.go` session type assertions | Use the checked `switch s := session.(type)` form with explicit cases; no unchecked assertion risk (`multi_device.go:218-223`). |
| `crypto/pqxdh.go` pointer dereferences | All deref sites are nil-guarded (e.g. `params.PeerOneTimePreKeyPublic != nil && *... != [32]byte{}`). |
| `transport/nym_packetconn.go:74` — uint32 overflow on `len(p)` | `WriteTo` is only fed serialized Tox packets bounded far below 4 GiB; unreachable from untrusted input. |
| `transport` version-negotiation parse ambiguity (2-byte vs extended) | Parser correctly tries legacy first then extended; both lengths handled (`version_negotiation.go`). |
| `dht` packet parsers (`parseNodeEntry`, `parseRelayAnnouncements`) OOB | Bounds checked before slice access (e.g. `storage offset+announcementLen > len(data)`); `DeserializeAnnouncement` guards `nameLen` (F-DHT-H3). |
| `transport.peerVersions` unbounded map | LRU eviction enforces `maxPeerVersionEntries` (`version_negotiation.go:793-796`). |
| `av/video/processor.go:222` `w*h` overflow | `ValidateFrameSize()` caps dimensions to 16383×16383; product fits `int` on 32-bit. |
| `av/rtp/transport.go:300` odd-length audio drops last byte | Intentional handling of odd PCM frame lengths, not a bounds bug. |
| `file`/`messaging`/`group` chunk math, path traversal, role checks | Bounds-validated serialize/deserialize, `filepath.Clean` + traversal guard, and correct `self.Role > target.Role` permission logic; close-on-defer present. |
| `toxnet` `net.*` Read/Write/Close, `ToxPacketConn.processPackets` | Conn lifecycle uses `closeOnce`, context cancel, and `defer wg.Done()`; no leak. |
| `toxcore_friends.go:302` `cloneReflectValue` unexported-field sharing | Acknowledged L-4 limitation (documented at `:291-301`); listed as L-04, not a new defect. |

## Remaining Scope (session completed)

A full coverage pass completed for all 24 in-boundary packages (see Coverage Log).
Example/CLI/testnet trees (`examples/*`, `cmd/*`, `testnet/*`) are out-of-boundary
demonstration code and were not exhaustively line-audited; no security-relevant
findings surfaced during the opportunistic scan. No package remains unaudited.

| Package | Status | Notes |
|---------|--------|-------|
| All in-boundary packages | Audited | Complete |
| `examples/*`, `cmd/*`, `testnet/*` | Scanned (non-boundary) | Demo/utility code; deep audit not warranted |
