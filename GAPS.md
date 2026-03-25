# Implementation Gaps — 2026-03-25

This document identifies gaps between toxcore-go's stated goals and its current implementation.

---

## File Transfer Callbacks Not Invoked

- **Stated Goal**: README and doc.go describe full file transfer capability with `OnFileRecv`, `OnFileRecvChunk`, and `OnFileChunkRequest` callbacks for receiving files.
- **Current State**: The callbacks are registered in toxcore_callbacks.go (lines 129-146) but the functions that invoke them (`fileRecvCallback()`, `fileRecvChunkCallback()`, `fileChunkRequestCallback()`) are never called. The file/manager.go has `handleFileRequest()` and `handleFileData()` handlers, but these are not wired to toxcore's packet dispatch.
- **Impact**: Applications cannot receive incoming files. `FileSend()` works for outbound transfers, but the receiving side never gets notified. File transfer is effectively send-only.
- **Closing the Gap**: 
  1. Trace packet routing for `PacketFileRequest`, `PacketFileData`, `PacketFileControl` in toxcore.go
  2. Wire these packets to call `file.Manager` handlers
  3. Have handlers invoke the appropriate callbacks
  4. Add end-to-end integration test: send file from peer A, receive and verify on peer B

---

## Multi-Network Listen Support Incomplete

- **Stated Goal**: README claims "Multi-Network Support: IPv4, IPv6, Tor .onion, I2P .b32.i2p, Nym .nym, and Lokinet .loki" with a table showing Listen/Dial/UDP capabilities.
- **Current State**: 
  - **IPv4/IPv6**: ✅ Full Listen + Dial + UDP
  - **Tor .onion**: ✅ Full Listen + Dial (TCP only, as documented)
  - **I2P .b32.i2p**: ✅ Full Listen + Dial + UDP
  - **Nym .nym**: ❌ Dial only. `Listen()` returns `ErrNymNotImplemented` (requires Nym SDK websocket integration)
  - **Lokinet .loki**: ❌ Dial only. `Listen()` returns error (requires manual lokinet.ini SNApp configuration)
- **Impact**: Users expecting to host services on Nym or Lokinet cannot do so through the library API. They must configure external daemons manually.
- **Closing the Gap**:
  1. Update README table to accurately show "❌" or "Manual config" for Nym/Lokinet Listen columns
  2. Long-term: Implement Nym SDK websocket client for Listen support
  3. Long-term: Implement Lokinet API integration for programmatic SNApp creation
  4. Document workarounds for manual daemon configuration

---

## ~~Lokinet Not Registered in MultiTransport~~ ✅ RESOLVED

- **Stated Goal**: MultiTransport should automatically route `.loki` addresses to the Lokinet transport.
- **Current State**: ✅ RESOLVED — `NewMultiTransport()` in transport/multi_transport.go:35 now registers Lokinet: `mt.RegisterTransport("loki", NewLokinetTransport())`. Address routing at lines 82-83 properly selects the "loki" transport for `.loki` addresses.
- **Impact**: N/A — Gap is closed.
- **Evidence**: `transport/multi_transport.go:35`, `transport/multi_transport.go:82-83`

---

## Identity Obfuscation Documentation Mismatch

- **Stated Goal**: README describes identity obfuscation as a key feature protecting metadata from storage nodes.
- **Current State**: The feature is **fully implemented** in async/obfs.go (418 lines) with `GenerateRecipientPseudonym()`, `GenerateSenderPseudonym()`, `CreateObfuscatedMessage()`, `EncryptPayload()`, etc. However, docs/OBFS.md line 5 states `Status: Design Document`, incorrectly implying it's not implemented.
- **Impact**: Users consulting OBFS.md may believe the feature is unimplemented and avoid using it, or may implement their own solution unnecessarily.
- **Closing the Gap**:
  1. Update docs/OBFS.md line 5 to `Status: Implemented in toxcore-go v1.0+`
  2. Add cross-reference to async/obfs.go in the documentation
  3. Ensure consistency between all specification docs and implementation status

