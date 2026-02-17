# Audit: github.com/opd-ai/toxcore/friend
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The friend package implements core Tox friend management including FriendInfo struct, friend requests, and request handling. While the basic structure is sound with good test coverage for core FriendInfo methods, the package has medium-severity issues: missing serialization support for persistence, incomplete integration with Tox main loop, and missing input validation. All high-severity issues have been resolved (type duplication, concurrency protection, deterministic timestamps).

## Issues Found
- [x] high — **Duplicate Friend type definition** — Friend struct defined both in `friend/friend.go` and `toxcore.go:1736`, causing type conflicts and maintenance burden; the toxcore.go version is used in actual Tox implementation while friend/ version is isolated (`friend.go:41`, `toxcore.go:1736`) — **RESOLVED**: Renamed `friend.Friend` to `friend.FriendInfo` to avoid namespace collision; toxcore.Friend remains the production type
- [x] high — **Missing concurrency protection** — RequestManager has no mutex protection for pendingRequests slice accessed from multiple methods, risking race conditions (`request.go:102-186`) — **RESOLVED**: Added `sync.RWMutex` to protect all pendingRequests access in RequestManager
- [x] high — **Non-deterministic timestamp usage** — Uses `time.Now()` directly in multiple locations, violating deterministic procgen requirement for reproducible state (`friend.go:64`, `friend.go:146`, `request.go:38`, `request.go:91`) — **RESOLVED**: Implemented `TimeProvider` interface with `DefaultTimeProvider`; added `NewWithTimeProvider()` and `NewRequestWithTimeProvider()` functions for deterministic testing
- [ ] med — **Missing serialization support** — No Marshal/Unmarshal methods for Friend or Request persistence, making savedata integration impossible (`friend.go:41-49`, `request.go:13-19`)
- [ ] med — **No error logging on failures** — Error paths in request.go don't use structured logging with logrus.WithFields, making debugging difficult (`request.go:26`, `request.go:32`, `request.go:52`, `request.go:70`, `request.go:83`)
- [ ] med — **RequestManager not integrated** — Zero usage of RequestManager in codebase (grep shows 0 imports), indicating incomplete integration with Tox main loop (`request.go:101-186`)
- [ ] med — **Missing input validation** — No length limits on Name, StatusMessage, or friend request Message fields, allowing unbounded memory allocation (`friend.go:81-110`, `request.go:24-42`)
- [ ] low — **Incomplete test coverage** — Only 25.7% coverage (target: 65%); missing tests for Request encryption/decryption, RequestManager operations, and error paths (`friend_test.go:1-178`)
- [ ] low — **Missing doc.go** — Package documentation exists in friend.go but no dedicated doc.go file for package-level overview
- [ ] low — **Status type name collision** — friend.Status type name may conflict with similar status types in other packages; consider more specific naming like FriendStatus to match toxcore.go convention (`friend.go:20`)
- [ ] low — **Logging inconsistency** — SetStatusMessage (line 108) lacks structured logging while other setters have comprehensive logging (`friend.go:108-110`)
- [ ] low — **Unused recipientPublicKey parameter** — NewRequest accepts recipientPublicKey parameter but never uses it in Request struct or logic (`request.go:24`)

## Test Coverage
25.7% (target: 65%)

**Missing test areas:**
- Request.Encrypt() and DecryptRequest() cryptographic operations (0% coverage)
- RequestManager.AddRequest() duplicate handling (0% coverage)
- RequestManager.AcceptRequest() and RejectRequest() (0% coverage)
- RequestManager.SetHandler() callback flow (0% coverage)
- Error path testing for invalid packets and encryption failures (0% coverage)

## Integration Status
**Critical integration gaps identified:**
- The friend/ package defines a FriendInfo type that is separate from toxcore.Friend (toxcore.go:1736) which has slightly different fields (adds IsTyping field, uses FriendStatus instead of Status enum). This is now intentional to allow independent evolution.
- RequestManager has zero integration points - not instantiated or used anywhere in the codebase.
- No registration in system initialization or packet handlers.
- No serialization support means FriendInfo/Request state cannot be persisted in savedata.
- Friend package does not interact with DHT, transport, or messaging layers despite being foundational to peer-to-peer communication.

**Expected integration points (missing):**
- Tox struct should have requestManager field using friend.RequestManager
- Friend request packets should be routed through RequestManager in Tox.Iterate()
- FriendInfo state should serialize/deserialize for savedata support
- Connection status changes should trigger DHT routing updates

## Recommendations
1. **RESOLVED**: Duplicate Friend type addressed by renaming friend.Friend to friend.FriendInfo
2. **RESOLVED**: sync.RWMutex added to RequestManager
3. **RESOLVED**: Time provider pattern implemented for deterministic testing
4. **HIGH**: Implement Marshal/Unmarshal methods for FriendInfo and Request types to support savedata persistence
5. **MED**: Add comprehensive error logging with logrus.WithFields on all error return paths in request.go
6. **MED**: Implement input validation with length limits (Name: 128 bytes, StatusMessage: 1007 bytes per Tox spec, Request message: 1016 bytes)
7. **MED**: Integrate RequestManager into Tox struct and wire up packet handling in main iteration loop
8. **LOW**: Increase test coverage to 65% target by adding tests for encryption, RequestManager operations, and all error paths
9. **LOW**: Create doc.go with comprehensive package-level documentation and usage examples
10. **LOW**: Add structured logging to SetStatusMessage for consistency with other setters
