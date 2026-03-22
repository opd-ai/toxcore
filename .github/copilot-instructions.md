# Project Overview

toxcore-go is a pure Go implementation of the Tox peer-to-peer encrypted messaging protocol, designed for secure communications without centralized infrastructure. The project provides a complete implementation including DHT-based peer discovery, friend management, 1-to-1 and group messaging, file transfers, audio/video calling (ToxAV), asynchronous offline messaging with forward secrecy, and identity obfuscation. It supports multi-network transports including IPv4/IPv6, Tor (.onion), I2P (.b32.i2p), Nym (.nym), and Lokinet (.loki).

The target audience includes developers building privacy-focused communication applications, researchers working on decentralized protocols, and contributors to the Tox ecosystem. The core Go libraries have no cgo dependencies and are a pure Go solution suitable for cross-platform deployment on Linux, macOS, and Windows (amd64/arm64). Optional C API bindings are provided via the `capi` package and require cgo for cross-language interoperability.

Key differentiators include Noise Protocol Framework (IK pattern) integration for enhanced handshake security, epoch-based forward secrecy with automatic key rotation, cryptographic identity obfuscation to protect metadata from storage nodes, and automatic message padding (256B, 1024B, 4096B) to resist traffic analysis.

## Technical Stack
- **Primary Language**: Go 1.24.0 (toolchain go1.24.12), module path `github.com/opd-ai/toxcore`
- **Frameworks**:
  - `github.com/flynn/noise v1.1.0` — Noise Protocol Framework for secure handshakes (Noise-IK pattern)
  - `github.com/go-i2p/onramp v0.33.92` — I2P network transport via SAM bridge protocol
  - `github.com/pion/opus v0.0.0-20250902022847-c2c56b95f05c` — Opus audio codec for ToxAV
  - `github.com/pion/rtp v1.8.22` — RTP packet handling for audio/video streams
  - `github.com/sirupsen/logrus v1.9.4` — Structured logging with levels and fields
  - `golang.org/x/crypto v0.48.0` — Cryptographic primitives (ChaCha20-Poly1305, Curve25519, Ed25519)
  - `golang.org/x/net v0.50.0` — Network utilities
  - `golang.org/x/sys v0.41.0` — System-level operations
- **Testing**: Go's built-in `testing` package with `github.com/stretchr/testify v1.11.1` for assertions. Race detection enabled (`-race`). Coverage reported via Codecov. Build tag `nonet` excludes network-dependent tests in CI.
- **Build/Deploy**: `go build` with `gofmt` and `go vet` for code quality. CI/CD via GitHub Actions (`.github/workflows/toxcore.yml`). Cross-platform matrix builds for linux/darwin/windows on amd64 and arm64 (with `windows/arm64` explicitly excluded in the CI matrix). Semantic versioning with optional version string injection via `-ldflags` as configured in `.github/workflows/toxcore.yml` and the version variable defined in the toxcore module. No Makefile; use `go` commands directly.

## Code Assistance Guidelines

1. **Use Interface Types for Networking**: Never use concrete network types (`net.UDPConn`, `net.TCPConn`, `net.UDPAddr`, `net.TCPAddr`). Always use interface types (`net.PacketConn`, `net.Conn`, `net.Addr`, `net.Listener`). Never use type assertions or type switches to convert from interface to concrete types — use interface methods instead. This is critical for testability with mock transports (see `transport/network_transport.go`).

2. **Follow Error Wrapping Conventions**: Use `fmt.Errorf("context: %w", err)` for all error propagation. Every public function should return descriptive, wrapped errors that provide call-site context. Never silently discard errors.

3. **Implement Secure Memory Handling**: For any code touching cryptographic keys or secrets, follow the patterns in `crypto/secure_memory.go`. Wipe sensitive data from memory using `defer` statements. Use `crypto/rand` for all random number generation in security-sensitive contexts. Use constant-time comparison for cryptographic operations.

4. **Write Table-Driven Tests with Testify**: Follow the existing test patterns using `testify/assert` and `testify/require`. Name test files with suffixes: `_unit_test.go` for unit tests, `_integration_test.go` for integration tests, `_benchmark_test.go` for benchmarks. Prefer channel-based synchronization with `select`/timeout for concurrent test coordination and avoid relying on `time.Sleep` where possible (see `async/prekey_test.go` for `select`/timeout examples).

5. **Use Build Constraints for Platform-Specific Code**: Use both `//go:build` and legacy `// +build` lines. For platform-specific filesystem operations, follow the pattern in `async/storage_limits_statfs.go` (linux/darwin/BSD), `async/storage_limits_nostatfs.go` (WASM/other), and `async/storage_limits_windows.go`. The project supports `GOOS=js GOARCH=wasm` builds.

6. **Follow Callback Registration Patterns**: Use the established callback pattern for event handling: `OnFriendRequest`, `OnFriendMessage`, `OnFriendConnectionStatus`, etc. All callbacks must be goroutine-safe. Register callbacks before starting the iteration loop.

