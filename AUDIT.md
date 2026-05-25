# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-25

## Project Profile

- **Project**: `github.com/opd-ai/toxcore` — pure-Go implementation of the Tox P2P encrypted-messaging protocol.
- **Stated goals** (from `README.md`): DHT-based peer discovery; friend management; 1-to-1 & group messaging; chunked file transfers; ToxAV audio/video calling (Opus + VP8); asynchronous offline messaging with forward secrecy and identity obfuscation; multi-network transports (IPv4/IPv6 UDP/TCP, Tor `.onion`, I2P `.b32.i2p`, Lokinet, Nym); Noise-IK and Noise-XX handshakes via `flynn/noise`; NAT traversal (STUN/UPnP/hole-punching); pure-Go core (no cgo except `capi/`); `net.Conn` / `net.Listener` / `net.PacketConn` adapters in `toxnet/`; libtoxcore-compatible C API.
- **Target users**: developers building privacy-focused communication apps; cross-platform (Linux/macOS/Windows on amd64/arm64).
- **Deployment model**: library imported into client applications; no central infrastructure; some bridge processes (`testnet/`, `cmd/gen-bootstrap-nodes`).
- **Critical paths** (where bugs have highest impact, given stated goals):
  1. `crypto/` — keypair, AEAD encrypt/decrypt, secure memory, replay protection, encrypted keystore
  2. `noise/` — Noise-IK handshake, PSK session resumption
  3. `async/` — pre-keys, forward-secrecy manager, obfuscation, padding, WAL, retrieval scheduler
  4. `transport/` — UDP/TCP/Noise/proxy transports, NAT traversal, address parsing, negotiation
  5. `dht/` — routing, k-bucket maintenance, bootstrap, iterative lookup, local & mDNS discovery
  6. `messaging/` — message construction, delivery receipts, persistence, priority queue
  7. `toxcore.go` + siblings — public API facade integrating all subsystems

## Audit Scope

| Item | Value |
|---|---|
| Packages enumerated by `go list ./...` | 51 (incl. `examples/*`, `testnet/*`, `cmd/*`) |
| Production packages audited | 26 (per `go-stats-generator`) |
| Non-test source files analyzed | 238 |
| Functions inspected (statically via go-stats-generator) | 1155 free functions + 2879 methods |
| Manual deep-reads | high-complexity functions (>10 cyclomatic), all sites flagged by category greps below |
| Build environment | Go 1.25.0 toolchain go1.25.8 (per `go.mod`) |
| Static analysis | `go vet ./...` → **0 warnings** |
| Test execution (`-tags nonet -race -count=1`) | core packages: `crypto` ✅, `async` ✅, `dht` ✅, `transport` **1 FAIL + panic** (`TestHandshakePacketHandling`) |

### Hunting methodology

For each bug-class in Phase 3 of the prompt, a tree-wide ripgrep filter (excluding `*_test.go` and `examples/`) was used to enumerate candidate sites. Sites matching the pattern were then manually inspected with full surrounding context (≥30-line windows) and traced to a concrete consequence, applying the false-positive checks in Phase 3l (data-flow reachability, upstream guards, surrounding comments, exploitability). Sites passing the filter were also cross-referenced against the `go-stats-generator` complexity list to prioritise deep reads.

## Coverage Log

✅ = full category sweep completed for the package; ⚠️ = sweep completed but did not include line-by-line review of every function (relies on category greps + targeted hot-spot reads); ➖ = not applicable (no code matching that category).

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| `toxcore` (root) | ⚠️ | ⚠️ | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ | ✅ | ⚠️ |
| `crypto`         | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `noise`          | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `async`          | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ✅ |
| `dht`            | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| `transport`      | ⚠️ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ✅ | ✅ | ⚠️ |
| `transport/internal/addressing` | ✅ | ✅ | ✅ | ➖ | ➖ | ✅ | ✅ | ✅ | ✅ |
| `messaging`      | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `friend`         | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `file`           | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `group`          | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ✅ |
| `av`             | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ✅ |
| `av/audio`       | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `av/video`       | ⚠️ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `av/rtp`         | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `toxnet`         | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | ✅ |
| `bootstrap`      | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `capi`           | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `factory`        | ✅ | ✅ | ✅ | ➖ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `interfaces`     | ➖ | ➖ | ➖ | ➖ | ➖ | ➖ | ➖ | ➖ | ✅ |
| `limits`         | ✅ | ✅ | ✅ | ➖ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `real`           | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `simulation`     | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `testnet/*`      | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `cmd/gen-bootstrap-nodes` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `examples/*`     | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ⚠️ |

