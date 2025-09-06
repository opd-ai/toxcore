# NetworkAddress System Documentation

## Overview

The NetworkAddress system provides a foundation for multi-network support in the Tox protocol, enabling support for IPv4, IPv6, Tor .onion, I2P .b32.i2p, Nym .nym, and Lokinet .loki addresses while maintaining backward compatibility with existing code.

## Design Principles

### 1. Network Type Abstraction
Instead of assuming IP-based addressing, the system uses an enumerated `AddressType` to identify different network types:

```go
type AddressType uint8

const (
    AddressTypeIPv4     AddressType = 0x01  // IPv4 addresses
    AddressTypeIPv6     AddressType = 0x02  // IPv6 addresses  
    AddressTypeOnion    AddressType = 0x03  // Tor .onion addresses
    AddressTypeI2P      AddressType = 0x04  // I2P .b32.i2p addresses
    AddressTypeNym      AddressType = 0x05  // Nym .nym addresses
    AddressTypeLoki     AddressType = 0x06  // Lokinet .loki addresses
    AddressTypeUnknown  AddressType = 0xFF  // Unknown/unsupported types
)
```

### 2. Variable-Length Address Data
The `NetworkAddress` struct stores address data as a byte slice, allowing different network types to use appropriate address formats:

```go
type NetworkAddress struct {
    Type     AddressType  // Network type identifier
    Data     []byte       // Variable-length address data
    Port     uint16       // Port number (0 if not applicable)
    Network  string       // Network protocol ("tcp", "udp", "tor", etc.)
}
```

### 3. Backward Compatibility
The system provides seamless conversion between the new `NetworkAddress` and existing `net.Addr` interfaces:

```go
// Convert from net.Addr to NetworkAddress
netAddr, err := ConvertNetAddrToNetworkAddress(addr)

// Convert back to net.Addr
addr := netAddr.ToNetAddr()
```

## Usage Examples

### Basic Address Conversion

```go
// Working with existing net.Addr
udpAddr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 8080}

// Convert to NetworkAddress
netAddr, err := transport.ConvertNetAddrToNetworkAddress(udpAddr)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Type: %s\n", netAddr.Type.String())           // Type: IPv4
fmt.Printf("Address: %s\n", netAddr.String())             // Address: IPv4://192.168.1.1:8080
fmt.Printf("Private: %t\n", netAddr.IsPrivate())          // Private: true
fmt.Printf("Routable: %t\n", netAddr.IsRoutable())        // Routable: false
```

### Creating Multi-Network Addresses

```go
// Tor .onion address
onionAddr := &transport.NetworkAddress{
    Type:    transport.AddressTypeOnion,
    Data:    []byte("exampleexampleexample.onion"),
    Port:    8080,
    Network: "tcp",
}

// I2P .b32.i2p address
i2pAddr := &transport.NetworkAddress{
    Type:    transport.AddressTypeI2P,
    Data:    []byte("example12345678901234567890123456.b32.i2p"),
    Port:    8080,
    Network: "tcp",
}

// Both can be used with existing net.Addr interfaces
listener, err := net.Listen(onionAddr.ToNetAddr().Network(), onionAddr.ToNetAddr().String())
```

### Network-Specific Privacy Detection

```go
addresses := []*transport.NetworkAddress{
    {Type: transport.AddressTypeIPv4, Data: []byte{192, 168, 1, 1}},  // Private
    {Type: transport.AddressTypeIPv4, Data: []byte{8, 8, 8, 8}},      // Public
    {Type: transport.AddressTypeOnion, Data: []byte("example.onion")}, // Private (by design)
    {Type: transport.AddressTypeI2P, Data: []byte("example.b32.i2p")}, // Private (by design)
}

for _, addr := range addresses {
    fmt.Printf("%s: Private=%t, Routable=%t\n", 
        addr.Type.String(), addr.IsPrivate(), addr.IsRoutable())
}
```

## Integration Guidelines

### For Existing Code
The system is designed to integrate with existing code with minimal changes:

1. **No Changes Required**: Existing code using `net.Addr` continues to work
2. **Gradual Migration**: Use `ConvertNetAddrToNetworkAddress()` when network-type awareness is needed
3. **Future-Proof**: New network types can be added without breaking existing functionality

### For New Code
When writing new code that needs network-type awareness:

1. **Use NetworkAddress**: Accept `*NetworkAddress` parameters instead of `net.Addr`
2. **Provide Conversion**: Offer both `NetworkAddress` and `net.Addr` interfaces for compatibility
3. **Check Capabilities**: Use `IsPrivate()` and `IsRoutable()` instead of string parsing

## Performance Characteristics

Benchmarks show excellent performance for the core operations:

- **ConvertNetAddrToNetworkAddress**: ~141 ns/op
- **NetworkAddress.ToNetAddr**: ~31 ns/op
- **Memory Efficient**: Uses byte slices for minimal memory overhead
- **Zero Allocations**: For most common operations

## Error Handling

The system provides comprehensive error handling:

```go
// Conversion errors
netAddr, err := transport.ConvertNetAddrToNetworkAddress(invalidAddr)
if err != nil {
    // Handle conversion failure
}

// Graceful degradation for unknown types
unknownAddr := &transport.NetworkAddress{
    Type: transport.AddressTypeUnknown,
    Data: []byte("unknown://example"),
}
// IsPrivate() returns true for safety
// IsRoutable() returns false for safety
```

## Wire Protocol Considerations

The new address system is designed to support future wire protocol extensions:

1. **Version Negotiation**: Address types can be negotiated during handshake
2. **Extensible Format**: New address types can be added with unique type codes
3. **Backward Compatibility**: Legacy IPv4/IPv6 addresses use existing wire format

## Security Considerations

1. **Privacy by Default**: Unknown address types are assumed private for safety
2. **Address Validation**: All address parsing includes bounds checking
3. **Network Isolation**: Different network types maintain proper isolation
4. **No Information Leakage**: Address parsing failures degrade gracefully

## Future Extensions

The system is designed to easily support additional network types:

1. **Add New AddressType**: Define new constant and string representation
2. **Implement Parser**: Add parsing logic in `ConvertNetAddrToNetworkAddress()`
3. **Update Methods**: Extend `IsPrivate()` and `IsRoutable()` for new type
4. **Maintain Compatibility**: Existing code continues to work unchanged
