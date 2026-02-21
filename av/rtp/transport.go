// Package rtp provides RTP transport integration with Tox infrastructure.
//
// This file handles the integration between RTP sessions and the
// existing Tox transport layer, providing seamless audio/video
// transmission over the secure Tox network.
package rtp

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// AudioReceiveCallback is called when a complete audio frame is received.
// Parameters: friendNumber, pcm samples, sample count, channels, sampling rate
type AudioReceiveCallback func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32)

// VideoReceiveCallback is called when a complete video frame is received.
// Parameters: friendNumber, pictureID, raw frame data
type VideoReceiveCallback func(friendNumber uint32, pictureID uint16, frameData []byte)

// TransportIntegration manages RTP sessions over Tox transport.
//
// This provides the bridge between ToxAV RTP sessions and the
// underlying Tox transport infrastructure, handling packet
// routing and session management.
type TransportIntegration struct {
	mu           sync.RWMutex
	transport    transport.Transport
	sessions     map[uint32]*Session // friendNumber -> Session
	addrToFriend map[string]uint32   // address string -> friendNumber
	friendToAddr map[uint32]net.Addr // friendNumber -> net.Addr

	// Callbacks for received media frames
	audioReceiveCallback AudioReceiveCallback
	videoReceiveCallback VideoReceiveCallback

	// handlersSetup guards against multiple handler registrations
	handlersSetup bool
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
		transport:    transport,
		sessions:     make(map[uint32]*Session),
		addrToFriend: make(map[string]uint32),
		friendToAddr: make(map[uint32]net.Addr),
	}

	// Register packet handlers for audio/video frames
	integration.setupPacketHandlers()

	logrus.WithFields(logrus.Fields{
		"function": "NewTransportIntegration",
	}).Info("RTP transport integration created successfully")

	return integration, nil
}

// setupPacketHandlers registers RTP packet handlers with the transport.
// This method is idempotent - calling it multiple times has no effect
// after the first call. This prevents duplicate handler registrations
// which could cause issues with packet processing.
func (ti *TransportIntegration) setupPacketHandlers() {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	// Guard against multiple registrations
	if ti.handlersSetup {
		return
	}
	ti.handlersSetup = true

	// Handler for incoming audio frames
	audioHandler := func(packet *transport.Packet, addr net.Addr) error {
		return ti.handleIncomingAudioFrame(packet, addr)
	}
	ti.transport.RegisterHandler(transport.PacketAVAudioFrame, audioHandler)

	// Handler for incoming video frames
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

	// Register address-to-friend mapping
	addrKey := remoteAddr.String()
	ti.addrToFriend[addrKey] = friendNumber
	ti.friendToAddr[friendNumber] = remoteAddr

	logrus.WithFields(logrus.Fields{
		"function":      "CreateSession",
		"friend_number": friendNumber,
		"remote_addr":   addrKey,
	}).Debug("Registered address-to-friend mapping")

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

	// Remove address mappings
	if addr, exists := ti.friendToAddr[friendNumber]; exists {
		addrKey := addr.String()
		delete(ti.addrToFriend, addrKey)
		delete(ti.friendToAddr, friendNumber)

		logrus.WithFields(logrus.Fields{
			"function":      "CloseSession",
			"friend_number": friendNumber,
			"remote_addr":   addrKey,
		}).Debug("Removed address-to-friend mapping")
	}

	// Remove from sessions map
	delete(ti.sessions, friendNumber)

	return nil
}

