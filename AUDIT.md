# MAINTAINABILITY AUDIT — 2026-04-23

## Project Maintenance Context
- **Team size / ownership**: Small maintainer-led project with heavy bot-assisted delivery (last 12 months commit authors: `user` 930, `copilot-swe-agent[bot]` 238, `opdai` 126, `Copilot` 75, `GitHub Copilot` 57).
- **Maturity stage**: Active development (1,426 commits in last 12 months; 162 commits in last 30 days from GitHub commit feed).
- **Contributor diversity / bus factor**: Human contributor concentration is high (dominant `user` author plus limited additional humans), lowering bus factor despite high bot throughput.
- **Change concentration (6-month churn)**: Highest Go-code churn is in `transport` (230), `async` (207), `av` (199), `dht` (145), `group` (95); top files include `toxcore.go` (78), `group/chat.go` (41), `av/manager.go` (25), `async/manager.go` (25), `async/client.go` (22).
- **Quality infrastructure**: CI enforces `gofmt`, `go vet`, `staticcheck`, `govulncheck`, race tests and coverage (`.github/workflows/toxcore.yml`). `staticcheck.conf` disables several checks including `U1000`.
- **Online research (brief)**:
  - Open issue volume is low (1 open issue; not maintainability-related).
  - Recent PR stream is maintainability/security-audit heavy and concentrated in core subsystems (`transport`, `async`, `av`, root APIs).
  - Contribution guidance is explicit in `README.md` and CI requirements, but automation dominates contribution traffic.

## Maintainability Scorecard
| Category | Status | Key Metric |
|----------|--------|------------|
| Complexity | ⚠️ | 0 functions with cyclomatic >15; 29 functions >50 lines; 54 non-test files >500 LOC |
| Duplication | ✅ | 31 clone pairs, 480 duplicated lines, 0.56% ratio |
| Coupling | ⚠️ | 0 circular deps; max fan-in 19 (`transport`/`toxcore`), max fan-out 12 (`toxcore`) |
| Documentation | ⚠️ | 93.2% overall GoDoc coverage; 2 exported methods undocumented |
| Code Freshness | ✅ | Go 1.25.0 (toolchain 1.25.8), 0 `ioutil` usages, 0 production TODO/FIXME |
| Testability | ⚠️ | `transport` high churn + high coupling + flaky test (`transport/worker_pool_test.go:282`) |

## Complexity Hotspots
| Function | File:Line | Complexity | Lines | Params | Risk |
|----------|-----------|-----------|-------|--------|------|
| run | cmd/gen-bootstrap-nodes/main.go:51 | 15.3 overall (cyclomatic 11) | 49 | 0 | MEDIUM |
| validatePlaneParams | av/video/encoder_cgo.go:212 | 14.8 overall (cyclomatic 11) | 38 | 4 | MEDIUM |
| SendAsyncMessage | async/client.go:279 | 12.7 overall (cyclomatic 9) | 70 | 3 | HIGH |
| tryDecryptWithSender | async/client.go:1300 | 12.7 overall (cyclomatic 9) | 53 | 2 | HIGH |
| decryptRetrievedMessages | async/client.go:588 | 10.6 overall (cyclomatic 7) | 57 | 1 | MEDIUM |
| restoreFriendsList | toxcore_lifecycle.go:458 | 9.8 overall, nesting depth 4 | 30 | 0 | MEDIUM |
| toggleCallState | av/manager.go:1257 | 5.7 overall | 23 | 8 | MEDIUM |
| createToxInstance | toxcore.go:580 | 1.3 overall | 28 | 10 | MEDIUM |

## Findings
### CRITICAL
- [x] No CRITICAL maintainability debt met the provided threshold set (no circular dependencies; no cyclomatic >30 hotspots).

### HIGH
- [x] **Hot-path async complexity in high-churn file** — `async/client.go:279`, `:588`, `:1300` — metrics: overall complexity 12.7/10.6/12.7 with lengths 70/57/53 lines; file churn 22 (6 months) — impact: this code is frequently changed and currently dense, increasing regression probability in offline-message flows — **Remediation:** split `SendAsyncMessage` into validation, encryption, routing, and persistence stages; split decrypt pipeline into deterministic subroutines with focused tests; keep behavior identical and validate with `go test -tags nonet -race ./async ./messaging` and `go vet ./...`.
- [x] **Oversized high-churn architecture center** — `toxcore.go:1` (1,524 LOC, churn 78), `group/chat.go:1` (2,032 LOC, churn 41), `av/manager.go:1` (1,891 LOC, churn 25) — impact: frequent edits in very large files raise cognitive load, review fatigue, and merge risk — **Remediation:** incrementally extract cohesive slices into focused files (API composition, lifecycle, callbacks, transport wiring) in behavior-preserving PRs; validate each slice with package tests and `go-stats-generator analyze . --skip-tests`.

