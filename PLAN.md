# Implementation Plan: Core Protocol Correctness & Code Quality

> **Generated:** 2026-03-21  
> **Tool:** `go-stats-generator` v1.0.0 + repository analysis  
> **Scope:** 835 functions, 2,119 methods, 24 packages, 184 non-test files, 31,475 LOC

---

## Project Context

- **What it does**: Pure Go, no-CGo implementation of the Tox P2P messaging protocol with Noise-IK forward secrecy, identity obfuscation, and multi-network transport (UDP/TCP/Tor/I2P).
- **Current goal**: Make the core Tox protocol work correctly end-to-end — DHT peer discovery, friend reconnection, and group chat discovery are all stubbed out in the shipped binary.
- **Estimated Scope**: **Large** (>15 items above threshold across correctness, safety, and quality metrics)

---

## Goal-Achievement Status

| # | Stated Goal | Status | This Plan Addresses |
|---|-------------|--------|---------------------|
| 1 | Pure Go, no CGo | ✅ Achieved | No |
| 2 | Core Tox protocol (friend/messaging) | ⚠️ Partial — key loops are stubs | **Yes** (Steps 1–4) |
| 3 | Multi-network IPv4/IPv6 | ✅ Achieved | No |
| 4 | Multi-network Tor | ⚠️ UDP leaks | **Yes** (Step 14) |
| 5 | Multi-network I2P Listen | ⚠️ Not implemented | No (future work) |
| 6 | Clean Go API | ⚠️ Test code in production hot path | **Yes** (Step 6) |
| 7 | C bindings | ⚠️ Partial | No |
| 8 | Noise-IK protocol | ⚠️ Unencrypted fallback contradicts forward secrecy | **Yes** (Step 13) |
| 9 | Forward secrecy | ✅ Achieved | No |
| 10 | Identity obfuscation | ✅ Achieved | No |
| 11 | Audio/Video (ToxAV) | ✅ Achieved | No |
| 12 | Async messaging | ⚠️ Not driven by `Iterate()` | **Yes** (Step 5) |
| 13 | Group chat | ⚠️ DHT discovery always returns error | **Yes** (Step 3) |
| 14 | Proxy support (SOCKS5) | ⚠️ UDP bypasses proxy | **Yes** (Step 14) |
| 15 | NAT traversal | ⚠️ Symmetric NAT relay missing | No (future work) |
| 16 | Local discovery | ✅ Achieved | No |
| 17 | Documentation >80% | ✅ **Now 92.8%** (ROADMAP figure 64.31% is stale) | No |
| 18 | Test suite passes (`go test ./...`) | ❌ Fails due to port conflicts + double-close | **Yes** (Steps 7–8) |

**Overall: 10/18 fully achieved (56%), 8 partial or broken**

---

## Metrics Summary

| Metric | Value | Threshold | Status |
|--------|-------|-----------|--------|
| Functions with complexity > 9.0 (core packages) | 7 | <5 small / 5–15 medium | ⚠️ Medium |
| Duplication ratio | 0.56% | <3% | ✅ Excellent |
| Doc coverage (overall) | 92.8% | >80% | ✅ Exceeds target |
| Doc coverage (methods) | 91.1% | >80% | ✅ Exceeds target |
| Bare error returns (high severity) | 393 | 0 ideal | ❌ Large |
| Goroutine leak risk (high severity) | 50 | 0 ideal | ❌ Large |
| Resource leaks (critical severity) | 22 | 0 ideal | ❌ Large |
| Panic in library code | 1 | 0 | ❌ `dht/routing.go:147` |
| Deprecated APIs pending removal | 20 | 0 | ⚠️ Medium |
| Staticcheck in CI | Disabled | Enabled | ❌ |

### Complexity Hotspots on Goal-Critical Paths

