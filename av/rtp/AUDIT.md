# Audit: github.com/opd-ai/toxcore/av/rtp
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The av/rtp package provides RTP transport functionality for ToxAV audio/video streaming. Overall implementation quality is good with 89.4% test coverage and proper integration with Tox transport infrastructure. However, several non-deterministic operations, incomplete jitter buffer implementation, and minor code quality issues need attention before production use.

## Issues Found
- [ ] **med** Non-determinism — `crypto/rand` used for SSRC generation instead of deterministic seeded PRNG (`packet.go:82`, `session.go:105`)
- [ ] **med** Non-determinism — `time.Now()` used for timestamp calculations instead of monotonic time source (`session.go:118`, `session.go:200`, `packet.go:381`, `packet.go:448`, `packet.go:478`)
- [ ] **med** Non-determinism — `time.Since()` used for jitter buffer timing (`packet.go:435`)
- [ ] **low** Incomplete implementation — Jitter buffer does not order packets by timestamp, uses arbitrary iteration (`packet.go:446` comment: "simplified - should order by timestamp")
- [ ] **low** Code quality — `fmt.Printf` debug statement should use structured logging only (`packet.go:312`)
- [ ] **low** Doc coverage — Package lacks `doc.go` file for comprehensive package documentation
- [ ] **low** Incomplete implementation — Placeholder comment for video handler implementation (`transport.go:79` comment: "placeholder for Phase 3")
- [ ] **low** Resource management — No capacity limits on jitter buffer map, potential memory leak (`packet.go:408`)
- [ ] **low** Error handling — Video receive callback data is unused with comment but no callback registration mechanism documented (`transport.go:289` `_ = videoData`)
- [ ] **low** Error handling — Audio receive callback data is unused with comment but no callback registration mechanism documented (`transport.go:232` `_ = audioData`)

## Test Coverage
89.4% (target: 65%)
✅ **PASS** - Exceeds target coverage

## Integration Status
The av/rtp package integrates properly with the ToxAV system:
- ✅ Used by `av/types.go` for Call RTP session management
- ✅ Used by `examples/audio_streaming_demo` for demonstration
- ✅ Properly integrates with `transport.Transport` interface for packet transmission
- ✅ Registers packet handlers for `transport.PacketAVAudioFrame` and `transport.PacketAVVideoFrame`
- ✅ Uses `av/video` package for video RTP packetization/depacketization
- ⚠️ No callback mechanism documented for routing received audio/video data to application layer

## Recommendations
1. **HIGH PRIORITY**: Replace `crypto/rand` SSRC generation with deterministic seeded PRNG (e.g., `math/rand` with seed from session context) to ensure reproducible testing and deterministic behavior
2. **HIGH PRIORITY**: Replace `time.Now()` and `time.Since()` with monotonic time source or session-relative timestamps to enable deterministic testing and replay
3. **MEDIUM PRIORITY**: Implement proper timestamp-ordered jitter buffer using min-heap or sorted structure instead of arbitrary map iteration
4. **MEDIUM PRIORITY**: Add capacity limits to jitter buffer with eviction policy (e.g., oldest packets) to prevent unbounded memory growth
5. **LOW PRIORITY**: Remove `fmt.Printf` debug statement in favor of logrus structured logging only
6. **LOW PRIORITY**: Create `doc.go` with comprehensive package documentation, usage examples, and integration patterns
7. **LOW PRIORITY**: Document callback registration mechanism for received audio/video data or implement interface for data delivery to application layer
