# toxcore-go

A pure Go implementation of the Tox Messenger core protocol with enhanced security through Noise Protocol Framework integration.

## Overview

toxcore-go is a clean, idiomatic Go implementation of the Tox protocol, designed for simplicity, security, and performance. It provides a comprehensive, CGo-free implementation with C binding annotations for cross-language compatibility.

### Key Features

- **🔒 Enhanced Security**: Noise Protocol Framework integration providing forward secrecy and Key Compromise Impersonation (KCI) resistance
- **🔄 Backward Compatibility**: Full compatibility with legacy Tox protocol implementations
- **⚡ Automatic Protocol Selection**: Seamless switching between Noise and legacy protocols based on peer capabilities
- **🌐 Pure Go Implementation**: No CGo dependencies for maximum portability
- **📡 Comprehensive Protocol Support**: Complete implementation of Tox messaging, file transfers, and group chats
- **🎯 Clean API Design**: Idiomatic Go patterns with proper error handling and concurrency
- **🔗 C Binding Support**: Cross-language compatibility through export annotations
- **🧪 Extensive Testing**: Comprehensive test suite with integration tests for all protocols

### Security Enhancements

**Noise Protocol Framework Integration:**
- **Forward Secrecy**: Past communications remain secure even if long-term keys are compromised
- **KCI Resistance**: Protection against Key Compromise Impersonation attacks
- **Mutual Authentication**: Cryptographic proof of identity for both parties
- **Protocol Agility**: Support for multiple cryptographic suites with secure negotiation

**Dual Protocol Support:**
- **Automatic Detection**: Transparent protocol selection based on peer capabilities
- **Graceful Fallback**: Seamless fallback to legacy protocol when needed
- **Session Management**: Persistent secure sessions for ongoing communications
- **Migration Ready**: Future-proof architecture for protocol upgrades

## Installation

```bash
go get github.com/opd-ai/toxcore
```

## Quick Start

### Basic Messaging with Enhanced Security

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/toxcore"
)

func main() {
	// Create a new Tox instance with Noise protocol enabled by default
	options := toxcore.NewOptions()
	options.UDPEnabled = true
	
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatal(err)
	}
	defer tox.Kill()
	
	// Print our Tox ID
	fmt.Println("My Tox ID:", tox.SelfGetAddress())
	fmt.Println("Noise Protocol Enabled:", tox.IsNoiseEnabled())
	
	// Set up callbacks for friend requests (supports both Noise and legacy)
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		fmt.Printf("Friend request received: %s\n", message)
		
		// Automatically accept friend requests
		friendID, err := tox.AddFriendByPublicKey(publicKey)
		if err != nil {
			fmt.Printf("Error accepting friend request: %v\n", err)
		} else {
			fmt.Printf("Accepted friend request. Friend ID: %d\n", friendID)
			
			// Check if a secure Noise session was established
			if session, exists := tox.GetNoiseSession(publicKey); exists && session != nil {
				fmt.Println("✅ Secure Noise session established for enhanced security")
			} else {
				fmt.Println("⚠️  Using legacy protocol (still secure, but without forward secrecy)")
			}
		}
	})
	
	// Message callback handles both Noise-encrypted and legacy messages transparently
	tox.OnFriendMessage(func(friendID uint32, message string, messageType toxcore.MessageType) {
		fmt.Printf("Message from friend %d: %s\n", friendID, message)
		
		// Echo the message back (automatically uses best available protocol)
		err := tox.SendFriendMessage(friendID, "You said: "+message)
		if err != nil {
			fmt.Printf("Error sending message: %v\n", err)
		}
	})
	
	// Connect to a bootstrap node
	err = tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		log.Printf("Warning: Bootstrap failed: %v", err)
	}
	
	// Main loop
	fmt.Println("Running Tox with enhanced security...")
	for tox.IsRunning() {
		tox.Iterate()
		time.Sleep(tox.IterationInterval())
	}
}
```

### Advanced Usage with Protocol Selection

```go
// Check peer protocol capabilities
peerCapabilities, err := tox.GetPeerCapabilities(friendPublicKey)
if err == nil {
	if peerCapabilities.SupportsNoise {
		fmt.Println("Peer supports Noise protocol - enhanced security available")
	}
	fmt.Printf("Peer supported versions: %v\n", peerCapabilities.SupportedVersions)
}