| Function | Package | Overall | Cyclomatic | File |
|----------|---------|---------|------------|------|
| `Start` | bootstrap | 14.0 | 10 | bootstrap/server.go:114 |
| `startOnion` | bootstrap | 12.7 | 9 | bootstrap/server.go:323 |
| `startI2P` | bootstrap | 12.7 | 9 | bootstrap/server.go:383 |
| `tryConnectionMethods` | transport | 10.1 | 7 | transport/advanced_nat.go:140 |
| `processPacketLoop` | transport | 10.1 | 7 | transport/tcp.go:400 |
| `decryptObfuscatedMessage` | async | 9.8 | 6 | async/client.go:965 |
| `doFriendConnections` | toxcore | 9.8 | 6 | toxcore.go:1129 |
| `pruneDeadNodes` | dht | 9.8 | 6 | dht/maintenance.go:336 |
| `GetDominantAddressType` | dht | 9.6 | 7 | dht/address_detection.go:178 |
| `SendMessage` | group | 9.6 | 7 | group/chat.go:840 |

---

## Implementation Steps

Steps are ordered: **prerequisites first**, then **descending impact on stated goals**.

---

### Step 1: Implement DHT Maintenance Loop

**Deliverable**: `toxcore.go:doDHTMaintenance()` — replace the empty comment block with a real maintenance cycle using existing APIs.

**Problem**: `doDHTMaintenance()` (lines 1104–1126) checks node count but executes zero instructions in the sparse-routing-table branch. Nodes bootstrapped successfully will gradually lose all routing-table entries and become isolated within minutes.

**Changes**:
- When `nodeCount < 10`: call `t.bootstrapManager.Bootstrap()` for each node in `t.bootstrapNodes`
- When `nodeCount >= 10`: issue `FIND_NODE` queries targeting the local public key using `t.dht.FindClosestNodes()`
- Gate on `t.iterationCount % 120 == 0` (every ~6 seconds at 50ms tick) to avoid flooding
- Use existing `dht.RoutingTable` and `dht.BootstrapManager` — no new APIs needed

**Dependencies**: None — this is the prerequisite for Steps 2 and 3.

**Goal Impact**: Goals 2, 13 (friend reconnection and group discovery both depend on a live DHT).

**Acceptance**: After 10+ minutes of operation, `t.dht.NodeCount()` remains non-zero and monotonically increasing.

**Validation**:
```bash
go test -tags nonet -race -run TestDHTMaintenance ./...
go-stats-generator analyze . --skip-tests --format json --sections functions 2>/dev/null \
  | python3 -c "import json,sys; fs=json.load(sys.stdin)['functions']; \
    [print(f) for f in fs if f['name']=='doDHTMaintenance' and f['complexity']['cyclomatic']>1]"
```

---

### Step 2: Implement Friend Reconnection

**Deliverable**: `toxcore.go:doFriendConnections()` — replace `_ = friendID` discard with actual connection attempt using DHT lookup results.

**Problem**: Lines 1128–1153 find the closest DHT nodes for each offline friend but then explicitly discard the results with `_ = friendID`. Friend connections never recover after disconnect.

**Changes**:
- When `closestNodes` is non-empty for an offline friend, extract the friend's last-known address from `closestNodes[0]`
- Re-queue the friend's public key via `retryPendingFriendRequests()` mechanism (already exists)
- On receiving a ping response from the friend's address, update `friend.ConnectionStatus` to `ConnectionUDP` or `ConnectionTCP`
- Guard with exponential backoff: attempt no more than once per `2^attempts × 5s`

**Dependencies**: Step 1 (DHT must be maintaining routes for lookup results to be meaningful).

**Goal Impact**: Goal 2 — friends that go offline can reconnect; real-time delivery resumes instead of always falling back to async.

**Acceptance**: Two test peers that disconnect and reconnect re-establish `ConnectionUDP` status within 30 seconds.

**Validation**:
```bash
go test -tags nonet -race -run TestFriendReconnection ./...
```

---

### Step 3: Fix Group Chat DHT Discovery

**Deliverable**: `dht/group_storage.go:QueryGroup()` — implement response collection instead of unconditional error return.

**Problem**: Line 220 unconditionally returns `fmt.Errorf("DHT query sent, response handling not yet implemented")`. Every `group.JoinGroup()` call fails. Group chat is limited to same-process test instances.

