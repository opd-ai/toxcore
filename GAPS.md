# Implementation Gaps — 2026-06-02

This document records gaps between what `toxcore-go` claims (README, GoDoc, feature list) and
what the code actually does. Each gap is confirmed against a specific file. Gaps are ordered by
user impact. None of these are critical; the project is broadly faithful to its stated goals.

## Gap 1 — Nym mixnet transport is dial-only and cannot host services

- **Stated Goal:** README Features list and `transport/` documentation advertise
  "Nym `.nym` (dial-only)" multi-network transport support.
- **Current State:** `transport/nym_transport_impl.go` implements `Dial` and `DialPacket`
  via a local Nym SOCKS5 proxy, but `Listen`/service hosting returns
  `fmt.Errorf("Nym service hosting not supported via SOCKS5: %w", ErrNymNotImplemented)`
  (`transport/nym_transport_impl.go:100`). The `ErrNymNotImplemented` sentinel also explicitly
  states "requires the Nym SDK websocket client which is not yet implemented"
  (`transport/nym_transport_impl.go:15-18`). Dialing additionally requires an externally running
  `nym-socks5-client` and the `NYM_CLIENT_ADDR` environment variable.
- **Impact:** Accurate for callers who only need outbound Nym connectivity, but a user expecting
  to *receive* Tox traffic over Nym (i.e. run a reachable node) cannot do so. The "dial-only"
  qualifier is correct but easy to overlook, and the hard external-dependency prerequisite is not
  surfaced in the headline feature list.
- **Closing the Gap:** Either (a) implement the Nym websocket-client listener path described in the
  `ErrNymNotImplemented` message, or (b) keep the README label "(dial-only)" but add a one-line
  note in the Multi-Network Transport section stating that a running `nym-socks5-client` is a
  prerequisite and that inbound/listening over Nym is unsupported.

## Gap 2 — README claims cgo is needed "only for C API bindings"

- **Stated Goal:** README Requirements: "**cgo** required only for C API bindings (`capi/`
  package)"; headline: "all without cgo dependencies in the core library."
- **Current State:** Two non-`capi/` packages contain cgo translation units behind build tags:
  - `crypto/secure_alloc_cgo.go` — `//go:build cgo && (linux || darwin)`, uses
    `mman.h`/`mlock` for hardened secure allocation.
  - `av/video/encoder_cgo.go` — `//go:build cgo && libvpx`, libvpx VP8 encoder with P-frames.
  Both have pure-Go fallbacks (`crypto/secure_alloc_nocgo.go`, `av/video/encoder_purgo.go`), and
  a `CGO_ENABLED=0 go build ./crypto/... ./av/... ./dht/... ./transport/... ./async/...` succeeds
  (verified during this audit). So the primary "no cgo in core" claim holds — cgo is strictly
  opt-in — but the narrower "only for C API bindings" statement is inaccurate.
- **Impact:** Low. A reader auditing the supply chain for cgo usage could be misled into thinking
  `crypto/` and `av/video/` are cgo-free in all build configurations.
- **Closing the Gap:** Reword the Requirements bullet to: "cgo is optional and used only when
  explicitly enabled — for the C API bindings (`capi/`), hardened locked memory (`crypto/`, cgo +
  Linux/macOS), and libvpx VP8 encoding (`av/video/`, `-tags libvpx`). The core library builds and
  runs with `CGO_ENABLED=0`."

## Gap 3 — Noise transport silently drops the first packet to a new peer (undocumented contract)

- **Stated Goal:** README ToxAV/Noise sections and `examples/noise_demo` present
  "Bidirectional communication" and "Encrypted message transmission" as demonstrated,
  working features.
