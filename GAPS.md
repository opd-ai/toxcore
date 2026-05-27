# Implementation Gaps — 2026-05-27

This document records places where the project's stated goals (from `README.md`,
`doc.go`, and package-level GoDoc) diverge from what the code actually does.
Each gap links back to the corresponding `AUDIT.md` finding where the
underlying bug is recorded.

## `NonceStore.Close()` is not idempotent
- **Stated Goal**: README — "secure memory wiping" and a hardened crypto
  layer; `crypto/replay_protection.go` package doc — a long-lived nonce store
  used by the transport layer.
- **Current State**: `NonceStore.Close()` (`crypto/replay_protection.go:294-301`)
  calls `close(ns.stopChan)` unconditionally. A second call panics with
  `close of closed channel`. There is no `sync.Once`, no `closed bool` flag,
  and no documented "single call only" contract.
- **Impact**: Defensive `defer ns.Close()` paired with an explicit shutdown
  (a common Go pattern, and one the rest of the project follows for other
  resources) will crash the entire process — dropping every active Tox
  session, in-flight call, and queued message. The crash is silent in unit
  tests because no test exercises double-close.
- **Closing the Gap**: See `AUDIT.md` F-CRYPTO-001. Wrap the close in
  `sync.Once.Do`, then add `TestNonceStoreCloseIdempotent` to lock the
  behaviour in.

## Asynchronous offline messages have no observable retention bound
- **Stated Goal**: README — "store-and-forward delivery through distributed
  storage nodes with end-to-end encryption, forward secrecy via one-time
  pre-keys". Implicit promise: callers can reason about whether a message they
  enqueued will eventually be sent or expire.
- **Current State**: `AsyncManager.sendQueuedMessages`
  (`async/manager.go:861-933`) re-queues every message indefinitely when the
  peer's pre-key channel times out (line 888-902 and 924-928). There is no
  TTL, no `QueueDepth()` accessor, no eviction policy, and no metric exposed
  to API consumers — so callers cannot detect or bound retention.
- **Impact**: A peer that never returns causes their queued messages to live
  forever in `pendingMessages` (until the process restarts, at which point
  they are silently lost because the queue is in-memory only). Memory grows
  monotonically; callers cannot tell the difference between "delivered" and
  "queued for eternity".
- **Closing the Gap**: See `AUDIT.md` F-ASYNC-001. Either add a per-message
  expiry (configurable via `AsyncManagerOptions`) plus an introspection API
  (`AsyncManager.QueueDepth(friendPK [32]byte) int`), or document the current
  retention semantics in `async/doc.go` so that callers can build the
  bookkeeping themselves.

## Retrieved offline messages alias storage-internal buffers
- **Stated Goal**: README — distributed storage nodes that "store and forward"
  encrypted payloads. Implicit promise (and the conventional Go contract):
  retrieving a stored message returns a value the caller can freely process
  without affecting the storage node.
- **Current State**: `RetrieveMessagesByPseudonym`
  (`async/storage.go:670-695`) copies `AsyncMessage` structs by value into the
  returned slice but leaves `EncryptedPayload []byte` aliasing the internal
  buffer. A caller that decrypts in place, scrubs the buffer, or otherwise
  mutates the slice corrupts the storage node's copy and every subsequent
  retrieval.
- **Impact**: Subtle data corruption for any user who follows the natural
  pattern of "decrypt and zero" the returned payload. The bug is latent only
  because no current caller mutates the payload — a guarantee no API can rely
  on indefinitely.
- **Closing the Gap**: See `AUDIT.md` F-ASYNC-002. Deep-copy
  `EncryptedPayload` (and any future slice fields) before returning, then
  add a regression test that mutates the returned payload and re-queries.

## `NAT.GetPublicAddress` can return a mismatched (address, type) pair
- **Stated Goal**: README — NAT traversal that exposes a stable external
  address discovered by STUN / UPnP / hole-punching.
