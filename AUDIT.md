# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-06-04

## Project Profile

**`toxcore-go`** is a pure-Go implementation of the Tox peer-to-peer encrypted
messaging protocol (`github.com/opd-ai/toxcore`, Go 1.25, toolchain go1.25.11).
It provides DHT peer discovery, friend management, 1:1 and group messaging, file
transfer, ToxAV audio/video calling, asynchronous offline messaging with forward
secrecy, multi-network transport (UDP/TCP/Tor/I2P/Nym/Lokinet), Noise-IK
handshakes, NAT traversal, a libtoxcore-compatible C API (cgo), and `net.*`
adapters.

- **Target users:** Go application developers embedding Tox, and C/C++ programs
  linking the shared library via `capi/`.
- **Deployment model:** Library / embedded daemon; the core builds with
  `CGO_ENABLED=0`. Processes parse **untrusted network input** at every transport
  and protocol boundary, so parsing/deserialization safety and concurrency are the
  dominant risk surfaces.
- **Critical paths:** `crypto/` (key exchange, AEAD, PQXDH/X3DH, sealed sender),
  `ratchet/` (Double Ratchet), `async/` (offline store-and-forward, prekeys,
  forward secrecy), `transport/` + `dht/` + `noise/` (untrusted wire parsing and
  handshakes), and the root `toxcore` package (lifecycle, friends, iteration
  pipelines).

**Maturity note:** The tree carries embedded prior-audit labels throughout
(`C-/H-/M-/L-…`, `F-DHT-L1`, `M-06`, `M-08`, `M-17`, `M-ASYNC-2`, `L-4`, `L-12`).
A prior `GAPS.md` referenced an `AUDIT.md` (findings `H-1…H-6`, `M-1…M-12`) whose
issues were **re-verified during this pass and found remediated in the current
tree** (see "False Positives Considered and Rejected" and `GAPS.md`). This is an
actively maintained codebase where findings get fixed; current confirmed findings
are therefore concentrated at LOW severity (defense-in-depth hardening).

## Audit Scope

Deep manual audit (all checklist categories) was performed on the core-path
packages below, guided by `go-stats-generator` structural metrics and corroborated
by `go vet` (clean) and `go test -race ./...` (all passing). Five parallel
package-group passes were run and **every candidate finding was independently
re-verified against the source** before inclusion; the majority of machine-raised
candidates were refuted (see rejected list).

- Functions/methods inspected (structural pass): **4,488** (1,365 functions +
  3,123 methods) across 261 non-test files.
- All 38 functions with length > 50 lines and all 11 functions with cyclomatic
  complexity > 10 were manually inspected (the five with cyclomatic ≥ 15 received
  line-by-line review): `PQXDHInitiate`, `PQXDHRespond` (`crypto/pqxdh.go`),
  `UpdateDeviceList` (`crypto/multi_device.go`), `cloneReflectValue`
  (`toxcore_friends.go`), `ImportPreKeys` (`async/prekeys.go`).

## Coverage Log

✅ = checklist category completed for the package during this pass.

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ratchet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport (+internal/addressing) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/audio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/rtp | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

Not deeply audited this pass (non-core: demos, tooling, simulation, bootstrap data
— see Remaining Scope): `examples/*`, `cmd/*`, `testnet/*`, `simulation`, `real`,
`factory`, `interfaces`, `limits`, `bootstrap`, `bootstrap/nodes`.

## Goal-Achievement Summary

| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| DHT routing / peer discovery | ✅ | — (`dht.Node` now lock-protected, F-DHT-L1) |
| Friend management | ✅ | — (L-01 theoretical only) |
| 1-to-1 messaging | ✅ | — |
| Group chat (sender keys, replay protection) | ✅ | — (prior replay sentinel bug fixed: `SeenFirstMessage`) |
| File transfers (pause/resume/cancel) | ✅ | — (Go API); ⚠️ C-API events: see GAPS Gap 1 |
| ToxAV audio/video | ✅ | — (L-05 empty-frame edge only) |
| Async offline messaging + forward secrecy | ✅ | — (L-02 shutdown-blocking edge) |
| Multi-network transport | ⚠️ | Nym is dial-only by design (documented); service-host unimplemented |
| Noise-IK handshakes + version negotiation | ✅ | — (prior downgrade-commitment routing/binding fixed) |
| NAT traversal (STUN/UPnP/hole-punch) | ✅ | — (UPnP SSRF mitigated, M-06) |
| Cryptography (Curve25519/ChaCha20-Poly1305/Ed25519) | ✅ | — |
| C API bindings (libtoxcore-compatible) | ⚠️ | Conference & file callbacks register but don't invoke C; conference title unimplemented (GAPS Gap 1, 2) |
| Go `net.*` interfaces | ✅ | — |
| Protocol version negotiation | ✅ | — |
| Concurrent iteration pipelines | ✅ | — (L-03 sub-interval scheduling imprecision) |

