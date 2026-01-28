# toxcore-go

A pure Go implementation of the Tox Messenger core protocol.

## Overview

toxcore-go is a clean, idiomatic Go implementation of the Tox protocol, designed for simplicity, security, and performance. It provides a comprehensive, CGo-free implementation with C binding annotations for cross-language compatibility.

Key features:
- Pure Go implementation with no CGo dependencies
- Comprehensive implementation of the Tox protocol
- **Multi-Network Support**: IPv4, IPv6, Tor .onion, I2P .b32.i2p, Nym .nym, and Lokinet .loki
- Clean API design with proper Go idioms
- C binding annotations for cross-language use
- Robust error handling and concurrency patterns

## Installation

**Requirements:** Go 1.23.2 or later

```bash
go get github.com/opd-ai/toxcore
```

### Verification

To verify the installation works correctly:

```bash
go mod tidy
go build ./...
go test ./...
```

## Basic Usage

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/toxcore"
)

func main() {
	// Create a new Tox instance
	options := toxcore.NewOptions()
	options.UDPEnabled = true
	
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatal(err)
	}
	defer tox.Kill()
	
	// Print our Tox ID
	fmt.Println("My Tox ID:", tox.SelfGetAddress())
	
	// Set up callbacks
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		fmt.Printf("Friend request: %s\n", message)
		
		// Automatically accept friend requests
		friendID, err := tox.AddFriendByPublicKey(publicKey)
		if err != nil {
			fmt.Printf("Error accepting friend request: %v\n", err)
		} else {
			fmt.Printf("Accepted friend request. Friend ID: %d\n", friendID)
		}
	})
	
	tox.OnFriendMessage(func(friendID uint32, message string) {
		fmt.Printf("Message from friend %d: %s\n", friendID, message)
		
		// Echo the message back (message type parameter is optional via variadic arguments, defaults to normal)
		tox.SendFriendMessage(friendID, "You said: "+message)
	})
	
	// Connect to a bootstrap node
	err = tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		log.Printf("Warning: Bootstrap failed: %v", err)
	}
	
	// Main loop
	fmt.Println("Running Tox...")
	for tox.IsRunning() {
		tox.Iterate()
		time.Sleep(tox.IterationInterval())
	}
}
```

> **Note:** For more message sending options including action messages, see the [Sending Messages](#sending-messages) section.

## Configuration Options

### Proxy Support

toxcore-go includes proxy configuration options in the `Options` struct for HTTP and SOCKS5 proxies:

```go
options := toxcore.NewOptions()
options.Proxy = &toxcore.ProxyOptions{
    Type:     toxcore.ProxyTypeSOCKS5,
    Host:     "127.0.0.1",
    Port:     9050,
    Username: "",  // Optional
    Password: "",  // Optional
}
```

**Current Status**: The proxy configuration API exists but is **not yet implemented**. Setting proxy options will have no effect on network traffic. All connections currently use direct UDP/TCP transport without proxy support.

**Workaround**: For users requiring proxy support (e.g., for Tor), use system-level proxy routing (iptables, proxychains, or network namespace routing) as a temporary solution until proxy integration is implemented.

## Multi-Network Support

toxcore-go includes a multi-network address system with IPv4/IPv6 support and architecture for privacy networks.

### Supported Network Types

- **IPv4/IPv6**: Traditional internet protocols (fully implemented)
- **Tor .onion**: Tor hidden services (interface ready, implementation planned)
- **I2P .b32.i2p**: I2P darknet addresses (interface ready, implementation planned)
- **Nym .nym**: Nym mixnet addresses (interface ready, implementation planned)
- **Lokinet .loki**: Lokinet onion routing addresses (interface ready, implementation planned)

### Usage Example

```go
package main

import (
    "fmt"
    "log"
    "net"
    
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Working with traditional IP addresses (fully supported)
    // Note: We resolve to get a net.Addr interface type
    addr, err := net.ResolveUDPAddr("udp", "192.168.1.1:8080")
    if err != nil {
        log.Fatal(err)
    }
    
    // Convert to the new NetworkAddress system
    netAddr, err := transport.ConvertNetAddrToNetworkAddress(addr)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Type: %s\n", netAddr.Type.String())           // Type: IPv4
    fmt.Printf("Address: %s\n", netAddr.String())             // Address: IPv4://192.168.1.1:8080
    fmt.Printf("Private: %t\n", netAddr.IsPrivate())          // Private: true
    fmt.Printf("Routable: %t\n", netAddr.IsRoutable())        // Routable: false
    
    // Privacy network addresses (interface ready, implementations planned)
    onionAddr := &transport.NetworkAddress{
        Type:    transport.AddressTypeOnion,
        Data:    []byte("exampleexampleexample.onion"),
        Port:    8080,
        Network: "tcp",
    }
    
    i2pAddr := &transport.NetworkAddress{
        Type:    transport.AddressTypeI2P,
        Data:    []byte("example12345678901234567890123456.b32.i2p"),
        Port:    8080,
        Network: "tcp",
    }
    
    // Address types work with existing net.Addr interfaces
    fmt.Printf("Onion: %s\n", onionAddr.ToNetAddr().String())
    fmt.Printf("I2P: %s\n", i2pAddr.ToNetAddr().String())
    
    // Note: Actual network connections for privacy networks require
    // implementation of the underlying network libraries
}
```

### Network-Specific Features

- **Privacy Detection**: Automatically detects if addresses are in private ranges
- **Routing Awareness**: Knows which addresses are routable through their respective networks
- **Backward Compatibility**: Existing code using `net.Addr` continues to work unchanged
- **Performance**: Sub-microsecond address conversions with minimal memory overhead

For detailed documentation, see [NETWORK_ADDRESS.md](docs/NETWORK_ADDRESS.md).

## Noise Protocol Framework Integration

toxcore-go implements the Noise Protocol Framework's IK (Initiator with Knowledge) pattern for enhanced security and protection against Key Compromise Impersonation (KCI) attacks. This provides:

- **Forward Secrecy**: Past communications remain secure even if long-term keys are compromised
- **KCI Resistance**: Resistant to key compromise impersonation attacks
- **Mutual Authentication**: Both parties verify each other's identity
- **Formal Security**: Uses formally verified cryptographic protocols

**Note**: Noise-IK requires explicit configuration and is disabled by default in standard bootstrap managers.

### Using NoiseTransport

The NoiseTransport wraps existing UDP/TCP transports with Noise-IK encryption:

```go
package main

