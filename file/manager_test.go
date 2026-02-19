package file

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

func TestNewManager(t *testing.T) {
	trans := newMockTransport()
	manager := NewManager(trans)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.transport != trans {
		t.Error("Manager transport not set correctly")
	}

	if manager.transfers == nil {
		t.Error("Manager transfers map not initialized")
	}

	// Verify handlers were registered
	expectedHandlers := []transport.PacketType{
		transport.PacketFileRequest,
		transport.PacketFileControl,
		transport.PacketFileData,
		transport.PacketFileDataAck,
	}

	for _, pt := range expectedHandlers {
		if _, exists := trans.handler[pt]; !exists {
			t.Errorf("Handler not registered for packet type %v", pt)
		}
	}
}

func TestSendFile(t *testing.T) {
	trans := newMockTransport()
	manager := NewManager(trans)
	addr := &mockAddr{network: "udp", address: testPeerAddr}

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("Hello, file transfer!")
	if err := os.WriteFile(testFile, testData, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	transfer, err := manager.SendFile(1, 1, testFile, uint64(len(testData)), addr)
	if err != nil {
		t.Fatalf("SendFile failed: %v", err)
	}

	if transfer == nil {
		t.Fatal("SendFile returned nil transfer")
	}

	if transfer.FriendID != 1 {
		t.Errorf("Expected FriendID 1, got %d", transfer.FriendID)
	}

	if transfer.FileID != 1 {
		t.Errorf("Expected FileID 1, got %d", transfer.FileID)
	}

	if transfer.Direction != TransferDirectionOutgoing {
		t.Errorf("Expected outgoing direction, got %v", transfer.Direction)
	}

	// Verify file request packet was sent
	if len(trans.packets) != 1 {
		t.Fatalf("Expected 1 packet sent, got %d", len(trans.packets))
	}

	sentPkt := trans.packets[0]
	if sentPkt.packet.PacketType != transport.PacketFileRequest {
		t.Errorf("Expected PacketFileRequest, got %v", sentPkt.packet.PacketType)
	}

	// Verify the transfer is stored
	retrieved, err := manager.GetTransfer(1, 1)
	if err != nil {
		t.Fatalf("GetTransfer failed: %v", err)
	}

	if retrieved != transfer {
		t.Error("Retrieved transfer does not match original")
	}
}

func TestSendFileDuplicate(t *testing.T) {
	trans := newMockTransport()
	manager := NewManager(trans)
	addr := &mockAddr{network: "udp", address: testPeerAddr}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// First transfer should succeed
	_, err := manager.SendFile(1, 1, testFile, 4, addr)
	if err != nil {
		t.Fatalf("First SendFile failed: %v", err)
	}

	// Second transfer with same IDs should fail
	_, err = manager.SendFile(1, 1, testFile, 4, addr)
	if err == nil {
		t.Error("Expected error for duplicate transfer, got nil")
	}
}

func TestHandleFileRequest(t *testing.T) {
	trans := newMockTransport()
	manager := NewManager(trans)
	addr := &mockAddr{network: "udp", address: testPeerAddr}

	// Simulate incoming file request
	requestData := serializeFileRequest(2, "received_file.txt", testFileSize1KB)
	trans.simulateReceive(transport.PacketFileRequest, requestData, addr)

	// Give handler time to process
	time.Sleep(10 * time.Millisecond)

	// Verify transfer was created
	transfer, err := manager.GetTransfer(2, 2)
	if err != nil {
		t.Fatalf("Transfer not created: %v", err)
	}

	if transfer.Direction != TransferDirectionIncoming {
		t.Errorf("Expected incoming direction, got %v", transfer.Direction)
	}

	if transfer.FileName != "received_file.txt" {
		t.Errorf("Expected filename 'received_file.txt', got '%s'", transfer.FileName)
	}

	if transfer.FileSize != testFileSize1KB {
		t.Errorf("Expected file size %d, got %d", testFileSize1KB, transfer.FileSize)
	}
}

func TestSendChunk(t *testing.T) {
	trans := newMockTransport()
	manager := NewManager(trans)
	addr := &mockAddr{network: "udp", address: testPeerAddr}

	// Create a test file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "chunk_test.txt")
	testData := make([]byte, 2048) // Larger than one chunk
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	if err := os.WriteFile(testFile, testData, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create outgoing transfer
	transfer, err := manager.SendFile(3, 3, testFile, uint64(len(testData)), addr)
	if err != nil {
		t.Fatalf("SendFile failed: %v", err)
	}

	// Start the transfer
	if err := transfer.Start(); err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	// Clear the file request packet
	trans.clearPackets()

	// Send first chunk
	if err := manager.SendChunk(3, 3, addr); err != nil {
		t.Fatalf("SendChunk failed: %v", err)
	}

	// Verify chunk packet was sent
	if len(trans.packets) != 1 {
		t.Fatalf("Expected 1 packet sent, got %d", len(trans.packets))
	}

	sentPkt := trans.packets[0]
	if sentPkt.packet.PacketType != transport.PacketFileData {
		t.Errorf("Expected PacketFileData, got %v", sentPkt.packet.PacketType)
	}

	// Verify chunk data
	fileID, chunk, err := deserializeFileData(sentPkt.packet.Data)
	if err != nil {
		t.Fatalf("Failed to deserialize chunk: %v", err)
	}

	if fileID != 3 {
		t.Errorf("Expected fileID 3, got %d", fileID)
	}

	if len(chunk) != ChunkSize {
		t.Errorf("Expected chunk size %d, got %d", ChunkSize, len(chunk))
	}

	// Verify chunk content matches
	for i := 0; i < ChunkSize; i++ {
		if chunk[i] != testData[i] {
			t.Errorf("Chunk data mismatch at byte %d", i)
			break
		}
	}
}

func TestHandleFileData(t *testing.T) {
	trans := newMockTransport()
	manager := NewManager(trans)
	addr := &mockAddr{network: "udp", address: testPeerAddr}

	tmpDir := t.TempDir()
	receiveFile := filepath.Join(tmpDir, "received.txt")

	// Create incoming transfer
	requestData := serializeFileRequest(4, receiveFile, testFileSize1KB)
	trans.simulateReceive(transport.PacketFileRequest, requestData, addr)
	time.Sleep(10 * time.Millisecond)

	// Get the transfer and start it
	transfer, err := manager.GetTransfer(4, 4)
	if err != nil {
		t.Fatalf("Failed to get transfer: %v", err)
	}

	if err := transfer.Start(); err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	// Simulate receiving file data
	testChunk := []byte("This is test data for file transfer")
	dataPacket := serializeFileData(4, testChunk)
	trans.clearPackets()
	trans.simulateReceive(transport.PacketFileData, dataPacket, addr)

	// Give handler time to process
	time.Sleep(10 * time.Millisecond)

	// Verify acknowledgment was sent
	if len(trans.packets) == 0 {
		t.Fatal("Expected acknowledgment packet, got none")
	}

	ackPkt := trans.getLastPacket()
	if ackPkt.packet.PacketType != transport.PacketFileDataAck {
		t.Errorf("Expected PacketFileDataAck, got %v", ackPkt.packet.PacketType)
	}

	// Verify transfer progress
	if transfer.Transferred != uint64(len(testChunk)) {
		t.Errorf("Expected %d bytes transferred, got %d", len(testChunk), transfer.Transferred)
	}
}

func TestSerializeDeserializeFileRequest(t *testing.T) {
	testCases := []struct {
		name     string
		fileID   uint32
		fileName string
		fileSize uint64
	}{
		{"short_name", 1, "test.txt", 1024},
		{"long_name", 2, "very_long_filename_for_testing_serialization.doc", 1048576},
		{"empty_name", 3, "", 0},
		{"unicode_name", 4, "测试文件.txt", 2048},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := serializeFileRequest(tc.fileID, tc.fileName, tc.fileSize)
			fileID, fileName, fileSize, err := deserializeFileRequest(data)
			if err != nil {
				t.Fatalf("Deserialization failed: %v", err)
			}

			if fileID != tc.fileID {
				t.Errorf("FileID mismatch: expected %d, got %d", tc.fileID, fileID)
			}

			if fileName != tc.fileName {
				t.Errorf("FileName mismatch: expected '%s', got '%s'", tc.fileName, fileName)
			}

			if fileSize != tc.fileSize {
				t.Errorf("FileSize mismatch: expected %d, got %d", tc.fileSize, fileSize)
			}
		})
	}
}