## Findings

All findings below were confirmed by reading the exact source and tracing the data
flow. No CRITICAL or HIGH confirmed findings were identified in this pass; the
machine-raised CRITICAL/HIGH candidates were all refuted (see rejected table).

### CRITICAL

_None confirmed._

### HIGH

_None confirmed._

### MEDIUM

- [ ] **Unauthenticated pre-key packet triggers proportional allocation before the known-friend check** — `async/manager.go:1190` (and `:1262` `extractPreKeysFromPacket`) — security / resource (defense-in-depth) — `handlePreKeyExchangePacket` calls `parsePreKeyExchangePacket` (which allocates `make([]PreKeyForExchange, keyCount)`) **before** the `am.friendAddresses[senderPK]` anti-spam check at `:1131`. `keyCount` is a wire `uint16` (max 65535 → ~2.36 MB) and is validated only against the packet's own exact length (`verifyPreKeyPacketSize`, `:1234`), so the allocation is **bounded 1:1 by the received bytes (no amplification)** and is further capped in practice by datagram MTU. A self-signed packet (the Ed25519 key is taken from the packet itself) is cheap to forge, so an unauthenticated peer can force one packet-sized allocation per packet. Concrete consequence: modest transient memory churn under a flood; not an amplification vector. **Data flow:** untrusted datagram → `handlePreKeyExchangePacket` → `parsePreKeyExchangePacket` (`verifyPreKeyPacketSize` exact-length, then `verifyPreKeyPacketSignature` self-signed, then `extractPreKeysFromPacket` allocates) → only afterward the known-friend gate at `:1131` rejects unknown senders. **Remediation:** add an explicit protocol maximum (e.g. `const maxPreKeysPerExchange = 512`) and reject larger `keyCount` in `extractPreKeyPacketHeaders` (`:1225`) before allocation; optionally move the cheap `friendAddresses` membership check ahead of `parsePreKeyExchangePacket`. Validate with `go test -race ./async/...` plus a new table test feeding an oversized `keyCount`.

### LOW

