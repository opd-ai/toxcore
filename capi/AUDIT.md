# Audit: github.com/opd-ai/toxcore/capi
**Date**: 2026-02-19
**Status**: Complete

## Summary
The capi package provides C API bindings for toxcore-go enabling cross-language interoperability. Overall health is good with well-structured code, comprehensive documentation, and 72.4% test coverage. Critical issues include unused error parameters in multiple API functions and cross-file variable access that violates package encapsulation.

## Issues Found
- [x] **high** Error Handling — error_ptr parameter now properly populated in toxav_call, toxav_answer, toxav_call_control and all bit rate/frame functions with appropriate error codes
- [x] **high** API Design — Created GetToxInstanceByID accessor function with proper mutex protection to replace direct map access
- [x] **med** Concurrency Safety — getToxInstance now uses the thread-safe GetToxInstanceByID accessor
- [x] **med** Error Handling — Added bounds validation in audio/video frame functions before unsafe slice conversions
- [x] **med** API Design — getToxInstance function now uses the thread-safe GetToxInstanceByID accessor with mutex protection
- [x] **low** Documentation — Added comprehensive godoc comments for toxavCallbacks struct documenting all callback fields and usage patterns (`toxav_c.go:227-242`)
- [x] **low** Error Handling — hex_string_to_bin now uses unsafe.Slice for input and copy builtin for output (`toxcore_c.go:161-182`)
- [x] **low** API Design — main() function now has comprehensive godoc explaining c-shared build mode requirements (`toxcore_c.go:12-18`)
- [x] **low** Memory Safety — Added bounds validation for unsafe slice conversions (`toxav_c.go:580,625`)

## Test Coverage
72.4% (target: 65%) ✓

## Dependencies
**External:**
- `github.com/opd-ai/toxcore` (parent package for Tox/ToxAV types)
- `github.com/opd-ai/toxcore/av` (for CallControl and CallState types)
- `github.com/sirupsen/logrus` (structured logging)
- `encoding/hex` (hex string decoding)
- `C` (CGo for C interop)

**Integration Points:**
- Bridges between C function pointers and Go callbacks using invoke_* C helper functions
- Manages opaque pointer handles for ToxAV instances via toxavHandles map
- Cross-file dependency on toxInstances map from toxcore_c.go

## Recommendations
1. **Fix error_ptr handling** — Update all API functions to properly populate error_ptr parameters with appropriate error codes instead of leaving them unused
2. **Add exported accessor for toxInstances** — Create getToxInstanceByID() exported function in toxcore_c.go to properly encapsulate instance map access
3. **Add bounds validation** — Add explicit bounds checks before unsafe slice conversions in audio/video frame functions
4. **Document thread safety** — Add comments explaining mutex protection strategy for callback storage and instance maps
5. **Review defer/recover pattern** — Consider safer alternative to getToxIDFromPointer's panic recovery approach
