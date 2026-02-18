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
- [x] `capi/AUDIT_FRESH.md` — Needs Work — 10 issues (3 high, 4 med, 3 low)
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
- [x] `net/AUDIT_FRESH.md` — Needs Work — 16 issues (6 high, 5 med, 5 low)
- [x] `noise/AUDIT.md` — Previously audited
- [x] `noise/AUDIT_FRESH.md` — Needs Work — 7 issues (2 high, 2 med, 3 low)
- [x] `real/AUDIT.md` — Previously audited
- [x] `transport/AUDIT.md` — Previously audited

### Supporting Packages
- [x] `factory/AUDIT.md` — Previously audited
- [x] `factory/AUDIT_FRESH.md` — Complete — 0 issues (0 high, 0 med, 0 low)
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
- Total packages audited: 39 (34 previous + 5 fresh re-audits: noise, crypto, factory, capi, net)
- Packages needing work: 15 (root, noise, capi, net, examples/noise_demo, examples/async_demo, examples/async_obfuscation_demo, examples/toxav_integration, examples/file_transfer_demo, examples/audio_effects_demo, examples/multi_transport_demo, examples/privacy_networks, examples/toxav_video_call, net/example, net/examples/packet)
- Packages complete: 2 (crypto [FRESH AUDIT], factory [FRESH AUDIT])
- Total critical issues: 54 high-priority issues (3 in root [1 fixed], 2 in noise [FRESH AUDIT], 3 in capi [FRESH AUDIT], 6 in net [FRESH AUDIT], 0 in crypto [FRESH AUDIT], 0 in factory [FRESH AUDIT], 2 in noise_demo, 4 in async_demo, 4 in async_obfuscation_demo, 11 in toxav_integration, 2 in file_transfer_demo, 3 in audio_effects_demo, 3 in multi_transport_demo, 2 in privacy_networks, 5 in toxav_video_call, 2 in net/example, 2 in net/examples/packet)

## Key Issues to Address
1. **CRITICAL BUG in noise package**: `IKHandshake.GetLocalStaticKey()` returns ephemeral instead of static key, breaking peer identity verification (noise/handshake.go:246)
2. **CRITICAL BUG in capi package**: Callback functions use placeholder implementations that don't bridge to C, breaking C interoperability (capi/toxav_c.go:527-640); go vet reports unsafe.Pointer misuse (capi/toxav_c.go:268)
3. **CRITICAL BUG in net package**: Timeout mechanism broken - TestDialTimeout fails consistently, taking 5 seconds instead of 10-200ms; DialTimeout function ignores provided timeout (conn_test.go:33-43, dial.go:83-100)
4. **CRITICAL BUG in net package**: ToxConn.setupCallbacks overwrites global Tox callbacks causing severe message collision when multiple connections exist (conn.go:82-107)
5. ~~Non-deterministic time usage in root package (3 high-priority instances remaining)~~ (**FIXED**: Added injectable TimeProvider), noise package (1 instance), net package (6 instances), async_demo (4 instances), toxav_integration (8 instances), toxav_video_call (5 instances), multi_transport_demo (1 instance), and net/examples/packet (1 instance)
6. ~~Concrete network type assertions in root package~~ (**FIXED**), async_demo, file_transfer_demo, and net/example (violates interface guidelines)
7. Test coverage below 65% target in root package (64.3%), capi package (57.2%), and net package (43.5%); 0% in async_demo, async_obfuscation_demo, toxav_integration, file_transfer_demo, audio_effects_demo, multi_transport_demo, privacy_networks, toxav_video_call, net/example, and net/examples/packet
8. Standard library logging instead of structured logging in net/example (9 instances), toxav_integration (5 instances), file_transfer_demo (32 instances), audio_effects_demo (16 instances), toxav_video_call (31 instances), async_obfuscation_demo (4 instances), multi_transport_demo (4 instances), privacy_networks (34 instances), and net/examples/packet (5 instances)
9. Swallowed errors in async_demo example (9 instances), async_obfuscation_demo (4 instances of transport errors), multi_transport_demo (2 instances of Write() errors), and capi package (toxcore.New error not logged in tox_new)
10. Stub implementations blocking real usage: net.PacketListen creates invalid ToxAddr with nil toxID (dial.go:189-190); net.ToxPacketConn.WriteTo bypasses Tox encryption (packet_conn.go:264-266)

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
