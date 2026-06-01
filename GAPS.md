# Implementation Gaps — 2026-05-31 (updated 2026-06-01)

## Group Broadcast API Thread-Safety Gap — RESOLVED
- Stated Goal: Production-grade concurrent operation for messaging/group workflows with goroutine-safe behavior.
- Root Cause Found: `Leave`, `KickPeer`, `SetPeerRole`, `SetName`, `SetPrivacy`, `SetSelfName` each held
  `g.mu.Lock()` (write lock) while calling `broadcastGroupUpdateTyped`, which internally calls
  `collectOnlinePeerJobs` → `g.mu.RLock()`. Go's `sync.RWMutex` does not support recursive locking;
  the second lock acquisition deadlocked the goroutine.
  `SendMessage` and `HandlePeerListRequest` held `g.mu.RLock()` and then called the same broadcast path —
  a pending writer would block the second `RLock()` while waiting for the first to release, causing a
  three-way deadlock.
- Fix Applied (2026-06-01): All eight functions now release `g.mu` before invoking
  `broadcastGroupUpdateTyped`. State mutations are performed under the lock first; the captured values
  are used when building the broadcast payload after unlocking. `go test -race ./group/...` passes.

## Callback Isolation Gap (Peer Discovery) — RESOLVED
- Stated Goal: Reliable callback-driven group behavior without destabilizing core runtime.
- Fix Applied (prior session): `OnPeerDiscovered` callback at `group/chat.go:1990` routes through
  `safeInvokeCallback`, which wraps the call in a goroutine with `recover`. Panics are logged and
  do not propagate to the caller.

## Transport Handler Hardening Gap (Noise) — RESOLVED
- Stated Goal: Hardened encrypted transport pipeline under untrusted packet ingress.
- Fix Applied (prior session): Handler dispatch in `handleEncryptedPacket` at
  `transport/noise_transport.go:847` is wrapped in a goroutine with a `defer recover()` that logs
  the panic and absorbs it, preventing process termination.

## Auditability Gap (Execution Tooling in Session) — RESOLVED
- Stated Goal: CI-grade validation workflow includes go test -race and go vet baselines.
- Fix Applied (2026-06-01): `go vet ./...` passes with zero findings. `go test -race -short ./group/...`
  passes. The previous session's terminal-execution failures have been resolved.
