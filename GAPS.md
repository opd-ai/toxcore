# Implementation Gaps — 2026-06-04

This document records gaps between what `toxcore-go` **claims** (README.md, ROADMAP.md,
SECURITY.md, package GoDoc, and the libtoxcore-compatible C API contract) and what the
current tree actually does. It complements `AUDIT.md`, which lists the same findings as a
severity-classified checklist. Report-only — no source code was modified.

**Re-verification note:** Several gaps recorded in earlier passes were re-checked against
the current tree and found **remediated**; they are *not* repeated as open gaps below:

- **ToxAV bitrate callbacks are now wired** — `SetAudioBitRateCallback`/`SetVideoBitRateCallback`
  forward into the implementation (`toxav.go:1403`, `toxav.go:1431`). The prior
  "callbacks never invoked" gap is closed.
- **Friend and ToxAV C-API callbacks bridge into C correctly**
  (`capi/toxcore_c.go:781,825,864`).
- **Group DHT query *response collection* is implemented** — `registerQuery` /
  `waitForGroupAnnouncement` / `handleGroupQueryResponse` (`dht/group_storage.go:62,307,429`).
  Only a stale doc comment remains (see Gap 6 / `AUDIT.md` L-02).
- **`file` package is integrated into `toxcore.Tox`** (`toxcore_file.go`); only the GoDoc
  in `file/doc.go` is stale (see Gap 5 / `AUDIT.md` L-01).
- **Multi-device one-time pre-keys are consumed single-use** via the `UsedOPKs` map
  (`crypto/multi_device.go:151-199`).

The gaps below are those that **remain open** in the current tree, ordered by the
tiebreaker priority (partially-wired features → stubs → stale/interface → documented scope).

---

## Gap 1 — C-API conference message/invite callbacks register but never fire

- **Intended Behavior**: README advertises "C API Bindings — libtoxcore-compatible C
  function exports." `capi/` exports `tox_callback_conference_message` and
  `tox_callback_conference_invite`; per the Tox contract these must invoke the registered
  C function pointer whenever a conference message/invite arrives.
- **Current State**: Both registrars store the C pointer and emit a Debug log only
  ("Would need to connect this to toxcore's group message handler",
  `capi/toxcore_c.go:1190`); no internal Go handler is bridged, so the pointer is never
  called. `capi/toxcore_c.go:1184` (message), `:1197` (invite).
- **Blocked Goal**: libtoxcore-compatible C API — conference receive surface is
  non-functional for C/C++ consumers.
- **Implementation Path**: Add a cgo bridge helper mirroring `invoke_friend_message_cb`
  (`capi/toxcore_c.go:51`, used at `:825`). Inside each registrar, register an internal
  conference message/invite handler on the `toxcore.Tox` instance and, when it fires,
  call the stored C pointer with its user-data (look up via the existing
  `groupMessageCallbacks` / `groupInviteCallbacks` maps).
- **Dependencies**: Requires a Go-level conference message/invite event hook on
  `toxcore.Tox` (verify one exists; `toxcore_conference.go` / `group` provide the
  underlying events).
- **Effort**: medium

## Gap 2 — C-API file-transfer receive callbacks never invoke the C pointer

- **Intended Behavior**: `tox_callback_file_recv`, `tox_callback_file_recv_chunk`, and
  `tox_callback_file_chunk_request` must invoke their registered C function pointer on the
  corresponding file-transfer event.
- **Current State**: Each correctly wires a Go-side handler (`OnFileRecv`,
  `OnFileRecvChunk`, `OnFileChunkRequest`) but the handler body only logs
  ("Would need to call C callback here", `capi/toxcore_c.go:1381`) and never calls the
  stored C pointer. `capi/toxcore_c.go:1374`, `:1397`, `:1418`.
- **Blocked Goal**: libtoxcore-compatible file-transfer API — C clients are receive-blind
  for file events even though the Go core delivers them.
- **Implementation Path**: Add cgo C helpers `invoke_file_recv_cb`,
  `invoke_file_recv_chunk_cb`, `invoke_file_chunk_request_cb` (following the
  `invoke_friend_*_cb` pattern) and call them from inside each registered handler, passing
  the registered user-data and converting Go args (e.g. `filename`, `data`) to C types.
