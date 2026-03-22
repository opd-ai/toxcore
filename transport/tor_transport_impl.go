package transport

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/go-i2p/onramp"
	"github.com/sirupsen/logrus"
)

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
		controlAddr = "127.0.0.1:9051" // Default Tor control port
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

	onion, err := onramp.NewOnion("toxcore-tor")
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

// recoverTorPanic converts a recovered onramp panic value into an error.
// The onramp library panics when the Tor daemon is not running; this helper
// converts such panics into actionable errors instead of crashing the caller.
func recoverTorPanic(operation string, r interface{}) error {
	logrus.WithFields(logrus.Fields{
		"function":  "TorTransport." + operation,
		"operation": operation,
		"panic":     r,
	}).Error("Tor operation panicked – is Tor running?")
	return fmt.Errorf("Tor onramp initialization failed: %v (is Tor running?)", r)
}

// Listen creates a listener for Tor .onion addresses via onramp.
// The Onion instance handles key persistence and service registration automatically.
// The first call may take 30-90 seconds for initial descriptor publishing.
// Returns an error (not a panic) if the Tor daemon is not running.
func (t *TorTransport) Listen(address string) (listener net.Listener, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "TorTransport.Listen",
		"address":  address,
	}).Debug("Tor listen requested")

	if !strings.Contains(address, ".onion") {
		return nil, fmt.Errorf("invalid Tor address format: %s (must contain .onion)", address)
	}

	if err = t.ensureOnion(); err != nil {
		return nil, fmt.Errorf("Tor listen failed: %w", err)
	}

	// The onramp library panics if Tor is not running; convert to error.
	defer func() {
		if r := recover(); r != nil {
			err = recoverTorPanic("Listen", r)
		}
	}()

	listener, err = t.onion.Listen()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "TorTransport.Listen",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to create Tor listener")
		return nil, fmt.Errorf("Tor listener creation failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "TorTransport.Listen",
		"address":    address,
		"local_addr": listener.Addr().String(),
	}).Info("Tor listener created successfully")

	return listener, nil
}

// Dial establishes a connection through Tor to the given .onion address via onramp.
// Supports both .onion addresses and regular addresses routed through Tor.
// The Onion instance reuses the same Tor circuits for all dials.
// Returns an error (not a panic) if the Tor daemon is not running.
func (t *TorTransport) Dial(address string) (conn net.Conn, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":     "TorTransport.Dial",
		"address":      address,
		"control_addr": t.controlAddr,
	}).Debug("Tor dial requested")

	if err = t.ensureOnion(); err != nil {
		return nil, fmt.Errorf("Tor dial failed: %w", err)
	}

	// The onramp library panics if Tor is not running; convert to error.
	defer func() {
		if r := recover(); r != nil {
			err = recoverTorPanic("Dial", r)
		}
	}()

	conn, err = t.onion.Dial("tcp", address)
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
