package transport

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// MultiTransport orchestrates multiple NetworkTransport implementations,
// automatically selecting the appropriate transport based on address format.
// This enables toxcore to seamlessly handle connections across different
// network types (IP, Tor, I2P, Nym, Loki) through a unified interface.
type MultiTransport struct {
	transports map[string]NetworkTransport
	mu         sync.RWMutex
}

// NewMultiTransport creates a new multi-transport instance with default transports.
// It initializes support for IP networks and provides placeholders for
// privacy networks (Tor, I2P, Nym) that can be implemented later.
func NewMultiTransport() *MultiTransport {
	logrus.WithField("function", "NewMultiTransport").Info("Creating multi-transport")

	mt := &MultiTransport{
		transports: make(map[string]NetworkTransport),
	}

	// Register default transports
	mt.RegisterTransport("ip", NewIPTransport())
	mt.RegisterTransport("tor", NewTorTransport())
	mt.RegisterTransport("i2p", NewI2PTransport())
	mt.RegisterTransport("nym", NewNymTransport())

	logrus.WithFields(logrus.Fields{
		"function":           "NewMultiTransport",
		"registered_count":   len(mt.transports),
		"supported_networks": mt.GetSupportedNetworks(),
	}).Info("Multi-transport initialized")

	return mt
}

// RegisterTransport registers a network transport for a specific network type.
// This allows dynamic addition of transport implementations.
func (mt *MultiTransport) RegisterTransport(networkType string, transport NetworkTransport) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":     "MultiTransport.RegisterTransport",
		"network_type": networkType,
		"transport":    fmt.Sprintf("%T", transport),
	}).Info("Registering network transport")

	mt.transports[networkType] = transport
}

// selectTransport chooses the appropriate transport based on the address format.
// It analyzes the address to determine the network type and returns the
// corresponding transport implementation.
func (mt *MultiTransport) selectTransport(address string) (NetworkTransport, error) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	logrus.WithFields(logrus.Fields{
		"function": "MultiTransport.selectTransport",
		"address":  address,
	}).Debug("Selecting transport for address")

	// Determine network type based on address format
	var networkType string
	switch {
	case strings.Contains(address, ".onion"):
		networkType = "tor"
	case strings.Contains(address, ".i2p"):
		networkType = "i2p"
	case strings.Contains(address, ".nym"):
		networkType = "nym"
	case strings.Contains(address, ".loki"):
		networkType = "loki"
	default:
		// Default to IP for standard addresses
		networkType = "ip"
	}

	transport, exists := mt.transports[networkType]
	if !exists {
		logrus.WithFields(logrus.Fields{
			"function":     "MultiTransport.selectTransport",
			"address":      address,
			"network_type": networkType,
		}).Error("No transport registered for network type")
		return nil, fmt.Errorf("no transport registered for network type: %s", networkType)
	}

	logrus.WithFields(logrus.Fields{
		"function":     "MultiTransport.selectTransport",
		"address":      address,
		"network_type": networkType,
		"transport":    fmt.Sprintf("%T", transport),
	}).Debug("Transport selected successfully")

	return transport, nil
}

// Listen creates a listener on the given address using the appropriate transport.
// The address format determines which transport is used:
// - Standard IP addresses use IPTransport
// - .onion addresses use TorTransport
// - .i2p addresses use I2PTransport
// - .nym addresses use NymTransport
func (mt *MultiTransport) Listen(address string) (net.Listener, error) {
	logrus.WithFields(logrus.Fields{
		"function": "MultiTransport.Listen",
		"address":  address,
	}).Info("Creating listener via multi-transport")

	transport, err := mt.selectTransport(address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "MultiTransport.Listen",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to select transport")
		return nil, fmt.Errorf("transport selection failed: %w", err)
	}

	listener, err := transport.Listen(address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "MultiTransport.Listen",
			"address":   address,
			"transport": fmt.Sprintf("%T", transport),
			"error":     err.Error(),
		}).Error("Failed to create listener")
		return nil, fmt.Errorf("listen failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "MultiTransport.Listen",
		"address":    address,
		"transport":  fmt.Sprintf("%T", transport),
		"local_addr": listener.Addr().String(),
	}).Info("Listener created successfully")

	return listener, nil
}

// Dial establishes a connection to the given address using the appropriate transport.
func (mt *MultiTransport) Dial(address string) (net.Conn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "MultiTransport.Dial",
		"address":  address,
	}).Info("Dialing connection via multi-transport")

	transport, err := mt.selectTransport(address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "MultiTransport.Dial",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to select transport")
		return nil, fmt.Errorf("transport selection failed: %w", err)
	}

	conn, err := transport.Dial(address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "MultiTransport.Dial",
			"address":   address,
			"transport": fmt.Sprintf("%T", transport),
			"error":     err.Error(),
		}).Error("Failed to dial connection")
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "MultiTransport.Dial",
		"address":     address,
		"transport":   fmt.Sprintf("%T", transport),
		"local_addr":  conn.LocalAddr().String(),
		"remote_addr": conn.RemoteAddr().String(),
	}).Info("Connection established successfully")

	return conn, nil
}

