# GAPS — Documented Product Surface vs. Actual Implementation

**Repository:** `opd-ai/toxcore`
**Date:** 2026-04-21
**Companion to:** [`AUDIT.md`](AUDIT.md)
**Scope:** Discrepancies between what `README.md` (and linked `docs/*`) promises to users and what the `opd-ai/toxcore` source actually implements. No code changes were made.

Severity legend:

- 🟥 **High** — A documented feature is absent or broken; users following the README will fail.
- 🟧 **Medium** — A claim is materially inaccurate (overstated capability) and could mislead an evaluator.
- 🟨 **Low** — A claim is slightly inaccurate or understated; no user-facing breakage.
- 🟦 **Informational** — Public API or capability exists but is undocumented; not a broken promise, but an adoption-friction gap.

---

## GAP-B — 🟧 Medium: NAT-PMP / PCP is advertised but not implemented

**Promise (README §Features, bullet "NAT Traversal"):**

> **NAT Traversal** — STUN, UPnP, NAT-PMP detection with TCP relay fallback
> (`transport/nat.go`, `transport/hole_puncher.go`, `transport/advanced_nat.go`)

**Additional reinforcement in `transport/doc.go:75`:**

> - NAT-PMP/PCP support for Apple and other devices

**Reality:**

The `transport/` directory contains NAT-traversal implementations for:

- `stun_client.go` — RFC 5389 STUN client (external-address discovery)
- `upnp_client.go` — UPnP IGD port mapping
- `hole_puncher.go` — UDP hole-punching
- `advanced_nat.go` — detection + TCP relay fallback
- `nat.go`, `nat_helper_test.go`, `nat_integration_test.go`

There is **no NAT-PMP or PCP client**. A full source tree search
(`grep -rni "nat-pmp|natpmp|nat_pmp|pmp" transport/ net/`) finds exactly one
reference — the `transport/doc.go` comment itself. No RFC 6886 (NAT-PMP) or
RFC 6887 (PCP) packet formats, opcodes, or client state machine are
implemented.

**User impact:**

Developers on Apple AirPort, many consumer routers, and other PCP-capable
devices will find that port-mapping silently falls back to UPnP-only
behaviour, contrary to the documented capability. This is a feature
advertised on the landing README that does not exist in code.

**Suggested resolution (no code change performed):**

- Either implement a NAT-PMP/PCP client (RFC 6886/6887) — typically a small
  additional file alongside `upnp_client.go`, or
- Remove "NAT-PMP" from the README bullet and the
  `transport/doc.go` "NAT-PMP/PCP support for Apple and other devices" line to
  accurately reflect current capability (STUN + UPnP + hole-punching + TCP
  relay).

---

## GAP-A — 🟨 Low: VP8 capability understated in README

**Promise (README §Audio/Video Calls / **Limitations**):**

> **Limitations**: The VP8 encoder produces key frames only, resulting in
> higher bandwidth compared to full inter-frame encoding. The `opd-ai/vp8`
> library does not yet support P-frame encoding.

And in the Features bullet:

> VP8 video via `opd-ai/vp8`, RTP transport, adaptive bitrate, and jitter
> buffering

**Reality (source of truth):**

- `av/video/encoder_purgo.go` (build constraint `!cgo || !libvpx`, the
  default build):

  ```
  // The opd-ai/vp8 library supports both I-frames and P-frames with
  // motion estimation.
  //
  // This encoder supports both I-frames (key frames) and P-frames (inter
  // frames) with motion estimation. Key frames are emitted periodically
  // based on the configured key frame interval (default: every 30 frames).
  ```

- `av/video/codec.go:44`:

  ```
  // Produces RFC 6386 compliant VP8 bitstreams with both key frames
  // (I-frames) and inter frames (P-frames) that are compatible with
  // standard VP8 decoders...
  ```

- `av/video/encoder_cgo.go` (optional, built with `-tags libvpx`) wraps
  `xlab/libvpx-go` for libvpx-backed VP8 with full P-frame support.

**User impact:**

The README tells users and evaluators that video calls will consume
significantly more bandwidth than a standard VP8 codec and that P-frame
encoding is unavailable. In fact the default pure-Go encoder now supports
P-frames. Prospective users may discount ToxAV for bandwidth-sensitive
deployments based on an outdated caveat.

**Suggested resolution:**

- Remove or rewrite the "Limitations" paragraph in README §Audio/Video Calls
  to describe the actual capability (pure-Go I+P-frame encoding by default,
  optional libvpx backend via `-tags libvpx`).

---

## GAP-C — 🟦 Informational: `Save` / `Load` / `SaveSnapshot` / `LoadSnapshot` public API is undocumented in README

**Promise (README §State Persistence):**

> Save and restore the Tox instance state (private keys, friend list, name,
> status):
> ```go
> savedata := tox.GetSavedata()
> …
> tox, err = toxcore.NewFromSavedata(nil, savedata)
> ```
> Alternatively, pass savedata through `Options`.

**Reality:**

`toxcore_lifecycle.go` exports four additional public methods that are not
referenced anywhere in the README:

- `func (t *Tox) Save() ([]byte, error)`         — `toxcore_lifecycle.go:280`
- `func (t *Tox) Load(data []byte) error`        — `toxcore_lifecycle.go:304`
- `func (t *Tox) SaveSnapshot() ([]byte, error)` — `toxcore_lifecycle.go:336`
- `func (t *Tox) LoadSnapshot(data []byte) error` — `toxcore_lifecycle.go:369`

**User impact:**

Developers will not discover these APIs from the README and may be unsure
how `Save`/`Load` relate to `GetSavedata`/`NewFromSavedata`, or what
`SaveSnapshot` offers versus `Save`. Not a broken promise, but a
discoverability gap.

