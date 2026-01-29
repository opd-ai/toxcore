# Functional Audit Report

**Date:** 2026-01-29  
**Package:** github.com/opd-ai/toxcore  
**Auditor:** Automated Code Audit  
**Version:** Based on current repository state

---

## AUDIT SUMMARY

This audit examines discrepancies between the documented functionality in README.md and the actual implementation. The codebase demonstrates strong overall quality with comprehensive documentation, but some documentation-to-implementation gaps remain.

| Category | Count | Severity Impact | Status |
|----------|-------|-----------------|--------|
| FUNCTIONAL MISMATCH | 2 | Medium | Open |
| MISSING FEATURE | 2 | Low (Documented as Planned) | Open |
| EDGE CASE BUG | 1 | Low | Open |
| DOCUMENTATION INACCURACY | 3 | Low | ✅ **ALL FIXED** |

**Overall Assessment:** The codebase is well-documented with clear statements about feature limitations. Most gaps between documentation and implementation are explicitly acknowledged in the README.md's roadmap section. No critical bugs affecting core functionality were identified.

**Recent Updates (2026-01-29):**
- ✅ Fixed OnFriendMessage callback signature documentation in ASYNC.md
- ✅ Added IsAsyncMessagingAvailable() API method with comprehensive tests
- ✅ Verified all ToxAV example directories exist and contain working code

---

## DETAILED FINDINGS

### FUNCTIONAL MISMATCH: Proxy Transport Does Not Route Network Traffic

**File:** transport/proxy.go:101-116  
**Severity:** Medium  
**Description:** The `ProxyTransport.Send()` method delegates directly to the underlying transport's Send method instead of routing packets through the proxy. This means configuring proxy options has no effect on actual network traffic.

**Expected Behavior:** According to README.md lines 103-118, configuring proxy options should route traffic through the specified proxy (SOCKS5 or HTTP).

**Actual Behavior:** The proxy transport creates a dialer but never uses it for packet transmission. The `Send()` method on line 115 simply calls `t.underlying.Send(packet, addr)`, bypassing the proxy entirely.

**Impact:** Users who configure proxy settings expecting traffic to route through Tor or other SOCKS5 proxies will have their traffic sent directly instead. This is a privacy concern for users relying on proxy anonymity.

**Reproduction:** Configure a SOCKS5 proxy in Options, then monitor network traffic - packets will be sent directly to destination rather than through the proxy.

**Code Reference:**
```go
// transport/proxy.go lines 105-116
func (t *ProxyTransport) Send(packet *Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":    "ProxyTransport.Send",
		"packet_type": packet.PacketType,
		"dest_addr":   addr.String(),
		"proxy_type":  t.proxyType,
	}).Debug("Sending packet via proxy transport")

	// For now, delegate to underlying transport
	// Full proxy support would require establishing connections via proxy
	return t.underlying.Send(packet, addr)
}
```

**Note:** The README.md (lines 116-118) correctly documents this limitation: "The proxy configuration API exists but is **not yet implemented**."

~~~~

### FUNCTIONAL MISMATCH: HTTP Proxy Type Returns Error Instead of Fallback

**File:** transport/proxy.go:75-82  
**Severity:** Low  
**Description:** When HTTP proxy type is specified, `NewProxyTransport()` returns an error rather than falling back gracefully. The README documents HTTP as a supported proxy type alongside SOCKS5.

**Expected Behavior:** README line 107 lists `ProxyTypeHTTP` as a valid configuration option, implying HTTP proxies are supported.

**Actual Behavior:** HTTP proxy configuration immediately returns an error with message "HTTP proxy support is not yet implemented for direct transport layer (use SOCKS5 instead)".

**Impact:** Applications configured to use HTTP proxies will fail during initialization rather than falling back to direct connection. This could cause unexpected application failures.

**Reproduction:** Set `options.Proxy.Type = toxcore.ProxyTypeHTTP` and call `toxcore.New(options)`.

**Code Reference:**
```go
// transport/proxy.go lines 75-82
case "http":
	// golang.org/x/net/proxy doesn't support HTTP CONNECT proxies directly
	logrus.WithFields(logrus.Fields{
		"function":   "NewProxyTransport",
		"proxy_type": config.Type,
		"proxy_addr": proxyAddr,
	}).Warn("HTTP proxy support requires custom implementation - currently unsupported for direct transport")
	return nil, fmt.Errorf("HTTP proxy support is not yet implemented for direct transport layer (use SOCKS5 instead)")
```

~~~~

### MISSING FEATURE: Privacy Network Transport Not Implemented

