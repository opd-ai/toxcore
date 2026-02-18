package toxcore

import (
	"testing"
)

// TestFileManagerIntegration verifies that the file manager is properly
// initialized and integrated with the Tox instance.
func TestFileManagerIntegration(t *testing.T) {
	// Create options with UDP disabled to avoid port conflicts
	options := NewOptions()
	options.UDPEnabled = false
	options.LocalDiscovery = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// FileManager is created even without transport - it just can't send packets
	// This allows the manager to be configured and ready when transport becomes available
	fileManager := tox.FileManager()
	if fileManager == nil {
		t.Errorf("Expected fileManager to be initialized (even without transport)")
	}
}

// TestFileManagerWithTransport verifies the file manager is initialized
// when transport is available.
func TestFileManagerWithTransport(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.LocalDiscovery = false
	options.StartPort = 33480 // Use non-default port to avoid conflicts
	options.EndPort = 33490

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test that FileManager() returns a valid manager when transport is available
	fileManager := tox.FileManager()
	if fileManager == nil {
		t.Errorf("Expected fileManager to be initialized when UDP is enabled")
	}
}

// TestFileManagerCleanup verifies that fileManager is properly cleaned up
// when Kill() is called.
func TestFileManagerCleanup(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.LocalDiscovery = false
	options.StartPort = 33491
	options.EndPort = 33499

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Verify fileManager is set before Kill
	if tox.fileManager == nil {
		t.Errorf("Expected fileManager to be set before Kill")
	}

	tox.Kill()

	// Verify fileManager is nil after Kill
	if tox.fileManager != nil {
		t.Errorf("Expected fileManager to be nil after Kill")
	}
}
