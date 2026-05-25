# Privacy Network Quick-Start Guide

This guide provides step-by-step instructions for setting up and testing toxcore-go with privacy networks (Tor, I2P, Lokinet, Nym) using Docker.

## Prerequisites

- Docker installed and running
- Go 1.25.0 or later
- toxcore-go repository cloned

```bash
git clone https://github.com/opd-ai/toxcore
cd toxcore
```

## Quick Start: All Networks with Docker Compose

Create a `docker-compose.yml` file for running all privacy networks:

```yaml
version: '3.8'

services:
  tor:
    image: dperson/torproxy:latest
    container_name: toxcore-tor
    ports:
      - "9050:9050"  # SOCKS5 proxy
      - "9051:9051"  # Control port
    environment:
      - TOR_SOCKS_PORT=9050
      - TOR_CONTROL_PORT=9051
    restart: unless-stopped

  i2pd:
    image: purplei2p/i2pd:latest
    container_name: toxcore-i2pd
    ports:
      - "7070:7070"  # Web console
      - "7656:7656"  # SAM bridge
    volumes:
      - i2pd-data:/home/i2pd/data
    restart: unless-stopped

volumes:
  i2pd-data:
```

Start all services:

```bash
docker-compose up -d
```

Verify services are running:

```bash
docker-compose ps
```

## Individual Network Setup

### Tor Network

#### 1. Start Tor Proxy

```bash
docker run -d --name toxcore-tor \
  -p 9050:9050 \
  -p 9051:9051 \
  dperson/torproxy:latest
```

#### 2. Verify Tor Connection

```bash
# Check if Tor is running
curl --socks5-hostname 127.0.0.1:9050 https://check.torproject.org/ | grep Congratulations

# Or simply check if the port is open
nc -zv 127.0.0.1 9050
```

#### 3. Configure toxcore-go

```go
opts := toxcore.NewOptions()
opts.UDPEnabled = false // Tor proxying is TCP-only by default
opts.Proxy = &toxcore.ProxyOptions{
    Type: toxcore.ProxyTypeSOCKS5,
    Host: "127.0.0.1",
    Port: 9050,
}

tox, err := toxcore.New(opts)
if err != nil {
    log.Fatal(err)
}
defer tox.Kill()
```

#### 4. Listen on Tor (Onion Service)

```go
tor := transport.NewTorTransport()
defer tor.Close()

// Tor listening automatically creates a hidden service.
listener, err := tor.Listen("toxcore.onion:33445")
if err != nil {
    log.Fatal(err)
}
defer listener.Close()

// Get your .onion address
onionAddr := listener.Addr()
log.Printf("Listening on: %s", onionAddr.String())
```

### I2P Network

#### 1. Start I2P Router with SAM Bridge

```bash
docker run -d --name toxcore-i2pd \
  -p 7070:7070 \
  -p 7656:7656 \
  purplei2p/i2pd:latest
```

#### 2. Verify I2P is Running

```bash
# Check SAM bridge
nc -zv 127.0.0.1 7656

# Access web console
open http://127.0.0.1:7070
```

#### 3. Configure toxcore-go

```go
i2p := transport.NewI2PTransportWithSAMAddr("127.0.0.1:7656")
defer i2p.Close()
```

#### 4. Listen on I2P

```go
i2p := transport.NewI2PTransportWithSAMAddr("127.0.0.1:7656")
defer i2p.Close()

listener, err := i2p.Listen("toxcore.b32.i2p")
if err != nil {
    log.Fatal(err)
}
defer listener.Close()

// Get your .b32.i2p address
i2pAddr := listener.Addr()
log.Printf("Listening on: %s", i2pAddr.String())
```

### Lokinet (Dial-Only)

Lokinet support is currently dial-only (listening not yet supported).

#### 1. Install Lokinet

```bash
# Ubuntu/Debian
curl -so /etc/apt/trusted.gpg.d/oxen.gpg https://deb.oxen.io/pub.gpg
echo "deb https://deb.oxen.io $(lsb_release -sc) main" | sudo tee /etc/apt/sources.list.d/oxen.list
sudo apt update && sudo apt install lokinet

# Start service
sudo systemctl start lokinet
```

#### 2. Configure toxcore-go

```go
lokinet := transport.NewLokinetTransport()
defer lokinet.Close()

conn, err := lokinet.Dial("example.loki:80")
if err != nil {
    log.Fatal(err)
}
defer conn.Close()
```

### Nym (Dial-Only)

Nym support is currently dial-only (listening not yet supported).

#### 1. Install Nym Client

```bash
# Download from https://nymtech.net/download/
wget https://github.com/nymtech/nym/releases/latest/download/nym-client-linux.tar.gz
tar -xzf nym-client-linux.tar.gz

# Initialize and start
./nym-client init --id toxcore-client
./nym-client run --id toxcore-client
```

#### 2. Configure toxcore-go

```go
nym := transport.NewNymTransport()
defer nym.Close()

conn, err := nym.Dial("example.nym:80")
if err != nil {
    log.Fatal(err)
}
defer conn.Close()
```

## Testing Connectivity

### Test Tor Connection

