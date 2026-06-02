package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMixedPeerCompatibilityMatrix validates that protocol negotiation
// works correctly for all combinations of peer capabilities.
// This is the acceptance criteria for Task 1.2.
func TestMixedPeerCompatibilityMatrix(t *testing.T) {
	tests := []struct {
		name          string
		ourPolicy     SessionPolicy
		ourVersions   []ProtocolVersion
		peerVersions  []ProtocolVersion
		peerRatchet   RatchetCapability
		expectedMode  SessionMode
		shouldSucceed bool
	}{
		// Scenario 1: Legacy-only peer <-> Legacy-only peer
		{
			name:          "legacy/legacy peers -> legacy mode",
			ourPolicy:     PolicyLegacyOnly,
			ourVersions:   []ProtocolVersion{ProtocolLegacy},
			peerVersions:  []ProtocolVersion{ProtocolLegacy},
			peerRatchet:   RatchetUnsupported,
			expectedMode:  SessionModeLegacy,
			shouldSucceed: true,
		},

		// Scenario 2: Legacy+Noise peer <-> Legacy-only peer
		{
			name:          "legacy+noise/legacy peers -> legacy mode",
			ourPolicy:     PolicyNoiseWithRatchet,
			ourVersions:   []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			peerVersions:  []ProtocolVersion{ProtocolLegacy},
			peerRatchet:   RatchetUnsupported,
			expectedMode:  SessionModeLegacy,
			shouldSucceed: true,
		},

		// Scenario 3: Legacy+Noise peer <-> Noise-only peer
		{
			name:          "legacy+noise/noise peers -> noise mode",
			ourPolicy:     PolicyNoiseWithRatchet,
			ourVersions:   []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			peerVersions:  []ProtocolVersion{ProtocolNoiseIK},
			peerRatchet:   RatchetUnsupported,
			expectedMode:  SessionModeNoise,
			shouldSucceed: true,
		},

		// Scenario 4: Noise-only peer <-> Noise-only peer
		{
			name:          "noise/noise peers -> noise mode",
			ourPolicy:     PolicyNoiseOnly,
			ourVersions:   []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			peerVersions:  []ProtocolVersion{ProtocolNoiseIK},
			peerRatchet:   RatchetUnsupported,
			expectedMode:  SessionModeNoise,
			shouldSucceed: true,
		},

		// Scenario 5: Noise+Ratchet peer <-> Noise-only peer
		{
			name:          "noise+ratchet/noise peers -> noise mode (no ratchet)",
			ourPolicy:     PolicyNoiseWithRatchet,
			ourVersions:   []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			peerVersions:  []ProtocolVersion{ProtocolNoiseIK},
			peerRatchet:   RatchetUnsupported,
			expectedMode:  SessionModeNoise,
			shouldSucceed: true,
		},

		// Scenario 6: Noise+Ratchet peer <-> Legacy-only peer
		{
			name:          "noise+ratchet/legacy peers -> legacy mode (fallback)",
			ourPolicy:     PolicyNoiseWithRatchet,
			ourVersions:   []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			peerVersions:  []ProtocolVersion{ProtocolLegacy},
			peerRatchet:   RatchetUnsupported,
			expectedMode:  SessionModeLegacy,
			shouldSucceed: true,
		},

		// Scenario 7: Noise+Ratchet peer <-> Noise+Ratchet peer
		{
			name:          "noise+ratchet/noise+ratchet peers -> noise+ratchet mode",
			ourPolicy:     PolicyNoiseWithRatchet,
			ourVersions:   []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			peerVersions:  []ProtocolVersion{ProtocolNoiseIK},
			peerRatchet:   RatchetSupported,
			expectedMode:  SessionModeNoiseWithRatchet,
			shouldSucceed: true,
		},

		// Scenario 8: NoiseOnly policy against Noise+Ratchet capable peer
		{
			name:          "noise-only policy/noise+ratchet peer -> noise mode (no ratchet)",
			ourPolicy:     PolicyNoiseOnly,
			ourVersions:   []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			peerVersions:  []ProtocolVersion{ProtocolNoiseIK},
			peerRatchet:   RatchetSupported,
			expectedMode:  SessionModeNoise,
			shouldSucceed: true,
		},

		// Scenario 9: LegacyOnly policy ignores peer capabilities
		{
			name:          "legacy-only policy ignores peer ratchet capability",
			ourPolicy:     PolicyLegacyOnly,
			ourVersions:   []ProtocolVersion{ProtocolLegacy},
			peerVersions:  []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			peerRatchet:   RatchetSupported,
			expectedMode:  SessionModeLegacy,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a policy config based on the policy
			var config *PolicyConfig
			switch tt.ourPolicy {
			case PolicyLegacyOnly:
				cfg := LegacyCompatPolicyConfig()
				config = &cfg
			case PolicyNoiseOnly:
				cfg := DefaultPolicyConfig()
				cfg.DefaultPolicy = PolicyNoiseOnly
				cfg.RequireRatchetForNoise = false
				config = &cfg
			case PolicyNoiseWithRatchet:
				cfg := DefaultPolicyConfig()
				config = &cfg
			}

			// Apply policy to filter versions
			filteredPeerVersions := tt.ourPolicy.FilterVersions(tt.peerVersions)

			// Select the negotiated version (best of both sides)
			negotiatedVersion := selectBestSupportedVersion(tt.ourVersions, filteredPeerVersions)

			// Select the session mode based on negotiated version and ratchet capability
			selectedMode, err := SelectSessionMode(tt.ourPolicy, negotiatedVersion, tt.peerRatchet, config)
			assert.NoError(t, err, "session mode selection should not error")

			assert.Equal(t, tt.expectedMode, selectedMode, "Unexpected session mode")

			if tt.shouldSucceed {
				assert.NotEqual(t, SessionMode(255), selectedMode, "Session mode selection should succeed")
			}
		})
	}
}

