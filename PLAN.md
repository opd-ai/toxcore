# Implementation Plan: Network Topology & Protocol Resilience

## Project Context
- **What it does**: A pure Go implementation of the Tox peer-to-peer encrypted messaging protocol with Noise-IK security, forward secrecy, and multi-network support.
- **Current goal**: Complete network topology resilience and group chat scalability (ROADMAP Priority 11-12 items)
- **Estimated Scope**: Medium (11 items requiring implementation across 5-6 packages)

## Goal-Achievement Status

| Stated Goal | Current Status | This Plan Addresses |
|-------------|----------------|---------------------|
| Pure Go, no CGo | ✅ Achieved | No |
| Core Tox protocol | ✅ Achieved | No |
| Multi-network (IPv4/IPv6) | ✅ Achieved | No |
| Multi-network (Tor/I2P) | ⚠️ Partial (dial only) | No — requires external daemon |
| Clean Go API | ✅ Achieved | No |
| C bindings | ⚠️ Partial (116 naming violations) | No — intentional for C compat |
| Noise-IK protocol | ✅ Achieved | No |
| Forward secrecy | ✅ Achieved | No |
| Identity obfuscation | ✅ Achieved | No |
| Audio/Video (ToxAV) | ⚠️ Partial (Opus/VP8 simplified) | Yes — Step 4-5 |
| Async messaging | ✅ Achieved | No |
| Group chat | ✅ Achieved (basic) | Yes — Step 6-7 |
| Documentation (>80%) | ✅ Achieved (92.8%) | No |
| DHT scalability | ⚠️ Partial | Yes — Step 1-3 |
| Network resilience | ❌ Not achieved | Yes — Step 1-3 |
| Pre-key DHT storage | ❌ Not achieved | Yes — Step 8 |

## Metrics Summary

- **Complexity hotspots on goal-critical paths**: 2 functions above threshold 9.0 in core packages (transport/worker_pool.go, transport/socks5_udp.go)
- **Duplication ratio**: 0.75% (excellent; below 3% threshold)
- **Doc coverage**: 92.8% overall, 98.5% function coverage (exceeds 80% target)
- **Package coupling**: `toxcore` (6.0), `async` (4.0), `transport` (3.5), `dht` (2.5)
- **Circular dependencies**: 0 (clean architecture)
- **Files processed**: 215 non-test files, 38,292 lines of code, 1,033 functions

## Key Findings from Analysis

### What the ROADMAP Tracks as Incomplete

1. **Priority 11 (Network Topology & Resilience)** — All items unchecked:
   - Gossip-based bootstrap protocol
   - mDNS for local discovery
   - Network partition detection
   - DHT replication for group announcements
   - Adaptive routing table sizing

2. **Priority 12 (Scalability Patterns)** — All items unchecked:
   - Sender-key protocol for O(1) group encryption
   - Push notification proxying
   - Pre-key bundles in DHT
   - Message ordering (Lamport/vector clocks)

3. **GAPS.md High-Priority Items**:
   - Opus encoding not implemented (audio passthrough)
   - VP8 encoding simplified
   - DeleteFriend resource cleanup incomplete
   - Call online status check missing

### Dependency Security

- **flynn/noise v1.1.0**: Protected from GO-2022-0425 (v1.0.0+ is patched). Nonce exhaustion mitigation via 2^32 rekey threshold already in place (`transport/noise_transport.go:50`).

---

## Implementation Steps

### Step 1: Gossip-Based Bootstrap Protocol ✅ COMPLETED

- **Deliverable**: New file `dht/gossip_bootstrap.go` implementing peer exchange protocol; modification to `dht/bootstrap.go` to use gossip as fallback when hard-coded nodes fail.
- **Dependencies**: None
- **Goal Impact**: Priority 11 — Eliminates single point of failure on bootstrap nodes; enables organic network growth.
- **Acceptance**: Bootstrap succeeds with empty initial node list when ≥1 existing peer is reachable; `maxAttempts` failure mode triggers gossip fallback.
- **Validation**: 
  ```bash
  go test -race -run TestGossipBootstrap ./dht/...
  go-stats-generator analyze ./dht --skip-tests 2>/dev/null | grep -A3 "gossip"
  ```
- **Estimated Effort**: Medium (2-3 days)
- **Files**: `dht/gossip_bootstrap.go` (new), `dht/bootstrap.go` (modify)
- **Status**: Implemented. GossipBootstrap struct with peer exchange, routing table integration, and fallback mechanism.

### Step 2: mDNS Local Discovery ✅ COMPLETED

- **Deliverable**: New file `dht/mdns_discovery.go` implementing RFC 6762 multicast DNS; `dht/local_discovery.go` fallback to mDNS when broadcast fails.
- **Dependencies**: None
- **Goal Impact**: Priority 11 — Replaces IPv4 broadcast (fails in cloud/container) with mDNS (works with Docker bridge networks, Kubernetes pods).
- **Acceptance**: Local peer discovery works in Docker Compose environment without `--network=host`.
- **Validation**: 
  ```bash
  go test -race -run TestMDNSDiscovery ./dht/...
  ```