- [ ] **Length-prefixed wire fields lack explicit protocol-maximum bounds before `make()`** — `transport/version_negotiation.go:137` (`numVersions`), `transport/versioned_handshake.go:179` (`readNoiseMessage` `noiseLen`), `transport/parser.go:229` (`addrLen` for overlay address types Onion/I2P/Nym/Loki) — logic/boundary (defense-in-depth) — each reads a length from untrusted input and allocates a slice. In every case the code first validates the **exact** remaining buffer length (`len(data) != 2+numVersions`; `len(data) < offset+noiseLen`; `len(data) < currentOffset+addrLen+2`), so the allocation is bounded 1:1 by the received packet and **cannot over-read** — these are not amplification or OOB bugs, only missing upper-bound sanity limits on a trust boundary. **Remediation:** introduce named maxima (`maxSupportedVersions`, `MaxNoiseMessageSize`, `maxOverlayAddressLen`) checked before allocation, and add fuzz targets (`go test -fuzz=… ./transport/`). Backward-compatible (server-side validation only).
- [ ] **`ForwardSecurityManager.Close()` can block on a slow caller-supplied `preKeyRefreshFunc`** — `async/forward_secrecy.go:259` / `:273` — resource lifecycle — `Close()` correctly unlocks `closedMu` (`:263`) **before** `cleanupWg.Wait()` (`:273`), so there is no deadlock; however, any proactive-refresh goroutine spawned by `proactiveRefreshAll` (`:245`) invokes the user-registered `preKeyRefreshFunc` with no timeout, so if that callback blocks (e.g. a hung network call) `Close()` waits for it. Bounded by peer count and only triggered on the weekly refresh tick. **Remediation:** run `preKeyRefreshFunc` under a `context.WithTimeout` and/or skip in-flight refreshes when `stopCleanup` is signalled. Validate with `go test -race -run TestForwardSecrecy ./async/...`.
- [ ] **Iteration-pipeline sub-`MessageInterval` intervals round down to "run every tick"** — `iteration_pipelines.go:296-297` — logic — `dhtMod`/`friendMod` are computed by integer division `DHTInterval / MessageInterval`; if a caller sets `DHTInterval < MessageInterval` the result is `0`, so `runScheduledPipelines` (`:333` `dhtCounter >= dhtMod`) fires the DHT pass every tick. `MessageInterval` itself is guarded `> 0` (M-08), so **there is no division-by-zero panic**; the only effect is scheduling imprecision (the sub-interval pass runs at the most frequent possible cadence). **Remediation:** clamp `DHTInterval`/`FriendInterval` to at least `MessageInterval` in `NewIterationPipelines` after the existing M-08 guards, or document the rounding. Validate with `go test ./... -run TestIterationPipelines`.
- [ ] **`generateFriendID` can return the reserved `0` sentinel only after 2³²−1 live friends** — `toxcore_friends.go:116-124` — logic (theoretical) — the unbounded `for { id++ }` loop would wrap `uint32` to `0` and could return the reserved not-found sentinel, but only if ~4.3 billion friend IDs are simultaneously occupied, which is infeasible (memory-bound long before then). **Remediation:** optional guard returning an error when `id == math.MaxUint32`. No practical impact; documented for completeness.
- [ ] **4-byte VP8 RTP payload yields an empty frame fragment** — `av/video/rtp.go:495` — nil/boundary — a payload whose VP8 descriptor consumes all 4 bytes passes the `len(payload) < 4` check and produces an empty `frameData` slice that is appended to the frame buffer; accumulation is bounded by `maxPacketsPerFrame`, so the impact is a bounded number of empty fragments, not unbounded growth or a crash. **Remediation:** require `len(payload) >= 5` (descriptor + ≥1 byte bitstream). Validate with `go test -race ./av/video/...`.
- [ ] **`cloneReflectValue` shallow-aliases unexported reference-type struct fields** — `toxcore_friends.go:345-358` — data aliasing (acknowledged L-4) — the struct branch does `result.Set(value)` then deep-clones only settable (exported) fields; unexported pointer/slice/map fields therefore remain shared with the source. Already acknowledged at `toxcore_friends.go:283` (L-4) as low-impact because no public setter exposes such fields on the cloned types. **Remediation:** none required; retain the L-4 acknowledgement. Re-verify if new exported types with unexported reference fields are added to the cloned state.
- [ ] **Multi-device one-time-prekey single-use accounting is not enforced** — `crypto/multi_device.go:152-156` — API/behavioral contract — `AddDevice` selects `&dev.OneTimePreKeys[0]` and hard-codes `selectedOPKID = 1` with a `TODO`, so OPKs are not tracked or consumed as single-use in the simplified multi-device session. This weakens the one-time-prekey property (a replayed bundle reuses the same OPK). Cross-listed in `GAPS.md` (Gap 4). **Remediation:** extend `DeviceBundle` with per-OPK IDs and mark consumed OPKs; validate with a test asserting an OPK is not reused across two `AddDevice` calls.

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total functions | 1,365 |
| Total methods | 3,123 |
| Functions + methods analyzed | 4,488 |
| Functions above cyclomatic complexity 15 | 5 |
| Functions above cyclomatic complexity 10 | 11 |
| Functions > 50 lines | 38 |
| Avg cyclomatic complexity | 3.6 |
| Doc coverage (overall) | 93.7% |
| Duplication ratio | 0.66% (39 clone pairs, 630 lines) |
| Test pass rate | 35/35 packages with tests pass; 0 failures, 0 data races under `-race`; 24 packages have no test files |
| `go vet ./...` warnings | 0 |

## False Positives Considered and Rejected

