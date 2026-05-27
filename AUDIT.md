# toxcore-go End-to-End Bug-Hunting Audit

## Completion Status

- [x] C-ASYNC-1 — Recursive RWMutex deadlock in async message retrieval
- [x] H-NOISE-1 — PSK 0-RTT handshake responder cannot decrypt initiator payload
- [x] M-CRYPTO-1 — Unbounded NonceStore growth between cleanup ticks
- [ ] M-NET-1 — Concrete `net.UDPAddr` use violates documented architectural rule
- [x] L-BOOT-1 — `bootstrap.Server` can panic with `close of closed channel`
- [x] L-ASYNC-1 — `sendQueuedMessages` re-queue ordering double-prepends pending slice

**Audit target:** `github.com/opd-ai/toxcore` @ working tree (Go 1.25.0, toolchain go1.25.8)
**Scope:** Source code only — no modifications were made.
**Methodology:** `go vet ./...`, `go test -race -tags nonet ./...`, structured grep sweeps for known anti-patterns (recursive locking, concrete `net.*` types, `panic`, `math/rand`, `InsecureSkipVerify`, unbounded maps, double-`close(chan)`, missing `defer` cleanup, integer parsing without bounds, unauthenticated I/O, lock-order inversions), and targeted code review of high-complexity functions.

---

## 1. Project Profile

| Item | Value |
| --- | --- |
| Module path | `github.com/opd-ai/toxcore` |
| Go version | 1.25.0 / toolchain go1.25.8 |
| Top-level packages | `toxcore` (root), `async`, `av`, `bootstrap`, `capi`, `crypto`, `dht`, `factory`, `file`, `friend`, `group`, `interfaces`, `limits`, `messaging`, `mocks` (test), `net`, `noise`, `real`, `simulation`, `testnet`, `toxnet`, `transport`, plus subpackages under `av/`, `transport/`, `examples/`, `cmd/` |
| LoC (approximate) | ~42 k Go (excluding `_test.go`) |
| Build tags | `nonet` excludes network-dependent tests in CI |
| Key dependencies | `flynn/noise`, `golang.org/x/crypto`, `pion/rtp`, `opd-ai/magnum` (Opus), `sirupsen/logrus`, `stretchr/testify` |

---

## 2. Audit Scope & Methodology

| Check | Tool / Approach | Result |
| --- | --- | --- |
| Static analysis | `go vet ./...` | **CLEAN** (0 diagnostics) |
| Race detector + unit tests | `go test -race -tags nonet -timeout 120s ./...` | **2 packages FAIL** (`async`, `noise`); all others PASS |
| Format / lint | `gofmt -l .` | (not re-run during audit; assumed clean per CI) |
| Network anti-patterns | grep for concrete `net.UDPAddr`/`net.TCPConn` etc. | 4 violations, internal-only (see M-NET-1) |
| Crypto hygiene | review of `crypto/`, `noise/`, `async/forward_secrecy.go`, `async/obfs.go` | One protocol test failure (H-NOISE-1) |
| Concurrency | manual review of every `sync.*Mutex` site near `defer`-driven unlocks; lock-ordering | One recursive RWMutex (C-ASYNC-1) and one Close-double-close risk (L-BOOT-1) |
| Channel / goroutine leaks | grep for `WithCancel`/`WithTimeout`, `close(chan)`, `wg.Wait()` | All checked sites properly cancel/wait |
| Loop-var capture | grep `go func()` inside `for` loops | Not applicable — Go 1.22+ loopvar semantics, project requires 1.25.0 |
| Path traversal | review `file/transfer.go::ValidatePath` and call order | OK (`validateAndSanitizePath` runs before `openTransferFile`) |
| Wire-format parsing | review STUN, SOCKS5 length-prefixed parsers | All length-bounded; no OOB read found |

### 2.1 Per-package coverage log

`vet` = `go vet`; `race` = race-enabled tests under `nonet`; `read` = manual code review of risky funcs identified by complexity / grep hits; `grep` = pattern sweep for the categories above.

