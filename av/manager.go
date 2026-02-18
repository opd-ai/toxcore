package av

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CallTimeout is the duration after which a call is considered timed out
// if no frames have been received.
const CallTimeout = 10 * time.Second

// Manager handles multiple concurrent audio/video calls and integrates with Tox.
//
// The Manager follows established patterns from the toxcore-go codebase:
// - Reuses existing transport and crypto infrastructure
// - Thread-safe operations with appropriate mutex usage
// - Interface-based design for testability
// - Integration with existing friend management system
type Manager struct {
	// Core integration - transport for signaling and media
	transport TransportInterface

	// Friend lookup function for packet routing
	// This function should return the friend's network address given their friend number
	friendAddressLookup func(friendNumber uint32) ([]byte, error)

	// Reverse lookup function to map network addresses back to friend numbers.
	// If nil, a simplified fallback is used for backward compatibility.
	addressFriendLookup func(addr []byte) (uint32, error)

	// Active calls mapping friend numbers to call instances
	calls map[uint32]*Call

	// State management
	running bool

	// Thread safety following established patterns
	mu sync.RWMutex

	// Iteration timing for integration with Tox event loop
	iterationInterval time.Duration

	// Call ID generation for unique call identification
	nextCallID uint32

	// Quality monitoring system
	qualityMonitor *QualityMonitor

	// Performance optimization system
	performanceOptimizer *PerformanceOptimizer

	// Call event callbacks
	callCallback      func(friendNumber uint32, audioEnabled, videoEnabled bool)
	callStateCallback func(friendNumber uint32, state CallState)

	// Frame receive callbacks for audio and video
	audioReceiveCallback func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32)
	videoReceiveCallback func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int)

	// Time provider for deterministic testing.
	// If nil, DefaultTimeProvider is used.
	timeProvider TimeProvider
}

// TransportInterface defines the minimal interface needed for AV signaling.
// This allows the manager to work with any transport implementation without
// tight coupling to specific transport types.
type TransportInterface interface {
	// Send sends a packet to the specified address
	Send(packetType byte, data, addr []byte) error

	// RegisterHandler registers a handler for specific packet types
	RegisterHandler(packetType byte, handler func(data, addr []byte) error)
}

// NewManager creates a new ToxAV manager instance with transport integration.
//
// The manager integrates with an existing transport and friend lookup system
// to provide audio/video calling capabilities. This follows the established
// pattern of constructor functions in toxcore-go.
//
// Parameters:
//   - transport: The transport interface for signaling and media
//   - friendAddressLookup: Function to get friend network addresses
//
// Returns:
//   - *Manager: The new manager instance
//   - error: Any error that occurred during setup
func NewManager(transport TransportInterface, friendAddressLookup func(uint32) ([]byte, error)) (*Manager, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewManager",
	}).Info("Creating new ToxAV manager instance")

	if transport == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewManager",
			"error":    "transport interface cannot be nil",
		}).Error("Transport validation failed")
		return nil, errors.New("transport interface cannot be nil")
	}
	if friendAddressLookup == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewManager",
			"error":    "friend address lookup function cannot be nil",
		}).Error("Friend lookup validation failed")
		return nil, errors.New("friend address lookup function cannot be nil")
	}

	manager := &Manager{
		transport:            transport,
		friendAddressLookup:  friendAddressLookup,
		calls:                make(map[uint32]*Call),
		running:              false,
		iterationInterval:    20 * time.Millisecond, // 50 FPS, typical for A/V applications
		nextCallID:           1,
		qualityMonitor:       NewQualityMonitor(nil), // Use default thresholds
		performanceOptimizer: NewPerformanceOptimizer(),
		timeProvider:         DefaultTimeProvider{},
	}

	logrus.WithFields(logrus.Fields{
		"function":           "NewManager",
		"iteration_interval": manager.iterationInterval,
		"initial_call_id":    manager.nextCallID,
	}).Debug("Manager instance configured")

	// Register packet handlers for AV signaling
	manager.registerPacketHandlers()

	logrus.WithFields(logrus.Fields{
		"function": "NewManager",
	}).Info("ToxAV manager created successfully")

	return manager, nil
}

// registerPacketHandlers sets up packet handlers for AV signaling.
// This integrates with the existing transport system to handle call-related packets.
func (m *Manager) registerPacketHandlers() {
	logrus.WithFields(logrus.Fields{
		"function": "registerPacketHandlers",
	}).Info("Registering ToxAV packet handlers")

	// Register handlers for AV packet types
	// Note: Using simple byte constants that will map to transport.PacketType values
	packetHandlers := map[byte]string{
		0x30: "CallRequest",
		0x31: "CallResponse",
		0x32: "CallControl",
		0x35: "BitrateControl",
	}

	for packetType, handlerName := range packetHandlers {
		logrus.WithFields(logrus.Fields{
			"function":     "registerPacketHandlers",
			"packet_type":  packetType,
			"handler_name": handlerName,
		}).Debug("Registering packet handler")
	}

	m.transport.RegisterHandler(0x30, m.handleCallRequest)    // PacketAVCallRequest
	m.transport.RegisterHandler(0x31, m.handleCallResponse)   // PacketAVCallResponse
	m.transport.RegisterHandler(0x32, m.handleCallControl)    // PacketAVCallControl
	m.transport.RegisterHandler(0x33, m.handleAudioFrame)     // PacketAVAudioFrame
	m.transport.RegisterHandler(0x34, m.handleVideoFrame)     // PacketAVVideoFrame
	m.transport.RegisterHandler(0x35, m.handleBitrateControl) // PacketAVBitrateControl

	logrus.WithFields(logrus.Fields{
		"function":      "registerPacketHandlers",
		"handler_count": len(packetHandlers) + 2, // Include audio and video frame handlers
	}).Info("ToxAV packet handlers registered successfully")
}

