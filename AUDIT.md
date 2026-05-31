# Implementation Audit Report
Generated: 2026-05-31T00:00:00Z
Sources: PLAN.md, Go codebase, docs/PROTOCOL_SPEC.md

## Executive Summary
- Total PLAN feature groups audited: 18 (1.1 through 5.4)
- Status breakdown:
  - Complete (implemented + documented): 13/18 (72.2%)
  - Partial (implemented but incomplete scope or missing integration): 3/18 (16.7%)
  - Missing/Blocked (not implementable fully in-repo): 2/18 (11.1%)
- Critical findings (highest priority):
  1. Rekey threshold mismatch: PLAN 1.2 said lowered threshold; code still used half-uint64.
  2. Manual-accept option naming gap: behavior existed, explicit `WithManualAccept()` API was absent.
  3. C ABI security-critical exports gap: no C ABI functions for safety number, secure wipe, and key generation.
- Estimated remediation effort:
  - Completed in this cycle: 3 code remediations + 3 doc/test updates
  - Remaining effort: medium (disappearing-message end-to-end integration + optional API hardening + external/process items)

## Audit Method
- Read PLAN.md in full and mapped each top-level checklist group (1.1..5.4) to implementation evidence.
- Cross-referenced docs/PROTOCOL_SPEC.md values and behavior descriptions against code constants and flows.
- Verified each finding against concrete symbols/functions in source and tests.

## Detailed Findings

### Fully Implemented and Documented

1) 1.1 Double Ratchet for live chat
- PLAN.md Reference: Phase 1, item 1.1
- Code Location:
  - ratchet/session.go
  - ratchet/doc.go
  - messaging/message.go (ratchet integration path)
  - ratchet/ratchet_test.go
- Spec Location: docs/PROTOCOL_SPEC.md (live-session encryption and forward-secrecy sections)
- Status: Complete
- Notes: Includes skipped-key store, per-message key derivation, DH ratchet tests, and integration fallback behavior.

2) 1.2 Re-handshake interim protection
- PLAN.md Reference: Phase 1, item 1.2
- Code Location:
  - transport/noise_transport.go (`DefaultRekeyThreshold`, duration and idle triggers)
  - transport/noise_transport_test.go
- Spec Location: docs/PROTOCOL_SPEC.md §3.2
- Status: Complete (after remediation)
- Notes: Threshold set to 500 messages; time-based triggers already existed.

3) 2.1 Safety Number primitive
- PLAN.md Reference: Phase 2, item 2.1
- Code Location:
  - crypto/safety_number.go
  - crypto/safety_number_test.go
  - toxcore_self.go (`Tox.SafetyNumber`)
  - toxnet/safety_number.go
- Spec Location: docs/PROTOCOL_SPEC.md security sections
- Status: Complete
- Notes: Known-answer tests present.

4) 2.2 Friend-request MITM hardening (core behavior)
- PLAN.md Reference: Phase 2, item 2.2
- Code Location:
  - toxnet/dial.go (`Listen` defaults to manual accept)
  - toxnet/listener.go (manual handler with precomputed safety number)
  - toxnet/dial_test.go
- Spec Location: behavior reflected in library docs and flow text
- Status: Complete (includes example flow)

5) 2.3 Signed pre-key bundle (X3DH parity)
- PLAN.md Reference: Phase 2, item 2.3
- Code Location:
  - async/prekeys.go (`SignedPreKey`)
  - async/forward_secrecy.go (`PreKeyExchangeMessage` signed key field)
  - async/manager.go (packet signature verification)
  - async/prekey_signature_test.go
- Spec Location: docs/PROTOCOL_SPEC.md async message structures and security notes
- Status: Complete

6) 3.2 Pre-key exhaustion resilience
- PLAN.md Reference: Phase 3, item 3.2
- Code Location:
  - async/forward_secrecy.go (`PreKeyMinimum=20`, low watermark callback, rate limiting)
  - async/prekeys.go (`PreKeysPerPeer=200`, `PreKeyRefreshThreshold=50`)
