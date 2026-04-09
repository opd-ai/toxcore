# Multi-Network System Documentation

## Overview

The toxcore-go multi-network system enables secure peer-to-peer communication across IPv4, IPv6, Tor (.onion), I2P (.b32.i2p), Nym (.nym), and Lokinet (.loki) addresses. It eliminates IP-specific assumptions and provides a unified interface with full backward compatibility.

## Architecture Overview

The multi-network system consists of four core layers:

1. **Address Resolution Layer** - Unified address parsing and validation
2. **Network Detection Layer** - Capability-based network analysis
3. **Transport Selection Layer** - Automatic protocol selection and management
4. **NAT Traversal Layer** - Multi-network connectivity and public address resolution

```
┌─────────────────────────────────────────────────────────────────┐
│                     Application Layer                          │
└─────────────────────────┬───────────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│                  Address Resolution                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────┐ │
│  │ IPv4/IPv6   │ │ Tor .onion  │ │ I2P .b32.i2p│ │ Nym .nym │ │
│  │ Parser      │ │ Parser      │ │ Parser      │ │ Parser   │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └──────────┘ │
└─────────────────────────┬───────────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│                  Network Detection                             │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────┐ │
│  │ IP Network  │ │ Tor Network │ │ I2P Network │ │ Nym      │ │
│  │ Detector    │ │ Detector    │ │ Detector    │ │ Detector │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └──────────┘ │
└─────────────────────────┬───────────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│                  Transport Selection                           │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────┐ │
│  │ IP          │ │ Tor         │ │ I2P         │ │ Nym      │ │
│  │ Transport   │ │ Transport   │ │ Transport   │ │Transport │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └──────────┘ │
└─────────────────────────┬───────────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│               Network Protocol Layer                           │
│     UDP/TCP          Tor Proxy        I2P Router    Nym Client │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start Guide

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Create address parser for multi-network support
    parser := transport.NewMultiNetworkParser()
    defer parser.Close()
    
    // Parse addresses across different networks
    addresses := []string{
        "192.168.1.1:33445",                    // IPv4
        "[2001:db8::1]:33445",                  // IPv6
        "facebookcorewwwi.onion:443",           // Tor
        "7rmath4f27le5rmq...64a.b32.i2p:9150", // I2P
        "abc123.clients.nym:1789",              // Nym
    }
    
    for _, addr := range addresses {
        networkAddrs, err := parser.Parse(addr)
        if err != nil {
            log.Printf("Failed to parse %s: %v", addr, err)
            continue
        }
        
        for _, netAddr := range networkAddrs {
            fmt.Printf("Parsed: %s -> Type: %s, Network: %s\n", 
                addr, netAddr.Type, netAddr.Network)
        }
    }
}
```

### Transport Layer Usage

```go
// Create multi-transport for automatic protocol selection
mt := transport.NewMultiTransport()
defer mt.Close()

// Automatic transport selection based on address format
conn, err := mt.Dial("example.onion:80")  // Uses TorTransport
if err != nil {
    log.Printf("Failed to connect: %v", err)
}
defer conn.Close()

// Listen on multiple networks simultaneously
listener, err := mt.Listen("0.0.0.0:8080")  // Uses IPTransport
if err != nil {
    log.Printf("Failed to listen: %v", err)
}
defer listener.Close()
```

## Supported Networks

### 1. IP Networks (IPv4/IPv6)

**Status**: ✅ **Fully Implemented**  
**Transport**: UDP/TCP over IP  
**Use Cases**: Traditional internet connectivity, local networks

```go
// IPv4 examples
"192.168.1.1:33445"     // Private IPv4
"8.8.8.8:53"            // Public IPv4

// IPv6 examples
"[2001:db8::1]:33445"   // IPv6 with brackets
```

**Configuration**:
```go
// IP transport is automatically registered
ipTransport := transport.NewIPTransport()
mt.RegisterTransport("ip", ipTransport)
```

### 2. Tor Network (.onion)