// DialPacket creates a packet connection to the given address using the appropriate transport.
//
// When both Tor and I2P are simultaneously registered, and the address would normally
// be routed to Tor (a TCP-only transport that cannot carry UDP), I2P datagrams are used
// instead. This ensures Tox UDP protocol messages work correctly even when Tor is active.
//
// Routing priority:
//  1. Regular IP addresses → IPTransport (normal UDP).
//  2. .i2p addresses → I2PTransport (I2P native datagrams).
//  3. .onion addresses with both Tor+I2P registered → I2P datagrams (Tor cannot do UDP).
//  4. .onion addresses without I2P → Tor (which will return an unsupported error).
func (mt *MultiTransport) DialPacket(address string) (net.PacketConn, error) {
	logrus.WithFields(logrus.Fields{
		"function": "MultiTransport.DialPacket",
		"address":  address,
	}).Info("Creating packet connection via multi-transport")

	selectedTransport, err := mt.selectTransport(address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "MultiTransport.DialPacket",
			"address":  address,
			"error":    err.Error(),
		}).Error("Failed to select transport")
		return nil, fmt.Errorf("transport selection failed: %w", err)
	}

	// When the selected transport is Tor (TCP-only) and I2P is also registered,
	// prefer I2P datagrams. Tor cannot carry UDP/datagram traffic (Tox protocol
	// messages), so I2P datagrams are the correct choice when both are enabled.
	// We detect Tor via SupportedNetworks() to avoid interface-to-concrete type assertions.
	mt.mu.RLock()
	i2pTransport, i2pRegistered := mt.transports["i2p"]
	mt.mu.RUnlock()

	if i2pRegistered && isTorOnlyTransport(selectedTransport) {
		logrus.WithFields(logrus.Fields{
			"function": "MultiTransport.DialPacket",
			"address":  address,
		}).Info("Tor+I2P simultaneously enabled: using I2P datagrams for Tox messages (Tor is TCP-only)")

		conn, i2pErr := i2pTransport.DialPacket(address)
		if i2pErr != nil {
			logrus.WithFields(logrus.Fields{
				"function": "MultiTransport.DialPacket",
				"address":  address,
				"error":    i2pErr.Error(),
			}).Error("I2P datagram failed in Tor+I2P simultaneous mode")
			return nil, fmt.Errorf("I2P datagram failed (Tor+I2P mode): %w", i2pErr)
		}

		logrus.WithFields(logrus.Fields{
			"function":   "MultiTransport.DialPacket",
			"address":    address,
			"local_addr": conn.LocalAddr().String(),
		}).Info("I2P datagram connection created (Tor+I2P simultaneous mode)")

		return conn, nil
	}

	conn, err := selectedTransport.DialPacket(address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "MultiTransport.DialPacket",
			"address":   address,
			"transport": fmt.Sprintf("%T", selectedTransport),
			"error":     err.Error(),
		}).Error("Failed to create packet connection")
		return nil, fmt.Errorf("dial packet failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "MultiTransport.DialPacket",
		"address":    address,
		"transport":  fmt.Sprintf("%T", selectedTransport),
		"local_addr": conn.LocalAddr().String(),
	}).Info("Packet connection created successfully")

	return conn, nil
}

// isTorOnlyTransport reports whether the given transport exclusively supports the Tor
// network (i.e. it is a TCP-only Tor transport that cannot handle UDP datagrams).
// Uses SupportedNetworks() to avoid interface-to-concrete type assertions per the
// project's networking conventions. This is only called on the Tor-selected path
// (for .onion addresses), so the per-call slice allocation is acceptable.
func isTorOnlyTransport(t NetworkTransport) bool {
	nets := t.SupportedNetworks()
	return len(nets) == 1 && nets[0] == "tor"
}

// GetSupportedNetworks returns a list of all network types supported by registered transports.
func (mt *MultiTransport) GetSupportedNetworks() []string {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	var networks []string
	for _, transport := range mt.transports {
		networks = append(networks, transport.SupportedNetworks()...)
	}

	logrus.WithFields(logrus.Fields{
		"function": "MultiTransport.GetSupportedNetworks",
		"networks": networks,
		"count":    len(networks),
	}).Debug("Retrieved supported networks")

	return networks
}

// GetTransport returns the transport registered for a specific network type.
// This allows direct access to transport implementations when needed.
func (mt *MultiTransport) GetTransport(networkType string) (NetworkTransport, bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	transport, exists := mt.transports[networkType]

	logrus.WithFields(logrus.Fields{
		"function":     "MultiTransport.GetTransport",
		"network_type": networkType,
		"exists":       exists,
	}).Debug("Retrieved transport for network type")

	return transport, exists
}

// Close closes all registered transports and releases resources.
func (mt *MultiTransport) Close() error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":        "MultiTransport.Close",
		"transport_count": len(mt.transports),
	}).Info("Closing multi-transport")

	var errs []error
	for networkType, transport := range mt.transports {
		if err := transport.Close(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function":     "MultiTransport.Close",
				"network_type": networkType,
				"error":        err.Error(),
			}).Error("Failed to close transport")
			errs = append(errs, fmt.Errorf("failed to close %s transport: %w", networkType, err))
		}
	}

	// Clear the transports map
	mt.transports = make(map[string]NetworkTransport)

	if len(errs) > 0 {
		// Return the first error, but log all errors
		return errs[0]
	}

	logrus.WithField("function", "MultiTransport.Close").Info("Multi-transport closed successfully")
	return nil
}
