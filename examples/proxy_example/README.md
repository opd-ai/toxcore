# Proxy Example

This example demonstrates how to configure toxcore-go to route all network traffic through a SOCKS5 proxy, such as Tor.

## Features Demonstrated

- SOCKS5 proxy configuration
- Optional proxy authentication (username/password)
- Network traffic routing through proxy
- Bootstrap through proxy

## Prerequisites

- A SOCKS5 proxy server (e.g., Tor)
- For Tor:
  ```bash
  # Install Tor (Debian/Ubuntu)
  sudo apt-get install tor
  
  # Start Tor service
  sudo systemctl start tor
  # Or run Tor in foreground
  tor
  ```

## Building

```bash
go build
```

## Running

```bash
# Make sure your SOCKS5 proxy is running first
# For Tor, ensure it's running on default port 9050
./proxy_example
```

## Configuration

The example configures a SOCKS5 proxy with the following settings:

```go
options.Proxy = &toxcore.ProxyOptions{
    Type:     toxcore.ProxyTypeSOCKS5,
    Host:     "127.0.0.1",
    Port:     9050,          // Default Tor SOCKS5 port
    Username: "",            // Optional authentication
    Password: "",            // Optional authentication
}
```

### Supported Proxy Types

- **SOCKS5**: Full support with optional authentication
- **HTTP**: Not yet implemented (use SOCKS5 instead)

### Tor Configuration

By default, Tor runs a SOCKS5 proxy on `127.0.0.1:9050`. No additional configuration needed.

To verify Tor is running:
```bash
curl --socks5 localhost:9050 https://check.torproject.org
```

## Privacy Considerations

When using toxcore-go with Tor:

1. **Anonymity**: All Tox network traffic is routed through Tor, hiding your IP address
2. **Performance**: Expect higher latency due to Tor's onion routing
3. **Compatibility**: Works with both UDP and TCP transports
4. **Persistence**: Proxy settings are runtime configuration (not persisted in savedata)

## Advanced Usage

### Custom SOCKS5 Proxy

```go
options.Proxy = &toxcore.ProxyOptions{
    Type:     toxcore.ProxyTypeSOCKS5,
    Host:     "proxy.example.com",
    Port:     1080,
    Username: "myuser",
    Password: "mypass",
}
```

### Disabling Proxy

```go
// Don't set Proxy field, or set Type to ProxyTypeNone
options.Proxy = &toxcore.ProxyOptions{
    Type: toxcore.ProxyTypeNone,
}
```

## Troubleshooting

**Connection failures:**
- Ensure your SOCKS5 proxy is running
- Check proxy host/port configuration
- Verify proxy authentication credentials
- Test proxy connectivity: `curl --socks5 localhost:9050 https://example.com`

**Bootstrap failures:**
- Normal when proxy isn't running
- Check Tor logs: `sudo journalctl -u tor`
- Try different bootstrap nodes

## See Also

- [Tor Project](https://www.torproject.org/)
- [toxcore-go Documentation](../../README.md)
- [SOCKS5 Protocol](https://tools.ietf.org/html/rfc1928)