---

## VP8 Video Codec I-Frame Only Limitation

- **Stated Goal**: README describes "Video Calling: Video transmission with configurable quality" using VP8 codec.
- **Current State**: The `RealVP8Encoder` in av/video/processor.go uses opd-ai/vp8 which only produces key frames (I-frames). No inter-frame prediction (P-frames, B-frames) is implemented. This is documented in README line ~903 and GAPS.md.
- **Impact**: Video calling requires approximately 5-10x more bandwidth than standard VP8. 720p@30fps needs 5-10 Mbps instead of 500K-1M. Video calling is impractical on mobile networks or bandwidth-constrained connections.
- **Closing the Gap**:
  1. Current documentation is adequate—this is a known limitation
  2. Long-term: Replace opd-ai/vp8 with a VP8 encoder supporting temporal prediction
  3. Alternative: Consider WebRTC-compatible codecs or hardware acceleration paths
  4. Document bandwidth requirements prominently in ToxAV examples

---

## Async Message Storage Not Persistent

- **Stated Goal**: README describes "distributed storage nodes" that store offline messages for later delivery.
- **Current State**: `MessageStorage` in async/storage.go stores messages in memory only. No on-disk persistence is implemented despite a WAL framework existing. Messages are lost if the storage node process restarts.
- **Impact**: Users relying on async messaging for offline delivery will lose messages if any storage node restarts. The "distributed" nature doesn't help if all nodes lose state simultaneously (e.g., coordinated restarts).
- **Closing the Gap**:
  1. Implement disk-backed storage using append-only log or SQLite
  2. Add crash recovery that replays persisted messages on startup
  3. Implement message acknowledgment so senders know delivery succeeded
  4. Add integration test: store message, restart node, verify message survives

---

## Async Storage Node Discovery Not Automated

- **Stated Goal**: README describes "distributed network of storage nodes" with automatic participation.
- **Current State**: Storage nodes must be manually added via `AddStorageNode()` calls. No DHT-based discovery of storage nodes exists. The "automatic participation" only means nodes store messages if initialized, not that they discover each other.
- **Impact**: Users must manually configure storage node addresses. There's no way for a new node to discover existing storage infrastructure automatically.
- **Closing the Gap**:
  1. Implement storage node announcement via DHT (similar to group announcements)
  2. Add `DiscoverStorageNodes()` that queries DHT for announced nodes
  3. Auto-discover on `AsyncManager.Start()`
  4. Implement gossip protocol for storage node peer exchange

---

## Group Chat Encryption Not Applied

- **Stated Goal**: Group chat should provide secure communication like 1-to-1 messaging.
- **Current State**: `group/sender_key.go` implements group encryption primitives (sender keys, key rotation), but `group/chat.go:SendMessage()` broadcasts JSON-encoded messages in plaintext. The encryption layer is not integrated into the broadcast path.
- **Impact**: Anyone monitoring the network can read group chat messages. This defeats the purpose of a privacy-focused protocol.
- **Closing the Gap**:
  1. Integrate `sender_key.go` encryption into `SendMessage()` 
  2. Encrypt message payload before JSON encoding
  3. Implement key distribution for new group members
  4. Add tests verifying messages are encrypted on the wire

---

## Group Peer Discovery Incomplete

- **Stated Goal**: Group chat with DHT-based discovery allowing users to join groups by ID.
- **Current State**: `Join()` in group/chat.go finds group metadata via DHT but doesn't auto-discover existing group members. New joiners must manually call `UpdatePeerAddress()` for each known peer.
- **Impact**: Users joining a group see an empty peer list until peers manually announce themselves or addresses are shared out-of-band.
- **Closing the Gap**:
  1. Implement peer list exchange after successful join
  2. Query founder/known peers for current member list
  3. Broadcast join announcements to existing members
  4. Add periodic peer list refresh

