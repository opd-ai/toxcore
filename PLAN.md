# Architectural Redesign Plan for Multi-Network Support

## Executive Summary

This document outlines the architectural changes required to eliminate RED FLAG areas in `dht/handler.go` and `transport/nat.go` that currently prevent support for alternative network types (.onion, .b32.i2p, .nym, .loki). The current implementation assumes IP-based addressing and performs address parsing that breaks abstraction.

## Current Problems

### Critical Issues Identified

1. **Wire Protocol Assumptions** (`dht/handler.go`)
   - Fixed 16-byte IP + 2-byte port binary format
   - IPv4/IPv6 format detection and parsing
   - Address serialization tied to IP semantics

2. **NAT Detection Logic** (`transport/nat.go`)
   - IP address parsing for private range detection
   - Interface enumeration with IP-specific logic
   - Public address detection through IP parsing

3. **Protocol Rigidity**
   - No extensibility for new address types
   - Hardcoded assumptions about address structure
   - No negotiation mechanism for supported networks

## Redesign Architecture

### Phase 1: Core Protocol Extensions

#### 1.1 Address Type System

**New Address Type Enumeration:**
```go
type AddressType uint8

const (
    AddressTypeIPv4     AddressType = 0x01
    AddressTypeIPv6     AddressType = 0x02
    AddressTypeOnion    AddressType = 0x03  // Tor .onion
    AddressTypeI2P      AddressType = 0x04  // I2P .b32.i2p
    AddressTypeNym      AddressType = 0x05  // Nym .nym
    AddressTypeLoki     AddressType = 0x06  // Lokinet .loki
    AddressTypeUnknown  AddressType = 0xFF
)
```

**Network Address Abstraction:**
```go
type NetworkAddress struct {
    Type     AddressType
    Data     []byte          // Variable-length address data
    Port     uint16          // Optional port (0 if not applicable)
    Network  string          // Network identifier ("tcp", "udp", "tor", etc.)
}

func (na *NetworkAddress) ToNetAddr() net.Addr
func (na *NetworkAddress) String() string
func (na *NetworkAddress) IsPrivate() bool
func (na *NetworkAddress) IsRoutable() bool
```

#### 1.2 Wire Protocol Extensions

**Variable-Length Address Encoding:**
```
Node Entry Format (New):
+----------+----------+----------+----------+
| PubKey   | AddrType | AddrLen  | Address  |
| (32B)    | (1B)     | (1B)     | (var)    |
+----------+----------+----------+----------+
| Port     | Padding  |
| (2B)     | (var)    |
+----------+----------+

Total: 36 + address_length + padding_to_align
```

**Backward Compatibility:**
- AddressType 0x01/0x02 use legacy 16-byte format
- New types use variable-length encoding
- Protocol version negotiation determines supported formats

### Phase 2: DHT Handler Redesign

#### 2.1 Address Parsing Abstraction

**Current Problem Area:**
```go
// RED FLAG: dht/handler.go parseAddressFromPacket()
func (bm *BootstrapManager) parseAddressFromPacket(data []byte, nodeOffset int) net.Addr
```

**Proposed Solution:**
```go
// New wire protocol parser
type PacketParser interface {
    ParseNodeEntry(data []byte, offset int) (NodeEntry, int, error)
    SerializeNodeEntry(entry NodeEntry) ([]byte, error)
}

type NodeEntry struct {
    PublicKey [32]byte
    Address   NetworkAddress
    LastSeen  time.Time
}

// Protocol-specific parsers
type LegacyIPParser struct{}  // For backward compatibility
type ExtendedParser struct{}  // For new address types

func (bm *BootstrapManager) parseNodeEntry(data []byte, offset int) (NodeEntry, int, error) {
    parser := bm.selectParser(data, offset)
    return parser.ParseNodeEntry(data, offset)
}
```

#### 2.2 Address Serialization

**Current Problem Area:**
```go
// RED FLAG: dht/handler.go formatIPAddress()
func (bm *BootstrapManager) formatIPAddress(addr net.Addr) []byte
```

