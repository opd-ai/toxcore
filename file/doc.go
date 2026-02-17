// Package file implements file transfer functionality for the Tox protocol,
// providing secure peer-to-peer file transmission with pause, resume, and
// cancellation support.
//
// # Overview
//
// The file package provides two primary components:
//
//   - Transfer: Manages individual file transfer state, progress tracking,
//     and file I/O operations
//   - Manager: Coordinates multiple concurrent transfers with packet routing
//     and friend address resolution
//
// # File Transfers
//
// Create and manage file transfers:
//
//	// Create a new outgoing transfer
//	transfer := file.NewTransfer(friendID, fileID, fileName, fileSize)
//
//	// Set progress callback
//	transfer.OnProgress(func(received uint64) {
//	    progress := float64(received) / float64(fileSize) * 100
//	    fmt.Printf("Progress: %.2f%%\n", progress)
//	})
//
//	// Start the transfer
//	if err := transfer.Start(); err != nil {
//	    log.Fatal(err)
//	}
//
// # Transfer States
//
// Transfers progress through defined states:
//
//	const (
//	    TransferStatePending    // Waiting to start
//	    TransferStateRunning    // In progress
//	    TransferStatePaused     // Temporarily paused
//	    TransferStateCompleted  // Successfully finished
//	    TransferStateCancelled  // Cancelled by user or peer
//	    TransferStateError      // Failed due to error
//	)
//
//	// Control transfer state
//	transfer.Pause()
//	transfer.Resume()
//	transfer.Cancel()
//
//	// Query state
//	state := transfer.GetState()
//	speed := transfer.GetSpeed() // bytes per second
//
// # Transfer Manager
//
// Manager coordinates multiple transfers with network transport:
//
//	// Create manager with transport
//	manager := file.NewManager(transport)
//
//	// Configure friend address resolution
//	manager.SetAddressResolver(func(addr net.Addr) (uint32, bool) {
//	    // Resolve network address to friend ID
//	    return friendID, true
//	})
//
//	// Handle incoming file requests
//	manager.OnFileRequest(func(friendID, fileID uint32, fileName string, fileSize uint64) {
//	    fmt.Printf("File request: %s (%d bytes)\n", fileName, fileSize)
//	    manager.AcceptFile(friendID, fileID, savePath)
//	})
//
//	// Send a file
//	fileID, err := manager.SendFile(friendID, filePath)
//
// # Chunked Transfer
//
// Files are transferred in chunks for efficient streaming:
//
//	const ChunkSize = 1024      // Default chunk size
//	const MaxChunkSize = 65536  // Maximum allowed chunk size
//
//	// Write a received chunk
//	err := transfer.WriteChunk(offset, data)
//
//	// Read next chunk for sending
//	chunk, err := transfer.ReadChunk(offset, size)
//
// # Security
//
// The package includes security protections:
//
// Path Validation: Directory traversal attacks are prevented:
//
//	if err := file.ValidatePath(path); err != nil {
//	    // err == file.ErrDirectoryTraversal
//	}
//
// Chunk Size Limits: Oversized chunks are rejected:
//
//	// ErrChunkTooLarge returned for chunks > MaxChunkSize
//
// # Address Resolution
//
// The AddressResolver interface maps network addresses to friend IDs:
//
//	type AddressResolver interface {
//	    ResolveFriendID(addr net.Addr) (uint32, bool)
//	}
//
//	// Configure custom resolver
//	manager.SetAddressResolver(resolver)
//
// This enables proper routing of file packets to the correct friend context.
//
// # Deterministic Testing
//
// For reproducible test scenarios, use the TimeProvider interface:
//
//	type TimeProvider interface {
//	    Now() time.Time
//	    Since(t time.Time) time.Duration
//	}
//
//	// Inject mock time for testing
//	transfer.SetTimeProvider(&MockTimeProvider{fixedTime})
//
// # Progress Tracking
//
// Transfer progress is tracked with callbacks:
//
//	transfer.OnProgress(func(received uint64) {
//	    // Called after each chunk
//	})
//
//	transfer.OnComplete(func() {
//	    // Called when transfer finishes successfully
//	})
//
//	transfer.OnError(func(err error) {
//	    // Called on transfer failure
//	})
//
// Statistics are available:
//
//	stats := transfer.GetStats()
//	fmt.Printf("Transferred: %d/%d bytes\n", stats.Transferred, stats.FileSize)
//	fmt.Printf("Speed: %d bytes/sec\n", stats.Speed)
//
// # Packet Types
//
// File transfer uses dedicated packet types registered in transport layer:
//
//   - PacketFileRequest: Initiates file transfer negotiation
//   - PacketFileControl: Pause, resume, cancel commands
//   - PacketFileData: File chunk payload
//   - PacketFileDataAck: Chunk acknowledgment for flow control
//
// # Thread Safety
//
// Transfer methods use sync.RWMutex for concurrent access safety.
// Manager methods are thread-safe for handling multiple simultaneous transfers.
// Callbacks are invoked synchronously; long-running operations should be
// offloaded to separate goroutines.
//
// # Integration Status
//
// The file package is partially integrated:
//
//   - ✅ Packet types registered in transport layer
//   - ✅ Transport integration via Manager.NewManager(transport)
//   - ✅ AddressResolver for friend ID resolution
//   - ⚠️ Not yet integrated into main Tox struct (standalone usage)
//   - ⚠️ No persistence/serialization for active transfers
//
// # Example: Complete File Transfer
//
//	// Sender side
//	manager := file.NewManager(transport)
//	fileID, err := manager.SendFile(friendID, "/path/to/file.txt")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Receiver side
//	manager := file.NewManager(transport)
//	manager.OnFileRequest(func(friendID, fileID uint32, name string, size uint64) {
//	    manager.AcceptFile(friendID, fileID, "/downloads/"+name)
//	})
//
// # Error Handling
//
// The package provides sentinel errors for common failure modes:
//
//	var (
//	    ErrDirectoryTraversal  // Path contains directory traversal attempt
//	    ErrChunkTooLarge       // Chunk exceeds MaxChunkSize
//	)
//
// All errors are wrapped with context for debugging.
package file
