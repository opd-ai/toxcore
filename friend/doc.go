// Package friend implements friend management for the Tox protocol, handling
// friend requests, friend list operations, and peer-to-peer relationship state.
//
// # Overview
//
// The friend package provides two primary components:
//
//   - FriendInfo: Represents a friend's state including public key, name,
//     status message, online status, and connection status
//   - RequestManager: Thread-safe management of incoming friend requests with
//     callback support and duplicate detection
//
// # FriendInfo
//
// FriendInfo tracks the state of a friend relationship. Note that it is named
// FriendInfo (not Friend) to avoid namespace collision with toxcore.Friend.
//
//	f := friend.New(publicKey)
//	if err := f.SetName("Alice"); err != nil {
//	    log.Fatal(err) // Name exceeds MaxNameLength (128 bytes)
//	}
//	if err := f.SetStatusMessage("Available for chat"); err != nil {
//	    log.Fatal(err) // Message exceeds MaxStatusMessageLength (1007 bytes)
//	}
//
//	status := f.GetStatus()           // FriendStatusNone, FriendStatusAway, FriendStatusBusy, FriendStatusOnline
//	connStatus := f.GetConnectionStatus() // ConnectionNone, ConnectionTCP, ConnectionUDP
//	lastSeenAgo := f.LastSeenDuration()   // Duration since friend was last seen
//
// # Friend Requests
//
// The Request type represents a friend request with encrypted transmission:
//
//	// Create an outgoing friend request
//	request, err := friend.NewRequest(recipientPublicKey, "Hi, let's connect!", senderSecretKey)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Encrypt for network transmission
//	packet, err := request.Encrypt(senderKeyPair, recipientPublicKey)
//	// Send packet over transport...
//
//	// Decrypt received friend request
//	received, err := friend.DecryptRequest(packet, recipientSecretKey)
//	fmt.Printf("Request from %x: %s\n", received.SenderPublicKey[:8], received.Message)
//
// # RequestManager
//
// RequestManager provides thread-safe friend request handling with callbacks:
//
//	manager := friend.NewRequestManager()
//
//	// Set handler for incoming requests
//	manager.SetHandler(func(request *friend.Request) bool {
//	    fmt.Printf("Friend request from %x: %s\n",
//	        request.SenderPublicKey[:8], request.Message)
//	    return true // Accept the request
//	})
//
//	// Process incoming request (typically called from packet handler)
//	manager.AddRequest(receivedRequest)
//
//	// Query pending requests
//	pending := manager.GetPendingRequests()
//
//	// Accept or reject by public key
//	manager.AcceptRequest(publicKey)
//	manager.RejectRequest(publicKey)
//
// # Deterministic Testing
//
// For reproducible test scenarios, use the TimeProvider variants:
//
//	type MockTimeProvider struct {
//	    currentTime time.Time
//	}
//
//	func (m *MockTimeProvider) Now() time.Time {
//	    return m.currentTime
//	}
//
//	mockTime := &MockTimeProvider{currentTime: time.Unix(1000000, 0)}
//	f := friend.NewWithTimeProvider(publicKey, mockTime)
//	request, _ := friend.NewRequestWithTimeProvider(pubKey, "Hello", secretKey, mockTime)
//
// # Thread Safety
//
// FriendInfo methods are not thread-safe; callers must synchronize access.
// RequestManager uses sync.RWMutex internally and is safe for concurrent use.
//
// # Integration
//
// This package is used by the main toxcore.Tox type for managing friend
// relationships. Friend requests are routed through the transport layer
// and processed via registered packet handlers. The FriendInfo type mirrors
// the friend state tracked in the Tox protocol specification.
//
// # C Bindings
//
// Exported functions are annotated with //export directives for C interoperability:
//
//   - ToxFriendInfo, ToxFriendInfoNew - Friend state management
//   - ToxFriendRequest, ToxFriendRequestNew, ToxFriendRequestDecrypt - Request handling
//   - ToxFriendRequestManagerNew, ToxFriendRequestManagerSetHandler - Manager operations
package friend
