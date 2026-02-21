package transport

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/go-i2p/onramp"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// ErrNymNotImplemented is returned when attempting to use NymTransport functionality.
// Nym mixnet integration requires the Nym SDK websocket client which is not yet implemented.
// See NymTransport documentation for implementation guidance.
var ErrNymNotImplemented = errors.New("nym transport not implemented: requires Nym SDK websocket client integration")

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

// TorTransport implements NetworkTransport for Tor .onion networks via onramp library.
// Onramp wraps Tor controller with automatic lifecycle management including key persistence,
// service registration, and signal-based cleanup.
//
// IMPLEMENTATION STATUS:
//   - Dial(): Fully implemented via onramp Onion instance. Can connect to .onion addresses
//     and regular addresses (which will be routed through Tor network).
//   - Listen(): Fully implemented via onramp Onion instance. Creates a persistent v3 onion
//     service with automatic key management (keys stored in onionkeys/ directory).
//   - DialPacket(): Not supported. Tor primarily uses TCP; UDP over Tor is experimental.
//
// PREREQUISITES: Tor daemon must be running with control port enabled.
// Configure control port via TOR_CONTROL_ADDR environment variable (default: 127.0.0.1:9051).
// If TOR_CONTROL_ADDR is not set but TOR_PROXY_ADDR is set, falls back to SOCKS5 dialing only.
//
// USAGE EXAMPLE:
//
//	tor := transport.NewTorTransport()
//	defer tor.Close()
//
//	// Host a Tor hidden service
//	listener, err := tor.Listen("myservice.onion:80")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Connect to a Tor hidden service
//	conn, err := tor.Dial("example.onion:8080")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use conn for communication...
//
// See also: https://2019.www.torproject.org/docs/documentation for Tor documentation.
type TorTransport struct {
	mu          sync.RWMutex
	controlAddr string
	onion       *onramp.Onion
}

// NewTorTransport creates a new Tor transport instance.
// Uses TOR_CONTROL_ADDR environment variable or defaults to 127.0.0.1:9051.
// The onramp Onion instance is lazily initialized on first use of Dial/Listen.
func NewTorTransport() *TorTransport {
	controlAddr := os.Getenv("TOR_CONTROL_ADDR")
	if controlAddr == "" {
		controlAddr = onramp.TOR_CONTROL // Default Tor control port (127.0.0.1:9051)
	}

	logrus.WithFields(logrus.Fields{
		"function":     "NewTorTransport",
		"control_addr": controlAddr,
	}).Info("Creating Tor transport (onramp)")

	return &TorTransport{
		controlAddr: controlAddr,
	}
}

// ensureOnion lazily initializes the onramp Onion instance.
func (t *TorTransport) ensureOnion() error {
	if t.onion != nil {
		return nil
	}

	onion, err := onramp.NewOnion("toxcore-tor", t.controlAddr, onramp.OPT_DEFAULTS)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":     "TorTransport.ensureOnion",
			"control_addr": t.controlAddr,
			"error":        err.Error(),
		}).Error("Failed to create onramp Onion instance")
		return fmt.Errorf("Tor onramp initialization failed: %w", err)
	}

	t.onion = onion
	return nil
}

// Listen creates a listener for Tor .onion addresses via onramp.
// The Onion instance handles key persistence and service registration automatically.
// The first call may take 30-90 seconds for initial descriptor publishing.
func (t *TorTransport) Listen(address string) (net.Listener, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "TorTransport.Listen",
		"address":  address,
	}).Debug("Tor listen requested")

	if !strings.Contains(address, ".onion") {
		return nil, fmt.Errorf("invalid Tor address format: %s (must contain .onion)", address)
	}

	if err := t.ensureOnion(); err != nil {
		return nil, fmt.Errorf("Tor listen failed: %w", err)
	}

	listener, err := t.onion.Listen()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "TorTransport.Listen",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to create Tor listener")
		return nil, fmt.Errorf("Tor listener creation failed: %w", err)
	}

	// Get the actual onion address
	onionAddr, err := t.onion.Addr()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "TorTransport.Listen",
			"error":    err.Error(),
		}).Warn("Could not retrieve onion address")
	}

	logrus.WithFields(logrus.Fields{
		"function":   "TorTransport.Listen",
		"address":    address,
		"onion_addr": onionAddr,
		"local_addr": listener.Addr().String(),
	}).Info("Tor listener created successfully")

	return listener, nil
}

