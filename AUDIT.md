# Implementation Gap Analysis
Generated: 2026-01-30T19:57:25Z
Codebase Version: ad7561404d0265c6ea7a9cb859eb39fab131780e

## Executive Summary
Total Gaps Found: 6
- Critical: 0
- Moderate: 3 (1 completed)
- Minor: 2
- Completed: 1

---

## Detailed Findings

### Gap #1: Storage Capacity Update Interval Mismatch ✅ COMPLETED
**Status:** Fixed in commit ad7561404d0265c6ea7a9cb859eb39fab131780e+

**Documentation Reference:**
> "Capacity automatically updates based on available disk space ... Automatically updates every 5 minutes during operation" (README.md:1256-1261)

**Implementation Location:** `async/manager.go:284`

**Expected Behavior:** Storage capacity automatically updates every 5 minutes during operation.

**Actual Implementation:** ~~Storage capacity updates every 1 hour (not 5 minutes).~~ **FIXED:** Now updates every 5 minutes as documented.

**Gap Details:** The `storageMaintenanceLoop()` function creates a capacity ticker with `time.NewTicker(1 * time.Hour)` at line 284, but the README.md documents that capacity "automatically updates every 5 minutes during operation."

**Resolution:** Changed `async/manager.go:284` from `time.NewTicker(1 * time.Hour)` to `time.NewTicker(5 * time.Minute)` to match the documented behavior.

**Changes Made:**
```diff
-		capacity: time.NewTicker(1 * time.Hour),    // Update capacity every hour
+		capacity: time.NewTicker(5 * time.Minute),  // Update capacity every 5 minutes
```

**Verification:** Code compiles successfully and async manager tests pass.

---

### Gap #2: ToxAV CallbackAudioReceiveFrame sampleCount Type Mismatch
**Documentation Reference:**
> "toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount uint16, channels uint8, samplingRate uint32)" (README.md:959-960)

**Implementation Location:** `toxav.go:1173`

**Expected Behavior:** The `sampleCount` parameter should be `uint16` as documented.

**Actual Implementation:** The `sampleCount` parameter is `int`, not `uint16`.

**Gap Details:** The README shows the callback signature using `sampleCount uint16`, but the actual implementation uses `sampleCount int`. This is a type mismatch that would cause compilation errors if developers copy code directly from the README.

**Reproduction:**
```go
// README example (line 959-962):
toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, 
    sampleCount uint16, channels uint8, samplingRate uint32) {
    // This won't compile - type mismatch
})

// Actual working code:
toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, 
    sampleCount int, channels uint8, samplingRate uint32) {
    // This compiles correctly
})
```

**Production Impact:** Moderate - Developers copying example code from README will get compilation errors and must discover the correct type through trial and error or code inspection.

**Evidence:**
```go
// From toxav.go:1173
func (av *ToxAV) CallbackAudioReceiveFrame(callback func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32)) {
```

---

### Gap #3: ToxAV CallbackVideoReceiveFrame Signature Mismatch (Missing Stride Parameters)
**Documentation Reference:**
> "toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte) {" (README.md:964-965)

**Implementation Location:** `toxav.go:1202`

**Expected Behavior:** The callback should have signature `func(friendNumber uint32, width, height uint16, y, u, v []byte)` with 6 parameters as documented.

**Actual Implementation:** The callback has signature `func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int)` with 9 parameters.

**Gap Details:** The README omits the three stride parameters (`yStride`, `uStride`, `vStride`) from the callback signature. These are essential for correctly interpreting video frame data in non-contiguous memory layouts.

**Reproduction:**
```go
// README example (line 964-967):
toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, 
    y, u, v []byte) {
    // This won't compile - missing stride parameters
})

// Actual working code:
toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, 
    y, u, v []byte, yStride, uStride, vStride int) {
    // This compiles correctly
})
```

**Production Impact:** Moderate - Developers copying example code will get compilation errors. Additionally, omitting stride parameters from documentation may lead developers to incorrectly process video frames assuming contiguous memory.

**Evidence:**
```go
// From toxav.go:1202
func (av *ToxAV) CallbackVideoReceiveFrame(callback func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int)) {
```

---

### Gap #4: MinStorageCapacity Constant Not Defined in Code
**Documentation Reference:**
> "MinStorageCapacity = 1536 // Minimum storage capacity (1MB / ~650 bytes per message)" (README.md:1252)

**Implementation Location:** `async/storage_limits.go:239`

**Expected Behavior:** A constant `MinStorageCapacity = 1536` should be defined and used for capacity bounds checking.

**Actual Implementation:** The constant `MinStorageCapacity` is not defined. Instead, a local constant `minCapacity = 100` is used in `EstimateMessageCapacity()`.

**Gap Details:** The README documents `MinStorageCapacity = 1536` as a configuration constant, but the implementation uses a different value (`100`) defined as a local constant within a function. The value `1536` comes from the calculation `1MB / ~650 bytes = ~1536 messages`, but the actual minimum in code is 100 messages.

**Reproduction:**
```go
// README.md documentation (line 1252):
const MinStorageCapacity = 1536  // Expected constant

// Actual implementation in async/storage_limits.go:239-243:
const minCapacity = 100  // Local constant, different name and value
if capacity < minCapacity {
    finalCapacity = minCapacity
```

**Production Impact:** Moderate - The documented minimum of 1536 messages is ~15x higher than the actual minimum of 100 messages. Systems with very limited disk space may accept more messages than documented (up to 1536), potentially causing storage issues, or conversely may reject storage node participation when documentation suggests they should be able to participate.

**Evidence:**
```go
// From async/storage_limits.go:239-243
const minCapacity = 100
const maxCapacity = 100000

var finalCapacity int
if capacity < minCapacity {
    finalCapacity = minCapacity
```