- **Current State:** `NoiseTransport.Send` (`transport/noise_transport.go:377-415`) initiates a
  handshake on the first send to a peer with no established session and then returns
  `ErrNoiseSessionIncomplete` (line 405) **without queuing the payload** — the application data is
  dropped. This is an intentional, security-motivated choice (no cleartext downgrade), but the
  "first message is lost until the handshake completes; retry on `ErrNoiseSessionIncomplete`"
  contract is not documented on the exported method, and the shipped example does not implement the
  retry for the reverse direction. As a result `examples/noise_demo`'s `TestNoiseMessageExchange`
  fails reproducibly (see AUDIT.md finding M-01).
- **Impact:** Medium. Developers copying the example will see dropped first messages and may
  wrongly conclude the transport is broken, or — worse — add an insecure fallback.
- **Closing the Gap:** (1) Document the drop-and-retry contract in the `Send` GoDoc and reference
  `ErrNoiseSessionIncomplete`. (2) Update `examples/noise_demo` to retry on
  `ErrNoiseSessionIncomplete` (and/or send an explicit reverse-direction handshake trigger) so the
  advertised bidirectional demo passes. (3) Optionally provide a small helper that buffers one
  pending packet per peer and flushes it when the session completes.

## Gap 4 — `net.*` wrappers do not shut down when the owning `Tox` instance is killed

- **Stated Goal:** README: "Go net.* Interfaces — `net.Conn`, `net.Listener`,
  `net.PacketConn`, and `net.Addr` implementations for stream and datagram Tox communication
  (`toxnet/`)", presented as drop-in standard-library-style primitives.
- **Current State:** `ToxConn`, `ToxListener`, and `PacketConn` build their cancellation contexts
  from `context.Background()` (`toxnet/conn.go:61`, `toxnet/listener.go:59`,
  `toxnet/packet_conn.go:79`) rather than from the parent `Tox` lifecycle. Calling `Tox.Kill()`
  does not cancel these contexts; only the wrapper's own `Close()` does. A `ToxListener` also spawns
  an accept goroutine (`toxnet/listener.go:108`) tied to that context.
- **Impact:** Medium. Killing the Tox instance without separately closing every outstanding
  connection/listener leaves goroutines and contexts alive for the process lifetime. This matches
  the conventional `net` "caller must Close" contract, but differs from what users may expect for a
  wrapper whose lifetime is logically bounded by its `Tox` parent.
- **Closing the Gap:** Derive each wrapper's context from the owning `Tox` instance's context so
  that `Tox.Kill()` cascades cancellation, while retaining `Close()` for explicit teardown. Add a
  GoDoc note clarifying the ownership/lifecycle relationship between `Tox` and its `toxnet` wrappers.

## Gap 5 — Async fallback path advertises "forward secrecy" but degrades to no per-message FS

- **Stated Goal:** README: "Asynchronous Offline Messaging — … end-to-end encryption, forward
  secrecy via one-time pre-keys"; Features: "Forward secrecy — One-time pre-keys consumed per
  message".
- **Current State:** When no `ForwardSecurityManager` is configured or no pre-keys are available
  for the recipient, `AsyncClient.SendAsyncMessage` falls back to
  `createFallbackForwardSecureMessage` (`async/client.go:315`, `373-403`), which wraps the message
  **without** per-message one-time-pre-key forward secrecy (the payload is protected only by the
  outer epoch-obfuscation/long-term-key layer). The code comments are honest about this
  (`async/client.go:312-313, 378-380`) and emit a runtime `Warn` (`async/client.go:364-368`), but
  the README presents forward secrecy as unconditional.
- **Impact:** Low–Medium (security expectation). A message sent before a pre-key exchange has
  completed lacks the forward-secrecy property the README implies, and compromise of the long-term
  key could expose such messages.
- **Closing the Gap:** Document the precondition in the README/Async section: forward secrecy
  applies once a pre-key exchange has succeeded for the recipient; messages sent before that fall
  back to long-term-key-protected delivery. Consider making the fallback opt-in (e.g. an option to
  fail closed instead of degrading) for deployments that require strict forward secrecy. Also fix
  the deterministic fallback message ID (AUDIT.md L-01) while touching this path.
