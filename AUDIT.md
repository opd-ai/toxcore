# Functional Audit Report: toxcore-go

**Audit Date:** 2026-02-18  
**Auditor:** Automated Code Audit  
**Version:** Latest commit on main branch  
**Scope:** Documentation vs Implementation comparison focused on bugs, missing features, and functional misalignments

---

## AUDIT SUMMARY

| Category | Count | Severity Distribution |
|----------|-------|----------------------|
| CRITICAL BUG | 0 | N/A |
| FUNCTIONAL MISMATCH | 2 | 1 Medium, 1 Low |
| MISSING FEATURE | 1 | Low |
| EDGE CASE BUG | 2 (1 resolved) | 1 Medium, ~~1 Low~~ |
| PERFORMANCE ISSUE | 0 | N/A |
| **TOTAL** | **5 (1 resolved)** | 2 Medium, 3 Low |

**Overall Assessment:** The codebase demonstrates strong alignment between documentation and implementation. The toxcore-go project is well-implemented with comprehensive test coverage (all tests pass including race detection). The findings identified are minor documentation clarifications and edge case improvements rather than critical bugs.

---

## DETAILED FINDINGS

~~~~
### FUNCTIONAL MISMATCH: Nym Transport Documented as "Interface Ready" but Not Functional
**File:** transport/network_transport_impl.go:478-560
**Severity:** Low
**Description:** The README.md states Nym .nym addresses have "interface ready, implementation planned" status. However, the NymTransport implementation is a complete stub that returns errors for all operations (Dial, Listen, DialPacket). While the documentation accurately mentions it's "planned," the Transport interface methods exist but always fail, which could be confusing to users who attempt to use them.

**Expected Behavior:** Based on README ("interface ready"), users might expect the transport to be instantiable with stub methods that communicate their limitations gracefully.

**Actual Behavior:** The implementation is present but all methods return immediate errors with "not implemented" messages. This is technically accurate but the "interface ready" language in documentation suggests more progress than exists.

**Impact:** Users exploring privacy network options may attempt to use Nym transport and receive cryptic errors. Low impact as documentation does clarify implementation is planned.

**Reproduction:** 
```go
nymTransport := transport.NewNymTransport()
conn, err := nymTransport.Dial("example.nym:8080")
// Returns: "Nym transport Dial not implemented - requires Nym SDK websocket client"
```

**Code Reference:**
```go
// From transport/network_transport_impl.go:527-540
func (t *NymTransport) Dial(address string) (net.Conn, error) {
    // ...
    return nil, fmt.Errorf("Nym transport Dial not implemented - requires Nym SDK websocket client")
}
```

**Recommendation:** Update README.md to clarify that Nym support is a "placeholder/stub" rather than "interface ready" to set clearer expectations. The current I2P and Lokinet implementations are functional, while Nym is not.
~~~~

~~~~
### FUNCTIONAL MISMATCH: README Example Uses Different AddFriend Signature
**File:** toxcore.go:1969-2002 vs README.md
**Severity:** Low  
**Description:** The README.md basic usage example shows `tox.AddFriendByPublicKey(publicKey)` being called inside the `OnFriendRequest` callback with a `[32]byte` public key. The actual implementation correctly accepts `[32]byte`, but the documentation could be clearer about the distinction between `AddFriend` (takes Tox ID string with message) and `AddFriendByPublicKey` (takes public key array, used for accepting requests).

**Expected Behavior:** The README example accurately reflects the API.

**Actual Behavior:** The example is correct, but the inline comment "// Automatically accept friend requests" could mislead users into thinking this is the recommended pattern without explaining when to use `AddFriend` vs `AddFriendByPublicKey`.

**Impact:** Minor documentation clarity issue. Users reading the quick start may not understand the two methods serve different purposes.

**Reproduction:** Review README.md "Basic Usage" section.

**Code Reference:**
```go
// From README.md Basic Usage:
tox.OnFriendRequest(func(publicKey [32]byte, message string) {
    fmt.Printf("Friend request: %s\n", message)
    // Automatically accept friend requests
    friendID, err := tox.AddFriendByPublicKey(publicKey)
    // ...
})

// From toxcore.go:1992-2002:
func (t *Tox) AddFriendByPublicKey(publicKey [32]byte) (uint32, error) {
    // Correctly implemented - accepts [32]byte public key
}
```

**Recommendation:** Add a brief note in the README explaining: "Use `AddFriend(toxID, message)` to send friend requests, and `AddFriendByPublicKey(publicKey)` to accept incoming requests where you already have the public key from the callback."
~~~~

~~~~
### MISSING FEATURE: Relay Connection in Advanced NAT Traversal
**File:** transport/advanced_nat.go:292
**Severity:** Low
**Description:** The advanced NAT traversal system includes a `connectViaRelay` method that returns `errors.New("relay connection not implemented")`. This is mentioned in the codebase but not documented in README.md as a limitation. The README discusses NAT traversal techniques but doesn't clarify that relay-based connectivity for symmetric NAT scenarios is not yet implemented.

**Expected Behavior:** README should document which NAT traversal techniques are fully implemented vs planned.

**Actual Behavior:** The code structure suggests relay support is planned, but the implementation stub exists without documentation.

**Impact:** Users behind symmetric NAT who cannot establish direct connections may expect relay fallback to work. Low impact as most users won't encounter this scenario.

**Reproduction:**
```go
// Internal method - not directly exposed to users
// Called when direct NAT traversal fails
err := natTraversal.connectViaRelay()
// Returns: "relay connection not implemented"
```

**Code Reference:**
```go
// From transport/advanced_nat.go:291-292
func (n *AdvancedNAT) connectViaRelay() error {
    return errors.New("relay connection not implemented")
}
```

