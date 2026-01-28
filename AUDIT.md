# Functional Audit Report: toxcore-go

**Audit Date:** 2026-01-28  
**Codebase Version:** Commit aaf2d67 (main branch)  
**Auditor:** Go Code Audit System  

---

## AUDIT SUMMARY

This audit compares the documented functionality in README.md against the actual implementation in the codebase. The analysis was performed systematically, starting with Level 0 dependencies (crypto, limits) and proceeding through higher-level modules (transport, async, dht, messaging, toxcore).

| Category | Count |
|----------|-------|
| CRITICAL BUG | 0 (1 fixed) |
| FUNCTIONAL MISMATCH | 1 (1 fixed) |
| MISSING FEATURE | 1 |
| EDGE CASE BUG | 2 |
| PERFORMANCE ISSUE | 0 |

**Overall Assessment:** The codebase is generally well-structured and follows Go idioms. Most documented features are implemented correctly. The critical nil pointer dereference bug and bootstrap error handling inconsistency have been fixed with comprehensive test coverage. A few edge cases remain where behavior differs from documentation.

---

## DETAILED FINDINGS

### ✅ FIXED: CRITICAL BUG: Nil Pointer Dereference in Async Client Initialization

**Status:** RESOLVED  
**Fixed in:** commit [current]  
**File:** async/client.go:51-91  
**Severity:** High (was causing SIGSEGV panics)

**Fix Summary:** Added nil check before calling `trans.RegisterHandler()` and added graceful error handling in transport-dependent methods (`storeObfuscatedMessageOnNode` and `retrieveObfuscatedMessagesFromNode`).

**Changes Made:**
1. Modified `NewAsyncClient()` to check if `trans` is nil before registering handlers
2. Added warning log when transport is nil to inform about degraded functionality
3. Added nil transport checks in `storeObfuscatedMessageOnNode()` (line 461)
4. Added nil transport checks in `retrieveObfuscatedMessagesFromNode()` (line 607)
5. Created comprehensive test suite in `async/nil_transport_test.go`
6. Created regression test in `critical_nil_transport_regression_test.go`

**Original Description:** The `NewAsyncClient` function called `trans.RegisterHandler()` on line 72 without first checking if `trans` (the transport parameter) was nil. When `initializeAsyncMessaging()` was called in `toxcore.go:418` with a nil transport (which occurs when UDP is disabled via options), this caused a nil pointer dereference panic.

**Expected Behavior:** According to README.md line 783-790, async messaging is described as an optional feature that should gracefully degrade when not available: "If storage node initialization fails, async messaging features will be unavailable but core Tox functionality remains intact."

**Actual Behavior (BEFORE FIX):** The application crashed with a SIGSEGV panic when creating a Tox instance with UDP disabled:
```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x28 pc=0x6850f9]
```

**Actual Behavior (AFTER FIX):** The application creates successfully with a warning log message, and async messaging gracefully degrades with descriptive error messages when operations are attempted.

**Impact (BEFORE FIX):** Users who create Tox instances with `UDPEnabled = false` (as documented in options) would experience application crashes. This violated the documented graceful degradation behavior.

**Impact (AFTER FIX):** No crashes occur. Core Tox functionality remains available, and async messaging operations return clear error messages indicating the feature is unavailable.

**Reproduction (BEFORE FIX):** 
1. Create options with `UDPEnabled = false`
2. Call `toxcore.New(options)`
3. Application panics during async client initialization

**Verification (AFTER FIX):**
1. Run test: `go test -run TestCriticalBugNilPointerDereference`
2. Run test: `go test -run TestNilTransportGracefulDegradation`
3. Run test: `go test ./async/ -run Nil`
4. All tests pass without panics

**Code Reference (FIXED):**
```go
// async/client.go:51-91 (FIXED)
func NewAsyncClient(keyPair *crypto.KeyPair, trans transport.Transport) *AsyncClient {
    // ... initialization code ...
    
    ac := &AsyncClient{
        // ... fields ...
    }

    // FIXED: Check if transport is available before registering handlers
    if trans != nil {
        trans.RegisterHandler(transport.PacketAsyncRetrieveResponse, ac.handleRetrieveResponse)
    } else {
        logrus.Warn("Transport is nil - async messaging features will be unavailable")
    }
    // ...
}
```

