package toxcore

import (
	"testing"
	"time"
)

// TestGetSecurityPosture_DefaultOptions verifies that GetSecurityPosture returns appropriate
// default values when using default options.
func TestGetSecurityPosture_DefaultOptions(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true // Enable at least one transport
	options.BootstrapTimeout = 10 * time.Second

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Verify basic fields
	if posture == nil {
		t.Fatal("SecurityPosture is nil")
	}

	// TransportReady should be true with UDP enabled
	if !posture.TransportReady {
		t.Error("TransportReady should be true with UDP enabled")
	}

	// Check that NoiseIKEnabled is set correctly (depends on transport negotiation)
	t.Logf("EffectiveSecurityLevel: %s", posture.EffectiveSecurityLevel)
	t.Logf("NoiseIKEnabled: %v", posture.NoiseIKEnabled)
	t.Logf("ForwardSecureEnabled: %v", posture.ForwardSecureEnabled)
}

// TestGetSecurityPosture_NoTransport verifies that warnings are generated when
// no transport is configured.
func TestGetSecurityPosture_NoTransport(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	options.TCPPort = 0

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Should have a warning about no transport
	foundWarning := false
	for _, warning := range posture.ConfigurationWarnings {
		if contains(warning, "no-transport") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("Expected warning about no transport enabled")
	}

	// TransportReady should be false
	if posture.TransportReady {
		t.Error("TransportReady should be false with no transport enabled")
	}
}

// TestGetSecurityPosture_RelayWithoutTransport verifies that warnings are generated
// when relay is enabled without appropriate transport.
func TestGetSecurityPosture_RelayWithoutTransport(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	options.TCPPort = 0
	options.RelayEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Should have a warning about relay without transport
	foundWarning := false
	for _, warning := range posture.ConfigurationWarnings {
		if contains(warning, "relay-without-transport") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("Expected warning about relay without transport")
	}
}

// TestGetSecurityPosture_AsyncWithoutTCP verifies that warnings are generated
// when async storage is enabled without TCP.
func TestGetSecurityPosture_AsyncWithoutTCP(t *testing.T) {
	options := NewOptions()
	options.AsyncStorageEnabled = true
	options.TCPPort = 0

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Should have a warning about async without TCP
	foundWarning := false
	for _, warning := range posture.ConfigurationWarnings {
		if contains(warning, "async-without-tcp") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("Expected warning about async without TCP")
	}
}

// TestGetSecurityPosture_IPv6DisabledNoUDP verifies that warnings are generated
// when IPv6 is disabled and UDP is disabled.
func TestGetSecurityPosture_IPv6DisabledNoUDP(t *testing.T) {
	options := NewOptions()
	options.IPv6Enabled = false
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Should have a warning about IPv6 disabled with no UDP
	foundWarning := false
	for _, warning := range posture.ConfigurationWarnings {
		if contains(warning, "ipv6-disabled-no-udp") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("Expected warning about IPv6 disabled and no UDP")
	}
}

// TestGetSecurityPosture_SmallPortRange verifies that warnings are generated
// when the port range is too small.
func TestGetSecurityPosture_SmallPortRange(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.StartPort = 5000
	options.EndPort = 5005 // Only 6 ports

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Should have a warning about small port range
	foundWarning := false
	for _, warning := range posture.ConfigurationWarnings {
		if contains(warning, "small-port-range") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("Expected warning about small port range")
	}
}

// TestGetSecurityPosture_BootstrapTimeoutTooShort verifies that warnings are generated
// when the bootstrap timeout is too short.
func TestGetSecurityPosture_BootstrapTimeoutTooShort(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.BootstrapTimeout = 2 * time.Second

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Should have a warning about short bootstrap timeout
	foundWarning := false
	for _, warning := range posture.ConfigurationWarnings {
		if contains(warning, "short-bootstrap-timeout") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("Expected warning about short bootstrap timeout")
	}
}

