package toxcore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opd-ai/toxcore/file"
)

func TestFileTransferIntegration(t *testing.T) {
	// Create two Tox instances for testing file transfer
	options1 := NewOptions()
	options1.UDPEnabled = false // Disable UDP for testing
	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	options2 := NewOptions()
	options2.UDPEnabled = false // Disable UDP for testing
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Test FileSend method
	t.Run("FileSend", func(t *testing.T) {
		// Create a test file
		testDir := t.TempDir()
		testFile := filepath.Join(testDir, "test.txt")
		testContent := "Hello, this is a test file for Tox file transfer!"

		err := os.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Add friend relationship (mock)
		friendID := uint32(1)
		tox1.friends[friendID] = &Friend{
			PublicKey:        tox2.keyPair.Public,
			Status:           FriendStatusOnline,
			ConnectionStatus: ConnectionUDP,
		}

		// Send file
		fileID := [32]byte{}
		copy(fileID[:], "test-file-id-12345678901234567890")

		transferID, err := tox1.FileSend(friendID, 0, uint64(len(testContent)), fileID, testFile)
		if err != nil {
			t.Errorf("FileSend failed: %v", err)
		}

		if transferID == 0 {
			t.Error("Expected non-zero transfer ID")
		}

		// Verify transfer was created
		transfer, err := tox1.GetFileTransfer(transferID)
		if err != nil {
			t.Errorf("Failed to get file transfer: %v", err)
			return // Skip further checks if transfer doesn't exist
		}

		if transfer.FriendID != friendID {
			t.Errorf("Expected friend ID %d, got %d", friendID, transfer.FriendID)
		}

		if transfer.Direction != file.TransferDirectionOutgoing {
			t.Errorf("Expected outgoing transfer, got %v", transfer.Direction)
		}
	})

	// Test FileControl method
	t.Run("FileControl", func(t *testing.T) {
		friendID := uint32(2)
		transferID := uint32(10)

		// Create a mock transfer
		transfer := file.NewTransfer(friendID, transferID, "test.txt", 1024, file.TransferDirectionOutgoing)
		transfer.State = file.TransferStateRunning

		tox1.fileTransferMutex.Lock()
		tox1.fileTransfers[transferID] = transfer
		tox1.fileTransferMutex.Unlock()

		// Test pause
		err := tox1.FileControl(friendID, transferID, FileControlPause)
		if err != nil {
			t.Errorf("FileControl pause failed: %v", err)
		}

		if transfer.State != file.TransferStatePaused {
			t.Errorf("Expected transfer to be paused, got state %v", transfer.State)
		}

		// Test resume
		err = tox1.FileControl(friendID, transferID, FileControlResume)
		if err != nil {
			t.Errorf("FileControl resume failed: %v", err)
		}

		if transfer.State != file.TransferStateRunning {
			t.Errorf("Expected transfer to be running, got state %v", transfer.State)
		}

		// Test cancel
		err = tox1.FileControl(friendID, transferID, FileControlCancel)
		if err != nil {
			t.Errorf("FileControl cancel failed: %v", err)
		}

		if transfer.State != file.TransferStateCancelled {
			t.Errorf("Expected transfer to be cancelled, got state %v", transfer.State)
		}

		// Verify transfer was removed from active transfers
		_, err = tox1.GetFileTransfer(transferID)
		if err == nil {
			t.Error("Expected transfer to be removed after cancellation")
		}
	})

	// Test AcceptFileTransfer method
	t.Run("AcceptFileTransfer", func(t *testing.T) {
		friendID := uint32(3)
		fileID := uint32(20)

		testDir := t.TempDir()
		filename := filepath.Join(testDir, "received.txt")

		err := tox2.AcceptFileTransfer(friendID, fileID, filename)
		if err != nil {
			t.Errorf("AcceptFileTransfer failed: %v", err)
		}

		// Verify transfer was created
		transfer, err := tox2.GetFileTransfer(fileID)
		if err != nil {
			t.Errorf("Failed to get accepted file transfer: %v", err)
		}

		if transfer.Direction != file.TransferDirectionIncoming {
			t.Errorf("Expected incoming transfer, got %v", transfer.Direction)
		}

		if transfer.State != file.TransferStateRunning {
			t.Errorf("Expected running transfer, got state %v", transfer.State)
		}
	})

	// Test GetActiveFileTransfers method
	t.Run("GetActiveFileTransfers", func(t *testing.T) {
		// Clear existing transfers
		tox1.fileTransferMutex.Lock()
		tox1.fileTransfers = make(map[uint32]*file.Transfer)
		tox1.fileTransferMutex.Unlock()

		// Add some test transfers
		transfer1 := file.NewTransfer(1, 100, "file1.txt", 1024, file.TransferDirectionOutgoing)
		transfer2 := file.NewTransfer(2, 200, "file2.txt", 2048, file.TransferDirectionIncoming)

		tox1.fileTransferMutex.Lock()
		tox1.fileTransfers[100] = transfer1
		tox1.fileTransfers[200] = transfer2
		tox1.fileTransferMutex.Unlock()

		activeTransfers := tox1.GetActiveFileTransfers()

		if len(activeTransfers) != 2 {
			t.Errorf("Expected 2 active transfers, got %d", len(activeTransfers))
		}

		if _, exists := activeTransfers[100]; !exists {
			t.Error("Transfer 100 not found in active transfers")
		}

		if _, exists := activeTransfers[200]; !exists {
			t.Error("Transfer 200 not found in active transfers")
		}
	})

	// Test file transfer callbacks
	t.Run("FileTransferCallbacks", func(t *testing.T) {
		var fileRecvCalled bool
		var fileChunkRecvCalled bool
		var chunkRequestCalled bool

		// Set up callbacks
		tox2.OnFileRecv(func(friendID uint32, fileID uint32, kind uint32, fileSize uint64, filename string) {
			fileRecvCalled = true
			t.Logf("File receive callback: friend=%d, file=%d, kind=%d, size=%d, name=%s",
				friendID, fileID, kind, fileSize, filename)
		})

		tox2.OnFileRecvChunk(func(friendID uint32, fileID uint32, position uint64, data []byte) {
			fileChunkRecvCalled = true
			t.Logf("File chunk receive callback: friend=%d, file=%d, pos=%d, size=%d",
				friendID, fileID, position, len(data))
		})

		tox2.OnFileChunkRequest(func(friendID uint32, fileID uint32, position uint64, length int) {
			chunkRequestCalled = true
			t.Logf("File chunk request callback: friend=%d, file=%d, pos=%d, len=%d",
				friendID, fileID, position, length)
		})

		// Simulate callback triggers
		if tox2.fileRecvCallback != nil {
			tox2.fileRecvCallback(1, 1, 0, 1024, "test.txt")
		}

		if tox2.fileRecvChunkCallback != nil {
			tox2.fileRecvChunkCallback(1, 1, 0, []byte("test data"))
		}

		if tox2.fileChunkRequestCallback != nil {
			tox2.fileChunkRequestCallback(1, 1, 512, 256)
		}

		// Verify callbacks were called
		if !fileRecvCalled {
			t.Error("File receive callback was not called")
		}

		if !fileChunkRecvCalled {
			t.Error("File chunk receive callback was not called")
		}

		if !chunkRequestCalled {
			t.Error("File chunk request callback was not called")
		}
	})

	// Test file transfer processing in iteration loop
	t.Run("FileTransferProcessing", func(t *testing.T) {
		// Create a running transfer
		transfer := file.NewTransfer(1, 300, "test.txt", 1024, file.TransferDirectionOutgoing)
		transfer.State = file.TransferStateRunning

		tox1.fileTransferMutex.Lock()
		tox1.fileTransfers[300] = transfer
		tox1.fileTransferMutex.Unlock()

		// Call iterate to process transfers
		tox1.Iterate()

		// Verify the transfer is still active (should not fail)
		_, err := tox1.GetFileTransfer(300)
		if err != nil {
			t.Errorf("Transfer should still be active after iteration: %v", err)
		}
	})
}

