# Implementation Gap Analysis
Generated: 2026-01-30T22:03:30Z  
Codebase Version: 0cf04e88ab278f64d30a00529a694d7d074afa6c

## Executive Summary
Total Gaps Found: 5
- Critical: 0
- Moderate: 3
- Minor: 2

**Status:**
- ✅ Completed: 3 (Gaps #1, #2, #4)
- 🔧 Remaining: 2 (Gaps #3, #5)

This audit focuses on subtle discrepancies between the README.md documentation and the actual implementation. The codebase is mature and well-tested, so findings are nuanced behavioral differences rather than major missing features.

---

## Detailed Findings

### Gap #1: Message Padding Size Buckets Documentation Inconsistency
**Severity:** Minor
**Status:** ✅ COMPLETED

**Documentation Reference:**
> "Messages are now automatically padded to standard sizes (256B, 1024B, 4096B, 16384B)" (README.md:1309)

**Implementation Location:** `async/message_padding.go:14-19`

**Expected Behavior:** README implies four size buckets: 256B, 1024B, 4096B, 16384B

**Actual Implementation:** Implementation matches README correctly with four buckets:
```go
const (
    MessageSizeSmall  = 256
    MessageSizeMedium = 1024
    MessageSizeLarge  = 4096
    MessageSizeMax    = 16384
```

However, other documentation files show inconsistent bucket sizes:
- `docs/SECURITY_AUDIT_REPORT.md:2109`: "Messages padded to fixed sizes (256B, 1KB, 4KB)" - missing 16KB
- `docs/SECURITY_AUDIT_REPORT.md:3166`: "fixed-size padding to 256B/1024B/4096B buckets" - missing 16KB

**Resolution:**
Updated `docs/SECURITY_AUDIT_REPORT.md` to consistently reference all four padding buckets (256B, 1KB, 4KB, 16KB) at both locations (lines 2109 and 3166). The documentation now accurately reflects the implementation's privacy guarantees.

**Changes Made:**
- Line 2109: Updated from "256B, 1KB, 4KB" to "256B, 1KB, 4KB, 16KB"
- Line 3166: Updated from "256B/1024B/4096B buckets" to "256B/1024B/4096B/16384B buckets"

**Production Impact:** Minor - users reading security documentation may have incomplete understanding of padding sizes, but the implementation provides correct protection.

---

### Gap #2: OnFriendConnectionStatus Callback Not Documented
**Severity:** Moderate
**Status:** ✅ COMPLETED

**Documentation Reference:**
> The README documents callbacks including:
> - `OnFriendRequest` (README.md:63)
> - `OnFriendMessage` (README.md:75)
> - `OnFriendName` (README.md:3147)
> - `OnFriendStatusMessage` (README.md:3153)
> - `OnFriendTyping` (README.md:3162)

**Implementation Location:** `toxcore.go:1666`, `toxcore.go:1617`

**Resolution:**
Implemented `OnFriendConnectionStatus` callback that is triggered whenever a friend's connection status changes between None, UDP, or TCP. The callback provides both the friend ID and the new connection status.

**Implementation Details:**
- Added `FriendConnectionStatusCallback` type at `toxcore.go:1623`
- Added `OnFriendConnectionStatus` method at `toxcore.go:1683`
- Modified `SetFriendConnectionStatus` to trigger the callback when status changes
- Comprehensive tests added in `friend_callbacks_test.go`

**Evidence:**
```go
// toxcore.go:1623
type FriendConnectionStatusCallback func(friendID uint32, connectionStatus ConnectionStatus)

// toxcore.go:1683
func (t *Tox) OnFriendConnectionStatus(callback FriendConnectionStatusCallback) {
    t.friendConnectionStatusCallback = callback
}
```

---

### Gap #3: SendFriendMessage Documentation-Implementation Behavioral Mismatch
**Severity:** Moderate

**Documentation Reference:**
> "Returns an error if: ... The friend is not connected" (README.md:1810)

**Implementation Location:** `toxcore.go:1930-1942`

**Expected Behavior:** According to README, `SendFriendMessage` should return an error if the friend is not connected.

**Actual Implementation:** When a friend is offline, the implementation falls back to async messaging rather than returning an error:

```go
// toxcore.go:1930-1942
func (t *Tox) sendMessageToManager(friendID uint32, message string, msgType MessageType) error {
    friend, err := t.validateAndRetrieveFriend(friendID)
    if err != nil {
        return err
    }

    if friend.ConnectionStatus != ConnectionNone {
        return t.sendRealTimeMessage(friendID, message, msgType)
    } else {
        return t.sendAsyncMessage(friend.PublicKey, message, msgType)  // Falls back to async, doesn't error
    }
}
```

**Gap Details:** The README documents that `SendFriendMessage` returns an error when the friend is not connected, but the actual implementation silently attempts async delivery. The error only occurs if:
1. The friend doesn't exist, OR
2. Async messaging fails (e.g., no pre-keys available)

This is actually a feature (async messaging support), but the documentation doesn't reflect this behavior.

**Reproduction:**
```go
// Create friend who is offline (ConnectionStatus == ConnectionNone)
friendID, _ := tox.AddFriendByPublicKey(pubKey)

// According to README, this should error with "friend is not connected"
err := tox.SendFriendMessage(friendID, "Hello")

// Actual behavior: May succeed (async delivery queued) or fail with different error
// "friend is not connected and secure messaging keys are not available"
```

**Production Impact:** Moderate - Applications following README documentation may expect errors that don't occur, or may not realize messages are being queued for async delivery. This could lead to UX confusion if users think messages failed when they were actually queued.

**Evidence:**
```go
// toxcore.go:1972-1987
func (t *Tox) sendAsyncMessage(publicKey [32]byte, message string, msgType MessageType) error {
    // Friend is offline - use async messaging
    if t.asyncManager != nil {
        asyncMsgType := async.MessageType(msgType)
        err := t.asyncManager.SendAsyncMessage(publicKey, message, asyncMsgType)
        if err != nil {
            // Error only if async fails - no "not connected" error
            if strings.Contains(err.Error(), "no pre-keys available") {
                return fmt.Errorf("friend is not connected and secure messaging keys are not available. %v", err)
            }
            return err
        }
    }
    return nil  // Success - message queued, no error returned
}
```

---

### Gap #4: Missing OnFriendStatusChange Callback Referenced in Documentation
**Severity:** Moderate
**Status:** ✅ COMPLETED

**Documentation Reference:**
> "tox.OnFriendStatusChange(func(friendPK [32]byte, online bool) {" (docs/ASYNC.md:341, docs/ASYNC.md:826)

**Implementation Location:** N/A - callback did not exist

**Resolution:**
Implemented `OnFriendStatusChange` callback that is triggered when a friend transitions between online (connected) and offline (not connected). This callback is specifically for online/offline transitions, while `OnFriendConnectionStatus` tracks all connection status changes including UDP/TCP switches.

**Implementation Details:**
- Added `FriendStatusChangeCallback` type at `toxcore.go:1626`
- Added `OnFriendStatusChange` method at `toxcore.go:1689`
- Modified `updateFriendOnlineStatus` to trigger the callback
- The callback is only fired when online/offline state changes, not for UDP↔TCP transitions
- Comprehensive tests added in `friend_callbacks_test.go`

**Evidence:**
```go
// toxcore.go:1626
type FriendStatusChangeCallback func(friendPK [32]byte, online bool)

// toxcore.go:1689
func (t *Tox) OnFriendStatusChange(callback FriendStatusChangeCallback) {
    t.friendStatusChangeCallback = callback
}

// Example from docs/ASYNC.md now works:
tox.OnFriendStatusChange(func(friendPK [32]byte, online bool) {
    asyncManager.SetFriendOnlineStatus(friendPK, online)
})
```

---

### Gap #5: Storage Capacity Constants Documentation Shows Calculation But Not Implementation
**Severity:** Minor

**Documentation Reference:**
> "MinStorageCapacity = 1536       // Minimum storage capacity (1MB / ~650 bytes per message)
> MaxStorageCapacity = 1536000    // Maximum storage capacity (1GB / ~650 bytes per message)" (README.md:1253-1254)

**Implementation Location:** `async/storage.go:42-45`

**Expected Behavior:** The README explains the calculation: 1MB divided by ~650 bytes equals ~1536 messages.

**Actual Implementation:** The implementation matches the values but the comment explanations differ:

```go
// async/storage.go:42-45
// MinStorageCapacity is the minimum storage capacity (1MB / ~650 bytes per message)
MinStorageCapacity = 1536
// MaxStorageCapacity is the maximum storage capacity (1GB / ~650 bytes per message)
MaxStorageCapacity = 1536000
```

**Gap Details:** The README says "1MB / ~650 bytes per message" = 1536, but the actual math is:
- 1MB = 1,048,576 bytes
- 1,048,576 / 650 ≈ 1,613 messages (not 1536)

The implementation uses 1536 which corresponds to:
- 1536 × 650 ≈ 998,400 bytes (~975KB, not 1MB)

Similarly for max:
- 1GB = 1,073,741,824 bytes  
- 1,073,741,824 / 650 ≈ 1,651,910 messages (not 1,536,000)

The 1,536,000 corresponds to ~1GB / 700 bytes per message.

**Production Impact:** Minor - The actual storage capacity is slightly different from what the documented calculation suggests, but the difference is small (~5%) and the implementation works correctly with its defined values.

**Evidence:**
```go
// The documentation and code both say "1MB / ~650 bytes per message" = 1536
// But mathematically: 1MB / 650 bytes ≈ 1613, not 1536
// The values work, but the explanatory comments are slightly inaccurate
```

---

## Summary

The toxcore-go implementation is mature and well-tested. The gaps identified are primarily:

1. **Documentation inconsistencies** between different documentation files - Gap #1 (Minor) - ✅ **COMPLETED**
2. **Missing callbacks** - Gaps #2 and #4 (Moderate) - ✅ **COMPLETED**
3. **Behavioral documentation drift** where the implementation has evolved (async messaging fallback) but README wasn't updated - Gap #3 (Moderate)

**Completed Actions:**
1. ✅ Synced documentation files to consistently show all four message padding buckets (256B, 1KB, 4KB, 16KB) (Gap #1)
2. ✅ Added `OnFriendConnectionStatus` callback for applications needing friend connection events (Gap #2)
3. ✅ Implemented `OnFriendStatusChange` callback as documented in async messaging examples (Gap #4)

**Remaining Actions:**
1. Update README to document async messaging fallback behavior in `SendFriendMessage` (Gap #3)
2. Fix math/comments for storage capacity constants (Gap #5)

None of these gaps represent security vulnerabilities or major functional issues.
