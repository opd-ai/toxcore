package main

import (
	"net"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

func main() {
	// Configure structured logging for demonstration
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logrus.Info("Privacy Network Transport Examples")
	logrus.Info("====================================")

	demonstrateTorTransport()
	demonstrateI2PTransport()
	demonstrateLokinetTransport()
}

// demonstrateTransport is a helper function that demonstrates a privacy network transport
// by showing supported networks, attempting a connection, and providing configuration guidance.
func demonstrateTransport(name, number, address, configMsg string, dialer func(string) (net.Conn, error)) {
	log := logrus.WithField("transport", name)
	log.Infof("%s. %s Transport", number, name)

	log.WithField("address", address).Info("Attempting connection")

	conn, err := dialer(address)
	if err != nil {
		log.WithError(err).Warnf("Connection failed (expected if %s not running)", name)
	} else {
		log.WithFields(logrus.Fields{
			"local_addr":  conn.LocalAddr().String(),
			"remote_addr": conn.RemoteAddr().String(),
		}).Infof("Successfully connected through %s!", name)
		conn.Close()
	}

	if configMsg != "" {
		log.Info(configMsg)
	}
}

// demonstrateTorTransport demonstrates the Tor transport for connecting to .onion addresses.
// It creates a Tor transport, shows supported networks, and attempts a connection through
// the Tor SOCKS5 proxy. Connection failures are expected if Tor is not running.
func demonstrateTorTransport() {
	tor := transport.NewTorTransport()
	defer tor.Close()

	demonstrateTransport(
		"Tor",
		"1",
		"3g2upl4pq6kufc4m.onion:80",
		"Custom Tor proxy can be configured via TOR_PROXY_ADDR environment variable",
		tor.Dial,
	)
}

// demonstrateI2PTransport demonstrates the I2P transport for connecting to .b32.i2p addresses.
// It creates an I2P transport using the SAM bridge protocol, shows supported networks,
// and attempts a connection. Connection failures are expected if I2P router is not running.
func demonstrateI2PTransport() {
	i2p := transport.NewI2PTransport()
	defer i2p.Close()

	demonstrateTransport(
		"I2P",
		"2",
		"ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80",
		"Custom I2P SAM address can be configured via I2P_SAM_ADDR environment variable",
		i2p.Dial,
	)
}

// demonstrateLokinetTransport demonstrates the Lokinet transport for connecting to .loki addresses.
// It creates a Lokinet transport using the Lokinet SOCKS5 proxy, shows supported networks,
// and attempts a connection. Connection failures are expected if Lokinet daemon is not running.
func demonstrateLokinetTransport() {
	lokinet := transport.NewLokinetTransport()
	defer lokinet.Close()

	demonstrateTransport(
		"Lokinet",
		"3",
		"example.loki:80",
		"Custom Lokinet proxy can be configured via LOKINET_PROXY_ADDR environment variable",
		lokinet.Dial,
	)
}
