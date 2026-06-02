package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionPolicyString(t *testing.T) {
	tests := []struct {
		policy   SessionPolicy
		expected string
	}{
		{PolicyUnset, "unset"},
		{PolicyLegacyOnly, "legacy-only"},
		{PolicyNoiseOnly, "noise-only"},
		{PolicyNoiseWithRatchet, "noise+ratchet"},
		{SessionPolicy(255), "Unknown(255)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.policy.String())
		})
	}
}

func TestSessionPolicyAllowsVersion(t *testing.T) {
	tests := []struct {
		policy    SessionPolicy
		version   ProtocolVersion
		allowed   bool
		name      string
	}{
		// PolicyLegacyOnly
		{PolicyLegacyOnly, ProtocolLegacy, true, "LegacyOnly allows Legacy"},
		{PolicyLegacyOnly, ProtocolNoiseIK, false, "LegacyOnly denies Noise-IK"},

		// PolicyNoiseOnly
		{PolicyNoiseOnly, ProtocolLegacy, true, "NoiseOnly allows Legacy (fallback)"},
		{PolicyNoiseOnly, ProtocolNoiseIK, true, "NoiseOnly allows Noise-IK"},

		// PolicyNoiseWithRatchet
		{PolicyNoiseWithRatchet, ProtocolLegacy, true, "NoiseWithRatchet allows Legacy (fallback)"},
		{PolicyNoiseWithRatchet, ProtocolNoiseIK, true, "NoiseWithRatchet allows Noise-IK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.allowed, tt.policy.AllowsVersion(tt.version))
		})
	}
}

func TestSessionPolicyFilterVersions(t *testing.T) {
	allVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}

	tests := []struct {
		policy       SessionPolicy
		input        []ProtocolVersion
		expected     []ProtocolVersion
		name         string
	}{
		{
			PolicyLegacyOnly,
			allVersions,
			[]ProtocolVersion{ProtocolLegacy},
			"LegacyOnly filters to Legacy only",
		},
		{
			PolicyNoiseOnly,
			allVersions,
			[]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			"NoiseOnly keeps all (for fallback)",
		},
		{
			PolicyNoiseWithRatchet,
			allVersions,
			[]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			"NoiseWithRatchet keeps all (for fallback)",
		},
		{
			PolicyLegacyOnly,
			[]ProtocolVersion{},
			[]ProtocolVersion{},
			"Filter empty list returns empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.FilterVersions(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSessionPolicyDefaultVersion(t *testing.T) {
	tests := []struct {
		policy    SessionPolicy
		expected  ProtocolVersion
		name      string
	}{
		{PolicyLegacyOnly, ProtocolLegacy, "LegacyOnly defaults to Legacy"},
		{PolicyNoiseOnly, ProtocolNoiseIK, "NoiseOnly defaults to Noise-IK"},
		{PolicyNoiseWithRatchet, ProtocolNoiseIK, "NoiseWithRatchet defaults to Noise-IK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.policy.DefaultVersion())
		})
	}
}

func TestSessionPolicyDefaultSupportedVersions(t *testing.T) {
	tests := []struct {
		policy   SessionPolicy
		expected []ProtocolVersion
		name     string
	}{
		{
			PolicyLegacyOnly,
			[]ProtocolVersion{ProtocolLegacy},
			"LegacyOnly advertises only Legacy",
		},
		{
			PolicyNoiseOnly,
			[]ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			"NoiseOnly advertises Noise-IK and Legacy",
		},
		{
			PolicyNoiseWithRatchet,
			[]ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			"NoiseWithRatchet advertises Noise-IK and Legacy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.DefaultSupportedVersions()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultPolicyConfig(t *testing.T) {
	config := DefaultPolicyConfig()

	assert.Equal(t, PolicyNoiseWithRatchet, config.DefaultPolicy)
	assert.True(t, config.RequireRatchetForNoise)
	assert.True(t, config.AllowUnauthenticatedFallback)
}

func TestStrictPolicyConfig(t *testing.T) {
	config := StrictPolicyConfig()

	assert.Equal(t, PolicyNoiseWithRatchet, config.DefaultPolicy)
	assert.True(t, config.RequireRatchetForNoise)
	assert.False(t, config.AllowUnauthenticatedFallback)
}

func TestLegacyCompatPolicyConfig(t *testing.T) {
	config := LegacyCompatPolicyConfig()

	assert.Equal(t, PolicyLegacyOnly, config.DefaultPolicy)
	assert.False(t, config.RequireRatchetForNoise)
	assert.True(t, config.AllowUnauthenticatedFallback)
}

// TestPolicyConsistency ensures that FilterVersions is consistent with AllowsVersion
func TestPolicyConsistency(t *testing.T) {
	policies := []SessionPolicy{
		PolicyLegacyOnly,
		PolicyNoiseOnly,
		PolicyNoiseWithRatchet,
	}

	allVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}

	for _, policy := range policies {
		t.Run(policy.String(), func(t *testing.T) {
			filtered := policy.FilterVersions(allVersions)

			// Every version in filtered should be allowed by policy
			for _, v := range filtered {
				assert.True(t, policy.AllowsVersion(v),
					"FilterVersions returned disallowed version %v", v)
			}

			// Every version allowed by policy should be in filtered (in order)
			allowedCount := 0
			for _, v := range allVersions {
				if policy.AllowsVersion(v) {
					allowedCount++
				}
			}
			assert.Equal(t, allowedCount, len(filtered),
				"FilterVersions missing some allowed versions")
		})
	}
}

// TestMixedPeerScenarios tests negotiation results for different peer combinations
func TestMixedPeerVersionScenarios(t *testing.T) {
	tests := []struct {
		peerVersions     []ProtocolVersion
		selectedVersion  ProtocolVersion
		name             string
	}{
		{
			[]ProtocolVersion{ProtocolLegacy},
			ProtocolLegacy,
			"legacy peer -> select Legacy",
		},
		{
			[]ProtocolVersion{ProtocolNoiseIK},
			ProtocolNoiseIK,
			"noise peer -> select Noise-IK",
		},
		{
			[]ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy},
			ProtocolNoiseIK,
			"dual peer -> select Noise-IK (highest)",
		},
		{
			[]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			ProtocolNoiseIK,
			"dual peer (reversed order) -> select Noise-IK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := selectBestSupportedVersion([]ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy}, tt.peerVersions)
			assert.Equal(t, tt.selectedVersion, selected)
		})
	}
}

// TestPolicyBasedNegotiation tests that policies correctly filter negotiation options
func TestPolicyBasedNegotiation(t *testing.T) {
	peerVersions := []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy}

	tests := []struct {
		policy           SessionPolicy
		expectedSelected ProtocolVersion
		name             string
	}{
		{
			PolicyLegacyOnly,
			ProtocolLegacy,
			"LegacyOnly filters to Legacy before negotiation",
		},
		{
			PolicyNoiseOnly,
			ProtocolNoiseIK,
			"NoiseOnly selects Noise-IK when available",
		},
		{
			PolicyNoiseWithRatchet,
			ProtocolNoiseIK,
			"NoiseWithRatchet selects Noise-IK when available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate what a policy-aware negotiator would do:
			// 1. Filter peer's versions by policy
			filtered := tt.policy.FilterVersions(peerVersions)
			require.NotEmpty(t, filtered, "Policy filtered all versions")

			// 2. Select best from filtered
			selected := selectBestSupportedVersion(
				tt.policy.DefaultSupportedVersions(),
				filtered,
			)

			assert.Equal(t, tt.expectedSelected, selected)
		})
	}
}
