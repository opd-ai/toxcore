package toxcore

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

func TestNospamFunctionality(t *testing.T) {
	// Create a new Tox instance
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	t.Run("SelfGetAddress returns valid ToxID", func(t *testing.T) {
		address := tox.SelfGetAddress()

		// ToxID should be 76 hex characters (38 bytes)
		if len(address) != 76 {
			t.Errorf("Expected ToxID length 76, got %d", len(address))
		}

		// Should be valid hex string
		_, err := crypto.ToxIDFromString(address)
		if err != nil {
			t.Errorf("SelfGetAddress returned invalid ToxID: %v", err)
		}
	})

	t.Run("SelfGetNospam returns instance nospam", func(t *testing.T) {
		nospam := tox.SelfGetNospam()

		// The nospam should not be all zeros (would indicate the bug)
		if nospam == [4]byte{} {
			t.Error("Nospam is all zeros - generateNospam() may be broken")
		}
	})

	t.Run("SelfSetNospam changes ToxID", func(t *testing.T) {
		// Get original address
		originalAddress := tox.SelfGetAddress()

		// Set new nospam
		newNospam := [4]byte{0x12, 0x34, 0x56, 0x78}
		tox.SelfSetNospam(newNospam)

		// Get new address
		newAddress := tox.SelfGetAddress()

		// Addresses should be different
		if originalAddress == newAddress {
			t.Error("ToxID should change when nospam changes")
		}

		// Verify the nospam was actually set
		retrievedNospam := tox.SelfGetNospam()
		if retrievedNospam != newNospam {
			t.Errorf("Expected nospam %v, got %v", newNospam, retrievedNospam)
		}

		// Parse the ToxID and verify nospam is embedded correctly
		toxID, err := crypto.ToxIDFromString(newAddress)
		if err != nil {
			t.Fatalf("Failed to parse ToxID: %v", err)
		}

		if toxID.Nospam != newNospam {
			t.Errorf("ToxID contains wrong nospam: expected %v, got %v", newNospam, toxID.Nospam)
		}
	})

	t.Run("ToxID contains correct public key", func(t *testing.T) {
		address := tox.SelfGetAddress()
		publicKey := tox.SelfGetPublicKey()

		toxID, err := crypto.ToxIDFromString(address)
		if err != nil {
			t.Fatalf("Failed to parse ToxID: %v", err)
		}

		if toxID.PublicKey != publicKey {
			t.Error("ToxID contains wrong public key")
		}
	})
}

func TestNospamPersistence(t *testing.T) {
	// Create first instance with specific nospam
	options := NewOptions()
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Set custom nospam
	customNospam := [4]byte{0xAA, 0xBB, 0xCC, 0xDD}
	tox1.SelfSetNospam(customNospam)

	// Get the ToxID with custom nospam
	originalAddress := tox1.SelfGetAddress()

	t.Run("Savedata preserves nospam", func(t *testing.T) {
		// Save data
		savedata := tox1.GetSavedata()
		if len(savedata) == 0 {
			t.Fatal("GetSavedata returned empty data")
		}

		// Create new instance from savedata
		tox2, err := NewFromSavedata(nil, savedata)
		if err != nil {
			t.Fatalf("Failed to restore from savedata: %v", err)
		}
		defer tox2.Kill()

		// Verify nospam was restored
		restoredNospam := tox2.SelfGetNospam()
		if restoredNospam != customNospam {
			t.Errorf("Nospam not preserved: expected %v, got %v", customNospam, restoredNospam)
		}

		// Verify ToxID is the same
		restoredAddress := tox2.SelfGetAddress()
		if restoredAddress != originalAddress {
			t.Errorf("ToxID not preserved after restoration")
		}
	})

	t.Run("Load preserves nospam", func(t *testing.T) {
		// Create third instance
		tox3, err := New(nil)
		if err != nil {
			t.Fatalf("Failed to create third Tox instance: %v", err)
		}
		defer tox3.Kill()

		// Get original address (should be different)
		beforeLoadAddress := tox3.SelfGetAddress()
		if beforeLoadAddress == originalAddress {
			t.Error("New instance has same ToxID as saved one (unexpected)")
		}

		// Load savedata
		savedata := tox1.GetSavedata()
		err = tox3.Load(savedata)
		if err != nil {
			t.Fatalf("Failed to load savedata: %v", err)
		}

		// Verify nospam was loaded
		loadedNospam := tox3.SelfGetNospam()
		if loadedNospam != customNospam {
			t.Errorf("Nospam not loaded: expected %v, got %v", customNospam, loadedNospam)
		}

		// Verify ToxID matches
		afterLoadAddress := tox3.SelfGetAddress()
		if afterLoadAddress != originalAddress {
			t.Errorf("ToxID not restored after Load")
		}
	})
}