- Spec Location: docs/PROTOCOL_SPEC.md §3.5
- Status: Complete

7) 3.3 Padding alignment and bounds
- PLAN.md Reference: Phase 3, item 3.3
- Code Location:
  - messaging/message.go (`PaddingSizes` includes 16384; 4-byte length prefix; oversize reject)
  - messaging/validation_test.go
- Spec Location: docs/PROTOCOL_SPEC.md message/padding sections
- Status: Complete

8) 4.3 Go memory-hardening with mlock fallback
- PLAN.md Reference: Phase 4, item 4.3
- Code Location:
  - crypto/secure_memory.go (`SecureAllocate`, `MlockAvailable`)
  - crypto/secure_alloc_cgo.go
  - crypto/secure_alloc_nocgo.go
- Spec/Policy Location:
  - SECURITY.md swap/mlock notes
- Status: Complete

9) 4.4 Vulnerability disclosure policy
- PLAN.md Reference: Phase 4, item 4.4
- Code Location: N/A
- Spec/Policy Location: SECURITY.md
- Status: Complete

10) 4.5 Protocol-level test vectors
- PLAN.md Reference: Phase 4, item 4.5
- Code Location:
  - crypto/testvectors_test.go
  - async/testvectors_test.go
- Spec Location: docs/PROTOCOL_SPEC.md audit and testing references
- Status: Complete

11) 5.3 Pre-key pool size and backup/restore
- PLAN.md Reference: Phase 5, item 5.3
- Code Location:
  - async/prekeys.go (`PreKeysPerPeer=200`, `PreKeyRefreshThreshold=50`)
  - async/prekeys.go (`PreKeyBackup`, `ExportPreKeys`, import/restore path)
- Spec Location: docs/PROTOCOL_SPEC.md §3.5
- Status: Complete

12) 5.4 Live-session cover traffic
- PLAN.md Reference: Phase 5, item 5.4
- Code Location:
  - transport/cover_traffic.go
  - transport/noise_transport.go (integration)
- Spec Location: docs/PROTOCOL_SPEC.md metadata protection and packet types
- Status: Complete

13) 5.2 Key rotation default and emergency rotation APIs
- PLAN.md Reference: Phase 5, item 5.2
- Code Location:
  - crypto/key_rotation.go (`RotationPeriod=7d`, `EmergencyRotation`)
  - async/key_rotation_client.go (`EmergencyRotateIdentity`, `SetKeyRotationCallback`)
- Spec Location: security/key-management notes
- Status: Complete

### Partially Implemented

1) 1.3 Mandatory obfuscated async transport
- PLAN.md Reference: Phase 1, item 1.3
- Code Location:
  - async/forward_secrecy.go (deprecation + guard)
  - async/client.go / async/manager.go
- What Exists:
  - `ForwardSecureMessage` is deprecated.
  - `SendForwardSecureMessageDirect` guard returns `ErrMustUseObfuscatedTransport`.
- What Is Missing:
  - `ForwardSecureMessage` remains public and still appears in public method signatures.
- Spec Status: Documented with deprecation note.
- Impact: Residual API-footgun risk, though mitigated by guard and docs.

2) 3.1 Disappearing messages (end-to-end)
- PLAN.md Reference: Phase 3, item 3.1
- Code Location:
  - messaging/disappearing.go
  - messaging/message.go
  - toxcore.go friend struct field
- What Exists:
  - Data model and per-conversation timer manager.
- What Is Missing:
  - No complete end-to-end control-message synchronization path in toxcore message handling.
  - No explicit receive-path storage deletion integration for inbound messages at toxcore facade level.
- Spec Status: Partially reflected conceptually; behavior-level details remain sparse.
- Impact: Feature is not fully consumable as a protocol-level interoperable capability.