| Package | vet | race | read | grep | Notes |
| --- | :-: | :-: | :-: | :-: | --- |
| `toxcore` (root) | ✅ | ✅ | ✅ | ✅ | `toxav.go::Call`, bitrate APIs reviewed (historical F-TOXAV-H1/H2 confirmed fixed) |
| `async` | ✅ | ❌ | ✅ | ✅ | **C-ASYNC-1** deadlock in `RetrieveObfuscatedMessages`; `manager.go::sendQueuedMessages` lock-order discipline OK |
| `av`, `av/audio`, `av/video`, `av/rtp` | ✅ | ✅ | ✅ | ✅ | Effect mutexes in `av/audio/effects.go` present (historical F-AV-H3 fixed) |
| `bootstrap` | ✅ | ✅ | ✅ | ✅ | **L-BOOT-1** potential double `close(stopChan)` on partial-startup-then-Stop |
| `capi` | ✅ | (cgo, n/a) | grep only | ✅ | cgo bindings; tests not run (no cgo toolchain check needed for scope) |
| `crypto` | ✅ | ✅ | ✅ | ✅ | `keystore.go::reencryptWithNewKey` 3-phase commit with rollback verified; `replay_protection.go` cleanup is O(n) (historical M3 claim stale); **M-CRYPTO-1** unbounded `NonceStore.nonces` between cleanup ticks |
| `dht` | ✅ | ✅ | ✅ | ✅ | `local_discovery.go`, `mdns_discovery.go` use concrete `net.UDPAddr` (see M-NET-1) |
| `factory` | ✅ | ✅ | grep only | ✅ | — |
| `file` | ✅ | ✅ | ✅ | ✅ | Path-traversal hardened; `Start()` orders `validate → sanitize → open` correctly |
| `friend` | ✅ | ✅ | grep only | ✅ | — |
| `group` | ✅ | ✅ | ✅ | ✅ | `sender_key.go::Encrypt` refuses past `maxMessageCounter` (historical F-GROUP-H1 mitigated) |
| `interfaces` | ✅ | ✅ | grep only | ✅ | — |
| `limits` | ✅ | ✅ | grep only | ✅ | — |
| `messaging` | ✅ | ✅ | grep only | ✅ | — |
| `net`, `noise/net` helpers | ✅ | ✅ | grep only | ✅ | — |
| `noise` | ✅ | ❌ | ✅ | ✅ | **H-NOISE-1** `TestPSKHandshakeInitiatorResponder` AEAD MAC failure |
| `real` | ✅ | ✅ | grep only | ✅ | — |
| `simulation` | ✅ | ✅ | grep only | ✅ | — |
| `toxnet` | ✅ | ✅ | ✅ | ✅ | `conn.go::waitForDataSignal` release/reacquire pattern correct; F-TOXNET-H1 (timer leak) confirmed fixed |
| `transport` (+ subpackages) | ✅ | ✅ | ✅ | ✅ | STUN/SOCKS5 length-bounded; `worker_pool`, `udp`, `tcp`, `relay_mux` use `sync.Once` for Close. Several internal `net.UDPAddr` constructions (M-NET-1). |
| `examples/*` | n/a | n/a | spot | ✅ | `math/rand` only used here, with justifying comments |
| `cmd/gen-bootstrap-nodes`, `testnet/*` | ✅ | not run | spot | ✅ | — |

---

## 3. Goal-Achievement Summary

The README and `doc.go` advertise a complete pure-Go Tox stack with DHT, friend messaging, file transfer, group chat, ToxAV (audio/video), asynchronous offline messaging with forward secrecy + identity obfuscation, multiple privacy transports (Tor/I2P/Nym/Lokinet), and Noise-IK handshakes including a PSK 0-RTT resumption profile.

**Working as advertised** (verified by passing race-tests and review):

