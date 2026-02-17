# Audit: github.com/opd-ai/toxcore/file
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The file package implements file transfer functionality for the Tox protocol with support for sending, receiving, pausing, and resuming transfers. While the implementation is feature-complete with good documentation, it has several error handling gaps and integration issues that need addressing.

## Issues Found
- [x] high error-handling — File handle Close() errors are not checked in Cancel() and complete() methods (`transfer.go:278`, `transfer.go:437`) — **RESOLVED**: Added error checking and logging for Close() calls in both Cancel() and complete() methods
- [x] high test-coverage — Tests fail to compile due to missing IsConnectionOriented() method in mockTransport, resulting in 0% coverage (`manager_test.go:85,116,171,195,226,294,411,412`) — **RESOLVED**: Added IsConnectionOriented() method to mockTransport returning false
- [x] high deterministic — Uses time.Now() directly for transfer speed calculations and timestamps, making timing non-deterministic (`transfer.go:99`, `transfer.go:177`, `transfer.go:454`) — **RESOLVED**: Implemented TimeProvider interface with DefaultTimeProvider for production and injectable mock for testing; Transfer struct now uses timeProvider field
- [x] med network-interfaces — Manager stores net.Addr in sentPacket struct but follows interface conventions elsewhere (`manager_test.go:21`) — **RESOLVED**: Verified that sentPacket struct correctly uses net.Addr interface type; test code uses mockAddr implementing net.Addr interface
- [x] med integration — Friend ID resolution is stubbed with placeholder using fileID, breaking proper friend-to-transfer mapping (`manager.go:151-153`) — **RESOLVED**: Implemented AddressResolver interface with SetAddressResolver() method; handlers now use resolveFriendIDFromAddr() helper for consistent friend ID resolution with fallback behavior; added comprehensive tests for resolver functionality
- [x] med error-handling — No validation of file path safety (directory traversal attacks possible) (`transfer.go:150`, `transfer.go:159`) — **RESOLVED**: Added `ValidatePath()` function with directory traversal detection using `filepath.Clean()` and ".." checks; `Start()` validates paths before opening files; added `ErrDirectoryTraversal` sentinel error
- [x] med error-handling — WriteChunk and ReadChunk do not validate chunk size limits (`transfer.go:291`, `transfer.go:357`) — **RESOLVED**: Added `MaxChunkSize` constant (65536 bytes); `WriteChunk` and `ReadChunk` validate chunk size against limit; added `ErrChunkTooLarge` sentinel error
- [ ] med integration — File transfer manager is not integrated into main Tox struct in toxcore.go (standalone only)
- [ ] low doc-coverage — Missing package-level doc.go file (only inline package comment in transfer.go)
- [ ] low error-handling — serializeFileRequest does not handle excessively long file names (potential DoS) (`manager.go:285`)
- [ ] low error-handling — No timeout mechanism for stalled transfers in Transfer struct
- [ ] low test-coverage — Missing table-driven tests for TransferState transitions and error conditions
- [ ] low test-coverage — No benchmarks for chunk serialization/deserialization performance

## Test Coverage
Tests now compile and pass (was 0%; now functional)
Target: 65%

**Resolved Test Compilation Errors:**
- mockTransport now implements IsConnectionOriented() method
- All 9 test functions across manager_test.go now compile and pass
- Added comprehensive tests for AddressResolver functionality (3 new test functions)

## Integration Status
The file package is partially integrated:
- ✅ Packet types registered in transport/packet.go (PacketFileRequest, PacketFileControl, PacketFileData, PacketFileDataAck)
- ✅ Transport layer integration via Manager.NewManager(transport.Transport)
- ✅ Example code exists in examples/file_transfer_demo/
- ✅ AddressResolver interface for friend ID resolution from network addresses
- ❌ Not integrated into main Tox struct (toxcore.go has no fileManager field)
- ❌ No registration in system initialization or bootstrap process
- ⚠️  No persistence/serialization support for active transfers (no savedata integration)

## Recommendations
1. ~~**Fix test compilation** — Add IsConnectionOriented() bool method to mockTransport returning false (high priority)~~ — **DONE**
2. ~~**Add deterministic time abstraction** — Introduce TimeProvider interface with Now() method; inject into Transfer struct to allow test control~~ — **DONE**
3. ~~**Implement friend ID mapping** — Replace fileID placeholder with proper connection-to-friendID resolution from transport layer~~ — **DONE**: Added AddressResolver interface with SetAddressResolver() and resolveFriendIDFromAddr() helper; maintains backward compatibility with fallback to fileID when resolver not configured
4. ~~**Add file handle close checks** — Wrap FileHandle.Close() calls with error logging in Cancel() and complete() methods~~ — **DONE**
5. ~~**Validate file paths** — Use filepath.Clean() and check for directory traversal patterns before opening files~~ — **DONE**: Implemented `ValidatePath()` function with comprehensive traversal detection; added `ErrDirectoryTraversal` sentinel error; `Start()` validates paths before file operations
6. ~~**Add chunk size validation** — Enforce ChunkSize limit in ReadChunk size parameter and WriteChunk data length~~ — **DONE**: Added `MaxChunkSize` constant (65536 bytes); both `WriteChunk` and `ReadChunk` validate against limit; added `ErrChunkTooLarge` sentinel error; comprehensive tests added
7. **Integrate with main Tox** — Add fileManager field to Tox struct with getter methods and lifecycle management
8. **Create doc.go** — Add comprehensive package-level documentation with architecture overview and usage examples
9. **Add table-driven tests** — Create test tables for state transitions, error conditions, and edge cases
10. **Implement transfer timeouts** — Add configurable timeout mechanism to detect and handle stalled transfers