| Candidate (machine-raised) | Reason Rejected |
|----------------------------|-----------------|
| CRITICAL: `crypto/sealed_sender.go:175` — `aead.Open` return value discarded with `_` | `plaintextIdentity` is `make([]byte, 32)`; `Open(plaintextIdentity[:0], …)` appends into the **same backing array** (cap 32), so the subsequent `copy(senderIdentity[:], plaintextIdentity)` reads the decrypted plaintext correctly. Idiomatic and safe. |
| HIGH/MEDIUM: `crypto/multi_device.go:235,153` — `&newList.Devices[i]` / `&dev.OneTimePreKeys[0]` aliasing | The pointers are confined to the local `newDevices` map within `UpdateDeviceList` and are dereferenced immediately by `AddDevice`→`X3DHInitiate` (value-copied); none are retained past the call. No use-after-free path. |
| HIGH: `async/manager.go:1262` — "remote DoS via 65535 keys" | Allocation is bounded **1:1** by the exact-length-validated packet (`verifyPreKeyPacketSize`), uint16 cap, and datagram MTU — no amplification. Reclassified to MEDIUM (missing upper bound only). |
| MEDIUM: `async/lifecycle.go` `stopLoop` double-close panic | `stopLoop` holds `mu.Lock()` across the `if !*running` check **and** `close()`; a second concurrent `Stop()` observes `running == false` and returns without closing. No double-close. |
| MEDIUM: `async/forward_secrecy.go` `Close()` deadlock on `closedMu` | `Close()` unlocks `closedMu` at `:263` before `cleanupWg.Wait()` at `:273`; refresh goroutines never re-acquire `closedMu`. No deadlock (residual slow-callback blocking retained as L-02). |
| MEDIUM: `async/prekeys.go:803` `ImportPreKeys` in-place filter corrupts caller | `clonePreKeyBundle` (`:375`) deep-copies via `cp.Keys = make(...)` + `clonePreKey`; the `cp.Keys[:0]` filter operates on the private copy's backing array (labeled M-17/M-ASYNC-2). Caller data untouched. |
| LOW: `async/manager.go:892` `preKeyReadyCh` lost-channel race | Channel creation uses check-then-act **under** `am.mutex`; a concurrent caller sees `exists == true` and does not overwrite. Correct. |
| CRITICAL ×3: `transport/version_negotiation.go:137`, `versioned_handshake.go:179/249` — "unbounded allocation DoS" | All exact-length-validated before `make()`; `ProtocolVersion` is `uint8`; allocations are 1:1 with received bytes. Not amplification/OOB. Retained as a single LOW hardening item. |
| HIGH: `transport/parser.go:229` `addrLen` unbounded | Guarded by `len(data) < currentOffset+addrLen+2` before the `make`/`copy`; bounded by packet. Retained as LOW (missing overlay-address sanity max). |
| MEDIUM: `iteration_pipelines.go` division-by-zero panic | `MessageInterval` is guarded `> 0` (M-08); `dhtMod == 0` only affects a `>=` comparison, not a modulo/division. No panic. Retained as L-03 (scheduling). |
| MEDIUM: `file/transfer.go:692` `RollbackChunk` underflow/corruption | A negative `Seek(-int64(n), SeekCurrent)` returns an error **before** any state change; the `Transferred` underflow is explicitly guarded (`if uint64(n) > t.Transferred { … = 0 }`); in normal flow `n` equals the just-written chunk. No corruption. |
| LOW: `file/transfer.go` `deserializeFileData` shares packet backing array | Uses `make([]byte, len(data)-12)` + `copy`; the returned slice does not alias `packet.Data`. |
| MEDIUM: `av/rtp/session.go:712` VP8 PictureID OOB | Outer guard ensures `len >= 3`; the `else` (where `>=4` is false ⇒ `len == 3`) accesses `payload[2]`, which is in-bounds. No OOB. |
| `capi/toxav_c.go:200` `recover()` swallows panic | Intentional and required at the cgo boundary so a bad C pointer cannot crash the process; documented. Correct pattern (also applied via `invoke_*_cb` bridges). |
| Prior `GAPS.md` Gap 2 (downgrade commitment dead) | Remediated: `PacketVersionCommitment` is now registered on the decrypted-handler map (`transport/noise_transport.go:261-264`) and bound to the handshake transcript hash (`:717-722`), not a local nonce. |
| Prior `GAPS.md` Gap 3 (`SelfGetConnectionStatus` hard-wired offline) | Remediated: `updateConnectionStatus()` exists (`toxcore_self.go:75`) and is invoked from `toxcore_lifecycle.go:24`. |
| Prior `GAPS.md` Gap 5 (first-use prekey TOFU poisoning) | Remediated: unknown-owner bundles are now quarantined in `pendingValidation` and never used until explicit out-of-band `ValidateAndRegisterBundleForPeer` (`async/prekey_dht.go:429-442`). |
| Prior `GAPS.md` Gap 6 (group replay sentinel collision) | Remediated: replaced in-band `^uint64(0)` sentinel with explicit `SeenFirstMessage bool` (`group/sender_key.go:41,365,441,459`). |
| Prior `GAPS.md` Gap 7 (DHT `Node` unsynchronized) | Remediated: `dht.Node` now has `mu sync.RWMutex` (F-DHT-L1) with locked accessors `IsActive`/`Update`/`RecordPing*` (`dht/node.go:97,136-208`). |
| `transport/upnp_client.go` SSRF | Mitigated upstream by `validateUPnPLocationURL` (M-06). |

## Remaining Scope (lower-priority; not deeply audited this pass)

| Package | Status | Notes |
|---------|--------|-------|
| `examples/*` (35+ demos) | Not deeply audited | Demonstration `main` programs; not part of the library API surface. Resume here for a fully exhaustive pass. |
| `cmd/gen-bootstrap-nodes` | Not deeply audited | Build-time tooling. |
| `testnet/*` | Not deeply audited | Test harness / internal client. |
| `simulation`, `real`, `factory` | Not deeply audited | Test/simulation and small wiring helpers. |
| `interfaces`, `limits` | Not deeply audited | Tiny utility packages (1–2 files). |
| `bootstrap`, `bootstrap/nodes` | Not deeply audited | Bootstrap node-list data and loader. |

No confirmed finding above LOW was produced for any core package on this pass after
false-positive filtering; the single MEDIUM is a defense-in-depth hardening item.
A subsequent pass should complete the lower-priority packages above to satisfy the
end-to-end "every package" goal.