**Changes**:
- Add a pending-response registry in `dht/group_storage.go` keyed on query nonce (pattern already used in `dht/routing.go` for ping tracking)
- Register a response handler via `transport.RegisterHandler` before sending the query packet
- Collect responses with a 5-second timeout using a buffered channel
- Return the `GroupAnnouncement` with the most recent timestamp from collected responses
- Wire the response handler into the existing packet dispatch in `dht/handler.go`

**Dependencies**: Step 1 (DHT must have live nodes to query).

**Goal Impact**: Goal 13 — group chat discovery works across process boundaries.

**Acceptance**: `group.JoinGroup()` returns a non-error response when a matching group is announced on the DHT.

**Validation**:
```bash
go test -tags nonet -race -run TestGroupDHTDiscovery ./dht/... ./group/...
```

---

### Step 4: Integrate Async Delivery with `Iterate()`

**Deliverable**: `toxcore.go:doMessageProcessing()` — add explicit `t.asyncManager.TickDelivery()` call.

**Problem**: Lines 1173–1177 contain only a comment. The `async.AsyncManager` runs as an independent goroutine with its own 30-second poll cycle, decoupled from the Tox event loop. Deliveries may be attempted during `Kill()` shutdown, and applications treating `Iterate()` as the single event pump observe unpredictable delivery timing.

**Changes**:
- Add `TickDelivery()` method to `async.AsyncManager` that triggers one delivery check cycle
- Call `t.asyncManager.TickDelivery()` from `doMessageProcessing()` when a friend transitions to online
- The independent goroutine continues for background polling; `TickDelivery()` adds deterministic delivery on status change

**Dependencies**: Step 2 (friend online-status changes must be correctly detected).

**Goal Impact**: Goal 12 — async message delivery is deterministic and testable; delivery occurs within one `Iterate()` tick of friend coming online.

**Acceptance**: `go test -race -run TestAsyncDeliveryOnFriendOnline` passes deterministically across 5 runs.

**Validation**:
```bash
go test -tags nonet -race -count=5 -run TestAsyncDeliveryOnFriendOnline ./...
```

---

### Step 5: Fix Panic in Library Code

**Deliverable**: `dht/routing.go:147` — replace `panic()` with error return.

**Problem**: go-stats-generator flags a `panic()` call in library code (non-main package) at `dht/routing.go:147`. Library code must never panic on recoverable errors — panics propagate to the application without warning.

**Changes**:
- Identify the panic condition at line 147 (likely an invariant assertion or index bounds check)
- Replace with `return fmt.Errorf("dht: routing invariant violated: %w", err)` and propagate to caller
- Update the caller to handle the error

**Dependencies**: None.

**Goal Impact**: Goal 6 (clean API) — no panic in library code.

**Acceptance**: `go-stats-generator analyze . --skip-tests --format json --sections patterns 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); [print(x) for x in d['patterns']['anti_patterns']['performance_antipatterns'] if x['type']=='panic_in_library']"` returns empty.

**Validation**:
```bash
go-stats-generator analyze . --skip-tests --format json --sections patterns 2>/dev/null \
  | python3 -c "import json,sys; d=json.load(sys.stdin); \
    panics=[x for x in d['patterns']['anti_patterns']['performance_antipatterns'] if x['type']=='panic_in_library']; \
    print('panics:', len(panics))"
```

---

### Step 6: Fix UDP Transport Double-Close

**Deliverable**: `transport/udp.go:UDPTransport.Close()` — idempotent close with `sync.Once`.

**Problem**: `Kill()` triggers double-close of `UDPTransport`, generating spurious `"Error closing UDP connection: use of closed network connection"` log entries. This pollutes monitoring output and delays port release in tests, directly causing port-conflict failures.

**Changes**:
```go
type UDPTransport struct {
    closeOnce sync.Once
    // existing fields...
}

func (t *UDPTransport) Close() error {
    var err error
    t.closeOnce.Do(func() {
        t.cancel()
        err = t.conn.Close()
    })
    return err
}
```
- Apply the same `sync.Once` pattern to `TCPTransport.Close()`

**Dependencies**: None — but Step 7 depends on this.

