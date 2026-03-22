# Implementation Gaps — 2026-03-22

This document identifies gaps between toxcore-go's stated goals (per README and documentation) and the actual implementation state.

---

## ToxAV Real Codec Integration

- **Stated Goal**: "Audio Calling: High-quality audio with Opus codec support" and "Video Calling: Video transmission with configurable quality"
- **Current State**: 
  - Audio encoding uses `SimplePCMEncoder` (av/audio/processor.go:40-120) which passes through raw PCM data without Opus compression
  - Video encoding uses `SimpleVP8Encoder` (av/video/codec.go:36-198) which packs raw YUV420 frames without VP8 compression
  - Audio *decoding* works correctly via pion/opus decoder
  - All signaling, RTP transport, callbacks, and media pipelines are fully implemented
- **Impact**: Audio/video calls will not interoperate with standard Tox clients (qTox, uTox, Toxygen) that expect Opus-encoded audio and VP8-encoded video. Calls between two toxcore-go instances will work but with significantly higher bandwidth usage.
- **Closing the Gap**:
  1. Integrate `github.com/pion/opus` encoder (decoder already integrated)
  2. Implement VP8 encoding via CGo wrapper to libvpx or pure Go VP8 encoder
  3. Add codec negotiation to match peer capabilities
  4. Validation: Create integration test that verifies codec roundtrip with standard Tox client

---

## File Transfer Accept API

- **Stated Goal**: README shows `tox.FileControl(friendID, fileNumber, toxcore.FileControlResume)` to accept files, implying programmatic file acceptance
- **Current State**: 
  - `FileSend()` (toxcore.go:2965) for initiating transfers is implemented
  - `FileControl()` (toxcore.go:2933) for pause/resume/cancel is implemented
  - Callbacks `OnFileRecv`, `OnFileRecvChunk`, `OnFileChunkRequest` are implemented
  - No explicit `FileAccept()` or `FileReceive()` function exists
- **Impact**: Developers must manually track incoming file transfers and construct FileControl calls. The API is functional but less ergonomic than documented examples suggest.
- **Closing the Gap**:
  1. Add `FileAccept(friendID, fileNumber uint32) error` convenience method
  2. Add `FileReject(friendID, fileNumber uint32) error` convenience method
  3. Document the callback-based workflow in doc.go
  4. Validation: `go test -v -run TestFileTransferWorkflow ./file/...`

---

## Nym Transport Listen Capability

- **Stated Goal**: README's Multi-Network Support table shows Nym with Listen: ❌, but MULTINETWORK.md and user expectations suggest full transport parity
- **Current State**: 
  - `Dial()` works via SOCKS5 proxy to local Nym client (transport/network_transport_impl.go:579-815)
  - `DialPacket()` works with emulated packet framing over SOCKS5
  - `Listen()` returns `ErrNymNotImplemented` — this is an architectural limitation, not a bug
- **Impact**: Users cannot host services reachable via Nym addresses. They can only connect to Nym-hosted services.
- **Closing the Gap**:
  1. Document clearly that Nym Listen requires running a Nym service provider (out of scope for this library)
  2. Consider adding integration guide for Nym service provider setup
  3. Update README table to clarify "Requires Nym service provider configuration"
  4. Validation: Documentation review

---

## Lokinet Transport Listen and UDP

- **Stated Goal**: README shows Lokinet with Listen: ❌ and UDP: ❌
- **Current State**:
  - `Dial()` works via SOCKS5 proxy (transport/network_transport_impl.go:817-970)
  - `Listen()` returns error directing users to configure SNApp via lokinet.ini
  - `DialPacket()` returns `ErrLokinetUDPNotSupported`
- **Impact**: Users cannot host SNApps (Lokinet hidden services) or use UDP through Lokinet from this library.
- **Closing the Gap**:
  1. Document SNApp configuration workflow in docs/LOKINET_TRANSPORT.md (if not present)
  2. Consider implementing Listen via lokinet RPC API if available
  3. UDP limitation is inherent to SOCKS5 proxy approach — document workaround using system-level Lokinet
  4. Validation: Documentation completeness check

