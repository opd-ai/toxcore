# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-06-03

## Project Profile

**Purpose:** `toxcore-go` is a pure-Go implementation of the Tox peer-to-peer
encrypted messaging protocol. It provides DHT-based peer discovery, friend
management, 1-to-1 and group messaging, chunked file transfers, audio/video
calling (ToxAV), asynchronous offline messaging with forward secrecy, and
multi-network transport (UDP/TCP/Tor/I2P/Lokinet/Nym) — with no cgo in the core
library (`CGO_ENABLED=0` builds).

**Target users:** Go developers embedding a Tox client/relay; C/C++ consumers via
the libtoxcore-compatible `capi/` bindings.

**Deployment model:** Library linked into end-user P2P applications and into
bootstrap/storage nodes. Untrusted data enters from the network (DHT packets,
RTP media, async-storage envelopes, NAT/STUN/UPnP responses) and from persisted
savedata.

**Critical paths (deepest scrutiny):**
- `crypto/`, `noise/`, `ratchet/` — confidentiality, forward secrecy, KCI resistance.
- `async/` — offline store-and-forward, one-time pre-key forward secrecy, identity obfuscation.
- `dht/`, `transport/`, `toxnet/`, `bootstrap/` — the primary untrusted-input trust boundary.
- `av/`, `av/rtp/`, `av/audio/`, `av/video/`, `toxav.go` — real-time media from the network.
- root `toxcore` package, `friend/`, `group/`, `messaging/`, `file/` — instance lifecycle and state.

## Audit Scope

Audited every non-test `.go` file in the core library packages below (examples,
`testnet/`, and `simulation/` demo harnesses were inspected only where they
exercise core APIs). Five parallel deep passes covered the full checklist
(3b–3k) per package, followed by manual re-verification of every CRITICAL/HIGH
and most MEDIUM findings against current line numbers.

**go-stats-generator metrics (`--skip-tests`):**
- Total functions: 4,443 · methods/structs/interfaces: 429 structs, 40 interfaces, 27 packages
- Total LOC (non-test): 44,847
- Functions with cyclomatic complexity > 15: **1** (`cloneReflectValue`, cc 16)
- Average cyclomatic complexity: **2.40** (max 16)
- Documentation coverage: **93.6%** overall (packages 100%, functions 98.8%)
- Duplication ratio: **0.51%** (32 clone pairs, almost all in `examples/`)
- `go vet ./...`: **0 warnings**
- `go test -tags nonet -race ./...`: **34/34 packages pass, 0 failures, 0 data races detected**

This is a mature, well-documented, low-complexity codebase. The tool's
`bug_comments`/`potential_leaks` flags were verified to be false positives
(matches on the word "Debug" and on `for`-loops that already have
context/channel/ticker exit conditions).

## Coverage Log

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ratchet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bootstrap (+nodes) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport (+addressing) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/audio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/rtp | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| av/video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| toxcore (root) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| factory / real / simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| limits / interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary

| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| DHT routing / peer discovery | ✅ | — (L-05 observability only) |
| Friend management | ✅ | — |
| 1-to-1 messaging | ✅ | — |
| Group chat | ✅ | — (L-04 only) |
| File transfers | ✅ | — |
| **ToxAV audio/video calls** | ⚠️ | **H-02** (answering side sends no RTP media), M-01, M-09, M-10 |
| Async offline messaging | ✅ | H-05 (replay/accounting), M-02 |
| **Forward secrecy (one-time pre-keys)** | ⚠️ | **H-04** (refresh accounting broken), M-03 (silent fallback) |
| Double Ratchet (`ratchet/`) | ⚠️ | **H-01** (auth-before-commit violated) |
| Noise-IK handshakes | ✅ | M-04 (memory hygiene only) |
| Multi-network transport | ⚠️ | M-06 (UPnP SSRF), M-07 (Nym 32-bit DoS); Nym listen unsupported (see GAPS) |
| Cryptography / secure memory | ✅ | M-05, L-06 (wipe-on-error gaps) |
| C API bindings | ✅ | — |
| `net.*` interfaces | ✅ | — |

## Findings

### CRITICAL
*No confirmed CRITICAL findings.* `go vet` is clean and the full `-race` suite
passes; no confirmed data-corruption or remotely-triggerable memory-safety bug
was found on a default-configuration path.

