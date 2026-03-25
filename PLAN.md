# Implementation Plan: Complete Remaining Gaps & Maintainability

## Project Context
- **What it does**: toxcore-go is a pure Go implementation of the Tox peer-to-peer encrypted messaging protocol, providing secure communications with multi-network support (IPv4/IPv6, Tor, I2P, Lokinet, Nym), forward secrecy, and offline messaging.
- **Current goal**: Complete remaining functional gaps (message delivery receipts, DeleteFriend cleanup, C API completion) and improve long-term maintainability through documentation clarity and code organization.
- **Estimated Scope**: Medium (10 items requiring attention across functional gaps, documentation, and refactoring)

## Goal-Achievement Status

| Stated Goal | Current Status | This Plan Addresses |
|-------------|---------------|---------------------|
| Pure Go implementation | ✅ Achieved | No |
| Comprehensive Tox protocol | ✅ Achieved | No |
| Multi-network IPv4/IPv6 | ✅ Achieved | No |
| Multi-network Tor | ✅ Achieved | No |
| Multi-network I2P | ✅ Achieved | No |
| Multi-network Lokinet | ⚠️ Partial (Dial only) | Yes (documentation) |
| Multi-network Nym | ⚠️ Partial (Dial only) | Yes (documentation) |
| Forward secrecy (Noise-IK) | ✅ Achieved | No |
| Epoch-based pseudonym rotation | ✅ Achieved | Yes (clarify documentation) |
| Audio calling with Opus | ✅ Achieved | No |
| Video calling with VP8 | ✅ Achieved (I-frames only) | Yes (document limitation) |
| File transfers | ✅ Achieved | No |
| Group chat | ✅ Achieved | No |
| C API bindings | ⚠️ Partial (`tox_conference_delete` stub) | Yes |
| Message delivery confirmation | ❌ Not implemented | Yes |
| Friend deletion cleanup | ❌ Incomplete | Yes |
| DHT scalability | ⚠️ Limited (2,048 nodes) | No (long-term) |
| Main facade refactoring | ⚠️ `toxcore.go` at 2,859 lines | Yes |

## Metrics Summary

- **Complexity hotspots**: 1 function above threshold (cyclomatic > 9) — `receiveLoop` in `dht/mdns_discovery.go` (cyclomatic=11)
- **Duplication ratio**: 0.77% (624 lines in 38 clone pairs, largest clone 19 lines) — excellent
- **Documentation coverage**: 98%+ across all packages (only 1 undocumented export: `Error` in `dht/iterative_lookup.go`)
- **Largest files by function lines**:
  - `toxcore.go`: 210 functions, 2,859 lines (burden score 7.57)
  - `capi/toxcore_c.go`: 84 functions, 1,404 lines
  - `av/manager.go`: 74 functions, 1,274 lines
  - `group/chat.go`: 77 functions, 899 lines
  - `async/client.go`: 73 functions, 875 lines

## Implementation Steps

### Step 1: Implement C API `tox_conference_delete`

- **Deliverable**: Complete the stub function in `capi/toxcore_c.go:952-980` to call the underlying Go implementation for leaving/deleting group chats.
- **Dependencies**: None
- **Goal Impact**: Completes C API bindings (stated goal), prevents resource leaks for C API consumers.
- **Acceptance**: Function body implemented, calls `group.Chat.Leave()`, returns proper error codes. Build succeeds: `go build -buildmode=c-shared -o libtoxcore.so ./capi`
- **Validation**: 
  ```bash
  go build ./capi && nm $(go env GOPATH)/pkg/*/github.com/opd-ai/toxcore/capi.a 2>/dev/null | grep -q tox_conference_delete && echo "PASS"
  ```

### Step 2: Implement Friend Deletion Resource Cleanup

- **Deliverable**: Extend `DeleteFriend()` in `toxcore.go:3246-3288` to:
  1. Cancel active file transfers via `fileManager.CancelTransfersForFriend(friendID)`
  2. Clear pending async messages via `asyncManager.ClearMessagesForRecipient(friendPK)`
  3. End active ToxAV calls via `toxav.EndCallIfActive(friendID)`
- **Dependencies**: None
- **Goal Impact**: Addresses incomplete friend deletion (GAPS.md Priority 7), prevents orphaned resources.
- **Acceptance**: After `DeleteFriend()`, no file transfers, async messages, or call sessions remain for that friend.
- **Validation**: 
  ```bash
  go test -race -run TestDeleteFriend -v ./... 2>&1 | grep -E "(PASS|ok)"
  ```

### Step 3: Add Helper Methods for Cleanup

- **Deliverable**: Add the following methods if not present:
  - `file.Manager.CancelTransfersForFriend(friendID uint32)`
  - `async.Manager.ClearMessagesForRecipient(recipientPK [32]byte)`
  - `av.Manager.EndCallIfActive(friendID uint32)` (or integrate with existing `EndCall`)
