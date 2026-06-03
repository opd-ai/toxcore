package crypto

// Package crypto – PQXDH post-quantum hybrid initial key agreement.
//
// PQXDH extends the classical X3DH transcript with an ML-KEM-768
// shared secret (from cloudflare/circl), mixing both secrets into the
// session root key:
//
//	SK = HKDF-SHA256(F ‖ DH1 ‖ DH2 ‖ DH3 [‖ DH4] ‖ SS_pq_spk [‖ SS_pq_opk])
//
// This makes the initial session secret unrecoverable without breaking
// both X25519 (classical) and ML-KEM-768 (post-quantum), eliminating
// harvest-now-decrypt-later exposure of the session root.
//
// Capability gating: PQXDH is enabled only when both peers advertise
// CapPQXDH in the signed version-negotiation packet.  Classical-only
// X3DH (X3DHInitiate / X3DHRespond) remains the path for peers that
// do not advertise PQ support.

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	mlkem768 "github.com/cloudflare/circl/kem/mlkem/mlkem768"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// ML-KEM-768 wire sizes (FIPS 203 / cloudflare/circl).
const (
	// MLKEMPublicKeySize is the byte length of an ML-KEM-768 encapsulation key.
	MLKEMPublicKeySize = 1184
	// MLKEMCiphertextSize is the byte length of an ML-KEM-768 ciphertext.
	MLKEMCiphertextSize = 1088
	// MLKEMSharedSecretSize is the byte length of an ML-KEM-768 shared secret.
	MLKEMSharedSecretSize = 32
)

// PQPreKey holds a serialized ML-KEM-768 encapsulation (public) key and its
// corresponding decapsulation (private) key.  The public half is distributed
// in pre-key bundles; the private half is kept on-device and wiped after use.
type PQPreKey struct {
	// Public is the ML-KEM-768 encapsulation key (1184 bytes).
	Public [MLKEMPublicKeySize]byte
	// Private is the ML-KEM-768 decapsulation key (2400 bytes).
	Private [2400]byte
}

// SignedPQPreKey is an ML-KEM-768 encapsulation key that has been
// Ed25519-signed by the owner's long-term identity key.  It mirrors the
// role of SignedPreKey for the classical path.
type SignedPQPreKey struct {
	// ID uniquely identifies this key within the owner's PQ pre-key history.
	ID uint32
	// PublicKey is the ML-KEM-768 encapsulation key bytes.
	PublicKey [MLKEMPublicKeySize]byte
	// Signature is the Ed25519 signature of PublicKey made with the owner's
	// identity private key.
	Signature Signature
	// SignerPK is the Ed25519 public key derived from the owner's identity
	// private key; receivers verify that SignerPK matches the sender's known
	// identity before trusting the bundle.
	SignerPK [32]byte
}

// GeneratePQPreKey generates a fresh ML-KEM-768 key pair.
func GeneratePQPreKey() (*PQPreKey, error) {
	pub, priv, err := mlkem768.GenerateKeyPair(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("pqxdh: generate ML-KEM-768 key pair: %w", err)
	}

	pubBytes, err := pub.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("pqxdh: marshal ML-KEM-768 public key: %w", err)
	}
	privBytes, err := priv.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("pqxdh: marshal ML-KEM-768 private key: %w", err)
	}

	pk := &PQPreKey{}
	copy(pk.Public[:], pubBytes)
	copy(pk.Private[:], privBytes)
	ZeroBytes(privBytes)
	ZeroBytes(pubBytes)
	return pk, nil
}

// NewSignedPQPreKey generates a fresh ML-KEM-768 pre-key and signs the
// public half with the caller's Ed25519 identity key.
func NewSignedPQPreKey(id uint32, identityPrivKey [32]byte) (*SignedPQPreKey, *PQPreKey, error) {
	pqk, err := GeneratePQPreKey()
	if err != nil {
		return nil, nil, err
	}

	sig, err := Sign(pqk.Public[:], identityPrivKey)
	if err != nil {
		return nil, nil, fmt.Errorf("pqxdh: sign PQ pre-key: %w", err)
	}

	signerPK := GetSignaturePublicKey(identityPrivKey)

	spq := &SignedPQPreKey{
		ID:       id,
		SignerPK: signerPK,
	}
	copy(spq.PublicKey[:], pqk.Public[:])
	copy(spq.Signature[:], sig[:])

	return spq, pqk, nil
}

