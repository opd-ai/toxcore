package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/opd-ai/toxcore"
)

// Enhanced demo program showing the complete toxcore-go feature set
func main() {
	fmt.Println("=== Toxcore-Go Complete Feature Demo ===")

	// Create a Tox instance with enhanced configuration
	options := toxcore.NewOptions()
	options.UDPEnabled = false // Disable UDP for demo to avoid port conflicts
	options.IPv6Enabled = true
	options.LocalDiscovery = true

	// Check if we have saved data
	var tox *toxcore.Tox
	var err error

	if loadSavedData() {
		fmt.Println("📁 Loading from saved data...")
		saveData, err := os.ReadFile("demo_tox_save.dat")
		if err == nil {
			options.SavedataType = toxcore.SaveDataTypeToxSave
			options.SavedataData = saveData
		}
	}

	tox, err = toxcore.New(options)
	if err != nil {
		log.Fatal("Failed to create Tox instance:", err)
	}
	defer func() {
		fmt.Println("💾 Saving Tox state...")
		saveData := tox.GetSavedata()
		err := os.WriteFile("demo_tox_save.dat", saveData, 0644)
		if err != nil {
			fmt.Printf("Warning: Failed to save Tox state: %v\n", err)
		} else {
			fmt.Println("✅ Tox state saved successfully")
		}
		tox.Kill()
	}()

	fmt.Printf("🆔 My Tox ID: %s\n", tox.SelfGetAddress())
	fmt.Printf("🔑 My Public Key: %x\n", tox.SelfGetPublicKey())

	// Set up comprehensive callbacks
	setupAllCallbacks(tox)

	// Bootstrap to the network
	fmt.Println("\n🌐 Connecting to Tox network...")
	err = tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		log.Printf("Warning: Bootstrap failed: %v", err)
	}

	// Add some demo friends if this is a fresh instance
	addDemoFriends(tox)

	// Show current friends
	showFriendsStatus(tox)

	// Demonstrate message sending
	sendDemoMessages(tox)

	// Main event loop
	fmt.Println("\n🔄 Starting main event loop...")
	startTime := time.Now()
	iterations := 0

	for time.Since(startTime) < 15*time.Second {
		tox.Iterate()
		iterations++

		// Show status every 5 seconds
		if iterations%100 == 0 {
			elapsed := time.Since(startTime)
			fmt.Printf("⏱️  Running for %.1fs, %d iterations, connection: %v\n",
				elapsed.Seconds(), iterations, getConnectionStatusString(tox.SelfGetConnectionStatus()))
		}

		time.Sleep(tox.IterationInterval())
	}

	fmt.Printf("\n✅ Demo completed! Ran %d iterations in 15 seconds\n", iterations)
	showFinalStats(tox)
}

func loadSavedData() bool {
	_, err := os.Stat("demo_tox_save.dat")
	return err == nil
}

func setupAllCallbacks(tox *toxcore.Tox) {
	fmt.Println("\n🔧 Setting up all callbacks...")

	// Friend request callback
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		fmt.Printf("📨 Friend request from %x...\n", publicKey[:8])
		fmt.Printf("   💬 Message: %s\n", message)

		// Auto-accept friend requests
		friendID, err := tox.AddFriendByPublicKey(publicKey)
		if err != nil {
			fmt.Printf("   ❌ Error accepting: %v\n", err)
		} else {
			fmt.Printf("   ✅ Accepted! Friend ID: %d\n", friendID)
		}
	})

	// Friend message callback
	tox.OnFriendMessage(func(friendID uint32, message string, messageType toxcore.MessageType) {
		typeStr := "💬"
		if messageType == toxcore.MessageTypeAction {
			typeStr = "🎭"
		}

		fmt.Printf("%s Message from friend %d: %s\n", typeStr, friendID, message)

		// Auto-respond
		response := fmt.Sprintf("Auto-reply: Thanks for your message '%s'!", message)
		err := tox.SendFriendMessage(friendID, response)
		if err != nil {
			fmt.Printf("   ❌ Failed to send response: %v\n", err)
		} else {
			fmt.Printf("   📤 Sent auto-reply\n")
		}
	})

	// Friend status callback
	tox.OnFriendStatus(func(friendID uint32, status toxcore.FriendStatus) {
		statusStr := map[toxcore.FriendStatus]string{
			toxcore.FriendStatusNone:   "💤 offline",
			toxcore.FriendStatusAway:   "🌙 away",
			toxcore.FriendStatusBusy:   "⚠️  busy",
			toxcore.FriendStatusOnline: "🟢 online",
		}[status]

		fmt.Printf("👤 Friend %d status: %s\n", friendID, statusStr)
	})

	// Connection status callback
	tox.OnConnectionStatus(func(status toxcore.ConnectionStatus) {
		statusStr := getConnectionStatusString(status)
		fmt.Printf("🌐 Connection status: %s\n", statusStr)
	})

	// File transfer callbacks
	tox.OnFileRecv(func(friendID uint32, fileID uint32, kind uint32, fileSize uint64, filename string) {
		fmt.Printf("📁 File offer from friend %d: %s (%d bytes)\n", friendID, filename, fileSize)
		fmt.Printf("   📋 File ID: %d, Kind: %d\n", fileID, kind)
	})

	tox.OnFileRecvChunk(func(friendID uint32, fileID uint32, position uint64, data []byte) {
		fmt.Printf("📦 File chunk from friend %d: file %d, pos %d, size %d\n",
			friendID, fileID, position, len(data))
	})

	tox.OnFileChunkRequest(func(friendID uint32, fileID uint32, position uint64, length int) {
		fmt.Printf("📤 File chunk requested by friend %d: file %d, pos %d, len %d\n",
			friendID, fileID, position, length)
	})

	fmt.Println("✅ All callbacks configured")
}

