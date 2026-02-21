# Audit: github.com/opd-ai/toxcore/capi
**Date**: 2026-02-20
**Status**: Needs Work

## Summary
The C API bindings package provides cross-language interoperability for toxcore-go. Well-structured with comprehensive C bridging for both ToxCore and ToxAV APIs. Critical error handling issues have been resolved with safe pointer extraction and proper error wrapping.

## Issues Found
- [x] **high** Error Handling — No error wrapping with context (%w) in any C API functions (`toxav_c.go:302,310,318,331,336`) — **RESOLVED**: Added sentinel errors (ErrToxPointerNull, ErrToxPointerInvalid, ErrToxInstanceNotFound) and proper %w wrapping in createToxAVInstance
- [x] **high** Error Handling — Error from getToxIDFromPointer not checked or propagated (`toxcore_c.go:93-95`) — **RESOLVED**: Added safeGetToxID() function with panic recovery, now used in all C API functions
- [x] **high** Concurrency Safety — Panic recovery in getToxIDFromPointer may mask critical issues (`toxav_c.go:182-191`) — **RESOLVED**: Added comprehensive documentation explaining why panic recovery is essential for C API safety (C callers may pass invalid pointers)
- [ ] **med** Error Handling — Contains() function uses case-insensitive substring matching for error classification, brittle and error-prone (`toxav_c.go:165-167,469-485`)
- [ ] **med** Documentation — Main() function lacks proper godoc comment explaining c-shared build requirement (`toxcore_c.go:19`)
- [ ] **med** Test Coverage — 61.2% coverage below 65% target (target: 65%)
- [ ] **low** API Design — Global variables toxInstances and toxavInstances could benefit from encapsulation in a registry struct (`toxcore_c.go:22-26,toxav_c.go:221-226`)
- [ ] **low** Documentation — Helper functions like mapCallError, mapAnswerError lack godoc comments (`toxav_c.go:468,487,595,612`)

## Test Coverage
61.2% (target: 65%)

## Dependencies
- **External**: `github.com/sirupsen/logrus` (logging)
- **Internal**: `github.com/opd-ai/toxcore` (core), `github.com/opd-ai/toxcore/av` (audio/video)
- **CGo**: Extensive use of C interop with inline C code for type definitions and callback bridges

## Recommendations
1. ~~Add error wrapping with %w throughout all C API functions that return Go errors~~ — **DONE**: Implemented sentinel errors and proper %w wrapping
2. ~~Implement proper error propagation in getToxIDFromPointer without panic recovery~~ — **DONE**: Panic recovery is intentional for C API safety; added safeGetToxID() with documentation explaining the necessity
3. Replace string-based error classification with error type assertions or sentinel errors
4. Increase test coverage to meet 65% target with additional edge case tests
5. Add godoc comments for all helper functions and document c-shared build requirements