import (
    "log"
    "net"
    
    "github.com/opd-ai/toxcore/crypto"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Generate a long-term key pair
    keyPair, err := crypto.GenerateKeyPair()
    if err != nil {
        log.Fatal(err)
    }
    
    // Create UDP transport
    udpTransport, err := transport.NewUDPTransport("127.0.0.1:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer udpTransport.Close()
    
    // Wrap with Noise encryption
    noiseTransport, err := transport.NewNoiseTransport(udpTransport, keyPair.Private[:])
    if err != nil {
        log.Fatal(err)
    }
    defer noiseTransport.Close()
    
    // Add known peers for encrypted communication
    peerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8081")
    if err != nil {
        log.Fatal(err)
    }
    peerPublicKey := [32]byte{0x12, 0x34, 0x56, 0x78} // Replace with actual peer's public key
    err = noiseTransport.AddPeer(peerAddr, peerPublicKey[:])
    if err != nil {
        log.Fatal(err)
    }
    
    // Send encrypted messages automatically
    packet := &transport.Packet{
        PacketType: transport.PacketFriendMessage,
        Data:       []byte("Hello, encrypted world!"),
    }
    
    err = noiseTransport.Send(packet, peerAddr)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Security Features

- **Automatic Handshake**: NoiseTransport automatically initiates Noise-IK handshakes with known peers
- **Transparent Encryption**: All packets (except handshake) are encrypted when a session exists
- **Fallback Support**: Falls back to unencrypted transmission for unknown peers
- **Session Management**: Maintains per-peer encryption sessions with proper cipher states

### Migration Strategy

The implementation supports gradual migration:

1. **Phase 1**: Library integration with IK handshake implementation ✅
2. **Phase 2**: Transport layer integration with NoiseTransport wrapper ✅  
3. **Phase 3**: Protocol version negotiation for backward compatibility ✅
4. **Phase 4**: Full migration with performance optimization

## Version Negotiation and Backward Compatibility

toxcore-go includes automatic protocol version negotiation to enable gradual migration across the Tox network:

### NegotiatingTransport

The `NegotiatingTransport` automatically handles protocol version negotiation and fallback:

```go
import (
    "crypto/rand"
    "github.com/opd-ai/toxcore/transport"
)

// Create base UDP transport
udp, err := transport.NewUDPTransport("0.0.0.0:33445")
if err != nil {
    log.Fatal(err)
}

// Configure protocol capabilities
capabilities := &transport.ProtocolCapabilities{
    SupportedVersions: []transport.ProtocolVersion{
        transport.ProtocolLegacy,   // Original Tox protocol
        transport.ProtocolNoiseIK,  // Noise-IK enhanced protocol
    },
    PreferredVersion:     transport.ProtocolNoiseIK,
    EnableLegacyFallback: true,    // Allow fallback to legacy
    NegotiationTimeout:   5 * time.Second,
}

// Generate or load your 32-byte Curve25519 private key
staticKey := make([]byte, 32)
_, err = rand.Read(staticKey) // Generate random key or load from secure storage
if err != nil {
    log.Fatal(err)
}

// Create negotiating transport with your static key
negotiatingTransport, err := transport.NewNegotiatingTransport(udp, capabilities, staticKey)
if err != nil {
    log.Fatal(err)
}

// Use like any transport - version negotiation is automatic
packet := &transport.Packet{
    PacketType: transport.PacketFriendMessage,
    Data:       []byte("Hello!"),
}

// First send to unknown peer triggers version negotiation
// Subsequent sends use the negotiated protocol automatically
err = negotiatingTransport.Send(packet, peerAddr)
```

### Protocol Versions

- **Legacy (v0)**: Original Tox protocol for backward compatibility
- **Noise-IK (v1)**: Enhanced security with forward secrecy and KCI resistance

### Migration Configurations

**Conservative Deployment** (maximum compatibility):
```go
capabilities := &transport.ProtocolCapabilities{
    SupportedVersions:    []transport.ProtocolVersion{transport.ProtocolLegacy, transport.ProtocolNoiseIK},
    PreferredVersion:     transport.ProtocolNoiseIK,
    EnableLegacyFallback: true,  // Always allow legacy fallback
}
```

**Security-Focused Deployment** (Noise-IK only):
```go
capabilities := &transport.ProtocolCapabilities{
    SupportedVersions:    []transport.ProtocolVersion{transport.ProtocolNoiseIK},
    PreferredVersion:     transport.ProtocolNoiseIK,
    EnableLegacyFallback: false, // Reject legacy connections
}
```

### Features

- **Automatic Negotiation**: Peers automatically discover and use the best mutually supported protocol
- **Transparent Operation**: No API changes required - works as drop-in transport replacement
- **Per-Peer Versioning**: Each peer connection can use different protocol versions
- **Graceful Fallback**: Automatic fallback to legacy protocol when Noise-IK not supported
- **Zero Overhead**: Version negotiation happens once per peer, then cached
- **Thread-Safe**: Safe for concurrent use across multiple goroutines

## Advanced Message Callback API

For advanced users who need access to message types (normal vs action), toxcore-go provides a detailed callback API:

```go
// Use OnFriendMessageDetailed for access to message types
tox.OnFriendMessageDetailed(func(friendID uint32, message string, messageType toxcore.MessageType) {
	switch messageType {
	case toxcore.MessageTypeNormal:
		fmt.Printf("💬 Normal message from friend %d: %s\n", friendID, message)
	case toxcore.MessageTypeAction:
		fmt.Printf("🎭 Action message from friend %d: %s\n", friendID, message)
	}
})

// You can register both callbacks if needed - both will be called
tox.OnFriendMessage(func(friendID uint32, message string) {
	fmt.Printf("Simple callback: %s\n", message)
})
```

## Sending Messages

The `SendFriendMessage` method provides a consistent API for sending messages with optional message types:

```go
// Send a normal message (default behavior)
err := tox.SendFriendMessage(friendID, "Hello there!")
if err != nil {
    log.Printf("Failed to send message: %v", err)
}

// Send an explicit normal message  
err = tox.SendFriendMessage(friendID, "Hello there!", toxcore.MessageTypeNormal)

// Send an action message (like "/me waves" in IRC)
err = tox.SendFriendMessage(friendID, "waves hello", toxcore.MessageTypeAction)
```

**Message Limits:**
- Messages cannot be empty
- Maximum message length is 1372 UTF-8 bytes (not characters - multi-byte Unicode may be shorter)
- Friend must exist and be connected to receive messages

**Example:** The message "Hello 🎉" contains 7 characters but uses 10 UTF-8 bytes (6 for "Hello " + 4 for the emoji).

## Self Management API

toxcore-go provides complete self-management functionality for setting your name and status message:

```go
// Set your display name (max 128 bytes UTF-8)
err := tox.SelfSetName("Alice")
if err != nil {
    log.Printf("Failed to set name: %v", err)
}

// Get your current name
name := tox.SelfGetName()
fmt.Printf("My name: %s\n", name)

// Set your status message (max 1007 bytes UTF-8)
err = tox.SelfSetStatusMessage("Available for chat 💬")
if err != nil {
    log.Printf("Failed to set status message: %v", err)
}

// Get your current status message
statusMsg := tox.SelfGetStatusMessage()
fmt.Printf("My status: %s\n", statusMsg)
```

### Profile Management Example

```go
func setupProfile(tox *toxcore.Tox) error {
    // Set your identity
    if err := tox.SelfSetName("Alice Cooper"); err != nil {
        return fmt.Errorf("failed to set name: %w", err)
    }
    
    if err := tox.SelfSetStatusMessage("Building the future with Tox!"); err != nil {
        return fmt.Errorf("failed to set status: %w", err)
    }
    
    // Display current profile
    fmt.Printf("Profile: %s - %s\n", tox.SelfGetName(), tox.SelfGetStatusMessage())
    
    return nil
}
```

**Important Notes:**
- Names and status messages are automatically included in savedata and persist across restarts
- Both support full UTF-8 encoding including emojis
- Changes are immediately available to connected friends
- Length limits are enforced (128 bytes for names, 1007 bytes for status messages)

### Nospam Management

The nospam value is part of your Tox ID and can be changed to create a new Tox ID while keeping the same cryptographic identity:

```go
// Get your current nospam value
nospam := tox.SelfGetNospam()
fmt.Printf("Current nospam: %x\n", nospam)

// Set a new nospam value (changes your Tox ID)
newNospam := [4]byte{0x12, 0x34, 0x56, 0x78}
tox.SelfSetNospam(newNospam)

// Your Tox ID has now changed
fmt.Printf("New Tox ID: %s\n", tox.SelfGetAddress())
```

**Nospam Use Cases:**
- **Privacy**: Change your Tox ID without generating new keys
- **Anti-spam**: Stop receiving unwanted friend requests by changing nospam
- **Identity rotation**: Regularly rotate your public Tox ID for privacy

**Important Notes:**
- Nospam changes are automatically saved in savedata
- Existing friends are unaffected by nospam changes (they use your public key)
- New friend requests must use your updated Tox ID

## Friend Management API

toxcore-go provides comprehensive friend management functionality:

### Adding Friends

```go
// Accept a friend request (use in OnFriendRequest callback)
// Uses the public key [32]byte from the callback
friendID, err := tox.AddFriendByPublicKey(publicKey)

// Send a friend request with a message  
// Uses a Tox ID string (public key + nospam + checksum = 76 hex characters)
friendID, err := tox.AddFriend("76518406f6a9f2217e8dc487cc783c25cc16a15eb36ff32e335364ec37b1334912345678868a", "Hello!")
```

### Managing Friends

```go
// Get all friends
friends := tox.GetFriends()
for friendID, friend := range friends {
    fmt.Printf("Friend %d: %s\n", friendID, friend.Name)
}

// Get friend count
count := tox.GetFriendsCount()
fmt.Printf("Total friends: %d\n", count)

// Get friend's public key
publicKey, err := tox.GetFriendPublicKey(friendID)
if err != nil {
    log.Printf("Failed to get friend public key: %v", err)
}

// Remove a friend
err := tox.DeleteFriend(friendID)
if err != nil {
    log.Printf("Failed to delete friend: %v", err)
}
```

## C API Usage

toxcore-go can be used from C code via the provided C bindings:

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "toxcore.h"

void friend_request_callback(uint8_t* public_key, const char* message, void* user_data) {
    printf("Friend request received: %s\n", message);
    
    // Accept the friend request
    uint32_t friend_id;
    TOX_ERR_FRIEND_ADD err;
    friend_id = tox_friend_add_norequest(tox, public_key, &err);
    
    if (err != TOX_ERR_FRIEND_ADD_OK) {
        printf("Error accepting friend request: %d\n", err);
    } else {
        printf("Friend added with ID: %u\n", friend_id);
    }
}

void friend_message_callback(uint32_t friend_id, TOX_MESSAGE_TYPE type, 
                             const uint8_t* message, size_t length, void* user_data) {
    char* msg = malloc(length + 1);
    memcpy(msg, message, length);
    msg[length] = '\0';
    
    printf("Message from friend %u: %s\n", friend_id, msg);
    
    // Echo the message back
    tox_friend_send_message(tox, friend_id, type, message, length, NULL);
    
    free(msg);
}

int main() {
    // Create a new Tox instance
    struct Tox_Options options;
    tox_options_default(&options);
    
    TOX_ERR_NEW err;
    Tox* tox = tox_new(&options, &err);
    if (err != TOX_ERR_NEW_OK) {
        printf("Error creating Tox instance: %d\n", err);
        return 1;
    }
    
    // Register callbacks
    tox_callback_friend_request(tox, friend_request_callback, NULL);
    tox_callback_friend_message(tox, friend_message_callback, NULL);
    
    // Print our Tox ID
    uint8_t tox_id[TOX_ADDRESS_SIZE];
    tox_self_get_address(tox, tox_id);
    
    char id_str[TOX_ADDRESS_SIZE*2 + 1];
    for (int i = 0; i < TOX_ADDRESS_SIZE; i++) {
        sprintf(id_str + i*2, "%02X", tox_id[i]);
    }
    id_str[TOX_ADDRESS_SIZE*2] = '\0';
    
    printf("My Tox ID: %s\n", id_str);
    
    // Bootstrap
    uint8_t bootstrap_pub_key[TOX_PUBLIC_KEY_SIZE];
    hex_string_to_bin("F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67", bootstrap_pub_key);
    
    tox_bootstrap(tox, "node.tox.biribiri.org", 33445, bootstrap_pub_key, NULL);
    
    // Main loop
    printf("Running Tox...\n");
    while (1) {
        tox_iterate(tox, NULL);
        uint32_t interval = tox_iteration_interval(tox);
        usleep(interval * 1000);
    }
    
    tox_kill(tox);
    return 0;
}
```

## State Persistence

toxcore-go supports saving and restoring your Tox state, including your private key and friends list, allowing you to maintain your identity and connections across application restarts.

### Saving State

```go
// Get your Tox state as bytes for persistence
savedata := tox.GetSavedata()

// Save to file
err := os.WriteFile("my_tox_state.dat", savedata, 0600)
if err != nil {
    log.Printf("Failed to save state: %v", err)
}
```

### Restoring State

```go
// Load from file
savedata, err := os.ReadFile("my_tox_state.dat")
if err != nil {
    log.Printf("Failed to read state file: %v", err)
    // Create new instance
    tox, err = toxcore.New(options)
} else {
    // Restore from saved state
    tox, err = toxcore.NewFromSavedata(options, savedata)
}

if err != nil {
    log.Fatal(err)
}
```

### Loading State via Options

You can also load saved state during initialization by providing it in the Options:

```go
// Load savedata from file
savedata, err := os.ReadFile("my_tox_state.dat")
if err != nil {
    log.Printf("Failed to read state file: %v", err)
    return
}

// Create options with savedata
options := &toxcore.Options{
    UDPEnabled:     true,
    SavedataType:   toxcore.SaveDataTypeToxSave,
    SavedataData:   savedata,
    SavedataLength: uint32(len(savedata)),
}

// Initialize with saved state loaded automatically
tox, err := toxcore.New(options)
if err != nil {
    log.Printf("Failed to create Tox instance with savedata: %v", err)
    return
}
defer tox.Kill()

// Your friends list and settings are automatically restored
fmt.Printf("Restored Tox ID: %s\n", tox.SelfGetAddress())
fmt.Printf("Friends restored: %d\n", len(tox.GetFriends()))
```

### Updating Existing Instance

```go
// You can also load state into an existing Tox instance
err := tox.Load(savedata)
if err != nil {
    log.Printf("Failed to load state: %v", err)
}
```

### Complete Example with Persistence

```go
package main

import (
    "fmt"
    "log"
    "os"
    "time"
    
    "github.com/opd-ai/toxcore"
)

func main() {
    var tox *toxcore.Tox
    var err error
    
    // Try to load existing state
    savedata, err := os.ReadFile("tox_state.dat")
    if err != nil {
        // No existing state, create new instance
        fmt.Println("Creating new Tox instance...")
        options := toxcore.NewOptions()
        options.UDPEnabled = true
        tox, err = toxcore.New(options)
    } else {
        // Restore from saved state
        fmt.Println("Restoring from saved state...")
        tox, err = toxcore.NewFromSavedata(nil, savedata)
    }
    
    if err != nil {
        log.Fatal(err)
    }
    defer tox.Kill()
    
    fmt.Printf("My Tox ID: %s\n", tox.SelfGetAddress())
    
    // Set up callbacks
    tox.OnFriendRequest(func(publicKey [32]byte, message string) {
        fmt.Printf("Friend request: %s\n", message)
        friendID, err := tox.AddFriendByPublicKey(publicKey)
        if err == nil {
            fmt.Printf("Accepted friend request. Friend ID: %d\n", friendID)
            
            // Save state after adding friend
            saveState(tox)
        }
    })
    
    tox.OnFriendMessage(func(friendID uint32, message string) {
        fmt.Printf("Message from friend %d: %s\n", friendID, message)
    })
    
    // Bootstrap
    err = tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
    if err != nil {
        log.Printf("Warning: Bootstrap failed: %v", err)
    }
    
    // Save state periodically and on shutdown
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        for range ticker.C {
            saveState(tox)
        }
    }()
    
    // Save state on program exit
    defer saveState(tox)
    
    // Main loop
    fmt.Println("Running Tox...")
    for tox.IsRunning() {
        tox.Iterate()
        time.Sleep(tox.IterationInterval())
    }
}

func saveState(tox *toxcore.Tox) {
    savedata := tox.GetSavedata()
    err := os.WriteFile("tox_state.dat", savedata, 0600)
    if err != nil {
        log.Printf("Failed to save state: %v", err)
    } else {
        fmt.Println("State saved successfully")
    }
}
```

**Important Notes:**
- The savedata contains your private key and should be kept secure
- Always use appropriate file permissions (0600) when saving to disk
- Save state after significant changes (adding friends, etc.)
- Consider encrypting the savedata for additional security

## Asynchronous Message Delivery System (Unofficial Extension)

toxcore-go includes an experimental asynchronous message delivery system that enables offline messaging while maintaining Tox's decentralized nature and security properties. This is an **unofficial extension** of the Tox protocol.

### Overview

The async messaging system allows users to send messages to offline friends, with messages being temporarily stored on distributed storage nodes until the recipient comes online. All messages maintain end-to-end encryption and forward secrecy. **Users can become storage nodes when async manager initialization succeeds**, contributing 1% of their available disk space to help the network. If storage node initialization fails, async messaging features will be unavailable but core Tox functionality remains intact.

**Privacy Enhancement**: The system uses cryptographic peer identity obfuscation to hide real sender and recipient identities from storage nodes while maintaining message deliverability.

### Key Features

- **End-to-End Encryption**: Messages are encrypted by the sender using the recipient's public key
- **Peer Identity Obfuscation**: Storage nodes see only cryptographic pseudonyms, not real identities
- **Storage Node Participation**: Users can become storage nodes when initialization succeeds, with 1% disk space allocation
- **Fair Resource Usage**: Storage capacity dynamically calculated based on available disk space (1MB-1GB bounds)
- **Distributed Storage**: No single point of failure - messages distributed across multiple storage nodes
- **Automatic Expiration**: Messages automatically expire after 24 hours to prevent storage bloat
- **Anti-Spam Protection**: Per-recipient message limits and storage capacity controls
- **Seamless Integration**: Works alongside regular Tox messaging with automatic fallback

### Basic Usage

```go
package main

import (
    "log"
    "time"
    
    "github.com/opd-ai/toxcore"
    "github.com/opd-ai/toxcore/async"
    "github.com/opd-ai/toxcore/crypto"
    "github.com/opd-ai/toxcore/transport"
)

func main() {
    // Create Tox instance
    tox, err := toxcore.New(toxcore.NewOptions())
    if err != nil {
        log.Fatal(err)
    }
    defer tox.Kill()
    
    // Get key pair for async messaging
    keyPair, err := crypto.GenerateKeyPair()
    if err != nil {
        log.Fatal(err)
    }
    
    // Create async manager with automatic storage capabilities
    dataDir := "/path/to/user/data"
    transport, err := transport.NewUDPTransport("0.0.0.0:0") // Auto-assign port
    if err != nil {
        log.Fatal(err)
    }
    asyncManager, err := async.NewAsyncManager(keyPair, transport, dataDir)
    if err != nil {
        log.Fatal(err)
    }
    asyncManager.Start()
    defer asyncManager.Stop()
    
    // Monitor automatic storage participation
    stats := asyncManager.GetStorageStats()
    if stats != nil {
        log.Printf("Storage capacity: %d messages (1%% of available disk space)", stats.StorageCapacity)
    }
    
    // Set up async message handler
    asyncManager.SetAsyncMessageHandler(func(senderPK [32]byte, message string, messageType async.MessageType) {
        log.Printf("📨 Received async message from %x: %s", senderPK[:8], message)
    })
    
    // Send async message to offline friend
    friendPK := [32]byte{0x12, 0x34, 0x56, 0x78} // Friend's public key
    asyncManager.SetFriendOnlineStatus(friendPK, false) // Mark as offline
    
    err = asyncManager.SendAsyncMessage(friendPK, "Hello! This will be delivered when you come online.", async.MessageTypeNormal)
    if err != nil {
        log.Printf("Failed to send async message: %v", err)
    }
    
    // When friend comes online, messages are automatically retrieved
    time.Sleep(5 * time.Second)
    asyncManager.SetFriendOnlineStatus(friendPK, true)
    
    // Keep running to handle message retrieval
    time.Sleep(10 * time.Second)
}
```

### Privacy Protection (Automatic)

**All async messages automatically use peer identity obfuscation** - no configuration required:

- **Sender Anonymity**: Storage nodes see random, unlinkable pseudonyms instead of real sender public keys
- **Recipient Anonymity**: Storage nodes see time-rotating pseudonyms (6-hour epochs) instead of real recipient keys  
- **Message Unlinkability**: Each message appears completely unrelated to storage nodes
- **Forward Secrecy**: Messages maintain end-to-end encryption with forward secrecy guarantees
- **Zero Configuration**: Privacy protection works automatically with existing APIs

```go
// All these methods automatically provide peer identity obfuscation:
asyncManager.SendAsyncMessage(friendPK, "Private message", async.MessageTypeNormal)
messages, _ := asyncClient.RetrieveAsyncMessages()  // Retrieves via pseudonym lookup
asyncClient.SendForwardSecureAsyncMessage(fsMsg)   // Obfuscated transport

// No API changes needed - privacy protection is built-in by default
```

### Automatic Storage Node Operation

Users can participate as storage nodes when initialization succeeds, contributing to the network's resilience:

```go
// AsyncManager instances provide storage when successfully initialized
keyPair, err := crypto.GenerateKeyPair()
if err != nil {
    log.Fatal(err)
}
dataDir := "/path/to/user/data"
transport, err := transport.NewUDPTransport("0.0.0.0:0") // Auto-assign port
if err != nil {
    log.Fatal(err)
}

asyncManager, err := async.NewAsyncManager(keyPair, transport, dataDir)
if err != nil {
    log.Fatal(err)
}
asyncManager.Start()

// Monitor automatic storage statistics
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    for range ticker.C {
        stats := asyncManager.GetStorageStats()
        if stats != nil {
            log.Printf("Automatic storage: %d/%d messages (%.1f%% capacity)", 
                stats.TotalMessages, stats.StorageCapacity,
                float64(stats.TotalMessages)/float64(stats.StorageCapacity)*100)
        }
    }
}()

