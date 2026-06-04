# Implementation Gaps — 2026-06-04

This document records gaps between what `toxcore-go` **claims** (in `README.md`,
`SECURITY.md`, package GoDoc, and the libtoxcore-compatible C API contract) and
what the current code actually does. It complements `AUDIT.md`, which records
concrete bug findings with file/line references and severities.

**Scope note:** A prior `GAPS.md` documented eight gaps (C-API callbacks not
firing, Noise downgrade-commitment dead code, `SelfGetConnectionStatus` hard-wired
offline, async key-rotation breaking receipt, first-use prekey poisoning, group
replay-sentinel collision, DHT concurrency-contract violations, and missing
experimental-status disclosure). All were **re-verified against the current tree
during this pass and found remediated except where noted below**:

- Gap 2 (downgrade commitment): fixed — commitment is now routed to the decrypted
  handler map and bound to the handshake transcript hash
  (`transport/noise_transport.go:261-264,717-722`).
- Gap 3 (`SelfGetConnectionStatus`): fixed — `updateConnectionStatus()` is now
  called on bootstrap (`toxcore_self.go:75`, `toxcore_lifecycle.go:24`).
- Gap 4 (async key rotation): fixed — `ObfuscationManager.UpdateKeyPair`
  (`async/obfs.go:72`) is now invoked from both rotation paths
  (`async/key_rotation_client.go:80,142`).
- Gap 5 (first-use prekey poisoning): fixed — unknown-owner bundles are quarantined
  in `pendingValidation` and never used until explicit out-of-band validation
  (`async/prekey_dht.go:429-442`).
- Gap 6 (group replay sentinel): fixed — replaced with an explicit
  `SeenFirstMessage bool` (`group/sender_key.go:41,365,441,459`).
- Gap 7 (DHT concurrency): fixed — `dht.Node` now has a mutex and locked accessors
  (F-DHT-L1, `dht/node.go:97,136-208`).

The gaps below are those that **remain open** in the current tree.

## Gap 1 — C API conference and file-transfer callbacks register but never fire

- **Stated Goal:** `README.md` advertises "C API Bindings — libtoxcore-compatible C
  function exports for toxcore and ToxAV", and `capi/` exports
  `tox_callback_conference_message`, `tox_callback_conference_invite`,
  `tox_callback_file_recv`, `tox_callback_file_recv_chunk`, and
  `tox_callback_file_chunk_request`.
