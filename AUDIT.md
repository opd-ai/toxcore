# IMPLEMENTATION GAP AUDIT — 2026-07-02

## Project Architecture Overview
- **Purpose/intent (README + ROADMAP):** pure-Go Tox core with DHT discovery, friend/message/file/group flows, ToxAV, async/offline messaging, and multi-network transport.
- **Module/deps (go.mod):** module `github.com/opd-ai/toxcore`, Go `1.25.0` (`toolchain go1.25.11`), no unused required module detected by `go mod why -m`.
- **Package topology:** `go list ./...` reported 28 packages; internal dependency graph shows 108 internal edges with `toxcore`, `transport`, `crypto`, and `av` as primary hubs.
- **Ownership split (docs + layout):**
  - `toxcore` facade/orchestration
  - `transport` network backends + negotiation
  - `dht` discovery/routing
  - `async` offline/forward-secrecy storage paths
  - `av`/`av/audio`/`av/video`/`av/rtp` media stack
  - `nameresolver` Namecoin integration layer
  - `capi` C bindings

## Gap Summary
| Category | Count | Critical | High | Medium | Low |
|----------|-------|----------|------|--------|-----|
| Stubs/TODOs | 5 | 0 | 0 | 2 | 3 |
| Dead Code | 0 | 0 | 0 | 0 | 0 |
| Partially Wired | 3 | 0 | 1 | 2 | 0 |
| Interface Gaps | 2 | 0 | 0 | 1 | 1 |
| Dependency Gaps | 0 | 0 | 0 | 0 | 0 |

## Implementation Completeness by Package
| Package | Exported Functions | Implemented | Stubs | Dead | Coverage |
|---------|-------------------:|------------:|------:|-----:|----------|
| addressing | 2 | 2 | 0 | 0 | N/A |
| async | 275 | 275 | 0 | 0 | N/A |
| audio | 59 | 59 | 0 | 0 | N/A |
| av | 120 | 118 | 2 | 0 | N/A |
| bootstrap | 12 | 12 | 0 | 0 | N/A |
| common | 4 | 4 | 0 | 0 | N/A |
| crypto | 95 | 95 | 0 | 0 | N/A |
| dht | 202 | 202 | 0 | 0 | N/A |
| factory | 12 | 12 | 0 | 0 | N/A |
| file | 41 | 41 | 0 | 0 | N/A |
| friend | 58 | 58 | 0 | 0 | N/A |
| group | 73 | 73 | 0 | 0 | N/A |
| interfaces | 2 | 2 | 0 | 0 | N/A |
| internal | 56 | 56 | 0 | 0 | N/A |
| limits | 5 | 5 | 0 | 0 | N/A |
| main | 49 | 48 | 1 | 0 | N/A |
| messaging | 65 | 65 | 0 | 0 | N/A |
| nameresolver | 14 | 12 | 2 | 0 | N/A |
| nodes | 0 | 0 | 0 | 0 | N/A |
| noise | 42 | 42 | 0 | 0 | N/A |
| ratchet | 19 | 19 | 0 | 0 | N/A |
| real | 11 | 11 | 0 | 0 | N/A |
| rtp | 38 | 38 | 0 | 0 | N/A |
| simulation | 11 | 11 | 0 | 0 | N/A |
| toxcore | 142 | 142 | 0 | 0 | N/A |
| toxnet | 81 | 81 | 0 | 0 | N/A |
| transport | 445 | 442 | 3 | 0 | N/A |
| video | 110 | 110 | 0 | 0 | N/A |

## Findings
### CRITICAL
- [ ] None.

### HIGH
- [ ] Runtime bitrate control is partially wired (state-only update, no peer signaling/encoder reconfiguration) — `/home/runner/work/toxcore/toxcore/toxav.go:888-941`, `/home/runner/work/toxcore/toxcore/toxav.go:956-1010` — `AudioSetBitRate`/`VideoSetBitRate` explicitly state they only mutate local call state and do not send `BitrateControlPacket` or reconfigure active encoders, so adaptive bitrate is not end-to-end effective — blocks the stated A/V adaptive bitrate behavior in README. — **Remediation:** Implement full bitrate-control path in `ToxAV.AudioSetBitRate`/`ToxAV.VideoSetBitRate`: construct+send signaling packet, apply updates to active encoder instances, and add integration tests that verify remote side bitrate changes after control messages; validate with `go build ./...`, `go vet ./...`, and focused `go test ./... -run BitRate`.

