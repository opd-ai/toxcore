package toxcore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opd-ai/toxforge/async"
	"github.com/opd-ai/toxforge/crypto"
	"github.com/opd-ai/toxforge/transport"
)

// TestGap1ConstructorMismatch verifies that the AsyncManager constructor
// can be called with the correct 3-parameter signature that includes transport.
// This test serves as a regression test to ensure the documentation matches
// the implementation.
func TestGap1ConstructorMismatch(t *testing.T) {
	// Generate a key pair for testing
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create a transport (required parameter)
	udpTransport, err := transport.NewUDPTransport("0.0.0.0:0") // Auto-assign port
	if err != nil {
		t.Fatalf("Failed to create UDP transport: %v", err)
	}

	dataDir := filepath.Join(os.TempDir(), "test_async_manager")

	// This should now compile and work with the correct 3-parameter signature
	asyncManager, err := async.NewAsyncManager(keyPair, udpTransport, dataDir)
	if err != nil {
		t.Fatalf("Failed to create AsyncManager: %v", err)
	}

	// Verify the manager was created successfully
	if asyncManager == nil {
		t.Fatal("AsyncManager should not be nil")
	}

	// Clean up
	asyncManager.Stop()
}
