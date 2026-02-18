# Audit: github.com/opd-ai/toxcore/friend
**Date**: 2026-02-17
**Status**: Complete

## Summary
The friend package implements core Tox friend management including FriendInfo struct, friend requests, and request handling. While the basic structure is sound with good test coverage for core FriendInfo methods, the package has medium-severity issues: missing serialization support for persistence, incomplete integration with Tox main loop, and missing input validation. All high-severity issues have been resolved (type duplication, concurrency protection, deterministic timestamps).

## Issues Found
- [x] high — **Duplicate Friend type definition** — Friend struct defined both in `friend/friend.go` and `toxcore.go:1736`, causing type conflicts and maintenance burden; the toxcore.go version is used in actual Tox implementation while friend/ version is isolated (`friend.go:41`, `toxcore.go:1736`) — **RESOLVED**: Renamed `friend.Friend` to `friend.FriendInfo` to avoid namespace collision; toxcore.Friend remains the production type
- [x] high — **Missing concurrency protection** — RequestManager has no mutex protection for pendingRequests slice accessed from multiple methods, risking race conditions (`request.go:102-186`) — **RESOLVED**: Added `sync.RWMutex` to protect all pendingRequests access in RequestManager
- [x] high — **Non-deterministic timestamp usage** — Uses `time.Now()` directly in multiple locations, violating deterministic procgen requirement for reproducible state (`friend.go:64`, `friend.go:146`, `request.go:38`, `request.go:91`) — **RESOLVED**: Implemented `TimeProvider` interface with `DefaultTimeProvider`; added `NewWithTimeProvider()` and `NewRequestWithTimeProvider()` functions for deterministic testing
- [x] med — **Missing serialization support** — No Marshal/Unmarshal methods for Friend or Request persistence, making savedata integration impossible (`friend.go:41-49`, `request.go:13-19`) — **RESOLVED**: Added Marshal/Unmarshal methods to both FriendInfo and Request types; implemented using JSON serialization for consistency with toxcore savedata; added friendInfoSerialized and requestSerialized internal types for JSON encoding; added UnmarshalFriendInfo() and UnmarshalRequest() convenience functions; comprehensive tests added including round-trip validation; test coverage improved from 25.7% to 52.6%
- [x] med — **No error logging on failures** — Error paths in request.go don't use structured logging with logrus.WithFields, making debugging difficult (`request.go:26`, `request.go:32`, `request.go:52`, `request.go:70`, `request.go:83`) — **RESOLVED**: Added comprehensive structured logging with logrus.WithFields to all error paths in NewRequestWithTimeProvider(), Encrypt(), and DecryptRequestWithTimeProvider(); logs include context (public keys, message lengths, error details) at appropriate levels (Warn for validation failures, Error for operation failures, Debug for success)
- [x] med — **RequestManager not integrated** — Zero usage of RequestManager in codebase (grep shows 0 imports), indicating incomplete integration with Tox main loop (`request.go:101-186`) — **RESOLVED**: Added `requestManager *friend.RequestManager` field to Tox struct; initialized during `createToxInstance`; `receiveFriendRequest` routes incoming requests through RequestManager; added `RequestManager()` getter method; cleanup in `Kill()`; comprehensive integration tests added in `request_manager_integration_test.go`
- [x] med — **Missing input validation** — No length limits on Name, StatusMessage, or friend request Message fields, allowing unbounded memory allocation (`friend.go:81-110`, `request.go:24-42`) — **RESOLVED**: Added `MaxNameLength` (128), `MaxStatusMessageLength` (1007), `MaxFriendRequestMessageLength` (1016) constants per Tox spec; `SetName()` and `SetStatusMessage()` now return errors and validate length; `NewRequest()` validates message length; added `ErrNameTooLong`, `ErrStatusMessageTooLong`, `ErrFriendRequestMessageTooLong` sentinel errors; comprehensive tests added
- [x] low — **Incomplete test coverage** — Only 52.6% coverage (target: 65%); missing tests for Request encryption/decryption, RequestManager operations, and error paths (`friend_test.go:1-178`) — **RESOLVED**: Added comprehensive tests for Request.Encrypt(), DecryptRequest(), all RequestManager operations (AddRequest, AcceptRequest, RejectRequest, SetHandler, GetPendingRequests), time provider patterns, and error paths; coverage improved from 52.6% to 93.0%
- [x] low — **Missing doc.go** — Package documentation exists in friend.go but no dedicated doc.go file for package-level overview — **RESOLVED**: Created comprehensive doc.go with overview, FriendInfo usage, friend request handling, RequestManager operations, deterministic testing patterns, thread safety notes, integration points, and C bindings documentation
- [x] low — **Status type name collision** — friend.Status type name may conflict with similar status types in other packages; consider more specific naming like FriendStatus to match toxcore.go convention (`friend.go:20`) — **RESOLVED**: Renamed `Status` to `FriendStatus` and all related constants (`StatusNone` → `FriendStatusNone`, etc.) to prevent namespace collision and match toxcore.go naming conventions
- [x] low — **Logging inconsistency** — SetStatusMessage (line 108) lacks structured logging while other setters have comprehensive logging (`friend.go:108-110`) — **RESOLVED**: Added logrus.WithFields structured logging to SetStatusMessage consistent with SetName
- [x] low — **Unused recipientPublicKey parameter** — NewRequest accepts recipientPublicKey parameter but never uses it in Request struct or logic (`request.go:24`) — **N/A (by design)**: The recipientPublicKey parameter is intentionally used only for structured logging context (lines 79, 87, 103, 118); the Request struct represents an outgoing message before encryption, and the actual recipient key is provided separately to Encrypt(); storing it in Request would duplicate information and couple the request to a specific recipient before encryption

