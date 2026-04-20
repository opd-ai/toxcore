# IMPLEMENTATION GAP AUDIT — 2026-04-20

## Project Architecture Overview

**Project**: `github.com/opd-ai/toxcore` — a pure Go implementation of the Tox
peer-to-peer encrypted messaging protocol (Go 1.25.0, toolchain go1.25.8).

**Stated goals (from `README.md`, `doc.go`, `ROADMAP.md`, `PLAN.md`):**
- DHT-based peer discovery (k-buckets, iterative lookup, LAN/mDNS).
- Friend management, 1-to-1 encrypted messaging, group chat.
- Chunked file transfers with pause/resume/cancel.
- ToxAV: Opus audio (`opd-ai/magnum`) + VP8 video (`opd-ai/vp8`) over RTP.
- Asynchronous offline messaging with WAL persistence, pre-key forward secrecy,
  epoch-based pseudonym rotation, identity obfuscation, message padding.
- Multi-network transport: IPv4/IPv6 UDP+TCP, Tor `.onion` (Listen+Dial),
  I2P `.b32.i2p` (Listen+Dial), Lokinet `.loki` (Dial-only — documented),
  Nym `.nym` (Dial-only — documented).
- Noise-IK + XX handshakes via `flynn/noise` for forward secrecy and KCI
  resistance; protocol-version negotiation with legacy Tox.
- NAT traversal (STUN, UPnP, NAT-PMP, TCP relay fallback).
- Pure-Go core (no cgo); optional C API in `capi/` (cgo).
- `net.Conn`/`net.Listener`/`net.PacketConn` adapters in `toxnet/`.

**Architecture (25 packages, 237 non-test files, 41,024 LOC):**

| Layer | Packages | Responsibility |
|-------|----------|----------------|
| Facade | `toxcore` (root) | Public Go API, lifecycle, callback wiring |
| Crypto | `crypto`, `noise` | Curve25519/Ed25519/ChaCha20-Poly1305, Noise-IK/XX |
| DHT | `dht`, `bootstrap`, `bootstrap/nodes` | Kademlia routing, bootstrap |
| Transport | `transport` (41 files) | UDP/TCP/Noise/Tor/I2P/Lokinet/Nym/NAT |
| Net adapters | `toxnet` | `net.Conn`/`net.Listener`/`net.PacketConn` |
| Messaging | `messaging`, `friend`, `group`, `file` | Conversational features |
| Async | `async` (26 files) | Offline messaging, pre-keys, obfuscation, WAL |
| ToxAV | `av`, `av/audio`, `av/video`, `av/rtp` | Audio/video calling |
| C API | `capi` | libtoxcore-compatible C exports (cgo) |
| Packet delivery | `interfaces`, `factory`, `real`, `simulation` | Pluggable delivery (real vs sim) |
| Tooling/util | `cmd/gen-bootstrap-nodes`, `examples/*`, `testnet`, `limits` | Dev helpers |

## Phase 1 Research Notes

- **Issue #43 (TokTok maintainer)** — open: requests qTox CI/CD integration once
  toxcore-go is production-ready; cited by `PLAN.md` and `ROADMAP.md` as the
  primary external milestone awaiting VP8 P-frame parity. Not blocking core.
- **No open code-bug issues** beyond #43 are referenced by `PLAN.md`.
- **PLAN.md** and **ROADMAP.md** explicitly track the VP8 P-frame gap and the
  Lokinet/Nym Listen limitations as known, intentional, and externally blocked
  (immature SDKs / upstream library scope).

## Phase 2 Baseline Results

- `go build ./...` — **clean** (no errors).
- `go vet ./...` — **clean** (no diagnostics).
- `go-stats-generator analyze . --skip-tests`:
  - 41,024 LOC, 1,136 functions, 2,833 methods, 406 structs, 37 interfaces, 25 packages.
  - Documentation coverage **93.1%** overall (98.7% functions, 92.2% types,
    92.0% methods, 100% packages).
  - **TODO comments: 5** (all in one file: `av/video/encoder_cgo.go`).
  - **FIXME / HACK / XXX / STUB comments: 0**.
  - Stale annotations: 0.
  - Avg complexity 3.5; no functions >10.
  - Duplication ratio 0.58% (mostly examples).

## Gap Summary

