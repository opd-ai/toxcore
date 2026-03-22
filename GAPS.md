# Implementation Gaps — 2026-03-22

This document identifies gaps between toxcore-go's stated goals and current implementation, with actionable steps to close each gap.

---

## 1. Incomplete C API Bindings for Core Functions

- **Stated Goal**: "C binding annotations for cross-language use" (README) — implies comprehensive C API coverage matching c-toxcore.

- **Current State**: `capi/toxcore_c.go` exports only 7 functions:
  - `tox_new` — Create instance
  - `tox_kill` — Destroy instance  
  - `tox_bootstrap_simple` — Network bootstrap
  - `tox_iterate` — Event loop
  - `tox_iteration_interval` — Get iteration interval
  - `tox_self_get_address_size` — Address size constant
  - `hex_string_to_bin` — Hex conversion utility

  Meanwhile, `capi/toxav_c.go` exports 18 functions with full ToxAV coverage.

- **Impact**: C/C++ applications cannot use friend management, messaging, group chat, or file transfers via the C API. Only ToxAV calling functionality is accessible from C.

- **Closing the Gap**:
  1. Add `tox_self_get_address()` to retrieve full Tox ID
  2. Add `tox_friend_add()` and `tox_friend_add_norequest()` for friend management
  3. Add `tox_friend_send_message()` for messaging
  4. Add `tox_callback_friend_request()` and `tox_callback_friend_message()` callbacks
  5. Add `tox_friend_get_connection_status()` for status tracking
  6. Follow the registry pattern established in `toxav_c.go` for callback management
  7. Document all exports in generated `libtoxcore.h` header

  **Validation**: `go build -buildmode=c-shared -o libtoxcore.so ./capi/ && nm -D libtoxcore.so | grep tox_`

---

## 2. Nym Transport Listen Not Supported

- **Stated Goal**: "Multi-Network Support: ... Nym .nym" (README) — implies bidirectional communication.

- **Current State**: `transport/network_transport_impl.go:649-660`:
  ```go
  func (t *NymTransport) Listen(address string) (net.Listener, error) {
      return nil, fmt.Errorf("Nym service hosting not supported via SOCKS5: %w", ErrNymNotImplemented)
  }
  ```
  Only `Dial()` works via SOCKS5 proxy to local Nym client.

- **Impact**: Users expecting to host services on Nym network cannot do so. Only outbound connections work. The README doesn't clearly communicate this limitation.

- **Closing the Gap**:
  
  **Option A (Documentation)**: Update README to clarify:
  > **Nym .nym**: Outbound Dial only via SOCKS5 proxy (`NYM_CLIENT_ADDR`). Hosting/Listen requires Nym service provider configuration outside the scope of SOCKS5 integration.

  **Option B (Implementation)**: Integrate Nym SDK websocket client for bidirectional mixnet communication:
  1. Add `github.com/nymtech/nym` Go SDK dependency
  2. Implement `NymWebsocketTransport` using SDK's websocket client
  3. Handle Nym service provider registration for Listen
  4. This is significant work requiring Nym account/credentials

  **Validation**: `grep -A5 "Nym .nym" README.md` should show limitation clearly.

---

## 3. Lokinet Transport Listen Not Supported

- **Stated Goal**: "Multi-Network Support: ... Lokinet .loki" (README) — implies bidirectional communication.

- **Current State**: `transport/network_transport_impl.go:884-895`:
  ```go
  func (t *LokinetTransport) Listen(address string) (net.Listener, error) {
      return nil, fmt.Errorf("Lokinet SNApp hosting not supported via SOCKS5 - configure via Lokinet config")
  }
  ```
  Only `Dial()` works via SOCKS5 proxy.

- **Impact**: Users cannot host SNApps (Service Node Applications) via toxcore-go. The SOCKS5 approach only supports client-side connections.

- **Closing the Gap**:
  
  **Option A (Documentation)**: Update README to clarify:
  > **Lokinet .loki**: TCP Dial only via SOCKS5 proxy (`LOKINET_PROXY_ADDR`). SNApp hosting requires direct Lokinet configuration in `lokinet.ini`.

  **Option B (Implementation)**: Integrate Lokinet's native Go bindings if available, or document manual SNApp configuration workflow.

  **Validation**: `grep -A5 "Lokinet .loki" README.md` should show limitation clearly.

