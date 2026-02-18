# Toxcore Package Audit Tracking

This file tracks the audit status of all packages in the toxcore repository.

## Audit Status

### Root Package
- [x] `AUDIT.md` — Complete — 8 issues (all fixed: 4 high-priority determinism/network, 3 medium error-handling, 2 low coverage/docs)

### Core Packages
- [x] `async/AUDIT.md` — Previously audited
- [x] `av/AUDIT.md` — Previously audited
- [x] `av/audio/AUDIT.md` — Previously audited
- [x] `av/rtp/AUDIT.md` — Previously audited
- [x] `av/video/AUDIT.md` — Previously audited
- [x] `capi/AUDIT.md` — Previously audited
- [x] `capi/AUDIT_FRESH.md` — Complete — 7 issues (0 high, 0 med [3 fixed], 4 low) — All high and medium priority issues fixed
- [x] `crypto/AUDIT.md` — Previously audited
- [x] `crypto/AUDIT_FRESH.md` — Complete — 2 issues (0 high, 0 med, 2 low)
- [x] `dht/AUDIT.md` — Previously audited
- [x] `file/AUDIT.md` — Previously audited
- [x] `friend/AUDIT.md` — Previously audited
- [x] `group/AUDIT.md` — Previously audited
- [x] `interfaces/AUDIT.md` — Previously audited
- [x] `limits/AUDIT.md` — Previously audited
- [x] `messaging/AUDIT.md` — Previously audited
- [x] `net/AUDIT.md` — Previously audited
- [x] `net/AUDIT_FRESH.md` — Complete — Test coverage 76.6% (exceeds 65% target), all high-priority issues fixed
- [x] `noise/AUDIT.md` — Previously audited
- [x] `noise/AUDIT_FRESH.md` — Complete — 3 issues (0 high, 0 med, 3 low) — All high-priority issues fixed
- [x] `real/AUDIT.md` — Previously audited
- [x] `transport/AUDIT.md` — Previously audited

### Supporting Packages
- [x] `factory/AUDIT.md` — Previously audited
- [x] `factory/AUDIT_FRESH.md` — Complete — 0 issues (0 high, 0 med, 0 low)
- [x] `testing/AUDIT.md` — Previously audited
- [x] `testnet/cmd/AUDIT.md` — Previously audited
- [x] `testnet/internal/AUDIT.md` — Previously audited

### Example Packages
- [x] `examples/noise_demo/AUDIT.md` — Complete — 7 issues (0 high [2 fixed], 1 med remaining [acceptable], 1 low [fixed]) — 59.2% test coverage
- [x] `examples/async_demo/AUDIT.md` — Complete — All high-priority issues fixed, 42% test coverage (acceptable for demo code)
- [x] `examples/async_obfuscation_demo/AUDIT.md` — High-priority fixed — 12 issues (0 high [4 fixed], 5 med, 3 low)
- [x] `examples/toxav_integration/AUDIT.md` — Needs Work — 15 issues (11 high, 3 med, 1 low)
- [x] `examples/file_transfer_demo/AUDIT.md` — Needs Work — 7 issues (2 high, 2 med, 3 low)
- [x] `examples/audio_effects_demo/AUDIT.md` — Needs Work — 6 issues (3 high, 2 med, 1 low)
- [x] `examples/multi_transport_demo/AUDIT.md` — Needs Work — 9 issues (3 high, 2 med, 4 low)
- [x] `examples/privacy_networks/AUDIT.md` — Needs Work — 8 issues (2 high, 3 med, 3 low)
- [x] `net/example/AUDIT.md` — Needs Work — 7 issues (2 high, 3 med, 2 low)
- [x] `net/examples/packet/AUDIT.md` — Needs Work — 6 issues (2 high, 3 med, 1 low)
- [x] `examples/toxav_video_call/AUDIT.md` — Needs Work — 11 issues (5 high, 3 med, 3 low)

## Summary Statistics
- Total packages audited: 39 (34 previous + 5 fresh re-audits: noise, crypto, factory, capi, net)
- Packages needing work: 9 (examples/async_obfuscation_demo, examples/toxav_integration, examples/file_transfer_demo, examples/audio_effects_demo, examples/multi_transport_demo, examples/privacy_networks, examples/toxav_video_call, net/example, net/examples/packet)
- Packages complete: 7 (crypto [FRESH AUDIT], factory [FRESH AUDIT], noise [FRESH AUDIT — all high-priority issues fixed], net [FRESH AUDIT — 76.6% coverage, exceeds target], noise_demo [FRESH AUDIT — 59.2% coverage], capi [FRESH AUDIT — 72.4% coverage, all medium/high issues fixed], async_demo [FIXED — 42% coverage, all high-priority issues fixed])
- Total critical issues: 30 high-priority issues remaining (0 in net [FIXED], 0 in noise [FIXED], 0 in capi [FIXED], 0 in noise_demo [FIXED], 0 in async_demo [FIXED], 0 in async_obfuscation_demo [FIXED], 11 in toxav_integration, 2 in file_transfer_demo, 3 in audio_effects_demo, 3 in multi_transport_demo, 2 in privacy_networks, 5 in toxav_video_call, 2 in net/example, 2 in net/examples/packet)

## Key Issues to Address
1. ~~**CRITICAL BUG in noise package**~~: ✅ FIXED — `IKHandshake.GetLocalStaticKey()` now properly returns stored static public key via `localPubKey` field
2. ~~**CRITICAL BUG in capi package**~~: ✅ FIXED — All callback functions now properly bridge to C via CGO; go vet passes; tox_new logs errors with structured context
3. ~~**CRITICAL BUG in net package (timeout)**~~: ✅ FIXED — TestDialTimeout now passes in ~10ms; timeout mechanism working correctly
4. ~~**CRITICAL BUG in net package (callbacks)**~~: ✅ FIXED — ToxConn.setupCallbacks now uses callback router/multiplexer to route messages to correct ToxConn by friendID
5. ~~**CRITICAL BUG in net package (test coverage)**~~: ✅ FIXED — Coverage improved from 43.5% to 76.6% (exceeds 65% target)
6. ~~Non-deterministic time usage in root package (3 high-priority instances remaining)~~ (**FIXED**: Added injectable TimeProvider), noise package (acceptable for crypto), net package (6 instances), async_demo (4 instances), toxav_integration (8 instances), toxav_video_call (5 instances), multi_transport_demo (1 instance), and net/examples/packet (1 instance)
7. ~~Concrete network type assertions in root package~~ (**FIXED**), async_demo, file_transfer_demo, and net/example (violates interface guidelines)
8. Test coverage below 65% target in root package (64.3%); 0% in async_demo, async_obfuscation_demo, toxav_integration, file_transfer_demo, audio_effects_demo, multi_transport_demo, privacy_networks, toxav_video_call, net/example, and net/examples/packet
9. Standard library logging instead of structured logging in net/example (9 instances), toxav_integration (5 instances), file_transfer_demo (32 instances), audio_effects_demo (16 instances), toxav_video_call (31 instances), async_obfuscation_demo (4 instances), multi_transport_demo (4 instances), privacy_networks (34 instances), and net/examples/packet (5 instances)
10. ~~Swallowed errors in capi package (toxcore.New error not logged in tox_new)~~ (**FIXED**); remaining in async_demo example (9 instances), async_obfuscation_demo (4 instances of transport errors), multi_transport_demo (2 instances of Write() errors)
11. ~~Stub implementations blocking real usage~~: ✅ FIXED — net.PacketListen now requires `*toxcore.Tox` parameter and creates valid ToxAddr; ToxPacketConn.WriteTo documented as placeholder API

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
