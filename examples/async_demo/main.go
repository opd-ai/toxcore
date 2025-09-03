// Package main demonstrates the async message delivery system
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/opd-ai/toxcore/async"
	"github.com/opd-ai/toxcore/crypto"
)

func main() {
	fmt.Println("=== Tox Async Message Delivery System Demo ===")
	fmt.Println()

	// Create key pairs for our demo
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate Alice's key pair: %v", err)
	}

	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate Bob's key pair: %v", err)
	}

	storageNodeKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate storage node key pair: %v", err)
	}

	fmt.Printf("Alice's Public Key: %x\n", aliceKeyPair.Public[:8])
	fmt.Printf("Bob's Public Key: %x\n", bobKeyPair.Public[:8])
	fmt.Printf("Storage Node Public Key: %x\n", storageNodeKeyPair.Public[:8])
	fmt.Println()

	// Demo 1: Direct storage and retrieval
	fmt.Println("=== Demo 1: Direct Message Storage ===")
	demoDirectStorage(aliceKeyPair, bobKeyPair, storageNodeKeyPair)
	fmt.Println()

	// Demo 2: Async manager functionality
	fmt.Println("=== Demo 2: Async Manager ===")
	demoAsyncManager(aliceKeyPair, bobKeyPair)
	fmt.Println()

	// Demo 3: Storage node maintenance
	fmt.Println("=== Demo 3: Storage Node Maintenance ===")
	demoStorageMaintenance(storageNodeKeyPair)
}

func demoDirectStorage(aliceKeyPair, bobKeyPair, storageNodeKeyPair *crypto.KeyPair) {
	// Create a storage node
	storage := async.NewMessageStorage(storageNodeKeyPair, "/tmp")

	fmt.Printf("📦 Created storage node with capacity for %d messages\n", storage.GetMaxCapacity())
	fmt.Printf("💾 Storage utilization: %.1f%%\n", storage.GetStorageUtilization())

	// Alice sends an async message for Bob (who is offline)
	message := "Hello Bob! This is an async message from Alice. 👋"
	fmt.Println("💾 Alice preparing encrypted message for Bob...")

	// Encrypt the message for Bob using Alice's private key and Bob's public key
	encryptedData, nonce, err := async.EncryptForRecipient([]byte(message), bobKeyPair.Public, aliceKeyPair.Private)
	if err != nil {
		fmt.Printf("❌ Failed to encrypt message: %v\n", err)
		return
	}

	// Store the encrypted message
	messageID, err := storage.StoreMessage(bobKeyPair.Public, aliceKeyPair.Public, encryptedData, nonce, async.MessageTypeNormal)
	if err != nil {
		fmt.Printf("❌ Failed to store message: %v\n", err)
		return
	}

	fmt.Printf("💾 Alice stored message for Bob\n")
	fmt.Printf("   Message ID: %x\n", messageID[:8])
	fmt.Printf("   Content: %s\n", message)

	// Check storage stats
	stats := storage.GetStorageStats()
	fmt.Printf("📊 Storage Stats: %d messages, %d recipients\n",
		stats.TotalMessages, stats.UniqueRecipients)

	// Bob comes online and retrieves his messages
	fmt.Println("\n🔍 Bob comes online and retrieves messages...")
	messages, err := storage.RetrieveMessages(bobKeyPair.Public)
	if err != nil {
		log.Printf("Failed to retrieve messages: %v", err)
		return
	}

	fmt.Printf("📨 Bob retrieved %d message(s):\n", len(messages))
	for i, msg := range messages {
		fmt.Printf("   %d. From: %x\n", i+1, msg.SenderPK[:8])
		fmt.Printf("      Stored: %s\n", msg.Timestamp.Format(time.RFC3339))
		fmt.Printf("      Type: %v\n", msg.MessageType)

		// Decrypt the message (Bob would do this with his private key)
		decrypted, err := crypto.Decrypt(msg.EncryptedData, msg.Nonce,
			msg.SenderPK, bobKeyPair.Private)
		if err != nil {
			fmt.Printf("      ❌ Failed to decrypt: %v\n", err)
		} else {
			fmt.Printf("      📝 Content: %s\n", string(decrypted))
		}
	}

	// Bob deletes the message after reading
	if len(messages) > 0 {
		err = storage.DeleteMessage(messages[0].ID, bobKeyPair.Public)
		if err != nil {
			log.Printf("Failed to delete message: %v", err)
		} else {
			fmt.Println("🗑️  Bob deleted the message after reading")
		}
	}
}

