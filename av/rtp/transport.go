// Package rtp provides RTP transport integration with Tox infrastructure.
//
// This file handles the integration between RTP sessions and the
// existing Tox transport layer, providing seamless audio/video
// transmission over the secure Tox network.
package rtp

import (
	"fmt"
	"net"
	"sync"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// TransportIntegration manages RTP sessions over Tox transport.
//
// This provides the bridge between ToxAV RTP sessions and the
// underlying Tox transport infrastructure, handling packet
// routing and session management.
type TransportIntegration struct {
	mu        sync.RWMutex
	transport transport.Transport
	sessions  map[uint32]*Session // friendNumber -> Session
}

// NewTransportIntegration creates a new RTP transport integration.
//
// This sets up the integration layer between RTP sessions and
// Tox transport, registering appropriate packet handlers.
//
// Parameters:
//   - transport: The Tox transport to integrate with
//
// Returns:
//   - *TransportIntegration: New integration instance
//   - error: Any error that occurred during setup
func NewTransportIntegration(transport transport.Transport) (*TransportIntegration, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewTransportIntegration",
	}).Info("Creating new RTP transport integration")

	if transport == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewTransportIntegration",
			"error":    "transport cannot be nil",
		}).Error("Invalid transport")
		return nil, fmt.Errorf("transport cannot be nil")
	}

	integration := &TransportIntegration{
		transport: transport,
		sessions:  make(map[uint32]*Session),
	}

	// Register packet handlers for audio/video frames
	integration.setupPacketHandlers()

	logrus.WithFields(logrus.Fields{
		"function": "NewTransportIntegration",
	}).Info("RTP transport integration created successfully")

	return integration, nil
}

// setupPacketHandlers registers RTP packet handlers with the transport.
func (ti *TransportIntegration) setupPacketHandlers() {
	// Handler for incoming audio frames
	audioHandler := func(packet *transport.Packet, addr net.Addr) error {
		return ti.handleIncomingAudioFrame(packet, addr)
	}
	ti.transport.RegisterHandler(transport.PacketAVAudioFrame, audioHandler)

	// Handler for incoming video frames (placeholder for Phase 3)
	videoHandler := func(packet *transport.Packet, addr net.Addr) error {
		return ti.handleIncomingVideoFrame(packet, addr)
	}
	ti.transport.RegisterHandler(transport.PacketAVVideoFrame, videoHandler)
}

// CreateSession creates a new RTP session for a friend.
//
// This establishes an RTP session for audio/video communication
// with the specified friend over the Tox transport.
//
// Parameters:
//   - friendNumber: The friend number to create a session for
//   - remoteAddr: The remote address for this friend
//
// Returns:
//   - *Session: The created RTP session
//   - error: Any error that occurred during session creation
func (ti *TransportIntegration) CreateSession(friendNumber uint32, remoteAddr net.Addr) (*Session, error) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	// Check if session already exists
	if _, exists := ti.sessions[friendNumber]; exists {
		return nil, fmt.Errorf("session already exists for friend %d", friendNumber)
	}

	// Create new RTP session
	session, err := NewSession(friendNumber, ti.transport, remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create RTP session: %w", err)
	}

	// Store session
	ti.sessions[friendNumber] = session

	return session, nil
}

// GetSession retrieves an existing RTP session for a friend.
//
// Parameters:
//   - friendNumber: The friend number to get the session for
//
// Returns:
//   - *Session: The RTP session (nil if not found)
//   - bool: Whether the session exists
func (ti *TransportIntegration) GetSession(friendNumber uint32) (*Session, bool) {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	session, exists := ti.sessions[friendNumber]
	return session, exists
}

// CloseSession closes and removes an RTP session for a friend.
//
// Parameters:
//   - friendNumber: The friend number to close the session for
//
// Returns:
//   - error: Any error that occurred during session closure
func (ti *TransportIntegration) CloseSession(friendNumber uint32) error {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	session, exists := ti.sessions[friendNumber]
	if !exists {
		return fmt.Errorf("no session exists for friend %d", friendNumber)
	}

	// Close the session
	if err := session.Close(); err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	// Remove from sessions map
	delete(ti.sessions, friendNumber)

	return nil
}

// handleIncomingAudioFrame processes incoming audio RTP packets.
func (ti *TransportIntegration) handleIncomingAudioFrame(packet *transport.Packet, addr net.Addr) error {
	// Find the session for this address
	// In a full implementation, we would need address-to-friend mapping
	// For now, this is a placeholder for the integration

	// TODO: Implement address-to-friend mapping
	// TODO: Route packet to appropriate session's ReceivePacket method
	_ = packet
	_ = addr
	return nil
}

// handleIncomingVideoFrame processes incoming video RTP packets.
// This is a placeholder for Phase 3: Video Implementation.
func (ti *TransportIntegration) handleIncomingVideoFrame(packet *transport.Packet, addr net.Addr) error {
	// TODO: Implement in Phase 3: Video Implementation
	_ = packet
	_ = addr
	return nil
}

// GetAllSessions returns all active RTP sessions.
//
// Returns:
//   - map[uint32]*Session: Map of friend numbers to sessions
func (ti *TransportIntegration) GetAllSessions() map[uint32]*Session {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	// Return a copy to prevent external modification
	sessions := make(map[uint32]*Session)
	for friendNumber, session := range ti.sessions {
		sessions[friendNumber] = session
	}

	return sessions
}

// Close shuts down the transport integration and all sessions.
func (ti *TransportIntegration) Close() error {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	// Close all sessions
	for friendNumber, session := range ti.sessions {
		if err := session.Close(); err != nil {
			// Log error but continue closing other sessions
			fmt.Printf("Error closing session for friend %d: %v\n", friendNumber, err)
		}
	}

	// Clear sessions map
	ti.sessions = make(map[uint32]*Session)

	return nil
}