**Suggested resolution:**

- Add a short note in README §State Persistence describing when to use
  `Save`/`Load` vs. `GetSavedata`/`NewFromSavedata`, and what
  `SaveSnapshot`/`LoadSnapshot` are for. Alternatively, mark the extra
  methods as internal or deprecated if they are not intended for public use.

---

## GAP-D — 🟦 Informational: ToxAV public API partially documented

**Promise (README §Audio/Video Calls):**

The README demonstrates `Call`, `Answer`, `AudioSendFrame`,
`VideoSendFrame`, `CallbackCall`, `CallbackAudioReceiveFrame`,
`CallbackVideoReceiveFrame`, `Iterate`, `IterationInterval`, `Kill`.

**Reality:**

`toxav.go` also exports:

- `CallControl` — mute/unmute/pause/resume/cancel a call
- `AudioSetBitRate`, `VideoSetBitRate` — runtime bitrate control
- `CallbackCallState` — call state-change notifications (needed to know when
  a call ends or the peer rejects)
- `CallbackAudioBitRate`, `CallbackVideoBitRate` — adaptive bitrate
  notifications

These are core pieces of a call-lifecycle implementation (without
`CallbackCallState`, the sample code in the README cannot actually
terminate cleanly when the peer hangs up, and without `CallControl` the
app cannot end its own call).

**User impact:**

Developers writing a complete calling UI from the README alone will lack
the call-state and control hooks they need and will have to discover them
via godoc or source inspection.

**Suggested resolution:**

- Extend README §Audio/Video Calls with a second snippet covering call
  lifecycle (`CallControl`, `CallbackCallState`) and adaptive bitrate
  (`AudioSetBitRate`, `VideoSetBitRate`, `CallbackAudioBitRate`,
  `CallbackVideoBitRate`).

---

## GAP-E — 🟦 Informational: Function-level GoDoc coverage is below the documentation standard

**Promise (project "Quality Standards" conventions, implicit in
`doc.go`):**

> All public APIs must have GoDoc comments starting with the function/type
> name.

**Reality (from `go-stats-generator analyze .`):**

| Scope | Coverage |
|---|---|
| Packages | 96.2% |
| Types | 91.7% |
| Methods | 79.5% |
| **Functions** | **50.1%** |
| Overall | 63.3% |

Roughly half of non-method top-level functions lack a GoDoc comment.
Many of these are internal helper functions whose names start with lowercase
letters (genuinely unexported and not part of the public API), so the raw
50% figure is likely pessimistic for user-facing surface area. However it is
worth noting as documentation hygiene.

**User impact:**

Low — unexported helpers are not part of the advertised product surface.
Noted here only because the repository lists documentation quality as a
"Quality Standards" objective.

**Suggested resolution:**

- Run `golint`/`revive` with the `exported` rule against the public
  packages (`toxcore`, `toxav`, `async`, `crypto`, `transport`, `toxnet`,
  `capi`) and add GoDoc to any gaps surfaced there.

---

## Verified claims (no gap)

For completeness, the following README claims were checked and match
implementation exactly — they are *not* gaps:

- All 12 `NewOptions()` default values and all 5 `DeliveryRetryConfig`
  defaults match their README tables bit-for-bit.
- All async messaging constants match: `MaxMessageSize = 1372`,
  `MaxStorageTime = 24h`, `MaxMessagesPerRecipient = 100`,
  padding buckets 256 / 1024 / 4096 / 16384, `PreKeyRefreshThreshold = 20`,
  `EpochDuration = 6h`, 1% disk allocation.
- The multi-network transport table (IPv4/IPv6, Tor, I2P, Lokinet, Nym) is
  accurate — Lokinet and Nym `Listen` correctly return explicit
  "not supported via SOCKS5" errors; Tor and I2P support both `Listen` and
  `Dial`.
- `NewNoiseTransport`, `NewNegotiatingTransport`,
  `DefaultProtocolCapabilities`, and the `EnableLegacyFallback = false`
  secure-by-default behaviour all match the README.
- The `ProxyTypeHTTP` (TCP only) / `ProxyTypeSOCKS5` (TCP + optional UDP
  ASSOCIATE) matrix is accurate; `SOCKS5UDPAssociation` implementation
  exists at `transport/proxy.go`.
- Every directory listed in the README's "Project Structure" tree exists on
  disk with the advertised contents.
- Every document linked from the README's §Documentation section exists at
  its claimed path in `docs/`.
- `go build ./...` succeeds without cgo.
- 64 `//export` directives in `capi/toxcore_c.go` plus 18 in
  `capi/toxav_c.go` deliver the documented libtoxcore-compatible C API.

---

## Summary

| ID | Severity | Area | One-line description |
|---|---|---|---|
| GAP-A | 🟨 Low (docs lag) | ToxAV | README says VP8 is key-frames-only; implementation now supports P-frames with motion estimation. |
| GAP-B | 🟧 Medium (false claim) | NAT traversal | README and `transport/doc.go` advertise NAT-PMP/PCP support; no implementation exists. |
| GAP-C | 🟦 Info (discoverability) | Persistence | `Save`, `Load`, `SaveSnapshot`, `LoadSnapshot` are exported but undocumented in README. |
| GAP-D | 🟦 Info (discoverability) | ToxAV | README omits `CallControl`, `AudioSetBitRate`, `VideoSetBitRate`, `CallbackCallState`, `CallbackAudio/VideoBitRate`. |
| GAP-E | 🟦 Info (quality) | Docs | Top-level function GoDoc coverage is 50.1% (mostly internal helpers). |

**No 🟥 High-severity gaps were identified.** The product as shipped is substantively what the README describes; the one material inaccuracy is the NAT-PMP/PCP claim.
