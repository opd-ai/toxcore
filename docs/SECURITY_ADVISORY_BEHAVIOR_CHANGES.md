# Security Advisory: toxcore-go v1.4.0 Behavior Changes

## Overview

This advisory documents significant security hardening changes in toxcore-go v1.4.0, including new defaults for encryption, post-compromise security, and protocol negotiation behavior.

**Impact Level:** Applications relying on legacy fallback behavior may require updates. All changes maintain backward compatibility through negotiation and protocol downgrade paths.

---

## 1. Encryption Invariant Enforcement (Priority 1.1)

### Change
By default, toxcore-go now rejects outbound friend messages when encryption prerequisites (secure session, valid keys) are unavailable. Previously, some integration scenarios could result in plaintext transmission as a silent fallback.

### Affected APIs
- `Tox.SendFriendMessage()` — now returns `ErrSendPlaintextNotAllowed` if secure session unavailable
- All messaging paths enforce end-to-end encryption by default

### Migration Guide
**For applications expecting insecure fallback:** This behavior is no longer supported. Ensure:
1. Friend authentication is complete before sending messages
2. Session establishment succeeds before message transmission
3. Error handling distinguishes between transient (network) and fatal (security) failures

**For applications enforcing encryption by default:** No changes required.

### Compatibility
- **Legacy peers:** Continue to work through encrypted legacy transport (NaCl-box), not plaintext
- **Noise-IK peers:** Full support
- **Noise-IK + Ratchet peers:** Full support

---

## 2. Post-Compromise Security Defaults (Priority 1.2)

### Change
When both peers support it, `noise+ratchet` is now the automatic default for new sessions. Previously, ratcheting was optional or required explicit configuration.

### Behavior
1. **Session Negotiation:** `SelectSessionMode()` automatically chooses `SessionModeNoiseWithRatchet` when both peers advertise `RatchetSupported`
2. **Fallback Chain:** `noise+ratchet` → `ProtocolNoiseIK` → `ProtocolLegacy` (automatic, no user toggle)
3. **Ratchet State:** Authenticated bootstrap via transport identity; tracked per-session with key deletion limits

### Affected Applications
- **Forward-secrecy-sensitive:** Sessions now have per-message key rotation
- **High-throughput messaging:** Ratcheting adds ~10-15% overhead (benchmarked); can be profiled via `PROFILING.md`

### Migration Guide
**For applications needing legacy behavior:** Configure `PolicyLegacyOnly` in `NewToxOptions()`:
```go
opts := &ToxOptions{
    // ...
    SecurityPolicy: transport.PolicyLegacyOnly,
}
```

**For applications accepting new defaults:** No action required.

### Compatibility
- **Noise-only peers:** Fall back to `SessionModeNoise` (no ratchet)
- **Legacy-only peers:** Fall back to `SessionModeLegacy` (no Noise)
- **Mixed-capability peers:** Negotiate highest mutually supported mode

---

## 3. Trust Establishment and MITM Resistance (Priority 1.3)

### Changes

#### Signature Validation on Version Negotiation
Version negotiation packets are now signed when both peers support it, preventing active downgrade attacks.

**New Error:** `ErrSignatureVerificationFailed` (fatal) — explicit, observable, logged

#### TOFU State Machine
- Peers are tracked with key-change detection
- Out-of-band safety-number verification helpers available via `crypto.SafetyNumber()`
- Application callbacks triggered on key changes: `SetKeyChangeCallback()`

#### Affected APIs
- `VersionNegotiationOptions.RequireSignedNegotiation` — default `true` when both peers capable
- `Tox.VerifyPeerSafetyNumber()` — new API for safety-number verification
- `OnKeyChangeCallback` — invoked when peer identity changes

### Migration Guide
**For applications with out-of-band identity verification:** Wire up `SetKeyChangeCallback()` to handle re-authentication.

**For applications accepting TOFU model:** Rely on protocol-level signature verification (new default).

### Compatibility
- **Unsigned negotiation:** Allowed only if peer lacks signature support or `RequireSignedNegotiation=false`
- **Explicit downgrade:** Logged as `DowngradeEvent` with reason; observable via callbacks

---

## 4. Metadata Protection Maturity (Priority 1.4)

### Changes
Privacy feature flags now expose exact runtime state: enabled, disabled, unsupported.

**New APIs:**
- `IsPrivacyFeatureSupported(feature)` — query support
- `IsPrivacyFeatureEnabled(feature)` — query active status
- `PrivacyFeaturesRuntimeState()` — introspection across all features

### Affected Features
- Padding bucket selection (256B/1024B/4096B/16384B)
- Cover traffic scheduling
- Epoch-based pseudonym rotation (6-hour cycles)
- Async message obfuscation

### Migration Guide
**For privacy-critical applications:** Call `PrivacyFeaturesRuntimeState()` at startup to validate configuration.

**For applications indifferent to privacy features:** No action required (all enabled by default).

---

## 5. Compatibility Test Matrix (Priority 4)

All protocol version combinations are now tested and verified:

| Scenario | Expected Mode | Status |
|----------|--------------|--------|
| Legacy ↔ Legacy | Legacy | ✅ Tested |
| Legacy+Noise ↔ Legacy | Legacy | ✅ Tested |
| Legacy+Noise ↔ Noise | Noise | ✅ Tested |
| Noise ↔ Noise | Noise | ✅ Tested |
| Noise+Ratchet ↔ Noise | Noise | ✅ Tested |
| Noise+Ratchet ↔ Legacy | Legacy (fallback) | ✅ Tested |
| Noise+Ratchet ↔ Noise+Ratchet | Noise+Ratchet | ✅ Tested |

Wire-format regressions: None. Negotiation is deterministic and policy-compliant.

---

## 6. Operational Impact

### Performance
- Ratcheting overhead: ~10-15% per message (profile with `docs/PROFILING.md`)
- Signature verification: < 1ms per negotiation
- Privacy feature overhead: < 1% (cover traffic configurable)

### Logging & Observability
- **Downgrade events:** Logged at INFO level with reason, SLA target, and fallback decision
- **Security errors:** Logged at WARN level with context and recovery action
- **Key changes:** Logged at INFO level with peer identity, previous/new keys, timestamp

### Monitoring
Monitor these metrics for operational health:
- `session_mode_negotiation_failures` — indicates compatibility issues
- `ratchet_rekey_warnings` — indicates high-volume sessions
- `downgrade_events_total` — indicates peer capability variation

---

## 7. Release Timeline

**v1.4.0-qtox-preview:** 2026-04-04 (current)  
**v1.4.0 (stable):** 2026-06-15 (pending external audit closure)

---

## 8. Questions & Support

For integration questions or concerns about these changes:
1. Open an issue tagged `[security-behavior-change]`
2. For sensitive issues, use GitHub Private Security Advisory (see SECURITY.md)
3. Reference the security policy SLA for response timelines

---

## Appendix: Version Compatibility Table

| Library Version | Min Peer Version | Notes |
|-----------------|-----------------|-------|
| v1.4.0+ | v0.1.0 (legacy) | Automatic negotiation via downgrade |
| v1.4.0+ | v1.0.0+ (Noise) | Full Noise support |
| v1.4.0+ | v1.4.0+ (current) | Ratcheting enabled |

---

**Document Version:** 1.0  
**Last Updated:** 2026-06-03  
**Status:** Ready for Release
