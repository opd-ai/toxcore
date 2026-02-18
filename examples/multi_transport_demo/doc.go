// Package main provides a demonstration of the toxcore multi-protocol transport
// layer introduced in Phase 4.1.
//
// This example showcases the [transport.MultiTransport] system which provides
// a unified interface for multiple network protocols including:
//
//   - IP (TCP/UDP) - Standard Internet Protocol transport, fully implemented
//   - Tor - .onion address transport via SOCKS5 proxy
//   - I2P - .b32.i2p address transport via SAM bridge
//   - Nym - .nym address transport for mixnet anonymity
//
// # Running the Demo
//
// From the repository root:
//
//	go run ./examples/multi_transport_demo
//
// # What This Demo Shows
//
// The demo demonstrates three key aspects of the multi-transport system:
//
// 1. **Transport Selection** - How MultiTransport automatically selects the
// appropriate transport based on the address format (e.g., .onion → Tor,
// .b32.i2p → I2P).
//
// 2. **Working IP Transport** - A complete TCP echo server/client example
// showing listener creation, connection establishment, and bidirectional
// data transfer.
//
// 3. **Direct Transport Access** - How to retrieve specific transport
// implementations and register custom transports.
//
// # Prerequisites
//
// For IP transport: None required (works out of the box)
//
// For Tor transport: Running Tor daemon with SOCKS5 proxy on localhost:9050
//
// For I2P transport: Running I2P router with SAM bridge on localhost:7656
//
// For Nym transport: Not yet implemented (placeholder)
//
// # Example Output
//
//	=== Multi-Transport Demo ===
//	Demonstrating Phase 4.1: Multi-Protocol Transport Layer
//
//	Supported Networks:
//	  - ip
//	  - tor
//	  - i2p
//	  - nym
//
//	Transport Selection Examples:
//	Address: 127.0.0.1:8080
//	  Result: Listener created at 127.0.0.1:8080
//	...
//
// # Architecture
//
// The [transport.MultiTransport] type wraps multiple [transport.Transport]
// implementations, providing automatic routing based on address patterns:
//
//	MultiTransport
//	├── IPTransport     → handles IP addresses and hostnames
//	├── TorTransport    → handles .onion addresses
//	├── I2PTransport    → handles .b32.i2p addresses
//	└── NymTransport    → handles .nym addresses
//
// Each transport implements the standard [net.Listener], [net.Conn], and
// [net.PacketConn] interfaces for seamless integration with Go's networking
// ecosystem.
//
// # See Also
//
//   - [transport.MultiTransport] - Main multi-transport implementation
//   - [transport.Transport] - Interface implemented by each transport type
//   - examples/privacy_networks - Detailed privacy network transport examples
package main