// handleIncomingAudioFrame processes incoming audio RTP packets.
func (ti *TransportIntegration) handleIncomingAudioFrame(packet *transport.Packet, addr net.Addr) error {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	// Look up friend number from address
	addrKey := addr.String()
	friendNumber, exists := ti.addrToFriend[addrKey]
	if !exists {
		logrus.WithFields(logrus.Fields{
			"function":    "handleIncomingAudioFrame",
			"remote_addr": addrKey,
		}).Debug("No session found for address")
		return fmt.Errorf("no session found for address %s", addrKey)
	}

	// Get the session for this friend
	session, exists := ti.sessions[friendNumber]
	if !exists {
		logrus.WithFields(logrus.Fields{
			"function":      "handleIncomingAudioFrame",
			"friend_number": friendNumber,
		}).Debug("Session not found for friend")
		return fmt.Errorf("session not found for friend %d", friendNumber)
	}

	// Route packet to the session's ReceivePacket method
	audioData, mediaType, err := session.ReceivePacket(packet.Data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "handleIncomingAudioFrame",
			"friend_number": friendNumber,
			"error":         err.Error(),
		}).Error("Failed to process incoming audio packet")
		return fmt.Errorf("failed to process packet: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "handleIncomingAudioFrame",
		"friend_number": friendNumber,
		"media_type":    mediaType,
		"data_size":     len(audioData),
	}).Debug("Successfully processed incoming audio frame")

	// Invoke audio receive callback if registered
	if ti.audioReceiveCallback != nil && len(audioData) > 0 {
		// Convert raw bytes to int16 PCM samples using explicit little-endian
		// byte order. PCM audio in Tox protocol uses little-endian format
		// for cross-platform compatibility.
		sampleCount := len(audioData) / 2
		pcm := make([]int16, sampleCount)
		for i := 0; i < sampleCount; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(audioData[i*2:]))
		}
		// Get audio parameters from session configuration
		audioConfig := session.GetAudioConfig()
		ti.audioReceiveCallback(friendNumber, pcm, sampleCount, audioConfig.Channels, audioConfig.SamplingRate)
	}

	return nil
}

// handleIncomingVideoFrame processes incoming video RTP packets.
//
// This method routes incoming video packets to the appropriate
// RTP session based on the sender's address, using the video
// depacketizer to reassemble fragmented frames.
func (ti *TransportIntegration) handleIncomingVideoFrame(packet *transport.Packet, addr net.Addr) error {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	// Look up friend number from address
	addrKey := addr.String()
	friendNumber, exists := ti.addrToFriend[addrKey]
	if !exists {
		logrus.WithFields(logrus.Fields{
			"function":    "handleIncomingVideoFrame",
			"remote_addr": addrKey,
		}).Debug("No session found for address")
		return fmt.Errorf("no session found for address %s", addrKey)
	}

	// Get the session for this friend
	session, exists := ti.sessions[friendNumber]
	if !exists {
		logrus.WithFields(logrus.Fields{
			"function":      "handleIncomingVideoFrame",
			"friend_number": friendNumber,
		}).Debug("Session not found for friend")
		return fmt.Errorf("session not found for friend %d", friendNumber)
	}

	// Route packet to the session's ReceiveVideoPacket method
	videoData, pictureID, err := session.ReceiveVideoPacket(packet.Data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "handleIncomingVideoFrame",
			"friend_number": friendNumber,
			"error":         err.Error(),
		}).Error("Failed to process incoming video packet")
		return fmt.Errorf("failed to process video packet: %w", err)
	}

	// Log and invoke callback only when we have a complete frame
	if videoData != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "handleIncomingVideoFrame",
			"friend_number": friendNumber,
			"picture_id":    pictureID,
			"frame_size":    len(videoData),
		}).Debug("Successfully received complete video frame")

		// Invoke video receive callback if registered
		if ti.videoReceiveCallback != nil {
			ti.videoReceiveCallback(friendNumber, pictureID, videoData)
		}
	}

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
			logrus.WithFields(logrus.Fields{
				"function":      "Close",
				"friend_number": friendNumber,
				"error":         err.Error(),
			}).Error("Error closing session")
		}
	}

	// Clear all maps
	ti.sessions = make(map[uint32]*Session)
	ti.addrToFriend = make(map[string]uint32)
	ti.friendToAddr = make(map[uint32]net.Addr)

	return nil
}

// SetAudioReceiveCallback registers a callback for received audio frames.
// The callback is invoked when a complete audio frame is received and decoded.
// Pass nil to unregister the callback.
func (ti *TransportIntegration) SetAudioReceiveCallback(callback AudioReceiveCallback) {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	ti.audioReceiveCallback = callback

	logrus.WithFields(logrus.Fields{
		"function":        "SetAudioReceiveCallback",
		"callback_is_nil": callback == nil,
	}).Debug("Audio receive callback registered")
}

// SetVideoReceiveCallback registers a callback for received video frames.
// The callback is invoked when a complete video frame is received and decoded.
// Pass nil to unregister the callback.
func (ti *TransportIntegration) SetVideoReceiveCallback(callback VideoReceiveCallback) {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	ti.videoReceiveCallback = callback

	logrus.WithFields(logrus.Fields{
		"function":        "SetVideoReceiveCallback",
		"callback_is_nil": callback == nil,
	}).Debug("Video receive callback registered")
}
