# Security Audit Checklist — toxcore-go

> Generated 2026-04-06. All checks map to verifiable code-level properties with `file:line` references.

## Cross-Cutting Constraint: Backward Compatibility & Downgrade Resistance

**Requirement**: All protocol extensions (Noise-IK, S/Kademlia, async messaging, message padding, identity obfuscation, PSK resumption, multi-network transports) must be backward-compatible with the existing Tox network (c-toxcore and rstox peers). Once a peer upgrades, the implementation must resist downgrade attacks that force fallback to weaker protocol versions.

**Key principle**: Once two upgraded peers have successfully negotiated Noise-IK, neither should ever silently fall back to Legacy/unencrypted communication. Downgrade must require explicit user action, not merely a network disruption or attacker interference.

This constraint applies to **every** audit category below.

---

## Category 0 — Protocol Extension Compatibility & Downgrade Resistance

- [ ] **0.1 — Verify `EnableLegacyFallback` is disabled by default**
  - File: `transport/negotiating_transport.go:37`
  - Expected: `EnableLegacyFallback: false` in `NewNegotiatingTransport()` constructor so Noise-IK peers never silently downgrade.
  - Pitfall: If this defaults to `true`, an active attacker who blocks Noise handshake packets forces all traffic onto unencrypted legacy transport.
  - Verify: Read the constructor; confirm the default is `false`.

- [ ] **0.2 — Eliminate silent unencrypted fallback in noise_transport.go Send()**
  - File: `transport/noise_transport.go:314–320`
  - Expected: When a Noise handshake fails, `Send()` must return an error rather than silently transmitting via the unencrypted underlying transport.
  - Pitfall: Current code falls back to `nt.underlying.Send(packet, addr)` on handshake failure — this is a **critical downgrade vulnerability** where an attacker blocking handshake packets causes all subsequent messages to be sent in cleartext without any application-layer notification.
  - Verify: Read the `Send()` method; confirm that failed handshake returns an `error`, not a fallback send.

- [ ] **0.3 — Enforce version commitment binding on all Noise sessions**
  - File: `transport/version_commitment.go:16–19, 47–64, 128–163`
  - Expected: `CreateVersionCommitment()` is called on every handshake and `VerifyVersionCommitment()` is verified by both sides before data exchange begins.
  - Pitfall: If version commitment exchange is optional or skipped on some code paths, an attacker can replay an older version negotiation to force Legacy protocol.
  - Verify: Trace handshake flow in `versioned_handshake.go:451–470` `HandleHandshakeRequest()`; confirm `VerifyVersionCommitment()` is mandatory.

- [ ] **0.4 — Prevent permanent downgrade via protocol pinning**
  - File: `transport/negotiating_transport.go:243–247, 275–280`
  - Expected: If a peer is pinned to `ProtocolLegacy` via `SetPeerVersion()`, there must be a mechanism to re-negotiate after a timeout or on the next connection.
  - Pitfall: Once `setPeerVersion(addr, ProtocolLegacy)` is called, the peer is permanently downgraded with no recovery — an attacker who causes one negotiation failure permanently degrades the connection.
  - Verify: Check if `SetPeerVersion` has a TTL or re-negotiation trigger; currently it has **neither**.

- [ ] **0.5 — Validate signed version negotiation packets**
  - File: `transport/version_negotiation.go:108–174`
  - Expected: `SerializeSignedVersionNegotiation()` signs the version list with Ed25519; `ParseSignedVersionNegotiation()` verifies the signature before accepting the peer's version capabilities. Signature verification must be mandatory (not gated behind an optional flag).
  - Pitfall: If signature verification is skipped, an attacker can forge a version negotiation response claiming the peer only supports Legacy.
  - Verify: Read `ParseSignedVersionNegotiation()` line 154–161; confirm Ed25519 signature verification is mandatory and failure aborts negotiation.

- [ ] **0.6 — Verify extension packet types don't conflict with c-toxcore**
  - File: `transport/packet.go:128–153`, `transport/packet_extensions.go:67–74`
  - Expected: Extension packet types 249–254 with vendor magic `0xAB` are in a range that c-toxcore ignores (silently drops unknown types). Note that type 255 (`PacketRelayQueryResponse`) is also used.
  - Pitfall: If c-toxcore interprets any of these byte values as valid packet types, it will misparse traffic. Also, using 255 may collide with special sentinel values in some implementations.
  - Verify: Cross-reference packet type values against c-toxcore `network.h` and confirm 249–255 are unused/reserved in standard Tox.

