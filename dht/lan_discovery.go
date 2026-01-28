// Package dht provides LANDiscovery for local network peer discovery.
// This is currently a stub implementation - the full feature is reserved for future implementation.
package dht

import (
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

// LANDiscovery handles local network peer discovery via UDP broadcast/multicast.
// This is currently a stub implementation - the full feature is reserved for future implementation.
//
// Current Status: The LocalDiscovery option exists in the Options struct,
// but full implementation is pending. This stub allows the code to compile
// and gracefully handles the feature flag.
//
//export ToxDHTLANDiscovery
type LANDiscovery struct {
	publicKey [32]byte
	port      uint16
	callback  func(publicKey [32]byte, addr net.Addr)
	running   bool
	mu        sync.RWMutex
}

// NewLANDiscovery creates a new LAN discovery instance.
// This is currently a stub implementation that does not perform actual discovery.
//
//export ToxDHTLANDiscoveryNew
func NewLANDiscovery(publicKey [32]byte, port uint16) *LANDiscovery {
	logrus.WithFields(logrus.Fields{
		"function":   "NewLANDiscovery",
		"port":       port,
		"public_key": publicKey[:8],
	}).Debug("Creating LAN discovery instance (stub implementation)")

	return &LANDiscovery{
		publicKey: publicKey,
		port:      port,
	}
}

// OnPeer sets the callback for when a peer is discovered on the local network.
// The callback will be invoked with the peer's public key and network address.
//
//export ToxDHTLANDiscoveryOnPeer
func (l *LANDiscovery) OnPeer(callback func(publicKey [32]byte, addr net.Addr)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callback = callback

	logrus.WithFields(logrus.Fields{
		"function": "OnPeer",
	}).Debug("LAN discovery peer callback registered")
}

// Start begins the LAN discovery process.
// This is currently a stub implementation that does not perform actual discovery.
// Returns nil to indicate success (the feature silently does nothing).
//
//export ToxDHTLANDiscoveryStart
func (l *LANDiscovery) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return nil
	}

	l.running = true

	logrus.WithFields(logrus.Fields{
		"function": "Start",
		"port":     l.port,
		"status":   "stub_implementation",
	}).Info("LAN discovery started (stub implementation - feature reserved for future)")

	// Note: Full implementation would start UDP broadcast/multicast listening
	// and periodically announce our presence to the local network.
	// This is reserved for future implementation.

	return nil
}

// Stop halts the LAN discovery process.
//
//export ToxDHTLANDiscoveryStop
func (l *LANDiscovery) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return
	}

	l.running = false

	logrus.WithFields(logrus.Fields{
		"function": "Stop",
	}).Debug("LAN discovery stopped")
}

// IsRunning returns whether LAN discovery is currently active.
//
//export ToxDHTLANDiscoveryIsRunning
func (l *LANDiscovery) IsRunning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.running
}
