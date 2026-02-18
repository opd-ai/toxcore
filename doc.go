// Package toxcore implements the core functionality of the Tox protocol.
//
// Tox is a peer-to-peer, encrypted messaging protocol designed for secure
// communications without relying on centralized infrastructure. This package
// provides the main API facade that integrates all subsystems of the toxcore-go
// implementation: DHT routing, network transport, cryptography, friend management,
// async messaging, file transfers, and group chat.
//
// # Getting Started
//
// Create a new Tox instance with options and set up callbacks for events:
//
//	options := toxcore.NewOptions()
//	options.UDPEnabled = true
//
//	tox, err := toxcore.New(options)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tox.Kill()
//
//	// Set up event callbacks
//	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
//	    tox.AddFriendByPublicKey(publicKey)
//	})
//
//	tox.OnFriendMessage(func(friendID uint32, message string) {
//	    fmt.Printf("Message from %d: %s\n", friendID, message)
//	})
//
//	// Connect to the Tox network
//	err = tox.Bootstrap("node.tox.biribiri.org", 33445,
//	    "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Start the event loop
//	for tox.IsRunning() {
//	    tox.Iterate()
//	    time.Sleep(tox.IterationInterval())
//	}
//
// # Core Types
//
// The package defines several core types:
//
//   - [Tox]: Main API facade integrating all Tox subsystems
//   - [Options]: Configuration options for creating a new Tox instance
//   - [TimeProvider]: Interface for injectable time (testing support)
//
// # Friend Management
//
// Add friends and manage relationships:
//
//	// Add friend by Tox address (includes nospam)
//	friendID, err := tox.AddFriend(toxAddress, "Hello!")
//
//	// Add friend by public key (requires prior mutual agreement)
//	friendID, err := tox.AddFriendByPublicKey(publicKey)
//
//	// Get friend list
//	friends := tox.GetFriendList()
//
//	// Send message to friend
//	messageID, err := tox.SendMessage(friendID, "Hello, friend!")
//
// # Messaging Callbacks
//
// Register callbacks to handle incoming messages and events:
//
//	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
//	    // Handle incoming friend request
//	})
//
//	tox.OnFriendMessage(func(friendID uint32, message string) {
//	    // Handle incoming message
//	})
//
//	tox.OnFriendConnectionStatus(func(friendID uint32, status ConnectionStatus) {
//	    // Handle friend online/offline status change
//	})
//
//	tox.OnFriendTyping(func(friendID uint32, isTyping bool) {
//	    // Handle typing indicator
//	})
//
// # Self Information
//
// Manage your own Tox identity:
//
//	// Get your Tox address (share with others to connect)
//	address := tox.GetAddress()
//
//	// Get/set name and status
//	name := tox.GetName()
//	tox.SetName("Alice")
//
//	status := tox.GetStatusMessage()
//	tox.SetStatusMessage("Available")
//
//	// Get public key
//	pubKey := tox.GetPublicKey()
//
// # File Transfers
//
// Send and receive files:
//
//	// Send a file
//	fileNumber, err := tox.FileSend(friendID, toxcore.FileKindData,
//	    fileSize, nil, "photo.jpg")
//
//	// Handle file receive callback
//	tox.OnFileReceive(func(friendID, fileNumber uint32, kind uint32,
//	    size uint64, filename string) {
//	    // Accept or reject the file
//	    tox.FileControl(friendID, fileNumber, toxcore.FileControlResume)
//	})
//
// # Group Chat
//
// Create and manage group chats:
//
//	// Create new group
//	groupID, err := tox.GroupNew(toxcore.GroupPrivacyPublic, "Group Name", "name")
//
//	// Join existing group
//	groupID, err := tox.GroupJoin(chatID, password, "nick", "")
//
//	// Send message to group
//	err = tox.GroupSendMessage(groupID, toxcore.GroupMessageNormal, "Hello group!")
//
// # Network Configuration
//
// Configure network behavior:
//
//	options := toxcore.NewOptions()
//	options.UDPEnabled = true           // Enable UDP transport
//	options.TCPPort = 33445             // Set TCP listening port
//	options.ProxyType = toxcore.ProxySocks5
//	options.ProxyHost = "127.0.0.1"
//	options.ProxyPort = 9050
//
// # Persistence
//
// Save and restore Tox state:
//
//	// Save to file
//	data, err := tox.GetSavedata()
//	os.WriteFile("tox.save", data, 0600)
//
//	// Restore from saved data
//	options := toxcore.NewOptions()
//	options.Savedata = savedData
//	options.SavedataType = toxcore.SavedataTypeToxSave
//	tox, err := toxcore.New(options)
//
// # Deterministic Testing
//
// For reproducible testing, time-dependent components support injectable time providers:
//
//	tox, _ := toxcore.New(options)
//	mockTime := &MockTimeProvider{CurrentTime: time.Unix(1000, 0)}
//	tox.SetTimeProvider(mockTime)
//
// This enables deterministic testing of time-based features like friend request
// retry logic, LastSeen timestamps, and file transfer ID generation.
//
// # Thread Safety
//
// The Tox struct is safe for concurrent use. Internal synchronization ensures
// that callbacks and API calls can be made from multiple goroutines:
//
//   - All public methods use appropriate mutex protection
//   - Callbacks are invoked with proper locking semantics
//   - The iteration loop can run in a dedicated goroutine
//
// # Integration Architecture
//
// This package serves as the main integration point, orchestrating:
//
//   - [dht]: DHT routing and peer discovery
//   - [transport]: UDP/TCP network transport with Noise protocol
//   - [crypto]: Cryptographic operations and key management
//   - [friend]: Friend relationship management
//   - [async]: Asynchronous messaging with forward secrecy
//   - [file]: File transfer operations
//   - [group]: Group chat functionality
//   - [messaging]: Core message handling
//
// # C API Bindings
//
// The package supports C interoperability through the capi subpackage.
// Build with -buildmode=c-shared to generate a C-compatible library.
//
// See the capi package documentation for details on C bindings.
package toxcore
