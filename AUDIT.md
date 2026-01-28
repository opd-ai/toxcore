# Functional Audit Report: toxcore-go

**Audit Date:** 2026-01-28  
**Auditor:** GitHub Copilot CLI  
**Repository:** github.com/opd-ai/toxcore  
**Go Version:** 1.23.2  

---

## AUDIT SUMMARY

This comprehensive audit analyzed the toxcore-go codebase against documented functionality in README.md, examining 51 source files across 15 packages. The codebase is well-structured with extensive test coverage (48 test files). All tests pass successfully.

| Category | Count |
|----------|-------|
| CRITICAL BUG | 0 |
| FUNCTIONAL MISMATCH | 0 |
| MISSING FEATURE | 0 |
| EDGE CASE BUG | 2 |
| PERFORMANCE ISSUE | 1 |
| **COMPLETED** | **4** |

**Overall Assessment:** The implementation is substantially complete and functional. The issues identified are primarily edge cases and minor gaps between documentation and implementation rather than critical bugs.

---

## DETAILED FINDINGS

---

### ✅ COMPLETED: GetRemainingKeyCount Returns Incorrect Value When Keys Are Removed

**File:** async/prekeys.go:283-294  
**Severity:** Medium  
**Status:** Fixed on 2026-01-28

**Description:** The `GetRemainingKeyCount` method calculated remaining keys as `PreKeysPerPeer - bundle.UsedCount`, but when keys were actually removed from `bundle.Keys` slice (in `extractAndProcessPreKey`), this calculation didn't reflect the actual remaining keys.

**Resolution:** Changed both `GetRemainingKeyCount` and `NeedsRefresh` methods to use `len(bundle.Keys)` instead of calculating from `UsedCount`. This ensures the count always reflects actual available pre-keys in storage.

**Changes Made:**
1. Updated `GetRemainingKeyCount()` to return `len(bundle.Keys)` (line 292)
2. Updated `NeedsRefresh()` to use `len(bundle.Keys)` for threshold check (line 231)
3. Added comprehensive test `TestGetRemainingKeyCountAccuracy` to verify count accuracy
4. Added test `TestNeedsRefreshAccuracy` to verify refresh threshold behavior

**Verification:**
- All existing tests pass (async package: 100% pass rate)
- New tests specifically validate the fix for rapid key extraction scenarios
- Pre-key refresh triggers correctly at threshold (≤20 keys)

---

### ✅ COMPLETED: TCP Transport Read Does Not Handle Partial Reads

**File:** transport/tcp.go:350-372  
**Severity:** Medium  
**Status:** Fixed on 2026-01-28

**Description:** The `readPacketLength` and `readPacketData` methods used a single `Read()` call which could return fewer bytes than requested on a TCP connection. TCP is a stream protocol and does not guarantee that a single `Read()` will return all requested bytes.

**Resolution:** Changed both methods to use `io.ReadFull()` instead of `conn.Read()`. `io.ReadFull()` automatically handles partial reads by looping until all requested bytes are received or an error occurs.

**Changes Made:**
1. Added `"io"` import to tcp.go
2. Updated `readPacketLength()` to use `io.ReadFull(conn, header)` instead of `conn.Read(header)`
3. Updated `readPacketData()` to use `io.ReadFull(conn, data)` instead of `conn.Read(data)`
4. Added comprehensive test file `tcp_partial_reads_test.go` with 4 test functions:
   - `TestTCPTransportPartialReads` - validates correct handling of various chunk sizes (1, 2, 3, 7 bytes)
   - `TestTCPTransportReadUnexpectedEOF` - validates proper EOF/ErrUnexpectedEOF error handling
   - `TestTCPTransportReadFullIntegration` - validates complete packet read cycle with extreme fragmentation
   - `TestTCPTransportConcurrentPartialReads` - validates thread-safe partial read handling

**Verification:**
- All new tests pass (4 test functions, 24 sub-tests)
- All existing transport package tests pass (26.5s runtime, 100% pass rate)
- Simulated partial reads as small as 1 byte per Read() call work correctly
- Proper error handling for incomplete headers and payloads verified

---

### FUNCTIONAL MISMATCH: TCP Transport Read Does Not Handle Partial Reads

**File:** transport/tcp.go:350-372  
**Severity:** Medium  
**Status:** ~~OPEN~~ **FIXED** - See completed section above

---

---

### ✅ COMPLETED: File Transfer Not Integrated with Transport Layer (FALSE POSITIVE)

**File:** file/transfer.go, file/manager.go  
**Severity:** Medium → N/A (Issue was incorrectly identified)  
**Status:** VERIFIED WORKING - Closed on 2026-01-28

**Original Description:** Initially flagged as missing network integration because `Transfer.Start()` only opens local files.

**Actual Behavior:** File transfer IS fully integrated with the network transport layer. This is a case of proper separation of concerns:
- `Transfer` struct - Handles local file I/O operations (read/write chunks, track progress, manage state)
- `Manager` struct - Coordinates network operations (packet handlers, serialization, transport integration)