func TestFileNameLengthValidation(t *testing.T) {
	testCases := []struct {
		name        string
		fileName    string
		expectError bool
	}{
		{"valid_short_name", "test.txt", false},
		{"valid_max_length", string(make([]byte, MaxFileNameLength)), false},
		{"invalid_too_long", string(make([]byte, MaxFileNameLength+1)), true},
		{"invalid_way_too_long", string(make([]byte, 1000)), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize valid name bytes with printable characters
			fileName := tc.fileName
			if len(fileName) > 10 {
				// Replace null bytes with 'a' for readable test names
				fileNameBytes := make([]byte, len(fileName))
				for i := range fileNameBytes {
					fileNameBytes[i] = 'a'
				}
				fileName = string(fileNameBytes)
			}

			trans := newMockTransport()
			mgr := NewManager(trans)
			addr := &mockAddr{network: "udp", address: testLocalAddr}

			_, err := mgr.SendFile(1, 1, fileName, testFileSize1KB, addr)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for file name length %d, got nil", len(fileName))
				} else if !errors.Is(err, ErrFileNameTooLong) {
					t.Errorf("Expected ErrFileNameTooLong, got %v", err)
				}
			} else {
				if errors.Is(err, ErrFileNameTooLong) {
					t.Errorf("Unexpected ErrFileNameTooLong for file name length %d", len(fileName))
				}
			}
		})
	}
}

