# RESOURCE LIFECYCLE AUDIT — 2026-04-21

## Project Resource Profile

**Module:** `github.com/opd-ai/toxcore`  
**Go version:** 1.25.0 (toolchain go1.25.8)  
**Audit scope:** All non-test, non-example production source files  
**Tool baseline:** `go vet ./...` — passes with zero warnings  

**Resource categories found in production code:**
| Category | Count | Notes |
|---|---|---|
| File handles (`os.Open`, `os.Create`, `os.OpenFile`) | 4 sites | WAL, file transfer, testnet orchestrator |
| Network connections (TCP/UDP `net.Dial*`, `net.Listen*`) | 30+ sites | UDP transport, TCP transport, STUN, SOCKS5, relay, UPnP, hole-puncher, mDNS, LAN discovery |
| HTTP response bodies | 4 sites | UPnP device description, SOAP, bootstrap-node fetcher |
| Child processes (`exec.Command`) | 1 site | `cmd/gen-bootstrap-nodes` (developer tool only) |
| Tickers (`time.NewTicker`) | 35 sites | Maintenance loops, keepalive, cleanup goroutines |
| Goroutines spawned without WaitGroup | ~8 sites | Checkpoint goroutines in WAL are the critical case |
| `bufio.Writer` over file | 1 site | WAL 64 KB write buffer |

---

## Resource Inventory

| Package | File Handles | Net Connections | HTTP Bodies | Child Procs | Custom Closers | Temp Files |
|---------|:---:|:---:|:---:|:---:|:---:|:---:|
| `file` | 2 (send/recv) | 0 | 0 | 0 | Transfer.Cancel/complete | 0 |
| `async` (wal.go) | 1 (WAL file) | 0 | 0 | 0 | WriteAheadLog.Close | 0 |
| `transport` (udp) | 0 | 1 UDP PacketConn | 0 | 0 | UDPTransport.Close | 0 |
| `transport` (tcp) | 0 | 1 listener + N clients | 0 | 0 | TCPTransport.Close | 0 |
| `transport` (relay) | 0 | 1 TCP conn | 0 | 0 | RelayClient.Close | 0 |
| `transport` (socks5_udp) | 0 | 1 TCP + 1 UDP | 0 | 0 | SOCKS5UDPAssociation.Close | 0 |
| `transport` (upnp_client) | 0 | 1 UDP (DialUDP) | 2 GET/POST | 0 | defer conn.Close | 0 |
| `transport` (hole_puncher) | 0 | 1 UDP PacketConn | 0 | 0 | HolePuncher.Close | 0 |
| `transport` (stun_client) | 0 | 1 UDP conn per call | 0 | 0 | defer conn.Close | 0 |
| `transport` (reuseport) | 0 | N UDP sockets | 0 | 0 | ReusePortTransport.Close | 0 |
| `dht` (local_discovery) | 0 | 1 UDP PacketConn | 0 | 0 | LANDiscovery.Stop | 0 |
| `dht` (mdns_discovery) | 0 | 2 UDP PacketConns | 0 | 0 | MDNSDiscovery.Stop | 0 |
| `testnet/internal` | 1 (log file) | 0 | 0 | 0 | TestOrchestrator.Cleanup | 0 |
| `cmd/gen-bootstrap-nodes` | 0 | 0 | 1 GET | 1 (gofmt) | — | 0 |

---

## Findings

### CRITICAL

- [ ] **`Tox.Kill()` does not close active `file.Transfer` file handles** — `toxcore_lifecycle.go:63-71`, `toxcore.go:355`, `file/transfer.go:231,240` — **Demonstrated leak path:** (1) Call `tox.FileSend(friendID, filename, fileSize, fileID)` → `file.NewTransfer` is stored in `t.fileTransfers[key]` (toxcore.go:93). (2) The application calls `tox.FileControl(friendID, fileID, FileControlAccept)` or sends the transfer, which eventually calls `transfer.Start()` → `openTransferFile()` → `t.FileHandle, err = os.Open(t.FileName)` or `os.Create(t.FileName)`. (3) Application calls `tox.Kill()`. `Kill()` calls `closeTransports()`, `stopBackgroundServices()`, `cleanupManagers()`, `clearCallbacks()`. `cleanupManagers()` (toxcore_lifecycle.go:138-155) nils `t.fileManager` and `t.requestManager` but never iterates `t.fileTransfers`. No `Cancel()` or `complete()` is called on any in-progress transfer. **Result:** `Transfer.FileHandle` remains open — the OS file descriptor is leaked for every transfer that was active at shutdown time. **Impact:** FD leak in any long-running process that terminates while file transfers are in progress. On Linux, `ulimit -n` defaults to 1024; repeated restart-under-transfer cycles exhaust the FD table. **Remediation:** In `Kill()` (or `cleanupManagers()`), iterate `t.fileTransfers` and call `transfer.Cancel()` on each entry before clearing the map.