// TestPolicyEnforcesVersionFiltering ensures that policies correctly filter versions.
func TestPolicyEnforcesVersionFiltering(t *testing.T) {
	allVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}

	tests := []struct {
		policy           SessionPolicy
		input            []ProtocolVersion
		expectedOutput   []ProtocolVersion
		shouldBeFiltered bool
		name             string
	}{
		{
			PolicyLegacyOnly,
			allVersions,
			[]ProtocolVersion{ProtocolLegacy},
			true,
			"PolicyLegacyOnly filters out Noise",
		},
		{
			PolicyNoiseOnly,
			allVersions,
			allVersions,
			false,
			"PolicyNoiseOnly allows all for negotiation",
		},
		{
			PolicyNoiseWithRatchet,
			allVersions,
			allVersions,
			false,
			"PolicyNoiseWithRatchet allows all for negotiation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.FilterVersions(tt.input)
			assert.Equal(t, tt.expectedOutput, result)

			if tt.shouldBeFiltered {
				assert.Less(t, len(result), len(tt.input), "Filtering should reduce version count")
			} else {
				assert.Equal(t, len(result), len(tt.input), "No filtering should occur")
			}
		})
	}
}

// TestFallbackOrder validates that negotiation falls back correctly.
func TestFallbackOrder(t *testing.T) {
	// Test with policy that supports all versions
	ourVersions := []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy}

	tests := []struct {
		peerVersions    []ProtocolVersion
		expectedVersion ProtocolVersion
		name            string
	}{
		{
			[]ProtocolVersion{ProtocolNoiseIK},
			ProtocolNoiseIK,
			"peer supports noise -> select noise",
		},
		{
			[]ProtocolVersion{ProtocolLegacy},
			ProtocolLegacy,
			"peer supports only legacy -> select legacy",
		},
		{
			[]ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			ProtocolNoiseIK,
			"peer supports both -> select highest (noise)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			negotiatedVersion := selectBestSupportedVersion(ourVersions, tt.peerVersions)
			assert.Equal(t, tt.expectedVersion, negotiatedVersion)
		})
	}
}

// TestRatchetCapabilityDiscovery validates that ratchet capabilities are properly tracked.
func TestRatchetCapabilityDiscovery(t *testing.T) {
	tracker := NewRatchetCapabilityTracker()

	// Initially unknown
	assert.Equal(t, RatchetUnknown, tracker.GetCapability("peer1"))

	// Mark as unsupported
	tracker.MarkUnsupported("peer1")
	assert.Equal(t, RatchetUnsupported, tracker.GetCapability("peer1"))
	assert.False(t, tracker.IsSupported("peer1"))

	// Update to supported
	tracker.MarkSupported("peer1")
	assert.Equal(t, RatchetSupported, tracker.GetCapability("peer1"))
	assert.True(t, tracker.IsSupported("peer1"))

	// Different peers are independent
	assert.Equal(t, RatchetUnknown, tracker.GetCapability("peer2"))
}

// TestSessionModeEncryptionGuarantees validates that encryption is enforced.
func TestSessionModeEncryptionGuarantees(t *testing.T) {
	tests := []struct {
		policy            SessionPolicy
		version           ProtocolVersion
		ratchet           RatchetCapability
		expectedEncrypted bool
		name              string
	}{
		{
			PolicyLegacyOnly,
			ProtocolLegacy,
			RatchetUnsupported,
			false, // NaCl-box is not considered modern encryption
			"legacy mode is not encrypted",
		},
		{
			PolicyNoiseOnly,
			ProtocolNoiseIK,
			RatchetUnsupported,
			true,
			"noise mode is encrypted",
		},
		{
			PolicyNoiseWithRatchet,
			ProtocolNoiseIK,
			RatchetSupported,
			true,
			"noise+ratchet mode is encrypted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, _ := SelectSessionMode(tt.policy, tt.version, tt.ratchet, nil)
			assert.Equal(t, tt.expectedEncrypted, mode.IsEncrypted())
		})
	}
}

// TestPolicyCompliance validates that policy enforcement is correct.
func TestPolicyCompliance(t *testing.T) {
	tests := []struct {
		policy               SessionPolicy
		peerVersions         []ProtocolVersion
		peerRatchetCap       RatchetCapability
		expectedToUseRatchet bool
		name                 string
	}{
		{
			PolicyLegacyOnly,
			[]ProtocolVersion{ProtocolLegacy},
			RatchetSupported,
			false,
			"legacy policy never uses ratchet",
		},
		{
			PolicyNoiseOnly,
			[]ProtocolVersion{ProtocolNoiseIK},
			RatchetSupported,
			false,
			"noise-only policy never uses ratchet",
		},
		{
			PolicyNoiseWithRatchet,
			[]ProtocolVersion{ProtocolNoiseIK},
			RatchetSupported,
			true,
			"noise+ratchet policy uses ratchet when supported",
		},
		{
			PolicyNoiseWithRatchet,
			[]ProtocolVersion{ProtocolNoiseIK},
			RatchetUnsupported,
			false,
			"noise+ratchet policy falls back when peer doesn't support",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := tt.policy.FilterVersions(tt.peerVersions)
			require.NotEmpty(t, filtered)

			negotiated := selectBestSupportedVersion(tt.policy.DefaultSupportedVersions(), filtered)
			mode, _ := SelectSessionMode(tt.policy, negotiated, tt.peerRatchetCap, nil)

			assert.Equal(t, tt.expectedToUseRatchet, mode.CanUseRatchet())
		})
	}
}
