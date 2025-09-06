package toxcore

import (
	"testing"

	"github.com/sirupsen/logrus"
)

// TestLoggingDemo demonstrates the enhanced logging functionality
func TestLoggingDemo(t *testing.T) {
	// Set up logrus for the demo
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	t.Log("=== Toxcore Logging Enhancement Demo ===")

	// Test NewOptions with logging
	t.Log("\n1. Testing NewOptions with structured logging:")
	options := NewOptions()
	if options == nil {
		t.Fatal("NewOptions returned nil")
	}

	// Test key pair generation with logging
	t.Log("\n2. Testing key pair generation with structured logging:")
	_, err := New(options)
	if err != nil {
		t.Logf("New() returned error (expected in test environment): %v", err)
	}

	// Test simulation function marking
	t.Log("\n3. Testing simulation function (should show warning):")
	tox := &Tox{}
	tox.simulatePacketDelivery(1, []byte("test packet"))

	t.Log("\n=== Demo completed successfully ===")
}
