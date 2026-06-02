package transport

import (
	"errors"
	"sync"
)

// ErrRatchetRequired is returned by SelectSessionMode when ratchet is required
// by policy (RequireRatchetForNoise=true, AllowUnauthenticatedFallback=false)
// but the peer does not support it.
var ErrRatchetRequired = errors.New("ratchet required by policy but peer does not support it")

// RatchetCapability tracks whether a peer supports the Double Ratchet extension.
// Ratchet capability is orthogonal to ProtocolVersion and must be negotiated separately.
type RatchetCapability uint8

const (
	// RatchetUnknown indicates ratchet capability has not been negotiated yet.
	RatchetUnknown RatchetCapability = iota

	// RatchetUnsupported indicates the peer does not support ratchet extension.
	RatchetUnsupported

	// RatchetSupported indicates the peer supports ratchet extension.
	RatchetSupported
)

// String returns the human-readable name of the ratchet capability.
func (rc RatchetCapability) String() string {
	switch rc {
	case RatchetUnknown:
		return "unknown"
	case RatchetUnsupported:
		return "unsupported"
	case RatchetSupported:
		return "supported"
	default:
		return "unknown"
	}
}

// RatchetCapabilityTracker maintains the ratchet capabilities for all known peers.
// It allows caching and updating capability state.
type RatchetCapabilityTracker struct {
	mu           sync.RWMutex
	capabilities map[string]RatchetCapability // addr.String() -> RatchetCapability
}

// NewRatchetCapabilityTracker creates a new tracker for peer ratchet capabilities.
func NewRatchetCapabilityTracker() *RatchetCapabilityTracker {
	return &RatchetCapabilityTracker{
		capabilities: make(map[string]RatchetCapability),
	}
}

// GetCapability returns the known ratchet capability for a peer.
// Returns RatchetUnknown if the capability has not been discovered yet.
func (rct *RatchetCapabilityTracker) GetCapability(addr string) RatchetCapability {
	rct.mu.RLock()
	defer rct.mu.RUnlock()

	if cap, exists := rct.capabilities[addr]; exists {
		return cap
	}
	return RatchetUnknown
}

// SetCapability updates the ratchet capability for a peer.
func (rct *RatchetCapabilityTracker) SetCapability(addr string, cap RatchetCapability) {
	rct.mu.Lock()
	defer rct.mu.Unlock()

	rct.capabilities[addr] = cap
}

// MarkSupported marks a peer as supporting ratchet capability.
func (rct *RatchetCapabilityTracker) MarkSupported(addr string) {
	rct.SetCapability(addr, RatchetSupported)
}

// MarkUnsupported marks a peer as not supporting ratchet capability.
func (rct *RatchetCapabilityTracker) MarkUnsupported(addr string) {
	rct.SetCapability(addr, RatchetUnsupported)
}

// IsSupported returns true if the peer is known to support ratchet capability.
func (rct *RatchetCapabilityTracker) IsSupported(addr string) bool {
	return rct.GetCapability(addr) == RatchetSupported
}

// Clear removes all cached capability information.
// This is useful when resetting the transport or testing.
func (rct *RatchetCapabilityTracker) Clear() {
	rct.mu.Lock()
	defer rct.mu.Unlock()

	rct.capabilities = make(map[string]RatchetCapability)
}

// RemovePeer removes the capability entry for a peer.
// This is useful when a peer disconnects.
func (rct *RatchetCapabilityTracker) RemovePeer(addr string) {
	rct.mu.Lock()
	defer rct.mu.Unlock()

	delete(rct.capabilities, addr)
}

// SelectSessionMode determines the appropriate session mode based on:
// 1. The configured session policy
// 2. The negotiated protocol version
// 3. Peer ratchet capability (if Noise-IK is used)
//
// The returned SessionMode indicates which encryption/ratcheting should be used.
type SessionMode uint8

const (
	// SessionModeLegacy uses the original Tox protocol (NaCl-box).
	SessionModeLegacy SessionMode = iota

	// SessionModeNoise uses Noise-IK without ratchet.
	SessionModeNoise

	// SessionModeNoiseWithRatchet uses Noise-IK with Double Ratchet.
	SessionModeNoiseWithRatchet
)

// String returns the human-readable name of the session mode.
func (sm SessionMode) String() string {
	switch sm {
	case SessionModeLegacy:
		return "legacy"
	case SessionModeNoise:
		return "noise"
	case SessionModeNoiseWithRatchet:
		return "noise+ratchet"
	default:
		return "unknown"
	}
}

// SelectSessionMode determines the appropriate session mode based on policy and capabilities.
// This function implements the fallback logic:
// 1. If policy allows ratchet and both sides support it -> noise+ratchet
// 2. If policy allows noise -> noise (or error if strict policyConfig forbids fallback)
// 3. If policy allows legacy -> legacy
// 4. Return error if no compatible mode is available
func SelectSessionMode(
	policy SessionPolicy,
	protocolVersion ProtocolVersion,
	peerRatchetCap RatchetCapability,
	policyConfig *PolicyConfig,
) (SessionMode, error) {
	// If we negotiated Legacy, always use Legacy
	if protocolVersion == ProtocolLegacy {
		return SessionModeLegacy, nil
	}

	// If we negotiated Noise-IK, check if we can use ratchet
	if protocolVersion == ProtocolNoiseIK {
		// Check if policy supports ratchet
		if policy == PolicyNoiseWithRatchet {
			// Check if peer supports ratchet
			if peerRatchetCap == RatchetSupported {
				return SessionModeNoiseWithRatchet, nil
			}
			// Peer does not support ratchet: honour strict policy config if set.
			// Under strict config (RequireRatchetForNoise=true, AllowUnauthenticatedFallback=false)
			// refuse the connection rather than silently downgrading.
			if policyConfig != nil && policyConfig.RequireRatchetForNoise && !policyConfig.AllowUnauthenticatedFallback {
				return SessionModeLegacy, ErrRatchetRequired
			}
		}
		// Use plain Noise without ratchet
		return SessionModeNoise, nil
	}

	// Fallback to Legacy
	return SessionModeLegacy, nil
}

// CanUseRatchet returns true if the current mode uses ratcheting.
func (sm SessionMode) CanUseRatchet() bool {
	return sm == SessionModeNoiseWithRatchet
}

// IsEncrypted returns true if the session mode uses encryption.
func (sm SessionMode) IsEncrypted() bool {
	return sm != SessionModeLegacy // Both Noise modes use encryption
}
