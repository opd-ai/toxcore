# Implementation Gaps — 2026-03-22

This document identifies gaps between toxcore-go's stated goals and current implementation status, with actionable guidance for closing each gap.

---

## Noise-IK Cryptographic Bug

- **Stated Goal**: Noise Protocol Framework integration with IK pattern for enhanced security, forward secrecy, and KCI resistance.
- **Current State**: The Noise-IK implementation exists and uses the correct pattern (DH25519 + ChaCha20-Poly1305 + SHA256), but there is a critical cipher state assignment error at `noise/handshake.go:262-263`. After the initiator calls `ReadMessage()`, the returned cipher states are swapped—`recvCipher` is assigned to `ik.recvCipher` when it should be assigned to `ik.sendCipher`, and vice versa. The responder implementation (lines 234-235) is correct.
- **Impact**: All post-handshake encrypted communication fails for initiators. Messages encrypted by the initiator cannot be decrypted by the responder, and vice versa. This renders Noise-IK completely non-functional for real-world use despite passing basic handshake tests.
- **Closing the Gap**: 
  1. Swap the assignments at lines 262-263:
     ```go
     ik.sendCipher = recvCipher  // First return is for sending
     ik.recvCipher = sendCipher  // Second return is for receiving
     ```
  2. Add integration test in `noise/handshake_test.go` that performs bidirectional encryption after handshake completion.
  3. Validate: `go test -race ./noise/... -v`

---

## Callback Thread Safety

- **Stated Goal**: The Tox struct is safe for concurrent use. Internal synchronization ensures that callbacks and API calls can be made from multiple goroutines.
- **Current State**: Eight callback registration methods at `toxcore.go:2165-2238` (OnFriendRequest, OnFriendMessage, OnFriendMessageDetailed, OnFriendStatus, OnConnectionStatus, OnFriendConnectionStatus, OnFriendStatusChange, OnAsyncMessage) directly assign to callback fields without mutex protection. Other callbacks (OnFileRecv, OnFriendName, etc. at lines 3641-3835) correctly use `callbackMu.Lock()`. Additionally, callback dispatch at lines 1255-1264 and 1382-1386 reads callback pointers without locks, creating TOCTOU race conditions.
- **Impact**: Race condition when callbacks are registered from one goroutine while the iteration loop invokes them from another. Could cause nil pointer dereference, callback loss, or undefined behavior under concurrent load.
- **Closing the Gap**:
  1. Add mutex protection to all 8 unprotected callback registration methods:
     ```go
     func (t *Tox) OnFriendRequest(callback FriendRequestCallback) {
         t.callbackMu.Lock()
         defer t.callbackMu.Unlock()
         t.friendRequestCallback = callback
     }
     ```
  2. Add RLock protection to callback dispatch methods:
     ```go
     t.callbackMu.RLock()
     cb := t.friendRequestCallback
     t.callbackMu.RUnlock()
     if cb != nil { cb(...) }
     ```
  3. Validate: `go test -race -count=5 ./...`

---

## Nym Transport Listen() Support

- **Stated Goal**: Multi-network support including Nym .nym addresses for mixnet privacy.
- **Current State**: Nym transport at `transport/network_transport_impl.go:579-815` supports `Dial()` and `DialPacket()` via SOCKS5 proxy, but `Listen()` returns "Nym service hosting not supported via SOCKS5" (line 654). This is an architectural limitation—Nym's SOCKS5 proxy mode only supports outbound connections.
- **Impact**: Users can connect to Nym-hosted services but cannot host services over Nym. This limits Nym to client-only use cases, preventing peer-to-peer communication patterns that require both parties to accept connections.
- **Closing the Gap**:
  1. Nym hosting requires integration with the Nym Service Provider (SP) framework instead of SOCKS5 proxy.
  2. Implement native Nym client library integration or document that Nym Listen requires external SP configuration.
  3. Update README to clarify Nym is Dial-only via SOCKS5, with Listen requiring SP setup.
  4. Validate: Integration test with Nym SP (requires Nym network access).

---

## Lokinet UDP Transport

