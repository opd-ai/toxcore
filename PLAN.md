# Security Enhancement Plan: toxcore-go

## Roadmap to Signal Protocol-Level Security

**Document Version:** 1.0
**Target Library Version:** `main` (2026-06-03, classical X25519/Noise-IK + Double Ratchet) → `signal-equiv` (explicit X3DH/PQXDH, header-encrypted ratchet, sealed metadata)
**Last Updated:** 2026-06-03

---

## Executive Summary

`toxcore-go` already implements most of the Signal Protocol's cryptographic core: a spec-faithful Double Ratchet (`ratchet/`, HKDF-SHA256 root chain, HMAC-SHA256 symmetric chain, X25519 DH ratchet, XChaCha20-Poly1305 payloads), Ed25519-signed and one-time pre-keys (`async/prekeys.go`), iterated SHA-512 safety numbers (`crypto/safety_number.go`), and a Noise-IK transport handshake (`noise/`, `flynn/noise`). The gap to Signal-equivalence is therefore **not** the ratchet itself but four protocol-level properties: (1) the initial asynchronous key agreement is bootstrapped from Noise-IK rather than an explicit **X3DH** that binds identity, signed pre-key, and one-time pre-key into the ratchet root; (2) ratchet **headers are transmitted in plaintext**, leaking DH-ratchet public keys and message indices; (3) there is **no post-quantum hybrid** (Signal's PQXDH); and (4) metadata protection is mature only on the async path. This plan closes those gaps library-side, preserving wire/API compatibility with classic Tox (`ProtocolLegacy`) and Tox-IK (`ProtocolNoiseIK`), and excludes all client/UI concerns.

**Implementation Phases:**
- Phase 1: Critical Security Foundations — explicit X3DH initial agreement + Double Ratchet header encryption
- Phase 2: Advanced Security Features — PQXDH post-quantum hybrid, bounded untrusted decode, real-time sealed-sender metadata
- Phase 3: Hardening & Optimization — multi-device (Sesame-equivalent) sessions, cryptographic identity checksum, formal test-vector & side-channel validation

Phases are ordered by dependency, not by calendar: Phase 2 PQXDH extends the Phase 1 X3DH transcript, and Phase 3 multi-device builds on the Phase 1/2 session APIs.

---

## 1. Security Gap Analysis

### Critical Gaps (Must Fix - Phase 1)
| ID | Component | Current State | Signal Standard | Impact |
|----|-----------|---------------|-----------------|---------|
| C1 | Initial asynchronous key agreement | Ratchet root is seeded from a Noise-IK shared secret or a single static ECDH; one-time pre-keys exist (`async/prekeys.go`) but are not combined into the root via a defined multi-DH transcript (`ratchet/session.go:73` `InitInitiator(sharedKey,…)`) | X3DH: `DH1=DH(IK_A,SPK_B) ‖ DH2=DH(EK_A,IK_B) ‖ DH3=DH(EK_A,SPK_B) ‖ DH4=DH(EK_A,OPK_B)`, `SK=KDF(DH1‖DH2‖DH3‖DH4)` | Offline first message lacks per-session one-time-pre-key forward secrecy and full mutual-authentication/deniability binding into the ratchet root; weaker KCI guarantees |
| C2 | Double Ratchet header confidentiality | 40-byte header `(DHpub32 ‖ PN ‖ N)` sent as plaintext associated data (`ratchet/header.go`) | Double Ratchet **header encryption** variant: headers encrypted with a separate header key from the root KDF | Passive relays/observers can link messages to a session and read ratchet DH public keys and message counters (traffic-analysis metadata leak) |

### Important Gaps (Should Fix - Phase 2)
| ID | Component | Current State | Signal Standard | Impact |
|----|-----------|---------------|-----------------|---------|
| I1 | Post-quantum resistance | Purely classical X25519 across handshake, X3DH-seed, and DH ratchet | PQXDH: hybrid X25519 **+ ML-KEM-768 (Kyber)** in the initial agreement | Removes harvest-now-decrypt-later exposure of initial session secrets |
| I2 | Untrusted relay decode bounds | Retrieve-response payloads from untrusted storage nodes decoded with `encoding/gob` and no length/element cap (`async/client.go`) | Length-prefixed, bounded binary framing for all relay-facing decode | Closes a memory-pressure DoS reachable over TCP-relay transport (AUDIT M-1 / GAPS Gap 1) |
| I3 | Real-time metadata / sender anonymity | Async path has epoch pseudonyms + per-message sender pseudonyms (`async/obfs.go`); real-time path exposes sender identity to the transport peer | Sealed sender: sender identity encrypted to recipient, delivered under a derived/ephemeral envelope | Brings real-time path to the async path's metadata-protection level |

### Enhancements (Nice to Have - Phase 3)
| ID | Component | Current State | Signal Standard | Benefit |
|----|-----------|---------------|-----------------|---------|
| E1 | Multi-device session management | One session per peer identity; no per-device fan-out or device-list authentication | Sesame: per-device sessions keyed by an authenticated device list | Consistent E2EE across a user's devices without re-keying the identity |
| E2 | Identity integrity & dependency gating | ToxID trailing checksum is a 16-bit XOR (`crypto/toxid.go:130`), typo-detection only; no automated dependency-CVE gate in CI | Cryptographic fingerprint verification (safety numbers already SHA-512×5200) + continuous advisory tracking | Removes reliance on a non-cryptographic checksum and detects dependency CVEs within SLA |
| E3 | Formal validation & test vectors | Crypto has unit/property/fuzz tests but no cross-checked Signal known-answer vectors or documented side-channel review for the new components | Known-answer vectors vs. `libsignal`; constant-time/side-channel review | Demonstrable spec alignment and leakage-resistance for crypto-critical code |

