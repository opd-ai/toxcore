# Implementation Gaps — 2026-06-03

This document records gaps between what `toxcore-go` claims (README, GoDoc, feature
list) and what the code actually does. Each gap is confirmed against specific files
and line numbers in the current tree and is ordered by user impact.

## Gap 1 — ToxAV answering side transmits no audio/video in the standard deployment

- **Stated Goal:** README "ToxAV Audio/Video" advertises working peer-to-peer calling:
  `toxav.Answer(friendNumber, 64000, 500000)` followed by "Media is flowing"
  (`CallStateActive`), with bidirectional `AudioSendFrame`/`VideoSendFrame`.
- **Current State:** `Manager.AnswerCall` calls `call.SetupMedia(m.transport, friendNumber)`
  passing the manager transport **directly** (`av/manager.go:1232`), whereas
  `Manager.StartCall` first unwraps it via the `underlyingTransportProvider` /
  `GetUnderlyingTransport()` shim (`av/manager.go:1101–1115`). In the normal
  `toxcore.NewToxAV(tox)` path `m.transport` is a `toxAVTransportAdapter`, which does
  **not** implement `transport.Transport`. The assertion in `setupRTPSession`
  (`av/types.go:669–677`) therefore fails, logs *"Transport does not implement
  transport.Transport — RTP session will not be created. Audio/video will be processed
  but not transmitted via RTP."*, and returns nil. The answerer encodes media locally
  but sends no RTP, so the call is effectively one-directional (only the initiator
  transmits).
- **Impact:** High. Two peers using the documented call/answer flow get audio/video in
  only one direction. The README presents bidirectional calling as a working feature.
- **Closing the Gap:** In `AnswerCall`, unwrap `m.transport` exactly as `StartCall` does
  (check for `underlyingTransportProvider` and pass `GetUnderlyingTransport()` to
  `SetupMedia`). Add an integration test asserting the answerer's `rtpSession != nil`.
  (Tracked as AUDIT.md finding **H-02**.)

## Gap 2 — "Forward secrecy" is advertised unconditionally but degrades silently

- **Stated Goal:** README "Asynchronous Offline Messaging": *"All messages maintain
  end-to-end encryption and forward secrecy."* Features: *"Forward secrecy — One-time
  pre-keys consumed per message, auto-refreshed when fewer than 20 remain."*
