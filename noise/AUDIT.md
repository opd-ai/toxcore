# Audit: github.com/opd-ai/toxcore/noise
**Date**: 2026-02-20
**Status**: Needs Work

## Summary
The noise package provides Noise Protocol Framework implementations (IK and XX patterns) for secure cryptographic handshakes. Overall code quality is high with excellent documentation and test coverage (89.4%). Critical issue: unchecked error from rand.Read() at line 139 creates potential security vulnerability.

## Issues Found
- [x] **high** Error Handling — Unchecked error from `rand.Read(ik.nonce[:])` in handshake nonce generation (`handshake.go:139`)
- [ ] **med** Concurrency Safety — No mutex protection for IKHandshake and XXHandshake state, though documented as not thread-safe (`handshake.go:38,298`)
- [ ] **med** API Design — `GetRemoteStaticKey()` has inconsistent copy behavior between IKHandshake (copies, line 269-270) and XXHandshake (no copy, line 421)
- [ ] **low** Documentation — Thread safety warning exists in doc.go but not in struct godoc comments
- [ ] **low** Error Handling — `GetRemoteStaticKey()` for XXHandshake doesn't validate empty key like IKHandshake does (`handshake.go:421`)

## Test Coverage
89.4% (target: 65%) ✓

Test suite includes:
- Unit tests for IK and XX patterns
- Fuzzing tests for handshake robustness
- Coverage tests for edge cases
- Race detector passes cleanly

## Dependencies
**External:**
- `github.com/flynn/noise` v1.1.0 — Noise Protocol Framework (formally verified)

**Internal:**
- `github.com/opd-ai/toxcore/crypto` — Key pair generation, secure memory wiping

**Standard Library:**
- `crypto/rand` — Cryptographic random number generation
- `errors`, `fmt` — Error handling
- `time` — Timestamp generation

## Recommendations
1. **CRITICAL**: Check error return from `rand.Read(ik.nonce[:])` at line 139. Unchecked crypto RNG failure is a security vulnerability.
2. Add mutex protection to handshake structs if concurrent access is a real-world scenario, or add clear panics/errors on misuse.
3. Standardize `GetRemoteStaticKey()` behavior — XXHandshake should copy and validate like IKHandshake does.
4. Add thread safety warnings to IKHandshake and XXHandshake struct godoc comments (not just package doc).
5. Consider adding validation for XXHandshake's PeerStatic() return value before exposing it.
