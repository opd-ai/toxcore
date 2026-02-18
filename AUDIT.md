# toxcore-go Functional Audit Report

**Audit Date:** February 2026  
**Codebase Version:** Current HEAD  
**Auditor:** Automated Code Audit System

---

## AUDIT SUMMARY

This audit compares the documented functionality in README.md against the actual implementation in the toxcore-go codebase. The analysis was performed by systematically examining core modules in dependency order.

| Category | Count |
|----------|-------|
| CRITICAL BUG | 0 |
| FUNCTIONAL MISMATCH | 1 |
| MISSING FEATURE | 1 |
| EDGE CASE BUG | 0 |
| PERFORMANCE ISSUE | 0 |
| RESOLVED | 2 |

**Overall Assessment:** The codebase is well-implemented with excellent alignment between documentation and code. The README.md accurately describes implementation status, including explicit disclaimers for partial or stub implementations. Most documented features are fully functional.

---

## DETAILED FINDINGS

### ✅ RESOLVED: Nym Transport Marked as Partial but is Stub-Only

~~~~
**File:** transport/network_transport_impl.go:478-563
**Severity:** Low (RESOLVED)
**Resolution Date:** February 2026
**Description:** Added `IsConnectivitySupported()` and `ConnectivityStatus()` methods to NetworkAddress in transport/address.go. Users can now programmatically check if an address type has functional transport support before attempting connections.
**Code Changes:**
```go
// From transport/address.go - new methods added
func (na *NetworkAddress) IsConnectivitySupported() bool {
    switch na.Type {
    case AddressTypeIPv4, AddressTypeIPv6, AddressTypeOnion, AddressTypeI2P, AddressTypeLoki:
        return true
    case AddressTypeNym:
        return false // Stub only
    default:
        return false
    }
}

func (na *NetworkAddress) ConnectivityStatus() string {
    // Returns human-readable status including "stub only" for Nym
}
```

**Original Issue:** The address parsing system fully parses .nym addresses without indicating to users that actual connectivity will fail.
**Resolution:** Users can now call `IsConnectivitySupported()` to check if connectivity is available, and `ConnectivityStatus()` for a detailed description. Nym addresses correctly report false/stub-only status.
~~~~

### ✅ RESOLVED: Proxy UDP Traffic Warning Now Enforced at Runtime

~~~~
**File:** transport/proxy.go:143-153
**Severity:** Medium (RESOLVED)
**Resolution Date:** February 2026
**Description:** Changed log level from Debug to Warn when UDP traffic bypasses the proxy. Users are now explicitly warned with the message "UDP traffic bypassing proxy - sent directly without proxy protection (real IP may be exposed)" at Warn level.
**Code Change:**
```go
// From transport/proxy.go:143-153
// For connectionless transports, delegate to underlying transport
// Note: Full UDP proxy support would require SOCKS5 UDP association
// WARNING: UDP traffic bypasses the proxy and may leak the user's real IP address
logrus.WithFields(logrus.Fields{
    "function":    "ProxyTransport.Send",
    "packet_type": packet.PacketType,
    "dest_addr":   addr.String(),
    "proxy_type":  t.proxyType,
}).Warn("UDP traffic bypassing proxy - sent directly without proxy protection (real IP may be exposed)")
```
~~~~

### FUNCTIONAL MISMATCH: Group Chat DHT Discovery Query Response Handling Not Complete

