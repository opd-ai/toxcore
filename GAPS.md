# Implementation Gaps — 2026-06-01

Gaps between what `toxcore-go` (README/GoDoc) claims and what the code actually does.
Each gap references confirmed findings in `AUDIT.md`.

## Group chat administration and leaving are unusable (deadlock)
- **Stated Goal**: "Group Chat — DHT-based group chat with role-based permissions" with a
  documented C/Go API (`Leave`, `KickPeer`, `SetPeerRole`, `SetName`, `SetPrivacy`,
  `SetSelfName`).
- **Current State**: Every one of these exported methods takes `g.mu.Lock()` and then
  reaches `collectOnlinePeerJobs` (`group/chat.go:1669`) which calls `g.mu.RLock()` on the
  same non-reentrant `sync.RWMutex`, deadlocking permanently (finding **C‑GRP‑1**).
  Empirically, `go test -tags nonet -race ./group` hangs until the 10-minute timeout inside
  `TestLeaveGroupUnregistration`.
- **Impact**: A user can join and message a group but can never leave it or perform any
  administrative action without hanging the calling goroutine; the group test suite cannot
  complete, so CI cannot validate the feature.
- **Closing the Gap**: Snapshot the peer targets under the lock and release it before
  broadcasting (the pattern already used by `AnnounceSelf`/`RequestPeerList`), or add a
  `collectOnlinePeerJobsLocked` variant; add a `-race` regression test that calls each
  admin/leave method and asserts it returns.

## Documented file-transfer accept/control flow does not work
- **Stated Goal**: README shows accepting an incoming file with
  `tox.OnFileRecv(func(...) { tox.FileControl(friendID, fileID, FileControlResume) })`.
- **Current State**: Incoming transfers live only in `file.Manager`, while `FileControl`/
  `FileAccept`/`FileReject` consult only `t.fileTransfers` (populated solely by outgoing
  `FileSend`). The documented call returns `"file transfer not found"` (finding **C‑FILE‑1**).
  Separately, the root/C `FileSendChunk` emits a `[fileID][position][len][data]` frame that
  the only registered receiver decodes as `[fileID][chunk]`, corrupting the file
  (finding **H‑FILE‑2**).
- **Impact**: Applications cannot accept/pause/resume/cancel incoming transfers through the
  documented API, and chunks sent via the canonical C API arrive corrupted — the file
  transfer feature is effectively non-functional end-to-end through the public surface.
- **Closing the Gap**: Unify on a single canonical transfer registry and a single
  `PacketFileData` wire format shared by `toxcore_file.go` and `file/manager.go`, then add an
  end-to-end accept+transfer test.

## "Noise IK and XX patterns" — XX is broken for the responder
- **Stated Goal**: README/GoDoc advertise the "Noise Protocol Framework (IK and XX patterns)"
  with an exported `NewXXHandshake` and responder example.
- **Current State**: `finalizeHandshakeIfComplete` installs `sendCipher`/`recvCipher` without
  swapping for the responder, unlike the correct IK path, so a completed XX session cannot
  decrypt application traffic (finding **H‑NOISE‑1**). Production transport uses IK, so this
  is latent for library consumers who select XX.
- **Impact**: Any consumer using the documented XX pattern gets a non-working secure channel.
- **Closing the Gap**: Assign ciphers by role in `finalizeHandshakeIfComplete` exactly as the
  IK responder branch does; add an XX round-trip encryption test.

## `toxnet` does not honor the `net.Conn`/`net.PacketConn` contract
- **Stated Goal**: "Go net.* Interfaces — `net.Conn`, `net.Listener`, `net.PacketConn`, and
  `net.Addr` implementations."
- **Current State**: Timeout errors do not implement `net.Error.Timeout()` (**H‑NET‑1**);
  setting a deadline does not unblock an in-progress `Read` (**H‑NET‑4**); listener `Close`
  and the accept-queue-full path self-deadlock with active connections (**H‑NET‑2**,
  **H‑NET‑3**); and connection readiness is wired to presence (`OnFriendStatus`) instead of
  transport connectivity (`OnFriendConnectionStatus`), so `Dial`/`Write` can stall
  (**H‑NET‑5**).
- **Impact**: Standard Go networking code written against these interfaces mishandles
  timeouts, hangs on deadlines, and can deadlock on shutdown — the drop-in `net.*`
  compatibility promise is not met.
- **Closing the Gap**: Implement `Timeout()`/`Temporary()` on `ToxNetError`; add deadline
  wake-up signaling; close connections outside the listener lock; subscribe to
  `OnFriendConnectionStatus`. Validate with `go test -race ./toxnet`.

## "Production-grade concurrent operation" is not race-free on public APIs
- **Stated Goal**: GoDoc states managers are "safe for concurrent use"; the project enforces
  `-race` in CI.
