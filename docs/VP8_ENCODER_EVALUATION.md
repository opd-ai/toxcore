# VP8 Encoder Evaluation for P-Frame Support

## Executive Summary

**Updated:** April 2026

**Conclusion:** P-frame (inter-frame) encoding is now fully supported in the default
pure-Go build. The `opd-ai/vp8` library (v0.0.0-20260407) was updated to include
complete inter-frame prediction with diamond-search motion estimation, reference frame
management (last, golden, altref), adaptive coefficient probability updates, and
configurable DCT partition counts. No CGo or external libraries are required for
efficient VP8 encoding.

The optional CGo libvpx backend (`-tags libvpx`) remains available for users who
require Google's reference implementation.

---

## Evaluated Libraries

### 1. opd-ai/vp8 (Default Implementation)

**Repository:** https://github.com/opd-ai/vp8  
**Version used:** `v0.0.0-20260407023446-a01cf06c95d4`

| Attribute | Value |
|-----------|-------|
| Pure Go | ✅ Yes |
| I-frame Support | ✅ Yes |
| P-frame Support | ✅ Yes (motion estimation) |
| Loop Filter | ✅ Yes (`SetLoopFilterLevel`) |
| Golden / AltRef frames | ✅ Yes (`SetGoldenFrameInterval`, `ForceGoldenFrame`) |
| Adaptive coefficient probs | ✅ Yes (`SetProbabilityUpdates`) |
| Multi-partition encoding | ✅ Yes (1/2/4/8 partitions) |
| Per-plane quantizer deltas | ✅ Yes (`SetQuantizerDeltas`) |
| Segmentation | ❌ No (by design) |
| Sub-pixel motion estimation | ❌ No (integer-pel only, by design) |
| Platform | All (pure Go, including WASM) |
| CGo Required | No |

**Bandwidth Impact at 640×480@30fps:**
- I-frame-only (keyFrameInterval=0): ~500–800 kbps
- With P-frames (keyFrameInterval=30): ~50–150 kbps typical
- Savings: 5–10× depending on scene complexity

**New API exposed through toxcore** (`av/video/processor.go`):
```go
// Encoder interface — available on all encoder implementations
encoder.SetGoldenFrameInterval(10) // auto-update golden every 10 inter frames
encoder.ForceGoldenFrame()         // manual golden refresh after scene cut

// RealVP8Encoder only — advanced tuning
enc.SetPartitionCount(video.VP8TwoPartitions)
enc.SetProbabilityUpdates(true)
enc.SetQuantizerDeltas(0, 1, 1, -1, -1)
```

**VideoEncoderConfig** for one-shot configuration:
```go
cfg := video.VideoEncoderConfig{
    KeyframeInterval:    30,   // 1 key frame per second at 30fps
    GoldenFrameInterval: 10,   // golden update every 10 inter frames
    LoopFilterLevel:     20,   // moderate blocking-artifact reduction
    PartitionCount:      video.VP8TwoPartitions,
    ProbabilityUpdates:  true,
}
processor, err := video.NewProcessorWithConfig(640, 480, 512000, cfg)
```

---

### 2. xlab/libvpx-go (Optional CGo Backend)

**Repository:** https://github.com/xlab/libvpx-go  
**Go Package:** https://pkg.go.dev/github.com/xlab/libvpx-go

| Attribute | Value |
|-----------|-------|
| Pure Go | ❌ No (CGo) |
| I-frame Support | ✅ Yes |
| P-frame Support | ✅ Yes |
| B-frame Support | ❌ No (VP8 limitation) |
| All VP8 Features | ✅ Yes |
| Platform | Linux, macOS, Windows (with libvpx installed) |
| CGo Required | Yes |

**Assessment:** Full VP8 feature set via Google's reference implementation.
Available as an optional backend via `-tags libvpx`. Use when libvpx is
already available in the build environment and a battle-tested reference
implementation is preferred.

**Build instructions:**
```bash
# Ubuntu/Debian
apt-get install libvpx-dev
go build -tags libvpx ./...

# macOS
brew install libvpx
go build -tags libvpx ./...
```

---

### 3. pion/mediadevices/pkg/codec/vpx

