# AUDIT — 2026-03-25

## Project Goals

**toxcore-go** is a pure Go implementation of the Tox peer-to-peer encrypted messaging protocol. According to the README, it promises:

1. **Pure Go implementation** with no CGo dependencies (core libraries)
2. **Comprehensive Tox protocol implementation** including DHT, friend management, messaging
3. **Multi-network support**: IPv4, IPv6, Tor .onion, I2P .b32.i2p, Nym .nym, Lokinet .loki
4. **Noise Protocol Framework** integration (IK pattern) for enhanced security
5. **Forward secrecy** via epoch-based pre-key rotation
6. **Identity obfuscation** to protect metadata from storage nodes
7. **Audio/Video calling** (ToxAV) with Opus and VP8 codecs
8. **Asynchronous offline messaging** with automatic storage node participation
9. **File transfers** with pause/resume/cancel
10. **Group chat** with DHT-based discovery
11. **C API bindings** via the capi package

**Target Audience**: Developers building privacy-focused communication applications, researchers working on decentralized protocols, and contributors to the Tox ecosystem.

---

## Goal-Achievement Summary

| Goal | Status | Evidence |
|------|--------|----------|
| Pure Go implementation | ✅ Achieved | go.mod shows no cgo deps except capi/; 221 Go files, all tests pass |
| DHT peer discovery | ✅ Achieved | dht/routing.go, dht/bootstrap.go; 135 tests pass |
| Friend management | ✅ Achieved | friend/manager.go; callbacks registered and working |
| 1-to-1 messaging | ✅ Achieved | messaging/message.go; SendFriendMessage works |
| Multi-network transport | ⚠️ Partial | Tor/I2P Listen+Dial work; Lokinet/Nym Dial-only (no Listen) |
| Noise Protocol IK | ✅ Achieved | transport/noise_transport.go:1052 lines; full handshake |
| Forward secrecy | ✅ Achieved | async/forward_secrecy.go:450 lines; pre-keys working |
| Identity obfuscation | ✅ Achieved | async/obfs.go:418 lines; BUT docs/OBFS.md incorrectly says "Design Document" |
| ToxAV audio calling | ✅ Achieved | av/audio/ with real Opus via opd-ai/magnum |
| ToxAV video calling | ⚠️ Partial | av/video/ with VP8 but I-frames only (5-10x bandwidth) |
| Async offline messaging | ⚠️ Partial | async/ 85% complete; no on-disk persistence or DHT discovery |
| File transfers | ⚠️ Partial | file/ package exists; callbacks never invoked |
| Group chat | ⚠️ Partial | group/chat.go works; no encryption on broadcast, peer discovery incomplete |
| C API bindings | ✅ Achieved | capi/ package with comprehensive bindings |

---

## Findings

### CRITICAL

- [x] **File transfer callbacks never invoked** — toxcore_callbacks.go:129-146 — `OnFileRecv`, `OnFileRecvChunk`, `OnFileChunkRequest` callbacks are registered but the code paths that invoke them (`fileRecvCallback()`, `fileRecvChunkCallback()`, `fileChunkRequestCallback()`) are never called. Incoming file transfers cannot be received. — **Remediation:** Add callback invocation in packet handlers where `PacketFileRequest`, `PacketFileData` are processed. Trace `handleFileRequest()` in file/manager.go and wire it to toxcore's packet dispatch. Verify with: `go test -v -run TestFileTransferE2E ./...` (after adding integration test). — **FIXED:** Added `SetFileRecvCallback`, `SetFileRecvChunkCallback`, `SetFileChunkRequestCallback` setters to `file.Manager`, invoked callbacks in handlers, wired from toxcore in `initializeFileManager`.

- [x] **OBFS.md incorrectly states "Design Document"** — docs/OBFS.md:5 — The document claims `Status: Design Document` but async/obfs.go contains 418 lines of fully implemented, production-ready identity obfuscation code including `GenerateRecipientPseudonym()`, `CreateObfuscatedMessage()`, `EncryptPayload()`. This misleads users about feature availability. — **Remediation:** Update docs/OBFS.md line 5 to `Status: Implemented in toxcore-go v1.0+`. Verify: `grep -n "Status:" docs/OBFS.md`. — **FIXED:** Updated Status to "Implemented in toxcore-go v1.0+".

