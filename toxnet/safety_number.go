package toxnet

import (
	"github.com/opd-ai/toxcore/crypto"
)

// SafetyNumber derives a human-readable, versioned key fingerprint from two
// Curve25519 public keys.  The result is 12 groups of 5 decimal digits
// (60 digits total), suitable for out-of-band comparison to detect
// man-in-the-middle attacks across any transport type supported by toxnet.
//
// Both myPK and peerPK should be the Curve25519 public keys of the local and
// remote peers respectively.  The function is commutative:
//
//	SafetyNumber(a, b) == SafetyNumber(b, a)
//
// ⚠ SECURITY: Both parties MUST compare their safety numbers through an
// independent channel (e.g. voice call, in-person) at least once per contact.
// The fingerprint provides MITM detection only when verified through a channel
// the attacker cannot intercept or manipulate.
func SafetyNumber(myPK, peerPK [32]byte) string {
	return crypto.SafetyNumber(myPK, peerPK)
}

// SafetyNumberFromAddrs is a convenience wrapper that derives a safety number
// from two ToxAddr values, using their embedded public keys.
func SafetyNumberFromAddrs(myAddr, peerAddr *ToxAddr) string {
	return crypto.SafetyNumber(myAddr.PublicKey(), peerAddr.PublicKey())
}
