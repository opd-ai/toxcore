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

	tox := createPrimaryToxInstance()
	defer tox.Kill()

	displayInitialFriendStats(tox)

	tox2 := createSecondaryToxInstance()
	defer tox2.Kill()

	demonstrateFriendLookup(tox, tox2)
}

func createPrimaryToxInstance() *toxcore.Tox {
	fmt.Println("\n1. Creating new Tox instance:")
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatalf("Failed to create Tox instance: %v", err)
	}
	fmt.Printf("✅ Tox instance created: %s\n", tox.SelfGetAddress()[:16]+"...")
	return tox
}

func displayInitialFriendStats(tox *toxcore.Tox) {
	fmt.Println("\n2. Testing friend management operations:")
	friendsCount := tox.GetFriendsCount()
	fmt.Printf("✅ Initial friends count: %d\n", friendsCount)
	friends := tox.GetFriends()
	fmt.Printf("✅ Retrieved friends list: %d friends\n", len(friends))
}

func createSecondaryToxInstance() *toxcore.Tox {
	fmt.Println("\n3. Creating second Tox instance for friend testing:")
	tox2, err := toxcore.New(nil)
	if err != nil {
		log.Fatalf("Failed to create second Tox instance: %v", err)
	}
	fmt.Printf("✅ Second Tox instance created: %s\n", tox2.SelfGetAddress()[:16]+"...")
	return tox2
}

func demonstrateFriendLookup(tox, tox2 *toxcore.Tox) {
	fmt.Println("\n4. Testing friend lookup operations:")
	tox2PublicKey := tox2.SelfGetPublicKey()

	testUnknownFriendLookup(tox, tox2PublicKey)
	friendID := addFriendByPublicKey(tox, tox2PublicKey)
	verifyFriendLookup(tox, tox2PublicKey, friendID)
	displayUpdatedFriendStats(tox)
}

func testUnknownFriendLookup(tox *toxcore.Tox, publicKey [32]byte) {
	_, err := tox.GetFriendByPublicKey(publicKey)
	if err != nil {
		fmt.Printf("✅ Expected error for unknown friend: %v\n", err)
	}
}

func addFriendByPublicKey(tox *toxcore.Tox, publicKey [32]byte) uint32 {
	friendID, err := tox.AddFriendByPublicKey(publicKey)
	if err != nil {
		log.Fatalf("Failed to add friend: %v", err)
	}
	fmt.Printf("✅ Friend added with ID: %d\n", friendID)
	return friendID
}

func verifyFriendLookup(tox *toxcore.Tox, publicKey [32]byte, expectedFriendID uint32) {
	retrievedFriendID, err := tox.GetFriendByPublicKey(publicKey)
	if err != nil {
		log.Fatalf("Failed to get friend by public key: %v", err)
	}
	fmt.Printf("✅ Friend retrieved by public key: ID %d\n", retrievedFriendID)

	retrievedPublicKey, err := tox.GetFriendPublicKey(expectedFriendID)
	if err != nil {
		log.Fatalf("Failed to get friend's public key: %v", err)
	}
	fmt.Printf("✅ Friend's public key retrieved: %x...\n", retrievedPublicKey[:8])
}

func displayUpdatedFriendStats(tox *toxcore.Tox) {
	newFriendsCount := tox.GetFriendsCount()
	fmt.Printf("✅ Updated friends count: %d\n", newFriendsCount)
	updatedFriends := tox.GetFriends()
	fmt.Printf("✅ Updated friends list: %d friends\n", len(updatedFriends))
}