**Goal Impact**: Goal 6 (clean API), Goal 18 (test suite passes).

**Acceptance**: `go test -race -count=1 -tags nonet . 2>&1 | grep -c "Error closing"` returns `0`.

**Validation**:
```bash
go test -race -count=3 -tags nonet . 2>&1 | grep -c "Error closing"
```

---

### Step 7: Fix Test Suite Port Conflicts

**Deliverable**: `constants_test.go` + affected test cases — eliminate hardcoded port 33445 in tests.

**Problem**: `constants_test.go:8` defines `testDefaultPort = 33445`, used across `TestProxyConfiguration`, `TestBothTransportsEnabled`, `TestLocalDiscoveryIntegration`, and `TestLocalDiscoveryCleanup`. Sequential subtests fail with `"listen tcp 0.0.0.0:33445: bind: address already in use"`. `go test ./...` exits non-zero.

**Changes**:
- Add a `freePort(t *testing.T) int` helper that uses `net.Listen("tcp", ":0")` to obtain an OS-assigned ephemeral port
- Replace `testDefaultPort = 33445` with `freePort(t)` calls at the start of each conflicting subtest
- Add `t.Cleanup()` with port-release confirmation for local-discovery tests
- Add `t.Parallel()` to independent subtests within `TestProxyConfiguration`

**Dependencies**: Step 6 (double-close must be fixed so port is released before next subtest).

**Goal Impact**: Goal 18 — `go test -tags nonet -race ./...` passes reliably.

**Acceptance**: `go test -race -count=3 -tags nonet ./...` exits 0 all three times.

**Validation**:
```bash
go test -race -count=3 -tags nonet ./... && echo "PASS"
```

---

### Step 8: Remove Test Infrastructure from Production Code

**Deliverable**: `toxcore.go` — move `globalFriendRequestRegistry` and its accessors out of the production binary.

**Problem**: Lines 90–117, 1101, 1527–1547 embed a test-only in-process friend-request registry into every production `Iterate()` call, adding a `sync.RWMutex` lock + map lookup to the hot path. The comment at line 92 explicitly states: *"This is ONLY for testing and should not be used in production code paths."*

**Changes**:
- Move `globalFriendRequestRegistry`, `registerGlobalFriendRequest`, `checkGlobalFriendRequest`, and `processPendingFriendRequests` into the `testing/` package or a `_test.go` file
- Remove the `processPendingFriendRequests()` call from `Iterate()`
- Replace with a `RegisterPacketInjector(func(*transport.Packet, net.Addr))` method gated with a `//go:build testing` build tag
- Update any test that uses the registry to call the injector via the `testing/` package

**Dependencies**: Step 7 (test suite must be healthy before this refactor to catch regressions).

**Goal Impact**: Goal 6 (clean API) — production hot path free of test code.

**Acceptance**: `go build -tags nonet ./...` succeeds; `grep -n "globalFriendRequestRegistry" toxcore.go` returns nothing.

**Validation**:
```bash
go build -tags nonet ./... && \
grep -c "globalFriendRequestRegistry" toxcore.go | grep -q "^0$" && echo "PASS"
```

---

### Step 9: Fix Critical Resource Leaks (22 sites)

**Deliverable**: 22 resource leak sites in `transport/network_transport_impl.go` (6), `transport/proxy.go` (2), `crypto/decrypt.go` (2), `file/transfer.go` (2), and 5 other files.

**Problem**: go-stats-generator flags 22 critical resource leaks. Common pattern: opened file/connection handle not closed when an error occurs after open but before successful return. These cause file descriptor exhaustion under load.

**Changes** (per site):
- Wrap every `os.Open`, `os.Create`, net dial, and cipher init with `defer resource.Close()` immediately after successful open, before any early return
- For conditional closes (e.g., transfer success should not close), use a `cleanup` bool flag pattern:
  ```go
  cleanup := true
  defer func() { if cleanup { r.Close() } }()
  // ... on success:
  cleanup = false
  return r, nil
  ```
- `transport/network_transport_impl.go` is the highest priority (6 sites)

**Dependencies**: None.

