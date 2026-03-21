# Implementation Gaps — 2026-03-21

## Gap 1: DHT Maintenance Loop Is Non-Functional

- **Stated Goal**: The README promises a working DHT for peer discovery and routing: _"Distributed Hash Table for peer discovery and routing via the distributed hash table."_ The Tox protocol requires periodic node refresh, routing table maintenance, and stale-entry expiry to maintain network connectivity.
- **Current State**: `toxcore.go:1104–1126` — `doDHTMaintenance()` enters the sparse-routing-table branch but executes zero instructions. The inner block is exclusively comments: `"Basic bootstrap attempt - no advanced retry logic yet"` and `"Further maintenance features will be added in future updates"`. Long-running nodes will never re-discover peers after the initial bootstrap phase fades.
- **Impact**: A Tox instance that bootstraps successfully will gradually lose routing-table entries as nodes go offline. Without periodic `FIND_NODE` queries, the routing table empties and the instance becomes isolated from the network within minutes. Friend connections, DHT lookups, and group discovery all depend on a healthy routing table.
- **Closing the Gap**: Implement a refresh cycle inside `doDHTMaintenance()` using the existing `dht.RoutingTable.FindClosestNodes()` and `dht.BootstrapManager` APIs. At minimum: (1) when node count < 10, call `bootstrapManager.Bootstrap()` for each known bootstrap node; (2) when node count ≥ 10, issue `FIND_NODE` queries targeting the self-key every ~60 seconds. Use `t.iterationCount % N == 0` gating or a ticker goroutine.

---

## Gap 2: Friend Reconnection Is a No-Op

- **Stated Goal**: The README documents `OnFriendMessage`, friend connection tracking, and implies that friends that go offline will be re-connected when they return: _"Friend Online: Messages are delivered immediately via real-time messaging. Friend Offline: Messages automatically fall back to asynchronous messaging."_ The implication is that connections are managed dynamically.
- **Current State**: `toxcore.go:1128–1153` — `doFriendConnections()` iterates over offline friends, performs a DHT lookup for each, and then executes `_ = friendID` — an explicit discard with the comment `"Basic reconnection attempt - advanced logic to be added later"`. No packet is sent, no state is updated.
- **Impact**: Once a friend goes offline, the connection status remains `ConnectionNone` indefinitely even if the friend returns to the network. Real-time messaging between reconnected friends cannot be established. All messages fall through to async delivery even when the friend is online, increasing latency and consuming async storage capacity unnecessarily.
- **Closing the Gap**: When `closestNodes` is non-empty for an offline friend, send a friend-request retry packet through the existing `pendingFriendRequests` queue and `retryPendingFriendRequests()` mechanism. Update `friend.ConnectionStatus` to `ConnectionTCP` or `ConnectionUDP` upon receiving a ping response from the friend's address.

---

## Gap 3: Group Chat DHT Discovery Always Fails

- **Stated Goal**: The README and `group/chat.go` describe group chat with peer discovery: _"group chat functionality with role management."_ `dht/group_storage.go` exports `AnnounceGroup()` and `QueryGroup()` as the DHT-based discovery layer for groups.
- **Current State**: `dht/group_storage.go:220` — `QueryGroup()` sends a query packet to DHT nodes and then unconditionally returns `fmt.Errorf("DHT query sent, response handling not yet implemented")`. Every caller receives an error regardless of network conditions. `AnnounceGroup()` is fire-and-forget with no confirmation.
- **Impact**: Groups cannot be discovered by remote peers via the DHT. `group.JoinGroup()` and any invite-based flow that relies on DHT lookup will always fail. Group chat is limited to same-process instances in tests.
- **Closing the Gap**: Implement response collection in `QueryGroup()` using a pending-response registry (similar to DHT ping tracking in `dht/routing.go`). Register a response handler keyed on the query nonce, collect responses with a 5-second timeout, and return the best `GroupAnnouncement`. Wire the response handler into the transport's `RegisterHandler` call.

---

## Gap 4: Test Infrastructure Embedded in Production Code

