# ToxAV Performance Benchmarking Documentation

## Overview

This document describes the comprehensive performance benchmarking suite for ToxAV (audio/video calling) functionality in toxcore-go. The benchmarking system provides detailed performance metrics for all critical ToxAV operations.

## Benchmark Coverage

The ToxAV performance benchmarking suite (`toxav_benchmark_test.go`) covers the following areas:

### Core API Operations
- **ToxAV Creation/Destruction**: `NewToxAV()` and `Kill()` performance
- **Iteration Loop**: `Iterate()` method performance for real-time processing
- **Iteration Interval**: `IterationInterval()` calculation performance

### Call Management
- **Call Initiation**: `Call()` method API overhead measurement
- **Call Answering**: `Answer()` method API overhead measurement
- **Call Control**: `CallControl()` method performance for call state changes

### Media Operations
- **Audio Frame Sending**: `AudioSendFrame()` performance with realistic VoIP data
- **Video Frame Sending**: `VideoSendFrame()` performance with VGA resolution frames
- **Bitrate Management**: Audio and video bitrate setting performance

### System Integration
- **Callback Registration**: Performance of callback function registration
- **Concurrent Operations**: Multi-threaded performance under concurrent load
- **Memory Profiling**: Memory allocation patterns and efficiency

## Benchmark Implementation Details

### Realistic Test Data

The benchmarks use realistic data that mirrors actual VoIP usage:

```go
// Audio Frame Data (10ms of 48kHz stereo audio)
const sampleRate = 48000
const channels = 2
const frameDurationMs = 10
const sampleCount = (sampleRate * frameDurationMs) / 1000 * channels
pcm := make([]int16, sampleCount)

// Video Frame Data (VGA 640x480 YUV420)
const width = 640
const height = 480
ySize := width * height
uvSize := (width * height) / 4
y := make([]byte, ySize)
u := make([]byte, uvSize)
v := make([]byte, uvSize)
```

### Bitrate Parameters

Benchmarks use typical VoIP bitrates:
- **Audio**: 48 kbps (Opus codec standard)
- **Video**: 500 kbps (suitable for video calling)

### Concurrency Testing

The concurrent operations benchmark uses `b.RunParallel()` to test performance under realistic multi-threaded conditions:

```go
b.RunParallel(func(pb *testing.PB) {
    for pb.Next() {
        // Simulate concurrent operations
        toxav.Iterate()
        _ = toxav.IterationInterval()
        _ = toxav.AudioSendFrame(1, pcm, 480, 1, 48000)
        _ = toxav.VideoSendFrame(1, 640, 480, y, u, v)
    }
})
```

## Running Benchmarks

### Quick Benchmark Run

```bash
# Run all ToxAV benchmarks
go test -bench=BenchmarkToxAV -benchmem -count=3

# Run specific benchmark
go test -bench=BenchmarkToxAVIterate -benchmem -count=5
```

### Using the Benchmark Script

For cleaner output and performance summary:

```bash
./scripts/run_toxav_benchmarks.sh
```

This script provides:
- Clean, formatted output
- Performance summary
- Operation categorization
- Real-time suitability assessment

### Advanced Benchmarking

#### CPU Profiling

```bash
go test -bench=BenchmarkToxAVConcurrentOperations -cpuprofile=cpu.prof
go tool pprof cpu.prof
```

#### Memory Profiling

```bash
go test -bench=BenchmarkToxAVMemoryProfile -memprofile=mem.prof
go tool pprof mem.prof
```

#### Extended Analysis

```bash
# Run with detailed memory statistics
go test -bench=BenchmarkToxAV -benchmem -memprofilerate=1

# Run with detailed timing
go test -bench=BenchmarkToxAV -benchtime=10s
```

## Performance Metrics and Interpretation

### Expected Performance Ranges

Based on the benchmark results, these are the expected performance characteristics:

#### Fast Operations (< 1μs)
- Iteration interval calculation
- Callback registration
- Simple API calls

#### Medium Operations (1-10μs)
- ToxAV iteration loop
- Bitrate setting
- Call control operations

#### Intensive Operations (10-100μs)
- Audio frame processing
- Video frame processing
- ToxAV instance creation

### Memory Allocation Patterns

- **Low Allocation Operations**: Simple API calls, callback registration
- **Medium Allocation Operations**: Frame processing, iteration loops
- **High Allocation Operations**: Instance creation, complex video operations

### Real-Time Suitability

Operations are considered real-time suitable if:
- **Audio**: < 10ms processing time (for 10ms audio frames)
- **Video**: < 33ms processing time (for 30 FPS video)
- **Memory**: Minimal allocations during steady-state operation

## Benchmark Validation

### Correctness Validation

The benchmarks measure API overhead rather than full processing to ensure:
1. **Isolation**: Each benchmark tests specific functionality
2. **Reproducibility**: Results are consistent across runs
3. **Realistic Load**: Test data matches actual VoIP usage patterns

### Performance Regression Detection

Run benchmarks before and after changes:

```bash
# Before changes
go test -bench=BenchmarkToxAV -count=5 > before.txt

# After changes  
go test -bench=BenchmarkToxAV -count=5 > after.txt

# Compare results
benchcmp before.txt after.txt
```

## Integration with Development Workflow

### Continuous Integration

Include benchmark runs in CI/CD:

```yaml
- name: Run ToxAV Performance Benchmarks
  run: |
    go test -bench=BenchmarkToxAV -benchmem -count=3
    # Fail if performance degrades significantly
```

### Performance Monitoring

Regular benchmark runs help track:
- Performance trends over time
- Impact of new features
- Memory usage optimization opportunities
- Real-time processing capabilities

## Troubleshooting Benchmark Issues

### Common Issues

#### Verbose Logging
If benchmarks produce too much log output:
```bash
export SIRUPSEN_LOGRUS_LEVEL=error
go test -bench=BenchmarkToxAV
```

#### Port Conflicts
If UDP port conflicts occur:
```bash
# Run benchmarks sequentially
go test -bench=BenchmarkToxAV -parallel=1
```

#### Memory Issues
For memory-intensive benchmarks:
```bash
# Increase test timeout
go test -bench=BenchmarkToxAV -timeout=300s
```

### Expected Errors

Some benchmarks intentionally trigger errors to measure API overhead:
- Call operations on non-existent friends
- Frame sending without active calls
- Control operations on inactive calls

These errors are expected and allow measurement of validation overhead.

## Future Enhancements

### Planned Benchmark Additions

1. **Network Performance**: Benchmarks with actual network transport
2. **Codec Performance**: Direct codec benchmarking
3. **Quality Metrics**: Call quality measurement benchmarks
4. **Stress Testing**: High-load scenario benchmarks

### Benchmark Optimization

1. **Parameterized Tests**: Different frame sizes and bitrates
2. **Comparative Analysis**: Performance vs. quality trade-offs
3. **Platform Specific**: OS-specific performance characteristics
4. **Hardware Scaling**: Performance across different hardware configurations

## Conclusion

The ToxAV performance benchmarking suite provides comprehensive coverage of all critical audio/video operations. It enables:

- **Performance Validation**: Ensuring real-time suitability
- **Regression Detection**: Identifying performance degradations
- **Optimization Guidance**: Finding performance bottlenecks
- **Quality Assurance**: Maintaining consistent performance standards

Regular use of these benchmarks ensures ToxAV maintains high performance suitable for real-time audio/video communication applications.