### HIGH

- [x] **VP8 video encoding limited to I-frames only** — av/video/processor.go:41-160 — The `RealVP8Encoder` uses opd-ai/vp8 which only produces key frames (no P-frames or B-frames). This results in approximately 5-10x higher bandwidth usage than standard VP8. 720p@30fps requires 5-10 Mbps instead of 500K-1M. Video calling is impractical on bandwidth-constrained networks. — **Remediation:** Document this limitation prominently in README ToxAV section (already partially done). Long-term: integrate a VP8 encoder supporting inter-frame prediction, or use WebRTC-compatible alternative. Verify current state: `grep -n "key frame" av/video/*.go README.md`. — **FIXED:** README already documents this; added detailed "Current Video Codec Limitations" section to docs/TOXAV_BENCHMARKING.md with mitigation strategies.

- [x] **Async message storage is in-memory only** — async/storage.go — Messages stored by `MessageStorage` are lost on process restart. No on-disk persistence despite WAL framework existing. Users expecting offline message delivery will lose messages if storage node restarts. — **Remediation:** Implement disk-backed storage using the existing WAL framework pattern in async/. Add crash recovery with message replay. Verify: Add integration test that restarts storage node and confirms message recovery. — **FIXED:** WAL was already implemented but not enabled by default. Now `NewAsyncManager` automatically enables WAL when dataDir is provided and attempts recovery on startup.

- [ ] **Group chat broadcast messages are unencrypted** — group/chat.go:SendMessage — Despite `sender_key.go` existing with group encryption primitives, `SendMessage()` broadcasts JSON-encoded messages in plaintext. Anyone on the network can read group messages. — **Remediation:** Integrate `sender_key.go` encryption into the broadcast path. Encrypt payloads before JSON encoding. Verify: `grep -n "Encrypt" group/chat.go` should show encryption calls.

- [ ] **Lokinet not registered in MultiTransport** — transport/multi_transport.go:32-35 — `NewMultiTransport()` registers Tor, I2P, and Nym transports but NOT Lokinet despite `lokinet_transport_impl.go` existing. `.loki` addresses silently fall through to the default IP transport, causing connection failures. — **Remediation:** Add `registerLokinetTransport()` call in `NewMultiTransport()` initializer. Verify: `go test -v -run TestMultiTransportLokinet ./transport/...`.

### MEDIUM

- [ ] **StartCall() doesn't verify friend online status** — toxav.go — `StartCall()` allocates resources for calls to potentially offline friends, wasting resources for up to 30 seconds until timeout. — **Remediation:** Add `GetFriendConnectionStatus()` check at start of `Call()` method. Return `ErrFriendOffline` immediately if status is `ConnectionNone`. Verify: `go test -v -run TestCallOfflineFriend ./...`.

- [ ] **Friend deletion doesn't end active ToxAV calls** — toxcore.go:DeleteFriend — When a friend is deleted, any active ToxAV call session is not terminated, leaving orphaned call state. — **Remediation:** Add `toxAV.EndCall(friendID)` call in `DeleteFriend()` before removing friend. Verify: Add test that deletes friend during active call and confirms cleanup.

- [ ] **Group peer discovery incomplete** — group/chat.go:Join — `Join()` finds group metadata via DHT but doesn't auto-discover existing peers. Manual `UpdatePeerAddress()` calls required. — **Remediation:** Implement peer list exchange after successful join. Query founder/known peers for current member list. Verify: Integration test with 3+ peers joining sequentially.

- [ ] **Async storage nodes require manual configuration** — async/manager.go — README claims "distributed network of storage nodes" but `AddStorageNode()` must be called manually. No automatic DHT-based discovery of storage nodes. — **Remediation:** Implement storage node announcement via DHT similar to group announcements. Auto-discover nodes during `AsyncManager.Start()`. Verify: Add discovery integration test.