**Status**: ✅ **Interface Ready** (Implementation placeholder)  
**Transport**: TCP over Tor SOCKS5 proxy  
**Use Cases**: Anonymous communication, censorship resistance

```go
// Tor onion examples
"facebookcorewwwi.onion:443"           // v2 format (16 chars, deprecated)
"facebookwkhpilnemxj7asaniu7vnjjbiltxjqhye3mhbshg7kx5tfyd.onion:443"  // v3 format (56 chars)
```

**Configuration**:
```go
// Tor transport with SOCKS5 proxy
torTransport := transport.NewTorTransport()
// Future: Configure SOCKS5 proxy settings
// torTransport.SetProxy("127.0.0.1:9050")
mt.RegisterTransport("tor", torTransport)
```

### 3. I2P Network (.b32.i2p)

**Status**: ✅ **Interface Ready** (Implementation placeholder)  
**Transport**: Streaming over I2P SAM interface  
**Use Cases**: Decentralized anonymous networking

```go
// I2P base32 example
"7rmath4f27le5rmqbk2fmrlmvbvbfomt4mcqh73c6ukfhnpqdx4a.b32.i2p:9150"
```

**Configuration**:
```go
// I2P transport with SAM interface
i2pTransport := transport.NewI2PTransport()
// Future: Configure SAM bridge settings
// i2pTransport.SetSAMBridge("127.0.0.1:7656")
mt.RegisterTransport("i2p", i2pTransport)
```

### 4. Nym Network (.nym)

**Status**: ✅ **Interface Ready** (Implementation placeholder)  
**Transport**: Packets over Nym mixnet  
**Use Cases**: Metadata-resistant communication

```go
// Nym gateway examples
"abc123.clients.nym:1789"         // Nym client gateway
"service.gateways.nym:8080"       // Nym service gateway
```

**Configuration**:
```go
// Nym transport with mixnet client
nymTransport := transport.NewNymTransport()
// Future: Configure Nym client settings
// nymTransport.SetGateway("gateway.nym")
mt.RegisterTransport("nym", nymTransport)
```

### 5. Lokinet (.loki)

**Status**: 🔄 **Planned** (Interface ready for implementation)  
**Transport**: Custom protocol over Lokinet  
**Use Cases**: Low-latency anonymous routing

```go
// Lokinet examples (planned)
"service.loki:80"                 // Lokinet service
"app.loki:443"                    // Secure Lokinet service
```

## Network Configuration

### Address Parser Configuration

```go
parser := transport.NewMultiNetworkParser()

// Check supported networks
networks := parser.GetSupportedNetworks()
fmt.Println("Supported:", networks)  // Output: [ip tor i2p nym]

// Register custom network parser
type CustomParser struct{}
func (p *CustomParser) ParseAddress(addr string) (transport.NetworkAddress, error) {
    // Custom parsing logic
}
func (p *CustomParser) ValidateAddress(addr transport.NetworkAddress) error {
    // Custom validation logic
}
func (p *CustomParser) CanParse(address string) bool {
    return strings.HasSuffix(address, ".custom")
}
func (p *CustomParser) GetNetworkType() string {
    return "custom"
}

parser.RegisterNetwork("custom", &CustomParser{})
```

### Network Detection Configuration

```go
detector := transport.NewMultiNetworkDetector()

// Analyze address capabilities
addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.1:8080")
capabilities := detector.DetectCapabilities(addr)

fmt.Printf("NAT Support: %v\n", capabilities.SupportsNAT)
fmt.Printf("UPnP Support: %v\n", capabilities.SupportsUPnP)
fmt.Printf("Private Space: %v\n", capabilities.IsPrivateSpace)
fmt.Printf("Routing: %v\n", capabilities.RoutingMethod)
```

### Transport Configuration

```go
mt := transport.NewMultiTransport()

// Privacy network configuration examples
// tor: Configure SOCKS5 proxy, circuit management
// i2p: Configure SAM bridge, tunnel settings
// nym: Configure mixnet gateway, anonymity parameters
```

### Tor + I2P Anonymous Mode

