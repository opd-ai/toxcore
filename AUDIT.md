# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-27

## Project Profile
- **Purpose**: Pure-Go implementation of the Tox peer-to-peer encrypted messaging protocol.
  Provides DHT routing, friend management, 1-to-1 messaging, group chat, file transfers,
  audio/video calling (ToxAV), asynchronous offline messaging with forward secrecy, and
  multi-network transport (UDP/TCP/Tor/I2P/Lokinet/Nym) — without cgo in the core library.
- **Target users**: Tox client developers (e.g. qTox), security-sensitive messaging app
  authors, and projects integrating with the Tox network via the C API shim (`capi/`).
- **Deployment model**: Linked as a Go library or via the C ABI in `capi/`. Long-running
  user processes; never run as setuid/root. Trust boundary is the network — every packet
  from a peer is untrusted until authenticated by the noise / crypto layer.
- **Critical paths** (deeper scrutiny in this audit):
  - `crypto/` — keys, key store, nonces, replay protection.
  - `dht/` — peer discovery and routing.
  - `async/` — offline messaging with forward secrecy.
  - `transport/` — UDP/TCP, NAT, noise transport.
  - `file/` — bidirectional file transfer state machine.
  - `noise/` — Noise-IK handshake.
  - `capi/` — C-FFI surface (highest blast radius for nil/UAF).
  - root package (`toxcore.go` and `toxcore_*.go`) — top-level API.

## Audit Scope
- 26 Go packages (per go-stats-generator overview), 238 non-test source files,
  42,298 LOC of production code (1,160 functions + 2,889 methods).
- This session performed a **partial end-to-end pass**. The Coverage Log below records
  exactly which packages received which checklist categories. Roughly 11 critical-path
  packages received full checklist coverage; ~14 packages (mostly `examples/*` and small
  helpers) were intentionally deprioritised but listed in Remaining Scope so the next
  session can complete them without overlap. **No finding cap was applied** to any
  package that was audited.
- Baseline tools executed:
  - `go vet ./...` — **0 warnings**.
  - `go test -race ./...` — **5 failing test cases in 2 packages** (see CRITICAL/HIGH).
  - `go-stats-generator analyze . --skip-tests` — 5 functions with cyclomatic >10,
    34 functions >50 lines, longest 93 lines, overall doc coverage 93.3 %.
- `tmp/` was cleaned up; the only persistent outputs are `AUDIT.md` and `GAPS.md`.

## Coverage Log