| Category | Count | Critical | High | Medium | Low |
|----------|-------|----------|------|--------|-----|
| Stubs/TODOs | 1 | 0 | 1 | 0 | 0 |
| Dead Code | 0 | 0 | 0 | 0 | 0 |
| Partially Wired | 0 | 0 | 0 | 0 | 0 |
| Interface Gaps | 2 | 0 | 0 | 1 | 1 |
| Dependency Gaps | 0 | 0 | 0 | 0 | 0 |
| **Total** | **3** | **0** | **1** | **1** | **1** |

> The codebase is unusually "clean" relative to its size: zero
> `panic("not implemented")`, zero FIXME/HACK/XXX, no `go vet` findings, build
> succeeds. The single concrete code-level gap is `LibVPXEncoder`, which is
> compiled only under the opt-in `-tags libvpx` build tag and is documented as
> blocked-on-upstream in both `ROADMAP.md` and `PLAN.md`.

## Implementation Completeness by Package

Source: `/tmp/gap-audit-metrics.json` (now removed). "Stubs" counts production
functions with placeholder bodies that would prevent the package from
fulfilling its documented role. "Dead" counts production-only functions found
to be unreachable from any in-repo caller.

| Package | Files | Functions | Stubs | Dead | Doc % | Notes |
|---------|------:|----------:|------:|-----:|------:|-------|
| `async` | 26 | 479 | 0 | 0 | ✅ | WAL/forward-secrecy fully wired |
| `av` | 9 | 210 | 0 | 0 | ✅ | Manager + signaling complete |
| `av/audio` | 5 | 112 | 0 | 0 | ✅ | Opus via magnum |
| `av/rtp` | 4 | 73 | 0 | 0 | ✅ | RTP packetisation complete |
| `av/video` | 9 | 169 | **6** | 0 | ✅ | `LibVPXEncoder` placeholder under `-tags libvpx` only |
| `bootstrap` | 3 | 27 | 0 | 0 | ✅ | |
| `capi` | (root `main`) | — | 0 | 0 | ✅ | C exports cover ~79% of libtoxcore |
| `cmd/gen-bootstrap-nodes` | — | — | 0 | 0 | ✅ | |
| `crypto` | 16 | 95 | 0 | 0 | ✅ | |
| `dht` | 18 | 417 | 0 | 0 | ✅ | |
| `factory` | 2 | 19 | 0 | 0 | ✅ | Wires `interfaces`+`real`+`simulation` |
| `file` | 3 | 68 | 0 | 0 | ✅ | |
| `friend` | 5 | 66 | 0 | 0 | ✅ | |
| `group` | 4 | 131 | 0 | 0 | ✅ | |
| `interfaces` | 2 | 1 | 0 | 0 | ✅ | Defines `IPacketDelivery`/`INetworkTransport` |
| `limits` | 2 | 5 | 0 | 0 | ✅ | |
| `messaging` | 3 | 78 | 0 | 0 | ✅ | |
| `noise` | 3 | 60 | 0 | 0 | ✅ | |
| `real` | 2 | 23 | 0 | 0 | ✅ | Single concrete `IPacketDelivery` impl |
| `simulation` | 2 | 12 | 0 | 0 | ✅ | Single `IPacketDelivery` simulation impl |
| `toxcore` (root) | 15 | 333 | 0 | 0 | ✅ | API facade |
| `toxnet` | 10 | 156 | 0 | 0 | ✅ | net.Conn/Listener adapters |
| `transport` | 41 | 732 | 0 | 0 | ✅ | Includes documented Listen-not-supported on Lokinet/Nym |
| `bootstrap/nodes` | 1 | 0 | 0 | 0 | ✅ | Generated data |
| `examples/*` | 28 | — | 0 | 0 | n/a | Excluded from production gap counting |

(Test coverage % is not enforced per package by CI; the project tracks
~52.8% test-to-source file ratio with 206 test files.)

## Findings

### CRITICAL
*(none)*

