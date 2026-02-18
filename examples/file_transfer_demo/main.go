// Package main demonstrates the file transfer system with network integration.
//
// This example shows how to use the file.Manager to send and receive files
// over the Tox network using the transport layer.
package main

import (
	"net"
	"os"
	"path/filepath"

	"github.com/opd-ai/toxcore/file"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func main() {
	// Configure structured logging for demo output
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Create a temporary directory for our test files
	tmpDir, err := os.MkdirTemp("", "file_transfer_demo")
	if err != nil {
		log.WithError(err).Fatal("Failed to create temp dir")
	}
	defer os.RemoveAll(tmpDir)

	log.WithField("dir", tmpDir).Info("Working directory created")

	// Create a test file to transfer
	sourceFile := filepath.Join(tmpDir, "source.txt")
	sourceData := []byte("Hello from toxcore-go file transfer!\nThis is a demonstration of peer-to-peer file sharing.")
	if err := os.WriteFile(sourceFile, sourceData, 0o644); err != nil {
		log.WithError(err).Fatal("Failed to create source file")
	}

	log.WithFields(logrus.Fields{
		"file": sourceFile,
		"size": len(sourceData),
	}).Info("Created source file")

	// Create transport (for demo, we'll use UDP transport)
	// Use interface type transport.Transport for proper abstraction
	var udpTransport transport.Transport
	udpTransport, err = transport.NewUDPTransport(":0")
	if err != nil {
		log.WithError(err).Fatal("Failed to create transport")
	}
	defer udpTransport.Close()

	log.WithField("addr", udpTransport.LocalAddr().String()).Info("Transport listening")

	// Create file transfer manager
	manager := file.NewManager(udpTransport)
	log.Info("File transfer manager created")

	// Simulate initiating a file transfer to a friend
	// In a real application, you would get the friend's address from the DHT
	// Use net.Addr interface type to comply with interface-based networking guidelines
	friendAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
	if err != nil {
		log.WithError(err).Fatal("Failed to resolve friend address")
	}

	const (
		friendID = 1
		fileID   = 100
	)

	// Send file transfer request
	transfer, err := manager.SendFile(friendID, fileID, sourceFile, uint64(len(sourceData)), friendAddr)
	if err != nil {
		log.WithError(err).Fatal("Failed to send file")
	}

	log.WithFields(logrus.Fields{
		"friend_id": transfer.FriendID,
		"file_id":   transfer.FileID,
		"file":      transfer.FileName,
		"size":      transfer.FileSize,
		"direction": transfer.Direction,
		"state":     transfer.State,
	}).Info("File transfer initiated")

	// Set up progress callback
	transfer.OnProgress(func(bytes uint64) {
		progress := transfer.GetProgress()
		speed := transfer.GetSpeed()
		log.WithFields(logrus.Fields{
			"progress_pct": progress,
			"bytes":        bytes,
			"total":        transfer.FileSize,
			"speed_kbps":   speed / 1024.0,
		}).Debug("Transfer progress")
	})

	// Set up completion callback
	transfer.OnComplete(func(err error) {
		if err != nil {
			log.WithError(err).Error("Transfer failed")
		} else {
			log.Info("Transfer completed successfully")
		}
	})

	// Start the transfer
	if err := transfer.Start(); err != nil {
		log.WithError(err).Fatal("Failed to start transfer")
	}

	log.Info("Transfer started")

	// In a real application, you would send chunks in response to peer requests
	// For this demo, we'll just show how to send a single chunk
	log.Info("Sending first chunk...")
	if err := manager.SendChunk(friendID, fileID, friendAddr); err != nil {
		log.WithError(err).Warn("SendChunk error (expected in demo without peer)")
	}

	log.WithFields(logrus.Fields{
		"transferred": transfer.Transferred,
		"total":       transfer.FileSize,
		"progress":    transfer.GetProgress(),
	}).Info("First chunk sent")

	log.Info("=== File Transfer Demo Complete ===")
	log.Info("In a real application:")
	log.Info("- The transport would be connected to actual Tox peers")
	log.Info("- Chunks would be sent automatically based on peer requests")
	log.Info("- Progress callbacks would fire as data is transferred")
	log.Info("- The receiving peer would write chunks to their local file")
	log.Info("- Transfers can be paused, resumed, or cancelled")
}