**File:** transport/address.go (entire file)  
**Severity:** Low (Documented as Planned)  
**Description:** The README.md (lines 126-130) lists Tor .onion, I2P .b32.i2p, Nym .nym, and Lokinet .loki as supported network types. While address parsing is implemented, actual network communication over these privacy networks is not functional.

**Expected Behavior:** Users should be able to send/receive packets through Tor, I2P, Nym, or Lokinet networks using the documented address types.

**Actual Behavior:** The `NetworkAddress` type can parse and represent these address types, but there is no transport implementation that actually establishes connections through these networks. The address parsing exists but network transmission does not.

**Impact:** Users cannot use the library for anonymous communication through privacy networks without external system-level configuration.

**Reproduction:** Create a NetworkAddress with AddressTypeOnion type and attempt to send a packet - the packet will not reach a Tor hidden service.

**Code Reference:**
```go
// transport/address.go lines 333-352
func parseOnionAddress(addrStr, network string) (*NetworkAddress, error) {
	host, portStr, err := net.SplitHostPort(addrStr)
	// ... parsing only, no actual Tor connection
	return &NetworkAddress{
		Type:    AddressTypeOnion,
		Data:    []byte(host),
		Port:    port,
		Network: network,
	}, nil
}
```

**Note:** The README.md roadmap section (lines 1327-1330) correctly documents this as "Interface Ready, Implementation Planned".

~~~~

### MISSING FEATURE: Group Chat DHT Discovery Limited to Same Process

**File:** group/chat.go:125-172  
**Severity:** Low (Documented Limitation)  
**Description:** The `queryDHTForGroup()` function queries a local in-process registry rather than the distributed DHT network. Cross-process group discovery is not functional.

**Expected Behavior:** Groups created in one Tox instance should be discoverable by other Tox instances on the network via DHT.

**Actual Behavior:** The `groupRegistry` is a process-local `map[uint32]*GroupInfo` that only stores groups created within the same process. The `Join()` function fails with "group not found in DHT" for any group created in a different process.

**Impact:** Applications requiring cross-process or cross-network group chat discovery must implement their own group information sharing mechanism.

**Reproduction:** Create a group in Process A, then attempt to Join that group by ID from Process B - it will fail with group not found error.

**Code Reference:**
```go
// group/chat.go lines 125-130
var groupRegistry = struct {
	sync.RWMutex
	groups map[uint32]*GroupInfo
}{
	groups: make(map[uint32]*GroupInfo),
}

// group/chat.go lines 158-172
func queryDHTForGroup(chatID uint32) (*GroupInfo, error) {
	groupRegistry.RLock()
	defer groupRegistry.RUnlock()

	if info, exists := groupRegistry.groups[chatID]; exists {
		return &GroupInfo{...}, nil
	}

	return nil, fmt.Errorf("group %d not found in DHT", chatID)
}
```

**Note:** The package documentation (group/chat.go lines 1-36) and README.md (lines 1345-1355) correctly document this limitation.

~~~~

### EDGE CASE BUG: Async Message Send Fails Silently When No Pre-Keys Available

**File:** async/manager.go:106-130  
**Severity:** Low  
**Description:** When sending an async message to a recipient without pre-exchanged keys, the error message is informative but the pre-key exchange initiation is not automatic when the recipient comes online later.

**Expected Behavior:** Based on README.md line 1069, "Privacy protection works automatically with existing APIs."

**Actual Behavior:** If `SendAsyncMessage()` is called before pre-keys have been exchanged with the recipient, it returns an error. Pre-key exchange only happens when `SetFriendOnlineStatus()` is called with `online=true`, which requires application-level coordination.

**Impact:** Users may expect async messaging to "just work" for offline friends, but pre-key exchange requires both parties to have been online at the same time at least once.

**Reproduction:** 
1. Create two Tox instances A and B that have never been online simultaneously
2. Instance A calls `SendAsyncMessage()` to Instance B
3. Operation fails with "no pre-keys available for recipient"

**Code Reference:**
```go
// async/manager.go lines 113-119
func (am *AsyncManager) SendAsyncMessage(recipientPK [32]byte, message string, messageType MessageType) error {
	// ...
	if !am.forwardSecurity.CanSendMessage(recipientPK) {
		return fmt.Errorf("no pre-keys available for recipient %x - cannot send message. Exchange keys when both parties are online", recipientPK[:8])
	}
	// ...
}
```

~~~~

### ~~DOCUMENTATION INACCURACY: Async Manager Storage Node Failure Behavior~~

**Status:** ✅ FIXED  
**File:** toxcore.go:3208-3221  
**Severity:** Low  
**Description:** README.md states "Users can become storage nodes when async manager initialization succeeds" and "If storage node initialization fails, async messaging features will be unavailable but core Tox functionality remains intact." However, the code logs a warning and continues with `nil` asyncManager.