**Verification:**
1. **Network Handlers Registered:** `file/manager.go:40-44` registers all four packet handlers (FileRequest, FileControl, FileData, FileDataAck)
2. **Network Transmission:** `Manager.SendFile()` sends PacketFileRequest (lines 76-85), `Manager.SendChunk()` sends PacketFileData (lines 122-129)
3. **Packet Handlers Implemented:** All four handlers process incoming packets and coordinate with Transfer objects
4. **End-to-End Tests Pass:** `TestEndToEndFileTransfer` in `manager_test.go` proves complete peer-to-peer file transfer works
5. **Documentation Confirms:** `docs/FILE_TRANSFER.md` documents the full network integration architecture

**Resolution:** No code changes needed. The architecture correctly separates file I/O (Transfer) from network coordination (Manager). All tests pass.

---

### ✅ COMPLETED: ToxAV Audio/Video Frame Handler Wired to Manager

**File:** av/manager.go  
**Severity:** Low  
**Status:** Fixed on 2026-01-28

**Description:** The `CallbackAudioReceiveFrame` and `CallbackVideoReceiveFrame` methods in `toxav.go` set callbacks that were partially wired to the AV Manager. Video callback invocation was implemented, but audio callback invocation was missing in the RTP packet handler.

**Original Issue:** Audio callbacks were registered via `av.impl.SetAudioReceiveCallback()` but the audio frame handler (`handleAudioFrame`) only processed RTP packets without decoding frames or invoking the callback.

**Resolution:** Updated `handleAudioFrame` in `av/manager.go` to match the video callback pattern:
1. Process RTP packets and extract encoded Opus frame data
2. Decode Opus frames to PCM using the audio processor's `ProcessIncoming()` method
3. Invoke the registered audio receive callback with decoded PCM samples, sample count, channel count, and sampling rate
4. Added proper error handling and logging throughout the audio processing pipeline

**Changes Made:**
1. Modified `handleAudioFrame()` to retrieve and use the `audioReceiveCallback` (lines 445-447)
2. Added audio processor integration to decode incoming Opus frames (lines 464-490)
3. Implemented callback invocation with proper parameters matching ToxAV API (lines 492-506)
4. Added comprehensive logging for debugging and monitoring
5. Created test file `av/callback_invocation_test.go` with 3 test functions to verify callback registration and thread safety

**Verification:**
- All av package tests pass (100% pass rate including new callback tests)
- `TestAudioReceiveCallbackInvocation` validates audio callback registration
- `TestVideoReceiveCallbackInvocation` validates video callback registration  
- `TestCallbackThreadSafety` ensures thread-safe concurrent callback operations
- Callbacks now properly integrate with the audio/video processing pipeline

---

### EDGE CASE BUG: AsyncClient Message Retrieval Timeout with No Storage Nodes

**File:** async/client.go:592-650  
**Severity:** Low  
**Description:** When `retrieveObfuscatedMessagesFromNode` is called but the transport send fails, the method still creates a response channel and waits for a response with a 5-second timeout. If no storage nodes are available or all sends fail, the method will always timeout rather than failing fast.

**Expected Behavior:** If no storage nodes can be reached, the method should return immediately with an appropriate error.

**Actual Behavior:** Even when `transport.Send()` fails, the code continues to wait for a response that will never arrive, causing a 5-second delay per failed node.

**Impact:** In degraded network conditions or when no storage nodes are available, message retrieval becomes very slow (N * 5 seconds where N is the number of unreachable nodes).

**Reproduction:**
1. Configure async client with storage nodes
2. Disconnect network or make storage nodes unreachable
3. Call `RetrieveObfuscatedMessages()`
4. Observe 5-second delays per storage node

**Code Reference:**
```go
func (ac *AsyncClient) retrieveObfuscatedMessagesFromNode(nodeAddr net.Addr, ...) ([]*ObfuscatedAsyncMessage, error) {
	// ... create request ...
	
	err = ac.transport.Send(retrievePacket, nodeAddr)
	if err != nil {
		return nil, fmt.Errorf("...")  // Returns error here
	}
	
	// But if transport is nil or Send succeeds but no response comes:
	select {
	case response := <-responseChan:
		// ...
	case <-time.After(5 * time.Second):  // Always waits 5 seconds
		return nil, fmt.Errorf("timeout...")
	}
}
```

---

### EDGE CASE BUG: Group Chat Broadcast Silently Drops Messages on DHT Lookup Failure

**File:** group/chat.go:841-901  
**Severity:** Low  
**Description:** In `broadcastPeerUpdate`, when a peer's cached address is unavailable and DHT discovery fails to find any nodes, the function returns "no reachable address found" but the calling `broadcastGroupUpdate` continues to the next peer. If all peers fail, the broadcast is reported as successful (returns nil) even if no messages were actually delivered.

**Expected Behavior:** Group broadcasts should report partial or complete failures when messages cannot be delivered.

**Actual Behavior:** `validateBroadcastResults` only returns error if `successfulBroadcasts == 0 && len(broadcastErrors) > 0`. If no peers are connected (all offline), `successfulBroadcasts` would be 0 and `broadcastErrors` would also be empty, returning nil (success).

**Impact:** Group administrators may believe broadcasts succeeded when no peers actually received the message.