func TestFileTransferPacketHandling(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test handleFileOfferPacket
	t.Run("HandleFileOffer", func(t *testing.T) {
		// Mock friend
		friendID := uint32(1)
		friendPublicKey := [32]byte{}
		copy(friendPublicKey[:], "test-public-key-1234567890123456")

		tox.friends[friendID] = &Friend{
			PublicKey: friendPublicKey,
		}

		// Create a file offer packet
		filename := "test.txt"
		fileSize := uint64(1024)
		packet, err := tox.createFileOfferPacket(friendID, 1, 0, fileSize, filename)
		if err != nil {
			t.Fatalf("Failed to create file offer packet: %v", err)
		}

		// Set up callback to capture the offer
		var callbackCalled bool
		var receivedFriendID uint32
		var receivedSize uint64
		var receivedFilename string

		tox.OnFileRecv(func(fID uint32, fileID uint32, kind uint32, size uint64, fname string) {
			callbackCalled = true
			receivedFriendID = fID
			receivedSize = size
			receivedFilename = fname
		})

		// Handle the packet
		err = tox.handleFileOfferPacket(packet, nil)
		if err != nil {
			t.Errorf("Failed to handle file offer packet: %v", err)
		}

		// Verify callback was called with correct data
		if !callbackCalled {
			t.Error("File receive callback was not called")
		}

		if receivedFriendID != friendID {
			t.Errorf("Expected friend ID %d, got %d", friendID, receivedFriendID)
		}

		if receivedFilename != filename {
			t.Errorf("Expected filename %s, got %s", filename, receivedFilename)
		}

		if receivedSize != fileSize {
			t.Errorf("Expected file size %d, got %d", fileSize, receivedSize)
		}
	})

	// Test handleFileChunkPacket
	t.Run("HandleFileChunk", func(t *testing.T) {
		// Create an incoming transfer
		friendID := uint32(2)
		fileID := uint32(10)
		transfer := file.NewTransfer(friendID, fileID, "received.txt", 1024, file.TransferDirectionIncoming)

		// Create temp file for writing
		err := transfer.Start() // This would open the file
		if err != nil {
			t.Fatalf("Failed to start transfer: %v", err)
		}

		tox.fileTransferMutex.Lock()
		tox.fileTransfers[fileID] = transfer
		tox.fileTransferMutex.Unlock()

		// Mock friend
		friendPublicKey := [32]byte{}
		copy(friendPublicKey[:], "test-public-key-2345678901234567")
		tox.friends[friendID] = &Friend{
			PublicKey: friendPublicKey,
		}

		// Create a file chunk packet
		chunkData := []byte("Hello, chunk data!")
		position := uint64(0)
		packet, err := tox.createFileChunkPacket(friendID, fileID, position, chunkData)
		if err != nil {
			t.Fatalf("Failed to create file chunk packet: %v", err)
		}

		// Set up callback to capture the chunk
		var callbackCalled bool
		var receivedData []byte

		tox.OnFileRecvChunk(func(fID uint32, fFileID uint32, pos uint64, data []byte) {
			callbackCalled = true
			receivedData = make([]byte, len(data))
			copy(receivedData, data)
		})

		// Handle the packet
		err = tox.handleFileChunkPacket(packet, nil)
		if err != nil {
			t.Errorf("Failed to handle file chunk packet: %v", err)
		}

		// Verify callback was called with correct data
		if !callbackCalled {
			t.Error("File chunk receive callback was not called")
		}

		if string(receivedData) != string(chunkData) {
			t.Errorf("Expected chunk data %s, got %s", string(chunkData), string(receivedData))
		}
	})
}