3) 5.1 Mobile parity (multi-language bindings)
- PLAN.md Reference: Phase 5, item 5.1
- Code Location:
  - capi/toxcore_c.go
- What Exists:
  - C ABI foundation exists; security-critical exports now present.
- What Is Missing:
  - Separate Swift/Kotlin wrapper repositories and published SDK releases.
- Spec Status: Not a protocol-wire requirement; ecosystem/process item.
- Impact: Mobile parity remains incomplete.

### Not Implemented / Blocked in Current Repository Scope

1) 4.1 Third-party cryptographic audit execution and publication
- PLAN.md Reference: Phase 4, item 4.1
- Expected Location: external engagement + report publication artifacts
- Impact: Independent assurance gap remains until external audit is performed.
- Dependencies: external vendor engagement.

2) 4.2 Academic submission of standalone spec and formal proofs
- PLAN.md Reference: Phase 4, item 4.2
- Expected Location: external paper/preprint workflow
- Impact: Community peer-review process not complete.
- Dependencies: external collaboration/review cycle.

### Undocumented Features (Before Remediation)

1) Lowered rekey threshold (implemented in code during remediation)
- Feature: Interim rekey hard limit set to 500 messages.
- Code Location: transport/noise_transport.go
- Spec Gap: PROTOCOL_SPEC still described half-uint64 threshold.
- PLAN Status: Marked complete.

2) C ABI security primitives (implemented in code during remediation)
- Feature: C exports for key generation, secure wipe, safety number.
- Code Location: capi/toxcore_c.go
- Spec Gap: Not listed in protocol doc changelog before remediation.
- PLAN Status: 5.1 third sub-item was unchecked.

### Incorrect Implementations / Mismatches (Before Remediation)

1) 1.2 Rekey threshold mismatch
- PLAN.md Reference: item 1.2 says lower threshold ~100-500.
- Code Location: transport/noise_transport.go
- Expected Behavior: low threshold + existing time-based triggers.
- Actual Behavior: threshold was half-uint64.
- Discrepancy: plan and code diverged.
- Spec Status: matched old code, not plan.

2) 2.2 Option API naming/contract mismatch
- PLAN.md Reference: item 2.2 requests `WithManualAccept()` option.
- Code Location: toxnet/listener.go, toxnet/dial.go
- Expected Behavior: explicit option present.
- Actual Behavior: default behavior was manual accept, but explicit method absent.
- Discrepancy: intent present, requested API shape missing.
- Spec Status: behavior documented but option name not exposed.

## Remediation Checklist

Total Items: 9
Priority Breakdown: P0: 2, P1: 3, P2: 2, P3: 2

### P0: Critical Priority

[x] REM-001: Align Noise rekey threshold with PLAN 1.2
Type: Fix
PLAN.md Item: 1.2
Action Required: Lower default threshold from nonce-limit proxy to interim compatibility threshold.
Files Modified:
- transport/noise_transport.go - set `DefaultRekeyThreshold = 500` and clarify temporary rationale.
- docs/PROTOCOL_SPEC.md - update rekey table and audit notes.
Implementation Details:
- Keep existing time-based triggers (`DefaultRekeyInterval`, `DefaultRekeyIdleTimeout`).
- Preserve configurable threshold behavior via session-level override.
Acceptance Criteria:
- [x] `DefaultRekeyThreshold` equals 500.
- [x] Time-based triggers still active.
- [x] Spec reflects code values.
Dependencies: none
Estimated Complexity: Low
Status: COMPLETE

[x] REM-002: Add explicit `WithManualAccept()` option API
Type: Implementation
PLAN.md Item: 2.2
Action Required: Provide explicit listener option while preserving manual-accept default.
Files Modified:
- toxnet/listener.go - add `WithManualAccept() *ToxListener`.
- toxnet/dial_test.go - add coverage.
- toxnet/README.md - update API docs.
Implementation Details:
- `WithManualAccept()` sets `autoAccept=false` and returns same listener pointer for fluent chaining.
Acceptance Criteria:
- [x] Option method exists.
- [x] Test verifies behavior and fluent return.
- [x] Docs list method.
Dependencies: none
Estimated Complexity: Low
Status: COMPLETE

