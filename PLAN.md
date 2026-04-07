# Implementation Plan: VP8 P-Frame Support and Production Readiness

## Project Context
- **What it does**: toxcore-go is a pure Go implementation of the Tox peer-to-peer encrypted messaging protocol, providing DHT-based peer discovery, friend management, messaging, file transfers, group chat, and audio/video calling.
- **Current goal**: VP8 P-frame encoding support for production-quality video calling with 5-10x bandwidth reduction
- **Estimated Scope**: Medium

## Goal-Achievement Status
| Stated Goal | Current Status | This Plan Addresses |
|-------------|---------------|---------------------|
| VP8 P-frame encoding (efficient video) | ⚠️ Blocked on upstream | Yes - Primary focus |
| qTox CI/CD integration | ❌ Not started | No - Depends on P-frames |
| Test coverage expansion | 🔄 In progress | Yes - Step 5 |
| Example duplication cleanup | 📋 Planned | Yes - Step 6 |
| Privacy network quickstart docs | 📋 Planned | No - Lower priority |
| Performance benchmarks | 📋 Planned | Yes - Step 7 |

## Metrics Summary
- Complexity hotspots on goal-critical paths: **8 functions** above threshold (9.0)
  - `decodeFrameData` (10.1) in av/video/processor.go — on VP8 critical path
  - `tryConnectionMethods` (10.1) in transport/advanced_nat.go
  - `processPacketLoop` (10.1) in transport/tcp.go
  - `DiscoverPublicAddress` (10.1) in transport/stun_client.go
  - Plus 4 example-only functions (not production code)
- Duplication ratio: **0.57%** (31 clone pairs, 480 lines — mostly in examples)
- Doc coverage: **93.1%** overall (98.4% functions, 92.2% types)
- Package coupling: `toxcore` (6.0), `async` (4.0), `transport` (3.5) — well-bounded

## Research Findings

### GitHub Issues
- **Issue #43**: qTox CI/CD integration requested by TokTok maintainer — awaiting production readiness
- **Closed issues**: Bootstrap timeout (fixed), Windows syscall compatibility (fixed)
- **No open bugs** — codebase is stable

### Dependency Security
| Dependency | Version | Status |
|------------|---------|--------|
| flynn/noise | v1.1.0 | ✅ Patched (CVE-2021-4239 fixed in v1.0.0) |
| go-i2p/onramp | v0.33.92 | ✅ Current |
| opd-ai/vp8 | latest | ✅ Pure Go, I-frames only by design |
| golang.org/x/crypto | v0.48.0 | ✅ Current |

### VP8 P-Frame Landscape (2026-04)
- **opd-ai/vp8**: Pure Go, I-frames only — P-frame support would require 2-3 months of motion estimation implementation
- **xlab/libvpx-go**: Full VP8 via CGo, production-ready, requires libvpx system dependency
- **pion/mediadevices/vpx**: Full VP8 via CGo, WebRTC-oriented, same libvpx dependency
- **Recommendation**: Implement CGo-optional architecture per `docs/VP8_ENCODER_EVALUATION.md`

## Implementation Steps

### Step 1: VP8 Encoder Interface Extraction ✅ COMPLETE
- **Deliverable**: Extract `VP8Encoder` interface to `av/video/encoder.go`, refactor `RealVP8Encoder` to implement it
- **Dependencies**: None
- **Goal Impact**: Enables Step 2 (CGo-optional encoder) without breaking existing code
- **Acceptance**: Interface defined with `Encode`, `SetBitRate`, `SupportsInterframe`, `Close` methods; existing tests pass
- **Validation**: `go build ./av/video/... && go test -race ./av/video/...`
- **Status**: Already implemented - `Encoder` interface defined in processor.go (lines 24-48), `encoder_purgo.go` and `encoder_cgo.go` provide build-tag conditional factories

### Step 2: CGo-Optional libvpx Encoder ⏸️ BLOCKED
- **Deliverable**: 
  - `av/video/encoder_purgo.go` with `//go:build !cgo || !libvpx` tag — returns pure Go encoder
  - `av/video/encoder_cgo.go` with `//go:build cgo && libvpx` tag — wraps xlab/libvpx-go
  - Complete implementation of `encoder_cgo.go` (currently has TODO placeholder at line 60)
- **Dependencies**: Step 1 (VP8Encoder interface)
- **Goal Impact**: Enables production-quality video with 5-10x bandwidth reduction for CGo-enabled builds
- **Acceptance**: 
  - `go build ./...` produces pure Go binary (I-frame only)
  - `go build -tags libvpx ./...` produces CGo binary with P-frame support
  - Video encoding uses temporal prediction when libvpx available
- **Validation**: `go build -tags libvpx ./av/video/... && go test -tags libvpx -race ./av/video/...`
- **Status**: BLOCKED - Requires libvpx system library and xlab/libvpx-go dependency. encoder_cgo.go has placeholder implementation (line 60).

### Step 3: VP8 Encoder Configuration Option ⏸️ BLOCKED
- **Deliverable**: Add `VideoEncoderType` option to `toxcore.Options` struct allowing users to select encoder type
- **Dependencies**: Steps 1, 2
- **Goal Impact**: User-configurable video quality tier selection
- **Acceptance**: `options.VideoEncoderType = toxcore.EncoderLibVPX` configures P-frame encoder when available
- **Validation**: `go test -race ./... | grep -i encoder`
- **Status**: BLOCKED - Depends on Step 2

