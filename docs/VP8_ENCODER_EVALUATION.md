# VP8 Encoder Evaluation for P-Frame Support

## Executive Summary

**Evaluation Date:** April 2026

**Conclusion:** No pure-Go VP8 encoder with P-frame (inter-frame) support currently exists. To achieve standard VP8 compression efficiency (5-10x better than I-frame-only), the project must either:
1. Wait for upstream `opd-ai/vp8` to add P-frame support (major undertaking)
2. Implement an optional CGo path using `libvpx` (recommended for production video)
3. Accept current I-frame-only limitation for pure-Go constraint

---

## Evaluated Libraries

### 1. opd-ai/vp8 (Current Implementation)

**Repository:** https://github.com/opd-ai/vp8

| Attribute | Value |
|-----------|-------|
| Pure Go | ✅ Yes |
| I-frame Support | ✅ Yes |
| P-frame Support | ❌ No |
| B-frame Support | ❌ No |
| Loop Filter | ❌ No |
| Segmentation | ❌ No |
| Platform | All (pure Go) |
| CGo Required | No |

**Assessment:** The library explicitly states "I-frame only — every Encode call produces a key frame" in its README. This is by design to keep the implementation simple. Adding P-frame support would require:
- Motion estimation algorithms (block matching, diamond search)
- Reference frame buffer management
- Temporal prediction logic
- Significantly increased complexity and maintenance burden

**Bandwidth Impact:** I-frame-only encoding requires 5-10x more bandwidth than standard VP8:
- 720p@30fps I-frame-only: ~5-10 Mbps
- 720p@30fps with P-frames: ~500K-1 Mbps

---

### 2. xlab/libvpx-go

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

**Assessment:** Full VP8 feature set via Google's reference implementation. Provides:
- Complete temporal prediction (P-frames)
- Rate control and bitrate targeting
- Quality tuning (speed vs quality tradeoff)
- Real-time encoding mode optimized for video calls

**Dependencies:**
```bash
# Ubuntu/Debian
apt-get install libvpx-dev

# macOS
brew install libvpx

# Windows
# Download from https://github.com/nickstenning/libvpx-win
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

**Assessment:** Part of the pion WebRTC ecosystem. Well-maintained and actively developed. Good choice if already using pion for WebRTC.

**Dependencies:** Same as libvpx-go (requires libvpx headers and library)

---

### 4. golang.org/x/image/vp8 (Decoder Only)

**Go Package:** https://pkg.go.dev/golang.org/x/image/vp8

| Attribute | Value |
|-----------|-------|
| Pure Go | ✅ Yes |
| Encoder | ❌ No (decoder only) |
| I-frame Decode | ✅ Yes |
| P-frame Decode | ✅ Yes |

**Assessment:** Standard library decoder. Already used by toxcore-go for decoding. Supports both I-frames and P-frames on the decode side, so switching to a CGo encoder with P-frame output would be seamless for decoding.

---

## Comparison Matrix

| Library | Pure Go | I-frame | P-frame | Maintenance | Complexity |
|---------|---------|---------|---------|-------------|------------|
| opd-ai/vp8 | ✅ | ✅ | ❌ | Active | Low |
| xlab/libvpx-go | ❌ | ✅ | ✅ | Moderate | Medium |
| pion/mediadevices/vpx | ❌ | ✅ | ✅ | Active | Medium |
| golang.org/x/image/vp8 | ✅ | Decode | Decode | Go Team | N/A |

---

## Recommendation

### For Production Video Calling

Implement a **CGo-optional architecture** that:
1. Uses `opd-ai/vp8` by default (pure Go, I-frame-only)
2. Optionally uses `xlab/libvpx-go` when CGo is available and `//go:build cgo` tag is set
3. Provides build tags to choose encoder at compile time

This approach:
- Maintains pure-Go default for maximum portability
- Enables production-quality video for users who can use CGo
- Preserves WASM compatibility (pure Go path)
- Follows Go ecosystem conventions (like `sqlite` vs `sqlite3` packages)

### Implementation Design

```go
// av/video/encoder.go (interface)
type VP8Encoder interface {
    Encode(frame *VideoFrame) ([]byte, error)
    SetBitRate(bps int) error
    SupportsInterframe() bool
    Close() error
}

// av/video/encoder_purgo.go
//go:build !cgo || !libvpx

func NewVP8Encoder(width, height uint16, bitrate int) VP8Encoder {
    return NewRealVP8Encoder(width, height, bitrate) // I-frame only
}

// av/video/encoder_cgo.go  
//go:build cgo && libvpx

func NewVP8Encoder(width, height uint16, bitrate int) VP8Encoder {
    return NewLibVPXEncoder(width, height, bitrate) // Full VP8
}
```

### Build Tags

```bash
# Pure Go (default) - I-frame only
go build ./...

# With libvpx - Full VP8 with P-frames
go build -tags libvpx ./...
```

---

## Future Considerations

### Alternative Codecs

If VP8 P-frame support remains unavailable in pure Go, consider:

1. **AV1 (via rav1e bindings)**: Newer codec, better compression, but no pure-Go encoder
2. **VP9**: Better compression than VP8, same library situation
3. **H.264**: Patent-encumbered, typically CGo-only

### Upstream Contribution

Contributing P-frame support to `opd-ai/vp8` would require:
- Motion vector estimation (significant algorithmic work)
- Reference frame buffer (~3 frames for VP8)
- Inter-prediction modes (NEW_MV, NEAREST_MV, NEAR_MV, ZERO_MV)
- Estimated effort: 2-3 months of focused development

---

## References

- [RFC 6386 - VP8 Data Format and Decoding Guide](https://tools.ietf.org/html/rfc6386)
- [opd-ai/vp8 README](https://github.com/opd-ai/vp8/blob/main/README.md)
- [libvpx-go Documentation](https://pkg.go.dev/github.com/xlab/libvpx-go)
- [pion/mediadevices vpx](https://pkg.go.dev/github.com/pion/mediadevices/pkg/codec/vpx)
- [WebM VP8 Encoding Guide](https://www.webmproject.org/docs/encoder-parameters/)