- **Current State:** The friend callbacks now correctly bridge into C via
  `C.invoke_friend_request_cb` / `invoke_friend_message_cb` /
  `invoke_friend_connection_status_cb` (`capi/toxcore_c.go:781,825,864`), and all
  ToxAV callbacks bridge (`capi/toxav_c.go`). However the **conference** message/
  invite callbacks only register the pointer and log at Debug ("Would need to
  connect this to toxcore's group message handler", `capi/toxcore_c.go:1191`), and
  the **file-recv** callback likewise only logs ("Would need to call C callback
  here", `capi/toxcore_c.go:1381`). The stored C function pointers are never
  invoked when the corresponding events occur.
- **Impact:** A C/C++ program linking the shared library receives no conference
  messages/invites and no file-transfer events, even though it registered handlers.
  The toxcore-side conference and file-transfer surfaces of the C API are
  effectively receive-blind, so C clients relying on them cannot function.
- **Closing the Gap:** Add cgo bridge invokers mirroring the working
  `invoke_friend_*_cb` / `toxav_c.go` pattern — wire `OnFileRecv` and the
  conference message/invite handlers to call each stored C function pointer with
  its registered user-data — and add a C round-trip test
  (`go build -buildmode=c-shared ./capi`).

## Gap 2 — C API conference title setting is unimplemented

- **Stated Goal:** The libtoxcore contract exposes
  `tox_conference_set_title(...)` to set a conference/group title; `capi/` exports a
  corresponding wrapper.
- **Current State:** The wrapper validates its arguments and then returns `0`
  (failure) unconditionally — "Conference title setting is not yet implemented in
  the Go API" (`capi/toxcore_c.go:1768`).
- **Impact:** C clients cannot set conference titles; the call always reports
  failure. Combined with Gap 1, the conference surface of the C API is only
  partially functional.
- **Closing the Gap:** Implement a `SetConferenceTitle` method on the Go group/
  conference type and have the C wrapper call it, returning `1` on success. Add a
  unit test asserting the title round-trips.

## Gap 3 — One-time pre-keys are not consumed as single-use in multi-device sessions

- **Stated Goal:** The async/X3DH design (and `README.md` "forward secrecy via
  one-time pre-keys") relies on one-time pre-keys (OPKs) being used at most once, so
  that compromise of long-term keys cannot retroactively decrypt past sessions that
  consumed a now-deleted OPK.
- **Current State:** In the multi-device path, `AddDevice` selects
  `&dev.OneTimePreKeys[0]` and hard-codes `selectedOPKID = 1` with an explicit
  `TODO`; there is no per-OPK ID tracking and no consumption/deletion of the used
  OPK (`crypto/multi_device.go:152-156`). A comment notes the simplified session
  assumes keys are already Curve25519 and defers single-use accounting.
- **Impact:** The one-time property is not enforced for multi-device sessions: the
  same OPK can be reused across device additions or a replayed bundle, weakening the
  forward-secrecy guarantee the feature advertises. (Cross-listed as `AUDIT.md`
  LOW finding.)
- **Closing the Gap:** Extend `DeviceBundle` with per-OPK IDs, set `selectedOPKID`
  to the real ID, and mark/remove the OPK after use; add a test asserting an OPK is
  not selected twice across consecutive `AddDevice` calls.

## Gap 4 — "Experimental / pending external audit" status is disclosed only in SECURITY.md

- **Stated Goal:** `SECURITY.md` → "External Audit Status" states that no
  third-party professional audit has been performed and that the library should be
  treated as **experimental** and unsuitable for high-stakes production use until
  one is complete (`SECURITY.md:94-101`).
- **Current State:** This caveat lives only in `SECURITY.md`. The `README.md`
  feature list presents the rich cryptography and protocol feature set as
  ready-to-use, with no experimental/un-audited disclaimer at the point where a
  developer first integrates the library (a search of `README.md` for
  "experimental"/"audit" finds no such notice).
- **Impact:** Integrators reading only the `README.md` may deploy the library in a
  threat model it has not been independently validated for, assuming the breadth of
  crypto features implies external assurance.
- **Closing the Gap:** Surface the "experimental / pending third-party audit"
  notice in the `README.md` security/usage section (and ideally the top-level
  package GoDoc), linking to `SECURITY.md`. Documentation change only.

## Gap 5 — Nym transport is dial-only (service hosting unimplemented)

- **Stated Goal:** `README.md` lists "Multi-Network Transport — … Nym `.nym`
  (dial-only)" as a feature. The dial-only qualifier is accurate, so this is a
  **minor scoping note** rather than a contradiction.
- **Current State:** Outbound `Dial`/`DialPacket` over Nym are implemented via a
  SOCKS5 proxy to a local Nym client (`transport/nym_transport_impl.go:25-28`), but
  inbound service hosting returns `ErrNymNotImplemented` ("requires Nym SDK
  websocket client integration", `transport/nym_transport_impl.go:18,100`). The same
  dial-only limitation applies to Lokinet `.loki`, which `README.md` also documents
  as dial-only.
- **Impact:** Low and consistent with documentation: applications cannot *host*
  reachable services over Nym; they can only initiate outbound connections. Listed
  here for completeness so the limitation is discoverable alongside the other gaps.
- **Closing the Gap:** Either integrate the Nym SDK websocket client to support
  inbound service hosting, or keep the documented dial-only scope and ensure the
  `README.md` transport table continues to clearly mark Nym/Lokinet as dial-only.
