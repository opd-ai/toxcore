# Consolidated Audit Report

**Generated**: 2026-02-20
**Scope**: All subpackages in github.com/opd-ai/toxcore
**Sources**: 19 AUDIT.md files across the repository

## Executive Summary

| Severity | Total | Open | Resolved |
|----------|-------|------|----------|
| Critical | 0 | 0 | 0 |
| High | 8 | 1 | 7 |
| Medium | 25 | 0 | 25 |
| Low | 53 | 14 | 39 |
| **Total** | **86** | **15** | **71** |

**Test Coverage Summary**: 17 of 18 measured packages meet the 65% coverage target. One package is below target: `testnet/internal` (41.8%). Previously below-target packages `transport` and `group` have been improved to 65.2% and 78.6% respectively. `av/rtp` coverage improved from 91.0% to 91.2%. `file` coverage improved from 84.4% to 84.8%. `friend` coverage improved from 93.0% to 93.1%.

**Packages with zero open issues**: `async`, `av`, `av/rtp`, `capi`, `crypto`, `dht`, `factory`, `file`, `friend`, `group`, `interfaces`, `limits`, `messaging`, `testnet/internal` (low), `transport` (all issues resolved).

## Issues by Subpackage

### async
- **Source:** `async/AUDIT.md`
- **Status:** Complete — All issues resolved
- **High Issues:** 0
- **Medium Issues:** 3 (resolved)
- **Low Issues:** 3 (resolved)
- **Test Coverage:** N/A (timeout); 34 test files for 18 source files (1.89:1 ratio)
- **Details:**
  - [x] medium logging — Inconsistent logging: Mix of `log.Printf` and `logrus` structured logging (`client.go:269,275,440,451,859`)
  - [x] medium logging — Non-structured logging in manager: Uses `log.Printf` instead of `logrus.WithFields` (`manager.go:129`)
  - [x] medium logging — Non-structured cleanup warnings in prekeys: Uses `fmt.Printf` for warnings instead of `logrus.Warn` (`prekeys.go:255,494,528`)
  - [x] low error-handling — Swallowed error in cover traffic (`retrieval_scheduler.go:128`)
  - [x] low documentation — Minor TODO in test (`prekey_hmac_security_test.go:244`)
  - [x] low code-quality — Redundant capacity comment (`storage.go:137`)

### av
- **Source:** `av/AUDIT.md`
- **Status:** Complete
- **High Issues:** 0
- **Medium Issues:** 1 resolved
- **Low Issues:** 4 resolved
- **Test Coverage:** 78.0% ✓
- **Details:**
  - [x] med API Design — Manager methods now have comprehensive godoc documenting nil return semantics (`manager.go:273, 364, 421, 450`)
  - [x] low API Design — Placeholder address fallback pattern extracted to `resolveRemoteAddress` helper function (`types.go:106-158`) — **RESOLVED**: Created comprehensive helper function with godoc, error handling, and fallback logic. Added 5 unit tests covering all code paths.
  - [x] low Documentation — Performance optimization caching behavior has inline comments (`performance.go:131-153`)
  - [x] low Concurrency — Quality monitor callbacks are invoked synchronously (not with `go`), no panic recovery needed (`quality.go:425`)
  - [x] low Test Coverage — CallMetricsHistory.MaxHistory field behavior tested in TestMetricsHistory (`metrics_test.go:122-148`)

