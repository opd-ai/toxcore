# Security Remediation Plan

## Goal
Harden toxcore-go against the issues identified in the recent security review while preserving wire and API compatibility with:
- classic Tox (`ProtocolLegacy`)
- Tox-IK (`ProtocolNoiseIK`)

## Compatibility Guardrails (Must Not Break)
- [ ] Do not change `ProtocolLegacy = 0` or `ProtocolNoiseIK = 1` values.
- [ ] Keep legacy packet handling and NaCl-box compatibility intact for legacy-only peers.
- [ ] Keep signed negotiation and version commitment behavior for Noise-IK peers.
- [ ] Avoid breaking C API symbols and existing Go public APIs unless gated behind opt-in flags.
- [ ] Any stricter security defaults must include migration guidance and explicit compatibility toggles.

## Priority 0: Assurance and Governance

### 0.1 Independent Cryptography Audit
- [ ] Define external audit scope: negotiation, Noise transport, async pre-keys, obfuscation, ratchet integration, key lifecycle.
- [ ] Freeze a release candidate branch for audit reproducibility.
- [ ] Produce threat model document for auditors (passive, active, compromise, metadata adversaries).
- [ ] Track findings in a public remediation table with severity/SLA.
- [ ] Publish final report and verify all critical/high issues are closed.

Acceptance criteria:
- [ ] Third-party audit completed and report linked in docs.
- [ ] No unresolved critical findings before release.

## Priority 1: Core Security Gaps

### 1.1 Enforce Encryption Invariants (No Silent Insecure Fallback)
Issue addressed: plaintext send path can occur if encryption prerequisites are missing in some integration scenarios.

- [ ] Add a strict mode (`RequireE2EE`) that rejects outbound plaintext friend messages when keys/secure session are unavailable.
- [ ] Keep current behavior for compatibility only when strict mode is disabled.
- [ ] Emit structured warnings whenever compatibility plaintext fallback is used.
- [ ] Add migration docs for app developers to enable strict mode safely.

Compatibility safeguards:
- [ ] Legacy peers continue to work through encrypted legacy path (NaCl-box), not plaintext.
- [ ] Strict mode defaults staged: warn -> opt-in enforce -> future default (major release only).

Acceptance criteria:
- [ ] Tests prove no plaintext transmission in strict mode.
- [ ] Legacy and Noise-IK interoperability tests both pass.

### 1.2 Raise Post-Compromise Security to Signal-like Defaults
Issue addressed: ratcheting exists but is not uniformly guaranteed as default for all live messaging paths.

- [ ] Define session policy layer: `legacy-only`, `noise-only`, `noise+ratchet`.
- [ ] Introduce opt-in default for new apps: `noise+ratchet` where both peers support it.
- [ ] Keep fallback negotiation to `ProtocolNoiseIK` and then `ProtocolLegacy` per existing policy.
- [ ] Ensure ratchet state bootstrap is authenticated and bound to established transport identity.
- [ ] Add key deletion checks and skipped-key limits telemetry.

Compatibility safeguards:
- [ ] If peer does not support ratchet extension, continue existing Noise-IK behavior.
- [ ] If peer supports only legacy, preserve classic Tox behavior without protocol breakage.

Acceptance criteria:
- [ ] End-to-end tests cover mixed pairs: legacy/legacy, legacy/noise, noise/noise, noise+ratchet/noise.
- [ ] Replay and post-compromise recovery tests added for ratchet-enabled sessions.

### 1.3 Harden Trust Establishment and MITM Resistance
Issue addressed: strong primitives exist, but trust UX/workflow and enforcement can be inconsistent.

- [ ] Require signature validation on version negotiation where supported.
- [ ] Add explicit TOFU state machine with key-change alarms and app callback requirements.
- [ ] Bind signed pre-keys to identity verification state in async flows.
- [ ] Add safety-number verification helpers and status APIs for clients.

Compatibility safeguards:
- [ ] Maintain optional behavior flags for legacy clients without signature support.
- [ ] Never auto-downgrade security silently when signed negotiation fails.

Acceptance criteria:
- [ ] MITM downgrade tests fail closed under secure policy.
- [ ] Key-change detection and user-notification callbacks covered by tests.

