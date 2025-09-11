package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/crypto"
	"github.com/sirupsen/logrus"
)

func init() {
	// Configure logrus for structured logging demonstration
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		PrettyPrint:     true,
	})
}

func main() {
	fmt.Println("=== Toxcore Enhanced Logging Infrastructure Demonstration ===")
	fmt.Println()

	// Demonstrate enhanced crypto logging
	demonstrateCryptoLogging()

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// Demonstrate enhanced toxcore logging
	demonstrateToxcoreLogging()

	fmt.Println()
	fmt.Println("=== Logging Enhancement Demonstration Complete ===")
}

func demonstrateCryptoLogging() {
	fmt.Println("🔐 Demonstrating Enhanced Crypto Module Logging")
	fmt.Println("------------------------------------------------")

	// Generate cryptographic nonce with enhanced logging
	fmt.Println("\n1. Generating cryptographically secure nonce:")
	nonce, err := crypto.GenerateNonce()
	if err != nil {
		log.Fatalf("Failed to generate nonce: %v", err)
	}
	fmt.Printf("✅ Nonce generated: %x...\n", nonce[:4])

	// Generate key pair with enhanced logging
	fmt.Println("\n2. Generating cryptographic key pair:")
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate key pair: %v", err)
	}
	fmt.Printf("✅ Key pair generated: public key %x...\n", keyPair.Public[:8])

	// Demonstrate key derivation with enhanced logging
	fmt.Println("\n3. Deriving key pair from secret key:")
	derivedKeyPair, err := crypto.FromSecretKey(keyPair.Private)
	if err != nil {
		log.Fatalf("Failed to derive key pair: %v", err)
	}
	fmt.Printf("✅ Key pair derived: public key %x...\n", derivedKeyPair.Public[:8])

	// Demonstrate encryption with enhanced logging
	fmt.Println("\n4. Performing authenticated encryption:")
	testMessage := "Hello, enhanced logging world! 🚀"
	encrypted, err := crypto.Encrypt([]byte(testMessage), nonce, derivedKeyPair.Public, keyPair.Private)
	if err != nil {
		log.Fatalf("Failed to encrypt message: %v", err)
	}
	fmt.Printf("✅ Message encrypted: %d bytes -> %d bytes\n", len(testMessage), len(encrypted))

	// Demonstrate symmetric encryption with enhanced logging
	fmt.Println("\n5. Performing symmetric authenticated encryption:")
	var symmetricKey [32]byte
	copy(symmetricKey[:], keyPair.Private[:])
	symmetricEncrypted, err := crypto.EncryptSymmetric([]byte(testMessage), nonce, symmetricKey)
	if err != nil {
		log.Fatalf("Failed to encrypt message symmetrically: %v", err)
	}
	fmt.Printf("✅ Message encrypted symmetrically: %d bytes -> %d bytes\n", len(testMessage), len(symmetricEncrypted))
}

func demonstrateToxcoreLogging() {
	fmt.Println("🌐 Demonstrating Enhanced Toxcore Logging")
	fmt.Println("------------------------------------------")

	// Create new Tox instance with enhanced logging
	fmt.Println("\n1. Creating new Tox instance:")
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()
	fmt.Printf("✅ Tox instance created: %s\n", tox.SelfGetAddress()[:16]+"...")

	// Demonstrate friend management with enhanced logging
	fmt.Println("\n2. Testing friend management operations:")

	// Test friend count (initially empty)
	friendsCount := tox.GetFriendsCount()
	fmt.Printf("✅ Initial friends count: %d\n", friendsCount)

	// Get friends list (should be empty)
	friends := tox.GetFriends()
	fmt.Printf("✅ Retrieved friends list: %d friends\n", len(friends))

	// Create a second Tox instance for friend operations
	fmt.Println("\n3. Creating second Tox instance for friend testing:")
	tox2, err := toxcore.New(nil)
	if err != nil {
		log.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()
	fmt.Printf("✅ Second Tox instance created: %s\n", tox2.SelfGetAddress()[:16]+"...")

	// Add friend by public key with enhanced logging
	fmt.Println("\n4. Testing friend lookup operations:")
	tox2PublicKey := tox2.SelfGetPublicKey()

	// Test friend lookup (should fail initially)
	_, err = tox.GetFriendByPublicKey(tox2PublicKey)
	if err != nil {
		fmt.Printf("✅ Expected error for unknown friend: %v\n", err)
	}

	// Add friend and test again
	friendID, err := tox.AddFriendByPublicKey(tox2PublicKey)
	if err != nil {
		log.Fatalf("Failed to add friend: %v", err)
	}
	fmt.Printf("✅ Friend added with ID: %d\n", friendID)

	// Test friend lookup (should succeed now)
	retrievedFriendID, err := tox.GetFriendByPublicKey(tox2PublicKey)
	if err != nil {
		log.Fatalf("Failed to get friend by public key: %v", err)
	}
	fmt.Printf("✅ Friend retrieved by public key: ID %d\n", retrievedFriendID)

	// Test getting friend's public key
	retrievedPublicKey, err := tox.GetFriendPublicKey(friendID)
	if err != nil {
		log.Fatalf("Failed to get friend's public key: %v", err)
	}
	fmt.Printf("✅ Friend's public key retrieved: %x...\n", retrievedPublicKey[:8])

	// Test updated friends count
	newFriendsCount := tox.GetFriendsCount()
	fmt.Printf("✅ Updated friends count: %d\n", newFriendsCount)

	// Get updated friends list
	updatedFriends := tox.GetFriends()
	fmt.Printf("✅ Updated friends list: %d friends\n", len(updatedFriends))
}
