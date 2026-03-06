# Bootstrap Server

## Overview

The toxcore-go `bootstrap` package provides a simple, straightforward API for
running a Tox DHT bootstrap server simultaneously on:

- **Clearnet** — standard UDP at a public IP address and port
- **Onion** — Tor v3 hidden service (`.onion` address)
- **I2P** — I2P destination (`.b32.i2p` address)

All three endpoints share the same cryptographic identity (public key), so
clients can verify they are connecting to the same node regardless of which
network they use.

## Features

- **Single-key identity** — one public key across all three network types
- **Key persistence** — restore a server's identity on restart via `Config.SecretKey`
- **Graceful shutdown** — `Stop()` waits for background goroutines to finish
- **Context-aware startup** — `Start(ctx)` honours `Config.StartupTimeout` for
  Tor/I2P tunnel establishment
- **Thread-safe** — all exported methods are safe for concurrent use
- **Standard Go interfaces** — built on `net.Conn`, `net.Listener`, `net.PacketConn`
- **Structured logging** — via logrus, configurable with `Config.Logger`

## Requirements

### Clearnet

No special requirements. A publicly routable IP address and open UDP port are
needed for the server to be reachable by peers on the internet.

### Tor (onion)

- A running Tor daemon with the control port enabled (default: `127.0.0.1:9051`).
- Override with the `TOR_CONTROL_ADDR` environment variable.

**Linux:**
```bash
sudo apt-get install tor
sudo systemctl start tor
```

**macOS:**
```bash
brew install tor && brew services start tor
```

**torrc** (if control port is disabled):
```
ControlPort 9051
CookieAuthentication 1
```

### I2P

- A running I2P router (i2pd or Java I2P) with the SAM bridge enabled (default: `127.0.0.1:7656`).
- Override with the `I2P_SAM_ADDR` environment variable.

**Linux (i2pd):**
```bash
sudo apt-get install i2pd
sudo systemctl start i2pd
```

**Verify SAM bridge:**
```bash
echo "HELLO VERSION" | nc 127.0.0.1 7656
```

## Usage

### Clearnet-only bootstrap server

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/opd-ai/toxcore/bootstrap"
)

func main() {
    cfg := bootstrap.DefaultConfig()
    cfg.ClearnetPort = 33445

    srv, err := bootstrap.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    if err := srv.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
    defer srv.Stop()

    fmt.Println("Clearnet:", srv.GetClearnetAddr())
    fmt.Println("Pubkey:  ", srv.GetPublicKeyHex())
    // ... block until shutdown signal
}
```

### Multi-network bootstrap server

```go
cfg := bootstrap.DefaultConfig()
cfg.ClearnetPort  = 33445
cfg.OnionEnabled  = true   // requires Tor daemon
cfg.I2PEnabled    = true   // requires I2P router with SAM bridge

srv, err := bootstrap.New(cfg)
if err != nil { log.Fatal(err) }

if err := srv.Start(context.Background()); err != nil { log.Fatal(err) }
defer srv.Stop()

fmt.Println("Clearnet:", srv.GetClearnetAddr())
fmt.Println("Onion:   ", srv.GetOnionAddr())
fmt.Println("I2P:     ", srv.GetI2PAddr())
fmt.Println("Pubkey:  ", srv.GetPublicKeyHex())
```

### Persistent identity across restarts

```go
// --- First run: generate and save secret key ---
srv, _ := bootstrap.New(bootstrap.DefaultConfig())
srv.Start(context.Background())

secretKey := srv.GetPrivateKey()
// store secretKey securely (e.g. encrypted file, environment variable)
srv.Stop()

// --- Subsequent runs: restore identity ---
cfg := bootstrap.DefaultConfig()
cfg.SecretKey = secretKey  // 32-byte secret key from previous run

