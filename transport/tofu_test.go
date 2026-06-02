package transport

import (
	"sync/atomic"
	"testing"
)

func TestTOFUStore_FirstObservation(t *testing.T) {
	s := NewTOFUStore()
	var peer [32]byte
	peer[0] = 1
	var key [32]byte
	key[0] = 0xAA

	rec := s.Observe(peer, key)
	if rec.State != TrustOnFirstUse {
		t.Errorf("expected TrustOnFirstUse, got %s", rec.State)
	}
	if rec.TrustedKey != key {
		t.Errorf("trusted key mismatch")
	}
}

func TestTOFUStore_SameKeyReobservation(t *testing.T) {
	s := NewTOFUStore()
	var peer, key [32]byte
	peer[0], key[0] = 1, 0xAA
	s.Observe(peer, key)

	rec := s.Observe(peer, key)
	if rec.State != TrustOnFirstUse {
		t.Errorf("expected TrustOnFirstUse after re-observation with same key, got %s", rec.State)
	}
}

func TestTOFUStore_KeyChangedAlarm(t *testing.T) {
	s := NewTOFUStore()
	var peer, key1, key2 [32]byte
	peer[0], key1[0], key2[0] = 1, 0xAA, 0xBB

	var alarmCount int32
	var gotPeer, gotOld, gotNew [32]byte
	s.SetAlarmCallback(func(p, old, newK [32]byte) {
		atomic.AddInt32(&alarmCount, 1)
		gotPeer, gotOld, gotNew = p, old, newK
	})

	s.Observe(peer, key1) // first use
	rec := s.Observe(peer, key2)

	if rec.State != TrustKeyChanged {
		t.Errorf("expected TrustKeyChanged, got %s", rec.State)
	}
	if rec.AlarmKey != key2 {
		t.Errorf("alarm key should be the new observed key")
	}
	if atomic.LoadInt32(&alarmCount) != 1 {
		t.Errorf("expected 1 alarm callback, got %d", alarmCount)
	}
	if gotPeer != peer || gotOld != key1 || gotNew != key2 {
		t.Errorf("alarm callback received wrong arguments")
	}
}

func TestTOFUStore_MarkVerified_Normal(t *testing.T) {
	s := NewTOFUStore()
	var peer, key [32]byte
	peer[0], key[0] = 1, 0xAA
	s.Observe(peer, key)

	ok := s.MarkVerified(peer)
	if !ok {
		t.Fatal("MarkVerified returned false for known peer")
	}
	if s.State(peer) != TrustVerified {
		t.Errorf("expected TrustVerified, got %s", s.State(peer))
	}
}

func TestTOFUStore_MarkVerified_AfterAlarm(t *testing.T) {
	s := NewTOFUStore()
	var peer, key1, key2 [32]byte
	peer[0], key1[0], key2[0] = 1, 0xAA, 0xBB
	s.Observe(peer, key1)
	s.Observe(peer, key2) // triggers alarm

	s.MarkVerified(peer)

	rec := s.Get(peer)
	if rec == nil {
		t.Fatal("record should exist")
	}
	if rec.State != TrustVerified {
		t.Errorf("expected TrustVerified after acknowledging alarm, got %s", rec.State)
	}
	// The alarm key should now be the trusted key.
	if rec.TrustedKey != key2 {
		t.Errorf("trusted key should be updated to the alarm key after verification")
	}
}

func TestTOFUStore_MarkVerified_Unknown(t *testing.T) {
	s := NewTOFUStore()
	var peer [32]byte
	peer[0] = 99
	if s.MarkVerified(peer) {
		t.Error("MarkVerified should return false for unknown peer")
	}
}

func TestTOFUStore_Remove(t *testing.T) {
	s := NewTOFUStore()
	var peer, key [32]byte
	peer[0], key[0] = 1, 0xAA
	s.Observe(peer, key)
	s.Remove(peer)
	if s.State(peer) != TrustUnknown {
		t.Errorf("expected TrustUnknown after remove, got %s", s.State(peer))
	}
}

func TestTOFUStore_Get_ReturnsNilForUnknown(t *testing.T) {
	s := NewTOFUStore()
	var peer [32]byte
	if s.Get(peer) != nil {
		t.Error("Get should return nil for unknown peer")
	}
}

func TestTrustState_String(t *testing.T) {
	cases := []struct {
		state TrustState
		want  string
	}{
		{TrustUnknown, "unknown"},
		{TrustOnFirstUse, "trust-on-first-use"},
		{TrustKeyChanged, "key-changed-alarm"},
		{TrustVerified, "verified"},
		{TrustState(99), "invalid"},
	}
	for _, c := range cases {
		if got := c.state.String(); got != c.want {
			t.Errorf("TrustState(%d).String() = %q, want %q", c.state, got, c.want)
		}
	}
}