func TestGenerateNospam(t *testing.T) {
	t.Run("generateNospam returns random values", func(t *testing.T) {
		// Generate multiple nospam values
		nospams := make([][4]byte, 10)
		for i := 0; i < 10; i++ {
			nospam, err := generateNospam()
			if err != nil {
				t.Fatalf("generateNospam() failed: %v", err)
			}
			nospams[i] = nospam
		}

		// Check they're not all the same (should be very unlikely with random generation)
		allSame := true
		first := nospams[0]
		for i := 1; i < len(nospams); i++ {
			if nospams[i] != first {
				allSame = false
				break
			}
		}

		if allSame {
			t.Error("generateNospam() appears to return constant values - randomness broken")
		}

		// Check none are all zeros
		for i, nospam := range nospams {
			if nospam == [4]byte{} {
				t.Errorf("generateNospam() returned all zeros at index %d", i)
			}
		}
	})
}

func TestBackwardCompatibility(t *testing.T) {
	t.Run("Load handles savedata without nospam", func(t *testing.T) {
		// Create instance
		tox, err := New(nil)
		if err != nil {
			t.Fatalf("Failed to create Tox instance: %v", err)
		}
		defer tox.Kill()

		// Simulate old savedata format by creating savedata manually without nospam
		oldFormatData := toxSaveData{
			KeyPair:       tox.keyPair,
			Friends:       make(map[uint32]*Friend),
			Options:       tox.options,
			SelfName:      "Test Name",
			SelfStatusMsg: "Test Status",
			// Nospam intentionally omitted (zero value)
		}

		oldSavedata := oldFormatData.marshal()

		// Load old format data
		err = tox.Load(oldSavedata)
		if err != nil {
			t.Fatalf("Failed to load old format savedata: %v", err)
		}

		// Should generate new nospam (not zeros)
		nospam := tox.SelfGetNospam()
		if nospam == [4]byte{} {
			t.Error("Should generate new nospam for old savedata format, but got zeros")
		}

		// Should have valid ToxID
		address := tox.SelfGetAddress()
		_, err = crypto.ToxIDFromString(address)
		if err != nil {
			t.Errorf("Invalid ToxID after loading old savedata: %v", err)
		}
	})
}

func TestNospamConcurrency(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	t.Run("Concurrent nospam access is safe", func(t *testing.T) {
		// Test concurrent reads and writes don't race
		done := make(chan bool, 100)

		// Start readers
		for i := 0; i < 50; i++ {
			go func() {
				_ = tox.SelfGetNospam()
				_ = tox.SelfGetAddress()
				done <- true
			}()
		}

		// Start writers
		for i := 0; i < 50; i++ {
			go func(val byte) {
				nospam := [4]byte{val, val, val, val}
				tox.SelfSetNospam(nospam)
				done <- true
			}(byte(i))
		}

		// Wait for all goroutines
		for i := 0; i < 100; i++ {
			<-done
		}

		// Should still have valid state
		nospam := tox.SelfGetNospam()
		address := tox.SelfGetAddress()

		toxID, err := crypto.ToxIDFromString(address)
		if err != nil {
			t.Errorf("ToxID invalid after concurrent access: %v", err)
		}

		if toxID.Nospam != nospam {
			t.Error("ToxID nospam doesn't match stored nospam after concurrent access")
		}
	})
}
