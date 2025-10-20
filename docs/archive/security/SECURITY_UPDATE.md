# Forward Secrecy Security Update

## Summary

All asynchronous messaging in the toxcore implementation now uses forward secrecy by default. Insecure APIs have been deprecated or removed to prevent accidentally insecure usage.

## Changes Made

### 1. Deprecated Insecure APIs

#### `async.EncryptForRecipient()`
- **Status**: DEPRECATED ❌
- **Replacement**: Use `ForwardSecurityManager` or `AsyncManager.SendAsyncMessage()`
- **Reason**: Does not provide forward secrecy

#### `AsyncClient.SendAsyncMessage()`
- **Status**: DEPRECATED ❌ 
- **Replacement**: Use `AsyncManager.SendAsyncMessage()`
- **Reason**: Does not use forward-secure pre-exchanged keys

### 2. Secure APIs (Forward-Secure by Default)

#### `AsyncManager.SendAsyncMessage()`
- **Status**: SECURE ✅
- **Forward Secrecy**: YES - Uses pre-exchanged one-time keys
- **Behavior**: Requires pre-key exchange between peers when online

#### `ForwardSecurityManager`
- **Status**: SECURE ✅
- **Forward Secrecy**: YES - Signal-inspired pre-key protocol
- **Features**: 100 pre-keys per peer, automatic refresh, secure key exhaustion

### 3. Required Pre-Key Exchange

Forward-secure messaging now requires:

1. **Both peers must be online** for initial pre-key exchange
2. **Automatic refresh** when peers are online together  
3. **Key exhaustion protection** - refuses to send when no pre-keys available
4. **100 one-time keys per peer** for forward secrecy

## Migration Guide

### Before (Insecure)
```go
// DEPRECATED - No forward secrecy
client := async.NewAsyncClient(keyPair)
err := client.SendAsyncMessage(recipientPK, message, messageType)
```

### After (Forward-Secure)
```go
// SECURE - Forward secrecy enabled by default
transport, _ := transport.NewUDPTransport("0.0.0.0:0") // Auto-assign port
manager, _ := async.NewAsyncManager(keyPair, transport, dataDir)
manager.Start()

// Requires pre-key exchange when both peers are online
manager.SetFriendOnlineStatus(friendPK, true)  // Triggers pre-key exchange

// Later when friend is offline - forward-secure messaging
err := manager.SendAsyncMessage(friendPK, message, messageType)
```

## Security Properties

✅ **Forward Secrecy**: Past messages remain secure even if long-term keys are compromised  
✅ **One-Time Keys**: Each message uses a unique pre-exchanged key  
✅ **Automatic Key Management**: Pre-keys refreshed automatically when peers are online  
✅ **Secure Key Exhaustion**: System refuses to send messages when pre-keys are exhausted  
✅ **Replay Protection**: Messages cannot be replayed or reused  

## Backward Compatibility

- **Tests**: All existing tests updated to use secure APIs
- **Examples**: Updated to demonstrate forward-secure messaging  
- **Documentation**: Updated to reflect forward secrecy requirements
- **Error Messages**: Clear guidance when deprecated APIs are used

## Impact

- **Security**: Significantly improved - forward secrecy now enforced by default
- **Usability**: Minimal impact - pre-key exchange is automatic when peers are online
- **Performance**: Minimal overhead - pre-keys generated and cached locally
- **Compatibility**: Breaking change for direct usage of deprecated APIs, but guided migration

---

**All asynchronous messaging in toxcore now provides forward secrecy by default. No insecure APIs are exposed.**
