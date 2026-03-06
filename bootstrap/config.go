package bootstrap

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// DefaultClearnetPort is the standard Tox bootstrap port.
	DefaultClearnetPort = uint16(33445)

	// DefaultClearnetAddress is the default bind address for the clearnet server.
	DefaultClearnetAddress = "0.0.0.0"

	// DefaultTorControlAddr is the default Tor control-port address.
	// Override with the TOR_CONTROL_ADDR environment variable.
	DefaultTorControlAddr = "127.0.0.1:9051"

	// DefaultI2PSAMAddr is the default I2P SAM-bridge address.
	// Override with the I2P_SAM_ADDR environment variable.
	DefaultI2PSAMAddr = "127.0.0.1:7656"
)

// Config holds configuration for a multi-network bootstrap server.
// Use DefaultConfig to obtain a valid starting point, then customise as needed.
type Config struct {
	// --- Clearnet (UDP) ---

	// ClearnetEnabled controls whether the UDP bootstrap service is started.
	// Default: true.
	ClearnetEnabled bool

	// ClearnetAddress is the IP address to bind the UDP service on.
	// Use "0.0.0.0" to listen on all interfaces.
	// Default: "0.0.0.0".
	ClearnetAddress string

	// ClearnetPort is the UDP port to bind.
	// The Tox ecosystem conventionally uses 33445.
	// Default: 33445.
	ClearnetPort uint16

	// --- Tor / Onion service ---

	// OnionEnabled controls whether a Tor hidden-service endpoint is started.
	// Requires a running Tor daemon with the control port enabled.
	// Default: false.
	OnionEnabled bool

	// TorControlAddr is the address of the Tor control port used to manage the
	// hidden service. May also be set via the TOR_CONTROL_ADDR environment variable.
	// Default: "127.0.0.1:9051".
	TorControlAddr string

	// --- I2P ---

	// I2PEnabled controls whether an I2P destination endpoint is started.
	// Requires a running I2P router with the SAM bridge enabled.
	// Default: false.
	I2PEnabled bool

	// I2PSAMAddr is the address of the I2P SAM bridge.
	// May also be set via the I2P_SAM_ADDR environment variable.
	// Default: "127.0.0.1:7656".
	I2PSAMAddr string

	// --- Key persistence ---

	// SecretKey is an optional 32-byte secret key to use as the node's identity.
	// When set, the server reuses this key rather than generating a new one, so the
	// public key (and therefore the bootstrap node address) remains stable across
	// restarts. Leave nil to generate a new key pair on each call to New.
	SecretKey []byte

	// --- Timing ---

	// StartupTimeout is how long Start waits for each enabled network endpoint
	// to become ready before returning an error.
	// Default: 30 seconds.
	StartupTimeout time.Duration

	// IterationInterval is the period of the internal DHT iteration loop.
	// Smaller values make the server more responsive; larger values reduce CPU.
	// Default: 50 ms.
	IterationInterval time.Duration

	// --- Logging ---

	// Logger is the logrus logger to use. If nil, the standard global logger is used.
	Logger *logrus.Logger
}

// DefaultConfig returns a Config with sensible defaults.
// Only clearnet is enabled by default; set OnionEnabled and/or I2PEnabled to
// true to activate those networks.
func DefaultConfig() *Config {
	torControlAddr := os.Getenv("TOR_CONTROL_ADDR")
	if torControlAddr == "" {
		torControlAddr = DefaultTorControlAddr
	}

	i2pSAMAddr := os.Getenv("I2P_SAM_ADDR")
	if i2pSAMAddr == "" {
		i2pSAMAddr = DefaultI2PSAMAddr
	}

	return &Config{
		ClearnetEnabled:   true,
		ClearnetAddress:   DefaultClearnetAddress,
		ClearnetPort:      DefaultClearnetPort,
		OnionEnabled:      false,
		TorControlAddr:    torControlAddr,
		I2PEnabled:        false,
		I2PSAMAddr:        i2pSAMAddr,
		StartupTimeout:    30 * time.Second,
		IterationInterval: 50 * time.Millisecond,
	}
}
