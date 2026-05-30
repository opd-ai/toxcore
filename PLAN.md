# toxcore-go → Signal Protocol Parity: Security Improvement Checklist

## ⚠️ CRITICAL COMPATIBILITY REQUIREMENT
**All security improvements in this plan must maintain full backward compatibility with:**
- **Classic Tox** (original unencrypted protocol)
- **Tox-with-NoiseIK** (current Noise-IK implementation)

**Our security goal:** Drastically improve Tox security so that existing Tox users may seamlessly upgrade to a more secure version without breaking interoperability or requiring simultaneous client updates.

**Implementation strategy:** Security enhancements should be introduced as negotiated upgrades that allow mixed-version networks to coexist. Deprecated features must be kept functional throughout multiple release cycles to allow graceful migration paths.

---

Priority levels: 🔴 Critical  🟠 High  🟡 Medium  🟢 Low

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## PHASE 1 — Core Cryptographic Gaps (Real-Time Sessions)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

### 1.1 Implement a Double Ratchet for Live Chat Sessions 🔴
The biggest single gap. Currently, the Noise-IK cipher state established
in transport/noise_transport.go is reused for all messages in a session
until 2^32 messages or a manual re-handshake. Signal derives a new
message key for every single message.

  [x] Design a `ratchet` package implementing the Double Ratchet Algorithm
      (Signal spec: https://signal.org/docs/specifications/doubleratchet/)
      - KDF Chain (symmetric ratchet): HKDF with SHA-256 to derive a new
        message key from the current chain key on every send/receive
      - DH Ratchet step: inject a new X25519 ephemeral DH output into the
        root chain whenever the other party sends a new ratchet key
      - Maintain separate sending and receiving chain states

  [x] Implement root-chain, sending-chain, and receiving-chain KDF functions
      using golang.org/x/crypto/hkdf (already a transitive dep)

  [x] Integrate into the live-chat message path in messaging/message.go,
      replacing the static shared-secret encryption at encryptMessage()

  [x] Implement skipped-message-key store (bounded; max 1000 skipped keys)
      to handle out-of-order delivery without breaking the ratchet

  [x] Delete each message key immediately after single use; never reuse

  [x] Add per-ratchet-step unit tests verifying:
      - Compromise of message key N does not expose key N-1 or N+1
      - After a DH ratchet step, prior cipher states cannot be recomputed
      - Out-of-order messages decrypt correctly via the skipped-key store


### 1.2 Reduce Re-Handshake Threshold for Interim Protection 🟠
Until a full Double Ratchet is implemented, lower the current
DefaultRekeyThreshold (2^32 messages, transport/noise_transport.go) to
force re-keying far more aggressively, limiting the blast radius of a
compromised session cipher state.

  [x] Set DefaultRekeyThreshold to a much lower value, e.g. 100–500
      messages, giving approximate per-hundred-message forward secrecy
      without the full Double Ratchet architecture

  [x] Add a time-based rekey trigger in addition to the counter-based one:
      force re-handshake after N minutes of inactivity or M minutes
      elapsed since last handshake, whichever comes first

  [x] Expose rekeyThreshold as a named constant with a comment explaining
      it is a temporary measure pending full Double Ratchet implementation


### 1.3 Enforce Mandatory Use of ObfuscatedAsyncMessage 🟠
The base ForwardSecureMessage struct (async/forward_secrecy.go:19)
carries SenderPK [32]byte as a plaintext field, leaking the real sender
public key on the wire. Applications may use this path without realizing
they have bypassed the identity-obfuscation layer.

  [x] Deprecate or remove direct use of ForwardSecureMessage from the
      public API surface; make it an internal type

  [x] Expose only ObfuscatedAsyncMessage via the public async-messaging
      API so that sender anonymity is always enforced by default

  [x] Add a compile-time or runtime guard that panics (or returns an error)
      if ForwardSecureMessage is sent without first wrapping it in
      ObfuscatedAsyncMessage

  [x] Update all examples (examples/async_demo/main.go) to demonstrate
      only the obfuscated code path


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## PHASE 2 — Authentication & Trust Model
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

### 2.1 Add a Safety-Number / Key-Fingerprint Primitive 🟠
Signal displays a 60-digit "Safety Number" derived from both parties'
identity keys. toxcore-go has no built-in fingerprint comparison API,
leaving MITM detection entirely to application developers.

  [x] Implement a SafteyNumber(myPK, peerPK [32]byte) string function in
      the crypto package that produces a human-readable, versioned
      fingerprint (e.g., 12 groups of 5 decimal digits, consistent with
      Signal's derivation using SHA-512 over both public keys)

  [x] Expose the function via the public Tox API and in the toxnet package
      so all transport types have access to it

  [x] Document clearly that users MUST compare safety numbers out-of-band
      at least once per contact to defeat MITM attacks

  [x] Add a test vector: known inputs → known fingerprint output


### 2.2 Harden the Friend-Request Flow Against MITM 🟠
toxnet/listener.go auto-accepts friend requests by public key alone.
Without fingerprint verification, a MITM during the initial key exchange
can substitute their own public key silently.

  [x] Change toxnet/listener.go:setupCallbacks() to NOT auto-accept
      friend requests by default; require explicit application-layer
      acceptance with an opportunity to display and verify the safety
      number before AddFriendByPublicKey is called

  [x] Add a WithManualAccept() option to ToxListener so auto-accept is
      opt-in rather than opt-out

  [x] Provide example code showing a correct friend-accept flow that
      includes safety-number display and confirmation


### 2.3 Signed Pre-Key Bundle (X3DH Parity) 🟡
Signal's X3DH uses a signed pre-key (SPK) — a medium-term key that is
Ed25519-signed by the identity key — in addition to one-time pre-keys.
This binds the pre-key bundle to the identity, preventing a storage node
from substituting a bogus pre-key bundle.

  [x] Add a SignedPreKey type to async/prekey.go: a Curve25519 key pair
      whose public key is signed by the owner's Ed25519 identity key

  [x] Include the signature and signer public key in PreKeyExchangeMessage
      (async/forward_secrecy.go:29)

  [x] Verify the signature in ProcessPreKeyExchange before storing the
      bundle; reject bundles with invalid signatures

  [x] Rotate the signed pre-key on a schedule (Signal rotates weekly)


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## PHASE 3 — Privacy & User-Facing Security Features
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

### 3.1 Per-Conversation Disappearing Messages for Live Chat 🟡
MaxStorageTime (24h) in async/storage.go applies only to storage-node
TTL for async messages. There is no user-configurable disappearing
message timer for live real-time conversations. Signal provides timers
from 30 seconds to 4 weeks.

  [x] Add a DisappearingMessageConfig struct to the messaging package:
        - Timer: time.Duration (e.g., 30s, 5m, 1h, 1d, 1w)
        - Enabled: bool
        - SetAt: time.Time (for synchronisation)

  [x] Store per-conversation timer in the Friend struct in toxcore_friends.go

  [x] On receipt of a message with disappearing mode enabled, schedule
      a time.AfterFunc to zero/delete the message from local storage

  [x] Sync the timer setting to the peer so both sides delete at the same
      time (include timer value in a control message type)

  [x] Test edge cases: timer change mid-conversation, peer offline when
      timer fires, device restart before timer fires


### 3.2 Pre-Key Pool Exhaustion Resilience 🟡
When peerPreKeys drops below PreKeyMinimum (5 keys, async/forward_secrecy.go:71)
async messaging is blocked. A targeted DoS consuming pre-keys could
silence a user.

  [ ] Increase PreKeyMinimum from 5 to at least 20 to give more headroom
      between the low-watermark refresh trigger and actual exhaustion

  [ ] Implement a hard limit on how quickly a single peer can consume
      pre-keys (rate-limit pre-key consumption per sender public key)

  [ ] Add monitoring/alerting hook: fire an event when pool drops below
      PreKeyLowWatermark so the application can warn the user

  [ ] Implement staggered pre-key refresh: do not wait until near
      exhaustion; refresh proactively on a time schedule (e.g. weekly)
      in addition to the watermark trigger


### 3.3 Expand Message Padding to Cover Live-Chat Path Consistently 🟡
messaging/message.go pads to 3 tiers (256B / 1024B / 4096B), while
async/message_padding.go pads to 4 tiers (256B / 1024B / 4096B / 16384B).
The live-chat path drops messages over 4096B unpadded (padMessage returns
data unchanged if it exceeds all tiers, messaging/message.go:888–896).

  [ ] Align both padding implementations: add the 16384B tier to the
      messaging package's PaddingSizes slice

  [ ] Add a length-prefix encoding to the live-chat padding (matching
      the async path's LengthPrefixSize approach) so padding is
      cleanly strippable at the receiver

  [ ] Enforce a hard upper bound: reject unpadded oversized messages
      rather than sending them at their real size


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## PHASE 4 — Implementation Quality & Audit
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

### 4.1 Commission a Third-Party Cryptographic Security Audit 🔴
This is the single most impactful non-code action. Signal has been
audited by Cure53, NCC Group, and the Open Crypto Audit Project.
toxcore-go has only a self-audit (AUDIT.md, 2026-05-29).

  [ ] Engage a qualified cryptographic security firm (e.g., Cure53,
      Trail of Bits, NCC Group, Quarkslab) for a full source-code audit
      focusing on: Double Ratchet integration (when complete), Noise
      handshake state machine, pre-key management, epoch/obfuscation
      math, and memory-wiping correctness in Go's GC environment

  [ ] Publish the audit report in full (no redacted version) in the repo
      under docs/AUDIT_EXTERNAL_<FIRM>_<YEAR>.md

  [ ] Address all Critical and High findings before the v2.0 release tag

  [ ] Schedule follow-up audits at major version increments


### 4.2 Publish a Standalone Protocol Specification 🟠
Signal Protocol has a formal, self-contained specification separate from
the code. toxcore-go's security properties are documented across multiple
internal docs (docs/ASYNC.md, docs/OBFS.md, docs/FORWARD_SECRECY.md)
but there is no unified specification document suitable for external
academic review.

  [ ] Author a single docs/PROTOCOL_SPEC.md covering: identity model,
      key types, handshake flows, forward-secrecy mechanism, group
      messaging, metadata obfuscation, and wire formats

  [ ] Include security proofs or informal security arguments for each
      major component (or references to the underlying formal proofs
      for Noise Protocol and X3DH)

  [ ] Submit the specification to an academic venue or pre-print server
      (e.g., IACR ePrint) for community review


### 4.3 Harden Go-Specific Memory Security Limitations 🟠
Go's GC can copy heap objects; mlock(2) is unavailable in pure Go,
meaning key material can appear in swap. Signal (Rust/libsignal) uses
explicit stack allocation and mlock for key buffers.

  [ ] Evaluate cgo-based mlock wrappers (e.g., golang.org/x/sys/unix.Mlock)
      for locking key-material pages in physical RAM on Linux/macOS;
      add a build tag (+cgo) guard so the pure-Go path is preserved

  [ ] Add a SecureAllocate(size int) function in crypto/secure_memory.go
      that allocates a byte slice from a mlock'd memory region where
      available, falling back to standard allocation where not

  [ ] Document clearly in the security policy that deploying toxcore-go
      on systems with swap enabled reduces key secrecy guarantees and
      recommend encrypted swap or swapoff for sensitive deployments


### 4.4 Establish a Formal Vulnerability Disclosure Policy 🟡

  [ ] Add a SECURITY.md at the repository root specifying:
      - How to report vulnerabilities (encrypted email, GitHub private
        advisory, or equivalent)
      - Scope of the program
      - Expected response SLA (e.g., acknowledgement within 48h,
        fix within 90 days for Critical findings)
      - CVE assignment process (CNA or MITRE direct)

  [ ] Consider a public bug-bounty program (HackerOne, Immunefi) once
      the external audit is complete and the codebase is stabilised


### 4.5 Add Protocol-Level Test Vectors 🟡
Signal publishes known-answer test vectors for its cryptographic
operations. toxcore-go's tests are property-based and fuzz-based but
lack fixed test vectors for the core protocol constructs.

  [ ] Add test vectors for:
      - Noise-IK handshake: fixed inputs → fixed handshake transcript
        and derived cipher states (cross-check against the official
        Noise test-vector suite)
      - Pre-key encryption/decryption: fixed key bundle → fixed ciphertext
      - Epoch pseudonym derivation: fixed PK + epoch → fixed pseudonym
      - Message padding: fixed plaintext → fixed padded output

  [ ] Include these in a dedicated crypto/testvectors_test.go file so
      they are run on every CI build


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## PHASE 5 — Ecosystem & Operational Parity
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

### 5.1 Multi-Language Bindings (Mobile Parity) 🟡
Signal has official libsignal bindings for Swift (iOS), Kotlin (Android),
and TypeScript (web). toxcore-go has only the optional C bindings (capi).

  [ ] Design a stable C ABI layer (building on capi) that can be consumed
      by Swift/Kotlin via FFI without requiring CGo in the consuming app

  [ ] Publish a toxcore-swift and toxcore-kotlin thin wrapper library
      using the C ABI, covering: key generation, Noise handshake,
      forward-secure messaging, and safety-number display

  [ ] Ensure the C ABI exports all security-critical operations
      (SecureWipe, key generation, safety number) so bindings do not
      need to reimplement cryptographic primitives


### 5.2 Key Rotation Period — Tighten the Default 🟢
KeyRotationManager defaults to a 30-day RotationPeriod
(crypto/key_rotation.go:51). Signal's signed pre-key is rotated weekly.
A compromised long-term identity key has a 30-day validity window.

  [ ] Reduce the default RotationPeriod from 30 days to 7 days to match
      Signal's signed-pre-key rotation cadence

  [ ] Expose a user-facing API for manual EmergencyRotation() that is
      easy for applications to wire to a "Reset Identity" UI action

  [ ] Ensure that after identity key rotation, all active Noise sessions
      are renegotiated with the new key within one round-trip


### 5.3 Increase Pre-Key Pool Size and Refresh Buffer 🟢
Current PreKeysPerPeer is 100 (async/doc.go, docs/ASYNC.md).
Signal maintains 100 one-time pre-keys on its server; the difference is
that Signal's server-held model allows replenishment without requiring
both peers online simultaneously.

  [ ] Increase PreKeysPerPeer to 200 and PreKeyRefreshThreshold to 50
      to reduce the window of pre-key exhaustion under heavy messaging

  [ ] Implement a pre-key backup/restore mechanism so a user restoring
      from backup does not immediately exhaust the peer's pre-key pool


### 5.4 Cover Traffic for Live P2P Sessions 🟢
docs/COVER_TRAFFIC.md acknowledges that the direct P2P channel between
two online peers is not protected by cover traffic (only async retrieval
has cover traffic via RetrievalScheduler). A network observer can see
exactly when two peers are communicating.

  [ ] Implement transport-layer dummy packet injection for live Noise
      sessions: send randomly-timed zero-payload encrypted packets to
      obscure real message timing for active conversations

  [ ] Make the dummy-packet rate configurable per-session so latency-
      sensitive callers can disable it while privacy-sensitive ones
      can enable aggressive cover traffic


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## SUMMARY — Priority Order
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  🔴  1.1  Double Ratchet for real-time sessions          (Phase 1)
  🔴  4.1  Third-party cryptographic audit                (Phase 4)
  🟠  1.2  Lower rekey threshold (interim protection)     (Phase 1)
  🟠  1.3  Enforce ObfuscatedAsyncMessage exclusively     (Phase 1)
  🟠  2.1  Safety Number / fingerprint primitive          (Phase 2)
  🟠  2.2  Harden friend-request MITM surface             (Phase 2)
  🟠  4.2  Publish standalone protocol specification      (Phase 4)
  🟠  4.3  Harden Go memory (mlock investigation)         (Phase 4)
  🟡  2.3  Signed pre-key bundle (X3DH parity)            (Phase 2)
  🟡  3.1  Disappearing messages for live chat            (Phase 3)
  🟡  3.2  Pre-key pool exhaustion resilience             (Phase 3)
  🟡  3.3  Align padding tiers across live/async paths    (Phase 3)
  🟡  4.4  Formal vulnerability disclosure policy         (Phase 4)
  🟡  4.5  Protocol-level test vectors                    (Phase 4)
  🟡  5.1  Mobile language bindings (Swift/Kotlin)        (Phase 5)
  🟢  5.2  Tighten key rotation default to 7 days         (Phase 5)
  🟢  5.3  Increase pre-key pool size and refresh buffer  (Phase 5)
  🟢  5.4  Live-session cover traffic (dummy packets)     (Phase 5)
