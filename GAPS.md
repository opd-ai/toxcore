# GAPS ANALYSIS — Implementation vs. Stated Goals
## toxcore-go (github.com/opd-ai/toxcore) — 2026-05-27

This document cross-references every high-level feature claim in the README, doc.go, and technical specification documents under `docs/` against the actual source-code implementation, identifying gaps, stubs, and protocol deviations.

---

## 1. Noise Protocol Framework — IK Pattern

**Stated goal:** Noise-IK pattern for mutual authentication of all peer-to-peer sessions, providing identity-hiding and forward secrecy at the handshake layer.

**Gaps:**
- `noise/handshake.go` implements IK and XX patterns, but the XX `WriteMessage` at line 505-519 accepts a `receivedMessage` parameter that is **never processed** — it is not passed to `state.ReadMessage`. The XX responder constructs its reply without processing the initiator's transcript, breaking the 3-message XX flow. (The function is currently used as a shim and the XX pattern is not used in the primary flow, but it is exported and callable.)
- The Noise-IK initiator flow completes two messages (`Send` + `Receive`) without any check that the remote static key matches a previously-trusted peer key. There is no explicit TOFU or pinned-key validation path wired into the `IKHandshake`; callers must perform this check themselves, which several call sites do not.

**Reference:** `noise/handshake.go:504-519`, `noise/handshake.go:61-81`

---

## 2. Epoch-Based Forward Secrecy

**Stated goal:** Automatic epoch-based key rotation with forward secrecy; each message uses a one-time pre-key; compromising the current key does not expose past messages.

**Gaps:**
- **`SenderEphemeralPK` is always the zero value.** `async/obfs.go:generateMessagePseudonyms` returns `[32]byte{}` as the third argument (line 311). `ObfuscatedAsyncMessage.SenderEphemeralPK` is described in the struct comment as the *sender's ephemeral public key for recipient ECDH*, but it is never populated. Any recipient path that attempts ECDH with this field derives a shared secret from the zero point, which is cryptographically meaningless. The protocol spec in `docs/ASYNC.md` requires this field.
- **`ProcessPreKeyExchange` replaces rather than merges pre-key bundles.** When a second bundle arrives before all keys from the first are consumed, the unconsumed keys are silently dropped (`async/forward_secrecy.go:388`). This creates delivery gaps for messages encrypted to keys from the first bundle.
- **Old private keys not immediately scrubbed in `AsyncClient`.** After rotation, `async/key_rotation_client.go:77` replaces the pointer but the old `*KeyPair` memory is not zeroed until GC eventually collects it. The `KeyRotationManager.TrimHistory` wipe path does not apply to the `AsyncClient`-held copy.

**Reference:** `async/obfs.go:295-313`, `async/forward_secrecy.go:384-389`, `async/key_rotation_client.go:75-80`

---

## 3. Cryptographic Identity Obfuscation

**Stated goal:** Sender and recipient identities are hidden from storage nodes through cryptographic pseudonyms. Storage nodes cannot correlate messages to real identities.

**Gaps:**
- The `SenderEphemeralPK` field (zero value — see §2) defeats ECDH-based identity obfuscation for the sender. The zero public key is trivially distinguishable from a real ephemeral key and would be visible to any storage node inspecting the `ObfuscatedAsyncMessage` structure.
- `async/obfs.go` implements `GenerateSenderPseudonym` and `GenerateRecipientPseudonym` correctly, but the overall `CreateObfuscatedMessage` pipeline is incomplete because the ephemeral key required to derive the sender pseudonym via ECDH is never generated.

**Reference:** `async/obfs.go:295-370`

---

## 4. Offline Asynchronous Messaging (Storage Nodes)

**Stated goal:** Messages can be deposited at storage nodes while the recipient is offline; retrieved when they come online; with forward secrecy and identity obfuscation.

**Gaps:**
- **WAL concurrent access is unsafe under high write load.** Unbounded checkpoint goroutine spawning (`async/wal.go:297-303`) means sustained writes produce goroutine explosion; data race on `closeErr` during concurrent `Close()` calls (`async/wal.go:521`).
- **Untracked delivery goroutines** (`async/manager.go:739-743`) prevent clean shutdown; goroutines spawned after `Stop()` is called may access freed resources.
- **`queryDHT` returning `(nil, nil)`** (`async/prekey_dht.go:318`) means storage node discovery silently fails without an error, and the calling code proceeds as if discovery succeeded.
- **Erasure coding verification** (`async/erasure.go:248-252`) returns `(false, nil)` for a valid partial shard set (3 of 5 shards), making it impossible for callers to distinguish "not enough shards yet" from "data is corrupt." This breaks the retry/wait decision in the retrieval scheduler.

**Reference:** `async/wal.go:297-303,519-528`, `async/manager.go:739-743`, `async/prekey_dht.go:318`, `async/erasure.go:248-252`

---

## 5. ToxAV Audio/Video Calling

