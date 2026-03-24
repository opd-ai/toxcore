# Implementation Gaps — 2026-03-22

This document identifies gaps between toxcore-go's stated goals and current implementation, with actionable guidance for closing each gap.

---

## ToxAV Audio Codec (Opus Encoding) ✅ RESOLVED

- **Stated Goal**: "Audio calling with Opus codec support" — README ToxAV section claims high-quality audio with Opus codec.

- **Current State**: The `OpusCodec` struct in `av/audio/codec.go` uses `MagnumOpusEncoder` which wraps the `opd-ai/magnum` pure-Go Opus encoder for full Opus compression. The `magnum.Decoder` handles decoding. Both encoding and decoding are fully implemented with SILK (8/16 kHz) and CELT (24/48 kHz) codec paths.

- **Resolution**:
  1. Replaced `pion/opus` with `opd-ai/magnum` for both encoding and decoding
  2. Implemented `MagnumOpusEncoder` wrapping `magnum.Encoder` with VoIP application mode
  3. Round-trip encode→decode tests pass with proper Opus compression
  4. Bitrate configuration supported (8-512 kbps)

---

## ToxAV Video Codec (VP8 Encoding)

- **Stated Goal**: "Video calling with configurable quality" — README promises video transmission with adjustable bitrates.

- **Current State**: The `VP8Codec` struct in `av/video/codec.go:13-82` uses a "simple encoder implementation" (line 43 comment). The decoder works via pure Go implementation, but encoding is not production-grade.

- **Impact**:
  - Video quality is lower than expected for configured bitrates
  - Compression efficiency is suboptimal
  - High bandwidth consumption for video calls
  - Potential interoperability issues with c-toxcore video streams

- **Closing the Gap**:
  1. Evaluate VP8 encoder options: CGo binding to libvpx or pure Go implementation
  2. Implement configurable quality presets (low/medium/high)
  3. Add frame rate control and keyframe interval configuration
  4. Implement temporal scalability for adaptive streaming
  5. Add benchmark tests: `go test -bench=. ./av/video/...`

---

## Nym Network Service Hosting

- **Stated Goal**: README network support table shows Nym with "Dial ✅" implying bidirectional capability for privacy-focused applications.

- **Current State**: In `transport/nym_transport_impl.go`:
  - `Dial()` works via SOCKS5 proxy (lines 106-134)
  - `Listen()` returns error "nym: listening not supported via SOCKS5" (lines 90-101)
  - `DialPacket()` emulates UDP via TCP framing (lines 195-240), not true UDP

- **Impact**:
  - Users cannot host Tox nodes reachable via Nym addresses
  - Nym support is asymmetric (outbound only)
  - Privacy-focused users expecting full Nym integration are limited

- **Closing the Gap**:
  1. Document current Nym limitations clearly in README network table
  2. Evaluate Nym SDK websocket client for service hosting (noted in code comment at line 18)
  3. Implement Nym service provider registration for inbound connections
  4. Add configuration for Nym mixnet parameters (hops, delays)
  5. Update README to accurately reflect "Dial only" status until Listen is implemented

---

## Lokinet Network Support

- **Stated Goal**: README shows Lokinet with TCP Dial capability for .loki addresses.

- **Current State**: In `transport/lokinet_transport_impl.go`:
  - `Dial()` works via SOCKS5 proxy (lines 96-145)
  - `Listen()` returns error "lokinet: listening requires SNApp configuration" (lines 81-92)
  - `DialPacket()` returns error "lokinet: UDP not supported via SOCKS5 proxy" (lines 149-156)

- **Impact**:
  - Users cannot host Tox SNApps on Lokinet
  - No UDP support limits DHT functionality over Lokinet
  - Service hosting requires manual Lokinet configuration outside toxcore

- **Closing the Gap**:
  1. Update README network table to show "Listen ❌" and "UDP ❌" for Lokinet
  2. Document manual SNApp configuration requirements for advanced users
  3. Investigate lokinet-go bindings for native SNApp creation
  4. Consider Lokinet RPC API integration for programmatic SNApp setup
  5. Add example demonstrating Lokinet dial-only usage

---

## Friend Online Status Verification Before Calls

- **Stated Goal**: README ToxAV documentation shows initiating calls to friends, implying proper connection checking.

- **Current State**: `StartCall()` in `av/manager.go:1000-1120` creates call structures without verifying the friend's `ConnectionStatus`. Calls may be initiated to offline friends.

- **Impact**:
  - Call requests sent to offline friends fail silently or timeout
  - Resources allocated for calls that cannot succeed
  - User experience degraded with unclear failure modes

- **Closing the Gap**:
  1. Add `ConnectionStatus` check at the start of `StartCall()`
  2. Return `ErrFriendOffline` if status is `ConnectionNone`
  3. Optionally queue call request for when friend comes online
  4. Add test case for calling offline friend scenario
  5. Validate: `go test -race -run TestCallOfflineFriend ./av/...`

---

## Friend Deletion Resource Cleanup

- **Stated Goal**: `DeleteFriend()` should cleanly remove all friend-associated resources.

- **Current State**: `DeleteFriend()` in `toxcore.go:3147-3152` only removes the friend from the store. No cleanup of:
  - Pending file transfers
  - Queued async messages
  - Active call sessions

- **Impact**:
  - Orphaned file transfer state may accumulate
  - Async messages to deleted friends consume storage
  - Memory leaks from uncleaned call state

