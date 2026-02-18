package factory

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/opd-ai/toxcore/interfaces"
	"github.com/opd-ai/toxcore/real"
	"github.com/opd-ai/toxcore/testing"
	"github.com/sirupsen/logrus"
)

// Validation constants for configuration bounds checking.
const (
	// MinNetworkTimeout is the minimum allowed network timeout in milliseconds.
	MinNetworkTimeout = 100
	// MaxNetworkTimeout is the maximum allowed network timeout in milliseconds (10 minutes).
	MaxNetworkTimeout = 600000
	// MinRetryAttempts is the minimum allowed retry attempts.
	MinRetryAttempts = 0
	// MaxRetryAttempts is the maximum allowed retry attempts.
	MaxRetryAttempts = 100
)

// PacketDeliveryFactory creates packet delivery implementations based on configuration.
// It is safe for concurrent use; all methods are protected by an internal mutex.
type PacketDeliveryFactory struct {
	mu            sync.RWMutex
	defaultConfig *interfaces.PacketDeliveryConfig
}

// TestConfigOption is a functional option for customizing test simulation configuration.
type TestConfigOption func(*interfaces.PacketDeliveryConfig)

// NewPacketDeliveryFactory creates a new factory with default configuration
func NewPacketDeliveryFactory() *PacketDeliveryFactory {
	defaultConfig := createDefaultConfig()
	applyEnvironmentOverrides(defaultConfig)
	logConfigurationInfo(defaultConfig)

	return &PacketDeliveryFactory{
		defaultConfig: defaultConfig,
	}
}

// createDefaultConfig initializes the default packet delivery configuration.
// It sets up sensible defaults for all configuration parameters.
//
// Default Value Rationale:
//   - UseSimulation: false - Production mode by default; simulation must be explicitly enabled
//   - NetworkTimeout: 5000ms - Balances responsiveness with allowing time for network latency
//   - RetryAttempts: 3 - Standard retry count that handles transient failures without excessive delays
//   - EnableBroadcast: true - Broadcast is generally desired for peer discovery and announcements
func createDefaultConfig() *interfaces.PacketDeliveryConfig {
	return &interfaces.PacketDeliveryConfig{
		UseSimulation:   false, // Default to real implementation
		NetworkTimeout:  5000,  // 5 seconds
		RetryAttempts:   3,
		EnableBroadcast: true,
	}
}

// applyEnvironmentOverrides updates configuration based on environment variables.
// It checks for TOX_* environment variables and overrides defaults if valid values are found.
func applyEnvironmentOverrides(config *interfaces.PacketDeliveryConfig) {
	parseSimulationSetting(config)
	parseTimeoutSetting(config)
	parseRetrySetting(config)
	parseBroadcastSetting(config)
}

// parseSimulationSetting updates the UseSimulation config from TOX_USE_SIMULATION environment variable.
// It safely parses the boolean value, logs a warning if parsing fails, and only updates config if parsing succeeds.
func parseSimulationSetting(config *interfaces.PacketDeliveryConfig) {
	if useSimStr := os.Getenv("TOX_USE_SIMULATION"); useSimStr != "" {
		useSim, err := strconv.ParseBool(useSimStr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":    "parseSimulationSetting",
				"env_var":     "TOX_USE_SIMULATION",
				"value":       useSimStr,
				"error":       err.Error(),
				"using_value": config.UseSimulation,
			}).Warn("Failed to parse TOX_USE_SIMULATION environment variable, using default")
			return
		}
		config.UseSimulation = useSim
	}
}

// parseTimeoutSetting updates the NetworkTimeout config from TOX_NETWORK_TIMEOUT environment variable.
// It validates the value is within bounds [MinNetworkTimeout, MaxNetworkTimeout] and logs warnings for
// invalid values. Only updates config if parsing succeeds and value is within valid range.
func parseTimeoutSetting(config *interfaces.PacketDeliveryConfig) {
	if timeoutStr := os.Getenv("TOX_NETWORK_TIMEOUT"); timeoutStr != "" {
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":    "parseTimeoutSetting",
				"env_var":     "TOX_NETWORK_TIMEOUT",
				"value":       timeoutStr,
				"error":       err.Error(),
				"using_value": config.NetworkTimeout,
			}).Warn("Failed to parse TOX_NETWORK_TIMEOUT environment variable, using default")
			return
		}
		if timeout < MinNetworkTimeout || timeout > MaxNetworkTimeout {
			logrus.WithFields(logrus.Fields{
				"function":    "parseTimeoutSetting",
				"env_var":     "TOX_NETWORK_TIMEOUT",
				"value":       timeout,
				"min":         MinNetworkTimeout,
				"max":         MaxNetworkTimeout,
				"using_value": config.NetworkTimeout,
			}).Warn("TOX_NETWORK_TIMEOUT value out of bounds, using default")
			return
		}
		config.NetworkTimeout = timeout
	}
}

