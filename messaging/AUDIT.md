# Audit: github.com/opd-ai/toxcore/messaging
**Date**: 2026-02-19
**Status**: Needs Work

## Summary
The messaging package provides core message handling, encryption, delivery tracking, and retry logic. Implementation is mostly complete with well-documented interfaces and comprehensive test coverage. However, critical race conditions exist in tests accessing message state without synchronization, and test coverage is below the target threshold at 53.3%.

## Issues Found
- [x] **high** Concurrency Safety — Race condition: Test code directly accesses `message.State` and `message.Text` without acquiring `message.mu` lock (`encryption_test.go:99,104,108,109,90`)
- [x] **high** Concurrency Safety — Race condition: mockTransport.sentMessages accessed without synchronization (`mocks_test.go:64`, `encryption_test.go:104,108`)
- [ ] **med** Test Coverage — Coverage at 53.3%, below 65% target; missing coverage for retry logic paths and edge cases
- [ ] **med** API Design — NewMessage function uses `time.Now()` directly instead of timeProvider, bypassing determinism for standalone usage (`message.go:185`)
- [ ] **low** Documentation — TimeProvider interface lacks godoc comment explaining its purpose for testing and security (`message.go:102`)
- [ ] **low** Error Handling — Error from `crypto.GenerateKeyPair()` silently ignored in test setup (`mocks_test.go:34`)
- [ ] **low** API Design — Message.deliveryCallback field is unexported but has exported setter; consider making Message.mu unexported and providing GetState() accessor (`message.go:130,132`)
- [ ] **low** Determinism — NewMessage export uses time.Now() bypassing timeProvider abstraction; could affect reproducibility in external callers (`message.go:185`)

## Test Coverage
53.3% (target: 65%)

Coverage gaps:
- Retry logic branches (message retry count edge cases)
- Context cancellation paths in shutdown
- Error handling for nil transport with pending messages
- ProcessPendingMessages edge cases with rapid state transitions

## Dependencies
**External:**
- `github.com/sirupsen/logrus` — Structured logging
- `encoding/base64` — Safe encoding for encrypted binary data
- `context` — Graceful shutdown with cancellation

**Internal:**
- `github.com/opd-ai/toxcore/crypto` — Encryption operations (GenerateNonce, Encrypt)
- `github.com/opd-ai/toxcore/limits` — Message size constants (MaxPlaintextMessage)

**Integration Points:**
- MessageTransport interface: Implemented by toxcore.go for packet transmission
- KeyProvider interface: Implemented by toxcore.go for key management

## Recommendations
1. **CRITICAL**: Add synchronization to test code accessing Message state fields, or provide thread-safe accessor methods (GetState(), GetText())
2. **CRITICAL**: Add mutex to mockTransport.sentMessages or use channels for thread-safe test message capture
3. **HIGH**: Increase test coverage to 65% by adding tests for retry logic, context cancellation, and error paths
4. **MED**: Handle error from crypto.GenerateKeyPair() in mock setup or document why panic is acceptable
5. **LOW**: Add godoc comment to TimeProvider interface explaining deterministic testing and security benefits
6. **LOW**: Consider making Message.mu unexported and providing GetState() accessor to enforce lock discipline