```go
package main

import (
    "log"
    "time"
    "github.com/opd-ai/toxcore"
)

func main() {
    opts := toxcore.NewOptions()
    opts.Proxy = &toxcore.ProxyOptions{
        Type: toxcore.ProxyTypeSOCKS5,
        Host: "127.0.0.1",
        Port: 9050,
    }
    opts.UDPEnabled = false  // Tor doesn't support UDP

    tox, err := toxcore.New(opts)
    if err != nil {
        log.Fatal(err)
    }
    defer tox.Kill()

    log.Println("Successfully created Tox instance with Tor proxy")
    log.Printf("Tox ID: %s", tox.SelfGetAddress())

    // Bootstrap through Tor
    bootstrapNodes := []struct {
        address string
        port    uint16
        pubkey  string
    }{
        // Add your bootstrap nodes here
    }

    for _, node := range bootstrapNodes {
        err := tox.Bootstrap(node.address, node.port, node.pubkey)
        if err != nil {
            log.Printf("Bootstrap failed: %v", err)
        }
    }

    // Run for 30 seconds
    for i := 0; i < 30; i++ {
        tox.Iterate()
        time.Sleep(time.Second)
    }
}
```

### Test I2P Connection

```go
package main

import (
    "log"
    "time"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    i2p := transport.NewI2PTransportWithSAMAddr("127.0.0.1:7656")
    defer i2p.Close()

    listener, err := i2p.Listen("toxcore-test.b32.i2p")
    if err != nil {
        log.Fatal(err)
    }
    defer listener.Close()

    log.Printf("Listening on I2P: %s", listener.Addr().String())

    // Accept connections
    go func() {
        for {
            conn, err := listener.Accept()
            if err != nil {
                log.Printf("Accept error: %v", err)
                return
            }
            log.Printf("New connection from: %s", conn.RemoteAddr())
            conn.Close()
        }
    }()

    // Keep running
    time.Sleep(5 * time.Minute)
}
```

## Multi-Network Example

Run a multi-transport dialer that supports all available networks:

```go
package main

import (
    "log"
    "os"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    _ = os.Setenv("TOR_CONTROL_ADDR", "127.0.0.1:9051")
    _ = os.Setenv("I2P_SAM_ADDR", "127.0.0.1:7656")
    _ = os.Setenv("LOKINET_PROXY_ADDR", "127.0.0.1:1090")
    _ = os.Setenv("NYM_CLIENT_ADDR", "127.0.0.1:1080")

    mt := transport.NewMultiTransport()
    defer mt.Close()

    for _, addr := range []string{
        "duckduckgogg42xjoc72x3sjasowoarfbgcmvfimaftt6twagswzczad.onion:80",
        "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80",
        "example.loki:80",
        "example.nym:80",
    } {
        conn, err := mt.Dial(addr)
        if err != nil {
            log.Printf("dial %s failed: %v", addr, err)
            continue
        }
        conn.Close()
    }
}
```

## Troubleshooting

### Tor Issues

**Problem**: "Connection refused" on port 9050

```bash
# Check if Tor is running
docker ps | grep toxcore-tor

# Check Tor logs
docker logs toxcore-tor

# Restart Tor
docker restart toxcore-tor
```

**Problem**: Tor connection is very slow

- Tor naturally has higher latency (typical: 500ms-2s)
- Consider using bridges if Tor is blocked in your region
- Ensure Docker container has enough resources

### I2P Issues

**Problem**: SAM bridge not accessible on port 7656

```bash
# Check if I2P is running
docker ps | grep toxcore-i2pd

# Check I2P logs
docker logs toxcore-i2pd

# Verify SAM is enabled in web console
open http://127.0.0.1:7070
```

**Problem**: I2P connections fail immediately

- I2P requires 5-10 minutes to integrate into the network on first startup
- Check web console for tunnel status
- Ensure sufficient peers (minimum 8-10 connections)

### General Network Issues

**Problem**: Cannot connect to any bootstrap nodes

```bash
# Test basic connectivity
curl -I https://nodes.tox.chat

# Test through Tor
curl --socks5-hostname 127.0.0.1:9050 -I https://nodes.tox.chat

# Check Go build tags
go test -tags nonet ./...  # Should pass (network tests disabled)
go test ./...              # May fail without privacy networks running
```

## Docker Cleanup

Stop and remove all containers:

```bash
# Using docker-compose
docker-compose down -v

# Or individually
docker stop toxcore-tor toxcore-i2pd
docker rm toxcore-tor toxcore-i2pd
docker volume rm toxcore_i2pd-data
```

## Performance Considerations

| Network | Latency | Bandwidth | Anonymity | Listening Support |
|---------|---------|-----------|-----------|-------------------|
| IPv4/IPv6 | <10ms | High | None | ✅ |
| Tor (.onion) | 500-2000ms | Low | High | ✅ |
| I2P (.b32.i2p) | 200-1000ms | Medium | High | ✅ |
| Lokinet (.loki) | 100-500ms | Medium | Medium | ❌ (dial-only) |
| Nym (.nym) | 1000-3000ms | Low | Very High | ❌ (dial-only) |

## Further Reading

- [Tor Transport Documentation](TOR_TRANSPORT.md)
- [I2P Transport Documentation](I2P_TRANSPORT.md)
- [Multi-Network Transport Overview](MULTINETWORK.md)
- [Network Address Format](NETWORK_ADDRESS.md)

## Examples

Full working examples are available in the `examples/` directory:

- `examples/tor_communication/` - Tor messaging example
- `examples/i2p_communication/` - I2P messaging example
- `examples/multi_network/` - Multi-network example

## Security Notes

1. **Never reuse Tox IDs across networks** - Generate separate identities for each privacy network
2. **Tor over VPN** - Consider using Tor over VPN for additional protection
3. **Metadata protection** - Enable identity obfuscation in async messaging
4. **Port forwarding** - Not required for privacy networks (they provide NAT traversal)
5. **Exit node selection** - Use Tor for outbound-only traffic to reduce exit node risk

## Contributing

Found an issue or want to improve this guide? Please open an issue or pull request on GitHub.