---

## 2. Implementation Roadmap

### Phase 1: Critical Security Foundations
**Goal:** Make the initial session secret an explicit X3DH derivation and remove ratchet-header metadata leakage, so offline-first sessions match Signal's forward-secrecy and confidentiality guarantees.

**Components:**
1. **X3DH Initial Key Agreement** - C1
   - Current: Root key seeded from Noise-IK / single static ECDH; one-time pre-keys consumed but not folded into a multi-DH transcript.
   - Target: Four-DH X3DH transcript (`IK`, `SPK`, `OPK`, ephemeral `EK`) feeding `KDF` → ratchet root, with associated data binding both identities.
   - Key changes:
     - Define an X3DH transcript builder over the existing `async/prekeys.go` bundle (signed pre-key, one-time pre-key, identity key).
     - Convert Ed25519 identity keys to X25519 for DH (XEdDSA / birational map) without changing the published identity key.
     - Derive `SK = HKDF-SHA256(F ‖ DH1 ‖ DH2 ‖ DH3 ‖ DH4)` and pass `SK` as the ratchet root seed to `InitInitiator` / `InitRecipient`.
     - Authenticate the signed pre-key (already Ed25519-signed) and enforce one-time-pre-key single use + zeroization.
   - Success criteria:
     - Initial root key is reproducible only by holders of the four DH inputs (KAT-verified).
     - Compromise of a long-term identity key alone does not reconstruct a past session root.
     - One-time pre-keys are consumed exactly once and wiped (`crypto.ZeroBytes`).

2. **Double Ratchet Header Encryption** - C2
   - Current: Header `(DHpub ‖ PN ‖ N)` is plaintext associated data (`ratchet/header.go`).
   - Target: Signal header-encryption variant — header keys (`HK_s`, `NHK_s` / `HK_r`, `NHK_r`) derived in the root KDF and used to AEAD-encrypt the header.
   - Key changes:
     - Extend `kdfRootChain` outputs to also yield sending/receiving header keys and next-header keys.
     - Encrypt the serialized header with XChaCha20-Poly1305 under the current header key; trial-decrypt with current then next header key on receive.
     - Bind the encrypted header as associated data of the payload AEAD.
     - Negotiate header-encryption capability so non-upgraded peers continue with the plaintext-header ratchet.
   - Success criteria:
     - A passive observer cannot recover `DHpub`, `PN`, or `N` from a captured message.
     - Receiver correctly distinguishes current-vs-next header key (DH-ratchet step) and skipped messages.
     - Mixed-capability pairs negotiate deterministically with no plaintext-header downgrade once both support encryption.

**Phase Exit Criteria:**
- [x] X3DH KATs pass and forward-secrecy/KCI properties are test-proven for the offline-first path.
- [x] Header-encryption interop tests pass for upgraded↔upgraded and upgraded↔legacy-ratchet pairs.
- [x] No ratchet metadata (DH pubkey, counters) is recoverable from captured ciphertext in the encrypted-header mode.
- [x] `go test ./ratchet/... ./async/... ./crypto/...` green; no public-API or wire regressions for `ProtocolLegacy`/`ProtocolNoiseIK`.

### Phase 2: Advanced Security Features
**Goal:** Add post-quantum protection to the initial agreement, eliminate the untrusted-relay decode DoS, and extend metadata protection to the real-time path.

**Components:**
1. **PQXDH Post-Quantum Hybrid** - I1
   - Current: Classical X25519 only in the initial agreement.
   - Target: Hybrid X3DH + ML-KEM-768 KEM, mixing both shared secrets into the root KDF.
   - Key changes:
     - Add a signed last-resort ML-KEM-768 pre-key and one-time ML-KEM pre-keys to the bundle.
     - Compute `SS_pq = ML-KEM.Decaps/Encaps` alongside the four X25519 DHs.
     - Derive `SK = HKDF(F ‖ DH1..DH4 ‖ SS_pq)`; keep classical-only path for peers without PQ capability.
     - Add a PQ-capability bit to negotiation, signed to prevent PQ-downgrade.
   - Success criteria:
     - Session secret is unrecoverable without breaking **both** X25519 and ML-KEM-768.
     - PQ-downgrade is detected/rejected when both peers advertise PQ support.
     - Bundle size/perf impact stays within documented mobile/embedded budgets.

2. **Bounded Untrusted Relay Decode** - I2
   - Current: `encoding/gob` decode of relay payloads with no size/element cap (`async/client.go`).
   - Target: Length-prefixed binary framing with explicit per-field and element-count bounds before allocation.
   - Key changes:
     - Validate `packet.Data` via `limits.ValidateProcessingBuffer` (or a tighter async bound) before decode.
     - Replace `gob` with the project's existing length-prefixed binary codec for the retrieve-response type.
     - Cap decoded slice lengths (e.g., `[]*ObfuscatedAsyncMessage`) and reject over-count payloads.
   - Success criteria:
     - An oversized/over-count relay payload is rejected before large allocation (regression test).
     - No behavioral change for well-formed responses.