### HIGH
- [x] Double Ratchet mutates session state before AEAD authentication — `ratchet/session.go:163` (and 168, 173–175) — security/state corruption — `RatchetDecrypt` performs `dhRatchetStep`, `skipMessageKeys`, and advances `ckr`/`nr` **before** the authenticating `decryptWithMsgKey` at line 177. A forged/tampered ciphertext whose header looks valid permanently advances the receive chain (and discards/creates skipped keys), so the *legitimate* next message can no longer be decrypted — a remote desynchronization/DoS. The Double Ratchet spec requires cloning state and committing only after successful decryption. — **Remediation:** in `RatchetDecrypt`, operate on a copy of the receive state (or buffer the derived `mk` and skipped-key changes), call `decryptWithMsgKey` first, and only commit `ckr`/`nr`/`dhr`/skipped mutations on success; retain skipped keys on auth failure. Validate with `go test -run 'TestRatchetDecrypt' -race ./ratchet` plus a new tampered-ciphertext-does-not-desync test.
- [x] ToxAV answering side never creates an RTP session (no media transmitted) — `av/manager.go:1232` — logic/init-order on a critical path — `AnswerCall` calls `call.SetupMedia(m.transport, …)` passing the manager transport **directly**, whereas `StartCall` first unwraps it via `GetUnderlyingTransport()` (`av/manager.go:1101–1115`). In the standard `toxcore.NewToxAV(tox)` deployment `m.transport` is a `toxAVTransportAdapter`, which fails the `transportArg.(transport.Transport)` assertion in `setupRTPSession` (`av/types.go:669–677`); that path logs "Audio/video will be processed but not transmitted via RTP" and returns nil. Result: the call *answerer* encodes media locally but sends **no** audio/video RTP, breaking bidirectional calling. — **Remediation:** mirror `StartCall` — unwrap `m.transport` through the `underlyingTransportProvider`/`GetUnderlyingTransport()` shim before calling `SetupMedia` in `AnswerCall`. Validate with `go test -tags nonet ./av -run AnswerCall` and an integration test asserting the answerer's `rtpSession != nil`.
- [x] Incoming bitrate-control packet writes call fields without the Call lock — `av/manager.go:535` — concurrency/data race — `handleBitrateControl` runs under the manager lock `m.mu` and writes `call.audioBitRate`/`call.videoBitRate` directly, but `Call.GetAudioBitRate`/`GetVideoBitRate` read those fields under `call.mu.RLock()` (`av/types.go:340–349`) and `m.mu` is a different mutex. A peer-driven bitrate packet therefore races with any reader/setter that uses `call.mu`. — **Remediation:** replace the direct writes with `call.SetAudioBitRate`/`SetVideoBitRate` (which lock `call.mu`). Validate with `go test -race -tags nonet ./av -run Bitrate -count=100`.
- [x] Pre-key refresh accounting counts already-used keys → forward-secrecy exhaustion — `async/prekeys.go:315` and `async/prekeys.go:387` — logic/security — On the receive side, consumed one-time pre-keys are marked `Used=true` but **kept** in `bundle.Keys` (`markKeyAsUsedSecurely`, lines 528–545). `NeedsRefresh` (line 315) and `GetRemainingKeyCount` (line 387, documented "number of unused keys") both return `len(bundle.Keys)`, which includes used keys. The README guarantees pre-keys are "auto-refreshed when fewer than 20 remain"; with this bug a bundle of 200 fully-consumed keys still reports 200, so refresh never fires and the low-watermark hook never warns. Eventually peers have no usable pre-keys and senders silently drop to the non-forward-secure fallback (see M-03). — **Remediation:** count only `!Used` keys in both `NeedsRefresh` and `GetRemainingKeyCount` (e.g. iterate and skip `Used`, or track `MaxKeys-UsedCount`). Validate with a consume-then-refresh unit test and `go test -tags nonet ./async`.
- [x] Duplicate obfuscated MessageID poisons storage index and quota — `async/storage.go:645` — replay/accounting/DoS — `storeAndIndexMessage` overwrites `obfuscatedMessages[MessageID]` (map, deduped) but **always appends** to `pseudonymIndex[...][epoch]` (line 655). `MessageID` is supplied by the (untrusted) sender and there is no duplicate check in `validateMessageForStorage` (lines 620–633). Replaying the same MessageID inflates the per-pseudonym count toward `MaxMessagesPerRecipient`, returns the message twice on retrieval, and leaves stale index pointers after a single delete/expiry (the map entry is gone but index entries remain). A malicious sender/relay can thus exhaust a recipient's quota or cause duplicate delivery. — **Remediation:** reject a `MessageID` already present in `obfuscatedMessages` (or make storage idempotent and remove *all* matching index entries on delete). Validate with a replay+delete unit test and `go test -tags nonet ./async`.

