# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-29

## Project Profile
`toxcore-go` is a pure-Go Tox peer-to-peer encrypted messaging implementation for Go applications and C API consumers. The README promises DHT peer discovery, friend management, 1-to-1 and group messaging, file transfer, ToxAV audio/video, asynchronous offline messaging with forward secrecy, multi-network transport, Noise handshakes, NAT traversal, cryptography, C bindings, and Go `net.*` interfaces. Critical paths are transport/proxying, crypto/key handling, DHT/bootstrap, messaging/friends, file transfer, async offline delivery, and ToxAV media processing.

Trust boundaries audited: public Go APIs, C API entry points, network packet parsing, DHT/bootstrap inputs, peer-supplied messages/files/group data, proxy/overlay transports, savedata, and local filesystem persistence.

## Audit Scope
All repository Go packages listed by `go list ./...` were included. `go-stats-generator` processed 238 non-test Go files, 42,529 LOC, 1,166 functions, 2,899 methods, 408 structs, 38 interfaces, and 26 package names. Manual review focused on high-complexity/long functions and risk-pattern hits; package-scoped exploration passes covered core, async/crypto/noise, transport/DHT/toxnet/bootstrap, AV/C API/examples, and utility packages.

Baseline validation:
- `go test -tags nonet -race ./...` passed for all packages.
- `go vet ./...` completed successfully; output only contained dependency download messages.
- GitHub open issues: one open qTox CI/CD integration issue, not a code bug.
- GitHub open PRs: none.
- Code/secret scanning alert APIs were inaccessible to the integration (403), so no alert data was available.