---

## 4. Tor Transport UDP Not Supported

- **Stated Goal**: "Multi-Network Support: ... Tor .onion" with "UDP Enabled" option (README).

- **Current State**: `transport/network_transport_impl.go:310-318`:
  ```go
  func (t *TorTransport) DialPacket(address string) (net.PacketConn, error) {
      return nil, fmt.Errorf("Tor UDP transport not supported: Tor primarily uses TCP; UDP over Tor is experimental and not widely supported: %w", ErrTorUnsupported)
  }
  ```
  TCP via onramp works perfectly. UDP is correctly rejected.

- **Impact**: DHT operations (which prefer UDP) cannot run over Tor. This is a protocol limitation, not a bug.

- **Closing the Gap**:
  
  Update README to clarify:
  > **Tor .onion**: Full TCP support (Listen + Dial) via onramp library. UDP not supported (inherent Tor limitation). Set `UDPEnabled = false` when using Tor-only mode.

  **Validation**: `grep -B2 -A5 "Tor .onion" README.md`

---

## 5. Documentation of Proxy Limitations

- **Stated Goal**: "Proxy Support" section describes SOCKS5 with `UDPProxyEnabled` for UDP traffic routing.

- **Current State**: README states "The Tor network itself does not support UDP, so even with a SOCKS5 proxy to Tor, UDP traffic cannot be tunneled through Tor's onion routing."

  However, users may not understand that:
  1. HTTP proxies only support TCP
  2. Only SOCKS5 proxies support UDP ASSOCIATE
  3. UDP proxy requires `UDPProxyEnabled: true` flag

- **Impact**: Users may configure proxies incorrectly and expose real IP via UDP.

- **Closing the Gap**:
  
  Add explicit warning box in README:
  ```markdown
  > ⚠️ **UDP Proxy Warning**: When using a proxy, UDP traffic is only protected if:
  > 1. Proxy type is SOCKS5 (HTTP proxies are TCP-only)
  > 2. `UDPProxyEnabled: true` is set in ProxyOptions
  > 3. The SOCKS5 proxy supports UDP ASSOCIATE (RFC 1928)
  > 
  > Without these conditions, UDP traffic (including DHT) bypasses the proxy.
  ```

  **Validation**: Review `ProxyOptions` struct documentation completeness.

---

## 6. WAL Recover Function Complexity

- **Stated Goal**: Project conventions emphasize maintainability and testability.

- **Current State**: `async/wal.go:Recover()` has:
  - Cyclomatic complexity: 12
  - Overall complexity: 17.6 (highest in codebase)
  - 55 lines of code
  - Multiple responsibilities: file reading, checksum validation, entry parsing, state reconstruction

- **Impact**: Difficult to unit test individual recovery scenarios. Higher risk of bugs in edge cases.

- **Closing the Gap**:
  
  Refactor into smaller functions:
  ```go
  func (w *WriteAheadLog) Recover() error {
      entries, err := w.readAllEntries()
      if err != nil {
          return fmt.Errorf("read entries: %w", err)
      }
      
      validEntries := w.filterValidEntries(entries)
      return w.applyEntries(validEntries)
  }
  
  func (w *WriteAheadLog) readAllEntries() ([]WALEntry, error) { ... }
  func (w *WriteAheadLog) filterValidEntries(entries []WALEntry) []WALEntry { ... }
  func (w *WriteAheadLog) applyEntries(entries []WALEntry) error { ... }
  ```

  **Validation**: `go-stats-generator analyze . --format json | jq '.functions[] | select(.name=="Recover") | .complexity'`

---

## 7. toxcore.go File Size

- **Stated Goal**: Clean, idiomatic Go with proper package organization.

- **Current State**: `toxcore.go` is 2855 lines with 218 functions — maintenance burden score 8.10 (highest).

- **Impact**: Difficult to navigate, understand, and maintain. New contributors face steep learning curve.

