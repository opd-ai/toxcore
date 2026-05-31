# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-31

## Project Profile
Pure-Go implementation of the Tox protocol for decentralized encrypted messaging, group chat, file transfer, AV calling, and async offline messaging.

Target users:
- Application developers building privacy-focused messaging clients
- Contributors to the Tox ecosystem

Deployment model:
- Long-running networked peer process
- Handles untrusted network input across DHT, transport, messaging, and group subsystems

Critical paths (from README claims and code ownership):
- Root toxcore API orchestration
- transport package (packet IO, Noise, NAT)
- dht package (routing/bootstrap/discovery)
- group package (group state, peer discovery, broadcast)
- async package (offline messaging, key lifecycle)
- crypto package (key handling and encryption)

Trust boundaries:
- Network packet ingress via transport handlers and DHT listeners
- Peer-provided metadata (names, message payloads, peer announce/list entries)
- Local persistence inputs (savedata, WAL recovery, keystore files)

## Audit Scope
Requested scope was full repository and all functions/packages.

What was completed this session:
- README and go.mod claim/dependency extraction
- Repository-wide static pattern sweeps for security/resource/concurrency smells
- Manual line-by-line validation of high-risk candidates in:
  - [group/chat.go](group/chat.go)
  - [transport/noise_transport.go](transport/noise_transport.go)
  - [transport/reuseport.go](transport/reuseport.go)
  - [crypto/keystore.go](crypto/keystore.go)
  - [dht/skademlia.go](dht/skademlia.go)
  - [dht/routing.go](dht/routing.go)
  - [transport/lru_session_cache.go](transport/lru_session_cache.go)
  - [messaging/priority_queue.go](messaging/priority_queue.go)
  - [file/transfer.go](file/transfer.go)

Tooling limitation:
- Terminal execution is unavailable in this session due ENOPRO on run_in_terminal calls rooted at workspace path, so the following required commands could not be executed here:
  - go-stats-generator baseline
  - go test -race ./...
  - go vet ./...

## Coverage Log
Legend: ✅ completed, 🟡 partial in this session, ⬜ not completed in this session.

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| toxcore (root) | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 |
| async | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| av | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| bootstrap | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| capi | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| cmd | 🟡 | 🟡 | 🟡 | 🟡 | ⬜ | 🟡 | ⬜ | ⬜ | 🟡 |
| crypto | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 |
| dht | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 |
| factory | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| file | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | ⬜ | 🟡 |
| friend | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| interfaces | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| limits | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| messaging | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | ⬜ | 🟡 |
| noise | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| real | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| simulation | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| testnet | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| toxnet | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 🟡 | ✅ | ✅ |

## Goal-Achievement Summary
| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| Stable, goroutine-safe callback/event architecture for group messaging | ⚠️ | H-01, H-02 |
| Robust encrypted transport handling under untrusted network input | ⚠️ | M-01 |
| Production-safe concurrent operation across messaging/group paths | ⚠️ | H-01, L-01 |

## Findings