**Recommendation:** Add a note in the README Roadmap section clarifying that relay-based NAT traversal is planned but not yet implemented. Users behind symmetric NAT should be aware they may need to use TCP relay nodes.
~~~~

~~~~
### EDGE CASE BUG: Panic on Nospam Generation Failure
**File:** toxcore.go:2061
**Severity:** Medium
**Description:** The `generateNospam()` function panics if cryptographic random generation fails. While this is documented in the code comment as intentional ("Panic on crypto failure as it indicates serious system-level issues"), this behavior differs from the general error-handling pattern used elsewhere in the codebase where errors are returned rather than panicking.

**Expected Behavior:** Based on Go idioms and the rest of the codebase, functions should return errors rather than panic, allowing callers to handle failures gracefully.

**Actual Behavior:** The function panics with a formatted error message, which could crash the application without giving it a chance to recover or log appropriately.

**Impact:** If the system's CSPRNG fails (extremely rare, but possible on resource-constrained systems or containers with limited entropy), the entire application crashes rather than failing gracefully. This is a security-conscious design choice but may surprise users.

**Reproduction:**
```go
// If crypto/rand.Read fails (e.g., /dev/urandom unavailable):
nospam := generateNospam() // Panics
```

**Code Reference:**
```go
// From toxcore.go:2059-2062
func generateNospam() [4]byte {
    nospam, err := crypto.GenerateNospam()
    if err != nil {
        panic(fmt.Sprintf("failed to generate nospam: %v", err))
    }
    return nospam
}
```

**Recommendation:** While the panic is a reasonable security choice (a non-functional CSPRNG is a critical system failure), consider:
1. Documenting this behavior in the README security section
2. Or converting to an error return with clear documentation that callers MUST check this error
~~~~

~~~~
### ~~EDGE CASE BUG: Windows Platform Stub Panic in Async Storage~~ ✅ RESOLVED
**File:** async/storage_limits_unix.go:6-9
**Severity:** Low
**Status:** Fixed (2026-02-18)

**Resolution:** Changed the `getWindowsDiskSpace` function stub to return an error instead of panicking:
```go
func getWindowsDiskSpace(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
    return 0, 0, 0, errors.New("getWindowsDiskSpace not available on non-Windows platforms")
}
```

This allows graceful failure handling if build tags are misconfigured or cross-compilation issues occur, rather than crashing the application.
~~~~

---

## VERIFIED IMPLEMENTATIONS

The following documented features were verified as correctly implemented:

### ✅ Core Protocol
- Friend management (add, delete, list) - **Working**
- Real-time messaging with types (normal, action) - **Working**
- Friend request handling - **Working**
- Connection status tracking - **Working**
- Name and status message management - **Working**
- Nospam value management - **Working**

### ✅ Network Communication  
- IPv4/IPv6 UDP and TCP transport - **Working**
- DHT peer discovery and routing - **Working**
- Bootstrap node connectivity - **Working**
- Packet encryption with NaCl - **Working**
- LAN Discovery (LocalDiscovery option) - **Working**

### ✅ Cryptographic Security
- Ed25519 digital signatures - **Working**
- Curve25519 key exchange - **Working**
- Noise Protocol Framework (IK pattern) - **Working**
- Forward secrecy with pre-key system - **Working**
- Secure memory handling - **Working**

### ✅ Advanced Features
- Asynchronous messaging with offline delivery - **Working**
- Message padding for traffic analysis resistance - **Working**
- Identity obfuscation for async messaging - **Working**
- State persistence (save/load) - **Working**
- ToxAV audio/video infrastructure - **Working**
- Group chat functionality - **Working** (with documented limitations)
- File transfer API - **Working**

### ✅ Privacy Network Transports
- Tor .onion (SOCKS5 proxy) - **Working**
- I2P .b32.i2p (SAM bridge) - **Working**  
- Lokinet .loki (SOCKS5 proxy) - **Working**
- Nym .nym - **Placeholder Only** (not functional)

### ✅ C API Bindings
- Core Tox functions - **Working**
- ToxAV functions - **Working**
- Callback registration - **Working**

### ✅ Testing & Quality
- Test suite passes with race detection - **Verified**
- Build succeeds without warnings - **Verified**
- Documentation examples are accurate - **Verified** (minor clarifications suggested)

---

## DOCUMENTATION ACCURACY NOTES

### Accurately Documented Limitations

The following limitations are correctly documented in README.md:

1. **Proxy Support**: Correctly states UDP bypasses proxy configuration
2. **Privacy Networks**: Correctly marks Nym as "interface ready, implementation planned"
3. **Group Chat DHT Discovery**: Correctly notes same-process limitation
4. **Local Discovery**: Correctly documents it's disabled in testing mode
5. **ToxAV**: Accurately describes integration requirements

### Minor Documentation Improvements Suggested

1. Clarify Nym is a stub rather than "interface ready"
2. Add note about `AddFriend` vs `AddFriendByPublicKey` usage patterns
3. Document relay connection limitation for symmetric NAT
4. Note the panic behavior on CSPRNG failure in security documentation

---

## CONCLUSION

The toxcore-go codebase demonstrates excellent alignment between documented functionality and actual implementation. The project maintains high code quality with:

- Comprehensive test coverage passing with race detection
- Clean builds without warnings
- Well-documented public APIs with GoDoc comments
- Security-conscious implementation patterns

The findings in this audit are predominantly documentation clarifications and minor edge cases rather than functional bugs. The codebase is production-quality for the documented feature set.

**Recommendation:** Address the medium-severity documentation items to improve developer experience, but no blocking issues were identified for deployment.