// Verify checks that the signed PQ pre-key's public key was signed by the
// claimed signer.  Returns an error if the signature is invalid.
func (s *SignedPQPreKey) Verify() error {
	valid, err := Verify(s.PublicKey[:], s.Signature, s.SignerPK)
	if err != nil {
		return fmt.Errorf("pqxdh: PQ pre-key signature error: %w", err)
	}
	if !valid {
		return errors.New("pqxdh: PQ pre-key has invalid signature")
	}
	return nil
}

// PQXDHInitiatorParams bundles the parameters needed for a PQXDH initiation.
// Classical fields mirror X3DHInitiatorParams; PQ fields add the ML-KEM-768
// signed last-resort pre-key and optional one-time PQ pre-key.
type PQXDHInitiatorParams struct {
	// Classical X3DH fields – see X3DHInitiatorParams for semantics.
	SelfIdentityPrivate      [32]byte
	SelfEphemeralPrivate     [32]byte
	PeerIdentityPublic       [32]byte
	PeerSignedPreKeyPublic   [32]byte
	PeerOneTimePreKeyPublic  *[32]byte // nil → 3-DH classical fallback

	// PQ fields.
	// PeerPQSignedPreKey is the peer's signed ML-KEM-768 encapsulation key.
	// Its signature must be verified by the caller before passing here.
	PeerPQSignedPreKey [MLKEMPublicKeySize]byte
	// PeerPQOneTimePreKey is an optional one-time ML-KEM-768 encapsulation key.
	// If nil, only the signed last-resort PQ pre-key is used.
	PeerPQOneTimePreKey *[MLKEMPublicKeySize]byte
}

// PQXDHResponderParams bundles the parameters needed for PQXDH response.
type PQXDHResponderParams struct {
	// Classical X3DH fields.
	SelfIdentityPrivate       [32]byte
	SelfSignedPreKeyPrivate   [32]byte
	SelfOneTimePreKeyPrivate  *[32]byte // nil → 3-DH classical fallback

	PeerIdentityPublic  [32]byte
	PeerEphemeralPublic [32]byte

	// PQ fields.
	// SelfPQSignedPreKeyPrivate is the decapsulation (private) key corresponding
	// to the signed PQ pre-key the initiator encapsulated to.
	SelfPQSignedPreKeyPrivate [2400]byte
	// SelfPQOneTimePreKeyPrivate is the decapsulation key for the one-time PQ
	// pre-key (if the initiator used one).  Must be nil when no one-time PQ
	// pre-key was used.
	SelfPQOneTimePreKeyPrivate *[2400]byte
}

// PQXDHResult carries the outputs of a successful PQXDH initiation.
type PQXDHResult struct {
	// SK is the 32-byte hybrid session root key ready to seed the Double Ratchet.
	SK [32]byte
	// KEMCiphertextSPK is the ML-KEM-768 ciphertext encapsulated to the peer's
	// signed PQ pre-key.  Must be included in the initial message so the
	// responder can decapsulate.
	KEMCiphertextSPK [MLKEMCiphertextSize]byte
	// KEMCiphertextOPK is the ML-KEM-768 ciphertext encapsulated to the peer's
	// one-time PQ pre-key.  Empty (all zeros) when no PQ-OPK was used.
	KEMCiphertextOPK [MLKEMCiphertextSize]byte
	// UsedPQOPK reports whether a PQ one-time pre-key was included.
	UsedPQOPK bool
}

// pqxdhHKDF derives the PQXDH session key from the classical X3DH transcript
// plus the ML-KEM-768 shared secret(s).
//
// transcript = F ‖ DH1 ‖ DH2 ‖ DH3 [‖ DH4] ‖ SS_pq_spk [‖ SS_pq_opk]
func pqxdhHKDF(transcript []byte) ([32]byte, error) {
	hkdfReader := hkdf.New(sha256.New, transcript, nil, []byte("TOX_PQXDH_SHARED_SECRET_V1"))
	sk32 := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, sk32); err != nil {
		return [32]byte{}, fmt.Errorf("pqxdh: HKDF failed: %w", err)
	}
	var sk [32]byte
	copy(sk[:], sk32)
	ZeroBytes(sk32)
	return sk, nil
}