// parseRetrySetting updates the RetryAttempts config from TOX_RETRY_ATTEMPTS environment variable.
// It validates the value is within bounds [MinRetryAttempts, MaxRetryAttempts] and logs warnings for
// invalid values. Only updates config if parsing succeeds and value is within valid range.
func parseRetrySetting(config *interfaces.PacketDeliveryConfig) {
	if retriesStr := os.Getenv("TOX_RETRY_ATTEMPTS"); retriesStr != "" {
		retries, err := strconv.Atoi(retriesStr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":    "parseRetrySetting",
				"env_var":     "TOX_RETRY_ATTEMPTS",
				"value":       retriesStr,
				"error":       err.Error(),
				"using_value": config.RetryAttempts,
			}).Warn("Failed to parse TOX_RETRY_ATTEMPTS environment variable, using default")
			return
		}
		if retries < MinRetryAttempts || retries > MaxRetryAttempts {
			logrus.WithFields(logrus.Fields{
				"function":    "parseRetrySetting",
				"env_var":     "TOX_RETRY_ATTEMPTS",
				"value":       retries,
				"min":         MinRetryAttempts,
				"max":         MaxRetryAttempts,
				"using_value": config.RetryAttempts,
			}).Warn("TOX_RETRY_ATTEMPTS value out of bounds, using default")
			return
		}
		config.RetryAttempts = retries
	}
}

// parseBroadcastSetting updates the EnableBroadcast config from TOX_ENABLE_BROADCAST environment variable.
// It safely parses the boolean value, logs a warning if parsing fails, and only updates config if parsing succeeds.
func parseBroadcastSetting(config *interfaces.PacketDeliveryConfig) {
	if broadcastStr := os.Getenv("TOX_ENABLE_BROADCAST"); broadcastStr != "" {
		broadcast, err := strconv.ParseBool(broadcastStr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":    "parseBroadcastSetting",
				"env_var":     "TOX_ENABLE_BROADCAST",
				"value":       broadcastStr,
				"error":       err.Error(),
				"using_value": config.EnableBroadcast,
			}).Warn("Failed to parse TOX_ENABLE_BROADCAST environment variable, using default")
			return
		}
		config.EnableBroadcast = broadcast
	}
}

// logConfigurationInfo logs the final configuration settings for debugging purposes.
// It provides structured logging of all configuration parameters.
func logConfigurationInfo(config *interfaces.PacketDeliveryConfig) {
	logrus.WithFields(logrus.Fields{
		"function":         "NewPacketDeliveryFactory",
		"use_simulation":   config.UseSimulation,
		"network_timeout":  config.NetworkTimeout,
		"retry_attempts":   config.RetryAttempts,
		"enable_broadcast": config.EnableBroadcast,
	}).Info("Created packet delivery factory with configuration")
}

// CreatePacketDelivery creates a packet delivery implementation based on configuration
func (f *PacketDeliveryFactory) CreatePacketDelivery(transport interfaces.INetworkTransport) (interfaces.IPacketDelivery, error) {
	f.mu.RLock()
	config := f.defaultConfig
	f.mu.RUnlock()
	return f.CreatePacketDeliveryWithConfig(transport, config)
}

// CreatePacketDeliveryWithConfig creates a packet delivery implementation with custom configuration
func (f *PacketDeliveryFactory) CreatePacketDeliveryWithConfig(transport interfaces.INetworkTransport, config *interfaces.PacketDeliveryConfig) (interfaces.IPacketDelivery, error) {
	if config == nil {
		f.mu.RLock()
		config = f.defaultConfig
		f.mu.RUnlock()
	}

	logrus.WithFields(logrus.Fields{
		"function":         "CreatePacketDeliveryWithConfig",
		"use_simulation":   config.UseSimulation,
		"network_timeout":  config.NetworkTimeout,
		"retry_attempts":   config.RetryAttempts,
		"enable_broadcast": config.EnableBroadcast,
	}).Info("Creating packet delivery implementation")

	if config.UseSimulation {
		logrus.WithFields(logrus.Fields{
			"function": "CreatePacketDeliveryWithConfig",
			"type":     "simulation",
		}).Info("Creating simulation packet delivery implementation")

		return testing.NewSimulatedPacketDelivery(config), nil
	}

	if transport == nil {
		return nil, fmt.Errorf("transport is required for real packet delivery implementation")
	}

	logrus.WithFields(logrus.Fields{
		"function": "CreatePacketDeliveryWithConfig",
		"type":     "real",
	}).Info("Creating real packet delivery implementation")

	return real.NewRealPacketDelivery(transport, config), nil
}