3. **Real-Time Sealed Sender** - I3
   - Current: Real-time transport peer learns sender identity; only async path is pseudonymous.
   - Target: Encrypt sender identity to the recipient under a derived envelope, mirroring `async/obfs.go` design for the live path.
   - Key changes:
     - Define a sender-certificate/envelope encrypted to the recipient's identity key.
     - Reuse HKDF-SHA256 pseudonym/proof construction already validated in `async/obfs.go`.
     - Gate behind capability negotiation; fall back to authenticated (non-sealed) delivery for legacy peers.
   - Success criteria:
     - Transport peer cannot derive sender identity from a sealed real-time message.
     - Recipient authenticates the true sender after unsealing; spoofed envelopes are rejected.

**Phase Exit Criteria:**
- [x] PQXDH KATs (ML-KEM-768) pass; classical and hybrid paths both interoperate and resist downgrade.
- [x] Relay-decode fuzz/regression suite rejects malicious payloads with bounded memory.
- [x] Sealed-sender tests confirm sender-identity confidentiality + authenticity on the real-time path.
- [x] `go test ./...` green; benchmarks within target budgets (§6).

### Phase 3: Hardening & Optimization
**Goal:** Round out Signal-equivalence with authenticated multi-device sessions, a cryptographic identity check, and formal validation of all new crypto.

**Components:**
1. **Multi-Device Sessions (Sesame-equivalent)** - E1
   - Current: Single session per peer identity.
   - Target: Per-device sessions keyed by an authenticated, signed device list.
   - Key changes:
     - Define a signed device-list record and per-device pre-key bundles.
     - Fan out X3DH/PQXDH + ratchet per active device; encrypt once per device.
     - Handle device add/remove with session teardown and key zeroization.
   - Success criteria:
     - Adding/removing a device does not weaken or re-key the long-term identity.
     - Messages decrypt on all authenticated devices; removed devices lose access.

2. **Cryptographic Identity Checksum & Dependency Gate** - E2
   - Current: 16-bit XOR ToxID checksum (`crypto/toxid.go:130`); no `govulncheck` gate.
   - Target: Keep safety numbers as the trust anchor; document XOR as typo-only, and add a continuous dependency-advisory gate.
   - Key changes:
     - Clarify in code/docs that integrity rests on safety numbers (SHA-512×5200), not the checksum.
     - Add a `govulncheck ./...` CI job failing on known advisories.
   - Success criteria:
     - Safety-number verification is the documented identity-integrity mechanism.
     - CI fails on any advisory in `golang.org/x/crypto`, `flynn/noise`, ML-KEM, or transitive deps.

3. **Formal Test Vectors & Side-Channel Review** - E3
   - Current: Unit/property/fuzz tests, no cross-implementation KATs for new components.
   - Target: Known-answer vectors vs. `libsignal` + constant-time review of new crypto.
   - Key changes:
     - Add KAT suites for X3DH, header-encryption KDF, PQXDH, and existing ratchet chains.
     - Constant-time/side-channel review of header trial-decryption and KEM handling.
   - Success criteria:
     - New crypto matches reference vectors bit-for-bit.
     - No secret-dependent branches/timing in the new code paths.

**Phase Exit Criteria:**
- [ ] Multi-device add/remove and fan-out tests pass with correct key lifecycle.
- [ ] KAT suites green against reference vectors; `govulncheck` integrated and passing.
- [ ] Side-channel review documented with sign-off; `go test ./...` green.

---

## 3. Technical Implementation Details

### 3.1 X3DH Initial Key Agreement

**Addresses Gap:** C1
**Phase:** 1 (Critical Foundations)
**Priority:** CRITICAL

#### Current vs. Target Architecture
**Current:**
```
Pre-key bundle (async/prekeys.go): Ed25519-signed SPK, 200 one-time pre-keys, identity key.
Initial session secret:
  sharedKey = Noise-IK output  OR  single static X25519 ECDH
  ratchet.InitInitiator(sharedKey, peerRatchetPub)   // ratchet/session.go:73
One-time pre-keys are consumed but not folded into the ratchet root transcript.
```

**Target (Signal-equivalent):**
```
Initiator A → Responder B (B offline), using B's published bundle (IK_B, SPK_B, OPK_B):
  DH1 = DH(IK_A,  SPK_B)
  DH2 = DH(EK_A,  IK_B)
  DH3 = DH(EK_A,  SPK_B)
  DH4 = DH(EK_A,  OPK_B)          // omitted if no OPK available
  SK  = HKDF-SHA256(F ‖ DH1 ‖ DH2 ‖ DH3 ‖ DH4)   // F = 32×0xFF domain prefix
  AD  = Encode(IK_A_pub) ‖ Encode(IK_B_pub)
  ratchet root := SK ; initial message carries IK_A, EK_A, SPK_B-id, OPK_B-id
DH keys are X25519; identity DH uses XEdDSA mapping of the Ed25519 identity key.
```

#### Implementation Steps

**1. Identity key DH mapping**
- **What:** Use the long-term Ed25519 identity key for X25519 DH without changing the published key.
- **How:** Apply the Ed25519→X25519 (Montgomery) birational map / XEdDSA; reuse `crypto/shared_secret.go` `curve25519.X25519` for the DH itself.
- **Validates:** Mutual authentication binding identities into `SK`.

