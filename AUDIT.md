# AUDIT — 2026-03-21

## Project Goals

toxcore-go is a **pure Go, CGo-free** implementation of the Tox peer-to-peer encrypted messaging protocol. Per the README and inline documentation it promises:

1. **Core Tox protocol**: friend requests, friend messages, self-management (name, status, nospam), Tox ID addressing.
2. **State persistence**: `GetSavedata()`, `NewFromSavedata()`, `Load()`.
3. **Multi-network support**: IPv4/IPv6 (full), Tor .onion, I2P .b32.i2p, Nym .nym, Lokinet .loki.
4. **Noise Protocol Framework (IK)**: forward secrecy, KCI resistance, mutual authentication.
5. **Protocol version negotiation**: auto-negotiation between Legacy and Noise-IK; Phase 1–3 marked complete.
6. **Proxy support**: HTTP/SOCKS5 with acknowledged limitation that UDP bypasses the proxy.
7. **Asynchronous messaging**: store-and-forward for offline friends via `async` package.
8. **Audio/Video (ToxAV)**: Opus audio, VP8/H.264 video, call lifecycle, bit-rate management.
9. **File transfer**: `FileSend`, `FileSendChunk`, `FileControl`, callbacks.
10. **Group chat**: group creation, invitations, role management.
11. **DHT**: peer discovery, routing table, bootstrap server.
12. **C binding annotations**: `//export` markers for c-shared build mode.
13. **Test coverage >90%**, race-safe, table-driven tests.
14. **No circular dependencies**.
15. **Requirements: Go 1.23.2 or later** (README statement).

---

## Goal-Achievement Summary