// pqEncapsulate encapsulates to the given ML-KEM-768 public key bytes.
// Returns the shared secret and the ciphertext.  Both the ciphertext and
// the shared secret are returned as fixed-size arrays; the shared secret is
// appended to transcript.
func pqEncapsulate(pubKeyBytes [MLKEMPublicKeySize]byte) (ss [MLKEMSharedSecretSize]byte, ct [MLKEMCiphertextSize]byte, err error) {
	ek := new(mlkem768.PublicKey)
	if unpackErr := ek.Unpack(pubKeyBytes[:]); unpackErr != nil {
		return [32]byte{}, [MLKEMCiphertextSize]byte{}, fmt.Errorf("pqxdh: unpack ML-KEM-768 public key: %w", unpackErr)
	}

	ssBuf := make([]byte, MLKEMSharedSecretSize)
	ctBuf := make([]byte, MLKEMCiphertextSize)
	ek.EncapsulateTo(ctBuf, ssBuf, nil) // nil seed → use crypto/rand internally

	copy(ss[:], ssBuf)
	copy(ct[:], ctBuf)
	ZeroBytes(ssBuf)
	ZeroBytes(ctBuf)
	return ss, ct, nil
}

// pqDecapsulate decapsulates the given ML-KEM-768 ciphertext using the
// private key bytes.  The shared secret is appended to transcript and the
// private key bytes are zeroized.
func pqDecapsulate(privKeyBytes *[2400]byte, ct [MLKEMCiphertextSize]byte) (ss [MLKEMSharedSecretSize]byte, err error) {
	defer ZeroBytes(privKeyBytes[:])

	dk := new(mlkem768.PrivateKey)
	if unpackErr := dk.Unpack(privKeyBytes[:]); unpackErr != nil {
		return [32]byte{}, fmt.Errorf("pqxdh: unpack ML-KEM-768 private key: %w", unpackErr)
	}

	ssBuf := make([]byte, MLKEMSharedSecretSize)
	dk.DecapsulateTo(ssBuf, ct[:])
	copy(ss[:], ssBuf)
	ZeroBytes(ssBuf)
	return ss, nil
}