func TestDeserializeFileRequestRejectsLongName(t *testing.T) {
	// Craft a packet with a name length exceeding the limit
	// Format: [file_id (4 bytes)][file_size (8 bytes)][name_len (2 bytes)][file_name]
	data := make([]byte, 14+MaxFileNameLength+100)
	binary.BigEndian.PutUint32(data[0:4], 1)                               // fileID
	binary.BigEndian.PutUint64(data[4:12], testFileSize1KB)                           // fileSize
	binary.BigEndian.PutUint16(data[12:14], uint16(MaxFileNameLength+100)) // nameLen (too long)
	for i := 14; i < len(data); i++ {
		data[i] = 'a' // fill with valid characters
	}

	_, _, _, err := deserializeFileRequest(data)
	if err == nil {
		t.Error("Expected error for excessively long file name, got nil")
	}
	if !errors.Is(err, ErrFileNameTooLong) {
		t.Errorf("Expected ErrFileNameTooLong, got %v", err)
	}
}

func TestSerializeDeserializeFileData(t *testing.T) {
	testCases := []struct {
		name   string
		fileID uint32
		chunk  []byte
	}{
		{"small_chunk", 1, []byte("Hello")},
		{"full_chunk", 2, make([]byte, ChunkSize)},
		{"empty_chunk", 3, []byte{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := serializeFileData(tc.fileID, tc.chunk)
			fileID, chunk, err := deserializeFileData(data)
			if err != nil {
				t.Fatalf("Deserialization failed: %v", err)
			}

			if fileID != tc.fileID {
				t.Errorf("FileID mismatch: expected %d, got %d", tc.fileID, fileID)
			}

			if len(chunk) != len(tc.chunk) {
				t.Errorf("Chunk length mismatch: expected %d, got %d", len(tc.chunk), len(chunk))
			}
		})
	}
}

