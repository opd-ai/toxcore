package transport

import (
	"sync"
	"time"
)

// TrustState represents the trust/identity-verification state for a peer key.
//
// Transitions:
//
//	TrustUnknown  → TrustOnFirstUse  (first key seen for that peer identity)
//	TrustOnFirstUse → TrustVerified   (app explicitly marks peer as verified)
//	TrustOnFirstUse → TrustKeyChanged (a different key is seen for a previously trusted peer)
//	TrustKeyChanged → TrustVerified   (app explicitly re-verifies after alarm)
type TrustState uint8

const (
	// TrustUnknown is the initial state before any key has been recorded for a peer.
	TrustUnknown TrustState = iota

	// TrustOnFirstUse is set when the first key is recorded for a peer (TOFU).
	// No out-of-band verification has been performed yet.
	TrustOnFirstUse

	// TrustKeyChanged is set when a new key is seen for a previously trusted peer.
	// This is a security alarm: the peer's identity may have changed or be under attack.
	TrustKeyChanged

	// TrustVerified is set when the application has explicitly confirmed the peer's
	// current key via an out-of-band mechanism (e.g. safety-number comparison).
	TrustVerified
)

// String returns a human-readable description of the trust state.
func (ts TrustState) String() string {
	switch ts {
	case TrustUnknown:
		return "unknown"
	case TrustOnFirstUse:
		return "trust-on-first-use"
	case TrustKeyChanged:
		return "key-changed-alarm"
	case TrustVerified:
		return "verified"
	default:
		return "invalid"
	}
}

// PeerTrustRecord stores the trust state and known key for a single peer identity.
type PeerTrustRecord struct {
	// State is the current trust state.
	State TrustState
	// TrustedKey is the key that is currently considered trusted (the first-seen or
	// last-verified key).
	TrustedKey [32]byte
	// AlarmKey is populated when State == TrustKeyChanged. It holds the
	// newly-observed key that triggered the alarm.
	AlarmKey [32]byte
	// FirstSeen is when this record was created.
	FirstSeen time.Time
	// LastUpdated is when the state or key last changed.
	LastUpdated time.Time
}

// KeyChangeAlarmCallback is called when a peer presents a different key than the
// previously trusted one. The alarm must be acknowledged by the application
// (e.g. via TOFUStore.MarkVerified) before the new key becomes trusted.
//
// Parameters:
//   - peerID: the Curve25519 public key identifying the peer (stable identity)
//   - oldKey: the previously trusted signing/identity key
//   - newKey: the newly observed key that triggered the alarm
type KeyChangeAlarmCallback func(peerID, oldKey, newKey [32]byte)

// TOFUStore is a thread-safe store for peer trust records.
// It implements the Trust-On-First-Use security model and fires a
// KeyChangeAlarmCallback when a key mismatch is detected.
type TOFUStore struct {
	mu       sync.RWMutex
	records  map[[32]byte]*PeerTrustRecord
	onAlarm  KeyChangeAlarmCallback
}

// NewTOFUStore creates a new, empty TOFUStore with no alarm callback.
func NewTOFUStore() *TOFUStore {
	return &TOFUStore{
		records: make(map[[32]byte]*PeerTrustRecord),
	}
}

// SetAlarmCallback registers a callback that is invoked whenever a key-change
// alarm is triggered.  Replaces any previously registered callback.
// Safe to call from any goroutine.
func (s *TOFUStore) SetAlarmCallback(cb KeyChangeAlarmCallback) {
	s.mu.Lock()
	s.onAlarm = cb
	s.mu.Unlock()
}

// Observe records an observed key for a peer identity and returns the
// resulting trust state.
//
//   - If the peer is unknown, the key is recorded as TrustOnFirstUse.
//   - If the observed key matches the trusted key, the existing state is
//     returned unchanged.
//   - If the observed key differs from the trusted key, the state transitions
//     to TrustKeyChanged and the alarm callback is fired (if set).
//
// Returns the trust record after the observation.
func (s *TOFUStore) Observe(peerID, observedKey [32]byte) *PeerTrustRecord {
	s.mu.Lock()
	rec, exists := s.records[peerID]
	if !exists {
		rec = &PeerTrustRecord{
			State:       TrustOnFirstUse,
			TrustedKey:  observedKey,
			FirstSeen:   time.Now(),
			LastUpdated: time.Now(),
		}
		s.records[peerID] = rec
		s.mu.Unlock()
		return rec
	}

	if rec.TrustedKey == observedKey {
		s.mu.Unlock()
		return rec
	}

	// Key mismatch: raise alarm.
	oldKey := rec.TrustedKey
	rec.AlarmKey = observedKey
	rec.State = TrustKeyChanged
	rec.LastUpdated = time.Now()
	alarm := s.onAlarm
	s.mu.Unlock()

	if alarm != nil {
		alarm(peerID, oldKey, observedKey)
	}
	return rec
}

// Get returns the trust record for a peer identity, or nil if unknown.
func (s *TOFUStore) Get(peerID [32]byte) *PeerTrustRecord {
	s.mu.RLock()
	rec := s.records[peerID]
	s.mu.RUnlock()
	if rec == nil {
		return nil
	}
	copy := *rec
	return &copy
}

// State returns the current TrustState for a peer, or TrustUnknown if not seen.
func (s *TOFUStore) State(peerID [32]byte) TrustState {
	s.mu.RLock()
	rec := s.records[peerID]
	s.mu.RUnlock()
	if rec == nil {
		return TrustUnknown
	}
	return rec.State
}

// MarkVerified transitions a peer's trust state to TrustVerified, accepting
// the current TrustedKey (or, if a key-change alarm is active, accepting the
// AlarmKey as the new trusted key).
//
// This is the correct way for an application to acknowledge a key-change alarm:
// call MarkVerified after the user confirms the safety number matches.
//
// Returns true if the record was found and updated, false if the peer is unknown.
func (s *TOFUStore) MarkVerified(peerID [32]byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, exists := s.records[peerID]
	if !exists {
		return false
	}
	if rec.State == TrustKeyChanged {
		rec.TrustedKey = rec.AlarmKey
		rec.AlarmKey = [32]byte{}
	}
	rec.State = TrustVerified
	rec.LastUpdated = time.Now()
	return true
}

// Remove deletes the trust record for a peer (e.g. when a friend is removed).
func (s *TOFUStore) Remove(peerID [32]byte) {
	s.mu.Lock()
	delete(s.records, peerID)
	s.mu.Unlock()
}
