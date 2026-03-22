# AUDIT — 2026-03-22

## Project Goals

**toxcore-go** is a pure Go implementation of the Tox peer-to-peer encrypted messaging protocol. According to its README and documentation, the project promises:

1. **Pure Go implementation** with no CGo dependencies (CGo isolated to optional `capi/` package)
2. **Multi-Network Transport Support**: IPv4/IPv6, Tor .onion, I2P .b32.i2p, Nym .nym, Lokinet .loki
3. **Noise Protocol Framework** (IK pattern) for enhanced handshake security
4. **Forward Secrecy** with epoch-based pre-key rotation (6-hour cycles)
5. **Identity Obfuscation** to protect metadata from storage nodes
6. **Message Padding** (256B, 1024B, 4096B) to resist traffic analysis
7. **DHT-based peer discovery** with k-bucket routing
8. **Complete friend management** with callbacks and state persistence
9. **Group chat** functionality
10. **File transfers** with progress tracking
11. **ToxAV** audio/video calling with Opus/VP8 codec frameworks
12. **C API bindings** for cross-language interoperability
13. **State persistence** via GetSavedata/Load

**Target Audience**: Developers building privacy-focused communication applications, researchers working on decentralized protocols, and Tox ecosystem contributors.

---

## Goal-Achievement Summary

| Goal | Status | Evidence |
|------|--------|----------|
| Pure Go (no CGo in core) | ✅ Achieved | Only `capi/toxav_c.go` uses `import "C"` |
| IPv4/IPv6 Support | ✅ Achieved | `transport/network_transport_impl.go:23-123` |
| Tor .onion Transport | ⚠️ Partial | `transport/network_transport_impl.go:159-375` — TCP only via onramp |
| I2P .b32.i2p Transport | ✅ Achieved | `transport/network_transport_impl.go:376-577` — Full SAM integration |
| Nym .nym Transport | ⚠️ Partial | `transport/network_transport_impl.go:610-815` — Dial-only, no Listen |
| Lokinet .loki Transport | ⚠️ Partial | `transport/network_transport_impl.go:845-970` — Dial-only, no Listen |
| Noise IK Pattern | ✅ Achieved | `noise/handshake.go`, `transport/noise_transport.go:82-100` |
| Forward Secrecy (epoch-based) | ✅ Achieved | `async/forward_secrecy.go:42-81`, `async/epoch.go:10-120` |
| Identity Obfuscation | ✅ Achieved | `async/obfs.go:30-233` — HKDF pseudonyms + AES-GCM |
| Message Padding | ✅ Achieved | `async/message_padding.go:17-82` — 256B, 1024B, 4096B buckets |
| DHT K-Bucket Routing | ✅ Achieved | `dht/routing.go:145-210`, `dht/iterative_lookup.go:30-49` |
| Friend Management | ✅ Achieved | `toxcore.go:2169-2308`, `friend/request.go`, `friend/friend.go` |
| Group Chat | ✅ Achieved | `group/chat.go:854-896` — Create, join, message with DHT discovery |
| File Transfers | ✅ Achieved | `file/manager.go`, `file/transfer.go` — Full state machine |
| ToxAV Audio/Video | ✅ Achieved | `toxav.go:332-965`, `av/audio/codec.go`, `av/video/codec.go` |
| C API (toxcore) | ⚠️ Partial | `capi/toxcore_c.go` — Only 7 exports (minimal coverage) |
| C API (toxav) | ✅ Achieved | `capi/toxav_c.go` — 18 exports (comprehensive) |
| State Persistence | ✅ Achieved | `toxcore.go:371-1020` — Full key/friend/metadata serialization |

---

## Findings

### CRITICAL

*No critical findings. All documented core features are functional.*

### HIGH

- [ ] **Incomplete C API for toxcore base functions** — `capi/toxcore_c.go:1-200` — The C API only exports 7 functions (`tox_new`, `tox_kill`, `tox_bootstrap_simple`, `tox_iterate`, `tox_iteration_interval`, `tox_self_get_address_size`, `hex_string_to_bin`). Missing: `tox_friend_add*`, `tox_friend_send_message`, `tox_self_get_address`, `tox_group_*`, `tox_file_*`, and all friend/group/file callbacks. — **Remediation:** Implement remaining c-toxcore compatible exports in `capi/toxcore_c.go` following the pattern established in `capi/toxav_c.go`. Priority exports: `tox_self_get_address`, `tox_friend_add`, `tox_friend_add_norequest`, `tox_friend_send_message`. Validate with `go build -buildmode=c-shared -o libtoxcore.so ./capi/`.

- [ ] **Nym Transport Listen() not implemented** — `transport/network_transport_impl.go:649-660` — Returns `ErrNymNotImplemented`. README claims "Nym .nym" support without clarifying it's outbound-only. — **Remediation:** Update README Multi-Network section to explicitly state: "Nym .nym: outbound Dial only via SOCKS5 proxy. Listen/hosting requires Nym service provider configuration and is not supported via SOCKS5." Validate documentation accuracy with `grep -n "Nym" README.md`.

- [ ] **Lokinet Transport Listen() not implemented** — `transport/network_transport_impl.go:884-895` — Returns error "Lokinet SNApp hosting not supported via SOCKS5". README doesn't clarify this limitation. — **Remediation:** Update README Multi-Network section to explicitly state: "Lokinet .loki: TCP Dial only via SOCKS5 proxy. SNApp hosting requires manual Lokinet configuration." Validate with `grep -n "Lokinet" README.md`.

### MEDIUM

