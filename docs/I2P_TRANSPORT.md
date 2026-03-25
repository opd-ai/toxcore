# I2P Transport for Tox

## Overview

The toxcore-go I2P transport enables Tox peers to communicate over the I2P network using `.i2p` and `.b32.i2p` addresses. Built on the [onramp](https://github.com/go-i2p/onramp) library (which wraps the SAMv3 bridge protocol), it plugs into the toxcore `NetworkTransport` interface so that Tox friend connections, messaging, and file transfers work transparently over I2P — the same way they work over IP or Tor.

Because I2P is a fully internal network with no exit nodes, every Tox peer on I2P is both a client and a reachable destination. This makes I2P particularly well-suited for Tox's peer-to-peer model: both sides of a friend connection can dial *and* accept connections without NAT traversal or relay servers.

## Features

- **Drop-in Tox Transport**: Implements the `NetworkTransport` interface — Tox connections and messages work without protocol changes
- **Bidirectional Peer Connectivity**: Both `Dial()` and `Listen()` are fully supported, enabling true peer-to-peer Tox friend connections without relays
- **SAMv3 Protocol via onramp**: Communicates with the I2P router through the SAM bridge; the onramp `Garlic` instance handles session multiplexing and lifecycle management
- **Lazy Initialization**: The onramp `Garlic` instance is created on first use, not at transport creation
- **Key Persistence**: I2P destination keys are automatically stored in the `i2pkeys/` directory, giving your Tox node a stable I2P address across restarts
- **TCP Streaming**: Streaming connections to `.i2p` destinations via `Dial()` — carries Tox protocol traffic the same way TCP does over IP
- **I2P Datagrams**: Datagram communication via `DialPacket()` using I2P native datagrams — suitable for Tox's UDP-oriented protocol messages
- **Multi-Transport Integration**: Registered automatically in `MultiTransport`; the address suffix (`.i2p`) routes traffic to this transport
- **Thread-Safe**: All operations are protected by a read-write mutex for concurrent safety
- **Standard Go Interfaces**: Returns `net.Conn`, `net.Listener`, and `net.PacketConn` — no Tox code changes needed

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

### Hosting a Tox Node on I2P (Listen)

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

    // Listen creates a persistent I2P destination for your Tox node
    // Keys are automatically stored in the i2pkeys/ directory
    // NOTE: First call may take a few minutes for tunnel establishment
    listener, err := i2p.Listen("toxcore.b32.i2p")
    if err != nil {
        log.Fatal("Failed to listen:", err)
    }
    defer listener.Close()

    log.Printf("Tox node reachable at: %s", listener.Addr().String())

    // Accept incoming Tox friend connections
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

### Tox DHT Packets via Datagrams (DialPacket)

```go
package main

import (
    "log"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    i2p := transport.NewI2PTransport()
    defer i2p.Close()

    // Create an I2P datagram connection for Tox UDP packets
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

### How Tox Maps to I2P

The I2P transport fits into toxcore's `NetworkTransport` interface, so the Tox protocol layer uses it the same way it uses IP or Tor:

| Tox Operation | I2P Mechanism | Notes |
|---------------|---------------|-------|
| Connect to a friend | `Dial()` → SAM streaming session | Opens an I2P stream to the friend's `.b32.i2p` destination |
| Accept friend connections | `Listen()` → SAM listener | Your node gets a reachable `.b32.i2p` address — no NAT issues |
| Send/receive messages | `net.Conn` read/write | Tox protocol messages flow over the I2P stream unchanged |
| DHT packets | `DialPacket()` → SAM datagrams | Tox UDP packets sent as I2P datagrams (best-effort, like UDP) |
| Multi-network | `MultiTransport` routing | Addresses ending in `.i2p` are automatically routed to this transport |
| Tor+I2P anonymous mode | `MultiTransport.DialPacket()` → I2P | When Tor and I2P are both registered, all UDP/datagram traffic routes through I2P (Tor is TCP-only); Tox DHT messages only reach I2P peers |

Because I2P is a fully internal network (no exit nodes), both peers in a Tox friend connection are reachable destinations. This eliminates the need for relay servers or NAT hole-punching that Tox requires over clearnet IP.

### Layer Architecture

The I2P transport uses a layered architecture:

```
┌───────────────────────────────────────┐
│     Tox Protocol Layer                │
│  (friend connections, messaging,      │
│   file transfers, DHT)                │
├───────────────────────────────────────┤
│          I2PTransport                 │
│  (transport/i2p_transport_impl.go)    │
│  - Implements NetworkTransport        │
│  - Returns net.Conn / net.PacketConn  │
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

1. **`I2PTransport`** — Implements `NetworkTransport` for Tox. Holds the SAM address, a mutex for thread safety, and a lazily-initialized `onramp.Garlic` instance. Tox code interacts with this through the standard `Dial()`, `Listen()`, and `DialPacket()` methods.

2. **`onramp.Garlic`** — Wraps the SAM bridge protocol with automatic lifecycle management. Created with the `OPT_SMALL` option, which uses fewer tunnels — a good match for Tox's messaging workload (small, infrequent packets rather than bulk data).

3. **`sam3`** — The underlying SAMv3 protocol implementation that communicates with the I2P router. This is an indirect dependency used by onramp.

4. **`i2pkeys`** — Handles I2P destination key encoding and storage. Keys are persisted in the `i2pkeys/` directory so that your Tox node keeps the same `.b32.i2p` address across restarts.

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

2. **Initial Tunnel Establishment**: The first `Listen()` call may take a few minutes while I2P tunnels are being built.
   - Subsequent operations reuse established tunnels and are faster.
   - This is a one-time cost — once tunnels are ready, Tox connections proceed normally.

3. **Overlay Network Latency**: Like any anonymizing overlay, I2P adds latency compared to direct IP connections. Typical round-trip times within I2P are in the hundreds of milliseconds range — comparable to Tor hidden service connections. This is well within the tolerances of Tox text messaging and file transfers. For Tox audio/video calls, direct IP or Tor transport may be preferable depending on tunnel conditions.

4. **I2P Datagrams**: `DialPacket()` uses I2P native datagrams, not traditional UDP.
   - Datagram size is limited by I2P tunnel MTU (approximately 31 KB), which is well above Tox's typical packet sizes.
   - Delivery is best-effort (no guaranteed delivery, similar to UDP) — matching the semantics Tox already expects from UDP.

5. **Key Directory**: Persistent keys are stored in the `i2pkeys/` directory relative to the working directory.
   - Ensure the application has write access to this directory.
   - Deleting this directory will generate a new I2P destination on next startup (your Tox peer will get a new `.b32.i2p` address).

### Hosting a Tox Node on I2P

I2PTransport supports inbound connections, which means your Tox node can be reachable by other I2P-connected peers — no NAT traversal or relay needed:

```go
i2pTransport := transport.NewI2PTransport()
listener, err := i2pTransport.Listen("myservice.b32.i2p:33445")
// The onramp library creates the I2P destination automatically.
// Keys are persisted in the i2pkeys/ directory.
// Share this address with friends so they can reach your Tox node:
log.Printf("Tox node reachable at: %s", listener.Addr().String())
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

## Performance with Tox Workloads

Tox's traffic patterns are a good fit for I2P:

1. **Text Messaging**: Tox messages are small (typically under 1 KB). I2P handles these with latency comparable to Tor hidden services — generally hundreds of milliseconds round-trip. This is imperceptible for text chat.
2. **File Transfers**: Tox file transfers work over I2P streaming connections. Throughput depends on tunnel capacity, but the `OPT_SMALL` tunnel configuration used by the transport is adequate for typical file sharing.
3. **Tunnel Startup**: Initial tunnel establishment takes some time on first use. Once tunnels are built, the onramp `Garlic` instance reuses SAM sessions, so subsequent Tox operations proceed without additional delay.
4. **Audio/Video Calls**: Tox A/V requires low, consistent latency. Over any anonymizing overlay (I2P or Tor), call quality depends on tunnel conditions. For the best A/V experience, direct IP connections are recommended when privacy requirements allow it.
5. **Datagrams**: Tox's UDP-oriented protocol packets are well within I2P's datagram MTU (~31 KB). The additional overhead from I2P's encryption layers is negligible for Tox packet sizes.

## Security Notes

1. **Layered Encryption**: I2P encrypts traffic at the transport layer; Tox provides its own end-to-end encryption independently. Compromising I2P anonymity does not compromise Tox message confidentiality.
2. **No Exit Nodes**: Unlike Tor, I2P is a closed network — there are no exit nodes that could observe Tox traffic in transit. Every peer is an internal destination.
3. **IP Address Protection**: I2P hides your real IP address from Tox friends and from the network. Your Tox public key and your I2P destination are cryptographically independent identities.
4. **Garlic Routing**: I2P bundles multiple messages together (garlic routing), which adds resistance to traffic analysis compared to per-message routing.
5. **Destination Persistence**: I2P destination keys stored in `i2pkeys/` give your Tox node a stable `.b32.i2p` address. Protect this directory as you would protect private keys.
6. **Bidirectional Connectivity**: Because both Tox peers can accept inbound I2P connections, there is no reliance on third-party relays — reducing the attack surface compared to NAT-traversal techniques.

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

Potential improvements for Tox over I2P:

1. **Tunnel Configuration**: Expose onramp options for tunnel length and quantity — allow users to trade anonymity for speed depending on their threat model
2. **Friend Address Sharing**: Integrate I2P destination addresses into Tox friend request workflow
3. **Multi-Session Support**: Multiple independent SAM sessions for per-friend stream isolation
4. **Health Monitoring**: Periodic SAM bridge health checks with automatic reconnection to keep Tox connections alive
5. **DHT Bootstrap over I2P**: Support bootstrapping the Tox DHT through I2P-only nodes

## See Also

- [I2P Project Documentation](https://geti2p.net/)
- [SAMv3 Protocol Specification](https://geti2p.net/en/docs/api/samv3)
- [onramp Library (go-i2p/onramp)](https://github.com/go-i2p/onramp)
- [sam3 Library (go-i2p/sam3)](https://github.com/go-i2p/sam3)
- [Tor Transport Implementation](TOR_TRANSPORT.md)
- [Nym Transport Implementation](NYM_TRANSPORT.md)
- [Multi-Network Transport](MULTINETWORK.md)