The codebase is large (~42 kLOC excluding tests, 238 source files). The ⚠️ marks above reflect a category sweep using ripgrep filters with manual inspection of every site that matched a candidate pattern, plus deep reads of all functions whose cyclomatic complexity exceeded the 10 reported by `go-stats-generator`. Where a sweep is marked ⚠️ rather than ✅, the next session should also walk every function in that package line-by-line (see **Remaining Scope** below).

## Goal-Achievement Summary

| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| DHT routing with k-buckets, iterative lookups, LAN/mDNS discovery | ✅ | — |
| Friend management with sharded state storage | ✅ | — |
| 1-to-1 encrypted messaging with retry and padding | ✅ | — |
| Group chat with role-based permissions | ✅ | — |
| Chunked file transfers (pause/resume/cancel) | ✅ | — |
| ToxAV (Opus audio + VP8 video, RTP, jitter buffer) | ✅ | — |
| Async offline messaging with forward secrecy and pseudonyms | ⚠️ | H-001 (queued-message delivery depends on a 500 ms sleep timer rather than a synchronization primitive); H-003 (cover-traffic randomness silently produces zero on `crypto/rand` failure) |
| Multi-network transport (Tor/I2P/Lokinet/Nym + UDP/TCP) | ✅ | — |
| Noise-IK handshakes via `flynn/noise` | ⚠️ | H-002 (`transport.TestHandshakePacketHandling` failure + responder panic in test path) |
| NAT traversal (STUN, UPnP, hole-punching) | ✅ | — |
| Pure-Go core (no cgo outside `capi/`) | ✅ | Verified: `grep -r '"C"' --include='*.go'` returns matches only under `capi/`, `examples/`, `av/audio/codec_cgo.go`-style files guarded by build tags |
| `net.Conn`/`Listener`/`PacketConn` adapters (`toxnet`) | ✅ | — |
| Traffic-analysis-resistant padding (256/1024/4096 B) | ⚠️ | M-001 (random padding tail not error-checked) |
| Encrypted keystore with atomic rotation | ⚠️ | M-002 (rollback path in `reencryptWithNewKey` cannot truly restore originals once renames have started; documented but data-loss risk) |
| `go vet ./...` clean | ✅ | — |

## Findings

> Naming: **C-NNN** = CRITICAL; **H-NNN** = HIGH; **M-NNN** = MEDIUM; **L-NNN** = LOW. Every finding includes file:line and a concrete consequence.

### CRITICAL

_None confirmed in this pass._ No traceable data-corruption, RCE, or fully-broken documented feature was discovered. Items that initially looked critical (e.g. WAL `json.Marshal` ignored errors, keystore rotation rollback) were demoted after data-flow analysis (the marshaled values are simple structs with no unmarshalable fields, and the keystore failure path is documented and only reachable on filesystem-rename failure).

### HIGH

- [ ] **H-001 — `async/manager.go:842` sleeps 500 ms instead of waiting on the pre-key-exchange completion event** — Concurrency / API contract. `sendQueuedMessages` is invoked when pre-key publication completes; it drains `pendingMessages`, then calls `time.Sleep(500 * time.Millisecond)` "to wait briefly for pre-key exchange to complete" before invoking `sendForwardSecureMessage`. **Consequence**: on a slow link or under load the pre-key exchange can exceed 500 ms, in which case every queued message will fail to send (`sendForwardSecureMessage` will return an error that goes to `log.Printf` only — there is no retry, no re-queueing). On a fast loopback the sleep is pure latency and slows interactive sending. This directly affects the README's claimed "Asynchronous offline messaging" goal. **Remediation**: replace the timer with an explicit synchronization primitive — have the pre-key exchange completion path close a per-peer `chan struct{}` (stored in `am.pendingMessages` alongside the messages) and `select { case <-doneCh: case <-time.After(timeout): }` here, where `timeout` is a configurable maximum. Re-queue messages on failure rather than dropping them. **Validate**: `go test -race -tags nonet ./async/... -run TestAsync.*Pending` and add a test that publishes pre-keys after a 1 s artificial delay and asserts that queued messages are still delivered.

