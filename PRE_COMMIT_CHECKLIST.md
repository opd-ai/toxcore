# Pre-Commit Checklist for toxcore-go

Use this checklist before committing code to ensure adherence to project standards.

---

## 📋 Code Quality Checklist

### Cross-Platform Compatibility
- [ ] No hardcoded `/tmp` paths - use `os.TempDir()` instead
- [ ] All file paths use `filepath.Join()` instead of string concatenation
- [ ] No platform-specific syscalls without `runtime.GOOS` checks
- [ ] No Unix-specific shell commands without Windows alternatives
- [ ] File permissions are cross-platform compatible

### Modern Go Idioms (Go 1.18+)
- [ ] Use `any` instead of `interface{}`
- [ ] Use `errors.Is()` and `errors.As()` for error checking
- [ ] Error wrapping uses `%w` not `%v`
- [ ] `context.Context` is first parameter (when needed)
- [ ] Struct initialization uses field names
- [ ] Consider generics for type-safe collections

### Error Handling
- [ ] All errors are checked (no `_` without comment)
- [ ] Errors are wrapped with context: `fmt.Errorf("context: %w", err)`
- [ ] Sentinel errors are declared as `var`, not `const`
- [ ] Custom errors implement `error` interface correctly
- [ ] No panics in library code (only in main/fatal conditions)

### Resource Management
- [ ] All `os.Open` calls have `defer Close()`
- [ ] All `http.Client` requests close response bodies
- [ ] Database connections use proper pooling
- [ ] Context deadlines set for I/O operations
- [ ] Cleanup happens in error paths too
- [ ] No file descriptor leaks

### Concurrency
- [ ] All goroutines have clear termination conditions
- [ ] Channels closed on sender side only
- [ ] `sync.Mutex`/`RWMutex` used correctly (not copied)
- [ ] `WaitGroup` calls balanced (Add/Done)
- [ ] Context cancellation is propagated
- [ ] No obvious data races (run `go test -race`)

### Documentation
- [ ] All exported functions have godoc comments
- [ ] Godoc comments start with function name
- [ ] Package-level godoc exists
- [ ] Complex logic has explanatory comments
- [ ] Examples in `_test.go` files if appropriate

### Testing
- [ ] New features have tests
- [ ] Tests use table-driven pattern with `t.Run()`
- [ ] Tests use `t.Cleanup()` or `defer` for cleanup
- [ ] Tests cover error cases
- [ ] No platform-specific build tags in tests

### Security
- [ ] No hardcoded credentials or secrets
- [ ] Crypto uses `crypto/rand` not `math/rand`
- [ ] No SQL injection vulnerabilities
- [ ] Input validation present
- [ ] File operations check for path traversal
- [ ] TLS uses secure configurations

### Performance
- [ ] No unnecessary allocations in hot paths
- [ ] String concatenation in loops uses `strings.Builder`
- [ ] Slices/maps pre-allocated when size known
- [ ] No defer in tight loops (performance-critical code)
- [ ] Large structs passed by pointer
- [ ] Regex compiled once (package-level `MustCompile`)

---

## 🔨 Pre-Commit Commands

Run these commands before committing:

```bash
# 1. Format code
go fmt ./...
# OR with gofmt for more control:
gofmt -w .

# 2. Run linter (if available)
go vet ./...

# 3. Run tests
go test ./...

# 4. Run race detector
go test -race ./...

# 5. Check test coverage
go test -cover ./...

# 6. Cross-platform build check
GOOS=linux go build ./...
GOOS=darwin go build ./...
GOOS=windows go build ./...

# 7. Check for vulnerabilities (if govulncheck installed)
govulncheck ./...
```

---

## 🚫 Common Mistakes to Avoid

### ❌ Wrong: Hardcoded paths
```go
file, _ := os.Open("/tmp/data.txt")
```
### ✅ Right: Cross-platform paths
```go
file, err := os.Open(filepath.Join(os.TempDir(), "data.txt"))
if err != nil {
    return fmt.Errorf("failed to open data file: %w", err)
}
defer file.Close()
```

---

### ❌ Wrong: interface{}
```go
func Process(data map[string]interface{}) error
```
### ✅ Right: any
```go
func Process(data map[string]any) error
```

---

### ❌ Wrong: Ignored errors
```go
_, _ = someFunction()
```
### ✅ Right: Logged errors
```go
if err := someFunction(); err != nil {
    logrus.WithError(err).Warn("Operation failed")
}
```

