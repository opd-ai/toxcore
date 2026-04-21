# Resource Management Gaps - 2026-04-21

**Repository:** `opd-ai/toxcore`
**Companion to:** [`AUDIT.md`](AUDIT.md)
**Scope:** Structural or design-level gaps in resource lifecycle management - places where the
project's architecture makes it difficult or error-prone to acquire, track, and release resources
correctly. Each gap is backed by concrete evidence from source code.

---

## GAP-1 - `Tox.Kill()` Has No Resource Registry for Active File Transfers

- **Stated Goal**: `toxcore_lifecycle.go:60` documents `Kill()` as stopping the Tox instance and
  releasing "all resources". `doc.go:20` shows `defer tox.Kill()` as the canonical cleanup pattern,
  implying Kill() is sufficient for complete teardown.
- **Current State**: `Kill()` calls `closeTransports()`, `stopBackgroundServices()`,
  `cleanupManagers()`, and `clearCallbacks()`. `cleanupManagers()` (line 138-155) nils out
  `t.messageManager`, `t.fileManager`, and `t.requestManager` - but `t.fileTransfers` (the
  `map[uint64]*file.Transfer` at `toxcore.go:355`) is never iterated or emptied. No
  `Transfer.Cancel()` or `Transfer.FileHandle.Close()` is called on any active transfer during
  shutdown. The `file.Transfer` type has no finalizer to close the underlying `*os.File` on GC.
- **Risk**: Every file transfer in `TransferStateRunning` or `TransferStatePaused` when `Kill()` is
  called leaks one OS file descriptor per active transfer. In processes that start many transfers and
  call `Kill()` without explicit per-transfer cancellation (the common pattern shown in examples),
  descriptors accumulate across restarts until `EMFILE` is returned.
- **Closing the Gap**: (1) In `cleanupManagers()`, iterate `t.fileTransfers`:
  `for _, tr := range t.fileTransfers { _ = tr.Cancel() }; t.fileTransfers = nil`. (2) As
  defense-in-depth, add a `runtime.SetFinalizer` on `*Transfer` that closes `FileHandle` if
  non-nil, emitting a `logrus.Warn` to identify the leak site. (3) Document in `file/doc.go` that
  callers must either cancel all transfers before `Kill()` or rely on `Kill()` to cancel them.

---

## GAP-2 - WAL `Close()` Is Not a Synchronous Shutdown Boundary

- **Stated Goal**: `async/wal.go:476` documents `Close()` as closing the WAL file. The conventional
  Go contract for `io.Closer` is that after `Close()` returns, no further I/O occurs on the
  underlying resource. The WAL is used for crash-recovery durability, so "no further I/O after
  Close" is a correctness requirement, not merely a style preference.
- **Current State**: `logEntry()` (line 263) spawns `go func() { w.Checkpoint() }()` with no
  `sync.WaitGroup`. `Close()` (line 476) flushes and closes `w.file` without waiting for queued
  checkpoint goroutines. A checkpoint goroutine scheduled between the spawn point and the `Close()`
  mutex acquisition will (correctly) detect `w.closed == true` and return an error - but the
  checkpoint write is silently dropped, and the goroutine's lifetime extends past `Close()`. There
  is no mechanism for callers to know that pending checkpoints were abandoned.
- **Risk**: (1) Goroutine leak under write pressure - each `shouldCheckpoint() == true` event spawns
  an additional goroutine that outlives `Close()`. (2) Silent data loss: a checkpoint that was
  logically requested never completes; the WAL file may be larger than necessary on next open, and
  replay will process more entries than expected. (3) If the WAL is used in a context where the
  process exits shortly after `Close()`, the Go runtime may kill these goroutines mid-execution.
- **Closing the Gap**: Add `checkpointWg sync.WaitGroup` to `WriteAheadLog`. Before spawning the
  goroutine, call `w.checkpointWg.Add(1)`; inside the goroutine, `defer w.checkpointWg.Done()`. In
  `Close()`, call `w.checkpointWg.Wait()` after setting `w.closed = true` (while the mutex is
  released) and before closing the file. This guarantees all in-flight checkpoint goroutines observe
  `w.closed` and exit cleanly before the file handle is closed.

---

## GAP-3 - Global `logrus` Output Mutation Is Not Reverted on Cleanup

- **Stated Goal**: `testnet/internal/orchestrator.go` is a test orchestration utility. `Cleanup()`
  (line 445) is documented as releasing "resources held by the orchestrator". Standard Go resource
  cleanup contracts require that after `Cleanup()` returns, all resources owned by the object are
  released and no further side effects occur.
- **Current State**: `NewTestOrchestrator()` calls `logrus.SetOutput(logFile)` (line 166) as a
  global side-effect. `Cleanup()` closes `to.logFile` but does not restore the global logrus output
  (e.g., to `os.Stderr`). The global logrus singleton continues pointing to the now-closed file. Any
  subsequent log statement from any goroutine - including deferred functions in `cmd/main.go` that
  fire after `cleanupOrchestrator()` - writes to the closed fd.
- **Risk**: (1) `write: file already closed` panic or silent `EBADF` in global logger after
  cleanup. (2) Any test suite that creates multiple orchestrators sequentially or logs after cleanup
  silently fails. (3) Global logrus mutation is an implicit side-effect that affects any other
  goroutine currently using logrus.