- **Estimated Effort**: Medium (2 days)
- **Files**: `dht/mdns_discovery.go` (new), `dht/local_discovery.go` (modify)
- **Status**: Implemented 2026-03-23. MDNSDiscovery with peer exchange protocol, automatic fallback from broadcast failures, and EnableMDNS() for manual container-aware deployment.

### Step 3: Network Partition Detection & Recovery ✅ COMPLETED

- **Deliverable**: New file `dht/partition_detector.go` tracking routing table health; modification to `dht/maintenance.go` to trigger re-bootstrap on partition detection.
- **Dependencies**: Step 1 (gossip bootstrap provides recovery mechanism)
- **Goal Impact**: Priority 11 — Automatic recovery from network partitions; improves uptime SLA.
- **Acceptance**: Simulated partition (all k-bucket entries unreachable) triggers re-bootstrap within 60 seconds; recovery logged.
- **Validation**: 
  ```bash
  go test -race -run TestPartitionRecovery ./dht/...
  go-stats-generator analyze ./dht --skip-tests 2>/dev/null | grep "partition_detector"
  ```
- **Estimated Effort**: Medium (2 days)
- **Files**: `dht/partition_detector.go` (new), `dht/partition_detector_test.go` (new)
- **Status**: Implemented 2026-03-23. PartitionDetector with state monitoring, automatic recovery, and gossip/bootstrap fallback.

### Step 4: Opus Audio Encoding ✅ COMPLETE

- **Deliverable**: Replace `pion/opus` with `opd-ai/magnum` for both encoding and decoding; implement `MagnumOpusEncoder` with proper Opus compression.
- **Dependencies**: None
- **Goal Impact**: GAPS.md #1 — Reduces audio bandwidth by 5-10×; enables interoperability with c-toxcore.
- **Acceptance**: `OpusCodec.Encode()` produces valid Opus packets; round-trip encode→decode preserves audio quality.
- **Validation**: 
  ```bash
  go test -race -run TestOpus ./av/audio/...
  go test -bench=BenchmarkOpus ./av/audio/...
  ```
- **Estimated Effort**: Medium (2 days)
- **Files**: `av/audio/codec.go` (modified), `av/audio/processor.go` (modified), `av/audio/codec_test.go` (modified), `av/audio/processor_test.go` (modified)
- **Status**: Implemented 2026-03-24. Replaced `pion/opus` with `opd-ai/magnum`. Full Opus encoding via CELT (48kHz) and SILK (8/16kHz) codec paths. Round-trip tests pass.

### Step 5: VP8 Video Encoding Improvement

- **Deliverable**: Evaluate pure Go VP8 encoder or document CGo-optional path; improve `av/video/codec.go` quality presets.
- **Dependencies**: None
- **Goal Impact**: GAPS.md #2 — Improves video compression efficiency; reduces bandwidth.
- **Acceptance**: Video encoding produces valid VP8 keyframes; quality presets (low/medium/high) produce measurably different bitrates.
- **Validation**: 
  ```bash
  go test -race -run TestVP8 ./av/video/...
  go test -bench=BenchmarkVP8 ./av/video/...
  ```
- **Estimated Effort**: High (3-5 days) — May require research into pure Go encoders or CGo wrapper.
- **Files**: `av/video/codec.go` (modify), `av/video/codec_test.go` (add tests)

### Step 6: Sender-Key Protocol for Group Chat

- **Deliverable**: New file `group/sender_key.go` implementing Signal's sender-key distribution; modification to `group/chat.go:BroadcastMessage` to use O(1) encryption.
- **Dependencies**: None
- **Goal Impact**: Priority 12 — Reduces group message encryption from O(n) to O(1); enables 100K+ member groups.
- **Acceptance**: 100-member group message encrypted once with sender key, decrypted by all members; key rotation on member removal works correctly.
- **Validation**: 
  ```bash
  go test -race -run TestSenderKey ./group/...
  go test -bench=BenchmarkGroupBroadcast ./group/...
  ```
- **Estimated Effort**: High (3-4 days)
- **Files**: `group/sender_key.go` (new), `group/chat.go` (modify)

### Step 7: DHT Replication for Group Announcements

- **Deliverable**: Modification to `group/chat.go` to store announcements at k=5 nearest nodes; add `group/dht_replication.go` for announcement queries.
- **Dependencies**: Step 6 (sender-key improves group scalability first)
- **Goal Impact**: Priority 11 — Groups discoverable across process boundaries; announcement survives node churn.
- **Acceptance**: Group announcement stored at 5 nodes; announcement retrievable when 2 of 5 nodes offline.
- **Validation**: 
  ```bash
  go test -race -run TestGroupDHTReplication ./group/...
  ```
- **Estimated Effort**: Medium (2 days)
- **Files**: `group/dht_replication.go` (new), `group/chat.go` (modify)

### Step 8: Pre-Key Bundles in DHT

