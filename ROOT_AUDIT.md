# Toxcore Package Audit Tracking

This file tracks the audit status of all packages in the toxcore repository.

## Audit Status

### Root Package
- [x] `AUDIT.md` — Needs Work — 9 issues (4 high, 3 med, 2 low)

### Core Packages
- [x] `async/AUDIT.md` — Previously audited
- [x] `av/AUDIT.md` — Previously audited
- [x] `av/audio/AUDIT.md` — Previously audited
- [x] `av/rtp/AUDIT.md` — Previously audited
- [x] `av/video/AUDIT.md` — Previously audited
- [x] `capi/AUDIT.md` — Previously audited
- [x] `crypto/AUDIT.md` — Previously audited
- [x] `dht/AUDIT.md` — Previously audited
- [x] `file/AUDIT.md` — Previously audited
- [x] `friend/AUDIT.md` — Previously audited
- [x] `group/AUDIT.md` — Previously audited
- [x] `interfaces/AUDIT.md` — Previously audited
- [x] `limits/AUDIT.md` — Previously audited
- [x] `messaging/AUDIT.md` — Previously audited
- [x] `net/AUDIT.md` — Previously audited
- [x] `noise/AUDIT.md` — Previously audited
- [x] `real/AUDIT.md` — Previously audited
- [x] `transport/AUDIT.md` — Previously audited

### Supporting Packages
- [x] `factory/AUDIT.md` — Previously audited
- [x] `testing/AUDIT.md` — Previously audited
- [x] `testnet/cmd/AUDIT.md` — Previously audited
- [x] `testnet/internal/AUDIT.md` — Previously audited

### Example Packages
- [x] `examples/noise_demo/AUDIT.md` — Needs Work — 7 issues (2 high, 3 med, 2 low)
- [x] `examples/async_demo/AUDIT.md` — Needs Work — 12 issues (4 high, 5 med, 3 low)
- [x] `examples/toxav_integration/AUDIT.md` — Needs Work — 15 issues (11 high, 3 med, 1 low)
- [x] `net/example/AUDIT.md` — Needs Work — 7 issues (2 high, 3 med, 2 low)

## Summary Statistics
- Total packages audited: 27
- Packages needing work: 5 (root, examples/noise_demo, examples/async_demo, examples/toxav_integration, net/example)
- Total critical issues: 23 high-priority issues (4 in root, 2 in noise_demo, 4 in async_demo, 11 in toxav_integration, 2 in net/example)

## Key Issues to Address
1. Non-deterministic time usage in root package (4 high-priority instances), async_demo (4 instances), and toxav_integration (8 instances)
2. Concrete network type assertions in root package, async_demo, and net/example (violates interface guidelines)
3. Test coverage below 65% target in root package; 0% in async_demo, toxav_integration, and net/example
4. Standard library logging instead of structured logging in net/example (9 instances) and toxav_integration (5 instances)
5. Swallowed errors in async_demo example (9 instances without proper handling)

## Audit Guidelines
See individual package AUDIT.md files for detailed findings following these categories:
- Stub/incomplete code
- ECS compliance (if applicable)
- Deterministic procgen (randomness patterns)
- Network interfaces (interface vs concrete types)
- Error handling (no swallowed errors)
- Test coverage (≥65% target)
- Documentation coverage
- Integration points