// Manual protocol preference (usually automatic)
tox.SetProtocolPreference(toxcore.ProtocolPreferNoise)

// Monitor session status
sessions := tox.GetActiveSessions()
fmt.Printf("Active Noise sessions: %d\n", len(sessions))
		
```

## Protocol Compatibility

### Noise Protocol Framework Integration

toxcore-go implements the Noise-IK pattern for enhanced security:

```
Protocol Flow:
Alice                                    Bob
  |                                       |
  | Friend Request (with capabilities)    |
  |-------------------------------------->|
  |                                       |
  | Noise Handshake (if supported)       |
  |<------------------------------------->|
  |                                       |
  | Encrypted Messages                    |
  |<------------------------------------->|
```

**Security Properties:**
- **Forward Secrecy**: Session keys are ephemeral and destroyed after use
- **KCI Resistance**: Compromised long-term keys cannot impersonate peers
- **Mutual Authentication**: Both parties cryptographically prove their identity
- **Protocol Agility**: Support for multiple cryptographic suites

### Backward Compatibility Matrix

| Your Client | Peer Client | Protocol Used | Security Level |
|-------------|-------------|---------------|----------------|
| toxcore-go (Noise) | toxcore-go (Noise) | Noise-IK | Enhanced |
| toxcore-go (Noise) | c-toxcore (Legacy) | Legacy | Standard |
| toxcore-go (Legacy Mode) | Any | Legacy | Standard |

### Migration and Interoperability

- **Seamless Migration**: Existing Tox IDs and friend relationships are preserved
- **Automatic Detection**: Protocol selection happens transparently
- **Graceful Degradation**: Falls back to legacy protocol when needed
- **Network Compatibility**: Full compatibility with existing Tox network

## API Reference

### Core Functions

```go
// Instance Management
func New(options *Options) (*Tox, error)
func (t *Tox) Kill()
func (t *Tox) Iterate()
func (t *Tox) IsRunning() bool

// Identity and Addressing
func (t *Tox) SelfGetAddress() string
func (t *Tox) SelfGetPublicKey() [32]byte
func (t *Tox) SelfSetName(name string) error
func (t *Tox) SelfSetStatusMessage(message string) error

// Friend Management
func (t *Tox) AddFriend(address string) (uint32, error)
func (t *Tox) AddFriendMessage(address string, message string) (uint32, error)
func (t *Tox) AddFriendByPublicKey(publicKey [32]byte) (uint32, error)
func (t *Tox) DeleteFriend(friendID uint32) error
func (t *Tox) GetFriendList() []uint32

// Messaging (Automatic Protocol Selection)
func (t *Tox) SendFriendMessage(friendID uint32, message string) error
func (t *Tox) FriendSendMessage(friendID uint32, message string, messageType MessageType) (uint32, error)

// File Transfers
func (t *Tox) FileSend(friendID uint32, kind uint32, fileSize uint64, fileID [32]byte, filename string) (uint32, error)
func (t *Tox) FileControl(friendID uint32, fileID uint32, control FileControl) error
func (t *Tox) AcceptFileTransfer(friendID uint32, fileID uint32, filename string) error

// Network and Bootstrap
func (t *Tox) Bootstrap(address string, port uint16, publicKeyHex string) error
func (t *Tox) AddTcpRelay(address string, port uint16, publicKeyHex string) error

// Enhanced Security Features
func (t *Tox) IsNoiseEnabled() bool
func (t *Tox) GetNoiseSession(peerKey [32]byte) (*NoiseSession, bool)
func (t *Tox) GetPeerCapabilities(peerKey [32]byte) (*ProtocolCapabilities, error)
func (t *Tox) GetActiveSessions() map[[32]byte]*NoiseSession
func (t *Tox) SetProtocolPreference(preference ProtocolPreference)
```

### Callback Registration

```go
// Event Callbacks
func (t *Tox) OnFriendRequest(callback FriendRequestCallback)
func (t *Tox) OnFriendMessage(callback FriendMessageCallback)
func (t *Tox) OnFriendStatus(callback FriendStatusCallback)
func (t *Tox) OnConnectionStatus(callback ConnectionStatusCallback)