- **Dependencies**: Step 2 depends on these methods existing
- **Goal Impact**: Enables clean friend deletion without orphaned resources.
- **Acceptance**: Methods exist and are callable from `toxcore.Tox`.
- **Validation**:
  ```bash
  go build ./... && grep -rn "CancelTransfersForFriend\|ClearMessagesForRecipient\|EndCallIfActive" file/ async/ av/
  ```

### Step 4: Clarify Pre-Key vs Epoch Documentation

- **Deliverable**: Update `docs/ASYNC.md` (or create `docs/FORWARD_SECRECY.md`) to clearly distinguish:
  1. **Pre-keys** (`async/forward_secrecy.go:195-211`): Provide cryptographic forward secrecy through one-time key consumption
  2. **Epochs** (`async/epoch.go`, `async/obfs.go:62-77`): Provide metadata privacy through 6-hour pseudonym rotation for unlinkability
- **Dependencies**: None
- **Goal Impact**: Resolves documentation confusion (ROADMAP.md Priority 8), helps users understand security model.
- **Acceptance**: Documentation clearly states: "Forward secrecy via one-time pre-key consumption; epoch-based pseudonym rotation provides metadata privacy (unlinkability), not cryptographic forward secrecy."
- **Validation**: Manual review of `docs/ASYNC.md` or `docs/FORWARD_SECRECY.md`

### Step 5: Document VP8 I-Frame Only Limitation

- **Deliverable**: Add a note in `docs/TOXAV_BENCHMARKING.md` or README ToxAV section explaining:
  1. Current VP8 implementation produces only key frames (I-frames)
  2. This results in ~5-10x higher bandwidth than inter-frame encoding
  3. Future enhancement: P-frame support when opd-ai/vp8 library adds it
- **Dependencies**: None
- **Goal Impact**: Sets accurate expectations for video calling bandwidth requirements.
- **Acceptance**: Documentation includes bandwidth implications and future roadmap note.
- **Validation**: `grep -i "I-frame\|key.frame\|bandwidth" docs/TOXAV_BENCHMARKING.md README.md`

### Step 6: Design Message Delivery Receipts

- **Deliverable**: Design document in `docs/MESSAGE_RECEIPTS.md` specifying:
  1. Delivery receipt packet format (compatible with Tox protocol spec)
  2. `OnMessageDelivered(friendID, messageID)` callback interface
  3. Pending message tracking data structure
  4. Retry logic with exponential backoff
- **Dependencies**: None
- **Goal Impact**: Preparatory step for message delivery confirmation (GAPS.md), enables application-level delivery tracking.
- **Acceptance**: Design document exists with packet format, callback signature, and state management approach.
- **Validation**: `test -f docs/MESSAGE_RECEIPTS.md && echo "PASS"`

### Step 7: Implement Message Delivery Receipts

- **Deliverable**: Implement delivery receipts in `messaging/` package:
  1. Add `MessageDeliveryCallback` type and registration method
  2. Add pending message tracking in `MessageManager`
  3. Send delivery receipt packet when message received
  4. Fire callback when receipt received
  5. Implement configurable retry with exponential backoff
- **Dependencies**: Step 6 (design document)
- **Goal Impact**: Completes message delivery confirmation (GAPS.md Priority 10), enables reliable messaging.
- **Acceptance**: Test demonstrates callback fires when recipient acknowledges message.
- **Validation**:
  ```bash
  go test -race -run TestMessageDeliveryReceipt -v ./messaging/... 2>&1 | grep -E "(PASS|ok)"
  ```

### Step 8: Refactor `toxcore.go` — Extract Friend Methods

- **Deliverable**: Create `toxcore_friends.go` and move friend-related methods:
  - `AddFriend`, `AddFriendByPublicKey`, `DeleteFriend`
  - `GetFriendByPublicKey`, `GetFriendList`, `GetFriendConnectionStatus`
  - `SetFriendTypingStatus`, `GetFriendTypingStatus`
  - All `OnFriend*` callback registration methods
- **Dependencies**: None (can be done independently)
- **Goal Impact**: Improves maintainability (ROADMAP.md Priority 11), reduces `toxcore.go` from 2,859 to ~2,200 lines.
- **Acceptance**: All friend methods in `toxcore_friends.go`, all tests pass, no API changes.
- **Validation**:
  ```bash
  go test -race ./... && wc -l toxcore.go | awk '{if ($1 < 2500) print "PASS"; else print "FAIL: " $1 " lines"}'
  ```

### Step 9: Refactor `toxcore.go` — Extract Messaging Methods

- **Deliverable**: Create `toxcore_messaging.go` and move messaging-related methods:
  - `SendFriendMessage`, `SendFriendAction`
  - `SetMessageCallback`, `OnFriendMessage`
  - Message queue processing internals
