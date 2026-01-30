package main

import (
	"fmt"
	"time"

	"github.com/opd-ai/toxcore"
)

// Example demonstrating the new friend status callbacks:
// - OnFriendConnectionStatus: Called when friend connection status changes (None/UDP/TCP)
// - OnFriendStatusChange: Called when friend goes online or offline
func main() {
	// Create a Tox instance
	tox, err := toxcore.New(toxcore.NewOptions())
	if err != nil {
		panic(err)
	}
	defer tox.Kill()

	// Set up OnFriendConnectionStatus callback
	// This is triggered for all connection status changes
	tox.OnFriendConnectionStatus(func(friendID uint32, status toxcore.ConnectionStatus) {
		statusName := map[toxcore.ConnectionStatus]string{
			toxcore.ConnectionNone: "Offline",
			toxcore.ConnectionUDP:  "UDP",
			toxcore.ConnectionTCP:  "TCP",
		}
		fmt.Printf("Friend %d connection status changed to: %s\n", friendID, statusName[status])
	})

	// Set up OnFriendStatusChange callback
	// This is triggered only for online/offline transitions
	tox.OnFriendStatusChange(func(friendPK [32]byte, online bool) {
		if online {
			fmt.Printf("Friend %x is now ONLINE\n", friendPK[:8])
		} else {
			fmt.Printf("Friend %x is now OFFLINE\n", friendPK[:8])
		}
	})

	// Add a friend for demonstration
	testPK := [32]byte{1, 2, 3, 4, 5, 6, 7, 8}
	friendID, err := tox.AddFriendByPublicKey(testPK)
	if err != nil {
		panic(err)
	}

	fmt.Println("Demonstrating callback behavior:")
	fmt.Println("=================================")

	// Scenario 1: Friend comes online via UDP
	fmt.Println("\n1. Friend comes online (UDP):")
	tox.SetFriendConnectionStatus(friendID, toxcore.ConnectionUDP)
	time.Sleep(100 * time.Millisecond)
	// Output:
	// - OnFriendConnectionStatus: UDP
	// - OnFriendStatusChange: online=true

	// Scenario 2: Friend switches from UDP to TCP (stays online)
	fmt.Println("\n2. Friend switches to TCP (still online):")
	tox.SetFriendConnectionStatus(friendID, toxcore.ConnectionTCP)
	time.Sleep(100 * time.Millisecond)
	// Output:
	// - OnFriendConnectionStatus: TCP
	// - OnFriendStatusChange: NOT called (still online)

	// Scenario 3: Friend goes offline
	fmt.Println("\n3. Friend goes offline:")
	tox.SetFriendConnectionStatus(friendID, toxcore.ConnectionNone)
	time.Sleep(100 * time.Millisecond)
	// Output:
	// - OnFriendConnectionStatus: Offline
	// - OnFriendStatusChange: online=false

	fmt.Println("\n=================================")
	fmt.Println("Demonstration complete!")
}