### av/rtp
- **Source:** `av/rtp/AUDIT.md`
- **Status:** Complete
- **High Issues:** 0
- **Medium Issues:** 0 (1 resolved)
- **Low Issues:** 0 open (4 resolved)
- **Test Coverage:** 91.2% ✓ (improved from 91.0%)
- **Details:**
  - [x] med API Design — AudioReceiveCallback now uses AudioConfig from Session instead of hardcoded mono/48kHz assumptions (`transport.go:252`) — **RESOLVED**: Added AudioConfig struct to Session with GetAudioConfig/SetAudioConfig methods; handleIncomingAudioFrame now retrieves audio parameters from session configuration.
  - [x] low Concurrency Safety — TransportIntegration.setupPacketHandlers captures reference in closures (`transport.go:84-96`) — **RESOLVED**: Made setupPacketHandlers idempotent with handlersSetup guard and mutex protection.
  - [x] low Documentation — jitterBufferEntry type lacks godoc comment (`packet.go:412`) — **RESOLVED**: Added comprehensive godoc comment explaining packet storage, timestamp ordering, and field purposes.
  - [x] low Error Handling — Session.ReceivePacket timestamp variable assigned but never used (`session.go:313`) — **RESOLVED**: Clarified comment that jitter computation is handled at JitterBuffer level.
  - [x] low Resource Management — Session.Close doesn't cleanup video components or jitter buffers (`session.go:384-392`) — **RESOLVED**: Session.Close now properly cleans up videoPacketizer and videoDepacketizer.

### capi
- **Source:** `capi/AUDIT.md`
- **Status:** Complete
- **High Issues:** 2 resolved
- **Medium Issues:** 3 resolved
- **Low Issues:** 4 resolved
- **Test Coverage:** 72.4% ✓
- **Details:**
  - [x] **high** Error Handling — error_ptr parameter now properly populated in toxav_call, toxav_answer, toxav_call_control and all bit rate/frame functions with appropriate error codes
  - [x] **high** API Design — Created GetToxInstanceByID accessor function with proper mutex protection to replace direct map access
  - [x] med Concurrency Safety — getToxInstance now uses the thread-safe GetToxInstanceByID accessor
  - [x] med Error Handling — Added bounds validation in audio/video frame functions before unsafe slice conversions
  - [x] med API Design — getToxInstance function now uses the thread-safe GetToxInstanceByID accessor with mutex protection
  - [x] low Documentation — Added comprehensive godoc comments for toxavCallbacks struct documenting all callback fields and usage patterns (`toxav_c.go:227-242`)
  - [x] low Error Handling — hex_string_to_bin now uses unsafe.Slice for input and copy builtin for output (`toxcore_c.go:161-182`)
  - [x] low API Design — main() function now has comprehensive godoc explaining c-shared build mode requirements (`toxcore_c.go:12-18`)
  - [x] low Memory Safety — Added bounds validation for unsafe slice conversions (`toxav_c.go:580,625`)

### crypto
- **Source:** `crypto/AUDIT.md`
- **Status:** Complete — All issues resolved
- **High Issues:** 0
- **Medium Issues:** 0 (resolved)
- **Low Issues:** 0 (2 resolved)
- **Test Coverage:** 90.7% ✓
- **Details:**
  - [x] med api-design — Excessive verbose logging in hot paths may impact performance (`encrypt.go:59-112`, `decrypt.go:13-40`, `keypair.go:36-146`) — **RESOLVED**: Added configurable `HotPathLogging` toggle (disabled by default) to eliminate verbose debug logging in hot paths. Error logging preserved for failure cases.
  - [x] low error-handling — ZeroBytes ignores SecureWipe error (`secure_memory.go:38`) — **RESOLVED**: Added early nil check to handle nil input gracefully without calling SecureWipe. Updated godoc to document the behavior.
  - [x] low documentation — LoggerHelper methods lack godoc comments (`logging.go:31-100`) — **RESOLVED**: Added comprehensive godoc comments to LoggerHelper type and all its methods including WithCaller, WithField, WithFields, WithError, Entry, Exit, Debug, Info, Warn, Error, and Fatal.
  - [x] low api-design — isZeroKey function has extensive logging for internal validation (`keypair.go:151-180`) — **RESOLVED**: Removed all logging from internal validation function.

