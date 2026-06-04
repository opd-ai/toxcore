# Implementation Gaps — 2026-06-04

This document records gaps between what `toxcore-go` **claims** (in `README.md`,
`SECURITY.md`, package GoDoc, and the libtoxcore-compatible C API contract) and what
the current code actually does. It complements `AUDIT.md`, which records concrete bug
findings with file/line references and severities.

**Re-verification note:** Several gaps documented in earlier passes were re-checked
against the current tree and found **remediated**, and are therefore *not* repeated
here:

- C-API friend and ToxAV callbacks now bridge into C correctly
  (`capi/toxcore_c.go:781,825,864`, `capi/toxav_c.go`).
- Noise downgrade-commitment is routed and transcript-bound
  (`transport/noise_transport.go`).
- `SelfGetConnectionStatus` is now driven by `updateConnectionStatus()` on bootstrap
  (`toxcore_self.go:75`, `toxcore_lifecycle.go`).
- Async key rotation invokes `ObfuscationManager.UpdateKeyPair` (`async/obfs.go`).
- First-use prekey poisoning is mitigated via `pendingValidation` quarantine
  (`async/prekey_dht.go`).
- Group replay sentinel replaced with explicit `SeenFirstMessage` (`group/sender_key.go`).
- `dht.Node` mutex + locked accessors added (F-DHT-L1, `dht/node.go:97,232`) — although
  four call sites still bypass `GetStatus()`; see `AUDIT.md` finding **M-01**.
- **Multi-device one-time pre-keys are now consumed single-use** via the `UsedOPKs`
  map (`crypto/multi_device.go:151-193`) — the prior "OPK reuse" gap is **closed**.

The gaps below are those that **remain open** in the current tree.

## Gap 1 — C API conference and file-transfer callbacks register but never fire

- **Stated Goal:** `README.md` advertises "C API Bindings — libtoxcore-compatible C
  function exports for toxcore and ToxAV." `capi/` exports
  `tox_callback_conference_message`, `tox_callback_conference_invite`,
  `tox_callback_file_recv`, `tox_callback_file_recv_chunk`, and
  `tox_callback_file_chunk_request`.
- **Current State:** Friend and ToxAV callbacks bridge into C correctly, but the
  **conference** message/invite handlers and the **file-recv** handler only store the
  C function pointer and log a placeholder rather than invoking it:
  `capi/toxcore_c.go:1190` ("Would need to connect this to toxcore's group message
  handler") and `capi/toxcore_c.go:1381` ("Would need to call C callback here"). The
  stored pointers are never called when the corresponding events occur.
- **Impact:** A C/C++ program linking the shared library registers conference and
  file-transfer handlers but receives no conference messages/invites and no
  file-transfer events. The conference and file surfaces of the C API are
  effectively receive-blind, so C clients relying on them cannot function — a
  documented feature is non-functional for C consumers.
- **Closing the Gap:** Add cgo bridge invokers mirroring the working
  `invoke_friend_*_cb` / `toxav_c.go` pattern: wire `OnFileRecv`/`OnFileRecvChunk`
  and the conference message/invite handlers to call each stored C function pointer
  with its registered user-data, and add a C round-trip test
  (`go build -buildmode=c-shared ./capi`).

## Gap 2 — C API conference title setting is unimplemented

- **Stated Goal:** The libtoxcore contract exposes `tox_conference_set_title(...)`;
  `capi/` exports a corresponding wrapper.
- **Current State:** The wrapper validates its arguments and then returns `0`
  (failure) unconditionally — "Conference title setting is not yet implemented in the
  Go API" (`capi/toxcore_c.go:1768`).
- **Impact:** C clients cannot set conference titles; the call always reports failure.
  Combined with Gap 1, the conference surface of the C API is only partially
  functional.
- **Closing the Gap:** Implement a `SetConferenceTitle` method on the Go
  group/conference type and have the C wrapper call it, returning `1` on success. Add
  a unit test asserting the title round-trips.

## Gap 3 — "Experimental / pending external audit" status is disclosed only in SECURITY.md

- **Stated Goal:** `SECURITY.md` → "External Audit Status" states that no third-party
  professional audit has been performed and that the library should be treated as
  **experimental** and unsuitable for high-stakes production use until one is complete
  (`SECURITY.md:94-101`).
- **Current State:** This caveat lives only in `SECURITY.md`. A search of `README.md`
  for "experimental"/"audit" finds **no** such notice; the feature list presents the
  cryptography and protocol feature set as ready-to-use at the point a developer first
  integrates the library.
- **Impact:** Integrators reading only `README.md` may deploy the library in a threat
  model it has not been independently validated for, assuming the breadth of crypto
  features implies external assurance.
- **Closing the Gap:** Surface the "experimental / pending third-party audit" notice
  in the `README.md` security/usage section (and ideally the top-level package GoDoc),
  linking to `SECURITY.md`. Documentation change only.

## Gap 4 — Nym (and Lokinet) transport is dial-only; inbound service hosting is unimplemented

- **Stated Goal:** `README.md` lists "Multi-Network Transport — … Lokinet `.loki`
  (dial-only), and Nym `.nym` (dial-only)." The dial-only qualifier is accurate, so
  this is a **scoping note** rather than a contradiction.
- **Current State:** Outbound `Dial`/`DialPacket` over Nym work via a SOCKS5 proxy to a
  local Nym client, but inbound service hosting returns `ErrNymNotImplemented`
  ("requires Nym SDK websocket client integration", `transport/nym_transport_impl.go:18,100`).
  The same dial-only limitation applies to Lokinet `.loki`.
- **Impact:** Low and consistent with documentation: applications cannot *host*
  reachable services over Nym/Lokinet; they can only initiate outbound connections.
  Listed for discoverability alongside the other gaps.
- **Closing the Gap:** Either integrate the Nym SDK websocket client to support inbound
  service hosting, or keep the documented dial-only scope and ensure the `README.md`
  transport table continues to clearly mark Nym/Lokinet as dial-only.

## Gap 5 — Multi-device X3DH assumes pre-converted Curve25519 keys

- **Stated Goal:** The async/X3DH and multi-device design implies that identity keys
  (Ed25519 in Tox) are converted to Curve25519 before Diffie-Hellman, as the GoDoc on
  `AddDevice` itself describes.
- **Current State:** `crypto/multi_device.go:171-173` documents that "in production,
  Ed25519 identity keys in `DeviceBundle` would be converted to Curve25519 using
  `DeriveX25519FromEd25519Seed` before X3DH initiation," but the simplified path
  assumes the supplied keys are already Curve25519 and performs no conversion or
  type check.
- **Impact:** Low today — no exported path feeds Ed25519-form keys into `AddDevice`.
  But a caller constructing a `DeviceBundle` from raw Tox (Ed25519) identity keys would
  silently establish sessions with the wrong key material, with no error. (Cross-listed
  as `AUDIT.md` finding **L-03**.)
- **Closing the Gap:** Either perform the documented `DeriveX25519FromEd25519Seed`
  conversion inside `AddDevice`, or document the Curve25519 precondition on the
  exported method and reject non-conforming input; add a unit test covering both key
  forms.
