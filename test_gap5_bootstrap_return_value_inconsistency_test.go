package toxcore

import (
	"testing"
)

// TestGap5BootstrapReturnValueInconsistency is a regression test ensuring that Bootstrap method
// handles transient failures gracefully as documented in README.md while still returning errors for permanent issues
// This addresses Gap #5 from AUDIT.md - bootstrap method return value inconsistency
func TestGap5BootstrapReturnValueInconsistency(t *testing.T) {
	tox, err := New(NewOptions())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test 1: Invalid domain should now be handled gracefully (no error returned)
	// This represents a transient DNS issue that should not disrupt the application
	err1 := tox.Bootstrap("invalid.domain.example", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")

	// Test 2: Valid domain but invalid public key should still return error (permanent issue)
	err2 := tox.Bootstrap("google.com", 33445, "invalid_public_key")

	// After the fix: DNS resolution failures should be handled gracefully
	if err1 != nil {
		t.Errorf("Expected graceful handling (nil error) for DNS resolution failure, but got: %v", err1)
	} else {
		t.Log("DNS resolution failure handled gracefully as documented (no error returned)")
	}

	// Invalid public key should still return an error (permanent configuration issue)
	if err2 == nil {
		t.Error("Expected error for invalid public key, but got nil")
	} else {
		t.Logf("Invalid public key correctly returns error: %v", err2)
	}

	// Verify the behavior now matches the documentation pattern:
	// Transient issues (DNS) are warnings, permanent issues (invalid config) are errors
	t.Log("Bootstrap method now handles failures according to documentation")
}