### HIGH
- [ ] **`LibVPXEncoder` is a placeholder** —
      `av/video/encoder_cgo.go:60,92,109,121,132` — Five `TODO` comments
      indicate that the libvpx-backed encoder, which is the *only*
      `Encoder` produced by `NewDefaultEncoder()` under the
      `//go:build cgo && libvpx` tag (`encoder_cgo.go:140-142`), has never
      been wired to its underlying library:
      - `NewLibVPXEncoder` (line 52) skips `vpx.CodecEncInitVer`.
      - `Encode` (line 87) returns
        `fmt.Errorf("libvpx encoding not yet implemented…")`.
      - `SetBitRate` (line 107), `SetKeyFrameInterval` (line 120),
        and `Close` (line 131) are no-ops.
      - **Stated goal blocked**: `README.md` claims "Video Calling: Video
        transmission with configurable quality" with VP8; `ROADMAP.md`
        lists "VP8 P-frames" as the **Priority 1** goal-achievement gap;
        `PLAN.md` Step 2 calls this out as `BLOCKED`. Any user invoking a
        video call after `go build -tags libvpx ./...` will see every
        `Encode` call fail at runtime, silently downgrading the stated
        "P-frame support" promise to a hard error.
      - **Remediation**: implement the full `xlab/libvpx-go` integration as
        scripted in the inline TODO at line 60. Specifically:
        1. Add `github.com/xlab/libvpx-go` to `go.mod` (will be CGo-only
           and require libvpx system headers — gated by the same build
           tag).
        2. In `NewLibVPXEncoder`, allocate `vpx.CodecCtx`, fill
           `vpx.CodecEncCfg` (`GW`/`GH`/`RcTargetBitrate`/`GTimebase`/
           `RcEndUsage = vpx.RcModeVBR`), and call
           `vpx.CodecEncInitVer`. Wrap any failure with
           `fmt.Errorf("libvpx init: %w", err)`.
        3. In `Encode`, copy `frame.Y`/`U`/`V` planes into a
           `vpx.Image`, call `vpx.CodecEncode` with
           `vpx.EFlagForceKf` when `e.keyFrame` is true, then drain
           packets via `vpx.CodecGetCxData` and concatenate the
           compressed bytes.
        4. Honour `SetBitRate` by calling `vpx.CodecEncConfigSet`
           after mutating `cfg.RcTargetBitrate`.
        5. Implement `SetKeyFrameInterval` by storing the value and
           passing `vpx.EFlagForceKf` every Nth frame; or set
           `cfg.KfMaxDist`.
        6. In `Close`, call `vpx.CodecDestroy(e.encoder)` and zero
           the pointer.
      - **Validation**:
        `go build -tags libvpx ./av/video/...` &&
        `go test -tags libvpx -race ./av/video/...` &&
        a new round-trip benchmark in
        `av/video/processor_benchmark_test.go` proving an encoded P-frame
        is produced (size <10% of preceding I-frame at the same quality).
      - **Severity rationale**: HIGH (not CRITICAL) because the default
        pure-Go build (`go build ./...`) selects
        `encoder_purgo.NewDefaultEncoder` (which routes to the working
        `RealVP8Encoder`), so out-of-the-box video calling still works
        with I-frame-only output; only the explicitly opted-in
        `-tags libvpx` build is broken. PLAN.md acknowledges this.