- **Current State:** Two compounding issues break the guarantee:
  1. **Refresh accounting counts used keys.** Consumed receive-side pre-keys are marked
     `Used=true` but left in `bundle.Keys` (`async/prekeys.go:528–545`), yet
     `NeedsRefresh` and `GetRemainingKeyCount` both return `len(bundle.Keys)`
     (`async/prekeys.go:315`, `async/prekeys.go:387` — the latter documented as
     "number of unused keys"). A fully-consumed 200-key bundle still reports 200, so the
     "fewer than 20 remain" auto-refresh and the low-watermark warning **never fire**.
  2. **Silent non-FS fallback.** When pre-keys are unavailable, `SendAsyncMessage` stores
     the message via `createFallbackForwardSecureMessage` with `PreKeyID=0` and no
     per-message forward secrecy (`async/client.go:315`, `373–404`) — protected only by
     the outer obfuscation layer and the long-term key.
  Together, exhaustion (caused by #1) routinely steers senders into the non-FS path (#2),
  so messages the README implies are forward-secure are not.
- **Impact:** Medium–High (security expectation). Compromise of a long-term static key can
  expose messages users believed had per-message forward secrecy.
- **Closing the Gap:** (a) Count only `!Used` keys in `NeedsRefresh`/`GetRemainingKeyCount`
  so auto-refresh and the watermark hook work as documented; (b) make the non-FS fallback
  fail-closed (error/queue) when a `ForwardSecurityManager` is configured, or document the
  precondition explicitly in the README. (Tracked as AUDIT.md **H-04** and **M-03**.)

## Gap 3 — Double Ratchet does not satisfy its "decrypt-then-commit" invariant

- **Stated Goal:** `ratchet/` provides a Signal-style Double Ratchet for forward secrecy
  and post-compromise security; `RatchetDecrypt`'s GoDoc states "Message keys are deleted
  immediately after use," implying ordinary ratchet semantics where a failed message does
  not corrupt the session.
- **Current State:** `RatchetDecrypt` advances DH/chain/skipped-key state (`session.go:163`,
  `168`, `173–175`) **before** the authenticating `decryptWithMsgKey` at line 177. A
  tampered/forged ciphertext with a plausible header permanently desynchronizes the receive
  chain, so the legitimate message can no longer be decrypted. The Double Ratchet
  specification requires operating on a copy and committing only after successful
  authentication (the sibling `dhRatchetStep` already follows "complete fallible work before
  mutating," but the top-level decrypt does not).
- **Impact:** Medium–High. A network/relay attacker who can inject one packet can DoS a
  ratchet session.
- **Closing the Gap:** Decrypt against a candidate copy of the receive state and commit
  mutations only on success; retain skipped keys on auth failure. (Tracked as AUDIT.md
  **H-01**.)

## Gap 4 — Nym transport is dial-only and cannot host a reachable node

- **Stated Goal:** README Features and `transport/` docs advertise "Nym `.nym` (dial-only)"
  multi-network transport support.
- **Current State:** `transport/nym_transport_impl.go` implements `Dial`/`DialPacket` via a
  local Nym SOCKS5 proxy, but service hosting returns
  `fmt.Errorf("Nym service hosting not supported via SOCKS5: %w", ErrNymNotImplemented)`
  (`transport/nym_transport_impl.go:100`), and `ErrNymNotImplemented` states the listener
  "requires Nym SDK websocket client integration" (`:18`). Dialing additionally requires an
  externally running `nym-socks5-client` and the `NYM_CLIENT_ADDR` environment variable.
  The Nym read path is also DoS-prone on 32-bit builds (AUDIT.md **M-07**).
- **Impact:** Low–Medium. The "(dial-only)" qualifier is accurate but easy to overlook, and
  the hard external-dependency prerequisite is not surfaced in the headline feature list; a
  user expecting to *receive* Tox traffic over Nym cannot.
- **Closing the Gap:** Either implement the Nym websocket-client listener, or keep the
  "(dial-only)" label and add a one-line note that a running `nym-socks5-client` is required
  and inbound/listening over Nym is unsupported.

## Gap 5 — VP8 inter-frame (P-frame) decoding is not supported on the receive path

- **Stated Goal:** README headline lists "ToxAV Audio/Video — … VP8 video via `opd-ai/vp8`
  … with both I-frames (key frames) and P-frames (inter frames) for bandwidth-efficient
  video."
- **Current State:** The README's own ToxAV section discloses the limitation: *"Current
  decode behavior is keyframe-oriented: inter frames are not decoded by the existing decoder
  path and will display as the last decoded key frame instead."* So while P-frames are
  *encoded/sent*, a receiver cannot decode them; clean per-frame video requires forcing
  all-keyframes at higher bandwidth cost.
- **Impact:** Low–Medium. The capability is disclosed in prose, but the headline feature
  bullet ("P-frames for bandwidth-efficient video") implies a working decode path that does
  not exist; users relying on inter-frame video will see frozen frames.
- **Closing the Gap:** Implement P-frame decoding in the receive path, or align the
  feature-list bullet with the disclosed limitation (e.g. "VP8 encode supports I/P-frames;
  the bundled decoder currently decodes key frames only").

## Gap 6 — UPnP NAT traversal trusts SSDP responders (LAN SSRF)

- **Stated Goal:** README "NAT Traversal — STUN external-address discovery, UPnP port
  mapping, UDP hole punching, and TCP relay fallback," presented as a safe convenience
  feature.
- **Current State:** `transport/upnp_client.go:139` fetches the SSDP `LOCATION` URL without
  validating scheme/host, and follows absolute control URLs from the device description.
  A spoofed SSDP responder on the LAN can drive the client to fetch arbitrary internal
  endpoints (server-side request forgery).
- **Impact:** Medium (security). Exploitable only by an attacker already on the local
  network, but UPnP is enabled as a convenience and the risk is undocumented.
- **Closing the Gap:** Restrict the description/control fetch to `http(s)` and private/LAN
  hosts matching the responder/gateway, and disable redirects. (Tracked as AUDIT.md
  **M-06**.)