// Capacity automatically updates based on available disk space
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        asyncManager.UpdateStorageCapacity() // Automatic capacity adjustment
    }
}()
```

### Direct Message Storage API

For advanced users who want direct control over message storage with forward secrecy:

```go
// Create storage instance with automatic capacity
storageKeyPair, err := crypto.GenerateKeyPair()
if err != nil {
    log.Fatal(err)
}
dataDir := "/path/to/storage/data"
storage := async.NewMessageStorage(storageKeyPair, dataDir)

// Monitor storage capacity (automatically calculated)
log.Printf("Storage capacity: %d messages", storage.GetMaxCapacity())

// Create forward security manager for forward-secure messaging
senderKeyPair, err := crypto.GenerateKeyPair()
if err != nil {
    log.Fatal(err)
}
senderFSM, err := async.NewForwardSecurityManager(senderKeyPair, dataDir)
if err != nil {
    log.Fatal(err)
}

// Recipient must also create their own forward security manager
recipientKeyPair, err := crypto.GenerateKeyPair()
if err != nil {
    log.Fatal(err)
}
recipientFSM, err := async.NewForwardSecurityManager(recipientKeyPair, dataDir)
if err != nil {
    log.Fatal(err)
}

// Step 1: Exchange pre-keys between sender and recipient (both directions)
// Sender generates pre-keys for recipient
if err := senderFSM.GeneratePreKeysForPeer(recipientKeyPair.Public); err != nil {
    log.Fatal(err)
}
senderPreKeyMsg, err := senderFSM.ExchangePreKeys(recipientKeyPair.Public)
if err != nil {
    log.Fatal(err)
}