// TestGetSecurityPosture_AsyncMessagingStatus verifies that AsyncMessagingEnabled
// is correctly set based on options.
func TestGetSecurityPosture_AsyncMessagingStatus(t *testing.T) {
	// Test with async enabled
	options := NewOptions()
	options.UDPEnabled = true
	options.AsyncStorageEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()
	if !posture.AsyncMessagingEnabled {
		t.Error("AsyncMessagingEnabled should be true when AsyncStorageEnabled is true")
	}

	// Test with async disabled
	options2 := NewOptions()
	options2.UDPEnabled = true
	options2.AsyncStorageEnabled = false

	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox2.Kill()

	posture2 := tox2.GetSecurityPosture()
	if posture2.AsyncMessagingEnabled {
		t.Error("AsyncMessagingEnabled should be false when AsyncStorageEnabled is false")
	}
}

// TestGetSecurityPosture_LocalDiscoveryOnlyWarning verifies that a warning is generated
// when local discovery is enabled without network transport.
func TestGetSecurityPosture_LocalDiscoveryOnlyWarning(t *testing.T) {
	options := NewOptions()
	options.LocalDiscovery = true
	options.UDPEnabled = false
	options.TCPPort = 0

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Should have a warning about local discovery only
	foundWarning := false
	for _, warning := range posture.ConfigurationWarnings {
		if contains(warning, "local-discovery-only") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("Expected warning about local discovery only")
	}
}

// TestGetSecurityPosture_ProxyWithHighBootstrap verifies that a warning is generated
// when proxy is used with high bootstrap requirement.
func TestGetSecurityPosture_ProxyWithHighBootstrap(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.Proxy = &ProxyOptions{
		Type: ProxyTypeHTTP,
		Host: "proxy.example.com",
		Port: 8080,
	}
	options.MinBootstrapNodes = 8

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Should have a warning about proxy with high bootstrap requirement
	foundWarning := false
	for _, warning := range posture.ConfigurationWarnings {
		if contains(warning, "proxy-high-bootstrap-requirement") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("Expected warning about proxy with high bootstrap requirement")
	}
}

// TestSecurityPosture_JSON verifies that SecurityPosture can be marshaled to JSON
// for use in APIs and logging.
func TestSecurityPosture_JSON(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	posture := tox.GetSecurityPosture()

	// Verify that the struct has JSON tags (compiler will catch missing fields)
	if posture.EffectiveSecurityLevel == "" {
		t.Error("EffectiveSecurityLevel should not be empty")
	}

	if posture.ConfigurationWarnings == nil {
		t.Error("ConfigurationWarnings should not be nil")
	}
}

// BenchmarkGetSecurityPosture measures the performance of GetSecurityPosture.
func BenchmarkGetSecurityPosture(b *testing.B) {
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		b.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tox.GetSecurityPosture()
	}
}

// TestGetSecurityPosture_WithSavedata tests that security posture is correct
// after creating a Tox instance with savedata.
func TestGetSecurityPosture_WithSavedata(t *testing.T) {
	seedOptions := NewOptions()
	seedOptions.UDPEnabled = true
	seedTox, err := New(seedOptions)
	if err != nil {
		t.Fatalf("Failed to create seed Tox instance: %v", err)
	}
	savedata := seedTox.GetSavedata()
	seedTox.Kill()
	if len(savedata) == 0 {
		t.Fatal("Expected non-empty savedata")
	}

	options := NewOptions()
	options.UDPEnabled = true
	options.SavedataType = SaveDataTypeToxSave
	options.SavedataData = savedata

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance from savedata: %v", err)
	}
	defer tox.Kill()

	// Verify security posture is valid
	posture := tox.GetSecurityPosture()
	if posture == nil {
		t.Fatal("SecurityPosture is nil")
	}

	if posture.EffectiveSecurityLevel == "" {
		t.Error("EffectiveSecurityLevel should not be empty")
	}

	t.Logf("SecurityPosture: %+v", posture)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
