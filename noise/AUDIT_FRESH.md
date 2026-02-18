# Audit: github.com/opd-ai/toxcore/noise
**Date**: 2026-02-18
**Status**: Needs Work

## Summary
The noise package implements Noise Protocol Framework (IK and XX patterns) for secure handshakes with mutual authentication and forward secrecy. The package has strong test coverage (89.0%) and integrates with transport layer. However, a **CRITICAL BUG** exists in `IKHandshake.GetLocalStaticKey()` returning ephemeral instead of static key, breaking identity verification. Additionally, non-deterministic time usage prevents reproducible testing, and package lacks doc.go documentation.

## Issues Found
- [ ] **high** bug — `IKHandshake.GetLocalStaticKey()` returns `LocalEphemeral()` instead of static public key, breaking peer identity verification (`handshake.go:246`)
- [ ] **high** architecture — IKHandshake struct lacks `localPubKey` field (unlike XXHandshake), causing GetLocalStaticKey() to incorrectly use ephemeral key (`handshake.go:38-46`)
- [ ] **med** determinism — Uses `time.Now().Unix()` for handshake timestamp, preventing deterministic testing (`handshake.go:105`)
- [ ] **med** determinism — Uses `crypto/rand.Read` for nonce generation; acceptable for security but prevents deterministic testing (`handshake.go:109`)
- [ ] **low** doc-coverage — Package lacks `doc.go` file with overview of IK vs XX pattern selection guidance (root of `noise/`)
- [ ] **low** integration — No timestamp validation helper provided despite GetTimestamp() accessor (`handshake.go:262-264`)
- [ ] **low** integration — No nonce replay validation helper despite GetNonce() accessor (`handshake.go:257-259`)

## Test Coverage
89.0% (target: 65%) ✅

**Test Quality:**
- Comprehensive unit tests with table-driven patterns (`handshake_test.go`)
- Extensive coverage tests for edge cases and error paths (`handshake_coverage_test.go`)
- Fuzz testing for malformed message handling (`handshake_fuzz_test.go`)
- Tests per source file: 3 test files for 1 source file (3:1 ratio)
- Coverage exceeds target by 24 percentage points

## Integration Status
**Primary Integration Points:**
- `transport/noise_transport.go` — Main wrapper using `toxnoise.IKHandshake` for session encryption (2 usages)
- `transport/versioned_handshake.go` — Protocol version negotiation with Noise handshake (2 usages)
- Test usage in `security_validation_test.go`, `dht/version_negotiation_test.go`

**Integration Strengths:**
- Replay protection via nonce tracking implemented in transport layer
- Timestamp freshness validation with `HandshakeMaxAge` (5 minutes) and `HandshakeMaxFutureDrift` (1 minute)
- Session lifecycle management with cleanup goroutines and idle timeout (5 minutes)
- Proper aliasing as `toxnoise` to avoid conflict with `flynn/noise` library

**Integration Gaps:**
- No centralized handshake pattern registry (IK/XX selection is ad-hoc)
- No metrics/telemetry for handshake success/failure rates
- Validation helpers (timestamp/nonce checking) live in transport layer instead of this package
- Configuration for timeouts hardcoded in transport (should be configurable)

## Recommendations
1. **CRITICAL**: Fix `GetLocalStaticKey()` bug — Add `localPubKey []byte` field to IKHandshake struct (line 44, after timestamp field); populate it during `NewIKHandshake()` (after line 76: `copy(localPubKey, keyPair.Public[:])`); change `GetLocalStaticKey()` to return copy of `localPubKey` instead of `LocalEphemeral()`
2. **High Priority**: Add time provider abstraction — Create `TimeProvider` interface with `Now() time.Time` method; add optional parameter to `NewIKHandshake()` (default `time.Now`); enables deterministic testing and aligns with crypto package patterns
3. **High Priority**: Add nonce provider abstraction — Create `NonceProvider` interface with `Read([]byte) error` method; add optional parameter to `NewIKHandshake()` (default `crypto/rand.Read`); enables deterministic testing
4. **Medium Priority**: Create `doc.go` — Add package-level documentation explaining IK vs XX pattern selection criteria, security properties (mutual authentication, forward secrecy, KCI resistance), and integration examples
5. **Low Priority**: Add validation helpers — Implement `ValidateTimestamp(timestamp int64, maxAge time.Duration) error` and `ValidateNonce(nonce [32]byte, usedNonces map[[32]byte]bool) error` to centralize validation logic currently in transport layer