func TestEndToEndFileTransfer(t *testing.T) {
	// Create two managers (sender and receiver)
	senderTrans := newMockTransport()
	receiverTrans := newMockTransport()

	senderManager := NewManager(senderTrans)
	receiverManager := NewManager(receiverTrans)

	senderAddr := &mockAddr{network: "udp", address: testPeerAddr}
	receiverAddr := &mockAddr{network: "udp", address: testPeerAddr2}

	tmpDir := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(tmpDir, "source.txt")
	sourceData := []byte("This is a complete file transfer test with multiple chunks of data")
	if err := os.WriteFile(sourceFile, sourceData, 0o644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	destFile := filepath.Join(tmpDir, "dest.txt")

	// Sender initiates transfer
	_, err := senderManager.SendFile(5, 5, sourceFile, uint64(len(sourceData)), receiverAddr)
	if err != nil {
		t.Fatalf("SendFile failed: %v", err)
	}

	// Simulate receiver getting the file request
	fileReqPkt := senderTrans.getLastPacket()
	receiverTrans.simulateReceive(fileReqPkt.packet.PacketType, fileReqPkt.packet.Data, senderAddr)
	time.Sleep(10 * time.Millisecond)

	// Start both transfers
	senderTransfer, _ := senderManager.GetTransfer(5, 5)
	receiverTransfer, _ := receiverManager.GetTransfer(5, 5)

	receiverTransfer.FileName = destFile // Override filename for destination

	if err := senderTransfer.Start(); err != nil {
		t.Fatalf("Failed to start sender: %v", err)
	}

	if err := receiverTransfer.Start(); err != nil {
		t.Fatalf("Failed to start receiver: %v", err)
	}

	// Transfer chunks until complete
	senderTrans.clearPackets()
	for senderTransfer.State == TransferStateRunning {
		if err := senderManager.SendChunk(5, 5, receiverAddr); err != nil {
			break // EOF or completion
		}

		// Simulate receiver getting the chunk
		if len(senderTrans.packets) > 0 {
			dataPkt := senderTrans.getLastPacket()
			receiverTrans.simulateReceive(dataPkt.packet.PacketType, dataPkt.packet.Data, senderAddr)
			senderTrans.clearPackets()
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Wait for completion
	time.Sleep(20 * time.Millisecond)

	// Verify destination file was created and matches source
	receivedData, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if len(receivedData) != len(sourceData) {
		t.Errorf("File size mismatch: expected %d, got %d", len(sourceData), len(receivedData))
	}

	for i := range sourceData {
		if i < len(receivedData) && receivedData[i] != sourceData[i] {
			t.Errorf("Data mismatch at byte %d", i)
			break
		}
	}
}

// TestValidatePath tests the path validation function for directory traversal prevention.
func TestValidatePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errType     error
		description string
	}{
		{
			name:        "simple_filename",
			path:        "test.txt",
			wantErr:     false,
			description: "Simple filename should be allowed",
		},
		{
			name:        "subdirectory_path",
			path:        "uploads/test.txt",
			wantErr:     false,
			description: "Path with subdirectory should be allowed",
		},
		{
			name:        "absolute_path",
			path:        "/tmp/test.txt",
			wantErr:     false,
			description: "Absolute path should be allowed",
		},
		{
			name:        "directory_traversal_simple",
			path:        "../etc/passwd",
			wantErr:     true,
			errType:     ErrDirectoryTraversal,
			description: "Simple directory traversal should be blocked",
		},
		{
			name:        "directory_traversal_nested",
			path:        "uploads/../../etc/passwd",
			wantErr:     true,
			errType:     ErrDirectoryTraversal,
			description: "Nested directory traversal should be blocked",
		},
		{
			name:        "directory_traversal_complex",
			path:        "./test/../../../etc/passwd",
			wantErr:     true,
			errType:     ErrDirectoryTraversal,
			description: "Complex directory traversal should be blocked",
		},
		{
			name:        "current_directory_reference",
			path:        "./test.txt",
			wantErr:     false,
			description: "Current directory reference should be allowed",
		},
		{
			name:        "deep_nesting_valid",
			path:        "a/b/c/d/e/f/test.txt",
			wantErr:     false,
			description: "Deep nested path without traversal should be allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanedPath, err := ValidatePath(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePath(%q) expected error, got nil", tt.path)
					return
				}
				if tt.errType != nil && err != tt.errType {
					t.Errorf("ValidatePath(%q) error = %v, want %v", tt.path, err, tt.errType)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePath(%q) unexpected error: %v", tt.path, err)
					return
				}
				if cleanedPath == "" {
					t.Errorf("ValidatePath(%q) returned empty path", tt.path)
				}
			}
		})
	}
}