---

### ❌ Wrong: Error wrapping with %v
```go
return fmt.Errorf("failed: %v", err)
```
### ✅ Right: Error wrapping with %w
```go
return fmt.Errorf("failed: %w", err)
```

---

### ❌ Wrong: No defer close
```go
file, err := os.Open(path)
// ... use file ...
file.Close() // Might not execute on early return!
```
### ✅ Right: Defer close
```go
file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close()
// ... use file ...
```

---

### ❌ Wrong: Direct error comparison
```go
if err == ErrNotFound {
    // Doesn't work with wrapped errors
}
```
### ✅ Right: errors.Is()
```go
if errors.Is(err, ErrNotFound) {
    // Works with wrapped errors
}
```

---

### ❌ Wrong: Context not first parameter
```go
func Fetch(userID string, ctx context.Context) error
```
### ✅ Right: Context first
```go
func Fetch(ctx context.Context, userID string) error
```

---

## 🎯 File-Specific Guidelines

### Adding a new `.go` file
- [ ] Package declaration matches directory
- [ ] Imports are organized (stdlib, external, internal)
- [ ] Package-level godoc comment exists
- [ ] All exports have godoc comments

### Modifying existing code
- [ ] Maintain consistent style with surrounding code
- [ ] Update tests if behavior changes
- [ ] Update godoc if signature changes
- [ ] Check if README needs updates

### Adding dependencies
- [ ] Check if dependency is necessary
- [ ] Verify no CGo dependencies (pure Go only)
- [ ] Run `go mod tidy`
- [ ] Commit updated `go.mod` and `go.sum`
- [ ] Check for known vulnerabilities with `govulncheck`

---

## 📊 Coverage Targets

| Category | Target | Current |
|----------|--------|---------|
| Unit test coverage | >80% | 94% |
| Test files ratio | >40% | 49% |
| Godoc coverage | 100% | 100% |
| Cross-platform builds | 100% | 85%* |

*After fixing /tmp paths

---

## 🔍 Static Analysis

If `staticcheck` is installed:
```bash
staticcheck ./...
```

Common issues staticcheck catches:
- Unused variables
- Deprecated functions
- Inefficient constructs
- Common mistakes

---

## 📝 Commit Message Guidelines

Good commit messages:
```
✅ Fix hardcoded /tmp paths in async tests

Replace hardcoded /tmp paths with os.TempDir() to ensure
cross-platform compatibility on Windows.

Fixes #123
```

Bad commit messages:
```
❌ fix stuff
❌ WIP
❌ changes
```

---

## 🐛 If Tests Fail

1. **Check error messages carefully**
   - Read the full error output
   - Look for file and line numbers

2. **Run specific test**
   ```bash
   go test -v -run TestSpecificTest ./package
   ```

3. **Run with race detector**
   ```bash
   go test -race -run TestSpecificTest ./package
   ```

4. **Check test logs**
   - Tests use logrus for structured logging
   - Enable debug logging if needed

5. **Cross-platform issues?**
   ```bash
   GOOS=windows go test ./...
   ```

---

## 📚 Resources

- Project audit: `GO_PROJECT_AUDIT_REPORT.md`
- Quick reference: `GO_BEST_PRACTICES_QUICK_REFERENCE.md`
- Architecture: `README.md`
- Security: `COMPREHENSIVE_SECURITY_AUDIT.md`

---

## ✨ Before Opening PR

- [ ] All tests pass locally
- [ ] Race detector passes
- [ ] Code is formatted (`go fmt`)
- [ ] No new linter warnings (`go vet`)
- [ ] Cross-platform builds succeed
- [ ] Documentation updated if needed
- [ ] CHANGELOG updated (if applicable)
- [ ] Commit messages are descriptive
- [ ] PR description explains changes

---

## 🚀 Quick Commands

```bash
# Format, test, and vet in one go
make check  # If Makefile exists

# Or manually:
go fmt ./... && go vet ./... && go test ./...

# Full CI pipeline locally
go fmt ./... && \
go vet ./... && \
go test -race ./... && \
GOOS=linux go build ./... && \
GOOS=windows go build ./... && \
GOOS=darwin go build ./...
```

---

**Keep this checklist open while coding!**

Print it out or keep it in a browser tab. Following this checklist will prevent 95% of code review issues.

---

**Version**: 1.0  
**Last Updated**: October 2025  
**Based on**: Comprehensive audit findings
