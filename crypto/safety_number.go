package crypto

import (
	"crypto/sha512"
	"fmt"
	"strings"
)

// SafetyNumber derives a human-readable, versioned key fingerprint from two
// Curve25519 public keys. The result is formatted as 12 groups of 5 decimal
// digits (60 digits total), suitable for out-of-band comparison to defeat
// MITM attacks.
//
// Derivation: SHA-512 is applied 5200 times over
//
//	version_byte (0x02) || canonical_key1 || canonical_key2
//
// where the keys are placed in canonical (lexicographic) order so that
// SafetyNumber(a, b) == SafetyNumber(b, a). The first 60 bytes of the
// resulting hash are encoded as 12 × 5-digit decimal groups.
//
// ⚠ SECURITY: Users MUST compare safety numbers through an independent
// out-of-band channel (e.g. voice call, in-person meeting) at least once
// per contact. The fingerprint provides MITM detection only when the
// comparison is made through a channel that an attacker cannot intercept
// or manipulate.
//
//export ToxSafetyNumber
func SafetyNumber(myPK, peerPK [KeySize]byte) string {
	const version = byte(0x02)

	// Canonical ordering guarantees commutativity: SafetyNumber(a,b) == SafetyNumber(b,a)
	first, second := canonicalKeyOrder(myPK, peerPK)

	// preimage = version || key1 || key2
	var preimage [1 + 2*KeySize]byte
	preimage[0] = version
	copy(preimage[1:], first[:])
	copy(preimage[1+KeySize:], second[:])

	// Iterate SHA-512 5200 times (matches Signal Protocol's iteration count)
	hash := sha512.Sum512(preimage[:])
	for i := 1; i < 5200; i++ {
		hash = sha512.Sum512(hash[:])
	}

	// Encode the first 60 bytes as 12 groups of 5 decimal digits
	return encodeSafetyNumber(hash[:60])
}

// canonicalKeyOrder returns the two keys in lexicographic order so that
// SafetyNumber is commutative.
func canonicalKeyOrder(a, b [KeySize]byte) ([KeySize]byte, [KeySize]byte) {
	for i := 0; i < KeySize; i++ {
		if a[i] < b[i] {
			return a, b
		}
		if a[i] > b[i] {
			return b, a
		}
	}
	return a, b
}

// encodeSafetyNumber converts 60 bytes to 12 groups of 5 zero-padded decimal
// digits separated by spaces.  Each group encodes 5 bytes as a big-endian
// uint64 reduced modulo 100 000.
func encodeSafetyNumber(b []byte) string {
	groups := make([]string, 12)
	for i := range groups {
		offset := i * 5
		var v uint64
		for j := 0; j < 5; j++ {
			v = v<<8 | uint64(b[offset+j])
		}
		groups[i] = fmt.Sprintf("%05d", v%100000)
	}
	return strings.Join(groups, " ")
}
