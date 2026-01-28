package transport

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// IPTransport implements NetworkTransport for IPv4 and IPv6 networks.
// It handles both TCP and UDP connections over IP networks.
type IPTransport struct {
	mu sync.RWMutex
}

// NewIPTransport creates a new IP transport instance.
func NewIPTransport() *IPTransport {
	logrus.WithField("function", "NewIPTransport").Info("Creating IP transport")
	return &IPTransport{}
}

// Listen creates a listener on the given IP address.
// The address should be in the format "host:port" or ":port".
func (t *IPTransport) Listen(address string) (net.Listener, error) {
	logrus.WithFields(logrus.Fields{
		"function": "IPTransport.Listen",
		"address":  address,
	}).Debug("Creating TCP listener")

	// Default to TCP for stream-oriented listening
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "IPTransport.Listen",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to create TCP listener")
		return nil, fmt.Errorf("IP transport listen failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "IPTransport.Listen",
		"address":    address,
		"local_addr": listener.Addr().String(),
	}).Info("TCP listener created successfully")

	return listener, nil
}

// Dial establishes a TCP connection to the given IP address.
func (t *IPTransport) Dial(address string) (net.Conn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "IPTransport.Dial",
		"address":  address,
	}).Debug("Dialing TCP connection")

	conn, err := net.Dial("tcp", address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "IPTransport.Dial",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to dial TCP connection")
		return nil, fmt.Errorf("IP transport dial failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "IPTransport.Dial",
		"address":     address,
		"local_addr":  conn.LocalAddr().String(),
		"remote_addr": conn.RemoteAddr().String(),
	}).Info("TCP connection established")

	return conn, nil
}

// DialPacket creates a UDP packet connection to the given IP address.
func (t *IPTransport) DialPacket(address string) (net.PacketConn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "IPTransport.DialPacket",
		"address":  address,
	}).Debug("Creating UDP packet connection")

	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "IPTransport.DialPacket",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to create UDP packet connection")
		return nil, fmt.Errorf("IP transport dial packet failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "IPTransport.DialPacket",
		"address":    address,
		"local_addr": conn.LocalAddr().String(),
	}).Info("UDP packet connection created")

	return conn, nil
}

// SupportedNetworks returns the network types supported by IP transport.
func (t *IPTransport) SupportedNetworks() []string {
	return []string{"tcp", "udp", "tcp4", "tcp6", "udp4", "udp6"}
}

// Close closes the IP transport. Currently a no-op as IP transport is stateless.
func (t *IPTransport) Close() error {
	logrus.WithField("function", "IPTransport.Close").Debug("Closing IP transport")
	return nil
}

// TorTransport implements NetworkTransport for Tor .onion networks via SOCKS5 proxy.
// It connects to a Tor SOCKS5 proxy (default: 127.0.0.1:9050) to route traffic through Tor.
// The proxy address can be configured via TOR_PROXY_ADDR environment variable.
type TorTransport struct {
	mu          sync.RWMutex
	proxyAddr   string
	socksDialer proxy.Dialer
}

// NewTorTransport creates a new Tor transport instance.
// Uses TOR_PROXY_ADDR environment variable or defaults to 127.0.0.1:9050.
func NewTorTransport() *TorTransport {
	proxyAddr := os.Getenv("TOR_PROXY_ADDR")
	if proxyAddr == "" {
		proxyAddr = "127.0.0.1:9050" // Default Tor SOCKS5 port
	}

	logrus.WithFields(logrus.Fields{
		"function":   "NewTorTransport",
		"proxy_addr": proxyAddr,
	}).Info("Creating Tor transport")

	// Create SOCKS5 dialer for the Tor proxy
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "NewTorTransport",
			"proxy_addr": proxyAddr,
			"error":      err.Error(),
		}).Warn("Failed to create SOCKS5 dialer, will retry on Dial")
	}

	return &TorTransport{
		proxyAddr:   proxyAddr,
		socksDialer: dialer,
	}
}

