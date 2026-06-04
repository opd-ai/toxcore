# IMPLEMENTATION GAP AUDIT — 2026-06-04

> Scope: implementation **gap discovery** audit (stubs, TODOs, incomplete code paths,
> dead code, unreachable features, partially-wired components). Report-only — no source
> code was modified. Findings are evaluated against the project's **own stated goals**
> (README.md, ROADMAP.md, SECURITY.md, package GoDoc, and the libtoxcore-compatible C
> API contract), not aspirational features.

## Project Architecture Overview

`toxcore-go` is a **pure-Go implementation of the Tox P2P encrypted messaging protocol**
(README.md, doc.go). Its stated goals (ROADMAP.md "Goal-Achievement Summary") are a
complete Tox stack with DHT routing, friend management, 1:1 messaging, file transfers,
group chat, ToxAV audio/video calling, multi-network transport (IPv4/IPv6, Tor, I2P,
Lokinet, Nym), Noise-IK handshakes, epoch-based forward secrecy, identity obfuscation,
and **libtoxcore-compatible C API bindings**.

Package responsibilities (27 packages, 261 files, 46,340 LOC per `go-stats-generator`):

| Layer | Packages | Responsibility |
|-------|----------|----------------|
| Core facade | `toxcore` (root) | Public Go API: lifecycle, friends, messaging, file, conference, self |
| Transport | `transport`, `toxnet`, `transport/internal/addressing` | UDP/TCP, Noise-IK, Tor/I2P/Lokinet/Nym, version negotiation |
| DHT | `dht`, `bootstrap`, `bootstrap/nodes` | Kademlia routing, bootstrap, group/relay storage |
| Crypto | `crypto`, `noise`, `ratchet` | KeyPair, X3DH/PQXDH, sealed sender, Noise, double ratchet |
| Async | `async` | Offline store-and-forward, pre-keys, forward secrecy, erasure coding |
| Messaging/social | `messaging`, `friend`, `group` | Message types, friend manager, conference/group chat |
| Media | `av`, `av/audio`, `av/video`, `av/rtp` | ToxAV: Opus audio, VP8 video, RTP |
| File | `file` | File transfer manager |
| C bindings | `capi` | cgo exports (`tox_*`, `toxav_*`) |
| Support | `interfaces`, `factory`, `limits`, `real`, `simulation`, `testnet` | Abstractions, DI, limits, test infra |

The project is **mature and substantially complete**: `go build ./...` and `go vet ./...`
are both clean; documentation coverage is 93.7%; `go-stats-generator` reports 0 dead-code
lines, 0 stale annotations, and 0 raw TODO/FIXME/HACK/XXX comments in non-test code. The
implementation gaps that remain are concentrated almost entirely in the **optional `capi`
C-binding layer** and in a small number of **stale documentation comments**.

## Gap Summary

| Category | Count | Critical | High | Medium | Low |
|----------|-------|----------|------|--------|-----|
| Stubs/TODOs | 3 | 0 | 0 | 2 | 1 |
| Dead Code | 1 | 0 | 0 | 0 | 1 |
| Partially Wired | 2 | 0 | 2 | 0 | 0 |
| Interface Gaps | 4 | 0 | 0 | 0 | 4 |
| Dependency Gaps | 0 | 0 | 0 | 0 | 0 |
| **Total** | **10** | **0** | **2** | **2** | **6** |

No CRITICAL gaps: the project's core stated purpose — a pure-Go Tox library with DHT,
messaging, files, groups, ToxAV, multi-network transport, and forward secrecy — is
implemented and exercised on the main execution path. The two HIGH findings affect the
**optional C API binding** (`capi/`), which README lists as a feature but which is not on
the pure-Go core path.

## Implementation Completeness by Package

Counts are total functions+methods reported by `go-stats-generator` (`--skip-tests`).
"Gaps" counts confirmed open findings located in that package.

| Package | Functions/Methods | Structs | Stubs/Partial | Dead/Stale | Status |
|---------|-------------------|---------|---------------|------------|--------|
| `capi` | 376 | 39 | 3 (H-01,H-02,M-01) | 1 (L-03) | Partial — callbacks/title gaps |
| `toxcore` (root) | 614 | 33 | M-02 (self-status) | 0 | Complete core |
| `transport` | 865 | 127 | 0 | 1 (L-04 documented) | Complete (Nym/Loki dial-only) |
| `dht` | 447 | 54 | 0 | 1 (L-02 stale sentinel) | Complete |
| `async` | 550 | 62 | 0 | 0 | Complete |
| `av`/`audio`/`video`/`rtp` | 621 | 78 | 0 | 0 | Complete |
| `crypto` | 138 | 23 | 0 | 1 (L-06 unenforced precond.) | Complete |
| `noise`/`ratchet` | 110 | 15 | 0 | 0 | Complete |
| `group` | 141 | 38 | 0 | 0 | Complete |
| `file` | 81 | 13 | 0 | 1 (L-01 stale doc) | Integrated |
| `friend`/`messaging` | 168 | 36 | 0 | 0 | Complete |
| `toxnet` | 177 | 12 | 0 | 0 | Complete |
| root file `options.go` | 0 | 0 | 0 | 1 (L-05 empty file) | Empty scaffold |

