// ToxAV Integration Example
//
// This example demonstrates how to integrate ToxAV with existing Tox functionality
// including friend management, messaging, and call coordination. It shows a complete
// Tox client with both messaging and audio/video calling capabilities.

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/av"
)

const (
	// Audio/Video configuration
	defaultAudioBitRate = 64000  // 64 kbps
	defaultVideoBitRate = 500000 // 500 kbps
	saveDataFile        = "toxav_integration_profile.dat"
)

// ToxAVClient represents a complete Tox client with AV capabilities
type ToxAVClient struct {
	tox   *toxcore.Tox
	toxav *toxcore.ToxAV
	mu    sync.RWMutex

	// State management
	running     bool
	activeCalls map[uint32]*CallSession
	friends     map[uint32]*FriendInfo

	// Statistics
	messagesSent     uint64
	messagesReceived uint64
	callsInitiated   uint64
	callsReceived    uint64
}

// CallSession tracks an active audio/video call
type CallSession struct {
	FriendNumber uint32
	AudioEnabled bool
	VideoEnabled bool
	StartTime    time.Time
	FramesSent   uint64
	FramesRecv   uint64
	mu           sync.RWMutex
}

// FriendInfo stores information about a friend
type FriendInfo struct {
	Number    uint32
	Name      string
	Status    string
	LastSeen  time.Time
	PublicKey [32]byte
}

// NewToxAVClient creates a new integrated Tox+ToxAV client
func NewToxAVClient() (*ToxAVClient, error) {
	fmt.Println("🚀 ToxAV Integration Demo - Initializing client...")

	// Try to load existing profile
	var tox *toxcore.Tox
	var err error

	if savedata, readErr := os.ReadFile(saveDataFile); readErr == nil {
		fmt.Printf("📁 Loading existing profile (%d bytes)\n", len(savedata))
		options := &toxcore.Options{
			UDPEnabled:     true,
			SavedataLength: uint32(len(savedata)),
		}
		tox, err = toxcore.New(options)
		if err != nil {
			fmt.Printf("⚠️  Failed to load profile, creating new one: %v\n", err)
			tox, err = createNewProfile()
		} else {
			fmt.Println("✅ Profile loaded successfully")
		}
	} else {
		fmt.Println("📝 Creating new profile...")
		tox, err = createNewProfile()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Tox instance: %w", err)
	}

	// Create ToxAV instance
	toxav, err := toxcore.NewToxAV(tox)
	if err != nil {
		tox.Kill()
		return nil, fmt.Errorf("failed to create ToxAV instance: %w", err)
	}

	client := &ToxAVClient{
		tox:         tox,
		toxav:       toxav,
		running:     true,
		activeCalls: make(map[uint32]*CallSession),
		friends:     make(map[uint32]*FriendInfo),
	}

	// Load existing friends
	client.loadFriends()

	// Set up all callbacks
	client.setupCallbacks()

	fmt.Printf("✅ Tox ID: %s\n", tox.SelfGetAddress())
	fmt.Printf("👤 Name: %s\n", tox.SelfGetName())
	fmt.Printf("💬 Status: %s\n", tox.SelfGetStatusMessage())
	fmt.Printf("👥 Friends: %d\n", len(client.friends))

	return client, nil
}

// createNewProfile creates a new Tox profile
func createNewProfile() (*toxcore.Tox, error) {
	options := toxcore.NewOptions()
	options.UDPEnabled = true

	tox, err := toxcore.New(options)
	if err != nil {
		return nil, err
	}

	// Set up profile
	if err := tox.SelfSetName("ToxAV Integration Demo"); err != nil {
		log.Printf("Warning: Failed to set name: %v", err)
	}

	if err := tox.SelfSetStatusMessage("Integrated Tox client with AV calling"); err != nil {
		log.Printf("Warning: Failed to set status: %v", err)
	}

	return tox, nil
}

// loadFriends loads existing friends into the client
func (c *ToxAVClient) loadFriends() {
	friends := c.tox.GetFriends()
	for friendNumber, friend := range friends {
		publicKey, err := c.tox.GetFriendPublicKey(friendNumber)
		if err != nil {
			log.Printf("Warning: Failed to get public key for friend %d: %v", friendNumber, err)
			continue
		}

		c.friends[friendNumber] = &FriendInfo{
			Number:    friendNumber,
			Name:      friend.Name,
			Status:    "",
			LastSeen:  time.Now(),
			PublicKey: publicKey,
		}
	}
}

