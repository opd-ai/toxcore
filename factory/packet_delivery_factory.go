package factory

import (
	"fmt"
	"os"
	"strconv"

	"github.com/opd-ai/toxcore/interfaces"
	"github.com/opd-ai/toxcore/real"
	"github.com/opd-ai/toxcore/testing"
	"github.com/sirupsen/logrus"
)

// PacketDeliveryFactory creates packet delivery implementations based on configuration
type PacketDeliveryFactory struct {
	defaultConfig *interfaces.PacketDeliveryConfig
}

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
// It safely parses the boolean value and only updates config if parsing succeeds.
func parseSimulationSetting(config *interfaces.PacketDeliveryConfig) {
	if useSimStr := os.Getenv("TOX_USE_SIMULATION"); useSimStr != "" {
		if useSim, err := strconv.ParseBool(useSimStr); err == nil {
			config.UseSimulation = useSim
		}
	}
}

// parseTimeoutSetting updates the NetworkTimeout config from TOX_NETWORK_TIMEOUT environment variable.
// It safely parses the integer value and only updates config if parsing succeeds.
func parseTimeoutSetting(config *interfaces.PacketDeliveryConfig) {
	if timeoutStr := os.Getenv("TOX_NETWORK_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			config.NetworkTimeout = timeout
		}
	}
}

// parseRetrySetting updates the RetryAttempts config from TOX_RETRY_ATTEMPTS environment variable.
// It safely parses the integer value and only updates config if parsing succeeds.
func parseRetrySetting(config *interfaces.PacketDeliveryConfig) {
	if retriesStr := os.Getenv("TOX_RETRY_ATTEMPTS"); retriesStr != "" {
		if retries, err := strconv.Atoi(retriesStr); err == nil {
			config.RetryAttempts = retries
		}
	}
}

// parseBroadcastSetting updates the EnableBroadcast config from TOX_ENABLE_BROADCAST environment variable.
// It safely parses the boolean value and only updates config if parsing succeeds.
func parseBroadcastSetting(config *interfaces.PacketDeliveryConfig) {
	if broadcastStr := os.Getenv("TOX_ENABLE_BROADCAST"); broadcastStr != "" {
		if broadcast, err := strconv.ParseBool(broadcastStr); err == nil {
			config.EnableBroadcast = broadcast
		}
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
	return f.CreatePacketDeliveryWithConfig(transport, f.defaultConfig)
}

// CreatePacketDeliveryWithConfig creates a packet delivery implementation with custom configuration
func (f *PacketDeliveryFactory) CreatePacketDeliveryWithConfig(transport interfaces.INetworkTransport, config *interfaces.PacketDeliveryConfig) (interfaces.IPacketDelivery, error) {
	if config == nil {
		config = f.defaultConfig
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

// CreateSimulationForTesting creates a simulation implementation specifically for testing
func (f *PacketDeliveryFactory) CreateSimulationForTesting() interfaces.IPacketDelivery {
	testConfig := &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  1000, // Shorter timeout for testing
		RetryAttempts:   1,    // Single attempt for testing
		EnableBroadcast: true,
	}

	logrus.WithFields(logrus.Fields{
		"function": "CreateSimulationForTesting",
		"config":   testConfig,
	}).Info("Creating simulation implementation for testing")

	return testing.NewSimulatedPacketDelivery(testConfig)
}

// SwitchToSimulation switches the configuration to use simulation
func (f *PacketDeliveryFactory) SwitchToSimulation() {
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
	return &interfaces.PacketDeliveryConfig{
		UseSimulation:   f.defaultConfig.UseSimulation,
		NetworkTimeout:  f.defaultConfig.NetworkTimeout,
		RetryAttempts:   f.defaultConfig.RetryAttempts,
		EnableBroadcast: f.defaultConfig.EnableBroadcast,
	}
}

// IsUsingSimulation returns true if the factory is configured for simulation
func (f *PacketDeliveryFactory) IsUsingSimulation() bool {
	return f.defaultConfig.UseSimulation
}

// UpdateConfig updates the factory's default configuration
func (f *PacketDeliveryFactory) UpdateConfig(config *interfaces.PacketDeliveryConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

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