---

## Pre-Key Lifecycle Cleanup

- **Stated Goal**: docs/ASYNC.md specifies "Automatic message cleanup and expiration" and forward secrecy with pre-key rotation
- **Current State**:
  - Pre-key generation works (100 keys per peer)
  - Pre-key exchange works
  - Pre-key consumption for forward secrecy works
  - `CleanupExpiredData()` exists (async/forward_secrecy.go:339) but is never automatically called
  - No periodic cleanup goroutine
- **Impact**: Over extended operation (30+ days), expired pre-keys accumulate on disk. While not a security vulnerability (old keys are unusable), it causes unbounded storage growth.
- **Closing the Gap**:
  1. Add cleanup goroutine to `NewForwardSecurityManager()` that runs every 24 hours
  2. Add configuration option for cleanup interval
  3. Add metrics for pre-key storage usage
  4. Validation: `go test -v -run TestPreKeyCleanupAutomation ./async/...`

---

## C API Coverage

- **Stated Goal**: "C binding annotations for cross-language use" and example C code in README
- **Current State**:
  - capi/toxcore_c.go implements ~15 core functions
  - capi/toxav_c.go implements ~18 ToxAV functions
  - Total: ~33 functions vs ~80+ in libtoxcore/libtoxav headers
  - Missing categories: most conference functions, status query functions, iterate functions
- **Impact**: C/C++ applications cannot use this as a drop-in replacement for libtoxcore. Significant wrapper code would be needed.
- **Closing the Gap**:
  1. Create comprehensive mapping of libtoxcore API to Go functions
  2. Implement missing functions in priority order (commonly used first)
  3. Add C header file generation for IDE support
  4. Validation: Compare function coverage against c-toxcore/toxcore/tox.h

---

## Bootstrap Reliability

- **Stated Goal**: "Connect to the Tox network" and "Bootstrap node connectivity" listed as fully implemented
- **Current State**:
  - Bootstrap works but default timeout (5 seconds) is too aggressive
  - GitHub issues #30, #35 show users experiencing timeout failures
  - No automatic retry logic
  - BootstrapTimeout is configurable but default is problematic
- **Impact**: First-time users following README examples experience failures connecting to the network under normal conditions.
- **Closing the Gap**:
  1. Increase default BootstrapTimeout from 5s to 30s
  2. Add automatic retry with exponential backoff (up to 3 retries)
  3. Add multiple bootstrap nodes in examples (not just one)
  4. Document known working bootstrap nodes with their typical response times
  5. Validation: Manual testing against tox.initramfs.io, node.tox.biribiri.org

---

## Documentation-Implementation Sync

- **Stated Goal**: Comprehensive documentation with accurate examples
- **Current State**:
  - README examples are accurate for basic usage
  - doc.go accurately describes API
  - Some inconsistencies in group chat terminology (README says "GroupNew", code says "Create")
  - docs/CHANGELOG.md may be outdated
- **Impact**: Minor confusion for developers comparing README to API.
- **Closing the Gap**:
  1. Audit all README code examples against current API
  2. Ensure function names in documentation match exported Go functions
  3. Update CHANGELOG.md with recent changes
  4. Validation: Extract all code blocks from README and verify they compile

---

## Summary Priority Matrix

| Gap | Severity | Effort | Priority |
|-----|----------|--------|----------|
| ToxAV Real Codec Integration | High | High | 1 |
| Pre-Key Lifecycle Cleanup | Medium | Low | 2 |
| Bootstrap Reliability | Medium | Low | 3 |
| File Transfer Accept API | Low | Low | 4 |
| C API Coverage | Medium | High | 5 |
| Nym Transport Listen | Low | N/A* | 6 |
| Lokinet Transport Listen/UDP | Low | N/A* | 7 |
| Documentation-Implementation Sync | Low | Low | 8 |

*Architectural limitations — documentation updates only, no code changes possible.
