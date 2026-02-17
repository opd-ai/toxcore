// Package transport provides network transport implementations for the Tox
// protocol, enabling secure peer-to-peer communication across multiple network
// types including UDP, TCP, Tor, I2P, Nym, and Lokinet.
//
// # Architecture
//
// The transport layer abstracts network I/O for the DHT, friend system, async
// messaging, and all packet routing. It follows Go's interface-based design
// with net.Addr, net.Conn, net.PacketConn, and net.Listener used throughout
// (no concrete types like *net.UDPAddr).
//
// The core abstraction is the Transport interface which all implementations
// satisfy:
//
//	type Transport interface {
//	    Send(packet *Packet, addr net.Addr) error
//	    Close() error
//	    LocalAddr() net.Addr
//	    RegisterHandler(packetType PacketType, handler PacketHandler)
//	    IsConnectionOriented() bool
//	}
//
// # Transport Implementations
//
// UDP Transport:
//
//	transport, err := NewUDPTransport(":33445")
//	// Connectionless, low-latency, suitable for most Tox traffic
//
// TCP Transport:
//
//	transport, err := NewTCPTransport(":33445")
//	// Connection-oriented, reliable delivery, NAT traversal support
//
// Noise Transport (encrypted wrapper):
//
//	noiseTransport := NewNoiseTransport(underlying, keypair, nil)
//	// Wraps UDP/TCP with Noise-IK encryption for forward secrecy
//
// Proxy Transport (SOCKS5/HTTP):
//
//	config := &ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 9050}
//	proxyTransport, err := NewProxyTransport(underlying, config)
//	// Routes traffic through Tor, I2P, or other proxy services
//
// # Noise Protocol Integration
//
// The Noise-IK pattern provides mutual authentication, forward secrecy, and
// resistance to Key Compromise Impersonation (KCI) attacks. The NoiseTransport
// automatically handles:
//
//   - Handshake negotiation with replay protection via nonce tracking
//   - Timestamp freshness validation (HandshakeMaxAge = 5 minutes)
//   - Session lifecycle with idle timeout cleanup (SessionIdleTimeout = 5 minutes)
//   - Transparent encryption/decryption of all packet types except handshakes
//
// # Multi-Network Support
//
// The NetworkTransport interface enables routing over alternative networks:
//
//   - Tor (.onion addresses) - Anonymous routing via onion service
//   - I2P (.i2p addresses) - Invisible Internet Project garlic routing
//   - Nym (.nym addresses) - Mixnet for traffic analysis resistance
//   - Lokinet (.loki addresses) - Oxen network's onion routing
//
// Network capability detection is handled by NetworkDetector which determines
// available routing methods without relying on IP address parsing.
//
// # NAT Traversal
//
// The package includes comprehensive NAT traversal support:
//
//   - STUN client for discovering external addresses (RFC 5389)
//   - UPnP client for automatic port mapping on compatible routers
//   - NAT-PMP/PCP support for Apple and other devices
//   - UDP hole punching with persistent keepalive
//   - Advanced NAT detection with relay fallback for symmetric NAT
//
// # Version Negotiation
//
// Protocol version negotiation ensures backward compatibility as the Tox
// protocol evolves. The VersionedHandshakeManager coordinates version
// discovery and capability exchange during connection establishment.
//
// # Packet Types
//
// All Tox packet types are defined in packet.go with type-safe constants:
//
//	const (
//	    PacketPingRequest     PacketType = 0
//	    PacketPingResponse    PacketType = 1
//	    PacketFriendRequest   PacketType = 5
//	    PacketNoiseHandshake  PacketType = 250
//	    PacketNoiseMessage    PacketType = 251
//	    // ... see packet.go for complete list
//	)
//
// # Handler Registration
//
// Packet handlers are registered per-type for efficient dispatch:
//
//	transport.RegisterHandler(PacketPingRequest, func(p *Packet, addr net.Addr) error {
//	    // Handle ping request
//	    return nil
//	})
//
// # Thread Safety
//
// All transport implementations use sync.RWMutex for concurrent access safety.
// Session state, handler maps, and client connections are protected from data
// races. The package passes Go's race detector validation.
//
// # Error Handling
//
// All errors are wrapped with context using fmt.Errorf and logged with
// structured fields via logrus.WithFields. Sentinel errors are provided for
// common failure modes:
//
//	var (
//	    ErrNoiseNotSupported   // Peer doesn't support Noise protocol
//	    ErrNoiseSessionNotFound // No active session with peer
//	    ErrHandshakeReplay     // Replay attack detected
//	    ErrHandshakeTooOld     // Handshake timestamp expired
//	)
package transport
