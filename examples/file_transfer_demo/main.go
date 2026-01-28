// Package main demonstrates the file transfer system with network integration.
//
// This example shows how to use the file.Manager to send and receive files
// over the Tox network using the transport layer.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/opd-ai/toxcore/file"
	"github.com/opd-ai/toxcore/transport"
)

func main() {
	// Create a temporary directory for our test files
	tmpDir, err := os.MkdirTemp("", "file_transfer_demo")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Working directory: %s\n\n", tmpDir)

	// Create a test file to transfer
	sourceFile := filepath.Join(tmpDir, "source.txt")
	sourceData := []byte("Hello from toxcore-go file transfer!\nThis is a demonstration of peer-to-peer file sharing.")
	if err := os.WriteFile(sourceFile, sourceData, 0644); err != nil {
		log.Fatalf("Failed to create source file: %v", err)
	}

	fmt.Printf("Created source file: %s (%d bytes)\n", sourceFile, len(sourceData))

	// Create transport (for demo, we'll use UDP transport)
	udpTransport, err := transport.NewUDPTransport(":0")
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}
	defer udpTransport.Close()

	fmt.Printf("Transport listening on: %s\n\n", udpTransport.LocalAddr())

	// Create file transfer manager
	manager := file.NewManager(udpTransport)
	fmt.Println("File transfer manager created")

	// Simulate initiating a file transfer to a friend
	// In a real application, you would get the friend's address from the DHT
	friendAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 33445,
	}

	const (
		friendID = 1
		fileID   = 100
	)

	// Send file transfer request
	transfer, err := manager.SendFile(friendID, fileID, sourceFile, uint64(len(sourceData)), friendAddr)
	if err != nil {
		log.Fatalf("Failed to send file: %v", err)
	}

	fmt.Printf("\nFile transfer initiated:\n")
	fmt.Printf("  Friend ID: %d\n", transfer.FriendID)
	fmt.Printf("  File ID:   %d\n", transfer.FileID)
	fmt.Printf("  File:      %s\n", transfer.FileName)
	fmt.Printf("  Size:      %d bytes\n", transfer.FileSize)
	fmt.Printf("  Direction: %v\n", transfer.Direction)
	fmt.Printf("  State:     %v\n", transfer.State)

	// Set up progress callback
	transfer.OnProgress(func(bytes uint64) {
		progress := transfer.GetProgress()
		speed := transfer.GetSpeed()
		fmt.Printf("Progress: %.1f%% (%d/%d bytes) - Speed: %.2f KB/s\n",
			progress, bytes, transfer.FileSize, speed/1024.0)
	})

	// Set up completion callback
	transfer.OnComplete(func(err error) {
		if err != nil {
			fmt.Printf("Transfer failed: %v\n", err)
		} else {
			fmt.Printf("Transfer completed successfully!\n")
		}
	})

	// Start the transfer
	if err := transfer.Start(); err != nil {
		log.Fatalf("Failed to start transfer: %v", err)
	}

	fmt.Println("\nTransfer started")

	// In a real application, you would send chunks in response to peer requests
	// For this demo, we'll just show how to send a single chunk
	fmt.Println("\nSending first chunk...")
	if err := manager.SendChunk(friendID, fileID, friendAddr); err != nil {
		log.Printf("SendChunk error: %v", err)
	}

	fmt.Printf("\nFirst chunk sent!\n")
	fmt.Printf("Bytes transferred: %d/%d\n", transfer.Transferred, transfer.FileSize)
	fmt.Printf("Progress: %.1f%%\n", transfer.GetProgress())

	fmt.Println("\n=== File Transfer Demo Complete ===")
	fmt.Println("\nIn a real application:")
	fmt.Println("- The transport would be connected to actual Tox peers")
	fmt.Println("- Chunks would be sent automatically based on peer requests")
	fmt.Println("- Progress callbacks would fire as data is transferred")
	fmt.Println("- The receiving peer would write chunks to their local file")
	fmt.Println("- Transfers can be paused, resumed, or cancelled")
}