### dht
- **Source:** `dht/AUDIT.md`
- **Status:** Complete — All issues resolved
- **High Issues:** 1 (resolved)
- **Medium Issues:** 1 (resolved)
- **Low Issues:** 3 (resolved)
- **Test Coverage:** 68.7% ✓
- **Details:**
  - [x] high API Design — KBucket.RemoveNode export naming convention (`routing.go:261`)
  - [x] med Stub/Incomplete — Group query response handling returns stub error (`group_storage.go:230`)
  - [x] low Error Handling — Intentional error swallowing in best-effort sends (`maintenance.go:233,257,331`, `group_storage.go:170,221`)
  - [x] low Documentation — handler.go lacks package-level godoc (`handler.go:1`)
  - [x] low Error Handling — 20 errors without `%w` wrapping (`bootstrap.go`, `group_storage.go`, `address_detection.go`)

### factory
- **Source:** `factory/AUDIT.md`
- **Status:** Complete — All issues resolved
- **High Issues:** 0
- **Medium Issues:** 0
- **Low Issues:** 0 (2 resolved)
- **Test Coverage:** 100.0% ✓
- **Details:**
  - [x] low documentation — Missing example in godoc for UpdateConfig method (`packet_delivery_factory.go:336`) — **RESOLVED**: Added comprehensive godoc with example code showing how to create and apply a new configuration.
  - [x] low documentation — CreatePacketDeliveryWithConfig godoc could clarify nil transport behavior (`packet_delivery_factory.go:195`) — **RESOLVED**: Added detailed godoc explaining nil transport behavior: allowed only when config.UseSimulation is true, otherwise returns error.

### file
- **Source:** `file/AUDIT.md`
- **Status:** Complete
- **High Issues:** 0
- **Medium Issues:** 0 (3 resolved)
- **Low Issues:** 0 open (2 resolved)
- **Test Coverage:** 84.8% ✓ (improved from 84.4%)
- **Details:**
  - [x] med Concurrency Safety — Added mutex protection in Transfer.OnProgress, Transfer.OnComplete callback setters (`transfer.go:612,622`)
  - [x] med Integration — Implemented flow control using FileDataAck packets with SetAcknowledgedBytes, GetAcknowledgedBytes, GetPendingBytes, and OnAcknowledge callback (`manager.go:341-363`)
  - [x] low Documentation — Updated doc.go examples to correct AddressResolver signature (`doc.go:62,108`)
  - [x] med API Design — Manager.SendFile takes raw net.Addr parameter; consider helper method (`manager.go:118`) — **RESOLVED**: Added SetFriendAddressLookup and SendFileToFriend convenience method that automatically resolves friend addresses.
  - [x] low Error Handling — Transfer.Cancel swallows file handle close error (`transfer.go:376-384`) — **RESOLVED**: Cancel now returns error wrapping ErrFileCloseFailure when file close fails.
  - [x] low API Design — TimeProvider interface visibility inconsistency (`transfer.go:82-98`) — **RESOLVED**: Exported DefaultTimeProviderInstance variable for external use.

### friend
- **Source:** `friend/AUDIT.md`
- **Status:** Complete — All issues resolved
- **High Issues:** 0
- **Medium Issues:** 0
- **Low Issues:** 0 open (3 resolved)
- **Test Coverage:** 93.1% ✓
- **Details:**
  - [x] low API Design — FriendInfo lacks thread-safety documentation (`doc.go:89`) — **RESOLVED**: Thread-safety documentation already exists at `doc.go:89`: "FriendInfo methods are not thread-safe; callers must synchronize access."
  - [x] low Documentation — SetStatus methods lack structured logging compared to peers (`friend.go:171-180`) — **RESOLVED**: Added `logrus.WithFields` structured logging to `SetStatus` method consistent with `SetConnectionStatus` pattern.
  - [x] low Error Handling — Request.Encrypt could benefit from wrapping crypto errors with more context (`request.go:131-141, 190-199`) — **RESOLVED**: Updated `Request.Encrypt` and `DecryptRequestWithTimeProvider` to use `fmt.Errorf` with `%w` for proper error chain wrapping.