// Dial establishes a connection through Tor to the given .onion address via onramp.
// Supports both .onion addresses and regular addresses routed through Tor.
// The Onion instance reuses the same Tor circuits for all dials.
func (t *TorTransport) Dial(address string) (net.Conn, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":     "TorTransport.Dial",
		"address":      address,
		"control_addr": t.controlAddr,
	}).Debug("Tor dial requested")

	if err := t.ensureOnion(); err != nil {
		return nil, fmt.Errorf("Tor dial failed: %w", err)
	}

	conn, err := t.onion.Dial("tcp", address)
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

	// Tor primarily uses TCP, UDP over Tor is experimental
	return nil, fmt.Errorf("Tor UDP transport not supported")
}

// SupportedNetworks returns the network types supported by Tor transport.
func (t *TorTransport) SupportedNetworks() []string {
	return []string{"tor"}
}

// Close closes the Tor transport and the underlying onramp Onion instance.
func (t *TorTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithField("function", "TorTransport.Close").Debug("Closing Tor transport")

	if t.onion != nil {
		err := t.onion.Close()
		t.onion = nil
		return err
	}

	return nil
}

// I2PTransport implements NetworkTransport for I2P .b32.i2p networks via the onramp library.
// Onramp wraps the SAM bridge protocol with automatic lifecycle management including
// key persistence, session multiplexing, and signal-based cleanup.
//
// The SAM bridge address can be configured via I2P_SAM_ADDR environment variable.
//
// IMPLEMENTATION STATUS:
//   - Dial(): Fully implemented via onramp Garlic instance. Can connect to .i2p destinations.
//   - Listen(): Fully implemented via onramp Garlic instance. Creates a persistent I2P
//     destination with automatic key management (keys stored in i2pkeys/ directory).
//   - DialPacket(): Fully implemented via onramp Garlic instance using I2P datagrams.
//
// PREREQUISITES: An I2P router (i2pd or Java I2P) must be running with SAM enabled.
// If no SAM bridge is detected on port 7656, onramp can auto-start an embedded SAM bridge.
// Configure SAM port via I2P_SAM_ADDR environment variable (default: 127.0.0.1:7656).
//
// USAGE EXAMPLE:
//
//	i2p := transport.NewI2PTransport()
//	defer i2p.Close()
//
//	// Listen for incoming I2P connections
//	listener, err := i2p.Listen("toxcore.b32.i2p")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Connect to an I2P destination
//	conn, err := i2p.Dial("example.b32.i2p:8080")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use conn for communication...
//
// See also: https://geti2p.net/en/docs/api/samv3 for SAM protocol documentation.
type I2PTransport struct {
	mu      sync.RWMutex
	samAddr string
	garlic  *onramp.Garlic
}

// NewI2PTransport creates a new I2P transport instance.
// Uses I2P_SAM_ADDR environment variable or defaults to 127.0.0.1:7656.
// The onramp Garlic instance is lazily initialized on first use of Dial/Listen/DialPacket.
func NewI2PTransport() *I2PTransport {
	samAddr := os.Getenv("I2P_SAM_ADDR")
	if samAddr == "" {
		samAddr = onramp.SAM_ADDR // Default I2P SAM port (127.0.0.1:7656)
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewI2PTransport",
		"sam_addr": samAddr,
	}).Info("Creating I2P transport (onramp)")

	return &I2PTransport{
		samAddr: samAddr,
	}
}

// ensureGarlic lazily initializes the onramp Garlic instance.
func (t *I2PTransport) ensureGarlic() error {
	if t.garlic != nil {
		return nil
	}

	garlic, err := onramp.NewGarlic("toxcore-i2p", t.samAddr, onramp.OPT_SMALL)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "I2PTransport.ensureGarlic",
			"sam_addr": t.samAddr,
			"error":    err.Error(),
		}).Error("Failed to create onramp Garlic instance")
		return fmt.Errorf("I2P onramp initialization failed: %w", err)
	}

	t.garlic = garlic
	return nil
}