// Listen creates a listener for Tor .onion addresses.
// Note: Creating onion services requires Tor control port access and is not
// supported via SOCKS5 alone. Applications should configure onion services
// via Tor configuration and use the regular IP transport to bind locally.
func (t *TorTransport) Listen(address string) (net.Listener, error) {
	logrus.WithFields(logrus.Fields{
		"function": "TorTransport.Listen",
		"address":  address,
	}).Debug("Tor listen requested")

	if !strings.Contains(address, ".onion") {
		return nil, fmt.Errorf("invalid Tor address format: %s (must contain .onion)", address)
	}

	// Tor onion service hosting requires Tor control port or configuration file setup.
	// SOCKS5 proxy only supports outbound connections to .onion addresses.
	return nil, fmt.Errorf("Tor onion service hosting not supported via SOCKS5 - configure via Tor control port or torrc")
}

// Dial establishes a connection through Tor to the given .onion address via SOCKS5.
// Supports both .onion addresses and regular addresses routed through Tor.
func (t *TorTransport) Dial(address string) (net.Conn, error) {
	t.mu.RLock()
	dialer := t.socksDialer
	proxyAddr := t.proxyAddr
	t.mu.RUnlock()

	logrus.WithFields(logrus.Fields{
		"function":   "TorTransport.Dial",
		"address":    address,
		"proxy_addr": proxyAddr,
	}).Debug("Tor dial requested")

	// Recreate dialer if it wasn't initialized during construction
	if dialer == nil {
		var err error
		dialer, err = proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":   "TorTransport.Dial",
				"proxy_addr": proxyAddr,
				"error":      err.Error(),
			}).Error("Failed to create SOCKS5 dialer")
			return nil, fmt.Errorf("Tor SOCKS5 dialer creation failed: %w", err)
		}

		t.mu.Lock()
		t.socksDialer = dialer
		t.mu.Unlock()
	}

	// Dial through SOCKS5 proxy
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "TorTransport.Dial",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to dial through Tor")
		return nil, fmt.Errorf("Tor dial failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "TorTransport.Dial",
		"address":     address,
		"local_addr":  conn.LocalAddr().String(),
		"remote_addr": conn.RemoteAddr().String(),
	}).Info("Tor connection established")

	return conn, nil
}

// DialPacket creates a packet connection through Tor.
// Currently returns an error as Tor transport primarily uses TCP.
func (t *TorTransport) DialPacket(address string) (net.PacketConn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "TorTransport.DialPacket",
		"address":  address,
	}).Debug("Tor packet dial requested")

	// Tor primarily uses TCP, UDP over Tor is complex
	return nil, fmt.Errorf("Tor UDP transport not supported")
}

// SupportedNetworks returns the network types supported by Tor transport.
func (t *TorTransport) SupportedNetworks() []string {
	return []string{"tor"}
}

// Close closes the Tor transport.
func (t *TorTransport) Close() error {
	logrus.WithField("function", "TorTransport.Close").Debug("Closing Tor transport")
	return nil
}

// I2PTransport implements NetworkTransport for I2P .b32.i2p networks.
// This is a placeholder implementation awaiting I2P library integration.
//
// IMPLEMENTATION PATH FOR I2P:
// 1. Use go-i2p library (github.com/go-i2p/go-i2p) or I2P SAM bridge
// 2. For SAM bridge: connect to I2P router's SAM port (default 7656)
// 3. Create I2P destinations (similar to onion addresses)
// 4. Implement streaming connections via SAM STREAM or native I2P protocol
// 5. Handle I2P-specific features: tunnels, leasesets, and garlic routing
//
// Alternative: Use SAMv3 protocol which is similar to SOCKS5 for basic connectivity
type I2PTransport struct {
	mu sync.RWMutex
}

// NewI2PTransport creates a new I2P transport instance.
func NewI2PTransport() *I2PTransport {
	logrus.WithField("function", "NewI2PTransport").Info("Creating I2P transport")
	return &I2PTransport{}
}

// Listen creates a listener for I2P .i2p addresses.
// I2P destination hosting requires SAM bridge or native I2P integration.
// See type documentation for implementation guidance.
func (t *I2PTransport) Listen(address string) (net.Listener, error) {
	logrus.WithFields(logrus.Fields{
		"function": "I2PTransport.Listen",
		"address":  address,
	}).Debug("I2P listen requested")

	if !strings.Contains(address, ".i2p") {
		return nil, fmt.Errorf("invalid I2P address format: %s (must contain .i2p)", address)
	}

	return nil, fmt.Errorf("I2P transport requires SAM bridge or go-i2p library integration - not yet implemented")
}