// PQXDHInitiate computes the PQXDH hybrid initial agreement from the
// initiator's perspective.
//
// It performs the classical X3DH transcript (DH1..DH3, optionally DH4),
// then encapsulates to the peer's signed PQ pre-key (and optionally a PQ
// one-time pre-key), and derives SK via HKDF over the concatenated transcript.
//
// The caller is responsible for verifying PeerPQSignedPreKey's Ed25519
// signature (via SignedPQPreKey.Verify) before calling this function.
func PQXDHInitiate(params PQXDHInitiatorParams) (PQXDHResult, error) {
	// --- Input validation ---
	if params.SelfIdentityPrivate == [32]byte{} {
		return PQXDHResult{}, errors.New("pqxdh: self identity private key is empty")
	}
	if params.SelfEphemeralPrivate == [32]byte{} {
		return PQXDHResult{}, errors.New("pqxdh: self ephemeral private key is empty")
	}
	if params.PeerIdentityPublic == [32]byte{} {
		return PQXDHResult{}, errors.New("pqxdh: peer identity public key is empty")
	}
	if params.PeerSignedPreKeyPublic == [32]byte{} {
		return PQXDHResult{}, errors.New("pqxdh: peer signed pre-key is empty")
	}
	if params.PeerPQSignedPreKey == [MLKEMPublicKeySize]byte{} {
		return PQXDHResult{}, errors.New("pqxdh: peer PQ signed pre-key is empty")
	}

	// --- Classical X3DH transcript (DH1..DH3 [‖ DH4]) ---
	dh := func(priv, pub [32]byte) ([32]byte, error) {
		defer ZeroBytes(priv[:])
		res, err := curve25519.X25519(priv[:], pub[:])
		if err != nil {
			return [32]byte{}, err
		}
		defer ZeroBytes(res)
		var out [32]byte
		copy(out[:], res)
		return out, nil
	}

	// DH1 = DH(IK_A, SPK_B)
	dh1, err := dh(params.SelfIdentityPrivate, params.PeerSignedPreKeyPublic)
	if err != nil {
		return PQXDHResult{}, fmt.Errorf("pqxdh: DH1: %w", err)
	}
	defer ZeroBytes(dh1[:])

	// DH2 = DH(EK_A, IK_B)
	dh2, err := dh(params.SelfEphemeralPrivate, params.PeerIdentityPublic)
	if err != nil {
		return PQXDHResult{}, fmt.Errorf("pqxdh: DH2: %w", err)
	}
	defer ZeroBytes(dh2[:])

	// DH3 = DH(EK_A, SPK_B)
	dh3, err := dh(params.SelfEphemeralPrivate, params.PeerSignedPreKeyPublic)
	if err != nil {
		return PQXDHResult{}, fmt.Errorf("pqxdh: DH3: %w", err)
	}
	defer ZeroBytes(dh3[:])

	// DH4 = DH(EK_A, OPK_B) if available
	var dh4 [32]byte
	hasDH4 := params.PeerOneTimePreKeyPublic != nil && *params.PeerOneTimePreKeyPublic != [32]byte{}
	if hasDH4 {
		dh4, err = dh(params.SelfEphemeralPrivate, *params.PeerOneTimePreKeyPublic)
		if err != nil {
			return PQXDHResult{}, fmt.Errorf("pqxdh: DH4: %w", err)
		}
	}
	defer ZeroBytes(dh4[:])

	// --- ML-KEM-768 encapsulation ---
	// Encapsulate to the peer's signed PQ pre-key.
	ssSPK, ctSPK, err := pqEncapsulate(params.PeerPQSignedPreKey)
	if err != nil {
		return PQXDHResult{}, fmt.Errorf("pqxdh: encapsulate to PQ SPK: %w", err)
	}
	defer ZeroBytes(ssSPK[:])

	var ctOPK [MLKEMCiphertextSize]byte
	var ssOPK [MLKEMSharedSecretSize]byte
	usedPQOPK := params.PeerPQOneTimePreKey != nil && *params.PeerPQOneTimePreKey != [MLKEMPublicKeySize]byte{}
	if usedPQOPK {
		ssOPK, ctOPK, err = pqEncapsulate(*params.PeerPQOneTimePreKey)
		if err != nil {
			return PQXDHResult{}, fmt.Errorf("pqxdh: encapsulate to PQ OPK: %w", err)
		}
		defer ZeroBytes(ssOPK[:])
	}

	// --- Hybrid transcript ---
	// transcript = F ‖ DH1 ‖ DH2 ‖ DH3 [‖ DH4] ‖ SS_pq_spk [‖ SS_pq_opk]
	transcriptCap := 32 * 7 // F + DH1..DH4 + ssSPK + ssOPK (max transcript length)
	transcript := make([]byte, 0, transcriptCap)
	transcript = append(transcript, X3DHDomainSeparator[:]...)
	transcript = append(transcript, dh1[:]...)
	transcript = append(transcript, dh2[:]...)
	transcript = append(transcript, dh3[:]...)
	if hasDH4 {
		transcript = append(transcript, dh4[:]...)
	}
	transcript = append(transcript, ssSPK[:]...)
	if usedPQOPK {
		transcript = append(transcript, ssOPK[:]...)
	}
	defer ZeroBytes(transcript)

	sk, err := pqxdhHKDF(transcript)
	if err != nil {
		return PQXDHResult{}, err
	}

	return PQXDHResult{
		SK:               sk,
		KEMCiphertextSPK: ctSPK,
		KEMCiphertextOPK: ctOPK,
		UsedPQOPK:        usedPQOPK,
	}, nil
}