func demoAsyncManager(aliceKeyPair, bobKeyPair *crypto.KeyPair) {
	// Alice creates an async manager (acts as both client and storage node)
	aliceManager, err := async.NewAsyncManager(aliceKeyPair, "/tmp/alice")
	if err != nil {
		log.Fatalf("Failed to create Alice's async manager: %v", err)
	}
	bobManager, err := async.NewAsyncManager(bobKeyPair, "/tmp/bob") // Bob is just a client
	if err != nil {
		log.Fatalf("Failed to create Bob's async manager: %v", err)
	}

	// Set up message handlers
	bobReceivedMessages := make([]string, 0)
	bobManager.SetAsyncMessageHandler(func(senderPK [32]byte, message string,
		messageType async.MessageType) {
		fmt.Printf("📨 Bob received async message from %x: %s\n", senderPK[:8], message)
		bobReceivedMessages = append(bobReceivedMessages, message)
	})

	// Start the managers
	aliceManager.Start()
	bobManager.Start()
	defer aliceManager.Stop()
	defer bobManager.Stop()

	fmt.Println("🚀 Started async managers for Alice and Bob")

	// Alice adds her node as a storage node for Bob
	aliceAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
	bobManager.AddStorageNode(aliceKeyPair.Public, aliceAddr)

	// Set Bob as offline initially
	aliceManager.SetFriendOnlineStatus(bobKeyPair.Public, false)
	fmt.Println("📴 Bob is offline")

	// Alice tries to send a message to offline Bob
	message := "Hey Bob, this message will be stored for when you come back online!"
	err = aliceManager.SendAsyncMessage(bobKeyPair.Public, message, async.MessageTypeNormal)
	if err != nil {
		log.Printf("Failed to send async message: %v", err)
	} else {
		fmt.Printf("📤 Alice sent async message to offline Bob\n")
	}

	// Show Alice's storage stats
	if stats := aliceManager.GetStorageStats(); stats != nil {
		fmt.Printf("📊 Alice's storage stats: %d messages stored\n", stats.TotalMessages)
	}

	// Simulate Bob coming online
	time.Sleep(100 * time.Millisecond) // Give time for async operations
	fmt.Println("\n🟢 Bob comes online!")
	aliceManager.SetFriendOnlineStatus(bobKeyPair.Public, true)

	// Give time for message delivery
	time.Sleep(200 * time.Millisecond)

	if len(bobReceivedMessages) > 0 {
		fmt.Printf("✅ Success! Bob received %d async message(s)\n", len(bobReceivedMessages))
	} else {
		fmt.Println("ℹ️  Note: In this demo, actual network delivery is simulated")
		fmt.Println("   In a real implementation, messages would be delivered over the network")
	}
}

func demoStorageMaintenance(storageNodeKeyPair *crypto.KeyPair) {
	storage := async.NewMessageStorage(storageNodeKeyPair, "/tmp")

	// Create some test key pairs
	user1, _ := crypto.GenerateKeyPair()
	user2, _ := crypto.GenerateKeyPair()
	sender, _ := crypto.GenerateKeyPair()

	// Store some messages
	messages := []string{
		"Message 1 for user1",
		"Message 2 for user1",
		"Message 1 for user2",
		"Another message for user2",
	}

	for i, msg := range messages {
		recipient := user1.Public
		if i >= 2 {
			recipient = user2.Public
		}

		encryptedData, nonce, err := async.EncryptForRecipient([]byte(msg), recipient, sender.Private)
		if err != nil {
			log.Printf("Failed to encrypt message: %v", err)
			continue
		}

		_, err = storage.StoreMessage(recipient, sender.Public, encryptedData, nonce, async.MessageTypeNormal)
		if err != nil {
			log.Printf("Failed to store message: %v", err)
		}
	}

	fmt.Printf("📦 Stored %d messages for testing\n", len(messages))

	// Show initial stats
	stats := storage.GetStorageStats()
	fmt.Printf("📊 Initial stats: %d messages, %d recipients\n",
		stats.TotalMessages, stats.UniqueRecipients)

	// Simulate message expiration by creating a new storage instance
	// and demonstrating the cleanup functionality
	fmt.Println("🕐 Simulating expired message cleanup...")

	// For demo purposes, store a few more messages and then run cleanup
	// In reality, cleanup would remove messages older than 24 hours
	for i := 0; i < 2; i++ {
		msg := fmt.Sprintf("Additional message %d", i+1)
		encryptedData, nonce, err := async.EncryptForRecipient([]byte(msg), user1.Public, sender.Private)
		if err != nil {
			log.Printf("Failed to encrypt additional message: %v", err)
			continue
		}
		storage.StoreMessage(user1.Public, sender.Public, encryptedData, nonce, async.MessageTypeNormal)
	}

	fmt.Printf("� Added more messages for cleanup demo\n")

	// Run cleanup (in a real scenario, this would remove messages older than 24 hours)
	expiredCount := storage.CleanupExpiredMessages()
	fmt.Printf("🧹 Cleanup process ran (removed %d expired messages)\n", expiredCount)
	fmt.Println("   Note: No messages were actually expired in this demo")
	fmt.Println("   In production, messages older than 24 hours would be removed")

	// Show final stats
	finalStats := storage.GetStorageStats()
	fmt.Printf("📊 Final stats: %d messages, %d recipients\n",
		finalStats.TotalMessages, finalStats.UniqueRecipients)

	fmt.Println("✅ Storage maintenance demo complete")
}