// Listen creates a listener for I2P .i2p addresses via onramp.
// The Garlic instance handles key persistence and tunnel setup automatically.
// The first call may take 2-5 minutes for initial I2P tunnel establishment.
func (t *I2PTransport) Listen(address string) (net.Listener, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "I2PTransport.Listen",
		"address":  address,
	}).Debug("I2P listen requested")

	if !strings.Contains(address, ".i2p") {
		return nil, fmt.Errorf("invalid I2P address format: %s (must contain .i2p)", address)
	}

	if err := t.ensureGarlic(); err != nil {
		return nil, fmt.Errorf("I2P listen failed: %w", err)
	}

	listener, err := t.garlic.Listen()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "I2PTransport.Listen",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to create I2P listener")
		return nil, fmt.Errorf("I2P listener creation failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "I2PTransport.Listen",
		"address":    address,
		"local_addr": listener.Addr().String(),
	}).Info("I2P listener created successfully")

	return listener, nil
}

// Dial establishes a connection through I2P to the given .i2p address via onramp.
// Supports both .b32.i2p addresses and regular .i2p addresses.
// The Garlic instance reuses the same PRIMARY session tunnels for all dials.
func (t *I2PTransport) Dial(address string) (net.Conn, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "I2PTransport.Dial",
		"address":  address,
		"sam_addr": t.samAddr,
	}).Debug("I2P dial requested")

	if err := t.validateI2PAddress(address); err != nil {
		return nil, err
	}

	if err := t.ensureGarlic(); err != nil {
		return nil, fmt.Errorf("I2P dial failed: %w", err)
	}

	conn, err := t.garlic.Dial("tcp", address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "I2PTransport.Dial",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to dial through I2P")
		return nil, fmt.Errorf("I2P dial failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "I2PTransport.Dial",
		"address":     address,
		"local_addr":  conn.LocalAddr().String(),
		"remote_addr": conn.RemoteAddr().String(),
	}).Info("I2P connection established")

	return conn, nil
}

// validateI2PAddress checks if the address is a valid I2P format.
func (t *I2PTransport) validateI2PAddress(address string) error {
	if !strings.Contains(address, ".i2p") {
		return fmt.Errorf("invalid I2P address format: %s (must contain .i2p)", address)
	}
	return nil
}

// DialPacket creates a packet connection through I2P via onramp.
// Uses onramp's ListenPacket() which provides I2P datagram support.
func (t *I2PTransport) DialPacket(address string) (net.PacketConn, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "I2PTransport.DialPacket",
		"address":  address,
	}).Debug("I2P packet dial requested")

	if !strings.Contains(address, ".i2p") {
		return nil, fmt.Errorf("invalid I2P address format: %s (must contain .i2p)", address)
	}

	if err := t.ensureGarlic(); err != nil {
		return nil, fmt.Errorf("I2P datagram failed: %w", err)
	}

	conn, err := t.garlic.ListenPacket()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "I2PTransport.DialPacket",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to create I2P datagram connection")
		return nil, fmt.Errorf("I2P datagram creation failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "I2PTransport.DialPacket",
		"address":    address,
		"local_addr": conn.LocalAddr().String(),
	}).Info("I2P datagram connection created")

	return conn, nil
}

// SupportedNetworks returns the network types supported by I2P transport.
func (t *I2PTransport) SupportedNetworks() []string {
	return []string{"i2p"}
}

// Close closes the I2P transport and the underlying onramp Garlic instance.
func (t *I2PTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithField("function", "I2PTransport.Close").Debug("Closing I2P transport")

	if t.garlic != nil {
		err := t.garlic.Close()
		t.garlic = nil
		return err
	}

	return nil
}