**Stated goal:** Full ToxAV support including call setup, audio/video frame send/receive, bit-rate control, and call quality monitoring.

**Gaps:**
- **TOCTOU in bit-rate and frame-send methods.** `AudioSetBitRate`, `VideoSetBitRate`, `AudioSendFrame`, and `VideoSendFrame` in `toxav.go` (lines 884, 938, 1034, 1223) release the read lock before using `impl`. Concurrent `Kill()` renders `impl` nil/stopped in the window between lock release and use.
- **Quality monitor misclassifies high-jitter calls as "Fair."** `av/quality.go:382` returns `QualityFair` for jitter ≥ 200 ms (the `PoorJitter` threshold). The correct return is `QualityPoor`. All other quality thresholds correctly map `Poor*` → `QualityPoor`.
- **Brand-new calls are immediately reported as `QualityUnacceptable`.** `av/quality.go:buildBasicMetrics` computes frame age from a zero `LastFrameTime`, yielding an age of ~55 years which exceeds `FrameTimeout` (2 s). Quality callbacks fire "unacceptable" before the first RTP packet arrives.
- **`validateAudioFrameCall` / `validateVideoFrameCall` access `m.addressFriendLookup` without holding `m.mu`.** The field is written under `m.mu.Lock()`. This is a data race per the Go memory model.
- **CPU profiling context fields are not mutex-protected.** `av/performance.go:281-282` writes `po.profilingCtx` and `po.profilingCancel` without any mutex while `StopCPUProfiling` reads them without a mutex.
- **`getTimeProvider()` in `av/metrics.go` reads `ma.timeProvider` without holding `ma.mu`**, creating a data race with `SetTimeProvider`.

**Reference:** `toxav.go:884,938,1034,1223`, `av/quality.go:381-382,256-283`, `av/manager.go:527,684`, `av/performance.go:281-282`, `av/metrics.go:491-494`

---

## 6. File Transfer

**Stated goal:** Bidirectional file transfer with pause/resume/cancel semantics.

**Gaps:**
- **Incoming duplicate file request leaks open file handle.** If a retransmitted file request arrives after the transfer has been started (file opened), the existing `Transfer` is overwritten without calling `Cancel()`, leaking the `*os.File` (`file/manager.go:355-357`).
- **Race on `transfer.Transferred` counter.** The unlocked snapshot read at `file/manager.go:442` races with concurrent ACK processing, giving the receive callback an incorrect byte offset.

**Reference:** `file/manager.go:355-357,442-443`

---

## 7. Privacy-Network Transport (Tor, I2P, Nym, Lokinet)

**Stated goal:** Multi-network transport supporting IPv4/IPv6, Tor, I2P, Nym, and Lokinet with interface-only networking types.

**Gaps:**
- **Concrete `*net.UDPAddr` used in several transport files.** `transport/nat.go:21,449`, `transport/gossip_bootstrap.go:298`, `transport/advanced_nat.go:251` use `*net.UDPAddr` or ignore `net.Addr` interfaces. The project convention ("Never use `net.UDPAddr`") is required for privacy-network transports (Tor/I2P addresses are not UDP addresses) to work correctly through the transport layer. Violating this causes `nil` returns or wrong address parsing for non-IP networks.
- **`extractUDPTransport` in `toxcore.go:573-581`** uses type switches on `transport.Transport`, hardcoding awareness of exactly two concrete transport types. Any new transport wrapper (including Tor/I2P/Nym wrappers) will silently return `nil`, disabling NAT traversal for those networks.
- **`transport/tcp.go:289-305`** — write deadline is not reset after a successful write. On long-lived TCP connections (expected in Tor/I2P where connection setup is expensive), this causes spurious timeouts on subsequent packets.

**Reference:** `transport/nat.go:21,449`, `dht/gossip_bootstrap.go:298`, `transport/advanced_nat.go:251`, `toxcore.go:573-581`, `transport/tcp.go:289-305`

---

## 8. DHT Routing and Bootstrap

**Stated goal:** S/Kademlia-based DHT with gossip bootstrap, k-bucket routing, iterative lookup, and partition detection.

**Gaps:**
- **`GossipPeer` IP addresses alias into raw packet buffer** (`dht/gossip_bootstrap.go:317,320`). In any read-loop reusing the buffer, stored peer IPs are corrupted silently. Gossip bootstrap is the primary peer-discovery mechanism; corrupted IPs produce connection failures.
- **`IterativeLookup.queryNode` concurrent-key clobber** (`dht/iterative_lookup.go:314-320`): parallel queries to the same public key overwrite each other's response channel. The clobbered goroutine hangs until context cancellation, slowing lookups.
- **Partition detector TOCTOU** (`dht/partition_detector.go:229-244`): `oldState` is captured outside the lock in `checkHealth`, enabling spurious state transitions.
- **Discovery port fallback to 1** (`dht/local_discovery.go:45-48`): silent privileged-port assignment breaks local discovery on non-root processes without any logged error.

