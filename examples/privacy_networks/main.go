package main

import (
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

// demonstrateTorTransport demonstrates the Tor transport for connecting to .onion addresses.
// It creates a Tor transport, shows supported networks, and attempts a connection through
// the Tor SOCKS5 proxy. Connection failures are expected if Tor is not running.
func demonstrateTorTransport() {
	log := logrus.WithField("transport", "tor")
	log.Info("1. Tor Transport (.onion addresses)")

	tor := transport.NewTorTransport()
	defer tor.Close()

	log.WithField("networks", tor.SupportedNetworks()).Info("Supported networks")

	exampleOnion := "3g2upl4pq6kufc4m.onion:80"
	log.WithField("address", exampleOnion).Info("Attempting connection")

	conn, err := tor.Dial(exampleOnion)
	if err != nil {
		log.WithError(err).Warn("Connection failed (expected if Tor not running)")
	} else {
		log.WithFields(logrus.Fields{
			"local_addr":  conn.LocalAddr().String(),
			"remote_addr": conn.RemoteAddr().String(),
		}).Info("Successfully connected through Tor!")
		conn.Close()
	}

	log.Info("Custom Tor proxy can be configured via TOR_PROXY_ADDR environment variable")
}

// demonstrateI2PTransport demonstrates the I2P transport for connecting to .b32.i2p addresses.
// It creates an I2P transport using the SAM bridge protocol, shows supported networks,
// and attempts a connection. Connection failures are expected if I2P router is not running.
func demonstrateI2PTransport() {
	log := logrus.WithField("transport", "i2p")
	log.Info("2. I2P Transport (.i2p addresses)")

	i2p := transport.NewI2PTransport()
	defer i2p.Close()

	log.WithField("networks", i2p.SupportedNetworks()).Info("Supported networks")

	exampleI2P := "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80"
	log.WithField("address", exampleI2P).Info("Attempting connection")

	conn, err := i2p.Dial(exampleI2P)
	if err != nil {
		log.WithError(err).Warn("Connection failed (expected if I2P not running)")
	} else {
		log.WithFields(logrus.Fields{
			"local_addr":  conn.LocalAddr().String(),
			"remote_addr": conn.RemoteAddr().String(),
		}).Info("Successfully connected through I2P!")
		conn.Close()
	}

	log.WithField("default_sam_addr", "127.0.0.1:7656").Info("Custom I2P SAM address can be configured via I2P_SAM_ADDR environment variable")
}

// demonstrateLokinetTransport demonstrates the Lokinet transport for connecting to .loki addresses.
// It creates a Lokinet transport using the Lokinet SOCKS5 proxy, shows supported networks,
// and attempts a connection. Connection failures are expected if Lokinet daemon is not running.
func demonstrateLokinetTransport() {
	log := logrus.WithField("transport", "lokinet")
	log.Info("3. Lokinet Transport (.loki addresses)")

	lokinet := transport.NewLokinetTransport()
	defer lokinet.Close()

	log.WithField("networks", lokinet.SupportedNetworks()).Info("Supported networks")

	exampleLoki := "example.loki:80"
	log.WithField("address", exampleLoki).Info("Attempting connection")

	conn, err := lokinet.Dial(exampleLoki)
	if err != nil {
		log.WithError(err).Warn("Connection failed (expected if Lokinet not running)")
	} else {
		log.WithFields(logrus.Fields{
			"local_addr":  conn.LocalAddr().String(),
			"remote_addr": conn.RemoteAddr().String(),
		}).Info("Successfully connected through Lokinet!")
		conn.Close()
	}

	log.WithField("default_proxy_addr", "127.0.0.1:9050").Info("Custom Lokinet proxy can be configured via LOKINET_PROXY_ADDR environment variable")
}
