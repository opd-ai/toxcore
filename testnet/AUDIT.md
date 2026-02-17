# Audit: github.com/opd-ai/toxcore/testnet
**Date**: 2026-02-17
**Status**: Needs Work

## Summary
The testnet package provides a comprehensive integration test suite for validating Tox protocol operations with bootstrap servers, test clients, and complete peer-to-peer workflows. The implementation is well-structured with clean separation of concerns but has 13 issues including non-deterministic time.Now() usage, standard library logging instead of structured logging, missing test coverage, and hardcoded port configurations.

## Issues Found
- [ ] **med** Error handling — Error reassignments shadow previous errors without checking them (`internal/protocol.go:144,149,183`)
- [ ] **low** Documentation — No doc.go file for internal package, relying on single bootstrap.go package comment
- [ ] **low** Test coverage — 0.0% test coverage for both cmd and internal packages (target: 65%)
- [ ] **med** Non-deterministic time — Extensive use of `time.Now()` in 11 locations for timestamps and metrics instead of injectable time source (`internal/bootstrap.go:88,111`, `internal/client.go:140,166,184,192,266,307,333`, `internal/orchestrator.go:148,214,347`)
- [ ] **low** Logging — Using standard library `log.Logger` instead of structured logging with `logrus.WithFields` for error context (111 logger calls across all files)
- [ ] **low** Documentation — Missing godoc comments for several exported functions: `DefaultClientConfig`, `DefaultProtocolConfig` constructors lack function-level docs
- [ ] **low** Hardcoded configuration — Magic port numbers embedded in code: 33445 (bootstrap), 33500-33599 (Alice), 33600-33699 (Bob), 33700-33799 (other) should be configurable constants (`internal/client.go:96-100`, `internal/orchestrator.go:103`, `internal/bootstrap.go:51`)
- [ ] **med** Resource management — Bootstrap server starts goroutine in Start() but no mechanism to wait for graceful shutdown completion; eventLoop may continue briefly after Stop() (`internal/bootstrap.go:108`)
- [ ] **low** Configuration validation — Client port range validation missing: could overlap or exceed valid port range 1-65535 (`internal/client.go:92-111`)
- [ ] **low** Error context — Generic error messages without structured fields: "timeout waiting for connection" lacks context about which client, public key, connection status (`internal/client.go:375`)
- [ ] **low** Test workflow — No integration with existing test suite: testnet is standalone executable not imported by parent module tests
- [ ] **low** Metrics — Metrics structures use sync.RWMutex but could benefit from atomic operations for simple counters (`internal/bootstrap.go:30-36`, `internal/client.go:70-77`)
- [ ] **low** Documentation — README.md example output shows hypothetical hex keys that don't match actual 64-character Ed25519 public key format (shows 64 chars but mixing different formatting)

## Test Coverage
0.0% (target: 65%)

The package has zero test coverage. All code is production implementation without accompanying unit tests, table-driven tests, or integration tests. This is particularly concerning for a test infrastructure package that other tests depend on.

## Integration Status
The testnet package is a standalone Go module (separate go.mod) that depends on the parent toxcore package via replace directive. It provides integration test infrastructure but is not integrated into the parent module's test suite. The package exports:

- `internal.TestOrchestrator` — Manages complete test execution workflow with configurable timeouts and retry logic
- `internal.BootstrapServer` — Localhost bootstrap server for test network initialization  
- `internal.TestClient` — Tox client wrapper with callback channels for testing
- `internal.ProtocolTestSuite` — Coordinates bootstrap/client/protocol workflow
- `cmd/main.go` — CLI executable for running integration tests

**Missing registrations**: N/A (test infrastructure, not integrated with parent system)

**Usage pattern**: Executed as standalone binary via `go run testnet/cmd/main.go` or built executable, not imported by other packages.

## Recommendations
1. **[HIGH]** Implement injectable time source (e.g., `TimeProvider` interface) to eliminate non-deterministic `time.Now()` calls for reproducible test runs
2. **[HIGH]** Add comprehensive unit tests for all internal packages with table-driven tests: `TestBootstrapServerLifecycle`, `TestClientCallbacks`, `TestOrchestratorWorkflow`, `TestProtocolSuite`, targeting 65%+ coverage
3. **[MED]** Replace standard library logging with `logrus.WithFields` structured logging to provide error context (client name, public key, operation type, retry attempt)
4. **[MED]** Fix error shadowing in protocol.go by checking errors before reassignment: `if err != nil { return fmt.Errorf("failed to connect Client A: %w", err) }`
5. **[MED]** Implement graceful goroutine shutdown with WaitGroup or done channel in bootstrap server eventLoop to ensure clean Stop()
6. **[LOW]** Extract hardcoded ports to named constants: `const (BootstrapDefaultPort = 33445; AlicePortRangeStart = 33500; ...)`
7. **[LOW]** Add port range validation in DefaultClientConfig to prevent overlapping ranges and invalid ports
8. **[LOW]** Create internal/doc.go with comprehensive package documentation explaining test workflow, architecture, and usage patterns
9. **[LOW]** Add benchmark tests for message throughput and friend request latency to validate performance characteristics
10. **[LOW]** Consider making testnet importable by parent module for reuse in other integration tests rather than CLI-only usage