- [ ] **H-002 — `transport/noise_transport.go` handleHandshakePacket fails responder authentication in race-mode tests** — Cryptography / API contract. `go test -tags nonet -race ./transport/...` produces:
  ```
  --- FAIL: TestHandshakePacketHandling (0.01s)
      noise_transport_test.go:348: Handshake handling failed: failed to generate handshake response: responder read failed: chacha20poly1305: message authentication failed
      noise_transport_test.go:354: Expected 1 response packet, got 0
  panic: runtime error: index out of range [0] with length 0
  ```
  The test constructs an initiator handshake message with `noise.NewHandshakeState(...)` and feeds the resulting bytes to `noiseTransport.handleHandshakePacket`. The responder reports an AEAD-auth failure, meaning either (a) the static-key/prologue/handshake pattern used by the test does not match what the production responder constructs in `handleHandshakePacket`, or (b) the production responder discards/normalizes the initiator's static key in a way the test does not anticipate. **Consequence**: at minimum a test-suite failure breaks CI; at worst a live initiator using a slightly different but legitimate construction would similarly fail to complete Noise-IK, leaving the connection to fall back to legacy Tox encryption (see `transport/negotiating_transport.go`) — silently undermining the stated KCI-resistance and forward-secrecy guarantees. **Remediation**: fix `transport/noise_transport.go:handleHandshakePacket` (and its helpers `validateHandshakePacket` / `processHandshake` if present) so the test's hand-rolled IK initiator is accepted; or, if the test setup is genuinely invalid, document why and replace the test with one that uses the production initiator path so the round-trip is exercised end-to-end. Add a second test that drives both sides via the production code only. **Validate**: `go test -tags nonet -race ./transport/... -run TestHandshakePacketHandling -count=2`.

- [ ] **H-003 — `async/message_padding.go:64` ignores `crypto/rand.Read` error when filling padding tail** — Security / cryptography. `PadMessageToStandardSize` is the documented mechanism for traffic-analysis resistance (`docs/COVER_TRAFFIC.md`, README "automatic message padding (256B, 1024B, 4096B)"). It calls `rand.Read(paddedMessage[originalLen+LengthPrefixSize:])` with no return-value check. **Consequence**: if `crypto/rand.Read` ever returns an error (rare but possible on entropy-starved systems, broken `/dev/urandom`, or sandbox environments) the padding tail is left as zero bytes from `make`. A passive observer who knows the protocol could then distinguish a padded short message from a genuinely large one by looking at the entropy of the trailing bytes — exactly the attack the padding was designed to defeat. **Remediation**: change to `if _, err := rand.Read(paddedMessage[originalLen+LengthPrefixSize:]); err != nil { return nil, fmt.Errorf("padding randomness failed: %w", err) }`. Callers already propagate the error from `PadMessageToStandardSize` (`async/forward_secrecy.go`, `async/obfs.go`). **Validate**: `go test -race ./async/... -run TestPadding` plus a new test injecting a fake `io.Reader` returning an error and asserting `PadMessageToStandardSize` returns an error.

### MEDIUM

- [ ] **M-001 — `crypto/keystore.go:430-447` per-file rename rollback in `reencryptWithNewKey` cannot restore originals once any rename has succeeded** — Logic / data safety. After a successful `os.Rename(tmpPath, finalPath)` the original ciphertext at `finalPath` is overwritten by the new-key version. The rollback at lines 434-438 then attempts `os.Rename(filepath.Join(ks.dataDir, alreadyRenamed), tmpRestorePath)` which only renames the (already-overwritten) new-key file back to a `.reencrypt.tmp` suffix; the *old*-key ciphertext that was at `finalPath` before the rename is permanently gone. The function also restores `ks.encryptionKey = oldKey`, so the in-memory key no longer matches what is on disk for any `alreadyRenamed` file. **Consequence**: on a transient rename failure mid-rotation (e.g. concurrent process holding a handle on Windows, fs full, EXDEV across mounts) the affected files become permanently unreadable with the in-memory key. The README and `crypto/doc.go` document the keystore as a durable encrypted store. **Remediation**: stage all renames against a copy of each `finalPath` (e.g. write the new-key ciphertext to a `.reencrypt.tmp` and rename the *old* `finalPath` to `.preencrypt.tmp` first, then rename `.reencrypt.tmp` to `finalPath`, then unlink `.preencrypt.tmp` after the salt rename succeeds). Roll back by renaming `.preencrypt.tmp` back to `finalPath`. **Validate**: `go test -race ./crypto/... -run TestEncryptedKeyStore.*Rotat` and add a test that uses a fault-injecting filesystem stub failing the third rename, asserting all original files remain readable with the old key.

- [ ] **M-002 — `async/retrieval_scheduler.go:173` ignored `rand.Int` error can crash on next line** — Reliability. `shouldSendCoverTraffic` does `randomBig, _ := rand.Int(rand.Reader, big.NewInt(1000))` and then immediately dereferences `randomBig.Int64()`. If `rand.Int` returns an error, `randomBig` is `nil` and the next call panics with a nil-pointer dereference. **Consequence**: a single entropy-source hiccup tears down the entire `RetrievalScheduler` goroutine, silently halting cover-traffic generation and possibly the message-retrieval loop. **Remediation**: handle the error — on failure, default to `false` (skip cover traffic for this tick) and log a warning, mirroring the error handling at line 119 in the same file. **Validate**: `go test -race ./async/... -run TestRetrievalScheduler`.