---

## ToxAV Call Resource Management

- **Stated Goal**: ToxAV should handle call lifecycle correctly, including edge cases.
- **Current State**: 
  - `StartCall()` doesn't verify friend is online before allocating resources
  - `DeleteFriend()` doesn't terminate active ToxAV calls
- **Impact**: Resources wasted for 30s when calling offline friends. Orphaned call state when friends are deleted mid-call.
- **Closing the Gap**:
  1. Add `GetFriendConnectionStatus()` check at start of `Call()`
  2. Return `ErrFriendOffline` immediately if friend is offline
  3. Add `toxAV.EndCall(friendID)` in `DeleteFriend()` before friend removal
  4. Add tests for both scenarios

---

## DHT Routing Table Scalability

- **Stated Goal**: DHT should support peer discovery across the Tox network.
- **Current State**: Fixed 2,048-node routing table capacity (256 buckets × 8 nodes per bucket). Suitable for networks under ~10K users but not for global scale.
- **Impact**: In a large network, routing efficiency degrades as the table cannot hold enough nodes for optimal routing.
- **Closing the Gap**:
  1. Current implementation is adequate for expected deployment scale
  2. Document the limitation clearly
  3. Long-term: Implement dynamic bucket resizing based on network density
  4. Consider hierarchical routing for global scale

---

## C API Group Deletion Stub

- **Stated Goal**: C API should provide full Tox protocol functionality for cross-language use.
- **Current State**: `tox_conference_delete()` in capi/toxcore_c.go returns an error rather than implementing group deletion. C API users cannot delete groups they've created or joined.
- **Impact**: C/C++ applications using the capi bindings cannot clean up group chat resources.
- **Closing the Gap**:
  1. Implement group deletion by calling appropriate group.Chat methods
  2. Handle the case where group is owned vs. joined
  3. Add C API test for conference deletion

---

## Bootstrap Manager Default Configuration

- **Stated Goal**: Noise Protocol IK integration for enhanced security.
- **Current State**: `NewBootstrapManager()` disables versioned handshakes because no private key is provided. Users must use `NewBootstrapManagerWithKeyPair()` for Noise-IK support, but this isn't obvious from documentation.
- **Impact**: Users using the simpler constructor get reduced security (no Noise-IK) without realizing it.
- **Closing the Gap**:
  1. Update godoc for `NewBootstrapManager()` to note security limitation
  2. Recommend `NewBootstrapManagerWithKeyPair()` as preferred constructor
  3. Consider deprecating keyless constructor or auto-generating ephemeral keys

---

## Summary Priority Matrix

| Gap | Severity | Effort | Priority |
|-----|----------|--------|----------|
| ~~File transfer callbacks~~ | ~~CRITICAL~~ | ~~Medium~~ | ~~P0~~ ✅ RESOLVED |
| ~~OBFS.md status mismatch~~ | ~~CRITICAL~~ | ~~Low~~ | ~~P0~~ ✅ RESOLVED |
| Group chat encryption | HIGH | Medium | P1 |
| ~~Lokinet MultiTransport~~ | ~~HIGH~~ | ~~Low~~ | ~~P1~~ ✅ RESOLVED |
| Lokinet MultiTransport | HIGH | Low | P1 |
| Async storage persistence | HIGH | High | P1 |
| VP8 I-frame limitation | HIGH | High | P2 |
| Multi-network Listen gaps | MEDIUM | High | P2 |
| Async node discovery | MEDIUM | Medium | P2 |
| Group peer discovery | MEDIUM | Medium | P2 |
| ToxAV resource management | MEDIUM | Low | P2 |
| DHT scalability | LOW | High | P3 |
| C API group deletion | LOW | Low | P3 |
| Bootstrap manager docs | LOW | Low | P3 |