**Related Code in toxcore.go:416-424:**
```go
func initializeAsyncMessaging(keyPair *crypto.KeyPair, udpTransport transport.Transport) *AsyncManager {
    dataDir := getDefaultDataDir()
    // udpTransport can be nil here, which gets passed to NewAsyncManager
    asyncManager, err := async.NewAsyncManager(keyPair, udpTransport, dataDir)
    if err != nil {
        fmt.Printf("Warning: failed to initialize async messaging: %v\n", err)
        return nil
    }
    return asyncManager
}
```

---

### ✅ FIXED: FUNCTIONAL MISMATCH: Bootstrap DNS Failure Handling Behavior

**Status:** RESOLVED  
**Fixed in:** commit [current]  
**File:** toxcore.go:1183-1195  
**Severity:** Medium (was causing inconsistent error handling)

**Fix Summary:** Modified Bootstrap function to return errors for DNS resolution failures, matching the documentation and providing consistent error handling across all failure types.

**Changes Made:**
1. Changed DNS resolution error handling in `toxcore.go:Bootstrap()` to return descriptive errors
2. Updated error message to include address and port for better debugging
3. Updated `TestGap5BootstrapReturnValueInconsistency` to expect errors for DNS failures
4. All bootstrap failures now return errors consistently

**Original Description:** The Bootstrap function returned `nil` (success) when DNS resolution failed, rather than returning an error. While the comment stated this was for "graceful degradation," this silent failure masked legitimate configuration problems and made debugging difficult.

**Expected Behavior:** According to README.md Basic Usage example (line 82-86):
```go
err = tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA...")
if err != nil {
    log.Printf("Warning: Bootstrap failed: %v", err)
}
```
The documentation shows that Bootstrap is expected to return errors for failures, which can then be logged or handled.

**Actual Behavior (BEFORE FIX):** When DNS resolution failed, the function logged a warning but returned `nil`, causing the caller to believe bootstrap succeeded. However, public key validation errors DID return an error. This inconsistent behavior created confusion.

**Actual Behavior (AFTER FIX):** All bootstrap failures (DNS resolution, invalid public key, etc.) now consistently return descriptive errors. Applications can properly detect, log, and handle all failure cases.

**Impact (BEFORE FIX):** Applications might silently fail to connect to the Tox network, and the caller had no way to distinguish between successful bootstrap and DNS failure. Only public key format errors were reported.

**Impact (AFTER FIX):** Applications receive consistent error reporting for all bootstrap failures, enabling proper error handling, logging, and retry logic.

**Reproduction (BEFORE FIX):** 
1. Call `Bootstrap("nonexistent-domain.invalid", 33445, "valid-public-key")`
2. Function returns nil (success)
3. No error is returned despite bootstrap failure

**Verification (AFTER FIX):**
1. Run test: `go test -run TestGap5BootstrapReturnValueInconsistency`
2. Test verifies DNS failures now return errors
3. All bootstrap-related tests pass

**Code Reference (FIXED):**
```go
// toxcore.go:1183-1195 (FIXED)
addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(address, fmt.Sprintf("%d", port)))
if err != nil {
    // Return error for DNS resolution failures to match documentation
    // Applications can handle these errors appropriately (log, retry, etc.)
    logrus.WithFields(logrus.Fields{
        "function": "Bootstrap",
        "address":  address,
        "port":     port,
        "error":    err.Error(),
    }).Error("Bootstrap address resolution failed")
    return fmt.Errorf("failed to resolve bootstrap address %s:%d: %w", address, port, err)
}
```

---

### FUNCTIONAL MISMATCH: Async EncryptForRecipient Function Deprecated Without Alternative Path

**File:** async/storage.go:612-616  
**Severity:** Medium  
**Description:** The `EncryptForRecipient` function documented in README.md (lines 960-970) is marked as deprecated and immediately returns an error stating "deprecated: EncryptForRecipient does not provide forward secrecy - use ForwardSecurityManager instead". However, the documentation still shows this function as the primary way to encrypt messages for storage.

**Expected Behavior:** According to README.md lines 960-970:
```go
message := "Hello, offline friend!"
encryptedData, nonce, err := async.EncryptForRecipient([]byte(message), recipientPK, senderKeyPair.Private)
if err != nil {
    log.Fatal(err)
}
```
This should encrypt a message and return the encrypted data with a nonce.

