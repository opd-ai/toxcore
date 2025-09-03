package toxcore

import (
	"bytes"
	"testing"
	"time"
)

// TestGetSavedata tests basic savedata serialization functionality
func TestGetSavedata(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Get savedata
	savedata := tox.GetSavedata()
	if len(savedata) == 0 {
		t.Fatal("GetSavedata returned empty data")
	}

	// Verify it's valid JSON
	var testData toxSaveData
	if err := testData.unmarshal(savedata); err != nil {
		t.Fatalf("Failed to unmarshal savedata: %v", err)
	}

	// Verify key pair is present
	if testData.KeyPair == nil {
		t.Fatal("Savedata missing key pair")
	}

	// Verify key pair matches original
	if !bytes.Equal(testData.KeyPair.Public[:], tox.keyPair.Public[:]) {
		t.Error("Public key mismatch in savedata")
	}
	if !bytes.Equal(testData.KeyPair.Private[:], tox.keyPair.Private[:]) {
		t.Error("Private key mismatch in savedata")
	}
}

// TestSavedataRoundTrip tests save and load functionality together
func TestSavedataRoundTrip(t *testing.T) {
	// Create first Tox instance
	options := NewOptions()
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Add some friends to test friend persistence
	publicKey1 := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	publicKey2 := [32]byte{32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}

	friendID1, err := tox1.AddFriendByPublicKey(publicKey1)
	if err != nil {
		t.Fatalf("Failed to add first friend: %v", err)
	}

	friendID2, err := tox1.AddFriendByPublicKey(publicKey2)
	if err != nil {
		t.Fatalf("Failed to add second friend: %v", err)
	}

	// Manually set friend data for testing
	tox1.friends[friendID1].Name = "Test Friend 1"
	tox1.friends[friendID1].StatusMessage = "Hello World"
	tox1.friends[friendID1].Status = FriendStatusOnline
	tox1.friends[friendID1].LastSeen = time.Now()

	tox1.friends[friendID2].Name = "Test Friend 2"
	tox1.friends[friendID2].StatusMessage = "Another Status"
	tox1.friends[friendID2].Status = FriendStatusAway
	tox1.friends[friendID2].LastSeen = time.Now()

	// Get savedata from first instance
	savedata := tox1.GetSavedata()
	if len(savedata) == 0 {
		t.Fatal("GetSavedata returned empty data")
	}

	// Create second Tox instance from savedata
	tox2, err := NewFromSavedata(nil, savedata)
	if err != nil {
		t.Fatalf("Failed to create Tox instance from savedata: %v", err)
	}
	defer tox2.Kill()

	// Verify key pairs match
	if !bytes.Equal(tox1.keyPair.Public[:], tox2.keyPair.Public[:]) {
		t.Error("Public keys don't match after restore")
	}
	if !bytes.Equal(tox1.keyPair.Private[:], tox2.keyPair.Private[:]) {
		t.Error("Private keys don't match after restore")
	}

	// Verify friends were restored
	if len(tox2.friends) != 2 {
		t.Errorf("Expected 2 friends, got %d", len(tox2.friends))
	}

	// Check friend 1 data
	friend1, exists := tox2.friends[friendID1]
	if !exists {
		t.Fatal("Friend 1 not found after restore")
	}
	if !bytes.Equal(friend1.PublicKey[:], publicKey1[:]) {
		t.Error("Friend 1 public key mismatch after restore")
	}
	if friend1.Name != "Test Friend 1" {
		t.Errorf("Friend 1 name mismatch: expected 'Test Friend 1', got '%s'", friend1.Name)
	}
	if friend1.StatusMessage != "Hello World" {
		t.Errorf("Friend 1 status message mismatch: expected 'Hello World', got '%s'", friend1.StatusMessage)
	}
	if friend1.Status != FriendStatusOnline {
		t.Errorf("Friend 1 status mismatch: expected %d, got %d", FriendStatusOnline, friend1.Status)
	}

	// Check friend 2 data
	friend2, exists := tox2.friends[friendID2]
	if !exists {
		t.Fatal("Friend 2 not found after restore")
	}
	if !bytes.Equal(friend2.PublicKey[:], publicKey2[:]) {
		t.Error("Friend 2 public key mismatch after restore")
	}
	if friend2.Name != "Test Friend 2" {
		t.Errorf("Friend 2 name mismatch: expected 'Test Friend 2', got '%s'", friend2.Name)
	}
}