### HIGH

- [ ] **`async/wal.go:logEntry()` spawns unbounded checkpoint goroutines with no WaitGroup** — `async/wal.go:291-299` — **Demonstrated leak path:** `logEntry()` acquires `w.mu`, increments `w.entriesCount`, checks `shouldCheckpoint()`, and if true executes `go func() { w.Checkpoint() }()` (line 291). This goroutine is spawned with no `sync.WaitGroup` tracking. `Close()` (line 476) flushes, syncs, and closes the file but contains no `wg.Wait()` call. **Race scenario:** (1) Heavy write load causes `shouldCheckpoint()` to return true on many successive `logEntry()` calls. (2) Multiple checkpoint goroutines accumulate in the scheduler — all blocked on acquiring `w.mu` behind the current write. (3) Application calls `Close()`. `Close()` acquires `w.mu`, sets `w.closed = true`, flushes and closes `w.file`. (4) Queued checkpoint goroutines run, acquire `w.mu`, observe `w.closed == true`, return an error, and log a warning — **the WAL file itself is not double-closed**, but these goroutines have no bounded lifetime relative to `Close()`. Under sustained write pressure dozens of goroutines pile up, none of which are accounted for by the WAL's shutdown. **Impact:** Goroutine leak proportional to write volume; `Close()` is not a synchronous shutdown boundary. **Remediation:** Add a `checkpointWg sync.WaitGroup` to `WriteAheadLog`. In `logEntry()`, call `w.checkpointWg.Add(1)` before `go func()` and `defer w.checkpointWg.Done()` inside the goroutine. In `Close()`, call `w.checkpointWg.Wait()` before closing the file.

### MEDIUM

- [ ] **`testnet/internal/orchestrator.go:Cleanup()` closes log file without resetting global `logrus` output** — `testnet/internal/orchestrator.go:162-166,447-453` — **Demonstrated leak path:** `NewTestOrchestrator` calls `logrus.SetOutput(logFile)` (line 166) when `config.LogFile != ""`. `Cleanup()` calls `to.logFile.Close()` (line 449) and sets `to.logFile = nil` — but never calls `logrus.SetOutput(os.Stderr)` to redirect the global logger away from the closed file. Any log statement fired after `Cleanup()` returns (e.g., from a still-running goroutine, a deferred cleanup in `cmd/main.go`, or a subsequent test) writes to the now-closed file descriptor. On Linux, this results in `write /path/to/log: file already closed` panic or silent errno `EBADF`. **Impact:** Use-after-close of the global logrus output; can trigger panics or silently discard post-cleanup log events. **Remediation:** In `Cleanup()`, call `logrus.SetOutput(os.Stderr)` (or a caller-provided writer) immediately before closing `to.logFile`.

- [ ] **`transport/upnp_client.go:73` — `net.DialUDP` returns concrete `*net.UDPConn`, violating project interface rules** — `transport/upnp_client.go:73-80` — **Evidence:** `conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(239,255,255,250), Port: 1900})`. The project rules (documented in code-change guidelines and `transport/doc.go`) explicitly state: "Never use `net.UDPConn`; use `net.PacketConn` instead. Never use `net.UDPAddr` or `net.TCPAddr`; use `net.Addr` only." The file also constructs `&net.UDPAddr{…}` directly for the destination address. `defer conn.Close()` is present, so no FD leak occurs. However, this is the only production violation found; all other transport code correctly uses `net.PacketConn`, `net.Conn`, and `net.Addr`. **Impact:** Code is non-testable via mock transports; violates the single consistent networking contract; the concrete type `*net.UDPConn` exposes methods not part of the `net.PacketConn` interface (e.g., `SetReadBuffer`), which may be relied upon in future code. **Remediation:** Replace `net.DialUDP` with `net.Dial("udp4", "239.255.255.250:1900")` (returns `net.Conn`) and use the `net.Conn` interface throughout `ssdpDiscover`. Replace `&net.UDPAddr{…}` construction with `net.ResolveUDPAddr("udp4", "239.255.255.250:1900")` if an `Addr` is needed, but for a simple dial, `net.Dial` eliminates the need for an explicit address struct.

### LOW

- [ ] **`file/transfer.go:complete()` does not nil `FileHandle` after closing** — `file/transfer.go:642-666` — **Evidence:** `complete()` calls `t.FileHandle.Close()` (line 645) but does not subsequently set `t.FileHandle = nil`. `Cancel()` guards its own close with `if t.FileHandle != nil` (line 450). In the current state machine, once `complete()` runs, `t.State` is set to `TransferStateCompleted` or `TransferStateError` and `Cancel()` returns early with "transfer already finished" — so no double-close occurs via normal flows. However, the non-nil handle is a latent hazard: if any future code path calls `complete()` twice (e.g., a race on the callback), or if `FileHandle` is inspected after completion, the closed `*os.File` remains accessible as a non-nil pointer. **Impact:** Low risk today; defensive nil-out is missing. **Remediation:** Add `t.FileHandle = nil` immediately after `t.FileHandle.Close()` in both `complete()` and `Cancel()`.

