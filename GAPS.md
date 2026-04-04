# Implementation Gaps — 2026-04-04

This document identifies gaps between toxcore-go's stated goals (per README and documentation) and its current implementation.

---

## Port Prediction NAT Traversal

- **Stated Goal**: README line 1471 lists "port prediction" as a NAT traversal technique alongside UDP hole punching.
- **Current State**: No explicit port prediction algorithm found in the codebase. The `transport/hole_puncher.go` implements standard UDP hole punching with retries, and `transport/stun_client.go` discovers public IP/port, but there is no sequential port allocation prediction logic.
- **Impact**: Users expecting explicit port prediction for symmetric NAT traversal may find the feature absent. The existing advanced NAT traversal (`transport/advanced_nat.go`) provides TCP relay as fallback, which works but with higher latency.
- **Closing the Gap**: 
  1. **Option A (Documentation)**: Remove "port prediction" from README NAT traversal list if not intended.
  2. **Option B (Implementation)**: Implement sequential port prediction in `transport/hole_puncher.go` by:
     - Sending multiple probe packets to different ports on STUN server
     - Analyzing allocated port sequence to predict next allocation
     - Using predicted ports in hole punching attempts
  3. **Validation**: `grep -rn "port.*predict" transport/`

---

## Group Chat Callback Documentation

- **Stated Goal**: README documents comprehensive callback APIs for friend messages, file transfers, and connection status. Group chat functionality is claimed as "✅ Fully Implemented" in the roadmap.
- **Current State**: Conference APIs exist (`ConferenceNew`, `ConferenceInvite`, `ConferenceSendMessage`) in `toxcore_conference.go`, but:
  - No `OnConferenceMessage` callback documented in README
  - No `OnConferenceInvite` callback example
  - No group chat code samples in Basic Usage section
- **Impact**: Developers cannot easily discover how to receive group messages or invitations without reading source code. The API exists but is undocumented.
- **Closing the Gap**:
  1. Add "Group Chat" section to README after "Friend Management API"
  2. Document callback registration: `tox.OnConferenceMessage(func(groupID uint32, peerID uint32, message string) {...})`
  3. Add example showing group creation, invitation, and message handling
  4. **Validation**: `grep -n "OnConference" README.md` should return matches after fix

---

## Lokinet Listen Support

- **Stated Goal**: README table (line 269) shows Lokinet .loki with "❌ Listen" and notes it's "low priority and blocked by immature Lokinet SDK".
- **Current State**: `transport/lokinet_transport_impl.go` implements Dial-only via SOCKS5 proxy. Listen capability is not implemented.
- **Impact**: Users cannot host Tox nodes accessible via Lokinet addresses. This limits Lokinet to client-only use cases.
- **Closing the Gap**:
  1. **Short-term**: Already correctly documented as unsupported; no immediate action needed.
  2. **Long-term**: Monitor Lokinet SDK development for stable Go bindings or programmatic SNApp hosting API.
  3. **Validation**: Current behavior matches documentation; gap is acknowledged and tracked.

---

## Nym Listen Support

- **Stated Goal**: README table (line 270) shows Nym .nym with "❌ Listen" and notes it requires local Nym native client.
- **Current State**: `transport/nym_transport_impl.go` implements Dial-only via SOCKS5 proxy to local Nym client.
- **Impact**: Users cannot host Tox nodes accessible via Nym mixnet addresses.
- **Closing the Gap**:
  1. **Short-term**: Already correctly documented as unsupported.
  2. **Long-term**: Nym's Rust SDK does not have stable Go bindings; requires FFI wrapper or native Go implementation.
  3. **Validation**: Current behavior matches documentation; gap is acknowledged.

---

## VP8 Inter-Frame Encoding

- **Stated Goal**: README line 1036 documents VP8 as "key frames only" with 5-10x bandwidth overhead. This is a known limitation.
- **Current State**: `av/video/processor.go` uses `opd-ai/vp8` which produces I-frames only. `av/video/VIDEO_CODEC.md` lists "Phase 3.1: Inter-frame Prediction (P-frames)" as future work.
- **Impact**: Video calls use significantly more bandwidth than possible with full VP8. Users in bandwidth-constrained environments may experience quality issues.
- **Closing the Gap**:
  1. **Documentation**: Already correctly documented with mitigation advice (reduce frame rate/resolution).
  2. **Implementation**: Requires P-frame support in `opd-ai/vp8` library (upstream dependency).
  3. **Alternative**: Consider integrating CGo-based VP8 encoder as optional high-performance path.
  4. **Validation**: This is an acknowledged limitation, not a bug.

---

## Test Coverage Claim

