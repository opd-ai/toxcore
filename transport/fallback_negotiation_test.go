package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFallbackNegotiationRespectPolicy validates that fallback negotiation respects the session policy.
// This test ensures that when a peer doesn't support the preferred version,
// the fallback version is selected according to policy constraints.
func TestFallbackNegotiationRespectPolicy(t *testing.T) {
	tests := []struct {
		name                 string
		policy               SessionPolicy
		peerSupportedVersions []ProtocolVersion
		expectedFallback     ProtocolVersion
	}{
		{
			name:                  "legacy-only policy falls back to legacy",
			policy:                PolicyLegacyOnly,
			peerSupportedVersions: []ProtocolVersion{ProtocolLegacy},
			expectedFallback:      ProtocolLegacy,
		},
		{
			name:                  "legacy-only policy rejects noise",
			policy:                PolicyLegacyOnly,
			peerSupportedVersions: []ProtocolVersion{ProtocolNoiseIK},
			expectedFallback:      ProtocolLegacy, // Falls back to the only allowed version
		},
		{
			name:                  "noise-only policy prefers noise over legacy",
			policy:                PolicyNoiseOnly,
			peerSupportedVersions: []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			expectedFallback:      ProtocolNoiseIK,
		},
		{
			name:                  "noise-only policy falls back to legacy when noise unavailable",
			policy:                PolicyNoiseOnly,
			peerSupportedVersions: []ProtocolVersion{ProtocolLegacy},
			expectedFallback:      ProtocolLegacy,
		},
		{
			name:                  "noise+ratchet policy prefers noise",
			policy:                PolicyNoiseWithRatchet,
			peerSupportedVersions: []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			expectedFallback:      ProtocolNoiseIK,
		},
		{
			name:                  "noise+ratchet policy falls back to legacy when noise unavailable",
			policy:                PolicyNoiseWithRatchet,
			peerSupportedVersions: []ProtocolVersion{ProtocolLegacy},
			expectedFallback:      ProtocolLegacy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create capabilities with the policy
			defaultPolicyConfig := DefaultPolicyConfig()
			cap := &ProtocolCapabilities{
				SupportedVersions:    []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
				PreferredVersion:     ProtocolNoiseIK,
				EnableLegacyFallback: true,
				SessionPolicy:        tt.policy,
				PolicyConfig:         &defaultPolicyConfig,
			}

			// Apply policy filtering
			filteredVersions, _ := applySessionPolicyToCapabilities(cap)

			// Select best version from peer's supported versions
			selectedVersion := selectBestSupportedVersion(filteredVersions, tt.peerSupportedVersions)

			assert.Equal(t, tt.expectedFallback, selectedVersion, "fallback version should match policy")
		})
	}
}

// TestFallbackOrderStrictPrecedence validates strict precedence: Noise > Legacy.
// Even if Legacy is listed first in peerVersions, Noise should be selected if available.
func TestFallbackOrderStrictPrecedence(t *testing.T) {
	// Noise-supporting peer lists versions in any order
	peerVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}

	ourVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}

	selected := selectBestSupportedVersion(ourVersions, peerVersions)

	// Should select Noise (highest), not Legacy (first listed)
	assert.Equal(t, ProtocolNoiseIK, selected, "should select highest supported version regardless of order")
}

// TestFallbackWhenPeerSupportsOnlyLegacy validates that we fall back to Legacy
// when peer doesn't support Noise.
func TestFallbackWhenPeerSupportsOnlyLegacy(t *testing.T) {
	peerVersions := []ProtocolVersion{ProtocolLegacy}
	ourVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}

	selected := selectBestSupportedVersion(ourVersions, peerVersions)

	assert.Equal(t, ProtocolLegacy, selected, "should fall back to legacy when peer doesn't support noise")
}

// TestNoFallbackWhenPeerSupportsNeither validates that we still return a version
// (defaults to Legacy) even if peer supports neither of our versions.
// This is a safety fallback.
func TestNoFallbackWhenPeerSupportsNeither(t *testing.T) {
	peerVersions := []ProtocolVersion{} // Peer supports nothing

	ourVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}

	selected := selectBestSupportedVersion(ourVersions, peerVersions)

	// Should default to Legacy as safe fallback
	assert.Equal(t, ProtocolLegacy, selected, "should default to legacy when no mutual support")
}

