# Audit: github.com/opd-ai/toxcore/async
**Date**: 2026-02-20
**Status**: Needs Work

## Summary
The async package implements forward-secure asynchronous messaging with identity obfuscation, comprising 18 source files (4,798 LOC) and 34 test files. The package demonstrates sophisticated cryptographic design with excellent concurrency patterns and comprehensive documentation. Critical issues include swallowed errors in cover traffic, potential resource leaks in key rotation, and insufficient error context in message delivery paths.

## Issues Found
- [x] **high** Error Handling — Swallowed error in cover traffic retrieval prevents silent failure detection (`retrieval_scheduler.go:128`)
- [x] **high** Resource Management — Key rotation goroutine lacks shutdown mechanism, creating goroutine leak (`key_rotation_client.go:40-50`)
- [x] **high** Error Context — Message delivery errors silently ignored, preventing debugging of decryption/handler failures (`manager.go:385-386`)
- [x] **med** Concurrency Safety — Race condition risk in messageHandler callback usage across goroutines (`manager.go:382-455`)
- [x] **med** Error Handling — No error wrapping for storage retrieval failures loses diagnostic context (`manager.go:405-407`)
- [x] **low** Documentation — Comment for TODO enhancement visible in production code (`prekey_hmac_security_test.go:244`)
- [x] **low** API Design — Exported constants (PreKeyLowWatermark, PreKeyMinimum) lack package prefix for namespace clarity (`forward_secrecy.go:50-64`)

## Test Coverage
Unable to measure full coverage due to test timeouts (network-intensive tests). Single test sample shows 0.3% coverage, but package includes 34 comprehensive test files covering unit tests, integration tests, benchmarks, and fuzz tests. Test-to-source ratio: 1.89:1 (34 test files for 18 source files).

**Test File Categories:**
- Unit tests: 20+ files covering core functionality
- Integration tests: 4 files (retrieval_integration_test.go, network_operations_test.go)
- Benchmark tests: 4 files (client_benchmark_test.go, crypto_benchmark_test.go, manager_benchmark_test.go, storage_benchmark_test.go)
- Security validation: 4 files (prekey_hmac_security_test.go, prekey_signature_test.go, secure_prekey_test.go)
- Fuzz tests: async_fuzz_test.go

## Dependencies
**External Dependencies:**
- `github.com/sirupsen/logrus` — Structured logging (appropriate for security-critical operations)
- `golang.org/x/crypto/curve25519` — Elliptic curve key exchange (standard crypto primitive)
- `golang.org/x/crypto/hkdf` — Key derivation for pseudonyms (industry standard)
- `golang.org/x/sys/unix` — Platform-specific storage detection (appropriate)

**Internal Dependencies:**
- `github.com/opd-ai/toxcore/crypto` — Cryptographic operations and key management
- `github.com/opd-ai/toxcore/transport` — Network transport abstraction
- `github.com/opd-ai/toxcore/limits` — Centralized message size limits

**Integration Points:**
- Transport layer via `transport.Transport` interface for packet handling
- Packet types: `PacketAsyncPreKeyExchange`, `PacketAsyncRetrieveResponse`
- Platform-specific build tags for storage capacity detection (unix/windows)

## Code Quality Analysis

### Strengths
1. **Excellent Documentation**: Comprehensive 250-line doc.go with usage examples, security properties, and integration guidelines
2. **Strong Concurrency Safety**: All major types use sync.RWMutex/Mutex with documented race-free patterns
3. **Security-First Design**: HKDF pseudonyms, AES-GCM encryption, forward secrecy via one-time pre-keys
4. **Interface-Based Design**: Clean separation with transport.Transport abstraction
5. **Comprehensive Testing**: 34 test files including benchmarks, fuzz tests, and security validation
6. **Proper Error Types**: Package-level sentinel errors (ErrMessageNotFound, ErrStorageFull, etc.)
7. **Platform Abstraction**: Build-tagged storage detection for Unix/Windows

### API Design
- 28 exported types with clear naming conventions
- 226 total methods (mix of exported/unexported)
- 117 error-returning functions demonstrate proper error handling patterns
- MinimalStorageCapacity/MaxStorageCapacity constants well-documented with rationale
- Message padding to standard sizes (256B, 1024B, 4096B) prevents traffic analysis

### Concurrency Patterns
- AsyncManager uses sync.RWMutex for state protection
- AsyncClient uses dual mutex strategy (mutex for read/write, channelMutex for channels)
- PreKeyStore properly locks bundle access
- Race detector validation mentioned in doc.go:210
- Explicit race avoidance documented in manager.go:168, 382, 414, 423, 455

### Error Handling
- Consistent error wrapping with fmt.Errorf and %w
- Validation functions separate concerns (validateMessage, validatePreKeys)
- Most errors include diagnostic context
- **Gap**: Cover traffic silently swallows errors (HIGH priority)
- **Gap**: Message delivery failures not propagated to caller (HIGH priority)

## Recommendations
1. **[HIGH] Fix error swallowing in retrieval_scheduler.go:128** — Log or track cover traffic errors: `if _, err := rs.client.RetrieveObfuscatedMessages(); err != nil { logrus.WithError(err).Warn("Cover traffic retrieval failed") }`
2. **[HIGH] Add shutdown mechanism for key rotation goroutine** — Store context/cancel in AsyncClient, call cancel in cleanup: prevents goroutine leak in long-running services
3. **[HIGH] Propagate message delivery errors** — Return error from deliverPendingMessagesWithHandler and log handler failures: critical for debugging decryption/callback issues
4. **[MED] Wrap storage retrieval errors** — Change `return nil, err` to `return nil, fmt.Errorf("failed to retrieve messages for %x: %w", friendPK[:8], err)` in manager.go:407
5. **[MED] Document thread-safety of messageHandler callback** — Add godoc comment explaining callback must be thread-safe or use synchronization
6. **[LOW] Remove production TODO comment** — Move enhancement notes to separate ENHANCEMENTS.md file
7. **[LOW] Consider renaming exported constants** — Add "Async" prefix: AsyncPreKeyLowWatermark, AsyncPreKeyMinimum for namespace clarity
