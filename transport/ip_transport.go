package transport

import (
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
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
