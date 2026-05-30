# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-30

## Project Profile

**Purpose:** `toxcore-go` is a pure-Go implementation of the Tox peer-to-peer
encrypted messaging protocol (module `github.com/opd-ai/toxcore`, Go 1.25.0). It
provides DHT-based peer discovery, friend management, 1-to-1 and group messaging,
file transfers, ToxAV audio/video calling, asynchronous offline messaging with
forward secrecy, multi-network transport (IPv4/IPv6, Tor, I2P, Lokinet, Nym),
Noise-IK handshakes, and libtoxcore-compatible C bindings.

**Target users:** Application developers embedding a Tox client; C/C++ programs via
the `capi/` cgo bindings.

**Deployment model:** Library linked into a host process. Every node both initiates
and accepts connections from untrusted peers across the internet and overlay
networks. **Trust boundary:** all DHT, transport, async-storage, and ToxAV media
packets arrive from untrusted peers and are parsed before authentication. Savedata
and keystore files are application-controlled.

**Critical paths (primary stated goals):**
- DHT routing and packet parsing (`dht/`, `transport/parser.go`) — untrusted input.
- Cryptography and forward secrecy (`crypto/`, `async/`, `ratchet/`, `noise/`).
- Async offline messaging end-to-end (`async/`).
- ToxAV media transmission (`toxav.go`, `av/`).
- Friend/message/file APIs (`toxcore*.go`, `messaging/`, `file/`, `friend/`).

## Audit Scope

- **Packages audited:** all 27 non-example packages (root `toxcore`, `async`,
  `crypto`, `dht`, `transport` (+`internal/addressing`), `av` (+`audio`,`rtp`,`video`),
  `messaging`, `group`, `file`, `friend`, `noise`, `ratchet`, `toxnet`, `bootstrap`
  (+`nodes`), `limits`, `factory`, `real`, `simulation`, `interfaces`, `capi`, `cmd`).
  Example programs under `examples/` and `testnet/` were not deeply audited (non-shipping).
- **Functions inspected:** 1,303 functions + 3,029 methods across 251 files
  (43,059 LOC, skip-tests). All 20 functions >50 lines and the single function with
  cyclomatic complexity >10 (`async.ImportPreKeys`) were manually inspected.
- **Tooling:** `go-stats-generator` baseline, `go vet ./...` (0 warnings),
  `go test -race ./...` (35/35 testable packages pass, no data races reported).

