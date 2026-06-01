# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-06-01

## Project Profile
- **Purpose**: `toxcore-go` is a pure‑Go implementation of the Tox peer‑to‑peer encrypted
  messaging protocol (DHT discovery, friends, 1:1 + group messaging, file transfer, ToxAV
  audio/video, async offline messaging, multi‑network transport, Noise‑IK handshakes).
- **Target users**: Go application developers embedding Tox, and C/C++ consumers via the
  `capi/` libtoxcore‑compatible shared library (cgo).
- **Deployment model**: Library linked into long‑running peer processes that accept
  **untrusted network packets** (UDP/TCP/Noise/RTP) as the primary trust boundary, plus
  untrusted peer‑supplied filenames, savedata, and pre‑key bundles.
- **Critical paths**: packet parsing (`transport/`, `dht/`), crypto + forward secrecy
  (`crypto/`, `async/`, `noise/`, `ratchet/`), messaging state machine (`messaging/`),
  group chat (`group/`), file transfer (`file/`, `toxcore_file.go`), ToxAV media
  (`av/**`, `toxav.go`), and the C ABI (`capi/`).
- **Conventions**: sentinel errors + `%w` wrapping (`errors.Is`/`errors.As`); structured
  `logrus` logging; `crypto.ZeroBytes`/`SecureWipe` for wiping secrets; `sync.RWMutex`
  guarding shared maps; functional options; time injected via `TimeProvider` for tests.

## Audit Scope
Audited every non‑test `.go` file in the production packages below. `examples/`,
`simulation/`, `testnet/`, and `cmd/` demos were excluded from finding generation (not
shipped to library consumers) but were used to confirm reachability.

- **Packages audited**: `toxcore` (root), `async`, `av`, `av/audio`, `av/rtp`, `av/video`,
  `bootstrap`, `capi`, `crypto`, `dht`, `factory`, `file`, `friend`, `group`, `interfaces`,
  `limits`, `messaging`, `noise`, `ratchet`, `real`, `toxnet`, `transport`,
  `transport/internal/addressing`.
- **go-stats-generator metrics**: 251 files, 43,776 LOC, 1,317 functions + 3,046 methods,
  421 structs, 40 interfaces, 27 packages. Avg function length 12.1 lines; avg cyclomatic
  3.5; only 2 functions > complexity 10 (`cloneReflectValue` 16, `ImportPreKeys` 12);
  doc coverage 93.5%; duplication ratio 0.5%.