## Detailed Analysis

### ✅ Stub/Incomplete Code
**PASS** — No stub implementations, TODOs, or FIXMEs found. All functions have complete implementations using flynn/noise library.

### ✅ ECS Compliance
**N/A** — This is a pure cryptographic library package with no ECS components or systems.

### ⚠️ Deterministic Procgen
**PARTIAL FAIL** — 2 issues found:
1. **Non-deterministic time**: `time.Now().Unix()` at line 105 for timestamp generation
2. **Non-deterministic randomness**: `crypto/rand.Read` at line 109 for nonce generation

**Note**: Cryptographic operations SHOULD use OS entropy (`crypto/rand`) for security. The issue is lack of abstraction for deterministic *testing*. Production code correctly uses secure randomness.

**No global rand usage** — Package correctly uses `crypto/rand.Reader` (via `rand.Read()`) which is appropriate for cryptographic nonces. No `math/rand` usage found.

### ✅ Network Interfaces  
**N/A** — This package has no network operations. It operates on byte slices and cryptographic state.

### ✅ Error Handling
**PASS** — All errors properly checked and wrapped:
- Line 72: `crypto.FromSecretKey()` error checked, secure cleanup on error (line 74-75)
- Line 109: `rand.Read()` error checked with descriptive message (line 110)
- Line 114: `noise.NewHandshakeState()` error checked (line 115-116)
- All `state.WriteMessage()` and `state.ReadMessage()` calls have error checking (lines 141, 163, 169, 194, 333, 355)
- Errors wrapped with context via `fmt.Errorf(..., %w, err)` pattern throughout

**No logging**: Package correctly has no logging statements (pure library code). Logging responsibility is on consuming code (transport layer).

### ✅ Test Coverage
**PASS** — 89.0% coverage exceeds 65% target by 24 percentage points

**Test Quality Assessment:**
- `handshake_test.go` — Basic handshake flows and error cases
- `handshake_coverage_test.go` — Edge cases including the GetLocalStaticKey bug test (line showing it returns ephemeral)
- `handshake_fuzz_test.go` — Fuzzing for malformed inputs
- Table-driven tests present for error scenarios
- No benchmarks found (would be valuable for performance-critical crypto operations)

### ⚠️ Doc Coverage
**PARTIAL FAIL** — 1 issue:
- **Missing `doc.go`**: No package-level documentation file (root of `noise/`)
- Package comment exists in `handshake.go:1-4` but should be in dedicated doc.go

**Strong Points:**
- All exported types have godoc comments (IKHandshake, XXHandshake, HandshakeRole)
- All exported functions have comprehensive godoc (13 of 13 exported methods documented)
- Constants (Initiator, Responder) have inline comments (lines 29-32)
- Error variables have descriptive comments (lines 17-22)
- Internal helper `validateHandshakePattern` well-documented (lines 403-436)

### ⚠️ Integration Points
**PARTIAL FAIL** — Well-integrated but missing centralized validation:

**✅ Properly Integrated:**
- Transport layer wraps IKHandshake for encryption (`noise_transport.go:47`)
- Version negotiation uses IKHandshake (`versioned_handshake.go`)
- Security validation tests demonstrate correct usage
- Aliased as `toxnoise` to avoid name conflicts with `flynn/noise`

