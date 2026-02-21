# File Transfer Network Integration

This document describes the file transfer network integration implementation added to toxcore-go.

## Overview

The file transfer system now includes full network transport integration, enabling peer-to-peer file sharing over the Tox network. The implementation consists of:

- **Manager**: Coordinates file transfers with the network transport layer
- **Packet Handlers**: Process incoming file transfer protocol messages
- **Serialization**: Converts file transfer data to/from network packets
- **Integration**: Seamlessly works with existing Transport interface

## Architecture

```
┌─────────────┐
│   Manager   │ ← Coordinates transfers with transport
└──────┬──────┘
       │
       ├─→ Transport Interface (UDP/TCP/Noise)
       │
       ├─→ Transfer State Management
       │
       └─→ Packet Handlers:
           ├─ FileRequest    (initiate transfer)
           ├─ FileControl    (pause/resume/cancel)
           ├─ FileData       (chunk transmission)
           └─ FileDataAck    (acknowledgment)
```

## File Transfer Protocol

### Packet Types

The implementation uses four packet types defined in `transport/packet.go`:

1. **PacketFileRequest** (18) - Initiate a file transfer
2. **PacketFileControl** (19) - Control operations (pause, resume, cancel)
3. **PacketFileData** (20) - Transmit file chunks
4. **PacketFileDataAck** (21) - Acknowledge received chunks

### Packet Formats

#### FileRequest Packet
```
[file_id (4 bytes)][file_size (8 bytes)][name_len (2 bytes)][file_name (variable)]
```

#### FileControl Packet
```
[file_id (4 bytes)][control_type (1 byte)]
  control_type: 1=Pause, 2=Resume, 3=Cancel
```

#### FileData Packet
```
[file_id (4 bytes)][chunk_data (variable, max 1024 bytes)]
```

#### FileDataAck Packet
```
[file_id (4 bytes)][bytes_received (8 bytes)]
```

## Usage

### Basic Usage

```go
import (
    "github.com/opd-ai/toxcore/file"
    "github.com/opd-ai/toxcore/transport"
)

// Create transport
udpTransport, err := transport.NewUDPTransport(":33445")
if err != nil {
    log.Fatal(err)
}
defer udpTransport.Close()

// Create file transfer manager
manager := file.NewManager(udpTransport)

// Send a file to a friend
transfer, err := manager.SendFile(
    friendID,      // Friend ID
    fileID,        // Unique file transfer ID
    "/path/to/file.txt",
    fileSize,
    friendAddr,    // Friend's network address
)
if err != nil {
    log.Fatal(err)
}

// Set up progress tracking
transfer.OnProgress(func(bytes uint64) {
    fmt.Printf("Progress: %.1f%%\n", transfer.GetProgress())
})

// Start the transfer
transfer.Start()

// Send chunks (typically in response to peer requests)
for transfer.State == file.TransferStateRunning {
    manager.SendChunk(friendID, fileID, friendAddr)
}
```

### Receiving Files

File reception is handled automatically by the Manager's packet handlers:

```go
// When a FileRequest packet is received:
// 1. Manager creates an incoming Transfer
// 2. Application can retrieve it via GetTransfer()
// 3. Start the transfer to begin writing to disk
// 4. FileData packets are automatically written to the file
// 5. Acknowledgments are sent automatically

transfer, _ := manager.GetTransfer(friendID, fileID)
transfer.FileName = "/path/to/save/file.txt" // Override destination
transfer.Start()

// Progress callbacks work the same way
transfer.OnProgress(func(bytes uint64) {
    fmt.Printf("Received: %d/%d bytes\n", bytes, transfer.FileSize)
})
```

### Transfer Control

```go
// Pause a transfer
transfer.Pause()

// Resume a paused transfer
transfer.Resume()

// Cancel a transfer
transfer.Cancel()

// Get transfer statistics
progress := transfer.GetProgress()        // Percentage complete
speed := transfer.GetSpeed()             // Bytes per second
timeLeft := transfer.GetEstimatedTimeRemaining()
```

