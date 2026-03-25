# Implementation Gaps — 2026-03-25

This document identifies gaps between toxcore-go's stated goals (per README.md and documentation) and the current implementation.

---

## Lokinet Listen() Not Supported

- **Stated Goal**: README Multi-Network Support table lists "Lokinet .loki" as a supported network type.
- **Current State**: `transport/lokinet_transport_impl.go:81` — The `Listen()` method returns an explicit error: `"SNApp hosting not supported via SOCKS5: requires manual Lokinet configuration"`. Only `Dial()` is functional, routing traffic through the configured SOCKS5 proxy.
- **Impact**: Users cannot host Tox nodes reachable via .loki addresses. Lokinet users can connect to clearnet nodes but cannot receive incoming connections from other Lokinet users. This breaks peer-to-peer symmetry for Lokinet-only deployments.
- **Closing the Gap**: 
  1. Implement SNApp hosting by parsing `lokinet.ini` configuration and registering with the local lokinet daemon
  2. OR update README to clarify "Lokinet: Dial only — Listen requires manual lokinet.ini SNApp configuration"
  3. Validation: `go test -race -run TestLokinetListen ./transport/`

---

## Nym Listen() Not Supported

- **Stated Goal**: README Multi-Network Support table lists "Nym .nym" as a supported network type.
- **Current State**: `transport/nym_transport_impl.go:90` — The `Listen()` method returns `ErrNymNotImplemented` with message "requires Nym SDK websocket client integration". Only `Dial()` and `DialPacket()` (via length-prefixed TCP framing) are functional.
- **Impact**: Users cannot host Tox nodes reachable via .nym addresses. Nym provides stronger anonymity than Tor, but without Listen() support, users must rely on other transport layers for incoming connections.
- **Closing the Gap**:
  1. Integrate Nym websocket SDK for service provider hosting
  2. OR update README to clarify "Nym: Dial only — Listen requires Nym service provider configuration (out of scope)"
  3. Validation: `go test -race -run TestNymListen ./transport/`

---

## C API tox_conference_delete() Not Implemented

- **Stated Goal**: README documents "C binding annotations for cross-language use" and shows C API examples for group chat management.
- **Current State**: `capi/toxcore_c.go:952-980` — The `tox_conference_delete()` function logs a warning and returns error code 1. Comment states: "ConferenceDelete may need to be implemented in toxcore.go".
- **Impact**: C API users cannot leave or delete group chats. Groups persist indefinitely in memory, causing resource leaks in long-running applications using the C bindings.
- **Closing the Gap**:
  1. Implement the function body to call `group.Chat.Leave()` and remove from the conferences map
  2. Example implementation:
     ```go
     func tox_conference_delete(tox *C.Tox, conference_number C.uint32_t, error *C.TOX_ERR_CONFERENCE_DELETE) C.bool {
         t := getToxInstance(tox)
         if t == nil {
             setError(error, C.TOX_ERR_CONFERENCE_DELETE_CONFERENCE_NOT_FOUND)
             return C.false
         }
         if err := t.ConferenceDelete(uint32(conference_number)); err != nil {
             setError(error, C.TOX_ERR_CONFERENCE_DELETE_CONFERENCE_NOT_FOUND)
             return C.false
         }
         return C.true
     }
     ```
  3. Validation: `go build -buildmode=c-shared -o libtoxcore.so ./capi && nm libtoxcore.so | grep tox_conference_delete`

---

## VP8 Video Codec Limited to I-Frames Only