- [ ] **M-003 — `transport/noise_transport_test.go:354-359` test asserts then dereferences without guarding** — Test reliability. The test does `if len(packets) != 1 { t.Errorf(...) }` (not `t.Fatalf`) then continues to `packets[0].packet.PacketType` which panics when `len(packets) == 0`. This panic is exactly what propagates the H-002 failure into a `panic: runtime error: index out of range` instead of a clean fail report. **Consequence**: makes future regressions in the same test print stack-traces instead of useful diagnostics; obscures the root cause. **Remediation**: change `t.Errorf` to `t.Fatalf` (or wrap the index access in `if len(packets) > 0`). **Validate**: `go test -tags nonet -race ./transport/... -run TestHandshakePacketHandling` should now report a clean failure instead of panicking. (This finding is independent of H-002; fixing only this would still leave the underlying handshake bug.)

- [ ] **M-004 — `transport/address.go:357-367` and `transport/network_detector.go:222-235` violate the project's own "no type assertions to concrete `net.*Addr`" rule** — API consistency. The project README and the in-tree comment at `transport/doc.go:10` (and `interfaces/doc.go:108`) explicitly state "never use concrete network types … never use a type switch or type assertion to convert from an interface type to a concrete type." The two switch statements above do exactly that on `*net.TCPAddr`, `*net.UDPAddr`, `*net.IPAddr`. **Consequence**: mock transports that return a custom `net.Addr` implementation fall through to `parseIPFromString(addr.String())`, which works but bypasses the fast path; the inconsistency invites copy-paste of the anti-pattern to new code. **Remediation**: implement IP/port extraction by adding an optional internal interface (e.g. `interface{ IP() net.IP; Port() int }`) and use a single-form assertion `if a, ok := addr.(ipPortAddr); ok { ... }` instead of switching on concrete net types; or accept this as a documented exception by adding a `//nolint:` comment explaining why this site is the one place the concrete-type switch is required. **Validate**: `go vet ./transport/... && go test -race ./transport/...`.

- [ ] **M-005 — `transport/proxy.go:552` silently discards SOCKS5 proxy password** — Error handling. `password, _ := d.proxyURL.User.Password()` ignores the boolean second return that indicates whether a password was set. **Consequence**: when the user configures a proxy URL without `:password` in `User`, the dialer transparently sends an empty password rather than rejecting the request — a silent authentication misconfiguration that the user will only notice as a proxy auth failure. **Remediation**: replace with `password, hasPwd := d.proxyURL.User.Password(); if !hasPwd { ... handle missing password explicitly ... }` (either return an error or skip the auth round-trip if the SOCKS5 server allows anonymous). **Validate**: `go test -race ./transport/... -run TestSOCKS5`.

### LOW

- [ ] **L-001 — `async/wal.go:234, 362, 394` ignore `json.Marshal` errors when computing CRC for WAL entries** — Defensive coding. The marshaled values are `WALEntry` structs containing only primitive types and `[]byte` slices, so `json.Marshal` cannot fail in practice; nonetheless, every other Marshal/Unmarshal site in the package checks the error. **Consequence**: a future schema change adding an unsupported field (e.g. a `chan`, `func`, or self-referential map) would produce silent zero-checksum entries on the write path and a checksum-mismatch error on the read path with no diagnostic. **Remediation**: at each of the three sites, check the error and return/log it — e.g. `dataForChecksum, err := json.Marshal(entry); if err != nil { return fmt.Errorf("wal: marshal for checksum failed: %w", err) }`. **Validate**: `go test -race ./async/... -run TestWAL`.

- [ ] **L-002 — `dht/maintenance.go:233, 257, 331` and similar (`dht/group_storage.go:320`, `dht/relay_storage.go:299`, `async/prekey_dht.go:311`, `group/dht_replication.go:217`) discard transport `Send` errors as "best effort"** — Observability. These are correctly best-effort sends, but none of them logs the error, so a persistently broken transport (e.g. NIC down, IPv6 disabled) produces zero visible signal to the operator. **Consequence**: degraded DHT/prekey distribution that operators cannot diagnose from logs alone. **Remediation**: wrap with `if err := tr.Send(...); err != nil { logger.WithError(err).Debug("best-effort send failed") }` — Debug-level so it doesn't spam Info logs but is available with `--log-level=debug`. **Validate**: `go vet ./... && go test -race ./dht/...`.

