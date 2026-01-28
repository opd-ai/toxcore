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
| FUNCTIONAL MISMATCH | 1 |
| MISSING FEATURE | 2 |
| EDGE CASE BUG | 2 |
| PERFORMANCE ISSUE | 1 |
| **COMPLETED** | **1** |

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

### FUNCTIONAL MISMATCH: TCP Transport Read Does Not Handle Partial Reads

**File:** transport/tcp.go:350-372  
**Severity:** Medium  
**Description:** The `readPacketLength` and `readPacketData` methods use a single `Read()` call which may return fewer bytes than requested on a TCP connection. TCP is a stream protocol and does not guarantee that a single `Read()` will return all requested bytes.

**Expected Behavior:** According to TCP semantics and the Tox protocol specification, packet data should be reliably read in full before processing.

**Actual Behavior:** A single `Read()` call may return partial data, causing packet parsing to fail or produce corrupted data. The method does not loop until all bytes are received.

**Impact:** On slow or congested networks, TCP packet transmission could fail intermittently, causing message loss or protocol errors.

**Reproduction:**
1. Establish TCP connection with artificial network delay/throttling
2. Send large packets that trigger fragmentation
3. Observe partial reads causing packet parse failures

**Code Reference:**
```go
// readPacketLength reads the 4-byte packet length header and returns the parsed length.
func (t *TCPTransport) readPacketLength(conn net.Conn, header []byte) (uint32, error) {
	_, err := conn.Read(header)  // May return partial read
	if err != nil {
		return 0, err
	}
	// ... parsing assumes all 4 bytes were read
}

// readPacketData reads packet data of the specified length from the connection.
func (t *TCPTransport) readPacketData(conn net.Conn, length uint32) ([]byte, error) {
	data := make([]byte, length)
	_, err := conn.Read(data)  // May return partial read
	if err != nil {
		return nil, err
	}
	return data, nil  // May be incomplete
}
```

---

### MISSING FEATURE: File Transfer Not Integrated with Transport Layer

**File:** file/transfer.go, file/manager.go  
**Severity:** Medium  
**Description:** The file transfer functionality documented in README.md is implemented at the struct level with `Transfer` and `TransferManager`, but there is no integration with the transport layer to actually send/receive file chunks over the network. The `Start()` method opens local files but does not establish network communication.

**Expected Behavior:** README documents "File transfer operations" as part of the feature set, implying end-to-end file transfer between Tox peers.

**Actual Behavior:** File transfers are only local file operations. There is no network packet handler for `PacketFileData`, no friend resolution for file transfer targets, and no actual network transmission of file chunks.

**Impact:** Users cannot transfer files between Tox peers. The feature is documented but not usable in practice for peer-to-peer communication.

**Reproduction:**
1. Create a file transfer with `NewTransfer()`
2. Call `Start()` and `ReadChunk()`
3. Observe that chunks are only read from local filesystem, never sent to peers

**Code Reference:**
```go
// transfer.go - Start only opens local files, no network integration
func (t *Transfer) Start() error {
	// ... state validation ...
	if t.Direction == TransferDirectionOutgoing {
		t.FileHandle, err = os.Open(t.FileName)  // Local file only
	} else {
		t.FileHandle, err = os.Create(t.FileName)
	}
	// No transport.Send() or network integration
	return nil
}
```

---

### MISSING FEATURE: ToxAV Audio/Video Frame Handler Not Wired to Manager

**File:** toxav.go:1106-1156  
**Severity:** Low  
**Description:** The `CallbackAudioReceiveFrame` and `CallbackVideoReceiveFrame` methods set callbacks on the ToxAV struct and call `av.impl.SetAudioReceiveCallback()` and `av.impl.SetVideoReceiveCallback()`, but the AV Manager (`av/manager.go`) does not invoke these callbacks when processing incoming RTP packets. The callbacks are stored but never triggered.

**Expected Behavior:** README documents "ToxAV integration for audio/video calls" with callback support for receiving media frames.

**Actual Behavior:** Callbacks are registered but the RTP packet handlers in the AV subsystem do not invoke them when audio/video frames are received.

**Impact:** Applications cannot receive incoming audio/video frames during calls, making ToxAV receive functionality non-operational.

**Reproduction:**
1. Create ToxAV instance
2. Register `CallbackAudioReceiveFrame` 
3. Initiate call and have peer send audio
4. Observe callback is never invoked

**Code Reference:**
```go
// toxav.go - Callback is stored but depends on manager wiring
func (av *ToxAV) CallbackAudioReceiveFrame(callback func(...)) {
	av.mu.Lock()
	defer av.mu.Unlock()
	av.audioReceiveCb = callback

	// Wire the callback to the underlying av.Manager
	if av.impl != nil {
		av.impl.SetAudioReceiveCallback(callback)  // Manager must invoke this
	}
}
```

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
