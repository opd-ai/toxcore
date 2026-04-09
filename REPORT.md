# REPORT.md — toxcore-go Distributed Systems Scalability Analysis

> **Generated:** 2026-03-19 | **Scope:** 835 functions, 24 packages, 31,453 LOC  
> **Repository:** [opd-ai/toxcore](https://github.com/opd-ai/toxcore) @ `923e45c`

---

## 1. Executive Summary

toxcore-go is a well-architected pure Go Tox P2P implementation with strong cryptographic foundations. However, **it cannot scale to billions of users** due to three critical blockers: **(1)** DHT routing table hard-capped at 2,048 nodes (256 buckets × 8), requiring O(33) hops at billion-node scale; **(2)** single-threaded `Iterate()` event loop with 50ms ticks (~20 ops/sec ceiling); **(3)** entirely in-memory state with no persistence, sharding, or replication.

---

## 2. Scale Targets

| Metric | Target |
|--------|--------|
| Concurrent users | 5–8 billion |
| Messages/second | 2–5 million |
| Connection latency | < 500ms p95 |
| Offline delivery | 99.99% within 24h |
| DHT lookup | < 2s |
| NAT traversal | > 95% success |
| Group chat | 100K+ members |

---

## 3. Key Limitations & Required Changes

### 3.1 DHT Scalability
Fixed 256 k-buckets × 8 nodes = 2,048 max entries. `FindClosestNodes` scans all 256 buckets. `pruneDeadNodes` generates up to 2,048 pings/minute. **Fix:** Hierarchical/recursive Kademlia with parallel α-lookups, dynamic bucket sizing, S/Kademlia Sybil resistance.

### 3.2 Transport Layer
Single UDP socket saturates at ~400K pkt/s. Unbounded goroutine spawning per packet. Noise-IK session map grows linearly with connections. **Fix:** `SO_REUSEPORT` multi-socket, bounded worker pool, session resumption/0-RTT.

### 3.3 Message Delivery
Sequential `Iterate()` processes DHT maintenance, friend connections, and messaging in series. Async polling every 30s with 100-message per-recipient cap. **Fix:** Decouple into goroutine pipelines, priority queues, push-based notifications, erasure-coded redundant storage.

### 3.4 State Management
All state in-memory (`sync.RWMutex`-protected maps). No state survives restart except explicit `Save()`/`Load()`. `recipientIndex` stores full message copies (not pointers). **Fix:** WAL for crash recovery, sharded friend state, pointer-based indexing, LRU eviction.

### 3.5 Network Topology
Bootstrap depends on hard-coded nodes (SPOF). LAN discovery via IPv4 broadcast only. No Sybil resistance in DHT. **Fix:** Gossip-based bootstrap, mDNS for LAN, cryptographic proof-of-work for node IDs, DHT replication for group announcements.

---

## 4. Performance Bottlenecks

| Rank | Issue | Location | Fix Complexity |
|------|-------|----------|---------------|
| 1 | Single-threaded Iterate() | toxcore.go | High |
| 2 | Single UDP socket | transport/udp.go | Medium |
| 3 | Fixed 2,048-node routing table | dht/routing.go | Medium |
| 4 | Unbounded goroutine spawning | transport/udp.go | Medium |
| 5 | 100-message per-recipient cap | async/storage.go | Low |
| 6 | 30-second async polling | async/manager.go | Low |
| 7 | Global friend mutex | toxcore.go | Medium |
| 8 | JSON serialization for persistence | toxcore.go | Medium |

---

## 5. Path to Achievement

### Phase 1: Foundation (0–6 months)
Refactor `Iterate()` into goroutine pipelines. Bounded worker pool for UDP dispatch. Optimize `FindClosestNodes` to start from target bucket. Complete ROADMAP Priority 1 (UDP proxy) and 3 (NAT relay).

### Phase 2: Scale Architecture (6–18 months)
Multi-socket UDP with `SO_REUSEPORT`. S/Kademlia extensions. Erasure-coded async storage (k=5). Push-based async delivery. WAL persistence. Noise session resumption.

### Phase 3: Global Scale (18–36 months)
Hierarchical DHT with super-node election. Geographic-aware routing. Adaptive routing table sizing. Gossip bootstrap. Group chat sharding for 100K+ members.

---

## 6. Comparative Analysis

| Aspect | Signal/WhatsApp | toxcore-go |
|--------|----------------|-----------|
| Peer discovery | Server O(1) | DHT O(log N) |
| Offline messages | Server queue | Distributed storage |
| E2E encryption | Signal Protocol | Noise-IK + forward secrecy |
| Metadata exposure | Server sees graph | DHT reveals lookup patterns |

**Adaptable patterns:** Sender-key for groups (O(1) encrypt), push notification proxying via relay nodes, DHT-stored prekey bundles, Lamport/vector clocks for message ordering.

---

## 7. Open Questions

1. Super-node incentive structure for hierarchical DHT
2. DHT privacy vs performance trade-off with pseudonym rotation at scale
3. Optimal replication factor for async messages (k=3? k=5?)
4. TCP relay topology: DHT-discovered vs separate overlay
5. Group chat consistency model: causal (vector clocks), eventual (CRDTs), or leader-based
6. Light client mode for battery-constrained mobile devices
7. Cookie-based DoS mitigation before Noise-IK handshake

