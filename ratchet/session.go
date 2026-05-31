package ratchet

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/crypto/curve25519"

	"github.com/opd-ai/toxcore/crypto"
)

// KeyPair is a Curve25519 key pair used as the DH ratchet key.
type KeyPair struct {
	Public  [32]byte
	Private [32]byte
}

// GenerateKeyPair creates a fresh ephemeral Curve25519 key pair for use as a
// ratchet key.  Private bytes are generated from crypto/rand.
func GenerateKeyPair() (KeyPair, error) {
	var kp KeyPair
	if _, err := rand.Read(kp.Private[:]); err != nil {
		return KeyPair{}, fmt.Errorf("ratchet: key generation failed: %w", err)
	}
	curve25519.ScalarBaseMult(&kp.Public, &kp.Private)
	return kp, nil
}

// dh computes the X25519 shared secret between our private key and their
// public key, then zeros both input keys.
func dh(ourPrivate, theirPublic [32]byte) ([32]byte, error) {
	// Zero our private key after use; theirPublic is a public value and does
	// not require secure deletion.
	defer crypto.ZeroBytes(ourPrivate[:])
	out, err := curve25519.X25519(ourPrivate[:], theirPublic[:])
	if err != nil {
		return [32]byte{}, fmt.Errorf("ratchet: DH failed: %w", err)
	}
	var result [32]byte
	copy(result[:], out)
	return result, nil
}

// Session holds the full Double Ratchet state for one end of a conversation.
// Use [InitInitiator] or [InitRecipient] to create a Session.
type Session struct {
	mu sync.Mutex

	dhs KeyPair  // our current sending ratchet key pair
	dhr [32]byte // remote party's current ratchet public key
	rk  [32]byte // root key
	cks [32]byte // sending chain key
	ckr [32]byte // receiving chain key

	ns uint32 // send counter
	nr uint32 // receive counter
	pn uint32 // previous sending-chain length

	skipped *skippedKeyStore

	// dhrSet is true once we have received the remote party's first ratchet key.
	dhrSet bool
	cksSet bool
}

// InitInitiator creates a Session as the conversation initiator (Alice).
// sharedKey is the Diffie-Hellman output from the prior key exchange (e.g.,
// Noise-IK).  theirPub is the remote party's initial ratchet public key.
//
// On return the session is ready to send immediately.
func InitInitiator(sharedKey [32]byte, theirPub [32]byte) (*Session, error) {
	kp, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	dhOut, err := dh(kp.Private, theirPub)
	if err != nil {
		return nil, err
	}

	// Zeroize copies before handing off to kdfRootChain (which zeros them too).
	skCopy := sharedKey
	rk, cks, err := kdfRootChain(skCopy, dhOut)
	if err != nil {
		return nil, err
	}

	s := &Session{
		dhs:     kp,
		dhr:     theirPub,
		dhrSet:  true,
		cksSet:  true,
		rk:      rk,
		cks:     cks,
		skipped: newSkippedKeyStore(),
	}
	return s, nil
}

// InitRecipient creates a Session as the conversation responder (Bob).
// sharedKey is the Diffie-Hellman output from the prior key exchange.
// myKeyPair is Bob's ratchet key pair, whose public key Alice must know in
// advance (e.g., provided via a pre-key bundle).
//
// On return the session has no sending chain and is ready to receive; the
// sending chain is established after the first DH ratchet step.
func InitRecipient(sharedKey [32]byte, myKeyPair KeyPair) *Session {
	return &Session{
		dhs:     myKeyPair,
		rk:      sharedKey,
		skipped: newSkippedKeyStore(),
	}
}

// RatchetEncrypt encrypts plaintext and returns the message header and
// ciphertext.  ad is additional data that is authenticated but not encrypted.
// The returned Header must be transmitted alongside the ciphertext and passed
// verbatim to [Session.RatchetDecrypt] on the receiving end.
func (s *Session) RatchetEncrypt(plaintext, ad []byte) (Header, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cksSet {
		return Header{}, nil, errors.New("ratchet: sending chain not initialized")
	}

	var newCKs, mk [32]byte
	newCKs, mk = kdfChain(s.cks)
	s.cks = newCKs

	h := Header{DHPub: s.dhs.Public, PN: s.pn, N: s.ns}
	s.ns++

	// Authenticated data = AD || serialised header.
	associatedData := buildAD(ad, h)
	ct, err := encryptWithMsgKey(mk, plaintext, associatedData)
	if err != nil {
		return Header{}, nil, err
	}
	return h, ct, nil
}

