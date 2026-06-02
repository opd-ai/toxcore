package transport

import (
	"fmt"
)

// SessionPolicy defines which protocol versions and features are allowed for a session.
// It enables control over security trade-offs between forward secrecy (ratchet),
// modern encryption (Noise-IK), and compatibility (Legacy).
type SessionPolicy uint8

const (
	// PolicyUnset is the zero value, indicating no explicit session policy has been configured.
	// When unset, no version filtering is applied and the original capabilities are used as-is.
	PolicyUnset SessionPolicy = iota

	// PolicyLegacyOnly allows only the original Tox protocol (NaCl-box).
	// No forward secrecy, no Noise-IK.
	PolicyLegacyOnly

	// PolicyNoiseOnly allows Noise-IK but not ratcheting.
	// Provides forward secrecy via handshake, but not per-message ratcheting.
	PolicyNoiseOnly

	// PolicyNoiseWithRatchet requires Noise-IK with Double Ratchet when possible.
	// Maximum forward secrecy (Signal-like). Falls back to Noise-IK if peer doesn't support ratchet,
	// then to Legacy if necessary.
	PolicyNoiseWithRatchet
)

// String returns the human-readable name of the session policy.
func (p SessionPolicy) String() string {
	switch p {
	case PolicyUnset:
		return "unset"
	case PolicyLegacyOnly:
		return "legacy-only"
	case PolicyNoiseOnly:
		return "noise-only"
	case PolicyNoiseWithRatchet:
		return "noise+ratchet"
	default:
		return fmt.Sprintf("Unknown(%d)", p)
	}
}

// AllowsVersion reports whether a protocol version is permitted by this policy.
// This is used during negotiation to filter out unsupported versions.
func (p SessionPolicy) AllowsVersion(version ProtocolVersion) bool {
	switch p {
	case PolicyLegacyOnly:
		// Only allow Legacy
		return version == ProtocolLegacy

	case PolicyNoiseOnly:
		// Allow Noise-IK and Legacy (for fallback)
		return version == ProtocolNoiseIK || version == ProtocolLegacy

	case PolicyNoiseWithRatchet:
		// Allow all versions for negotiation fallback
		// Ratchet support is determined via capability negotiation, not via ProtocolVersion
		return version == ProtocolLegacy || version == ProtocolNoiseIK

	default:
		// Unknown policy - be conservative and allow only Legacy
		return version == ProtocolLegacy
	}
}

// FilterVersions returns only the versions from the list that are allowed by this policy,
// in the same order as the input list.
func (p SessionPolicy) FilterVersions(versions []ProtocolVersion) []ProtocolVersion {
	if len(versions) == 0 {
		return []ProtocolVersion{}
	}
	var filtered []ProtocolVersion
	for _, v := range versions {
		if p.AllowsVersion(v) {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// DefaultVersion returns the preferred protocol version for this policy.
// This is used when no version has been negotiated yet.
func (p SessionPolicy) DefaultVersion() ProtocolVersion {
	switch p {
	case PolicyLegacyOnly:
		return ProtocolLegacy

	case PolicyNoiseOnly:
		// Prefer Noise-IK, fall back to Legacy
		return ProtocolNoiseIK

	case PolicyNoiseWithRatchet:
		// Prefer Noise-IK (ratchet is negotiated separately)
		return ProtocolNoiseIK

	default:
		return ProtocolLegacy
	}
}

// DefaultSupportedVersions returns the list of versions this policy supports.
// This is used in the initial version negotiation to advertise capabilities.
func (p SessionPolicy) DefaultSupportedVersions() []ProtocolVersion {
	switch p {
	case PolicyLegacyOnly:
		// Only advertise Legacy
		return []ProtocolVersion{ProtocolLegacy}

	case PolicyNoiseOnly:
		// Advertise Noise-IK and Legacy for fallback
		return []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy}

	case PolicyNoiseWithRatchet:
		// Advertise both versions (ratchet negotiated separately)
		return []ProtocolVersion{ProtocolNoiseIK, ProtocolLegacy}

	default:
		// Unknown policy - be conservative
		return []ProtocolVersion{ProtocolLegacy}
	}
}

// PolicyConfig holds the session policy configuration for a transport.
// It can be set per-instance or per-peer.
type PolicyConfig struct {
	// DefaultPolicy is the policy to use when no peer-specific policy is set.
	DefaultPolicy SessionPolicy

	// RequireRatchetForNoise controls whether Noise-IK sessions should require
	// ratcheting when both peers support it. If true and ratchet initialization fails,
	// the connection falls back to Legacy.
	RequireRatchetForNoise bool

	// AllowUnauthenticatedFallback controls whether to allow fallback to Legacy
	// if Noise negotiation or ratchet bootstrap fails. If false, the connection fails.
	AllowUnauthenticatedFallback bool
}

// DefaultPolicyConfig returns a policy config with sensible defaults:
// - Use NoiseWithRatchet as default (maximum security)
// - Require ratchet for Noise sessions
// - Allow fallback to Legacy for compatibility
func DefaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		DefaultPolicy:                PolicyNoiseWithRatchet,
		RequireRatchetForNoise:       true,
		AllowUnauthenticatedFallback: true,
	}
}

// StrictPolicyConfig returns a policy config that maximizes security:
// - Use NoiseWithRatchet as default
// - Require ratchet for Noise sessions
// - Do NOT allow fallback to Legacy (connection fails if ratchet not available)
func StrictPolicyConfig() PolicyConfig {
	return PolicyConfig{
		DefaultPolicy:                PolicyNoiseWithRatchet,
		RequireRatchetForNoise:       true,
		AllowUnauthenticatedFallback: false,
	}
}

// LegacyCompatPolicyConfig returns a policy config for maximum compatibility:
// - Use LegacyOnly as default
// - No ratchet requirement
// - Allow all fallback paths
func LegacyCompatPolicyConfig() PolicyConfig {
	return PolicyConfig{
		DefaultPolicy:                PolicyLegacyOnly,
		RequireRatchetForNoise:       false,
		AllowUnauthenticatedFallback: true,
	}
}