**2. X3DH transcript builder**
- **What:** Compute `DH1..DH4` and `SK` from a pre-key bundle.
- **How:** Add a transcript function consuming `async/prekeys.go` bundle fields; verify the Ed25519 signature on `SPK_B` first; concatenate DH outputs in fixed order; `HKDF-SHA256(F ‖ transcript)` → 32-byte `SK`; zeroize all DH outputs.
- **Validates:** Forward secrecy from ephemeral + one-time pre-key; deniability (no signatures over messages).

**3. Ratchet integration**
- **What:** Seed the Double Ratchet with `SK` and the X3DH associated data.
- **How:** Pass `SK` to `InitInitiator`/`InitRecipient` (`ratchet/session.go:73,110`) as the root seed and `AD` as the first message's associated data; enforce one-time-pre-key single use and wipe.
- **Validates:** End-to-end forward secrecy from the first offline message onward.

**4. Capability gating & legacy fallback (backward compatibility)**
- **What:** Enable X3DH only when both peers advertise it; otherwise keep the existing Noise-IK / static-ECDH seeding for the async offline path.
- **How:** Add a signed X3DH-capability bit to the negotiation (`transport/version_negotiation.go`); the initial async message carries a self-describing version tag so a recipient distinguishes an X3DH initiation from a legacy one and selects the matching root-seeding path. Never silently downgrade once both peers advertise X3DH (bind the choice into the version-commitment HMAC).
- **Validates:** Backward compatibility — legacy and Noise-IK-only peers continue with the current seeding and existing wire/API; no `ProtocolLegacy`/`ProtocolNoiseIK` regression.

#### Technical Specifications
- **Cryptographic primitives:** X25519 (RFC 7748) for DH; XEdDSA for identity-key DH; HKDF-SHA256 (RFC 5869) for `SK`; Ed25519 (RFC 8032) for SPK signature.
- **Key management:** OPKs generated in batches (existing 200 / refresh-at-50 / 30-day policy), consumed once, zeroized; SPK rotated every 7 days (existing).
- **Protocol flow:** verify SPK signature → assemble `DH1..DH4` → `HKDF` → ratchet root → send initial message with key identifiers.
- **Edge cases:** missing OPK (3-DH fallback), replayed initial message (bind to ratchet replay protection), SPK rotation race, invalid/forged SPK signature (reject), peer without X3DH capability (retain existing Noise-IK/static-ECDH seeding — capability-gated, no downgrade once mutually supported).

#### API Impact
**New APIs:**
```
// async (initial agreement)
func X3DHInitiate(bundle PreKeyBundle, selfIdentity IdentityKeyPair) (sk [32]byte, initMsg InitialMessage, err error)
func X3DHRespond(initMsg InitialMessage, self PreKeyStore) (sk [32]byte, err error)
```

**Modified APIs:**
```
// ratchet/session.go — root seed (sharedKey) now supplied by the X3DH SK (signatures unchanged)
func InitInitiator(sharedKey [32]byte, theirPub [32]byte) (*Session, error)
func InitRecipient(sharedKey [32]byte, myKeyPair KeyPair) *Session
```

**Deprecated:**
```
// Noise-IK/static-ECDH seeding of the ratchet root remains supported for non-X3DH
// peers (capability-gated, not removed); X3DH is preferred only when both peers advertise it.
```

#### Testing Requirements
- **Unit tests:** transcript ordering, 4-DH vs 3-DH (no OPK), SPK signature verification, `SK` zeroization.
- **Integration tests:** A-online→B-offline first message decrypts after B comes online; ratchet continues correctly.
- **Security tests:** identity-key-only compromise cannot reconstruct past `SK`; OPK single-use enforced; KCI resistance.
- **Performance tests:** initial agreement < 5 ms on desktop; bundle fetch/parse within async budgets.

#### Dependencies
- **Requires:** existing pre-key store (`async/prekeys.go`), X25519 (`crypto/shared_secret.go`).
- **Blocks:** 3.3 PQXDH (extends this transcript), 3.5 multi-device fan-out.
- **External:** `golang.org/x/crypto` (HKDF, curve25519); no new runtime deps for the classical path.

---

### 3.2 Double Ratchet Header Encryption

**Addresses Gap:** C2
**Phase:** 1 (Critical Foundations)
**Priority:** CRITICAL

#### Current vs. Target Architecture
**Current:**
```
header.go: 40-byte plaintext header = DHpub(32) ‖ PN(uint32 BE) ‖ N(uint32 BE)
Header is passed as AEAD associated data of the payload (XChaCha20-Poly1305).
→ DH-ratchet public key and message counters are visible on the wire.
```

**Target (Signal-equivalent):**
```
Root KDF additionally outputs header keys:
  (RK, CK, HK_s, NHK_s) on the sending DH step; (RK, CK, HK_r, NHK_r) on receiving.
encHeader = XChaCha20-Poly1305.Seal(HK_s, nonce, serialize(DHpub,PN,N))
Payload AEAD uses encHeader as associated data.
Receive: trial-decrypt with HK_r; on failure trial-decrypt with NHK_r ⇒ DH-ratchet step.
```

#### Implementation Steps