## Coverage Log
Each checklist category (3b–3j) was applied to each package. ✅ = inspected, no
finding above LOW in that category for that package; ⚠️ = finding(s) recorded.

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ |
| ratchet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ⚠️ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| toxcore (root) | ⚠️ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| messaging | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| group | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| av | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| av/audio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/video | ⚠️ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/rtp | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ✅ |
| toxav (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| capi | ✅ | ⚠️ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| toxnet | ⚠️ | ✅ | ⚠️ | ⚠️ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| bootstrap | ⚠️ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| factory | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ✅ |
| real | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| limits | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary
| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| DHT routing & LAN discovery | ✅ | — |
| Friend management | ✅ | — |
| 1‑to‑1 messaging (fail‑closed E2EE) | ⚠️ | H‑MSG‑1 (config races); stale tests T‑1 |
| Group chat | ❌ | **C‑GRP‑1** (admin/leave APIs deadlock) |
| File transfers | ❌ | **C‑FILE‑1** (documented accept path broken), H‑FILE‑2 (wire‑format corruption) |
| ToxAV audio/video | ⚠️ | H‑AV‑1 (RTP memory exhaustion), H‑AV‑2 (call‑callback deadlock), M‑AV‑* |
| Async offline messaging / forward secrecy | ⚠️ | H‑ASYNC‑1 (prekey bundle auth), M‑ASYNC‑* (nil panics) |
| Multi‑network transport | ⚠️ | H‑TR‑1 (TCP framing race), H‑TR‑2 (relay crash), M‑TR‑* |
| Noise‑IK + XX handshakes | ⚠️ | H‑NOISE‑1 (XX responder ciphers swapped) |
| `net.*` interfaces (`toxnet`) | ⚠️ | H‑NET‑1..5 (contract violations, deadlocks) |
| C API bindings | ⚠️ | H‑CAPI‑1 (cgo handle), H‑CAPI‑2 (callback race), M‑CAPI‑3 |

## Findings

### CRITICAL

- [ ] **C‑GRP‑1** Group write‑lock methods self‑deadlock on every broadcast — `group/chat.go:1096` (`Leave`), `1230` (`KickPeer`), `1258` (`SetPeerRole`), `1298` (`SetName`), `1331` (`SetPrivacy`), `1387` (`SetSelfName`) — concurrency (non‑reentrant `RWMutex`) — Each method takes `g.mu.Lock()` with a deferred unlock, then calls `broadcastGroupUpdateTyped` → `sendToConnectedPeersWithConfig` → `collectOnlinePeerJobs`, which unconditionally calls `g.mu.RLock()` at `group/chat.go:1669`. Go's `sync.RWMutex` is **not reentrant**, so a goroutine holding the write lock that asks for a read lock blocks forever. These are exported, documented C‑API entry points (`//export ToxGroupLeave`, `ToxGroupKickPeer`, `ToxGroupSetPeerRole`, `ToxGroupSetName`, `ToxGroupSetPrivacy`, `ToxGroupSetSelfName`). **Empirically confirmed**: `go test -tags nonet -race ./group` hangs and is killed by the 10‑minute timeout inside `TestLeaveGroupUnregistration` (`group/chat_test.go:797` → `Leave` → `collectOnlinePeerJobs` blocked on `RLock`). — **Consequence:** any caller leaving a group or changing group/peer state hangs permanently; the group package test suite never completes. — **Remediation:** snapshot the data needed for the broadcast and call `g.mu.Unlock()` *before* broadcasting (the pattern already used correctly by `AnnounceSelf`/`RequestPeerList`), or add an unexported `collectOnlinePeerJobsLocked` that assumes the lock is held and drive the broadcast without re‑locking. Validate with `go test -tags nonet -race ./group -run 'Leave|Kick|Role|Name|Privacy'`.

- [ ] **C‑FILE‑1** Documented incoming‑file accept path is non‑functional — `toxcore_file.go:25-46` (`FileControl`/`FileAccept`/`FileReject`) — API contract / data flow — Incoming transfers are stored only inside `file.Manager` (`file/manager.go:342-343`); the bridge in `toxcore.go:647-653` forwards the `OnFileRecv` notification but never mirrors the transfer into `t.fileTransfers`. `t.fileTransfers` is populated **only** by outgoing `FileSend` (`toxcore_file.go:95`). `FileControl` looks up `t.fileTransfers` exclusively, so the exact flow shown in the README — `tox.OnFileRecv(func(...){ tox.FileControl(id, fid, FileControlResume) })` — returns `"file transfer not found"`. — **Consequence:** applications cannot accept, pause, resume, or cancel an incoming file transfer through the documented public/C API. — **Remediation:** route `FileControl`/`FileAccept`/`FileReject` through `file.Manager` (single canonical transfer registry), or register incoming transfers into `t.fileTransfers` when `OnFileRecv` fires. Validate with a new test that calls `FileAccept` from the `OnFileRecv` callback: `go test -tags nonet ./... -run 'File.*Accept|File.*Recv'`.

### HIGH

- [ ] **H‑FILE‑2** Root/C `FileSendChunk` emits a wire format the receiver cannot parse — `toxcore_file.go:283-305` (`buildFileChunkPacket`) vs `file/manager.go:619-639` (`serializeFileData`/`deserializeFileData`) — logic / protocol inconsistency — `Tox.FileSendChunk` (exported `ToxFileSendChunk`, used by `capi/toxcore_c.go:1168`) sends a `PacketFileData` packet framed as `[fileID(4)][position(8)][len(2)][data]`, but the only registered receiver for `PacketFileData` is `file.Manager.handleFileData`, which decodes `[fileID(4)][chunk]`. The 10 header bytes (`position`+`len`) are written into the destination file as if they were payload, and the real `position` is ignored. — **Consequence:** file data sent via the canonical root/C chunk API is corrupted on the receiver. — **Remediation:** unify on one `PacketFileData` wire format used by both `buildFileChunkPacket` and `serializeFileData`/`deserializeFileData`, including explicit position handling and bounds checks. Validate with `go test -tags nonet ./file . -run 'File.*Chunk|File.*Transfer'`.

- [ ] **H‑NOISE‑1** XX handshake responder installs send/recv ciphers in the wrong direction — `noise/handshake.go:508-516` (`finalizeHandshakeIfComplete`) — logic / crypto — `flynn/noise` `Split()` always returns `(cs1, cs2)` where `cs1` = initiator‑write / responder‑read and `cs2` = responder‑write / initiator‑read (verified in `state.go:479-480, 607-608`). The IK path correctly swaps for the responder (`noise/handshake.go:311-312`), but `finalizeHandshakeIfComplete` assigns `sendCipher=cs1, recvCipher=cs2` for **both** roles. The XX responder therefore encrypts with the initiator's cipher and decrypts with its own, so neither side can read the other's post‑handshake traffic. XX is an exported, README‑advertised pattern (`NewXXHandshake`, `noise/doc.go:84,104`). — **Consequence:** every completed XX session produces an unusable encrypted channel; the documented XX pattern is broken for the responder. — **Remediation:** map ciphers by `xx.role` exactly as the IK path does (responder swaps `send`/`recv`). Validate with a full XX handshake test that exchanges encrypted payloads: `go test ./noise`.

- [ ] **H‑ASYNC‑1** Pre‑key DHT bundles are self‑authenticating, not owner‑authenticating — `async/prekey_dht.go:45-53, 227-244, 341-390` — security — Bundle validation verifies the Ed25519 signature using `bundle.SigningPK` carried *inside the bundle*, with no binding proving that `SigningPK` belongs to the claimed `OwnerPK`. An attacker can publish a validly self‑signed bundle for any victim `OwnerPK`. — **Consequence:** pre‑key poisoning / denial of delivery for arbitrary identities and weakened guarantees on the (deprecated) direct forward‑secrecy path. — **Remediation:** bind the signing key to the owner identity (e.g. require the bundle be signed by, or certified against, the owner's known long‑term key) and reject bundles whose `SigningPK` is not provably the owner's. Validate by forging a bundle with a victim `OwnerPK` + attacker `SigningPK` and asserting rejection: `go test ./async`.

- [ ] **H‑MSG‑1** `MessageManager` reads mutable config without holding `mm.mu` — `messaging/message.go:1103,1112,1116` (`keyProvider`), `1240` (`transport`), `957,964` (`retryEnabled`/`timeProvider`) — concurrency — `SetKeyProvider`/`SetTransport`/`SetRetryConfig`/`SetTimeProvider` write these fields under `mm.mu` (`440-499`), but `encryptWithNaCl`, `sendThroughTransport`, and `shouldProcessMessage` read them without `mm.mu` (`encryptMessage` releases the lock at `1063` before calling `encryptWithNaCl`). The type GoDoc states methods are safe for concurrent use (`216-220`). — **Consequence:** data races (detectable under `-race`) and torn reads of transport/key provider during concurrent reconfigure + send. — **Remediation:** snapshot config fields under `mm.mu` before use, or store them via `atomic.Value`. Validate with `go test -tags nonet -race ./messaging`.

- [ ] **H‑AV‑1** Unbounded RTP video frame assembly enables remote memory exhaustion — `av/video/rtp.go:298-303` (`addPacketToAssembly`) — resources / DoS — Untrusted RTP flows `Manager.handleVideoFrame` → `Session.ReceiveVideoPacket` → `RTPDepacketizer.ProcessPacket`. `getOrCreateFrameAssembly` caps only the *number* of distinct timestamps (`maxFrames`), while `addPacketToAssembly` appends packets and grows `receivedSize` for a single timestamp with no cap and no duplicate/marker‑timeout enforcement. A peer can send unlimited fragments for one never‑completed frame. — **Consequence:** a remote peer can drive unbounded heap growth and OOM the process during a call. — **Remediation:** cap packets and bytes per frame assembly, reject duplicate sequence numbers, and evict assemblies exceeding a sane VP8 frame bound. Validate with `go test ./av/video ./av/rtp`.

- [ ] **H‑AV‑2** Incoming‑call callback invoked while the AV manager mutex is held — `av/manager.go:193-209` (`processIncomingCall` → `notifyIncomingCall`) — concurrency (deadlock) — `processIncomingCall` holds `m.mu.Lock()` (deferred unlock) and invokes the user callback via `notifyIncomingCall` (`247-248`) before unlocking. If the callback calls back into the manager (e.g. `AnswerCall`, which locks `m.mu` at `1181`), the goroutine deadlocks on the non‑reentrant mutex — the documented "answer in the call callback" pattern. — **Consequence:** deterministic hang when applications answer/act on a call from within the call callback. — **Remediation:** copy the callback + needed state under lock, unlock, then invoke. Validate with `go test -race ./av -run Callback`.

- [ ] **H‑CAPI‑1** C handles are Go heap pointers retained by C across calls — `capi/toxcore_c.go:309-312` (`tox_new`), `capi/toxav_c.go:454-464` (`toxav_new`) — cgo memory safety — `tox_new` returns `unsafe.Pointer(new(int))` to C; the only live reference to that `*int` lives in C, so it is unreachable from Go and eligible for garbage collection while C still holds and later dereferences it (`getToxFromPointer`). This violates the cgo rule that C must not retain Go pointers after the call returns. — **Consequence:** use‑after‑free / reading a reclaimed handle, yielding a wrong/`!ok` instance lookup or a crash in shared‑library consumers; may trip `cgocheck`. — **Remediation:** allocate the opaque handle with `C.malloc`, store only an integer instance ID in C memory, and `C.free` it in `tox_kill`/`toxav_kill`. Validate with `GOEXPERIMENT=cgocheck2 go test ./capi`.

- [ ] **H‑CAPI‑2** ToxAV C callback table mutated/read without synchronization — `capi/toxav_c.go:280-284` (`GetCallbacks`) vs registrations `1042-1044,1106-1108,1193-1195` and bridge reads `1047-1050,1120-1139,1219-1239` — concurrency — `GetCallbacks` locks only while fetching the `*toxavCallbacks` pointer; subsequent field reads/writes (callback fn + user data) happen unlocked while callback bridges run on the ToxAV iteration goroutine. — **Consequence:** data race and torn callback/user‑data pairs → invoking a callback with mismatched user data. — **Remediation:** guard callback structs with a mutex, or swap in immutable snapshots atomically. Validate with `go test -race ./capi`.

- [ ] **H‑NET‑1** `toxnet` timeout errors do not implement `net.Error` — `toxnet/errors.go:30-76` (`ToxNetError`) — API contract — Deadline paths return `*ToxNetError` wrapping `ErrTimeout` (`conn.go:155,187`, `packet_conn.go:266`, `packet_listener.go:429`), but `ToxNetError` implements only `Error()` and `Unwrap()` — no `Timeout() bool`/`Temporary() bool`. Standard code `if ne, ok := err.(net.Error); ok && ne.Timeout()` therefore never recognizes a toxnet timeout. — **Consequence:** consumers treating these `net.Conn`/`net.PacketConn` implementations like the stdlib will mis‑handle timeouts as fatal errors. — **Remediation:** implement `Timeout() bool` (`errors.Is(e.Err, ErrTimeout)`) and `Temporary() bool` on `*ToxNetError`. Validate with `go test ./toxnet`.

- [ ] **H‑NET‑2** `ToxPacketListener.Close` self‑deadlocks with active connections — `toxnet/packet_listener.go:310-315` vs `toxnet/packet_listener.go:505-514` — concurrency (deadlock) — `Close` holds `l.connMu.Lock()` and calls `conn.Close()` in a loop; `ToxPacketConnection.Close` acquires the same `l.connMu.Lock()` to remove itself from the map → non‑reentrant deadlock whenever any connection is active. — **Consequence:** listener shutdown hangs. — **Remediation:** copy the connection slice under lock, unlock, then close each connection. Validate with `go test -race ./toxnet`.

- [ ] **H‑NET‑3** Accept‑queue‑full path self‑deadlocks and leaks a writer goroutine — `toxnet/packet_listener.go:205-247` — concurrency — `getOrCreateConnection` holds `l.connMu` (deferred), starts `conn.processWrites()` (213), then calls `notifyNewConnection` (215); if `acceptCh` is full it calls `conn.Close()` (247) which re‑locks `l.connMu` (512). — **Consequence:** packet processing wedges with `connMu` held and the started writer goroutine leaks. — **Remediation:** enqueue/notify outside `connMu`; start `processWrites` only after successful enqueue; roll back without `Close` while locked. Validate with `go test -race ./toxnet`.

- [ ] **H‑NET‑4** Setting a deadline does not unblock already‑blocked I/O — `toxnet/conn.go:497-500` (`SetReadDeadline`), `packet_conn.go:411`, `packet_listener.go:548` — API contract — `Read` snapshots the deadline once before blocking; `SetReadDeadline` only assigns the field and never signals the waiting goroutine, so a goroutine already blocked in `Read` will not wake when another goroutine sets a (past) deadline. This violates the `net.Conn` contract that deadlines affect in‑progress calls. — **Consequence:** goroutines can hang until data arrives or the conn closes, defeating cancellation. — **Remediation:** add a deadline‑change notification (channel/`sync.Cond`) and select on it inside the blocking read/write loops. Validate with `go test -race ./toxnet`.

- [ ] **H‑NET‑5** `ToxConn` subscribes to the wrong status callback — `toxnet/callback_router.go:137` — logic / API contract — The connection's readiness is driven by `OnFriendStatus` (user presence Away/Busy/Online), but transport connectivity changes are emitted via `OnFriendConnectionStatus` (`toxcore.go:1186-1194`). `DialContext`/`Write` wait on `IsConnected()`/`connStateCh`, which may never flip even after the friend is transport‑connected. — **Consequence:** `Dial`, listener auto‑accept delivery, and `Write` can stall until timeout despite an active connection. — **Remediation:** register `OnFriendConnectionStatus` and set connected when `status != ConnectionNone`. Validate with `go test -race ./toxnet`.

- [ ] **H‑REAL‑1** `RealPacketDelivery` races transport replacement with delivery — `real/packet_delivery.go:95,122` vs `277-291` — concurrency — `SetNetworkTransport` writes/closes `r.transport` under lock, but `DeliverPacket` (`fetchAndCacheAddress`, `attemptDeliveryWithRetries`) reads `r.transport` without an `RLock`, although `interfaces.IPacketDelivery` requires concurrency safety. — **Consequence:** concurrent replace + deliver can use a closed transport or panic. — **Remediation:** snapshot `r.transport` under `RLock` for the whole delivery attempt. Validate with `go test -race ./real`.

- [ ] **H‑REAL‑2** `SetNetworkTransport(nil)` accepted, then panics on next delivery — `real/packet_delivery.go:277-291` — nil / API contract — The interface specifies returning an error for nil/invalid transport, but the method assigns nil without checking; a later `DeliverPacket` dereferences `r.transport` (`95`/`122`) and panics. — **Consequence:** a valid public call sequence crashes the process. — **Remediation:** reject nil before replacing the existing transport. Validate with `go test ./real`.

- [ ] **H‑TR‑1** Concurrent TCP sends interleave length prefix and payload — `transport/tcp.go:297-313` (`writePacketToConnection`) — concurrency / framing — `Send` reuses a shared `net.Conn` (`getOrCreateConnection`) and writes the 4‑byte length prefix and the body as two separate `Write` calls with no per‑connection write lock. Concurrent sends to the same peer can produce `prefixA, prefixB, bodyA, bodyB` on the wire. — **Consequence:** corrupted framing at the remote → desynchronized stream, dropped/garbled packets, connection reset. — **Remediation:** marshal prefix+payload into one buffer and write once, or guard each connection with a write mutex. Validate with `go test -race -tags nonet ./transport -run TCP -count=20`.

- [ ] **H‑TR‑2** Relay keepalive goroutine nil‑derefs the ticker after a remote disconnect — `transport/relay.go:466-499` — concurrency / panic — `handleDisconnect` (reachable from a remote `RelayPacketDisconnect`, `relay.go:363`) sets `rc.keepaliveTicker = nil` under lock but does **not** cancel `rc.ctx`. `runKeepaliveLoop` keeps running and evaluates `rc.keepaliveTicker.C` (498) without the lock; reading `.C` on a nil `*time.Ticker` panics (and races the writer). — **Consequence:** a remote relay can crash the process by disconnecting. — **Remediation:** exit the keepalive goroutine on disconnect (cancel a dedicated stop channel / the relay context) and read the ticker under lock. Validate with `go test -race -tags nonet ./transport -run Relay -count=50`.

### MEDIUM

- [ ] **M‑ASYNC‑1** `ProcessPreKeyExchange` panics on a nil exchange — `async/forward_secrecy.go:585-588` — nil safety — `exchange.PreKeys` is dereferenced with no `exchange == nil` guard. — **Consequence:** malformed/internal nil input crashes the process. — **Remediation:** return an error for nil input. Validate `go test ./async`.

- [ ] **M‑ASYNC‑2** Imported pre‑key backup with nil keypairs panics later — `async/prekeys.go:767-815` then `579-584`/`267-272` — nil safety — `ImportPreKeys` accepts unused entries with `KeyPair == nil`; a later `GetAvailablePreKey` → `copyKeyPair(nil)` dereferences `original.Public`. — **Consequence:** corrupted/untrusted backup causes a delayed crash. — **Remediation:** validate imported keypairs (reject nil) at import time. Validate `go test ./async`.

- [ ] **M‑MSG‑2** Nil transport silently marks messages `Sent`, preventing later delivery — `messaging/message.go:1239-1245` vs doc `466-467` — API contract — Docs say a nil transport leaves messages pending until configured, but `sendThroughTransport` sets `MessageStateSent` when `transport == nil`; `shouldProcessMessage` then skips them, so configuring a transport later never sends them. — **Consequence:** silent outbound message loss. — **Remediation:** keep state `Pending` when transport is nil. Validate `go test -tags nonet ./messaging -run Transport`.

- [ ] **M‑MSG‑3** Persisted `null` message panics on load — `messaging/message.go:830-844` (`LoadMessages`/`restoreMessagesFromSnapshot`) — nil safety — JSON like `{"messages":[null]}` yields a nil `*Message` that is dereferenced (`msg.ID`) without a check; store contents are persistent and may be corrupted. — **Consequence:** crash during startup/load. — **Remediation:** skip/reject nil messages and validate fields on restore. Validate `go test -tags nonet ./messaging -run LoadMessages`.

- [ ] **M‑FILE‑3** File transfer callbacks invoked while holding the transfer mutex — `file/transfer.go:557-567` (`recordTransferredBytes`), `696-720` (`completeLocked`), `897-939` (`SetAcknowledgedBytes`) — concurrency — `WriteChunk`/`ReadChunk` hold `t.mu` while invoking `progressCallback`/`completeCallback`/`ackCallback`; if a callback calls a safe getter (`GetProgress`/`GetState`/`GetAcknowledgedBytes`) it deadlocks on the same mutex. — **Consequence:** a user callback can permanently hang a transfer. — **Remediation:** copy callback + state under lock, unlock, then invoke. Validate `go test -race ./file -run Callback`.

- [ ] **M‑GRP‑2** Failed invitation stays pending and blocks retries — `group/chat.go:880-948` (`InviteFriend`) — logic / error handling — The pending invitation is stored before the network send; if `processInvitationPacket` fails the entry remains, and a retry is rejected as "friend already has a pending invitation". — **Consequence:** a transient send failure permanently blocks inviting that friend. — **Remediation:** add the pending entry only after a successful send, or delete it on error. Validate `go test -tags nonet ./group -run Invite`.

- [ ] **M‑AV‑3** RTP receive callback invoked under the transport read lock — `av/rtp/transport.go:314-360` (`handleIncomingVideoFrame`) — concurrency — The video receive callback is called while `ti.mu.RLock()` is held; a callback that re‑registers itself needs `ti.mu.Lock()` (`426-429`) and deadlocks. — **Consequence:** re‑entrant callback use hangs the RTP receive path. — **Remediation:** copy callback/session under lock, unlock, then invoke. Validate `go test -race ./av/rtp -run Callback`.

- [ ] **M‑AV‑4** System metrics remain stale after the last call ends — `av/metrics.go:297-301,398-401` (`updateSystemMetrics`) — logic — When `ActiveCalls` drops to 0, `updateSystemMetrics` returns early without resetting averages, quality buckets, duration, or `LastUpdate`. — **Consequence:** monitoring reports "0 active calls" with stale packet‑loss/quality/bitrate from prior calls, misleading adaptive logic. — **Remediation:** reset aggregate fields when the active count is zero. Validate `go test ./av -run Metrics`.

- [ ] **M‑AV‑5** Video processor RTP timestamps run at 90 Hz, not 90 kHz — `av/video/processor.go:1040-1043` (`generateTimestamp`) — logic — `UnixNano()/1000*90/1000000` reduces to `seconds*90` (≈90 ticks/s); RTP video requires a 90,000‑tick/s clock. The comment claims "90kHz". — **Consequence:** broken RTP timing/jitter estimation and A/V sync on the video processor path. — **Remediation:** compute `uint32(now.UnixNano() * 90000 / int64(time.Second))`. Validate `go test ./av/video -run Timestamp`.

- [ ] **M‑CAPI‑3** Exported C functions call `unsafe.Slice` on caller pointers without nil checks — `capi/toxcore_c.go:435` (`tox_self_get_public_key`), `452,456` (`tox_friend_add`), and similar — panic on bad input — After validating only the Tox handle, these slice C buffers directly; a NULL pointer with nonzero length makes `unsafe.Slice` panic and crash the shared library. c‑toxcore returns `false`/error instead. — **Consequence:** bad C input crashes the host process. — **Remediation:** nil‑check every C buffer before `unsafe.Slice`; return the documented error/`false`. Validate `go test ./capi -run 'Null|Nil|Pointer'`.

- [ ] **M‑TR‑3** Per‑packet goroutine spawn has no backpressure — `transport/udp.go:39` (`dispatchPacketHandler`), also `tcp.go:497`, `noise_transport.go:847` — resources / DoS — Every handled inbound packet launches a new goroutine with no worker‑pool bound or queue limit. — **Consequence:** a packet flood combined with a slow handler can exhaust goroutines/memory. — **Remediation:** dispatch through a bounded worker pool or bounded channel with drop/backpressure. Validate with a flood test asserting bounded goroutine count plus `go test -race -tags nonet ./transport`.

- [ ] **M‑TR‑4** IP parser stores `host:port` ASCII where raw IP bytes are expected — `transport/address_parser.go:283-290` (`buildNetworkAddress`) — data representation — It sets `Data: []byte(net.JoinHostPort(ip.String(), port))`, but `NetworkAddress.ToNetAddr`/`ToBytes` read `Data[:4]`/`Data[:16]` as raw IPv4/IPv6 bytes (`address.go:96-99,132-141`). For `127.0.0.1:33445`, consumers see `Data[:4]` = ASCII `"127."` → IP `49.50.55.46`. — **Consequence:** addresses parsed via `IPAddressParser` are misrouted/misserialized. — **Remediation:** store `ip.To4()`/`ip.To16()` in `Data`. Validate `go test -tags nonet ./transport -run AddressParser`.

- [ ] **M‑TR‑5** SOCKS5 UDP domain source resolves to a nil IP without error — `transport/socks5_udp.go:623-636` (`resolveDomainToUDPAddr`/`parseDomainHeader`) — error handling — When `net.LookupIP` fails, `ip` stays nil but `NewUDPAddr(nil, port)` is still returned and treated as a valid source address by `ReceiveUDP`. — **Consequence:** packets from unresolvable domain sources are accepted with a nil‑IP source, undermining source‑address assumptions. — **Remediation:** return an error when resolution fails. Validate `go test -tags nonet ./transport -run SOCKS5`.

- [ ] **M‑BOOT‑1** Clearnet port `0` reports an unusable bound address — `bootstrap/server.go:329` (`startClearnet`) — logic / lifecycle — `Config.ClearnetPort` documents `0` = OS‑chosen port, but the server records `s.clearnetAddr` as `0.0.0.0:0` rather than the actual bound UDP port. — **Consequence:** `GetClearnetAddr()` returns an address clients cannot bootstrap to. — **Remediation:** read the real local UDP address after binding (or reject port 0). Validate `go test ./bootstrap`.

- [ ] **M‑FACT‑1** "Concurrent‑safe" factory leaks config access outside the lock — `factory/packet_delivery_factory.go:217` vs `325` — concurrency — `CreatePacketDelivery` copies only the `defaultConfig` *pointer* under `RLock`, then reads its fields after unlocking, while `SwitchToSimulation` mutates the pointed‑to struct under a different lock scope. — **Consequence:** data race + inconsistent implementation/config selection. — **Remediation:** copy config *values* under lock before use. Validate `go test -race ./factory`.

- [ ] **M‑REAL‑3** Failed `AddFriend` leaves a stale local registration — `real/packet_delivery.go:320-336` — error handling / state — `r.friendAddrs[friendID] = addr` is written before `transport.RegisterFriend`; on transport rejection the method returns an error but never removes the cache entry. — **Consequence:** stats/broadcast target selection believe the friend is registered, so later sends fail unexpectedly. — **Remediation:** register with the transport first, or roll back the map entry on error. Validate `go test ./real`.

- [ ] **T‑1** Test suite does not pass on the default branch — `group/chat_test.go:797`, `messaging/encryption_test.go:638`, `messaging/lifecycle_test.go:113`, `messaging/manager_test.go:103,141` — testing — `go test -tags nonet -race ./...` yields 2 failing packages: `group` hangs to the 10‑min timeout (root cause C‑GRP‑1), and `messaging` fails because tests were not updated after the fail‑closed E2EE migration (production now wraps `ErrNoEncryption` in `ErrOutboundPlaintextBlocked` and blocks plaintext sends, while the tests assert the old exact error string / that the message is sent). — **Consequence:** CI is red/hangs; real regressions can hide behind known failures. — **Remediation:** fix C‑GRP‑1; update the messaging tests to assert `errors.Is(err, ErrNoEncryption)` (not exact string) and the blocked‑send behavior. Validate `go test -tags nonet -race ./group ./messaging`.

### LOW

- [ ] **L‑1** `av/metrics.go:421` — performance/overflow (theoretical) — `totalBitrate += uint64(metrics.AudioBitRate + metrics.VideoBitRate)` adds two `uint32` before widening; only overflows above ~4.29 Gbps combined, which is unreachable for realistic bitrates. — **Remediation:** widen each operand first: `uint64(a) + uint64(b)`. Validate `go test ./av -run Metrics`.

- [ ] **L‑2** `transport/version_negotiation.go:201-203` (`NewVersionNegotiator`) — API/panic (local misuse) — Falls back to `supported[0]` without checking `len(supported) == 0`; an empty slice panics. — **Remediation:** reject empty input or default to `ProtocolLegacy`. Validate `go test -tags nonet ./transport -run VersionNegotiator`.

- [ ] **L‑3** `toxav.go:927-937,991-1001` (`AudioSetBitRate`/`VideoSetBitRate`) — API contract — Setters update only local call fields; comments acknowledge no `BitrateControlPacket` is sent and the encoder/processor is not reconfigured, so the README's "adjust bitrates at runtime" is only partially realized. — **Remediation:** reconfigure the processor encoder and emit bitrate signaling to the peer. Validate `go test ./... -run 'BitRate|Bitrate'`.

- [ ] **L‑4** `toxcore_friends.go:277-351` (`cloneReflectValue`, complexity 16) — data aliasing (documented best‑effort) — The reflective deep copy cannot set unexported struct fields (`CanSet()==false`), so a `Friend.UserData` containing unexported pointer fields is shallow‑shared between the original and the copy returned by `GetFriends`. No reachable public setter for arbitrary `UserData` was found, so impact is currently theoretical. — **Remediation:** document the unexported‑field limitation in the GoDoc; no code change required. 

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total functions | 1,317 (+3,046 methods) |
| Functions above complexity 15 | 1 (`cloneReflectValue`, 16) |
| Functions above complexity 10 | 2 |
| Avg cyclomatic complexity | 3.5 |
| Doc coverage (overall) | 93.5% (functions 98.8%, methods 92.4%, types 92.4%) |
| Duplication ratio | 0.5% (470 lines, largest clone 17 lines) |
| Test pass rate (packages) | 32 / 34 with tests pass; `group` (deadlock timeout) + `messaging` (stale tests) FAIL |
| `go vet ./...` warnings | 0 |

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|----------------|
| `transport/parser.go ParseNodeEntry` slice indexing on untrusted bytes | All offsets non‑negative and length‑guarded before every slice. |
| `transport/socks5_udp.go` IPv4/IPv6/domain header slicing | Lengths checked before each access. |
| `dht/local_discovery.go handlePacket` | Guards `len(data) >= 34` before indexing. |
| `dht/gossip_bootstrap.go`, `dht/relay_storage.go`, `dht/mdns_discovery.go` | Length prefixes / capacity checks precede slicing; map writes under mutex. |
| `transport/noise_transport.go` handler dispatch panic | Already wrapped in `recover()` (the old GAPS.md "handler hardening" gap is fixed). |
| `group` broadcast peer‑map iteration race (old GAPS.md) | `collectOnlinePeerJobs` now iterates under `g.mu.RLock()`; peer callbacks routed through `safeInvokeCallback` (goroutine + recover). Superseded by C‑GRP‑1 (a different, real deadlock). |
| `async/obfs.go EncryptPayload` nonce reuse | Uses `crypto/rand` nonce + AES‑GCM correctly. |
| `crypto/keystore.go` path traversal | Filename validation rejects separators/traversal. |
| `file/manager.go` incoming path traversal | `filepath.Base` + defensive revalidation in `Start`. |
| `file/manager.go handleFileDataAck` ACK overrun | `SetAcknowledgedBytes` rejects ACKs beyond sent/file size and regressions. |
| `toxcore_persistence.go` snapshot bounds | Length‑prefixed reads via `ensureBytes`; friend count bounded by remaining bytes. |
| `iteration_pipelines.go` goroutine leak | Non‑blocking cap‑1 trigger sends; `Stop` cancels context before `Wait`. |
| `av/audio/effects.go` gain overflow | Gain range‑checked and int16‑clamped. |
| `av/rtp/packet.go` jitter buffer race | Packet count capped, protected by mutex. |
| SSRC generation | Uses `crypto/rand`, not `math/rand`. |
| `toxnet` double‑close channel panic | `markClosed` gates close paths; broadcast only after first successful close. |
| `toxnet` packet buffer aliasing | UDP buffers copied before enqueue; writes copy user buffers. |

## Remaining Scope (if session ended before completion)
Complete — a full pass covered every production package. `examples/`, `simulation/`,
`testnet/`, and `cmd/` demo programs were intentionally excluded from finding generation
(developer tooling, not part of the shipped library) and reviewed only to confirm
reachability of the findings above.

| Package | Status | Notes |
|---------|--------|-------|
| (all production packages) | Audited | See Coverage Log |
| examples/, simulation/, testnet/, cmd/ | Excluded | Non‑shipped demo/tooling; reviewed for reachability only |