- [ ] **Nym Listen() not implemented** — transport/nym_transport_impl.go:Listen — Returns `ErrNymNotImplemented` with message "requires Nym SDK websocket client integration". README claims Nym support without qualifying this limitation. — **Remediation:** Update README multi-network table to show "Dial only" for Nym Listen column. Long-term: implement Nym SDK integration. Verify: `grep -n "Listen" transport/nym_transport_impl.go`.

- [ ] **Lokinet Listen() not implemented** — transport/lokinet_transport_impl.go:Listen — Returns error "SNApp hosting not supported via SOCKS5". README claims Lokinet support without qualifying this limitation. — **Remediation:** Update README multi-network table to show "❌ Manual config" for Lokinet Listen column. Verify: `grep -n "Listen" transport/lokinet_transport_impl.go`.

### LOW

- [ ] **capi tox_conference_delete returns error** — capi/toxcore_c.go — C API stub for `tox_conference_delete()` returns error rather than implementing group deletion. C API users cannot delete groups. — **Remediation:** Implement group deletion by calling `group.Chat.Leave()` or similar. Verify: `grep -n "tox_conference_delete" capi/*.go`.

- [ ] **Package naming violations** — Multiple files — go-stats-generator reports 2 package name violations: `toxcore` (directory mismatch), `common` (generic name). Also 8 file naming violations including stuttering (`friend/friend_store.go`). — **Remediation:** Consider renaming `friend/friend_store.go` to `friend/store.go`. Low priority as functionality unaffected. Verify: `go-stats-generator analyze . --sections naming`.

- [ ] **DHT routing table capacity limited** — dht/routing.go — Fixed 2,048-node capacity (256 buckets × 8 nodes) suitable for <10K user networks but not global scale. — **Remediation:** Documented in GAPS.md; acceptable for current use cases. For global deployment, implement dynamic bucket resizing. Verify: `grep -n "KBucketSize" dht/routing.go`.

- [ ] **NewBootstrapManager disables versioned handshakes** — dht/bootstrap.go — Default constructor doesn't enable Noise-IK versioned handshakes because no private key is provided. Users must use `NewBootstrapManagerWithKeyPair()` for full security. — **Remediation:** Document in package godoc that `NewBootstrapManagerWithKeyPair()` is preferred. Verify: `grep -n "NewBootstrapManager" dht/bootstrap.go`.

---

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total Lines of Code | 39,731 |
| Total Functions | 1,049 |
| Total Methods | 2,697 |
| Total Structs | 387 |
| Total Interfaces | 37 |
| Total Packages | 24 |
| Total Files | 221 |
| Average Function Length | 13.0 lines |
| Average Complexity | 3.6 |
| High Complexity (>10) | 1 function |
| Functions >50 lines | 26 (0.7%) |
| Clone Pairs Detected | 38 |
| Duplication Ratio | 0.76% |
| Test Files | 135+ tests passing |
| go vet | ✅ Clean (0 warnings) |
| go test -race | ✅ All packages pass |

### Package Metrics (Largest by Function Count)

| Package | Functions | Structs | Files |
|---------|-----------|---------|-------|
| transport | 717 | 111 | 41 |
| async | 424 | 54 | 25 |
| dht | 406 | 52 | 18 |
| toxcore | 314 | 33 | 7 |
| av | 210 | 25 | 9 |

### Complexity Hotspots

| Function | Package | Lines | Complexity |
|----------|---------|-------|------------|
| receiveLoop | dht | 37 | 15.8 |
| runMainLoop | main (example) | 28 | 13.2 |
| run | main (testnet) | 93 | 12.7 |
| DeleteFriend | toxcore | 50 | 11.4 |
| Submit | transport | 40 | 11.4 |

---

## Verification Commands

```bash
# Baseline health check
go vet ./...
go test -tags nonet -race -short ./...

# Verify file transfer callback gap
grep -rn "fileRecvCallback(" . --include="*.go" | grep -v "="

# Verify OBFS.md status
grep -n "Status:" docs/OBFS.md

# Verify Lokinet in MultiTransport
grep -n "lokinet\|Lokinet" transport/multi_transport.go

# Run full test suite with coverage
go test -tags nonet -race -coverprofile=coverage.txt -covermode=atomic ./...

# Check complexity metrics
go-stats-generator analyze . --skip-tests --sections complexity
```