- **Stated Goal**: Lokinet .loki transport support for onion routing.
- **Current State**: Lokinet transport at `transport/network_transport_impl.go:817-970` supports TCP `Dial()` via SOCKS5 proxy, but `Listen()` returns "Lokinet SNApp hosting not supported via SOCKS5" (line 889) and `DialPacket()` returns "Lokinet UDP transport not supported via SOCKS5" (line 954).
- **Impact**: UDP-based protocols (including standard DHT operations) cannot use Lokinet transport. TCP-only operation limits Lokinet to connection-oriented patterns, excluding real-time communication that benefits from UDP's lower latency.
- **Closing the Gap**:
  1. Investigate Lokinet's native API for UDP support (may require lokinet daemon configuration).
  2. For Listen, SNApp hosting requires lokinet.ini configuration—document this requirement.
  3. Consider implementing UDP framing over TCP similar to Nym's `nymPacketConn` if native UDP is unavailable.
  4. Update transport documentation to clarify TCP-only limitation.
  5. Validate: Manual testing with lokinet daemon configured for SNApp.

---

## ToxAV Audio Encoding

- **Stated Goal**: ToxAV audio/video calling with Opus codec support.
- **Current State**: Audio decoding uses real pion/opus library (`av/audio/processor.go:487`), but encoding uses `SimplePCMEncoder` (`av/audio/processor.go:68`) which passes raw PCM samples instead of Opus-encoded data. Comment at line 68 states: "In future phases, this will be replaced with proper Opus encoding."
- **Impact**: 
  - Audio sent to other Tox clients will not decode correctly (expecting Opus, receiving PCM).
  - Bandwidth usage is ~10x higher than Opus-encoded audio.
  - Interoperability with qTox, uTox, and other Tox clients is broken for audio calls.
- **Closing the Gap**:
  1. Implement Opus encoding using pion/opus library's encoder (pure Go).
  2. Alternatively, integrate with libopus via CGo for higher performance.
  3. Add bit rate configuration (currently stubbed at `av/audio/codec.go:139-161`).
  4. Validate: Audio call test with qTox or reference Tox client.

---

## ToxAV Video Encoding

- **Stated Goal**: Video calling with VP8 codec support.
- **Current State**: Video handling uses `SimpleVP8Encoder` (`av/video/processor.go:71`) which passes raw YUV420 frames instead of VP8-encoded data. Comment at line 71 states: "In future phases, this will be replaced with proper VP8 encoding."
- **Impact**:
  - Video sent to other Tox clients will not decode correctly (expecting VP8, receiving raw YUV).
  - Bandwidth usage is dramatically higher (1080p raw YUV is ~3 Gbps vs ~2-5 Mbps for VP8).
  - Video calling is non-functional for interoperability.
- **Closing the Gap**:
  1. Integrate VP8 encoding library (options: pure Go implementation or CGo wrapper for libvpx).
  2. Implement keyframe/delta frame management for proper video stream.
  3. Add resolution and bitrate configuration.
  4. Validate: Video call test with reference Tox client.

---

## NAT Traversal for Symmetric NAT

- **Stated Goal**: NAT traversal techniques including UDP hole punching and port prediction.
- **Current State**: UDP hole punching is implemented in `transport/hole_puncher.go`, but README states: "Relay-based NAT traversal for symmetric NAT is planned but not yet implemented. Users behind symmetric NAT may need to use TCP relay nodes as a workaround."
- **Impact**: Users behind symmetric NAT (common in enterprise and mobile networks) cannot establish direct UDP connections. They must use TCP relay fallback, which adds latency and requires available relay nodes.
- **Closing the Gap**:
  1. Implement TURN-style relay protocol for symmetric NAT traversal.
  2. Enhance `AddRelayServer()` (`toxcore.go:1948+`) to support automatic relay selection.
  3. Add relay discovery via DHT using existing `dht/relay_storage.go` infrastructure.
  4. Validate: Test with symmetric NAT simulation (e.g., iptables MASQUERADE).

---

## Constant-Time Cryptographic Comparisons

