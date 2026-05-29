# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-29

## Project Profile
A pure-Go Tox implementation targeting peer-to-peer encrypted messaging, group chat, file transfer, AV calling, async offline messaging, and multi-network transport (UDP/TCP/Tor/I2P/Lokinet/Nym). Primary users are application developers embedding the library and operators running bootstrap/discovery paths. Critical paths: transport lifecycle, async messaging lifecycle, DHT bootstrap/discovery, and public exported constructors.

## Audit Scope
- Packages audited: all 57 packages returned by `go list ./...`
- Total functions inspected (stats baseline): 4067
- High-risk structural set (cyclomatic >15 OR >50 lines): 42 functions manually spot-checked
- Baseline commands executed:
  - `go test -race ./...`
  - `go vet ./...`
  - `go-stats-generator analyze . --skip-tests --format json --sections functions,packages,documentation,duplication,patterns,interfaces,structs`

## Coverage Log
| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| / | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /av | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /av/audio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /av/rtp | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /av/video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /bootstrap | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /bootstrap/nodes | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /cmd/gen-bootstrap-nodes | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/address_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/address_parser_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/api_fix_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/async_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/async_obfuscation_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/audio_effects_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/audio_streaming_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/av_quality_monitor | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/bootstrap_server_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/color_temperature_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/common | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/file_transfer_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/friend_callbacks_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/friend_loading_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/integration_test | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/multi_transport_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/noise_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/privacy_networks | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/proxy_example | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/qtox_integration | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/tor_transport_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/toxav_audio_call | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/toxav_basic_call | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/toxav_call_control_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/toxav_effects_processing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/toxav_integration | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/toxav_video_call | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/version_negotiation_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /examples/vp8_codec_demo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /factory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /limits | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /real | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /toxnet/example | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /toxnet/examples/packet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| /transport/internal/addressing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |


## Goal-Achievement Summary
| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| Asynchronous offline messaging with forward secrecy | ⚠️ | F-M1, F-L1 |
| DHT-based peer discovery (LAN/mDNS fallback) | ⚠️ | F-M2, F-M3 |
| Robust multi-network transport lifecycle | ⚠️ | F-M4 |
| Production-safe public API behavior | ⚠️ | F-H1 |
| Core crypto/messaging/AV behavior | ✅ | None found above LOW |

## Findings

### CRITICAL
- [ ] None confirmed.

### HIGH
- [ ] **F-H1: Public bootstrap constructors panic on invalid input** — `/tmp/workspace/opd-ai/toxcore/dht/bootstrap.go:126`, `:159`, `:203` — [API contract / error-handling] — passing `nil` `routingTable` to exported constructors triggers `panic`, crashing the process instead of returning an error; reachable via direct calls to `NewBootstrapManager*` from embedding applications — **Remediation:** change these constructors to return `(*BootstrapManager, error)` (or add safe wrapper constructors) and replace panic branches with explicit validation errors; validate with `go test -race ./dht/...` and call-site compile checks.

### MEDIUM
- [ ] **F-M1: AsyncManager cannot safely restart after Stop** — `/tmp/workspace/opd-ai/toxcore/async/manager.go:188-206`, `:233`, `:493`, `:538` — [concurrency/lifecycle] — `Stop()` closes `am.stopChan`, but `Start()` does not recreate it; subsequent `Start()` spawns loops that immediately select closed channel and exit, silently disabling async retrieval/discovery loops — **Remediation:** recreate `stopChan` on each successful `Start()` transition and gate repeated starts with explicit lifecycle state; validate with `go test -race ./async/...` plus a Start→Stop→Start regression test.
- [ ] **F-M2: LANDiscovery cannot restart after Stop** — `/tmp/workspace/opd-ai/toxcore/dht/local_discovery.go:67-92`, `:122`, `:165`, `:337` — [concurrency/lifecycle] — `Stop()` permanently closes `ld.stopChan`; `Start()` does not reinitialize it, so restarted broadcast/receive goroutines exit immediately and LAN discovery silently stops working — **Remediation:** recreate `stopChan` during `Start()` when transitioning from stopped state and add restart lifecycle tests; validate with `go test -race ./dht/...`.
- [ ] **F-M3: MDNSDiscovery restart path is broken after Stop** — `/tmp/workspace/opd-ai/toxcore/dht/mdns_discovery.go:73`, `:78`, `:124-133`, `:244-246`, `:277`, `:297`, `:323` — [concurrency/lifecycle] — `Stop()` cancels context and closes `stopChan`, but `Start()` does not rebuild either; background loops terminate immediately on restart, leaving mDNS discovery non-functional while `Start()` returns success — **Remediation:** recreate both lifecycle controls (`ctx/cancel`, `stopChan`) on each fresh `Start()` and add Start→Stop→Start coverage; validate with `go test -race ./dht/...`.
- [ ] **F-M4: NAT periodic detection Start is non-idempotent and non-restartable** — `/tmp/workspace/opd-ai/toxcore/transport/nat.go:106`, `:180-210`, `:221` — [concurrency/resource lifecycle] — `NewNATTraversal()` auto-starts periodic detection, but calling `StartPeriodicDetection()` again adds extra goroutines; after `StopPeriodicDetection()` closes channel, later starts exit immediately — producing duplicate work before stop and disabled detection after restart — **Remediation:** add started/stopped state guarding, recreate stop channel on restart, and make start idempotent; validate with `go test -race ./transport/...`.

### LOW
- [ ] **F-L1: PreKeyDHT auto-refresh lifecycle is one-shot** — `/tmp/workspace/opd-ai/toxcore/async/prekey_dht.go:104`, `:382-398`, `:406` — [concurrency/lifecycle] — `StopAutoRefresh()` closes `stopRefresh`, while `StartAutoRefresh()` does not reinitialize or guard against repeated starts; restart attempts silently do nothing and repeated starts can create duplicate refresh workers before stop — **Remediation:** protect Start/Stop with lifecycle state and recreate `stopRefresh` when restarting; validate with `go test -race ./async/...`.
- [ ] **F-L2: Dependency risk exposure in security-sensitive modules** — `/tmp/workspace/opd-ai/toxcore/go.mod:17`, `:19` — [security/dependency hygiene] — online advisory review indicates multiple 2026 advisories reported against `golang.org/x/crypto` and `golang.org/x/net`; project uses these dependencies across transport/crypto paths (reachability not fully proven from local offline data) — **Remediation:** run `govulncheck ./...` in CI with network access, upgrade to patched versions, and retest with `go test -race ./...`.

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total functions | 4067 |
| Functions above complexity 15 | 2 |
| Avg cyclomatic complexity | 2.43 |
| Doc coverage | 93.32% |
| Duplication ratio | 0.47% |
| Test pass rate | 34/34 (packages with tests) |
| go vet warnings | 0 |

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|----------------|
| `transport/nat.go:27` init panic on fallback address parse | Address is compile-time constant (`203.0.113.1:0`), parse-only path is deterministic; not practically attacker-controlled. |
| `dht/mdns_discovery.go:44-47` init panics | Compile-time multicast literals are parsed at init; no external data flow into these calls. |
| `async/retrieval_scheduler.go:149` ignored retrieval error in cover mode | Intentional cover-traffic behavior documented in adjacent comments; result intentionally discarded to preserve indistinguishability. |

## Remaining Scope (if session ended before completion)
| Package | Status | Notes |
|---------|--------|-------|
| None | Complete | Full package list covered in Coverage Log; no additional unaudited package remains in this pass. |
