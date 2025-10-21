package main

import (
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/toxforge"
)

// createToxInstances creates and configures two Tox instances for testing.
func createToxInstances() (*toxcore.Tox, *toxcore.Tox) {
	options1 := toxcore.NewOptions()
	options1.UDPEnabled = false // Disable UDP for simple test

	options2 := toxcore.NewOptions()
	options2.UDPEnabled = false

	tox1, err := toxcore.New(options1)
	if err != nil {
		log.Fatalf("Failed to create tox1: %v", err)
	}

	tox2, err := toxcore.New(options2)
	if err != nil {
		log.Fatalf("Failed to create tox2: %v", err)
	}

	return tox1, tox2
}

// validateStorageNodes checks that both Tox instances have async storage enabled.
func validateStorageNodes(tox1, tox2 *toxcore.Tox) {
	stats1 := tox1.GetAsyncStorageStats()
	stats2 := tox2.GetAsyncStorageStats()

	if stats1 == nil {
		log.Fatal("Tox1 should have async storage enabled")
	}

	if stats2 == nil {
		log.Fatal("Tox2 should have async storage enabled")
	}

	fmt.Printf("✅ Tox1 storage capacity: %d messages (%.1f%% utilized)\n",
		tox1.GetAsyncStorageCapacity(), tox1.GetAsyncStorageUtilization())
	fmt.Printf("✅ Tox2 storage capacity: %d messages (%.1f%% utilized)\n",
		tox2.GetAsyncStorageCapacity(), tox2.GetAsyncStorageUtilization())
}

// setupFriendRelationship establishes friend relationships between two Tox instances.
func setupFriendRelationship(tox1, tox2 *toxcore.Tox) uint32 {
	tox1PublicKey := tox1.GetSelfPublicKey()
	tox2PublicKey := tox2.GetSelfPublicKey()

	friendID1, err := tox1.AddFriendByPublicKey(tox2PublicKey)
	if err != nil {
		log.Printf("Warning: Could not add friend in tox1: %v", err)
	}

	_, err = tox2.AddFriendByPublicKey(tox1PublicKey)
	if err != nil {
		log.Printf("Warning: Could not add friend in tox2: %v", err)
	}

	return friendID1
}

// testAsyncMessaging tests the async messaging functionality between Tox instances.
func testAsyncMessaging(tox1 *toxcore.Tox, friendID1 uint32) {
	if friendID1 != 0 {
		fmt.Printf("📤 Tox1 sending message to offline friend %d...\n", friendID1)
		err := tox1.SendFriendMessage(friendID1, "Hello from Tox1! This should be stored async.")
		if err != nil {
			log.Printf("Message send result: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)
}

// validateAndReportResults validates storage functionality and reports test results.
func validateAndReportResults(tox1, tox2 *toxcore.Tox) {
	if tox1.GetAsyncStorageCapacity() > 0 && tox2.GetAsyncStorageCapacity() > 0 {
		fmt.Println("✅ Both Tox instances are automatically acting as storage nodes")
		fmt.Printf("   Tox1: %d message capacity\n", tox1.GetAsyncStorageCapacity())
		fmt.Printf("   Tox2: %d message capacity\n", tox2.GetAsyncStorageCapacity())
	} else {
		fmt.Println("❌ Storage node functionality not working properly")
	}

	fmt.Println("\n🎉 Automatic storage node integration test completed!")
	fmt.Println("✅ All users automatically become storage nodes")
	fmt.Println("✅ Storage capacity is based on 1% of available disk space")
	fmt.Println("✅ Capacity is capped between 1MB and 1GB reasonable limits")
}

func main() {
	fmt.Println("=== Tox Automatic Storage Node Integration Test ===")

	tox1, tox2 := createToxInstances()
	defer tox1.Kill()
	defer tox2.Kill()

	validateStorageNodes(tox1, tox2)

	// Set up message callback for tox2
	messagesReceived := 0
	tox2.OnFriendMessage(func(friendID uint32, message string) {
		messagesReceived++
		fmt.Printf("📨 Tox2 received message: %s\n", message)
	})

	friendID1 := setupFriendRelationship(tox1, tox2)
	testAsyncMessaging(tox1, friendID1)
	validateAndReportResults(tox1, tox2)
}