- **Stated Goal**: README line 1499 claims ">90% coverage" in developer features section.
- **Current State**: go-stats-generator reports 93% documentation coverage, but actual line coverage varies by package. The claim may be conflating documentation coverage with test coverage.
- **Impact**: Developers may expect higher test coverage than actually exists in some packages.
- **Closing the Gap**:
  1. Run `go test -coverprofile=coverage.txt ./...` and verify actual line coverage
  2. Clarify README to distinguish documentation coverage from test line coverage
  3. If line coverage is below 90%, either improve tests or adjust claim
  4. **Validation**: `go tool cover -func=coverage.txt | grep total`

---

## Dead Code (242 Unreferenced Functions)

- **Stated Goal**: Clean, maintainable codebase with no unnecessary code.
- **Current State**: Analysis via `deadcode -test ./...` identified unreferenced functions. Majority are:
  - **C API bindings** (capi/*.go): Intentionally exported for C FFI consumers
  - **Public API constructors**: Functions like `NewClientWithKeyRotation`, `NewSKademliaRoutingTableWithCacheSize` are intentionally exported for external use
  - **Platform-specific fallbacks**: Functions like `getWindowsFilesystemStats`, `getDefaultFilesystemStats` are compile-time selected
- **Actions Taken** (2026-04-04):
  1. Removed `generateMessageID()` from toxcore.go (duplicate of async/forward_secrecy.go version)
  2. Removed `retrieveMessagesFromSingleNode()` from async/client.go (superseded by `WithTimeout` variant)
  3. Removed deprecated `shouldStopMaintenance()` from async/manager.go
  4. Removed unused helpers from group/chat.go: `checkLocalDHTStorage`, `waitForNetworkResponse`, `collectBroadcastResults`, `logBroadcastResults`
  5. Cleaned up unused `crypto/rand` import
- **Impact**: Reduced dead internal code while preserving intentionally exported public APIs
- **Validation**: `go vet ./...` and `go test -race ./...` pass

---

## Pre-Key Refresh Timing

- **Stated Goal**: Forward secrecy via one-time pre-key consumption with automatic refresh.
- **Current State**: `async/forward_secrecy.go:58-69` enforces a 5-key minimum (`MinPreKeys`), with refresh at 20 keys and low watermark at 10 keys.
- **Impact**: Rapid message senders could exhaust pre-keys before async refresh completes, causing temporary send failures.
- **Closing the Gap**:
  1. Consider increasing `MinPreKeys` from 5 to 10 for larger safety margin
  2. Add rate limiting documentation for high-frequency async messaging
  3. Consider synchronous refresh when below critical threshold
  4. **Validation**: `grep -n "MinPreKeys" async/forward_secrecy.go`

---

## BUG Annotations Not Tracked

- **Stated Goal**: All known issues should be tracked and visible to contributors.
- **Current State**: 4 BUG annotations exist in code (`crypto/constants.go:17,23,115`, `toxav.go:774`) but are not linked to GitHub issues.
- **Impact**: Contributors may not know these are known issues; duplicate bug reports possible.
- **Closing the Gap**:
  1. Review each BUG annotation
  2. Create GitHub issues for legitimate bugs
  3. Convert informational BUGs to NOTE annotations
  4. **Validation**: `grep -rn "// BUG" --include="*.go" . | wc -l` should decrease

---

## Async Message Handler Example Missing

- **Stated Goal**: Comprehensive documentation for async messaging system.
- **Current State**: `OnAsyncMessage` callback exists in `toxcore_callbacks.go:112` but is not shown in README examples.
- **Impact**: Developers may not discover async message reception capability without reading source.
- **Closing the Gap**:
  1. Add `OnAsyncMessage` to README callback examples section
  2. Show integration with forward-secure message decryption
  3. **Validation**: `grep -n "OnAsyncMessage" README.md` should return matches after fix

---

## Summary Table

| Gap | Severity | Effort | Status |
|-----|----------|--------|--------|
| Port prediction NAT | Medium | Medium | ✅ Already removed from README (no longer claims this feature) |
| Group chat callbacks | Medium | Low | ✅ Already documented (README line 730-772) |
| Lokinet Listen | Low | High | Blocked by upstream |
| Nym Listen | Low | High | Blocked by upstream |
| VP8 inter-frame | Low | High | Blocked by upstream |
| Test coverage claim | Low | Medium | ✅ Fixed - clarified to 63% statement/93% doc coverage |
| Dead code | Low | Medium | ✅ Reviewed - removed 7 truly dead internal functions; remaining are intentional public APIs or platform-specific |
| Pre-key timing | Low | Low | ✅ Reviewed - current values (min=5, low=10) are documented with trade-off rationale |
| BUG annotations | Low | Low | ✅ Already resolved (BUG annotations no longer present) |
| Async message example | Low | Low | ✅ Fixed (README line 1304-1333) |

---

*Generated from functional audit comparing stated goals against implementation.*
