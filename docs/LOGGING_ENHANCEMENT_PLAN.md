# Toxcore Go Logging Infrastructure Enhancement Plan

## Executive Summary

Current logging coverage: **67.7%** (903/1,333 functions)
Required threshold: **97%**
**Action Required**: Comprehensive logging enhancement with structured logging migration

## Enhancement Strategy

### Phase 1: Core Infrastructure Enhancement (Priority 1)

#### 1.1 Crypto Module Logging Enhancement
**Target Files**: `crypto/*.go`
- Add function entry/exit logging for all cryptographic operations
- Implement secure parameter logging (without exposing sensitive data)
- Enhanced error context for key operations

#### 1.2 DHT Module Logging Enhancement  
**Target Files**: `dht/*.go`
- Add peer discovery operation logging
- Network routing decision logging
- Bootstrap process tracking

#### 1.3 Transport Layer Logging Enhancement
**Target Files**: `transport/*.go`, `net/*.go`
- Connection lifecycle logging
- Packet transmission/reception logging
- Network error analysis logging

### Phase 2: Application Layer Enhancement (Priority 2)

#### 2.1 Friend Management Logging
**Target Files**: `friend/*.go`
- Friend request processing logging
- Relationship state change logging
- Friend discovery and validation logging

#### 2.2 Messaging System Logging
**Target Files**: `messaging/*.go`
- Message processing pipeline logging
- Delivery status tracking
- Queue management logging

#### 2.3 File Transfer Logging
**Target Files**: `file/*.go`
- Transfer progress logging
- Chunk processing logging
- Error recovery logging

### Phase 3: Advanced Features Enhancement (Priority 3)

#### 3.1 Async Messaging Logging
**Target Files**: `async/*.go`
- Forward secrecy operations logging
- Identity obfuscation logging
- Storage node interaction logging

#### 3.2 AV Processing Logging
**Target Files**: `av/*.go`
- Audio/video processing pipeline logging
- Codec operation logging
- RTP transport logging

## Structured Logging Standards

### Required Fields for All Log Entries

```go
logrus.WithFields(logrus.Fields{
    "function": "FunctionName",
    "package":  "package.subpackage",
    "caller":   runtime.Caller(1), // File:Line info
    // Operation-specific fields
}).LogLevel("Message")
```

### Security-Sensitive Logging Guidelines

1. **Never Log**: Private keys, passwords, personal data
2. **Hash Before Logging**: Public keys, addresses (first 8 chars + "...")
3. **Sanitize**: User input, file paths
4. **Redact**: Sensitive configuration values

### Error Context Standards

```go
logrus.WithFields(logrus.Fields{
    "function":    "FunctionName",
    "package":     "package.name",
    "error":       err.Error(),
    "error_type":  reflect.TypeOf(err).String(),
    "operation":   "specific_operation",
    // Context-specific fields
}).Error("Operation failed")
```

## Implementation Approach

### 1. Automated Logging Injection

Create tooling to automatically add logging to functions lacking coverage:

```go
// Template for function entry logging
func ExampleFunction(param1 string, param2 int) error {
    logrus.WithFields(logrus.Fields{
        "function": "ExampleFunction",
        "package":  "example.package",
        "param1":   param1,
        "param2":   param2,
    }).Debug("Function entry")
    
    defer func() {
        logrus.WithFields(logrus.Fields{
            "function": "ExampleFunction", 
            "package":  "example.package",
        }).Debug("Function exit")
    }()
    
    // Function implementation...
}
```

### 2. Enhanced Error Handling

Upgrade error handling with structured context:

```go
// Before
return fmt.Errorf("operation failed: %v", err)

// After  
logrus.WithFields(logrus.Fields{
    "function":   "FunctionName",
    "package":    "package.name", 
    "error":      err.Error(),
    "operation":  "specific_operation",
    "context":    additionalContext,
}).Error("Operation failed")
return fmt.Errorf("operation failed in %s: %w", "specific_operation", err)
```

### 3. Goroutine and Concurrency Logging

Enhanced logging for concurrent operations:

```go
func goroutineWorker(ctx context.Context, id int) {
    logger := logrus.WithFields(logrus.Fields{
        "function":    "goroutineWorker",
        "package":     "async.worker",
        "goroutine_id": id,
        "worker_type":  "message_processor",
    })
    
    logger.Info("Goroutine started")
    defer logger.Info("Goroutine terminated")
    
    for {
        select {
        case <-ctx.Done():
            logger.WithField("reason", "context_cancelled").Info("Stopping worker")
            return
        // Worker logic with logging...
        }
    }
}
```

### 4. Performance and Resource Logging

Add performance tracking and resource monitoring:

```go
func performanceAwareFunction() error {
    start := time.Now()
    logger := logrus.WithFields(logrus.Fields{
        "function": "performanceAwareFunction",
        "package":  "performance.monitor",
    })
    
    defer func() {
        duration := time.Since(start)
        logger.WithFields(logrus.Fields{
            "duration_ms": duration.Milliseconds(),
            "memory_used": getMemoryUsage(),
        }).Info("Function completed")
    }()
    
    // Function implementation...
}
```

## Validation and Testing

### 1. Coverage Verification

Automated tooling to verify logging coverage:

```bash
# Check function coverage
go run scripts/check_logging_coverage.go

# Validate log format consistency  
go run scripts/validate_log_format.go

# Test log output in different scenarios
go test -v ./... -tags=logging_test
```

### 2. Performance Impact Assessment

Monitor logging performance impact:

- Benchmark critical paths with/without logging
- Memory allocation analysis for log generation
- Configurable log levels for production optimization

### 3. Security Audit

Verify no sensitive data exposure:

- Automated scanning for potential data leaks
- Review of all logged fields for security implications
- Compliance with privacy requirements

## Expected Outcomes

### Post-Enhancement Metrics

- **Target Coverage**: 98%+ (1,307+ functions with logging)
- **Error Context**: 100% of error paths with structured logging
- **Security Compliance**: 0 sensitive data exposures
- **Performance Impact**: <5% overhead in critical paths

### Maintainability Improvements

1. **Debugging Efficiency**: Comprehensive trace through any operation
2. **Production Monitoring**: Structured logs for automated analysis
3. **Security Auditing**: Complete audit trail for compliance
4. **Performance Analysis**: Detailed timing and resource usage data

## Timeline and Milestones

### Week 1-2: Core Infrastructure (Phase 1)
- Crypto module enhancement
- DHT module enhancement  
- Transport layer enhancement

### Week 3-4: Application Layer (Phase 2)
- Friend management enhancement
- Messaging system enhancement
- File transfer enhancement

### Week 5-6: Advanced Features (Phase 3)
- Async messaging enhancement
- AV processing enhancement
- Testing and validation

### Week 7: Final Integration
- Performance optimization
- Security audit
- Documentation completion

## Success Criteria

✅ **Coverage**: ≥97% function logging coverage
✅ **Structure**: All logs use logrus.WithFields pattern
✅ **Context**: Mandatory caller context in all entries
✅ **Security**: Zero sensitive data exposure
✅ **Performance**: <5% performance impact
✅ **Maintainability**: Consistent logging patterns across codebase

This enhancement plan transforms the toxcore codebase from good (67.7%) to excellent (98%+) logging coverage while maintaining security, performance, and maintainability standards.
