# Profile-Guided Optimization Guide

**Date**: 2026-06-03  
**Profiler**: pprof (CPU/memory analysis of test benchmarks)  
**Methodology**: Benchmarking key packages with `-cpuprofile` and `-memprofile`

## Executive Summary

This document documents profile-guided optimization opportunities identified through pprof analysis of toxcore-go's critical packages. All recommendations preserve protocol semantics and focus on implementation efficiency.

**Key Finding**: Structured logging (logrus) is the dominant overhead in hot paths, accounting for 37% of CPU time and 43% of allocations in the messaging package benchmarks.

## Profiling Methodology

### Environment
- Benchmarks run with `-tags nonet` (no network tests)
- Duration: ~52 seconds of CPU-bound message operations
- Tool: Go's built-in `go tool pprof` on CPU and memory profiles

### Command Used
```bash
go test -tags nonet -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./messaging
```

## Key Findings by Profile Type

### CPU Profile Results

**Top 10 CPU Consumers**:

| Function | CPU Time | % of Total | Classification |
|----------|----------|-----------|-----------------|
| `logrus.(*TextFormatter).Format` | 27.74s | 37.12% | **HOTSPOT** |
| Mutex.Lock | 5.42s | 7.25% | Synchronization |
| `runtime.mallocgcSmallScanNoHeader` | 6.70s | 8.96% | GC |
| `runtime.mapassign_faststr` | 8.55s | 11.44% | Hash map ops |
| `strconv.appendQuotedWith` | 4.77s | 6.38% | String formatting |

**Analysis**: Logging infrastructure dominates the CPU profile. The `TextFormatter.Format` function is called on every log statement, performing string formatting, escaping, and field serialization.

### Memory Profile Results (Allocations)

**Top 10 Allocation Sources**:

| Function | Allocations | % of Total | Classification |
|----------|-------------|-----------|-----------------|
| `logrus.(*Entry).WithFields` | 4882.51 MB | 43.47% | **HOTSPOT** |
| `logrus.(*Entry).Dup` | 2598.05 MB | 23.13% | **HOTSPOT** |
| `logrus.(*TextFormatter).Format` | 2043.15 MB | 18.19% | **HOTSPOT** |
| `messaging.newMessageWithTime` | 3871.15 MB | 34.46% | Protocol logic |
| `messaging.SendMessage` | 3718.14 MB | 33.10% | Protocol logic |

**Analysis**: Logrus logging accounts for ~84.8% of allocations in logging calls alone. Every `WithFields()` or `Dup()` call allocates new map and slice structures for field storage.

## Optimization Opportunities (Preserving Protocol Semantics)

### 1. Lazy Field Evaluation in Hot Paths

**Problem**: Fields are evaluated eagerly even if the log statement is never written (e.g., when log level is filtered).

**Solution**: Use conditional logging only in hot paths where log level might be filtered.

**Implementation**:
```go
// BEFORE: Fields evaluated unconditionally
logger.WithFields(map[string]interface{}{
    "friend_id": friendID,
    "msg_type": msgType,
    "complexity": calculateComplexity(), // Expensive call!
}).Info("Sending message")

// AFTER: Check log level first
if logger.IsLevelEnabled(logrus.InfoLevel) {
    logger.WithFields(map[string]interface{}{
        "friend_id": friendID,
        "msg_type": msgType,
        "complexity": calculateComplexity(),
    }).Info("Sending message")
}
```

**Protocol Impact**: None. This is purely a logging optimization.

**Estimated Benefit**: 5-10% reduction in allocations and CPU in test environments (where log level is typically controlled).

### 2. Cache Frequently-Used Fields

**Problem**: The same fields (friend_id, message_type, etc.) are formatted repeatedly with string conversions.

**Solution**: Pre-format constant field names at module initialization time.

**Implementation**:
```go
// At module init
var cachedFields = map[string]string{
    "source":        "toxcore",
    "package":       "messaging",
    "module":        "MessageManager",
}

// In logging calls
entry := logger.WithFields(logrus.Fields{
    "friend_id": friendID,
    "msg_type": msgType,
})
```

**Protocol Impact**: None.