**Reference:** `dht/gossip_bootstrap.go:317,320`, `dht/iterative_lookup.go:314-320`, `dht/partition_detector.go:229-244`, `dht/local_discovery.go:45-48`

---

## 9. Replay Protection

**Stated goal:** Nonce-based replay protection with persistent storage across restarts.

**Gaps:**
- **Potential deadlock in `NonceStore.Close()`** (`crypto/replay_protection.go:298-302`). `Close()` acquires `ns.mu.RLock()` then calls `save()` which also acquires `ns.mu.RLock()`. Go `sync.RWMutex` is non-reentrant; under write contention this deadlocks.

**Reference:** `crypto/replay_protection.go:298-302`

---

## 10. Key Management

**Stated goal:** Secure key rotation with controlled key-material erasure and forward-secure identity.

**Gaps:**
- **`FindKeyForPublicKey` returns a raw pointer to live mutable state** (`crypto/key_rotation.go:125-131`). After the `RLock` is released, a concurrent `RotateKey` can wipe the memory. The safe pattern (value copy) is already implemented in `GetCurrentKeyPair` at line 110.
- **Old key not immediately zeroed in `AsyncClient`** (`async/key_rotation_client.go:77`) — see §2.

**Reference:** `crypto/key_rotation.go:125-131`, `async/key_rotation_client.go:77`

---

## 11. Packet Delivery and Retry

**Stated goal:** Reliable packet delivery with configurable retry and exponential back-off.

**Gaps:**
- **Nil dereference panic when `RetryAttempts == 0`** (`real/packet_delivery.go:162`). With zero retries configured (attempt once and give up on failure), `lastErr` is nil when `handleDeliveryFailure` calls `lastErr.Error()`. This is the most immediate crash path in the library.

**Reference:** `real/packet_delivery.go:114-126,157-165`

---

## 12. Friend Management

**Stated goal:** Friend request, acceptance, and relationship state management with thread-safe storage.

**Gaps:**
- **TOCTOU on friend count in `FriendStore.Set()`** (`friend/store.go:48-53`). Two concurrent `Set()` calls for the same friend ID can both observe "new friend" and double-increment the counter. The counter is used in capacity checks and exported APIs.
- **`GetFriendEncryptionLevel` does not check per-friend pre-key exchange status** (`toxcore_friends.go:406-409`). Returns `EncryptionForwardSecure` for any online friend when `asyncManager != nil`, regardless of whether that specific friend has completed key exchange.

**Reference:** `friend/store.go:48-53`, `toxcore_friends.go:406-409`

---

## 13. Conference / Group Calls

**Stated goal:** Multi-party conference support.

**Gaps:**
- **Peer ID `0` falsely treated as "not a member" sentinel** (`toxcore_conference.go:125`). The guard `SelfPeerID == 0 && len(conference.Peers) == 0` incorrectly identifies the sentinel condition using a valid peer-ID value.

**Reference:** `toxcore_conference.go:125`

---

## 14. Noise Transport — Send Path

**Stated goal:** All traffic encrypted with Noise-IK via `NoiseTransport`; nonce reuse is impossible.

**Gaps:**
- **Data race on `CipherState` nonce in `encryptPacket`** (`transport/noise_transport.go:836-847`). The send cipher is copied under `RLock`, the lock is released, then `Encrypt()` is called. Concurrent sends race on the internal nonce counter. Nonce reuse breaks ChaCha20-Poly1305 confidentiality and integrity.

**Reference:** `transport/noise_transport.go:836-847`

---

## Summary Table

| # | Goal | Gap Severity | Key File/Line |
|---|------|-------------|---------------|
| 1 | Noise-IK handshake | HIGH | noise/handshake.go:504-519 |
| 2 | Forward secrecy / pre-key rotation | CRITICAL | async/obfs.go:311, async/forward_secrecy.go:388 |
| 3 | Identity obfuscation | CRITICAL | async/obfs.go:311 |
| 4 | Offline async messaging | HIGH | async/wal.go:297-303, async/manager.go:739 |
| 5 | ToxAV audio/video calls | HIGH | toxav.go:884,938,1034,1223; av/quality.go:382 |
| 6 | File transfer | MEDIUM | file/manager.go:357, 442 |
| 7 | Privacy-network transport | MEDIUM | transport/nat.go:21,449; toxcore.go:573 |
| 8 | DHT routing and bootstrap | MEDIUM | dht/gossip_bootstrap.go:317; dht/iterative_lookup.go:314 |
| 9 | Replay protection | CRITICAL | crypto/replay_protection.go:298-302 |
| 10 | Key management | HIGH | crypto/key_rotation.go:125-131 |
| 11 | Packet delivery retry | CRITICAL | real/packet_delivery.go:162 |
| 12 | Friend management | MEDIUM | friend/store.go:48-53 |
| 13 | Conference support | LOW | toxcore_conference.go:125 |
| 14 | Noise transport encryption | CRITICAL | transport/noise_transport.go:836-847 |
