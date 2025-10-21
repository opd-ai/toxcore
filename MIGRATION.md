# Migration Guide: toxcore-go → ToxForge

## Overview

This project has been renamed from **toxcore-go** to **ToxForge** to better reflect its evolution beyond a simple Tox implementation. ToxForge incorporates significant security enhancements (Noise-IK protocol, forward secrecy, identity obfuscation) and modern architectural improvements while maintaining full compatibility with the Tox network.

## For End Users

### What Changed
- **Project Name**: toxcore-go → ToxForge
- **Go Module Path**: `github.com/opd-ai/toxcore` → `github.com/opd-ai/toxforge`
- **Enhanced Features**: Noise Protocol Framework integration, async messaging with identity obfuscation, improved privacy protections

### What Didn't Change
- **Network Compatibility**: Full compatibility with existing Tox network and clients
- **Savedata Format**: Your existing savedata files work without modification
- **Tox ID**: Your Tox ID and friend list remain unchanged
- **Core API**: Standard Tox protocol operations remain identical

### Migration Steps

1. **Update your import path**:
   ```go
   // Old import
   import "github.com/opd-ai/toxcore"
   
   // New import
   import "github.com/opd-ai/toxforge"
   ```

2. **Update your go.mod**:
   ```bash
   # Remove old module
   go mod edit -droprequire github.com/opd-ai/toxcore
   
   # Add new module
   go get github.com/opd-ai/toxforge@latest
   
   # Clean up
   go mod tidy
   ```

3. **Update import statements in your code**:
   ```bash
   # Linux/macOS
   find . -name "*.go" -type f -exec sed -i 's|github.com/opd-ai/toxcore|github.com/opd-ai/toxforge|g' {} +
   
   # Or manually update all imports from toxcore to toxforge
   ```

4. **Rebuild your application**:
   ```bash
   go build ./...
   go test ./...
   ```

### Compatibility Guarantees

- ✅ **Network Protocol**: 100% compatible with Tox network
- ✅ **Savedata**: No conversion needed, existing savedata works
- ✅ **Friend Lists**: All existing friends remain connected
- ✅ **Core API**: No breaking changes to standard Tox operations
- ✅ **Optional Features**: New features (Noise-IK, async) are opt-in only

## For Developers

### Package Rename Summary

All subpackages have been updated:

```go
// Old imports → New imports
github.com/opd-ai/toxcore          → github.com/opd-ai/toxforge
github.com/opd-ai/toxcore/async    → github.com/opd-ai/toxforge/async
github.com/opd-ai/toxcore/av       → github.com/opd-ai/toxforge/av
github.com/opd-ai/toxcore/crypto   → github.com/opd-ai/toxforge/crypto
github.com/opd-ai/toxcore/dht      → github.com/opd-ai/toxforge/dht
github.com/opd-ai/toxcore/friend   → github.com/opd-ai/toxforge/friend
github.com/opd-ai/toxcore/group    → github.com/opd-ai/toxforge/group
github.com/opd-ai/toxcore/noise    → github.com/opd-ai/toxforge/noise
github.com/opd-ai/toxcore/transport → github.com/opd-ai/toxforge/transport
// ... all other subpackages follow the same pattern
```

### API Stability

**No Breaking Changes** — The core API remains 100% backward compatible:

```go
// These APIs work exactly as before
tox, err := toxforge.New(options)
tox.Bootstrap(address, port, publicKey)
tox.AddFriend(toxID, message)
tox.SendFriendMessage(friendID, message)
tox.OnFriendRequest(callback)
tox.OnFriendMessage(callback)
tox.Iterate()
```

### New Optional Features

ToxForge adds opt-in enhancements that don't affect existing code:

1. **Noise-IK Transport** (optional):
   ```go
   import "github.com/opd-ai/toxforge/transport"
   noiseTransport, err := transport.NewNoiseTransport(udpTransport, privateKey)
   ```

2. **Async Messaging** (optional):
   ```go
   import "github.com/opd-ai/toxforge/async"
   asyncManager, err := async.NewAsyncManager(keyPair, transport, dataDir)
   ```

3. **Identity Obfuscation** (automatic when using async):
   - Enabled by default in async messaging
   - No configuration needed

### Testing Your Migration

Run the full test suite to verify compatibility:

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package tests
go test -v github.com/opd-ai/toxforge/crypto
go test -v github.com/opd-ai/toxforge/transport
```

### Example Migration

**Before** (toxcore-go):
```go
package main

import (
    "github.com/opd-ai/toxcore"
    "github.com/opd-ai/toxcore/crypto"
)

func main() {
    tox, _ := toxcore.New(toxcore.NewOptions())
    defer tox.Kill()
    
    keyPair, _ := crypto.GenerateKeyPair()
    // ... rest of your code
}
```

**After** (ToxForge):
```go
package main

import (
    "github.com/opd-ai/toxforge"
    "github.com/opd-ai/toxforge/crypto"
)

func main() {
    tox, _ := toxforge.New(toxforge.NewOptions())
    defer tox.Kill()
    
    keyPair, _ := crypto.GenerateKeyPair()
    // ... rest of your code (unchanged)
}
```

## Acknowledgments

ToxForge builds upon:
- **Tox Protocol Specification**: The foundational peer-to-peer encrypted messaging protocol
- **Noise Protocol Framework**: By Trevor Perrin, providing forward secrecy and formal security guarantees

We acknowledge these prior works and their contributions to secure, decentralized communication.

## Support

- **Documentation**: See [README.md](README.md) for updated usage examples
- **Security Features**: See [docs/SECURITY_AUDIT_REPORT.md](docs/SECURITY_AUDIT_REPORT.md)
- **Async Messaging**: See [docs/ASYNC.md](docs/ASYNC.md)
- **Identity Obfuscation**: See [docs/OBFS.md](docs/OBFS.md)

## Questions?

If you encounter any issues during migration:
1. Check that all imports are updated
2. Run `go mod tidy` to clean dependencies
3. Verify tests pass with `go test ./...`
4. Review the compatibility guarantees above

The rename is purely organizational — your code should work with minimal changes (just import path updates).