- **Closing the Gap**:
  1. Add `file.CancelTransfersForFriend(friendID)` call in DeleteFriend
  2. Add `asyncManager.ClearMessagesForRecipient(friendPK)` call
  3. Add `toxav.EndCallIfActive(friendID)` call
  4. Add test verifying resource cleanup on friend deletion
  5. Validate: `go test -race -run TestDeleteFriendCleanup ./...`

---

## Pre-Key vs Epoch Terminology Clarity

- **Stated Goal**: README claims "forward secrecy via epoch-based pre-key rotation" in the async messaging section.

- **Current State**: Implementation uses two separate mechanisms:
  1. **Pre-keys**: Consumed one-time per message, refreshed at threshold (20 remaining) — `async/forward_secrecy.go:195-211`
  2. **Epochs**: 6-hour windows that rotate recipient pseudonyms — `async/epoch.go:8-10`, `async/obfs.go:62-77`

- **Impact**:
  - Documentation confusion about forward secrecy mechanism
  - Users may misunderstand the security model
  - Epochs don't rotate pre-keys; they rotate pseudonyms

- **Closing the Gap**:
  1. Update README async section to clarify: "Forward secrecy via one-time pre-key consumption with epoch-based pseudonym rotation"
  2. Add explanation that pre-keys provide cryptographic forward secrecy
  3. Add explanation that epochs provide metadata privacy via pseudonym rotation
  4. Document the 6-hour epoch window and its purpose
  5. Add security documentation section explaining both mechanisms

---

## Automatic Storage Node Participation Documentation

- **Stated Goal**: README says "Users can become storage nodes when async manager initialization succeeds" suggesting optional participation.

- **Current State**: Storage node participation is automatic and mandatory when `NewMessageStorage()` is called (`async/storage.go:176-188`). Users automatically contribute 1% of available disk space with no opt-out.

- **Impact**:
  - Users unaware their disk space is being used
  - No configuration to disable storage node behavior
  - May conflict with resource-constrained environments

- **Closing the Gap**:
  1. Add `StorageNodeEnabled bool` option to async manager configuration
  2. Default to `true` for backward compatibility
  3. When `false`, skip `NewMessageStorage()` initialization
  4. Document storage node behavior prominently in README
  5. Document disk space allocation (1% with 1MB-1GB bounds)

---

## Message Delivery Confirmation

- **Stated Goal**: README shows reliable messaging with online/offline delivery paths.

- **Current State**: `SendFriendMessage()` returns success when message is queued, not when delivered. No delivery receipt mechanism exists in `messaging/message.go`.

- **Impact**:
  - Senders don't know if messages were received
  - No retry logic for failed deliveries
  - Users may assume delivery when message is stuck

- **Closing the Gap**:
  1. Design delivery receipt packet type per Tox protocol
  2. Implement receipt callback: `OnMessageDelivered(friendID, messageID)`
  3. Store pending message IDs until receipt confirmed
  4. Implement configurable retry with exponential backoff
  5. Document delivery semantics in README messaging section

---

## flynn/noise Dependency Security

- **Stated Goal**: Secure cryptographic implementation with Noise Protocol Framework.

- **Current State**: Using `flynn/noise v1.1.0` which has known nonce handling vulnerability (GHSA-g9mp-8g3h-3c5c). The project mitigates with rekey threshold at 2^32 messages (`transport/noise_transport.go:50`), but the underlying issue remains.

- **Impact**:
  - Theoretical nonce overflow risk (practically very low)
  - Security audit findings may flag the dependency
  - Compliance requirements may require patched version

- **Closing the Gap**:
  1. Monitor flynn/noise repository for security patches
  2. Update to patched version when available
  3. Current mitigation (2^32 rekey threshold) provides defense-in-depth
  4. Document mitigation in `docs/SECURITY_AUDIT_REPORT.md`
  5. Consider contributing patch upstream if maintainer unresponsive

---

## Summary Table

| Gap | Severity | Effort | Priority |
|-----|----------|--------|----------|
| Opus encoding not implemented | HIGH | Medium | 1 |
| VP8 encoding simplified | HIGH | Medium | 2 |
| Nym Listen() not supported | HIGH | High | 3 |
| Lokinet Listen()/UDP not supported | HIGH | High | 4 |
| flynn/noise vulnerability | HIGH | Low | 5 |
| Call online status check | MEDIUM | Low | 6 |
| DeleteFriend cleanup | MEDIUM | Low | 7 |
| Pre-key terminology | MEDIUM | Low | 8 |
| Storage node documentation | LOW | Low | 9 |
| Message delivery receipts | MEDIUM | Medium | 10 |

**Effort Scale**: Low (<1 day), Medium (1-3 days), High (>3 days)

---

## Verification Commands

```bash
# Verify Opus codec implementation status
grep -n "MagnumOpusEncoder\|magnum" av/audio/processor.go

# Verify VP8 encoder status
grep -n "simple encoder" av/video/codec.go

# Check Nym/Lokinet Listen implementation
grep -A5 "func.*Listen" transport/nym_transport_impl.go transport/lokinet_transport_impl.go

# Check flynn/noise version
go list -m github.com/flynn/noise

# Verify rekey threshold mitigation
grep -n "rekeyThreshold\|DefaultRekeyThreshold" transport/noise_transport.go

# Run full test suite
go test -race ./...
```
