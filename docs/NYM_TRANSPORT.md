# Nym Transport for Tox

## Overview

The toxcore-go Nym transport enables Tox peers to communicate through the [Nym mixnet](https://nymtech.net/) via a local Nym native client running in SOCKS5 mode. The Nym network provides stronger anonymity guarantees than Tor by introducing cover traffic and mixnet delays, making it resistant to global passive adversaries who can observe all network traffic.

Because Nym operates as a mixnet rather than an onion-routing network, it is particularly well-suited for privacy-critical Tox messaging where metadata protection is a higher priority than low latency. The transport plugs into the toxcore `NetworkTransport` interface so that Tox friend connections and messaging work transparently over Nym — the same way they work over IP, Tor, or I2P.

## Features

- **Drop-in Tox Transport**: Implements the `NetworkTransport` interface — Tox connections and messages work without protocol changes
- **SOCKS5 Proxy Support**: Connects to `.nym` addresses through the Nym native client's SOCKS5 interface
- **Configurable Proxy**: Uses `NYM_CLIENT_ADDR` environment variable or default Nym client address
- **Eager Initialization with Retry**: The SOCKS5 dialer is initialized eagerly at creation time; if initialization fails, it automatically retries on the next `Dial()` or `DialPacket()` call
- **UDP-over-Stream (DialPacket)**: Length-prefixed packet framing over a SOCKS5 stream, emulating UDP-like semantics for Tox's datagram protocol
- **Thread-Safe**: All operations are protected by a read-write mutex for concurrent safety
- **Standard Go Interfaces**: Returns `net.Conn` and `net.PacketConn` — no Tox code changes needed
- **Actionable Errors**: Clear error messages when the Nym client is not reachable, including the configured proxy address
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

**Building from source:**
```bash
# Requires Rust toolchain
git clone https://github.com/nymtech/nym.git
cd nym
cargo build --release -p nym-socks5-client
./target/release/nym-socks5-client init --id myid --provider <service-provider-address>
./target/release/nym-socks5-client run --id myid
```

### Verifying the Nym Client

You can verify the Nym client SOCKS5 proxy is running:
```bash
# Should accept connections (not refused)
nc -z 127.0.0.1 1080 && echo "Nym SOCKS5 proxy is reachable"

# Or test with curl through the SOCKS5 proxy
curl --socks5 127.0.0.1:1080 http://example.com
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

### Packet (UDP-like) Connection (DialPacket)

```go
package main

import (
    "log"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
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
}
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
| Docker deployment | `127.0.0.1:1080` |
| Custom port | Configured via `NYM_CLIENT_ADDR` |

## Architecture

### How Tox Maps to Nym

The Nym transport fits into toxcore's `NetworkTransport` interface, so the Tox protocol layer uses it the same way it uses IP, Tor, or I2P:

| Tox Operation | Nym Mechanism | Notes |
|---------------|---------------|-------|
| Connect to a friend | `Dial()` → SOCKS5 → Nym mixnet | Opens a stream connection through the Nym mixnet |
| Accept friend connections | Not supported | `Listen()` returns error — requires Nym service provider framework |
| Send/receive messages | `net.Conn` read/write | Tox protocol messages flow over the Nym stream unchanged |
| DHT packets | `DialPacket()` → length-prefixed framing over SOCKS5 | Tox UDP packets framed as length-prefixed datagrams over TCP |
| Multi-network | `MultiTransport` routing | Addresses ending in `.nym` are automatically routed to this transport |

Because the Nym SOCKS5 proxy only supports outbound connections, Nym transport is primarily suited for **client-initiated** Tox connections. For bidirectional peer-to-peer connectivity, combine with another transport (e.g., I2P or Tor for inbound, Nym for outbound).

### Layer Architecture

The Nym transport uses a layered architecture:

```
┌───────────────────────────────────────┐
│     Tox Protocol Layer                │
│  (friend connections, messaging,      │
│   file transfers)                     │
├───────────────────────────────────────┤
│          NymTransport                 │
│  (transport/network_transport_impl.go)│
│  - Implements NetworkTransport        │
│  - Returns net.Conn / net.PacketConn  │
│  - SOCKS5 dialer with retry logic     │
├───────────────────────────────────────┤
│          NymPacketConn                │
│  (transport/nym_packetconn.go)        │
│  - Length-prefixed packet framing     │
│  - UDP-like semantics over stream     │
├───────────────────────────────────────┤
│          golang.org/x/net/proxy       │
│  - SOCKS5 protocol client            │
│  - TCP connection through proxy       │
├───────────────────────────────────────┤
│          Nym Native Client            │
│  (nym-socks5-client)                  │
│  - SOCKS5 server interface            │
│  - Mixnet packet routing              │
│  - Cover traffic generation           │
│  - Sphinx packet construction         │
├───────────────────────────────────────┤
│          Nym Mixnet                   │
│  (mix nodes, gateways, validators)    │
│  - Multi-hop mixing                   │
│  - Timing obfuscation                 │
│  - Cover traffic mixing               │
└───────────────────────────────────────┘
```

### Key Components

1. **`NymTransport`** — Implements `NetworkTransport` for Tox. Holds the SOCKS5 proxy address, a mutex for thread safety, and an eagerly-initialized SOCKS5 dialer. If the dialer fails to initialize at creation time (Nym client not running), it automatically retries on the next `Dial()` or `DialPacket()` call.

2. **`NymPacketConn`** — Implements `net.PacketConn` over a stream connection using length-prefixed framing. Each packet is transmitted as a 4-byte big-endian length prefix followed by the payload. This emulates UDP-like semantics over the Nym SOCKS5 stream transport. Handles oversized packets gracefully by draining the stream to maintain synchronization.

3. **`golang.org/x/net/proxy`** — Provides the SOCKS5 client implementation. The proxy dialer routes TCP connections through the local Nym client's SOCKS5 interface into the Nym mixnet.

4. **Nym Native Client** — External process that must be running separately. Exposes a SOCKS5 interface (default port 1080) and handles Sphinx packet construction, mixnet routing, and cover traffic generation.

### Address Type

Nym addresses use the `.nym` suffix (e.g., `service.nym:80`). The `MultiTransport` automatically routes addresses containing `.nym` to this transport.

## Packet Framing (DialPacket)

Since the Nym SOCKS5 interface is stream-based, `DialPacket` emulates UDP-like semantics using
length-prefixed framing over a TCP stream. Each packet is transmitted as:

```
[4 bytes: uint32 big-endian payload length][N bytes: payload]
```

### Wire Format Details

- **Length prefix**: 4 bytes, unsigned 32-bit integer in big-endian byte order
- **Payload**: Variable length, up to the size of the receiving buffer
- **Oversized packets**: If a received packet exceeds the buffer size, the payload is drained from the stream to maintain synchronization, and an error is returned
- **WriteTo addr parameter**: Accepted for `net.PacketConn` interface compliance but semantically unused — the underlying SOCKS5 stream connection determines the actual destination, so all writes go to the remote end established during `DialPacket()`
- **ReadFrom addr return**: Always returns the remote address of the underlying SOCKS5 connection

Both sender and receiver must use `NymPacketConn` (returned by `DialPacket`) for correct framing. The maximum packet size is limited by the receiving buffer passed to `ReadFrom`.

## Dependencies

| Package | Version | Role |
|---------|---------|------|
| `golang.org/x/net` | v0.50.0 | SOCKS5 proxy client (`golang.org/x/net/proxy`) |

## Limitations

### Current Implementation

1. **No Listener Support**: Nym's SOCKS5 proxy only supports outbound connections.
   - Incoming connections require configuring a Nym service provider application.
   - `Listen()` returns an error directing users to the Nym service provider framework.
   - For bidirectional Tox peer-to-peer connectivity, combine Nym (outbound) with I2P or Tor (inbound).

2. **SOCKS5 Only**: The current implementation uses the Nym client's built-in SOCKS5 interface.
   - The native Nym WebSocket API (port 1977) is not used in this implementation.
   - SOCKS5 mode provides equivalent connectivity with simpler integration.

3. **Higher Latency**: Nym routing adds intentional latency due to mixnet delays and cover traffic.
   - Round-trip times can range from seconds to tens of seconds.
   - Not suitable for real-time applications (voice/video calls).
   - Best suited for async messaging, file transfers, and privacy-critical operations.

4. **Address Format**: Addresses must contain `.nym` (e.g., `service.nym:80`).

5. **External Process Required**: The Nym native client (`nym-socks5-client`) must be running as a separate process.
   - The transport communicates with it over the local SOCKS5 interface.
   - If the Nym client stops, subsequent `Dial()` and `DialPacket()` calls will fail with actionable error messages.

### Hosting Nym Services

To accept incoming connections as a Nym service:

1. Configure a Nym network requester or service provider application
2. Register your service with the Nym service provider framework
3. Set up a Nym service provider that forwards incoming mixnet traffic to your Tox node's local port
4. See the [Nym documentation](https://nymtech.net/docs) for detailed service provider setup instructions

## Error Handling

Common errors and solutions:

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

### "Nym packet dial failed (is Nym client running on ...?)"
- `DialPacket()` could not establish a connection through the SOCKS5 proxy
- Solution: Same as for `Dial()` — verify the Nym client is running

### "nym packet: buffer too small (...) for packet of size ..."
- The receiving buffer passed to `ReadFrom()` is smaller than the incoming packet
- The packet payload is drained to maintain stream synchronization
- Solution: Increase the buffer size in `ReadFrom()` calls (4096 bytes is recommended for Tox)

## Performance with Tox Workloads

Nym's mixnet design prioritizes anonymity over speed, which affects different Tox workloads:

1. **Text Messaging**: Tox messages are small (typically under 1 KB). Nym's mixnet adds seconds to tens of seconds of latency per message, which is noticeable but acceptable for asynchronous text chat. For real-time conversation, Tor or direct IP is more appropriate.

2. **File Transfers**: Tox file transfers work over the Nym SOCKS5 connection. Throughput is limited by mixnet capacity and cover traffic scheduling. Small files (documents, images) transfer adequately; large files will be significantly slower than over direct connections.

3. **Cover Traffic**: The Nym client continuously sends dummy packets regardless of actual usage. This provides strong traffic analysis resistance but consumes bandwidth even when the Tox node is idle.

4. **Audio/Video Calls**: Tox A/V requires low, consistent latency. Nym's intentional mixing delays make real-time audio/video impractical over this transport. Use direct IP or Tor transport for A/V calls.

5. **Async Messaging**: Nym is an excellent match for Tox's async messaging feature. The asynchronous nature of store-and-forward messaging is unaffected by mixnet latency, and the strong anonymity properties protect message metadata.

6. **Connection Establishment**: The SOCKS5 connection to the Nym client is fast (local), but the first packet traversing the mixnet incurs the full mixing delay. Subsequent packets on the same stream flow through established mixnet routes.

## Security Notes

1. **Stronger Than Tor**: Nym's mixnet provides resistance to **global passive adversaries** — attackers who can observe all network traffic simultaneously. Tor's onion routing is vulnerable to traffic correlation by such adversaries; Nym's mixing and cover traffic defeat this attack.

2. **Cover Traffic**: The Nym client continuously sends dummy packets indistinguishable from real traffic. This prevents traffic analysis attacks that infer communication patterns from packet timing and volume.

3. **Mixnet Architecture**: Unlike Tor's circuit-based routing, Nym uses a mixnet where each packet is independently mixed through multiple layers. This provides per-packet unlinkability — an attacker cannot correlate incoming and outgoing packets at a mix node.

4. **Layered Encryption**: Nym provides transport-layer anonymity through Sphinx packet encryption; Tox provides its own end-to-end encryption independently. Compromising Nym anonymity does not compromise Tox message confidentiality.

5. **Trust Model**: You trust the Nym mixnet nodes for anonymity (hiding your communication patterns), not for encryption (Tox handles that). The Nym network cannot read Tox message contents.

6. **Metadata Protection**: Nym protects not just message contents but also communication metadata — who talks to whom, when, and how much. This is a stronger guarantee than Tor, which primarily protects IP addresses.

7. **No Listener Exposure**: Since the Nym SOCKS5 transport is outbound-only, your Tox node does not expose a network-reachable endpoint. This reduces your attack surface compared to transports that accept inbound connections.

## Testing

### Unit Tests (No Nym Client Required)

```bash
# Run all Nym transport tests (no real Nym client needed)
go test ./transport/... -run TestNym -v
```

The unit tests verify:
- Transport creation with default and custom proxy addresses
- Supported network types (`["nym"]`)
- Address validation (rejects non-`.nym` addresses)
- Error handling when Nym client is unavailable
- SOCKS5 dialer retry logic
- `NymPacketConn` framing (length-prefix encoding/decoding)
- Concurrent safety
- `Close()` operations

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

## Example Application

The [`examples/privacy_networks/`](../examples/privacy_networks/) example demonstrates the Nym transport alongside Tor and I2P transports in a single application:

```bash
# Build and run the privacy networks example
cd examples/privacy_networks
go build -o privacy_networks .
./privacy_networks
```

The example creates transport instances for each privacy network, displays supported network types, and attempts connections. Connection failures are handled gracefully if the respective network daemons are not running.

## Future Enhancements

Potential improvements for Tox over Nym:

1. **WebSocket API**: Integrate with the Nym native WebSocket API (port 1977) for more control over mixnet routing and cover traffic parameters
2. **Service Provider Integration**: Built-in support for Nym service provider framework to enable `Listen()` for inbound Tox connections
3. **Surb-Based Reply**: Use Single Use Reply Blocks (SURBs) for anonymous reply channels — enabling bidirectional messaging without a persistent listener
4. **Cover Traffic Tuning**: Expose cover traffic parameters so users can trade bandwidth for stronger anonymity depending on their threat model
5. **Health Monitoring**: Periodic SOCKS5 proxy health checks with automatic reconnection to keep Tox connections alive
6. **Mixnet Route Selection**: Control over mixnet topology selection for latency/anonymity tradeoffs

## See Also

- [Nym Project Documentation](https://nymtech.net/docs)
- [Nym GitHub Repository](https://github.com/nymtech/nym)
- [Tor Transport Implementation](TOR_TRANSPORT.md)
- [I2P Transport Implementation](I2P_TRANSPORT.md)
- [Multi-Network Transport](MULTINETWORK.md)