- **Closing the Gap**: (1) In `Cleanup()`, add `logrus.SetOutput(os.Stderr)` before
  `to.logFile.Close()`. (2) Long-term: use a `*logrus.Logger` instance (not the global logger) so
  that `TestOrchestrator` has its own non-shared logger. Pass `to.logger.Logger` to components that
  need a logger reference. This avoids global state mutation entirely.

---

## GAP-4 - No Lint or Vet Rule Enforcing the `net.Conn`/`net.PacketConn` Interface Contract

- **Stated Goal**: The project's networking guidelines (documented in `transport/doc.go` and
  code-change instructions) explicitly prohibit use of concrete network types (`net.UDPConn`,
  `net.TCPConn`, `net.UDPAddr`, `net.TCPAddr`) and type assertions from interface to concrete. The
  rule is described as "critical for testability with mock transports".
- **Current State**: One production violation exists at `transport/upnp_client.go:73`:
  `conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{...})`. The rest of the codebase (63
  transport source files) correctly uses interface types. There is no static analysis rule in
  `staticcheck.conf`, no `go:generate` check, and no CI step that enforces this contract.
- **Risk**: Future contributions will introduce additional concrete-type usage as the codebase grows,
  making it progressively harder to substitute mock transports in tests, to support new overlay
  networks (Tor, I2P, Nym), and to maintain the interface-uniform contract on which the
  `MultiTransport` aggregation depends.
- **Closing the Gap**: (1) Fix the existing violation: replace `net.DialUDP` with
  `net.Dial("udp4", addr)` and remove `&net.UDPAddr{...}` construction. (2) Add a `staticcheck`
  or `go-critic` rule (or a `grep` in CI) that flags `net.DialUDP`, `net.DialTCP`, `net.ListenUDP`,
  `net.ListenTCP`, `net.UDPAddr{`, and `net.TCPAddr{` outside of test files. (3) Document the rule
  in `CONTRIBUTING.md`.

---

## GAP-5 - `file.Transfer` Has No `io.Closer` Interface and No Finalizer

- **Stated Goal**: The file transfer subsystem is presented as a complete, lifecycle-managed feature.
  The `Transfer` type owns an OS file handle (`FileHandle *os.File`) and is stored in the
  `Tox.fileTransfers` map for its entire duration.
- **Current State**: `file.Transfer` exposes `Start()`, `Pause()`, `Resume()`, `Cancel()`, and
  internal `complete()` - but no `Close()` method implementing `io.Closer`. The file handle is only
  closed inside `Cancel()` and `complete()`. If a `Transfer` is removed from `fileTransfers` without
  calling either, the file handle leaks. `runtime.SetFinalizer` is not set on `*Transfer`. Go's GC
  will eventually collect the `*os.File` and the OS will reclaim the descriptor, but at
  indeterminate GC timing.
- **Risk**: As described in GAP-1, `Kill()` does not call `Cancel()`. Additionally, if the
  application layer or future refactoring removes entries from `fileTransfers` directly, file handles
  would leak silently with no `io.Closer` contract to signal the requirement.
- **Closing the Gap**: (1) Implement `func (t *Transfer) Close() error` as an alias for `Cancel()`
  (or a dedicated variant that closes the file without state-machine side effects). This satisfies
  the `io.Closer` interface and makes ownership explicit. (2) Call
  `runtime.SetFinalizer(t, func(t *Transfer) { if t.FileHandle != nil { _ = t.FileHandle.Close() } })`
  in `NewTransfer` as a safety net, with a `logrus.Warn` in the finalizer body so leak sites are
  identifiable in logs. (3) Fix GAP-1 so `Kill()` always calls `transfer.Cancel()` before clearing
  the map.

---

## GAP-6 - `clearBootstrapManager()` Silently Drops Gossip Goroutine Without Stopping It

- **Stated Goal**: `toxcore_lifecycle.go:60` states Kill() releases "all resources". `dht/gossip_bootstrap.go`
  provides a `Stop()` method specifically to cancel the gossip exchange goroutine via a context.
- **Current State**: `clearBootstrapManager()` (toxcore_lifecycle.go:130-132) sets
  `t.bootstrapManager = nil` without calling `t.bootstrapManager.StopGossipExchange()`. If
  `StartGossipExchange()` had been called before `Kill()`, the `exchangeRoutine` goroutine
  (gossip_bootstrap.go:354) and its ticker would continue running because the termination signal
  (`gb.cancel()`) is only sent by `GossipBootstrap.Stop()` which is only reachable via
  `StopGossipExchange()`. **Note:** An exhaustive search shows `StartGossipExchange()` is never
  called in current production paths - making this a latent rather than currently-triggered leak.
  However, the cleanup gap exists today and `StopGossipExchange` exists specifically to be called
  from `Kill()`.
- **Risk**: If `StartGossipExchange()` is wired into the initialization path in any future release
  (a natural evolution of the bootstrap subsystem), goroutine and ticker leaks will silently appear
  in `Kill()` without any code change. The cleanup gap already exists.
- **Closing the Gap**: In `clearBootstrapManager()`, change `t.bootstrapManager = nil` to:
  `if t.bootstrapManager != nil { t.bootstrapManager.StopGossipExchange(); t.bootstrapManager = nil }`.
  This is a no-op today (Stop is idempotent when the goroutine was not started) and prevents a
  silent future regression.
