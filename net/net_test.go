package net

import (
	"testing"

	"github.com/opd-ai/toxforge"
	"github.com/opd-ai/toxforge/crypto"
)

func TestToxAddr(t *testing.T) {
	// Create a valid ToxID programmatically
	publicKey := [32]byte{
		0x76, 0x51, 0x84, 0x06, 0xF6, 0xA9, 0xF2, 0x21,
		0x7E, 0x8D, 0xC4, 0x87, 0xCC, 0x78, 0x3C, 0x25,
		0xCC, 0x16, 0xA1, 0x5E, 0xB3, 0x6F, 0xF3, 0x2E,
		0x33, 0x53, 0x64, 0xEC, 0x37, 0x16, 0x6A, 0x87,
	}
	nospam := [4]byte{0x12, 0xA2, 0x0C, 0x01}

	// Create ToxID and get its string representation
	toxID := crypto.NewToxID(publicKey, nospam)
	validToxIDString := toxID.String()

	addr, err := NewToxAddr(validToxIDString)
	if err != nil {
		t.Fatalf("Failed to create ToxAddr: %v", err)
	}

	if addr.Network() != "tox" {
		t.Errorf("Expected network 'tox', got '%s'", addr.Network())
	}

	if addr.String() != validToxIDString {
		t.Errorf("Expected address '%s', got '%s'", validToxIDString, addr.String())
	}

	// Test invalid ToxID
	_, err = NewToxAddr("invalid")
	if err == nil {
		t.Error("Expected error for invalid ToxID")
	}
}

func TestToxAddrEqual(t *testing.T) {
	// Create a valid ToxID programmatically
	publicKey := [32]byte{
		0x76, 0x51, 0x84, 0x06, 0xF6, 0xA9, 0xF2, 0x21,
		0x7E, 0x8D, 0xC4, 0x87, 0xCC, 0x78, 0x3C, 0x25,
		0xCC, 0x16, 0xA1, 0x5E, 0xB3, 0x6F, 0xF3, 0x2E,
		0x33, 0x53, 0x64, 0xEC, 0x37, 0x16, 0x6A, 0x87,
	}
	nospam := [4]byte{0x12, 0xA2, 0x0C, 0x01}

	toxID := crypto.NewToxID(publicKey, nospam)
	validToxIDString := toxID.String()

	addr1, _ := NewToxAddr(validToxIDString)
	addr2, _ := NewToxAddr(validToxIDString)

	if !addr1.Equal(addr2) {
		t.Error("Expected equal addresses")
	}

	// Test different addresses
	publicKey1 := [32]byte{1, 2, 3}
	publicKey2 := [32]byte{4, 5, 6}
	nospam0 := [4]byte{0, 0, 0, 0}

	addr3 := NewToxAddrFromPublicKey(publicKey1, nospam0)
	addr4 := NewToxAddrFromPublicKey(publicKey2, nospam0)

	if addr3.Equal(addr4) {
		t.Error("Expected different addresses")
	}
}

func TestIsToxAddr(t *testing.T) {
	// Create a valid ToxID programmatically
	publicKey := [32]byte{
		0x76, 0x51, 0x84, 0x06, 0xF6, 0xA9, 0xF2, 0x21,
		0x7E, 0x8D, 0xC4, 0x87, 0xCC, 0x78, 0x3C, 0x25,
		0xCC, 0x16, 0xA1, 0x5E, 0xB3, 0x6F, 0xF3, 0x2E,
		0x33, 0x53, 0x64, 0xEC, 0x37, 0x16, 0x6A, 0x87,
	}
	nospam := [4]byte{0x12, 0xA2, 0x0C, 0x01}

	toxID := crypto.NewToxID(publicKey, nospam)
	validToxIDString := toxID.String()

	if !IsToxAddr(validToxIDString) {
		t.Error("Expected valid ToxID to be recognized")
	}

	// Invalid cases
	invalidCases := []string{
		"invalid",
		"76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37166A8712A20C018A5FA6B",     // too short
		"76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37166A8712A20C018A5FA6B0123", // too long
		"GGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG",        // invalid hex
	}

	for _, invalid := range invalidCases {
		if IsToxAddr(invalid) {
			t.Errorf("Expected invalid ToxID '%s' to be rejected", invalid)
		}
	}
}

func TestToxListener(t *testing.T) {
	// Create a test Tox instance
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create listener
	listener, err := Listen(tox)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Check listener address
	addr := listener.Addr()
	if addr.Network() != "tox" {
		t.Errorf("Expected network 'tox', got '%s'", addr.Network())
	}

	toxAddr, ok := addr.(*ToxAddr)
	if !ok {
		t.Fatal("Expected ToxAddr")
	}

	expectedAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	if !toxAddr.Equal(expectedAddr) {
		t.Error("Listener address doesn't match expected address")
	}
}

func TestToxAddrFromPublicKey(t *testing.T) {
	publicKey := [32]byte{1, 2, 3, 4, 5}
	nospam := [4]byte{10, 20, 30, 40}

	addr := NewToxAddrFromPublicKey(publicKey, nospam)

	if addr.PublicKey() != publicKey {
		t.Error("Public key doesn't match")
	}

	if addr.Nospam() != nospam {
		t.Error("Nospam doesn't match")
	}

	if addr.Network() != "tox" {
		t.Error("Network should be 'tox'")
	}
}
