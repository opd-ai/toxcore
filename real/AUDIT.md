# Audit: github.com/opd-ai/toxcore/real
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The `real` package provides production network-based packet delivery implementation as a wrapper around the transport layer. Implementation is functionally complete with proper error handling and concurrency safety, but lacks test coverage (0%), package-level documentation, and contains non-deterministic sleep behavior that may affect testing reproducibility.

## Issues Found
- [ ] high **Test coverage** — Zero test coverage (0.0%, target: 65%) for production-critical code (`packet_delivery.go`)
- [ ] high **Test coverage** — No test file exists; missing unit tests for all methods including DeliverPacket, BroadcastPacket, retry logic, and concurrent access
- [ ] high **Deterministic procgen** — Non-deterministic sleep using `time.Sleep` with variable duration in retry loop (`packet_delivery.go:90`)
- [ ] med **Doc coverage** — Missing package-level documentation (no `doc.go` file)
- [ ] med **Integration points** — Type assertions violate interface abstraction in toxcore.go; casting to concrete `*real.RealPacketDelivery` type (`toxcore.go:3606,3640,3664`)
- [ ] med **Error handling** — Silent error in SetNetworkTransport when closing old transport; logs warning but doesn't propagate error to caller (`packet_delivery.go:174-179`)
- [ ] low **Doc coverage** — GetStats method lacks godoc comment explaining return value fields (`packet_delivery.go:255`)
- [ ] low **Doc coverage** — AddFriend method lacks documentation of when transport registration might fail (`packet_delivery.go:199`)
- [ ] low **Doc coverage** — RemoveFriend method lacks documentation of behavior when friend doesn't exist (`packet_delivery.go:234`)
- [ ] low **Network interfaces** — Code correctly uses interface types (net.Addr) throughout, but integration layer violates this by using type assertions to concrete *RealPacketDelivery

## Test Coverage
0.0% (target: 65%)

**Critical gaps:**
- No tests for retry logic with exponential backoff (lines 69-92)
- No tests for concurrent access to friendAddrs map under mutex
- No tests for broadcast with exclusions (lines 105-161)
- No tests for transport swapping via SetNetworkTransport (lines 164-191)
- No tests for stats collection (lines 255-267)
- No tests for friend registration/deregistration (lines 199-252)

## Integration Status
The real package properly implements `interfaces.IPacketDelivery` and is instantiated via `factory.PacketDeliveryFactory`. Integration is functional but violates abstraction:

**Integration points:**
- ✅ Registered in `factory/packet_delivery_factory.go:139` as production implementation
- ✅ Used by `toxcore.go` main Tox instance for real network operations
- ❌ Type assertions to `*real.RealPacketDelivery` in toxcore.go break interface abstraction (lines 3606, 3640, 3664)
- ✅ Proper interface-based transport usage (net.Addr throughout)

**Missing:**
- Interface should expose AddFriend/RemoveFriend/GetStats to avoid type assertions
- Consider extending IPacketDelivery interface with these methods for proper abstraction

## Recommendations
1. **HIGH PRIORITY**: Add comprehensive test suite with table-driven tests covering retry logic, concurrent access, broadcast scenarios, and error conditions (target 70%+ coverage)
2. **HIGH PRIORITY**: Replace non-deterministic `time.Sleep(time.Duration(500*(attempt+1)) * time.Millisecond)` at line 90 with injectable time/clock interface for testability
3. **MEDIUM PRIORITY**: Add `doc.go` with package-level documentation explaining real vs. simulation implementations
4. **MEDIUM PRIORITY**: Extend `interfaces.IPacketDelivery` interface to include AddFriend/RemoveFriend/GetStats methods to eliminate type assertions in toxcore.go
5. **LOW PRIORITY**: Add godoc comments for GetStats return value structure, AddFriend failure conditions, and RemoveFriend edge cases
6. **LOW PRIORITY**: Consider propagating Close() error from SetNetworkTransport instead of silent logging (line 174-179)