- **Dependencies**: None beyond the existing `OnFileRecv*` hooks, which are already wired.
- **Effort**: medium

## Gap 3 — C-API `tox_conference_set_title` is an unconditional-failure stub

- **Intended Behavior**: `tox_conference_set_title(...)` sets a conference's title and
  returns success; it is the write counterpart to the working `tox_conference_get_title`.
- **Current State**: Validates arguments then returns `0` (failure) unconditionally —
  "Conference title setting is not yet implemented in the Go API"
  (`capi/toxcore_c.go:1768`).
- **Blocked Goal**: Symmetric read/write of conference titles in the C API; combined with
  Gap 1, the conference surface is only partially functional.
- **Implementation Path**: The Go layer already supports renaming via
  `group.Chat.SetName` (`group/chat.go:1330`), and the wrapper already obtains the
  conference handle through `ValidateConferenceAccess`. Call
  `conference.SetName(string(unsafe.Slice(title, length)))` and return `1` on success,
  `0` on error.
- **Dependencies**: None (`SetName` exists and broadcasts the change).
- **Effort**: small

## Gap 4 — Self user-status (None/Away/Busy) is not tracked, so the C-API accessors are no-ops

- **Intended Behavior**: `tox_self_set_status` / `tox_self_get_status` set and report the
  local user's presence status (0=None, 1=Away, 2=Busy) per the libtoxcore contract.
- **Current State**: `tox_self_get_status` always returns `0` (`capi/toxcore_c.go:1463`)
  and `tox_self_set_status` is a logged no-op (`capi/toxcore_c.go:1472`). The root
  `toxcore.Tox` only tracks the status *message* string (`toxcore.go:355` `selfStatusMsg`,
  `toxcore_self.go:149` `SelfSetStatusMessage`), not the status enum.
- **Blocked Goal**: libtoxcore `tox_self_set_status`/`tox_self_get_status` contract.
- **Implementation Path**: Add a `selfStatus UserStatus` field plus
  `SelfSetStatus(UserStatus)` / `SelfGetStatus() UserStatus` methods on `toxcore.Tox`
  (mirroring the existing status-message accessors and broadcast helper), then delegate
  from the two C wrappers. Optionally broadcast the status to connected friends as
  `SelfSetStatusMessage` does.
- **Dependencies**: None; a small core addition unblocks the C wrappers.
- **Effort**: small

## Gap 5 — Stale GoDoc: `file` package claims it is not integrated into `Tox`

- **Intended Behavior**: Package documentation should reflect actual wiring so contributors
  trust it.
- **Current State**: `file/doc.go:173` lists "⚠️ Not yet integrated into main Tox struct
  (standalone usage)", but the package is integrated: `toxcore.go:53` imports it,
  `toxcore_file.go:95` calls `file.NewTransfer`, and `toxcore_file.go:313` exposes
  `FileManager()`.
- **Blocked Goal**: None functionally; the doc misleads contributors about wiring status.
- **Implementation Path**: Update the "Integration Status" block in `file/doc.go` to mark
  Tox-struct integration as done (via `toxcore_file.go`). Documentation-only.
- **Dependencies**: None.
- **Effort**: small

## Gap 6 — Stale "not yet implemented" sentinel/doc for group DHT queries

- **Intended Behavior**: Error names and docs should describe current behavior.
- **Current State**: `dht/group_storage.go:16-20` defines `ErrGroupDHTNotImplemented` with
  the comment "Response collection from remote DHT nodes is not yet implemented", but
  response collection is fully wired (`registerQuery`/`waitForGroupAnnouncement` at
  `:62`/`:307`, and `handleGroupQueryResponse`→`HandleGroupQueryResponse`→`notifyResponse`
  at `:429`, `dht/routing.go:611`). The sentinel now only fires defensively when
  `groupStorage == nil`.
- **Blocked Goal**: None functionally; the doc misrepresents implemented behavior.
- **Implementation Path**: Reword the sentinel doc to "returned only when group storage is
  unavailable", or rename to reflect the nil-storage case. Documentation/identifier change.
- **Dependencies**: None.
- **Effort**: small

## Gap 7 — Stale comment on `tox_conference_offline_peer_count`

