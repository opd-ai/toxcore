# AUDIT.md - Functional Audit Report

## AUDIT SUMMARY

```text
Total Findings: 6
Critical Bugs: 1
Functional Mismatches: 2
Missing Features: 2
Edge Case Bugs: 1
Performance Issues: 0
```

## DETAILED FINDINGS

### CRITICAL BUG: Multi-Network Address Conversion Missing Implementation

**File:** transport/address.go:125-140
**Severity:** High
**Description:** The README documents ConvertNetAddrToNetworkAddress function as "fully supported" for IPv4/IPv6, but the function doesn't exist in the codebase. This breaks all multi-network functionality shown in README examples.
**Expected Behavior:** `transport.ConvertNetAddrToNetworkAddress(udpAddr)` should convert net.Addr to NetworkAddress and return Type, IsPrivate(), IsRoutable() functionality as documented
**Actual Behavior:** Function does not exist, causing compile-time failures for any code following README examples
**Impact:** All multi-network code examples in README fail to compile; users cannot use documented multi-network features
**Reproduction:** 
1. Follow README multi-network example
2. Call `transport.ConvertNetAddrToNetworkAddress(&net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 8080})`
3. Compilation fails with undefined function error
**Code Reference:**
```go
// From README example - this function does not exist
netAddr, err := transport.ConvertNetAddrToNetworkAddress(udpAddr)
if err != nil {
    log.Fatal(err)
}
```

### FUNCTIONAL MISMATCH: Noise-IK Transport Not Available to Users

**File:** transport/noise_transport.go:52-70
**Severity:** Medium  
**Description:** README documents NewNoiseTransport as a public API for users to create Noise-encrypted transports, but the README usage examples show incorrect import paths and the function signature doesn't match documented usage.
**Expected Behavior:** Users should be able to import and use `transport.NewNoiseTransport()` directly as shown in README examples
**Actual Behavior:** Function exists but is primarily used internally; README examples use incorrect patterns that don't match actual API
**Impact:** Users cannot create secure Noise-IK transports following README documentation
**Reproduction:** Follow README Noise transport example and encounter API mismatch
**Code Reference:**
```go
// README shows this pattern, but API doesn't match
noiseTransport, err := transport.NewNoiseTransport(udpTransport, keyPair.Private[:])
```

### MISSING FEATURE: Bootstrap Method Return Value Documentation Mismatch

**File:** toxcore.go:1141-1200
**Severity:** Medium
**Description:** README examples show Bootstrap method calls without error handling (suggesting success-only operation), but actual implementation returns errors that should be handled.
**Expected Behavior:** Based on README, Bootstrap should either not return errors or documentation should show proper error handling
**Actual Behavior:** Bootstrap returns error but README examples don't show error handling, misleading users about proper usage
**Impact:** Users following README examples will have improper error handling in production code
**Reproduction:** Follow basic usage example from README that omits Bootstrap error handling
**Code Reference:**
```go
// README example omits error handling:
tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")

// Actual implementation requires:
err = tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
if err != nil {
    log.Printf("Warning: Bootstrap failed: %v", err)
}
```

### MISSING FEATURE: Load Method Not Documented

**File:** toxcore.go:2985-3020
**Severity:** Low
**Description:** Implementation includes `Load()` method for updating existing Tox instances with saved state, but this method is not documented in README's State Persistence section.
**Expected Behavior:** README should document the Load method as an alternative to NewFromSavedata for updating existing instances
**Actual Behavior:** Load method exists and works but is undocumented, creating an incomplete API reference
**Impact:** Users unaware of this functionality may recreate instances unnecessarily instead of using more efficient Load method
**Reproduction:** Search README for "Load" method - not found despite being implemented
**Code Reference:**
```go
// Undocumented but implemented:
err := tox.Load(savedata)
if err != nil {
    log.Printf("Failed to load state: %v", err)
}
```

### EDGE CASE BUG: C API Documentation Without Full Implementation

**File:** capi/toxcore_c.go:1-100
**Severity:** Low
**Description:** README shows extensive C API usage example including functions like `hex_string_to_bin` and complete error handling, but C bindings provide only basic functionality without helper functions.
**Expected Behavior:** C API should provide all helper functions shown in README example or example should be updated to show actual available functions
**Actual Behavior:** C example in README uses functions that don't exist in the C bindings implementation
**Impact:** C developers following README example will encounter undefined function errors
**Reproduction:** Attempt to compile README C example - `hex_string_to_bin` function not found
**Code Reference:**
```c
// README shows this function which doesn't exist:
hex_string_to_bin("F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67", bootstrap_pub_key);
```

### FUNCTIONAL MISMATCH: Async Message Handler Registration Missing from Main API

**File:** async/manager.go:22-35
**Severity:** Medium
**Description:** README documents `SetAsyncMessageHandler` as part of async manager API, but this functionality is not exposed through the main Tox interface despite async manager being integrated.
**Expected Behavior:** Main Tox API should expose async message handler registration as shown in README examples
**Actual Behavior:** Async handler can only be set directly on AsyncManager, not through main Tox interface, requiring users to access internal components
**Impact:** Users cannot properly set up async message handling following documented patterns without accessing internal APIs
**Reproduction:** Follow README async demo example and find no equivalent method on main Tox instance
**Code Reference:**
```go
// README shows pattern like this but method not available on Tox instance:
tox.SetAsyncMessageHandler(func(senderPK [32]byte, message string, messageType async.MessageType) {
    log.Printf("📨 Received async message from %x: %s", senderPK[:8], message)
})
```