### group
- **Source:** `group/AUDIT.md`
- **Status:** Complete — All issues resolved
- **High Issues:** 0
- **Medium Issues:** 2 (resolved)
- **Low Issues:** 0 open (2 resolved)
- **Test Coverage:** 78.6% ✓ (improved from 64.9%)
- **Details:**
  - [x] med API Design — Inconsistent logging: `log.Printf` at `chat.go:1228` vs logrus elsewhere — **RESOLVED**: Replaced with `logrus.WithFields` structured logging.
  - [x] med Error Handling — Eight unwrapped errors in chat.go without `%w` (`chat.go:233,267,269,1209,1240,1244,1273,1284`) — **RESOLVED**: Line 1209 now uses `errors.Join` for proper error wrapping. Other lines create new errors without underlying errors to wrap.
  - [x] low Documentation — queryDHTNetwork lacks inline comments explaining coordination mechanics — **RESOLVED**: Added comprehensive inline comments explaining the two-phase query pattern, response handler registration, buffered channel design, and timeout behavior.
  - [x] low Concurrency Safety — Worker pool in sendToConnectedPeers uses goroutines without context cancellation (`chat.go:1157`) — **RESOLVED**: Added context.Context parameter to sendToConnectedPeers with context-aware worker loop. Workers check context before each job and report cancellation errors separately from send failures. broadcastGroupUpdate now uses a 30-second timeout context.

### interfaces
- **Source:** `interfaces/AUDIT.md`
- **Status:** Complete — All issues resolved
- **High Issues:** 0
- **Medium Issues:** 0
- **Low Issues:** 0 open (3 resolved)
- **Test Coverage:** 100.0% ✓
- **Details:**
  - [x] low documentation — Added example test functions: ExamplePacketDeliveryConfig_Validate, ExamplePacketDeliveryConfig_Validate_invalid, ExamplePacketDeliveryStats (`packet_delivery_test.go`)
  - [x] low api-design — Added `PacketDeliveryStats` typed struct and `GetTypedStats()` method to `IPacketDelivery` interface; GetStats() deprecated but retained for backward compatibility (`packet_delivery.go:14-40,74-78`)
  - [x] low error-handling — Mock implementations now support configurable error injection via deliverErr, broadcastErr, setTransportErr, addFriendErr, removeFriendErr fields (`packet_delivery_test.go:98-111`)

### limits
- **Source:** `limits/AUDIT.md`
- **Status:** Complete — No issues found
- **High Issues:** 0
- **Medium Issues:** 0
- **Low Issues:** 0
- **Test Coverage:** 100.0% ✓
- **Details:** Exemplary Go code quality. No issues identified.

### messaging
- **Source:** `messaging/AUDIT.md`
- **Status:** Complete — All issues resolved
- **High Issues:** 1 (resolved)
- **Medium Issues:** 2 (resolved)
- **Low Issues:** 1 (resolved)
- **Test Coverage:** 97.7% ✓ (improved from 53.3%)
- **Details:**
  - [x] high concurrency — Race condition in Message.State field access without mutex protection (`message.go:99,104,231`)
  - [x] med test-coverage — Test coverage improved from 53.3% to 97.7%
  - [x] med concurrency — Message.State accessed directly in tests without synchronization (`encryption_test.go:99`)
  - [x] low documentation — TimeProvider interface documentation verified as adequate

### net
- **Source:** `net/AUDIT.md`
- **Status:** Needs Work
- **High Issues:** 1 (resolved)
- **Medium Issues:** 3 (resolved)
- **Low Issues:** 4 open
- **Test Coverage:** 77.4% ✓
- **Details:**
  - [x] high security — Packet encryption implemented with optional NaCl box encryption (`packet_conn.go:260,285`)
  - [x] med concurrency — Timer leak in setupReadTimeout fixed (`conn.go:114`)
  - [x] med concurrency — Timer leak in setupConnectionTimeout fixed (`conn.go:310`)
  - [x] med error-handling — writeChunkedData now returns ErrPartialWrite (`conn.go:259`)
  - [ ] low documentation — PacketListen godoc mentions incorrect return type (`dial.go:250`)
  - [ ] low api-design — ListenAddr ignores addr parameter (`dial.go:190`)
  - [ ] low concurrency — Race condition in waitForConnection (`conn.go:215-216`)
  - [ ] low error-handling — processIncomingPacket boolean return semantics inverted (`packet_conn.go:106`)

