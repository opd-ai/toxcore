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

**Requirements:** Go 1.25.0 or later

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
		
		// Accept this friend request using AddFriendByPublicKey
		// Note: Use AddFriend(toxID, message) to SEND requests, and
		// AddFriendByPublicKey(publicKey) to ACCEPT incoming requests
		friendID, err := tox.AddFriendByPublicKey(publicKey)
		if err != nil {
			fmt.Printf("Error accepting friend request: %v\n", err)
		} else {
			fmt.Printf("Accepted friend request. Friend ID: %d\n", friendID)
		}
	})
	
	tox.OnFriendMessage(func(friendID uint32, message string) {
		fmt.Printf("Message from friend %d: %s\n", friendID, message)
		
		// Echo the message back as a normal message (default)
		tox.SendFriendMessage(friendID, "You said: "+message)
		
		// Or send an action message
		// tox.SendFriendMessage(friendID, "received your message", toxcore.MessageTypeAction)
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

## Bootstrap Node Connectivity

Connecting to the Tox DHT network requires bootstrapping to at least one known node.

```go
// Basic bootstrap
err := tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
if err != nil {
    log.Printf("Bootstrap warning: %v", err)
    // Don't treat as fatal - LAN discovery or existing friends may still connect
}
```

The `Options.BootstrapTimeout` setting controls how long to wait for initial DHT connectivity (default: 30 seconds). For production reliability, attempt multiple bootstrap nodes with fallback:

```go
bootstrapNodes := []struct {
    Host   string
    Port   uint16
    PubKey string
}{
    {"node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"},
    {"tox.verdict.gg", 33445, "1C5293AEF2114717547B39DA8EA6F1E331E5E358B35F9B6B5F19317911C5F976"},
    {"tox.initramfs.io", 33445, "3F0A45A268367C1BEA652F258C85F4A66DA76BCAA667A49E770BCC4917AB6A25"},
}

for _, node := range bootstrapNodes {
    if err := tox.Bootstrap(node.Host, node.Port, node.PubKey); err == nil {
        log.Printf("Successfully bootstrapped to %s", node.Host)
        break
    }
}
```

Monitor connection status via callback:

```go
tox.OnSelfConnectionStatus(func(status toxcore.ConnectionStatus) {
    switch status {
    case toxcore.ConnectionNone:
        log.Println("Disconnected from DHT network")
    case toxcore.ConnectionTCP:
        log.Println("Connected via TCP relay")
    case toxcore.ConnectionUDP:
        log.Println("Connected via UDP (optimal)")
    }
})
```

## Configuration Options

### Proxy Support

toxcore-go includes proxy configuration options in the `Options` struct for HTTP and SOCKS5 proxies:

```go
options := toxcore.NewOptions()
options.Proxy = &toxcore.ProxyOptions{
    Type:            toxcore.ProxyTypeSOCKS5,
    Host:            "127.0.0.1",
    Port:            9050,
    Username:        "",   // Optional
    Password:        "",   // Optional
    UDPProxyEnabled: true, // Enable SOCKS5 UDP ASSOCIATE for UDP traffic
}
```

**Current Status**: The proxy configuration API is **fully implemented** for SOCKS5 proxies:
- TCP connections are routed through the proxy (HTTP/SOCKS5)
- UDP connections can be routed through SOCKS5 proxies using UDP ASSOCIATE (RFC 1928) by setting `UDPProxyEnabled: true`

When `UDPProxyEnabled` is true and a SOCKS5 proxy is configured, all UDP traffic (including DHT operations) will be relayed through the proxy, protecting your real IP address.

**Note**: HTTP proxies only support TCP. For UDP proxy support, use a SOCKS5 proxy with `UDPProxyEnabled: true`. The Tor network itself does not support UDP, so even with a SOCKS5 proxy to Tor, UDP traffic cannot be tunneled through Tor's onion routing.

## Multi-Network Support

toxcore-go includes a multi-network address system with IPv4/IPv6 support and architecture for privacy networks.

### Supported Network Types

| Network | Listen | Dial | UDP | Notes |
|---------|--------|------|-----|-------|
| **IPv4/IPv6** | ✅ | ✅ | ✅ | Traditional internet protocols, fully implemented |
| **Tor .onion** | ✅ | ✅ | ❌ | TCP only via onramp; UDP not supported (Tor protocol limitation) |
| **I2P .b32.i2p** | ✅ | ✅ | ❌ | Full SAM bridge integration; TCP only |
| **Lokinet .loki** | ❌ | ✅ | ❌ | TCP Dial only via SOCKS5 proxy; Listen support is low priority and blocked by immature Lokinet SDK |
| **Nym .nym** | ❌ | ✅ | ❌ | Dial only via SOCKS5 proxy; Listen support is low priority and blocked by immature Nym SDK |

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

toxcore-go implements the Noise-IK (Initiator with Knowledge) pattern for forward secrecy, KCI resistance, and mutual authentication. Noise-IK requires explicit configuration and is disabled by default.

```go
// Wrap existing transport with Noise encryption
keyPair, _ := crypto.GenerateKeyPair()
udpTransport, _ := transport.NewUDPTransport("127.0.0.1:8080")
noiseTransport, _ := transport.NewNoiseTransport(udpTransport, keyPair.Private[:])
defer noiseTransport.Close()

// Add known peers — handshakes happen automatically
noiseTransport.AddPeer(peerAddr, peerPublicKey[:])

// Send encrypted messages transparently
noiseTransport.Send(packet, peerAddr)
```

The implementation supports automatic handshakes, transparent encryption for known peers, and fallback to unencrypted for unknown peers.

## Version Negotiation and Backward Compatibility

toxcore-go includes automatic protocol version negotiation with two protocol versions:
- **Legacy (v0)**: Original Tox protocol for backward compatibility
- **Noise-IK (v1)**: Enhanced security with forward secrecy and KCI resistance

```go
// Secure default: Noise-IK required
capabilities := transport.DefaultProtocolCapabilities()

// Create negotiating transport
negotiatingTransport, err := transport.NewNegotiatingTransport(udp, capabilities, staticKey)

// Use like any transport - version negotiation is automatic per-peer
err = negotiatingTransport.Send(packet, peerAddr)
```

> ⚠️ **Security Warning**: `EnableLegacyFallback: true` allows MITM downgrade attacks. Only enable for interoperability with legacy c-toxcore peers.

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
- Friend must exist to send messages

**Message Delivery Behavior:**
- **Friend Online:** Messages are delivered immediately via real-time messaging
- **Friend Offline:** Messages automatically fall back to asynchronous messaging for store-and-forward delivery when the friend comes online
- If async messaging is unavailable (no pre-keys exchanged), an error is returned

**Example:** The message "Hello 🎉" contains 7 characters but uses 10 UTF-8 bytes (6 for "Hello " + 4 for the emoji).

## Self Management API

```go
// Set your display name (max 128 bytes UTF-8)
err := tox.SelfSetName("Alice")

// Set your status message (max 1007 bytes UTF-8)
err = tox.SelfSetStatusMessage("Available for chat 💬")

// Get current values
name := tox.SelfGetName()
statusMsg := tox.SelfGetStatusMessage()
```

Names and status messages persist across restarts via savedata, support full UTF-8, and are immediately visible to connected friends.

### Nospam Management

The nospam value is part of your Tox ID and can be changed to create a new Tox ID while keeping the same cryptographic identity:

```go
nospam := tox.SelfGetNospam()
newNospam := [4]byte{0x12, 0x34, 0x56, 0x78}
tox.SelfSetNospam(newNospam) // Changes your Tox ID
```

Existing friends are unaffected by nospam changes (they use your public key). New friend requests must use your updated Tox ID.

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

## Group Chat (Conference) API

toxcore-go provides group chat functionality through the conference APIs on the main Tox object and the `group.Chat` interface.

### Creating and Managing Conferences

```go
// Create a new conference (group chat)
conferenceID, err := tox.ConferenceNew()
if err != nil {
    log.Printf("Failed to create conference: %v", err)
}

// Invite a friend to the conference
err = tox.ConferenceInvite(friendID, conferenceID)
if err != nil {
    log.Printf("Failed to invite friend: %v", err)
}

// Send a message to the conference
err = tox.ConferenceSendMessage(conferenceID, "Hello everyone!", toxcore.MessageTypeNormal)
if err != nil {
    log.Printf("Failed to send message: %v", err)
}

// Leave and delete the conference
err = tox.ConferenceDelete(conferenceID)
if err != nil {
    log.Printf("Failed to delete conference: %v", err)
}
```

### Group Chat Callbacks

The `group.Chat` interface provides callbacks for receiving group events. Access the underlying `group.Chat` via `ValidateConferenceAccess()`:

```go
import "github.com/opd-ai/toxcore/group"

// Access the group.Chat for a conference
chat, err := tox.ValidateConferenceAccess(conferenceID)
if err != nil {
    log.Printf("Failed to access conference: %v", err)
}

// Register callback for receiving group messages
chat.OnMessage(func(groupID, peerID uint32, message string) {
    fmt.Printf("[Group %d] Peer %d: %s\n", groupID, peerID, message)
})

// Register callback for peer changes (join/leave/name change)
chat.OnPeerChange(func(groupID, peerID uint32, changeType group.PeerChangeType) {
    switch changeType {
    case group.PeerChangeJoined:
        fmt.Printf("Peer %d joined group %d\n", peerID, groupID)
    case group.PeerChangeLeft:
        fmt.Printf("Peer %d left group %d\n", peerID, groupID)
    case group.PeerChangeNameChanged:
        fmt.Printf("Peer %d in group %d changed name\n", peerID, groupID)
    }
})

// Register callback for auto-discovered peers
chat.OnPeerDiscovered(func(groupID, peerID uint32, peer *group.Peer) {
    fmt.Printf("Discovered peer %d (%s) in group %d\n", peerID, peer.Name, groupID)
})
```

### Available Group Callbacks

| Callback | Signature | Description |
|----------|-----------|-------------|
| `OnMessage` | `func(groupID, peerID uint32, message string)` | Called when a message is received in the group |
| `OnPeerChange` | `func(groupID, peerID uint32, changeType PeerChangeType)` | Called when a peer joins, leaves, or changes name |
| `OnPeerDiscovered` | `func(groupID, peerID uint32, peer *Peer)` | Called when a peer is auto-discovered |

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

toxcore-go supports saving and restoring your Tox state (private key, friends list) across restarts.

```go
// Save state
savedata := tox.GetSavedata()
err := os.WriteFile("tox_state.dat", savedata, 0600)

// Restore from saved state
savedata, err := os.ReadFile("tox_state.dat")
if err == nil {
    tox, err = toxcore.NewFromSavedata(nil, savedata)
} else {
    tox, err = toxcore.New(toxcore.NewOptions())
}
```

Alternatively, load via Options:

```go
options := &toxcore.Options{
    UDPEnabled:     true,
    SavedataType:   toxcore.SaveDataTypeToxSave,
    SavedataData:   savedata,
    SavedataLength: uint32(len(savedata)),
}
tox, err := toxcore.New(options)
```

**Important**: The savedata contains your private key — use appropriate file permissions (0600) and consider encrypting it.

## Audio/Video Calls with ToxAV

ToxAV enables secure peer-to-peer audio and video calling. Create a ToxAV instance from an existing Tox instance:

```go
tox, err := toxcore.New(options)
if err != nil {
    log.Fatal(err)
}
defer tox.Kill()

toxav, err := toxcore.NewToxAV(tox)
if err != nil {
    log.Fatal(err)
}
defer toxav.Kill()

// Set up call callbacks
toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
    log.Printf("📞 Incoming call from friend %d", friendNumber)
    toxav.Answer(friendNumber, 64000, 500000) // 64kbps audio, 500kbps video
})

toxav.CallbackCallState(func(friendNumber uint32, state av.CallState) {
    log.Printf("Call state changed: %v", state)
})

// Both instances need iterate() calls
for tox.IsRunning() {
    tox.Iterate()
    toxav.Iterate()
    time.Sleep(tox.IterationInterval())
}
```

### Making Calls

```go
// Voice chat (audio only)
toxav.Call(friendNumber, 64000, 0)

// Video call
toxav.Call(friendNumber, 64000, 1000000) // 64kbps audio + 1Mbps video
```

### Receiving Frames

```go
toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, 
    sampleCount int, channels uint8, samplingRate uint32) {
    // Process received audio
})

toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, 
    y, u, v []byte, yStride, uStride, vStride int) {
    // Process received video (YUV420 format)
})
```

### Known Limitations

- **VP8 Key Frames Only**: Current VP8 encoder produces only I-frames, resulting in ~5-10x higher bandwidth vs full VP8 with P-frames. The pure-Go `opd-ai/vp8` library does not yet support P-frame encoding.
- **Audio Codec**: Opus encoding uses VoIP application mode optimized for voice clarity.

See the **[ToxAV Examples README](examples/ToxAV_Examples_README.md)** for complete examples with audio generation, video patterns, and effects processing.

## Asynchronous Message Delivery System (Unofficial Extension)

toxcore-go includes an experimental asynchronous message delivery system for offline messaging while maintaining decentralization and security. This is an **unofficial extension** of the Tox protocol.

### Overview

Messages to offline friends are temporarily stored on distributed storage nodes until the recipient comes online. All messages maintain end-to-end encryption and forward secrecy. **By default, users automatically participate as storage nodes** (contributing 1% of available disk space). Set `options.AsyncStorageEnabled = false` to opt out.

**Privacy**: The system uses cryptographic peer identity obfuscation — storage nodes see only cryptographic pseudonyms, not real identities. Messages are automatically padded (256B, 1024B, 4096B, 16384B buckets) to resist traffic analysis.

### Basic Usage

```go
// Simplified API via main Tox interface
options := toxcore.NewOptions()
tox, err := toxcore.New(options)
if err != nil {
    log.Fatal(err)
}
defer tox.Kill()

// Register callback for offline messages
tox.OnAsyncMessage(func(senderPK [32]byte, message string, messageType async.MessageType) {
    fmt.Printf("📨 Received offline message from %x: %s\n", senderPK[:8], message)
})

// Regular messaging automatically falls back to async when friend is offline
err = tox.SendFriendMessage(friendID, "Hello! I'll wait for you to come online.")
```

### Direct AsyncManager API

For advanced control over message storage with forward secrecy:

```go
keyPair, _ := crypto.GenerateKeyPair()
transport, _ := transport.NewUDPTransport("0.0.0.0:0")
asyncManager, _ := async.NewAsyncManager(keyPair, transport, "/path/to/data")
asyncManager.Start()
defer asyncManager.Stop()

// Send async message
asyncManager.SendAsyncMessage(friendPK, "Hello!", async.MessageTypeNormal)

// Monitor storage
stats := asyncManager.GetStorageStats()
log.Printf("Storage: %d messages", stats.TotalMessages)
```

### Privacy Features (Automatic)

- **Sender Anonymity**: Random, unlinkable pseudonyms per message
- **Recipient Anonymity**: Time-rotating pseudonyms (6-hour epochs)
- **Forward Secrecy**: One-time pre-keys consumed per message, auto-refreshed below threshold (20 remaining)
- **Zero Configuration**: Privacy protection works automatically

> **Note**: Pre-keys protect **message confidentiality** against key compromise, while epochs protect **metadata privacy** from storage nodes. See [docs/FORWARD_SECRECY.md](docs/FORWARD_SECRECY.md) for details.

### Configuration

```go
// Key constants (async package)
MaxMessageSize          = 1372           // Maximum message size in bytes
MaxStorageTime          = 24 * time.Hour // Message expiration
MaxMessagesPerRecipient = 100            // Anti-spam limit
// Storage capacity: 1% of available disk, bounded 1MB-1GB, auto-updates every 5 minutes
```

### Limitations

- **Unofficial Extension**: Not part of official Tox protocol specification
- **Best-Effort Delivery**: Messages may be lost if all storage nodes fail
- **Storage Capacity**: Limited by 1% disk allocation and 24h expiration

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the full goal-achievement assessment and priority roadmap.

### Feature Status Overview

#### ✅ Fully Implemented
- **Core Protocol**: Friend management, messaging, file transfers, group chat, state persistence
- **Network**: IPv4/IPv6 UDP/TCP, DHT routing, bootstrap, NAT traversal (TCP relay), LAN discovery
- **Cryptography**: Ed25519, Curve25519, ChaCha20-Poly1305, Noise-IK, forward secrecy, identity obfuscation
- **ToxAV**: Audio (Opus) and video (VP8) calling infrastructure
- **Async Messaging**: Offline delivery with distributed storage and privacy protection
- **C API Bindings**: 63 functions (~79% API coverage)

#### ⚠️ Partial Support
- **Lokinet .loki**: TCP Dial only (Listen blocked by immature SDK)
- **Nym .nym**: TCP Dial only (Listen blocked by immature SDK)
- **VP8 Video**: Key frames only (~5-10x bandwidth overhead)

#### 📋 Future Considerations
- GarliCat/Snowflake transport integration
- Group chat history sync, multi-device sync
- Connection pooling, message batching, DHT query caching

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.