### 1.4 Metadata Protection Maturity
Issue addressed: strong privacy features exist, but implementation completeness and consistency need hardening.

- [ ] Reconcile design docs with implementation status (especially cover traffic).
- [ ] Ensure privacy feature flags expose exact runtime state (enabled, disabled, unsupported).
- [ ] Add regression tests for padding bucket behavior and pseudonym rotation invariants.
- [ ] Add adversarial timing-analysis simulation tests for cover traffic scheduler behavior.

Compatibility safeguards:
- [ ] Keep extension packet range behavior backward-compatible for legacy peers.
- [ ] Ensure unknown extension packet handling remains safe and non-breaking.

Acceptance criteria:
- [ ] Documentation and code paths are consistent and versioned.
- [ ] Privacy feature test suite passes across transport variants.

## Priority 2: Implementation Quality and Safety

### 2.1 Reduce Multi-Path Crypto Risk
- [ ] Map all encryption paths (legacy, Noise, async, ratchet) and declare allowed transitions.
- [ ] Add centralized policy checks so insecure transitions are impossible by default.
- [ ] Add static assertions/integration tests that block unreviewed crypto path additions.

### 2.2 Side-Channel and Memory Hygiene Verification
- [ ] Add focused tests/benchmarks for key zeroization paths and sensitive buffer lifetimes.
- [ ] Audit logs for potential key material leakage and enforce safe logging guidelines.
- [ ] Add CI checks for forbidden debug fields in crypto-sensitive packages.

### 2.3 Error-Handling Security
- [ ] Standardize error classes: fatal security errors vs compatibility warnings.
- [ ] Ensure all downgrade or verification-failure paths are explicit, observable, and test-covered.

Acceptance criteria (Priority 2):
- [ ] Security policy checks are unit-tested and integration-tested.
- [ ] CI enforces logging/error invariants.

## Priority 3: Practical and Operational Hardening

### 3.1 Performance Under Secure Defaults
- [ ] Benchmark overhead of `noise+ratchet` and privacy features per transport.
- [ ] Add profile-guided optimizations without changing protocol semantics.
- [ ] Define recommended secure profiles for desktop/mobile/embedded.

### 3.2 Developer Experience
- [ ] Publish secure integration guide with decision tables and example configs.
- [ ] Add a runtime security posture API (effective mode, downgrade state, weak settings).
- [ ] Add startup warnings for risky config combinations.

### 3.3 Maintenance and Response
- [ ] Add security patch playbook with release timelines.
- [ ] Expand CI with targeted protocol compatibility matrix and fuzz corpus growth.
- [ ] Track dependency risk with periodic reviews and lockfile verification guidance.

Acceptance criteria (Priority 3):
- [ ] Benchmarks and secure profiles published.
- [ ] Security posture API documented and covered by tests.

## Compatibility Test Matrix (Required for Every Security PR)
- [ ] Legacy-only peer <-> Legacy-only peer
- [ ] Legacy+Noise peer <-> Legacy-only peer
- [ ] Legacy+Noise peer <-> Noise-only peer
- [ ] Noise-only peer <-> Noise-only peer
- [ ] Noise+Ratchet peer <-> Noise-only peer
- [ ] Noise+Ratchet peer <-> Legacy-only peer (expected negotiated fallback)
- [ ] Signed negotiation required vs unsupported peer behavior
- [ ] Version commitment mismatch handling

Definition of done for matrix:
- [ ] No wire-format regressions for classic Tox or Tox-IK.
- [ ] Negotiation results are deterministic and policy-compliant.
- [ ] Security downgrade paths are explicit and logged.

## Release Gating Checklist
- [ ] All Priority 1 items completed or explicitly deferred with risk sign-off.
- [ ] External audit critical/high findings resolved.
- [ ] Compatibility matrix green in CI.
- [ ] Updated protocol/spec docs merged with code changes.
- [ ] Security advisory notes prepared for behavior changes.

## Suggested Implementation Order
1. Enforce encryption invariants with compatibility toggles.
2. Add policy layer for ratchet rollout and fallback safety.
3. Complete trust-establishment hardening and tests.
4. Reconcile privacy implementation/docs and timing tests.
5. Run external audit and close findings.