## Findings

### CRITICAL
- _None._ No core stated-purpose feature is a stub on a critical execution path.

### HIGH
- [ ] H-01 — Conference C-API callbacks register but never fire — `capi/toxcore_c.go:1184` (`tox_callback_conference_message`), `capi/toxcore_c.go:1197` (`tox_callback_conference_invite`) — the registered C function pointer is stored and a Debug line is logged ("Would need to connect this to toxcore's group message handler", line 1190), but no Go event handler is bridged to invoke the C pointer — **blocks** README goal "C API Bindings — libtoxcore-compatible C function exports": C clients registering conference handlers receive no conference messages or invites — **Remediation:** add a cgo bridge helper (mirroring `invoke_friend_message_cb` at `capi/toxcore_c.go:51`/`:825`), register an internal conference message/invite handler on the `toxcore.Tox` instance inside the callback registrar, and invoke the stored C pointer with its user-data when the event fires. Validate with `go build ./capi/...` and a C round-trip test asserting the callback executes. _Blocked in this pass: toxcore currently exposes no conference receive/invite callback hooks to wire from capi without broader API-path additions._

- [x] H-02 — File-transfer C-API receive callbacks register but never invoke the C pointer — `capi/toxcore_c.go:1374` (`tox_callback_file_recv`), `:1397` (`tox_callback_file_recv_chunk`), `:1418` (`tox_callback_file_chunk_request`) — each correctly wires a Go-side `OnFileRecv*`/`OnFileChunkRequest` handler, but the handler body only logs ("Would need to call C callback here", line 1381) and never calls the stored C function pointer — **blocks** the libtoxcore-compatible file-transfer surface: C clients are receive-blind for file events even though the Go core delivers them — **Remediation:** add cgo `invoke_file_recv_cb` / `invoke_file_recv_chunk_cb` / `invoke_file_chunk_request_cb` C helpers and call them from inside each registered handler with the stored user-data, following the working `invoke_friend_*_cb` pattern. Validate with `go build ./capi/...` plus a C round-trip test that triggers a file receive and asserts the C callback runs.

### MEDIUM
- [x] M-01 — `tox_conference_set_title` is a stub that always reports failure — `capi/toxcore_c.go:1768` — validates arguments then returns `0` unconditionally ("Conference title setting is not yet implemented in the Go API"), even though the Go layer already supports renaming via `group.Chat.SetName` (`group/chat.go:1330`) and `tox_conference_get_title` reads `conference.Name` (`capi/toxcore_c.go:1788`) — **blocks** symmetric read/write of conference titles in the C API — **Remediation:** call `conference.SetName(string(title[:length]))` (the conference handle is already obtained via `ValidateConferenceAccess`) and return `1` on success, `0` on error. Validate with a unit test asserting `set_title` followed by `get_title` round-trips.

- [x] M-02 — Self user-status (None/Away/Busy) is not tracked by the Go core, so the C-API accessors are no-ops — `capi/toxcore_c.go:1463` (`tox_self_get_status` always returns `0`), `capi/toxcore_c.go:1472` (`tox_self_set_status` is a logged no-op) — the root `toxcore.Tox` struct only tracks the status *message* (`toxcore.go:355` `selfStatusMsg`, `toxcore_self.go:149`), not the status enum — **blocks** the libtoxcore `tox_self_set_status`/`tox_self_get_status` contract — **Remediation:** add a `selfStatus` field plus `SelfSetStatus(UserStatus)`/`SelfGetStatus()` methods on `toxcore.Tox` (mirroring `SelfSetStatusMessage`/`SelfGetStatusMessage`), then have the two C wrappers delegate to them. Validate with a unit test asserting set→get round-trips across the three enum values.

### LOW
- [x] L-01 — Stale GoDoc claims the `file` package is not integrated — `file/doc.go:173` ("⚠️ Not yet integrated into main Tox struct (standalone usage)") — the package **is** integrated: `toxcore.go:53` imports it and `toxcore_file.go` uses `file.NewTransfer` (`:95`) and exposes `FileManager()` (`:313`) — misleads contributors about wiring status — **Remediation:** update the "Integration Status" block in `file/doc.go` to reflect that file transfer is wired into `toxcore.Tox` via `toxcore_file.go`. Validate by re-reading the doc against `toxcore_file.go`.