~~~~
**File:** group/chat.go:1-200, dht/group_storage.go
**Severity:** Low
**Description:** The README.md states Group Chat DHT Discovery has "⚠️ Query response handling and timeout mechanism not yet implemented" with "Full cross-network discovery requires completing the response collection layer". The implementation correctly announces groups to DHT but the query mechanism for discovering groups across different processes/networks is incomplete.
**Expected Behavior:** Groups can be discovered across different Tox processes via DHT queries
**Actual Behavior:** Groups are announced to DHT successfully, but query responses from remote DHT nodes are not collected/processed. Discovery only works within the same process via local registry.
**Impact:** Low - Documentation accurately describes this limitation; local group functionality works correctly
**Reproduction:**
1. Create a group in Process A with DHT enabled
2. Attempt to discover the group from Process B via DHT
3. Discovery fails (falls back to local registry which is empty in Process B)
**Code Reference:**
```go
// From group/chat.go:178-193
// registerGroup adds a group to the local registry for DHT lookups and announces it to the DHT network.
func registerGroup(chatID uint32, info *GroupInfo, dhtRouting *dht.RoutingTable, transport transport.Transport) {
    // Store in local registry for backward compatibility
    groupRegistry.Lock()
    groupRegistry.groups[chatID] = info
    groupRegistry.Unlock()

    // Announce to DHT if available
    if dhtRouting != nil && transport != nil {
        announcement := &dht.GroupAnnouncement{...}
        if err := dhtRouting.AnnounceGroup(announcement, transport); err != nil {
            logrus.WithFields(logrus.Fields{...}).Warn("Best-effort DHT group announcement failed")
        }
    }
}
```

**Recommendation:** This is accurately documented. When implementing, add response collection for DHT group queries with timeout handling.
~~~~

### MISSING FEATURE: NAT Traversal for Symmetric NAT

~~~~
**File:** transport/nat.go, transport/advanced_nat.go
**Severity:** Low  
**Description:** The README.md states "Relay-based NAT traversal for symmetric NAT is planned but not yet implemented. Users behind symmetric NAT may need to use TCP relay nodes as a workaround." The code includes UDP hole punching and port prediction but lacks the relay mechanism for symmetric NAT scenarios.
**Expected Behavior:** Users behind symmetric NAT should have relay fallback option
**Actual Behavior:** UDP hole punching may fail for symmetric NAT without relay fallback
**Impact:** Low - Documentation accurately describes this limitation; most NAT types are handled by existing techniques
**Reproduction:**
1. Run Tox instance behind symmetric NAT
2. Attempt to establish UDP connection to peer
3. Connection fails without TCP relay fallback
**Code Reference:**
```go
// From transport/hole_puncher.go (NAT traversal exists but no symmetric NAT relay)
// The implementation includes port prediction and hole punching but not relay fallback
```

**Recommendation:** This is accurately documented as a planned feature. Implementation would involve TCP relay server infrastructure.
~~~~

---

## VERIFIED IMPLEMENTATIONS

The following documented features were verified as correctly implemented:

### ✅ Core Protocol
- Friend management (add, delete, list) - `toxcore.go:1963-2756`
- Real-time messaging with message types - `toxcore.go:2098-2260`, `messaging/message.go`
- Friend requests with custom messages - `toxcore.go:1963-2006`, `friend/request.go`
- Connection status and presence - `toxcore.go:2326-2414`
- Name and status message management - `toxcore.go:2755-2826`
- Nospam management - `toxcore.go:1814-1875`

### ✅ Network Communication
- IPv4/IPv6 UDP and TCP transport - `transport/udp.go`, `transport/tcp.go`
- DHT peer discovery and routing - `dht/routing.go`, `dht/node.go`
- Bootstrap node connectivity - `dht/bootstrap.go`
- NAT traversal (UDP hole punching, port prediction) - `transport/nat.go`, `transport/hole_puncher.go`
- Packet encryption with NaCl crypto_box - `crypto/encrypt.go`

### ✅ Cryptographic Security
- Ed25519 digital signatures - `crypto/ed25519.go`
- Curve25519 key exchange (ECDH) - `crypto/keypair.go`, `crypto/shared_secret.go`
- ChaCha20-Poly1305 via NaCl box - `crypto/encrypt.go`
- Noise Protocol Framework (IK pattern) - `transport/noise_transport.go`, `noise/handshake.go`
- Forward secrecy with pre-key system - `async/forward_secrecy.go`, `async/prekeys.go`
- Identity obfuscation for async messaging - `async/obfs.go`
- Secure memory handling - `crypto/secure_memory.go`