- **Stated Goal**: Proper cryptographic security with secure coding practices.
- **Current State**: Public key comparisons at `crypto/key_rotation.go:122,128` and ID comparisons at `crypto/toxid.go:56,104-106,112` use direct `==` operator instead of `subtle.ConstantTimeCompare()`. While public keys aren't secret, this violates cryptographic best practices and could enable timing-based analysis in certain attack scenarios.
- **Impact**: Low practical risk since public keys are public, but represents deviation from defense-in-depth cryptographic coding standards. Could become relevant if comparison logic is reused for secret material.
- **Closing the Gap**:
  1. Create constant-time comparison utility:
     ```go
     func ConstantTimeEqual32(a, b [32]byte) bool {
         return subtle.ConstantTimeCompare(a[:], b[:]) == 1
     }
     ```
  2. Replace all `==` comparisons for cryptographic types with constant-time version.
  3. Document cryptographic coding standards in `crypto/doc.go`.
  4. Validate: Code review (timing analysis requires specialized tooling).

---

## Error Wrapping Consistency

- **Stated Goal**: Robust error handling with proper Go idioms.
- **Current State**: In `toxcore.go`, only 18 of 88 error returns use `fmt.Errorf("context: %w", err)` wrapping. The remaining 70 use bare `errors.New()` without error chain context, making debugging difficult when errors propagate through multiple layers.
- **Impact**: When errors occur deep in the call stack, callers cannot easily trace the origin or context. This complicates debugging and error handling in applications using toxcore-go.
- **Closing the Gap**:
  1. Define sentinel errors for common conditions:
     ```go
     var ErrAlreadyFriend = errors.New("already a friend")
     var ErrFriendNotFound = errors.New("friend not found")
     ```
  2. Wrap errors with context at each level:
     ```go
     return fmt.Errorf("add friend by address: %w", ErrAlreadyFriend)
     ```
  3. Update all 70 unwrapped error returns across toxcore.go.
  4. Validate: `go vet ./...` (partial); manual review.

---

## Scalability Limitations

- **Stated Goal**: Production-ready Tox implementation.
- **Current State**: According to internal documentation (REPORT.md), the implementation has known scalability constraints:
  - Single-threaded `Iterate()` loop with 50ms ticks (~20 ops/sec max)
  - Fixed DHT routing table (256 buckets × 8 nodes = 2,048 nodes max)
  - Single UDP socket bottleneck (~400K-500K pps max)
  - In-memory state only (no persistence/sharding)
  - 100-message per-recipient offline limit
- **Impact**: Cannot scale to global messaging scale (documented as acknowledged limitation). Suitable for development, testing, and small-to-medium deployments, not millions of concurrent users.
- **Closing the Gap**:
  1. This is acknowledged and documented—not a "gap" per se, but a scope limitation.
  2. Roadmap exists with phased approach (Phase 1: 6mo, Phase 2: 18mo).
  3. For current use: Document recommended deployment limits in README.
  4. Long-term: Implement hierarchical DHT, parallel UDP sockets, persistent state.

---

## Test Coverage for Post-Handshake Encryption

- **Stated Goal**: Comprehensive test suite ensuring correctness.
- **Current State**: `noise/handshake_test.go:85-177` (TestIKHandshakeFlow) tests handshake completion but never tests `Encrypt()`/`Decrypt()` after handshake. This test gap allowed the cipher state swap bug to go undetected.
- **Impact**: Critical cryptographic bug exists despite tests passing. Handshake "works" but subsequent encrypted communication fails.
- **Closing the Gap**:
  1. Add integration test that completes handshake then encrypts/decrypts bidirectionally:
     ```go
     func TestIKPostHandshakeEncryption(t *testing.T) {
         // Complete handshake between initiator and responder
         // Initiator encrypts message → Responder decrypts
         // Responder encrypts response → Initiator decrypts
         // Verify both directions work correctly
     }
     ```
  2. This test should fail until the cipher swap bug is fixed.
  3. Validate: New test passes after bug fix.

---

*Generated from functional audit comparing stated goals against implementation.*