- **Stated Goal**: The project aspires to a clean, idiomatic Go codebase with proper separation of concerns. The comment at `toxcore.go:92` itself states: _"NOTE: This is ONLY for testing and should not be used in production code paths."_
- **Current State**: `toxcore.go:90–117,1101,1527–1547` — A global in-process friend-request registry (`globalFriendRequestRegistry`), its accessors (`registerGlobalFriendRequest`, `checkGlobalFriendRequest`), and its caller (`processPendingFriendRequests`) are compiled into every production binary. `Iterate()` checks this registry on every tick, adding a mutex acquisition and map lookup to the hot path.
- **Impact**: Every production `Iterate()` call incurs an unnecessary `sync.RWMutex` lock + map read. The test-registry functions occupy production binary space. The pattern sets a precedent for further test-only code leaking into production paths. It also makes the network delivery path harder to reason about: friend requests can arrive via two channels (network transport and in-process registry) with no clear priority.
- **Closing the Gap**: Move `globalFriendRequestRegistry`, `registerGlobalFriendRequest`, `checkGlobalFriendRequest`, and `processPendingFriendRequests` into `toxcore_test.go` or the `testing/` helper package. Remove the `processPendingFriendRequests()` call from `Iterate()`. Replace with a testable hook: expose a `RegisterPacketInjector(func(*transport.Packet, net.Addr))` method guarded by a build tag or documented as test-only.

---

## Gap 5: Nym Network Support Is Inconsistently Described and Partially Functional

- **Stated Goal**: README states: _"Nym .nym: Nym mixnet addresses (functional via SOCKS5 proxy, requires local Nym client)."_
- **Current State**: Three layers contradict each other:
  1. `transport/address.go:219,235,257` — `IsSupported()` returns `false`; `SupportMessage()` returns `"stub only - requires Nym SDK websocket client integration (not yet implemented)"`.
  2. `transport/network_transport_impl.go:649–659` — `NymTransport.Listen()` always returns an error.
  3. `transport/network_transport_impl.go:665–689` — `NymTransport.Dial()` works via SOCKS5 if a local Nym client is running.
  The address routing layer (`transport/address.go:IsSupported`) blocks Nym addresses before they reach the functional Dial path.
- **Impact**: Even when a Nym SOCKS5 client is running, connections to `.nym` addresses will fail at the address-routing layer before the functional Dial code is reached. The feature is unreachable through the normal transport stack. Users following the README will be unable to connect to Nym addresses.
- **Closing the Gap**: Either (a) update the README and `transport/address.go` to correctly document Nym as "outbound Dial only via SOCKS5; hosting not supported; Listen always fails," or (b) fix `IsSupported()` to return `true` for Nym addresses and ensure the Dial path is reachable from the multi-transport routing. `NymTransport.Listen` should return a clear `ErrNymNotImplemented` sentinel rather than a generic error string.

---

## Gap 6: UDP Transport Double-Close on Shutdown

