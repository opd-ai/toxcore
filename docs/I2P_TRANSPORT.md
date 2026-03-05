# I2P Transport Implementation

## Overview

The toxcore-go I2P transport implementation provides connectivity to I2P destinations (`.i2p` and `.b32.i2p` addresses) via the [onramp](https://github.com/go-i2p/onramp) library, which wraps the SAMv3 bridge protocol. This enables routing Tox traffic through the I2P network for enhanced privacy, and supports dialing to remote destinations, hosting I2P services, and I2P datagram communication.

Unlike the Tor transport which uses a SOCKS5 proxy, the I2P transport communicates directly with the I2P router through the SAM (Simple Anonymous Messaging) v3 protocol. The onramp library provides automatic lifecycle management including key persistence, session multiplexing, and signal-based cleanup.

## Features

- **onramp Library Integration**: Connects to and hosts `.i2p` destinations via the onramp `Garlic` instance
- **SAMv3 Protocol**: Communicates with the I2P router through the SAM bridge (v3) protocol
- **Lazy Initialization**: The onramp `Garlic` instance is created on first use, not at transport creation
- **Key Persistence**: I2P destination keys are automatically stored in the `i2pkeys/` directory for persistent identities
- **TCP Streaming**: Full TCP-like streaming connections to `.i2p` destinations via `Dial()`
- **Listener Support**: Host I2P services with automatic tunnel setup via `Listen()`
- **I2P Datagrams**: UDP-like datagram communication via `DialPacket()` using I2P native datagrams
- **Thread-Safe**: All operations are protected by a read-write mutex for concurrent safety
- **Configurable SAM Address**: Uses `I2P_SAM_ADDR` environment variable or default SAM port
- **Standard Go Interfaces**: Returns `net.Conn`, `net.Listener`, and `net.PacketConn` for compatibility
- **Comprehensive Logging**: Structured logging via logrus for debugging and monitoring

## Requirements

1. **Running I2P Router**: You must have an I2P router (i2pd or Java I2P) running
2. **SAM Bridge Enabled**: The I2P router must have the SAM bridge enabled (default port: 7656)

### Installing and Running I2P

**Linux (Debian/Ubuntu) — i2pd (lightweight C++ router):**
```bash
sudo apt-get install i2pd
sudo systemctl start i2pd
# SAM bridge is enabled by default on 127.0.0.1:7656
```

**Linux (Debian/Ubuntu) — Java I2P:**
```bash
# Download from https://geti2p.net/en/download
# After installation, start the I2P router:
i2prouter start
# Enable the SAM bridge via the I2P console at http://127.0.0.1:7657/configclients
# SAM bridge listens on 127.0.0.1:7656 by default
```

**macOS:**
```bash
brew install i2pd
brew services start i2pd
# SAM bridge is enabled by default on 127.0.0.1:7656
```

**Docker (i2pd):**
```bash
docker run -d --name i2pd \
    -p 7656:7656 \
    purplei2p/i2pd --sam.enabled=true --sam.address=0.0.0.0 --sam.port=7656
```

### Verifying SAM Bridge

You can verify the SAM bridge is running by connecting to it:
```bash
# Should return "HELLO REPLY RESULT=OK VERSION=3.1" or similar
echo "HELLO VERSION" | nc 127.0.0.1 7656
```

## Usage

### Basic Connection (Dial)

```go
package main

import (
    "log"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Create I2P transport (uses default SAM address 127.0.0.1:7656)
    i2p := transport.NewI2PTransport()
    defer i2p.Close()

    // Connect to a .b32.i2p address
    conn, err := i2p.Dial("ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80")
    if err != nil {
        log.Fatal("Failed to connect:", err)
    }
    defer conn.Close()

    // Use connection like any net.Conn
    _, err = conn.Write([]byte("Hello through I2P!"))
    if err != nil {
        log.Fatal("Write failed:", err)
    }
}
```

### Hosting an I2P Service (Listen)

```go
package main

import (
    "log"
    "net"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    i2p := transport.NewI2PTransport()
    defer i2p.Close()

    // Listen creates a persistent I2P destination
    // Keys are automatically stored in the i2pkeys/ directory
    // NOTE: First call may take 2-5 minutes for tunnel establishment
    listener, err := i2p.Listen("toxcore.b32.i2p")
    if err != nil {
        log.Fatal("Failed to listen:", err)
    }
    defer listener.Close()

    log.Printf("I2P service available at: %s", listener.Addr().String())

    // Accept incoming connections
    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Printf("Accept error: %v", err)
            continue
        }
        go handleConnection(conn)
    }
}

func handleConnection(conn net.Conn) {
    defer conn.Close()
    buf := make([]byte, 4096)
    n, err := conn.Read(buf)
    if err != nil {
        log.Printf("Read error: %v", err)
        return
    }
    log.Printf("Received: %s", buf[:n])
}
```

### Datagram Communication (DialPacket)

```go
package main

import (
    "log"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    i2p := transport.NewI2PTransport()
    defer i2p.Close()

    // Create an I2P datagram connection (UDP-like)
    pconn, err := i2p.DialPacket("destination.b32.i2p:9000")
    if err != nil {
        log.Fatal("Failed to create datagram connection:", err)
    }
    defer pconn.Close()

    log.Printf("Local I2P datagram address: %s", pconn.LocalAddr().String())

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
}
```

### Custom SAM Bridge Configuration

```go
import (
    "os"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Configure custom SAM bridge address
    os.Setenv("I2P_SAM_ADDR", "127.0.0.1:7657")

    i2p := transport.NewI2PTransport()
    defer i2p.Close()

    // Now uses custom SAM bridge
    conn, err := i2p.Dial("example.b32.i2p:8080")
    // ...
}
```

### Using with Multi-Transport

```go
import (
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // NewMultiTransport already registers IP, Tor, I2P, Lokinet, and Nym transports
    mt := transport.NewMultiTransport()
    defer mt.Close()

    // Multi-transport automatically selects I2P for .i2p addresses
    conn, err := mt.Dial("example.b32.i2p:80") // Uses I2P
    // ...
}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `I2P_SAM_ADDR` | `127.0.0.1:7656` | Address of the local I2P SAM bridge |

### Common SAM Bridge Addresses

| I2P Installation | Default SAM Address |
|-----------------|---------------------|
| i2pd (default) | `127.0.0.1:7656` |
| Java I2P (default) | `127.0.0.1:7656` |
| Custom configuration | Set via `I2P_SAM_ADDR` |

## Architecture

### How It Works

The I2P transport uses a layered architecture:

```
┌───────────────────────────────────────┐
│          I2PTransport                 │
│  (transport/network_transport_impl.go)│
├───────────────────────────────────────┤
│          onramp.Garlic                │
│  (github.com/go-i2p/onramp)          │
│  - Session multiplexing              │
│  - Key persistence (i2pkeys/)        │
│  - Lifecycle management              │
├───────────────────────────────────────┤
│          SAM v3 Protocol              │
│  (github.com/go-i2p/sam3)            │
│  - Stream connections                │
│  - Datagram support                  │
│  - Session management                │
├───────────────────────────────────────┤
│          I2P Router                   │
│  (i2pd or Java I2P)                  │
│  - Garlic routing                    │
│  - Tunnel management                 │
│  - Peer discovery                    │
└───────────────────────────────────────┘
```

### Key Components

1. **`I2PTransport`** — The main transport struct, holding the SAM address, a mutex for thread safety, and a lazily-initialized `onramp.Garlic` instance.

2. **`onramp.Garlic`** — Wraps the SAM bridge protocol with automatic lifecycle management. Created with the `OPT_SMALL` option for reduced tunnel count (suitable for low-bandwidth messaging).

3. **`sam3`** — The underlying SAMv3 protocol implementation that communicates with the I2P router. This is an indirect dependency used by onramp.

4. **`i2pkeys`** — Handles I2P destination key encoding and storage. Keys are persisted in the `i2pkeys/` directory so that the same I2P destination is reused across restarts.

### Address Types

The transport supports two I2P address formats:

| Format | Example | Description |
|--------|---------|-------------|
| `.i2p` | `example.i2p:80` | Human-readable I2P address (requires address book lookup) |
| `.b32.i2p` | `ukeu3k5...dnkdq.b32.i2p:80` | Base32-encoded I2P destination hash (self-contained) |

Both formats are accepted by `Dial()`, `Listen()`, and `DialPacket()`.

### Address Type Constant

The I2P address type is registered as `AddressTypeI2P = 0x04` in the transport address system (see `transport/address.go`), enabling automatic address detection and routing in the multi-transport layer.

## Dependencies

| Package | Version | Role |
|---------|---------|------|
| `github.com/go-i2p/onramp` | v0.33.92 | SAM bridge wrapper with lifecycle management |
| `github.com/go-i2p/sam3` | v0.33.92 | SAMv3 protocol implementation (indirect) |
| `github.com/go-i2p/i2pkeys` | v0.33.92 | I2P destination key handling (indirect) |

## Limitations

### Current Implementation

1. **I2P Router Required**: An I2P router (i2pd or Java I2P) must be running with the SAM bridge enabled.
   - Without a running router, all operations (`Dial`, `Listen`, `DialPacket`) will fail during garlic initialization.

2. **Initial Tunnel Establishment**: The first `Listen()` call may take 2–5 minutes while I2P tunnels are being built.
   - Subsequent operations reuse established tunnels and are faster.

3. **Higher Latency**: I2P uses garlic routing (bundled encrypted messages), which inherently adds more latency than Tor.
   - Not suitable for real-time applications (voice/video calls).
   - Well-suited for asynchronous messaging and file transfers.

4. **I2P Datagrams**: `DialPacket()` uses I2P native datagrams, not traditional UDP.
   - Datagram size is limited by I2P tunnel MTU (approximately 31 KB).
   - Delivery is best-effort (no guaranteed delivery, similar to UDP).

5. **Key Directory**: Persistent keys are stored in the `i2pkeys/` directory relative to the working directory.
   - Ensure the application has write access to this directory.
   - Deleting this directory will generate a new I2P destination on next startup.

### Hosting I2P Services

I2PTransport supports full I2P service hosting via the onramp library:

```go
i2pTransport := transport.NewI2PTransport()
listener, err := i2pTransport.Listen("myservice.b32.i2p:33445")
// The onramp library creates the I2P destination automatically.
// Keys are persisted in the i2pkeys/ directory.
log.Printf("I2P service available at: %s", listener.Addr().String())
```

## Error Handling

Common errors and solutions:

### "I2P onramp initialization failed"
- The SAM bridge is not running or not reachable at the configured address
- Solution: Start the I2P router and ensure SAM bridge is enabled
- Solution: Check `I2P_SAM_ADDR` environment variable

### "I2P dial failed"
- The onramp Garlic instance could not establish a connection to the destination
- The `.i2p` destination may be offline or unreachable
- Solution: Verify the I2P destination address, check I2P router status

### "invalid I2P address format: ... (must contain .i2p)"
- The address does not use the `.i2p` suffix required by I2P transport
- Solution: Use addresses in the format `destination.b32.i2p:port` or `destination.i2p:port`

### "I2P listener creation failed"
- The onramp library could not create an I2P listener
- Tunnel establishment may have timed out
- Solution: Check I2P router logs, ensure SAM bridge is healthy

### "I2P datagram creation failed"
- The onramp library could not create a datagram connection
- Solution: Check I2P router status and SAM bridge availability

## Performance Considerations

1. **Higher Latency**: I2P garlic routing adds significant latency (typically 2–10 seconds per hop)
2. **Tunnel Build Time**: Initial tunnel establishment takes 2–5 minutes
3. **Session Reuse**: The onramp `Garlic` instance reuses SAM sessions, so subsequent operations after initialization are faster
4. **Small Tunnels**: The transport uses `OPT_SMALL` for reduced tunnel count, trading bandwidth for lower resource usage
5. **Datagram Overhead**: I2P datagrams have additional overhead compared to regular UDP due to garlic encryption

For latency-sensitive applications (e.g., voice/video calls), consider:
- Using direct IP connections when privacy permits
- Using Tor transport for lower-latency anonymity
- Implementing connection pooling and keep-alive
- Setting appropriate timeout values

## Security Notes

1. **End-to-End Encryption**: I2P provides transport-layer anonymity; Tox provides end-to-end encryption on top
2. **Identity Separation**: I2P hides your IP address; your Tox cryptographic identity is separate from your I2P destination
3. **Garlic Routing**: I2P bundles multiple messages together (garlic routing), making traffic analysis harder than with Tor's onion routing
4. **No Exit Nodes**: Unlike Tor, I2P is designed primarily for internal network services — there are no "exit nodes" that could monitor clearnet traffic
5. **Destination Persistence**: I2P destination keys stored in `i2pkeys/` provide a stable identity. Protect this directory as you would protect private keys
6. **Trust Model**: You trust the I2P network for anonymity, not for encryption — Tox encryption operates independently

## Testing

### Unit Tests (No I2P Router Required)

```bash
# Run all I2P transport tests
go test ./transport/... -run TestI2P -v
```

The unit tests verify:
- Transport creation with default and custom SAM addresses
- Supported network types (`["i2p"]`)
- Address validation (rejects non-I2P addresses)
- Error handling when SAM bridge is unavailable
- Concurrent safety
- Idempotent `Close()` operations

### Integration Tests (Requires Running I2P Router)

For integration testing with a real I2P router, ensure the router is running with SAM bridge enabled, then use actual `.b32.i2p` addresses:

```go
// Verify SAM bridge is reachable before running integration tests
i2p := transport.NewI2PTransport()
defer i2p.Close()

// Attempt to listen (will create an I2P destination)
listener, err := i2p.Listen("integrationtest.b32.i2p")
if err != nil {
    t.Skip("I2P router not available:", err)
}
defer listener.Close()

log.Printf("Test destination: %s", listener.Addr().String())
```

### Testing Without a Real I2P Router

```go
// Use non-existent SAM bridge for unit testing error handling
os.Setenv("I2P_SAM_ADDR", "127.0.0.1:39999")
i2p := transport.NewI2PTransport()
_, err := i2p.Dial("test.b32.i2p:80")
// Expect: "I2P dial failed: I2P onramp initialization failed: ..."
```

## Example Application

A complete example demonstrating I2P transport usage is available in [`examples/privacy_networks/`](../examples/privacy_networks/):

```bash
# Build and run the privacy networks example
cd examples/privacy_networks
go build -o privacy_networks .
./privacy_networks
```

The example creates an I2P transport, displays supported networks, and attempts a connection to a `.b32.i2p` address. Connection failures are handled gracefully if the I2P router is not running.

## Future Enhancements

Potential future improvements:

1. **Tunnel Configuration**: Expose onramp options for tunnel length, quantity, and backup tunnels
2. **Address Book Integration**: Support for I2P address book lookups (`.i2p` to `.b32.i2p` resolution)
3. **Lease Set Management**: Fine-grained control over I2P lease set publishing
4. **Multi-Session Support**: Multiple independent SAM sessions for stream isolation
5. **Health Monitoring**: Periodic SAM bridge health checks and automatic reconnection

## See Also

- [I2P Project Documentation](https://geti2p.net/)
- [SAMv3 Protocol Specification](https://geti2p.net/en/docs/api/samv3)
- [onramp Library (go-i2p/onramp)](https://github.com/go-i2p/onramp)
- [sam3 Library (go-i2p/sam3)](https://github.com/go-i2p/sam3)
- [Tor Transport Implementation](TOR_TRANSPORT.md)
- [Nym Transport Implementation](NYM_TRANSPORT.md)
- [Multi-Network Transport](MULTINETWORK.md)
