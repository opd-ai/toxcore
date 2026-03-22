package transport

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// ErrNymNotImplemented is returned when attempting to use NymTransport functionality.
// Nym mixnet integration requires the Nym SDK websocket client which is not yet implemented.
// See NymTransport documentation for implementation guidance.
var ErrNymNotImplemented = errors.New("nym transport not implemented: requires Nym SDK websocket client integration")

// NymTransport implements NetworkTransport for the Nym mixnet via the local Nym SOCKS5 proxy.
// The Nym native client exposes a SOCKS5 proxy interface that routes traffic through the
// mixnet, providing strong anonymity through cover traffic and mixnet delays.
//
// IMPLEMENTATION STATUS:
//   - Dial(): Fully implemented via SOCKS5 proxy to local Nym client.
//   - Listen(): Not supported. Nym does not expose a listener interface via SOCKS5.
//     Incoming connections require a dedicated Nym service node configuration.
//   - DialPacket(): Implemented via length-prefixed packet framing over a SOCKS5 stream.
//
// PREREQUISITES: A Nym native client must be running with SOCKS5 mode enabled.
// Configure the client proxy address via NYM_CLIENT_ADDR environment variable
// (default: 127.0.0.1:1080).
//
// Running a local Nym client in SOCKS5 mode:
//
//	nym-socks5-client run --id myid
//
// USAGE EXAMPLE:
//
//	nym := transport.NewNymTransport()
//	defer nym.Close()
//
//	// Connect to a .nym address through the Nym mixnet
//	conn, err := nym.Dial("example.nym:8080")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use conn for communication...
//
// See also: https://nymtech.net/docs for Nym protocol documentation.
type NymTransport struct {
	mu          sync.RWMutex
	proxyAddr   string
	socksDialer proxy.Dialer
}

// NewNymTransport creates a new Nym transport instance.
// Uses NYM_CLIENT_ADDR environment variable or defaults to 127.0.0.1:1080.
// The SOCKS5 dialer is initialized eagerly; Dial will re-create it if initialization fails.
func NewNymTransport() *NymTransport {
	proxyAddr := os.Getenv("NYM_CLIENT_ADDR")
	if proxyAddr == "" {
		proxyAddr = "127.0.0.1:1080" // Default Nym SOCKS5 proxy port
	}

	logrus.WithFields(logrus.Fields{
		"function":   "NewNymTransport",
		"proxy_addr": proxyAddr,
	}).Info("Creating Nym transport (SOCKS5)")

	// Create SOCKS5 dialer for the Nym proxy
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "NewNymTransport",
			"proxy_addr": proxyAddr,
			"error":      err.Error(),
		}).Warn("Failed to create SOCKS5 dialer, will retry on Dial")
	}

	return &NymTransport{
		proxyAddr:   proxyAddr,
		socksDialer: dialer,
	}
}

// Listen creates a listener for Nym addresses.
// Nym does not provide a listener interface via its SOCKS5 proxy; this operation
// is unsupported. Configure a dedicated Nym service via the Nym service provider framework.
func (t *NymTransport) Listen(address string) (net.Listener, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NymTransport.Listen",
		"address":  address,
	}).Debug("Nym listen requested")

	if !strings.Contains(address, ".nym") {
		return nil, fmt.Errorf("invalid Nym address format: %s (must contain .nym)", address)
	}

	return nil, fmt.Errorf("Nym service hosting not supported via SOCKS5: %w", ErrNymNotImplemented)
}

// Dial establishes a connection through the Nym mixnet to the given .nym address via SOCKS5.
// Requires a local Nym native client running in SOCKS5 mode on NYM_CLIENT_ADDR (default: 127.0.0.1:1080).
// If the Nym client is not reachable, an actionable error is returned.
func (t *NymTransport) Dial(address string) (net.Conn, error) {
	t.mu.RLock()
	dialer := t.socksDialer
	proxyAddr := t.proxyAddr
	t.mu.RUnlock()

	logrus.WithFields(logrus.Fields{
		"function":   "NymTransport.Dial",
		"address":    address,
		"proxy_addr": proxyAddr,
	}).Debug("Nym dial requested")

	if err := validateNymAddress(address); err != nil {
		return nil, err
	}

	dialer, err := t.ensureDialerInitialized(dialer, proxyAddr)
	if err != nil {
		return nil, err
	}

	conn, err := dialThroughNymProxy(dialer, address, proxyAddr)
	if err != nil {
		return nil, err
	}

	logNymConnectionEstablished(address, conn)
	return conn, nil
}

