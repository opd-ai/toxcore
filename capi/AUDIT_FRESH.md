# Audit: github.com/opd-ai/toxcore/capi
**Date**: 2026-02-18
**Status**: Complete

## Summary
The `capi` package provides C-compatible bindings for toxcore-go (toxcore and toxav), enabling cross-language interoperability. The package consists of 4 Go source files (~1218 total lines) implementing complete ToxAV and basic Tox C API functions. Test coverage is at 57.2% (below the 65% target). All medium and high priority issues have been resolved. Remaining low-priority issues include test coverage improvements and additional godoc comments.

## Issues Found
- [x] high stub — ✅ FIXED: All six callback registration functions now properly bridge to C via CGO using `invoke_*_cb` C functions (`toxav_c.go:578-776`)
- [x] high unsafe — ✅ FIXED: `go vet` passes; pointer handling now uses proper `unsafe.Pointer` storage via `toxavHandles` map (`toxav_c.go:277`)
- [x] high error-handling — ✅ FIXED: Error from `toxcore.New()` in `tox_new()` now logged with structured logrus.WithFields context (`toxcore_c.go:29-35`)
- [x] med stub — ✅ FIXED: Removed obsolete `toxav_placeholder.go` file — `toxav_c.go` provides complete implementation
- [x] med doc — ✅ FIXED: Created `doc.go` with comprehensive package-level documentation including build instructions, C API usage examples, callback bridging notes, thread safety, and limitations
- [x] med error-handling — ✅ FIXED: All seven ToxAV operation functions (Call, Answer, CallControl, AudioSetBitRate, VideoSetBitRate, AudioSendFrame, VideoSendFrame) now log failures with structured context including function name, parameters, and error details
- [ ] low test-coverage — Test coverage at 57.2%, below 65% target by 7.8 percentage points — Missing coverage for error paths in callback registration, unsafe pointer recovery paths, and edge cases in frame sending functions
- [ ] low doc — Exported function `getToxIDFromPointer` lacks godoc comment (`toxav_c.go:75`) — Function has good inline documentation but missing standard godoc format
- [ ] low doc — Exported function `getToxInstance` lacks godoc comment (`toxav_c.go:119`) — Function serves as critical bridge between toxcore_c.go and toxav_c.go but lacks godoc
- [ ] low doc — Exported function `getToxAVID` lacks comprehensive godoc (`toxav_c.go:138`) — Function has basic comment but should document return values and error conditions

## Test Coverage
57.2% (target: 65%)

**Test files:**
- `toxav_c_test.go` (141 lines) — Basic nil pointer handling tests and thread safety
- `toxav_integration_test.go` (234 lines) — Integration tests for ToxAV instance lifecycle, retrieval, cleanup, and concurrent access

**Coverage gaps:**
- Callback registration with valid C function pointers (placeholder implementation not exercised)
- Error paths in `toxav_new` when `NewToxAV` fails
- Recovery from panic in `getToxIDFromPointer` (defer/recover path at line 88-96)
- `hex_string_to_bin` function in toxcore_c.go (0% coverage)
- Error paths in frame sending functions (AudioSendFrame, VideoSendFrame)

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

**Missing Integrations:**
- No handler registration in DHT or transport layers (C API is standalone)
- Callback bridge to C function pointers not implemented (all callbacks are placeholders)
- No serialization/deserialization support (not applicable for C bindings layer)

## Recommendations
1. ~~**High Priority**: Implement C callback bridge in all six callback registration functions~~ ✅ FIXED
2. ~~**High Priority**: Fix unsafe.Pointer misuse at `toxav_c.go:268`~~ ✅ FIXED
3. ~~**High Priority**: Add structured logging for `toxcore.New()` error in `tox_new()`~~ ✅ FIXED
4. ~~**Medium Priority**: Remove or repurpose `toxav_placeholder.go`~~ ✅ FIXED — File removed; `toxav_c.go` provides complete implementation
5. ~~**Medium Priority**: Create `doc.go` with package-level documentation~~ ✅ FIXED — Created comprehensive `doc.go` with build instructions, C API usage examples, callback bridging notes, thread safety, and limitations
6. ~~**Medium Priority**: Add error logging to ToxAV operation functions~~ ✅ FIXED — All seven functions now log failures with structured context
7. **Low Priority**: Add godoc comments to exported bridge functions — Document `getToxIDFromPointer`, `getToxInstance`, `getToxAVID` with standard godoc format including parameter and return value descriptions
8. **Low Priority**: Increase test coverage to 65%+ — Add tests for `hex_string_to_bin`, error paths in `toxav_new`, panic recovery in `getToxIDFromPointer`, and frame sending edge cases
