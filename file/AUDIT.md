# Audit: github.com/opd-ai/toxcore/file
**Date**: 2026-02-19
**Status**: Complete

## Summary
The file package implements peer-to-peer file transfer with chunked transmission, pause/resume/cancel controls, and stall detection. Code quality is high with proper concurrency safety, path validation against directory traversal, and good test coverage (83.9%). Security measures include chunk size limits, file name length restrictions, and resource exhaustion protections. Flow control is implemented via acknowledgment tracking.

## Issues Found
- [x] **low** Documentation — Outdated example in doc.go shows incorrect AddressResolver signature (`func(net.Addr) (uint32, bool)` instead of `func(net.Addr) (uint32, error)`) (`doc.go:62,108`) — **RESOLVED**: Updated examples to match actual interface signature.
- [x] **med** Concurrency Safety — Missing mutex protection in Transfer.OnProgress, Transfer.OnComplete callback setters allows race condition when setting callbacks from multiple goroutines (`transfer.go:612,619`) — **RESOLVED**: Mutex protection added (previously resolved).
- [ ] **low** Error Handling — Transfer.Cancel does not return or log file handle close error properly, only logs warning but swallows error in return (`transfer.go:376-384`)
- [ ] **med** API Design — Manager.SendFile takes raw net.Addr parameter but most callers will need to construct addresses; consider helper method or builder pattern (`manager.go:118`)
- [ ] **low** API Design — TimeProvider interface exposed publicly but defaultTimeProvider variable is package-private, inconsistent visibility (`transfer.go:82-98`)
- [x] **med** Integration — Manager.handleFileDataAck does not use acknowledged bytes for flow control or congestion management, acknowledgments are logged but not utilized (`manager.go:341-363`) — **RESOLVED**: Implemented flow control with SetAcknowledgedBytes, GetAcknowledgedBytes, GetPendingBytes, and OnAcknowledge callback. handleFileDataAck now updates transfer acknowledged bytes and logs pending bytes.

## Test Coverage
83.9% (target: 65%)

## Dependencies
**External:**
- `github.com/sirupsen/logrus` - Structured logging
- `github.com/opd-ai/toxcore/transport` - Network transport layer

**Standard Library:**
- `encoding/binary` - Packet serialization
- `os` - File I/O operations
- `sync` - Mutex for concurrency safety
- `time` - Transfer timing and timeout detection
- `path/filepath` - Path validation
- `net` - Network address abstraction (interface only, no concrete types)

**Integration Points:**
- Registers 4 packet handlers with transport layer (FileRequest, FileControl, FileData, FileDataAck)
- Requires AddressResolver for mapping network addresses to friend IDs
- Not yet integrated into main Tox struct (standalone usage only)

## Recommendations
1. ~~**Fix callback setter race condition**~~ — **RESOLVED**: Mutex protection in OnProgress/OnComplete (`transfer.go:612,619`)
2. ~~**Update doc.go AddressResolver examples**~~ — **RESOLVED**: Updated to `(net.Addr) (uint32, error)` (`doc.go:62,108`)
3. ~~**Implement flow control**~~ — **RESOLVED**: FileDataAck packets now update transfer's acknowledged bytes for flow control (`manager.go:341-363`)
4. **Standardize TimeProvider visibility** - Either export defaultTimeProvider or make TimeProvider interface package-private
5. **Add helper methods** - Consider Manager.SendFileByPath(friendID, filePath) wrapper that handles address resolution internally