// TestLoadInvalidData tests error handling for invalid savedata
func TestLoadInvalidData(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test empty data
	err = tox.Load([]byte{})
	if err == nil {
		t.Error("Expected error for empty data")
	}

	// Test invalid JSON
	err = tox.Load([]byte("invalid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	// Test JSON without key pair
	invalidData := []byte(`{"friends":{},"options":{}}`)
	err = tox.Load(invalidData)
	if err == nil {
		t.Error("Expected error for data without key pair")
	}
}

// TestNewFromSavedataErrors tests error cases for NewFromSavedata
func TestNewFromSavedataErrors(t *testing.T) {
	// Test empty savedata
	_, err := NewFromSavedata(nil, []byte{})
	if err == nil {
		t.Error("Expected error for empty savedata")
	}

	// Test invalid savedata
	_, err = NewFromSavedata(nil, []byte("invalid"))
	if err == nil {
		t.Error("Expected error for invalid savedata")
	}

	// Test savedata without key pair
	invalidData := []byte(`{"friends":{},"options":{}}`)
	_, err = NewFromSavedata(nil, invalidData)
	if err == nil {
		t.Error("Expected error for savedata without key pair")
	}
}

// TestSavedataWithoutFriends tests savedata functionality with no friends
func TestSavedataWithoutFriends(t *testing.T) {
	options := NewOptions()
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Get savedata with no friends
	savedata := tox1.GetSavedata()
	if len(savedata) == 0 {
		t.Fatal("GetSavedata returned empty data")
	}

	// Restore from savedata
	tox2, err := NewFromSavedata(nil, savedata)
	if err != nil {
		t.Fatalf("Failed to restore from savedata: %v", err)
	}
	defer tox2.Kill()

	// Verify keys match
	if !bytes.Equal(tox1.keyPair.Public[:], tox2.keyPair.Public[:]) {
		t.Error("Public keys don't match")
	}

	// Verify empty friends list
	if len(tox2.friends) != 0 {
		t.Errorf("Expected 0 friends, got %d", len(tox2.friends))
	}
}

// TestSavedataMultipleRoundTrips tests multiple save/restore cycles
func TestSavedataMultipleRoundTrips(t *testing.T) {
	options := NewOptions()
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	originalPublicKey := tox1.keyPair.Public

	// First round trip
	savedata1 := tox1.GetSavedata()
	tox2, err := NewFromSavedata(nil, savedata1)
	if err != nil {
		t.Fatalf("Failed first round trip: %v", err)
	}
	defer tox2.Kill()

	// Second round trip
	savedata2 := tox2.GetSavedata()
	tox3, err := NewFromSavedata(nil, savedata2)
	if err != nil {
		t.Fatalf("Failed second round trip: %v", err)
	}
	defer tox3.Kill()

	// Verify key consistency
	if !bytes.Equal(originalPublicKey[:], tox3.keyPair.Public[:]) {
		t.Error("Public key changed after multiple round trips")
	}
}

// TestSavedataFormat tests the structure of the saved data
func TestSavedataFormat(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	savedata := tox.GetSavedata()

	var data toxSaveData
	if err := data.unmarshal(savedata); err != nil {
		t.Fatalf("Failed to unmarshal savedata: %v", err)
	}

	// Verify structure contains expected fields
	if data.KeyPair == nil {
		t.Error("Savedata missing KeyPair")
	}
	if data.Friends == nil {
		t.Error("Savedata missing Friends map")
	}
	if data.Options == nil {
		t.Error("Savedata missing Options")
	}

	// Verify key pair structure
	if len(data.KeyPair.Public) != 32 {
		t.Errorf("Public key wrong length: expected 32, got %d", len(data.KeyPair.Public))
	}
	if len(data.KeyPair.Private) != 32 {
		t.Errorf("Private key wrong length: expected 32, got %d", len(data.KeyPair.Private))
	}
}

// TestNewWithToxSavedata tests the New function with SaveDataTypeToxSave
func TestNewWithToxSavedata(t *testing.T) {
	// Create first Tox instance and add a friend
	options1 := NewOptions()
	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Add a test friend
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox1.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set some self information
	err = tox1.SelfSetName("Test User")
	if err != nil {
		t.Fatalf("Failed to set name: %v", err)
	}

	// Get savedata from first instance
	savedata := tox1.GetSavedata()
	if len(savedata) == 0 {
		t.Fatal("GetSavedata returned empty data")
	}

	// Create new Tox instance using the savedata in options
	options2 := &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   savedata,
		SavedataLength: uint32(len(savedata)),
	}

	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create Tox instance with savedata: %v", err)
	}
	defer tox2.Kill()

	// Verify the key pair was restored
	if !bytes.Equal(tox1.keyPair.Public[:], tox2.keyPair.Public[:]) {
		t.Error("Public keys don't match after loading from options")
	}
	if !bytes.Equal(tox1.keyPair.Private[:], tox2.keyPair.Private[:]) {
		t.Error("Private keys don't match after loading from options")
	}

	// Verify the friend was restored
	if !tox2.FriendExists(friendID) {
		t.Error("Friend was not restored from savedata")
	}

	// Verify friend's public key
	restoredKey, err := tox2.GetFriendPublicKey(friendID)
	if err != nil {
		t.Fatalf("Failed to get friend public key: %v", err)
	}
	if !bytes.Equal(testPublicKey[:], restoredKey[:]) {
		t.Error("Friend public key doesn't match")
	}

	// Verify self information was restored
	selfName := tox2.SelfGetName()
	if selfName != "Test User" {
		t.Errorf("Self name not restored: expected 'Test User', got '%s'", selfName)
	}
}

