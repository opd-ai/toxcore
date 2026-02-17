# Audit: github.com/opd-ai/toxcore/file
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The file package implements file transfer functionality for the Tox protocol with support for sending, receiving, pausing, and resuming transfers. While the implementation is feature-complete with good documentation, it has critical test failures, non-deterministic timing code, and several error handling gaps that need addressing.

## Issues Found
- [ ] high error-handling — File handle Close() errors are not checked in Cancel() and complete() methods (`transfer.go:278`, `transfer.go:437`)
- [ ] high test-coverage — Tests fail to compile due to missing IsConnectionOriented() method in mockTransport, resulting in 0% coverage (`manager_test.go:85,116,171,195,226,294,411,412`)
- [ ] high deterministic — Uses time.Now() directly for transfer speed calculations and timestamps, making timing non-deterministic (`transfer.go:99`, `transfer.go:177`, `transfer.go:454`)
- [ ] med network-interfaces — Manager stores net.Addr in sentPacket struct but follows interface conventions elsewhere (`manager_test.go:21`)
- [ ] med integration — Friend ID resolution is stubbed with placeholder using fileID, breaking proper friend-to-transfer mapping (`manager.go:151-153`)
- [ ] med error-handling — No validation of file path safety (directory traversal attacks possible) (`transfer.go:150`, `transfer.go:159`)
- [ ] med error-handling — WriteChunk and ReadChunk do not validate chunk size limits (`transfer.go:291`, `transfer.go:357`)
- [ ] med integration — File transfer manager is not integrated into main Tox struct in toxcore.go (standalone only)
- [ ] low doc-coverage — Missing package-level doc.go file (only inline package comment in transfer.go)
- [ ] low error-handling — serializeFileRequest does not handle excessively long file names (potential DoS) (`manager.go:285`)
- [ ] low error-handling — No timeout mechanism for stalled transfers in Transfer struct
- [ ] low test-coverage — Missing table-driven tests for TransferState transitions and error conditions
- [ ] low test-coverage — No benchmarks for chunk serialization/deserialization performance

## Test Coverage
0% (tests do not compile; target: 65%)

**Test Compilation Errors:**
- mockTransport missing IsConnectionOriented() method to satisfy transport.Transport interface
- 9 test functions affected across manager_test.go

## Integration Status
The file package is partially integrated:
- ✅ Packet types registered in transport/packet.go (PacketFileRequest, PacketFileControl, PacketFileData, PacketFileDataAck)
- ✅ Transport layer integration via Manager.NewManager(transport.Transport)
- ✅ Example code exists in examples/file_transfer_demo/
- ❌ Not integrated into main Tox struct (toxcore.go has no fileManager field)
- ❌ No registration in system initialization or bootstrap process
- ⚠️  Friend ID resolution is stubbed (manager.go:153) - uses fileID as friendID placeholder
- ⚠️  No persistence/serialization support for active transfers (no savedata integration)

## Recommendations
1. **Fix test compilation** — Add IsConnectionOriented() bool method to mockTransport returning false (high priority)
2. **Add deterministic time abstraction** — Introduce TimeProvider interface with Now() method; inject into Transfer struct to allow test control
3. **Implement friend ID mapping** — Replace fileID placeholder with proper connection-to-friendID resolution from transport layer
4. **Add file handle close checks** — Wrap FileHandle.Close() calls with error logging in Cancel() and complete() methods
5. **Validate file paths** — Use filepath.Clean() and check for directory traversal patterns before opening files
6. **Add chunk size validation** — Enforce ChunkSize limit in ReadChunk size parameter and WriteChunk data length
7. **Integrate with main Tox** — Add fileManager field to Tox struct with getter methods and lifecycle management
8. **Create doc.go** — Add comprehensive package-level documentation with architecture overview and usage examples
9. **Add table-driven tests** — Create test tables for state transitions, error conditions, and edge cases
10. **Implement transfer timeouts** — Add configurable timeout mechanism to detect and handle stalled transfers