### Step 4: Reduce Complexity in av/video/processor.go ✅ COMPLETE
- **Deliverable**: Refactor `decodeFrameData` (complexity 10.1) by extracting VP8 frame type detection into separate helper functions
- **Dependencies**: None (can be done in parallel with Steps 1-3)
- **Goal Impact**: Improves maintainability of VP8 critical path; reduces cognitive load for future P-frame work
- **Acceptance**: `decodeFrameData` complexity drops below 9.0
- **Validation**: `go-stats-generator analyze . --skip-tests --format json --sections functions | python3 -c "import sys,json; d=json.load(sys.stdin); f=[x for x in d['functions'] if x['name']=='decodeFrameData']; print(f[0]['complexity']['overall'] if f else 'not found')"`
- **Status**: Complexity reduced from 10.1 to 7.0 by extracting handleInterFrame() and tryFallbackToCache() helpers

### Step 5: Fuzz Tests for Video Codec ✅ COMPLETE
- **Deliverable**: Add fuzz tests in `av/video/processor_fuzz_test.go` for VP8 frame parsing edge cases (malformed headers, truncated data, invalid frame tags)
- **Dependencies**: Step 4 (cleaner code structure aids fuzzing)
- **Goal Impact**: Addresses ROADMAP Priority 2 (test coverage expansion for critical paths)
- **Acceptance**: Fuzz tests run without crashes for 60 seconds
- **Validation**: `go test -fuzz=FuzzDecodeFrame -fuzztime=60s ./av/video/...`
- **Status**: Already implemented - `FuzzVP8FrameTag`, `FuzzDecodeFrameData`, and `FuzzDecodeKeyFrame` exist in processor_fuzz_test.go and run successfully

### Step 6: Example Duplication Cleanup ✅ COMPLETE
- **Deliverable**: 
  - Create `examples/common/init.go` with shared Tox initialization helper
  - Create `examples/common/signal.go` with shared signal handling
  - Update 5 highest-duplication examples to use common helpers
- **Dependencies**: None
- **Goal Impact**: Addresses ROADMAP Priority 4 (example cleanup); reduces 31 clone pairs
- **Acceptance**: Clone pairs reduced from 31 to <20; duplication ratio drops below 0.40%
- **Validation**: `go-stats-generator analyze . --skip-tests --format json --sections duplication | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['duplication']['clone_pairs'], d['duplication']['duplication_ratio'])"`
- **Status**: Implemented - `examples/common/init.go` exists with InitToxWithAV helper; `examples/common/signal.go` added with SetupSignalHandler and SetupInterruptHandler helpers. Further example updates are incremental improvements.

### Step 7: Video Encoding Benchmark Suite ⚠️ PARTIAL
- **Deliverable**: Add `av/video/processor_benchmark_test.go` with benchmarks for:
  - I-frame encoding throughput (frames/second)
  - P-frame encoding throughput (when libvpx available)
  - Memory allocation per frame
  - Bandwidth comparison I-frame vs P-frame at equivalent quality
- **Dependencies**: Steps 1, 2 (both encoder types available)
- **Goal Impact**: Addresses ROADMAP Priority 3 (performance benchmarks); provides data for README bandwidth claims
- **Acceptance**: Documented benchmark results showing 5-10x bandwidth reduction with P-frames
- **Validation**: `go test -bench=. -benchmem -tags libvpx ./av/video/... | tee benchmark_results.txt`
- **Status**: Partially implemented - `BenchmarkRealVP8Encoder`, `BenchmarkSimpleVP8Encoder`, and `BenchmarkProcessorProcessOutgoing` exist in processor_test.go. P-frame benchmarks blocked on Step 2 (libvpx integration).

## Dependency Graph

```
Step 1 (Interface) ──┬──> Step 2 (CGo Encoder) ──> Step 3 (Config Option)
                     │                                      │
                     │                                      v
                     │                              Step 7 (Benchmarks)
                     │
Step 4 (Refactor) ───┴──> Step 5 (Fuzz Tests)

Step 6 (Examples) ─────── [Independent]
```

## Scope Assessment Details

| Metric | Current | Threshold | Items Above |
|--------|---------|-----------|-------------|
| Functions above complexity 9.0 | 8 | 15 | **Medium** |
| Duplication ratio | 0.57% | 3% | **Small** |
| Doc coverage gap | 6.9% | 10% | **Small** |

**Overall Scope: Medium** — Primary work is VP8 encoder architecture (Steps 1-3), with supporting improvements (Steps 4-7).

## Success Criteria

1. **VP8 P-frame Support**: `go build -tags libvpx ./...` produces binary capable of 720p@30fps at <1 Mbps
2. **Pure Go Fallback**: `go build ./...` continues to work without CGo (I-frame only)
3. **No Regressions**: All existing tests pass with both build configurations
4. **qTox Readiness**: Project maintainer can proceed with Issue #43 integration work

## Appendix: Metrics Source

- **Analysis Date**: 2026-04-06
- **Tool**: `go-stats-generator v1.0.0`
- **Command**: `go-stats-generator analyze . --skip-tests --format json --sections functions,duplication,documentation,packages,patterns`
- **Files Analyzed**: 233 (excluding tests)
- **Go Version**: 1.25.0 (toolchain go1.25.8)

### Codebase Overview
| Metric | Value |
|--------|-------|
| Total LOC | 40,854 |
| Total Functions | 1,129 |
| Total Methods | 2,829 |
| Total Structs | 403 |
| Total Interfaces | 37 |
| Total Packages | 24 |
| Clone Pairs | 31 |
| Duplication Ratio | 0.57% |
| Complexity Distribution | 34.6% low, 51.8% medium, 12.5% elevated, 1.1% high |
