# Technical Debt Gaps — 2026-04-23

## Gap 1: Async client hotspot decomposition is lagging behind churn
- **Current State**: `async/client.go` is heavily modified (churn 22 in 6 months) and contains long, high-complexity functions (`SendAsyncMessage` at `async/client.go:279` = 70 LOC / overall 12.7; `tryDecryptWithSender` at `:1300` = 53 LOC / overall 12.7; `decryptRetrievedMessages` at `:588` = 57 LOC / overall 10.6).
- **Impact**: High regression probability when changing offline-message and decryption behavior; slower review and harder fault isolation.
- **Remediation**: Extract behavior-preserving stages (input validation, cryptographic envelope creation, storage-node routing, post-send bookkeeping, decrypt strategy selection) and lock each stage with focused tests.
- **Effort**: medium (~10-16 hours)
- **Dependencies**: Stabilize `transport` test flakiness first to keep CI trustworthy for refactor PRs.
- **Quick Win**: no

## Gap 2: Large-file concentration in high-churn core entrypoints
- **Current State**: `toxcore.go` (1,524 LOC, churn 78), `group/chat.go` (2,032 LOC, churn 41), `av/manager.go` (1,891 LOC, churn 25) exceed maintainability-friendly file boundaries.
- **Impact**: Elevated cognitive load, broader merge conflicts, and difficult scoped ownership.
- **Remediation**: Partition by responsibility into smaller files/modules (construction/configuration, lifecycle, callback dispatch, transport integration, per-feature managers) with no behavior changes.
- **Effort**: large (~24-40 hours)
- **Dependencies**: Establish slice boundaries and package contracts first.
- **Quick Win**: no

## Gap 3: Transport package coupling and fan-in are too concentrated
- **Current State**: `transport` has 733 functions across 41 files (go-stats), fan-in 19 (highest), and highest directory churn (230 in 6 months).
- **Impact**: Shotgun surgery risk; small protocol or addressing changes ripple through many files and tests.
- **Remediation**: Introduce internal subpackage boundaries and strict interface seams for NAT, relay, address resolution, and handshake concerns while preserving import acyclicity.
- **Effort**: large (~20-32 hours)
- **Dependencies**: Inventory public API contracts consumed by `toxcore`, `async`, and `dht` first.
- **Quick Win**: no

## Gap 4: Production duplicate blocks still require synchronized edits
- **Current State**: Multiple non-test clone pairs remain: `bootstrap/server.go:377-391` & `:416-430` (15 lines), `capi/toxcore_c.go:944-958` & `:965-978` (15 lines), `noise/handshake.go:260-272` & `noise/psk_resumption.go:519-532` (13 lines). Global duplication ratio is low (0.56%) but these are in active code.
- **Impact**: Fixes and behavior changes can drift between copies.
- **Remediation**: Replace repeated blocks with shared helpers local to each domain and verify output equivalence with existing package tests.
- **Effort**: medium (~8-14 hours)
- **Dependencies**: None.
- **Quick Win**: yes

## Gap 5: Static dead-code detection is intentionally disabled
- **Current State**: `staticcheck.conf:13` disables `U1000` despite CI running staticcheck.
- **Impact**: Unused declarations can accumulate undetected, increasing long-term comprehension overhead.
- **Remediation**: Re-enable `U1000` incrementally (start with `transport`, `async`, `av`), fixing/removing dead declarations in small PR batches.
- **Effort**: medium (~6-12 hours)
- **Dependencies**: Agreement on acceptable generated/compatibility exceptions.
- **Quick Win**: yes

## Gap 6: Change-safety friction from flaky test in hot package
- **Current State**: `transport/worker_pool_test.go:282` intermittently failed under race run (`Expected at least 50 processed, got 49`) during baseline validation.
- **Impact**: Refactor confidence drops; CI signal quality degrades in the highest-churn package.
- **Remediation**: Replace timing-sensitive threshold assertion with deterministic synchronization-based success criteria and run repeated stress (`-count=20`).
- **Effort**: small (~2-4 hours)
- **Dependencies**: None.
- **Quick Win**: yes

## Gap 7: Small but avoidable public API documentation misses
- **Current State**: `async/storage_discovery.go:50-51` contains undocumented exported methods (`Network`, `String`) while overall docs are high (93.2%).
- **Impact**: Minor onboarding/API discoverability friction.
- **Remediation**: Add method-level GoDoc comments and keep documentation checks in routine maintenance.
- **Effort**: small (~0.5-1 hour)
- **Dependencies**: None.
- **Quick Win**: yes