- **Current State**: `messaging.MessageManager` reads `keyProvider`/`transport`/retry/time
  fields without `mm.mu` while setters write them under the lock (**H‑MSG‑1**);
  `real.RealPacketDelivery` reads `transport` without `RLock` during delivery and accepts a
  nil transport (**H‑REAL‑1/2**); the `factory` reads shared config outside the lock
  (**M‑FACT‑1**); ToxAV C callbacks are mutated/read without synchronization (**H‑CAPI‑2**).
- **Impact**: Data races and torn reads under concurrent configure+use, contradicting the
  documented concurrency guarantee.
- **Closing the Gap**: Snapshot shared fields under the relevant lock (or use atomics) before
  use, and reject nil dependencies; verify with `go test -race ./...`.

## ToxAV media path is not hardened against untrusted RTP and re-entrant callbacks
- **Stated Goal**: "ToxAV Audio/Video — peer-to-peer calling … RTP transport, adaptive
  bitrate, and jitter buffering."
- **Current State**: RTP video frame assembly grows without bound for a single timestamp,
  enabling remote memory exhaustion (**H‑AV‑1**); the incoming-call and RTP-receive callbacks
  run while a mutex is held, deadlocking the documented "answer in the callback" pattern
  (**H‑AV‑2**, **M‑AV‑3**); runtime bitrate setters do not reconfigure the encoder or signal
  the peer (**L‑3**); video RTP timestamps use a 90 Hz clock instead of 90 kHz (**M‑AV‑5**).
- **Impact**: A malicious peer can OOM a callee; well-behaved apps can deadlock answering a
  call; A/V sync and adaptive bitrate are degraded.
- **Closing the Gap**: Bound per-frame assembly; invoke callbacks after releasing locks; send
  bitrate-control signaling and reconfigure encoders; fix the 90 kHz timestamp.

## Async forward-secrecy pre-keys are not owner-authenticated
- **Stated Goal**: "Forward secrecy via one-time pre-keys" and identity obfuscation for
  offline messaging.
- **Current State**: DHT pre-key bundles are verified with an attacker-controlled signing key
  embedded in the bundle, with no binding to the claimed owner identity (**H‑ASYNC‑1**);
  malformed pre-key exchanges / imported backups can panic (**M‑ASYNC‑1/2**).
- **Impact**: An attacker can poison pre-keys for any identity (denial of delivery) and can
  crash a node with malformed input.
- **Closing the Gap**: Bind the bundle signing key to the owner's long-term identity and
  reject unbound/nil inputs.

## C API handle lifetime violates cgo pointer rules
- **Stated Goal**: "C API Bindings — libtoxcore-compatible C function exports."
- **Current State**: `tox_new`/`toxav_new` return Go heap pointers (`new(int)`/`new(uintptr)`)
  to C, which retains them across calls with no Go-side reference keeping them alive
  (**H‑CAPI‑1**); several exported functions slice C buffers without nil checks (**M‑CAPI‑3**).
- **Impact**: Use-after-free / wrong-handle lookups and crashes in C/C++ consumers; NULL
  arguments crash the shared library instead of returning an error.
- **Closing the Gap**: Use `C.malloc`-backed opaque integer-ID handles freed in
  `tox_kill`/`toxav_kill`; nil-check all C buffers before `unsafe.Slice`.

## Transport stream framing and relay lifecycle are not concurrency-safe
- **Stated Goal**: "Multi-Network Transport — IPv4/IPv6 UDP/TCP … TCP relay fallback."
- **Current State**: Concurrent TCP sends to one peer interleave the length prefix and body
  (**H‑TR‑1**); a remote relay disconnect nil-derefs the keepalive ticker and crashes
  (**H‑TR‑2**); inbound packets spawn unbounded goroutines (**M‑TR‑3**); `IPAddressParser`
  stores ASCII `host:port` where raw IP bytes are expected (**M‑TR‑4**); SOCKS5 UDP domain
  sources can resolve to a nil IP without error (**M‑TR‑5**).
- **Impact**: Frame corruption/disconnects under concurrent sends, a remote-triggerable crash,
  DoS exposure, and address misrouting.
- **Closing the Gap**: Single-write framing (or per-conn write lock); cancel the keepalive
  goroutine on disconnect; bounded worker pool for packet dispatch; store raw IP bytes; error
  on failed domain resolution.

## The repository's own test suite and stale reports do not reflect current behavior
- **Stated Goal**: README/CONTRIBUTING claim CI runs `go test -tags nonet -race ./...` green;
  prior `GAPS.md` documented three issues.
- **Current State**: Two packages fail (`group` deadlock timeout; `messaging` tests not
  updated after the fail-closed E2EE migration) — finding **T‑1**. The previous `GAPS.md`'s
  three gaps (group broadcast race, peer-discovery callback isolation, Noise handler
  hardening) are now fixed in code and were rewritten here.
- **Impact**: CI cannot currently pass; stale tests can mask real regressions.
- **Closing the Gap**: Fix C‑GRP‑1; update messaging tests to assert
  `errors.Is(err, ErrNoEncryption)` and the blocked-send (fail-closed) behavior rather than
  the legacy exact error string / "message sent" expectation.