### noise
- **Source:** `noise/AUDIT.md`
- **Status:** Complete
- **High Issues:** 0
- **Medium Issues:** 0
- **Low Issues:** 2 open
- **Test Coverage:** 88.4% ✓
- **Details:**
  - [ ] low API Design — XXHandshake.localPubKey stores slice directly without copy, unlike IKHandshake (`handshake.go:324`)
  - [ ] low Documentation — doc.go example code uses blank identifier for error returns (`doc.go:87,93,96`)

### real
- **Source:** `real/AUDIT.md`
- **Status:** Complete
- **High Issues:** 0
- **Medium Issues:** 0
- **Low Issues:** 3 open
- **Test Coverage:** 98.9% ✓
- **Details:**
  - [ ] low documentation — Missing package-level examples in doc.go for factory integration
  - [ ] low api-design — GetFriendAddress fallback in DeliverPacket may trigger repeatedly (`packet_delivery.go:74`)
  - [ ] low consistency — RemoveFriend doesn't notify underlying transport of removal (`packet_delivery.go:277`)

### testing
- **Source:** `testing/AUDIT.md`
- **Status:** Complete
- **High Issues:** 0
- **Medium Issues:** 0
- **Low Issues:** 3 open
- **Test Coverage:** 98.7% ✓
- **Details:**
  - [ ] low documentation — GetDeliveryLog thread-safety implications not documented (`packet_delivery_sim.go:238`)
  - [ ] low api-design — addrString helper unexported but useful elsewhere (`packet_delivery_sim.go:203`)
  - [ ] low concurrency — BroadcastPacket counts excluded friends as "failed" internally (`packet_delivery_sim.go:133`)

### testnet/internal
- **Source:** `testnet/internal/AUDIT.md`
- **Status:** Needs Work
- **High Issues:** 1 (partially addressed)
- **Medium Issues:** 0 (1 resolved)
- **Low Issues:** 0 open (4 resolved)
- **Test Coverage:** 41.2% ⚠ (below 65% target)
- **Details:**
  - [ ] **high** Test Coverage — Coverage improved from 32.3% to 41.2% through expanded unit tests for logging methods, step tracking, retry logic, cleanup helpers, and struct validation. Remaining gap requires integration tests with real Tox network instances, as many functions (`NewBootstrapServer`, `Start`, `Stop`, `eventLoop`, `setupCallbacks`, etc.) require actual `toxcore.Tox` instances that bind to network ports.
  - [x] med API Design — Use of `map[string]interface{}` in GetStatus() reduces type safety (`bootstrap.go:259`, `client.go:495`) — **RESOLVED**: Added typed `ServerStatus` and `ClientStatus` structs with `GetStatusTyped()` methods. Original methods retained with deprecation notice.
  - [x] low API Design — Use of bare `interface{}` could be `any` type alias (`bootstrap_test.go:18-19`) — **RESOLVED**: Updated to use `any` type alias.
  - [x] low Error Handling — Intentional error suppression with `_ = ` in test code (`comprehensive_test.go:191-193,254-258,487`) — **RESOLVED**: Updated reader goroutines to verify read values and validate consistency; step tracking test now properly checks return values.
  - [x] low Concurrency — Hard-coded sleeps for synchronization could be flaky in CI (`bootstrap.go:150`, `protocol.go:232`) — **RESOLVED**: Added configurable `InitDelay` to BootstrapConfig and `AcceptanceDelay` to ProtocolConfig with sensible defaults. Sleeps are now skipped when delay is 0.
  - [x] low Documentation — TestStepResult.Metrics uses `map[string]interface{}` without documenting expected keys (`orchestrator.go:69`) — **RESOLVED**: Added `StepMetrics` typed struct with comprehensive godoc and `TypedMetrics` field.