- DHT discovery, bootstrap, k-bucket maintenance
- Friend management, 1:1 messaging
- File transfer (incl. path-traversal hardening)
- Group sender-key messaging (with counter-exhaustion safeguard)
- Audio/video pipeline (effects mutexes in place; `Call` API hold-locks correctly)
- Synchronous Noise-IK handshake, persistent crypto keystore (with rotation rollback)
- Bootstrap server, multi-network transport plumbing (UDP/TCP/relay)

**Not working as advertised** (see findings below):

- **Asynchronous offline message retrieval is functionally broken** at runtime: the very first call to `RetrieveAsyncMessages`/`RetrieveObfuscatedMessages` deadlocks in production builds (C-ASYNC-1). The race detector test reproduces this deterministically.
- **PSK 0-RTT Noise handshake is non-functional**: responder AEAD validation fails on every initiator message (H-NOISE-1).

These are the only **functional regressions** found. Everything else passes vet and race-tests under `-tags nonet`.

---

## 4. Findings

### CRITICAL

#### C-ASYNC-1 — Recursive RWMutex deadlock in async message retrieval

- **File:** `async/client.go`
- **Lines:** `405–422` (entry), `455` (intermediate), `993–1003` (offender)
- **Reproduction:** `go test -race -tags nonet -run TestObfuscatedMessageRetrieval ./async` deadlocks; goroutine dump shows `sync.runtime_SemacquireRWMutexR` reached through `RetrieveObfuscatedMessages → retrieveMessagesForEpoch → findAvailableStorageNodes → findStorageNodes → collectCandidateNodes`.
- **Data flow:**
  1. `RetrieveObfuscatedMessages` acquires the **write lock**: `ac.mutex.Lock(); defer ac.mutex.Unlock()` (line 406–407).
  2. Within the locked region it calls `ac.retrieveMessagesForEpoch(epoch)` (line 416).
  3. `retrieveMessagesForEpoch → findAvailableStorageNodes → findStorageNodes` reaches `collectCandidateNodes` (line 993), which itself calls `ac.mutex.RLock(); defer ac.mutex.RUnlock()` (line 994–995).
  4. Go's `sync.RWMutex` is **non-recursive**: an `RLock` from the same goroutine that already holds `Lock` blocks forever. Additionally, `collectMessagesFromNodes` at line 455 acquires `RLock` on the same mutex — same deadlock from a second path.
- **Impact:** Any caller of `RetrieveAsyncMessages` (the only public API for receiving offline messages — `manager.go` invokes it from background polling) hangs forever, taking with it any caller waiting on the result. The headline "asynchronous offline messaging with forward secrecy" feature is unusable.
- **Confirmation that this is not a false positive:** The doc comment at line 992 explicitly says *"Must be called while holding `ac.mutex` (at least RLock)"* — i.e. the author intended `collectCandidateNodes` to be a leaf-helper that assumes the caller already holds the lock. The implementation contradicts the contract: it re-acquires the lock itself. The single caller (`findStorageNodes`) is reached only from paths that already hold the lock (write or read), so every real-world call path deadlocks.
- **Remediation outline (do not commit during audit):**
  - Remove the `RLock/RUnlock` from `collectCandidateNodes` (and add an `// invariant: caller holds ac.mutex` comment) so it honours its documented precondition, **or**
  - Replace the outer `Lock`/`RLock` callers with a `Lock`-free design that snapshots `ac.storageNodes` into a local slice while holding a short critical section, then runs the rest of the algorithm lock-free. The second option is preferable because the existing `collectMessagesFromNodes` makes network calls under the lock, which serialises every retrieval through a single goroutine.

### HIGH

#### H-NOISE-1 — PSK 0-RTT handshake responder cannot decrypt initiator payload

