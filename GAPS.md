# Implementation Gaps — 2026-05-28

This document lists gaps between what `toxcore-go` claims (in the README, package GoDoc, and
in-code documentation) and what the code actually does. Each gap was verified against the
source. The core library is mature and the README is largely accurate; the gaps below are the
exceptions.

## 1. Linux socket receive-buffer tuning is non-functional
- **Stated Goal**: `transport/batch_receive_linux.go` provides "socket tuning for improved
  throughput" via the exported helper `SetSocketReceiveBuffer(conn net.PacketConn, size int)`,
  documented as setting the kernel-level receive buffer (typical 256 KB–4 MB).
- **Current State**: The function uses a single-value type assertion to a custom interface whose
  method signature does not match the standard library:
  `conn.(interface{ SyscallConn() (interface{}, error) })`. Real connections implement
  `SyscallConn() (syscall.RawConn, error)`, so no `net.PacketConn` satisfies the asserted
  interface and the call **panics** for every valid connection (verified empirically). The
  helper can never succeed.
- **Impact**: Any caller attempting the advertised socket tuning crashes. In practice the helper
  has no in-tree callers, so the *advertised* throughput tuning is simply unavailable rather than
  actively breaking the running node — but the public API is broken as written.
- **Closing the Gap**: Fix the assertion to `conn.(interface{ SyscallConn() (syscall.RawConn, error) })`
  (or assert to the standard `syscall.Conn`) using the two-value form and return an error when the
  connection does not implement it; add a unit test exercising a real UDP socket. (See AUDIT.md F-1.)

## 2. ToxAV inter-frame (P-frame) video decoding is not implemented
- **Stated Goal**: The README ToxAV section says video uses VP8 "with both I-frames (key frames)
  and P-frames (inter frames) for bandwidth-efficient video."
- **Current State**: The same README paragraph then discloses that "Current decode behavior is
  keyframe-oriented: inter frames are not decoded by the existing decoder path and will display
  as the last decoded key frame." Received P-frames are therefore not rendered as distinct
  frames on the decode side.
- **Impact**: Receivers see frozen video (the last key frame) between key frames unless the
  sender is configured to force all-key-frames, which substantially increases bandwidth — the
  opposite of the "bandwidth-efficient" goal for the receive path.
- **Closing the Gap**: Implement P-frame (inter-frame) decoding in the VP8 decoder path so
  received inter frames update the displayed frame, or document the all-key-frames configuration
  as the supported mode. This is a known/disclosed limitation rather than a hidden discrepancy.

## 3. Relay stream multiplexer is present but not wired into the transport
- **Stated Goal**: The README highlights NAT traversal with "TCP relay fallback for symmetric
  NAT," and `transport/relay_mux.go` implements a full Noise-stream multiplexer (`RelayMux`,
  `OpenStream`, `handleStreamOpen`, etc.) with per-peer stream tracking.
- **Current State**: `NewRelayMux` has **no non-test callers** — the multiplexer is not
  instantiated anywhere in the production code paths. Additionally, `handleStreamOpen`
  overwrites the per-peer entry in `streamsByKey` without honoring the one-stream-per-peer
  invariant that `OpenStream` enforces (AUDIT.md F-2), and `NewRelayMux` panics on invalid
  config instead of returning an error (AUDIT.md F-10).
- **Impact**: The relay-multiplexing capability appears complete in the package surface but is
  effectively dormant; if it is later wired in without addressing F-2, simultaneous-open glare
  between peers would orphan streams and violate the dedup contract.
- **Closing the Gap**: Either wire `RelayMux` into the relay fallback path (after fixing F-2 and
  F-10) or mark it clearly as experimental/unused in its package documentation.

## 4. RTP video reassembly relies on an undocumented single-goroutine invariant
- **Stated Goal**: The README advertises ToxAV "RTP transport ... and jitter buffering" and the
  project's correctness goal of being race-free (`go test -race`).
- **Current State**: `av/video/rtp.go`'s `RTPDepacketizer` mutates its `frameBuffer` map (and the
  per-frame `packets` slices) with **no mutex**. This is currently safe only because all
  mutations flow through `ProcessPacket`, which is invoked solely from the single UDP receive
  goroutine, and the lone concurrent reader (`Processor.GetRTPStats` → `GetBufferedFrameCount`)
  has no production callers. The safety invariant is neither documented nor enforced.
- **Impact**: The type is not safe for concurrent use, contrary to the reasonable expectation
  for an RTP component. If `GetRTPStats` is ever called from a stats/metrics goroutine, or if
  packet dispatch is ever parallelized, it will trigger a fatal concurrent-map-access. The
  `-race` suite does not currently exercise a concurrent caller, so this would pass tests today.
- **Closing the Gap**: Add a `sync.Mutex`/`sync.RWMutex` to `RTPDepacketizer` guarding all
  `frameBuffer`/assembly access, or document the single-goroutine contract explicitly in the
  type's GoDoc. (See AUDIT.md "False Positives" entry for `av/video/rtp.go`.)

## 5. `GetSavedata` cannot report serialization failure
- **Stated Goal**: README State Persistence section presents `savedata := tox.GetSavedata()` as
  the way to obtain the persisted state bytes (private keys, friend list, name, status).
- **Current State**: `Tox.GetSavedata()` (`toxcore.go:417`) returns only `[]byte`; on a marshal
  error it logs and returns `nil`, so callers cannot distinguish a serialization failure from an
  empty/no-op result. The error-returning `Tox.Save() ([]byte, error)` exists but the README's
  primary persistence example uses `GetSavedata`.
- **Impact**: A serialization failure during a save would silently produce `nil`, which an
  application could persist as if it were valid state, risking unnoticed data loss.
- **Closing the Gap**: Document the `nil`-on-error contract in the `GetSavedata` GoDoc and point
  callers needing error detail to `Save()`, or have `GetSavedata` delegate to `Save()`.
  (See AUDIT.md F-8.)
