# Tor Transport Implementation

## Overview

The toxcore-go Tor transport implementation provides connectivity to Tor hidden services (.onion addresses) via the onramp library. This enables routing Tox traffic through the Tor network for enhanced privacy, and supports both dialing to and hosting .onion services.

## Features

- **onramp Library Integration**: Connects to and hosts .onion addresses via the onramp library
- **Configurable Control Port**: Uses environment variable or default Tor control port address
- **Thread-Safe**: Concurrent dial operations are safe
- **Standard Go net.Conn**: Returns standard Go connection interface for compatibility
- **Comprehensive Logging**: Structured logging for debugging and monitoring

## Requirements

1. **Running Tor Service**: You must have Tor running locally or accessible via network
2. **Tor Control Port**: Tor daemon must be running with control port enabled (default: 127.0.0.1:9051)

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
- Control port is typically accessible at 127.0.0.1:9051
- Set `TOR_CONTROL_ADDR=127.0.0.1:9051` if needed

## Usage

### Basic Connection

```go
package main

import (
    "log"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Create Tor transport (uses default control port 127.0.0.1:9051)
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
    // Configure custom Tor control port address
    os.Setenv("TOR_CONTROL_ADDR", "127.0.0.1:9051") // Custom Tor control port
    
    tor := transport.NewTorTransport()
    defer tor.Close()
    
    // Now uses custom control port
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
    // NewMultiTransport already registers IP, Tor, I2P, and Nym transports
    mt := transport.NewMultiTransport()
    defer mt.Close()

    // Multi-transport automatically selects Tor for .onion addresses
    conn, err := mt.Dial("example.onion:80") // Uses Tor
    // ...
}
```

## Configuration

### Environment Variables

- `TOR_CONTROL_ADDR`: Tor control port address (default: `127.0.0.1:9051`)

### Common Control Port Addresses

| Tor Installation | Default Control Port Address |
|-----------------|------------------------------|
| System Tor (Linux/macOS) | 127.0.0.1:9051 |
| Custom Tor | Configured in torrc (ControlPort directive) |

## Limitations

### Current Implementation

1. **Listen Requires .onion Address**: `Listen()` requires an address containing `.onion`.
   - Onion service creation is handled automatically by the onramp library.
   - Keys are persisted in the `onionkeys/` directory across restarts.

2. **TCP Only**: Tor transport only supports TCP connections
   - `DialPacket()` returns error (UDP over Tor is not supported)
   - Tox protocol can adapt to TCP-only mode

3. **No Circuit Control**: Uses Tor's default circuit selection
   - Cannot manually select exit nodes or circuit paths
   - For advanced circuit control, use Tor control protocol

### Hosting Onion Services

TorTransport supports full onion service hosting via the onramp library:

```go
torTransport := transport.NewTorTransport()
listener, err := torTransport.Listen("anythinghere.onion:33445")
// The onramp library creates the onion service automatically.
// Keys are persisted in the onionkeys/ directory.
```

## Error Handling

Common errors and solutions:

### "connection refused" / "Tor onramp initialization failed"
- Tor daemon is not running or the control port is not accessible
- Solution: Start Tor service or check `TOR_CONTROL_ADDR`

### "dial timeout"
- Tor cannot reach the onion service
- The .onion address doesn't exist or is offline
- Solution: Verify onion address, check Tor logs

### "Tor onramp initialization failed: invalid control address"
- Invalid control port address format
- Solution: Check `TOR_CONTROL_ADDR` format (should be "host:port")

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
// Use non-existent control port for unit testing error handling
os.Setenv("TOR_CONTROL_ADDR", "127.0.0.1:19999")
tor := transport.NewTorTransport()
_, err := tor.Dial("test.onion:80")
// Expect: "Tor onramp initialization failed: ..."
```

For integration testing with real Tor, ensure Tor is running and use actual .onion addresses or local test services.

## Future Enhancements

Potential future improvements:

1. **Circuit Control**: Expose Tor control protocol for manual circuit management
2. **Bridge Support**: Configuration for Tor bridges in censored networks
3. **Stream Isolation**: Per-connection circuit isolation for enhanced privacy
4. **Pluggable Transports**: Support for obfs4 and other Tor pluggable transports

## See Also

- [Tor Project Documentation](https://www.torproject.org/docs/documentation.html)
- [I2P Transport Implementation Plan](I2P_TRANSPORT.md) (future)
- [Nym Transport Implementation Plan](NYM_TRANSPORT.md) (future)
