# AUDIT — 2026-03-22

## Project Goals

**toxcore-go** is a pure Go implementation of the Tox Messenger protocol claiming to provide:

1. **Pure Go Implementation** — No CGo dependencies for core functionality
2. **Complete Tox Protocol** — Friend management, messaging, file transfers, group chat
3. **Multi-Network Support** — IPv4/IPv6, Tor, I2P, Nym, Lokinet transports
4. **Noise Protocol Security** — Noise-IK pattern with forward secrecy
5. **ToxAV Audio/Video** — Opus audio and VP8 video calling
6. **Asynchronous Messaging** — Offline message delivery with forward secrecy
7. **Identity Obfuscation** — Cryptographic pseudonyms for storage nodes
8. **C API Bindings** — Cross-language interoperability

**Target Audience:** Developers building privacy-focused communication applications, researchers working on decentralized protocols, and contributors to the Tox ecosystem.

---

## Goal-Achievement Summary

| Goal | Status | Evidence |
|------|--------|----------|
| Pure Go (no CGo) | ✅ Achieved | Core packages have no CGo; capi/ uses CGo for bindings only |
| Friend Management | ✅ Achieved | `toxcore.go:2243-2833` — AddFriend, GetFriends, DeleteFriend |
| Real-time Messaging | ✅ Achieved | `toxcore.go:2378-2402` — SendFriendMessage with types |
| File Transfers | ✅ Achieved | `file/` package + `toxcore.go:3336-3594` |
| Group Chat | ✅ Achieved | `group/chat.go` — Full DHT-based group discovery |
| IPv4/IPv6 Transport | ✅ Achieved | `transport/udp.go`, `transport/tcp.go` |
| Tor Transport (TCP) | ✅ Achieved | `transport/network_transport_impl.go:159-339` |
| I2P Transport | ✅ Achieved | `transport/network_transport_impl.go:376-577` |
| Nym Transport (Dial) | ⚠️ Partial | Dial works; Listen unsupported by architecture |
| Lokinet Transport (TCP) | ⚠️ Partial | TCP Dial works; UDP unsupported |
| Noise-IK Encryption | ⚠️ Partial | Implementation exists but has critical cipher bug |
| Forward Secrecy | ✅ Achieved | `async/forward_secrecy.go:131-258` — Pre-key system |
| ToxAV Audio | ✅ Achieved | `av/audio/` — Real Opus decoding via pion/opus |
| ToxAV Video | ⚠️ Partial | YUV420 handling; VP8 encoding is passthrough |
| Async Messaging | ✅ Achieved | `async/manager.go:114-509` — Complete offline flow |
| Identity Obfuscation | ✅ Achieved | `async/obfs.go:62-376` — HKDF pseudonyms + AES-GCM |
| C API Bindings | ✅ Achieved | `capi/toxcore_c.go`, `capi/toxav_c.go` |
| State Persistence | ✅ Achieved | `toxcore.go:2984+` — GetSavedata/NewFromSavedata |
| DHT Routing | ✅ Achieved | `dht/routing.go` — K-bucket implementation |
| Bootstrap Connectivity | ✅ Achieved | `dht/bootstrap.go:300-343` — Parallel with fallback |
| LAN Discovery | ✅ Achieved | `dht/local_discovery.go:163-309` — UDP broadcast |
| NAT Traversal | ⚠️ Partial | UDP hole punching done; relay for symmetric NAT planned |

**Overall: 17/21 goals fully achieved (81%), 4 partial**

---

## Findings

### CRITICAL

- [ ] **Noise-IK Cipher State Swap** — `noise/handshake.go:262-263` — Initiator's sendCipher and recvCipher are swapped after ReadMessage, breaking all post-handshake encryption/decryption. Responder (lines 234-235) is correct. **Remediation:** Swap the variable assignments at lines 262-263:
  ```go
  // CURRENT (WRONG):
  ik.recvCipher = recvCipher
  ik.sendCipher = sendCipher
  // FIXED:
  ik.sendCipher = recvCipher  // First return is for sending
  ik.recvCipher = sendCipher  // Second return is for receiving
  ```
  **Validation:** `go test -race ./noise/... -run TestIKHandshake`