- [ ] **0.7 — Confirm S/Kademlia is optional for DHT interop**
  - File: `dht/skademlia.go:89–92, 105`, `dht/routing.go:322–325`
  - Expected: `RequireProofs` defaults to `false`, and `AddNode()` accepts nodes without `NodeIDProof`. Standard Tox nodes without S/Kademlia proof-of-work can join the routing table.
  - Pitfall: If S/Kademlia proof is mandatory, this implementation cannot participate in the standard Tox DHT, causing a network partition.
  - Verify: Read `SKademliaConfig` struct; confirm `RequireProofs: false` is the default. Read `AddNode()` in `routing.go`; confirm no proof is required at that level.

- [ ] **0.8 — Verify async extension messages are invisible to legacy peers**
  - File: `async/` package, `transport/packet_extensions.go:14–54`
  - Expected: Async/offline messaging packets are either: (a) encapsulated inside standard Tox encrypted payloads so legacy peers ignore them, or (b) use extension packet types (249–254) that legacy peers silently drop.
  - Pitfall: Async messages sent as raw packets to a c-toxcore peer cause connection reset or misparse.
  - Verify: Trace async message send path; confirm encapsulation or extension-type usage.

---

## Category 1 — Cryptographic Implementation

- [ ] **1.1 — Verify Noise-IK pattern uses correct role assignment**
  - File: `noise/handshake.go:66–127`
  - Expected: Initiator and responder roles are determined by a stable, non-manipulable criterion (e.g., lower public key initiates). Noise IK pattern requires the initiator to know the responder's static key.
  - Pitfall: If both sides can claim initiator role, the Noise handshake fails or creates a symmetric state that an attacker can exploit.

- [ ] **1.2 — Verify Noise-IK sessions coexist with NaCl encryption**
  - File: `noise/handshake.go`, `transport/noise_transport.go`, `crypto/`
  - Expected: When communicating with a legacy peer, standard NaCl (Curve25519 + XSalsa20-Poly1305) is used; when communicating with an upgraded peer, Noise-IK (Curve25519 + ChaCha20-Poly1305) is used. Both paths must be fully functional.
  - Pitfall: Noise-IK code path works but NaCl path is stale/broken due to lack of testing.
  - Verify: Run cross-implementation test with c-toxcore peer; confirm NaCl encryption/decryption works end-to-end.

- [ ] **1.3 — Confirm key material isolation between Noise and NaCl sessions**
  - File: `noise/handshake.go`, `crypto/`
  - Expected: A Noise session key is never reused as a NaCl shared secret or vice versa.
  - Pitfall: Key confusion between the two schemes allows an attacker who compromises one session to derive the other.
  - Verify: Trace key derivation for both paths; confirm no shared key material.

- [ ] **1.4 — Verify nonce rekey threshold enforcement**
  - File: `transport/noise_transport.go` (NoiseSession counters)
  - Expected: `Encrypt()` and `Decrypt()` return `ErrRekeyRequired` when the counter reaches the threshold (default 2^32). Sessions must not continue encrypting past this point.
  - Pitfall: Counter overflow allows nonce reuse, breaking ChaCha20-Poly1305 IND-CPA security.
  - Verify: Read `Encrypt()` and `Decrypt()` methods; confirm counter check is performed before every encryption.

- [ ] **1.5 — Verify forward secrecy key material is wiped after use**
  - File: `crypto/secure_memory.go:9–46`, `async/obfs.go:158–172`, `async/forward_secrecy.go`
  - Expected: Ephemeral keys, pre-keys, and derived shared secrets are zeroed using `crypto.ZeroBytes`/`SecureWipe` after use via `defer` statements.
  - Pitfall: Key material left in memory after use is recoverable via memory dump.
  - Verify: Search for `defer.*ZeroBytes\|defer.*SecureWipe` in `async/` and `noise/` packages.

- [ ] **1.6 — Verify pre-key generation uses crypto/rand**
  - File: `async/forward_secrecy.go:42–74`
  - Expected: All pre-keys are generated using `crypto/rand.Read()`, never `math/rand`.
  - Pitfall: Predictable pre-keys allow an attacker to derive shared secrets for offline messages.

