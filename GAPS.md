# Implementation Gaps — 2026-04-07

This document identifies gaps between the stated goals in the README and the actual implementation status. Each gap includes the stated goal, current state, user impact, and remediation path.

---

## VP8 Inter-Frame Encoding

- **Stated Goal**: The README states "Video Calling: Video transmission with configurable quality" and lists VP8 codec support. The Known Limitations section acknowledges the I-frame-only limitation.

- **Current State**: The `RealVP8Encoder` in av/video/processor.go:63-126 produces only key frames (I-frames) using the opd-ai/vp8 library. The `LibVPXEncoder` in av/video/encoder_cgo.go:30-140 has 5 TODO comments indicating the libvpx integration was never completed. P-frame and B-frame encoding are not functional.

- **Impact**: Video bandwidth usage is 5-10x higher than typical VP8 implementations. Users in bandwidth-constrained environments (mobile networks, limited connections) may experience poor video quality or excessive data consumption. WebRTC interoperability may be affected since receiving clients may expect standard temporal prediction.

- **Closing the Gap**: 
  1. Complete the libvpx CGo integration when xlab/libvpx-go provides stable bindings
  2. Alternatively, contribute P-frame support to opd-ai/vp8 pure Go encoder
  3. Document recommended settings for bandwidth-constrained scenarios (lower resolution, reduced frame rate)
  4. Consider adding bitrate adaptation that automatically reduces quality when bandwidth is limited

---

## Lokinet Listen Support

- **Stated Goal**: README Multi-Network Support table shows Lokinet .loki with checkmarks for Dial but acknowledges "Listen support is low priority and blocked by immature Lokinet SDK."

- **Current State**: transport/lokinet_transport_impl.go:77-80 explicitly states `Listen()` returns `ErrNotSupported`. Only SOCKS5-based Dial() is implemented. Users cannot host services accessible via .loki addresses programmatically.

- **Impact**: Applications requiring inbound Lokinet connections (servers, peer-to-peer nodes accepting connections) cannot use the programmatic API. Workaround requires manual Lokinet SNApp configuration outside the Go application.

- **Closing the Gap**:
  1. Monitor Lokinet SDK development for stable Go bindings or REST API
  2. Document the SNApp configuration workaround in docs/LOKINET_MANUAL.md
  3. Provide example showing hybrid approach: Lokinet SNApp + IP transport binding
  4. Consider contributing to Lokinet Go SDK development

---

## Nym Listen Support

- **Stated Goal**: README lists Nym .nym in the multi-network support table with acknowledgment that "Listen support is low priority and blocked by immature Nym SDK."

- **Current State**: transport/nym_transport_impl.go implements only SOCKS5-based Dial(). Users can connect to Nym network destinations but cannot host services accessible via .nym addresses.

- **Impact**: Similar to Lokinet, applications requiring inbound Nym connections cannot function. The Nym mixnet's anonymity properties are only available for outbound traffic.

- **Closing the Gap**:
  1. Monitor Nym SDK for stable service provider API
  2. Document requirement for local Nym native client in SOCKS5 mode
  3. Add example in docs/NYM_TRANSPORT.md showing client setup
  4. Consider WebSocket-based integration if Nym exposes such interface

---

## UDP Over Privacy Networks

- **Stated Goal**: The README mentions "UDP transport" support generally, but privacy network documentation indicates UDP is not supported over Tor, I2P, Lokinet, or Nym.

- **Current State**: 
  - Tor: transport/tor_transport_impl.go:23 explicitly states "UDP over Tor is experimental"
  - I2P: DialPacket() is implemented (i2p_transport_impl.go:24) but may not work reliably
  - Lokinet/Nym: DialPacket() returns ErrNotSupported

- **Impact**: DHT operations typically use UDP. Users routing all traffic through privacy networks may find DHT peer discovery non-functional, forcing TCP-only operation which affects NAT traversal and peer-to-peer connectivity.

- **Closing the Gap**:
  1. Clearly document in MULTINETWORK.md that privacy networks are TCP-only
  2. Implement TCP-based DHT fallback mode when UDP is unavailable
  3. Add integration tests verifying DHT works in TCP-only mode
  4. Consider TCP relay-based peer discovery for privacy network users

---

## LibVPX CGo Encoder Integration

- **Stated Goal**: av/video/encoder_cgo.go defines LibVPXEncoder type with comments indicating intended libvpx integration.

- **Current State**: Five TODO comments at lines 60, 92, 109, 121, 132 indicate the encoder was never implemented. The Encode() method at line 92 contains `// TODO: Implement actual encoding when xlab/libvpx-go is added` and returns passthrough frames.

- **Impact**: Users expecting high-quality VP8 encoding with full feature support (P-frames, configurable quality, rate control) will get only the RealVP8Encoder fallback with I-frame-only output.

- **Closing the Gap**:
  1. Either complete the libvpx integration or remove LibVPXEncoder entirely
  2. If removing, update documentation to clarify RealVP8Encoder is the only option
  3. Add build tag `libvpx` to conditionally compile CGo encoder when dependencies are available
  4. Update TOXAV_BENCHMARKING.md with comparative benchmarks