**Actual Behavior:** Calling this function always fails with error: "deprecated: EncryptForRecipient does not provide forward secrecy - use ForwardSecurityManager instead"

**Impact:** Users following the README.md documentation for Direct Message Storage API will encounter immediate errors. The documentation does not provide an updated example using ForwardSecurityManager for the storage API use case.

**Reproduction:**
1. Follow README.md "Direct Message Storage API" example
2. Call `async.EncryptForRecipient(message, recipientPK, senderSK)`
3. Function always returns error

**Code Reference:**
```go
// async/storage.go:612-616
// EncryptForRecipient is DEPRECATED - does not provide forward secrecy
// Use ForwardSecurityManager for forward-secure messaging instead
// This function is kept for backward compatibility only
func EncryptForRecipient(message []byte, recipientPK, senderSK [32]byte) ([]byte, [24]byte, error) {
    return nil, [24]byte{}, errors.New("deprecated: EncryptForRecipient does not provide forward secrecy - use ForwardSecurityManager instead")
}
```

---

### MISSING FEATURE: NewMessageStorage Constructor Not Exported for Direct Use

**File:** async/storage.go:90  
**Severity:** Low  
**Description:** README.md lines 946-952 show creating a MessageStorage instance directly:
```go
storage, err := async.NewMessageStorage(storageKeyPair, dataDir)
```
However, `NewMessageStorage` takes a `*crypto.KeyPair` parameter, not two separate parameters. The documentation incorrectly suggests the function accepts different parameters.

**Expected Behavior:** According to README.md:
```go
storageKeyPair, err := crypto.GenerateKeyPair()
// ...
storage, err := async.NewMessageStorage(storageKeyPair, dataDir)
```

**Actual Behavior:** The function signature matches the documented usage, but the constructor returns `*MessageStorage` not `(*MessageStorage, error)`. The function does not return an error as shown in the documentation.

**Impact:** Users copying the documented code will encounter compilation errors because they're expecting a two-return-value function.

**Code Reference:**
```go
// async/storage.go:90-139
func NewMessageStorage(keyPair *crypto.KeyPair, dataDir string) *MessageStorage {
    // Returns *MessageStorage only, not (*MessageStorage, error)
    // ...
    return storage
}
```

---

### EDGE CASE BUG: GetFriends Returns Shallow Copy Exposing Internal State

**File:** toxcore.go:1783-1795  
**Severity:** Low  
**Description:** The `GetFriends()` method creates a copy of the friends map but copies the Friend pointers, not the Friend values. This means the caller receives pointers to the same Friend objects stored internally, allowing modification of internal state.

**Expected Behavior:** The comment on line 1783 states "Return a copy of the friends map to prevent external modification", suggesting a deep copy is intended.

**Actual Behavior:** The method copies map keys and values, but since values are `*Friend` (pointers), the caller can modify the internal Friend objects.

**Impact:** External code can unintentionally (or intentionally) corrupt internal friend state by modifying the returned Friend objects. This violates encapsulation and can lead to race conditions.

**Reproduction:**
```go
friends := tox.GetFriends()
friends[0].Name = "Modified"  // This modifies internal state
```

**Code Reference:**
```go
// toxcore.go:1783-1795
func (t *Tox) GetFriends() map[uint32]*Friend {
    t.friendsMutex.RLock()
    defer t.friendsMutex.RUnlock()

    // Return a copy of the friends map to prevent external modification
    friendsCopy := make(map[uint32]*Friend)
    for id, friend := range t.friends {
        friendsCopy[id] = friend  // BUG: Copies pointer, not Friend value
    }
    return friendsCopy
}
```

---

### EDGE CASE BUG: Async Manager SetFriendOnlineStatus Potential Deadlock Pattern

**File:** async/manager.go:121-133  
**Severity:** Low  
**Description:** The `SetFriendOnlineStatus` method acquires a mutex lock and then launches a goroutine (`go am.handleFriendOnline(friendPK)`). The `handleFriendOnline` function performs operations that may need to acquire the same mutex, creating potential deadlock scenarios. While the current implementation releases the lock before the goroutine runs, the pattern is fragile.

