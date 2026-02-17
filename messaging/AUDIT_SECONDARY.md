# Audit: github.com/opd-ai/toxcore/messaging
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
This is a secondary deep-dive audit of the messaging package, following up on the initial audit from 2026-02-17. The messaging package provides core message handling with encryption, delivery tracking, retry logic, and state management. While structurally sound with good concurrency safety patterns and comprehensive encryption tests, the implementation contains critical security vulnerabilities: non-deterministic timestamp usage enabling timing attacks, missing automatic message padding allowing traffic analysis, no message size validation risking memory exhaustion, and incomplete encrypted data encoding specification creating interoperability risks.

## Issues Found

### High Severity
- [ ] **high** security — Non-deterministic `time.Now()` creates timing side-channels; attackers can correlate message timestamps with network activity for traffic analysis (`message.go:111`)
- [ ] **high** security — Non-deterministic `time.Now()` in retry scheduling leaks information about network congestion and connection quality to timing attackers (`message.go:289`)
- [ ] **high** security — Non-deterministic `time.Since()` in retry interval calculation may behave unpredictably in simulations or during system clock changes (`message.go:239`)
- [ ] **high** security — Missing automatic message padding to standard sizes (256B, 1024B, 4096B); message length leaks metadata enabling traffic analysis attacks per Tox security model (`message.go:274-282`)
- [ ] **high** validation — No maximum message length validation; unbounded `text` field in `SendMessage` allows memory exhaustion attacks (Tox protocol specifies 1372 bytes max) (`message.go:178-180`)
- [ ] **high** integration — Encrypted message text stored as raw bytes converted to string without proper encoding; comment states "base64 or hex encoding would be done at transport layer" but no interface contract enforces this, risking data corruption (`message.go:279-280`)

### Medium Severity
- [ ] **med** error-handling — `encryptMessage` returns `nil` error for backward compatibility when no key provider exists; should return typed sentinel error (e.g., `ErrNoEncryption`) for explicit handling by callers (`message.go:249-256`)
- [ ] **med** concurrency — `SendMessage` launches unbounded goroutine via `attemptMessageSend` without lifecycle management; potential goroutine leak on Tox shutdown if messages are pending (`message.go:197`)
- [ ] **med** determinism — Retry intervals use wall-clock time comparison which fails in deterministic testing environments and can behave incorrectly during system clock adjustments (NTP, timezone changes) (`message.go:239`)
- [ ] **med** integration — No verification that transport layer correctly handles encrypted binary data in `Message.Text` field; string field may not preserve binary data integrity (`message.go:280`)
- [ ] **med** state-machine — `shouldKeepInQueue` mutates message state from `MessageStateFailed` back to `MessageStatePending` during iteration, breaking encapsulation and making state transitions non-obvious (`message.go:362`)

### Low Severity
- [ ] **low** documentation — Missing `doc.go` package documentation file; package comment in `message.go:1-12` should be extracted to `doc.go` per Go conventions
- [ ] **low** documentation — `MessageTransport` interface lacks comprehensive godoc explaining implementation requirements, especially error semantics and thread-safety guarantees (`message.go:54-57`)
- [ ] **low** documentation — `KeyProvider` interface lacks godoc explaining key lifecycle, rotation requirements, and relationship with friend management (`message.go:60-63`)
- [ ] **low** documentation — `MessageManager` type lacks godoc comment explaining concurrency safety model, initialization requirements, and lifecycle management (`message.go:84-94`)
- [ ] **low** documentation — `SetTransport` method lacks godoc explaining when this should be called in initialization sequence and whether it can be called multiple times (`message.go:161-165`)
- [ ] **low** documentation — `SetKeyProvider` method lacks godoc explaining when this should be called in initialization sequence and whether it can be called multiple times (`message.go:168-172`)
- [ ] **low** documentation — `ProcessPendingMessages` method lacks godoc explaining when this should be called (iteration loop, timer-based, event-driven) (`message.go:203-207`)
- [ ] **low** documentation — `encryptMessage` is not exported but performs critical security operations; should have comprehensive internal documentation explaining encryption scheme, nonce generation, and encoding strategy (`message.go:247-283`)
- [ ] **low** style — Inconsistent error construction: `SendMessage` uses `errors.New`, `KeyProvider` mock uses custom `MessageError` type, crypto package likely uses custom errors; should standardize on custom error types with codes (`message.go:179`)
- [ ] **low** optimization — `GetMessagesByFriend` allocates slice without size hint despite iterating all messages; pre-allocating with capacity would reduce allocations for high-message-count scenarios (`message.go:413`)
- [ ] **low** testing — Test coverage at 46% misses critical paths: `ProcessPendingMessages` workflow, `cleanupProcessedMessages` edge cases, `MarkMessageDelivered`/`MarkMessageRead` callback interaction, and `GetMessagesByFriend` with multiple messages (`encryption_test.go`)

