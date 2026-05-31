# Implementation Gaps — 2026-05-31

## TCP-only transport gap
- **Stated Goal**: The project describes multi-network transport and proxy-capable TCP support in the README and configuration docs.
- **Current State**: Friend messages, file transfers, typing notifications, and packet sends in the toxcore facade still go through UDP-only helpers and return or silently drop when `udpTransport` is unavailable.
- **Impact**: A TCP-only or UDP-disabled deployment cannot actually exchange ordinary friend traffic, so the advertised transport flexibility is incomplete for user-facing messaging.
- **Closing the Gap**: Route message/file/typing send paths through the active transport abstraction instead of hard-coding `udpTransport`, or add a TCP fallback path that exercises the already-initialized TCP transport.

## Friend snapshot is not fully isolated
- **Stated Goal**: `GetFriends` says it returns a deep copy to prevent external modification.
- **Current State**: The copied `Friend` values still share the `UserData` interface payload by reference.
- **Impact**: Callers can still mutate shared state if the payload is a pointer, map, or slice, so the snapshot is not fully isolated.
- **Closing the Gap**: Either document that `UserData` is shallow-copied, or add an explicit cloning strategy for user data when deep isolation is required.
