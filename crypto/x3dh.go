package crypto

import (
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// X3DHDomainSeparator is the 32-byte domain prefix (32×0xFF) used in the X3DH HKDF derivation.
var X3DHDomainSeparator = [32]byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// X3DHInitiatorParams bundles the parameters needed for X3DH initiation.
// All public keys should be X25519 (Curve25519) keys, not Ed25519.
// Use DeriveX25519FromEd25519Seed to convert Ed25519 identity keys to X25519 for inclusion in bundles.
type X3DHInitiatorParams struct {
	SelfIdentityPrivate     [32]byte  // Our Curve25519 identity private key
	SelfEphemeralPrivate    [32]byte  // Our ephemeral Curve25519 private key
	PeerIdentityPublic      [32]byte  // Peer's Curve25519 identity public key
	PeerSignedPreKeyPublic  [32]byte  // Peer's signed pre-key (Curve25519)
	PeerOneTimePreKeyPublic *[32]byte // Peer's one-time pre-key (nullable for 3-DH)
	PeerOneTimePreKeyID     uint32    // ID of the one-time pre-key (0 if not used)
}

// X3DHResponderParams bundles the parameters needed for X3DH response.
// All public keys should be X25519 (Curve25519) keys.
type X3DHResponderParams struct {
	SelfIdentityPrivate      [32]byte  // Our Curve25519 identity private key
	SelfSignedPreKeyPrivate  [32]byte  // Our signed pre-key (Curve25519) private
	SelfOneTimePreKeyPrivate *[32]byte // Our one-time pre-key (nullable)
	PeerIdentityPublic       [32]byte  // Peer's Curve25519 identity public key
	PeerEphemeralPublic      [32]byte  // Peer's ephemeral Curve25519 public key
}

// DeriveX25519FromEd25519Seed converts an Ed25519 private key seed to a Curve25519 private key
// using the XEdDSA method: SHA-512(seed)[0:32], clamped for Curve25519.
// The resulting private key can be used for DH operations and identity in X3DH.
func DeriveX25519FromEd25519Seed(ed25519Seed [32]byte) [32]byte {
	// SHA-512 of the Ed25519 seed
	h := sha512.New()
	h.Write(ed25519Seed[:])
	hash := h.Sum(nil)

	var curve25519Private [32]byte
	copy(curve25519Private[:], hash[:32])

	// Per RFC 7748, clamp the scalar for Curve25519
	curve25519Private[0] &= 248
	curve25519Private[31] &= 127
	curve25519Private[31] |= 64

	// Zeroize hash
	ZeroBytes(hash[:])

	return curve25519Private
}

// x3dhDH performs a single X25519 Diffie-Hellman computation between:
// - our private key
// - their public key
// The private key is zeroized after use. Their public key is not modified.
func x3dhDH(ourPrivate, theirPublic [32]byte) ([32]byte, error) {
	defer ZeroBytes(ourPrivate[:])
	result, err := curve25519.X25519(ourPrivate[:], theirPublic[:])
	if err != nil {
		return [32]byte{}, fmt.Errorf("x3dh: DH failed: %w", err)
	}
	var dh [32]byte
	copy(dh[:], result)
	return dh, nil
}

// X3DHInitiate computes the X3DH initial agreement from the initiator's perspective.
// This implements the 4-DH (or 3-DH if no one-time pre-key is available) agreement
// as specified in the Signal Protocol.
//
// Returns:
// - sk: the derived session key (32 bytes, ready to seed the Double Ratchet)
// - dh1Pub: the public key of the peer's signed pre-key (for wire format)
// - dh4ID: the ID of the peer's one-time pre-key used (0 if 3-DH fallback)
// - error: on validation or DH failure
func X3DHInitiate(params X3DHInitiatorParams) (sk [32]byte, dh1Pub [32]byte, dh4ID uint32, err error) {
	// Validate inputs
	if params.SelfIdentityPrivate == [32]byte{} {
		return [32]byte{}, [32]byte{}, 0, errors.New("x3dh: self identity private key is empty")
	}
	if params.SelfEphemeralPrivate == [32]byte{} {
		return [32]byte{}, [32]byte{}, 0, errors.New("x3dh: self ephemeral private key is empty")
	}
	if params.PeerIdentityPublic == [32]byte{} {
		return [32]byte{}, [32]byte{}, 0, errors.New("x3dh: peer identity public key is empty")
	}
	if params.PeerSignedPreKeyPublic == [32]byte{} {
		return [32]byte{}, [32]byte{}, 0, errors.New("x3dh: peer signed pre-key is empty")
	}

	// DH1 = DH(IK_A, SPK_B)
	dh1, err := x3dhDH(params.SelfIdentityPrivate, params.PeerSignedPreKeyPublic)
	if err != nil {
		return [32]byte{}, [32]byte{}, 0, fmt.Errorf("x3dh: DH1 failed: %w", err)
	}
	defer ZeroBytes(dh1[:])
	dh1Pub = params.PeerSignedPreKeyPublic

	// DH2 = DH(EK_A, IK_B)
	dh2, err := x3dhDH(params.SelfEphemeralPrivate, params.PeerIdentityPublic)
	if err != nil {
		return [32]byte{}, [32]byte{}, 0, fmt.Errorf("x3dh: DH2 failed: %w", err)
	}
	defer ZeroBytes(dh2[:])

	// DH3 = DH(EK_A, SPK_B)
	dh3, err := x3dhDH(params.SelfEphemeralPrivate, params.PeerSignedPreKeyPublic)
	if err != nil {
		return [32]byte{}, [32]byte{}, 0, fmt.Errorf("x3dh: DH3 failed: %w", err)
	}
	defer ZeroBytes(dh3[:])

	// DH4 = DH(EK_A, OPK_B) if available, otherwise 3-DH fallback
	var dh4 [32]byte
	if params.PeerOneTimePreKeyPublic != nil && *params.PeerOneTimePreKeyPublic != [32]byte{} {
		dh4, err = x3dhDH(params.SelfEphemeralPrivate, *params.PeerOneTimePreKeyPublic)
		if err != nil {
			return [32]byte{}, [32]byte{}, 0, fmt.Errorf("x3dh: DH4 failed: %w", err)
		}
		defer ZeroBytes(dh4[:])
		// Use the provided OPK ID (may be 0 if not specified, but OPK was provided)
		dh4ID = params.PeerOneTimePreKeyID
		if dh4ID == 0 {
			// OPK was provided but ID was not explicitly set; use default marker ID = 1
			// (In production, callers should always provide a proper OPK ID for accounting)
			dh4ID = 1
		}
	}
	defer ZeroBytes(dh4[:])

	// Derive SK = HKDF-SHA256(F ‖ DH1 ‖ DH2 ‖ DH3 [‖ DH4])
	ikm := make([]byte, 0, 32+32+32+32+32)
	ikm = append(ikm, X3DHDomainSeparator[:]...)
	ikm = append(ikm, dh1[:]...)
	ikm = append(ikm, dh2[:]...)
	ikm = append(ikm, dh3[:]...)
	if dh4ID > 0 {
		ikm = append(ikm, dh4[:]...)
	}
	defer ZeroBytes(ikm)

	// Use HKDF-SHA256 to derive SK
	hkdfReader := hkdf.New(sha256.New, ikm, nil, []byte("TOX_X3DH_SHARED_SECRET_V1"))
	sk32 := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, sk32); err != nil {
		return [32]byte{}, [32]byte{}, 0, fmt.Errorf("x3dh: HKDF failed: %w", err)
	}
	copy(sk[:], sk32)
	ZeroBytes(sk32)

	return sk, dh1Pub, dh4ID, nil
}