### HIGH
- [x] H-01 Concurrent map read/write risk in exported group broadcast API — [group/chat.go](group/chat.go#L1630) and [group/chat.go](group/chat.go#L1654) and [group/chat.go](group/chat.go#L1953) — Concurrency
  - Concrete consequence: fatal runtime panic with concurrent map iteration/write or race corruption under concurrent use.
  - Concrete data flow / code path:
    - Caller invokes [BroadcastGroupUpdateTypedWithOptions](group/chat.go#L1630) with no lock acquisition.
    - Broadcast path iterates g.Peers in [collectOnlinePeerJobs](group/chat.go#L1654).
    - Concurrent write path exists in [HandlePeerAnnounce](group/chat.go#L1953) (writes g.Peers under lock), and external callers can invoke APIs concurrently.
  - Remediation:
    - Acquire g.mu.RLock around peer map reads in collectOnlinePeerJobs and validatePeerForBroadcast, or snapshot peers under lock then release before network sends.
    - If BroadcastGroupUpdateTypedWithOptions is intended internal-only, make it unexported; otherwise document and enforce thread safety in code.
    - Validation command: go test -race ./group -run Broadcast

- [x] H-02 Unrecovered panic path in user-supplied peer discovery callback — [group/chat.go](group/chat.go#L1143) and [group/chat.go](group/chat.go#L1966) — API/Concurrency
  - Concrete consequence: process-wide crash if application callback panics.
  - Concrete data flow / code path:
    - Application registers callback via [OnPeerDiscovered](group/chat.go#L1143).
    - Network event triggers [HandlePeerAnnounce](group/chat.go#L1937).
    - Callback invoked in goroutine at [group/chat.go](group/chat.go#L1966) without recovery wrapper.
  - Remediation:
    - Replace direct goroutine call with safeInvokeCallback wrapper pattern already used elsewhere in this package.
    - Add regression test asserting callback panic does not crash and logs recovery.
    - Validation command: go test -race ./group -run PeerDiscovered

### MEDIUM
- [x] M-01 Unrecovered panic path in registered Noise packet handler — [transport/noise_transport.go](transport/noise_transport.go#L487) and [transport/noise_transport.go](transport/noise_transport.go#L847) — Error handling/Concurrency
  - Concrete consequence: process crash when a registered handler panics during processing of remote traffic.
  - Concrete data flow / code path:
    - Handler is installed via [RegisterHandler](transport/noise_transport.go#L487).
    - Untrusted remote packet reaches [handleEncryptedPacket](transport/noise_transport.go#L817).
    - Handler invoked via goroutine in [transport/noise_transport.go](transport/noise_transport.go#L847) with no recover guard.
  - Remediation:
    - Wrap handler invocation in defer/recover and structured error logging, matching worker pool hardening style.
    - Add test with intentionally panicking handler to assert containment.
    - Validation command: go test -race ./transport -run NoiseTransport

### LOW
- [x] L-01 Global time provider mutable without synchronization — [group/chat.go](group/chat.go#L164) and [group/chat.go](group/chat.go#L173) and [group/chat.go](group/chat.go#L595) — Initialization/Concurrency
  - Concrete consequence: data race when SetDefaultTimeProvider is called concurrently with Create/Join or other reads.
  - Reachability note: likely in tests and advanced runtime instrumentation; impact is low but real under race detector.
  - Remediation:
    - Guard defaultTimeProvider with sync.RWMutex or atomic.Pointer pattern.
    - Keep API but serialize reads/writes through helpers.
    - Validation command: go test -race ./group

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total functions | N/A (go-stats-generator not runnable in this session) |
| Functions above complexity 15 | N/A |
| Avg cyclomatic complexity | N/A |
| Doc coverage | N/A |
| Duplication ratio | N/A |
| Test pass rate | N/A (terminal execution unavailable) |
| go vet warnings | N/A (terminal execution unavailable) |
| Workspace diagnostics | No editor diagnostics reported by get_errors |

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|-----------------|
| Type assertions in LRU/list-backed caches in dht and transport | Container invariants ensure stored element types; no untrusted dynamic type path found |
| Panic in init for NAT/mDNS pre-resolved constants | Explicit invariant on compile-time constants; not user-input reachable |
| C-memory leak wording in secure allocator comments | Intentionally documented tradeoff; explicitly acknowledged pattern |
| Queue Close broadcasting condition variable without lock | Allowed by sync.Cond semantics; no correctness break confirmed from this alone |

## Remaining Scope (if session ended before completion)
| Package | Status | Notes |
|---------|--------|-------|
| async | Not yet audited | Full Phase 3b-3k still pending |
| av | Not yet audited | Full Phase 3b-3k still pending |
| bootstrap | Not yet audited | Full Phase 3b-3k still pending |
| capi | Not yet audited | Full Phase 3b-3k still pending |
| factory | Not yet audited | Full Phase 3b-3k still pending |
| friend | Not yet audited | Full Phase 3b-3k still pending |
| interfaces | Not yet audited | Full Phase 3b-3k still pending |
| limits | Not yet audited | Full Phase 3b-3k still pending |
| noise | Not yet audited | Full Phase 3b-3k still pending |
| real | Not yet audited | Full Phase 3b-3k still pending |
| simulation | Not yet audited | Full Phase 3b-3k still pending |
| testnet | Not yet audited | Full Phase 3b-3k still pending |
| toxnet | Not yet audited | Full Phase 3b-3k still pending |
| root toxcore package | Partial | Many files/functions remain beyond sampled high-risk paths |
| cmd | Partial | Security/process handling reviewed only partially |
| crypto | Partial | Keystore and secure memory paths sampled; broad pass pending |
| dht | Partial | Cache and init paths sampled; routing/bootstrap full pass pending |
| file | Partial | Transfer and path validation sampled; full pass pending |
| messaging | Partial | Priority queue sampled; remaining manager/state machine pending |
| transport | Partial | Noise/reuseport sampled deeply; other modules pending |