- **Deliverable**: Modification to `async/forward_secrecy.go` to publish pre-keys to DHT; add `async/prekey_dht.go` for DHT storage/retrieval.
- **Dependencies**: None
- **Goal Impact**: Priority 12 — Enables forward-secure messaging with offline recipients; removes requirement for both parties online simultaneously.
- **Acceptance**: Pre-key bundle published to k=3 DHT nodes; recipient retrieves pre-key without sender being online.
- **Validation**: 
  ```bash
  go test -race -run TestPreKeyDHT ./async/...
  ```
- **Estimated Effort**: Medium (2-3 days)
- **Files**: `async/prekey_dht.go` (new), `async/forward_secrecy.go` (modify)

### Step 9: Message Ordering with Lamport Timestamps ✅ COMPLETED

- **Deliverable**: Add `LamportClock` field to `async/message.go:AsyncMessage`; implement causal ordering in `async/manager.go` message retrieval.
- **Dependencies**: None
- **Goal Impact**: Priority 12 — Provides causal message ordering without central authority; prevents message reordering issues.
- **Acceptance**: Messages retrieved in causal order; concurrent messages from different senders properly interleaved.
- **Validation**: 
  ```bash
  go test -race -run TestLamportOrdering ./async/...
  ```
- **Estimated Effort**: Low (1-2 days)
- **Files**: `async/storage.go` (modify - added LamportClock and SenderClockHint fields), `async/lamport.go` (new), `async/manager.go` (modify - added messageOrdering and timestamp methods)
- **Status**: Implemented 2026-03-23. Added LamportClock implementation with full test coverage.

### Step 10: DeleteFriend Resource Cleanup ✅ COMPLETED (per AUDIT.md)

- **Deliverable**: Modify `toxcore.go:DeleteFriend()` to cancel file transfers, clear async messages, end active calls.
- **Dependencies**: None
- **Goal Impact**: GAPS.md #7 — Prevents resource leaks; improves memory stability.
- **Acceptance**: After `DeleteFriend()`: file transfers canceled, async queue cleared, no orphaned call state.
- **Validation**: 
  ```bash
  go test -race -run TestDeleteFriendCleanup ./...
  ```
- **Estimated Effort**: Low (0.5-1 day)
- **Files**: `toxcore.go` (modify)
- **Status**: Implemented (per AUDIT.md - CancelTransfersForFriend and ClearPendingMessagesForFriend added)

### Step 11: Call Online Status Verification ✅ COMPLETED (per AUDIT.md)

- **Deliverable**: Modify `av/manager.go:StartCall()` to check friend connection status before initiating call.
- **Dependencies**: Step 10 (cleanup consistency)
- **Goal Impact**: GAPS.md #6 — Better error reporting; prevents wasted resources on offline calls.
- **Acceptance**: `StartCall()` returns `ErrFriendOffline` when friend status is `ConnectionNone`.
- **Validation**: 
  ```bash
  go test -race -run TestCallOfflineFriend ./av/...
  ```
- **Estimated Effort**: Low (0.5 day)
- **Files**: `av/manager.go` (modify), `av/errors.go` (add error type)
- **Status**: Implemented (per AUDIT.md - validateFriendOnline check added to ToxAV.Call)

---

## Scope Assessment

| Metric | Value | Threshold | Assessment |
|--------|-------|-----------|------------|
| Functions above complexity 9.0 | 4 (all in examples/capi) | <5 Small, 5-15 Medium | ✅ Small |
| Duplication ratio | 0.75% | <3% Small | ✅ Small |
| Doc coverage gap | 0% (92.8% achieved) | <10% Small | ✅ Achieved |
| Items requiring implementation | 11 steps | 5-15 Medium | ⚠️ Medium |

**Overall Scope: Medium** — 11 implementation steps, but most are isolated changes to specific packages with clear acceptance criteria.

---

## Validation Commands

```bash
# Full metrics analysis after all changes
go-stats-generator analyze . --skip-tests

# Build verification
go build ./...

# Full test suite with race detection
go test -tags nonet -race ./...

# Specific package tests
go test -race ./dht/... ./group/... ./async/... ./av/...

# Check complexity hasn't increased
go-stats-generator analyze . --skip-tests 2>/dev/null | grep -A15 "Top Complex Functions"

# Verify no new circular dependencies
go-stats-generator analyze . --skip-tests 2>/dev/null | grep -A3 "CIRCULAR"
```

---

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| VP8 encoding may require CGo | Document CGo-optional build; keep pure Go fallback |
| Sender-key protocol complexity | Reference Signal spec; extensive testing with key rotation |
| DHT replication may increase bandwidth | Make replication factor configurable (default k=3) |
| mDNS library compatibility | Test on Linux, macOS, Windows; fallback to broadcast |

---

## Open Questions (Deferred to Future Planning)

From ROADMAP.md §7, these require design decisions before implementation:

1. Super-node incentive structure for hierarchical DHT
2. Group chat consistency model (vector clocks vs CRDTs vs leader-based)
3. Light client mode for mobile devices
4. Handshake DoS mitigation (cookie-based, like DTLS)

These are explicitly out of scope for this plan but should be addressed in a subsequent planning cycle.