### MEDIUM
- [ ] RTP session pointer read without Call lock can nil-deref on teardown — `av/manager.go:632` — concurrency/nil safety — `receiveAudioRTPPacket` (and the video equivalent) dereference `call.rtpSession.ReceivePacket(...)` without holding `call.mu`; concurrent `EndCall`/`CleanupMedia` can set `rtpSession = nil`, producing a panic on the media-receive goroutine. Not currently caught by `-race` because no test races receive against teardown. — **Remediation:** snapshot `call.GetRTPSession()` (a locked getter) into a local and nil-check it before use. Validate with `go test -race -tags nonet ./av -count=100`.
- [x] One-time pre-key is consumed before ciphertext authentication — `async/forward_secrecy.go:493` — replay/DoS — `DecryptForwardSecureMessage` calls `CheckAndMarkPreKeyUsed` (irreversibly marks the key `Used`) *before* `crypto.Decrypt` at line 502. A peer who knows a valid `PreKeyID` can submit a garbage ciphertext that fails to decrypt yet still burns the recipient's one-time pre-key. (This path is `Deprecated` in favor of the obfuscated retrieval path, lowering severity.) — **Remediation:** reserve the key under lock, authenticate/decrypt, then commit the `Used` state only on success (preventing concurrent reuse during the window). Validate with a corrupt-ciphertext pre-key-retention test and `go test -tags nonet ./async`. **STATUS:** Documented with comment noting this path is deprecated and the recommended obfuscated path is unaffected; full remediation deferred pending refactoring consensus.
- [x] Async send silently downgrades configured forward secrecy to plaintext inner payload — `async/client.go:315` — security/API contract — When a `ForwardSecurityManager` is configured but no pre-keys are available for the recipient, `SendAsyncMessage` falls through to `createFallbackForwardSecureMessage`, which stores the padded message in `EncryptedData` with `PreKeyID=0` and no per-message forward secrecy (only the outer obfuscation layer). A caller who set up FS reasonably expects fail-closed behavior. (Honest code comments + a `Warn` exist; combined with H-04 the fallback is reached more often than intended.) — **Remediation:** when `fsm != nil` and pre-keys are unavailable, return an error or queue for retry instead of degrading; optionally gate the legacy fallback behind an explicit opt-in option. Validate with a no-prekeys send test and `go test -tags nonet ./async`. **STATUS:** Fixed to return error when FSM configured but pre-keys unavailable, enforcing fail-closed semantics.
- [x] PSK-resumption handshake never wipes the copied static private key — `noise/psk_resumption.go:545` — resource lifecycle/memory hygiene — `NewPSKHandshake` copies `StaticPrivKey` into `noiseConfig.StaticKeypair.Private`, but unlike the IK/XX handshakes there is no stored `staticPriv` slice and no completion-time wipe, so long-term private-key material lingers in heap memory after the handshake. — **Remediation:** retain the copied slice on `PSKHandshake` and `crypto.ZeroBytes` it on both initiator/responder completion paths, matching the IK/XX pattern. Validate with `go test -run TestPSKHandshake ./noise`. **STATUS:** Fixed by storing staticPriv in PSKHandshake struct and wiping in both responder completion (processResponderMessage) and initiator completion (ReadMessage via afterComplete callback).
- [x] Key-rotation error paths leave decrypted plaintext buffers unwiped — `crypto/keystore.go:420` (and `:472`) — security/resource lifecycle — `RotateKey` → `decryptAllFiles` may decrypt several key files and then `return` on a later file's error without `ZeroBytes`-ing the already-decrypted plaintext map; `reencryptWriteTempFiles` similarly skips wiping the failed file's plaintext. — **Remediation:** wipe every accumulated decrypted buffer (including the active failure buffer) on all error returns, e.g. via a `defer` that ranges the map. Validate with `go test -run TestEncryptedKeyStore_KeyRotation ./crypto`. **STATUS:** Fixed by adding explicit wipe loop in decryptAllFiles on error path, and defer in RotateKey to wipe fileData on all error returns (M-05).
- [ ] UPnP SSDP `LOCATION` is fetched without scheme/host validation (LAN SSRF) — `transport/upnp_client.go:139` — security/SSRF — The untrusted `LOCATION` URL from an SSDP response is fetched directly, and the device-description XML can then steer SOAP requests via absolute control URLs. On a hostile LAN this lets a spoofed SSDP responder drive the client to arbitrary internal hosts. — **Remediation:** require `http(s)`, restrict the host to a private/LAN range matching the SSDP responder/default gateway, and disable redirects on the description fetch. Validate with `go test ./transport -run TestUPnP`.
- [ ] Nym length-prefix overflows `int` on 32-bit, bypassing the size guard — `transport/nym_packetconn.go:38` — boundary safety/DoS — `pktLen` (uint32) is compared as `int(pktLen) > len(p)`. On 32-bit GOARCH a value ≥ `0x80000000` becomes negative, the guard is skipped, and `p[:pktLen]` then panics with a slice-bounds error (or `io.ReadFull` blocks on a huge length). The bytes originate from the Nym SOCKS5 stream. — **Remediation:** cap `pktLen` against a constant using `uint64`/explicit bound *before* converting to `int`. Validate with `GOARCH=386 go test ./transport -run TestNymPacketConn`.
- [ ] Negative `PipelineConfig` interval panics the exported iteration API — `iteration_pipelines.go:98` — API/boundary safety — `NewIterationPipelines` only defaults intervals that equal zero (lines 98–105); a negative `MessageInterval`/`DHTInterval`/`FriendInterval` flows through to `time.NewTicker`, which panics. `ToxRunWithPipelines`/`Start` are exported and reachable across the cgo boundary. — **Remediation:** treat `<= 0` as "use default" (or return an error). Validate with `go test ./... -run TestIterationPipelines`.
- [x] Remote/timeout call teardown deletes calls without media cleanup — `av/manager.go:488` — resource lifecycle — Peer `CallControlCancel`, rejected responses, timeouts, and completed-call removal `delete(m.calls, …)` without first invoking `call.CleanupMedia()`, leaving codecs/RTP sessions/adapters unclosed. — **Remediation:** call `CleanupMedia()` before every `delete(m.calls, …)` teardown path. Validate with `go test -tags nonet ./av -run Cleanup -count=10`.
- [x] `CleanupMedia` omits `BitrateAdapter.Close` (goroutine leak) — `av/types.go:1169` — resource lifecycle/goroutine leak — `BitrateAdapter` spawns callback goroutines tracked by `callbackWg`, but `CleanupMedia` never calls its `Close`, so those goroutines can outlive an ended call. — **Remediation:** close and nil `c.bitrateAdapter` inside `CleanupMedia`. Validate with `go test -race -tags nonet ./av -run BitrateAdapter -count=50`.

