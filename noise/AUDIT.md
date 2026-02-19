# Audit: github.com/opd-ai/toxcore/noise
**Date**: 2026-02-19
**Status**: Complete

## Summary
The noise package implements Noise Protocol Framework handshakes (IK and XX patterns) for secure cryptographic communication with formally verified flynn/noise library. Code quality is excellent with 88.4% test coverage, comprehensive documentation, and proper security practices. Five low-severity issues identified related to API consistency and determinism.

## Issues Found
- [ ] low API Design — XXHandshake missing GetNonce() and GetTimestamp() methods for consistency with IKHandshake API surface (`handshake.go:269`)
- [ ] low API Design — XXHandshake.GetRemoteStaticKey() returns raw PeerStatic() without defensive copy, inconsistent with IKHandshake implementation (`handshake.go:390`)
- [ ] low Determinism — Direct time.Now().Unix() usage in NewIKHandshake prevents deterministic testing and reproducible builds (`handshake.go:106`)
- [ ] low Documentation — doc.go references HandshakeMaxAge and HandshakeMaxFutureDrift constants that are defined in transport package, not noise package (`doc.go:106-107`)
- [ ] low Test Coverage — All error ignoring (`_ = err`) in test files is intentional for coverage exercises, but test documentation could clarify intent (`handshake_coverage_test.go:558-792`)

## Test Coverage
88.4% (target: 65%) ✓

**Test Files:**
- `handshake_test.go` (484 lines): Core functionality tests with table-driven patterns
- `handshake_coverage_test.go` (818 lines): Edge case and error path coverage
- `handshake_fuzz_test.go` (143 lines): Fuzz testing for security validation

**Race Detection:** PASS - No race conditions detected

## Dependencies
**External:**
- `github.com/flynn/noise v1.1.0` - Formally verified Noise Protocol implementation (justified: cryptographic correctness)
- `github.com/opd-ai/toxcore/crypto` - Secure memory handling and key derivation

**Standard Library:**
- `crypto/rand` - Cryptographically secure random number generation
- `time` - Timestamp generation (issue #3 above)
- `errors`, `fmt` - Error handling

**Import Surface:** 0 packages import noise directly (used via transport layer abstraction)

## Recommendations
1. Add GetNonce() and GetTimestamp() methods to XXHandshake for API consistency with IKHandshake
2. Fix XXHandshake.GetRemoteStaticKey() to return defensive copy like IKHandshake implementation
3. Inject time provider interface to NewIKHandshake for deterministic testing (follow async package pattern)
4. Move HandshakeMaxAge/HandshakeMaxFutureDrift constants to noise package or remove doc.go references
5. Add inline comment in handshake_coverage_test.go explaining intentional error ignoring for coverage