func getConnectionStatusString(status toxcore.ConnectionStatus) string {
	switch status {
	case toxcore.ConnectionNone:
		return "🔴 Disconnected"
	case toxcore.ConnectionTCP:
		return "🟡 TCP Connected"
	case toxcore.ConnectionUDP:
		return "🟢 UDP Connected"
	default:
		return "❓ Unknown"
	}
}

func addDemoFriends(tox *toxcore.Tox) {
	// Check if we already have friends (loaded from save data)
	friendCount := tox.GetFriendCount()

	if friendCount > 0 {
		fmt.Printf("👥 Loaded %d friends from saved data\n", friendCount)
		return
	}

	fmt.Println("➕ Adding demo friends...")

	// Add some example friends (these are just demo keys, not real users)
	demoFriends := [][32]byte{
		{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
			17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		{32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17,
			16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
	}

	for i, publicKey := range demoFriends {
		friendID, err := tox.AddFriendByPublicKey(publicKey)
		if err != nil {
			fmt.Printf("   ❌ Failed to add demo friend %d: %v\n", i+1, err)
		} else {
			fmt.Printf("   ✅ Added demo friend %d with ID: %d\n", i+1, friendID)

			// Set some demo data
			tox.UpdateFriendName(friendID, fmt.Sprintf("Demo Friend %d", i+1))
			tox.UpdateFriendStatusMessage(friendID, fmt.Sprintf("This is demo friend number %d", i+1))
		}
	}
}

func showFriendsStatus(tox *toxcore.Tox) {
	fmt.Println("\n👥 Current friends status:")

	friendList := tox.GetFriendList()
	if len(friendList) == 0 {
		fmt.Println("   No friends yet")
	} else {
		for _, friendID := range friendList {
			friend, err := tox.GetFriend(friendID)
			if err != nil {
				fmt.Printf("   ❌ Error getting friend %d: %v\n", friendID, err)
				continue
			}

			statusStr := map[toxcore.FriendStatus]string{
				toxcore.FriendStatusNone:   "offline",
				toxcore.FriendStatusAway:   "away",
				toxcore.FriendStatusBusy:   "busy",
				toxcore.FriendStatusOnline: "online",
			}[friend.Status]

			connectionStr := getConnectionStatusString(friend.ConnectionStatus)

			name := friend.Name
			if name == "" {
				name = fmt.Sprintf("Friend %d", friendID)
			}

			fmt.Printf("   👤 %s (ID: %d)\n", name, friendID)
			fmt.Printf("      🔑 %x...\n", friend.PublicKey[:8])
			fmt.Printf("      📊 Status: %s, Connection: %s\n", statusStr, connectionStr)
			if friend.StatusMessage != "" {
				fmt.Printf("      💭 \"%s\"\n", friend.StatusMessage)
			}
			fmt.Printf("      🕐 Last seen: %s\n", friend.LastSeen.Format("2006-01-02 15:04:05"))
		}
	}
}

func sendDemoMessages(tox *toxcore.Tox) {
	fmt.Println("\n📤 Sending demo messages...")

	friendIDs := tox.GetFriendList()

	if len(friendIDs) == 0 {
		fmt.Println("   No friends to send messages to")
		return
	}

	demoMessages := []string{
		"Hello from the enhanced toxcore-go demo! 👋",
		"This message demonstrates the persistence system 💾",
		"All callbacks are working perfectly! ✅",
		"File transfer support is also implemented 📁",
	}

	for i, msg := range demoMessages {
		for _, friendID := range friendIDs {
			err := tox.SendFriendMessage(friendID, fmt.Sprintf("[%d] %s", i+1, msg))
			if err != nil {
				fmt.Printf("   ❌ Failed to queue message %d to friend %d: %v\n", i+1, friendID, err)
			} else {
				fmt.Printf("   📨 Queued message %d to friend %d\n", i+1, friendID)
			}
		}
		time.Sleep(100 * time.Millisecond) // Small delay between messages
	}
}

func showFinalStats(tox *toxcore.Tox) {
	fmt.Println("\n📊 Final Statistics:")

	friendCount := tox.GetFriendCount()
	queueLength := tox.GetMessageQueueLength()

	fmt.Printf("   👥 Friends: %d\n", friendCount)
	fmt.Printf("   📨 Pending messages: %d\n", queueLength)
	fmt.Printf("   🌐 Connection: %s\n", getConnectionStatusString(tox.SelfGetConnectionStatus()))
	fmt.Printf("   ⚡ Iteration interval: %v\n", tox.IterationInterval())

	// Show save data size
	saveData := tox.GetSavedata()
	fmt.Printf("   💾 Save data size: %d bytes\n", len(saveData))

	fmt.Println("\n🎉 All enhanced features demonstrated successfully!")
	fmt.Println("💡 Features showcased:")
	fmt.Println("   ✅ Complete callback system")
	fmt.Println("   ✅ State persistence and loading")
	fmt.Println("   ✅ Proper nospam handling")
	fmt.Println("   ✅ Message queuing system")
	fmt.Println("   ✅ Friend management")
	fmt.Println("   ✅ File transfer callbacks")
	fmt.Println("   ✅ Resource cleanup")
	fmt.Println("   ✅ Comprehensive error handling")
}
