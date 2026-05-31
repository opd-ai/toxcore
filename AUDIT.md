# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-31

## Project Profile
toxcore-go is a pure Go implementation of the Tox peer-to-peer encrypted messaging protocol. The stated target users are developers building privacy-focused communication apps and contributors to the Tox ecosystem. The critical paths are the public toxcore facade, friend and message delivery, async offline messaging, file transfers, transport setup, DHT/bootstrap, and the group and AV surfaces.

## Audit Scope
Audited packages in this pass: toxcore, async, file, messaging, transport, group, dht, crypto, and real.

Baseline tooling requested by the prompt could not be executed through the workspace-bound command runners in this session, so full go-stats-generator metrics, go test -race, and go vet outputs were not collected here.

## Coverage Log
| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| toxcore | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| real | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary
| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| Pure Go Tox messaging and transport stack | ⚠️ | [TCP-only transport gap](GAPS.md#tcp-only-transport-gap) |
| Friend management and typing/status propagation | ⚠️ | [Typing notifications can be silently dropped](#high), [friend snapshot aliasing](#low) |
| File transfers with bounded, correct chunk progression | ⚠️ | [file-chunk validation bug](#medium), [transfer counter race](#medium) |
| Async offline messaging with forward secrecy | ⚠️ | No blocking bug found in the inspected slice, but full repository coverage was not completed in this session |

## Findings

### HIGH
- [ ] DHT reads are not consistently synchronized before use in public APIs — [toxcore_network.go#L542](toxcore_network.go#L542), [toxcore_network.go#L693](toxcore_network.go#L693) — concurrency / nil-safety — `DiscoverRelayServers` and `resolveFriendIDFromAddress` read `t.dht` without taking `dhtMutex`, while `Kill()` clears the field under lock. A shutdown race can therefore turn the second dereference into a nil-pointer panic or produce an inconsistent routing-table snapshot during file-transfer address resolution. **Remediation:** snapshot `t.dht` under `dhtMutex.RLock()` at the top of each function, then use the local snapshot only; validate with `go test -race ./...` and a targeted shutdown test for relay discovery and file-transfer callbacks.
- [ ] Typing notifications silently succeed when no UDP transport is present — [toxcore.go#L1102](toxcore.go#L1102) — API / error handling — `SetTyping` returns nil even when `t.udpTransport` is nil, so TCP-only or transport-less configurations report success while sending nothing. That makes the public typing API lie to callers and drops the notification without any error signal. **Remediation:** return an explicit error from `sendTypingPacket` when UDP transport is unavailable, or route typing packets through the active transport abstraction instead; validate with `go test ./...` plus a focused test for `SetTyping` on a no-UDP instance.

### MEDIUM
- [ ] FileSendChunk accepts an out-of-range position and can mark a transfer advanced past its declared size — [toxcore_file.go#L200](toxcore_file.go#L200), [toxcore_file.go#L215](toxcore_file.go#L215), [toxcore_file.go#L219](toxcore_file.go#L219) — logic / protocol consistency — The guard only rejects `position > fileSize`, so `position == fileSize` with non-empty data is allowed. `updateTransferProgress` then writes `Transferred = position + len(data)` without checking the file boundary, so the sender can believe the transfer completed even though the receiver will reject the chunk because no bytes remain. **Remediation:** reject any chunk where `position >= fileSize` or `len(data) > fileSize-position`, and clamp progress updates to the transfer’s declared size; validate with a dedicated file-transfer test plus `go test ./file ./...`.
- [ ] Transfer progress is written without holding the transfer mutex — [toxcore_file.go#L215](toxcore_file.go#L215), [file/transfer.go#L977](file/transfer.go#L977) — concurrency — `updateTransferProgress` mutates `transfer.Transferred` directly even though the `Transfer` type exposes `GetTransferred` and related methods as mutex-protected. Concurrent readers can therefore race with sender-side progress updates during file transfer monitoring. **Remediation:** add a mutex-protected setter on `Transfer` or lock `transfer.mu` around the assignment in `updateTransferProgress`; validate with `go test -race ./file ./...`.
- [ ] Friend-to-address resolution uses an unguarded DHT snapshot in a second public path — [toxcore_network.go#L542](toxcore_network.go#L542) — concurrency / nil-safety — `DiscoverRelayServers` checks `t.dht` for nil and then calls `t.dht.QueryRelays(...)` without first copying the pointer under `dhtMutex`. A concurrent `Kill()` can therefore null the field between the check and the method call. **Remediation:** capture the DHT pointer under lock once and use the local copy; validate with `go test -race ./...` and a shutdown regression test.

### LOW
- [ ] GetFriends does not fully deep-copy friend user data — [toxcore_friends.go#L247](toxcore_friends.go#L247) — data aliasing / API contract — The method clones the Friend struct, but the `UserData` interface field is copied by reference. Any caller that stores a mutable pointer or map in `UserData` can still mutate shared state through the returned snapshot, which contradicts the method’s “deep copy” comment. **Remediation:** either document that `UserData` is shallow-copied or clone it via a user-supplied copier hook; validate with a focused unit test around `GetFriends`.

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total functions | Not collected in this session |
| Functions above complexity 15 | Not collected in this session |
| Avg cyclomatic complexity | Not collected in this session |
| Doc coverage | Not collected in this session |
| Duplication ratio | Not collected in this session |
| Test pass rate | Not run in this session |
| go vet warnings | Not run in this session |

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|-----------------|
| Panic in dht/mdns_discovery.go init() | The panic is explicitly documented as a fail-fast invariant for compile-time constants, not a runtime error path. |
| Panic in transport/nat.go init() | The panic is also documented as an invariant check for a constant fallback address, so it is intentional initialization behavior. |
| Async queueing when pre-keys are unavailable | The queueing path is intentional and covered by the async manager design; no bug was confirmed in the inspected slice. |

## Remaining Scope
| Package | Status | Notes |
|---------|--------|-------|
| av | Not yet fully audited | Resume with audio/video call setup, RTP handling, and callback wiring. |
| bootstrap | Not yet fully audited | Inspect bootstrap, relay, and discovery logic. |
| capi | Not yet fully audited | Verify cgo export paths and error translation. |
| friend | Not yet fully audited | Check friend store, request manager, and sharded state behavior. |
| interfaces | Not yet fully audited | Review interface contracts and adapter implementations. |
| noise | Not yet fully audited | Inspect handshake/session lifecycle and nonce handling. |
| ratchet | Not yet fully audited | Review double-ratchet state transitions and persistence. |
| toxnet | Not yet fully audited | Verify stream/datagram network abstractions. |
| examples, cmd, scripts, simulation, testnet | Not yet fully audited | Outside the core runtime path, but still part of the repository-wide pass. |
