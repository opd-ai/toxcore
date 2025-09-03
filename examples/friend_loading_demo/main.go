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

	// Step 1: Create a Tox instance and add a friend
	fmt.Println("1. Creating initial Tox instance...")
	options1 := toxcore.NewOptions()
	tox1, err := toxcore.New(options1)
	if err != nil {
		log.Fatalf("Failed to create Tox instance: %v", err)
	}

	fmt.Printf("   Tox ID: %s\n", tox1.SelfGetAddress())

	// Add a test friend
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

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

	// Step 2: Get savedata from the first instance
	fmt.Println()
	fmt.Println("2. Getting savedata from first instance...")
	savedata := tox1.GetSavedata()
	fmt.Printf("   Savedata size: %d bytes\n", len(savedata))

	// Clean up first instance
	tox1.Kill()

	// Step 3: Create new instance using savedata in Options (NEW FEATURE)
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
	defer tox2.Kill()

	// Step 4: Verify everything was restored automatically
	fmt.Println()
	fmt.Println("4. Verifying automatic restoration...")
	fmt.Printf("   Restored Tox ID: %s\n", tox2.SelfGetAddress())
	fmt.Printf("   Restored name: %s\n", tox2.SelfGetName())

	if tox2.FriendExists(friendID) {
		fmt.Printf("   ✅ Friend %d automatically restored!\n", friendID)

		restoredKey, err := tox2.GetFriendPublicKey(friendID)
		if err != nil {
			log.Printf("   Failed to get friend key: %v", err)
		} else {
			fmt.Printf("   Friend public key matches: %v\n", restoredKey == testPublicKey)
		}
	} else {
		fmt.Printf("   ❌ Friend was not restored\n")
	}

	fmt.Println()
	fmt.Println("=== Demo completed successfully! ===")
	fmt.Println()
	fmt.Println("Key Benefits:")
	fmt.Println("• Friends automatically loaded during initialization")
	fmt.Println("• No need for separate Load() call")
	fmt.Println("• Clean error handling with automatic cleanup")
	fmt.Println("• Backward compatible with existing code")
}