When both Tor and I2P are registered in a `MultiTransport`, the system enters **anonymous mode** with the following routing behaviour:

| Operation | Address Type | Routed Via | Notes |
|-----------|-------------|------------|-------|
| `Dial()` | `.onion` | Tor (TCP) | Friend connections and A/V calls to .onion peers |
| `Dial()` | `.i2p` | I2P Streaming | Friend connections and A/V calls to .i2p peers |
| `Dial()` | Clearnet | Tor (TCP) | Clearnet friend connections routed through Tor |
| `DialPacket()` | `.i2p` | I2P Datagrams | Tox DHT packets to I2P peers |
| `DialPacket()` | Any other | I2P Datagrams | All UDP/datagram traffic routes through I2P since Tor is TCP-only |

Because Tor is TCP-only, `MultiTransport.DialPacket()` always routes through I2P when both are registered, restricting Tox DHT to the I2P network. Intra-network messages between unreachable peers (e.g., I2P-only ↔ clearnet-only) are relayed through peers with dual connectivity.

```go
// Tor+I2P anonymous mode: only Tor and I2P registered, no direct IP access.
// NewMultiTransport() registers ip, tor, i2p, nym and lokinet by default.
// Build a custom instance with only the anonymous transports to restrict
// Tox DHT/UDP to I2P peers.
mt := transport.NewMultiTransport()
mt.RegisterTransport("tor", transport.NewTorTransport())
mt.RegisterTransport("i2p", transport.NewI2PTransport())
// With Tor+I2P co-registered: DialPacket() always routes through I2P.
// Dial(".onion") → Tor, Dial(".i2p") → I2P Streaming.
```

### NAT Traversal Configuration

```go
natTraversal := transport.NewNATTraversal()

// Configure STUN servers
natTraversal.SetSTUNServers([]string{
    "stun.l.google.com:19302",
    "stun.cloudflare.com:3478",
})

// Resolve public address
publicAddr, err := natTraversal.GetPublicAddress()
if err != nil {
    log.Printf("NAT traversal failed: %v", err)
}
```

## Adding New Networks

### Step 1: Implement Network Parser

```go
package transport

type MyNetworkParser struct {
    logger *logrus.Entry
}

func NewMyNetworkParser() *MyNetworkParser {
    return &MyNetworkParser{
        logger: logrus.WithField("component", "MyNetworkParser"),
    }
}

func (p *MyNetworkParser) ParseAddress(address string) (NetworkAddress, error) {
    // Parse address format specific to your network
    if !strings.HasSuffix(address, ".mynet") {
        return NetworkAddress{}, fmt.Errorf("invalid format for MyNetwork")
    }
    
    host, port, err := net.SplitHostPort(address)
    if err != nil {
        return NetworkAddress{}, err
    }
    
    portNum, err := strconv.Atoi(port)
    if err != nil {
        return NetworkAddress{}, err
    }
    
    return NetworkAddress{
        Type:    AddressTypeCustom,  // Define your own type
        Data:    []byte(address),
        Port:    uint16(portNum),
        Network: "mynet",
    }, nil
}

func (p *MyNetworkParser) ValidateAddress(addr NetworkAddress) error {
    if addr.Type != AddressTypeCustom {
        return fmt.Errorf("invalid address type for MyNetwork")
    }
    
    // Add your validation logic
    if len(addr.Data) < 10 {
        return fmt.Errorf("address too short")
    }
    
    return nil
}

func (p *MyNetworkParser) CanParse(address string) bool {
    return strings.HasSuffix(address, ".mynet")
}

func (p *MyNetworkParser) GetNetworkType() string {
    return "mynet"
}
```

### Step 2: Implement Network Transport