## Test Coverage
46.0% (target: 65%)

### Coverage Analysis
**Covered functionality:**
- Message creation and initialization (`NewMessage`)
- Message encryption with various key configurations
- Encryption failure handling (missing friend keys)
- Transport failure scenarios with retry logic
- Concurrent message sending
- Unencrypted message fallback with warning logs
- Message type preservation during encryption
- Multi-friend encryption with unique nonces

**Missing test coverage:**
- `ProcessPendingMessages()` - No tests for the periodic processing workflow
- `retrievePendingMessages()` - No tests for safe queue copying
- `processMessageBatch()` - No tests for batch processing logic
- `shouldProcessMessage()` - No tests for retry interval timing logic
- `cleanupProcessedMessages()` - No tests for queue cleanup edge cases
- `shouldKeepInQueue()` - No tests for state-based retention logic
- `MarkMessageDelivered()` - No tests for delivery state updates with callbacks
- `MarkMessageRead()` - No tests for read receipts with callbacks
- `GetMessage()` - No tests for message retrieval by ID
- `GetMessagesByFriend()` - No tests with multiple messages per friend
- No benchmark tests for high-throughput scenarios (1000+ messages/sec)
- No integration tests with real `Transport` and `KeyProvider` implementations

## Integration Status

### Confirmed Integrations
The messaging package is properly integrated with toxcore:
- **Declaration**: `toxcore.go:288` declares `messageManager *messaging.MessageManager` field in `Tox` struct
- **Initialization**: `toxcore.go:637` creates `MessageManager` via `messaging.NewMessageManager()`
- **Transport binding**: `toxcore.go:3438` implements `SendMessagePacket` method making Tox conform to `MessageTransport` interface
- **Key provider binding**: Tox implements `KeyProvider` interface (verified in `messagemanager_initialization_test.go`)
- **Message sending**: `toxcore.go:2140-2141` uses `MessageManager.SendMessage` in friend message flow

### Integration Gaps
- **No savedata persistence**: `Message` and `MessageManager` state not serialized in Tox savedata format; pending messages lost on restart
- **No message ID mapping**: Tox API likely uses different message IDs than `MessageManager.nextID`; no bidirectional mapping for acknowledgments
- **Incomplete async integration**: No connection between `MessageManager` and `async.AsyncManager` for offline message queueing via obfuscated storage nodes
- **Missing transport layer encoding**: `message.go:279` comment indicates transport layer should handle base64/hex encoding, but `toxcore.go:3438` implementation may not perform encoding
- **No callback wiring**: `OnDeliveryStateChange` callbacks set by users may not be triggered by transport layer delivery confirmations
- **No error propagation**: Encryption errors in `attemptMessageSend` logged but not surfaced to caller; SendMessage returns immediately before encryption happens

## Recommendations

### Critical (Address Immediately)

1. **Implement deterministic time provider** — Create `TimeProvider` interface with `Now()` and `Since()` methods; inject into `MessageManager` constructor; default to real time, allow mock time for tests and simulations (`message.go:111, 239, 289`)
   ```go
   type TimeProvider interface {
       Now() time.Time
       Since(t time.Time) time.Duration
   }
   ```

2. **Add automatic message padding** — Implement padding in `encryptMessage` to round messages to 256B/1024B/4096B boundaries per Tox protocol; prevents traffic analysis attacks on message length (`message.go:274-282`)
   ```go
   func padMessage(data []byte) []byte {
       sizes := []int{256, 1024, 4096}
       for _, size := range sizes {
           if len(data) < size {
               padded := make([]byte, size)
               copy(padded, data)
               return padded
           }
       }
       return data
   }
   ```

3. **Enforce maximum message length** — Define `const MaxMessageLength = 1372` per Tox spec; validate in `SendMessage` before queueing (`message.go:178`)