## Coverage Log
| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| github.com/opd-ai/toxcore | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/async | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/av | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/av/audio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/av/rtp | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/av/video | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/bootstrap | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/bootstrap/nodes | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/capi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/cmd/gen-bootstrap-nodes | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/crypto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/dht | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/examples/* | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/factory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/friend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/group | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/limits | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/messaging | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/noise | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/real | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/toxnet | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/transport | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| github.com/opd-ai/toxcore/transport/internal/addressing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary
| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| Pure Go Tox peer-to-peer encrypted messaging core | ⚠️ | F-HIGH-1 affects proxy privacy configuration; no core crypto correctness bug confirmed. |
| DHT peer discovery and bootstrap | ✅ | None confirmed. |
| Friend management and 1-to-1 messaging | ✅ | None confirmed. |
| Group chat | ✅ | None confirmed. |
| File transfers | ✅ | None confirmed. |
| ToxAV audio/video | ⚠️ | F-MED-1 affects exported direct video encoder/processor paths with strided frames. |
| Async offline messaging with forward secrecy | ✅ | None confirmed. |
| Multi-network transport and proxy support | ⚠️ | F-HIGH-1 can send UDP directly after requested SOCKS5 UDP proxy setup fails. |
| Noise handshakes and cryptography | ✅ | None confirmed. |
| C API bindings | ✅ | None confirmed. |
| Go `net.*` interfaces | ✅ | None confirmed. |

## Findings

### CRITICAL
No confirmed CRITICAL findings.

### HIGH
- [ ] F-HIGH-1 — SOCKS5 UDP proxy failure falls back to direct UDP traffic — `transport/proxy.go:84` and `transport/proxy.go:207` — security/resource policy bug — **Code path:** `toxcore.New(options)` with `Options.Proxy.Type=ProxyTypeSOCKS5`, `UDPProxyEnabled=true`, and UDP enabled calls `setupUDPTransport` → `wrapWithProxyIfConfigured` → `transport.NewProxyTransport`; if `NewSOCKS5UDPAssociation` fails, `NewProxyTransport` logs a warning and disables `udpProxyEnabled` instead of returning an error. Later `ProxyTransport.Send` sees no `udpAssociation` and delegates to `t.underlying.Send`, sending packets directly. **Concrete consequence:** an application that requested SOCKS5 UDP proxying can expose the user's real IP address on the primary UDP messaging path when the proxy association is unavailable. **Remediation:** In `transport/proxy.go`, make SOCKS5 UDP association failure fatal when `config.UDPProxyEnabled` is true, and make `ProxyTransport.Send` return an error instead of falling back to direct UDP when proxying was explicitly requested. Validate with `go test -tags nonet -race ./transport/... ./...` plus a test that a failing UDP association prevents direct underlying sends.

### MEDIUM
- [ ] F-MED-1 — Strided or malformed `VideoFrame` planes can panic instead of returning an error — `av/video/processor.go:198` and `av/video/processor.go:224` — nil/boundary/API contract bug — **Code path:** callers using the exported `video.NewRealVP8Encoder(...).Encode(frame)` directly, or `Processor.ProcessOutgoing*` with a `VideoFrame` whose stride exceeds row width but whose backing plane length only satisfies the current width×height check, reach `packPlane`. `packPlane` slices `src[y*stride:y*stride+width]` without validating `stride >= width` and `len(src) >= (height-1)*stride+width`; with a short or inconsistent plane, Go panics. **Concrete consequence:** invalid application-provided video frames can crash the process rather than returning a normal encoding error, violating the exported API's error-return pattern. The ToxAV facade path (`ToxAV.VideoSendFrame` → `av.Call.SendVideoFrame`) uses packed strides and validates minimum plane sizes, so the crash is primarily reachable through the exported `av/video` package APIs. **Remediation:** In `av/video/processor.go`, add stride-aware plane validation before encoding and make `packPlane` return an error instead of panicking; update `RealVP8Encoder.Encode` and processor validation to require `stride >= width` and enough backing data for each plane. Validate with `go test -race ./av/video/...` and table tests for short Y/U/V planes and oversized strides.

### LOW
- [ ] F-LOW-1 — SOCKS5 username/password lengths are not bounded to RFC 1929 one-byte fields — `transport/socks5_udp.go:209` — API/edge-case bug — `performUsernamePasswordAuth` encodes `len(username)` and `len(password)` with `byte(...)` but does not reject lengths above 255. Overlong credentials generate malformed authentication frames and can make UDP proxy setup fail. **Remediation:** In `transport/socks5_udp.go`, reject username or password lengths greater than 255 before building the request. Validate with `go test -race ./transport/...` using overlong credential cases.

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total non-test files processed | 238 |
| Total lines of code | 42,529 |
| Total functions | 1,166 |
| Total methods | 2,899 |
| Functions above cyclomatic complexity 15 | 2 |
| Functions longer than 50 lines | 39 |
| Average function length | 12.8 lines |
| Average cyclomatic complexity | 3.6 |
| Documentation coverage | 93.32% overall |
| Duplication ratio | 0.46% |
| Duplicate clone pairs | 27 |
| Test pass rate | 59/59 packages passed or had no test files under `go test -tags nonet -race ./...` |
| go vet warnings | 0 |

High-risk functions manually inspected included `crypto/keystore.go:389 reencryptWithNewKey`, `toxnet/conn.go:146 waitForDataSignal`, `async/manager.go:895 sendQueuedMessages`, `cmd/gen-bootstrap-nodes/main.go:51 run`, `bootstrap/server.go:startOverlayListener`, and other >50-line functions surfaced by go-stats-generator.

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|----------------|
| `crypto/secure_memory.go:48` panic after `ZeroBytes` | Defensive unreachable path after explicit error checking; not reachable as a normal API failure. |
| `dht/bootstrap.go` constructor panics on nil routing table | Constructor documents a required dependency and panics before background work starts; treated as invalid programmer input, not a runtime bug above LOW. |
| `file/transfer.go:242`/`:251` open/create without immediate local defer | Transfer lifecycle owns `FileHandle`; `Cancel`, completion, and manager replacement paths close it. No confirmed leak on audited paths. |
| `file/manager.go:599` peer-supplied filename traversal | `deserializeFileRequest` strips to `filepath.Base`, and incoming `Transfer.validateAndSanitizePath` rejects path separators before `os.Create`. |
| `cmd/gen-bootstrap-nodes/main.go:103` `exec.Command("gofmt")` | Command is fixed, not user-controlled; no injection path. |
| `async/client.go:1280` recover around channel send | Panic is logged and scoped to avoiding a closed-channel race during response delivery; not silently swallowed. |
| `group/chat.go:184` goroutine callback recover | The code logs recovered panics and isolates user callbacks from group broadcast internals; intentional callback boundary. |
| `toxnet/conn.go:146 waitForDataSignal` complexity | Manual inspection showed lock release/reacquire around `readNotify` is deliberate and avoids missed broadcasts; race tests passed. |
| `crypto/keystore.go:443` ignored `os.Rename` while backing up originals | Earlier `decryptAllFiles` only includes existing files; rollback paths log restoration failures. Risk exists under concurrent external filesystem mutation but no confirmed project call path mutates those files concurrently. |
| `av/video/processor.go:954` inter-frame decoder returns cached key frame | README explicitly documents keyframe-oriented decode behavior and the stale-frame tradeoff. |

## Remaining Scope (if session ended before completion)
| Package | Status | Notes |
|---------|--------|-------|
| All packages | Completed | No remaining unaudited package scope recorded. |
