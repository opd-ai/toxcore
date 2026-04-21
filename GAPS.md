# Implementation Gaps — 2026-04-20

Companion to `AUDIT.md`. Three gaps were found in 41,024 LOC across 25
production packages. See `AUDIT.md` for the package-by-package completeness
matrix and the false-positive rejection table.

---

## Gap 1 — `LibVPXEncoder` is a non-functional placeholder under `-tags libvpx`

- **Severity**: HIGH
- **Location**: `av/video/encoder_cgo.go:52-134` (entire file is a placeholder)
  and the factory at `av/video/encoder_cgo.go:140-142`.
- **Intended Behavior**:
  - `README.md` Features list: "ToxAV Audio/Video — Peer-to-peer calling with
    Opus audio … and VP8 video via `opd-ai/vp8`, RTP transport, adaptive
    bitrate, and jitter buffering."
  - `ROADMAP.md` Priority 1: "VP8 P-Frames" with the actionable items
    `Implement CGo-optional libvpx encoder (//go:build cgo && libvpx)`,
    `Add EncoderType config option`, `Benchmark P-frame bandwidth savings`.
  - `PLAN.md` Step 2: "CGo-Optional libvpx Encoder … Complete implementation of
    `encoder_cgo.go` (currently has TODO placeholder at line 60)".
  - `av/video/encoder_cgo.go:7`: doc comment promises "full VP8 encoding with
    P-frames" when built with `-tags libvpx`.
  - `av/video/encoder_cgo.go:140-142`: `NewDefaultEncoder` (the package-level
    factory used by `av/video/processor.go` to build the production encoder)
    delegates entirely to `NewLibVPXEncoder` under this build tag.
- **Current State**:
  - `NewLibVPXEncoder` (line 52) constructs a struct without initialising any
    libvpx state — the `vpx.CodecCtx` field is commented out (line 32) and
    no `vpx.CodecEncInitVer` call is made (TODO at line 60).
  - `Encode` (line 87) returns
    `fmt.Errorf("libvpx encoding not yet implemented: add github.com/xlab/libvpx-go dependency")`
    on every call (line 103) — there is no fallback.
  - `SetBitRate` (line 107) only mutates a Go field; no `vpx.CodecEncConfigSet`.
  - `SetKeyFrameInterval` (line 120) discards its argument (`_ = interval`).
  - `Close` (line 131) returns nil without calling `vpx.CodecDestroy`.
  - The `xlab/libvpx-go` dependency is **not** in `go.mod`.
  - 5 `TODO` comments (the only TODOs reported by `go-stats-generator` in the
    entire production tree) all live in this file: lines 60, 92, 109, 121, 132.