// setupCallbacks configures all Tox and ToxAV callbacks
func (c *ToxAVClient) setupCallbacks() {
	// === Tox Messaging Callbacks ===

	// Friend requests
	c.tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		fmt.Printf("\n📞 Friend request received: %s\n", message)
		fmt.Printf("Public key: %x\n", publicKey)
		fmt.Print("Accept? (y/n): ")
		// In a real app, you'd prompt the user or have auto-accept logic
	})

	// Friend messages
	c.tox.OnFriendMessage(func(friendNumber uint32, message string) {
		c.messagesReceived++

		friendName := "Unknown"
		if friend, exists := c.friends[friendNumber]; exists {
			friendName = friend.Name
			friend.LastSeen = time.Now()
		}

		fmt.Printf("\n💬 Message from %s (%d): %s\n", friendName, friendNumber, message)

		// Auto-respond to special commands
		c.handleMessageCommand(friendNumber, message)
		fmt.Print("> ")
	})

	// === ToxAV Callbacks ===

	// Incoming calls
	c.toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
		c.callsReceived++

		friendName := "Unknown"
		if friend, exists := c.friends[friendNumber]; exists {
			friendName = friend.Name
		}

		fmt.Printf("\n📞 Incoming call from %s (%d) - Audio: %v, Video: %v\n",
			friendName, friendNumber, audioEnabled, videoEnabled)

		// Create call session
		session := &CallSession{
			FriendNumber: friendNumber,
			AudioEnabled: audioEnabled,
			VideoEnabled: videoEnabled,
			StartTime:    time.Now(),
		}

		c.mu.Lock()
		c.activeCalls[friendNumber] = session
		c.mu.Unlock()

		// Auto-answer for demo
		audioBR := uint32(0)
		videoBR := uint32(0)
		if audioEnabled {
			audioBR = defaultAudioBitRate
		}
		if videoEnabled {
			videoBR = defaultVideoBitRate
		}

		if err := c.toxav.Answer(friendNumber, audioBR, videoBR); err != nil {
			fmt.Printf("❌ Failed to answer call: %v\n", err)
		} else {
			fmt.Printf("✅ Call answered\n")
		}
		fmt.Print("> ")
	})

	// Call state changes
	c.toxav.CallbackCallState(func(friendNumber uint32, state av.CallState) {
		friendName := "Unknown"
		if friend, exists := c.friends[friendNumber]; exists {
			friendName = friend.Name
		}

		fmt.Printf("\n📡 Call state with %s (%d): %d\n", friendName, friendNumber, uint32(state))

		if state == av.CallStateFinished {
			c.mu.Lock()
			if session, exists := c.activeCalls[friendNumber]; exists {
				duration := time.Since(session.StartTime)
				fmt.Printf("📞 Call ended after %v\n", duration.Round(time.Second))
				delete(c.activeCalls, friendNumber)
			}
			c.mu.Unlock()
		}
		fmt.Print("> ")
	})

	// Audio frames received
	c.toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		c.mu.Lock()
		if session, exists := c.activeCalls[friendNumber]; exists {
			session.mu.Lock()
			session.FramesRecv++
			session.mu.Unlock()
		}
		c.mu.Unlock()

		// Optional: log audio reception
		// fmt.Printf("🔊 Audio frame: %d samples @ %dHz\n", sampleCount, samplingRate)
	})

	// Video frames received
	c.toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		c.mu.Lock()
		if session, exists := c.activeCalls[friendNumber]; exists {
			session.mu.Lock()
			session.FramesRecv++
			session.mu.Unlock()
		}
		c.mu.Unlock()

		// Optional: log video reception
		// fmt.Printf("📹 Video frame: %dx%d\n", width, height)
	})
}

