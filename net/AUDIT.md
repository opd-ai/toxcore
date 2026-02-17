# Audit: github.com/opd-ai/toxcore/net
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The `net` package provides Go standard library networking interfaces (net.Conn, net.Listener, net.Addr) for Tox protocol communication. Overall architecture is sound with proper interface abstractions, but contains 3 high-severity issues: stub implementation in PacketListen, missing Tox packet encryption in ToxPacketConn.WriteTo, and callback collision risks. Test coverage at 43.5% falls significantly below the 65% target. The package correctly follows toxcore-go networking standards by using interface types (net.Addr, net.PacketConn, net.Conn) throughout.

## Issues Found
- [x] **high** Stub/incomplete code — `PacketListen` creates invalid ToxAddr with nil toxID, making packet listener unusable (`dial.go:189-190`)
- [x] **high** Stub/incomplete code — `ToxPacketConn.WriteTo` writes directly to UDP without Tox packet formatting/encryption; comment acknowledges this is incomplete (`packet_conn.go:264-266`)
- [x] **high** Error handling — `ToxConn.setupCallbacks` overwrites global Tox callbacks, causing collision if multiple connections exist; callbacks should be per-connection or use multiplexing (`conn.go:82-107`)
- [x] **med** Deterministic procgen — Uses `time.Now()` for deadline checks which is acceptable for I/O timing, but creates non-deterministic behavior in tests (`conn.go:255,291`, `packet_conn.go:99,256`, `packet_listener.go:124,395`)
- [x] **med** Error handling — `ToxPacketConn.Close` logs but returns error from underlying UDP close, should wrap error with ToxNetError for consistency (`packet_conn.go:299-312`)
- [x] **low** Test coverage — Package coverage at 43.5%, below 65% target; missing tests for PacketListen, PacketDial, error paths in packet handling
- [x] **low** Test coverage — No table-driven tests for ToxAddr validation, Equal method, or IsToxAddr function
- [x] **low** Test coverage — Missing benchmark tests for throughput-critical operations (Read/Write, packet processing)
- [x] **low** Doc coverage — README.md and PACKET_NETWORKING.md exist but package-level doc.go could include more usage examples for packet-based APIs
- [x] **low** Error handling — `ToxPacketConn.processIncomingPacket` sets read deadline in hot loop, could be optimized (`packet_conn.go:99`)
- [x] **low** Integration points — No clear serialization support for ToxAddr; other packages may need to persist Tox addresses

## Test Coverage
43.5% (target: 65%)

**Coverage gaps:**
- PacketDial and PacketListen functions (0% coverage)
- ToxPacketConnection Read/Write methods (estimated <30%)
- Error paths in packet buffer overflow scenarios
- Deadline/timeout edge cases across all connection types
- ToxConn callback handling and state machine transitions

## Integration Status
The `net` package serves as a foundational networking abstraction layer for toxcore-go. It correctly implements Go's standard `net.Conn`, `net.Listener`, `net.Addr`, and `net.PacketConn` interfaces, enabling seamless integration with existing Go networking code.

**Integration points:**
- **Transport layer**: Not yet imported by `transport/` package (expected integration point)
- **DHT layer**: Not yet imported by `dht/` package (expected integration point)
- **Direct Tox API usage**: Wraps `toxcore.Tox` for friend messaging and status callbacks
- **Crypto integration**: Depends on `crypto.ToxID` for address handling
- **Interface compliance**: Fully implements net.Conn, net.Listener, net.Addr, net.PacketConn per toxcore-go standards

**Missing registrations:**
- No system initialization needed (pure library package)
- ToxAddr does not implement Gob/JSON encoding for persistence

**Network interface compliance:**
✅ Uses `net.Addr` interface (not concrete net.UDPAddr/net.TCPAddr)
✅ Uses `net.PacketConn` interface (not concrete net.UDPConn)
✅ Uses `net.Conn` interface (not concrete net.TCPConn)
✅ Uses `net.Listener` interface (not concrete net.TCPListener/net.UDPListener)
✅ No type assertions to concrete network types in production code (test code uses net.UDPAddr appropriately)

## Recommendations
1. **[HIGH PRIORITY]** Fix `PacketListen` stub: Require `*toxcore.Tox` parameter to create valid ToxAddr, matching API pattern of `Listen` function
2. **[HIGH PRIORITY]** Implement Tox packet formatting/encryption in `ToxPacketConn.WriteTo` or document limitations and intended use case
3. **[HIGH PRIORITY]** Fix callback collision: Implement callback multiplexing in toxcore.Tox or make ToxConn use connection-specific message routing
4. **[MEDIUM PRIORITY]** Increase test coverage to 65%: Add table-driven tests for ToxAddr, integration tests for packet-based networking
5. **[MEDIUM PRIORITY]** Wrap all errors with ToxNetError consistently for better error handling downstream
6. **[LOW PRIORITY]** Add serialization support (JSON/Gob) for ToxAddr to enable address persistence
7. **[LOW PRIORITY]** Optimize deadline management in packet processing hot loops
8. **[LOW PRIORITY]** Add benchmark tests for Read/Write throughput and packet processing performance