- [ ] **Race Condition in Callback Registration** — `toxcore.go:2165-2238` — Eight callback registration methods (OnFriendRequest, OnFriendMessage, OnFriendMessageDetailed, OnFriendStatus, OnConnectionStatus, OnFriendConnectionStatus, OnFriendStatusChange, OnAsyncMessage) write to callback fields without mutex protection, while other callbacks (OnFileRecv, OnFriendName, etc. at lines 3641-3835) properly use `callbackMu.Lock()`. **Remediation:** Add mutex protection to all unprotected callback registrations:
  ```go
  func (t *Tox) OnFriendRequest(callback FriendRequestCallback) {
      t.callbackMu.Lock()
      defer t.callbackMu.Unlock()
      t.friendRequestCallback = callback
  }
  ```
  Apply same pattern to lines 2173, 2181, 2188, 2209, 2217, 2226, 2234.
  **Validation:** `go test -race ./... -count=3`

### HIGH

- [ ] **Callback Invocation Without Lock** — `toxcore.go:1255-1264, 1382-1386` — Callback dispatch reads callback function pointers without holding `callbackMu`, creating TOCTOU race conditions. **Remediation:** Use RLock when reading callbacks:
  ```go
  func (t *Tox) dispatchFriendMessage(...) {
      t.callbackMu.RLock()
      cb := t.simpleFriendMessageCallback
      t.callbackMu.RUnlock()
      if cb != nil {
          cb(friendID, message)
      }
  }
  ```
  **Validation:** `go test -race ./... -count=5`

- [ ] **Non-Constant-Time Public Key Comparisons** — `crypto/key_rotation.go:122,128` and `crypto/toxid.go:56,104-106,112` — Public key and checksum comparisons use direct `==` operator instead of `subtle.ConstantTimeCompare()`. While public keys aren't secret, this violates cryptographic best practices. **Remediation:** Create and use constant-time comparison helper:
  ```go
  func ConstantTimeEqual32(a, b [32]byte) bool {
      return subtle.ConstantTimeCompare(a[:], b[:]) == 1
  }
  ```
  **Validation:** Code review; no runtime test for timing attacks

- [ ] **ToxAV Opus Encoding is PCM Passthrough** — `av/audio/processor.go:68` — Audio encoding passes raw PCM instead of Opus-encoded data. This is documented as "Phase 2" but affects interoperability with other Tox clients. **Remediation:** Implement proper Opus encoding using pion/opus encoder or integrate with libopus via CGo. **Validation:** Manual testing with qTox client for audio quality.

### MEDIUM

- [ ] **Unused Error Variable** — `transport/network_transport_impl.go:19` — `ErrNymNotImplemented` declared but never used in code. **Remediation:** Remove the unused variable or use it in the Nym Listen() method's error return. **Validation:** `go vet ./transport/...`

- [ ] **Concrete UDPConn Type in Internal Code** — `transport/hole_puncher.go:19` — Uses concrete `*net.UDPConn` instead of `net.PacketConn` interface. While internal, this violates the project's abstraction conventions. **Remediation:** Change to `conn net.PacketConn` and use interface methods. **Validation:** `go build ./transport/...`

- [ ] **Inconsistent Error Wrapping** — `toxcore.go` — Only 18 of 88 errors use `fmt.Errorf("context: %w", err)` wrapping; 70 use bare `errors.New()` without chain context. **Remediation:** Standardize error returns to use wrapping pattern for better debugging:
  ```go
  // Instead of:
  return errors.New("already a friend")
  // Use:
  return fmt.Errorf("add friend: %w", ErrAlreadyFriend)
  ```
  **Validation:** `go vet ./...` (partial); manual review

- [ ] **FindNode High Cyclomatic Complexity** — `dht/iterative_lookup.go:133-251` — FindNode function has complexity score of 24.6 (threshold: 15). The function was flagged by go-stats-generator as highest complexity in codebase. **Remediation:** Extract helper functions for node selection (lines 170-177), parallel querying (lines 180-192), and response handling (lines 195-228). **Validation:** `go-stats-generator analyze . --format json | jq '.functions[] | select(.name=="FindNode")'`

- [ ] **Video VP8 Encoding is Passthrough** — `av/video/processor.go:71` — Video encoding passes raw YUV420 frames instead of VP8-encoded data. Documented as "Phase 3" but affects bandwidth and interoperability. **Remediation:** Integrate VP8 encoding via pure Go library or CGo wrapper. **Validation:** Manual testing with video calling.

### LOW

