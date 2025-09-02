package toxcore

import (
	"strings"
	"testing"
)

// TestSelfSetName tests setting and retrieving the self name
func TestSelfSetName(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting a normal name
	testName := "Alice"
	err = tox.SelfSetName(testName)
	if err != nil {
		t.Errorf("Failed to set name: %v", err)
	}

	// Test retrieving the name
	retrievedName := tox.SelfGetName()
	if retrievedName != testName {
		t.Errorf("Name mismatch: expected '%s', got '%s'", testName, retrievedName)
	}
}

// TestSelfSetNameEmpty tests setting an empty name
func TestSelfSetNameEmpty(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting empty name (should be allowed)
	err = tox.SelfSetName("")
	if err != nil {
		t.Errorf("Failed to set empty name: %v", err)
	}

	// Test retrieving empty name
	retrievedName := tox.SelfGetName()
	if retrievedName != "" {
		t.Errorf("Expected empty name, got '%s'", retrievedName)
	}
}

// TestSelfSetNameTooLong tests name length validation
func TestSelfSetNameTooLong(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting a name that's too long (>128 bytes)
	longName := strings.Repeat("a", 129)
	err = tox.SelfSetName(longName)
	if err == nil {
		t.Error("Expected error for name that's too long")
	}

	// Test that the name wasn't changed
	retrievedName := tox.SelfGetName()
	if retrievedName != "" {
		t.Errorf("Name should be empty after failed set, got '%s'", retrievedName)
	}
}

// TestSelfSetNameMaxLength tests the maximum allowed name length
func TestSelfSetNameMaxLength(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting a name that's exactly 128 bytes (should work)
	maxName := strings.Repeat("a", 128)
	err = tox.SelfSetName(maxName)
	if err != nil {
		t.Errorf("Failed to set max length name: %v", err)
	}

	// Test retrieving the max length name
	retrievedName := tox.SelfGetName()
	if retrievedName != maxName {
		t.Errorf("Max length name mismatch: expected length %d, got length %d", len(maxName), len(retrievedName))
	}
}

// TestSelfSetStatusMessage tests setting and retrieving the status message
func TestSelfSetStatusMessage(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting a normal status message
	testMessage := "I'm online and ready to chat!"
	err = tox.SelfSetStatusMessage(testMessage)
	if err != nil {
		t.Errorf("Failed to set status message: %v", err)
	}

	// Test retrieving the status message
	retrievedMessage := tox.SelfGetStatusMessage()
	if retrievedMessage != testMessage {
		t.Errorf("Status message mismatch: expected '%s', got '%s'", testMessage, retrievedMessage)
	}
}

// TestSelfSetStatusMessageEmpty tests setting an empty status message
func TestSelfSetStatusMessageEmpty(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting empty status message (should be allowed)
	err = tox.SelfSetStatusMessage("")
	if err != nil {
		t.Errorf("Failed to set empty status message: %v", err)
	}

	// Test retrieving empty status message
	retrievedMessage := tox.SelfGetStatusMessage()
	if retrievedMessage != "" {
		t.Errorf("Expected empty status message, got '%s'", retrievedMessage)
	}
}

// TestSelfSetStatusMessageTooLong tests status message length validation
func TestSelfSetStatusMessageTooLong(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting a status message that's too long (>1007 bytes)
	longMessage := strings.Repeat("a", 1008)
	err = tox.SelfSetStatusMessage(longMessage)
	if err == nil {
		t.Error("Expected error for status message that's too long")
	}

	// Test that the status message wasn't changed
	retrievedMessage := tox.SelfGetStatusMessage()
	if retrievedMessage != "" {
		t.Errorf("Status message should be empty after failed set, got '%s'", retrievedMessage)
	}
}

// TestSelfSetStatusMessageMaxLength tests the maximum allowed status message length
func TestSelfSetStatusMessageMaxLength(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting a status message that's exactly 1007 bytes (should work)
	maxMessage := strings.Repeat("a", 1007)
	err = tox.SelfSetStatusMessage(maxMessage)
	if err != nil {
		t.Errorf("Failed to set max length status message: %v", err)
	}

	// Test retrieving the max length status message
	retrievedMessage := tox.SelfGetStatusMessage()
	if retrievedMessage != maxMessage {
		t.Errorf("Max length status message mismatch: expected length %d, got length %d", len(maxMessage), len(retrievedMessage))
	}
}

