package ratchet

import (
	"errors"
)

// skippedKey identifies a skipped message key by ratchet public key + counter.
type skippedKey struct {
	dhPub [32]byte
	n     uint32
}

// skippedKeyStore is a bounded map of out-of-order message keys.
// Entries are evicted FIFO once MaxSkippedKeys is reached.
type skippedKeyStore struct {
	keys  map[skippedKey][32]byte
	order []skippedKey // insertion order for FIFO eviction
}

// newSkippedKeyStore allocates an empty store.
func newSkippedKeyStore() *skippedKeyStore {
	return &skippedKeyStore{
		keys:  make(map[skippedKey][32]byte),
		order: make([]skippedKey, 0, MaxSkippedKeys),
	}
}

func (s *skippedKeyStore) clone() *skippedKeyStore {
	clone := &skippedKeyStore{
		keys:  make(map[skippedKey][32]byte, len(s.keys)),
		order: append(make([]skippedKey, 0, len(s.order)), s.order...),
	}
	for k, v := range s.keys {
		clone.keys[k] = v
	}
	return clone
}

// get retrieves and removes a skipped message key.  The second return value
// is true only if the key was found.
func (s *skippedKeyStore) get(dhPub [32]byte, n uint32) ([32]byte, bool) {
	k := skippedKey{dhPub, n}
	mk, ok := s.keys[k]
	if ok {
		s.keys[k] = [32]byte{}
		delete(s.keys, k)
		s.removeFromOrder(k)
	}
	return mk, ok
}

// store saves a skipped message key, evicting the oldest entry if the store
// is at capacity.  Returns an error if the store would exceed the hard limit.
func (s *skippedKeyStore) store(dhPub [32]byte, n uint32, mk [32]byte) error {
	if len(s.keys) >= MaxSkippedKeys {
		if len(s.order) == 0 {
			return errors.New("ratchet: skipped key store full")
		}
		oldest := s.order[0]
		s.order = s.order[1:]
		s.keys[oldest] = [32]byte{}
		delete(s.keys, oldest)
	}
	k := skippedKey{dhPub, n}
	s.keys[k] = mk
	s.order = append(s.order, k)
	return nil
}

// removeFromOrder removes k from the insertion-order slice.
func (s *skippedKeyStore) removeFromOrder(k skippedKey) {
	for i, v := range s.order {
		if v == k {
			s.order = append(s.order[:i], s.order[i+1:]...)
			return
		}
	}
}
