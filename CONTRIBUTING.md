# Contributing to toxcore-go

Thank you for your interest in contributing to toxcore-go! This guide will help you get oriented with the project structure, coding conventions, and contribution workflows.

## Project Description

toxcore-go is a pure Go implementation of the Tox Messenger core protocol, designed for secure peer-to-peer communication without centralized infrastructure. The project aims to provide a clean, idiomatic Go alternative to the original C libtoxcore implementation while maintaining protocol compatibility and extending functionality with Go-native features.

Current development goals include:
- Completing DHT routing table functionality for peer discovery
- Enhancing file transfer capabilities with resumable transfers
- Improving TCP relay support for NAT traversal scenarios

The target audience consists of developers building secure messaging applications who need a CGo-free, cross-platform implementation with comprehensive Tox protocol coverage.

## Coding Style

We follow standard Go conventions with some project-specific preferences:

### General Guidelines
- **Interface Usage**: Prefer network interface types (`net.Addr`, `net.PacketConn`, `net.Conn`) over concrete types for enhanced testability
- **Concurrency Safety**: Always protect shared state with appropriate mutex types (`sync.RWMutex` for read-heavy operations, `sync.Mutex` for writes)
- **Error Handling**: Return descriptive errors from all fallible operations; avoid panics except for programming errors
- **Export Annotations**: Add `//export FunctionName` comments above public functions intended for C binding compatibility

### Code Examples
```go
// Preferred: Interface type for flexibility
func handlePacket(conn net.PacketConn, addr net.Addr) error {
    // Implementation
}

// Preferred: Proper mutex protection pattern
func (t *Tox) UpdateFriend(id uint32) {
    t.friendsMutex.Lock()
    defer t.friendsMutex.Unlock()
    // Modify friend data
}
```

### Documentation
- Document all public APIs with godoc-compatible comments
- Include parameter descriptions and return values
- Provide usage examples in package documentation

## Project Structure

```
/workspaces/toxcore/
├── toxcore.go              # Main API and Tox struct implementation
├── crypto/                 # Cryptographic operations (Ed25519, Curve25519, ToxID management)
├── dht/                    # Distributed Hash Table for peer discovery and routing
├── transport/              # Network transport layers (UDP, TCP, packet handling)
├── friend/                 # Friend request management and relationship handling
├── file/                   # File transfer implementation with chunking and progress tracking
├── examples/               # Usage examples and demo applications
├── tests/                  # Integration and unit tests
├── docs/                   # Additional documentation and protocol specifications
├── README.md               # Project overview and basic usage
├── go.mod                  # Go module definition
└── CONTRIBUTING.md         # This file
```

**Core Functionality**: `toxcore.go`, `crypto/`, `dht/`, `transport/` contain essential protocol implementation
**Supporting Features**: `friend/`, `file/` provide higher-level messaging and transfer capabilities
**Development Support**: `examples/`, `tests/`, `docs/` assist with development and testing

## Examples

### Adding a New Packet Handler
When extending protocol support, you'll commonly add new packet handlers:

1. **Define packet type** in `/transport/packet.go` (add new `PacketType` constant)
2. **Implement handler method** in `/toxcore.go` following the pattern:
   ```go
   func (t *Tox) handleNewPacketType(packet *transport.Packet, addr net.Addr) error {
       // Validate packet data
       // Process packet content
       // Trigger appropriate callbacks
       return nil
   }
   ```
3. **Register handler** in `registerUDPHandlers()` method:
   ```go
   t.udpTransport.RegisterHandler(transport.PacketNewType, t.handleNewPacketType)
   ```

### Extending the Friend API
To add new friend-related functionality:

1. **Add method to Tox struct** in `/toxcore.go` with proper export annotation
2. **Implement thread-safe access** using `t.friendsMutex` protection pattern
3. **Add corresponding callback type** if the feature requires user notification
4. **Update save/load logic** in `GetSavedata()` and `loadFromSaveData()` if state persistence is needed

### Adding File Transfer Features
File transfer enhancements typically involve:

1. **Extend Transfer struct** in `/file/transfer.go` with new fields or methods
2. **Update packet handlers** in `/toxcore.go` (`handleFileOfferPacket`, `handleFileChunkPacket`)
3. **Modify file control logic** in `FileControl()` method to support new operations
4. **Add progress tracking** through existing callback mechanisms