- **File:** `noise/psk_resumption.go`
- **Lines:** `425–446` (config), `448–520` (WriteMessage / processInitiatorMessage / processResponderMessage)
- **Reproduction:** `go test -race -tags nonet -run TestPSKHandshakeInitiatorResponder ./noise` fails deterministically with `PSK responder read failed: chacha20poly1305: message authentication failed`.
- **Data flow:**
  1. Both sides construct `noise.Config` via `createPSKNoiseConfig` (line 417). The config uses `Pattern: noise.HandshakeIK` with `PresharedKey: config.PSK[:]` and `PresharedKeyPlacement: 2` (default — i.e. `IKpsk2`).
  2. Initiator calls `WriteMessage(earlyData, nil)` which invokes `noise.HandshakeState.WriteMessage(nil, earlyData)` for the first IK message and returns the encrypted bytes.
  3. Responder calls `WriteMessage(nil, msg1)` which on line 493 calls `psk.state.ReadMessage(nil, receivedMessage)` and fails the AEAD authentication.
- **Likely root cause(s):** With `IKpsk2`, the PSK token mixes into the chaining key during the *second* message (responder → initiator), not the first. The first AEAD key on `-> e, es, s, ss` is derived solely from DH outputs and does **not** include the PSK. Therefore both sides must end up with identical chaining keys after `es, ss`. The MAC failure most plausibly indicates that initiator and responder derive different static keys: `createPSKNoiseConfig` (line 422) copies `keyPair.Private` into a 32-byte `DHKey.Private`, while `flynn/noise` expects the X25519 *DH* keypair, not the Ed25519 signing key. If `keyPair` is the Tox identity Ed25519 keypair (or a Curve25519 public derived elsewhere), the responder's `PeerStatic` (set by the initiator's view of the responder's identity) will not match what `noise` computes from the responder's `StaticKeypair.Private`. See also the historical `crypto.KeyPair` confusion called out in `BACKLOG_ANALYSIS.md`.
- **Impact:** The "Noise PSK 0-RTT session resumption" feature is non-functional. Any production code path that opts into PSK resumption will fail every handshake and degrade to a plain Noise-IK fallback at best, or simply drop the connection.
- **Why this is not a false positive:** The test exercises the public `WriteMessage`/`ReadMessage` API exactly as a caller would; no mocking is involved. The `chacha20poly1305: message authentication failed` error comes from inside `flynn/noise` and only arises when the symmetric keys diverge between peers.
- **Remediation outline:** Verify that `keyPair` passed into `createPSKNoiseConfig` is the X25519 keypair used by the static-key half of Noise-IK (not Ed25519). If `crypto.KeyPair` is Ed25519, convert with `extra25519` or equivalent before calling `noise.NewHandshakeState`. Add a sanity self-test (initiator+responder in the same process) to CI.

### MEDIUM

#### M-CRYPTO-1 — `NonceStore` grows unbounded between cleanup ticks

- **File:** `crypto/replay_protection.go`
- **Lines:** `~270–310` (struct, `Store`, `cleanup`, `Close`)
- **Data flow:** `NonceStore.Store` inserts each accepted nonce into `ns.nonces` (a `map[[24]byte]time.Time`). The background goroutine started in `NewNonceStore` calls `cleanup()` at a fixed interval (default 10 minutes) to evict entries older than the replay window. Between ticks the map size is bounded only by *received traffic* — there is no hard cap.
- **Impact:** A malicious or buggy peer that can submit valid-looking unique-nonce ciphertexts (or simply a high-throughput legitimate workload) can consume O(messages-per-10-minutes × 32 bytes) of process memory before the next cleanup. Coupled with C-ASYNC-1, which forces all retrieval into a single critical section, a saturated node can be OOM-killed.
- **Why this is real:** The replay window is intentionally large to tolerate clock skew (no smaller window will help). The map has no `len(ns.nonces) >= cap` guard.
- **Remediation outline:** Enforce a hard maximum (e.g. 100 k entries) and either trigger an immediate `cleanup()` when crossed or reject new nonces with a logged warning. Optionally use a counter-window/bitmap for the common case.

#### M-NET-1 — Concrete `net.UDPAddr` use violates documented architectural rule

- **Files / lines:**
  - `dht/local_discovery.go:~80, ~140` (multicast listener / sender)
  - `dht/mdns_discovery.go:~95, ~155, ~190`
  - `transport/socks5_udp.go:328, 339, 359, 571, 581, 595` (SOCKS5 UDP-associate parsing returns `&net.UDPAddr{…}`)
  - `transport/stun_client.go:322, 337, 358` (STUN response parsing)
  - `transport/address.go` (helper conversions)