- [ ] **WAL Recover function complexity** — `async/wal.go:55` — Cyclomatic complexity 12, overall complexity 17.6. Highest in codebase. — **Remediation:** Split `Recover()` into smaller helper functions: `readWALEntry()`, `validateChecksum()`, `applyWALEntry()`, `handleCorruptedEntry()`. Target complexity <10. Validate with `go-stats-generator analyze . --skip-tests --format json | jq '.functions[] | select(.name=="Recover")'`.

- [ ] **Oversized toxcore.go file** — `toxcore.go:1-2855` — 2855 lines, 218 functions, maintenance burden score 8.10 (highest). — **Remediation:** Extract callback registration into `toxcore_callbacks.go`, self-management into `toxcore_self.go`, friend operations into `toxcore_friend.go`. Target <500 lines per file. Validate with `wc -l toxcore*.go`.

- [ ] **159 unreferenced functions detected** — Various files — `go-stats-generator` reports 159 potentially dead code functions across the codebase. — **Remediation:** Run `go-stats-generator analyze . --skip-tests --format json | jq '.maintenance.dead_code.unreferenced'` to get full list. Review each for deprecation candidates or missing integration. Remove confirmed dead code.

- [ ] **Magic numbers in codebase** — Various files — 12,506 magic numbers detected across the codebase. — **Remediation:** Define named constants for repeated values in `limits/constants.go` or package-level `const` blocks. Priority: cryptographic sizes (32, 64), protocol limits (1372), timeout values. Validate reduction with `go-stats-generator analyze . --format json | jq '.maintenance.magic_numbers.total'`.

- [ ] **Package name collisions** — `net/` and `testing/` — Package names collide with Go standard library, requiring qualified imports. — **Remediation:** Rename `net/` to `netutil/` or `toxnet/`, rename `testing/` to `testutil/` or `simulation/`. Update all imports. Validate with `go build ./...`.

### LOW

- [ ] **Duplication in example code** — `examples/` various files — 32 clone pairs with 524 duplicated lines (0.7% ratio). Largest clone: 28 lines in `dht/group_storage.go:212`. — **Remediation:** Extract common bootstrap/setup patterns into `examples/common/setup.go`. Validate with `go-stats-generator analyze . --format json | jq '.duplication.duplication_ratio'`.

- [ ] **C API naming conventions** — `capi/toxav_c.go:491-1234` — 55 identifier violations using underscores (e.g., `toxav_get_tox_from_av`). — **Remediation:** No action required — underscore naming is intentional for C API compatibility with c-toxcore. Document as acknowledged exception.

- [ ] **File name stuttering** — `friend/friend.go`, `limits/limits.go` — File names repeat package names unnecessarily. — **Remediation:** Rename to `friend/manager.go` and `limits/constants.go`. Update imports. Validate with `go build ./...`.

- [ ] **Low cohesion files** — 79 files with cohesion <0.5 — Many type definition files (`types.go`) have no internal method relationships. — **Remediation:** Acceptable for type definition files. Review files with both types and functions for potential splitting. No immediate action required.

- [ ] **BUG comments are false positives** — `crypto/logging.go:17,23,115`, `toxav.go:724` — `go-stats-generator` flagged "BUG" as critical annotations, but these are GoDoc comments for functions starting with "log" (matching regex for "BUG"). — **Remediation:** No action required — false positive detection. Consider renaming functions to avoid false matches (e.g., `logDebugMessage` → `debugLog`).

---

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total Lines of Code | 36,401 |
| Total Functions | 917 |
| Total Methods | 2,447 |
| Total Structs | 358 |
| Total Interfaces | 36 |
| Total Packages | 24 |
| Total Files | 204 |
| Average Function Length | 13.3 lines |
| Average Complexity | 3.6 |
| High Complexity Functions (>10) | 1 (`Recover` at 17.6) |
| Documentation Coverage | 92.5% |
| Function Doc Coverage | 98.5% |
| Duplication Ratio | 0.70% |
| Unreferenced Functions | 159 |
| Circular Dependencies | 0 |

### Complexity Hotspots

| Function | Package | Lines | Cyclomatic | Overall |
|----------|---------|-------|------------|---------|
| Recover | async | 55 | 12 | 17.6 |
| doFriendConnections | toxcore | 42 | 10 | 15.0 |
| deliveryLoop | async | 39 | 10 | 15.0 |
| readLoop | transport | 53 | 10 | 14.5 |
| GenerateNodeIDProofWithCancel | dht | 45 | 10 | 14.5 |

### Package Size Distribution

| Package | Functions | Structs | Files |
|---------|-----------|---------|-------|
| transport | 671 | 111 | 37 |
| async | 372 | 48 | 23 |
| dht | 289 | 42 | 14 |
| toxcore | 287 | 30 | 5 |
| av | 209 | 25 | 9 |

---

## Test Results

```
go test -tags nonet -race ./...
```

**Result**: All 53 packages pass with race detection enabled.

```
go vet ./...
```

**Result**: No warnings.

---

## Validation Commands

```bash
# Verify documentation coverage
go-stats-generator analyze . --skip-tests --format json | jq '.documentation.coverage'

# Check complexity hotspots
go-stats-generator analyze . --skip-tests --format json | jq '.complexity.top_10'

# Run tests with race detection
go test -tags nonet -race -coverprofile=coverage.txt -covermode=atomic ./...

# Build C shared library
cd capi && go build -buildmode=c-shared -o libtoxcore.so . && cd ..

# Verify no circular dependencies
go-stats-generator analyze . --skip-tests --format json | jq '.packages.circular_dependencies'
```
