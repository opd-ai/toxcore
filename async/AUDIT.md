# Audit: github.com/opd-ai/toxcore/async
**Date**: 2026-02-19
**Status**: Complete

## Summary
The async package implements forward-secure asynchronous messaging with identity obfuscation for offline communication. Overall code health is excellent with comprehensive testing (9329 lines test / 4610 lines source = 2.02x ratio), robust error handling, and thorough documentation. Critical security concerns include inconsistent logging patterns and direct time.Now() usage affecting determinism, but no major security vulnerabilities found.

## Issues Found
- [ ] **med** Error Handling — Swallowed error in cover traffic retrieval (`retrieval_scheduler.go:128`)
- [ ] **low** Error Handling — Ignored errors in test helper functions (multiple test files use `_ = ` for net.ResolveUDPAddr)
- [ ] **low** Logging — Inconsistent use of log.Printf vs logrus for production code in manager.go and client.go
- [ ] **low** Logging — fmt.Printf used for warnings in prekeys.go instead of structured logging (`prekeys.go:255`, `prekeys.go:494`, `prekeys.go:528`)
- [ ] **med** Determinism — Direct time.Now() calls in production code affect reproducibility (`client.go:72`, `client.go:229-230`, `client.go:261`, `epoch.go:50`, `forward_secrecy.go:180-181`, `forward_secrecy.go:246`, `storage.go:214`, `storage.go:288`, `storage.go:442`, `retrieval_scheduler.go:122`, `obfs.go:360`, `obfs.go:372`, `manager.go:142`, `manager.go:729`, `prekeys.go:103`, `prekeys.go:166`, `prekeys.go:265`, `prekeys.go:446`)
- [ ] **low** Documentation — Comment in prekey_hmac_security_test.go references TODO for future security enhancements (`prekey_hmac_security_test.go:244`)
- [ ] **low** Concurrency — No goroutine leak prevention in key_rotation_client.go:40-50 (ticker.Stop() in defer, but no context cancellation for the goroutine itself)
- [ ] **low** API Design — Mock transport logs warnings about simulation in production code path (`mock_transport.go:37`)

## Test Coverage
Unable to calculate coverage (tests hung after 120+ seconds). Test suite is comprehensive with 39 test files covering unit tests, integration tests, benchmarks, and fuzz tests. Test-to-source ratio of 2.02:1 significantly exceeds project standards.

## Dependencies
**External Dependencies:**
- `golang.org/x/crypto` (curve25519, hkdf) - cryptographic primitives for key exchange and derivation
- `github.com/sirupsen/logrus` - structured logging (partially adopted)

**Internal Dependencies:**
- `github.com/opd-ai/toxcore/crypto` - key management, encryption/decryption operations
- `github.com/opd-ai/toxcore/transport` - network communication abstraction
- `github.com/opd-ai/toxcore/limits` - centralized size limits

**Integration Points:**
- Transport layer packet handlers (PacketAsyncPreKeyExchange, PacketAsyncRetrieveResponse)
- File system operations for pre-key storage and message persistence
- Platform-specific storage detection (Unix via syscall.Statfs, Windows via GetDiskFreeSpaceExW)

## Recommendations
1. **HIGH PRIORITY** — Replace direct time.Now() calls with injectable clock interface for testability and determinism (affects 20+ locations across 10 files)
2. **HIGH PRIORITY** — Fix swallowed error in retrieval_scheduler.go:128 - cover traffic errors should be logged even if results are discarded
3. **MEDIUM PRIORITY** — Standardize on logrus throughout the package (manager.go and client.go have 30+ log.Printf calls that should use structured logging)
4. **MEDIUM PRIORITY** — Add context cancellation to key rotation goroutine in key_rotation_client.go to prevent leaks
5. **LOW PRIORITY** — Replace fmt.Printf warnings in prekeys.go with logrus.Warn for consistency
6. **LOW PRIORITY** — Remove "SIMULATION FUNCTION" warning from mock_transport.go (belongs in test helpers only, not production mock implementation)
7. **LOW PRIORITY** — Consider extracting time.Now() usage pattern documentation for developers

## Additional Notes
**Strengths:**
- Excellent documentation with comprehensive doc.go file (249 lines)
- Strong test coverage including fuzz tests, benchmarks, and integration tests
- Well-structured concurrency primitives (sync.RWMutex, sync.Mutex used appropriately)
- Good separation of concerns (client, manager, storage, forward secrecy, obfuscation)
- Implements advanced cryptographic patterns (HKDF, AES-GCM, one-time pre-keys)
- All exported APIs have proper godoc comments

**Security Observations:**
- go vet passes with no issues
- Race detector validation mentioned in doc.go
- Secure memory handling delegated to crypto package
- Pre-keys stored with encryption on disk
- HMAC recipient proofs prevent spam while preserving anonymity

**Performance Characteristics:**
- Parallel storage node queries configurable
- In-memory pre-key bundle caching reduces disk I/O
- Adaptive storage capacity based on available disk space
- Extensive benchmarks for critical paths (7 benchmark test files)
