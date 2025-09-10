package main

import (
	"fmt"
	"log"

	"github.com/opd-ai/toxcore"
)

// This demo shows the new automatic friend loading feature during initialization
func main() {
	fmt.Println("=== Friend Loading During Initialization Demo ===")
	fmt.Println()

	// Step 1: Create initial instance and setup
	tox1, friendID, testPublicKey := setupInitialInstance()

	// Step 2: Get savedata for restoration
	savedata := extractSavedataFromInstance(tox1)

	// Step 3: Create new instance with automatic loading
	restoredInstance := createInstanceWithSavedata(savedata)
	defer restoredInstance.Kill()

	// Step 4: Verify automatic restoration
	verifyAutomaticRestoration(restoredInstance, friendID, testPublicKey)

	displayDemoSummary()
}

// setupInitialInstance creates a Tox instance, adds a friend, and sets up the demo user.
// Returns the Tox instance, friend ID and public key for later use.
func setupInitialInstance() (*toxcore.Tox, uint32, [32]byte) {
	fmt.Println("1. Creating initial Tox instance...")
	options1 := toxcore.NewOptions()
	tox1, err := toxcore.New(options1)
	if err != nil {
		log.Fatalf("Failed to create Tox instance: %v", err)
	}

	fmt.Printf("   Tox ID: %s\n", tox1.SelfGetAddress())

	// Add a test friend
	testPublicKey := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}

	friendID, err := tox1.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		log.Fatalf("Failed to add friend: %v", err)
	}

	err = tox1.SelfSetName("Demo User")
	if err != nil {
		log.Fatalf("Failed to set name: %v", err)
	}

	fmt.Printf("   Added friend ID: %d\n", friendID)
	fmt.Printf("   Self name: %s\n", tox1.SelfGetName())

	return tox1, friendID, testPublicKey
}

// extractSavedataFromInstance retrieves savedata from the instance and cleans it up.
// Returns the savedata bytes for creating a new instance.
func extractSavedataFromInstance(tox1 *toxcore.Tox) []byte {
	fmt.Println()
	fmt.Println("2. Getting savedata from first instance...")
	savedata := tox1.GetSavedata()
	fmt.Printf("   Savedata size: %d bytes\n", len(savedata))

	// Clean up first instance
	tox1.Kill()

	return savedata
}

// createInstanceWithSavedata creates a new Tox instance using the provided savedata.
// Returns the restored Tox instance.
func createInstanceWithSavedata(savedata []byte) *toxcore.Tox {
	fmt.Println()
	fmt.Println("3. Creating new instance with automatic friend loading...")
	options2 := &toxcore.Options{
		UDPEnabled:     true,
		SavedataType:   toxcore.SaveDataTypeToxSave,
		SavedataData:   savedata,
		SavedataLength: uint32(len(savedata)),
	}

	tox2, err := toxcore.New(options2)
	if err != nil {
		log.Fatalf("Failed to create Tox instance with savedata: %v", err)
	}

	return tox2
}

// verifyAutomaticRestoration checks that friends and user data were properly restored.
func verifyAutomaticRestoration(tox *toxcore.Tox, friendID uint32, testPublicKey [32]byte) {
	fmt.Println()
	fmt.Println("4. Verifying automatic restoration...")
	fmt.Printf("   Restored Tox ID: %s\n", tox.SelfGetAddress())
	fmt.Printf("   Restored name: %s\n", tox.SelfGetName())

	if tox.FriendExists(friendID) {
		fmt.Printf("   ✅ Friend %d automatically restored!\n", friendID)

		restoredKey, err := tox.GetFriendPublicKey(friendID)
		if err != nil {
			log.Printf("   Failed to get friend key: %v", err)
		} else {
			fmt.Printf("   Friend public key matches: %v\n", restoredKey == testPublicKey)
		}
	} else {
		fmt.Printf("   ❌ Friend was not restored\n")
	}
}

// displayDemoSummary shows the completion message and key benefits.
func displayDemoSummary() {
	fmt.Println()
	fmt.Println("=== Demo completed successfully! ===")
	fmt.Println()
	fmt.Println("Key Benefits:")
	fmt.Println("• Friends automatically loaded during initialization")
	fmt.Println("• No need for separate Load() call")
	fmt.Println("• Clean error handling with automatic cleanup")
	fmt.Println("• Backward compatible with existing code")
}
