# Audit: github.com/opd-ai/toxcore
**Date**: 2026-02-18
**Status**: Needs Work

## Summary
The root package provides the main Tox protocol API with 3 source files (toxcore.go, toxav.go, options.go) implementing 143 Tox methods across ~5100 lines. Overall health is good with comprehensive functionality, but test coverage (64.3%) falls slightly below the 65% target. Critical issues include non-deterministic time usage and several intentionally swallowed errors in test code paths.

## Issues Found
- [x] high network — Type assertions to concrete net types (*net.UDPAddr) violate interface-based networking guidelines (`toxav.go:26-30`, `toxav.go:120`, `toxav.go:136`) — **FIXED**: Refactored `extractIPBytes()` to use `net.SplitHostPort()` and `net.ParseIP()` instead of type switches; refactored `Send()` to use `net.ResolveUDPAddr()` instead of creating concrete `*net.UDPAddr` directly
- [x] high determinism — Non-deterministic time.Now() usage in friend request retry logic (`toxcore.go:1294`, `toxcore.go:1308`, `toxcore.go:1310`, `toxcore.go:1335`) — **FIXED**: Introduced `TimeProvider` interface with injectable `RealTimeProvider` (production) and `MockTimeProvider` (testing); replaced direct `time.Now()` calls with `t.now()` method
- [x] high determinism — Non-deterministic time.Now() usage for LastSeen timestamps (`toxcore.go:1909`, `toxcore.go:1947`, `toxcore.go:2280`) — **FIXED**: All LastSeen timestamp assignments now use injectable time provider via `t.now()`
- [x] high determinism — Non-deterministic file transfer ID generation using time.Now().UnixNano() (`toxcore.go:2941`) — **FIXED**: File transfer ID generation now uses `t.now().UnixNano()` for deterministic testing
- [x] med error-handling — Error intentionally swallowed in test path best-effort send (`toxcore.go:1273`) — **FIXED**: Added comprehensive comment explaining why error is intentionally ignored (test-only best-effort, already queued for retry, global test registry provides alternate delivery)
- [x] med error-handling — Error intentionally swallowed in test path friend request handler (`toxcore.go:1424`) — **FIXED**: Added comprehensive comment explaining why error is intentionally ignored (test helper function, errors logged internally, best-effort delivery mechanism)
- [x] med error-handling — Unused variable msg with swallowed potential warnings (`toxcore.go:2179`) — **FIXED**: Replaced `msg, err := ...` with `_, err := ...` and added comment explaining the Message object is intentionally discarded because caller only needs success/failure status
- [ ] low test-coverage — Test coverage at 64.3%, below 65% target (needs ~1% improvement)
- [ ] low doc-coverage — Package lacks doc.go file (though package comment exists in toxcore.go:1-35)

## Test Coverage
64.3% (target: 65%)

## Integration Status
This is the primary integration point for the entire toxcore library. It orchestrates:
- DHT routing via `dht` package
- Network transport via `transport` package (UDP/TCP)
- Cryptographic operations via `crypto` package
- Friend management via `friend` package
- Async messaging via `async` package
- File transfers via `file` package
- Group chat via `group` package
- Message handling via `messaging` package

All components properly registered and initialized in New() constructor (toxcore.go:400+). The Tox struct serves as the main API facade integrating all subsystems. Bootstrap manager, packet delivery factory, and message manager are all properly initialized.

## Recommendations
1. ~~**High Priority**: Replace time.Now() calls with injectable time provider for deterministic testing (affects friend requests, LastSeen timestamps, file transfer IDs)~~ — **COMPLETED**: Added `TimeProvider` interface, `RealTimeProvider`, `SetTimeProvider()` method, and `now()` helper; tests in `time_provider_test.go`
2. ~~**High Priority**: Refactor extractIPBytes() in toxav.go to eliminate type assertions to concrete net types - use interface methods like .Network() and .String() parsing instead~~ — **COMPLETED**
3. ~~**Medium Priority**: Document swallowed errors with explicit comments explaining why errors are intentionally ignored in test paths (toxcore.go:1273, 1424)~~ — **COMPLETED**: Added comprehensive comments explaining test-only best-effort semantics
4. ~~**Medium Priority**: Replace unused msg variable handling with explicit error check or document why message object is not needed (toxcore.go:2179)~~ — **COMPLETED**: Changed to `_, err := ...` with explanatory comment
5. **Low Priority**: Increase test coverage by ~1% to meet 65% target - focus on error paths and edge cases in friend request retry logic
6. **Low Priority**: Create doc.go file to formalize package documentation (currently embedded in toxcore.go header)