srv2, _ := bootstrap.New(cfg)
srv2.Start(context.Background())
// srv2.GetPublicKey() == srv.GetPublicKey()  ✓
```

## Configuration

| Field | Default | Description |
|---|---|---|
| `ClearnetEnabled` | `true` | Enable UDP clearnet service |
| `ClearnetPort` | `33445` | UDP port; use 0 to let the OS pick |
| `OnionEnabled` | `false` | Enable Tor hidden service (onramp manages Tor internally via `TOR_CONTROL_ADDR` env) |
| `I2PEnabled` | `false` | Enable I2P endpoint |
| `I2PSAMAddr` | `"127.0.0.1:7656"` | I2P SAM bridge address; takes precedence over `I2P_SAM_ADDR` env var when non-empty |
| `SecretKey` | `nil` | 32-byte secret key for identity persistence |
| `StartupTimeout` | `30s` | Timeout for Tor/I2P tunnel establishment |
| `IterationInterval` | `50ms` | DHT iteration loop period |
| `Logger` | `logrus.StandardLogger()` | Custom logrus logger |

## API

```go
// Create a server (does not start listening).
func New(config *Config) (*Server, error)

// Start all enabled network endpoints.
func (s *Server) Start(ctx context.Context) error

// Stop all endpoints and release resources.
func (s *Server) Stop() error

// Identity
func (s *Server) GetPublicKey() [32]byte
func (s *Server) GetPublicKeyHex() string   // lowercase hex, 64 chars
func (s *Server) GetPrivateKey() []byte     // 32 bytes; store securely

// Endpoint addresses (empty string if disabled or not yet started)
func (s *Server) GetClearnetAddr() string   // "0.0.0.0:port" (toxcore always binds 0.0.0.0)
func (s *Server) GetOnionAddr() string      // "*.onion:port"
func (s *Server) GetI2PAddr() string        // "*.b32.i2p:port"

// Status
func (s *Server) IsRunning() bool
```

## Architecture

```
bootstrap.Server
├── Clearnet (UDP)
│   └── toxcore.Tox (standard DHT node with injected key pair)
│       └── transport.UDPTransport → standard UDP socket
├── Onion (TCP via Tor)
│   ├── transport.TorTransport.Listen() → net.Listener (.onion)
│   ├── transport.NewTCPTransportFromListener(listener)
│   └── dht.BootstrapManager (handles get_nodes, ping, etc.)
└── I2P (TCP via I2P)
    ├── transport.I2PTransport.Listen() → net.Listener (.b32.i2p)
    ├── transport.NewTCPTransportFromListener(listener)
    └── dht.BootstrapManager
```

The `NewTCPTransportFromListener` function (added to the `transport` package as
part of this feature) creates a full DHT-capable TCP transport from any
`net.Listener`, enabling clean integration with Tor and I2P listeners obtained
through the `NetworkTransport` interface.

## Limitations

- **Tor listener establishment** may take 30–90 seconds on first run while Tor
  publishes the hidden service descriptor. Subsequent starts are faster because
  the key is persisted in `onionkeys/`.
- **I2P tunnel establishment** may take 2–5 minutes on first run. Keys are
  persisted in `i2pkeys/`.
- **Tor is TCP-only.** The Tox DHT is primarily UDP-based, but the onion
  endpoint speaks the Tox protocol over TCP (using `transport.TCPTransport`)
  which is fully supported by the toxcore DHT stack.
- **I2P datagram** support is available in the underlying `I2PTransport` but the
  bootstrap server currently uses streaming connections for simplicity.

## Security Notes

- `GetPrivateKey()` returns the raw 32-byte secret key. **Store it securely** —
  it is the server's identity. Exposure allows impersonation.
- All three endpoints share the same key, so a single key compromise affects
  all networks.
- The public key published in bootstrap node lists identifies the server to
  clients. Clients use it to verify Noise-IK handshakes.

## Testing

```bash
# Clearnet-only tests (no external services required)
go test ./bootstrap/...

# Skip tests that require external networks (Tor, I2P)
go test -tags nonet ./bootstrap/...

# Example demo
go run ./examples/bootstrap_server_demo --port 33445
go run ./examples/bootstrap_server_demo --port 33445 --onion --i2p
```

## Example Application

See [`examples/bootstrap_server_demo/`](../examples/bootstrap_server_demo/) for
a complete command-line demo that accepts `--port`, `--onion`, and `--i2p` flags
and prints the server addresses on startup.

## See Also

- [`docs/TOR_TRANSPORT.md`](TOR_TRANSPORT.md) — Tor transport details
- [`docs/I2P_TRANSPORT.md`](I2P_TRANSPORT.md) — I2P transport details
- [`docs/MULTINETWORK.md`](MULTINETWORK.md) — Multi-network transport system
- [`testnet/internal/bootstrap.go`](../testnet/internal/bootstrap.go) — Internal
  bootstrap server used by integration tests