### LOW
- [ ] `GetSelfPublicKey` reads keypair without `selfMutex` — `toxcore_self.go:129` — concurrency — reads `t.keyPair.Public` unlocked while `Load`/`LoadSnapshot` can replace `t.keyPair` under lock; `SelfGetPublicKey` already locks correctly. — **Remediation:** copy the public key under `selfMutex.RLock()`. Validate with `go test -race ./... -run TestGetSelfPublicKey`.
- [ ] `SafetyNumber` reads keypair without `selfMutex` — `toxcore_self.go:152` — concurrency — same pattern as L-01 on the hot-reload path. — **Remediation:** snapshot the public key under `selfMutex.RLock()`. Validate with `go test -race ./...`.
- [ ] Zero-retry delivery reports "after 0 attempts" — `real/packet_delivery.go:186` — logic/error reporting — `attemptDeliveryWithRetries` clamps `RetryAttempts=0` to one real attempt but the failure message reports the original `0`. — **Remediation:** report the clamped attempt count. Validate with `go test ./real -run TestRealPacketDelivery`.
- [ ] Sender-key rotation reads `onKeyRotation` without lock — `group/sender_key.go:186` — concurrency (theoretical) — `RotateSenderKey` reads/invokes `skm.onKeyRotation` without `skm.mu` while `SetOnKeyRotation` writes under lock; in practice the setter is called at init. — **Remediation:** copy the callback under lock, invoke after unlock. Validate with `go test -race ./group`.
- [ ] DHT address stats updated without synchronization — `dht/handler.go:427` — concurrency (observability only) — concurrent `PacketSendNodes` handlers increment/reset `bm.addressStats` unsynchronized, corrupting metrics and tripping `-race` if exercised concurrently. — **Remediation:** guard with a mutex or use atomics. Validate with `go test -race ./dht`.
- [ ] Derived obfuscation keys not explicitly wiped after use — `async/obfs.go:347` — memory hygiene — `CreateObfuscatedMessage`/`validateAndDecryptPayload` keep `payloadKey`/`sharedSecret` copies live after AES-GCM ops. — **Remediation:** `defer crypto.ZeroBytes(payloadKey[:])` and wipe caller-side `sharedSecret`. Validate with `go test -tags nonet ./async`.
- [ ] Bandwidth preset multiply overflows uint32 — `av/video/presets.go:201` — integer overflow — `bandwidthKbps * 800` wraps above ~5.37 Mbps-kbps, selecting too-low quality at extreme bandwidths. — **Remediation:** compute/compare in `uint64`. Validate with `go test -tags nonet ./av/video -run Preset`.
- [ ] `NewMetricsAggregator(0)` panics on non-positive interval — `av/metrics.go:335` — boundary safety — a zero/negative interval reaches `time.NewTicker` and panics instead of returning an error. — **Remediation:** validate the interval in the constructor/`Start`. Validate with `go test -tags nonet ./av -run MetricsAggregator`.
- [ ] VP8 PictureID parsed from the wrong byte — `av/rtp/session.go:696` — RTP parsing — `deserializeVideoRTPPacket` treats the VP8 extension byte as the PictureID high byte; metadata is wrong for extended VP8 packets (the depacketizer reparses the payload, so playback is unaffected today). — **Remediation:** parse the RFC 7741 extension/I/M bits before PictureID. Validate with `go test -tags nonet ./av/rtp -run VP8`.
- [ ] Mixed-mode packet path may deliver undecryptable bytes as plaintext — `toxnet/packet_conn.go:633` — API contract (speculative) — with encryption enabled-but-not-required, an undecryptable packet is surfaced to the caller as plaintext. Labeled LOW/uncertain pending confirmation that "mixed mode" is a supported, documented configuration. — **Remediation:** default to strict drop, or tag mixed-mode packets so callers can distinguish. Validate with `go test ./toxnet -run TestPacketConn`.
- [ ] libvpx packet size can panic an unsafe slice (build-tag only) — `av/video/encoder_cgo.go:306` — boundary safety — under `-tags libvpx`, a `C.pktSz` larger than `maxCPlaneBytes` panics during `[:sz:sz]`. The size originates from libvpx (not the network), so impact is low. — **Remediation:** reject `sz < 0 || sz > maxCPlaneBytes || nil buf`. Validate with `go test -tags 'libvpx nonet' ./av/video`.

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total functions | 4,443 |
| Functions above complexity 15 | 1 |
| Avg cyclomatic complexity | 2.40 |
| Doc coverage | 93.6% |
| Duplication ratio | 0.51% |
| Test pass rate (`-race -tags nonet`) | 34/34 packages, 0 races |
| go vet warnings | 0 |

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|-----------------|
| `cloneReflectValue` shallow-copies unexported pointer fields (`toxcore_friends.go:288`) | Explicitly documented as L-4 with no reachable public setter exposing the pattern; acknowledged. |
| Noise IK responder cipher send/recv mapping | Comments and passing tests confirm correct initiator→responder cipher assignment. |
| Ratchet header decoding bounds (`DecodeHeader`) | Guarded by `len(buf) < HeaderSize` check. |
| File-transfer path traversal via peer filename | Incoming filenames reduced to `filepath.Base`; absolute/`..` paths rejected before open. |
| RTP/audio short-header parsing | Uses `pion/rtp.Unmarshal`; truncated headers are rejected upstream. |
| SSRC / sequence init uses weak RNG | Confirmed `crypto/rand`, not `math/rand`. |
| `async` storage-node loops flagged as leaks (`storage.go:177`, etc.) | Not infinite loops / use stop channels + tickers with proper exit. |
| DHT/relay/gossip/STUN/SOCKS5 length parsing | Traced to upstream length guards or fixed-size buffers. |
| Crypto nonce-persistence parse loop | Bounded by `offset+40 <= len(data)`. |
| ToxID checksum weakness | Documented protocol limitation, not used for cryptographic integrity. |
| Loop-variable capture in goroutines | go.mod is go 1.25 — per-iteration capture is the language default; not a bug. |

## Remaining Scope (session complete)
| Package | Status | Notes |
|---------|--------|-------|
| All core library packages | ✅ Audited | A full pass produced the findings above; the `examples/`, `testnet/` (separate module), and `simulation/` demo harnesses were reviewed only at their core-API boundaries and yielded no findings above LOW. |