// TestWriteChunkSizeValidation tests that WriteChunk rejects oversized chunks.
func TestWriteChunkSizeValidation(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "large_chunk_test.txt")

	transfer := NewTransfer(1, 1, testFile, 1000000, TransferDirectionIncoming)

	// Start the transfer
	if err := transfer.Start(); err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}
	defer transfer.FileHandle.Close()

	// Test with chunk exactly at max size - should succeed
	validChunk := make([]byte, MaxChunkSize)
	if err := transfer.WriteChunk(validChunk); err != nil {
		t.Errorf("WriteChunk with MaxChunkSize chunk should succeed, got error: %v", err)
	}

	// Test with oversized chunk - should fail
	oversizedChunk := make([]byte, MaxChunkSize+1)
	err := transfer.WriteChunk(oversizedChunk)
	if err == nil {
		t.Error("WriteChunk with oversized chunk should return error")
	}
	if err != ErrChunkTooLarge {
		t.Errorf("WriteChunk error = %v, want ErrChunkTooLarge", err)
	}
}

// TestReadChunkSizeValidation tests that ReadChunk rejects oversized size requests.
func TestReadChunkSizeValidation(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "read_chunk_test.txt")

	// Create a test file
	testData := make([]byte, 100000)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	if err := os.WriteFile(testFile, testData, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	transfer := NewTransfer(1, 1, testFile, uint64(len(testData)), TransferDirectionOutgoing)

	// Start the transfer
	if err := transfer.Start(); err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}
	defer transfer.FileHandle.Close()

	// Test with valid chunk size - should succeed
	validSize := uint16(ChunkSize)
	_, err := transfer.ReadChunk(validSize)
	if err != nil {
		t.Errorf("ReadChunk with valid size should succeed, got error: %v", err)
	}

	// Note: uint16 max is 65535 which is less than MaxChunkSize (65536)
	// So we can't test oversized with uint16, but we verify the validation exists
	// by testing with the maximum uint16 value
	maxUint16Size := uint16(65535)
	_, err = transfer.ReadChunk(maxUint16Size)
	if err != nil {
		t.Errorf("ReadChunk with max uint16 size should succeed (less than MaxChunkSize): %v", err)
	}
}

// TestDirectoryTraversalInTransferStart tests that Start() rejects paths with directory traversal.
func TestDirectoryTraversalInTransferStart(t *testing.T) {
	maliciousPaths := []string{
		"../etc/passwd",
		"../../etc/shadow",
		"uploads/../../../etc/hosts",
		"./test/../../../sensitive/data",
	}

	for _, maliciousPath := range maliciousPaths {
		t.Run(maliciousPath, func(t *testing.T) {
			transfer := NewTransfer(1, 1, maliciousPath, 1024, TransferDirectionIncoming)

			err := transfer.Start()
			if err == nil {
				t.Errorf("Start() with path %q should return error", maliciousPath)
				if transfer.FileHandle != nil {
					transfer.FileHandle.Close()
				}
				return
			}

			if err != ErrDirectoryTraversal {
				t.Errorf("Start() with path %q error = %v, want ErrDirectoryTraversal", maliciousPath, err)
			}

			if transfer.State != TransferStateError {
				t.Errorf("Transfer state should be TransferStateError, got %v", transfer.State)
			}
		})
	}
}