// handleMessageCommand processes special message commands
func (c *ToxAVClient) handleMessageCommand(friendNumber uint32, message string) {
	lower := strings.ToLower(strings.TrimSpace(message))

	switch {
	case lower == "call":
		c.initiateCall(friendNumber, true, false) // Audio only
	case lower == "videocall":
		c.initiateCall(friendNumber, true, true) // Audio + Video
	case lower == "status":
		c.sendStatus(friendNumber)
	case lower == "help":
		c.sendHelp(friendNumber)
	case strings.HasPrefix(lower, "echo "):
		response := "Echo: " + message[5:]
		c.sendMessage(friendNumber, response)
	}
}

// sendMessage sends a message to a friend
func (c *ToxAVClient) sendMessage(friendNumber uint32, message string) {
	if err := c.tox.SendFriendMessage(friendNumber, message); err != nil {
		fmt.Printf("❌ Failed to send message: %v\n", err)
	} else {
		c.messagesReceived++
		fmt.Printf("📤 Sent: %s\n", message)
	}
}

// sendStatus sends current status to a friend
func (c *ToxAVClient) sendStatus(friendNumber uint32) {
	c.mu.RLock()
	activeCalls := len(c.activeCalls)
	totalFriends := len(c.friends)
	c.mu.RUnlock()

	status := fmt.Sprintf("Status: %d friends, %d active calls, messages sent/received: %d/%d",
		totalFriends, activeCalls, c.messagesSent, c.messagesReceived)

	c.sendMessage(friendNumber, status)
}

// sendHelp sends help information to a friend
func (c *ToxAVClient) sendHelp(friendNumber uint32) {
	help := "Commands: 'call' (audio), 'videocall' (audio+video), 'status', 'help', 'echo <text>'"
	c.sendMessage(friendNumber, help)
}

// initiateCall starts a call with a friend
func (c *ToxAVClient) initiateCall(friendNumber uint32, audio, video bool) {
	c.mu.Lock()
	if _, exists := c.activeCalls[friendNumber]; exists {
		c.mu.Unlock()
		fmt.Printf("❌ Call already active with friend %d\n", friendNumber)
		return
	}
	c.mu.Unlock()

	audioBR := uint32(0)
	videoBR := uint32(0)
	if audio {
		audioBR = defaultAudioBitRate
	}
	if video {
		videoBR = defaultVideoBitRate
	}

	if err := c.toxav.Call(friendNumber, audioBR, videoBR); err != nil {
		fmt.Printf("❌ Failed to initiate call: %v\n", err)
	} else {
		c.callsInitiated++

		session := &CallSession{
			FriendNumber: friendNumber,
			AudioEnabled: audio,
			VideoEnabled: video,
			StartTime:    time.Now(),
		}

		c.mu.Lock()
		c.activeCalls[friendNumber] = session
		c.mu.Unlock()

		friendName := "Unknown"
		if friend, exists := c.friends[friendNumber]; exists {
			friendName = friend.Name
		}

		fmt.Printf("📞 Calling %s (%d) - Audio: %v, Video: %v\n",
			friendName, friendNumber, audio, video)
	}
}

// processCommand handles user input commands
func (c *ToxAVClient) processCommand(command string) {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return
	}

	switch strings.ToLower(parts[0]) {
	case "help", "h":
		c.showHelp()
	case "friends", "f":
		c.showFriends()
	case "calls", "c":
		c.showActiveCalls()
	case "stats", "s":
		c.showStats()
	case "add":
		c.handleAddFriendCommand(parts)
	case "msg", "m":
		c.handleSendMessageCommand(parts)
	case "call":
		c.handleCallCommand(parts)
	case "videocall", "vcall":
		c.handleVideoCallCommand(parts)
	case "hangup", "end":
		c.handleHangupCommand(parts)
	case "save":
		c.saveProfile()
	case "quit", "exit", "q":
		c.running = false
	default:
		fmt.Printf("Unknown command: %s (type 'help' for commands)\n", parts[0])
	}
}

// handleAddFriendCommand processes the 'add' command to add a friend
func (c *ToxAVClient) handleAddFriendCommand(parts []string) {
	if len(parts) >= 2 {
		c.addFriend(parts[1], strings.Join(parts[2:], " "))
	} else {
		fmt.Println("Usage: add <tox_id> [message]")
	}
}

