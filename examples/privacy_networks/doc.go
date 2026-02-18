// Package main provides a demonstration of privacy network transports (Tor, I2P, Lokinet)
// using the toxcore transport layer.
//
// This example showcases how to create and use privacy-preserving network transports
// for anonymous communication. Each transport type supports different overlay networks:
//
//   - Tor: Routes traffic through the Tor network for .onion addresses
//   - I2P: Uses the Invisible Internet Project network for .b32.i2p addresses
//   - Lokinet: Uses the Loki Service Node network for .loki addresses
//
// # Prerequisites
//
// To successfully connect through any of these transports, you need the corresponding
// network daemon running locally:
//
//   - Tor: Run Tor Browser or Tor daemon with SOCKS5 proxy (default: 127.0.0.1:9050)
//   - I2P: Run I2P router with SAM bridge enabled (default: 127.0.0.1:7656)
//   - Lokinet: Run Lokinet daemon with SOCKS5 proxy (default: 127.0.0.1:9050)
//
// # Environment Variables
//
// Custom proxy addresses can be configured via environment variables:
//
//   - TOR_PROXY_ADDR: Custom Tor SOCKS5 proxy address
//   - I2P_SAM_ADDR: Custom I2P SAM bridge address
//   - LOKINET_PROXY_ADDR: Custom Lokinet SOCKS5 proxy address
//
// # Usage
//
// Run the example to see connection attempts to each privacy network:
//
//	go run ./examples/privacy_networks/
//
// Expected behavior: Connection attempts will fail gracefully if the corresponding
// network daemon is not running, demonstrating proper error handling patterns.
package main
