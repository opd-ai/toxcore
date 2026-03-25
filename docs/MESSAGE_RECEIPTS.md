# Message Delivery Receipts Design Document

**Version**: 1.0  
**Date**: March 2026  
**Status**: Design Document (Implementation Pending)  

## Abstract

This document specifies the message delivery receipt mechanism for toxcore-go, providing applications with reliable notification when messages are delivered to and read by recipients. This feature enables acknowledgment-based messaging patterns and retry logic for reliable message delivery.

## Motivation

The current `SendFriendMessage()` returns success when a message is queued for sending, not when it is actually delivered to the recipient. Applications have no way to know:

1. Whether the message was successfully transmitted
2. Whether the message was received by the friend's client
3. Whether the friend has read the message

This leads to poor user experience in messaging applications where users expect delivery confirmation.

## Design Goals

- **Tox Protocol Compatibility**: Use existing Tox protocol mechanisms where possible
- **Minimal Overhead**: Receipts should add minimal network traffic
- **Application Flexibility**: Provide callbacks for delivery and read receipts separately
- **Retry Support**: Enable configurable automatic retry for undelivered messages
- **Backward Compatible**: Work with existing Tox clients that may not support receipts

## Packet Format

### Delivery Receipt Packet (Type: 0x63)

```
Delivery Receipt Packet:
+-------------------+
| Message ID (4B)   |  uint32 - ID from original message
+-------------------+
| Receipt Type (1B) |  0x00 = delivered, 0x01 = read
+-------------------+
```

**Note**: This aligns with the Tox protocol specification for message receipts.

## Callback Interface

### MessageDeliveryCallback

```go
// MessageDeliveryCallback is called when a delivery receipt is received.
// messageID is the ID returned by SendFriendMessage.
// state indicates the delivery state.
type MessageDeliveryCallback func(friendID uint32, messageID uint32, state MessageDeliveryState)

// MessageDeliveryState indicates the delivery state of a message.
type MessageDeliveryState uint8

const (
    DeliveryStatePending   MessageDeliveryState = iota  // Message queued for sending
    DeliveryStateSent                                    // Message sent, no receipt yet
    DeliveryStateDelivered                               // Friend's client received message
    DeliveryStateRead                                    // Friend has read the message
    DeliveryStateFailed                                  // Delivery failed after retries
)
```

### Registration Methods

```go
// OnMessageDelivery registers a callback for message delivery status changes.
func (t *Tox) OnMessageDelivery(callback MessageDeliveryCallback)

// GetMessageDeliveryStatus returns the current delivery status of a message.
// Returns DeliveryStatePending for unknown message IDs.
func (t *Tox) GetMessageDeliveryStatus(friendID uint32, messageID uint32) MessageDeliveryState
```

## Pending Message Tracking

### Data Structure

```go
type pendingMessage struct {
    friendID    uint32
    messageID   uint32
    message     []byte
    messageType MessageType
    sentAt      time.Time
    retries     int
    maxRetries  int
    retryAfter  time.Duration
    status      MessageDeliveryState
}

type pendingMessageStore struct {
    mu       sync.RWMutex
    messages map[uint64]*pendingMessage  // key: friendID<<32 | messageID
    maxAge   time.Duration               // remove after this duration
}
```

### Cleanup Strategy

- Messages are removed from tracking after:
  1. Receipt received (DeliveryReceived or DeliveryRead)
  2. Max retries exceeded (DeliveryFailed)
  3. TTL expired (default: 24 hours)
- Periodic cleanup goroutine runs every minute

## Retry Logic

### Configuration

```go
type DeliveryRetryConfig struct {
    Enabled        bool          // Enable automatic retry (default: true)
    MaxRetries     int           // Maximum retry attempts (default: 3)
    InitialDelay   time.Duration // First retry delay (default: 5s)
    MaxDelay       time.Duration // Maximum retry delay (default: 5m)
    BackoffFactor  float64       // Exponential backoff multiplier (default: 2.0)
}
```

### Retry Algorithm

```
delay = min(InitialDelay * (BackoffFactor ^ retryCount), MaxDelay)
```

Example with defaults:
- Retry 1: 5 seconds
- Retry 2: 10 seconds
- Retry 3: 20 seconds
- Failed: Callback with DeliveryFailed