// handleSendMessageCommand processes the 'msg' command to send a message
func (c *ToxAVClient) handleSendMessageCommand(parts []string) {
	if len(parts) >= 3 {
		if friendNum, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
			message := strings.Join(parts[2:], " ")
			c.sendMessage(uint32(friendNum), message)
		} else {
			fmt.Println("Invalid friend number")
		}
	} else {
		fmt.Println("Usage: msg <friend_number> <message>")
	}
}

// handleCallCommand processes the 'call' command to initiate an audio call
func (c *ToxAVClient) handleCallCommand(parts []string) {
	if len(parts) >= 2 {
		if friendNum, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
			c.initiateCall(uint32(friendNum), true, false)
		} else {
			fmt.Println("Invalid friend number")
		}
	} else {
		fmt.Println("Usage: call <friend_number>")
	}
}

// handleVideoCallCommand processes the 'videocall' command to initiate a video call
func (c *ToxAVClient) handleVideoCallCommand(parts []string) {
	if len(parts) >= 2 {
		if friendNum, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
			c.initiateCall(uint32(friendNum), true, true)
		} else {
			fmt.Println("Invalid friend number")
		}
	} else {
		fmt.Println("Usage: videocall <friend_number>")
	}
}

// handleHangupCommand processes the 'hangup' command to end a call
func (c *ToxAVClient) handleHangupCommand(parts []string) {
	if len(parts) >= 2 {
		if friendNum, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
			c.hangupCall(uint32(friendNum))
		} else {
			fmt.Println("Invalid friend number")
		}
	} else {
		fmt.Println("Usage: hangup <friend_number>")
	}
}

// showHelp displays available commands
func (c *ToxAVClient) showHelp() {
	fmt.Println("\n📋 Available Commands:")
	fmt.Println("  help, h              - Show this help")
	fmt.Println("  friends, f           - List friends")
	fmt.Println("  calls, c             - Show active calls")
	fmt.Println("  stats, s             - Show statistics")
	fmt.Println("  add <tox_id> [msg]   - Add friend")
	fmt.Println("  msg <num> <message>  - Send message")
	fmt.Println("  call <num>           - Audio call")
	fmt.Println("  videocall <num>      - Video call")
	fmt.Println("  hangup <num>         - End call")
	fmt.Println("  save                 - Save profile")
	fmt.Println("  quit, exit, q        - Exit client")
	fmt.Println("\nFriends can send: 'call', 'videocall', 'status', 'help', 'echo <text>'")
}

// showFriends lists all friends
func (c *ToxAVClient) showFriends() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.friends) == 0 {
		fmt.Println("👥 No friends added yet")
		return
	}

	fmt.Printf("\n👥 Friends (%d):\n", len(c.friends))
	for _, friend := range c.friends {
		fmt.Printf("  [%d] %s (last seen: %v)\n",
			friend.Number, friend.Name, friend.LastSeen.Format("15:04:05"))
	}
}

// showActiveCalls lists all active calls
func (c *ToxAVClient) showActiveCalls() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.activeCalls) == 0 {
		fmt.Println("📞 No active calls")
		return
	}

	fmt.Printf("\n📞 Active Calls (%d):\n", len(c.activeCalls))
	for _, session := range c.activeCalls {
		friendName := "Unknown"
		if friend, exists := c.friends[session.FriendNumber]; exists {
			friendName = friend.Name
		}

		duration := time.Since(session.StartTime)
		session.mu.RLock()
		frames := session.FramesRecv
		session.mu.RUnlock()

		fmt.Printf("  [%d] %s - %v (Audio: %v, Video: %v, Frames: %d)\n",
			session.FriendNumber, friendName, duration.Round(time.Second),
			session.AudioEnabled, session.VideoEnabled, frames)
	}
}

// showStats displays client statistics
func (c *ToxAVClient) showStats() {
	c.mu.RLock()
	activeCalls := len(c.activeCalls)
	totalFriends := len(c.friends)
	c.mu.RUnlock()

	fmt.Printf("\n📊 Statistics:\n")
	fmt.Printf("  Friends: %d\n", totalFriends)
	fmt.Printf("  Active calls: %d\n", activeCalls)
	fmt.Printf("  Messages sent: %d\n", c.messagesSent)
	fmt.Printf("  Messages received: %d\n", c.messagesReceived)
	fmt.Printf("  Calls initiated: %d\n", c.callsInitiated)
	fmt.Printf("  Calls received: %d\n", c.callsReceived)
}

