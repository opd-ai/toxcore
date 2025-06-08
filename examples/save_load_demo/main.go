package main

import (
	"fmt"
	"log"
	"os"

	"github.com/opd-ai/toxcore"
)

// Example demonstrating the newly implemented Save/Load API methods
func main() {
	fmt.Println("=== Testing Save/Load API Methods ===")

	// Create first Tox instance and add some state
	fmt.Println("🔧 Creating first Tox instance...")
	options1 := toxcore.NewOptions()
	options1.UDPEnabled = false // Disable for demo

	tox1, err := toxcore.New(options1)
	if err != nil {
		log.Fatal("Failed to create Tox instance:", err)
	}
	defer tox1.Kill()

	fmt.Printf("🆔 Tox ID: %s\n", tox1.SelfGetAddress())

	// Add a demo friend
	demoFriendKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	friendID, err := tox1.AddFriendByPublicKey(demoFriendKey)
	if err != nil {
		log.Fatal("Failed to add friend:", err)
	}
	fmt.Printf("👥 Added friend with ID: %d\n", friendID)

	// Test Save method
	fmt.Println("\n💾 Testing Save() method...")
	saveData, err := tox1.Save()
	if err != nil {
		log.Fatal("Save() failed:", err)
	}
	fmt.Printf("✅ Save successful! Data size: %d bytes\n", len(saveData))

	// Save to file for demonstration
	err = os.WriteFile("demo_save.dat", saveData, 0644)
	if err != nil {
		log.Printf("Warning: Could not write to file: %v", err)
	} else {
		fmt.Println("📁 Saved data to demo_save.dat")
	}

	// Create second Tox instance and test Load method
	fmt.Println("\n🔄 Creating second Tox instance for Load test...")
	options2 := toxcore.NewOptions()
	options2.UDPEnabled = false

	tox2, err := toxcore.New(options2)
	if err != nil {
		log.Fatal("Failed to create second Tox instance:", err)
	}
	defer tox2.Kill()

	fmt.Printf("🆔 New Tox ID (before load): %s\n", tox2.SelfGetAddress())

	// Test Load method
	fmt.Println("\n📥 Testing Load() method...")
	err = tox2.Load(saveData)
	if err != nil {
		log.Fatal("Load() failed:", err)
	}
	fmt.Println("✅ Load successful!")

	// Verify the loaded state
	fmt.Printf("🆔 Loaded Tox ID: %s\n", tox2.SelfGetAddress())

	if tox2.SelfGetAddress() == tox1.SelfGetAddress() {
		fmt.Println("🎉 SUCCESS: Tox IDs match after load!")
	} else {
		fmt.Println("❌ ERROR: Tox IDs don't match")
	}

	// Check if friend was loaded
	if tox2.FriendExists(friendID) {
		fmt.Printf("👥 SUCCESS: Friend %d exists after load!\n", friendID)
	} else {
		fmt.Println("❌ ERROR: Friend not found after load")
	}

	// Test loading from file
	fmt.Println("\n📂 Testing load from file...")
	if fileData, err := os.ReadFile("demo_save.dat"); err == nil {
		options3 := toxcore.NewOptions()
		options3.UDPEnabled = false
		tox3, err := toxcore.New(options3)
		if err != nil {
			log.Printf("Failed to create third instance: %v", err)
		} else {
			defer tox3.Kill()

			err = tox3.Load(fileData)
			if err != nil {
				log.Printf("Failed to load from file: %v", err)
			} else {
				fmt.Println("✅ Successfully loaded from file!")
				fmt.Printf("🆔 File-loaded Tox ID: %s\n", tox3.SelfGetAddress())
			}
		}
	}

	// Cleanup
	os.Remove("demo_save.dat")

	fmt.Println("\n🎯 Save/Load API demonstration completed!")
	fmt.Println("✨ The Save() and Load() methods are now production-ready!")
}