// handleCallRequest processes incoming call request packets.
func (m *Manager) handleCallRequest(data, addr []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":  "handleCallRequest",
		"data_size": len(data),
		"addr_size": len(addr),
	}).Info("Processing incoming call request")

	req, err := DeserializeCallRequest(data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "handleCallRequest",
			"error":    err.Error(),
		}).Error("Failed to deserialize call request")
		return fmt.Errorf("failed to deserialize call request: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":       "handleCallRequest",
		"call_id":        req.CallID,
		"audio_bit_rate": req.AudioBitRate,
		"video_bit_rate": req.VideoBitRate,
		"audio_enabled":  req.AudioBitRate > 0,
		"video_enabled":  req.VideoBitRate > 0,
	}).Debug("Call request deserialized")

	// Find which friend this request is from (simplified for Phase 1)
	friendNumber := m.findFriendByAddress(addr)
	if friendNumber == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "handleCallRequest",
			"error":    "call request from unknown friend",
		}).Error("Friend lookup failed")
		return errors.New("call request from unknown friend")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "handleCallRequest",
		"friend_number": friendNumber,
		"call_id":       req.CallID,
	}).Info("Call request from known friend")

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if there's already an active call
	if _, exists := m.calls[friendNumber]; exists {
		logrus.WithFields(logrus.Fields{
			"function":      "handleCallRequest",
			"friend_number": friendNumber,
			"call_id":       req.CallID,
			"action":        "rejecting - call already active",
		}).Warn("Rejecting call request - friend already has active call")
		// Send rejection response
		return m.sendCallResponse(friendNumber, req.CallID, false, 0, 0)
	}

	// Create new incoming call
	call := NewCall(friendNumber)
	call.callID = req.CallID
	call.audioEnabled = req.AudioBitRate > 0
	call.videoEnabled = req.VideoBitRate > 0
	call.audioBitRate = req.AudioBitRate
	call.videoBitRate = req.VideoBitRate
	m.updateCallState(call, CallStateSendingAudio) // Indicate incoming call state

	// Configure address resolver for RTP session setup
	if m.friendAddressLookup != nil {
		call.SetAddressResolver(m.friendAddressLookup)
	}

	m.calls[friendNumber] = call

	logrus.WithFields(logrus.Fields{
		"function":      "handleCallRequest",
		"friend_number": friendNumber,
		"call_id":       req.CallID,
		"audio_enabled": call.audioEnabled,
		"video_enabled": call.videoEnabled,
		"call_state":    call.GetState(),
	}).Info("Incoming call created successfully")

	// Invoke call callback if registered
	if m.callCallback != nil {
		m.callCallback(friendNumber, call.audioEnabled, call.videoEnabled)
		logrus.WithFields(logrus.Fields{
			"function":      "handleCallRequest",
			"friend_number": friendNumber,
		}).Debug("Call callback invoked")
	} else {
		// Fallback for when no callback is registered
		fmt.Printf("Incoming call from friend %d (audio: %t, video: %t)\n",
			friendNumber, call.audioEnabled, call.videoEnabled)
	}

	return nil
}

// handleCallResponse processes incoming call response packets.
func (m *Manager) handleCallResponse(data, addr []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":  "handleCallResponse",
		"data_size": len(data),
		"addr_size": len(addr),
	}).Info("Processing incoming call response")

	resp, err := DeserializeCallResponse(data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "handleCallResponse",
			"error":    err.Error(),
		}).Error("Failed to deserialize call response")
		return fmt.Errorf("failed to deserialize call response: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":       "handleCallResponse",
		"call_id":        resp.CallID,
		"accepted":       resp.Accepted,
		"audio_bit_rate": resp.AudioBitRate,
		"video_bit_rate": resp.VideoBitRate,
	}).Debug("Call response deserialized")

	friendNumber := m.findFriendByAddress(addr)
	if friendNumber == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "handleCallResponse",
			"error":    "call response from unknown friend",
		}).Error("Friend lookup failed")
		return errors.New("call response from unknown friend")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "handleCallResponse",
		"friend_number": friendNumber,
		"call_id":       resp.CallID,
		"accepted":      resp.Accepted,
	}).Info("Call response from known friend")

	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists || call.callID != resp.CallID {
		logrus.WithFields(logrus.Fields{
			"function":         "handleCallResponse",
			"friend_number":    friendNumber,
			"response_call_id": resp.CallID,
			"call_exists":      exists,
			"stored_call_id": func() uint32 {
				if exists {
					return call.callID
				} else {
					return 0
				}
			}(),
			"error": "call response for unknown call",
		}).Error("Call validation failed")
		return errors.New("call response for unknown call")
	}

	if resp.Accepted {
		call.audioEnabled = resp.AudioBitRate > 0
		call.videoEnabled = resp.VideoBitRate > 0
		call.audioBitRate = resp.AudioBitRate
		call.videoBitRate = resp.VideoBitRate
		m.updateCallState(call, CallStateSendingAudio)

		logrus.WithFields(logrus.Fields{
			"function":      "handleCallResponse",
			"friend_number": friendNumber,
			"call_id":       resp.CallID,
			"audio_enabled": call.audioEnabled,
			"video_enabled": call.videoEnabled,
			"call_state":    call.GetState(),
		}).Info("Call accepted by friend")

		fmt.Printf("Call accepted by friend %d (audio: %t, video: %t)\n",
			friendNumber, call.audioEnabled, call.videoEnabled)
	} else {
		m.updateCallState(call, CallStateFinished)
		delete(m.calls, friendNumber)

		fmt.Printf("Call rejected by friend %d\n", friendNumber)
	}

	return nil
}

