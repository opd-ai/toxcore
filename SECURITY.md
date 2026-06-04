# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest (`main`) | ✅ Yes |
| older tags | ❌ No (upgrade to latest) |

Security fixes are applied only to the latest commit on `main`. Older tagged
versions are not patched. Users should always run the most recent version.

---

## Scope

The following are in scope for vulnerability reports:

- Cryptographic correctness (key derivation, encryption/decryption, signatures)
- Protocol-level weaknesses (pre-key handling, Noise handshake, forward secrecy)
- Memory-safety issues affecting key material (improper wiping, leaks via GC)
- Authentication bypasses (friend-request validation, pre-key bundle spoofing)
- Denial-of-service attacks on core protocol state (e.g., pre-key exhaustion)
- Safety-number / fingerprint collisions or predictability

The following are **out of scope**:

- Vulnerabilities in third-party dependencies not controlled by this project
  (report those to the respective upstream project)
- Physical attacks on the host machine
- Social engineering of project maintainers

---

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Please report vulnerabilities privately through one of the following channels:

1. **GitHub Private Security Advisory** (preferred):
   Navigate to [Security → Advisories → New draft advisory](../../security/advisories/new)
   in this repository and fill in the details. This is end-to-end encrypted and
   visible only to repository maintainers.

2. **Encrypted email**: Send a report to the maintainer listed in `go.mod` (see
   the module path for contact information). Encrypt the message with the
   maintainer's public GPG key if one is published.

Please include the following in your report:

- A clear description of the vulnerability and its impact
- Steps to reproduce (proof-of-concept code if applicable)
- The affected Go packages, files, and functions
- Your suggested severity (Critical / High / Medium / Low)
- Whether you would like to be credited in the advisory

---

## Response SLA

| Milestone | Target |
|-----------|--------|
| Acknowledgement of receipt | Within **48 hours** |
| Initial triage and severity assessment | Within **7 days** |
| Fix or mitigation for Critical/High findings | Within **90 days** |
| Public disclosure (coordinated) | After the fix is released, typically 14 days after patching |

We follow [coordinated disclosure](https://en.wikipedia.org/wiki/Coordinated_vulnerability_disclosure).
We will credit reporters in the GitHub Security Advisory unless they request
anonymity.

---

## CVE Assignment

For vulnerabilities that warrant a CVE:

- We will request a CVE from [MITRE](https://cveform.mitre.org/) or through
  GitHub's CNA (Common Vulnerabilities and Exposures Numbering Authority)
  process.
- We will reference the CVE in the fix commit and the public advisory.

---

## Bug Bounty

There is currently no paid bug-bounty programme. We may establish one (e.g.,
via HackerOne or Immunefi) after the first external cryptographic audit is
complete and the codebase is considered stable.

---

## External Audit Status

| Date | Scope | Firm | Report |
|------|-------|------|--------|
| 2026-05-29 | Self-audit (internal) | opd-ai | [BACKLOG_ANALYSIS.md](BACKLOG_ANALYSIS.md) |

A third-party professional audit has not yet been performed. Until one is
complete, treat this library as **experimental** and unsuitable for
production deployments where compromise would cause serious harm.

---

## Security-Relevant Design Decisions

### Cryptographic Features

toxcore-go implements several modern cryptographic enhancements beyond the original Tox protocol:

- **PQXDH (Post-Quantum Hybrid)** — ML-KEM-768 (FIPS 203) combined with X3DH provides
  quantum-resistant initial session establishment. The session root key is derived from
  both classical X25519 ECDH and post-quantum shared secrets via HKDF-SHA256, protecting
  against harvest-now-decrypt-later attacks (`crypto/pqxdh.go`).
- **X3DH (Extended Triple Diffie-Hellman)** — Signal Protocol's X3DH for initial key
  agreement with perfect forward secrecy, KCI resistance, and deniable authentication via
  four DH exchanges (`crypto/x3dh.go`).
- **Sealed Sender** — Encrypts sender identity within message envelopes using AES-256-GCM
  under a recipient-derived key, preventing transport-layer sender identification
  (`crypto/sealed_sender.go`).
- **Double Ratchet Header Encryption** — Encrypts ratchet message headers with
  XChaCha20-Poly1305 under a separate header key, hiding message sequence numbers and
  ratchet state from network observers (`ratchet/`).
- **Protocol Version Negotiation** — Per-peer negotiation between `ProtocolLegacy` and `ProtocolNoiseIK`.
  Negotiation packets can be Ed25519-signed to mitigate MITM downgrade attacks (`transport/version_negotiation.go`).
  The signed packet format also defines a reserved 1-byte capability bitmask (`CapX3DH`, `CapPQXDH`, `CapHeaderEncryption`),
  but outgoing packets currently advertise `Capabilities = 0`.

All advanced features are opt-in via capability flags and maintain full backward
compatibility with legacy Tox implementations.

### Memory Protection

- All key material is wiped after use via `crypto.ZeroBytes` / `crypto.SecureWipe`
  (see `crypto/secure_memory.go`).
- `crypto.SecureAllocate` allocates key buffers via C `malloc(3)` + `mlock(2)` (on
  Linux/macOS with CGo enabled) so that key material cannot be paged to swap.
  On pure-Go builds or unsupported platforms, it falls back to a regular Go
  allocation. See `MlockAvailable()` to query build-time capability.

### Authentication & Integrity

- Pre-key bundles are Ed25519-signed (see `async/prekeys.go`) to prevent relay
  substitution attacks.
- Safety numbers (see `crypto/safety_number.go`) allow out-of-band identity
  verification, following Signal Protocol's approach.
- One-time pre-key consumption is rate-limited per peer to resist pool-exhaustion
  denial-of-service attacks.
