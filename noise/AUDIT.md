# Audit: github.com/opd-ai/toxcore/noise
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The noise package implements the Noise Protocol Framework (IK and XX patterns) for secure handshakes with mutual authentication and forward secrecy. Overall implementation is solid with comprehensive test coverage (89.0%), but contains medium-severity non-determinism issues with time.Now() usage and several low-severity documentation and consistency concerns.

## Issues Found
- [x] ~~**high** GetLocalStaticKey bug~~ ✅ FIXED — IKHandshake.GetLocalStaticKey() now returns static public key stored in localPubKey field (`handshake.go:247-255`)
- [x] **med** Non-deterministic timestamp — Uses time.Now().Unix() for handshake timestamp, preventing deterministic replay testing (`handshake.go:105`)
- [x] **med** Non-deterministic nonce — Uses crypto/rand.Read for nonce generation; acceptable for security but prevents deterministic testing (`handshake.go:109`)
- [x] **low** Missing doc.go — Package lacks doc.go with overview of IK vs XX pattern selection guidance
- [x] ~~**low** Inconsistent static key storage~~ ✅ FIXED — IKHandshake now includes localPubKey field like XXHandshake (`handshake.go:46`)
- [x] **low** Unused timestamp validation — GetTimestamp() returns timestamp but no validation helper provided for HandshakeMaxAge checks (`handshake.go:262-264`)
- [x] **low** Missing nonce replay validation — GetNonce() returns nonce but no IsNonceUsed() validation helper for replay protection (`handshake.go:256-259`)

## Test Coverage
89.0% (target: 65%) ✅

Test suite includes:
- Comprehensive unit tests with table-driven patterns (handshake_test.go)
- Extensive coverage tests for edge cases and error paths (handshake_coverage_test.go)  
- Fuzz testing for malformed message handling (handshake_fuzz_test.go)
- Benchmarks for handshake creation and complete flows

Coverage exceeds target significantly, demonstrating thorough validation.

## Integration Status
**Strong integration with transport layer:**
- Used by `transport/NoiseTransport` for encryption wrapper (`noise_transport.go:47`)
- Used by `transport/VersionedHandshakeManager` for protocol negotiation (`versioned_handshake.go`)
- Properly aliased as `toxnoise` to avoid conflict with flynn/noise library

**Security registrations:**
- Replay protection via nonce tracking implemented in transport layer
- Timestamp freshness validation (HandshakeMaxAge) in transport layer  
- Session lifecycle management in NoiseTransport with cleanup goroutines

**Missing integration points:**
- No centralized handshake pattern registry (IK/XX selection is ad-hoc)
- No metrics/telemetry for handshake success/failure rates
- No configuration for handshake timeouts (hardcoded in transport)

## Recommendations
1. ~~**CRITICAL**: Fix GetLocalStaticKey bug~~ ✅ FIXED — localPubKey field added to IKHandshake and populated during NewIKHandshake; GetLocalStaticKey now returns localPubKey copy
2. Add doc.go with package-level documentation explaining IK vs XX pattern selection criteria, security properties, and integration examples
3. Add optional TimeProvider interface parameter to NewIKHandshake for deterministic testing (default to time.Now)
4. Add optional NonceProvider interface parameter for deterministic testing (default to crypto/rand.Read)
5. Add validation helpers: ValidateTimestamp(timestamp int64, maxAge time.Duration) error and ValidateNonce(nonce [32]byte, usedNonces map[[32]byte]bool) error
6. Consider adding handshake pattern registry/factory for consistent pattern selection across codebase
7. Add integration test demonstrating full IK handshake flow with matching keypairs (current tests use mismatched keys causing errors)
