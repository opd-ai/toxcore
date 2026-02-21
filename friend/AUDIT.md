# Audit: github.com/opd-ai/toxcore/friend
**Date**: 2026-02-20
**Status**: Complete

## Summary
The friend package implements friend relationship and friend request management for the Tox protocol with cryptographic security. The package is well-structured with 93.1% test coverage, proper error handling, and comprehensive documentation. No critical security issues or incomplete implementations found.

## Issues Found
- [ ] low error-handling — Test code swallows errors from SetName/SetStatusMessage; should use t.Fatal() or explicit checks (`friend_test.go:291-292, 321-322, 367, 530-531`)
- [x] med concurrency — FriendInfo lacks thread-safety documentation and protection; caller must synchronize access per doc.go:88-89 but no enforcement mechanism (`friend.go:52-61`) — **RESOLVED**: Added sync.RWMutex to FriendInfo struct with proper locking in all getter/setter methods
- [x] low concurrency — RequestManager.AddRequest potential deadlock if handler calls back into manager; mutex unlocked before handler but lock reacquired after (`request.go:272-275`) — **RESOLVED**: Refactored to properly release lock before handler callback, preventing deadlock when handler calls back into RequestManager methods
- [x] med api-design — Request.Encrypt requires KeyPair but SenderPublicKey field is never populated during NewRequest (`request.go:70-123, 126-158`) — **RESOLVED**: NewRequest now derives SenderPublicKey from senderSecretKey using crypto.FromSecretKey()
- [ ] low documentation — LastSeenDuration method exists but GetLastSeen() referenced in doc.go:28 doesn't exist (`doc.go:28`, `friend.go:240`)

## Test Coverage
93.1% (target: 65%)

## Dependencies
**External:**
- `github.com/sirupsen/logrus` — Structured logging (justified for security audit trail)

**Standard Library:**
- `encoding/json` — Serialization for savedata persistence
- `sync` — Thread-safe RequestManager operations
- `time` — Timestamp tracking with abstraction via TimeProvider interface
- `errors`, `fmt` — Error handling

**Internal:**
- `github.com/opd-ai/toxcore/crypto` — Encryption/decryption and nonce generation

## Recommendations
1. ~~Fix Request.SenderPublicKey population in NewRequest or document intended usage pattern~~ — **DONE**: NewRequest derives SenderPublicKey from senderSecretKey
2. ~~Add explicit mutex protection to FriendInfo or document caller synchronization requirements in struct godoc~~ — **DONE**: Added sync.RWMutex with thread-safety documentation
3. ~~Review RequestManager.AddRequest handler callback pattern to prevent potential deadlock scenarios~~ — **DONE**: Refactored to properly release lock before handler callback
4. Update doc.go example to use LastSeenDuration() instead of non-existent GetLastSeen()
5. Add explicit error checking in test cases instead of swallowing errors with `_ =`