- **Stated Goal**: README ToxAV section promises "Video transmission with configurable quality" and "Video Calling: Video transmission with configurable quality".
- **Current State**: `av/video/processor.go:60-95` — `RealVP8Encoder` wraps `opd-ai/vp8.Encoder` which produces only key frames (I-frames). No temporal prediction (P-frames) or bidirectional prediction (B-frames) is implemented.
- **Impact**: Video calls use approximately 10x more bandwidth than necessary. A 720p call at 30fps requires ~5-10 Mbps instead of ~500 Kbps-1 Mbps with proper inter-frame encoding. This makes video calls impractical on bandwidth-constrained connections (mobile data, rural internet).
- **Closing the Gap**:
  1. Integrate a full VP8 encoder library supporting inter-frame prediction (e.g., libvpx via CGo, or wait for opd-ai/vp8 to add P-frame support)
  2. Implement temporal layer support in RTP packetization (`av/rtp/`)
  3. Add bitrate adaptation based on frame type (I-frames larger, P-frames smaller)
  4. Validation: `go test -bench=BenchmarkVP8Encode -benchmem ./av/video/ && ffprobe -show_frames output.ivf | grep -c "pict_type=I"`

---

## StartCall() Doesn't Verify Friend Online Status

- **Stated Goal**: ToxAV documentation shows call initiation with proper call state management and error handling.
- **Current State**: `av/manager.go:1069-1131` — `StartCall()` proceeds with call setup (generates call ID, serializes packet, creates session) without checking if the friend is currently online.
- **Impact**: Attempting to call an offline friend allocates resources (Call struct, RTP session, codecs) that are never used. The call eventually times out, but resources are held for the timeout duration (~30 seconds).
- **Closing the Gap**:
  1. Add online status check at the start of `StartCall()`:
     ```go
     func (m *Manager) StartCall(friendNumber, audioBitRate, videoBitRate uint32) error {
         if !m.isFriendOnline(friendNumber) {
             return ErrFriendOffline
         }
         // ... existing code
     }
     ```
  2. Expose `isFriendOnline()` helper or integrate with `friend.Manager.GetConnectionStatus()`
  3. Validation: `go test -race -run TestStartCallOfflineFriend ./av/`

---

## Friend Deletion Incomplete Resource Cleanup

- **Stated Goal**: README Friend Management section shows `DeleteFriend()` for removing friends.
- **Current State**: `toxcore.go:3246-3288` — `DeleteFriend()` removes the friend from FriendStore and broadcasts OnFriendDeleted callback, but does not:
  - Cancel active file transfers with that friend
  - Clear pending async messages for that recipient
  - End active ToxAV calls with that friend
- **Impact**: Deleting a friend leaves orphaned resources:
  - File transfers continue attempting to send/receive until timeout
  - Async messages remain in storage until TTL expiration (24 hours)
  - Active calls continue consuming resources until call timeout
- **Closing the Gap**:
  1. Add cleanup calls in `DeleteFriend()`:
     ```go
     func (t *Tox) DeleteFriend(friendID uint32) error {
         friend, err := t.friendStore.Get(friendID)
         if err != nil {
             return err
         }
         
         // Cleanup resources
         if t.fileManager != nil {
             t.fileManager.CancelTransfersForFriend(friendID)
         }
         if t.asyncManager != nil {
             t.asyncManager.ClearMessagesForRecipient(friend.PublicKey)
         }
         if t.toxav != nil {
             t.toxav.EndCallIfActive(friendID)
         }
         
         // ... existing deletion code
     }
     ```
  2. Validation: `go test -race -run TestDeleteFriendCleanup ./...`

---

## Message Delivery Confirmation Not Implemented

- **Stated Goal**: README Sending Messages section documents `SendFriendMessage()` with message delivery behavior: "Friend Online: Messages are delivered immediately via real-time messaging".
- **Current State**: `messaging/message.go` — `SendFriendMessage()` returns success when the message is queued for sending, not when it is acknowledged by the recipient. No delivery receipts or read receipts are implemented.
- **Impact**: Applications cannot distinguish between "message sent" and "message delivered". Users may believe a message was delivered when it was only queued locally. This is particularly problematic for:
  - Messages sent just before a friend goes offline
  - Network interruptions during transmission
  - Messages exceeding retry limits