```go
type MyNetworkTransport struct {
    logger *logrus.Entry
}

func NewMyNetworkTransport() *MyNetworkTransport {
    return &MyNetworkTransport{
        logger: logrus.WithField("component", "MyNetworkTransport"),
    }
}

func (t *MyNetworkTransport) Listen(address string) (net.Listener, error) {
    // Implement listening logic for your network
    t.logger.WithField("address", address).Info("Starting MyNetwork listener")
    
    // Example: delegate to underlying protocol
    return net.Listen("tcp", address)
}

func (t *MyNetworkTransport) Dial(address string) (net.Conn, error) {
    // Implement connection logic for your network
    t.logger.WithField("address", address).Info("Connecting via MyNetwork")
    
    // Example: custom connection logic
    return net.Dial("tcp", address)
}

func (t *MyNetworkTransport) DialPacket(address string) (net.PacketConn, error) {
    // Implement packet connection logic
    return net.ListenPacket("udp", address)
}

func (t *MyNetworkTransport) SupportedNetworks() []string {
    return []string{"mynet"}
}

func (t *MyNetworkTransport) Close() error {
    t.logger.Info("Closing MyNetwork transport")
    return nil
}
```

### Step 3: Implement Network Detector

```go
type MyNetworkDetector struct{}

func NewMyNetworkDetector() *MyNetworkDetector {
    return &MyNetworkDetector{}
}

func (d *MyNetworkDetector) DetectCapabilities(addr net.Addr) transport.NetworkCapabilities {
    // Analyze your network's capabilities
    return transport.NetworkCapabilities{
        SupportsNAT:      false,  // MyNetwork doesn't need NAT
        SupportsUPnP:     false,  // No UPnP support
        IsPrivateSpace:   false,  // All addresses are public
        RoutingMethod:    transport.RoutingProxy,  // Uses proxy routing
    }
}

func (d *MyNetworkDetector) SupportedNetworks() []string {
    return []string{"mynet"}
}
```

### Step 4: Register Your Network

```go
// Register parser
parser := transport.NewMultiNetworkParser()
parser.RegisterNetwork("mynet", NewMyNetworkParser())

// Register transport
mt := transport.NewMultiTransport()
mt.RegisterTransport("mynet", NewMyNetworkTransport())

// MultiNetworkDetector uses built-in detectors; use it directly for capability detection.
// Custom detection requires implementing NetworkDetector in your own aggregation layer.
detector := transport.NewMultiNetworkDetector()
caps := detector.DetectCapabilities(addr) // detect capabilities for an address
_ = caps

// Your network is now available
addresses, err := parser.Parse("service.mynet:8080")
if err == nil {
    conn, err := mt.Dial("service.mynet:8080")
    // Handle connection...
}
```

### Step 5: Add Address Type (Optional)

If your network requires a new address type, add it to the enumeration:

```go
// In transport/address.go, add to AddressType constants:
const (
    // ... existing types ...
    AddressTypeMyNet    AddressType = 0x07  // Your custom network
)

// Update string conversion methods
func (at AddressType) String() string {
    switch at {
    // ... existing cases ...
    case AddressTypeMyNet:
        return "MyNet"
    default:
        return "Unknown"
    }
}
```

## Best Practices

### 1. Error Handling

```go
addresses, err := parser.Parse(userInput)
if err != nil {
    return fmt.Errorf("invalid address format: %w", err)
}

for _, addr := range addresses {
    if !addr.IsRoutable() {
        log.Printf("Address %s is not routable", addr.String())
        continue
    }
    // Process routable address
}
```

### 2. Resource Management