- **Blocked Goal**: README's "VP8 video" promise and ROADMAP Priority 1
  ("VP8 P-Frames"). External-facing impact: any consumer running
  `go build -tags libvpx ./...` and initiating a video call will receive
  encode-time errors instead of P-frames; ROADMAP completion requires this.
  qTox CI/CD integration (Issue #43) is downstream-blocked per `PLAN.md`.
- **Implementation Path**:
  1. **Dependency**: `go get github.com/xlab/libvpx-go@latest`. Verify
     `gh-advisory-database` (ecosystem `go`) before pinning. The dependency
     is CGo-only and gated by the existing `//go:build cgo && libvpx` tag, so
     the pure-Go default build is unaffected.
  2. **`NewLibVPXEncoder` (line 52)**:
     - Allocate `ctx := &vpx.CodecCtx{}` and `cfg := &vpx.CodecEncCfg{}`.
     - Populate `cfg.GW`, `cfg.GH` from `width`/`height`.
     - `cfg.RcTargetBitrate = bitRate / 1000` (kbps).
     - `cfg.GTimebase.Num = 1; cfg.GTimebase.Den = 30`.
     - `cfg.RcEndUsage = vpx.RcModeVBR`.
     - `cfg.KfMaxDist = defaultKeyframeInterval` (e.g. 30 frames).
     - Call `vpx.CodecEncConfigDefault(vpx.CodecVP8Encoder(), cfg, 0)` first,
       then override the fields above.
     - Call `vpx.CodecEncInitVer(ctx, vpx.CodecVP8Encoder(), cfg, 0,
       vpx.EncoderABIVersion)`; wrap any error with
       `fmt.Errorf("libvpx init: %w", err)`.
     - Store `ctx` and `cfg` on the struct (uncomment the `encoder` field).
  3. **`Encode` (line 87)**:
     - Wrap `frame.Y/U/V` planes into a `vpx.Image` (`vpx.ImgWrap` with
       `vpx.ImgFmtI420`).
     - `flags := vpx.EncFrameFlags(0); if e.keyFrame { flags |=
       vpx.EFlagForceKf }`.
     - Call `vpx.CodecEncode(ctx, img, pts, 1, flags, vpx.DlRealtime)`.
     - Drain compressed packets with `iter := vpx.CodecIter(nil)` and
       `for pkt := vpx.CodecGetCxData(ctx, &iter); pkt != nil; … { append }`.
     - On success, set `e.keyFrame = false` and return concatenated bytes.
  4. **`SetBitRate` (line 107)**: after mutating `e.bitRate`, set
     `e.cfg.RcTargetBitrate = bitRate / 1000` and call
     `vpx.CodecEncConfigSet(ctx, cfg)`; return wrapped error.
  5. **`SetKeyFrameInterval` (line 120)**: store the value, mirror it into
     `e.cfg.KfMaxDist`, and call `vpx.CodecEncConfigSet`.
  6. **`Close` (line 131)**: call `vpx.CodecDestroy(e.encoder)`, zero
     `e.encoder`, return any error wrapped.
  7. Add a unit test `av/video/encoder_cgo_test.go` (also tagged
     `cgo && libvpx`) that encodes a synthetic 320×240 YUV frame, asserts
     a non-nil byte slice and that the second encoded frame is materially
     smaller than the first when `keyFrame=false`.
  8. Extend `av/video/processor_benchmark_test.go` (per `PLAN.md` Step 7)
     with a `-tags libvpx` benchmark proving the 5–10× bandwidth claim
     from the README.
- **Dependencies**: none — Step 1 of `PLAN.md` (interface extraction) is
  already complete (`Encoder` interface in `av/video/processor.go:24-48`,
  build-tag dispatch via `encoder_purgo.go` / `encoder_cgo.go`).
- **Effort**: medium (≈300 LOC plus tests; gated by libvpx system headers
  in CI which would need a `libvpx-dev` install step).
- **Validation**:
  - `go build ./...` (no tag) — pure-Go path must remain green.
  - `go build -tags libvpx ./...` — must build without TODO errors.
  - `go test -tags libvpx -race ./av/video/...` — new tests must pass.
  - Bench: P-frame size <10% of preceding I-frame size at the same quality.

---

## Gap 2 — `IPacketDelivery` abstraction has only one production implementation

- **Severity**: MEDIUM
- **Locations**:
  - Interface: `interfaces/packet_delivery.go:45-106` (`IPacketDelivery`,
    `INetworkTransport`).
  - Factory: `factory/packet_delivery_factory.go:215-266`.
  - Real impl: `real/packet_delivery.go` (`real.NewRealPacketDelivery`).
  - Sim impl: `simulation/packet_delivery_sim.go`
    (`simulation.NewSimulatedPacketDelivery`).
  - Sole consumers: `toxcore.go:537` (`setupPacketDelivery`),
    `toxcore.go:1295` (`createPacketDelivery`),
    `toxcore.go:1303` (`createRealPacketDelivery`).
- **Intended Behavior**: `interfaces/doc.go:1-109` describes
  `IPacketDelivery` as "the foundational interface that enables switching
  between simulation and real network implementations, supporting both
  production deployments and deterministic testing scenarios" and offers a
  worked example of a third-party implementation (`MyTransport`). The
  factory pattern (`UseSimulation` flag, `TOX_USE_SIMULATION` env, hot-swap
  via `SwitchToSimulation`/`SwitchToReal`) is explicitly designed for
  pluggability.
- **Current State**:
  - The interface has exactly two in-tree implementations:
    `real.RealPacketDelivery` and `simulation.SimulatedPacketDelivery`.
  - The factory is never invoked from outside `toxcore.go`; the simulation
    path is reachable in production only when `udpTransport == nil` or
    `TOX_USE_SIMULATION=1`, and is otherwise used solely by tests.
  - The factory and `interfaces` package are **not** re-exported from the
    root `toxcore` package, so external consumers cannot register their
    own `IPacketDelivery` against a `*Tox` instance — defeating the
    documented extension point.
  - Net effect: `interfaces`, `factory`, `real`, and `simulation` add four
    packages and ~1,400 LOC of indirection for one production wiring
    decision (real vs. fallback-sim).
  - `go vet`/`go build` are clean; this is a structural/architectural
    concern, not a correctness bug.
- **Blocked Goal**: none of the README/ROADMAP/PLAN goals require this
  abstraction; it is internal scaffolding. The gap is the mismatch
  between `interfaces/doc.go`'s "pluggable" framing and the absence of any
  third-party plug point.
- **Implementation Path** (pick one):
  - **Option A — Make the abstraction real**:
    1. Re-export `interfaces.IPacketDelivery`, `interfaces.INetworkTransport`,
       and `factory.PacketDeliveryFactory` (or a constructor that accepts a
       custom `IPacketDelivery`) from the root `toxcore` package.
    2. Add `Options.PacketDelivery interfaces.IPacketDelivery` (nil → use
       factory default) and wire it through `createToxInstance` at
       `toxcore.go:579`.
    3. Add at least one example under `examples/custom_delivery_demo/`
       that supplies a user-defined `IPacketDelivery` (for instance, a
       loopback or a metrics-decorating wrapper).
    4. Update `interfaces/doc.go` so its example compiles against the
       newly-exported root API.
  - **Option B — Collapse the abstraction**:
    1. Move `real.RealPacketDelivery` into a new internal package
       (`internal/delivery/real.go`) or directly into `toxcore.go`.
    2. Move `simulation.SimulatedPacketDelivery` into a `_test.go` helper
       (`toxcore_simulation_test.go`) — it is only consumed by tests and
       by the `udpTransport == nil` fallback (which can be replaced with
       a simpler in-process loopback).
    3. Delete `interfaces/`, `factory/`, `real/`, `simulation/`.
    4. Replace the deprecated
       `IPacketDelivery.GetStats() map[string]interface{}` method with the
       already-exposed `GetPacketDeliveryTypedStats()`.
- **Dependencies**: none. Option A subsumes Gap 3 (the deprecated
  `GetStats()` becomes part of the now-public API and warrants a real
  v2.0.0 plan). Option B subsumes Gap 3 (the deprecated method is deleted
  outright).
- **Effort**: small for Option A (≈150 LOC + one example); medium for
  Option B (~1,400 LOC moved/deleted, all callers updated, full test
  re-run).
- **Validation**:
  - `go build ./...` and `go vet ./...` clean.
  - All existing `*packet_delivery*_test.go` tests in `factory/`, `real/`,
    `simulation/`, and `toxcore_*_test.go` pass.
  - `IsPacketDeliverySimulation()`, `GetPacketDeliveryStats()`, and
    `GetPacketDeliveryTypedStats()` retain their current behaviour for
    the C API (`capi/`).

---

## Gap 3 — `IPacketDelivery.GetStats()` is deprecated for v2.0.0 with no tracked removal

- **Severity**: LOW
- **Locations**:
  - Interface declaration: `interfaces/packet_delivery.go:89-99`.
  - Real impl: `real/packet_delivery.go` (mirrored).
  - Sim impl: `simulation/packet_delivery_sim.go:269-286`.
  - Public exposure: `toxcore.go:1336-1352`
    (`(*Tox).GetPacketDeliveryStats`).
- **Intended Behavior**: The doc comment promises a v2.0.0 removal:
  ```
  // Deprecated: Use GetTypedStats() for type-safe access to statistics.
  // This method will be removed in v2.0.0. Migration timeline:
  //   - v1.x: GetStats() available but deprecated
  //   - v2.0.0: GetStats() removed, use GetTypedStats() exclusively
  ```
- **Current State**:
  - `GetTypedStats() PacketDeliveryStats` exists and is the preferred path
    (`toxcore.go:1354-1363`).
  - There is no `v2` branch, no GitHub milestone, and no issue cross-link
    enforcing the v2.0.0 removal.
  - The deprecated method is still called by
    `(*Tox).GetPacketDeliveryStats` (a deprecated wrapper of the same
    name) — but the implementation rebuilds its map from the typed stats
    anyway, so the underlying interface method is **never actually
    invoked** in the in-tree code paths.
  - This is the "TODO with linked issue" pattern from Phase 3f rule 4 —
    except the link does not exist.
- **Blocked Goal**: none directly; this is API hygiene.
- **Implementation Path** (pick one):
  - **Track**: open a tracking issue
    "Remove `IPacketDelivery.GetStats()` in v2.0.0" enumerating the four
    call sites above, and reference the issue from each `Deprecated:` doc
    comment. No code change required.
  - **Pull forward**: in a v1.x minor, log a one-time `logrus.Warn` from
    each impl when `GetStats()` is called, so any external consumer
    (currently none in-tree) is alerted. Schedule actual removal for the
    next major.
  - **Remove now (preferred if Gap 2 Option B is chosen)**: delete
    `GetStats()` from the interface and its two impls; update
    `(*Tox).GetPacketDeliveryStats` to build the legacy-shaped map
    directly from `GetTypedStats()` (this is already what it does — the
    method body would shrink by zero lines). This eliminates a deprecated
    public method without losing behaviour.
- **Dependencies**: ideally bundled with Gap 2's resolution to avoid
  touching the same files twice.
- **Effort**: small (≤30 LOC if removing; zero if just opening an issue).
- **Validation**:
  - `grep -rn "\.GetStats()" --include='*.go' . | grep -v _test.go` should
    no longer reference `interfaces.IPacketDelivery` after removal.
  - `go build ./...`, `go vet ./...`, and the full test suite must remain
    green.
  - The C API exports in `capi/` must still produce statistics output of
    the same shape (currently consumed only via
    `(*Tox).GetPacketDeliveryStats`).

---

## Cross-cutting observations (not separate gaps)

- **No production `panic("not implemented")`** anywhere in the tree; every
  unsupported transport operation (Lokinet/Nym Listen, Tor/Lokinet UDP)
  returns an actionable wrapped error consistent with documented limits.
- **All five TODOs** in the production tree are in
  `av/video/encoder_cgo.go` and are folded into Gap 1.
- **Build-tag matrix is complete and self-consistent**:
  Linux/Darwin/BSD/Windows/wasm all have a corresponding
  `storage_limits_*.go`; pure-Go and `-tags libvpx` builds both compile.
  Any new platform should follow the `storage_limits` template.
- **Doc coverage 93.1%** with no stale annotations is a strong
  intent-signal: the codebase audits cleanly against its own stated
  surface, which is why the gap count is so low for a 41kLOC project.