- **Stated rule (`doc.go`, custom instructions, README "Networking Best Practices"):** *"Never use `net.UDPAddr`, `net.IPAddr`, or `net.TCPAddr`. Use `net.Addr` only instead."*
- **Impact:** Constructions are internal-only (returned as `net.Addr` from the helper boundary), so callers see the interface as required — there is no concrete-type assertion downstream. The functional impact today is **zero**, but the divergence between rule and implementation:
  - Blocks future swap of these subsystems for mock transports in tests.
  - Confuses contributors who try to follow the documented rule elsewhere.
- **Why this is not a false positive:** The rule is stated unconditionally; the violations exist; mocking these subpackages currently requires patching their internals rather than substituting transports.
- **Remediation outline:** Construct addresses via a small `transport.NewIPAddress(ip, port) net.Addr` factory (already partly present in `transport/address.go`) and return its result, hiding the concrete type behind the interface boundary even within the subpackages.

### LOW

#### L-BOOT-1 — `bootstrap.Server` can panic with `close of closed channel` on a Start-failure-then-Stop sequence

- **File:** `bootstrap/server.go`
- **Lines:** `196–211` (`stopRunningGoroutines` called from partial-startup failure paths), `215–235` (`Stop`)
- **Data flow:** If `Start()` fails partway through (e.g. `startI2P` returns an error after I2P listener has already been created), the recovery path calls `s.stopRunningGoroutines()`, which executes `close(s.stopChan)`. If the caller then invokes `Stop()` — for example because tests defer it unconditionally, or because the wrapping process treats `Start` errors as transient — line 227 closes the same channel again and panics. The `s.running` guard at line 219 does **not** protect against this because `s.running` is only set to `true` *after* a fully successful start (and not reset by `stopRunningGoroutines`).
- **Impact:** Process-crashing panic on a recoverable startup failure; cleanest fix would be to wrap the close in `sync.Once`.
- **Why this is real:** The two close sites are reachable without any guard between them (`stopRunningGoroutines` runs while `s.running` is still false, and `Stop` short-circuits only when `!s.running`).
- **Remediation outline:** Wrap channel close in `sync.Once`, or set `s.stopped = true` (and check it) at the end of `stopRunningGoroutines`.

#### L-ASYNC-1 — `sendQueuedMessages` re-queue ordering double-prepends pending slice

- **File:** `async/manager.go`
- **Lines:** `881–885`, `906–910`
- **Data flow:** When a re-queue happens, the code does `am.pendingMessages[friendPK] = append(queued, am.pendingMessages[friendPK]...)` (line 884) which puts the older `queued` slice ahead of any newly arrived messages — correct for FIFO. But the failed-resend path at line 908 uses `append(failed, am.pendingMessages[friendPK]...)`, dropping the rest of `queued` that *did* succeed and only re-queueing the failures. That is fine; however, if `signalPreKeyReady` fires *between* the timeout branch (line 879) and `am.mutex.Lock()` (line 882), the `select { case <-ch }` arm misses the signal, falls into the timeout branch, and we re-queue messages that could have been sent immediately. The pre-key signal channel is consumed once and recreated — losing it costs `preKeyExchangeTimeout`-worth of latency, not a deadlock.
- **Impact:** Worst-case latency tail for queued messages equal to `preKeyExchangeTimeout` when pre-keys arrive in a narrow race window. No data loss.
- **Remediation outline:** Use a `time.NewTimer` + `defer timer.Stop()` so the `select` can be re-checked deterministically, or move the `select` inside the mutex and re-check signal state under the lock before timing out.

---

## 5. Metrics Snapshot