// File Transfer Callbacks
func (t *Tox) OnFileRecv(callback FileRecvCallback)
func (t *Tox) OnFileRecvChunk(callback FileRecvChunkCallback)
func (t *Tox) OnFileChunkRequest(callback FileChunkRequestCallback)

// Security Event Callbacks
func (t *Tox) OnNoiseSessionEstablished(callback NoiseSessionCallback)
func (t *Tox) OnProtocolNegotiation(callback ProtocolNegotiationCallback)
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

## Comparison with libtoxcore

toxcore-go offers significant improvements over the original C implementation:

### Security Enhancements
| Feature | libtoxcore | toxcore-go |
|---------|------------|------------|
| Forward Secrecy | ❌ No | ✅ Yes (Noise Protocol) |
| KCI Resistance | ❌ Vulnerable | ✅ Protected |
| Protocol Agility | ❌ Single protocol | ✅ Multiple protocols |
| Automatic Upgrades | ❌ Manual | ✅ Automatic |

### Implementation Differences
1. **Language and Style**: Pure Go implementation with idiomatic Go patterns and comprehensive error handling
2. **Memory Management**: Uses Go's garbage collection instead of manual memory management
3. **Concurrency**: Leverages Go's goroutines and channels for concurrent operations
4. **API Design**: Cleaner, more consistent API following Go conventions
5. **Security**: Enhanced cryptographic security through Noise Protocol Framework
6. **Testing**: Comprehensive test suite with integration tests for all protocols
7. **Maintainability**: Modern design patterns with clear separation of concerns

### Performance Characteristics
- **Handshake Overhead**: Slightly higher due to enhanced security (1.5x RTT vs 1x)
- **Message Throughput**: Comparable performance with additional security guarantees
- **Memory Usage**: Efficient with Go's garbage collection and session management
- **CPU Usage**: Optimized cryptographic operations with hardware acceleration support

## Testing

Run the complete test suite:

```bash
# Run all tests
make test

# Run tests with race detection
make test-race

# Run integration tests
go test -v ./... -tags=integration

# Run specific test categories
go test -v ./friend_request_unit_test.go      # Friend request tests
go test -v ./noise_integration_test.go        # Noise protocol tests
go test -v ./toxcore_test.go                  # Core functionality tests
```

### Test Coverage

- **Unit Tests**: Individual component testing with mocked dependencies
- **Integration Tests**: End-to-end protocol testing with real network simulation  
- **Security Tests**: Cryptographic property validation and attack resistance
- **Compatibility Tests**: Interoperability with legacy Tox implementations
- **Performance Tests**: Benchmarking and stress testing

## Examples

Explore practical examples in the `/examples` directory:

- **`callback_demo/`**: Basic callback usage and event handling
- **`complete_demo/`**: Full-featured Tox client implementation
- **`save_load_demo/`**: State persistence and restoration
- **`tcp_relay_demo/`**: TCP relay and NAT traversal
- **`noise_security_demo/`**: Advanced security features demonstration

## Dependencies

Core dependencies:
- **flynn/noise**: Noise Protocol Framework implementation
- **golang.org/x/crypto**: Extended cryptographic functions

No CGo dependencies - pure Go implementation for maximum portability.

## Versioning and Compatibility

- **Version**: v2.0.0+ (with Noise Protocol support)  
- **Go Version**: Requires Go 1.19 or later
- **Protocol Compatibility**: 
  - Tox Protocol v0.2.x (legacy)
  - Tox Protocol v0.3.x (with Noise enhancements)
- **Network Compatibility**: Full backward compatibility with existing Tox network

## Security

### Vulnerability Reporting

If you discover a security vulnerability, please send an email to security@toxcore-go.org. All security vulnerabilities will be promptly addressed.

### Security Features

- **End-to-End Encryption**: All communications are encrypted
- **Forward Secrecy**: Past communications remain secure
- **Identity Verification**: Cryptographic proof of identity
- **Protocol Security**: Noise Protocol Framework implementation
- **Secure Defaults**: Safe configuration out of the box

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the GPL-3.0 License - see the LICENSE file for details.