// addFriend adds a new friend
func (c *ToxAVClient) addFriend(toxID, message string) {
	if message == "" {
		message = "Friend request from ToxAV Integration Demo"
	}

	friendNumber, err := c.tox.AddFriend(toxID, message)
	if err != nil {
		fmt.Printf("❌ Failed to add friend: %v\n", err)
		return
	}

	publicKey, err := c.tox.GetFriendPublicKey(friendNumber)
	if err != nil {
		fmt.Printf("⚠️  Friend added but failed to get public key: %v\n", err)
	}

	c.mu.Lock()
	c.friends[friendNumber] = &FriendInfo{
		Number:    friendNumber,
		Name:      fmt.Sprintf("Friend_%d", friendNumber),
		Status:    "",
		LastSeen:  time.Now(),
		PublicKey: publicKey,
	}
	c.mu.Unlock()

	fmt.Printf("✅ Friend request sent to %s\n", toxID[:16]+"...")
}

// hangupCall ends a call with a friend
func (c *ToxAVClient) hangupCall(friendNumber uint32) {
	c.mu.Lock()
	session, exists := c.activeCalls[friendNumber]
	if !exists {
		c.mu.Unlock()
		fmt.Printf("❌ No active call with friend %d\n", friendNumber)
		return
	}
	delete(c.activeCalls, friendNumber)
	c.mu.Unlock()

	if err := c.toxav.CallControl(friendNumber, av.CallControlCancel); err != nil {
		fmt.Printf("❌ Failed to hang up call: %v\n", err)
	} else {
		duration := time.Since(session.StartTime)
		fmt.Printf("📞 Call ended after %v\n", duration.Round(time.Second))
	}
}

// saveProfile saves the current profile to disk
func (c *ToxAVClient) saveProfile() {
	savedata := c.tox.GetSavedata()
	if err := os.WriteFile(saveDataFile, savedata, 0o600); err != nil {
		fmt.Printf("❌ Failed to save profile: %v\n", err)
	} else {
		fmt.Printf("💾 Profile saved (%d bytes)\n", len(savedata))
	}
}

// Run starts the client
func (c *ToxAVClient) Run() {
	fmt.Println("\n🎯 ToxAV Integration Demo")
	fmt.Println("========================")
	fmt.Println("This demo shows complete Tox+ToxAV integration.")
	fmt.Println("Type 'help' for commands, Ctrl+C to exit.")

	// Bootstrap to network
	err := c.tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		log.Printf("⚠️  Bootstrap warning: %v", err)
	} else {
		fmt.Println("🌐 Connected to Tox network")
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start input reader
	go c.inputReader()

	// Main iteration loop
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	fmt.Print("\n> ")

	for c.running {
		select {
		case <-sigChan:
			fmt.Println("\n🛑 Shutdown signal received")
			c.running = false

		case <-ticker.C:
			c.tox.Iterate()
			c.toxav.Iterate()

		default:
			time.Sleep(1 * time.Millisecond)
		}
	}

	c.shutdown()
}

// inputReader handles user input
func (c *ToxAVClient) inputReader() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() && c.running {
		command := scanner.Text()
		if command != "" {
			c.processCommand(command)
		}
		if c.running {
			fmt.Print("> ")
		}
	}
}

// shutdown cleans up resources
func (c *ToxAVClient) shutdown() {
	fmt.Println("\n🧹 Shutting down...")

	// Save profile
	c.saveProfile()

	// Show final statistics
	c.showStats()

	// Clean up
	if c.toxav != nil {
		c.toxav.Kill()
	}
	if c.tox != nil {
		c.tox.Kill()
	}

	fmt.Println("✅ Shutdown completed")
}

func main() {
	client, err := NewToxAVClient()
	if err != nil {
		log.Fatalf("❌ Failed to create client: %v", err)
	}

	client.Run()
	fmt.Println("👋 Demo completed")
}