4. **Formalize encrypted data encoding contract** — Either implement base64 encoding in `encryptMessage` before setting `message.Text`, or change `Message.Text` to `[]byte` and document that transport must handle encoding (`message.go:279-280`)

### High Priority

5. **Add goroutine lifecycle management** — Pass `context.Context` to `attemptMessageSend`; cancel goroutines on `MessageManager.Close()`; prevents goroutine leaks (`message.go:197`)

6. **Return typed encryption errors** — Create `ErrNoEncryption` sentinel error; return from `encryptMessage` when no key provider; allows callers to distinguish unencrypted mode (`message.go:249-256`)

7. **Increase test coverage to 65%** — Add table-driven tests for `ProcessPendingMessages`, `shouldProcessMessage`, `cleanupProcessedMessages`, delivery/read callbacks, and multi-message scenarios

### Medium Priority

8. **Fix state mutation in iteration** — Move state transition logic from `shouldKeepInQueue` to explicit `retryMessage()` method; maintain clear state machine boundaries (`message.go:362`)

9. **Add message persistence** — Implement `Serialize()`/`Deserialize()` methods for `Message`; integrate with Tox savedata format; persist pending messages across restarts

10. **Wire delivery confirmations** — Connect transport layer ACKs to `MarkMessageDelivered`; propagate to `OnDeliveryStateChange` callbacks

### Low Priority

11. **Create `doc.go`** — Extract package comment to separate `doc.go` file; add comprehensive examples showing initialization sequence, encryption setup, and callback usage

12. **Enhance godoc coverage** — Add comprehensive documentation for all exported types and methods; document thread-safety guarantees, initialization order, and lifecycle requirements

13. **Standardize error types** — Create custom error types with error codes; replace `errors.New` with structured errors for better error handling

14. **Add benchmark suite** — Create benchmarks for high-throughput message sending, encryption performance, and queue management scalability

## Security Considerations

### Timing Attacks (Critical)
The use of `time.Now()` at `message.go:111` and `message.go:289` creates timing side-channels. An attacker observing network traffic can correlate message timestamps with packet transmission times, enabling traffic analysis even with encryption. **Recommendation**: Use monotonic time source or dependency-injected time provider.

### Traffic Analysis via Message Length (Critical)
Without automatic padding, message lengths leak metadata. An attacker can distinguish "yes"/"no" replies from longer messages, identify command patterns, or fingerprint applications. Tox protocol specifies padding to 256B/1024B/4096B boundaries. **Recommendation**: Implement padding in `encryptMessage` before encryption.

### Memory Exhaustion (Critical)
No validation of message length allows malicious callers to send gigabyte-sized messages, exhausting memory. Tox protocol limits messages to 1372 bytes. **Recommendation**: Add `MaxMessageLength` constant and validate in `SendMessage`.

### Encrypted Data Encoding (High)
Storing encrypted binary data in `string` field without documented encoding risks data corruption (null bytes, UTF-8 validation, string interning). The comment at `message.go:279` punts encoding to "transport layer" without interface contract. **Recommendation**: Change `Message.Text` to `[]byte` or implement explicit base64 encoding.

### Goroutine Lifecycle (Medium)
Unbounded goroutines launched in `SendMessage` via `attemptMessageSend` lack cancellation mechanism. On application shutdown, pending messages continue attempting delivery, delaying cleanup. **Recommendation**: Add context cancellation.

## ECS Compliance
**N/A** — This package is not part of an ECS (Entity Component System) architecture. It follows a service/manager pattern appropriate for messaging infrastructure.

## Network Interface Compliance
**PASS** — No concrete network types (`net.UDPConn`, `net.TCPAddr`, etc.) used. Package defines abstract `MessageTransport` interface allowing any transport implementation.

## Deterministic Procgen Compliance
**FAIL** — Package uses non-deterministic time sources (`time.Now()`, `time.Since()`) which would make message replay non-deterministic in simulation environments. Crypto nonce generation via `crypto.GenerateNonce()` may also use OS entropy (not verified in this audit).

## Performance Notes
- Test coverage includes concurrent message sending (10 messages in parallel) without data races
- No profiling data available; benchmark tests recommended for production optimization
- `GetMessagesByFriend` iterates entire message map; consider indexing by friend ID for O(1) lookups with high message counts
- Queue cleanup via `cleanupProcessedMessages` creates new slice on each call; consider mark-and-sweep or circular buffer for better memory behavior
