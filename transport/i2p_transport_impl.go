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
	return NewI2PTransportWithSAMAddr(samAddr)
}

// NewI2PTransportWithSAMAddr creates a new I2P transport using the given SAM bridge address.
// This bypasses the I2P_SAM_ADDR environment variable, allowing the address to be
// supplied programmatically.
// The onramp Garlic instance is lazily initialized on first use of Dial/Listen/DialPacket.
func NewI2PTransportWithSAMAddr(samAddr string) *I2PTransport {
	if samAddr == "" {
		samAddr = onramp.SAM_ADDR
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewI2PTransportWithSAMAddr",
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

// Dial establishes a connection through I2P to the given .b32.i2p address via onramp.
// The Garlic instance reuses the same I2P session for all dials.
func (t *I2PTransport) Dial(address string) (net.Conn, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "I2PTransport.Dial",
		"address":  address,
		"sam_addr": t.samAddr,
	}).Debug("I2P dial requested")

	if !strings.Contains(address, ".i2p") {
		return nil, fmt.Errorf("invalid I2P address format: %s (must contain .i2p)", address)
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

// DialPacket creates a packet connection through I2P using SAM datagrams.
// The Garlic instance reuses the same I2P session for all dials.
func (t *I2PTransport) DialPacket(address string) (net.PacketConn, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "I2PTransport.DialPacket",
		"address":  address,
		"sam_addr": t.samAddr,
	}).Debug("I2P packet dial requested")

	if !strings.Contains(address, ".i2p") {
		return nil, fmt.Errorf("invalid I2P address format: %s (must contain .i2p)", address)
	}

	if err := t.ensureGarlic(); err != nil {
		return nil, fmt.Errorf("I2P packet dial failed: %w", err)
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