// handleCallControl processes incoming call control packets.
func (m *Manager) handleCallControl(data, addr []byte) error {
	ctrl, err := DeserializeCallControl(data)
	if err != nil {
		return fmt.Errorf("failed to deserialize call control: %w", err)
	}

	friendNumber := m.findFriendByAddress(addr)
	if friendNumber == 0 {
		return errors.New("call control from unknown friend")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists || call.callID != ctrl.CallID {
		return errors.New("call control for unknown call")
	}

	switch ctrl.ControlType {
	case CallControlCancel:
		m.updateCallState(call, CallStateFinished)
		delete(m.calls, friendNumber)
		fmt.Printf("Call cancelled by friend %d\n", friendNumber)

	case CallControlPause:
		call.SetPaused(true)
		fmt.Printf("Call paused by friend %d\n", friendNumber)

	case CallControlResume:
		call.SetPaused(false)
		fmt.Printf("Call resumed by friend %d\n", friendNumber)

	case CallControlMuteAudio:
		// Friend has muted their audio - we won't receive audio frames
		fmt.Printf("Friend %d muted their audio\n", friendNumber)

	case CallControlUnmuteAudio:
		// Friend has unmuted their audio - we will receive audio frames
		fmt.Printf("Friend %d unmuted their audio\n", friendNumber)

	case CallControlHideVideo:
		// Friend has hidden their video - we won't receive video frames
		fmt.Printf("Friend %d hid their video\n", friendNumber)

	case CallControlShowVideo:
		// Friend has shown their video - we will receive video frames
		fmt.Printf("Friend %d showed their video\n", friendNumber)

	default:
		fmt.Printf("Call control %v from friend %d\n", ctrl.ControlType, friendNumber)
	}

	return nil
}

// handleBitrateControl processes incoming bitrate control packets.
func (m *Manager) handleBitrateControl(data, addr []byte) error {
	ctrl, err := DeserializeBitrateControl(data)
	if err != nil {
		return fmt.Errorf("failed to deserialize bitrate control: %w", err)
	}

	friendNumber := m.findFriendByAddress(addr)
	if friendNumber == 0 {
		return errors.New("bitrate control from unknown friend")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists || call.callID != ctrl.CallID {
		return errors.New("bitrate control for unknown call")
	}

	call.audioBitRate = ctrl.AudioBitRate
	call.videoBitRate = ctrl.VideoBitRate

	fmt.Printf("Bitrate changed by friend %d (audio: %d, video: %d)\n",
		friendNumber, ctrl.AudioBitRate, ctrl.VideoBitRate)

	return nil
}

// handleAudioFrame processes incoming audio RTP packets.
// This routes audio packets to the appropriate RTP session for the call
// and triggers the audio receive callback when complete frames are decoded.
func (m *Manager) handleAudioFrame(data, addr []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":  "handleAudioFrame",
		"data_size": len(data),
	}).Trace("Processing incoming audio frame")

	// Find the friend number for this address
	friendNumber := m.findFriendByAddress(addr)
	if friendNumber == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "handleAudioFrame",
			"error":    "audio frame from unknown friend",
		}).Warn("Ignoring audio frame from unknown peer")
		return errors.New("audio frame from unknown friend")
	}

	m.mu.RLock()
	call, exists := m.calls[friendNumber]
	audioCallback := m.audioReceiveCallback
	m.mu.RUnlock()

	if !exists {
		logrus.WithFields(logrus.Fields{
			"function":      "handleAudioFrame",
			"friend_number": friendNumber,
			"error":         "no active call",
		}).Warn("Received audio frame for non-existent call")
		return errors.New("no active call for audio frame")
	}

	// Update last frame time to prevent timeout
	call.updateLastFrame()

	// Route to RTP session and audio processor if available
	if call.rtpSession != nil && call.GetAudioProcessor() != nil {
		// Process RTP packet and get Opus-encoded frame data
		frameData, _, err := call.rtpSession.ReceivePacket(data)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":      "handleAudioFrame",
				"friend_number": friendNumber,
				"error":         err.Error(),
			}).Warn("Failed to process audio RTP packet")
			return fmt.Errorf("failed to process audio frame: %w", err)
		}

		// Frame not complete yet (still assembling fragments)
		if frameData == nil {
			logrus.WithFields(logrus.Fields{
				"function":      "handleAudioFrame",
				"friend_number": friendNumber,
			}).Trace("Audio frame not complete, waiting for more packets")
			return nil
		}

		// Decode the complete Opus frame to PCM
		audioProcessor := call.GetAudioProcessor()
		pcmSamples, sampleRate, err := audioProcessor.ProcessIncoming(frameData)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":      "handleAudioFrame",
				"friend_number": friendNumber,
				"error":         err.Error(),
			}).Warn("Failed to decode audio frame")
			return fmt.Errorf("failed to decode audio frame: %w", err)
		}

		logrus.WithFields(logrus.Fields{
			"function":      "handleAudioFrame",
			"friend_number": friendNumber,
			"sample_count":  len(pcmSamples),
			"sample_rate":   sampleRate,
		}).Trace("Audio frame decoded successfully")

		// Trigger audio receive callback if registered
		if audioCallback != nil {
			// Determine channel count (mono = 1, stereo = 2)
			// For now, assume mono. In future, this could be enhanced to detect stereo.
			channels := uint8(1)

			logrus.WithFields(logrus.Fields{
				"function":      "handleAudioFrame",
				"friend_number": friendNumber,
				"sample_count":  len(pcmSamples),
				"channels":      channels,
				"sample_rate":   sampleRate,
			}).Debug("Triggering audio receive callback")

			audioCallback(
				friendNumber,
				pcmSamples,
				len(pcmSamples),
				channels,
				sampleRate,
			)
		}

		logrus.WithFields(logrus.Fields{
			"function":      "handleAudioFrame",
			"friend_number": friendNumber,
			"frame_size":    len(data),
		}).Trace("Audio frame processed successfully")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":      "handleAudioFrame",
			"friend_number": friendNumber,
			"rtp_session":   call.rtpSession != nil,
			"audio_proc":    call.GetAudioProcessor() != nil,
		}).Debug("Audio processing not fully initialized, skipping")
	}

	return nil
}

// handleVideoFrame processes incoming video RTP packets.
// This routes video packets to the appropriate RTP session for the call
// and triggers the video receive callback when complete frames are decoded.
func (m *Manager) handleVideoFrame(data, addr []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":  "handleVideoFrame",
		"data_size": len(data),
	}).Trace("Processing incoming video frame")

	// Find the friend number for this address
	friendNumber := m.findFriendByAddress(addr)
	if friendNumber == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "handleVideoFrame",
			"error":    "video frame from unknown friend",
		}).Warn("Ignoring video frame from unknown peer")
		return errors.New("video frame from unknown friend")
	}

	m.mu.RLock()
	call, exists := m.calls[friendNumber]
	videoCallback := m.videoReceiveCallback
	m.mu.RUnlock()

	if !exists {
		logrus.WithFields(logrus.Fields{
			"function":      "handleVideoFrame",
			"friend_number": friendNumber,
			"error":         "no active call",
		}).Warn("Received video frame for non-existent call")
		return errors.New("no active call for video frame")
	}

	// Update last frame time to prevent timeout
	call.updateLastFrame()

	// Route to RTP session and video processor if available
	if call.rtpSession != nil && call.GetVideoProcessor() != nil {
		// Process RTP packet and get VP8-encoded frame data
		frameData, _, err := call.rtpSession.ReceiveVideoPacket(data)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":      "handleVideoFrame",
				"friend_number": friendNumber,
				"error":         err.Error(),
			}).Warn("Failed to process video RTP packet")
			return fmt.Errorf("failed to process video frame: %w", err)
		}

		// Frame not complete yet (still assembling fragments)
		if frameData == nil {
			logrus.WithFields(logrus.Fields{
				"function":      "handleVideoFrame",
				"friend_number": friendNumber,
			}).Trace("Video frame not complete, waiting for more packets")
			return nil
		}

		// Decode the complete VP8 frame to YUV420
		videoProcessor := call.GetVideoProcessor()
		decodedFrame, err := videoProcessor.ProcessIncomingLegacy(frameData)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":      "handleVideoFrame",
				"friend_number": friendNumber,
				"error":         err.Error(),
			}).Warn("Failed to decode video frame")
			return fmt.Errorf("failed to decode video frame: %w", err)
		}

		logrus.WithFields(logrus.Fields{
			"function":      "handleVideoFrame",
			"friend_number": friendNumber,
			"frame_width":   decodedFrame.Width,
			"frame_height":  decodedFrame.Height,
		}).Trace("Video frame decoded successfully")

		// Trigger video receive callback if registered
		if videoCallback != nil {
			logrus.WithFields(logrus.Fields{
				"function":      "handleVideoFrame",
				"friend_number": friendNumber,
				"width":         decodedFrame.Width,
				"height":        decodedFrame.Height,
			}).Debug("Triggering video receive callback")

			videoCallback(
				friendNumber,
				decodedFrame.Width,
				decodedFrame.Height,
				decodedFrame.Y,
				decodedFrame.U,
				decodedFrame.V,
				decodedFrame.YStride,
				decodedFrame.UStride,
				decodedFrame.VStride,
			)
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"function":      "handleVideoFrame",
			"friend_number": friendNumber,
			"rtp_session":   call.rtpSession != nil,
			"video_proc":    call.GetVideoProcessor() != nil,
		}).Debug("Video processing not fully initialized, skipping")
	}

	return nil
}