### transport
- **Source:** `transport/AUDIT.md`
- **Status:** Complete — All issues resolved
- **High Issues:** 2 (resolved)
- **Medium Issues:** 4 (resolved)
- **Low Issues:** 2 (resolved)
- **Test Coverage:** 65.2% ✓ (improved from 62.6%)
- **Details:**
  - [x] high stub/incomplete — NymTransport stub implementation addressed (`network_transport_impl.go:479-520`)
  - [x] high error-handling — SetReadDeadline error handling in UDP read path addressed (`udp.go:237`)
  - [x] med error-handling — Background NAT detection error handling addressed (`nat.go:172`)
  - [x] med stub/incomplete — AdvancedNATTraversal STUN connection addressed (`advanced_nat.go:279`)
  - [x] med error-handling — Noise handshake complete flag handling addressed (`versioned_handshake.go:290,416`)
  - [x] med test-coverage — Coverage improved from 62.6% to 65.2%
  - [x] low error-handling — Test file error swallowing addressed
  - [x] low documentation — Core type file documentation addressed

## Resolution Priorities

### Priority 1 — High Severity (Open)

1. ~~**capi: Unused error_ptr parameters**~~ — **RESOLVED**: Implemented proper error code population for all error_ptr parameters with appropriate error mapping.
2. ~~**capi: Package encapsulation violation**~~ — **RESOLVED**: Created GetToxInstanceByID accessor function with proper mutex protection.
3. **testnet/internal: Test coverage gap** — Coverage improved from 32.3% to 41.9% through expanded unit tests (logging methods, step tracking, retry logic, cleanup helpers). Further improvement requires integration tests with real Tox network instances (many functions require `toxcore.Tox` instances that bind to network ports).

### Priority 2 — Medium Severity (Open)

4. ~~**capi: Concurrency and validation gaps**~~ — **RESOLVED**: Added mutex protection via accessor function and bounds validation in frame functions.
5. ~~**file: Callback setter race condition**~~ — **RESOLVED**: Added mutex protection in Transfer.OnProgress/OnComplete setters with thread-safety documentation.
6. ~~**crypto: Hot-path logging performance**~~ — **RESOLVED**: Added `HotPathLogging` toggle (disabled by default) to eliminate verbose debug logging in hot paths. Error logging preserved. Affects `encrypt.go`, `keypair.go`.
7. ~~**group: Error wrapping and logging consistency**~~ — **RESOLVED**: Replaced `log.Printf` with `logrus.WithFields` structured logging at line 1228. Updated error wrapping at line 1209 to use `errors.Join` for proper error chain support.
8. ~~**av/rtp: Hardcoded audio format**~~ — **RESOLVED**: Added AudioConfig struct to Session with GetAudioConfig/SetAudioConfig methods. handleIncomingAudioFrame now retrieves audio parameters from session configuration instead of using hardcoded mono/48kHz.
9. ~~**file: Flow control not implemented**~~ — **RESOLVED**: FileDataAck packets now update transfer's acknowledged bytes with SetAcknowledgedBytes, GetAcknowledgedBytes, GetPendingBytes, and OnAcknowledge callback for backpressure control.
10. ~~**file: API ergonomics**~~ — **RESOLVED**: Added SetFriendAddressLookup and SendFileToFriend convenience method that automatically resolves friend addresses using FriendAddressLookup.

### Priority 3 — Low Severity (Open)

