package toxcore

import (
	"testing"
)

func TestSaveLoadAPI(t *testing.T) {
	// Create a Tox instance with some state
	options := NewOptions()
	options.UDPEnabled = false // Disable UDP for test
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Add friends and set some state
	friend1PublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friend2PublicKey := [32]byte{32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17,
		16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}

	friendID1, err := tox1.AddFriendByPublicKey(friend1PublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend 1: %v", err)
	}

	friendID2, err := tox1.AddFriendByPublicKey(friend2PublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend 2: %v", err)
	}

	// Set friend details
	tox1.friendsMutex.Lock()
	tox1.friends[friendID1].Name = "Save Test Friend 1"
	tox1.friends[friendID1].StatusMessage = "Status message 1"
	tox1.friends[friendID2].Name = "Save Test Friend 2"
	tox1.friends[friendID2].StatusMessage = "Status message 2"
	tox1.friendsMutex.Unlock()

	// Test Save method
	saveData, err := tox1.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	if len(saveData) == 0 {
		t.Fatal("Save() returned empty data")
	}

	// Create a new Tox instance
	options2 := NewOptions()
	options2.UDPEnabled = false
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Test Load method
	err = tox2.Load(saveData)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify the loaded state
	if tox2.keyPair.Public != tox1.keyPair.Public {
		t.Error("Public key not loaded correctly")
	}

	if tox2.keyPair.Private != tox1.keyPair.Private {
		t.Error("Private key not loaded correctly")
	}

	if tox2.nospam != tox1.nospam {
		t.Error("Nospam not loaded correctly")
	}

	// Check friends were loaded
	tox2.friendsMutex.RLock()
	if len(tox2.friends) != 2 {
		t.Errorf("Expected 2 friends loaded, got %d", len(tox2.friends))
	}

	// Verify friend details
	friend1Loaded, exists1 := tox2.friends[friendID1]
	friend2Loaded, exists2 := tox2.friends[friendID2]
	tox2.friendsMutex.RUnlock()

	if !exists1 {
		t.Error("Friend 1 not loaded")
	} else {
		if friend1Loaded.Name != "Save Test Friend 1" {
			t.Errorf("Friend 1 name not loaded correctly: expected 'Save Test Friend 1', got '%s'", friend1Loaded.Name)
		}
		if friend1Loaded.StatusMessage != "Status message 1" {
			t.Errorf("Friend 1 status not loaded correctly: expected 'Status message 1', got '%s'", friend1Loaded.StatusMessage)
		}
	}

	if !exists2 {
		t.Error("Friend 2 not loaded")
	} else {
		if friend2Loaded.Name != "Save Test Friend 2" {
			t.Errorf("Friend 2 name not loaded correctly: expected 'Save Test Friend 2', got '%s'", friend2Loaded.Name)
		}
		if friend2Loaded.StatusMessage != "Status message 2" {
			t.Errorf("Friend 2 status not loaded correctly: expected 'Status message 2', got '%s'", friend2Loaded.StatusMessage)
		}
	}

	t.Log("Save/Load API test completed successfully")
}

func TestSaveLoadEdgeCases(t *testing.T) {
	// Test Load with empty data
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test loading empty data
	err = tox.Load([]byte{})
	if err == nil {
		t.Error("Load() should fail with empty data")
	}

	// Test loading nil data
	err = tox.Load(nil)
	if err == nil {
		t.Error("Load() should fail with nil data")
	}

	// Test Save on fresh instance
	saveData, err := tox.Save()
	if err != nil {
		t.Errorf("Save() should not fail on fresh instance: %v", err)
	}

	if len(saveData) == 0 {
		t.Error("Save() should return data even for fresh instance")
	}

	t.Log("Save/Load edge cases test completed successfully")
}

func TestSaveLoadRoundTrip(t *testing.T) {
	// Test multiple save/load cycles to ensure data integrity
	options := NewOptions()
	options.UDPEnabled = false
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Add a friend
	friendPublicKey := [32]byte{42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42,
		42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42}
	friendID, err := tox1.AddFriendByPublicKey(friendPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Original values
	originalPublicKey := tox1.keyPair.Public
	originalPrivateKey := tox1.keyPair.Private
	originalNospam := tox1.nospam

	// First save/load cycle
	saveData1, err := tox1.Save()
	if err != nil {
		t.Fatalf("First Save() failed: %v", err)
	}

	options2 := NewOptions()
	options2.UDPEnabled = false
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	err = tox2.Load(saveData1)
	if err != nil {
		t.Fatalf("First Load() failed: %v", err)
	}

	// Second save/load cycle
	saveData2, err := tox2.Save()
	if err != nil {
		t.Fatalf("Second Save() failed: %v", err)
	}

	options3 := NewOptions()
	options3.UDPEnabled = false
	tox3, err := New(options3)
	if err != nil {
		t.Fatalf("Failed to create third Tox instance: %v", err)
	}
	defer tox3.Kill()

	err = tox3.Load(saveData2)
	if err != nil {
		t.Fatalf("Second Load() failed: %v", err)
	}

	// Verify data integrity after multiple cycles
	if tox3.keyPair.Public != originalPublicKey {
		t.Error("Public key corrupted after round trip")
	}

	if tox3.keyPair.Private != originalPrivateKey {
		t.Error("Private key corrupted after round trip")
	}

	if tox3.nospam != originalNospam {
		t.Error("Nospam corrupted after round trip")
	}

	// Verify friend still exists
	tox3.friendsMutex.RLock()
	_, exists := tox3.friends[friendID]
	tox3.friendsMutex.RUnlock()

	if !exists {
		t.Error("Friend not preserved after round trip")
	}

	t.Log("Save/Load round trip test completed successfully")
}