- **Closing the Gap**:
  
  Split into focused files:
  ```
  toxcore.go          (~500 lines) - Core Tox struct, New(), Kill(), Iterate()
  toxcore_self.go     (~300 lines) - SelfGet*, SelfSet* methods
  toxcore_friend.go   (~400 lines) - Friend management, AddFriend, DeleteFriend
  toxcore_callbacks.go(~400 lines) - OnFriendRequest, OnFriendMessage, etc.
  toxcore_bootstrap.go(~200 lines) - Bootstrap, connection management
  toxcore_state.go    (~300 lines) - GetSavedata, Load, persistence
  ```

  **Validation**: `wc -l toxcore*.go` — each file should be <500 lines.

---

## 8. Standard Library Package Name Collisions

- **Stated Goal**: Idiomatic Go following best practices.

- **Current State**: 
  - `net/` package collides with standard library `net`
  - `testing/` package collides with standard library `testing`

- **Impact**: Requires awkward qualified imports, confusing for new contributors.

- **Closing the Gap**:
  
  Rename packages:
  - `net/` → `toxnet/` or `netutil/`
  - `testing/` → `testutil/` or `simulation/`

  Update all imports across codebase:
  ```bash
  find . -name "*.go" -exec sed -i 's|"github.com/opd-ai/toxcore/net"|"github.com/opd-ai/toxcore/toxnet"|g' {} \;
  find . -name "*.go" -exec sed -i 's|"github.com/opd-ai/toxcore/testing"|"github.com/opd-ai/toxcore/testutil"|g' {} \;
  mv net toxnet
  mv testing testutil
  ```

  **Validation**: `go build ./...` should succeed without import conflicts.

---

## 9. Unreferenced Functions (Potential Dead Code)

- **Stated Goal**: Clean, maintainable codebase.

- **Current State**: 159 functions detected as unreferenced by `go-stats-generator`.

- **Impact**: Code bloat, confusion about what's actually used, maintenance burden.

- **Closing the Gap**:
  
  1. Generate full list:
     ```bash
     go-stats-generator analyze . --format json | jq '.maintenance.dead_code.unreferenced[]' > unreferenced.txt
     ```
  
  2. Categorize each function:
     - **Deprecated**: Add `// Deprecated:` comment and timeline for removal
     - **Future use**: Add `// TODO:` comment explaining planned integration
     - **Actually dead**: Remove from codebase
     - **Test helpers**: Move to `*_test.go` files if only used in tests
  
  3. For exported functions that should remain but aren't used internally, add examples in `example_test.go` files.

  **Validation**: Re-run analysis and verify reduction in unreferenced count.

---

## 10. Noise Protocol Opt-In Not Documented

- **Stated Goal**: "Noise Protocol Framework Integration" for enhanced security.

- **Current State**: Noise-IK is implemented in `noise/` and `transport/noise_transport.go`, but it's **opt-in**. Users must explicitly wrap transports with `NewNoiseTransport()`. Default transports use legacy encryption.

- **Impact**: Users may assume Noise is automatic and miss the configuration step. Security expectations may not match reality.

- **Closing the Gap**:
  
  Add to README:
  ```markdown
  > **Note**: Noise-IK requires explicit configuration. Wrap your transport:
  > ```go
  > noiseTransport, err := transport.NewNoiseTransport(udpTransport, privateKey)
  > ```
  > Default transports use legacy Tox encryption for c-toxcore compatibility.
  ```

  **Validation**: README should clearly state Noise is opt-in.

---

## Summary

| Gap | Severity | Effort | Priority |
|-----|----------|--------|----------|
| Incomplete C API (toxcore) | HIGH | Medium | 1 |
| Nym Listen not supported | HIGH | Documentation: Low / Implementation: High | 2 |
| Lokinet Listen not supported | HIGH | Documentation: Low | 3 |
| Proxy UDP warning | MEDIUM | Low | 4 |
| WAL complexity | MEDIUM | Medium | 5 |
| toxcore.go size | MEDIUM | Medium | 6 |
| Package name collisions | LOW | Low | 7 |
| Dead code cleanup | LOW | Medium | 8 |
| Noise opt-in docs | LOW | Low | 9 |
| Tor UDP docs | LOW | Low | 10 |

**Recommended Order**: Start with documentation gaps (2, 3, 4, 9, 10) as quick wins, then address C API (1), then code quality (5, 6, 7, 8).