**1. Extend root KDF for header keys**
- **What:** Produce header and next-header keys alongside root/chain keys.
- **How:** Lengthen `kdfRootChain` HKDF output (`ratchet/ratchet.go:31`) to emit `HK`/`NHK`; store per-direction header keys in session state (`ratchet/session.go`).
- **Validates:** Header confidentiality keyed independently from message keys.

**2. Encrypt/decrypt headers**
- **What:** AEAD-protect the serialized header.
- **How:** Reuse `chacha20poly1305.NewX` with a header-specific HKDF nonce; on receive, trial current `HK_r` then `NHK_r` to detect DH-ratchet steps; keep skipped-key logic (`ratchet/skipped.go`) keyed on decrypted `(DHpub, N)`.
- **Validates:** No plaintext ratchet metadata; correct out-of-order + DH-step handling.

**3. Capability negotiation**
- **What:** Avoid breaking peers on the plaintext-header ratchet.
- **How:** Add a header-encryption capability flag to the signed negotiation (`transport/version_negotiation.go`); enable only when both peers advertise it; never downgrade silently once mutually supported.
- **Validates:** Backward compatibility without a metadata-leaking downgrade.

#### Technical Specifications
- **Cryptographic primitives:** XChaCha20-Poly1305 (RFC 8439 / XChaCha draft) for headers; HKDF-SHA256 for `HK`/`NHK` derivation.
- **Key management:** header keys rotate with each DH-ratchet step; old `HK`/`NHK` zeroized after rotation.
- **Protocol flow:** seal header → seal payload (AD=encHeader) → on receive, trial-decrypt header → derive message key → decrypt payload.
- **Edge cases:** skipped messages across a DH step (store skipped header+message keys), simultaneous DH steps, trial-decrypt cost bounding (constant-time failure handling).

#### API Impact
**New APIs:**
```
// Capability surfaced in negotiation result
const CapHeaderEncryption = 1 << n
```

**Modified APIs:**
```
// ratchet/header.go — Marshal/Unmarshal gain sealed-header forms
func (h Header) SealAndMarshal(hk [32]byte) ([]byte, error)
func UnmarshalAndOpen(data []byte, hkCurrent, hkNext [32]byte) (Header, bool /*ratchetStep*/, error)
// RatchetEncrypt/RatchetDecrypt internally route through sealed headers when negotiated.
```

**Deprecated:**
```
// Plaintext-header path retained only for non-upgraded peers; documented as metadata-leaking.
```

#### Testing Requirements
- **Unit tests:** root-KDF header-key derivation vectors; seal/open round-trip; current-vs-next key selection.
- **Integration tests:** in-order, out-of-order, and DH-step transitions in encrypted-header mode; mixed-capability pairs.
- **Security tests:** captured ciphertext yields no `DHpub`/`PN`/`N`; trial-decrypt is constant-time on failure.
- **Performance tests:** added header seal/open < 0.2 ms per message; throughput regression < 5%.

#### Dependencies
- **Requires:** existing ratchet KDF and AEAD (`ratchet/ratchet.go`), signed negotiation (`transport/version_negotiation.go`).
- **Blocks:** none (independent of X3DH but shares the session lifecycle).
- **External:** `golang.org/x/crypto/chacha20poly1305`, `.../hkdf` (already vendored).

---

### 3.3 PQXDH Post-Quantum Hybrid

**Addresses Gap:** I1
**Phase:** 2 (Advanced Security Features)
**Priority:** HIGH

#### Current vs. Target Architecture
**Current:**
```
All initial-agreement secrecy rests on X25519 (classical only).
```

**Target (Signal-equivalent):**
```
Hybrid: SK = HKDF(F ‖ DH1..DH4 ‖ SS_pq)
  SS_pq from ML-KEM-768 encapsulation against B's signed PQ pre-key (PQSPK_B) + PQ one-time pre-key.
Both classical (X25519) and PQ (ML-KEM-768) secrets must be broken to recover SK.
```

#### Implementation Steps

**1. PQ pre-keys**
- **What:** Add signed ML-KEM-768 last-resort + one-time pre-keys to the bundle.
- **How:** Extend `async/prekeys.go` with PQ key fields, Ed25519-signed like the classical SPK; persist/rotate alongside existing pre-keys.
- **Validates:** Authenticated PQ key material.

**2. Hybrid encapsulation**
- **What:** Mix the KEM shared secret into `SK`.
- **How:** Initiator encapsulates to `PQSPK_B`/PQ-OPK → `(ct, SS_pq)`; concatenate `SS_pq` after `DH4` in the X3DH HKDF input; send `ct` in the initial message.
- **Validates:** Harvest-now-decrypt-later resistance for the initial secret.

**3. Capability + downgrade protection**
- **What:** Negotiate PQ without enabling rollback.
- **How:** Add a signed PQ-capability bit to `transport/version_negotiation.go`; require hybrid when both advertise PQ.
- **Validates:** No silent PQ-downgrade.

#### Technical Specifications
- **Cryptographic primitives:** ML-KEM-768 (FIPS 203) KEM; X25519 (RFC 7748); HKDF-SHA256; Ed25519 signatures over PQ pre-keys.
- **Key management:** PQ-OPKs single-use + zeroized; PQSPK rotated on the SPK schedule; larger bundle accounted in storage limits.
- **Protocol flow:** verify PQSPK signature → encapsulate → append `SS_pq` to transcript → derive `SK`.
- **Edge cases:** peer without PQ capability (classical-only path), KEM decapsulation failure, bundle-size limits on constrained transports.