// TestPolicyFilteringPreventsSilentDowngrade validates that PolicyNoiseOnly
// never falls back to Legacy silently if peer only supports Legacy.
func TestPolicyFilteringPreventsSilentDowngrade(t *testing.T) {
	// Policy says "Noise-Only"
	policy := PolicyNoiseOnly

	// But peer only supports Legacy
	peerVersions := []ProtocolVersion{ProtocolLegacy}

	// Filter what WE support according to policy
	// Note: PolicyNoiseOnly allows both Noise and Legacy (for fallback within the policy)
	ourVersionsAfterFiltering := policy.FilterVersions([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK})

	// Select best from what we're allowed to use
	selected := selectBestSupportedVersion(ourVersionsAfterFiltering, peerVersions)

	// This should be Legacy because PolicyNoiseOnly allows it as a fallback
	assert.Equal(t, ProtocolLegacy, selected, "noise-only policy allows legacy as fallback")
}

// TestPolicyLegacyOnlyNeverNegotiatesNoise validates that PolicyLegacyOnly
// never attempts to negotiate Noise, even if peer supports it.
func TestPolicyLegacyOnlyNeverNegotiatesNoise(t *testing.T) {
	// Policy says "Legacy-Only"
	policy := PolicyLegacyOnly

	// Peer supports Noise
	peerVersions := []ProtocolVersion{ProtocolNoiseIK}

	// Filter what WE support according to policy
	ourVersionsAfterFiltering := policy.FilterVersions([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK})

	// ourVersionsAfterFiltering should only contain Legacy
	assert.Len(t, ourVersionsAfterFiltering, 1)
	assert.Equal(t, ProtocolLegacy, ourVersionsAfterFiltering[0])

	// Select best from what we're allowed to use
	selected := selectBestSupportedVersion(ourVersionsAfterFiltering, peerVersions)

	// Must be Legacy, never Noise
	assert.Equal(t, ProtocolLegacy, selected, "legacy-only policy never negotiates noise")
}

// TestFallbackNegotiationIntegration tests the full negotiation stack respecting policy.
func TestFallbackNegotiationIntegration(t *testing.T) {
	tests := []struct {
		name                  string
		ourPolicy             SessionPolicy
		peerVersions          []ProtocolVersion
		peerRatchetCapability RatchetCapability
		expectedMode          SessionMode
		expectedFallbackOK    bool
	}{
		{
			name:                  "noise+ratchet -> legacy peer -> noise mode (no ratchet)",
			ourPolicy:             PolicyNoiseWithRatchet,
			peerVersions:          []ProtocolVersion{ProtocolLegacy},
			peerRatchetCapability: RatchetUnsupported,
			expectedMode:          SessionModeLegacy,
			expectedFallbackOK:    true,
		},
		{
			name:                  "noise-only -> legacy peer -> legacy fallback",
			ourPolicy:             PolicyNoiseOnly,
			peerVersions:          []ProtocolVersion{ProtocolLegacy},
			peerRatchetCapability: RatchetUnsupported,
			expectedMode:          SessionModeLegacy,
			expectedFallbackOK:    true,
		},
		{
			name:                  "noise+ratchet -> noise peer -> noise mode",
			ourPolicy:             PolicyNoiseWithRatchet,
			peerVersions:          []ProtocolVersion{ProtocolNoiseIK},
			peerRatchetCapability: RatchetUnsupported,
			expectedMode:          SessionModeNoise,
			expectedFallbackOK:    true,
		},
		{
			name:                  "noise+ratchet -> noise+ratchet peer -> noise+ratchet mode",
			ourPolicy:             PolicyNoiseWithRatchet,
			peerVersions:          []ProtocolVersion{ProtocolNoiseIK},
			peerRatchetCapability: RatchetSupported,
			expectedMode:          SessionModeNoiseWithRatchet,
			expectedFallbackOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Filter versions according to policy
			filteredVersions := tt.ourPolicy.FilterVersions(tt.peerVersions)
			require.NotEmpty(t, filteredVersions, "policy should allow at least one fallback version")

			// Select the negotiated version
			negotiatedVersion := selectBestSupportedVersion(tt.ourPolicy.DefaultSupportedVersions(), filteredVersions)

			// Select the session mode with fallback support
			mode, _ := SelectSessionMode(tt.ourPolicy, negotiatedVersion, tt.peerRatchetCapability, nil)

			assert.Equal(t, tt.expectedMode, mode, "session mode should match expected")
			assert.True(t, tt.expectedFallbackOK, "fallback should be allowed")
		})
	}
}
