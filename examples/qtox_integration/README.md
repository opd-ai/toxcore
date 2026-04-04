# qTox Integration Example

This example demonstrates how to use toxcore-go to communicate with qTox, the most popular Tox client. It covers the complete workflow from bootstrap to message exchange.

## Overview

toxcore-go is designed to be compatible with the c-toxcore reference implementation used by qTox. This example shows:

1. **Proper Bootstrap Sequence** - Connecting to the DHT network with timeout handling and fallbacks
2. **Friend Request/Accept Flow** - Adding qTox users as friends and accepting their requests
3. **Message Exchange** - Sending and receiving messages with a qTox client

## Prerequisites

- Go 1.22 or later
- qTox installed on another device or computer (download from [tox.chat](https://tox.chat/download.html))

## Quick Start

```bash
# Run this example
go run main.go
```

The program will:
1. Display your Tox ID (share this with qTox user)
2. Wait for friend requests from qTox
3. Auto-accept requests and enable messaging
4. Echo back any messages received

## Testing with qTox

### Step 1: Get Your Tox ID

When you run the example, it will print:

```
Your Tox ID: ABCD1234...5678
Share this with qTox to add you as a friend
```

### Step 2: Add as Friend in qTox

1. Open qTox
2. Click the "Add friend" button (person icon with +)
3. Paste the Tox ID from Step 1
4. Add a message (optional) and click "Send friend request"

### Step 3: Exchange Messages

Once connected:
- Send a message from qTox to this example
- The example will echo back your message
- You'll see all messages logged in the console

## API Compatibility Notes

toxcore-go has some behavioral differences from c-toxcore:

| Feature | c-toxcore | toxcore-go | Notes |
|---------|-----------|------------|-------|
| Address format | 38 binary bytes | 38 binary bytes | Compatible |
| Friend numbers | 0-based | 1-based | First friend is #1 |
| Status tracking | Tracks status | No-op (planned) | Set functions work but get returns 0 |
| Connection | TCP/UDP | TCP/UDP + Noise-IK | Enhanced security |

## Configuration

You can modify the example to customize:

- **Bootstrap nodes**: Edit the `bootstrapNodes` slice
- **Auto-accept**: Change `autoAcceptFriends` to require manual approval
- **Echo behavior**: Modify the `OnFriendMessage` callback

## Troubleshooting

### "Connection timeout" on bootstrap
- Try running with `options.BootstrapTimeout = 60 * time.Second`
- The example already tries multiple bootstrap nodes automatically

### Friend request not appearing
- Verify both sides are connected to the DHT (status shows "Connected")
- Check that Tox IDs are copied exactly (76 hex characters)
- Try TCP relay: Ensure your firewall allows outbound connections

### Messages not delivering
- Both peers must be online simultaneously
- Check `OnFriendConnectionStatus` callback for disconnection events

## Code Structure

```
examples/qtox_integration/
├── main.go           # Complete example with all features
└── README.md         # This file
```

## Related Documentation

- [Bootstrap Node Connectivity](../../README.md#bootstrap-node-connectivity) - Robust bootstrap patterns
- [Friend Management API](../../README.md#friend-management-api) - Full friend API reference
- [C API Usage](../../README.md#c-api-usage) - For direct c-toxcore compatibility
