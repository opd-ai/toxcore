# Audit: github.com/opd-ai/toxcore/net
**Date**: 2026-02-19
**Status**: Needs Work

## Summary
The `net` package provides Go standard library networking interfaces (net.Conn, net.Listener, net.PacketConn, net.Addr) for Tox protocol integration. Overall architecture is solid with proper concurrency patterns. Packet encryption is now implemented with optional NaCl box encryption support. Test coverage at 77.4% exceeds target. Several edge cases need attention.

## Issues Found
- [x] high security — Packet encryption not implemented - WriteTo bypasses Tox protocol encryption (`packet_conn.go:260`, `packet_conn.go:285`) — **Fixed: Added optional NaCl box encryption via EnableEncryption()/AddPeerKey() with proper nonce handling and address normalization**
- [x] med concurrency — Timer leak in setupReadTimeout - timer.C returned but never stopped when not used (`conn.go:114`) — **Fixed: Returns cleanup function to stop timer**
- [x] med concurrency — Timer leak in setupConnectionTimeout - timer.Stop comment inaccurate, no cleanup guaranteed (`conn.go:310`) — **Fixed: Returns cleanup function to stop timer**
- [x] med error-handling — writeChunkedData returns partial write on error but doesn't signal partial success properly (`conn.go:259`) — **Fixed: Returns ErrPartialWrite wrapped with underlying error**
- [ ] low documentation — PacketListen godoc mentions returning net.Listener instead of actual signature behavior (`dial.go:250`)
- [ ] low api-design — ListenAddr ignores addr parameter, confusing API contract (`dial.go:190`)
- [ ] low concurrency — Race condition in waitForConnection - reads connected without lock after RLock released (`conn.go:215-216`)
- [ ] low error-handling — processIncomingPacket boolean return semantics inverted (returns true to stop vs continue) (`packet_conn.go:106`)

## Test Coverage
77.4% (target: 65%)

## Dependencies
**External**:
- github.com/opd-ai/toxcore - Core Tox protocol implementation
- github.com/sirupsen/logrus - Structured logging

**Standard Library**:
- net - Standard networking interfaces
- context - Cancellation and timeout management
- sync - Concurrency primitives (Mutex, RWMutex, Cond, WaitGroup)
- time - Deadline and timeout handling
- bytes - Buffer management for stream reassembly
- encoding/hex - Tox ID parsing

**Integration Points**: Heavy integration with toxcore package for callbacks (OnFriendMessage, OnFriendStatus, OnFriendRequest), friend management (AddFriend, GetFriends), and message sending (FriendSendMessage).

## Recommendations
1. ~~Implement Tox packet encryption in ToxPacketConn.WriteTo() to ensure protocol compliance and security~~ **DONE**
2. ~~Fix timer leaks by ensuring all timers are stopped in setupReadTimeout and setupConnectionTimeout paths~~ **DONE**
3. ~~Add explicit partial write error wrapping in writeChunkedData to distinguish success/failure states~~ **DONE**
4. Review and fix race condition in conn.go:215-216 by holding lock during connected read
5. Standardize boolean return semantics for clarity (consider using named returns or error returns)
