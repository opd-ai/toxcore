# Privacy Network Transport Examples

This example demonstrates how to use toxcore-go's privacy network transports to connect through Tor, I2P, and Lokinet networks.

## Supported Privacy Networks

### 1. Tor (The Onion Router)
- **Address format:** `*.onion:port`
- **Transport:** SOCKS5 proxy
- **Default proxy:** `127.0.0.1:9050`
- **Environment variable:** `TOR_PROXY_ADDR`
- **Status:** ✅ Fully functional

### 2. I2P (Invisible Internet Project)
- **Address format:** `*.i2p:port` or `*.b32.i2p:port`
- **Transport:** SAM bridge (v3)
- **Default SAM address:** `127.0.0.1:7656`
- **Environment variable:** `I2P_SAM_ADDR`
- **Status:** ✅ Fully functional

### 3. Lokinet
- **Address format:** `*.loki:port`
- **Transport:** SOCKS5 proxy
- **Default proxy:** `127.0.0.1:9050`
- **Environment variable:** `LOKINET_PROXY_ADDR`
- **Status:** ✅ Fully functional

## Prerequisites

Before running this example, you need to have the appropriate privacy network daemon running:

### Tor
Install and start Tor daemon:
```bash
# Ubuntu/Debian
sudo apt install tor
sudo systemctl start tor

# macOS (Homebrew)
brew install tor
brew services start tor

# Or use Tor Browser which includes the SOCKS5 proxy
```

### I2P
Install and start I2P router:
```bash
# Download from https://geti2p.net/
# Start the I2P router and enable SAM bridge in config
# SAM bridge must be listening on 127.0.0.1:7656
```

### Lokinet
Install and start Lokinet daemon:
```bash
# Ubuntu/Debian
sudo curl -so /etc/apt/trusted.gpg.d/oxen.gpg https://deb.oxen.io/pub.gpg
echo "deb https://deb.oxen.io $(lsb_release -sc) main" | sudo tee /etc/apt/sources.list.d/oxen.list
sudo apt update
sudo apt install lokinet
sudo systemctl start lokinet
```

## Building and Running

```bash
# Build the example
go build -o privacy_networks main.go

# Run the example
./privacy_networks
```

## Example Output

When all three privacy network daemons are running, you should see output like:

```
Privacy Network Transport Examples
====================================

1. Tor Transport (.onion addresses)
-----------------------------------
Supported networks: [tor]
Attempting to connect to: 3g2upl4pq6kufc4m.onion:80
✓ Successfully connected through Tor!
  Local address:  127.0.0.1:xxxxx
  Remote address: 3g2upl4pq6kufc4m.onion:80

Custom Tor proxy can be configured via TOR_PROXY_ADDR environment variable
Current proxy address: (would use 127.0.0.1:9150 if configured)

2. I2P Transport (.i2p addresses)
----------------------------------
Supported networks: [i2p]
Attempting to connect to: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80
✓ Successfully connected through I2P!
  Local address:  [I2P destination]
  Remote address: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80

Custom I2P SAM address can be configured via I2P_SAM_ADDR environment variable
Default SAM address: 127.0.0.1:7656

3. Lokinet Transport (.loki addresses)
--------------------------------------
Supported networks: [loki lokinet]
Attempting to connect to: example.loki:80
✓ Successfully connected through Lokinet!
  Local address:  127.0.0.1:xxxxx
  Remote address: example.loki:80

Custom Lokinet proxy can be configured via LOKINET_PROXY_ADDR environment variable
Default proxy address: 127.0.0.1:9050
```

If daemons are not running, you'll see connection errors (which is expected).

## Using in Your Application

```go
package main

import (
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Create privacy network transports
    tor := transport.NewTorTransport()
    i2p := transport.NewI2PTransport()
    lokinet := transport.NewLokinetTransport()
    
    defer tor.Close()
    defer i2p.Close()
    defer lokinet.Close()
    
    // Connect through Tor
    conn, err := tor.Dial("example.onion:80")
    if err != nil {
        // Handle error
    }
    defer conn.Close()
    
    // Use the connection like any net.Conn
    // ...
}
```

## Features and Limitations

### Tor Transport
- ✅ TCP connections through SOCKS5
- ✅ .onion addresses and regular domains
- ✅ Configurable proxy address
- ❌ UDP not supported (SOCKS5 limitation)
- ❌ Hosting .onion services requires Tor control port

### I2P Transport
- ✅ TCP connections through SAM bridge
- ✅ .i2p and .b32.i2p addresses
- ✅ Automatic ephemeral destination creation
- ✅ Configurable SAM bridge address
- ❌ UDP datagrams not yet implemented
- ❌ Hosting .i2p services not supported (requires persistent destinations)

### Lokinet Transport
- ✅ TCP connections through SOCKS5
- ✅ .loki addresses and regular domains
- ✅ Configurable proxy address
- ❌ UDP not supported (SOCKS5 limitation)
- ❌ Hosting .loki services requires SNApp configuration

## Security Considerations

- Always verify that your privacy network daemon is running and properly configured
- These transports route ALL traffic through the privacy network, providing anonymity
- Connection latency will be higher than direct IP connections
- I2P has the highest latency due to garlic routing
- Tor provides good balance of speed and anonymity
- Lokinet offers low-latency onion routing

## Further Reading

- Tor Project: https://www.torproject.org/
- I2P Network: https://geti2p.net/
- Lokinet: https://lokinet.org/
- SAM Protocol: https://geti2p.net/en/docs/api/samv3