- [ ] **L-003 — `dht/mdns_discovery.go:321,329,350,358` ignore `net.ResolveUDPAddr` errors on constant strings** — Defensive coding. Inputs are compile-time string constants (`mdnsIPv4Addr`, `mdnsIPv6Addr`) so resolution cannot fail; nevertheless this is the only place in the package where `ResolveUDPAddr` is not error-checked. **Consequence**: future maintainer who changes the constant to a variable will inherit a silent error path. **Remediation**: use `MustResolveUDPAddr`-style helper or `panic` on the impossible error to make intent explicit; alternatively, lift these to package-level `var` initialized in `init()` with explicit error handling. **Validate**: `go vet ./dht/...`.

- [ ] **L-004 — `av/video/processor.go:1011` `make([]byte, width*height)` without explicit overflow guard** — Logic / boundary. `width` and `height` are `int`s derived from VP8-decoded frame dimensions. On 64-bit hosts overflow is unreachable for plausible inputs; on 32-bit `int` (e.g. some embedded ARMv7 targets in scope of the README's "linux/amd64/arm64 (Windows arm64 excluded)") it would require `width*height > 2³¹` — also implausible but theoretical. Adjacent validator `validatePlaneParams` (cyclomatic 11, same file) bounds the inputs upstream. **Consequence**: theoretical only. **Remediation**: if 32-bit arm is a real deployment target, add an explicit `if width > 0 && height > 0 && width*height/height == width { ... } else { return nil, fmt.Errorf("plane too large: %dx%d", width, height) }` guard. Otherwise close as won't-fix with a comment. **Validate**: `go test -race ./av/video/...`.

- [ ] **L-005 — `crypto/secure_memory.go:45` `_ = SecureWipe(data)` discards error** — Defensive coding. `SecureWipe` is the canonical zeroing helper; suppressing its error here in the public `WipeBytes` wrapper hides any future failure mode (none today). **Consequence**: minor; current implementation can't fail. **Remediation**: propagate the error or panic if it ever becomes non-nil (`if err := SecureWipe(data); err != nil { panic(err) }` — panic is appropriate for a security-invariant violation). **Validate**: `go test -race ./crypto/...`.

- [ ] **L-006 — `transport/hole_puncher.go:357` exposes `*net.UDPAddr` in a public function signature** — API consistency. `SimultaneousPunch(ctx context.Context, remoteAddr *net.UDPAddr, ...)` takes a concrete type, violating the project guideline ("never use `*net.UDPAddr` … use `net.Addr` only"). **Consequence**: callers cannot supply a non-UDP-backed mock; complicates testing of NAT code. **Remediation**: change parameter to `net.Addr` and use the interface methods. **Validate**: `go vet ./transport/...`.

- [ ] **L-007 — `dht/maintenance.go:287` `rand.Read(randomKey[:])` ignored error in periodic random-key generation** — Reliability. Errors from `crypto/rand` are extremely rare, but if one ever occurs the function continues with an all-zero key and probes the DHT for distance-from-zero — biased traffic that can be fingerprinted. **Consequence**: low; relies on a `crypto/rand` outage. **Remediation**: check the error and skip the probe round on failure (`if _, err := rand.Read(...); err != nil { return err }`). **Validate**: `go test -race ./dht/... -run TestMaintenance`.

- [ ] **L-008 — `crypto/toxid.go:87` `_, err := rand.Read(nospam[:])` returns err but several callers may discard it** — Defensive coding. The function itself returns the error correctly; spot-check on callers showed all check it, but the pattern is brittle. **Consequence**: none with current callers. **Remediation**: none needed; included for completeness of the audit log. (Closed; not a bug.)

- [ ] **L-009 — `toxnet/conn.go waitForDataSignal` has cyclomatic complexity 19 (highest in repo)** — Maintainability. The function does multiple `select`s around a `sync.Cond` and starts a goroutine to broadcast on timeout/ctx-done. The synchronization is correct (the inner goroutine takes the same mutex as `cond.Wait()`, which is released atomically), but the structure makes future modification high-risk. **Consequence**: long-term maintainability cost; no current bug. **Remediation**: refactor to a single `select` over an "available", "ctx-done", and "timeout" channel maintained by the writer side — eliminates the goroutine spawn-per-Wait. **Validate**: `go test -race ./toxnet/... -run TestToxConn`.

- [ ] **L-010 — `group/sender_key.go` `createDistributionForPeer` (77 lines, complexity 15.3) and `ProcessDistribution` (75 lines, complexity 13.5)** — Maintainability. Two longest functions in the codebase; both deal with sender-key distribution. No defect found, but the length increases the risk of off-by-one or state-machine drift on future edits. **Consequence**: maintainability. **Remediation**: break into per-step helpers (validate / generate / encrypt / store), one per logical phase. **Validate**: `go test -race ./group/...`.

- [ ] **L-011 — `examples/version_negotiation_demo/main.go:64,65,173` and similar example files ignore `rand.Read` errors** — Example hygiene. Examples are meant to be illustrative; the README directly invites users to copy them. Ignoring `crypto/rand.Read` teaches an anti-pattern. **Consequence**: educational only. **Remediation**: in each example file change `rand.Read(key)` to `if _, err := rand.Read(key); err != nil { log.Fatal(err) }`. **Validate**: `go build ./examples/...`.

- [ ] **L-012 — `transport/version_negotiation.go:237` ignores `singleflight.Group.Do` shared-bool return** — Documentation. `result, err, _ := vn.negotiationGroup.Do(...)` discards the `shared` indicator. **Consequence**: cannot count coalesced negotiations for metrics. **Remediation**: optional — capture `shared` and increment a metric when true. **Validate**: not required.

- [ ] **L-013 — `toxcore.go:1079` `id, _ := t.friends.FindByPublicKey(...)` discards the not-found error** — Defensive coding. The two-return form is used here only for the ID; the function is invoked from a path that already verified existence. **Consequence**: brittle to refactor. **Remediation**: rename `_` to `_ /* presence already verified above */` or add a defensive `if err != nil { return }`. **Validate**: `go test -race ./...`.

- [ ] **L-014 — `crypto/keystore.go:406-407, 419, 441-443` discard `os.Remove` errors when cleaning up temp files on rollback** — Defensive coding. Failure to remove a temp file is non-fatal but should at least be logged for operator visibility. **Consequence**: low — temp files accumulate in `dataDir` on repeated failed rotations. **Remediation**: log the error at Warn level. **Validate**: `go test -race ./crypto/... -run TestEncryptedKeyStore`.

- [ ] **L-015 — `real/packet_delivery.go:73` and `toxnet/dial.go:201` use `_ = addr`** — Code smell. These are explicit no-op assignments documenting intentional unused parameters. The `toxnet/dial.go` site has an explanatory comment ("address derived from tox instance"). The `real/packet_delivery.go` site does not — add one. **Consequence**: cosmetic. **Remediation**: add an explanatory comment at `real/packet_delivery.go:73`. **Validate**: `go vet ./real/...`.

- [ ] **L-016 — `av/rtp/session.go:424` `_ = timestamp` discards a computed timestamp** — Logic / API contract. The function appears to compute a value that is then thrown away — either dead code or a missing field assignment. **Consequence**: depends on intent (couldn't be conclusively determined in this pass). **Remediation**: read the function in full, decide whether to delete the dead computation or wire the timestamp into the RTP header. **Validate**: `go test -race ./av/rtp/...`.

- [ ] **L-017 — `av/video/rtp.go:504` `_ = fmt.Sprintf("sequence gap detected: expected %d, got %d", ...)` builds a string then throws it away** — Performance / logic. This is either a stub that meant to log and forgot, or a deliberate no-op. The `fmt.Sprintf` still allocates and runs on the hot RTP path. **Consequence**: per-gap heap allocation that does nothing useful. **Remediation**: either remove the line or replace with `logger.Debugf("sequence gap detected: expected %d, got %d", ...)`. **Validate**: `go test -race ./av/video/...`.

- [ ] **L-018 — `capi/toxav_c.go:377, 382` and `capi/toxcore_c.go:949-1557` use the `_ = x` pattern repeatedly to suppress unused-parameter warnings on stub C-API exports** — Documentation. These mark stub functions whose C signatures are exported but Go implementations are not yet wired up. **Consequence**: callers from C see a function that silently does nothing rather than returning a "not implemented" error. **Remediation**: where the C error_ptr is available, set it to a `TOX_ERR_*_NOT_IMPLEMENTED` enum (define one if absent); where there's no error channel, return a sentinel value documenting the gap. Also link these to `GAPS.md`. **Validate**: `go build ./capi/...`.

- [ ] **L-019 — `transport/nat.go:128` `fallbackAddr, _ := net.ResolveUDPAddr("udp", "203.0.113.1:0")` ignores error on a constant** — Defensive coding. Constant input (RFC 5737 documentation address) cannot fail; still inconsistent with the rest of the package. **Consequence**: none in practice. **Remediation**: convert to a package-level `var` with explicit `init()` resolution, or accept as-is. **Validate**: `go vet ./transport/...`.

- [ ] **L-020 — `transport/hole_puncher.go:121` `time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)` linear back-off without context cancellation** — Concurrency. The hole-puncher's retry loop sleeps without checking `ctx.Done()` — a caller that cancels the context still waits the full sleep before returning. **Consequence**: cancel latency up to ~`n*100ms` on long retry chains. **Remediation**: replace with `select { case <-time.After(...): case <-ctx.Done(): return ctx.Err() }`. **Validate**: `go test -race ./transport/... -run TestHolePuncher`.

- [ ] **L-021 — `dht/local_discovery.go:188, 213, 420`, `dht/mdns_discovery.go:167, 179, 509`, `dht/gossip_bootstrap.go:298`, `transport/socks5_udp.go:328-602`, `transport/address.go:105-107`, `transport/address_resolver.go:220-330`, `transport/nat.go:441`, `transport/reuseport_unix.go:53-123`, `transport/stun_client.go:322-365`, `av/types.go:153`, several `examples/*` construct `*net.UDPAddr` / `*net.TCPAddr` literals** — API consistency with the project's own networking guidelines. Most of these are necessary because the underlying syscall/SAM/proxy libraries require concrete types (`golang.org/x/net/ipv4.JoinGroup`, `syscall.Sockaddr` building, `socks5` wire serialization). The guideline in the README and `interfaces/doc.go` is "never use concrete net types"; in practice the codebase routinely violates it where forced by external APIs. **Consequence**: documentation/code-style drift. **Remediation**: amend the README to clarify "use `net.Addr` in public APIs; concrete types are acceptable internally only when an external dependency requires them" — and add a comment at each unavoidable site (`reuseport_unix.go`, `mdns_discovery.go` for `JoinGroup`, etc.) referencing that exception. Alternatively, audit each site and wrap in a thin internal helper that hides the concrete type. **Validate**: documentation only — `gofmt -l . && go vet ./...`.

- [ ] **L-022 — `staticcheck.conf` is committed but the project does not run staticcheck in CI (per `.github/workflows/toxcore.yml`)** — Documentation gap. The project ships a `staticcheck.conf` (existence verified via `ls` in repo root) suggesting staticcheck is part of the intended toolchain, but the CI workflow runs only `go vet` and tests. **Consequence**: developers running staticcheck locally may see findings that never block PRs. **Remediation**: either add a `staticcheck ./...` step to CI or remove the config file to avoid implying a stricter standard than is enforced. **Validate**: inspect workflow after the change.

- [ ] **L-023 — Identifier-naming and file-naming violations (108 + 7 reported by `go-stats-generator`)** — Style. Not enumerated here individually; mostly relate to Go-idiomatic naming. **Consequence**: cosmetic. **Remediation**: run `go-stats-generator analyze . --skip-tests --sections naming --format json` to enumerate and `gofmt -s` to fix the trivially-fixable subset. **Validate**: `gofmt -l .`.

- [ ] **L-024 — `BACKLOG_ANALYSIS.md` and `ROADMAP.md` exist alongside the README; their relationship to "current implementation" is undefined** — Documentation. Users cannot tell which document represents the current state. **Consequence**: confusing for new contributors. **Remediation**: add a one-line preamble to each clarifying its scope ("planned", "historical", "current"). **Validate**: human review.

- [ ] **L-025 — Doc-coverage on some packages is below the typical 50% threshold** — Documentation. `go-stats-generator` reports low cohesion / sparse doc on `interfaces`, `nodes`, `simulation`, `limits`, `addressing`, `common`. **Consequence**: harder onboarding. **Remediation**: add a `doc.go` to each (most already have one — the issue is per-function GoDoc on unexported helpers, which is acceptable). **Validate**: `go doc github.com/opd-ai/toxcore/limits`.

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Files processed | 238 |
| Total LOC (non-test) | 42,124 |
| Total functions | 1,155 |
| Total methods | 2,879 |
| Total structs | 407 |
| Total interfaces | 37 |
| Total packages | 26 |
| Average function length | 12.8 lines |
| Longest function | `createDistributionForPeer` — 77 lines |
| Functions > 50 lines | 33 (0.8 %) |
| Functions > 100 lines | 0 |
| Average cyclomatic complexity | 3.5 |
| Functions with complexity > 10 | 5 (`waitForDataSignal` 19, `reencryptWithNewKey` 12, `createDistributionForPeer` 11, `cmd run` 11, `validatePlaneParams` 11) |
| Functions with complexity > 15 | 1 (`waitForDataSignal`) |
| Duplication ratio | 0.51 % (30 clone pairs, 447 duplicated lines, largest 18 lines) |
| Circular dependencies | 0 |
| go vet warnings | 0 |
| Test packages passing (`crypto`+`async`+`dht`+`transport`) | 3 of 4 (transport fails — see H-002) |
| Failing tests | `TestHandshakePacketHandling` (panics on len==0 assertion path) |
| `go vet` second-pass after audit | 0 (no source modifications were made) |

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|----------------|
| `async/wal.go` ignored `json.Marshal` errors marked as HIGH | The marshaled type is `WALEntry` with only primitive/`[]byte` fields; `json.Marshal` cannot fail on these. Demoted to L-001 (defensive only). |
| `math/rand` use in `dht/local_discovery_test.go:4` | Test-only file; randomness is for test address selection, not security. |
| `panic(err)` in `examples/friend_callbacks_demo/main.go:17,46` | Example top-level `main()` — `panic` on bootstrap error is acceptable convention; not production code. |
| `panic("intentional test panic")` in `group/dht_timeout_test.go:282`, `transport/worker_pool_test.go:208` | Tests deliberately verifying panic-recovery paths. |
| `_ = pm.fsManager.ProcessPreKeyExchange(exchange)` (`async/prekey_dht.go:354`) | Surrounding code path: invoked from a fan-out handler that intentionally drops processing errors for already-received exchanges; behavior is documented in `async/doc.go`. |
| `_ = pm.PublishPreKeys()` (`async/prekey_dht.go:390`) | Periodic best-effort republication; failures retried on the next tick. Not a bug. |
| Type assertion `result.(ProtocolVersion)` in `transport/version_negotiation.go:245` | Uses the two-value form `negotiatedVersion, ok := result.(ProtocolVersion)`; safe. |
| `elem.Value.(*lookupCacheEntry)` and similar in `dht/routing.go`, `transport/lru_session_cache.go`, `messaging/priority_queue.go` | `container/list.Element.Value` is `interface{}` but the only producer in the same file inserts `*lookupCacheEntry` / `*sessionEntry` / `*PriorityItem`; invariant holds; matching tests verify. |
| `po.callSlicePool.Get().([]*Call)` (`av/performance.go:229`) | `sync.Pool.Get()` falls back to `New` which returns `[]*Call`; safe single-form assertion. |
| `examples/av_quality_monitor/main.go:197-209` use of `math/rand` | Example simulating jitter/loss for demo purposes — explicitly not for security. |
| `time.Sleep(500 * time.Millisecond)` cited in tests | Test-only sleeps in `testnet/internal/*` and similar are deterministic-enough for the platform tests; not a production correctness issue. |
| `InsecureSkipVerify` | grep returned **no** matches anywhere. Confirmed absent. |
| Hardcoded secrets / private keys | grep for `-----BEGIN` / `PRIVATE KEY` / `Bearer ` / API tokens returned only `crypto/secure_memory_test.go` literals (test vectors). |

## Remaining Scope

This audit was performed end-to-end across all 26 production packages, with category-grep sweeps applied to every non-test file and deep manual reads of every function above complexity 10. The packages marked ⚠️ in the Coverage Log received the category sweeps but did not receive a function-by-function line read; given each ⚠️ package contains hundreds of methods, a confirmatory next pass should:

| Package | Pending follow-up | Notes |
|---------|---------|---|
| `toxcore` (root) — 15 source files, 339 functions | Per-function line read | Sweeps complete; deep read of `toxcore_friends.go` (21 KB), `toxcore_unit_test.go`-driven flow into `toxcore.go` recommended. |
| `async` — 26 files, 495 functions | Per-function line read of `obfs.go`, `forward_secrecy.go`, `erasure.go`, `wal.go` recovery path | All sweeps complete; deeper read may surface additional aliasing / state-machine issues. |
| `dht` — 18 files, 420 functions | Per-function line read of `iterative_lookup.go` and `bootstrap.go` | High-complexity site `joinMulticastGroup` (cyclomatic 13) already inspected. |
| `transport` — 41 files, 738 functions (largest package) | Per-function line read; in particular `noise_transport.go` (root cause of H-002) deserves a full session | Includes Tor/I2P/Nym/Lokinet impls behind build tags; ⚠️ also reflects that tests of these subsystems are gated by `nonet`. |
| `group` — 4 files, 131 functions | Deep read of `sender_key.go` two longest functions and `chat.go` event paths | |
| `av`, `av/video` | Deep read of `processor.go` (decode pipeline), `rtp.go` (sequencing), `metrics.go` | |
| `examples/*` | Apply L-011 fix across all examples; otherwise low priority | |

Per the prompt's "iterative until done" rule, a follow-up session should resume from `toxcore_friends.go` (the largest unread file) and proceed through the packages above in the listed order. Findings discovered in those reads should be appended to this report rather than replacing it.
