# Implementation Plan: Core Protocol Functionality Completion

## Project Context
- **What it does**: Pure Go implementation of the Tox Messenger core protocol for secure peer-to-peer communication without centralized infrastructure
- **Current goal**: Complete non-functional protocol paths (DHT maintenance, friend reconnection, group discovery) that prevent the protocol from working as advertised
- **Estimated Scope**: Medium (7 functions above complexity threshold, 6 core gaps requiring fix)

## Goal-Achievement Status
| Stated Goal | Current Status | This Plan Addresses |
|-------------|---------------|---------------------|
| DHT peer discovery & routing | ⚠️ Partial (maintenance loop empty) | **Yes** |
| Friend connection management | ⚠️ Partial (reconnection is no-op) | **Yes** |
| Group chat with DHT discovery | ⚠️ Partial (QueryGroup always fails) | **Yes** |
| Documentation coverage >80% | ✅ Achieved (93.0% overall) | No |
| Pure Go, no CGo | ✅ Achieved | No |
| Noise-IK encryption | ✅ Achieved | No |
| Forward secrecy | ✅ Achieved | No |
| Identity obfuscation | ✅ Achieved | No |
| UDP proxy support | ⚠️ Partial (UDP bypasses proxy) | **Yes** (Priority 6) |
| Symmetric NAT relay | ❌ Not implemented | Roadmap only |
| Test suite reliability | ⚠️ Partial (port conflicts) | **Yes** |
| Clean resource management | ⚠️ Partial (double-close on shutdown) | **Yes** |

## Metrics Summary
- Complexity hotspots on goal-critical paths: **7** functions above threshold (complexity >10)
  - `doFriendConnections`: 15.0 (core gap)
  - `AnnounceGroup`: 13.2 (core gap)
  - `startOnion`/`startI2P`: 12.7 (bootstrap)
- Duplication ratio: **0.57%** (excellent, below 3% threshold)
- Doc coverage: **93.0%** overall, **99.2%** function coverage (exceeds 80% target)
- Package coupling: `toxcore` (6.0), `async` (3.5), `transport` (3.0)
- Circular dependencies: **0** (clean architecture)
- Dead/unreferenced functions: **144** (maintenance opportunity)

## Research Findings

### Community & Issues
- **GitHub Issue #43**: qTox CI/CD integration request from c-toxcore maintainer (iphydf) — demonstrates external interest and need for protocol compatibility
- **Security concern**: flynn/noise has GHSA-g9mp-8g3h-3c5c (nonce handling) — already mitigated in ROADMAP Priority 5

### Dependency Status
- `flynn/noise v1.1.0`: Nonce overflow vulnerability documented; key rotation before exhaustion planned
- `golang.org/x/crypto v0.48.0`: Current, no known critical issues
- `go-i2p/sam3 v0.33.92`: Actively maintained, used for I2P transport

---

## Implementation Steps

### Step 1: Implement DHT Maintenance Loop
- **Deliverable**: Working `doDHTMaintenance()` in `toxcore.go:1104-1126` that refreshes routing table
- **Dependencies**: None
- **Goal Impact**: Enables sustained DHT connectivity (Goal: DHT peer discovery)
- **Acceptance**: DHT routing table maintains >10 nodes after 10 minutes of operation with no external connections
- **Validation**: 
  ```bash
  go test -race -count=1 -run TestDHTMaintenance ./... 
  # Must show routing table refresh activity in logs
  ```
- **Specific Changes**:
  1. When node count < 10: call `bootstrapManager.Bootstrap()` for known bootstrap nodes
  2. When node count ≥ 10: issue `FIND_NODE` queries targeting self-key every ~60 seconds
  3. Use `t.iterationCount % 1200 == 0` gating (60s at 50ms tick)

### Step 2: Implement Friend Reconnection Logic
- **Deliverable**: Working `doFriendConnections()` in `toxcore.go:1128-1153` that re-establishes connections
- **Dependencies**: Step 1 (DHT must be functional for lookups)
- **Goal Impact**: Enables friend reconnection (Goal: Friend connection management)
- **Acceptance**: Offline friend transitions to `ConnectionUDP` or `ConnectionTCP` within 30s of coming online
- **Validation**:
  ```bash
  go test -race -count=1 -run TestFriendReconnection ./...
  # Must show connection status change from None to UDP/TCP
  ```
- **Specific Changes**:
  1. When `closestNodes` is non-empty: send friend-request retry via `pendingFriendRequests`
  2. Update `friend.ConnectionStatus` upon receiving ping response
  3. Integrate with existing `retryPendingFriendRequests()` mechanism

