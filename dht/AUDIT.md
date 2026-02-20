# Audit: github.com/opd-ai/toxcore/dht
**Date**: 2026-02-20
**Status**: Needs Work

## Summary
The DHT package is a foundational component implementing peer discovery and routing via Kademlia. Overall structure is solid with 69.1% test coverage, good documentation (doc.go with 157 lines), and thread-safe designs. However, the package suffers from complexity creep with oversized handler.go (874 lines) and some incomplete error handling patterns.

## Issues Found
- [x] high API Design — handler.go is oversized at 874 lines, violates single responsibility (handles packets, version negotiation, group queries, address detection) (`handler.go:1`)
- [x] high Error Handling — BootstrapManager methods send packets via `_ = tr.Send(packet, addr)` silently ignoring transmission failures in critical paths (`maintenance.go:233`, `maintenance.go:257`, `maintenance.go:331`, `group_storage.go:170`, `group_storage.go:253`)
- [x] med Concurrency Safety — LANDiscovery.receiveLoop accesses `ld.conn` in tight loop; connection could be closed mid-read despite mu.RLock check (`local_discovery.go:237`)
- [x] med API Design — Mixed use of TimeProvider injection (good) but some components still use `time.Now()` directly instead of `getTimeProvider()` (`routing.go:199-201` reads bucket nodes without time provider)
- [x] med Documentation — Missing godoc comments on exported functions: `SerializeAnnouncement`, `DeserializeAnnouncement`, `LANDiscoveryPacketData`, `ParseLANDiscoveryPacket` (`group_storage.go:100-103`, `local_discovery.go:320-337`)
- [x] low API Design — BootstrapManager has 3 constructors (New, NewWithKeyPair, NewForTesting) with significant duplication; factory pattern could reduce redundancy (`bootstrap.go:85-204`)
- [x] low Error Handling — QueryGroup returns error "DHT query sent, response handling not yet implemented" indicating incomplete async operation handling (`group_storage.go:220`)

## Test Coverage
69.1% (target: 65%) ✓ PASS

## Dependencies
**External:**
- `github.com/sirupsen/logrus` — Structured logging (heavy use throughout)

**Internal:**
- `github.com/opd-ai/toxcore/crypto` — ToxID, KeyPair, cryptographic operations
- `github.com/opd-ai/toxcore/transport` — Transport interface, packet types, address types, protocol version negotiation
- `container/heap` — Efficient node distance calculations in routing
- `net`, `context`, `sync`, `time` — Standard library networking and concurrency

**Integration Surface:**
17 direct imports, 101 total dependencies, indicating high connectivity within codebase.

## Recommendations
1. **Refactor handler.go** — Split into handler_packets.go (25-40), handler_versioning.go (69-248), handler_groups.go (259-350), handler_validation.go (helpers) to improve maintainability and single responsibility adherence
2. **Fix error handling** — Replace `_ = tr.Send(...)` with proper error handling or logging; at minimum log transmission failures in maintenance and group storage paths
3. **Complete async operations** — Implement response handling for QueryGroup DHT queries or mark as experimental/WIP in API docs
4. **Add missing godoc** — Document all exported functions, especially serialization utilities that are part of public API
5. **Consolidate constructors** — Use functional options pattern for BootstrapManager to reduce duplication while maintaining backward compatibility
