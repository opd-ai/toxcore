# Tor Transport Implementation

## Overview

The toxcore-go Tor transport implementation provides connectivity to Tor hidden services (.onion addresses) via SOCKS5 proxy. This enables routing Tox traffic through the Tor network for enhanced privacy and access to onion services.

## Features

- **SOCKS5 Proxy Support**: Connects to .onion addresses through Tor SOCKS5 proxy
- **Configurable Proxy**: Uses environment variable or default Tor proxy address
- **Thread-Safe**: Concurrent dial operations are safe
- **Standard Go net.Conn**: Returns standard Go connection interface for compatibility
- **Comprehensive Logging**: Structured logging for debugging and monitoring

## Requirements

1. **Running Tor Service**: You must have Tor running locally or accessible via network
2. **SOCKS5 Port**: Tor's SOCKS5 proxy must be accessible (default: 127.0.0.1:9050)

### Installing and Running Tor

**Linux (Debian/Ubuntu):**
```bash
sudo apt-get install tor
sudo systemctl start tor
```

**macOS:**
```bash
brew install tor
brew services start tor
```

**Tor Browser Bundle:**
- Default SOCKS5 port: 127.0.0.1:9150
- Set `TOR_PROXY_ADDR=127.0.0.1:9150`

## Usage

### Basic Connection

```go
package main

import (
    "log"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Create Tor transport (uses default proxy 127.0.0.1:9050)
    tor := transport.NewTorTransport()
    defer tor.Close()

    // Connect to a .onion address
    conn, err := tor.Dial("exampleonionaddress.onion:80")
    if err != nil {
        log.Fatal("Failed to connect:", err)
    }
    defer conn.Close()

    // Use connection like any net.Conn
    _, err = conn.Write([]byte("Hello through Tor!"))
    if err != nil {
        log.Fatal("Write failed:", err)
    }
}
```

### Custom Proxy Configuration

```go
import (
    "os"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Configure custom Tor proxy address
    os.Setenv("TOR_PROXY_ADDR", "127.0.0.1:9150") // Tor Browser
    
    tor := transport.NewTorTransport()
    defer tor.Close()
    
    // Now uses custom proxy
    conn, err := tor.Dial("example.onion:443")
    // ...
}
```

### Using with Multi-Transport

```go
import (
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Create multi-transport with Tor support
    mt := transport.NewMultiTransport()
    
    // Add Tor transport
    torTransport := transport.NewTorTransport()
    mt.AddTransport("tor", torTransport)
    
    // Add regular IP transport
    ipTransport := transport.NewIPTransport()
    mt.AddTransport("ip", ipTransport)
    
    // Multi-transport will choose appropriate transport based on address
    conn, err := mt.Dial("example.onion:80") // Uses Tor
    // ...
}
```

## Configuration

### Environment Variables

- `TOR_PROXY_ADDR`: Custom Tor SOCKS5 proxy address (default: `127.0.0.1:9050`)

### Common Proxy Addresses

| Tor Installation | Default SOCKS5 Address |
|-----------------|----------------------|
| System Tor (Linux/macOS) | 127.0.0.1:9050 |
| Tor Browser Bundle | 127.0.0.1:9150 |
| Custom Tor | Configured in torrc |

## Limitations

### Current Implementation

1. **Outbound Only**: Can only dial to .onion addresses, cannot host onion services
   - Hosting requires Tor control port or torrc configuration
   - Use regular IP transport for local binding

2. **TCP Only**: Tor transport only supports TCP connections
   - `DialPacket()` returns error (UDP over Tor is not supported)
   - Tox protocol can adapt to TCP-only mode

3. **No Circuit Control**: Uses Tor's default circuit selection
   - Cannot manually select exit nodes or circuit paths
   - For advanced circuit control, use Tor control protocol

### Hosting Onion Services

To accept connections as an onion service:

1. Configure Tor to create onion service in `torrc`:
```
HiddenServiceDir /var/lib/tor/tox_service/
HiddenServicePort 33445 127.0.0.1:33445
```

2. Use regular IP transport to listen locally:
```go
ipTransport := transport.NewIPTransport()
listener, err := ipTransport.Listen("127.0.0.1:33445")
// Tor will forward onion service traffic to this listener
```

3. Your onion address will be in `/var/lib/tor/tox_service/hostname`

## Error Handling

Common errors and solutions:

### "connection refused"
- Tor is not running
- Wrong proxy address configured
- Solution: Start Tor service or check `TOR_PROXY_ADDR`

### "dial timeout"
- Tor cannot reach the onion service
- The .onion address doesn't exist or is offline
- Solution: Verify onion address, check Tor logs

### "SOCKS5 dialer creation failed"
- Invalid proxy address format
- Solution: Check `TOR_PROXY_ADDR` format (should be "host:port")

## Performance Considerations

1. **Higher Latency**: Tor routing adds ~2-6 seconds latency
2. **Lower Bandwidth**: Tor bandwidth is limited by relay capacity
3. **Connection Overhead**: Circuit building takes time on first connection

For latency-sensitive applications (e.g., voice/video calls), consider:
- Using direct IP connections when privacy permits
- Implementing connection pooling and keep-alive
- Setting appropriate timeout values

## Security Notes

1. **End-to-End Encryption**: Tor provides transport-layer anonymity, Tox provides end-to-end encryption
2. **Identity Separation**: Tor hides IP address, Tox cryptographic identity is separate
3. **Traffic Correlation**: Long-lived connections may be vulnerable to traffic correlation attacks
4. **Trust Model**: You trust the Tor network for anonymity, not for encryption

## Testing

To test without real Tor:

```go
// Use non-existent proxy for unit testing error handling
os.Setenv("TOR_PROXY_ADDR", "127.0.0.1:19999")
tor := transport.NewTorTransport()
_, err := tor.Dial("test.onion:80")
// Expect connection refused error
```

For integration testing with real Tor, ensure Tor is running and use actual .onion addresses or local test services.

## Future Enhancements

Potential future improvements:

1. **Onion Service Hosting**: Tor control port integration for creating onion services
2. **Circuit Control**: Expose Tor control protocol for manual circuit management  
3. **Bridge Support**: Configuration for Tor bridges in censored networks
4. **Stream Isolation**: Per-connection circuit isolation for enhanced privacy
5. **Pluggable Transports**: Support for obfs4 and other Tor pluggable transports

## See Also

- [Tor Project Documentation](https://www.torproject.org/docs/documentation.html)
- [SOCKS5 Protocol (RFC 1928)](https://tools.ietf.org/html/rfc1928)
- [I2P Transport Implementation Plan](I2P_TRANSPORT.md) (future)
- [Nym Transport Implementation Plan](NYM_TRANSPORT.md) (future)
