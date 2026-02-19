# Audit: github.com/opd-ai/toxcore/capi
**Date**: 2026-02-19
**Status**: Complete

## Summary
The capi package provides C API bindings for toxcore-go enabling cross-language interoperability. Overall health is good with well-structured code, comprehensive documentation, and 72.4% test coverage. Critical issues include unused error parameters in multiple API functions and cross-file variable access that violates package encapsulation.

## Issues Found
- [ ] **high** Error Handling — error_ptr parameter unused in toxav_call, toxav_answer, toxav_call_control and all bit rate/frame functions (`toxav_c.go:392,426,460,495,528,561,604`)
- [ ] **high** API Design — Direct access to toxInstances map from toxcore_c.go breaks package encapsulation (`toxav_c.go:162`)
- [ ] **med** Concurrency Safety — Potential data race in getToxIDFromPointer with defer/recover pattern (`toxav_c.go:123-143`)
- [ ] **med** Error Handling — No validation of C pointer arithmetic in audio/video frame functions (`toxav_c.go:580,625`)
- [ ] **med** API Design — getToxInstance function accesses package-level variables without mutex protection (`toxav_c.go:159-166`)
- [ ] **low** Documentation — Missing godoc comments for toxavCallbacks struct (`toxav_c.go:179`)
- [ ] **low** Error Handling — hex_string_to_bin uses manual byte iteration instead of copy builtin (`toxcore_c.go:150-172`)
- [ ] **low** API Design — main() function is empty stub for c-shared build mode (`toxcore_c.go:15`)
- [ ] **low** Memory Safety — Large unsafe slice conversions (1<<20, 1<<24) without bounds validation (`toxav_c.go:580,625`)

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