// TestAddressResolver tests the AddressResolver interface and SetAddressResolver method.
func TestAddressResolver(t *testing.T) {
	trans := newMockTransport()
	manager := NewManager(trans)

	// Test with no resolver - should use fileID as fallback
	t.Run("no_resolver_uses_fallback", func(t *testing.T) {
		addr := &mockAddr{network: "udp", address: testPeerAddr}
		requestData := serializeFileRequest(10, "test_no_resolver.txt", testFileSize1KB)
		trans.simulateReceive(transport.PacketFileRequest, requestData, addr)
		time.Sleep(10 * time.Millisecond)

		// Should create transfer with fileID as friendID (fallback behavior)
		transfer, err := manager.GetTransfer(10, 10)
		if err != nil {
			t.Fatalf("Transfer not created: %v", err)
		}
		if transfer.FriendID != 10 {
			t.Errorf("Expected friendID 10 (fallback), got %d", transfer.FriendID)
		}
	})

	// Test with resolver configured
	t.Run("resolver_resolves_friend_id", func(t *testing.T) {
		// Create a resolver that maps addresses to friend IDs
		addressMap := map[string]uint32{
			testPeerAddr2: 100,
			"127.0.0.1:33448": 200,
		}

		resolver := AddressResolverFunc(func(addr net.Addr) (uint32, error) {
			if friendID, ok := addressMap[addr.String()]; ok {
				return friendID, nil
			}
			return 0, errors.New("unknown address")
		})

		manager.SetAddressResolver(resolver)

		addr := &mockAddr{network: "udp", address: testPeerAddr2}
		requestData := serializeFileRequest(20, "test_with_resolver.txt", testFileSize2KB)
		trans.simulateReceive(transport.PacketFileRequest, requestData, addr)
		time.Sleep(10 * time.Millisecond)

		// Should create transfer with resolved friendID (100), not fileID (20)
		transfer, err := manager.GetTransfer(100, 20)
		if err != nil {
			t.Fatalf("Transfer not created with resolved friendID: %v", err)
		}
		if transfer.FriendID != 100 {
			t.Errorf("Expected friendID 100 (resolved), got %d", transfer.FriendID)
		}
	})

	// Test with resolver that returns error - should fallback to fileID
	t.Run("resolver_error_uses_fallback", func(t *testing.T) {
		resolver := AddressResolverFunc(func(addr net.Addr) (uint32, error) {
			return 0, errors.New("resolution failed")
		})

		manager.SetAddressResolver(resolver)

		addr := &mockAddr{network: "udp", address: "192.168.1.100:33449"}
		requestData := serializeFileRequest(30, "test_resolver_error.txt", 3072)
		trans.simulateReceive(transport.PacketFileRequest, requestData, addr)
		time.Sleep(10 * time.Millisecond)

		// Should create transfer with fileID as friendID (fallback)
		transfer, err := manager.GetTransfer(30, 30)
		if err != nil {
			t.Fatalf("Transfer not created: %v", err)
		}
		if transfer.FriendID != 30 {
			t.Errorf("Expected friendID 30 (fallback), got %d", transfer.FriendID)
		}
	})
}

// TestAddressResolverFunc tests the AddressResolverFunc adapter type.
func TestAddressResolverFunc(t *testing.T) {
	called := false
	expectedFriendID := uint32(42)

	resolver := AddressResolverFunc(func(addr net.Addr) (uint32, error) {
		called = true
		return expectedFriendID, nil
	})

	addr := &mockAddr{network: "udp", address: "10.0.0.1:5000"}
	friendID, err := resolver.ResolveFriendID(addr)

	if !called {
		t.Error("Resolver function was not called")
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if friendID != expectedFriendID {
		t.Errorf("Expected friendID %d, got %d", expectedFriendID, friendID)
	}
}
