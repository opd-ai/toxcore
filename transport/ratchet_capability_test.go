package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRatchetCapabilityString(t *testing.T) {
	tests := []struct {
		cap      RatchetCapability
		expected string
	}{
		{RatchetUnknown, "unknown"},
		{RatchetUnsupported, "unsupported"},
		{RatchetSupported, "supported"},
		{RatchetCapability(255), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cap.String())
		})
	}
}

func TestRatchetCapabilityTracker_GetCapability(t *testing.T) {
	tracker := NewRatchetCapabilityTracker()

	// Unknown peer should return RatchetUnknown
	assert.Equal(t, RatchetUnknown, tracker.GetCapability("peer1"))

	// Set and verify
	tracker.SetCapability("peer1", RatchetSupported)
	assert.Equal(t, RatchetSupported, tracker.GetCapability("peer1"))

	// Different peer is unaffected
	assert.Equal(t, RatchetUnknown, tracker.GetCapability("peer2"))
}

func TestRatchetCapabilityTracker_SetCapability(t *testing.T) {
	tracker := NewRatchetCapabilityTracker()

	tracker.SetCapability("peer1", RatchetSupported)
	assert.Equal(t, RatchetSupported, tracker.GetCapability("peer1"))

	// Update capability
	tracker.SetCapability("peer1", RatchetUnsupported)
	assert.Equal(t, RatchetUnsupported, tracker.GetCapability("peer1"))
}

func TestRatchetCapabilityTracker_MarkSupported(t *testing.T) {
	tracker := NewRatchetCapabilityTracker()

	tracker.MarkSupported("peer1")
	assert.Equal(t, RatchetSupported, tracker.GetCapability("peer1"))
	assert.True(t, tracker.IsSupported("peer1"))
}

func TestRatchetCapabilityTracker_MarkUnsupported(t *testing.T) {
	tracker := NewRatchetCapabilityTracker()

	tracker.MarkUnsupported("peer1")
	assert.Equal(t, RatchetUnsupported, tracker.GetCapability("peer1"))
	assert.False(t, tracker.IsSupported("peer1"))
}

func TestRatchetCapabilityTracker_IsSupported(t *testing.T) {
	tracker := NewRatchetCapabilityTracker()

	// Unknown is not supported
	assert.False(t, tracker.IsSupported("unknown"))

	// Mark unsupported
	tracker.MarkUnsupported("unsupported")
	assert.False(t, tracker.IsSupported("unsupported"))

	// Mark supported
	tracker.MarkSupported("supported")
	assert.True(t, tracker.IsSupported("supported"))
}

func TestRatchetCapabilityTracker_Clear(t *testing.T) {
	tracker := NewRatchetCapabilityTracker()

	tracker.MarkSupported("peer1")
	tracker.MarkUnsupported("peer2")
	assert.Equal(t, RatchetSupported, tracker.GetCapability("peer1"))
	assert.Equal(t, RatchetUnsupported, tracker.GetCapability("peer2"))

	// Clear all
	tracker.Clear()
	assert.Equal(t, RatchetUnknown, tracker.GetCapability("peer1"))
	assert.Equal(t, RatchetUnknown, tracker.GetCapability("peer2"))
}

func TestRatchetCapabilityTracker_RemovePeer(t *testing.T) {
	tracker := NewRatchetCapabilityTracker()

	tracker.MarkSupported("peer1")
	tracker.MarkSupported("peer2")

	// Remove one peer
	tracker.RemovePeer("peer1")
	assert.Equal(t, RatchetUnknown, tracker.GetCapability("peer1"))
	assert.Equal(t, RatchetSupported, tracker.GetCapability("peer2"))
}