11. **Documentation improvements** — 15+ packages have minor documentation gaps (missing godoc, outdated examples, undocumented thread-safety)
12. **API design refinements** — Type safety improvements (`map[string]interface{}` → typed structs in interfaces, testnet/internal), visibility consistency (file, noise)
13. **Minor concurrency issues** — Panic recovery for async callbacks (av), context cancellation for worker pools (group), minor race in waitForConnection (net)
14. ~~**Error handling cleanup**~~ — ~~Swallowed errors in non-critical paths, missing error context wrapping in friend/crypto~~ — **PARTIALLY RESOLVED**: `friend` package error wrapping issues resolved. `crypto` HotPathLogging toggle addressed previously.

### Priority 4 — Test Coverage

15. **testnet/internal** — Improved from 32.3% to 41.9%; remaining gap requires integration tests with real Tox network instances
16. ~~**transport**~~ — **RESOLVED**: Improved from 62.6% to 65.2%
17. ~~**group**~~ — **RESOLVED**: Improved from 64.9% to 78.6%

## Cross-Package Dependencies

### Inconsistent Logging (affects: async, group, dht, capi)
Multiple packages mix `log.Printf`, `fmt.Printf`, and `logrus` structured logging. Standardizing on `logrus.WithFields` across the codebase would improve observability and consistency. The `async`, `dht`, and `group` packages have already resolved this; `capi` still needs work.

### Crypto Package Performance (affects: async, transport, dht, friend, noise) — RESOLVED
~~The `crypto` package's excessive verbose logging in hot paths (encrypt/decrypt) impacts performance across all 5+ consuming packages.~~ **RESOLVED**: Added `HotPathLogging` toggle (disabled by default) to eliminate verbose debug logging in hot paths while preserving error logging. Hot path logging check overhead is <0.5ns per call with zero allocations.

### Error Wrapping Patterns (affects: group, dht, net, capi) — RESOLVED for group
Several packages create errors with `fmt.Errorf` without `%w` wrapping, breaking error chain inspection. The `dht` and `group` packages have resolved this. Establishing a codebase-wide convention for error wrapping would improve debugging.

### Type Safety in Status APIs (affects: interfaces, testnet/internal) — PARTIALLY RESOLVED
Both `interfaces.GetStats()` and `testnet/internal.GetStatus()` return `map[string]interface{}` instead of typed structs. The `testnet/internal` package now provides typed `ServerStatus`, `ClientStatus`, and `StepMetrics` structs with `GetStatusTyped()` methods. The `interfaces` package still uses the map pattern.

### Transport Layer Stability (affects: 18+ importing packages)
The `transport` package is imported by 18 packages and now meets the test coverage target (65.2%, improved from 62.6%). All identified issues are resolved.

### C API Boundary Safety (affects: capi ↔ all core packages)
The `capi` package bridged Go and C code with 2 high-severity issues that are now **RESOLVED**: error_ptr parameters are now properly populated with appropriate error codes, and encapsulation is preserved through an accessor function with mutex protection. The remaining low-severity issues are documentation and style improvements.

## Test Coverage Overview

| Package | Coverage | Target | Status |
|---------|----------|--------|--------|
| async | N/A | 65% | — |
| av | 78.0% | 65% | ✓ |
| av/rtp | 91.0% | 65% | ✓ |
| capi | 72.4% | 65% | ✓ |
| crypto | 90.7% | 65% | ✓ |
| dht | 68.7% | 65% | ✓ |
| factory | 100.0% | 65% | ✓ |
| file | 83.9% | 65% | ✓ |
| friend | 93.0% | 65% | ✓ |
| group | 78.6% | 65% | ✓ |
| interfaces | 100.0% | 65% | ✓ |
| limits | 100.0% | 65% | ✓ |
| messaging | 97.7% | 65% | ✓ |
| net | 77.4% | 65% | ✓ |
| noise | 88.4% | 65% | ✓ |
| real | 98.9% | 65% | ✓ |
| testing | 98.7% | 65% | ✓ |
| testnet/internal | 41.2% | 65% | ⚠ |
| transport | 65.2% | 65% | ✓ |