- [ ] **1.7 — Fix timing side-channel in public key comparison**
  - File: `async/client.go:1163`
  - Expected: Use `crypto/subtle.ConstantTimeCompare()` for public key comparisons instead of `bytes.Equal()`.
  - Pitfall: `bytes.Equal()` is not constant-time; timing differences leak information about key prefixes, enabling targeted attacks on identity obfuscation.
  - Verify: `grep -n "bytes.Equal.*Key\|bytes.Equal.*Public\|bytes.Equal.*PK" async/client.go`

---

## Category 2 — Protocol Compliance & Wire Compatibility

- [ ] **2.1 — Verify dual-format handshake is parseable by both peer types**
  - File: `transport/versioned_handshake.go:42–53`
  - Expected: `VersionedHandshakeRequest` includes both `NoiseMessage` and `LegacyData` fields; legacy peers parse only the legacy portion and ignore Noise data.
  - Pitfall: Legacy peers reject the packet entirely because its total length or structure doesn't match the expected format.
  - Verify: Capture wire-level handshake from this implementation and feed it to c-toxcore; confirm c-toxcore processes the legacy portion.

- [ ] **2.2 — Confirm PacketVersionNegotiation (type 249) is safely ignored by c-toxcore**
  - File: `transport/packet.go:128`
  - Expected: c-toxcore receives a type-249 packet and silently drops it without disconnecting or crashing.
  - Pitfall: Some c-toxcore versions may log errors, rate-limit, or disconnect on unknown packet types.
  - Verify: Send type-249 packet to a c-toxcore peer in a test environment; observe behavior.

- [ ] **2.3 — Validate address format compatibility**
  - File: `capi/compatibility_test.go:42–56`
  - Expected: Binary Tox address format (38 bytes) matches c-toxcore exactly. The documented discrepancy where `tox_self_get_address_size()` returns 76 (hex string length) must be resolved or clearly documented as intentional.
  - Pitfall: Address format difference breaks interop with any application that passes binary addresses.
  - Verify: Confirm the C API layer correctly translates between formats.

- [ ] **2.4 — Verify DHT query/response wire compatibility with c-toxcore**
  - File: `dht/` package
  - Expected: Standard ping, get_nodes, send_nodes packets use the exact same format as c-toxcore. No extra fields (e.g., proof-of-work data) appended to standard packets.
  - Pitfall: Extra fields appended to standard packets cause c-toxcore to reject them.
  - Verify: Compare serialized DHT packet bytes against c-toxcore wire captures.

- [ ] **2.5 — Verify SelectBestVersion() fallback behavior**
  - File: `transport/version_negotiation.go:314–330`
  - Expected: `SelectBestVersion()` returns `ProtocolLegacy` when no mutual version is found. This is correct for backward compatibility, but the caller must handle Legacy appropriately (not silently use unencrypted transport).
  - Pitfall: Fallback to Legacy without security downgrade warning allows a MITM to strip version capabilities.

- [ ] **2.6 — Verify relay packet types (253–255) don't break standard relay protocol**
  - File: `transport/packet.go:145–153`
  - Expected: `PacketRelayAnnounce` (253), `PacketRelayQuery` (254), `PacketRelayQueryResponse` (255) are only exchanged between peers that have completed version negotiation. They must not be sent to legacy relay nodes.
  - Pitfall: Legacy TCP relay nodes may misinterpret packet types 253–255, causing relay disconnection or data corruption.

---

## Category 3 — Network Security

- [ ] **3.1 — Verify Tor transport does not send extension packets outside Tor**
  - File: `transport/tor_transport.go`
  - Expected: When Tor transport is active, extension packets (249–254) are only sent over the Tor circuit, never over clearnet UDP.
  - Pitfall: Extension packets contain metadata (vendor magic, version info) that could fingerprint the user as an opd-ai/toxcore user.
  - Verify: Trace extension packet send paths; confirm they respect the active transport.

- [ ] **3.2 — Confirm version negotiation doesn't leak client implementation identity**
  - File: `transport/packet_extensions.go:59`
  - Expected: Version negotiation packets don't contain implementation name, version string, or other fingerprinting data beyond the protocol version number.
  - Pitfall: Vendor magic `0xAB` in extension packets identifies this as an opd-ai client — this is acceptable for protocol dispatch but shouldn't appear in cleartext before encryption is established.
  - Verify: Confirm extension packets are only exchanged after Noise handshake establishes encryption, OR that the vendor magic alone doesn't create a distinguishable fingerprint.

