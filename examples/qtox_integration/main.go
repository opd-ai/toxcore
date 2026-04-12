// Package main provides a complete example demonstrating toxcore-go integration
// with qTox, including bootstrap, friend requests, and message exchange.
package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opd-ai/toxcore"
)

// autoAcceptFriends controls whether friend requests are auto-accepted
const autoAcceptFriends = true

func main() {
	fmt.Println("=== toxcore-go qTox Integration Example ===")
	fmt.Println()

	// Create Tox instance with default options
	options := toxcore.NewOptions()
	options.BootstrapTimeout = 30 * time.Second

	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Display our Tox ID for sharing with qTox
	toxID := tox.SelfGetAddress()
	fmt.Printf("Your Tox ID: %s\n", toxID)
	fmt.Println("Share this with qTox to add you as a friend")
	fmt.Println()

	// Set up our identity
	name := "toxcore-go-qtox-example"
	if err := tox.SelfSetName(name); err != nil {
		log.Printf("Warning: Failed to set name: %v", err)
	}

	statusMsg := "Running toxcore-go qTox integration example"
	if err := tox.SelfSetStatusMessage(statusMsg); err != nil {
		log.Printf("Warning: Failed to set status message: %v", err)
	}

	fmt.Printf("Name: %s\n", name)
	fmt.Printf("Status: %s\n", statusMsg)
	fmt.Println()

	// Set up callbacks
	setupCallbacks(tox)

	// Bootstrap to the DHT network
	fmt.Println("Bootstrapping to DHT network...")
	bootstrapToDHT(tox)

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the main loop
	fmt.Println()
	fmt.Println("Running... Press Ctrl+C to exit")
	fmt.Println("Waiting for friend requests from qTox...")
	fmt.Println()

	run(tox, sigChan)

	fmt.Println("\nShutting down...")
}

// setupCallbacks registers all the necessary callbacks for Tox events
func setupCallbacks(tox *toxcore.Tox) {
	// Connection status - shows when we connect to the DHT
	tox.OnConnectionStatus(func(status toxcore.ConnectionStatus) {
		switch status {
		case toxcore.ConnectionNone:
			fmt.Println("[STATUS] Disconnected from DHT network")
		case toxcore.ConnectionTCP:
			fmt.Println("[STATUS] Connected via TCP relay")
		case toxcore.ConnectionUDP:
			fmt.Println("[STATUS] Connected via UDP (optimal)")
		}
	})

	// Friend request - handle incoming friend requests from qTox
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		pubKeyHex := hex.EncodeToString(publicKey[:])
		fmt.Printf("[REQUEST] Friend request from: %s...\n", pubKeyHex[:16])
		fmt.Printf("[REQUEST] Message: %s\n", message)

		if autoAcceptFriends {
			friendNum, err := tox.AddFriendByPublicKey(publicKey)
			if err != nil {
				fmt.Printf("[REQUEST] Failed to accept: %v\n", err)
				return
			}
			fmt.Printf("[REQUEST] Accepted! Friend number: %d\n", friendNum)
		} else {
			fmt.Println("[REQUEST] Auto-accept disabled. Call tox.AddFriendByPublicKey(publicKey) to accept.")
		}
	})

	// Friend connection status - shows when friends connect/disconnect
	tox.OnFriendConnectionStatus(func(friendID uint32, status toxcore.ConnectionStatus) {
		switch status {
		case toxcore.ConnectionNone:
			fmt.Printf("[FRIEND %d] Disconnected\n", friendID)
		case toxcore.ConnectionTCP:
			fmt.Printf("[FRIEND %d] Connected via TCP\n", friendID)
		case toxcore.ConnectionUDP:
			fmt.Printf("[FRIEND %d] Connected via UDP\n", friendID)
		}
	})

	// Friend message - handle incoming messages from qTox
	tox.OnFriendMessage(func(friendID uint32, message string) {
		fmt.Printf("[MESSAGE] From friend %d: %s\n", friendID, message)

		// Echo the message back
		echoMsg := fmt.Sprintf("Echo: %s", message)
		err := tox.SendFriendMessage(friendID, echoMsg, toxcore.MessageTypeNormal)
		if err != nil {
			fmt.Printf("[MESSAGE] Failed to send echo: %v\n", err)
		} else {
			fmt.Printf("[MESSAGE] Sent echo to friend %d\n", friendID)
		}
	})

	// Friend name change
	tox.OnFriendName(func(friendID uint32, name string) {
		fmt.Printf("[FRIEND %d] Changed name to: %s\n", friendID, name)
	})

	// Friend status message change
	tox.OnFriendStatusMessage(func(friendID uint32, statusMsg string) {
		fmt.Printf("[FRIEND %d] Status message: %s\n", friendID, statusMsg)
	})
}

// bootstrapToDHT connects to the DHT network using the default bootstrap nodes
func bootstrapToDHT(tox *toxcore.Tox) {
	if err := tox.BootstrapDefaults(); err != nil {
		fmt.Printf("Warning: Default bootstrap failed: %v. Will try LAN discovery.\n", err)
	} else {
		fmt.Println("Successfully bootstrapped to default nodes")
	}
}

// run executes the main Tox iteration loop until interrupted
func run(tox *toxcore.Tox, sigChan chan os.Signal) {
	statusTicker := time.NewTicker(30 * time.Second)
	defer statusTicker.Stop()

	bootstrapTicker := time.NewTicker(5 * time.Minute)
	defer bootstrapTicker.Stop()

	for {
		if shouldExit := processRunLoopEvent(tox, sigChan, statusTicker, bootstrapTicker); shouldExit {
			return
		}
	}
}

// processRunLoopEvent handles a single event in the run loop.
func processRunLoopEvent(tox *toxcore.Tox, sigChan chan os.Signal, statusTicker, bootstrapTicker *time.Ticker) bool {
	select {
	case <-sigChan:
		return true
	case <-statusTicker.C:
		printConnectionStatus(tox)
	case <-bootstrapTicker.C:
		handleRebootstrap(tox)
	default:
		tox.Iterate()
		time.Sleep(tox.IterationInterval())
	}
	return false
}

// printConnectionStatus displays the current connection status.
func printConnectionStatus(tox *toxcore.Tox) {
	status := tox.SelfGetConnectionStatus()
	friendCount := tox.GetFriendsCount()
	fmt.Printf("[INFO] Connection: %v, Friends: %d\n", connectionStatusString(status), friendCount)
}

// handleRebootstrap re-bootstraps to DHT if disconnected.
func handleRebootstrap(tox *toxcore.Tox) {
	if tox.SelfGetConnectionStatus() == toxcore.ConnectionNone {
		fmt.Println("[INFO] Reconnecting to DHT...")
		bootstrapToDHT(tox)
	}
}

// connectionStatusString returns a human-readable connection status
func connectionStatusString(status toxcore.ConnectionStatus) string {
	switch status {
	case toxcore.ConnectionNone:
		return "Disconnected"
	case toxcore.ConnectionTCP:
		return "TCP"
	case toxcore.ConnectionUDP:
		return "UDP"
	default:
		return "Unknown"
	}
}