// RatchetDecrypt decrypts ciphertext produced by the remote party's
// RatchetEncrypt.  ad must be the same value that was passed to RatchetEncrypt.
// Message keys are deleted immediately after use.
func (s *Session) RatchetDecrypt(h Header, ciphertext, ad []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Fast path: skipped key from a previous DH epoch or out-of-order delivery.
	if mk, ok := s.skipped.get(h.DHPub, h.N); ok {
		return decryptWithMsgKey(mk, ciphertext, buildAD(ad, h))
	}

	// New DH ratchet public key → advance ratchet.
	if !s.dhrSet || h.DHPub != s.dhr {
		if err := s.skipMessageKeys(h.PN); err != nil {
			return nil, err
		}
		if err := s.dhRatchetStep(h.DHPub); err != nil {
			return nil, err
		}
	}

	if err := s.skipMessageKeys(h.N); err != nil {
		return nil, err
	}

	var newCKr, mk [32]byte
	newCKr, mk = kdfChain(s.ckr)
	s.ckr = newCKr
	s.nr++

	return decryptWithMsgKey(mk, ciphertext, buildAD(ad, h))
}

// dhRatchetStep performs a single DH ratchet step, advancing the root chain
// twice: once to derive the receiving chain key (from old DHs + newDHr) and
// once to derive the new sending chain key (from fresh DHs + newDHr).
// All fallible operations are completed before any state mutation so that a
// failure leaves the session unchanged (L-12).
func (s *Session) dhRatchetStep(newDHr [32]byte) error {
	// Step 1: derive receive-chain keys using the current sending key pair.
	dhOut, err := dh(s.dhs.Private, newDHr)
	if err != nil {
		return err
	}
	rk1, ckr, err := kdfRootChain(s.rk, dhOut)
	if err != nil {
		return err
	}

	// Step 2: generate fresh sending key pair.
	newKP, err := GenerateKeyPair()
	if err != nil {
		return err
	}

	// Step 3: derive send-chain keys using the fresh key pair.
	dhOut2, err := dh(newKP.Private, newDHr)
	if err != nil {
		crypto.ZeroBytes(newKP.Private[:])
		return err
	}
	rk2, cks, err := kdfRootChain(rk1, dhOut2)
	if err != nil {
		crypto.ZeroBytes(newKP.Private[:])
		return err
	}

	// All fallible steps succeeded — commit state atomically.
	s.pn = s.ns
	s.ns = 0
	s.nr = 0
	s.dhr = newDHr
	s.dhrSet = true
	s.rk = rk2
	s.ckr = ckr
	crypto.ZeroBytes(s.dhs.Private[:])
	s.dhs = newKP
	s.cks = cks
	s.cksSet = true
	return nil
}

// skipMessageKeys advances the receiving chain to msgNum, storing each skipped
// message key in the skipped-key store.
func (s *Session) skipMessageKeys(msgNum uint32) error {
	// Compare in uint32 space to handle wrap-around correctly: msgNum may have
	// rolled over past 2^32 while s.nr has not (or vice versa), so casting both
	// to uint64 before subtracting would give wrong results.  Instead, compute
	// the gap as a uint32 difference; if msgNum <= s.nr there is no gap.
	if msgNum > s.nr && uint32(msgNum-s.nr) > uint32(MaxSkippedKeys) {
		return fmt.Errorf("ratchet: gap of %d skipped keys exceeds limit %d",
			uint32(msgNum-s.nr), MaxSkippedKeys)
	}
	for s.nr < msgNum {
		if !s.dhrSet {
			return errors.New("ratchet: cannot skip without a receiving chain")
		}
		var newCKr, mk [32]byte
		newCKr, mk = kdfChain(s.ckr)
		s.ckr = newCKr
		if err := s.skipped.store(s.dhr, s.nr, mk); err != nil {
			crypto.ZeroBytes(mk[:])
			return err
		}
		s.nr++
	}
	return nil
}

// buildAD concatenates the caller-supplied associated data with the encoded
// header to produce the full AEAD additional-data input.
func buildAD(ad []byte, h Header) []byte {
	enc := h.Encode()
	combined := make([]byte, len(ad)+len(enc))
	copy(combined, ad)
	copy(combined[len(ad):], enc)
	return combined
}
