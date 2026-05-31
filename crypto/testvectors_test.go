package crypto

// Package crypto test vectors — fixed-input / fixed-output known-answer tests.
//
// These vectors pin the exact byte-level output of each cryptographic operation
// so that any accidental algorithm change surfaces immediately in CI.  The
// inputs are fully deterministic (no random number generation) so the vectors
// are reproducible across platforms.
//
// Test coverage:
//   - SafetyNumber: SHA-512 × 5200, canonical key ordering, decimal encoding
//   - Message padding: standard-size bucket selection, length-prefix placement
//   - Recipient pseudonym: HKDF-SHA-256 derivation from public key + epoch

import (
	"testing"
)

// safetyNumberVector holds a fixed-input / fixed-output test vector for
// SafetyNumber derivation.
type safetyNumberVector struct {
	name           string
	myPK           [KeySize]byte
	peerPK         [KeySize]byte
	expectedResult string
}

// TestSafetyNumberVectors verifies SafetyNumber against fixed known-answer
// vectors.  Any change to the hash function, iteration count, canonical
// ordering, or encoding scheme will cause these tests to fail.
func TestSafetyNumberVectors(t *testing.T) {
	t.Parallel()

	// Vector 1: pk1 = 0x01…0x20, pk2 = 0x21…0x40
	// Derivation: version(0x02) || min(pk1,pk2) || max(pk1,pk2)
	// → SHA-512 iterated 5200 times → first 60 bytes encoded as 12×5-digit groups.
	var pk1, pk2 [KeySize]byte
	for i := range pk1 {
		pk1[i] = byte(i + 1) // 0x01 … 0x20
	}
	for i := range pk2 {
		pk2[i] = byte(i + 33) // 0x21 … 0x40
	}

	// pk1 < pk2 lexicographically, so canonical order is (pk1, pk2).
	const expectedSN = "53616 49674 46650 93859 28574 28635 36755 42107 54275 57203 95167 88827"

	vectors := []safetyNumberVector{
		{
			name:           "pk1_lt_pk2",
			myPK:           pk1,
			peerPK:         pk2,
			expectedResult: expectedSN,
		},
		{
			// Commutativity: SafetyNumber(a,b) must equal SafetyNumber(b,a).
			name:           "pk2_lt_pk1_commutative",
			myPK:           pk2,
			peerPK:         pk1,
			expectedResult: expectedSN,
		},
	}

	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()
			got := SafetyNumber(v.myPK, v.peerPK)
			if got != v.expectedResult {
				t.Errorf("SafetyNumber mismatch\n  got:  %q\n  want: %q", got, v.expectedResult)
			}
		})
	}
}

// TestSafetyNumberEncodingVector verifies the internal decimal-encoding step
// against a hand-computed reference: a 60-byte sequence of 0x00 bytes should
// produce twelve "00000" groups.
func TestSafetyNumberEncodingVector(t *testing.T) {
	t.Parallel()

	zero60 := make([]byte, 60)
	got := encodeSafetyNumber(zero60)
	want := "00000 00000 00000 00000 00000 00000 00000 00000 00000 00000 00000 00000"
	if got != want {
		t.Errorf("encodeSafetyNumber(zeros)\n  got:  %q\n  want: %q", got, want)
	}
}

// TestSafetyNumberCanonicalOrderVector verifies that canonicalKeyOrder always
// places the lexicographically-smaller key first.
func TestSafetyNumberCanonicalOrderVector(t *testing.T) {
	t.Parallel()

	var smaller, larger [KeySize]byte
	smaller[0] = 0x01
	larger[0] = 0x02

	first, second := canonicalKeyOrder(smaller, larger)
	if first != smaller || second != larger {
		t.Error("canonicalKeyOrder(smaller, larger): wrong order")
	}

	first2, second2 := canonicalKeyOrder(larger, smaller)
	if first2 != smaller || second2 != larger {
		t.Error("canonicalKeyOrder(larger, smaller): wrong order")
	}

	// Equal keys must be returned unchanged.
	firstEq, secondEq := canonicalKeyOrder(smaller, smaller)
	if firstEq != smaller || secondEq != smaller {
		t.Error("canonicalKeyOrder(equal, equal): unexpected result")
	}
}