#### API Impact
**New APIs:**
```
func PQXDHInitiate(bundle PreKeyBundle, self IdentityKeyPair) (sk [32]byte, initMsg InitialMessage, err error)  // hybrid superset of X3DHInitiate
```

**Modified APIs:**
```
// PreKeyBundle / InitialMessage gain PQ fields (PQSPK, PQ-OPK id, KEM ciphertext).
```

**Deprecated:**
```
// Classical-only initiation remains supported for non-PQ peers (not deprecated, capability-gated).
```

#### Testing Requirements
- **Unit tests:** ML-KEM-768 KATs; hybrid transcript ordering; PQ signature verification.
- **Integration tests:** PQ↔PQ hybrid and PQ↔classical fallback; downgrade rejection.
- **Security tests:** `SK` unrecoverable without both X25519 and ML-KEM secrets (model/test).
- **Performance tests:** encapsulation < 1 ms; bundle size within mobile/embedded budgets.

#### Dependencies
- **Requires:** 3.1 X3DH transcript (PQXDH extends it).
- **Blocks:** none.
- **External:** a vetted Go ML-KEM-768 implementation (e.g., `golang.org/x/crypto` ML-KEM once stabilized); pin and gate via dependency-CVE job (E2).

---

### 3.4 Bounded Untrusted Relay Decode

**Addresses Gap:** I2
**Phase:** 2
**Priority:** MEDIUM

#### Current vs. Target Architecture
**Current:**
```
async/client.go retrieve-response: encoding/gob decode of attacker-controlled bytes,
no length or element-count cap (reachable up to 1 MiB over TCP-relay transport).
```

**Target (Signal-equivalent):**
```
Validate buffer (limits.ValidateProcessingBuffer) → length-prefixed binary decode
→ per-field + element-count bounds enforced before allocation.
A version/format tag distinguishes the new bounded framing from the legacy gob
framing, so upgraded and non-upgraded storage nodes/clients remain interoperable
during rollout (decoder accepts both; encoder uses bounded framing only with peers
known to support it).
```

#### Implementation Steps
**1. Pre-decode validation** — bound `packet.Data` size before any decode.
- **How:** call `limits.ValidateProcessingBuffer` (or a tighter async constant) and reject oversize early.
- **Validates:** Availability against memory-pressure DoS.

**2. Replace gob with bounded framing** — use the project's length-prefixed binary codec.
- **How:** define explicit decode of `[]*ObfuscatedAsyncMessage` with a max element count; reject over-count.
- **Validates:** No disproportionate allocation from a malicious relay.

**3. Backward-compatible transition** — keep interop with non-upgraded storage nodes/clients.
- **How:** tag responses with a format/version byte; the decoder accepts both legacy `gob` and the new bounded framing (legacy `gob` decode kept behind the same `ValidateProcessingBuffer` size cap so it is no longer unbounded); emit bounded framing only to peers that advertise support. Apply the same per-field/element-count caps to the legacy path during the transition.
- **Validates:** Backward compatibility — mixed-version deployments keep retrieving messages while the DoS bound applies to both formats.

#### Technical Specifications
- **Cryptographic primitives:** none changed (framing/parsing only).
- **Key management:** unchanged.
- **Protocol flow:** validate → detect format tag → decode header → bounded element loop.
- **Edge cases:** truncated payloads, max-count boundary, empty response, legacy `gob` response from a non-upgraded peer (decoded under the same size/count bounds).

#### API Impact
**New APIs:** none (internal codec change).
**Modified APIs:** internal `decodeRetrieveResponse` gains bounded binary framing while still accepting legacy `gob` (both under explicit size/element bounds) for mixed-version interop.
**Deprecated:** unbounded `gob` decode for network-facing async responses; bounded `gob` decoding is retained for the transition and removed only once all deployed nodes advertise bounded framing.

#### Testing Requirements
- **Unit tests:** boundary counts; truncated/oversize rejection; legacy `gob` and new framing both decode correctly under bounds.
- **Security tests:** crafted over-count payload rejected with bounded memory for **both** formats (regression for AUDIT M-1).
- **Performance tests:** decode latency unchanged for well-formed responses.
- **Compatibility tests:** upgraded↔non-upgraded storage-node/client pairs retrieve messages successfully during rollout.

#### Dependencies
- **Requires:** `limits` package, existing binary codec.
- **Blocks:** none.
- **External:** none.

---

### 3.5 Real-Time Sealed Sender, Multi-Device, Identity & Validation

**Addresses Gaps:** I3, E1, E2, E3
**Phase:** 2–3
**Priority:** MEDIUM

#### Implementation Steps
**1. Real-time sealed sender (I3)** — encrypt sender identity to recipient, reusing the validated `async/obfs.go` HKDF pseudonym/proof construction; capability-gated, with authenticated fallback for legacy peers.
**2. Multi-device sessions (E1)** — signed device list + per-device pre-key bundles; fan out X3DH/PQXDH + ratchet per device; teardown + zeroize on device removal.
**3. Identity integrity & dependency gate (E2)** — document safety numbers (SHA-512×5200) as the integrity anchor; add `govulncheck ./...` CI job.
**4. Formal validation (E3)** — KAT suites for X3DH, header-encryption KDF, PQXDH, ratchet chains vs. `libsignal`; constant-time review of header trial-decrypt and KEM handling.