## Implementation Details

### Manager Structure

```go
type Manager struct {
    transport           transport.Transport
    transfers           map[transferKey]*Transfer
    addressResolver     AddressResolver
    friendAddressLookup FriendAddressLookup
    mu                  sync.RWMutex
}
```

The Manager:
- Maintains a map of active transfers indexed by (friendID, fileID)
- Registers packet handlers with the transport layer on creation
- Provides thread-safe access to transfers via RWMutex
- Automatically handles incoming protocol messages

### Transfer Key

```go
type transferKey struct {
    friendID uint32
    fileID   uint32
}
```

Transfers are uniquely identified by the combination of friend ID and file ID, allowing multiple concurrent transfers with the same friend.

### Handler Registration

The Manager registers handlers for all file transfer packet types during initialization:

```go
t.RegisterHandler(transport.PacketFileRequest, m.handleFileRequest)
t.RegisterHandler(transport.PacketFileControl, m.handleFileControl)
t.RegisterHandler(transport.PacketFileData, m.handleFileData)
t.RegisterHandler(transport.PacketFileDataAck, m.handleFileDataAck)
```

### Chunk Management

- Default chunk size: 1024 bytes (defined in `file.ChunkSize`)
- Chunks are read from the file using `Transfer.ReadChunk()`
- Outgoing chunks are sent via `Manager.SendChunk()`
- Incoming chunks are written via `Transfer.WriteChunk()`
- Progress tracking and speed calculation are automatic

## Testing

The implementation includes comprehensive tests:

- **TestNewManager**: Verifies manager initialization and handler registration
- **TestSendFile**: Tests outgoing transfer initiation
- **TestSendFileDuplicate**: Ensures duplicate transfers are rejected
- **TestHandleFileRequest**: Validates incoming transfer creation
- **TestSendChunk**: Tests chunk transmission
- **TestHandleFileData**: Validates chunk reception and acknowledgment
- **TestSerializeDeserializeFileRequest**: Tests request packet format
- **TestSerializeDeserializeFileData**: Tests data packet format
- **TestEndToEndFileTransfer**: Full transfer simulation

All tests pass with 57.7% code coverage.

## Example

See `examples/file_transfer_demo/main.go` for a complete working example.

## Thread Safety

All Manager operations are thread-safe:
- Transfer map access is protected by RWMutex
- Individual Transfer state is protected by its own mutex
- Packet handlers can be called concurrently from the transport layer

## Error Handling

The implementation follows Go best practices for error handling:
- All errors are wrapped with context using `fmt.Errorf`
- Network errors are propagated to the caller
- File I/O errors trigger transfer state changes (TransferStateError)
- Invalid packets are logged but don't crash the Manager

## Future Enhancements

Potential improvements for future versions:

1. **Encryption**: Add encryption layer for file data packets
2. **Compression**: Optional compression for certain file types
3. **Resume Support**: Save transfer state to disk for resumption after restart
4. **Bandwidth Limiting**: Configurable upload/download speed limits
5. **Chunked Hashing**: Verify integrity of individual chunks
6. **Priority Queue**: Prioritize certain transfers over others

## Integration with toxcore.go

To integrate file transfers into the main Tox instance:

```go
type Tox struct {
    // ... existing fields ...
    fileManager *file.Manager
}

func (t *Tox) SendFile(friendID uint32, path string) (*file.Transfer, error) {
    // Get friend's address from DHT
    addr := t.dht.GetFriendAddress(friendID)
    
    // Generate unique file ID
    fileID := t.generateFileID()
    
    // Get file size
    info, _ := os.Stat(path)
    
    // Initiate transfer
    return t.fileManager.SendFile(friendID, fileID, path, uint64(info.Size()), addr)
}
```

## Compliance

The implementation follows the established patterns in toxcore-go:

- Uses interface types for network operations (net.Addr, not concrete types)
- Follows Go naming conventions and style guidelines
- Integrates with the existing Transport interface
- Maintains compatibility with the Tox protocol specification
- Includes comprehensive logging using logrus