// findFriendByAddress maps network addresses to friend numbers.
// Uses the addressFriendLookup callback if configured, otherwise falls back
// to a simplified implementation for backward compatibility.
func (m *Manager) findFriendByAddress(addr []byte) uint32 {
	// Use the configured callback if available
	if m.addressFriendLookup != nil {
		friendNumber, err := m.addressFriendLookup(addr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "findFriendByAddress",
				"error":    err.Error(),
			}).Debug("Address-to-friend lookup failed, returning 0")
			return 0
		}
		return friendNumber
	}

	// Fallback: simplified implementation for backward compatibility
	// In full integration, addressFriendLookup should be configured
	if len(addr) >= 4 {
		// Use first byte as the friend number (simplified for testing)
		return uint32(addr[0])
	}
	return 0
}

// SetAddressFriendLookup configures the callback for reverse address lookups.
// This should map network addresses back to friend numbers for incoming packets.
func (m *Manager) SetAddressFriendLookup(lookup func(addr []byte) (uint32, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addressFriendLookup = lookup
}

// getTimeProvider returns the time provider, defaulting to DefaultTimeProvider if nil.
func (m *Manager) getTimeProvider() TimeProvider {
	if m.timeProvider == nil {
		return DefaultTimeProvider{}
	}
	return m.timeProvider
}

// SetTimeProvider sets the time provider for deterministic testing.
// If tp is nil, DefaultTimeProvider is used.
func (m *Manager) SetTimeProvider(tp TimeProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tp == nil {
		tp = DefaultTimeProvider{}
	}
	m.timeProvider = tp
}

// sendCallResponse sends a call response packet to a friend.
func (m *Manager) sendCallResponse(friendNumber, callID uint32, accepted bool, audioBitRate, videoBitRate uint32) error {
	resp := &CallResponsePacket{
		CallID:       callID,
		Accepted:     accepted,
		AudioBitRate: audioBitRate,
		VideoBitRate: videoBitRate,
		Timestamp:    m.getTimeProvider().Now(),
	}

	data, err := SerializeCallResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to serialize call response: %w", err)
	}

	addr, err := m.friendAddressLookup(friendNumber)
	if err != nil {
		return fmt.Errorf("failed to get friend address: %w", err)
	}

	return m.transport.Send(0x31, data, addr) // PacketAVCallResponse
}

// validateCallPrerequisites checks if the manager is running and validates call state.
func (m *Manager) validateCallPrerequisites(friendNumber uint32) error {
	if !m.running {
		logrus.WithFields(logrus.Fields{
			"function": "StartCall",
			"error":    "manager is not running",
		}).Error("Manager state validation failed")
		return errors.New("manager is not running")
	}

	// Check if there's already an active call with this friend
	if _, exists := m.calls[friendNumber]; exists {
		logrus.WithFields(logrus.Fields{
			"function":      "StartCall",
			"friend_number": friendNumber,
			"error":         "call already active with this friend",
		}).Error("Call state validation failed")
		return errors.New("call already active with this friend")
	}

	return nil
}

// generateUniqueCallID generates and returns a unique call ID.
func (m *Manager) generateUniqueCallID(friendNumber uint32) uint32 {
	callID := m.nextCallID
	m.nextCallID++

	logrus.WithFields(logrus.Fields{
		"function":      "StartCall",
		"friend_number": friendNumber,
		"call_id":       callID,
	}).Debug("Generated unique call ID")

	return callID
}

// createAndSendCallRequest creates a call request packet and sends it to the friend.
func (m *Manager) createAndSendCallRequest(friendNumber, callID, audioBitRate, videoBitRate uint32) error {
	// Create call request packet
	req := &CallRequestPacket{
		CallID:       callID,
		AudioBitRate: audioBitRate,
		VideoBitRate: videoBitRate,
		Timestamp:    m.getTimeProvider().Now(),
	}

	// Serialize and send the request
	logrus.WithFields(logrus.Fields{
		"function": "StartCall",
		"call_id":  callID,
	}).Debug("Serializing call request packet")

	data, err := SerializeCallRequest(req)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "StartCall",
			"call_id":  callID,
			"error":    err.Error(),
		}).Error("Failed to serialize call request")
		return fmt.Errorf("failed to serialize call request: %w", err)
	}

	addr, err := m.friendAddressLookup(friendNumber)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "StartCall",
			"friend_number": friendNumber,
			"error":         err.Error(),
		}).Error("Failed to lookup friend address")
		return fmt.Errorf("failed to get friend address: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "StartCall",
		"friend_number": friendNumber,
		"call_id":       callID,
		"packet_size":   len(data),
		"addr_size":     len(addr),
	}).Debug("Sending call request packet")

	err = m.transport.Send(0x30, data, addr) // PacketAVCallRequest
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "StartCall",
			"call_id":  callID,
			"error":    err.Error(),
		}).Error("Failed to send call request")
		return fmt.Errorf("failed to send call request: %w", err)
	}

	return nil
}

// createCallSession creates a new call session with the specified parameters.
func (m *Manager) createCallSession(friendNumber, callID, audioBitRate, videoBitRate uint32) *Call {
	call := NewCall(friendNumber)
	call.callID = callID
	call.audioEnabled = audioBitRate > 0
	call.videoEnabled = videoBitRate > 0
	call.audioBitRate = audioBitRate
	call.videoBitRate = videoBitRate
	m.updateCallState(call, CallStateSendingAudio) // Outgoing call state
	call.startTime = m.getTimeProvider().Now()

	// Configure time provider for the call to match the manager's time provider
	call.SetTimeProvider(m.timeProvider)

	// Configure address resolver for RTP session setup
	if m.friendAddressLookup != nil {
		call.SetAddressResolver(m.friendAddressLookup)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "StartCall",
		"friend_number": friendNumber,
		"call_id":       callID,
		"call_state":    call.GetState(),
	}).Debug("Call session created, setting up media")

	return call
}

