# Audit: github.com/opd-ai/toxcore/messaging
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The messaging package implements core message handling for the Tox protocol with encryption support, delivery tracking, and retry logic. While the implementation is structurally sound, it has several critical issues: non-deterministic timestamp usage, missing message padding for traffic analysis resistance, lack of message size validation, and insufficient godoc documentation. Test coverage is below target at 46%.

## Issues Found

### High Severity
- [ ] **high** security — Non-deterministic `time.Now()` usage creates timing side-channels vulnerable to traffic analysis (`message.go:111`)
- [ ] **high** security — Non-deterministic `time.Now()` usage for retry scheduling may leak information about network conditions (`message.go:289`)
- [ ] **high** security — Missing automatic message padding to standard sizes (256B, 1024B, 4096B) allows traffic analysis attacks on message length (`message.go:274-282`)
- [ ] **high** validation — No maximum message length validation; unbounded text field could cause memory exhaustion (`message.go:178-180`)

### Medium Severity
- [ ] **med** error-handling — `encryptMessage` returns nil for backward compatibility when no key provider exists; should use typed error for explicit handling (`message.go:249-256`)
- [ ] **med** concurrency — `ProcessPendingMessages` launches goroutine in `SendMessage` without controlled context; potential goroutine leak on shutdown (`message.go:197`)
- [ ] **med** determinism — Retry intervals use wall-clock time comparison which may be non-deterministic in simulation/testing environments (`message.go:239`)
- [ ] **med** integration — No verification that `encryptMessage` correctly handles encrypted data encoding (mentioned as "base64 or hex encoding would be done at transport layer" but not implemented) (`message.go:279-280`)

### Low Severity
- [ ] **low** documentation — Missing `doc.go` package documentation file explaining messaging architecture and integration with Tox core
- [ ] **low** documentation — Exported `MessageTransport` interface lacks comprehensive godoc comments explaining implementation requirements (`message.go:54-57`)
- [ ] **low** documentation — Exported `KeyProvider` interface lacks comprehensive godoc comments explaining key lifecycle (`message.go:60-63`)
- [ ] **low** documentation — `MessageManager` type lacks godoc comments explaining concurrency safety and lifecycle (`message.go:84-94`)
- [ ] **low** documentation — `SetTransport` method lacks godoc explaining when this should be called in initialization sequence (`message.go:161-165`)
- [ ] **low** documentation — `SetKeyProvider` method lacks godoc explaining when this should be called in initialization sequence (`message.go:168-172`)
- [ ] **low** style — Inconsistent error handling: `SendMessage` returns typed errors but internal methods use generic errors (`message.go:179, 261, 400`)
- [ ] **low** optimization — `GetMessagesByFriend` allocates slice without size hint despite knowing message count (`message.go:413`)

## Test Coverage
46.0% (target: 65%)

### Missing Test Coverage
- No tests for `ProcessPendingMessages`, `retrievePendingMessages`, `processMessageBatch` workflow
- No tests for `shouldProcessMessage` retry interval logic
- No tests for `cleanupProcessedMessages` and `shouldKeepInQueue` edge cases
- No tests for `MarkMessageDelivered` and `MarkMessageRead` callback interaction
- No tests for `GetMessagesByFriend` with multiple messages
- No benchmark tests for high-throughput message sending

## Integration Status
The messaging package is properly integrated with the main Tox core:
- `toxcore.go:288` declares `messageManager *messaging.MessageManager` field
- `toxcore.go:637-639` initializes `MessageManager` and registers Tox as both `MessageTransport` and `KeyProvider`
- `toxcore.go:2139-2142` integrates `SendMessage` into friend message sending flow
- Tox properly implements both `MessageTransport` and `KeyProvider` interfaces (verified in `messagemanager_initialization_test.go`)

### Integration Gaps
- No serialization/deserialization support for `Message` persistence in savedata format
- Message state is not persisted across Tox instance restarts
- No integration with async messaging system for offline message handling
- Missing registration/hooks for message encryption in transport layer packet encoding

## Recommendations

1. **[CRITICAL] Replace `time.Now()` with dependency-injected time source** — Create `TimeProvider` interface allowing deterministic time in tests and preventing timing side-channels (`message.go:111, 289`)
   
2. **[CRITICAL] Implement automatic message padding** — Add padding logic in `encryptMessage` to round messages to standard sizes (256B, 1024B, 4096B) following Tox protocol specifications (`message.go:274-282`)

3. **[HIGH] Add message length validation** — Define and enforce `MaxMessageLength` constant (typically 1372 bytes for Tox) in `SendMessage` to prevent oversized messages (`message.go:178`)

4. **[HIGH] Fix encrypted data encoding** — Implement proper base64/hex encoding of encrypted data or clarify contract with transport layer about who owns encoding (`message.go:279-280`)

5. **[MEDIUM] Add graceful shutdown** — Pass context to goroutines in `attemptMessageSend` to enable clean shutdown and prevent goroutine leaks (`message.go:197`)

6. **[MEDIUM] Increase test coverage to 65%** — Add missing tests for pending message processing, retry logic, and state transitions

7. **[LOW] Create `doc.go`** — Add package-level documentation explaining messaging architecture, security properties, and integration patterns

8. **[LOW] Add godoc for all exported types** — Document `MessageTransport`, `KeyProvider`, `MessageManager`, and key methods with comprehensive comments
