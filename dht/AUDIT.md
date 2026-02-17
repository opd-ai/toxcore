# Audit: github.com/opd-ai/toxcore/dht
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The DHT package implements peer discovery and routing for the Tox protocol with comprehensive multi-network support (.onion, .i2p, .nym, .loki). The codebase demonstrates good architectural design with 66.5% test coverage and strong integration capabilities. However, several medium-severity issues exist around non-deterministic time usage, incomplete version negotiation implementation, and missing package documentation that should be addressed.

## Issues Found
- [ ] med: Non-deterministic time usage — Multiple uses of `time.Now()` throughout package for timestamps instead of injectable clock (`node.go:60,88,121,132`, `bootstrap.go:503`, `maintenance.go:352`, `local_discovery.go:219`, `handler.go:494`)
- [ ] low: Missing package doc.go — No package-level documentation file for godoc (`dht/` directory)
- [ ] low: Incomplete implementation — Version negotiation protocol parsing marked as TODO (`handler.go:92`)
- [ ] low: Deprecated function — `parseAddressFromPacket()` marked deprecated with architectural concerns but still in use (`handler.go:420`)
- [ ] low: Type assertions present — Uses concrete net types with type switches in address detection (`address_detection.go:72-80`)

## Test Coverage
66.5% (target: 65%) ✓

Coverage breakdown:
- Core functionality: Well tested with integration tests
- Bootstrap process: Comprehensive test coverage including versioned handshakes
- Local discovery: Interface-based testing implemented
- Group storage: Integration tests present
- Maintenance: Basic coverage present

## Integration Status
The DHT package integrates well with the toxcore ecosystem:
- **Transport Layer**: Properly uses `transport.Transport` interface for network abstraction
- **Crypto Layer**: Correctly integrates with `crypto.ToxID` and key management
- **Group Discovery**: Implements cross-process group announcement storage and querying
- **Multi-Network Support**: Comprehensive address type detection for alternative networks
- **Version Negotiation**: Framework present for protocol versioning (partial implementation)

**Registrations Present**:
- Packet handlers registered for DHT-specific packet types
- Group packet handlers properly initialized in bootstrap manager
- Transport parser selector integrated for multi-network parsing

**Missing Registrations**: None identified - all integration points appear complete

## Recommendations
1. **High Priority**: Inject time provider for deterministic testing — Replace all `time.Now()` calls with injectable clock interface for testability and determinism (affects 5+ files)
2. **Medium Priority**: Complete version negotiation implementation — Remove TODO at `handler.go:92` by implementing full protocol version negotiation handshake parsing
3. **Medium Priority**: Remove or complete deprecated function migration — Either complete removal of `parseAddressFromPacket()` or finish migration to `parseNodeEntry()` throughout codebase
4. **Low Priority**: Add package doc.go — Create comprehensive package documentation with usage examples and architecture overview
5. **Low Priority**: Eliminate type assertions — Replace type switches in `address_detection.go:72-80` with interface-based detection to fully comply with network abstraction guidelines
