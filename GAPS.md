# Implementation Gaps — 2026-06-03

This document records gaps between what `toxcore-go` **claims** (in `README.md`,
`SECURITY.md`, package GoDoc, and the libtoxcore-compatible C API contract) and what the
current code actually does. It complements `AUDIT.md`, which records concrete findings with
file/line references and severities; finding IDs below (e.g. `H-6`, `M-1`) cross-reference
that report.

A prior `GAPS.md` documented the async `gob` allocation DoS and the UPnP `controlURL`
SSRF; both were re-checked and found **remediated** in the current tree
(`async/client.go:1334-1357`, `transport/upnp_client.go:314`). The gaps below are those that
remain open.

## Gap 1 — The C API advertises libtoxcore compatibility but never delivers core callbacks

- **Stated Goal:** `README.md` lists "C API Bindings — libtoxcore-compatible C function
  exports for toxcore and ToxAV" as a headline feature, and `capi/doc.go` documents the
  exported callback registration functions.
- **Current State:** `tox_callback_friend_request`, `tox_callback_friend_message`,
  `tox_callback_friend_connection_status` (and the conference/file callbacks) store the C
  function pointer but the Go-side handler only logs at `Debug` level — it never invokes the
  registered C callback (`capi/toxcore_c.go:694-706,723-736,752-765`; some sites carry a
  literal "Would need to call C callback here" comment). Only the ToxAV callbacks
  (`capi/toxav_c.go`) bridge correctly. (Tracked as `AUDIT.md` finding **H-6**.)
- **Impact:** A C/C++ program linking the shared library receives no friend requests,
  incoming messages, or connection-status changes — the toxcore half of the C API is
  effectively receive-blind, so any non-trivial C client cannot function.
- **Closing the Gap:** Add cgo bridge invokers mirroring the working `toxav_c.go` pattern that
  call each stored function pointer with its registered user-data, and add a C round-trip test
  (`go build -buildmode=c-shared ./capi`).

## Gap 2 — Noise-IK "downgrade protection" via version commitment is wired but inert

- **Stated Goal:** `README.md` ("Noise-IK Handshakes", "Protocol Version Negotiation") and
  the security warning about `EnableLegacyFallback` imply that protocol-version selection is
  protected against rollback/downgrade. The code carries an explicit version-commitment
  exchange and a post-handshake `versionCommitted` rollback check for this purpose.
- **Current State:** The encrypted commitment packet is never routed to its verifier — the
  handler is registered on the underlying transport instead of the decrypted-packet handler
  map, so `handleEncryptedPacket` drops it (`transport/noise_transport.go:261,911-923`).
  Independently, the commitment MAC is keyed with each side's *local* random handshake nonce
  rather than shared transcript state, so the two sides could never agree even if routing were
  fixed (`noise_transport.go:845-846`). `session.versionCommitted` therefore never becomes
  true. (Tracked as `AUDIT.md` findings **M-1**, **M-2**.)
- **Impact:** The advertised anti-downgrade signal is dead code. An active attacker able to
  influence version negotiation would not be detected by this mechanism. Practical exposure is
  bounded because default capabilities disable legacy fallback, but the documented protection
  does not exist as implemented.
- **Closing the Gap:** Register the commitment handler in the decrypted-packet handler map (or
  dispatch directly to the verifier), key the commitment to `GetChannelBinding()` shared
  transcript state, and add a test asserting `IsVersionCommitted()` is true after a handshake.

## Gap 3 — `SelfGetConnectionStatus` claims to report connectivity but is hard-wired to offline

- **Stated Goal:** The libtoxcore contract (and GoDoc) for `tox_self_get_connection_status` /
  `SelfGetConnectionStatus()` is to report whether the instance has UDP/TCP connectivity to the
  network.
- **Current State:** `t.connectionStatus` is set once to `ConnectionNone` at construction
  (`toxcore.go:598`) and never updated, so the getter and its C export always return "offline"
  (`toxcore_self.go:69-70`). (Tracked as `AUDIT.md` finding **M-8**.)
- **Impact:** Applications (and C clients) that gate behavior on self-connection status — UI
  indicators, retry/backoff, "wait until online" loops — observe a permanently-offline node
  even when fully connected.
- **Closing the Gap:** Update `t.connectionStatus` on bootstrap/transport state transitions and
  fire the connection-status callback; add a unit test asserting the transition after
  bootstrap.

## Gap 4 — Async identity key rotation is offered but breaks message receipt

- **Stated Goal:** `async/key_rotation_client.go` documents identity key rotation, including
  `EmergencyRotateIdentity` "for when key compromise is suspected", with a callback to re-key
  Noise sessions.
