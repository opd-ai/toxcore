package transport

import (
	"errors"
	"fmt"
)

// ErrCryptoDowngradeBlocked is returned by ValidateCryptoPathTransition when
// a requested downgrade violates the active SessionPolicy.
var ErrCryptoDowngradeBlocked = errors.New("crypto path downgrade blocked by policy")

// CryptoPathMap documents all encryption paths available in toxcore-go and their
// security ordering. Higher values are stronger. Transitions may only move to
// equal or higher values unless policy explicitly permits downgrades.
//
// Path hierarchy (weakest → strongest):
//
//	SessionModeLegacy          NaCl-box (c-toxcore DHT protocol)
//	SessionModeNoise           Noise-IK handshake, no ratchet
//	SessionModeNoiseWithRatchet Noise-IK + Double Ratchet (forward secrecy)
//
// Async messaging layer (orthogonal to the above):
//   - Epoch-rotating recipient pseudonyms (async/obfs.go)
//   - Single-use sender pseudonyms per message
//   - Pre-key-based forward secrecy in async (async/forward_secrecy.go)
//
// These paths are declared here as a single authoritative reference so that
// future additions require an explicit update to this file, making the
// change visible in code review.
//
// AllowedTransitions is a registry of (from, to) pairs that are unconditionally
// allowed regardless of policy. Upgrades (lower→higher) are always in this set.
// Policy-gated transitions (downgrades) are checked by ValidateCryptoPathTransition.
var AllowedTransitions = map[[2]SessionMode]bool{
	// Upgrades are always permitted.
	{SessionModeLegacy, SessionModeNoise}:            true,
	{SessionModeLegacy, SessionModeNoiseWithRatchet}: true,
	{SessionModeNoise, SessionModeNoiseWithRatchet}:  true,

	// Same-mode (no change) is always valid.
	{SessionModeLegacy, SessionModeLegacy}:                   true,
	{SessionModeNoise, SessionModeNoise}:                     true,
	{SessionModeNoiseWithRatchet, SessionModeNoiseWithRatchet}: true,
}

// IsCryptoUpgrade reports whether transitioning from → to moves to a stronger
// encryption path. Upgrades never require policy approval.
func IsCryptoUpgrade(from, to SessionMode) bool {
	return to > from
}

// IsCryptoDowngrade reports whether transitioning from → to moves to a weaker
// encryption path. Downgrades require policy approval.
func IsCryptoDowngrade(from, to SessionMode) bool {
	return to < from
}

// ValidateCryptoPathTransition checks whether transitioning from the current
// SessionMode to the target SessionMode is permitted under the given SessionPolicy.
//
// Rules:
//   - Upgrades and same-mode transitions are always allowed.
//   - Under PolicyNoiseWithRatchet, all downgrades are blocked.
//   - Under PolicyNoiseOnly, downgrades to Legacy are blocked.
//   - Under PolicyLegacyOnly and PolicyUnset, downgrades are allowed (for
//     interoperability with non-upgraded peers).
func ValidateCryptoPathTransition(policy SessionPolicy, from, to SessionMode) error {
	if !IsCryptoDowngrade(from, to) {
		return nil
	}

	switch policy {
	case PolicyNoiseWithRatchet:
		return fmt.Errorf("%w: %s → %s rejected under %s policy",
			ErrCryptoDowngradeBlocked, from, to, policy)
	case PolicyNoiseOnly:
		if to == SessionModeLegacy {
			return fmt.Errorf("%w: %s → %s rejected under %s policy",
				ErrCryptoDowngradeBlocked, from, to, policy)
		}
	}
	return nil
}