// Recipient generates pre-keys for sender
if err := recipientFSM.GeneratePreKeysForPeer(senderKeyPair.Public); err != nil {
    log.Fatal(err)
}
recipientPreKeyMsg, err := recipientFSM.ExchangePreKeys(senderKeyPair.Public)
if err != nil {
    log.Fatal(err)
}

// Exchange pre-keys (in real usage, this happens over the network)
if err := senderFSM.ProcessPreKeyExchange(recipientPreKeyMsg); err != nil {
    log.Fatal(err)
}
if err := recipientFSM.ProcessPreKeyExchange(senderPreKeyMsg); err != nil {
    log.Fatal(err)
}

// Step 2: Send forward-secure message
message := "Hello, offline friend!"
fsMsg, err := senderFSM.SendForwardSecureMessage(recipientKeyPair.Public, []byte(message), async.MessageTypeNormal)
if err != nil {
    log.Fatal(err)
}

// Store the forward-secure message (storage would normally serialize this)
log.Printf("Stored forward-secure message ID: %x", fsMsg.MessageID[:8])

// Step 3: Retrieve and decrypt messages (recipient side)
// In real usage, recipient would retrieve stored forward-secure messages
decrypted, err := recipientFSM.DecryptForwardSecureMessage(fsMsg)
if err != nil {
    log.Fatal(err)
}