- **Current State:** Rotation updates `ac.keyPair` but not the obfuscation manager's stored
  key, and pseudonym/recipient-proof validation is computed from that stale key
  (`async/obfs.go:388,403`). After rotation, offline messages addressed to the new identity are
  retrieved and then discarded as "not intended for this recipient". (Tracked as `AUDIT.md`
  finding **M-5**.)
- **Impact:** The compromise-recovery feature silently disables offline message receipt for the
  new identity — the worst possible time for messages to vanish without error.
- **Closing the Gap:** Add `ObfuscationManager.UpdateKeyPair`, call it from both rotation paths,
  and retrieve/validate across all active identities (`GetAllActiveIdentities`).

## Gap 5 — "Untrusted relays" threat model vs. first-use prekey trust

- **Stated Goal:** `SECURITY.md` scopes "pre-key bundle spoofing" and "authentication bypasses"
  as in-scope vulnerabilities, and the async design treats storage/DHT nodes as untrusted.
- **Current State:** Prekey-DHT bundles are pinned by `SigningPK` only *after* first contact;
  on first use any self-consistent bundle for a victim `OwnerPK` is accepted, with no
  cryptographic binding between the Curve25519 `OwnerPK` and the Ed25519 `SigningPK`
  (`async/prekey_dht.go:398-410`). (Tracked as `AUDIT.md` finding **M-6**.)
- **Impact:** A malicious DHT/relay node can poison a victim's prekeys before the victim's
  correspondents have pinned a signing key, enabling a first-contact key-substitution attack —
  exactly the "pre-key bundle spoofing" class the policy claims to defend.
- **Closing the Gap:** Bind `OwnerPK` to `SigningPK` (e.g. require `OwnerPK == X25519(SigningPK)`
  when seeds are shared) or require an authenticated channel to establish the pin before
  accepting any bundle.

## Gap 6 — Group replay protection is documented but defeatable via a sentinel-counter collision

- **Stated Goal:** `README.md` and `group/` describe sender-key distribution with monotonic
  per-sender message counters for replay protection.
- **Current State:** The "no message yet" state is encoded as the in-band sentinel `^uint64(0)`,
  which is also a legal wire counter; a message carrying that counter is accepted without a
  replay check and re-arms the sentinel, permanently disabling replay detection for that
  sender/key epoch (`group/sender_key.go:356-360,435,452-454`). (Tracked as `AUDIT.md` finding
  **M-7**.)
- **Impact:** Any party holding the distributed sender key can defeat replay protection for the
  group, contradicting the stated guarantee.
- **Closing the Gap:** Represent first-message state out-of-band (a `seenFirstMessage bool` or a
  nullable counter) so no legal counter value disables the check.

## Gap 7 — Documented concurrency-safety contracts are not uniformly honored

- **Stated Goal:** Several types document their own locking contracts — `dht.Node.mu` "Protects
  concurrent access to mutable fields", `messaging` documents an explicit `message.mu`-before-
  `mm.mu` lock order and supports concurrent `ProcessPendingMessages`, and `toxnet` advertises a
  strict `RequireEncryption` mode.
- **Current State:** DHT maintenance reads/writes `Node.Status`/`LastSeen` without `Node.mu`
  and recursively `RLock`s a k-bucket (`AUDIT.md` **H-1**, **H-2**); `messaging` cleanup inverts
  the documented lock order (`H-3`); file-transfer cancel/complete races double-fire callbacks
  (`H-4`); ToxAV media teardown can nil-deref a processor mid-decode (`H-5`); and `toxnet`
  strict-mode is read outside a single critical section (`M-12`).
- **Impact:** Under concurrency these can deadlock the DHT maintenance loop, deadlock the
  message manager, crash a media goroutine, corrupt cached frames, or briefly admit plaintext —
  none caught by the current `-race` suite because the racing paths are not exercised together.
- **Closing the Gap:** Add locked accessors / single-snapshot reads as described per finding,
  and add targeted `-race` stress tests that run maintenance, teardown, and cleanup concurrently
  with their writers to convert these traced findings into regression-guarded fixes.

## Gap 8 — "Experimental / un-audited" status is documented only in SECURITY.md

- **Stated Goal:** `SECURITY.md` → "External Audit Status" states no third-party audit has been
  performed and the library should be treated as experimental and unsuitable for high-stakes
  production use until audited.
- **Current State:** This caveat lives only in `SECURITY.md`; the `README.md` feature list and
  package GoDoc (`doc.go`) present the cryptography as production-ready without surfacing the
  un-audited status where a developer first integrates the library.
- **Impact:** Integrators may deploy the library in a threat model it has not been validated
  for, assuming the rich crypto feature set implies external assurance.
- **Closing the Gap:** Surface the "experimental / pending third-party audit" notice in the
  `README.md` security/usage section and in the package GoDoc, linking to `SECURITY.md`.
  (Documentation change only.)