// WithNetworkTimeout sets a custom network timeout for the test configuration.
func WithNetworkTimeout(timeout int) TestConfigOption {
	return func(c *interfaces.PacketDeliveryConfig) {
		c.NetworkTimeout = timeout
	}
}

// WithRetryAttempts sets custom retry attempts for the test configuration.
func WithRetryAttempts(retries int) TestConfigOption {
	return func(c *interfaces.PacketDeliveryConfig) {
		c.RetryAttempts = retries
	}
}

// WithBroadcast enables or disables broadcast for the test configuration.
func WithBroadcast(enabled bool) TestConfigOption {
	return func(c *interfaces.PacketDeliveryConfig) {
		c.EnableBroadcast = enabled
	}
}

// CreateSimulationForTesting creates a simulation implementation specifically for testing.
// It accepts optional TestConfigOption functions to override default test values.
// Default test configuration uses: NetworkTimeout=1000ms, RetryAttempts=1, EnableBroadcast=true.
func (f *PacketDeliveryFactory) CreateSimulationForTesting(opts ...TestConfigOption) interfaces.IPacketDelivery {
	testConfig := &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  1000, // Shorter timeout for testing
		RetryAttempts:   1,    // Single attempt for testing
		EnableBroadcast: true,
	}

	// Apply optional overrides
	for _, opt := range opts {
		opt(testConfig)
	}

	logrus.WithFields(logrus.Fields{
		"function":         "CreateSimulationForTesting",
		"network_timeout":  testConfig.NetworkTimeout,
		"retry_attempts":   testConfig.RetryAttempts,
		"enable_broadcast": testConfig.EnableBroadcast,
	}).Info("Creating simulation implementation for testing")

	return testing.NewSimulatedPacketDelivery(testConfig)
}

// SwitchToSimulation switches the configuration to use simulation
func (f *PacketDeliveryFactory) SwitchToSimulation() {
	f.mu.Lock()
	defer f.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "SwitchToSimulation",
		"previous": f.defaultConfig.UseSimulation,
	}).Info("Switching factory to simulation mode")

	f.defaultConfig.UseSimulation = true

	logrus.WithFields(logrus.Fields{
		"function": "SwitchToSimulation",
		"current":  f.defaultConfig.UseSimulation,
	}).Info("Factory switched to simulation mode")
}

// SwitchToReal switches the configuration to use real implementation
func (f *PacketDeliveryFactory) SwitchToReal() {
	f.mu.Lock()
	defer f.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "SwitchToReal",
		"previous": f.defaultConfig.UseSimulation,
	}).Info("Switching factory to real mode")

	f.defaultConfig.UseSimulation = false

	logrus.WithFields(logrus.Fields{
		"function": "SwitchToReal",
		"current":  f.defaultConfig.UseSimulation,
	}).Info("Factory switched to real mode")
}

// GetCurrentConfig returns a copy of the current default configuration
func (f *PacketDeliveryFactory) GetCurrentConfig() *interfaces.PacketDeliveryConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return &interfaces.PacketDeliveryConfig{
		UseSimulation:   f.defaultConfig.UseSimulation,
		NetworkTimeout:  f.defaultConfig.NetworkTimeout,
		RetryAttempts:   f.defaultConfig.RetryAttempts,
		EnableBroadcast: f.defaultConfig.EnableBroadcast,
	}
}

// IsUsingSimulation returns true if the factory is configured for simulation
func (f *PacketDeliveryFactory) IsUsingSimulation() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.defaultConfig.UseSimulation
}

// UpdateConfig updates the factory's default configuration
func (f *PacketDeliveryFactory) UpdateConfig(config *interfaces.PacketDeliveryConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":       "UpdateConfig",
		"old_simulation": f.defaultConfig.UseSimulation,
		"new_simulation": config.UseSimulation,
		"old_timeout":    f.defaultConfig.NetworkTimeout,
		"new_timeout":    config.NetworkTimeout,
	}).Info("Updating factory configuration")

	f.defaultConfig = &interfaces.PacketDeliveryConfig{
		UseSimulation:   config.UseSimulation,
		NetworkTimeout:  config.NetworkTimeout,
		RetryAttempts:   config.RetryAttempts,
		EnableBroadcast: config.EnableBroadcast,
	}

	logrus.WithFields(logrus.Fields{
		"function": "UpdateConfig",
		"updated":  true,
	}).Info("Factory configuration updated successfully")

	return nil
}