// setupCallMedia sets up media components for the call and handles cleanup on failure.
func (m *Manager) setupCallMedia(call *Call, friendNumber, callID uint32) error {
	// Setup media components for audio frame processing (Phase 2 integration)
	err := call.SetupMedia(m.transport, friendNumber)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "StartCall",
			"call_id":  callID,
			"error":    err.Error(),
		}).Error("Failed to setup media for call")
		// Clean up call if media setup fails
		delete(m.calls, friendNumber)
		return fmt.Errorf("failed to setup media for call: %w", err)
	}

	return nil
}

// StartCall initiates a new audio/video call to a friend.
//
// This method sends a call request packet and creates a new call session.
// It follows the established pattern of async operations in toxcore-go.
//
// Parameters:
//   - friendNumber: The friend to call
//   - audioBitRate: Audio bit rate (0 to disable audio)
//   - videoBitRate: Video bit rate (0 to disable video)
//
// Returns:
//   - error: Any error that occurred during call initiation
func (m *Manager) StartCall(friendNumber, audioBitRate, videoBitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":       "StartCall",
		"friend_number":  friendNumber,
		"audio_bit_rate": audioBitRate,
		"video_bit_rate": videoBitRate,
		"audio_enabled":  audioBitRate > 0,
		"video_enabled":  videoBitRate > 0,
	}).Info("Starting call to friend")

	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate prerequisites
	if err := m.validateCallPrerequisites(friendNumber); err != nil {
		return err
	}

	// Generate unique call ID
	callID := m.generateUniqueCallID(friendNumber)

	// Create and send call request
	if err := m.createAndSendCallRequest(friendNumber, callID, audioBitRate, videoBitRate); err != nil {
		return err
	}

	// Create call session
	call := m.createCallSession(friendNumber, callID, audioBitRate, videoBitRate)

	// Setup media components
	if err := m.setupCallMedia(call, friendNumber, callID); err != nil {
		return err
	}

	// Store call session
	m.calls[friendNumber] = call

	logrus.WithFields(logrus.Fields{
		"function":      "StartCall",
		"friend_number": friendNumber,
		"call_id":       callID,
		"audio_enabled": call.audioEnabled,
		"video_enabled": call.videoEnabled,
		"call_state":    call.GetState(),
	}).Info("Call started successfully")

	fmt.Printf("Started call to friend %d (callID: %d, audio: %t, video: %t)\n",
		friendNumber, callID, call.audioEnabled, call.videoEnabled)

	return nil
}

// AnswerCall accepts an incoming call from a friend.
//
// This method sends a call response packet accepting the call and updates
// the call state. It follows established patterns for response handling.
//
// Parameters:
//   - friendNumber: The friend whose call to answer
//   - audioBitRate: Audio bit rate to accept (0 to disable audio)
//   - videoBitRate: Video bit rate to accept (0 to disable video)
//
// Returns:
//   - error: Any error that occurred during call acceptance
func (m *Manager) AnswerCall(friendNumber, audioBitRate, videoBitRate uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return errors.New("manager is not running")
	}

	call, exists := m.calls[friendNumber]
	if !exists {
		return errors.New("no incoming call from this friend")
	}

	// Send acceptance response
	err := m.sendCallResponse(friendNumber, call.callID, true, audioBitRate, videoBitRate)
	if err != nil {
		return fmt.Errorf("failed to send call response: %w", err)
	}

	// Update call state
	call.audioEnabled = audioBitRate > 0
	call.videoEnabled = videoBitRate > 0
	call.audioBitRate = audioBitRate
	call.videoBitRate = videoBitRate
	m.updateCallState(call, CallStateSendingAudio)
	call.startTime = m.getTimeProvider().Now()

	// Ensure call uses the same time provider as the manager
	call.SetTimeProvider(m.timeProvider)

	// Setup media components for audio frame processing (Phase 2 integration)
	err = call.SetupMedia(m.transport, friendNumber)
	if err != nil {
		// If media setup fails, end the call
		m.updateCallState(call, CallStateError)
		return fmt.Errorf("failed to setup media for answered call: %w", err)
	}

	fmt.Printf("Answered call from friend %d (audio: %t, video: %t)\n",
		friendNumber, call.audioEnabled, call.videoEnabled)

	return nil
}

// EndCall terminates an active call with a friend.
//
// This method sends a call control packet to cancel the call and cleans up
// the call session. It follows established cleanup patterns.
//
// Parameters:
//   - friendNumber: The friend whose call to end
//
// Returns:
//   - error: Any error that occurred during call termination
func (m *Manager) EndCall(friendNumber uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists {
		return errors.New("no active call with this friend")
	}

	// Send call control packet to cancel the call
	ctrl := &CallControlPacket{
		CallID:      call.callID,
		ControlType: CallControlCancel,
		Timestamp:   time.Now(),
	}

	data, err := SerializeCallControl(ctrl)
	if err != nil {
		return fmt.Errorf("failed to serialize call control: %w", err)
	}

	addr, err := m.friendAddressLookup(friendNumber)
	if err != nil {
		return fmt.Errorf("failed to get friend address: %w", err)
	}

	err = m.transport.Send(0x32, data, addr) // PacketAVCallControl
	if err != nil {
		return fmt.Errorf("failed to send call control: %w", err)
	}

	// Clean up call session and media resources
	m.updateCallState(call, CallStateFinished)
	call.CleanupMedia() // Release audio processor and RTP session resources
	delete(m.calls, friendNumber)

	fmt.Printf("Ended call with friend %d\n", friendNumber)

	return nil
}

// PauseCall pauses an active call, stopping media transmission.
//
// This sends a pause control packet to the friend and updates the call state.
// Media transmission will stop until the call is resumed.
//
// Parameters:
//   - friendNumber: The friend whose call to pause
//
// Returns:
//   - error: Any error that occurred during pause
func (m *Manager) PauseCall(friendNumber uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists {
		return errors.New("no active call with this friend")
	}

	if call.IsPaused() {
		return errors.New("call is already paused")
	}

	// Send pause control packet
	ctrl := &CallControlPacket{
		CallID:      call.callID,
		ControlType: CallControlPause,
		Timestamp:   time.Now(),
	}

	data, err := SerializeCallControl(ctrl)
	if err != nil {
		return fmt.Errorf("failed to serialize call control: %w", err)
	}

	addr, err := m.friendAddressLookup(friendNumber)
	if err != nil {
		return fmt.Errorf("failed to get friend address: %w", err)
	}

	err = m.transport.Send(0x32, data, addr)
	if err != nil {
		return fmt.Errorf("failed to send pause control: %w", err)
	}

	call.SetPaused(true)
	logrus.WithFields(logrus.Fields{
		"function":      "PauseCall",
		"friend_number": friendNumber,
	}).Info("Call paused successfully")

	return nil
}