**Proposed Solution:**
```go
// Network-agnostic serialization
func (bm *BootstrapManager) serializeNodeEntry(entry NodeEntry) []byte {
    return bm.parser.SerializeNodeEntry(entry)
}

// Address conversion utilities
func ConvertNetAddrToNetworkAddress(addr net.Addr) (NetworkAddress, error) {
    // Detect address type and convert appropriately
    switch addr.Network() {
    case "tcp", "udp":
        return parseIPAddress(addr)
    case "tor":
        return parseTorAddress(addr)
    case "i2p":
        return parseI2PAddress(addr)
    default:
        return NetworkAddress{Type: AddressTypeUnknown}, nil
    }
}
```

### Phase 3: NAT Traversal Redesign

#### 3.1 Network Type Detection

**Current Problem Areas:**
```go
// RED FLAG: transport/nat.go detectNATType()
// RED FLAG: transport/nat.go isPrivateAddr()
```

**Proposed Solution:**
```go
type NetworkCapabilities struct {
    SupportsNAT     bool
    SupportsUPnP    bool
    IsPrivateSpace  bool
    RoutingMethod   RoutingMethod
}

type RoutingMethod int
const (
    RoutingDirect     RoutingMethod = iota  // Direct routing
    RoutingNAT                              // Behind NAT
    RoutingProxy                            // Through proxy (Tor, I2P)
    RoutingMixed                            // Multiple methods
)

// Network-specific capability detection
type NetworkDetector interface {
    DetectCapabilities(addr net.Addr) NetworkCapabilities
    SupportedNetworks() []string
}

type IPNetworkDetector struct{}      // IPv4/IPv6 detection
type TorNetworkDetector struct{}     // Tor .onion detection  
type I2PNetworkDetector struct{}     // I2P .b32.i2p detection
```

#### 3.2 Multi-Network Public Address Detection

**Current Problem Area:**
```go
// RED FLAG: transport/nat.go detectPublicAddress()
```

**Proposed Solution:**
```go
type PublicAddressResolver interface {
    ResolvePublicAddress(localAddr net.Addr) (net.Addr, error)
    SupportsNetwork(network string) bool
}

type MultiNetworkResolver struct {
    resolvers map[string]PublicAddressResolver
}

// Network-specific resolvers
type IPResolver struct{}       // STUN/UPnP for IP networks
type TorResolver struct{}      // Tor descriptor for .onion
type I2PResolver struct{}      // I2P netdb for .b32.i2p

func (mnr *MultiNetworkResolver) ResolvePublicAddress(addr net.Addr) (net.Addr, error) {
    resolver := mnr.selectResolver(addr.Network())
    return resolver.ResolvePublicAddress(addr)
}
```

### Phase 4: Transport Layer Integration

#### 4.1 Multi-Protocol Transport

**Current Limitation:**
- Transport assumes UDP/TCP over IP
- No support for Tor/I2P/Nym transports

**Proposed Architecture:**
```go
type NetworkTransport interface {
    Listen(address string) (net.Listener, error)
    Dial(address string) (net.Conn, error)
    DialPacket(address string) (net.PacketConn, error)
    SupportedNetworks() []string
}

// Network-specific transports
type IPTransport struct{}       // UDP/TCP over IPv4/IPv6
type TorTransport struct{}      // TCP over Tor
type I2PTransport struct{}      // Streaming over I2P
type NymTransport struct{}      // Packets over Nym mixnet

type MultiTransport struct {
    transports map[string]NetworkTransport
}

func (mt *MultiTransport) selectTransport(addr net.Addr) NetworkTransport {
    return mt.transports[addr.Network()]
}
```

#### 4.2 Address Resolution Service

**Bootstrap Process Redesign:**
```go
type AddressResolver interface {
    Resolve(address string) ([]NetworkAddress, error)
    RegisterNetwork(name string, resolver NetworkResolver)
}

type NetworkResolver interface {
    ParseAddress(address string) (NetworkAddress, error)
    ValidateAddress(addr NetworkAddress) error
}

// Example usage
resolver := NewMultiNetworkResolver()
resolver.RegisterNetwork("tor", &TorResolver{})
resolver.RegisterNetwork("i2p", &I2PResolver{})

addresses, err := resolver.Resolve("friend.onion:8888")
```

## Implementation Strategy

### Phase 1: Foundation (Week 1-2)
1. **Define new address type system**
   - Implement `AddressType` enumeration
   - Create `NetworkAddress` struct with interface methods
   - Add backward compatibility layer

