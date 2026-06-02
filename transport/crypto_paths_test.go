package transport_test

import (
	"errors"
	"testing"

	"github.com/opd-ai/toxcore/transport"
)

// TestCryptoPathHierarchyOrder is a static assertion that the SessionMode
// constants remain in weak-to-strong order, which ValidateCryptoPathTransition
// depends on via numeric comparison.
func TestCryptoPathHierarchyOrder(t *testing.T) {
	if transport.SessionModeLegacy >= transport.SessionModeNoise {
		t.Errorf("SessionModeLegacy (%d) must be < SessionModeNoise (%d)",
			transport.SessionModeLegacy, transport.SessionModeNoise)
	}
	if transport.SessionModeNoise >= transport.SessionModeNoiseWithRatchet {
		t.Errorf("SessionModeNoise (%d) must be < SessionModeNoiseWithRatchet (%d)",
			transport.SessionModeNoise, transport.SessionModeNoiseWithRatchet)
	}
}

// TestAllowedTransitionsMapCompleteness verifies that every upgrade pair is
// listed in AllowedTransitions. This is a compile-time-safe regression guard:
// if a new SessionMode is added without updating AllowedTransitions, the test fails.
func TestAllowedTransitionsMapCompleteness(t *testing.T) {
	modes := []transport.SessionMode{
		transport.SessionModeLegacy,
		transport.SessionModeNoise,
		transport.SessionModeNoiseWithRatchet,
	}
	for _, from := range modes {
		for _, to := range modes {
			if transport.IsCryptoUpgrade(from, to) || from == to {
				key := [2]transport.SessionMode{from, to}
				if !transport.AllowedTransitions[key] {
					t.Errorf("upgrade/same-mode transition %s → %s missing from AllowedTransitions", from, to)
				}
			}
		}
	}
}

// TestIsCryptoUpgradeDowngrade verifies the directional helpers.
func TestIsCryptoUpgradeDowngrade(t *testing.T) {
	cases := []struct {
		from, to         transport.SessionMode
		wantUp, wantDown bool
	}{
		{transport.SessionModeLegacy, transport.SessionModeNoise, true, false},
		{transport.SessionModeLegacy, transport.SessionModeNoiseWithRatchet, true, false},
		{transport.SessionModeNoise, transport.SessionModeNoiseWithRatchet, true, false},
		{transport.SessionModeNoise, transport.SessionModeLegacy, false, true},
		{transport.SessionModeNoiseWithRatchet, transport.SessionModeNoise, false, true},
		{transport.SessionModeNoiseWithRatchet, transport.SessionModeLegacy, false, true},
		{transport.SessionModeNoise, transport.SessionModeNoise, false, false},
	}
	for _, tc := range cases {
		if got := transport.IsCryptoUpgrade(tc.from, tc.to); got != tc.wantUp {
			t.Errorf("IsCryptoUpgrade(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.wantUp)
		}
		if got := transport.IsCryptoDowngrade(tc.from, tc.to); got != tc.wantDown {
			t.Errorf("IsCryptoDowngrade(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.wantDown)
		}
	}
}

// TestValidateCryptoPathTransition_UpgradesAlwaysAllowed checks that upgrades
// are never blocked regardless of policy.
func TestValidateCryptoPathTransition_UpgradesAlwaysAllowed(t *testing.T) {
	policies := []transport.SessionPolicy{
		transport.PolicyUnset,
		transport.PolicyLegacyOnly,
		transport.PolicyNoiseOnly,
		transport.PolicyNoiseWithRatchet,
	}
	upgrades := [][2]transport.SessionMode{
		{transport.SessionModeLegacy, transport.SessionModeNoise},
		{transport.SessionModeLegacy, transport.SessionModeNoiseWithRatchet},
		{transport.SessionModeNoise, transport.SessionModeNoiseWithRatchet},
	}
	for _, pol := range policies {
		for _, pair := range upgrades {
			if err := transport.ValidateCryptoPathTransition(pol, pair[0], pair[1]); err != nil {
				t.Errorf("policy %s: upgrade %s → %s unexpectedly blocked: %v", pol, pair[0], pair[1], err)
			}
		}
	}
}

// TestValidateCryptoPathTransition_StrictPolicyBlocksDowngrades verifies that
// PolicyNoiseWithRatchet (the default secure policy) blocks all downgrades,
// preventing downgrade attacks.
func TestValidateCryptoPathTransition_StrictPolicyBlocksDowngrades(t *testing.T) {
	downgrades := [][2]transport.SessionMode{
		{transport.SessionModeNoiseWithRatchet, transport.SessionModeNoise},
		{transport.SessionModeNoiseWithRatchet, transport.SessionModeLegacy},
		{transport.SessionModeNoise, transport.SessionModeLegacy},
	}
	for _, pair := range downgrades {
		err := transport.ValidateCryptoPathTransition(
			transport.PolicyNoiseWithRatchet, pair[0], pair[1])
		if err == nil {
			t.Errorf("PolicyNoiseWithRatchet: downgrade %s → %s should be blocked", pair[0], pair[1])
		}
		if !errors.Is(err, transport.ErrCryptoDowngradeBlocked) {
			t.Errorf("PolicyNoiseWithRatchet: expected ErrCryptoDowngradeBlocked, got %v", err)
		}
	}
}

// TestValidateCryptoPathTransition_NoiseOnlyBlocksLegacyDowngrade verifies that
// PolicyNoiseOnly blocks downgrades to legacy but permits ratchet→noise.
func TestValidateCryptoPathTransition_NoiseOnlyBlocksLegacyDowngrade(t *testing.T) {
	// Legacy downgrade must be blocked.
	err := transport.ValidateCryptoPathTransition(
		transport.PolicyNoiseOnly,
		transport.SessionModeNoise,
		transport.SessionModeLegacy,
	)
	if err == nil {
		t.Error("PolicyNoiseOnly: Noise → Legacy downgrade should be blocked")
	}

	// Ratchet → Noise is permitted under PolicyNoiseOnly.
	err = transport.ValidateCryptoPathTransition(
		transport.PolicyNoiseOnly,
		transport.SessionModeNoiseWithRatchet,
		transport.SessionModeNoise,
	)
	if err != nil {
		t.Errorf("PolicyNoiseOnly: NoiseWithRatchet → Noise should be allowed, got %v", err)
	}
}

// TestValidateCryptoPathTransition_SameMode verifies that same-mode transitions
// are always valid.
func TestValidateCryptoPathTransition_SameMode(t *testing.T) {
	modes := []transport.SessionMode{
		transport.SessionModeLegacy,
		transport.SessionModeNoise,
		transport.SessionModeNoiseWithRatchet,
	}
	policies := []transport.SessionPolicy{
		transport.PolicyUnset,
		transport.PolicyLegacyOnly,
		transport.PolicyNoiseOnly,
		transport.PolicyNoiseWithRatchet,
	}
	for _, pol := range policies {
		for _, mode := range modes {
			if err := transport.ValidateCryptoPathTransition(pol, mode, mode); err != nil {
				t.Errorf("policy %s: same-mode transition %s → %s unexpectedly blocked: %v", pol, mode, mode, err)
			}
		}
	}
}
