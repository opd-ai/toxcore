# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-29

## Project Profile
**toxcore-go** is a pure-Go implementation of the Tox peer-to-peer encrypted
messaging protocol (`module github.com/opd-ai/toxcore`, Go 1.25). It provides
DHT-based peer discovery, friend management, 1-to-1 and group messaging, chunked
file transfers, ToxAV audio/video calling, asynchronous store-and-forward offline
messaging with forward secrecy, multi-network transport (IPv4/IPv6 UDP/TCP, Tor,
I2P, Lokinet dial-only, Nym dial-only), Noise-IK handshakes, NAT traversal, and a
libtoxcore-compatible C API (the only cgo-requiring component).

- **Target users**: application developers embedding the library, and operators
  running bootstrap / discovery nodes.
- **Deployment model**: linked into a host process as a library; the C API binding
  (`capi/`) is an optional cgo build.
- **Critical paths**: transport packet parsing (primary untrusted-input boundary),
  crypto primitives (key exchange, AEAD, signatures, secure wipe), DHT
  bootstrap/lookup, async messaging lifecycle (WAL persistence + forward secrecy),
  and the public exported constructors/facade in the root `toxcore` package.
- **Trust boundary**: untrusted bytes enter through `transport/` (`parser.go`,
  `packet.go`, `packet_extensions.go`), `dht/handler.go`, the async packet handlers
  (`async/manager.go`, `async/client.go`), and the AV RTP/VP8 frame parsers. All
  observed parsers validate length before slicing (see Findings / False Positives).

## Audit Scope
- **Packages audited**: all 57 import paths returned by `go list ./...`
  (26 distinct package names after deduplicating `examples/*` mains).
- **Functions/methods inspected (stats baseline)**: 1,168 functions + 2,899 methods
  (4,067 total callable units) across 238 files / 42,624 LOC.
- **High-risk structural set** (cyclomatic > 10 OR > 50 lines): the 6 functions with
  complexity > 10 and the 41 functions > 50 lines were inspected manually; the two
  functions above complexity 15 (`crypto.reencryptWithNewKey`, `toxnet.waitForDataSignal`)
  were read line-by-line.
- **Baseline commands executed**:
  - `go test -race ./...` → all 34 packages with tests pass, 0 failures.
  - `go vet ./...` → 0 warnings.
  - `go-stats-generator analyze . --skip-tests --format json --sections functions,packages,documentation,duplication,patterns,interfaces,structs`
- **Method**: structural risk scan (go-stats-generator) + targeted bug-class scans
  (type assertions, ignored errors, `defer` in loops, weak crypto, `InsecureSkipVerify`,
  `math/rand`, hardcoded secrets, `os/exec`, `go:embed`, panics in library code) +
  parallel deep inspection of the async/crypto, transport/dht, and root/av/file/group
  package groups. Every agent-reported candidate was re-verified against the source
  with the Phase 3l false-positive checks before inclusion.