### MEDIUM
- [ ] **`IPacketDelivery` has only the in-tree `real` and `simulation`
      implementations** — `interfaces/packet_delivery.go:45-106`,
      `factory/packet_delivery_factory.go:215-266`,
      `real/packet_delivery.go`, `simulation/packet_delivery_sim.go`. The
      4-package abstraction (`interfaces` → `factory` → {`real`,
      `simulation`}) is invoked from exactly three call sites in
      `toxcore.go` (lines 537, 1295, 1303). The factory selects the
      simulation when `udpTransport == nil` *or* when `TOX_USE_SIMULATION`
      is set; otherwise it always returns `real.NewRealPacketDelivery`.
      The interface is therefore not a true plug-in point — there is one
      production consumer and one production producer, and the simulation
      exists chiefly for unit tests. This is an "interface with one
      implementation" gap (Phase 3d): a premature abstraction whose
      ongoing cost (extra packages, dependency direction
      `toxcore → factory → real → transport`, the deprecated `GetStats`
      method scheduled for v2.0.0 removal) outweighs its current value.
      - **Stated goal**: none of the README/ROADMAP/PLAN goals depend on
        the abstraction. It does not appear in any architectural diagram.
      - **Remediation** (one of two paths; pick based on roadmap):
        1. **Keep & justify**: add at least one additional production
           implementation (for example, a `mocknet` implementation backed
           by `simulation.NewSimulatedPacketDelivery` for the
           `examples/integration_test/` harness, or a metrics-decorating
           wrapper in `real/`). Document the abstraction in
           `interfaces/doc.go` as a public extension point and surface
           `factory.PacketDeliveryFactory` from the root API so external
           consumers can supply their own `IPacketDelivery`.
        2. **Collapse**: inline `real.NewRealPacketDelivery` into
           `toxcore.go` (or a new `toxcore/internal/delivery` package)
           and move the simulation into `*_test.go` helpers. Delete
           `interfaces/`, `factory/`, `real/`, `simulation/` once their
           sole consumer is gone. This removes ~1,400 lines of indirection
           without changing public behaviour.
      - **Validation**: `go build ./...`, `go vet ./...`, and the
        existing `*_packet_delivery*_test.go` suites must all still pass.
        `IsPacketDeliverySimulation()` and `GetPacketDeliveryStats()`
        must continue to behave as before for capi consumers.
      - **Severity rationale**: MEDIUM — the abstraction does no harm at
        runtime, but it is structurally present and not connected to a
        second implementation, which is the textbook "partially wired
        component" symptom in Phase 3c/3d.

### LOW
- [ ] **`IPacketDelivery.GetStats() map[string]interface{}` is documented
      as deprecated for v2.0.0 with no removal milestone tracked** —
      `interfaces/packet_delivery.go:89-99`,
      `simulation/packet_delivery_sim.go:269-286`,
      `toxcore.go:1336-1352`. The deprecation banner names a v2.0.0
      removal target, but no `v2` branch, milestone, or issue is recorded
      in the repository. This is a tracked TODO equivalent (Phase 3a/3d).
      - **Stated goal**: none.
      - **Remediation**: open a tracking issue titled
        "Remove deprecated `IPacketDelivery.GetStats()` in v2.0.0" with
        the call sites enumerated above, or pull the deprecation
        forward to a `v1.x` minor and emit `logrus.Warn` from each
        implementation when called. No code change is required for
        v1.x correctness.
      - **Validation**: `grep -rn "GetStats()" --include='*.go' .`
        yields the same set of call sites as listed in this finding.
      - **Severity rationale**: LOW — purely a documentation/tracking
        issue; the method continues to function and has a typed
        replacement (`GetTypedStats()`) already in use.

## False Positives Considered and Rejected