// ResumeCall resumes a paused call, restarting media transmission.
//
// This sends a resume control packet to the friend and updates the call state.
//
// Parameters:
//   - friendNumber: The friend whose call to resume
//
// Returns:
//   - error: Any error that occurred during resume
func (m *Manager) ResumeCall(friendNumber uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists {
		return errors.New("no active call with this friend")
	}

	if !call.IsPaused() {
		return errors.New("call is not paused")
	}

	// Send resume control packet
	ctrl := &CallControlPacket{
		CallID:      call.callID,
		ControlType: CallControlResume,
		Timestamp:   time.Now(),
	}

	data, err := SerializeCallControl(ctrl)
	if err != nil {
		return fmt.Errorf("failed to serialize call control: %w", err)
	}

	addr, err := m.friendAddressLookup(friendNumber)
	if err != nil {
		return fmt.Errorf("failed to get friend address: %w", err)
	}

	err = m.transport.Send(0x32, data, addr)
	if err != nil {
		return fmt.Errorf("failed to send resume control: %w", err)
	}

	call.SetPaused(false)
	logrus.WithFields(logrus.Fields{
		"function":      "ResumeCall",
		"friend_number": friendNumber,
	}).Info("Call resumed successfully")

	return nil
}

// MuteAudio mutes outgoing audio for the call.
//
// This sends a mute audio control packet to the friend. Audio frames
// will continue to be received but will not be transmitted.
//
// Parameters:
//   - friendNumber: The friend whose call audio to mute
//
// Returns:
//   - error: Any error that occurred during muting
func (m *Manager) MuteAudio(friendNumber uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists {
		return errors.New("no active call with this friend")
	}

	if call.IsAudioMuted() {
		return errors.New("audio is already muted")
	}

	// Send mute audio control packet
	ctrl := &CallControlPacket{
		CallID:      call.callID,
		ControlType: CallControlMuteAudio,
		Timestamp:   time.Now(),
	}

	data, err := SerializeCallControl(ctrl)
	if err != nil {
		return fmt.Errorf("failed to serialize call control: %w", err)
	}

	addr, err := m.friendAddressLookup(friendNumber)
	if err != nil {
		return fmt.Errorf("failed to get friend address: %w", err)
	}

	err = m.transport.Send(0x32, data, addr)
	if err != nil {
		return fmt.Errorf("failed to send mute control: %w", err)
	}

	call.SetAudioMuted(true)
	logrus.WithFields(logrus.Fields{
		"function":      "MuteAudio",
		"friend_number": friendNumber,
	}).Info("Audio muted successfully")

	return nil
}

// UnmuteAudio unmutes outgoing audio for the call.
//
// This sends an unmute audio control packet to the friend. Audio frames
// will resume being transmitted.
//
// Parameters:
//   - friendNumber: The friend whose call audio to unmute
//
// Returns:
//   - error: Any error that occurred during unmuting
func (m *Manager) UnmuteAudio(friendNumber uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists {
		return errors.New("no active call with this friend")
	}

	if !call.IsAudioMuted() {
		return errors.New("audio is not muted")
	}

	// Send unmute audio control packet
	ctrl := &CallControlPacket{
		CallID:      call.callID,
		ControlType: CallControlUnmuteAudio,
		Timestamp:   time.Now(),
	}

	data, err := SerializeCallControl(ctrl)
	if err != nil {
		return fmt.Errorf("failed to serialize call control: %w", err)
	}

	addr, err := m.friendAddressLookup(friendNumber)
	if err != nil {
		return fmt.Errorf("failed to get friend address: %w", err)
	}

	err = m.transport.Send(0x32, data, addr)
	if err != nil {
		return fmt.Errorf("failed to send unmute control: %w", err)
	}

	call.SetAudioMuted(false)
	logrus.WithFields(logrus.Fields{
		"function":      "UnmuteAudio",
		"friend_number": friendNumber,
	}).Info("Audio unmuted successfully")

	return nil
}

// HideVideo hides outgoing video for the call.
//
// This sends a hide video control packet to the friend. Video frames
// will continue to be received but will not be transmitted.
//
// Parameters:
//   - friendNumber: The friend whose call video to hide
//
// Returns:
//   - error: Any error that occurred during hiding
func (m *Manager) HideVideo(friendNumber uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists {
		return errors.New("no active call with this friend")
	}

	if call.IsVideoHidden() {
		return errors.New("video is already hidden")
	}

	// Send hide video control packet
	ctrl := &CallControlPacket{
		CallID:      call.callID,
		ControlType: CallControlHideVideo,
		Timestamp:   time.Now(),
	}

	data, err := SerializeCallControl(ctrl)
	if err != nil {
		return fmt.Errorf("failed to serialize call control: %w", err)
	}

	addr, err := m.friendAddressLookup(friendNumber)
	if err != nil {
		return fmt.Errorf("failed to get friend address: %w", err)
	}

	err = m.transport.Send(0x32, data, addr)
	if err != nil {
		return fmt.Errorf("failed to send hide video control: %w", err)
	}

	call.SetVideoHidden(true)
	logrus.WithFields(logrus.Fields{
		"function":      "HideVideo",
		"friend_number": friendNumber,
	}).Info("Video hidden successfully")

	return nil
}

// ShowVideo shows outgoing video for the call.
//
// This sends a show video control packet to the friend. Video frames
// will resume being transmitted.
//
// Parameters:
//   - friendNumber: The friend whose call video to show
//
// Returns:
//   - error: Any error that occurred during showing
func (m *Manager) ShowVideo(friendNumber uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists {
		return errors.New("no active call with this friend")
	}

	if !call.IsVideoHidden() {
		return errors.New("video is not hidden")
	}

	// Send show video control packet
	ctrl := &CallControlPacket{
		CallID:      call.callID,
		ControlType: CallControlShowVideo,
		Timestamp:   time.Now(),
	}

	data, err := SerializeCallControl(ctrl)
	if err != nil {
		return fmt.Errorf("failed to serialize call control: %w", err)
	}

	addr, err := m.friendAddressLookup(friendNumber)
	if err != nil {
		return fmt.Errorf("failed to get friend address: %w", err)
	}

	err = m.transport.Send(0x32, data, addr)
	if err != nil {
		return fmt.Errorf("failed to send show video control: %w", err)
	}

	call.SetVideoHidden(false)
	logrus.WithFields(logrus.Fields{
		"function":      "ShowVideo",
		"friend_number": friendNumber,
	}).Info("Video shown successfully")

	return nil
}

