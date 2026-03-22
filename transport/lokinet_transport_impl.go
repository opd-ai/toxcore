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
