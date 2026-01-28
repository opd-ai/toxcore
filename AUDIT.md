# toxcore-go Functional Audit Report

**Audit Date:** 2026-01-28  
**Auditor:** Automated Code Audit System  
**Codebase Version:** Current HEAD  
**Go Version:** 1.23.2

---

## AUDIT SUMMARY

| Category | Count | Severity Distribution |
|----------|-------|----------------------|
| CRITICAL BUG | 0 | - |
| FUNCTIONAL MISMATCH | 2 | Medium: 2 |
| MISSING FEATURE | 1 | Low: 1 |
| EDGE CASE BUG | 1 | Low: 1 |
| PERFORMANCE ISSUE | 0 | - |

**Overall Assessment:** The codebase demonstrates strong alignment with documented functionality. All tests pass successfully, build completes without errors, and the implementation generally matches README.md documentation. The issues identified are minor discrepancies that do not affect core functionality.

---

## DETAILED FINDINGS

~~~~
### FUNCTIONAL MISMATCH: AddFriend Tox ID Format Documentation

**File:** toxcore.go:1400-1439  
**Severity:** Medium  

**Description:** The README.md documentation shows AddFriend using a 64-character public key hex string, but the actual implementation expects a 76-character Tox ID (public key + nospam + checksum). This is a documentation inconsistency rather than a code bug.

**Expected Behavior (per README.md):**
```go
friendID, err := tox.AddFriend("76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37B13349", "Hello!")
```
The example shows a 64-character hex string which is a 32-byte public key.

**Actual Behavior:**
The code parses the address using `crypto.ToxIDFromString(address)` which expects a 76-character Tox ID string (32 bytes public key + 4 bytes nospam + 2 bytes checksum = 38 bytes = 76 hex chars).

**Impact:** Users following the README example will receive an "invalid Tox ID length" error when calling AddFriend with only a public key. The example in the README should use a complete 76-character Tox ID.

**Code Reference:**
```go
func (t *Tox) AddFriend(address, message string) (uint32, error) {
    // Parse the Tox ID
    toxID, err := crypto.ToxIDFromString(address)
    if err != nil {
        return 0, err
    }
    // ...
}
```

**Reproduction:** Call `tox.AddFriend("76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37B13349", "Hello!")` as shown in README - will fail with invalid length error.
~~~~

~~~~
### FUNCTIONAL MISMATCH: Privacy Network Transport Implementation Status

**File:** transport/address.go, README.md:104-109  
**Severity:** Medium  

**Description:** The README.md states that Tor .onion, I2P .b32.i2p, Nym .nym, and Lokinet .loki addresses have "interface ready, implementation planned" status. This is accurate, but the documentation could be clearer that these network types are not currently functional for actual network communication.

**Expected Behavior:** Users might expect basic connectivity through these networks based on the "interface ready" language.

**Actual Behavior:** The NetworkAddress types are defined and can be created/used for address representation, but no actual transport-level implementation exists to establish connections over these privacy networks. The code correctly returns appropriate address type identifiers but cannot send/receive data over these networks.

**Impact:** Low - the README does state "implementation planned" which indicates the feature is not complete. Users attempting to use Tor/I2P transports will find the system correctly identifies addresses but cannot establish connections.

**Code Reference:**
```go
// From transport/address.go
const (
    AddressTypeIPv4  AddressType = iota // Fully implemented
    AddressTypeIPv6                      // Fully implemented
    AddressTypeOnion                     // Interface only
    AddressTypeI2P                       // Interface only
    AddressTypeNym                       // Interface only
    AddressTypeLoki                      // Interface only
)
```

**Reproduction:** Create a NetworkAddress with AddressTypeOnion and attempt to use it for transport operations - the address will be valid but transport Send/Receive operations will not reach Tor nodes.
~~~~

~~~~
### MISSING FEATURE: Local Discovery Implementation

**File:** toxcore.go:163, options.go  
**Severity:** Low  

**Description:** The `LocalDiscovery` option is defined in Options struct and defaults to `true` in NewOptions(), but there is no actual implementation of local network discovery functionality. The option is accepted but has no effect on behavior.

**Expected Behavior (per Options struct):** When LocalDiscovery is enabled, the system should discover other Tox instances on the local network for direct peer-to-peer connections without bootstrap nodes.

**Actual Behavior:** The LocalDiscovery field is stored but never used. The codebase contains no LAN discovery broadcast/multicast implementation.

**Impact:** Low impact - most users rely on bootstrap nodes for discovery. Local discovery is an optimization feature primarily useful for testing and local network scenarios.

**Code Reference:**
```go
// From toxcore.go NewOptions()
options := &Options{
    UDPEnabled:        true,
    IPv6Enabled:       true,
    LocalDiscovery:    true,  // Set but unused
    // ...
}
```

**Reproduction:** Set `options.LocalDiscovery = true`, create two Tox instances on the same LAN - they will not discover each other without bootstrap nodes.
~~~~

~~~~
### EDGE CASE BUG: AsyncManager Nil Check Before Friend Status Update

**File:** toxcore.go:1672-1683  
**Severity:** Low  

