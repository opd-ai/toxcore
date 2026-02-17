# Audit: github.com/opd-ai/toxcore/capi
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The `capi` package provides C-compatible bindings for toxcore-go, enabling cross-language interoperability. The package has good structure with 57.2% test coverage, but suffers from incomplete callback implementations, unsafe pointer misuse flagged by `go vet`, error handling gaps, and missing package documentation. Critical issues include placeholder callback bridges and unsafe pointer conversion that violates Go's safety guarantees.

## Issues Found
- [x] **high** Stub/incomplete code — Callback functions use placeholder implementations that don't bridge to C (`toxav_c.go:527-640`)
- [x] **high** Error handling — Error from `toxcore.New()` not logged with structured context (`toxcore_c.go:29-31`)
- [x] **high** Error handling — Error from `Bootstrap()` not logged with structured context (`toxcore_c.go:82-84`)
- [x] **high** Safety violation — `go vet` reports "possible misuse of unsafe.Pointer" in toxav_to_tox mapping (`toxav_c.go:268`)
- [x] **med** Doc coverage — Package lacks `doc.go` file explaining C API architecture and build instructions
- [x] **med** Error handling — Multiple errors returned as boolean/int without logging context (`toxav_c.go:337,361,387,411,435,466,503`)
- [x] **med** Stub/incomplete code — `toxav_placeholder.go` entire file is placeholder with no implementation (lines 1-29)
- [x] **low** Doc coverage — Exported helper functions `getToxIDFromPointer` and `getToxInstance` lack godoc comments (`toxav_c.go:78,119`)
- [x] **low** Error handling — Panic recovery in `getToxIDFromPointer` uses generic message instead of detailed context (`toxav_c.go:88-96`)
- [x] **low** Test coverage — 57.2% below 65% target; missing tests for error logging paths and unsafe pointer edge cases
- [x] **low** Code style — Empty callback bodies in registration functions could be replaced with explicit TODO comments (`toxav_c.go:529,552,574,595,616,637`)
- [x] **low** Integration points — No verification that C header files (toxav.h, toxcore.h) are generated correctly during build

## Test Coverage
57.2% (target: 65%)

**Gap Analysis:**
- Error logging code paths not covered (toxcore_c.go error returns)
- Unsafe pointer dereference panic recovery not tested
- C callback bridge integration not testable until implemented
- Build artifact validation (generated .h files) not automated

## Integration Status
The package integrates with toxcore-go core library through direct imports. C API functions are exported using CGO's `//export` directive for building as c-shared library. 

**Integration Points:**
- **Tox Core**: Wraps `toxcore.Tox` via instance map in `toxcore_c.go`
- **ToxAV**: Wraps `toxcore.ToxAV` via instance map in `toxav_c.go`
- **C Bindings**: 24 exported C functions across both files
- **Build System**: Requires `go build -buildmode=c-shared -o libtoxav.so capi/*.go`

**Missing Registrations:**
- No system registration needed (C API layer, not ECS)
- C header generation not validated in CI pipeline
- No documented process for C API versioning/compatibility

## Recommendations
1. **Fix unsafe.Pointer violation** (`toxav_c.go:268`) - Use proper type assertion or interface method instead of storing `unsafe.Pointer` as `uintptr` in map
2. **Implement C callback bridge** - Replace placeholder callbacks with proper CGO function pointer invocations and user_data handling
3. **Add structured logging** - Use `logrus.WithFields` for all error paths with context (function name, parameters, error details)
4. **Create doc.go** - Document C API architecture, build process, compatibility guarantees, and usage examples
5. **Increase test coverage** - Add tests for error logging, panic recovery, concurrent access patterns, and multiple instance lifecycle
6. **Remove placeholder file** - Delete `toxav_placeholder.go` or move to documentation
7. **Add build validation** - Automate verification that generated C headers match implementation signatures
8. **Add godoc comments** - Document all exported helper functions per Go conventions
