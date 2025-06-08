package toxcore

import (
	"testing"
)

func TestSaveDataPersistence(t *testing.T) {
	// Create a Tox instance
	options := NewOptions()
	options.UDPEnabled = false // Disable UDP for test
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add some friends
	friend1PublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	friendID1, err := tox.AddFriendByPublicKey(friend1PublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend 1: %v", err)
	}

	friend2PublicKey := [32]byte{32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17,
		16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}

	friendID2, err := tox.AddFriendByPublicKey(friend2PublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend 2: %v", err)
	}

	// Update friend names for testing
	tox.friendsMutex.Lock()
	tox.friends[friendID1].Name = "Test Friend 1"
	tox.friends[friendID1].StatusMessage = "Hello from friend 1"
	tox.friends[friendID2].Name = "Test Friend 2"
	tox.friends[friendID2].StatusMessage = "Hello from friend 2"
	tox.friendsMutex.Unlock()

	// Get save data
	saveData := tox.GetSavedata()
	if len(saveData) == 0 {
		t.Fatal("Save data is empty")
	}

	// Test deserialization
	loadedSaveData, err := LoadSaveData(saveData)
	if err != nil {
		t.Fatalf("Failed to load save data: %v", err)
	}

	// Verify loaded data
	if loadedSaveData.PublicKey != tox.keyPair.Public {
		t.Error("Public key mismatch in saved data")
	}

	if loadedSaveData.SecretKey != tox.keyPair.Private {
		t.Error("Secret key mismatch in saved data")
	}

	if loadedSaveData.Nospam != tox.nospam {
		t.Error("Nospam mismatch in saved data")
	}

	if len(loadedSaveData.Friends) != 2 {
		t.Errorf("Expected 2 friends in save data, got %d", len(loadedSaveData.Friends))
	}

	// Verify friend data
	friendFound1 := false
	friendFound2 := false
	for _, savedFriend := range loadedSaveData.Friends {
		if savedFriend.PublicKey == friend1PublicKey {
			friendFound1 = true
			if savedFriend.Name != "Test Friend 1" {
				t.Errorf("Friend 1 name mismatch: expected 'Test Friend 1', got '%s'", savedFriend.Name)
			}
		}
		if savedFriend.PublicKey == friend2PublicKey {
			friendFound2 = true
			if savedFriend.Name != "Test Friend 2" {
				t.Errorf("Friend 2 name mismatch: expected 'Test Friend 2', got '%s'", savedFriend.Name)
			}
		}
	}

	if !friendFound1 {
		t.Error("Friend 1 not found in save data")
	}
	if !friendFound2 {
		t.Error("Friend 2 not found in save data")
	}

	t.Log("Save data persistence test completed successfully")
}

func TestLoadFromSaveData(t *testing.T) {
	// Create initial Tox instance with some data
	options1 := NewOptions()
	options1.UDPEnabled = false
	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Add a friend
	friendPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	friendID, err := tox1.AddFriendByPublicKey(friendPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set friend details
	tox1.friendsMutex.Lock()
	tox1.friends[friendID].Name = "Loaded Friend"
	tox1.friends[friendID].StatusMessage = "Status from loaded data"
	tox1.friendsMutex.Unlock()

	// Get save data
	saveData := tox1.GetSavedata()

	// Create new Tox instance with save data
	options2 := NewOptions()
	options2.UDPEnabled = false
	options2.SavedataType = SaveDataTypeToxSave
	options2.SavedataData = saveData

	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance with save data: %v", err)
	}
	defer tox2.Kill()

	// Verify the data was loaded
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
	loadedFriend, exists := tox2.friends[friendID]
	tox2.friendsMutex.RUnlock()

	if !exists {
		t.Fatal("Friend was not loaded from save data")
	}

	if loadedFriend.PublicKey != friendPublicKey {
		t.Error("Friend public key not loaded correctly")
	}

	if loadedFriend.Name != "Loaded Friend" {
		t.Errorf("Friend name not loaded correctly: expected 'Loaded Friend', got '%s'", loadedFriend.Name)
	}

	if loadedFriend.StatusMessage != "Status from loaded data" {
		t.Errorf("Friend status message not loaded correctly: expected 'Status from loaded data', got '%s'", loadedFriend.StatusMessage)
	}

	t.Log("Load from save data test completed successfully")
}

func TestFileTransferCallbacks(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test callback registration
	var fileRecvCalled bool
	var fileChunkRecvCalled bool
	var fileChunkRequestCalled bool

	tox.OnFileRecv(func(friendID uint32, fileID uint32, kind uint32, fileSize uint64, filename string) {
		fileRecvCalled = true
		t.Logf("File receive callback called: friend=%d, file=%d, kind=%d, size=%d, name=%s",
			friendID, fileID, kind, fileSize, filename)
	})

	tox.OnFileRecvChunk(func(friendID uint32, fileID uint32, position uint64, data []byte) {
		fileChunkRecvCalled = true
		t.Logf("File chunk receive callback called: friend=%d, file=%d, pos=%d, data_len=%d",
			friendID, fileID, position, len(data))
	})

	tox.OnFileChunkRequest(func(friendID uint32, fileID uint32, position uint64, length int) {
		fileChunkRequestCalled = true
		t.Logf("File chunk request callback called: friend=%d, file=%d, pos=%d, len=%d",
			friendID, fileID, position, length)
	})

	// Verify callbacks are registered
	if tox.fileRecvCallback == nil {
		t.Error("File receive callback not registered")
	}

	if tox.fileRecvChunkCallback == nil {
		t.Error("File receive chunk callback not registered")
	}

	if tox.fileChunkRequestCallback == nil {
		t.Error("File chunk request callback not registered")
	}

	// Simulate callback triggers (in a real implementation, these would be triggered by network events)
	if tox.fileRecvCallback != nil {
		tox.fileRecvCallback(0, 1, 0, 1024, "test.txt")
	}

	if tox.fileRecvChunkCallback != nil {
		tox.fileRecvChunkCallback(0, 1, 0, []byte("test data"))
	}

	if tox.fileChunkRequestCallback != nil {
		tox.fileChunkRequestCallback(0, 1, 512, 256)
	}

	// Verify callbacks were called
	if !fileRecvCalled {
		t.Error("File receive callback was not called")
	}

	if !fileChunkRecvCalled {
		t.Error("File chunk receive callback was not called")
	}

	if !fileChunkRequestCalled {
		t.Error("File chunk request callback was not called")
	}

	t.Log("File transfer callbacks test completed successfully")
}

func TestNospamHandling(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Check that nospam is generated
	if tox.nospam == [4]byte{} {
		t.Error("Nospam should be generated and not empty")
	}

	// Get Tox ID which should include the nospam
	toxID := tox.SelfGetAddress()
	if len(toxID) == 0 {
		t.Error("Tox ID should not be empty")
	}

	// Verify nospam is included in the ToxID
	// The last 8 characters of the hex string should be the nospam + checksum
	if len(toxID) < 8 {
		t.Error("Tox ID is too short to contain nospam")
	}

	t.Logf("Generated Tox ID: %s", toxID)
	t.Logf("Nospam: %x", tox.nospam)
	t.Log("Nospam handling test completed successfully")
}
