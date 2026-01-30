# Friend Callbacks Demo

This example demonstrates the use of friend status callbacks in toxcore-go:

## Callbacks Demonstrated

### OnFriendConnectionStatus
Called whenever a friend's connection status changes between:
- `ConnectionNone` (offline)
- `ConnectionUDP` (connected via UDP)
- `ConnectionTCP` (connected via TCP)

This callback is triggered for **all** connection status changes.

### OnFriendStatusChange
Called when a friend transitions between online and offline states:
- `online=true`: Friend has come online (any connection type)
- `online=false`: Friend has gone offline

This callback is triggered only for **online/offline transitions**, not for UDP↔TCP switches.

## Usage Example

```go
// Monitor all connection status changes
tox.OnFriendConnectionStatus(func(friendID uint32, status toxcore.ConnectionStatus) {
    fmt.Printf("Friend %d status: %v\n", friendID, status)
})

// React to online/offline events
tox.OnFriendStatusChange(func(friendPK [32]byte, online bool) {
    if online {
        fmt.Printf("Friend %x came online\n", friendPK)
        // Good place to send queued messages
    } else {
        fmt.Printf("Friend %x went offline\n", friendPK)
    }
})
```

## Running the Demo

```bash
go run main.go
```

## Expected Output

The demo shows three scenarios:
1. Friend comes online (UDP) - both callbacks fired
2. Friend switches to TCP - only OnFriendConnectionStatus fired
3. Friend goes offline - both callbacks fired

This demonstrates how `OnFriendStatusChange` filters for online/offline transitions while `OnFriendConnectionStatus` reports all status changes.
