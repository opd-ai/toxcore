# Audit: github.com/opd-ai/toxcore/av
**Date**: 2026-02-19
**Status**: Complete

## Summary
The av package implements audio/video calling functionality with 23 source files across main package and 3 sub-packages (audio, rtp, video). Overall health is good with comprehensive test coverage (78-90% across sub-packages) and well-structured interfaces. Main concerns involve direct use of concrete network types violating project network guidelines and multiple direct time.Now() calls that could impact determinism.

## Issues Found

### High Priority
None

### Medium Priority
- [ ] med **API Design** — Concrete network type usage violates project guidelines (`types.go:592`, `types.go:609`)
- [ ] med **Determinism** — Direct time.Now() usage in production code reduces reproducibility (`manager.go:1040`, `manager.go:1095`, `manager.go:1148`, `manager.go:1202`, `manager.go:1256`, `manager.go:1310`, `manager.go:1364`)

### Low Priority
- [ ] low **Documentation** — Placeholder address comments suggest incomplete implementation (`types.go:141`, `types.go:468`, `types.go:577-617`)
- [ ] low **Code Quality** — fmt.Printf usage for user-facing output instead of callbacks (`manager.go:1011-1012`, `manager.go:1063`)
- [ ] low **API Design** — Multiple callback function types could be consolidated into interface (`manager.go:57-62`)
- [ ] low **Error Handling** — Some functions return nil without context in success paths (acceptable pattern but verbose)
- [ ] low **Dependencies** — External codec dependencies (pion/opus, pion/rtp) should be justified in docs

## Test Coverage

**Overall**: 78.0% (main av package)
**Sub-packages**:
- av/audio: 85.2%
- av/rtp: 89.5%
- av/video: 89.8%

All sub-packages exceed 65% target. Main package at 78% is acceptable but could improve.

**Race Detection**: PASS (go test -race completed successfully)
**Table-Driven Tests**: Present in all _test.go files
**Benchmarks**: Present in performance_test.go

## Dependencies

**Internal**:
- github.com/opd-ai/toxcore/transport (RTP session integration)
- github.com/opd-ai/toxcore/av/audio (Opus codec)
- github.com/opd-ai/toxcore/av/rtp (RTP protocol)
- github.com/opd-ai/toxcore/av/video (VP8 codec)

**External**:
- github.com/pion/opus (audio encoding/decoding)
- github.com/pion/rtp (RTP packet handling)
- github.com/sirupsen/logrus (structured logging)

External codec dependencies are justified for production-quality media handling.

## Recommendations

1. **HIGH**: Refactor `types.go:592` and `types.go:609` to use `net.Addr` interface instead of concrete `&net.UDPAddr{}` to align with project networking best practices
2. **MEDIUM**: Replace all direct `time.Now()` calls in production code paths with TimeProvider pattern (already defined but not consistently used)
3. **MEDIUM**: Replace fmt.Printf user-facing output with proper callbacks or return structured events
4. **LOW**: Document the "placeholder address" fallback strategy or implement full address resolution
5. **LOW**: Consider callback consolidation interface for cleaner API (breaking change, low priority)

