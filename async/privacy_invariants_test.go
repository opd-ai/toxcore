package async

import (
	"bytes"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestPaddingBucketBoundaries verifies exact bucket assignment at every boundary.
// These are regression guards: if the switch conditions in PadMessageToStandardSize
// change, one of these tests will fail.
func TestPaddingBucketBoundaries(t *testing.T) {
	cases := []struct {
		name        string
		msgLen      int
		wantPadded  int
	}{
		// Zero-length message → small bucket
		{"zero_length", 0, MessageSizeSmall},
		// One byte below small boundary → small bucket
		{"small_boundary_minus1", MessageSizeSmall - LengthPrefixSize - 1, MessageSizeSmall},
		// Exactly at small boundary → small bucket
		{"small_boundary_exact", MessageSizeSmall - LengthPrefixSize, MessageSizeSmall},
		// One byte above small boundary → medium bucket
		{"small_boundary_plus1", MessageSizeSmall - LengthPrefixSize + 1, MessageSizeMedium},
		// One byte below medium boundary → medium bucket
		{"medium_boundary_minus1", MessageSizeMedium - LengthPrefixSize - 1, MessageSizeMedium},
		// Exactly at medium boundary → medium bucket
		{"medium_boundary_exact", MessageSizeMedium - LengthPrefixSize, MessageSizeMedium},
		// One byte above medium boundary → large bucket
		{"medium_boundary_plus1", MessageSizeMedium - LengthPrefixSize + 1, MessageSizeLarge},
		// One byte below large boundary → large bucket
		{"large_boundary_minus1", MessageSizeLarge - LengthPrefixSize - 1, MessageSizeLarge},
		// Exactly at large boundary → large bucket
		{"large_boundary_exact", MessageSizeLarge - LengthPrefixSize, MessageSizeLarge},
		// One byte above large boundary → max bucket
		{"large_boundary_plus1", MessageSizeLarge - LengthPrefixSize + 1, MessageSizeMax},
		// Exactly at max boundary → max bucket
		{"max_boundary_exact", MessageSizeMax - LengthPrefixSize, MessageSizeMax},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := bytes.Repeat([]byte{0xAB}, tc.msgLen)
			padded, err := PadMessageToStandardSize(msg)
			if err != nil {
				t.Fatalf("PadMessageToStandardSize(%d bytes): %v", tc.msgLen, err)
			}
			if len(padded) != tc.wantPadded {
				t.Errorf("msg len %d: got padded size %d, want %d", tc.msgLen, len(padded), tc.wantPadded)
			}
			// Round-trip invariant
			got, err := UnpadMessage(padded)
			if err != nil {
				t.Fatalf("UnpadMessage: %v", err)
			}
			if !bytes.Equal(got, msg) {
				t.Error("round-trip failed: UnpadMessage(PadMessageToStandardSize(msg)) != msg")
			}
		})
	}
}

// TestPaddingBucketInvariant verifies that all messages in the same bucket produce
// the same padded length, regardless of their position within that bucket.
func TestPaddingBucketInvariant(t *testing.T) {
	type bucket struct {
		name       string
		min, max   int
		wantPadded int
	}
	buckets := []bucket{
		{"small", 0, MessageSizeSmall - LengthPrefixSize, MessageSizeSmall},
		{"medium", MessageSizeSmall - LengthPrefixSize + 1, MessageSizeMedium - LengthPrefixSize, MessageSizeMedium},
		{"large", MessageSizeMedium - LengthPrefixSize + 1, MessageSizeLarge - LengthPrefixSize, MessageSizeLarge},
		{"max", MessageSizeLarge - LengthPrefixSize + 1, MessageSizeMax - LengthPrefixSize, MessageSizeMax},
	}

	// Sample three sizes from each bucket (min, mid, max).
	for _, b := range buckets {
		mid := (b.min + b.max) / 2
		for _, msgLen := range []int{b.min, mid, b.max} {
			msg := bytes.Repeat([]byte{0xCD}, msgLen)
			padded, err := PadMessageToStandardSize(msg)
			if err != nil {
				t.Fatalf("bucket %s, msg len %d: %v", b.name, msgLen, err)
			}
			if len(padded) != b.wantPadded {
				t.Errorf("bucket %s, msg len %d: got padded %d, want %d",
					b.name, msgLen, len(padded), b.wantPadded)
			}
		}
	}
}

// TestPseudonymRotationInvariant verifies the key privacy properties of epoch-based
// pseudonym rotation:
//  1. Determinism: same key + same epoch → identical pseudonym.
//  2. Epoch isolation: same key + different epochs → distinct pseudonyms.
//  3. Key isolation: different keys + same epoch → distinct pseudonyms.
//  4. Cross-epoch unlinkability: across N epochs, all pseudonyms are distinct.
func TestPseudonymRotationInvariant(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	em := NewEpochManager()
	om := NewObfuscationManager(keyPair, em)

	recipientPK := [32]byte{0x11, 0x22, 0x33}
	baseEpoch := uint64(1000)
	numEpochs := 20

	// Collect pseudonyms for consecutive epochs.
	seen := make(map[[32]byte]uint64, numEpochs)
	for i := 0; i < numEpochs; i++ {
		epoch := baseEpoch + uint64(i)

		ps, err := om.GenerateRecipientPseudonym(recipientPK, epoch)
		if err != nil {
			t.Fatalf("epoch %d: GenerateRecipientPseudonym: %v", epoch, err)
		}

		// Determinism check: calling again with identical inputs must return same value.
		ps2, err := om.GenerateRecipientPseudonym(recipientPK, epoch)
		if err != nil {
			t.Fatalf("epoch %d (second call): %v", epoch, err)
		}
		if ps != ps2 {
			t.Errorf("epoch %d: pseudonym not deterministic", epoch)
		}

		// Cross-epoch uniqueness (unlinkability).
		if prev, conflict := seen[ps]; conflict {
			t.Errorf("epoch %d produced same pseudonym as epoch %d (linkability!)", epoch, prev)
		}
		seen[ps] = epoch
	}

	// Key isolation: different PK at same epoch must differ.
	otherPK := [32]byte{0x44, 0x55, 0x66}
	psA, err := om.GenerateRecipientPseudonym(recipientPK, baseEpoch)
	if err != nil {
		t.Fatalf("key isolation: %v", err)
	}
	psB, err := om.GenerateRecipientPseudonym(otherPK, baseEpoch)
	if err != nil {
		t.Fatalf("key isolation other: %v", err)
	}
	if psA == psB {
		t.Error("different recipient keys produced the same pseudonym for the same epoch")
	}
}

// TestSenderPseudonymUnlinkability verifies that sender pseudonyms are single-use:
// distinct nonces always produce distinct pseudonyms so messages cannot be linked.
func TestSenderPseudonymUnlinkability(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	em := NewEpochManager()
	om := NewObfuscationManager(keyPair, em)

	senderSK := [32]byte{0xAA, 0xBB}
	recipientPK := [32]byte{0xCC, 0xDD}

	const n = 20
	seen := make(map[[32]byte]int, n)
	for i := 0; i < n; i++ {
		var nonce [24]byte
		nonce[0] = byte(i)
		nonce[1] = byte(i >> 8)
		ps, err := om.GenerateSenderPseudonym(senderSK, recipientPK, nonce)
		if err != nil {
			t.Fatalf("GenerateSenderPseudonym nonce %d: %v", i, err)
		}
		if prev, conflict := seen[ps]; conflict {
			t.Errorf("nonce %d produced same sender pseudonym as nonce %d", i, prev)
		}
		seen[ps] = i
	}
}