// TestNewWithToxSavedataErrors tests error cases for New with ToxSave data
func TestNewWithToxSavedataErrors(t *testing.T) {
	// Test with empty savedata
	options := &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   []byte{},
		SavedataLength: 0,
	}
	_, err := New(options)
	if err == nil {
		t.Error("Expected error for empty ToxSave data")
	}

	// Test with nil savedata
	options = &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   nil,
		SavedataLength: 0,
	}
	_, err = New(options)
	if err == nil {
		t.Error("Expected error for nil ToxSave data")
	}

	// Test with invalid savedata
	options = &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   []byte("invalid json"),
		SavedataLength: uint32(len("invalid json")),
	}
	_, err = New(options)
	if err == nil {
		t.Error("Expected error for invalid ToxSave data")
	}

	// Test with length mismatch
	validSavedata := []byte(`{"keyPair":{"public":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","private":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},"friends":{},"options":{"savedataType":0},"selfName":"","selfStatusMsg":"","nospam":"AAAAAA=="}`)
	options = &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   validSavedata,
		SavedataLength: uint32(len(validSavedata) + 10), // Wrong length
	}
	_, err = New(options)
	if err == nil {
		t.Error("Expected error for length mismatch")
	}
}

// TestNewWithDifferentSavedataTypes tests different savedata type handling
func TestNewWithDifferentSavedataTypes(t *testing.T) {
	// Test with SaveDataTypeNone
	options := &Options{
		SavedataType: SaveDataTypeNone,
	}
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox with SaveDataTypeNone: %v", err)
	}
	tox.Kill()

	// Test with SaveDataTypeSecretKey (should work as before)
	testSecretKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	options = &Options{
		SavedataType:   SaveDataTypeSecretKey,
		SavedataData:   testSecretKey[:],
		SavedataLength: 32,
	}
	tox, err = New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox with SaveDataTypeSecretKey: %v", err)
	}
	tox.Kill()

	// Test with unknown savedata type
	options = &Options{
		SavedataType: SaveDataType(255), // Unknown type
	}
	_, err = New(options)
	if err == nil {
		t.Error("Expected error for unknown savedata type")
	}
}