### P1: High Priority

[x] REM-003: Export security-critical C ABI operations
Type: Implementation
PLAN.md Item: 5.1 (third sub-item)
Action Required: Expose safety number, key generation, and secure wipe through C ABI.
Files Modified:
- capi/toxcore_c.go
- capi/toxcore_c_test.go
- PLAN.md
- docs/PROTOCOL_SPEC.md
Implementation Details:
- Added `tox_crypto_generate_keypair`, `tox_crypto_secure_wipe`, `tox_self_get_safety_number`.
- Added tests for all new exports.
Acceptance Criteria:
- [x] Three required exports implemented.
- [x] Unit tests cover basic behavior.
- [x] PLAN updated to implemented status for this sub-item.
Dependencies: none
Estimated Complexity: Medium
Status: COMPLETE

[ ] REM-004: Finish disappearing-message protocol integration
Type: Implementation
PLAN.md Item: 3.1
Action Required: Wire control-message sync and receive-path deletion semantics through toxcore facade and callbacks/storage hooks.
Files to Modify:
- toxcore_messaging.go
- toxcore_callbacks.go
- messaging/message.go
- docs/PROTOCOL_SPEC.md
Implementation Details:
- Add explicit setter/getter API for per-friend config.
- Send/receive `MessageTypeDisappearingConfig` payloads with conflict resolution by `SetAt`.
- Define local-storage deletion contract for inbound messages.
Acceptance Criteria:
- [ ] Config changes sync to peer over control message.
- [ ] Inbound/outbound message deletion behavior deterministic and tested.
- [ ] Restart/offline edge cases covered by tests.
Dependencies: none
Estimated Complexity: High
Status: OPEN

[ ] REM-005: Reduce `ForwardSecureMessage` API exposure
Type: Fix
PLAN.md Item: 1.3
Action Required: Further limit public API reliance on `ForwardSecureMessage`.
Files to Modify:
- async/client.go
- async/forward_secrecy.go
- docs/PROTOCOL_SPEC.md
Implementation Details:
- Keep backward compatibility, but provide stricter API paths and stronger compiler/runtime guidance.
Acceptance Criteria:
- [ ] Public docs and signatures default to obfuscated-only flow.
- [ ] Direct non-obfuscated send path impossible or hard-failed.
Dependencies: none
Estimated Complexity: Medium
Status: OPEN

### P2: Medium Priority

[ ] REM-007: Expose emergency key rotation at top-level `toxcore.Tox`
Type: Implementation
PLAN.md Item: 5.2
Action Required: Add facade-level API that app developers can call directly.
Files to Modify:
- toxcore.go / toxcore_lifecycle.go / toxcore_self.go (as appropriate)
- docs/PROTOCOL_SPEC.md
Acceptance Criteria:
- [ ] Public Tox API includes emergency rotation entrypoint.
- [ ] Callback path for session renegotiation documented.
Dependencies: none
Estimated Complexity: Medium
Status: OPEN

[ ] REM-008: Mobile wrapper publication scaffolding
Type: Ecosystem
PLAN.md Item: 5.1 (first two sub-items)
Action Required: Define interface boundaries and repository split for Swift/Kotlin wrappers.
Files to Modify:
- docs/ (design note)
Acceptance Criteria:
- [ ] Interface contract documented.
- [ ] External repository plan and ownership defined.
Dependencies: REM-003
Estimated Complexity: Medium
Status: BLOCKED (external repos)

### P3: Low Priority