log.Printf("Decrypted message: %s", decrypted)
```

**Note**: The deprecated `async.EncryptForRecipient` function does not provide forward secrecy and should not be used. Always use `ForwardSecurityManager` for new applications to ensure proper forward secrecy guarantees.

### Security Considerations

- **End-to-End Encryption**: Messages are encrypted using NaCl/box with the recipient's public key
- **Forward Secrecy**: Each message uses a unique nonce for encryption
- **Peer Identity Obfuscation**: Storage nodes cannot see real sender or recipient identities (cryptographic pseudonyms)
- **Ephemeral Pseudonyms**: Sender pseudonyms are unique per message, preventing message correlation
- **Time-Based Rotation**: Recipient pseudonyms rotate every 6 hours to prevent long-term tracking
- **Anti-Spam Protection**: HMAC-based recipient proofs prevent message injection without identity knowledge
- **Storage Security**: Storage nodes cannot read message contents, only encrypted metadata
- **Fair Resource Usage**: Automatic 1% disk space allocation with 1MB-1GB bounds prevents abuse
- **Automatic Expiration**: Messages older than 24 hours are automatically deleted
- **No Single Point of Failure**: Messages are distributed across multiple automatic storage nodes

### Network Integration

The async messaging system is designed to integrate with Tox's existing network:

- **Optional Storage Participation**: Users contribute storage when async manager initialization succeeds
- **DHT Integration**: Storage nodes discovered through existing DHT network
- **Transport Layer**: Uses existing UDP/TCP transports with optional Noise-IK encryption
- **Backward Compatibility**: Regular Tox clients unaffected by async messaging nodes

### Limitations

- **Unofficial Extension**: Not part of official Tox protocol specification
- **Storage Capacity**: Limited by optional 1% disk space allocation and expiration policies
- **Network Effect**: Improved reliability with storage node participation when available
- **No Delivery Guarantees**: Best-effort delivery, messages may be lost if all storage nodes fail
- **Optional Storage Node**: If async manager initialization fails, storage node features are disabled while core Tox functionality continues

### Configuration Options

```go
// Async messaging configuration (modify constants in async package)
const (
    MaxMessageSize = 1372           // Maximum message size in bytes
    MaxStorageTime = 24 * time.Hour // Message expiration time
    MaxMessagesPerRecipient = 100   // Anti-spam limit per recipient
    
    // Storage capacity automatically calculated as 1% of available disk space
    MinStorageCapacity = 1536       // Minimum storage capacity (1MB / ~650 bytes per message)
    MaxStorageCapacity = 1536000    // Maximum storage capacity (1GB / ~650 bytes per message)
)