#### Technical Specifications
- **Cryptographic primitives:** HKDF-SHA256 (sealed-sender envelope, reused from `async/obfs.go`); Ed25519 (device-list signatures); SHA-512×5200 (safety numbers, existing).
- **Key management:** per-device session keys zeroized on removal; device list re-signed on change.
- **Protocol flow:** authenticate device list → per-device session init → per-device encrypt.
- **Edge cases:** stale device list, partial-device delivery, removed-device replay.

#### API Impact
**New APIs:**
```
func SealSender(envelope SenderCert, recipientIdentity [32]byte) ([]byte, error)
func OpenSender(sealed []byte, self IdentityKeyPair) (SenderCert, error)
type DeviceList struct { Devices []DeviceBundle; Signature [64]byte }
```
**Modified APIs:** session creation accepts a device target; negotiation gains sealed-sender + multi-device capability bits.
**Deprecated:** none.

#### Testing Requirements
- **Security tests:** transport peer cannot derive sealed sender identity; removed devices lose access; KAT bit-match.
- **Integration tests:** multi-device fan-out delivery; device add/remove lifecycle.
- **Performance tests:** per-device encryption scales linearly within budget.

#### Dependencies
- **Requires:** 3.1 X3DH (multi-device fan-out), `async/obfs.go` (sealed-sender primitives), `crypto/safety_number.go`.
- **Blocks:** none.
- **External:** `govulncheck`; reference `libsignal` vectors (test-only).

---

## 4. Validation Strategy

### 4.1 Security Testing
- **Property verification:** forward secrecy (compromise at message N never decrypts 1..N-1), post-compromise security (recovery after K DH steps), header confidentiality, KCI resistance, downgrade resistance for header-encryption and PQ capabilities.
- **Test vectors:** Signal `libsignal` Double Ratchet + X3DH KATs; FIPS 203 ML-KEM-768 KATs; RFC 5869 HKDF and RFC 8439 ChaCha20-Poly1305 KATs.
- **Attack scenarios:** malicious relay (over-count payload, AUDIT M-1), MITM version/PQ/header downgrade, replayed initial X3DH message, forged signed pre-key, one-time-pre-key reuse, sealed-sender spoofing.
- **Fuzzing:** extend `crypto/crypto_fuzz_test.go`-style harnesses to header parsing, X3DH/PQXDH initial-message parsing, and the bounded relay decoder; continuous fuzzing in CI on the parsing surfaces.

### 4.2 Cryptographic Validation
- **Algorithm compliance:** verify X25519/Ed25519/HKDF/ChaCha20-Poly1305 against RFC vectors and ML-KEM against FIPS 203; assert HKDF info-string and transcript-ordering correctness.
- **Known-answer tests:** cross-check ratchet chains, X3DH `SK`, and header-key derivation against reference outputs (`crypto/testvectors_test.go` pattern).
- **Side-channel analysis:** constant-time header trial-decryption (no secret-dependent branch on current-vs-next key), constant-time AEAD failure, `subtle`-based comparisons, KEM constant-time decapsulation; verify key zeroization (`crypto.ZeroBytes`/`SecureWipe`) on all new paths.

### 4.3 Integration Testing
- **End-to-end flows:** offline-first X3DH/PQXDH message; full ratchet exchange with encrypted headers; multi-device fan-out; sealed-sender real-time message.
- **Error conditions:** SPK-signature failure, KEM decap failure, oversize relay payload, capability mismatch, device-list staleness.
- **Backward compatibility:** legacy↔legacy, legacy↔Noise-IK, Noise-IK↔Noise-IK, plaintext-header↔encrypted-header, classical↔PQ — all deterministic and non-breaking (extend the existing compatibility matrix).
- **Performance benchmarks:** per-message latency/throughput and per-session memory under secure defaults (extend `toxcore_benchmark_test.go`).

### 4.4 External Audit
- **Pre-audit requirements:** Phase 1 + Phase 2 complete; KATs green; threat model updated; RC branch frozen (per existing audit-prep practice).
- **Audit scope:** X3DH/PQXDH transcript, header-encryption KDF/state machine, sealed sender, multi-device device-list authentication, negotiation/downgrade protection, key lifecycle/zeroization.
- **Post-audit process:** track findings in the public remediation table with severity/SLA (existing `SECURITY.md` process); resolve all critical/high before release.

---

## 5. Risk Management

### Technical Risks
| Risk | Mitigation |
|------|-----------|
| Header-encryption state-machine bugs break out-of-order/DH-step handling | Reuse existing skipped-key logic; exhaustive ordering/step integration tests + KATs before enabling by default |
| ML-KEM-768 increases bundle/handshake size on mobile/embedded | Capability-gate PQ; benchmark bundle size; document per-profile budgets; keep classical path |
| X3DH identity-key DH mapping (XEdDSA) implemented incorrectly | Use vetted Ed25519↔X25519 mapping; KAT against reference; constant-time review |
| New crypto dependency (ML-KEM) introduces supply-chain risk | Pin/version-lock; gate with `govulncheck` CI; prefer `golang.org/x/crypto` once stabilized |