### Retry Conditions

Retry only when:
1. Friend is online (ConnectionStatus != None)
2. Previous attempt timed out (no receipt within timeout)
3. Retry count < MaxRetries

Do NOT retry when:
1. Receipt received (success)
2. Friend offline for >5 minutes
3. Message TTL expired

## Integration Points

### SendFriendMessage Enhancement

```go
func (t *Tox) SendFriendMessage(friendID uint32, message string, messageType ...MessageType) error {
    // ... existing validation ...
    
    messageID := t.getNextMessageID()
    
    // Track pending message if delivery callbacks registered
    if t.deliveryCallback != nil {
        t.pendingMessages.add(friendID, messageID, []byte(message), messageType)
    }
    
    // ... existing send logic ...
    
    return nil
}
```

### Packet Handler

```go
func (t *Tox) handleDeliveryReceipt(friendID uint32, data []byte) {
    if len(data) < 5 {
        return  // Invalid packet
    }
    
    messageID := binary.BigEndian.Uint32(data[0:4])
    receiptType := data[4]
    
    var status DeliveryStatus
    switch receiptType {
    case 0x00:
        status = DeliveryReceived
    case 0x01:
        status = DeliveryRead
    default:
        return  // Unknown receipt type
    }
    
    // Update pending message status
    t.pendingMessages.updateStatus(friendID, messageID, status)
    
    // Fire callback
    t.callbackMu.RLock()
    cb := t.deliveryCallback
    t.callbackMu.RUnlock()
    
    if cb != nil {
        cb(friendID, messageID, status)
    }
}
```

## Implementation Phases

### Phase 1: Basic Delivery Receipts
- [ ] Add `MessageDeliveryCallback` type and registration
- [ ] Implement pending message tracking
- [ ] Handle incoming receipt packets
- [ ] Fire callbacks on receipt

### Phase 2: Automatic Retry
- [ ] Add `DeliveryRetryConfig` to options
- [ ] Implement retry goroutine
- [ ] Handle retry with exponential backoff
- [ ] Fire DeliveryFailed callback after max retries

### Phase 3: Read Receipts
- [ ] Send read receipt when message displayed
- [ ] API: `MarkMessageAsRead(friendID, messageID)`
- [ ] Handle incoming read receipts

## Testing Strategy

### Unit Tests
- `TestDeliveryCallbackRegistration`
- `TestPendingMessageTracking`
- `TestDeliveryReceiptPacketParsing`
- `TestRetryWithExponentialBackoff`
- `TestMaxRetryExceeded`
- `TestTTLExpiration`

### Integration Tests
- `TestMessageDeliveryE2E` — Send message, verify receipt callback
- `TestMessageRetryOnTimeout` — Simulate timeout, verify retry
- `TestReadReceiptE2E` — Verify read receipt flow

## Security Considerations

1. **Receipt Spoofing**: Receipts are authenticated via the Tox friend protocol (encrypted channel)
2. **Replay Protection**: Message IDs are unique per friend session
3. **Privacy**: Read receipts are optional; some users may disable them
4. **DoS**: Rate limit receipt processing to prevent flooding

## API Summary

```go
// Callback types
type MessageDeliveryCallback func(friendID uint32, messageID uint32, status DeliveryStatus)
type DeliveryStatus int

// Registration
func (t *Tox) OnMessageDelivery(callback MessageDeliveryCallback)

// Query
func (t *Tox) GetMessageDeliveryStatus(friendID uint32, messageID uint32) DeliveryStatus

// Configuration (via NewOptions)
type Options struct {
    // ... existing fields ...
    DeliveryRetryEnabled  bool
    DeliveryMaxRetries    int
    DeliveryRetryDelay    time.Duration
}
```

## Compatibility Notes

- **c-toxcore**: Compatible — uses same packet type 0x63
- **uTox/qTox**: Compatible — implement same receipt mechanism
- **Older clients**: Gracefully degrade — no receipt means DeliveryPending forever

## References

- [Tox Protocol Specification - Message Receipts](https://toktok.ltd/spec.html#message-receipts)
- toxcore-go `messaging/message.go` for current implementation
- c-toxcore `toxcore/friend_requests.c` for reference implementation
