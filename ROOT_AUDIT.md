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
- [x] `noise/AUDIT_FRESH.md` — Needs Work — 7 issues (2 high, 2 med, 3 low)
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
- [x] `examples/async_obfuscation_demo/AUDIT.md` — Needs Work — 12 issues (4 high, 5 med, 3 low)
- [x] `examples/toxav_integration/AUDIT.md` — Needs Work — 15 issues (11 high, 3 med, 1 low)
- [x] `examples/file_transfer_demo/AUDIT.md` — Needs Work — 7 issues (2 high, 2 med, 3 low)
- [x] `examples/audio_effects_demo/AUDIT.md` — Needs Work — 6 issues (3 high, 2 med, 1 low)
- [x] `examples/multi_transport_demo/AUDIT.md` — Needs Work — 9 issues (3 high, 2 med, 4 low)
- [x] `examples/privacy_networks/AUDIT.md` — Needs Work — 8 issues (2 high, 3 med, 3 low)
- [x] `net/example/AUDIT.md` — Needs Work — 7 issues (2 high, 3 med, 2 low)
- [x] `net/examples/packet/AUDIT.md` — Needs Work — 6 issues (2 high, 3 med, 1 low)
- [x] `examples/toxav_video_call/AUDIT.md` — Needs Work — 11 issues (5 high, 3 med, 3 low)

## Summary Statistics
- Total packages audited: 35 (34 previous + 1 fresh re-audit)
- Packages needing work: 13 (root, noise, examples/noise_demo, examples/async_demo, examples/async_obfuscation_demo, examples/toxav_integration, examples/file_transfer_demo, examples/audio_effects_demo, examples/multi_transport_demo, examples/privacy_networks, examples/toxav_video_call, net/example, net/examples/packet)
- Total critical issues: 46 high-priority issues (4 in root, 2 in noise [FRESH AUDIT], 2 in noise_demo, 4 in async_demo, 4 in async_obfuscation_demo, 11 in toxav_integration, 2 in file_transfer_demo, 3 in audio_effects_demo, 3 in multi_transport_demo, 2 in privacy_networks, 5 in toxav_video_call, 2 in net/example, 2 in net/examples/packet)

## Key Issues to Address
1. **CRITICAL BUG in noise package**: `IKHandshake.GetLocalStaticKey()` returns ephemeral instead of static key, breaking peer identity verification (noise/handshake.go:246)
2. Non-deterministic time usage in root package (4 high-priority instances), noise package (1 instance), async_demo (4 instances), toxav_integration (8 instances), toxav_video_call (5 instances), multi_transport_demo (1 instance), and net/examples/packet (1 instance)
3. Concrete network type assertions in root package, async_demo, file_transfer_demo, and net/example (violates interface guidelines)
4. Test coverage below 65% target in root package; 0% in async_demo, async_obfuscation_demo, toxav_integration, file_transfer_demo, audio_effects_demo, multi_transport_demo, privacy_networks, toxav_video_call, net/example, and net/examples/packet
5. Standard library logging instead of structured logging in net/example (9 instances), toxav_integration (5 instances), file_transfer_demo (32 instances), audio_effects_demo (16 instances), toxav_video_call (31 instances), async_obfuscation_demo (4 instances), multi_transport_demo (4 instances), privacy_networks (34 instances), and net/examples/packet (5 instances)
6. Swallowed errors in async_demo example (9 instances), async_obfuscation_demo (4 instances of transport errors), and multi_transport_demo (2 instances of Write() errors)

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
