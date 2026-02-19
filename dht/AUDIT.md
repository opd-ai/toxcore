# Audit: github.com/opd-ai/toxcore/dht
**Date**: 2026-02-19
**Status**: Complete

## Summary
The DHT package implements a modified Kademlia distributed hash table for peer discovery and routing in the Tox network. Overall health is **good** with strong concurrency patterns, comprehensive testing (68.6% coverage), and well-designed abstractions. The package has 10 source files (~3,700 LOC production code) with proper thread safety via mutexes, deterministic time providers for testing, and multi-network address support. Critical risks are minimal, with most issues being low-priority documentation or minor improvements.

## Issues Found
- [ ] **low** Documentation — Missing godoc for private helper function `lessDistance` (`routing.go:246`)
- [ ] **low** Error Handling — Ignored transport.Send errors in best-effort operations; consider logging (`group_storage.go:170,221`, `maintenance.go:233,257,331`)
- [ ] **low** Determinism — Direct `time.Since()` calls in test code could use mock time providers for full determinism (`group_storage.go:62,75`)
- [ ] **med** API Design — `RemoveNode` method has inconsistent receiver (on KBucket but used on RoutingTable context) (`routing.go:261`)
- [ ] **low** Concurrency Safety — Double RLock in `collectInactiveNodes` unnecessary (bucket already locked in GetNodes call) (`maintenance.go:206-208`)
- [ ] **low** Documentation — Export comment for `RemoveNode` uses lowercase prefix against Go conventions (`routing.go:260`)
- [ ] **med** Error Handling — `QueryGroup` returns generic error message instead of using named error types for better handling (`group_storage.go:230`)

## Test Coverage
68.6% (target: 65%) ✓ **PASS**

**Coverage by component:**
- routing.go: Well covered with heap-based FindClosest optimization tests
- bootstrap.go: Comprehensive versioned handshake integration tests
- maintenance.go: Lifecycle and concurrent ping routine tests
- handler.go: Multi-network packet handling with version negotiation
- group_storage.go: Serialization and expiration cleanup tests
- local_discovery.go: Broadcast/receive integration with interface-based addressing
- address_detection.go: Multi-network type detection coverage
- node.go: Strong TimeProvider abstraction for deterministic testing

## Dependencies

**External dependencies:**
- `github.com/sirupsen/logrus` — Structured logging (justified for debugging DHT operations)
- `github.com/opd-ai/toxcore/crypto` — Internal cryptographic operations (ToxID, public keys)
- `github.com/opd-ai/toxcore/transport` — Internal transport layer abstractions

**Standard library:**
- `container/heap` — Efficient k-closest node finding in routing table
- `crypto/rand` — Secure random for node lookup operations
- `math/rand/v2` — Non-cryptographic random with fallback pattern
- `encoding/binary`, `encoding/hex`, `encoding/json` — Serialization
- `context` — Cancellation and timeouts for bootstrap operations
- `net` — Network abstractions using interface types (no concrete type assertions)
- `sync` — Thread-safe concurrent access patterns

**Integration points:**
- RoutingTable ↔ BootstrapManager: Bidirectional node discovery
- Maintainer → RoutingTable: Periodic cleanup and health checks
- GroupStorage ↔ RoutingTable: DHT-based group announcement distribution
- AddressTypeDetector → Transport: Multi-network address validation
- TimeProvider abstraction: Deterministic testing support across all components

**Circular dependency analysis:** None detected. Clean layered architecture with dht → transport → net.

## Recommendations
1. **HIGH**: Refactor `RemoveNode` to be a method on `RoutingTable` with proper bucket selection logic, or document why it's on `KBucket`
2. **MED**: Create named error types (`ErrGroupNotFound`, `ErrQueryTimeout`) for `QueryGroup` to enable proper error handling by callers
3. **MED**: Add structured logging (with rate limiting) for ignored best-effort `Send` errors to aid debugging
4. **LOW**: Add godoc comment for `lessDistance` helper function explaining byte-wise comparison
5. **LOW**: Remove redundant `bucket.mu.RLock()` in `collectInactiveNodes` since `GetNodes()` already acquires read lock
6. **LOW**: Update export comment for `RemoveNode` to start with function name per Go conventions
7. **LOW**: Consider adding `CleanExpired()` to periodic maintenance routine (currently manual cleanup only)