**Goal Impact**: Goal 6 (production quality), implicitly Goals 3–5 (transports must not leak FDs under load).

**Acceptance**:
```bash
go-stats-generator analyze . --skip-tests --format json --sections patterns 2>/dev/null \
  | python3 -c "import json,sys; d=json.load(sys.stdin); \
    rl=[x for x in d['patterns']['anti_patterns']['performance_antipatterns'] if x['type']=='resource_leak']; \
    print('resource_leaks:', len(rl))"
```
Target: `resource_leaks: 0`.

**Validation**:
```bash
go test -race -tags nonet ./transport/... ./crypto/... ./file/...
```

---

### Step 10: Fix Goroutine Leak Risks (50 sites)

**Deliverable**: 50 goroutine-leak-risk sites, prioritized by package: `transport/tcp.go` (5), `async/manager.go` (4), `bootstrap/server.go` (4), `dht/maintenance.go` (3), `transport/noise_transport.go` (3).

**Problem**: go-stats-generator flags 50 high-severity goroutine leak risks. Common pattern: `go func()` launched without a corresponding cancel/done signal, leaving goroutines running after the owning struct is closed.

**Changes** (per site):
- Ensure every goroutine launched in `Start()` / `Connect()` / background loops reads from a `ctx.Done()` channel or a `done chan struct{}`
- In `transport/tcp.go:processPacketLoop()` (complexity 10.1), add `select { case <-t.done: return; default: }` at the top of the loop
- In `async/manager.go`, pass the `AsyncManager` context to all background goroutines
- In `bootstrap/server.go:Start()` (complexity 14.0), track all spawned goroutines in a `sync.WaitGroup` and drain on `Shutdown()`

**Dependencies**: Step 6 (transport close must be idempotent before goroutine shutdown is added).

**Goal Impact**: Goal 6 (production quality); prevents goroutine explosion under high load (REPORT.md §3.2).

**Acceptance**:
```bash
go-stats-generator analyze . --skip-tests --format json --sections patterns 2>/dev/null \
  | python3 -c "import json,sys; d=json.load(sys.stdin); \
    gl=[x for x in d['patterns']['anti_patterns']['performance_antipatterns'] if x['type']=='goroutine_leak']; \
    print('goroutine_leaks:', len(gl))"
```
Target: `goroutine_leaks: 0` (or ≤5 for intentional background services with documented lifecycle).

**Validation**:
```bash
go test -race -count=2 -tags nonet ./transport/... ./async/... ./bootstrap/... ./dht/...
```

---

### Step 11: Fix Noise Phase 4 — Remove Unencrypted Fallback

**Deliverable**: `transport/noise_transport.go` — disable unencrypted fallback for unknown peers; implement proper handshake queuing.

**Problem**: GAPS.md §10 documents that `transport/noise_transport.go` contains `"Fallback to unencrypted transmission for unknown peers"`. Phase 4 of the Noise migration (tracked nowhere, no timeline) must eliminate this fallback. While the fallback exists, peers that have not completed Noise-IK communicate in plaintext, contradicting the forward-secrecy guarantee stated in the README.

**Changes**:
- Set `DefaultProtocolCapabilities().EnableLegacyFallback = false`
- When no established Noise session exists for a peer: queue the outgoing packet, initiate the Noise-IK handshake, and flush the queue upon handshake completion
- Return `ErrHandshakePending` to the caller for packets queued during handshake (instead of falling back to plaintext)
- Add a ROADMAP entry for Phase 4 completion criteria: (a) network adoption metric, (b) benchmark gate, (c) deprecation timeline

**Dependencies**: Steps 6, 10 (transport must be stable and goroutine-safe before changing handshake flow).

**Goal Impact**: Goals 8, 9 — Noise-IK forward secrecy guarantee is actually enforced.

**Acceptance**: `go test -race -tags nonet -run TestNoiseOnlyTransmission ./transport/...` passes; no plaintext fallback occurs in transport logs.

**Validation**:
```bash
go test -race -tags nonet -run TestNoise ./transport/... && \
grep -r "Fallback to unencrypted" transport/ | wc -l | grep -q "^0$" && echo "PASS"
```

---

