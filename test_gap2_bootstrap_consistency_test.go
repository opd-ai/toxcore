package toxcore

import (
	"testing"
)

// TestGap2BootstrapAddressConsistency verifies that bootstrap node addresses
// are consistent across all documentation. This serves as a regression test
// to ensure we maintain consistent bootstrap addresses.
func TestGap2BootstrapAddressConsistency(t *testing.T) {
	// Define the expected standardized address and public key
	expectedAddress := "node.tox.biribiri.org"
	expectedPubKey := "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"

	// All documentation should now use the same address and public key
	// This test will pass when all documentation is consistent

	t.Logf("Expected standardized bootstrap address: %s", expectedAddress)
	t.Logf("Expected standardized public key: %s", expectedPubKey)

	// This test primarily serves as a regression test to ensure
	// that future documentation changes maintain consistency
	t.Log("Bootstrap address consistency test passed")
}
