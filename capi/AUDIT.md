# Audit: github.com/opd-ai/toxcore/capi
**Date**: 2026-02-19
**Status**: Complete

## Summary
The `capi` package provides C-compatible FFI bindings for toxcore-go, enabling cross-language interoperability. The package exports 25 C functions across core Tox and ToxAV APIs with proper instance management, thread-safe operations, and comprehensive error handling. Code quality is high with strong test coverage (72.4%) and no critical issues. All functions properly handle nil pointers and invalid instances gracefully.

## Issues Found
- [ ] **low** API Design — Exported Go helper functions lack godoc comments (`getToxIDFromPointer:118`, `getToxInstance:159`, `getToxAVID:198` in `toxav_c.go`)
- [ ] **low** Documentation — C exported functions lack consistent godoc comments (only 3 of 25 have godoc: `toxav_new:211`, `toxav_kill:292`, `toxav_get_tox_from_av:320` in `toxav_c.go`)
- [ ] **low** Error Handling — Errors not wrapped with context before logging (`toxcore_c.go:33-36`, `toxav_c.go:244-251`)
- [ ] **med** Concurrency Safety — Global instance maps use basic incrementing IDs without overflow protection (`toxcore_c.go:21`, `toxav_c.go:174`)
- [ ] **low** API Design — Function `hex_string_to_bin` uses unsafe pointer arithmetic that could be refactored with safer slice operations (`toxcore_c.go:150-175`)
- [ ] **low** Test Coverage — Missing table-driven tests for C callback bridge functions (only smoke tests present in `toxav_c_test.go`)
- [ ] **low** Documentation — Package-level doc.go describes "package main" but doesn't document individual exported symbols (`doc.go:113`)
- [ ] **low** Memory Safety — Potential panic recovery in `getToxIDFromPointer` could mask legitimate bugs; consider more explicit validation (`toxav_c.go:128-138`)

## Test Coverage
72.4% (target: 65%)

**Breakdown:**
- `toxcore_c.go`: Well-covered with unit, integration tests
- `toxav_c.go`: Good coverage for instance management and basic operations
- Missing: Complex callback bridge invocation scenarios with actual C callbacks

## Dependencies

**External Dependencies:**
- `github.com/opd-ai/toxcore` — Core Go implementation (internal dependency)
- `github.com/opd-ai/toxcore/av` — Audio/video package (internal dependency)
- `github.com/sirupsen/logrus` — Structured logging
- Standard library: `unsafe`, `sync`, `encoding/hex`

**Integration Points:**
- Bridges Go toxcore API to C FFI
- Shared instance management across `toxcore_c.go` and `toxav_c.go`
- No external packages import this (leaf package for FFI)

## Recommendations
1. Add godoc comments to all exported Go helper functions (3 functions in `toxav_c.go`)
2. Add godoc comments to all C-exported functions following pattern from existing ones
3. Implement ID overflow protection or use uint64/UUID for instance IDs
4. Wrap errors with context using `fmt.Errorf("%w", err)` before logging
5. Add table-driven tests for callback bridge functions with mock C callbacks
6. Consider removing panic recovery in `getToxIDFromPointer` or document security implications
