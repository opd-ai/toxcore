# Audit: github.com/opd-ai/toxcore/net
**Date**: 2026-02-18
**Status**: Needs Work

## Summary
The net package implements Go standard library networking interfaces (net.Conn, net.Listener, net.Addr, net.PacketConn) for Tox protocol communication, enabling seamless integration with existing Go networking code. The package consists of 10 source files (~1,900 LOC) with ToxAddr, ToxConn, ToxListener, and packet-based networking implementations. Test coverage is critically low at 43.5% (target: 65%), with one failing test. Multiple high-severity issues remain including non-deterministic time usage, stub implementations blocking real usage, and a broken timeout mechanism causing test failures.

## Issues Found
- [ ] high test-coverage — Test coverage at 43.5%, significantly below 65% target; needs 21.5% improvement (`go test -cover ./net`)
- [ ] high test-failure — TestDialTimeout fails consistently, taking 5 seconds instead of expected 10-200ms timeout; timeout mechanism appears broken (`conn_test.go:33-43`)
- [ ] high determinism — Non-deterministic `time.Now()` usage in deadline checks affects testability (`conn.go:255`, `conn.go:291`, `packet_conn.go:99`, `packet_conn.go:256`, `packet_listener.go:124`, `packet_listener.go:395`)
- [ ] high stub — `PacketListen` function creates invalid ToxAddr with nil toxID, making it completely unusable for real applications (`dial.go:189-190`)
- [ ] high stub — `ToxPacketConn.WriteTo` writes directly to UDP without Tox packet formatting or encryption; comment explicitly states this is incomplete implementation (`packet_conn.go:264-266`)
- [ ] high integration — `ToxConn.setupCallbacks` overwrites global Tox callbacks on every connection, causing severe collision bugs when multiple connections exist; all connections would receive each other's messages (`conn.go:82-107`)
- [ ] med error-handling — `ToxPacketConn.Close()` returns unwrapped UDP close error instead of ToxNetError, breaking error handling consistency (`packet_conn.go:299-312`)
- [ ] med determinism — `waitForConnection` in dial.go uses `time.NewTicker` with hardcoded 100ms interval instead of injectable time source (`dial.go:85`)
- [ ] med determinism — `ToxListener.waitAndCreateConnection` uses hardcoded 30-second and 100ms timeouts with no injection mechanism (`listener.go:109-110`)
- [ ] med test-coverage — No table-driven tests for ToxAddr validation functions (IsToxAddr, Equal, ParseToxAddr) despite multiple validation code paths
- [ ] med test-coverage — PacketDial and PacketListen functions have 0% test coverage; these are exported API functions
- [ ] med test-coverage — ToxPacketConnection Read/Write methods have minimal coverage despite being core functionality
- [ ] low test-coverage — No benchmark tests for performance-critical operations (Read, Write, packet processing loops)
- [ ] low integration — ToxAddr lacks JSON/Gob serialization methods for persistence in savedata or configuration files
- [ ] low doc-coverage — Package doc.go lacks examples for packet-based networking APIs (ToxPacketConn, ToxPacketListener)
- [ ] low performance — `ToxPacketConn.processIncomingPacket` calls `SetReadDeadline` in hot loop on every packet; could cache deadline calculation (`packet_conn.go:99`)

## Test Coverage
43.5% (target: 65%)

**Major coverage gaps:**
- PacketDial: 0% (lines 142-172 in dial.go)
- PacketListen: 0% (lines 178-204 in dial.go)
- ToxPacketConnection: ~30% estimated (Read/Write methods barely tested)
- Error paths for buffer overflows: untested
- Deadline/timeout edge cases: partial coverage
- ToxConn state machine transitions: incomplete
- Callback multiplexing scenarios: not tested

**Existing test functions (13 total):**
- TestDialTimeout (FAILING)
- TestDialInvalidToxID
- TestToxConnInterface
- TestListenerInterface
- Tests in net_test.go (addr validation)
- Tests in packet_test.go (partial packet conn coverage)

## Integration Status
The net package provides foundational networking abstractions but has limited integration with other toxcore packages.

**Current Integration:**
- ✅ Depends on `crypto.ToxID` for address parsing and validation
- ✅ Wraps `toxcore.Tox` for friend messaging callbacks
- ✅ Correctly uses `net.Addr`, `net.Conn`, `net.PacketConn`, `net.Listener` interfaces (no concrete types)
- ✅ Proper error wrapping with ToxNetError (mostly consistent)

**Integration Gaps:**
- ❌ Not imported by `transport/` package (expected integration point for packet transport)
- ❌ Not imported by `dht/` package (expected for DHT packet routing)
- ❌ No system registration in root toxcore.go
- ❌ ToxAddr lacks serialization for persistence in savedata format
- ❌ Callback collision issue prevents multiple ToxConn instances from working correctly

**Network Interface Compliance:** ✅ EXCELLENT
- Uses net.Addr interface exclusively (no concrete net.UDPAddr/net.TCPAddr in production code)
- Uses net.PacketConn interface (no concrete net.UDPConn)
- Uses net.Conn interface (no concrete net.TCPConn)
- Uses net.Listener interface (no concrete net.TCPListener)
- Test code correctly uses concrete types (net.UDPAddr) for mock data, which is acceptable

## Recommendations
1. **[CRITICAL]** Fix failing TestDialTimeout — Investigate why DialTimeout ignores provided timeout and waits 5+ seconds; likely issue in `waitForConnection` function not respecting context timeout (`dial.go:83-100`)
2. **[CRITICAL]** Fix callback collision bug — Implement per-connection message routing or callback multiplexing in ToxConn to prevent message cross-contamination between connections (`conn.go:82-107`)
3. **[HIGH]** Implement TimeProvider interface — Replace all `time.Now()` calls with injectable time source for deterministic testing, following patterns from dht/transport packages
4. **[HIGH]** Complete PacketListen implementation — Require `*toxcore.Tox` parameter to create valid ToxAddr, or remove function if packet-based listening is not yet supported
5. **[HIGH]** Complete ToxPacketConn.WriteTo — Implement Tox packet formatting/encryption or clearly document this as a placeholder API not ready for production use
6. **[HIGH]** Increase test coverage to 65%+ — Add table-driven tests for ToxAddr, integration tests for PacketDial/PacketListen, error path tests
7. **[MEDIUM]** Add ToxAddr serialization — Implement MarshalJSON/UnmarshalJSON and GobEncode/GobDecode for address persistence
8. **[MEDIUM]** Wrap all errors consistently — Ensure ToxPacketConn.Close and other methods return ToxNetError instead of raw errors
9. **[LOW]** Add benchmark tests — Create benchmarks for Read/Write throughput and packet processing to catch performance regressions
10. **[LOW]** Optimize deadline management — Cache deadline calculations in packet processing hot loops to reduce overhead