- [ ] **3.3 — Verify PSK session ticket replay protection**
  - File: `noise/psk_resumption.go:48–65`
  - Expected: `SessionTicket` includes `MessageIDCounter` for replay protection and `HandshakeHash` for binding to the original handshake. Expired tickets (past `ExpiresAt`) must be rejected.
  - Pitfall: Replayed session tickets allow an attacker to resume a session they've captured. Without handshake binding, ticket theft enables impersonation.
  - Verify: Read `ValidateTicket()` or equivalent; confirm all three checks (expiry, counter, hash) are enforced.

- [ ] **3.4 — Verify PSK ticket lifetime bounds**
  - File: `noise/psk_resumption.go:33–39`
  - Expected: `DefaultSessionTicketLifetime = 24h`, `MaxSessionTicketLifetime = 7 days`. Tickets exceeding max lifetime must be rejected.
  - Pitfall: Unbounded ticket lifetime allows long-lived session resumption, weakening forward secrecy guarantees.

- [ ] **3.5 — Verify no clearnet DNS leaks on privacy transports**
  - File: `transport/tor_transport.go`, `transport/i2p_transport.go`, `transport/nym_transport.go`
  - Expected: When using Tor/I2P/Nym transports, all DNS resolution goes through the respective privacy network. No calls to system DNS resolver.
  - Pitfall: DNS leaks reveal the destination to the local network, defeating anonymity.

---

## Category 4 — DHT & Routing Security

- [ ] **4.1 — Verify routing table accepts standard Tox nodes without S/Kademlia proof**
  - File: `dht/routing.go:322–325`, `dht/skademlia.go:89–92, 105`
  - Expected: Nodes without `NodeIDProof` are inserted into the routing table. `RequireProofs` defaults to `false`.
  - Pitfall: If S/Kademlia is mandatory, this node cannot bootstrap from the standard Tox DHT.
  - Verify: Read `AddNode()` method; confirm it doesn't require `NodeIDProof`.

- [ ] **4.2 — Verify PoW difficulty constants are reasonable**
  - File: `dht/skademlia.go:27–38`
  - Expected: `DefaultPoWDifficulty = 16` (leading zero bits), `MinPoWDifficulty = 8`, `MaxPoWDifficulty = 32`, `ProofNonceSize = 8`.
  - Pitfall: If difficulty is too low, Sybil attacks remain practical. If too high, legitimate nodes can't join. 16 bits ≈ 65536 hash attempts — this is very fast on modern hardware and may be too easy.
  - Verify: Benchmark proof generation time at difficulty 16 on target hardware.

- [ ] **4.3 — Verify DHT bootstrap nodes are compatible with standard Tox network**
  - File: `dht/bootstrap.go`
  - Expected: Bootstrap node list includes standard Tox bootstrap nodes. Handshake with bootstrap nodes uses standard Tox protocol (not Noise-IK) since they run c-toxcore.
  - Pitfall: If bootstrap attempts use Noise-IK against c-toxcore bootstrap nodes, bootstrapping fails entirely.

- [ ] **4.4 — Verify k-bucket distance calculation uses PublicKey field**
  - File: `dht/` package
  - Expected: `Node.Distance()` uses `Node.PublicKey` (top-level field), not `Node.ID.PublicKey`. Both fields must be set when creating temporary nodes.
  - Pitfall: Using the wrong field causes incorrect routing, splitting the DHT.

---

## Category 5 — Concurrency Safety

- [ ] **5.1 — Verify peer version map is race-free**
  - File: `transport/negotiating_transport.go:275–280`
  - Expected: `setPeerVersion()` and `getPeerVersion()` use proper synchronization (mutex or `sync.Map`).
  - Pitfall: Concurrent goroutines reading/writing the peer version map cause data races, potentially resulting in one goroutine seeing `ProtocolNoiseIK` while another sees `ProtocolLegacy` for the same peer.
  - Verify: `go test -race` on `transport/` package; review locking in `negotiating_transport.go`.

- [ ] **5.2 — Fix goroutine leak in key rotation checker**
  - File: `async/key_rotation_client.go:40–44`
  - Expected: `startKeyRotationChecker()` must have a stop mechanism (context cancellation or stop channel). The current implementation uses `for range ticker.C` with no way to exit the goroutine.
  - Pitfall: Every `AsyncClient` that starts leaks a goroutine that runs forever. Over time this exhausts memory and goroutine limits.
  - Verify: Read `startKeyRotationChecker()`; confirm there is a `ctx.Done()` or stop channel in the select. Currently there is **neither**.

