# Lokinet Manual Configuration Guide

This guide explains how to use toxcore-go with the Lokinet privacy network.

## Overview

toxcore-go supports outbound connections (Dial) to `.loki` addresses via Lokinet's SOCKS5 proxy interface. However, listening (hosting a Tox node reachable via a `.loki` address) requires manual SNApp (Service Node Application) configuration in Lokinet itself, as this cannot be done through SOCKS5 alone.

### Capability Summary

| Operation | Status | Notes |
|-----------|--------|-------|
| Dial (TCP) | ✅ Supported | Via SOCKS5 proxy |
| Listen (TCP) | ❌ Not Supported | Requires SNApp configuration |
| UDP | ❌ Not Supported | SOCKS5 does not support UDP |

## Prerequisites

1. **Lokinet installed and running** - Download from https://lokinet.org
2. **SOCKS5 proxy enabled** - Lokinet exposes SOCKS5 by default on `127.0.0.1:9050`

## Connecting to .loki Addresses (Dial)

### Basic Usage

```go
package main

import (
    "log"
    
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Create Lokinet transport (uses LOKINET_PROXY_ADDR or default 127.0.0.1:9050)
    loki := transport.NewLokinetTransport()
    defer loki.Close()
    
    // Connect to a .loki address
    conn, err := loki.Dial("example.loki:8080")
    if err != nil {
        log.Fatalf("Dial failed: %v", err)
    }
    defer conn.Close()
    
    // Use conn for communication...
}
```

### Custom Proxy Address

Set the `LOKINET_PROXY_ADDR` environment variable to use a non-default proxy:

```bash
export LOKINET_PROXY_ADDR="127.0.0.1:1055"
```

## Hosting a Tox Node on Lokinet (Advanced)

To make your Tox node reachable via a `.loki` address, you must configure a Lokinet SNApp manually.

### Step 1: Configure lokinet.ini

Edit your Lokinet configuration file (typically `~/.lokinet/lokinet.ini` on Linux):

```ini
[network]
# Enable inbound connections
inbound=true

# Configure a hidden service endpoint
[snapp]
# Your desired local keyfile (will be generated if it doesn't exist)
keyfile=/path/to/your/snapp.private
# Local TCP port to forward traffic to
bind-port=33445
```

### Step 2: Start Lokinet

Start or restart Lokinet to apply the configuration:

```bash
lokinet
```

### Step 3: Get Your .loki Address

Once Lokinet starts with the SNApp configuration, it will generate (or use existing) keys and print your `.loki` address. You can also find it by running:

```bash
lokinet-vpn --print-self
```

The address will look something like: `abc123def456.loki`

### Step 4: Configure Your Tox Application

Bind your Tox node to the local port that Lokinet is forwarding to:

```go
options := toxcore.NewOptions()
options.TCPPort = 33445  // Match the bind-port in lokinet.ini
options.UDPEnabled = false  // UDP not supported via Lokinet

tox, err := toxcore.New(options)
// ...
```

### Step 5: Share Your .loki Address

Other Lokinet users can now bootstrap to your node using:

```go
err := tox.Bootstrap("your-address.loki", 33445, "YOUR_PUBLIC_KEY_HEX")
```

## Limitations

1. **No UDP Support**: The Tox DHT requires UDP for optimal operation. Lokinet over SOCKS5 only supports TCP, which limits DHT functionality. Nodes behind Lokinet should use TCP relays for full connectivity.

2. **No Programmatic SNApp Creation**: Lokinet does not currently expose APIs for programmatic SNApp creation. Manual configuration is required.

3. **Key Management**: SNApp private keys are stored in the Lokinet keyfile. Ensure proper backup and security measures.

## Troubleshooting

### Connection Refused
- Verify Lokinet is running: `pgrep lokinet`
- Check the SOCKS5 port is listening: `ss -tlnp | grep 9050`

### Dial Timeout
- Ensure the target `.loki` address exists and is online
- Check Lokinet has successfully connected to the network

### SNApp Not Reachable
- Verify the SNApp configuration in `lokinet.ini`
- Check that your local service is listening on the configured port
- Review Lokinet logs for errors: `journalctl -u lokinet`

## References

- [Lokinet Documentation](https://docs.lokinet.dev)
- [Lokinet GitHub](https://github.com/oxen-io/lokinet)
- [SOCKS5 Protocol (RFC 1928)](https://datatracker.ietf.org/doc/html/rfc1928)