- **Intended Behavior**: Comments should match the code they describe.
- **Current State**: `capi/toxcore_c.go:1863` says "currently returns 0 as offline peer
  tracking is not fully implemented", but the body iterates `conference.Peers` and counts
  peers with `Connection == 0` (`:1877-1883`).
- **Blocked Goal**: None; comment contradicts working code.
- **Implementation Path**: Delete the stale sentence. Documentation-only.
- **Dependencies**: None.
- **Effort**: small

## Gap 8 — Nym/Lokinet inbound `Listen` (service hosting) is unimplemented (documented scope)

- **Intended Behavior**: README lists Lokinet `.loki` and Nym `.nym` transports; ROADMAP
  qualifies them as "Dial only (SDK immature)".
- **Current State**: Outbound `Dial`/`DialPacket` work via SOCKS5 to a local client, but
  inbound hosting returns `ErrNymNotImplemented` (`transport/nym_transport_impl.go:18,100`)
  and Lokinet `Listen` returns an explicit error (`transport/lokinet_transport_impl.go:91`).
  Covered by tests (`transport/lokinet_transport_test.go:64`).
- **Blocked Goal**: None — this matches the documented dial-only scope; recorded here for
  discoverability, classified LOW.
- **Implementation Path**: Keep the documented dial-only scope, or integrate the Nym SDK
  websocket client / Lokinet SNApp config for inbound hosting when upstream SDKs mature.
- **Dependencies**: External (Nym/Lokinet SDK maturity).
- **Effort**: large

## Gap 9 — `options.go` is an empty scaffolded file

- **Intended Behavior**: Source files should contain declarations or be removed.
- **Current State**: `options.go` contains only `package toxcore` — no types or functions.
  Tox options live elsewhere (e.g. `toxcore_defaults.go`).
- **Blocked Goal**: None; minor clutter / confusion.
- **Implementation Path**: Remove the file, or relocate an options/config type into it.
  Confirm nothing references it (`grep -rn options.go`), then `go build ./...`.
- **Dependencies**: None.
- **Effort**: small

## Gap 10 — `AddDevice` documents a Curve25519 precondition but does not enforce it

- **Intended Behavior**: Multi-device X3DH requires Curve25519 key material; identity keys
  (Ed25519 in Tox) must be converted via `DeriveX25519FromEd25519Seed` first.
- **Current State**: `crypto/multi_device.go:177-179` documents the precondition in a
  comment but performs no type check or conversion; a caller supplying raw Ed25519 keys
  would silently establish a session with wrong key material and no error.
- **Blocked Goal**: None today (no exported path feeds Ed25519-form keys here), but it is a
  latent correctness/security foot-gun.
- **Implementation Path**: Either perform `DeriveX25519FromEd25519Seed` conversion inside
  `AddDevice`, or validate/reject non-conforming input with an explicit error; add a unit
  test covering both key forms.
- **Dependencies**: None (`DeriveX25519FromEd25519Seed` already exists in `crypto`).
- **Effort**: small

---

## Summary

| Gap | Area | Severity (AUDIT.md) | Effort |
|-----|------|---------------------|--------|
| 1 | C-API conference callbacks not fired | HIGH (H-01) | medium |
| 2 | C-API file callbacks not fired | HIGH (H-02) | medium |
| 3 | C-API `set_title` stub | MEDIUM (M-01) | small |
| 4 | Self-status not tracked | MEDIUM (M-02) | small |
| 5 | `file/doc.go` stale | LOW (L-01) | small |
| 6 | Group DHT stale sentinel/doc | LOW (L-02) | small |
| 7 | `offline_peer_count` stale comment | LOW (L-03) | small |
| 8 | Nym/Lokinet dial-only (documented) | LOW (L-04) | large |
| 9 | Empty `options.go` | LOW (L-05) | small |
| 10 | `AddDevice` unenforced precondition | LOW (L-06) | small |

The open gaps are concentrated in the **optional `capi` C-binding layer** (Gaps 1–4, 7)
plus **stale documentation** (Gaps 5, 6) and minor cleanup (Gaps 9, 10). The pure-Go core
— the project's primary stated purpose — has no open implementation gaps above the LOW
threshold. Closing Gaps 1–4 would make the `capi` conference, file-transfer, and
self-status surfaces fully libtoxcore-compatible.
