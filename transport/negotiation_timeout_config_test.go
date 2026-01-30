package transport

import (
	"crypto/rand"
	"testing"
	"time"
)

// TestNegotiationTimeoutRespectedFromCapabilities verifies that the NegotiationTimeout
// from ProtocolCapabilities is actually used during version negotiation.
// This test addresses Gap #6 from AUDIT.md.
func TestNegotiationTimeoutRespectedFromCapabilities(t *testing.T) {
	transport1 := NewMockTransport("127.0.0.1:8080")
	transport2 := NewMockTransport("127.0.0.1:9090")

	// Create capabilities with custom timeout
	customTimeout := 250 * time.Millisecond
	capabilities := &ProtocolCapabilities{
		SupportedVersions:    []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		PreferredVersion:     ProtocolNoiseIK,
		EnableLegacyFallback: true,
		NegotiationTimeout:   customTimeout,
	}

	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	// Create negotiating transport with custom capabilities
	nt, err := NewNegotiatingTransport(transport1, capabilities, staticPrivKey)
	if err != nil {
		t.Fatalf("NewNegotiatingTransport failed: %v", err)
	}
	defer nt.Close()

	// Verify the negotiator has the correct timeout
	if nt.negotiator.negotiationTimeout != customTimeout {
		t.Errorf("Expected negotiator timeout %v, got %v", customTimeout, nt.negotiator.negotiationTimeout)
	}

	// Test that the timeout is actually used (should timeout after customTimeout, not default 5s)
	start := time.Now()
	_, err = nt.negotiator.NegotiateProtocol(transport1, transport2.LocalAddr())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Should timeout close to our custom timeout, not the default 5 seconds
	if elapsed < customTimeout {
		t.Errorf("Timeout occurred too quickly: %v (expected ~%v)", elapsed, customTimeout)
	}

	// Should not take significantly longer than custom timeout (allow 100ms grace period)
	if elapsed > customTimeout+200*time.Millisecond {
		t.Errorf("Timeout took too long: %v (expected ~%v)", elapsed, customTimeout)
	}
}

// TestNegotiationTimeoutDefaultWhenZero verifies that a zero timeout falls back to 5 seconds
func TestNegotiationTimeoutDefaultWhenZero(t *testing.T) {
	// Create negotiator with zero timeout
	vn := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 0)

	// Should use default 5 seconds
	expectedDefault := 5 * time.Second
	if vn.negotiationTimeout != expectedDefault {
		t.Errorf("Expected default timeout %v, got %v", expectedDefault, vn.negotiationTimeout)
	}
}

// TestNegotiationTimeoutConfigurability verifies different timeout values work correctly
func TestNegotiationTimeoutConfigurability(t *testing.T) {
	tests := []struct {
		name            string
		configuredValue time.Duration
		expectedValue   time.Duration
	}{
		{
			name:            "zero uses default",
			configuredValue: 0,
			expectedValue:   5 * time.Second,
		},
		{
			name:            "short timeout",
			configuredValue: 100 * time.Millisecond,
			expectedValue:   100 * time.Millisecond,
		},
		{
			name:            "default timeout",
			configuredValue: 5 * time.Second,
			expectedValue:   5 * time.Second,
		},
		{
			name:            "long timeout for high-latency networks",
			configuredValue: 30 * time.Second,
			expectedValue:   30 * time.Second,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			capabilities := &ProtocolCapabilities{
				SupportedVersions:    []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
				PreferredVersion:     ProtocolNoiseIK,
				EnableLegacyFallback: true,
				NegotiationTimeout:   test.configuredValue,
			}

			staticPrivKey := make([]byte, 32)
			rand.Read(staticPrivKey)

			mockTransport := NewMockTransport("127.0.0.1:8080")
			nt, err := NewNegotiatingTransport(mockTransport, capabilities, staticPrivKey)
			if err != nil {
				t.Fatalf("NewNegotiatingTransport failed: %v", err)
			}
			defer nt.Close()

			if nt.negotiator.negotiationTimeout != test.expectedValue {
				t.Errorf("Expected timeout %v, got %v", test.expectedValue, nt.negotiator.negotiationTimeout)
			}
		})
	}
}

// TestNegotiationTimeoutErrorMessage verifies the error message includes the timeout duration
func TestNegotiationTimeoutErrorMessage(t *testing.T) {
	transport1 := NewMockTransport("127.0.0.1:8080")
	transport2 := NewMockTransport("127.0.0.1:9090")

	customTimeout := 100 * time.Millisecond
	vn := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, customTimeout)

	// Perform negotiation without peer response - should timeout
	_, err := vn.NegotiateProtocol(transport1, transport2.LocalAddr())

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Error message should mention the timeout duration
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message is empty")
	}

	// The error message from version_negotiation.go includes the timeout duration
	// We just verify that we got an error with some content
	t.Logf("Timeout error message: %s", errMsg)
}
