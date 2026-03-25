# Tor Transport for Tox

## Overview

The toxcore-go Tor transport enables Tox peers to communicate over the Tor network using `.onion` hidden service addresses. Built on the [onramp](https://github.com/go-i2p/onramp) library (which wraps the Tor control protocol), it plugs into the toxcore `NetworkTransport` interface so that Tox friend connections and messaging work transparently over Tor — the same way they work over IP or I2P.

Because Tor hidden services provide end-to-end connectivity between two `.onion` addresses, both sides of a Tox friend connection can dial *and* accept connections without exposing their real IP addresses. The onramp library handles onion service creation, key persistence, and circuit management automatically.

## Features

- **Drop-in Tox Transport**: Implements the `NetworkTransport` interface — Tox connections and messages work without protocol changes
- **Bidirectional Peer Connectivity**: Both `Dial()` and `Listen()` are fully supported, enabling Tox friend connections over hidden services
- **onramp Library Integration**: Connects to and hosts .onion addresses via the onramp library; the `Onion` instance handles circuit management and lifecycle
- **Lazy Initialization**: The onramp `Onion` instance is created on first use of `Dial()` or `Listen()`, not at transport creation
- **Key Persistence**: Onion service keys are automatically stored in the `onionkeys/` directory, giving your Tox node a stable `.onion` address across restarts
- **Panic Recovery**: The onramp library panics when the Tor daemon is unreachable; the transport converts these panics into actionable Go errors instead of crashing callers
- **Configurable Control Port**: Uses `TOR_CONTROL_ADDR` environment variable or default Tor control port address
- **Multi-Transport Integration**: Registered automatically in `MultiTransport`; the address suffix (`.onion`) routes traffic to this transport
- **Thread-Safe**: All operations are protected by a read-write mutex for concurrent safety
- **Standard Go Interfaces**: Returns `net.Conn` and `net.Listener` — no Tox code changes needed
- **Comprehensive Logging**: Structured logging via logrus for debugging and monitoring

## Requirements

1. **Running Tor Service**: You must have Tor running locally or accessible via network
2. **Tor Control Port**: Tor daemon must be running with control port enabled (default: 127.0.0.1:9051)

### Installing and Running Tor

**Linux (Debian/Ubuntu):**
```bash
sudo apt-get install tor
sudo systemctl start tor
# Control port is enabled by default on 127.0.0.1:9051
```

**macOS:**
```bash
brew install tor
brew services start tor
# Control port is enabled by default on 127.0.0.1:9051
```

**Docker:**
```bash
docker run -d --name tor \
    -p 9051:9051 \
    dperson/torproxy
```

**Custom torrc Configuration:**
If you need to enable the control port manually, add these lines to your `torrc` file:
```
ControlPort 9051
CookieAuthentication 1
```

### Verifying Tor

You can verify that Tor is running and the control port is accessible:
```bash
# Test the control port connection (should not be refused)
nc -z 127.0.0.1 9051 && echo "Tor control port is reachable"
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

### Hosting a Tox Node on Tor (Listen)

```go
package main

import (
    "log"
    "net"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    tor := transport.NewTorTransport()
    defer tor.Close()

    // Listen creates a persistent onion service for your Tox node
    // Keys are automatically stored in the onionkeys/ directory
    // NOTE: First call may take 30-90 seconds for descriptor publishing
    listener, err := tor.Listen("myservice.onion:33445")
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

### Custom Control Port Configuration

```go
import (
    "os"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Configure custom Tor control port address
    os.Setenv("TOR_CONTROL_ADDR", "127.0.0.1:9051")

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

| Variable | Default | Description |
|----------|---------|-------------|
| `TOR_CONTROL_ADDR` | `127.0.0.1:9051` | Address of the local Tor control port |

### Common Control Port Addresses

| Tor Installation | Default Control Port Address |
|-----------------|------------------------------|
| System Tor (Linux/macOS) | `127.0.0.1:9051` |
| Tor Browser Bundle | `127.0.0.1:9051` |
| Docker (dperson/torproxy) | `127.0.0.1:9051` |
| Custom Tor | Configured in torrc (`ControlPort` directive) |

## Architecture

### How Tox Maps to Tor

The Tor transport fits into toxcore's `NetworkTransport` interface, so the Tox protocol layer uses it the same way it uses IP or I2P:

| Tox Operation | Tor Mechanism | Notes |
|---------------|---------------|-------|
| Connect to a friend | `Dial()` → onramp Onion TCP | Opens a Tor circuit to the friend's `.onion` address |
| Accept friend connections | `Listen()` → onramp Onion listener | Your node gets a reachable `.onion` address — no NAT issues |
| Send/receive messages | `net.Conn` read/write | Tox protocol messages flow over the Tor stream unchanged |
| DHT packets | Not supported | `DialPacket()` returns error — Tor does not support UDP |
| Multi-network | `MultiTransport` routing | Addresses ending in `.onion` are automatically routed to this transport |

Both peers in a Tox friend connection can host onion services, eliminating the need for relay servers or NAT hole-punching that Tox requires over clearnet IP.

### Layer Architecture

The Tor transport uses a layered architecture:

```
┌───────────────────────────────────────┐
│     Tox Protocol Layer                │
│  (friend connections, messaging,      │
│   file transfers)                     │
├───────────────────────────────────────┤
│          TorTransport                 │
│  (transport/network_transport_impl.go)│
│  - Implements NetworkTransport        │
│  - Returns net.Conn / net.Listener    │
│  - Panic recovery for onramp errors   │
├───────────────────────────────────────┤
│          onramp.Onion                 │
│  (github.com/go-i2p/onramp)          │
│  - Circuit management                │
│  - Key persistence (onionkeys/)      │
│  - Onion service lifecycle           │
├───────────────────────────────────────┤
│          cretz/bine                   │
│  (github.com/cretz/bine)             │
│  - Tor control protocol              │
│  - Process management                │
├───────────────────────────────────────┤
│          Tor Daemon                   │
│  (system tor service)                │
│  - Onion routing                     │
│  - Circuit building                  │
│  - Hidden service descriptors        │
└───────────────────────────────────────┘
```

### Key Components

1. **`TorTransport`** — Implements `NetworkTransport` for Tox. Holds the control port address, a mutex for thread safety, and a lazily-initialized `onramp.Onion` instance. Tox code interacts with this through the standard `Dial()`, `Listen()`, and `Close()` methods. Includes panic recovery to convert onramp panics into Go errors.

2. **`onramp.Onion`** — Wraps the Tor control protocol with automatic lifecycle management. Created with the name `"toxcore-tor"`, which determines the key file name (`onionkeys/toxcore-tor.onion.private`). Handles circuit building, descriptor publishing, and connection management.

3. **`cretz/bine`** — The underlying Tor control protocol implementation that communicates with the Tor daemon. This is an indirect dependency used by onramp.

4. **`onionkeys/`** — Directory where onion service private keys are persisted. This gives your Tox node a stable `.onion` address across restarts. Deleting this directory will generate a new onion address on next startup.

### Address Type Constant

The Tor address type is registered as `AddressTypeTor = 0x03` in the transport address system (see `transport/address.go`), enabling automatic address detection and routing in the multi-transport layer.

## Dependencies

| Package | Version | Role |
|---------|---------|------|
| `github.com/go-i2p/onramp` | v0.33.92 | Tor onion service wrapper with lifecycle management |
| `github.com/cretz/bine` | v0.2.0 | Tor control protocol implementation (indirect) |

## Limitations

### Current Implementation

1. **Tor Daemon Required**: A Tor daemon must be running with the control port enabled.
   - Without a running daemon, all operations (`Dial`, `Listen`) will fail during Onion initialization.
   - The transport converts onramp panics into actionable Go errors rather than crashing.

2. **TCP Only**: Tor transport only supports TCP connections.
   - `DialPacket()` returns an error — UDP over Tor is not supported by the Tor network.
   - Tox protocol adapts to TCP-only mode automatically; text messaging and file transfers work normally.

3. **Initial Descriptor Publishing**: The first `Listen()` call may take 30–90 seconds while the onion service descriptor is published to the Tor network.
   - Subsequent operations reuse the established service and are faster.
   - This is a one-time cost — once the descriptor is published, Tox connections proceed normally.

4. **No Circuit Control**: Uses Tor's default circuit selection.
   - Cannot manually select exit nodes or circuit paths.
   - For advanced circuit control, use the Tor control protocol directly.

5. **Key Directory**: Persistent keys are stored in the `onionkeys/` directory relative to the working directory.
   - Ensure the application has write access to this directory.
   - Deleting this directory will generate a new onion address on next startup (your Tox peer will get a new `.onion` address).

### Hosting Onion Services

TorTransport supports inbound connections, which means your Tox node can be reachable by other Tor-connected peers — no NAT traversal or relay needed:

```go
torTransport := transport.NewTorTransport()
listener, err := torTransport.Listen("myservice.onion:33445")
// The onramp library creates the onion service automatically.
// Keys are persisted in the onionkeys/ directory.
// Share this address with friends so they can reach your Tox node:
log.Printf("Tox node reachable at: %s", listener.Addr().String())
```

## Error Handling

Common errors and solutions:

### "Tor onramp initialization failed: ... (is Tor running?)"
- The Tor daemon is not running or the control port is not accessible
- This error is produced when the onramp library panics and the transport recovers
- Solution: Start Tor service and verify the control port is reachable
- Solution: Check `TOR_CONTROL_ADDR` environment variable

### "connection refused" / "Tor onramp initialization failed"
- Tor daemon is not running or the control port is not accessible
- Solution: Start Tor service or check `TOR_CONTROL_ADDR`

### "invalid Tor address format: ... (must contain .onion)"
- The address does not use the `.onion` suffix required by Tor transport
- Solution: Use addresses in the format `example.onion:port`

### "Tor listener creation failed"
- The onramp library could not create an onion service listener
- Descriptor publishing may have timed out
- Solution: Check Tor daemon logs, ensure control port is healthy

### "dial timeout"
- Tor cannot reach the onion service
- The .onion address doesn't exist or is offline
- Solution: Verify onion address, check Tor logs

### "Tor onramp initialization failed: invalid control address"
- Invalid control port address format
- Solution: Check `TOR_CONTROL_ADDR` format (should be `host:port`)

## Performance with Tox Workloads

Tox's traffic patterns work well over Tor hidden services:

1. **Text Messaging**: Tox messages are small (typically under 1 KB). Tor hidden service connections add ~2–6 seconds of initial circuit-building latency, but once established, message round-trip times are in the hundreds of milliseconds range. This is imperceptible for text chat.

2. **File Transfers**: Tox file transfers work over Tor streaming connections. Throughput depends on the Tor circuit's relay capacity, but is adequate for typical file sharing. Large files will transfer more slowly than over direct IP.

3. **Descriptor Publishing**: Initial onion service setup takes 30–90 seconds for descriptor publishing. Once the service is live, the onramp `Onion` instance reuses the same circuits, so subsequent Tox operations proceed without additional delay.

4. **Audio/Video Calls**: Tox A/V requires low, consistent latency. Over Tor, call quality depends on circuit conditions and typically has 2–6 seconds of added latency. For the best A/V experience, direct IP connections are recommended when privacy requirements allow it.

5. **Connection Overhead**: Tor circuit building adds latency to the first connection. Subsequent connections through the same transport reuse established circuits and are faster. Consider using connection keep-alive for long-lived Tox sessions.

## Security Notes

1. **Layered Encryption**: Tor provides transport-layer anonymity; Tox provides its own end-to-end encryption independently. Compromising Tor anonymity does not compromise Tox message confidentiality.

2. **IP Address Protection**: Tor hides your real IP address from Tox friends and from the network. Your Tox public key and your `.onion` address are cryptographically independent identities.

3. **Identity Separation**: Tor hides IP address; Tox cryptographic identity is separate. An attacker who learns your `.onion` address cannot determine your real IP.

4. **Traffic Correlation**: Long-lived connections may be vulnerable to traffic correlation attacks by a global passive adversary. For stronger resistance to traffic analysis, consider the [Nym transport](NYM_TRANSPORT.md) which uses cover traffic.

5. **Exit Node Risk (Clearnet)**: When dialing non-`.onion` addresses through Tor, traffic exits through a Tor exit node which could observe unencrypted data. Tox's end-to-end encryption mitigates this, but the exit node can see the destination IP. For `.onion`-to-`.onion` connections, there are no exit nodes.

6. **Onion Service Persistence**: Onion service keys stored in `onionkeys/` give your Tox node a stable `.onion` address. Protect this directory as you would protect private keys — an attacker who obtains these keys can impersonate your onion service.

7. **Trust Model**: You trust the Tor network for anonymity (hiding your IP), not for encryption (Tox handles that). The Tor network cannot read Tox message contents.

## Testing

### Unit Tests (No Tor Daemon Required)

```bash
# Run all Tor transport tests
go test ./transport/... -run TestTor -v
```

The unit tests verify:
- Transport creation with default and custom control addresses
- Supported network types (`["tor"]`)
- Address validation (rejects non-`.onion` addresses)
- Error handling when Tor daemon is unavailable
- Concurrent safety
- Idempotent `Close()` operations

### Integration Tests (Requires Running Tor Daemon)

For integration testing with a real Tor daemon, ensure Tor is running with the control port enabled, then use actual `.onion` addresses:

```go
// Verify Tor daemon is reachable before running integration tests
tor := transport.NewTorTransport()
defer tor.Close()

// Attempt to listen (will create an onion service)
listener, err := tor.Listen("integrationtest.onion:33445")
if err != nil {
    t.Skip("Tor daemon not available:", err)
}
defer listener.Close()

log.Printf("Test onion address: %s", listener.Addr().String())
```

### Testing Without a Real Tor Daemon

```go
// Use non-existent control port for unit testing error handling
os.Setenv("TOR_CONTROL_ADDR", "127.0.0.1:19999")
tor := transport.NewTorTransport()
_, err := tor.Dial("test.onion:80")
// Expect: "Tor dial failed: Tor onramp initialization failed: ..."
```

## Example Application

A complete example demonstrating Tor transport usage is available in [`examples/tor_transport_demo/`](../examples/tor_transport_demo/):

```bash
# Build and run the Tor transport demo
cd examples/tor_transport_demo
go build -o tor_demo .
./tor_demo
```

The example creates a Tor transport, displays supported networks, attempts to create an onion service listener, and dials a `.onion` address. Connection failures are handled gracefully if the Tor daemon is not running.

Additionally, the [`examples/privacy_networks/`](../examples/privacy_networks/) example demonstrates Tor alongside I2P and Lokinet transports in a single application.

## Future Enhancements

Potential improvements for Tox over Tor:

1. **Circuit Control**: Expose Tor control protocol for manual circuit management
2. **Bridge Support**: Configuration for Tor bridges in censored networks
3. **Stream Isolation**: Per-connection circuit isolation for enhanced privacy
4. **Pluggable Transports**: Support for obfs4 and other Tor pluggable transports
5. **Onion Service v3 Options**: Expose advanced v3 onion service configuration (client authorization, max streams)
6. **Health Monitoring**: Periodic control port health checks with automatic reconnection to keep Tox connections alive

## See Also

- [Tor Project Documentation](https://www.torproject.org/docs/documentation.html)
- [onramp Library (go-i2p/onramp)](https://github.com/go-i2p/onramp)
- [bine Library (cretz/bine)](https://github.com/cretz/bine)
- [I2P Transport Implementation](I2P_TRANSPORT.md)
- [Nym Transport Implementation](NYM_TRANSPORT.md)
- [Multi-Network Transport](MULTINETWORK.md)
