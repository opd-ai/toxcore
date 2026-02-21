# Nym Transport Implementation

## Overview

The toxcore-go Nym transport implementation provides connectivity through the Nym mixnet via a local Nym native client running in SOCKS5 mode. The Nym network provides stronger anonymity than Tor through cover traffic and mixnet delays, making it well-suited for privacy-sensitive messaging.

## Features

- **SOCKS5 Proxy Support**: Connects to `.nym` addresses through the Nym SOCKS5 proxy
- **Configurable Proxy**: Uses environment variable or default Nym client address
- **Thread-Safe**: Concurrent dial operations are safe
- **UDP-over-Stream (DialPacket)**: Length-prefixed packet framing over a SOCKS5 stream
- **Standard Go net.Conn / net.PacketConn**: Returns standard interfaces for compatibility
- **Actionable Errors**: Clear error messages when the Nym client is not reachable
- **Comprehensive Logging**: Structured logging via logrus for debugging and monitoring

## Requirements

1. **Running Nym Client**: A Nym native client must be running in SOCKS5 mode locally
2. **SOCKS5 Port**: The Nym client's SOCKS5 interface must be accessible (default: `127.0.0.1:1080`)

### Installing and Running the Nym Client

Download the Nym native client from the [Nym GitHub releases](https://github.com/nymtech/nym/releases).

**Initialize and run in SOCKS5 mode:**
```bash
# Initialize a new Nym client identity
nym-socks5-client init --id myid --provider <service-provider-address>

# Run the client (exposes SOCKS5 on 127.0.0.1:1080 by default)
nym-socks5-client run --id myid
```

**Using Docker:**
```bash
docker run -p 1080:1080 nymtech/nym-socks5-client:latest run --id myid
```

## Usage

### Basic Connection

```go
package main

import (
    "log"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Create Nym transport (uses default proxy 127.0.0.1:1080)
    nym := transport.NewNymTransport()
    defer nym.Close()

    // Connect to a .nym address through the Nym mixnet
    conn, err := nym.Dial("exampleservice.nym:80")
    if err != nil {
        log.Fatal("Failed to connect:", err)
    }
    defer conn.Close()

    // Use connection like any net.Conn
    _, err = conn.Write([]byte("Hello through Nym!"))
    if err != nil {
        log.Fatal("Write failed:", err)
    }
}
```

### Packet (UDP-like) Connection

```go
nym := transport.NewNymTransport()
defer nym.Close()

// UDP-like connection via length-prefixed framing over SOCKS5 stream
pconn, err := nym.DialPacket("exampleservice.nym:9000")
if err != nil {
    log.Fatal("Failed to create packet connection:", err)
}
defer pconn.Close()

// Write a datagram
_, err = pconn.WriteTo([]byte("ping"), pconn.LocalAddr())
if err != nil {
    log.Fatal("WriteTo failed:", err)
}

// Read a datagram
buf := make([]byte, 4096)
n, addr, err := pconn.ReadFrom(buf)
if err != nil {
    log.Fatal("ReadFrom failed:", err)
}
log.Printf("Received %d bytes from %v", n, addr)
```

### Custom Proxy Configuration

```go
import (
    "os"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Configure custom Nym client address
    os.Setenv("NYM_CLIENT_ADDR", "127.0.0.1:1090")

    nym := transport.NewNymTransport()
    defer nym.Close()

    conn, err := nym.Dial("example.nym:443")
    // ...
}
```

### Using with Multi-Transport

```go
import (
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // MultiTransport automatically selects Nym for .nym addresses
    mt := transport.NewMultiTransport()
    defer mt.Close()

    // Routes to NymTransport based on .nym suffix
    conn, err := mt.Dial("example.nym:80")
    // ...
}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NYM_CLIENT_ADDR` | `127.0.0.1:1080` | Address of the local Nym SOCKS5 proxy |

### Common Nym Client Addresses

| Configuration | Default SOCKS5 Address |
|--------------|----------------------|
| Nym native client (default) | `127.0.0.1:1080` |
| Custom port | Configured via `NYM_CLIENT_ADDR` |

## Packet Framing (DialPacket)

Since the Nym SOCKS5 interface is stream-based, `DialPacket` emulates UDP-like semantics using
length-prefixed framing over a TCP stream. Each packet is transmitted as:

```
[4 bytes: uint32 big-endian payload length][N bytes: payload]
```

Both sender and receiver must use `NymPacketConn` (returned by `DialPacket`) for correct framing.
The maximum packet size is limited by the receiving buffer passed to `ReadFrom`.

## Limitations

### Current Implementation

1. **No Listener Support**: Nym's SOCKS5 proxy only supports outbound connections.
   - Incoming connections require configuring a Nym service provider application.
   - `Listen()` returns an error directing users to the Nym service provider framework.

2. **SOCKS5 Only**: The current implementation uses the Nym client's built-in SOCKS5 interface.
   - The native Nym WebSocket API (port 1977) is not used in this implementation.
   - SOCKS5 mode provides equivalent connectivity with simpler integration.

3. **Higher Latency**: Nym routing adds latency due to mixnet delays and cover traffic.
   - Not suitable for real-time applications (voice/video calls).
   - Best suited for async messaging, file transfers, and privacy-critical operations.

4. **Address Format**: Addresses must contain `.nym` (e.g., `service.nym:80`).

### Hosting Nym Services

To accept connections as a Nym service:

1. Configure a Nym network requester or service provider application
2. Register your service with the Nym service provider framework
3. See the [Nym documentation](https://nymtech.net/docs) for service provider setup

## Error Handling

### "Nym dial failed (is Nym client running on 127.0.0.1:1080?)"
- The Nym SOCKS5 client is not running or not reachable
- Solution: Start the Nym native client with `nym-socks5-client run --id myid`
- Solution: Check the `NYM_CLIENT_ADDR` environment variable

### "Nym SOCKS5 dialer creation failed"
- Invalid proxy address format
- Solution: Check `NYM_CLIENT_ADDR` format (should be `host:port`)

### "invalid Nym address format: ... (must contain .nym)"
- The address does not use the `.nym` suffix required by Nym transport
- Solution: Use addresses in the format `service.nym:port`

### "Nym service hosting not supported via SOCKS5"
- `Listen()` is not supported; Nym SOCKS5 only supports outbound connections
- Solution: Configure a Nym service provider for inbound connections

## Performance Considerations

1. **High Latency**: Nym mixnet introduces intentional delays (seconds to tens of seconds)
2. **Cover Traffic**: The Nym client sends cover traffic regardless of actual usage
3. **Strong Anonymity**: Unlike Tor, Nym is resistant to global passive adversaries

For latency-sensitive applications, use direct IP or Tor transport instead.

## Security Notes

1. **Stronger Than Tor**: Nym's mixnet provides resistance to traffic correlation attacks
2. **Cover Traffic**: The Nym client continuously sends dummy packets to prevent traffic analysis
3. **End-to-End Encryption**: Nym provides transport-layer anonymity; Tox provides end-to-end encryption
4. **Trust Model**: You trust the Nym mixnet nodes for anonymity, not for encryption

## Testing

### Unit Tests (No Nym Client Required)

```bash
# Run all Nym transport tests (no real Nym client needed)
go test ./transport/... -run TestNym
```

### Integration Tests (Requires Running Nym Client)

```bash
# Ensure Nym client is running, then:
NYM_INTEGRATION_TEST=1 go test ./transport/... -run TestNymTransport_Integration -v
```

### Testing Without a Real Nym Client

```go
// Use non-existent proxy for unit testing error handling
os.Setenv("NYM_CLIENT_ADDR", "127.0.0.1:19999")
nym := transport.NewNymTransport()
_, err := nym.Dial("test.nym:80")
// Expect: "Nym dial failed (is Nym client running on 127.0.0.1:19999?): ..."
```

## See Also

- [Nym Project Documentation](https://nymtech.net/docs)
- [Nym GitHub Repository](https://github.com/nymtech/nym)
- [Tor Transport Implementation](TOR_TRANSPORT.md)
- [I2P Transport Implementation](I2P_TRANSPORT.md)
- [Multi-Network Transport](MULTINETWORK.md)