// validateNymAddress checks if the address is a valid Nym address.
func validateNymAddress(address string) error {
	if !strings.Contains(address, ".nym") {
		return fmt.Errorf("invalid Nym address format: %s (must contain .nym)", address)
	}
	return nil
}

// ensureDialerInitialized creates a SOCKS5 dialer if not already initialized.
func (t *NymTransport) ensureDialerInitialized(dialer proxy.Dialer, proxyAddr string) (proxy.Dialer, error) {
	if dialer != nil {
		return dialer, nil
	}

	newDialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "NymTransport.Dial",
			"proxy_addr": proxyAddr,
			"error":      err.Error(),
		}).Error("Failed to create SOCKS5 dialer")
		return nil, fmt.Errorf("Nym SOCKS5 dialer creation failed: %w", err)
	}

	t.mu.Lock()
	t.socksDialer = newDialer
	t.mu.Unlock()

	return newDialer, nil
}

// dialThroughNymProxy establishes connection through SOCKS5 proxy.
func dialThroughNymProxy(dialer proxy.Dialer, address, proxyAddr string) (net.Conn, error) {
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "NymTransport.Dial",
			"address":    address,
			"proxy_addr": proxyAddr,
			"error":      err.Error(),
		}).Error("Failed to dial through Nym - ensure Nym client is running on " + proxyAddr)
		return nil, fmt.Errorf("Nym dial failed (is Nym client running on %s?): %w", proxyAddr, err)
	}
	return conn, nil
}

// logNymConnectionEstablished logs successful Nym connection.
func logNymConnectionEstablished(address string, conn net.Conn) {
	logrus.WithFields(logrus.Fields{
		"function":    "NymTransport.Dial",
		"address":     address,
		"local_addr":  conn.LocalAddr().String(),
		"remote_addr": conn.RemoteAddr().String(),
	}).Info("Nym connection established")
}

// DialPacket creates a packet connection through the Nym mixnet via length-prefixed framing.
// Since Nym's SOCKS5 interface is stream-based, UDP-like semantics are emulated by framing
// each packet with a 4-byte big-endian length prefix over a TCP stream through the proxy.
func (t *NymTransport) DialPacket(address string) (net.PacketConn, error) {
	t.mu.RLock()
	dialer := t.socksDialer
	proxyAddr := t.proxyAddr
	t.mu.RUnlock()

	logrus.WithFields(logrus.Fields{
		"function": "NymTransport.DialPacket",
		"address":  address,
	}).Debug("Nym packet dial requested")

	if !strings.Contains(address, ".nym") {
		return nil, fmt.Errorf("invalid Nym address format: %s (must contain .nym)", address)
	}

	// Recreate dialer if needed
	if dialer == nil {
		var err error
		dialer, err = proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("Nym SOCKS5 dialer creation failed: %w", err)
		}
		t.mu.Lock()
		t.socksDialer = dialer
		t.mu.Unlock()
	}

	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "NymTransport.DialPacket",
			"address":    address,
			"proxy_addr": proxyAddr,
			"error":      err.Error(),
		}).Error("Failed to dial Nym for packet connection - ensure Nym client is running on " + proxyAddr)
		return nil, fmt.Errorf("Nym packet dial failed (is Nym client running on %s?): %w", proxyAddr, err)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "NymTransport.DialPacket",
		"address":    address,
		"local_addr": conn.LocalAddr().String(),
	}).Info("Nym packet connection established")

	return newNymPacketConn(conn), nil
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
