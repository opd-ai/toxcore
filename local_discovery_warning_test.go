package toxcore

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestLocalDiscoveryWarning verifies that enabling LocalDiscovery logs a warning
// about the feature not being implemented yet.
func TestLocalDiscoveryWarning(t *testing.T) {
	// Capture log output
	var logBuffer bytes.Buffer
	originalOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logBuffer)
	defer logrus.SetOutput(originalOutput)

	// Set log level to capture warnings
	originalLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.WarnLevel)
	defer logrus.SetLevel(originalLevel)

	// Create options with LocalDiscovery enabled
	options := NewOptions()
	options.LocalDiscovery = true
	options.UDPEnabled = true

	// Create Tox instance
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify warning was logged
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "LocalDiscovery") {
		t.Errorf("Expected warning about LocalDiscovery, but got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "not yet implemented") {
		t.Errorf("Expected warning to mention 'not yet implemented', but got: %s", logOutput)
	}
}

// TestLocalDiscoveryDisabledNoWarning verifies that no warning is logged
// when LocalDiscovery is disabled.
func TestLocalDiscoveryDisabledNoWarning(t *testing.T) {
	// Capture log output
	var logBuffer bytes.Buffer
	originalOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logBuffer)
	defer logrus.SetOutput(originalOutput)

	// Set log level to capture warnings
	originalLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.WarnLevel)
	defer logrus.SetLevel(originalLevel)

	// Create options with LocalDiscovery disabled
	options := NewOptions()
	options.LocalDiscovery = false
	options.UDPEnabled = true

	// Create Tox instance
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify no LocalDiscovery warning was logged
	logOutput := logBuffer.String()
	if strings.Contains(logOutput, "LocalDiscovery") && strings.Contains(logOutput, "not yet implemented") {
		t.Errorf("Expected no warning about LocalDiscovery when disabled, but got: %s", logOutput)
	}
}

// TestLocalDiscoveryDefaultBehavior verifies the default value of LocalDiscovery
// and ensures proper behavior with default options.
func TestLocalDiscoveryDefaultBehavior(t *testing.T) {
	options := NewOptions()

	// Verify default value
	if !options.LocalDiscovery {
		t.Error("Expected LocalDiscovery to default to true")
	}

	// Capture log output
	var logBuffer bytes.Buffer
	originalOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logBuffer)
	defer logrus.SetOutput(originalOutput)

	// Set log level to capture warnings
	originalLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.WarnLevel)
	defer logrus.SetLevel(originalLevel)

	// Create Tox instance with default options
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify warning was logged since default is true
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "LocalDiscovery") || !strings.Contains(logOutput, "not yet implemented") {
		t.Errorf("Expected warning about LocalDiscovery with default options, but got: %s", logOutput)
	}
}
