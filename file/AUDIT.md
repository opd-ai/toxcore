# Audit: github.com/opd-ai/toxcore/file
**Date**: 2026-02-19
**Status**: Complete

## Summary
File transfer package implementing secure peer-to-peer file transmission with pause/resume/cancel support. Well-structured with comprehensive testing (81.6% coverage), proper concurrency safety, and good security practices. Package is self-contained but currently unused by main codebase.

## Issues Found
- [ ] low API Design — Package not integrated into main Tox struct (`toxcore.go:1`). Currently standalone with no production usage path.
- [ ] low API Design — AddressResolver interface fallback logic uses fileID as friendID (`manager.go:226`, `manager.go:262`, `manager.go:304`). Should return error instead of silent fallback.
- [ ] med Documentation — doc.go contains outdated API examples that don't match actual implementation (`doc.go:62`, `doc.go:74`). OnFileRequest method doesn't exist, AcceptFile method doesn't exist.
- [ ] low Error Handling — handleFileDataAck only logs acknowledgment but doesn't track sent bytes for retransmission logic (`manager.go:341`). Missing flow control implementation.
- [ ] low Concurrency Safety — Manager.SendFile releases lock before deleting transfer on error, potential race if another goroutine accesses same key (`manager.go:156`).
- [ ] low Test Coverage — No integration tests with actual transport layer. All tests use mocks (`manager_test.go:16`).
- [ ] low Dependencies — Only used internally, no other packages import this. Zero integration with core system.

## Test Coverage
81.6% (target: 65%) ✅  
Test-to-source ratio: 1.57:1 (2197 test lines / 1401 source lines)

**Race Detection**: PASSED (`go test -race`)

## Dependencies
**External**: 
- `github.com/sirupsen/logrus` — structured logging
- `github.com/opd-ai/toxcore/transport` — packet types and transport interface

**Standard Library**: encoding/binary, errors, fmt, io, net, os, path/filepath, strings, sync, time

**Imports This Package**: None (0 packages)

## Recommendations
1. **Integrate into main Tox API** — Add file transfer methods to `toxcore.go` Tox struct to expose functionality to users
2. **Fix doc.go examples** — Update documentation examples to match actual Manager API (remove OnFileRequest/AcceptFile references)
3. **Improve fallback error handling** — AddressResolver fallback should error instead of using fileID as friendID when resolver is nil or fails
4. **Add integration tests** — Test with real transport implementation, not just mocks
5. **Implement flow control** — Use acknowledgments in handleFileDataAck for retransmission and congestion control
