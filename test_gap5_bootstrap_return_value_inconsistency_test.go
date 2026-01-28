package toxcore

import (
	"testing"
)

// TestGap5BootstrapReturnValueInconsistency is a regression test ensuring that Bootstrap method
// returns errors for all failure types to match documentation in README.md
// This addresses Gap #5 from AUDIT.md - bootstrap method return value inconsistency
func TestGap5BootstrapReturnValueInconsistency(t *testing.T) {
	tox, err := New(NewOptions())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test 1: Invalid domain should return error (DNS resolution failure)
	// This allows applications to handle failures appropriately (log, retry, etc.)
	err1 := tox.Bootstrap("invalid.domain.example", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")

	// Test 2: Invalid public key should also return error (configuration issue)
	err2 := tox.Bootstrap("google.com", 33445, "invalid_public_key")

	// After the fix: Both DNS resolution failures and invalid config should return errors
	if err1 == nil {
		t.Error("Expected error for DNS resolution failure, but got nil")
	} else {
		t.Logf("DNS resolution failure correctly returns error: %v", err1)
	}

	// Invalid public key should return an error
	if err2 == nil {
		t.Error("Expected error for invalid public key, but got nil")
	} else {
		t.Logf("Invalid public key correctly returns error: %v", err2)
	}

	// Verify the behavior now matches the documentation pattern:
	// All failures return errors for proper error handling
	t.Log("Bootstrap method now returns errors for all failures, matching documentation")
}