- [ ] **5.3 — Verify Noise session map is concurrent-safe**
  - File: `transport/noise_transport.go`
  - Expected: The session map (mapping peer addresses to `NoiseSession`) uses proper locking for concurrent access from multiple goroutines.
  - Pitfall: Concurrent handshake initiation for the same peer could create duplicate sessions, wasting resources and potentially causing state confusion.

- [ ] **5.4 — Verify epoch manager is goroutine-safe**
  - File: `async/epoch.go`
  - Expected: Epoch transitions (every 6 hours) are atomic. Concurrent calls to `CurrentEpoch()` during a transition return consistent results.
  - Pitfall: Race between epoch transition and pseudonym generation could use stale epoch, creating unlinkable pseudonyms that the recipient can't resolve.

---

## Category 6 — Memory & Buffer Safety

- [ ] **6.1 — Fix integer truncation in relay storage address serialization**
  - File: `dht/relay_storage.go:176`
  - Expected: `len(addrBytes)` is bounds-checked before casting to `uint16`. If length exceeds 65535, return an error.
  - Pitfall: Silent truncation of address length causes the receiver to read fewer bytes than were written, corrupting the remaining data stream.
  - Verify: Read line 176; confirm bounds check exists before `uint16()` cast.

- [ ] **6.2 — Verify dual-format handshake parsing doesn't over-read**
  - File: `transport/versioned_handshake.go:114–139`
  - Expected: `ParseVersionedHandshakeRequest()` correctly handles packets where `NoiseMessage` is empty (legacy peer) or `LegacyData` is empty (Noise-only peer).
  - Pitfall: Parsing assumes both fields are present, reading past buffer end.
  - Verify: Feed a pure-legacy handshake (no Noise data) and a pure-Noise handshake (no legacy data) to the parser; confirm no panic or slice bounds violation.

- [ ] **6.3 — Verify message padding doesn't exceed maximum packet size**
  - File: `async/message_padding.go:18–27`
  - Expected: Padding buckets are 256, 1024, 4096, 16384. Messages exceeding 16384 bytes must be handled (error or fragmentation), not silently truncated.
  - Pitfall: Oversized padded messages exceed Tox's maximum payload size and are dropped by the network, causing silent message loss.
  - Verify: Check `PadMessageToStandardSize()` behavior when input > 16384 bytes.

- [ ] **6.4 — Verify unpadding validates length prefix**
  - File: `async/message_padding.go` (`UnpadMessage()`)
  - Expected: The 4-byte length prefix is validated against the total padded message size. A length prefix larger than the padded buffer must return an error.
  - Pitfall: A maliciously crafted length prefix causes `UnpadMessage()` to return a slice extending beyond the buffer, leading to memory corruption or information disclosure.

---

## Category 7 — Error Handling & Panics

- [ ] **7.1 — Verify downgrade events are logged with peer address**
  - File: `transport/negotiating_transport.go:181–192`
  - Expected: When `EnableLegacyFallback` causes a downgrade, the log entry includes the peer address at WARN level with `"Cryptographic downgrade"` message.
  - Pitfall: Downgrade happens silently, making it invisible to operators monitoring for security incidents.
  - Verify: Confirm log line exists and includes `addr` for incident response.

- [ ] **7.2 — Verify version commitment verification failure aborts handshake**
  - File: `transport/version_commitment.go:128–163`
  - Expected: `VerifyVersionCommitment()` failure returns an error that propagates up and aborts the connection. The handshake must not continue after commitment failure.
  - Pitfall: Commitment failure is logged but handshake continues, allowing version rollback.
  - Verify: Trace commitment verification failure path; confirm it returns an error that propagates to abort the connection.

- [ ] **7.3 — Verify Noise handshake errors don't expose internal state**
  - File: `noise/handshake.go`, `transport/noise_transport.go`
  - Expected: Failed handshake errors are wrapped with context (`fmt.Errorf("...: %w", err)`) but don't expose key material, session state, or internal addresses.
  - Pitfall: Verbose error messages containing key bytes or session IDs aid attackers in cryptanalysis.

