# Implementation Plan: qTox Integration Readiness

## Project Context
- **What it does**: Pure Go implementation of the Tox P2P encrypted messaging protocol with DHT peer discovery, friend management, file transfers, audio/video calling, async offline messaging, and multi-network transport support (IPv4/IPv6, Tor, I2P, Nym, Lokinet).
- **Current goal**: qTox CI/CD integration (GitHub issue #43) — providing a production-ready Go alternative that can be deployed as an experimental release artifact.
- **Estimated Scope**: Medium

## Goal-Achievement Status

| Stated Goal | Current Status | This Plan Addresses |
|-------------|----------------|---------------------|
| Pure Go, no CGo (core) | ✅ Achieved | No |
| Core Tox protocol | ✅ Achieved | No |
| Multi-network (IPv4/IPv6) | ✅ Achieved | No |
| Multi-network (Tor) | ✅ Achieved (TCP) | No |
| Multi-network (I2P) | ✅ Achieved | No |
| Clean Go API | ✅ Achieved | No |
| C bindings coverage | ⚠️ Partial (~33 of ~80 functions) | Yes |
| ToxAV real codecs | ⚠️ Partial (raw PCM/YUV, no Opus/VP8 encode) | Yes |
| Function documentation | ✅ Achieved (96.1%) | No |
| Pre-key lifecycle cleanup | ⚠️ Incomplete (no auto-cleanup) | Yes |
| Bootstrap reliability | ⚠️ Partial (5s timeout too aggressive) | Yes |
| File transfer accept API | ⚠️ Missing convenience methods | Yes |
| Concurrent event processing | ✅ Implemented (IterationPipelines) | No |
| DHT scalability | ✅ Implemented (S/Kademlia, iterative lookup, caching) | No |
| Transport scalability | ✅ Implemented (ReusePort, WorkerPool, LRU cache) | No |

## Metrics Summary
- **Complexity hotspots on goal-critical paths**: 73 functions above threshold (overall > 9)
  - capi package: 6 high-complexity functions (C binding interop)
  - transport package: 16 high-complexity functions (transport layer)
  - dht package: 10 high-complexity functions (DHT operations)
- **Very high complexity (>15)**: 1 function (`tox_conference_send_message` in capi)
- **Documentation coverage**: 96.1% (3383/3519 functions documented)
- **Exported functions without docs**: 1 (0.1%)
- **Total codebase**: 37,450 LOC, 973 functions, 24 packages, 211 files
- **Package coupling**: transport (702 functions), async (380 functions), dht (340 functions) are the largest modules

## Implementation Steps

### Step 1: ToxAV Opus Encoder Integration ⏸️ BLOCKED
- **Deliverable**: Replace `SimplePCMEncoder` in `av/audio/processor.go` with real Opus encoding using `github.com/pion/opus` encoder API. Audio data sent to peers will be Opus-compressed.
- **Dependencies**: None (decoder already integrated)
- **Goal Impact**: Advances qTox interoperability — qTox expects Opus-encoded audio; without this, audio calls will fail between toxcore-go and native Tox clients.
- **Acceptance**: Audio frames encoded with Opus codec; round-trip test decodes successfully with pion/opus decoder.
- **Validation**: `go test -v -run TestOpusEncoderRoundtrip ./av/audio/...`
- **Status**: BLOCKED - pion/opus is decode-only. Opus encoding requires CGo (e.g., xlab/opus-go), which would violate the project's "pure Go, no CGo" goal for core libraries.

### Step 2: Pre-Key Lifecycle Automation ✅ COMPLETED
- **Deliverable**: Add a cleanup goroutine to `ForwardSecurityManager` in `async/forward_secrecy.go` that calls `CleanupExpiredData()` every 24 hours (configurable). Add `CleanupInterval` config option and metrics for pre-key storage usage.
- **Dependencies**: None
- **Goal Impact**: Prevents unbounded disk growth from expired pre-keys; production reliability requirement for long-running nodes.
- **Acceptance**: After 24 hours of simulated operation, expired pre-keys are automatically purged without manual intervention.
- **Validation**: `go test -v -run TestPreKeyCleanupAutomation ./async/...`
- **Status**: Implemented in async/forward_secrecy.go with NewForwardSecurityManagerWithInterval(), automatic cleanup goroutine, and Close() method.

### Step 3: Bootstrap Reliability Improvements ✅ COMPLETED
- **Deliverable**: 
  1. Increase default `BootstrapTimeout` from 5s to 30s in `dht/bootstrap.go`
  2. Add automatic retry with exponential backoff (up to 3 retries, max 2 min backoff)
  3. Update examples to use multiple bootstrap nodes
- **Dependencies**: None
- **Goal Impact**: First-time users following README examples will successfully connect to the network; critical for qTox CI testing.
- **Acceptance**: Bootstrap succeeds within 3 retries against production Tox bootstrap nodes under typical network conditions.
- **Validation**: Manual integration test against `tox.initramfs.io`, `node.tox.biribiri.org`; `go test -v -run TestBootstrapRetry ./dht/...`
- **Status**: Implemented in toxcore.go - BootstrapTimeout increased to 30s, retry logic with exponential backoff (1s/2s/4s).

### Step 4: File Transfer Convenience API ✅ COMPLETED
- **Deliverable**: Add `FileAccept(friendID, fileNumber uint32) error` and `FileReject(friendID, fileNumber uint32) error` convenience methods to `toxcore.go`. Document the callback-based workflow in `doc.go`.
- **Dependencies**: None
- **Goal Impact**: Improves API ergonomics for qTox integration — developers can accept files with single method call instead of manual `FileControl` construction.
- **Acceptance**: New methods successfully wrap existing `FileControl` functionality; README examples updated.
- **Validation**: `go test -v -run TestFileTransferWorkflow ./file/...`
- **Status**: Implemented FileAccept() and FileReject() in toxcore.go with test coverage.

### Step 5: C API Coverage Expansion (Phase 1 - Core Functions) ✅ COMPLETED
- **Deliverable**: Implement missing core C API functions in `capi/toxcore_c.go`:
  - `tox_self_get_status`, `tox_self_set_status`
  - `tox_friend_get_status`, `tox_friend_get_connection_status`
  - `tox_iteration_interval`, `tox_iterate` (wrapper for main loop)
  - `tox_self_get_nospam`, `tox_self_set_nospam`
- **Dependencies**: Step 4 (FileAccept/FileReject for file-related C bindings)
- **Goal Impact**: Expands C API coverage from ~33 to ~45 functions (~56%); enables basic qTox functionality.
- **Acceptance**: Each new C function compiles with CGo and maps correctly to Go implementation.
- **Validation**: `CGO_ENABLED=1 go build ./capi/...`; compare function signatures against `c-toxcore/toxcore/tox.h`
- **Status**: Implemented all listed functions plus additional ones (tox_friend_get_name, tox_friend_get_public_key, tox_friend_get_last_online, tox_friend_exists, tox_self_get_friend_list, tox_file_get_file_id, tox_hash, tox_conference_get_type, tox_conference_peer_count, tox_conference_set_title). C API now has 55 exported functions (~69% coverage).

### Step 6: Refactor High-Complexity C Binding Functions ✅ COMPLETED
- **Deliverable**: Reduce complexity of `tox_conference_send_message` (complexity 15.3) in `capi/toxcore_c.go:827` by extracting validation logic into helper functions and simplifying error code mapping.
- **Dependencies**: None
- **Goal Impact**: Improves maintainability of C bindings; reduces bug risk in interop layer critical for qTox integration.
- **Acceptance**: `tox_conference_send_message` complexity reduced below 12.0; no behavior changes.
- **Validation**: `go-stats-generator analyze ./capi --skip-tests --format json | jq '.functions[] | select(.name == "tox_conference_send_message") | .complexity.overall'` returns value < 12.0
- **Status**: Refactored to extract validateConferenceToxInstance(), setConfError(), validateConferenceMessage(), convertConferenceMessageType() helper functions. Complexity reduced from 15.3 to 5.7.

### Step 7: ToxAV VP8 Encoder Integration
- **Deliverable**: Replace `SimpleVP8Encoder` in `av/video/codec.go` with real VP8 encoding. Options: (a) CGo wrapper to libvpx, or (b) pure Go VP8 encoder (lower quality but maintains CGo-free goal).
- **Dependencies**: Step 1 (establishes codec integration pattern)
- **Goal Impact**: Completes video codec stack for qTox interoperability — video calls will work with native Tox clients.
- **Acceptance**: Video frames encoded with VP8 codec; decoded frames match source within acceptable PSNR threshold (>30 dB).
- **Validation**: `go test -v -run TestVP8EncoderRoundtrip ./av/video/...`

### Step 8: C API Coverage Expansion (Phase 2 - Conference Functions)
- **Deliverable**: Implement remaining conference C API functions in `capi/toxcore_c.go`:
  - `tox_conference_get_type`, `tox_conference_get_title`, `tox_conference_set_title`
  - `tox_conference_peer_count`, `tox_conference_peer_get_name`
  - `tox_conference_peer_get_public_key`, `tox_conference_connected`
  - `tox_conference_offline_peer_count`, `tox_conference_offline_peer_get_name`
- **Dependencies**: Step 6 (conference function refactoring)
- **Goal Impact**: Expands C API coverage to ~60 functions (~75%); enables group chat functionality in qTox.
- **Acceptance**: Conference C bindings pass smoke tests with multiple peers.
- **Validation**: `CGO_ENABLED=1 go build ./capi/...`; integration test with mock conference

### Step 9: Documentation-Implementation Sync
- **Deliverable**: 
  1. Audit all README code examples against current API
  2. Fix group chat terminology (README says "GroupNew", code says "Create")
  3. Update `docs/CHANGELOG.md` with recent changes
  4. Extract all code blocks from README and verify they compile
- **Dependencies**: Steps 1-4 (API changes must be finalized first)
- **Goal Impact**: Ensures qTox developers using this library have accurate documentation.
- **Acceptance**: All README code examples compile without modification; CHANGELOG reflects current state.
- **Validation**: `go build` succeeds on extracted example code; manual review of CHANGELOG

### Step 10: qTox Integration Test Suite
- **Deliverable**: Create `testnet/qtox_compat_test.go` with integration tests validating:
  1. Friend request/accept workflow matches c-toxcore behavior
  2. Message delivery semantics match c-toxcore
  3. File transfer protocol compatibility
  4. Conference join/leave/message workflow
  5. ToxAV call establishment and media flow (if Steps 1+7 complete)
- **Dependencies**: Steps 1-9
- **Goal Impact**: Provides automated validation for qTox CI/CD pipeline; directly enables GitHub issue #43.
- **Acceptance**: Test suite passes against both toxcore-go and c-toxcore reference (via Docker or native build).
- **Validation**: `go test -v -tags qtox_compat ./testnet/...`

---

## Scope Calibration

| Metric | This Codebase | Threshold | Assessment |
|--------|---------------|-----------|------------|
| Functions above complexity 9.0 | 73 | <5 small, 5-15 medium, >15 large | Large (complexity debt) |
| Duplication ratio | Not measured | <3% small, 3-10% medium, >10% large | N/A |
| Doc coverage gap | 3.9% (target 100%) | <10% small | Small (already excellent) |
| Overall scope | 10 steps | - | Medium |

## Validation Commands

```bash
# Overall codebase metrics
go-stats-generator analyze . --skip-tests --format json | jq '.overview'

# Complexity hotspots in goal-critical packages
go-stats-generator analyze ./capi ./av --skip-tests --format json | jq '[.functions[] | select(.complexity.overall > 9)] | length'

# Documentation coverage
go-stats-generator analyze . --skip-tests --format json | jq '.functions | [.[] | select(.documentation.has_comment)] | length, length'

# Build verification
go build ./...
CGO_ENABLED=1 go build ./capi/...

# Test suite
go test -tags nonet -race ./...
```

## Risk Factors

1. **VP8 encoding without CGo**: Pure Go VP8 encoder may not exist at production quality; may require CGo dependency on libvpx, partially compromising "pure Go" goal.
2. **C API behavior parity**: Subtle differences in error handling or edge cases between Go and C implementations could cause qTox integration issues.
3. **Codec licensing**: VP8 is royalty-free, but ensure all codec dependencies have compatible licenses.

## Notes

- The ROADMAP.md shows most infrastructure work (DHT scalability, transport scaling, concurrent processing) is already complete.
- GAPS.md identifies ToxAV codecs and pre-key cleanup as the highest-priority gaps, which this plan addresses first.
- GitHub issue #43 (qTox CI/CD) from @iphydf provides external validation of readiness milestone.
- Current documentation coverage (96.1%) exceeds the 80% target; no additional doc work needed.
- The single very-high-complexity function (`tox_conference_send_message` at 15.3) is in the C bindings layer and should be refactored for maintainability.