**Resolution:** Added `IsAsyncMessagingAvailable()` method to the Tox struct that returns `true` if async messaging features are available and `false` otherwise. Applications can now programmatically check async availability before using async-related methods. Added comprehensive test coverage in `onasync_api_test.go`.

**Code Reference:**
```go
// toxcore.go lines 3216-3221
// IsAsyncMessagingAvailable returns true if async messaging features are available.
// Returns false if async manager initialization failed during Tox instance creation.
// Applications should check this before calling async-related methods.
func (t *Tox) IsAsyncMessagingAvailable() bool {
	return t.asyncManager != nil
}
```

~~~~

### ~~DOCUMENTATION INACCURACY: OnFriendMessage Callback Signature in ASYNC.md~~

**Status:** ✅ FIXED  
**File:** docs/ASYNC.md:845  
**Severity:** Low  
**Description:** The ASYNC.md documentation shows `OnFriendMessage` with a different callback signature than the actual implementation.

**Resolution:** Updated ASYNC.md to show the correct callback signature for `OnFriendMessage`, which uses `SimpleFriendMessageCallback func(friendID uint32, message string)`. Users needing message type should use `OnFriendMessageDetailed` instead.

**Corrected Documentation (docs/ASYNC.md:845):**
```go
// Handle regular messages (simple callback)
tox.OnFriendMessage(func(friendID uint32, message string) {
    log.Printf("Real-time message from friend %d: %s", friendID, message)
})
```

**Actual Implementation (toxcore.go:1589, 1608):**
```go
type SimpleFriendMessageCallback func(friendID uint32, message string)

func (t *Tox) OnFriendMessage(callback SimpleFriendMessageCallback)
```

~~~~

### ~~DOCUMENTATION INACCURACY: ToxAV Examples README References Non-Existent Examples~~

**Status:** ✅ VERIFIED  
**File:** examples/ToxAV_Examples_README.md, README.md:878-911  
**Severity:** Low  
**Description:** README.md references specific example directories under `examples/` for ToxAV functionality (toxav_basic_call/, toxav_audio_call/, toxav_video_call/, audio_effects_demo/, toxav_call_control_demo/) that may not all exist or may have different names.

**Resolution:** Verified that all referenced example directories exist with working code:
- ✅ `examples/toxav_basic_call/` - Contains main.go and README.md
- ✅ `examples/toxav_audio_call/` - Contains main.go and README.md
- ✅ `examples/toxav_video_call/` - Contains main.go and README.md
- ✅ `examples/audio_effects_demo/` - Contains main.go
- ✅ `examples/toxav_call_control_demo/` - Contains main.go

**Impact:** None - documentation is accurate and all examples exist.

~~~~

## VERIFICATION NOTES

### Properly Documented Limitations

The following features are correctly documented as "planned" or "limited" in the README.md roadmap section:

1. **Privacy Network Transport** (lines 1327-1337): Correctly states "Interface Ready, Implementation Planned"
2. **Proxy Support** (lines 1357-1363): Correctly states "API Ready, Implementation Pending"
3. **Group Chat DHT Discovery** (lines 1345-1355): Correctly states "(Limited Implementation)"
4. **Local Network Discovery** (lines 1337-1341): Actually fully implemented in dht/local_discovery.go

### Features Working As Documented

The following major features were verified to work according to documentation:

1. **Core Tox Protocol**: Friend management, messaging, connection status ✅
2. **Noise Protocol Framework Integration**: NegotiatingTransport properly wraps UDP/TCP ✅
3. **State Persistence**: GetSavedata/NewFromSavedata/Load work correctly ✅
4. **ToxAV Integration**: ToxAV callbacks and methods match documentation ✅
5. **Self Management API**: SelfSetName, SelfGetName, SelfSetStatusMessage work as documented ✅
6. **Message API**: SendFriendMessage variadic parameter for message type works correctly ✅
7. **Local Discovery**: LANDiscovery implementation is complete and functional ✅

---

## RECOMMENDATIONS

1. **Proxy Implementation Priority**: Consider implementing actual proxy routing in ProxyTransport.Send() since users configuring proxies expect traffic to route through them.

2. ~~**API for Async Status**: Add a method like `IsAsyncMessagingAvailable() bool` to the Tox struct so applications can determine if async features are active.~~ ✅ **COMPLETED**

3. ~~**Documentation Sync**: Update docs/ASYNC.md callback signature to match actual implementation.~~ ✅ **COMPLETED**

4. ~~**Example Verification**: Ensure all example directories referenced in documentation exist and contain working code.~~ ✅ **VERIFIED**

---

*End of Audit Report*