[ ] REM-009: Third-party audit execution tracking
Type: Process
PLAN.md Item: 4.1
Action Required: Track firm engagement, report publication, and remediation status.
Files to Modify:
- SECURITY.md
- docs/AUDIT_EXTERNAL_<FIRM>_<YEAR>.md (future)
Acceptance Criteria:
- [ ] External audit contract executed.
- [ ] Report published.
Dependencies: External
Estimated Complexity: High
Status: BLOCKED (external)

[ ] REM-010: Academic protocol publication workflow
Type: Process/Documentation
PLAN.md Item: 4.2
Action Required: Submit standalone spec and gather external review.
Files to Modify:
- docs/PROTOCOL_SPEC.md
- external venue artifacts
Acceptance Criteria:
- [ ] Submission completed.
- [ ] Feedback cycle tracked.
Dependencies: External
Estimated Complexity: High
Status: BLOCKED (external)

## Implementation Specifications

### Transport Rekeying Hardening
Overview: Ensure interim forward-secrecy blast-radius control prior to universal ratchet coverage.

Technical Requirements:
- Counter-based rekey threshold in 100-500 range.
- Time-based and idle-based forced rekey remain active.

Code Structure:
- transport/noise_transport.go constants remain the source of truth.
- Session-level override support remains unchanged.

Documentation Requirements:
- PROTOCOL_SPEC §3.2 table and audit notes must match code constants.

### Friend-Request Manual Acceptance
Overview: Enforce safer defaults and explicit API for manual verification flow.

Technical Requirements:
- Manual accept is default.
- Explicit option method available for discoverability and policy control.

Code Structure:
- toxnet/listener.go for API
- toxnet/dial.go for construction defaults
- toxnet tests for behavior

Documentation Requirements:
- toxnet README API block updated.

### C ABI Security Operations
Overview: Export security-critical primitives so wrappers do not reimplement crypto.

Technical Requirements:
- C ABI exports for key generation, secure wipe, and safety number.
- Input validation and deterministic return codes.

Code Structure:
- capi/toxcore_c.go exports
- capi/toxcore_c_test.go tests

Documentation Requirements:
- PROTOCOL_SPEC changelog tracks export additions.
- PLAN item status updated.

## Verification Criteria

After remediation, the following are true for completed items:

Code Completeness:
- [x] PLAN 1.2 threshold mismatch resolved.
- [x] PLAN 2.2 option API gap resolved.
- [x] PLAN 5.1 security-critical C ABI sub-item implemented.
- [ ] Remaining open/blocked items listed explicitly (not silently assumed complete).

Documentation Completeness:
- [x] PROTOCOL_SPEC rekey constants updated.
- [x] PROTOCOL_SPEC changelog includes new remediations.
- [x] toxnet README reflects current listener security behavior.

Consistency:
- [x] No contradiction between updated transport constants and spec table.
- [x] PLAN checklist updated for implemented C ABI sub-item.
- [ ] External/process items still require human/external completion.

Quality Standards:
- [x] Added/updated unit tests for new toxnet and capi APIs.
- [x] Error handling in new C ABI functions is explicit.
- [ ] Remaining open items need follow-on tests once implemented.

## Appendix: File Inventory (Audit Focus)

Go Source Areas Analyzed:
- transport/noise_transport.go
- messaging/message.go
- messaging/disappearing.go
- ratchet/*.go
- async/*.go (forward secrecy, prekeys, obfuscation, storage)
- toxcore_messaging.go, toxcore_self.go, toxcore.go
- toxnet/listener.go, toxnet/dial.go, toxnet/safety_number.go
- capi/toxcore_c.go

Test Files Analyzed (Representative):
- ratchet/ratchet_test.go
- async/prekey_signature_test.go
- async/prekey_hmac_security_test.go
- async/testvectors_test.go
- messaging/disappearing_test.go
- transport/noise_transport_test.go
- toxnet/dial_test.go
- capi/toxcore_c_test.go

Documentation Files:
- PLAN.md
- docs/PROTOCOL_SPEC.md
- SECURITY.md
- toxnet/README.md