- **Closing the Gap**:
  1. Implement message delivery receipts per Tox protocol specification:
     - Add `OnMessageDelivered(friendID, messageID)` callback
     - Track pending message IDs in a map
     - Send delivery receipt packet when message received
     - Fire callback when receipt received
  2. Optionally implement read receipts (requires friend client cooperation)
  3. Validation: `go test -race -run TestMessageDeliveryReceipt ./messaging/`

---

## DHT Routing Table Scalability Limits

- **Stated Goal**: REPORT.md §2 targets "5-8 billion concurrent users" for global messaging replacement.
- **Current State**: `dht/routing.go:72-79` — Routing table is fixed at 256 k-buckets × 8 nodes = 2,048 maximum entries. REPORT.md §3.1 documents this requires O(log₂(1B)) ≈ 33 network hops for billion-node peer discovery.
- **Impact**: The current architecture is suitable for small-to-medium deployments (1K-10K users per node) but cannot scale to global messaging. At billion-node scale:
  - Lookup latency: 33 hops × ~100ms RTT = 3+ seconds
  - Churn impact: 8 nodes per bucket means high probability of stale entries
  - Maintenance overhead: 2,048 pings/minute for keep-alive
- **Closing the Gap**:
  1. Phase 1 (0-6 months per REPORT.md §5): Implement dynamic bucket sizing based on network density
  2. Phase 2 (6-18 months): Add S/Kademlia extensions for Sybil resistance (partially implemented in `dht/skademlia.go`)
  3. Phase 3 (18-36 months): Implement hierarchical DHT with geographic routing
  4. Validation: `go test -bench=BenchmarkDHTLookup -benchmem ./dht/`

---

## Single-Threaded Iterate() Event Loop

- **Stated Goal**: README shows `tox.Iterate()` in a main loop for event processing. REPORT.md documents scalability concerns.
- **Current State**: `toxcore.go:1430-1444` — `Iterate()` sequentially processes:
  1. DHT maintenance (every 120 iterations = ~6 seconds)
  2. Friend connections (every 240 iterations = ~12 seconds)
  3. Message queue (every iteration)
  4. Friend request retries (every iteration)
  
  Default `IterationInterval()` returns 50ms, capping throughput at ~20 iterations/second.
- **Impact**: 
  - DHT maintenance delays block message delivery
  - Cannot utilize multi-core CPUs
  - Throughput ceiling regardless of hardware
  - Latency spikes when message queue is deep
- **Closing the Gap**:
  1. Enable concurrent iteration pipelines (`iteration_pipelines.go` already exists):
     ```go
     tox.SetIterationMode(toxcore.IterationModeConcurrent)
     ```
  2. Implement priority queues: real-time messages > DHT > file transfers
  3. Consider separate goroutines for each subsystem with channel-based coordination
  4. Validation: `go test -race -bench=BenchmarkIterateConcurrent ./...`

---

## Summary

| Gap | Severity | Effort | Priority |
|-----|----------|--------|----------|
| Lokinet Listen() | HIGH | Medium | P1 |
| Nym Listen() | HIGH | High | P2 |
| C API tox_conference_delete() | HIGH | Low | P1 |
| VP8 I-frames only | HIGH | High | P2 |
| StartCall() online check | MEDIUM | Low | P1 |
| Friend deletion cleanup | MEDIUM | Low | P1 |
| Message delivery confirmation | MEDIUM | Medium | P2 |
| DHT scalability | MEDIUM | Very High | P3 |
| Single-threaded Iterate() | MEDIUM | Medium | P2 |

**Recommended Priority Order:**
1. **Quick wins (P1, Low effort):** C API delete, StartCall check, friend cleanup
2. **Documentation updates:** Clarify Lokinet/Nym dial-only status in README
3. **Medium-term (P2):** Message receipts, concurrent iteration, VP8 improvements
4. **Long-term (P3):** DHT scalability per REPORT.md roadmap