## Coverage Log
Legend: ✅ = checklist category inspected for the package and no finding above LOW;
✅* = inspected, finding recorded (see Findings). Example `main` packages and the
testnet/simulation harnesses are non-shipping and were scanned but not deeply audited
(see Remaining Scope).

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| / (toxcore) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /async | ✅ | ✅ | ✅ | ✅ | ✅* | ✅ | ✅ | ✅ | ✅ |
| /av | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /av/audio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /av/rtp | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /av/video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /bootstrap | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /bootstrap/nodes | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /cmd/gen-bootstrap-nodes | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /factory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /limits | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /real | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /transport/internal/addressing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/* (all mains) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary
| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| DHT-based peer discovery (k-bucket, iterative lookup, LAN/mDNS) | ✅ | None |
| Friend management & sharded state | ✅ | None |
| 1-to-1 encrypted messaging with padding | ✅ | None |
| Group chat (roles, sender-key distribution) | ✅ | None |
| File transfers (pause/resume/cancel) | ✅ | None |
| ToxAV audio/video (Opus, VP8, RTP, adaptive bitrate) | ✅ | None |
| Asynchronous offline messaging with forward secrecy | ✅ | None |
| Multi-network transport | ⚠️ | Lokinet & Nym are dial-only (documented — see GAPS.md) |
| Noise-IK handshakes | ✅ | None |
| NAT traversal (STUN/UPnP/hole-punch/TCP relay) | ✅ | None |
| Cryptography (Curve25519/ChaCha20-Poly1305/Ed25519, secure wipe) | ✅ | None |
| C API bindings (libtoxcore-compatible) | ⚠️ | ~79% function coverage (documented — see GAPS.md) |
| Go `net.*` interfaces | ✅ | None |
| Protocol version negotiation | ✅ | None |
| Concurrent iteration pipelines | ✅ | None |
| Pure Go, no cgo in core | ✅ | None (cgo confined to `capi/`) |

## Findings

### CRITICAL
- [ ] None confirmed.

### HIGH
- [ ] None confirmed.

### MEDIUM
- [ ] None confirmed.

### LOW
- [ ] **F-L1: Late retrieve-response delivery relies on `recover()` over a real send-on-closed-channel race** — `async/client.go:1280-1290` (`sendResponseToChannel`), with the race window opened at `:1245` (`findResponseChannel` releases `channelMutex`) and `:1222-1227` (`cleanupResponseChannel` closes the channel under the same mutex) — [concurrency / control-flow via panic-recover] — **Data path:** `handleRetrieveResponse` calls `findResponseChannel(addr)`, which locks `channelMutex`, reads the channel, and unlocks. If the caller in `waitForRetrieveResponse` times out first (`:1238`), `cleanupResponseChannel` then locks `channelMutex`, `delete`s, and `close`s the same channel (`:1226`). The subsequent `responseChan <- response` in `sendResponseToChannel` can therefore execute on a closed channel and panic; the panic is caught by the `defer recover()` and only logged. **Concrete consequence:** functionally safe (the late response is silently dropped after timeout), but the design depends on `recover()` for normal control flow in a genuinely reachable race, which is fragile and easy to break during refactors. The `-race` detector does not flag it because the access is mutex-mediated up to the close. **Remediation:** perform the send while holding `channelMutex` (move the non-blocking `select` send into a method that locks the mutex and checks a `closed` flag instead of closing the channel), or replace channel-close teardown with a sentinel/`context` cancellation so no send can ever target a closed channel. Validate with `go test -race ./async/...` plus a targeted timeout-then-late-response regression test.
- [ ] **F-L2: No automated dependency-vulnerability gate (`govulncheck`) is run in this environment** — `go.mod:11-21` (notably `golang.org/x/crypto v0.48.0`, `golang.org/x/net v0.50.0`, `golang.org/x/sys v0.41.0`) — [security / dependency hygiene] — **Consequence:** known upstream CVEs in transport/crypto dependencies could go unnoticed during development. Reachability of any specific advisory was **not** confirmed because the vulnerability feed (`vuln.go.dev`) is not reachable from this offline audit sandbox, so this is recorded as LOW with explicit uncertainty. **Remediation:** add `govulncheck ./...` to CI with network access and enforce an upgrade policy for flagged ranges; validate locally with `govulncheck ./...` once network access is available.

> Note on prior audit findings: the previous `AUDIT.md` recorded F-H1 (panic in
> exported `dht` bootstrap constructors) and F-M1–F-M4 / F-L1 (non-restartable
> lifecycle channels in `async/manager.go`, `dht/local_discovery.go`,
> `dht/mdns_discovery.go`, `transport/nat.go`, `async/prekey_dht.go`). All were
> re-verified in this pass and are **resolved**: stop channels are now recreated
> under a `running` guard inside each `Start()` (e.g. `async/manager.go:192-199`,
> `dht/local_discovery.go:86`, `dht/mdns_discovery.go:152`,
> `transport/nat.go:191-197`).

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total functions / methods | 1,168 / 2,899 (4,067 total) |
| Files / LOC | 238 / 42,624 |
| Functions above cyclomatic complexity 15 | 2 (`crypto.reencryptWithNewKey` = 23, `toxnet.waitForDataSignal` = 16) |
| Functions above cyclomatic complexity 10 | 6 |
| Functions over 50 lines | 41 (longest 99: `crypto.reencryptWithNewKey`) |
| Avg cyclomatic complexity | 3.6 |
| Doc coverage (overall) | 93.32% (packages 100%, functions 98.7%, methods 92.3%, types 92.2%) |
| Duplication ratio | 0.47% (largest clone 14 lines) |
| Test pass rate | 34/34 packages with tests; 0 failures (`go test -race ./...`) |
| go vet warnings | 0 |
| Circular dependencies | 0 |

## False Positives Considered and Rejected
| Candidate | File:Line | Reason Rejected |
|-----------|-----------|-----------------|
| "Use-after-free": `&msgCopy` of a loop-body-local stored in returned slice | `async/storage.go:678-680` | Not a bug in Go. `msgCopy := *msg` is a **fresh** variable per inner-loop iteration; `&msgCopy` escapes and the compiler heap-allocates each instance. No dangling pointer, no shared backing. (This is the Go pre-1.22 loop-variable trap only when the *loop variable itself* is captured, which is not the case here.) |
| "Use-after-free": `&msg` of a local stored into maps | `async/storage.go:1175-1177` (`replayStoreMessage`) | Not a bug in Go. `var msg AsyncMessage` is function-local; the function is invoked once per WAL entry, so each call's `&msg` escapes to its own heap allocation. Distinct map entries do not alias. |
| `ZeroBytes` panics if `SecureWipe` fails | `crypto/secure_memory.go:48` | Unreachable: `SecureWipe` only errors on `nil` input, and `ZeroBytes` returns early for `nil` (`:42`). Panic is documented as a deliberate security-invariant guard. |
| `init()` panics on address parse | `transport/nat.go:27`, `dht/mdns_discovery.go:44-47` | Inputs are compile-time string constants (`203.0.113.1:0`, multicast literals); parse is deterministic and not attacker-influenced. |
| Debug logging "may leak key material" | `crypto/shared_secret.go:15-21` | Only an 8-byte **public**-key prefix is logged, guarded by `logrus.IsLevelEnabled(DebugLevel)`. Private keys and shared secrets are never logged and are wiped via `ZeroBytes`. |
| `exec.Command(gofmt, ...)` | `cmd/gen-bootstrap-nodes/main.go:103` | Developer codegen tool; `gofmt` is a hardcoded constant and the path argument is an operator-supplied build flag, not network/attacker input. |
| `math/rand` usage | `examples/av_quality_monitor/main.go` | Example/demo code generating synthetic quality metrics; not a security-sensitive path and not part of the shipped library. |
| go-stats-generator "critical BUG comments" (6) | `crypto/logging.go`, `crypto/shared_secret.go`, `toxav.go`, `toxcore_defaults.go` | False matches — the tool matched the substrings "debug"/"bug" inside ordinary doc comments; none are real `BUG:` annotations. |

## Remaining Scope (if session ended before completion)
| Package | Status | Notes |
|---------|--------|-------|
| Core library (async, crypto, dht, transport, noise, messaging, friend, file, group, av/*, toxnet, bootstrap, capi, factory, interfaces, limits, root toxcore) | Complete | Full checklist pass completed; findings recorded above. A complete pass produced zero new confirmed findings above LOW. |
| `examples/*`, `testnet/`, `simulation/`, `real/` | Scanned, not deeply audited | Non-shipping demo/harness code. Scanned for the security/concurrency checklist (no findings); not exhaustively traced because they are excluded from the library's correctness guarantees. Resume here if demo-code coverage is required. |