2. **Wire protocol versioning**
   - Add protocol version field to handshake
   - Implement feature negotiation
   - Create parser interface

### Phase 2: DHT Refactoring (Week 3-4)
1. **Replace address parsing in `dht/handler.go`**
   - Implement `PacketParser` interface
   - Create `LegacyIPParser` for backward compatibility
   - Add `ExtendedParser` for new address types

2. **Update bootstrap manager**
   - Replace `parseAddressFromPacket()` with `parseNodeEntry()`
   - Replace `formatIPAddress()` with `serializeNodeEntry()`
   - Add address type detection logic

### Phase 3: NAT System Redesign (Week 5-6)
1. **Replace IP-specific logic in `transport/nat.go`**
   - Implement `NetworkDetector` interface
   - Create network-specific detectors
   - Replace `isPrivateAddr()` with capability-based detection

2. **Multi-network public address resolution**
   - Implement `PublicAddressResolver` interface
   - Create network-specific resolvers
   - Update `detectPublicAddress()` with multi-network support

### Phase 4: Transport Integration (Week 7-8)
1. **Multi-protocol transport layer**
   - Implement `NetworkTransport` interface
   - Create transport implementations for each network type
   - Integrate with existing UDP/TCP transport

2. **Address resolution service**
   - Implement `AddressResolver` interface
   - Create network-specific address parsers
   - Update bootstrap and connection logic

## Testing Strategy

### Unit Testing
- **Address type conversion**: Test all network address formats
- **Wire protocol compatibility**: Test parsing with legacy and new formats
- **Network detection**: Mock network interfaces for different scenarios
- **Transport selection**: Test routing to appropriate transports

### Integration Testing  
- **Multi-network bootstrap**: Test bootstrapping across different networks
- **Address resolution**: Test resolving addresses for each network type
- **NAT detection**: Test capability detection for different network types
- **Cross-network communication**: Test communication between different address types

### Compatibility Testing
- **Backward compatibility**: Ensure legacy clients can still connect
- **Protocol negotiation**: Test feature detection and fallback
- **Wire format validation**: Test parsing of malformed packets

## Migration Path

### Immediate Actions (Remove RED FLAGS)
1. **Mark problematic functions as deprecated**
   ```go
   // Deprecated: Use NetworkAddress-based methods instead
   func (bm *BootstrapManager) parseAddressFromPacket(...) net.Addr
   ```

2. **Add interface-based alternatives**
   ```go
   func (bm *BootstrapManager) parseNodeEntryFromPacket(...) (NodeEntry, error)
   ```

3. **Update function signatures**
   ```go
   // Before: func DetectNATType(localAddr net.Addr) (NATType, error)
   // After:  func DetectNetworkCapabilities(addr net.Addr) (NetworkCapabilities, error)
   ```

### Gradual Migration
1. **Phase 1**: Add new interfaces alongside existing code
2. **Phase 2**: Update callers to use new interfaces  
3. **Phase 3**: Remove deprecated functions
4. **Phase 4**: Add support for new network types

## Risk Mitigation

### Backward Compatibility
- **Wire protocol versioning** ensures old clients continue working
- **Dual parser support** handles both legacy and new packet formats
- **Gradual deprecation** provides migration time for dependent code

### Performance Impact
- **Lazy initialization** of network detectors and resolvers
- **Caching** of network capabilities and address resolutions
- **Batched operations** for address validation and conversion

### Security Considerations
- **Address validation** prevents injection of malicious addresses
- **Network isolation** ensures proper routing between network types
- **Privacy protection** maintains anonymity properties of overlay networks

## Success Criteria

1. **Zero RED FLAG markers** in production code
2. **No address parsing** using string manipulation or IP-specific logic
3. **Support for new network types** without code changes to core logic
4. **Backward compatibility** with existing Tox network
5. **Performance parity** or improvement over current implementation

## Next Steps

1. **Review and approve** architectural plan
2. **Implement Phase 1** foundation components
3. **Create migration timeline** with stakeholder input
4. **Begin development** with unit tests for new interfaces
5. **Plan integration testing** across multiple network types

---

**Document Version**: 1.0  
**Last Updated**: September 6, 2025  
**Review Date**: September 20, 2025