// Dial establishes a connection through I2P to the given .i2p address.
// I2P connectivity requires SAM bridge or native I2P integration.
// See type documentation for implementation guidance.
func (t *I2PTransport) Dial(address string) (net.Conn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "I2PTransport.Dial",
		"address":  address,
	}).Debug("I2P dial requested")

	if !strings.Contains(address, ".i2p") {
		return nil, fmt.Errorf("invalid I2P address format: %s (must contain .i2p)", address)
	}

	return nil, fmt.Errorf("I2P transport requires SAM bridge or go-i2p library integration - not yet implemented")
}

// DialPacket creates a packet connection through I2P.
// I2P datagram support requires SAM bridge or native I2P integration.
// See type documentation for implementation guidance.
func (t *I2PTransport) DialPacket(address string) (net.PacketConn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "I2PTransport.DialPacket",
		"address":  address,
	}).Debug("I2P packet dial requested")

	if !strings.Contains(address, ".i2p") {
		return nil, fmt.Errorf("invalid I2P address format: %s (must contain .i2p)", address)
	}

	return nil, fmt.Errorf("I2P transport requires SAM bridge or go-i2p library integration - not yet implemented")
}

// SupportedNetworks returns the network types supported by I2P transport.
func (t *I2PTransport) SupportedNetworks() []string {
	return []string{"i2p"}
}

// Close closes the I2P transport.
func (t *I2PTransport) Close() error {
	logrus.WithField("function", "I2PTransport.Close").Debug("Closing I2P transport")
	return nil
}

// NymTransport implements NetworkTransport for Nym mixnet networks.
// This is a placeholder implementation awaiting Nym SDK integration.
//
// IMPLEMENTATION PATH FOR NYM:
// 1. Use Nym SDK or websocket client to connect to Nym mixnet
// 2. Nym uses websocket protocol for client connections (default port 1977)
// 3. Implement SURB (Single Use Reply Block) handling for bidirectional comms
// 4. Handle Nym-specific addressing: recipient addresses are Nym client IDs
// 5. Manage mixnet delays and message padding for traffic analysis resistance
//
// Note: Nym provides stronger anonymity than Tor through mixnet delays and cover traffic,
// but has higher latency. Best suited for async messaging rather than real-time calls.
type NymTransport struct {
	mu sync.RWMutex
}

// NewNymTransport creates a new Nym transport instance.
func NewNymTransport() *NymTransport {
	logrus.WithField("function", "NewNymTransport").Info("Creating Nym transport")
	return &NymTransport{}
}

// Listen creates a listener for Nym addresses.
// Nym mixnet requires websocket client SDK integration for bidirectional communication.
// See type documentation for implementation guidance.
func (t *NymTransport) Listen(address string) (net.Listener, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NymTransport.Listen",
		"address":  address,
	}).Debug("Nym listen requested")

	if !strings.Contains(address, ".nym") {
		return nil, fmt.Errorf("invalid Nym address format: %s (must contain .nym)", address)
	}

	return nil, fmt.Errorf("Nym transport requires Nym SDK websocket client integration - not yet implemented")
}

// Dial establishes a connection through Nym mixnet to the given address.
// Nym mixnet requires websocket client SDK integration.
// See type documentation for implementation guidance.
func (t *NymTransport) Dial(address string) (net.Conn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NymTransport.Dial",
		"address":  address,
	}).Debug("Nym dial requested")

	if !strings.Contains(address, ".nym") {
		return nil, fmt.Errorf("invalid Nym address format: %s (must contain .nym)", address)
	}

	return nil, fmt.Errorf("Nym transport requires Nym SDK websocket client integration - not yet implemented")
}

// DialPacket creates a packet connection through Nym mixnet.
// Currently returns an error as Nym integration is not implemented.
func (t *NymTransport) DialPacket(address string) (net.PacketConn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NymTransport.DialPacket",
		"address":  address,
	}).Debug("Nym packet dial requested")

	if !strings.Contains(address, ".nym") {
		return nil, fmt.Errorf("invalid Nym address format: %s (must contain .nym)", address)
	}

	return nil, fmt.Errorf("Nym transport requires Nym SDK websocket client integration - not yet implemented")
	return nil, fmt.Errorf("Nym transport not yet implemented")
}

// SupportedNetworks returns the network types supported by Nym transport.
func (t *NymTransport) SupportedNetworks() []string {
	return []string{"nym"}
}

// Close closes the Nym transport.
func (t *NymTransport) Close() error {
	logrus.WithField("function", "NymTransport.Close").Debug("Closing Nym transport")
	return nil
}