## Test Coverage
93.0% (target: 65%) ✅

**Covered test areas:**
- FriendInfo.Marshal() and FriendInfo.Unmarshal() serialization (100% coverage)
- Request.Marshal() and Request.Unmarshal() serialization (100% coverage)
- UnmarshalFriendInfo() and UnmarshalRequest() convenience functions (100% coverage)
- Invalid data handling for both FriendInfo and Request unmarshal
- Round-trip serialization validation
- Request.Encrypt() and DecryptRequest() cryptographic operations (100% coverage)
- RequestManager.AddRequest() including duplicate handling (100% coverage)
- RequestManager.AcceptRequest() and RejectRequest() (100% coverage)
- RequestManager.SetHandler() callback flow (100% coverage)
- Error path testing for invalid packets and encryption failures (100% coverage)
- TimeProvider deterministic testing patterns (100% coverage)

## Integration Status
**Critical integration gaps identified:**
- The friend/ package defines a FriendInfo type that is separate from toxcore.Friend (toxcore.go:1736) which has slightly different fields (adds IsTyping field, uses FriendStatus instead of Status enum). This is now intentional to allow independent evolution.
- ~~RequestManager has zero integration points - not instantiated or used anywhere in the codebase.~~ — **DONE**: RequestManager integrated into Tox struct with full lifecycle management
- No registration in system initialization or packet handlers.
- No serialization support means FriendInfo/Request state cannot be persisted in savedata.
- Friend package does not interact with DHT, transport, or messaging layers despite being foundational to peer-to-peer communication.

**Expected integration points (missing):**
- ~~Tox struct should have requestManager field using friend.RequestManager~~ — **DONE**: Added requestManager field initialized in createToxInstance()
- ~~Friend request packets should be routed through RequestManager in Tox.Iterate()~~ — **DONE**: receiveFriendRequest() routes through RequestManager
- ~~FriendInfo state should serialize/deserialize for savedata support~~ — **DONE**: Marshal/Unmarshal methods implemented
- Connection status changes should trigger DHT routing updates

## Recommendations
1. **RESOLVED**: Duplicate Friend type addressed by renaming friend.Friend to friend.FriendInfo
2. **RESOLVED**: sync.RWMutex added to RequestManager
3. **RESOLVED**: Time provider pattern implemented for deterministic testing
4. ~~**HIGH**: Implement Marshal/Unmarshal methods for FriendInfo and Request types to support savedata persistence~~ — **DONE**: Added Marshal/Unmarshal methods to both FriendInfo and Request types using JSON serialization; added convenience functions UnmarshalFriendInfo() and UnmarshalRequest(); comprehensive tests added; coverage improved to 52.6%
5. **MED**: Add comprehensive error logging with logrus.WithFields on all error return paths in request.go
6. ~~**MED**: Implement input validation with length limits (Name: 128 bytes, StatusMessage: 1007 bytes per Tox spec, Request message: 1016 bytes)~~ — **DONE**: Added `MaxNameLength`, `MaxStatusMessageLength`, `MaxFriendRequestMessageLength` constants; `SetName()`, `SetStatusMessage()` return errors; `NewRequest()` validates message length; sentinel errors and tests added
7. ~~**MED**: Integrate RequestManager into Tox struct and wire up packet handling in main iteration loop~~ — **DONE**: Added `requestManager` field to Tox struct; initialized in `createToxInstance`; `receiveFriendRequest` routes through RequestManager; `RequestManager()` getter added; cleanup in `Kill()`; integration tests in `request_manager_integration_test.go`
8. ~~**LOW**: Increase test coverage to 65% target by adding tests for encryption, RequestManager operations, and all error paths~~ — **DONE**: Added comprehensive tests for Request.Encrypt(), DecryptRequest(), all RequestManager operations; coverage improved from 52.6% to 93.0%
9. ~~**LOW**: Create doc.go with comprehensive package-level documentation and usage examples~~ — **DONE**: Created friend/doc.go with overview, FriendInfo usage, friend request handling, RequestManager operations, deterministic testing patterns, thread safety notes, integration points, and C bindings documentation
10. ~~**LOW**: Add structured logging to SetStatusMessage for consistency with other setters~~ — **DONE**: Added logrus.WithFields logging to SetStatusMessage
