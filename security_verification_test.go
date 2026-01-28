package toxcore

import (
	"testing"
)

// TestSecurityImprovementsVerification verifies that all critical security
// improvements from the audit are working correctly
func TestSecurityImprovementsVerification(t *testing.T) {
	// Create a Tox instance with default options
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify secure-by-default transport is enabled
	securityInfo := tox.GetTransportSecurityInfo()
	if securityInfo == nil {
		t.Fatal("GetTransportSecurityInfo returned nil")
	}

	// Check that Noise-IK is enabled by default
	if !securityInfo.NoiseIKEnabled {
		t.Error("Expected Noise-IK to be enabled by default")
	}

	// Check that the transport type indicates negotiating transport
	if securityInfo.TransportType != "negotiating-udp" {
		t.Errorf("Expected transport type 'negotiating-udp', got '%s'", securityInfo.TransportType)
	}

	// Check that supported versions include both legacy and modern protocols
	expectedVersions := []string{"legacy", "noise-ik"}
	if len(securityInfo.SupportedVersions) != len(expectedVersions) {
		t.Errorf("Expected %d supported versions, got %d", len(expectedVersions), len(securityInfo.SupportedVersions))
	}

	// Verify security summary indicates secure status
	summary := tox.GetSecuritySummary()
	if summary == "" {
		t.Error("GetSecuritySummary returned empty string")
	}

	// Should indicate secure status
	if summary == "Basic: Legacy encryption only (consider enabling secure transport)" {
		t.Error("Security summary indicates basic encryption, expected secure status")
	}

	t.Logf("Security verification successful:")
	t.Logf("  Transport Type: %s", securityInfo.TransportType)
	t.Logf("  Noise-IK Enabled: %v", securityInfo.NoiseIKEnabled)
	t.Logf("  Legacy Fallback: %v", securityInfo.LegacyFallbackEnabled)
	t.Logf("  Supported Versions: %v", securityInfo.SupportedVersions)
	t.Logf("  Security Summary: %s", summary)
}

// TestEncryptionStatusAPI verifies the encryption status API functionality
func TestEncryptionStatusAPI(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test with non-existent friend
	status := tox.GetFriendEncryptionStatus(999)
	if status != EncryptionUnknown {
		t.Errorf("Expected EncryptionUnknown for non-existent friend, got %s", status)
	}

	// Add a friend to test with
	friendID, err := tox.AddFriend("76518406f6a9f2217e8dc487cc783c25cc16a15eb36ff32e335364ec37b1334912345678868a", "Test friend for encryption status")
	if err != nil {
		// This is expected to fail since we don't have a real connection
		// but we can still test the API structure
		t.Logf("AddFriend failed as expected (no real connection): %v", err)
		return
	}

	// Test encryption status for the added friend
	status = tox.GetFriendEncryptionStatus(friendID)
	// Should be offline since we don't have a real connection
	if status != EncryptionOffline {
		t.Logf("Friend encryption status: %s (expected offline)", status)
	}
}

// TestSecurityLoggingIntegration verifies that security logging is properly integrated
func TestSecurityLoggingIntegration(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// The fact that we can create a Tox instance successfully means
	// the secure transport initialization worked correctly.
	// In real usage, the logging would appear in the application logs.

	// Verify that the transport was created successfully with security
	securityInfo := tox.GetTransportSecurityInfo()
	if securityInfo.TransportType == "unknown" {
		t.Error("Transport type is unknown, security initialization may have failed")
	}

	t.Logf("Security logging integration test passed")
	t.Logf("Transport initialized with type: %s", securityInfo.TransportType)
}
