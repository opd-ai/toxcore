# Implementation Gaps — 2026-05-29

This document records gaps between what toxcore-go **claims** (README.md, ROADMAP.md,
GoDoc) and what the code **actually does**. The codebase is generally honest about its
limitations; the gaps below are documented-but-partial features and one fragile
implementation pattern surfaced by the audit. None block the project's primary stated
goals.

## Lokinet & Nym transports are dial-only (no inbound listening)
- **Stated Goal**: README lists "Multi-Network Transport — IPv4/IPv6 UDP/TCP, Tor
  `.onion`, I2P `.b32.i2p`, Lokinet `.loki` (dial-only), and Nym `.nym` (dial-only)".
- **Current State**: Matches the documentation. `NymTransport.Listen` returns
  `ErrNymNotImplemented` (`transport/nym_transport_impl.go:90-101`) and the Lokinet
  transport is similarly dial-only. `Dial`/`DialPacket` are implemented via the local
  SOCKS5 proxy.
- **Impact**: Peers cannot **receive** inbound connections over Lokinet or Nym; these
  networks can only be used to reach other reachable peers. This limits fully
  symmetric P2P operation over those mixnets.
- **Closing the Gap**: Integrate the Nym SDK websocket service-provider client and a
  Lokinet inbound listener so `Listen` can accept connections, then update the README
  to drop the "dial-only" qualifier. Until then the current honest documentation is
  the correct state.

## C API coverage is partial (~79% of libtoxcore functions)
- **Stated Goal**: README: "C API Bindings — libtoxcore-compatible C function exports
  for toxcore and ToxAV". ROADMAP records "63 functions (~79% coverage)".
- **Current State**: A substantial but incomplete subset of the libtoxcore C surface
  is exported in `capi/`. Roughly one in five upstream functions is not yet bound.
- **Impact**: Existing C clients written against full libtoxcore may fail to link or
  must avoid the missing entry points; "drop-in" compatibility is not complete.
- **Closing the Gap**: Enumerate the libtoxcore symbol set, identify the unbound
  functions, and add the remaining exports in `capi/` with matching signatures and
  tests (`go test ./capi/...`).

## Async response delivery depends on `recover()` over a real race window
- **Stated Goal**: Robust, production-grade asynchronous offline messaging is a core
  advertised feature.
- **Current State**: `async/client.go` reads a per-node response channel under
  `channelMutex`, releases the mutex, then sends on the channel
  (`sendResponseToChannel`, `:1280-1290`) — while a concurrent timeout path closes the
  same channel under the mutex (`cleanupResponseChannel`, `:1222-1227`). A
  send-on-closed-channel panic is therefore reachable and is absorbed by a
  `defer recover()`.
- **Impact**: Functionally safe today (a late response is dropped after timeout), but
  control flow relies on panic recovery in a genuine race, which is fragile and could
  regress into a crash or lost-message bug during refactoring. See AUDIT.md F-L1.
- **Closing the Gap**: Send under `channelMutex` with a `closed` guard, or replace the
  channel-close teardown with a sentinel / `context` cancellation so no send can target
  a closed channel. Validate with `go test -race ./async/...` and a
  timeout-then-late-response regression test.

## No automated dependency-vulnerability scanning in the audit environment
- **Stated Goal**: README emphasizes a strong cryptographic and secure-transport
  posture.
- **Current State**: There is no evidence of an automated `govulncheck` gate, and this
  audit could not query the vulnerability feed (`vuln.go.dev` is unreachable from the
  sandbox). Pinned versions include `golang.org/x/crypto v0.48.0` and
  `golang.org/x/net v0.50.0`.
- **Impact**: Known upstream CVEs in security-sensitive dependencies could remain
  unnoticed during normal development. (No specific advisory was confirmed reachable in
  this run — recorded with explicit uncertainty.)
- **Closing the Gap**: Add `govulncheck ./...` to CI with network access and enforce an
  upgrade policy for flagged dependency ranges. See AUDIT.md F-L2.