**Description:** The `updateFriendOnlineStatus` method correctly checks if asyncManager is nil, but the public-facing friend connection methods don't always call this function when friend status changes. This is a minor inconsistency in the friend status notification flow.

**Expected Behavior:** All friend connection status changes should notify the async manager so it can trigger pre-key exchanges when friends come online.

**Actual Behavior:** The asyncManager notification depends on which code path updates friend status. Some paths (like receiving connection status packets) may not consistently call updateFriendOnlineStatus.

**Impact:** In edge cases where friend status changes without proper notification, the async manager may not immediately trigger pre-key exchange, potentially causing a delay in the first async message until the next retrieval cycle.

**Code Reference:**
```go
func (t *Tox) updateFriendOnlineStatus(friendID uint32, online bool) {
    if t.asyncManager != nil {
        t.friendsMutex.RLock()
        friend, exists := t.friends[friendID]
        t.friendsMutex.RUnlock()

        if exists {
            t.asyncManager.SetFriendOnlineStatus(friend.PublicKey, online)
        }
    }
}
```

**Reproduction:** Simulate friend status changing through internal packet processing rather than through the main friend connection flow - async manager may not be immediately notified.
~~~~

---

## VERIFICATION NOTES

### Tests Verified
All tests pass successfully:
- `github.com/opd-ai/toxcore` - PASS
- `github.com/opd-ai/toxcore/async` - PASS  
- `github.com/opd-ai/toxcore/crypto` - PASS
- `github.com/opd-ai/toxcore/dht` - PASS
- `github.com/opd-ai/toxcore/transport` - PASS
- All other packages - PASS

### Build Status
- `go build ./...` - PASS (exit code 0)

### Documentation Alignment Verified

The following documented features were verified as correctly implemented:

1. **Core Tox API** - `New()`, `NewOptions()`, `Kill()`, `Iterate()`, `IterationInterval()`, `IsRunning()` all function as documented
2. **Friend Management** - `AddFriend()`, `AddFriendByPublicKey()`, `DeleteFriend()`, `GetFriends()`, `GetFriendsCount()` work correctly
3. **Message Sending** - `SendFriendMessage()` with variadic message type parameter works as documented
4. **Callbacks** - `OnFriendRequest()`, `OnFriendMessage()`, `OnFriendMessageDetailed()` register and trigger correctly
5. **Self Management** - `SelfSetName()`, `SelfGetName()`, `SelfSetStatusMessage()`, `SelfGetStatusMessage()`, `SelfGetNospam()`, `SelfSetNospam()` all work correctly
6. **State Persistence** - `GetSavedata()`, `Load()`, `NewFromSavedata()` correctly serialize/deserialize state
7. **Bootstrap** - `Bootstrap()` correctly validates public key format and handles DNS resolution
8. **Noise Protocol** - `NegotiatingTransport` correctly wraps UDP with Noise-IK capability
9. **Async Messaging** - Forward secrecy, identity obfuscation, and pseudonym-based storage all function correctly
10. **ToxAV Integration** - `NewToxAV()`, call management, and audio/video frame methods are implemented

### Security Features Verified

1. **Forward Secrecy** - Pre-key system correctly implements one-time key usage
2. **Identity Obfuscation** - HKDF-based pseudonyms correctly hide real identities from storage nodes
3. **Secure Memory** - `crypto.ZeroBytes()` used appropriately for sensitive data cleanup
4. **Constant-Time Comparisons** - `subtle.ConstantTimeCompare` used for cryptographic comparisons
5. **AES-GCM Encryption** - Payload encryption in async messaging uses proper authenticated encryption

---

## RECOMMENDATIONS

### High Priority
None - no critical issues identified.

### Medium Priority
~~1. **Update README.md AddFriend Example** - Change the example Tox ID from 64 characters to a valid 76-character Tox ID to prevent user confusion.~~ **COMPLETED 2026-01-28**: Updated README.md line 490 and security_verification_test.go line 79 to use valid 76-character Tox ID `76518406f6a9f2217e8dc487cc783c25cc16a15eb36ff32e335364ec37b1334912345678868a` instead of the 64-character public key. Also added clarifying comment noting the format (public key + nospam + checksum = 76 hex characters).

### Low Priority
1. **Document Privacy Network Status More Clearly** - Consider adding a "Roadmap" section to clarify which features are complete vs planned.
2. **Implement Local Discovery** - Either implement the feature or mark it as "reserved for future implementation" in documentation.
3. **Review Friend Status Notification Flow** - Ensure all paths that update friend connection status notify the async manager.

---

## CONCLUSION

The toxcore-go implementation demonstrates high quality and strong alignment with its documentation. The codebase successfully implements:
- Pure Go Tox protocol implementation with no CGo dependencies
- Comprehensive cryptographic security with NaCl and Noise-IK protocols
- Forward-secure asynchronous messaging with identity obfuscation
- Multi-network address system architecture
- Audio/video calling infrastructure

The identified issues are minor documentation discrepancies and edge cases that do not compromise the core functionality or security of the implementation. The test suite provides comprehensive coverage and all tests pass, indicating a stable and well-maintained codebase.