- [ ] **7.4 — Verify DHT packet parsing returns errors for malformed input**
  - File: `dht/` package
  - Expected: Parsing functions for ping, get_nodes, send_nodes return descriptive errors for truncated, oversized, or malformed packets. No panics on adversarial input.
  - Pitfall: Panics on malformed DHT packets allow remote denial of service by any node in the DHT.

---

## Category 8 — Boundary & Off-by-One

- [ ] **8.1 — Verify versioned handshake length field accounts for optional Noise message**
  - File: `transport/versioned_handshake.go:67–111`
  - Expected: The 2-byte Noise message length prefix correctly encodes zero when Noise is absent (legacy-only handshake).
  - Pitfall: Length field of zero is misinterpreted as "read next 0 bytes" but parser skips differently, causing offset miscalculation for subsequent fields.
  - Verify: Serialize a legacy-only handshake request; confirm Noise length is 0 and `LegacyData` offset is correct.

- [ ] **8.2 — Verify S/Kademlia proof nonce size matches constant**
  - File: `dht/skademlia.go:41`
  - Expected: `ProofNonceSize = 8` and all proof generation/verification code uses exactly 8-byte nonces.
  - Pitfall: Nonce size mismatch between generator and verifier causes all proofs to fail, effectively disabling S/Kademlia.

- [ ] **8.3 — Verify key rotation keeps correct number of previous keys**
  - File: `crypto/key_rotation.go:29–37, 55–56`
  - Expected: `MaxPreviousKeys = 3` (default). After 4+ rotations, the oldest key is evicted. Contacts using the evicted key can no longer decrypt.
  - Pitfall: Off-by-one in key eviction either keeps too many old keys (wasted memory, wider compromise window) or too few (breaks decryption for slow contacts).
  - Verify: Write a test that rotates 5 times and confirms exactly 3 previous keys are retained.

- [ ] **8.4 — Verify epoch boundary transitions**
  - File: `async/epoch.go`
  - Expected: Epoch calculation from network genesis (January 1, 2025 00:00:00 UTC) with 6-hour periods is correct at all boundaries. `CurrentEpoch()` at 05:59:59 and 06:00:00 return different epochs.
  - Pitfall: Integer division rounding errors cause epoch boundaries to drift, making pseudonym resolution fail across implementations.

---

## Extension Backward Compatibility Summary

| Extension | Compat Check | Downgrade Vector | Priority |
|---|---|---|---|
| Noise-IK handshake | Must fall back cleanly to NaCl for legacy peers | `noise_transport.go:317` silent unencrypted fallback | **Critical** |
| Version negotiation | Signed packets must be ignored by c-toxcore | Forged version response → forced Legacy | **Critical** |
| Version commitment | Must bind version choice to handshake | Optional verification → rollback attack | **Critical** |
| S/Kademlia PoW | Standard nodes must be accepted without PoW | Mandatory PoW → network partition | **High** |
| Extension packets 249–255 | Must be silently dropped by c-toxcore | Packet rejection → connection failure | **High** |
| Async messaging | Must be encapsulated or use extension types | Raw async packet → legacy misparse | **High** |
| Message padding | Must not change standard packet structure | Over-size padded packets → rejection | **Medium** |
| PSK resumption | Must degrade to full handshake gracefully | Ticket replay → session hijack | **Medium** |
| Identity obfuscation | Internal to async; no wire-level impact | None (extension-only feature) | **Low** |
| Multi-network transports | Orthogonal to wire protocol | Transport-layer, not protocol-level | **Low** |

---

## References

- `docs/SECURITY_AUDIT_REPORT.md` — Existing security audit report (nonce exhaustion mitigation)
- `docs/SECURITY_AUDIT_SUMMARY.md` — Executive summary of security features
- `GAPS.md` — Implementation gaps analysis
- `docs/ASYNC.md` — Async messaging extension specification (notes "unofficial extension")
- `docs/FORWARD_SECRECY.md` — Forward secrecy design
- `docs/OBFS.md` — Identity obfuscation design
- `docs/MULTINETWORK.md` — Multi-network transport architecture
- `docs/FRIEND_REQUEST_TRANSPORT.md` — Migration notes and backward compatibility
- `docs/MESSAGE_RECEIPTS.md` — Compatible with c-toxcore, uTox, qTox
- `capi/compatibility_test.go` — Documented behavioral differences from c-toxcore