### Step 12: Fix Bare Error Returns in Core Packages (393 sites → prioritize top 3 files)

**Deliverable**: Systematic error-context wrapping in `toxcore/toxcore.go` (41 sites), `async/obfs.go` (22 sites), `toxcore/toxav.go` (19 sites).

**Problem**: go-stats-generator reports 393 bare error returns (no `fmt.Errorf("context: %w", err)` wrapping). This makes error diagnosis from logs impossible — callers cannot distinguish which internal operation failed.

**Changes** (top 3 files first, then remaining in order of site count):
- Wrap each `return err` with `return fmt.Errorf("<function>: %w", err)` where the function name provides enough context
- For `async/obfs.go:DecryptObfuscatedMessage` (complexity 9.6), add per-step context: `"obfs: step 3 key derivation: %w"`
- Follow existing project convention from `crypto/` package which already uses this pattern

**Dependencies**: None for mechanical changes; Step 7 ensures tests pass to catch regressions.

**Goal Impact**: Goal 6 (production-quality error handling per project conventions).

**Acceptance**:
```bash
go-stats-generator analyze . --skip-tests --format json --sections patterns 2>/dev/null \
  | python3 -c "import json,sys; d=json.load(sys.stdin); \
    be=[x for x in d['patterns']['anti_patterns']['performance_antipatterns'] \
        if x['type']=='bare_error_return' and any(f in x.get('file','') for f in ['toxcore.go','obfs.go','toxav.go'])]; \
    print('bare_errors_in_top3:', len(be))"
```
Target: `bare_errors_in_top3: 0`.

**Validation**:
```bash
go build ./... && go test -race -tags nonet ./...
```

---

### Step 13: Fix Go Version Requirement in README

**Deliverable**: `README.md` — change "Go 1.23.2 or later" to "Go 1.24.0 or later".

**Problem**: GAPS.md §9 documents that `go.mod` specifies `go 1.24.0` with `toolchain go1.24.12`, but README says "Go 1.23.2 or later". Developers with Go 1.23.x receive a confusing toolchain error on `go build`.

**Changes**:
- Update "Requirements" section in README: `Go 1.24.0 or later`
- Add CI step to `toxcore.yml` that verifies README version matches `go.mod`: `grep "1.24" README.md || exit 1`

**Dependencies**: None.

**Goal Impact**: Accuracy of stated requirements; reduces first-time contributor friction.

**Acceptance**: `grep "1.24.0" README.md` returns a match.

**Validation**:
```bash
grep "Go 1.24" README.md && echo "PASS"
```

---

### Step 14: Implement UDP Proxy Support (SOCKS5 UDP Association)

**Deliverable**: `transport/proxy.go` — implement SOCKS5 UDP ASSOCIATE command; `transport/tor_transport.go` — route all DHT traffic through proxy.

**Problem**: ROADMAP Priority 1. `transport/proxy.go:18` explicitly notes: "UDP traffic is not proxied (passed through to underlying transport)." For users expecting Tor/SOCKS5 anonymity, DHT UDP packets leak directly, de-anonymizing the user.

**Changes**:
- Implement `transport/socks5_udp.go` with RFC 1928 UDP ASSOCIATE command:
  - Dial SOCKS5 control connection, send `UDP ASSOCIATE` request
  - Wrap outgoing UDP packets in SOCKS5 UDP request header: `RSV(2) FRAG(1) ATYP(1) DST.ADDR DST.PORT DATA`
  - Receive and strip SOCKS5 UDP reply headers on incoming packets
- Add `UDPProxyEnabled bool` to `transport.ProxyConfig`
- When `UDPProxyEnabled` is true, use the SOCKS5 UDP relay instead of the direct UDP socket in `UDPTransport`
- Update `transport/tor_transport.go` to enable `UDPProxyEnabled` when a Tor SOCKS5 endpoint is configured
- Update README proxy documentation: remove "UDP leaks" warning

**Dependencies**: Steps 6, 10 (transport must be stable before adding SOCKS5 UDP layer).

**Goal Impact**: Goals 4, 14 — Tor and SOCKS5 proxy users have full traffic anonymization.

