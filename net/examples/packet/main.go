// Package main demonstrates the Tox packet networking interfaces.
//
// This example shows how to use the PacketDial and PacketListen functions
// along with the ToxPacketConn and ToxPacketListener implementations.
package main

import (
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/crypto"
	toxnet "github.com/opd-ai/toxcore/net"
	"github.com/sirupsen/logrus"
)

// TimeProvider is an interface for getting the current time.
// This allows injecting a mock time provider for deterministic testing.
type TimeProvider interface {
	Now() time.Time
}

// RealTimeProvider implements TimeProvider using the actual system time.
type RealTimeProvider struct{}

// Now returns the current system time.
func (RealTimeProvider) Now() time.Time {
	return time.Now()
}

// timeProvider is the package-level time provider used for deadline calculations.
// Can be replaced with a mock for testing.
var timeProvider TimeProvider = RealTimeProvider{}

func main() {
	logrus.Info("Tox Packet Networking Example")
	logrus.Info("=============================")

	// Example 1: Direct packet connection usage
	logrus.Info("1. Direct ToxPacketConn Usage:")
	if err := demonstratePacketConn(); err != nil {
		logrus.WithError(err).Fatal("Packet connection demo failed")
	}

	// Example 2: Packet listener usage
	logrus.Info("2. ToxPacketListener Usage:")
	if err := demonstratePacketListener(); err != nil {
		logrus.WithError(err).Fatal("Packet listener demo failed")
	}

	// Example 3: PacketDial and PacketListen functions
	logrus.Info("3. PacketDial/PacketListen Functions:")
	if err := demonstratePacketDialListen(); err != nil {
		logrus.WithError(err).Warn("PacketDial/Listen demo incomplete")
	}

	// Example 4: Integration with net.PacketConn interface
	logrus.Info("4. Integration Example:")
	if err := integrationExample(); err != nil {
		logrus.WithError(err).Fatal("Integration example failed")
	}
}

func demonstratePacketConn() error {
	// Generate a test Tox address
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return err
	}

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := toxnet.NewToxAddrFromPublicKey(keyPair.Public, nospam)

	logrus.WithField("address", localAddr.String()).Info("Generated Tox Address")

	// Create packet connection
	conn, err := toxnet.NewToxPacketConn(localAddr, ":0")
	if err != nil {
		return err
	}
	defer conn.Close()

	logrus.WithField("local_addr", conn.LocalAddr().String()).Info("Local address")
	logrus.Info("Packet connection created successfully")

	// Test deadline setting using injectable time provider
	deadline := timeProvider.Now().Add(5 * time.Second)
	conn.SetDeadline(deadline)
	logrus.WithField("deadline", deadline.Format(time.RFC3339)).Info("Set deadline")

	// In a real implementation, you would use WriteTo to send packets
	// and ReadFrom to receive them
	return nil
}

func demonstratePacketListener() error {
	// Generate a test Tox address
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return err
	}

	nospam := [4]byte{0x05, 0x06, 0x07, 0x08}
	localAddr := toxnet.NewToxAddrFromPublicKey(keyPair.Public, nospam)

	logrus.WithField("address", localAddr.String()).Info("Generated Tox Address")

	// Create packet listener
	listener, err := toxnet.NewToxPacketListener(localAddr, ":0")
	if err != nil {
		return err
	}
	defer listener.Close()

	logrus.WithField("listener_addr", listener.Addr().String()).Info("Listener address")
	logrus.Info("Packet listener created successfully")

	// In a real implementation, you would call Accept() in a loop
	// to handle incoming connections
	return nil
}

func demonstratePacketDialListen() error {
	// Test with invalid network (should fail)
	_, err := toxnet.PacketDial("invalid", "test-addr")
	if err != nil {
		logrus.WithField("error", err.Error()).Info("Expected error for invalid network")
	}

	// Test with invalid address (should fail)
	_, err = toxnet.PacketDial("tox", "invalid-tox-id")
	if err != nil {
		logrus.WithField("error", err.Error()).Info("Expected error for invalid Tox ID")
	}

	// Test PacketListen with invalid network (should fail)
	_, err = toxnet.PacketListen("invalid", ":0", nil)
	if err != nil {
		logrus.WithField("error", err.Error()).Info("Expected error for invalid network")
	}

	// Test PacketListen with nil Tox instance (should fail)
	_, err = toxnet.PacketListen("tox", ":0", nil)
	if err != nil {
		logrus.WithField("error", err.Error()).Info("Expected error for nil Tox instance")
	}

	// Test PacketListen with valid Tox instance
	opts := toxcore.NewOptions()
	tox, err := toxcore.New(opts)
	if err != nil {
		logrus.WithError(err).Warn("Could not create Tox instance for demo")
		return err
	}
	defer tox.Kill()

	listener, err := toxnet.PacketListen("tox", ":0", tox)
	if err != nil {
		logrus.WithError(err).Warn("Unexpected error creating packet listener")
		return err
	}
	defer listener.Close()
	logrus.WithField("address", listener.Addr().String()).Info("PacketListen created successfully")

	logrus.Info("PacketDial and PacketListen functions tested")
	return nil
}

// integrationExample shows how to integrate with existing net package patterns.
// Our ToxPacketConn can be used as a drop-in replacement for net.PacketConn.
func integrationExample() error {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return err
	}
	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := toxnet.NewToxAddrFromPublicKey(keyPair.Public, nospam)

	// Our ToxPacketConn implements net.PacketConn
	packetConn, err := toxnet.NewToxPacketConn(localAddr, ":0")
	if err != nil {
		return err
	}
	defer packetConn.Close()

	// Demonstrate that it can be used anywhere that expects net.PacketConn
	// by verifying the interface methods are available
	_ = packetConn.LocalAddr()
	logrus.Info("Integration with net.PacketConn interface: ✓")
	return nil
}
