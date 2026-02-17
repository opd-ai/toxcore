package internal

import (
	"log"
	"os"
	"testing"
	"time"
)

// TestProtocolConfigStruct tests the ProtocolConfig struct fields.
func TestProtocolConfigStruct(t *testing.T) {
	customLogger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)

	config := &ProtocolConfig{
		BootstrapTimeout:     20 * time.Second,
		ConnectionTimeout:    60 * time.Second,
		FriendRequestTimeout: 30 * time.Second,
		MessageTimeout:       15 * time.Second,
		RetryAttempts:        5,
		RetryBackoff:         2 * time.Second,
		Logger:               customLogger,
	}

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"BootstrapTimeout", config.BootstrapTimeout, 20 * time.Second},
		{"ConnectionTimeout", config.ConnectionTimeout, 60 * time.Second},
		{"FriendRequestTimeout", config.FriendRequestTimeout, 30 * time.Second},
		{"MessageTimeout", config.MessageTimeout, 15 * time.Second},
		{"RetryAttempts", config.RetryAttempts, 5},
		{"RetryBackoff", config.RetryBackoff, 2 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}

	if config.Logger != customLogger {
		t.Error("Logger was not set correctly")
	}
}

// TestProtocolTestSuiteCleanupWithNilComponents tests Cleanup with nil components.
func TestProtocolTestSuiteCleanupWithNilComponents(t *testing.T) {
	suite := NewProtocolTestSuite(nil)

	// Cleanup should not panic with nil components
	err := suite.Cleanup()
	if err != nil {
		// With nil components, there should be no errors
		t.Errorf("Cleanup() with nil components returned error: %v", err)
	}
}

// TestProtocolTestSuiteServerNilAfterCreation tests that server is nil after creation.
func TestProtocolTestSuiteServerNilAfterCreation(t *testing.T) {
	suite := NewProtocolTestSuite(nil)

	if suite.server != nil {
		t.Error("server should be nil after NewProtocolTestSuite")
	}

	if suite.clientA != nil {
		t.Error("clientA should be nil after NewProtocolTestSuite")
	}

	if suite.clientB != nil {
		t.Error("clientB should be nil after NewProtocolTestSuite")
	}
}

// TestProtocolTestSuiteConfigValues tests that config values are properly set.
func TestProtocolTestSuiteConfigValues(t *testing.T) {
	config := &ProtocolConfig{
		BootstrapTimeout:     5 * time.Second,
		ConnectionTimeout:    10 * time.Second,
		FriendRequestTimeout: 8 * time.Second,
		MessageTimeout:       6 * time.Second,
		RetryAttempts:        2,
		RetryBackoff:         500 * time.Millisecond,
		Logger:               log.Default(),
	}

	suite := NewProtocolTestSuite(config)

	if suite.config.BootstrapTimeout != 5*time.Second {
		t.Errorf("BootstrapTimeout = %v, want %v", suite.config.BootstrapTimeout, 5*time.Second)
	}

	if suite.config.RetryAttempts != 2 {
		t.Errorf("RetryAttempts = %d, want %d", suite.config.RetryAttempts, 2)
	}

	if suite.config.RetryBackoff != 500*time.Millisecond {
		t.Errorf("RetryBackoff = %v, want %v", suite.config.RetryBackoff, 500*time.Millisecond)
	}
}

// TestDefaultProtocolConfigNonZeroValues tests that all default values are sensible.
func TestDefaultProtocolConfigNonZeroValues(t *testing.T) {
	config := DefaultProtocolConfig()

	if config.BootstrapTimeout <= 0 {
		t.Error("BootstrapTimeout should be positive")
	}

	if config.ConnectionTimeout <= 0 {
		t.Error("ConnectionTimeout should be positive")
	}

	if config.FriendRequestTimeout <= 0 {
		t.Error("FriendRequestTimeout should be positive")
	}

	if config.MessageTimeout <= 0 {
		t.Error("MessageTimeout should be positive")
	}

	if config.RetryAttempts <= 0 {
		t.Error("RetryAttempts should be positive")
	}

	if config.RetryBackoff <= 0 {
		t.Error("RetryBackoff should be positive")
	}
}

// TestProtocolTestSuiteLoggerInheritance tests that logger is inherited from config.
func TestProtocolTestSuiteLoggerInheritance(t *testing.T) {
	customLogger := log.New(os.Stderr, "[CUSTOM] ", log.LstdFlags)
	config := DefaultProtocolConfig()
	config.Logger = customLogger

	suite := NewProtocolTestSuite(config)

	if suite.logger != customLogger {
		t.Error("Logger should be inherited from config")
	}
}

// TestProtocolTestSuiteDefaultLogger tests that default logger is used when config has nil logger.
func TestProtocolTestSuiteDefaultLogger(t *testing.T) {
	config := &ProtocolConfig{
		BootstrapTimeout:     10 * time.Second,
		ConnectionTimeout:    30 * time.Second,
		FriendRequestTimeout: 15 * time.Second,
		MessageTimeout:       10 * time.Second,
		RetryAttempts:        3,
		RetryBackoff:         time.Second,
		Logger:               nil, // Explicitly nil
	}

	suite := NewProtocolTestSuite(config)

	// Logger should be nil from config
	if suite.logger != nil {
		t.Error("Logger should be nil when config.Logger is nil")
	}
}

// TestDefaultProtocolConfigLogger tests that default config has non-nil logger.
func TestDefaultProtocolConfigLogger(t *testing.T) {
	config := DefaultProtocolConfig()

	if config.Logger == nil {
		t.Error("Default config should have non-nil Logger")
	}
}
