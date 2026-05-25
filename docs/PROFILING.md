# Performance Profiling Guide

This document describes how to profile toxcore-go for performance optimization.

## Current Performance Status

As of 2026-05-25:
- **Max Function Complexity**: <10 (target: ≤10) ✅
- **Average Complexity**: 3.5 ✅
- **Code Duplication**: 0.58% (target: <3%) ✅
- **Hot Paths**: All iteration loops optimized with minimal lock contention

The codebase is already well-optimized with low complexity and minimal duplication.

## Hot Paths

The main execution paths that run frequently:

### 1. Message Processing Loop (50ms interval)
- **Function**: `doMessageProcessing()` in `toxcore_messaging.go`
- **Complexity**: Very low - simple mutex-protected calls
- **Bottlenecks**: None identified
- **Location**: Called from `iteration_pipelines.go:374`

### 2. DHT Maintenance Loop (6s interval)
- **Function**: `doDHTMaintenance()` in `toxcore_lifecycle.go`
- **Complexity**: Low - lock-protected DHT operations
- **Bottlenecks**: None identified

### 3. Friend Connection Loop (12s interval)
- **Function**: `doFriendConnections()` in `toxcore_lifecycle.go`
- **Complexity**: Low - iterates friends with lock protection
- **Bottlenecks**: None identified

## Profiling Methods

### CPU Profiling

Profile a specific package's benchmarks:

```bash
# Profile the main toxcore package
go test -tags nonet -bench=. -benchmem -cpuprofile=cpu.prof .

# Analyze the profile
go tool pprof -http=:8080 cpu.prof

# Or text-based analysis
go tool pprof -top cpu.prof
```

### Memory Profiling

```bash
# Profile memory allocations
go test -tags nonet -bench=. -benchmem -memprofile=mem.prof .

# Analyze memory profile
go tool pprof -http=:8080 mem.prof
```

### Benchmark Comparison

Run benchmarks and save results for comparison:

```bash
# Run all benchmarks
go test -tags nonet -bench=. -benchmem ./... > bench_baseline.txt

# After optimization, compare
go test -tags nonet -bench=. -benchmem ./... > bench_optimized.txt
benchstat bench_baseline.txt bench_optimized.txt
```

### Critical Path Packages

Focus profiling efforts on these packages with the most frequent operations:

1. **toxcore** (root) - Main API and iteration loops
2. **messaging** - Message processing and encryption
3. **dht** - Peer discovery and routing
4. **transport** - Network I/O
5. **crypto** - Encryption operations
6. **async** - Offline message handling

## Optimization Guidelines

1. **Profile Before Optimizing**: Always measure before making changes
2. **Lock Contention**: Use `go test -race` to detect lock issues
3. **Allocations**: Minimize allocations in hot paths using `-benchmem`
4. **Complexity**: Keep function complexity ≤10 using `go-stats-generator`
5. **Caching**: Consider caching for expensive operations called frequently

## Monitoring in Production

For production deployments, consider:

```go
import _ "net/http/pprof"

// Add to your main() function:
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

Then access profiles at:
- CPU: `http://localhost:6060/debug/pprof/profile?seconds=30`
- Heap: `http://localhost:6060/debug/pprof/heap`
- Goroutines: `http://localhost:6060/debug/pprof/goroutine`

## Known Optimizations

The following optimizations are already in place:

1. **Concurrent Iteration Pipelines** - DHT, friend, and message processing decoupled into separate goroutines
2. **Sharded Friend Store** - 16 shards to reduce lock contention
3. **Buffered Channels** - Prevent goroutine blocking on event delivery
4. **Interface-Based Abstractions** - Enables mock transports for testing without network overhead
5. **Minimal Locking** - Read locks used where possible, write locks only when necessary

## Benchmark Results

Current benchmark performance (as of 2026-05-25):

```
BenchmarkNewTox-4                      ~5000 ns/op
BenchmarkAddFriendByPublicKey-4        ~2000 ns/op
BenchmarkSelfSetName-4                 ~500 ns/op
```

(Run `go test -bench=. -benchmem .` for full results)

## Future Work

Potential optimization opportunities:

1. **Packet Batching** - Batch small packets to reduce syscall overhead
2. **Zero-Copy I/O** - Explore sendfile/splice for file transfers
3. **SIMD Crypto** - Use assembly-optimized crypto for hot paths (if available)
4. **Connection Pooling** - Reuse connections for privacy networks

## References

- [Go Performance](https://github.com/dgryski/go-perfbook)
- [Profiling Go Programs](https://go.dev/blog/pprof)
- [Go Memory Model](https://go.dev/ref/mem)