| Metric | Value |
| --- | --- |
| Packages built cleanly | All (vet, build) |
| Packages with race-test failures | 2 (`async`, `noise`) |
| Findings: CRITICAL | 1 |
| Findings: HIGH | 1 |
| Findings: MEDIUM | 2 |
| Findings: LOW | 2 |
| Historical HIGH findings verified as fixed/mitigated in current tree | 5 (F-AV-H3, F-TOXAV-H1, F-TOXAV-H2, F-GROUP-H1, F-TOXNET-H1) |
| Source modifications made | **0** (audit-only) |

---

## 6. False Positives Considered and Rejected

These looked suspicious but turned out to be benign on closer inspection. Documented here to save future auditors time.

| Candidate | File | Why rejected |
| --- | --- | --- |
| Replay-protection cleanup is O(n²) (historical F-CRYPTO-M3) | `crypto/replay_protection.go::cleanup` | Current implementation iterates the map exactly once with `delete()` per stale entry. `delete` during `range` over a `map` is documented-safe in Go and is O(1) amortised. |
| Loop-variable capture in `go func()` inside `for` | dht/transport/async (~20 sites) | Project requires Go 1.25 (`go.mod` line 3); Go 1.22+ already gives each iteration its own variable. |
| `keystore.go::reencryptWithNewKey` corruption on partial failure | `crypto/keystore.go:380–470` | Reviewed: 3-phase commit (write `.reencrypt.tmp` → rename original to `.preencrypt.tmp` → rename new to final) with explicit rollback paths that restore from backups on any failure. Correct. |
| Lost-wakeup in `toxnet.waitForDataSignal` | `toxnet/conn.go:374–393` | Captures the notify channel under lock, then unlocks and selects on it. The notifier holds the same lock when replacing the channel, so no signal can be lost. Standard release-and-reacquire pattern. |
| Init-time `panic()` calls in `dht/mdns_discovery.go:42,45`, `transport/nat.go:26`, `crypto/secure_memory.go:48` | various | All execute at package init and only on programmer errors (impossible-to-reach constants); they fail fast and do not affect runtime stability. |
| `math/rand` in `examples/` | `examples/*.go` | Demos, not production paths; each site has a comment explaining why `crypto/rand` is unnecessary (display jitter etc.). |
| STUN attribute parser infinite loop on `attrLength == 0` | `transport/stun_client.go:267–296` | Per iteration the offset advances by 4 (header) + `attrLength` + padding; with `attrLength == 0` the loop still advances by 4 bytes/iter and terminates once `offset+4 > len(attributes)`. |
| SOCKS5 UDP parser OOB read on attacker-supplied `domainLen` | `transport/socks5_udp.go:344–355` | The code allocates `make([]byte, domainLen+2)` and reads exactly that many bytes via `readWithTimeout` before indexing; no read past slice end is possible. |
| File-transfer directory traversal via peer-supplied name | `file/transfer.go::Start → validateAndSanitizePath → openTransferFile` | `validateAndSanitizePath` runs **before** `openTransferFile`, calls `filepath.Base` check then `ValidatePath` which rejects absolute paths and `..` components. For outgoing transfers the path is caller-supplied (trusted). |

---

## 7. Remaining Scope / Out-of-Scope

- `capi/` C bindings were inspected by grep only — race-tests for cgo paths require a C toolchain and the `nonet` constraint excludes most of them. A targeted cgo + LeakSanitizer pass would be a useful follow-up.
- `examples/` and `testnet/internal` are demo / harness code — only spot-checked.
- Tor/I2P/Nym/Lokinet integration paths require external daemons; they were not exercised at runtime under `-tags nonet`.
- Performance: no benchmarks were rerun. The `collectMessagesFromNodes` design (network calls under a single write lock — pre-deadlock) is a structural performance smell worth revisiting alongside C-ASYNC-1.

---

## 8. Reproduction Commands

```bash
go vet ./...
go test -race -tags nonet -timeout 120s ./...
# To reproduce the two failures in isolation:
go test -race -tags nonet -timeout 90s -run TestObfuscatedMessageRetrieval ./async
go test -race -tags nonet -timeout 60s -run TestPSKHandshakeInitiatorResponder ./noise
```
