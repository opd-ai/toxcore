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
	configureLogging()
	tmpDir := createWorkingDirectory()
	defer os.RemoveAll(tmpDir)

	sourceFile, sourceData := createTestFile(tmpDir)
	udpTransport := createTransport()
	defer udpTransport.Close()

	manager := file.NewManager(udpTransport)
	log.Info("File transfer manager created")

	friendAddr := resolveFriendAddress()
	transfer, friendID, fileID := initiateFileTransfer(manager, sourceFile, sourceData, friendAddr)
	setupTransferCallbacks(transfer)
	startFileTransfer(transfer)
	sendFirstChunk(manager, friendID, fileID, friendAddr, transfer)
	displayUsageNotes()
}

func configureLogging() {
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func createWorkingDirectory() string {
	tmpDir, err := os.MkdirTemp("", "file_transfer_demo")
	if err != nil {
		log.WithError(err).Fatal("Failed to create temp dir")
	}
	log.WithField("dir", tmpDir).Info("Working directory created")
	return tmpDir
}

func createTestFile(tmpDir string) (string, []byte) {
	sourceFile := filepath.Join(tmpDir, "source.txt")
	sourceData := []byte("Hello from toxcore-go file transfer!\nThis is a demonstration of peer-to-peer file sharing.")
	if err := os.WriteFile(sourceFile, sourceData, 0o644); err != nil {
		log.WithError(err).Fatal("Failed to create source file")
	}
	log.WithFields(logrus.Fields{
		"file": sourceFile,
		"size": len(sourceData),
	}).Info("Created source file")
	return sourceFile, sourceData
}

func createTransport() transport.Transport {
	var udpTransport transport.Transport
	udpTransport, err := transport.NewUDPTransport(":0")
	if err != nil {
		log.WithError(err).Fatal("Failed to create transport")
	}
	log.WithField("addr", udpTransport.LocalAddr().String()).Info("Transport listening")
	return udpTransport
}

func resolveFriendAddress() net.Addr {
	friendAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
	if err != nil {
		log.WithError(err).Fatal("Failed to resolve friend address")
	}
	return friendAddr
}

func initiateFileTransfer(manager *file.Manager, sourceFile string, sourceData []byte, friendAddr net.Addr) (*file.Transfer, uint32, uint32) {
	const (
		friendID = 1
		fileID   = 100
	)
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
	return transfer, friendID, fileID
}

func setupTransferCallbacks(transfer *file.Transfer) {
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

	transfer.OnComplete(func(err error) {
		if err != nil {
			log.WithError(err).Error("Transfer failed")
		} else {
			log.Info("Transfer completed successfully")
		}
	})
}

func startFileTransfer(transfer *file.Transfer) {
	if err := transfer.Start(); err != nil {
		log.WithError(err).Fatal("Failed to start transfer")
	}
	log.Info("Transfer started")
}

func sendFirstChunk(manager *file.Manager, friendID, fileID uint32, friendAddr net.Addr, transfer *file.Transfer) {
	log.Info("Sending first chunk...")
	if err := manager.SendChunk(friendID, fileID, friendAddr); err != nil {
		log.WithError(err).Warn("SendChunk error (expected in demo without peer)")
	}
	log.WithFields(logrus.Fields{
		"transferred": transfer.Transferred,
		"total":       transfer.FileSize,
		"progress":    transfer.GetProgress(),
	}).Info("First chunk sent")
}

func displayUsageNotes() {
	log.Info("=== File Transfer Demo Complete ===")
	log.Info("In a real application:")
	log.Info("- The transport would be connected to actual Tox peers")
	log.Info("- Chunks would be sent automatically based on peer requests")
	log.Info("- Progress callbacks would fire as data is transferred")
	log.Info("- The receiving peer would write chunks to their local file")
	log.Info("- Transfers can be paused, resumed, or cancelled")
}