- [x] L-02 — Stale "not yet implemented" sentinel and doc for group DHT queries — `dht/group_storage.go:16-20` (`ErrGroupDHTNotImplemented` + comment "Response collection from remote DHT nodes is not yet implemented") — response collection **is** implemented: `registerQuery`/`waitForGroupAnnouncement` (`dht/group_storage.go:62`,`:307`) and the `PacketGroupQueryResponse` handler `handleGroupQueryResponse` → `HandleGroupQueryResponse` → `notifyResponse` (`dht/group_storage.go:429`, `dht/routing.go:611`) deliver responses to pending channels. The sentinel now only fires defensively when `groupStorage == nil` — the comment misrepresents current behavior — **Remediation:** reword the sentinel's doc to "returned only when group storage is unavailable" (the network query path is implemented), or rename the sentinel to reflect the nil-storage case. Validate by tracing `queryNetwork` in `dht/group_storage.go`.

- [x] L-03 — Stale comment on `tox_conference_offline_peer_count` — `capi/toxcore_c.go:1863` ("This implementation currently returns 0 as offline peer tracking is not fully implemented") — the body that follows actually iterates `conference.Peers` and counts peers with `Connection == 0` (`capi/toxcore_c.go:1877-1883`), so it does **not** unconditionally return 0 — comment contradicts the code — **Remediation:** delete the stale sentence in the doc comment. Documentation change only.

- [x] L-04 — Inbound service hosting (`Listen`) is unimplemented for Nym and Lokinet — `transport/nym_transport_impl.go:18,100` (`ErrNymNotImplemented`), `transport/lokinet_transport_impl.go:91` ("Lokinet SNApp hosting not supported via SOCKS5") — this is a **documented, intentional scope**: ROADMAP.md marks Lokinet/Nym as "⚠️ Dial only (SDK immature)" and README labels them "dial-only"; `Dial`/`DialPacket` work and the limitation is covered by tests (`transport/lokinet_transport_test.go:64`) — tracked, not an undiscovered gap — **Remediation:** keep the documented dial-only scope, or integrate the Nym SDK websocket client / Lokinet SNApp config for inbound hosting when the upstream SDKs mature. Validate that README/ROADMAP continue to mark these dial-only.

- [x] L-05 — `options.go` is an empty scaffolded file — `options.go:1` contains only `package toxcore` with no declarations — appears to be a placeholder for an options/config surface that was never filled in (Tox options are instead defined elsewhere, e.g. `toxcore_defaults.go`) — harmless but adds confusion — **Remediation:** either remove `options.go` or move the relevant options type into it; confirm no build-tag or generator references it first (`grep -rn options.go`). Validate with `go build ./...`.

- [ ] L-06 — `AddDevice` documents a Curve25519 precondition but does not enforce it — `crypto/multi_device.go:177-179` states "DeviceBundle.IdentityPublic MUST already be in Curve25519 format … use DeriveX25519FromEd25519Seed" but performs no type check or conversion before X3DH — a caller passing raw Ed25519 Tox identity keys would silently derive wrong key material — low risk today (no exported path feeds Ed25519 keys here) — **Remediation:** either call `DeriveX25519FromEd25519Seed` inside `AddDevice`, or validate/reject non-conforming input with an explicit error; add a unit test covering both key forms. Validate with `go test ./crypto/...`.

## False Positives Considered and Rejected

| Candidate Finding | Reason Rejected |
|-------------------|-----------------|
| `async/storage_limits_unix.go` `getWindowsDiskSpace` returns an error stub | Intentional cross-platform build-tag shim (`//go:build !windows`); the real implementation lives in the Windows-tagged file. Fulfils its documented purpose. |
| `transport/batch_receive_linux.go` `DefaultBatchSize` "placeholder for API compatibility" | Documented, intentional: Go's `x/sys/unix` does not expose `recvmmsg`; the constant exists for API symmetry and the file implements real dynamic buffer tuning. Not incomplete against stated scope. |
| `examples/audio_streaming_demo/main.go` `Send` "not implemented in demo" | Demo mock transport, explicitly labelled "in demo"; example code, not a library code path. |
| Numerous `Deprecated:` methods (e.g. `GetStats`, `GetStatus`) returning legacy shapes | Intentional backward-compat shims that delegate to typed replacements (`GetTypedStats`/`GetStatusTyped`); deprecation is a maintained convention, not a gap. |
| `av/video/rtp.go:290` returns `nil` "Frame not yet complete" | Correct reassembly semantics for fragmented RTP frames — returns nil until all fragments arrive. Documented behavior, not a stub. |
| Interfaces with a single concrete implementation (e.g. transport abstractions) | Used for testability/mocking and dependency injection (`factory`, `mocks_test.go`); the abstraction has current value. Not premature. |
| Exported functions with no internal callers in `capi`/`toxcore` | Public/C-API surface intended for external consumers; presence without internal callers is expected for a library. |

## Validation Performed

```text
go build ./...   → clean (exit 0)
go vet ./...     → clean (exit 0)
go-stats-generator analyze . --skip-tests
   → 46,340 LOC, 27 packages, 93.7% doc coverage,
     0 dead-code lines, 0 stale annotations, 0 TODO/FIXME/HACK/XXX in non-test code
```

See `GAPS.md` for the detailed gap-by-gap implementation roadmap.