---

## Test Suite Timeout in toxnet Package

- **Stated Goal**: README claims "Comprehensive test suite (~63% statement coverage)" and CI runs tests with race detection.

- **Current State**: `go test -race ./toxnet/...` hangs in TestToxListener (toxnet/net_test.go:112) during async messaging initialization. The test creates a Tox instance that triggers WAL recovery (async/wal.go:227-439) which appears to run indefinitely.

- **Impact**: The toxnet package cannot be fully tested with race detection. CI may need to exclude this package or use shorter timeouts, reducing test coverage confidence.

- **Closing the Gap**:
  1. Add context.Context with timeout to async initialization in toxcore.go:520
  2. Add test-specific configuration to disable or mock async messaging in unit tests
  3. Ensure WAL recovery has bounded operation time
  4. Add `-timeout` flag to CI test commands for early failure detection

---

## CVE-2018-25022 Mitigation Status

- **Stated Goal**: docs/SECURITY_AUDIT_REPORT.md documents various security measures but does not specifically address CVE-2018-25022 (IP disclosure via onion routing in c-toxcore).

- **Current State**: The DHT implementation in dht/handler.go may be vulnerable to the same attack vector if NAT ping packets are routed without proper filtering. No explicit mitigation is documented.

- **Impact**: If the vulnerability exists, attackers could potentially discover a user's real IP address using only their Tox ID, undermining the privacy properties the protocol claims.

- **Closing the Gap**:
  1. Audit dht/handler.go packet routing against CVE-2018-25022 description
  2. Implement packet type filtering for onion-routed messages if vulnerable
  3. Document the analysis and mitigation in SECURITY_AUDIT_REPORT.md
  4. Add integration tests that verify IP non-disclosure through DHT operations

---

## Group Chat Cross-Client Interoperability

- **Stated Goal**: README claims "Group Chat Functionality ✅ *Fully Implemented*" with DHT-based discovery.

- **Current State**: group/chat.go (2032 lines) implements group chat, but conference invitations use plain text format (toxcore_conference.go:62: `fmt.Sprintf("CONF_INVITE:%d:%s", ...)`). This format may not be compatible with c-toxcore or other Tox clients.

- **Impact**: Users attempting to create groups with mixed client ecosystems (toxcore-go + qTox/uTox) may find invitations don't work cross-client.

- **Closing the Gap**:
  1. Review c-toxcore group invitation packet format
  2. Implement compatible serialization in group invitation packets
  3. Add integration tests with c-toxcore reference implementation
  4. Document any known interoperability limitations

---

## Async Storage Node Discovery

- **Stated Goal**: README describes "Distributed Storage: No single point of failure - messages distributed across multiple storage nodes" and "DHT Integration: Storage nodes discovered through existing DHT network."

- **Current State**: async/manager.go and async/storage.go implement storage node functionality, but DHT-based discovery of other storage nodes is not clearly demonstrated in tests. The automatic storage participation relies on local initialization.

- **Impact**: Users may need to manually configure or bootstrap storage node connections. Without automatic discovery, the "distributed" property depends on pre-configured node lists.

- **Closing the Gap**:
  1. Add explicit DHT announcements for storage node availability
  2. Implement storage node discovery queries via DHT
  3. Add integration tests demonstrating multi-node message relay
  4. Document manual storage node configuration as fallback

---

## Message Delivery Guarantees

- **Stated Goal**: README Limitations section states "No Delivery Guarantees: Best-effort delivery, messages may be lost if all storage nodes fail."

- **Current State**: This limitation is accurately documented, but there's no mechanism for users to detect or recover from message loss. The async client has no delivery receipt or acknowledgment system visible in the public API.

- **Impact**: Applications requiring reliable messaging (important notifications, transaction confirmations) cannot detect whether messages were delivered.

- **Closing the Gap**:
  1. Implement optional delivery receipts for async messages
  2. Add callback for delivery confirmation or failure notification
  3. Document recommended patterns for applications requiring reliability
  4. Consider adding message retry with exponential backoff

---

## Summary Statistics

| Gap Category | Count | Severity |
|--------------|-------|----------|
| Missing functionality | 4 | HIGH |
| Partial implementation | 3 | MEDIUM |
| Documentation gaps | 2 | LOW |
| Test infrastructure | 1 | MEDIUM |

### Priority Order for Remediation

1. **Test suite timeout** — Blocks CI/testing (CRITICAL)
2. **CVE-2018-25022 audit** — Security concern (HIGH)
3. **VP8 P-frame support** — User-facing quality issue (HIGH)
4. **Lokinet/Nym Listen** — Feature completeness (MEDIUM)
5. **LibVPX cleanup** — Code hygiene (MEDIUM)
6. **Group chat interop** — Ecosystem compatibility (MEDIUM)
7. **UDP documentation** — User expectation management (LOW)
8. **Storage node discovery** — Feature completeness (LOW)
9. **Delivery guarantees** — Feature enhancement (LOW)