**Expected Behavior:** Thread-safe status updates with predictable behavior.

**Actual Behavior:** The code currently works because the mutex is released before the goroutine acquires it, but if `handleFriendOnline` is ever called synchronously or if the mutex scope changes, deadlock would occur.

**Impact:** Low risk of actual deadlock with current implementation, but the pattern makes the code fragile for future modifications.

**Code Reference:**
```go
// async/manager.go:121-133
func (am *AsyncManager) SetFriendOnlineStatus(friendPK [32]byte, online bool) {
    am.mutex.Lock()
    defer am.mutex.Unlock()

    wasOffline := !am.onlineStatus[friendPK]
    am.onlineStatus[friendPK] = online

    // If friend just came online, handle pre-key exchange and message retrieval
    if wasOffline && online {
        go am.handleFriendOnline(friendPK)  // Launches goroutine that may need mutex
    }
}
```

---

## NOTES ON DOCUMENTATION ACCURACY

1. **README.md line 19**: States "Go 1.23.2 or later" - verified correct via go.mod.

2. **Multi-Network Support (lines 100-175)**: Correctly states that Tor, I2P, Nym, Lokinet are "interface ready, implementation planned". The code confirms these address types are defined but not fully implemented.

3. **Noise Protocol Integration (lines 176-262)**: Documented features match implementation. The NegotiatingTransport correctly wraps UDP with Noise-IK.

4. **Message Limits (lines 391-396)**: Documentation states "1372 UTF-8 bytes" limit, code correctly enforces this in `limits/limits.go` and `toxcore.go`.

5. **Async Messaging Privacy (lines 876-891)**: Documentation about automatic peer identity obfuscation matches implementation in `async/obfs.go` and `async/manager.go`.

---

## RECOMMENDATIONS

1. **✅ COMPLETED - Priority 1 - Fix Critical Bug**: Added nil check in `async/client.go:NewAsyncClient()` before calling `trans.RegisterHandler()`. The fix includes:
   - Nil transport check with warning log in `NewAsyncClient()`
   - Graceful error handling in `storeObfuscatedMessageOnNode()` 
   - Graceful error handling in `retrieveObfuscatedMessagesFromNode()`
   - Comprehensive test suite covering nil transport scenarios
   - Regression tests to prevent future regressions

2. **✅ COMPLETED - Priority 2 - Fix Bootstrap Behavior**: Modified `toxcore.go:Bootstrap()` to return an error for DNS resolution failures, matching the documentation examples in README.md. The fix includes:
   - Changed DNS resolution error handling to return descriptive errors instead of nil
   - Updated `TestGap5BootstrapReturnValueInconsistency` to expect errors for DNS failures
   - All bootstrap failures now return errors consistently for proper application error handling
   - Applications can now detect, log, and handle all bootstrap failures appropriately

3. **Priority 3 - Update Documentation**: Either restore `EncryptForRecipient` functionality or update README.md to show the correct ForwardSecurityManager-based approach for the Direct Message Storage API.

4. **Priority 4 - Fix GetFriends**: Return deep copies of Friend objects to maintain encapsulation:
```go
friendsCopy[id] = &Friend{
    PublicKey:        friend.PublicKey,
    Status:           friend.Status,
    // ... other fields
}
```

5. **Priority 5 - Code Review**: Review async/manager.go mutex patterns for potential future issues.

---

## APPENDIX: Files Analyzed

### Level 0 (No internal imports)
- limits/limits.go ✓
- crypto/keypair.go ✓
- crypto/encrypt.go ✓
- crypto/decrypt.go ✓
- crypto/secure_memory.go ✓

### Level 1 (Imports Level 0)
- transport/types.go ✓
- transport/address.go ✓
- transport/packet.go ✓
- friend/friend.go ✓
- messaging/message.go ✓

### Level 2 (Imports Level 1)
- transport/udp.go ✓
- transport/noise_transport.go ✓
- transport/negotiating_transport.go ✓
- async/storage.go ✓
- async/obfs.go ✓
- dht/routing.go ✓

### Level 3 (Imports Level 2)
- async/client.go ✓
- async/manager.go ✓
- dht/bootstrap.go ✓

### Level 4 (Main package)
- toxcore.go ✓
- options.go ✓

---

*End of Audit Report*
