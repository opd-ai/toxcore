package bootstrap

import (
	"os"
	"time"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultClearnetPort is the standard Tox bootstrap port.
	DefaultClearnetPort = uint16(33445)

	// DefaultI2PSAMAddr is the default I2P SAM-bridge address.
	// Override with the I2P_SAM_ADDR environment variable or Config.I2PSAMAddr.
	DefaultI2PSAMAddr = "127.0.0.1:7656"
)

// RelayServerConfig holds configuration for a TCP relay server.
type RelayServerConfig struct {
	// Address is the hostname or IP address of the relay server.
	Address string

	// Port is the TCP port of the relay server.
	Port uint16

	// PublicKey is the 32-byte public key of the relay server.
	PublicKey [32]byte

	// Priority determines the order in which relay servers are tried.
	// Lower values have higher priority (tried first).
	Priority int
}

// ToRelayServerInfo converts a RelayServerConfig to transport.RelayServerInfo.
func (c *RelayServerConfig) ToRelayServerInfo() transport.RelayServerInfo {
	return transport.RelayServerInfo{
		Address:   c.Address,
		Port:      c.Port,
		PublicKey: c.PublicKey,
		Priority:  c.Priority,
	}
}

// Config holds configuration for a multi-network bootstrap server.
// Use DefaultConfig to obtain a valid starting point, then customise as needed.
type Config struct {
	// --- Clearnet (UDP) ---

	// ClearnetEnabled controls whether the UDP bootstrap service is started.
	// Default: true.
	ClearnetEnabled bool

	// ClearnetPort is the UDP port to bind.
	// The Tox ecosystem conventionally uses 33445.
	// A value of 0 lets the OS pick an available port.
	// Default: 33445.
	ClearnetPort uint16

	// --- Tor / Onion service ---

	// OnionEnabled controls whether a Tor hidden-service endpoint is started.
	// Requires a running Tor daemon. The onramp library manages Tor internally
	// and reads configuration from the TOR_CONTROL_ADDR environment variable
	// (default: 127.0.0.1:9051).
	// Default: false.
	OnionEnabled bool

	// --- I2P ---

	// I2PEnabled controls whether an I2P destination endpoint is started.
	// Requires a running I2P router with the SAM bridge enabled.
	// Default: false.
	I2PEnabled bool

	// I2PSAMAddr is the address of the I2P SAM bridge.
	// When non-empty, takes precedence over the I2P_SAM_ADDR environment variable.
	// Default: value of I2P_SAM_ADDR env var, or "127.0.0.1:7656".
	I2PSAMAddr string

	// --- Relay servers ---

	// RelayServers is a list of TCP relay servers for symmetric NAT fallback.
	// When direct UDP connections fail, clients can use these relays.
	// Default: empty (no relay servers configured).
	RelayServers []RelayServerConfig

	// RelayEnabled controls whether relay fallback is enabled by default.
	// When true and relay servers are configured, connections that fail via
	// direct UDP will attempt to connect through relay servers.
	// Default: false.
	RelayEnabled bool

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

	// Logger is the logrus logger to use.
	// If nil, logrus.StandardLogger() (the global logrus logger) is used.
	Logger *logrus.Logger
}

// DefaultConfig returns a Config with sensible defaults.
// Only clearnet is enabled by default; set OnionEnabled and/or I2PEnabled to
// true to activate those networks.
func DefaultConfig() *Config {
	i2pSAMAddr := os.Getenv("I2P_SAM_ADDR")
	if i2pSAMAddr == "" {
		i2pSAMAddr = DefaultI2PSAMAddr
	}

	return &Config{
		ClearnetEnabled:   true,
		ClearnetPort:      DefaultClearnetPort,
		OnionEnabled:      false,
		I2PEnabled:        false,
		I2PSAMAddr:        i2pSAMAddr,
		RelayServers:      nil,
		RelayEnabled:      false,
		StartupTimeout:    30 * time.Second,
		IterationInterval: 50 * time.Millisecond,
	}
}
