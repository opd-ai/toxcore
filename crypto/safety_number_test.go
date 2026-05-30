package crypto

import (
	"strings"
	"testing"
)

// TestSafetyNumberTestVector verifies a known-answer test vector for SafetyNumber.
// The expected output was generated from the reference implementation and serves
// as a regression guard for the derivation algorithm.
func TestSafetyNumberTestVector(t *testing.T) {
	t.Parallel()

	var alice, bob [KeySize]byte
	for i := range alice {
		alice[i] = byte(i + 1)
	}
	for i := range bob {
		bob[i] = byte(i + 0x41)
	}

	const want = "72229 96823 09216 95981 62541 01061 08798 45263 49000 38973 07014 51931"

	got := SafetyNumber(alice, bob)
	if got != want {
		t.Errorf("SafetyNumber: got %q, want %q", got, want)
	}
}

// TestSafetyNumberCommutative verifies that SafetyNumber(a,b) == SafetyNumber(b,a).
func TestSafetyNumberCommutative(t *testing.T) {
	t.Parallel()

	var a, b [KeySize]byte
	for i := range a {
		a[i] = byte(i * 7)
		b[i] = byte(255 - i)
	}

	if SafetyNumber(a, b) != SafetyNumber(b, a) {
		t.Error("SafetyNumber is not commutative")
	}
}

// TestSafetyNumberFormat checks that the output contains exactly 12 groups
// of 5 decimal digits separated by spaces.
func TestSafetyNumberFormat(t *testing.T) {
	t.Parallel()

	var myPK, peerPK [KeySize]byte
	for i := range myPK {
		myPK[i] = byte(i + 3)
		peerPK[i] = byte(i + 200)
	}

	sn := SafetyNumber(myPK, peerPK)
	groups := strings.Split(sn, " ")

	if len(groups) != 12 {
		t.Fatalf("expected 12 groups, got %d: %s", len(groups), sn)
	}

	for _, g := range groups {
		if len(g) != 5 {
			t.Errorf("expected 5-digit group, got %q in %s", g, sn)
		}
		for _, ch := range g {
			if ch < '0' || ch > '9' {
				t.Errorf("non-digit character %q in group %q", ch, g)
			}
		}
	}
}

// TestSafetyNumberDistinctKeys checks that distinct key pairs produce distinct fingerprints.
func TestSafetyNumberDistinctKeys(t *testing.T) {
	t.Parallel()

	var pk1, pk2, pk3, pk4 [KeySize]byte
	pk1[0] = 1
	pk2[0] = 2
	pk3[0] = 3
	pk4[0] = 4

	sn12 := SafetyNumber(pk1, pk2)
	sn13 := SafetyNumber(pk1, pk3)
	sn34 := SafetyNumber(pk3, pk4)

	if sn12 == sn13 {
		t.Error("different key pairs produced identical safety numbers")
	}
	if sn12 == sn34 {
		t.Error("different key pairs produced identical safety numbers")
	}
}