// TestSelfInfoPersistence tests that self information persists in savedata
func TestSelfInfoPersistence(t *testing.T) {
	// Create first Tox instance
	options := NewOptions()
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Set self information
	testName := "Bob"
	testMessage := "Building cool things with Tox!"

	err = tox1.SelfSetName(testName)
	if err != nil {
		t.Fatalf("Failed to set name: %v", err)
	}

	err = tox1.SelfSetStatusMessage(testMessage)
	if err != nil {
		t.Fatalf("Failed to set status message: %v", err)
	}

	// Get savedata
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

	// Verify self information was restored
	retrievedName := tox2.SelfGetName()
	if retrievedName != testName {
		t.Errorf("Name not restored: expected '%s', got '%s'", testName, retrievedName)
	}

	retrievedMessage := tox2.SelfGetStatusMessage()
	if retrievedMessage != testMessage {
		t.Errorf("Status message not restored: expected '%s', got '%s'", testMessage, retrievedMessage)
	}
}

// TestSelfInfoPersistenceWithLoad tests persistence using Load method
func TestSelfInfoPersistenceWithLoad(t *testing.T) {
	// Create first Tox instance
	options := NewOptions()
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Set self information
	testName := "Charlie"
	testMessage := "Working on Tox protocol"

	err = tox1.SelfSetName(testName)
	if err != nil {
		t.Fatalf("Failed to set name: %v", err)
	}

	err = tox1.SelfSetStatusMessage(testMessage)
	if err != nil {
		t.Fatalf("Failed to set status message: %v", err)
	}

	// Get savedata
	savedata := tox1.GetSavedata()

	// Create second Tox instance
	tox2, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Load the savedata
	err = tox2.Load(savedata)
	if err != nil {
		t.Fatalf("Failed to load savedata: %v", err)
	}

	// Verify self information was loaded
	retrievedName := tox2.SelfGetName()
	if retrievedName != testName {
		t.Errorf("Name not loaded: expected '%s', got '%s'", testName, retrievedName)
	}

	retrievedMessage := tox2.SelfGetStatusMessage()
	if retrievedMessage != testMessage {
		t.Errorf("Status message not loaded: expected '%s', got '%s'", testMessage, retrievedMessage)
	}
}

// TestSelfInfoUTF8 tests UTF-8 support in name and status message
func TestSelfInfoUTF8(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test UTF-8 name
	utf8Name := "Alice 🌟"
	err = tox.SelfSetName(utf8Name)
	if err != nil {
		t.Errorf("Failed to set UTF-8 name: %v", err)
	}

	retrievedName := tox.SelfGetName()
	if retrievedName != utf8Name {
		t.Errorf("UTF-8 name mismatch: expected '%s', got '%s'", utf8Name, retrievedName)
	}

	// Test UTF-8 status message
	utf8Message := "Having fun with Go 🐹 and Tox 💬"
	err = tox.SelfSetStatusMessage(utf8Message)
	if err != nil {
		t.Errorf("Failed to set UTF-8 status message: %v", err)
	}

	retrievedMessage := tox.SelfGetStatusMessage()
	if retrievedMessage != utf8Message {
		t.Errorf("UTF-8 status message mismatch: expected '%s', got '%s'", utf8Message, retrievedMessage)
	}
}

// TestSelfInfoConcurrency tests concurrent access to self information
func TestSelfInfoConcurrency(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Run concurrent operations
	done := make(chan bool, 4)

	// Concurrent name operations
	go func() {
		for i := 0; i < 100; i++ {
			_ = tox.SelfSetName("Name1")
			_ = tox.SelfGetName()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = tox.SelfSetName("Name2")
			_ = tox.SelfGetName()
		}
		done <- true
	}()

	// Concurrent status message operations
	go func() {
		for i := 0; i < 100; i++ {
			_ = tox.SelfSetStatusMessage("Status1")
			_ = tox.SelfGetStatusMessage()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = tox.SelfSetStatusMessage("Status2")
			_ = tox.SelfGetStatusMessage()
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 4; i++ {
		<-done
	}

	// No specific assertions needed - the test passes if no race conditions occur
	t.Log("Concurrency test completed successfully")
}