// PQXDHRespond computes the PQXDH hybrid initial agreement from the
// responder's perspective, deriving the same SK as the initiator.
//
// The caller must supply the decapsulation keys corresponding to the PQ
// pre-keys the initiator encapsulated to (read from the initial message
// header), and must enforce single-use / zeroization of PQ-OPKs after
// this call.
func PQXDHRespond(params PQXDHResponderParams, kemCtSPK [MLKEMCiphertextSize]byte, kemCtOPK *[MLKEMCiphertextSize]byte) ([32]byte, error) {
	// --- Input validation ---
	if params.SelfIdentityPrivate == [32]byte{} {
		return [32]byte{}, errors.New("pqxdh: self identity private key is empty")
	}
	if params.SelfSignedPreKeyPrivate == [32]byte{} {
		return [32]byte{}, errors.New("pqxdh: self signed pre-key private is empty")
	}
	if params.PeerIdentityPublic == [32]byte{} {
		return [32]byte{}, errors.New("pqxdh: peer identity public key is empty")
	}
	if params.PeerEphemeralPublic == [32]byte{} {
		return [32]byte{}, errors.New("pqxdh: peer ephemeral public key is empty")
	}
	if params.SelfPQSignedPreKeyPrivate == [2400]byte{} {
		return [32]byte{}, errors.New("pqxdh: self PQ signed pre-key private is empty")
	}

	// --- Classical X3DH transcript ---
	dh := func(priv, pub [32]byte) ([32]byte, error) {
		defer ZeroBytes(priv[:])
		res, err := curve25519.X25519(priv[:], pub[:])
		if err != nil {
			return [32]byte{}, err
		}
		defer ZeroBytes(res)
		var out [32]byte
		copy(out[:], res)
		return out, nil
	}

	// DH1 = DH(SPK_B, IK_A)
	dh1, err := dh(params.SelfSignedPreKeyPrivate, params.PeerIdentityPublic)
	if err != nil {
		return [32]byte{}, fmt.Errorf("pqxdh: DH1: %w", err)
	}
	defer ZeroBytes(dh1[:])

	// DH2 = DH(IK_B, EK_A)
	dh2, err := dh(params.SelfIdentityPrivate, params.PeerEphemeralPublic)
	if err != nil {
		return [32]byte{}, fmt.Errorf("pqxdh: DH2: %w", err)
	}
	defer ZeroBytes(dh2[:])

	// DH3 = DH(SPK_B, EK_A)
	dh3, err := dh(params.SelfSignedPreKeyPrivate, params.PeerEphemeralPublic)
	if err != nil {
		return [32]byte{}, fmt.Errorf("pqxdh: DH3: %w", err)
	}
	defer ZeroBytes(dh3[:])

	// DH4 = DH(OPK_B, EK_A) if we have a one-time pre-key
	var dh4 [32]byte
	hasDH4 := params.SelfOneTimePreKeyPrivate != nil && *params.SelfOneTimePreKeyPrivate != [32]byte{}
	if hasDH4 {
		dh4, err = dh(*params.SelfOneTimePreKeyPrivate, params.PeerEphemeralPublic)
		if err != nil {
			return [32]byte{}, fmt.Errorf("pqxdh: DH4: %w", err)
		}
	}
	defer ZeroBytes(dh4[:])

	// --- ML-KEM-768 decapsulation ---
	// We need a copy of the private key bytes since pqDecapsulate zeroizes them.
	spkPrivCopy := params.SelfPQSignedPreKeyPrivate
	ssSPK, err := pqDecapsulate(&spkPrivCopy, kemCtSPK)
	if err != nil {
		return [32]byte{}, fmt.Errorf("pqxdh: decapsulate PQ SPK: %w", err)
	}
	defer ZeroBytes(ssSPK[:])

	var ssOPK [MLKEMSharedSecretSize]byte
	usedPQOPK := kemCtOPK != nil && params.SelfPQOneTimePreKeyPrivate != nil &&
		*params.SelfPQOneTimePreKeyPrivate != [2400]byte{}
	if usedPQOPK {
		opkPrivCopy := *params.SelfPQOneTimePreKeyPrivate
		ssOPK, err = pqDecapsulate(&opkPrivCopy, *kemCtOPK)
		if err != nil {
			return [32]byte{}, fmt.Errorf("pqxdh: decapsulate PQ OPK: %w", err)
		}
	}
	defer ZeroBytes(ssOPK[:])

	// --- Hybrid transcript (must match initiator's order) ---
	transcript := make([]byte, 0, 32*7)
	transcript = append(transcript, X3DHDomainSeparator[:]...)
	transcript = append(transcript, dh1[:]...)
	transcript = append(transcript, dh2[:]...)
	transcript = append(transcript, dh3[:]...)
	if hasDH4 {
		transcript = append(transcript, dh4[:]...)
	}
	transcript = append(transcript, ssSPK[:]...)
	if usedPQOPK {
		transcript = append(transcript, ssOPK[:]...)
	}
	defer ZeroBytes(transcript)

	return pqxdhHKDF(transcript)
}