// Storage capacity is dynamically calculated based on available disk space:
// - Uses syscall.Statfs to detect available space
// - Allocates 1% of available space to async message storage
// - Bounded between 1MB and 1GB to ensure reasonable limits
// - Automatically updates every 5 minutes during operation
```

This async messaging system provides a foundation for offline communication while maintaining Tox's core principles of decentralization and security. The automatic storage participation ensures network resilience without requiring user configuration.

toxcore-go differs from the original C implementation in several ways:

1. **Language and Style**: Pure Go implementation with idiomatic Go patterns and error handling.
2. **Memory Management**: Uses Go's garbage collection instead of manual memory management.
3. **Concurrency**: Leverages Go's goroutines and channels for concurrent operations.
4. **API Design**: Cleaner, more consistent API following Go conventions.
5. **Simplicity**: Focused on clean, maintainable code with modern design patterns.

## Roadmap

### Feature Status Overview

toxcore-go implements the core Tox protocol with several advanced features. This section clarifies the implementation status of various components.

#### ✅ Fully Implemented Features

These features are production-ready and fully functional:

- **Core Tox Protocol**
  - Friend management (add, delete, list friends)
  - Real-time messaging with message types (normal, action)
  - Friend requests with custom messages
  - Connection status and presence
  - Name and status message management
  
- **Network Communication**
  - IPv4/IPv6 UDP and TCP transport
  - DHT peer discovery and routing
  - Bootstrap node connectivity
  - NAT traversal techniques
  - Packet encryption with NaCl crypto_box
  
- **Cryptographic Security**
  - Ed25519 digital signatures
  - Curve25519 key exchange (ECDH)
  - ChaCha20-Poly1305 AEAD encryption
  - Noise Protocol Framework (IK pattern) integration
  - Forward secrecy with pre-key system
  - Identity obfuscation for async messaging
  - Secure memory handling with automatic wiping
  
- **Advanced Features**
  - Asynchronous messaging with offline delivery
  - Message padding for traffic analysis resistance
  - Pseudonym-based storage node routing
  - State persistence (save/load Tox profile)
  - ToxAV audio/video calling infrastructure
  - **Group Chat Functionality** ⚠️ *with known limitations*
    - Group creation and management
    - Group invitations and member management
    - Group messaging with broadcast
    - **Limitation**: DHT-based group discovery is currently limited to same-process groups (see note below)
  
- **Developer Features**
  - C API bindings for cross-language use
  - Comprehensive test suite (>90% coverage)
  - Mock transport for deterministic testing
  - Detailed documentation and examples

#### 🚧 Planned Features

These features have architectural support but are not yet fully functional:

- **Privacy Network Transport** (Interface Ready, Implementation Planned)
  - Tor .onion addresses - Type system supports onion addresses, but no Tor SOCKS5 proxy integration
  - I2P .b32.i2p addresses - Address parsing implemented, SAM/BOB protocol integration pending
  - Nym .nym addresses - Address type defined, mixnet client integration planned
  - Lokinet .loki addresses - Address support ready, lokinet daemon integration planned
  
  **Current Status**: The `transport.NetworkAddress` type system can represent and validate these address types, but actual network communication over these privacy networks requires additional proxy/daemon integration. Users can create and parse these addresses, but cannot send/receive packets through these networks yet.

- **Local Network Discovery** ✅ Implemented
  - LAN peer discovery via UDP broadcast/multicast
  - Automatic peer connection without bootstrap nodes
  - Useful for local testing and air-gapped networks
  
  **Current Status**: Fully implemented in `dht/local_discovery.go`. The `LocalDiscovery` option in the Options struct defaults to `true` and enables automatic discovery of Tox peers on the local network through UDP broadcast. The implementation includes periodic announcements, peer discovery callbacks, and proper lifecycle management with goroutine-based broadcast and receive loops.

- **Group Chat DHT Discovery** (Limited Implementation)
  - Group creation and messaging fully functional
  - Group invitations work correctly
  - DHT-based discovery limited to same-process groups
  
  **Current Status**: Group chat functionality is implemented in `group/chat.go`, but DHT-based group discovery uses a local in-process registry instead of distributed DHT queries. This means:
  
  - ✅ Creating groups and inviting members works correctly
  - ✅ Sending and receiving group messages works within the group
  - ⚠️ Groups created in Process A cannot be discovered from Process B via DHT
  - ⚠️ The `Join(chatID, password)` function only works for groups in the same process
  
  **Workaround**: Applications should share group IDs and connection information through friend messages or use invitation mechanisms rather than relying on DHT-based discovery. This limitation exists because the Tox protocol's group chat DHT announcement specification is still evolving. See `group/chat.go` package documentation for detailed limitations and recommended patterns.

- **Proxy Support** (API Ready, Implementation Pending)
  - HTTP proxy configuration API exists
  - SOCKS5 proxy configuration API exists
  - Network routing through proxies not implemented
  
  **Current Status**: The `ProxyOptions` struct in the Options allows configuring HTTP and SOCKS5 proxies, but these settings are not yet used during transport setup. All network traffic uses direct UDP/TCP connections. See the "Configuration Options" section for workarounds using system-level proxy routing.

#### 📋 Future Considerations

Features under consideration for future development:

- **Enhanced Privacy Networks**
  - GarliCat integration for I2P
  - Snowflake pluggable transport for Tor
  - Additional mixnet protocols
  
- **Protocol Extensions**
  - Group chat message history synchronization
  - Multi-device synchronization
  - File transfer resumption
  - Voice message support
  
- **Performance Optimizations**
  - Connection pooling for TCP relays
  - Adaptive pre-key bundle sizes
  - Message batching for high-throughput scenarios
  - DHT query caching

### Migration Notes

When privacy network support is implemented, the existing code will continue to work without changes. The transport layer uses interface-based design, so new network types will integrate transparently:

```go
// This code works now with IPv4/IPv6 and will work with Tor/I2P once implemented
tox, err := toxcore.New(options)
if err != nil {
    log.Fatal(err)
}

// The transport layer will automatically select the appropriate protocol
// based on the bootstrap node address type
```

For users requiring Tor/I2P support today, consider using system-level SOCKS5 proxies or network namespace routing as a temporary solution.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.