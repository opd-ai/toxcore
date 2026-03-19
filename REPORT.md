# REPORT.md — toxcore-go Distributed Systems Scalability Analysis

> **Generated:** 2026-03-19 | **Scope:** 835 functions, 24 packages, 31,453 LOC  
> **Repository:** [opd-ai/toxcore](https://github.com/opd-ai/toxcore) @ `923e45c`

---

## 1. Executive Summary

toxcore-go is a well-architected pure Go implementation of the Tox P2P messaging protocol with strong cryptographic foundations (Noise-IK, forward secrecy, identity obfuscation) and clean modular design. However, **it cannot scale to billions of concurrent users in its current form**. The three critical blockers are: **(1)** the Kademlia DHT routing table is hard-capped at 2,048 nodes per peer (256 buckets × 8 nodes), making billion-node peer discovery require O(33) network hops with high churn-induced failure rates; **(2)** the single-threaded `Iterate()` event loop with 50ms tick intervals creates a throughput ceiling of ~20 iterations/second regardless of hardware; and **(3)** the entirely in-memory state model (`map[uint32]*Friend`, `map[[16]byte]*AsyncMessage`) has no persistence, sharding, or replication strategy, meaning any node failure loses all pending offline messages. Reaching global messaging scale requires fundamental architectural changes to the DHT hierarchy, transport parallelism, and state management—changes that are feasible but represent multi-year engineering efforts.

---

## 2. Scale Target Definition

To "replace phone/text messaging" quantitatively means:

| Metric | Target | Derivation |
|--------|--------|------------|
| **Concurrent users** | 5–8 billion | Global smartphone + feature phone population |
| **Messages/second globally** | 2–5 million | ~200B messages/day (WhatsApp alone handles 100B+), with diurnal peaks 3–5× average |
| **Connection establishment latency** | < 500ms p95 | Users expect sub-second message send confirmation |
| **Offline message delivery** | 99.99% within 24h | Messages must survive sender/receiver being offline simultaneously for hours |
| **DHT lookup latency** | < 2s for peer discovery | Users won't wait longer to start a conversation |
| **NAT traversal success rate** | > 95% | ~62% of mobile users are behind carrier-grade NAT |
| **Group chat scale** | 100K+ members | Institutional/community use cases |

---

## 3. Distributed Systems Analysis

### 3.1 DHT Scalability

**Current design:** Fixed 256 k-buckets with 8 nodes each (`dht/routing.go:72-79`), giving a maximum routing table of 2,048 entries. Lookups use a heap-based `FindClosestNodes` (`dht/routing.go:166-177`) that iterates all 256 buckets on every query.

**Limitation:** In a billion-node network, Kademlia requires O(log₂(N)) ≈ 30 iterative hops to locate any peer. Each hop depends on routing table freshness. With 8 nodes per bucket, the probability that all nodes in a critical bucket are stale rises sharply with churn. The current `pruneDeadNodes` routine (`dht/maintenance.go:336-362`) iterates all 256 buckets linearly every `PingInterval` (1 minute), generating up to 2,048 pings/minute per node. At billion-node scale, the 1-minute ping cycle means stale routing entries persist long enough to cause multi-hop lookup failures.

**The `AddNode` string comparison** (`dht/routing.go:32`) uses `existingNode.ID.String() == node.ID.String()`, which allocates hex-encoded strings on every comparison—an O(n) operation per bucket that generates garbage for the GC. At high churn rates with full buckets, this becomes measurable.

**Required changes:**
- Implement hierarchical/recursive Kademlia with parallel α-lookups (α=3 standard) to reduce hop latency
- Increase bucket size dynamically based on network density estimates
- Replace string-based ID comparison with direct byte comparison
- Add iterative lookup caching with TTL to reduce repeated DHT queries
- Implement S/Kademlia extensions for Sybil resistance (see §3.5)

### 3.2 Transport Layer

**Current design:** Single `net.PacketConn` per node (`transport/udp.go:16`), one goroutine reading into a fixed 2,048-byte buffer (`transport/udp.go:181`), dispatching to per-type handlers via `go handler(packet, addr)` (`transport/udp.go:323`).

**Limitation:** A single UDP socket on Linux can handle ~300K–500K packets/second before kernel buffer overflows cause drops. The 2,048-byte buffer silently discards larger packets (`transport/udp.go:260-266`). The `readPacketData` function sets a 100ms read deadline per iteration (`transport/udp.go:237`), which under heavy load causes unnecessary timeout cycling. Each dispatched handler spawns an unbounded goroutine—under attack or at scale, this creates goroutine explosion risks.

**Noise-IK sessions** are stored per-address in `map[string]*NoiseSession` (`transport/noise_transport.go:69`) with a `SessionIdleTimeout` of 5 minutes (`transport/noise_transport.go:39`). Each handshake requires a Curve25519 DH + symmetric cipher setup. At 1M concurrent connections, the session map alone consumes significant memory, and session establishment latency (~2 RTT for Noise-IK) compounds at scale.

**Required changes:**
- Use `SO_REUSEPORT` with multiple UDP sockets across CPU cores
- Implement a goroutine worker pool with bounded concurrency for packet handlers
- Increase receive buffer size dynamically; use recvmmsg-style batching
- Implement session resumption/0-RTT for Noise to reduce handshake overhead
- Add connection multiplexing for TCP relay mode (per ROADMAP Priority 3)

### 3.3 Message Delivery & Ordering

**Current design:** The `Iterate()` method (`toxcore.go:1084-1102`) is called by the application in a tight loop with `time.Sleep(tox.IterationInterval())`. It sequentially calls `doDHTMaintenance()`, `doFriendConnections()`, `doMessageProcessing()`, and `retryPendingFriendRequests()`. The default `IterationInterval` maps to a 50ms tick.

**Limitation:** Sequential processing means DHT maintenance delays message delivery. At 20 iterations/second, maximum message throughput is bounded by per-iteration processing capacity. There is no prioritization—a burst of DHT pings blocks message delivery. The `doMessageProcessing` function processes the entire message queue in a single tick, creating latency spikes proportional to queue depth.

**Async message retrieval** (`async/manager.go:256-268`) polls every 30 seconds via `messageRetrievalLoop`. Offline messages have a hard 24-hour TTL (`async/storage.go:48`) and a per-recipient cap of 100 messages (`async/storage.go:50`). Storage capacity is 10,000 messages per node (`async/storage.go:52`). At scale, a popular user could easily exceed 100 pending messages in minutes, causing message loss.

**Required changes:**
- Decouple `Iterate()` into separate goroutines for DHT, friend connections, and messaging
- Implement priority queues for message types (real-time messages > DHT maintenance > file transfers)
- Replace polling-based async retrieval with push-based notifications from storage nodes
- Implement erasure-coded redundant storage across multiple storage nodes
- Increase per-recipient limits dynamically based on storage node capacity

### 3.4 State Management

**Current design:** All state is in-memory Go maps protected by `sync.RWMutex`:
- `friends map[uint32]*Friend` (`toxcore.go:315`)
- `fileTransfers map[uint64]*file.Transfer` (`toxcore.go:323`)
- `conferences map[uint32]*group.Chat` (`toxcore.go:328`)
- `messages map[[16]byte]*AsyncMessage` (`async/storage.go:105`)
- `recipientIndex map[[32]byte][]AsyncMessage` (`async/storage.go:106`)
- `sessions map[string]*NoiseSession` (`transport/noise_transport.go:69`)

**Limitation:** No state survives process restart except through explicit `Save()`/`Load()` via JSON serialization (`toxcore.go:202-219`). The `sync.RWMutex` on `friendsMutex` (`toxcore.go:316`) becomes a contention point when thousands of friends are being managed concurrently—every message send acquires a read lock on the friends map. The `recipientIndex` stores full `AsyncMessage` copies (not pointers), causing memory duplication for each stored message.

**Required changes:**
- Implement write-ahead logging for crash recovery of critical state
- Shard friend state by key prefix to reduce mutex contention
- Use pointer-based indexing in `recipientIndex` to eliminate duplication
- Add LRU eviction for Noise sessions and DHT node caches
- Implement state snapshots for faster recovery

### 3.5 Network Topology & Resilience

**Current design:** Bootstrap depends on hard-coded nodes (`dht/bootstrap.go:109`; minimum 4 required). LAN discovery uses IPv4 broadcast to fixed private network ranges (`dht/local_discovery.go:180-184`). No Sybil attack resistance exists in the DHT—any node can claim any ID.

**Limitation:** Bootstrap nodes are single points of failure. If all configured bootstrap nodes become unreachable, new peers cannot join the network. The `maxAttempts` of 5 with exponential backoff capped at 2 minutes (`dht/bootstrap.go:111-112`) means a new node gives up after ~6 minutes. LAN broadcast doesn't scale beyond a single broadcast domain and fails entirely in cloud/container environments.

**The group chat system** uses a global `groupRegistry` (`group/chat.go:260-265`) with an in-process `sync.RWMutex`—groups are not discoverable across processes without DHT, and the DHT group announcement mechanism has no replication guarantee.

**Required changes:**
- Implement a gossip-based bootstrap protocol to eliminate centralized bootstrap dependency
- Add cryptographic proof-of-work or stake for DHT node ID binding (S/Kademlia)
- Replace LAN broadcast with mDNS for local discovery
- Implement network partition detection and automatic re-bootstrapping
- Add DHT replication factor for group announcements (store at k nearest nodes)

---

## 4. Performance Bottlenecks

| Rank | What | Where | Impact | Fix Complexity |
|------|------|-------|--------|---------------|
| 1 | **Single-threaded `Iterate()` event loop** | `toxcore.go:Iterate()` (line 1084) | Limits throughput to ~20 ops/sec; blocks at ~10K active friends | High |
| 2 | **Single UDP socket per node** | `transport/udp.go:NewUDPTransport()` (line 30) | Saturates at ~400K pkt/s; single-core bottleneck | Medium |
| 3 | **Fixed 2,048-node routing table** | `dht/routing.go:NewRoutingTable()` (line 84) | Insufficient for billion-node keyspace coverage | Medium |
| 4 | **String-based node ID comparison** | `dht/routing.go:KBucket.AddNode()` (line 32) | GC pressure from hex allocation on every comparison | Low |
| 5 | **Unbounded goroutine spawning per packet** | `transport/udp.go:dispatchPacketToHandler()` (line 323) | Goroutine explosion under load or attack | Medium |
| 6 | **100-message per-recipient cap** | `async/storage.go:MaxMessagesPerRecipient` (line 50) | Popular users lose messages within minutes at scale | Low |
| 7 | **30-second polling for async messages** | `async/manager.go:messageRetrievalLoop()` (line 257) | Minimum 30s offline delivery latency | Low |
| 8 | **Global `sync.RWMutex` on all friends** | `toxcore.go:friendsMutex` (line 316) | Contention at >1K concurrent friend operations | Medium |
| 9 | **JSON serialization for persistence** | `toxcore.go:toxSaveData.marshal()` (line 205) | O(n) serialization of entire state; not incremental | Medium |
| 10 | **Linear bucket scan in `FindClosestNodes`** | `dht/routing.go:buildNodeHeap()` (line 187) | Scans all 256 buckets even when target bucket is known | Low |

---

## 5. Path to Achievement

*Note: This roadmap extends rather than duplicates `ROADMAP.md`. Existing priorities (UDP proxy, documentation, symmetric NAT relay) remain unchanged and are prerequisites for the phases below.*

### Phase 1: Foundation (0–6 months)

**Goal:** Eliminate single-threaded bottlenecks and prepare for concurrent operation.

| Change | Validation | Dependencies |
|--------|------------|-------------|
| Refactor `Iterate()` into separate goroutine pipelines (DHT, messaging, file transfer) with channel-based coordination | Benchmark: 10× throughput improvement on synthetic friend/message load | None |
| Implement bounded worker pool for UDP packet dispatch (replace `go handler()`) | No goroutine count explosion under 100K pkt/s synthetic load | None |
| Replace string-based ID comparisons with byte slice comparisons across `dht/` | Benchmark: 3× faster `AddNode`/`FindClosestNodes` | None |
| Optimize `FindClosestNodes` to start from target bucket index instead of scanning all 256 | Benchmark: 50% reduction in lookup CPU time | None |
| Complete ROADMAP Priority 1 (UDP proxy) and Priority 3 (symmetric NAT relay) | NAT traversal success rate >90% in test matrix | ROADMAP items |

### Phase 2: Scale Architecture (6–18 months)

**Goal:** Support 1M+ concurrent users per deployment with P2P architecture intact.

| Change | Validation | Dependencies |
|--------|------------|-------------|
| Implement multi-socket UDP transport with `SO_REUSEPORT` | Linear throughput scaling to 4 cores; 1M pkt/s sustained | Phase 1 worker pool |
| Add S/Kademlia extensions (parallel lookups, sibling broadcast, crypto ID binding) | Sybil resistance validated with 30% malicious node simulation | Phase 1 DHT optimizations |
| Implement erasure-coded async message storage across k=5 storage nodes | 99.9% message survival with 2-of-5 node failures | Phase 1 concurrent pipeline |
| Replace polling-based async retrieval with DHT-routed push notifications | Offline delivery latency < 5s when recipient comes online | Phase 2 S/Kademlia |
| Add write-ahead log and incremental state persistence | Zero message loss on clean shutdown; < 100ms recovery | Phase 1 pipeline refactor |
| Implement Noise session resumption (0-RTT PSK mode) | Reconnection latency < 100ms for previously-connected peers | None |

### Phase 3: Global Scale (18–36 months)

**Goal:** Architecture validated for billion-user scale through hierarchical P2P design.

| Change | Validation | Dependencies |
|--------|------------|-------------|
| Implement hierarchical DHT with super-node election (reliability-scored from `PingStats`) | O(log N) lookup with N=1B simulated nodes (via ns-3 or Shadow) | Phase 2 S/Kademlia |
| Add geographic-aware routing using RTT measurements to reduce cross-continent hops | p95 message latency < 500ms in geo-distributed testbed | Phase 2 multi-socket |
| Implement adaptive routing table sizing (dynamic k per bucket based on network density) | Routing table efficiency metric within 2× of theoretical optimum | Phase 2 S/Kademlia |
| Add gossip-based bootstrap protocol eliminating centralized bootstrap dependency | Network recovers from 50% simultaneous node failure in < 5 minutes | Phase 2 erasure coding |
| Implement group chat sharding for 100K+ member groups | Message delivery to all members in < 2s for 100K-member group | Phase 2 push notifications |
| Full privacy network integration (ROADMAP Priorities 4–5) with UDP proxy (Priority 1) | End-to-end Tor connectivity without IP leaks | ROADMAP items |

---

## 6. Comparative Analysis

| Aspect | Signal (centralized) | WhatsApp (centralized) | Telegram (hybrid) | toxcore-go (P2P) |
|--------|---------------------|----------------------|-------------------|-----------------|
| **Peer discovery** | Server lookup O(1) | Server lookup O(1) | Server lookup O(1) | DHT O(log N) hops |
| **Offline messages** | Server-side queue | Server-side queue | Cloud storage | Distributed storage nodes |
| **NAT traversal** | TURN relays | TURN relays | TURN relays | UDP hole-punch + (planned) TCP relay |
| **E2E encryption** | Signal Protocol | Signal Protocol | MTProto (optional) | Noise-IK + forward secrecy |
| **Metadata exposure** | Server sees graph | Server sees graph | Server sees graph | DHT reveals lookup patterns |
| **Group scale** | ~1,000 members | ~1,024 members | 200,000 members | Implementation-limited |

**Adaptable patterns from centralized systems (without sacrificing decentralization):**

1. **Sender-key for groups:** Signal's sender-key protocol (one encrypt, fan-out decrypt) can be adapted for P2P group chat, replacing the current per-peer encryption in `group/chat.go:BroadcastMessage`. This reduces O(n) encryptions to O(1).

2. **Push notification proxying:** WhatsApp's push notification infrastructure could be replicated via a voluntary relay layer—friends' always-online nodes can proxy wake-up signals, similar to how the async storage nodes already operate but with lighter payloads.

3. **Prekey bundles:** Signal's server-hosted prekey bundles map directly to toxcore-go's existing `ForwardSecurityManager` pre-key system (`async/forward_secrecy.go`), but need to be stored in the DHT rather than requiring both parties online simultaneously.

4. **Message ordering via logical clocks:** Telegram's server-assigned message IDs provide total ordering. In P2P, Lamport timestamps or vector clocks attached to each `AsyncMessage` would provide causal ordering without a central authority.

---

## 7. Open Questions

1. **Super-node incentive structure:** Hierarchical DHT requires some nodes to handle disproportionate load. Without economic incentives (unlike Lokinet/blockchain approaches), what prevents free-rider problems? Should the existing "all users are storage nodes" model (`async/manager.go:57`) be formalized with reputation scoring?

2. **DHT privacy vs. performance trade-off:** The identity obfuscation system (`async/obfs.go`) uses pseudonym rotation with epoch-based keys. At billion-user scale, how much additional DHT lookup overhead does pseudonym resolution add, and can it be amortized through friend-of-friend caching?

3. **Optimal replication factor for async messages:** The current single-storage-node model has no redundancy. What replication factor (k=3? k=5?) balances storage overhead against delivery guarantees for the target 99.99% reliability?

4. **TCP relay topology:** The planned TCP relay system (ROADMAP Priority 3) needs a topology decision: should relays be DHT-discovered (adding lookup latency) or maintained as a separate overlay? How does this interact with Tor/I2P transports that already provide relay-like functionality?

5. **Group chat consistency model:** The current broadcast model (`group/chat.go:BroadcastConfig`) provides best-effort delivery. For large groups, should toxcore-go adopt causal consistency (vector clocks), eventual consistency (CRDTs for shared state), or accept total ordering via a designated group leader?

6. **Mobile device constraints:** Battery-constrained devices cannot maintain DHT participation. Should the protocol define a "light client" mode that delegates DHT operations to a trusted always-online companion node, similar to Bitcoin SPV wallets?

7. **Handshake amplification attack surface:** The Noise-IK handshake in `transport/noise_transport.go` requires server-side DH computation on receipt of the first message. At scale, this enables CPU-exhaustion attacks. Should a cookie-based DoS mitigation (like DTLS) precede the Noise handshake?