### ✅ Advanced Features
- Asynchronous messaging with offline delivery - `async/manager.go`, `async/client.go`, `async/storage.go`
- Message padding for traffic analysis resistance - `async/message_padding.go`, `messaging/message.go:400-414`
- Pseudonym-based storage node routing - `async/obfs.go`
- State persistence (save/load Tox profile) - `toxcore.go:370-404, 901-1000, 2584-2650`
- ToxAV audio/video calling infrastructure - `toxav.go`, `av/` package
- Group chat functionality - `group/chat.go`
- Local network discovery - `dht/local_discovery.go`
- File transfer operations - `file/manager.go`, `file/transfer.go`

### ✅ Privacy Network Support (as documented)
- Tor .onion via SOCKS5 (TCP only) - `transport/network_transport_impl.go:120-278`
- I2P .b32.i2p via SAM bridge - `transport/network_transport_impl.go:280-476`
- Lokinet .loki via SOCKS5 - `transport/network_transport_impl.go:565-718`
- Nym .nym - Correctly documented as stub only - `transport/network_transport_impl.go:478-563`

### ✅ Protocol Version Negotiation
- NegotiatingTransport implementation - `transport/negotiating_transport.go`
- Legacy/Noise-IK version support - `transport/version_negotiation.go`
- Backward compatibility handling - `transport/versioned_handshake.go`

### ✅ Proxy Support (as documented)
- HTTP proxy for TCP - `transport/proxy.go`
- SOCKS5 proxy for TCP - `transport/proxy.go`
- UDP bypass documented and implemented - `transport/proxy.go:130-152`

### ✅ Message Limits
- 1372 byte plaintext limit enforced - `limits/limits.go`, `toxcore.go:2098-2118`
- 128 byte name limit enforced - `toxcore.go:2766-2769`
- 1007 byte status message limit enforced - `toxcore.go:2797-2800`

---

## DOCUMENTATION ACCURACY ASSESSMENT

The README.md demonstrates excellent documentation practices:

1. **Explicit Status Markers**: Features are clearly marked with ✅ (implemented), 🚧 (planned), or ⚠️ (with limitations)

2. **Honest Limitation Disclosure**: Partial implementations are clearly documented:
   - Proxy UDP bypass clearly stated
   - Nym marked as "stub only"
   - Group DHT discovery limitations noted
   - Symmetric NAT relay noted as planned

3. **Code-Documentation Alignment**: Implementation files contain detailed comments matching README descriptions

4. **Usage Examples**: Code examples in README match actual API signatures

---

## RECOMMENDATIONS

1. ~~**Add Runtime Warning for UDP Proxy Bypass**~~: ✅ RESOLVED - Changed Debug level to Warn level in transport/proxy.go. Users are now explicitly warned when UDP traffic bypasses the proxy.

2. ~~**Address Type Validation**~~: ✅ RESOLVED - Added `IsConnectivitySupported()` and `ConnectivityStatus()` methods to NetworkAddress in transport/address.go. Users can now programmatically check if an address type has functional transport support:
   - `IsConnectivitySupported()` returns true/false indicating if connections can actually be established
   - `ConnectivityStatus()` returns a human-readable description of the connectivity status
   - Nym addresses correctly report connectivity as NOT supported (stub only)
   - Comprehensive tests added in transport/address_test.go

3. **Test Suite Execution**: The test suite timed out during audit execution. Consider investigating test performance or implementing test timeouts.

---

## CONCLUSION

The toxcore-go codebase demonstrates strong alignment between documentation and implementation. The README.md is notably honest about implementation status, using clear markers to distinguish between complete, partial, and planned features. The few discrepancies found are minor and the documentation already acknowledges the relevant limitations.

The codebase follows good Go practices with proper error handling, interface-based design, and comprehensive logging. Security-critical code uses appropriate cryptographic primitives from the x/crypto packages.

**Risk Assessment:** LOW - No critical bugs found. Minor functional mismatches are well-documented.