### Security Risks During Transition
| Risk | Mitigation |
|------|-----------|
| Downgrade to plaintext-header or classical-only between upgraded peers | Carry header-encryption/PQ capability in the **signed** negotiation; require the stronger mode when both advertise it; fail closed |
| Mixed old/new sessions weaken forward secrecy during rollout | Per-session capability binding; never reuse a classical root once a hybrid is negotiated; version-commitment HMAC binds the chosen mode |
| Key compromise during migration (dual code paths) | Zeroize superseded roots/header keys immediately; minimize lifetime of transitional secrets; single centralized policy gate for allowed transitions |

---

## 6. Success Metrics

### Security Metrics
- **Gap closure:** 100% of Critical (C1–C2) and Important (I1–I3) gaps implemented and test-proven; Enhancements (E1–E3) tracked to closure.
- **Audit results:** zero unresolved critical/high findings before release.
- **Security properties:** forward secrecy, post-compromise security, header confidentiality, PQ initial-secret resistance, downgrade resistance — each independently verifiable by a dedicated test.

### Quality Metrics
- **Test coverage:** ≥ 90% line/branch for new crypto in `ratchet/`, `async/`, `crypto/`.
- **Defect density:** zero known correctness defects in crypto paths at release; all KATs green.
- **Specification alignment:** X3DH/PQXDH/Double-Ratchet (header-encryption) transcripts match the Signal specifications; documented per-construction mapping (Appendix).

### Performance Metrics
- **Latency targets:** ratchet encrypt/decrypt < 1 ms per ≤1 KiB message; header seal/open overhead < 0.2 ms; X3DH < 5 ms; ML-KEM encapsulation < 1 ms.
- **Resource limits:** per-session state within existing budgets; skipped-key cache capped at `MaxSkippedKeys = 1000` (`ratchet/ratchet.go:21`).
- **Throughput requirements:** < 5% throughput regression versus the current ratchet under secure defaults.

---

## 7. References

### Signal Protocol Documentation
- Double Ratchet specification — https://signal.org/docs/specifications/doubleratchet/ (incl. header-encryption variant)
- X3DH specification — https://signal.org/docs/specifications/x3dh/
- PQXDH specification — https://signal.org/docs/specifications/pqxdh/
- Sesame (multi-device) specification — https://signal.org/docs/specifications/sesame/
- XEdDSA / VXEdDSA — https://signal.org/docs/specifications/xeddsa/
- libsignal reference implementation — https://github.com/signalapp/libsignal

### Cryptographic Standards
- RFC 7748 — Elliptic Curves for Security (X25519)
- RFC 8032 — EdDSA (Ed25519)
- RFC 5869 — HKDF
- RFC 8439 — ChaCha20 and Poly1305 (with XChaCha20 extension)
- NIST FIPS 203 — ML-KEM (Module-Lattice KEM, Kyber)
- NIST SP 800-38D — AES-GCM (async payload AEAD)
- Noise Protocol Framework — https://noiseprotocol.org/noise.html (IK pattern)

---

## Appendix: Cryptographic Primitives Mapping

| Function | Signal Standard | Current Implementation | Action Required |
|----------|-----------------|------------------------|-----------------|
| Initial key agreement | X3DH (4×X25519 DH) | Noise-IK / static ECDH seed (`ratchet/session.go:73`) | Add — explicit X3DH transcript, capability-gated; legacy seeding retained (C1) |
| Post-quantum agreement | PQXDH (X25519 + ML-KEM-768) | None (classical only) | Add — hybrid KEM (I1) |
| Message ratchet | Double Ratchet (HKDF root, HMAC chain) | Double Ratchet, HKDF-SHA256 root + HMAC-SHA256 chain (`ratchet/ratchet.go:31,49`) | None — spec-aligned |
| Header protection | Header encryption (separate header keys) | Plaintext header AD (`ratchet/header.go`) | Add — header encryption (C2) |
| Message encryption | AEAD (AES-GCM / ChaCha20-Poly1305) | XChaCha20-Poly1305 (`ratchet/ratchet.go:74`); AES-256-GCM async (`async/obfs.go`) | None — AEAD equivalent |
| Key derivation | HKDF-SHA256 | HKDF-SHA256 (`ratchet/ratchet.go:35`, `async/obfs.go`) | None |
| Authentication / MAC | HMAC-SHA256 | HMAC-SHA256 chain + version-commitment HMAC (`transport/version_commitment.go`) | None |
| Key exchange (DH) | X25519 | X25519 (`crypto/shared_secret.go:34`) | None |
| Signatures | XEdDSA / Ed25519 | Ed25519 (`crypto/ed25519.go`); pre-key signatures (`async/prekeys.go`) | Upgrade — add XEdDSA mapping for identity-key DH (C1) |
| Identity verification | Safety numbers (iterated hash) | SHA-512 ×5200 safety numbers (`crypto/safety_number.go`) | None — already Signal-aligned |
| Identity checksum | Cryptographic fingerprint | 16-bit XOR ToxID checksum (`crypto/toxid.go:130`) | Document — rely on safety numbers, mark XOR typo-only (E2) |
| Sender metadata | Sealed sender | Async epoch pseudonyms (`async/obfs.go`); real-time exposes sender | Add — real-time sealed sender (I3) |
| Multi-device | Sesame | Single session per identity | Add — per-device sessions (E1) |