- **Stated Goal**: The project promises robust resource management: _"Proper resource management with defer statements, secure memory wiping, and connection management."_
- **Current State**: `toxcore.go:1715–1720` and `transport/udp.go:144–166` — `Kill()` calls `t.udpTransport.Close()` which propagates through `NegotiatingTransport → NoiseTransport → UDPTransport`. The underlying `UDPTransport.Close()` is invoked twice per shutdown, generating a logged error `"Error closing UDP connection: use of closed network connection"` on every `Kill()`. The log is visible in all test runs with `-v`.
- **Impact**: Each shutdown logs a spurious error, polluting log output and making it harder to distinguish real close errors from expected ones. The double-close can delay port release in tests (OS may not release the port until the second close returns), directly causing the port-conflict test failures in Gap 7. In production, spurious close errors may alert monitoring systems unnecessarily.
- **Closing the Gap**: Add `sync.Once` idempotent-close protection to `UDPTransport`:
  ```go
  type UDPTransport struct {
      closeOnce sync.Once
      // ...
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
  Apply the same pattern to `TCPTransport.Close()`. Validate: `go test -race -count=1 -tags nonet . 2>&1 | grep -c "Error closing"` must return 0.

---

## Gap 7: Root Package Test Suite Fails Due to Hardcoded Port Reuse

- **Stated Goal**: The README installation section states: `go test ./...` should pass as a verification step.
- **Current State**: `constants_test.go:8` — `testDefaultPort = 33445` is used across multiple test cases within `TestProxyConfiguration` and the local-discovery tests. When run sequentially (as `go test ./...` does), a previous subtest's Tox instance holds port 33445 when the next subtest tries to bind TCP on the same port, producing `"listen tcp 0.0.0.0:33445: bind: address already in use"`. Four tests fail: `TestBothTransportsEnabled`, `TestProxyConfiguration/SOCKS5_proxy_with_TCP`, `TestLocalDiscoveryIntegration`, `TestLocalDiscoveryCleanup`.
- **Impact**: `go test ./...` exits with a non-zero status. CI fails. The README's own verification command does not pass. This undermines trust in the test suite and masks real failures.
- **Closing the Gap**: In `TestProxyConfiguration`, the `"SOCKS5 proxy with TCP"` case should use `tcpPort: 0` (OS-assigned) or pick a free port via `net.Listen("tcp", ":0")`. For discovery tests, use `t.Cleanup()` with a short `time.Sleep` to confirm port release. Add `t.Parallel()` where subtests are independent. Alternatively, add `testDefaultPort` as a per-test random-ephemeral-port helper. Validate: `go test -race -count=3 -tags nonet .` passes all three runs.

---

## Gap 8: Async Message Delivery Not Driven by `Iterate()`

- **Stated Goal**: README states: _"Friend Offline: Messages automatically fall back to asynchronous messaging for store-and-forward delivery when the friend comes online."_ The implied behavior is that `Iterate()` drives periodic delivery checks.
- **Current State**: `toxcore.go:1173–1177` — The async manager block inside `doMessageProcessing()` is empty: `// Basic async message check - advanced processing handled by async package`. The async manager (`async.AsyncManager`) runs as an independent goroutine and does its own timing, but scheduled deliveries triggered by friend online-status changes are not coordinated through `Iterate()`.
- **Impact**: Async delivery timing is decoupled from the Tox event loop. Deliveries may be attempted while the transport is in a transient state (e.g., during `Kill()`). Applications that rely on `Iterate()` as the single event-pump will observe unpredictable delivery timing. The README's implication that `Iterate()` orchestrates all protocol activity is inaccurate.
- **Closing the Gap**: Add an explicit `t.asyncManager.TickDelivery()` (or equivalent) call inside `doMessageProcessing()`. The `async.AsyncManager` already has the pending-delivery state; exposing a tick method makes the delivery loop deterministic and testable.

---

## Gap 9: Go Version Requirement in README Is Out of Date

- **Stated Goal**: README states: _"Requirements: Go 1.23.2 or later."_
- **Current State**: `go.mod:3-4` specifies `go 1.24.0` with `toolchain go1.24.12`. Go's toolchain directive enforces a minimum version; attempting to build with Go 1.23.x will produce a toolchain version error.
- **Impact**: Developers following the README installation instructions with Go 1.23.x will receive a confusing toolchain error with no mention of the actual minimum version. This is a documentation correctness gap that affects first-time users.
- **Closing the Gap**: Update README "Requirements" to `Go 1.24.0 or later`. Cross-check `go.mod` on every release to keep this in sync. Consider adding a CI step that checks for version consistency between README and `go.mod`.

---

## Gap 10: Protocol Migration Phase 4 Has No Tracking or Timeline

- **Stated Goal**: The README's Noise Protocol section documents a four-phase migration plan, with Phases 1–3 marked ✅. Phase 4 is _"Full migration with performance optimization"_ with no completion marker.
- **Current State**: There is no GitHub issue, ROADMAP.md entry, or code TODO linked to Phase 4. The `NegotiatingTransport` and `NoiseTransport` implementations are functional but the codebase still contains explicit `"Fallback to unencrypted transmission for unknown peers"` behavior in `transport/noise_transport.go` — the legacy fallback that Phase 4 should eliminate.
- **Impact**: Without tracking, Phase 4 will remain indefinitely incomplete. The "fallback to unencrypted" path means that peers who have not completed Noise-IK handshakes communicate in plaintext, which directly contradicts the forward-secrecy guarantee stated in the README.
- **Closing the Gap**: Create a ROADMAP.md entry for Phase 4 with: (a) criteria for removing the unencrypted fallback, (b) performance benchmarks that must pass before the fallback is removed, and (c) a deprecation timeline for legacy-protocol peers. Set `EnableLegacyFallback: false` as the `DefaultProtocolCapabilities()` default once the network has sufficient Noise-IK adoption.
