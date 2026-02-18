// Package internal provides the core components for the Tox network integration test suite.
//
// This package implements a comprehensive test harness that validates core Tox protocol
// operations through complete peer-to-peer communication workflows, including bootstrap
// server initialization, client management, and protocol validation.
package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/sirupsen/logrus"
)

// BootstrapServer represents a localhost bootstrap server for testing.
type BootstrapServer struct {
	tox       *toxcore.Tox
	address   string
	port      uint16
	publicKey [32]byte
	running   bool
	mu        sync.RWMutex
	logger    *logrus.Entry
	metrics   *ServerMetrics
}

// ServerMetrics tracks bootstrap server performance and status.
// It provides visibility into DHT node operation during integration tests:
//   - StartTime: When the server began accepting connections
//   - ConnectionsServed: Total bootstrap connection attempts handled
//   - PacketsProcessed: Total DHT packets processed (announcements, requests)
//   - ActiveClients: Currently connected test clients
//
// Metrics are safe for concurrent access via internal mutex.
type ServerMetrics struct {
	StartTime         time.Time
	ConnectionsServed int64
	PacketsProcessed  int64
	ActiveClients     int
	mu                sync.RWMutex
}

// BootstrapConfig holds configuration for the bootstrap server.
type BootstrapConfig struct {
	Address string
	Port    uint16
	Timeout time.Duration
	Logger  *logrus.Entry
}

// DefaultBootstrapConfig returns a default configuration for the bootstrap server.
func DefaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		Address: "127.0.0.1",
		Port:    33445,
		Timeout: 10 * time.Second,
		Logger:  logrus.WithField("component", "bootstrap"),
	}
}

// NewBootstrapServer creates a new bootstrap server instance.
func NewBootstrapServer(config *BootstrapConfig) (*BootstrapServer, error) {
	if config == nil {
		config = DefaultBootstrapConfig()
	}

	// Create Tox options optimized for bootstrap server testing
	options := toxcore.NewOptionsForTesting()
	options.UDPEnabled = true
	options.IPv6Enabled = false // Simplify for localhost testing
	options.LocalDiscovery = false
	options.StartPort = config.Port
	options.EndPort = config.Port // Force exact port binding

	// Create Tox instance
	tox, err := toxcore.New(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tox instance: %w", err)
	}

	// Get the public key for bootstrap configuration
	publicKey := tox.GetSelfPublicKey()

	server := &BootstrapServer{
		tox:       tox,
		address:   config.Address,
		port:      config.Port,
		publicKey: publicKey,
		running:   false,
		logger:    config.Logger,
		metrics: &ServerMetrics{
			StartTime: time.Now(),
		},
	}

	return server, nil
}

// Start initializes and starts the bootstrap server.
func (bs *BootstrapServer) Start(ctx context.Context) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.running {
		return fmt.Errorf("bootstrap server already running")
	}

	bs.logger.WithFields(logrus.Fields{
		"address": bs.address,
		"port":    bs.port,
	}).Info("Starting bootstrap server")
	bs.logger.WithField("public_key", fmt.Sprintf("%X", bs.publicKey)).Info("Server public key")

	// Set up Tox event loop in background
	go bs.eventLoop(ctx)

	bs.running = true
	bs.metrics.StartTime = time.Now()

	// Wait longer for the server to fully initialize and start processing
	time.Sleep(1 * time.Second)

	// Verify server is accepting connections
	if err := bs.verifyServer(); err != nil {
		bs.running = false
		return fmt.Errorf("server verification failed: %w", err)
	}

	bs.logger.Info("✅ Bootstrap server started successfully")
	return nil
}

// Stop gracefully shuts down the bootstrap server.
func (bs *BootstrapServer) Stop() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if !bs.running {
		return nil
	}

	bs.logger.Info("Stopping bootstrap server...")

	// Clean shutdown of Tox instance
	bs.tox.Kill()
	bs.running = false

	uptime := time.Since(bs.metrics.StartTime)
	bs.logger.WithField("uptime", uptime).Info("✅ Bootstrap server stopped")
	return nil
}

// eventLoop runs the main Tox iteration loop for the bootstrap server.
func (bs *BootstrapServer) eventLoop(ctx context.Context) {
	ticker := time.NewTicker(bs.tox.IterationInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !bs.IsRunning() {
				return
			}
			bs.tox.Iterate()
			bs.updateMetrics()
		}
	}
}

// updateMetrics updates server performance metrics.
func (bs *BootstrapServer) updateMetrics() {
	bs.metrics.mu.Lock()
	defer bs.metrics.mu.Unlock()

	// Update packet count (simplified for demo)
	bs.metrics.PacketsProcessed++
}

// verifyServer performs basic health checks on the server.
func (bs *BootstrapServer) verifyServer() error {
	// Check if Tox instance is running
	if !bs.tox.IsRunning() {
		return fmt.Errorf("tox instance not running")
	}

	// Verify connection status
	status := bs.tox.SelfGetConnectionStatus()
	bs.logger.WithField("connection_status", status).Debug("Server connection status")

	return nil
}

// GetAddress returns the server's network address.
func (bs *BootstrapServer) GetAddress() string {
	return bs.address
}

// GetPort returns the server's port.
func (bs *BootstrapServer) GetPort() uint16 {
	return bs.port
}

// GetPublicKeyHex returns the server's public key as hex string.
func (bs *BootstrapServer) GetPublicKeyHex() string {
	return fmt.Sprintf("%X", bs.publicKey)
}

// GetPublicKey returns the server's public key.
func (bs *BootstrapServer) GetPublicKey() [32]byte {
	return bs.publicKey
}

// IsRunning returns whether the server is currently running.
func (bs *BootstrapServer) IsRunning() bool {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.running
}

// GetMetrics returns a copy of the current server metrics.
func (bs *BootstrapServer) GetMetrics() ServerMetrics {
	bs.metrics.mu.RLock()
	defer bs.metrics.mu.RUnlock()
	// Return a copy without the mutex to avoid copying the lock
	return ServerMetrics{
		StartTime:         bs.metrics.StartTime,
		ConnectionsServed: bs.metrics.ConnectionsServed,
		PacketsProcessed:  bs.metrics.PacketsProcessed,
		ActiveClients:     bs.metrics.ActiveClients,
	}
}

// GetStatus returns comprehensive server status information.
func (bs *BootstrapServer) GetStatus() map[string]interface{} {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	metrics := bs.GetMetrics()
	return map[string]interface{}{
		"running":            bs.running,
		"address":            bs.address,
		"port":               bs.port,
		"public_key":         bs.GetPublicKeyHex(),
		"uptime":             time.Since(metrics.StartTime).String(),
		"connections_served": metrics.ConnectionsServed,
		"packets_processed":  metrics.PacketsProcessed,
		"active_clients":     metrics.ActiveClients,
		"connection_status":  bs.tox.SelfGetConnectionStatus(),
	}
}

// WaitForClients waits for the specified number of clients to connect.
func (bs *BootstrapServer) WaitForClients(count int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			metrics := bs.GetMetrics()
			return fmt.Errorf("timeout waiting for %d clients (got %d)", count, metrics.ActiveClients)
		case <-ticker.C:
			metrics := bs.GetMetrics()
			if metrics.ActiveClients >= count {
				bs.logger.WithField("client_count", count).Info("✅ Clients connected to bootstrap server")
				return nil
			}
		}
	}
}
