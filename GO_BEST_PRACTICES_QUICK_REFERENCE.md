# Go Best Practices Quick Reference for toxcore-go

This guide provides quick reference for avoiding common issues found in the audit.

---

## ✅ DO: Cross-Platform Paths

```go
// ✅ CORRECT - Cross-platform
import (
    "os"
    "path/filepath"
)

tempDir := os.TempDir()
dataPath := filepath.Join(tempDir, "tox_data")
configPath := filepath.Join(dataPath, "config.json")
```

```go
// ❌ WRONG - Unix-only
dataPath := "/tmp/tox_data"
configPath := "/tmp/tox_data/config.json"
```

**Why**: `os.TempDir()` returns the correct temp directory for each platform:
- Linux: `/tmp`
- macOS: `/var/folders/...`
- Windows: `C:\Users\...\AppData\Local\Temp`

---

## ✅ DO: Use `any` Instead of `interface{}`

```go
// ✅ CORRECT - Modern Go 1.18+
func processData(data map[string]any) error {
    // ...
}

type Response struct {
    Data any `json:"data"`
}
```

```go
// ❌ WRONG - Outdated syntax
func processData(data map[string]interface{}) error {
    // ...
}

type Response struct {
    Data interface{} `json:"data"`
}
```

**Why**: `any` is a built-in type alias for `interface{}` in Go 1.18+, more readable and idiomatic.

---

## ✅ DO: Wrap Errors with `%w`

```go
// ✅ CORRECT - Preserves error chain
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

```go
// ❌ WRONG - Breaks error chain
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %v", err)
}
```

**Why**: `%w` allows `errors.Is()` and `errors.Unwrap()` to work with the error chain.

---

## ✅ DO: Use `errors.Is()` and `errors.As()`

```go
// ✅ CORRECT - Works with wrapped errors
if err != nil {
    if errors.Is(err, ErrNotFound) {
        // Handle not found
    }
}

// For type checking
var pathErr *os.PathError
if errors.As(err, &pathErr) {
    fmt.Printf("Path error on: %s\n", pathErr.Path)
}
```

```go
// ❌ WRONG - Doesn't work with wrapped errors
if err == ErrNotFound {
    // Won't match if err is wrapped
}

// Type assertion (fragile)
if pathErr, ok := err.(*os.PathError); ok {
    // Won't work if err is wrapped
}
```

**Why**: Modern error handling works with wrapped errors and is more robust.

---

## ✅ DO: Always Close Resources

```go
// ✅ CORRECT - Resource properly closed
file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close()

// Use file...
```

```go
// ❌ WRONG - Resource leak
file, err := os.Open(path)
if err != nil {
    return err
}
// Missing defer file.Close()!

// Use file...
```

**Why**: File descriptors are limited resources. Always use `defer` to ensure cleanup.

---

## ✅ DO: Handle Errors in Goroutines

```go
// ✅ CORRECT - Errors are logged
go func() {
    if err := backgroundTask(); err != nil {
        logrus.WithError(err).Warn("Background task failed")
    }
}()
```

```go
// ❌ WRONG - Errors silently ignored
go func() {
    _, _ = backgroundTask() // Silent failure!
}()
```

**Why**: Silent failures make debugging impossible. Always log errors.

---

## ✅ DO: Use Context as First Parameter

```go
// ✅ CORRECT - Context first
func FetchData(ctx context.Context, userID string, limit int) ([]Data, error) {
    // Check context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Do work...
}
```

```go
// ❌ WRONG - Context in wrong position
func FetchData(userID string, ctx context.Context, limit int) ([]Data, error) {
    // Context should be first parameter
}
```

**Why**: Go convention is context always first (except for receiver methods).

---

## ✅ DO: Check Build on Multiple Platforms

```bash
# ✅ CORRECT - Test all target platforms
GOOS=linux go build ./...
GOOS=darwin go build ./...
GOOS=windows go build ./...
```

**Why**: Catch platform-specific issues early in development.

---

## ✅ DO: Run Tests with Race Detector

```bash
# ✅ CORRECT - Catch concurrency bugs
go test -race ./...
```

**Why**: Race detector finds data races that might cause production bugs.

---

## ✅ DO: Use Table-Driven Tests

```go
// ✅ CORRECT - Table-driven with subtests
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid input", "test@example.com", false},
        {"empty input", "", true},
        {"invalid format", "not-an-email", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

**Why**: Makes tests more maintainable and provides better error messages.

---

## ✅ DO: Document Exported Symbols

```go
// ✅ CORRECT - Proper godoc
// GenerateKeyPair creates a new Ed25519 key pair for use in Tox.
// Returns the key pair and an error if key generation fails.
func GenerateKeyPair() (*KeyPair, error) {
    // ...
}
```

```go
// ❌ WRONG - Missing or bad godoc
// generates key pair
func GenerateKeyPair() (*KeyPair, error) {
    // Comment doesn't start with function name
}
```

**Why**: Godoc comments make your API discoverable and understandable.

---

## ✅ DO: Use String Builder for Concatenation

```go
// ✅ CORRECT - Efficient
var builder strings.Builder
for _, item := range items {
    builder.WriteString(item)
    builder.WriteString("\n")
}
result := builder.String()
```

```go
// ❌ WRONG - Inefficient (creates many temporary strings)
var result string
for _, item := range items {
    result += item + "\n"
}
```

**Why**: `strings.Builder` is much more efficient for repeated concatenation.

---

## ✅ DO: Initialize Structs with Field Names

```go
// ✅ CORRECT - Clear and maintainable
config := Config{
    Host:    "localhost",
    Port:    8080,
    Timeout: 30 * time.Second,
}
```

```go
// ❌ WRONG - Fragile, breaks with field reordering
config := Config{
    "localhost",
    8080,
    30 * time.Second,
}
```

**Why**: Named fields are clearer and don't break when struct fields are reordered.

---

## ✅ DO: Pre-allocate Slices When Size Known

```go
// ✅ CORRECT - Pre-allocated
results := make([]Result, 0, len(inputs))
for _, input := range inputs {
    results = append(results, process(input))
}
```

```go
// ❌ WRONG - Multiple reallocations
var results []Result
for _, input := range inputs {
    results = append(results, process(input))
}
```

**Why**: Pre-allocation avoids repeated memory allocations and copying.

---

## ✅ DO: Use Defer for Cleanup

```go
// ✅ CORRECT - Cleanup guaranteed
func processFile(path string) error {
    mu.Lock()
    defer mu.Unlock()
    
    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()
    
    // Even if panic occurs, cleanup happens
}
```

**Why**: Defer ensures cleanup happens even on panic or early return.

---

## Testing Checklist

Before pushing code:

```bash
# 1. Format code
gofmt -w .

# 2. Run tests
go test ./...

# 3. Race detector
go test -race ./...

# 4. Vet (static analysis)
go vet ./...

# 5. Build for target platforms
GOOS=linux go build ./...
GOOS=windows go build ./...
GOOS=darwin go build ./...

# 6. Check coverage
go test -cover ./...
```

---

## Quick Fix Scripts

### Fix interface{} → any
```bash
find . -name "*.go" -exec sed -i 's/interface{}/any/g' {} +
```

### Find hardcoded /tmp
```bash
grep -rn '"/tmp' --include="*.go" .
```

### Find missing error checks
```bash
grep -rn '_, _' --include="*.go" .
```

---

## Resources

- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [Go Project Layout](https://github.com/golang-standards/project-layout)

---

**Last Updated**: October 2025  
**For**: toxcore-go project  
**Based On**: Comprehensive audit findings