// Start begins the manager's operation.
//
// This method should be called after creating the manager and before
// starting any calls. It follows the established pattern of lifecycle
// management in toxcore-go components.
func (m *Manager) Start() error {
	logrus.WithFields(logrus.Fields{
		"function": "Start",
	}).Debug("Starting AV manager")

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		logrus.WithFields(logrus.Fields{
			"function": "Start",
			"error":    "already running",
		}).Error("AV manager is already running")
		return errors.New("manager is already running")
	}

	m.running = true

	logrus.WithFields(logrus.Fields{
		"function": "Start",
	}).Info("AV manager started successfully")

	return nil
}

// Stop gracefully shuts down the manager.
//
// This method ends all active calls and stops the manager operation.
// It follows the established cleanup patterns in toxcore-go.
func (m *Manager) Stop() error {
	logrus.WithFields(logrus.Fields{
		"function": "Stop",
	}).Debug("Stopping AV manager")

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		logrus.WithFields(logrus.Fields{
			"function": "Stop",
		}).Debug("AV manager already stopped")
		return nil
	}

	activeCallCount := len(m.calls)
	logrus.WithFields(logrus.Fields{
		"function":          "Stop",
		"active_call_count": activeCallCount,
	}).Info("Ending all active calls before shutdown")

	// End all active calls
	for friendNumber, call := range m.calls {
		logrus.WithFields(logrus.Fields{
			"function":      "Stop",
			"friend_number": friendNumber,
		}).Debug("Ending call with friend")

		m.updateCallState(call, CallStateFinished)
		delete(m.calls, friendNumber)
	}

	m.running = false

	logrus.WithFields(logrus.Fields{
		"function":    "Stop",
		"calls_ended": activeCallCount,
	}).Info("AV manager stopped successfully")

	return nil
}

// IsRunning returns whether the manager is currently running.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// IterationInterval returns the recommended interval for calling Iterate().
//
// This follows the established pattern in toxcore-go where components
// provide their own iteration timing requirements.
func (m *Manager) IterationInterval() time.Duration {
	return m.iterationInterval
}

// Iterate performs one iteration of the manager's event loop.
//
// This method should be called regularly (at IterationInterval) to
// process A/V events, handle timeouts, and maintain call state.
// It follows the established iteration pattern in toxcore-go.
//
// Performance optimizations:
// - Uses object pooling to minimize allocations
// - Employs caching to reduce lock contention
// - Provides conditional logging for optimal performance
func (m *Manager) Iterate() {
	// Use performance optimizer for fast-path iteration
	callSlice, shouldProcess := m.performanceOptimizer.OptimizeIteration(m)
	if !shouldProcess {
		return
	}

	// Ensure we return the call slice to the pool
	defer m.performanceOptimizer.ReturnCallSlice(callSlice)

	callCount := len(callSlice)
	if callCount > 0 && m.performanceOptimizer.IsDetailedLoggingEnabled() {
		logrus.WithFields(logrus.Fields{
			"function":   "Iterate",
			"call_count": callCount,
		}).Trace("Processing active calls")
	}

	// Process each active call
	for _, call := range callSlice {
		m.processCall(call)
	}

	if callCount > 0 && m.performanceOptimizer.IsDetailedLoggingEnabled() {
		logrus.WithFields(logrus.Fields{
			"function":   "Iterate",
			"call_count": callCount,
		}).Trace("AV manager iteration completed")
	}
}

// processCall handles the processing for an individual call.
//
// This method checks for timeouts, processes incoming media,
// and handles state transitions. It's called during iteration
// for each active call.
func (m *Manager) processCall(call *Call) {
	friendNumber := call.GetFriendNumber()
	logrus.WithFields(logrus.Fields{
		"function":      "processCall",
		"friend_number": friendNumber,
	}).Trace("Processing call")

	// Handle call timeouts - check if call has been inactive too long
	state := call.GetState()
	if state != CallStateNone && state != CallStateError && state != CallStateFinished {
		lastFrame := call.GetLastFrameTime()
		startTime := call.GetStartTime()

		// Only check timeout if call has actually started (startTime is set)
		if !startTime.IsZero() && !lastFrame.IsZero() {
			timeSinceLastFrame := time.Since(lastFrame)
			if timeSinceLastFrame > CallTimeout {
				logrus.WithFields(logrus.Fields{
					"function":              "processCall",
					"friend_number":         friendNumber,
					"time_since_last_frame": timeSinceLastFrame,
					"timeout_threshold":     CallTimeout,
				}).Warn("Call timed out due to inactivity")

				// Mark call as finished due to timeout
				m.mu.Lock()
				m.updateCallState(call, CallStateFinished)
				delete(m.calls, friendNumber)
				m.mu.Unlock()
				return
			}
		}
	}

	// Process quality monitoring for active calls
	if state != CallStateNone && state != CallStateError && state != CallStateFinished {
		// Get bitrate adapter for this call (if available)
		adapter := call.GetBitrateAdapter()

		// Monitor call quality
		_, err := m.qualityMonitor.MonitorCall(call, adapter)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":      "processCall",
				"friend_number": friendNumber,
				"error":         err.Error(),
			}).Warn("Quality monitoring failed")
		}
	}

	// Remove failed or finished calls
	if state == CallStateError || state == CallStateFinished {
		logrus.WithFields(logrus.Fields{
			"function":      "processCall",
			"friend_number": friendNumber,
			"state":         state,
		}).Info("Removing completed call")

		// Remove failed/finished calls
		m.mu.Lock()
		delete(m.calls, friendNumber)
		m.mu.Unlock()

		logrus.WithFields(logrus.Fields{
			"function":      "processCall",
			"friend_number": friendNumber,
		}).Info("Failed call removed from active calls")
	}
}

// GetCall retrieves the call instance for a specific friend.
//
// This method provides access to call information for monitoring
// and control purposes. Returns nil if no call exists.
func (m *Manager) GetCall(friendNumber uint32) *Call {
	logrus.WithFields(logrus.Fields{
		"function":      "GetCall",
		"friend_number": friendNumber,
	}).Trace("Looking up call for friend")

	m.mu.RLock()
	defer m.mu.RUnlock()
	call := m.calls[friendNumber]

	logrus.WithFields(logrus.Fields{
		"function":      "GetCall",
		"friend_number": friendNumber,
		"call_found":    call != nil,
	}).Trace("Call lookup completed")

	return call
}

// GetCallCount returns the number of currently active calls.
func (m *Manager) GetCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.calls)
}

// GetQualityMonitor returns the quality monitoring system.
//
// This provides access to the quality monitoring configuration
// and allows customization of quality monitoring behavior.
func (m *Manager) GetQualityMonitor() *QualityMonitor {
	return m.qualityMonitor
}