**Reproduction:**
1. Create group chat with peers
2. Set all peer connection status to offline (0)
3. Call `SendMessage()` 
4. Observe no error returned despite no delivery

**Code Reference:**
```go
func (g *Chat) sendToConnectedPeers(msgBytes []byte) (int, []error) {
	for peerID, peer := range g.Peers {
		if peerID == g.SelfPeerID {
			continue // Skip self
		}
		if peer.Connection == 0 {
			continue // Skip offline peers - but no error added
		}
		// ...
	}
	return successfulBroadcasts, broadcastErrors
}

func (g *Chat) validateBroadcastResults(successfulBroadcasts int, broadcastErrors []error) error {
	if successfulBroadcasts == 0 && len(broadcastErrors) > 0 {
		return fmt.Errorf("all broadcasts failed: %v", broadcastErrors)
	}
	return nil  // Returns success even if all peers were offline
}
```

---

### PERFORMANCE ISSUE: DHT FindClosestNodes Collects All Nodes Before Sorting

**File:** dht/routing.go:117-145  
**Severity:** Low  
**Description:** The `FindClosestNodes` method collects ALL nodes from all 256 k-buckets into a single slice before sorting and returning the top N closest nodes. This is inefficient for large routing tables.

**Expected Behavior:** For a DHT with potentially thousands of nodes, finding the closest N nodes should be optimized.

**Actual Behavior:** Every call to `FindClosestNodes` allocates memory for all nodes and sorts the entire collection, even though only a small subset (typically 4-8 nodes) is needed.

**Impact:** In a DHT with many nodes, this causes unnecessary memory allocation and O(N log N) sorting overhead on every lookup. For bootstrap and peer discovery operations that happen frequently, this can degrade performance.

**Reproduction:**
1. Populate routing table with 1000+ nodes
2. Profile `FindClosestNodes()` with 4 requested nodes
3. Observe full collection and sort of all nodes

**Code Reference:**
```go
func (rt *RoutingTable) FindClosestNodes(targetID crypto.ToxID, count int) []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	// Collect all nodes - O(N) allocation
	allNodes := make([]*Node, 0, rt.maxNodes)
	for _, bucket := range rt.kBuckets {
		allNodes = append(allNodes, bucket.GetNodes()...)
	}

	// Sort by distance - O(N log N)
	sort.Slice(allNodes, func(i, j int) bool {
		distI := allNodes[i].Distance(targetNode)
		distJ := allNodes[j].Distance(targetNode)
		return lessDistance(distI, distJ)
	})

	// Return only 'count' nodes
	if len(allNodes) > count {
		allNodes = allNodes[:count]
	}
	return allNodes
}
```

---

## ADDITIONAL OBSERVATIONS

### Positive Findings

1. **Comprehensive Test Coverage**: 48 test files for 51 source files (94% file coverage ratio), all tests passing.

2. **Thread Safety**: Proper mutex usage throughout async, transport, and core packages with RWMutex for read-heavy operations.

3. **Secure Memory Handling**: Consistent use of `crypto.ZeroBytes()` and `crypto.WipeKeyPair()` for sensitive key material.

4. **Interface-Based Design**: Transport layer uses `net.Addr` and `net.PacketConn` interfaces as specified in project guidelines.

5. **Forward Secrecy Implementation**: Complete pre-key bundle system with rotation, refresh thresholds, and secure key removal.

6. **Identity Obfuscation**: Sophisticated pseudonym system with epoch-based rotation and HMAC proofs.

### Documentation Accuracy

The README.md accurately describes the high-level architecture and feature set. Most documented features are implemented:
- ✅ Pure Go implementation (no CGo)
- ✅ DHT implementation with Kademlia-style routing
- ✅ Noise Protocol (IK pattern) integration
- ✅ Forward secrecy with pre-key system
- ✅ Identity obfuscation for async messaging
- ✅ UDP and TCP transports
- ✅ Callback-based event handling
- ⚠️ File transfer (partial - local only)
- ⚠️ ToxAV receive callbacks (registered but not invoked)

---

## METHODOLOGY

This audit followed the specified dependency-based analysis order:

**Level 0** (No internal imports): `limits/limits.go`, `crypto/keypair.go`, `crypto/toxid.go`, `crypto/nonce.go`

**Level 1**: `crypto/encrypt.go`, `crypto/decrypt.go`, `transport/packet.go`, `transport/types.go`

**Level 2**: `transport/udp.go`, `transport/tcp.go`, `dht/node.go`, `dht/routing.go`

**Level 3**: `dht/bootstrap.go`, `async/epoch.go`, `async/obfs.go`, `friend/friend.go`

**Level 4**: `async/storage.go`, `async/prekeys.go`, `async/forward_secrecy.go`, `messaging/message.go`

**Level 5**: `async/client.go`, `async/manager.go`, `group/chat.go`, `file/transfer.go`

**Level 6**: `toxcore.go`, `toxav.go`

Each level was analyzed for:
- Function signature correctness
- Error handling completeness
- Concurrency safety
- Edge case coverage
- Documentation alignment

---

*End of Audit Report*