**❌ Missing/Incomplete:**
- Timestamp validation logic lives in transport layer (`transport/noise_transport.go:30-31`) instead of this package
- Nonce replay protection tracking in transport layer instead of exported helper
- No pattern selection registry/factory for consistent IK/XX choice
- No exported constants for recommended handshake age limits

**No registration required** — Pure library package, no system initialization needed.

## Security Analysis

### Critical Security Bug
**🚨 HIGH SEVERITY**: `IKHandshake.GetLocalStaticKey()` returns the wrong key type!

**Impact:**
- Peer identity verification will fail or use wrong key
- Breaks the entire point of IK pattern (verifying peer identity)
- May allow impersonation attacks if peer verification relies on this method

**Code Evidence:**
```go
// handshake.go:244-254
func (ik *IKHandshake) GetLocalStaticKey() []byte {
    localEphemeral := ik.state.LocalEphemeral()  // ❌ WRONG: ephemeral, not static!
    if len(localEphemeral.Public) > 0 {
        key := make([]byte, len(localEphemeral.Public))
        copy(key, localEphemeral.Public)
        return key
    }
    return nil
}
```

**Compare to XXHandshake (correct implementation):**
```go
// handshake.go:392-401
func (xx *XXHandshake) GetLocalStaticKey() []byte {
    if len(xx.localPubKey) > 0 {  // ✅ CORRECT: uses stored static key
        key := make([]byte, len(xx.localPubKey))
        copy(key, xx.localPubKey)
        return key
    }
    return nil
}
```

**Architectural Issue:**
- IKHandshake struct lacks `localPubKey` field (line 38-46)
- XXHandshake correctly has `localPubKey []byte` field (line 274)
- IKHandshake should store static public key during initialization (line 52-120)

### Security Strengths
1. ✅ Uses formally verified Noise Protocol Framework (`flynn/noise` library)
2. ✅ Proper key clamping via `crypto.FromSecretKey()` (line 71)
3. ✅ Secure memory cleanup with `crypto.ZeroBytes()` (lines 74, 86, 303)
4. ✅ Cryptographically secure randomness via `crypto/rand.Reader` (lines 91, 109)
5. ✅ Mutual authentication via IK pattern (initiator knows responder's static key)
6. ✅ Forward secrecy through ephemeral key exchange
7. ✅ Authenticated encryption via ChaCha20-Poly1305 (line 88)
8. ✅ Replay protection via unique nonce per handshake (lines 44, 109)
9. ✅ Input validation (key lengths checked at lines 59-65)

### Security Recommendations
1. **Immediate**: Fix GetLocalStaticKey() bug to prevent identity verification failures
2. **High**: Add integration test with matching keypairs demonstrating successful mutual authentication
3. **Medium**: Document threat model and security assumptions in doc.go (replay attacks, KCI resistance, forward secrecy guarantees)
4. **Low**: Consider HSM integration points for production deployments with hardware key storage

## Code Quality Metrics
- **Source files**: 1 (`handshake.go`)
- **Lines of code**: 436
- **Test files**: 3 (handshake_test.go, handshake_coverage_test.go, handshake_fuzz_test.go)
- **Test coverage**: 89.0%
- **Exported types**: 3 (IKHandshake, XXHandshake, HandshakeRole)
- **Exported functions/methods**: 13
- **Error types**: 3 custom errors
- **go vet**: PASS ✅
- **Stub count**: 0
- **TODO/FIXME count**: 0
- **Critical bugs**: 1 (GetLocalStaticKey returns wrong key type)

## Conclusion
The noise package implements Noise Protocol Framework correctly for the most part, with excellent test coverage and secure cryptographic practices. However, a **critical bug in `GetLocalStaticKey()`** breaks identity verification by returning ephemeral instead of static keys. This must be fixed immediately. Additionally, non-deterministic time/nonce generation (appropriate for security) prevents deterministic testing. Package documentation is inline but should be in doc.go. Overall status: **Needs Work** due to critical bug, but otherwise solid foundation.