// NymTransport implements NetworkTransport for Nym mixnet networks.
//
// STATUS: NOT IMPLEMENTED - EXPERIMENTAL PLACEHOLDER
//
// This is a placeholder implementation awaiting Nym SDK integration.
// All methods return ErrNymNotImplemented until Nym SDK support is added.
// Callers should check for ErrNymNotImplemented using errors.Is().
//
// IMPLEMENTATION STATUS:
//   - Dial(): Not implemented. Returns ErrNymNotImplemented.
//   - Listen(): Not implemented. Returns ErrNymNotImplemented.
//   - DialPacket(): Not implemented. Returns ErrNymNotImplemented.
//
// IMPLEMENTATION PATH FOR NYM:
//  1. Use Nym SDK or websocket client to connect to Nym mixnet
//  2. Nym uses websocket protocol for client connections (default port 1977)
//  3. Implement SURB (Single Use Reply Block) handling for bidirectional comms
//  4. Handle Nym-specific addressing: recipient addresses are Nym client IDs
//  5. Manage mixnet delays and message padding for traffic analysis resistance
//
// Note: Nym provides stronger anonymity than Tor through mixnet delays and cover traffic,
// but has higher latency. Best suited for async messaging rather than real-time calls.
//
// See also: https://nymtech.net/docs for Nym protocol documentation.
type NymTransport struct {
	mu sync.RWMutex
}

// NewNymTransport creates a new Nym transport instance.
// Note: The returned transport is a placeholder; all methods return ErrNymNotImplemented.
func NewNymTransport() *NymTransport {
	logrus.WithField("function", "NewNymTransport").Warn("Creating Nym transport (NOT IMPLEMENTED - experimental placeholder)")
	return &NymTransport{}
}

// Listen creates a listener for Nym addresses.
// Returns ErrNymNotImplemented as Nym SDK integration is not yet available.
func (t *NymTransport) Listen(address string) (net.Listener, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NymTransport.Listen",
		"address":  address,
	}).Debug("Nym listen requested")

	if !strings.Contains(address, ".nym") {
		return nil, fmt.Errorf("invalid Nym address format: %s (must contain .nym)", address)
	}

	return nil, ErrNymNotImplemented
}

// Dial establishes a connection through Nym mixnet to the given address.
// Returns ErrNymNotImplemented as Nym SDK integration is not yet available.
func (t *NymTransport) Dial(address string) (net.Conn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NymTransport.Dial",
		"address":  address,
	}).Debug("Nym dial requested")

	if !strings.Contains(address, ".nym") {
		return nil, fmt.Errorf("invalid Nym address format: %s (must contain .nym)", address)
	}

	return nil, ErrNymNotImplemented
}

// DialPacket creates a packet connection through Nym mixnet.
// Returns ErrNymNotImplemented as Nym SDK integration is not yet available.
func (t *NymTransport) DialPacket(address string) (net.PacketConn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NymTransport.DialPacket",
		"address":  address,
	}).Debug("Nym packet dial requested")

	if !strings.Contains(address, ".nym") {
		return nil, fmt.Errorf("invalid Nym address format: %s (must contain .nym)", address)
	}

	return nil, ErrNymNotImplemented
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

// LokinetTransport implements NetworkTransport for Lokinet .loki networks via SOCKS5 proxy.
// Lokinet provides onion routing similar to Tor and supports SOCKS5 proxy interface.
// The proxy address can be configured via LOKINET_PROXY_ADDR environment variable.
//
// IMPLEMENTATION STATUS:
//   - Dial(): Fully implemented via SOCKS5 proxy. Can connect to .loki addresses.
//   - Listen(): Not supported. Lokinet SNApp (Service Node Application) hosting requires
//     configuration via lokinet.ini and cannot be done through SOCKS5 proxy alone.
//     Applications requiring Lokinet listener functionality should configure a SNApp
//     via the Lokinet configuration file.
//   - DialPacket(): Not supported. Lokinet primarily uses TCP via SOCKS5 proxy.
//
// PREREQUISITES: Lokinet daemon must be running with SOCKS5 proxy enabled.
// Configure proxy port via LOKINET_PROXY_ADDR environment variable (default: 127.0.0.1:9050).
//
// USAGE EXAMPLE:
//
//	loki := transport.NewLokinetTransport()
//	defer loki.Close()
//
//	// Connect to a Lokinet address
//	conn, err := loki.Dial("example.loki:8080")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use conn for communication...
//
// See also: https://docs.lokinet.dev for Lokinet documentation.
type LokinetTransport struct {
	mu          sync.RWMutex
	proxyAddr   string
	socksDialer proxy.Dialer
}