**Repository:** https://github.com/pion/mediadevices  
**Go Package:** https://pkg.go.dev/github.com/pion/mediadevices/pkg/codec/vpx

| Attribute | Value |
|-----------|-------|
| Pure Go | ❌ No (CGo) |
| I-frame Support | ✅ Yes |
| P-frame Support | ✅ Yes |
| WebRTC Integration | ✅ Native |
| Platform | Linux, macOS, Windows |
| CGo Required | Yes |

**Assessment:** Part of the pion WebRTC ecosystem. Not used by toxcore-go
since the pure-Go `opd-ai/vp8` now covers the use case without CGo.

---

### 4. golang.org/x/image/vp8 (Decoder Only)

**Go Package:** https://pkg.go.dev/golang.org/x/image/vp8

| Attribute | Value |
|-----------|-------|
| Pure Go | ✅ Yes |
| Encoder | ❌ No (decoder only) |
| I-frame Decode | ✅ Yes |
| P-frame Decode | ✅ Yes |

**Assessment:** Used by toxcore-go for the receive-side VP8 decode path.
Key frames are fully decoded. P-frames received from the network are handled
by returning the last decoded key frame (see `decodeFrameData` in
`av/video/processor.go`), so the display always shows a valid image.

---

## Comparison Matrix

| Library | Pure Go | I-frame | P-frame | Golden/AltRef | Maintenance |
|---------|---------|---------|---------|---------------|-------------|
| opd-ai/vp8 | ✅ | ✅ | ✅ | ✅ | Active |
| xlab/libvpx-go | ❌ CGo | ✅ | ✅ | ✅ (internal) | Moderate |
| pion/mediadevices/vpx | ❌ CGo | ✅ | ✅ | ✅ (internal) | Active |
| golang.org/x/image/vp8 | ✅ | Decode | Decode | N/A | Go Team |

---

## Current Implementation

### Default Build (no CGo required)

```bash
go build ./...        # uses opd-ai/vp8 — I-frames + P-frames
go test ./av/video/...
```

### Optional CGo Build

```bash
go build -tags libvpx ./...   # uses xlab/libvpx-go
```

### Checking Encoder Capabilities at Runtime

```go
encoder, _ := video.NewDefaultEncoder(640, 480, 512000)
fmt.Println(video.DefaultEncoderName())          // "opd-ai/vp8 (I-frames and P-frames)"
fmt.Println(encoder.SupportsInterframe())        // true

encoder.SetKeyFrameInterval(30)   // 1 key frame per second at 30fps
encoder.SetGoldenFrameInterval(10) // golden frame every 10 inter frames
encoder.ForceKeyFrame()            // force next frame to be a key frame
encoder.ForceGoldenFrame()         // force golden refresh on next inter frame
```

### Benchmarking P-Frame Bandwidth Savings

```bash
go test -bench='BenchmarkPFrameBandwidth' -benchmem ./av/video/...
```

Expected output shows per-frame byte sizes for:
- `BenchmarkPFrameBandwidthIFrameOnly` — key frames only (higher bandwidth)
- `BenchmarkPFrameBandwidthInterFrame` — mixed I+P frames (significantly lower bandwidth)

---

## Future Considerations

### P-Frame Decoding

The `golang.org/x/image/vp8` decoder handles only key frames. Received P-frames
are currently displayed as the last decoded key frame (graceful degradation).
Full P-frame decode would require either:
- Porting the `golang.org/x/image/vp8` inter-frame decode path
- Using a third-party VP8 decoder that supports inter frames

### AV1 / VP9

If VP8 P-frame quality becomes insufficient for future requirements:
1. **AV1**: Better compression, but no pure-Go encoder in the ecosystem
2. **VP9**: Better compression than VP8, same CGo-required situation

---

## References

- [RFC 6386 - VP8 Data Format and Decoding Guide](https://tools.ietf.org/html/rfc6386)
- [opd-ai/vp8 README](https://github.com/opd-ai/vp8/blob/main/README.md)
- [libvpx-go Documentation](https://pkg.go.dev/github.com/xlab/libvpx-go)
- [pion/mediadevices vpx](https://pkg.go.dev/github.com/pion/mediadevices/pkg/codec/vpx)
- [WebM VP8 Encoding Guide](https://www.webmproject.org/docs/encoder-parameters/)
