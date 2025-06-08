package toxcore

import (
	"strings"
	"testing"
)

func TestSelfSetName(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting a valid name
	testName := "TestUser"
	err = tox.SelfSetName(testName)
	if err != nil {
		t.Errorf("Failed to set name: %v", err)
	}

	// Test getting the name
	retrievedName := tox.SelfGetName()
	if retrievedName != testName {
		t.Errorf("Expected name %s, got %s", testName, retrievedName)
	}

	// Test setting a name that's too long (> 128 bytes)
	longName := strings.Repeat("a", 129)
	err = tox.SelfSetName(longName)
	if err == nil {
		t.Error("Expected error when setting name too long, but got none")
	}

	// Test that the name wasn't changed after the error
	retrievedName = tox.SelfGetName()
	if retrievedName != testName {
		t.Errorf("Name should not have changed after error. Expected %s, got %s", testName, retrievedName)
	}
}

func TestSelfSetStatusMessage(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test setting a valid status message
	testStatus := "Online and ready to chat!"
	err = tox.SelfSetStatusMessage(testStatus)
	if err != nil {
		t.Errorf("Failed to set status message: %v", err)
	}

	// Test getting the status message
	retrievedStatus := tox.SelfGetStatusMessage()
	if retrievedStatus != testStatus {
		t.Errorf("Expected status message %s, got %s", testStatus, retrievedStatus)
	}

	// Test setting a status message that's too long (> 1007 bytes)
	longStatus := strings.Repeat("a", 1008)
	err = tox.SelfSetStatusMessage(longStatus)
	if err == nil {
		t.Error("Expected error when setting status message too long, but got none")
	}

	// Test that the status message wasn't changed after the error
	retrievedStatus = tox.SelfGetStatusMessage()
	if retrievedStatus != testStatus {
		t.Errorf("Status message should not have changed after error. Expected %s, got %s", testStatus, retrievedStatus)
	}
}

func TestSelfInfoPersistence(t *testing.T) {
	// Create a Tox instance and set self information
	tox1, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	testName := "PersistentUser"
	testStatus := "Testing persistence"

	err = tox1.SelfSetName(testName)
	if err != nil {
		t.Errorf("Failed to set name: %v", err)
	}

	err = tox1.SelfSetStatusMessage(testStatus)
	if err != nil {
		t.Errorf("Failed to set status message: %v", err)
	}

	// Save the state
	saveData, err := tox1.Save()
	if err != nil {
		t.Fatalf("Failed to save Tox state: %v", err)
	}

	tox1.Kill()

	// Create a new Tox instance and load the saved state
	options := NewOptions()
	options.SavedataType = SaveDataTypeToxSave
	options.SavedataData = saveData

	tox2, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance from saved data: %v", err)
	}
	defer tox2.Kill()

	// Verify that the self information was restored
	retrievedName := tox2.SelfGetName()
	if retrievedName != testName {
		t.Errorf("Expected name %s after loading, got %s", testName, retrievedName)
	}

	retrievedStatus := tox2.SelfGetStatusMessage()
	if retrievedStatus != testStatus {
		t.Errorf("Expected status message %s after loading, got %s", testStatus, retrievedStatus)
	}
}

func TestSelfInfoDefaults(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test that default name is empty
	defaultName := tox.SelfGetName()
	if defaultName != "" {
		t.Errorf("Expected empty default name, got %s", defaultName)
	}

	// Test that default status message is empty
	defaultStatus := tox.SelfGetStatusMessage()
	if defaultStatus != "" {
		t.Errorf("Expected empty default status message, got %s", defaultStatus)
	}
}

func TestSelfInfoBoundaryValues(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test maximum valid name length (128 bytes)
	maxName := strings.Repeat("a", 128)
	err = tox.SelfSetName(maxName)
	if err != nil {
		t.Errorf("Failed to set maximum length name: %v", err)
	}

	retrievedName := tox.SelfGetName()
	if retrievedName != maxName {
		t.Errorf("Maximum length name not preserved. Expected length %d, got %d", len(maxName), len(retrievedName))
	}

	// Test maximum valid status message length (1007 bytes)
	maxStatus := strings.Repeat("b", 1007)
	err = tox.SelfSetStatusMessage(maxStatus)
	if err != nil {
		t.Errorf("Failed to set maximum length status message: %v", err)
	}

	retrievedStatus := tox.SelfGetStatusMessage()
	if retrievedStatus != maxStatus {
		t.Errorf("Maximum length status message not preserved. Expected length %d, got %d", len(maxStatus), len(retrievedStatus))
	}

	// Test empty strings (should be valid)
	err = tox.SelfSetName("")
	if err != nil {
		t.Errorf("Failed to set empty name: %v", err)
	}

	err = tox.SelfSetStatusMessage("")
	if err != nil {
		t.Errorf("Failed to set empty status message: %v", err)
	}

	if tox.SelfGetName() != "" {
		t.Error("Empty name not preserved")
	}

	if tox.SelfGetStatusMessage() != "" {
		t.Error("Empty status message not preserved")
	}
}