- **Current State**: `GetPublicAddress` (`transport/nat.go:155-169`) reads
  `publicAddr` under a read lock, releases the lock, and then calls
  `DetectNATType` which re-acquires the same lock. A concurrent
  `SetPublicAddress` (or a periodic refresh) between the two lock acquisitions
  can install a different address before the type is sampled.
- **Impact**: Detection callbacks occasionally report a public address that
  belongs to a different network topology than the reported NAT type — a
  confusing observability bug that can poison upstream signalling and waste
  hole-punching attempts.
- **Closing the Gap**: See `AUDIT.md` F-TRANSPORT-001. Compute the
  `(addr, type)` tuple under a single lock acquisition; or pass the sampled
  address explicitly into `DetectNATType` so the second lock is no longer
  required.

## cgo audio/video callbacks have an undocumented pointer-lifetime contract
- **Stated Goal**: README — "libtoxcore-compatible C function exports for
  toxcore and ToxAV". Implicit promise: a C consumer can substitute this
  library for upstream `libtoxav` without rediscovering memory rules.
- **Current State**: `bridgeAudioReceiveFrame` (`capi/toxav_c.go:1196-1209`)
  and the video bridge (`capi/toxav_c.go:1268-1307`) pass pointers into Go
  slice memory to the registered C callback. cgo pins the memory only for the
  duration of the C call; any C client that retains the pointer beyond return
  silently triggers use-after-free. The library does not document this
  expectation anywhere, and does not use `runtime.Pinner` to extend lifetime.
- **Impact**: A drop-in replacement for `libtoxav` whose author follows the
  upstream pattern of "memcpy on first inspection" works; one whose author
  buffers the pointer for later processing crashes intermittently with GC
  pressure. The bug is invisible until a C consumer reports random
  corruption.
- **Closing the Gap**: See `AUDIT.md` F-CAPI-001. Add GoDoc on the exported
  `toxav_callback_*` surface stating the lifetime contract; optionally
  introduce a `_copy` variant that hands ownership to C-allocated memory
  for clients that want explicit lifetime control.

## `cmd/gen-bootstrap-nodes` resolves the output path by filesystem heuristic
- **Stated Goal**: The tool is the generator behind `bootstrap/nodes/default_nodes.go`
  (referenced by `go:generate` in the bootstrap package).
- **Current State**: `main.go:68-76` defaults the output path to
  `bootstrap/nodes/default_nodes.go`, then inspects the CWD: if no
  `bootstrap` directory is present *and* a sibling `node_info.go` exists, it
  silently rewrites the path to `default_nodes.go` (relative to CWD). Any
  unrelated directory containing a file named `node_info.go` will be silently
  overwritten.
- **Impact**: Mostly a developer-ergonomics gap, but the silent overwrite is
  exactly the kind of issue that ruins a generator's reputation. The fix is
  trivial and improves reproducibility of `go generate ./...` runs.
- **Closing the Gap**: See `AUDIT.md` F-CAPI-002. Use `os.Getenv("GOFILE")` or
  require an explicit `-out` flag; remove the filesystem heuristic.

## Bootstrap overlay listener may leak on startup-timeout race
- **Stated Goal**: README — robust multi-network transport that brings up
  Tor / I2P listeners cleanly or fails fast.
- **Current State**: `Server.startOverlayListener` (`bootstrap/server.go:519-537`)
  spawns a goroutine that calls `cfg.transport.Listen` and sends the result
  on a buffered channel. On the `startCtx.Done()` branch it calls
  `cfg.transport.Close()` and returns — but if the overlay transport returns
  a successful listener concurrently with `Close()` (typical for Tor/I2P SAM
  bridges that have already passed their cancellation check by the time the
  context fires), the listener lands in the buffered channel with no receiver
  and is leaked until process exit.
- **Impact**: A bootstrap server that repeatedly times out during start-up
  (slow Tor circuit, congested I2P SAM bridge) accumulates file descriptors
  and onion-service registrations.
- **Closing the Gap**: See `AUDIT.md` F-BOOTSTRAP-001. Drain `listenerCh`
  after timeout and close any listener that arrives, or refactor `Listen` to
  honour `startCtx` cooperatively.