### Step 3: Implement Group DHT Query Response Handling
- **Deliverable**: Working `QueryGroup()` in `dht/group_storage.go:220` that collects responses
- **Dependencies**: Step 1 (DHT maintenance required for reliable queries)
- **Goal Impact**: Enables group discovery (Goal: Group chat with DHT discovery)
- **Acceptance**: `QueryGroup()` returns valid `GroupAnnouncement` for announced groups within 5s
- **Validation**:
  ```bash
  go test -race -count=1 -run TestGroupDHTQuery ./dht/...
  # Must show successful query response collection
  ```
- **Specific Changes**:
  1. Add pending-response registry keyed on query nonce (similar to DHT ping tracking)
  2. Register response handler via transport's `RegisterHandler`
  3. Collect responses with 5-second timeout, return best `GroupAnnouncement`

### Step 4: Fix UDP Transport Double-Close
- **Deliverable**: Idempotent `Close()` on `UDPTransport` and `TCPTransport` in `transport/udp.go`, `transport/tcp.go`
- **Dependencies**: None
- **Goal Impact**: Clean resource management (Goal: Robust error handling)
- **Acceptance**: Zero "Error closing" messages in test output
- **Validation**:
  ```bash
  go test -race -count=1 -tags nonet ./... 2>&1 | grep -c "Error closing"
  # Must return 0
  ```
- **Specific Changes**:
  1. Add `closeOnce sync.Once` to `UDPTransport` struct
  2. Wrap `Close()` body in `t.closeOnce.Do(func() { ... })`
  3. Apply same pattern to `TCPTransport`

### Step 5: Fix Test Port Conflicts
- **Deliverable**: Ephemeral port allocation in `constants_test.go` and affected tests
- **Dependencies**: None
- **Goal Impact**: Reliable test suite (Goal: Test suite reliability)
- **Acceptance**: `go test -race -count=3 -tags nonet .` passes all three runs
- **Validation**:
  ```bash
  go test -race -count=3 -tags nonet . 2>&1 | grep -c FAIL
  # Must return 0
  ```
- **Specific Changes**:
  1. Replace `testDefaultPort = 33445` with helper function `getTestPort()` using `net.Listen("tcp", ":0")`
  2. Update `TestProxyConfiguration` SOCKS5 case to use `tcpPort: 0`
  3. Add `t.Cleanup()` with port release verification in discovery tests

### Step 6: Move Test Infrastructure Out of Production Code
- **Deliverable**: Test-only code moved from `toxcore.go` to `toxcore_test.go` or `testing/` package
- **Dependencies**: None
- **Goal Impact**: Clean production code (Goal: Clean API design)
- **Acceptance**: `globalFriendRequestRegistry` not present in production binary
- **Validation**:
  ```bash
  go build -o /tmp/toxcore-bin . && nm /tmp/toxcore-bin | grep -c globalFriendRequest
  # Must return 0 (symbol not exported in production)
  ```
- **Specific Changes**:
  1. Move `globalFriendRequestRegistry`, `registerGlobalFriendRequest`, `checkGlobalFriendRequest` to `toxcore_test.go`
  2. Remove `processPendingFriendRequests()` call from `Iterate()`
  3. Expose `RegisterPacketInjector(func(*transport.Packet, net.Addr))` for test hooks

---

## Deferred Work (Not in This Plan)

These items are documented in ROADMAP.md and require larger architectural decisions:

| Item | Rationale for Deferral |
|------|------------------------|
| SOCKS5 UDP Association (Priority 1) | Requires protocol research (RFC 1928 UDP ASSOCIATE) |
| Symmetric NAT Relay (Priority 3) | Requires new TCP relay protocol design |
| I2P Listen Support (Priority 4) | Requires persistent destination management design |
| flynn/noise Key Rotation (Priority 5) | Security-critical, requires careful design review |
| staticcheck CI Integration (Priority 6) | Low impact, can be done independently |
| Scalability Refactors (Priorities 7-12) | Multi-month engineering efforts per REPORT.md |

---

## Validation Commands

```bash
# After all steps complete, full validation:
go test -race -count=3 -tags nonet ./...

# Metrics verification:
go-stats-generator analyze . --skip-tests 2>&1 | grep -E "(Complexity|Coverage|Duplication)"

# Build verification (no test symbols in production):
CGO_ENABLED=0 go build -o /tmp/toxcore-test . && rm /tmp/toxcore-test
```

## Success Criteria

1. **DHT Maintenance**: Routing table stays populated >10 nodes after 10-minute idle period
2. **Friend Reconnection**: Offline→Online transition triggers connection within 30s
3. **Group Discovery**: `QueryGroup()` returns valid results for announced groups
4. **Clean Shutdown**: Zero spurious error logs on `Kill()`
5. **Test Reliability**: `go test -count=3` passes all three runs
6. **Code Hygiene**: Test-only code not in production binary
