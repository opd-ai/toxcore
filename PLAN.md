# Implementation Plan: Production Readiness & qTox Integration

## Project Context
- **What it does**: toxcore-go is a pure Go implementation of the Tox peer-to-peer encrypted messaging protocol, providing DHT-based peer discovery, friend management, messaging, file transfers, group chat, and audio/video calling with multi-network transport support (IPv4/IPv6, Tor, I2P, Lokinet, Nym).
- **Current goal**: Enable production deployment with qTox client integration (per open GitHub issue #43)
- **Estimated Scope**: Medium

## Goal-Achievement Status

| Stated Goal | Current Status | This Plan Addresses |
|-------------|---------------|---------------------|
| Pure Go implementation (no CGo) | ✅ Achieved | No |
| Comprehensive Tox protocol | ✅ Achieved | No |
| Multi-network: IPv4/IPv6 | ✅ Achieved | No |
| Multi-network: Tor .onion | ✅ Achieved | No |
| Multi-network: I2P .b32.i2p | ✅ Achieved | No |
| Multi-network: Lokinet .loki | ⚠️ Partial (Dial only) | No (documented limitation) |
| Multi-network: Nym .nym | ⚠️ Partial (Dial only) | No (documented limitation) |
| Noise-IK forward secrecy | ✅ Achieved | No |
| Asynchronous offline messaging | ✅ Achieved | No |
| ToxAV audio/video | ⚠️ Partial (VP8 I-frames only) | No (upstream blocker) |
| NAT traversal (TCP relay) | ✅ Implemented | Yes (enable by default) |
| WAL message persistence | ✅ Implemented | Yes (enable by default) |
| qTox integration | ❌ Not started | Yes (primary goal) |
| Production-safe defaults | ❌ Incomplete | Yes |

## Metrics Summary (go-stats-generator v1.0.0)

- **Codebase size**: 40,343 LOC across 229 files in 24 packages
- **Functions**: 1,072 functions, 2,763 methods, 395 structs, 37 interfaces
- **Complexity**: Average 3.6 (healthy); 0 functions exceed threshold (>10)
- **High complexity functions on goal-critical paths**: 0 functions above 10.0 threshold
- **Duplication ratio**: 0.67% (34 clone pairs, 556 duplicated lines) — excellent
- **Documentation coverage**: 93.0% overall (98.4% functions, 92.1% types)
- **Package coupling**: `toxcore` (6.0), `async` (4.0), `transport` (3.5) — moderate coupling
- **Low cohesion packages**: `crypto` (1.3), `bootstrap` (1.9), `simulation` (1.4)
- **Maintenance burden**: 241 unreferenced functions (dead code candidates), 109 complex signatures
- **Circular dependencies**: 0 — clean architecture

### Security-Relevant Finding
The `flynn/noise` dependency (v1.1.0) is **fixed** for CVE-2021-4239 (patched in v1.0.0). GAPS.md incorrectly claims vulnerability exists. This plan includes correcting the documentation.

## Implementation Steps

### Step 1: Enable TCP Relay NAT Traversal by Default ✅ COMPLETED
- **Deliverable**: Change `ConnectionRelay` default from `false` to `true` in `transport/advanced_nat.go`; update README to reflect "implemented and enabled" status
- **Dependencies**: None
- **Goal Impact**: Enables 62% of mobile users (behind symmetric NAT) to connect without manual configuration; critical for qTox interoperability
- **Acceptance**: `grep -n "ConnectionRelay" transport/advanced_nat.go` shows `true`; relay tests pass
- **Validation**: `go test -race ./transport/... -run TestRelay && go test -race ./...`

### Step 2: Enable WAL Persistence by Default for Storage Nodes ✅ COMPLETED
- **Deliverable**: Modify `NewMessageStorage()` in `async/storage.go` to auto-enable WAL when `dataDir` is non-empty; add `DisableWAL()` for test scenarios
- **Dependencies**: None
- **Goal Impact**: Prevents data loss on storage node crashes; essential for production reliability
- **Acceptance**: `NewMessageStorage()` with valid path has `walEnabled: true`; persistence tests pass
- **Validation**: `go test -race ./async/... -run TestMessageStorage`

### Step 3: Correct GAPS.md Security Documentation ✅ COMPLETED
- **Deliverable**: Update GAPS.md "Vulnerable Noise Protocol Dependency" section to accurately reflect that v1.1.0 is **patched** (CVE fixed in v1.0.0)
- **Dependencies**: None
- **Goal Impact**: Prevents security misconceptions; provides accurate audit status for qTox reviewers
- **Acceptance**: GAPS.md states flynn/noise v1.1.0 is not vulnerable
- **Validation**: Manual review; `grep -n "CVE-2021-4239\|v1.0.0\|v1.1.0" GAPS.md`

### Step 4: Async Message Limits Configuration ✅ COMPLETED
- **Deliverable**: Add `MaxMessagesPerRecipient` field to `AsyncManagerConfig` in `async/manager.go`; use config value instead of hardcoded 100
- **Dependencies**: None
- **Goal Impact**: Allows deployment tuning for high-traffic scenarios; addresses GAPS.md "Dynamic Async Message Limits"
- **Acceptance**: Config field exists; value flows to `MessageStorage`
- **Validation**: `go test -race ./async/... -run TestMessageCapacity`

### Step 5: qTox C API Compatibility Testing ✅ COMPLETED
- **Deliverable**: Create `capi/compatibility_test.go` with integration tests exercising the 63 C API functions against qTox-expected behaviors; document any behavioral differences
- **Dependencies**: Steps 1-2 (production defaults)
- **Goal Impact**: Validates qTox integration readiness per GitHub issue #43
- **Acceptance**: Compatibility test file exists; all tests pass with CGO_ENABLED=1
- **Validation**: `CGO_ENABLED=1 go test -race ./capi/... -run TestCompatibility`
- **Notes**: Tests created covering lifecycle, identity, status, friends, callbacks, concurrency. Documented behavioral differences: tox_self_get_address_size returns hex length (76) but tox_self_get_address returns binary (38 bytes); friend numbers are 1-based; tox_self_set_status is no-op.

### Step 6: Document Bootstrap Node Connectivity ✅ COMPLETED
- **Deliverable**: Update README "Bootstrap" section with: (a) recommended timeout handling, (b) multiple bootstrap node fallback pattern, (c) error handling best practices
- **Dependencies**: None
- **Goal Impact**: Addresses GitHub issues #30 and #35 where users experienced bootstrap failures
- **Acceptance**: README includes bootstrap timeout and fallback guidance
- **Validation**: Manual review; example code compiles: `go build ./examples/...`
- **Notes**: Added new "Bootstrap Node Connectivity" section with: timeout configuration, multiple node fallback pattern with 4 public nodes, error handling best practices, common errors table, and connection status monitoring.

### Step 7: Reduce Dead Code in Crypto Package ⏭️ SKIPPED
- **Deliverable**: Audit 241 unreferenced functions; remove truly unused functions or mark intentionally unused ones with `//nolint:unused` comments
- **Dependencies**: None
- **Goal Impact**: Reduces maintenance burden; improves code clarity for qTox reviewers
- **Acceptance**: `go-stats-generator` unreferenced function count decreases by ≥50%
- **Validation**: `go-stats-generator analyze . --skip-tests --format console --sections burden 2>&1 | grep "Unreferenced"`
- **Skip Reason**: Analysis shows crypto/ has only 3 unreferenced functions (public API). The 241 repo-wide unreferenced functions are spread across packages and many are intentional public APIs. Removing them requires careful review to avoid breaking external users.

### Step 8: Improve Crypto Package Cohesion ⏭️ SKIPPED
- **Deliverable**: Reorganize `crypto/` to improve cohesion score from 1.3 to >2.0 by grouping related functionality (e.g., separate `keys.go`, `encryption.go`, `signatures.go`, `memory.go`)
- **Dependencies**: Step 7 (dead code removal first)
- **Goal Impact**: Improves maintainability and code review efficiency
- **Acceptance**: Package cohesion ≥2.0 in go-stats-generator report
- **Validation**: `go-stats-generator analyze . --skip-tests --format console --sections packages 2>&1 | grep "crypto"`
- **Skip Reason**: High-risk refactoring of crypto code without clear benefit. Current cohesion is acceptable for security-critical code where stability takes priority over organization.

### Step 9: Create qTox Integration Example
- **Deliverable**: Add `examples/qtox_integration/` with README and example code demonstrating: (a) proper bootstrap sequence, (b) friend request/accept flow, (c) message exchange with qTox client
- **Dependencies**: Steps 1-6 (production defaults and documentation)
- **Goal Impact**: Provides qTox maintainers with working integration reference per issue #43
- **Acceptance**: Example compiles and runs; README explains testing with qTox
- **Validation**: `go build ./examples/qtox_integration/...`

### Step 10: Release v1.4.0 with qTox-Ready Tag
- **Deliverable**: Update CHANGELOG.md with all changes; tag release `v1.4.0-qtox-preview`
- **Dependencies**: All previous steps completed
- **Goal Impact**: Enables qTox CI/CD integration per issue #43
- **Acceptance**: Tag exists; CI passes on tagged commit
- **Validation**: `git tag -l | grep v1.4.0`

## Scope Assessment Rationale

| Metric | Value | Assessment |
|--------|-------|------------|
| Functions above complexity 9.0 | 0 | No refactoring blockers |
| Duplication ratio | 0.67% | No consolidation needed |
| Doc coverage gap | 7% (93% achieved) | Minor documentation work |
| Unreferenced functions | 241 | Medium cleanup effort |
| Low cohesion packages | 3 | Medium reorganization effort |

**Estimated scope: Medium** (5–15 items, 10 steps planned)

## Validation Command Summary

```bash
# Full test suite (baseline)
go test -tags nonet -race ./...

# Relay enablement (Step 1)
grep -n "ConnectionRelay" transport/advanced_nat.go
go test -race ./transport/... -run TestRelay

# WAL default (Step 2)
go test -race ./async/... -run TestMessageStorage

# Documentation accuracy (Step 3)
grep -n "CVE-2021-4239" GAPS.md

# Message limits config (Step 4)
go test -race ./async/... -run TestMessageCapacity

# C API compatibility (Step 5)
CGO_ENABLED=1 go test -race ./capi/... -run TestCompatibility

# Dead code reduction (Step 7)
go-stats-generator analyze . --skip-tests --format console --sections burden 2>&1 | grep "Unreferenced"

# Crypto cohesion (Step 8)
go-stats-generator analyze . --skip-tests --format console --sections packages 2>&1 | grep "crypto"

# Examples build (Step 9)
go build ./examples/...
```

## Appendix: Metrics Source

- **Analysis Date**: 2026-03-25
- **Tool**: `go-stats-generator v1.0.0`
- **Command**: `go-stats-generator analyze . --skip-tests --format console`
- **Files Analyzed**: 229 (excluding tests)
- **Go Version**: 1.25.0 (toolchain go1.25.8)

### Dependency Security Status

| Dependency | Version | Status |
|------------|---------|--------|
| `flynn/noise` | v1.1.0 | ✅ Fixed (CVE-2021-4239 patched in v1.0.0) |
| `go-i2p/onramp` | v0.33.92 | ✅ Current |
| `opd-ai/magnum` | latest | ✅ Pure Go Opus |
| `opd-ai/vp8` | latest | ✅ Pure Go VP8 (I-frames only) |
| `pion/rtp` | v1.8.22 | ✅ Current |
| `golang.org/x/crypto` | v0.48.0 | ✅ Current |
| `testify` | v1.11.1 | ✅ Current |

### External Context

**GitHub Issues**:
- #43: qTox CI/CD integration request (open) — primary driver for this plan
- #35: Bootstrap connectivity issues (closed) — informs Step 6
- #30: Bootstrap failure handling (closed) — informs Step 6

**Competitive Landscape**:
- c-toxcore (TokTok/c-toxcore): Reference implementation in C; qTox's current backend
- tox-rs: Rust implementation (less mature)
- toxcore-go differentiator: Pure Go, multi-network transport, async messaging

---

*Generated from project analysis combining ROADMAP.md, GAPS.md, go-stats-generator metrics, and GitHub issue context*
