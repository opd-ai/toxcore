# Implementation Gaps — 2026-05-31

## Group Broadcast API Thread-Safety Gap
- Stated Goal: Production-grade concurrent operation for messaging/group workflows with goroutine-safe behavior.
- Current State: Exported broadcast entry point [BroadcastGroupUpdateTypedWithOptions](group/chat.go#L1630) does not enforce locking, while peer map iteration occurs in [collectOnlinePeerJobs](group/chat.go#L1654) and peer map mutation occurs in handlers like [HandlePeerAnnounce](group/chat.go#L1953).
- Impact: Concurrent callers can trigger runtime panic from concurrent map access or race bugs in active group sessions.
- Closing the Gap:
  - Enforce synchronization in broadcast read paths, ideally by copying peer targets under lock then sending outside lock.
  - Add race-focused tests around simultaneous peer announcement and broadcast operations.

## Callback Isolation Gap (Peer Discovery)
- Stated Goal: Reliable callback-driven group behavior without destabilizing core runtime.
- Current State: User callback registered by [OnPeerDiscovered](group/chat.go#L1143) is invoked directly via goroutine at [group/chat.go](group/chat.go#L1966) without panic containment.
- Impact: A panic in application callback logic can terminate the process.
- Closing the Gap:
  - Route peer discovery callback invocation through the package-safe callback wrapper with recover.
  - Add a regression test for panic containment and error logging consistency.

## Transport Handler Hardening Gap (Noise)
- Stated Goal: Hardened encrypted transport pipeline under untrusted packet ingress.
- Current State: Packet handlers registered through [RegisterHandler](transport/noise_transport.go#L487) are invoked from [handleEncryptedPacket](transport/noise_transport.go#L817) with direct goroutine dispatch at [transport/noise_transport.go](transport/noise_transport.go#L847) and no recover guard.
- Impact: Panic in any registered handler can terminate process while handling network traffic.
- Closing the Gap:
  - Add recover/log boundary around handler dispatch in Noise transport.
  - Add targeted test with panic-inducing handler.

## Auditability Gap (Execution Tooling in Session)
- Stated Goal: CI-grade validation workflow includes go-stats-generator metrics, go test -race, and go vet baselines.
- Current State: Terminal execution was unavailable in this session due workspace ENOPRO failures on run_in_terminal; required baseline commands could not be executed here.
- Impact: Metrics and dynamic validation evidence are incomplete in this specific run.
- Closing the Gap:
  - Re-run full baseline commands in an environment where terminal execution is functional.
  - Merge resulting quantitative metrics and test/vet outputs into the audit report for complete evidence.