### MEDIUM
- [ ] **Coupling concentration in `transport` package** — `transport/network_transport.go:1` (package anchor) — metrics: 733 functions / 41 files from go-stats, fan-in 19, and directory churn 230 (6 months) — impact: shotgun-surgery risk when transport abstractions change — **Remediation:** carve transport into narrower internal subpackages by responsibility (addressing/resolution, relay, NAT traversal, protocol framing) behind stable interfaces; verify with `go list ./...` (no cycles), `go vet ./...`, targeted `go test -tags nonet -race ./transport/...`.
- [ ] **Repeat clone blocks in production paths** — `bootstrap/server.go:377-391` vs `:416-430` (15-line clone), `capi/toxcore_c.go:944-958` vs `:965-978` (15-line clone), `noise/handshake.go:260-272` vs `noise/psk_resumption.go:519-532` (13-line clone) — metrics: repository duplication 31 clone pairs / 0.56% — impact: duplicated logic raises drift risk during protocol or API updates — **Remediation:** extract shared helpers per domain (bootstrap response handling, C-API marshalling helpers, handshake record construction) while preserving call contracts; validate with `go test -tags nonet -race ./bootstrap ./capi ./noise`.
- [ ] **Static dead-code guardrail disabled** — `.github/workflows/toxcore.yml:50` enforces staticcheck, but `staticcheck.conf:13` disables `U1000` — impact: dormant code can accumulate unnoticed and increase maintenance burden — **Remediation:** re-enable `U1000` incrementally via package-by-package allowlist cleanup, starting with high-churn packages; verify with `staticcheck ./...` and staged suppression removal.
- [ ] **Change-safety instability in high-churn package tests** — `transport/worker_pool_test.go:282` failed in this run (`Expected at least 50 processed, got 49`) during `go test -tags nonet -race ./...` — impact: flaky tests reduce confidence in maintainability refactors and inflate CI noise — **Remediation:** make assertion deterministic (controlled synchronization/barriers rather than timing-sensitive threshold); validate via `go test -tags nonet -race -count=20 ./transport -run TestWorkerPoolStats`.

### LOW
- [ ] **Minor exported API doc gap** — `async/storage_discovery.go:50-51` (`Network`, `String`) — metrics: doc coverage 93.2% overall; exactly 2 exported methods undocumented by go-stats — impact: low but avoidable onboarding friction — **Remediation:** add concise GoDoc comments for both methods; verify with `go-stats-generator analyze . --skip-tests --sections documentation`.
- [ ] **Formatting drift presently in repository** — `transport/noise_transport.go`, `dht/dht_fuzz_test.go`, `async/client.go` reported by pre-audit `gofmt -l` — impact: unnecessary review noise and avoidable CI failures — **Remediation:** enforce pre-commit formatting hooks or CI autofix workflow; validate with `gofmt -l $(find . -name '*.go' | grep -v vendor)`.

## Technical Debt Inventory
| Debt Item | Category | Effort | Impact | Priority |
|-----------|----------|--------|--------|----------|
| Decompose async send/decrypt hotspots (`async/client.go`) | complexity | M | Reduces regression risk in frequently changed secure messaging path | P1 |
| Split large, high-churn root files (`toxcore.go`, `group/chat.go`, `av/manager.go`) | coupling/cohesion | L | Improves reviewability and parallel development | P1 |
| Reduce `transport` package coupling concentration | coupling | L | Lowers shotgun-surgery risk across networking features | P1 |
| Remove production clone blocks (bootstrap/capi/noise) | duplication | M | Prevents divergence and duplicated bug fixes | P2 |
| Stabilize flaky `TestWorkerPoolStats` | change-safety | S | Increases CI trust for refactor work | P2 |
| Re-enable `staticcheck` dead-code detection (`U1000`) | freshness/process | M | Prevents silent code rot accumulation | P2 |
| Fill exported method doc gaps in async discovery | docs | S | Improves API clarity with low effort | P3 |

## False Positives Considered and Rejected
| Candidate Finding | Reason Rejected |
|-------------------|----------------|
| `cmd/gen-bootstrap-nodes/main.go:51` complexity 15.3 | Slightly above medium threshold but in isolated tooling path, moderate size (49 LOC), and not a core runtime hotspot. |
| Switch blocks >10 cases in `examples/toxav_integration/main.go:521,553` | Both are example/demo code, not core library paths; refactoring priority is low for maintainability of production packages. |
| Widespread concrete `net.*Addr` usage in low-level transport files | Much of this appears intentional at system-boundary code (socket/NAT/address translation) where concrete stdlib address structs are required; not all occurrences are actionable debt. |