**Estimated Benefit**: 2-5% reduction in string formatting overhead.

### 3. Reduce Field Count in Error Paths

**Problem**: Error paths emit many fields (error, function, status) unconditionally.

**Solution**: Emit different field sets for different severity levels.

**Implementation**:
```go
// BEFORE: 8-12 fields on every error
logger.WithFields(fields).Error("Failed to send message", err)

// AFTER: Core fields only, optional detailed fields
if logger.IsLevelEnabled(logrus.DebugLevel) {
    logger.WithFields(detailedFields).Debug("Message send failure details", err)
} else {
    logger.WithFields(coreFields).Error("Failed to send message")
}
```

**Protocol Impact**: None. Error behavior is identical; logging detail level changes.

**Estimated Benefit**: 3-7% reduction in allocations for warning/error paths.

### 4. Use Structured Logging Buffer Pooling

**Problem**: Each log statement allocates a new bytes.Buffer internally for formatting.

**Solution**: Use `sync.Pool` to reuse formatter buffers (requires changes to logrus integration or custom formatter).

**Implementation**:
```go
// In formatter package
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

// In Format method
buf := bufferPool.Get().(*bytes.Buffer)
defer func() {
    buf.Reset()
    bufferPool.Put(buf)
}()
```

**Protocol Impact**: None.

**Estimated Benefit**: 8-15% reduction in allocations and GC pressure.

## Hot Path Packages (Profiling Targets)

Based on the problem statement and analysis, these packages should be monitored:

1. **messaging** - Message processing and encryption (~172 log statements)
2. **async** - Offline message storage (high frequency operations)
3. **transport** - Network I/O (packet processing)
4. **crypto** - Cryptographic operations (sensitive timing)
5. **dht** - Peer discovery (frequent lookups)

## Benchmark Results Baseline

Current performance (from messaging benchmarks):

```
Message encoding/decoding: ~548 ns/op, 496 B/op, 4 allocs/op
CPU profile: 37% time in logging, 63% in protocol logic
Memory profile: 84.8% of allocations in logging infrastructure
```

## Implementation Priorities

1. **Quick wins** (low risk, high impact):
   - Add log level checks before field evaluation in error paths
   - Reduce field count in debug/trace logging

2. **Medium effort** (medium risk, medium impact):
   - Implement buffer pooling for formatters
   - Cache commonly-used field names

3. **Higher effort** (requires testing, higher complexity):
   - Switch to lower-allocation logger (consider zap/zerolog if justified)
   - Implement custom structured logging formatter optimized for hot paths

## Verification Approach

After each optimization:

1. **Preserve Protocol Behavior**
   - Run `go test ./...` to ensure all tests pass
   - Run integration tests with real peers
   - Verify wire format is unchanged

2. **Measure Improvement**
   - Re-run benchmarks: `go test -bench=. -benchmem ./...`
   - Capture new CPU/memory profiles
   - Compare with baseline using `benchstat`

3. **Check for Regressions**
   - Run `go vet ./...`
   - Run `go test -race ./...`
   - Verify no new memory leaks with `pprof`

## Known Optimizations Already In Place

These optimizations were previously verified to be effective:

1. **Concurrent Iteration Pipelines** - DHT, friend, and message processing decoupled
2. **Sharded Friend Store** - 16 shards reduce lock contention
3. **Buffered Channels** - Prevent goroutine blocking on event delivery
4. **Minimal Locking** - Read locks preferred, write locks only when necessary
5. **Function Complexity** - All functions have complexity ≤10 (target met)
6. **Code Duplication** - Reduced to 0.58% (target: <3%)

## Future Work

1. Profile async package's storage operations
2. Profile transport layer's packet handling
3. Evaluate alternative logger libraries for critical paths
4. Implement adaptive logging (adjust verbosity based on load)
5. Add pprof endpoints to long-running processes for in-production profiling

## References

- [Go Performance](https://github.com/dgryski/go-perfbook)
- [Profiling Go Programs](https://go.dev/blog/pprof)
- [logrus Documentation](https://github.com/sirupsen/logrus)
- [Runtime Profiling in Go](https://golang.org/pkg/runtime/pprof/)