- [ ] **Package Name Collisions** — `net/`, `testing/` — Package names collide with Go standard library, requiring import aliases. **Remediation:** Rename to `toxnet/` and `toxtesting/` or use import paths that don't conflict. **Validation:** Code review.

- [ ] **File Name Stuttering** — `friend/friend.go`, `limits/limits.go` — File names repeat package name. **Remediation:** Rename to `friend/manager.go`, `limits/constants.go` or similar. **Validation:** Code review.

- [ ] **doFriendConnections Complexity** — `toxcore.go:42` — Second highest complexity (15.0) after FindNode. **Remediation:** Extract friend state machine logic into helper methods. **Validation:** `go-stats-generator analyze .`

- [ ] **Missing Post-Handshake Encryption Test** — `noise/handshake_test.go:85-177` — TestIKHandshakeFlow completes handshake but never tests Encrypt/Decrypt after handshake, which would have caught the cipher swap bug. **Remediation:** Add test case that performs handshake then encrypts/decrypts bidirectionally. **Validation:** New test should fail until cipher bug is fixed, then pass.

---

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total Lines of Code | 34,462 |
| Total Functions | 890 |
| Total Methods | 2,290 |
| Total Structs | 331 |
| Total Interfaces | 36 |
| Total Packages | 24 |
| Total Files | 197 |
| Average Function Length | 13.4 lines |
| Average Complexity | 3.6 |
| High Complexity Functions (>10) | 1 (FindNode: 24.6) |
| Functions >50 lines | 27 (0.8%) |
| Clone Pairs Detected | 28 |
| Duplicated Lines | 473 |
| Duplication Ratio | 0.67% |
| Naming Score | 0.98 |
| Test Files | 206 (52.8% ratio) |
| `go vet` Issues | 0 |
| `go test -race` Status | PASS |

---

## Test Results

```
$ go test -tags nonet -race ./...
ok      github.com/opd-ai/toxcore               12.536s
ok      github.com/opd-ai/toxcore/async         (cached)
ok      github.com/opd-ai/toxcore/av            (cached)
ok      github.com/opd-ai/toxcore/av/audio      (cached)
ok      github.com/opd-ai/toxcore/av/rtp        (cached)
ok      github.com/opd-ai/toxcore/av/video      (cached)
ok      github.com/opd-ai/toxcore/bootstrap     (cached)
ok      github.com/opd-ai/toxcore/capi          (cached)
ok      github.com/opd-ai/toxcore/crypto        (cached)
ok      github.com/opd-ai/toxcore/dht           (cached)
... (all packages pass)
```

---

## Security Assessment Summary

| Area | Status | Notes |
|------|--------|-------|
| NaCl/Box Encryption | ✅ Secure | Proper authenticated encryption |
| Memory Wiping | ✅ Secure | Uses `subtle.XORBytes()` + `runtime.KeepAlive()` |
| Random Generation | ✅ Secure | Only `crypto/rand` used, no `math/rand` |
| Replay Protection | ✅ Implemented | Nonce tracking with timestamp freshness |
| Rekey Threshold | ✅ Implemented | 2^32 messages before mandatory rekey |
| Forward Secrecy | ✅ Implemented | Pre-key system with consumption tracking |
| Identity Obfuscation | ✅ Implemented | HKDF pseudonyms + AES-GCM payloads |
| Noise-IK Handshake | ❌ **CRITICAL BUG** | Cipher state swap breaks initiator |
| Callback Thread Safety | ❌ **RACE** | 8 callbacks lack mutex protection |
| Constant-Time Comparison | ⚠️ Partial | Public key comparison uses `==` |

---

## Dependency Assessment

| Dependency | Version | Status |
|------------|---------|--------|
| github.com/flynn/noise | v1.1.0 | ✅ No known vulnerabilities |
| github.com/go-i2p/onramp | v0.33.92 | ✅ Maintained, no advisories |
| github.com/pion/opus | (untagged) | ✅ Active development |
| github.com/pion/rtp | v1.8.22 | ✅ No known vulnerabilities |
| github.com/sirupsen/logrus | v1.9.4 | ✅ Stable, widely used |
| golang.org/x/crypto | v0.48.0 | ✅ Current, maintained by Go team |
| golang.org/x/net | v0.50.0 | ✅ Current |
| golang.org/x/sys | v0.41.0 | ✅ Current |

---

*Generated by functional audit against stated project goals.*
*Analysis tool: go-stats-generator v1.0.0*