## Coverage Log

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av / audio / rtp / video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxav (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ratchet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bootstrap | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| real / simulation / factory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| limits / interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary

| Stated Goal (README/ROADMAP) | Status | Blocking Findings |
|------------------------------|--------|-------------------|
| DHT routing & peer discovery | ⚠️ | C-04, C-05 (parser panics from untrusted packets) |
| Friend management | ⚠️ | H-02 (name/status propagation broken), H-04 (duplicate-friend race) |
| 1-to-1 messaging (online) | ✅ | — |
| Async offline messaging + forward secrecy | ❌ | C-02, C-03 (DHT pre-key retrieval & forward-secure decryption broken) |
| File transfers (pause/resume/cancel) | ⚠️ | H-15 (failed-send byte skip), M-09 (oversize final chunk) |
| Group chat (role-based permissions) | ⚠️ | M-06 (unauthenticated peer role trust) |
| ToxAV audio/video calling over RTP | ❌ | C-01 (RTP session never created via public API) |
| Adaptive bitrate | ⚠️ | M-11 (adaptation never wired into iteration) |
| Multi-network transport | ✅ (Lokinet/Nym dial-only, documented) | — |
| Noise-IK handshakes | ✅ | — |
| State persistence | ⚠️ | H-03 (silent identity regeneration), M-13 (dropped friend fields) |
| C API bindings | ✅ (~79% coverage, documented) | — |
| Robustness vs untrusted input (DoS) | ❌ | C-04..C-06, H-09..H-13 (panics & unbounded memory) |

## Findings

Severity legend and counts: **CRITICAL 6 · HIGH 22 · MEDIUM 21 · LOW 14**.
All line numbers refer to the audited working tree. `go test -race ./...` passes for
all packages, which is empirical (not conclusive) evidence against some of the
concurrency findings below — noted where relevant.

### CRITICAL

- [x] **C-01 ToxAV media transmission is non-functional via the public API** — `av/types.go:586-679`, `toxav.go:548`, `av/manager.go:1067` — bug class: API / documented-but-nonfunctional — `NewToxAV` builds a `*toxAVTransportAdapter` (whose `Send(packetType byte, data, addr []byte) error` and `RegisterHandler(byte, func([]byte,[]byte) error)` do **not** match `transport.Transport`, and which lacks `Close`/`LocalAddr`/`IsConnectionOriented`). `Call.SetupMedia` does `toxTransport, ok := transportArg.(transport.Transport)`; the assertion **always fails**, so `createRTPSession` is never reached and `call.rtpSession` stays `nil`. `isAudioProcessingReady`/`isVideoProcessingReady` (`av/manager.go:568,727`) then skip every send/receive. Result: audio/video frames are encoded locally but never transmitted over RTP, despite README advertising "peer-to-peer calling … RTP transport". — **Remediation:** route RTP through the `av.TransportInterface` abstraction (add a media-send method), or pass a real `transport.Transport` into `SetupMedia`; make `SetupMedia` and the send paths return an error when an RTP session cannot be created instead of silently returning success. Validate with a cross-instance integration test asserting received frames, plus `go test -race ./av/... .`.

- [ ] **C-02 DHT-published pre-key bundles never verify (forward secrecy over DHT broken)** — `async/prekey_dht.go:204-212, 373` — bug class: Security / documented-but-nonfunctional — `signBundle` signs with `crypto.Sign(data, pm.keyPair.Private)`, which treats the 32-byte private key as an **Ed25519 seed** (`crypto/ed25519.go:19-32`). `validateBundle` verifies with `crypto.Verify(data, sig, bundle.OwnerPK)` where `OwnerPK = pm.keyPair.Public` is the **Curve25519** public key (`crypto/keypair.go`), which is not the Ed25519 public key derived from that seed. `ed25519.Verify` therefore always fails, so every legitimately published bundle is rejected and DHT pre-key retrieval cannot succeed. (Tests pass because the end-to-end DHT publish→retrieve→verify path is not covered by an integration test.) — **Remediation:** store and verify against the Ed25519 public key derived from the signing seed (e.g. add a signer-PK field bound to `OwnerPK`, or sign a binding of `OwnerPK`), and verify with that key. Add a publish/validate round-trip test under `go test ./async/...`.

- [ ] **C-03 Forward-secure async messages fail decryption (pre-key IDs lost in transit)** — `async/manager.go:1017-1020, 1202-1215` — bug class: Logic / documented-but-nonfunctional — `createPreKeyExchangePacket` serializes only each pre-key's 32-byte public key and drops the ID; `extractPreKeysFromPacket` reconstructs IDs as `uint32(i)` (0..n-1). But pre-keys are generated with **random 32-bit IDs** (`async/prekeys.go:168-178`). When a sender later builds a `ForwardSecureMessage` with `PreKeyID: preKey.ID` (`async/forward_secrecy.go:475`), the recipient resolves it via `CheckAndMarkPreKeyUsed(SenderPK, PreKeyID)` against its own store of random IDs (`async/forward_secrecy.go:492`, `async/prekeys.go:597`); the synthetic index will not match, so the one-time key is not found and the message cannot be decrypted. — **Remediation:** serialize each `PreKeyForExchange.ID` alongside its public key and parse the original ID in `extractPreKeysFromPacket`. Validate with a forward-secrecy round-trip test that crosses the serialize/deserialize boundary (`go test -race ./async/...`).

- [ ] **C-04 Nil-address panic from malformed `send_nodes` packet (remote DoS)** — `dht/handler.go:403-411`, reachable via `transport/parser.go:192-243` — bug class: Nil/Boundary — `ExtendedParser.ParseNodeEntry` accepts `addrLen == 0` for an IPv4/IPv6 address-type entry (a 36-byte entry passes the `>=35` and `currentOffset+addrLen+2` checks), producing a `NetworkAddress` with empty `Data`. `processNodeEntryVersionAware` calls `entry.Address.ToNetAddr()`, which returns a **nil interface** for empty IPv4/IPv6 data (`transport/address.go:90-101`). `DetectAddressType(nil)` returns an error (`dht/address_detection.go:40-42`), and the error branch formats `addr.String()` on the nil interface → nil-pointer panic. Data path: untrusted `PacketSendNodes` → `handleSendNodesPacket` → `processReceivedNodesWithVersionDetection` → panic, crashing the process (panics in handler goroutines are unrecovered here). — **Remediation:** in `processNodeEntryVersionAware`, check `addr == nil` before any `addr.String()`; reject node entries whose IPv4/IPv6 `Data` length is wrong in the parser. Validate with a fuzz/table test feeding a 36-byte IPv4 entry and `go test -race ./dht/...`.

- [ ] **C-05 Integer-wrap slice panic in relay announcement deserialization (remote DoS)** — `dht/relay_storage.go:200-206` — bug class: Logic / Nil-Boundary — `addrLen := binary.BigEndian.Uint16(data[51:53])` is a `uint16`; the guard `len(data) < int(53+addrLen)` computes `53+addrLen` in **uint16 arithmetic**, so `addrLen == 0xFFFF` wraps to `52`, the guard passes for any buffer ≥52 bytes, and `data[53 : 53+addrLen]` becomes `data[53:52]` → "slice bounds out of range" panic. Data path: untrusted `PacketRelayAnnounce` / relay-query response → `DeserializeRelayAnnouncement`. — **Remediation:** convert to `int` before arithmetic (`end := 53 + int(addrLen)`), cap `addrLen` to a protocol maximum, then validate `end <= len(data)`. Validate with a malformed-packet test and `go test -race ./dht/...`.

- [ ] **C-06 Unbounded allocation from attacker-controlled TCP length prefix (remote OOM DoS)** — `transport/tcp.go:437-453` — bug class: Boundary / Resources — `readPacketLength` reads a 4-byte big-endian length with no maximum; `readPacketData` then does `make([]byte, length)` with `length` up to `0xFFFFFFFF` (~4 GiB) before reading any payload. A single peer can force a multi-gigabyte allocation per connection. Data path: `Accept` → `handleConnection` → `readPacketLength` → `readPacketData`. — **Remediation:** reject lengths above a fixed protocol maximum (the Tox max packet/message size) before allocating; consider pooled buffers. Validate with a test sending an oversized length prefix and `go test ./transport/...`.

### HIGH

- [ ] **H-01 `EncryptedKeyStore` has no synchronization** — `crypto/keystore.go:23-40` — bug class: Concurrency — mutable `encryptionKey`/`masterPassword`/`salt` and rotation state are read by `WriteEncrypted`/`ReadEncrypted` while `RotateKey`/`Close` mutate or wipe them, with no mutex; concurrent use races and can encrypt/decrypt with a half-rotated key. The GoDoc (`crypto/doc.go:114-119`) implies safe lifecycle use. — **Remediation:** add a `sync.RWMutex` guarding every method that touches key/password/salt/file-rotation state. Validate with `go test -race ./crypto/...`.

- [ ] **H-02 `SelfSetName`/`SelfSetStatusMessage` propagation is mis-routed to friend 0** — `toxcore_self.go:161-205`, `toxcore.go:931-941` — bug class: Logic — `broadcastNameUpdate`/`broadcastStatusMessageUpdate` write `friendID = 0` ("placeholder for self") into the packet, and the receiver's `processFriendNameUpdatePacket` reads that field and calls `receiveFriendNameUpdate(0, name)`, updating the recipient's **local friend #0** regardless of who sent it. Remote name/status changes are therefore applied to the wrong contact (or a non-existent one), and never to the actual sender. — **Remediation:** embed the sender's public key (as `processFriendRequestPacket` does), and resolve it to the local friend ID on receipt. Validate with a two-instance test asserting the correct friend's name updates (`go test ./...`).

- [ ] **H-03 `SaveDataTypeSecretKey` with wrong length silently generates a new identity** — `toxcore.go:440-449` — bug class: API — `createKeyPair` only honors saved secret-key bytes when `len(options.SavedataData) == 32`; any other length silently falls through to `crypto.GenerateKeyPair()`, producing a brand-new identity and discarding the user's intended key (and thus their Tox ID / friends). — **Remediation:** when `SavedataType == SaveDataTypeSecretKey`, return an explicit error unless the data is exactly 32 bytes. Validate with a unit test passing a 31-byte secret key.

- [ ] **H-04 Duplicate-friend race (TOCTOU around add)** — `toxcore_friends.go:24-60` — bug class: Concurrency — the public-key existence check runs before `friendsAddMu` is taken, so two concurrent `AddFriend`/`AddFriendByPublicKey` calls for the same key can both pass the check and create duplicate entries with different IDs. — **Remediation:** perform the duplicate check and ID allocation under `friendsAddMu`. Validate with `go test -race` exercising concurrent adds.

- [ ] **H-05 `FriendStore.Get` returns a mutable `*Friend` shared with mutators** — `toxcore.go:1039`, callers in `toxcore_friends.go` — bug class: Concurrency/Aliasing — readers dereference fields of the returned pointer outside the store lock while `SetFriendConnectionStatus`/`Update` mutate the same struct, producing data races on `ConnectionStatus`, `Name`, etc. — **Remediation:** use `FriendStore.Read`/`Update` closures or return a copy. Validate with `go test -race ./...`.

- [ ] **H-06 `clearCallbacks` writes callback fields without `callbackMu`** — `toxcore_lifecycle.go:178-200` — bug class: Concurrency — `Kill` → `clearCallbacks` assigns callback fields with no lock while dispatchers read them under `callbackMu.RLock()`, racing a teardown against in-flight `Iterate`/callback dispatch. — **Remediation:** take `callbackMu.Lock()` while clearing, and clear all callback fields (see L-02). Validate with `go test -race`.

- [ ] **H-07 `doFriendConnections` reads `t.dht` without `dhtMutex`** — `toxcore_lifecycle.go:248-270` — bug class: Concurrency — the friend-connection pipeline reads `t.dht` unsynchronized and later dereferences it (`maybeScheduleRetryForFriend`) while `Kill` can set `t.dht = nil` under `dhtMutex`, risking a race or nil-deref panic. — **Remediation:** snapshot `t.dht` under `dhtMutex` once and pass it through; bail if nil. Validate with `go test -race`.

- [ ] **H-08 Late transport callbacks dereference `t.bootstrapManager` after `Kill`** — `toxcore_network.go:255-270` — bug class: Concurrency/Nil — registered packet handlers use `t.bootstrapManager` after `Kill` may have niled it, causing a nil-pointer panic from a packet that arrives during/after shutdown. — **Remediation:** guard `bootstrapManager` with a mutex/nil check, or unregister handlers before clearing it. Validate with a shutdown-during-traffic race test.

- [ ] **H-09 Unbounded per-connection goroutines with no read deadline (TCP)** — `transport/tcp.go:360-435` — bug class: Resources/Concurrency — every accepted connection spawns a goroutine that blocks in `io.ReadFull` with no read deadline and no cap on concurrent connections; idle/slow-loris peers exhaust goroutines and file descriptors. — **Remediation:** set per-connection read deadlines, cap concurrent connections, and close stalled handshakes. Validate with a connection-flood test.

- [ ] **H-10 Unbounded Noise session map; pre-auth session creation** — `transport/noise_transport.go:575-590, 815-825` — bug class: Resources/Security — each unique source of a `PacketNoiseHandshake` creates a responder session before the handshake is validated, and failed handshakes linger until timeout, so address-spoofed floods consume memory/CPU. — **Remediation:** cap sessions (LRU + rate limit), and delete a session immediately on handshake failure. Validate with a handshake-flood test.

- [ ] **H-11 `storeNegotiatedVersion` grows `peerVersions` without eviction** — `dht/handler.go:145-160` — bug class: Resources — untrusted `PacketVersionNegotiation` packets insert into `peerVersions` without the `maxPeerVersionEntries` eviction used elsewhere (`SetPeerProtocolVersion`), allowing unbounded map growth (memory DoS). — **Remediation:** route through `SetPeerProtocolVersion` or replicate its eviction/ordering. Validate with `go test ./dht/...`.

- [ ] **H-12 Unbounded group-announcement storage** — `dht/group_storage.go:86-95` — bug class: Resources — arbitrary network group announcements are stored by `GroupID` with a 24h TTL and no cap; unique IDs fill memory. — **Remediation:** cap entries, evict expired/oldest first, validate/rate-limit sources. Validate with `go test ./dht/...`.

- [ ] **H-13 Unbounded relay-announcement storage** — `dht/relay_storage.go:80-90` — bug class: Resources — relay announcements are stored by public key with 24h TTL and no cap, and addresses are unbounded length; attacker fills memory with unique keys/large addresses. — **Remediation:** cap entries, evict LRU/expired, cap address length (also see C-05). Validate with `go test ./dht/...`.

- [ ] **H-14 WAL checkpoints before the operation is applied; no commit** — `async/wal.go:300-330` — bug class: Logic — `logEntry` schedules a checkpoint before `StoreMessage`/`DeleteMessage` mutates storage, and storage never calls `Commit`; a crash after checkpoint but before the map mutation makes `Recover` skip the pending entry (via `lastCheckpointSeq`), losing a store or resurrecting a delete. — **Remediation:** apply the storage mutation first, then commit/checkpoint only applied sequences. Validate with a crash-recovery test in `async`.

- [ ] **H-15 File `SendChunk` advances `Transferred` before send; failed send skips bytes** — `file/manager.go:300-320`, `file/transfer.go` `ReadChunk` — bug class: Logic — `ReadChunk` increments `Transferred` before `transport.Send`; if the send fails, the next attempt reads the following chunk, permanently skipping the un-sent bytes and silently corrupting the received file. — **Remediation:** advance progress only after a successful send, or seek/rollback on failure. Validate with a send-failure unit test in `file`.

- [ ] **H-16 Async retrieve request sent before response channel is registered** — `async/client.go:1165-1180` — bug class: Concurrency — `retrieveObfuscatedMessagesFromNode` sends the request before `setupResponseChannel` registers the channel; a fast (e.g. loopback) node response reaches `handleRetrieveResponse` first, is logged as "unexpected", and the caller times out. — **Remediation:** register the response channel before sending, and clean up on send failure. Validate with a fast-loopback test and `go test -race ./async/...`.

- [ ] **H-17 Nil obfuscated message from untrusted node panics decryptor** — `async/obfs.go:411-425`, `decryptRetrievedMessages` → `DecryptObfuscatedMessage` — bug class: Nil/Boundary — `deserializeRetrieveResponse` accepts `[]*ObfuscatedAsyncMessage` from an untrusted storage node; a nil element flows into `DecryptObfuscatedMessage` and dereferences `obfMsg.Epoch`, panicking. — **Remediation:** skip/reject nil elements before decryption. Validate with a malformed-response test.

- [ ] **H-18 Negative erasure shard index panics reconstruction** — `async/erasure.go:345-360`, `ReconstructMessage` — bug class: Nil/Boundary — `StoreShard` stores `shard.Index` without range validation; a negative index from a peer survives, and reconstruction only checks `idx < len(shards)` (not `idx >= 0`), so `shards[-1]` panics. — **Remediation:** reject indexes `<0` or `>= TotalShards` in `StoreShard`/`DecodeShards`/reconstruction. Validate with `go test ./async/...`.

- [ ] **H-19 `ReconstructMessage` iterates a shared shard map after releasing the lock** — `async/erasure.go:330-345` — bug class: Concurrency — the function copies the `shardMap` reference under `RLock`, unlocks, then ranges it; a concurrent `StoreShard`/`DeleteMessage` write triggers a fatal "concurrent map iteration and map write". — **Remediation:** snapshot the shard pointers into a slice while holding the lock. Validate with `go test -race ./async/...`.

- [ ] **H-20 `GetBundle` returns the internal bundle after unlock; racy publish of consumed keys** — `async/prekeys.go:345-360` — bug class: Aliasing/Concurrency — `GetBundle` returns the internal `*PreKeyBundle` after releasing `pks.mutex`; `ExchangePreKeys` then iterates `bundle.Keys` without the store lock while `GetAvailablePreKey`/`RefreshPreKeys` remove or wipe keys, racing and potentially publishing already-consumed keys. — **Remediation:** return a deep snapshot of the bundle/keys (or expose a locked iterator). Validate with `go test -race ./async/...`.

- [ ] **H-21 `ExportPreKeys` shares `*crypto.KeyPair` pointers; later wipe corrupts the backup** — `async/prekeys.go:730-745` — bug class: Aliasing — the "deep-copy" backup appends `PreKey` values that still share the live `*crypto.KeyPair`; a later `MarkPreKeyUsed` wipes the store key's private bytes (`crypto.ZeroBytes`), corrupting the exported backup so restored pre-keys cannot decrypt. — **Remediation:** clone each `PreKey` and its `KeyPair` (private/public bytes) in `ExportPreKeys`. Validate with an export-then-consume-then-restore test.

- [ ] **H-22 Pre-key exchange packet trusts a self-asserted Ed25519 key (peer pre-key poisoning)** — `async/manager.go:1180-1196`, `handlePreKeyExchangePacket` — bug class: Security — `verifyPreKeyPacketSignature` authenticates with the Ed25519 key carried in bytes 37:69 of the packet, while the caller only checks the Curve25519 `senderPK` against known friend addresses. An attacker who knows a friend's (public) Curve25519 key can present an attacker Ed25519 key+signature and overwrite that friend's `peerPreKeys`, causing the victim to encrypt forward-secure messages to attacker-controlled keys. — **Remediation:** verify a trusted binding between `senderPK` and the Ed25519 signing key (derive the expected Ed25519 key from the friend's identity, or require a signed identity binding). Validate with a spoofed-packet test under `go test ./async/...`.

### MEDIUM

- [ ] **M-01 Keystore path traversal via unvalidated `filename`** — `crypto/keystore.go:170, 219, 278, 290` — bug class: Security — `WriteEncrypted`/`ReadEncrypted`/`DeleteEncrypted` join `filename` to `dataDir` with no validation, so `..`/absolute paths escape the directory (read/overwrite/delete arbitrary process-writable files). Impact is bounded by the library trust model (filenames are usually application-supplied), but any app exposing key names to user input is exploitable. — **Remediation:** reject absolute paths, separators, and `..`, and verify the cleaned joined path stays under `dataDir`. Validate with traversal unit tests.

- [ ] **M-02 Replay-protection expiry integer overflow** — `crypto/replay_protection.go:118-128` — bug class: Logic — `expiry := timestamp + 360` can overflow for a caller-supplied near-`MaxInt64` timestamp; the negative expiry is later cleaned up, after which the same nonce is accepted again. — **Remediation:** reject `timestamp > math.MaxInt64-360`, or derive expiry from trusted local time. Validate with a boundary test.

- [ ] **M-03 Unsynchronized package-global time providers** — `crypto/time_provider.go:22` and `toxnet/time_provider.go:44` — bug class: Concurrency — `Get/SetDefaultTimeProvider` read/write package globals with no synchronization; concurrent set/get races. — **Remediation:** guard with `sync.RWMutex` or `atomic.Value`. Validate with `go test -race`.

- [ ] **M-04 `SetTimeProvider` writes `t.timeProvider` unsynchronized and accepts nil** — `toxcore_lifecycle.go:48-60` — bug class: Concurrency/Nil — concurrent `now()` reads race the write, and a nil provider later panics. — **Remediation:** guard with a mutex and reject nil. Validate with `go test -race`.

- [ ] **M-05 `SelfGetPublicKey`/`SelfGetSecretKey` read `t.keyPair` without `selfMutex`** — `toxcore_self.go:44-60` — bug class: Concurrency — `Load` replaces `t.keyPair` under `selfMutex`, but these getters read it unlocked, racing a hot-reload. — **Remediation:** read under `selfMutex`. Validate with `go test -race`.

- [ ] **M-06 Group peer roles are taken from unauthenticated network data** — `group/chat.go:1940-1960` (`HandlePeerListResponse`→`HandlePeerAnnounce`) — bug class: Security — `PeerAnnounceData.Role` from the network is trusted directly, so a malicious peer can appear locally as admin/founder, affecting moderation and UI trust. — **Remediation:** default discovered peers to the lowest role and only elevate via signed founder/admin role-change messages. Validate with a forged-announce test.

- [ ] **M-07 Incoming file chunk written before size check (one oversize final chunk)** — `file/transfer.go:500-515` — bug class: Nil/Boundary — `WriteChunk` writes the peer's chunk before validating remaining size, so a peer advertising a small `FileSize` can write one oversized final chunk beyond the declared size. — **Remediation:** reject/truncate when `len(data) > FileSize-Transferred` before writing. Validate with a unit test.

- [ ] **M-08 File create follows pre-existing symlinks** — `file/transfer.go:245-255` — bug class: Security — the validated incoming basename is opened with `os.Create`, which follows an existing symlink of the same name, allowing an attacker-chosen filename to overwrite outside the intended destination. — **Remediation:** open with `O_NOFOLLOW`/`Lstat` checks rejecting symlinks. Validate with a symlink test.

- [ ] **M-09 Forged file ACK corrupts flow-control accounting** — `file/manager.go:530-545` — bug class: Logic — a peer-supplied ACK `bytesReceived` is accepted without bounding by `Transferred`/`FileSize`; forged ACKs collapse pending-byte accounting and can stall/skip progress callbacks. — **Remediation:** reject ACKs above sent/declared bounds and ignore regressions. Validate with a unit test.

- [ ] **M-10 RTP statistics (loss/jitter/bandwidth) are never tracked** — `av/rtp/session.go:410-420`, consumed at `av/quality.go:307-315` — bug class: API — receive paths increment only `PacketsReceived`; `PacketsLost`, `Jitter`, `Bandwidth` stay zero, so quality reports are falsely "healthy". — **Remediation:** track RFC 3550 sequence gaps/jitter/bandwidth and expose them. Validate with a loss/jitter unit test.

- [ ] **M-11 Automatic bitrate adaptation is never driven** — `av/adaptation.go:240-250`, `av/manager.go:1655` — bug class: API — `BitrateAdapter.UpdateNetworkStats` is never called in production (`Iterate` only calls `MonitorCall`), so the advertised adaptive bitrate does not operate unless the app manually drives internals. — **Remediation:** create/configure a per-call adapter and feed it RTP stats during iteration. Validate with an integration test asserting bitrate changes.

- [ ] **M-12 Quality jitter thresholds misclassify** — `av/quality.go:385-395` — bug class: Logic — with excellent packet loss, `jitter >= FairJitter` still returns `QualityGood` (e.g. 150 ms reported "good"), masking degraded calls. — **Remediation:** map jitter consistently to excellent/good/fair/poor/unacceptable. Validate with a threshold table test.

- [ ] **M-13 `GetFriends`/persistence drop friend fields** — `toxcore_friends.go:235`, `toxcore_persistence.go:360-370` (`cloneFriendEntry`) — bug class: API — the friend copy omits `IsTyping` and `DisappearingMessages`; `Save`/`Load` therefore silently lose the disappearing-message configuration across restart. — **Remediation:** copy every persistent `Friend` field. Validate with a save/load round-trip test.

- [ ] **M-14 `FriendSendMessage` returns the wrong message ID** — `toxcore_messaging.go:300-308` — bug class: API — it returns `t.lastMessageID` rather than the ID produced by `MessageManager.SendMessage` (or the async path), so delivery-status callbacks can be keyed to a mismatched ID. — **Remediation:** return the manager/async ID consistently. Validate with a delivery-tracking test.

- [ ] **M-15 Pipelines cannot be restarted after `Stop`** — `iteration_pipelines.go:131-165` — bug class: Concurrency/Logic — `Stop` calls `p.cancel()` permanently; a later `Start` sets `running=true` and spawns workers that immediately observe the already-cancelled `p.ctx` and exit, silently doing nothing. — **Remediation:** recreate the context/cancel in `Start`, or document/enforce single-use. Validate with a stop-then-start test.

- [ ] **M-16 UDP port-scan loop can spin forever when `EndPort == 65535`** — `toxcore_network.go:36` — bug class: Logic — `for port := StartPort; port <= EndPort; port++` with `uint16` ports wraps after 65535 to 0, so if `EndPort==65535` and every bind fails the loop never terminates, hanging `New()`. (Default `EndPort` is 33545, so this needs misconfiguration.) — **Remediation:** iterate with an `int` and break after the last port. Validate with a unit test using `EndPort=65535`.

- [ ] **M-17 `ImportPreKeys` shares `Keys` slices and `KeyPair` pointers with caller backup** — `async/prekeys.go:760-790` — bug class: Aliasing — both the unknown-peer adopt path (`cp := *imported`) and the merge-append path store `PreKey`/`KeyPair` values that alias the caller's backup; later mutation/wiping on either side corrupts the other (also see H-21). This is the highest-complexity function in the codebase (overall 17.1). — **Remediation:** deep-copy `Keys` and `KeyPair` values before storing/appending (`clonePreKey`). Validate with `go test -race ./async/...`.

- [ ] **M-18 Async/storage shallow copies share ciphertext slices** — `async/storage.go:300-315, 340-350, 620-640` — bug class: Aliasing — `StoreMessage`/`RetrieveMessages`/`StoreObfuscatedMessage` keep or hand back caller slice fields (`EncryptedData`/`EncryptedPayload`) and pointers, so post-store mutation can corrupt stored data or indexes. — **Remediation:** copy ciphertext/byte fields on store and on retrieval; deep-copy obfuscated messages before indexing. Validate with a mutation test.

- [ ] **M-19 `AsyncClient.Close` is not idempotent** — `async/client.go:145-152` — bug class: API — `Close` blindly closes `stopChan`; a second or concurrent call panics on close-of-closed-channel, despite being a public lifecycle method. — **Remediation:** guard with `sync.Once` or a closed flag. Validate with a double-Close test.

- [ ] **M-20 `Subscriber.Handler` read without lock during delivery** — `async/push_notifications.go:345-390` — bug class: Concurrency — `Subscribe` can replace an existing subscriber's `Handler` under `h.mu` while `deliveryLoop`/`deliverWithTimeout` read and invoke `sub.Handler` without a lock, racing a resubscribe (possible nil call). — **Remediation:** capture `Handler` under the hub lock (or use atomic) before invocation. Validate with `go test -race ./async/...`.

- [ ] **M-21 Unbounded LAN-input maps and SSDP/SOAP reads** — `dht/mdns_discovery.go:540-555`, `transport/upnp_client.go:149, 341` — bug class: Resources/Security — mDNS `knownPeers` grows with random LAN public keys until a 10-minute cleanup (no cap/rate limit), and UPnP device-description/SOAP responses use unbounded `io.ReadAll`, so a malicious LAN gateway can exhaust memory and amplify logs. — **Remediation:** cap/rate-limit `knownPeers`; wrap UPnP reads in `io.LimitReader` and truncate bodies in error messages. Validate with size-limit tests.

### LOW

- [ ] **L-01 `crypto/doc.go` examples reference non-existent APIs** — `crypto/doc.go:24-35, 70-72` — bug class: Documentation — the GoDoc shows `EncryptWithPeer`, `DecryptWithPeer`, `SharedSecret`, `SymmetricEncrypt`, `StoreKey`, `LoadKey`, none of which exist (actual: `Encrypt`, `Decrypt`, `DeriveSharedSecret`, `EncryptSymmetric`); copy-pasting the docs does not compile. — **Remediation:** update examples to the real API names. (Doc-only; no test needed.)

- [ ] **L-02 `clearCallbacks` omits several callbacks despite "all callbacks" comment** — `toxcore_lifecycle.go:178-200` — bug class: Resources/Documentation — file/name/status/typing/deleted callbacks are not cleared, retaining references after `Kill`. — **Remediation:** clear every callback field (under `callbackMu`, per H-06).

- [ ] **L-03 Generated private-key buffers not wiped after copy** — `crypto/keypair.go:37`, `crypto/ed25519.go:123, 142` — bug class: Security — `GenerateKeyPair`/`GenerateEd25519KeyPair`/`Ed25519PrivateKeyFromSeed` copy the private key into the return value but never `ZeroBytes` the originating buffer, leaving duplicate key material until GC. — **Remediation:** `defer ZeroBytes(...)` on the source buffers after copying. Validate with `go test ./crypto/...`.

- [ ] **L-04 `SecureFieldHash` returns a raw prefix, not a hash** — `crypto/logging.go:143` — bug class: Security — it returns a hex prefix of the sensitive input, so callers logging it leak up to 8 bytes of key/secret material. — **Remediation:** hash with SHA-256/HMAC and log a digest prefix, or rename/document as a non-secret preview.

- [ ] **L-05 `LoggerHelper.WithError(nil, …)` panics** — `crypto/logging.go:96` — bug class: Nil — dereferences `err.Error()` without a nil check. — **Remediation:** handle nil errors explicitly.

- [ ] **L-06 Replay cleanup uses `<` instead of `<=`** — `crypto/replay_protection.go:280` — bug class: Logic — a nonce expiring exactly at `now` is retained one extra second. — **Remediation:** use `expiry <= now`.

- [ ] **L-07 `LamportClock.Update` can wrap at `MaxUint64`** — `async/lamport.go:45-55` — bug class: Logic — `max(current,received)+1` overflows to 0 for `MaxUint64` input, breaking causal ordering. — **Remediation:** saturate or return an error on overflow.

- [ ] **L-08 `CalculateDynamicRecipientLimit` divides by unchecked `CapacityDivisor`** — `async/storage.go:150-158` — bug class: API — a caller `CapacityDivisor==0` panics. — **Remediation:** validate `> 0`.

- [ ] **L-09 `RetrievalScheduler.Configure` accepts non-positive intervals** — `async/retrieval_scheduler.go:185-195` — bug class: API — a `<=0` base interval makes `calculateNextInterval` return `<=0`, spinning the retrieval loop on immediate `time.After` (CPU/network churn). — **Remediation:** validate positive interval, jitter range, and ratio.

- [ ] **L-10 Address-length byte truncates long addresses** — `async/storage_discovery.go:280-290` — bug class: Logic — `SerializeBinary` writes a one-byte address length but the full address; addresses >255 bytes deserialize truncated. — **Remediation:** reject >255-byte addresses or use a wider length field with exact-length validation.

- [ ] **L-11 `real.attemptDeliveryWithRetries` sends zero times when `RetryAttempts==0`** — `real/packet_delivery.go:115` — bug class: Logic/API — the loop `attempt < RetryAttempts` means a `0` value (or an unset config) silently never transmits the packet and returns failure; the field name "retries" is also misleading (it is the total attempt count). — **Remediation:** clamp to a minimum of 1, or rename and document as total attempts. Validate with a `RetryAttempts=0` test.

- [ ] **L-12 `dhRatchetStep` mutates ratchet state before fallible KDF steps** — `ratchet/session.go:175-200` — bug class: Logic — `pn/ns/nr/dhr/rk/ckr` are updated before later key-generation/KDF calls that may error, leaving partial state on failure. — **Remediation:** compute into locals and commit atomically after all error checks. Validate with `go test ./ratchet/...`.

- [ ] **L-13 Internal pointers returned from getters allow unlocked mutation** — `group/chat.go:1164, 1353`, `friend/request.go:350`, `noise/psk_resumption.go:221`, `toxcore_conference.go:109` — bug class: Aliasing/Concurrency — `GetPeer`/`GetPeerList`/`GetPendingRequests`/`GetTicket(ByID)`/`ValidateConferenceAccess` return internal pointers (including PSK-bearing tickets) that callers can mutate outside the owning lock, racing internal state or bypassing role checks. — **Remediation:** return copies/immutable views (and wipe replaced secrets in the PSK store). Validate with `go test -race`.

- [ ] **L-14 Discarded/`%v`-wrapped errors lose context** — `toxnet/packet_listener.go:132` (discarded `SetReadDeadline` error), `toxcore_messaging.go:146`, `av/types.go:1107, 1200`, `toxav.go:1211` — bug class: Errors — deadline-set failures are ignored before `ReadFrom`, and several error returns use `%v`/drop `Close()` errors, breaking `errors.Is/As` and hiding cleanup failures. — **Remediation:** check the deadline error and route it through the error handler; use `%w`; collect/log `Close()` errors.

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total functions | 1,303 (+ 3,029 methods) |
| Files processed (skip-tests) | 251 |
| Total LOC (skip-tests) | 43,059 |
| Functions above complexity 15 | 1 (`async.ImportPreKeys`, 17.1) |
| Functions > 50 lines | 20 (0.5%) |
| Avg cyclomatic complexity | 3.5 |
| Doc coverage (overall) | 93.4% |
| Duplication ratio | 0.49% (largest clone 17 lines) |
| Test pass rate | 35/35 testable packages, `-race` clean |
| `go vet` warnings | 0 |

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|----------------|
| `async/client.go` send-on-closed response channel (prior GAPS F-L1) | Refactored: channel is now intentionally never closed and `sendResponseToChannel` uses a non-blocking `select`/`default` (`client.go:1228-1297`); no panic path remains. |
| `crypto` use of `math/rand` for keys/nonces | All key/nonce/nospam generation uses `crypto/rand`. |
| `crypto` MAC/key comparison with `==`/`bytes.Equal` | Secret comparisons use `crypto/subtle`; no reachable variable-time compare found. |
| GCM nonce reuse in keystore | 96-bit `crypto/rand` nonce per write; no deterministic reuse observed. |
| `SecureWipe` self-aliasing `subtle.XORBytes(d,d,d)` | Exact overlap is permitted by the Go docs. |
| `transportArg.(transport.Transport)` panic in `SetupMedia` | Guarded by comma-ok; the real bug is silent RTP disablement (C-01), not a panic. |
| `CallState` "bitmask vs enum" mismatch | Code is enum-style and internally consistent; matches README. |
| ToxAV audio defaults (48 kHz mono / 64 kbps) | Match README. |
| VP8 inter-frame "cached keyframe" behavior | Explicitly documented in `decodeFrameData` and README. |
| `transport/parser.go:215` address-length slice | Length byte is bounds-checked before slicing (legacy parser). |
| `dht/gossip_bootstrap.go` peer cache | Bounds-checked with a `MaxCachedPeers` cap. |
| `transport/relay_mux.go` frame length | Capped before allocation. |
| `transport/nat.go:27` init panic | Documented invariant for a constant fallback address. |
| `async/prekeys.go:599` pre-key double-use TOCTOU | `CheckAndMarkPreKeyUsed` holds `pks.mutex` across check+wipe+mark+persist. |
| `async/epoch.go:122` unsigned underflow | Guarded by a prior `epoch > currentEpoch` check. |
| `async/push_notifications.go:205` send-to-closed `Queue` | `Notify` holds `h.mu.RLock` during send, blocking concurrent `Unsubscribe` close. |
| `messaging` priority-queue type assertions | Heap only ever receives internal `*PriorityItem`. |
| `tor_transport_impl.go` `recover` | Converts a known library panic into an error; not silent swallowing. |
| Root-package savedata slice aliasing in `GetSavedata` | `marshal` returns freshly allocated bytes, not internal backing. |

## Remaining Scope (session completed)

A full pass over all 27 shipping packages was completed; no package remains
un-audited. Example programs (`examples/`, `toxnet/example*`) and the separate
`testnet/` module were intentionally excluded as non-shipping and are the only
suggested follow-up scope. Dependency CVE scanning (`govulncheck`) could not be run
(the vulnerability feed is unreachable from the sandbox) and is recommended as a
CI gate — see `GAPS.md`.