- **Dependencies**: Step 8 (sequential refactoring reduces merge conflicts)
- **Goal Impact**: Further improves maintainability, brings `toxcore.go` below 1,500 lines.
- **Acceptance**: All messaging methods in `toxcore_messaging.go`, all tests pass, no API changes.
- **Validation**:
  ```bash
  go test -race ./... && wc -l toxcore.go | awk '{if ($1 < 2000) print "PASS"; else print "FAIL: " $1 " lines"}'
  ```

### Step 10: Document Storage Node Participation

- **Deliverable**: Add documentation in README or `docs/ASYNC.md` explaining:
  1. Async manager automatically initializes storage node participation
  2. Storage uses 1% of available disk (1MB-1GB bounds)
  3. Future: Add `StorageNodeEnabled` option to opt-out
- **Dependencies**: None
- **Goal Impact**: Addresses storage node documentation gap (ROADMAP.md Priority 9), sets user expectations.
- **Acceptance**: Documentation clearly states automatic participation and disk usage.
- **Validation**: `grep -i "storage.node\|1%.disk\|1MB.*1GB" README.md docs/ASYNC.md`

## Scope Assessment Calibration

| Metric | Current Value | Threshold | Assessment |
|--------|---------------|-----------|------------|
| Functions above complexity 9.0 | 1 | <5 = Small | ✅ Small |
| Duplication ratio | 0.77% | <3% = Small | ✅ Small |
| Documentation coverage gap | ~2% | <10% = Small | ✅ Small |
| Undocumented exports | 1 | <5 = Small | ✅ Small |
| Files >1000 lines | 5 | <5 = Small, 5-15 = Medium | ⚠️ Medium |

**Overall Scope: Medium** — Code quality metrics are excellent, but 5 files exceed 1000 lines requiring refactoring, and 4 functional gaps need implementation work.

## Dependency Graph

```
Step 1 (C API) ────────────────────────────────────────────────────────────────┐
Step 3 (Helper Methods) ──► Step 2 (DeleteFriend Cleanup) ─────────────────────┤
Step 4 (Documentation: Pre-Key/Epoch) ─────────────────────────────────────────┤
Step 5 (Documentation: VP8 Limitation) ────────────────────────────────────────┤
Step 6 (Design: Message Receipts) ──► Step 7 (Implement Message Receipts) ─────┤
Step 8 (Refactor: Friends) ──► Step 9 (Refactor: Messaging) ───────────────────┤
Step 10 (Documentation: Storage Node) ─────────────────────────────────────────┘
                                                                               │
                                                                               ▼
                                                                      All Goals Met
```

## Priority Order

1. **Quick wins (P1)**: Steps 1, 3, 2 — Complete C API, add helper methods, implement DeleteFriend cleanup
2. **Documentation clarity (P2)**: Steps 4, 5, 10 — Clarify pre-key/epoch, VP8 limitations, storage node participation
3. **Major feature (P3)**: Steps 6, 7 — Design and implement message delivery receipts
4. **Maintainability (P4)**: Steps 8, 9 — Refactor `toxcore.go` into smaller files

## Verification Commands

```bash
# Run full test suite with race detection
go test -tags nonet -race ./...

# Check file sizes after refactoring
wc -l toxcore.go toxcore_*.go

# Verify C API build
go build ./capi

# Check documentation coverage
go-stats-generator analyze . --skip-tests --format json --sections documentation | jq '.documentation.coverage'

# Check complexity hotspots
go-stats-generator analyze . --skip-tests --format json --sections functions | \
  jq '[.functions[] | select(.complexity.cyclomatic > 9)] | length'
```

## Appendix: Metrics Source

- **Analysis performed**: 2026-03-25
- **Tool**: `go-stats-generator v1.0.0`
- **Files analyzed**: 221 (excluding tests)
- **Configuration**: `--skip-tests --sections functions,duplication,documentation,packages,patterns`

### Key Metrics Summary

| Category | Count |
|----------|-------|
| Total LOC | 39,688 |
| Functions | 1,049 |
| Methods | 2,695 |
| Structs | 387 |
| Interfaces | 37 |
| Packages | 24 |
| Files | 221 |
| High-complexity functions (>9) | 1 |
| Duplication ratio | 0.77% |
| Clone pairs | 38 |
| Undocumented exports | 1 |

### Research Findings

1. **flynn/noise vulnerability**: The flynn/noise v1.1.0 dependency has a known nonce-handling vulnerability (GHSA-g9mp-8g3h-3c5c) that could weaken cryptographic security after 2^64 encryptions. The project mitigates this with a 2^32 rekey threshold in `transport/noise_transport.go:50`. No patched version is available upstream.

2. **Competitive landscape**: toxcore-go differentiates from c-toxcore by offering pure Go builds (no CGo for core functionality), native Go concurrency patterns, and extended privacy network support (Tor, I2P, Lokinet, Nym). The trade-off is newer maturity and potentially less optimized AV performance.

3. **Community priorities**: Based on GitHub issues and ROADMAP.md, the community prioritizes protocol compatibility with c-toxcore, privacy network functionality, and reliable offline messaging.