Always `defer Close()` on parsers, transports, and connections (as shown in the [Quick Start Guide](#quick-start-guide)).

### 3. Concurrent Operations

Use goroutines safely with proper synchronization:

```go
var wg sync.WaitGroup
addresses := []string{"addr1.onion:80", "addr2.i2p:443", "addr3.nym:1789"}

for _, addr := range addresses {
    wg.Add(1)
    go func(address string) {
        defer wg.Done()
        
        conn, err := mt.Dial(address)
        if err != nil {
            log.Printf("Failed to connect to %s: %v", address, err)
            return
        }
        defer conn.Close()
        
        // Handle connection
    }(addr)
}

wg.Wait()
```

### 4. Network-Specific Configuration

Configure each transport according to its requirements (Tor: SOCKS5 proxy and circuit preferences; I2P: SAM bridge and tunnel settings; Nym: mixnet parameters and gateway selection). Refer to the per-network configuration blocks in [Supported Networks](#supported-networks).

## Performance Considerations

### Benchmarking Results

- **Address Parsing**: ~150ns per address (sub-microsecond)
- **Network Detection**: ~30ns per capability check
- **Transport Selection**: <1μs per selection
- **Cross-Network Compatibility**: ~29ns per check

### Optimization Tips

1. **Reuse Parsers**: Create parsers once and reuse them across multiple parsing operations
2. **Connection Pooling**: Implement connection pooling for frequently accessed addresses
3. **Async Operations**: Use goroutines for parallel network operations (see [Concurrent Operations](#3-concurrent-operations))
4. **Caching**: Cache address parsing and network detection results when appropriate

## Troubleshooting

### Common Issues

#### 1. Address Format Errors

```
Error: "no parser found for address: xyz.unknown:123"
```

**Solution**: Ensure the address format matches a supported network type. Check supported networks:

```go
networks := parser.GetSupportedNetworks()
fmt.Printf("Supported networks: %v\n", networks)
```

#### 2. Transport Not Available

```
Error: "transport not registered for network type: custom"
```

**Solution**: Register the transport before use:

```go
mt.RegisterTransport("custom", NewCustomTransport())
```

### Debug Logging

Enable detailed logging for troubleshooting:

```go
import "github.com/sirupsen/logrus"

logrus.SetLevel(logrus.DebugLevel)
// All multi-network components will now emit debug-level logs
```

## Migration Guide

### From IP-Only to Multi-Network

1. **Replace direct net.Addr usage**:
```go
// Before
addr, err := net.ResolveTCPAddr("tcp", "192.168.1.1:8080")

// After  
parser := transport.NewMultiNetworkParser()
addresses, err := parser.Parse("192.168.1.1:8080")
netAddr := addresses[0].ToNetAddr()  // Convert back if needed
```

2. **Update connection logic**:
```go
// Before
conn, err := net.Dial("tcp", "192.168.1.1:8080")

// After
mt := transport.NewMultiTransport()
conn, err := mt.Dial("192.168.1.1:8080")  // Automatically selects IP transport
```

3. **Add privacy network support**:
```go
// Now you can also connect to privacy networks
conn, err := mt.Dial("example.onion:80")      // Tor
conn, err := mt.Dial("service.b32.i2p:443")   // I2P
conn, err := mt.Dial("gateway.nym:1789")      // Nym
```

### Backward Compatibility

The multi-network system is fully backward compatible: existing `net.Addr` interfaces, IP address parsing, and public APIs are unchanged. Gradual adoption is possible.

## Security Considerations

### Network-Specific Security

| Network | Key Properties | Risks |
|---------|---------------|-------|
| **IP** | Direct connectivity | Reveals location; vulnerable to traffic analysis |
| **Tor** | Strong anonymity via onion routing | Malicious exit nodes (non-HTTPS); circuit correlation |
| **I2P** | Decentralized garlic routing | Smaller network; longer setup time |
| **Nym** | Mixnet with cover traffic | Metadata-resistant; higher latency |

### Best Security Practices

1. **Validate All Addresses**: Always validate addresses before use
2. **Use TLS/Encryption**: Add application-layer encryption for sensitive data
3. **Network Isolation**: Consider network-specific configurations
4. **Regular Updates**: Keep transport implementations updated
5. **Monitor Connections**: Log and monitor network connections for anomalies

## Examples and Use Cases

Refer to the [Quick Start Guide](#quick-start-guide) for basic parsing and transport usage, [Adding New Networks](#adding-new-networks) for extending the system, and the [examples/](../examples/) directory for complete working programs.

## Conclusion

The multi-network system provides a unified, extensible interface for peer-to-peer communication across IP, Tor, I2P, Nym, and Lokinet with sub-microsecond core operations and full backward compatibility. See [NETWORK_ADDRESS.md](NETWORK_ADDRESS.md), [examples/](../examples/), and [transport/](../transport/) for further details.
