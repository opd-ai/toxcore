# Audit: github.com/opd-ai/toxcore/capi
**Date**: 2026-02-20
**Status**: Needs Work

## Summary
The C API bindings package provides cross-language interoperability for toxcore-go. Well-structured with comprehensive C bridging for both ToxCore and ToxAV APIs. Major concerns include lack of error wrapping, inconsistent nil checks, and below-target test coverage.

## Issues Found
- [ ] **high** Error Handling — No error wrapping with context (%w) in any C API functions (`toxav_c.go:302,310,318,331,336`)
- [ ] **high** Error Handling — Error from getToxIDFromPointer not checked or propagated (`toxcore_c.go:93-95`)
- [ ] **high** Concurrency Safety — Panic recovery in getToxIDFromPointer may mask critical issues and violates Go best practices (`toxav_c.go:182-191`)
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
1. Add error wrapping with %w throughout all C API functions that return Go errors
2. Implement proper error propagation in getToxIDFromPointer without panic recovery
3. Replace string-based error classification with error type assertions or sentinel errors
4. Increase test coverage to meet 65% target with additional edge case tests
5. Add godoc comments for all helper functions and document c-shared build requirements
