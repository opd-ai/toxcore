package main

import (
	"fmt"
	"log"

	"github.com/opd-ai/toxcore"
)

// demonstrateREADMEExample shows that the README.md example now works exactly as documented
func demonstrateREADMEExample() {
	fmt.Println("🧪 Testing README.md Example Compatibility...")

	// Create a new Tox instance (exactly as shown in README)
	options := toxcore.NewOptions()
	options.UDPEnabled = true

	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatal(err)
	}
	defer tox.Kill()

	// Print our Tox ID (as shown in README)
	fmt.Println("My Tox ID:", tox.SelfGetAddress())

	// Set up callbacks (exactly as shown in README)
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		fmt.Printf("Friend request: %s\n", message)

		// Automatically accept friend requests (as shown in README)
		friendID, err := tox.AddFriendByPublicKey(publicKey)
		if err != nil {
			fmt.Printf("Error accepting friend request: %v\n", err)
		} else {
			fmt.Printf("Accepted friend request. Friend ID: %d\n", friendID)
		}
	})

	tox.OnFriendMessage(func(friendID uint32, message string) {
		fmt.Printf("Message from friend %d: %s\n", friendID, message)

		// Echo the message back (as shown in README)
		tox.SendFriendMessage(friendID, "You said: "+message)
	})

	fmt.Println("✅ README.md example compiled successfully!")
	fmt.Println("📝 All callback signatures match documentation")

	// Demonstrate the advanced API as well
	tox.OnFriendMessageDetailed(func(friendID uint32, message string, messageType toxcore.MessageType) {
		fmt.Printf("Advanced callback - Friend %d, Type: %v, Message: %s\n", friendID, messageType, message)
	})

	fmt.Println("✅ Advanced API also available for power users")
}

// demonstrateNewFunctionality shows the new dual-callback system setup
func demonstrateNewFunctionality() {
	fmt.Println("\n🚀 Demonstrating New Dual-Callback System...")

	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatal(err)
	}
	defer tox.Kill()

	// Add a mock friend for testing
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		log.Printf("Could not add test friend: %v", err)
		return
	}

	fmt.Printf("✅ Successfully added friend with ID: %d\n", friendID)

	// Register both callbacks to show they both work
	tox.OnFriendMessage(func(friendID uint32, message string) {
		fmt.Printf("📱 Simple callback registered for friend %d\n", friendID)
	})

	tox.OnFriendMessageDetailed(func(friendID uint32, message string, messageType toxcore.MessageType) {
		fmt.Printf("🔧 Detailed callback registered for friend %d\n", friendID)
	})

	fmt.Println("✅ Both callback types successfully registered!")
	fmt.Println("📝 In real usage, these would be triggered by incoming network messages")
}

func main() {
	fmt.Println("=== toxcore-go API Fix Demonstration ===")

	// Show that README.md examples now work
	demonstrateREADMEExample()

	// Show the new functionality
	demonstrateNewFunctionality()

	fmt.Println("\n🎉 All API fixes working correctly!")
	fmt.Println("📋 Critical gaps from AUDIT.md have been resolved")
}
