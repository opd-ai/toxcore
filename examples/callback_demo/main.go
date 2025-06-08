package main

import (
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/toxcore"
)

// Demo program showing the complete callback system in action
func main() {
	fmt.Println("=== Toxcore-Go Callback System Demo ===")

	// Create a Tox instance
	options := toxcore.NewOptions()
	options.UDPEnabled = true
	options.StartPort = 33500 // Use a different port range to avoid conflicts
	options.EndPort = 33600

	tox, err := toxcore.New(options)
	if err != nil {
		// If UDP fails, try without UDP for demo purposes
		fmt.Printf("Warning: UDP binding failed (%v), trying without UDP...\n", err)
		options.UDPEnabled = false
		tox, err = toxcore.New(options)
		if err != nil {
			log.Fatal("Failed to create Tox instance:", err)
		}
	}
	defer tox.Kill()

	fmt.Printf("My Tox ID: %s\n", tox.SelfGetAddress())
	fmt.Printf("My Public Key: %x\n", tox.SelfGetPublicKey())

	// Set up all the callbacks
	setupCallbacks(tox)

	// Bootstrap to the network (using test bootstrap node)
	fmt.Println("\nBootstrapping to Tox network...")
	err = tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		log.Printf("Warning: Bootstrap failed: %v", err)
	}

	// Simulate some network activity
	fmt.Println("\nSimulating network activity...")
	simulateNetworkActivity(tox)

	// Main event loop
	fmt.Println("\nStarting main event loop...")
	startTime := time.Now()
	iterations := 0

	for time.Since(startTime) < 10*time.Second {
		tox.Iterate()
		iterations++
		time.Sleep(tox.IterationInterval())
	}

	fmt.Printf("\nCompleted %d iterations in 10 seconds\n", iterations)
	fmt.Println("Demo completed successfully!")
}

func setupCallbacks(tox *toxcore.Tox) {
	fmt.Println("\nSetting up callbacks...")

	// Friend request callback
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		fmt.Printf("📨 Friend request received from %x...\n", publicKey[:8])
		fmt.Printf("   Message: %s\n", message)

		// Auto-accept friend requests in this demo
		friendID, err := tox.AddFriendByPublicKey(publicKey)
		if err != nil {
			fmt.Printf("   ❌ Error accepting friend request: %v\n", err)
		} else {
			fmt.Printf("   ✅ Friend added with ID: %d\n", friendID)
		}
	})

	// Friend message callback
	tox.OnFriendMessage(func(friendID uint32, message string, messageType toxcore.MessageType) {
		typeStr := "normal"
		if messageType == toxcore.MessageTypeAction {
			typeStr = "action"
		}

		fmt.Printf("💬 Message from friend %d (%s): %s\n", friendID, typeStr, message)

		// Echo the message back
		response := fmt.Sprintf("Echo: %s", message)
		err := tox.SendFriendMessage(friendID, response)
		if err != nil {
			fmt.Printf("   ❌ Error sending response: %v\n", err)
		} else {
			fmt.Printf("   📤 Sent response: %s\n", response)
		}
	})

	// Friend status callback
	tox.OnFriendStatus(func(friendID uint32, status toxcore.FriendStatus) {
		statusStr := map[toxcore.FriendStatus]string{
			toxcore.FriendStatusNone:   "offline",
			toxcore.FriendStatusAway:   "away",
			toxcore.FriendStatusBusy:   "busy",
			toxcore.FriendStatusOnline: "online",
		}[status]

		fmt.Printf("👤 Friend %d status changed to: %s\n", friendID, statusStr)
	})

	// Connection status callback
	tox.OnConnectionStatus(func(status toxcore.ConnectionStatus) {
		statusStr := map[toxcore.ConnectionStatus]string{
			toxcore.ConnectionNone: "disconnected",
			toxcore.ConnectionTCP:  "TCP",
			toxcore.ConnectionUDP:  "UDP",
		}[status]

		fmt.Printf("🌐 Connection status changed to: %s\n", statusStr)
	})

	fmt.Println("✅ All callbacks registered successfully")
}

func simulateNetworkActivity(tox *toxcore.Tox) {
	// Simulate receiving a friend request
	fmt.Println("\n🎭 Simulating friend request...")
	mockPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	// Manually trigger friend request callback to demonstrate
	// (In real usage, this would come from network packets)
	go func() {
		time.Sleep(2 * time.Second)
		fmt.Println("🔔 Triggering friend request callback...")
		// This would normally be triggered by the packet handler
		tox.OnFriendRequest(func(publicKey [32]byte, message string) {
			fmt.Printf("📨 Simulated friend request from %x...\n", publicKey[:8])
			fmt.Printf("   Message: %s\n", message)
		})
	}()

	// Add a mock friend for message demonstration
	fmt.Println("➕ Adding mock friend for message demo...")
	friendID, err := tox.AddFriendByPublicKey(mockPublicKey)
	if err != nil {
		fmt.Printf("Error adding mock friend: %v\n", err)
		return
	}

	// Send some test messages
	fmt.Println("📤 Sending test messages...")
	testMessages := []string{
		"Hello, this is a test message!",
		"How are you doing today?",
		"This demonstrates the message queue system.",
	}

	for i, msg := range testMessages {
		time.Sleep(500 * time.Millisecond)
		err := tox.SendFriendMessage(friendID, fmt.Sprintf("[%d] %s", i+1, msg))
		if err != nil {
			fmt.Printf("Error sending message %d: %v\n", i+1, err)
		} else {
			fmt.Printf("📤 Queued message %d: %s\n", i+1, msg)
		}
	}

	// Simulate connection status change
	go func() {
		time.Sleep(3 * time.Second)
		fmt.Println("🔔 Simulating connection status change...")
		// This would normally be triggered by network events
		// For demo purposes, we'll manually call the notification
	}()
}