func TestSessionModeString(t *testing.T) {
	tests := []struct {
		mode     SessionMode
		expected string
	}{
		{SessionModeLegacy, "legacy"},
		{SessionModeNoise, "noise"},
		{SessionModeNoiseWithRatchet, "noise+ratchet"},
		{SessionMode(255), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestSessionModeCanUseRatchet(t *testing.T) {
	tests := []struct {
		mode     SessionMode
		expected bool
		name     string
	}{
		{SessionModeLegacy, false, "legacy cannot use ratchet"},
		{SessionModeNoise, false, "noise cannot use ratchet"},
		{SessionModeNoiseWithRatchet, true, "noise+ratchet can use ratchet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.CanUseRatchet())
		})
	}
}

func TestSessionModeIsEncrypted(t *testing.T) {
	tests := []struct {
		mode     SessionMode
		expected bool
		name     string
	}{
		{SessionModeLegacy, false, "legacy is not encrypted (NaCl)"},
		{SessionModeNoise, true, "noise is encrypted"},
		{SessionModeNoiseWithRatchet, true, "noise+ratchet is encrypted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.IsEncrypted())
		})
	}
}

func TestSelectSessionMode(t *testing.T) {
	tests := []struct {
		name       string
		policy     SessionPolicy
		version    ProtocolVersion
		ratchetCap RatchetCapability
		config     *PolicyConfig
		expected   SessionMode
	}{
		// Legacy version always results in Legacy mode
		{
			"legacy version -> legacy mode",
			PolicyNoiseWithRatchet,
			ProtocolLegacy,
			RatchetSupported,
			nil,
			SessionModeLegacy,
		},

		// Noise version with policy=LegacyOnly -> Noise (fallback)
		{
			"legacy policy + noise version -> noise mode",
			PolicyLegacyOnly,
			ProtocolNoiseIK,
			RatchetSupported,
			nil,
			SessionModeNoise,
		},

		// Noise version with policy=NoiseOnly -> Noise
		{
			"noise policy + noise version -> noise mode",
			PolicyNoiseOnly,
			ProtocolNoiseIK,
			RatchetSupported,
			nil,
			SessionModeNoise,
		},

		// Noise version with policy=NoiseWithRatchet and peer supports ratchet -> NoiseWithRatchet
		{
			"noise+ratchet policy + noise version + peer supports -> noise+ratchet mode",
			PolicyNoiseWithRatchet,
			ProtocolNoiseIK,
			RatchetSupported,
			nil,
			SessionModeNoiseWithRatchet,
		},

		// Noise version with policy=NoiseWithRatchet but peer doesn't support ratchet -> Noise
		{
			"noise+ratchet policy + noise version + peer unsupported -> noise mode",
			PolicyNoiseWithRatchet,
			ProtocolNoiseIK,
			RatchetUnsupported,
			nil,
			SessionModeNoise,
		},

		// Noise version with policy=NoiseWithRatchet and unknown ratchet capability -> Noise
		{
			"noise+ratchet policy + noise version + unknown ratchet -> noise mode",
			PolicyNoiseWithRatchet,
			ProtocolNoiseIK,
			RatchetUnknown,
			nil,
			SessionModeNoise,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := SelectSessionMode(tt.policy, tt.version, tt.ratchetCap, tt.config)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, mode)
		})
	}
}

// TestSelectSessionMode_StrictPolicy validates that a strict PolicyConfig causes
// SelectSessionMode to return ErrRatchetRequired when the peer does not support ratchet.
func TestSelectSessionMode_StrictPolicy(t *testing.T) {
	strict := StrictPolicyConfig()

	// Strict config + NoiseWithRatchet policy + peer unsupported -> error
	mode, err := SelectSessionMode(PolicyNoiseWithRatchet, ProtocolNoiseIK, RatchetUnsupported, &strict)
	assert.ErrorIs(t, err, ErrRatchetRequired)
	assert.Equal(t, SessionModeLegacy, mode)

	// Strict config but peer supports ratchet -> no error
	mode, err = SelectSessionMode(PolicyNoiseWithRatchet, ProtocolNoiseIK, RatchetSupported, &strict)
	assert.NoError(t, err)
	assert.Equal(t, SessionModeNoiseWithRatchet, mode)
}

// TestSessionModeSelection_FallbackHierarchy verifies the fallback priority:
// 1. noise+ratchet (if policy supports and peer supports)
// 2. noise (if policy allows)
// 3. legacy (if policy allows)
func TestSessionModeSelection_FallbackHierarchy(t *testing.T) {
	tests := []struct {
		name                string
		policy              SessionPolicy
		peerCanDoRatchet    bool
		expectedPriority    []SessionMode
	}{
		{
			"NoiseWithRatchet policy",
			PolicyNoiseWithRatchet,
			true,
			[]SessionMode{SessionModeNoiseWithRatchet, SessionModeNoise, SessionModeLegacy},
		},
		{
			"NoiseOnly policy",
			PolicyNoiseOnly,
			true,
			[]SessionMode{SessionModeNoise, SessionModeLegacy},
		},
		{
			"LegacyOnly policy",
			PolicyLegacyOnly,
			true,
			[]SessionMode{SessionModeLegacy},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test each protocol version negotiation
			modes := []SessionMode{}

			// First: Noise version with ratchet support (if policy allows)
			if tt.policy != PolicyLegacyOnly {
				ratchetCap := RatchetUnsupported
				if tt.policy == PolicyNoiseWithRatchet && tt.peerCanDoRatchet {
					ratchetCap = RatchetSupported
				}
				mode, _ := SelectSessionMode(tt.policy, ProtocolNoiseIK, ratchetCap, nil)
				modes = append(modes, mode)
			}

			// Then: Noise version without ratchet (fallback)
			if tt.policy != PolicyLegacyOnly {
				mode, _ := SelectSessionMode(tt.policy, ProtocolNoiseIK, RatchetUnsupported, nil)
				// Skip duplicates
				if len(modes) == 0 || modes[len(modes)-1] != mode {
					modes = append(modes, mode)
				}
			}

			// Finally: Legacy version (fallback)
			mode, _ := SelectSessionMode(tt.policy, ProtocolLegacy, RatchetUnsupported, nil)
			if len(modes) == 0 || modes[len(modes)-1] != mode {
				modes = append(modes, mode)
			}

			assert.Equal(t, tt.expectedPriority, modes)
		})
	}
}
