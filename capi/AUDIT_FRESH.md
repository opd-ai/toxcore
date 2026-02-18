# Audit: github.com/opd-ai/toxcore/capi
**Date**: 2026-02-18
**Status**: Complete

## Summary
The `capi` package provides C-compatible bindings for toxcore-go (toxcore and toxav), enabling cross-language interoperability. The package consists of 4 Go source files (~1218 total lines) implementing complete ToxAV and basic Tox C API functions. Test coverage is at 72.4% (exceeds the 65% target). All issues have been resolved.

## Issues Found
- [x] high stub — ✅ FIXED: All six callback registration functions now properly bridge to C via CGO using `invoke_*_cb` C functions (`toxav_c.go:578-776`)
- [x] high unsafe — ✅ FIXED: `go vet` passes; pointer handling now uses proper `unsafe.Pointer` storage via `toxavHandles` map (`toxav_c.go:277`)
- [x] high error-handling — ✅ FIXED: Error from `toxcore.New()` in `tox_new()` now logged with structured logrus.WithFields context (`toxcore_c.go:29-35`)
- [x] med stub — ✅ FIXED: Removed obsolete `toxav_placeholder.go` file — `toxav_c.go` provides complete implementation
- [x] med doc — ✅ FIXED: Created `doc.go` with comprehensive package-level documentation including build instructions, C API usage examples, callback bridging notes, thread safety, and limitations
- [x] med error-handling — ✅ FIXED: All seven ToxAV operation functions (Call, Answer, CallControl, AudioSetBitRate, VideoSetBitRate, AudioSendFrame, VideoSendFrame) now log failures with structured context including function name, parameters, and error details
- [x] low test-coverage — ✅ FIXED: Test coverage improved to 72.4%, exceeds 65% target
- [x] low doc — ✅ N/A: `getToxIDFromPointer` is unexported (lowercase first letter) — godoc not required for package-internal functions; function has comprehensive inline documentation
- [x] low doc — ✅ N/A: `getToxInstance` is unexported (lowercase first letter) — godoc not required for package-internal functions; function has comprehensive inline documentation  
- [x] low doc — ✅ N/A: `getToxAVID` is unexported (lowercase first letter) — godoc not required for package-internal functions; function has comprehensive inline documentation

## Test Coverage
72.4% (target: 65%) ✅ EXCEEDS TARGET

**Test files:**
- `toxav_c_test.go` (141 lines) — Basic nil pointer handling tests and thread safety
- `toxav_integration_test.go` (234 lines) — Integration tests for ToxAV instance lifecycle, retrieval, cleanup, and concurrent access

## Integration Status
The capi package serves as the primary C interoperability layer for toxcore-go:

**Integration Points:**
- Bridges Go toxcore/toxav implementations to C applications via CGO
- Uses global instance maps (`toxInstances`, `toxavInstances`) with mutex protection for thread safety
- Properly exports functions with `//export` directives for C visibility
- Links ToxAV instances to originating Tox instances via `toxavToTox` map

**C API Compatibility:**
- ToxAV API functions match libtoxcore signatures exactly per comments
- C header declarations provided in toxav_c.go lines 18-63
- Supports building as c-shared library (requires `package main` and `func main()`)

## Recommendations
All issues have been resolved. The package is complete.