### MEDIUM
- [ ] Name resolver write-path contract is stubbed in Phase 1 — `/home/runner/work/toxcore/toxcore/nameresolver/nmcd.go:151-160` — `RegisterName` and `RenewName` return `ErrRegistrationNotSupported` unconditionally while `Resolver` interface advertises these operations — blocks full Namecoin lifecycle management (resolve+register+renew). — **Remediation:** Implement nmcd-backed register/renew transaction flows in `NmcdResolver`, including key-based signing/funding, update path tests, and error-wrapping consistency checks; validate with `go build ./...` and `go test ./nameresolver/...`.
- [ ] Nym transport listener path is declared but unreachable by design (`Listen` always errors) — `/home/runner/work/toxcore/toxcore/transport/nym_transport_impl.go:90-100` — API surface exposes listener semantics but backend never provides hosting support, leaving the component partially wired for inbound use cases — blocks symmetric Nym service hosting under the transport abstraction. — **Remediation:** Add real inbound Nym service integration (or split interface so unsupported operations are not exposed on this transport); add transport-level listener tests and compatibility matrix entries; validate with `go build ./...` and `go test ./transport/...`.
- [ ] Lokinet transport exposes unsupported interface methods (`Listen`/`DialPacket`) that always return errors — `/home/runner/work/toxcore/toxcore/transport/lokinet_transport_impl.go:81-92`, `/home/runner/work/toxcore/toxcore/transport/lokinet_transport_impl.go:147-156` — component is structurally present but not fully connected to `NetworkTransport` capability set for inbound/packet use — blocks full Lokinet parity under shared transport contracts. — **Remediation:** Implement SNApp/listener and packet path support, or narrow advertised capabilities per transport type and enforce at call sites; verify with `go test ./transport/...`, `go build ./...`, `go vet ./...`.

### LOW
- [ ] Example audio streaming transport adapter contains explicit send stub — `/home/runner/work/toxcore/toxcore/examples/audio_streaming_demo/main.go:207-218` — demo `Send` returns “not implemented” and `RegisterHandler` intentionally skips registration, so the example is not end-to-end runnable networking logic — blocks example completeness for integrators using this sample as reference. — **Remediation:** Wire demo adapter to an actual transport-backed send/receive loop (or mark sample as pseudocode-only in README/docs and remove runnable expectation); validate by executing the example and adding a minimal smoke test.
- [ ] Address connectivity docstring is stale vs behavior — `/home/runner/work/toxcore/toxcore/transport/address.go:220-241` — comment says `.nym` connectivity is “stub only/not implemented”, while `IsConnectivitySupported` currently returns `true` for Nym outbound dial — creates misleading contract documentation and false diagnostics for maintainers/users. — **Remediation:** Align comments with current behavior/support matrix and add a doc consistency unit test around `ConnectivityStatus`/`IsConnectivitySupported` semantics.
- [ ] Phase marker text in C API docs is stale and ambiguous — `/home/runner/work/toxcore/toxcore/capi/toxav_c.go:15` — the package header still says, *“This will be implemented in Phase 1 as part of the core infrastructure.”* even though the file already contains a full C API implementation, which can mislead maintainers about feature completeness. — **Remediation:** Replace with explicit completion wording (for example: “Implemented in Phase 1; C API is currently available and maintained”) and, if any remaining work exists, append a tracked issue/milestone reference; verify docs consistency in `go doc`/README references.

## False Positives Considered and Rejected
| Candidate Finding | Reason Rejected |
|-------------------|----------------|
| `/home/runner/work/toxcore/toxcore/async/storage_limits_unix.go:8` Windows disk-space stub | Intentional build-tag compatibility shim for non-Windows targets; fulfills documented platform abstraction. |
| `/home/runner/work/toxcore/toxcore/nameresolver/disabled.go` zero-allocation stub resolver | Intentional Null Object for `NameResolverEnabled=false`; this is documented API behavior, not unfinished work. |
| `/home/runner/work/toxcore/toxcore/av/video/processor.go:493-502` no-op keyframe/golden-frame methods | Explicitly documented no-op behavior for simple/passthrough encoder variant; not a hidden stub. |
| `/home/runner/work/toxcore/toxcore/capi/toxcore_c.go:120` empty `main()` | Required for cgo shared library entrypoint pattern in `package main`; not dead/incomplete code. |
| Exported APIs with no internal callers | Public module API may be consumed externally; absence of internal call sites alone is insufficient to classify as dead code. |