- [ ] **`async/wal.go` goroutine accesses WAL after `Close()` completes** — `async/wal.go:287-299` — **Evidence:** Because there is no WaitGroup (see HIGH finding above), checkpoint goroutines spawned by `logEntry()` can acquire `w.mu` and call `w.writeEntry()` → `w.writer.Flush()` after `Close()` has already called `w.file.Close()`. The `w.closed` flag prevents the write (line 323: `if w.closed { return errors.New("WAL is closed") }`), so no double-close occurs. But: (a) the goroutine proceeds to log `"Failed to write checkpoint entry"` or `"Failed to create checkpoint"` via `w.logger.WithError(err).Warn(...)`, which invokes the global logrus logger potentially after the log file has been closed (see MEDIUM finding above); (b) there is no caller-observable way to know that a checkpoint write was silently dropped. **Impact:** Silent data loss for in-flight checkpoints; WAL's "checkpoint completed" semantics are not guaranteed even when no error is returned by `logEntry()`. **Remediation:** Same WaitGroup fix as the HIGH finding resolves this; additionally consider returning a channel from `logEntry()` that signals when the checkpoint goroutine completes, or make checkpointing synchronous under write lock.

---

## False Positives Considered and Rejected

| Candidate Finding | Reason Rejected |
|---|---|
| `dht/iterative_lookup.go:222` — `defer wg.Done()` inside loop | The defer is inside a goroutine closure (`go func(n *Node) { defer wg.Done(); … }(node)`) launched within the for loop. The goroutine function's defer scope is the goroutine itself, not the loop body. Correct and idiomatic. |
| `group/dht_replication.go:216`, `group/chat.go:1758`, `async/client.go:518` — same pattern | All are `defer wg.Done()` inside goroutine closures launched in loops. Not defer-in-loop anti-patterns. |
| `transport/relay.go:RelayClient.Close()` does not close `activeConn` on keepalive loss | `Close()` (line 581-604) does close `activeConn` and stops `keepaliveTicker`. The keepalive path that calls `a.Close()` is within `SOCKS5UDPAssociation`, a separate type. No leak. |
| `dht/mdns_discovery.go` — `conn4`/`conn6` not closed on `joinMulticastGroup` failure | If `conn4` succeeds but `conn6` fails (line 67-84), `conn4` is stored in `md.conn4`; only `conn6` is returned as nil. `Stop()` → `closeConnections()` closes `md.conn4`. No leak on partial failure. |
| `transport/tcp.go:createNewConnection` — connection not closed if context cancelled after dial but before store | `storeNewConnection` runs immediately after `net.Dial` (line 264). The context is checked only via `t.ctx` in `acceptConnections`, not in `createNewConnection`. If context is cancelled, the goroutine `handleConnection` will eventually detect EOF and call `cleanupConnection`. No permanent leak. |
| `noise/psk_resumption.go:SessionCache` — goroutine started in `NewSessionCache` but `Close()` never called in production | `NewSessionCache` is only called in test files; it is never called in production transport or noise code paths found by exhaustive search. The goroutine is therefore never actually started in production. Not a production leak. |
| `async/wal.go:Checkpoint()` called from `logEntry()` goroutine — could double-flush | `Checkpoint()` acquires `w.mu` independently; all WAL I/O is mutex-protected. `flushAndSync()` can be called by multiple goroutines serially (they block on `w.mu`). No double-flush; each call to `flushAndSync` sees a fresh, unflushed state. |
| `cmd/gen-bootstrap-nodes/main.go:94` — `exec.Command("gofmt", …)` child process | `cmd.CombinedOutput()` waits for the child to exit (internally calls `Wait()`). Process is properly reaped. Developer tool only; no production usage. |
| `transport/socks5_udp.go:keepAliveTimer` — `time.AfterFunc` goroutine may fire after `Close()` | `Close()` calls `stopKeepAliveTimer()` which calls `a.keepAliveTimer.Stop()`. If the timer fires concurrently with `Stop()`, the goroutine checks `a.closed` (line 621) and returns immediately. `time.AfterFunc` guarantees at most one concurrent invocation. No leak. |
| `testnet/internal/orchestrator.go` — log file leaked if `NewTestOrchestrator` fails after opening log | On failure, the function returns `nil, err` without calling `logFile.Close()` — but the only post-open failure path is `return &TestOrchestrator{…}, nil` which always succeeds. The struct is constructed with zero-value fields plus the opened file, so no failure path exists between `os.OpenFile` and the return. Not a leak. |