**Acceptance**: `go test -tags nonet -run TestSOCKS5UDPAssociate ./transport/...` passes using a local mock SOCKS5 server.

**Validation**:
```bash
go test -race -tags nonet -run TestProxy ./transport/... && echo "PASS"
```

---

### Step 15: Enable staticcheck in CI

**Deliverable**: `.github/workflows/toxcore.yml` — uncomment staticcheck installation and run steps; resolve flagged issues.

**Problem**: staticcheck is installed but commented out in CI (lines visible in workflow). Disabled static analysis means bugs that `go vet` misses go undetected until runtime.

**Changes**:
- Uncomment staticcheck in `.github/workflows/toxcore.yml`
- Run `staticcheck ./...` locally and address each finding:
  - Add `//nolint:SA...` with justification for intentional patterns (e.g., C API naming conventions in `capi/`)
  - Fix legitimate issues (unused parameters, deprecated API calls, incorrect error returns)
- The 20 deprecated API items identified by go-stats-generator are candidates for cleanup here

**Dependencies**: Steps 8, 12 (remove test code from production and fix error wrapping first, to reduce staticcheck noise).

**Goal Impact**: Goal 6 (production quality); catches regressions in CI that `go vet` misses.

**Acceptance**: `staticcheck ./...` exits 0 in CI.

**Validation**:
```bash
go install honnef.co/go/tools/cmd/staticcheck@latest && staticcheck ./...
```

---

## Open Questions (Require Design Decisions Before Implementation)

These items from REPORT.md §7 and ROADMAP.md cannot be planned without team input:

1. **DHT Sybil resistance**: Should S/Kademlia extensions (crypto proof-of-work for node ID binding) be added, and if so, does this break compatibility with the original Tox DHT protocol?
2. **Symmetric NAT relay topology**: Should relay nodes be DHT-discovered (adds lookup latency) or maintained as a separate bootstrap overlay?
3. **Async message replication factor**: What k (k=3? k=5?) balances storage overhead against 99.99% delivery guarantee?
4. **Group chat consistency model**: Causal consistency (vector clocks), eventual consistency (CRDTs), or total ordering via group leader?
5. **Light client mode**: Should mobile devices delegate DHT operations to a trusted always-online companion node?
6. **`Iterate()` refactor**: Decoupling the event loop into separate goroutines (REPORT.md §3.3) changes the threading model — requires a semver-major API decision.

---

## Validation Commands (Run After Each Step)

```bash
# Full codebase build (all platforms)
GOOS=linux GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=arm64 go build ./...
GOOS=windows GOARCH=amd64 go build ./...
GOOS=js GOARCH=wasm go build ./...

# Test suite (excludes network-dependent tests)
go test -race -count=3 -tags nonet ./...

# go-stats-generator baseline check
go-stats-generator analyze . --skip-tests 2>/dev/null \
  | grep -E "documentation|duplication|functions|packages"

# Anti-pattern count (track progress)
go-stats-generator analyze . --skip-tests --format json --sections patterns 2>/dev/null \
  | python3 -c "
import json, sys
d = json.load(sys.stdin)
perfs = d['patterns']['anti_patterns']['performance_antipatterns']
from collections import Counter
counts = Counter(x['type'] for x in perfs)
for t, n in sorted(counts.items()):
    print(f'{n:4d} {t}')
print(f'{len(perfs):4d} TOTAL')
"

# vet
go vet ./...
```

---

## Dependency Graph

```
Step 1 (DHT maintenance)
  └─ Step 2 (friend reconnect)
       └─ Step 4 (async + Iterate)
  └─ Step 3 (group DHT discovery)

Step 6 (double-close fix)
  └─ Step 7 (port conflict fix)
       └─ Step 8 (remove test code)
            └─ Step 15 (staticcheck)
  └─ Step 10 (goroutine leaks)
       └─ Step 11 (Noise Phase 4)
       └─ Step 14 (UDP proxy)

Step 5 (panic in library) — independent
Step 9 (resource leaks) — independent
Step 12 (bare error returns) — independent
Step 13 (README version) — independent
```