// NewLokinetTransport creates a new Lokinet transport instance.
// Uses LOKINET_PROXY_ADDR environment variable or defaults to 127.0.0.1:9050.
func NewLokinetTransport() *LokinetTransport {
	proxyAddr := os.Getenv("LOKINET_PROXY_ADDR")
	if proxyAddr == "" {
		proxyAddr = "127.0.0.1:9050" // Default Lokinet SOCKS5 port
	}

	logrus.WithFields(logrus.Fields{
		"function":   "NewLokinetTransport",
		"proxy_addr": proxyAddr,
	}).Info("Creating Lokinet transport")

	// Create SOCKS5 dialer for the Lokinet proxy
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "NewLokinetTransport",
			"proxy_addr": proxyAddr,
			"error":      err.Error(),
		}).Warn("Failed to create SOCKS5 dialer, will retry on Dial")
	}

	return &LokinetTransport{
		proxyAddr:   proxyAddr,
		socksDialer: dialer,
	}
}

// Listen creates a listener for Lokinet .loki addresses.
// Note: Creating Lokinet services requires SNApp configuration and is not
// supported via SOCKS5 alone. Applications should configure SNApps
// via Lokinet configuration and use the regular IP transport to bind locally.
func (t *LokinetTransport) Listen(address string) (net.Listener, error) {
	logrus.WithFields(logrus.Fields{
		"function": "LokinetTransport.Listen",
		"address":  address,
	}).Debug("Lokinet listen requested")

	if !strings.Contains(address, ".loki") {
		return nil, fmt.Errorf("invalid Lokinet address format: %s (must contain .loki)", address)
	}

	return nil, fmt.Errorf("Lokinet SNApp hosting not supported via SOCKS5 - configure via Lokinet config")
}

// Dial establishes a connection through Lokinet to the given .loki address via SOCKS5.
// Supports both .loki addresses and regular addresses routed through Lokinet.
func (t *LokinetTransport) Dial(address string) (net.Conn, error) {
	t.mu.RLock()
	dialer := t.socksDialer
	proxyAddr := t.proxyAddr
	t.mu.RUnlock()

	logrus.WithFields(logrus.Fields{
		"function":   "LokinetTransport.Dial",
		"address":    address,
		"proxy_addr": proxyAddr,
	}).Debug("Lokinet dial requested")

	// Recreate dialer if it wasn't initialized during construction
	if dialer == nil {
		var err error
		dialer, err = proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":   "LokinetTransport.Dial",
				"proxy_addr": proxyAddr,
				"error":      err.Error(),
			}).Error("Failed to create SOCKS5 dialer")
			return nil, fmt.Errorf("Lokinet SOCKS5 dialer creation failed: %w", err)
		}

		t.mu.Lock()
		t.socksDialer = dialer
		t.mu.Unlock()
	}

	// Dial through SOCKS5 proxy
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "LokinetTransport.Dial",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to dial through Lokinet")
		return nil, fmt.Errorf("Lokinet dial failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "LokinetTransport.Dial",
		"address":     address,
		"local_addr":  conn.LocalAddr().String(),
		"remote_addr": conn.RemoteAddr().String(),
	}).Info("Lokinet connection established")

	return conn, nil
}

// DialPacket creates a packet connection through Lokinet.
// Currently returns an error as Lokinet transport primarily uses TCP.
func (t *LokinetTransport) DialPacket(address string) (net.PacketConn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "LokinetTransport.DialPacket",
		"address":  address,
	}).Debug("Lokinet packet dial requested")

	return nil, fmt.Errorf("Lokinet UDP transport not supported via SOCKS5")
}

// SupportedNetworks returns the network types supported by Lokinet transport.
func (t *LokinetTransport) SupportedNetworks() []string {
	return []string{"loki", "lokinet"}
}

// Close closes the Lokinet transport.
func (t *LokinetTransport) Close() error {
	logrus.WithField("function", "LokinetTransport.Close").Debug("Closing Lokinet transport")
	return nil
}