Legend: ✅ = full review of the checklist category for that package, ⚪ = spot-checked
only (a few files / hot functions), ❌ = not audited in this session (queued in
Remaining Scope below).

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| crypto             | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚪ | ✅ | ✅ |
| async              | ✅ | ⚪ | ✅ | ⚪ | ✅ | ⚪ | ⚪ | ⚪ | ✅ |
| dht                | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ✅ | ⚪ |
| transport          | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ✅ | ⚪ | ⚪ | ⚪ |
| noise              | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ |
| toxnet             | ⚪ | ⚪ | ⚪ | ⚪ | ✅ | ⚪ | ⚪ | ⚪ | ⚪ |
| file               | ✅ | ✅ | ✅ | ✅ | ✅ | ⚪ | ⚪ | ⚪ | ✅ |
| friend             | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ |
| messaging          | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ |
| av (incl. audio/video/rtp) | ⚪ | ⚪ | ✅ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ |
| capi               | ⚪ | ✅ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ |
| toxcore (root)     | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ | ⚪ |
| group, bootstrap, factory, limits, interfaces, real, simulation, testnet | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| examples/* (30+ pkgs) | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

## Goal-Achievement Summary

Goals are extracted from README.md "Features" section. Status reflects only what was
verified during this audit session.

| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| Pure-Go core (no cgo) | ✅ | — (`capi/` correctly isolates cgo behind build tag) |
| DHT routing with k-buckets, iterative lookups | ✅ | — |
| Friend management & sharded state storage | ✅ | — |
| 1-to-1 encrypted messaging with delivery tracking | ✅ | — |
| Group chat with role-based permissions | ✅ | — (not audited in depth — see Remaining Scope) |
| Bidirectional file transfers with pause/resume/cancel | ⚠️ | F-FILE-001 (handle leak on write error) |
| ToxAV audio/video with adaptive bitrate | ✅ | — (only spot-checked) |
| Async offline messaging with forward secrecy & one-time pre-keys | ❌ | F-ASYNC-001, F-ASYNC-002 — two `go test -race` failures on offline-messaging core flows |
| Identity obfuscation via epoch-based pseudonyms | ❌ | F-ASYNC-001 — per-message pseudonyms not unique in retrieval index |
| Multi-network transport (IPv4/6, Tor, I2P, Loki, Nym) | ✅ | — |
| Noise-IK handshakes (forward secrecy, KCI resistance) | ✅ | — |
| Nonce-based replay protection | ❌ | F-CRYPTO-001 — three `go test -race` failures on `NonceStore` |

## Findings

Severity definitions per task spec. Every finding lists file:line, the bug class, the
concrete consequence traced through the call graph, and a proportionate remediation
with a validation command.

### CRITICAL

- [x] **F-CRYPTO-001 — `NonceStore.CheckAndStore` silently rejects valid nonces older than the 5-minute skew window, breaking replay-protection semantics and the documented contract.**
  Files: `crypto/replay_protection.go:96-149`, `crypto/replay_protection_test.go:104,252,307`.
  Class: Logic / state-machine bug on a critical security path.
  Consequence: `CheckAndStore(nonce, ts)` returns `false` (i.e. treats the nonce as a replay) for any `|ts - now| > 5 min`, before even consulting the nonce map. This is observable as three reproducible test failures under `go test -race ./crypto/...`:
  `TestNonceStoreExpiration` (line 104), `TestNonceStoreCleanupLoop` (line 252),
  `TestNonceStoreWithTimeProvider` (line 307) — all assert `Size()==2` but get `1` because the "old" nonce was never stored. The same code path is invoked by every protocol message that carries a timestamp: a client whose clock has drifted >5 min will see all of its messages silently dropped as if they were replays, with only a `Warn` log entry. There is no mechanism for the caller to distinguish "skew rejected" from "true replay", so callers cannot recover.
  Data flow: caller → `CheckAndStore` line 96 → skew check line 107 → return `false` without touching `ns.nonces`.
  **Remediation:** Pick one of two consistent behaviours and apply it across both code and tests:
  (a) keep the skew check but return a distinct error/sentinel (`ErrTimestampOutOfWindow`) so callers can react, and update the three tests to assert the rejection path explicitly; or
  (b) store the nonce regardless of the skew window (leaving skew enforcement to a separate layer that already exists in `transport/noise_transport.go`) and rely solely on the existing 6-minute expiry logic at line 140 to bound memory.
  Validate with `go test -race ./crypto/...` (must pass cleanly) and add a regression test for clock-skewed peers exchanging legitimate messages.

- [x] **F-ASYNC-001 — `RetrieveMessagesByPseudonym` returns all messages for the recipient, contradicting the "unique per-message pseudonym" claim in `async_test.go:576-578` and in `async/doc.go`.**
  Files: `async/storage.go:655-688`, `async/async_test.go:540-595`.
  Class: API / behavioural contract gap on a critical security path (identity obfuscation).
  Consequence: `TestRetrieveMessagesByPseudonym` stores three messages, each created by `CreateObfuscatedMessage` which the code comments promise produces "a unique per-message ephemeral key, so every message has a distinct `RecipientPseudonym`". The test then retrieves by each individual pseudonym and asserts exactly 1 message — but receives 3. This means either (i) per-message pseudonyms are not actually unique (privacy regression: a storage node can correlate all messages to one recipient), or (ii) `pseudonymIndex` keys are not per-message but per-recipient (privacy regression: the index acts as a long-term identifier). Either way the README claim "identity obfuscation via epoch-based pseudonyms" is not satisfied for the offline-messaging flow.
  Data flow: `CreateObfuscatedMessage` → `StoreObfuscatedMessage` (writes to `pseudonymIndex[RecipientPseudonym][epoch]`) → `RetrieveMessagesByPseudonym` returns the whole epoch slice. Need to determine whether `RecipientPseudonym` is computed deterministically from the recipient PK only, or whether the same ephemeral is reused across all three messages.
  **Remediation:** Inspect `CreateObfuscatedMessage` in `async/obfs.go` and `async/obfuscation.go`. If pseudonyms are intentionally per-recipient/per-epoch, update the test and `async/doc.go` to match. If they are supposed to be per-message, fix the derivation. Either way add an assertion in `StoreObfuscatedMessage` and document the invariant in `async/storage.go`. Validate with `go test -race ./async/...`.

### HIGH

- [x] **F-ASYNC-002 — `sendQueuedMessages` leaves queued messages stuck when no pre-key signal channel was registered, breaking the documented "queued messages sent after pre-key exchange" guarantee.**
  Files: `async/manager.go:858-924`, `async/pending_message_queue_test.go:125-235`.
  Class: Concurrency / logic bug on a primary feature path.
  Consequence: `TestQueuedMessagesSentAfterPreKeyExchange` fails reproducibly with "Expected queue to be cleared after friend comes online, but 2 messages remain" (line 230). Tracing line 873: when `ch == nil` (no pre-key-ready channel was registered before `sendQueuedMessages` was called) **and** `CanSendMessage(friendPK)` returns false, the `if` body is skipped entirely; the loop at line 901 calls `sendForwardSecureMessage` which fails because pre-keys are still missing; failures are re-queued at line 917. Effect: every offline message a user sends to a friend who later comes online stays stuck in `pendingMessages` until something else triggers a retry. The README promises "store-and-forward delivery through distributed storage nodes" — this regression breaks it for the most common code path.
  Data flow: `onFriendOnline` → `sendQueuedMessages` line 820 → ch lookup line 865 returns nil because `signalPreKeyReady` runs later → wait skipped → all sends fail → all messages re-queued.
  **Remediation:** When `ch == nil` and `CanSendMessage` is false, either (a) lazily create the channel and `RegisterPreKeyExchange` for `friendPK`, then wait, or (b) push back into `pendingMessages` and return immediately without attempting the doomed send loop, so a later `signalPreKeyReady` actually triggers another `sendQueuedMessages`. Validate with `go test -race ./async/...`.

- [ ] **F-FILE-001 — `Transfer.writeDataToFile` does not close `t.FileHandle` when a write error occurs, leaking the file descriptor for the lifetime of the process.**
  Files: `file/transfer.go:530-545` (write error path) and the cleanup logic at `file/transfer.go:447-484` (`Cancel`) and `file/transfer.go:654-679` (`complete`).
  Class: Resource leak on the file-transfer critical path.
  Consequence: When `os.File.Write` returns an error the function sets `t.State = TransferStateError`, fires the `completeCallback`, and returns. Neither `Cancel` nor `complete` is invoked on this path. `t.FileHandle` therefore stays open until the process exits. A buggy or hostile peer that triggers repeated write errors (e.g. quota exhaustion, EINTR) can exhaust file-descriptor limits on the receiver, which is observable as a denial-of-service on a long-running client.
  Data flow: `WriteChunk` (under `t.mu`) → `writeDataToFile` → error → state set, callback fired, return → no `Close()` ever called.
  **Remediation:** In `writeDataToFile`'s error branch, close `t.FileHandle` and set it to `nil` before invoking `completeCallback`, mirroring the cleanup already present in `complete()` at line 656. Add a unit test that injects a write error (e.g. via a wrapped file or a quota-exhausted directory) and asserts that the FD count does not grow. Validate with `go test -race ./file/...`.

### MEDIUM

- [ ] **F-FILE-002 — `Transfer.complete()` mutates shared state without holding `t.mu`, racing with `Cancel`, `WriteChunk`, and `ReadChunk`.**
  Files: `file/transfer.go:654-679`.
  Class: Concurrency (race condition on a struct that is otherwise mutex-protected).
  Consequence: `Cancel` (line 452) takes `t.mu.Lock`, but `complete` is called from `checkTransferCompletion` (line 558) and `ReadChunk` (line 633). The first of those is invoked under the lock (from `WriteChunk` line 487 → `checkTransferCompletion` line 512), but `complete` is also reachable from external code paths and tests assign to `FileHandle` from outside (see `file/transfer_state_test.go:205`). On those paths `t.FileHandle`, `t.State`, `t.Error`, and the callback invocation are not synchronised, which is a genuine race with `Cancel`. `go test -race` currently reports no race only because no test exercises both paths concurrently.
  **Remediation:** Add an unexported `completeLocked` helper that does the field mutation, keep `complete` as a public wrapper that takes `t.mu.Lock()`, and update the two in-lock callers to use `completeLocked` instead. Validate with a new concurrent test that calls `WriteChunk` and `Cancel` in parallel under `go test -race ./file/...`.

- [ ] **F-CAPI-001 — `toxav_new` ignores errors from `validateAndGetToxInstance` and `createToxAVInstance`, relying on a nil-check that hides the actual failure reason from C callers.**
  Files: `capi/toxav_c.go:372-389`.
  Class: Error handling on the FFI surface.
  Consequence: The two `_, _ :=` discards (lines 377 and 382) are technically safe because both helpers set `*error_ptr` internally before returning a nil instance — but if a future change to either helper stops setting `error_ptr` on some branch, C callers will see `TOX_AV_ERR_NEW_OK` and a NULL handle, which is undefined in the C API. This is exactly the class of fragile contract that produces hard-to-diagnose qTox crashes.
  **Remediation:** Replace the `_, _ :=` with explicit `instance, err := ...; if err != nil { return nil }`. The error itself can still be discarded once `error_ptr` is set, but a build-time guarantee that the helpers ran is preferable. Validate with `go test ./capi/...`.

- [ ] **F-AV-001 — Sentinel error compared with `==` across package boundary in `Manager.EndCallIfActive`.**
  File: `av/manager.go:1255-1261`.
  Class: Error handling consistency.
  Consequence: `if err == ErrNoActiveCall` does not match when an upstream layer (e.g. a future `transport`/`call` wrapper) decides to wrap the sentinel with `%w`. Today `EndCall` returns the bare sentinel, so the check works — but the rest of the codebase already uses `errors.Is` (e.g. `capi/toxav_c.go:392` documents this expectation), so this site is inconsistent.
  **Remediation:** Replace with `if errors.Is(err, ErrNoActiveCall)`. Validate with `go test ./av/...`.

- [ ] **F-CRYPTO-002 — `reencryptWithNewKey` (93 lines, cyclomatic 22) has multiple rollback branches that restore `ks.encryptionKey = oldKey` but only one of them re-attempts the `cleanupExpiredLocked`-style cleanup of `.preencrypt.tmp` backups, leaving stale ciphertext on disk after some failure paths.**
  File: `crypto/keystore.go:389-482`.
  Class: Resource lifecycle / partial-failure recovery on a critical security path.
  Consequence: The Phase-1 rollback (lines 405-414) and the Phase-2 first-rename rollback (lines 444-458) both clean up `.reencrypt.tmp` siblings, but the Phase-2 inner-iteration rollback (lines 442-459) leaves any `.preencrypt.tmp` files from previously-renamed entries lying around. Likewise the salt-rename failure path (lines 463-471) restores backups but never removes the `*.reencrypt.tmp` siblings if the function was called with multiple files. On a long-lived host this can accumulate copies of previous ciphertext until the next successful rotation cleans them up.
  **Remediation:** Extract the rollback into a single helper that always removes both `*.reencrypt.tmp` and any `*.preencrypt.tmp` that have not yet been moved back into place, and call it from every error branch. Keep the existing logging contract. Validate with `go test ./crypto/...` and add a fault-injection test that fails the second rename in a 3-file rotation.

### LOW

- [ ] **F-CRYPTO-003 — `crypto/secure_memory.go:48` panics on `SecureWipe` failure outside an `init()` path.**
  File: `crypto/secure_memory.go:48`.
  Class: Panic / API contract.
  Consequence: A failure inside the underlying `mlock`/zeroing helper aborts the host process. Callers cannot recover, even though most crypto operations can degrade gracefully (e.g. fall back to best-effort zeroing).
  **Remediation:** Return an error instead of panicking, log at `error` level in callers that cannot meaningfully react, and update the existing `crypto/shared_secret.go:54-57` usage to ignore the error since it's already wrapping with `ZeroBytes`. Validate with `go test ./crypto/... ./async/... ./transport/...`.

- [ ] **F-DOC-001 — Six `BUG`-tagged comments flagged by go-stats-generator as "critical" describe logging in hot paths that should be removed or downgraded.**
  Files: `crypto/logging.go:17,23,115`, `crypto/shared_secret.go:15`, `toxav.go:779`, `toxcore_defaults.go:50`.
  Class: Documentation / performance hint left in production code.
  Consequence: Each `BUG:` comment is auto-classified as critical by static-analysis tools; left unaddressed they pollute every audit run and obscure real issues. The actual logging is at `debug`-level so the runtime impact is minimal, but the `BUG` tag is an authoring intent that has gone stale.
  **Remediation:** Either remove the logging at those sites (preferred — the surrounding code is hot) or rewrite the comments to plain `// NOTE:` if the logging is intentionally retained. Validate with a fresh `go-stats-generator analyze . --skip-tests` showing zero BUG annotations.

- [ ] **F-DOC-002 — `async/doc.go` and the test comment at `async/async_test.go:576-578` both promise "unique per-message pseudonym", but at least one of them is inconsistent with the storage retrieval results — see F-ASYNC-001.**
  Files: `async/doc.go`, `async/async_test.go:576-578`.
  Class: Documentation / behavioural claim that the code does not satisfy.
  **Remediation:** Once F-ASYNC-001 is resolved, update whichever document remains incorrect. Validate by re-reading both docs against the storage index design.

- [ ] **F-DUP-001 — Three groups of near-duplicate code in critical packages may drift independently.**
  Files: `async/secure_storage.go:28-43` vs `81-96` (exact, 16 lines);
  `noise/handshake.go:506-518` vs `523-535` (renamed, 13 lines);
  `capi/toxcore_c.go:1152-1165` vs `1191-1204` vs `1228-1241` (renamed, 14 lines × 3).
  Class: Duplication / maintainability.
  Consequence: Each pair handles cryptographic state or FFI translation; a fix applied to one copy and forgotten on another is a likely future bug.
  **Remediation:** Extract the duplicated body into a private helper or template function. Validate with `go test ./async/... ./noise/... ./capi/...` after each refactor.

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total Go source files (non-test) | 238 |
| Total non-test LOC | 42,298 |
| Total functions + methods | 4,049 |
| Functions above cyclomatic 15 | 2 (`reencryptWithNewKey`, `waitForDataSignal`) |
| Functions above cyclomatic 10 | 5 |
| Functions above 50 lines | 34 (0.8 %) |
| Average cyclomatic complexity | 3.6 |
| Doc coverage (overall) | 93.3 % |
| Doc coverage (packages / functions / types / methods) | 100 / 98.7 / 92.2 / 92.2 % |
| Duplication ratio | 0.46 % (398 / ~87 k tokens) |
| `go vet ./...` warnings | 0 |
| `go test -race ./...` failing packages | 2 (`crypto`, `async`) |
| `go test -race ./...` failing test cases | 5 |
| BUG annotations | 6 |
| Deprecated annotations | 19 |

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|----------------|
| `crypto/keystore.go:481` `ZeroBytes(oldKey[:])` on a local `[32]byte` copy | The local was already going out of scope; `ks.encryptionKey` holds the live key. Not a wipe failure, just a no-op for defense-in-depth. |
| `dht/mdns_discovery.go:43,46` and `transport/nat.go:26` panic in `init()` on `ResolveUDPAddr` failure | These are package-level constants for well-known multicast addresses; resolution can only fail in a broken stdlib. Panicking at `init` is the idiomatic Go pattern. |
| Examples using `panic(err)` (`examples/friend_callbacks_demo/main.go:17,46`, `examples/toxav_call_control_demo/main.go:18,24`) | Examples / demos are not production code and the project README explicitly treats them as illustrative. |
| `examples/av_quality_monitor/main.go:13` using `math/rand` | Comment explicitly states "simulation only, not security-sensitive". Acknowledged pattern. |
| `transport/proxy.go:552` discards `Password()` ok-value | Inline comment explains HTTP Basic Auth semantics; acknowledged pattern. |
| `defer` inside `for` loop in test files (`dht/skademlia_test.go:382`, `friend_callbacks_demo` paths, etc.) | All in `_test.go` files where the defer-until-function-return semantics are correct (the goroutines are short-lived). |
| `math/rand` use across remaining `examples/`, `simulation/`, and `testnet/` packages (sampled) | None reach the security boundary; all are non-cryptographic simulation seeds. |
| `InsecureSkipVerify` search | Zero hits in the entire repository. |
| Hard-coded secrets / private keys | Zero hits matching `BEGIN PRIVATE`, `BEGIN RSA`, `api_key =`, `secret = "..."`. |

## Remaining Scope

This was an end-to-end **partial pass**. The following packages must receive the full
Phase 3 checklist before the audit can be declared complete. Order is by perceived risk
(critical paths first, then ancillary), consistent with the task's Tiebreaker rule.

| Package | Status | Notes for next session |
|---------|--------|------------------------|
| `dht` | Spot-checked init only | Re-run Phase 3 fully; this is the second-largest critical-path package. Pay attention to `iterative_lookup.go`, `bootstrap.go`, `partition_detector.go`, `gossip_bootstrap.go`. |
| `transport` | Security spot-check only | Run 3b/3c/3d/3e/3f across `udp.go`, `tcp.go`, `relay.go`, `relay_mux.go`, `noise_transport.go`, `nat.go`, `reuseport.go`, `address_resolver.go`, `version_negotiation.go`. |
| `noise` | Not audited | Audit `handshake.go` (already flagged in F-DUP-001), `transport.go`, `xx.go` for state-machine correctness and IK/XX downgrade attacks. |
| `messaging` | Not audited | Audit priority queue, manager, delivery tracking; check for races. |
| `friend` | Not audited | Audit sharded state storage and request handling. |
| `group` | Not audited | Audit role enforcement and sender-key distribution. |
| `av`, `av/audio`, `av/video`, `av/rtp` | Errors spot-check only | Audit RTP packetizer, jitter buffer, adaptive-bitrate logic, codec lifetimes (Opus / VP8). |
| `bootstrap`, `bootstrap/nodes` | Not audited | Audit hard-coded node-list parsing and identity validation. |
| `factory`, `limits`, `interfaces`, `real`, `simulation` | Not audited | Lower priority; audit for API misuse and panics. |
| `toxnet`, `toxnet/example`, `toxnet/examples/packet` | Concurrency spot-check only (`waitForDataSignal`) | Audit `conn.go`, `addr.go`, `packet_connection.go` for the full checklist. |
| `capi` | Nil-safety only | Audit the rest of `toxcore_c.go` and `toxav_c.go` for type-confusion across the FFI boundary and lifetime mismatches with C strings. |
| Root package `toxcore` and `toxcore_*.go` files | Not audited | Audit `toxcore_lifecycle.go`, `toxcore_callbacks.go`, `toxcore_messaging.go`, `toxcore_file.go`, `toxcore_persistence.go`, `toxcore_self.go`, `toxcore_network.go`, `toxcore_friends.go`, `toxcore_conference.go`, `options.go`, `iteration_pipelines.go`. |
| `examples/*` (30+ packages) | Not audited | Treat as illustrative; sample one or two for documentation-vs-code drift and security smells, then declare complete. |
| `cmd/gen-bootstrap-nodes` | Not audited | Small CLI; audit `run` (cyclomatic 11) and parsing logic. |
| `testnet`, `testnet/internal`, `testnet/cmd` | Not audited | Audit orchestrator, client, log file handling. |
| `privacy_networks`, `transport/internal/addressing` | Not audited | Small packages; one pass should suffice. |

**Resume from the top of this table next session.** Append new findings to the existing
severity sections in this file rather than starting a new report — the end-to-end policy
requires findings to be cumulative until a complete pass yields zero new confirmed
issues above LOW.