7. **Complete Feature Implementations**: Always prefer completing the full implementation of any feature rather than leaving partial or placeholder code. When a complete implementation is not feasible, insert clear inline `TODO` comments describing what remains, why it was deferred, and any known constraints (e.g., `// TODO: Implement retry logic once the error categorization schema is finalized`). Never leave code in a silently incomplete state.

## Project Context

- **Domain**: Decentralized, end-to-end encrypted peer-to-peer messaging using the Tox protocol. Key domain concepts include DHT routing with k-buckets, NaCl-based encryption (Curve25519 + ChaCha20-Poly1305), Noise-IK handshakes for mutual authentication, forward secrecy via epoch-based pre-key rotation, and identity obfuscation using cryptographic pseudonyms. The system operates without central servers — peers discover each other through a distributed hash table and bootstrap nodes.

- **Architecture**: The main `toxcore` package (`toxcore.go`, `doc.go`) is the API facade that integrates all subsystems. Subsystems are organized as independent packages: `dht/` (peer discovery and routing), `transport/` (UDP/TCP/Noise/privacy network transports), `crypto/` (cryptographic operations), `friend/` (relationship management), `messaging/` (message types and processing), `async/` (offline messaging with forward secrecy), `file/` (file transfers), `group/` (group chat), `av/` (audio/video calling with `audio/`, `rtp/`, `video/` subpackages), `noise/` (Noise Protocol handshakes), `net/` (network utilities, STUN, port mapping), and `capi/` (C bindings).

- **Key Directories**:
  - `transport/` — 63 source files covering UDP, TCP, Noise, Tor, I2P, Nym, Lokinet transports and NAT traversal
  - `async/` — 54 source files for asynchronous messaging, forward secrecy, storage nodes, and identity obfuscation
  - `crypto/` — 27 source files for encryption, signatures, key management, and secure memory
  - `dht/` — 23 source files for DHT routing, bootstrap, k-bucket, and node management
  - `examples/` — 28 example programs demonstrating all major features
  - `docs/` — 15 technical specification documents (ASYNC.md, OBFS.md, MULTINETWORK.md, transport docs)

- **Configuration**: Use `toxcore.NewOptions()` for struct-based configuration with sensible defaults. Key options: `UDPEnabled` (default: true), `TCPPort`, `ProxyType`/`ProxyHost`/`ProxyPort` for SOCKS5/HTTP proxies, `Savedata`/`SavedataType` for persistence. Tests requiring external networks (Tor, I2P, Nym, Lokinet) use `//go:build !nonet` and CI passes `-tags nonet` to exclude them.

- **Node Identity**: `Node.Distance()` uses `Node.PublicKey` (top-level field), not `Node.ID.PublicKey`. Both fields must be set when creating temporary nodes for distance calculations. Use `PublicKey` byte comparison (`==`) instead of `ID.String()` for ToxID equality checks to avoid GC pressure from hex-encoded string allocation.

## Quality Standards

- **Testing Requirements**: Maintain high test coverage (currently 52.8% test-to-source file ratio with 206 test files across 390 Go files). Run tests with race detection: `go test -tags nonet -race -coverprofile=coverage.txt -covermode=atomic ./...`. Use `go test -race -count=2 ./...` locally to detect intermittent failures from sync issues.
- **Code Formatting**: All code must pass `gofmt` (enforced in CI). Run `gofmt -l .` to check before committing.
- **Static Analysis**: All code must pass `go vet ./...` (enforced in CI).
- **Dependency Verification**: Run `go mod verify` to validate dependency integrity.
- **Documentation Standards**: All public APIs must have GoDoc comments starting with the function/type name. Follow the comprehensive documentation pattern in `doc.go` (198 lines of package-level documentation with examples).
- **Security Reviews**: All changes to cryptographic code (`crypto/`, `noise/`, `async/forward_secrecy.go`, `async/obfs.go`) require security-focused review. Follow patterns documented in `docs/INDEX.md` (Security Audit section).
- **Benchmarks**: Include benchmark tests (`*_benchmark_test.go`) for performance-critical paths. Use `go test -bench=. -benchmem` to measure allocations.

## Networking Best Practices

When declaring network variables, always use interface types:
- Never use `net.UDPAddr`, `net.IPAddr`, or `net.TCPAddr`. Use `net.Addr` only instead.
- Never use `net.UDPConn`, use `net.PacketConn` instead.
- Never use `net.TCPConn`, use `net.Conn` instead.
- Never use `net.UDPListener` or `net.TCPListener`, use `net.Listener` instead.
- Never use a type switch or type assertion to convert from an interface type to a concrete type. Use the interface methods instead.

This approach enhances testability and flexibility when working with different network implementations or mocks. See `transport/network_transport.go` for the `NetworkTransport` interface pattern and use `SupportedNetworks()` to distinguish transport types.