---

### Gap #5: Proxy Transport Does Not Route UDP Traffic Through Proxy
**Documentation Reference:**
> "The proxy configuration API exists but is **not yet implemented**. Setting proxy options will have no effect on network traffic." (README.md:116-117)

**Implementation Location:** `transport/proxy.go:130-153`

**Expected Behavior:** Based on the documented "not yet implemented" status, one might expect proxy configuration to do nothing.

**Actual Implementation:** The proxy transport IS implemented and functional for TCP connections, but silently falls back to direct transmission for UDP without warning.

**Gap Details:** The README states proxy support is "not yet implemented," but the `transport/proxy.go` file contains a working `ProxyTransport` implementation. However, the implementation only proxies TCP connections. For UDP (the default Tox transport), the `Send()` method silently delegates to the underlying transport without using the proxy, as shown at lines 145-152.

**Reproduction:**
```go
// Configure proxy expecting all traffic to be proxied
options := toxcore.NewOptions()
options.Proxy = &toxcore.ProxyOptions{
    Type: toxcore.ProxyTypeSOCKS5,
    Host: "127.0.0.1",
    Port: 9050,
}

tox, _ := toxcore.New(options)
// UDP traffic will NOT go through proxy (silent fallback)
// Only TCP traffic will be proxied
```

**Production Impact:** Minor - Users expecting Tor/SOCKS5 proxy protection may have UDP traffic leak outside the proxy. While the README correctly warns that proxy "will have no effect," the actual behavior is more nuanced: TCP works while UDP doesn't, which could create a false sense of security.

**Evidence:**
```go
// From transport/proxy.go:130-152
func (t *ProxyTransport) Send(packet *Packet, addr net.Addr) error {
    // ...
    
    // Check if underlying transport is connection-oriented using interface method
    if t.underlying.IsConnectionOriented() {
        return t.sendViaTCPProxy(packet, addr)
    }

    // For connectionless transports, delegate to underlying transport
    // Note: Full UDP proxy support would require SOCKS5 UDP association
    logrus.WithFields(logrus.Fields{
        // ...
    }).Debug("Delegating to underlying connectionless transport (proxy not applicable)")

    return t.underlying.Send(packet, addr)  // <-- UDP bypasses proxy silently
}
```

---

### Gap #6: MaxStorageCapacity Value Documented But maxCapacity Implementation Differs
**Documentation Reference:**
> "MaxStorageCapacity = 1536000 // Maximum storage capacity (1GB / ~650 bytes per message)" (README.md:1253)

**Implementation Location:** `async/storage_limits.go:240` and `async/storage.go:43`

**Expected Behavior:** Maximum storage capacity should be 1,536,000 messages as documented.

**Actual Implementation:** The `MaxStorageCapacity = 1536000` constant exists in `storage.go`, but `EstimateMessageCapacity()` uses a local `maxCapacity = 100000` that caps the actual maximum at 100,000 messages.

**Gap Details:** There's a disconnect between the documented/defined constant `MaxStorageCapacity = 1536000` and the actual enforcement in `EstimateMessageCapacity()` which uses `maxCapacity = 100000`. This means even with 1GB of available space, the system will only store up to 100,000 messages instead of the documented 1,536,000.

**Reproduction:**
```go
// async/storage.go:43 defines:
MaxStorageCapacity = 1536000  // ~1.5M messages

// But async/storage_limits.go:240 enforces:
const maxCapacity = 100000  // Only 100K messages max

// Result: 1GB storage = min(1GB/650, 100000) = 100,000 messages, not 1,536,000
```

**Production Impact:** Minor - Systems with large available disk space will be artificially capped at ~6.5% of their potential capacity. Storage nodes may reject messages earlier than expected based on documented limits.

**Evidence:**
```go
// From async/storage.go:43
MaxStorageCapacity = 1536000  // Documented constant

// From async/storage_limits.go:240,250-253
const maxCapacity = 100000  // Actual enforcement

if capacity > maxCapacity {
    finalCapacity = maxCapacity  // Caps at 100000, not 1536000
```

---

## Summary

| # | Gap Description | Severity | Impact Area | Status |
|---|----------------|----------|-------------|--------|
| 1 | Storage capacity update interval (1h vs 5min) | Moderate | Async Storage | ✅ COMPLETED |
| 2 | CallbackAudioReceiveFrame sampleCount type (int vs uint16) | Moderate | ToxAV API | Pending |
| 3 | CallbackVideoReceiveFrame missing stride parameters | Moderate | ToxAV API | Pending |
| 4 | MinStorageCapacity constant (100 vs 1536) | Moderate | Async Storage | Pending |
| 5 | UDP proxy silently bypasses SOCKS5/HTTP proxy | Minor | Network/Privacy | Pending |
| 6 | MaxStorageCapacity enforcement (100K vs 1.5M) | Minor | Async Storage | Pending |

## Recommendations

1. **Gap #1**: ✅ COMPLETED - Updated `async/manager.go:284` to use `time.NewTicker(5 * time.Minute)`.

2. **Gap #2 & #3**: Update README.md callback examples to match actual implementation signatures:
   - `sampleCount int` instead of `sampleCount uint16`
   - Add `yStride, uStride, vStride int` parameters to video callback

3. **Gap #4 & #6**: Either:
   - Define `MinStorageCapacity` and use `MaxStorageCapacity` in `EstimateMessageCapacity()`, OR
   - Update README to document actual values (100 min, 100000 max)

4. **Gap #5**: Update README proxy section to clarify that TCP proxy works but UDP does not, rather than stating it's "not yet implemented."