| Candidate Finding | Reason Rejected |
|-------------------|----------------|
| `LokinetTransport.Listen` (`transport/lokinet_transport_impl.go:91`) returns `"Lokinet SNApp hosting not supported via SOCKS5"`. | Documented as intentional in `README.md` ("Lokinet `.loki` (dial-only)"), `ROADMAP.md` (Lokinet/Nym Listen blocked on immature SDK), and `GAPS.md` predecessor. The function fulfils its documented purpose: rejecting Listen requests with an actionable error. Phase 3f rule: minimalist behaviour matching docs is not a gap. |
| `NymTransport.Listen` returns the wrapped sentinel `ErrNymNotImplemented` (`transport/nym_transport_impl.go:18,100`). | Same as above — documented dial-only support. The exported `ErrNymNotImplemented` *is* returned in production (via `Listen`), so it is not an unused sentinel. |
| `LokinetTransport.DialPacket` and `TorTransport.DialPacket` return "UDP not supported" errors. | Documented in `transport/tor_transport_impl.go:23` and `lokinet_transport_impl.go:24`. Privacy networks use TCP; this is a deliberate protocol limit, not an implementation gap. |
| `getWindowsDiskSpace` on non-Windows (`async/storage_limits_unix.go:11`) returns an error. | Build-tag fallback for `//go:build !windows` so cross-platform builds succeed; only invoked from the Windows-tagged path (which is excluded by the `!windows` build tag). Intentional symmetric stub. |
| `getFilesystemStatistics` in `async/storage_limits_nostatfs.go` returns conservative defaults. | Build-tagged for non-statfs platforms (e.g. WASM) and explicitly documented. The project's `doc.go` lists `GOOS=js GOARCH=wasm` as a supported target. |
| 79 functions reported by `go-stats-generator` with `lines.code <= 1`. | Manual inspection: all are constructor wrappers (`NewLamportClock`, `NewSession`, `NewPacketizer`, etc.), pure return-of-a-helper, or build-tag dispatchers. None are `panic("TODO")` or empty-body stubs. |
| `panic(...)` calls under text search. | All occurrences (`group/dht_timeout_test.go:282`, `transport/worker_pool_test.go:208`, `testnet/internal/comprehensive_test.go:198,268`, `examples/friend_callbacks_demo/main.go:17,46`) are in tests or demos exercising panic-recovery code paths. No production-path placeholder panics exist. |
| `interfaces.IPacketDelivery` having a single in-tree real impl. | Counted as a MEDIUM finding above (interface with one implementation), not a false positive — but the abstraction is *internally consistent*, the simulation impl is independently meaningful (used by `factory.CreateSimulationForTesting`), and the symmetry is documented in `interfaces/doc.go`. Severity capped at MEDIUM because the code is not broken, only over-abstracted. |
| Doc-coverage shortfall (6.9% of types/methods undocumented). | Spread across many files; none are exported on a critical path that `go-stats-generator` flags as undocumented and unimplemented. Treated as code-style debt, not an implementation gap. |
| Examples duplication (31 clone pairs at 0.58%). | Cleanup is tracked as `ROADMAP.md` Priority 4 and `PLAN.md` Step 6 (already partially complete via `examples/common/`). Not a behavioural gap. |
| `dht/skademlia.go:524 GetStats()` and `av/video/rtp.go:559 GetStats()`. | Distinct exported APIs, not the deprecated `IPacketDelivery.GetStats`. Both are concrete, called, and documented. |
| Alleged "test suite timeout in toxnet" (mentioned in the prior `GAPS.md`). | Could not be reproduced at audit time because the audit explicitly skipped `go test`. Even if reproducible, it is a test-infrastructure issue, not an *implementation* gap; outside the scope of this report's intent (production code completeness). |
| Alleged "CVE-2018-25022 mitigation" gap (prior `GAPS.md`). | This is a security-audit topic, not an implementation-completeness topic; no missing function or stub references it. Belongs in a security audit, not a gap audit. |
| Alleged "Group chat cross-client interop" gap (prior `GAPS.md`). | The current implementation behaves as documented for in-network use; cross-client interop with c-toxcore is an **enhancement** beyond the README's stated goals (which never claim wire compatibility with c-toxcore for conferences). Phase 3f rule: aspirational completeness ≠ gap. |
| Alleged "Async storage node DHT discovery" gap (prior `GAPS.md`). | `ROADMAP.md` lists "Async Storage Node DHT Discovery" under **Completed Priorities**; `async/storage_discovery.go` exists with the announce/query implementation. Stale finding from the prior report. |
| `ErrRTPFailed`, `ErrInvalidBitRate`, `ErrFileNameTooLong`, `ErrRecipientOnline` flagged by stats-generator placement suggestions. | These are exported sentinel errors checked by callers (`errors.Is`) and used in tests; placement suggestions are a code-organisation hint, not unused-symbol indicators. |

## Verification Commands

```bash
go build ./...                                                # → clean
go vet ./...                                                  # → clean
go-stats-generator analyze . --skip-tests                     # → 5 TODOs, all in av/video/encoder_cgo.go
go build -tags libvpx ./av/video/...                          # → builds (non-functional encoder)
grep -rn "panic(\"not implemented\"\|panic(\"TODO\")" --include='*.go' .  # → no matches
grep -rn "TODO\|FIXME\|HACK\|XXX" --include='*.go' . | grep -v _test.go   # → 5 lines, all encoder_cgo.go
```

## Tiebreaker Application

Findings ordered per the tiebreaker (stubs on critical paths → partially wired
features → dead code → interface gaps → tracked TODOs):

1. HIGH — `LibVPXEncoder` placeholder (stub on opt-in critical path).
2. MEDIUM — `IPacketDelivery` premature abstraction (interface gap).
3. LOW — `GetStats()` deprecation tracking (tracked TODO).

There are no CRITICAL findings: every stated README/ROADMAP goal that ships
under the default `go build ./...` invocation is backed by a working,
documented, exercised implementation.
