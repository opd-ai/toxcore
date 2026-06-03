# Implementation Gaps — 2026-06-03

This document records gaps between what `toxcore-go` claims (README, GoDoc, feature list)
and what the code actually does. Each gap is confirmed against specific files and line
numbers in the current tree and is ordered by user impact.

**Re-verification note:** A prior `GAPS.md` documented six gaps (ToxAV answerer media,
forward-secrecy refresh accounting, Double Ratchet decrypt-then-commit, Nym dial-only, VP8
P-frame decode, UPnP SSRF). Four of the six (answerer media, forward secrecy, ratchet,
UPnP SSRF) were independently re-checked and found **remediated** in the current code — see
`AUDIT.md` → "Previously Reported — Re-verified as Fixed". The gaps below are those that
remain open in the current tree.

## Gap 1 — `RequireEncryption()` advertises packet dropping but delivers plaintext

- **Stated Goal:** `ToxPacketConn.RequireEncryption` GoDoc
  (`toxnet/packet_conn.go:532-535`): *"enables strict encryption mode: packets from unknown
  peers and packets that fail decryption are dropped instead of passed through as plaintext.
  This prevents a caller from inadvertently accepting unauthenticated data when it believes
  the connection is encrypted (M-05 fix)."*
- **Current State:** The decryption helper honours strict mode and returns an error for
  unknown peers (`toxnet/packet_conn.go:605-610`) and AEAD failures (`:621-626`), but the
  sole caller `createPacketWithAddr` discards that error and falls back to the original,
  unauthenticated bytes (`:184-189`). `processIncomingPacket` then unconditionally enqueues
  the packet (`:146-147`), so `ReadFrom()` returns it to the application. The documented
  "drop" never happens; strict mode behaves identically to default mixed mode. The feature
  also has no test coverage.
- **Impact:** High (security). Any application that opts into `RequireEncryption()` to refuse
  unauthenticated traffic still receives forged/plaintext datagrams as if they were
  authenticated — a silent authentication bypass on the `toxnet` `net.PacketConn` surface.
- **Closing the Gap:** Make `createPacketWithAddr` propagate the strict-mode decryption error
  (e.g. return an `ok bool`) so `processIncomingPacket` skips `enqueuePacket` when
  `encryptionRequired` is set and decryption fails; retain pass-through only for mixed mode.
  Add a regression test asserting a non-decryptable datagram is not returned by `ReadFrom`.
  (Tracked as `AUDIT.md` finding **H-1**.)

## Gap 2 — VP8 inter-frame (P-frame) decoding is not implemented on the receive path

- **Stated Goal:** README headline feature lists VP8 video *"with both I-frames (key frames)
  and P-frames (inter frames) for bandwidth-efficient video"* (`README.md:340-341`).
- **Current State:** The README's own ToxAV detail section discloses the limitation
  (`README.md:342-345`): *"Current decode behavior is keyframe-oriented: inter frames are not
  decoded by the existing decoder path and will display as the last decoded key frame…"*.
  P-frames are encoded and transmitted, but a receiver cannot decode them; clean per-frame
  video requires forcing all-keyframes at higher bandwidth cost.
- **Impact:** Low–Medium. The limitation is disclosed in prose, but the headline bullet
  ("P-frames for bandwidth-efficient video") implies a working decode path that does not
  exist. A user relying on inter-frame video sees frozen frames between keyframes.
- **Closing the Gap:** Either implement P-frame decoding in the receive path, or align the
  headline feature bullet with the disclosed limitation (e.g. "VP8 encode supports I/P-frames;
  the bundled decoder currently decodes key frames only").

## Gap 3 — Nym / Lokinet transports are dial-only and require an external proxy not surfaced in the headline

- **Stated Goal:** README Features advertise multi-network transport including
  *"Lokinet `.loki` (dial-only), and Nym `.nym` (dial-only)"* (`README.md:47`), reinforced by
  the capability table (`README.md:287-288`).
- **Current State:** The "(dial-only)" qualifier is accurate, and service hosting is honestly
  rejected: `transport/nym_transport_impl.go:100` returns
  *"Nym service hosting not supported via SOCKS5"* wrapping `ErrNymNotImplemented`
  (`:15-18`, *"requires Nym SDK websocket client integration"*). However, even dialing
  depends on an **externally running** `nym-socks5-client` reachable via the `NYM_CLIENT_ADDR`
  environment variable; this hard prerequisite is not stated in the headline feature list, and
  a user expecting to *receive* Tox traffic over Nym/Lokinet cannot.
- **Impact:** Low–Medium. The label is correct but easy to overlook; the external-dependency
  prerequisite and inbound-unsupported nature are under-documented.
- **Closing the Gap:** Either implement the Nym websocket-client listener for inbound, or keep
  the "(dial-only)" label and add a one-line note in the feature list that a running
  `nym-socks5-client` (`NYM_CLIENT_ADDR`) is required and that inbound/listening over
  Nym/Lokinet is unsupported.

## Gap 4 — `MetricsAggregator` report delivery is fire-and-forget despite a `Stop()` that implies clean shutdown

- **Stated Goal:** `MetricsAggregator.Stop()` reads as an orderly shutdown of the metrics
  subsystem (`av/metrics.go:178-195`), and the sibling `av/adaptation.go` establishes the
  project convention of tracking callback goroutines with a `WaitGroup`.
- **Current State:** `generateReport` dispatches each report via `go callback(report)`
  (`av/metrics.go:395`) with no `WaitGroup`; `Stop()` only calls `cancel()` and does not wait
  for in-flight callbacks. Under a slow/blocking user callback, goroutines accumulate every
  `reportInterval`, and a report may execute after `Stop()` returns.
- **Impact:** Low–Medium. Bounded for well-behaved callbacks; a misbehaving callback leaks
  goroutines and breaks the implied "stopped means quiesced" contract.
- **Closing the Gap:** Track the callback goroutine with a `sync.WaitGroup` (mirroring
  `adaptation.go`) and `Wait()` in `Stop()`, or invoke the callback synchronously.
  (Tracked as `AUDIT.md` finding **M-1**.)
