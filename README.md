# toxcore-go

A pure Go implementation of the Tox Messenger core protocol.

## Overview

toxcore-go is a clean, idiomatic Go implementation of the Tox protocol, designed for simplicity, security, and performance. It provides a comprehensive, CGo-free implementation with C binding annotations for cross-language compatibility.

Key features:
- Pure Go implementation with no CGo dependencies
- Comprehensive implementation of the Tox protocol
- Clean API design with proper Go idioms
- C binding annotations for cross-language use
- Robust error handling and concurrency patterns

## Installation

```bash
go get github.com/opd-ai/toxcore
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
		
		// Echo the message back
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
- Maximum message length is 1372 bytes
- Friend must exist and be connected to receive messages

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

## Friend Management API

toxcore-go provides multiple ways to add friends depending on your use case:

```go
// Accept a friend request (use in OnFriendRequest callback)
friendID, err := tox.AddFriendByPublicKey(publicKey)

// Send a friend request with a message
friendID, err := tox.AddFriend("76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37B13349", "Hello!")
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

## Comparison with libtoxcore

toxcore-go differs from the original C implementation in several ways:

1. **Language and Style**: Pure Go implementation with idiomatic Go patterns and error handling.
2. **Memory Management**: Uses Go's garbage collection instead of manual memory management.
3. **Concurrency**: Leverages Go's goroutines and channels for concurrent operations.
4. **API Design**: Cleaner, more consistent API following Go conventions.
5. **Simplicity**: Focused on clean, maintainable code with modern design patterns.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the GPL-3.0 License - see the LICENSE file for details.