| Goal | Status | Evidence |
|------|--------|----------|
| Pure Go, no CGo | ✅ Achieved | `go.mod` has no CGo imports; `capi/` uses `//export` directives via standard Go toolchain |
| Core Tox API (friend, message, self) | ✅ Achieved | `toxcore.go:2009–2943` — all documented callbacks and methods present |
| State persistence (savedata) | ✅ Achieved | `toxcore.go:371,997` — `GetSavedata`, `NewFromSavedata`, JSON+binary marshalling implemented |
| IPv4/IPv6 transport | ✅ Achieved | `transport/udp.go`, `transport/tcp.go` — functional UDP and TCP transports |
| Tor .onion transport | ⚠️ Partial | `transport/network_transport_impl.go:267` — Dial works via onramp; TCP-only (no UDP) |
| I2P .b32.i2p transport | ⚠️ Partial | SAM bridge integration via `go-i2p/onramp`; outbound functional, reliability untested without network |
| Nym .nym transport | ⚠️ Partial | `transport/network_transport_impl.go:649` — Dial proxied via SOCKS5; Listen always returns error; documented as "stub only" in `transport/address.go:235` |
| Lokinet .loki transport | ⚠️ Partial | SOCKS5 proxy path only; no direct Lokinet integration |
| Noise-IK protocol | ✅ Achieved | `transport/noise_transport.go`, `noise/handshake.go` — IK pattern implemented via `flynn/noise` |
| Protocol version negotiation (Phase 1–3) | ✅ Achieved | `transport/negotiating_transport.go` — NegotiatingTransport with legacy fallback |
| Protocol migration Phase 4 | ❌ Missing | README roadmap shows Phase 4 (performance optimization) not complete |
| Proxy support (HTTP/SOCKS5) | ⚠️ Partial | `toxcore.go:514` — TCP wrapping functional; UDP bypass acknowledged; test suite reveals SOCKS5+TCP fails in contiguous test runs |
| Async messaging (store-and-forward) | ✅ Achieved | `async/` package — ForwardSecurityManager, MessageStorage, obfuscation implemented and tested |
| ToxAV (audio/video) | ✅ Achieved | `toxav.go`, `av/` package — Opus codec, VP8 video, call lifecycle; all av/* tests pass |
| File transfer | ✅ Achieved | `file/transfer.go`, `toxcore.go:3142–3502` — full transfer lifecycle with chunk I/O |
| Group chat | ⚠️ Partial | `group/chat.go` implemented; DHT-based group discovery always returns error (`dht/group_storage.go:220`) |
| DHT peer discovery & bootstrap | ⚠️ Partial | Routing table, bootstrap manager functional; **periodic DHT maintenance loop is hollow** (`toxcore.go:1106–1125`) |
| C binding annotations | ✅ Achieved | `capi/toxcore_c.go` — ToxRegistry, C-safe pointer handling, `//export` directives throughout |
| Test coverage >90% | ⚠️ Partial | Root package test suite **fails** in standard runs due to port-conflict bugs; other packages pass |
| Race-safe | ⚠️ Partial | `-race ./...` with `-tags nonet` exposes 4 root-package failures; individual packages clean |
| No circular dependencies | ✅ Achieved | go-stats-generator: "No circular dependencies detected" |
| Go version requirement (README: 1.23.2) | ❌ Missing | `go.mod` requires `go 1.24.0`; README is outdated |

---

## Findings

### CRITICAL

- [x] **Test-registry code runs on every production `Iterate()` call** — `toxcore.go:1101,1527–1547` — `processPendingFriendRequests()` is called on every production tick inside `Iterate()`. The function header reads: _"NOTE: This is a testing helper that uses the global test registry for same-process testing."_ The global `globalFriendRequestRegistry` map (`toxcore.go:90–99`) is allocated in production memory, checked on every iteration, and is never guarded by a build tag. Any external caller that knows the public key of a Tox instance can insert synthetic friend requests into it via the unexported registry functions, which are reachable from the same process. This is an architectural boundary violation that adds latency to every `Iterate()` call and can mask bugs in the real network path. **Remediation:** Move `processPendingFriendRequests`, `registerGlobalFriendRequest`, `checkGlobalFriendRequest`, and `globalFriendRequestRegistry` into a `_test.go` file (or a separate `testing/` helper package imported only in tests). Remove the call from `Iterate()`. Provide a test hook via the existing `testing/` package instead. Validate with: `grep -rn "globalFriendRequestRegistry" --include="*.go" | grep -v "_test.go"` (should return zero results).

- [x] **Root package test suite fails in standard `-race` run** — `toxcore_integration_test.go:1157,1259,1597,1646` — Four tests fail when run in sequence due to hardcoded port 33445 reuse: `TestBothTransportsEnabled`, `TestProxyConfiguration/SOCKS5_proxy_with_TCP`, `TestLocalDiscoveryIntegration`, `TestLocalDiscoveryCleanup`. The subtest `"SOCKS5 proxy with TCP"` uses `tcpPort: testDefaultPort` (33445, `constants_test.go:8`), which conflicts with the prior UDP subtest still holding the same port. This means `go test -race ./...` exits with FAIL on the root package, breaking CI. **Remediation:** Replace `testDefaultPort` (33445) in `TestProxyConfiguration` and discovery tests with `:0` (OS-assigned ephemeral port) or use `net.Listen("tcp", ":0")` to find a free port before each subtest. Add `t.Parallel()` isolation where subtests share no port state. Validate with: `go test -race -count=3 -tags nonet . -run "TestProxyConfiguration|TestLocalDiscovery|TestBothTransports"`.

- [x] **DHT `QueryGroup()` always returns an error** — `dht/group_storage.go:220` — The function body sends a query packet to DHT nodes but immediately returns `fmt.Errorf("DHT query sent, response handling not yet implemented")`. Group chat peer discovery over DHT is silently non-functional. Any code path that calls `RoutingTable.QueryGroup` (e.g., group join) will always fail. **Remediation:** Implement response collection with a timeout channel pattern consistent with the existing DHT patterns in `dht/routing.go`. If the feature cannot be completed immediately, gate group chat DHT queries behind an explicit `ErrGroupDHTNotImplemented` sentinel and document this limitation in `group/chat.go`. Validate with: `go test -race -tags nonet ./dht/... -run TestQueryGroup`.

- [x] **Double-close of `UDPTransport` on every `Kill()`** — `toxcore.go:1716–1719`, `transport/udp.go:144–166` — Every `tox.Kill()` call logs `"Error closing UDP connection: use of closed network connection"`. The `t.udpTransport` field holds a `NegotiatingTransport` wrapping a `NoiseTransport` wrapping the raw `UDPTransport`. When `Kill()` calls `t.udpTransport.Close()`, it propagates through all layers and closes the underlying `net.PacketConn`. A second `Close()` is then triggered (exact path: `NegotiatingTransport.Close()` → `NoiseTransport.Close()` → `UDPTransport.Close()` once more). This generates logged errors on every shutdown and may interfere with port reuse in tests. **Remediation:** Add a `sync.Once` guard in `UDPTransport.Close()` so the underlying `net.PacketConn` is closed exactly once, returning `nil` on subsequent calls. Pattern: `t.closeOnce.Do(func() { err = t.conn.Close() })`. Validate with: `go test -race -count=1 -tags nonet . 2>&1 | grep -c "Error closing UDP"` (should return 0).

---

### HIGH

- [x] **`doDHTMaintenance()` body is a no-op** — `toxcore.go:1104–1126` — The function checks node count and enters a branch for sparse routing tables, but the inner block is entirely empty comments: `// Basic bootstrap attempt - no advanced retry logic yet` and `// Further maintenance features will be added in future updates`. There is no actual node refresh, no bucket maintenance, no expiry of stale entries. In a long-running session the routing table will stagnate. **Remediation:** Implement periodic `FIND_NODE` queries targeting the self-key (standard Tox DHT maintenance). The `dht.BootstrapManager` and `dht.RoutingTable.FindClosestNodes` are already available. Call `bootstrapManager.Bootstrap()` when `len(allNodes) < 10`, and send periodic `FIND_NODE` packets when `len(allNodes) >= 10`. Validate with: `go test -race -tags nonet ./dht/... -run TestDHTMaintenance`.

- [x] **`doFriendConnections()` performs no actual reconnection** — `toxcore.go:1128–1153` — For offline friends the code does `_ = friendID` (explicit discard) inside a DHT lookup block. The comment reads: `"Basic reconnection attempt - advanced logic to be added later"`. Friends that go offline are never reconnected through the protocol's normal re-establishment path. **Remediation:** Implement friend re-discovery by sending `FIND_NODE` requests for the friend's public key via DHT and queuing a friend request retransmit when a route is found. Use the existing `pendingFriendRequests` queue and `retryPendingFriendRequests()` as the retry mechanism. Validate with: a new integration test `TestFriendReconnection` that simulates offline/online transition.

- [ ] **`doMessageProcessing()` ignores async manager** — `toxcore.go:1155–1178` — The async manager check block is empty: `// Basic async message check - advanced processing handled by async package`. The async manager runs independently, but any scheduled async delivery that requires the main Tox iteration loop will not be driven from `Iterate()`. **Remediation:** Call `t.asyncManager.ProcessPendingDeliveries()` (or equivalent) inside `doMessageProcessing()` after the `mm.ProcessPendingMessages()` call. Validate with: `go test -race -tags nonet ./async/... -run TestAsyncDeliveryViaIterate`.

- [ ] **`AV` address resolution silently falls back to localhost** — `av/types.go:108–158` — When the address resolver fails or returns fewer than 6 bytes, the code silently substitutes `127.0.0.1:(10000 + friendNumber)` as the media endpoint. This means AV frames may be silently misdirected to a local port instead of failing loudly. **Remediation:** Return an error from `ResolveMediaAddr` when resolution fails rather than substituting a placeholder. Callers should handle the error. Validate with: `go test -race -tags nonet ./av/... -run TestResolveMediaAddr_Failure`.

- [ ] **Nym transport `Listen()` always returns error** — `transport/network_transport_impl.go:649–659` — The Listen implementation unconditionally returns `"Nym service hosting not supported via SOCKS5"`. The README states Nym is _"functional via SOCKS5 proxy"_ and lists it as a supported network. A peer hosting a Nym service address will never receive connections. **Remediation:** Either (a) remove Nym from the supported-network list in the README and mark it outbound-only, or (b) implement Nym service hosting via the Nym SDK websocket integration as described in `ErrNymNotImplemented`. Validate with: `go test -tags nonet ./transport/... -run TestNymTransport`.

---

### MEDIUM

- [ ] **Go version requirement mismatch** — `README.md` states "Requirements: Go 1.23.2 or later" but `go.mod:3` requires `go 1.24.0` with `toolchain go1.24.12`. Users following the README may attempt to build with Go 1.23.x and receive toolchain errors. **Remediation:** Update the README Installation section to read "Requirements: Go 1.24.0 or later". Validate with: `grep "Go 1" README.md` matches `go.mod`.

- [ ] **`globalFriendRequestRegistry` persists in production binary** — `toxcore.go:90–117` — The global map and its accessor functions are compiled into every production binary. Because they are unexported and not build-tag guarded, they cannot be removed by the linker. This is dead weight in production and a maintenance hazard. **Remediation:** See CRITICAL finding above; the fix is the same (move to `_test.go`).

- [ ] **Nym address marked "stub only" but README claims "functional"** — `transport/address.go:235,257` — `IsSupported()` returns `false` and `SupportMessage()` returns `"stub only - requires Nym SDK websocket client integration (not yet implemented)"`. The README's network table lists Nym as "functional via SOCKS5 proxy, requires local Nym client". The outbound Dial path does work via SOCKS5, but the address layer signals non-support, creating contradictory behavior between routing and transport layers. **Remediation:** Align `IsSupported()` to return `true` for the Nym Dial path and update `SupportMessage()` accordingly, OR update the README to describe Nym as "outbound-only, listen not supported". Validate with: `go test -tags nonet ./transport/... -run TestNymAddressSupport`.

- [ ] **Group chat DHT announcement is fire-and-forget** — `dht/group_storage.go:177–221` — `AnnounceGroup()` sends packets to DHT nodes but does not wait for confirmation or retry on failure. Combined with the missing `QueryGroup` implementation, group chat over DHT is effectively non-functional end-to-end. **Remediation:** Implement a callback-based or channel-based confirmation mechanism with retry. Validate with: `go test -tags nonet ./dht/... -run TestGroupAnnounce`.

- [ ] **`bootstrap/server.go:Start()` has highest complexity** — `bootstrap/server.go:~line 1` — Overall complexity score 14.0 (cyclomatic 10) per go-stats-generator. The function orchestrates clearnet, Tor onion, and I2P startup paths in a single function body. **Remediation:** Extract `startClearnet()`, `startOnion()`, and `startI2P()` sub-functions (partial decomposition already exists; `Start()` should be a thin orchestrator). Validate with: `go-stats-generator analyze . --skip-tests --format json | python3 -c "import json,sys; fns=json.load(sys.stdin)['functions']; [print(f['name'], f['complexity']['cyclomatic']) for f in fns if f['name']=='Start']"`.

- [ ] **`capi` unsafe pointer recovery masks invalid C pointer bugs** — `capi/toxcore_c.go:safeGetToxID` — The function uses `recover()` to catch panics from invalid C pointers. While this prevents crashes, it silently swallows programming errors in C code that passes stale pointers, making debugging very difficult. **Remediation:** Log a structured error with the pointer value when recovery occurs. Consider adding a debug build mode that panics instead of recovering. Validate with: review `capi/toxcore_c_test.go` for coverage of the invalid-pointer path.

---

### LOW

- [ ] **Low-cohesion packages** — `interfaces/` (cohesion 0.5), `limits/` (0.5), `common/` (0.6) — go-stats-generator reports these packages contain fewer functions than their file count warrants. They are close to empty utility packages. **Remediation:** Merge `interfaces/packet_delivery.go` into the `transport` package, and `limits/` constants into their primary consumer. No functional change.

- [ ] **24 code clone pairs detected (duplication ratio 0.57%)** — go-stats-generator identifies 375 duplicated lines across 24 clone pairs (largest: 17 lines). While the ratio is low, clones in security-critical error-handling paths are a maintenance risk. **Remediation:** Run `go-stats-generator analyze . --skip-tests --format json --sections duplication | jq '.duplication.clone_pairs'` to identify exact locations and consolidate with shared helper functions.

- [ ] **`bootstrap` package test coverage is 46%** — `bootstrap/server.go` — The `bootstrap` package covers the public-facing `BootstrapServer` API (clearnet, Tor, I2P startup paths). At 46% coverage, more than half of its code paths are untested, including the `startOnion()` (complexity 12.7) and `startI2P()` (complexity 12.7) functions that contain the most complex branching. This is the highest-risk untested code in the project. **Remediation:** Add unit tests for `startOnion()` and `startI2P()` using mock `net.Listener` implementations (following the existing mock-transport pattern in `testing/packet_delivery_sim.go`). Target ≥80% coverage. Validate with: `go test -tags nonet -cover ./bootstrap/... | grep coverage`.

- [ ] **`capi` and `dht` packages below 70% coverage** — `capi` at 67.1%, `dht` at 69.5% — Both packages fall below the project's stated >90% goal. `dht` contains the DHT routing and maintenance logic (already partially hollow per CRITICAL findings). `capi` contains the C binding layer with unsafe pointer handling — the highest-risk code in the project from a memory-safety perspective. **Remediation:** Add tests for `capi` error-recovery paths (null pointer, stale pointer, concurrent access). Add DHT routing tests for bucket overflow, node expiry, and stale-entry removal. Validate with: `go test -tags nonet -cover ./capi/... ./dht/...`.

- [ ] **`av/rtp/doc.go:116` notes video handler as placeholder** — `av/rtp/doc.go:116` — "Video handler is placeholder pending Phase 3 implementation." This is an inline documentation note that has not been updated to reflect current state. **Remediation:** Update the doc comment to accurately reflect whether Phase 3 RTP video handling is implemented.

---

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total source files | 184 |
| Total lines of code | 31,475 |
| Total functions | 835 |
| Total methods | 2,119 |
| Total structs | 283 |
| Total interfaces | 36 |
| Total packages | 24 |
| Average function length | 13.2 lines |
| Functions >50 lines | 17 (0.6%) |
| Functions >100 lines | 0 (0.0%) |
| Average cyclomatic complexity | 3.5 |
| High complexity functions (>10) | 0 |
| Peak complexity | `bootstrap.Start` — overall 14.0, cyclomatic 10 |
| Duplication ratio | 0.57% (24 clone pairs, 375 lines) |
| Circular dependencies | 0 |
| Root package test result | **FAIL** (port-conflict, 4 tests) |
| All other non-example packages | **PASS** |
| `go vet ./...` | **PASS** (clean) |

### Coverage by Package (`go test -tags nonet -cover`)

| Package | Coverage | Status |
|---------|----------|--------|
| messaging | 97.7% | ✅ |
| noise | 93.4% | ✅ |
| friend | 92.6% | ✅ |
| av/rtp | 90.7% | ✅ |
| av/video | 90.7% | ✅ |
| crypto | 89.1% | ✅ |
| file | 85.0% | ✅ |
| av/audio | 84.8% | ✅ |
| group | 82.5% | ✅ |
| net | 77.7% | ✅ |
| av | 77.0% | ✅ |
| async | 73.0% | ⚠️ |
| dht | 69.5% | ⚠️ below stated 90% goal |
| capi | 67.1% | ⚠️ below stated 90% goal |
| bootstrap | **46.0%** | ❌ critically low |
