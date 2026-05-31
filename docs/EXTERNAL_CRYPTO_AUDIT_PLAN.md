# External Cryptography Audit Plan

This document defines the execution plan for an independent cryptography audit of toxcore-go.
It is intended to make the audit reproducible, scoped, and measurable.

## 1. Audit Scope

The external audit scope includes the following components and interfaces:

1. Protocol negotiation and downgrade resistance
   - `transport/version_negotiation.go`
   - `transport/negotiating_transport.go`
   - Signed negotiation parsing and verification paths
2. Noise transport handshake and session lifecycle
   - `transport/noise_transport.go`
   - `noise/` handshake helpers
3. Async pre-key and forward secrecy flows
   - `async/prekeys.go`
   - `async/forward_secrecy.go`
   - `async/client*.go`
4. Identity obfuscation and pseudonym rotation
   - `async/obfs.go`
   - `async/epoch.go`
5. Ratchet integration and key lifecycle guarantees
   - `ratchet/`
   - Ratchet usage in messaging and async paths
6. Secret handling and key erasure hygiene
   - `crypto/secure_memory.go`
   - Related key lifecycle code in `crypto/` and `transport/`

Out of scope:

1. UI and application-level trust UX in downstream clients
2. Infrastructure hardening outside toxcore-go source control
3. Non-security refactors with no cryptographic behavior impact

## 2. Reproducible Release Candidate Freeze

The audit must run against a fixed release candidate branch and commit.

Branch convention:

- `audit/rc-<YYYYMMDD>`

Freeze requirements:

1. Tag the audited commit using annotated tag format `audit-rc-<YYYYMMDD>`.
2. Record commit SHA, Go toolchain version, and dependency graph snapshot (`go.mod`, `go.sum`).
3. Disallow non-audit changes on the RC branch until audit findings are triaged.
4. Require all remediation patches to reference finding IDs from the table in Section 4.

## 3. Auditor Threat Model Pack

The audit must evaluate behavior against the following adversary classes:

1. Passive network adversary
   - Can observe packet timing and sizes.
   - Goal: metadata correlation and relationship inference.
2. Active network adversary
   - Can inject, replay, and drop packets.
   - Goal: downgrade security level, induce insecure state transitions.
3. Key compromise adversary
   - Obtains long-term keys or selected session state.
   - Goal: impersonation (including KCI) and post-compromise decryption.
4. Metadata-focused storage adversary
   - Honest-but-curious async storage nodes.
   - Goal: infer sender/recipient linkage and communication cadence.

Primary security properties under review:

1. Mutual authentication and downgrade resilience
2. Forward secrecy and post-compromise recovery behavior
3. Replay resistance and state machine fail-closed behavior
4. Sensitive material handling (zeroization, log safety)

## 4. Public Remediation Table and SLA

All findings are tracked in this table and kept public in-repo.

| Finding ID | Area | Severity | Status | SLA (days) | Owner | Opened | Target Fix | Notes |
|------------|------|----------|--------|------------|-------|--------|------------|-------|
| AUDIT-PLACEHOLDER-001 | negotiation | TBD | open | TBD | TBD | TBD | TBD | Placeholder row until external report is delivered |

Severity SLA policy:

1. Critical: 7 days
2. High: 14 days
3. Medium: 30 days
4. Low: next scheduled release

## 5. Exit Criteria

Audit governance is considered complete when all are true:

1. External report is published and linked from `docs/README.md`.
2. No unresolved critical findings remain.
3. No unresolved high findings remain without explicit risk sign-off.
4. All remediations include regression tests.