// X3DHRespond computes the X3DH initial agreement from the responder's perspective.
// This derives the same session key as the initiator's X3DHInitiate, enabling
// the responder to decrypt the initiator's first message.
//
// Returns:
// - sk: the derived session key (32 bytes)
// - error: on validation or DH failure
func X3DHRespond(params X3DHResponderParams) (sk [32]byte, err error) {
	// Validate inputs
	if params.SelfIdentityPrivate == [32]byte{} {
		return [32]byte{}, errors.New("x3dh: self identity private key is empty")
	}
	if params.SelfSignedPreKeyPrivate == [32]byte{} {
		return [32]byte{}, errors.New("x3dh: self signed pre-key private is empty")
	}
	if params.PeerIdentityPublic == [32]byte{} {
		return [32]byte{}, errors.New("x3dh: peer identity public key is empty")
	}
	if params.PeerEphemeralPublic == [32]byte{} {
		return [32]byte{}, errors.New("x3dh: peer ephemeral public key is empty")
	}

	// DH1 = DH(SPK_B, IK_A)
	dh1, err := x3dhDH(params.SelfSignedPreKeyPrivate, params.PeerIdentityPublic)
	if err != nil {
		return [32]byte{}, fmt.Errorf("x3dh: DH1 failed: %w", err)
	}
	defer ZeroBytes(dh1[:])

	// DH2 = DH(IK_B, EK_A)
	dh2, err := x3dhDH(params.SelfIdentityPrivate, params.PeerEphemeralPublic)
	if err != nil {
		return [32]byte{}, fmt.Errorf("x3dh: DH2 failed: %w", err)
	}
	defer ZeroBytes(dh2[:])

	// DH3 = DH(SPK_B, EK_A)
	dh3, err := x3dhDH(params.SelfSignedPreKeyPrivate, params.PeerEphemeralPublic)
	if err != nil {
		return [32]byte{}, fmt.Errorf("x3dh: DH3 failed: %w", err)
	}
	defer ZeroBytes(dh3[:])

	// DH4 = DH(OPK_B, EK_A) if we have a one-time pre-key
	var dh4 [32]byte
	if params.SelfOneTimePreKeyPrivate != nil && *params.SelfOneTimePreKeyPrivate != [32]byte{} {
		dh4, err = x3dhDH(*params.SelfOneTimePreKeyPrivate, params.PeerEphemeralPublic)
		if err != nil {
			return [32]byte{}, fmt.Errorf("x3dh: DH4 failed: %w", err)
		}
		defer ZeroBytes(dh4[:])
	}
	defer ZeroBytes(dh4[:])

	// Derive SK = HKDF-SHA256(F ‖ DH1 ‖ DH2 ‖ DH3 [‖ DH4])
	ikm := make([]byte, 0, 32+32+32+32+32)
	ikm = append(ikm, X3DHDomainSeparator[:]...)
	ikm = append(ikm, dh1[:]...)
	ikm = append(ikm, dh2[:]...)
	ikm = append(ikm, dh3[:]...)
	if params.SelfOneTimePreKeyPrivate != nil && *params.SelfOneTimePreKeyPrivate != [32]byte{} {
		ikm = append(ikm, dh4[:]...)
	}
	defer ZeroBytes(ikm)

	// Use HKDF-SHA256 to derive SK (same info string as initiator)
	hkdfReader := hkdf.New(sha256.New, ikm, nil, []byte("TOX_X3DH_SHARED_SECRET_V1"))
	sk32 := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, sk32); err != nil {
		return [32]byte{}, fmt.Errorf("x3dh: HKDF failed: %w", err)
	}
	copy(sk[:], sk32)
	ZeroBytes(sk32)

	return sk, nil
}