// SetQualityCallback sets a callback for quality changes across all calls.
//
// This is a convenience method that configures the global quality
// monitoring callback for all calls managed by this instance.
func (m *Manager) SetQualityCallback(callback func(friendNumber uint32, metrics CallMetrics)) {
	if m.qualityMonitor != nil {
		m.qualityMonitor.SetQualityCallback(callback)
	}
}

// Performance Optimization Methods

// GetPerformanceMetrics returns current performance statistics.
//
// Provides detailed insights into system performance including:
// - Total iterations and calls processed since startup
// - Average and peak iteration times
// - Current caching status and configuration
func (m *Manager) GetPerformanceMetrics() PerformanceMetrics {
	if m.performanceOptimizer == nil {
		return PerformanceMetrics{}
	}
	return m.performanceOptimizer.GetPerformanceMetrics()
}

// EnablePerformanceOptimization configures performance optimization settings.
//
// Parameters:
//   - detailedLogging: Enable detailed logging for debugging (impacts performance)
//   - cpuProfiling: Enable CPU profiling for performance analysis
//
// This method allows fine-tuning of performance characteristics based on
// deployment requirements (development vs production).
func (m *Manager) EnablePerformanceOptimization(detailedLogging, cpuProfiling bool) error {
	if m.performanceOptimizer == nil {
		return errors.New("performance optimizer not initialized")
	}

	logrus.WithFields(logrus.Fields{
		"function":         "EnablePerformanceOptimization",
		"detailed_logging": detailedLogging,
		"cpu_profiling":    cpuProfiling,
	}).Info("Configuring performance optimization")

	m.performanceOptimizer.EnableDetailedLogging(detailedLogging)

	if cpuProfiling {
		if err := m.performanceOptimizer.StartCPUProfiling(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "EnablePerformanceOptimization",
				"error":    err.Error(),
			}).Error("Failed to start CPU profiling")
			return fmt.Errorf("failed to start CPU profiling: %w", err)
		}
	} else {
		m.performanceOptimizer.StopCPUProfiling()
	}

	logrus.WithFields(logrus.Fields{
		"function": "EnablePerformanceOptimization",
	}).Info("Performance optimization configuration completed")

	return nil
}

// ResetPerformanceMetrics resets all performance counters and statistics.
//
// Useful for benchmarking and performance testing to obtain clean measurements
// without historical data affecting the results.
func (m *Manager) ResetPerformanceMetrics() {
	if m.performanceOptimizer != nil {
		m.performanceOptimizer.ResetPerformanceMetrics()
	}
}

// GetPerformanceOptimizer provides direct access to the performance optimizer.
//
// This method is primarily intended for testing and advanced use cases
// where direct access to optimizer functionality is required.
func (m *Manager) GetPerformanceOptimizer() *PerformanceOptimizer {
	return m.performanceOptimizer
}

// SetAudioReceiveCallback registers a callback for incoming audio frames.
//
// The callback will be invoked when complete audio frames are received and
// decoded from incoming RTP packets. This allows the ToxAV layer to notify
// applications about received audio data.
//
// Parameters:
//   - callback: Function to call when an audio frame is received, or nil to unregister
func (m *Manager) SetAudioReceiveCallback(callback func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32)) {
	logrus.WithFields(logrus.Fields{
		"function":        "SetAudioReceiveCallback",
		"callback_is_nil": callback == nil,
	}).Debug("Registering audio receive callback")

	m.mu.Lock()
	defer m.mu.Unlock()
	m.audioReceiveCallback = callback

	logrus.WithFields(logrus.Fields{
		"function": "SetAudioReceiveCallback",
	}).Info("Audio receive callback registered")
}

// SetVideoReceiveCallback registers a callback for incoming video frames.
//
// The callback will be invoked when complete video frames are received and
// decoded from incoming RTP packets. The frames are provided in YUV420 format
// with stride information for proper frame reconstruction.
//
// Parameters:
//   - callback: Function to call when a video frame is received, or nil to unregister
func (m *Manager) SetVideoReceiveCallback(callback func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int)) {
	logrus.WithFields(logrus.Fields{
		"function":        "SetVideoReceiveCallback",
		"callback_is_nil": callback == nil,
	}).Debug("Registering video receive callback")

	m.mu.Lock()
	defer m.mu.Unlock()
	m.videoReceiveCallback = callback

	logrus.WithFields(logrus.Fields{
		"function": "SetVideoReceiveCallback",
	}).Info("Video receive callback registered")
}

// SetCallCallback registers a callback for incoming call requests.
//
// The callback will be invoked when a friend initiates a call with this user.
// This allows the ToxAV layer to notify applications about incoming calls
// so they can be accepted or rejected.
//
// Parameters:
//   - callback: Function to call when a call request is received, or nil to unregister
func (m *Manager) SetCallCallback(callback func(friendNumber uint32, audioEnabled, videoEnabled bool)) {
	logrus.WithFields(logrus.Fields{
		"function":        "SetCallCallback",
		"callback_is_nil": callback == nil,
	}).Debug("Registering call request callback")

	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCallback = callback

	logrus.WithFields(logrus.Fields{
		"function": "SetCallCallback",
	}).Info("Call request callback registered")
}

// SetCallStateCallback registers a callback for call state changes.
//
// The callback will be invoked whenever a call's state changes, such as
// when a call is answered, paused, resumed, or ended. This allows the
// ToxAV layer to notify applications about call state transitions.
//
// Parameters:
//   - callback: Function to call when call state changes, or nil to unregister
func (m *Manager) SetCallStateCallback(callback func(friendNumber uint32, state CallState)) {
	logrus.WithFields(logrus.Fields{
		"function":        "SetCallStateCallback",
		"callback_is_nil": callback == nil,
	}).Debug("Registering call state change callback")

	m.mu.Lock()
	defer m.mu.Unlock()
	m.callStateCallback = callback

	logrus.WithFields(logrus.Fields{
		"function": "SetCallStateCallback",
	}).Info("Call state change callback registered")
}

// updateCallState is a helper method that updates a call's state and invokes
// the call state callback if registered. This ensures callbacks are consistently
// triggered for all state changes.
//
// This method must be called with m.mu already locked.
func (m *Manager) updateCallState(call *Call, newState CallState) {
	call.SetState(newState)

	// Invoke call state callback if registered
	if m.callStateCallback != nil {
		friendNumber := call.friendNumber
		m.callStateCallback(friendNumber, newState)
		logrus.WithFields(logrus.Fields{
			"function":      "updateCallState",
			"friend_number": friendNumber,
			"new_state":     newState,
		}).Debug("Call state callback invoked")
	}
}
